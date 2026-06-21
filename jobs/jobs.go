// Package jobs runs idempotent lifecycle workers. Each query claims rows with
// FOR UPDATE SKIP LOCKED so multiple instances never double-process.
package jobs

import (
	"context"
	"time"

	"billing/base"
	"billing/subscriptions"
	"billing/webhooks"
)

// Runner holds job configuration.
type Runner struct {
	graceDays int
	subs      *subscriptions.Repo
	outbox    *webhooks.Outbox
}

func NewRunner(graceDays int) *Runner {
	return &Runner{graceDays: graceDays, subs: subscriptions.NewRepo(), outbox: webhooks.NewOutbox()}
}

type claim struct {
	id         int64
	productID  int64
	customerID int64
	status     subscriptions.Status
}

// claimSubs selects subscriptions matching cond (SKIP LOCKED). cond is a WHERE
// fragment referencing alias s and may use args starting at $1.
func (r *Runner) claimSubs(ctx context.Context, db base.PGXDB, cond string, args ...any) ([]claim, error) {
	q := `SELECT s.id, c.product_id, s.customer_id, s.status
	      FROM subscription s JOIN customer c ON c.id=s.customer_id
	      WHERE ` + cond + `
	      ORDER BY s.id FOR UPDATE OF s SKIP LOCKED`
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []claim
	for rows.Next() {
		var c claim
		if err := rows.Scan(&c.id, &c.productID, &c.customerID, &c.status); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Runner) advance(ctx context.Context, db base.PGXDB, c claim, ev subscriptions.Event, reason, evtType string) error {
	to, ok := subscriptions.Next(c.status, ev)
	if !ok {
		return nil // already advanced / illegal — idempotent no-op
	}
	if err := r.subs.SetStatus(ctx, db, c.id, c.status, to, reason, "system", nil, nil, nil); err != nil {
		return err
	}
	return r.outbox.Enqueue(ctx, db, c.productID, evtType, webhooks.Payload{
		CustomerID: c.customerID, SubscriptionID: c.id, Status: string(to),
	})
}

// TrialExpiry: trialing past trial_end -> past_due.
func (r *Runner) TrialExpiry(ctx context.Context, db base.PGXDB) error {
	cs, err := r.claimSubs(ctx, db, `s.status='trialing' AND s.trial_end IS NOT NULL AND s.trial_end <= now()`)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if err := r.advance(ctx, db, c, subscriptions.EvPeriodEnd, "trial_expired", "subscription.past_due"); err != nil {
			return err
		}
	}
	return nil
}

// GraceExpiry: past_due longer than graceDays -> suspended.
func (r *Runner) GraceExpiry(ctx context.Context, db base.PGXDB) error {
	cutoff := time.Now().AddDate(0, 0, -r.graceDays)
	cs, err := r.claimSubs(ctx, db,
		`s.status='past_due' AND s.id IN (
		   SELECT subscription_id FROM subscription_status_history
		   WHERE to_status='past_due' AND created_at <= $1)`, cutoff)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if err := r.advance(ctx, db, c, subscriptions.EvGraceExpired, "grace_expired", "subscription.suspended"); err != nil {
			return err
		}
	}
	return nil
}

// RenewalDue: active subscriptions past current_period_end -> past_due, issuing
// an invoice for the next period (idempotent: the sub leaves 'active' so a
// re-run won't re-select it).
func (r *Runner) RenewalDue(ctx context.Context, db base.PGXDB) error {
	cs, err := r.claimSubs(ctx, db, `s.status='active' AND s.current_period_end IS NOT NULL AND s.current_period_end <= now()`)
	if err != nil {
		return err
	}
	for _, c := range cs {
		_, _ = db.Exec(ctx,
			`INSERT INTO invoice (subscription_id, customer_id, currency, amount, status, period_start, period_end, due_date)
			 SELECT s.id, s.customer_id, s.currency, s.amount, 'open', now(), now()+interval '1 month', now()
			 FROM subscription s WHERE s.id=$1
			 ON CONFLICT (subscription_id, period_start) DO NOTHING`, c.id)
		if err := r.advance(ctx, db, c, subscriptions.EvPeriodEnd, "period_end", "subscription.past_due"); err != nil {
			return err
		}
	}
	return nil
}
