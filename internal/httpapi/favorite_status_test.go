package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFavoriteStatusIsUserScopedAndVersioned(t *testing.T) {
	server, _ := newTestServer(t)
	token := loginAs(t, server, "user")
	other := loginAs(t, server, "admin")
	server.do(t, http.MethodPost, "/api/product/favorite", `{"productId":1,"favorite":true}`, bearer(token))
	for _, prefix := range []string{"/api", "/api/v1"} {
		for _, tc := range []struct {
			token string
			want  bool
		}{{token, true}, {other, false}} {
			got := server.do(t, http.MethodGet, prefix+"/product/favorite?id=1", "", bearer(tc.token))
			require.Equal(t, http.StatusOK, got.Code)
			var result service.FavoriteResult
			require.NoError(t, json.Unmarshal(decode(t, got).Data, &result))
			assert.Equal(t, uint(1), result.ProductID)
			assert.Equal(t, tc.want, result.Favorite)
		}
	}
	server.do(t, http.MethodPost, "/api/product/favorite", `{"productId":1,"favorite":false}`, bearer(token))
	got := server.do(t, http.MethodGet, "/api/product/favorite?id=1", "", bearer(token))
	var result service.FavoriteResult
	require.NoError(t, json.Unmarshal(decode(t, got).Data, &result))
	assert.False(t, result.Favorite)
}

func TestFavoriteStatusRejectsInvalidAndHiddenProducts(t *testing.T) {
	server, db := newTestServer(t)
	token := loginAs(t, server, "user")
	require.Equal(t, http.StatusUnauthorized, server.do(t, http.MethodGet, "/api/product/favorite?id=1", "", nil).Code)
	for _, id := range []string{"", "0", "-1", "abc"} {
		assert.Equal(t, http.StatusUnprocessableEntity, server.do(t, http.MethodGet, "/api/product/favorite?id="+id, "", bearer(token)).Code)
	}
	assert.Equal(t, http.StatusNotFound, server.do(t, http.MethodGet, "/api/product/favorite?id=999999", "", bearer(token)).Code)
	require.NoError(t, db.Model(&model.Product{}).Where("id = ?", 1).Update("status", model.StatusDraft).Error)
	assert.Equal(t, http.StatusNotFound, server.do(t, http.MethodGet, "/api/product/favorite?id=1", "", bearer(token)).Code)
}

func TestFavoriteRequiresExplicitBooleanState(t *testing.T) {
	server, _ := newTestServer(t)
	token := loginAs(t, server, "user")
	for _, body := range []string{`{"productId":1}`, `{"productId":1,"favorite":null}`, `{"productId":1,"favorite":"false"}`} {
		assert.Equal(t, http.StatusUnprocessableEntity, server.do(t, http.MethodPost, "/api/product/favorite", body, bearer(token)).Code)
	}
	assert.Equal(t, http.StatusOK, server.do(t, http.MethodPost, "/api/product/favorite", `{"productId":1,"favorite":false}`, bearer(token)).Code)
}
