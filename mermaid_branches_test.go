package main

import (
	"strings"
	"testing"

	"github.com/etl-madness/flow"
)

func TestMermaidControlFlow(t *testing.T) {
	input := `<pipeline>
<variables><variable name="greeting" type="string" value="hello"/></variables>
<databases><database name="source"/></databases>
<preflight><script id="check" lang="go"/></preflight>
<flow>
  <parallel><script id="a"/><script id="b" language="go"/></parallel>
  <if var="enabled" equals="true"><then><sql id="yes" target_table="items"/></then><else><script id="no"/></else></if>
  <foreach id="items" var="item"><script id="loop_body"/></foreach>
  <group><excel_read id="sheet" file="input.xlsx"/></group>
  <if/><while id="empty"/><unknown id="other"/>
</flow></pipeline>`
	diagram, err := generateMermaid([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"greeting", "db_source", "PreflightStart", "check", "Parallel Execution", "a", "b", "enabled == true", "Yes / Then", "No / Else", "items", "loop_body", "input.xlsx", "Check Condition", "other"} {
		if !strings.Contains(*diagram, want) {
			t.Errorf("diagram missing %q:\n%s", want, *diagram)
		}
	}
	if _, err := generateMermaid([]byte("<pipeline>")); err == nil {
		t.Fatal("expected invalid XML error")
	}
}

func TestApplyVariableOverrides(t *testing.T) {
	registry := flow.NewRegistry()
	applyVariableOverrides(registry, "integer=12,boolean=true,float=1.5,text=value, malformed, ")
	for key, want := range map[string]any{"integer": 12, "boolean": true, "float": 1.5, "text": "value"} {
		got := registry.GetVar(key)
		if got != want {
			t.Errorf("%s = %v; want %v", key, got, want)
		}
	}
}
