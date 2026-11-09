package browser

import "testing"

// TestMonospaceFontSizeAgainstChromium replays the probe cases of
// tools/reference/gen-browser-fontmono-probe.mjs: the font size of an
// element whose font family is only monospace. Chromium has a default size
// of 13px for monospace. A size that is not absolute is scaled by 13/16
// when the family changes to monospace, and back when it changes away: a
// keyword size comes from the table of the other default size. The 16px
// that mermaid sets on the svg element is absolute, so a code element in a
// label keeps the size of the label. Every value must be equal to the
// value of Chromium.
func TestMonospaceFontSizeAgainstChromium(t *testing.T) {
	macOSProbe(t)
	exact, closeTo, diff := replayProbeAll(t, "fontmono-cases.json", "fontmono-results.json", true)
	if closeTo != 0 || diff != 0 {
		t.Errorf("exact=%d close=%d diff=%d", exact, closeTo, diff)
	}
	t.Logf("exact=%d", exact)
}
