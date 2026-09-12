# Pipeline Specification & Flowchart .\.private\gcloud_billing.xml

## Execution Flow Diagram

```mermaid
flowchart TD
    subgraph VarsBox ["📋 Pipeline Variables"]
        vars_node["• <b>full_table_path</b> <i>(string)</i>: <code></code><br/>• <b>database1_connection_string</b> <i>(string)</i>: <code>sqlserver://T15P:1433?database=P...</code><br/>• <b>log_db_cs</b> <i>(string)</i>: <code>sqlserver://sa:Password123!@loca...</code><br/>• <b>GCP_PROJECT_ID</b> <i>(string)</i>: <code>osdiscovery-antigravity</code><br/>• <b>BQ_BILLING_TABLE</b> <i>(string)</i>: <code>osdiscovery-antigravity.dsetbill...</code><br/>• <b>GOOGLE_APPLICATION_CREDENTIALS</b> <i>(string)</i>: <code>C:/Users/U00001/keys/gcp-key.json</code>"]
    end

    subgraph DBBox ["🗄️ Configured Databases"]
        db_database1[("Database: database1")]
        db_log_db[("Database: log_db")]
    end

    subgraph PreflightBox ["✈️ Preflight Flow"]
        PreflightStart([Start Preflight])
    sql_preflight_check_1["sql_preflight_check_1<br/>(SQL)"]
    PreflightStart --> sql_preflight_check_1
    sql_preflight_check_2["sql_preflight_check_2<br/>(SQL)"]
    sql_preflight_check_1 --> sql_preflight_check_2
    assert_preflight_check_1["assert_preflight_check_1<br/>(ASSERT)"]
    sql_preflight_check_2 --> assert_preflight_check_1
    assert_preflight_check_2["assert_preflight_check_2<br/>(ASSERT)"]
    assert_preflight_check_1 --> assert_preflight_check_2
        assert_preflight_check_2 --> PreflightEnd([End Preflight])
    end

    subgraph FlowBox ["⚡ Main Execution Flow"]
        Start([Start Pipeline])
    extract_billing_data_gcloud["extract_billing_data_gcloud<br/>(BASH)"]
    Start --> extract_billing_data_gcloud
    load_billing_data_gcloud["load_billing_data_gcloud<br/>(SQL)"]
    extract_billing_data_gcloud --> load_billing_data_gcloud
        load_billing_data_gcloud --> End_1([End Pipeline])
    end
```

## Configured Variables

| Name | Type | Default Value |
|---|---|---|
| **full_table_path** | `string` | `` |
| **database1_connection_string** | `string` | `sqlserver://T15P:1433?database=PROTO&integrated+security=true&trustServerCertificate=true` |
| **log_db_cs** | `string` | `sqlserver://sa:Password123!@localhost:1433?database=database1&trustServerCertificate=true` |
| **GCP_PROJECT_ID** | `string` | `osdiscovery-antigravity` |
| **BQ_BILLING_TABLE** | `string` | `osdiscovery-antigravity.dsetbilling_exports.gcp_billing_export_resource_v1_01612D_F4426C_7251BC` |
| **GOOGLE_APPLICATION_CREDENTIALS** | `string` | `C:/Users/U00001/keys/gcp-key.json` |

## Configured Databases

| Alias Name | Connection String / Variable Reference |
|---|---|
| **database1** | `{{database1_connection_string}}` |
| **log_db** | `{{log_db_cs}}` |

## SCRIPTS

| Language | ID/Name | XPath Location | Source Database | Target Database | Target Table | Batch Size | Value |
|---|---|---|---|---|---|---|---|
| **** | **sql_preflight_check_1** | `/Q{}pipeline[1]/Q{}preflight[1]/Q{}sql[1]` | `database1` | `database1` | `` | `` | <code>SELECT 1</code> |
| **** | **sql_preflight_check_2** | `/Q{}pipeline[1]/Q{}preflight[1]/Q{}sql[2]` | `database1` | `database1` | `` | `` | <code>SELECT CURRENT_USER;</code> |
| **** | **assert_preflight_check_1** | `/Q{}pipeline[1]/Q{}preflight[1]/Q{}assert[1]` | `` | `` | `` | `` | <code></code> |
| **** | **assert_preflight_check_2** | `/Q{}pipeline[1]/Q{}preflight[1]/Q{}assert[2]` | `` | `` | `` | `` | <code></code> |
| **bash** | **extract_billing_data_gcloud** | `/Q{}pipeline[1]/Q{}flow[1]/Q{}script[1]` | `` | `` | `` | `` | <code>./bqBilling.exe -GCP_PROJECT_ID="{{GCP_PROJECT_ID}}" -BQ_BILLING_TABLE="{{BQ_BILLING_TABLE}}" -GOOGLE_APPLICATION_CREDENTIALS="{{GOOGLE_APPLICATION_CREDENTIALS}}"</code> |
| **** | **load_billing_data_gcloud** | `/Q{}pipeline[1]/Q{}flow[1]/Q{}sql[1]` | `database1` | `database1` | `` | `` | <code>INSERT INTO dbo.GcpBillingExport<br/>SELECT<br/>-- Root Identifiers<br/>BillingAccountID          = billing_account_id,<br/>-- Service Info<br/>ServiceID                 = service_id,<br/>ServiceDescription       = service_description,<br/>-- SKU Info<br/>SkuID                     = sku_id,<br/>SkuDescription           = sku_description,<br/>-- Timestamps<br/>UsageStartTime            = usage_start_time,<br/>UsageEndTime              = usage_end_time,<br/>ExportTime                = export_time,<br/>-- Project Info<br/>ProjectID                 = project_id,<br/>ProjectName               = project_name,<br/>ProjectNumber             = project_number,<br/>ProjectAncestryNumbers   = project_ancestry_numbers,<br/>ProjectLabels             = project_labels,<br/>-- Location Info<br/>Location                  = location_location,<br/>LocationCountry           = location_country,<br/>LocationRegion            = location_region,<br/>LocationZone              = location_zone,<br/>-- Financials<br/>Cost                      = cost,<br/>CostAtList                = cost_at_list,<br/>CostType                  = cost_type,<br/>Currency                  = currency,<br/>CurrencyConversionRate    = currency_conversion_rate,<br/>-- Usage Data<br/>UsageAmount               = usage_amount,<br/>UsageUnit                 = usage_unit,<br/>UsageAmountInPricingUnits = usage_amount_in_pricing_units,<br/>UsagePricingUnit          = usage_pricing_unit,<br/>-- Invoice<br/>InvoiceMonth              = invoice_month,<br/>-- Resource Info<br/>ResourceName              = resource_name,<br/>ResourceGlobalName        = resource_global_name,<br/>-- Adjustment Info<br/>AdjustmentID              = adjustment_id,<br/>AdjustmentDescription     = adjustment_description,<br/>AdjustmentMode            = adjustment_mode,<br/>AdjustmentType            = adjustment_type,<br/>-- Nested Array Fields (Preserved as Raw JSON Strings)<br/>Labels                    = labels,<br/>SystemLabels              = system_labels,<br/>Credits                   = credits,<br/>Tags                      = tags<br/>FROM OPENJSON('{{GCLOUD_BILLING_JSON}}')<br/>WITH (<br/>billing_account_id            VARCHAR(32)           '$.billing_account_id',<br/>usage_start_time              DATETIMEOFFSET        '$.usage_start_time',<br/>usage_end_time                DATETIMEOFFSET        '$.usage_end_time',<br/>export_time                   DATETIMEOFFSET        '$.export_time',<br/>cost                          DECIMAL(18,6)         '$.cost',<br/>currency                      VARCHAR(10)           '$.currency',<br/>currency_conversion_rate      DECIMAL(18,6)         '$.currency_conversion_rate',<br/>cost_type                     VARCHAR(50)           '$.cost_type',<br/>cost_at_list                  DECIMAL(18,6)         '$.cost_at_list',<br/>-- Nested Object Paths<br/>service_id                    VARCHAR(64)           '$.service.id',<br/>service_description           NVARCHAR(255)         '$.service.description',<br/>sku_id                        VARCHAR(64)           '$.sku.id',<br/>sku_description               NVARCHAR(255)         '$.sku.description',<br/>project_id                    NVARCHAR(100)         '$.project.id',<br/>project_name                  NVARCHAR(100)         '$.project.name',<br/>project_number                VARCHAR(32)           '$.project.number',<br/>project_ancestry_numbers      NVARCHAR(255)         '$.project.ancestry_numbers',<br/>location_location             NVARCHAR(100)         '$.location.location',<br/>location_country              NVARCHAR(100)         '$.location.country',<br/>location_region               NVARCHAR(100)         '$.location.region',<br/>location_zone                 NVARCHAR(100)         '$.location.zone',<br/>usage_amount                  FLOAT                 '$.usage.amount',<br/>usage_unit                    NVARCHAR(50)          '$.usage.unit',<br/>usage_amount_in_pricing_units FLOAT                 '$.usage.amount_in_pricing_units',<br/>usage_pricing_unit            NVARCHAR(50)          '$.usage.pricing_unit',<br/>invoice_month                 VARCHAR(6)            '$.invoice.month',<br/>resource_name                 NVARCHAR(255)         '$.resource.name',<br/>resource_global_name          NVARCHAR(1000)        '$.resource.global_name',<br/>adjustment_id                 NVARCHAR(100)         '$.adjustment_info.id',<br/>adjustment_description        NVARCHAR(255)         '$.adjustment_info.description',<br/>adjustment_mode               NVARCHAR(50)          '$.adjustment_info.mode',<br/>adjustment_type               NVARCHAR(50)          '$.adjustment_info.type',<br/>-- Array Paths (AS JSON preserves array structures)<br/>project_labels                NVARCHAR(MAX)         '$.project.labels' AS JSON,<br/>labels                        NVARCHAR(MAX)         '$.labels'         AS JSON,<br/>system_labels                 NVARCHAR(MAX)         '$.system_labels'  AS JSON,<br/>credits                       NVARCHAR(MAX)         '$.credits'        AS JSON,<br/>tags                          NVARCHAR(MAX)         '$.tags'           AS JSON<br/>);<br/>SELECT  @@ROWCOUNT;</code> |

