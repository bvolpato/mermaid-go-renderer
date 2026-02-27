package mermaid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFidelityFixturesRenderByDefault(t *testing.T) {
	pattern := filepath.Join("testdata", "fidelity", "*.mmd")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %q failed: %v", pattern, err)
	}
	if len(paths) == 0 {
		t.Fatalf("no fidelity fixtures found at %q", pattern)
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture %q failed: %v", path, err)
			}
			svg, err := RenderWithOptions(string(content), DefaultRenderOptions())
			if err != nil {
				t.Fatalf("render fixture %q failed: %v", path, err)
			}
			if !strings.Contains(svg, "<svg") || !strings.Contains(svg, "</svg>") {
				t.Fatalf("fixture %q did not return SVG", path)
			}
		})
	}
}
