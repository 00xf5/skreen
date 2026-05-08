package domain

// Triggering IDE refresh

import (
	"time"

	"github.com/gorilla/websocket"
)

// MessageType defines the type of WebSocket message
type MessageType string

const (
	MsgRegister          MessageType = "register"
	MsgHeartbeat         MessageType = "heartbeat"
	MsgCommand           MessageType = "command"
	MsgResult            MessageType = "result"
	MsgListAgents        MessageType = "list_agents"
	MsgAgents            MessageType = "agents"
	MsgError             MessageType = "error"
	MsgTogglePersistence MessageType = "toggle_persistence"
	MsgStatus            MessageType = "status"
	MsgMetrics           MessageType = "metrics"
	MsgUninstall         MessageType = "uninstall"

	// Phase 2: WebRTC signaling — server forwards these blindly, never inspects payload.
	MsgStartStream    MessageType = "start_stream"
	MsgStopStream     MessageType = "stop_stream"
	MsgWebRTCOffer    MessageType = "webrtc_offer"
	MsgWebRTCAnswer   MessageType = "webrtc_answer"
	MsgICECandidate   MessageType = "ice_candidate"
	MsgStreamReady    MessageType = "stream_ready"    // agent → controller: stream is up
	MsgStreamStopped  MessageType = "stream_stopped"  // agent → controller: stream ended

	// Phase 2.5: Remote Control signaling
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

	// Phase 3.1: Clipboard + Process + Quality + Status
	MsgClipboardGet  MessageType = "clipboard_get"
	MsgClipboardSet  MessageType = "clipboard_set"
	MsgClipboardData MessageType = "clipboard_data"
	MsgProcessList   MessageType = "process_list"
	MsgProcessKill   MessageType = "process_kill"
	MsgStreamQuality MessageType = "stream_quality"
)

// Message is the base protocol envelope
type Message struct {
	Type      MessageType `json:"type"`
	AgentID   string      `json:"agent_id,omitempty"`
	Token     string      `json:"token,omitempty"`
	Command   string      `json:"command,omitempty"`
	Output    string      `json:"output,omitempty"`
	Error     string      `json:"error,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
	HMAC      string      `json:"hmac,omitempty"`
	Nonce     string      `json:"nonce,omitempty"`   // Replay protection
	SeqNum    int64       `json:"seq_num,omitempty"` // Sequence number for ordering
	// Phase 2: WebRTC signaling payloads (omitempty — zero cost when unused)
	SDP       string `json:"sdp,omitempty"`       // offer / answer
	Candidate string `json:"candidate,omitempty"` // ICE candidate JSON

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
	FileSize   int64  `json:"file_size,omitempty"`
	ChunkIndex int    `json:"chunk_index,omitempty"`
	ChunkCount int    `json:"chunk_count,omitempty"`
	ChunkData  string `json:"chunk_data,omitempty"`  // base64 encoded chunk
	Code       string `json:"code,omitempty"`        // Session invite code
}

// StructuredCommand represents a validated, structured command
type StructuredCommand struct {
	ID            string            `json:"id"`
	Type          string            `json:"type"`   // "exec", "download", "upload", "info"
	Action        string            `json:"action"` // Specific action
	Params        map[string]string `json:"params"` // Command parameters
	Timeout       time.Duration     `json:"timeout"`
	RequiresAdmin bool              `json:"requires_admin"`
	AuditLevel    string            `json:"audit_level"` // "low", "medium", "high"
}

// CommandRequest represents a command to be sent to an agent
type CommandRequest struct {
	ID        string
	AgentID   string
	Command   string
	Timeout   time.Duration
	StartedAt time.Time
}

// AuditLogEntry represents a single audit event
type AuditLogEntry struct {
	Timestamp    time.Time         `json:"timestamp"`
	Level        string            `json:"level"`      // "info", "warning", "error", "critical"
	EventType    string            `json:"event_type"` // "auth", "command", "connection", "system"
	AgentID      string            `json:"agent_id,omitempty"`
	CommandID    string            `json:"command_id,omitempty"`
	ControllerID string            `json:"controller_id,omitempty"`
	Action       string            `json:"action"`
	Details      map[string]string `json:"details"`
	Success      bool              `json:"success"`
	SourceIP     string            `json:"source_ip,omitempty"`
}

// MetricsSnapshot represents system metrics at a point in time
type MetricsSnapshot struct {
	Timestamp      time.Time                `json:"timestamp"`
	TotalAgents    int                      `json:"total_agents"`
	OnlineAgents   int                      `json:"online_agents"`
	OfflineAgents  int                      `json:"offline_agents"`
	CommandsTotal  int64                    `json:"commands_total"`
	CommandsFailed int64                    `json:"commands_failed"`
	AvgLatency     time.Duration            `json:"avg_latency"`
	AgentLatencies map[string]time.Duration `json:"agent_latencies"`
	CommandRates   map[string]float64       `json:"command_rates"` // failure rate per agent
}

// AgentCredentials represents cryptographic identity for an agent
type AgentCredentials struct {
	AgentID    string     `json:"agent_id"`
	PublicKey  []byte     `json:"public_key"`            // Ed25519 public key
	PrivateKey []byte     `json:"private_key,omitempty"` // Server stores hash only
	KeyAlgo    string     `json:"key_algo"`              // "ed25519", "ecdsa", "rsa"
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// Agent represents a connected agent
type Agent struct {
	ID           string
	Conn         *websocket.Conn
	Token        string // Per-agent unique token
	TokenHash    string // Token hash for validation
	IsRevoked    bool   // Token revocation flag
	LastSeen     time.Time
	RegisteredAt time.Time
	IsOnline     bool
	Meta         AgentMeta
	Credentials  *AgentCredentials // Cryptographic identity
	SeqNum       int64             // Last seen sequence number (replay protection)
	SeenNonces   map[string]bool   // Recent nonces for replay detection
	CommandCount int64             // Total commands executed
	FailedCount  int64             // Failed command count
	LastLatency  time.Duration     // Last command latency
}

// PrivilegeLevel indicates the agent's privilege level
type PrivilegeLevel string

const (
	PrivUser   PrivilegeLevel = "user"
	PrivAdmin  PrivilegeLevel = "admin"
	PrivSystem PrivilegeLevel = "system"
)

// SystemStats represents hardware telemetry
type SystemStats struct {
	CPU       string `json:"cpu"`
	RAMTotal  uint64 `json:"ram_total"`
	RAMUsed   uint64 `json:"ram_used"`
	DiskTotal uint64 `json:"disk_total"`
	DiskFree  uint64 `json:"disk_free"`
	Uptime    uint64 `json:"uptime"`
	LocalIP   string `json:"local_ip"`
	PublicIP  string `json:"public_ip"`
}

// AgentMeta contains agent metadata
type AgentMeta struct {
	Hostname           string         `json:"hostname"`
	OS                 string         `json:"os"`
	Version            string         `json:"version"`
	Privilege          PrivilegeLevel `json:"privilege"`
	PersistenceEnabled bool           `json:"persistence_enabled"`
	Username           string         `json:"username"`    // currently logged-in user on the agent machine
	IdleSeconds        int64          `json:"idle_seconds"` // seconds since last user input
	Stats              SystemStats    `json:"stats"`
}

// CommandResult represents command execution result
type CommandResult struct {
	ID          string        `json:"id"`
	AgentID     string        `json:"agent_id"`
	CommandID   string        `json:"command_id"`
	Output      string        `json:"output"`
	Error       string        `json:"error,omitempty"`
	ExitCode    int           `json:"exit_code"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Duration    time.Duration `json:"duration"`
	Completed   bool          `json:"completed"`
}

// ServerMetrics represents real-time backend health
type ServerMetrics struct {
	OnlineAgents      int     `json:"online_agents"`
	TotalAgents       int     `json:"total_agents"`
	ActiveControllers int     `json:"active_controllers"`
	WebSocketLoad     float64 `json:"websocket_load"`
	MemoryUsageBytes  uint64  `json:"memory_usage_bytes"`
	UptimeSeconds     int64   `json:"uptime_seconds"`
}

// ClientType identifies the type of WebSocket client
type ClientType int

const (
	ClientUnknown ClientType = iota
	ClientAgent
	ClientController
)

// ClientConnection wraps a WebSocket connection with metadata
type ClientConnection struct {
	Conn    *websocket.Conn
	Type    ClientType
	AgentID string
	Send    chan Message
	Done    chan struct{}
}

// NewClientConnection creates a new client connection wrapper
func NewClientConnection(conn *websocket.Conn, clientType ClientType) *ClientConnection {
	return &ClientConnection{
		Conn: conn,
		Type: clientType,
		Send: make(chan Message, 256),
		Done: make(chan struct{}),
	}
}
