package js

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// TestStringsAgainstV8 compares the UTF-16 string functions with values that
// tools/reference/gen-jison-str.mjs records from V8. The strings hold ASCII,
// characters of every UTF-8 length, surrogate pairs and lone surrogates.
func TestStringsAgainstV8(t *testing.T) {
	data, err := os.ReadFile("testdata/str_jison.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases []struct {
			U, T                                                 []rune
			A, B, C                                              int
			Slice, SliceFrom, Substr, CharAt, Concat, WellFormed []rune
			JSON                                                 string
		}
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Cases) < 500 {
		t.Fatalf("only %d cases", len(golden.Cases))
	}
	same := func(name string, i int, got string, want []rune) {
		t.Helper()
		if gotUnits := UTF16Units(got); !slices.Equal(gotUnits, want) && !(len(gotUnits) == 0 && len(want) == 0) {
			t.Errorf("case %d: %s = %U, want %U", i, name, gotUnits, want)
		}
	}
	for i, c := range golden.Cases {
		s := FromUTF16Units(c.U)
		other := FromUTF16Units(c.T)
		same("round trip", i, s, c.U)
		same("slice", i, StringSlice(s, c.A, c.B), c.Slice)
		same("slice from", i, StringSliceFrom(s, c.A), c.SliceFrom)
		same("substr", i, Substr(s, c.A, c.B), c.Substr)
		same("charAt", i, CharAt(s, c.C), c.CharAt)
		same("concat", i, Concat(s, other), c.Concat)
		same("toWellFormed", i, ToWellFormed(s), c.WellFormed)
		if got := QuoteJSON(s); got != c.JSON {
			t.Errorf("case %d: QuoteJSON = %s, want %s", i, got, c.JSON)
		}
	}
}
