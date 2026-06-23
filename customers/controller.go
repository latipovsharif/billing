package customers

import (
	"net/http"

	"billing/base"
	"billing/middleware"
	"billing/response"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	pool base.PGXDB
	repo *Repo
}

func NewController(pool base.PGXDB) *Controller { return &Controller{pool: pool, repo: NewRepo()} }

type registerReq struct {
	ExternalRef     string `json:"external_ref" binding:"required"`
	OwnerUserID     string `json:"owner_user_id" binding:"required"`
	DisplayName     string `json:"display_name" binding:"required"`
	DefaultCurrency string `json:"default_currency" binding:"required"`
}

// Register POST /v1/customers — idempotent tenant registration.
func (ct *Controller) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Err(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	middleware.LogField(c, "external_ref", req.ExternalRef)
	middleware.LogField(c, "currency", req.DefaultCurrency)
	cust, err := ct.repo.Register(c.Request.Context(), ct.pool,
		middleware.ProductID(c), req.ExternalRef, req.OwnerUserID, req.DisplayName, req.DefaultCurrency)
	if err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	middleware.LogField(c, "customer_id", cust.ID)
	response.OK(c, http.StatusOK, cust)
}

// Get GET /v1/customers/:ref
func (ct *Controller) Get(c *gin.Context) {
	cust, ok, err := ct.repo.ByRef(c.Request.Context(), ct.pool, middleware.ProductID(c), c.Param("ref"))
	if err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok {
		response.Err(c, http.StatusNotFound, "not_found", "customer not found")
		return
	}
	response.OK(c, http.StatusOK, cust)
}
