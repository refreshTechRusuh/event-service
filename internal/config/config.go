package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/refreshTechRusuh/event-service/pkg/logctx"
)

type Config struct {
	App AppConfig
	Log LogConfig
	DB  DBConfig
	JWT JWTConfig
	Kafka KafkaConfig
}

type AppConfig struct {
	Env  string
	Port int
}

type LogConfig struct {
	Level  string
	Format string
}

type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnTimeoutSecs int
}

type JWTConfig struct {
	AccessTokenSecret string
}

type KafkaConfig struct {
	Brokers       string
	Username      string
	Password      string
	TLS           bool
	ConsumerGroup string
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	cfg := &Config{
		App:   AppConfig{Env: getEnv("APP_ENV", "local"), Port: getEnvInt("APP_PORT", 8082)},
		Log:   LogConfig{Level: getEnv("LOG_LEVEL", "info"), Format: getEnv("LOG_FORMAT", "json")},
		DB: DBConfig{
			Host: getEnv("DB_HOST", "localhost"), Port: getEnvInt("DB_PORT", 5432),
			User: getEnv("DB_USER", "postgres"), Password: getEnv("DB_PASSWORD", ""),
			Name: getEnv("DB_NAME", "rusuh_booking"), SSLMode: getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 25), MaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnTimeoutSecs: getEnvInt("DB_CONN_TIMEOUT_SECONDS", 5),
		},
		JWT: JWTConfig{AccessTokenSecret: mustEnv("ACCESS_TOKEN_SECRET")},
		Kafka: KafkaConfig{
			Brokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
			Username: getEnv("KAFKA_USERNAME", ""), Password: getEnv("KAFKA_PASSWORD", ""),
			TLS: getEnvBool("KAFKA_TLS", false),
			ConsumerGroup: getEnv("KAFKA_CONSUMER_GROUP", "event-service"),
		},
	}
	return cfg, nil
}

func SetupLogger(cfg LogConfig) {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug": level = slog.LevelDebug
	case "warn":  level = slog.LevelWarn
	case "error": level = slog.LevelError
	}
	var h slog.Handler
	if cfg.Format == "text" {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	// Bungkus dengan ContextHandler agar txId dari context otomatis ikut di setiap log.
	slog.SetDefault(slog.New(logctx.NewContextHandler(h)))
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode, d.ConnTimeoutSecs)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" { return v }
	return fallback
}
func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil { return i }
	}
	return fallback
}
func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil { return b }
	}
	return fallback
}
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" { slog.Error("required env not set", "key", key); os.Exit(1) }
	return v
}
