package js

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// TestReplaceStringAgainstV8 compares ReplaceString with values that
// tools/reference/gen-kanban-jsreplace.mjs records from V8. The strings
// hold the replacement patterns, surrogate pairs and lone surrogates.
func TestReplaceStringAgainstV8(t *testing.T) {
	data, err := os.ReadFile("testdata/str_kanban.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases []struct {
			S, Search, Replacement, Out []rune
		}
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Cases) < 500 {
		t.Fatalf("only %d cases", len(golden.Cases))
	}
	for i, c := range golden.Cases {
		got := UTF16Units(ReplaceString(FromUTF16Units(c.S), FromUTF16Units(c.Search), FromUTF16Units(c.Replacement)))
		if !slices.Equal(got, c.Out) && !(len(got) == 0 && len(c.Out) == 0) {
			t.Errorf("case %d: replace(%U, %U, %U) = %U, want %U", i, c.S, c.Search, c.Replacement, got, c.Out)
		}
	}
}
