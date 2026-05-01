package domain

import "errors"

// Domain errors for consistent error handling
var (
	// Auth errors
	ErrInvalidToken     = errors.New("invalid or missing authentication token")
	ErrTokenExpired     = errors.New("authentication token expired")
	ErrHMACInvalid      = errors.New("invalid HMAC signature")
	
	// Agent errors
	ErrAgentNotFound    = errors.New("agent not found")
	ErrAgentOffline     = errors.New("agent is offline")
	ErrAgentExists      = errors.New("agent already registered")
	ErrAgentTimeout     = errors.New("agent heartbeat timeout")
	
	// Command errors
	ErrCommandEmpty     = errors.New("command cannot be empty")
	ErrCommandTooLong   = errors.New("command exceeds maximum length")
	ErrCommandBlocked   = errors.New("command blocked by policy")
	ErrCommandTimeout   = errors.New("command execution timeout")
	
	// Connection errors
	ErrConnClosed       = errors.New("connection closed")
	ErrConnTimeout      = errors.New("connection timeout")
	ErrMaxConnections   = errors.New("maximum connections reached")
	
	// Protocol errors
	ErrInvalidMessage   = errors.New("invalid message format")
	ErrUnknownType      = errors.New("unknown message type")
	ErrMissingField     = errors.New("required field missing")
)

// IsAuthError returns true for authentication-related errors
func IsAuthError(err error) bool {
	return errors.Is(err, ErrInvalidToken) || 
	       errors.Is(err, ErrTokenExpired) || 
	       errors.Is(err, ErrHMACInvalid)
}

// IsAgentError returns true for agent-related errors
func IsAgentError(err error) bool {
	return errors.Is(err, ErrAgentNotFound) || 
	       errors.Is(err, ErrAgentOffline) || 
	       errors.Is(err, ErrAgentExists) ||
	       errors.Is(err, ErrAgentTimeout)
}
