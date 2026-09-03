package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/etl-madness/flow"
	"github.com/google/uuid"
	"github.com/traefik/yaegi/interp"
)

func main() {

	executionID := uuid.New()
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
	results, execErr := executor.Execute(ctx, nodes)
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
		outputCSV(executionID, &results)
	default:
		outputCSV(executionID, &results)
	}

	end := time.Now()
	duration := end.Sub(start)

	fmt.Println("Pipeline End Time:  ", end.Format("2006-01-02 15:04:05.000"))
	fmt.Println("Pipeline Duration:  ", duration)
	if execErr != nil {
		os.Exit(1)
	}
}
