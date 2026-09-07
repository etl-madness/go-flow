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
				{Name: "driver", Label: "Driver", Type: "select", Mandatory: true, Default: "sqlite", Options: []string{"sqlite", "sqlserver", "postgres", "mysql", "oracle", "bbolt", "badger", "redis", "etcd"}},
				{Name: "connection_string", Label: "Connection String / DSN", Type: "text", Mandatory: true, Description: "Driver-specific connection string or path. Supports {{Variables}}"},
				{Name: "max_open_conns", Label: "Max Open Conns", Type: "int", Mandatory: false, Default: "25"},
				{Name: "max_idle_conns", Label: "Max Idle Conns", Type: "int", Mandatory: false, Default: "10"},
				{Name: "conn_max_lifetime_seconds", Label: "Max Lifetime (sec)", Type: "text", Mandatory: false, Default: "300"},
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
			Type:        "group",
			Tag:         "group",
			Name:        "Execution Group",
			Category:    "Control Flow",
			Description: "Groups pipeline steps together with independent retry, transaction boundaries, and error policies.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Group ID", Type: "text", Mandatory: true, Description: "Unique group step identifier"},
				{Name: "transaction", Label: "Transactional", Type: "bool", Mandatory: false, Default: "false", Options: []string{"true", "false"}},
				{Name: "on_error", Label: "On Error", Type: "select", Mandatory: false, Default: "stop", Options: []string{"stop", "continue", "retry"}},
				{Name: "retry_count", Label: "Retry Count", Type: "int", Mandatory: false, Default: "0"},
			},
			DefaultXML: `<group id="TransformBatch" transaction="false" on_error="stop"></group>`,
		},
		{
			Type:        "if",
			Tag:         "if",
			Name:        "Conditional Branch",
			Category:    "Control Flow",
			Description: "Evaluates an expression to execute steps when condition evaluates to true.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Condition ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "condition", Label: "Condition Expression", Type: "text", Mandatory: true, Description: "Go template or boolean expression, e.g. eq .Environment \"prod\""},
			},
			DefaultXML: `<if id="CheckEnv" condition="eq .Environment \"prod\""></if>`,
		},
		{
			Type:        "foreach",
			Tag:         "foreach",
			Name:        "Foreach Loop",
			Category:    "Control Flow",
			Description: "Iterates over a dataset, CSV list, or slice variable, exposing each item inside the loop.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Loop ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "collection", Label: "Collection Variable", Type: "text", Mandatory: true, Description: "Variable holding collection or comma-separated string"},
				{Name: "item", Label: "Current Item Variable", Type: "text", Mandatory: true, Default: "item", Description: "Variable name assigned to current element"},
				{Name: "parallel", Label: "Parallel", Type: "bool", Mandatory: false, Default: "false", Options: []string{"true", "false"}},
			},
			DefaultXML: `<foreach id="ProcessFiles" collection="{{FileList}}" item="currentFile"></foreach>`,
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
				{Name: "condition", Label: "Condition Expression", Type: "text", Mandatory: true, Description: "Loop while this condition is true"},
				{Name: "max_iterations", Label: "Max Iterations", Type: "int", Mandatory: false, Default: "100"},
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
				{Name: "max_concurrency", Label: "Max Concurrency", Type: "int", Mandatory: false, Default: "4"},
			},
			DefaultXML: `<parallel id="ParallelDownloads" max_concurrency="4"></parallel>`,
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
				{Name: "db", Label: "Target Database", Type: "text", Mandatory: true, Description: "Connection name declared in <databases>"},
				{Name: "action", Label: "Action", Type: "select", Mandatory: false, Default: "exec", Options: []string{"exec", "query", "query_row"}},
				{Name: "into", Label: "Into Variable", Type: "text", Mandatory: false, Description: "Variable name to store query result"},
			},
			DefaultXML: `<sql id="LoadSummary" db="local_sqlite" action="exec">
SELECT count(*) FROM raw_records;
</sql>`,
		},
		{
			Type:        "sql_bulk",
			Tag:         "sql_bulk",
			Name:        "SQL Bulk Insert / Copy",
			Category:    "Database & SQL",
			Description: "High-performance bulk stream copy of rows from memory or file into target database tables.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "db", Label: "Target Database", Type: "text", Mandatory: true, Description: "Connection name declared in <databases>"},
				{Name: "table", Label: "Target Table", Type: "text", Mandatory: true, Description: "Target destination table name"},
				{Name: "source", Label: "Source Dataset / File", Type: "text", Mandatory: true, Description: "Source dataset variable or file path"},
				{Name: "batch_size", Label: "Batch Size", Type: "int", Mandatory: false, Default: "5000"},
			},
			DefaultXML: `<sql_bulk id="BulkInsertRecords" db="local_sqlite" table="customers" source="{{CustomerData}}" batch_size="5000" />`,
		},

		// 4. Key-Value & Cache Operations
		{
			Type:        "kv",
			Tag:         "kv",
			Name:        "Key-Value Store",
			Category:    "Key-Value & Cache",
			Description: "Gets, sets, or deletes key-value pairs in embedded (BBolt, Badger) or remote (Redis, etcd) stores.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "db", Label: "KV Connection", Type: "text", Mandatory: true, Description: "KV database connection handle"},
				{Name: "action", Label: "Action", Type: "select", Mandatory: true, Default: "get", Options: []string{"get", "set", "delete", "exists"}},
				{Name: "bucket", Label: "Bucket / Keyspace", Type: "text", Mandatory: false, Default: "default"},
				{Name: "key", Label: "Key", Type: "text", Mandatory: true, Description: "Target key name"},
				{Name: "value", Label: "Value", Type: "text", Mandatory: false, Description: "Value to write (for set action)"},
				{Name: "into", Label: "Store Result Into", Type: "text", Mandatory: false, Description: "Variable name to capture retrieved value"},
			},
			DefaultXML: `<kv id="CacheToken" db="kv_store" action="set" bucket="auth" key="jwt" value="{{NewToken}}" />`,
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
				{Name: "action", Label: "Action", Type: "select", Mandatory: true, Default: "batch_set", Options: []string{"batch_set", "scan_prefix"}},
				{Name: "bucket", Label: "Bucket", Type: "text", Mandatory: false, Default: "default"},
				{Name: "source", Label: "Source Dataset", Type: "text", Mandatory: true, Description: "Dataset containing key/value map"},
			},
			DefaultXML: `<kv_bulk id="BulkCache" db="kv_store" action="batch_set" bucket="lookup" source="{{LookupMap}}" />`,
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
				{Name: "file", Label: "Output File", Type: "text", Mandatory: false, Description: "File path to save rendered template directly"},
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
				{Name: "file", Label: "Output HTML File", Type: "text", Mandatory: false, Description: "File path for HTML output"},
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
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "path", Label: "File Path", Type: "text", Mandatory: true, Description: "Path to input file. Supports {{Variables}}"},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: true, Description: "Variable to receive file content"},
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
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "path", Label: "File Path", Type: "text", Mandatory: true, Description: "Path to destination file"},
				{Name: "source", Label: "Source Variable", Type: "text", Mandatory: true, Description: "Variable containing data to write"},
				{Name: "mode", Label: "Write Mode", Type: "select", Mandatory: false, Default: "overwrite", Options: []string{"overwrite", "append"}},
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
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "path", Label: "Excel Path", Type: "text", Mandatory: true, Description: "Path to .xlsx file"},
				{Name: "sheet", Label: "Sheet Name", Type: "text", Mandatory: false, Default: "Sheet1"},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: true, Description: "Variable to store extracted dataset"},
			},
			DefaultXML: `<excel_read id="ReadAccounts" path="./spreadsheets/Accounts.xlsx" sheet="Active" into="AccountList" />`,
		},
		{
			Type:        "excel_write",
			Tag:         "excel_write",
			Name:        "Excel Write (.xlsx)",
			Category:    "File & Storage",
			Description: "Exports tabular dataset variables into styled Excel spreadsheets.",
			Section:     "flow",
			HasContent:  false,
			Fields: []ComponentField{
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "path", Label: "Target Excel Path", Type: "text", Mandatory: true, Description: "Path to write .xlsx file"},
				{Name: "source", Label: "Source Variable", Type: "text", Mandatory: true, Description: "Dataset variable to export"},
				{Name: "sheet", Label: "Sheet Name", Type: "text", Mandatory: false, Default: "Sheet1"},
			},
			DefaultXML: `<excel_write id="WriteSummary" path="./output/Summary.xlsx" source="{{CustomerData}}" sheet="Summary" />`,
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
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "source", Label: "Source JSON Variable", Type: "text", Mandatory: true, Description: "Variable holding JSON string"},
				{Name: "query", Label: "JSONPath Query", Type: "text", Mandatory: true, Description: "e.g. $.items[*].id"},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: true, Description: "Variable to store query result"},
			},
			DefaultXML: `<json_path id="ExtractItems" source="{{ApiResponse}}" query="$.items[*].id" into="ItemIDs" />`,
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
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "source", Label: "Source YAML Variable", Type: "text", Mandatory: true, Description: "Variable holding YAML"},
				{Name: "query", Label: "Path Expression", Type: "text", Mandatory: true, Description: "Path expression"},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: true, Description: "Variable to receive value"},
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
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "source", Label: "Source XML Variable", Type: "text", Mandatory: true, Description: "Variable holding XML string"},
				{Name: "query", Label: "XPath Query", Type: "text", Mandatory: true, Description: "e.g. //book/title/text()"},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: true, Description: "Variable to receive result"},
			},
			DefaultXML: `<xml_xpath id="ExtractTitles" source="{{XmlCatalog}}" query="//book/title/text()" into="BookTitles" />`,
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
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "condition", Label: "Condition Expression", Type: "text", Mandatory: true, Description: "Must evaluate to true, e.g. gt .RecordCount 0"},
				{Name: "message", Label: "Failure Message", Type: "text", Mandatory: false, Description: "Error message when assertion fails"},
			},
			DefaultXML: `<assert id="CheckRecordCount" condition="gt .RecordCount 0" message="Record count must be greater than zero!" />`,
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
				{Name: "id", Label: "Step ID", Type: "text", Mandatory: true, Description: "Unique step ID"},
				{Name: "url", Label: "Endpoint URL", Type: "text", Mandatory: true, Description: "URL endpoint. Supports {{Variables}}"},
				{Name: "method", Label: "HTTP Method", Type: "select", Mandatory: true, Default: "GET", Options: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"}},
				{Name: "headers", Label: "Headers (JSON)", Type: "text", Mandatory: false, Description: `e.g. {"Authorization":"Bearer {{Token}}"}`},
				{Name: "into", Label: "Target Variable", Type: "text", Mandatory: false, Description: "Variable to capture response body"},
				{Name: "status_into", Label: "Status Code Variable", Type: "text", Mandatory: false, Description: "Variable to capture HTTP status code"},
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