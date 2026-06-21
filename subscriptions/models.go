package subscriptions

import "time"

// Subscription is the persisted lifecycle record.
type Subscription struct {
	ID                 int64      `json:"id"`
	CustomerID         int64      `json:"customer_id"`
	PlanID             int64      `json:"plan_id"`
	Status             Status     `json:"status"`
	Currency           string     `json:"currency"`
	Interval           string     `json:"interval"`
	Amount             int64      `json:"amount"`
	TrialEnd           *time.Time `json:"trial_end,omitempty"`
	CurrentPeriodStart *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	CancelAt           *time.Time `json:"cancel_at,omitempty"`
	CanceledAt         *time.Time `json:"canceled_at,omitempty"`
}
