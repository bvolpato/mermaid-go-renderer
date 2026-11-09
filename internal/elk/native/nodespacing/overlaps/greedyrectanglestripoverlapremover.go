// Ported from GreedyRectangleStripOverlapRemover.java and
// IRectangleStripOverlapRemovalStrategy.java in
// plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/overlaps
// of Eclipse ELK 0.9.1 (EPL-2.0).

package overlaps

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
)

// GreedyRectangleStripOverlapRemover is the class
// GreedyRectangleStripOverlapRemover: it places each rectangle, from left to
// right, at the first y coordinate where it overlaps no rectangle that is
// placed.
type GreedyRectangleStripOverlapRemover struct{}

// NewGreedyRectangleStripOverlapRemover is new
// GreedyRectangleStripOverlapRemover().
func NewGreedyRectangleStripOverlapRemover() *GreedyRectangleStripOverlapRemover {
	return &GreedyRectangleStripOverlapRemover{}
}

// RemoveOverlaps is removeOverlaps(overlapRemover).
func (*GreedyRectangleStripOverlapRemover) RemoveOverlaps(overlapRemover *RectangleStripOverlapRemover) float64 {
	verticalGap := overlapRemover.VerticalGap()

	alreadyPlacedNodes := gwt.NewSet[*RectangleNode]()
	stripSize := 0.0

	for _, currNode := range overlapRemover.RectangleNodes() {
		// We start with an initial y coordinate of zero
		yPos := 0.0

		// Sort the node's list of overlapping nodes by y coordinate
		gwt.SortFunc(currNode.OverlappingNodes(), CompareByYCoordinate)

		// We now iterate over every conflicting rectangle node that we have
		// already placed to see if the current coordinate would cause an
		// overlap
		for _, overlapNode := range currNode.OverlappingNodes() {
			if alreadyPlacedNodes.Contains(overlapNode) {
				currRect := currNode.Rectangle()
				overlapRect := overlapNode.Rectangle()

				// Check if the current y coordinate would cause an overlap
				// with the overlap node
				if yPos < overlapRect.Y+overlapRect.Height+verticalGap &&
					yPos+currRect.Height+verticalGap > overlapRect.Y {

					yPos = overlapRect.Y + overlapRect.Height + verticalGap
				}
			}
		}

		// Apply the y coordinate and remember that this node is now placed
		currNode.Rectangle().Y = yPos
		alreadyPlacedNodes.Add(currNode)

		// Update the strip size
		stripSize = max(stripSize, currNode.Rectangle().Y+currNode.Rectangle().Height)
	}

	return stripSize
}

// CompareByYCoordinate is compareByYCoordinate(node1, node2).
func CompareByYCoordinate(node1, node2 *RectangleNode) int {
	return gwt.DoubleCompare(node1.Rectangle().Y, node2.Rectangle().Y)
}
