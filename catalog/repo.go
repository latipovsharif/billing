package catalog

import (
	"context"
	"encoding/json"

	"billing/base"
)

// Repo is the catalog data layer.
type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

func (r *Repo) CreateProduct(ctx context.Context, db base.PGXDB, key, name, apiKey string) (int64, error) {
	var id int64
	err := db.QueryRow(ctx,
		`INSERT INTO product (key, name, api_key) VALUES ($1,$2,$3) RETURNING id`,
		key, name, apiKey).Scan(&id)
	return id, err
}

func (r *Repo) CreatePlan(ctx context.Context, db base.PGXDB, productID int64, code, name string, limits map[string]any, trialDays int) (int64, error) {
	raw, err := json.Marshal(limits)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.QueryRow(ctx,
		`INSERT INTO plan (product_id, code, name, limits, trial_days)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		productID, code, name, raw, trialDays).Scan(&id)
	return id, err
}

func (r *Repo) AddPrice(ctx context.Context, db base.PGXDB, planID int64, currency, interval string, amount int64) error {
	_, err := db.Exec(ctx,
		`INSERT INTO plan_price (plan_id, currency, interval, amount) VALUES ($1,$2,$3,$4)`,
		planID, currency, interval, amount)
	return err
}

func (r *Repo) ListPlansWithPrices(ctx context.Context, db base.PGXDB, productID int64) ([]PlanWithPrices, error) {
	rows, err := db.Query(ctx,
		`SELECT id, product_id, code, name, limits, trial_days, active
		 FROM plan WHERE product_id=$1 AND active ORDER BY id`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PlanWithPrices
	byID := map[int64]int{}
	for rows.Next() {
		var p PlanWithPrices
		var raw []byte
		if err := rows.Scan(&p.ID, &p.ProductID, &p.Code, &p.Name, &raw, &p.TrialDays, &p.Active); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &p.Limits)
		byID[p.ID] = len(out)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	prows, err := db.Query(ctx,
		`SELECT pr.id, pr.plan_id, pr.currency, pr.interval, pr.amount
		 FROM plan_price pr JOIN plan p ON p.id=pr.plan_id
		 WHERE p.product_id=$1 ORDER BY pr.id`, productID)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	for prows.Next() {
		var pr Price
		if err := prows.Scan(&pr.ID, &pr.PlanID, &pr.Currency, &pr.Interval, &pr.Amount); err != nil {
			return nil, err
		}
		if i, ok := byID[pr.PlanID]; ok {
			out[i].Prices = append(out[i].Prices, pr)
		}
	}
	return out, prows.Err()
}

// PriceFor returns the (currency, interval) price for a plan, or false.
func (r *Repo) PriceFor(ctx context.Context, db base.PGXDB, planID int64, currency, interval string) (Price, bool, error) {
	var pr Price
	err := db.QueryRow(ctx,
		`SELECT id, plan_id, currency, interval, amount FROM plan_price
		 WHERE plan_id=$1 AND currency=$2 AND interval=$3`, planID, currency, interval).
		Scan(&pr.ID, &pr.PlanID, &pr.Currency, &pr.Interval, &pr.Amount)
	if base.IsNotFound(err) {
		return Price{}, false, nil
	}
	return pr, err == nil, err
}

// PlanByCode resolves a plan by product + code.
func (r *Repo) PlanByCode(ctx context.Context, db base.PGXDB, productID int64, code string) (Plan, bool, error) {
	var p Plan
	var raw []byte
	err := db.QueryRow(ctx,
		`SELECT id, product_id, code, name, limits, trial_days, active
		 FROM plan WHERE product_id=$1 AND code=$2`, productID, code).
		Scan(&p.ID, &p.ProductID, &p.Code, &p.Name, &raw, &p.TrialDays, &p.Active)
	if base.IsNotFound(err) {
		return Plan{}, false, nil
	}
	if err == nil {
		_ = json.Unmarshal(raw, &p.Limits)
	}
	return p, err == nil, err
}
