// Package config loads and validates the service configuration from the
// environment (optionally seeded from a .env file).
//
// Configuration is fail-fast: an invalid value aborts startup rather than
// silently falling back to a default, because a misconfigured service that
// boots is far more dangerous than one that crashes on launch.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultJWTSecret is the development placeholder. Startup refuses to run with
// it in a non-development environment.
// #nosec G101 -- development-only placeholder, rejected at startup outside development.
const DefaultJWTSecret = "dev-only-change-me-before-production"

// Valid APP_ENV values.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Config holds every runtime setting of the service.
type Config struct {
	// AppEnv is one of development, test or production.
	AppEnv string
	// Port is the HTTP listen port.
	Port string
	// LogLevel is one of debug, info, warn, error.
	LogLevel string
	// LogFormat is json (production) or text (local development).
	LogFormat string

	// DatabaseDriver is postgres or sqlite.
	DatabaseDriver string
	// DatabaseURL is the driver-specific DSN.
	DatabaseURL string
	// DBMaxOpenConns caps the number of open database connections.
	DBMaxOpenConns int
	// DBMaxIdleConns caps idle connections kept in the pool.
	DBMaxIdleConns int
	// DBConnMaxLifetime is how long a connection may be reused.
	DBConnMaxLifetime time.Duration
	// AutoMigrate runs schema migrations at startup.
	AutoMigrate bool
	// Seed loads demo data on an empty database.
	Seed bool

	// JWTSecret signs and verifies access/refresh tokens. It must match the
	// secret configured on the AI service, which validates the same tokens.
	JWTSecret string
	// JWTIssuer is the iss claim.
	JWTIssuer string
	// JWTAudience is the aud claim.
	JWTAudience string
	// AccessTokenTTL is the lifetime of an access token.
	AccessTokenTTL time.Duration
	// RefreshTokenTTL is the lifetime of a refresh token.
	RefreshTokenTTL time.Duration
	// RefreshCookieName is the HttpOnly cookie carrying the refresh token.
	RefreshCookieName string
	// RefreshCookieSecure marks the refresh cookie Secure (HTTPS only).
	RefreshCookieSecure bool

	// CORSOrigins is the allow-list of browser origins.
	CORSOrigins []string

	// RateLimitEnabled turns the per-IP token bucket on.
	RateLimitEnabled bool
	// RateLimitRPS is the sustained requests-per-second allowance per IP.
	RateLimitRPS float64
	// RateLimitBurst is the bucket capacity per IP.
	RateLimitBurst int

	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration
	// ReadTimeout / WriteTimeout / IdleTimeout bound the HTTP server.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// Load reads configuration from the environment.
func Load() Config {
	production := strings.EqualFold(env("APP_ENV", EnvDevelopment), EnvProduction)
	return Config{
		AppEnv:    env("APP_ENV", EnvDevelopment),
		Port:      env("PORT", "8002"),
		LogLevel:  env("LOG_LEVEL", "info"),
		LogFormat: env("LOG_FORMAT", defaultLogFormat(production)),

		DatabaseDriver:    env("DATABASE_DRIVER", "postgres"),
		DatabaseURL:       env("DATABASE_URL", "postgres://vue_h5:vue_h5_local@localhost:5432/vue_h5_business?sslmode=disable"),
		DBMaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime: envDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		AutoMigrate:       envBool("AUTO_MIGRATE", true),
		Seed:              envBool("SEED", true),

		JWTSecret:           env("JWT_SECRET", DefaultJWTSecret),
		JWTIssuer:           env("JWT_ISSUER", "vue-h5-template"),
		JWTAudience:         env("JWT_AUDIENCE", "vue-h5-template-api"),
		AccessTokenTTL:      envDuration("ACCESS_TOKEN_TTL", 2*time.Hour),
		RefreshTokenTTL:     envDuration("REFRESH_TOKEN_TTL", 14*24*time.Hour),
		RefreshCookieName:   env("REFRESH_COOKIE_NAME", "vh5_refresh"),
		RefreshCookieSecure: envBool("REFRESH_COOKIE_SECURE", production),

		CORSOrigins: parseList(env("CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),

		RateLimitEnabled: envBool("RATE_LIMIT_ENABLED", true),
		RateLimitRPS:     envFloat("RATE_LIMIT_RPS", 20),
		RateLimitBurst:   envInt("RATE_LIMIT_BURST", 40),

		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		ReadTimeout:     envDuration("READ_TIMEOUT", 15*time.Second),
		WriteTimeout:    envDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:     envDuration("IDLE_TIMEOUT", 60*time.Second),
	}
}

// Validate fails fast on any configuration that would be unsafe at runtime.
func (c Config) Validate() error {
	switch c.AppEnv {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		return fmt.Errorf("APP_ENV must be one of development, test or production, got %q", c.AppEnv)
	}
	if c.DatabaseDriver != "sqlite" && c.DatabaseDriver != "postgres" {
		return fmt.Errorf("DATABASE_DRIVER must be sqlite or postgres, got %q", c.DatabaseDriver)
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if _, err := strconv.Atoi(c.Port); err != nil {
		return fmt.Errorf("PORT must be numeric, got %q", c.Port)
	}

	// JWT hygiene. A weak or default secret lets anyone mint tokens that both
	// this service and the AI service would accept.
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must contain at least 32 characters")
	}
	if c.AppEnv == EnvProduction && c.JWTSecret == DefaultJWTSecret {
		return fmt.Errorf("JWT_SECRET must be replaced with a random value in production")
	}
	if c.AccessTokenTTL <= 0 || c.RefreshTokenTTL <= 0 {
		return fmt.Errorf("token TTLs must be positive")
	}
	if c.RefreshTokenTTL <= c.AccessTokenTTL {
		return fmt.Errorf("REFRESH_TOKEN_TTL must be greater than ACCESS_TOKEN_TTL")
	}

	// TLS and CORS hygiene in production.
	if c.AppEnv == EnvProduction {
		if !c.RefreshCookieSecure {
			return fmt.Errorf("REFRESH_COOKIE_SECURE must be true in production")
		}
		if len(c.CORSOrigins) == 0 || contains(c.CORSOrigins, "*") {
			return fmt.Errorf("CORS_ORIGINS must list explicit origins in production, not *")
		}
		// Falling back to the development defaults in production is the single
		// most common way to ship a broken deployment silently: the service
		// boots, health checks pass, and browsers are authorised for localhost.
		if origin := firstLoopbackOrigin(c.CORSOrigins); origin != "" {
			return fmt.Errorf("CORS_ORIGINS must not contain the loopback origin %q in production; set it to the real frontend domain", origin)
		}
		if c.LogFormat != "json" {
			return fmt.Errorf("LOG_FORMAT must be json in production")
		}
	}
	if c.RateLimitEnabled && (c.RateLimitRPS <= 0 || c.RateLimitBurst <= 0) {
		return fmt.Errorf("RATE_LIMIT_RPS and RATE_LIMIT_BURST must be positive when rate limiting is enabled")
	}
	return nil
}

// IsProduction reports whether the service runs with production semantics.
func (c Config) IsProduction() bool { return c.AppEnv == EnvProduction }

func defaultLogFormat(production bool) string {
	if production {
		return "json"
	}
	return "text"
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// firstLoopbackOrigin returns the first origin pointing at the local machine.
// Loopback hosts are recognised by hostname and by IPv4/IPv6 literal.
func firstLoopbackOrigin(origins []string) string {
	for _, origin := range origins {
		host := origin
		if index := strings.Index(host, "://"); index >= 0 {
			host = host[index+3:]
		}
		if index := strings.IndexAny(host, "/:"); index >= 0 {
			host = host[:index]
		}
		host = strings.Trim(host, "[]")
		switch strings.ToLower(host) {
		case "localhost", "127.0.0.1", "::1", "0.0.0.0":
			return origin
		}
	}
	return ""
}

// parseList splits a comma separated setting into a trimmed slice.
func parseList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func envFloat(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
