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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	log.Println("🔧 SCON Server starting...")

	cfg := loadConfig()

	if cfg.Auth.SharedSecret == "" {
		log.Println("⚠️  Warning: No SCON_SECRET set. Running in development mode.")
	}

	authenticator := auth.NewSimpleAuthenticator(cfg.Auth)
	agentRegistry := registry.NewInMemoryRegistry()
	inviteStore := invite.NewStore()

	auditLogger, err := audit.New(cfg.Audit.LogPath)
	if err != nil {
		log.Printf("Warning: Failed to create audit logger: %v", err)
		auditLogger = nil
	} else {
		log.Printf("✅ Audit logging to: %s", cfg.Audit.LogPath)
		defer auditLogger.Close()
	}

	metricsCollector := metrics.NewCollector()
	log.Println("✅ Metrics collector initialized")

	hub := ws.NewHub(authenticator, agentRegistry, inviteStore, nil, auditLogger, metricsCollector)

	cmdRouter, err := commands.NewRouter(cfg.Commands, agentRegistry, hub)
	if err != nil {
		log.Fatalf("Failed to create command router: %v", err)
	}
	hub.SetRouter(cmdRouter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runAgentCleanup(ctx, agentRegistry, cfg.Timeouts.AgentCleanup)
	go runMetricsSnapshot(ctx, metricsCollector, agentRegistry, cfg.Metrics.Interval)
	if auditLogger != nil {
		go runAuditFlush(ctx, auditLogger, cfg.Audit.FlushInterval)
	}
	go hub.Run()

	// ── Setup Mux ──
	mux := http.NewServeMux()

	// WebSockets
	mux.HandleFunc("/ws/agent", hub.HandleAgentConnection)
	mux.HandleFunc("/ws/controller", hub.HandleControllerConnection)

	// Static
	mux.HandleFunc("/join/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, "join.html")
	})
	mux.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, "join.html")
	})

	// Health
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// API: Metrics
	mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		snapshot := metricsCollector.GetSnapshot(agentRegistry)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"total_agents":%d,"online_agents":%d,"offline_agents":%d,"commands_total":%d,"commands_failed":%d,"avg_latency_ms":%d}`,
			snapshot.TotalAgents, snapshot.OnlineAgents, snapshot.OfflineAgents,
			snapshot.CommandsTotal, snapshot.CommandsFailed, snapshot.AvgLatency.Milliseconds())
	})

	// API: Audit
	mux.HandleFunc("/api/audit", func(w http.ResponseWriter, r *http.Request) {
		if auditLogger == nil {
			http.Error(w, "Audit logging disabled", http.StatusServiceUnavailable)
			return
		}
		entries := auditLogger.GetRecent(100)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"count":%d,"entries":[]}`, len(entries))
	})

	// API: Agents List
	mux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		agents := agentRegistry.GetAll()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"count":%d,"agents":[`, len(agents))
		for i, a := range agents {
			if i > 0 { fmt.Fprint(w, ",") }
			fmt.Fprintf(w, `{"id":"%s","online":%t,"last_seen":"%s"}`,
				a.ID, a.IsOnline, a.LastSeen.Format(time.RFC3339))
		}
		fmt.Fprint(w, "]}")
	})

	// API: Invite Create
	mux.HandleFunc("/api/invite/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodOptions {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Company     string `json:"company"`
			Technician  string `json:"technician"`
			SessionType string `json:"session_type"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Company == "" { req.Company = "SCON Support" }
		if req.Technician == "" { req.Technician = "Technician" }
		
		sess := inviteStore.Create(req.Company, req.Technician, req.SessionType, 10*time.Minute)
		
		controllerURL := os.Getenv("CONTROLLER_URL")
		if controllerURL == "" { controllerURL = "http://localhost:3000" }

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":         sess.Code,
			"join_url":     fmt.Sprintf("%s/join/%s", controllerURL, sess.Code),
			"expires_in":   "10 minutes",
			"company":      sess.Company,
			"technician":   sess.Technician,
			"session_type": sess.SessionType,
		})
	})

	// API: Invite Validate
	mux.HandleFunc("/api/invite/validate", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		sess, err := inviteStore.Validate(code)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":        true,
			"company":      sess.Company,
			"technician":   sess.Technician,
			"session_type": sess.SessionType,
			"expires_at":   sess.ExpiresAt.Unix(),
		})
	})

	// Download Handler
	mux.HandleFunc("/download/agent", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if _, err := inviteStore.Validate(code); err != nil {
			http.Error(w, "Invalid or expired code", http.StatusForbidden)
			return
		}
		agentPath := "skreen-agent-setup.exe"
		if _, err := os.Stat(agentPath); err != nil {
			agentPath = "skreen-agent.exe"
		}
		downloadName := fmt.Sprintf("skreen-agent-setup-%s-%s.exe", code, r.Host)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", downloadName))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, agentPath)
	})

	// ── Server Start ──
	port := os.Getenv("PORT")
	if port == "" { port = "8080" }
	
	server := &http.Server{
		Addr:    ":" + port,
		Handler: corsMiddleware(mux),
	}

	go func() {
		log.Printf("🚀 Server listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\n🛑 Shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	agentRegistry.Close()
	hub.Shutdown()
	log.Println("👋 Server stopped")
}

func loadConfig() config.ServerConfig {
	cfg := config.FromEnv()
	if cfg.Auth.SharedSecret == "" {
		cfg.Auth.SharedSecret = os.Getenv("SCON_SECRET")
	}
	return cfg
}

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

func runMetricsSnapshot(ctx context.Context, collector *metrics.Collector, registry *registry.InMemoryRegistry, interval time.Duration) {
	if interval == 0 { interval = 60 * time.Second }
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			snapshot := collector.GetSnapshot(registry)
			collector.StoreSnapshot(snapshot)
		case <-ctx.Done():
			return
		}
	}
}

func runAuditFlush(ctx context.Context, logger *audit.Logger, interval time.Duration) {
	if interval == 0 { interval = 30 * time.Second }
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			logger.Flush()
		case <-ctx.Done():
			logger.Flush()
			return
		}
	}
}
