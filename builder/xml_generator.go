package builder

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// GenerateXML constructs a valid Flow XML scripts file from database nodes.
func GenerateXML(pipelineName string, varNodes, dbNodes, preflightNodes, flowNodes []PipelineNode) string {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<pipeline xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"` + "\n")
	buf.WriteString(`          xsi:noNamespaceSchemaLocation="https://raw.githubusercontent.com/etl-madness/flow/main/xsd/pipeline.xsd">` + "\n")
	// buf.WriteString(fmt.Sprintf(`          id=%q>`+"\n", pipelineName))

	// 1. Variables
	if len(varNodes) > 0 {
		buf.WriteString("    <variables>\n")
		for _, n := range varNodes {
			buf.WriteString(renderNodeXML(n, "        "))
		}
		buf.WriteString("    </variables>\n\n")
	}

	// 2. Databases
	if len(dbNodes) > 0 {
		buf.WriteString("    <databases>\n")
		for _, n := range dbNodes {
			buf.WriteString(renderNodeXML(n, "        "))
		}
		buf.WriteString("    </databases>\n\n")
	}

	// 3. Preflight
	if len(preflightNodes) > 0 {
		buf.WriteString("    <preflight>\n")
		for _, n := range preflightNodes {
			buf.WriteString(renderNodeXML(n, "        "))
		}
		buf.WriteString("    </preflight>\n\n")
	}

	// 4. Flow
	buf.WriteString("    <flow>\n")
	for _, n := range flowNodes {
		buf.WriteString(renderNodeXML(n, "        "))
	}
	buf.WriteString("    </flow>\n")

	buf.WriteString("</pipeline>\n")
	return buf.String()
}

func renderNodeXML(node PipelineNode, indent string) string {
	var buf bytes.Buffer
	tag := node.NodeType

	var keys []string
	for k := range node.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var orderedKeys []string
	preferredOrder := []string{
		"id", "name", "driver", "connection_string", "workload",
		"db", "database", "target_db", "target_database", "target_table", "table",
		"file", "path", "sheet", "source",
		"into", "output_var", "var", "variable",
		"transaction", "tx", "timeout",
		"stream", "buffer", "mode", "language",
		"batch_size", "tablock", "check_constraints", "fire_triggers", "keep_nulls",
		"header", "op", "bucket", "key", "value",
		"condition", "max_iterations", "max_concurrency",
		"on_error", "retry_count", "description",
	}
	for _, preferred := range preferredOrder {
		for _, k := range keys {
			if k == preferred {
				orderedKeys = append(orderedKeys, k)
			}
		}
	}
	for _, k := range keys {
		found := false
		for _, ok := range orderedKeys {
			if ok == k {
				found = true
				break
			}
		}
		if !found {
			orderedKeys = append(orderedKeys, k)
		}
	}

	var attrs []string
	for _, k := range orderedKeys {
		v := node.Attributes[k]
		if v != "" {
			attrs = append(attrs, fmt.Sprintf(`%s=%q`, k, v))
		}
	}

	attrStr := ""
	if len(attrs) > 0 {
		attrStr = " " + strings.Join(attrs, " ")
	}

	content := strings.TrimSpace(node.ContentText)
	if content == "" || tag == "variable" || tag == "database" {
		buf.WriteString(fmt.Sprintf("%s<%s%s />\n", indent, tag, attrStr))
	} else {
		buf.WriteString(fmt.Sprintf("%s<%s%s>\n", indent, tag, attrStr))
		lines := strings.Split(content, "\n")
		for _, l := range lines {
			buf.WriteString(fmt.Sprintf("%s    %s\n", indent, l))
		}
		buf.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))
	}

	return buf.String()
}
