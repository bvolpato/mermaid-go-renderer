package browser

import "testing"

// TestArcBoundsAgainstChromium replays the probe cases of
// tools/reference/gen-browser-arc-probe.mjs: the bounding boxes of paths
// with elliptical arcs that start or end on an axis of their ellipse, such
// as the arcs of the bucket shape of mermaid. The values must be equal to
// the float32 values of Chromium. SkPath::arcTo takes the angles of an arc
// from atan2f of the C library, and for the leftmost point of an ellipse
// the C library of macOS returns the float32 below pi. The bounding box of
// an arc around x 0 is then one float32 step wider.
func TestArcBoundsAgainstChromium(t *testing.T) {
	replayProbeWith(t, "arc-cases.json", "arc-results.json", probeOptions{exactOnly: true})
}
