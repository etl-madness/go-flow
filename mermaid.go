package main

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/etl-madness/flow"
)

// ============================================================================
// XML UNMARSHALING STRUCTS
// ============================================================================

type DiagramPipeline struct {
	XMLName   xml.Name          `xml:"pipeline"`
	Variables *DiagramVariables `xml:"variables"`
	Databases *DiagramDatabases `xml:"databases"`
	Preflight *DiagramBlock     `xml:"preflight"`
	Flow      *DiagramBlock     `xml:"flow"`
	Scripts   *DiagramBlock     `xml:"scripts"`
}

type DiagramVariables struct {
	Vars []DiagramVariable `xml:"variable"`
}

type DiagramVariable struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

type DiagramDatabases struct {
	DBs []DiagramDatabase `xml:"database"`
}

type DiagramDatabase struct {
	Name             string `xml:"name,attr"`
	ConnectionString string `xml:"connection_string,attr"`
	Driver           string `xml:"driver,attr"`
}

type DiagramBlock struct {
	Nodes []DiagramNode `xml:",any"`
}

type DiagramNode struct {
	XMLName   xml.Name
	ID        string        `xml:"id,attr"`
	Language  string        `xml:"language,attr"`
	Lang      string        `xml:"lang,attr"`
	TargetDB  string        `xml:"target_db,attr"`
	Var       string        `xml:"var,attr"`
	Equals    string        `xml:"equals,attr"`
	Condition string        `xml:"condition,attr"`
	Children  []DiagramNode `xml:",any"`
	Then      *DiagramBlock `xml:"then"`
	Else      *DiagramBlock `xml:"else"`
}

// ============================================================================
// MERMAID DIAGRAM GENERATOR HELPER FUNCTIONS
// ============================================================================

func generateMermaid(xmlBytes []byte) (*string, error) {
	var pipeline DiagramPipeline
	if err := xml.Unmarshal(xmlBytes, &pipeline); err != nil {
		return nil, err
	}

	gen := &MermaidGenerator{}
	gen.builder.WriteString("flowchart TD\n")

	// 1. Render Variables Subgraph Box
	if pipeline.Variables != nil && len(pipeline.Variables.Vars) > 0 {
		gen.builder.WriteString("    subgraph VarsBox [\"📋 Pipeline Variables\"]\n")
		var varLines []string
		for _, v := range pipeline.Variables.Vars {
			val := v.Value
			if len(val) > 35 {
				val = val[:32] + "..."
			}
			// Escape double quotes inside values to avoid Mermaid syntax errors
			val = strings.ReplaceAll(val, "\"", "'")
			varLines = append(varLines, fmt.Sprintf("• <b>%s</b> <i>(%s)</i>: <code>%s</code>", v.Name, v.Type, val))
		}
		label := strings.Join(varLines, "<br/>")
		gen.builder.WriteString(fmt.Sprintf("        vars_node[\"%s\"]\n", label))
		gen.builder.WriteString("    end\n\n")
	}

	// 2. Render Databases Subgraph Box
	if pipeline.Databases != nil && len(pipeline.Databases.DBs) > 0 {
		gen.builder.WriteString("    subgraph DBBox [\"🗄️ Configured Databases\"]\n")
		for _, db := range pipeline.Databases.DBs {
			dbID := fmt.Sprintf("db_%s", db.Name)
			gen.builder.WriteString(fmt.Sprintf("        %s[(\"Database: %s\")]\n", dbID, db.Name))
		}
		gen.builder.WriteString("    end\n\n")
	}

	// 3. Render Preflight Flow Box (if preflight nodes exist)
	if pipeline.Preflight != nil && len(pipeline.Preflight.Nodes) > 0 {
		gen.builder.WriteString("    subgraph PreflightBox [\"✈️ Preflight Flow\"]\n")
		gen.builder.WriteString("        PreflightStart([Start Preflight])\n")
		_, lastPreflight := gen.ProcessSequence(pipeline.Preflight.Nodes, "PreflightStart")
		if lastPreflight != "" {
			gen.builder.WriteString(fmt.Sprintf("        %s --> PreflightEnd([End Preflight])\n", lastPreflight))
		}
		gen.builder.WriteString("    end\n\n")
	}

	// 4. Render Main Pipeline Execution Flow Box
	var flowNodes []DiagramNode
	if pipeline.Flow != nil {
		flowNodes = append(flowNodes, pipeline.Flow.Nodes...)
	}
	if pipeline.Scripts != nil {
		flowNodes = append(flowNodes, pipeline.Scripts.Nodes...)
	}

	if len(flowNodes) > 0 {
		gen.builder.WriteString("    subgraph FlowBox [\"⚡ Main Execution Flow\"]\n")
		gen.builder.WriteString("        Start([Start Pipeline])\n")

		_, lastExit := gen.ProcessSequence(flowNodes, "Start")
		if lastExit != "" {
			endID := gen.nextID("End")
			gen.builder.WriteString(fmt.Sprintf("        %s --> %s([End Pipeline])\n", lastExit, endID))
		}
		gen.builder.WriteString("    end\n")
	}

	result := gen.builder.String()
	return &result, nil
}

type MermaidGenerator struct {
	builder     strings.Builder
	nodeCounter int
}

func (g *MermaidGenerator) nextID(prefix string) string {
	g.nodeCounter++
	return fmt.Sprintf("%s_%d", prefix, g.nodeCounter)
}

func (g *MermaidGenerator) ProcessSequence(nodes []DiagramNode, prevExitID string) (string, string) {
	currentPrev := prevExitID
	var firstEntry, lastExit string

	for _, node := range nodes {
		entryID, exitID := g.ProcessNode(node)
		if firstEntry == "" && entryID != "" {
			firstEntry = entryID
		}
		if currentPrev != "" && entryID != "" {
			g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", currentPrev, entryID))
		}
		if exitID != "" {
			currentPrev = exitID
			lastExit = exitID
		}
	}

	return firstEntry, lastExit
}

func (g *MermaidGenerator) ProcessNode(node DiagramNode) (string, string) {
	nodeName := strings.ToLower(node.XMLName.Local)

	switch nodeName {
	case "script":
		id := node.ID
		if id == "" {
			id = g.nextID("script")
		}
		lang := node.Language
		if lang == "" {
			lang = node.Lang
		}
		if lang == "" {
			lang = "sql"
		}

		label := fmt.Sprintf("%s<br/>(%s)", id, strings.ToUpper(lang))
		if node.TargetDB != "" {
			label += fmt.Sprintf("<br/>➔ Stream to %s", node.TargetDB)
		}

		g.builder.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		return id, id

	case "sql", "sql_bulk":
		id := node.ID
		if id == "" {
			id = g.nextID(nodeName)
		}
		label := fmt.Sprintf("%s<br/>(%s)", id, strings.ToUpper(nodeName))
		if node.TargetDB != "" {
			label += fmt.Sprintf("<br/>➔ Stream to %s", node.TargetDB)
		}
		g.builder.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
		return id, id

	case "group":
		if len(node.Children) > 0 {
			return g.ProcessSequence(node.Children, "")
		}
		return "", ""

	case "parallel":
		pStart := g.nextID("parallel_start")
		pEnd := g.nextID("parallel_end")

		g.builder.WriteString(fmt.Sprintf("    %s{\"⚡ Parallel Execution\"}\n", pStart))
		g.builder.WriteString(fmt.Sprintf("    %s(( Join ))\n", pEnd))

		for _, child := range node.Children {
			cEntry, cExit := g.ProcessNode(child)
			if cEntry != "" {
				g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", pStart, cEntry))
			}
			if cExit != "" {
				g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", cExit, pEnd))
			}
		}
		return pStart, pEnd

	case "if":
		ifStart := g.nextID("if_cond")
		ifEnd := g.nextID("if_end")

		condLabel := node.Condition
		if condLabel == "" && node.Var != "" {
			condLabel = fmt.Sprintf("%s == %s", node.Var, node.Equals)
		}
		if condLabel == "" {
			condLabel = "Check Condition"
		}

		g.builder.WriteString(fmt.Sprintf("    %s{\"❓ If: %s\"}\n", ifStart, condLabel))
		g.builder.WriteString(fmt.Sprintf("    %s(( Rejoin ))\n", ifEnd))

		var thenNodes []DiagramNode
		if node.Then != nil && len(node.Then.Nodes) > 0 {
			thenNodes = node.Then.Nodes
		} else if len(node.Children) > 0 {
			thenNodes = node.Children
		}

		if len(thenNodes) > 0 {
			firstEntry, lastExit := g.ProcessSequence(thenNodes, "")
			g.builder.WriteString(fmt.Sprintf("    %s -- \"Yes / Then\" --> %s\n", ifStart, firstEntry))
			g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", lastExit, ifEnd))
		} else {
			g.builder.WriteString(fmt.Sprintf("    %s -- \"Yes / Then\" --> %s\n", ifStart, ifEnd))
		}

		if node.Else != nil && len(node.Else.Nodes) > 0 {
			firstEntry, lastExit := g.ProcessSequence(node.Else.Nodes, "")
			g.builder.WriteString(fmt.Sprintf("    %s -- \"No / Else\" --> %s\n", ifStart, firstEntry))
			g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", lastExit, ifEnd))
		} else {
			g.builder.WriteString(fmt.Sprintf("    %s -- \"No / Else\" --> %s\n", ifStart, ifEnd))
		}

		return ifStart, ifEnd

	case "foreach", "loop", "while":
		loopStart := g.nextID("loop_start")
		loopEnd := g.nextID("loop_end")

		title := fmt.Sprintf("🔄 Loop: %s", node.ID)
		if node.Var != "" {
			title += fmt.Sprintf(" (%s)", node.Var)
		}

		g.builder.WriteString(fmt.Sprintf("    %s{\"%s\"}\n", loopStart, title))
		g.builder.WriteString(fmt.Sprintf("    %s(( Loop Exit ))\n", loopEnd))

		if len(node.Children) > 0 {
			firstEntry, lastExit := g.ProcessSequence(node.Children, "")
			g.builder.WriteString(fmt.Sprintf("    %s -- \"Next Row\" --> %s\n", loopStart, firstEntry))
			g.builder.WriteString(fmt.Sprintf("    %s --> %s\n", lastExit, loopStart))
		}

		g.builder.WriteString(fmt.Sprintf("    %s -- \"Done\" --> %s\n", loopStart, loopEnd))
		return loopStart, loopEnd
	}

	return "", ""
}

func applyVariableOverrides(r *flow.Registry, overrideStr string) {
	if strings.TrimSpace(overrideStr) == "" {
		return
	}

	pairs := strings.Split(overrideStr, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("Warning: Skipping malformed override parameter %q (expected format: var=value)\n", pair)
			continue
		}

		key := strings.TrimSpace(parts[0])
		rawVal := strings.TrimSpace(parts[1])

		var parsedVal interface{} = rawVal
		if i, err := strconv.Atoi(rawVal); err == nil {
			parsedVal = i
		} else if b, err := strconv.ParseBool(rawVal); err == nil {
			parsedVal = b
		} else if f, err := strconv.ParseFloat(rawVal, 64); err == nil {
			parsedVal = f
		}

		r.SetVar(key, parsedVal)
	}
}
