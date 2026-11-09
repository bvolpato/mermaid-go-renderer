// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/layout.js (MIT).

package dagre

import (
	"fmt"
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// layout holds the state of one Layout call.
type layout struct {
	state *State
	// debug, when set, gets the layout graph after each pipeline stage.
	debug func(stage string, g *graphlib.Graph)
}

// Layout lays out the graph g. It is the JavaScript function layout(g).
//
// The graph label must implement GraphAttrs. Every node label must be nil
// or implement NodeAttrs, and every edge label must implement EdgeAttrs.
// Layout reads the input attributes from the labels and writes the result
// to them: X and Y for each node, Width and Height for each node that has
// children, Points and the label position for each edge, and Width and
// Height for the graph.
//
// The state carries the lodash uniqueId counter. A nil state starts the
// counter at 0.
//
// Layout returns an error where the JavaScript function throws. Examples
// are an edge that starts or ends at a node with children, an Align value
// that is not an alignment, a leaf node whose id is a property name of
// Object.prototype (such as "constructor") in a graph with more than one
// rank, and inputs that give NaN coordinates. After an error the content of
// the labels is not defined.
func Layout(g *graphlib.Graph, state *State) error {
	if state == nil {
		state = &State{}
	}
	l := &layout{state: state}
	return l.run(g)
}

func (l *layout) run(g *graphlib.Graph) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("dagre: layout failed: %v", r)
		}
	}()
	layoutGraph, err := buildLayoutGraph(g)
	if err != nil {
		return err
	}
	l.runLayout(layoutGraph)
	updateInputGraph(g, layoutGraph)
	return nil
}

func (l *layout) stage(name string, g *graphlib.Graph) {
	if l.debug != nil {
		l.debug(name, g)
	}
}

func (l *layout) runLayout(g *graphlib.Graph) {
	makeSpaceForEdgeLabels(g)
	l.stage("makeSpaceForEdgeLabels", g)
	removeSelfEdges(g)
	l.stage("removeSelfEdges", g)
	l.acyclicRun(g)
	l.stage("acyclic", g)
	l.nestingGraphRun(g)
	l.stage("nestingGraph.run", g)
	l.rank(asNonCompoundGraph(g))
	l.stage("rank", g)
	l.injectEdgeLabelProxies(g)
	l.stage("injectEdgeLabelProxies", g)
	removeEmptyRanks(g)
	l.stage("removeEmptyRanks", g)
	nestingGraphCleanup(g)
	l.stage("nestingGraph.cleanup", g)
	normalizeRanks(g)
	l.stage("normalizeRanks", g)
	assignRankMinMax(g)
	l.stage("assignRankMinMax", g)
	removeEdgeLabelProxies(g)
	l.stage("removeEdgeLabelProxies", g)
	l.normalizeRun(g)
	l.stage("normalize.run", g)
	parentDummyChains(g)
	l.stage("parentDummyChains", g)
	l.addBorderSegments(g)
	l.stage("addBorderSegments", g)
	l.order(g)
	l.stage("order", g)
	l.insertSelfEdges(g)
	l.stage("insertSelfEdges", g)
	coordinateSystemAdjust(g)
	l.stage("adjustCoordinateSystem", g)
	l.position(g)
	l.stage("position", g)
	positionSelfEdges(g)
	l.stage("positionSelfEdges", g)
	removeBorderNodes(g)
	l.stage("removeBorderNodes", g)
	normalizeUndo(g)
	l.stage("normalize.undo", g)
	fixupEdgeLabelCoords(g)
	l.stage("fixupEdgeLabelCoords", g)
	coordinateSystemUndo(g)
	l.stage("undoCoordinateSystem", g)
	translateGraph(g)
	l.stage("translateGraph", g)
	assignNodeIntersects(g)
	l.stage("assignNodeIntersects", g)
	reversePointsForReversedEdges(g)
	l.stage("reversePoints", g)
	acyclicUndo(g)
	l.stage("acyclic.undo", g)
}

// updateInputGraph copies the final layout information from the layout
// graph back to the input graph.
func updateInputGraph(inputGraph, layoutGraph *graphlib.Graph) {
	for _, v := range inputGraph.Nodes() {
		inputLabel := nodeAttrs(inputGraph.Node(v))
		layoutLabel := nodeOf(layoutGraph, v)

		if inputLabel != nil {
			inputLabel.X = layoutLabel.x
			inputLabel.Y = layoutLabel.y

			if len(layoutGraph.Children(v)) > 0 {
				inputLabel.Width = layoutLabel.width
				inputLabel.Height = layoutLabel.height
			}
		}
	}

	for _, e := range inputGraph.Edges() {
		inputLabel := edgeAttrs(inputGraph.EdgeObj(e))
		layoutLabel := edgeOf(layoutGraph, e)

		inputLabel.Points = layoutLabel.points
		if layoutLabel.hasXY {
			inputLabel.X = N(layoutLabel.x)
			inputLabel.Y = N(layoutLabel.y)
		}
	}

	inputGraphLabel := graphAttrs(inputGraph.Graph())
	inputGraphLabel.Width = graphOf(layoutGraph).width
	inputGraphLabel.Height = graphOf(layoutGraph).height
}

func graphAttrs(label any) *GraphLabel {
	if a, ok := label.(GraphAttrs); ok {
		return a.DagreGraph()
	}
	return nil
}

func nodeAttrs(label any) *NodeLabel {
	if a, ok := label.(NodeAttrs); ok {
		return a.DagreNode()
	}
	return nil
}

func edgeAttrs(label any) *EdgeLabel {
	if a, ok := label.(EdgeAttrs); ok {
		return a.DagreEdge()
	}
	return nil
}

// The defaults of the JavaScript function:
//
//	graphDefaults = { ranksep: 50, edgesep: 20, nodesep: 50, rankdir: 'tb' }
//	nodeDefaults = { width: 0, height: 0 }
//	edgeDefaults = { minlen: 1, weight: 1, width: 0, height: 0, labeloffset: 10, labelpos: 'r' }

// buildLayoutGraph constructs a new graph from the input graph, which can
// be used for layout. It copies only the attributes that dagre uses.
func buildLayoutGraph(inputGraph *graphlib.Graph) (*graphlib.Graph, error) {
	g := graphlib.New(graphlib.Options{Multigraph: true, Compound: true})

	// JavaScript reads the attributes of an undefined label as absent. It
	// throws when it writes the result to such a graph or edge label, at
	// the end of the layout. The port reports these labels first.
	in := graphAttrs(inputGraph.Graph())
	if in == nil {
		return nil, fmt.Errorf("dagre: the graph label is %T, it must implement GraphAttrs", inputGraph.Graph())
	}
	graph := &graphLabel{
		ranksep: 50, edgesep: 20, nodesep: 50, rankdir: "tb",
		nodeRankFactor: nan, width: nan, height: nan,
	}
	if in.Nodesep.Set {
		graph.nodesep = in.Nodesep.V
	}
	if in.Edgesep.Set {
		graph.edgesep = in.Edgesep.V
	}
	if in.Ranksep.Set {
		graph.ranksep = in.Ranksep.V
	}
	graph.marginx = in.Marginx
	graph.marginy = in.Marginy
	graph.acyclicer = in.Acyclicer
	graph.ranker = in.Ranker
	if in.Rankdir != "" {
		graph.rankdir = in.Rankdir
	}
	graph.align = in.Align
	g.SetGraph(graph)

	for _, v := range inputGraph.Nodes() {
		label := inputGraph.Node(v)
		node := newNodeLabel()
		node.width = 0
		node.height = 0
		if label != nil {
			attrs := nodeAttrs(label)
			if attrs == nil {
				return nil, fmt.Errorf("dagre: the label of node %q is %T, it must implement NodeAttrs", v, label)
			}
			node.width = attrs.Width
			node.height = attrs.Height
		}
		g.SetNode(v, node)
		parent, ok := inputGraph.Parent(v)
		setParent(g, v, parent, ok)
	}

	for _, e := range inputGraph.Edges() {
		label := inputGraph.EdgeObj(e)
		attrs := edgeAttrs(label)
		if attrs == nil {
			return nil, fmt.Errorf("dagre: the label of edge %q -> %q is %T, it must implement EdgeAttrs", e.V, e.W, label)
		}
		edge := newEdgeLabel()
		edge.minlen = 1
		edge.weight = 1
		edge.width = attrs.Width
		edge.height = attrs.Height
		edge.labeloffset = 10
		edge.labelpos = "r"
		if attrs.Minlen.Set {
			edge.minlen = attrs.Minlen.V
		}
		if attrs.Weight.Set {
			edge.weight = attrs.Weight.V
		}
		if attrs.Labeloffset.Set {
			edge.labeloffset = attrs.Labeloffset.V
		}
		if attrs.Labelpos != "" {
			edge.labelpos = attrs.Labelpos
		}
		setEdgeObj(g, e, edge)
	}

	return g, nil
}

// makeSpaceForEdgeLabels doubles minlen and halves ranksep, so that an edge
// label can sit at the mid-point between two nodes. It also adds the label
// offset to the label size.
func makeSpaceForEdgeLabels(g *graphlib.Graph) {
	graph := graphOf(g)
	graph.ranksep /= 2
	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		edge.minlen = float64(edge.minlen * 2)
		if strings.ToLower(edge.labelpos) != "c" {
			if graph.rankdir == "TB" || graph.rankdir == "BT" {
				edge.width += edge.labeloffset
			} else {
				edge.height += edge.labeloffset
			}
		}
	}
}

// injectEdgeLabelProxies creates temporary dummy nodes that capture the
// rank in which each edge label is going to be, if the label has a non-zero
// width and height.
func (l *layout) injectEdgeLabelProxies(g *graphlib.Graph) {
	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		if truthy(edge.width) && truthy(edge.height) {
			v := nodeOf(g, e.V)
			w := nodeOf(g, e.W)
			label := newNodeLabel()
			label.rank = float64((w.rank-v.rank)/2) + v.rank
			label.hasRank = true
			label.e = e
			l.addDummyNode(g, "edge-proxy", label, "_ep")
		}
	}
}

func assignRankMinMax(g *graphlib.Graph) {
	for _, v := range g.Nodes() {
		node := nodeOf(g, v)
		if node.borderTop != "" {
			node.minRank = nodeOf(g, node.borderTop).rank
			node.maxRank = nodeOf(g, node.borderBottom).rank
			node.hasMinRank = true
		}
	}
	// The JavaScript function also sets g.graph().maxRank. No code reads it.
}

func removeEdgeLabelProxies(g *graphlib.Graph) {
	for _, v := range g.Nodes() {
		node := nodeOf(g, v)
		if node.dummy == "edge-proxy" {
			edgeOf(g, node.e).labelRank = node.rank
			g.RemoveNode(v)
		}
	}
}

func translateGraph(g *graphlib.Graph) {
	minX := math.Inf(1)
	maxX := 0.0
	minY := math.Inf(1)
	maxY := 0.0
	graphLabel := graphOf(g)
	marginX := graphLabel.marginx
	if !truthy(marginX) {
		marginX = 0
	}
	marginY := graphLabel.marginy
	if !truthy(marginY) {
		marginY = 0
	}

	getExtremes := func(x, y, w, h float64) {
		minX = js.Min(minX, x-float64(w/2))
		maxX = js.Max(maxX, x+float64(w/2))
		minY = js.Min(minY, y-float64(h/2))
		maxY = js.Max(maxY, y+float64(h/2))
	}

	for _, v := range g.Nodes() {
		node := nodeOf(g, v)
		getExtremes(node.x, node.y, node.width, node.height)
	}
	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		if edge.hasXY {
			getExtremes(edge.x, edge.y, edge.width, edge.height)
		}
	}

	minX -= marginX
	minY -= marginY

	for _, v := range g.Nodes() {
		node := nodeOf(g, v)
		node.x -= minX
		node.y -= minY
	}

	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		for i := range edge.points {
			edge.points[i].X -= minX
			edge.points[i].Y -= minY
		}
		if edge.hasXY {
			edge.x -= minX
			edge.y -= minY
		}
	}

	graphLabel.width = maxX - minX + marginX
	graphLabel.height = maxY - minY + marginY
}

func assignNodeIntersects(g *graphlib.Graph) {
	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		nodeV := nodeOf(g, e.V)
		nodeW := nodeOf(g, e.W)
		var p1, p2 Point
		if edge.points == nil {
			edge.points = []Point{}
			p1 = Point{X: nodeW.x, Y: nodeW.y}
			p2 = Point{X: nodeV.x, Y: nodeV.y}
		} else {
			p1 = edge.points[0]
			p2 = edge.points[len(edge.points)-1]
		}
		first := intersectRect(rect{nodeV.x, nodeV.y, nodeV.width, nodeV.height}, p1)
		edge.points = append([]Point{first}, edge.points...)
		edge.points = append(edge.points, intersectRect(rect{nodeW.x, nodeW.y, nodeW.width, nodeW.height}, p2))
	}
}

func fixupEdgeLabelCoords(g *graphlib.Graph) {
	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		if edge.hasXY {
			if edge.labelpos == "l" || edge.labelpos == "r" {
				edge.width -= edge.labeloffset
			}
			switch edge.labelpos {
			case "l":
				edge.x -= float64(edge.width/2) + edge.labeloffset
			case "r":
				edge.x += float64(edge.width/2) + edge.labeloffset
			}
		}
	}
}

func reversePointsForReversedEdges(g *graphlib.Graph) {
	for _, e := range g.Edges() {
		edge := edgeOf(g, e)
		if edge.reversed {
			for i, j := 0, len(edge.points)-1; i < j; i, j = i+1, j-1 {
				edge.points[i], edge.points[j] = edge.points[j], edge.points[i]
			}
		}
	}
}

func removeBorderNodes(g *graphlib.Graph) {
	for _, v := range g.Nodes() {
		if len(g.Children(v)) > 0 {
			node := nodeOf(g, v)
			t := nodeOf(g, node.borderTop)
			b := nodeOf(g, node.borderBottom)
			l := nodeOf(g, last(node.borderLeft))
			r := nodeOf(g, last(node.borderRight))

			node.width = math.Abs(r.x - l.x)
			node.height = math.Abs(b.y - t.y)
			node.x = l.x + float64(node.width/2)
			node.y = t.y + float64(node.height/2)
		}
	}

	for _, v := range g.Nodes() {
		if nodeOf(g, v).dummy == "border" {
			g.RemoveNode(v)
		}
	}
}

// last is _.last for an array of node ids. The result is empty for an
// empty array, where JavaScript gives undefined.
func last(array []string) string {
	if len(array) == 0 {
		return ""
	}
	return array[len(array)-1]
}

func removeSelfEdges(g *graphlib.Graph) {
	for _, e := range g.Edges() {
		if e.V == e.W {
			node := nodeOf(g, e.V)
			node.selfEdges = append(node.selfEdges, selfEdge{e: e, label: edgeOf(g, e)})
			g.RemoveEdgeObj(e)
		}
	}
}

func (l *layout) insertSelfEdges(g *graphlib.Graph) {
	layers := buildLayerMatrix(g)
	for _, layer := range layers {
		orderShift := 0
		for i, v := range layer {
			node := nodeOf(g, v)
			node.order = float64(i + orderShift)
			for _, selfEdge := range node.selfEdges {
				orderShift++
				attrs := newNodeLabel()
				attrs.width = selfEdge.label.width
				attrs.height = selfEdge.label.height
				attrs.rank = node.rank
				attrs.hasRank = true
				attrs.order = float64(i + orderShift)
				attrs.e = selfEdge.e
				attrs.label = selfEdge.label
				l.addDummyNode(g, "selfedge", attrs, "_se")
			}
			node.selfEdges = nil
		}
	}
}

func positionSelfEdges(g *graphlib.Graph) {
	for _, v := range g.Nodes() {
		node := nodeOf(g, v)
		if node.dummy == "selfedge" {
			selfNode := nodeOf(g, node.e.V)
			x := selfNode.x + float64(selfNode.width/2)
			y := selfNode.y
			dx := node.x - x
			dy := float64(selfNode.height / 2)
			setEdgeObj(g, node.e, node.label)
			g.RemoveNode(v)
			node.label.points = []Point{
				{X: x + (2*dx)/3, Y: y - dy},
				{X: x + (5*dx)/6, Y: y - dy},
				{X: x + dx, Y: y},
				{X: x + (5*dx)/6, Y: y + dy},
				{X: x + (2*dx)/3, Y: y + dy},
			}
			node.label.x = node.x
			node.label.y = node.y
			node.label.hasXY = true
		}
	}
}

func fmtNum(x float64) string {
	if x == 0 && math.Signbit(x) {
		return "-0"
	}
	return js.Str(x)
}
