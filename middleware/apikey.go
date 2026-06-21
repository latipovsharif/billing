package middleware

import (
	"strings"

	"billing/response"

	"github.com/gin-gonic/gin"
)

const ctxProductID = "product_id"

// Resolver maps a bearer API key to a product id.
type Resolver func(key string) (productID int64, ok bool)

// APIKey authenticates requests by per-product bearer key.
func APIKey(resolve Resolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		key := strings.TrimPrefix(auth, "Bearer ")
		if key == "" || key == auth { // missing prefix or empty
			response.Err(c, 401, "unauthorized", "missing bearer api key")
			c.Abort()
			return
		}
		pid, ok := resolve(key)
		if !ok {
			response.Err(c, 401, "unauthorized", "invalid api key")
			c.Abort()
			return
		}
		c.Set(ctxProductID, pid)
		c.Next()
	}
}

// ProductID returns the authenticated product id (0 if absent).
func ProductID(c *gin.Context) int64 {
	if v, ok := c.Get(ctxProductID); ok {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}
