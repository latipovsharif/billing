package subscriptions

import (
	"net/http"

	"billing/base"
	"billing/middleware"
	"billing/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// pool is anything that can both begin a tx and run queries (a *pgxpool.Pool).
type pool interface {
	base.Beginner
	base.PGXDB
}

type Controller struct {
	pool pool
	svc  *Service
}

func NewController(p pool) *Controller {
	return &Controller{pool: p, svc: NewService()}
}

type createReq struct {
	CustomerID int64  `json:"customer_id" binding:"required"`
	PlanCode   string `json:"plan_code" binding:"required"`
	Currency   string `json:"currency" binding:"required"`
	Interval   string `json:"interval" binding:"required,oneof=month year"`
	Trial      bool   `json:"trial"`
}

// Create POST /v1/subscriptions
func (ct *Controller) Create(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Err(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	pid := middleware.ProductID(c)
	middleware.LogField(c, "customer_id", req.CustomerID)
	middleware.LogField(c, "plan_code", req.PlanCode)
	middleware.LogField(c, "currency", req.Currency)
	middleware.LogField(c, "trial", req.Trial)
	var sub Subscription
	err := base.WithTx(c.Request.Context(), ct.pool, func(tx pgx.Tx) error {
		var e error
		sub, e = ct.svc.Create(c.Request.Context(), tx, pid, CreateInput{
			CustomerID: req.CustomerID, PlanCode: req.PlanCode,
			Currency: req.Currency, Interval: req.Interval, Trial: req.Trial,
		})
		return e
	})
	if err != nil {
		response.Err(c, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	middleware.LogField(c, "subscription_id", sub.ID)
	middleware.LogField(c, "sub_status", sub.Status)
	response.OK(c, http.StatusCreated, sub)
}

// Cancel POST /v1/subscriptions/:id/cancel
func (ct *Controller) Cancel(c *gin.Context) {
	id := parseID(c.Param("id"))
	pid := middleware.ProductID(c)
	err := base.WithTx(c.Request.Context(), ct.pool, func(tx pgx.Tx) error {
		return ct.svc.Cancel(c.Request.Context(), tx, pid, id, "operator")
	})
	if err != nil {
		response.Err(c, http.StatusBadRequest, "cancel_failed", err.Error())
		return
	}
	response.OK(c, http.StatusOK, gin.H{"status": "canceled"})
}

// Get GET /v1/subscriptions/:id
func (ct *Controller) Get(c *gin.Context) {
	sub, ok, err := NewRepo().Get(c.Request.Context(), ct.pool, parseID(c.Param("id")))
	if err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok {
		response.Err(c, http.StatusNotFound, "not_found", "subscription not found")
		return
	}
	response.OK(c, http.StatusOK, sub)
}
