package browser

import "testing"

// TestBoundsChangeAgainstChromium replays the probe cases of
// tools/reference/gen-browser-boundschange-probe.mjs: the bounding box of a
// container whose descendants have empty boxes at the origin. Blink
// computes the box of a container only when the layout of a child reports
// that bounds changed, and a group whose box stays the empty rectangle at
// the origin does not report it. So such a group is a part of the box of
// its parent, and its parent is not a part of the box of the next level.
// Every value must be equal to the value of Chromium.
func TestBoundsChangeAgainstChromium(t *testing.T) {
	replayProbeWith(t, "boundschange-cases.json", "boundschange-results.json", probeOptions{exactOnly: true})
}
