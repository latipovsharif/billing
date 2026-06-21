package subscriptions

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func parseID(s string) int64 {
	id, _ := strconv.ParseInt(s, 10, 64)
	return id
}

func GetRoutes(r *gin.RouterGroup, ct *Controller) {
	r.POST("/subscriptions", ct.Create)
	r.GET("/subscriptions/:id", ct.Get)
	r.POST("/subscriptions/:id/cancel", ct.Cancel)
}
