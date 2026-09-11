package builder

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type rawXMLNode struct {
	XMLName  xml.Name   `xml:""`
	Attrs    []xml.Attr `xml:",any,attr"`
	InnerXML string     `xml:",innerxml"`
}

type rawSection struct {
	Nodes []rawXMLNode `xml:",any"`
}

type rawPipeline struct {
	XMLName     xml.Name    `xml:"pipeline"`
	Name        string      `xml:"name,attr"`
	ID          string      `xml:"id,attr"`
	Description string      `xml:"description,attr"`
	Variables   *rawSection `xml:"variables"`
	Databases   *rawSection `xml:"databases"`
	Preflight   *rawSection `xml:"preflight"`
	Flow        *rawSection `xml:"flow"`
	Scripts     *rawSection `xml:"scripts"`
}

// ImportPipelineFromXML reads an XML pipeline file from disk and stores it as a new script draft in SQLite.
func ImportPipelineFromXML(filePath string, customName string, storage *Storage) (*Script, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pipeline file %s: %w", filePath, err)
	}

	defaultName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	return ImportPipelineFromBytes(data, defaultName, customName, filePath, storage)
}

// ImportPipelineFromBytes parses pipeline XML bytes and stores it in SQLite.
func ImportPipelineFromBytes(data []byte, defaultName, customName, sourcePath string, storage *Storage) (*Script, error) {
	var raw rawPipeline
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse pipeline XML: %w", err)
	}

	targetName := strings.TrimSpace(customName)
	if targetName == "" {
		if strings.TrimSpace(raw.Name) != "" {
			targetName = strings.TrimSpace(raw.Name)
		} else if strings.TrimSpace(raw.ID) != "" {
			targetName = strings.TrimSpace(raw.ID)
		} else {
			targetName = defaultName
		}
	}
	if targetName == "" {
		targetName = "imported_pipeline"
	}

	// Guarantee unique name in storage
	existingScripts, err := storage.ListScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to check existing scripts: %w", err)
	}

	candidate := targetName
	counter := 1
	for {
		found := false
		for _, sc := range existingScripts {
			if strings.EqualFold(sc.Name, candidate) {
				found = true
				break
			}
		}
		if !found {
			break
		}
		if counter == 1 {
			candidate = fmt.Sprintf("%s (imported)", targetName)
		} else {
			candidate = fmt.Sprintf("%s (imported %d)", targetName, counter)
		}
		counter++
	}

	description := strings.TrimSpace(raw.Description)
	if description == "" && sourcePath != "" {
		description = fmt.Sprintf("Imported from %s", filepath.Base(sourcePath))
	}

	script, err := storage.CreateScript(candidate, description)
	if err != nil {
		return nil, fmt.Errorf("failed to create script record: %w", err)
	}

	insertNodes := func(section string, nodes []rawXMLNode) error {
		for _, n := range nodes {
			if err := insertNodeTree(script.ID, section, n, nil, storage); err != nil {
				return err
			}
		}
		return nil
	}

	// 1. Variables
	if raw.Variables != nil && len(raw.Variables.Nodes) > 0 {
		if err := insertNodes("variables", raw.Variables.Nodes); err != nil {
			return nil, err
		}
	}

	// 2. Databases
	if raw.Databases != nil && len(raw.Databases.Nodes) > 0 {
		if err := insertNodes("databases", raw.Databases.Nodes); err != nil {
			return nil, err
		}
	}

	// 3. Preflight
	if raw.Preflight != nil && len(raw.Preflight.Nodes) > 0 {
		if err := insertNodes("preflight", raw.Preflight.Nodes); err != nil {
			return nil, err
		}
	}

	// 4. Flow
	if raw.Flow != nil && len(raw.Flow.Nodes) > 0 {
		if err := insertNodes("flow", raw.Flow.Nodes); err != nil {
			return nil, err
		}
	} else if raw.Scripts != nil && len(raw.Scripts.Nodes) > 0 {
		// Fallback for legacy <scripts> section
		if err := insertNodes("flow", raw.Scripts.Nodes); err != nil {
			return nil, err
		}
	}

	return script, nil
}

// Helper to extract XML attributes into a map, ignoring XML schema namespaces
func extractAttrs(attrs []xml.Attr) map[string]string {
	res := make(map[string]string)
	for _, a := range attrs {
		if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" || strings.HasPrefix(a.Name.Local, "xmlns:") {
			continue
		}
		if a.Name.Space == "xsi" || a.Name.Local == "noNamespaceSchemaLocation" {
			continue
		}
		res[a.Name.Local] = a.Value
	}
	return res
}

// parseContainerInner extracts text content and child XML elements from inner XML.
func parseContainerInner(innerXML string) (string, []rawXMLNode) {
	if strings.TrimSpace(innerXML) == "" {
		return "", nil
	}
	d := xml.NewDecoder(strings.NewReader("<wrapper>" + innerXML + "</wrapper>"))
	var textParts []string
	var children []rawXMLNode

	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.CharData:
			txt := strings.TrimSpace(string(t))
			if txt != "" {
				textParts = append(textParts, txt)
			}
		case xml.StartElement:
			if t.Name.Local == "wrapper" {
				continue
			}
			var child rawXMLNode
			if err := d.DecodeElement(&child, &t); err == nil {
				children = append(children, child)
			}
		}
	}
	return strings.Join(textParts, "\n"), children
}

// insertNodeTree recursively inserts an XML node and all its nested children into SQLite storage.
func insertNodeTree(scriptID int64, section string, n rawXMLNode, parentNodeID *int64, storage *Storage) error {
	tag := n.XMLName.Local
	if tag == "" {
		return nil
	}

	attrs := extractAttrs(n.Attrs)

	// Special handling for <if> conditional container
	if tag == "if" {
		ifNode, err := storage.AddNodeWithParent(scriptID, section, tag, attrs, "", parentNodeID)
		if err != nil {
			return fmt.Errorf("failed to add <if> node: %w", err)
		}

		thenNode, elseNode, err := storage.EnsureIfBranches(scriptID, section, ifNode.ID)
		if err != nil {
			return fmt.Errorf("failed to ensure if branches: %w", err)
		}

		_, children := parseContainerInner(n.InnerXML)
		for _, ch := range children {
			chTag := ch.XMLName.Local
			if chTag == "then" {
				_, thenChildren := parseContainerInner(ch.InnerXML)
				for _, tch := range thenChildren {
					if err := insertNodeTree(scriptID, section, tch, &thenNode.ID, storage); err != nil {
						return err
					}
				}
			} else if chTag == "else" {
				_, elseChildren := parseContainerInner(ch.InnerXML)
				for _, ech := range elseChildren {
					if err := insertNodeTree(scriptID, section, ech, &elseNode.ID, storage); err != nil {
						return err
					}
				}
			} else {
				// Inline child under <if> treated as <then>
				if err := insertNodeTree(scriptID, section, ch, &thenNode.ID, storage); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Container tags or nodes with child elements (group, parallel, foreach, while, etc.)
	isContainerTag := tag == "group" || tag == "parallel" || tag == "foreach" || tag == "while"
	driverText, children := parseContainerInner(n.InnerXML)

	if isContainerTag || len(children) > 0 {
		contentText := ""
		if tag == "foreach" {
			contentText = driverText
		}
		containerNode, err := storage.AddNodeWithParent(scriptID, section, tag, attrs, contentText, parentNodeID)
		if err != nil {
			return fmt.Errorf("failed to add <%s> node: %w", tag, err)
		}

		for _, ch := range children {
			if err := insertNodeTree(scriptID, section, ch, &containerNode.ID, storage); err != nil {
				return err
			}
		}
		return nil
	}

	// Leaf nodes (sql, script, database, variable, kv, etc.)
	content := strings.TrimSpace(n.InnerXML)
	_, err := storage.AddNodeWithParent(scriptID, section, tag, attrs, content, parentNodeID)
	if err != nil {
		return fmt.Errorf("failed to add node <%s>: %w", tag, err)
	}
	return nil
}
