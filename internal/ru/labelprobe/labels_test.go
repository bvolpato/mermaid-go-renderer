// Package labelprobe has the end-to-end test of the measurement engine for
// the labels of the unified renderer. It has no code of its own.
package labelprobe

import (
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
	// The rect shape, which goes through labelHelper and createText.
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
)

// TestLabelsAgainstMermaid replays testdata/labels.json.gz, which
// tools/reference/gen-labels.mjs records: a rect node with each label text
// and each label setting, built by the real insertNode of mermaid in
// Chromium. labelHelper measures the label in the browser and sizes the
// node from it, so the markup has the measured size of the label. The Go
// port must build the same markup with the same numbers: the test fails for
// a case whose markup or size differs.
func TestLabelsAgainstMermaid(t *testing.T) {
	if font.System().Platform() != font.PlatformDarwin {
		t.Skip("the expected values are those of Chromium on macOS")
	}
	// MMDG_LABELS names another golden file, to try new cases.
	path := "testdata/labels.json.gz"
	if p := os.Getenv("MMDG_LABELS"); p != "" {
		path = p
	}
	rutest.RunShapeGolden(t, path)
}
