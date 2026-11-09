// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/sharedTrackNudging.ts
// (MIT).

package swimlanes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// nudgeSharedInteriorSubpaths separates interior segments of different
// edges that run on the same track or too close to each other. The ends of
// the edges stay where they are.
func nudgeSharedInteriorSubpaths(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	const EPS_LOCAL = 1e-3
	const MIN_SHARED = 8
	const TRACK_SHIFT = 7
	const MIN_TRACK_GAP = TRACK_SHIFT
	const SOURCE_DETOUR_STUB = 20
	const BUFFER = 2
	const MAX_ITERATIONS = 12

	type SegmentLite struct {
		*OrthogonalSegment
		edge     *ru.Edge
		interior bool
	}

	realNodeRects, labelRects := collectNodeRectEntries(nodeByIdMap.Values())

	segmentsFor := func(edge *ru.Edge, points []Point) []*SegmentLite {
		segments := orthogonalSegmentsForPoints(points, EPS_LOCAL)
		out := make([]*SegmentLite, len(segments))
		for i, segment := range segments {
			out[i] = &SegmentLite{
				OrthogonalSegment: segment,
				edge:              edge,
				interior:          segment.Index >= 1 && segment.Index <= len(points)-3,
			}
		}
		return out
	}

	allSegments := func() []*SegmentLite {
		result := []*SegmentLite{}
		for _, edge := range edges {
			if edge.IsLayoutOnly {
				continue
			}
			points := edge.Points
			if len(points) < 2 {
				continue
			}
			result = append(result, segmentsFor(edge, dedupeConsecutivePoints(points))...)
		}
		return result
	}

	hasCrowdedParallelTrack := func(a, b *SegmentLite) bool {
		if a.Horizontal && b.Horizontal {
			return overlapLength(a.A.X, a.B.X, b.A.X, b.B.X) >= MIN_SHARED &&
				math.Abs(a.A.Y-b.A.Y) < MIN_TRACK_GAP
		}
		if a.Vertical && b.Vertical {
			return overlapLength(a.A.Y, a.B.Y, b.A.Y, b.B.Y) >= MIN_SHARED &&
				math.Abs(a.A.X-b.A.X) < MIN_TRACK_GAP
		}
		return false
	}

	candidateIsSafe := func(edge *ru.Edge, candidate []Point) bool {
		sourceId := edge.Start
		targetId := edge.End
		candidateSegments := segmentsFor(edge, candidate)
		if len(candidateSegments) != len(candidate)-1 {
			return false
		}

		endpointIds := endpointIdsOf(sourceId, targetId)
		var ownLabelIds []string
		if edge.LabelNodeID != "" {
			ownLabelIds = []string{edge.LabelNodeID}
		}
		for _, segment := range candidateSegments {
			if segmentHitsAnyRect(segment.A, segment.B, realNodeRects, endpointIds, -BUFFER) {
				return false
			}
			if segmentHitsAnyRect(segment.A, segment.B, labelRects, ownLabelIds, -BUFFER) {
				return false
			}
		}

		for _, other := range edges {
			if other == edge || other.IsLayoutOnly {
				continue
			}
			otherPoints := other.Points
			if len(otherPoints) < 2 {
				continue
			}
			for _, candidateSegment := range candidateSegments {
				for _, otherSegment := range segmentsFor(other, dedupeConsecutivePoints(otherPoints)) {
					if hasCrowdedParallelTrack(candidateSegment, otherSegment) {
						return false
					}
					if orthogonalSegmentsStrictlyCross(
						candidateSegment.A,
						candidateSegment.B,
						otherSegment.A,
						otherSegment.B,
						EPS_LOCAL,
					) {
						return false
					}
				}
			}
		}

		return true
	}

	// shiftedCandidate returns nil for undefined.
	shiftedCandidate := func(segment *SegmentLite, shift float64) []Point {
		points := dedupeConsecutivePoints(segment.edge.Points)
		if len(points) < 4 || segment.Index >= len(points)-1 {
			return nil
		}
		candidate := clonePoints(points)
		if segment.Horizontal {
			candidate[segment.Index].Y += shift
			candidate[segment.Index+1].Y += shift
		} else if segment.Vertical {
			candidate[segment.Index].X += shift
			candidate[segment.Index+1].X += shift
		} else {
			return nil
		}
		if len(segmentsFor(segment.edge, candidate)) == len(candidate)-1 {
			return candidate
		}
		return nil
	}

	type SourceDetourContext struct {
		sourceCenter Point
		targetCenter Point
		sourceRect   RectBounds
		tail         []Point
	}

	nodeCenter := func(node *ru.Node, rect RectBounds) Point {
		return Point{
			X: ru.NN(node.X, (rect.Left+rect.Right)/2),
			Y: ru.NN(node.Y, (rect.Top+rect.Bottom)/2),
		}
	}

	// sourceDetourContextFor returns nil for undefined.
	sourceDetourContextFor := func(segment *SegmentLite) *SourceDetourContext {
		edge := segment.edge
		points := dedupeConsecutivePoints(edge.Points)
		if len(points) != 4 || segment.Index != 1 {
			return nil
		}

		var sourceNode, targetNode *ru.Node
		if edge.Start != "" {
			sourceNode = nodeByIdMap.Get(edge.Start)
		}
		if edge.End != "" {
			targetNode = nodeByIdMap.Get(edge.End)
		}
		if sourceNode == nil || targetNode == nil {
			return nil
		}
		sourceRect, hasSourceRect := rectOfNodeBounds(sourceNode)
		targetRect, hasTargetRect := rectOfNodeBounds(targetNode)
		tail := points[segment.Index+2:]
		if !hasSourceRect || !hasTargetRect || len(tail) == 0 {
			return nil
		}

		return &SourceDetourContext{
			sourceCenter: nodeCenter(sourceNode, sourceRect),
			targetCenter: nodeCenter(targetNode, targetRect),
			sourceRect:   sourceRect,
			tail:         tail,
		}
	}

	// verticalSourceDetour and horizontalSourceDetour return nil for
	// undefined.
	verticalSourceDetour := func(segment *SegmentLite, shift float64, sourceCenter, targetCenter Point, sourceRect RectBounds, tail []Point) []Point {
		targetBelow := targetCenter.Y >= sourceCenter.Y
		sourcePortY := sourceRect.Top
		stubY := sourcePortY + -SOURCE_DETOUR_STUB
		if targetBelow {
			sourcePortY = sourceRect.Bottom
			stubY = sourcePortY + SOURCE_DETOUR_STUB
		}
		if (targetBelow && segment.B.Y <= stubY+EPS_LOCAL) ||
			(!targetBelow && segment.B.Y >= stubY-EPS_LOCAL) {
			return nil
		}

		railX := segment.A.X + shift
		return dedupeConsecutivePoints(
			cat([]Point{
				{X: sourceCenter.X, Y: sourcePortY},
				{X: sourceCenter.X, Y: stubY},
				{X: railX, Y: stubY},
				{X: railX, Y: segment.B.Y},
			}, tail),
			EPS_LOCAL,
		)
	}

	horizontalSourceDetour := func(segment *SegmentLite, shift float64, sourceCenter, targetCenter Point, sourceRect RectBounds, tail []Point) []Point {
		targetRight := targetCenter.X >= sourceCenter.X
		sourcePortX := sourceRect.Left
		stubX := sourcePortX + -SOURCE_DETOUR_STUB
		if targetRight {
			sourcePortX = sourceRect.Right
			stubX = sourcePortX + SOURCE_DETOUR_STUB
		}
		if (targetRight && segment.B.X <= stubX+EPS_LOCAL) ||
			(!targetRight && segment.B.X >= stubX-EPS_LOCAL) {
			return nil
		}

		railY := segment.A.Y + shift
		return dedupeConsecutivePoints(
			cat([]Point{
				{X: sourcePortX, Y: sourceCenter.Y},
				{X: stubX, Y: sourceCenter.Y},
				{X: stubX, Y: railY},
				{X: segment.B.X, Y: railY},
			}, tail),
			EPS_LOCAL,
		)
	}

	sourceDetourCandidate := func(segment *SegmentLite, shift float64) []Point {
		context := sourceDetourContextFor(segment)
		if context == nil {
			return nil
		}

		if segment.Vertical {
			return verticalSourceDetour(
				segment,
				shift,
				context.sourceCenter,
				context.targetCenter,
				context.sourceRect,
				context.tail,
			)
		}
		if segment.Horizontal {
			return horizontalSourceDetour(
				segment,
				shift,
				context.sourceCenter,
				context.targetCenter,
				context.sourceRect,
				context.tail,
			)
		}

		return nil
	}

	shifts := []float64{
		-TRACK_SHIFT,
		TRACK_SHIFT,
		-2 * TRACK_SHIFT,
		2 * TRACK_SHIFT,
		-3 * TRACK_SHIFT,
		3 * TRACK_SHIFT,
	}

	for iteration := 0; iteration < MAX_ITERATIONS; iteration++ {
		segments := allSegments()
		fixed := false

		for i := 0; i < len(segments) && !fixed; i++ {
			for j := i + 1; j < len(segments) && !fixed; j++ {
				first := segments[i]
				second := segments[j]
				if first.edge == second.edge || !hasCrowdedParallelTrack(first, second) {
					continue
				}

				var candidates []*SegmentLite
				for _, segment := range []*SegmentLite{first, second} {
					if segment.interior {
						candidates = append(candidates, segment)
					}
				}
				for _, segment := range candidates {
					for _, shift := range shifts {
						direct := shiftedCandidate(segment, shift)
						if direct != nil && candidateIsSafe(segment.edge, direct) {
							segment.edge.Points = direct
							fixed = true
							break
						}

						detoured := sourceDetourCandidate(segment, shift)
						if detoured != nil && candidateIsSafe(segment.edge, detoured) {
							segment.edge.Points = detoured
							fixed = true
							break
						}
					}
					if fixed {
						break
					}
				}
			}
		}

		if !fixed {
			return
		}
	}
}
