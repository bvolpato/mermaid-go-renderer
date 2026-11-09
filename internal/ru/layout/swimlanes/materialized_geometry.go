// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/materializedGeometry.ts
// (MIT).

package swimlanes

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// mgEpsLocal is EPS_LOCAL of materializedGeometry.ts.
const mgEpsLocal = 1e-3

// mgMinShared is MIN_SHARED of materializedGeometry.ts.
const mgMinShared = 8

// mgEdgeReplacementMap is EdgeReplacementMap: the points that stand for
// the points of an edge, by edge, in insertion order.
type mgEdgeReplacementMap = js.Map[*ru.Edge, []Point]

// mgUpperCaseDirection is upperCaseDirection(raw). An empty string stands
// for a value that is not a string.
func mgUpperCaseDirection(raw string) string {
	return strings.ToUpper(raw)
}

// mgDirectionOf is `node.direction ?? layoutDirection`.
func mgDirectionOf(node *ru.Node, layoutDirection string) string {
	if direction, ok := nodeDirection(node); ok {
		return direction
	}
	return layoutDirection
}

// mgSegmentsFor is segmentsFor: orthogonalSegmentsForPoints with the
// default epsilon.
func mgSegmentsFor(points []Point) []*OrthogonalSegment {
	return orthogonalSegmentsForPoints(points)
}

func mgOrthogonallyAligned(a, b Point) bool {
	return sameX(a, b, mgEpsLocal) || sameY(a, b, mgEpsLocal)
}

// mgVisibleEdges is edges.filter((edge) => !edge.isLayoutOnly).
func mgVisibleEdges(edges []*ru.Edge) []*ru.Edge {
	visibleEdges := []*ru.Edge{}
	for _, edge := range edges {
		if !edge.IsLayoutOnly {
			visibleEdges = append(visibleEdges, edge)
		}
	}
	return visibleEdges
}

// mgNodeOf is `id ? nodeByIdMap.get(id) : undefined`.
func mgNodeOf(nodeByIdMap *js.Map[string, *ru.Node], id string) *ru.Node {
	if id == "" {
		return nil
	}
	return nodeByIdMap.Get(id)
}

// mgStrictCrossingsBetween counts the pairs of segments that cross
// strictly.
func mgStrictCrossingsBetween(firstSegments, secondSegments []*OrthogonalSegment) int {
	count := 0
	for _, firstSegment := range firstSegments {
		for _, secondSegment := range secondSegments {
			// The segments come from mgSegmentsFor, so each is horizontal or
			// vertical for the epsilon of the test below, and two segments
			// of the same orientation never cross.
			if firstSegment.Horizontal == secondSegment.Horizontal {
				continue
			}
			if orthogonalSegmentsStrictlyCross(
				firstSegment.A,
				firstSegment.B,
				secondSegment.A,
				secondSegment.B,
				mgEpsLocal,
			) {
				count++
			}
		}
	}
	return count
}

func separateSharedRenderedTerminalLanes(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	const MIN_FACE_CLEARANCE = 16
	const TRACK_SHIFT = 7

	type TerminalLane struct {
		edge    *ru.Edge
		edgeId  string
		nodeId  string
		atStart bool
		// orientation is "H" or "V".
		orientation string
		coord       float64
		min         float64
		max         float64
		boundary    Point
		railEnd     Point
		rect        RectBounds
	}

	rectIntersect := func(node *ru.Node, point Point) Point {
		x := ru.NN(node.X, 0)
		y := ru.NN(node.Y, 0)
		dx := point.X - x
		dy := point.Y - y
		w := float64(ru.NN(node.Width, 0) / 2)
		h := float64(ru.NN(node.Height, 0) / 2)

		if math.Abs(dy)*w > math.Abs(dx)*h {
			if dy < 0 {
				h = -h
			}
			offset := 0.0
			if dy != 0 {
				offset = float64(h*dx) / dy
			}
			return Point{X: x + offset, Y: y + h}
		}

		if dx < 0 {
			w = -w
		}
		offset := 0.0
		if dx != 0 {
			offset = float64(w*dy) / dx
		}
		return Point{X: x + w, Y: y + offset}
	}

	terminalLaneFor := func(edge *ru.Edge, atStart bool) *TerminalLane {
		points := dedupeConsecutivePoints(edge.Points)
		if len(points) < 2 {
			return nil
		}

		nodeId := edge.End
		if atStart {
			nodeId = edge.Start
		}
		node := mgNodeOf(nodeByIdMap, nodeId)
		if node == nil || nodeId == "" {
			return nil
		}
		rect, ok := rectOfNodeBounds(node)
		if !ok {
			return nil
		}

		endpoint := points[len(points)-1]
		adjacent := points[len(points)-2]
		if atStart {
			endpoint = points[0]
			adjacent = points[1]
		}
		boundary := rectIntersect(node, endpoint)
		railEnd := endpoint
		if mgOrthogonallyAligned(adjacent, boundary) {
			railEnd = adjacent
		}

		if sameX(boundary, railEnd, mgEpsLocal) {
			return &TerminalLane{
				edge:        edge,
				edgeId:      edge.ID,
				nodeId:      nodeId,
				atStart:     atStart,
				orientation: "V",
				coord:       boundary.X,
				min:         js.Min(boundary.Y, railEnd.Y),
				max:         js.Max(boundary.Y, railEnd.Y),
				boundary:    boundary,
				railEnd:     railEnd,
				rect:        rect,
			}
		}
		if sameY(boundary, railEnd, mgEpsLocal) {
			return &TerminalLane{
				edge:        edge,
				edgeId:      edge.ID,
				nodeId:      nodeId,
				atStart:     atStart,
				orientation: "H",
				coord:       boundary.Y,
				min:         js.Min(boundary.X, railEnd.X),
				max:         js.Max(boundary.X, railEnd.X),
				boundary:    boundary,
				railEnd:     railEnd,
				rect:        rect,
			}
		}
		return nil
	}

	projectedOverlapLength := func(a, b *TerminalLane) float64 {
		return js.Max(0, js.Min(a.max, b.max)-js.Max(a.min, b.min))
	}

	sameTerminalFace := func(a, b *TerminalLane) bool {
		if a.nodeId != b.nodeId || a.orientation != b.orientation {
			return false
		}

		if a.orientation == "H" {
			aOnHorizontalFace := math.Abs(a.boundary.X-a.rect.Left) < 1 || math.Abs(a.boundary.X-a.rect.Right) < 1
			return aOnHorizontalFace && sameX(a.boundary, b.boundary, 1)
		}

		aOnVerticalFace := math.Abs(a.boundary.Y-a.rect.Top) < 1 || math.Abs(a.boundary.Y-a.rect.Bottom) < 1
		return aOnVerticalFace && sameY(a.boundary, b.boundary, 1)
	}

	exactTerminalLaneConflict := func(a, b *TerminalLane) bool {
		if a.nodeId != b.nodeId || a.orientation != b.orientation {
			return false
		}

		shared := projectedOverlapLength(a, b)
		return shared >= mgMinShared && math.Abs(a.coord-b.coord) < 0.5
	}

	nearTerminalLaneConflict := func(a, b *TerminalLane) bool {
		if a.nodeId != b.nodeId ||
			a.orientation != b.orientation ||
			a.orientation != "H" ||
			a.atStart == b.atStart {
			return false
		}

		shared := projectedOverlapLength(a, b)
		if shared < mgMinShared {
			return false
		}
		faceSpan := a.rect.Bottom - a.rect.Top
		if shared < faceSpan || shared > 2*faceSpan {
			return false
		}

		// Rendered terminal tracks on the same object face must be nudged
		// apart before the endpoint duplication pins them in place.
		return sameTerminalFace(a, b) && math.Abs(a.coord-b.coord) < MIN_FACE_CLEARANCE
	}

	// shiftedCandidate returns nil for undefined. A candidate has two points
	// or more.
	shiftedCandidate := func(lane *TerminalLane, shift float64) []Point {
		points := dedupeConsecutivePoints(lane.edge.Points)
		if len(points) < 2 {
			return nil
		}

		var shiftedBoundary Point
		if lane.orientation == "V" {
			shiftedBoundary = Point{X: lane.boundary.X + shift, Y: lane.boundary.Y}
		} else {
			shiftedBoundary = Point{X: lane.boundary.X, Y: lane.boundary.Y + shift}
		}
		var shiftedRailEnd Point
		if lane.orientation == "V" {
			shiftedRailEnd = Point{X: lane.railEnd.X + shift, Y: lane.railEnd.Y}
		} else {
			shiftedRailEnd = Point{X: lane.railEnd.X, Y: lane.railEnd.Y + shift}
		}

		boundaryStaysOnSameFace := func() bool {
			if math.Abs(lane.boundary.Y-lane.rect.Top) < 1 ||
				math.Abs(lane.boundary.Y-lane.rect.Bottom) < 1 {
				return sameY(shiftedBoundary, lane.boundary, mgEpsLocal) &&
					shiftedBoundary.X >= lane.rect.Left+1 &&
					shiftedBoundary.X <= lane.rect.Right-1
			}

			if math.Abs(lane.boundary.X-lane.rect.Left) < 1 ||
				math.Abs(lane.boundary.X-lane.rect.Right) < 1 {
				return sameX(shiftedBoundary, lane.boundary, mgEpsLocal) &&
					shiftedBoundary.Y >= lane.rect.Top+1 &&
					shiftedBoundary.Y <= lane.rect.Bottom-1
			}

			return false
		}

		if !boundaryStaysOnSameFace() {
			return nil
		}

		if lane.atStart {
			railEndIsAdjacent := len(points) > 1 && samePoint(points[1], lane.railEnd, mgEpsLocal)
			rest := points[1:]
			if railEndIsAdjacent {
				rest = points[2:]
			}
			if len(rest) > 0 && !mgOrthogonallyAligned(rest[0], shiftedRailEnd) {
				return nil
			}
			return cat([]Point{shiftedBoundary, shiftedRailEnd}, rest)
		}

		railEndIsAdjacent := len(points) > 1 && samePoint(points[len(points)-2], lane.railEnd, mgEpsLocal)
		before := points[:len(points)-1]
		if railEndIsAdjacent {
			before = points[:len(points)-2]
		}
		if len(before) > 0 && !mgOrthogonallyAligned(before[len(before)-1], shiftedRailEnd) {
			return nil
		}
		return cat(before, []Point{shiftedRailEnd, shiftedBoundary})
	}

	laneIsStraightCollinearConnector := func(lane *TerminalLane) bool {
		edge := lane.edge
		points := dedupeConsecutivePoints(edge.Points)
		if len(points) != 2 {
			return false
		}
		start := mgNodeOf(nodeByIdMap, edge.Start)
		end := mgNodeOf(nodeByIdMap, edge.End)
		if start == nil || end == nil {
			return false
		}

		startX := ru.NN(start.X, 0)
		startY := ru.NN(start.Y, 0)
		endX := ru.NN(end.X, 0)
		endY := ru.NN(end.Y, 0)
		a, b := points[0], points[1]

		return (sameY(a, b, mgEpsLocal) && math.Abs(startY-endY) < 1 && math.Abs(startX-endX) > 1) ||
			(sameX(a, b, mgEpsLocal) && math.Abs(startX-endX) < 1 && math.Abs(startY-endY) > 1)
	}

	shifts := []float64{
		-TRACK_SHIFT,
		TRACK_SHIFT,
		-2 * TRACK_SHIFT,
		2 * TRACK_SHIFT,
		-3 * TRACK_SHIFT,
		3 * TRACK_SHIFT,
	}

	for iteration := 0; iteration < 8; iteration++ {
		lanes := []*TerminalLane{}
		for _, edge := range edges {
			if edge.IsLayoutOnly {
				continue
			}
			for _, lane := range []*TerminalLane{terminalLaneFor(edge, true), terminalLaneFor(edge, false)} {
				if lane != nil {
					lanes = append(lanes, lane)
				}
			}
		}

		fixed := false
		for i := 0; i < len(lanes) && !fixed; i++ {
			for j := i + 1; j < len(lanes) && !fixed; j++ {
				first := lanes[i]
				second := lanes[j]
				if first.edge == second.edge ||
					!(exactTerminalLaneConflict(first, second) || nearTerminalLaneConflict(first, second)) {
					continue
				}

				fixingNearConflict := !exactTerminalLaneConflict(first, second)
				candidates := []*TerminalLane{first, second}
				sortFunc(candidates, func(a, b *TerminalLane) float64 {
					aPreservesStraight := laneIsStraightCollinearConnector(a)
					bPreservesStraight := laneIsStraightCollinearConnector(b)
					if aPreservesStraight != bPreservesStraight {
						return boolNum(aPreservesStraight) - boolNum(bPreservesStraight)
					}
					return boolNum(!b.atStart) - boolNum(!a.atStart)
				})
				for _, lane := range candidates {
					for _, shift := range shifts {
						candidate := shiftedCandidate(lane, shift)
						if candidate == nil {
							continue
						}
						// { ...lane.edge, points: candidate }: a copy of the edge
						// with other points.
						shiftedEdge := *lane.edge
						shiftedEdge.Points = candidate
						nextLane := terminalLaneFor(&shiftedEdge, lane.atStart)
						if nextLane == nil {
							continue
						}
						conflicts := false
						for _, other := range lanes {
							if other.edge != lane.edge &&
								(exactTerminalLaneConflict(nextLane, other) ||
									(fixingNearConflict && nearTerminalLaneConflict(nextLane, other))) {
								conflicts = true
								break
							}
						}
						if conflicts {
							continue
						}

						lane.edge.Points = candidate
						fixed = true
						break
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

func collapseRedundantRectangularDoglegs(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	const BUFFER = 2
	const MAX_ITERATIONS = 8

	realNodeRects, labelRects := collectNodeRectEntries(nodeByIdMap.Values())

	candidateIsSafe := func(edge *ru.Edge, candidate []Point) bool {
		sourceId := edge.Start
		targetId := edge.End
		candidateSegments := mgSegmentsFor(candidate)
		if len(candidateSegments) != len(candidate)-1 {
			return false
		}

		endpointIds := endpointIdsOf(sourceId, targetId)
		for _, segment := range candidateSegments {
			if segmentHitsAnyRect(segment.A, segment.B, realNodeRects, endpointIds, -BUFFER) {
				return false
			}
			if segmentHitsAnyRect(segment.A, segment.B, labelRects, nil, -BUFFER) {
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
				for _, otherSegment := range mgSegmentsFor(dedupeConsecutivePoints(otherPoints)) {
					if sameAxisSegmentOverlapLength(candidateSegment, otherSegment, 0.5) >= mgMinShared {
						return false
					}
					if orthogonalSegmentsStrictlyCross(
						candidateSegment.A,
						candidateSegment.B,
						otherSegment.A,
						otherSegment.B,
						mgEpsLocal,
					) {
						return false
					}
				}
			}
		}

		return true
	}

	// withoutDogleg returns nil for undefined. A result has one point or
	// more.
	withoutDogleg := func(points []Point, i int) []Point {
		if i+4 >= len(points) {
			return nil
		}
		p0 := points[i]
		p1 := points[i+1]
		p2 := points[i+2]
		p3 := points[i+3]
		p4 := points[i+4]

		terminalVerticalDogleg := isHorizontalSegment(p0, p1) &&
			isVerticalSegment(p1, p2) &&
			isHorizontalSegment(p2, p3) &&
			isVerticalSegment(p3, p4) &&
			sameX(p0, p3, mgEpsLocal) &&
			sameX(p0, p4, mgEpsLocal) &&
			sameX(p1, p2, mgEpsLocal) &&
			(p1.X-p0.X)*(p3.X-p2.X) < 0

		terminalHorizontalDogleg := isVerticalSegment(p0, p1) &&
			isHorizontalSegment(p1, p2) &&
			isVerticalSegment(p2, p3) &&
			isHorizontalSegment(p3, p4) &&
			sameY(p0, p3, mgEpsLocal) &&
			sameY(p0, p4, mgEpsLocal) &&
			sameY(p1, p2, mgEpsLocal) &&
			(p1.Y-p0.Y)*(p3.Y-p2.Y) < 0

		if terminalVerticalDogleg || terminalHorizontalDogleg {
			return dedupeConsecutivePoints(cat(points[:i+1], []Point{p4}, points[i+5:]))
		}

		if i+5 >= len(points) {
			return nil
		}
		p5 := points[i+5]

		verticalDogleg := isVerticalSegment(p0, p1) &&
			isHorizontalSegment(p1, p2) &&
			isVerticalSegment(p2, p3) &&
			isHorizontalSegment(p3, p4) &&
			isVerticalSegment(p4, p5) &&
			sameX(p0, p4, mgEpsLocal) &&
			sameX(p0, p5, mgEpsLocal) &&
			sameX(p2, p3, mgEpsLocal) &&
			(p2.X-p1.X)*(p4.X-p3.X) < 0

		horizontalDogleg := isHorizontalSegment(p0, p1) &&
			isVerticalSegment(p1, p2) &&
			isHorizontalSegment(p2, p3) &&
			isVerticalSegment(p3, p4) &&
			isHorizontalSegment(p4, p5) &&
			sameY(p0, p4, mgEpsLocal) &&
			sameY(p0, p5, mgEpsLocal) &&
			sameY(p2, p3, mgEpsLocal) &&
			(p2.Y-p1.Y)*(p4.Y-p3.Y) < 0

		if !verticalDogleg && !horizontalDogleg {
			return nil
		}

		return dedupeConsecutivePoints(cat(points[:i+1], []Point{p5}, points[i+6:]))
	}

	for iteration := 0; iteration < MAX_ITERATIONS; iteration++ {
		fixed := false
		for _, edge := range edges {
			if edge.IsLayoutOnly {
				continue
			}
			points := dedupeConsecutivePoints(edge.Points)
			for i := 0; i <= len(points)-5; i++ {
				candidate := withoutDogleg(points, i)
				if candidate == nil || !candidateIsSafe(edge, candidate) {
					continue
				}
				edge.Points = candidate
				fixed = true
				break
			}
			if fixed {
				break
			}
		}
		if !fixed {
			return
		}
	}
}

func liftObstacleHuggingSameSideRails(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	const BUFFER = 2
	const CLEARANCE = 20
	const MAX_ITERATIONS = 8

	type MiddleRail struct {
		index      int
		horizontal bool
		vertical   bool
		segment    *OrthogonalSegment
	}

	realNodeRects, labelRects := collectNodeRectEntries(nodeByIdMap.Values())
	visibleEdges := mgVisibleEdges(edges)

	// pointsFor has the optional parameters replacementEdge and replacement.
	// A nil replacementEdge stands for undefined.
	pointsFor := func(edge *ru.Edge, replacementEdge *ru.Edge, replacement []Point) []Point {
		if edge == replacementEdge {
			return dedupeConsecutivePoints(replacement)
		}
		return dedupeConsecutivePoints(edge.Points)
	}

	strictCrossingCount := func(replacementEdge *ru.Edge, replacement []Point) int {
		count := 0
		for i := 0; i < len(visibleEdges); i++ {
			firstSegments := mgSegmentsFor(pointsFor(visibleEdges[i], replacementEdge, replacement))
			for j := i + 1; j < len(visibleEdges); j++ {
				secondSegments := mgSegmentsFor(
					pointsFor(visibleEdges[j], replacementEdge, replacement),
				)
				count += mgStrictCrossingsBetween(firstSegments, secondSegments)
			}
		}
		return count
	}

	middleRail := func(points []Point) *MiddleRail {
		segments := mgSegmentsFor(points)
		if len(segments) != 3 {
			return nil
		}
		middle := segments[1]
		if segments[0].Horizontal == middle.Horizontal ||
			segments[2].Horizontal == middle.Horizontal {
			return nil
		}
		return &MiddleRail{
			index:      middle.Index,
			horizontal: middle.Horizontal,
			vertical:   middle.Vertical,
			segment:    middle,
		}
	}

	blockingRectsFor := func(edge *ru.Edge, rail *OrthogonalSegment) []RectEntry {
		endpointIds := endpointIdsOf(edge.Start, edge.End)
		result := []RectEntry{}
		for _, entry := range realNodeRects {
			if containsString(endpointIds, entry.ID) {
				continue
			}
			rect := entry.Rect
			if rail.Horizontal {
				xOverlap := overlapLength(rail.A.X, rail.B.X, rect.Left, rect.Right)
				if xOverlap >= mgMinShared &&
					rail.A.Y >= rect.Top-BUFFER &&
					rail.A.Y <= rect.Bottom+BUFFER {
					result = append(result, entry)
				}
				continue
			}
			yOverlap := overlapLength(rail.A.Y, rail.B.Y, rect.Top, rect.Bottom)
			if yOverlap >= mgMinShared && rail.A.X >= rect.Left-BUFFER && rail.A.X <= rect.Right+BUFFER {
				result = append(result, entry)
			}
		}
		return result
	}

	// candidateByMovingRail returns nil for undefined. A candidate has one
	// point or more.
	candidateByMovingRail := func(points []Point, rail *OrthogonalSegment, coord float64) []Point {
		candidate := clonePoints(points)
		if rail.Horizontal {
			candidate[rail.Index].Y = coord
			candidate[rail.Index+1].Y = coord
		} else if rail.Vertical {
			candidate[rail.Index].X = coord
			candidate[rail.Index+1].X = coord
		} else {
			return nil
		}
		simplified := simplifyPolyline(dedupeConsecutivePoints(candidate))
		if len(mgSegmentsFor(simplified)) == len(simplified)-1 {
			return simplified
		}
		return nil
	}

	candidateIsSafe := func(edge *ru.Edge, candidate []Point, currentCrossings int) bool {
		endpointIds := endpointIdsOf(edge.Start, edge.End)
		candidateSegments := mgSegmentsFor(candidate)
		if len(candidateSegments) != len(candidate)-1 {
			return false
		}

		for _, segment := range candidateSegments {
			if segmentHitsAnyRect(segment.A, segment.B, realNodeRects, endpointIds, -BUFFER) {
				return false
			}
			if segmentHitsAnyRect(segment.A, segment.B, labelRects, nil, -BUFFER) {
				return false
			}
		}

		for _, other := range visibleEdges {
			if other == edge {
				continue
			}
			for _, candidateSegment := range candidateSegments {
				for _, otherSegment := range mgSegmentsFor(pointsFor(other, nil, nil)) {
					if sameAxisSegmentOverlapLength(candidateSegment, otherSegment, 0.5) >= mgMinShared {
						return false
					}
				}
			}
		}

		return strictCrossingCount(edge, candidate) <= currentCrossings
	}

	for iteration := 0; iteration < MAX_ITERATIONS; iteration++ {
		currentCrossings := strictCrossingCount(nil, nil)
		fixed := false

		for _, edge := range visibleEdges {
			points := pointsFor(edge, nil, nil)
			rail := middleRail(points)
			if rail == nil {
				continue
			}
			blockers := blockingRectsFor(edge, rail.segment)
			if len(blockers) == 0 {
				continue
			}

			var lows, highs []float64
			for _, entry := range blockers {
				if rail.horizontal {
					lows = append(lows, entry.Rect.Top)
					highs = append(highs, entry.Rect.Bottom)
				} else {
					lows = append(lows, entry.Rect.Left)
					highs = append(highs, entry.Rect.Right)
				}
			}
			coords := []float64{
				js.Min(lows...) - CLEARANCE,
				js.Max(highs...) + CLEARANCE,
			}

			for _, coord := range coords {
				candidate := candidateByMovingRail(points, rail.segment, coord)
				if candidate == nil || !candidateIsSafe(edge, candidate, currentCrossings) {
					continue
				}
				edge.Points = candidate
				fixed = true
				break
			}
			if fixed {
				break
			}
		}

		if !fixed {
			return
		}
	}
}

// liftTopLaneTitleBandsAboveRails moves top-lane title bands above any rail
// that crosses them. A lane with no `direction` of its own takes the
// layout's `direction`. An empty layoutDirection stands for undefined.
func liftTopLaneTitleBandsAboveRails(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node], layoutDirection string) {
	const CLEARANCE = 4

	type LaneTitle struct {
		node *ru.Node
		rect RectBounds
	}

	validTitleRect := func(node *ru.Node) (RectBounds, bool) {
		rect := node.GroupTitleRect
		if rect == nil ||
			!isFiniteNumber(rect.Left) ||
			!isFiniteNumber(rect.Right) ||
			!isFiniteNumber(rect.Top) ||
			!isFiniteNumber(rect.Bottom) ||
			rect.Right <= rect.Left ||
			rect.Bottom <= rect.Top {
			return RectBounds{}, false
		}
		return RectBounds{Left: rect.Left, Right: rect.Right, Top: rect.Top, Bottom: rect.Bottom}, true
	}

	topLaneTitleFor := func(node *ru.Node) *LaneTitle {
		if !node.IsGroup || node.ParentID != "" {
			return nil
		}
		direction := mgUpperCaseDirection(mgDirectionOf(node, layoutDirection))
		if direction == "LR" || direction == "RL" || direction == "BT" {
			return nil
		}
		rect, ok := validTitleRect(node)
		y := node.Y
		height := node.Height
		if !ok ||
			!isFiniteNumber(y) ||
			!isFiniteNumber(height) ||
			height <= 0 {
			return nil
		}
		titleWidth := rect.Right - rect.Left
		titleHeight := rect.Bottom - rect.Top
		if titleHeight <= 0 || titleWidth < titleHeight {
			return nil
		}
		return &LaneTitle{node: node, rect: rect}
	}

	horizontalSegmentIntersectsTitle := func(segment *OrthogonalSegment, rect RectBounds) bool {
		if !segment.Horizontal {
			return false
		}
		y := segment.A.Y
		// A rail along the band's edge counts, as it does for validateLayout.
		if y < rect.Top-mgEpsLocal || y > rect.Bottom+mgEpsLocal {
			return false
		}
		return overlapLength(segment.A.X, segment.B.X, rect.Left, rect.Right) >= mgMinShared
	}

	lanes := []*LaneTitle{}
	for _, node := range nodeByIdMap.Values() {
		if lane := topLaneTitleFor(node); lane != nil {
			lanes = append(lanes, lane)
		}
	}
	if len(lanes) == 0 {
		return
	}

	topDelta := 0.0
	for _, edge := range edges {
		if edge.IsLayoutOnly {
			continue
		}
		points := dedupeConsecutivePoints(edge.Points)
		for _, segment := range mgSegmentsFor(points) {
			for _, lane := range lanes {
				if !horizontalSegmentIntersectsTitle(segment, lane.rect) {
					continue
				}
				topDelta = js.Max(topDelta, lane.rect.Bottom-segment.A.Y+CLEARANCE)
			}
		}
	}

	if topDelta <= mgEpsLocal {
		return
	}

	for _, lane := range lanes {
		y := lane.node.Y
		height := lane.node.Height
		if !isFiniteNumber(y) ||
			!isFiniteNumber(height) ||
			height <= 0 {
			continue
		}
		lane.node.Y = y - float64(topDelta/2)
		lane.node.Height = height + topDelta
		lane.node.GroupTitleRect = &ru.GroupTitleRect{
			Left:   lane.rect.Left,
			Right:  lane.rect.Right,
			Top:    lane.rect.Top - topDelta,
			Bottom: lane.rect.Bottom - topDelta,
		}
	}
}

// shiftLeftLaneTitleBandsLeftOfRails moves left-lane title bands of an LR
// layout left of any rail that crosses them. A lane with no `direction` of
// its own takes the layout's `direction`. An empty layoutDirection stands
// for undefined.
func shiftLeftLaneTitleBandsLeftOfRails(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node], layoutDirection string) {
	const CLEARANCE = 4

	type LaneTitle struct {
		node *ru.Node
		rect RectBounds
	}

	validTitleRect := func(node *ru.Node) (RectBounds, bool) {
		rect := node.GroupTitleRect
		if rect == nil ||
			!isFiniteNumber(rect.Left) ||
			!isFiniteNumber(rect.Right) ||
			!isFiniteNumber(rect.Top) ||
			!isFiniteNumber(rect.Bottom) ||
			rect.Right <= rect.Left ||
			rect.Bottom <= rect.Top {
			return RectBounds{}, false
		}
		return RectBounds{Left: rect.Left, Right: rect.Right, Top: rect.Top, Bottom: rect.Bottom}, true
	}

	leftLaneTitleFor := func(node *ru.Node) *LaneTitle {
		if !node.IsGroup || node.ParentID != "" {
			return nil
		}
		direction := mgUpperCaseDirection(mgDirectionOf(node, layoutDirection))
		if direction != "LR" {
			return nil
		}
		rect, ok := validTitleRect(node)
		x := node.X
		width := node.Width
		if !ok ||
			!isFiniteNumber(x) ||
			!isFiniteNumber(width) ||
			width <= 0 {
			return nil
		}
		titleWidth := rect.Right - rect.Left
		titleHeight := rect.Bottom - rect.Top
		if titleWidth <= 0 || titleHeight < titleWidth {
			return nil
		}
		return &LaneTitle{node: node, rect: rect}
	}

	verticalSegmentIntersectsTitle := func(segment *OrthogonalSegment, rect RectBounds) bool {
		if !segment.Vertical {
			return false
		}
		x := segment.A.X
		// A rail along the band's edge counts, as it does for validateLayout.
		if x < rect.Left-mgEpsLocal || x > rect.Right+mgEpsLocal {
			return false
		}
		return overlapLength(segment.A.Y, segment.B.Y, rect.Top, rect.Bottom) >= mgMinShared
	}

	horizontalSegmentIntersectsTitle := func(segment *OrthogonalSegment, rect RectBounds) bool {
		if !segment.Horizontal {
			return false
		}
		y := segment.A.Y
		// A rail along the band's edge counts, as it does for validateLayout.
		if y < rect.Top-mgEpsLocal || y > rect.Bottom+mgEpsLocal {
			return false
		}
		return overlapLength(segment.A.X, segment.B.X, rect.Left, rect.Right) >= mgMinShared
	}

	lanes := []*LaneTitle{}
	for _, node := range nodeByIdMap.Values() {
		if lane := leftLaneTitleFor(node); lane != nil {
			lanes = append(lanes, lane)
		}
	}
	if len(lanes) == 0 {
		return
	}

	leftDelta := 0.0
	for _, edge := range edges {
		if edge.IsLayoutOnly {
			continue
		}
		points := dedupeConsecutivePoints(edge.Points)
		for _, segment := range mgSegmentsFor(points) {
			for _, lane := range lanes {
				if verticalSegmentIntersectsTitle(segment, lane.rect) {
					leftDelta = js.Max(leftDelta, lane.rect.Right-segment.A.X+CLEARANCE)
				} else if horizontalSegmentIntersectsTitle(segment, lane.rect) {
					segmentLeft := js.Min(segment.A.X, segment.B.X)
					leftDelta = js.Max(leftDelta, lane.rect.Right-segmentLeft+CLEARANCE)
				}
			}
		}
	}

	if leftDelta <= mgEpsLocal {
		return
	}

	for _, lane := range lanes {
		x := lane.node.X
		width := lane.node.Width
		if !isFiniteNumber(x) ||
			!isFiniteNumber(width) ||
			width <= 0 {
			continue
		}
		lane.node.X = x - float64(leftDelta/2)
		lane.node.Width = width + leftDelta
		lane.node.GroupTitleRect = &ru.GroupTitleRect{
			Left:   lane.rect.Left - leftDelta,
			Right:  lane.rect.Right - leftDelta,
			Top:    lane.rect.Top,
			Bottom: lane.rect.Bottom,
		}
	}
}

func swapDestinationTerminalTailsToReduceCrossings(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	const BUFFER = 2
	const MAX_ITERATIONS = 4

	type TerminalTail struct {
		tailStart Point
		terminal  Point
	}

	realNodeRects, _ := collectNodeRectEntries(nodeByIdMap.Values())
	visibleEdges := mgVisibleEdges(edges)

	// replacementPointsFor has the default parameter `replacements = new
	// Map()`: nil stands for it.
	replacementPointsFor := func(edge *ru.Edge, replacements *mgEdgeReplacementMap) []Point {
		if replacements != nil {
			if points, ok := replacements.Lookup(edge); ok {
				return dedupeConsecutivePoints(points)
			}
		}
		return dedupeConsecutivePoints(edge.Points)
	}

	crossingCount := func(replacements *mgEdgeReplacementMap) int {
		count := 0
		for i := 0; i < len(visibleEdges); i++ {
			firstSegments := mgSegmentsFor(replacementPointsFor(visibleEdges[i], replacements))
			for j := i + 1; j < len(visibleEdges); j++ {
				secondSegments := mgSegmentsFor(replacementPointsFor(visibleEdges[j], replacements))
				count += mgStrictCrossingsBetween(firstSegments, secondSegments)
			}
		}
		return count
	}

	totalBends := func(replacements *mgEdgeReplacementMap) int {
		sum := 0
		for _, edge := range visibleEdges {
			sum = sum + countOrthogonalBends(replacementPointsFor(edge, replacements))
		}
		return sum
	}

	terminalTailFor := func(edge *ru.Edge) *TerminalTail {
		points := replacementPointsFor(edge, nil)
		if len(points) < 4 {
			return nil
		}
		tailStart := points[len(points)-2]
		terminal := points[len(points)-1]
		if !isHorizontalSegment(tailStart, terminal, mgEpsLocal) &&
			!isVerticalSegment(tailStart, terminal, mgEpsLocal) {
			return nil
		}
		return &TerminalTail{tailStart: tailStart, terminal: terminal}
	}

	// candidateWithDestinationTail returns nil for undefined. A candidate
	// has one point or more.
	candidateWithDestinationTail := func(edge *ru.Edge, tail *TerminalTail) []Point {
		points := replacementPointsFor(edge, nil)
		if len(points) < 3 {
			return nil
		}
		start := points[0]
		firstTurn := points[1]

		var connector Point
		if isHorizontalSegment(start, firstTurn, mgEpsLocal) {
			connector = Point{X: firstTurn.X, Y: tail.tailStart.Y}
		} else if isVerticalSegment(start, firstTurn, mgEpsLocal) {
			connector = Point{X: tail.tailStart.X, Y: firstTurn.Y}
		} else {
			return nil
		}

		candidate := simplifyPolyline(
			dedupeConsecutivePoints([]Point{start, firstTurn, connector, tail.tailStart, tail.terminal}),
		)
		if len(mgSegmentsFor(candidate)) == len(candidate)-1 {
			return candidate
		}
		return nil
	}

	pathHasNodeHit := func(edge *ru.Edge, path []Point) bool {
		endpointIds := endpointIdsOf(edge.Start, edge.End)
		for _, segment := range mgSegmentsFor(path) {
			if segmentHitsAnyRect(segment.A, segment.B, realNodeRects, endpointIds, -BUFFER) {
				return true
			}
		}
		return false
	}

	pathHasSharedTrack := func(edge *ru.Edge, path []Point, replacements *mgEdgeReplacementMap) bool {
		for _, other := range visibleEdges {
			if other == edge {
				continue
			}
			for _, candidateSegment := range mgSegmentsFor(path) {
				for _, otherSegment := range mgSegmentsFor(replacementPointsFor(other, replacements)) {
					if sameAxisSegmentOverlapLength(candidateSegment, otherSegment, 0.5) >= mgMinShared {
						return true
					}
				}
			}
		}
		return false
	}

	candidateIsSafe := func(edge *ru.Edge, path []Point, replacements *mgEdgeReplacementMap) bool {
		return !pathHasNodeHit(edge, path) && !pathHasSharedTrack(edge, path, replacements)
	}

	edgesByDestination := func() *js.Map[string, []*ru.Edge] {
		result := js.NewMap[string, []*ru.Edge]()
		for _, edge := range visibleEdges {
			dstId := edge.End
			if dstId == "" || !nodeByIdMap.Has(dstId) {
				continue
			}
			points := replacementPointsFor(edge, nil)
			if len(points) < 4 {
				continue
			}
			bucket := result.Get(dstId)
			bucket = append(bucket, edge)
			result.Set(dstId, bucket)
		}
		return result
	}

	for iteration := 0; iteration < MAX_ITERATIONS; iteration++ {
		currentCrossings := crossingCount(nil)
		if currentCrossings == 0 {
			return
		}
		currentBends := totalBends(nil)

		var bestReplacements *mgEdgeReplacementMap
		bestCrossings := currentCrossings
		bestBends := currentBends

		for _, destinationEdges := range edgesByDestination().Values() {
			for i := 0; i < len(destinationEdges); i++ {
				for j := i + 1; j < len(destinationEdges); j++ {
					first := destinationEdges[i]
					second := destinationEdges[j]
					firstTail := terminalTailFor(first)
					secondTail := terminalTailFor(second)
					if firstTail == nil || secondTail == nil {
						continue
					}

					firstCandidate := candidateWithDestinationTail(first, secondTail)
					secondCandidate := candidateWithDestinationTail(second, firstTail)
					if firstCandidate == nil || secondCandidate == nil {
						continue
					}

					replacements := js.NewMap[*ru.Edge, []Point]()
					replacements.Set(first, firstCandidate)
					replacements.Set(second, secondCandidate)
					if !candidateIsSafe(first, firstCandidate, replacements) ||
						!candidateIsSafe(second, secondCandidate, replacements) {
						continue
					}

					candidateCrossings := crossingCount(replacements)
					candidateBends := totalBends(replacements)
					if candidateCrossings >= currentCrossings {
						continue
					}
					if candidateCrossings > bestCrossings ||
						(candidateCrossings == bestCrossings && candidateBends >= bestBends) {
						continue
					}
					bestReplacements = replacements
					bestCrossings = candidateCrossings
					bestBends = candidateBends
				}
			}
		}

		if bestReplacements == nil {
			return
		}

		for _, edge := range bestReplacements.Keys() {
			edge.Points = bestReplacements.Get(edge)
		}
	}
}

// reassignCrossingExternalRailChannels reorders a small bundle of external
// rails as one transaction: moving either rail alone is neutral or worse,
// while swapping the shared external tracks removes crossings. The search
// is local and bounded, and it is scored globally, so that the crossing
// count stays the first acceptance criterion.
func reassignCrossingExternalRailChannels(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	const BUFFER = 2
	const RAIL_CHANNEL_GAP = 12
	const MAX_ITERATIONS = 4
	const MAX_EXHAUSTIVE_COMPONENT = 6

	type ExternalRail struct {
		edge         *ru.Edge
		points       []Point
		segmentIndex int
		// axis is "horizontal" or "vertical".
		axis  string
		side  RectSide
		coord float64
		min   float64
		max   float64
	}

	realNodeRects, labelRects := collectNodeRectEntries(nodeByIdMap.Values())
	visibleEdges := mgVisibleEdges(edges)

	// replacementPointsFor has the default parameter `replacements = new
	// Map()`: nil stands for it.
	replacementPointsFor := func(edge *ru.Edge, replacements *mgEdgeReplacementMap) []Point {
		if replacements != nil {
			if points, ok := replacements.Lookup(edge); ok {
				return dedupeConsecutivePoints(points)
			}
		}
		return dedupeConsecutivePoints(edge.Points)
	}

	strictCrossingCount := func(replacements *mgEdgeReplacementMap) int {
		count := 0
		for i := 0; i < len(visibleEdges); i++ {
			firstSegments := mgSegmentsFor(replacementPointsFor(visibleEdges[i], replacements))
			for j := i + 1; j < len(visibleEdges); j++ {
				secondSegments := mgSegmentsFor(replacementPointsFor(visibleEdges[j], replacements))
				count += mgStrictCrossingsBetween(firstSegments, secondSegments)
			}
		}
		return count
	}

	totalBends := func(replacements *mgEdgeReplacementMap) int {
		sum := 0
		for _, edge := range visibleEdges {
			sum = sum + countOrthogonalBends(replacementPointsFor(edge, replacements))
		}
		return sum
	}

	endpointRectsFor := func(edge *ru.Edge) (src, dst RectBounds, ok bool) {
		srcNode := mgNodeOf(nodeByIdMap, edge.Start)
		dstNode := mgNodeOf(nodeByIdMap, edge.End)
		var hasSrc, hasDst bool
		if srcNode != nil {
			src, hasSrc = rectOfNodeBounds(srcNode)
		}
		if dstNode != nil {
			dst, hasDst = rectOfNodeBounds(dstNode)
		}
		return src, dst, hasSrc && hasDst
	}

	externalRailForSegment := func(edge *ru.Edge, points []Point, segment *OrthogonalSegment) *ExternalRail {
		if segment.Index <= 0 || segment.Index+1 >= len(points)-1 {
			return nil
		}

		srcRect, dstRect, ok := endpointRectsFor(edge)
		if !ok {
			return nil
		}

		if segment.Vertical {
			coord := segment.A.X
			leftBound := js.Min(srcRect.Left, dstRect.Left)
			rightBound := js.Max(srcRect.Right, dstRect.Right)
			side := ""
			if coord < leftBound-mgEpsLocal {
				side = "left"
			} else if coord > rightBound+mgEpsLocal {
				side = "right"
			}
			if side == "" {
				return nil
			}
			return &ExternalRail{
				edge:         edge,
				points:       points,
				segmentIndex: segment.Index,
				axis:         "vertical",
				side:         side,
				coord:        coord,
				min:          js.Min(segment.A.Y, segment.B.Y),
				max:          js.Max(segment.A.Y, segment.B.Y),
			}
		}

		if segment.Horizontal {
			coord := segment.A.Y
			topBound := js.Min(srcRect.Top, dstRect.Top)
			bottomBound := js.Max(srcRect.Bottom, dstRect.Bottom)
			side := ""
			if coord < topBound-mgEpsLocal {
				side = "top"
			} else if coord > bottomBound+mgEpsLocal {
				side = "bottom"
			}
			if side == "" {
				return nil
			}
			return &ExternalRail{
				edge:         edge,
				points:       points,
				segmentIndex: segment.Index,
				axis:         "horizontal",
				side:         side,
				coord:        coord,
				min:          js.Min(segment.A.X, segment.B.X),
				max:          js.Max(segment.A.X, segment.B.X),
			}
		}

		return nil
	}

	collectExternalRails := func() []*ExternalRail {
		rails := []*ExternalRail{}
		for _, edge := range visibleEdges {
			points := replacementPointsFor(edge, nil)
			for _, segment := range mgSegmentsFor(points) {
				rail := externalRailForSegment(edge, points, segment)
				if rail != nil {
					rails = append(rails, rail)
				}
			}
		}
		return rails
	}

	railsInteract := func(a, b *ExternalRail) bool {
		return a.edge != b.edge &&
			a.axis == b.axis &&
			a.side == b.side &&
			overlapLength(a.min, a.max, b.min, b.max) >= mgMinShared
	}

	connectedComponents := func(rails []*ExternalRail) [][]*ExternalRail {
		result := [][]*ExternalRail{}
		seen := map[*ExternalRail]bool{}
		for _, rail := range rails {
			if seen[rail] {
				continue
			}
			queue := []*ExternalRail{rail}
			component := []*ExternalRail{}
			seen[rail] = true
			for len(queue) > 0 {
				current := queue[len(queue)-1]
				queue = queue[:len(queue)-1]
				component = append(component, current)
				for _, next := range rails {
					if !seen[next] && railsInteract(current, next) {
						seen[next] = true
						queue = append(queue, next)
					}
				}
			}
			if len(component) > 1 {
				result = append(result, component)
			}
		}
		return result
	}

	uniqueCoordsFor := func(component []*ExternalRail) []float64 {
		coords := []float64{}
		for _, rail := range component {
			found := false
			for _, coord := range coords {
				if math.Abs(coord-rail.coord) < mgEpsLocal {
					found = true
					break
				}
			}
			if !found {
				coords = append(coords, rail.coord)
			}
		}

		for len(coords) < len(component) {
			minCoord := js.Min(coords...)
			maxCoord := js.Max(coords...)
			side := component[0].side
			missing := float64(len(component) - len(coords))
			if side == "left" || side == "top" {
				coords = append(coords, minCoord-float64(RAIL_CHANNEL_GAP*missing))
			} else {
				coords = append(coords, maxCoord+float64(RAIL_CHANNEL_GAP*missing))
			}
		}
		return coords
	}

	coordinateAssignmentsFor := func(component []*ExternalRail) [][]float64 {
		current := make([]float64, len(component))
		for i, rail := range component {
			current[i] = rail.coord
		}
		coords := uniqueCoordsFor(component)
		assignments := [][]float64{}

		if len(component) <= MAX_EXHAUSTIVE_COMPONENT {
			used := make([]bool, len(coords))
			next := []float64{}
			var visit func()
			visit = func() {
				if len(next) == len(component) {
					differs := false
					for index, coord := range next {
						if math.Abs(coord-current[index]) >= mgEpsLocal {
							differs = true
							break
						}
					}
					if differs {
						assignments = append(assignments, append([]float64{}, next...))
					}
					return
				}
				for i, coord := range coords {
					if used[i] {
						continue
					}
					used[i] = true
					next = append(next, coord)
					visit()
					next = next[:len(next)-1]
					used[i] = false
				}
			}
			visit()
			return assignments
		}

		for i := 0; i < len(current); i++ {
			for j := i + 1; j < len(current); j++ {
				assignment := append([]float64{}, current...)
				assignment[i], assignment[j] = assignment[j], assignment[i]
				assignments = append(assignments, assignment)
			}
		}
		return assignments
	}

	// replacementsForAssignment returns nil for undefined.
	replacementsForAssignment := func(component []*ExternalRail, assignment []float64) *mgEdgeReplacementMap {
		// One draft per edge: every rail of an edge changes the same draft.
		draftByEdge := js.NewMap[*ru.Edge, []Point]()
		for i, rail := range component {
			coord := assignment[i]
			points, ok := draftByEdge.Lookup(rail.edge)
			if !ok {
				points = clonePoints(rail.points)
			}
			if rail.axis == "vertical" {
				points[rail.segmentIndex].X = coord
				points[rail.segmentIndex+1].X = coord
			} else {
				points[rail.segmentIndex].Y = coord
				points[rail.segmentIndex+1].Y = coord
			}
			draftByEdge.Set(rail.edge, points)
		}

		replacements := js.NewMap[*ru.Edge, []Point]()
		for _, edge := range draftByEdge.Keys() {
			points := draftByEdge.Get(edge)
			simplified := simplifyPolyline(dedupeConsecutivePoints(points))
			if len(mgSegmentsFor(simplified)) != len(simplified)-1 {
				return nil
			}
			replacements.Set(edge, simplified)
		}
		return replacements
	}

	candidateIsSafe := func(replacements *mgEdgeReplacementMap) bool {
		for _, edge := range replacements.Keys() {
			points := replacements.Get(edge)
			endpointIds := endpointIdsOf(edge.Start, edge.End)
			for _, segment := range mgSegmentsFor(points) {
				if segmentHitsAnyRect(segment.A, segment.B, realNodeRects, endpointIds, -BUFFER) {
					return false
				}
				if segmentHitsAnyRect(segment.A, segment.B, labelRects, nil, -BUFFER) {
					return false
				}
			}
		}

		for i := 0; i < len(visibleEdges); i++ {
			first := visibleEdges[i]
			firstChanged := replacements.Has(first)
			firstSegments := mgSegmentsFor(replacementPointsFor(first, replacements))
			for j := i + 1; j < len(visibleEdges); j++ {
				second := visibleEdges[j]
				if !firstChanged && !replacements.Has(second) {
					continue
				}
				secondSegments := mgSegmentsFor(replacementPointsFor(second, replacements))
				for _, firstSegment := range firstSegments {
					for _, secondSegment := range secondSegments {
						if sameAxisSegmentOverlapLength(firstSegment, secondSegment, 0.5) >= mgMinShared {
							return false
						}
					}
				}
			}
		}

		return true
	}

	for iteration := 0; iteration < MAX_ITERATIONS; iteration++ {
		currentCrossings := strictCrossingCount(nil)
		if currentCrossings == 0 {
			return
		}

		var bestReplacements *mgEdgeReplacementMap
		bestCrossings := currentCrossings
		bestBends := totalBends(nil)
		bestDisplacement := math.Inf(1)

		for _, component := range connectedComponents(collectExternalRails()) {
			for _, assignment := range coordinateAssignmentsFor(component) {
				replacements := replacementsForAssignment(component, assignment)
				if replacements == nil || !candidateIsSafe(replacements) {
					continue
				}

				candidateCrossings := strictCrossingCount(replacements)
				if candidateCrossings >= currentCrossings {
					continue
				}
				candidateBends := totalBends(replacements)
				candidateDisplacement := 0.0
				for index, rail := range component {
					candidateDisplacement = candidateDisplacement + math.Abs(assignment[index]-rail.coord)
				}

				if candidateCrossings > bestCrossings ||
					(candidateCrossings == bestCrossings &&
						(candidateBends > bestBends ||
							(candidateBends == bestBends && candidateDisplacement >= bestDisplacement))) {
					continue
				}

				bestReplacements = replacements
				bestCrossings = candidateCrossings
				bestBends = candidateBends
				bestDisplacement = candidateDisplacement
			}
		}

		if bestReplacements == nil {
			return
		}

		for _, edge := range bestReplacements.Keys() {
			edge.Points = bestReplacements.Get(edge)
		}
	}
}

func shortcutRedundantOrthogonalJogs(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	const BUFFER = 2
	const MAX_ITERATIONS = 8

	realNodeRects, labelRects := collectNodeRectEntries(nodeByIdMap.Values())
	visibleEdges := mgVisibleEdges(edges)

	// pointsFor has the optional parameters replacementEdge and replacement.
	// A nil replacementEdge stands for undefined.
	pointsFor := func(edge *ru.Edge, replacementEdge *ru.Edge, replacement []Point) []Point {
		if edge == replacementEdge {
			return dedupeConsecutivePoints(replacement)
		}
		return dedupeConsecutivePoints(edge.Points)
	}

	pathLength := func(points []Point) float64 {
		sum := 0.0
		for _, segment := range mgSegmentsFor(points) {
			dx := segment.A.X - segment.B.X
			dy := segment.A.Y - segment.B.Y
			sum = sum + js.Hypot(dx, dy)
		}
		return sum
	}

	strictCrossingCount := func(replacementEdge *ru.Edge, replacement []Point) int {
		count := 0
		for i := 0; i < len(visibleEdges); i++ {
			firstSegments := mgSegmentsFor(pointsFor(visibleEdges[i], replacementEdge, replacement))
			for j := i + 1; j < len(visibleEdges); j++ {
				secondSegments := mgSegmentsFor(
					pointsFor(visibleEdges[j], replacementEdge, replacement),
				)
				count += mgStrictCrossingsBetween(firstSegments, secondSegments)
			}
		}
		return count
	}

	segmentRunsAlongRectBorder := func(segment *OrthogonalSegment, rect RectBounds) bool {
		if segment.Horizontal {
			y := segment.A.Y
			onBorder := math.Abs(y-rect.Top) < 1 || math.Abs(y-rect.Bottom) < 1
			return onBorder && overlapLength(segment.A.X, segment.B.X, rect.Left, rect.Right) >= mgMinShared
		}

		if segment.Vertical {
			x := segment.A.X
			onBorder := math.Abs(x-rect.Left) < 1 || math.Abs(x-rect.Right) < 1
			return onBorder && overlapLength(segment.A.Y, segment.B.Y, rect.Top, rect.Bottom) >= mgMinShared
		}

		return false
	}

	endpointRectsFor := func(edge *ru.Edge) []RectBounds {
		endpointIds := endpointIdsOf(edge.Start, edge.End)
		rects := []RectBounds{}
		for _, id := range endpointIds {
			node := nodeByIdMap.Get(id)
			if node == nil {
				continue
			}
			if rect, ok := rectOfNodeBounds(node); ok {
				rects = append(rects, rect)
			}
		}
		return rects
	}

	shortcutCandidatesAt := func(points []Point, index int) [][]Point {
		if index+3 >= len(points) {
			return nil
		}

		p0 := points[index]
		p1 := points[index+1]
		p2 := points[index+2]
		p3 := points[index+3]
		isHVH := isHorizontalSegment(p0, p1, mgEpsLocal) &&
			isVerticalSegment(p1, p2, mgEpsLocal) &&
			isHorizontalSegment(p2, p3, mgEpsLocal)
		isVHV := isVerticalSegment(p0, p1, mgEpsLocal) &&
			isHorizontalSegment(p1, p2, mgEpsLocal) &&
			isVerticalSegment(p2, p3, mgEpsLocal)
		if !isHVH && !isVHV {
			return nil
		}
		var outerSegmentsOppose bool
		if isHVH {
			outerSegmentsOppose = js.Sign(p1.X-p0.X) != js.Sign(p3.X-p2.X)
		} else {
			outerSegmentsOppose = js.Sign(p1.Y-p0.Y) != js.Sign(p3.Y-p2.Y)
		}
		if !outerSegmentsOppose {
			return nil
		}

		var corners []Point
		if !(sameX(p0, p3, mgEpsLocal) || sameY(p0, p3, mgEpsLocal)) {
			corners = []Point{
				{X: p0.X, Y: p3.Y},
				{X: p3.X, Y: p0.Y},
			}
		}
		rawCandidates := [][]Point{}
		if len(corners) == 0 {
			rawCandidates = append(rawCandidates, cat(points[:index+1], points[index+3:]))
		} else {
			for _, corner := range corners {
				rawCandidates = append(rawCandidates, cat(
					points[:index+1],
					[]Point{corner},
					points[index+3:],
				))
			}
		}

		seen := map[string]bool{}
		simplified := make([][]Point, len(rawCandidates))
		for i, candidate := range rawCandidates {
			simplified[i] = simplifyPolyline(dedupeConsecutivePoints(candidate))
		}
		result := [][]Point{}
		for _, candidate := range simplified {
			if len(mgSegmentsFor(candidate)) != len(candidate)-1 {
				continue
			}
			hasP3 := false
			for _, point := range candidate {
				if samePoint(point, p3, mgEpsLocal) {
					hasP3 = true
					break
				}
			}
			if !hasP3 {
				continue
			}
			key := pathKey3(candidate)
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, candidate)
		}
		return result
	}

	candidateIsSafe := func(edge *ru.Edge, candidate []Point, currentCrossings int) bool {
		endpointIds := endpointIdsOf(edge.Start, edge.End)
		endpointRects := endpointRectsFor(edge)

		for _, segment := range mgSegmentsFor(candidate) {
			if segmentHitsAnyRect(segment.A, segment.B, realNodeRects, endpointIds, -BUFFER) {
				return false
			}
			if segmentHitsAnyRect(segment.A, segment.B, labelRects, nil, -BUFFER) {
				return false
			}
			for _, rect := range endpointRects {
				if segmentRunsAlongRectBorder(segment, rect) {
					return false
				}
			}
		}

		for _, other := range visibleEdges {
			if other == edge {
				continue
			}
			for _, candidateSegment := range mgSegmentsFor(candidate) {
				for _, otherSegment := range mgSegmentsFor(pointsFor(other, nil, nil)) {
					if sameAxisSegmentOverlapLength(candidateSegment, otherSegment, 0.5) >= mgMinShared {
						return false
					}
				}
			}
		}

		return strictCrossingCount(edge, candidate) <= currentCrossings
	}

	for iteration := 0; iteration < MAX_ITERATIONS; iteration++ {
		currentCrossings := strictCrossingCount(nil, nil)
		var bestEdge *ru.Edge
		var bestPath []Point
		bestCrossings := currentCrossings
		// bestBends starts at Number.POSITIVE_INFINITY.
		bestBends := math.Inf(1)
		bestLength := math.Inf(1)

		for _, edge := range visibleEdges {
			currentPoints := pointsFor(edge, nil, nil)
			currentBends := countOrthogonalBends(currentPoints, mgEpsLocal)
			currentLength := pathLength(currentPoints)
			for index := 0; index <= len(currentPoints)-4; index++ {
				for _, candidate := range shortcutCandidatesAt(currentPoints, index) {
					candidateBends := countOrthogonalBends(candidate, mgEpsLocal)
					candidateLength := pathLength(candidate)
					improvesShape := candidateBends < currentBends ||
						(candidateBends == currentBends && candidateLength < currentLength-mgEpsLocal)
					if !improvesShape || !candidateIsSafe(edge, candidate, currentCrossings) {
						continue
					}

					candidateCrossings := strictCrossingCount(edge, candidate)
					if candidateCrossings > bestCrossings ||
						(candidateCrossings == bestCrossings &&
							(float64(candidateBends) > bestBends ||
								(float64(candidateBends) == bestBends && candidateLength >= bestLength))) {
						continue
					}

					bestEdge = edge
					bestPath = candidate
					bestCrossings = candidateCrossings
					bestBends = float64(candidateBends)
					bestLength = candidateLength
				}
			}
		}

		// bestPath is set together with bestEdge.
		if bestEdge == nil {
			return
		}

		bestEdge.Points = bestPath
	}
}

func resolveRenderedOrthogonalCrossings(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	const ANCHOR = 20
	const EXTRA_CHANNEL_COUNT = 2
	const MAX_ITERATIONS = 4
	const MAX_PAIR_CANDIDATES_PER_EDGE = 48

	type NodeInfo struct {
		id   string
		cx   float64
		cy   float64
		rect RectBounds
	}

	type CrossingPair struct {
		first  *ru.Edge
		second *ru.Edge
		count  int
	}

	type CrossingSnapshot struct {
		count   int
		pairs   []CrossingPair
		edgeSet map[*ru.Edge]bool
		edges   []*ru.Edge
	}

	type PairCandidate struct {
		path                 []Point
		segments             []*OrthogonalSegment
		sharedTrackConflicts *js.Set[*ru.Edge]
		totalBends           int
		length               float64
	}

	type PairOption struct {
		edge       *ru.Edge
		candidates []*PairCandidate
	}

	type PairReplacementScore struct {
		replacements *mgEdgeReplacementMap
		crossings    int
		bends        int
		length       float64
	}

	type PairScoringContext struct {
		current          *CrossingSnapshot
		currentBends     int
		currentLength    float64
		baseBendsByEdge  map[*ru.Edge]int
		baseLengthByEdge map[*ru.Edge]float64
		baseSegments     map[*ru.Edge][]*OrthogonalSegment
	}

	realNodes := []*NodeInfo{}
	for _, node := range nodeByIdMap.Values() {
		if node.IsGroup || node.IsEdgeLabel {
			continue
		}
		cx := ru.NN(node.X, 0)
		cy := ru.NN(node.Y, 0)
		rect, ok := rectOfNodeBounds(node)
		if !ok {
			continue
		}
		realNodes = append(realNodes, &NodeInfo{
			id:   node.ID,
			cx:   cx,
			cy:   cy,
			rect: rect,
		})
	}

	if len(realNodes) == 0 {
		return
	}

	nodeInfoById := map[string]*NodeInfo{}
	realNodeRects := make([]RectEntry, 0, len(realNodes))
	var tops, bottoms, lefts, rights []float64
	for _, node := range realNodes {
		nodeInfoById[node.id] = node
		realNodeRects = append(realNodeRects, RectEntry{ID: node.id, Rect: node.rect})
		tops = append(tops, node.rect.Top)
		bottoms = append(bottoms, node.rect.Bottom)
		lefts = append(lefts, node.rect.Left)
		rights = append(rights, node.rect.Right)
	}
	sides := []RectSide{"top", "bottom", "left", "right"}
	outsideTracks := map[RectSide]float64{
		"top":    js.Min(tops...) - ANCHOR,
		"bottom": js.Max(bottoms...) + ANCHOR,
		"left":   js.Min(lefts...) - ANCHOR,
		"right":  js.Max(rights...) + ANCHOR,
	}

	visibleEdges := mgVisibleEdges(edges)
	edgeIndex := map[*ru.Edge]int{}
	for index, edge := range visibleEdges {
		edgeIndex[edge] = index
	}
	// byEdgeIndex is the comparator (a, b) => edgeIndex.get(a) - edgeIndex.get(b).
	byEdgeIndex := func(a, b *ru.Edge) float64 {
		return float64(edgeIndex[a] - edgeIndex[b])
	}

	outwardTracksForSide := func(side RectSide) []float64 {
		outward := 1.0
		if side == "left" || side == "top" {
			outward = -1
		}
		tracks := []float64{}
		for channel := 0; channel <= EXTRA_CHANNEL_COUNT; channel++ {
			tracks = append(tracks, outsideTracks[side]+float64(outward*ANCHOR*float64(channel)))
		}
		return tracks
	}

	// replacementPointsFor has the default parameter `replacements = new
	// Map()`: nil stands for it.
	replacementPointsFor := func(edge *ru.Edge, replacements *mgEdgeReplacementMap) []Point {
		if replacements != nil {
			if points, ok := replacements.Lookup(edge); ok {
				return dedupeConsecutivePoints(points)
			}
		}
		return dedupeConsecutivePoints(edge.Points)
	}

	crossingCountBetweenSegments := func(firstSegments, secondSegments []*OrthogonalSegment) int {
		return mgStrictCrossingsBetween(firstSegments, secondSegments)
	}

	// segmentCache has segmentsFor(replacementPointsFor(edge)) for the
	// current points of an edge. Upstream computes these segments again for
	// every candidate that it scores, although the edges do not change
	// until a candidate is applied. The main loop empties the cache at the
	// start of every iteration, because an iteration ends with new points.
	segmentCache := map[*ru.Edge][]*OrthogonalSegment{}
	currentSegmentsOf := func(edge *ru.Edge) []*OrthogonalSegment {
		if segments, ok := segmentCache[edge]; ok {
			return segments
		}
		segments := mgSegmentsFor(dedupeConsecutivePoints(edge.Points))
		segmentCache[edge] = segments
		return segments
	}
	// segmentsOfEdge is segmentsFor(replacementPointsFor(edge, replacements)).
	segmentsOfEdge := func(edge *ru.Edge, replacements *mgEdgeReplacementMap) []*OrthogonalSegment {
		if replacements != nil {
			if points, ok := replacements.Lookup(edge); ok {
				return mgSegmentsFor(dedupeConsecutivePoints(points))
			}
		}
		return currentSegmentsOf(edge)
	}

	crossingSnapshot := func(replacements *mgEdgeReplacementMap) *CrossingSnapshot {
		count := 0
		pairs := []CrossingPair{}
		edgeSet := map[*ru.Edge]bool{}
		edgeOrder := []*ru.Edge{}
		addEdge := func(edge *ru.Edge) {
			if !edgeSet[edge] {
				edgeSet[edge] = true
				edgeOrder = append(edgeOrder, edge)
			}
		}

		for i := 0; i < len(visibleEdges); i++ {
			first := visibleEdges[i]
			firstSegments := segmentsOfEdge(first, replacements)
			for j := i + 1; j < len(visibleEdges); j++ {
				second := visibleEdges[j]
				// crossingCountBetweenPaths of the points of the two edges.
				pairCount := crossingCountBetweenSegments(
					firstSegments,
					segmentsOfEdge(second, replacements),
				)
				if pairCount > 0 {
					count += pairCount
					pairs = append(pairs, CrossingPair{first: first, second: second, count: pairCount})
					addEdge(first)
					addEdge(second)
				}
			}
		}

		sortFunc(edgeOrder, byEdgeIndex)
		return &CrossingSnapshot{
			count:   count,
			pairs:   pairs,
			edgeSet: edgeSet,
			edges:   edgeOrder,
		}
	}

	crossingCountWithReplacements := func(current *CrossingSnapshot, replacements *mgEdgeReplacementMap) int {
		changed := map[*ru.Edge]bool{}
		for _, edge := range replacements.Keys() {
			changed[edge] = true
		}
		if len(changed) == 0 {
			return current.count
		}

		currentAffected := 0
		for _, pair := range current.pairs {
			if changed[pair.first] || changed[pair.second] {
				currentAffected += pair.count
			}
		}

		// The segments of the replaced edges, computed once. Upstream
		// computes the segments of both paths again for every pair.
		replaced := make(map[*ru.Edge][]*OrthogonalSegment, len(changed))
		for edge := range changed {
			replaced[edge] = segmentsOfEdge(edge, replacements)
		}
		segmentsOf := func(edge *ru.Edge) []*OrthogonalSegment {
			if segments, ok := replaced[edge]; ok {
				return segments
			}
			return currentSegmentsOf(edge)
		}

		replacementAffected := 0
		for i := 0; i < len(visibleEdges); i++ {
			first := visibleEdges[i]
			firstChanged := changed[first]
			for j := i + 1; j < len(visibleEdges); j++ {
				second := visibleEdges[j]
				if !firstChanged && !changed[second] {
					continue
				}
				// crossingCountBetweenPaths of the points of the two edges.
				replacementAffected += crossingCountBetweenSegments(segmentsOf(first), segmentsOf(second))
			}
		}

		return current.count - currentAffected + replacementAffected
	}

	crossingComponents := func(snapshot *CrossingSnapshot) [][]*ru.Edge {
		// The neighbors of an edge are visited in insertion order.
		neighbors := map[*ru.Edge]*js.Set[*ru.Edge]{}
		for _, pair := range snapshot.pairs {
			firstNeighbors := neighbors[pair.first]
			if firstNeighbors == nil {
				firstNeighbors = js.NewSet[*ru.Edge]()
			}
			firstNeighbors.Add(pair.second)
			neighbors[pair.first] = firstNeighbors

			secondNeighbors := neighbors[pair.second]
			if secondNeighbors == nil {
				secondNeighbors = js.NewSet[*ru.Edge]()
			}
			secondNeighbors.Add(pair.first)
			neighbors[pair.second] = secondNeighbors
		}

		components := [][]*ru.Edge{}
		seen := map[*ru.Edge]bool{}
		for _, edge := range snapshot.edges {
			if seen[edge] {
				continue
			}
			queue := []*ru.Edge{edge}
			component := []*ru.Edge{}
			seen[edge] = true
			for len(queue) > 0 {
				current := queue[len(queue)-1]
				queue = queue[:len(queue)-1]
				component = append(component, current)
				if set := neighbors[current]; set != nil {
					for _, next := range set.Values() {
						if !seen[next] {
							seen[next] = true
							queue = append(queue, next)
						}
					}
				}
			}
			sortFunc(component, byEdgeIndex)
			if len(component) > 1 {
				components = append(components, component)
			}
		}

		return components
	}

	endpointIdsFor := func(edge *ru.Edge) []string {
		return endpointIdsOf(edge.Start, edge.End)
	}

	pairSearchGroups := func(snapshot *CrossingSnapshot) [][]*ru.Edge {
		groups := [][]*ru.Edge{}
		for _, component := range crossingComponents(snapshot) {
			componentSet := map[*ru.Edge]bool{}
			componentEndpointIds := map[string]bool{}
			for _, edge := range component {
				componentSet[edge] = true
				for _, id := range endpointIdsFor(edge) {
					componentEndpointIds[id] = true
				}
			}
			group := append([]*ru.Edge{}, component...)
			for _, edge := range visibleEdges {
				if componentSet[edge] {
					continue
				}
				sharesEndpoint := false
				for _, id := range endpointIdsFor(edge) {
					if componentEndpointIds[id] {
						sharesEndpoint = true
						break
					}
				}
				if sharesEndpoint {
					group = append(group, edge)
				}
			}
			sortFunc(group, byEdgeIndex)
			groups = append(groups, group)
		}
		return groups
	}

	crossingCountWithSingleReplacement := func(current *CrossingSnapshot, edge *ru.Edge, replacement []Point) int {
		replacements := js.NewMap[*ru.Edge, []Point]()
		replacements.Set(edge, replacement)
		return crossingCountWithReplacements(current, replacements)
	}

	currentCrossingsByEdge := func(current *CrossingSnapshot) map[*ru.Edge]int {
		result := map[*ru.Edge]int{}
		for _, pair := range current.pairs {
			result[pair.first] = result[pair.first] + pair.count
			result[pair.second] = result[pair.second] + pair.count
		}
		return result
	}

	pathLength := func(points []Point) float64 {
		sum := 0.0
		for index := 1; index < len(points); index++ {
			point := points[index]
			previous := points[index-1]
			sum = sum + math.Abs(point.X-previous.X) + math.Abs(point.Y-previous.Y)
		}
		return sum
	}

	totalBends := func(replacements *mgEdgeReplacementMap) int {
		sum := 0
		for _, edge := range visibleEdges {
			sum = sum + countOrthogonalBends(replacementPointsFor(edge, replacements))
		}
		return sum
	}

	totalLength := func(replacements *mgEdgeReplacementMap) float64 {
		sum := 0.0
		for _, edge := range visibleEdges {
			sum = sum + pathLength(replacementPointsFor(edge, replacements))
		}
		return sum
	}

	// The two functions below read the segments of a path and do not keep
	// them, so each of them has one scratch for all of its calls.
	var conflictScratch, hitScratch segmentScratch

	pathHasSegmentConflict := func(edge *ru.Edge, path []Point, replacements *mgEdgeReplacementMap) bool {
		pathSegments := conflictScratch.of(path)
		for _, other := range visibleEdges {
			if other == edge {
				continue
			}
			otherSegments := segmentsOfEdge(other, replacements)
			for _, candidateSegment := range pathSegments {
				for _, otherSegment := range otherSegments {
					if sameAxisSegmentOverlapLength(candidateSegment, otherSegment, 0.5) >= mgMinShared {
						return true
					}
				}
			}
		}
		return false
	}

	pathHitsNode := func(edge *ru.Edge, path []Point) bool {
		endpointIds := endpointIdsOf(edge.Start, edge.End)
		for _, segment := range hitScratch.of(path) {
			if segmentHitsAnyRect(segment.A, segment.B, realNodeRects, endpointIds, -2) {
				return true
			}
		}
		return false
	}

	pushOrthogonalCandidate := func(candidates *[][]Point, points []Point) {
		candidate := simplifyPolyline(dedupeConsecutivePoints(points))
		if orthogonalSegmentCount(candidate) == len(candidate)-1 {
			*candidates = append(*candidates, candidate)
		}
	}

	sideIsHorizontal := func(side RectSide) bool {
		return side == "left" || side == "right"
	}

	localTrackForSameSide := func(src Point, side RectSide, dst Point) float64 {
		switch side {
		case "left":
			return js.Min(src.X, dst.X) - ANCHOR
		case "right":
			return js.Max(src.X, dst.X) + ANCHOR
		case "top":
			return js.Min(src.Y, dst.Y) - ANCHOR
		default: // "bottom"
			return js.Max(src.Y, dst.Y) + ANCHOR
		}
	}

	addSameSideCandidates := func(candidates *[][]Point, src Point, srcSide RectSide, dst Point) {
		outward := 1.0
		if srcSide == "left" || srcSide == "top" {
			outward = -1
		}
		trackSeeds := []float64{localTrackForSameSide(src, srcSide, dst), outsideTracks[srcSide]}
		for _, seed := range trackSeeds {
			for channel := 0; channel <= EXTRA_CHANNEL_COUNT; channel++ {
				pushOrthogonalCandidate(
					candidates,
					buildSameSideTrackPath(src, srcSide, dst, seed+float64(outward*ANCHOR*float64(channel))),
				)
			}
		}
	}

	addHorizontalToVerticalCandidates := func(candidates *[][]Point, src Point, srcSide RectSide, dst Point, dstSide RectSide) {
		for _, xTrack := range outwardTracksForSide(srcSide) {
			for _, yTrack := range outwardTracksForSide(dstSide) {
				pushOrthogonalCandidate(candidates, []Point{
					src,
					{X: xTrack, Y: src.Y},
					{X: xTrack, Y: yTrack},
					{X: dst.X, Y: yTrack},
					dst,
				})
			}
		}
	}

	addVerticalToHorizontalCandidates := func(candidates *[][]Point, src Point, srcSide RectSide, dst Point, dstSide RectSide) {
		for _, yTrack := range outwardTracksForSide(srcSide) {
			for _, xTrack := range outwardTracksForSide(dstSide) {
				pushOrthogonalCandidate(candidates, []Point{
					src,
					{X: src.X, Y: yTrack},
					{X: xTrack, Y: yTrack},
					{X: xTrack, Y: dst.Y},
					dst,
				})
			}
		}
	}

	addHorizontalPairCandidates := func(candidates *[][]Point, src Point, srcSide RectSide, dst Point, dstSide RectSide) {
		yTracks := append(outwardTracksForSide("top"), outwardTracksForSide("bottom")...)
		for _, srcTrack := range outwardTracksForSide(srcSide) {
			for _, dstTrack := range outwardTracksForSide(dstSide) {
				for _, yTrack := range yTracks {
					pushOrthogonalCandidate(candidates, []Point{
						src,
						{X: srcTrack, Y: src.Y},
						{X: srcTrack, Y: yTrack},
						{X: dstTrack, Y: yTrack},
						{X: dstTrack, Y: dst.Y},
						dst,
					})
				}
			}
		}
	}

	addVerticalPairCandidates := func(candidates *[][]Point, src Point, srcSide RectSide, dst Point, dstSide RectSide) {
		xTracks := append(outwardTracksForSide("left"), outwardTracksForSide("right")...)
		for _, srcTrack := range outwardTracksForSide(srcSide) {
			for _, dstTrack := range outwardTracksForSide(dstSide) {
				for _, xTrack := range xTracks {
					pushOrthogonalCandidate(candidates, []Point{
						src,
						{X: src.X, Y: srcTrack},
						{X: xTrack, Y: srcTrack},
						{X: xTrack, Y: dstTrack},
						{X: dst.X, Y: dstTrack},
						dst,
					})
				}
			}
		}
	}

	dedupeCandidatePaths := func(candidates [][]Point) [][]Point {
		seen := map[string]bool{}
		deduped := make([][]Point, len(candidates))
		for i, candidate := range candidates {
			deduped[i] = dedupeConsecutivePoints(candidate)
		}
		result := [][]Point{}
		for _, candidate := range deduped {
			key := pathKey3(candidate)
			if seen[key] || len(candidate) < 2 {
				continue
			}
			seen[key] = true
			result = append(result, candidate)
		}
		return result
	}

	// sideCandidates has the result of buildCandidatesForSides by its
	// arguments. The candidates between two ports depend on the ports and on
	// the nodes, which do not move here. Each iteration below asks for them
	// again for every edge, so they are built one time. The result is
	// shared: a caller reads it, and copies a path that becomes the points
	// of an edge.
	type sideCandidateKey struct {
		srcX, srcY uint64
		srcSide    RectSide
		dstX, dstY uint64
		dstSide    RectSide
	}
	sideCandidates := map[sideCandidateKey][][]Point{}

	buildCandidatesForSides := func(src Point, srcSide RectSide, dst Point, dstSide RectSide) [][]Point {
		// The bits of the coordinates: -0 and 0 are different arguments.
		key := sideCandidateKey{
			srcX: math.Float64bits(src.X), srcY: math.Float64bits(src.Y), srcSide: srcSide,
			dstX: math.Float64bits(dst.X), dstY: math.Float64bits(dst.Y), dstSide: dstSide,
		}
		if kept, ok := sideCandidates[key]; ok {
			return kept
		}
		candidates := [][]Point{}
		base := buildOrthogonalPortPath(src, srcSide, dst, dstSide, ANCHOR, mgEpsLocal)
		if base != nil {
			pushOrthogonalCandidate(&candidates, base)
		}
		if srcSide == dstSide {
			addSameSideCandidates(&candidates, src, srcSide, dst)
		}

		srcHorizontal := sideIsHorizontal(srcSide)
		dstHorizontal := sideIsHorizontal(dstSide)
		if srcHorizontal && !dstHorizontal {
			addHorizontalToVerticalCandidates(&candidates, src, srcSide, dst, dstSide)
		} else if !srcHorizontal && dstHorizontal {
			addVerticalToHorizontalCandidates(&candidates, src, srcSide, dst, dstSide)
		} else if srcHorizontal {
			addHorizontalPairCandidates(&candidates, src, srcSide, dst, dstSide)
		} else {
			addVerticalPairCandidates(&candidates, src, srcSide, dst, dstSide)
		}

		result := dedupeCandidatePaths(candidates)
		sideCandidates[key] = result
		return result
	}

	addVerticalDepartureOuterTrackCandidates := func(candidates *[][]Point, first, departure Point, dstNode *NodeInfo) {
		externalXTracks := append(outwardTracksForSide("left"), outwardTracksForSide("right")...)
		externalYTracks := append(outwardTracksForSide("top"), outwardTracksForSide("bottom")...)
		for _, side := range sides {
			dst := portForRectSide(dstNode.cx, dstNode.cy, dstNode.rect, side)
			targetYTracks := externalYTracks
			if side == "top" || side == "bottom" {
				targetYTracks = outwardTracksForSide(side)
			}
			for _, track := range externalXTracks {
				pushOrthogonalCandidate(candidates, []Point{
					first,
					departure,
					{X: track, Y: departure.Y},
					{X: track, Y: dst.Y},
					dst,
				})
				for _, targetTrack := range targetYTracks {
					pushOrthogonalCandidate(candidates, []Point{
						first,
						departure,
						{X: track, Y: departure.Y},
						{X: track, Y: targetTrack},
						{X: dst.X, Y: targetTrack},
						dst,
					})
				}
			}
		}
	}

	addHorizontalDepartureOuterTrackCandidates := func(candidates *[][]Point, first, departure Point, dstNode *NodeInfo) {
		externalXTracks := append(outwardTracksForSide("left"), outwardTracksForSide("right")...)
		externalYTracks := append(outwardTracksForSide("top"), outwardTracksForSide("bottom")...)
		for _, side := range sides {
			dst := portForRectSide(dstNode.cx, dstNode.cy, dstNode.rect, side)
			targetXTracks := externalXTracks
			if side == "left" || side == "right" {
				targetXTracks = outwardTracksForSide(side)
			}
			for _, track := range externalYTracks {
				pushOrthogonalCandidate(candidates, []Point{
					first,
					departure,
					{X: departure.X, Y: track},
					{X: dst.X, Y: track},
					dst,
				})
				for _, targetTrack := range targetXTracks {
					pushOrthogonalCandidate(candidates, []Point{
						first,
						departure,
						{X: departure.X, Y: track},
						{X: targetTrack, Y: track},
						{X: targetTrack, Y: dst.Y},
						dst,
					})
				}
			}
		}
	}

	// departureCandidates has the result of
	// terminalPreservingOuterTrackCandidates by the values that it reads.
	type departureCandidateKey struct {
		firstX, firstY         uint64
		departureX, departureY uint64
		dstNode                *NodeInfo
	}
	departureCandidates := map[departureCandidateKey][][]Point{}

	terminalPreservingOuterTrackCandidates := func(edge *ru.Edge) [][]Point {
		srcId := edge.Start
		var dstNode *NodeInfo
		if edge.End != "" {
			dstNode = nodeInfoById[edge.End]
		}
		if srcId == "" || dstNode == nil {
			return nil
		}

		points := dedupeConsecutivePoints(edge.Points)
		if len(points) < 4 {
			return nil
		}

		first := points[0]
		departure := points[1]
		// The candidates depend on the two points and on the node, as for
		// sideCandidates.
		key := departureCandidateKey{
			firstX: math.Float64bits(first.X), firstY: math.Float64bits(first.Y),
			departureX: math.Float64bits(departure.X), departureY: math.Float64bits(departure.Y),
			dstNode: dstNode,
		}
		if kept, ok := departureCandidates[key]; ok {
			return kept
		}
		candidates := [][]Point{}
		if isVerticalSegment(first, departure, mgEpsLocal) {
			addVerticalDepartureOuterTrackCandidates(&candidates, first, departure, dstNode)
		} else if isHorizontalSegment(first, departure, mgEpsLocal) {
			addHorizontalDepartureOuterTrackCandidates(&candidates, first, departure, dstNode)
		}

		departureCandidates[key] = candidates
		return candidates
	}

	nodeCandidates := map[[2]*NodeInfo][][]Point{}

	candidatePathsFor := func(edge *ru.Edge) [][]Point {
		var srcNode, dstNode *NodeInfo
		if edge.Start != "" {
			srcNode = nodeInfoById[edge.Start]
		}
		if edge.End != "" {
			dstNode = nodeInfoById[edge.End]
		}
		if srcNode == nil || dstNode == nil {
			return nil
		}

		// The candidates of all pairs of sides are the same for every edge
		// between the two nodes, so nodeCandidates has them.
		nodes := [2]*NodeInfo{srcNode, dstNode}
		between, ok := nodeCandidates[nodes]
		if !ok {
			between = [][]Point{}
			for _, srcSide := range sides {
				srcPort := portForRectSide(srcNode.cx, srcNode.cy, srcNode.rect, srcSide)
				for _, dstSide := range sides {
					between = append(
						between,
						buildCandidatesForSides(srcPort, srcSide, portForRectSide(dstNode.cx, dstNode.cy, dstNode.rect, dstSide), dstSide)...,
					)
				}
			}
			nodeCandidates[nodes] = between
		}
		preserving := terminalPreservingOuterTrackCandidates(edge)
		candidates := make([][]Point, 0, len(between)+len(preserving))
		candidates = append(candidates, between...)
		candidates = append(candidates, preserving...)
		return candidates
	}

	currentSegmentsByEdge := func() map[*ru.Edge][]*OrthogonalSegment {
		result := map[*ru.Edge][]*OrthogonalSegment{}
		for _, edge := range visibleEdges {
			result[edge] = currentSegmentsOf(edge)
		}
		return result
	}

	// baseSegmentsOf is `baseSegments.get(other) ??
	// segmentsFor(replacementPointsFor(other))`.
	baseSegmentsOf := func(baseSegments map[*ru.Edge][]*OrthogonalSegment, other *ru.Edge) []*OrthogonalSegment {
		if segments, ok := baseSegments[other]; ok {
			return segments
		}
		return currentSegmentsOf(other)
	}

	sharedTrackConflictsFor := func(edge *ru.Edge, candidateSegments []*OrthogonalSegment, baseSegments map[*ru.Edge][]*OrthogonalSegment) *js.Set[*ru.Edge] {
		conflicts := js.NewSet[*ru.Edge]()
		for _, other := range visibleEdges {
			if other == edge {
				continue
			}
			otherSegments := baseSegmentsOf(baseSegments, other)
			shares := false
		search:
			for _, candidateSegment := range candidateSegments {
				for _, otherSegment := range otherSegments {
					if sameAxisSegmentOverlapLength(candidateSegment, otherSegment, 0.5) >= mgMinShared {
						shares = true
						break search
					}
				}
			}
			if shares {
				conflicts.Add(other)
			}
		}
		return conflicts
	}

	pairCandidatesFor := func(edge *ru.Edge, current *CrossingSnapshot, baseSegments map[*ru.Edge][]*OrthogonalSegment, crossingCountByEdge map[*ru.Edge]int) []*PairCandidate {
		type scoredCandidate struct {
			candidate         []Point
			candidateSegments []*OrthogonalSegment
			crossings         int
			bends             int
			totalBends        int
			length            float64
		}

		seen := map[string]bool{}
		paths := candidatePathsFor(edge)
		simplified := make([][]Point, len(paths))
		for i, candidate := range paths {
			simplified[i] = simplifyPolyline(dedupeConsecutivePoints(candidate))
		}
		kept := [][]Point{}
		for _, candidate := range simplified {
			if pathHitsNode(edge, candidate) {
				continue
			}
			key := pathKey3(candidate)
			if seen[key] || len(candidate) < 2 {
				continue
			}
			seen[key] = true
			kept = append(kept, candidate)
		}
		candidates := []*scoredCandidate{}
		for _, candidate := range kept {
			candidateSegments := mgSegmentsFor(candidate)
			replacementAffected := 0
			for _, other := range visibleEdges {
				if other == edge {
					continue
				}
				replacementAffected += crossingCountBetweenSegments(
					candidateSegments,
					baseSegmentsOf(baseSegments, other),
				)
			}
			scored := &scoredCandidate{
				candidate:         candidate,
				candidateSegments: candidateSegments,
				crossings:         current.count - crossingCountByEdge[edge] + replacementAffected,
				bends:             countOrthogonalBends(candidate, mgEpsLocal),
				totalBends:        countOrthogonalBends(candidate),
				length:            pathLength(candidate),
			}
			if scored.crossings <= current.count {
				candidates = append(candidates, scored)
			}
		}
		sortFunc(candidates, func(a, b *scoredCandidate) float64 {
			if d := a.crossings - b.crossings; d != 0 {
				return float64(d)
			}
			if d := a.bends - b.bends; d != 0 {
				return float64(d)
			}
			return a.length - b.length
		})
		if len(candidates) > MAX_PAIR_CANDIDATES_PER_EDGE {
			candidates = candidates[:MAX_PAIR_CANDIDATES_PER_EDGE]
		}
		result := make([]*PairCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			result = append(result, &PairCandidate{
				path:     candidate.candidate,
				segments: candidate.candidateSegments,
				sharedTrackConflicts: sharedTrackConflictsFor(
					edge,
					candidate.candidateSegments,
					baseSegments,
				),
				totalBends: candidate.totalBends,
				length:     candidate.length,
			})
		}
		return result
	}

	pairCrossingCount := func(current *CrossingSnapshot, firstEdge *ru.Edge, firstCandidate *PairCandidate, secondEdge *ru.Edge, secondCandidate *PairCandidate, baseSegments map[*ru.Edge][]*OrthogonalSegment) int {
		currentAffected := 0
		for _, pair := range current.pairs {
			if pair.first == firstEdge ||
				pair.second == firstEdge ||
				pair.first == secondEdge ||
				pair.second == secondEdge {
				currentAffected += pair.count
			}
		}

		replacementAffected := crossingCountBetweenSegments(
			firstCandidate.segments,
			secondCandidate.segments,
		)
		for _, other := range visibleEdges {
			if other == firstEdge || other == secondEdge {
				continue
			}
			otherSegments := baseSegmentsOf(baseSegments, other)
			replacementAffected += crossingCountBetweenSegments(firstCandidate.segments, otherSegments) +
				crossingCountBetweenSegments(secondCandidate.segments, otherSegments)
		}

		return current.count - currentAffected + replacementAffected
	}

	conflictsOnlyWith := func(candidate *PairCandidate, edge *ru.Edge) bool {
		for _, conflict := range candidate.sharedTrackConflicts.Values() {
			if conflict != edge {
				return false
			}
		}
		return true
	}

	candidatesShareTrack := func(firstCandidate, secondCandidate *PairCandidate) bool {
		for _, firstSegment := range firstCandidate.segments {
			for _, secondSegment := range secondCandidate.segments {
				if sameAxisSegmentOverlapLength(firstSegment, secondSegment, 0.5) >= mgMinShared {
					return true
				}
			}
		}
		return false
	}

	pairCandidatesAreCompatible := func(first *PairOption, firstCandidate *PairCandidate, second *PairOption, secondCandidate *PairCandidate) bool {
		return conflictsOnlyWith(firstCandidate, second.edge) &&
			conflictsOnlyWith(secondCandidate, first.edge) &&
			!candidatesShareTrack(firstCandidate, secondCandidate)
	}

	// scorePairReplacement returns nil for undefined.
	scorePairReplacement := func(context *PairScoringContext, first *PairOption, firstCandidate *PairCandidate, second *PairOption, secondCandidate *PairCandidate) *PairReplacementScore {
		crossings := pairCrossingCount(
			context.current,
			first.edge,
			firstCandidate,
			second.edge,
			secondCandidate,
			context.baseSegments,
		)
		if crossings >= context.current.count {
			return nil
		}

		replacements := js.NewMap[*ru.Edge, []Point]()
		replacements.Set(first.edge, firstCandidate.path)
		replacements.Set(second.edge, secondCandidate.path)
		return &PairReplacementScore{
			replacements: replacements,
			crossings:    crossings,
			bends: context.currentBends -
				context.baseBendsByEdge[first.edge] -
				context.baseBendsByEdge[second.edge] +
				firstCandidate.totalBends +
				secondCandidate.totalBends,
			length: context.currentLength -
				context.baseLengthByEdge[first.edge] -
				context.baseLengthByEdge[second.edge] +
				firstCandidate.length +
				secondCandidate.length,
		}
	}

	pairScoreIsBetter := func(candidate, best *PairReplacementScore) bool {
		return candidate.crossings < best.crossings ||
			(candidate.crossings == best.crossings &&
				(candidate.bends < best.bends ||
					(candidate.bends == best.bends && candidate.length < best.length)))
	}

	bestScoreForOptionPair := func(context *PairScoringContext, first, second *PairOption, best *PairReplacementScore) *PairReplacementScore {
		pairBest := best
		for _, firstCandidate := range first.candidates {
			for _, secondCandidate := range second.candidates {
				if !pairCandidatesAreCompatible(first, firstCandidate, second, secondCandidate) {
					continue
				}
				score := scorePairReplacement(context, first, firstCandidate, second, secondCandidate)
				if score != nil && pairScoreIsBetter(score, pairBest) {
					pairBest = score
				}
			}
		}
		return pairBest
	}

	// bestPairedReplacement returns nil for undefined.
	bestPairedReplacement := func(current *CrossingSnapshot) *mgEdgeReplacementMap {
		currentBends := totalBends(nil)
		currentLength := totalLength(nil)
		baseSegments := currentSegmentsByEdge()
		crossingCountByEdge := currentCrossingsByEdge(current)
		baseBendsByEdge := map[*ru.Edge]int{}
		for _, edge := range visibleEdges {
			baseBendsByEdge[edge] = countOrthogonalBends(replacementPointsFor(edge, nil))
		}
		baseLengthByEdge := map[*ru.Edge]float64{}
		for _, edge := range visibleEdges {
			baseLengthByEdge[edge] = pathLength(replacementPointsFor(edge, nil))
		}
		optionsByEdge := map[*ru.Edge]*PairOption{}
		groups := pairSearchGroups(current)
		for _, group := range groups {
			for _, edge := range group {
				if _, ok := optionsByEdge[edge]; ok {
					continue
				}
				candidates := pairCandidatesFor(edge, current, baseSegments, crossingCountByEdge)
				if len(candidates) > 0 {
					optionsByEdge[edge] = &PairOption{edge: edge, candidates: candidates}
				}
			}
		}

		best := &PairReplacementScore{
			replacements: js.NewMap[*ru.Edge, []Point](),
			crossings:    current.count,
			bends:        currentBends,
			length:       currentLength,
		}
		scoringContext := &PairScoringContext{
			current:          current,
			currentBends:     currentBends,
			currentLength:    currentLength,
			baseBendsByEdge:  baseBendsByEdge,
			baseLengthByEdge: baseLengthByEdge,
			baseSegments:     baseSegments,
		}

		for _, group := range groups {
			crossingEdgeSet := map[*ru.Edge]bool{}
			for _, edge := range group {
				if current.edgeSet[edge] {
					crossingEdgeSet[edge] = true
				}
			}
			options := []*PairOption{}
			for _, edge := range group {
				if option := optionsByEdge[edge]; option != nil {
					options = append(options, option)
				}
			}
			for i := 0; i < len(options); i++ {
				first := options[i]
				for j := i + 1; j < len(options); j++ {
					second := options[j]
					if !crossingEdgeSet[first.edge] && !crossingEdgeSet[second.edge] {
						continue
					}
					best = bestScoreForOptionPair(scoringContext, first, second, best)
				}
			}
		}

		if best.replacements.Len() > 0 {
			return best.replacements
		}
		return nil
	}

	for iteration := 0; iteration < MAX_ITERATIONS; iteration++ {
		// The iteration before this one ended with new points of an edge.
		clear(segmentCache)
		current := crossingSnapshot(nil)
		currentCrossings := current.count
		if currentCrossings == 0 {
			return
		}

		var bestEdge *ru.Edge
		var bestPath []Point
		bestCrossings := currentCrossings
		// bestBends starts at Number.POSITIVE_INFINITY.
		bestBends := math.Inf(1)

		for _, edge := range current.edges {
			currentEdgeBends := countOrthogonalBends(replacementPointsFor(edge, nil), mgEpsLocal)
			for _, candidate := range candidatePathsFor(edge) {
				candidateHitsNode := pathHitsNode(edge, candidate)
				candidateHasSegmentConflict := !candidateHitsNode && pathHasSegmentConflict(edge, candidate, nil)
				// Upstream computes the crossings and the bends of the candidate
				// before this test. They are pure, so a rejected candidate
				// needs neither.
				if candidateHitsNode || candidateHasSegmentConflict {
					continue
				}
				candidateCrossings := crossingCountWithSingleReplacement(current, edge, candidate)
				candidateBends := countOrthogonalBends(candidate, mgEpsLocal)
				improvesCurrentEdge := candidateCrossings < currentCrossings ||
					(candidateCrossings == currentCrossings && candidateBends < currentEdgeBends)
				if !improvesCurrentEdge {
					continue
				}
				if candidateCrossings > bestCrossings ||
					(candidateCrossings == bestCrossings && float64(candidateBends) >= bestBends) {
					continue
				}
				bestEdge = edge
				bestPath = candidate
				bestCrossings = candidateCrossings
				bestBends = float64(candidateBends)
			}
		}

		// bestPath is set together with bestEdge. It can be a path of
		// sideCandidates, which stays as it is: the edge gets a copy.
		if bestEdge != nil {
			bestEdge.Points = clonePoints(bestPath)
			continue
		}

		pairedReplacement := bestPairedReplacement(current)
		if pairedReplacement == nil {
			return
		}
		for _, edge := range pairedReplacement.Keys() {
			edge.Points = pairedReplacement.Get(edge)
		}
	}
}
