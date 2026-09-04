# CLI Options & Configuration Guide

This pipeline CLI tool executes data workflows, validates XML pipeline ASTs, and supports custom XSLT transformations. Command options can be passed directly via command-line flags or loaded bulk from an XML file using the `-load` parameter.

---

## Hierarchy & Precedence Rules

Option values are resolved using a strict three-tier hierarchy to ensure command-line invocations can always selectively override preset configuration files:

| Precedence Tier | Source | Description |
| :--- | :--- | :--- |
| **1. Highest** | **Command-Line Flags** | Flags explicitly set at invocation (e.g., `-format json`) take top priority, overriding both XML options and built-in defaults. |
| **2. Medium** | **XML Option File (`-load`)** | Parameters defined in an XML file passed to `-load` fill in any flags that were **not** explicitly set on the command line. |
| **3. Lowest** | **Built-in Defaults** | Hardcoded flag fallbacks defined within the binary application. |

---

## Command-Line Flags Reference

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-load` | `string` | `""` | Path to an XML file containing CLI option defaults. |
| `-file` | `string` | `"scripts.xml"` | Path to the XML file defining scripts, databases, and workflow nodes. |
| `-format` | `string` | `"csv"` | Output formatting (`csv`, `text`, `json`, `jsonpretty`, `markdown`, or `summary`). |
| `-xsd` | `string` | `""` | Optional path to XSD file for schema validation via `xmllint`. |
| `-config` | `string` | `""` | Optional path to `CONFIG.xml` file containing variable and DB overrides. |
| `-validate` | `bool` | `false` | Validates XML schema and AST structure without executing the pipeline. |
| `-preflight` | `bool` | `false` | Executes preflight validation nodes only without running the main pipeline. |
| `-vars` | `string` | `""` | Comma-separated `key=value` runtime overrides (e.g., `-vars "Table=users,Limit=100"`). |
| `-debug` | `bool` | `false` | Enables detailed console execution logging. |
| `-gopath` | `string` | `$GOPATH` | GOPATH directory used for dynamic interpreter package imports. |
| `-xslt` | `string` | `""` | Optional path to custom XSLT stylesheet for XML transformations. |
| `-out` | `string` | `""` | Path to output file when saving transformed XML output from `-xslt`. |

---

## Usage Examples

### 1. Basic Execution (Default Flags)

Displays the help message with available command-line flags:

```bash
./flow -h
```

### 2. Loading Configuration via XML

Populates all CLI parameters directly from an XML configuration file:

```bash
./flow -load cli_options.xml
```

### 3. Overriding XML Loaded Options via CLI

Loads baseline settings from cli_options.xml, but forces output format to jsonpretty and enables debug logging:

```bash
./flow -load cli_options.xml -format jsonpretty -debug
```

## XML Option File Schema (`-load`)

To define CLI parameters in an XML file, use `<flow_cli_options>` as the root element with a nested `<options>` element containing child tags that match flag names:

### Example XML Option File
```xml cli_options.xml
<flow_cli_options>
    <options>
        <file>pipeline_main.xml</file>
        <format>jsonpretty</format>
        <xsd>schemas/pipeline.xsd</xsd>
        <config>environments/prod_config.xml</config>
        <validate>false</validate>
        <preflight>false</preflight>
        <vars>TargetTable=override_table,Threshold=500</vars>
        <debug>true</debug>
        <gopath>/usr/local/go</gopath>
        <xslt>transforms/diagram.xslt</xslt>
        <out>output/transformed.xml</out>
    </options>
</flow_cli_options>
```