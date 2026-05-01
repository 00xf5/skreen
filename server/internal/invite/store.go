package invite

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

type Session struct {
	Code       string    `json:"code"`
	Company    string    `json:"company"`
	Technician string    `json:"technician"`
	SessionType string   `json:"session_type"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Used       bool      `json:"used"`
	AgentID    string    `json:"agent_id,omitempty"`
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewStore() *Store {
	return &Store{sessions: make(map[string]*Session)}
}

func (s *Store) Create(company, technician, sessionType string, ttl time.Duration) *Session {
	code := generateCode()
	now := time.Now()

	sess := &Session{
		Code:        code,
		Company:     company,
		Technician:  technician,
		SessionType: sessionType,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}

	s.mu.Lock()
	s.sessions[code] = sess
	s.mu.Unlock()

	return sess
}

func (s *Store) Validate(code string) (*Session, error) {
	s.mu.RLock()
	sess, exists := s.sessions[code]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("invalid code")
	}
	if sess.Used {
		return nil, fmt.Errorf("code already used")
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, fmt.Errorf("code expired")
	}
	return sess, nil
}

func (s *Store) MarkUsed(code, agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[code]; ok {
		sess.Used = true
		sess.AgentID = agentID
	}
}

func (s *Store) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for code, sess := range s.sessions {
		if now.After(sess.ExpiresAt.Add(1 * time.Hour)) {
			delete(s.sessions, code)
		}
	}
}

func generateCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%02X%02X-%02X%02X", b[0], b[1], b[2], b[3])
}
