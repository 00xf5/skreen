package audit

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"scon/server/internal/domain"
)

// Logger handles audit logging with rotation and structured output
type Logger struct {
	mu         sync.RWMutex
	entries    []domain.AuditLogEntry
	file       *os.File
	encoder    *json.Encoder
	maxEntries int
	logPath    string
}

// New creates a new audit logger
func New(logPath string) (*Logger, error) {
	if logPath == "" {
		logPath = "audit.log"
	}

	// Ensure directory exists
	dir := filepath.Dir(logPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create audit log directory: %w", err)
		}
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}

	return &Logger{
		entries:    make([]domain.AuditLogEntry, 0, 1000),
		file:       file,
		encoder:    json.NewEncoder(file),
		maxEntries: 10000,
		logPath:    logPath,
	}, nil
}

// Log records an audit event
func (l *Logger) Log(entry domain.AuditLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Set timestamp if not set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Add to memory buffer
	l.entries = append(l.entries, entry)

	// Flush to disk immediately for critical events
	if entry.Level == "critical" || entry.Level == "error" {
		l.flushUnlocked()
	}

	// Rotate if buffer too large
	if len(l.entries) >= l.maxEntries {
		l.flushUnlocked()
		l.entries = l.entries[:0]
	}
}

// LogAuth records authentication events
func (l *Logger) LogAuth(agentID, action, sourceIP string, success bool, details string) {
	l.Log(domain.AuditLogEntry{
		Level:       l.levelFromSuccess(success),
		EventType:   "auth",
		AgentID:     agentID,
		Action:      action,
		Success:     success,
		SourceIP:    sourceIP,
		Details:     map[string]string{"details": details},
	})
}

// LogCommand records command execution events
func (l *Logger) LogCommand(agentID, commandID, action string, success bool, duration time.Duration, output string) {
	l.Log(domain.AuditLogEntry{
		Level:       l.levelFromSuccess(success),
		EventType:   "command",
		AgentID:     agentID,
		CommandID:   commandID,
		Action:      action,
		Success:     success,
		Details: map[string]string{
			"duration": duration.String(),
			"output":   output,
		},
	})
}

// LogConnection records connection events
func (l *Logger) LogConnection(agentID, action, sourceIP string, success bool) {
	l.Log(domain.AuditLogEntry{
		Level:       l.levelFromSuccess(success),
		EventType:   "connection",
		AgentID:     agentID,
		Action:      action,
		Success:     success,
		SourceIP:    sourceIP,
	})
}

// LogSecurity records security events (replay attacks, invalid tokens, etc.)
func (l *Logger) LogSecurity(agentID, action, sourceIP string, details string) {
	l.Log(domain.AuditLogEntry{
		Level:       "warning",
		EventType:   "security",
		AgentID:     agentID,
		Action:      action,
		Success:     false,
		SourceIP:    sourceIP,
		Details:     map[string]string{"details": details},
	})
}

// LogSystem records system-level events
func (l *Logger) LogSystem(action string, details map[string]string) {
	l.Log(domain.AuditLogEntry{
		Level:     "info",
		EventType: "system",
		Action:    action,
		Success:   true,
		Details:   details,
	})
}

// GetRecent returns recent audit entries
func (l *Logger) GetRecent(count int) []domain.AuditLogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if count > len(l.entries) {
		count = len(l.entries)
	}

	// Return copy in reverse order (newest first)
	result := make([]domain.AuditLogEntry, count)
	for i := 0; i < count; i++ {
		result[i] = l.entries[len(l.entries)-1-i]
	}
	return result
}

// Flush writes all buffered entries to disk
func (l *Logger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.flushUnlocked()
}

// flushUnlocked writes entries to disk (must hold lock)
func (l *Logger) flushUnlocked() error {
	for _, entry := range l.entries {
		if err := l.encoder.Encode(entry); err != nil {
			log.Printf("Failed to write audit entry: %v", err)
		}
	}
	return l.file.Sync()
}

// Rotate closes current log and starts a new one
func (l *Logger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.flushUnlocked(); err != nil {
		return err
	}

	if err := l.file.Close(); err != nil {
		return err
	}

	// Rename current file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	newPath := l.logPath + "." + timestamp
	if err := os.Rename(l.logPath, newPath); err != nil {
		return err
	}

	// Create new file
	file, err := os.OpenFile(l.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	l.file = file
	l.encoder = json.NewEncoder(file)
	l.entries = l.entries[:0]

	return nil
}

// Close flushes and closes the logger
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.flushUnlocked(); err != nil {
		return err
	}
	return l.file.Close()
}

// levelFromSuccess returns appropriate log level
func (l *Logger) levelFromSuccess(success bool) string {
	if success {
		return "info"
	}
	return "error"
}
