// Ported from layout-base 1.0.2 (MIT license): src/LEdge.js and
// src/fd/FDLayoutEdge.js, and from cose-base 1.0.3 (MIT license):
// src/CoSEEdge.js.

package cosebilkent

import "math"

// Edge is a CoSEEdge: the classes LEdge, FDLayoutEdge and CoSEEdge.
type Edge struct {
	id string

	// LEdge.
	isOverlapingSourceAndTarget bool
	source, target              *Node
	isInterGraph                bool
	length, lengthX, lengthY    float64
	lca                         *Graph
	sourceInLca, targetInLca    *Node

	// FDLayoutEdge.
	idealLength float64
}

// newEdge is new CoSEEdge(null, null, vEdge).
func newEdge() *Edge {
	// A length that was never computed is undefined in JavaScript.
	return &Edge{idealLength: defaultEdgeLength, length: nan, lengthX: nan, lengthY: nan}
}

func (e *Edge) getSource() *Node   { return e.source }
func (e *Edge) getTarget() *Node   { return e.target }
func (e *Edge) getLength() float64 { return e.length }

func (e *Edge) getOtherEnd(node *Node) *Node {
	if e.source == node {
		return e.target
	} else if e.target == node {
		return e.source
	}
	panic("Node is not incident with this edge")
}

func (e *Edge) getOtherEndInGraph(node *Node, graph *Graph) *Node {
	otherEnd := e.getOtherEnd(node)
	root := graph.getGraphManager().getRoot()

	for {
		if otherEnd.getOwner() == graph {
			return otherEnd
		}

		if otherEnd.getOwner() == root {
			break
		}

		otherEnd = otherEnd.getOwner().getParent()
	}

	return nil
}

func (e *Edge) updateLength() {
	clipPointCoordinates := [4]float64{nan, nan, nan, nan}

	e.isOverlapingSourceAndTarget = getIntersection2(e.target.getRect(), e.source.getRect(), &clipPointCoordinates)

	if !e.isOverlapingSourceAndTarget {
		e.lengthX = clipPointCoordinates[0] - clipPointCoordinates[2]
		e.lengthY = clipPointCoordinates[1] - clipPointCoordinates[3]

		if math.Abs(e.lengthX) < 1.0 {
			e.lengthX = iMathSign(e.lengthX)
		}

		if math.Abs(e.lengthY) < 1.0 {
			e.lengthY = iMathSign(e.lengthY)
		}

		e.length = math.Sqrt(float64(e.lengthX*e.lengthX) + float64(e.lengthY*e.lengthY))
	}
}
