package catalog

import "github.com/gin-gonic/gin"

// GetRoutes registers catalog endpoints under an already-authenticated group.
func GetRoutes(r *gin.RouterGroup, ct *Controller) {
	r.GET("/plans", ct.ListPlans)
}
