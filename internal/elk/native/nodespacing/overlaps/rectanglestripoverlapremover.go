// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/overlaps/RectangleStripOverlapRemover.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package overlaps has the overlap removal for rectangles along a strip
// (org.eclipse.elk.alg.common.overlaps). The port label placement and the
// end label placement use it.
//
// TODO(core): the Java package is a sibling of nodespacing. Move this
// package to internal/elk/native/overlaps.
package overlaps

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// OverlapRemovalDirection is the enum
// RectangleStripOverlapRemover.OverlapRemovalDirection: the direction in
// which the rectangles move away from the start coordinate.
type OverlapRemovalDirection int

// The constants of OverlapRemovalDirection, in the order of the Java enum.
const (
	OverlapRemovalDirectionUp OverlapRemovalDirection = iota
	OverlapRemovalDirectionDown
	OverlapRemovalDirectionLeft
	OverlapRemovalDirectionRight
)

var namesOverlapRemovalDirection = []string{"UP", "DOWN", "LEFT", "RIGHT"}

// String returns the name of the Java constant.
func (e OverlapRemovalDirection) String() string {
	return prop.EnumName(namesOverlapRemovalDirection, int(e))
}

// defaultGap is DEFAULT_GAP.
const defaultGap = 5

// RectangleNode is the class RectangleStripOverlapRemover.RectangleNode: a
// rectangle with the rectangles that overlap it along the strip.
type RectangleNode struct {
	// originalRectangle is the rectangle that the client added.
	originalRectangle *emath.ElkRectangle
	// rectangle is the rectangle in the coordinate system of the algorithm.
	rectangle *emath.ElkRectangle
	// overlappingNodes has the nodes that this node overlaps.
	overlappingNodes []*RectangleNode
}

// Rectangle is getRectangle().
func (n *RectangleNode) Rectangle() *emath.ElkRectangle { return n.rectangle }

// OverlappingNodes is getOverlappingNodes(). The slice shares the memory of
// the node, so a caller can sort it in place.
func (n *RectangleNode) OverlappingNodes() []*RectangleNode { return n.overlappingNodes }

// Strategy is IRectangleStripOverlapRemovalStrategy: it removes the overlaps
// of the rectangle nodes of a remover and returns the size of the strip.
type Strategy interface {
	RemoveOverlaps(overlapRemover *RectangleStripOverlapRemover) float64
}

// RectangleStripOverlapRemover is the class RectangleStripOverlapRemover: it
// moves rectangles along one axis so that no two of them overlap.
type RectangleStripOverlapRemover struct {
	overlapRemovalDirection OverlapRemovalDirection
	gapVertical             float64
	gapHorizontal           float64
	startCoordinate         float64
	overlapRemovalStrategy  Strategy
	rectangleNodes          []*RectangleNode
}

// CreateForDirection is createForDirection(direction).
func CreateForDirection(direction OverlapRemovalDirection) *RectangleStripOverlapRemover {
	return &RectangleStripOverlapRemover{
		overlapRemovalDirection: direction,
		gapVertical:             defaultGap,
		gapHorizontal:           defaultGap,
	}
}

// WithGap is withGap(theHorizontalGap, theVerticalGap). It returns the
// remover.
func (r *RectangleStripOverlapRemover) WithGap(theHorizontalGap, theVerticalGap float64) *RectangleStripOverlapRemover {
	r.gapHorizontal = theHorizontalGap
	r.gapVertical = theVerticalGap
	return r
}

// WithStartCoordinate is withStartCoordinate(coordinate). It returns the
// remover.
func (r *RectangleStripOverlapRemover) WithStartCoordinate(coordinate float64) *RectangleStripOverlapRemover {
	r.startCoordinate = coordinate
	return r
}

// WithOverlapRemovalStrategy is withOverlapRemovalStrategy(strategy). It
// returns the remover.
func (r *RectangleStripOverlapRemover) WithOverlapRemovalStrategy(strategy Strategy) *RectangleStripOverlapRemover {
	r.overlapRemovalStrategy = strategy
	return r
}

// AddRectangle is addRectangle(rectangle). RemoveOverlaps changes the
// rectangle. It returns the remover.
func (r *RectangleStripOverlapRemover) AddRectangle(rectangle *emath.ElkRectangle) *RectangleStripOverlapRemover {
	r.rectangleNodes = append(r.rectangleNodes, &RectangleNode{
		originalRectangle: rectangle,
		rectangle:         r.importRectangle(rectangle),
	})
	return r
}

// HorizontalGap is getHorizontalGap().
func (r *RectangleStripOverlapRemover) HorizontalGap() float64 { return r.gapHorizontal }

// VerticalGap is getVerticalGap().
func (r *RectangleStripOverlapRemover) VerticalGap() float64 { return r.gapVertical }

// RectangleNodes is getRectangleNodes().
func (r *RectangleStripOverlapRemover) RectangleNodes() []*RectangleNode { return r.rectangleNodes }

// importRectangle is importRectangle(rectangle): the rectangle in the
// coordinate system of the algorithm, where the strip grows downwards.
func (r *RectangleStripOverlapRemover) importRectangle(rectangle *emath.ElkRectangle) *emath.ElkRectangle {
	switch r.overlapRemovalDirection {
	case OverlapRemovalDirectionUp, OverlapRemovalDirectionDown:
		return rectangle
	case OverlapRemovalDirectionLeft, OverlapRemovalDirectionRight:
		return emath.NewElkRectangle(rectangle.Y, 0, rectangle.Height, rectangle.Width)
	}
	lgraph.Bail("RectangleStripOverlapRemover: unknown overlap removal direction %d", int(r.overlapRemovalDirection))
	return nil
}

// exportRectangle is exportRectangle(rectangleNode, stripSize): it applies
// the result to the rectangle of the client.
func (r *RectangleStripOverlapRemover) exportRectangle(rectangleNode *RectangleNode) {
	rectangle := rectangleNode.rectangle
	originalRectangle := rectangleNode.originalRectangle

	switch r.overlapRemovalDirection {
	case OverlapRemovalDirectionUp:
		originalRectangle.Y = r.startCoordinate - rectangle.Height - rectangle.Y
	case OverlapRemovalDirectionDown:
		originalRectangle.Y += r.startCoordinate
	case OverlapRemovalDirectionLeft:
		originalRectangle.X = r.startCoordinate - rectangle.Height - rectangle.Y
	case OverlapRemovalDirectionRight:
		originalRectangle.X = r.startCoordinate + rectangle.Y
	}
}

// RemoveOverlaps is removeOverlaps(): it removes the overlaps, changes the
// rectangles of the client, and returns the size of the strip.
func (r *RectangleStripOverlapRemover) RemoveOverlaps() float64 {
	// Check if an overlap removal strategy is used; if not, fall back on the
	// greedy strategy
	if r.overlapRemovalStrategy == nil {
		r.overlapRemovalStrategy = NewGreedyRectangleStripOverlapRemover()
	}

	// Sort the list of rectangles
	gwt.SortFunc(r.rectangleNodes, compareLeftRectangleBorders)

	// Compute and remove overlaps
	r.computeOverlaps()
	stripSize := r.overlapRemovalStrategy.RemoveOverlaps(r)

	// Apply the results
	for _, node := range r.rectangleNodes {
		r.exportRectangle(node)
	}

	return stripSize
}

// computeOverlaps is computeOverlaps(): a scan from left to right that
// finds the nodes that share a part of the x axis.
func (r *RectangleStripOverlapRemover) computeOverlaps() {
	// The Java set is a TreeSet with compareRightRectangleBorders: it is in
	// the order of the right borders, and it does not take a node whose
	// right border is equal to that of a node of the set.
	var intersectingNodes []*RectangleNode

	// We iterate over the nodes according to their x coordinate
	for _, currNode := range r.rectangleNodes {
		// Move the scanline to the new node's left border
		scanlinePos := currNode.rectangle.X

		// Remove intersecting node which do not intersect the scanline
		// anymore
		for len(intersectingNodes) != 0 {
			intersectingRectangle := intersectingNodes[0]
			if intersectingRectangle.rectangle.X+intersectingRectangle.rectangle.Width < scanlinePos {
				intersectingNodes = intersectingNodes[1:]
			} else {
				// Since we iterate over intersecting nodes by the coordinate
				// of their right border, once we have found a node the
				// scanline has not moved past, we can stop looking
				break
			}
		}

		// Add overlaps between the currently intersecting nodes and the new
		// node
		for _, intersectingNode := range intersectingNodes {
			intersectingNode.overlappingNodes = append(intersectingNode.overlappingNodes, currNode)
			currNode.overlappingNodes = append(currNode.overlappingNodes, intersectingNode)
		}

		// The new node is now part of the set of rectangles that intersect
		// the scanline
		intersectingNodes = addToSortedSet(intersectingNodes, currNode)
	}
}

// addToSortedSet is TreeSet.add for a set in the order of
// compareRightRectangleBorders.
func addToSortedSet(set []*RectangleNode, node *RectangleNode) []*RectangleNode {
	index := len(set)
	for i, other := range set {
		c := compareRightRectangleBorders(node, other)
		if c == 0 {
			return set
		}
		if c < 0 {
			index = i
			break
		}
	}
	// The set can be a tail of a longer slice, so the insert makes a new
	// slice.
	out := make([]*RectangleNode, 0, len(set)+1)
	out = append(out, set[:index]...)
	out = append(out, node)
	return append(out, set[index:]...)
}

// compareLeftRectangleBorders is compareLeftRectangleBorders(rn1, rn2).
func compareLeftRectangleBorders(rn1, rn2 *RectangleNode) int {
	return gwt.DoubleCompare(rn1.rectangle.X, rn2.rectangle.X)
}

// compareRightRectangleBorders is compareRightRectangleBorders(rn1, rn2).
func compareRightRectangleBorders(rn1, rn2 *RectangleNode) int {
	return gwt.DoubleCompare(rn1.rectangle.X+rn1.rectangle.Width, rn2.rectangle.X+rn2.rectangle.Width)
}
