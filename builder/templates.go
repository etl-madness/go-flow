package builder

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
)

//go:embed tmpl/*.html tmpl/partials/*.html
var templateFS embed.FS

func BuildTemplate() (*template.Template, error) {
	funcMap := template.FuncMap{
		"toJS": func(v any) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
	}

	tmpl := template.New("index").Funcs(funcMap)
	if fsys, err := fs.Sub(templateFS, "tmpl"); err == nil {
		if tmpl2, err := tmpl.ParseFS(fsys, "*.html", "partials/*.html"); err == nil {
			tmpl = tmpl2
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}

	if tmpl2, err := tmpl.Parse(IndexHTML); err != nil {
		return nil, err
	} else {
		tmpl = tmpl2
	}
	return tmpl, nil
}
