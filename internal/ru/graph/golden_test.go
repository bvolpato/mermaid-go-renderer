package graph_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"

	// The shapes of the nodes of the cases.
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

// TestGraphGolden replays the cases of tools/reference/gen-graph.mjs: the
// real functions of createGraph.ts and layout-algorithms/common/index.ts of
// mermaid in Chromium.
//
// On a machine without the system fonts of the macOS reference machine, a
// case that measures text fails the test only for an error, and the test is
// marked as skipped after the last case. See rutest.SkipComparison.
func TestGraphGolden(t *testing.T) {
	var g rutest.GraphGolden
	rutest.LoadGolden(t, "testdata/graph.json.gz", &g)
	type tally map[string]int
	counts := map[string]tally{}
	var kinds []string
	skipped := 0
	for _, c := range g.Cases {
		status, detail := runGraphCase(&g, c)
		if counts[c.Kind] == nil {
			counts[c.Kind] = tally{}
			kinds = append(kinds, c.Kind)
		}
		counts[c.Kind][status]++
		// measureGroup measures a label and removes it, so the markup of
		// the result has no text.
		skip := rutest.SkipComparison(c.Result.HTML) || c.Kind == "measureGroup" && !fonttest.IsMacOSReference()
		if skip {
			skipped++
		}
		switch {
		case status == "error", status == "different" && !skip:
			t.Errorf("%s: %s\n    %s", c.Name, status, detail)
		case status == "not-ported", status == "order":
			t.Logf("%s: %s: %s", c.Name, status, detail)
		}
	}
	for _, kind := range kinds {
		n := counts[kind]
		total := 0
		for _, v := range n {
			total += v
		}
		t.Logf("%s: %d cases: exact=%d equivalent=%d order=%d different=%d error=%d not-ported=%d",
			kind, total, n["exact"], n["equivalent"], n["order"], n["different"], n["error"], n["not-ported"])
	}
	rutest.RequireFonts(t, skipped)
}

// orderOnly marks a difference that is only the order in which the nodes
// of a Promise.all finished. The order comes from the awaits of the shape
// handlers (internal/ru and its shape packages), not from this package, and
// nothing reads it: it is the insertion order of the graph that
// createGraphWithElements returns. Such a case has the status "order". It
// is counted and logged and does not fail the test.
const orderOnly = "order only: "

// notPorted reports whether err says that a part of the renderer that the
// case needs is not ported yet.
func notPorted(err error) bool {
	var np *ru.NotPortedError
	return errors.As(err, &np)
}

func runGraphCase(g *rutest.GraphGolden, c rutest.GraphCase) (status, detail string) {
	defer func() {
		if p := recover(); p != nil {
			status, detail = "error", fmt.Sprintf("panic: %v", p)
		}
	}()
	want := c.Result
	check := &rutest.NumCheck{Exact: true}
	// finish gives the verdict of a case from the error of the Go function
	// and the first difference that the comparison found.
	finish := func(err error, diff string) (string, string) {
		if want.Error != "" {
			if err != nil {
				return "exact", ""
			}
			return "different", "upstream fails: " + want.Error
		}
		if err != nil {
			if notPorted(err) {
				return "not-ported", err.Error()
			}
			return "error", err.Error()
		}
		if strings.HasPrefix(diff, orderOnly) {
			return "order", strings.TrimPrefix(diff, orderOnly)
		}
		if diff != "" {
			return "different", diff
		}
		if check.Exact {
			return "exact", ""
		}
		return "equivalent", ""
	}
	parse := func(raw json.RawMessage) any {
		if len(raw) == 0 {
			return nil
		}
		v, err := js.ParseJSON(raw)
		if err != nil {
			panic(fmt.Sprintf("bad JSON in golden: %v", err))
		}
		return v
	}

	switch c.Kind {
	case "terminal":
		edge := rutest.EdgeFromJS(js.AsObj(parse(c.Edge)))
		// EdgeFromJS does not read terminalLabelCenters.
		if centers := js.AsObj(js.Path(parse(c.Edge), "terminalLabelCenters")); centers != nil {
			edge.TerminalLabelCenters = map[string]ru.Point{}
			centers.Each(func(k string, v any) {
				edge.TerminalLabelCenters[k] = ru.Point{X: js.ToNumber(js.Path(v, "x")), Y: js.ToNumber(js.Path(v, "y"))}
			})
		}
		var path []ru.Point
		if arr, ok := parse(c.Path).([]any); ok {
			path = []ru.Point{}
			for _, p := range arr {
				path = append(path, ru.Point{X: js.ToNumber(js.Path(p, "x")), Y: js.ToNumber(js.Path(p, "y"))})
			}
		}
		p, err := graph.TerminalLabelTranslate(edge, c.Key, path)
		if err != nil || want.Error != "" {
			return finish(err, "")
		}
		// Every number must be bit-exact.
		for i, got := range []float64{p.X, p.Y} {
			w := rutest.GoldenNum(want.Point[i])
			if math.Float64bits(got) != math.Float64bits(w) && !(got != got && w != w) {
				return "different", fmt.Sprintf("point[%d]: got %v, want %v", i, got, want.Point[i])
			}
		}
		return "exact", ""

	case "margins":
		m := graph.GetSubGraphTitleMargins(js.AsObj(parse(c.MarginConfig)))
		for i, got := range []float64{m.SubGraphTitleTopMargin, m.SubGraphTitleBottomMargin, m.SubGraphTitleTotalMargin} {
			if w := rutest.GoldenNum(want.Margins[i]); math.Float64bits(got) != math.Float64bits(w) {
				return "different", fmt.Sprintf("margins[%d]: got %v, want %v", i, got, want.Margins[i])
			}
		}
		return "exact", ""
	}

	if c.Config == nil || c.CSS == nil {
		return "error", "the golden case has no config"
	}
	cfg, err := rutest.PatchedConfig(g.BaseConfig, g.Configs[*c.Config])
	if err != nil {
		return "error", err.Error()
	}
	ctx := rutest.NewProbeContext(cfg, g.CSS[*c.CSS], c.Type)
	root := ctx.SVG.Append("g")
	markup := func(n *dom.Node) string {
		status, detail := rutest.CompareMarkup(n.InnerHTML(), want.HTML)
		if status == "different" {
			return detail
		}
		if status != "exact" {
			check.Exact = false
		}
		return ""
	}
	random := func() string {
		if next, w := ctx.Random(), rutest.RandomAfter(want.RandomCalls); next != w {
			return fmt.Sprintf("the number of random draws is not %d", want.RandomCalls)
		}
		return ""
	}
	first := func(diffs ...string) string {
		for _, d := range diffs {
			if d != "" {
				return d
			}
		}
		return ""
	}

	switch c.Kind {
	case "groups":
		var groups *graph.LayoutElementGroups
		if options := js.AsObj(parse(c.Options)); options == nil {
			groups = graph.CreateLayoutElementGroups(root)
		} else {
			class, _ := options.Get("edgePathsClass").(string)
			groups = graph.CreateLayoutElementGroups(root, graph.CreateLayoutElementGroupsOptions{EdgePathsClass: class})
		}
		children := groups.RootGroups.Children()
		order := []int{
			slices.Index(children, groups.Clusters), slices.Index(children, groups.EdgePaths),
			slices.Index(children, groups.EdgeLabels), slices.Index(children, groups.Nodes),
		}
		diff := markup(root)
		if diff == "" && !slices.Equal(order, want.Order) {
			diff = fmt.Sprintf("order of the groups: got %v, want %v", order, want.Order)
		}
		return finish(nil, diff)

	case "measureGroup":
		nodes := root.Append("g").Attr("class", "nodes")
		node := rutest.LayoutNodeFromJS(js.AsObj(parse(c.Node)))
		switch w := c.MeasureWidth.(type) {
		case nil:
			err = graph.MeasureGroupLabel(ctx, nodes, node)
		case string:
			err = graph.MeasureGroupLabel(ctx, nodes, node, rutest.GoldenNum(w))
		case float64:
			err = graph.MeasureGroupLabel(ctx, nodes, node, w)
		}
		if err != nil || want.Error != "" {
			return finish(err, "")
		}
		return finish(nil, first(markup(nodes), random(), rutest.CompareNodeState(check, node, *want.Node)))

	case "measuredNode":
		nodes := root.Append("g").Attr("class", "nodes")
		node := rutest.LayoutNodeFromJS(js.AsObj(parse(c.Node)))
		el, err := graph.InsertMeasuredNode(ctx, nodes, node, &ru.ShapeRenderOptions{Config: cfg, Dir: c.Dir, Padding: ru.Undef})
		if err != nil || want.Error != "" {
			return finish(err, "")
		}
		diff := first(markup(nodes), random(), rutest.CompareNodeState(check, node, *want.Node))
		if diff == "" && (el == lastElementChild(nodes)) != want.ElementIsLastChild {
			diff = "the result is not the element of the node"
		}
		if diff == "" {
			node.X, node.Y = 0, 0
			diff = rutest.CompareIntersects(check, node, want.Intersects)
		}
		return finish(nil, diff)

	case "createGraph":
		data := rutest.LayoutDataFromJS(js.AsObj(parse(c.Data)), cfg)
		edgeCount := len(data.Edges)
		var opts graph.CreateGraphOptions
		opts.UnwrapGroupLabels = js.Truthy(js.Path(parse(c.Options), "unwrapGroupLabels"))
		element := root
		if c.Detached {
			element = nil
		}
		result, err := graph.CreateGraphWithElements(ctx, element, data, opts)
		if err != nil || want.Error != "" {
			return finish(err, "")
		}
		diff := first(markup(root), random())
		for i := 0; diff == "" && i < len(want.Nodes); i++ {
			diff = rutest.CompareNodeState(check, data.Nodes[i], want.Nodes[i])
		}
		for i := 0; diff == "" && i < len(want.Edges); i++ {
			diff = rutest.CompareEdgeState(check, data.Edges[i], want.Edges[i])
		}
		if diff == "" && len(data.Edges)-edgeCount != want.EdgesAdded {
			diff = fmt.Sprintf("edges added to the layout data: got %d, want %d", len(data.Edges)-edgeCount, want.EdgesAdded)
		}
		if diff == "" {
			diff = compareGraph(check, result, data, &want)
		}
		return finish(nil, diff)

	case "render":
		data := rutest.LayoutDataFromJS(js.AsObj(parse(c.Data)), cfg)
		var flags defFlags
		if len(c.Def) > 0 {
			if err := json.Unmarshal(c.Def, &flags); err != nil {
				return "error", fmt.Sprintf("bad def in golden: %v", err)
			}
		}
		var log []string
		layout := graph.CreateCommonLayoutRenderer(makeDefinition(&flags, &log))
		err := layout(ctx, data, ctx.SVG, flags.Algorithm)
		if err != nil || want.Error != "" {
			return finish(err, "")
		}
		diff := first(markup(root), random())
		if diff == "" && !slices.Equal(log, want.Log) {
			diff = fmt.Sprintf("phases: got %v, want %v", log, want.Log)
		}
		for i := 0; diff == "" && i < len(want.Nodes); i++ {
			diff = rutest.CompareNodeState(check, data.Nodes[i], want.Nodes[i])
		}
		for i := 0; diff == "" && i < len(want.Edges); i++ {
			diff = rutest.CompareEdgeState(check, data.Edges[i], want.Edges[i])
		}
		return finish(nil, diff)
	}
	return "error", "unknown case kind " + c.Kind
}

func lastElementChild(n *dom.Node) *dom.Node {
	for c := n.LastChild; c != nil; c = c.Prev {
		if c.Type == dom.ElementNode {
			return c
		}
	}
	return nil
}

// compareGraph compares the result of CreateGraphWithElements with the
// graph, the groups and the node elements of upstream.
func compareGraph(check *rutest.NumCheck, result *graph.GraphWithElements, data *ru.LayoutData, want *rutest.GraphResult) string {
	gr, w := result.Graph, want.Graph
	if gr.IsDirected() != w.Directed || gr.IsMultigraph() != w.Multigraph || gr.IsCompound() != w.Compound {
		return "the options of the graph differ"
	}
	// Upstream adds a node to the graph when its measurement finishes, so
	// the order of the nodes of the graph follows the Promise.all. A
	// difference in the order alone is reported after everything else.
	order := ""
	if !slices.Equal(gr.Nodes(), w.Nodes) {
		gotNodes, wantNodes := slices.Clone(gr.Nodes()), slices.Clone(w.Nodes)
		slices.Sort(gotNodes)
		slices.Sort(wantNodes)
		if !slices.Equal(gotNodes, wantNodes) {
			return fmt.Sprintf("nodes of the graph: got %v, want %v", gr.Nodes(), w.Nodes)
		}
		order = fmt.Sprintf("%snodes of the graph: got %v, want %v", orderOnly, gr.Nodes(), w.Nodes)
	}
	for _, wl := range w.Labels {
		label, ok := gr.Node(wl.ID).(*ru.Node)
		if !ok {
			return fmt.Sprintf("the label of the graph node %s is not a node", wl.ID)
		}
		if d := check.Num("graph node "+wl.ID+".width", label.Width, wl.Width); d != "" {
			return d
		}
		if d := check.Num("graph node "+wl.ID+".height", label.Height, wl.Height); d != "" {
			return d
		}
		if (label.LabelBBox != nil) != (wl.LabelBBox != nil) {
			return fmt.Sprintf("graph node %s: labelBBox present: got %v", wl.ID, label.LabelBBox != nil)
		}
		isCopy := true
		for _, n := range data.Nodes {
			if n == label {
				isCopy = false
			}
		}
		if isCopy != wl.IsCopy {
			return fmt.Sprintf("graph node %s is a copy: got %v, want %v", wl.ID, isCopy, wl.IsCopy)
		}
	}
	var gotEdges, wantEdges []string
	for _, e := range gr.Edges() {
		gotEdges = append(gotEdges, e.V+"\x00"+e.W+"\x00"+e.Name)
	}
	for _, e := range w.Edges {
		name := ""
		if e[2] != nil {
			name = *e[2]
		}
		wantEdges = append(wantEdges, *e[0]+"\x00"+*e[1]+"\x00"+name)
	}
	if !slices.Equal(gotEdges, wantEdges) {
		return fmt.Sprintf("edges of the graph: got %q, want %q", gotEdges, wantEdges)
	}
	for i, e := range gr.Edges() {
		label, ok := gr.EdgeObj(e).(*ru.Edge)
		if !ok {
			return "the label of a graph edge is not an edge"
		}
		if d := check.Num("graph edge "+label.ID+".width", label.Width, w.EdgeLabels[i].Width); d != "" {
			return d
		}
		if d := check.Num("graph edge "+label.ID+".height", label.Height, w.EdgeLabels[i].Height); d != "" {
			return d
		}
	}
	if gotKeys := result.NodeElements.Keys(); !slices.Equal(gotKeys, want.NodeElements) && (len(gotKeys) > 0 || len(want.NodeElements) > 0) {
		sortedGot, sortedWant := slices.Clone(gotKeys), slices.Clone(want.NodeElements)
		slices.Sort(sortedGot)
		slices.Sort(sortedWant)
		if !slices.Equal(sortedGot, sortedWant) {
			return fmt.Sprintf("node elements: got %v, want %v", gotKeys, want.NodeElements)
		}
		if order == "" {
			order = fmt.Sprintf("%snode elements: got %v, want %v", orderOnly, gotKeys, want.NodeElements)
		}
	}
	groups := result.Groups
	for i, n := range []*dom.Node{groups.Clusters, groups.EdgePaths, groups.EdgeLabels, groups.Nodes, groups.RootGroups} {
		wantClass := want.GroupClasses[i]
		if (n != nil) != (wantClass != nil) {
			return fmt.Sprintf("group %d present: got %v, want %v", i, n != nil, wantClass != nil)
		}
		if n != nil && n.AttrOr("class", "") != *wantClass {
			return fmt.Sprintf("group %d: class %q, want %q", i, n.AttrOr("class", ""), *wantClass)
		}
	}
	return order
}

// toyCore is toyCore of tools/reference/probes/graph.js: the layout core of
// the render cases, with the same floating-point expressions. It returns
// the number of nodes.
func toyCore(data *ru.LayoutData) int {
	byID := map[string]*ru.Node{}
	for _, n := range data.Nodes {
		byID[n.ID] = n
	}
	x := 0.0
	i := 0
	for _, node := range data.Nodes {
		if node.IsGroup {
			continue
		}
		node.X = x + float64(node.Width/2)
		node.Y = 40 + float64(float64(i%3)*70) + float64(node.Height/2)
		x = x + node.Width + 30
		i++
	}
	inGroup := func(node, group *ru.Node) bool {
		p := node.ParentID
		seen := map[string]bool{}
		for p != "" && !seen[p] {
			if p == group.ID {
				return true
			}
			seen[p] = true
			if parent := byID[p]; parent != nil {
				p = parent.ParentID
			} else {
				p = ""
			}
		}
		return false
	}
	for _, group := range data.Nodes {
		if !group.IsGroup {
			continue
		}
		minX, minY := math.Inf(1), math.Inf(1)
		maxX, maxY := math.Inf(-1), math.Inf(-1)
		for _, node := range data.Nodes {
			if node.IsGroup || !inGroup(node, group) {
				continue
			}
			minX = js.Min(minX, node.X-float64(node.Width/2))
			minY = js.Min(minY, node.Y-float64(node.Height/2))
			maxX = js.Max(maxX, node.X+float64(node.Width/2))
			maxY = js.Max(maxY, node.Y+float64(node.Height/2))
		}
		if math.IsInf(minX, 1) {
			minX = x
			minY = 40
			maxX = x + 100
			maxY = 100
			x = x + 130
		}
		top := 0.0
		if group.LabelBBox != nil {
			top = ru.NN(group.LabelBBox.Height, 0)
		}
		group.Width = maxX - minX + 40
		group.Height = maxY - minY + 40 + top
		group.X = minX - 20 + float64(group.Width/2)
		group.Y = minY - 20 - top + float64(group.Height/2)
	}
	for _, edge := range data.Edges {
		s, t := byID[edge.Start], byID[edge.End]
		if edge.Start == "" || edge.End == "" || s == nil || t == nil {
			continue
		}
		mx := (s.X + t.X) / 2
		my := float64((s.Y+t.Y)/2) + 10
		edge.Points = []ru.Point{{X: s.X, Y: s.Y}, {X: mx, Y: my}, {X: t.X, Y: t.Y}}
		edge.X = mx
		edge.Y = my
	}
	return len(data.Nodes)
}

// defFlags are the flags of a render case, see makeDefinition of
// tools/reference/probes/graph.js.
type defFlags struct {
	Prepare      bool   `json:"prepare"`
	Measure      string `json:"measure"`
	Paint        string `json:"paint"`
	AfterPaint   bool   `json:"afterPaint"`
	Algorithm    string `json:"algorithm"`
	PaintOptions *struct {
		GetNodes      string          `json:"getNodes"`
		SkipNode      []string        `json:"skipNode"`
		NotCluster    []string        `json:"notCluster"`
		SkipEdge      []string        `json:"skipEdge"`
		SkipIntersect json.RawMessage `json:"skipIntersect"`
		GetEdgeNode   string          `json:"getEdgeNode"`
	} `json:"paintOptions"`
}

type prepared struct{ tag string }

// makeDefinition is makeDefinition of tools/reference/probes/graph.js.
func makeDefinition(flags *defFlags, log *[]string) graph.Definition {
	po := flags.PaintOptions
	tag := func(rc *graph.RenderContext) string {
		if p, ok := rc.PreparedLayout.(*prepared); ok {
			return p.tag
		}
		return "undefined"
	}
	var def graph.Definition
	if flags.Prepare {
		def.PrepareLayout = func(_ *ru.LayoutData, _ *graph.RenderContext) (any, error) {
			*log = append(*log, "prepare")
			return &prepared{tag: "prepared"}, nil
		}
	}
	switch flags.Measure {
	case "unwrap":
		def.MeasureLayout = func(d *ru.LayoutData, rc *graph.RenderContext) (graph.Measure, error) {
			*log = append(*log, "measure:"+tag(rc))
			measure, err := graph.DefaultMeasureLayout(d, rc, graph.CreateGraphOptions{UnwrapGroupLabels: true})
			if err != nil {
				return nil, err
			}
			return measure, nil
		}
	case "custom":
		def.MeasureLayout = func(d *ru.LayoutData, rc *graph.RenderContext) (graph.Measure, error) {
			*log = append(*log, "measure:"+tag(rc))
			groups := graph.CreateLayoutElementGroups(rc.Element, graph.CreateLayoutElementGroupsOptions{EdgePathsClass: "edgePaths"})
			for _, node := range d.Nodes {
				if !node.IsGroup || (po != nil && slices.Contains(po.NotCluster, node.ID)) {
					options := &ru.ShapeRenderOptions{Config: rc.Ctx.Config, Dir: d.Direction, Padding: ru.Undef}
					if _, err := graph.InsertMeasuredNode(rc.Ctx, groups.Nodes, node, options); err != nil {
						return nil, err
					}
				} else if err := graph.MeasureGroupLabel(rc.Ctx, groups.Nodes, node); err != nil {
					return nil, err
				}
			}
			return groups, nil
		}
	}
	def.RunLayoutCore = func(d *ru.LayoutData, rc *graph.RenderContext) (any, error) {
		algorithm := rc.Options.Algorithm
		if algorithm == "" {
			algorithm = "undefined"
		}
		*log = append(*log, "core:"+tag(rc)+":"+algorithm)
		return toyCore(d), nil
	}
	if flags.Paint == "custom" {
		def.PaintLayout = func(d *ru.LayoutData, pc *graph.PaintContext, core any) error {
			*log = append(*log, fmt.Sprintf("paint:%d", core.(int)))
			return graph.PaintLayoutData(d, pc, &graph.PaintOptions{SkipEdge: func(*ru.Edge) bool { return true }})
		}
	}
	if flags.AfterPaint {
		def.AfterPaint = func(_ *ru.LayoutData, pc *graph.PaintContext, core any) error {
			*log = append(*log, fmt.Sprintf("afterPaint:%d", core.(int)))
			pc.Measure.LayoutGroups().RootGroups.Append("g").Attr("class", "after-paint")
			return nil
		}
	}
	if po != nil {
		options := &graph.PaintOptions{}
		if po.GetNodes == "reverse" {
			options.GetNodes = func(d *ru.LayoutData, _ *graph.PaintContext) []*ru.Node {
				nodes := slices.Clone(d.Nodes)
				slices.Reverse(nodes)
				return nodes
			}
		}
		if po.SkipNode != nil {
			options.SkipNode = func(node *ru.Node, _ *graph.PaintContext) bool { return slices.Contains(po.SkipNode, node.ID) }
		}
		if po.NotCluster != nil {
			options.IsCluster = func(node *ru.Node, _ *graph.PaintContext) bool { return !slices.Contains(po.NotCluster, node.ID) }
		}
		if po.SkipEdge != nil {
			options.SkipEdge = func(edge *ru.Edge) bool { return slices.Contains(po.SkipEdge, edge.ID) }
		}
		if s := strings.TrimSpace(string(po.SkipIntersect)); s == "true" {
			options.SkipIntersect = true
		} else if strings.HasPrefix(s, "[") {
			var ids []string
			_ = json.Unmarshal(po.SkipIntersect, &ids)
			options.SkipIntersectFunc = func(edge *ru.Edge) bool { return slices.Contains(ids, edge.ID) }
		}
		if po.GetEdgeNode == "none" {
			options.GetEdgeNode = func(string, *ru.Edge, *graph.PaintContext) *ru.Node { return nil }
		}
		def.PaintOptions = options
	}
	return def
}

// TestCloneLayoutDataForDomMeasure checks the copy semantics of
// cloneLayoutDataForDomMeasure and copyMeasuredGraphOntoCanonical. The
// functions are not in the build of mermaid (only its test tooling imports
// them), so there is no golden file for them.
func TestCloneLayoutDataForDomMeasure(t *testing.T) {
	config := js.NewObj[any]()
	config.Set("theme", "default")
	node := ru.NewNode("a")
	node.Intersect = func(p ru.Point) ru.Point { return p }
	node.CalcIntersect = func(_ ru.Bounds, p ru.Point) ru.Point { return p }
	node.Width = 10
	edge := ru.NewEdge("e")
	edge.Points = []ru.Point{{X: 1, Y: 2}}
	layout := &ru.LayoutData{Nodes: []*ru.Node{node}, Edges: []*ru.Edge{edge}, Config: config, Type: "flowchart-v2", Direction: "LR"}

	clone := graph.CloneLayoutDataForDomMeasure(layout)
	if clone == layout || clone.Nodes[0] == node || clone.Edges[0] == edge || clone.Config == config {
		t.Fatal("the clone shares a node, an edge or the config with the original")
	}
	if clone.Nodes[0].Intersect != nil || clone.Nodes[0].CalcIntersect != nil {
		t.Error("the clone of a node has its intersect functions")
	}
	if node.Intersect == nil || node.CalcIntersect == nil {
		t.Error("the original node lost its intersect functions")
	}
	if clone.Nodes[0].ID != "a" || clone.Nodes[0].Width != 10 || clone.Edges[0].ID != "e" || clone.Type != "flowchart-v2" || clone.Direction != "LR" {
		t.Error("the clone lost a property")
	}
	if clone.Config.Get("theme") != "default" {
		t.Error("the clone of the config lost a property")
	}
	clone.Config.Set("theme", "dark")
	clone.Nodes[0].Width = 20
	if config.Get("theme") != "default" || node.Width != 10 {
		t.Error("a change of the clone changed the original")
	}

	canonical := &ru.LayoutData{Config: config}
	graph.CopyMeasuredGraphOntoCanonical(canonical, clone)
	if len(canonical.Nodes) != 1 || canonical.Nodes[0] != clone.Nodes[0] || canonical.Edges[0] != clone.Edges[0] || canonical.Config != config {
		t.Error("the measured nodes and edges are not on the canonical layout data")
	}
}
