package dagre

import (
	"fmt"
	"strings"
	"testing"

	dagrelib "github.com/bvolpato/mermaid-go-renderer/internal/dagre"
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file comes from tools/reference/gen-dagre-graphlib.mjs. It
// holds the results of the functions of mermaid-graphlib.js of mermaid
// 12.2.0 for random compound graphs.

type graphlibGolden struct {
	Cases []graphlibCase
}

type graphlibCase struct {
	Name        string
	Rankdir     string
	Ops         [][]any
	Validate    bool
	Sorted      []any
	NonCluster  [][]any
	Descendants []any
	Parents     []any
	Adjust      struct {
		Graph       any
		Error       *string
		ClusterDb   any `json:"clusterDb"`
		Descendants any
		Parents     any
	}
	Extract struct {
		Graph any
		Error *string
	}
}

// build replays the setNode, setParent and setEdge calls of the case.
func (c *graphlibCase) build() (*graphlib.Graph, error) {
	g := newGraph(newGraphLabel(c.Rankdir, 50.0, 50.0))
	for _, op := range c.Ops {
		switch op[0] {
		case "n":
			label := op[2].(map[string]any)
			node := ru.NewNode(decStr(label, "id"))
			_, node.IsGroup = label["isGroup"]
			node.Dir = decStr(label, "dir")
			g.SetNode(op[1].(string), &Node{Node: node})
		case "p":
			if err := g.SetParent(op[1].(string), op[2].(string)); err != nil {
				return nil, err
			}
		case "e":
			// A null label is undefined: the edge has no label.
			var edge any
			if label, ok := op[4].(map[string]any); ok {
				edge = &Edge{Edge: ru.NewEdge(decStr(label, "id"))}
			}
			var err error
			if name, ok := op[3].(string); ok {
				err = g.SetEdge(op[1].(string), op[2].(string), edge, name)
			} else {
				err = g.SetEdge(op[1].(string), op[2].(string), edge)
			}
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("bad op %v", op)
		}
	}
	return g, nil
}

func newTestState() *state {
	return &state{
		clusterDb:   js.NewMap[string, *ClusterEntry](),
		descendants: js.NewMap[string, []string](),
		parents:     js.NewMap[string, string](),
		dagre:       &dagrelib.State{},
	}
}

func strList(list []string) []any {
	out := []any{}
	for _, s := range list {
		out = append(out, s)
	}
	return out
}

// idOrNull is an id whose empty value stands for undefined.
func idOrNull(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func parentEntries(st *state) []any {
	out := []any{}
	for _, k := range st.parents.Keys() {
		out = append(out, []any{k, st.parents.Get(k)})
	}
	return out
}

func (c *graphlibCase) run() []string {
	var problems []string
	check := func(what string, got, want any) {
		diffJSON(what, got, want, &problems)
	}
	// errorOrGraph compares the result of a function that can throw.
	errorOrGraph := func(what string, err error, g *graphlib.Graph, wantGraph any, wantErr *string) {
		switch {
		case wantErr != nil && err == nil:
			problems = append(problems, fmt.Sprintf("%s: no error, want %q", what, *wantErr))
		case wantErr == nil && err != nil:
			problems = append(problems, fmt.Sprintf("%s: error %q, want none", what, err))
		case wantErr == nil:
			check(what+".graph", projectGraph(g, true, false), wantGraph)
		}
	}

	st := newTestState()
	g, err := c.build()
	if err != nil {
		return []string{"build: " + err.Error()}
	}
	nodes := g.Nodes()
	if got := Validate(g); got != c.Validate {
		problems = append(problems, fmt.Sprintf("validate: got %v, want %v", got, c.Validate))
	}
	check("sorted", strList(SortNodesByHierarchy(g)), c.Sorted)
	if len(c.NonCluster) != len(nodes) {
		return append(problems, fmt.Sprintf("nonCluster: %d nodes, want %d", len(nodes), len(c.NonCluster)))
	}
	for i, want := range c.NonCluster {
		id, other := want[0].(string), want[1].(string)
		if id != nodes[i] {
			return append(problems, fmt.Sprintf("node %d: got %q, want %q", i, nodes[i], id))
		}
		got := []any{id, other}
		for _, clusterID := range []string{"", id, other} {
			res, err := FindNonClusterChild(id, g, clusterID)
			if err != nil {
				return append(problems, fmt.Sprintf("findNonClusterChild(%q, %q): %v", id, clusterID, err))
			}
			got = append(got, idOrNull(res))
		}
		check(fmt.Sprintf("findNonClusterChild(%q)", id), got, []any(want))
	}
	st.clear()
	descendants := []any{}
	for _, id := range nodes {
		descendants = append(descendants, []any{id, strList(st.extractDescendants(id, g))})
	}
	check("descendants", descendants, c.Descendants)
	check("parents", parentEntries(st), c.Parents)

	st.clear()
	if g, err = c.build(); err != nil {
		return append(problems, "build: "+err.Error())
	}
	err = st.adjustClustersAndEdges(g, 0)
	errorOrGraph("adjust", err, g, c.Adjust.Graph, c.Adjust.Error)
	check("adjust.clusterDb", projectClusterDb(st), c.Adjust.ClusterDb)
	check("adjust.descendants", entriesOf(st.descendants), c.Adjust.Descendants)
	check("adjust.parents", parentEntries(st), c.Adjust.Parents)

	// extractor on a second copy, with the clusterDb of the first copy.
	if g, err = c.build(); err != nil {
		return append(problems, "build: "+err.Error())
	}
	err = st.extractor(g, 0)
	errorOrGraph("extract", err, g, c.Extract.Graph, c.Extract.Error)
	return problems
}

func TestMermaidGraphlibGolden(t *testing.T) {
	var golden graphlibGolden
	loadJSONGz(t, "testdata/graphlib.json.gz", &golden)

	failed, extracted, external, errors := 0, 0, 0, 0
	for i := range golden.Cases {
		c := &golden.Cases[i]
		if c.Adjust.Error != nil {
			errors++
		}
		text := show2(c.Adjust.Graph) + show2(c.Adjust.ClusterDb)
		if strings.Contains(text, `"clusterNode":true`) {
			extracted++
		}
		if strings.Contains(text, `"externalConnections":true`) {
			external++
		}
		if problems := c.run(); len(problems) > 0 {
			failed++
			if failed <= 10 {
				t.Errorf("%s:\n  %s", c.Name, strings.Join(problems, "\n  "))
			}
		}
	}
	t.Logf("%d cases (%d with an extracted cluster, %d with external connections, %d where upstream throws): %d differ", len(golden.Cases), extracted, external, errors, failed)
}

// show2 prints a JSON value without the length limit of show.
func show2(v any) string {
	return js.StringifyJSON(toDynamic(v))
}

// toDynamic converts a value of encoding/json to a dynamic value.
func toDynamic(v any) any {
	switch x := v.(type) {
	case map[string]any:
		o := js.NewObj[any]()
		for k, e := range x {
			o.Set(k, toDynamic(e))
		}
		return o
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = toDynamic(e)
		}
		return out
	case nil:
		return js.Null
	}
	return v
}
