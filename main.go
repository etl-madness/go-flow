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
	"github.com/traefik/yaegi/interp"
)

func main() {
	loadPath := flag.String("load", "", "Path to XML file containing CLI option defaults")
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

	// Apply XML load file options if specified
	if *loadPath != "" {
		if err := applyXMLOptions(*loadPath); err != nil {
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
		opts.Unrestricted = true
	})

	// =========================================================================
	// DYNAMIC EVENT SINK ASSEMBLY & DATABASE CONNECTION
	// =========================================================================
	var sinks []flow.EventSink
	var logDB *sql.DB
	var driverType string

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

			sinks = append(sinks, NewDatabaseSink(logDB, driverType))
			defer logDB.Close()
		}
	}

	// Attach console streaming sink if output format is summary
	if strings.ToLower(*format) == "summary" {
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

	if strings.ToLower(*format) == "summary" {
		results, execErr := executor.ExecuteRun(ctx, nodes)

		if logDB != nil {
			summary := RunSummaryRecord{
				RunID:        results.RunID,
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

		outputSummary(results, filePath, configPath)

	} else {
		results, execErr := executor.Execute(ctx, nodes)

		// Record run summary if log_db connection exists
		if logDB != nil {
			status := "SUCCESS"
			errClass := ""
			errMsg := ""
			if execErr != nil {
				status = "FAILED"
				errClass = "ExecutionError"
				errMsg = execErr.Error()
			}

			summary := RunSummaryRecord{
				RunID:        "",
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
			outputCSV(&results)
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