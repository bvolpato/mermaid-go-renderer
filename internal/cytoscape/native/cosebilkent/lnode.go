// Ported from layout-base 1.0.2 (MIT license): src/LNode.js and
// src/fd/FDLayoutNode.js, and from cose-base 1.0.3 (MIT license):
// src/CoSENode.js.
//
// The JavaScript has the classes LNode, FDLayoutNode and CoSENode, each one
// an extension of the one before. The layout creates CoSENode objects only,
// so Node has the fields of the three.

package cosebilkent

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
)

// Node is a CoSENode.
type Node struct {
	// id is the id that the cytoscape extension attaches. hasID is false for
	// the node of the root graph, whose id is undefined.
	id    string
	hasID bool

	// LNode.
	estimatedSize      float64
	inclusionTreeDepth int
	edges              []*Edge
	graphManager       *GraphManager
	rect               RectangleD
	owner              *Graph
	child              *Graph

	// FDLayoutNode: the spring, repulsion and gravitational forces that act
	// on the node.
	springForceX, springForceY           float64
	repulsionForceX, repulsionForceY     float64
	gravitationForceX, gravitationForceY float64
	// The amount by which the node moves in this iteration.
	displacementX, displacementY float64
	// The start and finish grid coordinates that the node falls into.
	startX, finishX, startY, finishY int
	// The geometric neighbors of the node.
	surrounding []*Node

	noOfChildren float64

	// processed and inSurrounding stand for the sets processedNodeSet and
	// surrounding of FDLayout.calcRepulsionForces: the node is a member when
	// the field holds the stamp of the set.
	processed, inSurrounding uint64
}

// newNode is new CoSENode(gm, loc, size): the constructor of LNode with a
// location and a size.
func newNode(gm *GraphManager, loc PointD, size PointD) *Node {
	return &Node{
		estimatedSize:      integerMinValue,
		inclusionTreeDepth: integerMaxValue,
		graphManager:       gm,
		rect:               RectangleD{loc.x, loc.y, size.x, size.y},
	}
}

// newNodeWithoutRect is new CoSENode(gm, vNode): the rectangle is empty.
func newNodeWithoutRect(gm *GraphManager) *Node {
	return &Node{
		estimatedSize:      integerMinValue,
		inclusionTreeDepth: integerMaxValue,
		graphManager:       gm,
	}
}

func (n *Node) getEdges() []*Edge  { return n.edges }
func (n *Node) getChild() *Graph   { return n.child }
func (n *Node) getOwner() *Graph   { return n.owner }
func (n *Node) getWidth() float64  { return n.rect.width }
func (n *Node) getHeight() float64 { return n.rect.height }

func (n *Node) getCenterX() float64 { return n.rect.x + float64(n.rect.width/2) }
func (n *Node) getCenterY() float64 { return n.rect.y + float64(n.rect.height/2) }

func (n *Node) getRect() *RectangleD { return &n.rect }

func (n *Node) getDiagonal() float64 {
	return math.Sqrt(float64(n.rect.width*n.rect.width) + float64(n.rect.height*n.rect.height))
}

func (n *Node) setCenter(cx, cy float64) {
	n.rect.x = cx - float64(n.rect.width/2)
	n.rect.y = cy - float64(n.rect.height/2)
}

func (n *Node) setLocation(x, y float64) {
	n.rect.x = x
	n.rect.y = y
}

func (n *Node) moveBy(dx, dy float64) {
	n.rect.x += dx
	n.rect.y += dy
}

func (n *Node) getEdgesBetween(other *Node) []*Edge {
	var edgeList []*Edge

	for _, edge := range n.edges {
		if !(edge.source == n || edge.target == n) {
			panic("Incorrect edge source and/or target")
		}

		if edge.target == other || edge.source == other {
			edgeList = append(edgeList, edge)
		}
	}

	return edgeList
}

// getNeighborsList returns the neighbors in the order in which a JavaScript
// Set enumerates them: each node one time, in the order of the edges.
func (n *Node) getNeighborsList() []*Node {
	var neighbors []*Node
	add := func(node *Node) {
		for _, x := range neighbors {
			if x == node {
				return
			}
		}
		neighbors = append(neighbors, node)
	}

	for _, edge := range n.edges {
		if edge.source == n {
			add(edge.target)
		} else {
			if edge.target != n {
				panic("Incorrect incidency!")
			}

			add(edge.source)
		}
	}

	return neighbors
}

// withChildren returns the node and the nodes inside it, in the order of the
// JavaScript Set.
func (n *Node) withChildren() []*Node {
	withNeighborsList := []*Node{n}

	if n.child != nil {
		bail.Out("cose-bilkent: a compound node")
	}

	return withNeighborsList
}

func (n *Node) getNoOfChildren() float64 {
	noOfChildren := 0.0

	if n.child == nil {
		noOfChildren = 1
	} else {
		nodes := n.child.getNodes()
		for _, childNode := range nodes {
			noOfChildren += childNode.getNoOfChildren()
		}
	}

	if noOfChildren == 0 {
		noOfChildren = 1
	}
	return noOfChildren
}

func (n *Node) getEstimatedSize() float64 {
	if n.estimatedSize == integerMinValue {
		panic("assert failed")
	}
	return n.estimatedSize
}

func (n *Node) calcEstimatedSize() float64 {
	if n.child == nil {
		n.estimatedSize = (n.rect.width + n.rect.height) / 2
		return n.estimatedSize
	}
	n.estimatedSize = n.child.calcEstimatedSize()
	n.rect.width = n.estimatedSize
	n.rect.height = n.estimatedSize

	return n.estimatedSize
}

func (n *Node) transform(trans *Transform) {
	left := n.rect.x

	if left > worldBoundary {
		left = worldBoundary
	} else if left < -worldBoundary {
		left = -worldBoundary
	}

	top := n.rect.y

	if top > worldBoundary {
		top = worldBoundary
	} else if top < -worldBoundary {
		top = -worldBoundary
	}

	leftTop := PointD{left, top}
	vLeftTop := trans.inverseTransformPoint(leftTop)

	n.setLocation(vLeftTop.x, vLeftTop.y)
}

func (n *Node) getLeft() float64   { return n.rect.x }
func (n *Node) getRight() float64  { return n.rect.x + n.rect.width }
func (n *Node) getTop() float64    { return n.rect.y }
func (n *Node) getBottom() float64 { return n.rect.y + n.rect.height }

func (n *Node) getParent() *Node {
	if n.owner == nil {
		return nil
	}

	return n.owner.getParent()
}

// setGridCoordinates is FDLayoutNode.setGridCoordinates.
func (n *Node) setGridCoordinates(startX, finishX, startY, finishY int) {
	n.startX = startX
	n.finishX = finishX
	n.startY = startY
	n.finishY = finishY
}

// move is CoSENode.move: it moves the node by the forces that act on it, and
// clears the forces.
func (n *Node) move() {
	layout := n.graphManager.getLayout()
	n.displacementX = layout.coolingFactor * (n.springForceX + n.repulsionForceX + n.gravitationForceX) / n.noOfChildren
	n.displacementY = layout.coolingFactor * (n.springForceY + n.repulsionForceY + n.gravitationForceY) / n.noOfChildren

	if math.Abs(n.displacementX) > layout.coolingFactor*layout.maxNodeDisplacement {
		n.displacementX = float64(layout.coolingFactor * layout.maxNodeDisplacement * iMathSign(n.displacementX))
	}

	if math.Abs(n.displacementY) > layout.coolingFactor*layout.maxNodeDisplacement {
		n.displacementY = float64(layout.coolingFactor * layout.maxNodeDisplacement * iMathSign(n.displacementY))
	}

	// a simple node, just move it
	if n.child == nil {
		n.moveBy(n.displacementX, n.displacementY)
	} else if len(n.child.getNodes()) == 0 {
		// an empty compound node, again just move it
		n.moveBy(n.displacementX, n.displacementY)
	} else {
		// non-empty compound node, propogate movement to children as well
		bail.Out("cose-bilkent: a compound node")
	}

	layout.totalDisplacement += math.Abs(n.displacementX) + math.Abs(n.displacementY)

	n.springForceX = 0
	n.springForceY = 0
	n.repulsionForceX = 0
	n.repulsionForceY = 0
	n.gravitationForceX = 0
	n.gravitationForceY = 0
	n.displacementX = 0
	n.displacementY = 0
}
