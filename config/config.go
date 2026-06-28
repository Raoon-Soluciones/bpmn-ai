package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the engine configuration.
type Config struct {
	Server      ServerConfig
	Database    DatabaseConfig
	Engine      EngineConfig
	AI          AIConfig
	Log         LogConfig
	Audit       AuditConfig
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Host           string
	Port           int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxBodySize    int64
	AllowedOrigins []string
	TLSCertFile    string
	TLSKeyFile     string
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	URL         string
	MaxConns    int
	MinConns    int
	MaxIdleTime time.Duration
}

// EngineConfig holds BPMN engine configuration.
type EngineConfig struct {
	WorkerCount       int
	MaxLoops          int
	ExecutionTimeout  time.Duration
	QueuePollInterval time.Duration
	MaxRetries        int
}

// AIConfig holds AI provider configuration.
type AIConfig struct {
	Provider       string
	APIKey         string
	BaseURL        string
	DefaultModel   string
	FallbackModel  string
	MaxTokens      int
	Temperature    float64
	Timeout        time.Duration
	Cache          AICacheConfig
	DefaultProfile string
	ExtraProviders string // comma-separated: name:key:url
}

// AIProviderConfig holds configuration for a named AI provider.
type AIProviderConfig struct {
	Provider string // "openai", "anthropic"
	Model    string
	APIKey   string
	BaseURL  string
}

// AICacheConfig holds AI response cache configuration.
type AICacheConfig struct {
	Enabled  bool
	TTL      time.Duration
	Type     string // "memory" or "redis"
	RedisURL string
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string // debug, info, warn, error
	Format string // json, text
}

// AuditConfig holds audit log configuration.
type AuditConfig struct {
	Enabled bool
	Dir     string
}

// Default returns a configuration with sensible defaults.
func Default() Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/bpmn?sslmode=disable"
	}

	return Config{
		Server: ServerConfig{
			Host:             "0.0.0.0",
			Port:             8080,
			ReadTimeout:      15 * time.Second,
			WriteTimeout:     30 * time.Second,
			MaxBodySize:      10 << 20, // 10MB
			AllowedOrigins:   []string{"https://localhost:3000"},
		},
		Database: DatabaseConfig{
			URL:         dbURL,
			MaxConns:    25,
			MinConns:    5,
			MaxIdleTime: 30 * time.Minute,
		},
		Engine: EngineConfig{
			WorkerCount:       4,
			MaxLoops:          100,
			ExecutionTimeout:  30 * time.Second,
			QueuePollInterval: 5 * time.Second,
			MaxRetries:        3,
		},
		AI: AIConfig{
			Provider:       envOrDefault("AI_PROVIDER", "openai"),
			APIKey:         os.Getenv("AI_API_KEY"),
			BaseURL:        os.Getenv("AI_BASE_URL"),
			DefaultModel:   envOrDefault("AI_DEFAULT_MODEL", "gpt-4o"),
			FallbackModel:  envOrDefault("AI_FALLBACK_MODEL", "gpt-4o-mini"),
			MaxTokens:      parseEnvInt("AI_MAX_TOKENS", 4096),
			Temperature:    parseEnvFloat("AI_TEMPERATURE", 0.7),
			Timeout:        parseEnvDuration("AI_TIMEOUT", 60*time.Second),
			DefaultProfile: envOrDefault("AI_DEFAULT_PROFILE", "auto"),
			ExtraProviders: os.Getenv("AI_EXTRA_PROVIDERS"),
			Cache: AICacheConfig{
				Enabled:  parseBoolEnv("AI_CACHE_ENABLED", false),
				TTL:      parseEnvDuration("AI_CACHE_TTL", 5*time.Minute),
				Type:     envOrDefault("AI_CACHE_TYPE", "memory"),
				RedisURL: os.Getenv("AI_REDIS_URL"),
			},
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
		Audit: AuditConfig{
			Enabled: parseBoolEnv("AUDIT_LOG_ENABLED", true),
			Dir:     envOrDefault("AUDIT_LOG_DIR", "./data/audit"),
		},
	}
}

func parseEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}

func parseEnvFloat(key string, defaultVal float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

func parseEnvDuration(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}
	return d
}

func parseBoolEnv(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}

func envOrDefault(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

// Validate checks the configuration for errors.
func (c Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}
	if c.Engine.WorkerCount < 1 {
		return fmt.Errorf("engine worker count must be at least 1")
	}
	if c.Engine.MaxLoops < 1 {
		return fmt.Errorf("engine max loops must be at least 1")
	}
	if c.Engine.ExecutionTimeout < 1*time.Second {
		return fmt.Errorf("engine execution timeout must be at least 1 second")
	}
	if c.Audit.Enabled && c.Audit.Dir == "" {
		return fmt.Errorf("audit log directory must not be empty when audit is enabled")
	}
	if (c.Server.TLSCertFile == "") != (c.Server.TLSKeyFile == "") {
		return fmt.Errorf("both TLSCertFile and TLSKeyFile must be provided for TLS")
	}
	return nil
}
