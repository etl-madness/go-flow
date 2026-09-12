package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/etl-madness/flow"
)

func TestMermaidGenerationWithV1231Nodes(t *testing.T) {
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<pipeline>
    <databases>
        <database name="main_db" driver="sqlite" connection_string=":memory:" />
        <database name="dw_db" driver="postgres" connection_string="postgresql://user:pass@localhost:5432/dw" />
    </databases>
    <flow>
        <sql id="extract_summary" db="main_db">
            SELECT * FROM raw_records;
        </sql>
        <sql_bulk id="bulk_replicate" db="main_db" target_db="dw_db" target_table="replicated_records">
            SELECT * FROM raw_records;
        </sql_bulk>
        <excel_write id="export_excel" db="dw_db" file="./reports/sales.xlsx" sheet="Summary">
            SELECT * FROM replicated_records;
        </excel_write>
        <excel_read id="read_excel" file="./reports/sales.xlsx" sheet="Summary" output_var="ReadData" />
        <html_template id="render_report" file="./reports/summary.html">
            <h1>Report</h1>
        </html_template>
        <kv id="cache_summary" db="main_db" bucket="cache" key="last_run" value="2026-09-10" />
    </flow>
</pipeline>`

	diagram, err := generateMermaid([]byte(xmlContent))
	if err != nil {
		t.Fatalf("failed to generate mermaid: %v", err)
	}
	if diagram == nil {
		t.Fatal("expected diagram output, got nil")
	}

	result := *diagram

	expectedSubstrings := []string{
		"extract_summary",
		"bulk_replicate",
		"➔ Stream to dw_db",
		"export_excel",
		"📄 ./reports/sales.xlsx",
		"read_excel",
		"render_report",
		"cache_summary",
		"Start --> extract_summary",
		"extract_summary --> bulk_replicate",
		"bulk_replicate --> export_excel",
		"export_excel --> read_excel",
		"read_excel --> render_report",
		"render_report --> cache_summary",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(result, expected) {
			t.Errorf("expected diagram to contain %q, but got:\n%s", expected, result)
		}
	}
}

func TestValidateXSDWithV1231Features(t *testing.T) {
	xsdPath := filepath.Join("xsd", "pipeline.xsd")
	if _, err := os.Stat(xsdPath); os.IsNotExist(err) {
		t.Fatalf("xsd file not found at %s", xsdPath)
	}

	pipelineXML := `<?xml version="1.0" encoding="UTF-8"?>
<pipeline xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:noNamespaceSchemaLocation="pipeline.xsd">
    <databases>
        <database name="src_db" driver="sqlite" connection_string=":memory:" workload="bulk" />
        <database name="dest_db" driver="sqlite" connection_string=":memory:" workload="oltp" />
    </databases>
    <flow>
        <group id="tx_group" tx="true" db="src_db" timeout="30s">
            <sql id="init_sql" db="src_db" timeout="15s" output_var="ROW_COUNT">
                CREATE TABLE IF NOT EXISTS items (id INT, name TEXT);
            </sql>
        </group>

        <foreach id="loop_items" db="src_db" var="item_id" stream="true" buffer="false" mode="stream">
            <sql id="process_item" db="src_db">
                UPDATE items SET processed = 1 WHERE id = 1;
            </sql>
        </foreach>

        <sql_bulk id="bulk_load" db="src_db" target_db="dest_db" target_table="dest_items"
                  batch_size="1000" tablock="true" timeout="5m" output_var="BULK_ROWS">
            SELECT id, name FROM items;
        </sql_bulk>

        <excel_write id="write_report" database="src_db" file="report.xlsx" sheet="Data">
            SELECT id, name FROM items;
        </excel_write>

        <html_template id="report_html" file="report.html">
            <![CDATA[<h1>Report Done</h1>]]>
        </html_template>
    </flow>
</pipeline>`

	tmpDir := t.TempDir()
	xmlFile := filepath.Join(tmpDir, "pipeline_v1231.xml")
	if err := os.WriteFile(xmlFile, []byte(pipelineXML), 0644); err != nil {
		t.Fatalf("failed to write temp pipeline xml: %v", err)
	}

	absXSD, err := filepath.Abs(xsdPath)
	if err != nil {
		t.Fatalf("failed to get absolute path of xsd: %v", err)
	}

	err = flow.ValidateXSD(xmlFile, absXSD)
	if err != nil {
		t.Fatalf("pipeline with v1.2.31 features failed XSD validation: %v", err)
	}
}

func TestProcessXSLTWithSchemaLocation(t *testing.T) {
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<pipeline xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xsi:noNamespaceSchemaLocation="https://raw.githubusercontent.com/etl-madness/flow/main/xsd/pipeline.xsd">
    <variables>
        <variable name="test_var" type="string" value="hello" />
    </variables>
    <flow>
        <script id="test_step" language="bash">
            echo "hello"
        </script>
    </flow>
</pipeline>`

	xsltContent, err := os.ReadFile("./autodoc/autodoc_md.xslt")
	if err != nil {
		t.Fatalf("failed to read XSLT stylesheet: %v", err)
	}

	diagram, err := generateMermaid([]byte(xmlContent))
	if err != nil {
		t.Fatalf("failed to generate mermaid: %v", err)
	}

	src := "test_pipeline.xml"
	out, err := ProcessXSLT([]byte(xmlContent), xsltContent, diagram, &src)
	if err != nil {
		t.Fatalf("ProcessXSLT failed: %v", err)
	}

	outStr := string(out)
	if !strings.Contains(outStr, "Execution Flow Diagram") {
		t.Fatalf("expected output to contain 'Execution Flow Diagram', got: %s", outStr)
	}
	if !strings.Contains(outStr, "test_var") {
		t.Fatalf("expected output to contain 'test_var', got: %s", outStr)
	}
}

