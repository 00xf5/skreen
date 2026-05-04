package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"scon/server/internal/domain"

	"github.com/gorilla/websocket"
)

// Upgrader websocket upgrader with sensible defaults
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development; restrict in production
		return true
	},
}

// Hub implements domain.WebSocketHub
type Hub struct {
	agents        map[string]*domain.ClientConnection
	controllers   map[*domain.ClientConnection]struct{}
	mu            sync.RWMutex
	registerCh    chan *domain.ClientConnection
	unregisterCh  chan *domain.ClientConnection
	shutdownCh    chan struct{}
	authenticator domain.Authenticator
	registry      domain.AgentRegistry
	router        domain.CommandRouter
	inviteStore   domain.InviteStore
	auditLogger   domain.AuditLogger
	metrics       domain.MetricsCollector
}

// NewHub creates a new WebSocket hub
func NewHub(auth domain.Authenticator, registry domain.AgentRegistry, inviteStore domain.InviteStore, router domain.CommandRouter, auditLogger domain.AuditLogger, metrics domain.MetricsCollector) *Hub {
	return &Hub{
		agents:        make(map[string]*domain.ClientConnection),
		controllers:   make(map[*domain.ClientConnection]struct{}),
		registerCh:    make(chan *domain.ClientConnection, 64),
		unregisterCh:  make(chan *domain.ClientConnection, 64),
		shutdownCh:    make(chan struct{}),
		authenticator: auth,
		registry:      registry,
		inviteStore:   inviteStore,
		router:        router,
		auditLogger:   auditLogger,
		metrics:       metrics,
	}
}

// SetRouter sets the command router (for dependency injection)
func (h *Hub) SetRouter(router domain.CommandRouter) {
	h.router = router
}

// SetAuditLogger sets the audit logger
func (h *Hub) SetAuditLogger(logger domain.AuditLogger) {
	h.auditLogger = logger
}

// SetMetricsCollector sets the metrics collector
func (h *Hub) SetMetricsCollector(metrics domain.MetricsCollector) {
	h.metrics = metrics
}

// Run starts the hub's event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.registerCh:
			h.registerClient(client)

		case client := <-h.unregisterCh:
			h.unregisterClient(client)

		case <-h.shutdownCh:
			return
		}
	}
}

// Shutdown gracefully shuts down the hub
func (h *Hub) Shutdown() {
	close(h.shutdownCh)
}

// RegisterClient registers a new WebSocket client
func (h *Hub) RegisterClient(conn *domain.ClientConnection) {
	h.registerCh <- conn
}

// UnregisterClient removes a WebSocket client
func (h *Hub) UnregisterClient(conn *domain.ClientConnection) {
	h.unregisterCh <- conn
}

// registerClient handles client registration
func (h *Hub) registerClient(client *domain.ClientConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch client.Type {
	case domain.ClientAgent:
		h.agents[client.AgentID] = client
		log.Printf("Agent registered: %s", client.AgentID)

		// ── Broadcast updated agent list to ALL connected controllers ──
		agents := h.registry.GetOnline()
		agentIDs := make([]string, len(agents))
		for i, a := range agents {
			agentIDs[i] = a.ID
		}
		// Also ensure this new agent is included (registry may not have committed yet)
		found := false
		for _, id := range agentIDs {
			if id == client.AgentID {
				found = true
				break
			}
		}
		if !found {
			agentIDs = append(agentIDs, client.AgentID)
		}
		broadcast := domain.Message{
			Type: domain.MsgAgents,
			Data: agentIDs,
		}
		for ctrl := range h.controllers {
			select {
			case ctrl.Send <- broadcast:
			default:
			}
		}

	case domain.ClientController:
		h.controllers[client] = struct{}{}
		log.Printf("Controller connected")

		// Send current agent list to new controller
		agents := h.registry.GetOnline()
		agentIDs := make([]string, len(agents))
		for i, a := range agents {
			agentIDs[i] = a.ID
		}

		msg := domain.Message{
			Type: domain.MsgAgents,
			Data: agentIDs,
		}
		select {
		case client.Send <- msg:
		default:
		}
	}
}

// unregisterClient handles client unregistration
func (h *Hub) unregisterClient(client *domain.ClientConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch client.Type {
	case domain.ClientAgent:
		if _, exists := h.agents[client.AgentID]; exists {
			delete(h.agents, client.AgentID)
			close(client.Send)
			client.Conn.Close()
			log.Printf("Agent unregistered: %s", client.AgentID)

			if agent, err := h.registry.Get(client.AgentID); err == nil {
				agent.IsOnline = false
				agent.Conn = nil
				h.registry.Save()
			}

			// Broadcast updated agent list to controllers
			go func() {
				agents := h.registry.GetOnline()
				agentIDs := make([]string, len(agents))
				for i, a := range agents {
					agentIDs[i] = a.ID
				}
				h.SendToControllers(domain.Message{
					Type: domain.MsgAgents,
					Data: agentIDs,
				})
			}()
		}

	case domain.ClientController:
		if _, exists := h.controllers[client]; exists {
			delete(h.controllers, client)
			close(client.Send)
			client.Conn.Close()
			log.Printf("Controller disconnected")
		}
	}
}

// SendToAgent sends a message to a specific agent
func (h *Hub) SendToAgent(agentID string, msg domain.Message) error {
	h.mu.RLock()
	client, exists := h.agents[agentID]
	h.mu.RUnlock()

	if !exists {
		return domain.ErrAgentNotFound
	}

	select {
	case client.Send <- msg:
		return nil
	default:
		return domain.ErrConnClosed
	}
}

// SendToControllers broadcasts a message to all controllers
func (h *Hub) SendToControllers(msg domain.Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.controllers {
		select {
		case client.Send <- msg:
		default:
			// Channel full, skip
		}
	}
}

// GetClient returns a client connection by agent ID
func (h *Hub) GetClient(agentID string) (*domain.ClientConnection, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	client, exists := h.agents[agentID]
	if !exists {
		return nil, domain.ErrAgentNotFound
	}

	return client, nil
}

// HandleAgentConnection handles WebSocket connections from agents
func (h *Hub) HandleAgentConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := domain.NewClientConnection(conn, domain.ClientAgent)

	// Wait for registration message with timeout
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	var msg domain.Message
	if err := conn.ReadJSON(&msg); err != nil {
		log.Printf("Failed to read registration: %v", err)
		conn.Close()
		return
	}

	if msg.Type != domain.MsgRegister {
		log.Printf("Expected register message, got: %s", msg.Type)
		conn.Close()
		return
	}

	// 1. Validate Invite Code (allows initial registration without a production token or HMAC)
	isNewRegistration := false
	if msg.Code != "" && h.inviteStore != nil {
		if _, err := h.inviteStore.Validate(msg.Code); err == nil {
			isNewRegistration = true
			log.Printf("Valid invite code %s provided by agent %s", msg.Code, msg.AgentID)
		}
	}

	// 2. Validate HMAC (unless it's a new registration with a valid code)
	if !isNewRegistration {
		if err := h.authenticator.ValidateHMAC(&msg, ""); err != nil {
			log.Printf("Invalid HMAC from %s: %v", r.RemoteAddr, err)
			h.sendError(conn, "Invalid HMAC")
			if h.auditLogger != nil {
				h.auditLogger.LogSecurity(msg.AgentID, "invalid_hmac", r.RemoteAddr, err.Error())
			}
			conn.Close()
			return
		}
	}

	// 3. Validate token (unless it's a new invite-based registration)
	if !isNewRegistration {
		if err := h.authenticator.ValidateAgentToken(msg.AgentID, msg.Token); err != nil {
			log.Printf("Authentication failed for agent %s: %v", msg.AgentID, err)
			h.sendError(conn, "Authentication failed")
			conn.Close()
			return
		}
	}

	// Generate per-agent unique token
	token, tokenHash, err := h.authenticator.GenerateAgentToken(msg.AgentID)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		h.sendError(conn, "Token generation failed")
		conn.Close()
		return
	}

	// Clear read deadline
	conn.SetReadDeadline(time.Time{})

	// Build agent struct with metadata
	agent := &domain.Agent{
		ID:        msg.AgentID,
		Conn:      conn,
		Token:     token,
		TokenHash: tokenHash,
		IsOnline:  true,
	}
	if msg.Data != nil {
		if meta, ok := msg.Data.(map[string]interface{}); ok {
			if hn, ok := meta["hostname"].(string); ok {
				agent.Meta.Hostname = hn
			}
			if osName, ok := meta["os"].(string); ok {
				agent.Meta.OS = osName
			}
			if ver, ok := meta["version"].(string); ok {
				agent.Meta.Version = ver
			}
		}
	}

	// 1. Register with registry FIRST — hub must see agent before broadcasting
	if err := h.registry.Register(agent); err != nil {
		if err == domain.ErrAgentExists {
			// Reconnect: update existing entry in place
			if existing, eErr := h.registry.Get(msg.AgentID); eErr == nil {
				existing.Conn = conn
				existing.IsOnline = true
				existing.LastSeen = time.Now()
				h.registry.Save()
				log.Printf("Agent %s reconnected, updated registry", msg.AgentID)
			}
		} else {
			log.Printf("Failed to register agent: %v", err)
			h.sendError(conn, "Registration failed")
			conn.Close()
			return
		}
	}

	// 2. Update token index (agent is now guaranteed in registry)
	h.registry.UpdateToken(msg.AgentID, token)

	// 3. Register with hub (triggers agent-list broadcast to controllers)
	client.AgentID = msg.AgentID
	h.RegisterClient(client)

	// Link session if code provided
	if msg.Code != "" && h.inviteStore != nil {
		h.inviteStore.MarkUsed(msg.Code, msg.AgentID)
	}

	// Send new token to agent
	conn.WriteJSON(domain.Message{
		Type:    domain.MsgRegister,
		AgentID: msg.AgentID,
		Token:   token,
	})

	// Audit log successful registration
	if h.auditLogger != nil {
		h.auditLogger.LogAuth(msg.AgentID, "register", r.RemoteAddr, true, "Per-agent token issued")
	}

	log.Printf("Agent %s registered with unique token", msg.AgentID)

	// Start goroutines for reading and writing
	go h.readAgentMessages(client)
	go h.writeMessages(client)
}

// HandleControllerConnection handles WebSocket connections from controllers
func (h *Hub) HandleControllerConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := domain.NewClientConnection(conn, domain.ClientController)
	h.RegisterClient(client)

	go h.readControllerMessages(client)
	go h.writeMessages(client)
}

// readAgentMessages handles incoming messages from agents
func (h *Hub) readAgentMessages(client *domain.ClientConnection) {
	defer h.UnregisterClient(client)

	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg domain.Message
		if err := client.Conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Agent %s connection error: %v", client.AgentID, err)
			}
			return
		}

		// Update read deadline on any message
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Validate message (replay protection)
		if msg.Nonce != "" {
			if valid := h.registry.ValidateNonce(client.AgentID, msg.Nonce); !valid {
				log.Printf("Replay attack detected from agent %s", client.AgentID)
				if h.auditLogger != nil {
					h.auditLogger.LogSecurity(client.AgentID, "replay_detected", "", "duplicate nonce")
				}
				continue
			}
		}

		// Validate sequence number
		if msg.SeqNum > 0 {
			if valid := h.authenticator.ValidateMessageSequence(client.AgentID, msg.SeqNum); !valid {
				log.Printf("Invalid sequence number from agent %s: %d", client.AgentID, msg.SeqNum)
				if h.auditLogger != nil {
					h.auditLogger.LogSecurity(client.AgentID, "invalid_sequence", "", fmt.Sprintf("seq: %d", msg.SeqNum))
				}
				continue
			}
		}

		// Handle message
		switch msg.Type {
		case domain.MsgHeartbeat:
			h.registry.UpdateHeartbeat(client.AgentID)

		case domain.MsgStatus:
			// Update agent metadata from status report
			if agent, err := h.registry.Get(client.AgentID); err == nil {
				if meta, ok := msg.Data.(map[string]interface{}); ok {
					if priv, ok := meta["privilege"].(string); ok {
						agent.Meta.Privilege = domain.PrivilegeLevel(priv)
					}
					if persist, ok := meta["persistence_enabled"].(bool); ok {
						agent.Meta.PersistenceEnabled = persist
					}
				}
				// Broadcast updated agent info to controllers
				h.SendToControllers(domain.Message{
					Type:    domain.MsgAgents,
					AgentID: client.AgentID,
					Data:    agent.Meta,
				})
			}

		case domain.MsgResult:
			// Handle command result
			result := &domain.CommandResult{
				ID:        msg.AgentID, // This should be command ID in real impl
				AgentID:   client.AgentID,
				Output:    msg.Output,
				Error:     msg.Error,
				Completed: true,
			}
			if h.router != nil {
				h.router.HandleResult(result)
			}

		// Phase 2: WebRTC signaling — forward to all controllers, never inspect payload.
		case domain.MsgWebRTCOffer, domain.MsgICECandidate, domain.MsgStreamReady, domain.MsgStreamStopped:
			msg.AgentID = client.AgentID // ensure agent identity is stamped
			h.SendToControllers(msg)

		// Phase 2.5: Remote Control Agent Responses
		case domain.MsgControlReq: // (Agent approves control)
			msg.AgentID = client.AgentID
			h.SendToControllers(msg)

		// Phase 3: File Transfer (Agent -> Controller)
		case domain.MsgFileReq, domain.MsgFileChunk, domain.MsgFileAck:
			msg.AgentID = client.AgentID
			h.SendToControllers(msg)

		// Phase 3.1: Clipboard + Process (Agent -> Controller)
		case domain.MsgClipboardData, domain.MsgProcessList, domain.MsgProcessKill:
			msg.AgentID = client.AgentID
			h.SendToControllers(msg)

		default:
			log.Printf("Unknown message type from agent %s: %s", client.AgentID, msg.Type)
		}
	}
}

// readControllerMessages handles incoming messages from controllers
func (h *Hub) readControllerMessages(client *domain.ClientConnection) {
	defer h.UnregisterClient(client)

	for {
		var msg domain.Message
		if err := client.Conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Controller connection error: %v", err)
			}
			return
		}

		// Handle message
		switch msg.Type {
		case domain.MsgListAgents:
			agents := h.registry.GetOnline()
			agentIDs := make([]string, len(agents))
			for i, a := range agents {
				agentIDs[i] = a.ID
			}

			response := domain.Message{
				Type: domain.MsgAgents,
				Data: agentIDs,
			}
			select {
			case client.Send <- response:
			default:
			}

		case domain.MsgCommand:
			// Forward command to agent
			if err := h.SendToAgent(msg.AgentID, msg); err != nil {
				h.sendErrorToController(client, fmt.Sprintf("failed to send command: %v", err))
			}

		case domain.MsgTogglePersistence:
			// Forward persistence toggle to agent
			if err := h.SendToAgent(msg.AgentID, msg); err != nil {
				h.sendErrorToController(client, fmt.Sprintf("failed to send toggle: %v", err))
			}

		// Phase 2: WebRTC signaling — forward to agent, never inspect payload.
		case domain.MsgStartStream, domain.MsgStopStream, domain.MsgWebRTCAnswer, domain.MsgICECandidate:
			if msg.AgentID == "" {
				h.sendErrorToController(client, "agent_id required for signaling")
				continue
			}
			if err := h.SendToAgent(msg.AgentID, msg); err != nil {
				h.sendErrorToController(client, fmt.Sprintf("signaling forward failed: %v", err))
			}

		// Phase 2.5: Remote Control Input Events
		case domain.MsgInputMouse, domain.MsgInputKeyboard, domain.MsgControlReq, domain.MsgControlStop:
			if msg.AgentID == "" {
				h.sendErrorToController(client, "agent_id required for control")
				continue
			}
			if err := h.SendToAgent(msg.AgentID, msg); err != nil {
				// Don't spam controller with errors for high-freq mouse moves
				if msg.Type != domain.MsgInputMouse {
					h.sendErrorToController(client, fmt.Sprintf("control forward failed: %v", err))
				}
			}

		// Phase 3: File Transfer (Controller -> Agent)
		case domain.MsgFileReq, domain.MsgFileChunk, domain.MsgFileAck:
			if msg.AgentID == "" {
				h.sendErrorToController(client, "agent_id required for file transfer")
				continue
			}
			if err := h.SendToAgent(msg.AgentID, msg); err != nil {
				h.sendErrorToController(client, fmt.Sprintf("file transfer forward failed: %v", err))
			}

		// Phase 3.1: Clipboard / Process / Quality / Status / Uninstall (Controller -> Agent)
		case domain.MsgClipboardGet, domain.MsgClipboardSet, domain.MsgProcessList, domain.MsgProcessKill, domain.MsgStreamQuality, domain.MsgUninstall:
			if msg.AgentID == "" {
				continue
			}
			h.SendToAgent(msg.AgentID, msg) //nolint:errcheck — best effort

		default:
			log.Printf("Unknown message type from controller: %s", msg.Type)
		}
	}
}

// writeMessages handles outgoing messages to a client
func (h *Hub) writeMessages(client *domain.ClientConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-client.Send:
			if !ok {
				// Channel closed
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.Conn.WriteJSON(msg); err != nil {
				log.Printf("Failed to write message: %v", err)
				return
			}

		case <-ticker.C:
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-client.Done:
			return

		case <-h.shutdownCh:
			return
		}
	}
}

// sendError sends an error message to a connection
func (h *Hub) sendError(conn *websocket.Conn, errMsg string) {
	msg := domain.Message{
		Type:  domain.MsgError,
		Error: errMsg,
	}
	data, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, data)
}

// sendErrorToController sends an error to a specific controller
func (h *Hub) sendErrorToController(client *domain.ClientConnection, errMsg string) {
	select {
	case client.Send <- domain.Message{
		Type:  domain.MsgError,
		Error: errMsg,
	}:
	default:
	}
}
