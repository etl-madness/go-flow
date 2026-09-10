package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/etl-madness/flow"
	"github.com/etl-madness/go-flow/builder"
	"github.com/traefik/yaegi/interp"
)

const (
	// Table names for pipeline events and runs in the database.
	// if resident schema needs to be other than default schema, set it here.
	// eg if the schema is "custom_schema", you could set:
	// pipeline_events = "custom_schema.pipeline_events"
	// pipeline_runs = "custom_schema.pipeline_runs"
	// --------------------------------------------------------------
	// note: if you change the schema, make sure to update all references
	// to these tables the SQL create table statements.

	pipeline_events = "pipeline_events"
	pipeline_runs   = "pipeline_runs"
)

func main() {
	builderFlag := flag.Bool("builder", false, "Start the local HTMX pipeline builder web server")
	builderPort := flag.Int("builder-port", 8080, "Port for the builder web server")
	builderDb := flag.String("builder-db", "flow_builder.db", "SQLite database file path for the visual builder")
	purgeDb := flag.Bool("purge-db", false, "Purge all data from the SQLite visual builder database and exit")
	importFile := flag.String("import-file", "", "Import a pipeline XML file into the SQLite builder database")
	importName := flag.String("import-name", "", "Custom name for imported pipeline (used with -import-file)")
	optionsPath := flag.String("options", "", "Path to XML file containing CLI option defaults")
	filePath := flag.String("file", "scripts.xml", "Path to XML file containing scripts and databases")
	format := flag.String("format", "csv", "Output format (json,jsonpretty, text, or markdown, csv)")
	xsdPath := flag.String("xsd", "", "Path to XSD file for schema validation (optional)")
	configPath := flag.String("config", "", "Optional path to CONFIG.xml file containing variable overrides")
	validateOnly := flag.Bool("validate", false, "Validate XML schema and structure without executing pipeline")
	preflight := flag.Bool("preflight", false, "Execute preflight validation nodes only without running main pipeline flow")
	varOverrides := flag.String("vars", "", "Comma-separated key=value overrides (e.g. -vars \"TargetTable=override_table,Threshold=500\")")
	debug := flag.Bool("debug", false, "Enable console logging")
	goPath := flag.String("gopath", os.Getenv("GOPATH"), "GOPATH directory for interpreter package imports")
	xsltPath := flag.String("xslt", "", "Path to custom XSLT stylesheet (optional)")
	outFile := flag.String("out", "", "Path to output file for transformed XML (optional)")
	flag.Parse()

	// Handle -purge-db flag: wipes all data from the SQLite visual builder database
	if *purgeDb {
		storage, err := builder.NewStorage(*builderDb)
		if err != nil {
			log.Fatalf("Failed to open builder database %s: %v", *builderDb, err)
		}
		defer storage.Close()

		defaultScript, err := storage.PurgeDatabase()
		if err != nil {
			log.Fatalf("Failed to purge database: %v", err)
		}
		fmt.Printf("Database %s successfully purged. Re-initialized default pipeline %q (ID: %d).\n", *builderDb, defaultScript.Name, defaultScript.ID)
		return
	}

	// Handle -import-file flag: imports a pipeline XML file into SQLite
	if *importFile != "" {
		storage, err := builder.NewStorage(*builderDb)
		if err != nil {
			log.Fatalf("Failed to open builder database %s: %v", *builderDb, err)
		}
		defer storage.Close()

		script, err := builder.ImportPipelineFromXML(*importFile, *importName, storage)
		if err != nil {
			log.Fatalf("Failed to import %s: %v", *importFile, err)
		}
		fmt.Printf("Successfully imported pipeline %q (ID: %d) from %s into %s.\n", script.Name, script.ID, *importFile, *builderDb)
		if !*builderFlag {
			return
		}
	}

	// Handle -builder flag: starts the interactive web UI
	if *builderFlag {
		if err := builder.StartServer(*builderPort, *builderDb); err != nil {
			log.Fatalf("Failed to start builder: %v", err)
		}
		return
	}

	// Apply XML load file options if specified
	if *optionsPath != "" {
		if err := applyXMLOptions(*optionsPath); err != nil {
			outputJSON(&[]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("Error applying load XML: %v", err),
			}})
			os.Exit(1)
		}
	}

	// 1. Optional XSD Validation Pass (runs xmllint if -xsd flag is provided)
	if *xsdPath != "" {
		if err := flow.ValidateXSD(*filePath, *xsdPath); err != nil {
			outputJSON(&[]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: err.Error(),
			}})
			os.Exit(1)
		}
	}

	// 2. Load and Parse XML File
	fileBytes, err := os.ReadFile(*filePath)
	if err != nil {
		outputJSON(&[]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: fmt.Sprintf("Error reading script file: %v", err),
		}})
		os.Exit(1)
	}

	if *xsltPath != "" {
		xsltBytes, err := os.ReadFile(*xsltPath)
		if err != nil {
			outputJSON(&[]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("Error reading XSLT file: %v", err),
			}})
			os.Exit(1)
		}

		// Generate diagram prior to running XSLT transformation
		diagram, err := generateMermaid(fileBytes)
		if err != nil {
			outputJSON(&[]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("Mermaid generation error: %v", err),
			}})
			os.Exit(1)
		}

		fileBytes, err = ProcessXSLT(fileBytes, xsltBytes, diagram, filePath)
		if err != nil {
			outputJSON(&[]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("XSLT processing error: %v", err),
			}})
			os.Exit(1)
		}
		if *outFile != "" {
			os.WriteFile(*outFile, fileBytes, 0644)
		}
		os.Exit(0)
	}

	cfg, err := flow.ParseXMLConfig(fileBytes)
	if err != nil {
		outputJSON(&[]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: fmt.Sprintf("XML parsing error in script file: %v", err),
		}})
		os.Exit(1)
	}

	varConfigs := cfg.Variables
	dbConfigs := cfg.Databases
	preflightNodes := cfg.PreflightNodes
	flowNodes := cfg.FlowNodes

	if *configPath != "" {
		configBytes, err := os.ReadFile(*configPath)
		if err != nil {
			outputJSON(&[]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("Error reading config override file: %v", err),
			}})
			os.Exit(1)
		}

		overrideCfg, err := flow.ParseXMLConfig(configBytes)
		if err != nil {
			outputJSON(&[]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("XML parsing error in config file: %v", err),
			}})
			os.Exit(1)
		}

		varConfigs = append(varConfigs, overrideCfg.Variables...)
		dbConfigs = append(dbConfigs, overrideCfg.Databases...)
		preflightNodes = append(preflightNodes, overrideCfg.PreflightNodes...)
		flowNodes = append(flowNodes, overrideCfg.FlowNodes...)
	}

	// 3. Semantic AST Validation Pass
	if err := flow.ValidateAST(preflightNodes, flowNodes, dbConfigs); err != nil {
		outputJSON(&[]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: err.Error(),
		}})
		os.Exit(1)
	}

	if *validateOnly {
		outputJSON(&[]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    0,
			ResultsString: "XML pipeline schema (XSD) and AST structure are valid.",
		}})
		os.Exit(0)
	}

	// Select execution target node slice
	nodes := flowNodes
	if *preflight {
		nodes = preflightNodes
	}

	// 4. Initialize State and Execute Pipeline
	start := time.Now()
	fmt.Println("Pipeline Start Time:", start.Format("2006-01-02 15:04:05.000"))
	registry := flow.NewRegistry()

	if err := registry.InitVariables(varConfigs); err != nil {
		outputJSON(&[]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: err.Error(),
		}})
		os.Exit(1)
	}

	applyVariableOverrides(registry, *varOverrides)

	if err := registry.InitDatabases(dbConfigs); err != nil {
		outputJSON(&[]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: err.Error(),
		}})
		os.Exit(1)
	}
	defer registry.CloseDatabases()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	executor := flow.NewExecutor(registry)
	executor.SetVerbose(*debug)
	executor.SetGoPath(*goPath)
	executor.SetInterpHook(func(opts *interp.Options) {
		opts.Unrestricted = false
	})

	// =========================================================================
	// DYNAMIC EVENT SINK ASSEMBLY & DATABASE CONNECTION
	// =========================================================================
	var sinks []flow.EventSink
	var logDB *sql.DB
	var driverType string
	var dbSink *DatabaseSink

	// Search dbConfigs in reverse order so -config database overrides take precedence
	var selectedLogDBCfg *flow.DatabaseConfig
	for i := len(dbConfigs) - 1; i >= 0; i-- {
		if dbConfigs[i].Name == "log_db" {
			selectedLogDBCfg = &dbConfigs[i]
			break
		}
	}

	if selectedLogDBCfg != nil {
		driverName := selectedLogDBCfg.Driver

		// Expand {{log_db_cs}} using merged variables from XML, -config, and -vars
		expandedDSN := resolveConnectionString(selectedLogDBCfg.ConnectionString, varConfigs, *varOverrides)

		if driverName == "" {
			driverName = detectDriverFromDSN(expandedDSN)
		}

		if *debug {
			log.Printf("Connecting to log_db with resolved DSN: %s", expandedDSN)
		}

		db, err := sql.Open(driverName, expandedDSN)
		if err != nil {
			log.Printf("Warning: failed to open log_db connection: %v", err)
		} else if err := db.PingContext(ctx); err != nil {
			log.Printf("Warning: failed to ping log_db: %v", err)
			db.Close()
		} else {
			logDB = db
			driverType = detectDriverType(logDB)
			if driverType == "unknown" && driverName != "" {
				driverType = driverName
			}

			if *debug && logDB != nil {
				var currentDB string
				if err := logDB.QueryRowContext(ctx, "SELECT DB_NAME()").Scan(&currentDB); err == nil {
					log.Printf("Connected to 'log_db' [%s] with driver dialect: %s", currentDB, driverType)
				} else {
					log.Printf("Connected to 'log_db' with driver dialect: %s", driverType)
				}
			}

			dbSink = NewDatabaseSink(logDB, driverType)
			sinks = append(sinks, dbSink)
			defer logDB.Close()
		}
	}

	// Attach console streaming sink if output format is stream
	if strings.ToLower(*format) == "stream" {
		fmt.Println("\n\nutc_runtime,run_id,execution_id,sequence,type,kind,id,status,error,row_counts_read,row_counts_written,row_counts_affected")
		sinks = append(sinks, &TextSink{Writer: os.Stdout})
	}

	// Register single or fan-out multi-sink
	if len(sinks) == 1 {
		executor.SetEventSink(sinks[0])
	} else if len(sinks) > 1 {
		executor.SetEventSink(&MultiSink{Sinks: sinks})
	}
	// =========================================================================

	if strings.ToLower(*format) == "stream" {
		results, execErr := executor.ExecuteRun(ctx, nodes)

		if logDB != nil {
			runID := results.RunID
			if runID == "" && dbSink != nil {
				runID = dbSink.RunID()
			}
			if runID == "" {
				runID = generateRunID()
			}
			summary := RunSummaryRecord{
				RunID:        runID,
				FilePath:     *filePath,
				ConfigPath:   *configPath,
				Status:       string(results.Status),
				StartedAt:    results.StartedAt,
				FinishedAt:   results.FinishedAt,
				Duration:     results.FinishedAt.Sub(results.StartedAt),
				TaskCount:    len(results.Nodes),
				ErrorClass:   string(results.ErrorClass),
				ErrorMessage: results.ErrorMessage,
			}
			if err := LogRunSummaryToDB(ctx, logDB, driverType, summary); err != nil {
				log.Printf("Failed to log run summary to database: %v", err)
			}
		}

		if execErr != nil {
			log.Printf("run %s finished with %s: %s", results.RunID, results.ErrorClass, results.ErrorMessage)
			os.Exit(1)
		}

		outputStreamSummary(results, filePath, configPath)

	} else {
		results, execErr := executor.Execute(ctx, nodes)

		// Record run summary if log_db connection exists
		if logDB != nil {
			runID, status, errClass, errMsg := "", "succeeded", "", ""
			if dbSink != nil {
				runID, status, errClass, errMsg = dbSink.RunDetails()
			}
			if execErr != nil {
				if status == "" || status == "succeeded" {
					status = "failed"
				}
				if errClass == "" {
					errClass = "ExecutionError"
				}
				if errMsg == "" {
					errMsg = execErr.Error()
				}
			}
			if runID == "" {
				runID = generateRunID()
			}
			if status == "" {
				status = "succeeded"
			}

			summary := RunSummaryRecord{
				RunID:        runID,
				FilePath:     *filePath,
				ConfigPath:   *configPath,
				Status:       status,
				StartedAt:    start,
				FinishedAt:   time.Now(),
				Duration:     time.Since(start),
				TaskCount:    len(nodes),
				ErrorClass:   errClass,
				ErrorMessage: errMsg,
			}
			if err := LogRunSummaryToDB(ctx, logDB, driverType, summary); err != nil {
				log.Printf("Failed to log run summary to database: %v", err)
			}
		}

		if execErr != nil {
			os.Exit(1)
		}

		switch strings.ToLower(*format) {
		case "markdown", "md", "table":
			outputMarkdownTable(&results)
		case "text":
			outputText(&results)
		case "json":
			outputRawJSON(&results)
		case "jsonpretty", "prettyjson":
			outputJSON(&results)
		case "csv":
			outputCSV(&results)
		default:
			outputText(&results)
		}
	}

	end := time.Now()
	duration := end.Sub(start)

	fmt.Println("Pipeline End Time:  ", end.Format("2006-01-02 15:04:05.000"))
	fmt.Println("Pipeline Duration:  ", duration)
}

// resolveConnectionString expands {{variable_name}} templates using XML variables and CLI overrides
// resolveConnectionString expands {{variable_name}} templates using merged variables
func resolveConnectionString(dsn string, varConfigs []flow.VariableConfig, varOverrides string) string {
	// Build a map of variable key -> value with correct precedence:
	// 1. Base XML variables (scripts.xml)
	// 2. Override XML variables (-config)
	// 3. CLI variable overrides (-vars)
	varsMap := make(map[string]string)

	for _, v := range varConfigs {
		varsMap[v.Name] = v.Value
	}

	if strings.TrimSpace(varOverrides) != "" {
		pairs := strings.Split(varOverrides, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(parts) == 2 {
				varsMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	resolved := dsn

	// Perform iterative replacement (handles nested variable references)
	for i := 0; i < 3; i++ {
		changed := false
		for k, v := range varsMap {
			placeholder := fmt.Sprintf("{{%s}}", k)
			if strings.Contains(resolved, placeholder) {
				resolved = strings.ReplaceAll(resolved, placeholder, v)
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	return resolved
}
