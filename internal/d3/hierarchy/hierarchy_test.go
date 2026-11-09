package hierarchy

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden file comes from d3-hierarchy 3.1.2 in Node.js
// (tools/reference/gen-d3-hierarchy.mjs). Every number is compared by bits.

type goldenTree struct {
	N         int       `json:"n"`
	Kids      [][]int   `json:"kids"`
	Values    d3test.FS `json:"values"`
	Has       []int     `json:"has"`
	EmptyKids bool      `json:"emptyKids"`
	ValueMode string    `json:"valueMode"`
	SortMode  string    `json:"sortMode"`
}

type goldenQuery struct {
	A           int       `json:"a"`
	B           int       `json:"b"`
	Ancestors   []int     `json:"ancestors"`
	Path        []int     `json:"path"`
	Descendants []int     `json:"descendants"`
	Leaves      []int     `json:"leaves"`
	Links       []int     `json:"links"`
	Find        []int     `json:"find"`
	Copy        []int     `json:"copy"`
	CopyValues  d3test.FS `json:"copyValues"`
}

type goldenHierarchy struct {
	goldenTree
	Depth       []int         `json:"depth"`
	Height      []int         `json:"height"`
	Parent      []int         `json:"parent"`
	Value       d3test.FS     `json:"value"`
	Each        []int         `json:"each"`
	EachBefore  []int         `json:"eachBefore"`
	EachAfter   []int         `json:"eachAfter"`
	Descendants []int         `json:"descendants"`
	Leaves      []int         `json:"leaves"`
	Links       []int         `json:"links"`
	Queries     []goldenQuery `json:"queries"`
}

type goldenTreemap struct {
	goldenTree
	Size    []float64 `json:"size"`
	Round   bool      `json:"round"`
	Padding struct {
		Kind string    `json:"kind"`
		V    []float64 `json:"v"`
	} `json:"padding"`
	Tile   string    `json:"tile"`
	Bounds d3test.FS `json:"bounds"`
	Second *struct {
		Values d3test.FS `json:"values"`
		Resort bool      `json:"resort"`
		Bounds d3test.FS `json:"bounds"`
	} `json:"second"`
	Subtree *struct {
		ID     int       `json:"id"`
		Bounds d3test.FS `json:"bounds"`
	} `json:"subtree"`
}

type golden struct {
	Groups struct {
		Hierarchy []goldenHierarchy `json:"hierarchy"`
		Treemap   []goldenTreemap   `json:"treemap"`
	} `json:"groups"`
}

// datum is the data of a node in the golden cases: {id, value, children}.
type datum struct {
	id       int
	value    float64
	has      bool
	children []*datum
}

func (t *goldenTree) data() []*datum {
	nodes := make([]*datum, t.N)
	for i := range nodes {
		nodes[i] = &datum{id: i, value: t.Values[i], has: t.Has[i] != 0}
	}
	for i, kids := range t.Kids {
		for _, k := range kids {
			nodes[i].children = append(nodes[i].children, nodes[k])
		}
	}
	return nodes
}

func children(d *datum) []*datum { return d.children }

// datumValue is the value property of a datum. For a datum that has none the
// property is undefined, and the unary plus of JavaScript makes NaN of it.
func datumValue(d *datum) float64 {
	if !d.has {
		return math.NaN()
	}
	return d.value
}

func applyValues(root *Node[*datum], mode string) {
	switch mode {
	case "count":
		root.Count()
	case "value":
		root.Sum(datumValue)
	case "leaf":
		root.Sum(func(d *datum) float64 {
			if len(d.children) > 0 {
				return 0
			}
			return datumValue(d)
		})
	case "one":
		root.Sum(func(*datum) float64 { return 1 })
	}
}

var sortFns = map[string]func(a, b *Node[*datum]) float64{
	"valueDesc": func(a, b *Node[*datum]) float64 { return b.Value - a.Value },
	"valueAsc":  func(a, b *Node[*datum]) float64 { return a.Value - b.Value },
	"heightValue": func(a, b *Node[*datum]) float64 {
		if d := b.Height - a.Height; d != 0 {
			return float64(d)
		}
		return b.Value - a.Value
	},
	"idDesc": func(a, b *Node[*datum]) float64 { return float64(b.Data.id - a.Data.id) },
}

// applySort sorts with the TimSort of Node.js 26. The algorithm matters only
// when a comparator is not consistent.
func applySort(root *Node[*datum], mode string) {
	if mode != "none" {
		root.SortWith(js.V8TimSort, sortFns[mode])
	}
}

func ids(nodes []*Node[*datum]) []int {
	out := make([]int, len(nodes))
	for i, d := range nodes {
		out[i] = -1
		if d != nil {
			out[i] = d.Data.id
		}
	}
	return out
}

func linkIDs(links []Link[*datum]) []int {
	out := make([]int, 0, 2*len(links))
	for _, l := range links {
		out = append(out, l.Source.Data.id, l.Target.Data.id)
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

// byID lists the nodes in the order of the ids.
func byID(root *Node[*datum], n int) []*Node[*datum] {
	out := make([]*Node[*datum], n)
	root.Each(func(d *Node[*datum], _ int) { out[d.Data.id] = d })
	return out
}

func TestHierarchyGolden(t *testing.T) {
	var g golden
	d3test.Load(t, "testdata/hierarchy.json.gz", &g)
	if len(g.Groups.Hierarchy) < 300 {
		t.Fatalf("golden file has %d hierarchy cases", len(g.Groups.Hierarchy))
	}
	nodes, queries := 0, 0
	for ci := range g.Groups.Hierarchy {
		c := &g.Groups.Hierarchy[ci]
		fail := func(format string, args ...any) {
			t.Helper()
			t.Errorf("hierarchy %d: %s", ci, fmt.Sprintf(format, args...))
		}
		root := New(c.data()[0], children)
		all := byID(root, c.N)
		nodes += c.N
		for id, d := range all {
			if d == nil {
				fail("node %d is not in the hierarchy", id)
				continue
			}
			parent := -1
			if d.Parent != nil {
				parent = d.Parent.Data.id
			}
			if d.Depth != c.Depth[id] || d.Height != c.Height[id] || parent != c.Parent[id] {
				fail("node %d: depth, height, parent = %d, %d, %d, want %d, %d, %d", id, d.Depth, d.Height, parent, c.Depth[id], c.Height[id], c.Parent[id])
			}
		}
		applyValues(root, c.ValueMode)
		for id, d := range all {
			if d != nil && !d3test.Same(d.Value, c.Value[id]) {
				fail("node %d: value = %v, want %v (%s)", id, d.Value, c.Value[id], c.ValueMode)
			}
		}
		applySort(root, c.SortMode)

		record := func(each func(func(*Node[*datum], int)) *Node[*datum]) []int {
			var out []int
			each(func(d *Node[*datum], i int) {
				if i != len(out) {
					fail("callback index %d, want %d", i, len(out))
				}
				out = append(out, d.Data.id)
			})
			return out
		}
		for _, tc := range []struct {
			name      string
			got, want []int
		}{
			{"each", record(root.Each), c.Each},
			{"eachBefore", record(root.EachBefore), c.EachBefore},
			{"eachAfter", record(root.EachAfter), c.EachAfter},
			{"descendants", ids(root.Descendants()), c.Descendants},
			{"leaves", ids(root.Leaves()), c.Leaves},
			{"links", linkIDs(root.Links()), c.Links},
		} {
			if !sameInts(tc.got, tc.want) {
				fail("%s = %v, want %v (sort %s)", tc.name, tc.got, tc.want, c.SortMode)
			}
		}

		for qi, q := range c.Queries {
			queries++
			a, b := all[q.A], all[q.B]
			m, r := q.Find[0], q.Find[1]
			found := -1
			if d := a.Find(func(d *Node[*datum], _ int) bool { return d.Data.id%m == r }); d != nil {
				found = d.Data.id
			}
			cp := a.Copy()
			var copied []int
			var copyValues []float64
			cp.EachBefore(func(d *Node[*datum], _ int) {
				parent := -1
				if d.Parent != nil {
					parent = d.Parent.Data.id
				}
				copied = append(copied, d.Data.id, d.Depth, d.Height, parent)
				copyValues = append(copyValues, d.Value)
			})
			for _, tc := range []struct {
				name      string
				got, want []int
			}{
				{"ancestors", ids(a.Ancestors()), q.Ancestors},
				{"path", ids(a.Path(b)), q.Path},
				{"descendants", ids(a.Descendants()), q.Descendants},
				{"leaves", ids(a.Leaves()), q.Leaves},
				{"links", linkIDs(a.Links()), q.Links},
				{"find", []int{m, r, found}, q.Find},
				{"copy", copied, q.Copy},
			} {
				if !sameInts(tc.got, tc.want) {
					fail("query %d (nodes %d, %d): %s = %v, want %v", qi, q.A, q.B, tc.name, tc.got, tc.want)
				}
			}
			if !d3test.SameSlice(copyValues, q.CopyValues) {
				fail("query %d: values of the copy = %v, want %v", qi, copyValues, q.CopyValues)
			}
			// The copy has new nodes.
			if cp == a || (len(cp.Children) > 0 && cp.Children[0] == a.Children[0]) {
				fail("query %d: the copy shares nodes", qi)
			}
		}
	}
	t.Logf("%d hierarchies, %d nodes, %d queries", len(g.Groups.Hierarchy), nodes, queries)
}

func (c *goldenTreemap) layout(t *testing.T) *Treemap[*datum] {
	layout := NewTreemap[*datum]()
	if c.Size != nil {
		layout.Size(c.Size[0], c.Size[1])
	}
	if c.Round {
		layout.Round(true)
	}
	branch := func(v float64) func(*Node[*datum]) float64 {
		return func(d *Node[*datum]) float64 {
			if len(d.Children) > 0 {
				return v
			}
			return 0
		}
	}
	p := c.Padding.V
	switch c.Padding.Kind {
	case "none":
	case "inner":
		layout.PaddingInner(p[0])
	case "all":
		layout.Padding(p[0])
	case "sides":
		layout.PaddingTop(p[0]).PaddingRight(p[1]).PaddingBottom(p[2]).PaddingLeft(p[3]).PaddingInner(p[4])
	case "mermaid":
		layout.PaddingTopFunc(branch(35)).
			PaddingInner(p[0]).
			PaddingLeftFunc(branch(10)).
			PaddingRightFunc(branch(10)).
			PaddingBottomFunc(branch(10))
	case "depth":
		layout.PaddingInnerFunc(func(d *Node[*datum]) float64 { return float64(d.Depth*2 + 1) }).
			PaddingOuterFunc(func(d *Node[*datum]) float64 { return float64(d.Height) })
	default:
		t.Fatalf("unknown padding %q", c.Padding.Kind)
	}
	kind, ratioText, hasRatio := strings.Cut(c.Tile, ":")
	ratio := 0.0
	if hasRatio {
		var err error
		if ratio, err = strconv.ParseFloat(ratioText, 64); err != nil {
			t.Fatal(err)
		}
	}
	switch kind {
	case "default":
	case "squarify":
		if hasRatio {
			layout.Tile(SquarifyRatio[*datum](ratio))
		} else {
			layout.Tile(Squarify[*datum])
		}
	case "resquarify":
		if hasRatio {
			layout.Tile(ResquarifyRatio[*datum](ratio))
		} else {
			layout.Tile(Resquarify[*datum]())
		}
	case "binary":
		layout.Tile(Binary[*datum])
	case "dice":
		layout.Tile(Dice[*datum])
	case "slice":
		layout.Tile(Slice[*datum])
	case "sliceDice":
		layout.Tile(SliceDice[*datum])
	default:
		t.Fatalf("unknown tile %q", c.Tile)
	}
	return layout
}

func boundsDiff(all []*Node[*datum], want []float64) string {
	for id, d := range all {
		got := []float64{d.X0, d.Y0, d.X1, d.Y1}
		if !d3test.SameSlice(got, want[4*id:4*id+4]) {
			return fmt.Sprintf("node %d: x0, y0, x1, y1 = %v, want %v", id, got, want[4*id:4*id+4])
		}
	}
	return ""
}

func TestTreemapGolden(t *testing.T) {
	var g golden
	d3test.Load(t, "testdata/hierarchy.json.gz", &g)
	if len(g.Groups.Treemap) < 900 {
		t.Fatalf("golden file has %d treemap cases", len(g.Groups.Treemap))
	}
	tiles := map[string]int{}
	paddings := map[string]int{}
	second, subtree := 0, 0
	for ci := range g.Groups.Treemap {
		c := &g.Groups.Treemap[ci]
		name := fmt.Sprintf("treemap %d (tile %s, padding %s, values %s, sort %s)", ci, c.Tile, c.Padding.Kind, c.ValueMode, c.SortMode)
		data := c.data()
		root := New(data[0], children)
		all := byID(root, c.N)
		applyValues(root, c.ValueMode)
		applySort(root, c.SortMode)
		layout := c.layout(t)
		if got := layout.Layout(root); got != root {
			t.Errorf("%s: the layout returns another node", name)
		}
		tiles[c.Tile]++
		paddings[c.Padding.Kind]++
		if diff := boundsDiff(all, c.Bounds); diff != "" {
			t.Errorf("%s: %s", name, diff)
			continue
		}
		if c.Second != nil {
			second++
			for id, d := range data {
				if d.has {
					d.value = c.Second.Values[id]
				}
			}
			applyValues(root, c.ValueMode)
			if c.Second.Resort {
				applySort(root, c.SortMode)
			}
			layout.Layout(root)
			if diff := boundsDiff(all, c.Second.Bounds); diff != "" {
				t.Errorf("%s: second layout: %s", name, diff)
				continue
			}
		}
		if c.Subtree != nil {
			subtree++
			layout.Layout(all[c.Subtree.ID])
			if diff := boundsDiff(all, c.Subtree.Bounds); diff != "" {
				t.Errorf("%s: layout of node %d: %s", name, c.Subtree.ID, diff)
			}
		}
	}
	t.Logf("%d treemaps: tiles %v, paddings %v, %d with a second layout, %d with the layout of a node below the root", len(g.Groups.Treemap), tiles, paddings, second, subtree)
}
