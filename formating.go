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
	"net/url"
	"os"
	"regexp"
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
	DB             *sql.DB
	Driver         string
	insertQuery    string
	configuredCols []string
	colsDetected   bool
	spid           int64
	dbUser         string
	mu             sync.RWMutex
	runID          string
	status         string
	errorClass     string
	errorMsg       string
	debug          bool
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

var defaultEventCols = []string{
	"run_id",
	"execution_id",
	"sequence_num",
	"occurred_at",
	"event_type",
	"node_kind",
	"node_id",
	"status",
	"user_name",
	"hostname",
	"options_path",
	"error_message",
	"rows_read",
	"rows_written",
	"rows_affected",
}

var knownEventCols = map[string]bool{
	"run_id":             true,
	"execution_id":       true,
	"sequence_num":       true,
	"occurred_at":        true,
	"event_type":         true,
	"node_kind":          true,
	"node_id":            true,
	"status":             true,
	"user_name":          true,
	"os_user_name":       true,
	"db_user_name":       true,
	"database_user_name": true,
	"spid":               true,
	"hostname":           true,
	"options_path":       true,
	"error_message":      true,
	"rows_read":          true,
	"rows_written":       true,
	"rows_affected":      true,
}

func filterAvailableEventCols(cols []string) []string {
	var result []string
	for _, c := range cols {
		lower := strings.ToLower(strings.TrimSpace(c))
		if knownEventCols[lower] {
			result = append(result, lower)
		}
	}
	if len(result) == 0 {
		return defaultEventCols
	}
	return result
}

// FetchDatabaseSessionIdentity queries the database for the current connection SPID (or equivalent)
// and database user name based on the driver dialect.
func FetchDatabaseSessionIdentity(ctx context.Context, querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, driver string) (spid int64, dbUser string, err error) {
	if querier == nil {
		return 0, "", fmt.Errorf("nil database querier")
	}

	normDriver := strings.ToLower(driver)
	switch {
	case strings.Contains(normDriver, "sqlserver") || strings.Contains(normDriver, "mssql"):
		var sVal sql.NullInt64
		var uVal sql.NullString
		err = querier.QueryRowContext(ctx, "SELECT @@SPID, SYSTEM_USER").Scan(&sVal, &uVal)
		if err == nil {
			return sVal.Int64, uVal.String, nil
		}
	case strings.Contains(normDriver, "postgres") || strings.Contains(normDriver, "pq") || strings.Contains(normDriver, "pgx"):
		var sVal sql.NullInt64
		var uVal sql.NullString
		err = querier.QueryRowContext(ctx, "SELECT pg_backend_pid(), CURRENT_USER").Scan(&sVal, &uVal)
		if err == nil {
			return sVal.Int64, uVal.String, nil
		}
	case strings.Contains(normDriver, "mysql"):
		var sVal sql.NullInt64
		var uVal sql.NullString
		err = querier.QueryRowContext(ctx, "SELECT CONNECTION_ID(), CURRENT_USER()").Scan(&sVal, &uVal)
		if err == nil {
			return sVal.Int64, uVal.String, nil
		}
	case strings.Contains(normDriver, "oracle") || strings.Contains(normDriver, "ora"):
		var sVal sql.NullInt64
		var uVal sql.NullString
		err = querier.QueryRowContext(ctx, "SELECT TO_NUMBER(SYS_CONTEXT('USERENV', 'SID')), SYS_CONTEXT('USERENV', 'SESSION_USER') FROM DUAL").Scan(&sVal, &uVal)
		if err == nil {
			return sVal.Int64, uVal.String, nil
		}
	case strings.Contains(normDriver, "sqlite"):
		userName, _ := runtimeIdentity()
		if userName == "" {
			userName = "sqlite"
		}
		return int64(os.Getpid()), userName, nil
	}

	userName, _ := runtimeIdentity()
	return int64(os.Getpid()), userName, nil
}

// NewDatabaseSink constructs a DatabaseSink and pre-builds the dialect-specific SQL query
func NewDatabaseSink(db *sql.DB, driver string) *DatabaseSink {
	sink := &DatabaseSink{
		DB:     db,
		Driver: driver,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if db != nil {
		spid, dbUser, err := FetchDatabaseSessionIdentity(ctx, db, driver)
		if err == nil {
			sink.spid = spid
			sink.dbUser = dbUser
		}
		sink.detectColumns(ctx)
	} else {
		sink.configuredCols = defaultEventCols
		sink.insertQuery = buildInsertQuery(driver, pipeline_events, defaultEventCols)
	}

	return sink
}

func (s *DatabaseSink) detectColumns(ctx context.Context) {
	if s.DB == nil {
		s.configuredCols = defaultEventCols
		s.insertQuery = buildInsertQuery(s.Driver, pipeline_events, defaultEventCols)
		return
	}

	table := pipeline_events
	if s.Driver == "sqlserver" && !strings.Contains(table, ".") {
		table = "dbo." + table
	}

	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s WHERE 1=0", table))
	if err == nil {
		colNames, colErr := rows.Columns()
		rows.Close()
		if colErr == nil && len(colNames) > 0 {
			s.configuredCols = filterAvailableEventCols(colNames)
			s.insertQuery = buildInsertQuery(s.Driver, pipeline_events, s.configuredCols)
			s.colsDetected = true
			return
		}
	}

	s.configuredCols = defaultEventCols
	s.insertQuery = buildInsertQuery(s.Driver, pipeline_events, defaultEventCols)
}

// SPID returns the session ID or process ID captured for the database connection
func (s *DatabaseSink) SPID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.spid
}

// DBUser returns the database user name captured for the database connection
func (s *DatabaseSink) DBUser() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dbUser
}

// SessionIdentity returns both SPID and DB User
func (s *DatabaseSink) SessionIdentity() (int64, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.spid, s.dbUser
}

// SetSessionIdentity explicitly overrides the session identity for testing or connection pooling
func (s *DatabaseSink) SetSessionIdentity(spid int64, dbUser string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spid = spid
	s.dbUser = dbUser
}

// Emit satisfies the flow.EventSink interface and persists execution events to the database
func (s *DatabaseSink) Emit(ctx context.Context, event flow.ExecutionEvent) error {
	event.OptionsPath = maskSensitiveSourceForDB(event.OptionsPath, s.debug)

	s.mu.Lock()
	if event.RunID != "" {
		s.runID = event.RunID
	}
	if event.Type == flow.EventRunFinished {
		s.status = string(event.Status)
		s.errorClass = string(event.ErrorClass)
		s.errorMsg = event.ErrorMessage
	}
	if !s.colsDetected && s.DB != nil {
		s.detectColumns(ctx)
	}
	cols := s.configuredCols
	query := s.insertQuery
	spid := s.spid
	dbUser := s.dbUser
	s.mu.Unlock()

	if query == "" || len(cols) == 0 {
		cols = defaultEventCols
		query = buildInsertQuery(s.Driver, pipeline_events, cols)
	}

	osUser := event.UserName
	if osUser == "" {
		osUser, _ = runtimeIdentity()
	}

	args := make([]any, 0, len(cols))
	for _, col := range cols {
		switch strings.ToLower(col) {
		case "run_id":
			args = append(args, event.RunID)
		case "execution_id":
			args = append(args, event.ExecutionID)
		case "sequence_num":
			args = append(args, event.Sequence)
		case "occurred_at":
			args = append(args, event.OccurredAt.UTC())
		case "event_type":
			args = append(args, string(event.Type))
		case "node_kind":
			args = append(args, event.NodeKind)
		case "node_id":
			args = append(args, event.NodeID)
		case "status":
			args = append(args, string(event.Status))
		case "user_name":
			args = append(args, osUser)
		case "os_user_name":
			args = append(args, osUser)
		case "db_user_name", "database_user_name":
			args = append(args, dbUser)
		case "spid":
			args = append(args, spid)
		case "hostname":
			args = append(args, event.Hostname)
		case "options_path":
			args = append(args, event.OptionsPath)
		case "error_message":
			args = append(args, event.ErrorMessage)
		case "rows_read":
			args = append(args, event.RowCounts.Read)
		case "rows_written":
			args = append(args, event.RowCounts.Written)
		case "rows_affected":
			args = append(args, event.RowCounts.Affected)
		default:
			args = append(args, nil)
		}
	}

	_, err := s.DB.ExecContext(ctx, query, args...)
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
		"%s,%s,%s,%d,%s,%s,%s,%s,%s,%s,%s,%s,%d,%d,%d\n",
		event.OccurredAt.UTC().Format(time.RFC3339),
		event.RunID,
		event.ExecutionID,
		event.Sequence,
		event.Type,
		event.NodeKind,
		event.NodeID,
		event.Status,
		event.UserName,
		event.Hostname,
		event.OptionsPath,
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
	fmt.Println("\n\nutc_runtime,file,config,status,started,finished,task_count,user_name,hostname,options_path")
	log.Printf(
		"%s,%s,%s,%s,%s,%s,%d,%s,%s,%s",
		time.Now().UTC().Format(time.RFC3339),
		*file,
		configStr,
		run.Status,
		run.StartedAt.UTC().Format(time.RFC3339),
		run.FinishedAt.UTC().Format(time.RFC3339),
		len(run.Nodes),
		run.UserName,
		run.Hostname,
		run.OptionsPath,
	)
}

var kvPasswordRegex = regexp.MustCompile(`(?i)\b(password|pwd)\s*=\s*[^;]+`)

func maskSensitiveSourceForDB(source string, debug bool) string {
	source = strings.TrimSpace(source)
	if source == "" || debug {
		return source
	}

	prefix := ""
	raw := source
	if strings.HasPrefix(source, "sql://") {
		prefix = "sql://"
		raw = strings.TrimPrefix(source, "sql://")
	} else if strings.HasPrefix(source, "db://") {
		prefix = "db://"
		raw = strings.TrimPrefix(source, "db://")
	}

	if prefix != "" {
		parts := strings.SplitN(raw, "#", 2)
		connSpec := parts[0]
		fragment := ""
		if len(parts) == 2 {
			fragment = "#" + parts[1]
		}

		if strings.Contains(connSpec, "@") {
			driverAndDSN := strings.SplitN(connSpec, "@", 2)
			driver := driverAndDSN[0]
			dsn := driverAndDSN[1]
			maskedDSN := maskURLUserInfo(dsn)
			maskedDSN = kvPasswordRegex.ReplaceAllString(maskedDSN, "${1}=******")
			return prefix + driver + "@" + maskedDSN + fragment
		}
	}

	masked := maskURLUserInfo(source)
	return kvPasswordRegex.ReplaceAllString(masked, "${1}=******")
}

func maskURLUserInfo(source string) string {
	parsed, err := url.Parse(source)
	if err != nil || parsed.User == nil {
		return source
	}

	username := parsed.User.Username()
	if username == "" {
		return source
	}

	maskedUserInfo := "******"
	if _, hasPassword := parsed.User.Password(); hasPassword {
		maskedUserInfo = username + ":******"
	}

	var b strings.Builder
	b.WriteString(parsed.Scheme)
	b.WriteString("://")
	b.WriteString(maskedUserInfo)
	b.WriteString("@")
	b.WriteString(parsed.Host)
	if p := parsed.EscapedPath(); p != "" {
		b.WriteString(p)
	}
	if parsed.RawQuery != "" {
		b.WriteString("?")
		b.WriteString(parsed.RawQuery)
	}
	if parsed.Fragment != "" {
		b.WriteString("#")
		b.WriteString(parsed.Fragment)
	}
	return b.String()
}

type RunSummaryRecord struct {
	RunID            string
	FilePath         string
	ConfigPath       string
	Status           string
	StartedAt        time.Time
	FinishedAt       time.Time
	Duration         time.Duration
	TaskCount        int
	UserName         string
	OSUserName       string
	DatabaseUserName string
	DBUserName       string
	SPID             int64
	Hostname         string
	OptionsPath      string
	ErrorClass       string
	ErrorMessage     string
}

var defaultRunCols = []string{
	"run_id", "file_path", "config_path", "status", "started_at",
	"finished_at", "duration_ms", "task_count", "user_name", "hostname",
	"options_path", "error_class", "error_message",
}

var knownRunCols = map[string]bool{
	"run_id":             true,
	"file_path":          true,
	"config_path":        true,
	"status":             true,
	"started_at":         true,
	"finished_at":        true,
	"duration_ms":        true,
	"task_count":         true,
	"user_name":          true,
	"os_user_name":       true,
	"db_user_name":       true,
	"database_user_name": true,
	"spid":               true,
	"hostname":           true,
	"options_path":       true,
	"error_class":        true,
	"error_message":      true,
}

func filterAvailableRunCols(cols []string) []string {
	var result []string
	for _, c := range cols {
		lower := strings.ToLower(strings.TrimSpace(c))
		if knownRunCols[lower] {
			result = append(result, lower)
		}
	}
	if len(result) == 0 {
		return defaultRunCols
	}
	return result
}

// LogRunSummaryToDB executes dynamic insert into pipeline_runs table
func LogRunSummaryToDB(ctx context.Context, db *sql.DB, driverType string, rec RunSummaryRecord, debug bool) error {
	if rec.RunID == "" {
		rec.RunID = generateRunID()
	}

	rec.FilePath = maskSensitiveSourceForDB(rec.FilePath, debug)
	rec.ConfigPath = maskSensitiveSourceForDB(rec.ConfigPath, debug)
	rec.OptionsPath = maskSensitiveSourceForDB(rec.OptionsPath, debug)

	table := pipeline_runs
	if driverType == "sqlserver" && !strings.Contains(table, ".") {
		table = "dbo." + table
	}

	cols := defaultRunCols
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s WHERE 1=0", table))
	if err == nil {
		colNames, colErr := rows.Columns()
		rows.Close()
		if colErr == nil && len(colNames) > 0 {
			cols = filterAvailableRunCols(colNames)
		}
	}

	query := buildInsertQuery(driverType, pipeline_runs, cols)

	osUser := rec.OSUserName
	if osUser == "" {
		osUser = rec.UserName
	}
	dbUser := rec.DBUserName
	if dbUser == "" {
		dbUser = rec.DatabaseUserName
	}

	args := make([]any, 0, len(cols))
	for _, c := range cols {
		switch strings.ToLower(c) {
		case "run_id":
			args = append(args, rec.RunID)
		case "file_path":
			args = append(args, rec.FilePath)
		case "config_path":
			args = append(args, rec.ConfigPath)
		case "status":
			args = append(args, rec.Status)
		case "started_at":
			args = append(args, rec.StartedAt.UTC())
		case "finished_at":
			args = append(args, rec.FinishedAt.UTC())
		case "duration_ms":
			args = append(args, rec.Duration.Milliseconds())
		case "task_count":
			args = append(args, rec.TaskCount)
		case "user_name":
			args = append(args, rec.UserName)
		case "os_user_name":
			args = append(args, osUser)
		case "db_user_name", "database_user_name":
			args = append(args, dbUser)
		case "spid":
			args = append(args, rec.SPID)
		case "hostname":
			args = append(args, rec.Hostname)
		case "options_path":
			args = append(args, rec.OptionsPath)
		case "error_class":
			args = append(args, rec.ErrorClass)
		case "error_message":
			args = append(args, rec.ErrorMessage)
		default:
			args = append(args, nil)
		}
	}

	_, err = db.ExecContext(ctx, query, args...)
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
