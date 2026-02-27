package mermaid

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type conformanceFixture struct {
	Name        string
	Diagram     string
	MaxMismatch float64
}

func TestConformanceAgainstMMDC(t *testing.T) {
	if os.Getenv("MMDG_CONFORMANCE") != "1" {
		t.Skip("set MMDG_CONFORMANCE=1 to run mmdc image conformance checks")
	}
	if _, err := exec.LookPath("mmdc"); err != nil {
		t.Skip("mmdc not found in PATH")
	}

	fixtures := []conformanceFixture{
		{
			Name: "flowchart_basic",
			Diagram: `flowchart LR
  A[Start] --> B{Check}
  B -->|yes| C[Done]
  B -->|no| D[Retry]
  D --> B`,
			MaxMismatch: 0.30,
		},
		{
			Name: "flowchart_subgraph",
			Diagram: `flowchart TD
  subgraph API
    A[Gateway] --> B[Auth]
  end
  subgraph Data
    C[(DB)] --> D[Cache]
  end
  B --> C`,
			MaxMismatch: 0.32,
		},
		{
			Name: "sequence_basic",
			Diagram: `sequenceDiagram
  participant Alice
  participant Bob
  Alice->>Bob: Hello
  Bob-->>Alice: Hi`,
			MaxMismatch: 0.30,
		},
		{
			Name: "class_basic",
			Diagram: `classDiagram
  class Animal {
    +int age
    +eat()
  }
  class Dog {
    +bark()
  }
  Animal <|-- Dog`,
			MaxMismatch: 0.32,
		},
		{
			Name: "state_basic",
			Diagram: `stateDiagram-v2
  [*] --> Idle
  Idle --> Running
  Running --> [*]`,
			MaxMismatch: 0.30,
		},
		{
			Name: "er_basic",
			Diagram: `erDiagram
  CUSTOMER ||--o{ ORDER : places
  ORDER ||--|{ LINE_ITEM : contains`,
			MaxMismatch: 0.32,
		},
		{
			Name: "pie_basic",
			Diagram: `pie showData
  title Pets
  "Dogs" : 10
  "Cats" : 5
  "Birds" : 2`,
			MaxMismatch: 0.30,
		},
		{
			Name: "mindmap_basic",
			Diagram: `mindmap
  root((Mindmap))
    Origins
      Long history
    Features
      Simplicity`,
			MaxMismatch: 0.32,
		},
		{
			Name: "journey_basic",
			Diagram: `journey
  title User Journey
  section Signup
    Visit site: 5: User
    Fill form: 3: User
  section Activation
    Verify email: 4: User`,
			MaxMismatch: 0.32,
		},
		{
			Name: "timeline_basic",
			Diagram: `timeline
  title Product Timeline
  2024 : alpha
  2025 : beta : ga`,
			MaxMismatch: 0.32,
		},
		{
			Name: "gantt_basic",
			Diagram: `gantt
  title Delivery Plan
  section Build
    Core Engine :done, core, 2026-01-01, 10d
    QA Cycle :active, qa, 2026-01-10, 6d`,
			MaxMismatch: 0.32,
		},
		{
			Name: "gitgraph_basic",
			Diagram: `gitGraph
  commit
  branch feature
  checkout feature
  commit
  checkout main
  merge feature`,
			MaxMismatch: 0.32,
		},
		{
			Name: "quadrant_basic",
			Diagram: `quadrantChart
  title Priorities
  x-axis Low --> High
  y-axis Low --> High
  Risk: [0.2, 0.9]
  Value: [0.8, 0.3]`,
			MaxMismatch: 0.32,
		},
		{
			Name: "xychart_basic",
			Diagram: `xychart-beta
  title Revenue
  x-axis [Q1, Q2, Q3]
  y-axis 0 --> 100
  bar [20, 50, 80]
  line [15, 45, 85]`,
			MaxMismatch: 0.32,
		},
	}
	const width = 1600
	const height = 1200

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			refImg, refPNG := renderWithMMDCPNG(t, fixture.Diagram, width, height)

			gotSVG, err := RenderWithOptions(
				fixture.Diagram,
				DefaultRenderOptions().WithAllowApproximate(true),
			)
			if err != nil {
				t.Fatalf("RenderWithOptions() error: %v", err)
			}

			gotImg, err := rasterizeSVG(gotSVG, width, height)
			if err != nil {
				savePartialArtifacts(t, fixture.Name, gotSVG, refPNG)
				t.Fatalf("rasterize rendered svg: %v", err)
			}

			mismatch, meanDelta, considered := compareDrawnPixels(gotImg, refImg)
			t.Logf(
				"fixture=%s mismatch=%.4f mean_delta=%.4f considered_pixels=%d",
				fixture.Name, mismatch, meanDelta, considered,
			)

			if mismatch > fixture.MaxMismatch {
				saveConformanceArtifacts(t, fixture.Name, gotSVG, refPNG, gotImg, refImg)
				t.Fatalf(
					"conformance mismatch %.4f exceeds threshold %.4f for %s",
					mismatch, fixture.MaxMismatch, fixture.Name,
				)
			}
		})
	}
}

func renderWithMMDCPNG(t *testing.T, diagram string, width, height int) (*image.NRGBA, []byte) {
	t.Helper()
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mmd")
	outputPath := filepath.Join(tmpDir, "ref.png")
	if err := os.WriteFile(inputPath, []byte(diagram), 0o644); err != nil {
		t.Fatalf("write mmd fixture: %v", err)
	}

	cmd := exec.Command(
		"mmdc",
		"-i", inputPath,
		"-o", outputPath,
		"-e", "png",
		"-w", fmt.Sprintf("%d", width),
		"-H", fmt.Sprintf("%d", height),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mmdc render failed: %v\n%s", err, string(out))
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read mmdc output: %v", err)
	}
	img, decodeErr := decodeImage(content)
	if decodeErr != nil {
		t.Fatalf("decode mmdc png output: %v", decodeErr)
	}
	return toNRGBA(img), content
}

func rasterizeSVG(svg string, width, height int) (*image.NRGBA, error) {
	intrinsicW, intrinsicH := detectSVGSize(svg)
	rendered, err := rasterizeSVGToImage(svg, intrinsicW, intrinsicH)
	if err != nil {
		return nil, fmt.Errorf("rasterize rendered svg: %w", err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(img, rendered.Bounds(), rendered, rendered.Bounds().Min, draw.Over)
	return img, nil
}

func toNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok {
		return nrgba
	}
	bounds := img.Bounds()
	converted := image.NewNRGBA(bounds)
	draw.Draw(converted, bounds, img, bounds.Min, draw.Src)
	return converted
}

func decodeImage(content []byte) (image.Image, error) {
	if pngImg, err := png.Decode(bytes.NewReader(content)); err == nil {
		return pngImg, nil
	}
	jpegImg, err := jpeg.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	return jpegImg, nil
}

func compareDrawnPixels(a, b *image.NRGBA) (mismatchRatio float64, meanDelta float64, considered int) {
	a, b = normalizeImageBounds(a, b)
	bounds := a.Bounds()

	mismatches := 0
	totalDelta := 0.0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ao := a.PixOffset(x, y)
			bo := b.PixOffset(x, y)

			ar, ag, ab := a.Pix[ao], a.Pix[ao+1], a.Pix[ao+2]
			br, bg, bb := b.Pix[bo], b.Pix[bo+1], b.Pix[bo+2]

			whiteA := isNearWhite(ar, ag, ab)
			whiteB := isNearWhite(br, bg, bb)
			if whiteA && whiteB {
				continue
			}
			considered++

			delta := absInt(int(ar)-int(br)) + absInt(int(ag)-int(bg)) + absInt(int(ab)-int(bb))
			totalDelta += float64(delta) / (3.0 * 255.0)
			if delta > 48 {
				mismatches++
			}
		}
	}

	if considered == 0 {
		return 0, 0, 0
	}
	return float64(mismatches) / float64(considered), totalDelta / float64(considered), considered
}

func saveConformanceArtifacts(
	t *testing.T,
	name string,
	gotSVG string,
	refPNG []byte,
	gotImg *image.NRGBA,
	refImg *image.NRGBA,
) {
	t.Helper()
	gotImg, refImg = normalizeImageBounds(gotImg, refImg)
	outDir := os.Getenv("MMDG_CONFORMANCE_OUT")
	if strings.TrimSpace(outDir) == "" {
		outDir = filepath.Join(os.TempDir(), "mmdg-conformance")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Logf("unable to create conformance out dir: %v", err)
		return
	}

	gotSVGPath := filepath.Join(outDir, fmt.Sprintf("%s-got.svg", name))
	refPNGPathRaw := filepath.Join(outDir, fmt.Sprintf("%s-ref-mmdc.png", name))
	gotPNGPath := filepath.Join(outDir, fmt.Sprintf("%s-got.png", name))
	refPNGPath := filepath.Join(outDir, fmt.Sprintf("%s-ref.png", name))
	diffPNGPath := filepath.Join(outDir, fmt.Sprintf("%s-diff.png", name))

	_ = os.WriteFile(gotSVGPath, []byte(gotSVG), 0o644)
	_ = os.WriteFile(refPNGPathRaw, refPNG, 0o644)

	if file, err := os.Create(gotPNGPath); err == nil {
		_ = png.Encode(file, gotImg)
		_ = file.Close()
	}
	if file, err := os.Create(refPNGPath); err == nil {
		_ = png.Encode(file, refImg)
		_ = file.Close()
	}

	diff := image.NewNRGBA(gotImg.Bounds())
	for y := diff.Bounds().Min.Y; y < diff.Bounds().Max.Y; y++ {
		for x := diff.Bounds().Min.X; x < diff.Bounds().Max.X; x++ {
			ao := gotImg.PixOffset(x, y)
			bo := refImg.PixOffset(x, y)
			ro := diff.PixOffset(x, y)
			diff.Pix[ro] = uint8(absInt(int(gotImg.Pix[ao]) - int(refImg.Pix[bo])))
			diff.Pix[ro+1] = uint8(absInt(int(gotImg.Pix[ao+1]) - int(refImg.Pix[bo+1])))
			diff.Pix[ro+2] = uint8(absInt(int(gotImg.Pix[ao+2]) - int(refImg.Pix[bo+2])))
			diff.Pix[ro+3] = 255
		}
	}
	if file, err := os.Create(diffPNGPath); err == nil {
		_ = png.Encode(file, diff)
		_ = file.Close()
	}

	t.Logf("saved conformance artifacts for %s to %s", name, outDir)
}

func savePartialArtifacts(t *testing.T, name string, gotSVG string, refPNG []byte) {
	t.Helper()
	outDir := os.Getenv("MMDG_CONFORMANCE_OUT")
	if strings.TrimSpace(outDir) == "" {
		outDir = filepath.Join(os.TempDir(), "mmdg-conformance")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(outDir, fmt.Sprintf("%s-got.svg", name)), []byte(gotSVG), 0o644)
	_ = os.WriteFile(filepath.Join(outDir, fmt.Sprintf("%s-ref-mmdc.png", name)), refPNG, 0o644)
}

func isNearWhite(r, g, b uint8) bool {
	return r > 245 && g > 245 && b > 245
}

func absInt(v int) int {
	return int(math.Abs(float64(v)))
}

func normalizeImageBounds(a, b *image.NRGBA) (*image.NRGBA, *image.NRGBA) {
	aw, ah := a.Bounds().Dx(), a.Bounds().Dy()
	bw, bh := b.Bounds().Dx(), b.Bounds().Dy()
	w := aw
	h := ah
	if bw > w {
		w = bw
	}
	if bh > h {
		h = bh
	}
	if aw == w && ah == h && bw == w && bh == h {
		return a, b
	}

	expand := func(src *image.NRGBA) *image.NRGBA {
		dst := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
		draw.Draw(dst, src.Bounds(), src, src.Bounds().Min, draw.Over)
		return dst
	}
	return expand(a), expand(b)
}
