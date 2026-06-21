package payments

import "github.com/gin-gonic/gin"

// GetAdminRoutes registers operator payment endpoints under an admin group.
func GetAdminRoutes(r *gin.RouterGroup, ct *Controller) {
	r.POST("/invoices/:id/mark-paid", ct.MarkPaid)
}
