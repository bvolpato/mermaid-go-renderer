// Ported from mermaid 12.2.0, rendering-util/layout-algorithms/dagre/index.js
// (MIT).

// Package dagre is the port of mermaid's dagre layout: the layout
// algorithm that rendering-util/render.ts registers under the name "dagre"
// (rendering-util/layout-algorithms/dagre/index.js and mermaid-graphlib.js).
// Importing the package registers the layout with ru.RegisterLayout.
//
// The layout builds a compound graph of the layout data, moves every
// cluster that has no edge to the outside into a graph of its own, and
// renders these graphs from the inside out: it draws and measures the nodes
// of a graph, lays the graph out with dagre (internal/dagre) and draws the
// clusters and edges. The top graph is drawn by the common layout renderer
// (internal/ru/graph).
//
// # Labels
//
// Upstream stores the node and edge objects of mermaid as the labels of
// the graph, and dagre writes its result to these objects. The labels here
// are *Node and *Edge: each holds the *ru.Node or *ru.Edge that the other
// packages work on, and the attribute struct of dagre. The sizes are copied
// to that struct right before a layout and the result is copied back right
// after it (toDagre, fromDagre). One *Node stands for one JavaScript
// object, so the sharing of upstream is kept: the same label is the
// clusterData of a cluster, the node of a clusterDb entry and the argument
// of insertCluster.
//
// # Module state
//
// clusterDb, descendants and parents of mermaid-graphlib.js, and the
// uniqueId counter of lodash that dagre draws its ids from, are kept per
// render (ctx.Value).
//
// # Order of the work
//
// Upstream starts the work for all nodes of a graph together and awaits it
// with Promise.all: every node runs until its first await, and then the
// nodes continue in the order of the microtask queue. The nested graph of
// a cluster is measured, laid out and drawn inside such a callback, so its
// work is interleaved with the work of the other nodes. The order shows in
// the output: it decides which element gets which numbers of Math.random
// (the handDrawn look) and where an element that is inserted after an
// await comes in the document. The port keeps the order with the scheduler
// of the render context (internal/render/async.go): ctx.All for each
// Promise.all, and ctx.Await where a ported async function returns.
package dagre

import (
	"errors"
	"math"

	dagrelib "github.com/bvolpato/mermaid-go-renderer/internal/dagre"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	rugraph "github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
)

func init() {
	ru.RegisterLayout("dagre", "", Render)
	// clearLayoutRenderState calls clear() of mermaid-graphlib.js.
	rugraph.RegisterClear(Clear)
}

func clamp(value, min, max float64) float64 {
	return js.Max(min, js.Min(max, value))
}

// getDefaultSelfLoopSide is getDefaultSelfLoopSide(rankdir). An empty
// rankdir stands for undefined: the default "TB".
func getDefaultSelfLoopSide(rankdir string) string {
	switch rankdir {
	case "BT":
		return "bottom"
	case "LR":
		return "right"
	case "RL":
		return "left"
	default:
		return "top"
	}
}

func shouldMergeSelfLoopSegments(diagramType string) bool {
	return diagramType == "flowchart" ||
		diagramType == "flowchart-v2" ||
		diagramType == "stateDiagram" ||
		diagramType == "er" ||
		diagramType == "classDiagram"
}

// The properties of a node that the layout result is copied with are
// DAGRE_NODE_LAYOUT_PROPERTIES of upstream: x, y, width, height, labelBBox,
// intersect, calcIntersect, diff and clusterNode. See applyDagreNodeLayout.

// EdgeToRender is an entry of the result of GetEdgesToRender: an edge and
// the ids of the nodes at its ends.
type EdgeToRender struct {
	Edge  *Edge
	Start string
	End   string
}

// getSelfLoopSide uses the place of the dummy nodes of a self-loop as a
// hint, so that loops are not always above the node.
func getSelfLoopSide(graph *graphlib.Graph, node *Node, segments []EdgeToRender, originalNodeID, rankdir string) string {
	var layoutHints []ru.Point
	dummyNodeIds := js.NewSet[string]()

	for _, segment := range segments {
		if segment.Start != originalNodeID {
			dummyNodeIds.Add(segment.Start)
		}
		if segment.End != originalNodeID {
			dummyNodeIds.Add(segment.End)
		}
	}

	for _, id := range dummyNodeIds.Values() {
		dummyNode := nodeOf(graph, id)
		if dummyNode != nil && !ru.IsUndef(dummyNode.X) && !ru.IsUndef(dummyNode.Y) {
			layoutHints = append(layoutHints, ru.Point{X: dummyNode.X, Y: dummyNode.Y})
		}
	}

	if len(layoutHints) == 0 {
		for _, segment := range segments {
			for _, point := range segment.Edge.Points {
				if !ru.IsUndef(point.X) && !ru.IsUndef(point.Y) {
					layoutHints = append(layoutHints, point)
				}
			}
		}
	}

	if len(layoutHints) == 0 {
		return getDefaultSelfLoopSide(rankdir)
	}

	center := ru.Point{X: 0, Y: 0}
	n := float64(len(layoutHints))
	for _, point := range layoutHints {
		center = ru.Point{
			X: center.X + point.X/n,
			Y: center.Y + point.Y/n,
		}
	}
	dx := center.X - node.X
	dy := center.Y - node.Y

	if math.Abs(dx) > math.Abs(dy) {
		if dx > 0 {
			return "right"
		}
		return "left"
	}
	if math.Abs(dy) > 0 {
		if dy > 0 {
			return "bottom"
		}
		return "top"
	}
	return getDefaultSelfLoopSide(rankdir)
}

// getSelfLoopPoints builds a compact loop around the node, in place of the
// long route of dagre over the dummy nodes.
func getSelfLoopPoints(node *Node, side string, yOffset, labelWidth float64) []ru.Point {
	x := node.X
	y := node.Y - yOffset
	halfWidth := float64(node.Width / 2)
	halfHeight := float64(node.Height / 2)
	maxSpan := js.Max(36, js.Min(100, node.Width*0.8))
	span := clamp(js.Max(labelWidth, node.Width*0.35), 36, maxSpan)
	depth := clamp(js.Min(node.Width, node.Height)*0.45, 24, 48)

	switch side {
	case "bottom":
		bottom := y + halfHeight
		return []ru.Point{
			{X: x - float64(span/2), Y: bottom},
			{X: x - float64(span/2), Y: bottom + depth},
			{X: x + float64(span/2), Y: bottom + depth},
			{X: x + float64(span/2), Y: bottom},
		}
	case "right":
		right := x + halfWidth
		return []ru.Point{
			{X: right, Y: y - float64(span/2)},
			{X: right + depth, Y: y - float64(span/2)},
			{X: right + depth, Y: y + float64(span/2)},
			{X: right, Y: y + float64(span/2)},
		}
	case "left":
		left := x - halfWidth
		return []ru.Point{
			{X: left, Y: y - float64(span/2)},
			{X: left - depth, Y: y - float64(span/2)},
			{X: left - depth, Y: y + float64(span/2)},
			{X: left, Y: y + float64(span/2)},
		}
	default:
		top := y - halfHeight
		return []ru.Point{
			{X: x - float64(span/2), Y: top},
			{X: x - float64(span/2), Y: top - depth},
			{X: x + float64(span/2), Y: top - depth},
			{X: x + float64(span/2), Y: top},
		}
	}
}

// getSelfLoopLabelPosition is getSelfLoopLabelPosition(node, points, side,
// yOffset, label). labelWidth and labelHeight are label.width and
// label.height, which can be undefined.
func getSelfLoopLabelPosition(node *Node, points []ru.Point, side string, yOffset, labelWidth, labelHeight float64) ru.Point {
	const gap = 4
	x := node.X
	y := node.Y - yOffset
	labelWidth = ru.NN(labelWidth, 0)
	labelHeight = ru.NN(labelHeight, 0)

	xs := make([]float64, len(points))
	ys := make([]float64, len(points))
	for i, point := range points {
		xs[i] = point.X
		ys[i] = point.Y
	}

	switch side {
	case "bottom":
		return ru.Point{X: x, Y: js.Max(ys...) + float64(labelHeight/2) + gap}
	case "right":
		return ru.Point{X: js.Max(xs...) + float64(labelWidth/2) + gap, Y: y}
	case "left":
		return ru.Point{X: js.Min(xs...) - float64(labelWidth/2) - gap, Y: y}
	default:
		return ru.Point{X: x, Y: js.Min(ys...) - float64(labelHeight/2) - gap}
	}
}

// firstNonEmpty is `a ?? b ?? c` for strings whose empty value stands for
// undefined.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// GetEdgesToRender is getEdgesToRender(graph, yOffset, { mergeSelfLoops }):
// it converts the edges of a laid-out graph into the edges to draw. With
// mergeSelfLoops, the three segments of a self-loop become one edge with a
// compact loop beside its node. The merged edges come after the other
// edges.
func GetEdgesToRender(graph *graphlib.Graph, yOffset float64, mergeSelfLoops bool) ([]EdgeToRender, error) {
	selfLoopEdgeGroups := js.NewMap[string, []EdgeToRender]()
	edgesToRender := []EdgeToRender{}
	rankdir := ""
	if label := graphOf(graph); label != nil {
		rankdir = label.Rankdir
	}

	for _, e := range graph.Edges() {
		edge := edgeOf(graph, e)
		if mergeSelfLoops {
			if edge == nil {
				return nil, typeError("Cannot read properties of undefined (reading 'selfLoop')")
			}
			if edge.SelfLoop != nil {
				key := edge.SelfLoop.ID
				selfLoopEdgeGroups.Set(key, append(selfLoopEdgeGroups.Get(key), EdgeToRender{Edge: edge, Start: e.V, End: e.W}))
				continue
			}
		}
		edgesToRender = append(edgesToRender, EdgeToRender{Edge: edge, Start: e.V, End: e.W})
	}

	for _, segments := range selfLoopEdgeGroups.Values() {
		if len(segments) != 3 {
			// A self-loop state that is not expected: draw the segments,
			// and do not drop edges.
			edgesToRender = append(edgesToRender, segments...)
			continue
		}

		js.SortFunc(segments, func(a, b EdgeToRender) float64 {
			return float64(a.Edge.SelfLoop.Order - b.Edge.SelfLoop.Order)
		})
		firstSegment, middleSegment, lastSegment := segments[0], segments[1], segments[2]
		originalEdge := firstSegment.Edge.OriginalEdge
		if originalEdge == nil {
			originalEdge = middleSegment.Edge.OriginalEdge
		}
		if originalEdge == nil {
			originalEdge = lastSegment.Edge.OriginalEdge
		}
		if originalEdge == nil {
			originalEdge = middleSegment.Edge.Edge
		}
		node := nodeOf(graph, originalEdge.Start)
		if node == nil {
			edgesToRender = append(edgesToRender, segments...)
			continue
		}
		labelWidth := middleSegment.Edge.Width
		labelHeight := middleSegment.Edge.Height
		// dagre uses the route over the dummy nodes for the layout. The SVG
		// has one edge for the self-loop.
		side := getSelfLoopSide(graph, node, segments, originalEdge.Start, rankdir)
		points := getSelfLoopPoints(node, side, yOffset, ru.NN(labelWidth, 0))
		labelPosition := getSelfLoopLabelPosition(node, points, side, yOffset, labelWidth, labelHeight)

		// { ...middleSegment.edge, ...originalEdge, ... }: the middle
		// segment is a copy of the original edge with the changes of
		// prepareLayoutForDagre, of the measure phase and of the layout.
		// The spread of the original edge takes the changes of
		// prepareLayoutForDagre back, and the properties after it set the
		// rest. An arrow type that the original edge does not have stays
		// "none", as it does upstream for a property that is absent.
		merged := *middleSegment.Edge.Edge
		merged.StartLabelLeft = originalEdge.StartLabelLeft
		merged.StartLabelRight = originalEdge.StartLabelRight
		merged.EndLabelLeft = originalEdge.EndLabelLeft
		merged.EndLabelRight = originalEdge.EndLabelRight
		if originalEdge.ArrowTypeStart != "" {
			merged.ArrowTypeStart = originalEdge.ArrowTypeStart
		}
		if originalEdge.ArrowTypeEnd != "" {
			merged.ArrowTypeEnd = originalEdge.ArrowTypeEnd
		}
		merged.ID = originalEdge.ID
		merged.Points = points
		merged.Start = originalEdge.Start
		merged.End = originalEdge.End
		merged.X = labelPosition.X
		merged.Y = labelPosition.Y
		merged.Width = labelWidth
		merged.Height = labelHeight
		merged.FromCluster = firstNonEmpty(firstSegment.Edge.FromCluster, middleSegment.Edge.FromCluster, lastSegment.Edge.FromCluster)
		merged.ToCluster = firstNonEmpty(firstSegment.Edge.ToCluster, middleSegment.Edge.ToCluster, lastSegment.Edge.ToCluster)
		// The merged edge has no selfLoop and no originalEdge.
		mergedEdge := &Edge{Edge: &merged}

		edgesToRender = append(edgesToRender, EdgeToRender{Edge: mergedEdge, Start: merged.Start, End: merged.End})
	}

	return edgesToRender, nil
}

// MeasuredLayout is the result of the measure phase of a graph: the graph
// with the sizes of its nodes and edge labels, and the groups that its
// elements are in. It is the Measure of the layout for the common paint.
type MeasuredLayout struct {
	// Elem is the g.root of the graph.
	Elem   *dom.Node
	Graph  *graphlib.Graph
	Groups *rugraph.LayoutElementGroups
	// DiagramType is data4Layout.type and ID is data4Layout.diagramId.
	DiagramType    string
	ID             string
	MergeSelfLoops bool
	// SubGraphTitleTotalMargin is the sum of the margins above and below
	// the title of a subgraph.
	SubGraphTitleTotalMargin float64

	sc *scope
}

// LayoutGroups returns the groups that the common paint draws into.
func (m *MeasuredLayout) LayoutGroups() *rugraph.LayoutElementGroups { return m.Groups }

// renderOptions are the arguments of measureDagreGraph.
type renderOptions struct {
	element     *dom.Node
	graph       *graphlib.Graph
	diagramType string
	id          string
	// parentCluster is the cluster node that the graph is the content of,
	// or nil for the top graph.
	parentCluster *Node
	siteConfig    *js.Obj[any]
}

func (st *state) measureDagreGraph(o renderOptions) (*MeasuredLayout, error) {
	ctx := st.ctx
	_elem, graph, diagramType, id, parentCluster, siteConfig := o.element, o.graph, o.diagramType, o.id, o.parentCluster, o.siteConfig
	graphLabel := graphOf(graph)
	if graphLabel == nil {
		return nil, typeError("Cannot read properties of undefined (reading 'rankdir')")
	}
	dir := graphLabel.Rankdir
	sc := &scope{graph: graph}
	if parentCluster != nil {
		sc.parentCluster = parentCluster.ID
	}

	groups := st.ops.createLayoutElementGroups(sc, _elem, rugraph.CreateLayoutElementGroupsOptions{
		EdgePathsClass: "edgePaths",
	})
	clusters, edgePaths, edgeLabels, nodes, elem := groups.Clusters, groups.EdgePaths, groups.EdgeLabels, groups.Nodes, groups.RootGroups
	_, _ = clusters, edgePaths
	mergeSelfLoops := shouldMergeSelfLoopSegments(diagramType)

	// Insert the nodes. This puts them into the DOM and gives each node a
	// size. The size is stored in the node and dagre uses it for the layout.
	//
	// await Promise.all(graph.nodes().map(async function (v) { ... }))
	nodeIDs := graph.Nodes()
	err := ctx.All(len(nodeIDs), func(i int) error {
		v := nodeIDs[i]
		node := nodeOf(graph, v)
		if parentCluster != nil {
			if parentCluster.ClusterData == nil {
				// JSON.parse(JSON.stringify(undefined)) throws.
				return errors.New(`"undefined" is not valid JSON`)
			}
			data := &Node{Node: cloneNodeJSON(parentCluster.ClusterData.Node)}
			graph.SetNode(parentCluster.ID, data)
			if parent, ok := graph.Parent(v); !ok || parent == "" {
				if err := graph.SetParent(v, parentCluster.ID); err != nil {
					return err
				}
			}
		}
		if node != nil && node.ClusterNode {
			if node.Graph == nil {
				return typeError("Cannot read properties of undefined (reading 'setGraph')")
			}
			// node.graph.setGraph applies the spacing of this graph to the
			// subgraph: without it the subgraph has the default values.
			// Only ranksep and nodesep are replaced.
			ranksep, nodesep := graphLabel.Ranksep, graphLabel.Nodesep
			subLabel := GraphLabel{Width: ru.Undef, Height: ru.Undef}
			if l := graphOf(node.Graph); l != nil {
				subLabel = *l
			}
			subLabel.Ranksep = addNumber(ranksep, 25)
			subLabel.Nodesep = nodesep
			node.Graph.SetGraph(&subLabel)

			// "o" has the whole cluster, not only the children.
			o, err := st.renderDagreSubgraph(renderOptions{
				element:       nodes,
				graph:         node.Graph,
				diagramType:   diagramType,
				id:            id,
				parentCluster: nodeOf(graph, v),
				siteConfig:    siteConfig,
			})
			if err != nil {
				return err
			}
			newEl := o.elem
			st.ops.updateNodeBounds(sc, node.Node, newEl)
			node.Diff = ru.Or(o.diff, 0)
			st.ops.setNodeElem(sc, newEl, node.Node)
		} else {
			n, err := childCount(graph, v)
			if err != nil {
				return err
			}
			if n > 0 {
				// This is a cluster that is not rendered as a graph of its
				// own.
				if node == nil {
					return typeError("Cannot read properties of undefined (reading 'id')")
				}
				anchor, err := FindNonClusterChild(node.ID, graph, "")
				if err != nil {
					return err
				}
				st.clusterDb.Set(node.ID, &ClusterEntry{ID: anchor, Node: node})
			} else {
				current := nodeOf(graph, v)
				if current == nil {
					return typeError("Cannot read properties of undefined (reading 'shape')")
				}
				if err := st.ops.insertMeasuredNode(sc, nodes, current.Node, &ru.ShapeRenderOptions{Config: siteConfig, Dir: dir, Padding: ru.Undef}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// processEdges: the labels of all edges are inserted together.
	//
	// const edgePromises = graph.edges().map(async function (e) { ... });
	// await Promise.all(edgePromises);
	edgeObjs := graph.Edges()
	err = ctx.All(len(edgeObjs), func(i int) error {
		edge := edgeOf(graph, edgeObjs[i])
		if edge == nil {
			if mergeSelfLoops {
				return typeError("Cannot read properties of undefined (reading 'selfLoop')")
			}
			return typeError("Cannot read properties of undefined (reading 'cssCompiledStyles')")
		}

		if mergeSelfLoops && edge.SelfLoop != nil {
			// Only the middle segment has the label, under the id of the
			// original edge.
			if edge.SelfLoop.Order != 1 {
				return nil
			}
			// { ...edge.originalEdge, ...edge, id, startLabelLeft, ... }
			labelEdge := *edge.Edge
			labelEdge.ID = edge.SelfLoop.ID
			if originalEdge := edge.OriginalEdge; originalEdge != nil {
				labelEdge.StartLabelLeft = firstNonEmpty(originalEdge.StartLabelLeft, edge.StartLabelLeft)
				labelEdge.StartLabelRight = firstNonEmpty(originalEdge.StartLabelRight, edge.StartLabelRight)
				labelEdge.EndLabelLeft = firstNonEmpty(originalEdge.EndLabelLeft, edge.EndLabelLeft)
				labelEdge.EndLabelRight = firstNonEmpty(originalEdge.EndLabelRight, edge.EndLabelRight)
			}
			if err := st.ops.insertEdgeLabel(sc, edgeLabels, &labelEdge); err != nil {
				return err
			}
			edge.Width = labelEdge.Width
			edge.Height = labelEdge.Height
			edge.LabelStyle = labelEdge.LabelStyle
			edge.LabelStyleText = labelEdge.LabelStyleText
			edge.LabelStyleIsText = labelEdge.LabelStyleIsText
			return nil
		}
		return st.ops.insertEdgeLabel(sc, edgeLabels, edge.Edge)
	})
	if err != nil {
		return nil, err
	}
	// await processEdges(): processEdges is an async function.
	ctx.Await()

	subGraphTitleTotalMargin := rugraph.GetSubGraphTitleMargins(siteConfig).SubGraphTitleTotalMargin
	// measureDagreGraph is an async function.
	ctx.Await()
	return &MeasuredLayout{
		Elem:                     elem,
		Graph:                    graph,
		Groups:                   groups,
		DiagramType:              diagramType,
		ID:                       id,
		MergeSelfLoops:           mergeSelfLoops,
		SubGraphTitleTotalMargin: subGraphTitleTotalMargin,
		sc:                       sc,
	}, nil
}

// runDagreGraphLayout is runDagreGraphLayout(graph): the layout of dagre.
func (st *state) runDagreGraphLayout(sc *scope, graph *graphlib.Graph) error {
	toDagre(graph)
	if st.trace != nil && st.trace.beforeLayout != nil {
		st.trace.beforeLayout(sc, graph)
	}
	if err := dagrelib.Layout(graph, st.dagre); err != nil {
		return err
	}
	fromDagre(graph)
	if st.trace != nil && st.trace.afterLayout != nil {
		st.trace.afterLayout(sc, graph)
	}
	return nil
}

// normalizeDagreNode returns a copy of the node with the title margin of
// the subgraphs applied, or nil when the node has no label.
func normalizeDagreNode(graph *graphlib.Graph, nodeID string, subGraphTitleTotalMargin float64) *ru.Node {
	node := nodeOf(graph, nodeID)
	if node == nil {
		return nil
	}

	normalizedNode := *node.Node
	if node.ClusterNode {
		normalizedNode.Y = ru.NN(node.Y, 0) + subGraphTitleTotalMargin
	} else if len(graph.Children(nodeID)) > 0 {
		normalizedNode.Height = ru.NN(node.Height, 0) + subGraphTitleTotalMargin
	} else {
		normalizedNode.Y = ru.NN(node.Y, 0) + float64(subGraphTitleTotalMargin/2)
	}
	return &normalizedNode
}

// applyDagreNodeLayout copies the layout properties that are not undefined
// from dagreNode to targetNode.
func applyDagreNodeLayout(targetNode, dagreNode *ru.Node) {
	if !ru.IsUndef(dagreNode.X) {
		targetNode.X = dagreNode.X
	}
	if !ru.IsUndef(dagreNode.Y) {
		targetNode.Y = dagreNode.Y
	}
	if !ru.IsUndef(dagreNode.Width) {
		targetNode.Width = dagreNode.Width
	}
	if !ru.IsUndef(dagreNode.Height) {
		targetNode.Height = dagreNode.Height
	}
	if dagreNode.LabelBBox != nil {
		targetNode.LabelBBox = dagreNode.LabelBBox
	}
	if dagreNode.Intersect != nil {
		targetNode.Intersect = dagreNode.Intersect
	}
	if dagreNode.CalcIntersect != nil {
		targetNode.CalcIntersect = dagreNode.CalcIntersect
	}
	if !ru.IsUndef(dagreNode.Diff) {
		targetNode.Diff = dagreNode.Diff
	}
	if dagreNode.ClusterNode {
		targetNode.ClusterNode = dagreNode.ClusterNode
	}
}

func normalizeDagreEdge(edge *Edge, start, end string, edgeOffsetY float64) *ru.Edge {
	normalized := *edge.Edge
	normalized.Start = firstNonEmpty(edge.Start, start)
	normalized.End = firstNonEmpty(edge.End, end)
	points := make([]ru.Point, len(edge.Points))
	for i, point := range edge.Points {
		points[i] = ru.Point{X: point.X, Y: point.Y + edgeOffsetY}
	}
	normalized.Points = points
	return &normalized
}

// ApplyDagreLayoutResult is applyDagreLayoutResult(data4Layout,
// measuredLayout): it copies the result of the layout of the top graph to
// the layout data. The nodes of the layout data get their position and
// size, and the edges of the layout data are replaced by the edges to
// draw.
//
// The result covers the top graph only. The children of a cluster that is
// rendered as a nested graph, and the edges between them, were drawn in
// the measure phase in the coordinates of that graph: these nodes keep
// the values that they had, and these edges are not in data4Layout.Edges.
// Where the segments of a self-loop are not merged, the three segments are
// in data4Layout.Edges in place of the self-loop.
func ApplyDagreLayoutResult(data4Layout *ru.LayoutData, measuredLayout *MeasuredLayout) (*ru.LayoutData, error) {
	graph, mergeSelfLoops, subGraphTitleTotalMargin := measuredLayout.Graph, measuredLayout.MergeSelfLoops, measuredLayout.SubGraphTitleTotalMargin
	nodeByID := make(map[string]*ru.Node, len(data4Layout.Nodes))
	for _, node := range data4Layout.Nodes {
		if node == nil {
			return nil, typeError("Cannot read properties of undefined (reading 'id')")
		}
		nodeByID[node.ID] = node
	}

	for _, nodeID := range SortNodesByHierarchy(graph) {
		dagreNode := normalizeDagreNode(graph, nodeID, subGraphTitleTotalMargin)
		if dagreNode == nil {
			continue
		}

		applyDagreNodeLayout(nodeOf(graph, nodeID).Node, dagreNode)

		if targetNode := nodeByID[nodeID]; targetNode != nil {
			applyDagreNodeLayout(targetNode, dagreNode)
		}
	}

	edgeOffsetY := float64(subGraphTitleTotalMargin / 2)
	edgesToRender, err := GetEdgesToRender(graph, edgeOffsetY, mergeSelfLoops)
	if err != nil {
		return nil, err
	}
	normalized := make([]*ru.Edge, len(edgesToRender))
	for i, er := range edgesToRender {
		if er.Edge == nil {
			return nil, typeError("Cannot read properties of undefined (reading 'start')")
		}
		normalized[i] = normalizeDagreEdge(er.Edge, er.Start, er.End, edgeOffsetY)
	}
	data4Layout.Edges = normalized

	return data4Layout, nil
}

// subgraphResult is the result of paintDagreLayoutCore: the g.root of the
// graph, and the diff of its last group node (undefined when no node is a
// group).
type subgraphResult struct {
	elem *dom.Node
	diff float64
}

// clusterDbFunc returns the clusterDb Map as insertEdge reads it.
func (st *state) clusterDbFunc() edges.ClusterDb {
	return func(id string) (ru.Bounds, bool) {
		entry, ok := st.clusterDb.Lookup(id)
		if !ok || entry.Node == nil {
			return ru.Bounds{}, false
		}
		return ru.NodeBounds(entry.Node.Node), true
	}
}

// ruNode returns the mermaid node of a label, or nil for an undefined label.
func ruNode(node *Node) *ru.Node {
	if node == nil {
		return nil
	}
	return node.Node
}

func (st *state) paintDagreLayoutCore(m *MeasuredLayout) (*subgraphResult, error) {
	ctx := st.ctx
	elem, graph, diagramType, id, mergeSelfLoops, subGraphTitleTotalMargin := m.Elem, m.Graph, m.DiagramType, m.ID, m.MergeSelfLoops, m.SubGraphTitleTotalMargin
	clusters, edgePaths := m.Groups.Clusters, m.Groups.EdgePaths
	sc := m.sc
	// Move the nodes to the correct place.
	//
	// await Promise.all(sortNodesByHierarchy(graph).map(async function (v) { ... }))
	diff := 0.0
	sorted := SortNodesByHierarchy(graph)
	err := ctx.All(len(sorted), func(i int) error {
		v := sorted[i]
		node := nodeOf(graph, v)
		if node == nil {
			return typeError("Cannot read properties of undefined (reading 'x')")
		}
		if node.ClusterNode {
			// Adjust for the padding when on the root level.
			node.Y += subGraphTitleTotalMargin

			entry, ok := st.clusterDb.Lookup(node.ID)
			if !ok {
				return typeError("Cannot set properties of undefined (setting 'node')")
			}
			entry.Node = node
			st.ops.positionNode(sc, node.Node)
		} else {
			n, err := childCount(graph, v)
			if err != nil {
				return err
			}
			if n > 0 {
				// A cluster that stays in this graph.
				node.Height += subGraphTitleTotalMargin
				if err := st.ops.insertCluster(sc, clusters, node.Node); err != nil {
					return err
				}

				entry, ok := st.clusterDb.Lookup(node.ID)
				if !ok {
					return typeError("Cannot set properties of undefined (setting 'node')")
				}
				entry.Node = node
			} else {
				// A regular node.
				node.Y += float64(subGraphTitleTotalMargin / 2)

				st.ops.positionNode(sc, node.Node)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Move the edge labels to the correct place after the layout.
	edgeOffsetY := float64(subGraphTitleTotalMargin / 2)
	edgesToRender, err := GetEdgesToRender(graph, edgeOffsetY, mergeSelfLoops)
	if err != nil {
		return nil, err
	}

	for _, er := range edgesToRender {
		edge, start, end := er.Edge, er.Start, er.End
		if edge == nil {
			return nil, typeError("Cannot read properties of undefined (reading 'points')")
		}

		for i := range edge.Points {
			edge.Points[i].Y += edgeOffsetY
		}
		startNode := nodeOf(graph, start)
		endNode := nodeOf(graph, end)
		paths, err := st.ops.insertEdge(sc, edgePaths, edge.Edge, st.clusterDbFunc(), diagramType, ruNode(startNode), ruNode(endNode), id)
		if err != nil {
			return nil, err
		}
		if err := st.ops.positionEdgeLabel(sc, edge.Edge, paths); err != nil {
			return nil, err
		}
	}

	for _, v := range graph.Nodes() {
		n := nodeOf(graph, v)
		if n == nil {
			return nil, typeError("Cannot read properties of undefined (reading 'type')")
		}
		if n.IsGroup {
			diff = n.Diff
		}
	}
	// paintDagreLayoutCore is an async function.
	ctx.Await()
	return &subgraphResult{elem: elem, diff: diff}, nil
}

func (st *state) renderDagreSubgraph(options renderOptions) (*subgraphResult, error) {
	measuredLayout, err := st.measureDagreGraph(options)
	if err != nil {
		return nil, err
	}
	if err := st.runDagreGraphLayout(measuredLayout.sc, measuredLayout.Graph); err != nil {
		return nil, err
	}
	result, err := st.paintDagreLayoutCore(measuredLayout)
	if err != nil {
		return nil, err
	}
	// renderDagreSubgraph is an async function: `return await ...`.
	st.ctx.Await()
	return result, nil
}

// PreparedLayout is the result of PrepareLayoutForDagre. The measure phase
// stores its result in it for the layout phase.
type PreparedLayout struct {
	Graph          *graphlib.Graph
	MeasuredLayout *MeasuredLayout
}

// spacing is `data4Layout.config?.[key] || own || data4Layout.config?.flowchart?.[key]`.
//
// The order is: a top-level config value (it exists only when a user sets
// it, because it has no schema default), then the value that the renderer
// of the diagram put on the layout data, then the flowchart config as the
// fallback. The flowchart keys have schema defaults and always exist, so
// they come last.
//
// own is the number field of the layout data (data4Layout.NodeSpacing or
// data4Layout.RankSpacing). A config value can be a string (the corpus has
// rankSpacing: '100'), and upstream hands the string to the graph label,
// where `ranksep + 25` of a nested graph then joins two strings. A diagram
// that has such a value puts it into data4Layout.Extra under the name of
// the property, and that value is used in place of the number.
func spacing(data4Layout *ru.LayoutData, key string, own float64) any {
	if v := js.Path(data4Layout.Config, key); js.Truthy(v) {
		return v
	}
	var ownValue any
	if !ru.IsUndef(own) {
		ownValue = own
	}
	if v, ok := data4Layout.Extra[key]; ok {
		ownValue = v
	}
	if js.Truthy(ownValue) {
		return ownValue
	}
	return js.Path(data4Layout.Config, "flowchart", key)
}

// PrepareLayoutForDagre is prepareLayoutForDagre(data4Layout): it builds
// the compound graph of the layout data. The label of a node is a *Node
// with a copy of the node, and the label of an edge is an *Edge with a
// copy of the edge. A self-loop becomes three edges over two small dummy
// nodes. Then AdjustClustersAndEdges moves the clusters without external
// connections into graphs of their own.
func PrepareLayoutForDagre(ctx *render.Context, data4Layout *ru.LayoutData) (*PreparedLayout, error) {
	st := getState(ctx)
	graph := newGraph(newGraphLabel(
		data4Layout.Direction,
		spacing(data4Layout, "nodeSpacing", data4Layout.NodeSpacing),
		spacing(data4Layout, "rankSpacing", data4Layout.RankSpacing),
	))

	for _, node := range data4Layout.Nodes {
		if node == nil {
			return nil, typeError("Cannot read properties of undefined (reading 'id')")
		}
		c := *node
		graph.SetNode(node.ID, &Node{Node: &c})
		if node.ParentID != "" {
			if err := graph.SetParent(node.ID, node.ParentID); err != nil {
				return nil, err
			}
		}
	}

	for _, edge := range data4Layout.Edges {
		if edge == nil {
			return nil, typeError("Cannot read properties of undefined (reading 'start')")
		}
		if edge.Start == edge.End {
			// Keep the dummy nodes of dagre for the layout. The segments
			// are merged before they are drawn.
			nodeID := edge.Start
			specialID1 := nodeID + "---" + nodeID + "---1"
			specialID2 := nodeID + "---" + nodeID + "---2"
			node := nodeOf(graph, nodeID)
			if node == nil {
				return nil, typeError("Cannot read properties of undefined (reading 'parentId')")
			}
			for _, specialID := range []string{specialID1, specialID2} {
				special := ru.NewNode(specialID)
				special.DomID = specialID
				special.ParentID = node.ParentID
				special.LabelStyle = ""
				special.Label = ""
				special.HasLabel = true
				special.Padding = 0
				special.Shape = "labelRect"
				special.Set("style", "")
				special.Width = 10
				special.Height = 10
				graph.SetNode(specialID, &Node{Node: special})
				if err := setParent(graph, specialID, node.ParentID, node.ParentID != ""); err != nil {
					return nil, err
				}
			}

			originalEdge := structuredCloneEdge(edge)
			// Keep the original edge, so that the path in the SVG has the
			// id and the data of the self-loop.
			edge1 := &Edge{Edge: structuredCloneEdge(edge), OriginalEdge: originalEdge, SelfLoop: &SelfLoop{ID: originalEdge.ID, Order: 0}}
			edgeMid := &Edge{Edge: structuredCloneEdge(edge), OriginalEdge: originalEdge, SelfLoop: &SelfLoop{ID: originalEdge.ID, Order: 1}}
			edge2 := &Edge{Edge: structuredCloneEdge(edge), OriginalEdge: originalEdge, SelfLoop: &SelfLoop{ID: originalEdge.ID, Order: 2}}
			edge1.Label = ""
			edge1.HasLabel = true
			edge1.ArrowTypeEnd = "none"
			edge1.EndLabelLeft = ""
			edge1.EndLabelRight = ""
			edge1.StartLabelLeft = ""
			edge1.ID = nodeID + "-cyclic-special-1"
			edgeMid.StartLabelRight = ""
			edgeMid.StartLabelLeft = ""
			edgeMid.EndLabelLeft = ""
			edgeMid.EndLabelRight = ""
			edgeMid.ArrowTypeStart = "none"
			edgeMid.ArrowTypeEnd = "none"
			edgeMid.ID = nodeID + "-cyclic-special-mid"
			edge2.Label = ""
			edge2.HasLabel = true
			edge2.StartLabelRight = ""
			edge2.StartLabelLeft = ""
			edge2.ArrowTypeStart = "none"
			if node.IsGroup {
				edge1.FromCluster = nodeID
				edge2.ToCluster = nodeID
			}
			edge2.ID = nodeID + "-cyclic-special-2"
			edge2.ArrowTypeStart = "none"
			if err := graph.SetEdge(nodeID, specialID1, edge1, nodeID+"-cyclic-special-0"); err != nil {
				return nil, err
			}
			if err := graph.SetEdge(specialID1, specialID2, edgeMid, nodeID+"-cyclic-special-1"); err != nil {
				return nil, err
			}
			if err := graph.SetEdge(specialID2, nodeID, edge2, nodeID+"-cyclic-special-2"); err != nil {
				return nil, err
			}
		} else {
			c := *edge
			label := &Edge{Edge: &c}
			var err error
			if edge.ID != "" {
				err = graph.SetEdge(edge.Start, edge.End, label, edge.ID)
			} else {
				err = graph.SetEdge(edge.Start, edge.End, label)
			}
			if err != nil {
				return nil, err
			}
		}
	}

	if st.trace != nil && st.trace.beforeAdjust != nil {
		st.trace.beforeAdjust(graph)
	}
	if err := st.adjustClustersAndEdges(graph, 0); err != nil {
		return nil, err
	}
	if st.trace != nil && st.trace.afterAdjust != nil {
		st.trace.afterAdjust(graph)
	}

	return &PreparedLayout{Graph: graph}, nil
}

// MeasureDagreLayout is measureDagreLayout(data4Layout, { element,
// preparedLayout }): the measure phase. It draws the nodes and the edge
// labels of the top graph into the root group and measures them. A cluster
// that is a nested graph is rendered completely here: measured, laid out
// and drawn.
func MeasureDagreLayout(data4Layout *ru.LayoutData, rc *rugraph.RenderContext) (*MeasuredLayout, error) {
	st := getState(rc.Ctx)
	prepared, _ := rc.PreparedLayout.(*PreparedLayout)
	if prepared == nil {
		var err error
		if prepared, err = PrepareLayoutForDagre(rc.Ctx, data4Layout); err != nil {
			return nil, err
		}
	}
	siteConfig := rc.Ctx.Config
	measuredLayout, err := st.measureDagreGraph(renderOptions{
		element:       rc.Element,
		graph:         prepared.Graph,
		diagramType:   data4Layout.Type,
		id:            data4Layout.DiagramID,
		parentCluster: nil,
		siteConfig:    siteConfig,
	})
	if err != nil {
		return nil, err
	}
	prepared.MeasuredLayout = measuredLayout
	// measureDagreLayout is an async function.
	rc.Ctx.Await()
	return measuredLayout, nil
}

// RunDagreLayoutCore is runDagreLayoutCore(data4Layout, context): the
// layout phase. It lays out the top graph with dagre and copies the result
// to the layout data (ApplyDagreLayoutResult).
func RunDagreLayoutCore(data4Layout *ru.LayoutData, rc *rugraph.RenderContext) (*MeasuredLayout, error) {
	st := getState(rc.Ctx)
	var measuredLayout *MeasuredLayout
	if prepared, _ := rc.PreparedLayout.(*PreparedLayout); prepared != nil {
		measuredLayout = prepared.MeasuredLayout
	}

	if measuredLayout == nil {
		return nil, errors.New("runDagreLayoutCore requires measureDagreLayout to run first")
	}

	if err := st.runDagreGraphLayout(measuredLayout.sc, measuredLayout.Graph); err != nil {
		return nil, err
	}
	if _, err := ApplyDagreLayoutResult(data4Layout, measuredLayout); err != nil {
		return nil, err
	}
	return measuredLayout, nil
}

// measureOf returns the measured layout of a paint context.
func measureOf(pc *rugraph.PaintContext) *MeasuredLayout {
	m, _ := pc.Measure.(*MeasuredLayout)
	return m
}

func getDagrePaintNodes(_ *ru.LayoutData, pc *rugraph.PaintContext) []*ru.Node {
	measure := measureOf(pc)
	nodes := []*ru.Node{}
	for _, nodeID := range SortNodesByHierarchy(measure.Graph) {
		if node := nodeOf(measure.Graph, nodeID); node != nil {
			nodes = append(nodes, node.Node)
		}
	}
	return nodes
}

func getDagreEdgeNode(nodeID string, _ *ru.Edge, pc *rugraph.PaintContext) *ru.Node {
	if nodeID == "" {
		return nil
	}
	return ruNode(nodeOf(measureOf(pc).Graph, nodeID))
}

// paintOptions are the paintOptions of the layout: how the common paint
// finds the nodes and edges of the top graph.
var paintOptions = &rugraph.PaintOptions{
	ClusterDb: func(pc *rugraph.PaintContext) edges.ClusterDb {
		return getState(pc.Ctx).clusterDbFunc()
	},
	GetNodes:    getDagrePaintNodes,
	GetEdgeNode: getDagreEdgeNode,
	SkipNode: func(node *ru.Node, pc *rugraph.PaintContext) bool {
		return !measureOf(pc).Graph.HasNode(node.ID)
	},
	IsCluster: func(node *ru.Node, pc *rugraph.PaintContext) bool {
		measure := measureOf(pc)
		return measure.Graph.HasNode(node.ID) && len(measure.Graph.Children(node.ID)) > 0
	},
}

// Render is the render function of the layout: render of index.js, built
// with createCommonLayoutRenderer.
var Render = rugraph.CreateCommonLayoutRenderer(rugraph.Definition{
	PrepareLayout: func(data4Layout *ru.LayoutData, rc *rugraph.RenderContext) (any, error) {
		prepared, err := PrepareLayoutForDagre(rc.Ctx, data4Layout)
		if err != nil {
			return nil, err
		}
		return prepared, nil
	},
	MeasureLayout: func(data4Layout *ru.LayoutData, rc *rugraph.RenderContext) (rugraph.Measure, error) {
		measuredLayout, err := MeasureDagreLayout(data4Layout, rc)
		if err != nil {
			return nil, err
		}
		return measuredLayout, nil
	},
	RunLayoutCore: func(data4Layout *ru.LayoutData, rc *rugraph.RenderContext) (any, error) {
		measuredLayout, err := RunDagreLayoutCore(data4Layout, rc)
		if err != nil {
			return nil, err
		}
		return measuredLayout, nil
	},
	PaintOptions: paintOptions,
})
