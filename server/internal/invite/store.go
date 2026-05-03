package invite

import (
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"

	"scon/server/internal/domain"
)

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*domain.InviteSession
}

func NewStore() *Store {
	return &Store{sessions: make(map[string]*domain.InviteSession)}
}

func (s *Store) Create(company, technician, sessionType string, ttl time.Duration) *domain.InviteSession {
	code := generateCode()
	now := time.Now()

	sess := &domain.InviteSession{
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

func (s *Store) Validate(code string) (*domain.InviteSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	code = strings.ToUpper(code)
	sess, exists := s.sessions[code]
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
