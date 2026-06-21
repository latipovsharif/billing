package response

import "github.com/gin-gonic/gin"

// OK writes a 2xx JSON body.
func OK(c *gin.Context, status int, body any) { c.JSON(status, body) }

// Err writes a machine-readable error: {"code","message"}.
func Err(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"code": code, "message": message})
}
