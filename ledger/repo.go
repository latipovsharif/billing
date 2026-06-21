package ledger

import (
	"context"

	"billing/base"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

// Append inserts an immutable entry and returns its id.
func (r *Repo) Append(ctx context.Context, db base.PGXDB, e Entry) (int64, error) {
	var id int64
	err := db.QueryRow(ctx,
		`INSERT INTO ledger_entry (customer_id, invoice_id, payment_id, type, currency, amount, ref)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		e.CustomerID, e.InvoiceID, e.PaymentID, e.Type, e.Currency, e.Amount, e.Ref).Scan(&id)
	return id, err
}

// Balance is the signed sum of entries for a customer+currency.
func (r *Repo) Balance(ctx context.Context, db base.PGXDB, customerID int64, currency string) (int64, error) {
	var bal int64
	err := db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM ledger_entry WHERE customer_id=$1 AND currency=$2`,
		customerID, currency).Scan(&bal)
	return bal, err
}
