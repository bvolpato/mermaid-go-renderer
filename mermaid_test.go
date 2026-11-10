package mermaid

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderInfo(t *testing.T) {
	svg, err := Render("info")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(svg, `<svg id="my-svg"`) || !strings.Contains(svg, "v"+MermaidVersion) {
		t.Fatalf("unexpected output: %.200s", svg)
	}
}

func TestOptions(t *testing.T) {
	svg, err := Render("info", WithID("diagram-1"), WithTheme("dark"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(svg, `<svg id="diagram-1"`) || !strings.Contains(svg, "#diagram-1{") {
		t.Fatalf("the id is not applied: %.200s", svg)
	}
	if _, err := Render("info", WithConfigJSON([]byte(`[1]`))); err == nil {
		t.Fatal("a config that is not an object must be an error")
	}
}

func TestDetectType(t *testing.T) {
	typ, err := DetectType("sequenceDiagram\n  A->>B: hi")
	if err != nil || typ != "sequence" {
		t.Fatalf("DetectType = %q, %v", typ, err)
	}
}

func TestZenUMLPages(t *testing.T) {
	source := "zenuml\n  A->B: hi"
	if typ, err := DetectType(source); err == nil || typ != "" {
		t.Fatalf("plain-page detection = %q, %v, want an error", typ, err)
	}
	if svg, err := Render(source); err == nil || svg != "" {
		t.Fatalf("plain-page render returned %d bytes, %v, want an error", len(svg), err)
	}
	opts := []Option{WithPage(PageMermaidCLI)}
	if typ, err := DetectType(source, opts...); err != nil || typ != "zenuml" {
		t.Fatalf("CLI-page detection = %q, %v, want zenuml", typ, err)
	}
	result, err := RenderResult(source, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "zenuml" || !strings.Contains(result.SVG, `class="message-line"`) || !strings.Contains(result.SVG, ">hi</text>") {
		t.Fatalf("CLI page did not render the ZenUML message: %s", result.SVG)
	}
	file, err := StandaloneSVG(result.SVG, append(opts, WithBackgroundColor("white"))...)
	if err != nil || !strings.Contains(file, `aria-roledescription="zenuml"`) {
		t.Fatalf("CLI SVG serialization: %v\n%s", err, file)
	}
}

// TestRenderContextStops renders with a context that is already done, and
// with a deadline that ends during the render. Both must return the error
// of the context.
// TestDeepSVGIsAnError checks a diagram whose SVG nests deeper than the HTML
// parser of this module reads. The render returned an empty string and no
// error, which a caller takes for a diagram.
func TestDeepSVGIsAnError(t *testing.T) {
	src := "railroad-beta\na = " + strings.Repeat("optional(", 700) + `terminal("x")` + strings.Repeat(")", 700) + " ;\n"
	svg, err := Render(src)
	if err == nil {
		t.Fatalf("no error, and an SVG of %d bytes", len(svg))
	}
	if svg != "" {
		t.Errorf("an error and an SVG of %d bytes", len(svg))
	}
}

func TestRenderContextStops(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "corpus", "mermaid-e2e", "sequence", "nightly-check", "should-render-a-simple-sequence-diagram.mmd"))
	if err != nil {
		matches, _ := filepath.Glob(filepath.Join("testdata", "corpus", "mermaid-e2e", "sequence", "*", "*.mmd"))
		if len(matches) == 0 {
			t.Skip("no sequence fixture")
		}
		if src, err = os.ReadFile(matches[0]); err != nil {
			t.Fatal(err)
		}
	}
	done, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RenderContext(done, string(src)); !errors.Is(err, context.Canceled) {
		t.Fatalf("a canceled context gave %v", err)
	}
	// A deadline in the past of the first measurement.
	short, cancel2 := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel2()
	time.Sleep(time.Millisecond)
	if _, err := RenderContext(short, string(src)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("an expired deadline gave %v", err)
	}
	// A context that stays open changes nothing.
	open, cancel3 := context.WithCancel(context.Background())
	defer cancel3()
	want, err := Render(string(src))
	if err != nil {
		t.Fatal(err)
	}
	got, err := RenderContext(open, string(src))
	if err != nil || got.SVG != want {
		t.Fatalf("a live context changed the result: %v", err)
	}
}

// TestRenderPNG checks the size and the content of PNG output: the scale
// doubles the bitmap, the diagram leaves ink on the background, and a
// transparent background stays transparent.
func TestRenderPNG(t *testing.T) {
	src := "pie title Pets\n  \"Dogs\" : 386\n  \"Cats\" : 85\n"
	decode := func(opts ...Option) (w, h, inked, clear int) {
		data, err := RenderPNG(src, opts...)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, a := img.At(x, y).RGBA()
				switch {
				case a == 0:
					clear++
				case r != 0xffff || g != 0xffff || bl != 0xffff:
					inked++
				}
			}
		}
		return b.Dx(), b.Dy(), inked, clear
	}
	w1, h1, inked, clear := decode()
	if w1 < 100 || h1 < 100 || inked == 0 || clear != 0 {
		t.Fatalf("default: %dx%d, %d inked, %d transparent", w1, h1, inked, clear)
	}
	w2, h2, _, _ := decode(WithScale(2))
	if w2 != 2*w1 || h2 != 2*h1 {
		t.Fatalf("scale 2 gave %dx%d from %dx%d", w2, h2, w1, h1)
	}
	if _, _, _, clear := decode(WithBackground(color.Transparent)); clear == 0 {
		t.Fatal("a transparent background left no transparent pixel")
	}
}
