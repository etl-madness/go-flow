package builder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	TablePipelineEvents = "pipeline_events"
	TablePipelineRuns   = "pipeline_runs"
)

var (
	activePipelineRunsTable   = TablePipelineRuns
	activePipelineEventsTable = TablePipelineEvents
	telemetryMu               sync.RWMutex

	ErrTableNotFound = errors.New("telemetry table not found in database")
)

// SetTelemetryTables configures the active table names for pipeline runs and events.
func SetTelemetryTables(runsTable, eventsTable string) {
	telemetryMu.Lock()
	defer telemetryMu.Unlock()
	if strings.TrimSpace(runsTable) != "" {
		activePipelineRunsTable = strings.TrimSpace(runsTable)
	}
	if strings.TrimSpace(eventsTable) != "" {
		activePipelineEventsTable = strings.TrimSpace(eventsTable)
	}
}

// GetTelemetryTables returns the active table names for pipeline runs and events.
func GetTelemetryTables() (string, string) {
	telemetryMu.RLock()
	defer telemetryMu.RUnlock()
	return activePipelineRunsTable, activePipelineEventsTable
}

// PipelineRunRecord mirrors the schema of pipeline_runs for log inspection.
type PipelineRunRecord struct {
	RunID        string     `json:"run_id"`
	FilePath     string     `json:"file_path"`
	ConfigPath   string     `json:"config_path"`
	Status       string     `json:"status"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	DurationMs   int64      `json:"duration_ms"`
	TaskCount    int        `json:"task_count"`
	UserName     string     `json:"user_name"`
	Hostname     string     `json:"hostname"`
	OptionsPath  string     `json:"options_path"`
	ErrorClass   string     `json:"error_class"`
	ErrorMessage string     `json:"error_message"`
}

// PipelineEventRecord mirrors the schema of pipeline_events for event inspection.
type PipelineEventRecord struct {
	ID           int64      `json:"id,omitempty"`
	RunID        string     `json:"run_id"`
	ExecutionID  string     `json:"execution_id"`
	SequenceNum  int64      `json:"sequence_num"`
	OccurredAt   *time.Time `json:"occurred_at"`
	EventType    string     `json:"event_type"`
	NodeKind     string     `json:"node_kind"`
	NodeID       string     `json:"node_id"`
	Status       string     `json:"status"`
	UserName     string     `json:"user_name"`
	Hostname     string     `json:"hostname"`
	OptionsPath  string     `json:"options_path"`
	ErrorMessage string     `json:"error_message"`
	RowsRead     int64      `json:"rows_read"`
	RowsWritten  int64      `json:"rows_written"`
	RowsAffected int64      `json:"rows_affected"`
}

// isTableNotFoundError detects standard database errors when a table does not exist.
func isTableNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "invalid object name") ||
		strings.Contains(msg, "table or view does not exist")
}

func parseFlexibleTime(val any) *time.Time {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case time.Time:
		t := v.UTC()
		return &t
	case *time.Time:
		if v == nil {
			return nil
		}
		t := v.UTC()
		return &t
	case []byte:
		return parseTimeString(string(v))
	case string:
		return parseTimeString(v)
	default:
		return nil
	}
}

func parseTimeString(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05.999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			utc := t.UTC()
			return &utc
		}
	}
	return nil
}

// QueryPipelineRuns retrieves pipeline execution runs from the selected database.
func QueryPipelineRuns(ctx context.Context, db *sql.DB, driver string, runsTable string, search string, status string, limit int) ([]PipelineRunRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if runsTable == "" {
		runsTable = TablePipelineRuns
	}

	targetTable := runsTable
	if driver == "sqlserver" && !strings.Contains(targetTable, ".") {
		targetTable = "dbo." + targetTable
	}

	whereParts := []string{"1=1"}
	var args []any
	paramIndex := 1

	replacePlaceholders := func(clause string) string {
		var b strings.Builder
		for i := 0; i < len(clause); i++ {
			if clause[i] == '?' {
				switch driver {
				case "postgres":
					fmt.Fprintf(&b, "$%d", paramIndex)
				case "sqlserver":
					fmt.Fprintf(&b, "@p%d", paramIndex)
				case "oracle":
					fmt.Fprintf(&b, ":%d", paramIndex)
				default:
					b.WriteByte('?')
				}
				paramIndex++
			} else {
				b.WriteByte(clause[i])
			}
		}
		return b.String()
	}

	addFilter := func(clause string, filterArgs ...any) {
		converted := replacePlaceholders(clause)
		whereParts = append(whereParts, converted)
		args = append(args, filterArgs...)
	}

	if status = strings.TrimSpace(status); status != "" && !strings.EqualFold(status, "all") {
		addFilter("LOWER(status) = LOWER(?)", status)
	}

	if search = strings.TrimSpace(search); search != "" {
		pattern := "%" + search + "%"
		addFilter("(run_id LIKE ? OR file_path LIKE ? OR config_path LIKE ? OR user_name LIKE ? OR hostname LIKE ? OR error_class LIKE ? OR error_message LIKE ?)",
			pattern, pattern, pattern, pattern, pattern, pattern, pattern)
	}

	whereClause := strings.Join(whereParts, " AND ")
	cols := "run_id, file_path, config_path, status, started_at, finished_at, duration_ms, task_count, user_name, hostname, options_path, error_class, error_message"

	var query string
	switch driver {
	case "sqlserver":
		query = fmt.Sprintf("SELECT TOP (%d) %s FROM %s WHERE %s ORDER BY started_at DESC", limit, cols, targetTable, whereClause)
	case "oracle":
		query = fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY started_at DESC FETCH FIRST %d ROWS ONLY", cols, targetTable, whereClause, limit)
	default:
		// postgres, mysql, sqlite
		query = fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY started_at DESC LIMIT %d", cols, targetTable, whereClause, limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		if isTableNotFoundError(err) {
			return nil, ErrTableNotFound
		}
		// If oracle fails on FETCH FIRST, try without FETCH FIRST and cap in memory
		if driver == "oracle" {
			fallbackQuery := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY started_at DESC", cols, targetTable, whereClause)
			fallbackRows, fallbackErr := db.QueryContext(ctx, fallbackQuery, args...)
			if fallbackErr != nil {
				return nil, fallbackErr
			}
			rows = fallbackRows
		} else {
			return nil, err
		}
	}
	defer rows.Close()

	var runs []PipelineRunRecord
	for rows.Next() {
		var (
			rec                   PipelineRunRecord
			runID                 sql.NullString
			filePath              sql.NullString
			configPath            sql.NullString
			statusVal             sql.NullString
			startedRaw            any
			finishedRaw           any
			durationMs            sql.NullInt64
			taskCount             sql.NullInt64
			userName              sql.NullString
			hostname              sql.NullString
			optionsPath           sql.NullString
			errorClass            sql.NullString
			errorMessage          sql.NullString
		)

		if err := rows.Scan(
			&runID,
			&filePath,
			&configPath,
			&statusVal,
			&startedRaw,
			&finishedRaw,
			&durationMs,
			&taskCount,
			&userName,
			&hostname,
			&optionsPath,
			&errorClass,
			&errorMessage,
		); err != nil {
			return nil, fmt.Errorf("failed scanning pipeline run row: %w", err)
		}

		rec.RunID = runID.String
		rec.FilePath = filePath.String
		rec.ConfigPath = configPath.String
		rec.Status = statusVal.String
		rec.StartedAt = parseFlexibleTime(startedRaw)
		rec.FinishedAt = parseFlexibleTime(finishedRaw)
		rec.DurationMs = durationMs.Int64
		rec.TaskCount = int(taskCount.Int64)
		rec.UserName = userName.String
		rec.Hostname = hostname.String
		rec.OptionsPath = optionsPath.String
		rec.ErrorClass = errorClass.String
		rec.ErrorMessage = errorMessage.String

		runs = append(runs, rec)
		if len(runs) >= limit {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return runs, nil
}

// QueryPipelineEvents retrieves step-level telemetry events for a given run_id.
func QueryPipelineEvents(ctx context.Context, db *sql.DB, driver string, eventsTable, runID string) ([]PipelineEventRecord, error) {
	if eventsTable == "" {
		eventsTable = TablePipelineEvents
	}

	targetTable := eventsTable
	if driver == "sqlserver" && !strings.Contains(targetTable, ".") {
		targetTable = "dbo." + targetTable
	}

	cols := "run_id, execution_id, sequence_num, occurred_at, event_type, node_kind, node_id, status, user_name, hostname, options_path, error_message, rows_read, rows_written, rows_affected"

	placeholder := "?"
	switch driver {
	case "postgres":
		placeholder = "$1"
	case "sqlserver":
		placeholder = "@p1"
	case "oracle":
		placeholder = ":1"
	}

	query := fmt.Sprintf("SELECT %s FROM %s WHERE run_id = %s ORDER BY sequence_num ASC", cols, targetTable, placeholder)

	rows, err := db.QueryContext(ctx, query, runID)
	if err != nil {
		if isTableNotFoundError(err) {
			return nil, ErrTableNotFound
		}
		return nil, err
	}
	defer rows.Close()

	var events []PipelineEventRecord
	for rows.Next() {
		var (
			rec          PipelineEventRecord
			runIDVal     sql.NullString
			execID       sql.NullString
			seqNum       sql.NullInt64
			occurredRaw  any
			eventType    sql.NullString
			nodeKind     sql.NullString
			nodeID       sql.NullString
			statusVal    sql.NullString
			userName     sql.NullString
			hostname     sql.NullString
			optionsPath  sql.NullString
			errorMessage sql.NullString
			rowsRead     sql.NullInt64
			rowsWritten  sql.NullInt64
			rowsAffected sql.NullInt64
		)

		if err := rows.Scan(
			&runIDVal,
			&execID,
			&seqNum,
			&occurredRaw,
			&eventType,
			&nodeKind,
			&nodeID,
			&statusVal,
			&userName,
			&hostname,
			&optionsPath,
			&errorMessage,
			&rowsRead,
			&rowsWritten,
			&rowsAffected,
		); err != nil {
			return nil, fmt.Errorf("failed scanning pipeline event row: %w", err)
		}

		rec.RunID = runIDVal.String
		rec.ExecutionID = execID.String
		rec.SequenceNum = seqNum.Int64
		rec.OccurredAt = parseFlexibleTime(occurredRaw)
		rec.EventType = eventType.String
		rec.NodeKind = nodeKind.String
		rec.NodeID = nodeID.String
		rec.Status = statusVal.String
		rec.UserName = userName.String
		rec.Hostname = hostname.String
		rec.OptionsPath = optionsPath.String
		rec.ErrorMessage = errorMessage.String
		rec.RowsRead = rowsRead.Int64
		rec.RowsWritten = rowsWritten.Int64
		rec.RowsAffected = rowsAffected.Int64

		events = append(events, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}
