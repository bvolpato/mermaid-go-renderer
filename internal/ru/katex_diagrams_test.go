// Tests label DOM from mermaid 12.2.0, rendering-util/createText.ts and
// diagrams/sequence/svgDraw.js (MIT), recorded by gen-math-diagrams.mjs.

package ru_test

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// TestMathDiagramLabelsAgainstMermaid compares the complete HTML label DOM.
// Geometry belongs to the browser package's MathML tests. This test omits
// foreignObject positions and sizes so that it runs with embedded fonts.
func TestMathDiagramLabelsAgainstMermaid(t *testing.T) {
	f, err := os.Open("testdata/math_diagrams.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var cases []struct{ Name, Text, SVG, Error string }
	if err := json.NewDecoder(z).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("the upstream golden file is empty")
	}
	for _, c := range cases {
		check := func(t *testing.T, text string, completeMirrors bool) {
			got, err := render.Render(text, render.Options{Fonts: font.Embedded()})
			if c.Error != "" {
				if err == nil || err.Error() != c.Error {
					t.Fatalf("render error: got %v, want %s", err, c.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			labels := func(svg string, duplicateActors bool) []string {
				doc, err := dom.ParseBody(svg)
				if err != nil {
					t.Fatal(err)
				}
				var out []string
				for _, fo := range doc.Body.SelectAll("foreignObject") {
					for _, key := range []string{"x", "y", "width", "height"} {
						fo.Attr(key, nil)
					}
					out = append(out, fo.OuterHTML())
					if duplicateActors && fo.Parent.Tag == "switch" {
						out = append(out, fo.OuterHTML())
					}
				}
				if len(out) == 0 {
					// With htmlLabels disabled, Mermaid keeps TeX as SVG text.
					for _, text := range doc.Body.SelectAll(".label text") {
						out = append(out, "svg-text:"+text.TextContent())
					}
				}
				return out
			}
			want, actual := labels(c.SVG, completeMirrors), labels(got.SVG, false)
			if completeMirrors {
				slices.Sort(want)
				slices.Sort(actual)
			}
			if !reflect.DeepEqual(actual, want) {
				t.Errorf("HTML label DOM:\ngot  %q\nwant %q", actual, want)
			}
		}
		t.Run(c.Name, func(t *testing.T) { check(t, c.Text, false) })
		if strings.Contains(c.Text, "mirrorActors: false") {
			// Mermaid starts actor math without awaiting it. Its footer labels
			// can be empty at serialization. The Go port finishes those labels.
			// Each footer must contain the same label as its recorded header.
			text := strings.Replace(c.Text, "mirrorActors: false", "mirrorActors: true", 1)
			t.Run(c.Name+"/complete-mirrors", func(t *testing.T) { check(t, text, true) })
		}
	}
}
