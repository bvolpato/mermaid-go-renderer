package mermaid

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFidelityFixturesRenderByDefault(t *testing.T) {
	if os.Getenv("MMDG_CONFORMANCE") != "1" {
		t.Skip("set MMDG_CONFORMANCE=1 to run mmdc image conformance checks")
	}
	if _, err := exec.LookPath("mmdc"); err != nil {
		t.Skip("mmdc not found in PATH")
	}

	const minimumFixtureCount = 45
	pattern := filepath.Join("testdata", "fidelity", "*.mmd")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %q failed: %v", pattern, err)
	}
	if len(paths) < minimumFixtureCount {
		t.Fatalf("expected at least %d fidelity fixtures at %q, found %d", minimumFixtureCount, pattern, len(paths))
	}

	const width = 1600
	const height = 1200

	for _, path := range paths {
		path := path
		fixtureName := strings.TrimSuffix(filepath.Base(path), ".mmd")
		t.Run(fixtureName, func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture %q failed: %v", path, err)
			}
			diagram := string(content)

			refImg, refPNG := renderWithMMDCPNG(t, diagram, width, height)

			gotSVG, err := RenderWithOptions(
				diagram,
				DefaultRenderOptions().WithAllowApproximate(true),
			)
			if err != nil {
				t.Fatalf("render fixture %q failed: %v", path, err)
			}

			if !strings.Contains(gotSVG, "<svg") || !strings.Contains(gotSVG, "</svg>") {
				t.Fatalf("fixture %q did not return SVG", path)
			}

			gotImg, err := rasterizeSVG(gotSVG, width, height)
			if err != nil {
				savePartialArtifacts(t, fixtureName+"-fidelity", gotSVG, refPNG)
				t.Fatalf("rasterize rendered svg: %v", err)
			}

			mismatch, meanDelta, considered := compareDrawnPixels(gotImg, refImg)
			t.Logf(
				"fixture=%s mismatch=%.4f mean_delta=%.4f considered_pixels=%d",
				fixtureName, mismatch, meanDelta, considered,
			)

			// Alert if mismatch is heavily breaking, ~0.35 is mostly safe across styles
			const maxMismatch = 0.35
			if mismatch > maxMismatch {
				saveConformanceArtifacts(t, fixtureName+"-fidelity", gotSVG, refPNG, gotImg, refImg)
				t.Errorf(
					"fidelity mismatch %.4f exceeds threshold %.4f for %s",
					mismatch, maxMismatch, fixtureName,
				)
			}
		})
	}
}
