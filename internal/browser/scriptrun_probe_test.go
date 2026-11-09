package browser

import "testing"

// TestScriptRunsAgainstChromium replays the probe cases of
// tools/reference/gen-browser-scriptrun-probe.mjs: texts that mix scripts.
// Blink shapes each run of one script alone, so that a kerning pair does not
// apply across the limit of two runs.
func TestScriptRunsAgainstChromium(t *testing.T) {
	replayProbe(t, "scriptrun-cases.json", "scriptrun-results.json")
}
