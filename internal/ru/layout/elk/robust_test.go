package elk

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	rugraph "github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
)

// The tests of this file feed malformed layout data and malformed results
// of ELK to the layout. The layout may return an error. It must not panic
// and it must end.

// rng is mulberry32.
type rng struct{ a uint32 }

func (r *rng) next() float64 {
	r.a += 0x6d2b79f5
	t := r.a
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

func (r *rng) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() * float64(n))
}

func (r *rng) chance(p float64) bool { return r.next() < p }

// oddNumbers are the numbers that a mutation puts in place of a size or a
// coordinate.
var oddNumbers = []float64{0, -1, 1e-9, 0.5, 13.999, 1e7, 1e300, -1e300, math.NaN(), math.Inf(1), math.Inf(-1)}

func (r *rng) odd() float64 { return oddNumbers[r.intn(len(oddNumbers))] }

func cloneObjs(list []*js.Obj[any]) []*js.Obj[any] {
	out := make([]*js.Obj[any], len(list))
	for i, o := range list {
		out[i] = js.AsObj(js.Clone(o))
	}
	return out
}

// mutateData changes the nodes and the edges of recorded layout data at
// random.
func mutateData(r *rng, nodes, edges []*js.Obj[any]) ([]*js.Obj[any], []*js.Obj[any]) {
	ids := []any{"missing", "", "undefined", "root", "constructor", "__proto__"}
	for _, n := range nodes {
		ids = append(ids, n.Get("id"))
	}
	anyID := func() any { return ids[r.intn(len(ids))] }
	for k := r.intn(4) + 1; k > 0; k-- {
		if len(nodes) == 0 {
			break
		}
		n := nodes[r.intn(len(nodes))]
		switch r.intn(12) {
		case 0:
			// A parent that can make a cycle: any node, also the node itself.
			n.Set("parentId", anyID())
		case 1:
			n.Set("isGroup", !js.Truthy(n.Get("isGroup")))
		case 2:
			n.Set("id", anyID())
		case 3:
			n.Delete("id")
		case 4:
			n.Set("width", r.odd())
			n.Set("height", r.odd())
		case 5:
			n.Delete("width")
		case 6:
			n.Set("labelBBox", obj("width", r.odd(), "height", r.odd()))
		case 7:
			n.Set("padding", r.odd())
		case 8:
			n.Set("shape", []any{"rect33", "noteGroup", "", "nope"}[r.intn(4)])
		case 9:
			nodes = append(nodes, js.AsObj(js.Clone(n)))
		case 10:
			n.Set("metadata", obj("algorithm", []any{"elk.box", "elk.rectpacking", "elk.nope", 3.0}[r.intn(4)]))
			n.Set("dir", "LR")
		case 11:
			i := r.intn(len(nodes))
			nodes = append(nodes[:i], nodes[i+1:]...)
		}
	}
	for k := r.intn(4); k > 0; k-- {
		if len(edges) == 0 {
			break
		}
		e := edges[r.intn(len(edges))]
		switch r.intn(8) {
		case 0:
			e.Set("start", anyID())
		case 1:
			e.Set("end", anyID())
		case 2:
			e.Delete("start")
		case 3:
			e.Set("id", edges[r.intn(len(edges))].Get("id"))
		case 4:
			e.Set("width", r.odd())
			e.Set("height", r.odd())
			e.Set("label", "x")
		case 5:
			e.Set("terminalLabelSizes", obj("startLeft", obj("width", r.odd(), "height", r.odd()), "endRight", obj("width", 20.0, "height", 16.5), "other", obj("width", 1.0, "height", 1.0)))
			e.Set("startLabelLeft", "1")
			e.Set("endLabelRight", "n")
		case 6:
			edges = append(edges, js.AsObj(js.Clone(e)))
		case 7:
			e.Delete("id")
		}
	}
	return nodes, edges
}

// fakeElk stands for the ELK engine: it gives each node of the graph a
// position and each edge a route, as elkjs does, with random numbers. With
// garbage set, some values are missing or have a wrong type.
func fakeElk(r *rng, garbage bool) func(graphJSON []byte) ([]byte, error) {
	coord := func() any {
		if garbage && r.chance(0.08) {
			switch r.intn(4) {
			case 0:
				return nil
			case 1:
				return "12"
			case 2:
				return js.Null
			}
			return r.odd()
		}
		return float64(int(r.next()*60000)) / 64
	}
	point := func() any {
		if garbage && r.chance(0.03) {
			return nil
		}
		return obj("x", coord(), "y", coord())
	}
	return func(graphJSON []byte) ([]byte, error) {
		if garbage && r.chance(0.03) {
			return []byte([]string{`[]`, `null`, `5`, `{"children":7,"edges":"x"}`, `{`, `{"children":[null,3,"a"],"edges":[null,{}]}`}[r.intn(6)]), nil
		}
		parsed, err := js.ParseJSON(graphJSON)
		if err != nil {
			return nil, err
		}
		var walk func(children any)
		walk = func(children any) {
			arr, _ := children.([]any)
			for _, c := range arr {
				node := js.AsObj(c)
				if node == nil {
					continue
				}
				node.Set("x", coord())
				node.Set("y", coord())
				if !node.Has("width") || (garbage && r.chance(0.1)) {
					node.Set("width", coord())
					node.Set("height", coord())
				}
				if labels, ok := node.Get("labels").([]any); ok {
					for _, l := range labels {
						if label := js.AsObj(l); label != nil {
							label.Set("x", coord())
							label.Set("y", coord())
						}
					}
				}
				if garbage && r.chance(0.03) {
					node.Delete("children")
				}
				walk(node.Get("children"))
			}
		}
		root := js.AsObj(parsed)
		walk(prop(root, "children"))
		edges, _ := prop(root, "edges").([]any)
		for _, e := range edges {
			edge := js.AsObj(e)
			if edge == nil {
				continue
			}
			if labels, ok := edge.Get("labels").([]any); ok {
				for _, l := range labels {
					if label := js.AsObj(l); label != nil && r.chance(0.8) {
						label.Set("x", coord())
						label.Set("y", coord())
					}
				}
			}
			switch {
			case r.chance(0.15):
				// No sections: elk.box and elk.rectpacking route no edges.
			case garbage && r.chance(0.05):
				edge.Set("sections", []any{})
			case garbage && r.chance(0.05):
				edge.Set("sections", []any{nil, obj("startPoint", 3.0)})
			default:
				section := obj("startPoint", point(), "endPoint", point())
				if r.chance(0.7) {
					var bends []any
					for k := r.intn(5); k > 0; k-- {
						bends = append(bends, point())
					}
					if bends != nil {
						section.Set("bendPoints", bends)
					}
				}
				edge.Set("sections", []any{section})
			}
		}
		if garbage && r.chance(0.03) {
			root.Delete("edges")
		}
		return []byte(js.StringifyJSON(parsed)), nil
	}
}

// runMalformed runs the layout and reports a panic as an error text.
func runMalformed(c *elkCall, nodes, edges []*js.Obj[any], algorithm string) (panicText string) {
	defer func() {
		if p := recover(); p != nil {
			panicText = fmt.Sprint(p)
		}
	}()
	data := &ru.LayoutData{Config: c.dataConfig}
	if s, ok := c.direction.(string); ok {
		data.Direction = s
	}
	for i, o := range nodes {
		n := ru.NodeFromJS(o)
		// A rectangle, or a function that gives numbers that are not finite.
		switch i % 3 {
		case 0:
			n.Intersect = kindIntersect("rect", n)
		case 1:
			n.Intersect = kindIntersect("ellipse", n)
		}
		data.Nodes = append(data.Nodes, n)
	}
	for _, o := range edges {
		data.Edges = append(data.Edges, edgeFromJS(o))
	}
	rc := &rugraph.RenderContext{
		Ctx:     &render.Context{Config: c.getConfig},
		Options: rugraph.RenderOptions{Algorithm: algorithm},
	}
	if _, err := prepareLayoutForElk(data, rc); err != nil {
		return ""
	}
	_, _ = runElkLayoutCore(data, rc)
	return ""
}

// TestMalformedLayoutData mutates the recorded layout data and the results
// of ELK. The layout must not panic and must end.
func TestMalformedLayoutData(t *testing.T) {
	calls := loadCalls(t)
	defer func(orig func([]byte) ([]byte, error)) { elkLayout = orig }(elkLayout)
	realElk := elkLayout

	type job struct {
		name string
		run  func() string
	}
	var jobs []job
	r := &rng{a: 0x454c4b21}
	for i := 0; i < 900; i++ {
		c := calls[r.intn(len(calls))]
		seed := uint32(7000 + i)
		// Every 30th case runs the real engine on mutated data. The others
		// run the fake engine: with sound data and a garbage result, or
		// with mutated data.
		mode := i % 30
		jobs = append(jobs, job{
			name: fmt.Sprintf("%s#%d", c.fixture, i),
			run: func() string {
				rr := &rng{a: seed}
				nodes, edges := cloneObjs(c.nodes), cloneObjs(c.edges)
				switch {
				case mode == 0:
					nodes, edges = mutateData(rr, nodes, edges)
					elkLayout = realElk
				case mode%2 == 1:
					elkLayout = fakeElk(rr, true)
				default:
					nodes, edges = mutateData(rr, nodes, edges)
					elkLayout = fakeElk(rr, rr.chance(0.5))
				}
				return runMalformed(c, nodes, edges, c.algorithm)
			},
		})
	}
	// A cycle of parents, and a group inside a group with the same id.
	special := func(name string, nodes, edges []*js.Obj[any]) {
		jobs = append(jobs, job{name: name, run: func() string {
			elkLayout = fakeElk(&rng{a: 1}, false)
			return runMalformed(calls[0], nodes, edges, "elk.layered")
		}})
	}
	special("parent cycle", []*js.Obj[any]{
		obj("id", "A", "isGroup", true, "parentId", "B"),
		obj("id", "B", "isGroup", true, "parentId", "A"),
		obj("id", "x", "isGroup", false, "parentId", "A", "width", 10.0, "height", 10.0),
		obj("id", "y", "isGroup", false, "width", 10.0, "height", 10.0),
	}, []*js.Obj[any]{obj("id", "e", "start", "x", "end", "y")})
	special("nested groups with one id", []*js.Obj[any]{
		obj("id", "G", "isGroup", true),
		obj("id", "G", "isGroup", true, "parentId", "G"),
		obj("id", "x", "isGroup", false, "parentId", "G", "width", 10.0, "height", 10.0),
	}, []*js.Obj[any]{obj("id", "e", "start", "x", "end", "G")})

	var panics []string
	for _, j := range jobs {
		done := make(chan string, 1)
		go func() { done <- j.run() }()
		select {
		case text := <-done:
			if text != "" {
				panics = append(panics, j.name+": panic: "+text)
			}
		case <-time.After(60 * time.Second):
			t.Fatalf("%s: the layout does not end", j.name)
		}
	}
	if len(panics) > 0 {
		t.Errorf("%d of %d cases panic:\n  %s", len(panics), len(jobs), strings.Join(panics[:min(len(panics), 10)], "\n  "))
	}
	t.Logf("%d cases", len(jobs))
}

// TestParentCycleIsAnError checks that data on which upstream does not
// terminate gives an error.
func TestParentCycleIsAnError(t *testing.T) {
	defer func(orig func([]byte) ([]byte, error)) { elkLayout = orig }(elkLayout)
	elkLayout = fakeElk(&rng{a: 1}, false)
	data := &ru.LayoutData{Config: js.NewObj[any]()}
	for _, o := range []*js.Obj[any]{
		obj("id", "A", "isGroup", true, "parentId", "B"),
		obj("id", "B", "isGroup", true, "parentId", "A"),
		obj("id", "x", "isGroup", false, "parentId", "A", "width", 10.0, "height", 10.0),
		obj("id", "y", "isGroup", false, "width", 10.0, "height", 10.0),
	} {
		data.Nodes = append(data.Nodes, ru.NodeFromJS(o))
	}
	data.Edges = append(data.Edges, edgeFromJS(obj("id", "e", "start", "x", "end", "y")))
	rc := &rugraph.RenderContext{Ctx: &render.Context{Config: js.NewObj[any]()}, Options: rugraph.RenderOptions{Algorithm: "elk.layered"}}
	if _, err := runElkLayoutCore(data, rc); err == nil {
		t.Error("no error for a cycle of parents")
	}
}
