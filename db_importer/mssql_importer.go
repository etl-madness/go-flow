package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"unicode/utf16"

	_ "github.com/microsoft/go-mssqldb"
)

func resolveImportTarget(table string) (string, string, error) {
	switch table {
	case "options":
		return "dbo.flow_options_content", "OptionsXML", nil
	case "config":
		return "dbo.flow_config_content", "ConfigXML", nil
	case "pipeline", "":
		return "dbo.flow_pipeline_content", "PipelineXML", nil
	default:
		return "", "", fmt.Errorf("unsupported import table %q: expected pipeline, options, or config", table)
	}
}

func buildImportQuery(tableName, xmlColumn string) string {
	return fmt.Sprintf(`
		MERGE INTO %s AS target
		USING (SELECT @p1 AS Name) AS source
		ON (target.Name = source.Name)
		WHEN MATCHED THEN
			UPDATE SET Description = @p2, %s = @p3
		WHEN NOT MATCHED THEN
			INSERT (Name, Description, %s) VALUES (@p1, @p2, @p3);
	`, tableName, xmlColumn, xmlColumn)
}

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
	tableName, xmlColumn, err := resolveImportTarget(*table)
	if err != nil {
		log.Fatalf("Invalid import target: %v", err)
	}

	// 4. Parameterized MERGE (Upsert) query
	query := buildImportQuery(tableName, xmlColumn)

	// 5. Execute with raw string parameter binding
	res, err := db.ExecContext(ctx, query, *name, *desc, decodeFileContent(xmlBytes))
	if err != nil {
		log.Fatalf("Failed to import XML into %s: %v", tableName, err)
	}

	rows, _ := res.RowsAffected()
	fmt.Printf("Successfully imported '%s' (%s) into %s (%d row(s) affected).\n", *name, *filePath, tableName, rows)
}

func decodeFileContent(data []byte) string {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return string(data[3:])
	}
	if len(data) >= 2 {
		if data[0] == 0xFF && data[1] == 0xFE && len(data)%2 == 0 {
			codeUnits := make([]uint16, 0, len(data)/2-1)
			for i := 2; i+1 < len(data); i += 2 {
				codeUnits = append(codeUnits, uint16(data[i])|uint16(data[i+1])<<8)
			}
			return string(utf16.Decode(codeUnits))
		}
		if data[0] == 0xFE && data[1] == 0xFF && len(data)%2 == 0 {
			codeUnits := make([]uint16, 0, len(data)/2-1)
			for i := 2; i+1 < len(data); i += 2 {
				codeUnits = append(codeUnits, uint16(data[i+1])|uint16(data[i])<<8)
			}
			return string(utf16.Decode(codeUnits))
		}
	}
	return string(data)
}

