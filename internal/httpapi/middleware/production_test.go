package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/metrics"
	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeadlineCancelsDownstreamAndReturnsJSON(t *testing.T) {
	router := gin.New()
	router.Use(RequestID(), Timeout(10*time.Millisecond))
	router.GET("/slow", func(c *gin.Context) {
		<-c.Request.Context().Done()
		response.Fail(c, apierr.Wrap(apierr.CodeInternal, "database request failed", c.Request.Context().Err()))
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/slow", nil))
	assert.Equal(t, http.StatusGatewayTimeout, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":5040`)
	assert.NotEmpty(t, recorder.Header().Get("X-Request-ID"))
}
func TestTimeoutPreservesEarlierParentDeadline(t *testing.T) {
	router := gin.New()
	router.Use(Timeout(time.Hour))
	router.GET("/", func(c *gin.Context) {
		require.ErrorIs(t, c.Request.Context().Err(), context.DeadlineExceeded)
		response.OK(c, nil)
	})
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
	assert.Equal(t, http.StatusGatewayTimeout, recorder.Code)
}
func TestRateLimitStateHasHardCapacityAndEvictsIdleVisitors(t *testing.T) {
	limiter := NewRateLimit(1, 1)
	limiter.maxVisitors = 2
	require.True(t, limiter.allow("a"))
	require.True(t, limiter.allow("b"))
	assert.False(t, limiter.allow("c"))
	assert.Len(t, limiter.visitors, 2)
	limiter.visitors["a"].last = time.Now().Add(-2 * limiter.ttl)
	limiter.lastSweep = time.Now().Add(-2 * time.Minute)
	assert.True(t, limiter.allow("c"))
	assert.Len(t, limiter.visitors, 2)
}
func TestRecoveryIsObservedByAccessLogAndMetrics(t *testing.T) {
	var log bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&log, nil))
	metricSet := metrics.New()
	router := gin.New()
	router.Use(RequestID(), AccessLog(logger), Prometheus(metricSet), Recovery(logger))
	router.GET("/panic", func(*gin.Context) { panic("private panic") })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))
	assert.Equal(t, 500, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "private panic")
	assert.Contains(t, log.String(), `"status":500`)
	scrape := httptest.NewRecorder()
	metricSet.Handler().ServeHTTP(scrape, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Contains(t, scrape.Body.String(), `http_requests_total{method="GET",route="/panic",status="500"} 1`)
}
