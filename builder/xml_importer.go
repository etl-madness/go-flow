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

	// Helper to extract attributes and insert nodes
	insertNodes := func(section string, nodes []rawXMLNode) error {
		for _, n := range nodes {
			tag := n.XMLName.Local
			if tag == "" {
				continue
			}

			attrs := make(map[string]string)
			for _, a := range n.Attrs {
				// Filter XML schema namespaces
				if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" || strings.HasPrefix(a.Name.Local, "xmlns:") {
					continue
				}
				if a.Name.Space == "xsi" || a.Name.Local == "noNamespaceSchemaLocation" {
					continue
				}
				attrs[a.Name.Local] = a.Value
			}

			content := strings.TrimSpace(n.InnerXML)
			_, err := storage.AddNode(script.ID, section, tag, attrs, content)
			if err != nil {
				return fmt.Errorf("failed to add %s node <%s>: %w", section, tag, err)
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
