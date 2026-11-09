package orthogonal

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p5edges/orthogonal/direction"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// The golden testdata/routeedges.json.gz has calls of
// OrthogonalRoutingGenerator.routeEdges of elkjs 0.9.3 with their results.
// tools/reference/gen-elk-orthogonal.mjs writes it and documents the format.

// num is a double of the golden: a JSON number, or {"$f": name} for a value
// that JSON cannot express.
type num float64

func (n *num) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '{' {
		var special struct {
			F string `json:"$f"`
		}
		if err := json.Unmarshal(data, &special); err != nil {
			return err
		}
		switch special.F {
		case "-0":
			*n = num(math.Copysign(0, -1))
		case "NaN":
			*n = num(math.NaN())
		case "Infinity":
			*n = num(math.Inf(1))
		case "-Infinity":
			*n = num(math.Inf(-1))
		default:
			return fmt.Errorf("unknown number %q", special.F)
		}
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	*n = num(f)
	return nil
}

type goldenPort struct {
	Side   int    `json:"sd"`
	Pos    [2]num `json:"pos"`
	Anchor [2]num `json:"an"`
	In     []int  `json:"in"`
	Out    []int  `json:"out"`
}

type goldenNode struct {
	Pos   [2]num       `json:"pos"`
	Ports []goldenPort `json:"ports"`
}

type goldenEdge struct {
	Source [2]int `json:"s"`
	Target [2]int `json:"t"`
	// BendPoints and JunctionPoints are the state before the call.
	BendPoints     [][2]num  `json:"bp"`
	JunctionPoints *[][2]num `json:"jp"`
}

type goldenEdgeResult struct {
	BendPoints     [][2]num  `json:"bp"`
	JunctionPoints *[][2]num `json:"jp"`
}

type goldenCall struct {
	Start       num                `json:"start"`
	Random      *[2]float64        `json:"rnd"`
	Source      *[]int             `json:"src"`
	Target      *[]int             `json:"tgt"`
	Nodes       []goldenNode       `json:"nodes"`
	Edges       []goldenEdge       `json:"edges"`
	Throws      bool               `json:"throws"`
	Ret         int                `json:"ret"`
	RandomAfter *[2]float64        `json:"rndAfter"`
	After       []goldenEdgeResult `json:"after"`
}

type goldenGroup struct {
	Kind    string       `json:"kind"`
	Dir     int          `json:"dir"`
	Spacing num          `json:"spacing"`
	Calls   []goldenCall `json:"calls"`
}

func loadGolden(t *testing.T) []goldenGroup {
	t.Helper()
	f, err := os.Open("testdata/routeedges.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Groups []goldenGroup `json:"groups"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&golden); err != nil {
		t.Fatal(err)
	}
	return golden.Groups
}

func chainOf(points [][2]num) *emath.KVectorChain {
	chain := emath.NewKVectorChain()
	for _, p := range points {
		chain.AddXY(float64(p[0]), float64(p[1]))
	}
	return chain
}

// builtCall is the graph of one call.
type builtCall struct {
	graph  *lgraph.LGraph
	source []*lgraph.LNode
	target []*lgraph.LNode
	edges  []*lgraph.LEdge
}

// build makes the graph of a call. It sets the fields directly, so that the
// order of each edge list is the order of the golden.
func build(c *goldenCall) *builtCall {
	b := &builtCall{graph: lgraph.NewLGraph()}
	if c.Random != nil {
		prop.Set(b.graph, iprops.Random, &gwt.Random{SeedHi: c.Random[0], SeedLo: c.Random[1]})
	}
	nodes := make([]*lgraph.LNode, len(c.Nodes))
	for i, gn := range c.Nodes {
		node := lgraph.NewLNode(b.graph)
		node.Pos.X, node.Pos.Y = float64(gn.Pos[0]), float64(gn.Pos[1])
		for _, gp := range gn.Ports {
			port := lgraph.NewLPort()
			port.Side = core.PortSide(gp.Side)
			port.Pos.X, port.Pos.Y = float64(gp.Pos[0]), float64(gp.Pos[1])
			port.Anchor.X, port.Anchor.Y = float64(gp.Anchor[0]), float64(gp.Anchor[1])
			port.SetNode(node)
		}
		nodes[i] = node
	}
	b.edges = make([]*lgraph.LEdge, len(c.Edges))
	for i, ge := range c.Edges {
		edge := lgraph.NewLEdge()
		edge.Source = nodes[ge.Source[0]].Ports[ge.Source[1]]
		edge.Target = nodes[ge.Target[0]].Ports[ge.Target[1]]
		edge.BendPoints = chainOf(ge.BendPoints)
		if ge.JunctionPoints != nil {
			prop.Set(edge, lopts.JunctionPoints, chainOf(*ge.JunctionPoints))
		}
		b.edges[i] = edge
	}
	for i, gn := range c.Nodes {
		for k, gp := range gn.Ports {
			port := nodes[i].Ports[k]
			for _, e := range gp.In {
				port.IncomingEdges = append(port.IncomingEdges, b.edges[e])
			}
			for _, e := range gp.Out {
				port.OutgoingEdges = append(port.OutgoingEdges, b.edges[e])
			}
		}
	}
	layer := func(ids *[]int) []*lgraph.LNode {
		if ids == nil {
			return nil
		}
		out := []*lgraph.LNode{}
		for _, id := range *ids {
			out = append(out, nodes[id])
		}
		return out
	}
	b.source = layer(c.Source)
	b.target = layer(c.Target)
	return b
}

func sameBits(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }

// diffChain compares a chain with the points of the golden, bit for bit.
func diffChain(what string, got *emath.KVectorChain, want [][2]num) string {
	if got.Len() != len(want) {
		return fmt.Sprintf("%s: %d points, want %d (got %v)", what, got.Len(), len(want), got)
	}
	for i, p := range got.Points {
		if !sameBits(p.X, float64(want[i][0])) || !sameBits(p.Y, float64(want[i][1])) {
			return fmt.Sprintf("%s: point %d is (%v, %v), want (%v, %v)", what, i, p.X, p.Y, float64(want[i][0]), float64(want[i][1]))
		}
	}
	return ""
}

// route runs RouteEdges and reports a stop of the layout.
func route(generator *OrthogonalRoutingGenerator, b *builtCall, start float64) (ret int, bailed bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*lgraph.BailError); ok {
				bailed = true
				return
			}
			panic(r)
		}
	}()
	return generator.RouteEdges(b.graph, b.source, 0, b.target, start), false
}

// checkCall runs one call and returns the differences from the golden.
func checkCall(generator *OrthogonalRoutingGenerator, c *goldenCall) []string {
	b := build(c)
	ret, bailed := route(generator, b, float64(c.Start))
	if c.Throws {
		if !bailed {
			return []string{"elkjs throws, and the port returned a result"}
		}
		return nil
	}
	if bailed {
		return []string{"the port stopped the layout"}
	}
	var diffs []string
	if ret != c.Ret {
		diffs = append(diffs, fmt.Sprintf("the result is %d, want %d", ret, c.Ret))
	}
	if c.RandomAfter != nil {
		random := prop.Get(b.graph, iprops.Random)
		if !sameBits(random.SeedHi, c.RandomAfter[0]) || !sameBits(random.SeedLo, c.RandomAfter[1]) {
			diffs = append(diffs, fmt.Sprintf("the random state is (%v, %v), want (%v, %v)", random.SeedHi, random.SeedLo, c.RandomAfter[0], c.RandomAfter[1]))
		}
	}
	for i, edge := range b.edges {
		want := c.After[i]
		if d := diffChain(fmt.Sprintf("edge %d bend points", i), edge.BendPoints, want.BendPoints); d != "" {
			diffs = append(diffs, d)
		}
		junctionPoints, has := prop.Lookup(edge, lopts.JunctionPoints)
		switch {
		case has != (want.JunctionPoints != nil):
			diffs = append(diffs, fmt.Sprintf("edge %d: has junction points is %v, want %v", i, has, want.JunctionPoints != nil))
		case has:
			if d := diffChain(fmt.Sprintf("edge %d junction points", i), junctionPoints, *want.JunctionPoints); d != "" {
				diffs = append(diffs, d)
			}
		}
	}
	return diffs
}

// TestRouteEdges compares RouteEdges with routeEdges of elkjs for each call
// of the golden: the number of routing slots, the state of the random number
// generator, and the bend points and the junction points of each edge, bit
// for bit.
func TestRouteEdges(t *testing.T) {
	groups := loadGolden(t)
	calls, failed, routed, throws := 0, 0, 0, 0
	byDir := map[direction.RoutingDirection]int{}
	byKind := map[string]int{}
	for gi, group := range groups {
		dir := direction.RoutingDirection(group.Dir)
		generator := NewOrthogonalRoutingGenerator(dir, float64(group.Spacing), "test")
		for ci := range group.Calls {
			c := &group.Calls[ci]
			calls++
			byDir[dir]++
			byKind[group.Kind]++
			if c.Throws {
				throws++
			} else if c.Ret > 0 {
				routed++
			}
			diffs := checkCall(generator, c)
			if len(diffs) == 0 {
				continue
			}
			failed++
			if failed <= 5 {
				if len(diffs) > 8 {
					diffs = diffs[:8]
				}
				t.Errorf("group %d (%s, %v, spacing %v), call %d differs from elkjs:\n  %s", gi, group.Kind, dir, float64(group.Spacing), ci, strings.Join(diffs, "\n  "))
			}
		}
	}
	t.Logf("%d calls in %d groups (%v, %v), %d calls route edges, %d calls stop the layout, %d calls differ from elkjs", calls, len(groups), byKind, byDir, routed, throws, failed)
	if failed > 0 {
		t.Errorf("%d of %d calls differ from elkjs", failed, calls)
	}
	if calls == 0 {
		t.Error("the golden has no calls")
	}
}

// bails tells that fn stops the layout with lgraph.Bail.
func bails(fn func()) (bailed bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*lgraph.BailError); ok {
				bailed = true
				return
			}
			panic(r)
		}
	}()
	fn()
	return false
}

// TestBail checks the cases where Java throws: the port stops the layout and
// does not panic with a runtime error.
func TestBail(t *testing.T) {
	if !bails(func() { NewOrthogonalRoutingGenerator(direction.RoutingDirection(7), 10, "") }) {
		t.Error("a generator for an unknown routing direction does not stop the layout")
	}
	if !bails(func() {
		port := lgraph.NewLPort()
		port.SetNode(lgraph.NewLNode(lgraph.NewLGraph()))
		NewHyperEdgeSegment(nil).AddPortPositions(port, gwt.NewMap[*lgraph.LPort, *HyperEdgeSegment]())
	}) {
		t.Error("AddPortPositions without a routing strategy does not stop the layout")
	}
	if !bails(func() {
		strategy := direction.ForRoutingDirection(direction.RoutingDirectionWestToEast)
		NewHyperEdgeSegment(strategy).AddPortPositions(lgraph.NewLPort(), gwt.NewMap[*lgraph.LPort, *HyperEdgeSegment]())
	}) {
		t.Error("AddPortPositions for a port without a node does not stop the layout")
	}
}
