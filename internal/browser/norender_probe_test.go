package browser

import "testing"

// TestNoLayoutObjectAgainstChromium replays the probe cases of
// tools/reference/gen-browser-norender-probe.mjs: SVG elements that have no
// layout object in Blink. Such an element has no box: getBBox and
// getBoundingClientRect return 0, 0, 0, 0 and getComputedTextLength returns
// 0. The cases have a switch element, where only the first valid child has
// a layout object (the journey diagram depends on it), the conditional
// processing attributes, and display: none, where a g element is a hidden
// container whose descendants still have a box for getBBox. Every value
// must be equal to the value of Chromium.
func TestNoLayoutObjectAgainstChromium(t *testing.T) {
	macOSProbe(t)
	exact, closeTo, diff := replayProbeAll(t, "norender-cases.json", "norender-results.json", true)
	if closeTo != 0 || diff != 0 {
		t.Errorf("exact=%d close=%d diff=%d", exact, closeTo, diff)
	}
	t.Logf("exact=%d", exact)
}
