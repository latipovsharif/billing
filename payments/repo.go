package payments

import (
	"context"
	"encoding/json"
	"time"

	"billing/base"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

// Invoice is the minimal view needed to pay.
type Invoice struct {
	ID             int64
	SubscriptionID int64
	CustomerID     int64
	Currency       string
	Amount         int64
	Status         string
}

func (r *Repo) GetInvoice(ctx context.Context, db base.PGXDB, id int64) (Invoice, bool, error) {
	var iv Invoice
	err := db.QueryRow(ctx,
		`SELECT id, subscription_id, customer_id, currency, amount, status FROM invoice WHERE id=$1`, id).
		Scan(&iv.ID, &iv.SubscriptionID, &iv.CustomerID, &iv.Currency, &iv.Amount, &iv.Status)
	if base.IsNotFound(err) {
		return Invoice{}, false, nil
	}
	return iv, err == nil, err
}

// InsertPayment appends a payment row (append-only) and returns its id.
func (r *Repo) InsertPayment(ctx context.Context, db base.PGXDB, invoiceID int64, provider, providerRef, currency string, amount int64, raw map[string]any) (int64, error) {
	b, _ := json.Marshal(raw)
	var id int64
	err := db.QueryRow(ctx,
		`INSERT INTO payment (invoice_id, provider, provider_ref, currency, amount, status, raw)
		 VALUES ($1,$2,$3,$4,$5,'succeeded',$6) RETURNING id`,
		invoiceID, provider, providerRef, currency, amount, b).Scan(&id)
	return id, err
}

// MarkInvoicePaid flips the cached invoice status.
func (r *Repo) MarkInvoicePaid(ctx context.Context, db base.PGXDB, invoiceID int64) error {
	_, err := db.Exec(ctx, `UPDATE invoice SET status='paid', paid_at=$2 WHERE id=$1`, invoiceID, time.Now())
	return err
}
