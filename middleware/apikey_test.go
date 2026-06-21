package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIKeyResolvesProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resolver := func(key string) (int64, bool) {
		if key == "good" {
			return 42, true
		}
		return 0, false
	}
	r := gin.New()
	r.Use(APIKey(resolver))
	r.GET("/x", func(c *gin.Context) { c.JSON(200, gin.H{"pid": ProductID(c)}) })

	// missing key -> 401
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing key: got %d", w.Code)
	}
	// good key -> 200 and product id in context
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer good")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("good key: got %d", w.Code)
	}
}
