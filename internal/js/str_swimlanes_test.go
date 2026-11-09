package js

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// TestLocaleCompare compares LocaleCompare with String.localeCompare of
// Chromium. The golden file is written by
// tools/reference/gen-js-localecompare.mjs. Each pair has the result of
// ICU and the result of the table model that LocaleCompare implements. The
// Go code must give the result of the model for every pair. For the sets
// that are marked exact (ASCII strings), the model is the result of ICU.
func TestLocaleCompare(t *testing.T) {
	f, err := os.Open("testdata/localecompare.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Sets []struct {
			Name  string  `json:"name"`
			Exact bool    `json:"exact"`
			Agree int     `json:"agree"`
			Pairs [][]any `json:"pairs"`
		} `json:"sets"`
	}
	if err := json.NewDecoder(zr).Decode(&golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Sets) == 0 {
		t.Fatal("no sets")
	}
	for _, set := range golden.Sets {
		agree := 0
		failures := 0
		for _, pair := range set.Pairs {
			a, b := pair[0].(string), pair[1].(string)
			real, model := int(pair[2].(float64)), int(pair[3].(float64))
			got := LocaleCompare(a, b)
			if got == real {
				agree++
			}
			want := model
			if set.Exact {
				want = real
			}
			if got != want {
				failures++
				if failures <= 10 {
					t.Errorf("%s: LocaleCompare(%q, %q) = %d, want %d (ICU %d, model %d)", set.Name, a, b, got, want, real, model)
				}
			}
		}
		if agree != set.Agree {
			t.Errorf("%s: %d pairs agree with ICU, the generator counted %d", set.Name, agree, set.Agree)
		}
		t.Logf("%s: %d pairs, %d agree with ICU, exact=%v", set.Name, len(set.Pairs), agree, set.Exact)
	}
}
