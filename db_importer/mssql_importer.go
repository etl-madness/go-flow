package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/etl-madness/go-flow/pkg/flowcrypto"
	_ "github.com/microsoft/go-mssqldb"
)

func resolveImportTarget(table string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(table)) {
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

func buildExportQuery(tableName, xmlColumn string) string {
	return fmt.Sprintf(`SELECT %s FROM %s WHERE Name = @p1`, xmlColumn, tableName)
}

func resolveSecureKey(cliKey string) string {
	return resolveSecureKeyWithFile(cliKey, "")
}

func resolveSecureKeyWithFile(cliKey, keyFilePath string) string {
	if strings.TrimSpace(cliKey) != "" {
		fmt.Fprintf(os.Stderr, "[WARNING] Passing encryption keys via CLI flags (-secure-key) may expose secrets in system process listings. Consider using the FLOW_SECURE_KEY environment variable or -key-file instead.\n")
		return strings.TrimSpace(cliKey)
	}
	if strings.TrimSpace(keyFilePath) != "" {
		data, err := os.ReadFile(strings.TrimSpace(keyFilePath))
		if err == nil {
			return strings.TrimSpace(string(data))
		}
		fmt.Fprintf(os.Stderr, "[WARNING] Failed reading key file %s: %v\n", keyFilePath, err)
	}
	if envKey := strings.TrimSpace(os.Getenv("FLOW_SECURE_KEY")); envKey != "" {
		return envKey
	}
	if envKey := strings.TrimSpace(os.Getenv("SECURE_KEY")); envKey != "" {
		return envKey
	}
	return ""
}

func main() {
	action := flag.String("action", "import", "Action to perform: import or export")
	exportFlag := flag.Bool("export", false, "Shortcut flag to export from database")
	dsn := flag.String("dsn", "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true", "SQL Server connection string")
	filePath := flag.String("file", "github_ai_credit_usage.xml", "Path to XML file to import or export destination")
	name := flag.String("name", "github_ai_credit_usage", "Name key in database")
	table := flag.String("table", "pipeline", "Target table type: pipeline, options, or config")
	desc := flag.String("desc", "Imported via Go importer", "Description of the XML content (import only)")
	encrypted := flag.Bool("encrypted", false, "Encrypt content before importing, or decrypt content after exporting")
	secureKey := flag.String("secure-key", "", "Encryption/decryption key (falls back to FLOW_SECURE_KEY or SECURE_KEY env var)")
	keyFile := flag.String("key-file", "", "Path to file containing encryption key/passphrase (avoids exposing key in process listings)")
	flag.Parse()

	actionValue := strings.ToLower(strings.TrimSpace(*action))
	if actionValue != "import" && actionValue != "export" {
		log.Fatalf("Invalid action %q: expected import or export", *action)
	}
	isExport := *exportFlag || actionValue == "export"
	key := resolveSecureKeyWithFile(*secureKey, *keyFile)

	if *encrypted && key == "" {
		log.Fatalf("Error: -encrypted was specified, but no key was provided. Specify -secure-key or set FLOW_SECURE_KEY.")
	}

	// 1. Determine table and column targets
	tableName, xmlColumn, err := resolveImportTarget(*table)
	if err != nil {
		log.Fatalf("Invalid table target: %v", err)
	}

	// 2. Open DB connection
	db, err := sql.Open("sqlserver", *dsn)
	if err != nil {
		log.Fatalf("Failed to connect to SQL Server: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if isExport {
		// =========================================================================
		// EXPORT ACTION
		// =========================================================================
		exportQuery := buildExportQuery(tableName, xmlColumn)

		var rawContent string
		err := db.QueryRowContext(ctx, exportQuery, *name).Scan(&rawContent)
		if err != nil {
			if err == sql.ErrNoRows {
				log.Fatalf("No record found in %s with Name = %q", tableName, *name)
			}
			log.Fatalf("Failed querying %s for Name = %q: %v", tableName, *name, err)
		}

		var finalBytes []byte
		if *encrypted || flowcrypto.IsEncryptedPayload([]byte(rawContent)) {
			if key == "" {
				log.Fatalf("Content in %s for %q is encrypted, but no secure key was provided. Specify -secure-key or set FLOW_SECURE_KEY.", tableName, *name)
			}
			decrypted, err := flowcrypto.DecryptAuto([]byte(rawContent), key, true)
			if err != nil {
				log.Fatalf("Failed to decrypt exported content for %q: %v", *name, err)
			}
			finalBytes = decrypted
		} else {
			finalBytes = []byte(rawContent)
		}

		if *filePath == "" || *filePath == "-" {
			_, err = os.Stdout.Write(finalBytes)
		} else {
			err = os.WriteFile(*filePath, finalBytes, 0600)
		}
		if err != nil {
			log.Fatalf("Failed to write exported content: %v", err)
		}

		if *filePath != "" && *filePath != "-" {
			fmt.Printf("Successfully exported '%s' from %s into %s.\n", *name, tableName, *filePath)
		}
		return
	}

	// =========================================================================
	// IMPORT ACTION
	// =========================================================================
	var rawData []byte
	if *filePath == "" || *filePath == "-" {
		rawData, err = io.ReadAll(os.Stdin)
	} else {
		rawData, err = os.ReadFile(*filePath)
	}
	if err != nil {
		log.Fatalf("Failed to read file %s: %v", *filePath, err)
	}

	decodedText := decodeFileContent(rawData)
	var contentToStore string

	if *encrypted {
		// Encrypt if not already armored
		if flowcrypto.IsEncryptedPayload([]byte(decodedText)) {
			contentToStore = decodedText
		} else {
			armored, err := flowcrypto.EncryptArmored([]byte(decodedText), key)
			if err != nil {
				log.Fatalf("Failed to encrypt content for import: %v", err)
			}
			contentToStore = armored
		}
	} else {
		contentToStore = decodedText
	}

	importQuery := buildImportQuery(tableName, xmlColumn)
	res, err := db.ExecContext(ctx, importQuery, *name, *desc, contentToStore)
	if err != nil {
		log.Fatalf("Failed to import XML into %s: %v", tableName, err)
	}

	rows, _ := res.RowsAffected()
	fmt.Printf("Successfully imported '%s' (%s) into %s (%d row(s) affected, encrypted: %t).\n", *name, *filePath, tableName, rows, *encrypted)
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
