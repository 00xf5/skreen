package registry

import (
	"sync"
	"time"

	"scon/server/internal/domain"
)

// InMemoryRegistry implements domain.AgentRegistry with thread-safe operations
type InMemoryRegistry struct {
	mu          sync.RWMutex
	agents      map[string]*domain.Agent
	tokenIdx    map[string]string // token hash -> agentID for lookup
	subs        map[chan domain.RegistryEvent]struct{}
	subsMu      sync.RWMutex
	persistPath string
}

// NewInMemoryRegistry creates a new in-memory agent registry
func NewInMemoryRegistry(persistPath string) *InMemoryRegistry {
	r := &InMemoryRegistry{
		agents:      make(map[string]*domain.Agent),
		tokenIdx:    make(map[string]string),
		subs:        make(map[chan domain.RegistryEvent]struct{}),
		persistPath: persistPath,
	}
	r.LoadPersisted()
	return r
}

// Register adds a new agent to the registry
func (r *InMemoryRegistry) Register(agent *domain.Agent) error {
	r.mu.Lock()
	defer r.Save()
	defer r.mu.Unlock()

	if _, exists := r.agents[agent.ID]; exists {
		return domain.ErrAgentExists
	}

	agent.RegisteredAt = time.Now()
	agent.LastSeen = time.Now()
	agent.IsOnline = true
	r.agents[agent.ID] = agent

	r.broadcast(domain.RegistryEvent{
		Type:    domain.EventAgentRegistered,
		AgentID: agent.ID,
		Agent:   agent,
	})

	return nil
}

// Unregister removes an agent from the registry
func (r *InMemoryRegistry) Unregister(agentID string) error {
	r.mu.Lock()
	defer r.Save()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return domain.ErrAgentNotFound
	}

	delete(r.agents, agentID)

	r.broadcast(domain.RegistryEvent{
		Type:    domain.EventAgentUnregistered,
		AgentID: agentID,
		Agent:   agent,
	})

	return nil
}

// Get retrieves an agent by ID
func (r *InMemoryRegistry) Get(agentID string) (*domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return nil, domain.ErrAgentNotFound
	}

	return agent, nil
}

// GetAll returns all registered agents
func (r *InMemoryRegistry) GetAll() []*domain.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*domain.Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		// Return a copy to prevent external mutation
		agents = append(agents, r.copyAgent(agent))
	}
	return agents
}

// GetOnline returns only online agents
func (r *InMemoryRegistry) GetOnline() []*domain.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*domain.Agent, 0)
	for _, agent := range r.agents {
		if agent.IsOnline {
			agents = append(agents, r.copyAgent(agent))
		}
	}

	return agents
}

// Count returns the total number of registered agents
func (r *InMemoryRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// GetOnlineCount returns the number of currently online agents
func (r *InMemoryRegistry) GetOnlineCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, agent := range r.agents {
		if agent.IsOnline {
			count++
		}
	}
	return count
}

// UpdateHeartbeat updates the last seen timestamp
func (r *InMemoryRegistry) UpdateHeartbeat(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return domain.ErrAgentNotFound
	}

	wasOffline := !agent.IsOnline
	agent.LastSeen = time.Now()
	agent.IsOnline = true

	r.broadcast(domain.RegistryEvent{
		Type:    domain.EventAgentHeartbeat,
		AgentID: agentID,
		Agent:   r.copyAgent(agent),
	})

	// Also send online event if agent was previously offline
	if wasOffline {
		r.broadcast(domain.RegistryEvent{
			Type:    domain.EventAgentRegistered,
			AgentID: agentID,
			Agent:   r.copyAgent(agent),
		})
	}

	return nil
}

// GetByToken retrieves an agent by its token hash
func (r *InMemoryRegistry) GetByToken(tokenHash string) (*domain.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agentID, exists := r.tokenIdx[tokenHash]
	if !exists {
		return nil, domain.ErrAgentNotFound
	}

	agent, exists := r.agents[agentID]
	if !exists {
		return nil, domain.ErrAgentNotFound
	}

	return agent, nil
}

// List returns all agents (for metrics)
func (r *InMemoryRegistry) List() ([]*domain.Agent, error) {
	return r.GetAll(), nil
}

// UpdateToken updates an agent's authentication token
func (r *InMemoryRegistry) UpdateToken(agentID, token string) error {
	r.mu.Lock()
	defer r.Save()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return domain.ErrAgentNotFound
	}

	// Remove old token from index
	if agent.TokenHash != "" {
		delete(r.tokenIdx, agent.TokenHash)
	}

	// Update token
	agent.Token = token
	agent.TokenHash = token // In production, this should be a hash
	agent.SeenNonces = make(map[string]bool)
	agent.SeqNum = 0

	// Add to index
	r.tokenIdx[agent.TokenHash] = agentID

	return nil
}

// RevokeToken marks an agent's token as revoked
func (r *InMemoryRegistry) RevokeToken(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return domain.ErrAgentNotFound
	}

	agent.IsRevoked = true
	return nil
}

// IsRevoked checks if an agent's token is revoked
func (r *InMemoryRegistry) IsRevoked(agentID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return true // Treat missing as revoked
	}

	return agent.IsRevoked
}

// ValidateNonce checks if a nonce has been seen (replay protection)
func (r *InMemoryRegistry) ValidateNonce(agentID, nonce string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return false
	}

	if agent.SeenNonces == nil {
		agent.SeenNonces = make(map[string]bool)
	}

	// Check if nonce already seen
	if agent.SeenNonces[nonce] {
		return false // Replay detected
	}

	// Record nonce
	agent.SeenNonces[nonce] = true

	// Cleanup old nonces if too many
	if len(agent.SeenNonces) > 10000 {
		agent.SeenNonces = make(map[string]bool)
		agent.SeenNonces[nonce] = true
	}

	return true
}

// CleanupOffline removes agents that haven't sent a heartbeat within timeout
func (r *InMemoryRegistry) CleanupOffline(timeout time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	offline := make([]string, 0)

	for id, agent := range r.agents {
		if now.Sub(agent.LastSeen) > timeout && agent.IsOnline {
			agent.IsOnline = false
			offline = append(offline, id)

			r.broadcast(domain.RegistryEvent{
				Type:    domain.EventAgentOffline,
				AgentID: id,
				Agent:   r.copyAgent(agent),
			})
		}
	}

	return offline
}

// Subscribe returns a channel for agent list changes
func (r *InMemoryRegistry) Subscribe() <-chan domain.RegistryEvent {
	ch := make(chan domain.RegistryEvent, 64)

	r.subsMu.Lock()
	r.subs[ch] = struct{}{}
	r.subsMu.Unlock()

	return ch
}

// Unsubscribe removes a subscription
func (r *InMemoryRegistry) Unsubscribe(ch <-chan domain.RegistryEvent) {
	r.subsMu.Lock()
	defer r.subsMu.Unlock()

	// Convert back to send channel for deletion
	for sendCh := range r.subs {
		if (<-chan domain.RegistryEvent)(sendCh) == ch {
			close(sendCh)
			delete(r.subs, sendCh)
			return
		}
	}
}

// broadcast sends an event to all subscribers (must hold lock)
func (r *InMemoryRegistry) broadcast(event domain.RegistryEvent) {
	r.subsMu.RLock()
	defer r.subsMu.RUnlock()

	for ch := range r.subs {
		select {
		case ch <- event:
		default:
			// Channel full, skip this subscriber
		}
	}
}

// copyAgent creates a safe copy of an agent
func (r *InMemoryRegistry) copyAgent(agent *domain.Agent) *domain.Agent {
	return &domain.Agent{
		ID:           agent.ID,
		Token:        agent.Token,
		TokenHash:    agent.TokenHash,
		IsRevoked:    agent.IsRevoked,
		LastSeen:     agent.LastSeen,
		RegisteredAt: agent.RegisteredAt,
		IsOnline:     agent.IsOnline,
		Meta:         agent.Meta,
		CommandCount: agent.CommandCount,
		FailedCount:  agent.FailedCount,
		LastLatency:  agent.LastLatency,
		// Note: Conn and SeenNonces are not copied as they're not safe to share
	}
}

// Close shuts down the registry and all subscriptions
func (r *InMemoryRegistry) Close() {
	r.subsMu.Lock()
	defer r.subsMu.Unlock()

	for ch := range r.subs {
		close(ch)
		delete(r.subs, ch)
	}
}
