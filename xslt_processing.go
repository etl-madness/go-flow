package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/etl-madness/flow"
	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xslt3"
)

type defaultURIResolver struct {
	client *http.Client
}

func newDefaultURIResolver() *defaultURIResolver {
	return &defaultURIResolver{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *defaultURIResolver) resolve(rawURI string) (io.ReadCloser, error) {
	if strings.HasSuffix(rawURI, "pipeline.xsd") {
		return io.NopCloser(bytes.NewReader(flow.GetSchemaXSD())), nil
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
		return resp.Body, nil
	}

	filePath := rawURI
	if strings.HasPrefix(filePath, "file://") {
		filePath = strings.TrimPrefix(filePath, "file://")
		if runtime.GOOS == "windows" && strings.HasPrefix(filePath, "/") && len(filePath) > 2 && filePath[2] == ':' {
			filePath = filePath[1:]
		}
	}
	return os.Open(filePath)
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
