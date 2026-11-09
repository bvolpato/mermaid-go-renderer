// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/terminalStub.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// collapseShortTerminalStub collapses a short terminal stub at the
// destination of an edge by retargeting the destination face and dropping
// the corner.
//
// Shape handled:
//
//	... → prev → penult → end
//
// where penult is perpendicular to the last segment and the last segment
// is shorter than MIN_STUB. The rewrite is ... → prev' → end': prev' keeps
// the "far" coordinate of prev and adopts the face center of the
// destination on the other axis, and end' is the face center of the
// destination on the approach axis.
//
// The rewrite is rejected when the new segment would enter a real node
// (other than the destination) or a label, cross a segment of another
// edge, or when prev' lies inside the source node.
func collapseShortTerminalStub(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	const MIN_STUB = 10
	const EPS_LOCAL = 1e-3
	const BUFFER = 2

	realNodeRects, labelRects := collectNodeRectEntries(nodeByIdMap.Values())

	for _, edge := range edges {
		if edge.IsLayoutOnly {
			continue
		}
		rawPts := edge.Points
		if len(rawPts) < 4 {
			continue
		}

		// Dedupe consecutive equal points so we measure the real last segment.
		pts := dedupeConsecutivePoints(rawPts, EPS_LOCAL)
		if len(pts) < 4 {
			continue
		}

		nLast := len(pts) - 1
		endPt := pts[nLast]
		penultPt := pts[nLast-1]
		prevPt := pts[nLast-2]

		// Last segment (penult → end): must be short.
		lastDx := endPt.X - penultPt.X
		lastDy := endPt.Y - penultPt.Y
		lastLen := js.Hypot(lastDx, lastDy)
		if lastLen >= MIN_STUB || lastLen < EPS_LOCAL {
			continue
		}

		// Penult segment (prev → penult): must be non-degenerate and
		// perpendicular to last.
		penultDx := penultPt.X - prevPt.X
		penultDy := penultPt.Y - prevPt.Y
		penultLen := js.Hypot(penultDx, penultDy)
		if penultLen < EPS_LOCAL {
			continue
		}

		lastIsHoriz := isHorizontalSegment(penultPt, endPt, EPS_LOCAL)
		lastIsVert := isVerticalSegment(penultPt, endPt, EPS_LOCAL)
		penultIsHoriz := isHorizontalSegment(prevPt, penultPt, EPS_LOCAL)
		penultIsVert := isVerticalSegment(prevPt, penultPt, EPS_LOCAL)
		if !((lastIsHoriz && penultIsVert) || (lastIsVert && penultIsHoriz)) {
			continue
		}

		dstId := edge.End
		srcId := edge.Start
		var dst *ru.Node
		if dstId != "" {
			dst = nodeByIdMap.Get(dstId)
		}
		if dst == nil {
			continue
		}
		dstCx := ru.NN(dst.X, 0)
		dstCy := ru.NN(dst.Y, 0)
		dstRect, ok := rectOfNodeBounds(dst)
		if !ok {
			continue
		}

		// Compute the new prev' and end'. The axis of approach is the axis of
		// the penult segment; the new face is on the perpendicular to the
		// approach, opposite to the approach direction.
		var newPrev, newEnd Point
		if penultIsVert {
			// Vertical approach: penult goes up (penultDy<0) or down.
			approachFromBelow := penultDy < 0
			newPrev = Point{X: dstCx, Y: prevPt.Y}
			if approachFromBelow {
				newEnd = Point{X: dstCx, Y: dstRect.Bottom}
			} else {
				newEnd = Point{X: dstCx, Y: dstRect.Top}
			}
		} else {
			// Horizontal approach: penult goes right (penultDx>0) or left.
			approachFromLeft := penultDx > 0
			newPrev = Point{X: prevPt.X, Y: dstCy}
			if approachFromLeft {
				newEnd = Point{X: dstRect.Right, Y: dstCy}
			} else {
				newEnd = Point{X: dstRect.Left, Y: dstCy}
			}
		}

		// Reject if the new prev'→end' segment would cross any real-node rect
		// (other than dst itself).
		if segmentHitsAnyRect(newPrev, newEnd, realNodeRects, []string{dstId}, -BUFFER) {
			continue
		}

		// Reject if the new approach segment would run through any label
		// rect.
		if segmentHitsAnyRect(newPrev, newEnd, labelRects, nil, -BUFFER) {
			continue
		}

		// Reject if new prev' lies inside the src node's rect (pathological).
		if srcId != "" {
			if src := nodeByIdMap.Get(srcId); src != nil {
				if srcRect, ok := rectOfNodeBounds(src); ok && pointInsideRect(newPrev, srcRect, BUFFER) {
					continue
				}
			}
		}

		// Also reject if the new prev'→end' segment crosses any other edge's
		// existing segment (excluding our own segments we are about to
		// replace).
		ownSegmentKey := func(a, b Point) string {
			return pointKey3(a) + "|" + pointKey3(b)
		}
		selfSegments := map[string]bool{}
		for i := 0; i < len(pts)-1; i++ {
			selfSegments[ownSegmentKey(pts[i], pts[i+1])] = true
		}

		segmentCrossesOtherEdge := func(from, to Point) bool {
			for _, other := range edges {
				if other == edge {
					continue
				}
				if other.IsLayoutOnly {
					continue
				}
				oPts := other.Points
				if len(oPts) < 2 {
					continue
				}
				for i := 0; i < len(oPts)-1; i++ {
					a := oPts[i]
					b := oPts[i+1]
					if selfSegments[ownSegmentKey(a, b)] {
						continue
					}
					if orthogonalSegmentsStrictlyCross(from, to, a, b, EPS_LOCAL) {
						return true
					}
				}
			}
			return false
		}

		if segmentCrossesOtherEdge(newPrev, newEnd) {
			continue
		}

		// The segment before the original prev (pts[nLast-3] → prev) becomes
		// (pts[nLast-3] → newPrev) when we shift prev's axis. Re-check the
		// extended segment for obstacles and crossings.
		if nLast-3 >= 0 {
			beforePrev := pts[nLast-3]
			endpointIds := endpointIdsOf(srcId, dstId)
			if segmentHitsAnyRect(beforePrev, newPrev, realNodeRects, endpointIds, -BUFFER) {
				continue
			}
			if segmentCrossesOtherEdge(beforePrev, newPrev) {
				continue
			}
		}

		// Build the rewritten polyline: pts[0..nLast-3] + newPrev + newEnd.
		// This drops the original prev, penult, and end (one fewer bend).
		head := pts[:nLast-2]
		newPts := cat(head, []Point{newPrev, newEnd})
		edge.Points = newPts

		// Re-anchor the edge's label (if any) onto the new polyline: at the
		// midpoint of the longest axis-aligned segment that can hold it.
		labelId := edge.LabelNodeID
		if labelId != "" {
			labelNode := nodeByIdMap.Get(labelId)
			if labelNode != nil {
				lw := ru.NN(labelNode.Width, 0)
				lh := ru.NN(labelNode.Height, 0)
				if lw > 0 && lh > 0 {
					var bestMidX, bestMidY float64
					found := false
					bestLen := -1.0
					for i := 0; i < len(newPts)-1; i++ {
						a := newPts[i]
						b := newPts[i+1]
						segLen := js.Hypot(b.X-a.X, b.Y-a.Y)
						isHoriz := sameY(a, b, EPS_LOCAL)
						isVert := sameX(a, b, EPS_LOCAL)
						// Require axis-aligned and long enough to hold the label.
						fits := (isHoriz && segLen >= lw+2) || (isVert && segLen >= lh+2)
						if !fits {
							continue
						}
						if segLen > bestLen {
							bestLen = segLen
							bestMidX = (a.X + b.X) / 2
							bestMidY = (a.Y + b.Y) / 2
							found = true
						}
					}
					if found {
						labelNode.X = bestMidX
						labelNode.Y = bestMidY
					}
				}
			}
		}
	}
}
