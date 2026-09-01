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
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, apierr.Validation("username and password are required"))
		return
	}

	result, pair, err := s.services.Auth.Login(c.Request.Context(), input.Username, input.Password, true)
	if err != nil {
		response.Fail(c, err)
		return
	}

	middleware.SetRefreshCookie(c, s.config, pair.RefreshToken, int(s.config.RefreshTokenTTL.Seconds()))
	response.OK(c, result)
}

// refresh rotates an expired access token using the HttpOnly refresh cookie, or
// a refresh token supplied in the body for non-browser clients.
//
// POST /api/auth/refresh
func (s *Server) refresh(c *gin.Context) {
	var body refreshRequest
	// The body is optional: browsers rely on the cookie alone.
	_ = c.ShouldBindJSON(&body)

	token := body.RefreshToken
	if token == "" {
		cookie, err := c.Cookie(s.config.RefreshCookieName)
		if err != nil || cookie == "" {
			response.Fail(c, apierr.Unauthorized("refresh token is required"))
			return
		}
		token = cookie
	}

	result, pair, err := s.services.Auth.Refresh(c.Request.Context(), token)
	if err != nil {
		middleware.ClearRefreshCookie(c, s.config)
		response.Fail(c, err)
		return
	}

	middleware.SetRefreshCookie(c, s.config, pair.RefreshToken, int(s.config.RefreshTokenTTL.Seconds()))
	response.OK(c, result)
}

// logout clears the refresh cookie. Access tokens are stateless, so revocation
// happens by dropping them client side.
//
// POST /api/auth/logout
func (s *Server) logout(c *gin.Context) {
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
	response.OK(c, user)
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
	response.OK(c, result)
}
