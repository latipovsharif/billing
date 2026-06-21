// Package jobs runs idempotent lifecycle workers. Each query claims rows with
// FOR UPDATE SKIP LOCKED so multiple instances never double-process.
package jobs

import (
	"context"
	"time"

	"billing/base"
	"billing/subscriptions"
	"billing/webhooks"

	log "github.com/sirupsen/logrus"
)

// Charger settles an invoice using the customer's saved card. Implemented by
// *payments.Service.ChargeInvoice. nil disables auto-charge (manual-only).
type Charger interface {
	ChargeInvoice(ctx context.Context, db base.PGXDB, productID, invoiceID int64) error
}

// Runner holds job configuration.
type Runner struct {
	graceDays int
	subs      *subscriptions.Repo
	outbox    *webhooks.Outbox
	charger   Charger
}

// NewRunner builds the job runner. charger may be nil (no auto-charge).
func NewRunner(graceDays int, charger Charger) *Runner {
	return &Runner{graceDays: graceDays, subs: subscriptions.NewRepo(), outbox: webhooks.NewOutbox(), charger: charger}
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

// ensureOpenInvoice returns the id of the subscription's current open invoice,
// creating one (amount/currency from the subscription) if none exists. Idempotent.
func (r *Runner) ensureOpenInvoice(ctx context.Context, db base.PGXDB, subID int64) (int64, error) {
	_, err := db.Exec(ctx,
		`INSERT INTO invoice (subscription_id, customer_id, currency, amount, status, period_start, period_end, due_date)
		 SELECT s.id, s.customer_id, s.currency, s.amount, 'open', now(), now()+interval '1 month', now()
		 FROM subscription s WHERE s.id=$1
		   AND NOT EXISTS (SELECT 1 FROM invoice WHERE subscription_id=s.id AND status='open')`, subID)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.QueryRow(ctx,
		`SELECT id FROM invoice WHERE subscription_id=$1 AND status='open' ORDER BY id DESC LIMIT 1`, subID).Scan(&id)
	return id, err
}

// DunningCharge auto-charges past_due subscriptions that have a verified default
// card. Covers trial conversion, renewal, and in-grace retries uniformly: a
// successful charge settles the invoice and the state-machine moves past_due ->
// active. Charge failures are left as-is (GraceExpiry eventually suspends).
func (r *Runner) DunningCharge(ctx context.Context, db base.PGXDB) error {
	if r.charger == nil {
		return nil
	}
	cs, err := r.claimSubs(ctx, db,
		`s.status='past_due' AND EXISTS (
		   SELECT 1 FROM payment_method m
		   WHERE m.customer_id=s.customer_id AND m.verified AND m.is_default)`)
	if err != nil {
		return err
	}
	for _, c := range cs {
		invID, err := r.ensureOpenInvoice(ctx, db, c.id)
		if err != nil {
			return err
		}
		if err := r.charger.ChargeInvoice(ctx, db, c.productID, invID); err != nil {
			log.Warnf("dunning charge sub %d: %v", c.id, err) // leave past_due; retry next tick
		}
	}
	return nil
}
