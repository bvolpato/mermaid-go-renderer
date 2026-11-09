package browser

import "testing"

// TestLineWrapAgainstChromium replays the probe cases of
// tools/reference/gen-browser-linewrap-probe.mjs: a text that wraps in a
// block of a fixed width. A part of a text on a line is as wide as the sum
// of its glyphs, rounded up to a LayoutUnit. A line that starts in the
// middle of a text is shaped again at its start, so its first glyph has no
// kerning from the space before it. And when the content after the last
// break opportunity of a line does not fit, the line ends at that break
// opportunity, also when the content is in an inline element such as
// strong. Every value must be equal to the value of Chromium.
func TestLineWrapAgainstChromium(t *testing.T) {
	macOSProbe(t)
	exact, closeTo, diff := replayProbeAll(t, "linewrap-cases.json", "linewrap-results.json", true)
	if closeTo != 0 || diff != 0 {
		t.Errorf("exact=%d close=%d diff=%d", exact, closeTo, diff)
	}
	t.Logf("exact=%d", exact)
}
