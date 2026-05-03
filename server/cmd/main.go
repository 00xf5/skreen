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

	authenticator := auth.NewSimpleAuthenticator(cfg.Auth)
	agentRegistry := registry.NewInMemoryRegistry()
	inviteStore := invite.NewStore()

	auditLogger, _ := audit.New(cfg.Audit.LogPath)
	metricsCollector := metrics.NewCollector()
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

	// ── Setup Consolidated Mux ──
	mux := http.NewServeMux()

	// WebSockets
	mux.HandleFunc("/ws/agent", hub.HandleAgentConnection)
	mux.HandleFunc("/ws/controller", hub.HandleControllerConnection)

	// Static: Join Page
	mux.HandleFunc("/join/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, "join.html")
	})
	mux.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, "join.html")
	})

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// API: Metrics
	mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		snapshot := metricsCollector.GetSnapshot(agentRegistry)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snapshot)
	})

	// API: Audit (Crucial for debugging)
	mux.HandleFunc("/api/audit", func(w http.ResponseWriter, r *http.Request) {
		if auditLogger == nil {
			http.Error(w, "Audit logging disabled", http.StatusServiceUnavailable)
			return
		}
		entries := auditLogger.GetRecent(100)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":   len(entries),
			"entries": entries,
		})
	})

	// API: Agents List
	mux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		agents := agentRegistry.GetAll()
		w.Header().Set("Content-Type", "application/json")
		
		type AgentInfo struct {
			ID       string `json:"id"`
			Online   bool   `json:"online"`
			LastSeen string `json:"last_seen"`
			Hostname string `json:"hostname"`
			OS       string `json:"os"`
		}
		
		infos := make([]AgentInfo, 0, len(agents))
		for _, a := range agents {
			infos = append(infos, AgentInfo{
				ID:       a.ID,
				Online:   a.IsOnline,
				LastSeen: a.LastSeen.Format(time.RFC3339),
				Hostname: a.Meta.Hostname,
				OS:       a.Meta.OS,
			})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":  len(infos),
			"agents": infos,
		})
	})

	// API: Invite Create
	mux.HandleFunc("/api/invite/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions { return }
		var req struct {
			Company    string `json:"company"`
			Technician string `json:"technician"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		sess := inviteStore.Create(req.Company, req.Technician, "Remote Access", 1*time.Hour)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":         sess.Code,
			"company":      sess.Company,
			"technician":   sess.Technician,
			"session_type": sess.SessionType,
			"created_at":   sess.CreatedAt,
			"expires_at":   sess.ExpiresAt,
			"expires_in":   "1 hour",
		})
	})

	// API: Invite Validate
	mux.HandleFunc("/api/invite/validate", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		sess, err := inviteStore.Validate(code)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":   true,
			"company": sess.Company,
			"code":    sess.Code,
		})
	})

	// Download Handler
	mux.HandleFunc("/download/agent", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if _, err := inviteStore.Validate(code); err != nil {
			http.Error(w, "Invalid code", http.StatusForbidden)
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

	// ── Start Server ──
	port := os.Getenv("PORT")
	if port == "" { port = "8080" }
	
	server := &http.Server{
		Addr:    ":" + port,
		Handler: corsMiddleware(mux),
	}

	go func() {
		log.Printf("🚀 SCON Server Listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	server.Shutdown(context.Background())
	agentRegistry.Close()
	hub.Shutdown()
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
			registry.CleanupOffline(timeout)
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
			collector.StoreSnapshot(collector.GetSnapshot(registry))
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
