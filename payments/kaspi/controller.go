package kaspi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"billing/base"
	"billing/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// pool can both begin a tx and run queries (a *pgxpool.Pool).
type pool interface {
	base.Beginner
	base.PGXDB
}

// QRMaker is the subset of *Client the controller needs.
type QRMaker interface {
	CreateQR(ctx context.Context, orderID string, amount int64) (qrInvoiceID, qrToken string, err error)
}

type Controller struct {
	pool   pool
	client QRMaker
	repo   *KaspiRepo
}

func NewController(p pool, client QRMaker, repo *KaspiRepo) *Controller {
	return &Controller{pool: p, client: client, repo: repo}
}

// CreateQR POST /v1/invoices/:id/kaspi-qr — returns a QR for an open invoice.
// Idempotent: an existing pending QR for the invoice is reused.
func (ct *Controller) CreateQR(c *gin.Context) {
	invID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ctx := c.Request.Context()

	amount, _, status, ok, err := ct.repo.InvoiceForQR(ctx, ct.pool, invID)
	if err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok {
		response.Err(c, http.StatusNotFound, "not_found", "invoice not found")
		return
	}
	if status != "open" {
		response.Err(c, http.StatusConflict, "invoice_not_open", "invoice is "+status)
		return
	}
	if qi, tok, exists, err := ct.repo.PendingByInvoice(ctx, ct.pool, invID); err == nil && exists {
		response.OK(c, http.StatusOK, gin.H{"qr_invoice_id": qi, "qr_token": tok})
		return
	}

	qi, tok, err := ct.client.CreateQR(ctx, fmt.Sprintf("inv-%d", invID), amount)
	if err != nil {
		response.Err(c, http.StatusBadGateway, "kaspi_create_failed", err.Error())
		return
	}
	if err := base.WithTx(ctx, ct.pool, func(tx pgx.Tx) error {
		return ct.repo.Save(ctx, tx, invID, qi, tok)
	}); err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	response.OK(c, http.StatusOK, gin.H{"qr_invoice_id": qi, "qr_token": tok})
}
