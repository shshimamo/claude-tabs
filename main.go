package main

import (
	"embed"
	"encoding/json"
	"fmt"
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

const addr = "localhost:6277"

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
}

// HistoryMessage is a simplified conversation message
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// HookInput is the JSON received from Claude Code hooks via stdin
type HookInput struct {
	SessionID string         `json:"session_id"`
	CWD       string         `json:"cwd"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
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

	// Client mode
	if isServerRunning() {
		browser.OpenURL("http://" + addr)
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
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
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
		// Mark old sessions with same PID as terminated
		if session.PID > 0 {
			markOldSessionsTerminated(session.PID, session.SessionID)
		}
	case "UserPromptSubmit":
		session.Status = "ai_working"
		session.Question = ""
	case "AskUserQuestion":
		session.Status = "waiting_input"
		if q, ok := input.ToolInput["question"].(string); ok {
			session.Question = q
		}
	case "PermissionRequest":
		session.Status = "permission_required"
		session.Question = ""
	case "Stop":
		session.Status = "idle"
		session.Question = ""
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
	mu       sync.RWMutex
	sessions map[string]*Session
	clients  map[*websocket.Conn]bool
	clientMu sync.Mutex
	upgrader websocket.Upgrader
}

func newServer() *server {
	return &server{
		sessions: make(map[string]*Session),
		clients:  make(map[*websocket.Conn]bool),
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
	s.sessions[session.SessionID] = &session
	s.mu.Unlock()
	s.broadcastSessions()
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

func (s *server) handleFocusTerminal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	s.mu.RLock()
	session, ok := s.sessions[id]
	var pid int
	if ok {
		pid = session.PID
	}
	s.mu.RUnlock()

	if !ok || pid <= 0 {
		http.Error(w, "session not found or no PID", http.StatusBadRequest)
		return
	}

	pidStr := strconv.Itoa(pid)

	// Try tty matching first
	found := false
	if out, err := exec.Command("ps", "-o", "tty=", "-p", pidStr).Output(); err == nil {
		tty := "/dev/" + strings.TrimSpace(string(out))
		script := fmt.Sprintf(`
tell application "iTerm2"
	activate
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is "%s" then
					select t
					tell w to select
					return "found"
				end if
			end repeat
		end repeat
	end repeat
	return "not_found"
end tell`, tty)
		if result, err := exec.Command("osascript", "-e", script).Output(); err == nil {
			if strings.TrimSpace(string(result)) == "found" {
				found = true
			}
		}
	}

	// Fallback: match by PID in tab name
	if !found {
		script := fmt.Sprintf(`
tell application "iTerm2"
	activate
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if name of s contains "%s" then
					select t
					tell w to select
					return "found"
				end if
			end repeat
		end repeat
	end repeat
	return "not_found"
end tell`, pidStr)
		if result, err := exec.Command("osascript", "-e", script).Output(); err == nil {
			if strings.TrimSpace(string(result)) == "found" {
				found = true
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if found {
		json.NewEncoder(w).Encode(map[string]string{"result": "found"})
	} else {
		json.NewEncoder(w).Encode(map[string]string{"result": "not_found"})
	}
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

	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()

	os.Remove(filepath.Join(sessionsDir(), id+".json"))
	w.WriteHeader(http.StatusNoContent)
	s.broadcastSessions()
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
	mux.HandleFunc("/api/sessions/name", srv.handleRenameSession)
	mux.HandleFunc("/api/sessions/focus", srv.handleFocusTerminal)
	mux.HandleFunc("/api/sessions/history", srv.handleHistory)

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

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude-tabs: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("claude-tabs server running on http://%s\n", addr)

	go func() {
		time.Sleep(100 * time.Millisecond)
		browser.OpenURL("http://" + addr)
	}()

	http.Serve(ln, mux)
}
