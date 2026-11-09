// Ported from layout-base 2.0.1 (MIT license): src/LGraph.js, and from
// cose-base 2.2.0 (MIT license): src/CoSEGraph.js.

package fcose

import "math"

// Graph is a CoSEGraph: the classes LGraph and CoSEGraph.
type Graph struct {
	estimatedSize float64
	margin        float64
	edges         []*Edge
	nodes         []*Node
	isConnected   bool
	parent        *Node
	graphManager  *GraphManager

	// The bounds are undefined in JavaScript until updateBounds runs.
	left, right, top, bottom float64
}

// newGraph is new CoSEGraph(null, graphMgr, vGraph).
func newGraph(graphMgr *GraphManager) *Graph {
	return &Graph{
		estimatedSize: integerMinValue,
		margin:        defaultGraphMargin,
		graphManager:  graphMgr,
		left:          nan, right: nan, top: nan, bottom: nan,
	}
}

func (g *Graph) getNodes() []*Node              { return g.nodes }
func (g *Graph) getEdges() []*Edge              { return g.edges }
func (g *Graph) getGraphManager() *GraphManager { return g.graphManager }
func (g *Graph) getParent() *Node               { return g.parent }
func (g *Graph) getLeft() float64               { return g.left }
func (g *Graph) getRight() float64              { return g.right }
func (g *Graph) getTop() float64                { return g.top }
func (g *Graph) getBottom() float64             { return g.bottom }

func (g *Graph) indexOfNode(node *Node) int {
	for i, n := range g.nodes {
		if n == node {
			return i
		}
	}
	return -1
}

// addNode is LGraph.add with one argument.
func (g *Graph) addNode(newNode *Node) *Node {
	if g.graphManager == nil {
		panic("Graph has no graph mgr!")
	}
	if g.indexOfNode(newNode) > -1 {
		panic("Node already in graph!")
	}
	newNode.owner = g
	g.nodes = append(g.nodes, newNode)

	return newNode
}

// addEdge is LGraph.add with an edge, its source and its target.
func (g *Graph) addEdge(newEdge *Edge, sourceNode, targetNode *Node) *Edge {
	if !(g.indexOfNode(sourceNode) > -1 && g.indexOfNode(targetNode) > -1) {
		panic("Source or target not in graph!")
	}

	if !(sourceNode.owner == targetNode.owner && sourceNode.owner == g) {
		panic("Both owners must be this graph!")
	}

	// set source and target
	newEdge.source = sourceNode
	newEdge.target = targetNode

	// set as intra-graph edge
	newEdge.isInterGraph = false

	// add to graph edge list
	g.edges = append(g.edges, newEdge)

	// add to incidency lists
	sourceNode.edges = append(sourceNode.edges, newEdge)

	if targetNode != sourceNode {
		targetNode.edges = append(targetNode.edges, newEdge)
	}

	return newEdge
}

func (g *Graph) updateBounds(recursive bool) {
	// calculate bounds
	left := float64(integerMaxValue)
	right := -float64(integerMaxValue)
	top := float64(integerMaxValue)
	bottom := -float64(integerMaxValue)
	var margin float64

	nodes := g.nodes
	for _, lNode := range nodes {
		if recursive && lNode.child != nil {
			lNode.updateBounds()
		}
		nodeLeft := lNode.getLeft()
		nodeRight := lNode.getRight()
		nodeTop := lNode.getTop()
		nodeBottom := lNode.getBottom()

		if left > nodeLeft {
			left = nodeLeft
		}

		if right < nodeRight {
			right = nodeRight
		}

		if top > nodeTop {
			top = nodeTop
		}

		if bottom < nodeBottom {
			bottom = nodeBottom
		}
	}

	boundingRect := RectangleD{left, top, right - left, bottom - top}
	if left == integerMaxValue {
		g.left = g.parent.getLeft()
		g.right = g.parent.getRight()
		g.top = g.parent.getTop()
		g.bottom = g.parent.getBottom()
	}

	// The JavaScript reads nodes[0] and throws for a graph without nodes.
	if parent := nodes[0].getParent(); parent.hasPadding {
		margin = parent.paddingLeft
	} else {
		margin = g.margin
	}

	g.left = boundingRect.x - margin
	g.right = boundingRect.x + boundingRect.width + margin
	g.top = boundingRect.y - margin
	g.bottom = boundingRect.y + boundingRect.height + margin
}

func (g *Graph) getInclusionTreeDepth() int {
	if g == g.graphManager.getRoot() {
		return 1
	}
	return g.parent.getInclusionTreeDepth()
}

func (g *Graph) getEstimatedSize() float64 {
	if g.estimatedSize == integerMinValue {
		panic("assert failed")
	}
	return g.estimatedSize
}

func (g *Graph) calcEstimatedSize() float64 {
	size := 0.0

	for _, lNode := range g.nodes {
		size += lNode.calcEstimatedSize()
	}

	if size == 0 {
		g.estimatedSize = emptyCompoundNodeSize
	} else {
		g.estimatedSize = size / math.Sqrt(float64(len(g.nodes)))
	}

	return g.estimatedSize
}

// updateConnected sets isConnected: whether the nodes of the graph are one
// component.
func (g *Graph) updateConnected() {
	if len(g.nodes) == 0 {
		g.isConnected = true
		return
	}

	// queue is the LinkedList of the JavaScript, and visited its Set: the
	// list has the members, and the stamp tells the membership.
	layout := g.graphManager.layout
	layout.stamp++
	stamp := layout.stamp
	var visited []*Node
	var queue []*Node
	currentNode := g.nodes[0]
	for _, node := range currentNode.withChildren(nil) {
		queue = append(queue, node)
		if node.visited != stamp {
			node.visited = stamp
			visited = append(visited, node)
		}
	}

	for head := 0; head < len(queue); head++ {
		currentNode = queue[head]

		// Traverse all neighbors of this node
		neighborEdges := currentNode.getEdges()
		for _, neighborEdge := range neighborEdges {
			currentNeighbor := neighborEdge.getOtherEndInGraph(currentNode, g)

			// Add unvisited neighbors to the list to visit
			if currentNeighbor != nil && currentNeighbor.visited != stamp {
				for _, node := range currentNeighbor.withChildren(nil) {
					queue = append(queue, node)
					if node.visited != stamp {
						node.visited = stamp
						visited = append(visited, node)
					}
				}
			}
		}
	}

	g.isConnected = false

	if len(visited) >= len(g.nodes) {
		noOfVisitedInThisGraph := 0

		for _, visitedNode := range visited {
			if visitedNode.owner == g {
				noOfVisitedInThisGraph++
			}
		}

		if noOfVisitedInThisGraph == len(g.nodes) {
			g.isConnected = true
		}
	}
}
