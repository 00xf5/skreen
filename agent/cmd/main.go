package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"scon/agent/internal/config"
	"scon/agent/internal/connection"
	"scon/agent/internal/executor"
	"scon/agent/internal/heartbeat"
	"scon/agent/internal/installer"
	"scon/agent/internal/persistence"
	"scon/agent/internal/privilege"
	"scon/agent/internal/sessionhelper"

	"github.com/google/uuid"
	"github.com/kardianos/service"
)

var (
	Version    = "1.0.0"
	ServerHost = "localhost"
	ServerPort = "8080"
)

const mutexName = "Global\\SkreenAgentMutex"

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
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("could not get executable path: %w", err)
		}
		return h.persistMgr.Enable(exePath)
	}
	return h.persistMgr.Disable()
}

func (h *agentHandler) GetStatus() (string, bool) {
	return string(h.privLevel), h.persistMgr.IsEnabled()
}

func (h *agentHandler) OnUninstall() error {
	log.Println("🗑️ Received remote uninstall request...")
	h.persistMgr.Disable()         // Remove registry Run key
	persistence.SelfDelete()        // Schedule binary for deletion on reboot
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

type program struct {
	cfg     config.AgentConfig
	handler *agentHandler
	connMgr *connection.Manager
	hb      *heartbeat.Heartbeat
	cancel  context.CancelFunc
}

func (p *program) Start(s service.Service) error {
	log.Println("Starting service logic...")
	p.handler = &agentHandler{
		exec:       executor.New(p.cfg.Behavior.CommandTimeout),
		config:     p.cfg,
		persistMgr: persistence.New(),
		privLevel:  privilege.Detect(),
	}

	p.connMgr = connection.NewManager(&p.cfg, p.handler)
	p.hb = heartbeat.New(p.cfg.Behavior.HeartbeatInterval, p.connMgr)
	
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	go p.hb.Start(ctx)
	go p.connMgr.Start()

	return nil
}

func (p *program) Stop(s service.Service) error {
	log.Println("Stopping service logic...")
	if p.cancel != nil {
		p.cancel()
	}
	if p.hb != nil {
		p.hb.Stop()
	}
	if p.connMgr != nil {
		p.connMgr.Stop()
	}
	return nil
}

func main() {
	// Hide console on Windows immediately
	installer.HideConsole()

	isHelper := false
	for _, arg := range os.Args {
		if arg == "-session-helper" {
			isHelper = true
		}
	}

	if isHelper {
		sessionhelper.RunSessionHelper()
		return
	}

	// Parse flags manually to handle service commands
	svcCommand := ""
	installerPath := "" // path of the setup .exe passed by NSIS via -installer flag
	for i, arg := range os.Args {
		if arg == "install" || arg == "uninstall" || arg == "start" || arg == "stop" || arg == "restart" {
			svcCommand = arg
		}
		if arg == "-installer" && i+1 < len(os.Args) {
			installerPath = os.Args[i+1]
		}
	}

	cfg := loadConfig()
	
	// Filename-based auto-configuration.
	// Prefer the installer's own filename (passed by NSIS as -installer "$EXEFILE")
	// since after extraction os.Executable() is just "skreen-agent.exe" with no code in the name.
	fnameSource := installerPath
	if fnameSource == "" {
		if exe, err := os.Executable(); err == nil {
			fnameSource = exe
		}
	}

	if fnameSource != "" {
		fname := filepath.Base(fnameSource)
		// Strip Windows copy suffixes: " (1)", "- Copy"
		fname = regexp.MustCompile(`\s*(\(\d+\)|- Copy)(\.exe)?$`).ReplaceAllString(fname, ".exe")

		re := regexp.MustCompile(`(?i)skreen-agent-setup-([A-Z0-9]{4}-[A-Z0-9]{4})-([a-z0-9][a-z0-9._:-]+[a-z0-9])`)
		if matches := re.FindStringSubmatch(fname); len(matches) > 2 {
			cfg.Code = matches[1]
			host := strings.TrimSuffix(strings.TrimSuffix(matches[2], ".exe"), ".zip")

			// Browsers replace ":" with "_" when downloading files with colons (e.g. localhost:8080)
			host = strings.ReplaceAll(host, "_", ":")

			cfg.Server.Host = host

			// Detect if it's localhost for local testing
			if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
				cfg.Server.TLS = false
				cfg.Server.Port = 8080
			} else {
				cfg.Server.Port = 443
				cfg.Server.TLS = true
			}

			log.Printf("Auto-configured from filename: Code=%s, Host=%s", cfg.Code, cfg.Server.Host)
		}
	}

	// Ensure ID
	if cfg.Agent.ID == "" {
		cfg.Agent.ID = loadOrGenerateID()
	}

	svcConfig := &service.Config{
		Name:        "SkreenAgent",
		DisplayName: "Skreen Remote Agent",
		Description: "Professional Remote Administration and Telemetry Agent",
		Arguments:   []string{"run"}, // Default run command when started by SCM
	}

	prg := &program{
		cfg: cfg,
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	if svcCommand != "" {
		err = service.Control(s, svcCommand)
		if err != nil {
			log.Fatalf("Service control %s failed: %v", svcCommand, err)
		}
		fmt.Printf("Service %s success\n", svcCommand)
		return
	}

	// Single instance mutex for non-service commands
	if svcCommand == "" {
		installer.EnsureSingleInstance(mutexName)
	}

	err = s.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// loadConfig loads configuration from various sources
func loadConfig() config.AgentConfig {
	cfg := config.FromEnv()

	// Config search order:
	// 1. %ProgramData%\Skreen\agent.json  — used by Windows Service (SYSTEM account)
	// 2. %APPDATA%\skreen\agent.json      — used by user-mode runs
	// 3. ./agent.json                      — local dev fallback
	configPaths := []string{
		filepath.Join(getProgramDataDir(), "Skreen", "agent.json"),
		filepath.Join(getConfigDir(), "skreen", "agent.json"),
		"agent.json",
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
		if ServerPort != "8080" && ServerPort != "" {
			fmt.Sscanf(ServerPort, "%d", &cfg.Server.Port)
			cfg.Server.TLS = (cfg.Server.Port == 443)
		}
	}

	log.Printf("Resolved server: %s (TLS=%v)", cfg.GetWebSocketURL(), cfg.Server.TLS)
	return cfg
}

// loadOrGenerateID loads existing ID or generates a new persistent one
func loadOrGenerateID() string {
	// Try ProgramData first (service account), then APPDATA, then local
	idPaths := []string{
		filepath.Join(getProgramDataDir(), "Skreen", "agent.id"),
		filepath.Join(getConfigDir(), "skreen", "agent.id"),
	}

	for _, idFile := range idPaths {
		if data, err := os.ReadFile(idFile); err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data))
		}
	}

	// Generate new UUID
	id := uuid.New().String()

	// Save to ProgramData so both service and user-mode share the same ID
	idFile := filepath.Join(getProgramDataDir(), "Skreen", "agent.id")
	os.MkdirAll(filepath.Dir(idFile), 0700)
	if err := os.WriteFile(idFile, []byte(id), 0600); err != nil {
		log.Printf("Warning: could not save agent ID to ProgramData: %v", err)
		// Fallback to APPDATA
		idFile = filepath.Join(getConfigDir(), "skreen", "agent.id")
		os.MkdirAll(filepath.Dir(idFile), 0700)
		os.WriteFile(idFile, []byte(id), 0600)
	}

	return id
}

// getConfigDir returns the platform-appropriate config directory (user-mode)
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

// getProgramDataDir returns %ProgramData% on Windows (accessible by SYSTEM service account)
func getProgramDataDir() string {
	if dir := os.Getenv("ProgramData"); dir != "" {
		return dir
	}
	// Fallback for non-Windows or if ProgramData is missing
	return getConfigDir()
}
