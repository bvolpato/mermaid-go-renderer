// Ported from mermaid 12.2.0, rendering-util/layout-algorithms/elk/render.ts
// (MIT).

// Package elk is the port of mermaid's ELK layout
// (rendering-util/layout-algorithms/elk): the default layout of flowchart,
// state, class, ER and requirement diagrams in mermaid 12.
//
// The layout has two halves that need no DOM. buildElkGraphFromLayoutData
// makes the ELK input graph from the measured layout data. The genuine
// elkjs lays it out (internal/elk). applyElkLayoutResult reads the result
// back: the node positions, and the edge routes with their clipping at the
// node shapes, the straightening of the ends and the label placement.
//
// Upstream hands ELK copies of its node and edge objects, and ELK writes
// the result into the same objects. Here the graph is a tree of js.Obj
// values with the property order of upstream, which goes to ELK as JSON.
// The result comes back as JSON and is decoded into NodeWithVertex and
// ElkEdge values (decodeElkResult).
package elk

import (
	"context"
	"math"
	"regexp"
	"strings"

	elkjs "github.com/bvolpato/mermaid-go-renderer/internal/elk"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	rugraph "github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// Node and Edge are the node and the edge of the layout data.
type (
	Node = ru.Node
	Edge = ru.Edge
)

// LabelData is the measured label of a group.
type LabelData struct {
	Width  float64
	Height float64
	// WrappingWidth is NaN when undefined.
	WrappingWidth float64
}

// ElkNodeOffset is where ELK put a node, in absolute coordinates.
type ElkNodeOffset struct {
	PosX   float64
	PosY   float64
	X      float64
	Y      float64
	Depth  int
	Width  float64
	Height float64
}

// ElkOrigin is where ELK put a container, kept when evenGroupFrames moves
// the drawn frame.
type ElkOrigin struct {
	PosX float64
	PosY float64
}

// ElkLabel is a label of a node or an edge of the ELK graph, with the
// position that ELK gave it. A number that is undefined is NaN.
type ElkLabel struct {
	// Terminal is startLeft, startRight, endLeft or endRight for a terminal
	// label of an edge. It is empty for other labels.
	Terminal string
	X        float64
	Y        float64
	Width    float64
	Height   float64
}

// NodeWithVertex is a node of the ELK graph: the copy of a layout node that
// ELK lays out, and one entry of nodeDb. A number that is undefined is NaN,
// and a string that is undefined is empty.
type NodeWithVertex struct {
	ID      string
	IsGroup bool
	// ElkOrigin is where ELK put this container, kept when evenGroupFrames
	// moves the drawn frame. Edge sections resolve against this, never
	// against the moved frame.
	ElkOrigin *ElkOrigin
	Padding   float64
	ParentID  string
	Shape     string
	Width     float64
	Height    float64
	X         float64
	Y         float64
	// Children is nil when the node has no children property.
	Children  []*NodeWithVertex
	LabelData *LabelData
	// Labels is nil when the node has no labels property.
	Labels []*ElkLabel
	Offset *ElkNodeOffset

	// Intersect is the intersect function of the layout node at the time
	// the graph was built, or nil.
	Intersect func(point P) P

	// labelData and labels are the values of the properties in the result
	// of ELK. applyElkNodePositions copies them to the layout node.
	labelData any
	labels    any
	hasLabels bool
}

func (n *NodeWithVertex) intersectFunc() func(point P) P {
	if n == nil {
		return nil
	}
	return n.Intersect
}

// ElkSection is a section of the route of an edge.
type ElkSection struct {
	StartPoint P
	EndPoint   P
	// BendPoints is nil when the section has none.
	BendPoints []P
}

// ElkEdge is an edge of the ELK graph after the layout. An id that is
// undefined is empty.
type ElkEdge struct {
	ID string
	// Source and Target are sources[0] and targets[0].
	Source string
	Target string
	// Start, End, SourceID and TargetID are the properties start, end,
	// sourceId and targetId of the edge.
	Start    string
	End      string
	SourceID string
	TargetID string
	// LayoutReversed says that the edge was handed to ELK from its target
	// to its source.
	LayoutReversed bool
	// Sections is nil when the edge has no sections property.
	Sections []*ElkSection
	// Labels is nil when the edge has no labels property.
	Labels []*ElkLabel
}

// ElkPreparedLayout is the result of the prepare phase.
type ElkPreparedLayout struct {
	// Algorithm is empty when undefined.
	Algorithm string
}

// ElkLayoutContext is what the layout functions get from the render.
type ElkLayoutContext struct {
	// Algorithm is the ELK algorithm of the layout, for example
	// "elk.layered". It is empty when undefined.
	Algorithm string
	// RootLayoutOptions are extra layoutOptions of the root graph, merged
	// last over the defaults of createRootElkGraph. It is not user-facing
	// config: the render never sets it.
	RootLayoutOptions *js.Obj[any]
	// GetConfig is getConfig of the mermaid helpers.
	GetConfig func() *js.Obj[any]
	// ctx is the render. It is nil in tests of the layout without a DOM.
	ctx *render.Context
}

// await is one await of upstream. See internal/render/async.go. It does
// nothing outside ctx.All.
func (c *ElkLayoutContext) await() {
	if c.ctx != nil {
		c.ctx.Await()
	}
}

// awaitTimeout is the wait for a timer of upstream.
func (c *ElkLayoutContext) awaitTimeout() {
	if c.ctx != nil {
		c.ctx.AwaitTimeout()
	}
}

// ElkLayoutState is the ELK graph of a layout with its lookup tables.
type ElkLayoutState struct {
	// elkGraph is the graph that goes to ELK.
	elkGraph *js.Obj[any]
	// nodeObjDb is nodeDb while the graph is built: the node objects of
	// elkGraph by id.
	nodeObjDb map[string]*js.Obj[any]
	// nodeDb is nodeDb after the layout: the nodes of the result by id.
	nodeDb         map[string]*NodeWithVertex
	parentLookupDb *TreeData
	// nodeIndex has, for each node object of elkGraph, the index of the
	// layout node that it is a copy of.
	nodeIndex map[*js.Obj[any]]int
}

// ElkLayoutResult is the graph that ELK returns.
type ElkLayoutResult struct {
	// Children and Edges are nil when the graph has no such property.
	Children []*NodeWithVertex
	Edges    []*ElkEdge
}

// Side is "start" or "end".
type Side = string

const MIN_END_MARKER_SEGMENT_LENGTH = 8

// markerPathOffset is how far getLineFunctionsWithOffset pulls a path end
// back along its last segment to make room for this marker. It reads the
// same tables that the edge drawing uses.
func markerPathOffset(arrowType string) float64 {
	a, ok := utils.MarkerOffset(arrowType)
	if !ok {
		a = 0
	}
	b, ok := utils.MarkerOffset2(arrowType)
	if !ok {
		b = 0
	}
	return js.Max(a, b)
}

// ARROW_MAP is [arrowTypeStart, arrowTypeEnd] per edge type. "No arrowhead
// on this end" is spelled `none`.
var ARROW_MAP = map[string][2]string{
	"arrow_open":          {"none", "none"},
	"arrow_cross":         {"none", "arrow_cross"},
	"double_arrow_cross":  {"arrow_cross", "arrow_cross"},
	"arrow_point":         {"none", "arrow_point"},
	"double_arrow_point":  {"arrow_point", "arrow_point"},
	"arrow_circle":        {"none", "arrow_circle"},
	"double_arrow_circle": {"arrow_circle", "arrow_circle"},
}

// PORTS_SURROUNDING_MARGIN is the margin reserved at the ends of each side
// of a node, so that a port cannot be placed on a corner.
// alignDegenerateNodeToAnchor treats a side shorter than twice this as
// having no usable anchor span.
const PORTS_SURROUNDING_MARGIN = 12

// PORTS_SURROUNDING is the margin spelled as an ELK margin.
var PORTS_SURROUNDING = "[top=" + js.Str(PORTS_SURROUNDING_MARGIN) + ",left=" + js.Str(PORTS_SURROUNDING_MARGIN) + ",bottom=" + js.Str(PORTS_SURROUNDING_MARGIN) + ",right=" + js.Str(PORTS_SURROUNDING_MARGIN) + "]"

// SUBGRAPH_PADDING is the padding between a subgraph frame and its
// children. The default of ELK is 12.
const SUBGRAPH_PADDING = 24

// DEFAULT_SUBGRAPH_SPACING_BASE_VALUE is the default spacing.baseValue for
// a subgraph that has no algorithm of its own.
const DEFAULT_SUBGRAPH_SPACING_BASE_VALUE = 24

// DEFAULT_SUBGRAPH_NODE_SPACING is the gap between two sibling nodes in a
// subgraph.
const DEFAULT_SUBGRAPH_NODE_SPACING = 50

// CONTAINER_PADDING is the inner padding reserved around a container that
// runs its own algorithm.
const CONTAINER_PADDING = 15

// RECTPACKING_CONTAINER_PADDING is the same, for elk.rectpacking.
const RECTPACKING_CONTAINER_PADDING = 10

// RECTPACKING_OPTIONS returns the shared layout options for
// elk.rectpacking, applied at both the root level and the group level.
func RECTPACKING_OPTIONS() *js.Obj[any] {
	return obj(
		"spacing.baseValue", 15.0,
		"spacing.nodeNode", 15.0,
		"elk.aspectRatio", "1.6",
		"elk.expandNodes", "true",
		"elk.rectpacking.trybox", "true",
		"elk.rectpacking.packing.compaction.rowHeightReevaluation", "true",
		"elk.rectpacking.packing.compaction.iterations", 10.0,
		"elk.rectpacking.whiteSpaceElimination.strategy", "EQUAL_BETWEEN_STRUCTURES",
		"elk.rectpacking.widthApproximation.strategy", "SCANLINE",
	)
}

// CONTAINER_ALGORITHM_OVERRIDES has every option that
// buildSubgraphLayoutOptions sets or overrides because a container asked
// for its own algorithm. When cross-boundary edges force the container back
// onto the inherited algorithm, all of these have to go.
var CONTAINER_ALGORITHM_OVERRIDES = append([]string{
	"nodeSize.constraints",
	"nodeSize.minimum",
	"elk.algorithm",
	"elk.aspectRatio",
	"elk.contentAlignment",
	"elk.expandNodes",
	"elk.padding",
}, RECTPACKING_OPTIONS().Keys()...)

// clearContainerAlgorithmOptions undoes the algorithm-scoped options on a
// container, restoring the values a plain subgraph would have had.
func clearContainerAlgorithmOptions(layoutOptions *js.Obj[any]) {
	for _, key := range CONTAINER_ALGORITHM_OVERRIDES {
		layoutOptions.Delete(key)
	}
	// spacing.baseValue and spacing.nodeNode are base options that the
	// rectpacking overrides stomp on, so restore the defaults rather than
	// leaving them unset.
	layoutOptions.Set("spacing.baseValue", float64(DEFAULT_SUBGRAPH_SPACING_BASE_VALUE))
	layoutOptions.Set("spacing.nodeNode", float64(DEFAULT_SUBGRAPH_NODE_SPACING))
}

// CONTAINER_ALGORITHMS are the ELK algorithm ids a container may select
// through `@{ algorithm: … }`. Anything outside this list is ignored, so a
// typo degrades to the default layout instead of losing the render.
var CONTAINER_ALGORITHMS = js.NewSet(
	"elk.layered",
	"elk.box",
	"elk.rectpacking",
	"elk.stress",
	"elk.force",
	"elk.mrtree",
	"elk.radial",
	"elk.sporeOverlap",
)

// resolveContainerAlgorithm resolves the requested layout algorithm of a
// container. The result is "" (undefined) when the request is absent, not
// a string, or not a supported ELK algorithm.
func resolveContainerAlgorithm(requested any) string {
	s, ok := requested.(string)
	if !ok {
		return ""
	}
	if !CONTAINER_ALGORITHMS.Has(s) {
		// Upstream logs a warning here.
		return ""
	}
	return s
}

func dir2ElkDirection(dir any) string {
	switch dir {
	case "LR":
		return "RIGHT"
	case "RL":
		return "LEFT"
	case "TB", "TD": // TD is an alias for TB in Mermaid
		return "DOWN"
	case "BT":
		return "UP"
	default:
		return "DOWN"
	}
}

// GroupTitleNode is what groupTitleWidth reads from a node. A number that
// is undefined is NaN.
type GroupTitleNode struct {
	Shape string
	// LabelDataWidth is labelData?.width.
	LabelDataWidth float64
	// LabelsWidth is labels?.[0]?.width.
	LabelsWidth float64
	Padding     float64
}

// groupTitleNodeOf reads a GroupTitleNode from a node object.
func groupTitleNodeOf(node *js.Obj[any]) GroupTitleNode {
	g := GroupTitleNode{LabelDataWidth: ru.Undef, LabelsWidth: ru.Undef, Padding: ru.Undef}
	g.Shape, _ = prop(node, "shape").(string)
	if v := prop(prop(node, "labelData"), "width"); !js.IsNullish(v) {
		g.LabelDataWidth = num(v)
	}
	if labels, ok := prop(node, "labels").([]any); ok && len(labels) > 0 {
		if v := prop(labels[0], "width"); !js.IsNullish(v) {
			g.LabelsWidth = num(v)
		}
	}
	if v := prop(node, "padding"); !js.IsNullish(v) {
		g.Padding = num(v)
	}
	return g
}

func (n *NodeWithVertex) groupTitleNode() GroupTitleNode {
	g := GroupTitleNode{Shape: n.Shape, LabelDataWidth: ru.Undef, LabelsWidth: ru.Undef, Padding: n.Padding}
	if n.LabelData != nil {
		g.LabelDataWidth = n.LabelData.Width
	}
	if len(n.Labels) > 0 {
		g.LabelsWidth = n.Labels[0].Width
	}
	return g
}

// groupTitleWidth is the width the frame painter needs for the title of the
// group, or 0 for cluster shapes that paint no title.
func groupTitleWidth(node GroupTitleNode) float64 {
	if !clusters.ClusterPaintsTitle(node.Shape) {
		return 0
	}
	// Match the frame painter's label width plus total horizontal padding.
	return ru.NN(ru.NN(node.LabelDataWidth, node.LabelsWidth), 0) + ru.NN(node.Padding, 0)
}

// groupTitleSizeOptions returns the ELK options reserving the painted title
// width before routing. It is empty for cluster shapes that paint no title,
// so they keep the default sizing of ELK.
func groupTitleSizeOptions(node GroupTitleNode) *js.Obj[any] {
	if !clusters.ClusterPaintsTitle(node.Shape) {
		return js.NewObj[any]()
	}
	return obj(
		"nodeSize.constraints", "[MINIMUM_SIZE, NODE_LABELS]",
		"nodeSize.minimum", "("+js.Str(groupTitleWidth(node))+", 0)",
	)
}

// buildSubgraphLayoutOptions returns the layout options of a subgraph. node
// is the node object of the ELK graph, elkConfig is config.elk, and
// algorithm is the algorithm of the layout ("" when undefined).
func buildSubgraphLayoutOptions(node *js.Obj[any], elkConfig any, algorithm string) *js.Obj[any] {
	// Every group gets its painted title width as a floor via
	// groupTitleSizeOptions below. Containers that run their own algorithm
	// override that floor with this wider minimum: the label plus both
	// paddings.
	labelData := prop(node, "labelData")
	labelW := num(nn(prop(labelData, "width"), 0.0))
	pad := num(nn(prop(node, "padding"), 0.0))
	minWidth := labelW + float64(2*pad)
	labelH := num(nn(prop(labelData, "height"), 0.0))
	preset := resolveElkPreset(prop(elkConfig, "preset"))

	layoutOptions := js.NewObj[any]()
	// Reserve the painted title width before routing.
	assign(layoutOptions, groupTitleSizeOptions(groupTitleNodeOf(node)))
	layoutOptions.Set("spacing.baseValue", float64(DEFAULT_SUBGRAPH_SPACING_BASE_VALUE))
	// The straight run an edge gets before the node it enters.
	layoutOptions.Set("elk.layered.spacing.edgeNodeBetweenLayers", 30.0)
	// Separation between edges sharing a lane.
	layoutOptions.Set("elk.spacing.edgeEdge", 20.0)
	// Node separation.
	layoutOptions.Set("spacing.nodeNode", float64(DEFAULT_SUBGRAPH_NODE_SPACING))
	// Breathing room between a frame and its children.
	layoutOptions.Set("elk.padding", "[top="+js.Str(SUBGRAPH_PADDING)+",left="+js.Str(SUBGRAPH_PADDING)+",bottom="+js.Str(SUBGRAPH_PADDING)+",right="+js.Str(SUBGRAPH_PADDING)+"]")
	layoutOptions.Set("nodeLabels.placement", "[H_CENTER V_TOP, INSIDE]")

	layoutOptions.Set("elk.layered.mergeEdges", prop(elkConfig, "mergeEdges"))
	layoutOptions.Set("elk.layered.nodePlacement.bk.fixedAlignment", nn(prop(elkConfig, "nodePlacementAlignment"), preset.alignment))
	// The preset resolves child placement separately from root placement.
	layoutOptions.Set("elk.layered.nodePlacement.strategy", nn(prop(elkConfig, "nodePlacementStrategy"), preset.containerPlacement))
	// Resolved here as well as at the root, because a container laid out on
	// its own never sees the value of the root.
	layoutOptions.Set("elk.layered.cycleBreaking.strategy", nn(prop(elkConfig, "cycleBreakingStrategy"), preset.cycleBreaking))
	// PORT_POSITION lets a node shift so an edge can leave straight.
	layoutOptions.Set("elk.layered.nodePlacement.networkSimplex.nodeFlexibility", "PORT_POSITION")
	// Keep the ports of a frame off its own corners.
	layoutOptions.Set("elk.spacing.portsSurrounding", PORTS_SURROUNDING)

	// Apply the per-group algorithm from metadata (for example
	// @{algorithm: elk.box}). SEPARATE_CHILDREN is required so that the
	// algorithm of the subgraph runs.
	algo := resolveContainerAlgorithm(prop(prop(node, "metadata"), "algorithm"))
	if algo != "" {
		// These algorithms also need a minimum height for their title strip.
		padTop := labelH + CONTAINER_PADDING
		layoutOptions.Set("nodeSize.constraints", "[MINIMUM_SIZE, NODE_LABELS]")
		// The minimum has to clear the whole reserved strip.
		layoutOptions.Set("nodeSize.minimum", "("+js.Str(minWidth)+", "+js.Str(padTop+CONTAINER_PADDING)+")")
		layoutOptions.Set("elk.algorithm", algo)
		layoutOptions.Set("elk.hierarchyHandling", "SEPARATE_CHILDREN")
		layoutOptions.Set("elk.aspectRatio", "2.0")
		layoutOptions.Set("elk.contentAlignment", "H_CENTER V_TOP")
		layoutOptions.Set("elk.expandNodes", "true")
		// Reserve top padding for the label so children do not overlap it.
		layoutOptions.Set("elk.padding", "[top="+js.Str(padTop)+",left="+js.Str(CONTAINER_PADDING)+",bottom="+js.Str(CONTAINER_PADDING)+",right="+js.Str(CONTAINER_PADDING)+"]")

		// Tighter spacing for rectpacking.
		if algo == "elk.rectpacking" {
			rectPadTop := labelH + RECTPACKING_CONTAINER_PADDING
			assign(layoutOptions, RECTPACKING_OPTIONS())
			assign(layoutOptions, obj(
				"elk.padding", "[top="+js.Str(rectPadTop)+",left="+js.Str(RECTPACKING_CONTAINER_PADDING)+",bottom="+js.Str(RECTPACKING_CONTAINER_PADDING)+",right="+js.Str(RECTPACKING_CONTAINER_PADDING)+"]",
				"nodeSize.minimum", "("+js.Str(minWidth)+", "+js.Str(rectPadTop+RECTPACKING_CONTAINER_PADDING)+")",
			))
		}
	} else if js.Truthy(prop(node, "dir")) {
		// Directional subgraph without explicit algorithm: run the parent
		// layered algorithm in the coordinate system of the subgraph.
		layoutOptions.Set("elk.algorithm", strOrUndef(algorithm))
		layoutOptions.Set("elk.direction", dir2ElkDirection(prop(node, "dir")))
		layoutOptions.Set("elk.hierarchyHandling", "SEPARATE_CHILDREN")
	}

	// Container-scoped experiments.
	assign(layoutOptions, SUBGRAPH_EXPERIMENT_OVERRIDES)

	return layoutOptions
}

// strOrUndef is the dynamic value of a string that is empty when undefined.
func strOrUndef(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// cyclicEntryNode is what findCyclicEntryNodes reads from a node.
type cyclicEntryNode struct {
	id string
	// parentId is "" with hasParent false when undefined.
	parentId  string
	hasParent bool
}

// cyclicEntryEdge is what findCyclicEntryNodes reads from an edge. An end
// that is undefined or null has ok false.
type cyclicEntryEdge struct {
	source, target     string
	sourceOk, targetOk bool
}

// findCyclicEntryNodes identifies the entry node of each recursive flow so
// it can be pinned to the top.
//
// For each container (grouped by parentId) it looks only at edges internal
// to that container and finds its weakly-connected components. A component
// with no natural source (no node with in-degree 0 once self-loops are
// ignored) must contain a cycle. For such components it breaks cycles
// greedily in edge declaration order: an edge that would close a directed
// cycle is treated as a back-edge and skipped, and the entry is the first
// node in declaration order that is a source of the remaining forward
// edges.
//
// The result has the ids of the nodes to constrain to the first layer.
func findCyclicEntryNodes(nodes []cyclicEntryNode, edges []cyclicEntryEdge) *js.Set[string] {
	entries := js.NewSet[string]()

	// Group node ids by container, preserving declaration order within each group.
	type groupKey struct {
		parentId  string
		hasParent bool
	}
	groups := js.NewMap[groupKey, []string]()
	for _, n := range nodes {
		key := groupKey{n.parentId, n.hasParent}
		groups.Set(key, append(groups.Get(key), n.id))
	}

	for _, ids := range groups.Values() {
		idSet := map[string]bool{}
		inDegree := map[string]int{}
		// Undirected adjacency, used only to find weakly-connected components.
		neighbors := map[string][]string{}
		for _, id := range ids {
			idSet[id] = true
			inDegree[id] = 0
			neighbors[id] = nil
		}
		// Container-internal directed edges in declaration order, for the
		// cycle-breaking fallback below.
		var internalEdges [][2]string

		for _, edge := range edges {
			source, target := edge.source, edge.target
			// Restrict to edges internal to this container; ignore self-loops.
			if !edge.sourceOk || !edge.targetOk || source == "" || target == "" || source == target {
				continue
			}
			if !idSet[source] || !idSet[target] {
				continue
			}
			inDegree[target] = inDegree[target] + 1
			neighbors[source] = append(neighbors[source], target)
			neighbors[target] = append(neighbors[target], source)
			internalEdges = append(internalEdges, [2]string{source, target})
		}

		// Label weakly-connected components.
		component := map[string]int{}
		componentCount := 0
		for _, id := range ids {
			if _, has := component[id]; has {
				continue
			}
			stack := []string{id}
			component[id] = componentCount
			for len(stack) > 0 {
				current := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				for _, next := range neighbors[current] {
					if _, has := component[next]; !has {
						component[next] = componentCount
						stack = append(stack, next)
					}
				}
			}
			componentCount++
		}

		// A component with no in-degree-0 node necessarily contains a cycle.
		hasSource := make([]bool, componentCount)
		for _, id := range ids {
			if inDegree[id] == 0 {
				hasSource[component[id]] = true
			}
		}
		allHaveSource := true
		for _, h := range hasSource {
			if !h {
				allHaveSource = false
			}
		}
		if allHaveSource {
			continue
		}

		// Recover the entry of each source-less component by breaking cycles
		// greedily in edge declaration order: skip any edge that would
		// close a directed cycle (a back-edge).
		forward := map[string][]string{}
		residualInDegree := map[string]int{}
		for _, id := range ids {
			forward[id] = nil
			residualInDegree[id] = 0
		}
		reaches := func(from, to string) bool {
			seen := map[string]bool{from: true}
			stack := []string{from}
			for len(stack) > 0 {
				current := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if current == to {
					return true
				}
				for _, next := range forward[current] {
					if !seen[next] {
						seen[next] = true
						stack = append(stack, next)
					}
				}
			}
			return false
		}
		for _, e := range internalEdges {
			source, target := e[0], e[1]
			if reaches(target, source) {
				continue
			}
			forward[source] = append(forward[source], target)
			residualInDegree[target] = residualInDegree[target] + 1
		}

		nominated := make([]bool, componentCount)
		for _, id := range ids {
			c := component[id]
			if !hasSource[c] && !nominated[c] && residualInDegree[id] == 0 {
				entries.Add(id)
				nominated[c] = true
			}
		}
	}

	return entries
}

// applyCyclicEntryConstraint pins the entry node of each recursive flow to
// the first layer when elk.keepEntryNodeOnTop is enabled.
func applyCyclicEntryConstraint(data4Layout *layoutObjects, nodeDb map[string]*js.Obj[any]) {
	if !js.Truthy(js.Path(data4Layout.config, "elk", "keepEntryNodeOnTop")) {
		return
	}

	nodes := make([]cyclicEntryNode, len(data4Layout.nodes))
	for i, node := range data4Layout.nodes {
		id, _ := idOf(prop(node, "id"))
		parentId := prop(node, "parentId")
		n := cyclicEntryNode{id: id}
		if parentId != nil {
			n.parentId, n.hasParent = js.ToString(parentId), true
		}
		nodes[i] = n
	}
	edges := make([]cyclicEntryEdge, len(data4Layout.edges))
	for i, edge := range data4Layout.edges {
		var e cyclicEntryEdge
		e.source, e.sourceOk = idOf(prop(edge, "start"))
		e.target, e.targetOk = idOf(prop(edge, "end"))
		edges[i] = e
	}
	entryNodeIds := findCyclicEntryNodes(nodes, edges)

	for _, id := range entryNodeIds.Values() {
		elkNode := nodeDb[id]
		if elkNode != nil {
			layoutOptions := spread(js.AsObj(elkNode.Get("layoutOptions")))
			layoutOptions.Set("elk.layered.layering.layerConstraint", "FIRST")
			elkNode.Set("layoutOptions", layoutOptions)
		}
	}
}

func prepareLayoutForElk(data4Layout *ru.LayoutData, context *rugraph.RenderContext) (any, error) {
	elkContext := getElkLayoutContext(context)
	if err := syncHostConfig(elkContext); err != nil {
		return nil, err
	}
	applyElkEdgeRenderData(data4Layout, elkContext)
	return &ElkPreparedLayout{Algorithm: elkContext.Algorithm}, nil
}

// elkLayout is elk.layout. Tests replace it.
var elkLayout = func(graphJSON []byte) ([]byte, error) {
	return elkjs.Layout(context.Background(), graphJSON)
}

// errLayoutLimit is the panic value for layout data on which upstream does
// not end in a useful time. The entry points of the layout recover the
// panic and return the error.
type errLayoutLimit string

func (e errLayoutLimit) Error() string { return "elk layout: " + string(e) }

// recoverLayoutError turns the panic of a parent chain with a cycle, or of
// a limit of the layout, into an error. Upstream does not terminate for
// such data.
func recoverLayoutError(err *error) {
	if r := recover(); r != nil {
		switch e := r.(type) {
		case errParentCycle:
			*err = e
		case errLayoutLimit:
			*err = e
		default:
			panic(r)
		}
	}
}

// runElkLayoutCore builds the ELK graph, lays it out with one call of ELK
// and writes the result into the layout data.
//
// It is an async function upstream, and so is runElkLayout. Neither has a
// Promise.all: they run in sequence. Each one yields before its return, as
// the convention of internal/render/async.go asks, for a caller that runs
// several layouts at once.
func runElkLayoutCore(data4Layout *ru.LayoutData, context *rugraph.RenderContext) (result any, err error) {
	defer recoverLayoutError(&err)
	elkContext := getElkLayoutContext(context)
	layoutState := buildElkGraphFromLayoutData(newLayoutObjects(data4Layout), elkContext)

	graph, err := runElkLayout(elkContext, layoutState, data4Layout)
	if err != nil {
		return nil, err
	}
	applyElkLayoutResult(data4Layout, graph, layoutState)
	orderNodesForElkPaint(data4Layout)
	elkContext.await()
	return graph, nil
}

func buildElkGraphFromLayoutData(data4Layout *layoutObjects, elkContext *ElkLayoutContext) *ElkLayoutState {
	nodeDb := map[string]*js.Obj[any]{}
	nodeIndex := map[*js.Obj[any]]int{}
	elkGraph := createRootElkGraph(data4Layout, elkContext.Algorithm, elkContext.RootLayoutOptions)

	dir := nn(data4Layout.direction, "DOWN")
	js.AsObj(elkGraph.Get("layoutOptions")).Set("elk.direction", dir2ElkDirection(dir))

	parentLookupDb := addSubGraphs(data4Layout.nodes)
	addVertices(data4Layout.nodes, elkGraph, nodeDb, nodeIndex, elkContext, nil, 0)
	addEdgesToElkGraph(data4Layout, elkGraph, nodeDb, elkContext)
	configureSubgraphNodes(data4Layout, nodeDb, parentLookupDb, elkContext)
	configureCrossHierarchyEdges(elkGraph, nodeDb, parentLookupDb)
	if !strictEq(js.Path(data4Layout.config, "elk", "orientFeedbackEdges"), false) {
		reverseSubgraphFeedbackEdges(elkGraph, parentLookupDb)
	}
	applyCyclicEntryConstraint(data4Layout, nodeDb)

	return &ElkLayoutState{elkGraph: elkGraph, nodeObjDb: nodeDb, parentLookupDb: parentLookupDb, nodeIndex: nodeIndex}
}

// firstID is ids[0] as an id: "" when the array is empty or the element is
// undefined or null.
func firstID(ids any) string {
	arr, _ := ids.([]any)
	if len(arr) == 0 {
		return ""
	}
	id, _ := idOf(arr[0])
	return id
}

// reverseSubgraphFeedbackEdges hands ELK the edges that re-enter a subgraph
// the other way round, so that ELK sees no cycle through the collapsed
// subgraph and routes them downstream instead of around it.
// applyElkEdgeLayout restores the direction.
func reverseSubgraphFeedbackEdges(elkGraph *js.Obj[any], parentLookupDb *TreeData) {
	edges, _ := elkGraph.Get("edges").([]any)
	ends := make([]EdgeEnds, len(edges))
	for i, edge := range edges {
		ends[i] = EdgeEnds{Source: firstID(prop(edge, "sources")), Target: firstID(prop(edge, "targets"))}
	}
	flags := findSubgraphFeedbackEdges(ends, parentLookupDb)
	for index, e := range edges {
		if flags[index] {
			edge := js.AsObj(e)
			sources, targets := edge.Get("sources"), edge.Get("targets")
			edge.Set("sources", targets)
			edge.Set("targets", sources)
			edge.Set("layoutReversed", true)
		}
	}
}

// Render is the render function of the ELK layouts.
var Render = rugraph.CreateCommonLayoutRenderer(rugraph.Definition{
	AfterPaint:    applyElkLineJumps,
	PrepareLayout: prepareLayoutForElk,
	// ELK derives the minimum size of a compound node from the measured
	// cluster label, so the label has to be measured the way insertCluster
	// paints it: unwrapped.
	MeasureLayout: func(data4Layout *ru.LayoutData, context *rugraph.RenderContext) (rugraph.Measure, error) {
		measure, err := rugraph.DefaultMeasureLayout(data4Layout, context, rugraph.CreateGraphOptions{UnwrapGroupLabels: true})
		if err != nil {
			return nil, err
		}
		return measure, nil
	},
	RunLayoutCore: runElkLayoutCore,
	PaintOptions: &rugraph.PaintOptions{
		SkipIntersect: true,
	},
})

// syncHostConfig copies the config of the host into the config module that
// this bundle uses: setConfig(getConfig()). Compiled into mermaid itself
// the two are the same module. The capture of the reference fixtures shows
// that the config is the same after the call, for every fixture.
func syncHostConfig(elkContext *ElkLayoutContext) error {
	ctx := elkContext.ctx
	if ctx == nil || ctx.ConfigState == nil {
		return nil
	}
	// setConfig sanitizes its argument in place, so it gets a copy.
	if _, err := ctx.ConfigState.SetConfig(ctx.ConfigState.GetConfig()); err != nil {
		return err
	}
	ctx.Config = ctx.ConfigState.GetConfig()
	return nil
}

func orderNodesForElkPaint(data4Layout *ru.LayoutData) {
	nodes := data4Layout.Nodes
	nodeById := map[string]*Node{}
	for _, node := range nodes {
		nodeById[node.ID] = node
	}

	js.SortFunc(nodes, func(a, b *Node) float64 {
		if a.IsGroup != b.IsGroup {
			if a.IsGroup {
				return -1
			}
			return 1
		}

		if a.IsGroup && b.IsGroup {
			return float64(getGroupDepth(a, nodeById) - getGroupDepth(b, nodeById))
		}

		return 0
	})
}

func getGroupDepth(node *Node, nodeById map[string]*Node) int {
	depth := 0
	visited := map[string]bool{}
	parentId := node.ParentID

	for parentId != "" && !visited[parentId] {
		visited[parentId] = true
		parent := nodeById[parentId]
		if parent == nil || !parent.IsGroup {
			break
		}
		depth++
		parentId = parent.ParentID
	}

	return depth
}

func getElkLayoutContext(context *rugraph.RenderContext) *ElkLayoutContext {
	algorithm := context.Options.Algorithm
	if prepared, ok := context.PreparedLayout.(*ElkPreparedLayout); ok && prepared != nil && prepared.Algorithm != "" {
		algorithm = prepared.Algorithm
	}
	ctx := context.Ctx
	return &ElkLayoutContext{
		Algorithm: algorithm,
		// ctx.Config is the result of getConfig().
		GetConfig: func() *js.Obj[any] { return ctx.Config },
		ctx:       ctx,
	}
}

// elkPreset is one entry of ELK_PRESETS.
type elkPreset struct {
	layering           string
	placement          string
	containerPlacement string
	alignment          string
	cycleBreaking      string
}

// ELK_PRESETS supply layout choices only when the caller leaves them
// unspecified. Root and container placement are independent, while both
// use the Brandes-Koepf alignment of the preset.
var ELK_PRESETS = map[string]elkPreset{
	// Balanced Brandes-Koepf centers simple branches and composite-state
	// entries.
	"default": {
		layering:           "NETWORK_SIMPLEX",
		placement:          "BRANDES_KOEPF",
		containerPlacement: "BRANDES_KOEPF",
		alignment:          "BALANCED",
		cycleBreaking:      "DEPTH_FIRST",
	},
	// Reproduce the layout before presets.
	"legacy": {
		layering:           "NETWORK_SIMPLEX",
		placement:          "BRANDES_KOEPF",
		containerPlacement: "BRANDES_KOEPF",
		alignment:          "NONE",
		cycleBreaking:      "GREEDY",
	},
	"modelOrder": {
		layering:           "NETWORK_SIMPLEX",
		placement:          "NETWORK_SIMPLEX",
		containerPlacement: "BRANDES_KOEPF",
		alignment:          "NONE",
		cycleBreaking:      "GREEDY_MODEL_ORDER",
	},
	// Preserve the previous default recipe for callers selecting it by name.
	"depthFirst": {
		layering:           "NETWORK_SIMPLEX",
		placement:          "NETWORK_SIMPLEX",
		containerPlacement: "BRANDES_KOEPF",
		alignment:          "NONE",
		cycleBreaking:      "DEPTH_FIRST",
	},
}

// resolveElkPreset resolves a preset name, falling back to `default` for an
// unknown one.
func resolveElkPreset(name any) elkPreset {
	if name != nil {
		if preset, ok := ELK_PRESETS[js.ToString(name)]; ok {
			return preset
		}
	}
	return ELK_PRESETS["default"]
}

func createRootElkGraph(data4Layout *layoutObjects, algorithm string, rootLayoutOptions *js.Obj[any]) *js.Obj[any] {
	elk := prop(data4Layout.config, "elk")
	preset := resolveElkPreset(prop(elk, "preset"))
	layoutOptions := obj(
		"elk.hierarchyHandling", "INCLUDE_CHILDREN",
		"elk.algorithm", strOrUndef(algorithm),
		"elk.layered.nodePlacement.strategy", nn(prop(elk, "nodePlacementStrategy"), preset.placement),
		"elk.layered.nodePlacement.bk.fixedAlignment", nn(prop(elk, "nodePlacementAlignment"), preset.alignment),
		"elk.layered.mergeEdges", prop(elk, "mergeEdges"),
		"elk.direction", "DOWN",
		"spacing.baseValue", 40.0,

		"elk.layered.crossingMinimization.forceNodeModelOrder", prop(elk, "forceNodeModelOrder"),
		"elk.layered.considerModelOrder.strategy", prop(elk, "considerModelOrder"),
		"elk.layered.unnecessaryBendpoints", true,
		"elk.layered.cycleBreaking.strategy", nn(prop(elk, "cycleBreakingStrategy"), preset.cycleBreaking),
		"elk.layered.layering.strategy", nn(prop(elk, "layeringStrategy"), preset.layering),
		// Only COFFMAN_GRAHAM reads this; the others ignore it.
		"elk.layered.layering.coffmanGraham.layerBound", prop(elk, "layeringLayerBound"),

		// Tweaking options
		"elk.layered.wrapping.multiEdge.improveCuts", true,
		"elk.layered.wrapping.multiEdge.improveWrappedEdges", true,
		"elk.layered.edgeRouting.selfLoopDistribution", "EQUALLY",
		"elk.layered.mergeHierarchyEdges", true,
		// Reserve a margin at the ends of every side so a port cannot land
		// on a corner.
		"elk.spacing.portsSurrounding", PORTS_SURROUNDING,
	)
	graph := obj(
		"id", "root",
		"layoutOptions", layoutOptions,
		"children", []any{},
		"edges", []any{},
	)

	// Optimize spacing when rectpacking is the root algorithm.
	if algorithm == "elk.rectpacking" {
		assign(layoutOptions, RECTPACKING_OPTIONS())
		assign(layoutOptions, obj(
			"elk.contentAlignment", "H_CENTER V_TOP",
			"elk.padding", "[top=15,left=15,bottom=15,right=15]",
		))
	}

	// Last, so a sweep override beats every default above.
	if rootLayoutOptions != nil {
		assign(layoutOptions, rootLayoutOptions)
	}

	// Hand-run experiments from elkoptioncatalogue.go, last so an option
	// switched on there wins over the preset, config.elk.* and the sweep.
	assign(layoutOptions, PLACEMENT_OPTIONS)
	assign(layoutOptions, EDGE_ROUTING_OPTIONS)
	assign(layoutOptions, ROOT_EXPERIMENT_OVERRIDES)

	return graph
}

func addSubGraphs(nodeArr []*js.Obj[any]) *TreeData {
	parentLookupDb := newTreeData()
	for _, subgraph := range nodeArr {
		if !js.Truthy(prop(subgraph, "isGroup")) {
			continue
		}
		subgraphId := prop(subgraph, "id")
		subgraphKey := js.ToString(subgraphId)
		for _, node := range nodeArr {
			if !strictEq(prop(node, "parentId"), subgraphId) {
				continue
			}
			nodeKey := js.ToString(prop(node, "id"))
			parentLookupDb.ParentByID[nodeKey] = subgraphKey
			parentLookupDb.ChildrenByID[subgraphKey] = append(parentLookupDb.ChildrenByID[subgraphKey], nodeKey)
		}
	}

	return parentLookupDb
}

// addVertices adds the nodes whose parent is parentId to graph. A nil
// parentId stands for undefined: the nodes of the root.
//
// depth is the nesting depth of the parent. Two groups with the same id,
// one inside the other, nest without end: upstream then fails with a stack
// overflow. Here the layout fails with an error.
func addVertices(nodeArr []*js.Obj[any], graph *js.Obj[any], nodeDb map[string]*js.Obj[any], nodeIndex map[*js.Obj[any]]int, elkContext *ElkLayoutContext, parentId any, depth int) *js.Obj[any] {
	if depth > len(nodeArr) {
		panic(errParentCycle{})
	}
	for index, node := range nodeArr {
		if !strictEq(prop(node, "parentId"), parentId) {
			continue
		}
		addVertex(graph, nodeArr, index, nodeDb, nodeIndex, elkContext, depth)
	}
	return graph
}

func addVertex(graph *js.Obj[any], nodeArr []*js.Obj[any], index int, nodeDb map[string]*js.Obj[any], nodeIndex map[*js.Obj[any]]int, elkContext *ElkLayoutContext, depth int) {
	node := nodeArr[index]
	child := createElkNode(node)
	push(graph, "children", child)
	nodeDb[js.ToString(prop(node, "id"))] = child
	nodeIndex[child] = index

	if js.Truthy(prop(node, "isGroup")) {
		child.Set("children", []any{})
		addVertices(nodeArr, child, nodeDb, nodeIndex, elkContext, prop(node, "id"), depth+1)
		child.Set("labelData", getMeasuredLabelData(node, elkContext.GetConfig()))
	}
}

func createElkNode(node *js.Obj[any]) *js.Obj[any] {
	child := spread(node)
	child.Delete("domId")

	if js.Truthy(prop(node, "isGroup")) {
		child.Set("children", []any{})
	} else {
		child.Set("width", nn(prop(node, "width"), 0.0))
		child.Set("height", nn(prop(node, "height"), 0.0))
		if js.Truthy(prop(node, "spreadPorts")) {
			// ELK packs the implicit ports of a fixed-size node tightly
			// around the middle of a side. CENTER alignment spreads them as
			// far as the side allows.
			layoutOptions := spread(js.AsObj(child.Get("layoutOptions")))
			layoutOptions.Set("elk.portAlignment.default", "CENTER")
			child.Set("layoutOptions", layoutOptions)
		}
	}

	return child
}

func getMeasuredLabelData(node *js.Obj[any], config *js.Obj[any]) any {
	existing := prop(node, "labelData")
	if js.Truthy(existing) {
		return existing
	}

	wrappingWidth := nn(prop(node, "wrappingWidth"), js.Path(config, "flowchart", "wrappingWidth"))
	if labelBBox := prop(node, "labelBBox"); js.Truthy(labelBBox) {
		return obj(
			"width", prop(labelBBox, "width"),
			"height", js.Max(0, num(prop(labelBBox, "height"))-2),
			"wrappingWidth", wrappingWidth,
		)
	}

	return obj(
		"width", 0.0,
		"height", 0.0,
		"wrappingWidth", wrappingWidth,
	)
}

func addEdgesToElkGraph(dataForLayout *layoutObjects, graph *js.Obj[any], nodeDb map[string]*js.Obj[any], elkContext *ElkLayoutContext) *js.Obj[any] {
	for _, edge := range dataForLayout.edges {
		// Upstream counts the uses of each edge id here and assigns the id
		// to the edge again. Neither changes the edge.
		source, target, sourceId, targetId := getEdgeStartEndPoint(edge, nodeDb)

		elkEdge := spread(edge)
		elkEdge.Set("sources", []any{source})
		elkEdge.Set("targets", []any{target})
		elkEdge.Set("sourceId", sourceId)
		elkEdge.Set("targetId", targetId)
		labels := []any{
			obj(
				"width", nn(prop(edge, "width"), 0.0),
				"height", nn(prop(edge, "height"), 0.0),
				"orgWidth", nn(prop(edge, "width"), 0.0),
				"orgHeight", nn(prop(edge, "height"), 0.0),
				"text", nn(prop(edge, "label"), ""),
				"layoutOptions", obj(
					"edgeLabels.inline", "true",
					"edgeLabels.placement", "CENTER",
				),
			),
		}
		// Only layered places end labels; the other algorithms leave them at the origin.
		if strictEq(nn(nn(prop(elkContext.RootLayoutOptions, "elk.algorithm"), strOrUndef(elkContext.Algorithm)), "elk.layered"), "elk.layered") {
			labels = append(labels, terminalElkLabels(edge)...)
		}
		elkEdge.Set("labels", labels)
		push(graph, "edges", elkEdge)
	}

	return graph
}

// TERMINAL_LABEL_MARKER_CLEARANCE is half the width of the relation markers
// (their paths span 12px, see markers.js).
const TERMINAL_LABEL_MARKER_CLEARANCE = 6

// terminalElkLabels hands the measured terminal (cardinality) labels to ELK
// as TAIL/HEAD labels so that it reserves room for them beside the ports.
func terminalElkLabels(edge *js.Obj[any]) []any {
	texts := map[string]any{
		"startLeft":  prop(edge, "startLabelLeft"),
		"startRight": prop(edge, "startLabelRight"),
		"endLeft":    prop(edge, "endLabelLeft"),
		"endRight":   prop(edge, "endLabelRight"),
	}
	var out []any
	sizes := js.AsObj(prop(edge, "terminalLabelSizes"))
	if sizes == nil {
		return out
	}
	sizes.Each(func(key string, size any) {
		atStart := strings.HasPrefix(key, "start")
		var marker any
		if atStart {
			marker = prop(edge, "arrowTypeStart")
		} else {
			marker = prop(edge, "arrowTypeEnd")
		}
		// Pad past the marker of the end, which ELK does not know about.
		pad := 0.0
		if js.Truthy(marker) && !strictEq(marker, "none") {
			pad = TERMINAL_LABEL_MARKER_CLEARANCE
		}
		placement := "HEAD"
		if atStart {
			placement = "TAIL"
		}
		out = append(out, obj(
			"terminal", key,
			// ELK skips a label without text.
			"text", texts[key],
			"width", num(prop(size, "width"))+float64(2*pad),
			"height", num(prop(size, "height"))+float64(2*pad),
			"layoutOptions", obj("edgeLabels.placement", placement),
		))
	})
	return out
}

// getEdgeStartEndPoint returns source, target, sourceId and targetId of an
// edge. sourceId and targetId are undefined when a node of the edge is not
// in the graph.
func getEdgeStartEndPoint(edge *js.Obj[any], nodeDb map[string]*js.Obj[any]) (source, target, sourceId, targetId any) {
	sourceId = prop(edge, "start")
	targetId = prop(edge, "end")
	source = sourceId
	target = targetId

	var startNode, endNode *js.Obj[any]
	if js.Truthy(sourceId) {
		startNode = nodeDb[js.ToString(sourceId)]
	}
	if js.Truthy(targetId) {
		endNode = nodeDb[js.ToString(targetId)]
	}

	if startNode == nil || endNode == nil {
		return source, target, nil, nil
	}

	return source, target, sourceId, targetId
}

func configureSubgraphNodes(data4Layout *layoutObjects, nodeDb map[string]*js.Obj[any], parentLookupDb *TreeData, elkContext *ElkLayoutContext) {
	for _, n := range data4Layout.nodes {
		node := nodeDb[js.ToString(prop(n, "id"))]
		if node == nil {
			continue
		}
		if _, has := parentLookupDb.ChildrenByID[js.ToString(prop(node, "id"))]; !has {
			continue
		}

		labelData := prop(node, "labelData")
		node.Set("labels", []any{
			obj(
				"text", prop(node, "label"),
				"width", nn(prop(labelData, "width"), 50.0),
				"height", nn(prop(labelData, "height"), 50.0),
			),
		})
		node.Set("layoutOptions", buildSubgraphLayoutOptions(node, prop(data4Layout.config, "elk"), elkContext.Algorithm))
		node.Delete("x")
		node.Delete("y")
		node.Delete("width")
		node.Delete("height")
	}
}

func configureCrossHierarchyEdges(elkGraph *js.Obj[any], nodeDb map[string]*js.Obj[any], parentLookupDb *TreeData) {
	edges, _ := elkGraph.Get("edges").([]any)
	for _, edge := range edges {
		source := firstID(prop(edge, "sources"))
		target := firstID(prop(edge, "targets"))

		sourceNode, targetNode := nodeDb[source], nodeDb[target]
		if source != "" && target != "" && sourceNode != nil && targetNode != nil && !strictEq(prop(sourceNode, "parentId"), prop(targetNode, "parentId")) {
			ancestorId := findCommonAncestor(source, target, parentLookupDb)
			setIncludeChildrenPolicy(nodeDb, source, ancestorId, 0)
			setIncludeChildrenPolicy(nodeDb, target, ancestorId, 0)
		}
	}
}

// setIncludeChildrenPolicy sets INCLUDE_CHILDREN on a node and on its
// ancestors up to ancestorId. depth counts the steps: a chain of parents
// that is longer than the number of nodes has a cycle.
func setIncludeChildrenPolicy(nodeDb map[string]*js.Obj[any], nodeId string, ancestorId string, depth int) {
	if depth > len(nodeDb) {
		panic(errParentCycle{})
	}
	node := nodeDb[nodeId]

	if node == nil {
		return
	}
	layoutOptions := js.AsObj(node.Get("layoutOptions"))
	if layoutOptions == nil {
		layoutOptions = js.NewObj[any]()
		node.Set("layoutOptions", layoutOptions)
	}

	// If this node has a user-specified custom algorithm (for example
	// elk.box) with SEPARATE_CHILDREN, clear it: cross-boundary edges are
	// incompatible with isolated layout algorithms. Nodes using the default
	// layered algorithm (set via the dir branch) keep theirs.
	if strictEq(layoutOptions.Get("elk.hierarchyHandling"), "SEPARATE_CHILDREN") &&
		resolveContainerAlgorithm(prop(prop(node, "metadata"), "algorithm")) != "" {
		clearContainerAlgorithmOptions(layoutOptions)
		assign(layoutOptions, groupTitleSizeOptions(groupTitleNodeOf(node)))
	}

	layoutOptions.Set("elk.hierarchyHandling", "INCLUDE_CHILDREN")
	parentId := prop(node, "parentId")
	if !strictEq(prop(node, "id"), ancestorId) && js.Truthy(parentId) {
		setIncludeChildrenPolicy(nodeDb, js.ToString(parentId), ancestorId, depth+1)
	}
}

// runElkLayout lays out the graph of layoutState with ELK and decodes the
// result.
//
// Upstream awaits elk.layout(elkGraph). elkjs runs the layout in a timer
// (the postMessage of its FakeWorker is a setTimeout), so the function
// continues after the microtask queue is empty.
func runElkLayout(elkContext *ElkLayoutContext, layoutState *ElkLayoutState, data4Layout *ru.LayoutData) (*ElkLayoutResult, error) {
	elkContext.awaitTimeout()
	out, err := elkLayout([]byte(js.StringifyJSON(layoutState.elkGraph)))
	if err != nil {
		return nil, err
	}
	graph, err := decodeElkResult(out, layoutState, func(index int) func(point P) P {
		if index < 0 || index >= len(data4Layout.Nodes) {
			return nil
		}
		return data4Layout.Nodes[index].Intersect
	})
	if err != nil {
		return nil, err
	}
	elkContext.await()
	return graph, nil
}

// decodeElkResult reads the graph that ELK returns. Upstream gets the
// objects of its own graph back, with the layout written into them, so the
// nodes of the result are the entries of nodeDb. Here the result is JSON:
// its nodes are decoded in the order in which addVertex made them, and
// nodeDb is filled in that order. intersect returns the intersect function
// of the layout node with the given index.
func decodeElkResult(out []byte, layoutState *ElkLayoutState, intersect func(index int) func(point P) P) (*ElkLayoutResult, error) {
	parsed, err := js.ParseJSON(out)
	if err != nil {
		return nil, err
	}
	root := js.AsObj(parsed)
	graph := &ElkLayoutResult{}
	layoutState.nodeDb = map[string]*NodeWithVertex{}

	var decodeNodes func(children any, input any) []*NodeWithVertex
	decodeNodes = func(children any, input any) []*NodeWithVertex {
		arr, ok := children.([]any)
		if !ok {
			return nil
		}
		inputArr, _ := input.([]any)
		nodes := make([]*NodeWithVertex, 0, len(arr))
		for i, c := range arr {
			o := js.AsObj(c)
			if o == nil {
				// applyElkNodePositions skips a child that is not a node.
				nodes = append(nodes, nil)
				continue
			}
			node := &NodeWithVertex{
				IsGroup:   js.Truthy(o.Get("isGroup")),
				Padding:   optNum(o.Get("padding")),
				Width:     optNum(o.Get("width")),
				Height:    optNum(o.Get("height")),
				X:         optNum(o.Get("x")),
				Y:         optNum(o.Get("y")),
				labelData: o.Get("labelData"),
				labels:    o.Get("labels"),
				hasLabels: o.Has("labels"),
			}
			node.ID, _ = idOf(o.Get("id"))
			node.ParentID, _ = idOf(o.Get("parentId"))
			node.Shape, _ = o.Get("shape").(string)
			if ld := js.AsObj(node.labelData); ld != nil {
				node.LabelData = &LabelData{
					Width:         optNum(ld.Get("width")),
					Height:        optNum(ld.Get("height")),
					WrappingWidth: optNum(ld.Get("wrappingWidth")),
				}
			}
			node.Labels = decodeLabels(node.labels)
			var inputNode *js.Obj[any]
			if i < len(inputArr) {
				inputNode = js.AsObj(inputArr[i])
			}
			if index, ok := layoutState.nodeIndex[inputNode]; ok && inputNode != nil {
				node.Intersect = intersect(index)
				// nodeDb[node.id] = child, before the children of the node.
				// A node that addVertex did not make (a child of a node
				// that is not a group) is not in nodeDb.
				layoutState.nodeDb[node.ID] = node
			}
			node.Children = decodeNodes(o.Get("children"), prop(inputNode, "children"))
			nodes = append(nodes, node)
		}
		return nodes
	}
	graph.Children = decodeNodes(prop(root, "children"), prop(layoutState.elkGraph, "children"))

	if edges, ok := prop(root, "edges").([]any); ok {
		graph.Edges = make([]*ElkEdge, 0, len(edges))
		for _, e := range edges {
			o := js.AsObj(e)
			edge := &ElkEdge{
				Source:         firstID(prop(o, "sources")),
				Target:         firstID(prop(o, "targets")),
				LayoutReversed: strictEq(prop(o, "layoutReversed"), true),
			}
			edge.ID, _ = idOf(prop(o, "id"))
			edge.Start, _ = idOf(prop(o, "start"))
			edge.End, _ = idOf(prop(o, "end"))
			edge.SourceID, _ = idOf(prop(o, "sourceId"))
			edge.TargetID, _ = idOf(prop(o, "targetId"))
			if sections, ok := prop(o, "sections").([]any); ok {
				edge.Sections = make([]*ElkSection, 0, len(sections))
				for _, s := range sections {
					section := &ElkSection{
						StartPoint: decodePoint(prop(s, "startPoint")),
						EndPoint:   decodePoint(prop(s, "endPoint")),
					}
					if bendPoints, ok := prop(s, "bendPoints").([]any); ok {
						section.BendPoints = make([]P, len(bendPoints))
						for i, b := range bendPoints {
							section.BendPoints[i] = decodePoint(b)
						}
					}
					edge.Sections = append(edge.Sections, section)
				}
			}
			edge.Labels = decodeLabels(prop(o, "labels"))
			graph.Edges = append(graph.Edges, edge)
		}
	}
	return graph, nil
}

// optNum is a number property: NaN when it is undefined or null.
func optNum(v any) float64 {
	if js.IsNullish(v) {
		return ru.Undef
	}
	return num(v)
}

func decodePoint(v any) P {
	return P{X: optNum(prop(v, "x")), Y: optNum(prop(v, "y"))}
}

// decodeLabels reads a labels property. The result is nil when the value is
// not an array.
func decodeLabels(v any) []*ElkLabel {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	labels := make([]*ElkLabel, len(arr))
	for i, l := range arr {
		label := &ElkLabel{
			X:      optNum(prop(l, "x")),
			Y:      optNum(prop(l, "y")),
			Width:  optNum(prop(l, "width")),
			Height: optNum(prop(l, "height")),
		}
		if terminal := prop(l, "terminal"); js.Truthy(terminal) {
			label.Terminal = js.ToString(terminal)
		}
		labels[i] = label
	}
	return labels
}

func applyElkLayoutResult(data4Layout *ru.LayoutData, graph *ElkLayoutResult, layoutState *ElkLayoutState) {
	nodeById := map[string]*Node{}
	for _, node := range data4Layout.Nodes {
		nodeById[node.ID] = node
	}
	applyElkNodePositions(graph.Children, layoutState, nodeById, 0, 0, 0)
	// Between positions and edges on purpose: boundsFor reads the box set
	// above, and cutter2 clips an edge that ends on a group against it, so
	// an edge attaching to a frame follows the frame when it moves.
	evenGroupFrames(graph.Children, layoutState, nodeById, graph)
	applyElkEdgeLayout(data4Layout, graph, layoutState)
}

// collectDescendantIds adds the ids of the descendants of elkNode to into.
func collectDescendantIds(elkNode *NodeWithVertex, into map[string]bool) map[string]bool {
	if into == nil {
		into = map[string]bool{}
	}
	for _, child := range elkNode.Children {
		if child == nil {
			continue
		}
		into[child.ID] = true
		collectDescendantIds(child, into)
	}
	return into
}

// edgeSource is edge.sources?.[0] ?? edge.start.
func edgeSource(edge *ElkEdge) string {
	if edge.Source != "" {
		return edge.Source
	}
	return edge.Start
}

// edgeTarget is edge.targets?.[0] ?? edge.end.
func edgeTarget(edge *ElkEdge) string {
	if edge.Target != "" {
		return edge.Target
	}
	return edge.End
}

// internalEdgePoints returns the absolute points of every edge that ELK
// routed inside this group (both of its endpoints are descendants of the
// group) plus the attachment point of every edge that terminates on the
// group itself.
//
// An edge with one endpoint outside has its lane in the layout around the
// group, so the frame should not be drawn around it. An edge whose endpoint
// is the group is anchored on the border of the frame, so the border must
// not be pulled past that anchor. Only the terminal point on the group is
// added then.
func internalEdgePoints(graph *ElkLayoutResult, descendants map[string]bool, layoutState *ElkLayoutState, groupId string) []P {
	points := []P{}
	for _, edge := range graph.Edges {
		source := edgeSource(edge)
		target := edgeTarget(edge)
		isInternal := descendants[source] && descendants[target]
		attachesAtStart := source == groupId
		attachesAtEnd := target == groupId
		if !isInternal && !attachesAtStart && !attachesAtEnd {
			continue
		}
		offset := calcOffset(source, target, layoutState.parentLookupDb, layoutState.nodeDb)
		for _, section := range edge.Sections {
			var sectionPoints []P
			if isInternal {
				sectionPoints = append(sectionPoints, section.StartPoint)
				sectionPoints = append(sectionPoints, section.BendPoints...)
				sectionPoints = append(sectionPoints, section.EndPoint)
			} else {
				if attachesAtStart {
					sectionPoints = append(sectionPoints, section.StartPoint)
				}
				if attachesAtEnd {
					sectionPoints = append(sectionPoints, section.EndPoint)
				}
			}
			for _, p := range sectionPoints {
				points = append(points, P{X: p.X + offset.X, Y: p.Y + offset.Y})
			}
		}
	}
	return points
}

// evenGroupFrames sits the frame of each group an even distance from its
// own contents.
//
// ELK sizes a container around everything it put inside, edges included. An
// edge that runs against the flow of the layout gets routed back around the
// outside, and when that happens inside a frame the frame grows to hold the
// lane, on one side only. The frame is pulled in to SUBGRAPH_PADDING from
// the children on the left, right and bottom, and the edge keeps its lane
// just outside. The top is left exactly as ELK set it: it carries the title
// strip of the subgraph.
//
// It runs deepest-first, so a parent measures against children that have
// already been pulled in.
func evenGroupFrames(elkNodes []*NodeWithVertex, layoutState *ElkLayoutState, nodeById map[string]*Node, graph *ElkLayoutResult) {
	for _, elkNode := range elkNodes {
		if elkNode == nil || !elkNode.IsGroup {
			continue
		}
		children := elkNode.Children
		evenGroupFrames(children, layoutState, nodeById, graph)

		group := layoutState.nodeDb[elkNode.ID]
		var boxes []*NodeWithVertex
		for _, child := range children {
			if child == nil {
				continue
			}
			box := layoutState.nodeDb[child.ID]
			if box != nil && box.Offset != nil && js.Truthy(box.Width) && js.Truthy(box.Height) {
				boxes = append(boxes, box)
			}
		}
		if group == nil || group.Offset == nil || len(boxes) == 0 {
			continue
		}

		lane := internalEdgePoints(graph, collectDescendantIds(elkNode, nil), layoutState, elkNode.ID)
		var xs, ys []float64
		for _, b := range boxes {
			xs = append(xs, b.Offset.PosX)
		}
		for _, b := range boxes {
			xs = append(xs, b.Offset.PosX+b.Width)
		}
		for _, p := range lane {
			xs = append(xs, p.X)
		}
		for _, b := range boxes {
			ys = append(ys, b.Offset.PosY)
		}
		for _, b := range boxes {
			ys = append(ys, b.Offset.PosY+b.Height)
		}
		for _, p := range lane {
			ys = append(ys, p.Y)
		}

		// Only ever pull a frame in. ELK sized it to hold everything it put
		// there.
		origin := group.Offset
		left := js.Max(origin.PosX, js.Min(xs...)-SUBGRAPH_PADDING)
		right := js.Min(origin.PosX+group.Width, js.Max(xs...)+SUBGRAPH_PADDING)
		bottom := js.Min(origin.PosY+group.Height, js.Max(ys...)+SUBGRAPH_PADDING)
		top := origin.PosY

		// Keep the same title-plus-padding floor reserved before ELK routing
		// and used by clipping and painting.
		labelFloor := groupTitleWidth(elkNode.groupTitleNode())
		x := left
		width := right - left
		if width < labelFloor {
			x -= float64((labelFloor - width) / 2)
			width = labelFloor
			// Still only ever pull in.
			origRight := origin.PosX + group.Width
			x = js.Max(origin.PosX, js.Min(x, origRight-width))
			width = js.Min(width, group.Width)
		}
		height := bottom - top
		if height <= 0 || width <= 0 {
			continue
		}

		// calcOffset resolves the section of an edge against the origin of
		// the container that owns it, so moving a frame would drag every
		// edge routed inside it. Keep what ELK chose and let calcOffset
		// read that instead.
		if group.ElkOrigin == nil {
			group.ElkOrigin = &ElkOrigin{PosX: origin.PosX, PosY: origin.PosY}
		}
		group.Offset.PosX = x
		group.Offset.Width = width
		group.Offset.Height = height
		group.Width = width
		group.Height = height
		group.X = x + float64(width/2)
		group.Y = top + float64(height/2)

		layoutNode := nodeById[elkNode.ID]
		if layoutNode != nil {
			layoutNode.X = group.X
			layoutNode.Y = group.Y
			// The title floor was reserved before routing; never grow past
			// the frame of ELK here.
			layoutNode.Width = width
			layoutNode.Height = height
		}
	}
}

func applyElkNodePositions(nodeArray []*NodeWithVertex, layoutState *ElkLayoutState, nodeById map[string]*Node, relX, relY float64, depth int) {
	for _, node := range nodeArray {
		if node == nil {
			continue
		}

		graphNode := layoutState.nodeDb[node.ID]
		if graphNode == nil {
			graphNode = node
		}
		// Math.max(node.width, node.labels ? node.labels[0]?.width || 0 : 0)
		labelsWidth := 0.0
		if node.hasLabels && js.Truthy(node.labels) && len(node.Labels) > 0 {
			labelsWidth = ru.Or(node.Labels[0].Width, 0)
		}
		width := js.Max(node.Width, labelsWidth)
		offset := &ElkNodeOffset{
			PosX:   node.X + relX,
			PosY:   node.Y + relY,
			X:      relX,
			Y:      relY,
			Depth:  depth,
			Width:  width,
			Height: node.Height,
		}
		// The node can be graphNode: read its size before it changes.
		nodeWidth, nodeHeight := node.Width, node.Height
		graphNode.Offset = offset
		graphNode.X = offset.PosX + float64(nodeWidth/2)
		graphNode.Y = offset.PosY + float64(nodeHeight/2)
		graphNode.Width = nodeWidth
		graphNode.Height = nodeHeight

		layoutNode := nodeById[node.ID]
		if layoutNode != nil {
			layoutNode.X = graphNode.X
			layoutNode.Y = graphNode.Y
			if node.IsGroup {
				labelDataWidth := 0.0
				if node.LabelData != nil {
					labelDataWidth = ru.NN(node.LabelData.Width, 0)
				}
				layoutNode.Width = js.Max(node.Width, labelDataWidth)
			} else {
				layoutNode.Width = node.Width
			}
			layoutNode.Height = node.Height
			// layoutNode.labelData = node.labelData; layoutNode.labels =
			// node.labels. A later layout of the same data copies them
			// into its graph.
			setOrDeleteExtra(layoutNode, "labelData", node.labelData)
			setOrDeleteExtra(layoutNode, "labels", node.labels)
		}

		if node.IsGroup {
			applyElkNodePositions(node.Children, layoutState, nodeById, offset.PosX, offset.PosY, depth+1)
		}
	}
}

// setOrDeleteExtra assigns an extra property of a layout node. A nil value
// is undefined: the property is removed.
func setOrDeleteExtra(node *Node, key string, v any) {
	if v == nil {
		delete(node.Extra, key)
		return
	}
	node.Set(key, v)
}

// TERMINAL_JOG_MAX is the largest port-to-channel jog worth collapsing.
//
// ELK layered spreads the port of an edge evenly along the side of the
// node, then routes the edge down an inter-layer channel whose row rarely
// lines up with that port exactly. The leftover is a staircase right at the
// border. A step this close to the border can only be that connector.
const TERMINAL_JOG_MAX = 16

// TERMINAL_RUN_MAX is how far from the node the step may sit and still
// count as the connector.
const TERMINAL_RUN_MAX = 30

// JOG_EPS is the tolerance for deciding whether a segment counts as
// axis-aligned, and whether a step is a step at all. It is much smaller
// than the shared EPS of 1: ELK routinely leaves a sub-pixel step between
// the port row and the channel row.
const JOG_EPS = 0.01

// axisOf is the axis of an axis-aligned segment: "h", "v", or "" when
// diagonal.
func axisOf(a, b P) string {
	dx := math.Abs(b.X - a.X)
	dy := math.Abs(b.Y - a.Y)
	if dx > JOG_EPS && dy <= JOG_EPS {
		return "h"
	}
	if dy > JOG_EPS && dx <= JOG_EPS {
		return "v"
	}
	return ""
}

// Segment is a pair of points.
type Segment struct {
	A, B P
}

// StraightenedRun is a run displaced by straightenFront, in both its
// original and its straightened position, so that a label that was riding
// along it can be moved with it.
type StraightenedRun struct {
	Old Segment
	New Segment
}

// straightenTerminalJogs straightens the port-to-channel staircase at
// either end of a clipped route, leaving both ports where they are.
func straightenTerminalJogs(points []P) []P {
	pts, _, _ := straightenTerminalJogsWithRuns(points)
	return pts
}

// straightenTerminalJogsWithRuns is the same as straightenTerminalJogs, and
// also reports which runs moved. changed is false when nothing applies:
// upstream then returns the array that it got.
func straightenTerminalJogsWithRuns(points []P) (pts []P, runs []StraightenedRun, changed bool) {
	pts = points

	if frontPoints, run, ok := straightenFront(pts); ok {
		pts = frontPoints
		runs = append(runs, run)
		changed = true
	}

	reversed := reversedPoints(pts)
	if backPoints, run, ok := straightenFront(reversed); ok {
		pts = reversedPoints(backPoints)
		runs = append(runs, run)
		changed = true
	}

	return pts, runs, changed
}

func reversedPoints(points []P) []P {
	out := make([]P, len(points))
	for i, p := range points {
		out[len(points)-1-i] = p
	}
	return out
}

// straightenFront straightens the staircase at the front of pts. ok is
// false when it does not apply.
//
// The step is removed by pulling the channel onto the row of the port,
// never by pulling the port onto the row of the channel. The run is only
// moved when the point after it is not the far terminal.
func straightenFront(pts []P) (points []P, run StraightenedRun, ok bool) {
	if len(pts) < 5 {
		return nil, run, false
	}
	p0, p1, p2, p3 := pts[0], pts[1], pts[2], pts[3]
	axis := axisOf(p0, p1)
	other := "h"
	if axis == "h" {
		other = "v"
	}
	if axis == "" || axisOf(p2, p3) != axis || axisOf(p1, p2) != other {
		return nil, run, false
	}
	// The step has to be next to the node to be the port-to-channel connector.
	if js.Hypot(p1.X-p0.X, p1.Y-p0.Y) > TERMINAL_RUN_MAX {
		return nil, run, false
	}
	var jog float64
	if axis == "h" {
		jog = math.Abs(p2.Y - p1.Y)
	} else {
		jog = math.Abs(p2.X - p1.X)
	}
	if jog < JOG_EPS || jog > TERMINAL_JOG_MAX {
		return nil, run, false
	}
	// The route has to keep travelling the same way after the step,
	// otherwise this is a real turn rather than a connector.
	var forward bool
	if axis == "h" {
		forward = sameSign(p1.X-p0.X, p3.X-p2.X)
	} else {
		forward = sameSign(p1.Y-p0.Y, p3.Y-p2.Y)
	}
	if !forward {
		return nil, run, false
	}
	// The whole run has to move, not just its first segment.
	last := 3
	for last+1 < len(pts) && axisOf(pts[last], pts[last+1]) == axis {
		last++
	}
	// The far terminal must not be inside the run.
	if last == len(pts)-1 {
		return nil, run, false
	}

	moved := append([]P(nil), pts...)
	for i := 2; i <= last; i++ {
		if axis == "h" {
			moved[i] = P{X: pts[i].X, Y: p0.Y}
		} else {
			moved[i] = P{X: p0.X, Y: pts[i].Y}
		}
	}
	run = StraightenedRun{
		Old: Segment{A: pts[2], B: pts[last]},
		New: Segment{A: moved[2], B: moved[last]},
	}
	// p1 and p2 are now collinear with p0 and the rest of the run.
	moved = append(moved[:1], moved[3:]...)
	return moved, run, true
}

// sameSign is Math.sign(a) === Math.sign(b).
func sameSign(a, b float64) bool {
	sa, sb := js.Sign(a), js.Sign(b)
	if sa != sb {
		return false
	}
	// Math.sign keeps the sign of zero, and 0 === -0. NaN is not equal to
	// itself.
	return sa == sa
}

// segmentsCrossStrict reports whether two axis-aligned segments cross at a
// point interior to both.
func segmentsCrossStrict(a1, a2, b1, b2 P) bool {
	side := func(o, p, q P) float64 {
		return float64((p.X-o.X)*(q.Y-o.Y)) - float64((p.Y-o.Y)*(q.X-o.X))
	}
	d1 := side(b1, b2, a1)
	d2 := side(b1, b2, a2)
	d3 := side(a1, a2, b1)
	d4 := side(a1, a2, b2)
	return ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) && ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0))
}

// crossingCount is how many times one polyline crosses another.
func crossingCount(a, b []P) int {
	n := 0
	for i := 0; i < len(a)-1; i++ {
		for j := 0; j < len(b)-1; j++ {
			if segmentsCrossStrict(a[i], a[i+1], b[j], b[j+1]) {
				n++
			}
		}
	}
	return n
}

// paramOnSegment is where the projection of p onto the line through a and b
// falls, clamped to the segment.
func paramOnSegment(p, a, b P) float64 {
	abx := b.X - a.X
	aby := b.Y - a.Y
	lenSq := float64(abx*abx) + float64(aby*aby)
	if lenSq == 0 {
		return 0
	}
	return js.Max(0, js.Min(1, (float64((p.X-a.X)*abx)+float64((p.Y-a.Y)*aby))/lenSq))
}

// pointAtParam is the point at parameter t (0 = a, 1 = b) along a-b.
func pointAtParam(a, b P, t float64) P {
	return P{X: a.X + float64(t*(b.X-a.X)), Y: a.Y + float64(t*(b.Y-a.Y))}
}

// projectLabelOntoStraightenedRun projects a label sitting on one of the
// straightened runs of an edge onto the new position of that run. Nearness
// is judged on the original position of each run. ok is false when there is
// no run.
func projectLabelOntoStraightenedRun(label P, runs []StraightenedRun) (point P, ok bool) {
	bestDistSq := 0.0
	for _, run := range runs {
		t := paramOnSegment(label, run.Old.A, run.Old.B)
		oldPoint := pointAtParam(run.Old.A, run.Old.B, t)
		dx := oldPoint.X - label.X
		dy := oldPoint.Y - label.Y
		distSq := float64(dx*dx) + float64(dy*dy)
		if !ok || distSq < bestDistSq {
			bestDistSq = distSq
			point = pointAtParam(run.New.A, run.New.B, t)
			ok = true
		}
	}
	return point, ok
}

// Box is a rectangle given by two corners.
type Box struct {
	X1, Y1, X2, Y2 float64
}

// segmentHitsBox reports whether segment a-b passes through the box
// (Liang-Barsky clip).
func segmentHitsBox(a, b P, box Box) bool {
	t0 := 0.0
	t1 := 1.0
	dx := b.X - a.X
	dy := b.Y - a.Y
	for _, pq := range [4][2]float64{
		{-dx, a.X - box.X1},
		{dx, box.X2 - a.X},
		{-dy, a.Y - box.Y1},
		{dy, box.Y2 - a.Y},
	} {
		p, q := pq[0], pq[1]
		if p == 0 {
			if q < 0 {
				return false
			}
		} else if p < 0 {
			t0 = js.Max(t0, q/p)
		} else {
			t1 = js.Min(t1, q/p)
		}
	}
	return t0 < t1
}

// Size is a width and a height.
type Size struct {
	Width, Height float64
}

func labelBox(center P, size Size) Box {
	return Box{
		X1: center.X - float64(size.Width/2),
		Y1: center.Y - float64(size.Height/2),
		X2: center.X + float64(size.Width/2),
		Y2: center.Y + float64(size.Height/2),
	}
}

// StraightenedEdge is an edge whose points changed in
// straightenEdgeTerminals, with the runs that moved.
type StraightenedEdge struct {
	Edge *Edge
	Runs []StraightenedRun
}

// edgeLabelBox is the centre and the size of the label of an edge.
type edgeLabelBox struct {
	centre P
	size   Size
}

// straightenEdgeTerminals straightens the port-to-channel step on every
// edge that has one, but only where doing so does not buy a crossing or a
// label overlap.
//
// It runs once over the finished layout rather than per edge, because the
// decision needs the other edges: the step is removed by displacing one of
// the runs of this edge onto the row of the port, and that run can land in
// a lane something else already occupies.
//
// It returns, for each edge whose points changed, the runs that moved, so
// that the caller can carry the main label of the edge along.
func straightenEdgeTerminals(edges []*Edge) []StraightenedEdge {
	routes := make([][]P, len(edges))
	for i, edge := range edges {
		routes[i] = edge.Points
	}
	labels := make([]*edgeLabelBox, len(edges))
	for i, edge := range edges {
		if ru.IsUndef(edge.X) || ru.IsUndef(edge.Y) || edge.Label == "" || !js.Truthy(edge.Width) || !js.Truthy(edge.Height) {
			continue
		}
		labels[i] = &edgeLabelBox{centre: P{X: edge.X, Y: edge.Y}, size: Size{Width: edge.Width, Height: edge.Height}}
	}
	covers := func(route []P, label *edgeLabelBox) int {
		if label == nil {
			return 0
		}
		box := labelBox(label.centre, label.size)
		for i := 0; i < len(route)-1; i++ {
			if segmentHitsBox(route[i], route[i+1], box) {
				return 1
			}
		}
		return 0
	}
	var changed []StraightenedEdge

	for index, edge := range edges {
		original := routes[index]
		if len(original) < 5 {
			continue
		}
		candidate, runs, moved := straightenTerminalJogsWithRuns(original)
		if !moved {
			continue
		}

		// Where the label lands if this goes ahead: the call site carries it along.
		label := labels[index]
		candidateLabel := label
		if label != nil {
			if p, ok := projectLabelOntoStraightenedRun(label.centre, runs); ok {
				candidateLabel = &edgeLabelBox{centre: p, size: label.size}
			}
		}

		before := 0
		after := 0
		// Net counts, as for crossings: label-over-edge pairs in either direction.
		coveredBefore := 0
		coveredAfter := 0
		for other, route := range routes {
			if other == index || len(route) < 2 {
				continue
			}
			before += crossingCount(original, route)
			after += crossingCount(candidate, route)
			coveredBefore += covers(route, label) + covers(original, labels[other])
			coveredAfter += covers(route, candidateLabel) + covers(candidate, labels[other])
		}
		if after > before || coveredAfter > coveredBefore {
			continue
		}

		edge.Points = candidate
		routes[index] = candidate
		labels[index] = candidateLabel
		changed = append(changed, StraightenedEdge{Edge: edge, Runs: runs})
	}
	return changed
}

// terminalLabelPort is the pair of ports of an edge that has terminal
// labels.
type terminalLabelPort struct {
	Start P
	End   P
}

func applyElkEdgeLayout(data4Layout *ru.LayoutData, graph *ElkLayoutResult, layoutState *ElkLayoutState) {
	edgeById := map[string]*Edge{}
	for _, edge := range data4Layout.Edges {
		edgeById[edge.ID] = edge
	}
	// Opt-out rather than opt-in: the step this removes is never intentional.
	straightenEdges := !strictEq(js.Path(data4Layout.Config, "elk", "straightenEdges"), false)
	terminalLabelPorts := js.NewMap[*Edge, terminalLabelPort]()
	// Centres from an earlier run on the same data would outlive a layout that places none.
	for _, edge := range data4Layout.Edges {
		edge.TerminalLabelCenters = nil
	}

	// Alignment pre-pass: move degenerately-anchored small nodes onto their
	// routed lines before any edge points are built, so that every edge
	// sees the node at its final position.
	layoutNodeById := map[string]*Node{}
	for _, node := range data4Layout.Nodes {
		layoutNodeById[node.ID] = node
	}
	alignedNodes := map[string]bool{}
	for _, edge := range graph.Edges {
		if len(edge.Sections) == 0 {
			continue
		}
		startNode := layoutState.nodeDb[edgeSource(edge)]
		endNode := layoutState.nodeDb[edgeTarget(edge)]
		if startNode == nil || endNode == nil {
			continue
		}
		// Every node of nodeDb has an offset after applyElkNodePositions,
		// when the result of ELK has the nodes of the graph. Upstream
		// throws for a node without one.
		if startNode.Offset == nil || endNode.Offset == nil {
			continue
		}
		// edge.start ?? edge.sourceId ?? edge.sources?.[0]
		sourceId := firstDefined(edge.Start, edge.SourceID, edge.Source)
		targetId := firstDefined(edge.End, edge.TargetID, edge.Target)
		offset := calcOffset(sourceId, targetId, layoutState.parentLookupDb, layoutState.nodeDb)
		section := edge.Sections[0]
		if startNode.Shape != "rect33" {
			alignDegenerateNodeToAnchor(
				startNode,
				P{X: section.StartPoint.X + offset.X, Y: section.StartPoint.Y + offset.Y},
				layoutNodeById,
				alignedNodes,
			)
		}
		if endNode.Shape != "rect33" {
			alignDegenerateNodeToAnchor(
				endNode,
				P{X: section.EndPoint.X + offset.X, Y: section.EndPoint.Y + offset.Y},
				layoutNodeById,
				alignedNodes,
			)
		}
	}

	for _, edge := range graph.Edges {
		layoutEdge := edgeById[edge.ID]
		if layoutEdge == nil {
			continue
		}

		// A reversed edge was laid out from its target to its source.
		// Everything below works in the direction of the edge itself, so
		// its ends are swapped back here and its route reversed once built.
		reversed := edge.LayoutReversed
		startId := edgeSource(edge)
		endId := edgeTarget(edge)
		if reversed {
			startId, endId = endId, startId
		}
		startNode := layoutState.nodeDb[startId]
		endNode := layoutState.nodeDb[endId]
		if startNode == nil || endNode == nil {
			continue
		}

		// elk.box and elk.rectpacking place nodes but never route edges, so
		// ELK returns no sections. Fall back to a straight line between the
		// two node centres, clipped back to the node borders.
		if len(edge.Sections) == 0 {
			centre := func(node *NodeWithVertex) P {
				var posX, posY float64
				if node.Offset != nil {
					posX, posY = node.Offset.PosX, node.Offset.PosY
				} else {
					posX, posY = ru.NN(node.X, 0), ru.NN(node.Y, 0)
				}
				return P{
					X: posX + float64(ru.NN(node.Width, 0)/2),
					Y: posY + float64(ru.NN(node.Height, 0)/2),
				}
			}
			from := centre(startNode)
			to := centre(endNode)
			startNode.X = from.X
			startNode.Y = from.Y
			endNode.X = to.X
			endNode.Y = to.Y
			straightPoints := sanitizeElkEdgePoints([]P{from, to}, startNode, endNode)
			layoutEdge.Points = straightPoints
			layoutEdge.Curve = "linear"
			// No routing means no label position either. Put the label on
			// the midpoint of the line.
			if len(straightPoints) > 0 {
				lineStart := straightPoints[0]
				lineEnd := straightPoints[len(straightPoints)-1]
				layoutEdge.X = (lineStart.X + lineEnd.X) / 2
				layoutEdge.Y = (lineStart.Y + lineEnd.Y) / 2
			}
			continue
		}

		// Upstream throws for a node without an offset. See above.
		if startNode.Offset == nil || endNode.Offset == nil {
			continue
		}

		// edge.start ?? edge.sourceId ?? startId
		sourceId := firstDefined(edge.Start, edge.SourceID, startId)
		targetId := firstDefined(edge.End, edge.TargetID, endId)
		offset := calcOffset(sourceId, targetId, layoutState.parentLookupDb, layoutState.nodeDb)

		section := edge.Sections[0]
		points := createEdgePointsFromSection(section, offset)
		if reversed {
			points = reversedPoints(points)
		}
		// Captured after the reversal above, so start is the port at the
		// source of the edge itself.
		elkPorts := terminalLabelPort{Start: points[0], End: points[len(points)-1]}
		startNode.X = startNode.Offset.PosX + float64(startNode.Width/2)
		startNode.Y = startNode.Offset.PosY + float64(startNode.Height/2)
		endNode.X = endNode.Offset.PosX + float64(endNode.Width/2)
		endNode.Y = endNode.Offset.PosY + float64(endNode.Height/2)

		if startNode.Shape != "rect33" {
			points = append([]P{{X: startNode.X, Y: startNode.Y}}, points...)
		}

		if endNode.Shape != "rect33" {
			points = append(points, P{X: endNode.X, Y: endNode.Y})
		}

		clipped := sanitizeElkEdgePoints(points, startNode, endNode)
		layoutEdge.Points = ensureStartMarkerSegmentLength(
			ensureEndMarkerSegmentLength(
				clipped,
				boundsFor(endNode),
				getEndMarkerPathOffset(layoutEdge),
			),
			boundsFor(startNode),
			getStartMarkerPathOffset(layoutEdge),
		)
		layoutEdge.Curve = "rounded"

		if len(edge.Labels) > 0 {
			label := edge.Labels[0]
			layoutEdge.X = label.X + offset.X + float64(label.Width/2)
			layoutEdge.Y = label.Y + offset.Y + float64(label.Height/2)
		}
		for _, terminal := range edge.Labels {
			// A container algorithm other than layered leaves end labels
			// unplaced at the origin; those keep the along-the-path default.
			if terminal.Terminal == "" || (!js.Truthy(terminal.X) && !js.Truthy(terminal.Y)) {
				continue
			}
			if layoutEdge.TerminalLabelCenters == nil {
				layoutEdge.TerminalLabelCenters = map[string]P{}
			}
			layoutEdge.TerminalLabelCenters[terminal.Terminal] = P{
				X: terminal.X + offset.X + float64(terminal.Width/2),
				Y: terminal.Y + offset.Y + float64(terminal.Height/2),
			}
		}
		if layoutEdge.TerminalLabelCenters != nil {
			terminalLabelPorts.Set(layoutEdge, elkPorts)
		}
	}

	if straightenEdges {
		straightened := straightenEdgeTerminals(data4Layout.Edges)
		for _, s := range straightened {
			edge, runs := s.Edge, s.Runs
			// The x and y of the main label were set from the placement of
			// ELK on the route before the straightening. Project the label
			// onto the run it was riding on, so that it stays centred on
			// the line.
			if ru.IsUndef(edge.X) || ru.IsUndef(edge.Y) {
				continue
			}
			if projected, ok := projectLabelOntoStraightenedRun(P{X: edge.X, Y: edge.Y}, runs); ok {
				edge.X = projected.X
				edge.Y = projected.Y
			}
		}
	}
	for _, layoutEdge := range terminalLabelPorts.Keys() {
		followMovedEndpoints(layoutEdge, terminalLabelPorts.Get(layoutEdge))
	}
	// Off the frames first: the mirrored spot of a label across a frame is across it too.
	slideTerminalLabelsOffFrames(data4Layout.Edges, data4Layout.Nodes)
	putTerminalLabelsOnTheirSide(data4Layout.Edges, data4Layout.Nodes)
	separateOppositeEdgeLabels(data4Layout.Edges)
}

// firstDefined is a ?? b ?? c for ids, where an empty id is undefined.
func firstDefined(ids ...string) string {
	for _, id := range ids {
		if id != "" {
			return id
		}
	}
	return ""
}

// OPPOSITE_LABEL_MIN_GAP is the minimum clearance kept between the labels
// of two opposite-direction edges, beyond their own half-widths.
const OPPOSITE_LABEL_MIN_GAP = 4

// separateOppositeEdgeLabels pushes apart the labels of two
// opposite-direction edges between the same node pair when they would
// otherwise overlap.
func separateOppositeEdgeLabels(edges []*Edge) {
	byPair := js.NewMap[string, []*Edge]()
	for _, edge := range edges {
		if ru.IsUndef(edge.X) || ru.IsUndef(edge.Y) || edge.Label == "" || !js.Truthy(edge.Width) {
			continue
		}
		// [edge.start ?? '', edge.end ?? ''].sort().join('\u0000')
		pair := []string{edge.Start, edge.End}
		js.SortStrings(pair)
		key := pair[0] + "\x00" + pair[1]
		byPair.Set(key, append(byPair.Get(key), edge))
	}

	for _, bucket := range byPair.Values() {
		if len(bucket) != 2 {
			continue
		}
		a, b := bucket[0], bucket[1]
		// Labels drawn at meaningfully different heights are not actually
		// competing for the same horizontal space.
		if math.Abs(a.Y-b.Y) > js.Max(ru.NN(a.Height, 0), ru.NN(b.Height, 0)) {
			continue
		}
		required := float64(a.Width/2) + float64(b.Width/2) + OPPOSITE_LABEL_MIN_GAP
		current := b.X - a.X
		deficit := required - math.Abs(current)
		if deficit <= 0 {
			continue
		}
		sign := ru.Or(js.Sign(current), 1)
		a.X = a.X - float64((sign*deficit)/2)
		b.X = b.X + float64((sign*deficit)/2)
	}
}

func boxesOverlap(a, b Box) bool {
	return a.X1 < b.X2 && b.X1 < a.X2 && a.Y1 < b.Y2 && b.Y1 < a.Y2
}

// terminalObstacles is the result of terminalLabelObstacles.
type terminalObstacles struct {
	edges            []*Edge
	nodeBoxes        []Box
	frameSegments    [][2]P
	centreLabelBoxes []Box
}

// terminalLabelObstacles collects what a terminal label must not touch:
// class boxes, centre labels, group frames, other edges and other terminal
// labels.
func terminalLabelObstacles(edges []*Edge, nodes []*Node) *terminalObstacles {
	t := &terminalObstacles{edges: edges}
	boxOf := func(node *Node) Box {
		return labelBox(P{X: node.X, Y: node.Y}, Size{Width: ru.NN(node.Width, 0), Height: ru.NN(node.Height, 0)})
	}
	var placed []*Node
	for _, node := range nodes {
		if !ru.IsUndef(node.X) && !ru.IsUndef(node.Y) {
			placed = append(placed, node)
		}
	}
	for _, node := range placed {
		if !node.IsGroup {
			t.nodeBoxes = append(t.nodeBoxes, boxOf(node))
		}
	}
	// A group is hollow, so only its frame is in the way.
	for _, node := range placed {
		if !node.IsGroup {
			continue
		}
		b := boxOf(node)
		x1, y1, x2, y2 := b.X1, b.Y1, b.X2, b.Y2
		t.frameSegments = append(t.frameSegments,
			[2]P{{X: x1, Y: y1}, {X: x2, Y: y1}},
			[2]P{{X: x2, Y: y1}, {X: x2, Y: y2}},
			[2]P{{X: x2, Y: y2}, {X: x1, Y: y2}},
			[2]P{{X: x1, Y: y2}, {X: x1, Y: y1}},
		)
	}
	for _, edge := range edges {
		if edge.Label != "" && js.Truthy(edge.Width) && js.Truthy(edge.Height) && !ru.IsUndef(edge.X) {
			t.centreLabelBoxes = append(t.centreLabelBoxes, labelBox(P{X: edge.X, Y: edge.Y}, Size{Width: edge.Width, Height: edge.Height}))
		}
	}
	return t
}

// hitsFrame reports whether box, grown by frameClearance, touches the frame
// of a group.
func (t *terminalObstacles) hitsFrame(box Box, frameClearance float64) bool {
	padded := Box{
		X1: box.X1 - frameClearance,
		Y1: box.Y1 - frameClearance,
		X2: box.X2 + frameClearance,
		Y2: box.Y2 + frameClearance,
	}
	for _, s := range t.frameSegments {
		if segmentHitsBox(s[0], s[1], padded) {
			return true
		}
	}
	return false
}

// blocked reports whether a terminal label with this box touches an
// obstacle. edge, key and segment name the label and the end segment that
// it sits beside, which are not obstacles for it.
func (t *terminalObstacles) blocked(box Box, edge *Edge, key string, segment int, frameClearance float64) bool {
	for _, node := range t.nodeBoxes {
		if boxesOverlap(box, node) {
			return true
		}
	}
	for _, label := range t.centreLabelBoxes {
		if boxesOverlap(box, label) {
			return true
		}
	}
	if t.hitsFrame(box, frameClearance) {
		return true
	}
	for _, other := range t.edges {
		all := other.Points
		for i := range all {
			if i < len(all)-1 &&
				!(other == edge && i == segment) &&
				segmentHitsBox(all[i], all[i+1], box) {
				return true
			}
		}
	}
	for _, other := range t.edges {
		for _, otherKey := range terminalKeys(other.TerminalLabelCenters) {
			otherCenter := other.TerminalLabelCenters[otherKey]
			otherSize, has := other.TerminalLabelSizes[otherKey]
			if !(other == edge && otherKey == key) &&
				has &&
				boxesOverlap(box, labelBox(otherCenter, Size{Width: otherSize.Width, Height: otherSize.Height})) {
				return true
			}
		}
	}
	return false
}

// placedTerminalLabel is one placed terminal label with its size and the
// end segment it sits beside.
type placedTerminalLabel struct {
	edge    *Edge
	key     string
	center  P
	size    Size
	atStart bool
	segment int
	from    P
	to      P
	length  float64
}

// placedTerminalLabels calls yield for each placed terminal label. Upstream
// is a generator: the labels of an edge are read when the generator reaches
// the edge, after the caller handled the labels before it.
func placedTerminalLabels(edges []*Edge, yield func(l placedTerminalLabel)) {
	for _, edge := range edges {
		points := edge.Points
		if len(points) < 2 || edge.TerminalLabelCenters == nil || edge.TerminalLabelSizes == nil {
			continue
		}
		keys := terminalKeys(edge.TerminalLabelCenters)
		centers := make([]P, len(keys))
		for i, key := range keys {
			centers[i] = edge.TerminalLabelCenters[key]
		}
		for i, key := range keys {
			center := centers[i]
			size, has := edge.TerminalLabelSizes[key]
			atStart := strings.HasPrefix(key, "start")
			segment := len(points) - 2
			if atStart {
				segment = 0
			}
			from, to := points[segment], points[segment+1]
			length := js.Hypot(to.X-from.X, to.Y-from.Y)
			if !has || length == 0 {
				continue
			}
			yield(placedTerminalLabel{
				edge: edge, key: key, center: center, size: Size{Width: size.Width, Height: size.Height},
				atStart: atStart, segment: segment, from: from, to: to, length: length,
			})
		}
	}
}

// TERMINAL_LABEL_FRAME_CLEARANCE is how far a slid terminal label stays off
// the border of a group frame.
const TERMINAL_LABEL_FRAME_CLEARANCE = 2

// maxTerminalLabelSlide bounds the search of slideTerminalLabelsOffFrames,
// which takes one step for each pixel of the end segment of an edge. For a
// longer segment the layout fails with an error, where upstream searches
// on: for a segment of infinite length it does not end.
const maxTerminalLabelSlide = 1e6

// slideTerminalLabelsOffFrames slides a terminal label that ELK placed
// across the frame of a group along its end segment, by the shortest
// distance, until it clears every frame. The label never leaves that
// segment.
func slideTerminalLabelsOffFrames(edges []*Edge, nodes []*Node) {
	obstacles := terminalLabelObstacles(edges, nodes)
	placedTerminalLabels(edges, func(l placedTerminalLabel) {
		edge, key, center, size, atStart, segment, from, to, length := l.edge, l.key, l.center, l.size, l.atStart, l.segment, l.from, l.to, l.length
		if !obstacles.hitsFrame(labelBox(center, size), 0) {
			return
		}
		// Unit direction from the far point of the end segment towards its end.
		far, end := from, to
		if atStart {
			far, end = to, from
		}
		dir := P{X: (end.X - far.X) / length, Y: (end.Y - far.Y) / length}
		halfExtent := float64((float64(math.Abs(dir.X)*size.Width) + float64(math.Abs(dir.Y)*size.Height)) / 2)
		// Where the centre of the label sits along the segment: 0 at its
		// end, -length at the far point.
		along := func(c P) float64 {
			return float64((c.X-end.X)*dir.X) + float64((c.Y-end.Y)*dir.Y)
		}
		onSegment := func(c P) bool {
			return along(c)+halfExtent <= 0 && along(c)-halfExtent >= -length
		}
		if length > maxTerminalLabelSlide {
			panic(errLayoutLimit("the end segment of an edge with a terminal label is longer than 1e6"))
		}
		for step := 1.0; step <= length; step++ {
			for _, shift := range [2]float64{step, -step} {
				moved := P{X: center.X + float64(shift*dir.X), Y: center.Y + float64(shift*dir.Y)}
				if onSegment(moved) &&
					!obstacles.blocked(labelBox(moved, size), edge, key, segment, TERMINAL_LABEL_FRAME_CLEARANCE) {
					edge.TerminalLabelCenters[key] = moved
					return
				}
			}
		}
	})
}

// putTerminalLabelsOnTheirSide mirrors a terminal label that is on the
// wrong side of its edge across its end segment. A ...Right label belongs
// on the right of the direction of travel and a ...Left one on the left.
// The label moves only onto free space.
func putTerminalLabelsOnTheirSide(edges []*Edge, nodes []*Node) {
	obstacles := terminalLabelObstacles(edges, nodes)
	placedTerminalLabels(edges, func(l placedTerminalLabel) {
		edge, key, center, size, segment, from, to, length := l.edge, l.key, l.center, l.size, l.segment, l.from, l.to, l.length
		// Unit normal pointing to the right of travel (y points down).
		normal := P{X: -(to.Y - from.Y) / length, Y: (to.X - from.X) / length}
		offset := float64((center.X-from.X)*normal.X) + float64((center.Y-from.Y)*normal.Y)
		if offset == 0 || (offset > 0) == strings.HasSuffix(key, "Right") {
			return
		}
		mirrored := P{X: center.X - float64(2*offset*normal.X), Y: center.Y - float64(2*offset*normal.Y)}
		if !obstacles.blocked(labelBox(mirrored, size), edge, key, segment, 0) {
			edge.TerminalLabelCenters[key] = mirrored
		}
	})
}

// followMovedEndpoints moves each terminal label with its endpoint. ELK
// places terminal labels beside the port it routed from, but the clipping
// and straightening passes can slide the endpoint along the side of the
// node after that.
func followMovedEndpoints(edge *Edge, ports terminalLabelPort) {
	points := edge.Points
	if len(points) == 0 || edge.TerminalLabelCenters == nil {
		return
	}
	movedStart := P{X: points[0].X - ports.Start.X, Y: points[0].Y - ports.Start.Y}
	movedEnd := P{
		X: points[len(points)-1].X - ports.End.X,
		Y: points[len(points)-1].Y - ports.End.Y,
	}
	for _, key := range terminalKeys(edge.TerminalLabelCenters) {
		center := edge.TerminalLabelCenters[key]
		delta := movedEnd
		if strings.HasPrefix(key, "start") {
			delta = movedStart
		}
		edge.TerminalLabelCenters[key] = P{
			X: center.X + delta.X,
			Y: center.Y + delta.Y,
		}
	}
}

// alignDegenerateNodeToAnchor moves a small node onto the line of its
// anchor.
//
// ELK reserves PORTS_SURROUNDING_MARGIN at both ends of a node side before
// distributing edge anchors along it. On a side shorter than twice that
// margin the usable span is negative and the clamping of ELK parks the
// anchor off-centre. The line of the anchor is right: ELK has placed the
// node so that this clamped port sits on a straight route. So the node
// moves along the axis of the side until its centre sits on the anchor
// line. The first anchor wins, and both the nodeDb entry and the layout
// node move.
func alignDegenerateNodeToAnchor(node *NodeWithVertex, anchor P, layoutNodeById map[string]*Node, alignedNodes map[string]bool) {
	width := ru.NN(node.Width, 0)
	height := ru.NN(node.Height, 0)
	top := node.Offset.PosY
	bottom := top + height
	// ELK puts the anchor exactly on the border; the slack only absorbs
	// float error from the offset arithmetic.
	const tol = 0.5
	// An anchor on the top or bottom border spreads along the width; one on
	// the left or right border spreads along the height.
	alongWidth := math.Abs(anchor.Y-top) <= tol || math.Abs(anchor.Y-bottom) <= tol
	side := height
	if alongWidth {
		side = width
	}
	if side >= 2*PORTS_SURROUNDING_MARGIN {
		return
	}
	if alignedNodes[node.ID] {
		return
	}
	alignedNodes[node.ID] = true

	var delta float64
	if alongWidth {
		delta = anchor.X - (node.Offset.PosX + float64(width/2))
	} else {
		delta = anchor.Y - (node.Offset.PosY + float64(height/2))
	}
	if math.Abs(delta) < 0.01 {
		return
	}
	if alongWidth {
		node.Offset.PosX += delta
		node.X = node.Offset.PosX + float64(width/2)
	} else {
		node.Offset.PosY += delta
		node.Y = node.Offset.PosY + float64(height/2)
	}
	layoutNode := layoutNodeById[node.ID]
	if layoutNode != nil {
		layoutNode.X = node.Offset.PosX + float64(width/2)
		layoutNode.Y = node.Offset.PosY + float64(height/2)
	}
}

func createEdgePointsFromSection(section *ElkSection, offset P) []P {
	src := section.StartPoint
	dest := section.EndPoint
	points := make([]P, 0, len(section.BendPoints)+2)
	points = append(points, P{X: src.X + offset.X, Y: src.Y + offset.Y})
	for _, segment := range section.BendPoints {
		points = append(points, P{X: segment.X + offset.X, Y: segment.Y + offset.Y})
	}
	points = append(points, P{X: dest.X + offset.X, Y: dest.Y + offset.Y})
	return points
}

func calcOffset(src, dest string, parentLookupDb *TreeData, nodeDb map[string]*NodeWithVertex) P {
	ancestor := findCommonAncestor(src, dest, parentLookupDb)
	if ancestor == "" || ancestor == "root" {
		return P{X: 0, Y: 0}
	}

	// elkOrigin when present: evenGroupFrames may have moved the frame, but
	// the coordinates of a section are relative to where ELK put the
	// container, not to where the frame is now drawn.
	node := nodeDb[ancestor]
	if node == nil {
		return P{X: 0, Y: 0}
	}
	if node.ElkOrigin != nil {
		return P{X: node.ElkOrigin.PosX, Y: node.ElkOrigin.PosY}
	}
	if node.Offset != nil {
		return P{X: node.Offset.PosX, Y: node.Offset.PosY}
	}
	return P{X: 0, Y: 0}
}

func sanitizeElkEdgePoints(points []P, startNode, endNode *NodeWithVertex) []P {
	prevPoints := append([]P(nil), points...)

	var clippedPoints []P
	{
		startBounds := boundsFor(startNode)
		endBounds := boundsFor(endNode)

		startIsGroup := startNode != nil && startNode.IsGroup
		endIsGroup := endNode != nil && endNode.IsGroup

		startCandidate, startCenterApprox := getCandidateBorderPoint(prevPoints, startNode, "start")
		endCandidate, endCenterApprox := getCandidateBorderPoint(prevPoints, endNode, "end")

		skipStart := startIsGroup && onBorder(startBounds, startCandidate, 0.5)
		skipEnd := endIsGroup && onBorder(endBounds, endCandidate, 0.5)

		dropAutoCenterPoint(&prevPoints, "start", skipStart && startCenterApprox)
		dropAutoCenterPoint(&prevPoints, "end", skipEnd && endCenterApprox)

		// If a frame changed, remove its obsolete interior terminals and
		// intersect the actual crossing segment. A ray to the group centre
		// changes the approach direction and leaves a spurious segment
		// along the border.
		if startIsGroup && !skipStart {
			skipStart = clipGroupEndpoint(&prevPoints, startBounds, "start")
		}
		if endIsGroup && !skipEnd {
			skipEnd = clipGroupEndpoint(&prevPoints, endBounds, "end")
		}

		if skipStart || skipEnd {
			if !skipStart {
				applyStartIntersectionIfNeeded(&prevPoints, startNode, startBounds)
			}
			if !skipEnd {
				applyEndIntersectionIfNeeded(&prevPoints, endNode, endBounds)
			}

			clippedPoints = prevPoints
		} else {
			clippedPoints = cutter2(startNode, endNode, prevPoints)
		}
	}

	if len(clippedPoints) < 2 || hasInvalidPoint(clippedPoints) {
		var cleaned []P
		for _, p := range prevPoints {
			if isFinite(p.X) && isFinite(p.Y) {
				cleaned = append(cleaned, p)
			}
		}
		if len(cleaned) >= 2 {
			clippedPoints = cleaned
		} else {
			clippedPoints = prevPoints
		}
	}

	return dedupeConsecutivePoints(clippedPoints)
}

// isFinite is Number.isFinite.
func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func hasInvalidPoint(points []P) bool {
	for _, point := range points {
		if !isFinite(point.X) || !isFinite(point.Y) {
			return true
		}
	}
	return false
}

func dedupeConsecutivePoints(points []P) []P {
	deduped := make([]P, 0, len(points))
	for index, point := range points {
		if index == 0 {
			deduped = append(deduped, point)
			continue
		}
		prev := points[index-1]
		if math.Abs(point.X-prev.X) > 1e-6 || math.Abs(point.Y-prev.Y) > 1e-6 {
			deduped = append(deduped, point)
		}
	}
	return deduped
}

func getEndMarkerPathOffset(edge *Edge) float64 {
	return markerPathOffset(edge.ArrowTypeEnd)
}

func getStartMarkerPathOffset(edge *Edge) float64 {
	return markerPathOffset(edge.ArrowTypeStart)
}

// ensureStartMarkerSegmentLength is the mirror of
// ensureEndMarkerSegmentLength for the start of the path: the pull-back of
// the start marker walks forward along the first segment, so a short
// on-border stub there flips the start marker the same way.
func ensureStartMarkerSegmentLength(points []P, startBounds RectLike, markerOffset float64) []P {
	if markerOffset <= 0 || len(points) < 3 {
		return points
	}

	start := points[0]
	exit := points[1]
	segmentLength := js.Hypot(exit.X-start.X, exit.Y-start.Y)
	if segmentLength >= js.Max(MIN_END_MARKER_SEGMENT_LENGTH, markerOffset*2) {
		return points
	}

	if !onBorder(startBounds, exit, 1) {
		return points
	}

	adjusted := make([]P, 0, len(points)-1)
	adjusted = append(adjusted, start)
	adjusted = append(adjusted, points[2:]...)
	return adjusted
}

// ensureEndMarkerSegmentLength removes the point before the end when the
// last segment is shorter than the marker needs and that point is on the
// border of the end node.
func ensureEndMarkerSegmentLength(points []P, endBounds RectLike, markerOffset float64) []P {
	if markerOffset <= 0 || len(points) < 3 {
		return points
	}

	end := points[len(points)-1]
	entry := points[len(points)-2]
	segmentLength := js.Hypot(end.X-entry.X, end.Y-entry.Y)
	if segmentLength >= js.Max(MIN_END_MARKER_SEGMENT_LENGTH, markerOffset*2) {
		return points
	}

	if !onBorder(endBounds, entry, 1) {
		return points
	}

	adjusted := make([]P, 0, len(points)-1)
	adjusted = append(adjusted, points[:len(points)-2]...)
	adjusted = append(adjusted, end)
	return adjusted
}

// applyElkEdgeRenderData assigns the render data of buildEdgeData to each
// edge.
//
// Upstream reads defaultInterpolate and defaultStyle as properties of the
// edges array. No diagram db sets them on the array of the layout data, and
// a Go slice has no properties: they are read from the extra properties
// "edges.defaultInterpolate" (a string) and "edges.defaultStyle" (a
// []string) of the layout data.
func applyElkEdgeRenderData(data4Layout *ru.LayoutData, elkContext *ElkLayoutContext) {
	var defaults edgeDefaults
	if s, ok := data4Layout.Extra["edges.defaultStyle"].([]string); ok {
		defaults.defaultStyle = s
		defaults.hasDefaultStyle = true
	}

	for _, edge := range data4Layout.Edges {
		buildEdgeData(edge, defaults)
	}
}

func buildFallbackEdgeClasses(edge *Edge) (classes string, defined bool) {
	if !edge.ClassesUndefined || edge.Classes != "" {
		return edge.Classes, true
	}
	if edge.Start != "" && edge.End != "" {
		return "flowchart-link LS_" + edge.Start + " LE_" + edge.End, true
	}
	return "", false
}

// strokeResult is the result of computeStroke. A nil labelStyle and a nil
// style are empty arrays.
type strokeResult struct {
	thickness  string
	pattern    string
	style      []string
	labelStyle []string
}

func computeStroke(stroke string, defaultStyle []string, hasDefaultStyle bool, defaultLabelStyle []string, hasDefaultLabelStyle bool) strokeResult {
	res := strokeResult{thickness: "normal", pattern: "solid", style: []string{}, labelStyle: []string{}}

	if stroke == "dotted" {
		res.pattern = "dotted"
		res.style = []string{"fill:none", "stroke-width:2px", "stroke-dasharray:3"}
	} else if stroke == "thick" {
		res.thickness = "thick"
		res.style = []string{"stroke-width: 3.5px", "fill:none"}
	} else {
		if hasDefaultStyle {
			res.style = defaultStyle
		} else {
			res.style = []string{"fill:none"}
		}
		if hasDefaultLabelStyle {
			res.labelStyle = defaultLabelStyle
		}
	}
	return res
}

// edgeDefaults are the defaults that buildEdgeData gets. The curve of an
// edge becomes a function upstream, so the defaults for it are not read.
type edgeDefaults struct {
	defaultStyle         []string
	hasDefaultStyle      bool
	defaultLabelStyle    []string
	hasDefaultLabelStyle bool
}

// lineBreakRegex is common.lineBreakRegex: /<br\s*\/?>/gi.
var lineBreakRegex = regexp.MustCompile(`(?i)<br[\t\n\x0B\f\r \x{A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}]*/?>`)

// buildEdgeData computes the render data of an edge and assigns it to the
// edge (Object.assign(edge, buildEdgeData(...)) upstream).
//
// The structs of internal/ru cannot tell an empty string from undefined for
// text, arrowTypeStart, arrowTypeEnd, arrowheadStyle and labelpos. An empty
// value is taken as undefined here, so an edge whose db sets arrowTypeStart
// to the empty string gets "none". Both mean no marker in the edge drawing.
// Nothing reads arrowheadStyle, and no diagram db sets text or an empty
// labelpos.
func buildEdgeData(edge *Edge, defaults edgeDefaults) {
	// edgeData.minlen = edge.minlen ?? edge.length ?? 1
	minlen := ru.NN(ru.NN(edge.Minlen, edge.Length), 1)
	// edgeData.text = edge.text ?? edge.label
	text, hasTextValue := edge.Text, edge.Text != ""
	if !hasTextValue && (edge.HasLabel || edge.Label != "") {
		text, hasTextValue = edge.Label, true
	}

	arrowhead := edge.Arrowhead
	if arrowhead == "" {
		if edge.Type == "arrow_open" {
			arrowhead = "none"
		} else {
			arrowhead = "normal"
		}
	}

	edgeType := edge.Type
	if edgeType == "" {
		edgeType = "arrow_open"
	}
	arrowMap, ok := ARROW_MAP[edgeType]
	if !ok {
		arrowMap = ARROW_MAP["arrow_open"]
	}
	arrowTypeStart := edge.ArrowTypeStart
	if arrowTypeStart == "" {
		arrowTypeStart = arrowMap[0]
	}
	arrowTypeEnd := edge.ArrowTypeEnd
	if arrowTypeEnd == "" {
		arrowTypeEnd = arrowMap[1]
	}

	strokeRes := computeStroke(edge.Stroke, defaults.defaultStyle, defaults.hasDefaultStyle, defaults.defaultLabelStyle, defaults.hasDefaultLabelStyle)
	thickness := edge.Thickness
	if thickness == "" {
		thickness = strokeRes.thickness
	}
	pattern := edge.Pattern
	if pattern == "" {
		pattern = strokeRes.pattern
	}
	styleWasDefined := !edge.StyleUndefined
	classes, classesDefined := buildFallbackEdgeClasses(edge)

	// edgeData.curve = interpolateToCurve(getCurve(...), curveLinear): a
	// curve function. An empty Curve stands for a curve that is not a
	// string.
	hasText := hasTextValue && text != ""
	arrowheadStyle := edge.ArrowheadStyle
	if arrowheadStyle == "" && (hasText || styleWasDefined) {
		arrowheadStyle = "fill: #333"
	}
	labelpos := edge.Labelpos
	if labelpos == "" && hasText {
		labelpos = "c"
	}

	// edgeData.label = (edge.label ?? edgeData.text ?? '').replace(lineBreakRegex, '\n')
	label := ""
	if edge.HasLabel || edge.Label != "" {
		label = edge.Label
	} else if hasTextValue {
		label = text
	}
	label = lineBreakRegex.ReplaceAllString(label, "\n")

	// Object.assign(edge, edgeData)
	edge.Minlen = minlen
	edge.Text = text
	edge.Arrowhead = arrowhead
	edge.ArrowTypeStart = arrowTypeStart
	edge.ArrowTypeEnd = arrowTypeEnd
	edge.Thickness = thickness
	edge.Pattern = pattern
	if edge.StyleUndefined {
		edge.Style = strokeRes.style
		edge.StyleUndefined = false
	}
	if edge.LabelStyle == nil && !edge.LabelStyleIsText {
		edge.LabelStyle = strokeRes.labelStyle
	}
	edge.Classes = classes
	edge.ClassesUndefined = !classesDefined
	edge.Curve = ""
	edge.ArrowheadStyle = arrowheadStyle
	edge.Labelpos = labelpos
	edge.Label = label
	edge.HasLabel = true
}

func getEffectiveGroupWidth(node *NodeWithVertex) float64 {
	return js.Max(ru.NN(node.Width, 0), groupTitleWidth(node.groupTitleNode()))
}

func boundsFor(node *NodeWithVertex) RectLike {
	width := node.Width
	if node.IsGroup {
		width = getEffectiveGroupWidth(node)
	}
	return RectLike{
		X:       node.Offset.PosX + float64(node.Width/2),
		Y:       node.Offset.PosY + float64(node.Height/2),
		Width:   ru.NN(width, 0),
		Height:  ru.NN(node.Height, 0),
		Padding: node.Padding,
	}
}

// approxEq has the default eps = 1e-6 of upstream.
func approxEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

func isCenterApprox(point P, node *NodeWithVertex) bool {
	return approxEq(point.X, ru.NN(node.X, 0)) && approxEq(point.Y, ru.NN(node.Y, 0))
}

func getCandidateBorderPoint(points []P, node *NodeWithVertex, side Side) (candidate P, centerApprox bool) {
	if len(points) == 0 {
		return P{X: ru.NN(node.X, 0), Y: ru.NN(node.Y, 0)}, true
	}
	if side == "start" {
		first := points[0]
		centerApprox = isCenterApprox(first, node)
		candidate = first
		if centerApprox && len(points) > 1 {
			candidate = points[1]
		}
		return candidate, centerApprox
	}
	last := points[len(points)-1]
	centerApprox = isCenterApprox(last, node)
	candidate = last
	if centerApprox && len(points) > 1 {
		candidate = points[len(points)-2]
	}
	return candidate, centerApprox
}

func dropAutoCenterPoint(points *[]P, side Side, doDrop bool) {
	if !doDrop {
		return
	}
	if side == "start" {
		if len(*points) > 0 {
			*points = (*points)[1:]
		}
	} else {
		if len(*points) > 0 {
			*points = (*points)[:len(*points)-1]
		}
	}
}

func clipGroupEndpoint(pointsRef *[]P, bounds RectLike, side Side) bool {
	points := *pointsRef
	step := -1
	index := len(points) - 1
	if side == "start" {
		step = 1
		index = 0
	}
	terminalIndex := index
	for index >= 0 && index < len(points) && !outsideNode(bounds, points[index]) {
		index += step
	}
	if index == terminalIndex || index < 0 || index >= len(points) {
		return false
	}

	outside := points[index]
	inside := points[index-step]
	dx := outside.X - inside.X
	dy := outside.Y - inside.Y
	// Walk from the last interior point to the first exterior point. The
	// first side reached is the crossing, including diagonal and corner
	// approaches.
	tx := math.Inf(1)
	if dx != 0 {
		tx = (bounds.X + float64((js.Sign(dx)*bounds.Width)/2) - inside.X) / dx
	}
	ty := math.Inf(1)
	if dy != 0 {
		ty = (bounds.Y + float64((js.Sign(dy)*bounds.Height)/2) - inside.Y) / dy
	}
	t := js.Min(tx, ty)
	crossing := P{X: inside.X + float64(t*dx), Y: inside.Y + float64(t*dy)}
	if side == "start" {
		// points.splice(0, index, crossing)
		out := make([]P, 0, len(points)-index+1)
		out = append(out, crossing)
		out = append(out, points[index:]...)
		*pointsRef = out
	} else {
		// points.splice(index + 1, points.length - index - 1, crossing)
		out := make([]P, 0, index+2)
		out = append(out, points[:index+1]...)
		out = append(out, crossing)
		*pointsRef = out
	}
	return true
}

func applyStartIntersectionIfNeeded(pointsRef *[]P, startNode *NodeWithVertex, startBounds RectLike) {
	points := *pointsRef
	firstOutsideStartIndex := -1
	for index, point := range points {
		if outsideNode(startBounds, point) {
			firstOutsideStartIndex = index
			break
		}
	}
	if firstOutsideStartIndex != -1 {
		outsidePointForStart := points[firstOutsideStartIndex]
		startCenter := points[0]
		startIntersection := computeNodeIntersection(
			startNode,
			startBounds,
			outsidePointForStart,
			startCenter,
		)
		replaceEndpoint(pointsRef, "start", startIntersection, true, 0.1)
	}
}

func applyEndIntersectionIfNeeded(pointsRef *[]P, endNode *NodeWithVertex, endBounds RectLike) {
	points := *pointsRef
	outsideIndexForEnd := -1
	for index := len(points) - 1; index >= 0; index-- {
		if outsideNode(endBounds, points[index]) {
			outsideIndexForEnd = index
			break
		}
	}
	if outsideIndexForEnd != -1 {
		outsidePointForEnd := points[outsideIndexForEnd]
		endCenter := points[len(points)-1]
		endIntersection := computeNodeIntersection(
			endNode,
			endBounds,
			outsidePointForEnd,
			endCenter,
		)
		replaceEndpoint(pointsRef, "end", endIntersection, true, 0.1)
	}
}

// attachAlongDepartureAxis is the attachment point for the terminal at
// portIndex, on the axis the edge departs along.
//
// step is +1 at the start of the polyline and -1 at the end: the direction
// that walks away from the node, which gives the departure direction.
// Groups are excluded: their frame already is their outline. ok is false
// where upstream returns null.
func attachAlongDepartureAxis(node *NodeWithVertex, bounds RectLike, points []P, portIndex int, step int) (P, bool) {
	if node != nil && node.IsGroup {
		return P{}, false
	}
	nextIndex := portIndex + step
	if portIndex < 0 || portIndex >= len(points) || nextIndex < 0 || nextIndex >= len(points) {
		return P{}, false
	}
	port := points[portIndex]
	next := points[nextIndex]
	return outlineAttachPoint(node, bounds, port, next)
}

func cutter2(startNode, endNode *NodeWithVertex, originalPoints []P) []P {
	startBounds := boundsFor(startNode)
	endBounds := boundsFor(endNode)

	if len(originalPoints) == 0 {
		return []P{}
	}

	points := append([]P(nil), originalPoints...)
	startCenter := points[0]
	endCenter := points[len(points)-1]

	firstOutsideStartIndex := -1

	for index, point := range points {
		if firstOutsideStartIndex == -1 && outsideNode(startBounds, point) {
			firstOutsideStartIndex = index
		}
	}

	if firstOutsideStartIndex != -1 {
		outsidePointForStart := points[firstOutsideStartIndex]
		// Prefer an attachment on the departure axis of the edge itself;
		// see outlineAttachPoint. Falls back to the centre-ray
		// intersection, which is all a non-axis-aligned or shapeless
		// endpoint can offer.
		startIntersection, ok := attachAlongDepartureAxis(startNode, startBounds, points, firstOutsideStartIndex, 1)
		if !ok {
			startIntersection = computeNodeIntersection(startNode, startBounds, outsidePointForStart, startCenter)
		}
		replaceEndpoint(&points, "start", startIntersection, true, 0.1)
	}

	var outsidePointForEnd P
	hasOutsidePointForEnd := false
	outsideIndexForEnd := -1

	for index := len(points) - 1; index >= 0; index-- {
		if outsideNode(endBounds, points[index]) {
			outsidePointForEnd = points[index]
			hasOutsidePointForEnd = true
			outsideIndexForEnd = index
			break
		}
	}

	if !hasOutsidePointForEnd && len(points) > 1 {
		outsidePointForEnd = points[len(points)-2]
		hasOutsidePointForEnd = true
		outsideIndexForEnd = len(points) - 2
	}

	if hasOutsidePointForEnd {
		endIntersection, ok := attachAlongDepartureAxis(endNode, endBounds, points, outsideIndexForEnd, -1)
		if !ok {
			endIntersection = computeNodeIntersection(endNode, endBounds, outsidePointForEnd, endCenter)
		}
		replaceEndpoint(&points, "end", endIntersection, true, 0.1)
	}

	if len(points) > 1 {
		lastPoint := points[len(points)-1]
		secondLastPoint := points[len(points)-2]
		dx := lastPoint.X - secondLastPoint.X
		dy := lastPoint.Y - secondLastPoint.Y
		distance := math.Sqrt(float64(dx*dx) + float64(dy*dy))
		if distance < 2 {
			points = points[:len(points)-1]
		}
	}

	return points
}
