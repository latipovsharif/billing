package catalog

import (
	"net/http"

	"billing/middleware"
	"billing/response"

	"github.com/gin-gonic/gin"
)

type Controller struct{ svc *Service }

func NewController(svc *Service) *Controller { return &Controller{svc: svc} }

// ListPlans GET /v1/plans — plans+prices for the authenticated product.
func (ct *Controller) ListPlans(c *gin.Context) {
	plans, err := ct.svc.ListPlans(c.Request.Context(), middleware.ProductID(c))
	if err != nil {
		response.Err(c, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	response.OK(c, http.StatusOK, gin.H{"plans": plans})
}
