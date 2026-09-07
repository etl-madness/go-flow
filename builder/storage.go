package builder

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

func (s *Storage) AddNode(scriptID int64, section, nodeType string, attributes map[string]string, content string) (*PipelineNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var maxSeq int
	_ = s.db.QueryRow("SELECT COALESCE(MAX(sequence_order), 0) FROM pipeline_nodes WHERE script_id = ? AND section = ?", scriptID, section).Scan(&maxSeq)

	attrJSON, err := json.Marshal(attributes)
	if err != nil {
		attrJSON = []byte("{}")
	}

	res, err := s.db.Exec(`INSERT INTO pipeline_nodes (script_id, section, node_type, sequence_order, attributes_json, content_text)
		VALUES (?, ?, ?, ?, ?, ?)`, scriptID, section, nodeType, maxSeq+1, string(attrJSON), content)
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
		NodeType:      nodeType,
		SequenceOrder: maxSeq + 1,
		Attributes:    attributes,
		ContentText:   content,
	}, nil
}

func (s *Storage) UpdateNode(id int64, attributes map[string]string, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	attrJSON, err := json.Marshal(attributes)
	if err != nil {
		attrJSON = []byte("{}")
	}

	_, err = s.db.Exec("UPDATE pipeline_nodes SET attributes_json = ?, content_text = ? WHERE id = ?", string(attrJSON), content, id)
	return err
}

func (s *Storage) DeleteNode(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM pipeline_nodes WHERE id = ?", id)
	return err
}

func (s *Storage) MoveNode(id int64, direction string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var scriptID int64
	var section string
	var currentSeq int
	err := s.db.QueryRow("SELECT script_id, section, sequence_order FROM pipeline_nodes WHERE id = ?", id).
		Scan(&scriptID, &section, &currentSeq)
	if err != nil {
		return err
	}

	var neighborID int64
	var neighborSeq int

	if direction == "up" {
		err = s.db.QueryRow(`SELECT id, sequence_order FROM pipeline_nodes 
			WHERE script_id = ? AND section = ? AND sequence_order < ? 
			ORDER BY sequence_order DESC LIMIT 1`, scriptID, section, currentSeq).Scan(&neighborID, &neighborSeq)
	} else {
		err = s.db.QueryRow(`SELECT id, sequence_order FROM pipeline_nodes 
			WHERE script_id = ? AND section = ? AND sequence_order > ? 
			ORDER BY sequence_order ASC LIMIT 1`, scriptID, section, currentSeq).Scan(&neighborID, &neighborSeq)
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