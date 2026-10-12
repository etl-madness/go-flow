package builder

// ComponentField defines an attribute or field configuration for a component.
type ComponentField struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // text, textarea, select, bool, int
	Mandatory   bool     `json:"mandatory"`
	Default     string   `json:"default"`
	Description string   `json:"description"`
	Options     []string `json:"options,omitempty"`
}

// ComponentMeta defines the metadata, allowed attributes, default template, and documentation for a component.
type ComponentMeta struct {
	Type        string           `json:"type"`
	Tag         string           `json:"tag"`
	Name        string           `json:"name"`
	Category    string           `json:"category"`
	Description string           `json:"description"`
	Section     string           `json:"section"` // variables, databases, preflight, flow, or any
	HasContent  bool             `json:"has_content"`
	ContentHelp string           `json:"content_help,omitempty"`
	Fields      []ComponentField `json:"fields"`
	DefaultXML  string           `json:"default_xml"`
}

// ComponentCatalog provides the full list of supported components.
type ComponentCatalog struct {
	Components []ComponentMeta
	Categories []string
}

// GetCatalog returns the full built-in component catalog for Flow.
func GetCatalog() *ComponentCatalog {
	components := []ComponentMeta{
		// 1. Pipeline Setup & Environment
		{
			Type:        "variable",
			Tag:         "variable",
			Name:        "Variable",
			Category:    "Pipeline Setup",
			Description: "Initializes a named pipeline variable with explicit type conversion and variable interpolation support.",
			Section:     "variables",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Identifier", Type: "text", Mandatory: false, Description: "Optional unique identifier for the variable definition"},
				{Name: "name", Label: "Variable Name", Type: "text", Mandatory: true, Description: "Unique variable name accessed via {{Name}}"},
				{Name: "value", Label: "Value", Type: "text", Mandatory: true, Description: "Initial variable value"},
				{Name: "type", Label: "Data Type", Type: "select", Mandatory: false, Default: "string", Options: []string{"string", "int", "bool", "float", "datetime", "date"}},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Human-readable description of the variable"},
			},
			DefaultXML: `<variable name="TargetTable" value="processed_logs" type="string" description="Target table for ETL" />`,
		},
		{
			Type:        "database",
			Tag:         "database",
			Name:        "Database Connection",
			Category:    "Pipeline Setup",
			Description: "Defines a named database connection pool (MSSQL, PostgreSQL, MySQL, SQLite, Oracle, KV).",
			Section:     "databases",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "name", Label: "Connection Name", Type: "text", Mandatory: true, Description: "Unique connection name referenced by db attribute"},
				{Name: "driver", Label: "Driver", Type: "select", Mandatory: true, Default: "sqlite", Options: []string{"sqlite", "sqlserver", "azuresql", "postgres", "mysql", "oracle", "bbolt", "badger", "redis", "etcd"}},
				{Name: "connection_string", Label: "Connection String / DSN", Type: "text", Mandatory: true, Description: "Driver-specific connection string or path. Supports {{Variables}}"},
				{Name: "workload", Label: "Workload Profile", Type: "select", Mandatory: false, Default: "", Options: []string{"", "oltp", "bulk", "analytics", "batch"}, Description: "Workload tuning profile for connection pool defaults (oltp, bulk, analytics, batch)"},
				{Name: "max_open_conns", Label: "Max Open Conns", Type: "int", Mandatory: false, Default: "25"},
				{Name: "max_idle_conns", Label: "Max Idle Conns", Type: "int", Mandatory: false, Default: "10"},
				{Name: "conn_max_lifetime_seconds", Label: "Max Lifetime (sec or duration)", Type: "text", Mandatory: false, Default: "300", Description: "Maximum duration a connection is recycled (e.g. 300 or 5m)"},
			},
			DefaultXML: `<database name="local_sqlite" driver="sqlite" connection_string="./pipeline_data.db" />`,
		},
		{
			Type:        "preflight",
			Tag:         "preflight",
			Name:        "Preflight Container",
			Category:    "Pipeline Setup",
			Description: "A container for pre-execution quality gates, environment readiness checks, and assertions.",
			Section:     "preflight",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: false, Description: "Identifier for preflight group"},
			},
			DefaultXML: `<preflight></preflight>`,
		},

		// 2. Control Flow & Orchestration
		{
			Type:        "script",
			Tag:         "script",
			Name:        "Script Execution",
			Category:    "Control Flow",
			Description: "Executes dynamic code with cross-platform runtimes (zero-dependency embedded Go, PowerShell Core, Windows PowerShell, Bash, POSIX sh, Cmd, Git-Bash, Zsh, .NET Script).",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "Inline script code executed by the chosen interpreter runtime. Supports variable interpolation and host APIs.",
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and telemetry"},
				{Name: "language", Label: "Script Language / Runtime", Type: "select", Mandatory: true, Default: "powershell", Options: []string{"powershell", "pwsh", "shell", "bash", "cmd", "go", "git-bash", "zsh", "dotnet-script", "csx"}, Description: "Target interpreter runtime (e.g. go, pwsh, powershell, bash, sh)"},
				{Name: "output_var", Label: "Output Variable", Type: "text", Mandatory: false, Description: "Variable name receiving script standard output (stdout)"},
				{Name: "var", Label: "Script Body Variable", Type: "text", Mandatory: false, Description: "Optional pipeline variable containing script code to execute (overrides body)"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Execution timeout duration (e.g. 10s, 45s, 5m, 1h)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Human-readable explanation of script action"},
				{Name: "on_error", Label: "On Error", Type: "select", Mandatory: false, Default: "stop", Options: []string{"stop", "continue", "retry"}, Description: "Failure policy when the script exits with non-zero code"},
				{Name: "retry_count", Label: "Retry Count", Type: "int", Mandatory: false, Default: "0", Description: "Number of retries when on_error is set to retry"},
				{Name: "retry_interval", Label: "Retry Interval", Type: "text", Mandatory: false, Description: "Delay between retries (e.g. 2s, 10s)"},
				{Name: "db", Label: "Database Alias", Type: "text", Mandatory: false, Description: "Optional database connection alias for language='sql' or database scripts"},
			},
			DefaultXML: `<script id="RunScript" language="powershell" output_var="ScriptOutput">
Write-Host "Running pipeline script..."
</script>`,
		},
		{
			Type:        "group",
			Tag:         "group",
			Name:        "Execution Group",
			Category:    "Control Flow",
			Description: "Groups pipeline steps together with independent retry, transaction boundaries, and error policies.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Group ID", Type: "text", Mandatory: true, Description: "Unique group step identifier"},
				{Name: "transaction", Label: "Transactional", Type: "bool", Mandatory: false, Default: "false", Options: []string{"true", "false"}, Description: "Wraps child steps in an atomic database transaction with savepoint nesting"},
				{Name: "db", Label: "Database Connection", Type: "text", Mandatory: false, Description: "Database connection name for transaction group and savepoint scoping"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Transaction execution deadline duration (e.g. 30s, 5m)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Description of the group block"},
				{Name: "condition", Label: "Condition Expression", Type: "text", Mandatory: false, Description: "Inline boolean expression evaluated before execution (e.g. {{IsActive}} == true)"},
				{Name: "if_var", Label: "Condition Variable", Type: "text", Mandatory: false, Description: "Pipeline variable name evaluated before executing child steps"},
				{Name: "if_equals", Label: "Expected Value", Type: "text", Mandatory: false, Description: "Expected variable value required for execution"},
				{Name: "on_error", Label: "On Error", Type: "select", Mandatory: false, Default: "stop", Options: []string{"stop", "continue", "retry"}},
				{Name: "retry_count", Label: "Retry Count", Type: "int", Mandatory: false, Default: "0"},
				{Name: "retry_interval", Label: "Retry Interval", Type: "text", Mandatory: false, Description: "Delay between retries (e.g. 2s, 10s)"},
			},
			DefaultXML: `<group id="TransformBatch" transaction="false" on_error="stop"></group>`,
		},
		{
			Type:        "if",
			Tag:         "if",
			Name:        "Conditional Branch",
			Category:    "Control Flow",
			Description: "Evaluates an expression or variable equality to execute steps in <then> and <else> branches.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Condition ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "var", Label: "Variable Name", Type: "text", Mandatory: false, Description: "Variable name to evaluate (alias: if_var)"},
				{Name: "equals", Label: "Equals Value", Type: "text", Mandatory: false, Description: "Expected target value for truthy branch (aliases: val, value, if_val, if_equals)"},
				{Name: "condition", Label: "Condition Expression", Type: "text", Mandatory: false, Description: "Dynamic boolean or template expression (alias: cond)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Description of the conditional logic"},
			},
			DefaultXML: `<if id="CheckStatus" var="JobStatus" equals="COMPLETED">
    <then></then>
    <else></else>
</if>`,
		},
		{
			Type:        "foreach",
			Tag:         "foreach",
			Name:        "Foreach Loop",
			Category:    "Control Flow",
			Description: "Iterates over rows returned by a SQL query with constant-memory streaming or optional buffering.",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "SQL driver query that returns rows to iterate over (e.g. SELECT id, name FROM items).",
			Fields: []ComponentField{
				{Name: "id", Label: "Loop ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "db", Label: "Database Connection", Type: "text", Mandatory: true, Description: "Database connection name executing driver query"},
				{Name: "stream", Label: "Streaming Mode", Type: "bool", Mandatory: false, Default: "true", Options: []string{"true", "false"}, Description: "Constant-memory streaming cursor mode (default true)"},
				{Name: "buffer", Label: "Buffer In-Memory", Type: "bool", Mandatory: false, Default: "false", Options: []string{"true", "false"}, Description: "Opt-in in-memory buffering to read rows upfront (capped at 100k) and release DB cursor early"},
				{Name: "mode", Label: "Execution Mode", Type: "select", Mandatory: false, Default: "stream", Options: []string{"stream", "buffer"}, Description: "Loop execution mode: 'stream' (default, O(1) RAM) or 'buffer'"},
				{Name: "var", Label: "Query Variable", Type: "text", Mandatory: false, Description: "Optional variable containing driver query (if not provided in body)"},
				{Name: "language", Label: "Driver Language", Type: "select", Mandatory: false, Default: "sql", Options: []string{"sql"}, Description: "Driver query language runtime (default sql)"},
				{Name: "condition", Label: "Condition Expression", Type: "text", Mandatory: false, Description: "Optional pre-condition expression to skip or execute loop"},
				{Name: "on_error", Label: "On Error Action", Type: "select", Mandatory: false, Default: "stop", Options: []string{"stop", "continue", "retry"}, Description: "Error policy: stop (default), continue, or retry"},
				{Name: "retry_count", Label: "Retry Count", Type: "text", Mandatory: false, Description: "Number of retries for failed child steps"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Loop timeout duration (e.g. 30s, 5m)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Description of the loop"},
			},
			DefaultXML: `<foreach id="ProcessRecords" db="local_sqlite" stream="true">
SELECT id, name FROM raw_items WHERE status = 'pending';
</foreach>`,
		},
		{
			Type:        "while",
			Tag:         "while",
			Name:        "While Loop",
			Category:    "Control Flow",
			Description: "Executes inner steps repeatedly while a condition expression remains true.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Loop ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "condition", Label: "Condition Expression", Type: "text", Mandatory: false, Description: "Loop while this condition is true (e.g. ne .Status \"COMPLETE\")"},
				{Name: "var", Label: "Variable Name", Type: "text", Mandatory: false, Description: "Variable name to monitor (e.g. JobStatus)"},
				{Name: "equals", Label: "Equals Value", Type: "text", Mandatory: false, Description: "Loop while variable equals this value"},
				{Name: "max_iterations", Label: "Max Iterations", Type: "int", Mandatory: false, Default: "1000", Description: "Infinite loop guard ceiling (defaults to 1000)"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Optional timeout duration for entire while loop (e.g. 5m, 30s)"},
				{Name: "on_error", Label: "On Error Action", Type: "select", Mandatory: false, Default: "stop", Options: []string{"stop", "continue", "retry"}, Description: "Error policy: stop (default), continue, or retry"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Description of the while loop"},
			},
			DefaultXML: `<while id="PollStatus" condition="ne .Status \"COMPLETE\"" max_iterations="50"></while>`,
		},
		{
			Type:        "parallel",
			Tag:         "parallel",
			Name:        "Parallel Execution",
			Category:    "Control Flow",
			Description: "Executes child branches or tasks concurrently across multiple goroutines with synchronization.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Parallel Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "max_threads", Label: "Max Threads", Type: "int", Mandatory: false, Default: "4", Description: "Max concurrent worker threads (aliases: threads, concurrency)"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Optional timeout duration for parallel tasks to finish (e.g. 10m)"},
				{Name: "on_error", Label: "On Error Action", Type: "select", Mandatory: false, Default: "stop", Options: []string{"stop", "continue", "retry"}, Description: "Error policy: stop (default), continue, or retry"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Description of parallel block"},
			},
			DefaultXML: `<parallel id="ParallelDownloads" max_threads="4"></parallel>`,
		},

		// 3. Database & SQL Operations
		{
			Type:        "sql",
			Tag:         "sql",
			Name:        "SQL Query / Command",
			Category:    "Database & SQL",
			Description: "Executes a SQL statement or query with variable binding and optional result mapping.",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "Raw SQL query or DDL command. Variable interpolation e.g. {{.TableName}} supported.",
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "db", Label: "Database Connection", Type: "text", Mandatory: true, Description: "Connection name declared in <databases>"},
				{Name: "output_var", Label: "Output Variable", Type: "text", Mandatory: false, Description: "Variable name to capture query result set or affected row count"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Statement execution timeout duration (e.g. 30s, 45m)"},
				{Name: "into", Label: "Into Variable (Alias)", Type: "text", Mandatory: false, Description: "Legacy alias for output_var"},
				{Name: "on_error", Label: "On Error", Type: "select", Mandatory: false, Default: "stop", Options: []string{"stop", "continue", "retry"}, Description: "Failure handling strategy"},
				{Name: "retry_count", Label: "Retry Count", Type: "int", Mandatory: false, Default: "0", Description: "Number of retries on transient connection or statement failure"},
				{Name: "retry_interval", Label: "Retry Interval", Type: "text", Mandatory: false, Default: "1s", Description: "Wait interval between retries (e.g. 1s, 500ms)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Human-readable description of this SQL step"},
			},
			DefaultXML: `<sql id="LoadSummary" db="local_sqlite">
SELECT count(*) FROM raw_records;
</sql>`,
		},
		{
			Type:        "sql_bulk",
			Tag:         "sql_bulk",
			Name:        "SQL Bulk Insert / Copy",
			Category:    "Database & SQL",
			Description: "High-performance bulk stream copy of rows from a source SQL query directly into a destination table.",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "Source extraction SQL query that pipes rows directly into the destination table.",
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "db", Label: "Source Database", Type: "text", Mandatory: true, Description: "Source database connection handle declared in <databases>"},
				{Name: "target_table", Label: "Target Table", Type: "text", Mandatory: true, Description: "Destination table name receiving bulk inserts"},
				{Name: "target_db", Label: "Target Database", Type: "text", Mandatory: false, Description: "Destination database connection handle (defaults to source db)"},
				{Name: "batch_size", Label: "Batch Size", Type: "int", Mandatory: false, Default: "10000", Description: "Number of rows committed per network chunk"},
				{Name: "tablock", Label: "Table Lock (MSSQL)", Type: "bool", Mandatory: false, Default: "true", Options: []string{"true", "false"}, Description: "(SQL Server) Acquires table-level lock for minimal transaction logging"},
				{Name: "check_constraints", Label: "Check Constraints", Type: "bool", Mandatory: false, Default: "false", Options: []string{"true", "false"}, Description: "(SQL Server) Validates foreign keys/check constraints during bulk load"},
				{Name: "fire_triggers", Label: "Fire Triggers", Type: "bool", Mandatory: false, Default: "false", Options: []string{"true", "false"}, Description: "(SQL Server) Fires insert triggers on target table during bulk load"},
				{Name: "keep_nulls", Label: "Keep Nulls", Type: "bool", Mandatory: false, Default: "false", Options: []string{"true", "false"}, Description: "(SQL Server) Preserves incoming NULLs instead of applying default constraints"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Bulk copy operation timeout duration (e.g. 10m, 1h)"},
				{Name: "output_var", Label: "Output Variable", Type: "text", Mandatory: false, Description: "Stores total number of rows streamed to destination"},
				{Name: "on_error", Label: "On Error", Type: "select", Mandatory: false, Default: "stop", Options: []string{"stop", "continue", "retry"}, Description: "Failure handling strategy"},
				{Name: "retry_count", Label: "Retry Count", Type: "int", Mandatory: false, Default: "0", Description: "Number of retries on transient connection failure"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Human-readable description of bulk stream copy"},
			},
			DefaultXML: `<sql_bulk id="BulkInsertRecords" db="source_db" target_db="dest_db" target_table="customers" batch_size="10000" tablock="true">
SELECT id, name, email FROM raw_customers;
</sql_bulk>`,
		},

		// 4. Key-Value & Cache Operations
		{
			Type:        "kv",
			Tag:         "kv",
			Name:        "Key-Value Store",
			Category:    "Key-Value & Cache",
			Description: "Executes atomic key-value operations (get, put/set, delete/del, scan/list) against embedded (bbolt, BadgerDB) or remote (Redis, etcd) databases.",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "Optional inline DSL commands (e.g. SET mykey 'myval' or GET mykey). Overrides op/key/value attributes if provided.",
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for execution tracking"},
				{Name: "db", Label: "KV Connection", Type: "text", Mandatory: true, Description: "Database connection name configured with a KV driver (bbolt, badger, redis, etcd)"},
				{Name: "op", Label: "Operation", Type: "select", Mandatory: false, Default: "get", Options: []string{"get", "put", "delete", "scan"}, Description: "Atomic operation: 'get' (read), 'put' (write), 'delete' (remove), 'scan' (range list)"},
				{Name: "bucket", Label: "Bucket / Keyspace", Type: "text", Mandatory: false, Default: "default", Description: "Bucket name (bbolt/BadgerDB) or key prefix namespace (Redis/etcd)"},
				{Name: "key", Label: "Key", Type: "text", Mandatory: false, Description: "Target record key (supports {{VarName}})"},
				{Name: "value", Label: "Value", Type: "text", Mandatory: false, Description: "Value payload for write operations (supports JSON, strings, {{VarName}})"},
				{Name: "into", Label: "Output Variable", Type: "text", Mandatory: false, Description: "Variable name receiving retrieved value or operation result"},
				{Name: "var", Label: "DSL Variable", Type: "text", Mandatory: false, Description: "Optional environment variable containing dynamic DSL command string"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Operation deadline duration (e.g. 5s, 30s)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Human-readable description of this KV operation"},
				{Name: "on_error", Label: "On Error", Type: "select", Mandatory: false, Default: "stop", Options: []string{"stop", "continue", "retry"}, Description: "Error handling policy if operation fails"},
			},
			DefaultXML: `<kv id="CacheToken" db="kv_store" op="put" bucket="auth" key="jwt" value="{{NewToken}}" />`,
		},
		{
			Type:        "kv_bulk",
			Tag:         "kv_bulk",
			Name:        "KV Bulk Operations",
			Category:    "Key-Value & Cache",
			Description: "Executes batch key-value insertions, range scans, or prefix queries efficiently.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "db", Label: "KV Connection", Type: "text", Mandatory: true, Description: "KV database connection handle"},
				{Name: "bucket", Label: "Bucket", Type: "text", Mandatory: false, Default: "default"},
				{Name: "source", Label: "Source Dataset", Type: "text", Mandatory: true, Description: "Dataset containing key/value map"},
			},
			DefaultXML: `<kv_bulk id="BulkCache" db="kv_store" bucket="lookup" source="{{LookupMap}}" />`,
		},

		// 5. Template & Transformation
		{
			Type:        "template",
			Tag:         "template",
			Name:        "Template Generator",
			Category:    "Template & Transform",
			Description: "Renders text, SQL queries, or configurations using Go text/template with custom pipeline helpers.",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "Template content. Access variables via {{.VarName}} and built-in functions.",
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "into", Label: "Into Variable", Type: "text", Mandatory: false, Description: "Variable name to receive rendered text"},
				{Name: "file", Label: "Output / Source File", Type: "text", Mandatory: false, Description: "File path to write rendered output or load external template"},
				{Name: "name", Label: "Template Name", Type: "text", Mandatory: false, Description: "Optional template identifier during parsing"},
				{Name: "output_var", Label: "Output Variable (Alias)", Type: "text", Mandatory: false, Description: "Alias for into"},
				{Name: "var", Label: "Var (Alias)", Type: "text", Mandatory: false, Description: "Alias for into"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Template rendering timeout duration (e.g. 10s, 1m)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Human-readable description of this template step"},
			},
			DefaultXML: `<template id="RenderConfig" into="GeneratedReport">
Report Date: {{.Date}}
Total Records: {{.RecordCount}}
Status: {{.Status}}
</template>`,
		},
		{
			Type:        "template_html",
			Tag:         "template_html",
			Name:        "HTML Template",
			Category:    "Template & Transform",
			Description: "Safely renders contextual HTML templates with automatic escaping for reports and emails.",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "HTML template body with Go html/template syntax.",
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "into", Label: "Into Variable", Type: "text", Mandatory: false, Description: "Variable name to receive rendered HTML"},
				{Name: "file", Label: "Output HTML File", Type: "text", Mandatory: false, Description: "File path to write rendered HTML directly"},
				{Name: "name", Label: "Template Name", Type: "text", Mandatory: false, Description: "Optional template identifier during parsing"},
				{Name: "output_var", Label: "Output Variable (Alias)", Type: "text", Mandatory: false, Description: "Alias for into"},
				{Name: "var", Label: "Var (Alias)", Type: "text", Mandatory: false, Description: "Alias for into"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Template rendering timeout duration (e.g. 10s, 1m)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Human-readable description of this HTML template step"},
			},
			DefaultXML: `<template_html id="GenerateReportHTML" file="report.html">
<html>
<body>
    <h1>ETL Execution Report</h1>
    <p>Run Time: {{.Timestamp}}</p>
</body>
</html>
</template_html>`,
		},
		{
			Type:        "html_template",
			Tag:         "html_template",
			Name:        "HTML Template (Alias)",
			Category:    "Template & Transform",
			Description: "Safely renders contextual HTML templates with automatic escaping for reports and emails (alias for template_html).",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "HTML template body with Go html/template syntax.",
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "into", Label: "Into Variable", Type: "text", Mandatory: false, Description: "Variable name to receive rendered HTML"},
				{Name: "file", Label: "Output HTML File", Type: "text", Mandatory: false, Description: "File path to write rendered HTML directly"},
				{Name: "name", Label: "Template Name", Type: "text", Mandatory: false, Description: "Optional template identifier during parsing"},
				{Name: "output_var", Label: "Output Variable (Alias)", Type: "text", Mandatory: false, Description: "Alias for into"},
				{Name: "var", Label: "Var (Alias)", Type: "text", Mandatory: false, Description: "Alias for into"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Template rendering timeout duration (e.g. 10s, 1m)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Human-readable description of this HTML template step"},
			},
			DefaultXML: `<html_template id="GenerateReportHTML" file="report.html">
<html>
<body>
    <h1>ETL Execution Report</h1>
    <p>Run Time: {{.Timestamp}}</p>
</body>
</html>
</html_template>`,
		},

		// 6. File & Storage Operations
		{
			Type:        "file_read",
			Tag:         "file_read",
			Name:        "Read File",
			Category:    "File & Storage",
			Description: "Reads file contents from disk into a pipeline variable.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and orchestration"},
				{Name: "path", Label: "File Path", Type: "text", Mandatory: true, Description: "Path to input file on disk (alias file, supports {{Variables}})"},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: true, Description: "Variable name receiving file content (alias output_var)"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Timeout duration (e.g. 10s, 1m)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Step explanation displayed in logs and flowcharts"},
			},
			DefaultXML: `<file_read id="ReadPayload" path="./data/input.json" into="PayloadText" />`,
		},
		{
			Type:        "file_save",
			Tag:         "file_save",
			Name:        "Save File",
			Category:    "File & Storage",
			Description: "Writes variable text, bytes, or template outputs to a file on disk.",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "Inline text or template to write if source variable is not specified.",
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and orchestration"},
				{Name: "path", Label: "File Path", Type: "text", Mandatory: true, Description: "Destination file path on disk (alias file, supports {{Variables}})"},
				{Name: "source", Label: "Source Variable", Type: "text", Mandatory: false, Description: "Variable containing data to write (alias var)"},
				{Name: "mode", Label: "Write Mode", Type: "select", Mandatory: false, Default: "overwrite", Options: []string{"overwrite", "append"}, Description: "File write mode (overwrite or append)"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Timeout duration (e.g. 10s, 1m)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Step explanation displayed in logs and flowcharts"},
			},
			DefaultXML: `<file_save id="SaveOutput" path="./output/result.csv" source="{{CsvContent}}" mode="overwrite" />`,
		},
		{
			Type:        "excel_read",
			Tag:         "excel_read",
			Name:        "Excel Read (.xlsx)",
			Category:    "File & Storage",
			Description: "Parses Excel worksheets into structured pipeline rows and variables.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and orchestration"},
				{Name: "file", Label: "Excel File Path", Type: "text", Mandatory: true, Description: "Path to .xlsx file on disk (alias path, supports {{Variables}})"},
				{Name: "sheet", Label: "Sheet Name", Type: "text", Mandatory: false, Default: "Sheet1", Description: "Target worksheet name to read (defaults to first sheet)"},
				{Name: "header", Label: "Has Header Row", Type: "select", Mandatory: false, Default: "true", Options: []string{"true", "false"}, Description: "Whether first row contains column header keys for JSON objects"},
				{Name: "output_var", Label: "Output Variable", Type: "text", Mandatory: true, Description: "Variable name receiving extracted rows as a JSON array string (alias into)"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Timeout duration (e.g. 30s, 2m)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Step explanation displayed in logs and flowcharts"},
			},
			DefaultXML: `<excel_read id="ReadAccounts" file="./spreadsheets/Accounts.xlsx" sheet="Sheet1" header="true" output_var="AccountList" />`,
		},
		{
			Type:        "excel_write",
			Tag:         "excel_write",
			Name:        "Excel Write (.xlsx)",
			Category:    "File & Storage",
			Description: "Exports SQL query results into styled multi-tab Excel spreadsheets.",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "SQL query to extract rows for the worksheet (e.g. SELECT * FROM customers).",
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and orchestration"},
				{Name: "file", Label: "Target Excel File", Type: "text", Mandatory: true, Description: "Path to write destination .xlsx file (alias path)"},
				{Name: "sheet", Label: "Sheet Name", Type: "text", Mandatory: false, Default: "Sheet1", Description: "Target worksheet name to write (defaults to Sheet1)"},
				{Name: "db", Label: "Database Connection", Type: "text", Mandatory: true, Description: "Database connection name executing source export query"},
				{Name: "var", Label: "Query Variable", Type: "text", Mandatory: false, Description: "Optional variable containing SQL query if not in element body"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Timeout duration (e.g. 1m, 5m)"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Step explanation displayed in logs and flowcharts"},
			},
			DefaultXML: `<excel_write id="ExportSales" file="./reports/Sales.xlsx" sheet="Summary" db="local_sqlite">
SELECT date, product, revenue FROM sales_data;
</excel_write>`,
		},

		// 7. Structured Data Extraction
		{
			Type:        "json_path",
			Tag:         "json_path",
			Name:        "JSON Path Extraction",
			Category:    "Data Extraction",
			Description: "Queries JSON strings or documents using JSONPath expressions.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and orchestration"},
				{Name: "source", Label: "Source JSON Variable", Type: "text", Mandatory: true, Description: "Variable holding JSON string (alias var)"},
				{Name: "query", Label: "JSONPath Query", Type: "text", Mandatory: true, Description: "JSONPath expression (alias jsonpath, e.g. $.items[*].id)"},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: true, Description: "Variable receiving extracted result (alias output_var)"},
				{Name: "mode", Label: "Output Mode", Type: "select", Mandatory: false, Default: "value", Options: []string{"value", "json", "json_array"}, Description: "Output format: value (joined text), json (single object), json_array"},
				{Name: "file", Label: "Source File Path", Type: "text", Mandatory: false, Description: "Optional path to JSON file on disk instead of variable"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Timeout duration"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Step explanation displayed in logs and flowcharts"},
			},
			DefaultXML: `<json_path id="ExtractItems" source="{{ApiResponse}}" query="$.items[*].id" into="ItemIDs" mode="json_array" />`,
		},
		{
			Type:        "yaml_path",
			Tag:         "yaml_path",
			Name:        "YAML Path Extraction",
			Category:    "Data Extraction",
			Description: "Extracts scalar or complex structures from YAML strings and manifests.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and orchestration"},
				{Name: "source", Label: "Source YAML Variable", Type: "text", Mandatory: true, Description: "Variable holding YAML string (alias var)"},
				{Name: "query", Label: "Path Expression", Type: "text", Mandatory: true, Description: "Path query expression (alias path / yamlpath, e.g. spec.replicas)"},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: true, Description: "Variable receiving value (alias output_var)"},
				{Name: "mode", Label: "Output Mode", Type: "select", Mandatory: false, Default: "value", Options: []string{"value", "json", "json_array", "yaml"}, Description: "Output format: value, json, json_array, yaml"},
				{Name: "file", Label: "Source File Path", Type: "text", Mandatory: false, Description: "Optional path to YAML file on disk instead of variable"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Timeout duration"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Step explanation displayed in logs and flowcharts"},
			},
			DefaultXML: `<yaml_path id="ExtractK8sConfig" source="{{ManifestYaml}}" query="spec.replicas" into="ReplicaCount" />`,
		},
		{
			Type:        "xml_xpath",
			Tag:         "xml_xpath",
			Name:        "XML XPath Extraction",
			Category:    "Data Extraction",
			Description: "Evaluates XPath expressions against XML documents.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and orchestration"},
				{Name: "source", Label: "Source XML Variable", Type: "text", Mandatory: true, Description: "Variable holding XML string (alias var)"},
				{Name: "query", Label: "XPath Query", Type: "text", Mandatory: true, Description: "XPath expression (alias xpath, e.g. //book/title/text())"},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: true, Description: "Variable receiving result (alias output_var)"},
				{Name: "mode", Label: "Output Mode", Type: "select", Mandatory: false, Default: "text", Options: []string{"text", "xml", "json_array"}, Description: "Output format: text (joined text), xml (raw tags), json_array"},
				{Name: "file", Label: "Source File Path", Type: "text", Mandatory: false, Description: "Optional path to XML file on disk instead of variable"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Timeout duration"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Step explanation displayed in logs and flowcharts"},
			},
			DefaultXML: `<xml_xpath id="ExtractTitles" source="{{XmlCatalog}}" query="//book/title/text()" into="BookTitles" mode="text" />`,
		},
		{
			Type:        "xml_path",
			Tag:         "xml_path",
			Name:        "XML Path Extraction",
			Category:    "Data Extraction",
			Description: "Evaluates XPath expressions against XML documents (alias for xml_xpath).",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and orchestration"},
				{Name: "source", Label: "Source XML Variable", Type: "text", Mandatory: true, Description: "Variable holding XML string (alias var)"},
				{Name: "query", Label: "XPath Query", Type: "text", Mandatory: true, Description: "XPath expression (alias xpath, e.g. //book/title/text())"},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: true, Description: "Variable receiving result (alias output_var)"},
				{Name: "mode", Label: "Output Mode", Type: "select", Mandatory: false, Default: "text", Options: []string{"text", "xml", "json_array"}, Description: "Output format: text, xml, json_array"},
				{Name: "file", Label: "Source File Path", Type: "text", Mandatory: false, Description: "Optional path to XML file on disk instead of variable"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Timeout duration"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Step explanation displayed in logs and flowcharts"},
			},
			DefaultXML: `<xml_path id="ExtractTitles" source="{{XmlCatalog}}" query="//book/title/text()" into="BookTitles" mode="text" />`,
		},

		// 8. Quality Gates & Assertions
		{
			Type:        "assert",
			Tag:         "assert",
			Name:        "Assertion Check",
			Category:    "Quality & Validation",
			Description: "Enforces data quality rules and halts or warns if an expression fails.",
			Section:     "preflight",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and orchestration"},
				{Name: "var", Label: "Variable to Evaluate", Type: "text", Mandatory: false, Description: "Variable name in pipeline registry to test (e.g. ENV, RECORD_COUNT)"},
				{Name: "equals", Label: "Expected Value", Type: "text", Mandatory: false, Description: "Expected value for string equality comparison (alias value)"},
				{Name: "condition", Label: "Condition Expression", Type: "text", Mandatory: false, Description: "Condition expression (e.g. gt .RecordCount 0, eq .Status \"ready\")"},
				{Name: "message", Label: "Failure Message", Type: "text", Mandatory: false, Description: "Custom error or warning message logged when assertion fails"},
				{Name: "on_failure", Label: "On Failure Action", Type: "select", Mandatory: false, Default: "halt", Options: []string{"halt", "warn", "continue"}, Description: "halt stops execution; warn/continue logs warning and proceeds"},
				{Name: "fail_var", Label: "Failure Flag Variable", Type: "text", Mandatory: false, Description: "Optional variable name to set when assertion fails"},
				{Name: "fail_val", Label: "Failure Flag Value", Type: "text", Mandatory: false, Default: "true", Description: "Value to store in fail_var (defaults to true)"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Timeout duration"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Step explanation displayed in logs and flowcharts"},
			},
			DefaultXML: `<assert id="CheckEnv" var="ENV" equals="production" message="Pipeline must run in production!" on_failure="halt" />`,
		},

		// 9. Integration & Web Services
		{
			Type:        "http_client",
			Tag:         "http_client",
			Name:        "HTTP / REST Client",
			Category:    "Integration & Web",
			Description: "Executes HTTP GET/POST/PUT/DELETE requests with headers and payload handling.",
			Section:     "flow",
			HasContent:  true,
			ContentHelp: "Request body content (for POST/PUT). Supports JSON and variable interpolation.",
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID for logging and orchestration"},
				{Name: "url", Label: "Endpoint URL", Type: "text", Mandatory: true, Description: "Target endpoint URL (supports {{Variables}} & query params)"},
				{Name: "method", Label: "HTTP Method", Type: "select", Mandatory: true, Default: "GET", Options: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}, Description: "HTTP request verb"},
				{Name: "headers", Label: "Request Headers", Type: "text", Mandatory: false, Description: `Custom headers: {"Authorization":"Bearer {{Token}}"} or K1:V1,K2:V2`},
				{Name: "content_type", Label: "Content-Type", Type: "text", Mandatory: false, Description: "Shortcut for Content-Type header (e.g. application/json, text/xml)"},
				{Name: "into", Label: "Response Variable", Type: "text", Mandatory: false, Description: "Variable name receiving the response body (alias output_var)"},
				{Name: "status_into", Label: "Status Code Variable", Type: "text", Mandatory: false, Description: "Variable name receiving HTTP status code (e.g. 200, 404)"},
				{Name: "timeout", Label: "Timeout Duration", Type: "text", Mandatory: false, Description: "Total request timeout duration (e.g. 10s, 30s, 2m)"},
				{Name: "proxy", Label: "HTTP Proxy URL", Type: "text", Mandatory: false, Description: "Proxy URL (e.g. http://proxy.internal:8080)"},
				{Name: "tls_insecure_skip_verify", Label: "Skip TLS Verify", Type: "select", Mandatory: false, Default: "false", Options: []string{"false", "true"}, Description: "Disable SSL/TLS certificate validation checks"},
				{Name: "follow_redirects", Label: "Follow Redirects", Type: "select", Mandatory: false, Default: "true", Options: []string{"true", "false"}, Description: "Follow HTTP 3xx redirect responses automatically"},
				{Name: "description", Label: "Description", Type: "text", Mandatory: false, Description: "Step explanation displayed in logs and flowcharts"},
			},
			DefaultXML: `<http_client id="FetchRemoteData" url="https://api.example.com/data" method="GET" into="ApiResponse" />`,
		},
	}

	categories := []string{
		"Pipeline Setup",
		"Control Flow",
		"Database & SQL",
		"Key-Value & Cache",
		"Template & Transform",
		"File & Storage",
		"Data Extraction",
		"Quality & Validation",
		"Integration & Web",
	}

	return &ComponentCatalog{
		Components: components,
		Categories: categories,
	}
}

// GetComponentByType finds a component in the catalog by its type key.
func (c *ComponentCatalog) GetComponentByType(nodeType string) *ComponentMeta {
	for i := range c.Components {
		if c.Components[i].Type == nodeType {
			return &c.Components[i]
		}
	}
	return nil
}
