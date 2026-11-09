package browser

import (
	"encoding/json"
	"os"
	"testing"
)

// TestMinContentAgainstChromium replays the probe cases of
// tools/reference/gen-browser-mincontent-probe.mjs: the width of a div of
// display: table that is narrower than its widest word. The div is then as
// wide as the word (its min-content width). Blink measures a word of the
// min-content width as the difference of the positions of its two ends in
// the text, each rounded up to a LayoutUnit, so the width can be 1/64 px
// less than the width of the same word on a line. The spaces at the start
// of a line, in a text that keeps its spaces, are an item of their own, and
// the positions start again after them. The words break where the table of
// TestLineBreakASCIIAgainstChromium says. Every value must be equal to the
// value of Chromium.
func TestMinContentAgainstChromium(t *testing.T) {
	macOSProbe(t)
	replayProbeWith(t, "mincontent-cases.json", "mincontent-results.json", probeOptions{exactOnly: true})
}

// TestLineBreakASCIIAgainstChromium checks the table of break opportunities
// between two printable ASCII characters against the answers of Chromium
// (tools/reference/gen-browser-linebreak-probe.mjs), and checks that the
// line breaker of the engine follows the table: a block that is 1px wide
// with two characters has two lines exactly when the table has a break
// between them.
func TestLineBreakASCIIAgainstChromium(t *testing.T) {
	var golden struct {
		First       int      `json:"first"`
		Last        int      `json:"last"`
		Rows        []string `json:"rows"`
		HyphenDigit string   `json:"hyphenDigit"`
		AtStart     string   `json:"hyphenDigitAtStart"`
	}
	data, err := os.ReadFile("testdata/linebreak-ascii.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.First != asciiBreakFirst || golden.Last != asciiBreakLast || len(golden.Rows) != len(asciiBreakTable) {
		t.Fatalf("the table is %d..%d with %d rows, Chromium has %d..%d with %d rows", asciiBreakFirst, asciiBreakLast, len(asciiBreakTable), golden.First, golden.Last, len(golden.Rows))
	}
	// lines lays out a text in a narrow block and returns the number of
	// lines.
	lines := func(text string) int {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		svg := container.Append("svg").Attr("id", "my-svg").Attr("width", "100%")
		fo := svg.Append("foreignObject").Attr("width", 2000).Attr("height", 2000)
		div := fo.Append("xhtml:div").Attr("style", "width: 1px; font: 10px/10px arial; white-space: normal")
		div.Text(text)
		return int(div.BoundingClientRect().Height / 10)
	}
	pairs, bad := 0, 0
	for a := golden.First; a <= golden.Last; a++ {
		row := golden.Rows[a-golden.First]
		if row != asciiBreakTable[a-golden.First] {
			t.Errorf("row %q of the table\n got  %s\n want %s", rune(a), asciiBreakTable[a-golden.First], row)
		}
		for b := golden.First; b <= golden.Last; b++ {
			pairs++
			want := 1
			if row[b-golden.First] == '1' {
				want = 2
			}
			if got := lines(string([]rune{rune(a), rune(b)})); got != want {
				bad++
				if bad <= 20 {
					t.Errorf("%q has %d lines, Chromium has %d", string([]rune{rune(a), rune(b)}), got, want)
				}
			}
		}
	}
	if golden.HyphenDigit != asciiBreakHyphenDigit {
		t.Errorf("hyphen and digit\n got  %s\n want %s", asciiBreakHyphenDigit, golden.HyphenDigit)
	}
	// A digit after a hyphen: the break depends on the character before the
	// hyphen. The break before the hyphen, when the table has one, is one
	// more line.
	hyphen := '-' - rune(golden.First)
	for a := golden.First; a <= golden.Last; a++ {
		want := 1
		if golden.Rows[a-golden.First][hyphen] == '1' {
			want++
		}
		if golden.HyphenDigit[a-golden.First] == '1' {
			want++
		}
		if got := lines(string([]rune{rune(a), '-', '1'})); got != want {
			t.Errorf("%q has %d lines, Chromium has %d", string([]rune{rune(a), '-', '1'}), got, want)
		}
	}
	if got, want := lines("-1"), 1+int(golden.AtStart[0]-'0'); got != want {
		t.Errorf("%q has %d lines, Chromium has %d", "-1", got, want)
	}
	t.Logf("%d pairs", pairs)
}
