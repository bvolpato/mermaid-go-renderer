package set3

import (
	"path/filepath"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// TestShapesAgainstMermaid replays the golden files of this package: cases
// recorded from the real insertNode of mermaid in Chromium. The cases come
// from tools/reference/gen-shape-cases-set3.mjs.
func TestShapesAgainstMermaid(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.json.gz")
	if len(files) == 0 {
		t.Fatal("no golden files in testdata")
	}
	rutest.RunShapeGolden(t, files...)
}
