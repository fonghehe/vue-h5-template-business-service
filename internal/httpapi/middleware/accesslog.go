package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
)

// AccessLog emits one structured line per request. It runs after the handler so
// that status, latency and error information are known.
func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()
		if c.Writer.Status() == 0 {
			return
		}

		attrs := []any{
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"durationMs", time.Since(startedAt).Milliseconds(),
			"requestId", c.GetString(response.ContextKeyRequestID),
			"clientIp", c.ClientIP(),
			"userAgent", c.Request.UserAgent(),
		}
		if userID := c.GetUint(response.ContextKeyUserID); userID != 0 {
			attrs = append(attrs, "userId", userID)
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "error", c.Errors.String())
		}

		level := slog.LevelInfo
		switch {
		case c.Writer.Status() >= http.StatusInternalServerError:
			level = slog.LevelError
		case c.Writer.Status() >= http.StatusBadRequest:
			level = slog.LevelWarn
		}
		logger.Log(c.Request.Context(), level, "http request", attrs...)
	}
}

// Recovery converts a panic into a JSON envelope instead of a raw stack trace,
// and logs the stack so the incident is still debuggable.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				// A cancelled request is a client disconnect, not a crash; do not
				// page anyone for it.
				if err, ok := recovered.(error); ok && err == http.ErrAbortHandler {
					c.Abort()
					return
				}
				logger.Error("panic recovered",
					"panic", recovered,
					"requestId", c.GetString(response.ContextKeyRequestID),
					"path", c.Request.URL.Path,
					"stack", string(debug.Stack()),
				)
				response.Fail(c, apierr.Internal("Internal server error"))
				c.Abort()
			}
		}()
		c.Next()
	}
}
