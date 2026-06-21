package kaspi

import (
	"context"

	"billing/base"

	log "github.com/sirupsen/logrus"
)

// Settler settles an externally-confirmed invoice. Satisfied by
// *payments.Service (SettleExternal) — declared here to keep kaspi decoupled.
type Settler interface {
	SettleExternal(ctx context.Context, db base.PGXDB, productID, invoiceID int64, providerName, providerRef string, raw map[string]any) error
}

// Poller checks pending Kaspi QR payments and settles paid ones.
type Poller struct {
	client  *Client
	repo    *KaspiRepo
	settler Settler
}

func NewPoller(client *Client, repo *KaspiRepo, settler Settler) *Poller {
	return &Poller{client: client, repo: repo, settler: settler}
}

// Poll claims pending kaspi_payments (SKIP LOCKED), queries Kaspi, and settles
// or marks each. Idempotent: settle is a no-op on an already-paid invoice.
func (p *Poller) Poll(ctx context.Context, db base.PGXDB) error {
	rows, err := p.repo.ClaimPending(ctx, db)
	if err != nil {
		return err
	}
	for _, row := range rows {
		status, err := p.client.PaymentInfo(ctx, row.QRInvoiceID)
		if err != nil {
			log.Warnf("kaspi poll %s: %v", row.QRInvoiceID, err)
			continue // transient — retry next tick
		}
		switch status {
		case "paid":
			if err := p.settler.SettleExternal(ctx, db, row.ProductID, row.InvoiceID, "kaspi", row.QRInvoiceID, map[string]any{"qr_invoice_id": row.QRInvoiceID}); err != nil {
				return err
			}
			if err := p.repo.Mark(ctx, db, row.ID, "paid"); err != nil {
				return err
			}
		case "canceled", "expired":
			if err := p.repo.Mark(ctx, db, row.ID, status); err != nil {
				return err
			}
		}
	}
	return nil
}
