package set2

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// shapeFiles returns the names of the golden files of this package without
// the extension: one handler name each.
func shapeFiles(t *testing.T) []string {
	t.Helper()
	files, _ := filepath.Glob("testdata/*.json.gz")
	if len(files) == 0 {
		t.Fatal("no golden files in testdata")
	}
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = strings.TrimSuffix(filepath.Base(f), ".json.gz")
	}
	return names
}

// TestShapesAgainstMermaid replays the golden files of this package: cases
// recorded from the real insertNode of mermaid in Chromium. The cases come
// from tools/reference/gen-shape-cases-set2.mjs.
func TestShapesAgainstMermaid(t *testing.T) {
	var paths []string
	for _, shape := range shapeFiles(t) {
		paths = append(paths, "testdata/"+shape+".json.gz")
	}
	rutest.RunShapeGolden(t, paths...)
}
