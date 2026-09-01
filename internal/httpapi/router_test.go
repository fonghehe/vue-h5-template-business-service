package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
	"github.com/fonghehe/vue-h5-template-business-service/internal/logging"
)

// testSecret is long enough to satisfy config validation. Tests never talk to a
// real deployment, so a fixed value keeps JWT assertions deterministic.
const testSecret = "integration-test-secret-that-is-long-enough"

// newTestServer builds a server backed by a temporary SQLite database seeded
// with the demo catalogue. Each test gets its own database file, so tests are
// independent and safe to run in parallel.
func newTestServer(t *testing.T) (*Server, *gorm.DB) {
	t.Helper()

	cfg := config.Load()
	cfg.AppEnv = config.EnvTest
	cfg.Port = "0"
	cfg.DatabaseDriver = "sqlite"
	cfg.DatabaseURL = filepath.Join(t.TempDir(), "test.db")
	cfg.JWTSecret = testSecret
	cfg.JWTIssuer = "vue-h5-template"
	cfg.JWTAudience = "vue-h5-template-api"
	cfg.LogLevel = "error"
	cfg.LogFormat = "text"
	// Rate limiting is exercised separately; leaving it on would make these
	// tests order-dependent because they share a client IP.
	cfg.RateLimitEnabled = false
	require.NoError(t, cfg.Validate(), "test configuration must be valid")

	db, err := database.Open(database.Options{
		Driver:       cfg.DatabaseDriver,
		DSN:          cfg.DatabaseURL,
		MaxOpenConns: 1,
		MaxIdleConns: 1,
		AutoMigrate:  true,
		Seed:         true,
		Quiet:        true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return New(cfg, db, logging.New("error", "text")), db
}

// do performs a request against a fresh router.
func (s *Server) do(t *testing.T, method, path string, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	s.Router().ServeHTTP(recorder, request)
	return recorder
}

// envelope is the decoded response body shared by every endpoint.
type envelope struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	Error     json.RawMessage `json:"error"`
	RequestID string          `json:"requestId"`
}

// decode parses a response body into the envelope.
func decode(t *testing.T, recorder *httptest.ResponseRecorder) envelope {
	t.Helper()
	var decoded envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &decoded),
		"response must be valid JSON, got: %s", recorder.Body.String())
	return decoded
}

// loginAs authenticates the seeded demo account and returns the access token.
func loginAs(t *testing.T, s *Server, username string) string {
	t.Helper()
	recorder := s.do(t, http.MethodPost, "/api/auth/login",
		`{"username":"`+username+`","password":"123456"}`, nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	body := decode(t, recorder)
	require.Equal(t, 0, body.Code, "login must succeed")

	var payload struct {
		AccessToken string `json:"accessToken"`
	}
	require.NoError(t, json.Unmarshal(body.Data, &payload))
	require.NotEmpty(t, payload.AccessToken)
	return payload.AccessToken
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func TestHealthReportsServiceWithoutTouchingDatabase(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodGet, "/health", "", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decode(t, recorder)
	assert.Equal(t, 0, body.Code)
	assert.Equal(t, "ok", body.Message)
	assert.Contains(t, string(body.Data), `"service":"business"`)
}

func TestReadyReportsDatabaseConnectivity(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodGet, "/ready", "", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, string(decode(t, recorder).Data), `"database":"up"`)
}

func TestResponseCarriesCorrelationID(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodGet, "/health", "", map[string]string{
		"X-Request-ID": "trace-abc-123",
	})

	assert.Equal(t, "trace-abc-123", recorder.Header().Get("X-Request-ID"))
	assert.Equal(t, "trace-abc-123", decode(t, recorder).RequestID)
}

func TestLoginReturnsUserAndAccessToken(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodPost, "/api/auth/login",
		`{"username":"user","password":"123456"}`, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decode(t, recorder)
	require.Equal(t, 0, body.Code)

	var user struct {
		ID          uint     `json:"id"`
		Username    string   `json:"username"`
		RealName    string   `json:"realName"`
		Avatar      string   `json:"avatar"`
		Roles       []string `json:"roles"`
		AccessToken string   `json:"accessToken"`
		ExpiresIn   int64    `json:"expiresIn"`
	}
	require.NoError(t, json.Unmarshal(body.Data, &user))
	assert.Equal(t, "user", user.Username)
	assert.NotEmpty(t, user.RealName)
	assert.Equal(t, []string{"user"}, user.Roles)
	assert.NotEmpty(t, user.AccessToken)
	assert.Positive(t, user.ExpiresIn)

	// The refresh token must never be readable from JavaScript.
	assert.NotEmpty(t, recorder.Header().Get("Set-Cookie"))
	assert.Contains(t, recorder.Header().Get("Set-Cookie"), "HttpOnly")
}

func TestLoginRejectsBadCredentialsWithoutRevealingWhichFieldFailed(t *testing.T) {
	server, _ := newTestServer(t)

	wrongPassword := server.do(t, http.MethodPost, "/api/auth/login",
		`{"username":"user","password":"nope"}`, nil)
	unknownUser := server.do(t, http.MethodPost, "/api/auth/login",
		`{"username":"ghost","password":"nope"}`, nil)

	require.Equal(t, http.StatusUnauthorized, wrongPassword.Code)
	require.Equal(t, http.StatusUnauthorized, unknownUser.Code)

	// Identical code and message for both cases prevents account enumeration.
	assert.Equal(t, decode(t, wrongPassword).Message, decode(t, unknownUser).Message)
	assert.Equal(t, 4010, decode(t, wrongPassword).Code)
}

func TestLoginRejectsMissingCredentials(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodPost, "/api/auth/login", `{}`, nil)

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Equal(t, 4001, decode(t, recorder).Code)
}

func TestUserRequiresAuthentication(t *testing.T) {
	server, _ := newTestServer(t)

	missing := server.do(t, http.MethodGet, "/api/user/info", "", nil)
	invalid := server.do(t, http.MethodGet, "/api/user/info", "", bearer("not-a-token"))

	require.Equal(t, http.StatusUnauthorized, missing.Code)
	require.Equal(t, http.StatusUnauthorized, invalid.Code)
	assert.Equal(t, 4010, decode(t, missing).Code)
}

func TestUserInfoReturnsAuthenticatedAccount(t *testing.T) {
	server, _ := newTestServer(t)
	token := loginAs(t, server, "user")

	recorder := server.do(t, http.MethodGet, "/api/user/info", "", bearer(token))

	require.Equal(t, http.StatusOK, recorder.Code)
	var user struct {
		Username string   `json:"username"`
		RealName string   `json:"realName"`
		Roles    []string `json:"roles"`
	}
	require.NoError(t, json.Unmarshal(decode(t, recorder).Data, &user))
	assert.Equal(t, "user", user.Username)
	assert.Equal(t, []string{"user"}, user.Roles)
	// Password hashes must never reach the client.
	assert.NotContains(t, recorder.Body.String(), "PasswordHash")
	assert.NotContains(t, recorder.Body.String(), "$2a$")
}

func TestProductListReturnsPaginatedEnvelope(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodGet, "/api/product/list?page=1&pageSize=5", "", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	var page struct {
		Items []struct {
			ID       uint   `json:"id"`
			Title    string `json:"title"`
			ImgURL   string `json:"imgUrl"`
			Price    string `json:"price"`
			VipPrice string `json:"vipPrice"`
			ShopDesc string `json:"shopDesc"`
			Delivery string `json:"delivery"`
			ShopName string `json:"shopName"`
		} `json:"items"`
		Total    int  `json:"total"`
		Page     int  `json:"page"`
		PageSize int  `json:"pageSize"`
		HasMore  bool `json:"hasMore"`
	}
	require.NoError(t, json.Unmarshal(decode(t, recorder).Data, &page))

	assert.Len(t, page.Items, 5)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 5, page.PageSize)
	assert.Greater(t, page.Total, 5)
	assert.True(t, page.HasMore)
	for _, item := range page.Items {
		assert.NotEmpty(t, item.Title)
		assert.NotEmpty(t, item.ImgURL)
		assert.NotEmpty(t, item.Price)
	}
}

func TestProductListHidesNonPublicItems(t *testing.T) {
	server, _ := newTestServer(t)

	// The seeded catalogue contains exactly one sold-out product.
	recorder := server.do(t, http.MethodGet, "/api/product/list?pageSize=50", "", nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	var page struct {
		Items []struct {
			ID    uint   `json:"id"`
			Title string `json:"title"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(decode(t, recorder).Data, &page))
	assert.NotContains(t, recorder.Body.String(), "现货礼盒装")
}

func TestProductListSupportsKeywordSearch(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodGet, "/api/product/list?keyword=iPhone", "", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	var page struct {
		Items []struct {
			Title string `json:"title"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(decode(t, recorder).Data, &page))
	require.NotEmpty(t, page.Items)
	for _, item := range page.Items {
		assert.Contains(t, strings.ToLower(item.Title), "iphone")
	}
}

func TestProductDetailRequiresValidId(t *testing.T) {
	server, _ := newTestServer(t)

	missing := server.do(t, http.MethodGet, "/api/product/detail", "", nil)
	notFound := server.do(t, http.MethodGet, "/api/product/detail?id=999999", "", nil)

	require.Equal(t, http.StatusUnprocessableEntity, missing.Code)
	require.Equal(t, http.StatusNotFound, notFound.Code)
	assert.Equal(t, 4040, decode(t, notFound).Code)
}

func TestProductDetailReturnsProduct(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodGet, "/api/product/detail?id=1", "", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	var product struct {
		ID          uint   `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	require.NoError(t, json.Unmarshal(decode(t, recorder).Data, &product))
	assert.Equal(t, uint(1), product.ID)
	assert.NotEmpty(t, product.Title)
	assert.NotEmpty(t, product.Description)
}

func TestFavoriteRequiresAuthentication(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodPost, "/api/product/favorite",
		`{"productId":1,"favorite":true}`, nil)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestFavoriteToggleIsIdempotent(t *testing.T) {
	server, _ := newTestServer(t)
	token := loginAs(t, server, "user")

	added := server.do(t, http.MethodPost, "/api/product/favorite",
		`{"productId":1,"favorite":true}`, bearer(token))
	require.Equal(t, http.StatusOK, added.Code)

	// Repeating the same request must not raise a duplicate key error.
	again := server.do(t, http.MethodPost, "/api/product/favorite",
		`{"productId":1,"favorite":true}`, bearer(token))
	require.Equal(t, http.StatusOK, again.Code)

	var result struct {
		ProductID uint `json:"productId"`
		Favorite  bool `json:"favorite"`
	}
	require.NoError(t, json.Unmarshal(decode(t, again).Data, &result))
	assert.Equal(t, uint(1), result.ProductID)
	assert.True(t, result.Favorite)

	removed := server.do(t, http.MethodPost, "/api/product/favorite",
		`{"productId":1,"favorite":false}`, bearer(token))
	require.Equal(t, http.StatusOK, removed.Code)
	require.NoError(t, json.Unmarshal(decode(t, removed).Data, &result))
	assert.False(t, result.Favorite)
}

func TestFavoriteRejectsUnknownProduct(t *testing.T) {
	server, _ := newTestServer(t)
	token := loginAs(t, server, "user")

	recorder := server.do(t, http.MethodPost, "/api/product/favorite",
		`{"productId":987654,"favorite":true}`, bearer(token))

	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestFavoritesListReturnsBookmarkedProducts(t *testing.T) {
	server, _ := newTestServer(t)
	token := loginAs(t, server, "user")

	server.do(t, http.MethodPost, "/api/product/favorite", `{"productId":1,"favorite":true}`, bearer(token))
	server.do(t, http.MethodPost, "/api/product/favorite", `{"productId":3,"favorite":true}`, bearer(token))

	recorder := server.do(t, http.MethodGet, "/api/user/favorites?pageSize=10", "", bearer(token))

	require.Equal(t, http.StatusOK, recorder.Code)
	var page struct {
		Items []struct {
			ID uint `json:"id"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(decode(t, recorder).Data, &page))
	assert.Equal(t, 2, page.Total)
	assert.Len(t, page.Items, 2)
}

func TestAdminRoutesRequireAdminRole(t *testing.T) {
	server, _ := newTestServer(t)
	userToken := loginAs(t, server, "user")
	adminToken := loginAs(t, server, "admin")

	forbidden := server.do(t, http.MethodGet, "/api/admin/products", "", bearer(userToken))
	allowed := server.do(t, http.MethodGet, "/api/admin/products", "", bearer(adminToken))

	require.Equal(t, http.StatusForbidden, forbidden.Code)
	assert.Equal(t, 4030, decode(t, forbidden).Code)
	require.Equal(t, http.StatusOK, allowed.Code)
}

func TestAdminCanCreateUpdateAndDeleteProduct(t *testing.T) {
	server, _ := newTestServer(t)
	token := loginAs(t, server, "admin")

	created := server.do(t, http.MethodPost, "/api/admin/products", `{
		"title":"Integration Test Product",
		"imgUrl":"https://example.com/a.png",
		"price":"19.90",
		"vipPrice":"15.90",
		"shopDesc":"自营",
		"delivery":"京东物流",
		"shopName":"Test Shop",
		"description":"created by an automated test",
		"status":"on_sale"
	}`, bearer(token))
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	var product struct {
		ID     uint   `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(decode(t, created).Data, &product))
	require.NotZero(t, product.ID)

	// PATCH must only change the fields that were sent.
	patched := server.do(t, http.MethodPatch, "/api/admin/products/"+itoa(product.ID),
		`{"price":"9.90"}`, bearer(token))
	require.Equal(t, http.StatusOK, patched.Code, patched.Body.String())

	var updated struct {
		Title    string `json:"title"`
		Price    string `json:"price"`
		ShopName string `json:"shopName"`
	}
	require.NoError(t, json.Unmarshal(decode(t, patched).Data, &updated))
	assert.Equal(t, "9.90", updated.Price)
	assert.Equal(t, "Integration Test Product", updated.Title, "untouched fields must survive a PATCH")
	assert.Equal(t, "Test Shop", updated.ShopName)

	deleted := server.do(t, http.MethodDelete, "/api/admin/products/"+itoa(product.ID), "", bearer(token))
	require.Equal(t, http.StatusOK, deleted.Code)

	gone := server.do(t, http.MethodGet, "/api/admin/products/"+itoa(product.ID), "", bearer(token))
	require.Equal(t, http.StatusNotFound, gone.Code)
}

func TestAdminCreateRejectsInvalidPrice(t *testing.T) {
	server, _ := newTestServer(t)
	token := loginAs(t, server, "admin")

	recorder := server.do(t, http.MethodPost, "/api/admin/products", `{
		"title":"Bad Price",
		"imgUrl":"https://example.com/a.png",
		"price":"not-a-number",
		"vipPrice":"1",
		"shopName":"Test Shop",
		"status":"on_sale"
	}`, bearer(token))

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Equal(t, 4001, decode(t, recorder).Code)
}

func TestUnknownRouteReturnsEnvelope(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodGet, "/api/does-not-exist", "", nil)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	// Unknown paths must still return the standard JSON envelope, never the
	// default Gin HTML/text 404, so that clients can always parse the body.
	body := decode(t, recorder)
	assert.Equal(t, 4040, body.Code)
	assert.Equal(t, "Endpoint not found", body.Message)
}

func TestCORSPreflightIsAnswered(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodOptions, "/api/product/list", "", map[string]string{
		"Origin": "http://localhost:5173",
	})

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "http://localhost:5173", recorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", recorder.Header().Get("Access-Control-Allow-Credentials"))
}

func TestRefreshRotatesAccessTokenFromCookie(t *testing.T) {
	server, _ := newTestServer(t)

	login := server.do(t, http.MethodPost, "/api/auth/login",
		`{"username":"user","password":"123456"}`, nil)
	require.Equal(t, http.StatusOK, login.Code)

	cookie := login.Header().Get("Set-Cookie")
	require.NotEmpty(t, cookie)
	refreshToken := cookieValue(cookie)
	require.NotEmpty(t, refreshToken)

	recorder := server.do(t, http.MethodPost, "/api/auth/refresh", "", map[string]string{
		"Cookie": refreshToken,
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var payload struct {
		AccessToken string `json:"accessToken"`
	}
	require.NoError(t, json.Unmarshal(decode(t, recorder).Data, &payload))
	assert.NotEmpty(t, payload.AccessToken)
}

func TestRefreshRejectsInvalidToken(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodPost, "/api/auth/refresh", "", map[string]string{
		"Cookie": "vh5_refresh=tampered.token.value",
	})

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestLogoutClearsRefreshCookie(t *testing.T) {
	server, _ := newTestServer(t)

	recorder := server.do(t, http.MethodPost, "/api/auth/logout", "", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Set-Cookie"), "vh5_refresh=;")
	assert.Contains(t, recorder.Header().Get("Set-Cookie"), "HttpOnly")
}

func TestAccessTokensAreNotAcceptedAsRefreshTokens(t *testing.T) {
	server, _ := newTestServer(t)
	token := loginAs(t, server, "user")

	recorder := server.do(t, http.MethodPost, "/api/auth/refresh",
		`{"refreshToken":"`+token+`"}`, nil)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

// itoa avoids importing strconv in every assertion.
func itoa(value uint) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

// cookieValue extracts the "name=value" pair from a Set-Cookie header.
func cookieValue(header string) string {
	for _, part := range strings.Split(header, ";") {
		if trimmed := strings.TrimSpace(part); strings.HasPrefix(trimmed, "vh5_refresh=") {
			return trimmed
		}
	}
	return ""
}
