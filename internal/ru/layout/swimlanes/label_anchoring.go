// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/labelAnchoring.ts
// (MIT).

package swimlanes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

const labelEPS = 1e-3
const labelMarkerClearanceLength = 10
const labelMarkerClearanceHalfWidth = 7

// markerClearanceRectFor returns false for undefined.
func markerClearanceRectFor(pts []Point, atStart bool) (RectBounds, bool) {
	terminalIndex := len(pts) - 1
	step := -1
	if atStart {
		terminalIndex = 0
		step = 1
	}
	innerIndex := terminalIndex + step
	if terminalIndex < 0 || terminalIndex >= len(pts) || innerIndex < 0 || innerIndex >= len(pts) {
		return RectBounds{}, false
	}
	tip := pts[terminalIndex]
	inner := pts[innerIndex]

	dx := inner.X - tip.X
	dy := inner.Y - tip.Y
	length := math.Abs(dx) + math.Abs(dy)
	if length < labelEPS {
		return RectBounds{}, false
	}

	if math.Abs(dy) <= labelEPS {
		x2 := tip.X + float64(js.Sign(dx)*labelMarkerClearanceLength)
		return RectBounds{
			Left:   js.Min(tip.X, x2),
			Right:  js.Max(tip.X, x2),
			Top:    tip.Y - labelMarkerClearanceHalfWidth,
			Bottom: tip.Y + labelMarkerClearanceHalfWidth,
		}, true
	}

	if math.Abs(dx) <= labelEPS {
		y2 := tip.Y + float64(js.Sign(dy)*labelMarkerClearanceLength)
		return RectBounds{
			Left:   tip.X - labelMarkerClearanceHalfWidth,
			Right:  tip.X + labelMarkerClearanceHalfWidth,
			Top:    js.Min(tip.Y, y2),
			Bottom: js.Max(tip.Y, y2),
		}, true
	}

	return RectBounds{
		Left:   js.Min(tip.X, inner.X),
		Right:  js.Max(tip.X, inner.X),
		Top:    js.Min(tip.Y, inner.Y),
		Bottom: js.Max(tip.Y, inner.Y),
	}, true
}

func normalizeRect(rect RectBounds) RectBounds {
	return RectBounds{
		Left:   js.Min(rect.Left, rect.Right),
		Right:  js.Max(rect.Left, rect.Right),
		Top:    js.Min(rect.Top, rect.Bottom),
		Bottom: js.Max(rect.Top, rect.Bottom),
	}
}

func labelOverlapsOwnMarker(rect RectBounds, pts []Point) bool {
	visiblePts := dedupeConsecutivePoints(pts)
	if marker, ok := markerClearanceRectFor(visiblePts, true); ok && rectsOverlap(rect, normalizeRect(marker)) {
		return true
	}
	if marker, ok := markerClearanceRectFor(visiblePts, false); ok && rectsOverlap(rect, normalizeRect(marker)) {
		return true
	}
	return false
}

// anchorLabelsToPolyline moves the label node of every labelled edge onto
// a segment of the routed polyline of the edge, and into the lane that
// contains it.
func anchorLabelsToPolyline(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	// Build a set of foreign polylines once for overlap checks. Labelled
	// originals that have not been anchored yet are still included.
	type SegmentLite struct {
		edgeId string
		p1, p2 Point
	}
	allEdgeSegments := []SegmentLite{}
	for _, other := range edges {
		if other.IsLayoutOnly {
			continue
		}
		pts := other.Points
		if len(pts) < 2 {
			continue
		}
		for i := 0; i < len(pts)-1; i++ {
			allEdgeSegments = append(allEdgeSegments, SegmentLite{edgeId: other.ID, p1: pts[i], p2: pts[i+1]})
		}
	}

	type nodeRect struct {
		nodeId string
		rect   RectBounds
	}
	type laneRect struct {
		id   string
		rect RectBounds
	}
	foreignNodeRects := []nodeRect{}
	// Collect top-level lane groups so that the parentId of a label can be
	// re-assigned to whichever lane geometrically contains its anchored
	// position.
	laneGroups := []laneRect{}
	for _, n := range nodeByIdMap.Values() {
		isGroup := n.IsGroup
		parentId := n.ParentID
		if isGroup && parentId == "" {
			if rect, ok := rectOfNodeBounds(n); ok {
				laneGroups = append(laneGroups, laneRect{id: n.ID, rect: rect})
			}
			continue
		}
		if isGroup {
			continue
		}
		if n.IsEdgeLabel {
			continue
		}
		rect, ok := rectOfNodeBounds(n)
		if !ok {
			continue
		}
		foreignNodeRects = append(foreignNodeRects, nodeRect{nodeId: n.ID, rect: rect})
	}

	// Inflation margin for foreign-edge / foreign-node proximity: a little
	// more than the border-hugging tolerance of the layout validator.
	const LABEL_PLACEMENT_BUFFER = 3
	const LABEL_LANE_MARGIN = 1
	// The point marker of mermaid occupies roughly 10u at the edge endpoint;
	// keep labels a little farther away.
	const LABEL_ENDPOINT_CLEARANCE = 12

	labelOverlapsForeignNode := func(labelId string, rect RectBounds) bool {
		buffered := inflateRect(rect, LABEL_PLACEMENT_BUFFER)
		for _, f := range foreignNodeRects {
			if f.nodeId == labelId {
				continue
			}
			if rectsOverlap(buffered, f.rect) {
				return true
			}
		}
		return false
	}

	labelOverlapsForeignEdge := func(edgeId string, rect RectBounds) bool {
		buffered := inflateRect(rect, LABEL_PLACEMENT_BUFFER)
		for _, s := range allEdgeSegments {
			if s.edgeId == edgeId {
				continue
			}
			if segmentBoundsOverlapRect(s.p1, s.p2, buffered) {
				return true
			}
		}
		return false
	}

	labelOverlapsAnything := func(labelId, edgeId string, rect RectBounds) bool {
		return labelOverlapsForeignNode(labelId, rect) || labelOverlapsForeignEdge(edgeId, rect)
	}

	type placedLabel struct {
		labelId string
		rect    RectBounds
	}
	placedLabelRects := []placedLabel{}

	// findContainingLane returns "" for undefined.
	findContainingLane := func(rect RectBounds) string {
		for _, lane := range laneGroups {
			if rectContainsRect(lane.rect, rect) {
				return lane.id
			}
		}
		return ""
	}

	overlapsPlacedLabel := func(labelId string, rect RectBounds) bool {
		for _, placed := range placedLabelRects {
			if placed.labelId != labelId && rectsOverlap(rect, placed.rect) {
				return true
			}
		}
		return false
	}

	type SegmentCandidate struct {
		idx         int
		length      float64
		orientation string // "horizontal" or "vertical"
		midX        float64
		midY        float64
	}
	type anchorPoint struct {
		midX, midY float64
	}
	type placement struct {
		laneId string
		anchor anchorPoint
		rect   RectBounds
	}
	type choice struct {
		laneId string
		anchor anchorPoint
	}

	for _, edge := range edges {
		if edge.IsLayoutOnly {
			continue
		}
		labelId := edge.LabelNodeID
		if labelId == "" {
			continue
		}
		labelNode := nodeByIdMap.Get(labelId)
		if labelNode == nil {
			continue
		}
		pts := edge.Points
		if len(pts) < 2 {
			continue
		}
		lw := ru.NN(labelNode.Width, 0)
		lh := ru.NN(labelNode.Height, 0)
		if lw <= 0 || lh <= 0 {
			continue
		}

		// Collect every non-zero segment with orientation.
		segments := []*SegmentCandidate{}
		for i := 0; i < len(pts)-1; i++ {
			a := pts[i]
			b := pts[i+1]
			dx := math.Abs(a.X - b.X)
			dy := math.Abs(a.Y - b.Y)
			if dx < labelEPS && dy < labelEPS {
				continue
			}
			if dx >= labelEPS && dy >= labelEPS {
				continue // non-orthogonal: should not happen after orthogonalize
			}
			orientation := "vertical"
			if dx >= labelEPS {
				orientation = "horizontal"
			}
			segments = append(segments, &SegmentCandidate{
				idx:         i,
				length:      dx + dy,
				orientation: orientation,
				midX:        (a.X + b.X) / 2,
				midY:        (a.Y + b.Y) / 2,
			})
		}

		if len(segments) == 0 {
			continue
		}

		// Middle segments only (exclude first and last). Fall back to any
		// segment if the polyline has fewer than 3 segments.
		middleSegments := segments
		if len(segments) >= 3 {
			middleSegments = nil
			for _, s := range segments {
				if s.idx > 0 && s.idx < len(segments)-1 {
					middleSegments = append(middleSegments, s)
				}
			}
		}
		poolBase := segments
		if len(middleSegments) > 0 {
			poolBase = middleSegments
		}

		// Label long axis: horizontal if wider than tall, else vertical. The
		// long axis only drives preference, not hard filtering.
		labelLongAxis := "vertical"
		if lw >= lh {
			labelLongAxis = "horizontal"
		}

		// Candidate ranking: orientation matching the label long axis first,
		// then length >= labelExtent + 2, then the longest.
		rankSegments := func(pool []*SegmentCandidate) []*SegmentCandidate {
			ranked := append([]*SegmentCandidate{}, pool...)
			fits := func(s *SegmentCandidate) bool {
				extent := lh
				if s.orientation == "horizontal" {
					extent = lw
				}
				return s.length >= extent+2
			}
			sortFunc(ranked, func(a, b *SegmentCandidate) float64 {
				aLongAxis := a.orientation == labelLongAxis
				bLongAxis := b.orientation == labelLongAxis
				if aLongAxis != bLongAxis {
					if aLongAxis {
						return -1
					}
					return 1
				}
				aFits := fits(a)
				bFits := fits(b)
				if aFits != bFits {
					if aFits {
						return -1
					}
					return 1
				}
				return b.length - a.length
			})
			return ranked
		}

		// Try the middle-segment pool first, then expand to include every
		// orthogonal segment if the middle-only pool yields no
		// lane-containing, overlap-free candidate.
		//
		// Per segment, if the midpoint (t=0.5) collides with a foreign edge
		// or label, walk along the segment at additional parametric positions
		// before moving on.
		firstVisibleSegment := segments[0]
		lastVisibleSegment := segments[len(segments)-1]
		ALONG_SEGMENT_TS := []float64{0.5, 0.25, 0.75, 0.05, 0.95, 0.15, 0.85, 0.1, 0.9}
		anchorAtT := func(seg *SegmentCandidate, t float64) anchorPoint {
			a := pts[seg.idx]
			b := pts[seg.idx+1]
			return anchorPoint{
				midX: a.X + float64((b.X-a.X)*t),
				midY: a.Y + float64((b.Y-a.Y)*t),
			}
		}
		clamp := func(value, min, max float64) float64 {
			return js.Min(max, js.Max(min, value))
		}
		pointInsideRectInclusive := func(point anchorPoint, rect RectBounds) bool {
			return point.midX >= rect.Left-labelEPS &&
				point.midX <= rect.Right+labelEPS &&
				point.midY >= rect.Top-labelEPS &&
				point.midY <= rect.Bottom+labelEPS
		}
		// placementForAnchor returns nil for undefined.
		placementForAnchor := func(anchor anchorPoint) *placement {
			centeredRect := rectFromCenterSize(anchor.midX, anchor.midY, lw, lh)
			centeredLane := findContainingLane(centeredRect)
			if centeredLane != "" {
				return &placement{laneId: centeredLane, anchor: anchor, rect: centeredRect}
			}

			// If the segment is close to a lane border, keep the label box
			// inside the lane while requiring the original segment point to
			// remain inside that box.
			var containingLane *laneRect
			for i := range laneGroups {
				if pointInsideRectInclusive(anchor, laneGroups[i].rect) {
					containingLane = &laneGroups[i]
					break
				}
			}
			if containingLane == nil {
				return nil
			}

			minX := containingLane.rect.Left + float64(lw/2) + LABEL_LANE_MARGIN
			maxX := containingLane.rect.Right - float64(lw/2) - LABEL_LANE_MARGIN
			minY := containingLane.rect.Top + float64(lh/2) + LABEL_LANE_MARGIN
			maxY := containingLane.rect.Bottom - float64(lh/2) - LABEL_LANE_MARGIN
			if minX > maxX || minY > maxY {
				return nil
			}

			clampedAnchor := anchorPoint{
				midX: clamp(anchor.midX, minX, maxX),
				midY: clamp(anchor.midY, minY, maxY),
			}
			clampedRect := rectFromCenterSize(clampedAnchor.midX, clampedAnchor.midY, lw, lh)
			if pointInsideRectInclusive(anchor, clampedRect) {
				return &placement{laneId: containingLane.id, anchor: clampedAnchor, rect: clampedRect}
			}
			return nil
		}
		distanceAlongSegment := func(seg *SegmentCandidate, anchor anchorPoint, endpoint Point) float64 {
			if seg.orientation == "horizontal" {
				return math.Abs(anchor.midX - endpoint.X)
			}
			return math.Abs(anchor.midY - endpoint.Y)
		}
		labelClearsTerminalEndpoints := func(seg *SegmentCandidate, anchor anchorPoint) bool {
			labelHalfExtent := lh / 2
			if seg.orientation == "horizontal" {
				labelHalfExtent = lw / 2
			}
			requiredDistance := float64(labelHalfExtent) + LABEL_ENDPOINT_CLEARANCE
			if seg == firstVisibleSegment {
				start := pts[seg.idx]
				if distanceAlongSegment(seg, anchor, start)+labelEPS < requiredDistance {
					return false
				}
			}
			if seg == lastVisibleSegment {
				end := pts[seg.idx+1]
				if distanceAlongSegment(seg, anchor, end)+labelEPS < requiredDistance {
					return false
				}
			}
			return true
		}
		// tryPool and findLaneContainingFallback return nil for undefined.
		tryPool := func(pool []*SegmentCandidate) *choice {
			rankedPool := rankSegments(pool)
			for _, seg := range rankedPool {
				for _, t := range ALONG_SEGMENT_TS {
					anchor := anchorAtT(seg, t)
					if !labelClearsTerminalEndpoints(seg, anchor) {
						continue
					}
					placement := placementForAnchor(anchor)
					if placement == nil {
						continue
					}
					if labelOverlapsOwnMarker(placement.rect, pts) {
						continue
					}
					if overlapsPlacedLabel(labelId, placement.rect) {
						continue
					}
					if !labelOverlapsAnything(labelId, edge.ID, placement.rect) {
						return &choice{laneId: placement.laneId, anchor: placement.anchor}
					}
				}
			}
			return nil
		}

		findLaneContainingFallback := func(pool []*SegmentCandidate, requireEndpointClearance bool, allowForeignEdgeOverlap bool) *choice {
			rankedPool := rankSegments(pool)
			for _, seg := range rankedPool {
				anchor := anchorPoint{midX: seg.midX, midY: seg.midY}
				if requireEndpointClearance && !labelClearsTerminalEndpoints(seg, anchor) {
					continue
				}
				placement := placementForAnchor(anchor)
				if placement != nil &&
					!labelOverlapsOwnMarker(placement.rect, pts) &&
					!overlapsPlacedLabel(labelId, placement.rect) &&
					!labelOverlapsForeignNode(labelId, placement.rect) &&
					(allowForeignEdgeOverlap || !labelOverlapsForeignEdge(edge.ID, placement.rect)) {
					return &choice{laneId: placement.laneId, anchor: placement.anchor}
				}
			}
			return nil
		}

		chosen := tryPool(poolBase)
		if chosen == nil && len(poolBase) < len(segments) {
			chosen = tryPool(segments)
		}
		if chosen == nil {
			chosen = findLaneContainingFallback(segments, true, false)
		}
		if chosen == nil {
			chosen = findLaneContainingFallback(segments, false, false)
		}
		if chosen == nil {
			chosen = findLaneContainingFallback(segments, false, true)
		}

		if chosen != nil {
			labelNode.X = chosen.anchor.midX
			labelNode.Y = chosen.anchor.midY
			labelNode.ParentID = chosen.laneId
			chosenRect := rectFromCenterSize(chosen.anchor.midX, chosen.anchor.midY, lw, lh)
			priorIdx := -1
			for i, placed := range placedLabelRects {
				if placed.labelId == labelId {
					priorIdx = i
					break
				}
			}
			if priorIdx >= 0 {
				placedLabelRects[priorIdx] = placedLabel{labelId: labelId, rect: chosenRect}
			} else {
				placedLabelRects = append(placedLabelRects, placedLabel{labelId: labelId, rect: chosenRect})
			}
		}
	}
}
