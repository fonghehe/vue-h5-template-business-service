package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
)

func newRouter(handlers ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(handlers...)
	router.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return router
}

func get(router *gin.Engine, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if request.Header.Get("X-Forwarded-For") == "" {
		request.RemoteAddr = "10.0.0.1:1234"
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestRequestIDIsGeneratedWhenAbsent(t *testing.T) {
	recorder := get(newRouter(RequestID()), nil)

	assert.NotEmpty(t, recorder.Header().Get("X-Request-ID"))
}

func TestRequestIDHonoursIncomingValue(t *testing.T) {
	recorder := get(newRouter(RequestID()), map[string]string{"X-Request-ID": "from-frontend"})

	assert.Equal(t, "from-frontend", recorder.Header().Get("X-Request-ID"))
}

func TestRequestIDRejectsOversizedValue(t *testing.T) {
	// An unbounded header value would let a client blow up our log lines.
	recorder := get(newRouter(RequestID()), map[string]string{"X-Request-ID": string(make([]byte, 500))})

	assert.Len(t, recorder.Header().Get("X-Request-ID"), 36)
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	router := newRouter(CORS([]string{"https://app.example.com"}))

	recorder := get(router, map[string]string{"Origin": "https://app.example.com"})

	assert.Equal(t, "https://app.example.com", recorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", recorder.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "Origin", recorder.Header().Get("Vary"))
}

func TestCORSBlocksUnknownOrigin(t *testing.T) {
	router := newRouter(CORS([]string{"https://app.example.com"}))

	recorder := get(router, map[string]string{"Origin": "https://evil.example.com"})

	assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSPreflightShortCircuits(t *testing.T) {
	router := newRouter(CORS([]string{"https://app.example.com"}))
	request := httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "https://app.example.com")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestRateLimitAllowsUpToBurstThenRejects(t *testing.T) {
	limiter := NewRateLimit(0, 3) // no refill: bucket is exactly the burst

	for i := 0; i < 3; i++ {
		recorder := get(newRouter(limiter.Handler()), nil)
		require.Equal(t, http.StatusOK, recorder.Code, "request %d should be allowed", i+1)
	}

	recorder := get(newRouter(limiter.Handler()), nil)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	assert.Equal(t, "1", recorder.Header().Get("Retry-After"))
	assert.JSONEq(t, `{"code":4290,"message":"too many requests","data":null,"error":null}`, recorder.Body.String())
}

func TestRateLimitRefillsOverTime(t *testing.T) {
	// 100 tokens per second refills one token every 10ms.
	limiter := NewRateLimit(100, 1)

	require.Equal(t, http.StatusOK, get(newRouter(limiter.Handler()), nil).Code)
	require.Equal(t, http.StatusTooManyRequests, get(newRouter(limiter.Handler()), nil).Code)

	time.Sleep(40 * time.Millisecond)
	assert.Equal(t, http.StatusOK, get(newRouter(limiter.Handler()), nil).Code,
		"tokens should have refilled after waiting")
}

func TestRateLimitIsKeyedByClient(t *testing.T) {
	limiter := NewRateLimit(0, 1)
	router := newRouter(limiter.Handler())

	first := get(router, map[string]string{"X-Forwarded-For": "1.1.1.1"})
	second := get(router, map[string]string{"X-Forwarded-For": "2.2.2.2"})

	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, http.StatusOK, second.Code, "a separate client has its own bucket")
}

func TestSecurityHeadersAreApplied(t *testing.T) {
	recorder := get(newRouter(SecurityHeaders()), nil)

	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", recorder.Header().Get("Referrer-Policy"))
}

func TestRequestIDIsAvailableToHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(response.ContextKeyRequestID))
	})

	recorder := get(router, map[string]string{"X-Request-ID": "abc"})

	assert.Equal(t, "abc", recorder.Body.String())
}
