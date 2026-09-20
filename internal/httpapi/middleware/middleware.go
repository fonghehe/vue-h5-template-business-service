// Package middleware holds the cross-cutting HTTP concerns: request identity,
// structured access logging, panic recovery, CORS, authentication and rate
// limiting. Everything here is transport-only; no business rules live in it.
package middleware

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
)

// RequestID assigns a correlation id to every request. A caller supplied
// X-Request-ID is honoured so traces can span the frontend and both services.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 {
			requestID = uuid.NewString()
		}
		c.Set(response.ContextKeyRequestID, requestID)
		c.Request = c.Request.WithContext(response.WithRequestID(c.Request.Context(), requestID))
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// CORS applies the configured origin allow-list. Credentials are supported
// because the refresh token travels in an HttpOnly cookie.
func CORS(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	allowAll := false
	for _, origin := range origins {
		if origin == "*" {
			allowAll = true
			continue
		}
		allowed[strings.ToLower(origin)] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			_, ok := allowed[strings.ToLower(origin)]
			if allowAll || ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, Idempotency-Key, X-Mock-Signature")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
				c.Header("Access-Control-Max-Age", "86400")
				c.Header("Access-Control-Expose-Headers", "X-Request-ID, Retry-After")
			} else {
				response.Fail(c, apierr.Forbidden("Origin is not allowed"))
				c.Abort()
				return
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// visitor tracks the token bucket state of one client.
type visitor struct {
	tokens float64
	last   time.Time
}

// RateLimit is a fixed-capacity in-process token bucket keyed by client IP.
//
// It protects a single instance against accidental or abusive bursts; a
// multi-instance deployment should put a shared limiter at the edge instead.
// This implementation is intentionally dependency-free and bounded: idle
// visitors are evicted so the map cannot grow without limit.
type RateLimit struct {
	mu          sync.Mutex
	limit       float64 // tokens added per second
	burst       int
	visitors    map[string]*visitor
	ttl         time.Duration
	lastSweep   time.Time
	maxVisitors int
}

// NewRateLimit builds a limiter allowing rps sustained requests with a burst
// capacity of burst.
func NewRateLimit(rps float64, burst int) *RateLimit {
	return &RateLimit{
		limit:       rps,
		burst:       burst,
		visitors:    make(map[string]*visitor),
		ttl:         10 * time.Minute,
		lastSweep:   time.Now(),
		maxVisitors: 10000,
	}
}

// Handler rejects requests that exceed the allowance with HTTP 429.
func (rl *RateLimit) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, response.Envelope{
				Code:      4290,
				Message:   "too many requests",
				Data:      nil,
				Error:     nil,
				RequestID: c.GetString(response.ContextKeyRequestID),
			})
			return
		}
		c.Next()
	}
}

func (rl *RateLimit) allow(key string) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if now.Sub(rl.lastSweep) > time.Minute {
		rl.sweep(now)
	}

	state, ok := rl.visitors[key]
	if !ok {
		if len(rl.visitors) >= rl.maxVisitors {
			return false
		}
		rl.visitors[key] = &visitor{tokens: float64(rl.burst) - 1, last: now}
		return true
	}

	// Refill proportionally to elapsed time, capped at the burst capacity.
	elapsed := now.Sub(state.last).Seconds()
	state.tokens += elapsed * rl.limit
	if state.tokens > float64(rl.burst) {
		state.tokens = float64(rl.burst)
	}
	state.last = now

	if state.tokens < 1 {
		return false
	}
	state.tokens--
	return true
}

// sweep evicts visitors that have been idle longer than the TTL.
func (rl *RateLimit) sweep(now time.Time) {
	for key, state := range rl.visitors {
		if now.Sub(state.last) > rl.ttl {
			delete(rl.visitors, key)
		}
	}
	rl.lastSweep = now
}

// SecurityHeaders sets a conservative baseline. It does not replace a reverse
// proxy, but it means a misconfigured one still leaves us with sane defaults.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

// BodyLimit rejects oversized payloads before they are parsed.
func BodyLimit() gin.HandlerFunc {
	// 1 MiB is comfortably above any payload this API accepts.
	const maxBodyBytes = 1 << 20
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBodyBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, response.Envelope{
				Code:      4130,
				Message:   "request body too large",
				Data:      nil,
				Error:     nil,
				RequestID: c.GetString(response.ContextKeyRequestID),
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
		// Reading once bounds chunked requests and trailing data too. A JSON
		// decoder alone may stop after the first object without hitting the limit.
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			var sizeError *http.MaxBytesError
			switch {
			case errors.As(err, &sizeError):
				response.Fail(c, apierr.New(apierr.CodePayloadTooLarge, "request body too large"))
			case c.Request.Context().Err() != nil:
				response.Fail(c, c.Request.Context().Err())
			default:
				response.Fail(c, apierr.Validation("request body could not be read"))
			}
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}

// Timeout cooperatively cancels database/cache work without running Gin in a
// detached goroutine. Handlers must propagate the request context.
func Timeout(duration time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), duration)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		if ctx.Err() != nil && !c.Writer.Written() {
			response.Fail(c, ctx.Err())
		}
	}
}
