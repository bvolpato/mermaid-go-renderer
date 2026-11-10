// Tests @mermaid-js/mermaid-zenuml 1.0.1, src/zenumlRenderer.ts (MIT),
// against the SVG that mermaid-cli 12.0.0 writes in Chromium on Linux and macOS.

package zenuml

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/mmdc"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func TestZenUMLGolden(t *testing.T) {
	var golden struct {
		Cases []struct{ SourcePath, Source, SVG string }
	}
	loadGolden(t, "testdata/golden.json.gz", &golden)
	for _, c := range golden.Cases {
		t.Run(c.SourcePath, func(t *testing.T) {
			assertGolden(t, c.Source, c.SVG, font.Embedded().WithWebFonts())
		})
	}
}

func loadGolden(t *testing.T, name string, target any) {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := json.NewDecoder(r).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func assertGolden(t *testing.T, source, expected string, fonts *font.Library) {
	t.Helper()
	out, err := render.Render(source, render.Options{Page: render.PageMermaidCLI, Fonts: fonts})
	if err != nil {
		t.Fatal(err)
	}
	page, err := mmdc.Load(out.SVG, mmdc.Options{Fonts: fonts, PageSheets: render.PageSheets(render.PageMermaidCLI), Background: "white", SetBackground: true})
	if err != nil {
		t.Fatal(err)
	}
	got := page.XML()
	if got != expected {
		i := 0
		for i < len(got) && i < len(expected) && got[i] == expected[i] {
			i++
		}
		t.Errorf("SVG differs at byte %d\n got: %s\nwant: %s", i, got[max(0, i-50):min(len(got), i+100)], expected[max(0, i-50):min(len(expected), i+100)])
	}
	if err := page.EmbedFonts(); err != nil {
		t.Fatal(err)
	}
	if page.XML() != expected {
		t.Error("font embedding changed the reference SVG")
	}
}

func TestZenUMLEdgeGolden(t *testing.T) {
	var cases []struct{ Name, Environment, Source, SVG string }
	loadGolden(t, "testdata/edgecases.json.gz", &cases)
	for _, c := range cases {
		t.Run(c.Name+"/"+c.Environment, func(t *testing.T) {
			fonts := font.Embedded()
			if c.Environment == "macos" {
				if !fonttest.IsMacOSReference() {
					t.Skip("requires the macOS reference fonts")
				}
				fonts = font.System()
			}
			assertGolden(t, c.Source, c.SVG, fonts.WithWebFonts())
		})
	}
}

func TestZenUMLPageAndSizing(t *testing.T) {
	source := "zenuml\nA->B: Hello"
	if _, err := render.Render(source, render.Options{Fonts: font.Embedded()}); !config.IsUnknownDiagram(err) {
		t.Fatalf("plain page: got %v, want an unknown diagram error", err)
	}
	for _, fixed := range []bool{false, true} {
		t.Run(js.ToString(fixed), func(t *testing.T) {
			cfg := js.NewObj[any]()
			if fixed {
				seq := js.NewObj[any]()
				seq.Set("useMaxWidth", false)
				cfg.Set("sequence", seq)
			}
			out, err := render.Render(source, render.Options{Page: render.PageMermaidCLI, Fonts: font.Embedded(), Config: cfg})
			if err != nil {
				t.Fatal(err)
			}
			if out.Type != "zenuml" || !strings.Contains(out.SVG, `class="message-line"`) {
				t.Fatalf("missing ZenUML output: %s", out.SVG)
			}
			page, err := mmdc.Load(out.SVG, mmdc.Options{Fonts: font.Embedded()})
			if err != nil {
				t.Fatal(err)
			}
			if fixed == strings.Contains(page.XML(), `width="100%"`) {
				t.Fatalf("wrong sizing: %s", page.XML())
			}
		})
	}
}

func TestZenUMLCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := renderToSVG(ctx, "A->B: Hello", font.Embedded()); err != context.Canceled {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err := renderToSVG(ctx, strings.Repeat("A->B: Hello\n", 100), font.Embedded()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("interrupted engine: got %v, want context.DeadlineExceeded", err)
	}
}

func TestZenUMLConcurrent(t *testing.T) {
	const source = "title Concurrent\nif (emoji == \"😀😀\") { A.Sync() { B->C: Hi } }"
	fonts := font.Embedded()
	want, err := renderToSVG(context.Background(), source, fonts)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			got, err := renderToSVG(context.Background(), source, fonts)
			if err != nil || got != want {
				t.Errorf("concurrent render differs: %v", err)
			}
		})
	}
	wg.Wait()
}

func TestZenUMLCanvasWhitespace(t *testing.T) {
	const prefix = "A->B: "
	withTabs, err := renderToSVG(context.Background(), prefix+strings.Repeat("i\t", 40)+"i", font.Embedded())
	if err != nil {
		t.Fatal(err)
	}
	withSpaces, err := renderToSVG(context.Background(), prefix+strings.Repeat("i ", 40)+"i", font.Embedded())
	if err != nil {
		t.Fatal(err)
	}
	if withTabs.viewBox != withSpaces.viewBox {
		t.Fatalf("tabs: %s, spaces: %s", withTabs.viewBox, withSpaces.viewBox)
	}
}
