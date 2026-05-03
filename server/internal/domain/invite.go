package domain

import "time"

// InviteSession represents a temporary session invite
type InviteSession struct {
	Code       string    `json:"code"`
	Company    string    `json:"company"`
	Technician string    `json:"technician"`
	SessionType string   `json:"session_type"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Used       bool      `json:"used"`
	AgentID    string    `json:"agent_id,omitempty"`
}

// InviteStore defines the interface for session invites
type InviteStore interface {
	// Create generates a new invite session
	Create(company, technician, sessionType string, ttl time.Duration) *InviteSession

	// Validate checks if a code is valid and not used/expired
	Validate(code string) (*InviteSession, error)

	// MarkUsed flags a code as consumed by a specific agent
	MarkUsed(code, agentID string)

	// Cleanup removes old sessions
	Cleanup()
}
