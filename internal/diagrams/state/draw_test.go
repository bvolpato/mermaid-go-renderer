package state

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file testdata/draw.json.gz has the SVG that the real
// mermaid.render returns for state diagrams when the layouts are replaced
// by a stub: tools/reference/gen-state-draw.mjs writes it with the browser
// oracle. The stub draws one group for each node of the layout data, with
// the properties of the layout data as attributes. stubLayout is the same
// stub. So the test compares what the draw function gives to the layout,
// and what it does after the layout: the links, the title and the view box.

func init() {
	// This package does not link the layouts of the unified renderer, so
	// the stub stands for them in the tests of the package.
	ru.RegisterLayout("dagre", "", stubLayout)
	ru.RegisterLayout("elk", "elk.layered", stubLayout)
	ru.RegisterLayout("elk.stress", "elk.stress", stubLayout)
}

// The attribute value of a property of the layout data: none for a value
// that is falsy (text), undefined (number) or empty (list).
func textAttr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func numberAttr(v float64) any {
	if ru.IsUndef(v) {
		return nil
	}
	return js.Str(v)
}

func flagAttr(v bool) any {
	if !v {
		return nil
	}
	return "true"
}

func listAttr(v []string) any {
	if len(v) == 0 {
		return nil
	}
	return strings.Join(v, "|")
}

// stubLayout is the stub of tools/reference/state-draw-page.js.
func stubLayout(_ *render.Context, data *ru.LayoutData, svg *dom.Node, algorithm string) error {
	root := svg.
		Append("g").
		Attr("class", "root").
		Attr("data-type", textAttr(data.Type)).
		Attr("data-layout", textAttr(data.LayoutAlgorithm)).
		Attr("data-algorithm", textAttr(algorithm)).
		Attr("data-direction", textAttr(data.Direction)).
		Attr("data-node-spacing", numberAttr(data.NodeSpacing)).
		Attr("data-rank-spacing", numberAttr(data.RankSpacing)).
		Attr("data-markers", listAttr(data.Markers)).
		Attr("data-diagram-id", textAttr(data.DiagramID))
	nodes := root.Append("g").Attr("class", "nodes")
	for i, node := range data.Nodes {
		class := "node"
		if node.IsGroup {
			class = "cluster"
		} else if node.Look == "handDrawn" {
			class = "rough-node"
		}
		nodeType, _ := node.Get("type").(string)
		g := nodes.
			Append("g").
			Attr("class", class).
			Attr("id", node.DomID).
			Attr("data-id", node.ID).
			Attr("data-shape", textAttr(node.Shape)).
			Attr("data-label-type", textAttr(node.LabelType)).
			Attr("data-classes", textAttr(node.CSSClasses)).
			Attr("data-styles", listAttr(node.CSSStyles)).
			Attr("data-compiled", listAttr(node.CSSCompiledStyles)).
			Attr("data-parent", textAttr(node.ParentID)).
			Attr("data-dir", textAttr(node.Dir)).
			Attr("data-color-index", numberAttr(node.ColorIndex)).
			Attr("data-padding", numberAttr(node.Padding)).
			Attr("data-rx", numberAttr(node.Rx)).
			Attr("data-ry", numberAttr(node.Ry)).
			Attr("data-look", textAttr(node.Look)).
			Attr("data-position", textAttr(node.Position)).
			Attr("data-description", listAttr(node.Description)).
			Attr("data-wrapping-width", numberAttr(node.WrappingWidth)).
			Attr("data-min-width", numberAttr(node.MinWidth)).
			Attr("data-center-label", flagAttr(node.CenterLabel)).
			Attr("data-node-type", textAttr(nodeType))
		g.Append("rect").
			Attr("x", float64(i*100)).
			Attr("y", 0.0).
			Attr("width", 80.0).
			Attr("height", 40.0)
		g.Append("desc").Text(node.Label)
	}
	edges := root.Append("g").Attr("class", "edges")
	for _, edge := range data.Edges {
		style := ""
		if len(edge.Style) > 0 {
			style = edge.Style[0]
		}
		edges.
			Append("path").
			Attr("id", edge.ID).
			Attr("d", "M0,0L10,10").
			Attr("data-start", textAttr(edge.Start)).
			Attr("data-end", textAttr(edge.End)).
			Attr("data-label", textAttr(edge.Label)).
			Attr("data-classes", textAttr(edge.Classes)).
			Attr("data-pattern", textAttr(edge.Pattern)).
			Attr("data-arrowhead", textAttr(edge.Arrowhead)).
			Attr("data-arrow-type-end", textAttr(edge.ArrowTypeEnd)).
			Attr("data-arrowhead-style", textAttr(edge.ArrowheadStyle)).
			Attr("data-look", textAttr(edge.Look)).
			Attr("data-thickness", textAttr(edge.Thickness)).
			Attr("data-style", textAttr(style)).
			Attr("data-label-type", textAttr(edge.LabelType)).
			Attr("data-labelpos", textAttr(edge.Labelpos))
	}
	return nil
}

func loadDrawGolden(tb testing.TB) []any {
	tb.Helper()
	f, err := os.Open(filepath.Join("testdata", "draw.json.gz"))
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		tb.Fatal(err)
	}
	v, err := js.ParseJSON(data)
	if err != nil {
		tb.Fatal(err)
	}
	cases, _ := js.AsObj(v).Get("cases").([]any)
	return cases
}

// firstDifference shows the place where two texts start to differ.
func firstDifference(got, want string) string {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	from := max(i-80, 0)
	cut := func(s string) string {
		return s[from:min(i+160, len(s))]
	}
	return "at byte " + js.Str(float64(i)) + "\n  got:  ..." + cut(got) + "\n  want: ..." + cut(want)
}

// TestDrawGolden compares the SVG of render.Render with the SVG of real
// mermaid, with the stub layout on both sides.
func TestDrawGolden(t *testing.T) {
	cases := loadDrawGolden(t)
	failed := 0
	for _, item := range cases {
		c := js.AsObj(item)
		name, _ := c.Get("name").(string)
		text, _ := c.Get("text").(string)
		want, _ := c.Get("svg").(string)
		// The page of the oracle calls mermaid.initialize({ startOnLoad: false, ...site }).
		site := js.NewObj[any]()
		site.Set("startOnLoad", false)
		if extra := js.AsObj(c.Get("site")); extra != nil {
			extra.Each(func(k string, v any) { site.Set(k, v) })
		}
		res, err := render.Render(text, render.Options{Config: site})
		if err != nil {
			failed++
			t.Errorf("%s: error %q\n%s", name, errorMessage(err), indent(text))
			continue
		}
		if res.SVG != want {
			failed++
			t.Errorf("%s: the SVG differs %s\n%s", name, firstDifference(res.SVG, want), indent(text))
		}
	}
	t.Logf("%d cases: %d agree, %d differ", len(cases), len(cases)-failed, failed)
}
