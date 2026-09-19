package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/etl-madness/flow"
	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xslt3"
)

var utf16DeclRegex = regexp.MustCompile(`(?i)(<\?xml[^>]*\bencoding\s*=\s*)["']utf-16(?:le|be)?["']`)

type defaultURIResolver struct {
	client *http.Client
}

func normalizeXMLBytes(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	if len(data) >= 2 {
		switch {
		case data[0] == 0xFF && data[1] == 0xFE:
			// UTF-16 LE with BOM
			if len(data)%2 == 0 {
				codeUnits := make([]uint16, 0, len(data)/2-1)
				for i := 2; i+1 < len(data); i += 2 {
					codeUnits = append(codeUnits, uint16(data[i])|uint16(data[i+1])<<8)
				}
				data = []byte(string(utf16.Decode(codeUnits)))
			}
		case data[0] == 0xFE && data[1] == 0xFF:
			// UTF-16 BE with BOM
			if len(data)%2 == 0 {
				codeUnits := make([]uint16, 0, len(data)/2-1)
				for i := 2; i+1 < len(data); i += 2 {
					codeUnits = append(codeUnits, uint16(data[i+1])|uint16(data[i])<<8)
				}
				data = []byte(string(utf16.Decode(codeUnits)))
			}
		case len(data) >= 4 && data[0] == 0x3C && data[1] == 0x00 && data[2] == 0x3F && data[3] == 0x00 && len(data)%2 == 0:
			// UTF-16 LE without BOM
			codeUnits := make([]uint16, 0, len(data)/2)
			for i := 0; i+1 < len(data); i += 2 {
				codeUnits = append(codeUnits, uint16(data[i])|uint16(data[i+1])<<8)
			}
			data = []byte(string(utf16.Decode(codeUnits)))
		case len(data) >= 4 && data[0] == 0x00 && data[1] == 0x3C && data[2] == 0x00 && data[3] == 0x3F && len(data)%2 == 0:
			// UTF-16 BE without BOM
			codeUnits := make([]uint16, 0, len(data)/2)
			for i := 0; i+1 < len(data); i += 2 {
				codeUnits = append(codeUnits, uint16(data[i+1])|uint16(data[i])<<8)
			}
			data = []byte(string(utf16.Decode(codeUnits)))
		}
	}

	if utf16DeclRegex.Match(data) {
		data = utf16DeclRegex.ReplaceAll(data, []byte(`${1}"UTF-8"`))
	}
	return data
}

func newDefaultURIResolver() *defaultURIResolver {
	return &defaultURIResolver{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func normalizeSchemaLocation(xmlInput []byte) []byte {
	content := string(xmlInput)
	for _, schemaURL := range []string{
		"https://raw.githubusercontent.com/etl-madness/flow/main/xsd/pipeline.xsd",
		"https://raw.githubusercontent.com/etl-madness/flow/master/xsd/pipeline.xsd",
		"https://raw.githubusercontent.com/etl-madness/flow/main/xsd/pipeline.xsd?raw=1",
	} {
		content = strings.ReplaceAll(content, schemaURL, "pipeline.xsd")
	}
	return []byte(content)
}

func (r *defaultURIResolver) resolve(rawURI string) (io.ReadCloser, error) {
	trimmedURI := strings.TrimSpace(rawURI)
	if strings.HasSuffix(trimmedURI, "pipeline.xsd") || strings.Contains(trimmedURI, "/pipeline.xsd") {
		return io.NopCloser(bytes.NewReader(normalizeXMLBytes(flow.GetSchemaXSD()))), nil
	}

	if strings.HasPrefix(rawURI, "http://") || strings.HasPrefix(rawURI, "https://") {
		resp, err := r.client.Get(rawURI)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return nil, os.ErrNotExist
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
		}
		defer resp.Body.Close()
		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(normalizeXMLBytes(payload))), nil
	}

	filePath := rawURI
	if strings.HasPrefix(filePath, "file://") {
		filePath = strings.TrimPrefix(filePath, "file://")
		if runtime.GOOS == "windows" && strings.HasPrefix(filePath, "/") && len(filePath) > 2 && filePath[2] == ':' {
			filePath = filePath[1:]
		}
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	payload, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(normalizeXMLBytes(payload))), nil
}

func (r *defaultURIResolver) Resolve(uri string) (io.ReadCloser, error) {
	return r.resolve(uri)
}

func (r *defaultURIResolver) ResolveURI(uri string) (io.ReadCloser, error) {
	return r.resolve(uri)
}

func ProcessXSLT(xmlInput, xsltInput []byte, diagram *string, source *string) ([]byte, error) {
	ctx := context.Background()
	parser := helium.NewParser()
	resolver := newDefaultURIResolver()
	xmlInput = normalizeSchemaLocation(normalizeXMLBytes(xmlInput))
	xsltInput = normalizeXMLBytes(xsltInput)

	// 1. Parse the XSLT stylesheet into a *helium.Document
	stylesheetDoc, err := parser.Parse(ctx, xsltInput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XSLT input: %w", err)
	}

	// 2. Compile the parsed stylesheet document
	compiler := xslt3.NewCompiler().URIResolver(resolver)
	stylesheet, err := compiler.Compile(ctx, stylesheetDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to compile XSLT stylesheet: %w", err)
	}

	// 3. Parse the target XML document
	sourceDoc, err := parser.Parse(ctx, xmlInput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source XML input: %w", err)
	}

	// 4. Configure transformation parameters
	inv := stylesheet.Transform(sourceDoc).URIResolver(resolver)
	if (diagram != nil && *diagram != "") || (source != nil && *source != "") {
		params := xslt3.NewParameters()
		if diagram != nil {
			params.SetString("diagram", *diagram)
		}
		params.SetString("file", *source)

		// Example of passing multiple parameters:
		// params.SetString("anotherParam", "someValue")
		// params.SetString("debugMode", "true")

		inv = inv.GlobalParameters(params)
	}

	// 5. Transform the document and write serialized output to a buffer
	var buf bytes.Buffer
	if err := inv.WriteTo(ctx, &buf); err != nil {
		return nil, fmt.Errorf("failed to execute XSLT transformation: %w", err)
	}

	return buf.Bytes(), nil
}
