package browser

import "testing"

// TestTableCellAgainstChromium replays the probe cases of
// tools/reference/gen-browser-tablecell-probe.mjs: a div of display: table
// with width: 100% and height: 100% in a foreignObject, a table-cell in it
// with vertical-align, and blocks with a percentage height. A percentage
// height refers to the height of the containing block when that height is
// definite, the height of a table is a minimum, and a cell in a table is as
// wide and as high as the table. The journey, eventmodeling and venn
// diagrams center their texts this way. Every value must be equal to the
// value of Chromium.
func TestTableCellAgainstChromium(t *testing.T) {
	macOSProbe(t)
	exact, closeTo, diff := replayProbeAll(t, "tablecell-cases.json", "tablecell-results.json", true)
	if closeTo != 0 || diff != 0 {
		t.Errorf("exact=%d close=%d diff=%d", exact, closeTo, diff)
	}
	t.Logf("exact=%d", exact)
}
