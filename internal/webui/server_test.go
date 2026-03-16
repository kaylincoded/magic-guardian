package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaylincoded/magic-guardian/internal/store"
)

// --- Mock controller ---

type mockController struct {
	status     BotStatus
	guilds     []GuildInfo
	startErr   error
	leaveErr   error
	startCalls int
	stopCalls  int
	lastToken  string
	lastAppID  string
	lastLeave  string
}

func (m *mockController) Start(token, appID string) error {
	m.startCalls++
	m.lastToken = token
	m.lastAppID = appID
	if m.startErr != nil {
		return m.startErr
	}
	m.status.Running = true
	m.status.Status = "running"
	return nil
}

func (m *mockController) Stop() {
	m.stopCalls++
	m.status.Running = false
	m.status.Status = "stopped"
}

func (m *mockController) Status() BotStatus { return m.status }

func (m *mockController) Guilds() []GuildInfo { return m.guilds }

func (m *mockController) LeaveGuild(guildID string) error {
	m.lastLeave = guildID
	return m.leaveErr
}

// --- Test helpers ---

func setupTestServer(t *testing.T) (*Server, *mockController, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctrl := &mockController{
		status: BotStatus{Status: "stopped"},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv := NewServer(db, ctrl, logger)
	return srv, ctrl, db
}

func startTestServer(t *testing.T, srv *Server) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("GET /", srv.handleIndex)
	mux.HandleFunc("GET /api/status", srv.handleGetStatus)
	mux.HandleFunc("GET /api/config", srv.handleGetConfig)
	mux.HandleFunc("POST /api/config", srv.handleSaveConfig)
	mux.HandleFunc("POST /api/bot/start", srv.handleBotStart)
	mux.HandleFunc("POST /api/bot/stop", srv.handleBotStop)
	mux.HandleFunc("POST /api/config/boot", srv.handleSetBoot)
	mux.HandleFunc("GET /api/guilds", srv.handleGetGuilds)
	mux.HandleFunc("POST /api/guilds/leave", srv.handleLeaveGuild)
	mux.HandleFunc("GET /api/logs", srv.handleLogStream)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func getJSON(t *testing.T, url string, v interface{}) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if v != nil {
		if err := json.Unmarshal(body, v); err != nil {
			t.Fatalf("decode response from %s: %v (body: %s)", url, err, string(body))
		}
	}
	return resp.StatusCode
}

func postJSON(t *testing.T, url string, payload string) (int, map[string]string) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	return resp.StatusCode, result
}

// --- GET /api/status ---

func TestGetStatus_Stopped(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ts := startTestServer(t, srv)

	var status BotStatus
	code := getJSON(t, ts.URL+"/api/status", &status)
	if code != 200 {
		t.Fatalf("status code: got %d, want 200", code)
	}
	if status.Running {
		t.Error("Running: got true, want false")
	}
	if status.Status != "stopped" {
		t.Errorf("Status: got %q, want stopped", status.Status)
	}
}

func TestGetStatus_Running(t *testing.T) {
	srv, ctrl, _ := setupTestServer(t)
	ctrl.status = BotStatus{
		Running:   true,
		Status:    "running",
		Room:      "8GJG",
		Version:   "117",
		ShopCount: 4,
		Uptime:    "2h 15m",
	}
	ts := startTestServer(t, srv)

	var status BotStatus
	getJSON(t, ts.URL+"/api/status", &status)
	if !status.Running {
		t.Error("Running: got false, want true")
	}
	if status.Room != "8GJG" {
		t.Errorf("Room: got %q, want 8GJG", status.Room)
	}
	if status.Version != "117" {
		t.Errorf("Version: got %q, want 117", status.Version)
	}
	if status.ShopCount != 4 {
		t.Errorf("ShopCount: got %d, want 4", status.ShopCount)
	}
	if status.Uptime != "2h 15m" {
		t.Errorf("Uptime: got %q, want 2h 15m", status.Uptime)
	}
}

// --- GET /api/config ---

func TestGetConfig_Empty(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ts := startTestServer(t, srv)

	var cfg map[string]string
	getJSON(t, ts.URL+"/api/config", &cfg)
	// Empty config should return empty map, not error
	if cfg == nil {
		t.Error("expected non-nil map")
	}
}

func TestGetConfig_MasksToken(t *testing.T) {
	srv, _, db := setupTestServer(t)
	db.SetConfig("discord_token", "MTQ4MjU0MDQ0MDQxMzU0MDU0Mw.ABCDEF.xyz")
	db.SetConfig("app_id", "1482540440413540543")
	ts := startTestServer(t, srv)

	var cfg map[string]string
	getJSON(t, ts.URL+"/api/config", &cfg)

	token := cfg["discord_token"]
	if strings.Contains(token, "ABCDEF") {
		t.Errorf("token should be masked, got %q", token)
	}
	if !strings.Contains(token, "...") {
		t.Errorf("token should contain '...', got %q", token)
	}
	if token[:4] != "MTQ4" {
		t.Errorf("token should start with first 4 chars, got %q", token[:4])
	}

	if cfg["app_id"] != "1482540440413540543" {
		t.Errorf("app_id: got %q, want 1482540440413540543", cfg["app_id"])
	}
}

// --- POST /api/config ---

func TestSaveConfig_Valid(t *testing.T) {
	srv, _, db := setupTestServer(t)
	ts := startTestServer(t, srv)

	code, result := postJSON(t, ts.URL+"/api/config",
		`{"discord_token":"test-token-12345678","app_id":"12345"}`)

	if code != 200 {
		t.Fatalf("status: got %d, want 200", code)
	}
	if result["status"] != "saved" {
		t.Errorf("status: got %q, want saved", result["status"])
	}

	// Verify actually stored
	token, _ := db.GetConfig("discord_token")
	if token != "test-token-12345678" {
		t.Errorf("stored token: got %q, want test-token-12345678", token)
	}
	appID, _ := db.GetConfig("app_id")
	if appID != "12345" {
		t.Errorf("stored app_id: got %q, want 12345", appID)
	}
}

func TestSaveConfig_MissingFields(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ts := startTestServer(t, srv)

	code, result := postJSON(t, ts.URL+"/api/config", `{"discord_token":"","app_id":""}`)
	if code != 400 {
		t.Fatalf("status: got %d, want 400", code)
	}
	if result["error"] == "" {
		t.Error("expected error message")
	}
}

func TestSaveConfig_MaskedTokenNotOverwritten(t *testing.T) {
	srv, _, db := setupTestServer(t)
	db.SetConfig("discord_token", "original-real-token-value")
	ts := startTestServer(t, srv)

	// Send the masked version back -- should NOT overwrite
	postJSON(t, ts.URL+"/api/config",
		`{"discord_token":"orig...alue","app_id":"12345"}`)

	token, _ := db.GetConfig("discord_token")
	if token != "original-real-token-value" {
		t.Errorf("masked token should not overwrite original, got %q", token)
	}
}

func TestSaveConfig_InvalidJSON(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ts := startTestServer(t, srv)

	code, _ := postJSON(t, ts.URL+"/api/config", `{not json}`)
	if code != 400 {
		t.Fatalf("status: got %d, want 400", code)
	}
}

// --- POST /api/bot/start ---

func TestBotStart_Success(t *testing.T) {
	srv, ctrl, db := setupTestServer(t)
	db.SetConfig("discord_token", "real-token")
	db.SetConfig("app_id", "12345")
	ts := startTestServer(t, srv)

	code, result := postJSON(t, ts.URL+"/api/bot/start", `{}`)
	if code != 200 {
		t.Fatalf("status: got %d, want 200", code)
	}
	if result["status"] != "started" {
		t.Errorf("status: got %q, want started", result["status"])
	}
	if ctrl.startCalls != 1 {
		t.Errorf("startCalls: got %d, want 1", ctrl.startCalls)
	}
	if ctrl.lastToken != "real-token" {
		t.Errorf("token passed: got %q, want real-token", ctrl.lastToken)
	}
	if ctrl.lastAppID != "12345" {
		t.Errorf("appID passed: got %q, want 12345", ctrl.lastAppID)
	}
}

func TestBotStart_AlreadyRunning(t *testing.T) {
	srv, ctrl, db := setupTestServer(t)
	ctrl.status.Running = true
	db.SetConfig("discord_token", "tok")
	db.SetConfig("app_id", "id")
	ts := startTestServer(t, srv)

	code, result := postJSON(t, ts.URL+"/api/bot/start", `{}`)
	if code != 409 {
		t.Fatalf("status: got %d, want 409", code)
	}
	if !strings.Contains(result["error"], "already running") {
		t.Errorf("error: got %q, want 'already running'", result["error"])
	}
}

func TestBotStart_NoCredentials(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ts := startTestServer(t, srv)

	code, result := postJSON(t, ts.URL+"/api/bot/start", `{}`)
	if code != 400 {
		t.Fatalf("status: got %d, want 400", code)
	}
	if result["error"] == "" {
		t.Error("expected error about missing credentials")
	}
}

func TestBotStart_ControllerError(t *testing.T) {
	srv, ctrl, db := setupTestServer(t)
	ctrl.startErr = fmt.Errorf("connection refused")
	db.SetConfig("discord_token", "tok")
	db.SetConfig("app_id", "id")
	ts := startTestServer(t, srv)

	code, result := postJSON(t, ts.URL+"/api/bot/start", `{}`)
	if code != 500 {
		t.Fatalf("status: got %d, want 500", code)
	}
	if !strings.Contains(result["error"], "connection refused") {
		t.Errorf("error: got %q, want to contain 'connection refused'", result["error"])
	}
}

// --- POST /api/bot/stop ---

func TestBotStop(t *testing.T) {
	srv, ctrl, _ := setupTestServer(t)
	ts := startTestServer(t, srv)

	code, result := postJSON(t, ts.URL+"/api/bot/stop", `{}`)
	if code != 200 {
		t.Fatalf("status: got %d, want 200", code)
	}
	if result["status"] != "stopped" {
		t.Errorf("status: got %q, want stopped", result["status"])
	}
	if ctrl.stopCalls != 1 {
		t.Errorf("stopCalls: got %d, want 1", ctrl.stopCalls)
	}
}

// --- POST /api/config/boot ---

func TestSetBoot_Enable(t *testing.T) {
	srv, _, db := setupTestServer(t)
	ts := startTestServer(t, srv)

	code, _ := postJSON(t, ts.URL+"/api/config/boot", `{"enabled":true}`)
	if code != 200 {
		t.Fatalf("status: got %d, want 200", code)
	}

	val, _ := db.GetConfig("start_on_boot")
	if val != "true" {
		t.Errorf("stored: got %q, want true", val)
	}
}

func TestSetBoot_Disable(t *testing.T) {
	srv, _, db := setupTestServer(t)
	db.SetConfig("start_on_boot", "true")
	ts := startTestServer(t, srv)

	postJSON(t, ts.URL+"/api/config/boot", `{"enabled":false}`)

	val, _ := db.GetConfig("start_on_boot")
	if val != "false" {
		t.Errorf("stored: got %q, want false", val)
	}
}

// --- GET /api/guilds ---

func TestGetGuilds_NotRunning(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ts := startTestServer(t, srv)

	var guilds []GuildInfo
	getJSON(t, ts.URL+"/api/guilds", &guilds)
	if len(guilds) != 0 {
		t.Errorf("expected empty when not running, got %d", len(guilds))
	}
}

func TestGetGuilds_Running(t *testing.T) {
	srv, ctrl, _ := setupTestServer(t)
	ctrl.status.Running = true
	ctrl.guilds = []GuildInfo{
		{ID: "guild1", Name: "Test Server", Icon: "abc123"},
		{ID: "guild2", Name: "Other Server", Icon: ""},
	}
	ts := startTestServer(t, srv)

	var guilds []GuildInfo
	getJSON(t, ts.URL+"/api/guilds", &guilds)
	if len(guilds) != 2 {
		t.Fatalf("expected 2 guilds, got %d", len(guilds))
	}
	if guilds[0].ID != "guild1" || guilds[0].Name != "Test Server" || guilds[0].Icon != "abc123" {
		t.Errorf("guild[0]: got %+v", guilds[0])
	}
	if guilds[1].ID != "guild2" || guilds[1].Name != "Other Server" {
		t.Errorf("guild[1]: got %+v", guilds[1])
	}
}

// --- POST /api/guilds/leave ---

func TestLeaveGuild_Success(t *testing.T) {
	srv, ctrl, _ := setupTestServer(t)
	ts := startTestServer(t, srv)

	code, result := postJSON(t, ts.URL+"/api/guilds/leave", `{"guild_id":"guild1"}`)
	if code != 200 {
		t.Fatalf("status: got %d, want 200", code)
	}
	if result["status"] != "left" {
		t.Errorf("status: got %q, want left", result["status"])
	}
	if ctrl.lastLeave != "guild1" {
		t.Errorf("lastLeave: got %q, want guild1", ctrl.lastLeave)
	}
}

func TestLeaveGuild_MissingID(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ts := startTestServer(t, srv)

	code, result := postJSON(t, ts.URL+"/api/guilds/leave", `{}`)
	if code != 400 {
		t.Fatalf("status: got %d, want 400", code)
	}
	if result["error"] == "" {
		t.Error("expected error message")
	}
}

func TestLeaveGuild_ControllerError(t *testing.T) {
	srv, ctrl, _ := setupTestServer(t)
	ctrl.leaveErr = fmt.Errorf("not in guild")
	ts := startTestServer(t, srv)

	code, result := postJSON(t, ts.URL+"/api/guilds/leave", `{"guild_id":"guild1"}`)
	if code != 500 {
		t.Fatalf("status: got %d, want 500", code)
	}
	if !strings.Contains(result["error"], "not in guild") {
		t.Errorf("error: got %q", result["error"])
	}
}

// --- GET / (index) ---

func TestIndex_ReturnsHTML(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	ts := startTestServer(t, srv)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type: got %q, want text/html", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Magic Guardian") {
		t.Error("body should contain 'Magic Guardian'")
	}
}

// --- GET /api/logs (SSE) ---

func TestLogStream_SendsExistingAndNew(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	// Pre-populate log buffer
	srv.LogBuffer().Write("line1")
	srv.LogBuffer().Write("line2")

	ts := startTestServer(t, srv)

	// Start SSE request with timeout
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(ts.URL + "/api/logs")
	if err != nil {
		t.Fatalf("GET /api/logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type: got %q, want text/event-stream", resp.Header.Get("Content-Type"))
	}

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, "data: line1") {
		t.Error("expected 'data: line1' in SSE stream")
	}
	if !strings.Contains(text, "data: line2") {
		t.Error("expected 'data: line2' in SSE stream")
	}
}

// --- LogBuffer ---

func TestLogBuffer_WriteAndRead(t *testing.T) {
	buf := NewLogBuffer(3)
	buf.Write("a")
	buf.Write("b")
	buf.Write("c")

	lines := buf.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Errorf("lines: got %v, want [a b c]", lines)
	}
}

func TestLogBuffer_RingOverflow(t *testing.T) {
	buf := NewLogBuffer(2)
	buf.Write("a")
	buf.Write("b")
	buf.Write("c") // should evict "a"

	lines := buf.Lines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "b" || lines[1] != "c" {
		t.Errorf("lines: got %v, want [b c]", lines)
	}
}

func TestLogBuffer_SubscribeReceivesNew(t *testing.T) {
	buf := NewLogBuffer(10)
	ch := buf.Subscribe()
	defer buf.Unsubscribe(ch)

	buf.Write("hello")

	select {
	case line := <-ch:
		if line != "hello" {
			t.Errorf("got %q, want hello", line)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for subscriber")
	}
}

func TestLogBuffer_UnsubscribeStopsDelivery(t *testing.T) {
	buf := NewLogBuffer(10)
	ch := buf.Subscribe()
	buf.Unsubscribe(ch)

	buf.Write("after unsub")

	select {
	case line := <-ch:
		t.Errorf("should not receive after unsubscribe, got %q", line)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestLogBuffer_LinesReturnsCopy(t *testing.T) {
	buf := NewLogBuffer(10)
	buf.Write("original")

	lines := buf.Lines()
	lines[0] = "mutated"

	original := buf.Lines()
	if original[0] != "original" {
		t.Error("Lines() should return a copy")
	}
}

// --- Server Start/Stop with real listener ---

func TestServer_StartAndStop(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	err := srv.Start("127.0.0.1:0") // port 0 = OS picks
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	err = srv.Stop()
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestServer_PortConflict(t *testing.T) {
	srv1, _, _ := setupTestServer(t)
	err := srv1.Start("127.0.0.1:19876")
	if err != nil {
		t.Fatalf("srv1 Start: %v", err)
	}
	defer srv1.Stop()

	srv2, _, _ := setupTestServer(t)
	err = srv2.Start("127.0.0.1:19876")
	if err == nil {
		srv2.Stop()
		t.Fatal("expected error for port conflict, got nil")
	}
	if !strings.Contains(err.Error(), "address already in use") {
		t.Errorf("error: got %q, want 'address already in use'", err.Error())
	}
}
