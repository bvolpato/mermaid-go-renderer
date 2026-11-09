// Ported from layout-base 1.0.2 (MIT license): src/LGraphManager.js, and
// from cose-base 1.0.3 (MIT license): src/CoSEGraphManager.js.

package cosebilkent

import "github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"

// GraphManager is a CoSEGraphManager: the classes LGraphManager and
// CoSEGraphManager.
type GraphManager struct {
	layout *Layout
	graphs []*Graph
	// edges are the inter-graph edges.
	edges     []*Edge
	rootGraph *Graph

	// allNodes and allEdges are caches. nil stands for null.
	allNodes []*Node
	allEdges []*Edge
	// The JavaScript distinguishes an empty list from null.
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
	bail.Out("cose-bilkent: an edge between two graphs")
	return nil
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

func (gm *GraphManager) resetAllNodes() {
	gm.allNodes = nil
	gm.hasAllNodes = false
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

func (gm *GraphManager) includesInvalidEdge() bool {
	if len(gm.edges) > 0 {
		bail.Out("cose-bilkent: an edge between two graphs")
	}
	return false
}
