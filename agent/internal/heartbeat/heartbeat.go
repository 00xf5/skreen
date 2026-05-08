package heartbeat

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"scon/agent/internal/connection"
	"scon/agent/internal/sysinfo"
)

// Heartbeat manages periodic heartbeat messages to the server
type Heartbeat struct {
	interval time.Duration
	sender   Sender
	stopCh   chan struct{}
	running  atomic.Bool
}

// Sender is the interface for sending messages
type Sender interface {
	Send(msg connection.Message) error
	IsConnected() bool
	GetAgentID() string
}

// New creates a new heartbeat manager
func New(interval time.Duration, sender Sender) *Heartbeat {
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	if interval > 60*time.Second {
		interval = 60 * time.Second
	}

	return &Heartbeat{
		interval: interval,
		sender:   sender,
		stopCh:   make(chan struct{}),
	}
}

// Start begins sending periodic heartbeats
func (h *Heartbeat) Start(ctx context.Context) {
	if h.running.Load() {
		return
	}
	h.running.Store(true)

	log.Printf("Starting heartbeat every %v", h.interval)

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Send immediate heartbeat
	h.send()

	for {
		select {
		case <-ticker.C:
			h.send()

		case <-h.stopCh:
			h.running.Store(false)
			return

		case <-ctx.Done():
			h.running.Store(false)
			return
		}
	}
}

// Stop halts the heartbeat
func (h *Heartbeat) Stop() {
	if !h.running.Load() {
		return
	}
	close(h.stopCh)
}

// send transmits a heartbeat message with the current idle time.
func (h *Heartbeat) send() {
	if !h.sender.IsConnected() {
		return
	}

	msg := connection.Message{
		Type:      connection.MsgHeartbeat,
		AgentID:   h.sender.GetAgentID(),
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"idle_seconds": sysinfo.GetIdleSeconds(),
			"stats":        sysinfo.GetSystemStats(),
		},
	}

	if err := h.sender.Send(msg); err != nil {
		log.Printf("Failed to send heartbeat: %v", err)
	}
}

// IsRunning returns true if heartbeat is active
func (h *Heartbeat) IsRunning() bool {
	return h.running.Load()
}
