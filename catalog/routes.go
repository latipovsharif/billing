package catalog

import "github.com/gin-gonic/gin"

// GetRoutes registers catalog endpoints under an already-authenticated group.
func GetRoutes(r *gin.RouterGroup, ct *Controller) {
	r.GET("/plans", ct.ListPlans)
}

// GetAdminRoutes registers catalog provisioning endpoints under an
// already-authenticated admin group. Idempotent: safe to re-run.
func GetAdminRoutes(r *gin.RouterGroup, ct *AdminController) {
	r.POST("/plans", ct.UpsertPlan)
	r.POST("/plans/:id/prices", ct.AddPrice)
}
