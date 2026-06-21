package payments

import (
	"net/http"
	"strconv"

	"billing/base"
	"billing/middleware"
	"billing/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// pool can both begin a tx and run queries (a *pgxpool.Pool).
type pool interface {
	base.Beginner
	base.PGXDB
}

type Controller struct {
	pool pool
	svc  *Service
}

func NewController(p pool) *Controller {
	return &Controller{pool: p, svc: NewService(NewManual(), nil)}
}

// MarkPaid POST /v1/admin/invoices/:id/mark-paid
func (ct *Controller) MarkPaid(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	pid := middleware.ProductID(c)
	err := base.WithTx(c.Request.Context(), ct.pool, func(tx pgx.Tx) error {
		return ct.svc.MarkPaid(c.Request.Context(), tx, pid, id, "operator")
	})
	if err != nil {
		response.Err(c, http.StatusBadRequest, "mark_paid_failed", err.Error())
		return
	}
	response.OK(c, http.StatusOK, gin.H{"status": "paid"})
}
