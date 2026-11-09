// Ported from layout-base 1.0.2 (MIT license): src/LGraph.js, and from
// cose-base 1.0.3 (MIT license): src/CoSEGraph.js.

package cosebilkent

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
)

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

// updateLeftTop returns the left top corner of the nodes with the margin.
// ok is false when the graph has no node.
func (g *Graph) updateLeftTop() (leftTop PointD, ok bool) {
	top := float64(integerMaxValue)
	left := float64(integerMaxValue)
	var margin float64

	nodes := g.getNodes()

	for _, lNode := range nodes {
		nodeTop := lNode.getTop()
		nodeLeft := lNode.getLeft()

		if top > nodeTop {
			top = nodeTop
		}

		if left > nodeLeft {
			left = nodeLeft
		}
	}

	// Do we have any nodes in this graph?
	if top == integerMaxValue {
		return PointD{}, false
	}

	if nodes[0].getParent().hasPaddingLeft() {
		bail.Out("cose-bilkent: a compound node with a padding")
	} else {
		margin = g.margin
	}

	g.left = left - margin
	g.top = top - margin

	// Apply the margins and return the result
	return PointD{g.left, g.top}, true
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
			bail.Out("cose-bilkent: a compound node")
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
	if nodes[0].getParent().hasPaddingLeft() {
		bail.Out("cose-bilkent: a compound node with a padding")
	} else {
		margin = g.margin
	}

	g.left = boundingRect.x - margin
	g.right = boundingRect.x + boundingRect.width + margin
	g.top = boundingRect.y - margin
	g.bottom = boundingRect.y + boundingRect.height + margin
}

// calculateBounds is LGraph.calculateBounds.
func calculateBounds(nodes []*Node) RectangleD {
	left := float64(integerMaxValue)
	right := -float64(integerMaxValue)
	top := float64(integerMaxValue)
	bottom := -float64(integerMaxValue)

	for _, lNode := range nodes {
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

	return RectangleD{left, top, right - left, bottom - top}
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

	// queue is the LinkedList of the JavaScript, and visited its Set, which
	// keeps the order of insertion.
	var queue []*Node
	visited := map[*Node]bool{}
	var visitedOrder []*Node
	currentNode := g.nodes[0]
	childrenOfNode := currentNode.withChildren()
	for _, node := range childrenOfNode {
		queue = append(queue, node)
		if !visited[node] {
			visited[node] = true
			visitedOrder = append(visitedOrder, node)
		}
	}

	for head := 0; head < len(queue); head++ {
		currentNode = queue[head]

		// Traverse all neighbors of this node
		neighborEdges := currentNode.getEdges()
		for _, neighborEdge := range neighborEdges {
			currentNeighbor := neighborEdge.getOtherEndInGraph(currentNode, g)

			// Add unvisited neighbors to the list to visit
			if currentNeighbor != nil && !visited[currentNeighbor] {
				childrenOfNeighbor := currentNeighbor.withChildren()

				for _, node := range childrenOfNeighbor {
					queue = append(queue, node)
					if !visited[node] {
						visited[node] = true
						visitedOrder = append(visitedOrder, node)
					}
				}
			}
		}
	}

	g.isConnected = false

	if len(visitedOrder) >= len(g.nodes) {
		noOfVisitedInThisGraph := 0

		for _, visitedNode := range visitedOrder {
			if visitedNode.owner == g {
				noOfVisitedInThisGraph++
			}
		}

		if noOfVisitedInThisGraph == len(g.nodes) {
			g.isConnected = true
		}
	}
}

// hasPaddingLeft is the test `node.paddingLeft != undefined`. The extension
// gives a padding to each node of cytoscape. The node of the root graph has
// none.
func (n *Node) hasPaddingLeft() bool { return n.hasID }
