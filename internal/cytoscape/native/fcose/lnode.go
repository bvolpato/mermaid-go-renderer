// Ported from layout-base 2.0.1 (MIT license): src/LNode.js and
// src/fd/FDLayoutNode.js, and from cose-base 2.2.0 (MIT license):
// src/CoSENode.js.
//
// The JavaScript has the classes LNode, FDLayoutNode and CoSENode, each one
// an extension of the one before. The layout creates CoSENode objects only,
// so Node has the fields of the three.

package fcose

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

	// The padding that the extension attaches: parseInt(css('padding')).
	// hasPadding is false for the node of the root graph.
	paddingLeft float64
	hasPadding  bool

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
	// nodeRepulsion is the repulsion value of the node.
	nodeRepulsion float64

	noOfChildren float64

	// processed and inSurrounding stand for the sets processedNodeSet and
	// surrounding of FDLayout.calcRepulsionForces: the node is a member when
	// the field holds the stamp of the set.
	processed, inSurrounding uint64
	// visited stands for a Set of the JavaScript in the same way.
	visited uint64
}

// newNode is new CoSENode(gm, loc, size): the constructor of LNode with a
// location and a size.
func newNode(gm *GraphManager, loc PointD, size PointD) *Node {
	return &Node{
		estimatedSize:      integerMinValue,
		inclusionTreeDepth: integerMaxValue,
		graphManager:       gm,
		rect:               RectangleD{loc.x, loc.y, size.x, size.y},
		nodeRepulsion:      defaultRepulsionStrength,
	}
}

// newNodeWithoutRect is new CoSENode(gm, vNode): the rectangle is empty.
func newNodeWithoutRect(gm *GraphManager) *Node {
	return &Node{
		estimatedSize:      integerMinValue,
		inclusionTreeDepth: integerMaxValue,
		graphManager:       gm,
		nodeRepulsion:      defaultRepulsionStrength,
	}
}

func (n *Node) getEdges() []*Edge  { return n.edges }
func (n *Node) getChild() *Graph   { return n.child }
func (n *Node) getOwner() *Graph   { return n.owner }
func (n *Node) getWidth() float64  { return n.rect.width }
func (n *Node) getHeight() float64 { return n.rect.height }

func (n *Node) setWidth(width float64)   { n.rect.width = width }
func (n *Node) setHeight(height float64) { n.rect.height = height }

func (n *Node) getCenterX() float64 { return n.rect.x + float64(n.rect.width/2) }
func (n *Node) getCenterY() float64 { return n.rect.y + float64(n.rect.height/2) }

func (n *Node) getRect() *RectangleD { return &n.rect }

func (n *Node) setCenter(cx, cy float64) {
	n.rect.x = cx - float64(n.rect.width/2)
	n.rect.y = cy - float64(n.rect.height/2)
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

// withChildren appends the node and the nodes inside it to list, in the
// order of the JavaScript Set.
func (n *Node) withChildren(list []*Node) []*Node {
	list = append(list, n)

	if n.child != nil {
		for _, childNode := range n.child.getNodes() {
			list = childNode.withChildren(list)
		}
	}

	return list
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

// updateBounds is LNode.updateBounds: a compound node wraps its children.
func (n *Node) updateBounds() {
	if n.getChild() == nil {
		panic("assert failed")
	}
	if len(n.getChild().getNodes()) != 0 {
		// wrap the children nodes by re-arranging the boundaries
		childGraph := n.getChild()
		childGraph.updateBounds(true)

		n.rect.x = childGraph.getLeft()
		n.rect.y = childGraph.getTop()

		n.setWidth(childGraph.getRight() - childGraph.getLeft())
		n.setHeight(childGraph.getBottom() - childGraph.getTop())

		// Update compound bounds considering its label properties
		if nodeDimensionsIncludeLabels {
			bail.Out("fcose: labels in the node dimensions")
		}
	}
}

func (n *Node) getInclusionTreeDepth() int {
	if n.inclusionTreeDepth == integerMaxValue {
		panic("assert failed")
	}
	return n.inclusionTreeDepth
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

// calculateDisplacement is CoSENode.calculateDisplacement.
func (n *Node) calculateDisplacement() {
	layout := n.graphManager.getLayout()
	// The JavaScript divides by fixedNodeWeight for a compound node that
	// contains fixed nodes. The port has no fixed node constraint.
	n.displacementX += layout.coolingFactor * (n.springForceX + n.repulsionForceX + n.gravitationForceX) / n.noOfChildren
	n.displacementY += layout.coolingFactor * (n.springForceY + n.repulsionForceY + n.gravitationForceY) / n.noOfChildren

	if math.Abs(n.displacementX) > layout.coolingFactor*layout.maxNodeDisplacement {
		n.displacementX = float64(layout.coolingFactor * layout.maxNodeDisplacement * iMathSign(n.displacementX))
	}

	if math.Abs(n.displacementY) > layout.coolingFactor*layout.maxNodeDisplacement {
		n.displacementY = float64(layout.coolingFactor * layout.maxNodeDisplacement * iMathSign(n.displacementY))
	}

	// non-empty compound node, propogate movement to children as well
	if n.child != nil && len(n.child.getNodes()) > 0 {
		n.propogateDisplacementToChildren(n.displacementX, n.displacementY)
	}
}

func (n *Node) propogateDisplacementToChildren(dX, dY float64) {
	nodes := n.getChild().getNodes()
	for _, node := range nodes {
		if node.getChild() == nil {
			node.displacementX += dX
			node.displacementY += dY
		} else {
			node.propogateDisplacementToChildren(dX, dY)
		}
	}
}

// move is CoSENode.move.
func (n *Node) move() {
	layout := n.graphManager.getLayout()

	// a simple node or an empty compound node, move it
	if n.child == nil || len(n.child.getNodes()) == 0 {
		n.moveBy(n.displacementX, n.displacementY)

		layout.totalDisplacement += math.Abs(n.displacementX) + math.Abs(n.displacementY)
	}

	n.springForceX = 0
	n.springForceY = 0
	n.repulsionForceX = 0
	n.repulsionForceY = 0
	n.gravitationForceX = 0
	n.gravitationForceY = 0
	n.displacementX = 0
	n.displacementY = 0
}
