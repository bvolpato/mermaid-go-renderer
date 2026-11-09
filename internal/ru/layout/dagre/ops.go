package dagre

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	rugraph "github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
)

// scope names the graph that a call of ops is made for. The real functions
// do not use it. The tests replay recorded results by graph.
type scope struct {
	graph *graphlib.Graph
	// parentCluster is the id of the cluster that a nested graph is the
	// content of. It is empty for the top graph.
	parentCluster string
}

// ops are the functions of the other rendering-util modules that the
// layout calls: they draw into the SVG and measure what they drew. The
// layout reaches them only through this struct, so that the graph logic
// runs in a test without a DOM.
//
// insertMeasuredNode, insertEdgeLabel and insertCluster are async functions
// upstream. The real functions take their turns in the scheduler of the
// render context themselves (internal/render/async.go), so the layout adds
// no ctx.Await for these calls.
type ops struct {
	// createLayoutElementGroups is createLayoutElementGroups(element, options).
	createLayoutElementGroups func(sc *scope, element *dom.Node, options rugraph.CreateLayoutElementGroupsOptions) *rugraph.LayoutElementGroups
	// insertMeasuredNode is insertMeasuredNode(nodesGroup, node, renderOptions).
	insertMeasuredNode func(sc *scope, nodesGroup *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) error
	// insertEdgeLabel is insertEdgeLabel(elem, edge).
	insertEdgeLabel func(sc *scope, elem *dom.Node, edge *ru.Edge) error
	// updateNodeBounds is updateNodeBounds(node, element).
	updateNodeBounds func(sc *scope, node *ru.Node, element *dom.Node)
	// setNodeElem is setNodeElem(elem, node).
	setNodeElem func(sc *scope, elem *dom.Node, node *ru.Node)
	// insertCluster is insertCluster(elem, node).
	insertCluster func(sc *scope, elem *dom.Node, node *ru.Node) error
	// positionNode is positionNode(node).
	positionNode func(sc *scope, node *ru.Node)
	// insertEdge is insertEdge(elem, edge, clusterDb, diagramType,
	// startNode, endNode, id).
	insertEdge func(sc *scope, elem *dom.Node, edge *ru.Edge, clusterDb edges.ClusterDb, diagramType string, startNode, endNode *ru.Node, id string) (*edges.Paths, error)
	// positionEdgeLabel is positionEdgeLabel(edge, paths).
	positionEdgeLabel func(sc *scope, edge *ru.Edge, paths *edges.Paths) error
}

// trace sees the graphs of a render at fixed points. Every field may be nil.
type trace struct {
	// beforeAdjust and afterAdjust see the graph that
	// prepareLayoutForDagre built, around adjustClustersAndEdges.
	beforeAdjust func(g *graphlib.Graph)
	afterAdjust  func(g *graphlib.Graph)
	// beforeLayout and afterLayout see a graph around the layout of dagre.
	beforeLayout func(sc *scope, g *graphlib.Graph)
	afterLayout  func(sc *scope, g *graphlib.Graph)
}

// realOps returns the functions of the rendering-util packages.
func realOps(ctx *render.Context) *ops {
	return &ops{
		createLayoutElementGroups: func(_ *scope, element *dom.Node, options rugraph.CreateLayoutElementGroupsOptions) *rugraph.LayoutElementGroups {
			return rugraph.CreateLayoutElementGroups(element, options)
		},
		insertMeasuredNode: func(_ *scope, nodesGroup *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) error {
			_, err := rugraph.InsertMeasuredNode(ctx, nodesGroup, node, renderOptions)
			return err
		},
		insertEdgeLabel: func(_ *scope, elem *dom.Node, edge *ru.Edge) error {
			_, err := edges.InsertEdgeLabel(ctx, elem, edge)
			return err
		},
		updateNodeBounds: func(_ *scope, node *ru.Node, element *dom.Node) {
			ru.UpdateNodeBounds(node, element)
		},
		setNodeElem: func(_ *scope, elem *dom.Node, node *ru.Node) {
			ru.SetNodeElem(ctx, elem, node.ID)
		},
		insertCluster: func(_ *scope, elem *dom.Node, node *ru.Node) error {
			_, err := clusters.InsertCluster(ctx, elem, node)
			return err
		},
		positionNode: func(_ *scope, node *ru.Node) {
			ru.PositionNode(ctx, ru.PositionedNode{
				ID: node.ID, X: node.X, Y: node.Y, Width: node.Width, Height: node.Height,
				Diff: node.Diff, ClusterNode: node.ClusterNode,
			})
		},
		insertEdge: func(_ *scope, elem *dom.Node, edge *ru.Edge, clusterDb edges.ClusterDb, diagramType string, startNode, endNode *ru.Node, id string) (*edges.Paths, error) {
			return edges.InsertEdge(ctx, elem, edge, clusterDb, diagramType, startNode, endNode, id, false, "")
		},
		positionEdgeLabel: func(_ *scope, edge *ru.Edge, paths *edges.Paths) error {
			return edges.PositionEdgeLabel(ctx, edge, paths)
		},
	}
}
