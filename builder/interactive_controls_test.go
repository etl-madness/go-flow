package builder

import (
	"os"
	"strings"
	"testing"
)

func TestIndexInteractiveControls(t *testing.T) {
	template, err := os.ReadFile("tmpl/index.html")
	if err != nil {
		t.Fatal(err)
	}

	for name, content := range map[string]string{
		"template": string(template),
		"embedded": IndexHTML,
	} {
		t.Run(name, func(t *testing.T) {
			for _, snippet := range []string{
				`<button type="button" class="component-card`,
				`document.createElement('button')`,
				`<button type="button" onclick="loadDirectory(`,
				`<button type="button" onclick="selectFileFromBrowser(`,
				`<tr class="relative transition-colors `,
				`<button type="button" aria-current="' + isSelected + '" onclick="selectLogRun(`,
				`#log-runs-tbody button[aria-current="true"]`,
				`<button type="button" id="xml-preview-resizer"`,
				`aria-describedby="xml-preview-resize-help xml-preview-resize-value"`,
				`resizer.disabled = collapsed`,
				`resizer.addEventListener('keydown'`,
				`aria-label="Close database guide"`,
				`aria-label="Close HTTP guide"`,
				`aria-label="Close component guide"`,
			} {
				if !strings.Contains(content, snippet) {
					t.Errorf("missing interactive control markup %q", snippet)
				}
			}
		})
	}
}
