package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withEnv runs fn with a clean environment containing only the given entries,
// so tests never depend on the developer's shell.
func withEnv(t *testing.T, entries map[string]string, fn func()) {
	t.Helper()
	previous := os.Environ()
	os.Clearenv()
	for key, value := range entries {
		t.Setenv(key, value)
	}
	defer func() {
		os.Clearenv()
		for _, entry := range previous {
			if name, value, ok := strings.Cut(entry, "="); ok {
				_ = os.Setenv(name, value)
			}
		}
	}()
	fn()
}

// validConfig is the minimum production-safe configuration.
func validConfig() Config {
	cfg := Load()
	cfg.AppEnv = EnvProduction
	cfg.JWTSecret = strings.Repeat("s", 40)
	cfg.MockPaymentWebhookSecret = strings.Repeat("p", 40)
	cfg.LogFormat = "json"
	cfg.RefreshCookieSecure = true
	cfg.CORSOrigins = []string{"https://app.example.com"}
	cfg.Seed = false
	return cfg
}

func TestDefaultsAreDevelopmentFriendly(t *testing.T) {
	withEnv(t, map[string]string{}, func() {
		cfg := Load()

		assert.Equal(t, EnvDevelopment, cfg.AppEnv)
		assert.Equal(t, "8002", cfg.Port)
		assert.Equal(t, "postgres", cfg.DatabaseDriver)
		assert.Equal(t, "text", cfg.LogFormat, "local runs should be human readable")
		assert.False(t, cfg.RefreshCookieSecure, "plain HTTP localhost must work out of the box")
		assert.Contains(t, cfg.CORSOrigins, "http://localhost:5173")
	})
}

// A production deploy that forgets CORS_ORIGINS would otherwise fall back to
// the localhost defaults, boot successfully, pass its health check, and then
// authorise the wrong origin for real users.
func TestProductionRejectsLoopbackOrigins(t *testing.T) {
	withEnv(t, map[string]string{"APP_ENV": EnvProduction}, func() {
		cfg := Load()
		cfg.JWTSecret = strings.Repeat("s", 40)
		cfg.LogFormat = "json"
		cfg.RefreshCookieSecure = true

		require.Error(t, cfg.Validate(), "the development CORS defaults must be rejected in production")
	})
}

func TestProductionRejectsExplicitLoopbackOrigin(t *testing.T) {
	cfg := validConfig()
	cfg.CORSOrigins = []string{"https://app.example.com", "http://localhost:3000"}

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "loopback")
}

func TestProductionAcceptsRealOrigins(t *testing.T) {
	cfg := validConfig()
	cfg.CORSOrigins = []string{"https://app.example.com", "https://m.example.com"}

	require.NoError(t, cfg.Validate())
}

func TestProductionRejectsWildcardOrigin(t *testing.T) {
	cfg := validConfig()
	cfg.CORSOrigins = []string{"*"}

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "CORS_ORIGINS")
}

func TestProductionRejectsDefaultJWTSecret(t *testing.T) {
	cfg := validConfig()
	cfg.JWTSecret = DefaultJWTSecret

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestProductionRejectsDefaultPaymentWebhookSecret(t *testing.T) {
	cfg := validConfig()
	cfg.MockPaymentWebhookSecret = DefaultMockPaymentWebhookSecret

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MOCK_PAYMENT_WEBHOOK_SECRET")
}

func TestProductionNeverSeedsDemoCredentials(t *testing.T) {
	withEnv(t, map[string]string{"APP_ENV": EnvProduction}, func() {
		assert.False(t, Load().Seed)
	})
	cfg := validConfig()
	cfg.Seed = true
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SEED")
}

func TestShortJWTSecretIsRejectedInEveryEnvironment(t *testing.T) {
	cfg := validConfig()
	cfg.AppEnv = EnvDevelopment
	cfg.LogFormat = "text"
	cfg.RefreshCookieSecure = false
	cfg.JWTSecret = "too-short"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 32 characters")
}

func TestProductionRequiresSecureCookies(t *testing.T) {
	cfg := validConfig()
	cfg.RefreshCookieSecure = false

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "REFRESH_COOKIE_SECURE")
}

func TestProductionRequiresJSONLogs(t *testing.T) {
	cfg := validConfig()
	cfg.LogFormat = "text"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "LOG_FORMAT")
}

func TestValidProductionConfigPasses(t *testing.T) {
	require.NoError(t, validConfig().Validate())
}

func TestUnknownAppEnvIsRejected(t *testing.T) {
	cfg := validConfig()
	cfg.AppEnv = "staging"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "APP_ENV")
}

func TestUnsupportedDatabaseDriverIsRejected(t *testing.T) {
	cfg := validConfig()
	cfg.DatabaseDriver = "mysql"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_DRIVER")
}

func TestRefreshTTLMustExceedAccessTTL(t *testing.T) {
	cfg := validConfig()
	cfg.AccessTokenTTL = cfg.RefreshTokenTTL

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "REFRESH_TOKEN_TTL")
}

func TestNonNumericPortIsRejected(t *testing.T) {
	cfg := validConfig()
	cfg.Port = "eight-thousand"

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "PORT")
}

func TestCORSOriginsAreParsedAndTrimmed(t *testing.T) {
	withEnv(t, map[string]string{
		"CORS_ORIGINS": " https://a.example.com , https://b.example.com ,, ",
	}, func() {
		cfg := Load()

		assert.Equal(t, []string{"https://a.example.com", "https://b.example.com"}, cfg.CORSOrigins,
			"blank entries must be dropped so an empty origin can never be allowed")
	})
}

func TestIsProductionOnlyTrueForProduction(t *testing.T) {
	cfg := validConfig()
	assert.True(t, cfg.IsProduction())

	cfg.AppEnv = EnvTest
	assert.False(t, cfg.IsProduction())
}
