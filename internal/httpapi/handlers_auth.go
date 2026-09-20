package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/httpapi/middleware"
	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
)

type loginRequest struct {
	Username string `json:"username" binding:"required,max=64"`
	Password string `json:"password" binding:"required,max=128"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// login exchanges credentials for an access token and sets the refresh cookie.
//
// POST /api/auth/login
func (s *Server) login(c *gin.Context) {
	var input loginRequest
	if err := bindJSON(c, &input, false, "Invalid request payload"); err != nil {
		response.Fail(c, apierr.Validation("username and password are required"))
		return
	}

	result, pair, err := s.auth.Login(c.Request.Context(), input.Username, input.Password, true)
	if err != nil {
		response.Fail(c, err)
		return
	}

	middleware.SetRefreshCookie(c, s.config, pair.RefreshToken, int(s.config.RefreshTokenTTL.Seconds()))
	response.OK(c, toLogin(result))
}

// refresh rotates an expired access token using the HttpOnly refresh cookie, or
// a refresh token supplied in the body for non-browser clients.
//
// POST /api/auth/refresh
func (s *Server) refresh(c *gin.Context) {
	var body refreshRequest
	// The body is optional: browsers rely on the cookie alone.
	if err := bindJSON(c, &body, true, "invalid refresh payload"); err != nil {
		response.Fail(c, err)
		return
	}

	token := body.RefreshToken
	if token == "" {
		cookie, err := c.Cookie(s.config.RefreshCookieName)
		if err != nil || cookie == "" {
			response.Fail(c, apierr.Unauthorized("refresh token is required"))
			return
		}
		token = cookie
	}

	result, pair, err := s.auth.Refresh(c.Request.Context(), token)
	if err != nil {
		middleware.ClearRefreshCookie(c, s.config)
		response.Fail(c, err)
		return
	}

	middleware.SetRefreshCookie(c, s.config, pair.RefreshToken, int(s.config.RefreshTokenTTL.Seconds()))
	response.OK(c, toLogin(result))
}

// logout revokes the refresh session and clears its cookie. Clients must also
// discard stateless access tokens, which remain valid until expiry.
//
// POST /api/auth/logout
func (s *Server) logout(c *gin.Context) {
	var body refreshRequest
	if err := bindJSON(c, &body, true, "invalid logout payload"); err != nil {
		response.Fail(c, err)
		return
	}
	token := body.RefreshToken
	if token == "" {
		token, _ = c.Cookie(s.config.RefreshCookieName)
	}
	if err := s.auth.Logout(c.Request.Context(), token); err != nil {
		response.Fail(c, err)
		return
	}

	middleware.ClearRefreshCookie(c, s.config)
	response.OK(c, nil)
}

// userInfo returns the authenticated account.
//
// GET /api/user/info
func (s *Server) userInfo(c *gin.Context) {
	user, err := s.services.Users.ByID(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toUser(user))
}

// listFavorites returns the bookmarked products of the authenticated user.
//
// GET /api/user/favorites
func (s *Server) listFavorites(c *gin.Context) {
	page, pageSize := pagination(c, 10, 50)
	result, err := s.services.Favorites.List(c.Request.Context(), middleware.CurrentUserID(c), page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, mapPage(result, toProduct))
}
