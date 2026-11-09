package tidytree

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file is the output of tools/reference/gen-tidytree.mjs, which
// runs the layout of @mermaid-js/layout-tidy-tree in node. Its header
// comment has the format.

// num is a number of the golden file: a JSON number, one of the strings
// "NaN", "Infinity", "-Infinity" and "-0", or null for undefined.
type num struct {
	v     float64
	undef bool
}

func (n *num) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "null":
		*n = num{v: ru.Undef, undef: true}
	case `"NaN"`:
		*n = num{v: math.NaN()}
	case `"Infinity"`:
		*n = num{v: math.Inf(1)}
	case `"-Infinity"`:
		*n = num{v: math.Inf(-1)}
	case `"-0"`:
		*n = num{v: math.Copysign(0, -1)}
	default:
		*n = num{}
		return json.Unmarshal(data, &n.v)
	}
	return nil
}

// same reports whether the port has the number of upstream: the same bits,
// or NaN on both sides. In the port an undefined number is NaN too.
func (n num) same(got float64) bool {
	if math.IsNaN(n.v) {
		return math.IsNaN(got)
	}
	return math.Float64bits(n.v) == math.Float64bits(got)
}

func (n num) String() string {
	if n.undef {
		return "undefined"
	}
	return fmt.Sprintf("%v (%#016x)", n.v, math.Float64bits(n.v))
}

type goldenNode struct {
	ID     string  `json:"id"`
	Width  num     `json:"width"`
	Height num     `json:"height"`
	Shape  *string `json:"shape"`
}

type goldenEdge struct {
	ID    *string `json:"id"`
	Start *string `json:"start"`
	End   *string `json:"end"`
}

type goldenCase struct {
	Name   string       `json:"name"`
	Nodes  []goldenNode `json:"nodes"`
	Edges  []goldenEdge `json:"edges"`
	Random int          `json:"random"`
	Error  *struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"error"`
	Result *struct {
		Nodes [][]json.RawMessage `json:"nodes"`
		Edges [][]json.RawMessage `json:"edges"`
	} `json:"result"`
}

type goldenEdgePositions struct {
	Nodes  [][]json.RawMessage `json:"nodes"`
	Edges  [][]*string         `json:"edges"`
	Shift  float64             `json:"shift"`
	Result [][]json.RawMessage `json:"result"`
}

type goldenFile struct {
	Package   string       `json:"package"`
	Version   string       `json:"version"`
	Cases     []goldenCase `json:"cases"`
	Internals struct {
		Intersection  [][]num               `json:"intersection"`
		Circle        [][]num               `json:"circle"`
		EdgePositions []goldenEdgePositions `json:"edgePositions"`
	} `json:"internals"`
}

var (
	goldenOnce sync.Once
	golden     *goldenFile
	goldenErr  error
)

func readGolden(t *testing.T) *goldenFile {
	t.Helper()
	goldenOnce.Do(func() {
		f, err := os.Open(filepath.Join("testdata", "layout.json.gz"))
		if err != nil {
			goldenErr = err
			return
		}
		defer f.Close()
		zr, err := gzip.NewReader(f)
		if err != nil {
			goldenErr = err
			return
		}
		golden = &goldenFile{}
		goldenErr = json.NewDecoder(zr).Decode(golden)
	})
	if goldenErr != nil {
		t.Fatal(goldenErr)
	}
	if golden.Package != "@mermaid-js/layout-tidy-tree" || golden.Version != "1.0.1" {
		t.Fatalf("the golden file is from %s %s", golden.Package, golden.Version)
	}
	return golden
}

func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// field decodes one field of a row of the golden file.
func field[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("golden field %s: %v", raw, err)
	}
	return v
}

// errorName is the name of the JavaScript error that an error of the
// layout stands for.
func errorName(err error) string {
	switch err.(type) {
	case typeError:
		return "TypeError"
	case rangeError:
		return "RangeError"
	}
	return "Error"
}

// differ collects the differences of one case.
type differ struct {
	t     *testing.T
	diffs []string
}

func (d *differ) diff(format string, args ...any) {
	d.diffs = append(d.diffs, fmt.Sprintf(format, args...))
}

func (d *differ) number(what string, got float64, raw json.RawMessage) {
	d.t.Helper()
	want := field[num](d.t, raw)
	if !want.same(got) {
		d.diff("%s: got %v (%#016x), want %s", what, got, math.Float64bits(got), want)
	}
}

func (d *differ) text(what, got string, raw json.RawMessage) {
	d.t.Helper()
	if want := str(field[*string](d.t, raw)); got != want {
		d.diff("%s: got %q, want %q", what, got, want)
	}
}

// edges compares positioned edges with their rows of the golden file.
func (d *differ) edges(got []*PositionedEdge, want [][]json.RawMessage) {
	d.t.Helper()
	if len(got) != len(want) {
		d.diff("edges: got %d, want %d", len(got), len(want))
		return
	}
	for i, row := range want {
		e := got[i]
		what := fmt.Sprintf("edge %d (%s)", i, e.ID)
		d.text(what+" id", e.ID, row[0])
		d.text(what+" source", e.Source, row[1])
		d.text(what+" target", e.Target, row[2])
		d.number(what+" startX", e.StartX, row[3])
		d.number(what+" startY", e.StartY, row[4])
		d.number(what+" midX", e.MidX, row[5])
		d.number(what+" midY", e.MidY, row[6])
		d.number(what+" endX", e.EndX, row[7])
		d.number(what+" endY", e.EndY, row[8])
		points := field[[]json.RawMessage](d.t, row[9])
		if len(points) != 2*len(e.Points) {
			d.diff("%s points: got %d, want %d", what, len(e.Points), len(points)/2)
		} else {
			for j, p := range e.Points {
				d.number(fmt.Sprintf("%s point %d x", what, j), p.X, points[2*j])
				d.number(fmt.Sprintf("%s point %d y", what, j), p.Y, points[2*j+1])
			}
		}
		d.text(what+" sourceSection", e.SourceSection, row[10])
		d.text(what+" targetSection", e.TargetSection, row[11])
		d.number(what+" sourceWidth", e.SourceWidth, row[12])
		d.number(what+" sourceHeight", e.SourceHeight, row[13])
		d.number(what+" targetWidth", e.TargetWidth, row[14])
		d.number(what+" targetHeight", e.TargetHeight, row[15])
	}
}

// compareCase runs the layout for one case and returns the differences
// from the result of upstream.
func compareCase(t *testing.T, c *goldenCase) []string {
	t.Helper()
	nodes := make([]*ru.Node, len(c.Nodes))
	for i, n := range c.Nodes {
		node := ru.NewNode(n.ID)
		node.Width = n.Width.v
		node.Height = n.Height.v
		node.Shape = str(n.Shape)
		nodes[i] = node
	}
	var edges []*ru.Edge
	if c.Edges != nil {
		edges = make([]*ru.Edge, len(c.Edges))
		for i, e := range c.Edges {
			edge := ru.NewEdge(str(e.ID))
			edge.Start = str(e.Start)
			edge.End = str(e.End)
			edges[i] = edge
		}
	}
	index := map[*ru.Node]int{}
	for i, node := range nodes {
		index[node] = i
	}

	random := 0
	result, err := executeTidyTreeLayout(&env{random: func() float64 {
		random++
		return 0.5
	}}, nodes, edges)

	d := &differ{t: t}
	if random != c.Random {
		d.diff("calls of Math.random: got %d, want %d", random, c.Random)
	}
	if c.Error != nil {
		if err == nil {
			d.diff("got a result, want the error %s: %s", c.Error.Name, c.Error.Message)
		} else if errorName(err) != c.Error.Name || err.Error() != c.Error.Message {
			d.diff("error: got %s: %s, want %s: %s", errorName(err), err, c.Error.Name, c.Error.Message)
		}
		return d.diffs
	}
	if err != nil {
		d.diff("got the error %s: %s, want a result", errorName(err), err)
		return d.diffs
	}

	if len(result.Nodes) != len(c.Result.Nodes) {
		d.diff("nodes: got %d, want %d", len(result.Nodes), len(c.Result.Nodes))
		return d.diffs
	}
	for i, want := range c.Result.Nodes {
		got := result.Nodes[i]
		what := fmt.Sprintf("node %d (%s)", i, got.ID)
		d.text(what+" id", got.ID, want[0])
		d.number(what+" x", got.X, want[1])
		d.number(what+" y", got.Y, want[2])
		d.text(what+" section", got.Section, want[3])
		d.number(what+" width", got.Width, want[4])
		d.number(what+" height", got.Height, want[5])
		wantOrig := field[int](t, want[6])
		gotOrig, ok := index[got.OriginalNode]
		if !ok {
			gotOrig = -1
		}
		if gotOrig != wantOrig {
			d.diff("%s originalNode: got node %d, want node %d", what, gotOrig, wantOrig)
		}
	}
	d.edges(result.Edges, c.Result.Edges)
	return d.diffs
}

// report prints the first differences of a group of cases and fails the
// test when a case differs.
type report struct {
	t      *testing.T
	total  int
	failed int
}

func (r *report) add(name string, diffs []string) {
	r.t.Helper()
	r.total++
	if len(diffs) == 0 {
		return
	}
	r.failed++
	if r.failed <= 10 {
		if len(diffs) > 6 {
			diffs = append(diffs[:6:6], fmt.Sprintf("and %d more", len(diffs)-6))
		}
		r.t.Errorf("%s:\n  %s", name, strings.Join(diffs, "\n  "))
	}
}

func (r *report) done(min int) {
	r.t.Helper()
	r.t.Logf("%d cases, %d exact, %d different", r.total, r.total-r.failed, r.failed)
	if r.failed > 10 {
		r.t.Errorf("%d cases differ", r.failed)
	}
	if r.total < min {
		r.t.Errorf("the golden file has %d cases, want at least %d", r.total, min)
	}
}

// TestLayoutGolden compares executeTidyTreeLayout with the layout of
// @mermaid-js/layout-tidy-tree: the positioned nodes and edges, the error
// when the layout fails, and the number of calls of Math.random. Every
// number is compared by its bits.
func TestLayoutGolden(t *testing.T) {
	golden := readGolden(t)
	r := &report{t: t}
	kinds := map[string]int{}
	for i := range golden.Cases {
		c := &golden.Cases[i]
		kind, _, _ := strings.Cut(c.Name, "/")
		if c.Error != nil {
			kind += " " + c.Error.Name
		}
		kinds[kind]++
		r.add(c.Name, compareCase(t, c))
	}
	t.Logf("kinds: %v", kinds)
	r.done(900)
}

// TestIntersectionGolden compares intersection with direct calls of the
// function of upstream, by the bits of the result.
func TestIntersectionGolden(t *testing.T) {
	golden := readGolden(t)
	r := &report{t: t}
	for i, c := range golden.Internals.Intersection {
		node := &PositionedNode{X: c[0].v, Y: c[1].v, Width: c[2].v, Height: c[3].v}
		got := intersection(node, ru.Point{X: c[4].v, Y: c[5].v}, ru.Point{X: c[6].v, Y: c[7].v})
		var diffs []string
		if !c[8].same(got.X) || !c[9].same(got.Y) {
			diffs = []string{fmt.Sprintf("node (%v, %v) %v x %v, outside (%v, %v), inside (%v, %v): got (%v, %v), want (%s, %s)",
				c[0].v, c[1].v, c[2], c[3], c[4].v, c[5].v, c[6].v, c[7].v, got.X, got.Y, c[8], c[9])}
		}
		r.add(fmt.Sprintf("intersection %d", i), diffs)
	}
	r.done(4000)
}

// TestCircleIntersectionGolden compares computeCircleEdgeIntersection with
// direct calls of the function of upstream, by the bits of the result.
func TestCircleIntersectionGolden(t *testing.T) {
	golden := readGolden(t)
	r := &report{t: t}
	for i, c := range golden.Internals.Circle {
		circle := ru.Bounds{X: c[0].v, Y: c[1].v, Width: c[2].v, Height: c[3].v}
		got := computeCircleEdgeIntersection(circle, ru.Point{X: c[4].v, Y: c[5].v}, ru.Point{X: c[6].v, Y: c[7].v})
		var diffs []string
		if !c[8].same(got.X) || !c[9].same(got.Y) {
			diffs = []string{fmt.Sprintf("circle (%v, %v) %v x %v, line (%v, %v) to (%v, %v): got (%v, %v), want (%s, %s)",
				c[0].v, c[1].v, c[2].v, c[3].v, c[4].v, c[5].v, c[6].v, c[7].v, got.X, got.Y, c[8], c[9])}
		}
		r.add(fmt.Sprintf("circle %d", i), diffs)
	}
	r.done(1500)
}

// TestEdgePositionsGolden compares calculateEdgePositions with direct
// calls of the function of upstream. The positioned nodes have sections
// that the layout itself never makes, for example an edge that ends at the
// root.
func TestEdgePositionsGolden(t *testing.T) {
	golden := readGolden(t)
	r := &report{t: t}
	for i := range golden.Internals.EdgePositions {
		c := &golden.Internals.EdgePositions[i]
		nodes := make([]*PositionedNode, len(c.Nodes))
		for j, row := range c.Nodes {
			node := &PositionedNode{
				ID:      field[string](t, row[0]),
				X:       field[num](t, row[1]).v,
				Y:       field[num](t, row[2]).v,
				Section: str(field[*string](t, row[3])),
				Width:   field[num](t, row[4]).v,
				Height:  field[num](t, row[5]).v,
			}
			// null is a node with no original node, and the empty string
			// is an original node with no shape.
			if shape := field[*string](t, row[6]); shape != nil {
				node.OriginalNode = ru.NewNode(node.ID)
				node.OriginalNode.Shape = *shape
			}
			nodes[j] = node
		}
		edges := make([]*ru.Edge, len(c.Edges))
		for j, row := range c.Edges {
			edge := ru.NewEdge(str(row[0]))
			edge.Start = str(row[1])
			edge.End = str(row[2])
			edges[j] = edge
		}
		d := &differ{t: t}
		d.edges(calculateEdgePositions(edges, nodes, c.Shift), c.Result)
		r.add(fmt.Sprintf("edgePositions %d", i), d.diffs)
	}
	r.done(600)
}
