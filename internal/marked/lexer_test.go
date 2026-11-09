package marked

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// goldenCase is one case of lexer.json.gz. The generator is
// tools/reference/gen-marked.mjs.
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
		return str(c.Units)
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

// TestLexer compares the token trees with the ones of marked 16.3.0: for
// the label-like strings of the corpus, for specification examples, for
// generated strings, and for the whole text of each corpus fixture.
func TestLexer(t *testing.T) {
	var cases []goldenCase
	loadGolden(t, "lexer.json.gz", &cases)
	if len(cases) < 45000 {
		t.Fatalf("the golden file has %d cases", len(cases))
	}
	failed, files, sums := 0, 0, 0
	for i := range cases {
		c := &cases[i]
		in := c.input(t)
		tokens, err := Lexer(in)
		if err != nil {
			t.Errorf("case %d: Lexer(%q): %v", i, in, err)
			failed++
			continue
		}
		got := JSON(tokens)
		if c.File != "" {
			files++
		}
		if c.Sum != "" {
			sums++
			if sum16(got) == c.Sum {
				continue
			}
		} else if got == c.Want {
			continue
		}
		failed++
		if failed <= 10 {
			label := c.File
			if label == "" {
				label = in
			}
			t.Errorf("case %d: %q\n got %s\nwant %s%s", i, label, got, c.Want, c.Sum)
		}
	}
	t.Logf("%d cases (%d fixtures, %d compared by sum), %d failed", len(cases), files, sums, failed)
	if failed > 0 {
		t.Errorf("%d of %d cases failed", failed, len(cases))
	}
}
