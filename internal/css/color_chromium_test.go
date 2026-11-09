package css

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// TestColorAgainstChromium compares the color that ParseColor makes of a
// text in the hsl(), hsla(), rgb() or rgba() notation with the color that
// Chromium prints for it when a script sets it on a style. The cases are in
// testdata/color-cases.json.gz: results of khroma for random colors, random
// texts, colors whose channels are close to the middle between two 8-bit
// values, colors that tell apart the ways in which the conversion can
// round, the same colors in other notations, alpha values, and rgb() with
// numbers that are not integers. An empty result is a value that Chromium
// does not accept. Regenerate the file with
// node tools/reference/gen-css-color-probe.mjs.
func TestColorAgainstChromium(t *testing.T) {
	f, err := os.Open("testdata/color-cases.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases [][2]string `json:"cases"`
	}
	if err := json.NewDecoder(zr).Decode(&golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Cases) < 8000 {
		t.Fatalf("only %d cases", len(golden.Cases))
	}
	bad := 0
	for _, c := range golden.Cases {
		input, want := c[0], c[1]
		got := ""
		if color, ok := ParseColor(input); ok {
			got = color.CSSText()
		}
		if got != want {
			bad++
			if bad <= 20 {
				t.Errorf("%s: %q, Chromium has %q", input, got, want)
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d colors differ from Chromium", bad, len(golden.Cases))
	}
	t.Logf("%d colors print as in Chromium", len(golden.Cases)-bad)
}
