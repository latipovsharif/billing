package subscriptions

import (
	"context"
	"time"

	"billing/base"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

// Insert creates a subscription row and its initial history entry.
func (r *Repo) Insert(ctx context.Context, db base.PGXDB, s Subscription, reason, actor string) (int64, error) {
	var id int64
	err := db.QueryRow(ctx,
		`INSERT INTO subscription
		 (customer_id, plan_id, status, currency, interval, amount, trial_end, current_period_start, current_period_end)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		s.CustomerID, s.PlanID, s.Status, s.Currency, s.Interval, s.Amount,
		s.TrialEnd, s.CurrentPeriodStart, s.CurrentPeriodEnd).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, r.addHistory(ctx, db, id, "", string(s.Status), reason, actor)
}

// Get loads a subscription by id.
func (r *Repo) Get(ctx context.Context, db base.PGXDB, id int64) (Subscription, bool, error) {
	var s Subscription
	err := db.QueryRow(ctx,
		`SELECT id, customer_id, plan_id, status, currency, interval, amount,
		        trial_end, current_period_start, current_period_end, cancel_at, canceled_at
		 FROM subscription WHERE id=$1`, id).
		Scan(&s.ID, &s.CustomerID, &s.PlanID, &s.Status, &s.Currency, &s.Interval, &s.Amount,
			&s.TrialEnd, &s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelAt, &s.CanceledAt)
	if base.IsNotFound(err) {
		return Subscription{}, false, nil
	}
	return s, err == nil, err
}

// SetStatus moves a subscription to a new status and records history. Caller
// must validate the transition via Next().
func (r *Repo) SetStatus(ctx context.Context, db base.PGXDB, id int64, from, to Status, reason, actor string, periodStart, periodEnd, canceledAt *time.Time) error {
	_, err := db.Exec(ctx,
		`UPDATE subscription
		 SET status=$2,
		     current_period_start=COALESCE($3,current_period_start),
		     current_period_end=COALESCE($4,current_period_end),
		     canceled_at=COALESCE($5,canceled_at)
		 WHERE id=$1`, id, to, periodStart, periodEnd, canceledAt)
	if err != nil {
		return err
	}
	return r.addHistory(ctx, db, id, string(from), string(to), reason, actor)
}

func (r *Repo) addHistory(ctx context.Context, db base.PGXDB, subID int64, from, to, reason, actor string) error {
	_, err := db.Exec(ctx,
		`INSERT INTO subscription_status_history (subscription_id, from_status, to_status, reason, actor)
		 VALUES ($1, NULLIF($2,''), $3, $4, $5)`, subID, from, to, reason, actor)
	return err
}
