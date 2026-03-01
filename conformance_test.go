package mermaid

import (
	"bytes"
	"encoding/json"
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
	"sort"
	"strconv"
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

	fixtures := conformanceFixtures()
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

func TestPNGConformanceAgainstMMDC(t *testing.T) {
	if os.Getenv("MMDG_PNG_CONFORMANCE") != "1" {
		t.Skip("set MMDG_PNG_CONFORMANCE=1 to run mmdc PNG conformance checks")
	}
	if _, err := exec.LookPath("mmdc"); err != nil {
		t.Skip("mmdc not found in PATH")
	}

	fixtures := conformanceFixtures()
	const width = 1600
	const height = 1200
	updateBaseline := os.Getenv("MMDG_PNG_UPDATE_BASELINE") == "1"
	allowedRegression := 0.01
	if raw := strings.TrimSpace(os.Getenv("MMDG_PNG_ALLOWED_REGRESSION")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			t.Fatalf("invalid MMDG_PNG_ALLOWED_REGRESSION value %q: %v", raw, err)
		}
		allowedRegression = parsed
	}
	maxMismatch := -1.0
	if raw := strings.TrimSpace(os.Getenv("MMDG_PNG_MAX_MISMATCH")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			t.Fatalf("invalid MMDG_PNG_MAX_MISMATCH value %q: %v", raw, err)
		}
		maxMismatch = parsed
	}
	baselinePath := filepath.Join("testdata", "conformance", "png_mismatch_baseline.json")
	baselineMap := map[string]float64{}
	if !updateBaseline {
		baseline, err := loadPNGConformanceBaseline(baselinePath)
		if err != nil {
			t.Fatalf("load PNG baseline %s: %v", baselinePath, err)
		}
		if baseline.Width != width || baseline.Height != height {
			t.Fatalf(
				"baseline dimensions mismatch: baseline=%dx%d expected=%dx%d",
				baseline.Width, baseline.Height, width, height,
			)
		}
		for _, entry := range baseline.Entries {
			baselineMap[entry.Fixture] = entry.Mismatch
		}
	}
	computed := map[string]float64{}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			refImg, refPNG := renderWithMMDCPNG(t, fixture.Diagram, width, height)
			gotImg, gotPNG, gotSVG := renderWithMMDGPNG(t, fixture.Diagram, width, height)

			mismatch, meanDelta, considered := compareDrawnPixels(gotImg, refImg)
			computed[fixture.Name] = mismatch
			t.Logf(
				"fixture=%s png_mismatch=%.4f mean_delta=%.4f considered_pixels=%d",
				fixture.Name, mismatch, meanDelta, considered,
			)
			if maxMismatch >= 0 && mismatch > maxMismatch {
				saveConformanceArtifacts(t, fixture.Name+"-png", gotSVG, refPNG, gotImg, refImg)
				savePNGConformanceOutputs(t, fixture.Name, gotPNG, refPNG)
				t.Fatalf(
					"png mismatch %.4f exceeds max threshold %.4f for %s",
					mismatch, maxMismatch, fixture.Name,
				)
			}
			if updateBaseline {
				return
			}
			baselineMismatch, ok := baselineMap[fixture.Name]
			if !ok {
				t.Fatalf("missing PNG baseline entry for fixture %s", fixture.Name)
			}
			if mismatch > baselineMismatch+allowedRegression {
				saveConformanceArtifacts(t, fixture.Name+"-png", gotSVG, refPNG, gotImg, refImg)
				savePNGConformanceOutputs(t, fixture.Name, gotPNG, refPNG)
				t.Fatalf(
					"png mismatch regression %.4f > baseline %.4f + allowed %.4f for %s",
					mismatch, baselineMismatch, allowedRegression, fixture.Name,
				)
			}
		})
	}
	if updateBaseline {
		baseline := pngConformanceBaseline{
			Width:   width,
			Height:  height,
			Entries: make([]pngConformanceBaselineEntry, 0, len(computed)),
		}
		names := make([]string, 0, len(computed))
		for name := range computed {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			baseline.Entries = append(baseline.Entries, pngConformanceBaselineEntry{
				Fixture:  name,
				Mismatch: computed[name],
			})
		}
		if err := os.MkdirAll(filepath.Dir(baselinePath), 0o755); err != nil {
			t.Fatalf("create baseline directory: %v", err)
		}
		if err := writePNGConformanceBaseline(baselinePath, baseline); err != nil {
			t.Fatalf("write PNG baseline: %v", err)
		}
		t.Logf("updated PNG baseline at %s", baselinePath)
	}
}

func conformanceFixtures() []conformanceFixture {
	return []conformanceFixture{
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
}

type pngConformanceBaselineEntry struct {
	Fixture  string  `json:"fixture"`
	Mismatch float64 `json:"mismatch"`
}

type pngConformanceBaseline struct {
	Width   int                           `json:"width"`
	Height  int                           `json:"height"`
	Entries []pngConformanceBaselineEntry `json:"entries"`
}

func loadPNGConformanceBaseline(path string) (pngConformanceBaseline, error) {
	var baseline pngConformanceBaseline
	content, err := os.ReadFile(path)
	if err != nil {
		return baseline, err
	}
	if err := json.Unmarshal(content, &baseline); err != nil {
		return baseline, err
	}
	return baseline, nil
}

func writePNGConformanceBaseline(path string, baseline pngConformanceBaseline) error {
	content, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o644)
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

func renderWithMMDGPNG(t *testing.T, diagram string, width, height int) (*image.NRGBA, []byte, string) {
	t.Helper()
	svg, err := RenderWithOptions(
		diagram,
		DefaultRenderOptions().WithAllowApproximate(true),
	)
	if err != nil {
		t.Fatalf("RenderWithOptions() error: %v", err)
	}
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "got.png")
	_ = width
	_ = height
	if err := WriteOutputPNG(svg, outputPath); err != nil {
		t.Fatalf("WriteOutputPNG() error: %v", err)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read mmdg output: %v", err)
	}
	img, decodeErr := decodeImage(content)
	if decodeErr != nil {
		t.Fatalf("decode mmdg png output: %v", decodeErr)
	}
	return toNRGBA(img), content, svg
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
	bgAo := a.PixOffset(bounds.Min.X, bounds.Min.Y)
	bgBo := b.PixOffset(bounds.Min.X, bounds.Min.Y)
	bgAR, bgAG, bgAB := compositeOverWhite(a.Pix[bgAo], a.Pix[bgAo+1], a.Pix[bgAo+2], a.Pix[bgAo+3])
	bgBR, bgBG, bgBB := compositeOverWhite(b.Pix[bgBo], b.Pix[bgBo+1], b.Pix[bgBo+2], b.Pix[bgBo+3])

	mismatches := 0
	totalDelta := 0.0

	neighborhoodRadius := 4
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ao := a.PixOffset(x, y)
			bo := b.PixOffset(x, y)

			ar, ag, ab := compositeOverWhite(
				a.Pix[ao], a.Pix[ao+1], a.Pix[ao+2], a.Pix[ao+3],
			)
			br, bg, bb := compositeOverWhite(
				b.Pix[bo], b.Pix[bo+1], b.Pix[bo+2], b.Pix[bo+3],
			)

			bgA := isNearRGB(ar, ag, ab, bgAR, bgAG, bgAB, 14)
			bgB := isNearRGB(br, bg, bb, bgBR, bgBG, bgBB, 14)
			if bgA && bgB {
				continue
			}
			considered++

			delta := absInt(int(ar)-int(br)) + absInt(int(ag)-int(bg)) + absInt(int(ab)-int(bb))
			minDelta := delta
			if delta > 48 {
				for ny := max(bounds.Min.Y, y-neighborhoodRadius); ny <= min(bounds.Max.Y-1, y+neighborhoodRadius); ny++ {
					for nx := max(bounds.Min.X, x-neighborhoodRadius); nx <= min(bounds.Max.X-1, x+neighborhoodRadius); nx++ {
						if nx == x && ny == y {
							continue
						}
						nbo := b.PixOffset(nx, ny)
						nbr, nbg, nbb := compositeOverWhite(
							b.Pix[nbo], b.Pix[nbo+1], b.Pix[nbo+2], b.Pix[nbo+3],
						)
						nd := absInt(int(ar)-int(nbr)) + absInt(int(ag)-int(nbg)) + absInt(int(ab)-int(nbb))
						if nd < minDelta {
							minDelta = nd
						}
					}
				}
			}
			totalDelta += float64(minDelta) / (3.0 * 255.0)
			if minDelta > 48 {
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

func savePNGConformanceOutputs(t *testing.T, name string, gotPNG []byte, refPNG []byte) {
	t.Helper()
	outDir := os.Getenv("MMDG_CONFORMANCE_OUT")
	if strings.TrimSpace(outDir) == "" {
		outDir = filepath.Join(os.TempDir(), "mmdg-conformance")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(outDir, fmt.Sprintf("%s-got-direct-png.png", name)), gotPNG, 0o644)
	_ = os.WriteFile(filepath.Join(outDir, fmt.Sprintf("%s-ref-direct-png.png", name)), refPNG, 0o644)
}

func isNearWhite(r, g, b uint8) bool {
	return r > 245 && g > 245 && b > 245
}

func isNearRGB(r, g, b, rr, gg, bb uint8, tolerance int) bool {
	return absInt(int(r)-int(rr)) <= tolerance &&
		absInt(int(g)-int(gg)) <= tolerance &&
		absInt(int(b)-int(bb)) <= tolerance
}

func absInt(v int) int {
	return int(math.Abs(float64(v)))
}

func compositeOverWhite(r, g, b, a uint8) (uint8, uint8, uint8) {
	alpha := int(a)
	if alpha <= 0 {
		return 255, 255, 255
	}
	if alpha >= 255 {
		return r, g, b
	}
	blend := func(channel uint8) uint8 {
		c := int(channel)
		return uint8((c*alpha + 255*(255-alpha)) / 255)
	}
	return blend(r), blend(g), blend(b)
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
		bgOffset := src.PixOffset(src.Bounds().Min.X, src.Bounds().Min.Y)
		bg := color.NRGBA{
			R: src.Pix[bgOffset],
			G: src.Pix[bgOffset+1],
			B: src.Pix[bgOffset+2],
			A: src.Pix[bgOffset+3],
		}
		draw.Draw(dst, dst.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
		draw.Draw(dst, src.Bounds(), src, src.Bounds().Min, draw.Over)
		return dst
	}
	return expand(a), expand(b)
}
