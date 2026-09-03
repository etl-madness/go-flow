package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/etl-madness/flow"
)

type PrintableResult struct {
	ScriptID      string      `json:"script_id"`
	ReturnCode    any         `json:"return_code"`
	Duration      any         `json:"duration"`
	ResultsString interface{} `json:"results_string"`
}
type TextSink struct {
	Writer io.Writer
}

var lineReturnsReplacer = strings.NewReplacer(
	"\r\n", "\n",
	`\r\n`, "\n",
	"\r", "",
)

var jsonLineBreakReplacer = strings.NewReplacer(
	`\\`, `\\`,
	`\r\n`, "",
	`\n`, "",
	`\r`, "",
)

func formatLineReturns(s string) string {
	return lineReturnsReplacer.Replace(s)
}
func (s TextSink) Emit(_ context.Context, event flow.ExecutionEvent) error {
	_, err := fmt.Fprintf(
		s.Writer,
		"%s,%s,%s,%d,%s,%s,%s,%s,%s,%d\n",
		event.OccurredAt.UTC().Format(time.RFC3339),
		event.RunID,
		event.ExecutionID,
		event.Sequence,
		event.Type,
		event.NodeKind,
		event.NodeID,
		event.Status,
		event.ErrorMessage,
		event.RowCounts.Affected,
	)
	return err
}

func outputRawJSON(res *[]flow.ScriptResult) {
	jsonBytes, err := json.Marshal(res)
	if err != nil {
		fmt.Printf("[{\"script_id\": \"system\", \"return_code\": 1, \"results_string\": \"JSON encoding error: %v\"}]\n", err)
		return
	}
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	// Pass JSON output through line return cleanup before printing to stdout
	w.WriteString(formatLineReturns(string(jsonBytes)))
	w.WriteByte('\n')
}

func outputJSON(res *[]flow.ScriptResult) {
	var printable []PrintableResult

	for _, r := range *res {
		cleanStr := formatLineReturns(r.ResultsString)
		var val interface{} = cleanStr

		cleanBytes := []byte(cleanStr)
		// Embed as raw JSON object if ResultsString contains valid JSON
		if json.Valid(cleanBytes) {
			val = json.RawMessage(cleanBytes)
		}

		printable = append(printable, PrintableResult{
			ScriptID:      r.ScriptID,
			ReturnCode:    r.ReturnCode,
			ResultsString: val,
			Duration:      r.Duration,
		})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(printable); err != nil {
		fmt.Printf("[{\"script_id\": \"system\", \"return_code\": 1, \"results_string\": \"JSON encoding error: %v\"}]\n", err)
		return
	}

	// Unescape JSON-encoded literal newline characters (\n and \r\n) inside the marshaled buffer
	outputStr := jsonLineBreakReplacer.Replace(buf.String())

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	w.WriteString(outputStr)
}

func outputText(res *[]flow.ScriptResult) {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for _, r := range *res {
		fmt.Fprintf(w, "ScriptID: %s\nReturnCode: %d\nDuration: %v\nResultsString: %s\n\n",
			r.ScriptID, r.ReturnCode, r.Duration, r.ResultsString)
	}
}

func outputMarkdownTable(res *[]flow.ScriptResult) {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	fmt.Fprintln(w, "| Script ID | Return Code | Duration | Results |")
	fmt.Fprintln(w, "| :--- | :--- | :--- | :--- |")
	for _, r := range *res {
		// Escape newlines and pipe symbols to prevent table formatting breaks
		cleanResults := strings.ReplaceAll(r.ResultsString, "\n", "<br>")
		cleanResults = strings.ReplaceAll(cleanResults, "|", "\\|")
		fmt.Fprintf(w, "| %s | %d | %v | %s |\n", r.ScriptID, r.ReturnCode, r.Duration, cleanResults)
	}
}

func outputCSV(res *[]flow.ScriptResult) {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for _, r := range *res {
		// Escape newlines and pipe symbols to prevent table formatting breaks
		fmt.Fprintf(w, " %s,  %d,  %v,  %s\n", r.ScriptID, r.ReturnCode, r.Duration, r.ResultsString)
	}
}

func outputSummary(run flow.RunResult, file *string, config *string) {
	var configStr string
	if config != nil {
		configStr = *config
	}
	fmt.Println("\n\nutc_runtime,file,config,status,started,finished,task_count")
	log.Printf(
		"%s,%s,%s,%s,%s,%s,%d",
		time.Now().UTC().Format(time.RFC3339),
		*file,
		configStr,
		run.Status,
		run.StartedAt.UTC().Format(time.RFC3339),
		run.FinishedAt.UTC().Format(time.RFC3339),
		len(run.Nodes),
	)

}
