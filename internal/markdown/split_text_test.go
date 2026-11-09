package markdown

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// splitGolden is split.json.gz. The generator is
// tools/reference/gen-markdown.mjs.
type splitGolden struct {
	Chars []struct {
		In    *string `json:"in"`
		Units []rune  `json:"units"`
		Want  string  `json:"want"`
	} `json:"chars"`
	Splits []struct {
		Line     string `json:"line"`
		Fit      string `json:"fit"`
		N        int    `json:"n"`
		Want     string `json:"want"`
		Error    string `json:"error"`
		Calls    int    `json:"calls"`
		CallsSum string `json:"callsSum"`
	} `json:"splits"`
}

// stringsJSON is JSON.stringify for an array of strings.
func stringsJSON(list []string) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, s := range list {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(js.QuoteJSON(s))
	}
	sb.WriteByte(']')
	return sb.String()
}

// TestSplitTextToChars compares the grapheme clusters with the segments of
// Intl.Segmenter in V8.
func TestSplitTextToChars(t *testing.T) {
	var golden splitGolden
	loadGolden(t, "split.json.gz", &golden)
	if len(golden.Chars) < 5000 {
		t.Fatalf("the golden file has %d cases", len(golden.Chars))
	}
	failed := 0
	for i, c := range golden.Chars {
		in := js.FromUTF16Units(c.Units)
		if c.In != nil {
			in = *c.In
		}
		if got := stringsJSON(SplitTextToChars(in)); got != c.Want {
			failed++
			if failed <= 20 {
				t.Errorf("case %d: SplitTextToChars(%+q)\n got %+q\nwant %+q", i, in, got, c.Want)
			}
		}
	}
	t.Logf("%d cases, %d failed", len(golden.Chars), failed)
	if failed > 0 {
		t.Errorf("%d of %d cases failed", failed, len(golden.Chars))
	}
}

// fits returns the checkFit function of the generator with this name.
func fits(t *testing.T, kind string, n int) func(line []Word) bool {
	switch kind {
	case "units":
		// The line has at most n UTF-16 code units, with one more for each
		// gap between two words.
		return func(line []Word) bool {
			sum := max(len(line)-1, 0)
			for _, w := range line {
				sum += js.UTF16Len(w.Content)
			}
			return sum <= n
		}
	case "graphemes":
		// The line has at most n graphemes.
		return func(line []Word) bool {
			sum := 0
			for _, w := range line {
				sum += len(SplitTextToChars(w.Content))
			}
			return sum <= n
		}
	case "words":
		// The line has at most n words.
		return func(line []Word) bool { return len(line) <= n }
	case "never":
		return func([]Word) bool { return false }
	case "always":
		return func([]Word) bool { return true }
	}
	t.Fatalf("unknown checkFit function %q", kind)
	return nil
}

// TestSplitLineToFitWidth compares the lines with the ones of mermaid, and
// also the lines that checkFit gets, in order.
func TestSplitLineToFitWidth(t *testing.T) {
	var golden splitGolden
	loadGolden(t, "split.json.gz", &golden)
	if len(golden.Splits) < 8000 {
		t.Fatalf("the golden file has %d cases", len(golden.Splits))
	}
	failed, errors := 0, 0
	for i, c := range golden.Splits {
		var raw []struct {
			Content string `json:"content"`
			Type    string `json:"type"`
		}
		if err := json.Unmarshal([]byte(c.Line), &raw); err != nil {
			t.Fatal(err)
		}
		line := make([]Word, len(raw))
		for j, w := range raw {
			line[j] = Word{Content: w.Content, Type: w.Type}
		}
		before := lineJSON(line)

		checkFit := fits(t, c.Fit, c.N)
		var calls []string
		lines, err := SplitLineToFitWidth(line, func(l []Word) bool {
			calls = append(calls, lineJSON(l))
			return checkFit(l)
		})
		got, gotErr := "", ""
		if err != nil {
			gotErr = err.Error()
			errors++
		} else {
			got = linesJSON(lines)
		}
		callsSum := sum16(stringsJSON(calls))
		if got == c.Want && gotErr == c.Error && len(calls) == c.Calls && callsSum == c.CallsSum && lineJSON(line) == before {
			continue
		}
		failed++
		if failed <= 10 {
			t.Errorf("case %d: SplitLineToFitWidth(%s, %s %d)\n got %s %s (%d calls, %s)\nwant %s %s (%d calls, %s)",
				i, c.Line, c.Fit, c.N, got, gotErr, len(calls), callsSum, c.Want, c.Error, c.Calls, c.CallsSum)
		}
	}
	t.Logf("%d cases (%d with an error), %d failed", len(golden.Splits), errors, failed)
	if failed > 0 {
		t.Errorf("%d of %d cases failed", failed, len(golden.Splits))
	}
}
