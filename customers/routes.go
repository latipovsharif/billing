package customers

import "github.com/gin-gonic/gin"

func GetRoutes(r *gin.RouterGroup, ct *Controller) {
	r.POST("/customers", ct.Register)
	r.GET("/customers/:ref", ct.Get)
}
