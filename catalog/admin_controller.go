package catalog

import (
	"errors"
	"net/http"
	"strconv"

	"billing/base"
	"billing/middleware"
	"billing/response"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// errPlanNotFound signals the addressed plan does not belong to the product.
var errPlanNotFound = errors.New("plan not found")

// adminPool can both begin a tx and run queries (a *pgxpool.Pool).
type adminPool interface {
	base.Beginner
	base.PGXDB
}

// AdminController provisions the catalog (plans + prices) for the authenticated
// product. Writes are idempotent so operator setup can be re-run safely.
type AdminController struct {
	pool adminPool
	repo *Repo
}

func NewAdminController(p adminPool) *AdminController {
	return &AdminController{pool: p, repo: NewRepo()}
}

type priceReq struct {
	Currency string `json:"currency" binding:"required"`
	Interval string `json:"interval" binding:"required,oneof=month year"`
	Amount   int64  `json:"amount" binding:"gte=0"`
}

type upsertPlanReq struct {
	Code      string         `json:"code" binding:"required"`
	Name      string         `json:"name" binding:"required"`
	TrialDays int            `json:"trial_days"`
	Limits    map[string]any `json:"limits"`
	Prices    []priceReq     `json:"prices"`
}

// UpsertPlan POST /v1/admin/plans — create/update a plan and its inline prices.
func (ct *AdminController) UpsertPlan(c *gin.Context) {
	var req upsertPlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Err(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	// Omitted/zero trial length defaults to the table's 14 days; trial_days only
	// matters for trial subscriptions, so this is safe for active-only plans too.
	if req.TrialDays <= 0 {
		req.TrialDays = 14
	}
	pid := middleware.ProductID(c)
	middleware.LogField(c, "plan_code", req.Code)
	middleware.LogField(c, "prices", len(req.Prices))

	var out PlanWithPrices
	err := base.WithTx(c.Request.Context(), ct.pool, func(tx pgx.Tx) error {
		planID, e := ct.repo.UpsertPlan(c.Request.Context(), tx, pid, req.Code, req.Name, req.Limits, req.TrialDays)
		if e != nil {
			return e
		}
		for _, p := range req.Prices {
			if _, e := ct.repo.UpsertPrice(c.Request.Context(), tx, planID, p.Currency, p.Interval, p.Amount); e != nil {
				return e
			}
		}
		plans, e := ct.repo.ListPlansWithPrices(c.Request.Context(), tx, pid)
		if e != nil {
			return e
		}
		for _, pl := range plans {
			if pl.ID == planID {
				out = pl
				break
			}
		}
		return nil
	})
	if err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	response.OK(c, http.StatusCreated, out)
}

// AddPrice POST /v1/admin/plans/:id/prices — create/update one price on a plan
// owned by the authenticated product.
func (ct *AdminController) AddPrice(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Err(c, http.StatusBadRequest, "bad_request", "invalid plan id")
		return
	}
	var req priceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Err(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	pid := middleware.ProductID(c)

	var price Price
	err = base.WithTx(c.Request.Context(), ct.pool, func(tx pgx.Tx) error {
		_, ok, e := ct.repo.PlanByID(c.Request.Context(), tx, pid, planID)
		if e != nil {
			return e
		}
		if !ok {
			return errPlanNotFound
		}
		id, e := ct.repo.UpsertPrice(c.Request.Context(), tx, planID, req.Currency, req.Interval, req.Amount)
		if e != nil {
			return e
		}
		price = Price{ID: id, PlanID: planID, Currency: req.Currency, Interval: req.Interval, Amount: req.Amount}
		return nil
	})
	if err == errPlanNotFound {
		response.Err(c, http.StatusNotFound, "not_found", "plan not found")
		return
	}
	if err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	response.OK(c, http.StatusCreated, price)
}
