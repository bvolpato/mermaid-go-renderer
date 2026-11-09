package dagre

import (
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// dumpFields formats the properties of a label as the debug mode of
// tools/reference/gen-dagre.mjs does: key=value pairs, sorted by key. A
// property that is undefined, NaN, false or empty is left out.
type dumpFields map[string]string

func (f dumpFields) num(k string, v float64) {
	if v == v {
		f[k] = fmtNum(v)
	}
}

func (f dumpFields) str(k, v string) {
	if v != "" {
		f[k] = v
	}
}

func (f dumpFields) flag(k string, v bool) {
	if v {
		f[k] = "true"
	}
}

func (f dumpFields) String() string {
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + f[k]
	}
	return strings.Join(parts, " ")
}

func dumpEdgeObj(e graphlib.Edge) string {
	return e.V + ">" + e.W + ">" + e.Name
}

// dumpGraph prints the layout graph in the format of dumpGraph in
// tools/reference/gen-dagre.mjs.
func dumpGraph(g *graphlib.Graph) []string {
	lines := []string{}

	f := dumpFields{}
	if graph := graphOf(g); graph != nil {
		f.num("ranksep", graph.ranksep)
		f.num("edgesep", graph.edgesep)
		f.num("nodesep", graph.nodesep)
		f.str("rankdir", graph.rankdir)
		f.str("acyclicer", graph.acyclicer)
		f.str("ranker", graph.ranker)
		f.str("align", graph.align)
		f.str("nestingRoot", graph.nestingRoot)
		f.num("nodeRankFactor", graph.nodeRankFactor)
		if graph.dummyChains != nil {
			f["dummyChains"] = "[" + strings.Join(graph.dummyChains, ",") + "]"
		}
		f.num("width", graph.width)
		f.num("height", graph.height)
	}
	lines = append(lines, "G "+f.String())

	for _, v := range g.Nodes() {
		f := dumpFields{}
		if n := nodeOf(g, v); n != nil {
			f.num("width", n.width)
			f.num("height", n.height)
			f.num("x", n.x)
			f.num("y", n.y)
			if n.hasRank {
				f.num("rank", n.rank)
			}
			f.num("order", n.order)
			f.str("dummy", n.dummy)
			f.str("borderTop", n.borderTop)
			f.str("borderBottom", n.borderBottom)
			if n.borderLeft != nil {
				f["borderLeft"] = "[" + strings.Join(n.borderLeft, ",") + "]"
			}
			if n.borderRight != nil {
				f["borderRight"] = "[" + strings.Join(n.borderRight, ",") + "]"
			}
			if n.hasMinRank {
				f.num("minRank", n.minRank)
				f.num("maxRank", n.maxRank)
			}
			if len(n.selfEdges) > 0 {
				f["selfEdges"] = js.Str(float64(len(n.selfEdges)))
			}
			if n.e != (graphlib.Edge{}) {
				f["e"] = dumpEdgeObj(n.e)
			}
			if n.edgeObj != (graphlib.Edge{}) {
				f["edgeObj"] = dumpEdgeObj(n.edgeObj)
			}
			if n.hasLabelpos {
				f.str("labelpos", n.labelpos)
			}
			f.str("borderType", n.borderType)
		}
		parent := "null"
		if p, ok := g.Parent(v); ok {
			parent = js.StringifyJSON(p)
		}
		lines = append(lines, "N "+js.StringifyJSON(v)+" parent="+parent+" "+f.String())
	}

	for _, e := range g.Edges() {
		f := dumpFields{}
		if l := edgeOf(g, e); l != nil {
			f.num("minlen", l.minlen)
			f.num("weight", l.weight)
			f.num("width", l.width)
			f.num("height", l.height)
			f.num("labeloffset", l.labeloffset)
			f.str("labelpos", l.labelpos)
			if l.hasXY {
				f.num("x", l.x)
				f.num("y", l.y)
			}
			if l.points != nil {
				pts := make([]string, len(l.points))
				for i, p := range l.points {
					pts[i] = fmtNum(p.X) + "," + fmtNum(p.Y)
				}
				f["points"] = "[" + strings.Join(pts, " ") + "]"
			}
			f.num("labelRank", l.labelRank)
			f.flag("reversed", l.reversed)
			f.str("forwardName", l.forwardName)
			f.flag("nestingEdge", l.nestingEdge)
		}
		name := "null"
		if e.Name != "" {
			name = js.StringifyJSON(e.Name)
		}
		lines = append(lines, "E "+js.StringifyJSON(e.V)+" "+js.StringifyJSON(e.W)+" "+name+" "+f.String())
	}
	return lines
}

// TestDebugCase finds the first pipeline stage where the Go layout graph
// differs from the JavaScript layout graph, for one golden case:
//
//	DAGRE_DEBUG=<case name> go test ./internal/dagre -run TestDebugCase -v
//
// It runs tools/reference/gen-dagre.mjs --debug with node.
func TestDebugCase(t *testing.T) {
	name := os.Getenv("DAGRE_DEBUG")
	if name == "" {
		t.Skip("set DAGRE_DEBUG to a golden case name")
	}
	golden := loadGolden(t)
	var c *goldenCase
	for i := range golden.Cases {
		if golden.Cases[i].Name == name {
			c = &golden.Cases[i]
		}
	}
	if c == nil {
		t.Fatalf("no golden case %q", name)
	}

	cmd := exec.Command("node", "gen-dagre.mjs", "--debug", name)
	cmd.Dir = "../../tools/reference"
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var jsDump struct {
		Err    *string
		Stages []struct {
			Stage string
			Lines []string
		}
	}
	if err := json.Unmarshal(out, &jsDump); err != nil {
		t.Fatal(err)
	}

	type stage struct {
		name  string
		lines []string
	}
	var goStages []stage
	g, err := c.build()
	if err != nil {
		t.Fatal(err)
	}
	l := &layout{state: &State{IDCounter: c.IDStart, Sort: goldenSort(t, golden)}}
	l.debug = func(name string, g *graphlib.Graph) {
		goStages = append(goStages, stage{name, dumpGraph(g)})
	}
	goErr := l.run(g)
	t.Logf("go error: %v", goErr)
	if jsDump.Err != nil {
		t.Logf("js error: %s", *jsDump.Err)
	}

	for i, js := range jsDump.Stages {
		if i >= len(goStages) {
			t.Fatalf("stage %d %s: missing in Go", i, js.Stage)
		}
		gs := goStages[i]
		if gs.name != js.Stage {
			t.Fatalf("stage %d: Go has %s, JavaScript has %s", i, gs.name, js.Stage)
		}
		diffs := 0
		for j := 0; j < len(js.Lines) || j < len(gs.lines); j++ {
			var a, b string
			if j < len(js.Lines) {
				a = js.Lines[j]
			}
			if j < len(gs.lines) {
				b = gs.lines[j]
			}
			if a != b {
				diffs++
				if diffs <= 12 {
					t.Errorf("stage %s line %d:\n  js: %s\n  go: %s", js.Stage, j, a, b)
				}
			}
		}
		if diffs > 0 {
			t.Fatalf("stage %s: %d lines differ (first divergent stage)", js.Stage, diffs)
		}
	}
	if len(goStages) != len(jsDump.Stages) {
		t.Errorf("Go ran %d stages, JavaScript ran %d", len(goStages), len(jsDump.Stages))
	}
}
