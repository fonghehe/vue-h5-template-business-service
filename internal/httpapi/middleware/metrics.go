package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fonghehe/vue-h5-template-business-service/internal/metrics"
)

func Prometheus(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		// Extension methods are valid HTTP, but must not create unbounded
		// metric children on unauthenticated/unmatched requests.
		switch method {
		case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		default:
			method = "OTHER"
		}
		m.HTTPRequests.WithLabelValues(method, route, strconv.Itoa(c.Writer.Status())).Inc()
		m.HTTPRequestDuration.WithLabelValues(method, route).Observe(time.Since(started).Seconds())
	}
}
