package builder

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestIndexFormLabels(t *testing.T) {
	template, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatal(err)
	}

	for name, content := range map[string]string{
		"template": string(template),
		"embedded": IndexHTML,
	} {
		t.Run(name, func(t *testing.T) {
			root, err := html.Parse(strings.NewReader(content))
			if err != nil {
				t.Fatal(err)
			}

			ids := make(map[string][]*html.Node)
			var labels, controls []*html.Node
			var walk func(*html.Node)
			walk = func(node *html.Node) {
				if node.Type == html.ElementNode {
					if id := formLabelAttr(node, "id"); id != "" {
						ids[id] = append(ids[id], node)
					}
					switch node.Data {
					case "label":
						labels = append(labels, node)
					case "input", "select", "textarea":
						if formLabelAttr(node, "type") != "hidden" {
							controls = append(controls, node)
						}
					}
				}
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					walk(child)
				}
			}
			walk(root)

			labeled := make(map[*html.Node]bool)
			for _, label := range labels {
				if strings.TrimSpace(formLabelText(label)) == "" {
					t.Error("label has no accessible text")
				}
				if target := formLabelAttr(label, "for"); target != "" {
					nodes := ids[target]
					if len(nodes) != 1 || !isFormLabelControl(nodes[0]) {
						t.Errorf("label target %q does not identify exactly one form control", target)
						continue
					}
					labeled[nodes[0]] = true
					continue
				}
				nested := 0
				for _, control := range controls {
					for parent := control.Parent; parent != nil; parent = parent.Parent {
						if parent == label {
							labeled[control] = true
							nested++
							break
						}
					}
				}
				if nested != 1 {
					t.Errorf("label %q contains %d controls, expected one", strings.TrimSpace(formLabelText(label)), nested)
				}
			}
			for _, control := range controls {
				if !labeled[control] && strings.TrimSpace(formLabelAttr(control, "aria-label")) == "" {
					t.Errorf("%s %q has no accessible label", control.Data, formLabelAttr(control, "id"))
				}
			}

			for _, snippet := range []string{
				`<select aria-label="Attribute name"`,
				`<input type="text" aria-label="Custom attribute name"`,
				`<input type="text" aria-label="Attribute value"`,
				`<select aria-label="Select defined database"`,
				`<label for="' + (f.name === 'db' ? 'db-input-' + f.name : 'attr_' + f.name) + '"`,
				`inputHtml = '<select id="attr_' + f.name + '" name="attr_' + f.name`,
				`inputHtml = '<input type="text" id="attr_' + f.name + '" name="attr_' + f.name`,
			} {
				if !strings.Contains(content, snippet) {
					t.Errorf("dynamic field markup missing %q", snippet)
				}
			}
		})
	}
}

func formLabelAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func formLabelText(node *html.Node) string {
	var text strings.Builder
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return text.String()
}

func isFormLabelControl(node *html.Node) bool {
	return node.Type == html.ElementNode && (node.Data == "select" || node.Data == "textarea" ||
		(node.Data == "input" && formLabelAttr(node, "type") != "hidden"))
}
