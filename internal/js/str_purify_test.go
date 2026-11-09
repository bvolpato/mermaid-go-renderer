package js

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"testing"
	"unicode/utf8"
)

// TestToLowerCaseGolden compares ToLowerCase with String.prototype
// .toLowerCase of V8 (tools/reference/gen-purify-jslower.mjs): for every code
// point of Unicode, and for strings around the final sigma rule.
func TestToLowerCaseGolden(t *testing.T) {
	f, err := os.Open("testdata/lower_purify.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Changed [][]json.RawMessage `json:"changed"`
		Strings [][2]string         `json:"strings"`
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	want := map[rune]string{}
	for _, pair := range golden.Changed {
		var cp rune
		var lower string
		if json.Unmarshal(pair[0], &cp) != nil || json.Unmarshal(pair[1], &lower) != nil {
			t.Fatalf("bad entry %s", pair)
		}
		want[cp] = lower
	}
	failed := 0
	for cp := rune(0); cp <= utf8.MaxRune; cp++ {
		if cp >= 0xD800 && cp <= 0xDFFF {
			continue
		}
		s := string(cp)
		expect, ok := want[cp]
		if !ok {
			expect = s
		}
		if got := ToLowerCase(s); got != expect {
			failed++
			if failed <= 10 {
				t.Errorf("ToLowerCase(U+%04X) = %q, want %q", cp, got, expect)
			}
		}
	}
	for _, pair := range golden.Strings {
		if got := ToLowerCase(pair[0]); got != pair[1] {
			failed++
			if failed <= 20 {
				t.Errorf("ToLowerCase(%q) = %q, want %q", pair[0], got, pair[1])
			}
		}
	}
	total := int(utf8.MaxRune) + 1 - 0x800 + len(golden.Strings)
	t.Logf("toLowerCase: %d of %d cases pass", total-failed, total)
}
