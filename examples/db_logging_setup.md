# Database Logging Setup for Pipeline Runs and Events

If package has a database configured with the name `log_db`, logging to that database will be automatically set up using the schemas defined below.

## Example Usage

```xml

<pipeline xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:noNamespaceSchemaLocation="https://raw.githubusercontent.com/etl-madness/flow/main/xsd/pipeline.xsd">
    <databases>
        <database 
            name="log_db" 
            driver="sqlserver" 
            connection_string="sqlserver://sa:StrongPassword123!@localhost:1433?database=pipeline_telemetry&amp;encrypt=disable" />
    </databases>

    <flow>
        <!-- Pipeline scripts -->
    </flow>
</pipeline>
```

## PostgreSQL Schema

```sql
CREATE TABLE IF NOT EXISTS pipeline_runs (
    run_id VARCHAR(64) PRIMARY KEY,
    file_path VARCHAR(255) NOT NULL,
    config_path VARCHAR(255),
    status VARCHAR(32) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NOT NULL,
    duration_ms BIGINT NOT NULL,
    task_count INT NOT NULL,
    error_class VARCHAR(128),
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pipeline_events (
    id BIGSERIAL PRIMARY KEY,
    run_id VARCHAR(64) NOT NULL,
    execution_id VARCHAR(64) NOT NULL,
    sequence_num INT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    node_kind VARCHAR(64) NOT NULL,
    node_id VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    rows_read BIGINT DEFAULT 0,
    rows_written BIGINT DEFAULT 0,
    rows_affected BIGINT DEFAULT 0
);
```

## MySQL / MariaDB

```sql
CREATE TABLE IF NOT EXISTS pipeline_runs (
    run_id VARCHAR(64) PRIMARY KEY,
    file_path VARCHAR(255) NOT NULL,
    config_path VARCHAR(255),
    status VARCHAR(32) NOT NULL,
    started_at DATETIME(3) NOT NULL,
    finished_at DATETIME(3) NOT NULL,
    duration_ms BIGINT NOT NULL,
    task_count INT NOT NULL,
    error_class VARCHAR(128),
    error_message TEXT,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3)
);

CREATE TABLE IF NOT EXISTS pipeline_events (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    run_id VARCHAR(64) NOT NULL,
    execution_id VARCHAR(64) NOT NULL,
    sequence_num INT NOT NULL,
    occurred_at DATETIME(3) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    node_kind VARCHAR(64) NOT NULL,
    node_id VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    rows_read BIGINT DEFAULT 0,
    rows_written BIGINT DEFAULT 0,
    rows_affected BIGINT DEFAULT 0
);
```

## SQLite Schema

```sql
CREATE TABLE IF NOT EXISTS pipeline_runs (
    run_id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    config_path TEXT,
    status TEXT NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT NOT NULL,
    duration_ms INTEGER NOT NULL,
    task_count INTEGER NOT NULL,
    error_class TEXT,
    error_message TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pipeline_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL,
    execution_id TEXT NOT NULL,
    sequence_num INTEGER NOT NULL,
    occurred_at TEXT NOT NULL,
    event_type TEXT NOT NULL,
    node_kind TEXT NOT NULL,
    node_id TEXT NOT NULL,
    status TEXT NOT NULL,
    error_message TEXT,
    rows_read INTEGER DEFAULT 0,
    rows_written INTEGER DEFAULT 0,
    rows_affected INTEGER DEFAULT 0
);
```

## Microsoft SQL Server

```sql
CREATE TABLE pipeline_runs (
    run_id VARCHAR(64) PRIMARY KEY,
    file_path VARCHAR(255) NOT NULL,
    config_path VARCHAR(255),
    status VARCHAR(32) NOT NULL,
    started_at DATETIMEOFFSET NOT NULL,
    finished_at DATETIMEOFFSET NOT NULL,
    duration_ms BIGINT NOT NULL,
    task_count INT NOT NULL,
    error_class VARCHAR(128),
    error_message NVARCHAR(MAX),
    created_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
);

CREATE TABLE pipeline_events (
    id BIGINT IDENTITY(1,1) PRIMARY KEY,
    run_id VARCHAR(64) NOT NULL,
    execution_id VARCHAR(64) NOT NULL,
    sequence_num INT NOT NULL,
    occurred_at DATETIMEOFFSET NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    node_kind VARCHAR(64) NOT NULL,
    node_id VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message NVARCHAR(MAX),
    rows_read BIGINT DEFAULT 0,
    rows_written BIGINT DEFAULT 0,
    rows_affected BIGINT DEFAULT 0
);
```

## Oracle

```sql
CREATE TABLE pipeline_runs (
    run_id VARCHAR2(64) PRIMARY KEY,
    file_path VARCHAR2(255) NOT NULL,
    config_path VARCHAR2(255),
    status VARCHAR2(32) NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    finished_at TIMESTAMP WITH TIME ZONE NOT NULL,
    duration_ms NUMBER(19) NOT NULL,
    task_count NUMBER(10) NOT NULL,
    error_class VARCHAR2(128),
    error_message CLOB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE pipeline_events (
    id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id VARCHAR2(64) NOT NULL,
    execution_id VARCHAR2(64) NOT NULL,
    sequence_num NUMBER(10) NOT NULL,
    occurred_at TIMESTAMP WITH TIME ZONE NOT NULL,
    event_type VARCHAR2(64) NOT NULL,
    node_kind VARCHAR2(64) NOT NULL,
    node_id VARCHAR2(128) NOT NULL,
    status VARCHAR2(32) NOT NULL,
    error_message CLOB,
    rows_read NUMBER(19) DEFAULT 0,
    rows_written NUMBER(19) DEFAULT 0,
    rows_affected NUMBER(19) DEFAULT 0
);
```

