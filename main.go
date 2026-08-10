package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/pkg/browser"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

const defaultPort = 6277

func getPort() int {
	cfg := loadConfig()
	if cfg.Port > 0 {
		return cfg.Port
	}
	return defaultPort
}

func getAddr() string {
	return fmt.Sprintf("localhost:%d", getPort())
}

func getListenAddr() string {
	cfg := loadConfig()
	host := "localhost"
	if cfg.ListenAddress != "" {
		host = cfg.ListenAddress
	}
	return fmt.Sprintf("%s:%d", host, getPort())
}

// Session represents a Claude Code session
type Session struct {
	SessionID   string    `json:"session_id"`
	PID         int       `json:"pid"`
	CWD         string    `json:"cwd"`
	Status      string    `json:"status"`
	LastEvent   string    `json:"last_event"`
	LastUpdated time.Time `json:"last_updated"`
	Question    string    `json:"question,omitempty"`
	ProjectName string    `json:"project_name"`
	CustomName  string    `json:"custom_name,omitempty"`
	TTY         string    `json:"tty,omitempty"`
	LastOutput  string    `json:"last_output,omitempty"`
	LastPrompt  string    `json:"last_prompt,omitempty"`
}

// HistoryMessage is a simplified conversation message
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// HookInput is the JSON received from Claude Code hooks via stdin
type HookInput struct {
	SessionID            string         `json:"session_id"`
	CWD                  string         `json:"cwd"`
	ToolName             string         `json:"tool_name"`
	ToolInput            map[string]any `json:"tool_input"`
	LastAssistantMessage string         `json:"last_assistant_message"`
	Prompt               string         `json:"prompt"`
}

func sessionsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude-tabs", "sessions")
}

func projectName(cwd string) string {
	if cwd == "" {
		return "unknown"
	}
	return filepath.Base(cwd)
}

func main() {
	args := os.Args[1:]

	if len(args) >= 2 && args[0] == "hook" {
		handleHook(args[1:])
		return
	}

	if len(args) > 0 && args[0] == "--server" {
		runServer()
		return
	}

	if len(args) >= 3 && args[0] == "worktree" && args[1] == "create" {
		repo := args[2]
		branch := ""
		if len(args) >= 4 {
			branch = args[3]
		}
		if branch == "" {
			fmt.Fprintln(os.Stderr, "Usage: claude-tabs worktree create <repo> <branch>")
			os.Exit(1)
		}
		if _, _, err := worktreeCreate(repo, branch, ""); err != nil {
			fmt.Fprintf(os.Stderr, "claude-tabs: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Client mode
	if isServerRunning() {
		browser.OpenURL("http://" + getAddr())
		return
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude-tabs: %v\n", err)
		os.Exit(1)
	}
	cmd := exec.Command(exe, "--server")
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "claude-tabs: failed to start server: %v\n", err)
		os.Exit(1)
	}
	cmd.Process.Release()
}

func isServerRunning() bool {
	conn, err := net.DialTimeout("tcp", getAddr(), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// --- Hook Handler ---

func handleHook(args []string) {
	if len(args) == 0 {
		return
	}
	eventType := args[0]

	claudePID := 0
	for i, a := range args {
		if a == "--claude-pid" && i+1 < len(args) {
			claudePID, _ = strconv.Atoi(args[i+1])
		}
	}

	var input HookInput
	json.NewDecoder(os.Stdin).Decode(&input)

	if input.SessionID == "" {
		return
	}

	dir := sessionsDir()
	os.MkdirAll(dir, 0755)

	// Load existing session or create new
	filePath := filepath.Join(dir, input.SessionID+".json")
	var session Session
	if data, err := os.ReadFile(filePath); err == nil {
		json.Unmarshal(data, &session)
	}

	session.SessionID = input.SessionID
	session.CWD = input.CWD
	session.ProjectName = projectName(input.CWD)
	session.LastEvent = eventType
	session.LastUpdated = time.Now()
	if claudePID > 0 {
		session.PID = claudePID
	}

	switch eventType {
	case "SessionStart":
		session.Status = "idle"
		session.Question = ""
		session.LastOutput = ""
		// Mark old sessions with same PID as terminated
		if session.PID > 0 {
			markOldSessionsTerminated(session.PID, session.SessionID)
		}
	case "UserPromptSubmit":
		session.Status = "ai_working"
		session.Question = ""
		session.LastOutput = ""
		session.LastPrompt = input.Prompt
	case "AskUserQuestion":
		session.Status = "waiting_input"
		if q, ok := input.ToolInput["question"].(string); ok {
			session.Question = q
		}
	case "PermissionRequest":
		session.Status = "permission_required"
		session.Question = ""
		// Show tool name and input
		toolInfo := input.ToolName
		if input.ToolInput != nil {
			if b, err := json.Marshal(input.ToolInput); err == nil {
				toolInfo += ": " + string(b)
			}
		}
		session.LastOutput = toolInfo
	case "Stop":
		session.Status = "idle"
		session.Question = ""
		session.LastOutput = input.LastAssistantMessage
	}

	data, _ := json.MarshalIndent(session, "", "  ")
	os.WriteFile(filePath, data, 0644)
}

func markOldSessionsTerminated(pid int, currentSessionID string) {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		if s.PID == pid && s.SessionID != currentSessionID && s.Status != "terminated" {
			s.Status = "terminated"
			s.LastUpdated = time.Now()
			updated, _ := json.MarshalIndent(s, "", "  ")
			os.WriteFile(filepath.Join(dir, e.Name()), updated, 0644)
		}
	}
}

// --- Server ---

type server struct {
	mu         sync.RWMutex
	sessions   map[string]*Session
	clients    map[*websocket.Conn]bool
	clientMu   sync.Mutex
	upgrader   websocket.Upgrader
	pendingTTY  map[string]string // CWD -> TTY (auto-set on session creation)
	pendingName map[string]string // CWD -> Name (auto-set on session creation)
}

func newServer() *server {
	return &server{
		sessions:    make(map[string]*Session),
		clients:     make(map[*websocket.Conn]bool),
		pendingTTY:  make(map[string]string),
		pendingName: make(map[string]string),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *server) loadSessions() {
	dir := sessionsDir()
	os.MkdirAll(dir, 0755)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		s.sessions[session.SessionID] = &session
	}
}

func (s *server) getSessions() []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		result = append(result, *session)
	}
	return result
}

func (s *server) broadcastSessions() {
	sessions := s.getSessions()
	data, _ := json.Marshal(sessions)

	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	for conn := range s.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			delete(s.clients, conn)
		}
	}
}

func (s *server) handleFileChange(filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// File deleted
		name := strings.TrimSuffix(filepath.Base(filePath), ".json")
		s.mu.Lock()
		delete(s.sessions, name)
		s.mu.Unlock()
		s.broadcastSessions()
		return
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return
	}

	s.mu.Lock()
	if session.CWD != "" {
		changed := false
		if tty, ok := s.pendingTTY[session.CWD]; ok && session.TTY == "" {
			session.TTY = tty
			delete(s.pendingTTY, session.CWD)
			changed = true
		}
		if name, ok := s.pendingName[session.CWD]; ok {
			session.ProjectName = name
			delete(s.pendingName, session.CWD)
			changed = true
		}
		if changed {
			if updated, err := json.Marshal(&session); err == nil {
				os.WriteFile(filePath, updated, 0644)
			}
		}
	}
	oldSession := s.sessions[session.SessionID]
	oldStatus := ""
	if oldSession != nil {
		oldStatus = oldSession.Status
	}
	s.sessions[session.SessionID] = &session
	s.mu.Unlock()
	s.broadcastSessions()

	// Auto-save conversation on last_output change
	if session.LastOutput != "" && (oldSession == nil || oldSession.LastOutput != session.LastOutput) {
		entry := ConversationEntry{
			Output:  session.LastOutput,
			Input:   session.LastPrompt,
			SavedAt: time.Now().Format("2006/1/2 15:04:05"),
		}
		entries := loadConversations(session.SessionID)
		if len(entries) == 0 || entries[0].Output != entry.Output {
			entries = append([]ConversationEntry{entry}, entries...)
			saveConversations(session.SessionID, entries)
		}
	}

	// Auto-activate browser on attention status change
	if oldStatus != session.Status {
		cfg := loadConfig()
		if fbc := cfg.FocusBrowserOnAttention; fbc != nil && fbc.Enable {
			statuses := fbc.Statuses
			if len(statuses) == 0 {
				statuses = []string{"waiting_input", "permission_required"}
			}
			for _, st := range statuses {
				if session.Status == st {
					go activateBrowser(cfg)
					break
				}
			}
		}
	}
}

func activateBrowser(cfg Config) {
	var script string
	if cfg.BrowserApp != "" {
		script = fmt.Sprintf(`tell application "%s" to activate`, cfg.BrowserApp)
	} else {
		addr := getAddr()
		script = fmt.Sprintf(`tell application "Google Chrome"
	activate
	repeat with w in windows
		repeat with t in tabs of w
			if URL of t contains "%s" then
				set active tab index of w to (index of t)
				set index of w to 1
				return "found"
			end if
		end repeat
	end repeat
	return "not_found"
end tell`, addr)
	}
	exec.Command("osascript", "-e", script).Run()
}

var inactiveThresholds = []struct {
	Duration time.Duration
	Status   string
}{
	{24 * time.Hour, "inactive_24h"},
	{12 * time.Hour, "inactive_12h"},
	{3 * time.Hour, "inactive_3h"},
	{1 * time.Hour, "inactive_1h"},
}

func inactiveStatus(elapsed time.Duration) string {
	for _, t := range inactiveThresholds {
		if elapsed >= t.Duration {
			return t.Status
		}
	}
	return ""
}

func (s *server) healthCheck() {
	s.mu.Lock()
	changed := false
	now := time.Now()
	for _, session := range s.sessions {
		if strings.HasPrefix(session.Status, "inactive_") {
			// Re-evaluate tier
			newStatus := inactiveStatus(now.Sub(session.LastUpdated))
			if newStatus != session.Status {
				session.Status = newStatus
				changed = true
			}
			continue
		}
		if session.Status == "terminated" {
			continue
		}
		elapsed := now.Sub(session.LastUpdated)
		if newStatus := inactiveStatus(elapsed); newStatus != "" {
			session.Status = newStatus
			changed = true
		}
	}
	s.mu.Unlock()
	if changed {
		s.broadcastSessions()
	}
}

func (s *server) watchSessions() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude-tabs: fsnotify error: %v\n", err)
		return
	}
	defer watcher.Close()

	dir := sessionsDir()
	os.MkdirAll(dir, 0755)
	watcher.Add(dir)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				if strings.HasSuffix(event.Name, ".json") {
					// Small delay to ensure file write is complete
					time.Sleep(50 * time.Millisecond)
					s.handleFileChange(event.Name)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "claude-tabs: watcher error: %v\n", err)
		}
	}
}

func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.clientMu.Lock()
	s.clients[conn] = true
	s.clientMu.Unlock()

	// Send initial data
	sessions := s.getSessions()
	data, _ := json.Marshal(sessions)
	conn.WriteMessage(websocket.TextMessage, data)

	// Keep connection alive, remove on close
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			s.clientMu.Lock()
			delete(s.clients, conn)
			s.clientMu.Unlock()
			conn.Close()
			return
		}
	}
}

func (s *server) handleRenameSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	name := r.URL.Query().Get("name")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	session, ok := s.sessions[id]
	if ok {
		session.CustomName = name
		data, _ := json.MarshalIndent(session, "", "  ")
		os.WriteFile(filepath.Join(sessionsDir(), id+".json"), data, 0644)
	}
	s.mu.Unlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.broadcastSessions()
}

func (s *server) handleSetTTY(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	tty := r.URL.Query().Get("tty")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	// Normalize tty path
	if tty != "" && !strings.HasPrefix(tty, "/dev/") {
		if !strings.HasPrefix(tty, "ttys") {
			tty = "ttys" + tty
		}
		tty = "/dev/" + tty
	}

	s.mu.Lock()
	session, ok := s.sessions[id]
	if ok {
		session.TTY = tty
		data, _ := json.MarshalIndent(session, "", "  ")
		os.WriteFile(filepath.Join(sessionsDir(), id+".json"), data, 0644)
	}
	s.mu.Unlock()

	if ok {
		s.broadcastSessions()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleFocusTerminal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	s.mu.RLock()
	session, ok := s.sessions[id]
	var pid int
	var savedTTY string
	if ok {
		pid = session.PID
		savedTTY = session.TTY
	}
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "session not found", http.StatusBadRequest)
		return
	}

	ts := getTerminalScripts(loadConfig())

	focusTTY := func(tty string) bool {
		script := strings.ReplaceAll(ts.Focus, "{{TTY}}", tty)
		if r, err := exec.Command("osascript", "-e", script).Output(); err == nil {
			return strings.TrimSpace(string(r)) == "found"
		}
		return false
	}

	result := "not_found"

	// Try tty matching by PID
	if pid > 0 {
		if out, err := exec.Command("ps", "-o", "tty=", "-p", strconv.Itoa(pid)).Output(); err == nil {
			tty := "/dev/" + strings.TrimSpace(string(out))
			if focusTTY(tty) {
				result = "found"
			}
		}
	}

	// Fallback: try saved TTY
	if result != "found" && savedTTY != "" {
		if focusTTY(savedTTY) {
			result = "found"
		}
	}

	// Fallback: just activate iTerm2
	if result != "found" {
		exec.Command("osascript", "-e", ts.Activate).Run()
		result = "activated"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"result": result})
}

func handleActivateBrowser(w http.ResponseWriter, r *http.Request) {
	cfg := loadConfig()
	activateBrowser(cfg)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
}

func (s *server) handleScreen(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s.mu.RLock()
	session, ok := s.sessions[id]
	var tty string
	if ok {
		tty = session.TTY
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if !ok || tty == "" {
		json.NewEncoder(w).Encode(map[string]string{"content": ""})
		return
	}

	ts := getTerminalScripts(loadConfig())
	if ts.Screen == "" {
		json.NewEncoder(w).Encode(map[string]string{"content": ""})
		return
	}

	script := strings.ReplaceAll(ts.Screen, "{{TTY}}", tty)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"content": ""})
		return
	}
	cfg := loadConfig()
	maxLines := cfg.ScreenLines
	if maxLines <= 0 {
		maxLines = 20
	}
	content := strings.TrimSpace(string(out))
	lines := strings.Split(content, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	json.NewEncoder(w).Encode(map[string]string{"content": strings.Join(lines, "\n")})
}

func (s *server) handleSendInput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	text := r.URL.Query().Get("text")
	if id == "" || text == "" {
		http.Error(w, "id and text required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	session, ok := s.sessions[id]
	var pid int
	var savedTTY string
	if ok {
		pid = session.PID
		savedTTY = session.TTY
	}
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "session not found", http.StatusBadRequest)
		return
	}

	// Determine tty
	tty := ""
	if pid > 0 {
		if out, err := exec.Command("ps", "-o", "tty=", "-p", strconv.Itoa(pid)).Output(); err == nil {
			tty = "/dev/" + strings.TrimSpace(string(out))
		}
	}
	if tty == "" {
		tty = savedTTY
	}
	if tty == "" {
		http.Error(w, "no tty available", http.StatusBadRequest)
		return
	}

	// Send text + Enter via AppleScript
	ts := getTerminalScripts(loadConfig())
	escapedText := strings.ReplaceAll(text, `"`, `\"`)
	script := strings.ReplaceAll(strings.ReplaceAll(ts.Input, "{{TTY}}", tty), "{{TEXT}}", escapedText)

	result, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		http.Error(w, "AppleScript error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"result": strings.TrimSpace(string(result))})
}

func (s *server) handleSendKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	action := r.URL.Query().Get("action") // "allow", "allow_always", "deny"
	if id == "" || action == "" {
		http.Error(w, "id and action required", http.StatusBadRequest)
		return
	}

	// Build AppleScript commands: down arrows (newline NO) then Enter
	downArrow := `(ASCII character 27) & "[B"`
	var cmds string
	switch action {
	case "allow":
		// Option 1: already selected, just Enter
		cmds = `tell s to write text ""`
	case "allow_always":
		// Option 2: down once + Enter
		cmds = fmt.Sprintf("tell s to write text %s newline NO\n\t\t\t\t\tdelay 0.1\n\t\t\t\t\ttell s to write text \"\"", downArrow)
	case "deny":
		// Option 3: down twice + Enter
		cmds = fmt.Sprintf("tell s to write text %s newline NO\n\t\t\t\t\tdelay 0.1\n\t\t\t\t\ttell s to write text %s newline NO\n\t\t\t\t\tdelay 0.1\n\t\t\t\t\ttell s to write text \"\"", downArrow, downArrow)
	default:
		http.Error(w, "invalid action", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	session, ok := s.sessions[id]
	var pid int
	var savedTTY string
	if ok {
		pid = session.PID
		savedTTY = session.TTY
	}
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "session not found", http.StatusBadRequest)
		return
	}

	tty := ""
	if pid > 0 {
		if out, err := exec.Command("ps", "-o", "tty=", "-p", strconv.Itoa(pid)).Output(); err == nil {
			tty = "/dev/" + strings.TrimSpace(string(out))
		}
	}
	if tty == "" {
		tty = savedTTY
	}
	if tty == "" {
		http.Error(w, "no tty available", http.StatusBadRequest)
		return
	}

	ts := getTerminalScripts(loadConfig())
	script := strings.ReplaceAll(strings.ReplaceAll(ts.Keys, "{{TTY}}", tty), "{{CMDS}}", cmds)

	result, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		http.Error(w, "AppleScript error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update status after successful send
	res := strings.TrimSpace(string(result))
	if res == "sent" {
		s.mu.Lock()
		if sess, ok := s.sessions[id]; ok {
			if action == "allow" || action == "allow_always" {
				sess.Status = "ai_working"
			} else {
				sess.Status = "idle"
			}
			sess.LastUpdated = time.Now()
			s.sessions[id] = sess
			data, _ := json.MarshalIndent(sess, "", "  ")
			os.WriteFile(filepath.Join(sessionsDir(), id+".json"), data, 0644)
		}
		s.mu.Unlock()
		s.broadcastSessions()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"result": res})
}

func encodeCWDPath(cwd string) string {
	return strings.ReplaceAll(cwd, "/", "-")
}

func findTranscriptPath(sessionID, cwd string) string {
	home, _ := os.UserHomeDir()
	claudeDir := filepath.Join(home, ".claude", "projects")

	// Try encoded CWD path
	if cwd != "" {
		encoded := encodeCWDPath(cwd)
		candidate := filepath.Join(claudeDir, encoded, sessionID+".jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Search all project dirs
	entries, err := os.ReadDir(claudeDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(claudeDir, e.Name(), sessionID+".jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func parseTranscript(path string) []HistoryMessage {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var messages []HistoryMessage
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		role := ""
		content := ""

		// Format: {"type": "human"/"assistant", "message": {"content": [{"type":"text","text":"..."}]}}
		if t, ok := obj["type"].(string); ok {
			switch t {
			case "human":
				role = "user"
			case "assistant":
				role = "assistant"
			default:
				continue
			}
		}

		if msg, ok := obj["message"].(map[string]any); ok {
			if contentArr, ok := msg["content"].([]any); ok {
				var parts []string
				for _, c := range contentArr {
					if cm, ok := c.(map[string]any); ok {
						if text, ok := cm["text"].(string); ok {
							parts = append(parts, text)
						}
					}
				}
				content = strings.Join(parts, "\n")
			} else if contentStr, ok := msg["content"].(string); ok {
				content = contentStr
			}
		}

		if role != "" && content != "" {
			messages = append(messages, HistoryMessage{Role: role, Content: content})
		}
	}
	return messages
}

func (s *server) handleHistory(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s.mu.RLock()
	session, ok := s.sessions[id]
	var cwd string
	if ok {
		cwd = session.CWD
	}
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	path := findTranscriptPath(id, cwd)
	if path == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]HistoryMessage{})
		return
	}

	messages := parseTranscript(path)
	if messages == nil {
		messages = []HistoryMessage{}
	}
	// Reverse: newest first
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// sessionWorktreeInfo checks if a session's CWD is under worktree_base and returns worktree/sbx info
func sessionWorktreeInfo(cwd string) (wtPath, sbxName string) {
	if cwd == "" {
		return
	}
	cfg := loadConfig()
	wtBase := cfg.WorktreeBase
	if wtBase == "" {
		ghqRoot, err := exec.Command("ghq", "root").Output()
		if err != nil {
			return
		}
		wtBase = filepath.Join(strings.TrimSpace(string(ghqRoot)), "worktrees")
	}
	rel, err := filepath.Rel(wtBase, cwd)
	if err != nil || strings.HasPrefix(rel, "..") {
		return
	}
	// rel should be "repo/branch" or "repo/branch/..."
	parts := strings.SplitN(rel, string(filepath.Separator), 3)
	if len(parts) < 2 {
		return
	}
	repo, branch := parts[0], parts[1]
	wtPath = filepath.Join(wtBase, repo, branch)
	sbxName = "wt-" + repo + "-" + branch
	return
}

func (s *server) handleDeleteSessionCheck(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	s.mu.RLock()
	session, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		json.NewEncoder(w).Encode(map[string]any{"has_worktree": false, "has_sbx": false})
		return
	}

	wtPath, sbxName := sessionWorktreeInfo(session.CWD)
	hasWorktree := wtPath != "" && dirExists(wtPath)
	hasSbx := false
	if sbxName != "" {
		if err := exec.Command("sbx", "inspect", sbxName).Run(); err == nil {
			hasSbx = true
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"has_worktree": hasWorktree,
		"has_sbx":      hasSbx,
		"worktree_path": wtPath,
		"sbx_name":      sbxName,
	})
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (s *server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	removeWorktree := r.URL.Query().Get("remove_worktree") == "1"
	removeSbx := r.URL.Query().Get("remove_sbx") == "1"

	s.mu.Lock()
	session, ok := s.sessions[id]
	var cwd string
	if ok {
		cwd = session.CWD
	}
	delete(s.sessions, id)
	s.mu.Unlock()

	os.Remove(filepath.Join(sessionsDir(), id+".json"))

	if cwd != "" {
		wtPath, sbxName := sessionWorktreeInfo(cwd)
		if removeSbx && sbxName != "" {
			exec.Command("sbx", "rm", "-f", sbxName).Run()
		}
		if removeWorktree && wtPath != "" {
			// find git repo that owns this worktree
			gitDir, err := exec.Command("git", "-C", wtPath, "rev-parse", "--git-common-dir").Output()
			if err == nil {
				repoDir := filepath.Dir(strings.TrimSpace(string(gitDir)))
				branch := filepath.Base(wtPath)
				exec.Command("git", "-C", repoDir, "worktree", "remove", wtPath, "--force").Run()
				exec.Command("git", "-C", repoDir, "branch", "-D", branch).Run()
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
	s.broadcastSessions()
}

type Preset struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

var defaultPresets = []Preset{
	{Label: "Yes", Text: "yes"},
	{Label: "Commit", Text: "commit して"},
	{Label: "Commit & Push", Text: "commit して push して"},
}


type PluginConfig struct {
	Source  string   `json:"source"`
	Plugins []string `json:"plugins"`
}

type TerminalScripts struct {
	Focus    string `json:"focus"`
	Activate string `json:"activate"`
	Input    string `json:"input"`
	Keys     string `json:"keys"`
	NewTab   string `json:"new_tab"`
	Screen   string `json:"screen"`
}

type FocusBrowserConfig struct {
	Enable   bool     `json:"enable"`
	Statuses []string `json:"statuses"`
}

type Config struct {
	Presets           []Preset                   `json:"presets"`
	WorktreeBase      string                     `json:"worktree_base"`
	SbxTemplate       string                     `json:"sbx_template"`
	SbxDefaultMounts  []string                   `json:"sbx_default_mounts"`
	SbxPostCreateCmds [][]string                 `json:"sbx_post_create_cmds"`
	SbxKits           []string                   `json:"sbx_kits"`
	Plugins           []PluginConfig             `json:"plugins"`
	Terminal          string                     `json:"terminal"`
	TerminalPresets   map[string]TerminalScripts `json:"terminal_presets"`
	RepositoryBase    string                     `json:"repository_base"`
	ScreenLines       int                        `json:"screen_lines"`
	Port              int                        `json:"port"`
	BrowserApp              string                     `json:"browser_app"`
	FocusBrowserOnAttention *FocusBrowserConfig          `json:"focus_browser_on_attention"`
	ListenAddress           string                     `json:"listen_address"`
}

// Conversations

type ConversationEntry struct {
	Output  string `json:"output"`
	Input   string `json:"input,omitempty"`
	SavedAt string `json:"saved_at"`
}

func conversationsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude-tabs", "conversations")
}

func conversationFilePath(sessionID string) string {
	return filepath.Join(conversationsDir(), sessionID+".json")
}

func loadConversations(sessionID string) []ConversationEntry {
	data, err := os.ReadFile(conversationFilePath(sessionID))
	if err != nil {
		return []ConversationEntry{}
	}
	var entries []ConversationEntry
	json.Unmarshal(data, &entries)
	return entries
}

func saveConversations(sessionID string, entries []ConversationEntry) error {
	dir := conversationsDir()
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(conversationFilePath(sessionID), data, 0644)
}

func handleConversations(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		entries := loadConversations(sessionID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)

	case http.MethodDelete:
		indexStr := r.URL.Query().Get("index")
		if indexStr == "" {
			http.Error(w, "index required", http.StatusBadRequest)
			return
		}
		index := 0
		fmt.Sscanf(indexStr, "%d", &index)
		entries := loadConversations(sessionID)
		if index < 0 || index >= len(entries) {
			http.Error(w, "index out of range", http.StatusBadRequest)
			return
		}
		entries = append(entries[:index], entries[index+1:]...)
		if len(entries) == 0 {
			os.Remove(conversationFilePath(sessionID))
		} else {
			saveConversations(sessionID, entries)
		}
		w.WriteHeader(http.StatusOK)
	}
}

func configFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude-tabs", "config.json")
}

func loadConfig() Config {
	cfg := Config{
		Presets:     defaultPresets,
		SbxTemplate: "my-sbx:latest",
	}
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, &cfg)
	if len(cfg.Presets) == 0 {
		cfg.Presets = defaultPresets
	}
	if cfg.SbxTemplate == "" {
		cfg.SbxTemplate = "my-sbx:latest"
	}
	cfg.WorktreeBase = expandHome(cfg.WorktreeBase)
	for i := range cfg.SbxPostCreateCmds {
		for j := range cfg.SbxPostCreateCmds[i] {
			cfg.SbxPostCreateCmds[i][j] = expandHome(cfg.SbxPostCreateCmds[i][j])
		}
	}
	for i := range cfg.SbxDefaultMounts {
		cfg.SbxDefaultMounts[i] = expandHome(cfg.SbxDefaultMounts[i])
	}
	for i := range cfg.Plugins {
		cfg.Plugins[i].Source = expandHome(cfg.Plugins[i].Source)
	}
	cfg.RepositoryBase = expandHome(cfg.RepositoryBase)
	return cfg
}

var builtinTerminalPresets = map[string]TerminalScripts{
	"iterm2": {
		Focus: `tell application "iTerm2"
	activate
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is "{{TTY}}" then
					select t
					tell w to select
					return "found"
				end if
			end repeat
		end repeat
	end repeat
	return "not_found"
end tell`,
		Activate: `tell application "iTerm2" to activate`,
		Input: `tell application "iTerm2"
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is "{{TTY}}" then
					tell s to write text "{{TEXT}}"
					return "sent"
				end if
			end repeat
		end repeat
	end repeat
	return "not_found"
end tell`,
		Keys: `tell application "iTerm2"
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is "{{TTY}}" then
					{{CMDS}}
					return "sent"
				end if
			end repeat
		end repeat
	end repeat
	return "not_found"
end tell`,
		NewTab: `tell application "iTerm2"
	tell current window
		create tab with default profile
		tell current session of current tab
			write text "{{COMMAND}}"
			return tty
		end tell
	end tell
end tell`,
		Screen: `tell application "iTerm2"
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is "{{TTY}}" then
					return contents of s
				end if
			end repeat
		end repeat
	end repeat
	return ""
end tell`,
	},
	"terminal": {
		Focus: `tell application "Terminal"
	activate
	repeat with w in windows
		repeat with t in tabs of w
			if tty of t is "{{TTY}}" then
				set selected tab of w to t
				set index of w to 1
				return "found"
			end if
		end repeat
	end repeat
	return "not_found"
end tell`,
		Activate: `tell application "Terminal" to activate`,
		Input: `tell application "Terminal"
	do script "{{TEXT}}" in (first tab of first window whose tty is "{{TTY}}")
	return "sent"
end tell`,
		Keys: `tell application "System Events"
	tell process "Terminal"
		{{CMDS}}
	end tell
end tell
return "sent"`,
		NewTab: `tell application "Terminal"
	do script "{{COMMAND}}"
	return tty of selected tab of front window
end tell`,
		Screen: `tell application "Terminal"
	repeat with w in windows
		repeat with t in tabs of w
			if tty of t is "{{TTY}}" then
				return contents of t
			end if
		end repeat
	end repeat
	return ""
end tell`,
	},
}

func getTerminalScripts(cfg Config) TerminalScripts {
	terminal := cfg.Terminal
	if terminal == "" {
		terminal = "iterm2"
	}
	if cfg.TerminalPresets != nil {
		if ts, ok := cfg.TerminalPresets[terminal]; ok {
			return ts
		}
	}
	if ts, ok := builtinTerminalPresets[terminal]; ok {
		return ts
	}
	return builtinTerminalPresets["iterm2"]
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func handlePresets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loadConfig().Presets)
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(configFilePath())
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("{}"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		// validate JSON
		var v any
		if json.Unmarshal(body, &v) != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		// pretty print
		formatted, _ := json.MarshalIndent(v, "", "  ")
		if err := os.WriteFile(configFilePath(), append(formatted, '\n'), 0644); err != nil {
			http.Error(w, "write error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// worktreeCreate is the shared worktree creation logic used by both CLI and Web UI.
// resolveBranch resolves a PR URL to a branch name using gh CLI.
// If the input is not a PR URL, it returns the input as-is.
func resolveBranch(input string) (branch string, err error) {
	if strings.HasPrefix(input, "https://github.com/") && strings.Contains(input, "/pull/") {
		out, err := exec.Command("gh", "pr", "view", input, "--json", "headRefName", "-q", ".headRefName").Output()
		if err != nil {
			return "", fmt.Errorf("failed to resolve PR: %s", input)
		}
		return strings.TrimSpace(string(out)), nil
	}
	return input, nil
}

func worktreeCreate(repo, branch, baseBranch string) (tty, cwdPath string, err error) {
	branch, err = resolveBranch(branch)
	if err != nil {
		return "", "", err
	}

	cfg := loadConfig()

	// ghqからリポジトリ検索
	ghqOut, err := exec.Command("ghq", "list", "-p").Output()
	if err != nil {
		return "", "", fmt.Errorf("ghq list failed: %w", err)
	}
	var repoPath string
	for _, line := range strings.Split(strings.TrimSpace(string(ghqOut)), "\n") {
		if strings.HasSuffix(line, "/"+repo) {
			repoPath = line
			break
		}
	}
	if repoPath == "" {
		return "", "", fmt.Errorf("repository not found: %s", repo)
	}

	// worktree base
	wtBase := cfg.WorktreeBase
	if wtBase == "" {
		home, _ := os.UserHomeDir()
		wtBase = filepath.Join(home, "worktrees")
	}
	wtPath := filepath.Join(wtBase, repo, branch)
	sbxName := "wt-" + repo + "-" + branch

	// worktree作成
	if _, err := os.Stat(wtPath); err == nil {
		fmt.Println("Worktree already exists:", wtPath)
	} else {
		if out, err := exec.Command("git", "-C", repoPath, "fetch", "origin").CombinedOutput(); err != nil {
			return "", "", fmt.Errorf("git fetch failed: %s", out)
		}
		if baseBranch != "" {
			if exec.Command("git", "-C", repoPath, "rev-parse", "--verify", baseBranch).Run() != nil &&
				exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "origin/"+baseBranch).Run() != nil {
				return "", "", fmt.Errorf("base branch not found: %s", baseBranch)
			}
		}
		if err := exec.Command("git", "-C", repoPath, "rev-parse", "origin/"+branch).Run(); err == nil {
			if out, err := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, "origin/"+branch).CombinedOutput(); err != nil {
				return "", "", fmt.Errorf("git worktree add failed: %s", out)
			}
		} else if err := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", branch).Run(); err == nil {
			if out, err := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, branch).CombinedOutput(); err != nil {
				return "", "", fmt.Errorf("git worktree add failed: %s", out)
			}
		} else {
			args := []string{"-C", repoPath, "worktree", "add", wtPath, "-b", branch}
			if baseBranch != "" {
				args = append(args, baseBranch)
			}
			if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
				return "", "", fmt.Errorf("git worktree add (new branch) failed: %s", out)
			}
		}
	}

	// sbx create
	claudeTabsDir := expandHome("~/.claude-tabs")
	paths := []string{wtPath, claudeTabsDir}
	paths = append(paths, cfg.SbxDefaultMounts...)
	sbxArgs := []string{"create", "--name", sbxName, "-t", cfg.SbxTemplate}
	for _, kit := range cfg.SbxKits {
		sbxArgs = append(sbxArgs, "--kit", kit)
	}
	sbxArgs = append(sbxArgs, "claude")
	sbxArgs = append(sbxArgs, paths...)
	if out, err := exec.Command("sbx", sbxArgs...).CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("sbx create failed: %s", out)
	}

	// ~/.claude-tabs symlink (マウントパスがホスト側パスになるため)
	exec.Command("sbx", "exec", sbxName, "ln", "-sf", claudeTabsDir, expandHome("~")+"/.claude-tabs").Run()

	// setup commands (best effort)
	for _, cmd := range cfg.SbxPostCreateCmds {
		if len(cmd) > 0 {
			args := append([]string{"exec", sbxName}, cmd...)
			exec.Command("sbx", args...).Run()
		}
	}

	// plugins install
	for _, pc := range cfg.Plugins {
		exec.Command("sbx", "exec", sbxName, "claude", "plugins", "marketplace", "add", pc.Source).Run()
		if len(pc.Plugins) == 1 && pc.Plugins[0] == "auto" {
			marketplaceName := ""
			if mdata, err := os.ReadFile(filepath.Join(pc.Source, ".claude-plugin", "marketplace.json")); err == nil {
				var mj struct{ Name string `json:"name"` }
				if json.Unmarshal(mdata, &mj) == nil && mj.Name != "" {
					marketplaceName = mj.Name
				}
			}
			if marketplaceName != "" {
				if entries, err := os.ReadDir(filepath.Join(pc.Source, "plugins")); err == nil {
					for _, e := range entries {
						if e.IsDir() {
							exec.Command("sbx", "exec", sbxName, "claude", "plugins", "install", e.Name()+"@"+marketplaceName).Run()
						}
					}
				}
			}
		} else {
			for _, plugin := range pc.Plugins {
				exec.Command("sbx", "exec", sbxName, "claude", "plugins", "install", plugin).Run()
			}
		}
	}

	// 新タブでclaude起動、TTYを取得
	ts := getTerminalScripts(cfg)
	script := strings.ReplaceAll(ts.NewTab, "{{COMMAND}}", "sbx run --name "+sbxName+" claude")
	ttyOut, _ := exec.Command("osascript", "-e", script).Output()
	tty = strings.TrimSpace(string(ttyOut))
	cwdPath = wtPath

	fmt.Println("Worktree created and Claude started:", sbxName)
	return
}

func (s *server) handleWorktreeCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	repo := r.URL.Query().Get("repo")
	branch := r.URL.Query().Get("branch")
	if repo == "" || branch == "" {
		http.Error(w, "repo and branch required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	baseBranch := r.URL.Query().Get("base")
	tty, cwdPath, err := worktreeCreate(repo, branch, baseBranch)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
		return
	}
	sbxName := "wt-" + repo + "-" + branch
	if cwdPath != "" {
		s.mu.Lock()
		if tty != "" {
			s.pendingTTY[cwdPath] = tty
		}
		s.pendingName[cwdPath] = sbxName
		s.mu.Unlock()
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "Worktree created and Claude started: " + sbxName})
}

func handleSbxList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	out, err := exec.Command("sbx", "ls", "-q").Output()
	if err != nil {
		json.NewEncoder(w).Encode([]string{})
		return
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			names = append(names, line)
		}
	}
	if names == nil {
		names = []string{}
	}
	json.NewEncoder(w).Encode(names)
}

func handleRepoList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := loadConfig()
	if cfg.RepositoryBase == "" {
		json.NewEncoder(w).Encode([]string{})
		return
	}
	var repos []string
	scanGitRepos := func(base string, maxDepth int) {
		if base == "" {
			return
		}
		filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return filepath.SkipDir
			}
			if d.Name() == ".git" {
				rel, _ := filepath.Rel(base, filepath.Dir(path))
				repos = append(repos, rel)
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(base, path)
			if strings.Count(rel, string(filepath.Separator)) >= maxDepth {
				return filepath.SkipDir
			}
			return nil
		})
	}
	scanGitRepos(cfg.RepositoryBase, 4)
	scanGitRepos(cfg.WorktreeBase, 2)
	if repos == nil {
		repos = []string{}
	}
	json.NewEncoder(w).Encode(repos)
}

func (s *server) handleSbxRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sbxName := r.URL.Query().Get("sbx")
	repoPath := r.URL.Query().Get("repo")
	if sbxName == "" || repoPath == "" {
		http.Error(w, "sbx and repo required", http.StatusBadRequest)
		return
	}

	cfg := loadConfig()
	// worktree_base 配下に存在すればそちらを使う
	fullPath := filepath.Join(cfg.RepositoryBase, repoPath)
	if cfg.WorktreeBase != "" {
		wtPath := filepath.Join(cfg.WorktreeBase, repoPath)
		if _, err := os.Stat(wtPath); err == nil {
			fullPath = wtPath
		}
	}

	ts := getTerminalScripts(cfg)
	command := fmt.Sprintf("sbx exec -it %s sh -c 'cd %s && claude'", sbxName, fullPath)
	script := strings.ReplaceAll(ts.NewTab, "{{COMMAND}}", command)
	ttyOut, _ := exec.Command("osascript", "-e", script).Output()
	tty := strings.TrimSpace(string(ttyOut))

	w.Header().Set("Content-Type", "application/json")
	if tty != "" && fullPath != "" {
		s.mu.Lock()
		s.pendingTTY[fullPath] = tty
		s.pendingName[fullPath] = sbxName + ":" + filepath.Base(repoPath)
		s.mu.Unlock()
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "Claude started in " + sbxName + " at " + repoPath})
}

// createWorktreeOnly creates a worktree without creating a new sbx.
func createWorktreeOnly(repo, branch, baseBranch string) (wtPath string, err error) {
	branch, err = resolveBranch(branch)
	if err != nil {
		return "", err
	}

	cfg := loadConfig()

	// repository_base からリポジトリ検索
	var repoPath string
	if cfg.RepositoryBase != "" {
		filepath.WalkDir(cfg.RepositoryBase, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return filepath.SkipDir
			}
			if d.Name() == ".git" {
				dir := filepath.Dir(path)
				if filepath.Base(dir) == repo || strings.HasSuffix(dir, "/"+repo) {
					repoPath = dir
					return filepath.SkipAll
				}
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(cfg.RepositoryBase, path)
			if strings.Count(rel, string(filepath.Separator)) >= 4 {
				return filepath.SkipDir
			}
			return nil
		})
	}
	if repoPath == "" {
		return "", fmt.Errorf("repository not found: %s", repo)
	}

	wtBase := cfg.WorktreeBase
	if wtBase == "" {
		home, _ := os.UserHomeDir()
		wtBase = filepath.Join(home, "worktrees")
	}
	wtPath = filepath.Join(wtBase, repo, branch)

	if _, statErr := os.Stat(wtPath); statErr == nil {
		return wtPath, nil // already exists
	}

	if out, fetchErr := exec.Command("git", "-C", repoPath, "fetch", "origin").CombinedOutput(); fetchErr != nil {
		return "", fmt.Errorf("git fetch failed: %s", out)
	}
	if baseBranch != "" {
		if exec.Command("git", "-C", repoPath, "rev-parse", "--verify", baseBranch).Run() != nil &&
			exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "origin/"+baseBranch).Run() != nil {
			return "", fmt.Errorf("base branch not found: %s", baseBranch)
		}
	}
	if err := exec.Command("git", "-C", repoPath, "rev-parse", "origin/"+branch).Run(); err == nil {
		if out, addErr := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, "origin/"+branch).CombinedOutput(); addErr != nil {
			return "", fmt.Errorf("git worktree add failed: %s", out)
		}
	} else if err := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", branch).Run(); err == nil {
		if out, addErr := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, branch).CombinedOutput(); addErr != nil {
			return "", fmt.Errorf("git worktree add failed: %s", out)
		}
	} else {
		args := []string{"-C", repoPath, "worktree", "add", wtPath, "-b", branch}
		if baseBranch != "" {
			args = append(args, baseBranch)
		}
		if out, addErr := exec.Command("git", args...).CombinedOutput(); addErr != nil {
			return "", fmt.Errorf("git worktree add (new branch) failed: %s", out)
		}
	}
	return wtPath, nil
}

func (s *server) handleSbxAttachWorktree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sbxName := r.URL.Query().Get("sbx")
	repo := r.URL.Query().Get("repo")
	branch := r.URL.Query().Get("branch")
	baseBranch := r.URL.Query().Get("base")
	if sbxName == "" || repo == "" || branch == "" {
		http.Error(w, "sbx, repo, and branch required", http.StatusBadRequest)
		return
	}

	wtPath, err := createWorktreeOnly(repo, branch, baseBranch)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
		return
	}

	cfg := loadConfig()
	ts := getTerminalScripts(cfg)
	command := fmt.Sprintf("sbx exec -it %s sh -c 'cd %s && claude'", sbxName, wtPath)
	script := strings.ReplaceAll(ts.NewTab, "{{COMMAND}}", command)
	ttyOut, _ := exec.Command("osascript", "-e", script).Output()
	tty := strings.TrimSpace(string(ttyOut))

	w.Header().Set("Content-Type", "application/json")
	if tty != "" {
		s.mu.Lock()
		s.pendingTTY[wtPath] = tty
		s.pendingName[wtPath] = sbxName + ":" + repo + "/" + branch
		s.mu.Unlock()
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "Worktree created and Claude started in " + sbxName})
}

func runServer() {
	srv := newServer()
	srv.loadSessions()

	// Watch for file changes
	go srv.watchSessions()

	// Periodic health check (every 10s)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			srv.healthCheck()
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(srv.getSessions())
	})

	mux.HandleFunc("/api/sessions/delete", srv.handleDeleteSession)
	mux.HandleFunc("/api/sessions/delete-check", srv.handleDeleteSessionCheck)
	mux.HandleFunc("/api/sessions/name", srv.handleRenameSession)
	mux.HandleFunc("/api/sessions/tty", srv.handleSetTTY)
	mux.HandleFunc("/api/sessions/focus", srv.handleFocusTerminal)
	mux.HandleFunc("/api/sessions/screen", srv.handleScreen)
	mux.HandleFunc("/api/browser/activate", handleActivateBrowser)
	mux.HandleFunc("/api/sessions/input", srv.handleSendInput)
	mux.HandleFunc("/api/sessions/keys", srv.handleSendKeys)
	mux.HandleFunc("/api/sessions/history", srv.handleHistory)
	mux.HandleFunc("/api/worktree/create", srv.handleWorktreeCreate)
	mux.HandleFunc("/api/sbx/list", handleSbxList)
	mux.HandleFunc("/api/sbx/repos", handleRepoList)
	mux.HandleFunc("/api/sbx/run", srv.handleSbxRun)
	mux.HandleFunc("/api/sbx/attach-worktree", srv.handleSbxAttachWorktree)
	mux.HandleFunc("/api/presets", handlePresets)
	mux.HandleFunc("/api/config", handleConfig)
	mux.HandleFunc("/api/conversations", handleConversations)

	mux.HandleFunc("/api/ws", srv.handleWS)

	// Serve frontend
	distFS, _ := fs.Sub(frontendFS, "frontend/dist")
	fileServer := http.FileServer(http.FS(distFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		f, err := distFS.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	ln, err := net.Listen("tcp", getListenAddr())
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude-tabs: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("claude-tabs server running on http://%s (listening on %s)\n", getAddr(), getListenAddr())

	go func() {
		time.Sleep(100 * time.Millisecond)
		browser.OpenURL("http://" + getAddr())
	}()

	http.Serve(ln, mux)
}
