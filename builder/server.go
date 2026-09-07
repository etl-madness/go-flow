package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

type Server struct {
	storage  *Storage
	catalog  *ComponentCatalog
	template *template.Template
	port     int
}

func NewServer(storage *Storage, port int) (*Server, error) {
	tmpl, err := BuildTemplate()
	if err != nil {
		return nil, fmt.Errorf("failed to build templates: %w", err)
	}

	return &Server{
		storage:  storage,
		catalog:  GetCatalog(),
		template: tmpl,
		port:     port,
	}, nil
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

	varNodes, _ := s.storage.GetNodes(activeScript.ID, "variables")
	dbNodes, _ := s.storage.GetNodes(activeScript.ID, "databases")
	preNodes, _ := s.storage.GetNodes(activeScript.ID, "preflight")
	flowNodes, _ := s.storage.GetNodes(activeScript.ID, "flow")

	configContent := s.storage.GetOrCreateDefaultConfig()
	optionsContent := s.storage.GetOrCreateDefaultOptions()

	catJSON, _ := json.Marshal(s.catalog.Components)

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

	varNodes, _ := s.storage.GetNodes(activeScript.ID, "variables")
	dbNodes, _ := s.storage.GetNodes(activeScript.ID, "databases")
	preNodes, _ := s.storage.GetNodes(activeScript.ID, "preflight")
	flowNodes, _ := s.storage.GetNodes(activeScript.ID, "flow")

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
	ScriptID   int64             `json:"script_id"`
	NodeID     int64             `json:"node_id"`
	NodeType   string            `json:"node_type"`
	Section    string            `json:"section"`
	Attributes map[string]string `json:"attributes"`
	Content    string            `json:"content"`
}

func (s *Server) handleAddNode(w http.ResponseWriter, r *http.Request) {
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

	_, err := s.storage.AddNode(req.ScriptID, req.Section, req.NodeType, req.Attributes, req.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := s.storage.UpdateNode(req.NodeID, req.Attributes, req.Content)
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
	preNodes, _ := s.storage.GetNodes(activeScript.ID, "preflight")
	flowNodes, _ := s.storage.GetNodes(activeScript.ID, "flow")

	xmlContent := GenerateXML(activeScript.Name, varNodes, dbNodes, preNodes, flowNodes)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xmlContent))
}

func (s *Server) handleNewScript(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleSaveFile(w http.ResponseWriter, r *http.Request) {
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

	script, err := s.storage.GetScript(req.ScriptID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	varNodes, _ := s.storage.GetNodes(script.ID, "variables")
	dbNodes, _ := s.storage.GetNodes(script.ID, "databases")
	preNodes, _ := s.storage.GetNodes(script.ID, "preflight")
	flowNodes, _ := s.storage.GetNodes(script.ID, "flow")

	xmlContent := GenerateXML(script.Name, varNodes, dbNodes, preNodes, flowNodes)

	if err := os.WriteFile(req.Filename, []byte(xmlContent), 0644); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write file %s: %v", req.Filename, err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(fmt.Sprintf("Successfully saved pipeline to %s", req.Filename)))
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
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

	if err := s.storage.SaveConfigFile(req.Name, req.Content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = os.WriteFile(req.Name, []byte(req.Content), 0644)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSaveOptions(w http.ResponseWriter, r *http.Request) {
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

	if err := s.storage.SaveOptionsFile(req.Name, req.Content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = os.WriteFile(req.Name, []byte(req.Content), 0644)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleExecuteStream(w http.ResponseWriter, r *http.Request) {
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
	scriptFile := r.URL.Query().Get("file")
	configFile := r.URL.Query().Get("config")
	optionsFile := r.URL.Query().Get("options")

	if scriptFile == "" {
		scriptFile = "temp_run_script.xml"
	}

	if scriptIDStr != "" {
		if id, err := strconv.ParseInt(scriptIDStr, 10, 64); err == nil {
			if script, err := s.storage.GetScript(id); err == nil {
				varNodes, _ := s.storage.GetNodes(script.ID, "variables")
				dbNodes, _ := s.storage.GetNodes(script.ID, "databases")
				preNodes, _ := s.storage.GetNodes(script.ID, "preflight")
				flowNodes, _ := s.storage.GetNodes(script.ID, "flow")
				xmlContent := GenerateXML(script.Name, varNodes, dbNodes, preNodes, flowNodes)
				_ = os.WriteFile(scriptFile, []byte(xmlContent), 0644)
			}
		}
	}

	sendSSE("log", map[string]any{"type": "log", "message": fmt.Sprintf("[FLOW] Prepared script file: %s", scriptFile)})

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

	buf := make([]byte, 1024)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			msg := string(buf[:n])
			sendSSE("log", map[string]any{"type": "log", "message": msg})
		}
		if err != nil {
			break
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

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/canvas", s.handleCanvas)
	mux.HandleFunc("/api/preview", s.handlePreview)
	mux.HandleFunc("/api/nodes/add", s.handleAddNode)
	mux.HandleFunc("/api/nodes/get", s.handleGetNode)
	mux.HandleFunc("/api/nodes/update", s.handleUpdateNode)
	mux.HandleFunc("/api/nodes/delete", s.handleDeleteNode)
	mux.HandleFunc("/api/nodes/move", s.handleMoveNode)
	mux.HandleFunc("/api/nodes/reorder", s.handleReorderNodes)
	mux.HandleFunc("/api/scripts/new", s.handleNewScript)
	mux.HandleFunc("/api/save_file", s.handleSaveFile)
	mux.HandleFunc("/api/config/save", s.handleSaveConfig)
	mux.HandleFunc("/api/options/save", s.handleSaveOptions)
	mux.HandleFunc("/api/execute/stream", s.handleExecuteStream)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting Flow Visual Builder at http://localhost:%d\n", s.port)
	return http.ListenAndServe(addr, mux)
}

func StartServer(port int, dbPath string) error {
	storage, err := NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize sqlite storage: %w", err)
	}
	defer storage.Close()

	srv, err := NewServer(storage, port)
	if err != nil {
		return fmt.Errorf("failed to create builder server: %w", err)
	}

	url := fmt.Sprintf("http://localhost:%d", port)
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(url)
	}()

	return srv.Start()
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