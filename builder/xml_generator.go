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

// isXMLEntity checks if s begins with a valid XML entity reference like &amp;, &lt;, &gt;, &quot;, &apos;, or &#...;
func isXMLEntity(s string) (bool, int) {
	if !strings.HasPrefix(s, "&") {
		return false, 0
	}
	for _, ent := range []string{"&amp;", "&lt;", "&gt;", "&quot;", "&apos;"} {
		if strings.HasPrefix(s, ent) {
			return true, len(ent)
		}
	}
	if strings.HasPrefix(s, "&#") {
		semi := strings.IndexByte(s, ';')
		if semi > 2 && semi <= 10 {
			numPart := s[2:semi]
			if strings.HasPrefix(numPart, "x") || strings.HasPrefix(numPart, "X") {
				isHex := len(numPart) > 1
				for _, c := range numPart[1:] {
					if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
						isHex = false
						break
					}
				}
				if isHex {
					return true, semi + 1
				}
			} else {
				isDec := len(numPart) > 0
				for _, c := range numPart {
					if c < '0' || c > '9' {
						isDec = false
						break
					}
				}
				if isDec {
					return true, semi + 1
				}
			}
		}
	}
	return false, 0
}

// escapeXMLAttr safely escapes special characters for double-quoted XML attributes.
// It avoids double-escaping if the string already contains valid XML entities.
func escapeXMLAttr(s string) string {
	var buf strings.Builder
	for i := 0; i < len(s); {
		if isEnt, entLen := isXMLEntity(s[i:]); isEnt {
			buf.WriteString(s[i : i+entLen])
			i += entLen
			continue
		}
		c := s[i]
		switch c {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '"':
			buf.WriteString("&quot;")
		default:
			buf.WriteByte(c)
		}
		i++
	}
	return buf.String()
}

// formatContent renders node text content, protecting code or queries with CDATA if needed.
func formatContent(content string, indent string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	var buf strings.Builder
	if strings.HasPrefix(content, "<![CDATA[") && strings.HasSuffix(content, "]]>") {
		for _, l := range strings.Split(content, "\n") {
			buf.WriteString(fmt.Sprintf("%s    %s\n", indent, l))
		}
		return buf.String()
	}
	if strings.ContainsAny(content, "<>&") {
		buf.WriteString(fmt.Sprintf("%s    <![CDATA[\n", indent))
		for _, l := range strings.Split(content, "\n") {
			buf.WriteString(fmt.Sprintf("%s        %s\n", indent, l))
		}
		buf.WriteString(fmt.Sprintf("%s    ]]>\n", indent))
		return buf.String()
	}
	for _, l := range strings.Split(content, "\n") {
		buf.WriteString(fmt.Sprintf("%s    %s\n", indent, l))
	}
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
		"into", "output_var", "var", "equals", "variable",
		"transaction", "tx", "timeout",
		"stream", "buffer", "mode", "language",
		"batch_size", "tablock", "check_constraints", "fire_triggers", "keep_nulls",
		"header", "op", "bucket", "key", "value",
		"condition", "max_iterations", "max_threads", "threads", "max_concurrency", "concurrency",
		"on_error", "retry_count", "retry_interval", "description",
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
			attrs = append(attrs, fmt.Sprintf(`%s="%s"`, k, escapeXMLAttr(v)))
		}
	}

	attrStr := ""
	if len(attrs) > 0 {
		attrStr = " " + strings.Join(attrs, " ")
	}

	content := strings.TrimSpace(node.ContentText)

	// Special handling for <if> conditional container
	if tag == "if" {
		var thenNode *PipelineNode
		var elseNode *PipelineNode
		var otherChildren []PipelineNode

		for i := range node.Children {
			ch := &node.Children[i]
			if ch.NodeType == "then" {
				thenNode = ch
			} else if ch.NodeType == "else" {
				elseNode = ch
			} else {
				otherChildren = append(otherChildren, *ch)
			}
		}

		hasThen := thenNode != nil && (len(thenNode.Children) > 0 || strings.TrimSpace(thenNode.ContentText) != "")
		hasElse := elseNode != nil && (len(elseNode.Children) > 0 || strings.TrimSpace(elseNode.ContentText) != "")
		hasOther := len(otherChildren) > 0
		hasContent := content != ""

		if !hasThen && !hasElse && !hasOther && !hasContent {
			buf.WriteString(fmt.Sprintf("%s<if%s />\n", indent, attrStr))
			return buf.String()
		}

		buf.WriteString(fmt.Sprintf("%s<if%s>\n", indent, attrStr))
		if hasContent {
			buf.WriteString(formatContent(content, indent))
		}
		for _, ch := range otherChildren {
			buf.WriteString(renderNodeXML(ch, indent+"    "))
		}
		if hasThen {
			buf.WriteString(fmt.Sprintf("%s    <then>\n", indent))
			if strings.TrimSpace(thenNode.ContentText) != "" {
				buf.WriteString(formatContent(thenNode.ContentText, indent+"    "))
			}
			for _, ch := range thenNode.Children {
				buf.WriteString(renderNodeXML(ch, indent+"        "))
			}
			buf.WriteString(fmt.Sprintf("%s    </then>\n", indent))
		}
		if hasElse {
			buf.WriteString(fmt.Sprintf("%s    <else>\n", indent))
			if strings.TrimSpace(elseNode.ContentText) != "" {
				buf.WriteString(formatContent(elseNode.ContentText, indent+"    "))
			}
			for _, ch := range elseNode.Children {
				buf.WriteString(renderNodeXML(ch, indent+"        "))
			}
			buf.WriteString(fmt.Sprintf("%s    </else>\n", indent))
		}
		buf.WriteString(fmt.Sprintf("%s</if>\n", indent))
		return buf.String()
	}

	// Container nodes with children (group, parallel, foreach, while, etc.)
	if len(node.Children) > 0 {
		buf.WriteString(fmt.Sprintf("%s<%s%s>\n", indent, tag, attrStr))
		if content != "" {
			buf.WriteString(formatContent(content, indent))
		}
		for _, ch := range node.Children {
			buf.WriteString(renderNodeXML(ch, indent+"    "))
		}
		buf.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))
		return buf.String()
	}

	if content == "" || tag == "variable" || tag == "database" {
		buf.WriteString(fmt.Sprintf("%s<%s%s />\n", indent, tag, attrStr))
	} else {
		buf.WriteString(fmt.Sprintf("%s<%s%s>\n", indent, tag, attrStr))
		buf.WriteString(formatContent(content, indent))
		buf.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))
	}

	return buf.String()
}
