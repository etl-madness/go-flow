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
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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

// NodeDBTarget describes the database connection targets for a pipeline node
type NodeDBTarget struct {
	NodeKind string
	DBName   string
	TargetDB string
}

// DatabaseSessionIdentity captures the session ID, db user, application name, and server for a connection
type DatabaseSessionIdentity struct {
	SPID            int64
	DBUser          string
	ApplicationName string
	DBServer        string
}

type finishedNodeEvent struct {
	ExecutionID string
	NodeID      string
	NodePath    string
	Sequence    uint64
}

type DatabaseSink struct {
	DB               *sql.DB
	Driver           string
	insertQuery      string
	configuredCols   []string
	colsDetected     bool
	isNumericSPID    bool
	logNodeOutput    bool
	hasNodeOutputCol bool
	nodeOutputMaxLen int
	spid             int64
	dbUser           string
	appName          string
	dbServer         string
	mu               sync.RWMutex
	runID            string
	status           string
	errorClass       string
	errorMsg         string
	debug            bool
	registry         *flow.Registry
	nodeTargets      map[string]NodeDBTarget
	cachedIdentities map[string]DatabaseSessionIdentity
	finishedNodes    []finishedNodeEvent
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
	"db_server":          true,
	"database_server":    true,
	"spid":               true,
	"sspid":              true,
	"application_name":   true,
	"app_name":           true,
	"hostname":           true,
	"options_path":       true,
	"error_message":      true,
	"rows_read":          true,
	"rows_written":       true,
	"rows_affected":      true,
	"node_output":        true,
}

func defaultApplicationName() string {
	name := filepath.Base(os.Args[0])
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if name == "" || name == "." {
		return "flow"
	}
	return name
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

// FetchDatabaseSessionIdentity queries the database for the current connection SPID (or equivalent),
// database user name, and application name based on the driver dialect.
func FetchDatabaseSessionIdentity(ctx context.Context, querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, driver string) (spid int64, dbUser string, appName string, err error) {
	spid, dbUser, appName, _, err = FetchDatabaseSessionIdentityWithServer(ctx, querier, driver)
	return spid, dbUser, appName, err
}

func formatDBServerLabel(server, dbName string) string {
	server = strings.TrimSpace(server)
	dbName = strings.TrimSpace(dbName)
	switch {
	case server != "" && dbName != "":
		return server + "/" + dbName
	case server != "":
		return server
	default:
		return dbName
	}
}

// FetchDatabaseSessionIdentityWithServer queries the database for the current connection SPID,
// database user name, application name, and database server/catalog label based on the driver dialect.
func FetchDatabaseSessionIdentityWithServer(ctx context.Context, querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, driver string) (spid int64, dbUser string, appName string, dbServer string, err error) {
	if querier == nil {
		return 0, "", defaultApplicationName(), "", fmt.Errorf("nil database querier")
	}

	normDriver := strings.ToLower(driver)
	query5 := func(query string) (int64, string, string, string, error) {
		var sVal sql.NullInt64
		var uVal, aVal, srvVal, dbVal sql.NullString
		e := querier.QueryRowContext(ctx, query).Scan(&sVal, &uVal, &aVal, &srvVal, &dbVal)
		if e != nil {
			return 0, "", "", "", e
		}
		app := strings.TrimSpace(aVal.String)
		if app == "" {
			app = defaultApplicationName()
		}
		return sVal.Int64, uVal.String, app, formatDBServerLabel(srvVal.String, dbVal.String), nil
	}
	query3 := func(query string) (int64, string, string, error) {
		var sVal sql.NullInt64
		var uVal, aVal sql.NullString
		e := querier.QueryRowContext(ctx, query).Scan(&sVal, &uVal, &aVal)
		if e != nil {
			return 0, "", "", e
		}
		app := strings.TrimSpace(aVal.String)
		if app == "" {
			app = defaultApplicationName()
		}
		return sVal.Int64, uVal.String, app, nil
	}

	switch {
	case strings.Contains(normDriver, "sqlserver") || strings.Contains(normDriver, "mssql"):
		if s, u, a, srv, e := query5("SELECT CAST(@@SPID AS BIGINT), CAST(SYSTEM_USER AS NVARCHAR(256)), CAST(ISNULL(APP_NAME(), '') AS NVARCHAR(256)), CAST(COALESCE(CAST(SERVERPROPERTY('ServerName') AS NVARCHAR(256)), @@SERVERNAME, '') AS NVARCHAR(256)), CAST(ISNULL(DB_NAME(), '') AS NVARCHAR(256))"); e == nil {
			return s, u, a, srv, nil
		}
		if s, u, a, e := query3("SELECT @@SPID, SYSTEM_USER, ISNULL(APP_NAME(), '')"); e == nil {
			return s, u, a, "", nil
		}
	case strings.Contains(normDriver, "postgres") || strings.Contains(normDriver, "pq") || strings.Contains(normDriver, "pgx"):
		if s, u, a, srv, e := query5("SELECT pg_backend_pid(), CURRENT_USER, COALESCE(current_setting('application_name', true), ''), COALESCE(host(inet_server_addr()), 'localhost'), COALESCE(current_database(), '')"); e == nil {
			return s, u, a, srv, nil
		}
		if s, u, a, e := query3("SELECT pg_backend_pid(), CURRENT_USER, COALESCE(current_setting('application_name', true), '')"); e == nil {
			return s, u, a, "", nil
		}
	case strings.Contains(normDriver, "mysql"):
		if s, u, a, srv, e := query5("SELECT CONNECTION_ID(), CURRENT_USER(), COALESCE(@@session.program_name, ''), COALESCE(@@hostname, ''), COALESCE(DATABASE(), '')"); e == nil {
			return s, u, a, srv, nil
		}
		if s, u, a, srv, e := query5("SELECT CONNECTION_ID(), CURRENT_USER(), '', COALESCE(@@hostname, ''), COALESCE(DATABASE(), '')"); e == nil {
			return s, u, a, srv, nil
		}
		var sVal sql.NullInt64
		var uVal sql.NullString
		if e := querier.QueryRowContext(ctx, "SELECT CONNECTION_ID(), CURRENT_USER()").Scan(&sVal, &uVal); e == nil {
			return sVal.Int64, uVal.String, defaultApplicationName(), "", nil
		}
	case strings.Contains(normDriver, "oracle") || strings.Contains(normDriver, "ora"):
		if s, u, a, srv, e := query5("SELECT TO_NUMBER(SYS_CONTEXT('USERENV', 'SID')), SYS_CONTEXT('USERENV', 'SESSION_USER'), COALESCE(SYS_CONTEXT('USERENV', 'MODULE'), SYS_CONTEXT('USERENV', 'CLIENT_PROGRAM_NAME'), ''), COALESCE(SYS_CONTEXT('USERENV', 'SERVER_HOST'), ''), COALESCE(SYS_CONTEXT('USERENV', 'DB_NAME'), '') FROM DUAL"); e == nil {
			return s, u, a, srv, nil
		}
		if s, u, a, e := query3("SELECT TO_NUMBER(SYS_CONTEXT('USERENV', 'SID')), SYS_CONTEXT('USERENV', 'SESSION_USER'), COALESCE(SYS_CONTEXT('USERENV', 'MODULE'), SYS_CONTEXT('USERENV', 'CLIENT_PROGRAM_NAME'), '') FROM DUAL"); e == nil {
			return s, u, a, "", nil
		}
	case strings.Contains(normDriver, "sqlite"):
		userName, _ := runtimeIdentity()
		if userName == "" {
			userName = "sqlite"
		}
		srv := "sqlite"
		var seq sql.NullInt64
		var name, file sql.NullString
		if pErr := querier.QueryRowContext(ctx, "PRAGMA database_list").Scan(&seq, &name, &file); pErr == nil {
			f := strings.TrimSpace(file.String)
			if f != "" {
				if idx := strings.LastIndexAny(f, `/\`); idx >= 0 && idx+1 < len(f) {
					f = f[idx+1:]
				}
				srv = "sqlite:" + f
			}
		}
		return int64(os.Getpid()), userName, defaultApplicationName(), srv, nil
	}

	userName, _ := runtimeIdentity()
	return int64(os.Getpid()), userName, defaultApplicationName(), "", nil
}

// NewDatabaseSink constructs a DatabaseSink and pre-builds the dialect-specific SQL query
func NewDatabaseSink(db *sql.DB, driver string) *DatabaseSink {
	sink := &DatabaseSink{
		DB:      db,
		Driver:  driver,
		appName: defaultApplicationName(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if db != nil {
		spid, dbUser, appName, dbServer, err := FetchDatabaseSessionIdentityWithServer(ctx, db, driver)
		if err == nil {
			sink.spid = spid
			sink.dbUser = dbUser
			sink.appName = appName
			sink.dbServer = dbServer
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

	table := resolveCLITableName(s.Driver, pipeline_events)

	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s WHERE 1=0", table))
	if err == nil {
		colTypes, typeErr := rows.ColumnTypes()
		colNames, colErr := rows.Columns()
		rows.Close()
		if colErr == nil && len(colNames) > 0 {
			var available []string
			s.hasNodeOutputCol = false
			for _, c := range colNames {
				lower := strings.ToLower(strings.TrimSpace(c))
				if lower == "node_output" {
					s.hasNodeOutputCol = true
					if !s.logNodeOutput {
						continue
					}
				}
				if knownEventCols[lower] {
					available = append(available, lower)
				}
			}
			if len(available) == 0 {
				s.configuredCols = defaultEventCols
			} else {
				s.configuredCols = available
			}
			s.insertQuery = buildInsertQuery(s.Driver, pipeline_events, s.configuredCols)
			s.colsDetected = true
			if typeErr == nil && len(colTypes) > 0 {
				for _, ct := range colTypes {
					name := strings.ToLower(ct.Name())
					if name == "spid" || name == "sspid" {
						typeName := strings.ToUpper(ct.DatabaseTypeName())
						if (strings.Contains(typeName, "INT") || strings.Contains(typeName, "NUM") || strings.Contains(typeName, "DEC")) &&
							!strings.Contains(typeName, "CHAR") && !strings.Contains(typeName, "TEXT") {
							s.isNumericSPID = true
						}
					} else if name == "node_output" {
						if colLen, ok := ct.Length(); ok && colLen > 0 && colLen < int64(maxNodeOutputBytes) {
							s.nodeOutputMaxLen = int(colLen)
						}
					}
				}
			}
			return
		}
	}

	s.configuredCols = defaultEventCols
	s.insertQuery = buildInsertQuery(s.Driver, pipeline_events, defaultEventCols)
}

// SetLogNodeOutput enables or disables logging of node output to the node_output column
func (s *DatabaseSink) SetLogNodeOutput(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logNodeOutput = enabled
	if s.DB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		s.detectColumns(ctx)
	}
}

// LogNodeOutput returns whether logging of node output is enabled
func (s *DatabaseSink) LogNodeOutput() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logNodeOutput
}

// SPID returns the session ID or process ID captured for the database connection
func (s *DatabaseSink) SPID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.spid
}

// SSPID returns the session ID or process ID captured for the database connection
func (s *DatabaseSink) SSPID() int64 {
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

// ApplicationName returns the application name captured for the database connection
func (s *DatabaseSink) ApplicationName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.appName
}

// SessionIdentity returns SPID, DB User, and Application Name
func (s *DatabaseSink) SessionIdentity() (int64, string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.spid, s.dbUser, s.appName
}

// DBServer returns the database server/catalog label captured for the logging connection
func (s *DatabaseSink) DBServer() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dbServer
}

// SetSessionIdentity explicitly overrides the session identity for testing or connection pooling
func (s *DatabaseSink) SetSessionIdentity(spid int64, dbUser string, appName ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spid = spid
	s.dbUser = dbUser
	if len(appName) > 0 && appName[0] != "" {
		s.appName = appName[0]
	} else if s.appName == "" {
		s.appName = defaultApplicationName()
	}
	if len(appName) > 1 && appName[1] != "" {
		s.dbServer = appName[1]
	}
}

// SetRegistry sets the flow.Registry used to look up active database connection pools
func (s *DatabaseSink) SetRegistry(reg *flow.Registry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registry = reg
}

// SetDBSessionIdentity sets a known session identity for a specific database name
func (s *DatabaseSink) SetDBSessionIdentity(dbName string, spid int64, dbUser string, appName string, dbServer ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cachedIdentities == nil {
		s.cachedIdentities = make(map[string]DatabaseSessionIdentity)
	}
	srv := ""
	if len(dbServer) > 0 {
		srv = dbServer[0]
	}
	s.cachedIdentities[dbName] = DatabaseSessionIdentity{
		SPID:            spid,
		DBUser:          dbUser,
		ApplicationName: appName,
		DBServer:        srv,
	}
}

// RegisterNodeTarget registers the DB targets for a given node ID or node path
func (s *DatabaseSink) RegisterNodeTarget(nodeIDOrPath string, target NodeDBTarget) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nodeTargets == nil {
		s.nodeTargets = make(map[string]NodeDBTarget)
	}
	s.nodeTargets[nodeIDOrPath] = target
}

// RegisterNodes registers the AST nodes to map execution events to actual database connections
func (s *DatabaseSink) RegisterNodes(nodes []flow.PipelineNode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nodeTargets == nil {
		s.nodeTargets = make(map[string]NodeDBTarget)
	}
	s.registerNodesRecursive(nodes, "")
}

func (s *DatabaseSink) registerNodesRecursive(nodes []flow.PipelineNode, parentPath string) {
	for index, node := range nodes {
		kind, id := nodeIdentityFromAST(node)
		path := fmt.Sprintf("%s[%d]", kind, index)
		if parentPath != "" {
			path = fmt.Sprintf("%s/%s", parentPath, path)
		}

		var target NodeDBTarget
		hasTarget := false

		switch node.Kind {
		case flow.NodeSQL:
			if node.SQL != nil {
				target = NodeDBTarget{
					NodeKind: "sql",
					DBName:   node.SQL.DBName,
				}
				hasTarget = true
			}
		case flow.NodeSQLBulk:
			if node.SQLBulk != nil {
				targetDB := node.SQLBulk.TargetDB
				if targetDB == "" {
					targetDB = node.SQLBulk.DBName
				}
				target = NodeDBTarget{
					NodeKind: "sql_bulk",
					DBName:   node.SQLBulk.DBName,
					TargetDB: targetDB,
				}
				hasTarget = true
			}
		case flow.NodeGroup:
			if node.Transaction && node.DBName != "" {
				target = NodeDBTarget{
					NodeKind: "group",
					DBName:   node.DBName,
				}
				hasTarget = true
			}
		case flow.NodeKV:
			if node.KV != nil && node.KV.DBName != "" {
				target = NodeDBTarget{
					NodeKind: "kv",
					DBName:   node.KV.DBName,
				}
				hasTarget = true
			}
		case flow.NodeKVBulk:
			if node.KVBulk != nil && node.KVBulk.DBName != "" {
				target = NodeDBTarget{
					NodeKind: "kv_bulk",
					DBName:   node.KVBulk.DBName,
				}
				hasTarget = true
			}
		}

		if hasTarget {
			s.nodeTargets[path] = target
			if id != "" {
				s.nodeTargets[id] = target
			}
		}

		if len(node.Children) > 0 {
			s.registerNodesRecursive(node.Children, path)
		}
		if len(node.ElseNodes) > 0 {
			s.registerNodesRecursive(node.ElseNodes, path)
		}
	}
}

func nodeIdentityFromAST(node flow.PipelineNode) (string, string) {
	switch node.Kind {
	case flow.NodeKV:
		if node.KV != nil {
			return "kv", node.KV.ID
		}
		return "kv", ""
	case flow.NodeKVBulk:
		if node.KVBulk != nil {
			return "kv_bulk", node.KVBulk.ID
		}
		return "kv_bulk", ""
	case flow.NodeAssert:
		if node.Assert != nil {
			return "assert", node.Assert.ID
		}
		return "assert", ""
	case flow.NodeSQL:
		if node.SQL != nil {
			return "sql", node.SQL.ID
		}
		return "sql", ""
	case flow.NodeSQLBulk:
		if node.SQLBulk != nil {
			return "sql_bulk", node.SQLBulk.ID
		}
		return "sql_bulk", ""
	case flow.NodeScript:
		if node.Script != nil {
			return "script", node.Script.ID
		}
		return "script", ""
	case flow.NodeHTTPClient:
		if node.HTTPClient != nil {
			return "http_client", node.HTTPClient.ID
		}
		return "http_client", ""
	case flow.NodeTemplate:
		if node.Template != nil {
			return "template", node.Template.ID
		}
		return "template", ""
	case flow.NodeHtmlTemplate:
		if node.HtmlTemplate != nil {
			return "template_html", node.HtmlTemplate.ID
		}
		return "template_html", ""
	case flow.NodeFileSave:
		if node.FileSave != nil {
			return "file_save", node.FileSave.ID
		}
		return "file_save", ""
	case flow.NodeFileRead:
		if node.FileRead != nil {
			return "file_read", node.FileRead.ID
		}
		return "file_read", ""
	case flow.NodeExcelRead:
		if node.ExcelRead != nil {
			return "excel_read", node.ExcelRead.ID
		}
		return "excel_read", ""
	case flow.NodeExcelWrite:
		if node.ExcelWrite != nil {
			return "excel_write", node.ExcelWrite.ID
		}
		return "excel_write", ""
	case flow.NodeXMLXPath:
		if node.XmlXPath != nil {
			return "xml_xpath", node.XmlXPath.ID
		}
		return "xml_xpath", ""
	case flow.NodeJSONPath:
		if node.JsonPath != nil {
			return "json_path", node.JsonPath.ID
		}
		return "json_path", ""
	case flow.NodeYAMLPath:
		if node.YamlPath != nil {
			return "yaml_path", node.YamlPath.ID
		}
		return "yaml_path", ""
	case flow.NodeGroup:
		return "group", node.GroupID
	case flow.NodeParallel:
		return "parallel", node.GroupID
	case flow.NodeIf:
		return "if", node.GroupID
	case flow.NodeForEach:
		return "foreach", node.GroupID
	case flow.NodeWhile:
		return "while", node.GroupID
	default:
		return "unknown", ""
	}
}

func (s *DatabaseSink) resolveDBName(name string) string {
	name = strings.TrimSpace(name)
	if !strings.Contains(name, "{{") {
		return name
	}
	s.mu.RLock()
	reg := s.registry
	s.mu.RUnlock()
	if reg == nil {
		return name
	}
	vars := reg.CopyVariables()
	for k, v := range vars {
		placeholder := fmt.Sprintf("{{%s}}", k)
		name = strings.ReplaceAll(name, placeholder, fmt.Sprintf("%v", v))
	}
	return strings.TrimSpace(name)
}

func (s *DatabaseSink) getOrFetchSessionIdentity(ctx context.Context, dbName string) (DatabaseSessionIdentity, error) {
	s.mu.RLock()
	if s.cachedIdentities != nil {
		if ident, ok := s.cachedIdentities[dbName]; ok {
			s.mu.RUnlock()
			return ident, nil
		}
	}
	defaultSPID := s.spid
	defaultDBUser := s.dbUser
	defaultAppName := s.appName
	defaultDBServer := s.dbServer
	reg := s.registry
	s.mu.RUnlock()

	if dbName == "" || dbName == "log_db" {
		return DatabaseSessionIdentity{
			SPID:            defaultSPID,
			DBUser:          defaultDBUser,
			ApplicationName: defaultAppName,
			DBServer:        defaultDBServer,
		}, nil
	}

	if reg == nil {
		return DatabaseSessionIdentity{}, fmt.Errorf("registry not set")
	}

	handle, err := reg.GetDBHandle(dbName)
	if err != nil {
		return DatabaseSessionIdentity{}, err
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	spid, dbUser, appName, dbServer, err := FetchDatabaseSessionIdentityWithServer(fetchCtx, handle.Conn, handle.Driver)
	if err != nil {
		return DatabaseSessionIdentity{}, err
	}

	ident := DatabaseSessionIdentity{
		SPID:            spid,
		DBUser:          dbUser,
		ApplicationName: appName,
		DBServer:        dbServer,
	}

	s.mu.Lock()
	if s.cachedIdentities == nil {
		s.cachedIdentities = make(map[string]DatabaseSessionIdentity)
	}
	s.cachedIdentities[dbName] = ident
	s.mu.Unlock()

	return ident, nil
}

func formatBulkServerPair(src, tgt string) string {
	src = strings.TrimSpace(src)
	tgt = strings.TrimSpace(tgt)
	switch {
	case src != "" && tgt != "":
		return fmt.Sprintf("%s->%s", src, tgt)
	case tgt != "":
		return tgt
	default:
		return src
	}
}

func (s *DatabaseSink) resolveEventIdentity(ctx context.Context, event flow.ExecutionEvent) (spid any, srcSPIDInt int64, appName string, dbUser string, dbServer string) {
	s.mu.RLock()
	defaultSPID := s.spid
	defaultDBUser := s.dbUser
	defaultAppName := s.appName
	defaultDBServer := s.dbServer
	var target NodeDBTarget
	var found bool
	if s.nodeTargets != nil {
		if event.NodePath != "" {
			target, found = s.nodeTargets[event.NodePath]
		}
		if !found && event.NodeID != "" {
			target, found = s.nodeTargets[event.NodeID]
		}
	}
	s.mu.RUnlock()

	if !found {
		return defaultSPID, defaultSPID, defaultAppName, defaultDBUser, defaultDBServer
	}

	if target.NodeKind == "sql_bulk" {
		srcDB := s.resolveDBName(target.DBName)
		tgtDB := s.resolveDBName(target.TargetDB)
		if tgtDB == "" {
			tgtDB = srcDB
		}

		if srcDB != "" && tgtDB != "" && srcDB != tgtDB {
			srcIdent, srcErr := s.getOrFetchSessionIdentity(ctx, srcDB)
			tgtIdent, tgtErr := s.getOrFetchSessionIdentity(ctx, tgtDB)
			if srcErr == nil && tgtErr == nil {
				return fmt.Sprintf("%d->%d", srcIdent.SPID, tgtIdent.SPID),
					srcIdent.SPID,
					fmt.Sprintf("%s->%s", srcIdent.ApplicationName, tgtIdent.ApplicationName),
					fmt.Sprintf("%s->%s", srcIdent.DBUser, tgtIdent.DBUser),
					formatBulkServerPair(srcIdent.DBServer, tgtIdent.DBServer)
			}
		} else if srcDB != "" {
			ident, err := s.getOrFetchSessionIdentity(ctx, srcDB)
			if err == nil {
				return ident.SPID, ident.SPID, ident.ApplicationName, ident.DBUser, ident.DBServer
			}
		}
		return defaultSPID, defaultSPID, defaultAppName, defaultDBUser, defaultDBServer
	}

	dbName := s.resolveDBName(target.DBName)
	if dbName != "" {
		ident, err := s.getOrFetchSessionIdentity(ctx, dbName)
		if err == nil {
			return ident.SPID, ident.SPID, ident.ApplicationName, ident.DBUser, ident.DBServer
		}
	}

	return defaultSPID, defaultSPID, defaultAppName, defaultDBUser, defaultDBServer
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
	if event.Type == flow.EventNodeFinished {
		s.finishedNodes = append(s.finishedNodes, finishedNodeEvent{
			ExecutionID: event.ExecutionID,
			NodeID:      event.NodeID,
			NodePath:    event.NodePath,
			Sequence:    event.Sequence,
		})
	}
	if !s.colsDetected && s.DB != nil {
		s.detectColumns(ctx)
	}
	cols := s.configuredCols
	query := s.insertQuery
	s.mu.Unlock()

	spid, srcSPIDInt, appName, dbUser, dbServer := s.resolveEventIdentity(ctx, event)

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
		case "db_server", "database_server":
			args = append(args, dbServer)
		case "spid", "sspid":
			if s.isNumericSPID && s.Driver != "sqlite" && s.Driver != "sqlite3" {
				args = append(args, srcSPIDInt)
			} else {
				args = append(args, spid)
			}
		case "application_name", "app_name":
			args = append(args, appName)
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
		case "node_output":
			args = append(args, nil)
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

const (
	maxNodeOutputBytes         = 65536 // 64 KB safety cap for node_output
	nodeOutputTruncationSuffix = "\n...[TRUNCATED]"
)

var sensitiveOutputPattern = regexp.MustCompile(`(?i)(password|pwd|token|secret|authorization)\s*([=:])\s*([^\s,;]+)`)

func truncateNodeOutput(output string, maxLen int) string {
	if maxLen <= 0 || len(output) <= maxLen {
		return output
	}
	suffixLen := len(nodeOutputTruncationSuffix)
	if maxLen <= suffixLen {
		cut := maxLen
		for cut > 0 && !utf8.RuneStart(output[cut]) {
			cut--
		}
		return output[:cut]
	}
	cut := maxLen - suffixLen
	for cut > 0 && !utf8.RuneStart(output[cut]) {
		cut--
	}
	return output[:cut] + nodeOutputTruncationSuffix
}

func redactNodeOutput(output string) string {
	if output == "" {
		return ""
	}
	redacted := sensitiveOutputPattern.ReplaceAllString(output, "$1$2[REDACTED]")
	return truncateNodeOutput(redacted, maxNodeOutputBytes)
}

func buildUpdateNodeFinishedOutputQuery(driverType, table string) string {
	table = resolveCLITableName(driverType, table)
	switch driverType {
	case "postgres":
		return fmt.Sprintf("UPDATE %s SET node_output = $1 WHERE run_id = $2 AND execution_id = $3 AND event_type = 'node.finished'", table)
	case "sqlserver":
		return fmt.Sprintf("UPDATE %s SET node_output = @p1 WHERE run_id = @p2 AND execution_id = @p3 AND event_type = 'node.finished'", table)
	case "oracle":
		return fmt.Sprintf("UPDATE %s SET node_output = :1 WHERE run_id = :2 AND execution_id = :3 AND event_type = 'node.finished'", table)
	default:
		return fmt.Sprintf("UPDATE %s SET node_output = ? WHERE run_id = ? AND execution_id = ? AND event_type = 'node.finished'", table)
	}
}

func matchNodeOutputs(finished []finishedNodeEvent, results []flow.ScriptResult) map[string]string {
	matched := make(map[string]string)
	if len(finished) == 0 || len(results) == 0 {
		return matched
	}
	used := make([]bool, len(finished))
	for _, res := range results {
		foundIdx := -1
		for i, fn := range finished {
			if used[i] {
				continue
			}
			if fn.NodeID == res.ScriptID {
				foundIdx = i
				break
			}
		}
		if foundIdx >= 0 {
			used[foundIdx] = true
			if finished[foundIdx].ExecutionID != "" {
				matched[finished[foundIdx].ExecutionID] = redactNodeOutput(res.ResultsString)
			}
		}
	}
	return matched
}

// RecordNodeOutputs updates the node_output column on node.finished events in pipeline_events
// when node output logging is enabled and the column exists in the table.
func (s *DatabaseSink) RecordNodeOutputs(ctx context.Context, results []flow.ScriptResult) error {
	if s == nil || s.DB == nil || !s.LogNodeOutput() || len(results) == 0 {
		return nil
	}

	s.mu.Lock()
	if !s.colsDetected {
		s.detectColumns(ctx)
	}
	hasCol := s.hasNodeOutputCol
	maxColLen := s.nodeOutputMaxLen
	runID := s.runID
	finished := make([]finishedNodeEvent, len(s.finishedNodes))
	copy(finished, s.finishedNodes)
	s.mu.Unlock()

	if !hasCol || runID == "" || len(finished) == 0 {
		return nil
	}

	outputsByExecID := matchNodeOutputs(finished, results)
	if len(outputsByExecID) == 0 {
		return nil
	}

	query := buildUpdateNodeFinishedOutputQuery(s.Driver, pipeline_events)
	var firstErr error
	for execID, out := range outputsByExecID {
		if maxColLen > 0 {
			out = truncateNodeOutput(out, maxColLen)
		}
		if _, err := s.DB.ExecContext(ctx, query, out, runID, execID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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
	DBServer         string
	SPID             int64
	ApplicationName  string
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
	"db_server":          true,
	"database_server":    true,
	"spid":               true,
	"sspid":              true,
	"application_name":   true,
	"app_name":           true,
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

	table := resolveCLITableName(driverType, pipeline_runs)

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
	appName := rec.ApplicationName
	if appName == "" {
		appName = defaultApplicationName()
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
		case "db_server", "database_server":
			args = append(args, rec.DBServer)
		case "spid", "sspid":
			args = append(args, rec.SPID)
		case "application_name", "app_name":
			args = append(args, appName)
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
	case strings.Contains(lowerDSN, "fedauth=") || strings.Contains(lowerDSN, "activedirectory"):
		return "azuresql"
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
	case strings.Contains(driverPkg, "mssql"), strings.Contains(driverPkg, "azuread"):
		return "sqlserver"
	case strings.Contains(driverPkg, "godror"), strings.Contains(driverPkg, "oracle"):
		return "oracle"
	default:
		return driverPkg
	}
}

func resolveCLITableName(driverType, table string) string {
	d := strings.ToLower(strings.TrimSpace(driverType))
	t := strings.TrimSpace(table)
	switch d {
	case "sqlite", "sqlite3":
		return flow.BaseTableName(t)
	case "mysql", "mariadb":
		if strings.HasPrefix(strings.ToLower(t), "dbo.") {
			return t[4:]
		}
		return t
	case "sqlserver", "mssql", "azuresql":
		if !strings.Contains(t, ".") {
			return activeTableConfig.SchemaPrefix() + t
		}
		return t
	default:
		if !strings.Contains(t, ".") && activeTableConfig.Schema != "" && !strings.EqualFold(activeTableConfig.Schema, "dbo") {
			return activeTableConfig.SchemaPrefix() + t
		}
		return t
	}
}

func buildInsertQuery(driverType, table string, columns []string) string {
	var placeholders []string
	table = resolveCLITableName(driverType, table)
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
