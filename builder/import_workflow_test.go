package builder

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestImportPipelineNamesAndSections(t *testing.T) {
	storage, err := NewStorage(filepath.Join(t.TempDir(), "builder.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	xml := `<pipeline name="example" description="test">
<variables><variable name="v" value="x"/></variables>
<databases><database name="db" driver="sqlite" connection_string=":memory:"/></databases>
<preflight><sql id="check" db="db">SELECT 1</sql></preflight>
<flow><group id="group"><sql id="step" db="db">SELECT 2</sql></group></flow>
</pipeline>`
	for _, name := range []string{"example", "example (imported)", "example (imported 2)"} {
		script, err := ImportPipelineFromBytes([]byte(xml), "", "", "source.xml", storage)
		if err != nil || script.Name != name {
			t.Fatalf("import = %+v, %v; want %q", script, err, name)
		}
		for _, section := range []string{"variables", "databases", "preflight", "flow"} {
			nodes, err := storage.GetNodeTree(script.ID, section)
			if err != nil || len(nodes) == 0 {
				t.Fatalf("section %s = %+v, %v", section, nodes, err)
			}
		}
	}
	legacy, err := ImportPipelineFromBytes([]byte(`<pipeline id="legacy"><scripts><script id="step">select 1</script></scripts></pipeline>`), "", "", "legacy.xml", storage)
	if err != nil || legacy.Name != "legacy" || !strings.Contains(legacy.Description, "legacy.xml") {
		t.Fatalf("legacy import = %+v, %v", legacy, err)
	}
	nodes, err := storage.GetNodeTree(legacy.ID, "flow")
	if err != nil || len(nodes) != 1 {
		t.Fatalf("legacy nodes = %+v, %v", nodes, err)
	}
	if _, err := ImportPipelineFromBytes([]byte("<pipeline>"), "", "", "", storage); err == nil {
		t.Fatal("expected invalid XML error")
	}
}
