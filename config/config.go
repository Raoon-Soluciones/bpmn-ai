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
	WorkerCount      int
	MaxLoops         int
	ExecutionTimeout time.Duration
	QueuePollInterval time.Duration
	MaxRetries       int
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
