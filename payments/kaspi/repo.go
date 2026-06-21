package kaspi

import (
	"context"

	"billing/base"
)

type KaspiRepo struct{}

func NewRepo() *KaspiRepo { return &KaspiRepo{} }

// InvoiceForQR returns the amount, owning product, and status of an invoice
// (product via invoice→customer). ok=false if the invoice does not exist.
func (r *KaspiRepo) InvoiceForQR(ctx context.Context, db base.PGXDB, invoiceID int64) (amount, productID int64, status string, ok bool, err error) {
	err = db.QueryRow(ctx,
		`SELECT i.amount, c.product_id, i.status
		 FROM invoice i JOIN customer c ON c.id=i.customer_id
		 WHERE i.id=$1`, invoiceID).Scan(&amount, &productID, &status)
	if base.IsNotFound(err) {
		return 0, 0, "", false, nil
	}
	return amount, productID, status, err == nil, err
}

// Save inserts a pending kaspi_payment for an invoice.
func (r *KaspiRepo) Save(ctx context.Context, db base.PGXDB, invoiceID int64, qrInvoiceID, qrToken string) error {
	_, err := db.Exec(ctx,
		`INSERT INTO kaspi_payment (invoice_id, qr_invoice_id, qr_token) VALUES ($1,$2,$3)`,
		invoiceID, qrInvoiceID, qrToken)
	return err
}

// PendingByInvoice returns the live (pending) QR for an invoice, if any.
func (r *KaspiRepo) PendingByInvoice(ctx context.Context, db base.PGXDB, invoiceID int64) (qrInvoiceID, qrToken string, ok bool, err error) {
	err = db.QueryRow(ctx,
		`SELECT qr_invoice_id, qr_token FROM kaspi_payment WHERE invoice_id=$1 AND status='pending'`, invoiceID).
		Scan(&qrInvoiceID, &qrToken)
	if base.IsNotFound(err) {
		return "", "", false, nil
	}
	return qrInvoiceID, qrToken, err == nil, err
}

// Pending is one row the poller must check.
type Pending struct {
	ID          int64
	InvoiceID   int64
	QRInvoiceID string
	ProductID   int64
}

// ClaimPending selects pending rows with FOR UPDATE SKIP LOCKED.
func (r *KaspiRepo) ClaimPending(ctx context.Context, db base.PGXDB) ([]Pending, error) {
	rows, err := db.Query(ctx,
		`SELECT k.id, k.invoice_id, k.qr_invoice_id, c.product_id
		 FROM kaspi_payment k
		 JOIN invoice i ON i.id=k.invoice_id
		 JOIN customer c ON c.id=i.customer_id
		 WHERE k.status='pending'
		 ORDER BY k.id FOR UPDATE OF k SKIP LOCKED`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pending
	for rows.Next() {
		var p Pending
		if err := rows.Scan(&p.ID, &p.InvoiceID, &p.QRInvoiceID, &p.ProductID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Mark sets a kaspi_payment's terminal status.
func (r *KaspiRepo) Mark(ctx context.Context, db base.PGXDB, id int64, status string) error {
	_, err := db.Exec(ctx, `UPDATE kaspi_payment SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	return err
}
