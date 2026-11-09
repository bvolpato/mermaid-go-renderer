package markdown

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// goldenCase is one case of markdown.json.gz. The generator is
// tools/reference/gen-markdown.mjs.
type goldenCase struct {
	In    *string `json:"in"`
	Units []rune  `json:"units"`
	File  string  `json:"file"`
	Want  string  `json:"want"`
	Sum   string  `json:"sum"`
}

func loadGolden(t testing.TB, name string, v any) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(zr).Decode(v); err != nil {
		t.Fatal(err)
	}
}

// input returns the Go string of a case. Code units with a surrogate that
// has no partner become the three-byte form of internal/js.
func (c *goldenCase) input(t testing.TB) string {
	switch {
	case c.In != nil:
		return *c.In
	case c.Units != nil:
		return js.FromUTF16Units(c.Units)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(c.File)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// sum16 is the sum of the generator: the first 16 hex digits of the
// SHA-256 of s.
func sum16(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:16]
}

// linesJSON is JSON.stringify for an array of MarkdownLine.
func linesJSON(lines [][]Word) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, line := range lines {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(lineJSON(line))
	}
	sb.WriteByte(']')
	return sb.String()
}

// lineJSON is JSON.stringify for one MarkdownLine.
func lineJSON(line []Word) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, w := range line {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"content":`)
		sb.WriteString(js.QuoteJSON(w.Content))
		sb.WriteString(`,"type":`)
		sb.WriteString(js.QuoteJSON(w.Type))
		sb.WriteByte('}')
	}
	sb.WriteByte(']')
	return sb.String()
}

func config(t testing.TB, text string) *js.Obj[any] {
	t.Helper()
	v, err := js.ParseJSON([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return js.AsObj(v)
}

// TestMarkdown compares MarkdownToLines, MarkdownToHTML, NonMarkdownToLines
// and NonMarkdownToHTML with the functions of mermaid, with
// markdownAutoWrap true and false.
func TestMarkdown(t *testing.T) {
	var cases []goldenCase
	loadGolden(t, "markdown.json.gz", &cases)
	if len(cases) < 45000 {
		t.Fatalf("the golden file has %d cases", len(cases))
	}
	wrap := config(t, `{"markdownAutoWrap":true}`)
	noWrap := config(t, `{"markdownAutoWrap":false}`)
	// The generator checks that upstream returns the value of wrap for
	// these configs too.
	others := []*js.Obj[any]{nil, config(t, `{}`), config(t, `{"markdownAutoWrap":null}`), config(t, `{"htmlLabels":false}`)}

	// check returns the message of a case that fails.
	check := func(i int) string {
		c := &cases[i]
		in := c.input(t)
		lines := linesJSON(MarkdownToLines(in, wrap))
		html := MarkdownToHTML(in, wrap)
		got := "[" + lines + "," + js.QuoteJSON(html) + "," + js.QuoteJSON(MarkdownToHTML(in, noWrap)) + "," +
			linesJSON(NonMarkdownToLines(in)) + "," + js.QuoteJSON(NonMarkdownToHTML(in)) + "]"

		ok := got == c.Want
		if c.Sum != "" {
			ok = sum16(got) == c.Sum
		}
		// The other configs: one case in eight, to save time.
		if ok && i%8 == 0 && c.File == "" {
			if linesJSON(MarkdownToLines(in, noWrap)) != lines {
				ok = false
				got += " and MarkdownToLines depends on markdownAutoWrap"
			}
			for _, other := range others {
				if linesJSON(MarkdownToLines(in, other)) != lines || MarkdownToHTML(in, other) != html {
					ok = false
					got += " and the result depends on a config without markdownAutoWrap"
				}
			}
		}
		if ok {
			return ""
		}
		label := c.File
		if label == "" {
			label = in
		}
		return fmt.Sprintf("case %d: %q\n got %s\nwant %s%s", i, label, got, c.Want, c.Sum)
	}

	const workers = 8
	var failed atomic.Int64
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for i := w; i < len(cases); i += workers {
				if msg := check(i); msg != "" && failed.Add(1) <= 10 {
					t.Error(msg)
				}
			}
		})
	}
	wg.Wait()
	sums := 0
	for i := range cases {
		if cases[i].Sum != "" {
			sums++
		}
	}
	t.Logf("%d cases (%d compared by sum), %d failed", len(cases), sums, failed.Load())
	if failed.Load() > 0 {
		t.Errorf("%d of %d cases failed", failed.Load(), len(cases))
	}
}
