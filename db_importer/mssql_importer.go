package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

func main() {
	dsn := flag.String("dsn", "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true", "SQL Server connection string")
	filePath := flag.String("file", "github_ai_credit_usage.xml", "Path to XML file to import")
	name := flag.String("name", "github_ai_credit_usage", "Name key in database")
	table := flag.String("table", "pipeline", "Target table type: pipeline, options, or config")
	desc := flag.String("desc", "Imported via Go importer", "Description of the XML content")
	flag.Parse()

	// 1. Read raw file bytes (no quote escaping needed)
	xmlBytes, err := os.ReadFile(*filePath)
	if err != nil {
		log.Fatalf("Failed to read file %s: %v", *filePath, err)
	}

	// 2. Open DB connection
	db, err := sql.Open("sqlserver", *dsn)
	if err != nil {
		log.Fatalf("Failed to connect to SQL Server: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 3. Determine table and column targets
	var tableName, xmlColumn string
	switch *table {
	case "options":
		tableName = "dbo.flow_options_content"
		xmlColumn = "OptionsXML"
	case "config":
		tableName = "dbo.flow_config_content"
		xmlColumn = "ConfigXML"
	default:
		tableName = "dbo.flow_pipeline_content"
		xmlColumn = "PipelineXML"
	}

	// 4. Parameterized MERGE (Upsert) query
	query := fmt.Sprintf(`
		MERGE INTO %s AS target
		USING (SELECT @p1 AS Name) AS source
		ON (target.Name = source.Name)
		WHEN MATCHED THEN
			UPDATE SET Description = @p2, %s = @p3
		WHEN NOT MATCHED THEN
			INSERT (Name, Description, %s) VALUES (@p1, @p2, @p3);
	`, tableName, xmlColumn, xmlColumn)

	// 5. Execute with raw string parameter binding
	res, err := db.ExecContext(ctx, query, *name, *desc, string(xmlBytes))
	if err != nil {
		log.Fatalf("Failed to import XML into %s: %v", tableName, err)
	}

	rows, _ := res.RowsAffected()
	fmt.Printf("Successfully imported '%s' (%s) into %s (%d row(s) affected).\n", *name, *filePath, tableName, rows)
}