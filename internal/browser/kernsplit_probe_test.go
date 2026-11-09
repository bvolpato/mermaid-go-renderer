package browser

import "testing"

// TestKernSplitAgainstChromium replays the probe cases of
// tools/reference/gen-browser-kernsplit-probe.mjs: the widths of the words
// of an SVG label in a font that kerns with the legacy kern table. The kern
// machine of HarfBuzz halves the scaled kerning of a pair, so the first
// glyph of a pair with an odd value gets half a font unit. Every value
// must be equal to the value of Chromium.
func TestKernSplitAgainstChromium(t *testing.T) {
	replayProbeWith(t, "kernsplit-cases.json", "kernsplit-results.json", probeOptions{exactOnly: true})
}
