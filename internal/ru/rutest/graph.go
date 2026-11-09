package rutest

import (
	"encoding/json"
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// This file has the golden file format of tools/reference/gen-graph.mjs and
// helpers to replay its cases. The replay itself is in the test of
// internal/ru/graph, so that this package does not import the package under
// test.

// GraphNodeState is the state of a layout node after a case, as
// probes/graph.js records it. A number that is undefined or not finite is a
// string, see GoldenNum.
type GraphNodeState struct {
	ID           string     `json:"id"`
	X            any        `json:"x"`
	Y            any        `json:"y"`
	Width        any        `json:"width"`
	Height       any        `json:"height"`
	Diff         any        `json:"diff"`
	OffsetX      any        `json:"offsetX"`
	OffsetY      any        `json:"offsetY"`
	LabelBBox    *GoldenBox `json:"labelBBox"`
	HasIntersect bool       `json:"hasIntersect"`
}

// GraphEdgeState is the state of a layout edge after a case.
type GraphEdgeState struct {
	ID     string `json:"id"`
	Width  any    `json:"width"`
	Height any    `json:"height"`
	X      any    `json:"x"`
	Y      any    `json:"y"`
}

// GraphResult is what upstream produced for a case of probes/graph.js. The
// fields that are set depend on the kind of the case.
type GraphResult struct {
	Error       string `json:"error"`
	HTML        string `json:"html"`
	RandomCalls int    `json:"randomCalls"`

	// groups: the index of each group in g.root.
	Order []int `json:"order"`

	// measureGroup and measuredNode.
	Node               *GraphNodeState `json:"node"`
	ElementIsLastChild bool            `json:"elementIsLastChild"`
	Intersects         []any           `json:"intersects"`

	// createGraph and render.
	Nodes      []GraphNodeState `json:"nodes"`
	Edges      []GraphEdgeState `json:"edges"`
	EdgesAdded int              `json:"edgesAdded"`
	Graph      *struct {
		Directed   bool     `json:"directed"`
		Multigraph bool     `json:"multigraph"`
		Compound   bool     `json:"compound"`
		Nodes      []string `json:"nodes"`
		Labels     []struct {
			ID        string     `json:"id"`
			Width     any        `json:"width"`
			Height    any        `json:"height"`
			LabelBBox *GoldenBox `json:"labelBBox"`
			IsCopy    bool       `json:"isCopy"`
		} `json:"labels"`
		Edges      [][]*string `json:"edges"`
		EdgeLabels []struct {
			ID     string `json:"id"`
			Width  any    `json:"width"`
			Height any    `json:"height"`
		} `json:"edgeLabels"`
	} `json:"graph"`
	NodeElements []string  `json:"nodeElements"`
	GroupClasses []*string `json:"groupClasses"`
	// Sequential says whether upstream gives the same result when it
	// handles the nodes one after another, without Promise.all.
	Sequential *struct {
		SameHTML        bool   `json:"sameHtml"`
		SameRandomCalls bool   `json:"sameRandomCalls"`
		SameSizes       bool   `json:"sameSizes"`
		Error           string `json:"error"`
	} `json:"sequential"`
	Log []string `json:"log"`

	// terminal: [x, y]. margins: [top, bottom, total].
	Point   []any `json:"point"`
	Margins []any `json:"margins"`
}

// GraphCase is one case of tools/reference/probes/graph.js with the result
// that upstream produced. See the probe for the kinds and their inputs.
type GraphCase struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Type string `json:"type"`
	// Config and CSS are indexes into the lists of the golden file.
	Config *int `json:"config"`
	CSS    *int `json:"css"`

	Options      json.RawMessage `json:"options"`
	Node         json.RawMessage `json:"node"`
	MeasureWidth any             `json:"measureWidth"`
	Dir          string          `json:"dir"`
	Data         json.RawMessage `json:"data"`
	Detached     bool            `json:"detached"`
	Edge         json.RawMessage `json:"edge"`
	Key          string          `json:"key"`
	Path         json.RawMessage `json:"path"`
	MarginConfig json.RawMessage `json:"marginConfig"`
	Def          json.RawMessage `json:"def"`

	Result GraphResult `json:"result"`
}

// GraphGolden is the golden file of tools/reference/gen-graph.mjs.
type GraphGolden struct {
	// BaseConfig is an effective config of upstream. Configs are patches
	// of it, see PatchedConfig.
	BaseConfig json.RawMessage   `json:"baseConfig"`
	Configs    []json.RawMessage `json:"configs"`
	CSS        []string          `json:"css"`
	Cases      []GraphCase       `json:"cases"`
}

// LayoutDataFromJS builds layout data from a JavaScript object { nodes,
// edges, type, direction, diagramId, markers, layoutAlgorithm, nodeSpacing,
// rankSpacing } with nodes and edges as in types.ts.
func LayoutDataFromJS(o *js.Obj[any], config *js.Obj[any]) *ru.LayoutData {
	data := &ru.LayoutData{Config: config, NodeSpacing: ru.Undef, RankSpacing: ru.Undef}
	if o == nil {
		return data
	}
	str := func(key string) string {
		if v := o.Get(key); !js.IsNullish(v) {
			return js.ToString(v)
		}
		return ""
	}
	if arr, ok := o.Get("nodes").([]any); ok {
		for _, n := range arr {
			data.Nodes = append(data.Nodes, LayoutNodeFromJS(js.AsObj(n)))
		}
	}
	if arr, ok := o.Get("edges").([]any); ok {
		for _, e := range arr {
			data.Edges = append(data.Edges, EdgeFromJS(js.AsObj(e)))
		}
	}
	data.Type = str("type")
	data.Direction = str("direction")
	data.DiagramID = str("diagramId")
	data.LayoutAlgorithm = str("layoutAlgorithm")
	if v := o.Get("nodeSpacing"); !js.IsNullish(v) {
		data.NodeSpacing = js.ToNumber(v)
	}
	if v := o.Get("rankSpacing"); !js.IsNullish(v) {
		data.RankSpacing = js.ToNumber(v)
	}
	if arr, ok := o.Get("markers").([]any); ok {
		data.Markers = []string{}
		for _, m := range arr {
			data.Markers = append(data.Markers, js.ToString(m))
		}
	}
	return data
}

// CompareNodeState compares a layout node with the state that upstream
// recorded. It returns a description of the first difference, or "".
func CompareNodeState(check *NumCheck, node *ru.Node, want GraphNodeState) string {
	what := "node " + want.ID
	if node.ID != want.ID {
		return fmt.Sprintf("%s: got the node %q", what, node.ID)
	}
	for _, f := range []struct {
		name string
		got  float64
		want any
	}{
		{"x", node.X, want.X},
		{"y", node.Y, want.Y},
		{"width", node.Width, want.Width},
		{"height", node.Height, want.Height},
		{"diff", node.Diff, want.Diff},
		{"offsetX", node.OffsetX, want.OffsetX},
		{"offsetY", node.OffsetY, want.OffsetY},
	} {
		if d := check.Num(what+"."+f.name, f.got, f.want); d != "" {
			return d
		}
	}
	if (node.LabelBBox != nil) != (want.LabelBBox != nil) {
		return fmt.Sprintf("%s.labelBBox present: got %v, want %v", what, node.LabelBBox != nil, want.LabelBBox != nil)
	}
	if node.LabelBBox != nil {
		if d := check.Num(what+".labelBBox.width", node.LabelBBox.Width, want.LabelBBox.Width); d != "" {
			return d
		}
		if d := check.Num(what+".labelBBox.height", node.LabelBBox.Height, want.LabelBBox.Height); d != "" {
			return d
		}
	}
	if (node.Intersect != nil) != want.HasIntersect {
		return fmt.Sprintf("%s.intersect present: got %v, want %v", what, node.Intersect != nil, want.HasIntersect)
	}
	return ""
}

// CompareEdgeState compares a layout edge with the state that upstream
// recorded. It returns a description of the first difference, or "".
func CompareEdgeState(check *NumCheck, edge *ru.Edge, want GraphEdgeState) string {
	what := "edge " + want.ID
	if edge.ID != want.ID {
		return fmt.Sprintf("%s: got the edge %q", what, edge.ID)
	}
	for _, f := range []struct {
		name string
		got  float64
		want any
	}{
		{"width", edge.Width, want.Width},
		{"height", edge.Height, want.Height},
		{"x", edge.X, want.X},
		{"y", edge.Y, want.Y},
	} {
		if d := check.Num(what+"."+f.name, f.got, f.want); d != "" {
			return d
		}
	}
	return ""
}

// CompareIntersects calls node.Intersect for the rays of the probes and
// compares the points with upstream.
func CompareIntersects(check *NumCheck, node *ru.Node, want []any) string {
	for i, w := range want {
		pair, ok := w.([]any)
		if !ok || len(pair) != 2 {
			continue
		}
		if node.Intersect == nil {
			return "node.intersect is not set"
		}
		p := node.Intersect(ru.Point{X: shapeRays[i][0], Y: shapeRays[i][1]})
		if d := check.Num(fmt.Sprintf("intersect ray %d x", i), p.X, pair[0]); d != "" {
			return d
		}
		if d := check.Num(fmt.Sprintf("intersect ray %d y", i), p.Y, pair[1]); d != "" {
			return d
		}
	}
	return ""
}
