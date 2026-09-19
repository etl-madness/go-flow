# DB Importer Usage

To import an XML file into the database, use the following command:

## Import Pipeline

```shell
go run main.go -dsn "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true" -table pipeline -name "github_ai_credit_usage" -file "github_ai_credit_usage.xml" -desc "GitHub Copilot Billing Usage Pipeline"
```

## Import Options

```shell
go run main.go -dsn "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true" -table options -name "github_ai_credit_usage" -file "OPTIONS_GITHUB_AI_CREDIT_USAGE.xml" -desc "GitHub Copilot Billing Usage Options"
```

## Import Config

```shell
go run main.go -dsn "sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true" -table config -name "github_ai_credit_usage" -file "CONFIG_GITHUB_AI_CREDIT_USAGE.xml" -desc "GitHub Copilot Billing Usage Config"
```

## Run from Database

To run the flow from the database using the imported options, use the following command:

```shell
flow.exe -options "sql://sqlserver@sqlserver://sa:Password123!@localhost:1433?database=master&trustServerCertificate=true#SELECT OptionsXML from dbo.flow_options_content where Name = 'OPTIONS_GITHUB_AI_CREDIT_USAGE'" 
```