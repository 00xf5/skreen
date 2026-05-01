package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"scon/server/internal/domain"
	"strings"
	"sync"
	"time"
)

// Config holds authentication configuration
type Config struct {
	SharedSecret string
	TokenExpiry  time.Duration
	MaxCmdLength int
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		SharedSecret: "",
		TokenExpiry:  24 * time.Hour,
		MaxCmdLength: 4096,
	}
}

// SimpleAuthenticator implements domain.Authenticator
type SimpleAuthenticator struct {
	config       Config
	agentTokens  map[string]string // agentID -> token hash
	agentSeqNums map[string]int64  // agentID -> last sequence number
	tokenMu      sync.RWMutex
}

// Ensure interface compliance
var _ domain.Authenticator = (*SimpleAuthenticator)(nil)

// NewSimpleAuthenticator creates a new authenticator with the given config
func NewSimpleAuthenticator(config Config) *SimpleAuthenticator {
	return &SimpleAuthenticator{
		config:       config,
		agentTokens:  make(map[string]string),
		agentSeqNums: make(map[string]int64),
	}
}

// ValidateAgentToken checks if an agent's token is valid and not revoked
func (a *SimpleAuthenticator) ValidateAgentToken(agentID, token string) error {
	// First validate the token format
	if err := a.ValidateToken(token); err != nil {
		return err
	}

	a.tokenMu.RLock()
	defer a.tokenMu.RUnlock()

	// Check if we have a per-agent token registered
	expectedHash, exists := a.agentTokens[agentID]
	if !exists {
		// No per-agent token set - accept any valid token (first-time registration)
		return nil
	}

	// Verify token matches expected hash
	tokenHash := a.hashToken(token)
	if tokenHash != expectedHash {
		return domain.ErrInvalidToken
	}

	return nil
}

// GenerateAgentToken creates a unique token for an agent
func (a *SimpleAuthenticator) GenerateAgentToken(agentID string) (string, string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate token: %w", err)
	}

	token := hex.EncodeToString(tokenBytes)
	tokenHash := a.hashToken(token)

	// Store hash (not the actual token)
	a.tokenMu.Lock()
	a.agentTokens[agentID] = tokenHash
	a.agentSeqNums[agentID] = 0
	a.tokenMu.Unlock()

	return token, tokenHash, nil
}

// ValidateMessageSequence checks sequence number for replay protection
func (a *SimpleAuthenticator) ValidateMessageSequence(agentID string, seqNum int64) bool {
	if seqNum <= 0 {
		return true // No sequence validation for old clients
	}

	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	lastSeq, exists := a.agentSeqNums[agentID]
	if !exists {
		// First message from this agent
		a.agentSeqNums[agentID] = seqNum
		return true
	}

	// Sequence number must be greater than last seen
	if seqNum <= lastSeq {
		return false // Replay or out-of-order
	}

	a.agentSeqNums[agentID] = seqNum
	return true
}

// hashToken creates a hash of the token for storage
func (a *SimpleAuthenticator) hashToken(token string) string {
	h := sha256.Sum256([]byte(token + a.config.SharedSecret))
	return hex.EncodeToString(h[:])
}

// ValidateToken checks if a token matches the shared secret
func (a *SimpleAuthenticator) ValidateToken(token string) error {
	if a.config.SharedSecret == "" {
		// No secret configured - accept any token (development mode)
		return nil
	}

	if token == "" {
		return domain.ErrInvalidToken
	}

	// Split token: format is "payload.signature.timestamp"
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		// Simple shared secret mode
		if token == a.config.SharedSecret {
			return nil
		}
		return domain.ErrInvalidToken
	}

	// Validate timestamp not too old
	timestamp, err := parseTimestamp(parts[2])
	if err != nil {
		return domain.ErrInvalidToken
	}

	if time.Since(timestamp) > a.config.TokenExpiry {
		return domain.ErrTokenExpired
	}

	// Validate HMAC
	expectedSig := a.generateSignature(parts[0], timestamp)
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return domain.ErrInvalidToken
	}

	return nil
}

// ValidateHMAC verifies the HMAC signature of a message
func (a *SimpleAuthenticator) ValidateHMAC(msg *domain.Message, secret string) error {
	if secret == "" || a.config.SharedSecret == "" {
		return nil // No HMAC validation in dev mode
	}

	if msg.HMAC == "" {
		return domain.ErrHMACInvalid
	}

	// Copy message without HMAC for validation
	msgCopy := *msg
	msgCopy.HMAC = ""

	expectedHMAC := a.generateMessageHMAC(&msgCopy, secret)
	if !hmac.Equal([]byte(msg.HMAC), []byte(expectedHMAC)) {
		return domain.ErrHMACInvalid
	}

	return nil
}

// GenerateHMAC generates an HMAC signature for a message
func (a *SimpleAuthenticator) GenerateHMAC(msg *domain.Message, secret string) string {
	return a.generateMessageHMAC(msg, secret)
}

// GenerateToken creates a new signed token
func (a *SimpleAuthenticator) GenerateToken(payload string) string {
	timestamp := time.Now().Unix()
	sig := a.generateSignature(payload, time.Unix(timestamp, 0))
	return fmt.Sprintf("%s.%s.%d", payload, sig, timestamp)
}

// generateSignature creates HMAC for token validation
func (a *SimpleAuthenticator) generateSignature(payload string, timestamp time.Time) string {
	data := fmt.Sprintf("%s:%d", payload, timestamp.Unix())
	return a.hmacString(data)
}

// generateMessageHMAC creates HMAC for message validation
func (a *SimpleAuthenticator) generateMessageHMAC(msg *domain.Message, secret string) string {
	// Serialize message to canonical JSON
	data, _ := json.Marshal(struct {
		Type    domain.MessageType `json:"type"`
		AgentID string             `json:"agent_id,omitempty"`
		Command string             `json:"command,omitempty"`
		Output  string             `json:"output,omitempty"`
		Error   string             `json:"error,omitempty"`
		Ts      int64              `json:"timestamp,omitempty"`
	}{
		Type:    msg.Type,
		AgentID: msg.AgentID,
		Command: msg.Command,
		Output:  msg.Output,
		Error:   msg.Error,
		Ts:      msg.Timestamp,
	})

	return a.hmacWithSecret(string(data), secret)
}

// hmacString creates HMAC using shared secret
func (a *SimpleAuthenticator) hmacString(data string) string {
	return a.hmacWithSecret(data, a.config.SharedSecret)
}

// hmacWithSecret creates HMAC with specified secret
func (a *SimpleAuthenticator) hmacWithSecret(data, secret string) string {
	if secret == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// parseTimestamp parses Unix timestamp string
func parseTimestamp(s string) (time.Time, error) {
	var ts int64
	_, err := fmt.Sscanf(s, "%d", &ts)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(ts, 0), nil
}

// RevokeAgentToken marks an agent's token as invalid
func (a *SimpleAuthenticator) RevokeAgentToken(agentID string) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	delete(a.agentTokens, agentID)
	delete(a.agentSeqNums, agentID)
}
