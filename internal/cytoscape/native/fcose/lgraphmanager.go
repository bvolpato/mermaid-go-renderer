// Ported from layout-base 2.0.1 (MIT license): src/LGraphManager.js, and
// from cose-base 2.2.0 (MIT license): src/CoSEGraphManager.js.

package fcose

import "github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"

// GraphManager is a CoSEGraphManager: the classes LGraphManager and
// CoSEGraphManager.
type GraphManager struct {
	layout *Layout
	graphs []*Graph
	// edges are the inter-graph edges.
	edges     []*Edge
	rootGraph *Graph

	// allNodes and allEdges are caches. The JavaScript distinguishes an
	// empty list from null.
	allNodes                 []*Node
	allEdges                 []*Edge
	hasAllNodes, hasAllEdges bool

	allNodesToApplyGravitation    []*Node
	hasAllNodesToApplyGravitation bool
}

func newGraphManager(layout *Layout) *GraphManager {
	return &GraphManager{layout: layout}
}

func (gm *GraphManager) addRoot() *Graph {
	ngraph := gm.layout.newGraph()
	nnode := gm.layout.newNode()
	root := gm.addGraph(ngraph, nnode)
	gm.setRootGraph(root)
	return gm.rootGraph
}

// addGraph is LGraphManager.add with a graph and its parent node.
func (gm *GraphManager) addGraph(newGraph *Graph, parentNode *Node) *Graph {
	if newGraph == nil {
		panic("Graph is null!")
	}
	if parentNode == nil {
		panic("Parent node is null!")
	}
	for _, g := range gm.graphs {
		if g == newGraph {
			panic("Graph already in this graph mgr!")
		}
	}

	gm.graphs = append(gm.graphs, newGraph)

	if newGraph.parent != nil {
		panic("Already has a parent!")
	}
	if parentNode.child != nil {
		panic("Already has a child!")
	}

	newGraph.parent = parentNode
	parentNode.child = newGraph

	return newGraph
}

// addEdge is LGraphManager.add with an edge, its source and its target.
func (gm *GraphManager) addEdge(newEdge *Edge, sourceNode, targetNode *Node) *Edge {
	sourceGraph := sourceNode.getOwner()
	targetGraph := targetNode.getOwner()

	if !(sourceGraph != nil && sourceGraph.getGraphManager() == gm) {
		panic("Source not in this graph mgr!")
	}
	if !(targetGraph != nil && targetGraph.getGraphManager() == gm) {
		panic("Target not in this graph mgr!")
	}

	if sourceGraph == targetGraph {
		newEdge.isInterGraph = false
		return sourceGraph.addEdge(newEdge, sourceNode, targetNode)
	}
	newEdge.isInterGraph = true

	// set source and target
	newEdge.source = sourceNode
	newEdge.target = targetNode

	// add edge to inter-graph edge list
	for _, e := range gm.edges {
		if e == newEdge {
			panic("Edge already in inter-graph edge list!")
		}
	}

	gm.edges = append(gm.edges, newEdge)

	// add edge to source and target incidency lists
	newEdge.source.edges = append(newEdge.source.edges, newEdge)
	newEdge.target.edges = append(newEdge.target.edges, newEdge)

	return newEdge
}

func (gm *GraphManager) updateBounds() {
	gm.rootGraph.updateBounds(true)
}

func (gm *GraphManager) getGraphs() []*Graph { return gm.graphs }

func (gm *GraphManager) getAllNodes() []*Node {
	if !gm.hasAllNodes {
		var nodeList []*Node
		for _, g := range gm.getGraphs() {
			nodeList = append(nodeList, g.getNodes()...)
		}
		gm.allNodes = nodeList
		gm.hasAllNodes = true
	}
	return gm.allNodes
}

func (gm *GraphManager) getAllEdges() []*Edge {
	if !gm.hasAllEdges {
		var edgeList []*Edge
		for _, g := range gm.getGraphs() {
			edgeList = append(edgeList, g.getEdges()...)
		}

		edgeList = append(edgeList, gm.edges...)

		gm.allEdges = edgeList
		gm.hasAllEdges = true
	}
	return gm.allEdges
}

func (gm *GraphManager) getAllNodesToApplyGravitation() []*Node {
	return gm.allNodesToApplyGravitation
}

func (gm *GraphManager) setAllNodesToApplyGravitation(nodeList []*Node) {
	if gm.hasAllNodesToApplyGravitation {
		panic("assert failed")
	}

	gm.allNodesToApplyGravitation = nodeList
	gm.hasAllNodesToApplyGravitation = true
}

func (gm *GraphManager) getRoot() *Graph { return gm.rootGraph }

func (gm *GraphManager) setRootGraph(graph *Graph) {
	if graph.getGraphManager() != gm {
		panic("Root not in this graph mgr!")
	}

	gm.rootGraph = graph
	// root graph must have a root node associated with it for convenience
	if graph.parent == nil {
		graph.parent = gm.layout.newNode()
	}
}

func (gm *GraphManager) getLayout() *Layout { return gm.layout }

// isOneAncestorOfOther tells whether one of the nodes is the other one or an
// ancestor of it.
func (gm *GraphManager) isOneAncestorOfOther(firstNode, secondNode *Node) bool {
	if !(firstNode != nil && secondNode != nil) {
		panic("assert failed")
	}

	if firstNode == secondNode {
		return true
	}
	// Is second node an ancestor of the first one?
	ownerGraph := firstNode.getOwner()

	for {
		parentNode := ownerGraph.getParent()

		if parentNode == nil {
			break
		}

		if parentNode == secondNode {
			return true
		}

		ownerGraph = parentNode.getOwner()
		if ownerGraph == nil {
			break
		}
	}
	// Is first node an ancestor of the second one?
	ownerGraph = secondNode.getOwner()

	for {
		parentNode := ownerGraph.getParent()

		if parentNode == nil {
			break
		}

		if parentNode == firstNode {
			return true
		}

		ownerGraph = parentNode.getOwner()
		if ownerGraph == nil {
			break
		}
	}

	return false
}

func (gm *GraphManager) calcLowestCommonAncestors() {
	edges := gm.getAllEdges()
	for _, edge := range edges {
		sourceNode := edge.source
		targetNode := edge.target
		edge.lca = nil
		edge.sourceInLca = sourceNode
		edge.targetInLca = targetNode

		if sourceNode == targetNode {
			edge.lca = sourceNode.getOwner()
			continue
		}

		sourceAncestorGraph := sourceNode.getOwner()

		for edge.lca == nil {
			edge.targetInLca = targetNode
			targetAncestorGraph := targetNode.getOwner()

			for edge.lca == nil {
				if targetAncestorGraph == sourceAncestorGraph {
					edge.lca = targetAncestorGraph
					break
				}

				if targetAncestorGraph == gm.rootGraph {
					break
				}

				edge.targetInLca = targetAncestorGraph.getParent()
				targetAncestorGraph = edge.targetInLca.getOwner()
			}

			if sourceAncestorGraph == gm.rootGraph {
				break
			}

			if edge.lca == nil {
				edge.sourceInLca = sourceAncestorGraph.getParent()
				sourceAncestorGraph = edge.sourceInLca.getOwner()
			}
		}

		if edge.lca == nil {
			panic("assert failed")
		}
	}
}

// calcInclusionTreeDepths is LGraphManager.calcInclusionTreeDepths. The call
// without arguments is the call with the root graph and the depth 1.
func (gm *GraphManager) calcInclusionTreeDepths(graph *Graph, depth int) {
	for _, node := range graph.getNodes() {
		node.inclusionTreeDepth = depth

		if node.child != nil {
			gm.calcInclusionTreeDepths(node.child, depth+1)
		}
	}
}

// includesInvalidEdge removes the edges between a node and one of its
// ancestors in the JavaScript. The port does not remove edges: such an edge
// is not covered.
func (gm *GraphManager) includesInvalidEdge() bool {
	for _, edge := range gm.edges {
		if gm.isOneAncestorOfOther(edge.source, edge.target) {
			bail.Out("fcose: an edge between a node and its ancestor")
		}
	}

	// Invalid edges are cleared, so return false
	return false
}
