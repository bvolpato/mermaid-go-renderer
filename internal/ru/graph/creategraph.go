// Ported from mermaid 12.2.0, rendering-util/createGraph.ts (MIT).

// Package graph is the port of the layout scaffolding that the layout
// algorithms of mermaid's unified renderer share: rendering-util/createGraph.ts,
// rendering-util/layout-algorithms/common/index.ts,
// rendering-util/cloneLayoutDataForMeasure.ts and
// utils/subGraphTitleMargins.ts.
//
// A layout algorithm (dagre, ELK, swimlane) is built from phases with
// CreateCommonLayoutRenderer: prepare, measure, layout core, paint and after
// paint. See Definition. The measure phase inserts the nodes and the edge
// labels into the SVG and reads their sizes (CreateGraphWithElements,
// InsertMeasuredNode, MeasureGroupLabel). The paint phase draws the
// clusters, moves the nodes to their place and draws the edges
// (PaintLayoutData).
//
// The functions take the render context first, where upstream reads module
// state or the global config.
package graph

import (
	"errors"
	"math"
	"slices"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
)

// LayoutElementGroups is LayoutElementGroups of createGraph.ts: the groups
// that a layout draws into. RootGroups is the g.root that holds the other
// four, in this order: clusters, edge paths, edge labels, nodes.
type LayoutElementGroups struct {
	Clusters   *dom.Node
	EdgePaths  *dom.Node
	EdgeLabels *dom.Node
	Nodes      *dom.Node
	RootGroups *dom.Node
}

// LayoutGroups returns the receiver. It makes the groups a Measure, for a
// layout whose measure phase has no other result.
func (g *LayoutElementGroups) LayoutGroups() *LayoutElementGroups { return g }

// CreateLayoutElementGroupsOptions is CreateLayoutElementGroupsOptions.
type CreateLayoutElementGroupsOptions struct {
	// EdgePathsClass is the class of the group of the edge paths. Empty
	// stands for undefined: the default "edges edgePaths".
	EdgePathsClass string
}

// CreateLayoutElementGroups is createLayoutElementGroups(element, options):
// it adds g.root to element, with the groups for clusters, edge paths, edge
// labels and nodes in it.
func CreateLayoutElementGroups(element *dom.Node, options ...CreateLayoutElementGroupsOptions) *LayoutElementGroups {
	edgePathsClass := "edges edgePaths"
	if len(options) > 0 && options[0].EdgePathsClass != "" {
		edgePathsClass = options[0].EdgePathsClass
	}
	rootGroups := element.Insert("g").Attr("class", "root")
	clusters := rootGroups.Insert("g").Attr("class", "clusters")
	edgePaths := rootGroups.Insert("g").Attr("class", edgePathsClass)
	edgeLabels := rootGroups.Insert("g").Attr("class", "edgeLabels")
	nodes := rootGroups.Insert("g").Attr("class", "nodes")

	return &LayoutElementGroups{Clusters: clusters, EdgePaths: edgePaths, EdgeLabels: edgeLabels, Nodes: nodes, RootGroups: rootGroups}
}

// MeasureGroupLabel is measureGroupLabel(nodesGroup, node, measureWidth):
// it measures the label of a group node and stores the size in
// node.LabelBBox. The label is created in nodesGroup and removed again. A
// node without a label gets a zero size.
//
// measureWidth, when given, replaces node.Width for the measurement only.
// Pass math.Inf(1) to measure the label at its natural width.
func MeasureGroupLabel(ctx *render.Context, nodesGroup *dom.Node, node *ru.Node, measureWidth ...float64) error {
	if node.Label != "" {
		// measureWidth replaces the width for the measurement only. The
		// result is written to the real node.
		measured := node
		if len(measureWidth) > 0 {
			c := *node
			c.Width = measureWidth[0]
			measured = &c
		}
		res, err := ru.LabelHelper(ctx, nodesGroup, measured, "")
		if err != nil {
			return err
		}
		node.LabelBBox = &ru.Bounds{X: ru.Undef, Y: ru.Undef, Width: res.BBox.Width, Height: res.BBox.Height}
		res.ShapeSvg.Remove()
	} else {
		node.LabelBBox = &ru.Bounds{X: ru.Undef, Y: ru.Undef, Width: 0, Height: 0}
	}
	// The await of the caller. See render/async.go for the convention.
	ctx.Await()
	return nil
}

// InsertMeasuredNode is insertMeasuredNode(nodesGroup, node, renderOptions):
// it draws a node that is not a group into nodesGroup with ru.InsertNode
// and stores the size of the drawn element (its getBBox) in node.Width and
// node.Height. The result is the element of the node.
func InsertMeasuredNode(ctx *render.Context, nodesGroup *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) (*dom.Node, error) {
	childNodeEl, err := ru.InsertNode(ctx, nodesGroup, node, renderOptions)
	if err != nil {
		return nil, err
	}
	// childNodeEl.node()?.getBBox() ?? { width: 0, height: 0 }
	boundingBox := dom.Rect{Width: 0, Height: 0}
	if childNodeEl != nil {
		boundingBox = childNodeEl.BBox()
	}
	node.Width = boundingBox.Width
	node.Height = boundingBox.Height
	// The await of the caller.
	ctx.Await()
	return childNodeEl, nil
}

// CreateGraphOptions is CreateGraphOptions of createGraph.ts.
type CreateGraphOptions struct {
	// UnwrapGroupLabels measures the labels of clusters at their natural
	// width, and not wrapped at flowchart.wrappingWidth. A layout that
	// derives the size of a compound node from this measurement (ELK) sets
	// it, so that the measurement matches what InsertCluster paints.
	// Markdown labels are always measured wrapped.
	UnwrapGroupLabels bool
}

// GraphWithElements is the result of createGraphWithElements, which
// upstream also names CommonLayoutMeasure. It is the Measure of the layouts
// that use the default measure phase.
type GraphWithElements struct {
	// Graph is a directed compound multigraph with every node and edge of
	// the layout data. The label of a node is a *ru.Node and the label of
	// an edge is a *ru.Edge: shallow copies of the layout data entries, as
	// the `{ ...node }` of upstream, taken after the measurement. The name
	// of an edge is its id.
	Graph *graphlib.Graph
	// Groups are the groups that the nodes and labels were inserted into.
	Groups *LayoutElementGroups
	// NodeElements has the element of each node that is not a group, by
	// node id, in the order in which the nodes were measured.
	NodeElements *js.Map[string, *dom.Node]
}

// LayoutGroups returns the groups of the measure result.
func (g *GraphWithElements) LayoutGroups() *LayoutElementGroups {
	if g == nil {
		return nil
	}
	return g.Groups
}

// CreateGraphWithElements is createGraphWithElements(element, data4Layout,
// options): it adds the groups of a layout to element, draws every node
// that is not a group into the nodes group and measures it (node.Width,
// node.Height), measures the label of every group (node.LabelBBox), inserts
// the labels of the edges into the edge labels group (edge.Width,
// edge.Height) and builds the graph of the layout data.
//
// A nil element stands for a detached container: nothing is drawn or
// measured then, and only the graph is built.
//
// Upstream starts the work for all nodes together and awaits it with
// Promise.all. ctx.All reproduces the order in which the work of the nodes
// then continues (see render/async.go). The order shows in the result: the
// shapes of the handDrawn look draw from Math.random, and a node joins the
// graph when its measurement is done.
func CreateGraphWithElements(ctx *render.Context, element *dom.Node, data4Layout *ru.LayoutData, options ...CreateGraphOptions) (*GraphWithElements, error) {
	var opts CreateGraphOptions
	if len(options) > 0 {
		opts = options[0]
	}
	// Upstream throws a TypeError for an entry that is not an object.
	if slices.Contains(data4Layout.Nodes, nil) {
		return nil, errors.New("Cannot read properties of undefined (reading 'id')")
	}
	if slices.Contains(data4Layout.Edges, nil) {
		return nil, errors.New("Cannot read properties of undefined (reading 'label')")
	}
	// Create a directed, multi graph.
	graph := graphlib.New(graphlib.Options{
		Multigraph: true,
		Compound:   true,
	})
	edgesToProcess := slices.Clone(data4Layout.Edges)
	config := ctx.Config
	groups := CreateLayoutElementGroups(element)
	edgeLabels, nodesGroup := groups.EdgeLabels, groups.Nodes

	nodeElements := js.NewMap[string, *dom.Node]()
	nodeByID := make(map[string]*ru.Node, len(data4Layout.Nodes))
	for _, node := range data4Layout.Nodes {
		nodeByID[node.ID] = node
	}
	diagramDir := data4Layout.Direction

	// With a detached container (no real DOM) the nodes cannot be measured.
	// Then only the topology of the graph is built, with sizes that are not
	// measured.
	hasDom := element != nil

	// Insert nodes into the DOM and add them to the graph.
	err := ctx.All(len(data4Layout.Nodes), func(i int) error {
		node := data4Layout.Nodes[i]
		if node.IsGroup {
			if hasDom {
				// insertCluster paints plain cluster labels through
				// createLabel, with an infinite width, and labelHelper falls
				// back to flowchart.wrappingWidth when node.width is
				// undefined. A layout that sizes compound nodes from this
				// measurement asks to measure the way it is painted.
				// Markdown labels are painted wrapped, so they keep the
				// wrapped measurement.
				unwrap := opts.UnwrapGroupLabels && node.LabelType != "markdown"
				var err error
				if unwrap {
					err = MeasureGroupLabel(ctx, nodesGroup, node, math.Inf(1))
				} else {
					err = MeasureGroupLabel(ctx, nodesGroup, node)
				}
				if err != nil {
					return err
				}
			}
			c := *node
			graph.SetNode(node.ID, &c)
		} else {
			if hasDom {
				childNodeEl, err := InsertMeasuredNode(ctx, nodesGroup, node, &ru.ShapeRenderOptions{
					Config:  config,
					Dir:     resolveNodeDir(node, nodeByID, diagramDir),
					Padding: ru.Undef,
				})
				if err != nil {
					return err
				}
				nodeElements.Set(node.ID, childNodeEl)
			}
			c := *node
			graph.SetNode(node.ID, &c)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Add edges to the graph.

	for _, edge := range edgesToProcess {
		if hasDom && edges.HasEdgeLabel(edge) {
			if _, err := edges.InsertEdgeLabel(ctx, edgeLabels, edge); err != nil {
				return nil, err
			}
		}
		c := *edge
		var err error
		if edge.ID != "" {
			err = graph.SetEdge(edge.Start, edge.End, &c, edge.ID)
		} else {
			err = graph.SetEdge(edge.Start, edge.End, &c)
		}
		if err != nil {
			return nil, err
		}
		edgeExists := slices.ContainsFunc(data4Layout.Edges, func(existingEdge *ru.Edge) bool { return existingEdge.ID == edge.ID })
		if !edgeExists {
			data4Layout.Edges = append(data4Layout.Edges, edge)
		}
	}

	// The DDLT size capture of upstream is development tooling and is not
	// ported.

	// The await of the caller.
	ctx.Await()
	return &GraphWithElements{
		Graph:        graph,
		Groups:       groups,
		NodeElements: nodeElements,
	}, nil
}

// resolveNodeDir resolves the flow direction that applies to a node: the
// dir of the nearest ancestor group that declares one (a subgraph or a
// composite state with its own direction), else the direction of the
// diagram. Shapes that depend on the direction (fork and join bars) use it.
func resolveNodeDir(node *ru.Node, nodeByID map[string]*ru.Node, diagramDir string) string {
	current := node
	seen := map[string]bool{}
	for current != nil {
		if current.Dir != "" {
			return current.Dir
		}
		if current.ParentID == "" || seen[current.ParentID] {
			break
		}
		seen[current.ParentID] = true
		current = nodeByID[current.ParentID]
	}
	return diagramDir
}
