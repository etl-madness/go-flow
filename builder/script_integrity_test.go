package builder

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestExternalScriptIntegrity(t *testing.T) {
	template, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatal(err)
	}

	scripts := map[string]string{
		"https://unpkg.com/htmx.org@1.9.10":                              "sha384-D1Kt99CQMDuVetoL1lrYwg5t+9QdHe7NLX/SoJYkXDFfX37iInKRy5xLSi8nO7UC",
		"https://cdn.jsdelivr.net/npm/sortablejs@1.15.2/Sortable.min.js": "sha384-BSxuMLxX+FCbTdYec3TbXlnMGEEM2QXTFdtDaveen71o+jswm2J36+xFqp8k4VHM",
	}
	for name, content := range map[string]string{"template": string(template), "embedded": IndexHTML} {
		t.Run(name, func(t *testing.T) {
			root, err := html.Parse(strings.NewReader(content))
			if err != nil {
				t.Fatal(err)
			}
			found := make(map[string]int)
			attr := func(node *html.Node, key string) string {
				for _, item := range node.Attr {
					if item.Key == key {
						return item.Val
					}
				}
				return ""
			}
			var visit func(*html.Node)
			visit = func(node *html.Node) {
				if node.Type == html.ElementNode && node.Data == "script" {
					src := attr(node, "src")
					if integrity, ok := scripts[src]; ok {
						found[src]++
						if attr(node, "integrity") != integrity || attr(node, "crossorigin") != "anonymous" {
							t.Errorf("script %s has incorrect integrity or crossorigin attributes", src)
						}
					}
				}
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					visit(child)
				}
			}
			visit(root)
			for src := range scripts {
				if found[src] != 1 {
					t.Errorf("expected one script for %s; found %d", src, found[src])
				}
			}
		})
	}
}
