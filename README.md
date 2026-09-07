# Flow: Modern, Lightweight ETL & Workflow Orchestration Engine

**Flow** is a single-binary, high-performance ETL execution engine and workflow orchestrator built in Go. Designed as a modern alternative to bloated legacy ETL tools (e.g., SSIS) and complex code-heavy orchestrators (e.g., Apache Airflow), Flow combines declarative XML pipelines, an embedded visual HTMX web builder, native cross-database execution, and automated compliance telemetry.

Whether you are looking to streamline database batch jobs, replace fragile shell/Python scripts, or enforce strict audit logging across heterogeneous databases, Flow delivers zero-dependency pipeline execution with enterprise-grade reliability.

---

## Why Flow?

| Feature | Legacy ETL (SSIS, Informatica) | Code-Based Orchestrators (Airflow, Dagster) | **Flow Engine** |
| --- | --- | --- | --- |
| **Footprint & Dependencies** | Massive server footprint, complex runtimes | Heavy Python environments, Celery/Redis setups | **Single Go binary with zero external runtime dependencies**. |
| **Authoring Experience** | Heavy desktop GUIs, proprietary XML | Python code base, steep learning curve | **Embedded Web UI (`-builder`) or Git-friendly declarative XML**. |
| **Database Abstraction** | Manual connector setups per DB | Requires platform-specific Python operators | **Native dialect abstraction (Postgres, SQL Server, Oracle, MySQL, SQLite, K/V)**. |
| **Audit & Row Telemetry** | Fragmented server logs, custom logging jobs | Requires custom XComs/hooks for metrics | **Automatic step-level audit tables (`pipeline_events`, `pipeline_runs`) with row counts**. |
| **Documentation** | Hand-maintained Wiki docs (frequently stale) | Code-as-docs (hard for non-coders) | **Auto-generated Mermaid.js visual flowcharts & XSLT transformation**. |

---

## Core Capabilities

### 1. Visual Web UI & Git-Friendly Declarative Pipelines

* **Embedded HTMX Builder:** Launch an interactive local web interface using `-builder` (port `8080`) backed by `flow_builder.db` to visually design and manage workflows.


* **Clean XML Schema:** Store pipeline configurations in XML for standard version control, code reviews, and automated CI/CD deployment.


* **Hierarchical Overrides:** Merge base pipeline files with environment-specific overrides (`-config`) and runtime CLI key-value pairs (`-vars "Table=orders,BatchSize=5000"`).


* **Pre-Baked CLI Profiles:** Pre-populate CLI flags from XML option files using `-options` without overriding command-line arguments.



### 2. Multi-Engine & Cross-Dialect Database Support

* **Relational & Key-Value Stores:** Connect to PostgreSQL, Microsoft SQL Server, MySQL, SQLite, Oracle, and Key-Value stores out of the box.


* **Dynamic Parameter Translation:** Automatically translates SQL query placeholders across database dialects (`$1` for Postgres, `@p1` for MSSQL, `:1` for Oracle, `?` for MySQL/SQLite/KV).


* **Automated Connection Expansion:** Dynamically resolves connection string variables (e.g., `{{log_db_cs}}`) across base XML, overrides, and CLI arguments up to 3 levels deep.



### 3. Advanced Orchestration Primitives

* **Parallel Execution (`<parallel>`):** Run concurrent data tasks across multiple worker threads and join them automatically.


* **Conditional Logic (`<if>`):** Route pipeline execution dynamically based on variables or condition strings (`<then>` / `<else>`).


* **Iterative Loops (`<foreach>`, `<loop>`, `<while>`):** Iterate over row results or dataset criteria to execute child tasks repeatedly.


* **Preflight Execution (`<preflight>`):** Run diagnostic tasks (e.g., source file availability, target table truncations) before triggering main data flows.



### 4. Built-in Audit Telemetry & Observability

* **`pipeline_runs` Table:** Records run IDs, status (`succeeded`/`failed`), duration in milliseconds, task count, error classes, and error messages.


* **`pipeline_events` Table:** Tracks step-by-step sequence numbers, timestamps, node IDs, execution status, and precise row counts (`rows_read`, `rows_written`, `rows_affected`).


* **Multi-Sink Fanout:** Stream logs concurrently to stdout (`TextSink`) and audit databases (`DatabaseSink`) using `MultiSink`.



### 5. Self-Documenting Pipelines

* **Mermaid.js Diagram Generation:** Automatically parses pipeline structures into `flowchart TD` diagrams illustrating variable boxes, database schemas, parallel joins, and loop branches.


* **XSLT 3.0 Processing Engine:** Features an integrated stylesheet transformer (`helium`/`xslt3`) to embed generated diagrams and metadata directly into custom documentation or HTML reports.



---

## Quick Start

### 1. Launch the Visual Builder UI

Start the interactive HTMX web builder to create or inspect pipeline jobs visually:

```bash
flow -builder -builder-port 8080

```

### 2. Validate a Pipeline Configuration

Validate XML structural schemas (XSD) and AST node integrity without executing data operations:

```bash
flow -file production_etl.xml -validate

```

### 3. Execute Preflight Checks Only

Run pre-flight validation nodes to verify prerequisites:

```bash
flow -file production_etl.xml -preflight

```

### 4. Run Pipeline with Runtime Overrides & Telemetry

Execute a pipeline with custom variable overrides, an environment override config, and stream live execution telemetry:

```bash
flow -file pipelines/daily_etl.xml \
     -config config/prod.xml \
     -vars "BatchID=20260907,TargetSchema=dw_staging" \
     -format stream \
     -debug

```

---

## Pipeline Structure Example (`scripts.xml`)

```xml
<pipeline>
    <!-- Pipeline Variables -->
    <variables>
        <variable name="TargetTable" type="string" value="stg_customer_orders" />
        <variable name="BatchSize" type="int" value="10000" />
    </variables>

    <!-- Registered Database Connections -->
    <databases>
        <database name="source_db" connection_string="postgres://user:pass@localhost:5432/crm" driver="postgres" />
        <database name="target_db" connection_string="server=localhost;database=DW;user id=sa;password=pass;" driver="sqlserver" />
        <database name="log_db" connection_string="{{log_db_cs}}" />
    </databases>

    <!-- Preflight Sanitization Checks -->
    <preflight>
        <sql id="chk_staging_ready">
            SELECT 1 FROM sys.tables WHERE name = 'stg_customer_orders';
        </sql>
    </preflight>

    <!-- Main Execution Flow -->
    <flow>
        <!-- Parallel Data Loading -->
        <parallel id="parallel_ingest">
            <script id="load_customers" language="sql" target_db="target_db">
                SELECT customer_id, email, updated_at FROM customers;
            </script>
            <script id="load_orders" language="sql" target_db="target_db">
                SELECT order_id, customer_id, total_amount FROM orders;
            </script>
        </parallel>

        <!-- Conditional Execution -->
        <if var="BatchSize" equals="10000">
            <then>
                <sql_bulk id="bulk_insert_dw" target_db="target_db">
                    INSERT INTO dw.fact_orders SELECT * FROM stg_customer_orders;
                </sql_bulk>
            </then>
            <else>
                <script id="log_small_batch" language="go">
                    println("Processing small batch mode...")
                </script>
            </else>
        </if>
    </flow>
</pipeline>

```

---

## Command Line Interface Reference

| Parameter | Type | Default | Description |
| --- | --- | --- | --- |
| `-builder` | `bool` | `false` | Starts the embedded HTMX pipeline builder web server. |
| `-builder-port` | `int` | `8080` | Specifies the port for the builder web server. |
| `-file` | `string` | `scripts.xml` | Path to the XML file containing scripts, variables, and databases. |
| `-config` | `string` | `""` | Optional path to an override XML config file (variables, databases). |
| `-options` | `string` | `""` | XML file containing pre-configured default CLI option parameters. |
| `-vars` | `string` | `""` | Comma-separated runtime overrides (e.g., `-vars "Table=foo,Limit=100"`). |
| `-validate` | `bool` | `false` | Performs XSD schema and AST validation checks without running the job. |
| `-preflight` | `bool` | `false` | Executes preflight validation nodes only. |
| `-format` | `string` | `csv` | Result output format: `json`, `jsonpretty`, `text`, `markdown`, `csv`, `stream`. |
| `-xsd` | `string` | `""` | Optional XSD schema file path for XML structure validation. |
| `-xslt` | `string` | `""` | Optional XSLT stylesheet path for pipeline XML transformation. |
| `-out` | `string` | `""` | Output file path for transformed XML when using XSLT. |
| `-debug` | `bool` | `false` | Enables verbose console logging. |
| `-gopath` | `string` | `$GOPATH` | GOPATH directory for dynamic Go interpreter imports. |

---

## Operational Audit Database Schemas

Note: database specific schema definitions and SQL dialects may vary. Detailed instructions for various database engines can be found in the following documentation (./docs/db_logging_setup.md).
When a `log_db` database entry is provided in the configuration, Flow automatically creates connection pools and writes telemetry to two main tables:

### 1. `pipeline_runs` (Run-Level Execution Summary)

Stores executive-level status and runtime metrics per job invocation:

```sql
CREATE TABLE pipeline_runs (
    run_id        VARCHAR(64) PRIMARY KEY,
    file_path     VARCHAR(255),
    config_path   VARCHAR(255),
    status        VARCHAR(32),   -- 'succeeded', 'failed'
    started_at    TIMESTAMP,
    finished_at   TIMESTAMP,
    duration_ms   BIGINT,
    task_count    INT,
    error_class   VARCHAR(64),
    error_message TEXT
);

```

### 2. `pipeline_events` (Step-Level Audit & Data Lineage)

Tracks step-level state transitions and row counts for every task node executed in the workflow:

```sql
CREATE TABLE pipeline_events (
    run_id        VARCHAR(64),
    execution_id  VARCHAR(64),
    sequence_num  INT,
    occurred_at   TIMESTAMP,
    event_type    VARCHAR(64),
    node_kind     VARCHAR(32),   -- 'script', 'sql', 'parallel', 'if', etc.
    node_id       VARCHAR(64),
    status        VARCHAR(32),   -- 'started', 'completed', 'failed'
    error_message TEXT,
    rows_read     BIGINT,
    rows_written  BIGINT,
    rows_affected BIGINT
);

```