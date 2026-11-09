package browser

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
)

// macOSProbe skips a test that compares the engine with Chromium on macOS
// unless this machine has the system fonts of the macOS machine that
// recorded the expected values. Such a test uses the system fonts, and its
// expected values depend on the fonts, the font rasterizer and the C
// library of macOS. So Linux skips it, and a Mac with other versions of the
// fonts skips it too. The tests in linux_probe_test.go replay the same
// cases against Chromium on Linux with the embedded fonts, on every
// platform.
func macOSProbe(t *testing.T) {
	t.Helper()
	fonttest.RequireMacOSReference(t)
}
