package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/etl-madness/flow"
	"github.com/etl-madness/go-flow/builder"
	"github.com/etl-madness/go-flow/pkg/flowcrypto"
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

	pipeline_events = builder.TablePipelineEvents
	pipeline_runs   = builder.TablePipelineRuns
)

func runtimeIdentity() (string, string) {
	userName := ""
	if currentUser, err := user.Current(); err == nil && currentUser != nil {
		userName = currentUser.Username
	}
	if userName == "" {
		userName = os.Getenv("USER")
	}
	if userName == "" {
		userName = os.Getenv("USERNAME")
	}

	hostname := fqdnHostname()
	if hostname == "" {
		hostname = os.Getenv("HOSTNAME")
	}
	if hostname == "" {
		hostname = os.Getenv("COMPUTERNAME")
	}
	return userName, hostname
}

func configureExecutorOptionsPath(executor *flow.Executor, source string) {
	if executor == nil || strings.TrimSpace(source) == "" {
		return
	}
	executor.SetOptionsPath(source)
}

func fqdnHostname() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return ""
	}
	if strings.Contains(hostname, ".") {
		return strings.TrimSuffix(hostname, ".")
	}
	if hostnames, err := net.LookupCNAME(hostname); err == nil && hostnames != "" {
		return strings.TrimSuffix(hostnames, ".")
	}
	if ip, err := net.LookupIP(hostname); err == nil && len(ip) > 0 {
		for _, value := range ip {
			if names, err := net.LookupAddr(value.String()); err == nil && len(names) > 0 {
				return strings.TrimSuffix(names[0], ".")
			}
		}
	}
	return strings.TrimSuffix(hostname, ".")
}

func resolveSecureKey(cliKey string) string {
	if strings.TrimSpace(cliKey) != "" {
		return strings.TrimSpace(cliKey)
	}
	if envKey := strings.TrimSpace(os.Getenv("FLOW_SECURE_KEY")); envKey != "" {
		return envKey
	}
	if envKey := strings.TrimSpace(os.Getenv("SECURE_KEY")); envKey != "" {
		return envKey
	}
	return ""
}

func exportFromDatabase(dsn, itemType, name string) (string, error) {
	driver := "sqlserver"
	cleanDSN := dsn
	if strings.HasPrefix(dsn, "sql://") || strings.HasPrefix(dsn, "db://") {
		raw := strings.TrimPrefix(strings.TrimPrefix(dsn, "sql://"), "db://")
		parts := strings.SplitN(raw, "@", 2)
		if len(parts) == 2 {
			driver = strings.ToLower(strings.TrimSpace(parts[0]))
			cleanDSN = parts[1]
		}
	} else if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		driver = "postgres"
	} else if strings.HasPrefix(dsn, "mysql://") {
		driver = "mysql"
	}

	db, err := sql.Open(driver, cleanDSN)
	if err != nil {
		return "", fmt.Errorf("failed connecting to database: %w", err)
	}
	defer db.Close()

	var table, col string
	switch strings.ToLower(strings.TrimSpace(itemType)) {
	case "options":
		table, col = "dbo.flow_options_content", "OptionsXML"
	case "config":
		table, col = "dbo.flow_config_content", "ConfigXML"
	case "pipeline", "":
		table, col = "dbo.flow_pipeline_content", "PipelineXML"
	default:
		return "", fmt.Errorf("unsupported item type %q: expected pipeline, options, or config", itemType)
	}

	if driver != "sqlserver" {
		table = strings.TrimPrefix(table, "dbo.")
	}

	query := fmt.Sprintf("SELECT %s FROM %s WHERE Name = @p1", col, table)
	if driver == "postgres" {
		query = fmt.Sprintf("SELECT %s FROM %s WHERE Name = $1", col, table)
	} else if driver == "mysql" || driver == "sqlite" {
		query = fmt.Sprintf("SELECT %s FROM %s WHERE Name = ?", col, table)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var content string
	err = db.QueryRowContext(ctx, query, name).Scan(&content)
	if err != nil {
		return "", fmt.Errorf("failed querying %s for %q: %w", table, name, err)
	}
	return content, nil
}

func importToDatabase(dsn, itemType, name, desc, content string) error {
	driver := "sqlserver"
	cleanDSN := dsn
	if strings.HasPrefix(dsn, "sql://") || strings.HasPrefix(dsn, "db://") {
		raw := strings.TrimPrefix(strings.TrimPrefix(dsn, "sql://"), "db://")
		parts := strings.SplitN(raw, "@", 2)
		if len(parts) == 2 {
			driver = strings.ToLower(strings.TrimSpace(parts[0]))
			cleanDSN = parts[1]
		}
	} else if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		driver = "postgres"
	} else if strings.HasPrefix(dsn, "mysql://") {
		driver = "mysql"
	}

	db, err := sql.Open(driver, cleanDSN)
	if err != nil {
		return fmt.Errorf("failed connecting to database: %w", err)
	}
	defer db.Close()

	var table, col string
	switch strings.ToLower(strings.TrimSpace(itemType)) {
	case "options":
		table, col = "dbo.flow_options_content", "OptionsXML"
	case "config":
		table, col = "dbo.flow_config_content", "ConfigXML"
	default:
		table, col = "dbo.flow_pipeline_content", "PipelineXML"
	}

	if driver != "sqlserver" {
		table = strings.TrimPrefix(table, "dbo.")
	}

	var query string
	switch driver {
	case "sqlserver":
		query = fmt.Sprintf(`
			MERGE INTO %s AS target
			USING (SELECT @p1 AS Name) AS source
			ON (target.Name = source.Name)
			WHEN MATCHED THEN
				UPDATE SET Description = @p2, %s = @p3
			WHEN NOT MATCHED THEN
				INSERT (Name, Description, %s) VALUES (@p1, @p2, @p3);
		`, table, col, col)
	case "postgres":
		query = fmt.Sprintf(`
			INSERT INTO %s (Name, Description, %s) VALUES ($1, $2, $3)
			ON CONFLICT (Name) DO UPDATE SET Description = EXCLUDED.Description, %s = EXCLUDED.%s;
		`, table, col, col, col)
	case "mysql":
		query = fmt.Sprintf(`
			INSERT INTO %s (Name, Description, %s) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE Description = VALUES(Description), %s = VALUES(%s);
		`, table, col, col, col)
	case "sqlite":
		query = fmt.Sprintf(`
			INSERT INTO %s (Name, Description, %s) VALUES (?, ?, ?)
			ON CONFLICT(Name) DO UPDATE SET Description = excluded.Description, %s = excluded.%s;
		`, table, col, col, col)
	default:
		return fmt.Errorf("unsupported database driver %q for import; supported drivers are sqlserver, postgres, mysql, sqlite", driver)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = db.ExecContext(ctx, query, name, desc, content)
	return err
}

func main() {
	builderFlag := flag.Bool("builder", false, "Start the local HTMX pipeline builder web server")
	builderPort := flag.Int("builder-port", 0, "Port for the builder web server (default 0 for dynamic ephemeral port)")
	builderDb := flag.String("builder-db", "flow_builder.db", "SQLite database file path for the visual builder")
	purgeDb := flag.Bool("purge-db", false, "Purge all data from the SQLite visual builder database and exit")
	importFile := flag.String("import-file", "", "Import a pipeline, options, or config XML file into SQLite builder database or external DB")
	importName := flag.String("import-name", "", "Custom name for imported item (used with -import-file)")
	importType := flag.String("import-type", "pipeline", "Item type for -import-file: pipeline, options, or config")
	exportFile := flag.String("export-file", "", "Path to export a pipeline, options, or config file")
	exportName := flag.String("export-name", "", "Name of the pipeline, options, or config to export")
	exportType := flag.String("export-type", "pipeline", "Type of item to export: pipeline, options, or config")
	dbDsn := flag.String("dsn", "", "Optional external database DSN for import/export directly to external DB")
	encryptedFlag := flag.Bool("encrypted", false, "Indicates that config, options, or script content is encrypted and must be decrypted, or encrypts exports")
	secureKeyFlag := flag.String("secure-key", "", "Key or passphrase for decryption/encryption (falls back to FLOW_SECURE_KEY or SECURE_KEY env var)")
	verifySigFlag := flag.Bool("verify-signature", false, "Verify digital signature before decrypting or executing pipeline")
	pubKeyFlag := flag.String("public-key", "", "Public key file path for digital signature verification (RSA, ECDSA, Ed25519)")
	certFlag := flag.String("cert", "", "X.509 certificate file path for digital signature verification")
	caCertFlag := flag.String("ca-cert", "", "CA certificate file path for signature verification chain")
	keyringFlag := flag.String("keyring", "", "OpenPGP public keyring file path for signature verification")
	sigFileFlag := flag.String("signature", "", "Detached digital signature file path")

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

	cliSetFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		cliSetFlags[f.Name] = true
	})

	var effectiveKey string
	if cliSetFlags["secure-key"] {
		effectiveKey = *secureKeyFlag
	} else {
		effectiveKey = resolveSecureKey("")
	}

	secOpts := SecurityOptions{
		Encrypted:       *encryptedFlag,
		SecureKey:       effectiveKey,
		VerifySignature: *verifySigFlag,
		PublicKeyPath:   *pubKeyFlag,
		CertPath:        *certFlag,
		CACertPath:      *caCertFlag,
		KeyringPath:     *keyringFlag,
		SignaturePath:   *sigFileFlag,
	}

	// Apply XML load file options if specified
	if *optionsPath != "" {
		optSecOpts := secOpts
		optSecOpts.SignaturePath = "" // Explicit -signature flag applies to pipeline file
		if err := applyXMLOptionsVerified(*optionsPath, optSecOpts); err != nil {
			outputJSON(&[]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("Error applying load XML: %v", err),
			}})
			os.Exit(1)
		}
		// Refresh key and security options according to CLI > XML > Environment precedence
		if !cliSetFlags["secure-key"] {
			if strings.TrimSpace(*secureKeyFlag) != "" {
				effectiveKey = strings.TrimSpace(*secureKeyFlag)
			} else {
				effectiveKey = resolveSecureKey("")
			}
		} else {
			effectiveKey = *secureKeyFlag
		}
		secOpts.Encrypted = *encryptedFlag
		secOpts.SecureKey = effectiveKey
		secOpts.VerifySignature = *verifySigFlag
		secOpts.PublicKeyPath = *pubKeyFlag
		secOpts.CertPath = *certFlag
		secOpts.CACertPath = *caCertFlag
		secOpts.KeyringPath = *keyringFlag
		secOpts.SignaturePath = *sigFileFlag
	}

	// Allow environment variable override for builder port if not explicitly set on CLI
	if !cliSetFlags["builder-port"] {
		if envPort := os.Getenv("FLOW_BUILDER_PORT"); envPort != "" {
			if p, err := strconv.Atoi(envPort); err == nil && p >= 0 {
				*builderPort = p
			}
		} else if envPort := os.Getenv("PORT"); envPort != "" {
			if p, err := strconv.Atoi(envPort); err == nil && p >= 0 {
				*builderPort = p
			}
		}
	}

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

	// Handle -export-file flag: exports a pipeline, options, or config
	if *exportFile != "" {
		if *exportName == "" {
			log.Fatalf("Error: -export-name must be specified when using -export-file")
		}

		var rawContent string
		var err error

		if *dbDsn != "" {
			rawContent, err = exportFromDatabase(*dbDsn, *exportType, *exportName)
			if err != nil {
				log.Fatalf("Export from database failed: %v", err)
			}
		} else {
			storage, err := builder.NewStorage(*builderDb)
			if err != nil {
				log.Fatalf("Failed to open builder database %s: %v", *builderDb, err)
			}
			defer storage.Close()

			switch strings.ToLower(strings.TrimSpace(*exportType)) {
			case "options":
				rawContent, err = storage.GetOptionsFile(*exportName)
			case "config":
				rawContent, err = storage.GetConfigFile(*exportName)
			case "pipeline", "":
				rawContent, err = storage.ExportPipelineXML(*exportName)
			default:
				log.Fatalf("Unsupported -export-type: %q (expected pipeline, options, or config)", *exportType)
			}
			if err != nil {
				log.Fatalf("Failed exporting %s %q from %s: %v", *exportType, *exportName, *builderDb, err)
			}
		}

		var finalBytes []byte
		if *encryptedFlag {
			if effectiveKey == "" {
				log.Fatalf("Error: -encrypted was specified for export, but no key was provided. Specify -secure-key or set FLOW_SECURE_KEY.")
			}
			armored, err := flowcrypto.EncryptArmored([]byte(rawContent), effectiveKey)
			if err != nil {
				log.Fatalf("Failed encrypting exported content: %v", err)
			}
			finalBytes = []byte(armored)
		} else {
			finalBytes = []byte(rawContent)
		}

		if *exportFile == "-" {
			_, err = os.Stdout.Write(finalBytes)
		} else {
			err = os.WriteFile(*exportFile, finalBytes, 0644)
			if err == nil {
				fmt.Printf("Successfully exported %s %q to %s (encrypted: %t).\n", *exportType, *exportName, *exportFile, *encryptedFlag)
			}
		}
		if err != nil {
			log.Fatalf("Failed writing export file %s: %v", *exportFile, err)
		}
		return
	}

	// Handle -import-file flag: imports a pipeline, options, or config XML file into SQLite or external DB
	if *importFile != "" {
		importSecOpts := secOpts
		importSecOpts.SignaturePath = "" // Explicit -signature flag applies to pipeline file
		// Do not require source input to be encrypted; auto-detection will still decrypt if it is encrypted
		importSecOpts.Encrypted = false
		plainData, verRes, err := LoadResourceVerified(context.Background(), *importFile, importSecOpts)
		if err != nil {
			log.Fatalf("Failed loading import file %s: %v", *importFile, err)
		}
		if verRes != nil && verRes.Valid && *debug {
			log.Printf("Import file digital signature verified: %s via %s (signer: %s)", verRes.Format, verRes.Algorithm, verRes.SignerInfo)
		}

		targetName := strings.TrimSpace(*importName)
		if targetName == "" {
			targetName = strings.TrimSuffix(filepath.Base(*importFile), filepath.Ext(*importFile))
		}

		if *dbDsn != "" {
			contentToStore := string(plainData)
			if *encryptedFlag {
				if effectiveKey == "" {
					log.Fatalf("Error: -encrypted was specified for database import, but no key was provided. Specify -secure-key or set FLOW_SECURE_KEY.")
				}
				contentToStore, err = flowcrypto.EncryptArmored(plainData, effectiveKey)
				if err != nil {
					log.Fatalf("Failed encrypting for database import: %v", err)
				}
			}
			err = importToDatabase(*dbDsn, *importType, targetName, "Imported via go-flow", contentToStore)
			if err != nil {
				log.Fatalf("Failed importing into database: %v", err)
			}
			fmt.Printf("Successfully imported %s %q into database from %s (encrypted: %t).\n", *importType, targetName, *importFile, *encryptedFlag)
			if !*builderFlag {
				return
			}
		} else {
			storage, err := builder.NewStorage(*builderDb)
			if err != nil {
				log.Fatalf("Failed to open builder database %s: %v", *builderDb, err)
			}
			defer storage.Close()

			switch strings.ToLower(strings.TrimSpace(*importType)) {
			case "options":
				if err := storage.SaveOptionsFile(targetName, string(plainData)); err != nil {
					log.Fatalf("Failed saving options file %q: %v", targetName, err)
				}
				fmt.Printf("Successfully imported options file %q from %s into %s.\n", targetName, *importFile, *builderDb)
			case "config":
				if err := storage.SaveConfigFile(targetName, string(plainData)); err != nil {
					log.Fatalf("Failed saving config file %q: %v", targetName, err)
				}
				fmt.Printf("Successfully imported config file %q from %s into %s.\n", targetName, *importFile, *builderDb)
			case "pipeline", "":
				script, err := builder.ImportPipelineFromBytes(plainData, targetName, *importName, *importFile, storage)
				if err != nil {
					log.Fatalf("Failed to import %s: %v", *importFile, err)
				}
				fmt.Printf("Successfully imported pipeline %q (ID: %d) from %s into %s.\n", script.Name, script.ID, *importFile, *builderDb)
			default:
				log.Fatalf("Unsupported -import-type: %q (expected pipeline, options, or config)", *importType)
			}

			if !*builderFlag {
				return
			}
		}
	}

	// Handle -builder flag: starts the interactive web UI
	if *builderFlag {
		builder.SetTelemetryTables(pipeline_runs, pipeline_events)
		if err := builder.StartServer(*builderPort, *builderDb); err != nil {
			log.Fatalf("Failed to start builder: %v", err)
		}
		return
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
	ctx := context.Background()
	fileBytes, verRes, err := LoadResourceVerified(ctx, *filePath, secOpts)
	if err != nil {
		outputJSON(&[]flow.ScriptResult{{
			ScriptID:      "system",
			ReturnCode:    1,
			ResultsString: fmt.Sprintf("Error reading script file: %v", err),
		}})
		os.Exit(1)
	}
	if verRes != nil && verRes.Valid && *debug {
		log.Printf("Digital signature verified successfully: %s via %s (signer: %s)", verRes.Format, verRes.Algorithm, verRes.SignerInfo)
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
		cfgSecOpts := secOpts
		cfgSecOpts.SignaturePath = "" // Explicit -signature flag applies to pipeline file
		configBytes, cfgVerRes, err := LoadResourceVerified(ctx, *configPath, cfgSecOpts)
		if err != nil {
			outputJSON(&[]flow.ScriptResult{{
				ScriptID:      "system",
				ReturnCode:    1,
				ResultsString: fmt.Sprintf("Error reading config override file: %v", err),
			}})
			os.Exit(1)
		}
		if cfgVerRes != nil && cfgVerRes.Valid && *debug {
			log.Printf("Config digital signature verified: %s via %s (signer: %s)", cfgVerRes.Format, cfgVerRes.Algorithm, cfgVerRes.SignerInfo)
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
	start := time.Now().UTC()
	fmt.Println("Pipeline Start Time:", start.Format("2006-01-02 15:04:05.000 UTC"))
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
	configureExecutorOptionsPath(executor, resourceSourceLabel(*optionsPath))
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
			dbSink.debug = *debug
			sinks = append(sinks, dbSink)
			defer logDB.Close()
		}
	}

	// Attach console streaming sink if output format is stream
	if strings.ToLower(*format) == "stream" {
		fmt.Println("\n\nutc_runtime,run_id,execution_id,sequence,type,kind,id,status,user_name,hostname,options_path,error,row_counts_read,row_counts_written,row_counts_affected")
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
			if results.UserName == "" || results.Hostname == "" {
				userName, hostname := runtimeIdentity()
				if results.UserName == "" {
					results.UserName = userName
				}
				if results.Hostname == "" {
					results.Hostname = hostname
				}
			}
			summary := RunSummaryRecord{
				RunID:        runID,
				FilePath:     resourceSourceLabel(*filePath),
				ConfigPath:   resourceSourceLabel(*configPath),
				Status:       string(results.Status),
				StartedAt:    results.StartedAt,
				FinishedAt:   results.FinishedAt,
				Duration:     results.FinishedAt.Sub(results.StartedAt),
				TaskCount:    len(results.Nodes),
				UserName:     results.UserName,
				Hostname:     results.Hostname,
				OptionsPath:  results.OptionsPath,
				ErrorClass:   string(results.ErrorClass),
				ErrorMessage: results.ErrorMessage,
			}
			if err := LogRunSummaryToDB(ctx, logDB, driverType, summary, *debug); err != nil {
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
			userName, hostname := runtimeIdentity()

			summary := RunSummaryRecord{
				RunID:        runID,
				FilePath:     resourceSourceLabel(*filePath),
				ConfigPath:   resourceSourceLabel(*configPath),
				Status:       status,
				StartedAt:    start,
				FinishedAt:   time.Now().UTC(),
				Duration:     time.Now().UTC().Sub(start),
				TaskCount:    len(nodes),
				UserName:     userName,
				Hostname:     hostname,
				OptionsPath:  *optionsPath,
				ErrorClass:   errClass,
				ErrorMessage: errMsg,
			}
			if err := LogRunSummaryToDB(ctx, logDB, driverType, summary, *debug); err != nil {
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

	end := time.Now().UTC()
	duration := end.Sub(start)

	fmt.Println("Pipeline End Time:  ", end.Format("2006-01-02 15:04:05.000 UTC"))
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
