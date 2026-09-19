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

func TestValidateXSDAllowsInlineForeachQuery(t *testing.T) {
	xsdPath := filepath.Join("xsd", "pipeline.xsd")
	if _, err := os.Stat(xsdPath); os.IsNotExist(err) {
		t.Fatalf("xsd file not found at %s", xsdPath)
	}

	pipelineXML := `<?xml version="1.0" encoding="UTF-8"?>
<pipeline xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:noNamespaceSchemaLocation="pipeline.xsd">
    <flow>
        <foreach id="loop_items" db="src_db">
            SELECT id, name FROM items WHERE active = 1;
            <sql id="process_item" db="src_db">
                UPDATE items SET processed = 1 WHERE id = {{id}};
            </sql>
        </foreach>
    </flow>
</pipeline>`

	tmpDir := t.TempDir()
	xmlFile := filepath.Join(tmpDir, "pipeline_foreach_inline.xml")
	if err := os.WriteFile(xmlFile, []byte(pipelineXML), 0644); err != nil {
		t.Fatalf("failed to write temp pipeline xml: %v", err)
	}

	absXSD, err := filepath.Abs(xsdPath)
	if err != nil {
		t.Fatalf("failed to get absolute path of xsd: %v", err)
	}

	err = flow.ValidateXSD(xmlFile, absXSD)
	if err != nil {
		t.Fatalf("foreach inline SQL query should validate, but failed: %v", err)
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

func TestLoadResourceNormalizesUTF16XML(t *testing.T) {
	xmlText := `<?xml version="1.0" encoding="UTF-16"?>
<pipeline>
  <flow>
    <script id="test_step">echo "hello"</script>
  </flow>
</pipeline>`

	utf16Bytes := []byte{0xFF, 0xFE}
	for _, r := range xmlText {
		utf16Bytes = append(utf16Bytes, byte(r), 0)
	}

	tmpFile := filepath.Join(t.TempDir(), "utf16_pipeline.xml")
	if err := os.WriteFile(tmpFile, utf16Bytes, 0644); err != nil {
		t.Fatalf("failed to write UTF-16 test xml: %v", err)
	}

	content, err := LoadResource(t.Context(), tmpFile)
	if err != nil {
		t.Fatalf("LoadResource returned error for UTF-16 file: %v", err)
	}

	if !strings.Contains(string(content), "<pipeline>") {
		t.Fatalf("expected normalized UTF-16 XML content to include <pipeline>, got: %q", string(content))
	}
	if strings.Contains(string(content), "\x00") {
		t.Fatalf("expected UTF-16 null bytes to be removed during normalization, got: %q", string(content))
	}
	if strings.Contains(strings.ToLower(string(content)), "encoding=\"utf-16\"") {
		t.Fatalf("expected UTF-16 declaration to be rewritten to UTF-8, got: %q", string(content))
	}
	if !strings.Contains(string(content), "encoding=\"UTF-8\"") {
		t.Fatalf("expected XML declaration to contain UTF-8, got: %q", string(content))
	}
}

func TestNormalizeXMLBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		contains string
		notHas   string
	}{
		{
			name:     "UTF-8 with UTF-16 declaration",
			input:    []byte(`<?xml version="1.0" encoding="UTF-16"?><schema/>`),
			contains: `encoding="UTF-8"`,
			notHas:   `encoding="UTF-16"`,
		},
		{
			name:     "UTF-8 with single-quoted utf-16 declaration",
			input:    []byte(`<?xml version="1.0" encoding='utf-16'?><schema/>`),
			contains: `encoding="UTF-8"`,
			notHas:   `encoding='utf-16'`,
		},
		{
			name:     "UTF-8 with UTF-8 BOM",
			input:    append([]byte{0xEF, 0xBB, 0xBF}, []byte(`<pipeline/>`)...),
			contains: `<pipeline/>`,
			notHas:   "\xEF\xBB\xBF",
		},
		{
			name: "UTF-16LE with BOM and UTF-16 declaration",
			input: func() []byte {
				s := `<?xml version="1.0" encoding="UTF-16"?><test>ok</test>`
				b := []byte{0xFF, 0xFE}
				for _, r := range s {
					b = append(b, byte(r), 0)
				}
				return b
			}(),
			contains: `<test>ok</test>`,
			notHas:   "\x00",
		},
		{
			name: "UTF-16BE with BOM",
			input: func() []byte {
				s := `<test>ok</test>`
				b := []byte{0xFE, 0xFF}
				for _, r := range s {
					b = append(b, 0, byte(r))
				}
				return b
			}(),
			contains: `<test>ok</test>`,
			notHas:   "\x00",
		},
		{
			name: "UTF-16LE without BOM",
			input: func() []byte {
				s := `<?xml version="1.0"?><test>ok</test>`
				var b []byte
				for _, r := range s {
					b = append(b, byte(r), 0)
				}
				return b
			}(),
			contains: `<test>ok</test>`,
			notHas:   "\x00",
		},
		{
			name: "UTF-16BE without BOM",
			input: func() []byte {
				s := `<?xml version="1.0"?><test>ok</test>`
				var b []byte
				for _, r := range s {
					b = append(b, 0, byte(r))
				}
				return b
			}(),
			contains: `<test>ok</test>`,
			notHas:   "\x00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized := string(normalizeXMLBytes(tt.input))
			if !strings.Contains(normalized, tt.contains) {
				t.Errorf("expected output to contain %q, got: %q", tt.contains, normalized)
			}
			if tt.notHas != "" && strings.Contains(normalized, tt.notHas) {
				t.Errorf("expected output NOT to contain %q, got: %q", tt.notHas, normalized)
			}
		})
	}
}

