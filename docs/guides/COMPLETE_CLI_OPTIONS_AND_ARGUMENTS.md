# Command-Line Arguments Reference: `flow.exe`, `crypto_tool`, and `db_importer`

This document provides a comprehensive command-line reference for all three executable utilities in the **ETL Madness Flow** ecosystem:

1. [`flow.exe`](#1-flowexe-core-pipeline-engine--builder-server): The primary pipeline execution engine, preflight validator, XML/AST schema verifier, visual builder server, and database import/export manager.
2. [`crypto_tool`](#2-crypto_tool-cryptographic-utility): Standalone utility for AES-256-GCM encryption/decryption, digital signature signing/verification (OpenSSL, OpenPGP, PKCS#7/CMS), and cryptographic keypair generation.
3. [`db_importer`](#3-db_importer-mssql-database-xml-importer--exporter): Dedicated CLI utility to import XML pipelines, options, and configs into SQL Server repositories (or export to file/stdout), featuring automatic UTF-8/UTF-16 encoding handling and at-rest encryption.

---

## Architecture & Shared Standards

### Ecosystem Overview

```mermaid
flowchart LR
    subgraph SecurityLayer["1. Cryptography & Packaging"]
        CT["crypto_tool"]
        CT -->|"Encrypt (AES-256-GCM)"| ENC["Encrypted XML<br>(FLOWENC:v1:...)"]
        CT -->|"Sign (OpenSSL / PGP / PKCS#7)"| SIG["Signed Package<br>(FLOWSIGNED:v1:... or .sig)"]
    end

    subgraph StorageLayer["2. Repository Storage"]
        DBI["db_importer"]
        DB["SQL Server / Database<br>(dbo.flow_*_content)"]
        ENC --> DBI
        SIG --> DBI
        DBI -->|"MERGE / Upsert"| DB
    end

    subgraph RuntimeLayer["3. Execution Engine"]
        FLOW["flow.exe"]
        DB -->|"sql:// or db:// URI"| FLOW
        ENC -->|"Local file / HTTP"| FLOW
        SIG -->|"Local file / HTTP"| FLOW
        FLOW -->|"Telemetry & Events"| SINK["Audit Tables / DB Sink"]
    end
```

### Shared Secret Resolution Precedence

When encryption or decryption operations are performed, all three utilities resolve passphrases and cryptographic keys using the identical precedence hierarchy:

```mermaid
flowchart TD
    P1["1. Key File (-key-file &lt;path&gt;)<br>Safest: reads key directly from protected file"] -->|"If not provided"| P2["2. Command-Line Flag (-secure-key or -key)<br>Emits warning: exposed in system process listings"]
    P2 -->|"If not provided"| P3["3. XML Options Profile (&lt;secure-key&gt;)<br>(flow.exe only via -options profile)"]
    P3 -->|"If not provided"| P4["4. FLOW_SECURE_KEY<br>Environment variable"]
    P4 -->|"If not provided"| P5["5. SECURE_KEY<br>Fallback generic environment variable"]

    classDef safe fill:#e6fffa,stroke:#047857,stroke-width:2px;
    classDef warn fill:#fffbeb,stroke:#b45309,stroke-width:2px;
    classDef env fill:#f0f9ff,stroke:#0284c7,stroke-width:2px;
    class P1 safe;
    class P2 warn;
    class P3,P4,P5 env;
```

> [!WARNING]
> **Process Listing Security Warning**: Passing encryption keys or passphrases directly on the command line via `-secure-key` exposes secrets to other users and processes on the system (e.g., via `ps aux`, `Get-Process`, or Windows Task Manager). In production, automated schedulers, and CI/CD pipelines, always pass keys using `-key-file` or the `FLOW_SECURE_KEY` environment variable.

### Supported Resource URIs

Arguments in `flow.exe` that load resources (`-file`, `-options`, `-config`, `-import-file`) support uniform resource resolution:

| Resource Type | Format | Example |
| :--- | :--- | :--- |
| **Local File** | Standard relative or absolute path | `-file ".\pipelines\finance.xml"` |
| **HTTP / HTTPS** | `http://...` or `https://...` | `-file "https://internal.repo/pipelines/etl.xml"` |
| **HTTP Basic Auth** | `https://Basic%20<base64>@host/...` | `-file "https://Basic%20dXNlcjpwYXNz@internal.repo/etl.xml"` |
| **SQL Server Query** | `sql://sqlserver@<dsn>#<SELECT_QUERY>` | `-options "sql://sqlserver@localhost:1433?database=ETL#SELECT OptionsXML FROM dbo.flow_options_content WHERE Name='PROD'"` |
| **Generic DB Alias** | `db://<driver>@<dsn>#<SELECT_QUERY>` | `-file "db://postgres@app_user:pass@db:5432/app#SELECT pipeline_xml FROM flow_pipelines WHERE name='BILLING'"` |

### Common Exit Codes

All three tools conform to standard operating system exit codes:

* **`0`**: Success (pipeline ran successfully, schema/AST valid, verification passed, or import/export completed).
* **`1`**: Failure (execution error, validation error, signature verification failed, decryption key missing/invalid, or database connection error).

---

## 1. `flow.exe`: Core Pipeline Engine & Builder Server

`flow.exe` is the central executable. It executes ETL pipelines, runs preflight validations, starts an interactive HTMX web server for visual pipeline authoring, transforms pipeline definitions via XSLT into diagrams, and manages local or external repository persistence.

### Command Syntax

```powershell
.\flow.exe [options...]
```

### Full Flag Reference Table

| Flag | Type | Default | Category | Description |
| :--- | :---: | :---: | :---: | :--- |
| `-file` | `string` | `scripts.xml` | Execution | Path or URI to XML file containing scripts, databases, and workflow nodes. Supports local files, HTTP/HTTPS URLs, and `sql://` / `db://` database queries. |
| `-options` | `string` | `""` | Execution | Path or URI to an XML options defaults file (`<flow_cli_options><options>...`). Values in this file set flag defaults without overriding flags explicitly passed on the CLI. |
| `-config` | `string` | `""` | Execution | Optional path or URI to a `CONFIG.xml` file containing variable and database connection overrides. Merged hierarchically with the base file. |
| `-format` | `string` | `csv` | Execution | Output formatting applied to execution results. Supported: `csv`, `text`, `json`, `jsonpretty` (or `prettyjson`), `markdown` (or `md`, `table`), and `stream`. |
| `-vars` | `string` | `""` | Execution | Comma-separated `key=value` runtime variable overrides (e.g., `-vars "TargetTable=tbl_prod,Threshold=500"`). Recursively expands `{{var}}` placeholders up to 3 levels deep. |
| `-validate` | `bool` | `false` | Validation | Validates XML schema (XSD if specified) and AST semantic structure without executing any pipeline nodes. Exits with code `0` on success. |
| `-preflight` | `bool` | `false` | Validation | Executes preflight validation nodes only (`<preflight>` section), skipping the main pipeline `<flow>` nodes. |
| `-xsd` | `string` | `""` | Validation | Path to an XML Schema Definition (`.xsd`) file used to validate the pipeline XML via `xmllint` prior to parsing. |
| `-debug` | `bool` | `false` | Diagnostic | Enables verbose console logging, including connection string resolution, driver dialect detection, digital signature verification metadata, and sink telemetry. |
| `-gopath` | `string` | `$GOPATH` | Runtime | GOPATH directory for dynamic package imports used by the embedded Yaegi Go interpreter engine. Defaults to the host `GOPATH` environment variable. |
| `-xslt` | `string` | `""` | Visualization | Path to custom XSLT stylesheet. Transforms pipeline XML while injecting an auto-generated Mermaid workflow diagram. Exits immediately after transformation. |
| `-out` | `string` | `""` | Visualization | Output file path for XSLT-transformed XML produced by `-xslt`. |
| `-encrypted` | `bool` | `false` | Security | Indicates that input files/database resources are AES-256-GCM encrypted and must be decrypted prior to parsing, or instructs `-export-file` / `-import-file` to encrypt. |
| `-secure-key` | `string` | `""` | Security | Symmetric encryption or decryption key/passphrase. Emits a security warning to `stderr`; falls back to `FLOW_SECURE_KEY` or `SECURE_KEY`. |
| `-key-file` | `string` | `""` | Security | Path to file containing symmetric key/passphrase. Preferred over `-secure-key` to prevent exposing secrets in system process tables. |
| `-verify-signature`| `bool` | `false` | Security | Enforces digital signature verification before decrypting or executing the pipeline. |
| `-public-key` | `string` | `""` | Security | Path to public key file (PEM format; RSA, ECDSA, Ed25519) for digital signature verification. |
| `-cert` | `string` | `""` | Security | Path to X.509 certificate file (PEM or DER format) for signature verification (PKCS#7/CMS or X.509 RSA/ECDSA). |
| `-ca-cert` | `string` | `""` | Security | Path to root CA certificate file for signature verification certificate chain validation. |
| `-keyring` | `string` | `""` | Security | Path to OpenPGP armored public keyring file for GPG signature verification. |
| `-signature` | `string` | `""` | Security | Path to detached digital signature file (`.sig`, `.asc`, `.p7s`). When omitted, companion files with matching basenames are automatically checked. |
| `-builder` | `bool` | `false` | Builder UI | Starts the interactive local HTMX visual pipeline builder web server and opens the UI. |
| `-builder-port` | `int` | `0` | Builder UI | Port for the visual builder web server. Default `0` chooses an ephemeral dynamic port. Can be set via `FLOW_BUILDER_PORT` or `PORT` environment variables. |
| `-builder-db` | `string` | `flow_builder.db` | Builder UI | SQLite database file path used for visual builder local project storage. |
| `-purge-db` | `bool` | `false` | Builder UI | Purges all pipeline data from the SQLite visual builder database, re-initializes defaults, and exits. |
| `-import-file` | `string` | `""` | Repository | Path to a pipeline, options, or config XML file to import into the SQLite builder database or an external database specified by `-dsn`. |
| `-import-name` | `string` | `""` | Repository | Custom record name for item imported with `-import-file`. Defaults to the source file basename without extension. |
| `-import-type` | `string` | `pipeline` | Repository | Item type for `-import-file`: `pipeline`, `options`, or `config`. |
| `-export-file` | `string` | `""` | Repository | Destination file path to export a pipeline, options, or config file (or `-` for standard output `stdout`). |
| `-export-name` | `string` | `""` | Repository | Name of the pipeline, options, or config to export from the database (required when `-export-file` is specified). |
| `-export-type` | `string` | `pipeline` | Repository | Item type for `-export-file`: `pipeline`, `options`, or `config`. |
| `-dsn` | `string` | `""` | Repository | External database DSN for direct database-to-database repository import and export (supports SQL Server, PostgreSQL, MySQL, and SQLite). |

### Output Formats (`-format`)

| Format | Output Description |
| :--- | :--- |
| `csv` *(Default)* | Standard comma-separated table displaying `ScriptID`, `ReturnCode`, `Duration`, and `ResultsString`. |
| `text` | Human-readable plain text tabular format. |
| `json` | Compact JSON array containing all `ScriptResult` objects. |
| `jsonpretty` / `prettyjson` | Formatted multi-line JSON with 4-space indentation. |
| `markdown` / `md` / `table` | GitHub Flavored Markdown table. |
| `stream` | Real-time console event streaming mode (`ExecuteRun` engine) with live node transitions, parent/child execution relationships, and audit sink telemetry. |

### Environment Variables Recognized by `flow.exe`

* `FLOW_SECURE_KEY`: Primary symmetric passphrase used to decrypt encrypted scripts, options, and configs.
* `SECURE_KEY`: Secondary fallback passphrase if `FLOW_SECURE_KEY` is unset.
* `FLOW_BUILDER_PORT`: Default port for the HTMX builder server when `-builder-port` is not explicitly passed.
* `PORT`: Generic fallback port for web services (e.g., in cloud environments like Cloud Run or Heroku).
* `GOPATH`: Go workspace directory used by Yaegi to import third-party packages in interpreted script blocks.
* `USER` / `USERNAME`: Captured for execution audit logs in `dbo.pipeline_runs`.
* `HOSTNAME` / `COMPUTERNAME`: Captured for execution audit logs in `dbo.pipeline_runs`.

---

## 2. `crypto_tool`: Cryptographic Utility

`crypto_tool` is a self-contained, zero-dependency command-line utility for encryption, decryption, digital signing, signature verification, and keypair generation.

### Command Syntax

```bash
crypto_tool [options...]
```

### Full Flag Reference Table

| Flag | Alias | Type | Default | Category | Description |
| :--- | :---: | :---: | :---: | :---: | :--- |
| `-action` | — | `string` | `encrypt` | Operation | Cryptographic operation to perform: `encrypt`, `decrypt`, `gen-key`, `sign`, `verify`, or `gen-keypair`. |
| `-gen-key` | — | `bool` | `false` | Operation | Shortcut flag for `-action gen-key`. Generates a 256-bit cryptographically secure symmetric key. |
| `-in` | — | `string` | `""` | I/O | Input file path (use `"-"` or leave empty for standard input `stdin`). |
| `-out` | — | `string` | `""` | I/O | Output file path (use `"-"` or leave empty for standard output `stdout`). Output files containing secret keys are written with `0600` permissions. |
| `-secure-key` | `-key` | `string` | `""` | Symmetric | Passphrase or raw key for AES-256-GCM encryption or decryption. Emits a security warning to `stderr`; falls back to `FLOW_SECURE_KEY` or `SECURE_KEY`. |
| `-key-file` | — | `string` | `""` | Symmetric | Path to file containing encryption/decryption key/passphrase. Prevents exposing secrets in operating system process listings. |
| `-binary` | — | `bool` | `false` | Symmetric / Sig | Output raw binary ciphertext or ASN.1 signature bytes instead of the default text-safe armored envelope (`FLOWENC:v1:...` or `FLOWSIG:v1:...`). |
| `-sig-type` | — | `string` | `openssl` | Signing | Digital signature format for `-action sign`: `openssl` (RSA/ECDSA/Ed25519), `pgp` (OpenPGP), or `pkcs7` (CMS / PKCS#7 / Windows Authenticode). |
| `-private-key` | `-priv` | `string` | `""` | Signing | Private key file path (PEM or OpenPGP format) used to generate digital signatures. |
| `-passphrase` | — | `string` | `""` | Signing | Passphrase for decrypting password-protected private keys (PKCS#1, PKCS#8, or OpenPGP). |
| `-cert` | — | `string` | `""` | Sign / Verify | X.509 certificate file path (PEM or DER format). Required when creating PKCS#7 signatures (`-sig-type pkcs7`); used for cert-based verification. |
| `-wrap` | — | `bool` | `false` | Signing | Wraps both the digital signature and input content into a single unified `FLOWSIGNED:v1:...` envelope. Eliminates the need for detached `.sig` files. |
| `-signature` | `-sig` | `string` | `""` | Verification | Detached signature file path for `-action verify`. If omitted, automatically checks for companion files `<in>.sig`, `<in>.asc`, or `<in>.p7s`. |
| `-public-key` | `-pub` | `string` | `""` | Verification | Public key file path (PEM or OpenPGP format) for signature verification. |
| `-ca-cert` | — | `string` | `""` | Verification | Root or intermediate CA certificate file path used to validate certificate chains during PKCS#7 or X.509 verification. |
| `-keyring` | — | `string` | `""` | Verification | OpenPGP armored public keyring file path for OpenPGP/GPG signature verification. |
| `-keypair-type` | — | `string` | `rsa` | Keygen | Asymmetric keypair type for `-action gen-keypair`: `rsa`, `ecdsa` (P-256), `ed25519`, or `pgp` (aliases: `gpg`, `openpgp`). |
| `-bits` | — | `int` | `2048` | Keygen | Bit length for RSA keypair generation (e.g., `2048`, `3072`, `4096`). |
| `-out-priv` | — | `string` | `private_key.pem` | Keygen | Output file path for generated private key. File permissions are automatically restricted to `0600` (owner read/write only). |
| `-out-pub` | — | `string` | `public_key.pem` | Keygen | Output file path for generated public key (written with `0644` permissions). |
| `-out-cert` | — | `string` | `""` | Keygen | Optional output path to generate a self-signed X.509 certificate during keypair generation (valid for 365 days). |
| `-cn` | — | `string` | `Flow Code Signer` | Keygen | Common Name (`CN`) embedded in the generated X.509 certificate or OpenPGP user identity. |

---

## 3. `db_importer`: MSSQL Database XML Importer & Exporter

`db_importer` is a dedicated utility for migrating pipelines, options profiles, and variable configs into and out of relational database storage (SQL Server `dbo.flow_*_content` tables). It includes automatic UTF-8 and UTF-16 character encoding translation and supports transparent at-rest encryption.

### Command Syntax

```powershell
.\db_importer.exe [options...]
```

### Full Flag Reference Table

| Flag | Type | Default | Category | Description |
| :--- | :---: | :---: | :---: | :--- |
| `-action` | `string` | `import` | Operation | Database operation to execute: `import` (upserts record into table) or `export` (extracts record from table). |
| `-export` | `bool` | `false` | Operation | Shortcut boolean flag to perform an export operation (equivalent to `-action export`). |
| `-dsn` | `string` | `sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true` | Connection | Connection string for SQL Server instance (URL format or ADO/ODBC connection string). |
| `-file` | `string` | `github_ai_credit_usage.xml` | I/O | Path to input XML file to import, or destination path for exported XML. Use `"-"` or leave empty to read from `stdin` (import) or write to `stdout` (export). |
| `-name` | `string` | `github_ai_credit_usage` | Target | Unique logical name key of the record in the database (`Name` column in target table). Used for upsert matching and queries. |
| `-table` | `string` | `pipeline` | Target | Target table and column mapping: `pipeline`, `options`, or `config`. See [Database Mapping](#target-table-mappings) below. |
| `-desc` | `string` | `Imported via Go importer` | Metadata | Descriptive text stored in the `Description` column of the table (import only). |
| `-encrypted` | `bool` | `false` | Security | When importing: encrypts content into an armored `FLOWENC:v1:...` envelope before inserting into the database.<br>When exporting: decrypts encrypted database content before writing to file or stdout. |
| `-secure-key` | `string` | `""` | Security | Passphrase or key for AES-256-GCM encryption/decryption. Emits a security warning to `stderr`; falls back to `FLOW_SECURE_KEY` or `SECURE_KEY`. |
| `-key-file` | `string` | `""` | Security | Path to file containing the encryption key/passphrase. Preferred over `-secure-key` to avoid exposing secrets in process tables. |

### Target Table Mappings

| `-table` Value | Database Table Target | XML Payload Column | Record Identifier Column |
| :--- | :--- | :--- | :--- |
| `pipeline` *(Default)* | `dbo.flow_pipeline_content` | `PipelineXML` (`NVARCHAR(MAX)`) | `Name` (`VARCHAR(120)`) |
| `options` | `dbo.flow_options_content` | `OptionsXML` (`NVARCHAR(MAX)`) | `Name` (`VARCHAR(120)`) |
| `config` | `dbo.flow_config_content` | `ConfigXML` (`NVARCHAR(MAX)`) | `Name` (`VARCHAR(120)`) |

### Encoding & Security Features

* **BOM & UTF-16 Auto-Detection**: Automatically detects and decodes UTF-8 BOM (`0xEF 0xBB 0xBF`), UTF-16 Little Endian BOM (`0xFF 0xFE`), and UTF-16 Big Endian BOM (`0xFE 0xFF`). Files generated by PowerShell or Windows editors are cleanly ingested without manual conversion.
* **Idempotent Upsert (MERGE)**: Imports use an atomic `MERGE INTO` statement on SQL Server. If the record name already exists, its `Description` and XML payload are updated in place; otherwise, a new record is inserted.
* **Hardened File Permissions**: Exported decrypted files written to disk are automatically locked down with `0600` permissions (owner read/write only).

---

## Practical Usage Examples

### 1. `flow.exe` Workflows

#### Basic Pipeline Execution with Markdown Output
```powershell
.\flow.exe -file ".\pipelines\nightly_billing.xml" -format markdown
```

#### Passing Runtime Overrides (`-vars` and `-config`)
```powershell
.\flow.exe `
  -file ".\pipelines\etl.xml" `
  -config ".\configs\production.xml" `
  -vars "BatchDate=2026-10-10,MaxRetries=5,TargetSchema=stage" `
  -debug
```

#### Preflight Checks & Schema Validation Only
```powershell
# Validate XSD schema and AST without executing
.\flow.exe -file ".\pipelines\etl.xml" -xsd ".\xsd\pipeline.xsd" -validate

# Execute only the preflight nodes (smoke tests, DB pings)
.\flow.exe -file ".\pipelines\etl.xml" -preflight
```

#### Running Directly from a SQL Database Query
```powershell
.\flow.exe `
  -file "sql://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=ETL&trustServerCertificate=true#SELECT PipelineXML FROM dbo.flow_pipeline_content WHERE Name='BILLING_JOB'" `
  -options "sql://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=ETL&trustServerCertificate=true#SELECT OptionsXML FROM dbo.flow_options_content WHERE Name='PROD_OPTIONS'" `
  -key-file "C:\secrets\flow_master.key"
```

#### Starting the HTMX Visual Builder
```powershell
# Launch visual builder on a specific port with custom SQLite storage
.\flow.exe -builder -builder-port 8080 -builder-db "C:\flow\builder_workspace.db"

# Reset / purge the builder SQLite database
.\flow.exe -purge-db -builder-db "C:\flow\builder_workspace.db"
```

#### Generating a Mermaid Diagram via XSLT
```powershell
.\flow.exe -file ".\pipelines\order_processing.xml" -xslt ".\transforms\mermaid.xslt" -out ".\docs\diagram.xml"
```

---

### 2. `crypto_tool` Workflows

#### Symmetric Encryption & Key Generation
```bash
# 1. Generate a cryptographically secure 256-bit AES key
crypto_tool -gen-key -out master.key

# 2. Encrypt an XML pipeline (outputs text-armored FLOWENC:v1:... file)
crypto_tool -action encrypt -in pipeline.xml -out pipeline.xml.enc -key-file master.key

# 3. Decrypt the file
crypto_tool -action decrypt -in pipeline.xml.enc -out pipeline.xml -key-file master.key
```

#### Generating Asymmetric Keypairs & Certificates
```bash
# RSA Keypair + Self-Signed X.509 Certificate
crypto_tool -action gen-keypair -keypair-type rsa -bits 2048 \
  -out-priv release_priv.pem -out-pub release_pub.pem -out-cert release_cert.pem \
  -cn "Production Code Signer"

# ECDSA (P-256) Keypair
crypto_tool -action gen-keypair -keypair-type ecdsa -out-priv ec_priv.pem -out-pub ec_pub.pem

# OpenPGP / GPG Armored Keypair
crypto_tool -action gen-keypair -keypair-type pgp -out-priv gpg_priv.asc -out-pub gpg_pub.asc -cn "ETL Admin"
```

#### Signing Files
```bash
# OpenSSL Detached Signature (.sig)
crypto_tool -action sign -in pipeline.xml -out pipeline.xml.sig -private-key release_priv.pem -sig-type openssl

# PKCS#7 / CMS Detached Signature (.p7s)
crypto_tool -action sign -in pipeline.xml -out pipeline.xml.p7s -private-key release_priv.pem -cert release_cert.pem -sig-type pkcs7

# Self-Contained Signed Package (FLOWSIGNED:v1:... envelope)
crypto_tool -action sign -in pipeline.xml -out pipeline.xml.signed -private-key release_priv.pem -wrap
```

#### Verifying Signatures
```bash
# Verify detached signature
crypto_tool -action verify -in pipeline.xml -signature pipeline.xml.sig -public-key release_pub.pem

# Verify self-contained unified envelope and extract payload
crypto_tool -action verify -in pipeline.xml.signed -public-key release_pub.pem -out verified_pipeline.xml
```

---

### 3. `db_importer` Workflows

#### Importing XML Pipelines into SQL Server
```powershell
# Plain XML import
.\db_importer.exe `
  -dsn "sqlserver://sa:Password123!@localhost:1433?database=ETL&trustServerCertificate=true" `
  -table pipeline `
  -name "github_billing" `
  -file ".\pipelines\github_billing.xml" `
  -desc "Production GitHub Billing Workflow"

# Encrypted import (stored encrypted at rest in SQL Server)
.\db_importer.exe `
  -dsn "sqlserver://sa:Password123!@localhost:1433?database=ETL&trustServerCertificate=true" `
  -table options `
  -name "OPTIONS_FINANCE" `
  -file ".\options\finance_options.xml" `
  -encrypted `
  -key-file "C:\secrets\flow_master.key"
```

#### Exporting from SQL Server to File or Stdout
```powershell
# Export and automatically decrypt to local file with 0600 permissions
.\db_importer.exe `
  -action export `
  -dsn "sqlserver://sa:Password123!@localhost:1433?database=ETL&trustServerCertificate=true" `
  -table pipeline `
  -name "github_billing" `
  -file ".\exported_pipeline.xml" `
  -encrypted `
  -key-file "C:\secrets\flow_master.key"

# Pipe exported XML directly to stdout
.\db_importer.exe -export -table pipeline -name "github_billing" -file - | xmllint --format -
```

---

## End-to-End Enterprise Recipes

### Recipe 1: Production CI/CD Release (Encrypt-then-Sign)

Secure pipeline distribution workflow where untrusted files are cryptographically verified before decryption keys are ever accessed:

```mermaid
sequenceDiagram
    autonumber
    actor Developer as Build / CI Pipeline
    participant Crypto as crypto_tool
    participant Importer as db_importer
    participant SQL as SQL Server Repo
    participant Engine as flow.exe (Prod Host)

    Developer->>Crypto: crypto_tool -action encrypt -in pipeline.xml -out pipeline.xml.enc -key-file secret.key
    Developer->>Crypto: crypto_tool -action sign -in pipeline.xml.enc -out pipeline.xml.signed -private-key priv.pem -wrap
    Developer->>Importer: db_importer -table pipeline -name "DAILY_JOB" -file pipeline.xml.signed
    Importer->>SQL: MERGE INTO dbo.flow_pipeline_content
    Engine->>SQL: Query via sql:// URI
    Engine->>Engine: 1. Verify Digital Signature (release_pub.pem)
    Engine->>Engine: 2. Decrypt AES-256-GCM (FLOW_SECURE_KEY)
    Engine->>Engine: 3. Parse AST & Execute Workflow
```

**Step-by-step commands:**

```bash
# 1. Encrypt the pipeline definition
crypto_tool -action encrypt -in pipeline.xml -out pipeline.xml.enc -key-file prod.key

# 2. Sign the encrypted package into a unified FLOWSIGNED envelope
crypto_tool -action sign -in pipeline.xml.enc -out pipeline.xml.signed -private-key release_priv.pem -wrap

# 3. Import the signed package into SQL Server repository
db_importer -dsn "sqlserver://sa:Password@db:1433?database=ETL" -table pipeline -name "NIGHTLY_SYNC" -file pipeline.xml.signed

# 4. Execute on production server (verifies signature first, then decrypts)
export FLOW_SECURE_KEY="YourSecurePassphrase"
flow.exe \
  -file "sql://sqlserver@db:1433?database=ETL#SELECT PipelineXML FROM dbo.flow_pipeline_content WHERE Name='NIGHTLY_SYNC'" \
  -public-key "/etc/flow/release_pub.pem"
```

---

### Recipe 2: Options Profile Overrides (`-options`)

You can create an XML options file conforming to `xsd/options.xsd` to pre-configure defaults for production hosts:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<flow_cli_options>
    <options>
        <format>markdown</format>
        <debug>true</debug>
        <verify-signature>true</verify-signature>
        <public-key>C:\flow\keys\release_pub.pem</public-key>
        <encrypted>true</encrypted>
    </options>
</flow_cli_options>
```

Execute `flow.exe` referencing the options file:

```powershell
.\flow.exe -options "C:\flow\prod_options.xml" -file "C:\flow\pipelines\job.xml" -key-file "C:\secrets\key.txt"
```

> [!NOTE]
> Any flag provided explicitly on the command line (e.g., `-format json`) will override the value defined inside `-options`.

---

## Quick Flag Cross-Reference Matrix

| Capability | `flow.exe` | `crypto_tool` | `db_importer` |
| :--- | :---: | :---: | :---: |
| Specify Input File | `-file <path\|uri>` | `-in <path>` | `-file <path>` |
| Specify Output File | `-out <path>` *(XSLT)* | `-out <path>` | `-file <path>` *(Export)* |
| Use Stdin / Stdout | Via `-export-file -` | `-in -` / `-out -` | `-file -` |
| Symmetric Key File | `-key-file <path>` | `-key-file <path>` | `-key-file <path>` |
| Symmetric Key Flag | `-secure-key <str>` | `-secure-key` / `-key` | `-secure-key <str>` |
| Env Var Fallback | `FLOW_SECURE_KEY`, `SECURE_KEY` | `FLOW_SECURE_KEY`, `SECURE_KEY` | `FLOW_SECURE_KEY`, `SECURE_KEY` |
| Enable Encryption | `-encrypted` | `-action encrypt` | `-encrypted` |
| Generate Symmetric Key | — | `-gen-key` | — |
| Digital Signature File | `-signature <path>` | `-signature` / `-sig` | — |
| Digital Signing Key | — | `-private-key` / `-priv` | — |
| Signature Verification Key | `-public-key <path>` | `-public-key` / `-pub` | — |
| X.509 Certificate | `-cert <path>` | `-cert <path>` | — |
| Root CA Certificate | `-ca-cert <path>` | `-ca-cert <path>` | — |
| OpenPGP Keyring | `-keyring <path>` | `-keyring <path>` | — |
| Unified Signed Package | Transparently unwrapped | `-wrap` (create / extract) | Stored as text |
| Target Database DSN | `-dsn <str>` | — | `-dsn <str>` |
| Database Table Target | `-import-type` / `-export-type` | — | `-table <pipeline\|options\|config>` |
| Visual Builder Server | `-builder`, `-builder-port` | — | — |
