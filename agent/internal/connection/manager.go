package connection

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"scon/agent/internal/clipboard"
	"scon/agent/internal/config"
	"scon/agent/internal/control"
	"scon/agent/internal/fs"
	"scon/agent/internal/proc"
	"scon/agent/internal/screenshare"
	"scon/agent/internal/sysinfo"
	"path/filepath"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"os"
	"runtime"
)

// MessageType defines the type of WebSocket message
type MessageType string

const (
	MsgRegister          MessageType = "register"
	MsgHeartbeat         MessageType = "heartbeat"
	MsgCommand           MessageType = "command"
	MsgResult            MessageType = "result"
	MsgError             MessageType = "error"
	MsgTogglePersistence MessageType = "toggle_persistence"
	MsgStatus            MessageType = "status"

	// Phase 2: WebRTC signaling
	MsgStartStream    MessageType = "start_stream"
	MsgStopStream     MessageType = "stop_stream"
	MsgWebRTCOffer    MessageType = "webrtc_offer"
	MsgWebRTCAnswer   MessageType = "webrtc_answer"
	MsgICECandidate   MessageType = "ice_candidate"
	MsgStreamReady    MessageType = "stream_ready"
	MsgStreamStopped  MessageType = "stream_stopped"
	// Phase 2.5: WebRTC signaling
	MsgInputMouse    MessageType = "input_mouse"
	MsgInputKeyboard MessageType = "input_keyboard"
	MsgInputSpecial  MessageType = "input_special"
	MsgBlockInput    MessageType = "block_input"
	MsgSetDisplay    MessageType = "set_display"
	MsgControlReq    MessageType = "control_request"
	MsgControlStop   MessageType = "control_stop"
	MsgSetHiddenMode MessageType = "set_hidden_mode"
	// Phase 3: File Transfer
	MsgFileReq   MessageType = "file_req"
	MsgFileChunk MessageType = "file_chunk"
	MsgFileAck   MessageType = "file_ack"

	// Phase 3.1: Clipboard + Process + Quality
	MsgClipboardGet  MessageType = "clipboard_get"
	MsgClipboardSet  MessageType = "clipboard_set"
	MsgClipboardData MessageType = "clipboard_data"
	MsgProcessList   MessageType = "process_list"
	MsgProcessKill   MessageType = "process_kill"
	MsgStreamQuality MessageType = "stream_quality"
	MsgUninstall     MessageType = "uninstall"
	MsgFileList      MessageType = "file_list"
	MsgFileOp        MessageType = "file_op"
)

// Message is the protocol envelope
type Message struct {
	Type      MessageType `json:"type"`
	AgentID   string      `json:"agent_id,omitempty"`
	Token     string      `json:"token,omitempty"`
	Command   string      `json:"command,omitempty"`
	Output    string      `json:"output,omitempty"`
	Error     string      `json:"error,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`

	// Phase 2: WebRTC signaling payloads
	SDP       string `json:"sdp,omitempty"`
	Candidate string `json:"candidate,omitempty"`

	// Phase 2.5: Remote Control payloads
	InputEvent string  `json:"event,omitempty"`  // "move", "click", "scroll"
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	Button     string  `json:"button,omitempty"` // "left", "right"
	KeyState   string  `json:"state,omitempty"`  // "down", "up"
	Key        string  `json:"key,omitempty"`

	// Phase 3: File Transfer payloads
	TransferID string `json:"transfer_id,omitempty"`
	Action     string `json:"action,omitempty"`      // "upload", "download"
	Path       string `json:"path,omitempty"`        // file path
	NewPath    string `json:"new_path,omitempty"`    // for renames
	FileSize   int64  `json:"file_size,omitempty"`
	ChunkIndex int    `json:"chunk_index,omitempty"`
	ChunkCount int    `json:"chunk_count,omitempty"`
	ChunkData  string `json:"chunk_data,omitempty"`  // base64 encoded chunk

	// Phase 3.1: Clipboard / Process / Quality
	PID       int    `json:"pid,omitempty"`
	Quality   string `json:"quality,omitempty"`
	Code      string `json:"code,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// Handler defines callbacks for connection events
type Handler interface {
	OnConnect()
	OnDisconnect()
	OnCommand(msg Message) (string, error)
	OnTogglePersistence(enabled bool) error
	GetStatus() (privilege string, persistenceEnabled bool)
	OnUninstall() error
	OnError(err error)
}

// Manager handles WebSocket connection with auto-reconnect
type Manager struct {
	config      *config.AgentConfig
	handler     Handler
	conn        *websocket.Conn
	connMu      sync.RWMutex
	connected   atomic.Bool
	sendCh      chan Message
	stopCh      chan struct{}
	reconnectCh chan struct{}
	attempts    int
	attemptsMu  sync.Mutex

	sessionMu  sync.Mutex
	screenSess *screenshare.Session
	controlMgr *control.Manager
	fsMgr      *fs.Manager
}

// NewManager creates a new connection manager
func NewManager(cfg *config.AgentConfig, handler Handler) *Manager {
	m := &Manager{
		config:      cfg,
		handler:     handler,
		sendCh:      make(chan Message, 256),
		stopCh:      make(chan struct{}),
		reconnectCh: make(chan struct{}, 1),
		controlMgr:  control.NewManager(),
	}
	m.fsMgr = fs.NewManager(func(msg interface{}) {
		// Callback from fs manager to send a chunk or ack
		if mmap, ok := msg.(map[string]interface{}); ok {
			outMsg := Message{
				Type:       MessageType(mmap["type"].(string)),
				AgentID:    cfg.Agent.ID,
				TransferID: mmap["transfer_id"].(string),
			}
			if idx, ok := mmap["chunk_index"].(int); ok {
				outMsg.ChunkIndex = idx
			}
			if data, ok := mmap["chunk_data"].(string); ok {
				outMsg.ChunkData = data
			}
			if errStr, ok := mmap["error"].(string); ok {
				outMsg.Error = errStr
			}
			if size, ok := mmap["file_size"].(int64); ok {
				outMsg.FileSize = size
			}
			if count, ok := mmap["chunk_count"].(int); ok {
				outMsg.ChunkCount = count
			}
			m.Send(outMsg)
		}
	})
	return m
}

// Start begins the connection loop with auto-reconnect
func (m *Manager) Start() {
	go m.connectionLoop()
}

// Stop closes the connection and stops reconnection attempts
func (m *Manager) Stop() {
	close(m.stopCh)
	m.Close()
}

// Send sends a message to the server (non-blocking)
func (m *Manager) Send(msg Message) error {
	select {
	case m.sendCh <- msg:
		return nil
	default:
		return fmt.Errorf("send buffer full")
	}
}

// GetAgentID returns the unique agent ID
func (m *Manager) GetAgentID() string {
	return m.config.Agent.ID
}

// IsConnected returns true if currently connected
func (m *Manager) IsConnected() bool {
	return m.connected.Load()
}

// connectionLoop manages the connection lifecycle
func (m *Manager) connectionLoop() {
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		// Try to connect
		if err := m.connect(); err != nil {
			log.Printf("Connection failed: %v", err)
			m.handler.OnError(err)
			m.waitAndRetry()
			continue
		}

		// Connection established - run the connection
		done := make(chan struct{})
		go m.readLoop(done)
		go m.writeLoop(done)

		// Wait for connection to close
		<-done

		// Cleanup
		m.Close()
		m.handler.OnDisconnect()

		// Check if we should reconnect
		select {
		case <-m.stopCh:
			return
		default:
			m.waitAndRetry()
		}
	}
}

// connect establishes a WebSocket connection
func (m *Manager) connect() error {
	u, err := url.Parse(m.config.GetWebSocketURL())
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	log.Printf("Connecting to %s...", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	m.connMu.Lock()
	m.conn = conn
	m.connMu.Unlock()
	m.connected.Store(true)

	// Send registration message
	hostname, _ := os.Hostname()
	regMsg := Message{
		Type:      MsgRegister,
		AgentID:   m.config.Agent.ID,
		Token:     m.config.Security.Token,
		Code:      m.config.Code,
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"hostname":     hostname,
			"os":           runtime.GOOS,
			"version":      "1.0.0",
			"username":     sysinfo.GetUsername(),
			"idle_seconds": sysinfo.GetIdleSeconds(),
			"stats":        sysinfo.GetSystemStats(),
		},
	}

	// Sign registration message if we have a token (reconnect)
	if m.config.Security.Token != "" {
		signData := fmt.Sprintf("%s:%s:%d", regMsg.Type, regMsg.AgentID, regMsg.Timestamp)
		h := hmac.New(sha256.New, []byte(m.config.Security.Token))
		h.Write([]byte(signData))
		regMsg.Signature = hex.EncodeToString(h.Sum(nil))
	}

	if err := conn.WriteJSON(regMsg); err != nil {
		conn.Close()
		return fmt.Errorf("registration failed: %w", err)
	}

	log.Printf("Connected and waiting for registration response...")
	m.resetAttempts()
	return nil
}

// readLoop handles incoming messages
func (m *Manager) readLoop(done chan<- struct{}) {
	defer close(done)

	m.connMu.RLock()
	conn := m.conn
	m.connMu.RUnlock()

	if conn == nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Read error: %v", err)
			}
			return
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Handle message
		switch msg.Type {
		case MsgCommand:
			if m.config.Security.AllowCommands {
				output, err := m.handler.OnCommand(msg)
				result := Message{
					Type:      MsgResult,
					AgentID:   m.config.Agent.ID,
					Output:    output,
					Timestamp: time.Now().Unix(),
				}
				if err != nil {
					result.Error = err.Error()
				}
				m.Send(result)
			} else {
				m.Send(Message{
					Type:    MsgResult,
					AgentID: m.config.Agent.ID,
					Error:   "commands disabled",
				})
			}

		case MsgTogglePersistence:
			// Parse enable/disable from message data
			enabled := false
			if data, ok := msg.Data.(map[string]interface{}); ok {
				if v, ok := data["enabled"].(bool); ok {
					enabled = v
				}
			}

			err := m.handler.OnTogglePersistence(enabled)
			result := Message{
				Type:      MsgStatus,
				AgentID:   m.config.Agent.ID,
				Timestamp: time.Now().Unix(),
			}
			if err != nil {
				result.Error = err.Error()
			} else {
				// Send updated status
				priv, persist := m.handler.GetStatus()
				result.Data = map[string]interface{}{
					"privilege":           priv,
					"persistence_enabled": persist,
				}
			}
			m.Send(result)

		case MsgStartStream:
			m.handleStartStream(msg)

		case MsgStopStream:
			m.handleStopStream()

		case MsgWebRTCAnswer:
			m.sessionMu.Lock()
			sess := m.screenSess
			m.sessionMu.Unlock()
			if sess != nil {
				if err := sess.SetAnswer(msg.SDP); err != nil {
					log.Printf("SetAnswer failed: %v", err)
				} else {
					sess.StartCapture()
					m.Send(Message{
						Type:    MsgStreamReady,
						AgentID: m.config.Agent.ID,
						Data: map[string]interface{}{
							"displays": screenshare.NumDisplays(),
						},
					})
				}
			}

		case MsgICECandidate:
			m.sessionMu.Lock()
			sess := m.screenSess
			m.sessionMu.Unlock()
			if sess != nil && msg.Candidate != "" {
				sess.AddICECandidate(msg.Candidate)
			}

		case MsgControlReq:
			controllerID := "" // Since controller isn't passing its own ID right now, we can just use a dummy or let the manager handle it generally.
			if m.controlMgr.RequestControl(controllerID) {
				m.Send(Message{Type: MsgControlReq, AgentID: m.config.Agent.ID, Output: "approved"})
			} else {
				m.Send(Message{Type: MsgControlReq, AgentID: m.config.Agent.ID, Error: "denied"})
			}

		case MsgControlStop:
			m.controlMgr.StopControl("")
		
		case MsgInputMouse:
			m.controlMgr.HandleMouse(msg.InputEvent, msg.X, msg.Y, msg.Button, msg.KeyState)

		case MsgInputKeyboard:
			m.controlMgr.HandleKeyboard(msg.Key, msg.KeyState)

		case MsgInputSpecial:
			m.controlMgr.HandleSpecialKey(msg.Key)

		case MsgBlockInput:
			if block, ok := msg.Data.(bool); ok {
				m.controlMgr.SetBlockInput(block)
			}

		case MsgSetHiddenMode:
			if hidden, ok := msg.Data.(bool); ok {
				m.controlMgr.SetHiddenMode(hidden)
			}

		case MsgSetDisplay:
			m.sessionMu.Lock()
			sess := m.screenSess
			m.sessionMu.Unlock()
			if sess != nil {
				if idx, ok := msg.Data.(float64); ok {
					sess.SetDisplay(int(idx))
				}
			}

		case MsgFileList:
			files, err := m.fsMgr.ListDir(msg.Path)
			result := Message{
				Type:    MsgFileList,
				AgentID: m.config.Agent.ID,
				Path:    msg.Path,
			}
			if err != nil {
				result.Error = err.Error()
			} else {
				result.Data = files
			}
			m.Send(result)

		case MsgFileOp:
			err := m.fsMgr.FileOp(msg.Action, msg.Path, msg.NewPath)
			result := Message{
				Type:    MsgFileOp,
				AgentID: m.config.Agent.ID,
				Action:  msg.Action,
				Path:    msg.Path,
			}
			if err != nil {
				result.Error = err.Error()
			}
			m.Send(result)

			// Auto-refresh directory if successful
			if err == nil {
				dir := filepath.Dir(msg.Path)
				files, _ := m.fsMgr.ListDir(dir)
				m.Send(Message{
					Type:    MsgFileList,
					AgentID: m.config.Agent.ID,
					Path:    dir,
					Data:    files,
				})
			}

		case MsgFileReq:
			if msg.Action == "cancel" {
				m.fsMgr.CancelTransfer(msg.TransferID)
				continue
			}
			err := m.fsMgr.StartTransfer(msg.TransferID, msg.Action, msg.Path, msg.FileSize, msg.ChunkCount)
			if err != nil {
				m.Send(Message{Type: MsgFileAck, TransferID: msg.TransferID, AgentID: m.config.Agent.ID, Error: err.Error()})
				continue
			}
			// If it's a download request, the agent needs to tell the controller how big the file is and how many chunks
			if msg.Action == "download" {
				t := m.fsMgr.GetTransfer(msg.TransferID)
				if t != nil {
					m.Send(Message{
						Type:       MsgFileReq,
						AgentID:    m.config.Agent.ID,
						TransferID: msg.TransferID,
						Action:     "download",
						FileSize:   t.Size,
						ChunkCount: t.ChunkCount,
					})
				}
			} else {
				// Acknowledge upload request ready
				m.Send(Message{Type: MsgFileAck, TransferID: msg.TransferID, AgentID: m.config.Agent.ID})
			}

		case MsgFileChunk:
			// Controller is sending us a chunk for an upload
			err := m.fsMgr.HandleChunk(msg.TransferID, msg.ChunkIndex, msg.ChunkData)
			if err != nil {
				m.Send(Message{Type: MsgFileAck, TransferID: msg.TransferID, AgentID: m.config.Agent.ID, Error: err.Error()})
			} else {
				m.Send(Message{Type: MsgFileAck, TransferID: msg.TransferID, AgentID: m.config.Agent.ID, ChunkIndex: msg.ChunkIndex})
			}

		case MsgFileAck:
			// Controller acknowledged a chunk we sent, or says it's ready to receive next
			// Send next chunk
			err := m.fsMgr.SendNextChunk(msg.TransferID)
			if err != nil {
				m.Send(Message{Type: MsgFileAck, TransferID: msg.TransferID, AgentID: m.config.Agent.ID, Error: err.Error()})
			}

		case MsgClipboardGet:
			// Controller wants agent's clipboard
			text, err := clipboard.Get()
			if err != nil || text == "" {
				m.Send(Message{Type: MsgClipboardData, AgentID: m.config.Agent.ID, Data: ""})
			} else {
				m.Send(Message{Type: MsgClipboardData, AgentID: m.config.Agent.ID, Data: text})
			}

		case MsgClipboardSet:
			// Controller is pushing text to agent's clipboard
			if str, ok := msg.Data.(string); ok {
				clipboard.Set(str)
			}

		case MsgProcessList:
			procs, err := proc.List()
			if err != nil {
				m.Send(Message{Type: MsgProcessList, AgentID: m.config.Agent.ID, Error: err.Error()})
			} else {
				m.Send(Message{Type: MsgProcessList, AgentID: m.config.Agent.ID, Data: procs})
			}

		case MsgProcessKill:
			if msg.PID > 0 {
				err := proc.Kill(msg.PID)
				if err != nil {
					m.Send(Message{Type: MsgProcessKill, AgentID: m.config.Agent.ID, Error: fmt.Sprintf("kill pid %d: %v", msg.PID, err)})
				} else {
					m.Send(Message{Type: MsgProcessKill, AgentID: m.config.Agent.ID, Output: fmt.Sprintf("killed pid %d", msg.PID)})
				}
			}

		case MsgStreamQuality:
			// Adjust screen capture quality on the fly
			m.sessionMu.Lock()
			sess := m.screenSess
			m.sessionMu.Unlock()
			if sess != nil {
				sess.SetQuality(msg.Quality)
			}

		case MsgUninstall:
			// Agent is being remotely uninstalled
			m.handler.OnUninstall()

		case MsgRegister:
			// Server is issuing a unique per-agent token
			if msg.Token != "" {
				log.Printf("📥 Received unique token from server")
				m.config.Security.Token = msg.Token
				// Persist the token so it survives restarts
				configPath := "agent.json" // Fallback to current dir
				if err := m.config.SaveToFile(configPath); err != nil {
					log.Printf("⚠️  Failed to save token to file: %v", err)
				} else {
					log.Printf("✅ Token persisted to %s", configPath)
				}
				// Signal that we are now fully authenticated and ready
				m.handler.OnConnect()
			}

		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

// writeLoop handles outgoing messages
func (m *Manager) writeLoop(done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	m.connMu.RLock()
	conn := m.conn
	m.connMu.RUnlock()

	if conn == nil {
		return
	}

	for {
		select {
		case msg := <-m.sendCh:
			// Sign message if we have a token
			if m.config.Security.Token != "" && msg.Type != MsgRegister {
				// Create a canonical string for signing (Type + AgentID + Timestamp)
				signData := fmt.Sprintf("%s:%s:%d", msg.Type, msg.AgentID, msg.Timestamp)
				h := hmac.New(sha256.New, []byte(m.config.Security.Token))
				h.Write([]byte(signData))
				msg.Signature = hex.EncodeToString(h.Sum(nil))
			}

			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}

			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("Write error: %v", err)
				return
			}

		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-done:
			return

		case <-m.stopCh:
			return
		}
	}
}

// Close closes the current connection
func (m *Manager) Close() {
	m.connMu.Lock()
	defer m.connMu.Unlock()

	m.connected.Store(false)

	if m.conn != nil {
		m.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		m.conn.Close()
		m.conn = nil
	}

	// Always cleanup screenshare session if connection drops
	m.handleStopStream()
}

// waitAndRetry implements exponential backoff with jitter
func (m *Manager) waitAndRetry() {
	m.attemptsMu.Lock()
	m.attempts++
	attempt := m.attempts
	m.attemptsMu.Unlock()

	// Calculate delay: min + (2^attempt) with jitter
	delay := m.config.Behavior.ReconnectMinDelay.Seconds() * math.Pow(2, float64(attempt-1))
	maxDelay := m.config.Behavior.ReconnectMaxDelay.Seconds()

	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter (0-25%)
	jitter := delay * 0.25 * rand.Float64()
	delay += jitter

	waitTime := time.Duration(delay * float64(time.Second))
	log.Printf("Reconnecting in %v... (attempt %d)", waitTime, attempt)

	time.Sleep(waitTime)
}

// resetAttempts resets the reconnection attempt counter
func (m *Manager) resetAttempts() {
	m.attemptsMu.Lock()
	m.attempts = 0
	m.attemptsMu.Unlock()
}

// handleStartStream initializes the WebRTC screenshare session
func (m *Manager) handleStartStream(msg Message) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	if m.screenSess != nil {
		m.screenSess.Stop()
	}

	// Parse ICE servers from controller
	var iceServers []webrtc.ICEServer
	if data, ok := msg.Data.(map[string]interface{}); ok {
		if servers, ok := data["iceServers"].([]interface{}); ok {
			for _, s := range servers {
				if smap, ok := s.(map[string]interface{}); ok {
					var ice webrtc.ICEServer
					if urls, ok := smap["urls"].([]interface{}); ok {
						for _, u := range urls {
							if str, ok := u.(string); ok {
								ice.URLs = append(ice.URLs, str)
							}
						}
					}
					if user, ok := smap["username"].(string); ok {
						ice.Username = user
					}
					if cred, ok := smap["credential"].(string); ok {
						ice.Credential = cred
					}
					iceServers = append(iceServers, ice)
				}
			}
		}
	}

	sess, err := screenshare.NewSession(
		iceServers,
		func(candidateJSON string) {
			m.Send(Message{
				Type:      MsgICECandidate,
				AgentID:   m.config.Agent.ID,
				Candidate: candidateJSON,
			})
		},
		func(state webrtc.ICEConnectionState) {
			if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateDisconnected {
				m.handleStopStream()
			}
		},
	)
	if err != nil {
		log.Printf("Failed to create screenshare session: %v", err)
		m.Send(Message{Type: MsgError, Error: "Failed to start screenshare"})
		return
	}

	m.screenSess = sess
	sdp, err := sess.CreateOffer()
	if err != nil {
		log.Printf("Failed to create offer: %v", err)
		sess.Stop()
		m.screenSess = nil
		return
	}

	m.Send(Message{
		Type:    MsgWebRTCOffer,
		AgentID: m.config.Agent.ID,
		SDP:     sdp,
	})
}

func (m *Manager) handleStopStream() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	if m.screenSess != nil {
		m.screenSess.Stop()
		m.screenSess = nil
		m.Send(Message{Type: MsgStreamStopped, AgentID: m.config.Agent.ID})
	}
}
