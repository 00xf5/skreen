package registry

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"scon/server/internal/domain"
)

// LoadPersisted loads agents from two sources in priority order:
//  1. AGENTS_SEED environment variable (JSON array) — used on Render and other
//     ephemeral-filesystem hosts where writing to disk is pointless.
//  2. A JSON file at r.persistPath — used for local development.
//
// This dual strategy means local dev still works exactly as before, while
// production (Render) never loses agent tokens across restarts as long as the
// AGENTS_SEED env var is kept up-to-date.
func (r *InMemoryRegistry) LoadPersisted() {
	var savedAgents []*domain.Agent

	// ── 1. Try environment variable first ──────────────────────────────────────
	if seed := os.Getenv("AGENTS_SEED"); seed != "" {
		if err := json.Unmarshal([]byte(seed), &savedAgents); err != nil {
			log.Printf("[registry] WARNING: AGENTS_SEED is set but invalid JSON: %v", err)
			savedAgents = nil
		} else {
			log.Printf("[registry] Loaded %d agents from AGENTS_SEED env var", len(savedAgents))
		}
	}

	// ── 2. Fall back to file (local dev) ───────────────────────────────────────
	if savedAgents == nil && r.persistPath != "" {
		data, err := os.ReadFile(r.persistPath)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("[registry] Failed to read %s: %v", r.persistPath, err)
			}
			return
		}
		if err := json.Unmarshal(data, &savedAgents); err != nil {
			log.Printf("[registry] Failed to unmarshal %s: %v", r.persistPath, err)
			return
		}
		log.Printf("[registry] Loaded %d agents from %s", len(savedAgents), r.persistPath)
	}

	if len(savedAgents) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, a := range savedAgents {
		// Loaded agents start as offline until they reconnect.
		a.IsOnline = false
		a.Conn = nil
		r.agents[a.ID] = a
		if a.TokenHash != "" {
			r.tokenIdx[a.TokenHash] = a.ID
		}
	}
}

// Save persists the current agent list.
//
// On production (Render), r.persistPath is empty so the file write is skipped.
// Instead, the current state is always written to the local file when available.
// To update AGENTS_SEED after agents register, call DumpSeed() and copy the
// output into your Render environment variable.
func (r *InMemoryRegistry) Save() error {
	r.mu.RLock()
	agentsList := make([]*domain.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		agentsList = append(agentsList, a)
	}
	r.mu.RUnlock()

	// Always try to write the file for local dev.
	if r.persistPath != "" {
		if err := os.MkdirAll(filepath.Dir(r.persistPath), 0755); err == nil {
			data, err := json.MarshalIndent(agentsList, "", "  ")
			if err == nil {
				if writeErr := os.WriteFile(r.persistPath, data, 0644); writeErr != nil {
					log.Printf("[registry] Failed to write %s: %v", r.persistPath, writeErr)
				}
			}
		}
	}

	return nil
}

// DumpSeed prints the current agent registry as a compact JSON string to stderr.
// Copy this output and set it as the AGENTS_SEED environment variable on Render
// to survive restarts without losing agent tokens.
func (r *InMemoryRegistry) DumpSeed() {
	r.mu.RLock()
	agentsList := make([]*domain.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		// Zero out transient state before dumping
		copy := *a
		copy.Conn = nil
		copy.IsOnline = false
		agentsList = append(agentsList, &copy)
	}
	r.mu.RUnlock()

	data, err := json.Marshal(agentsList)
	if err != nil {
		log.Printf("[registry] DumpSeed marshal error: %v", err)
		return
	}
	// Write to stderr so it's visible in Render logs but doesn't corrupt stdout.
	log.Printf("[registry] ===== AGENTS_SEED SNAPSHOT (copy to Render env var) =====\n%s\n[registry] ===== END SNAPSHOT =====", string(data))
}
