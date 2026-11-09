// Ported from mermaid 12.2.0, rendering-util/layout-algorithms/common/index.ts
// (MIT).

package graph

import (
	"errors"
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// RenderOptions is RenderOptions of render.ts.
type RenderOptions struct {
	// Algorithm is the algorithm that the layout was registered with, for
	// example "elk.layered". Empty stands for undefined.
	Algorithm string
}

// RenderContext is CommonLayoutRenderContext: what the phases of a layout
// get. Upstream also passes the internal helpers of mermaid (the config,
// the logger, common functions); here the render context has them.
type RenderContext struct {
	// Ctx is the render: the config, the random generator, the per-render
	// state of the ported modules.
	Ctx *render.Context
	// Element is the root group of the svg: svg.select('g').
	Element *dom.Node
	// Options are the options of the layout.
	Options RenderOptions
	// PreparedLayout is what PrepareLayout returned, or nil. The later
	// phases can keep state in it: the dagre layout stores its measured
	// layout there.
	PreparedLayout any
}

// Measure is the result of the measure phase of a layout. The common paint
// needs the groups that it draws into. The layout can keep more in its own
// type and read it back in its paint callbacks with a type assertion on
// PaintContext.Measure. *GraphWithElements and *LayoutElementGroups are
// Measures.
type Measure interface {
	LayoutGroups() *LayoutElementGroups
}

// PaintContext is CommonLayoutPaintContext: the render context of the
// layout with the result of its measure phase.
type PaintContext struct {
	RenderContext
	Measure Measure
}

// PaintOptions is CommonLayoutPaintOptions: how PaintLayoutData finds the
// nodes and edges to draw. Every field is optional.
type PaintOptions struct {
	// ClusterDb returns the clusterDb argument of insertEdge for this
	// render. Upstream holds a module-level Map here; the Go layout keeps
	// its Map per render, so this is a function. Nil stands for an empty
	// Map.
	ClusterDb func(pc *PaintContext) edges.ClusterDb
	// GetNodes returns the nodes to paint, in paint order. Nil stands for
	// data4Layout.Nodes. The dagre layout returns the node labels of its
	// graph here: cluster shapes write into the nodes that they get.
	GetNodes func(data4Layout *ru.LayoutData, pc *PaintContext) []*ru.Node
	// GetEdgeNode returns the laid-out node at an end of an edge (id is
	// edge.Start or edge.End, "" when undefined). Nil, or a nil result,
	// stands for the node with that id in data4Layout.Nodes.
	GetEdgeNode func(id string, edge *ru.Edge, pc *PaintContext) *ru.Node
	// SkipNode says that a node is not painted.
	SkipNode func(node *ru.Node, pc *PaintContext) bool
	// IsCluster says whether a group node is painted as a cluster. Nil
	// stands for true. A node that is not a group is never a cluster.
	IsCluster func(node *ru.Node, pc *PaintContext) bool
	// SkipEdge says that an edge is not painted. Edges with IsLayoutOnly
	// are never painted.
	SkipEdge func(edge *ru.Edge) bool
	// SkipIntersect is the skipIntersect argument of insertEdge for every
	// edge. SkipIntersectFunc, when it is not nil, decides for each edge
	// and SkipIntersect is not used.
	SkipIntersect     bool
	SkipIntersectFunc func(edge *ru.Edge) bool
}

// Definition is CommonLayoutRendererDefinition: the phases of a layout
// algorithm. CreateCommonLayoutRenderer runs them in this order for each
// render:
//
//  1. insertMarkers and ClearLayoutRenderState,
//  2. PrepareLayout: changes to the layout data that the algorithm needs.
//     Its result goes to RenderContext.PreparedLayout.
//  3. MeasureLayout: insert the nodes and labels into the SVG and read their
//     sizes. Nil stands for DefaultMeasureLayout (CreateGraphWithElements).
//  4. RunLayoutCore: compute the positions and write them into the layout
//     data (node.X, node.Y, node.Width, node.Height, edge.Points). Required.
//  5. PaintLayout, when it is not nil: draw everything. Otherwise
//     PaintLayoutData with PaintOptions.
//  6. AfterPaint: work on the drawn SVG, for example line hops.
//
// An error of a phase stops the render and is returned.
type Definition struct {
	PrepareLayout func(data4Layout *ru.LayoutData, rc *RenderContext) (any, error)
	MeasureLayout func(data4Layout *ru.LayoutData, rc *RenderContext) (Measure, error)
	RunLayoutCore func(data4Layout *ru.LayoutData, rc *RenderContext) (any, error)
	// PaintLayout and AfterPaint get the result of RunLayoutCore.
	PaintLayout  func(data4Layout *ru.LayoutData, pc *PaintContext, coreResult any) error
	AfterPaint   func(data4Layout *ru.LayoutData, pc *PaintContext, coreResult any) error
	PaintOptions *PaintOptions
}

// CreateCommonLayoutRenderer is createCommonLayoutRenderer(definition): it
// returns the render function of a layout algorithm, to register with
// ru.RegisterLayout.
func CreateCommonLayoutRenderer(def Definition) ru.LayoutAlgorithm {
	// Use the measure phase of the definition, or createGraphWithElements
	// when there is none.
	measureLayoutFn := def.MeasureLayout
	if measureLayoutFn == nil {
		measureLayoutFn = func(data4Layout *ru.LayoutData, rc *RenderContext) (Measure, error) {
			measure, err := DefaultMeasureLayout(data4Layout, rc)
			if err != nil {
				return nil, err
			}
			return measure, nil
		}
	}

	return func(ctx *render.Context, data4Layout *ru.LayoutData, svg *dom.Node, algorithm string) error {
		element := svg.Select("g")
		if err := edges.InsertMarkers(ctx, element, data4Layout.Markers, data4Layout.Type, data4Layout.DiagramID); err != nil {
			return err
		}
		ClearLayoutRenderState(ctx)

		// Everything that the phases need to render.
		renderContext := &RenderContext{
			Ctx:     ctx,
			Element: element,
			Options: RenderOptions{Algorithm: algorithm},
		}

		// The changes to the parsed layout data that the algorithm needs.
		if def.PrepareLayout != nil {
			preparedLayout, err := def.PrepareLayout(data4Layout, renderContext)
			if err != nil {
				return err
			}
			renderContext.PreparedLayout = preparedLayout
		}

		// Get the sizes of the labels and the other elements.
		measure, err := measureLayoutFn(data4Layout, renderContext)
		if err != nil {
			return err
		}

		// Run the layout algorithm: it computes the positions of the nodes
		// and edges from the layout data and the measurements.
		if def.RunLayoutCore == nil {
			return errors.New("runLayoutCore is not a function")
		}
		coreResult, err := def.RunLayoutCore(data4Layout, renderContext)
		if err != nil {
			return err
		}

		paintContext := &PaintContext{
			RenderContext: *renderContext,
			Measure:       measure,
		}

		if def.PaintLayout != nil {
			// A paint phase of the definition does everything.
			if err := def.PaintLayout(data4Layout, paintContext, coreResult); err != nil {
				return err
			}
		} else {
			if err := PaintLayoutData(data4Layout, paintContext, def.PaintOptions); err != nil {
				return err
			}
		}
		// Work after the paint, for example on the drawn edge paths.
		if def.AfterPaint != nil {
			if err := def.AfterPaint(data4Layout, paintContext, coreResult); err != nil {
				return err
			}
		}
		return nil
	}
}

var (
	clearMu sync.RWMutex
	// clearFuncs are the clear functions of the layout modules.
	clearFuncs []func(ctx *render.Context)
)

// RegisterClear adds a function that ClearLayoutRenderState calls. Upstream
// calls the clear() of dagre/mermaid-graphlib.js there; the dagre layout
// registers it from init, because this package does not import the layouts.
func RegisterClear(fn func(ctx *render.Context)) {
	clearMu.Lock()
	defer clearMu.Unlock()
	clearFuncs = append(clearFuncs, fn)
}

// ClearLayoutRenderState is clearLayoutRenderState(): it empties the
// per-render maps of the nodes, the edges, the clusters and the layouts.
func ClearLayoutRenderState(ctx *render.Context) {
	ru.ClearNodes(ctx)
	edges.Clear(ctx)
	clusters.Clear(ctx)
	// clearGraphlib.
	clearMu.RLock()
	fns := clearFuncs
	clearMu.RUnlock()
	for _, fn := range fns {
		fn(ctx)
	}
}

// DefaultMeasureLayout is defaultMeasureLayout(data4Layout, context,
// options): the measure phase of a layout that has none of its own. It is
// CreateGraphWithElements on the root group.
func DefaultMeasureLayout(data4Layout *ru.LayoutData, rc *RenderContext, options ...CreateGraphOptions) (*GraphWithElements, error) {
	measure, err := CreateGraphWithElements(rc.Ctx, rc.Element, data4Layout, options...)
	if err != nil {
		return nil, err
	}
	// The await of the caller. See render/async.go for the convention.
	rc.Ctx.Await()
	return measure, nil
}

// PaintLayoutData is paintLayoutData(data4Layout, context, options): the
// paint phase of a layout that has none of its own. For each node it draws
// the cluster (clusters.InsertCluster) or moves the element of the node to
// its place (ru.PositionNode). Then it draws each edge (edges.InsertEdge),
// inserts its label when the measure phase did not, and moves the labels to
// their place. options may be nil.
func PaintLayoutData(data4Layout *ru.LayoutData, pc *PaintContext, options *PaintOptions) error {
	if options == nil {
		options = &PaintOptions{}
	}
	if pc.Measure == nil {
		return errors.New("Cannot destructure property 'groups' of 'measure' as it is undefined.")
	}
	groups := pc.Measure.LayoutGroups()
	if groups == nil {
		return errors.New("Cannot read properties of undefined (reading 'clusters')")
	}

	// Render clusters and position nodes. This also sets node.Intersect on
	// the cluster nodes.
	nodes := data4Layout.Nodes
	if options.GetNodes != nil {
		nodes = options.GetNodes(data4Layout, pc)
	}
	for _, node := range nodes {
		if node == nil {
			// Upstream throws a TypeError for an entry that is not an object.
			return errors.New("Cannot read properties of undefined (reading 'clusterNode')")
		}
		if options.SkipNode != nil && options.SkipNode(node, pc) {
			continue
		}
		if err := paintLayoutNode(groups, node, pc, options); err != nil {
			return err
		}
	}

	nodeByID := buildNodeLookup(data4Layout.Nodes)

	for _, edge := range data4Layout.Edges {
		if edge == nil {
			return errors.New("Cannot read properties of undefined (reading 'isLayoutOnly')")
		}
		if shouldSkipPaintEdge(edge, options) {
			continue
		}

		if err := paintLayoutEdge(groups, edge, nodeByID, data4Layout, options, pc); err != nil {
			return err
		}
	}
	// The await of the caller.
	pc.Ctx.Await()
	return nil
}

// positionNode is positionNode(node) of nodes.ts for a layout node.
func positionNode(ctx *render.Context, node *ru.Node) {
	ru.PositionNode(ctx, ru.PositionedNode{
		ID: node.ID, X: node.X, Y: node.Y, Width: node.Width, Height: node.Height,
		Diff: node.Diff, ClusterNode: node.ClusterNode,
	})
}

func paintLayoutNode(groups *LayoutElementGroups, node *ru.Node, pc *PaintContext, options *PaintOptions) error {
	if node.ClusterNode {
		positionNode(pc.Ctx, node)
	} else if shouldPaintAsCluster(node, pc, options) {
		if _, err := clusters.InsertCluster(pc.Ctx, groups.Clusters, node); err != nil {
			return err
		}
	} else {
		positionNode(pc.Ctx, node)
	}
	// The await of the caller.
	pc.Ctx.Await()
	return nil
}

func shouldPaintAsCluster(node *ru.Node, pc *PaintContext, options *PaintOptions) bool {
	if !node.IsGroup {
		return false
	}
	if options.IsCluster != nil {
		return options.IsCluster(node, pc)
	}
	return true
}

func buildNodeLookup(nodes []*ru.Node) map[string]*ru.Node {
	nodeByID := map[string]*ru.Node{}
	for _, node := range nodes {
		if node != nil && node.ID != "" {
			nodeByID[node.ID] = node
		}
	}
	return nodeByID
}

func shouldSkipPaintEdge(edge *ru.Edge, options *PaintOptions) bool {
	return edge.IsLayoutOnly || (options.SkipEdge != nil && options.SkipEdge(edge))
}

func paintLayoutEdge(groups *LayoutElementGroups, edge *ru.Edge, nodeByID map[string]*ru.Node, data4Layout *ru.LayoutData, options *PaintOptions, pc *PaintContext) error {
	ctx := pc.Ctx
	// options.clusterDb ?? new Map()
	var clusterDb edges.ClusterDb
	if options.ClusterDb != nil {
		clusterDb = options.ClusterDb(pc)
	}
	edgeCopy := *edge
	paths, err := edges.InsertEdge(
		ctx,
		groups.EdgePaths,
		&edgeCopy,
		clusterDb,
		data4Layout.Type,
		getRenderedNode(edge.Start, edge, nodeByID, pc, options),
		getRenderedNode(edge.End, edge, nodeByID, pc, options),
		data4Layout.DiagramID,
		shouldSkipIntersect(edge, options),
		data4Layout.LayoutAlgorithm,
	)
	if err != nil {
		return err
	}

	if edges.HasEdgeLabel(edge) {
		if _, ok := edges.EdgeLabels(ctx)[edge.ID]; !ok {
			if _, err := edges.InsertEdgeLabel(ctx, groups.EdgeLabels, edge); err != nil {
				return err
			}
		}
		if err := positionRenderedEdgeLabel(ctx, edge, paths); err != nil {
			return err
		}
	}
	// The await of the caller.
	ctx.Await()
	return nil
}

// getRenderedNode returns the laid-out node at an end of an edge. A nil
// result is the empty object that upstream passes for an unknown node.
func getRenderedNode(id string, edge *ru.Edge, nodeByID map[string]*ru.Node, pc *PaintContext, options *PaintOptions) *ru.Node {
	if options.GetEdgeNode != nil {
		if node := options.GetEdgeNode(id, edge, pc); node != nil {
			return node
		}
	}
	if id != "" {
		return nodeByID[id]
	}
	return nil
}

func shouldSkipIntersect(edge *ru.Edge, options *PaintOptions) bool {
	if options.SkipIntersectFunc != nil {
		return options.SkipIntersectFunc(edge)
	}
	return options.SkipIntersect
}

// templateNum is a number in a template literal that comes straight from
// an optional property: undefined (NaN in the data model) prints as
// "undefined".
func templateNum(v float64) string {
	if ru.IsUndef(v) {
		return "undefined"
	}
	return js.Str(v)
}

func positionRenderedEdgeLabel(ctx *render.Context, edge *ru.Edge, paths *edges.Paths) error {
	// paths?.updatedPath ?? paths?.originalPath. A nil path is undefined.
	var path []ru.Point
	if paths != nil {
		if paths.HasUpdatedPath {
			path = append([]ru.Point{}, paths.UpdatedPath...)
		} else if paths.HasOriginalPath {
			path = append([]ru.Point{}, paths.OriginalPath...)
		}
	}
	subGraphTitleTotalMargin := GetSubGraphTitleMargins(ctx.Config).SubGraphTitleTotalMargin
	if edge.Label != "" {
		el := edges.EdgeLabels(ctx)[edge.ID]
		p, err := edges.ResolveEdgeLabelPosition(edge, paths)
		if err != nil {
			return err
		}
		if el == nil {
			return errors.New("Cannot read properties of undefined (reading 'attr')")
		}
		// Without an updated path x is edge.x as it is, which can be
		// undefined. Every other value is the result of arithmetic.
		x := js.Str(p.X)
		if paths == nil || !paths.HasUpdatedPath {
			x = templateNum(edge.X)
		}
		el.Attr("transform", "translate("+x+", "+js.Str(p.Y+float64(subGraphTitleTotalMargin/2))+")")
	}

	for _, label := range [...]struct{ key, text string }{
		{"startLeft", edge.StartLabelLeft},
		{"startRight", edge.StartLabelRight},
		{"endLeft", edge.EndLabelLeft},
		{"endRight", edge.EndLabelRight},
	} {
		if label.text != "" {
			p, err := TerminalLabelTranslate(edge, label.key, path)
			if err != nil {
				return err
			}
			el := edges.TerminalLabels(ctx)[edge.ID].Get(label.key)
			if el == nil {
				return errors.New("Cannot read properties of undefined (reading 'attr')")
			}
			x, y := js.Str(p.X), js.Str(p.Y)
			if _, hasCenter := edge.TerminalLabelCenters[label.key]; !hasCenter && path == nil {
				// The point is (edge.x, edge.y) as they are.
				x, y = templateNum(edge.X), templateNum(edge.Y)
			}
			el.Attr("transform", "translate("+x+", "+y+")")
		}
	}
	return nil
}

// terminalLabelSides is TERMINAL_LABEL_SIDES.
var terminalLabelSides = map[string]string{
	"startLeft":  "start_left",
	"startRight": "start_right",
	"endLeft":    "end_left",
	"endRight":   "end_right",
}

// TerminalLabelTranslate is terminalLabelTranslate(edge, key, path): where
// to move the group of a terminal label of an edge. key is startLeft,
// startRight, endLeft or endRight. It is the placement of the layout
// (edge.TerminalLabelCenters) when there is one, else a point along the
// path. A nil path stands for undefined: the result is then (edge.X,
// edge.Y), which can be undefined (NaN).
func TerminalLabelTranslate(edge *ru.Edge, key string, path []ru.Point) (ru.Point, error) {
	if center, ok := edge.TerminalLabelCenters[key]; ok {
		return center, nil
	}
	if path == nil {
		return ru.Point{X: edge.X, Y: edge.Y}, nil
	}
	arrowType := edge.ArrowTypeEnd
	if strings.HasPrefix(key, "start") {
		arrowType = edge.ArrowTypeStart
	}
	terminalMarkerSize := 0.0
	if arrowType != "" {
		terminalMarkerSize = 10
	}
	points := make([]utils.Point, len(path))
	for i, p := range path {
		points[i] = utils.Point{X: p.X, Y: p.Y}
	}
	p, err := utils.CalcTerminalLabelPosition(terminalMarkerSize, terminalLabelSides[key], points)
	if err != nil {
		return ru.Point{}, err
	}
	return ru.Point{X: p.X, Y: p.Y}, nil
}
