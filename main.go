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
	Memo        string    `json:"memo,omitempty"`
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
		if _, _, _, err := worktreeCreate(repo, branch, ""); err != nil {
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
	case "PostToolUse":
		session.Status = "ai_working"
		session.Question = ""
	case "SessionEnd":
		session.Status = "terminated"
		session.Question = ""
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

func (s *server) handleMemo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	memo := r.URL.Query().Get("memo")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	session, ok := s.sessions[id]
	if ok {
		session.Memo = memo
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
	cfg := loadConfig()
	ts := getTerminalScripts(cfg)
	hasNewline := strings.Contains(text, "\n")
	escapedText := strings.ReplaceAll(text, `\`, `\\`)
	escapedText = strings.ReplaceAll(escapedText, `"`, `\"`)
	escapedText = strings.ReplaceAll(escapedText, "\n", `" & return & "`)
	escapedText = strings.ReplaceAll(escapedText, "\r", "")
	script := strings.ReplaceAll(strings.ReplaceAll(ts.Input, "{{TTY}}", tty), "{{TEXT}}", escapedText)

	result, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		http.Error(w, "AppleScript error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Multi-line text: input script's trailing Enter may not submit, send explicit Enter
	if hasNewline {
		time.Sleep(100 * time.Millisecond)
		terminal := cfg.Terminal
		if terminal == "" {
			terminal = "iterm2"
		}
		if terminal == "terminal" {
			// Terminal.app: use System Events keystroke
			enterScript := strings.ReplaceAll(strings.ReplaceAll(ts.Keys, "{{TTY}}", tty), "{{CMDS}}", `keystroke return`)
			exec.Command("osascript", "-e", enterScript).Run()
		} else {
			// iTerm2: write text "" sends Enter
			enterScript := strings.ReplaceAll(ts.Input, "{{TTY}}", tty)
			enterScript = strings.ReplaceAll(enterScript, "{{TEXT}}", "")
			exec.Command("osascript", "-e", enterScript).Run()
		}
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
	action := r.URL.Query().Get("action") // "1", "2", "3", "4"
	if id == "" || action == "" {
		http.Error(w, "id and action required", http.StatusBadRequest)
		return
	}

	// Send number key + Enter to select option
	switch action {
	case "1", "2", "3", "4":
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
	cmds := strings.ReplaceAll(ts.KeyCmd, "{{KEY}}", action)
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
	wtBase := cfg.Worktree.Base
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

	// Clean up project mapping
	pf := loadProjects()
	if _, ok := pf.SessionProjectMap[id]; ok {
		projectID := pf.SessionProjectMap[id]
		delete(pf.SessionProjectMap, id)
		// Remove from session order
		if order, exists := pf.SessionOrder[projectID]; exists {
			filtered := make([]string, 0, len(order))
			for _, sid := range order {
				if sid != id {
					filtered = append(filtered, sid)
				}
			}
			pf.SessionOrder[projectID] = filtered
		}
		saveProjects(pf)
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
	{Label: "Commit", Text: "commit"},
	{Label: "Commit & Push", Text: "commit and push"},
}


type PluginConfig struct {
	Source  string   `json:"source"`
	Plugins []string `json:"plugins"`
}


type FocusBrowserConfig struct {
	Enable   bool     `json:"enable"`
	Statuses []string `json:"statuses"`
}

type SbxConfig struct {
	Template       string         `json:"template"`
	DefaultMounts  []string       `json:"default_mounts"`
	PostCreateCmds [][]string     `json:"post_create_cmds"`
	Kits           []string       `json:"kits"`
	Plugins        []PluginConfig `json:"plugins"`
	CloneBase      string         `json:"clone_base"`
	RepositoryBase string         `json:"repository_base"`
}

type WorktreeConfig struct {
	Base string `json:"base"`
}

type Config struct {
	Presets                  []Preset                   `json:"presets"`
	Worktree                 WorktreeConfig             `json:"worktree"`
	Sbx                      SbxConfig                  `json:"sbx"`
	Terminal                 string                     `json:"terminal"`
	TerminalPresets          map[string]TerminalScripts `json:"terminal_presets"`
	ScreenLines              int                        `json:"screen_lines"`
	Port                     int                        `json:"port"`
	BrowserApp               string                     `json:"browser_app"`
	FocusBrowserOnAttention  *FocusBrowserConfig        `json:"focus_browser_on_attention"`
	ListenAddress            string                     `json:"listen_address"`
	ConversationMaxEntries   int                        `json:"conversation_max_entries"`
	Project                  ProjectConfig              `json:"project"`
}

type ProjectConfig struct {
	DefaultLinkSections []LinkSection `json:"default_link_sections"`
}

// Conversations

type ConversationEntry struct {
	Output   string `json:"output"`
	Input    string `json:"input,omitempty"`
	SavedAt  string `json:"saved_at"`
	Favorite bool   `json:"favorite,omitempty"`
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

func conversationMaxEntries() int {
	cfg := loadConfig()
	if cfg.ConversationMaxEntries > 0 {
		return cfg.ConversationMaxEntries
	}
	return 100
}

func saveConversations(sessionID string, entries []ConversationEntry) error {
	max := conversationMaxEntries()
	if len(entries) > max {
		// Keep favorites, trim non-favorites from the end
		var kept []ConversationEntry
		nonFavCount := 0
		for _, e := range entries {
			if e.Favorite {
				kept = append(kept, e)
			} else if nonFavCount < max {
				kept = append(kept, e)
				nonFavCount++
			}
		}
		entries = kept
	}
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

	case "PATCH":
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
		entries[index].Favorite = !entries[index].Favorite
		saveConversations(sessionID, entries)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"favorite": entries[index].Favorite})

	case http.MethodDelete:
		action := r.URL.Query().Get("action")
		if action == "delete_non_favorites" {
			entries := loadConversations(sessionID)
			var kept []ConversationEntry
			for _, e := range entries {
				if e.Favorite {
					kept = append(kept, e)
				}
			}
			if len(kept) == 0 {
				os.Remove(conversationFilePath(sessionID))
			} else {
				saveConversations(sessionID, kept)
			}
			w.WriteHeader(http.StatusOK)
			return
		}
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

// Projects

type NamedLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type LinkSection struct {
	Label string      `json:"label"`
	Links []NamedLink `json:"links"`
}

type Project struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	LinkSections []LinkSection `json:"link_sections"`
	Memo         string        `json:"memo"`
	Archived     bool          `json:"archived"`
	CreatedAt    string        `json:"created_at"`
	Order        int           `json:"order"`
}

type ProjectsFile struct {
	Projects          []Project            `json:"projects"`
	SessionProjectMap map[string]string    `json:"session_project_map"`
	SessionOrder      map[string][]string  `json:"session_order"`
}

func projectsFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude-tabs", "projects.json")
}

func loadProjects() ProjectsFile {
	data, err := os.ReadFile(projectsFilePath())
	if err != nil {
		return ProjectsFile{SessionProjectMap: map[string]string{}}
	}
	var pf ProjectsFile
	json.Unmarshal(data, &pf)
	if pf.SessionProjectMap == nil {
		pf.SessionProjectMap = map[string]string{}
	}
	if pf.SessionOrder == nil {
		pf.SessionOrder = map[string][]string{}
	}
	return pf
}

func saveProjects(pf ProjectsFile) error {
	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(projectsFilePath(), data, 0644)
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(strings.NewReader(fmt.Sprintf("%d", time.Now().UnixNano())), b); err != nil {
		// fallback
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pf := loadProjects()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pf)

	case http.MethodPost:
		// Create new project
		var p Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		pf := loadProjects()
		p.ID = generateID()
		p.CreatedAt = time.Now().Format(time.RFC3339)
		p.Order = len(pf.Projects)
		if p.LinkSections == nil {
			cfg := loadConfig()
			if len(cfg.Project.DefaultLinkSections) > 0 {
				p.LinkSections = make([]LinkSection, len(cfg.Project.DefaultLinkSections))
				for i, s := range cfg.Project.DefaultLinkSections {
					p.LinkSections[i] = LinkSection{Label: s.Label, Links: []NamedLink{}}
				}
			} else {
				p.LinkSections = []LinkSection{
					{Label: "GitHub", Links: []NamedLink{}},
					{Label: "PRD", Links: []NamedLink{}},
					{Label: "Spec", Links: []NamedLink{}},
					{Label: "NotebookLM", Links: []NamedLink{}},
					{Label: "Slack", Links: []NamedLink{}},
				}
			}
		}
		pf.Projects = append(pf.Projects, p)
		if err := saveProjects(pf); err != nil {
			http.Error(w, "save error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)

	case http.MethodPut:
		// Update project
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		var update Project
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		pf := loadProjects()
		found := false
		for i, p := range pf.Projects {
			if p.ID == id {
				update.ID = p.ID
				update.CreatedAt = p.CreatedAt
				if update.LinkSections == nil {
					update.LinkSections = []LinkSection{}
				}
				pf.Projects[i] = update
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := saveProjects(pf); err != nil {
			http.Error(w, "save error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		pf := loadProjects()
		filtered := make([]Project, 0, len(pf.Projects))
		for _, p := range pf.Projects {
			if p.ID != id {
				filtered = append(filtered, p)
			}
		}
		pf.Projects = filtered
		// Remove session mappings for this project
		for sid, pid := range pf.SessionProjectMap {
			if pid == id {
				delete(pf.SessionProjectMap, sid)
			}
		}
		if err := saveProjects(pf); err != nil {
			http.Error(w, "save error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleProjectSessionMap(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		projectID := r.URL.Query().Get("project_id")
		pf := loadProjects()
		if projectID == "" {
			// Remove mapping
			delete(pf.SessionProjectMap, sessionID)
		} else {
			pf.SessionProjectMap[sessionID] = projectID
		}
		if err := saveProjects(pf); err != nil {
			http.Error(w, "save error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleProjectReorder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var ids []string
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	pf := loadProjects()
	projectMap := make(map[string]*Project)
	for i := range pf.Projects {
		projectMap[pf.Projects[i].ID] = &pf.Projects[i]
	}
	reordered := make([]Project, 0, len(ids))
	for i, id := range ids {
		if p, ok := projectMap[id]; ok {
			p.Order = i
			reordered = append(reordered, *p)
			delete(projectMap, id)
		}
	}
	// Append any projects not in the reorder list
	for _, p := range pf.Projects {
		if _, ok := projectMap[p.ID]; ok {
			reordered = append(reordered, p)
		}
	}
	pf.Projects = reordered
	if err := saveProjects(pf); err != nil {
		http.Error(w, "save error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleSessionOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		http.Error(w, "project_id required", http.StatusBadRequest)
		return
	}
	var sessionIDs []string
	if err := json.NewDecoder(r.Body).Decode(&sessionIDs); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	pf := loadProjects()
	pf.SessionOrder[projectID] = sessionIDs
	if err := saveProjects(pf); err != nil {
		http.Error(w, "save error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func configFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude-tabs", "config.json")
}

func loadConfig() Config {
	cfg := Config{
		Presets: defaultPresets,
		Sbx: SbxConfig{
			Template: "my-sbx:latest",
		},
	}
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, &cfg)
	if len(cfg.Presets) == 0 {
		cfg.Presets = defaultPresets
	}
	if cfg.Sbx.Template == "" {
		cfg.Sbx.Template = "my-sbx:latest"
	}
	cfg.Worktree.Base = expandHome(cfg.Worktree.Base)
	for i := range cfg.Sbx.PostCreateCmds {
		for j := range cfg.Sbx.PostCreateCmds[i] {
			cfg.Sbx.PostCreateCmds[i][j] = expandHome(cfg.Sbx.PostCreateCmds[i][j])
		}
	}
	for i := range cfg.Sbx.DefaultMounts {
		cfg.Sbx.DefaultMounts[i] = expandHome(cfg.Sbx.DefaultMounts[i])
	}
	for i := range cfg.Sbx.Plugins {
		cfg.Sbx.Plugins[i].Source = expandHome(cfg.Sbx.Plugins[i].Source)
	}
	cfg.Sbx.RepositoryBase = expandHome(cfg.Sbx.RepositoryBase)
	return cfg
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

const defaultDockerfile = `FROM docker/sandbox-templates:claude-code

USER root

# Basic tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    zsh \
    curl \
    jq \
    make \
    vim \
    git \
    && rm -rf /var/lib/apt/lists/*

# Set zsh as default shell
RUN chsh -s /usr/bin/zsh agent

USER agent
`

func handleSbxDockerfile(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	dockerfilePath := filepath.Join(home, ".claude-tabs", "Dockerfile")

	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(dockerfilePath)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"content": defaultDockerfile, "is_default": "true"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"content": string(data), "is_default": "false"})
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		var req struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(body, &req) != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(filepath.Dir(dockerfilePath), 0755); err != nil {
			http.Error(w, "mkdir error", http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(dockerfilePath, []byte(req.Content), 0644); err != nil {
			http.Error(w, "write error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Saved"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleSbxBuildTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	home, _ := os.UserHomeDir()
	dockerfilePath := filepath.Join(home, ".claude-tabs", "Dockerfile")

	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Dockerfile not found. Save it first."})
		return
	}

	cfg := loadConfig()
	tag := cfg.Sbx.Template
	if tag == "" {
		tag = "my-sbx:latest"
	}

	w.Header().Set("Content-Type", "application/json")

	// docker build
	dir := filepath.Dir(dockerfilePath)
	buildOut, err := exec.Command("docker", "build", "-t", tag, "-f", dockerfilePath, dir).CombinedOutput()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "docker build failed: " + string(buildOut)})
		return
	}

	// docker save + sbx template load
	tmpFile := filepath.Join(os.TempDir(), "sbx-template-"+fmt.Sprintf("%d", time.Now().UnixNano())+".tar")
	saveOut, err := exec.Command("docker", "save", tag, "-o", tmpFile).CombinedOutput()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "docker save failed: " + string(saveOut)})
		return
	}
	defer os.Remove(tmpFile)

	loadOut, err := exec.Command("sbx", "template", "load", tmpFile).CombinedOutput()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "sbx template load failed: " + string(loadOut)})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Template '" + tag + "' built and loaded successfully"})
}

func handleGitClone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	repoURL := r.URL.Query().Get("url")
	if repoURL == "" {
		http.Error(w, `{"message":"url is required"}`, http.StatusBadRequest)
		return
	}

	cfg := loadConfig()
	cloneBase := cfg.Sbx.CloneBase
	if cloneBase == "" {
		cloneBase = "~/src"
	}
	// expand ~
	if strings.HasPrefix(cloneBase, "~/") {
		home, _ := os.UserHomeDir()
		cloneBase = filepath.Join(home, cloneBase[2:])
	}

	// ensure clone_base exists
	if err := os.MkdirAll(cloneBase, 0755); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "failed to create clone_base: " + err.Error()})
		return
	}

	// extract repo name from URL
	repoName := filepath.Base(strings.TrimSuffix(repoURL, ".git"))
	dest := filepath.Join(cloneBase, repoName)

	// check if already exists
	if _, err := os.Stat(dest); err == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Already cloned: " + dest, "path": dest})
		return
	}

	cmd := exec.Command("git", "clone", repoURL, dest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "git clone failed: " + string(output)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Cloned to " + dest, "path": dest})
}

func getCloneBase() string {
	cfg := loadConfig()
	cloneBase := cfg.Sbx.CloneBase
	if cloneBase == "" {
		cloneBase = "~/src"
	}
	return expandHome(cloneBase)
}

func handleRepoListWithBranch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cloneBase := getCloneBase()

	entries, err := os.ReadDir(cloneBase)
	if err != nil {
		json.NewEncoder(w).Encode([]any{})
		return
	}

	type RepoInfo struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Branch string `json:"branch"`
	}
	var repos []RepoInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		repoPath := filepath.Join(cloneBase, e.Name())
		if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
			continue
		}
		branch := ""
		if out, err := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
			branch = strings.TrimSpace(string(out))
		}
		repos = append(repos, RepoInfo{Name: e.Name(), Path: repoPath, Branch: branch})
	}
	if repos == nil {
		repos = []RepoInfo{}
	}
	json.NewEncoder(w).Encode(repos)
}

func handleRepoBranches(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	repo := r.URL.Query().Get("repo")
	if repo == "" {
		json.NewEncoder(w).Encode([]string{})
		return
	}
	repoPath := filepath.Join(getCloneBase(), repo)

	// fetch latest
	exec.Command("git", "-C", repoPath, "fetch", "origin").Run()

	out, err := exec.Command("git", "-C", repoPath, "branch", "-r", "--format=%(refname:short)").Output()
	if err != nil {
		json.NewEncoder(w).Encode([]string{})
		return
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "HEAD") {
			continue
		}
		// remove "origin/" prefix
		if strings.HasPrefix(line, "origin/") {
			line = line[7:]
		}
		branches = append(branches, line)
	}
	if branches == nil {
		branches = []string{}
	}
	json.NewEncoder(w).Encode(branches)
}

func handleSbxBranches(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sbxName := r.URL.Query().Get("sbx")
	repo := r.URL.Query().Get("repo")
	if sbxName == "" || repo == "" {
		json.NewEncoder(w).Encode([]any{})
		return
	}

	// get remote URL to resolve org/repo for gh
	remoteOut, err := exec.Command("sbx", "exec", sbxName, "git", "-C", repo, "remote", "get-url", "origin").Output()
	if err != nil {
		json.NewEncoder(w).Encode([]any{})
		return
	}
	remoteURL := strings.TrimSpace(string(remoteOut))

	// parse org/repo from remote URL
	ghRepo := ""
	if strings.Contains(remoteURL, "github.com") {
		// ssh: git@github.com:org/repo.git  or  https://github.com/org/repo.git
		remoteURL = strings.TrimSuffix(remoteURL, ".git")
		if idx := strings.Index(remoteURL, "github.com"); idx >= 0 {
			rest := remoteURL[idx+len("github.com"):]
			rest = strings.TrimPrefix(rest, ":")
			rest = strings.TrimPrefix(rest, "/")
			ghRepo = rest
		}
	}

	type BranchInfo struct {
		Name string `json:"name"`
		PR   int    `json:"pr,omitempty"`
	}

	// get branches via sbx
	out, err := exec.Command("sbx", "exec", sbxName, "git", "-C", repo, "branch", "-r", "--format=%(refname:short)").Output()
	if err != nil {
		json.NewEncoder(w).Encode([]BranchInfo{})
		return
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "HEAD") {
			continue
		}
		if strings.HasPrefix(line, "origin/") {
			line = line[7:]
		}
		branches = append(branches, line)
	}

	// get PR info via gh (on host)
	prMap := map[string]int{}
	if ghRepo != "" {
		prOut, err := exec.Command("gh", "pr", "list", "--repo", ghRepo, "--state", "open", "--json", "number,headRefName", "--limit", "200").Output()
		if err == nil {
			var prs []struct {
				Number      int    `json:"number"`
				HeadRefName string `json:"headRefName"`
			}
			if json.Unmarshal(prOut, &prs) == nil {
				for _, pr := range prs {
					prMap[pr.HeadRefName] = pr.Number
				}
			}
		}
	}

	result := make([]BranchInfo, 0, len(branches))
	for _, b := range branches {
		bi := BranchInfo{Name: b}
		if pr, ok := prMap[b]; ok {
			bi.PR = pr
		}
		result = append(result, bi)
	}
	json.NewEncoder(w).Encode(result)
}

func handleRepoCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	repo := r.URL.Query().Get("repo")
	branch := r.URL.Query().Get("branch")
	if repo == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "repo is required"})
		return
	}
	repoPath := filepath.Join(getCloneBase(), repo)

	// fetch
	if out, err := exec.Command("git", "-C", repoPath, "fetch", "origin").CombinedOutput(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "git fetch failed: " + string(out)})
		return
	}

	// switch branch if specified
	if branch != "" {
		if out, err := exec.Command("git", "-C", repoPath, "switch", branch).CombinedOutput(); err != nil {
			// try creating tracking branch
			if out2, err2 := exec.Command("git", "-C", repoPath, "switch", "-c", branch, "origin/"+branch).CombinedOutput(); err2 != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"message": "git switch failed: " + string(out) + "\n" + string(out2)})
				return
			}
		}
	}

	// pull
	if out, err := exec.Command("git", "-C", repoPath, "pull").CombinedOutput(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "git pull failed: " + string(out)})
		return
	}

	// get current branch after checkout
	currentBranch := ""
	if out, err := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		currentBranch = strings.TrimSpace(string(out))
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Checked out " + repo + " on " + currentBranch, "branch": currentBranch})
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
func resolveBranch(input string) (branch string, prNumber string, err error) {
	if strings.HasPrefix(input, "https://github.com/") && strings.Contains(input, "/pull/") {
		// Extract PR number from URL
		parts := strings.Split(input, "/pull/")
		if len(parts) == 2 {
			prNumber = strings.TrimRight(parts[1], "/")
		}
		out, err := exec.Command("gh", "pr", "view", input, "--json", "headRefName", "-q", ".headRefName").Output()
		if err != nil {
			return "", "", fmt.Errorf("failed to resolve PR: %s", input)
		}
		return strings.TrimSpace(string(out)), prNumber, nil
	}
	return input, "", nil
}

func worktreeCreate(repo, branch, baseBranch string) (tty, cwdPath, sbxName string, err error) {
	var prNumber string
	branch, prNumber, err = resolveBranch(branch)
	if err != nil {
		return "", "", "", err
	}

	cfg := loadConfig()

	// ghqからリポジトリ検索
	ghqOut, err := exec.Command("ghq", "list", "-p").Output()
	if err != nil {
		return "", "", "", fmt.Errorf("ghq list failed: %w", err)
	}
	var repoPath string
	for _, line := range strings.Split(strings.TrimSpace(string(ghqOut)), "\n") {
		if strings.HasSuffix(line, "/"+repo) {
			repoPath = line
			break
		}
	}
	if repoPath == "" {
		return "", "", "", fmt.Errorf("repository not found: %s", repo)
	}

	// worktree base
	wtBase := cfg.Worktree.Base
	if wtBase == "" {
		home, _ := os.UserHomeDir()
		wtBase = filepath.Join(home, "worktrees")
	}

	// Determine prefix and create worktree
	isRemote := false
	if out, err := exec.Command("git", "-C", repoPath, "fetch", "origin").CombinedOutput(); err != nil {
		return "", "", "", fmt.Errorf("git fetch failed: %s", out)
	}
	if baseBranch != "" {
		if exec.Command("git", "-C", repoPath, "rev-parse", "--verify", baseBranch).Run() != nil &&
			exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "origin/"+baseBranch).Run() != nil {
			return "", "", "", fmt.Errorf("base branch not found: %s", baseBranch)
		}
	}
	if exec.Command("git", "-C", repoPath, "rev-parse", "origin/"+branch).Run() == nil {
		isRemote = true
	}

	prefix := "wt-"
	if prNumber != "" {
		prefix = "pr" + prNumber + "-"
	} else if isRemote {
		prefix = "remote-"
	}
	safeBranch := strings.ReplaceAll(branch, "/", "__")
	sbxName = prefix + repo + "-" + safeBranch
	wtPath := filepath.Join(wtBase, repo, prefix+safeBranch)

	if _, err := os.Stat(wtPath); err == nil {
		fmt.Println("Worktree already exists:", wtPath)
	} else if isRemote {
		if out, err := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, "origin/"+branch).CombinedOutput(); err != nil {
			return "", "", "", fmt.Errorf("git worktree add failed: %s", out)
		}
	} else if exec.Command("git", "-C", repoPath, "rev-parse", "--verify", branch).Run() == nil {
		if out, err := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, branch).CombinedOutput(); err != nil {
			return "", "", "", fmt.Errorf("git worktree add failed: %s", out)
		}
	} else {
		args := []string{"-C", repoPath, "worktree", "add", wtPath, "-b", branch}
		if baseBranch != "" {
			args = append(args, baseBranch)
		}
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			return "", "", "", fmt.Errorf("git worktree add (new branch) failed: %s", out)
		}
	}

	// sbx create
	claudeTabsDir := expandHome("~/.claude-tabs")
	paths := []string{wtPath, claudeTabsDir}
	paths = append(paths, cfg.Sbx.DefaultMounts...)
	sbxArgs := []string{"create", "--name", sbxName, "-t", cfg.Sbx.Template}
	for _, kit := range cfg.Sbx.Kits {
		sbxArgs = append(sbxArgs, "--kit", kit)
	}
	sbxArgs = append(sbxArgs, "claude")
	sbxArgs = append(sbxArgs, paths...)
	if out, err := exec.Command("sbx", sbxArgs...).CombinedOutput(); err != nil {
		return "", "", "", fmt.Errorf("sbx create failed: %s", out)
	}

	// ~/.claude-tabs symlink (マウントパスがホスト側パスになるため、sbx内HOMEにリンク)
	exec.Command("sbx", "exec", sbxName, "sh", "-c", "ln -sf "+claudeTabsDir+" $HOME/.claude-tabs").Run()

	// setup commands (best effort)
	for _, cmd := range cfg.Sbx.PostCreateCmds {
		if len(cmd) > 0 {
			args := append([]string{"exec", sbxName}, cmd...)
			exec.Command("sbx", args...).Run()
		}
	}

	// plugins install
	for _, pc := range cfg.Sbx.Plugins {
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
	tty, cwdPath, sbxName, err := worktreeCreate(repo, branch, baseBranch)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
		return
	}
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

func (s *server) handleSbxCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sbxName := r.URL.Query().Get("name")
	if sbxName == "" {
		http.Error(w, `{"message":"name is required"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	cfg := loadConfig()

	// clone_base をメインのマウントパスに
	cloneBase := cfg.Sbx.CloneBase
	if cloneBase == "" {
		cloneBase = "~/src"
	}
	cloneBase = expandHome(cloneBase)

	claudeTabsDir := expandHome("~/.claude-tabs")
	paths := []string{cloneBase, claudeTabsDir}
	if cfg.Worktree.Base != "" {
		paths = append(paths, expandHome(cfg.Worktree.Base))
	}
	paths = append(paths, cfg.Sbx.DefaultMounts...)

	template := cfg.Sbx.Template
	if template == "" {
		template = "my-sbx:latest"
	}

	sbxArgs := []string{"create", "--name", sbxName, "-t", template}
	for _, kit := range cfg.Sbx.Kits {
		sbxArgs = append(sbxArgs, "--kit", kit)
	}
	sbxArgs = append(sbxArgs, "claude")
	sbxArgs = append(sbxArgs, paths...)
	if out, err := exec.Command("sbx", sbxArgs...).CombinedOutput(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "sbx create failed: " + string(out)})
		return
	}

	// ~/.claude-tabs symlink (マウントパスがホスト側パスになるため、sbx内HOMEにリンク)
	exec.Command("sbx", "exec", sbxName, "sh", "-c", "ln -sf "+claudeTabsDir+" $HOME/.claude-tabs").Run()

	// post-create commands
	for _, cmd := range cfg.Sbx.PostCreateCmds {
		if len(cmd) > 0 {
			args := append([]string{"exec", sbxName}, cmd...)
			exec.Command("sbx", args...).Run()
		}
	}

	// plugins install
	for _, pc := range cfg.Sbx.Plugins {
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

	// 新タブでclaude起動
	ts := getTerminalScripts(cfg)
	command := fmt.Sprintf("sbx run --name %s claude", sbxName)
	script := strings.ReplaceAll(ts.NewTab, "{{COMMAND}}", command)
	ttyOut, _ := exec.Command("osascript", "-e", script).Output()
	tty := strings.TrimSpace(string(ttyOut))

	if tty != "" {
		s.mu.Lock()
		s.pendingTTY[cloneBase] = tty
		s.pendingName[cloneBase] = sbxName
		s.mu.Unlock()
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "sbx created: " + sbxName})
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

func handleSbxDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	sbxName := r.URL.Query().Get("name")
	if sbxName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "name is required"})
		return
	}
	if out, err := exec.Command("sbx", "rm", "-f", sbxName).CombinedOutput(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "sbx rm failed: " + string(out)})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "Deleted " + sbxName})
}

func handleRepoList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	sbxName := r.URL.Query().Get("sbx")
	cfg := loadConfig()
	if sbxName != "" {
		// sbx内のリポジトリを動的に取得（config記載のbase配下のみ）
		var bases []string
		if cfg.Sbx.RepositoryBase != "" {
			bases = append(bases, expandHome(cfg.Sbx.RepositoryBase))
		}
		if cfg.Worktree.Base != "" {
			bases = append(bases, expandHome(cfg.Worktree.Base))
		}
		cloneBase := cfg.Sbx.CloneBase
		if cloneBase == "" {
			cloneBase = "~/src"
		}
		bases = append(bases, expandHome(cloneBase))

		type RepoWithBranch struct {
			Path   string `json:"path"`
			Branch string `json:"branch"`
		}
		var repos []RepoWithBranch
		for _, base := range bases {
			out, err := exec.Command("sbx", "exec", sbxName, "find", base, "-name", ".git", "-type", "d", "-maxdepth", "4").CombinedOutput()
			if err != nil {
				continue
			}
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				repoPath := filepath.Dir(line)
				branch := ""
				if bOut, err := exec.Command("sbx", "exec", sbxName, "git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
					branch = strings.TrimSpace(string(bOut))
				}
				repos = append(repos, RepoWithBranch{Path: repoPath, Branch: branch})
			}
		}
		if repos == nil {
			repos = []RepoWithBranch{}
		}
		json.NewEncoder(w).Encode(repos)
		return
	}

	// ホスト側スキャン（従来互換）
	if cfg.Sbx.RepositoryBase == "" {
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
	scanGitRepos(cfg.Sbx.RepositoryBase, 4)
	scanGitRepos(cfg.Worktree.Base, 2)
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
	var fullPath string
	if filepath.IsAbs(repoPath) {
		// sbx内のフルパス
		fullPath = repoPath
	} else {
		// 相対パス（従来互換）
		fullPath = filepath.Join(cfg.Sbx.RepositoryBase, repoPath)
		if cfg.Worktree.Base != "" {
			wtPath := filepath.Join(cfg.Worktree.Base, repoPath)
			if _, err := os.Stat(wtPath); err == nil {
				fullPath = wtPath
			}
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
func createWorktreeOnly(repo, branch, baseBranch string) (wtPath, resolvedBranch string, isRemote bool, err error) {
	var prNumber string
	branch, prNumber, err = resolveBranch(branch)
	if err != nil {
		return
	}
	resolvedBranch = branch

	cfg := loadConfig()

	// repository_base, clone_base からリポジトリ検索
	var repoPath string
	searchBases := []string{}
	if cfg.Sbx.RepositoryBase != "" {
		searchBases = append(searchBases, cfg.Sbx.RepositoryBase)
	}
	searchBases = append(searchBases, getCloneBase())
	for _, base := range searchBases {
		if repoPath != "" {
			break
		}
		filepath.WalkDir(base, func(path string, d fs.DirEntry, walkErr error) error {
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
			rel, _ := filepath.Rel(base, path)
			if strings.Count(rel, string(filepath.Separator)) >= 4 {
				return filepath.SkipDir
			}
			return nil
		})
	}
	if repoPath == "" {
		err = fmt.Errorf("repository not found: %s", repo)
		return
	}

	wtBase := cfg.Worktree.Base
	if wtBase == "" {
		home, _ := os.UserHomeDir()
		wtBase = filepath.Join(home, "worktrees")
	}

	if out, fetchErr := exec.Command("git", "-C", repoPath, "fetch", "origin").CombinedOutput(); fetchErr != nil {
		err = fmt.Errorf("git fetch failed: %s", out)
		return
	}
	if baseBranch != "" {
		if exec.Command("git", "-C", repoPath, "rev-parse", "--verify", baseBranch).Run() != nil &&
			exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "origin/"+baseBranch).Run() != nil {
			err = fmt.Errorf("base branch not found: %s", baseBranch)
			return
		}
	}
	if exec.Command("git", "-C", repoPath, "rev-parse", "origin/"+branch).Run() == nil {
		isRemote = true
	}

	dirPrefix := ""
	if prNumber != "" {
		dirPrefix = "pr" + prNumber + "-"
	}
	safeBranch := strings.ReplaceAll(branch, "/", "__")
	wtPath = filepath.Join(wtBase, repo, dirPrefix+safeBranch)

	if _, statErr := os.Stat(wtPath); statErr == nil {
		return // already exists
	}

	if isRemote {
		if out, addErr := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, "origin/"+branch).CombinedOutput(); addErr != nil {
			err = fmt.Errorf("git worktree add failed: %s", out)
			return
		}
	} else if exec.Command("git", "-C", repoPath, "rev-parse", "--verify", branch).Run() == nil {
		if out, addErr := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, branch).CombinedOutput(); addErr != nil {
			err = fmt.Errorf("git worktree add failed: %s", out)
			return
		}
	} else {
		args := []string{"-C", repoPath, "worktree", "add", wtPath, "-b", branch}
		if baseBranch != "" {
			args = append(args, baseBranch)
		}
		if out, addErr := exec.Command("git", args...).CombinedOutput(); addErr != nil {
			err = fmt.Errorf("git worktree add (new branch) failed: %s", out)
			return
		}
	}
	return
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

	// Detect PR number for display name prefix
	prNumber := ""
	if strings.HasPrefix(branch, "https://github.com/") && strings.Contains(branch, "/pull/") {
		parts := strings.Split(branch, "/pull/")
		if len(parts) == 2 {
			prNumber = strings.TrimRight(parts[1], "/")
		}
	}

	// repo may be an absolute path from sbx (e.g. /Users/shshimamo/src/claude-tabs)
	// extract the base name for createWorktreeOnly which searches by name
	repoName := filepath.Base(repo)
	wtPath, resolvedBranch, _, err := createWorktreeOnly(repoName, branch, baseBranch)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
		return
	}

	// Determine display name prefix
	prefix := "wt-"
	if prNumber != "" {
		prefix = "pr" + prNumber + "-"
	}
	displayName := prefix + repoName + "/" + resolvedBranch

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
		s.pendingName[wtPath] = displayName
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
	mux.HandleFunc("/api/sbx/create", srv.handleSbxCreate)
	mux.HandleFunc("/api/sbx/dockerfile", handleSbxDockerfile)
	mux.HandleFunc("/api/sbx/build-template", handleSbxBuildTemplate)
	mux.HandleFunc("/api/sbx/list", handleSbxList)
	mux.HandleFunc("/api/sbx/delete", handleSbxDelete)
	mux.HandleFunc("/api/sbx/repos", handleRepoList)
	mux.HandleFunc("/api/sbx/branches", handleSbxBranches)
	mux.HandleFunc("/api/sbx/run", srv.handleSbxRun)
	mux.HandleFunc("/api/sbx/attach-worktree", srv.handleSbxAttachWorktree)
	mux.HandleFunc("/api/git/clone", handleGitClone)
	mux.HandleFunc("/api/git/repos", handleRepoListWithBranch)
	mux.HandleFunc("/api/git/branches", handleRepoBranches)
	mux.HandleFunc("/api/git/checkout", handleRepoCheckout)
	mux.HandleFunc("/api/presets", handlePresets)
	mux.HandleFunc("/api/sessions/memo", srv.handleMemo)
	mux.HandleFunc("/api/config", handleConfig)
	mux.HandleFunc("/api/conversations", handleConversations)
	mux.HandleFunc("/api/projects", handleProjects)
	mux.HandleFunc("/api/projects/session-map", handleProjectSessionMap)
	mux.HandleFunc("/api/projects/reorder", handleProjectReorder)
	mux.HandleFunc("/api/projects/session-order", handleSessionOrder)

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
