package sankey

import (
	"fmt"
	"math"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden file comes from d3-sankey 0.12.3 in Node.js
// (tools/reference/gen-d3-sankey.mjs). Every number is compared by bits and
// every path byte for byte.

type goldenResult struct {
	NI    []int     `json:"ni"` // index, depth, height, layer of each node
	NF    d3test.FS `json:"nf"` // value, x0, x1, y0, y1 of each node
	SL    [][]int   `json:"sl"` // the link indices of sourceLinks of each node
	TL    [][]int   `json:"tl"`
	LF    d3test.FS `json:"lf"` // width, y0, y1 of each link
	Paths []string  `json:"paths"`
}

type goldenCase struct {
	N           int       `json:"n"`
	Links       [][2]int  `json:"links"`
	Values      d3test.FS `json:"values"`
	Fixed       []int     `json:"fixed"`
	FixedValues d3test.FS `json:"fixedValues"`
	Opt         struct {
		Align      string    `json:"align"`
		Width      *float64  `json:"width"`
		Padding    *float64  `json:"padding"`
		Extent     []float64 `json:"extent"`
		Size       []float64 `json:"size"`
		Iterations *float64  `json:"iterations"`
		NodeSort   string    `json:"nodeSort"`
		LinkSort   string    `json:"linkSort"`
		IDs        string    `json:"ids"`
	} `json:"opt"`
	Err string `json:"err"`
	goldenResult
	Again   *goldenResult `json:"again"`
	Moves   [][2]float64  `json:"moves"`
	Updated *goldenResult `json:"updated"`
}

type golden struct {
	Groups struct {
		Layout []goldenCase `json:"layout"`
		Errors []goldenCase `json:"errors"`
	} `json:"groups"`
}

var testAligns = map[string]Align{"left": Left, "right": Right, "center": Center, "justify": Justify}

var testNodeSorts = map[string]func(a, b *Node) float64{
	"indexDesc": func(a, b *Node) float64 { return float64(b.Index - a.Index) },
	"valueAsc":  func(a, b *Node) float64 { return a.Value - b.Value },
}

var testLinkSorts = map[string]func(a, b *Link) float64{
	"valueDesc": func(a, b *Link) float64 { return b.Value - a.Value },
	"indexDesc": func(a, b *Link) float64 { return float64(b.Index - a.Index) },
}

func (c *goldenCase) generator() *Sankey {
	// Node.js 26 has the TimSort of V8. The algorithm matters only for the
	// cases in which a position is NaN.
	s := New().SortAlgorithm(js.V8TimSort)
	if c.Opt.IDs == "string" {
		s.NodeID(func(d *Node, _ int, _ []*Node) any { return d.Data })
	}
	if c.Opt.Align != "" {
		s.NodeAlign(testAligns[c.Opt.Align])
	}
	if c.Opt.Width != nil {
		s.NodeWidth(*c.Opt.Width)
	}
	if c.Opt.Padding != nil {
		s.NodePadding(*c.Opt.Padding)
	}
	if c.Opt.Extent != nil {
		s.Extent(c.Opt.Extent[0], c.Opt.Extent[1], c.Opt.Extent[2], c.Opt.Extent[3])
	}
	if c.Opt.Size != nil {
		s.Size(c.Opt.Size[0], c.Opt.Size[1])
	}
	if c.Opt.Iterations != nil {
		s.Iterations(*c.Opt.Iterations)
	}
	switch c.Opt.NodeSort {
	case "":
	case "null":
		s.NodeSort(nil)
	default:
		s.NodeSort(testNodeSorts[c.Opt.NodeSort])
	}
	switch c.Opt.LinkSort {
	case "":
	case "null":
		s.LinkSort(nil)
	default:
		s.LinkSort(testLinkSorts[c.Opt.LinkSort])
	}
	return s
}

func (c *goldenCase) graph() *Graph {
	g := &Graph{}
	for i := 0; i < c.N; i++ {
		g.Nodes = append(g.Nodes, &Node{Data: "n" + fmt.Sprint(i)})
	}
	for k, i := range c.Fixed {
		v := c.FixedValues[k]
		g.Nodes[i].FixedValue = &v
	}
	for k, l := range c.Links {
		link := &Link{Value: c.Values[k]}
		switch c.Opt.IDs {
		case "string":
			link.SourceID, link.TargetID = "n"+fmt.Sprint(l[0]), "n"+fmt.Sprint(l[1])
		case "index":
			link.SourceID, link.TargetID = l[0], l[1]
		default:
			// The node itself, or the index when there is no such node.
			link.SourceID, link.TargetID = l[0], l[1]
			if l[0] >= 0 && l[0] < c.N {
				link.Source = g.Nodes[l[0]]
			}
			if l[1] >= 0 && l[1] < c.N {
				link.Target = g.Nodes[l[1]]
			}
		}
		g.Links = append(g.Links, link)
	}
	return g
}

func indices(links []*Link) []int {
	out := make([]int, len(links))
	for i, l := range links {
		out[i] = l.Index
	}
	return out
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// compare returns the first difference between the graph and the golden
// result, or "".
func compare(g *Graph, want *goldenResult) string {
	if len(want.NI) != 4*len(g.Nodes) || len(want.NF) != 5*len(g.Nodes) || len(want.LF) != 3*len(g.Links) {
		return "the golden result has another size"
	}
	for i, d := range g.Nodes {
		got := []int{d.Index, d.Depth, d.Height, d.Layer}
		if !sameInts(got, want.NI[4*i:4*i+4]) {
			return fmt.Sprintf("node %d: index, depth, height, layer = %v, want %v", i, got, want.NI[4*i:4*i+4])
		}
		gotF := []float64{d.Value, d.X0, d.X1, d.Y0, d.Y1}
		if !d3test.SameSlice(gotF, want.NF[5*i:5*i+5]) {
			return fmt.Sprintf("node %d: value, x0, x1, y0, y1 = %v, want %v", i, gotF, want.NF[5*i:5*i+5])
		}
		if got := indices(d.SourceLinks); !sameInts(got, want.SL[i]) {
			return fmt.Sprintf("node %d: sourceLinks = %v, want %v", i, got, want.SL[i])
		}
		if got := indices(d.TargetLinks); !sameInts(got, want.TL[i]) {
			return fmt.Sprintf("node %d: targetLinks = %v, want %v", i, got, want.TL[i])
		}
	}
	for i, l := range g.Links {
		gotF := []float64{l.Width, l.Y0, l.Y1}
		if !d3test.SameSlice(gotF, want.LF[3*i:3*i+3]) {
			return fmt.Sprintf("link %d: width, y0, y1 = %v, want %v", i, gotF, want.LF[3*i:3*i+3])
		}
		if got := LinkHorizontal(l); got != want.Paths[i] {
			return fmt.Sprintf("link %d: path = %q, want %q", i, got, want.Paths[i])
		}
	}
	return ""
}

func TestGolden(t *testing.T) {
	var g golden
	d3test.Load(t, "testdata/sankey.json.gz", &g)
	if len(g.Groups.Layout) < 500 || len(g.Groups.Errors) < 100 {
		t.Fatalf("golden file has %d layouts and %d error cases", len(g.Groups.Layout), len(g.Groups.Errors))
	}
	run := func(group string, cases []goldenCase) {
		ok, failed, again, updated := 0, 0, 0, 0
		aligns := map[string]int{}
		for i := range cases {
			c := &cases[i]
			name := fmt.Sprintf("%s %d", group, i)
			graph := c.graph()
			s := c.generator()
			err := s.Layout(graph)
			if c.Err != "" {
				failed++
				if err == nil || err.Error() != c.Err {
					t.Errorf("%s: error = %v, want %q", name, err, c.Err)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s: error = %v, want a layout", name, err)
				continue
			}
			ok++
			aligns[c.Opt.Align]++
			if diff := compare(graph, &c.goldenResult); diff != "" {
				t.Errorf("%s: %s", name, diff)
				continue
			}
			if c.Again != nil {
				again++
				if err := s.Layout(graph); err != nil {
					t.Errorf("%s: second layout: %v", name, err)
				} else if diff := compare(graph, c.Again); diff != "" {
					t.Errorf("%s: second layout: %s", name, diff)
				}
			}
			if c.Updated != nil {
				updated++
				for _, m := range c.Moves {
					node := graph.Nodes[int(m[0])]
					node.Y0 += m[1]
					node.Y1 += m[1]
				}
				s.Update(graph)
				if diff := compare(graph, c.Updated); diff != "" {
					t.Errorf("%s: update: %s", name, diff)
				}
			}
		}
		t.Logf("%s: %d cases, %d layouts (alignments %v), %d errors, %d laid out twice, %d updated", group, len(cases), ok, aligns, failed, again, updated)
	}
	run("layout", g.Groups.Layout)
	run("errors", g.Groups.Errors)
}

// TestCustomAlignment checks the two results of a custom alignment for
// which the port returns an error: see computeNodeLayers.
func TestCustomAlignment(t *testing.T) {
	graph := func() *Graph {
		return &Graph{
			Nodes: []*Node{{}, {}},
			Links: []*Link{{SourceID: 0, TargetID: 1, Value: 1}},
		}
	}
	// Every node in the first column: the second column has no nodes, and
	// d3-sankey throws a TypeError.
	if err := New().NodeAlign(func(*Node, int) float64 { return 0 }).Layout(graph()); err == nil {
		t.Error("no error for an empty column")
	}
	// d3-sankey keeps a node with the column NaN outside of all columns.
	// The port does not support that.
	if err := New().NodeAlign(func(*Node, int) float64 { return math.NaN() }).Layout(graph()); err == nil {
		t.Error("no error for the column NaN")
	}
	// A custom alignment that fills every column works.
	if err := New().NodeAlign(func(d *Node, n int) float64 { return float64(d.Index) + 0.5 }).Layout(graph()); err != nil {
		t.Error(err)
	}
}
