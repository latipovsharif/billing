package catalog

// Plan is a product's tariff with limits and trial length.
type Plan struct {
	ID        int64          `json:"id"`
	ProductID int64          `json:"product_id"`
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Limits    map[string]any `json:"limits"`
	TrialDays int            `json:"trial_days"`
	Active    bool           `json:"active"`
}

// Price is one (currency, interval) price line for a plan.
type Price struct {
	ID       int64  `json:"id"`
	PlanID   int64  `json:"plan_id"`
	Currency string `json:"currency"`
	Interval string `json:"interval"` // month | year
	Amount   int64  `json:"amount"`
}

// PlanWithPrices is the read model returned to clients (pricing page).
type PlanWithPrices struct {
	Plan
	Prices []Price `json:"prices"`
}
