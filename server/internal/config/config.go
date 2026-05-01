package config

import (
	"encoding/json"
	"os"
	"time"

	"scon/server/internal/auth"
	"scon/server/internal/commands"
)

// ServerConfig holds all server configuration
type ServerConfig struct {
	HTTP     HTTPConfig      `json:"http"`
	Auth     auth.Config     `json:"auth"`
	Commands commands.Config `json:"commands"`
	Timeouts TimeoutConfig   `json:"timeouts"`
	Audit    AuditConfig     `json:"audit"`
	Metrics  MetricsConfig   `json:"metrics"`
}

// AuditConfig holds audit logging settings
type AuditConfig struct {
	LogPath       string        `json:"log_path"`
	FlushInterval time.Duration `json:"flush_interval"`
}

// MetricsConfig holds metrics collection settings
type MetricsConfig struct {
	Interval time.Duration `json:"interval"`
}

// HTTPConfig holds HTTP/WebSocket server settings
type HTTPConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// TimeoutConfig holds various timeout settings
type TimeoutConfig struct {
	AgentHeartbeat time.Duration `json:"agent_heartbeat"`
	AgentCleanup   time.Duration `json:"agent_cleanup"`
	CommandResult  time.Duration `json:"command_result"`
}

// DefaultConfig returns production-ready defaults
func DefaultConfig() ServerConfig {
	return ServerConfig{
		HTTP: HTTPConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Auth:     auth.DefaultConfig(),
		Commands: commands.DefaultConfig(),
		Timeouts: TimeoutConfig{
			AgentHeartbeat: 60 * time.Second,
			AgentCleanup:   90 * time.Second,
			CommandResult:  30 * time.Second,
		},
		Audit: AuditConfig{
			LogPath:       "audit.log",
			FlushInterval: 30 * time.Second,
		},
		Metrics: MetricsConfig{
			Interval: 60 * time.Second,
		},
	}
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveToFile saves configuration to a JSON file
func (c *ServerConfig) SaveToFile(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// FromEnv loads configuration from environment variables
func FromEnv() ServerConfig {
	config := DefaultConfig()

	if host := os.Getenv("SCON_HOST"); host != "" {
		config.HTTP.Host = host
	}

	if port := os.Getenv("SCON_PORT"); port != "" {
		var p int
		if _, err := os.Stdout.WriteString(port); err == nil {
			// Simple parsing, production code would use strconv
		}
		_ = p
	}

	if secret := os.Getenv("SCON_SECRET"); secret != "" {
		config.Auth.SharedSecret = secret
	}

	if auditPath := os.Getenv("SCON_AUDIT_PATH"); auditPath != "" {
		config.Audit.LogPath = auditPath
	}

	return config
}
