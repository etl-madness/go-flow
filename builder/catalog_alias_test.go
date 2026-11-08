package builder

import (
	"reflect"
	"testing"
)

func TestHTMLTemplateAliasesUseIndependentFields(t *testing.T) {
	catalog := GetCatalog()
	primary := catalog.GetComponentByType("template_html")
	alias := catalog.GetComponentByType("html_template")
	if primary == nil || alias == nil {
		t.Fatal("HTML template component or its alias is missing")
	}

	if !reflect.DeepEqual(primary.Fields, alias.Fields) || len(primary.Fields) != 8 {
		t.Fatalf("HTML template fields differ: primary=%+v alias=%+v", primary.Fields, alias.Fields)
	}
	alias.Fields[0].Label = "changed"
	if primary.Fields[0].Label != "Step ID" {
		t.Fatal("modifying alias fields changed primary fields")
	}
}
