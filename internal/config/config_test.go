package config_test

import (
	"testing"

	"github.com/refreshTechRusuh/event-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("ACCESS_TOKEN_SECRET", "test-access-secret")
}

func TestLoad_defaults(t *testing.T) {
	setRequired(t)

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "local", cfg.App.Env)
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
	assert.Equal(t, "localhost", cfg.DB.Host)
	assert.Equal(t, 5432, cfg.DB.Port)
	assert.Equal(t, "postgres", cfg.DB.User)
	assert.False(t, cfg.Valkey.TLS)
	assert.Equal(t, "localhost:9092", cfg.Kafka.Brokers)
	assert.Equal(t, "test-access-secret", cfg.JWT.AccessTokenSecret)
}

func TestLoad_customValues(t *testing.T) {
	setRequired(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("DB_HOST", "10.0.0.5")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("VALKEY_TLS", "true")
	t.Setenv("KAFKA_BROKERS", "kafka.internal:9092")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "production", cfg.App.Env)
	assert.Equal(t, "warn", cfg.Log.Level)
	assert.Equal(t, "10.0.0.5", cfg.DB.Host)
	assert.Equal(t, 5433, cfg.DB.Port)
	assert.True(t, cfg.Valkey.TLS)
	assert.Equal(t, "kafka.internal:9092", cfg.Kafka.Brokers)
}

func TestLoad_customPort(t *testing.T) {
	setRequired(t)
	t.Setenv("PORT", "8082")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 8082, cfg.App.Port)
}

func TestLoad_DSN(t *testing.T) {
	setRequired(t)
	t.Setenv("DB_HOST", "myhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "admin")
	t.Setenv("DB_PASSWORD", "pass")
	t.Setenv("DB_NAME", "eventdb")
	t.Setenv("DB_SSL_MODE", "disable")
	t.Setenv("DB_CONN_TIMEOUT_SECONDS", "5")

	cfg, err := config.Load()
	require.NoError(t, err)
	dsn := cfg.DB.DSN()
	assert.Contains(t, dsn, "host=myhost")
	assert.Contains(t, dsn, "dbname=eventdb")
}

func TestLoad_boolEnvFalse(t *testing.T) {
	setRequired(t)
	t.Setenv("VALKEY_TLS", "false")
	t.Setenv("KAFKA_TLS", "0")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.False(t, cfg.Valkey.TLS)
	assert.False(t, cfg.Kafka.TLS)
}

func TestSetupLogger_allLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "unknown"} {
		t.Run(level, func(t *testing.T) {
			config.SetupLogger(config.LogConfig{Level: level, Format: "json"})
		})
	}
}

func TestSetupLogger_textFormat(t *testing.T) {
	config.SetupLogger(config.LogConfig{Level: "info", Format: "text"})
}
