package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/httpapi/middleware"
	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
)

func TestReadinessHonorsCancellationWhilePoolIsExhausted(t *testing.T) {
	server, db := newTestServer(t)
	pool, err := db.DB()
	require.NoError(t, err)
	connection, err := pool.Conn(context.Background())
	require.NoError(t, err)
	defer func() { _ = connection.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ready", nil).WithContext(ctx)
	completed := make(chan struct{})
	go func() {
		server.Router().ServeHTTP(recorder, request)
		close(completed)
	}()
	select {
	case <-completed:
	case <-time.After(time.Second):
		// Release the held connection before failing, so a regression cannot
		// leave the test goroutine stuck in sql.DB.Ping.
		_ = connection.Close()
		<-completed
		t.Fatal("readiness ignored request cancellation")
	}
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestInternalErrorIsLoggedWithRequestIDAndSafeResponse(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.AccessLog(logger))
	router.GET("/failure", func(c *gin.Context) {
		response.Fail(c, apierr.Wrap(apierr.CodeInternal, "Internal server error", errors.New("storage unavailable")))
	})
	request := httptest.NewRequest(http.MethodGet, "/failure", nil)
	request.Header.Set("X-Request-ID", "failure-correlation")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "storage unavailable")
	assert.Contains(t, output.String(), "storage unavailable")
	assert.Contains(t, output.String(), "failure-correlation")
}

func TestVeryLargePageDoesNotOverflowOffset(t *testing.T) {
	server, _ := newTestServer(t)
	recorder := server.do(t, http.MethodGet, "/api/product/list?page=9223372036854775807&pageSize=50", "", nil)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 0, decode(t, recorder).Code)
}
