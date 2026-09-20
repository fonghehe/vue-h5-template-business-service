package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
)

func TestSwaggerPageAndLocalAssets(t *testing.T) {
	server, _ := newTestServer(t)
	page := server.do(t, http.MethodGet, "/swagger/index.html", "", nil)
	require.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, page.Body.String(), "swagger-initializer.js")
	assert.NotContains(t, page.Body.String(), "https://", "page must not require a CDN")
	for _, asset := range []struct{ path, mime string }{
		{"/swagger/swagger-ui.css", "text/css"},
		{"/swagger/swagger-ui-bundle.js", "application/javascript"},
		{"/swagger/swagger-ui-standalone-preset.js", "application/javascript"},
	} {
		result := server.do(t, http.MethodGet, asset.path, "", nil)
		require.Equal(t, http.StatusOK, result.Code, asset.path)
		assert.Contains(t, result.Header().Get("Content-Type"), asset.mime)
		assert.Greater(t, result.Body.Len(), 1000)
	}
	initializer := server.do(t, http.MethodGet, "/swagger/swagger-initializer.js", "", nil)
	require.Equal(t, http.StatusOK, initializer.Code)
	assert.Contains(t, initializer.Body.String(), `url: "/openapi.yaml"`)
	assert.Contains(t, initializer.Body.String(), "validatorUrl: null")
}

func TestSwaggerIsUnavailableInProduction(t *testing.T) {
	server, _ := newTestServer(t)
	server.config.AppEnv = config.EnvProduction
	for _, path := range []string{"/swagger/index.html", "/openapi.yaml", "/swagger/swagger-ui.css", "/swagger/swagger-ui-bundle.js", "/swagger/swagger-initializer.js"} {
		result := server.do(t, http.MethodGet, path, "", nil)
		assert.Equal(t, http.StatusNotFound, result.Code, path)
		assert.Equal(t, 4040, decode(t, result).Code)
	}
}

func TestOpenAPIMatchesRegisteredRoutesAndResolvesReferences(t *testing.T) {
	server, _ := newTestServer(t)
	result := server.do(t, http.MethodGet, "/openapi.yaml", "", nil)
	require.Equal(t, http.StatusOK, result.Code)
	assert.Contains(t, result.Header().Get("Content-Type"), "application/yaml")
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(result.Body.Bytes(), &document))
	require.Equal(t, "3.0.3", document["openapi"])
	paths, ok := document["paths"].(map[string]any)
	require.True(t, ok)
	registered := make(map[string]bool)
	for _, route := range server.Router().Routes() {
		if route.Path == "/swagger/*any" {
			registered["get /swagger/index.html"] = true
			continue
		}
		path := strings.ReplaceAll(route.Path, ":id", "{id}")
		item, ok := paths[path].(map[string]any)
		require.True(t, ok, "undocumented route: %s %s", route.Method, route.Path)
		_, ok = item[strings.ToLower(route.Method)]
		assert.True(t, ok, "undocumented method: %s %s", route.Method, route.Path)
		registered[strings.ToLower(route.Method)+" "+path] = true
	}
	operationIDs := make(map[string]bool)
	for path, raw := range paths {
		for method, rawOperation := range raw.(map[string]any) {
			assert.True(t, registered[method+" "+path], "documented route does not exist: %s %s", method, path)
			operation := rawOperation.(map[string]any)
			id, ok := operation["operationId"].(string)
			require.True(t, ok)
			assert.False(t, operationIDs[id], "duplicate operationId: %s", id)
			operationIDs[id] = true
		}
	}
	var checkReferences func(any)
	checkReferences = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if ref, ok := value["$ref"].(string); ok {
				require.True(t, strings.HasPrefix(ref, "#/"), "only local references are expected")
				var target any = document
				for _, key := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
					object, ok := target.(map[string]any)
					require.True(t, ok, "unresolved reference: %s", ref)
					target, ok = object[key]
					require.True(t, ok, "unresolved reference: %s", ref)
				}
			}
			for _, child := range value {
				checkReferences(child)
			}
		case []any:
			for _, child := range value {
				checkReferences(child)
			}
		}
	}
	checkReferences(document)
}
