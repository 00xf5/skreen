package commands

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"scon/server/internal/domain"
)

// Config holds command router configuration
type Config struct {
	MaxLength       int
	DefaultTimeout  time.Duration
	MaxTimeout      time.Duration
	BlockedPatterns []string
	Allowlist       []string // If set, only these commands allowed
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxLength:      4096,
		DefaultTimeout: 30 * time.Second,
		MaxTimeout:     5 * time.Minute,
		BlockedPatterns: []string{
			`rm\s+-rf\s+/`,
			`:(){ :|:& };:`, // Fork bomb
			`>\s*/dev/[sh]da`,
		},
	}
}

// Router implements domain.CommandRouter
type Router struct {
	config       Config
	registry     domain.AgentRegistry
	hub          domain.WebSocketHub
	results      map[string]*domain.CommandResult
	resultsMu    sync.RWMutex
	pending      map[string]chan *domain.CommandResult
	pendingMu    sync.RWMutex
	blockRegexps []*regexp.Regexp
}

// NewRouter creates a new command router
func NewRouter(config Config, registry domain.AgentRegistry, hub domain.WebSocketHub) (*Router, error) {
	r := &Router{
		config:   config,
		registry: registry,
		hub:      hub,
		results:  make(map[string]*domain.CommandResult),
		pending:  make(map[string]chan *domain.CommandResult),
	}

	// Compile blocked patterns
	for _, pattern := range config.BlockedPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid blocked pattern %q: %w", pattern, err)
		}
		r.blockRegexps = append(r.blockRegexps, re)
	}

	return r, nil
}

// Route sends a command to the specified agent
func (r *Router) Route(ctx context.Context, cmd *domain.CommandRequest) error {
	// Validate command
	if err := r.ValidateCommand(cmd.Command); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Check agent exists and is online
	agent, err := r.registry.Get(cmd.AgentID)
	if err != nil {
		return err
	}
	if !agent.IsOnline {
		return domain.ErrAgentOffline
	}

	// Set default timeout if not specified
	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = r.config.DefaultTimeout
	}
	if timeout > r.config.MaxTimeout {
		timeout = r.config.MaxTimeout
	}

	// Create pending channel for result
	resultCh := make(chan *domain.CommandResult, 1)
	r.pendingMu.Lock()
	r.pending[cmd.ID] = resultCh
	r.pendingMu.Unlock()

	// Cleanup on exit
	defer func() {
		r.pendingMu.Lock()
		delete(r.pending, cmd.ID)
		r.pendingMu.Unlock()
	}()

	// Send command to agent
	msg := domain.Message{
		Type:      domain.MsgCommand,
		AgentID:   cmd.AgentID,
		Command:   cmd.Command,
		Timestamp: time.Now().Unix(),
	}

	if err := r.hub.SendToAgent(cmd.AgentID, msg); err != nil {
		return fmt.Errorf("failed to send command to agent: %w", err)
	}

	// Wait for result or timeout
	select {
	case result := <-resultCh:
		if result.Error != "" {
			return fmt.Errorf("command failed on agent: %s", result.Error)
		}
		return nil

	case <-ctx.Done():
		return fmt.Errorf("command cancelled: %w", ctx.Err())

	case <-time.After(timeout):
		return domain.ErrCommandTimeout
	}
}

// HandleResult processes a command result from an agent
func (r *Router) HandleResult(result *domain.CommandResult) error {
	// Store result
	r.resultsMu.Lock()
	r.results[result.ID] = result
	r.resultsMu.Unlock()

	// Notify pending waiter if exists
	r.pendingMu.RLock()
	ch, exists := r.pending[result.ID]
	r.pendingMu.RUnlock()

	if exists {
		select {
		case ch <- result:
		default:
			// Channel full or closed
		}
	}

	// Broadcast to controllers
	msg := domain.Message{
		Type:    domain.MsgResult,
		AgentID: result.AgentID,
		Output:  result.Output,
		Error:   result.Error,
		Data:    result,
	}
	r.hub.SendToControllers(msg)

	return nil
}

// ValidateCommand checks if a command is allowed
func (r *Router) ValidateCommand(cmd string) error {
	// Basic validation
	if strings.TrimSpace(cmd) == "" {
		return domain.ErrCommandEmpty
	}

	if len(cmd) > r.config.MaxLength {
		return domain.ErrCommandTooLong
	}

	// Check allowlist if configured
	if len(r.config.Allowlist) > 0 {
		allowed := false
		for _, allowedCmd := range r.config.Allowlist {
			if strings.HasPrefix(cmd, allowedCmd) {
				allowed = true
				break
			}
		}
		if !allowed {
			return domain.ErrCommandBlocked
		}
	}

	// Check blocked patterns
	for _, re := range r.blockRegexps {
		if re.MatchString(cmd) {
			return domain.ErrCommandBlocked
		}
	}

	return nil
}

// GetResult retrieves a stored command result
func (r *Router) GetResult(cmdID string) (*domain.CommandResult, bool) {
	r.resultsMu.RLock()
	defer r.resultsMu.RUnlock()
	result, exists := r.results[cmdID]
	return result, exists
}

// CleanupOldResults removes results older than the specified duration
func (r *Router) CleanupOldResults(maxAge time.Duration) int {
	r.resultsMu.Lock()
	defer r.resultsMu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, result := range r.results {
		if result.Completed && time.Unix(0, 0).Before(cutoff) {
			delete(r.results, id)
			removed++
		}
	}

	return removed
}

// RouteStructured sends a structured command to an agent
func (r *Router) RouteStructured(ctx context.Context, cmd *domain.StructuredCommand, agentID string) error {
	// Validate command
	if err := r.ValidateCommand(cmd.Action); err != nil {
		return fmt.Errorf("structured command validation failed: %w", err)
	}

	// Check agent exists and is online
	agent, err := r.registry.Get(agentID)
	if err != nil {
		return err
	}
	if !agent.IsOnline {
		return domain.ErrAgentOffline
	}

	// Set default timeout if not specified
	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = r.config.DefaultTimeout
	}
	if timeout > r.config.MaxTimeout {
		timeout = r.config.MaxTimeout
	}

	// Send structured command to agent
	msg := domain.Message{
		Type:      domain.MsgCommand,
		AgentID:   agentID,
		Command:   cmd.Action,
		Data:      cmd,
		Timestamp: time.Now().Unix(),
	}

	if err := r.hub.SendToAgent(agentID, msg); err != nil {
		return fmt.Errorf("failed to send structured command to agent: %w", err)
	}

	// For structured commands, we don't wait for result here
	// Result will be handled asynchronously by HandleResult
	return nil
}

// ParseStructuredCommand parses raw command into structured format
func (r *Router) ParseStructuredCommand(raw string) (*domain.StructuredCommand, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, domain.ErrCommandEmpty
	}

	// Basic parsing: treat entire raw as an "exec" action
	// In production, this could parse JSON or other formats
	cmd := &domain.StructuredCommand{
		ID:            generateCommandID(),
		Type:          "exec",
		Action:        raw,
		Params:        make(map[string]string),
		Timeout:       r.config.DefaultTimeout,
		RequiresAdmin: false,
		AuditLevel:    "medium",
	}

	return cmd, nil
}

// generateCommandID creates a unique command ID
func generateCommandID() string {
	return fmt.Sprintf("cmd-%d", time.Now().UnixNano())
}
