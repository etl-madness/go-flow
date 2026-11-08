package bqbilling

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"google.golang.org/api/iterator"
)

type testBillingIterator struct {
	rows []BillingExportRow
	err  error
}

func (it *testBillingIterator) Next(dst interface{}) error {
	if len(it.rows) == 0 {
		if it.err != nil {
			return it.err
		}
		return iterator.Done
	}
	*dst.(*BillingExportRow) = it.rows[0]
	it.rows = it.rows[1:]
	return nil
}

type failingWriter struct {
	calls int
	fail  int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == w.fail {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

func TestWriteBillingRows(t *testing.T) {
	for _, rows := range [][]BillingExportRow{
		nil,
		{{BillingAccountID: "one", Cost: 1.25}},
		{{BillingAccountID: "one", Cost: 1.25}, {BillingAccountID: "two", Cost: 2.5}},
	} {
		var output bytes.Buffer
		if err := writeBillingRows(&output, &testBillingIterator{rows: append([]BillingExportRow(nil), rows...)}); err != nil {
			t.Fatal(err)
		}
		var decoded []BillingExportRow
		if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
			t.Fatalf("invalid JSON %q: %v", output.String(), err)
		}
		if len(decoded) != len(rows) {
			t.Fatalf("got %d rows, want %d", len(decoded), len(rows))
		}
		for i := range rows {
			if decoded[i].BillingAccountID != rows[i].BillingAccountID || decoded[i].Cost != rows[i].Cost {
				t.Fatalf("row %d = %+v, want %+v", i, decoded[i], rows[i])
			}
		}
	}
}

func TestWriteBillingRowsErrors(t *testing.T) {
	queryErr := errors.New("query interrupted")
	var output bytes.Buffer
	if err := writeBillingRows(&output, &testBillingIterator{err: queryErr}); !errors.Is(err, queryErr) || !strings.Contains(err.Error(), "iterating") {
		t.Fatalf("iterator error = %v", err)
	}
	for call := 1; call <= 7; call++ {
		rows := []BillingExportRow{{BillingAccountID: "one"}, {BillingAccountID: "two"}}
		err := writeBillingRows(&failingWriter{fail: call}, &testBillingIterator{rows: rows})
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("write call %d: error = %v", call, err)
		}
	}
}
