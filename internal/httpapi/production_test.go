package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/auth"
	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAuth struct {
	AuthAPI
	calls int
}

func (m *mockAuth) Login(ctx context.Context, username, password string, _ bool) (service.LoginResult, auth.TokenPair, error) {
	m.calls++
	requireID := ctx.Value(testRequestKey{})
	if username != "user" || password != "password" || requireID != "present" {
		return service.LoginResult{}, auth.TokenPair{}, apierr.Internal("invalid mock invocation")
	}
	return service.LoginResult{}, auth.TokenPair{}, apierr.Unauthorized("username or password is incorrect")
}

type testRequestKey struct{}

func TestLoginHandlerCanReplaceService(t *testing.T) {
	cfg := config.Load()
	fake := &mockAuth{}
	server := &Server{config: cfg, auth: fake}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"user","password":"password"}`)).WithContext(context.WithValue(context.Background(), testRequestKey{}, "present"))
	server.login(c)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, 1, fake.calls)
}

func TestCurrentAccountStatusAndRolesOverrideOldJWT(t *testing.T) {
	server, db := newTestServer(t)
	token := loginAs(t, server, "admin")
	require.NoError(t, db.Model(&model.User{}).Where("username = ?", "admin").Update("roles", `["user"]`).Error)
	response := server.do(t, http.MethodGet, "/api/admin/products", "", bearer(token))
	assert.Equal(t, http.StatusForbidden, response.Code)
	require.NoError(t, db.Model(&model.User{}).Where("username = ?", "admin").Update("status", "disabled").Error)
	response = server.do(t, http.MethodGet, "/api/user/info", "", bearer(token))
	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestHTTPRefreshReplayAndLogoutRevocation(t *testing.T) {
	server, _ := newTestServer(t)
	login := server.do(t, http.MethodPost, "/api/auth/login", `{"username":"user","password":"123456"}`, nil)
	require.Equal(t, http.StatusOK, login.Code)
	first := map[string]string{"Cookie": cookieValue(login.Header().Get("Set-Cookie"))}
	rotated := server.do(t, http.MethodPost, "/api/auth/refresh", "", first)
	require.Equal(t, http.StatusOK, rotated.Code)
	assert.Equal(t, http.StatusUnauthorized, server.do(t, http.MethodPost, "/api/auth/refresh", "", first).Code)
	second := map[string]string{"Cookie": cookieValue(rotated.Header().Get("Set-Cookie"))}
	require.Equal(t, http.StatusOK, server.do(t, http.MethodPost, "/api/auth/logout", "", second).Code)
	assert.Equal(t, http.StatusUnauthorized, server.do(t, http.MethodPost, "/api/auth/refresh", "", second).Code)
}

func TestVersionedRoutesShareLoginRateLimit(t *testing.T) {
	server, _ := newTestServer(t)
	server.config.RateLimitEnabled = true
	router := server.Router()
	for i := 0; i < 6; i++ {
		path := "/api/auth/login"
		if i%2 == 1 {
			path = "/api/v1/auth/login"
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if i < 5 {
			assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
		}
	}
}

func TestStrictJSONAndChunkedBodyLimits(t *testing.T) {
	server, _ := newTestServer(t)
	for _, body := range []string{`{"username":"user","password":"123456"} {}`, `{"refreshToken":"x"`} {
		response := server.do(t, http.MethodPost, "/api/auth/refresh", body, nil)
		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"user","password":"123456"}`+strings.Repeat(" ", 1<<20)))
	request.ContentLength = -1
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
}
