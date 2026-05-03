package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"scon/agent/internal/config"
	"scon/agent/internal/connection"
	"scon/agent/internal/executor"
	"scon/agent/internal/heartbeat"
	"scon/agent/internal/installer"
	"scon/agent/internal/persistence"
	"scon/agent/internal/privilege"

	"github.com/google/uuid"
)

var (
	Version    = "1.0.0"
	ServerHost = "localhost" // Can be overridden via -ldflags "-X main.ServerHost=api.skreen.io"
	ServerPort = "8080"      // Can be overridden via -ldflags "-X main.ServerPort=443"
)


// agentHandler implements connection.Handler callbacks
type agentHandler struct {
	exec       *executor.Executor
	config     config.AgentConfig
	persistMgr *persistence.Manager
	privLevel  privilege.Level
}

func (h *agentHandler) OnConnect() {
	log.Println("✅ Connected to server")
}

func (h *agentHandler) OnDisconnect() {
	log.Println("❌ Disconnected from server")
}

func (h *agentHandler) OnCommand(msg connection.Message) (string, error) {
	log.Printf("Executing: %s", msg.Command)
	result := h.exec.Execute(msg.Command)
	return result.Output, fmt.Errorf("%s", result.Error)
}

func (h *agentHandler) OnTogglePersistence(enabled bool) error {
	log.Printf("Persistence toggle: %v", enabled)
	if enabled {
		return h.persistMgr.Enable("")
	}
	return h.persistMgr.Disable()
}

func (h *agentHandler) GetStatus() (string, bool) {
	return string(h.privLevel), h.persistMgr.IsEnabled()
}

func (h *agentHandler) OnUninstall() error {
	log.Println("🗑️ Received remote uninstall request...")
	h.persistMgr.Disable()
	// Optionally call installer.Uninstall() if it implements more complex cleanup
	log.Println("👋 Agent uninstalled, shutting down.")
	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
	return nil
}

func (h *agentHandler) OnError(err error) {
	log.Printf("Connection error: %v", err)
}

func main() {
	if installer.Install() {
		os.Exit(0)
	}

	log.Println("Skreen Agent starting...")

	// Load configuration
	cfg := loadConfig()

	// Parse command line flags for code
	for i, arg := range os.Args {
		if (arg == "-code" || arg == "--code") && i+1 < len(os.Args) {
			cfg.Code = os.Args[i+1]
		}
	}

	// Fallback: Try to extract code and host from filename (e.g. skreen-agent-setup-ABCD-EFGH-api.scon.com.exe)
	if exe, err := os.Executable(); err == nil {
		fname := filepath.Base(exe)
		
		// Pattern 1: Flexible extraction
		// Looks for skreen-agent-setup-[CODE]-[HOST].[any extension]
		// and handles Windows " (1)" suffixes
		reFull := regexp.MustCompile(`(?i)skreen-agent-setup-([A-Z0-9]{4}-[A-Z0-9]{4})-([a-z0-9.-]+)`)
		if matches := reFull.FindStringSubmatch(fname); len(matches) > 2 {
			cfg.Code = matches[1]
			cfg.Server.Host = strings.TrimSuffix(strings.TrimSuffix(matches[2], ".exe"), ".zip")
			// Remove any " (1)" or similar from host
			if idx := strings.Index(cfg.Server.Host, " "); idx != -1 {
				cfg.Server.Host = cfg.Server.Host[:idx]
			}
			cfg.Server.Port = 443
			cfg.Server.TLS = true
			log.Printf("Auto-configured from filename: Code=%s, Host=%s", cfg.Code, cfg.Server.Host)
		} else {
			// Pattern 2: Just Code
			reCode := regexp.MustCompile(`(?i)[-_]([A-Z0-9]{4}-[A-Z0-9]{4})`)
			if matches := reCode.FindStringSubmatch(fname); len(matches) > 1 {
				cfg.Code = matches[1]
				log.Printf("Extracted code from filename: %s", cfg.Code)
			}
		}
	}

	// Ensure agent ID
	if cfg.Agent.ID == "" {
		cfg.Agent.ID = loadOrGenerateID()
	}

	log.Printf("Agent ID: %s", cfg.Agent.ID)
	log.Printf("Hostname: %s", cfg.Agent.Hostname)
	log.Printf("Server: %s", cfg.GetWebSocketURL())

	// Create handler
	handler := &agentHandler{
		exec:       executor.New(cfg.Behavior.CommandTimeout),
		config:     cfg,
		persistMgr: persistence.New(),
		privLevel:  privilege.Detect(),
	}

	log.Printf("Privilege level: %s", handler.privLevel)

	// Create connection manager
	connMgr := connection.NewManager(cfg, handler)

	// Create and start heartbeat
	hb := heartbeat.New(cfg.Behavior.HeartbeatInterval, connMgr)
	ctx, cancel := context.WithCancel(context.Background())
	go hb.Start(ctx)

	// Start connection (with auto-reconnect)
	go connMgr.Start()

	// Wait for interrupt
	log.Println("Press Ctrl+C to exit")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\n🛑 Shutting down...")

	// Cleanup
	cancel()
	hb.Stop()
	connMgr.Stop()

	log.Println("👋 Agent stopped")
}

// loadConfig loads configuration from various sources
func loadConfig() config.AgentConfig {
	cfg := config.FromEnv()

	// Try to load from config file
	configPaths := []string{
		"agent.json",
		filepath.Join(getConfigDir(), "skreen", "agent.json"),
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			if fileCfg, err := config.LoadFromFile(path); err == nil {
				// Merge - env takes precedence
				if cfg.Agent.ID == "" {
					cfg.Agent.ID = fileCfg.Agent.ID
				}
				if cfg.Security.Token == "" {
					cfg.Security.Token = fileCfg.Security.Token
				}
				break
			}
		}
	}

	// Override with baked-in defaults if env/file didn't provide them
	if cfg.Server.Host == "" || cfg.Server.Host == "localhost" {
		cfg.Server.Host = ServerHost
	}
	if cfg.Server.Port == 0 || cfg.Server.Port == 8080 {
		fmt.Sscanf(ServerPort, "%d", &cfg.Server.Port)
	}

	return cfg
}

// loadOrGenerateID loads existing ID or generates a new persistent one
func loadOrGenerateID() string {
	idFile := filepath.Join(getConfigDir(), "skreen", "agent.id")

	// Try to load existing ID
	if data, err := os.ReadFile(idFile); err == nil && len(data) > 0 {
		return string(data)
	}

	// Generate new UUID
	id := uuid.New().String()

	// Save for persistence
	os.MkdirAll(filepath.Dir(idFile), 0700)
	if err := os.WriteFile(idFile, []byte(id), 0600); err != nil {
		log.Printf("Warning: could not save agent ID: %v", err)
	}

	return id
}

// getConfigDir returns the platform-appropriate config directory
func getConfigDir() string {
	if dir := os.Getenv("APPDATA"); dir != "" {
		return dir // Windows
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir // Linux
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config") // Linux fallback
	}
	return "."
}
