# DB Importer Usage

To import an XML file into the database, use mssql_importer.go.
Below are some examples of how to use it.

**Note:** Only tested with SQL Server, other databases may not be supported or may require modifications to the mssql_importer.go code. Adoption for other databases is planned but has not been implemented yet.

## Dependent Schemas

```sql
 CREATE TABLE dbo.flow_pipeline_content
(
    [Id] BIGINT IDENTITY PRIMARY KEY,
    [Name] VARCHAR(120),
    [Tag] VARCHAR(4000), -- ';' seperated tags
    [Description] VARCHAR(256), 
    [CreatedOnUTC] DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    [Enabled] BIT DEFAULT 1, -- 0 = False, 1 = True, NULL = Unknown
    [PipelineXML] NVARCHAR(MAX)  NOT NULL
);
CREATE TABLE dbo.flow_options_content
(
    [Id] BIGINT IDENTITY PRIMARY KEY,
    [Name] VARCHAR(120),
    [Tag] VARCHAR(4000), -- ';' seperated tags
    [Description] VARCHAR(256), 
    [CreatedOnUTC] DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    [Enabled] BIT DEFAULT 1, -- 0 = False, 1 = True, NULL = Unknown
    [OptionsXML] NVARCHAR(MAX)  NOT NULL
);
CREATE TABLE dbo.flow_config_content
(
    [Id] BIGINT IDENTITY PRIMARY KEY,
    [Name] VARCHAR(120),
    [Tag] VARCHAR(4000), -- ';' seperated tags
    [Description] VARCHAR(256), 
    [CreatedOnUTC] DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    [Enabled] BIT DEFAULT 1, -- 0 = False, 1 = True, NULL = Unknown
    [ConfigXML] NVARCHAR(MAX) NOT NULL
);
```

## Import Pipeline

```shell
go run mssql_importer.go -dsn "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true" -table pipeline -name "github_ai_credit_usage" -file "github_ai_credit_usage.xml" -desc "GitHub Copilot Billing Usage Pipeline"
```

## Import Options

```shell
go run mssql_importer.go -dsn "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true" -table options -name "github_ai_credit_usage" -file "OPTIONS_GITHUB_AI_CREDIT_USAGE.xml" -desc "GitHub Copilot Billing Usage Options"
```

## Import Config

```shell
go run mssql_importer.go -dsn "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true" -table config -name "github_ai_credit_usage" -file "CONFIG_GITHUB_AI_CREDIT_USAGE.xml" -desc "GitHub Copilot Billing Usage Config"
```

## Run from Database

To run the flow from the database using the imported options, use the following command:

```shell
flow.exe -options "sql://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true#SELECT OptionsXML from dbo.flow_options_content where Name = 'OPTIONS_GITHUB_AI_CREDIT_USAGE'" 
```

## Encrypted Import & Export

The db_importer supports AES-256-GCM encryption with PBKDF2 key derivation.

> **Security Note**: Passing encryption keys directly via CLI flags (`-secure-key`) may expose secrets in system process listings (`ps`, Task Manager). For secure operations, supply the key using `-key-file` or the `FLOW_SECURE_KEY` (or `SECURE_KEY`) environment variable.

Precedence order for key resolution:
1. `-secure-key` CLI flag (with process exposure warning)
2. `-key-file` (reads secret from file)
3. `FLOW_SECURE_KEY` environment variable
4. `SECURE_KEY` environment variable

Exported files containing decrypted content are automatically restricted to `0600` permissions (owner read/write only).

### Import and Encrypt into Database
```shell
# Recommended: Read key from file
go run mssql_importer.go -dsn "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true" -table pipeline -name "github_ai_credit_usage" -file "github_ai_credit_usage.xml" -encrypted -key-file "/path/to/secret.key"

# Or via environment variable:
export FLOW_SECURE_KEY="MySecretKey123"
go run mssql_importer.go -dsn "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true" -table pipeline -name "github_ai_credit_usage" -file "github_ai_credit_usage.xml" -encrypted
```

### Export from Database and Decrypt to File
```shell
# Export and write with 0600 permissions
go run mssql_importer.go -action export -dsn "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true" -table pipeline -name "github_ai_credit_usage" -file "github_ai_credit_usage_decrypted.xml" -encrypted -key-file "/path/to/secret.key"
```

### Run Encrypted from Database with go-flow
```shell
flow.exe -encrypted -key-file "/path/to/secret.key" -options "sql://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true#SELECT OptionsXML from dbo.flow_options_content where Name = 'OPTIONS_GITHUB_AI_CREDIT_USAGE'"
```
