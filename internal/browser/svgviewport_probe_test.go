package browser

import "testing"

// TestSVGViewportAgainstChromium replays the probe cases of
// tools/reference/gen-browser-svgviewport-probe.mjs. A percentage in an SVG
// length refers to the viewBox of the nearest svg element, or to its size:
// the title of a venn diagram is at x="50%". A length, and an argument of
// translate, is cut to the limits of a CSS length: the marker of today of
// a gantt chart in another century is at x = 33554428. A transform is
// about its transform-origin, as the milestone of a gantt chart is, and
// the CSS properties translate, rotate and scale come before it, as the
// scale of a highlighted slice of a pie chart does. And b and strong are
// bolder than their parent, so they have the weight 900 in a bold parent.
// Every value must be equal to the value of Chromium.
func TestSVGViewportAgainstChromium(t *testing.T) {
	macOSProbe(t)
	exact, closeTo, diff := replayProbeAll(t, "svgviewport-cases.json", "svgviewport-results.json", true)
	if closeTo != 0 || diff != 0 {
		t.Errorf("exact=%d close=%d diff=%d", exact, closeTo, diff)
	}
	t.Logf("exact=%d", exact)
}
