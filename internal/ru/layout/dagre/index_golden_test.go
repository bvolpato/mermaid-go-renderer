package dagre

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file comes from tools/reference/gen-dagre-index.mjs. It holds
// the results of the functions that the dagre layout module of mermaid
// 12.2.0 exports and that need no DOM: prepareLayoutForDagre,
// getEdgesToRender and applyDagreLayoutResult, for cases that are built by
// hand and at random.

type indexGolden struct {
	Cases []indexCase
}

type indexCase struct {
	Kind  string
	Name  string
	Error *string

	// prepare
	Input     map[string]any
	Graph     any
	ClusterDb any `json:"clusterDb"`

	// edges and apply
	Rankdir        *string
	YOffset        float64 `json:"yOffset"`
	MergeSelfLoops bool    `json:"mergeSelfLoops"`
	Nodes          [][]any
	Edges          [][]any
	Want           any

	// apply
	Margin    *float64
	DataNodes []map[string]any `json:"dataNodes"`
}

// nodeFromJSON builds a node from the JSON of a case.
func nodeFromJSON(m map[string]any) *ru.Node {
	node := ru.NewNode(decStr(m, "id"))
	node.X = decNum(m, "x")
	node.Y = decNum(m, "y")
	node.Width = decNum(m, "width")
	node.Height = decNum(m, "height")
	node.Diff = decNum(m, "diff")
	_, node.IsGroup = m["isGroup"]
	_, node.ClusterNode = m["clusterNode"]
	if box, ok := m["labelBBox"].(map[string]any); ok {
		node.LabelBBox = &ru.Bounds{X: ru.Undef, Y: ru.Undef, Width: decNum(box, "width"), Height: decNum(box, "height")}
	}
	if _, ok := m["intersect"]; ok {
		node.Intersect = func(ru.Point) ru.Point { return ru.Point{} }
	}
	return node
}

// edgeLabelFromJSON builds an edge label from the JSON of a case: selfLoop
// is [id, order] and originalEdge is another edge label.
func edgeLabelFromJSON(m map[string]any) *Edge {
	label := &Edge{Edge: edgeFromJSON(m)}
	if selfLoop, ok := m["selfLoop"].([]any); ok {
		id, _ := selfLoop[0].(string)
		label.SelfLoop = &SelfLoop{ID: id, Order: int(selfLoop[1].(float64))}
	}
	if original, ok := m["originalEdge"].(map[string]any); ok {
		label.OriginalEdge = edgeFromJSON(original)
	}
	return label
}

// buildGraph builds the graph of an edges or apply case.
func (c *indexCase) buildGraph() (*graphlib.Graph, error) {
	rankdir := ""
	if c.Rankdir != nil {
		rankdir = *c.Rankdir
	}
	g := newGraph(newGraphLabel(rankdir, 50.0, 50.0))
	for _, item := range c.Nodes {
		id := item[0].(string)
		if value, ok := item[2].(map[string]any); ok {
			g.SetNode(id, &Node{Node: nodeFromJSON(value)})
		} else {
			g.SetNode(id)
		}
		if parent, ok := item[1].(string); ok {
			if err := g.SetParent(id, parent); err != nil {
				return nil, err
			}
		}
	}
	for _, item := range c.Edges {
		if err := g.SetEdge(item[0].(string), item[1].(string), edgeLabelFromJSON(item[3].(map[string]any)), item[2].(string)); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// fullNode is the node as the generator prints it: the position, the label
// box and whether the node has an intersect function.
func fullNode(n *ru.Node) []any {
	var box any
	if n.LabelBBox != nil {
		box = []any{encDyn(n.LabelBBox.Width), encDyn(n.LabelBBox.Height)}
	}
	return append(encPos(n), box, n.Intersect != nil)
}

// checkError compares the error of the Go function with the case.
func (c *indexCase) checkError(err error, problems *[]string) bool {
	switch {
	case c.Error != nil && err == nil:
		*problems = append(*problems, fmt.Sprintf("no error, upstream throws %q", *c.Error))
	case c.Error == nil && err != nil:
		*problems = append(*problems, fmt.Sprintf("error %q, upstream does not throw", err))
	}
	return c.Error == nil && err == nil
}

func (c *indexCase) runPrepare() []string {
	var problems []string
	data, config := inputData(c.Input)
	st := newTestState()
	ctx := &render.Context{ID: data.DiagramID, Config: config}
	st.ctx = ctx
	ctx.Value(stateKey, func() any { return st })
	Clear(ctx)
	prepared, err := PrepareLayoutForDagre(ctx, data)
	if c.checkError(err, &problems) {
		diffJSON("graph", projectGraph(prepared.Graph, true, false), c.Graph, &problems)
	}
	diffJSON("clusterDb", projectClusterDb(st), c.ClusterDb, &problems)
	return problems
}

func (c *indexCase) runEdges() []string {
	var problems []string
	g, err := c.buildGraph()
	if err != nil {
		return []string{"build: " + err.Error()}
	}
	edgesToRender, err := GetEdgesToRender(g, c.YOffset, c.MergeSelfLoops)
	if c.checkError(err, &problems) {
		// "d" is what dagre would read from the edge. It is left out here:
		// these edges are drawn and never go to dagre. A merged edge of
		// upstream has the properties width and height also when their
		// value is undefined, and the Go model cannot tell such a property
		// from one that is absent.
		got := []any{}
		for _, er := range edgesToRender {
			got = append(got, []any{without(projectEdge(er.Edge, false), "d"), er.Start, er.End})
		}
		want := []any{}
		for _, item := range c.Want.([]any) {
			entry := item.([]any)
			want = append(want, []any{without(entry[0], "d"), entry[1], entry[2]})
		}
		diffJSON("edgesToRender", got, want, &problems)
	}
	return problems
}

func (c *indexCase) runApply() []string {
	var problems []string
	g, err := c.buildGraph()
	if err != nil {
		return []string{"build: " + err.Error()}
	}
	data := &ru.LayoutData{Edges: []*ru.Edge{ru.NewEdge("old")}}
	for _, m := range c.DataNodes {
		data.Nodes = append(data.Nodes, nodeFromJSON(m))
	}
	measured := &MeasuredLayout{Graph: g, MergeSelfLoops: c.MergeSelfLoops}
	if c.Margin != nil {
		measured.SubGraphTitleTotalMargin = *c.Margin
	}
	_, err = ApplyDagreLayoutResult(data, measured)
	if !c.checkError(err, &problems) {
		return problems
	}
	dataNodes := []any{}
	for _, node := range data.Nodes {
		dataNodes = append(dataNodes, fullNode(node))
	}
	graphNodes := []any{}
	for _, v := range g.Nodes() {
		var full any
		if node := nodeOf(g, v); node != nil {
			full = fullNode(node.Node)
		}
		graphNodes = append(graphNodes, []any{v, full})
	}
	edgeList := []any{}
	for _, edge := range data.Edges {
		o := projectRuEdge(edge)
		o["d"] = ruEdgeD(edge)
		edgeList = append(edgeList, o)
	}
	want, _ := c.Want.(map[string]any)
	// The edges of the layout data are ru.Edge values, which have no
	// selfLoop and no originalEdge.
	wantEdges := []any{}
	for _, item := range want["edges"].([]any) {
		wantEdges = append(wantEdges, without(item, "selfLoop", "originalEdge"))
	}
	diffJSON("dataNodes", dataNodes, want["dataNodes"], &problems)
	diffJSON("graphNodes", graphNodes, want["graphNodes"], &problems)
	diffJSON("edges", edgeList, wantEdges, &problems)
	return problems
}

func TestIndexGolden(t *testing.T) {
	var golden indexGolden
	loadJSONGz(t, "testdata/index.json.gz", &golden)

	counts := map[string]int{}
	failed := map[string]int{}
	shown := 0
	for i := range golden.Cases {
		c := &golden.Cases[i]
		var problems []string
		switch c.Kind {
		case "prepare":
			problems = c.runPrepare()
		case "edges":
			problems = c.runEdges()
		case "apply":
			problems = c.runApply()
		default:
			t.Fatalf("%s: kind %q", c.Name, c.Kind)
		}
		counts[c.Kind]++
		if len(problems) > 0 {
			failed[c.Kind]++
			if shown++; shown <= 10 {
				t.Errorf("%s:\n  %s", c.Name, strings.Join(problems, "\n  "))
			}
		}
	}
	for _, kind := range []string{"prepare", "edges", "apply"} {
		t.Logf("%s: %d cases, %d differ", kind, counts[kind], failed[kind])
	}
}
