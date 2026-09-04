package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"strings"
)

type XMLOption struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type XMLOptions struct {
	Flags []XMLOption `xml:",any"`
}

type FlowCLIOptions struct {
	XMLName xml.Name   `xml:"flow_cli_options"`
	Options XMLOptions `xml:"options"`
}

// applyXMLOptions parses the specified XML file wrapped in <flow_cli_options><options>
// and sets any flags that were not explicitly passed on the command line.
func applyXMLOptions(loadPath string) error {
	data, err := os.ReadFile(loadPath)
	if err != nil {
		return fmt.Errorf("error reading load file: %w", err)
	}

	var root FlowCLIOptions
	if err := xml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("error parsing XML load file: %w", err)
	}

	// Track flags explicitly provided on the CLI command line
	cliSetFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		cliSetFlags[f.Name] = true
	})

	for _, opt := range root.Options.Flags {
		flagName := opt.XMLName.Local
		flagVal := strings.TrimSpace(opt.Value)

		// Prevent infinite recursion if a load tag exists inside XML
		if flagName == "load" {
			continue
		}

		// Apply value from XML only if the flag was not explicitly provided on the CLI
		if !cliSetFlags[flagName] {
			if f := flag.Lookup(flagName); f != nil {
				if err := flag.Set(flagName, flagVal); err != nil {
					return fmt.Errorf("failed setting flag -%s to %q from load XML: %w", flagName, flagVal, err)
				}
			} else {
				return fmt.Errorf("unknown CLI flag -%s found in %s", flagName, loadPath)
			}
		}
	}

	return nil
}
