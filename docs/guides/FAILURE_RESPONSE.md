# Pipeline Failure Response & Error Propagation

**By default, `go-flow` uses fail-fast execution.** When a node fails, the remaining nodes in the pipeline will **not** continue.

This document details how failure propagation, cooperative cancellation, transactional rollbacks, and explicit recovery blocks work across every orchestration primitive in the `flow` runtime engine.

---

## Summary Matrix

| Orchestration Context | Default Behavior on Node Failure | Remaining Sibling Nodes | Downstream Pipeline Nodes | Special Recovery / Cleanup |
| :--- | :--- | :--- | :--- | :--- |
| **Sequential (`<flow>`, `<preflight>`, `<group>`, `<if>`)** | **Fail-Fast** | Skipped immediately | Skipped | None (unless inside `<assert>`) |
| **Parallel (`<parallel>`)** | **Cooperative Cancel** | Queued tasks skipped; in-flight workers receive `ctx.Done()` | Skipped after `wg.Wait()` | Worker variables only merged after barrier |
| **Loops (`<foreach>`, `<while>`)** | **Abort Loop** | Remaining children in current iteration & future iterations skipped | Skipped | Active DB cursor closed immediately |
| **Transactional Group (`<group transaction="true">`)** | **Rollback & Halt** | Skipped immediately | Skipped | Automatic `ROLLBACK` (or `ROLLBACK TO SAVEPOINT` if nested) |
| **Assertion (`<assert on_failure="halt">`)** | **Run Fallback, Then Halt** | Skipped | Skipped | Executes nested `<on_failure>` child nodes before halting |
| **Assertion (`<assert on_failure="warn">` / `"continue"`)** | **Warn & Continue** | **Executed normally** | **Executed normally** | Sets `fail_var`/`fail_val` and runs `<on_failure>` children |

---

## 1. Sequential Nodes (`<flow>`, `<preflight>`, `<group>`, `<if>`)

In `Executor.executeNodes`, pipeline nodes are evaluated sequentially inside a `for` loop. Each node handler returns a boolean `hasErr`:

* As soon as any node returns `hasErr == true`, `executeNodes` emits the node failure event to registered sinks (`pipeline_events`), appends the `ScriptResult`, and immediately returns `true` to its caller.
* All subsequent sibling nodes and parent containers are bypassed.
* `Executor.executeRun` records the overall run status as `failed` in `pipeline_runs`, and `go-flow` exits with return code `1`.

```mermaid
flowchart TD
    Start(["Start Pipeline Run"]) --> N1["Execute Node 1"]
    N1 --> Check1{"hasErr == true?"}
    Check1 -- "No (Success)" --> N2["Execute Node 2"]
    Check1 -- "Yes (Failed)" --> EmitFail["Emit Node Failed Event<br/>(pipeline_events)"]
    
    N2 --> Check2{"hasErr == true?"}
    Check2 -- "No (Success)" --> N3["Execute Node 3"]
    Check2 -- "Yes (Failed)" --> EmitFail

    N3 -. "Skipped on Node 1/2 Failure" .-> EmitFail
    EmitFail --> FinishRun["Mark Run Status = 'failed'<br/>(pipeline_runs)"]
    FinishRun --> Exit1(["OS Exit(1)"])
```

---

## 2. Parallel Blocks (`<parallel>`)

In `Executor.executeParallelNode`, child nodes run concurrently across a bounded worker pool (`max_threads`, defaulting to `4`) sharing a derived cancellable context (`pCtx, cancel := context.WithCancel(ctx)`).

When any child worker fails (`childErr == true`):
1. **Immediate Context Cancellation**: The failing worker sets an atomic flag (`hasErr.Store(true)`) and invokes `cancel()` on `pCtx`.
2. **Queued Siblings Skipped**: The dispatcher loop checks `hasErr.Load() || pCtx.Err() != nil` before acquiring the semaphore and immediately stops scheduling any remaining unstarted child nodes in the `<parallel>` block.
3. **In-Flight Siblings Notified**: Currently executing sibling goroutines receive the cancellation signal via `pCtx.Done()`, which aborts context-aware operations (such as SQL queries via `QueryContext`/`ExecContext`, shell/CSX scripts via `exec.CommandContext`, HTTP requests, and loops).
4. **Join Barrier & Halt**: `wg.Wait()` waits for all active workers to finish cleaning up. Because `executeParallelNode` returns `true`, **no downstream nodes after `<parallel>` will execute**.

```mermaid
sequenceDiagram
    participant Main as Main Executor
    participant Pool as <parallel> Dispatcher (pCtx)
    participant W1 as Worker 1 (Fails)
    participant W2 as Worker 2 (In-Flight)
    participant W3 as Worker 3 (Queued)

    Main->>Pool: Enter <parallel max_threads="2">
    Pool->>W1: Spawn Worker 1 (pCtx)
    Pool->>W2: Spawn Worker 2 (pCtx)
    Note over W3: Waiting for semaphore slot

    W1--xPool: Node fails (childErr = true)
    Pool->>Pool: hasErr.Store(true) & cancel(pCtx)
    Pool-->>W2: pCtx.Done() signal delivered
    Pool--xW3: Dispatcher breaks loop (Worker 3 never starts)
    W2-->>Pool: Aborts context-aware op & exits
    Pool->>Main: wg.Wait() completes -> returns hasErr = true
    Note over Main: Downstream pipeline nodes skipped
```

---

## 3. Iterative Loops (`<foreach>`, `<while>`)

Both `<foreach>` (`Executor.executeForEachNode`, in both streaming and buffered modes) and `<while>` (`Executor.executeWhileNode`) evaluate their child nodes sequentially on each iteration via `e.executeNodes(ctx, node.Children, results)`.

* If any child node fails during iteration $k$:
  * Remaining child nodes in iteration $k$ are skipped.
  * For `<foreach>`, the active database `sql.Rows` cursor is immediately closed (`rows.Close()`).
  * Subsequent iterations ($k+1, \dots, N$) are aborted.
  * The error bubbles up and halts the rest of the pipeline.

```mermaid
flowchart TD
    LoopStart(["Enter <foreach> / <while>"]) --> Cond{"Next Row /<br/>Condition True?"}
    Cond -- "No" --> LoopDone(["Loop Complete (Continue Pipeline)"])
    Cond -- "Yes" --> Child1["Iteration k: Child Node 1"]
    Child1 --> CheckC1{"hasErr?"}
    CheckC1 -- "No" --> Child2["Iteration k: Child Node 2"]
    CheckC1 -- "Yes" --> AbortLoop["Close DB Cursor (if <foreach>)<br/>Skip Child Node 2 & Future Iterations"]
    Child2 --> CheckC2{"hasErr?"}
    CheckC2 -- "No" --> Cond
    CheckC2 -- "Yes" --> AbortLoop
    AbortLoop --> Halt(["Bubble Error Up & Halt Pipeline"])
```

---

## 4. Transactional Groups (`<group transaction="true">`)

When a `<group>` has `transaction="true"` and a target `db` configured:

* **Top-Level Transaction**: Begins a database transaction (`dbConn.BeginTx`) and pushes it onto the executor's active transaction stack for that database.
* **Nested Transaction on Same DB**: Creates a dialect-aware savepoint (`SAVE TRANSACTION sp_<timestamp>` on SQL Server or `SAVEPOINT sp_<timestamp>` on PostgreSQL, MySQL, SQLite, Oracle).
* **On Child Failure**:
  1. Remaining child nodes inside the `<group>` are skipped.
  2. The `defer` handler detects `!committed` and automatically executes `tx.Rollback()` (or `ROLLBACK TRANSACTION` / `ROLLBACK TO SAVEPOINT` for nested transactions).
  3. The transaction is popped from the stack and the pipeline halts.

```mermaid
flowchart TD
    EnterTx(["Enter <group transaction='true' db='target_db'>"]) --> CheckNested{"Active Tx on<br/>'target_db'?"}
    CheckNested -- "No" --> BeginTx["dbConn.BeginTx()"]
    CheckNested -- "Yes" --> CreateSP["Create Savepoint<br/>(SAVEPOINT / SAVE TRANSACTION)"]

    BeginTx --> RunChildren["Execute Group Child Nodes"]
    CreateSP --> RunChildren

    RunChildren --> ChildCheck{"Any Child Failed?"}
    ChildCheck -- "No (All Succeeded)" --> CommitTx["Commit Tx / Release Savepoint"]
    CommitTx --> ContinuePipe(["Continue Pipeline"])

    ChildCheck -- "Yes (Child Failed)" --> RollbackChoice{"Is Nested?"}
    RollbackChoice -- "No" --> RollbackTx["tx.Rollback()"]
    RollbackChoice -- "Yes" --> RollbackSP["Rollback to Savepoint"]
    RollbackTx --> HaltPipe(["Skip Remaining Nodes & Halt Pipeline"])
    RollbackSP --> HaltPipe
```

---

## 5. Exceptions: When Nodes *Do* Continue (`<assert>`)

The `<assert>` component (`Executor.executeAssertNode`) is the only built-in node that provides explicit control over whether the pipeline halts or continues when a check fails, as well as a dedicated `<on_failure>` fallback block.

```mermaid
flowchart TD
    StartAssert(["Evaluate <assert> Condition"]) --> Eval{"Condition Passed?"}
    Eval -- "Yes" --> Pass(["ReturnCode = 0<br/>Continue Pipeline"])

    Eval -- "No (Assertion Failed)" --> CheckFailVar{"fail_var specified?"}
    CheckFailVar -- "Yes" --> SetVar["Set fail_var = fail_val<br/>(default 'true')"]
    CheckFailVar -- "No" --> CheckBlock{"Has <on_failure><br/>child nodes?"}
    SetVar --> CheckBlock

    CheckBlock -- "Yes" --> RunFallback["Execute <on_failure> Child Nodes<br/>(e.g., Cleanup SQL, Slack Alert)"]
    CheckBlock -- "No" --> Policy{"on_failure attribute?"}
    RunFallback --> Policy

    Policy -- "'warn' or 'continue'" --> WarnContinue(["Log Warning (ReturnCode = 0)<br/>CONTINUE Remaining Pipeline Nodes"])
    Policy -- "'halt' or default" --> HaltPipe(["Log Critical Failure<br/>HALT Remaining Pipeline Nodes"])
```

### Option A: Continue Execution on Failure (`on_failure="warn"` or `"continue"`)
Setting `on_failure="warn"` or `on_failure="continue"` logs a warning message, optionally sets a flag variable (`fail_var` / `fail_val`), executes any nested `<on_failure>` child nodes, and **allows all subsequent pipeline nodes to continue**.

```xml
<assert id="check_optional_feed"
        var="FEED_STATUS"
        equals="200"
        on_failure="warn"
        fail_var="USING_CACHED_FEED"
        fail_val="true"
        message="External feed unavailable; falling back to cache." />

<!-- Remaining nodes WILL continue executing -->
<if var="USING_CACHED_FEED" equals="true">
    <then>
        <sql id="load_from_cache" db="staging_db">
            INSERT INTO active_rates SELECT * FROM cached_rates;
        </sql>
    </then>
</if>
```

### Option B: Execute Cleanup Nodes Before Halting (`<on_failure>` with `on_failure="halt"`)
Even when `on_failure="halt"` (the default), any child nodes nested inside `<on_failure>...</on_failure>` **will** execute when the assertion fails, allowing you to run cleanup SQL or send alerts before the main pipeline halts.

```xml
<assert id="verify_row_threshold"
        var="ROW_COUNT"
        equals="1000"
        on_failure="halt"
        message="Row count mismatch!">
    <on_failure>
        <!-- These fallback nodes WILL execute before the pipeline halts -->
        <sql id="cleanup_staging" db="staging_db">
            TRUNCATE TABLE stg_import;
        </sql>
    </on_failure>
</assert>

<!-- This node will NOT execute because on_failure="halt" -->
<sql id="promote_to_prod" db="prod_db">
    INSERT INTO fact_orders SELECT * FROM stg_import;
</sql>
```
