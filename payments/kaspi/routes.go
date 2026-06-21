package kaspi

import "github.com/gin-gonic/gin"

// GetRoutes registers the Kaspi QR endpoint under the authenticated v1 group.
func GetRoutes(r *gin.RouterGroup, ct *Controller) {
	r.POST("/invoices/:id/kaspi-qr", ct.CreateQR)
}
