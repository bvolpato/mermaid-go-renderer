package mermaid

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const minimalSVG = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="120" height="80" viewBox="0 0 120 80">
  <rect x="0" y="0" width="120" height="80" fill="#ffffff"/>
  <text x="60" y="40" text-anchor="middle">PNG</text>
</svg>`

func TestWriteOutputPNGToFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "out.png")
	if err := WriteOutputPNG(minimalSVG, path); err != nil {
		t.Fatalf("WriteOutputPNG() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode png file: %v", err)
	}
	if img.Bounds().Dx() != 120 || img.Bounds().Dy() != 80 {
		t.Fatalf("unexpected png dimensions: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestWriteOutputPNGStdout(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	writeErr := WriteOutputPNG(minimalSVG, "")
	_ = w.Close()
	content, _ := io.ReadAll(r)
	_ = r.Close()

	if writeErr != nil {
		t.Fatalf("WriteOutputPNG() error = %v", writeErr)
	}
	if len(content) == 0 {
		t.Fatalf("expected PNG bytes on stdout")
	}
	if _, err := png.Decode(bytes.NewReader(content)); err != nil {
		t.Fatalf("decode stdout png: %v", err)
	}
}

func TestPrepareSVGForRasterizerMaterializesAncestorCSS(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20" viewBox="0 0 40 20">
  <style>
    .section rect { fill: #ECECFF; stroke: #111; }
    .face { fill: #FFF8DC; stroke: #999; }
  </style>
  <g class="section">
    <rect x="0" y="0" width="20" height="20" fill="#191970"/>
  </g>
  <circle class="face" cx="30" cy="10" r="5"/>
</svg>`

	prepared := strings.ToLower(prepareSVGForRasterizer(svg))
	if !strings.Contains(prepared, `fill="#ececff"`) {
		t.Fatalf("expected computed rect fill to be materialized, got: %s", prepared)
	}
	if !strings.Contains(prepared, `stroke="#111"`) {
		t.Fatalf("expected computed rect stroke to be materialized, got: %s", prepared)
	}
	if !strings.Contains(prepared, `fill="#fff8dc"`) {
		t.Fatalf("expected computed circle fill to be materialized, got: %s", prepared)
	}
}

func TestPrepareSVGForRasterizerMaterializesChildSelectorCSS(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="80" height="20" viewBox="0 0 80 20">
  <style>
    text.actor>tspan { fill: #000000; stroke: none; }
  </style>
  <text class="actor" x="40" y="10"><tspan x="40" dy="0">Alice</tspan></text>
</svg>`

	prepared := strings.ToLower(prepareSVGForRasterizer(svg))
	if !strings.Contains(prepared, `<tspan`) || !strings.Contains(prepared, `fill="#000000"`) {
		t.Fatalf("expected child-selector fill to be materialized onto tspan, got: %s", prepared)
	}
	if !strings.Contains(prepared, `stroke="none"`) {
		t.Fatalf("expected child-selector stroke to be materialized onto tspan, got: %s", prepared)
	}
}

func TestPrepareSVGForRasterizerInlinesERMarkers(t *testing.T) {
	diagram := strings.TrimSpace(`erDiagram
  CUSTOMER {
    string name
    string custNumber
  }
  ORDER {
    int orderNumber
  }
  CUSTOMER ||--o{ ORDER : places`)

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	prepared := prepareSVGForRasterizer(svg)
	if strings.Contains(prepared, "marker-start=") || strings.Contains(prepared, "marker-end=") {
		t.Fatalf("expected ER marker references to be inlined, got: %s", prepared)
	}
	if !strings.Contains(prepared, `<circle`) {
		t.Fatalf("expected ER zero-or-more marker circle to be materialized, got: %s", prepared)
	}
	if !strings.Contains(prepared, `rotate(`) {
		t.Fatalf("expected ER marker paths to include placement transforms, got: %s", prepared)
	}
}

func TestWriteOutputPNGAppliesClassCSSOverrides(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20" viewBox="0 0 40 20">
  <style>
    .section rect { fill: #ECECFF; stroke: #111; }
  </style>
  <g class="section">
    <rect x="0" y="0" width="40" height="20" fill="#191970"/>
  </g>
</svg>`

	img, err := rasterizeSVGToImage(svg, 40, 20)
	if err != nil {
		t.Fatalf("rasterizeSVGToImage() error = %v", err)
	}
	r, g, b, _ := rgba8At(img, 20, 10)
	if !nearRGB(r, g, b, 236, 236, 255, 8) {
		t.Fatalf("expected CSS fill override to render pastel, got rgb(%d,%d,%d)", r, g, b)
	}
}

func TestPrepareSVGForRasterizerKeepsMindmapViewBox(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" class="mindmapDiagram" viewBox="5 5 147.26 454.92" aria-roledescription="mindmap">
  <g transform="translate(83.59, 232.34)">
    <g class="label" transform="translate(-57, -12)"></g>
  </g>
</svg>`

	prepared := prepareSVGForRasterizer(svg)
	if !strings.Contains(prepared, `viewBox="5 5 147.26 454.92"`) {
		t.Fatalf("expected mindmap viewBox to remain unchanged, got: %s", prepared)
	}
}

func TestPrepareSVGForRasterizerKeepsKanbanViewBox(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="90 -310 630 99" aria-roledescription="kanban">
  <g transform="translate(200, -253)">
    <g class="label" transform="translate(-82.5, -12)"></g>
    <g class="label" transform="translate(-82.5, 12)"></g>
    <g class="label" transform="translate(82.5, 12)"></g>
  </g>
</svg>`

	prepared := prepareSVGForRasterizer(svg)
	if !strings.Contains(prepared, `viewBox="90 -310 630 99"`) {
		t.Fatalf("expected kanban viewBox to remain unchanged, got: %s", prepared)
	}
}

func TestPrepareSVGForRasterizerKeepsGitGraphViewBox(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="-110.91 -18 268.91 171.37" aria-roledescription="gitGraph">
  <g transform="translate(-29.42, 26.21) rotate(-45, 0, 0)">
    <rect x="-15.84" y="13.5" width="51.69" height="15"></rect>
    <text x="-13.84" y="25">0-c2d6a32</text>
  </g>
</svg>`

	prepared := prepareSVGForRasterizer(svg)
	if !strings.Contains(prepared, `viewBox="-110.91 -18 268.91 171.37"`) {
		t.Fatalf("expected gitGraph viewBox to remain unchanged, got: %s", prepared)
	}
}

func TestRasterizeSVGToImageDrawsTSpanPositionedText(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40" viewBox="0 0 120 40">
  <rect x="0" y="0" width="120" height="40" fill="#ffffff"/>
  <g transform="translate(10, 4)">
    <text fill="#000000"><tspan x="0" dy="1em">main</tspan></text>
  </g>
</svg>`

	prepared := prepareSVGForRasterizer(svg)
	if len(svgTextElementPattern.FindAllStringSubmatchIndex(prepared, -1)) != 1 {
		t.Fatalf("expected prepared SVG to keep one text node, got: %s", prepared)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 120, 40))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	overlaySVGText(img, prepared, 120, 40, svgViewBox{X: 0, Y: 0, W: 120, H: 40}, true)
	if countDarkPixels(img, 0, 0, 120, 40) == 0 {
		t.Fatalf("expected tspan-positioned text to render near its translated origin, prepared=%s", prepared)
	}
}

func TestOverlaySVGTextPrefersTSpanFillOverParentTextFill(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40" viewBox="0 0 120 40">
  <rect x="0" y="0" width="120" height="40" fill="#ECECFF"/>
  <text x="60" y="20" fill="#ECECFF" dominant-baseline="central" alignment-baseline="central" style="text-anchor: middle; font-size: 16px;">
    <tspan x="60" dy="0" fill="#000000">Alice</tspan>
  </text>
</svg>`

	prepared := prepareSVGForRasterizer(svg)
	img := image.NewNRGBA(image.Rect(0, 0, 120, 40))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.NRGBA{R: 0xEC, G: 0xEC, B: 0xFF, A: 0xFF}}, image.Point{}, draw.Src)
	overlaySVGText(img, prepared, 120, 40, svgViewBox{X: 0, Y: 0, W: 120, H: 40}, true)
	if countDarkPixels(img, 20, 8, 100, 32) == 0 {
		t.Fatalf("expected tspan fill to override parent text fill, prepared=%s", prepared)
	}
}

func TestStripSVGTextElementsForGanttRasterization(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" aria-roledescription="gantt">
  <text x="10" y="20">Title</text>
</svg>`

	if !shouldStripSVGTextForRasterizer(svg) {
		t.Fatalf("expected gantt SVG text to be stripped before native rasterization")
	}
	stripped := stripSVGTextElements(svg)
	if strings.Contains(stripped, "<text") {
		t.Fatalf("expected text nodes to be removed, got: %s", stripped)
	}
}

func TestRasterizeSVGToImagePreservesAspectRatio(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="100" height="20" viewBox="0 0 100 20">
  <rect x="0" y="0" width="100" height="20" fill="#000000"/>
</svg>`

	img, err := rasterizeSVGToImage(svg, 100, 100)
	if err != nil {
		t.Fatalf("rasterizeSVGToImage() error = %v", err)
	}

	topR, topG, topB, _ := rgba8At(img, 50, 5)
	if !nearRGB(topR, topG, topB, 255, 255, 255, 5) {
		t.Fatalf("expected top padding to remain white, got rgb(%d,%d,%d)", topR, topG, topB)
	}

	midR, midG, midB, _ := rgba8At(img, 50, 50)
	if !nearRGB(midR, midG, midB, 0, 0, 0, 5) {
		t.Fatalf("expected centered content to remain black, got rgb(%d,%d,%d)", midR, midG, midB)
	}
}

func TestExtractForeignObjectLabelsUsesInnermostStyle(t *testing.T) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40" viewBox="0 0 120 40">
  <g transform="translate(10, 8)">
    <foreignObject width="80" height="24">
      <div xmlns="http://www.w3.org/1999/xhtml" style="text-align: center; color: #111;">
        <span class="nodeLabel" style="text-align: right !important; color: #222;">Task</span>
      </div>
    </foreignObject>
  </g>
</svg>`

	labels := extractForeignObjectLabels(svg, svgViewBox{X: 0, Y: 0, W: 120, H: 40})
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if labels[0].TextAlign != "right" {
		t.Fatalf("expected innermost text-align override, got %q", labels[0].TextAlign)
	}
	if labels[0].Color != "#222" {
		t.Fatalf("expected innermost color override, got %q", labels[0].Color)
	}
}

func TestRenderKanbanIncludesExplicitSectionFill(t *testing.T) {
	diagram := strings.TrimSpace(`kanban
  Todo
    id1[Task A]
  In Progress
    id2[Task B]
  Done
    id3[Task C]`)

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}
	prepared := prepareSVGForRasterizer(svg)
	if !strings.Contains(prepared, `fill="#e8ffb9"`) {
		t.Fatalf("expected kanban section fill to be materialized, got: %s", prepared)
	}
}

func TestWriteOutputPNGFromClassDiagram(t *testing.T) {
	diagram := strings.TrimSpace(`classDiagram
  class Animal {
    +int age
    +eat()
  }
  class Dog {
    +bark()
  }
  Animal <|-- Dog`)

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "class.png")
	if err := WriteOutputPNG(svg, path); err != nil {
		t.Fatalf("WriteOutputPNG() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(content)); err != nil {
		t.Fatalf("decode class png: %v", err)
	}
}

func rgba8At(img image.Image, x, y int) (uint8, uint8, uint8, uint8) {
	r, g, b, a := img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)
}

func countDarkPixels(img image.Image, minX, minY, maxX, maxY int) int {
	count := 0
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			r, g, b, _ := rgba8At(img, x, y)
			if r < 200 || g < 200 || b < 200 {
				count++
			}
		}
	}
	return count
}

func nearRGB(r, g, b, rr, gg, bb uint8, tolerance int) bool {
	diff := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	return diff(r, rr) <= tolerance && diff(g, gg) <= tolerance && diff(b, bb) <= tolerance
}

func TestWriteOutputPNGFromGanttDiagram(t *testing.T) {
	diagram := strings.TrimSpace(`gantt
  title Delivery Plan
  section Build
    Core Engine :done, core, 2026-01-01, 10d
    QA Cycle :active, qa, 2026-01-10, 6d`)

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "gantt.png")
	if err := WriteOutputPNG(svg, path); err != nil {
		t.Fatalf("WriteOutputPNG() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(content)); err != nil {
		t.Fatalf("decode gantt png: %v", err)
	}
}

func TestWriteOutputPNGFromViewportAwareGanttDiagram(t *testing.T) {
	diagram := strings.TrimSpace(`gantt
  title Delivery Plan
  dateFormat YYYY-MM-DD
  section Build
    Core Engine :done, core, 2026-01-01, 5d
    QA Cycle :active, qa, 2026-01-05, 3d`)

	svg, err := RenderWithOptions(
		diagram,
		DefaultRenderOptions().WithAllowApproximate(true).WithViewportSize(1600, 1200),
	)
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "gantt-wide.png")
	if err := WriteOutputPNG(svg, path); err != nil {
		t.Fatalf("WriteOutputPNG() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode gantt png: %v", err)
	}
	if img.Bounds().Dx() != 1584 || img.Bounds().Dy() != 148 {
		t.Fatalf("unexpected viewport-aware gantt png dimensions: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestWriteOutputPNGFromZenUMLDiagram(t *testing.T) {
	diagram := strings.TrimSpace(`zenuml
  title Checkout Flow
  @Actor Customer
  @Boundary Gateway
  Customer->Gateway: submit()`)

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "zenuml.png")
	if err := WriteOutputPNG(svg, path); err != nil {
		t.Fatalf("WriteOutputPNG() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode zenuml png: %v", err)
	}
	if countNonWhitePixels(img) == 0 {
		t.Fatalf("expected non-empty zenuml png output")
	}
}

func TestWriteOutputPNGFromSequenceDiagramPreservesViewBoxHeight(t *testing.T) {
	diagram := strings.TrimSpace(`sequenceDiagram
  participant User
  participant App
  participant API
  participant DB
  User->>App: Open dashboard
  App->>API: GET /stats
  API->>DB: Query metrics
  DB-->>API: rows
  API-->>App: JSON
  App-->>User: Render charts`)

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "sequence.png")
	if err := WriteOutputPNG(svg, path); err != nil {
		t.Fatalf("WriteOutputPNG() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode sequence png: %v", err)
	}
	if img.Bounds().Dx() < 800 || img.Bounds().Dy() < 400 {
		t.Fatalf("unexpected sequence png dimensions: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestWriteOutputPNGFromSequenceDiagramDrawsActorLabels(t *testing.T) {
	diagram := strings.TrimSpace(`sequenceDiagram
  participant Alice
  participant Bob
  Alice->>Bob: Hello
  Bob-->>Alice: Hi`)

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "sequence-labels.png")
	if err := WriteOutputPNG(svg, path); err != nil {
		t.Fatalf("WriteOutputPNG() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode sequence png: %v", err)
	}

	regions := [][4]int{
		{80, 20, 170, 56},
		{280, 20, 370, 56},
		{80, 194, 170, 230},
		{280, 194, 370, 230},
	}
	for _, region := range regions {
		if countDarkPixels(img, region[0], region[1], region[2], region[3]) == 0 {
			t.Fatalf("expected actor label pixels in region %v, image size=%dx%d", region, img.Bounds().Dx(), img.Bounds().Dy())
		}
	}
}

func TestWriteOutputPNGFromFlowchartWithSubgraphs(t *testing.T) {
	diagram := strings.TrimSpace(`flowchart TD
    subgraph G1["Group A"]
        A["Node 1"] ~~~ B["Node 2"] ~~~ C["Node 3"]
    end
    G1 -->|"connect"| G2
    subgraph G2["Group B"]
        D["Step 1"] --> E["Step 2"]
        E --> F["Step 3"]
    end
    G2 -->|"output"| G3
    subgraph G3["Group C"]
        X["Result 1"]
        Y["Result 2"]
    end`)

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions())
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "subgraph.png")
	if err := WriteOutputPNG(svg, path); err != nil {
		t.Fatalf("WriteOutputPNG() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode subgraph png: %v", err)
	}
	if img.Bounds().Dx() < 400 || img.Bounds().Dy() < 200 {
		t.Fatalf("unexpected subgraph png dimensions: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
	if countNonWhitePixels(img) == 0 {
		t.Fatalf("expected non-empty subgraph png output")
	}
}

func TestWritePNGFromSource(t *testing.T) {
	diagram := `flowchart LR
    A[Input] --> B[Process] --> C[Output]`

	tmp := t.TempDir()
	path := filepath.Join(tmp, "source.png")
	if err := WritePNGFromSource(diagram, path); err != nil {
		t.Fatalf("WritePNGFromSource() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode source png: %v", err)
	}
	if img.Bounds().Dx() < 100 || img.Bounds().Dy() < 30 {
		t.Fatalf("unexpected source png dimensions: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
	if countNonWhitePixels(img) == 0 {
		t.Fatalf("expected non-empty source png output")
	}
}

func TestWritePNGFromSourceWithSubgraphs(t *testing.T) {
	diagram := `flowchart TD
    subgraph G1["Group A"]
        A["Node 1"] ~~~ B["Node 2"]
    end
    G1 --> G2
    subgraph G2["Group B"]
        C["Step 1"] --> D["Step 2"]
    end`

	tmp := t.TempDir()
	path := filepath.Join(tmp, "subgraph.png")
	if err := WritePNGFromSource(diagram, path); err != nil {
		t.Fatalf("WritePNGFromSource() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode subgraph source png: %v", err)
	}
	if img.Bounds().Dx() < 200 || img.Bounds().Dy() < 100 {
		t.Fatalf("unexpected subgraph source png dimensions: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
	if countNonWhitePixels(img) == 0 {
		t.Fatalf("expected non-empty subgraph source png output")
	}
}

func TestWriteOutputPNGFromTimelineDiagramPreservesSectionColors(t *testing.T) {
	diagram := strings.TrimSpace(`timeline
    title Product Timeline
    2024 : alpha
    2025 : beta
         : ga`)

	svg, err := RenderWithOptions(diagram, DefaultRenderOptions().WithAllowApproximate(true))
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "timeline.png")
	if err := WriteOutputPNG(svg, path); err != nil {
		t.Fatalf("WriteOutputPNG() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read png file: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode timeline png: %v", err)
	}

	year2024R, year2024G, year2024B, _ := rgba8At(img, 292, 138)
	if !nearRGB(year2024R, year2024G, year2024B, 134, 134, 255, 18) {
		t.Fatalf("expected first section to stay purple, got rgb(%d,%d,%d)", year2024R, year2024G, year2024B)
	}

	year2025R, year2025G, year2025B, _ := rgba8At(img, 492, 138)
	if !nearRGB(year2025R, year2025G, year2025B, 255, 255, 120, 18) {
		t.Fatalf("expected second section to stay yellow, got rgb(%d,%d,%d)", year2025R, year2025G, year2025B)
	}

	axisR, axisG, axisB, _ := rgba8At(img, 490, 227)
	if !nearRGB(axisR, axisG, axisB, 0, 0, 0, 20) {
		t.Fatalf("expected timeline axis to stay dark, got rgb(%d,%d,%d)", axisR, axisG, axisB)
	}
}

func countNonWhitePixels(img image.Image) int {
	bounds := img.Bounds()
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			if !(r8 > 245 && g8 > 245 && b8 > 245) {
				count++
			}
		}
	}
	return count
}
