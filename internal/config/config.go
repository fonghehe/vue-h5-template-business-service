// Package config loads and validates the service configuration from the
// process environment. The server loads the optional project .env file with
// LoadDotEnv before calling Load; existing process variables take precedence.
//
// Configuration is fail-fast: an invalid value aborts startup rather than
// silently falling back to a default, because a misconfigured service that
// boots is far more dangerous than one that crashes on launch.
package config

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultJWTSecret is the development placeholder. Startup refuses to run with
// it in a non-development environment.
// #nosec G101 -- development-only placeholder, rejected at startup outside development.
const DefaultJWTSecret = "dev-only-change-me-before-production"

// DefaultMockPaymentWebhookSecret signs local mock-provider callbacks. Like
// the JWT placeholder it is rejected in production.
// #nosec G101 -- development-only placeholder, rejected at startup in production.
const DefaultMockPaymentWebhookSecret = "dev-mock-payment-webhook-secret-change-me"

// Valid APP_ENV values.
const (
	EnvLocal       = "local"
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// Config holds every runtime setting of the service.
type Config struct {
	parseError error
	// AppEnv selects local, development, test, staging or production safeguards.
	AppEnv string
	// Port is the HTTP listen port.
	Port string
	// LogLevel is one of debug, info, warn, error.
	LogLevel string
	// LogFormat is json (production) or text (local development).
	LogFormat string

	// DatabaseDriver is postgres at runtime; sqlite is for isolated tests.
	DatabaseDriver string
	// DatabaseURL is the driver-specific DSN.
	DatabaseURL string
	// DBMaxOpenConns caps the number of open database connections.
	DBMaxOpenConns int
	// DBMaxIdleConns caps idle connections kept in the pool.
	DBMaxIdleConns int
	// DBConnMaxLifetime is how long a connection may be reused.
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration
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

	// OrderPaymentTTL is how long inventory and coupons remain reserved while
	// an order waits for payment.
	OrderPaymentTTL time.Duration
	// OrderExpirationInterval controls the background worker cadence.
	OrderExpirationInterval time.Duration
	// OrderExpirationBatchSize bounds each SKIP LOCKED transaction.
	OrderExpirationBatchSize int

	// MockPaymentWebhookSecret authenticates callbacks from the mock provider.
	MockPaymentWebhookSecret string

	// RedisURL enables the optional product-detail cache. An empty value
	// disables caching; PostgreSQL remains authoritative in either mode.
	RedisURL        string
	ProductCacheTTL time.Duration

	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration
	// ReadTimeout / WriteTimeout / IdleTimeout bound the HTTP server.
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	RequestTimeout time.Duration
}

// Load reads configuration from the environment.
func Load() Config {
	production := productionLike(env("APP_ENV", EnvDevelopment))
	var parseError error
	cfg := Config{
		AppEnv:    env("APP_ENV", EnvDevelopment),
		Port:      env("PORT", "8002"),
		LogLevel:  env("LOG_LEVEL", "info"),
		LogFormat: env("LOG_FORMAT", defaultLogFormat(production)),

		DatabaseDriver:    env("DATABASE_DRIVER", "postgres"),
		DatabaseURL:       env("DATABASE_URL", "postgres://vue_h5:vue_h5_local@localhost:5432/vue_h5_business?sslmode=disable"),
		DBMaxOpenConns:    envValue(&parseError, "DB_MAX_OPEN_CONNS", 25, strconv.Atoi),
		DBMaxIdleConns:    envValue(&parseError, "DB_MAX_IDLE_CONNS", 5, strconv.Atoi),
		DBConnMaxLifetime: envValue(&parseError, "DB_CONN_MAX_LIFETIME", 30*time.Minute, time.ParseDuration),
		DBConnMaxIdleTime: envValue(&parseError, "DB_CONN_MAX_IDLE_TIME", 5*time.Minute, time.ParseDuration),
		AutoMigrate:       envValue(&parseError, "AUTO_MIGRATE", !production, strconv.ParseBool),
		Seed:              envValue(&parseError, "SEED", !production, strconv.ParseBool),

		JWTSecret:           env("JWT_SECRET", DefaultJWTSecret),
		JWTIssuer:           env("JWT_ISSUER", "vue-h5-template"),
		JWTAudience:         env("JWT_AUDIENCE", "vue-h5-template-api"),
		AccessTokenTTL:      envValue(&parseError, "ACCESS_TOKEN_TTL", 2*time.Hour, time.ParseDuration),
		RefreshTokenTTL:     envValue(&parseError, "REFRESH_TOKEN_TTL", 14*24*time.Hour, time.ParseDuration),
		RefreshCookieName:   env("REFRESH_COOKIE_NAME", "vh5_refresh"),
		RefreshCookieSecure: envValue(&parseError, "REFRESH_COOKIE_SECURE", production, strconv.ParseBool),

		CORSOrigins: parseList(env("CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),

		RateLimitEnabled: envValue(&parseError, "RATE_LIMIT_ENABLED", true, strconv.ParseBool),
		RateLimitRPS:     envValue(&parseError, "RATE_LIMIT_RPS", 20, parseFloat),
		RateLimitBurst:   envValue(&parseError, "RATE_LIMIT_BURST", 40, strconv.Atoi),

		OrderPaymentTTL:          envValue(&parseError, "ORDER_PAYMENT_TTL", 15*time.Minute, time.ParseDuration),
		OrderExpirationInterval:  envValue(&parseError, "ORDER_EXPIRATION_INTERVAL", 30*time.Second, time.ParseDuration),
		OrderExpirationBatchSize: envValue(&parseError, "ORDER_EXPIRATION_BATCH_SIZE", 100, strconv.Atoi),
		MockPaymentWebhookSecret: env("MOCK_PAYMENT_WEBHOOK_SECRET", DefaultMockPaymentWebhookSecret),

		RedisURL:        strings.TrimSpace(env("REDIS_URL", "")),
		ProductCacheTTL: envValue(&parseError, "PRODUCT_CACHE_TTL", 5*time.Minute, time.ParseDuration),

		ShutdownTimeout: envValue(&parseError, "SHUTDOWN_TIMEOUT", 15*time.Second, time.ParseDuration),
		ReadTimeout:     envValue(&parseError, "READ_TIMEOUT", 15*time.Second, time.ParseDuration),
		WriteTimeout:    envValue(&parseError, "WRITE_TIMEOUT", 30*time.Second, time.ParseDuration),
		IdleTimeout:     envValue(&parseError, "IDLE_TIMEOUT", 60*time.Second, time.ParseDuration),
		RequestTimeout:  envValue(&parseError, "REQUEST_TIMEOUT", 10*time.Second, time.ParseDuration),
	}
	cfg.parseError = parseError
	return cfg
}

// Validate fails fast on any configuration that would be unsafe at runtime.
func (c Config) Validate() error {
	if c.parseError != nil {
		return c.parseError
	}
	switch c.AppEnv {
	case EnvLocal, EnvDevelopment, EnvTest, EnvStaging, EnvProduction:
	default:
		return fmt.Errorf("APP_ENV must be local, development, test, staging or production, got %q", c.AppEnv)
	}
	if c.DatabaseDriver != "sqlite" && c.DatabaseDriver != "postgres" {
		return fmt.Errorf("DATABASE_DRIVER must be sqlite or postgres, got %q", c.DatabaseDriver)
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 0 || port > 65535 || (port == 0 && c.AppEnv != EnvTest) {
		return fmt.Errorf("PORT must be between 1 and 65535 (0 is allowed in tests)")
	}
	if c.DatabaseDriver == "sqlite" && c.AppEnv != EnvTest {
		return fmt.Errorf("DATABASE_DRIVER must be postgres outside isolated tests")
	}
	if c.DBMaxOpenConns <= 0 || c.DBMaxIdleConns < 0 || c.DBMaxIdleConns > c.DBMaxOpenConns {
		return fmt.Errorf("DB_MAX_OPEN_CONNS must be positive; DB_MAX_IDLE_CONNS must be between 0 and DB_MAX_OPEN_CONNS")
	}
	if c.DBConnMaxLifetime < 0 {
		return fmt.Errorf("DB_CONN_MAX_LIFETIME must not be negative")
	}
	if c.DBConnMaxIdleTime < 0 {
		return fmt.Errorf("DB_CONN_MAX_IDLE_TIME must not be negative")
	}
	for name, timeout := range map[string]time.Duration{
		"SHUTDOWN_TIMEOUT": c.ShutdownTimeout, "READ_TIMEOUT": c.ReadTimeout,
		"WRITE_TIMEOUT": c.WriteTimeout, "IDLE_TIMEOUT": c.IdleTimeout,
		"REQUEST_TIMEOUT": c.RequestTimeout,
	} {
		if timeout <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	if c.RequestTimeout >= c.WriteTimeout {
		return fmt.Errorf("REQUEST_TIMEOUT must be shorter than WRITE_TIMEOUT")
	}
	if strings.TrimSpace(c.JWTIssuer) == "" || strings.TrimSpace(c.JWTAudience) == "" {
		return fmt.Errorf("JWT_ISSUER and JWT_AUDIENCE are required")
	}
	if c.RefreshCookieName == "" || strings.ContainsAny(c.RefreshCookieName, " \t\r\n;=,") {
		return fmt.Errorf("REFRESH_COOKIE_NAME must be a valid cookie name")
	}
	for _, origin := range c.CORSOrigins {
		if origin == "*" {
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("CORS_ORIGINS must contain HTTP(S) origins without credentials, paths, queries or fragments")
		}
	}
	if c.RedisURL != "" {
		u, err := url.Parse(c.RedisURL)
		if err != nil || (u.Scheme != "redis" && u.Scheme != "rediss") || u.Hostname() == "" {
			return fmt.Errorf("REDIS_URL must be a valid redis:// or rediss:// URL")
		}
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be debug, info, warn or error")
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		return fmt.Errorf("LOG_FORMAT must be text or json")
	}
	if math.IsNaN(c.RateLimitRPS) || math.IsInf(c.RateLimitRPS, 0) {
		return fmt.Errorf("RATE_LIMIT_RPS must be finite")
	}

	// JWT hygiene. A weak or default secret lets anyone mint tokens that both
	// this service and the AI service would accept.
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must contain at least 32 characters")
	}
	if productionLike(c.AppEnv) && c.JWTSecret == DefaultJWTSecret {
		return fmt.Errorf("JWT_SECRET must be replaced with a random value in production")
	}
	if len(c.MockPaymentWebhookSecret) < 32 {
		return fmt.Errorf("MOCK_PAYMENT_WEBHOOK_SECRET must contain at least 32 characters")
	}
	if productionLike(c.AppEnv) && c.MockPaymentWebhookSecret == DefaultMockPaymentWebhookSecret {
		return fmt.Errorf("MOCK_PAYMENT_WEBHOOK_SECRET must be replaced in production")
	}
	if c.AccessTokenTTL <= 0 || c.RefreshTokenTTL <= 0 {
		return fmt.Errorf("token TTLs must be positive")
	}
	if c.RefreshTokenTTL <= c.AccessTokenTTL {
		return fmt.Errorf("REFRESH_TOKEN_TTL must be greater than ACCESS_TOKEN_TTL")
	}

	// TLS and CORS hygiene in production.
	if productionLike(c.AppEnv) {
		if c.Seed {
			return fmt.Errorf("SEED must be false in production to prevent demo credentials and coupons")
		}
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
	if c.OrderPaymentTTL <= 0 {
		return fmt.Errorf("ORDER_PAYMENT_TTL must be positive")
	}
	if c.OrderExpirationInterval <= 0 || c.OrderExpirationBatchSize <= 0 || c.OrderExpirationBatchSize > 1000 {
		return fmt.Errorf("order expiration interval and batch size must be positive, and batch size must not exceed 1000")
	}
	if c.ProductCacheTTL <= 0 {
		return fmt.Errorf("PRODUCT_CACHE_TTL must be positive")
	}
	return nil
}

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
		u, err := url.Parse(origin)
		if err != nil {
			continue
		}
		host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
		ip := net.ParseIP(host)
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || (ip != nil && (ip.IsLoopback() || ip.IsUnspecified())) {
			return origin
		}
	}
	return ""
}

func productionLike(environment string) bool {
	return environment == EnvProduction || environment == EnvStaging
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
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// Defaults apply only to absent settings. Preserve the first parse failure so
// Validate rejects the configuration before any database or worker is opened.
func envValue[T any](failure *error, key string, fallback T, parse func(string) (T, error)) T {
	raw, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	value, err := parse(raw)
	if err != nil {
		if *failure == nil {
			*failure = fmt.Errorf("%s has an invalid value", key)
		}
		return fallback
	}
	return value
}

func parseFloat(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}
