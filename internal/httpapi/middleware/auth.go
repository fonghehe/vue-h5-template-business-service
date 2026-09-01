package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/auth"
	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
)

// Authenticate validates the bearer token and stores the resulting identity in
// the request context for downstream handlers.
//
// A refresh token is deliberately not accepted here: only access tokens may
// authorise arbitrary API calls.
func Authenticate(service TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := auth.Bearer(c.GetHeader("Authorization"))
		if !ok {
			response.Fail(c, apierr.Unauthorized("Authentication required"))
			c.Abort()
			return
		}
		claims, err := service.VerifyAccessToken(token)
		if err != nil {
			response.Fail(c, err)
			c.Abort()
			return
		}
		userID, err := claims.UserID()
		if err != nil {
			response.Fail(c, apierr.Unauthorized("Invalid token subject"))
			c.Abort()
			return
		}

		c.Set(response.ContextKeyUserID, userID)
		c.Set(response.ContextKeyUsername, claims.Username)
		c.Set(response.ContextKeyRoles, claims.Roles)
		c.Next()
	}
}

// TokenVerifier is the subset of the auth service the middleware depends on.
// Depending on an interface (rather than the concrete service) keeps the
// transport layer unit-testable without a database.
type TokenVerifier interface {
	VerifyAccessToken(token string) (*auth.Claims, error)
}

// RequireRole rejects callers that do not carry the given role. It must be
// registered after Authenticate.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, ok := c.Get(response.ContextKeyRoles)
		if !ok {
			response.Fail(c, apierr.Forbidden("Insufficient permission"))
			c.Abort()
			return
		}
		granted, ok := roles.([]string)
		if !ok {
			response.Fail(c, apierr.Forbidden("Insufficient permission"))
			c.Abort()
			return
		}
		for _, candidate := range granted {
			if strings.EqualFold(candidate, role) {
				c.Next()
				return
			}
		}
		response.Fail(c, apierr.Forbidden("Insufficient permission"))
		c.Abort()
	}
}

// CurrentUserID returns the authenticated user id, or 0 when unauthenticated.
func CurrentUserID(c *gin.Context) uint {
	return c.GetUint(response.ContextKeyUserID)
}

// SetRefreshCookie writes the refresh token as an HttpOnly cookie scoped to the
// auth routes, so browser JavaScript can never read it.
func SetRefreshCookie(c *gin.Context, cfg config.Config, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(cfg.RefreshCookieName, token, maxAge, "/api/auth", "", cfg.RefreshCookieSecure, true)
}

// ClearRefreshCookie expires the refresh cookie.
func ClearRefreshCookie(c *gin.Context, cfg config.Config) {
	SetRefreshCookie(c, cfg, "", -1)
}
