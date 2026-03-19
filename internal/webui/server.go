package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/kaylincoded/magic-guardian/internal/store"
	"github.com/kaylincoded/magic-guardian/internal/updater"
)

//go:embed static/*
var staticFiles embed.FS

// BotStatus represents the current state of the bot engine.
type BotStatus struct {
	Running   bool   `json:"running"`
	Status    string `json:"status"`
	Uptime    string `json:"uptime,omitempty"`
	Room      string `json:"room,omitempty"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
	ShopCount int    `json:"shopCount,omitempty"`
}

// GuildInfo holds basic information about a Discord guild.
type GuildInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// BotController is the interface the webui uses to start/stop the bot engine.
type BotController interface {
	Start(discordToken, appID string) error
	Stop()
	Status() BotStatus
	Guilds() []GuildInfo
	LeaveGuild(guildID string) error
}

// Server is the HTTP server that serves the web UI and API.
type Server struct {
	store      *store.Store
	controller BotController
	logger     *slog.Logger
	logBuffer  *LogBuffer
	server     *http.Server
	updater    *updater.Checker
}

// NewServer creates a new web UI server.
func NewServer(db *store.Store, controller BotController, logger *slog.Logger) *Server {
	s := &Server{
		store:      db,
		controller: controller,
		logger:     logger,
		logBuffer:  NewLogBuffer(200),
		updater:    updater.NewChecker(),
	}
	return s
}

// SetLogger sets the logger after construction (needed for circular init with MultiHandler).
func (s *Server) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// LogBuffer returns the log buffer for the web UI log handler to write to.
func (s *Server) LogBuffer() *LogBuffer {
	return s.logBuffer
}

// Start starts the HTTP server on the given address.
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// Serve embedded static files
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFiles)))

	// Page routes
	mux.HandleFunc("GET /", s.handleIndex)

	// API routes
	mux.HandleFunc("GET /api/status", s.handleGetStatus)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("POST /api/config", s.handleSaveConfig)
	mux.HandleFunc("POST /api/bot/start", s.handleBotStart)
	mux.HandleFunc("POST /api/bot/stop", s.handleBotStop)
	mux.HandleFunc("POST /api/config/boot", s.handleSetBoot)
	mux.HandleFunc("GET /api/guilds", s.handleGetGuilds)
	mux.HandleFunc("POST /api/guilds/leave", s.handleLeaveGuild)
	mux.HandleFunc("GET /api/logs", s.handleLogStream)

	// Update API routes
	mux.HandleFunc("GET /api/update/check", s.handleUpdateCheck)
	mux.HandleFunc("POST /api/update/dismiss", s.handleUpdateDismiss)
	mux.HandleFunc("POST /api/update/download", s.handleUpdateDownload)
	mux.HandleFunc("POST /api/update/apply", s.handleUpdateApply)
	mux.HandleFunc("POST /api/update/restart", s.handleUpdateRestart)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	s.logger.Info("web UI server starting", "addr", addr)

	// Use a channel to detect early bind failures (e.g. port already in use).
	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("web UI server error", "error", err)
			errCh <- err
		}
	}()

	// Brief window to catch immediate bind failures before returning.
	select {
	case err := <-errCh:
		return fmt.Errorf("web UI failed to start: %w", err)
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	status := s.controller.Status()
	writeJSON(w, status)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetAllConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load config")
		return
	}
	// Never send the full token to the frontend — mask it
	if token, ok := cfg["discord_token"]; ok && len(token) > 8 {
		cfg["discord_token"] = token[:4] + "..." + token[len(token)-4:]
	}
	writeJSON(w, cfg)
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DiscordToken string `json:"discord_token"`
		AppID        string `json:"app_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.DiscordToken == "" || req.AppID == "" {
		writeError(w, http.StatusBadRequest, "discord_token and app_id are required")
		return
	}

	// Only update token if it's not the masked version
	if len(req.DiscordToken) > 8 && req.DiscordToken[4:7] != "..." {
		if err := s.store.SetConfig("discord_token", req.DiscordToken); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save token")
			return
		}
	}

	if err := s.store.SetConfig("app_id", req.AppID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save app_id")
		return
	}

	writeJSON(w, map[string]string{"status": "saved"})
}

func (s *Server) handleSetBoot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	value := "false"
	if req.Enabled {
		value = "true"
	}
	if err := s.store.SetConfig("start_on_boot", value); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save setting")
		return
	}
	writeJSON(w, map[string]string{"status": "saved"})
}

func (s *Server) handleBotStart(w http.ResponseWriter, r *http.Request) {
	status := s.controller.Status()
	if status.Running {
		writeError(w, http.StatusConflict, "bot is already running")
		return
	}

	token, err := s.store.GetConfig("discord_token")
	if err != nil || token == "" {
		writeError(w, http.StatusBadRequest, "discord token not configured")
		return
	}
	appID, err := s.store.GetConfig("app_id")
	if err != nil || appID == "" {
		writeError(w, http.StatusBadRequest, "app ID not configured")
		return
	}

	if err := s.controller.Start(token, appID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start bot: %v", err))
		return
	}

	writeJSON(w, map[string]string{"status": "started"})
}

func (s *Server) handleBotStop(w http.ResponseWriter, r *http.Request) {
	s.controller.Stop()
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (s *Server) handleGetGuilds(w http.ResponseWriter, r *http.Request) {
	status := s.controller.Status()
	if !status.Running {
		writeJSON(w, []GuildInfo{})
		return
	}
	guilds := s.controller.Guilds()
	if guilds == nil {
		guilds = []GuildInfo{}
	}
	writeJSON(w, guilds)
}

func (s *Server) handleLeaveGuild(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GuildID string `json:"guild_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.GuildID == "" {
		writeError(w, http.StatusBadRequest, "guild_id is required")
		return
	}
	if err := s.controller.LeaveGuild(req.GuildID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to leave guild: %v", err))
		return
	}
	writeJSON(w, map[string]string{"status": "left"})
}

func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send existing log lines first
	for _, line := range s.logBuffer.Lines() {
		fmt.Fprintf(w, "data: %s\n\n", line)
	}
	flusher.Flush()

	// Subscribe to new log lines
	ch := s.logBuffer.Subscribe()
	defer s.logBuffer.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case line := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// LogBuffer is a thread-safe ring buffer for log lines with pub/sub.
type LogBuffer struct {
	mu          sync.RWMutex
	lines       []string
	maxLines    int
	subscribers map[chan string]struct{}
}

// NewLogBuffer creates a new log buffer with the given capacity.
func NewLogBuffer(maxLines int) *LogBuffer {
	return &LogBuffer{
		lines:       make([]string, 0, maxLines),
		maxLines:    maxLines,
		subscribers: make(map[chan string]struct{}),
	}
}

// Write adds a log line and notifies subscribers.
func (lb *LogBuffer) Write(line string) {
	lb.mu.Lock()
	if len(lb.lines) >= lb.maxLines {
		lb.lines = lb.lines[1:]
	}
	lb.lines = append(lb.lines, line)
	subs := make([]chan string, 0, len(lb.subscribers))
	for ch := range lb.subscribers {
		subs = append(subs, ch)
	}
	lb.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- line:
		default:
			// drop if subscriber is slow
		}
	}
}

// Lines returns a copy of all buffered log lines.
func (lb *LogBuffer) Lines() []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	cp := make([]string, len(lb.lines))
	copy(cp, lb.lines)
	return cp
}

// Subscribe returns a channel that receives new log lines.
func (lb *LogBuffer) Subscribe() chan string {
	ch := make(chan string, 50)
	lb.mu.Lock()
	lb.subscribers[ch] = struct{}{}
	lb.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel.
func (lb *LogBuffer) Unsubscribe(ch chan string) {
	lb.mu.Lock()
	delete(lb.subscribers, ch)
	lb.mu.Unlock()
}

// Update handlers

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	info, err := s.updater.Check(r.Context())
	if err != nil {
		s.logger.Error("update check failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to check for updates")
		return
	}

	// Check if this version was dismissed
	dismissed, _ := s.store.GetConfig("dismissed_update_version")

	response := map[string]interface{}{
		"available":       info.Available,
		"current_version": info.CurrentVersion,
		"latest_version":  info.LatestVersion,
		"download_url":    info.DownloadURL,
		"release_notes":   info.ReleaseNotes,
		"published_at":    info.PublishedAt,
		"dismissed":       dismissed == info.LatestVersion,
		"is_android":      updater.IsAndroid(),
	}
	writeJSON(w, response)
}

func (s *Server) handleUpdateDismiss(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Version == "" {
		writeError(w, http.StatusBadRequest, "version is required")
		return
	}

	if err := s.store.SetConfig("dismissed_update_version", req.Version); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save dismissed version")
		return
	}

	s.logger.Info("update dismissed", "version", req.Version)
	writeJSON(w, map[string]string{"status": "dismissed"})
}

func (s *Server) handleUpdateDownload(w http.ResponseWriter, r *http.Request) {
	// Get latest release info
	info, err := s.updater.Check(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check for updates")
		return
	}

	if !info.Available || info.DownloadURL == "" {
		writeError(w, http.StatusBadRequest, "no update available")
		return
	}

	// Get download path
	destPath, err := updater.GetDownloadPath()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get download path")
		return
	}

	s.logger.Info("downloading update", "version", info.LatestVersion, "url", info.DownloadURL)

	// Download the update
	err = s.updater.Download(r.Context(), info.DownloadURL, destPath, func(downloaded, total int64) {
		// Could stream progress via SSE, but for now just log
		if total > 0 {
			pct := float64(downloaded) / float64(total) * 100
			s.logger.Debug("download progress", "percent", int(pct))
		}
	})
	if err != nil {
		s.logger.Error("download failed", "error", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("download failed: %v", err))
		return
	}

	s.logger.Info("update downloaded", "version", info.LatestVersion, "path", destPath)
	writeJSON(w, map[string]string{
		"status":  "downloaded",
		"version": info.LatestVersion,
		"path":    destPath,
	})
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	destPath, err := updater.GetDownloadPath()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get download path")
		return
	}

	s.logger.Info("applying update", "path", destPath)

	// Apply the update (replace binary)
	if err := updater.ApplyUpdate(destPath); err != nil {
		s.logger.Error("apply update failed", "error", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to apply update: %v", err))
		return
	}

	// Clear dismissed version since we're updating
	_ = s.store.SetConfig("dismissed_update_version", "")

	s.logger.Info("update applied, restart required")
	writeJSON(w, map[string]string{
		"status":  "applied",
		"message": "Update applied. Please restart the application.",
	})
}

// handleUpdateRestart triggers a process restart.
func (s *Server) handleUpdateRestart(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("restart requested")

	writeJSON(w, map[string]string{
		"status": "restarting",
	})

	// Give time for response to be sent, then exit
	// The service manager (systemd, Android, etc.) should restart us
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}
