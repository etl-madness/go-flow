package builder

import (
	"reflect"
	"testing"
)

func TestXMLPathAliasesPreserveMetadata(t *testing.T) {
	catalog := GetCatalog()
	primary := catalog.GetComponentByType("xml_xpath")
	alias := catalog.GetComponentByType("xml_path")
	if primary == nil || alias == nil || len(primary.Fields) != 8 || len(alias.Fields) != 8 {
		t.Fatal("XML path component or its alias is missing fields")
	}
	if primary.Fields[4].Description != "Output format: text (joined text), xml (raw tags), json_array" ||
		alias.Fields[4].Description != "Output format: text, xml, json_array" {
		t.Fatal("XML path mode descriptions changed")
	}
	for i := range primary.Fields {
		if i != 4 && !reflect.DeepEqual(primary.Fields[i], alias.Fields[i]) {
			t.Errorf("field %d differs: %+v and %+v", i, primary.Fields[i], alias.Fields[i])
		}
	}
	alias.Fields[4].Options[0] = "changed"
	if primary.Fields[4].Options[0] != "text" {
		t.Fatal("modifying alias options changed primary options")
	}
}
