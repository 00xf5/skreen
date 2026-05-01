package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"encoding/json"

	"scon/server/internal/audit"
	"scon/server/internal/auth"
	"scon/server/internal/commands"
	"scon/server/internal/config"
	"scon/server/internal/invite"
	"scon/server/internal/metrics"
	"scon/server/internal/registry"
	"scon/server/internal/ws"
)

func main() {
	log.Println("🔧 SCON Server starting...")

	// Load configuration
	cfg := loadConfig()

	// Ensure we have a shared secret
	if cfg.Auth.SharedSecret == "" {
		log.Println("⚠️  Warning: No SCON_SECRET set. Running in development mode (no auth).")
		log.Println("   Set SCON_SECRET environment variable for production.")
	}

	// Initialize components (dependency injection)
	authenticator := auth.NewSimpleAuthenticator(cfg.Auth)
	agentRegistry := registry.NewInMemoryRegistry()
	inviteStore := invite.NewStore()

	// Initialize audit logger
	auditLogger, err := audit.New(cfg.Audit.LogPath)
	if err != nil {
		log.Printf("Warning: Failed to create audit logger: %v", err)
		auditLogger = nil
	} else {
		log.Printf("✅ Audit logging to: %s", cfg.Audit.LogPath)
		defer auditLogger.Close()
	}

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector()
	log.Println("✅ Metrics collector initialized")

	// Command router needs the hub, but hub needs router - use a two-step init
	hub := ws.NewHub(authenticator, agentRegistry, nil, auditLogger, metricsCollector)

	cmdRouter, err := commands.NewRouter(cfg.Commands, agentRegistry, hub)
	if err != nil {
		log.Fatalf("Failed to create command router: %v", err)
	}
	hub.SetRouter(cmdRouter) // Fix circular dependency

	// Start background tasks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start agent cleanup task
	go runAgentCleanup(ctx, agentRegistry, cfg.Timeouts.AgentCleanup)

	// Start metrics snapshot task
	go runMetricsSnapshot(ctx, metricsCollector, agentRegistry, cfg.Metrics.Interval)

	// Start audit flush task
	if auditLogger != nil {
		go runAuditFlush(ctx, auditLogger, cfg.Audit.FlushInterval)
	}

	// Start hub event loop
	go hub.Run()

	// Setup HTTP routes
	http.HandleFunc("/ws/agent", hub.HandleAgentConnection)
	http.HandleFunc("/ws/controller", hub.HandleControllerConnection)

	// Serve join page at /join and /join/{code}
	http.HandleFunc("/join/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, "join.html")
	})
	http.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, "join.html")
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Metrics endpoint
	http.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		snapshot := metricsCollector.GetSnapshot(agentRegistry)
		w.Header().Set("Content-Type", "application/json")
		// Simple JSON response
		fmt.Fprintf(w, `{"total_agents":%d,"online_agents":%d,"offline_agents":%d,"commands_total":%d,"commands_failed":%d,"avg_latency_ms":%d}`,
			snapshot.TotalAgents, snapshot.OnlineAgents, snapshot.OfflineAgents,
			snapshot.CommandsTotal, snapshot.CommandsFailed, snapshot.AvgLatency.Milliseconds())
	})

	// Audit log endpoint (recent events)
	http.HandleFunc("/api/audit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if auditLogger == nil {
			http.Error(w, "Audit logging disabled", http.StatusServiceUnavailable)
			return
		}
		entries := auditLogger.GetRecent(100)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"count":%d,"entries":[]}`, len(entries))
	})

	// Agent list endpoint (REST fallback)
	http.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		agents := agentRegistry.GetAll()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"count":%d,"agents":[`, len(agents))
		for i, a := range agents {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, `{"id":"%s","online":%t,"last_seen":"%s"}`,
				a.ID, a.IsOnline, a.LastSeen.Format(time.RFC3339))
		}
		fmt.Fprint(w, "]}")
	})

	// Invite: Create (controller-side)
	http.HandleFunc("/api/invite/create", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Company     string `json:"company"`
			Technician  string `json:"technician"`
			SessionType string `json:"session_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req.Company = "SCON Support"
			req.Technician = "Technician"
			req.SessionType = "Remote Assistance"
		}
		if req.Company == "" { req.Company = "SCON Support" }
		if req.Technician == "" { req.Technician = "Technician" }
		if req.SessionType == "" { req.SessionType = "Remote Assistance" }

		// Use CONTROLLER_URL from env, default to localhost for dev
		controllerURL := os.Getenv("CONTROLLER_URL")
		if controllerURL == "" {
			controllerURL = "http://localhost:3000"
		}

		sess := inviteStore.Create(req.Company, req.Technician, req.SessionType, 10*time.Minute)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":         sess.Code,
			"join_url":     fmt.Sprintf("%s/join/%s", controllerURL, sess.Code),
			"expires_in":   "10 minutes",
			"company":      sess.Company,
			"technician":   sess.Technician,
			"session_type": sess.SessionType,
		})
	})

	// Invite: Validate (join page calls this)
	http.HandleFunc("/api/invite/validate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		code := r.URL.Query().Get("code")
		sess, err := inviteStore.Validate(code)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":        true,
			"company":      sess.Company,
			"technician":   sess.Technician,
			"session_type": sess.SessionType,
			"expires_at":   sess.ExpiresAt.Unix(),
		})
	})

	// Agent download endpoint (serves the agent binary)
	http.HandleFunc("/download/agent", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		code := r.URL.Query().Get("code")
		if _, err := inviteStore.Validate(code); err != nil {
			http.Error(w, "Invalid or expired code", http.StatusForbidden)
			return
		}
		// Serve the NSIS installer if it exists, fall back to raw agent
		agentPath := "skreen-agent-setup.exe"
		downloadName := "skreen-agent-setup.exe"
		if _, err := os.Stat(agentPath); err != nil {
			// Fall back to raw agent binary
			agentPath = "skreen-agent.exe"
			downloadName = "skreen-agent.exe"
			if _, err := os.Stat(agentPath); err != nil {
				http.Error(w, "Agent binary not available", http.StatusNotFound)
				return
			}
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", downloadName))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, agentPath)
	})

	// Create server
	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		// Use PORT from env if available (Render)
		port := os.Getenv("PORT")
		if port == "" {
			port = fmt.Sprintf("%d", cfg.HTTP.Port)
		}
		if port == "" || port == "0" {
			port = "8080"
		}
		
		server.Addr = ":" + port
		log.Printf("🚀 Server listening on ws://0.0.0.0:%s/ws/agent", port)
		log.Printf("🌐 Controller dashboard: ws://0.0.0.0:%s/ws/controller", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\n🛑 Shutting down gracefully...")

	// Shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// Cleanup
	agentRegistry.Close()
	hub.Shutdown()

	log.Println("👋 Server stopped")
}

// loadConfig loads configuration from environment or defaults
func loadConfig() config.ServerConfig {
	cfg := config.FromEnv()

	// Override with environment if not already set
	if cfg.Auth.SharedSecret == "" {
		cfg.Auth.SharedSecret = os.Getenv("SCON_SECRET")
	}

	// Check for config file
	if _, err := os.Stat("config.json"); err == nil {
		fileCfg, err := config.LoadFromFile("config.json")
		if err == nil {
			// Merge: env takes precedence
			if cfg.Auth.SharedSecret == "" {
				cfg.Auth.SharedSecret = fileCfg.Auth.SharedSecret
			}
			if cfg.HTTP.Port == 0 {
				cfg.HTTP.Port = fileCfg.HTTP.Port
			}
		}
	}

	return cfg
}

// runAgentCleanup periodically removes offline agents
func runAgentCleanup(ctx context.Context, registry *registry.InMemoryRegistry, timeout time.Duration) {
	ticker := time.NewTicker(timeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			offline := registry.CleanupOffline(timeout)
			if len(offline) > 0 {
				log.Printf("Cleaned up %d offline agents: %v", len(offline), offline)
			}
		case <-ctx.Done():
			return
		}
	}
}

// runMetricsSnapshot periodically captures and stores metrics
func runMetricsSnapshot(ctx context.Context, collector *metrics.Collector, registry *registry.InMemoryRegistry, interval time.Duration) {
	if interval == 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			snapshot := collector.GetSnapshot(registry)
			collector.StoreSnapshot(snapshot)
			log.Printf("📊 Metrics snapshot: %d agents, %d commands, avg latency: %v",
				snapshot.OnlineAgents, snapshot.CommandsTotal, snapshot.AvgLatency)
		case <-ctx.Done():
			return
		}
	}
}

// runAuditFlush periodically flushes audit logs to disk
func runAuditFlush(ctx context.Context, logger *audit.Logger, interval time.Duration) {
	if interval == 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := logger.Flush(); err != nil {
				log.Printf("Failed to flush audit log: %v", err)
			}
		case <-ctx.Done():
			logger.Flush()
			return
		}
	}
}
