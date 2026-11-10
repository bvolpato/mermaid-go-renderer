// Tests the label output of mermaid 12.2.0, diagrams/common/common.ts and
// rendering-util/createText.ts (MIT), recorded by gen-math-labels.mjs.

package ru

import (
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func TestMathLabelsAgainstMermaid(t *testing.T) {
	f, err := os.Open("testdata/math_labels.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var cases []struct {
		Name, Text, Markup, Label, Error, LabelError string
		Config                                       json.RawMessage
	}
	if err := json.NewDecoder(z).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("the upstream golden file is empty")
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			cfg, err := js.ParseJSON(c.Config)
			if err != nil {
				t.Fatal(err)
			}
			ctx := render.NewContext(render.ContextOptions{Config: js.AsObj(cfg)})
			got, err := RenderKatexSanitized(ctx, c.Text)
			if c.Error != "" {
				if err == nil || err.Error() != c.Error {
					t.Fatalf("error: got %v, want %s", err, c.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.Markup {
				t.Errorf("renderKatexSanitized:\ngot  %s\nwant %s", got, c.Markup)
			}
			g := ctx.SVG.Append("g")
			o := DefaultTextOptions()
			o.Markdown, o.Width = false, math.Inf(1)
			_, err = CreateText(ctx, g, c.Text, o)
			if c.LabelError != "" {
				if err == nil || err.Error() != c.LabelError {
					t.Fatalf("createText error: got %v, want %s", err, c.LabelError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := g.InnerHTML(); got != c.Label {
				t.Errorf("createText:\ngot  %s\nwant %s", got, c.Label)
			}
		})
	}
}
