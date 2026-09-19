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

