package admin

import (
	"bytes"
	"encoding/csv"
	"os"
	"strings"
	"testing"
)

// TestFormulaSafeCell verifies that formulaSafeCell prefixes formula-triggering
// characters and leaves safe values untouched.
func TestFormulaSafeCell(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// Formula-leading characters must be prefixed with a single quote.
		{"=SUM(A1:A10)", "'=SUM(A1:A10)"},
		{"+1234", "'+1234"},
		{"-100", "'-100"},
		{"@IMPORTRANGE()", "'@IMPORTRANGE()"},
		{"\t tabbed", "'\t tabbed"},
		{"\r return", "'\r return"},

		// Safe values must pass through unchanged.
		{"", ""},
		{"hello world", "hello world"},
		{"user@example.com", "user@example.com"}, // @ in non-leading position is safe
		{"100.00", "100.00"},
		{"2024-01-15", "2024-01-15"},
		{"normal text with = inside", "normal text with = inside"},
		{" =leading space is safe", " =leading space is safe"},
	}

	for _, tc := range cases {
		got := formulaSafeCell(tc.input)
		if got != tc.want {
			t.Errorf("formulaSafeCell(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestFormulaSafeCell_AllLeadingTriggers verifies that every documented trigger
// character is neutralised.
func TestFormulaSafeCell_AllLeadingTriggers(t *testing.T) {
	triggers := []byte{'=', '+', '-', '@', '\t', '\r'}
	for _, b := range triggers {
		input := string(b) + "payload"
		got := formulaSafeCell(input)
		if len(got) == 0 || got[0] != '\'' {
			t.Errorf("trigger char 0x%02x: expected leading quote, got %q", b, got)
		}
		if !strings.HasSuffix(got, "payload") {
			t.Errorf("trigger char 0x%02x: rest of value corrupted, got %q", b, got)
		}
	}
}

// TestExportCSV_CommaNewlineQuoteEscaping verifies that exportTable's use of
// encoding/csv.Writer correctly escapes commas, newlines, and double-quotes.
// We exercise this by writing a temp file through a real csv.Writer using the
// same pattern as exportTable, then re-reading it.
func TestExportCSV_CommaNewlineQuoteEscaping(t *testing.T) {
	tricky := [][]string{
		{"id", "name", "notes"},
		{"1", "Alice, Wonderland", "no special chars"},
		{"2", "Bob\nNewline", "newline in field"},
		{"3", `Quote "me"`, "double-quote in field"},
		{"4", "=EVIL()", "formula prefix — neutralised by formulaSafeCell"},
		{"5", "+cmd", "plus prefix"},
	}

	// Write through csv.Writer + formulaSafeCell (mirrors exportTable behaviour).
	f, err := os.CreateTemp(t.TempDir(), "export-*.csv")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	for i, row := range tricky {
		record := make([]string, len(row))
		for j, v := range row {
			if i == 0 {
				// header — no sanitisation needed
				record[j] = v
			} else {
				record[j] = formulaSafeCell(v)
			}
		}
		if err := w.Write(record); err != nil {
			t.Fatalf("csv write row %d: %v", i, err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatalf("csv flush: %v", err)
	}

	// Re-read and verify round-trip fidelity.
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv read: %v", err)
	}
	if len(records) != len(tricky) {
		t.Fatalf("expected %d rows, got %d", len(tricky), len(records))
	}

	// Header row must be verbatim.
	for j, h := range tricky[0] {
		if records[0][j] != h {
			t.Errorf("header col %d: want %q, got %q", j, h, records[0][j])
		}
	}

	// Data rows: comma/newline/quote fields must survive the round-trip intact
	// (csv.Writer handles the escaping; the value read back equals the written value).
	type check struct {
		row int
		col int
		// The value that should be present after formulaSafeCell + csv round-trip.
		want string
	}
	checks := []check{
		{1, 1, "Alice, Wonderland"},            // comma preserved
		{2, 1, "Bob\nNewline"},                 // newline preserved
		{3, 1, `Quote "me"`},                   // quote preserved
		{4, 1, "'=EVIL()"},                     // formula prefixed
		{5, 1, "'+cmd"},                        // plus prefixed
	}
	for _, ck := range checks {
		got := records[ck.row][ck.col]
		if got != ck.want {
			t.Errorf("row %d col %d: want %q, got %q", ck.row, ck.col, ck.want, got)
		}
	}
}

// TestFormulaSafeCell_EmptyAndSingleChar covers edge cases around length-1 strings
// and the empty string to ensure no index-out-of-bounds.
func TestFormulaSafeCell_EmptyAndSingleChar(t *testing.T) {
	if got := formulaSafeCell(""); got != "" {
		t.Errorf("empty string: want %q, got %q", "", got)
	}
	// Single trigger char.
	if got := formulaSafeCell("="); got != "'=" {
		t.Errorf("single '=': want \"'=\", got %q", got)
	}
	// Single safe char.
	if got := formulaSafeCell("a"); got != "a" {
		t.Errorf("single 'a': want \"a\", got %q", got)
	}
}

// BenchmarkFormulaSafeCell measures the overhead of the prefix check on hot paths.
func BenchmarkFormulaSafeCell(b *testing.B) {
	inputs := []string{
		"normal value without special chars",
		"=FORMULA(A1)",
		"user@example.com",
		"+123456",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formulaSafeCell(inputs[i%len(inputs)])
	}
}

// --- helpers ---

// readCSVFromBytes parses a CSV from an in-memory byte slice.
func readCSVFromBytes(t *testing.T, data []byte) [][]string {
	t.Helper()
	r := csv.NewReader(bytes.NewReader(data))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("readCSVFromBytes: %v", err)
	}
	return records
}
