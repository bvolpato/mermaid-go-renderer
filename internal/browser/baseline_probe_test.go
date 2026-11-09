package browser

import "testing"

// TestTextBaselineAgainstChromium replays the probe cases of
// tools/reference/gen-browser-baseline-probe.mjs: the bounding box of an
// SVG text with a dominant-baseline or an alignment-baseline. Blink rounds
// the ascent and the descent of the baseline type to 1/64 px separately,
// so the box can be 1/64 px taller than ascent + descent.
func TestTextBaselineAgainstChromium(t *testing.T) {
	replayProbe(t, "baseline-cases.json", "baseline-results.json")
}
