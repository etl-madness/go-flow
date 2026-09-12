package builder

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Script represents a pipeline script metadata record.
type Script struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PipelineNode represents a node step within a pipeline draft.
type PipelineNode struct {
	ID            int64             `json:"id"`
	ScriptID      int64             `json:"script_id"`
	Section       string            `json:"section"` // variables, databases, preflight, flow
	ParentNodeID  *int64            `json:"parent_node_id,omitempty"`
	NodeType      string            `json:"node_type"`
	SequenceOrder int               `json:"sequence_order"`
	Attributes    map[string]string `json:"attributes"`
	ContentText   string            `json:"content_text"`
	Children      []PipelineNode    `json:"children,omitempty"`
}

// GetThenBranch returns the <then> branch child node for conditional nodes, if present.
func (n PipelineNode) GetThenBranch() *PipelineNode {
	for i := range n.Children {
		if n.Children[i].NodeType == "then" {
			return &n.Children[i]
		}
	}
	return nil
}

// GetElseBranch returns the <else> branch child node for conditional nodes, if present.
func (n PipelineNode) GetElseBranch() *PipelineNode {
	for i := range n.Children {
		if n.Children[i].NodeType == "else" {
			return &n.Children[i]
		}
	}
	return nil
}

// Storage handles local SQLite persistence for builder drafts.
type Storage struct {
	db *sql.DB
	mu sync.Mutex
}

// NewStorage opens/creates the SQLite database and runs migrations.
func NewStorage(dbPath string) (*Storage, error) {
	if dbPath == "" {
		dbPath = "flow_builder.db"
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database %s: %w", dbPath, err)
	}

	db.SetMaxOpenConns(1)

	s := &Storage{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration error: %w", err)
	}

	return s, nil
}

// Close closes the database handle.
func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) migrate() error {
	queries := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE IF NOT EXISTS scripts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS pipeline_nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			script_id INTEGER NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
			section TEXT NOT NULL DEFAULT 'flow',
			parent_node_id INTEGER NULL REFERENCES pipeline_nodes(id) ON DELETE CASCADE,
			node_type TEXT NOT NULL,
			sequence_order INTEGER NOT NULL DEFAULT 0,
			attributes_json TEXT NOT NULL DEFAULT '{}',
			content_text TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_script_seq ON pipeline_nodes(script_id, section, sequence_order);`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_parent ON pipeline_nodes(parent_node_id);`,
		`CREATE TABLE IF NOT EXISTS config_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			content TEXT NOT NULL DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS options_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			content TEXT NOT NULL DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) ListScripts() ([]Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query("SELECT id, name, description, created_at, updated_at FROM scripts ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scripts []Script
	for rows.Next() {
		var sc Script
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Description, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		scripts = append(scripts, sc)
	}
	return scripts, nil
}

func (s *Storage) GetScript(id int64) (*Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var sc Script
	err := s.db.QueryRow("SELECT id, name, description, created_at, updated_at FROM scripts WHERE id = ?", id).
		Scan(&sc.ID, &sc.Name, &sc.Description, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (s *Storage) CreateScript(name, description string) (*Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec("INSERT INTO scripts (name, description, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)", name, description)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Script{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (s *Storage) GetOrCreateDefaultScript() (Script, error) {
	scripts, err := s.ListScripts()
	if err != nil {
		return Script{}, err
	}
	if len(scripts) > 0 {
		return scripts[0], nil
	}

	sc, err := s.CreateScript("default_pipeline", "Default pipeline draft")
	if err != nil {
		return Script{}, err
	}

	// Seed basic sample nodes
	_, _ = s.AddNode(sc.ID, "variables", "variable", map[string]string{
		"name":        "TargetTable",
		"value":       "processed_logs",
		"type":        "string",
		"description": "Default destination table",
	}, "")

	_, _ = s.AddNode(sc.ID, "databases", "database", map[string]string{
		"name":              "local_sqlite",
		"driver":            "sqlite",
		"connection_string": "./pipeline_data.db",
	}, "")

	_, _ = s.AddNode(sc.ID, "flow", "sql", map[string]string{
		"id": "CreateTable",
		"db": "local_sqlite",
	}, "CREATE TABLE IF NOT EXISTS processed_logs (id INTEGER PRIMARY KEY, message TEXT, timestamp DATETIME DEFAULT CURRENT_TIMESTAMP);")

	return *sc, nil
}

func (s *Storage) GetNodes(scriptID int64, section string) ([]PipelineNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := "SELECT id, script_id, section, parent_node_id, node_type, sequence_order, attributes_json, content_text FROM pipeline_nodes WHERE script_id = ?"
	args := []any{scriptID}
	if section != "" {
		query += " AND section = ?"
		args = append(args, section)
	}
	query += " ORDER BY sequence_order ASC, id ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []PipelineNode
	for rows.Next() {
		var n PipelineNode
		var attrJSON string
		if err := rows.Scan(&n.ID, &n.ScriptID, &n.Section, &n.ParentNodeID, &n.NodeType, &n.SequenceOrder, &attrJSON, &n.ContentText); err != nil {
			return nil, err
		}
		n.Attributes = make(map[string]string)
		_ = json.Unmarshal([]byte(attrJSON), &n.Attributes)
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (s *Storage) GetNode(id int64) (*PipelineNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var n PipelineNode
	var attrJSON string
	err := s.db.QueryRow("SELECT id, script_id, section, parent_node_id, node_type, sequence_order, attributes_json, content_text FROM pipeline_nodes WHERE id = ?", id).
		Scan(&n.ID, &n.ScriptID, &n.Section, &n.ParentNodeID, &n.NodeType, &n.SequenceOrder, &attrJSON, &n.ContentText)
	if err != nil {
		return nil, err
	}
	n.Attributes = make(map[string]string)
	_ = json.Unmarshal([]byte(attrJSON), &n.Attributes)
	return &n, nil
}

func (s *Storage) AddNodeWithParent(scriptID int64, section, nodeType string, attributes map[string]string, content string, parentNodeID *int64) (*PipelineNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var maxSeq int
	if parentNodeID == nil {
		_ = s.db.QueryRow("SELECT COALESCE(MAX(sequence_order), 0) FROM pipeline_nodes WHERE script_id = ? AND section = ? AND parent_node_id IS NULL", scriptID, section).Scan(&maxSeq)
	} else {
		_ = s.db.QueryRow("SELECT COALESCE(MAX(sequence_order), 0) FROM pipeline_nodes WHERE script_id = ? AND section = ? AND parent_node_id = ?", scriptID, section, *parentNodeID).Scan(&maxSeq)
	}

	attrJSON, err := json.Marshal(attributes)
	if err != nil {
		attrJSON = []byte("{}")
	}

	var res sql.Result
	if parentNodeID == nil {
		res, err = s.db.Exec(`INSERT INTO pipeline_nodes (script_id, section, parent_node_id, node_type, sequence_order, attributes_json, content_text)
			VALUES (?, ?, NULL, ?, ?, ?, ?)`, scriptID, section, nodeType, maxSeq+1, string(attrJSON), content)
	} else {
		res, err = s.db.Exec(`INSERT INTO pipeline_nodes (script_id, section, parent_node_id, node_type, sequence_order, attributes_json, content_text)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, scriptID, section, *parentNodeID, nodeType, maxSeq+1, string(attrJSON), content)
	}
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &PipelineNode{
		ID:            id,
		ScriptID:      scriptID,
		Section:       section,
		ParentNodeID:  parentNodeID,
		NodeType:      nodeType,
		SequenceOrder: maxSeq + 1,
		Attributes:    attributes,
		ContentText:   content,
	}, nil
}

func (s *Storage) AddNode(scriptID int64, section, nodeType string, attributes map[string]string, content string) (*PipelineNode, error) {
	return s.AddNodeWithParent(scriptID, section, nodeType, attributes, content, nil)
}

// EnsureIfBranches ensures that an <if> node has both a <then> and <else> child container.
func (s *Storage) EnsureIfBranches(scriptID int64, section string, ifNodeID int64) (*PipelineNode, *PipelineNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var thenNode PipelineNode
	var thenAttrJSON string
	err := s.db.QueryRow("SELECT id, script_id, section, parent_node_id, node_type, sequence_order, attributes_json, content_text FROM pipeline_nodes WHERE script_id = ? AND parent_node_id = ? AND node_type = 'then'", scriptID, ifNodeID).
		Scan(&thenNode.ID, &thenNode.ScriptID, &thenNode.Section, &thenNode.ParentNodeID, &thenNode.NodeType, &thenNode.SequenceOrder, &thenAttrJSON, &thenNode.ContentText)
	if err == sql.ErrNoRows {
		res, err := s.db.Exec(`INSERT INTO pipeline_nodes (script_id, section, parent_node_id, node_type, sequence_order, attributes_json, content_text)
			VALUES (?, ?, ?, 'then', 1, '{}', '')`, scriptID, section, ifNodeID)
		if err != nil {
			return nil, nil, err
		}
		id, _ := res.LastInsertId()
		thenNode = PipelineNode{
			ID:            id,
			ScriptID:      scriptID,
			Section:       section,
			ParentNodeID:  &ifNodeID,
			NodeType:      "then",
			SequenceOrder: 1,
			Attributes:    map[string]string{},
		}
	} else if err != nil {
		return nil, nil, err
	} else {
		thenNode.Attributes = make(map[string]string)
		_ = json.Unmarshal([]byte(thenAttrJSON), &thenNode.Attributes)
	}

	var elseNode PipelineNode
	var elseAttrJSON string
	err = s.db.QueryRow("SELECT id, script_id, section, parent_node_id, node_type, sequence_order, attributes_json, content_text FROM pipeline_nodes WHERE script_id = ? AND parent_node_id = ? AND node_type = 'else'", scriptID, ifNodeID).
		Scan(&elseNode.ID, &elseNode.ScriptID, &elseNode.Section, &elseNode.ParentNodeID, &elseNode.NodeType, &elseNode.SequenceOrder, &elseAttrJSON, &elseNode.ContentText)
	if err == sql.ErrNoRows {
		res, err := s.db.Exec(`INSERT INTO pipeline_nodes (script_id, section, parent_node_id, node_type, sequence_order, attributes_json, content_text)
			VALUES (?, ?, ?, 'else', 2, '{}', '')`, scriptID, section, ifNodeID)
		if err != nil {
			return nil, nil, err
		}
		id, _ := res.LastInsertId()
		elseNode = PipelineNode{
			ID:            id,
			ScriptID:      scriptID,
			Section:       section,
			ParentNodeID:  &ifNodeID,
			NodeType:      "else",
			SequenceOrder: 2,
			Attributes:    map[string]string{},
		}
	} else if err != nil {
		return nil, nil, err
	} else {
		elseNode.Attributes = make(map[string]string)
		_ = json.Unmarshal([]byte(elseAttrJSON), &elseNode.Attributes)
	}

	return &thenNode, &elseNode, nil
}

// GetNodeTree returns root nodes with their nested child nodes recursively assembled in .Children.
func (s *Storage) GetNodeTree(scriptID int64, section string) ([]PipelineNode, error) {
	allNodes, err := s.GetNodes(scriptID, section)
	if err != nil {
		return nil, err
	}

	childMap := make(map[int64][]PipelineNode)
	var rootNodes []PipelineNode

	for _, n := range allNodes {
		if n.ParentNodeID == nil {
			rootNodes = append(rootNodes, n)
		} else {
			childMap[*n.ParentNodeID] = append(childMap[*n.ParentNodeID], n)
		}
	}

	var attachChildren func(node *PipelineNode)
	attachChildren = func(node *PipelineNode) {
		children, exists := childMap[node.ID]
		if !exists {
			node.Children = nil
			return
		}
		for i := range children {
			attachChildren(&children[i])
		}
		node.Children = children
	}

	for i := range rootNodes {
		attachChildren(&rootNodes[i])
	}

	return rootNodes, nil
}

func (s *Storage) UpdateNodeWithSection(id int64, section string, attributes map[string]string, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	attrJSON, err := json.Marshal(attributes)
	if err != nil {
		attrJSON = []byte("{}")
	}

	if section != "" {
		_, err = s.db.Exec("UPDATE pipeline_nodes SET section = ?, attributes_json = ?, content_text = ? WHERE id = ?", section, string(attrJSON), content, id)
		if err != nil {
			return err
		}
		// Also cascade section update to descendants
		var updateChildSections func(parentID int64) error
		updateChildSections = func(parentID int64) error {
			rows, err := s.db.Query("SELECT id FROM pipeline_nodes WHERE parent_node_id = ?", parentID)
			if err != nil {
				return err
			}
			var childIDs []int64
			for rows.Next() {
				var cid int64
				if err := rows.Scan(&cid); err == nil {
					childIDs = append(childIDs, cid)
				}
			}
			rows.Close()
			for _, cid := range childIDs {
				if _, err := s.db.Exec("UPDATE pipeline_nodes SET section = ? WHERE id = ?", section, cid); err != nil {
					return err
				}
				if err := updateChildSections(cid); err != nil {
					return err
				}
			}
			return nil
		}
		return updateChildSections(id)
	}

	_, err = s.db.Exec("UPDATE pipeline_nodes SET attributes_json = ?, content_text = ? WHERE id = ?", string(attrJSON), content, id)
	return err
}

func (s *Storage) UpdateNode(id int64, attributes map[string]string, content string) error {
	return s.UpdateNodeWithSection(id, "", attributes, content)
}

func (s *Storage) DeleteNode(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var deleteDescendants func(parentID int64) error
	deleteDescendants = func(parentID int64) error {
		rows, err := s.db.Query("SELECT id FROM pipeline_nodes WHERE parent_node_id = ?", parentID)
		if err != nil {
			return err
		}
		var childIDs []int64
		for rows.Next() {
			var cid int64
			if err := rows.Scan(&cid); err == nil {
				childIDs = append(childIDs, cid)
			}
		}
		rows.Close()

		for _, cid := range childIDs {
			if err := deleteDescendants(cid); err != nil {
				return err
			}
		}

		_, err = s.db.Exec("DELETE FROM pipeline_nodes WHERE id = ?", parentID)
		return err
	}

	return deleteDescendants(id)
}

func (s *Storage) MoveNode(id int64, direction string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var scriptID int64
	var section string
	var parentNodeID *int64
	var currentSeq int
	err := s.db.QueryRow("SELECT script_id, section, parent_node_id, sequence_order FROM pipeline_nodes WHERE id = ?", id).
		Scan(&scriptID, &section, &parentNodeID, &currentSeq)
	if err != nil {
		return err
	}

	var neighborID int64
	var neighborSeq int

	if parentNodeID == nil {
		if direction == "up" {
			err = s.db.QueryRow(`SELECT id, sequence_order FROM pipeline_nodes 
				WHERE script_id = ? AND section = ? AND parent_node_id IS NULL AND sequence_order < ? 
				ORDER BY sequence_order DESC LIMIT 1`, scriptID, section, currentSeq).Scan(&neighborID, &neighborSeq)
		} else {
			err = s.db.QueryRow(`SELECT id, sequence_order FROM pipeline_nodes 
				WHERE script_id = ? AND section = ? AND parent_node_id IS NULL AND sequence_order > ? 
				ORDER BY sequence_order ASC LIMIT 1`, scriptID, section, currentSeq).Scan(&neighborID, &neighborSeq)
		}
	} else {
		if direction == "up" {
			err = s.db.QueryRow(`SELECT id, sequence_order FROM pipeline_nodes 
				WHERE script_id = ? AND section = ? AND parent_node_id = ? AND sequence_order < ? 
				ORDER BY sequence_order DESC LIMIT 1`, scriptID, section, *parentNodeID, currentSeq).Scan(&neighborID, &neighborSeq)
		} else {
			err = s.db.QueryRow(`SELECT id, sequence_order FROM pipeline_nodes 
				WHERE script_id = ? AND section = ? AND parent_node_id = ? AND sequence_order > ? 
				ORDER BY sequence_order ASC LIMIT 1`, scriptID, section, *parentNodeID, currentSeq).Scan(&neighborID, &neighborSeq)
		}
	}

	if err != nil {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE pipeline_nodes SET sequence_order = ? WHERE id = ?", neighborSeq, id); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE pipeline_nodes SET sequence_order = ? WHERE id = ?", currentSeq, neighborID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Storage) ReorderNodes(scriptID int64, section string, nodeIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for idx, id := range nodeIDs {
		if _, err := tx.Exec("UPDATE pipeline_nodes SET sequence_order = ? WHERE id = ? AND script_id = ? AND section = ?", idx+1, id, scriptID, section); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Storage) GetOrCreateDefaultConfig() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var content string
	err := s.db.QueryRow("SELECT content FROM config_files WHERE name = 'CONFIG.xml'").Scan(&content)
	if err == nil && content != "" {
		return content
	}

	defaultConfig := `<?xml version="1.0" encoding="UTF-8"?>
<config>
    <variables>
        <variable name="Environment" value="development" />
        <variable name="LogLevel" value="DEBUG" />
    </variables>
    <databases>
        <database name="local_sqlite" driver="sqlite" connection_string="./pipeline_dev.db" />
    </databases>
</config>`

	_, _ = s.db.Exec("INSERT OR REPLACE INTO config_files (name, content, updated_at) VALUES ('CONFIG.xml', ?, CURRENT_TIMESTAMP)", defaultConfig)
	return defaultConfig
}

func (s *Storage) SaveConfigFile(name, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("INSERT OR REPLACE INTO config_files (name, content, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)", name, content)
	return err
}

func (s *Storage) GetOrCreateDefaultOptions() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var content string
	err := s.db.QueryRow("SELECT content FROM options_files WHERE name = 'options.xml'").Scan(&content)
	if err == nil && content != "" {
		return content
	}

	defaultOptions := `<?xml version="1.0" encoding="UTF-8"?>
<options xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:noNamespaceSchemaLocation="https://raw.githubusercontent.com/etl-madness/go-flow/main/xsd/options.xsd">
    <file>scripts.xml</file>
    <debug>true</debug>
    <preflight>true</preflight>
    <validate>true</validate>
</options>`

	_, _ = s.db.Exec("INSERT OR REPLACE INTO options_files (name, content, updated_at) VALUES ('options.xml', ?, CURRENT_TIMESTAMP)", defaultOptions)
	return defaultOptions
}

func (s *Storage) SaveOptionsFile(name, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("INSERT OR REPLACE INTO options_files (name, content, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)", name, content)
	return err
}

// PurgeDatabase drops/clears all scripts, nodes, configs, and options drafts,
// resets sequence counters, and re-initializes fresh default files and scripts.
func (s *Storage) PurgeDatabase() (*Script, error) {
	s.mu.Lock()
	tx, err := s.db.Begin()
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("failed to start purge transaction: %w", err)
	}
	defer tx.Rollback()

	tables := []string{"pipeline_nodes", "scripts", "config_files", "options_files"}
	for _, tbl := range tables {
		if _, err := tx.Exec("DELETE FROM " + tbl); err != nil {
			s.mu.Unlock()
			return nil, fmt.Errorf("failed to clear table %s: %w", tbl, err)
		}
	}
	_, _ = tx.Exec("DELETE FROM sqlite_sequence WHERE name IN ('scripts', 'pipeline_nodes', 'config_files', 'options_files')")

	if err := tx.Commit(); err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("failed to commit purge: %w", err)
	}
	s.mu.Unlock()

	s.GetOrCreateDefaultConfig()
	s.GetOrCreateDefaultOptions()

	sc, err := s.GetOrCreateDefaultScript()
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

// DeleteScript deletes a script and all of its associated nodes.
// If no scripts remain after deletion, a default script is automatically re-seeded.
func (s *Storage) DeleteScript(id int64) error {
	s.mu.Lock()
	tx, err := s.db.Begin()
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to start delete transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM pipeline_nodes WHERE script_id = ?", id); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to delete script nodes: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM scripts WHERE id = ?", id); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to delete script: %w", err)
	}

	var count int
	_ = tx.QueryRow("SELECT COUNT(*) FROM scripts").Scan(&count)

	if err := tx.Commit(); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to commit delete: %w", err)
	}
	s.mu.Unlock()

	if count == 0 {
		_, _ = s.GetOrCreateDefaultScript()
	}

	return nil
}

// CopyScript duplicates an existing pipeline script and all its child nodes.
func (s *Storage) CopyScript(sourceID int64, newName string) (*Script, error) {
	sourceScript, err := s.GetScript(sourceID)
	if err != nil {
		return nil, fmt.Errorf("source script not found: %w", err)
	}

	baseName := strings.TrimSpace(newName)
	if baseName == "" {
		baseName = sourceScript.Name + " (Copy)"
	}

	// Ensure unique name
	candidate := baseName
	copyIndex := 2
	for {
		existingScripts, err := s.ListScripts()
		if err != nil {
			return nil, err
		}
		exists := false
		for _, sc := range existingScripts {
			if strings.EqualFold(sc.Name, candidate) {
				exists = true
				break
			}
		}
		if !exists {
			break
		}
		candidate = fmt.Sprintf("%s (%d)", baseName, copyIndex)
		copyIndex++
	}

	newScript, err := s.CreateScript(candidate, sourceScript.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloned script record: %w", err)
	}

	nodes, err := s.GetNodes(sourceID, "")
	if err != nil {
		return newScript, nil
	}

	// Map old node ID -> new node ID so parent_node_id references are preserved
	idMap := make(map[int64]int64)
	remaining := make([]PipelineNode, len(nodes))
	copy(remaining, nodes)

	for len(remaining) > 0 {
		var nextRound []PipelineNode
		progress := false

		for _, node := range remaining {
			if node.ParentNodeID == nil {
				created, err := s.AddNodeWithParent(newScript.ID, node.Section, node.NodeType, node.Attributes, node.ContentText, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to duplicate root node %d: %w", node.ID, err)
				}
				idMap[node.ID] = created.ID
				progress = true
			} else if newParentID, ok := idMap[*node.ParentNodeID]; ok {
				created, err := s.AddNodeWithParent(newScript.ID, node.Section, node.NodeType, node.Attributes, node.ContentText, &newParentID)
				if err != nil {
					return nil, fmt.Errorf("failed to duplicate child node %d: %w", node.ID, err)
				}
				idMap[node.ID] = created.ID
				progress = true
			} else {
				nextRound = append(nextRound, node)
			}
		}

		if !progress && len(nextRound) > 0 {
			for _, orphan := range nextRound {
				created, _ := s.AddNodeWithParent(newScript.ID, orphan.Section, orphan.NodeType, orphan.Attributes, orphan.ContentText, nil)
				if created != nil {
					idMap[orphan.ID] = created.ID
				}
			}
			break
		}
		remaining = nextRound
	}

	return newScript, nil
}