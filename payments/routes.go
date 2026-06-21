package payments

import "github.com/gin-gonic/gin"

// GetAdminRoutes registers operator payment endpoints under an admin group.
func GetAdminRoutes(r *gin.RouterGroup, ct *Controller) {
	r.POST("/invoices/:id/mark-paid", ct.MarkPaid)
}

// GetCardRoutes registers card-binding endpoints under the authenticated v1 group.
func GetCardRoutes(r *gin.RouterGroup, ct *CardController) {
	r.POST("/payment-methods/card", ct.AddCard)
	r.POST("/payment-methods/:id/verify", ct.VerifyCard)
	r.GET("/payment-methods", ct.ListCards)
	r.DELETE("/payment-methods/:id", ct.RemoveCard)
}
