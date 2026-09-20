package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDotEnvLoadsProjectConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("# Project configuration\nDATABASE_URL='postgres://project@127.0.0.1:5433/project?sslmode=disable'\nPORT=8123\n"), 0600))
	withEnv(t, map[string]string{}, func() {
		require.NoError(t, LoadDotEnv(path))
		cfg := Load()
		assert.Equal(t, "postgres://project@127.0.0.1:5433/project?sslmode=disable", cfg.DatabaseURL)
		assert.Equal(t, "8123", cfg.Port)
	})
}

func TestLoadDotEnvPreservesProcessOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("DATABASE_URL=project-db\nREDIS_URL=redis://localhost:6379\n"), 0600))
	withEnv(t, map[string]string{"DATABASE_URL": "deployment-db", "REDIS_URL": ""}, func() {
		require.NoError(t, LoadDotEnv(path))
		assert.Equal(t, "deployment-db", Load().DatabaseURL)
		assert.Empty(t, Load().RedisURL)
	})
}

func TestLoadDotEnvMissingFileIsOptional(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	withEnv(t, map[string]string{"DATABASE_URL": "deployment-db"}, func() {
		require.NoError(t, LoadDotEnv(path))
		assert.Equal(t, "deployment-db", Load().DatabaseURL)
	})
}

func TestLoadDotEnvRejectsInvalidFileWithoutLeakingSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("PORT=8123\nJWT_SECRET='private-secret\n"), 0600))
	withEnv(t, map[string]string{}, func() {
		err := LoadDotEnv(path)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "private-secret")
		_, exists := os.LookupEnv("PORT")
		assert.False(t, exists, "invalid files must not be partially applied")
	})
}
