package middleware

import (
	"bytes"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const ctxLogFields = "log_fields"

// bodyCapture tees the response body into a buffer so the logger can include
// the error detail (e.g. {"code":"create_failed","message":"unknown plan ..."})
// on non-2xx responses.
type bodyCapture struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w bodyCapture) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

// LogField attaches a structured field to the current request, included in the
// line RequestLogger emits when the handler returns. Handlers use it to record
// domain context (customer_id, plan_code, ...) so successes and failures are
// diagnosable from billing's own logs.
func LogField(c *gin.Context, key string, val any) {
	f, _ := c.Get(ctxLogFields)
	fields, _ := f.(log.Fields)
	if fields == nil {
		fields = log.Fields{}
	}
	fields[key] = val
	c.Set(ctxLogFields, fields)
}

// RequestLogger emits one structured log line per request: method, route,
// status, latency, product_id, any fields set via LogField, and — for non-2xx —
// the response error body. 5xx -> error, 4xx -> warn, else info. /healthz is
// skipped to avoid noise.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/healthz" {
			c.Next()
			return
		}
		start := time.Now()
		buf := &bytes.Buffer{}
		c.Writer = bodyCapture{ResponseWriter: c.Writer, buf: buf}

		c.Next()

		status := c.Writer.Status()
		fields := log.Fields{
			"method":     c.Request.Method,
			"route":      c.FullPath(),
			"status":     status,
			"latency_ms": time.Since(start).Milliseconds(),
		}
		if pid := ProductID(c); pid != 0 {
			fields["product_id"] = pid
		}
		if f, ok := c.Get(ctxLogFields); ok {
			if extra, ok := f.(log.Fields); ok {
				for k, v := range extra {
					fields[k] = v
				}
			}
		}

		switch {
		case status >= 500:
			fields["error"] = truncate(buf.String(), 512)
			log.WithFields(fields).Error("request failed")
		case status >= 400:
			fields["error"] = truncate(buf.String(), 512)
			log.WithFields(fields).Warn("request rejected")
		default:
			log.WithFields(fields).Info("request")
		}
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
