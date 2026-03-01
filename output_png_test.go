package mermaid

import (
	"bytes"
	"image"
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
