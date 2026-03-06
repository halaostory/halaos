package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Upload   UploadConfig
	AI       AIConfig
	SMTP     SMTPConfig
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	Enabled  bool
}

type ServerConfig struct {
	Host string
	Port string
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DB       string
	SSLMode  string
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DB, p.SSLMode)
}

type RedisConfig struct {
	Addr     string
	Password string
}

type JWTConfig struct {
	Secret         string
	Expiry         time.Duration
	RefreshExpiry  time.Duration
}

type UploadConfig struct {
	Dir         string
	MaxFileSize int64
}

type AIConfig struct {
	AnthropicKey string
	OpenAIKey    string
	GeminiKey    string
	Enabled      bool
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Postgres: PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnv("POSTGRES_PORT", "5434"),
			User:     getEnv("POSTGRES_USER", "aigonhr"),
			Password: getEnv("POSTGRES_PASSWORD", "aigonhr_dev"),
			DB:       getEnv("POSTGRES_DB", "aigonhr"),
			SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6380"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", "dev-secret-change-me-in-production"),
			Expiry:        getEnvDuration("JWT_EXPIRY", 24*time.Hour),
			RefreshExpiry: getEnvDuration("JWT_REFRESH_EXPIRY", 720*time.Hour),
		},
		Upload: UploadConfig{
			Dir:         getEnv("UPLOAD_DIR", "./uploads"),
			MaxFileSize: getEnvInt64("MAX_FILE_SIZE", 10485760),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", ""),
			Port:     int(getEnvInt64("SMTP_PORT", 587)),
			User:     getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", "noreply@aigonhr.com"),
			Enabled:  getEnvBool("SMTP_ENABLED", false),
		},
		AI: AIConfig{
			AnthropicKey: getEnv("ANTHROPIC_API_KEY", ""),
			OpenAIKey:    getEnv("OPENAI_API_KEY", ""),
			GeminiKey:    getEnv("GEMINI_API_KEY", ""),
			Enabled:      getEnvBool("AI_ENABLED", true),
		},
	}
}

func (c *Config) Validate() error {
	if c.JWT.Secret == "" || c.JWT.Secret == "dev-secret-change-me-in-production" {
		if getEnv("GIN_MODE", "") == "release" {
			return fmt.Errorf("JWT_SECRET must be set in production")
		}
	}
	if c.Postgres.Password == "" && getEnv("GIN_MODE", "") == "release" {
		return fmt.Errorf("POSTGRES_PASSWORD is required")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
