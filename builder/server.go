package builder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"sync"

	"github.com/etl-madness/flow"
)

type Server struct {
	storage    *Storage
	catalog    *ComponentCatalog
	template   *template.Template
	port       int
	authToken  string
	sessions   map[string]time.Time
	sessionsMu sync.Mutex
}

const sessionCookieName = "flow_builder_session"

func generateSecureToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) GetAuthToken() string {
	return s.authToken
}

func (s *Server) SetAuthToken(token string) {
	s.authToken = token
}

func (s *Server) Port() int {
	return s.port
}

func (s *Server) generateCSRFToken() string {
	// Deprecated: CSRF validation now relies on X-Requested-With header
	return ""
}

func NewServer(storage *Storage, port int) (*Server, error) {
	tmpl, err := BuildTemplate()
	if err != nil {
		return nil, fmt.Errorf("failed to build templates: %w", err)
	}

	token, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secure auth token: %w", err)
	}

	s := &Server{
		storage:   storage,
		catalog:   GetCatalog(),
		template:  tmpl,
		port:      port,
		authToken: token,
		sessions:  make(map[string]time.Time),
	}
	return s, nil
}

type PageData struct {
	AllScripts            []Script
	ActiveScript          Script
	Components            []ComponentMeta
	Categories            []string
	CatalogJSON           template.JS
	VariableNodes         []PipelineNode
	DatabaseNodes         []PipelineNode
	PreflightNodes        []PipelineNode
	FlowNodes             []PipelineNode
	DefaultConfigContent  string
	DefaultOptionsContent string
	CSRFToken             string
}

func (s *Server) getActiveScript(r *http.Request) (Script, error) {
	scriptIDStr := r.URL.Query().Get("script_id")
	if scriptIDStr != "" {
		if id, err := strconv.ParseInt(scriptIDStr, 10, 64); err == nil {
			script, err := s.storage.GetScript(id)
			if err == nil {
				return *script, nil
			}
		}
	}
	return s.storage.GetOrCreateDefaultScript()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	scripts, err := s.storage.ListScripts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	activeScript, err := s.getActiveScript(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	varNodes, err := s.storage.GetNodes(activeScript.ID, "variables")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get variable nodes: %v", err), http.StatusInternalServerError)
		return
	}
	dbNodes, err := s.storage.GetNodes(activeScript.ID, "databases")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get database nodes: %v", err), http.StatusInternalServerError)
		return
	}
	preNodes, err := s.storage.GetNodeTree(activeScript.ID, "preflight")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get preflight nodes: %v", err), http.StatusInternalServerError)
		return
	}
	flowNodes, err := s.storage.GetNodeTree(activeScript.ID, "flow")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get flow nodes: %v", err), http.StatusInternalServerError)
		return
	}

	configContent := s.storage.GetOrCreateDefaultConfig()
	optionsContent := s.storage.GetOrCreateDefaultOptions()

	catJSON, err := json.Marshal(s.catalog.Components)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to marshal catalog: %v", err), http.StatusInternalServerError)
		return
	}

	data := PageData{
		AllScripts:            scripts,
		ActiveScript:          activeScript,
		Components:            s.catalog.Components,
		Categories:            s.catalog.Categories,
		CatalogJSON:           template.JS(catJSON),
		VariableNodes:         varNodes,
		DatabaseNodes:         dbNodes,
		PreflightNodes:        preNodes,
		FlowNodes:             flowNodes,
		DefaultConfigContent:  configContent,
		DefaultOptionsContent: optionsContent,
		CSRFToken:             "",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.template.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

func (s *Server) handleCanvas(w http.ResponseWriter, r *http.Request) {
	activeScript, err := s.getActiveScript(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	varNodes, err := s.storage.GetNodes(activeScript.ID, "variables")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get variable nodes: %v", err), http.StatusInternalServerError)
		return
	}
	dbNodes, err := s.storage.GetNodes(activeScript.ID, "databases")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get database nodes: %v", err), http.StatusInternalServerError)
		return
	}
	preNodes, err := s.storage.GetNodeTree(activeScript.ID, "preflight")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get preflight nodes: %v", err), http.StatusInternalServerError)
		return
	}
	flowNodes, err := s.storage.GetNodeTree(activeScript.ID, "flow")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get flow nodes: %v", err), http.StatusInternalServerError)
		return
	}

	data := PageData{
		ActiveScript:   activeScript,
		VariableNodes:  varNodes,
		DatabaseNodes:  dbNodes,
		PreflightNodes: preNodes,
		FlowNodes:      flowNodes,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.template.ExecuteTemplate(w, "canvas_nodes", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type NodeRequest struct {
	ScriptID     int64             `json:"script_id"`
	NodeID       int64             `json:"node_id"`
	ParentNodeID *int64            `json:"parent_node_id,omitempty"`
	NodeType     string            `json:"node_type"`
	Section      string            `json:"section"`
	Attributes   map[string]string `json:"attributes"`
	Content      string            `json:"content"`
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If authToken is explicitly empty, bypass authentication
		if s.authToken == "" {
			next(w, r)
			return
		}

		// 1. Check query parameter ?token=...
		reqToken := r.URL.Query().Get("token")
		if reqToken != "" && subtle.ConstantTimeCompare([]byte(reqToken), []byte(s.authToken)) == 1 {
			// Generate and store session cookie
			sessionID, err := generateSecureToken(24)
			if err == nil {
				s.sessionsMu.Lock()
				if len(s.sessions) > 100 {
					now := time.Now()
					for k, exp := range s.sessions {
						if now.After(exp) {
							delete(s.sessions, k)
						}
					}
				}
				s.sessions[sessionID] = time.Now().Add(24 * time.Hour)
				s.sessionsMu.Unlock()

				http.SetCookie(w, &http.Cookie{
					Name:     sessionCookieName,
					Value:    sessionID,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteStrictMode,
				})
			}

			// If visiting root with token, redirect to clean the URL
			if r.URL.Path == "/" {
				q := r.URL.Query()
				q.Del("token")
				target := "/"
				if len(q) > 0 {
					target += "?" + q.Encode()
				}
				http.Redirect(w, r, target, http.StatusSeeOther)
				return
			}

			next(w, r)
			return
		}

		// 2. Check Authorization header: Bearer <token>
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(bearerToken), []byte(s.authToken)) == 1 {
				next(w, r)
				return
			}
		}

		// 3. Check Session Cookie
		if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
			s.sessionsMu.Lock()
			expiry, exists := s.sessions[cookie.Value]
			s.sessionsMu.Unlock()

			if exists {
				if time.Now().Before(expiry) {
					next(w, r)
					return
				}
				s.sessionsMu.Lock()
				delete(s.sessions, cookie.Value)
				s.sessionsMu.Unlock()
			}
		}

		// Deny access
		w.Header().Set("WWW-Authenticate", `Bearer realm="FlowBuilder"`)
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "Unauthorized: valid session cookie or token required",
			})
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>401 Unauthorized - Flow Builder</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
.card { background: #1e293b; padding: 2rem; border-radius: 0.75rem; box-shadow: 0 10px 25px rgba(0,0,0,0.5); max-width: 450px; width: 100%%; text-align: center; border: 1px solid #334155; }
h1 { color: #ef4444; font-size: 1.5rem; margin-bottom: 0.5rem; }
p { color: #94a3b8; font-size: 0.875rem; line-height: 1.5; margin-bottom: 1.5rem; }
input[type=text] { width: 100%%; box-sizing: border-box; padding: 0.6rem 0.8rem; background: #0f172a; border: 1px solid #475569; border-radius: 0.375rem; color: #fff; margin-bottom: 1rem; font-family: monospace; font-size: 0.875rem; }
button { background: #2563eb; color: white; border: none; padding: 0.6rem 1.2rem; border-radius: 0.375rem; font-weight: 600; cursor: pointer; width: 100%%; }
button:hover { background: #1d4ed8; }
</style>
</head>
<body>
<div class="card">
  <h1>Access Restricted</h1>
  <p>Flow Builder is running in single-user mode. Only the user who launched this process may access the interface and filesystem APIs.</p>
  <form method="GET" action="/">
    <input type="text" name="token" placeholder="Enter session token..." required autofocus />
    <button type="submit">Authenticate</button>
  </form>
</div>
</body>
</html>`)
	}
}

func (s *Server) validateCSRF(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Requested-With"), "xmlhttprequest")
}

func (s *Server) sanitizePath(path string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get cwd: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if !strings.HasPrefix(absPath, cwd) {
		return "", fmt.Errorf("access denied: path %s is outside the working directory", path)
	}

	return absPath, nil
}

func (s *Server) sanitizeWritePath(filename string) (string, error) {
	sandboxDir := "exports"
	if err := os.MkdirAll(sandboxDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create sandbox directory: %w", err)
	}

	baseName := filepath.Base(filename)
	if baseName == "." || baseName == ".." {
		return "", fmt.Errorf("invalid filename")
	}

	return filepath.Join(sandboxDir, baseName), nil
}

func (s *Server) handleAddNode(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Section == "" {
		req.Section = "flow"
	}

	created, err := s.storage.AddNodeWithParent(req.ScriptID, req.Section, req.NodeType, req.Attributes, req.Content, req.ParentNodeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req.NodeType == "if" && created != nil {
		_, _, _ = s.storage.EnsureIfBranches(req.ScriptID, req.Section, created.ID)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := s.storage.UpdateNodeWithSection(req.NodeID, req.Section, req.Attributes, req.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	node, err := s.storage.GetNode(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := s.storage.DeleteNode(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleMoveNode(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	direction := r.URL.Query().Get("dir")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := s.storage.MoveNode(id, direction); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type ReorderRequest struct {
	ScriptID int64   `json:"script_id"`
	Section  string  `json:"section"`
	NodeIDs  []int64 `json:"node_ids"`
}

func (s *Server) handleReorderNodes(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.storage.ReorderNodes(req.ScriptID, req.Section, req.NodeIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	activeScript, err := s.getActiveScript(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	varNodes, _ := s.storage.GetNodes(activeScript.ID, "variables")
	dbNodes, _ := s.storage.GetNodes(activeScript.ID, "databases")
	preNodes, _ := s.storage.GetNodeTree(activeScript.ID, "preflight")
	flowNodes, _ := s.storage.GetNodeTree(activeScript.ID, "flow")

	xmlContent := GenerateXML(activeScript.Name, varNodes, dbNodes, preNodes, flowNodes)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xmlContent))
}

func (s *Server) handleNewScript(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	script, err := s.storage.CreateScript(req.Name, req.Description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(script)
}

func (s *Server) handleCopyScript(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID      int64  `json:"id"`
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cloned, err := s.storage.CopyScript(req.ID, req.NewName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"script":  cloned,
	})
}

func (s *Server) handleDeleteScript(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var id int64
	idStr := r.URL.Query().Get("id")
	if idStr != "" {
		parsed, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid script ID", http.StatusBadRequest)
			return
		}
		id = parsed
	} else {
		var req struct {
			ID int64 `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id = req.ID
	}

	if err := s.storage.DeleteScript(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	remaining, _ := s.storage.ListScripts()
	var nextScriptID int64
	if len(remaining) > 0 {
		nextScriptID = remaining[0].ID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":        true,
		"next_script_id": nextScriptID,
		"remaining":      remaining,
	})
}

func (s *Server) handleListScripts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	scripts, err := s.storage.ListScripts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scripts)
}

func (s *Server) handleImportScript(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FilePath string `json:"file_path"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filePath := strings.TrimSpace(req.FilePath)
	if filePath == "" {
		http.Error(w, "file_path is required", http.StatusBadRequest)
		return
	}

	// Restrict to current working directory and subdirectories
	cwd, err := os.Getwd()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(absPath, cwd) {
		http.Error(w, "Access denied: file must be within the current working directory", http.StatusForbidden)
		return
	}

	if strings.ToLower(filepath.Ext(absPath)) != ".xml" {
		http.Error(w, "Invalid file type: only .xml files are allowed", http.StatusBadRequest)
		return
	}

	// Validate against flow.ValidateAST
	fileBytes, err := os.ReadFile(absPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	cfg, err := flow.ParseXMLConfig(fileBytes)
	if err != nil {
		http.Error(w, fmt.Sprintf("XML parsing error: %v", err), http.StatusBadRequest)
		return
	}

	if err := flow.ValidateAST(cfg.PreflightNodes, cfg.FlowNodes, cfg.Databases); err != nil {
		http.Error(w, fmt.Sprintf("AST Validation failed: %v", err), http.StatusBadRequest)
		return
	}

	imported, err := ImportPipelineFromXML(absPath, req.Name, s.storage)
	if err != nil {
		http.Error(w, fmt.Sprintf("Import failed: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"script":  imported,
	})
}

func (s *Server) handlePurgeDatabase(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defaultScript, err := s.storage.PurgeDatabase()
	if err != nil {
		http.Error(w, fmt.Sprintf("Purge failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":           true,
		"message":           "Database successfully purged and reset to factory defaults",
		"default_script_id": defaultScript.ID,
	})
}

func (s *Server) handleSaveFile(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ScriptID int64  `json:"script_id"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Filename == "" {
		req.Filename = "scripts.xml"
	}

	writePath, err := s.sanitizeWritePath(req.Filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	script, err := s.storage.GetScript(req.ScriptID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	varNodes, _ := s.storage.GetNodes(script.ID, "variables")
	dbNodes, _ := s.storage.GetNodes(script.ID, "databases")
	preNodes, _ := s.storage.GetNodeTree(script.ID, "preflight")
	flowNodes, _ := s.storage.GetNodeTree(script.ID, "flow")

	xmlContent := GenerateXML(script.Name, varNodes, dbNodes, preNodes, flowNodes)

	if err := os.WriteFile(writePath, []byte(xmlContent), 0644); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write file %s: %v", writePath, err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(fmt.Sprintf("Successfully saved pipeline to %s", writePath)))
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writePath, err := s.sanitizeWritePath(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.storage.SaveConfigFile(req.Name, req.Content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = os.WriteFile(writePath, []byte(req.Content), 0644)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSaveOptions(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writePath, err := s.sanitizeWritePath(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.storage.SaveOptionsFile(req.Name, req.Content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = os.WriteFile(writePath, []byte(req.Content), 0644)

	w.WriteHeader(http.StatusOK)
}

type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

type BrowseResponse struct {
	CurrentDir string      `json:"current_dir"`
	ParentDir  string      `json:"parent_dir"`
	Entries    []FileEntry `json:"entries"`
}

func (s *Server) handleBrowseFiles(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = "."
	}

	cleanDir, err := s.sanitizePath(dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read directory: %v", err), http.StatusBadRequest)
		return
	}

	extFilter := strings.ToLower(r.URL.Query().Get("ext"))

	fileEntries := []FileEntry{}
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}

		entryPath := filepath.ToSlash(filepath.Join(cleanDir, name))
		if cleanDir == "." {
			entryPath = filepath.ToSlash(name)
		}

		isDir := entry.IsDir()
		if !isDir && extFilter != "" && !strings.HasSuffix(strings.ToLower(name), extFilter) {
			continue
		}

		var size int64
		var modTime time.Time
		if info, err := entry.Info(); err == nil {
			size = info.Size()
			modTime = info.ModTime()
		}

		fileEntries = append(fileEntries, FileEntry{
			Name:    name,
			Path:    entryPath,
			IsDir:   isDir,
			Size:    size,
			ModTime: modTime,
		})
	}

	// Sort directories first, then files alphabetically
	sort.Slice(fileEntries, func(i, j int) bool {
		if fileEntries[i].IsDir != fileEntries[j].IsDir {
			return fileEntries[i].IsDir
		}
		return strings.ToLower(fileEntries[i].Name) < strings.ToLower(fileEntries[j].Name)
	})

	parentDir := filepath.ToSlash(filepath.Dir(cleanDir))
	if cleanDir == "." || cleanDir == "" {
		parentDir = ""
	}

	resp := BrowseResponse{
		CurrentDir: filepath.ToSlash(cleanDir),
		ParentDir:  parentDir,
		Entries:    fileEntries,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleQuickFiles(w http.ResponseWriter, r *http.Request) {
	ext := r.URL.Query().Get("ext")
	if ext == "" {
		ext = ".xml"
	}
	ext = strings.ToLower(ext)

	root := r.URL.Query().Get("root")
	if root == "" {
		root = "."
	}

	cleanRoot, err := s.sanitizePath(root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files := []string{}
	_ = filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != cleanRoot && (strings.HasPrefix(name, ".") || name == "bin" || name == "obj" || name == "node_modules" || name == "dist") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(name), ext) {
			slashPath := filepath.ToSlash(path)
			slashPath = strings.TrimPrefix(slashPath, "./")
			files = append(files, slashPath)
		}
		return nil
	})

	sort.Strings(files)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(files)
}

func (s *Server) handleExecuteStream(w http.ResponseWriter, r *http.Request) {
	if !s.validateCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sendSSE := func(eventType string, data any) {
		payload, _ := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}

	scriptIDStr := r.URL.Query().Get("script_id")
	scriptFile := strings.TrimSpace(r.URL.Query().Get("file"))
	configFile := strings.TrimSpace(r.URL.Query().Get("config"))
	optionsFile := strings.TrimSpace(r.URL.Query().Get("options"))
	source := strings.TrimSpace(r.URL.Query().Get("source")) // "builder" or "file"
	preflightOnly := r.URL.Query().Get("preflight") == "true" || r.URL.Query().Get("preflight") == "1"

	if source == "builder" {
		if scriptFile == "" {
			scriptFile = "temp_run_script.xml"
		}
		if scriptIDStr != "" {
			if id, err := strconv.ParseInt(scriptIDStr, 10, 64); err == nil {
				if script, err := s.storage.GetScript(id); err == nil {
					varNodes, _ := s.storage.GetNodes(script.ID, "variables")
					dbNodes, _ := s.storage.GetNodes(script.ID, "databases")
					preNodes, _ := s.storage.GetNodeTree(script.ID, "preflight")
					flowNodes, _ := s.storage.GetNodeTree(script.ID, "flow")
					xmlContent := GenerateXML(script.Name, varNodes, dbNodes, preNodes, flowNodes)
					if err := os.WriteFile(scriptFile, []byte(xmlContent), 0644); err != nil {
						sendSSE("done", map[string]any{"type": "done", "status": "ERROR", "error": fmt.Sprintf("Failed to write draft XML: %v", err)})
						return
					}
				}
			}
		}
		sendSSE("log", map[string]any{"type": "log", "message": fmt.Sprintf("[FLOW] Exported builder draft to: %s", scriptFile)})
	} else {
		// Filesystem file mode
		if scriptFile == "" && optionsFile == "" {
			scriptFile = "scripts.xml"
		}
		if scriptFile != "" {
			if _, err := os.Stat(scriptFile); os.IsNotExist(err) {
				sendSSE("done", map[string]any{"type": "done", "status": "ERROR", "error": fmt.Sprintf("Script file not found: %s", scriptFile)})
				return
			}
			sendSSE("log", map[string]any{"type": "log", "message": fmt.Sprintf("[FLOW] Using filesystem script: %s", scriptFile)})
		} else if optionsFile != "" {
			sendSSE("log", map[string]any{"type": "log", "message": "[FLOW] No script file selected; using runtime default script behavior with options override."})
		}
	}

	if configFile != "" {
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			sendSSE("done", map[string]any{"type": "done", "status": "ERROR", "error": fmt.Sprintf("Config file not found: %s", configFile)})
			return
		}
	}

	if optionsFile != "" {
		if _, err := os.Stat(optionsFile); os.IsNotExist(err) {
			sendSSE("done", map[string]any{"type": "done", "status": "ERROR", "error": fmt.Sprintf("Options file not found: %s", optionsFile)})
			return
		}
	}

	exePath, err := os.Executable()
	if err != nil {
		exePath = "go-flow"
	}

	var args []string
	if optionsFile != "" {
		args = append(args, "-options", optionsFile)
	}
	if scriptFile != "" {
		args = append(args, "-file", scriptFile)
	}
	if configFile != "" {
		args = append(args, "-config", configFile)
	}
	if preflightOnly {
		args = append(args, "-preflight")
	}
	args = append(args, "-format", "stream")

	sendSSE("log", map[string]any{"type": "log", "message": fmt.Sprintf("[FLOW] Running: %s %v", filepath.Base(exePath), args)})

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, exePath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sendSSE("done", map[string]any{"type": "done", "status": "ERROR", "error": err.Error(), "duration": time.Since(startTime).String()})
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		sendSSE("done", map[string]any{"type": "done", "status": "ERROR", "error": err.Error(), "duration": time.Since(startTime).String()})
		return
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		sendSSE("log", map[string]any{"type": "log", "message": line})

		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, ",") {
			parts := strings.Split(trimmed, ",")
			if len(parts) >= 12 {
				evtType := strings.ToLower(parts[4])
				normEvt := strings.ReplaceAll(evtType, "_", ".")
				if normEvt == "node.started" || normEvt == "node.finished" || normEvt == "run.started" || normEvt == "run.finished" {
					readCount, _ := strconv.ParseInt(parts[len(parts)-3], 10, 64)
					writtenCount, _ := strconv.ParseInt(parts[len(parts)-2], 10, 64)
					affectedCount, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
					errorMsg := strings.Join(parts[8:len(parts)-3], ",")

					sendSSE("node_event", map[string]any{
						"type":          "node_event",
						"timestamp":     parts[0],
						"run_id":        parts[1],
						"execution_id":  parts[2],
						"sequence":      parts[3],
						"event_type":    evtType,
						"kind":          parts[5],
						"node_id":       parts[6],
						"status":        parts[7],
						"error":         errorMsg,
						"rows_read":     readCount,
						"rows_written":  writtenCount,
						"rows_affected": affectedCount,
					})
				}
			}
		}
	}

	cmdErr := cmd.Wait()
	duration := time.Since(startTime)

	if cmdErr != nil {
		sendSSE("done", map[string]any{
			"type":     "done",
			"status":   "FAILED",
			"error":    cmdErr.Error(),
			"duration": duration.String(),
		})
	} else {
		sendSSE("done", map[string]any{
			"type":     "done",
			"status":   "SUCCESS",
			"duration": duration.String(),
		})
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.requireAuth(s.handleIndex))
	mux.HandleFunc("/api/canvas", s.requireAuth(s.handleCanvas))
	mux.HandleFunc("/api/preview", s.requireAuth(s.handlePreview))
	mux.HandleFunc("/api/nodes/add", s.requireAuth(s.handleAddNode))
	mux.HandleFunc("/api/nodes/get", s.requireAuth(s.handleGetNode))
	mux.HandleFunc("/api/nodes/update", s.requireAuth(s.handleUpdateNode))
	mux.HandleFunc("/api/nodes/delete", s.requireAuth(s.handleDeleteNode))
	mux.HandleFunc("/api/nodes/move", s.requireAuth(s.handleMoveNode))
	mux.HandleFunc("/api/nodes/reorder", s.requireAuth(s.handleReorderNodes))
	mux.HandleFunc("/api/scripts/new", s.requireAuth(s.handleNewScript))
	mux.HandleFunc("/api/scripts/copy", s.requireAuth(s.handleCopyScript))
	mux.HandleFunc("/api/scripts/delete", s.requireAuth(s.handleDeleteScript))
	mux.HandleFunc("/api/scripts/import", s.requireAuth(s.handleImportScript))
	mux.HandleFunc("/api/scripts/list", s.requireAuth(s.handleListScripts))
	mux.HandleFunc("/api/db/purge", s.requireAuth(s.handlePurgeDatabase))
	mux.HandleFunc("/api/save_file", s.requireAuth(s.handleSaveFile))
	mux.HandleFunc("/api/config/save", s.requireAuth(s.handleSaveConfig))
	mux.HandleFunc("/api/options/save", s.requireAuth(s.handleSaveOptions))
	mux.HandleFunc("/api/execute/stream", s.requireAuth(s.handleExecuteStream))
	mux.HandleFunc("/api/files/browse", s.requireAuth(s.handleBrowseFiles))
	mux.HandleFunc("/api/files/quick", s.requireAuth(s.handleQuickFiles))

	return mux
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind builder server to %s: %w", addr, err)
	}
	defer listener.Close()

	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		s.port = tcpAddr.Port
	}

	log.Printf("Starting Flow Visual Builder at http://127.0.0.1:%d/?token=%s\n", s.port, s.authToken)
	return s.Serve(listener)
}

func (s *Server) Serve(listener net.Listener) error {
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		s.port = tcpAddr.Port
	}

	srv := &http.Server{
		Handler: s.Handler(),
	}

	return srv.Serve(listener)
}

func StartServer(port int, dbPath string) error {
	storage, err := NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize sqlite storage: %w", err)
	}

	srv, err := NewServer(storage, port)
	if err != nil {
		storage.Close()
		return fmt.Errorf("failed to create builder server: %w", err)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		storage.Close()
		return fmt.Errorf("failed to bind builder server to %s: %w", addr, err)
	}
	defer listener.Close()

	actualPort := listener.Addr().(*net.TCPAddr).Port
	srv.port = actualPort

	url := fmt.Sprintf("http://127.0.0.1:%d/?token=%s", actualPort, srv.authToken)
	log.Printf("Starting Flow Visual Builder at %s\n", url)

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(url)
	}()

	err = srv.Serve(listener)
	storage.Close()
	return err
}

func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	_ = exec.Command(cmd, args...).Start()
}
