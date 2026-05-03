package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// AgentConfig holds all agent configuration
type AgentConfig struct {
	Server    ServerConfig    `json:"server"`
	Agent     AgentIdentity   `json:"agent"`
	Behavior  BehaviorConfig  `json:"behavior"`
	Security  SecurityConfig  `json:"security"`
	Code      string          `json:"code,omitempty"`
}

// ServerConfig holds server connection settings
type ServerConfig struct {
	Host      string        `json:"host"`
	Port      int           `json:"port"`
	Path      string        `json:"path"`
	TLS       bool          `json:"tls"`
}

// AgentIdentity holds agent identification
type AgentIdentity struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
}

// BehaviorConfig holds runtime behavior settings
type BehaviorConfig struct {
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	ReconnectMinDelay time.Duration `json:"reconnect_min_delay"`
	ReconnectMaxDelay time.Duration `json:"reconnect_max_delay"`
	CommandTimeout    time.Duration `json:"command_timeout"`
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	Token         string `json:"token"`
	AllowCommands bool   `json:"allow_commands"`
}

// DefaultConfig returns production-ready defaults
func DefaultConfig() AgentConfig {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	return AgentConfig{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
			Path: "/ws/agent",
			TLS:  false,
		},
		Agent: AgentIdentity{
			Hostname: hostname,
		},
		Behavior: BehaviorConfig{
			HeartbeatInterval: 15 * time.Second,
			ReconnectMinDelay: 5 * time.Second,
			ReconnectMaxDelay: 60 * time.Second,
			CommandTimeout:    30 * time.Second,
		},
		Security: SecurityConfig{
			AllowCommands: true,
		},
	}
}

// GetWebSocketURL returns the full WebSocket URL
func (c *AgentConfig) GetWebSocketURL() string {
	scheme := "ws"
	if c.Server.TLS || c.Server.Port == 443 {
		scheme = "wss"
	}

	// For production (Render/Cloud), port might be omitted or 80/443
	if c.Server.Port == 0 || c.Server.Port == 80 || c.Server.Port == 443 {
		return fmt.Sprintf("%s://%s%s", scheme, c.Server.Host, c.Server.Path)
	}

	return fmt.Sprintf("%s://%s:%d%s", scheme, c.Server.Host, c.Server.Port, c.Server.Path)
}

// FromEnv loads configuration from environment variables
func FromEnv() AgentConfig {
	config := DefaultConfig()

	if host := os.Getenv("SCON_SERVER_HOST"); host != "" {
		config.Server.Host = host
	}
	if port := os.Getenv("SCON_SERVER_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &config.Server.Port)
	}
	if token := os.Getenv("SCON_TOKEN"); token != "" {
		config.Security.Token = token
	}
	if id := os.Getenv("SCON_AGENT_ID"); id != "" {
		config.Agent.ID = id
	}
	if code := os.Getenv("SCON_CODE"); code != "" {
		config.Code = code
	}

	return config
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(path string) (*AgentConfig, error) {
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
func (c *AgentConfig) SaveToFile(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
