package httpapi

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
)

//go:embed openapi.yaml
var openAPISpec []byte

// registerSwagger exposes local API documentation only outside production.
// gin-swagger owns the page and serves the resources from swaggo/files.
func registerSwagger(router *gin.Engine, appEnv string) {
	if appEnv == config.EnvProduction || appEnv == config.EnvStaging {
		return
	}
	router.GET("/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", openAPISpec)
	})
	// WrapHandler assigns Prefix on first use. Keep that mutable field local to
	// this router so independently constructed servers do not share it.
	files := *swaggerfiles.Handler
	router.GET("/swagger/*any", ginSwagger.WrapHandler(&files, ginSwagger.URL("/openapi.yaml")))
}
