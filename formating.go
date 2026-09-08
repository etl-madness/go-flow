package main

import (
	"bufio"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/etl-madness/flow"
)

type PrintableResult struct {
	ScriptID      string      `json:"script_id"`
	ReturnCode    any         `json:"return_code"`
	Duration      any         `json:"duration"`
	ResultsString interface{} `json:"results_string"`
}

type TextSink struct {
	Writer io.Writer
}

type DatabaseSink struct {
	DB          *sql.DB
	Driver      string
	insertQuery string
	mu          sync.RWMutex
	runID       string
	status      string
	errorClass  string
	errorMsg    string
}

// MultiSink fans out execution events to multiple sinks (e.g., Database + Stdout)
type MultiSink struct {
	Sinks []flow.EventSink
}

func (m *MultiSink) Emit(ctx context.Context, event flow.ExecutionEvent) error {
	var firstErr error
	for _, sink := range m.Sinks {
		if err := sink.Emit(ctx, event); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

var lineReturnsReplacer = strings.NewReplacer(
	"\r\n", "\n",
	`\r\n`, "\n",
	"\r", "",
)

var jsonLineBreakReplacer = strings.NewReplacer(
	`\\`, `\\`,
	`\r\n`, "",
	`\n`, "",
	`\r`, "",
)

func formatLineReturns(s string) string {
	return lineReturnsReplacer.Replace(s)
}

// NewDatabaseSink constructs a DatabaseSink and pre-builds the dialect-specific SQL query
func NewDatabaseSink(db *sql.DB, driver string) *DatabaseSink {
	cols := []string{
		"run_id",
		"execution_id",
		"sequence_num",
		"occurred_at",
		"event_type",
		"node_kind",
		"node_id",
		"status",
		"error_message",
		"rows_read",
		"rows_written",
		"rows_affected",
	}

	return &DatabaseSink{
		DB:          db,
		Driver:      driver,
		insertQuery: buildInsertQuery(driver, pipeline_events, cols),
	}
}

// Emit satisfies the flow.EventSink interface and persists execution events to the database
func (s *DatabaseSink) Emit(ctx context.Context, event flow.ExecutionEvent) error {
	s.mu.Lock()
	if event.RunID != "" {
		s.runID = event.RunID
	}
	if event.Type == flow.EventRunFinished {
		s.status = string(event.Status)
		s.errorClass = string(event.ErrorClass)
		s.errorMsg = event.ErrorMessage
	}
	s.mu.Unlock()

	query := s.insertQuery
	if query == "" {
		cols := []string{
			"run_id", "execution_id", "sequence_num", "occurred_at",
			"event_type", "node_kind", "node_id", "status",
			"error_message", "rows_read", "rows_written", "rows_affected",
		}
		query = buildInsertQuery(s.Driver, pipeline_events, cols)
	}

	_, err := s.DB.ExecContext(ctx, query,
		event.RunID,
		event.ExecutionID,
		event.Sequence,
		event.OccurredAt.UTC(),
		event.Type,
		event.NodeKind,
		event.NodeID,
		event.Status,
		event.ErrorMessage,
		event.RowCounts.Read,
		event.RowCounts.Written,
		event.RowCounts.Affected,
	)
	return err
}

// RunID returns the run ID captured from execution events
func (s *DatabaseSink) RunID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runID
}

// RunDetails returns run ID, status, error class, and error message captured from execution events
func (s *DatabaseSink) RunDetails() (runID, status, errorClass, errorMsg string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runID, s.status, s.errorClass, s.errorMsg
}

// generateRunID creates a cryptographically random 32-character hex ID (matching flow's internal run IDs)
func generateRunID() string {
	bytes := make([]byte, 16)
	if _, err := cryptorand.Read(bytes); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func (s TextSink) Emit(_ context.Context, event flow.ExecutionEvent) error {
	_, err := fmt.Fprintf(
		s.Writer,
		"%s,%s,%s,%d,%s,%s,%s,%s,%s,%d,%d,%d\n",
		event.OccurredAt.UTC().Format(time.RFC3339),
		event.RunID,
		event.ExecutionID,
		event.Sequence,
		event.Type,
		event.NodeKind,
		event.NodeID,
		event.Status,
		event.ErrorMessage,
		event.RowCounts.Read,
		event.RowCounts.Written,
		event.RowCounts.Affected,
	)
	return err
}

func outputRawJSON(res *[]flow.ScriptResult) {
	jsonBytes, err := json.Marshal(res)
	if err != nil {
		fmt.Printf("[{\"script_id\": \"system\", \"return_code\": 1, \"results_string\": \"JSON encoding error: %v\"}]\n", err)
		return
	}
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	w.WriteString(formatLineReturns(string(jsonBytes)))
	w.WriteByte('\n')
}

func outputJSON(res *[]flow.ScriptResult) {
	var printable []PrintableResult

	for _, r := range *res {
		cleanStr := formatLineReturns(r.ResultsString)
		var val interface{} = cleanStr

		cleanBytes := []byte(cleanStr)
		if json.Valid(cleanBytes) {
			val = json.RawMessage(cleanBytes)
		}

		printable = append(printable, PrintableResult{
			ScriptID:      r.ScriptID,
			ReturnCode:    r.ReturnCode,
			ResultsString: val,
			Duration:      r.Duration,
		})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(printable); err != nil {
		fmt.Printf("[{\"script_id\": \"system\", \"return_code\": 1, \"results_string\": \"JSON encoding error: %v\"}]\n", err)
		return
	}

	outputStr := jsonLineBreakReplacer.Replace(buf.String())

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	w.WriteString(outputStr)
}

func outputText(res *[]flow.ScriptResult) {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for _, r := range *res {
		fmt.Fprintf(w, "ScriptID: %s\nReturnCode: %d\nDuration: %v\nResultsString: %s\n\n",
			r.ScriptID, r.ReturnCode, r.Duration, r.ResultsString)
	}
}

func outputMarkdownTable(res *[]flow.ScriptResult) {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	fmt.Fprintln(w, "| Script ID | Return Code | Duration | Results |")
	fmt.Fprintln(w, "| :--- | :--- | :--- | :--- |")
	for _, r := range *res {
		cleanResults := strings.ReplaceAll(r.ResultsString, "\n", "<br>")
		cleanResults = strings.ReplaceAll(cleanResults, "|", "\\|")
		fmt.Fprintf(w, "| %s | %d | %v | %s |\n", r.ScriptID, r.ReturnCode, r.Duration, cleanResults)
	}
}

func outputCSV(res *[]flow.ScriptResult) {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for _, r := range *res {
		fmt.Fprintf(w, " %s,  %d,  %v,  %s\n", r.ScriptID, r.ReturnCode, r.Duration, r.ResultsString)
	}
}

func outputStreamSummary(run flow.RunResult, file *string, config *string) {
	var configStr string
	if config != nil {
		configStr = *config
	}
	fmt.Println("\n\nutc_runtime,file,config,status,started,finished,task_count")
	log.Printf(
		"%s,%s,%s,%s,%s,%s,%d",
		time.Now().UTC().Format(time.RFC3339),
		*file,
		configStr,
		run.Status,
		run.StartedAt.UTC().Format(time.RFC3339),
		run.FinishedAt.UTC().Format(time.RFC3339),
		len(run.Nodes),
	)
}

type RunSummaryRecord struct {
	RunID        string
	FilePath     string
	ConfigPath   string
	Status       string
	StartedAt    time.Time
	FinishedAt   time.Time
	Duration     time.Duration
	TaskCount    int
	ErrorClass   string
	ErrorMessage string
}

// LogRunSummaryToDB executes dynamic insert into pipeline_runs table
func LogRunSummaryToDB(ctx context.Context, db *sql.DB, driverType string, rec RunSummaryRecord) error {
	if rec.RunID == "" {
		rec.RunID = generateRunID()
	}

	cols := []string{
		"run_id", "file_path", "config_path", "status", "started_at",
		"finished_at", "duration_ms", "task_count", "error_class", "error_message",
	}

	query := buildInsertQuery(driverType, pipeline_runs, cols)

	_, err := db.ExecContext(ctx, query,
		rec.RunID,
		rec.FilePath,
		rec.ConfigPath,
		rec.Status,
		rec.StartedAt.UTC(),
		rec.FinishedAt.UTC(),
		rec.Duration.Milliseconds(),
		rec.TaskCount,
		rec.ErrorClass,
		rec.ErrorMessage,
	)
	return err
}

// detectDriverFromDSN infers SQL driver name from connection string if XML driver field is missing
func detectDriverFromDSN(dsn string) string {
	lowerDSN := strings.ToLower(dsn)
	switch {
	case strings.HasPrefix(lowerDSN, "postgres://"), strings.Contains(lowerDSN, "dbname="):
		return "postgres"
	case strings.HasPrefix(lowerDSN, "sqlserver://"), strings.Contains(lowerDSN, "server="):
		return "sqlserver"
	case strings.Contains(lowerDSN, "@tcp("), strings.HasPrefix(lowerDSN, "mysql://"):
		return "mysql"
	case strings.HasSuffix(lowerDSN, ".db"), strings.HasSuffix(lowerDSN, ".sqlite"), strings.HasSuffix(lowerDSN, ".sqlite3"):
		return "sqlite3"
	case strings.HasPrefix(lowerDSN, "oracle://"):
		return "oracle"
	default:
		return "postgres"
	}
}

func detectDriverType(db *sql.DB) string {
	if db == nil || db.Driver() == nil {
		return "unknown"
	}
	driverPkg := strings.ToLower(fmt.Sprintf("%T", db.Driver()))

	switch {
	case strings.Contains(driverPkg, "pq"), strings.Contains(driverPkg, "pgx"):
		return "postgres"
	case strings.Contains(driverPkg, "mysql"):
		return "mysql"
	case strings.Contains(driverPkg, "sqlite"):
		return "sqlite"
	case strings.Contains(driverPkg, "mssql"):
		return "sqlserver"
	case strings.Contains(driverPkg, "godror"), strings.Contains(driverPkg, "oracle"):
		return "oracle"
	default:
		return driverPkg
	}
}

func buildInsertQuery(driverType, table string, columns []string) string {
	var placeholders []string
	if driverType == "sqlserver" && !strings.Contains(table, ".") {
		table = "dbo." + table
	}
	for i := 1; i <= len(columns); i++ {
		switch driverType {
		case "postgres":
			placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		case "sqlserver":
			placeholders = append(placeholders, fmt.Sprintf("@p%d", i))
		case "oracle":
			placeholders = append(placeholders, fmt.Sprintf(":%d", i))
		case "mysql", "sqlite":
			fallthrough
		default:
			placeholders = append(placeholders, "?")
		}
	}

	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)
}
