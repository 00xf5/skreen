package domain

import (
	"context"
	"time"
)

// AgentRegistry defines the interface for agent storage
type AgentRegistry interface {
	// Register adds a new agent to the registry
	Register(agent *Agent) error

	// Unregister removes an agent from the registry
	Unregister(agentID string) error

	// Get retrieves an agent by ID
	Get(agentID string) (*Agent, error)

	// GetByToken retrieves an agent by its token
	GetByToken(token string) (*Agent, error)

	// GetAll returns all registered agents
	GetAll() []*Agent

	// Save commits current state to persistence layer
	Save() error

	// GetOnline returns only online agents
	GetOnline() []*Agent

	// Count returns the total number of registered agents
	Count() int

	// GetOnlineCount returns the number of currently online agents
	GetOnlineCount() int

	// List returns all agents for metrics
	List() ([]*Agent, error)

	// UpdateHeartbeat updates the last seen timestamp
	UpdateHeartbeat(agentID string) error

	// UpdateToken updates an agent's authentication token
	UpdateToken(agentID, token string) error

	// RevokeToken marks an agent's token as revoked
	RevokeToken(agentID string) error

	// IsRevoked checks if an agent's token is revoked
	IsRevoked(agentID string) bool

	// ValidateNonce checks if a nonce has been seen (replay protection)
	ValidateNonce(agentID, nonce string) bool

	// CleanupOffline removes agents that haven't heartbeat within timeout
	CleanupOffline(timeout time.Duration) []string

	// Subscribe returns a channel for agent list changes
	Subscribe() <-chan RegistryEvent

	// Unsubscribe removes a subscription
	Unsubscribe(ch <-chan RegistryEvent)
}

// RegistryEvent represents a change in the registry
type RegistryEvent struct {
	Type    EventType
	AgentID string
	Agent   *Agent
}

// EventType defines the type of registry event
type EventType int

const (
	EventAgentRegistered EventType = iota
	EventAgentUnregistered
	EventAgentHeartbeat
	EventAgentOffline
)

// Authenticator defines the interface for authentication
type Authenticator interface {
	// ValidateToken checks if a token is valid
	ValidateToken(token string) error

	// ValidateAgentToken checks if an agent's token is valid and not revoked
	ValidateAgentToken(agentID, token string) error

	// GenerateAgentToken creates a unique token for an agent
	GenerateAgentToken(agentID string) (string, string, error) // token, hash, error

	// ValidateHMAC verifies the HMAC signature of a message
	ValidateHMAC(msg *Message, secret string) error

	// GenerateHMAC generates an HMAC signature for a message
	GenerateHMAC(msg *Message, secret string) string

	// ValidateMessageSequence checks sequence number for replay protection
	ValidateMessageSequence(agentID string, seqNum int64) bool
}

// CommandRouter defines the interface for command management
type CommandRouter interface {
	// Route sends a command to the specified agent
	Route(ctx context.Context, cmd *CommandRequest) error

	// RouteStructured sends a structured command to an agent
	RouteStructured(ctx context.Context, cmd *StructuredCommand, agentID string) error

	// HandleResult processes a command result from an agent
	HandleResult(result *CommandResult) error

	// ValidateCommand checks if a command is allowed
	ValidateCommand(cmd string) error

	// ParseStructuredCommand parses raw command into structured format
	ParseStructuredCommand(raw string) (*StructuredCommand, error)
}

// WebSocketHub defines the interface for WebSocket management
type WebSocketHub interface {
	// RegisterClient registers a new WebSocket client
	RegisterClient(conn *ClientConnection)

	// UnregisterClient removes a WebSocket client
	UnregisterClient(conn *ClientConnection)

	// SendToAgent sends a message to a specific agent
	SendToAgent(agentID string, msg Message) error

	// SendToController sends a message to all controllers
	SendToControllers(msg Message)

	// GetClient returns a client connection by agent ID
	GetClient(agentID string) (*ClientConnection, error)
}

// AuditLogger defines the interface for audit logging
type AuditLogger interface {
	// Log records a generic audit event
	Log(entry AuditLogEntry)

	// LogAuth records authentication events
	LogAuth(agentID, action, sourceIP string, success bool, details string)

	// LogCommand records command execution events
	LogCommand(agentID, commandID, action string, success bool, duration time.Duration, output string)

	// LogConnection records connection events
	LogConnection(agentID, action, sourceIP string, success bool)

	// LogSecurity records security events (replay attacks, invalid tokens, etc.)
	LogSecurity(agentID, action, sourceIP string, details string)

	// LogSystem records system-level events
	LogSystem(action string, details map[string]string)

	// GetRecent returns recent audit entries
	GetRecent(count int) []AuditLogEntry

	// Flush writes buffered entries to disk
	Flush() error

	// Close flushes and closes the logger
	Close() error
}

// MetricsCollector defines the interface for metrics collection
type MetricsCollector interface {
	// RecordCommandStart records the start of a command
	RecordCommandStart(commandID string)

	// RecordCommandComplete records command completion with latency
	RecordCommandComplete(agentID, commandID string, success bool)

	// GetAgentMetrics returns metrics for a specific agent
	GetAgentMetrics(agentID string) (total int64, failures int64, avgLatency time.Duration)

	// GetFailureRate returns the failure rate for an agent (0.0 - 1.0)
	GetFailureRate(agentID string) float64

	// GetSnapshot creates a metrics snapshot
	GetSnapshot(registry AgentRegistry) MetricsSnapshot

	// StoreSnapshot saves a snapshot to history
	StoreSnapshot(snapshot MetricsSnapshot)

	// GetHistory returns stored snapshots
	GetHistory() []MetricsSnapshot

	// Reset clears all metrics
	Reset()
}
