# UTF-8 & UTF-16 Encoding Guide for Go-Flow

This guide explains how to use **Go-Flow** with UTF-8 and UTF-16 encoded XML pipelines, environment configuration overrides, pre-baked CLI options, and database-resident XML storage (e.g., Microsoft SQL Server `NVARCHAR(MAX)`).

---

## 1. Why UTF-16 in Enterprise ETL?

Modern enterprise environments frequently combine Unix/Linux microservices and Microsoft Windows infrastructure:
* **Microsoft SQL Server**: Stores Unicode strings in `NVARCHAR(n)` and `NVARCHAR(MAX)` columns using native 16-bit encoding (UCS-2 / UTF-16LE).
* **PowerShell**: Default redirection and `Out-File` operators on Windows PowerShell produce UTF-16LE files with a Byte Order Mark (`0xFF 0xFE`).
* **Legacy SSIS & DTS Packages**: XML configuration packages exported from Microsoft ETL tools often default to UTF-16 declarations (`<?xml version="1.0" encoding="UTF-16"?>`).

**Go-Flow automatically detects and normalizes all UTF-8 and UTF-16 encoding variants at runtime.** You do not need to convert files manually before running pipelines or validating schemas.

---

## 2. CLI Execution with UTF-16 Files

Go-Flow transparently handles UTF-16 files across all CLI inputs (`-file`, `-config`, `-options`).

### Running a UTF-16 Pipeline File
```shell
flow.exe -file "C:\pipelines\nightly_billing.xml"
```
Even if `nightly_billing.xml` is saved as UTF-16LE with BOM and begins with `<?xml version="1.0" encoding="UTF-16"?>`, Go-Flow normalizes the byte stream and executes all nodes cleanly.

### Running with UTF-16 Config Overrides
```shell
flow.exe -file "pipeline.xml" -config "C:\configs\config_prod_utf16.xml"
```
Environment variable overrides and database connection strings in UTF-16 config files are parsed and merged into the active pipeline registry.

### Pre-Baked CLI Options in UTF-16
```shell
flow.exe -options "C:\profiles\options_utf16.xml"
```
Options files (which can specify `-file`, `-config`, and `-vars`) may also be authored and saved in UTF-16.

### Validating UTF-16 Pipelines with `-validate`
```shell
flow.exe -file "C:\pipelines\nightly_billing.xml" -validate
```
Go-Flow uses `xmllint` under the hood. It normalizes UTF-16 bytes into standard UTF-8 and pipes them directly to `xmllint`'s standard input. Schema validation succeeds without altering the file on disk.

---

## 3. Database-Resident XML (SQL Server `NVARCHAR(MAX)`)

When storing XML pipelines directly in a relational database, queries return string data over the database driver protocol:

```shell
flow.exe -file "sql://sqlserver@localhost:1433?database=analytics&integrated+security=true#SELECT PipelineXML FROM dbo.flow_pipeline_content WHERE Name='copilot_usage'"
```

### Supported Table Layout
```sql
CREATE TABLE dbo.flow_pipeline_content (
    Name NVARCHAR(255) PRIMARY KEY,
    Description NVARCHAR(4000) NULL,
    PipelineXML NVARCHAR(MAX) NOT NULL
);
```

### How Database Loading Works
1. Go-Flow executes the SQL query using `database/sql`.
2. The SQL Server TDS driver fetches the `NVARCHAR(MAX)` column into a Go UTF-8 string or raw byte slice.
3. Go-Flow's resource loader inspects the byte payload. If the string contains an `encoding="UTF-16"` XML declaration or BOM units, it normalizes the stream into valid UTF-8 and updates the declaration to `encoding="UTF-8"`.
4. Go's standard XML parser compiles the pipeline AST without error.

---

## 4. Importing Files into the Database (`mssql_importer.go`)

The helper tool located in `db_importer/mssql_importer.go` allows you to upload local XML files into SQL Server tables.

```shell
cd db_importer
go run mssql_importer.go \
  -dsn "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true" \
  -table pipeline \
  -name "github_ai_credit_usage" \
  -file "../examples/github_ai_credit_usage.xml" \
  -desc "GitHub Copilot Billing Usage Pipeline"
```

### Avoiding TDS Driver Double-Encoding
When importing files saved in UTF-16LE, passing raw bytes directly into SQL Server driver parameter bindings (`@p3`) causes the driver to encode the already 16-bit bytes as an `NVARCHAR` string, producing corrupted Chinese/mojibake characters in SQL Server.

`mssql_importer.go` includes automatic text decoding:
* It inspects the input file for UTF-16 LE/BE BOM markers.
* It decodes the file into a native Go string before passing it to `db.ExecContext`.
* SQL Server receives clean Unicode text and stores it accurately in `NVARCHAR(MAX)`.

---

## 5. Converting File Encodings (PowerShell Reference)

If you need to convert files between UTF-8 and UTF-16 manually in your scripts:

### Convert to UTF-16LE (Unicode)
```powershell
# Windows PowerShell 5.1 & PowerShell 7+
Get-Content -Path "pipeline_utf8.xml" -Raw | Set-Content -Path "pipeline_utf16.xml" -Encoding Unicode
```

### Convert to UTF-8 (No BOM)
```powershell
# PowerShell 7+
Get-Content -Path "pipeline_utf16.xml" -Raw | Set-Content -Path "pipeline_utf8.xml" -Encoding utf8NoBOM
```

---

## 6. Troubleshooting Common Issues

### Issue 1: `XML syntax error on line 1: invalid UTF-8`
* **Cause**: An application or custom script passed raw UTF-16 bytes (starting with `0xFF 0xFE`) directly into Go's standard `xml.NewDecoder` without transcoding.
* **Resolution**: Use `flow.NormalizeXMLBytes(data)` or `flow.ParseXMLConfig(data)`, which automatically normalizes UTF-16 bytes to UTF-8.

### Issue 2: `xmllint: parser error : Blank needed here` on `pipeline.xsd`
* **Cause**: Declaring `encoding="UTF-16"` on line 1 of an XSD file that is stored as 8-bit UTF-8 text on disk. `xmllint` treats every 2 ASCII characters as a 16-bit code unit, corrupting the schema compilation.
* **Resolution**: Always keep `<?xml version="1.0" encoding="UTF-8"?>` on line 1 of `xsd/pipeline.xsd` and `xsd/options.xsd`. Pipeline files validated *against* the schema may still declare `encoding="UTF-16"`.

### Issue 3: Corrupted Mojibake / Chinese characters in SQL Server
* **Cause**: UTF-16 byte slices were converted directly via `string(bytes)` instead of being decoded via `unicode/utf16`.
* **Resolution**: Use `flow.DecodeTextBytes(bytes)` or the updated `mssql_importer.go`.
