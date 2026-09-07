package middleware

import (
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
		m.HTTPRequests.WithLabelValues(c.Request.Method, route, strconv.Itoa(c.Writer.Status())).Inc()
		m.HTTPRequestDuration.WithLabelValues(c.Request.Method, route).Observe(time.Since(started).Seconds())
	}
}
