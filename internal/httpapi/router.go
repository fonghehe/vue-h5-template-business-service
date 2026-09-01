// Package httpapi exposes the HTTP transport of the business service. Handlers
// in this package do three things only: bind input, call a service, write the
// envelope. Any rule that outlives HTTP belongs in internal/service.
package httpapi

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
	"github.com/fonghehe/vue-h5-template-business-service/internal/httpapi/middleware"
	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
	"github.com/fonghehe/vue-h5-template-business-service/internal/service"
)

// Server wires the service container into a Gin engine.
type Server struct {
	services *service.Container
	config   config.Config
	db       *gorm.DB
	logger   *slog.Logger
}

// New builds a server from configuration, database and logger.
func New(cfg config.Config, db *gorm.DB, logger *slog.Logger) *Server {
	return &Server{
		services: service.New(cfg, db),
		config:   cfg,
		db:       db,
		logger:   logger,
	}
}

// Router assembles the middleware chain and every route group.
func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	if err := router.SetTrustedProxies(nil); err != nil {
		// Failing to reset trusted proxies would let a spoofed X-Forwarded-For
		// bypass rate limiting, so this is not recoverable.
		panic(err)
	}

	router.Use(
		middleware.RequestID(),
		middleware.SecurityHeaders(),
		middleware.Recovery(s.logger),
		middleware.AccessLog(s.logger),
		middleware.CORS(s.config.CORSOrigins),
		middleware.BodyLimit(s.config),
	)

	// Liveness and readiness are unauthenticated: orchestrators poll them.
	router.GET("/health", s.health)
	router.GET("/ready", s.ready)

	api := router.Group("/api")
	api.GET("/health", s.health)
	if s.config.RateLimitEnabled {
		limiter := middleware.NewRateLimit(s.config.RateLimitRPS, s.config.RateLimitBurst)
		api.Use(limiter.Handler())
	}

	s.registerAuthRoutes(api)
	s.registerUserRoutes(api)
	s.registerProductRoutes(api)
	s.registerAdminRoutes(api)

	// Gin answers unknown paths with a plain text 404 by default. An API that
	// sometimes returns HTML breaks every client's JSON parser, so unknown
	// routes get the same envelope as a handled error.
	router.NoRoute(func(c *gin.Context) {
		response.Fail(c, apierr.NotFound("Endpoint not found"))
	})

	return router
}

// registerAuthRoutes mounts the public authentication endpoints.
func (s *Server) registerAuthRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	auth.POST("/login", s.login)
	auth.POST("/logout", s.logout)
	auth.POST("/refresh", s.refresh)
}

// registerUserRoutes mounts endpoints that require an access token.
func (s *Server) registerUserRoutes(api *gin.RouterGroup) {
	user := api.Group("/user")
	user.Use(middleware.Authenticate(s.services.Auth))
	user.GET("/info", s.userInfo)
	user.GET("/favorites", s.listFavorites)
}

// registerProductRoutes mounts the public catalog endpoints plus the
// authenticated favorite toggle.
func (s *Server) registerProductRoutes(api *gin.RouterGroup) {
	product := api.Group("/product")
	product.GET("/list", s.listProducts)
	product.GET("/detail", s.productDetail)

	toggle := product.Group("")
	toggle.Use(middleware.Authenticate(s.services.Auth))
	toggle.POST("/favorite", s.toggleFavorite)
}

// registerAdminRoutes mounts catalog management endpoints. These are the only
// routes that can mutate products and are gated on the admin role.
func (s *Server) registerAdminRoutes(api *gin.RouterGroup) {
	admin := api.Group("/admin")
	admin.Use(middleware.Authenticate(s.services.Auth), middleware.RequireRole("admin"))
	admin.GET("/products", s.adminListProducts)
	admin.POST("/products", s.adminCreateProduct)
	admin.GET("/products/:id", s.adminGetProduct)
	admin.PATCH("/products/:id", s.adminUpdateProduct)
	admin.DELETE("/products/:id", s.adminDeleteProduct)
}

// health is the liveness probe. It never touches the database so that a
// database outage cannot cause a restart loop.
func (s *Server) health(c *gin.Context) {
	response.OK(c, gin.H{
		"service": "business",
		"status":  "ok",
		"env":     s.config.AppEnv,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// ready is the readiness probe. It fails while the database is unreachable so
// that load balancers stop routing traffic to this instance.
func (s *Server) ready(c *gin.Context) {
	if err := database.Ping(s.db); err != nil {
		s.logger.Warn("readiness probe failed", "error", err,
			"requestId", c.GetString(response.ContextKeyRequestID))
		response.Fail(c, serviceUnavailable(err))
		return
	}
	response.OK(c, gin.H{"service": "business", "status": "ready", "database": "up"})
}
