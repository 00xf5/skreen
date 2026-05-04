package registry

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"scon/server/internal/domain"
)

// LoadPersisted loads agents from a JSON file on disk
func (r *InMemoryRegistry) LoadPersisted() {
	if r.persistPath == "" {
		return
	}

	data, err := os.ReadFile(r.persistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return // Normal on first start
		}
		log.Printf("Failed to read persisted agents from %s: %v", r.persistPath, err)
		return
	}

	var savedAgents []*domain.Agent
	if err := json.Unmarshal(data, &savedAgents); err != nil {
		log.Printf("Failed to unmarshal persisted agents: %v", err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, a := range savedAgents {
		// Loaded agents start as offline until they reconnect
		a.IsOnline = false
		r.agents[a.ID] = a
		if a.TokenHash != "" {
			r.tokenIdx[a.TokenHash] = a.ID
		}
	}
	log.Printf("Loaded %d persisted agents from %s", len(savedAgents), r.persistPath)
}

// Save writes the current agent list to a JSON file on disk
func (r *InMemoryRegistry) Save() error {
	if r.persistPath == "" {
		return nil
	}

	// Make sure directory exists
	if err := os.MkdirAll(filepath.Dir(r.persistPath), 0755); err != nil {
		log.Printf("Failed to create persistence directory: %v", err)
		return err
	}

	r.mu.RLock()
	agentsList := make([]*domain.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		agentsList = append(agentsList, a)
	}
	r.mu.RUnlock()

	data, err := json.MarshalIndent(agentsList, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal agents for persistence: %v", err)
		return err
	}

	if err := os.WriteFile(r.persistPath, data, 0644); err != nil {
		log.Printf("Failed to write persisted agents to %s: %v", r.persistPath, err)
		return err
	}
	return nil
}
