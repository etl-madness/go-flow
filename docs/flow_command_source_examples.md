# Flow CLI source examples for file, HTTP, and SQL DSN inputs

The `flow.exe` command supports loading pipeline source content from multiple source types:

- Local filesystem paths
- HTTP and HTTPS URLs
- SQL / database DSNs using `sql://...` or `db://...`

This is useful for the main pipeline file, the options file, and the config override file.

## Supported source formats

### Local filesystem

```powershell
.\flow.exe --file .\examples\CONFIG.xml
.\flow.exe --file .\examples\github_ai_credit_usage.xml --options .\examples\options_github_ai_credit_usage.xml
.\flow.exe --file .\examples\CONFIG.xml --config .\examples\production_config.xml
```

### HTTP / HTTPS

```powershell
.\flow.exe --file "https://example.com/pipeline.xml"
.\flow.exe --file "https://example.com/pipeline.xml" --options "https://example.com/options.xml"
.\flow.exe --file "https://example.com/pipeline.xml" --config "https://example.com/config.xml"
```

For Basic auth, the `Authorization` header value can be embedded directly in the URL itself instead of using `user:pass` syntax. Example:

```powershell
.\flow.exe --file "https://Basic%20dXNlcjpwYXNz@example.com/pipeline.xml"
```

This is translated to an HTTP header of:

```text
Authorization: Basic dXNlcjpwYXNz
```

### SQL / database DSN

The SQL URI format is:

```text
sql://<driver>@<dsn>#<SELECT ...>
```

Examples:

```powershell
.\flow.exe --file "sql://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true#SELECT XmlContent FROM dbo.flow_pipeline WHERE Name = 'PIPELINE_MAIN'"

.\flow.exe --options "sql://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true#SELECT OptionsXML FROM dbo.flow_options_content WHERE Name = 'OPTIONS_PROD'"

.\flow.exe --config "sql://postgres@postgres://app_user:Password123!@prod-db:5432/appdb?sslmode=require#SELECT config_xml FROM flow_config WHERE name = 'PROD'"
```

SQL Server with Windows integrated security / trusted connection:

```powershell
.\flow.exe --options "sql://sqlserver@sqlserver://T15P:1433?database=PROTO&integrated+security=true&trustServerCertificate=true#SELECT OptionsXML FROM dbo.flow_options_content WHERE Name = 'OPTIONS_GITHUB_AI_CREDIT_USAGE'"
```

This pattern is useful when the SQL Server instance is running on a named host and you want to authenticate using a trusted Windows login instead of a username/password in the DSN itself.

### Database alias form

The repository also supports a `db://` alias form:

```powershell
.\flow.exe --file "db://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true#SELECT XmlContent FROM dbo.flow_pipeline WHERE Name = 'PIPELINE_MAIN'"

.\flow.exe --options "db://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true#SELECT OptionsXML FROM dbo.flow_options_content WHERE Name = 'OPTIONS_GLOBAL'"
```

## Example with all three in one command

```powershell
.\flow.exe `
  --file "sql://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true#SELECT XmlContent FROM dbo.flow_pipeline WHERE Name = 'PIPELINE_MAIN'" `
  --config "https://example.com/config/prod.xml" `
  --options "sql://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true#SELECT OptionsXML FROM dbo.flow_options_content WHERE Name = 'OPTIONS_PROD'" `
  --debug
```

## Notes

- Put DSN/URI values in double quotes so `&`, `?`, and `#` are preserved correctly by the shell.
- Use `sql://` or `db://` for database-backed XML content.
- Use `http://` or `https://` for remote XML content.
- Use a plain path for local files.
- The `options`, `config`, and `file` arguments all follow the same source-loading rules.

## Typical production usage

```powershell
.\flow.exe --file "sql://sqlserver@sqlserver://sqlprod:1433?database=ETL&encrypt=true&trustServerCertificate=true#SELECT XmlContent FROM dbo.flow_pipeline WHERE Name = 'PIPELINE_FINANCE'" --config "sql://sqlserver@sqlserver://sqlprod:1433?database=ETL&encrypt=true&trustServerCertificate=true#SELECT ConfigXML FROM dbo.flow_config WHERE Name = 'PROD'" --options "sql://sqlserver@sqlserver://sqlprod:1433?database=ETL&encrypt=true&trustServerCertificate=true#SELECT OptionsXML FROM dbo.flow_options_content WHERE Name = 'OPTIONS_FINANCE'"
```
