// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/endpointClip.ts (MIT).

package swimlanes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

const (
	clipEPS             = 1e-3
	clipInsideEPS       = 0.5
	clipCornerClearance = 4
)

// BorderSide is "top", "bottom", "left" or "right".
type BorderSide = string

// clipRange is a range { lo, hi }.
type clipRange struct {
	lo, hi float64
}

// endpointContext is the result of endpointContextFor. hasSrcRect and
// hasDstRect are false when the rect is undefined.
type endpointContext struct {
	edge       *ru.Edge
	points     []Point
	srcRect    RectBounds
	hasSrcRect bool
	dstRect    RectBounds
	hasDstRect bool
}

// endpointContextFor returns nil for undefined.
func endpointContextFor(edge *ru.Edge, nodeByIdMap *js.Map[string, *ru.Node], minPoints int) *endpointContext {
	if edge.IsLayoutOnly || len(edge.Points) < minPoints {
		return nil
	}
	context := &endpointContext{edge: edge, points: edge.Points}
	if edge.Start != "" {
		if src := nodeByIdMap.Get(edge.Start); src != nil {
			context.srcRect, context.hasSrcRect = rectOfNodeBounds(src)
		}
	}
	if edge.End != "" {
		if dst := nodeByIdMap.Get(edge.End); dst != nil {
			context.dstRect, context.hasDstRect = rectOfNodeBounds(dst)
		}
	}
	return context
}

// segmentEnterPoint returns, for an axis-aligned segment from outside a
// rect to inside it, the point where the segment enters the rect boundary.
func segmentEnterPoint(outside, inside Point, r RectBounds) Point {
	if sameY(outside, inside, clipEPS) {
		x := r.Right
		if outside.X < r.Left {
			x = r.Left
		}
		return Point{X: x, Y: outside.Y}
	}
	if sameX(outside, inside, clipEPS) {
		y := r.Bottom
		if outside.Y < r.Top {
			y = r.Top
		}
		return Point{X: outside.X, Y: y}
	}
	return Point{
		X: js.Min(r.Right, js.Max(r.Left, outside.X)),
		Y: js.Min(r.Bottom, js.Max(r.Top, outside.Y)),
	}
}

func clipEndpoint(points []Point, rect RectBounds, atStart bool) []Point {
	step := -1
	outsideIndex := len(points) - 1
	if atStart {
		step = 1
		outsideIndex = 0
	}
	for outsideIndex >= 0 &&
		outsideIndex < len(points) &&
		pointInsideRect(points[outsideIndex], rect, clipInsideEPS) {
		outsideIndex += step
	}
	if outsideIndex < 0 || outsideIndex >= len(points) {
		return points
	}

	insideIndex := outsideIndex - step
	if insideIndex < 0 || insideIndex >= len(points) {
		return points
	}

	entry := segmentEnterPoint(points[outsideIndex], points[insideIndex], rect)
	if atStart {
		return cat([]Point{entry}, points[outsideIndex:])
	}
	return cat(points[:outsideIndex+1], []Point{entry})
}

func clipEdgeEndpointsToNodeBoundaries(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	for _, edge := range edges {
		context := endpointContextFor(edge, nodeByIdMap, 2)
		if context == nil {
			continue
		}

		next := clonePoints(context.points)
		if context.hasSrcRect {
			next = clipEndpoint(next, context.srcRect, true)
		}
		if context.hasDstRect {
			next = clipEndpoint(next, context.dstRect, false)
		}
		next = simplifyPolyline(orthogonalizePolyline(next))
		next = clearStraightEndpointCornerConnections(next, context)
		context.edge.Points = simplifyPolyline(orthogonalizePolyline(next))
	}
}

// snapEndpointToBoundary has the default parameter `useApproachSide =
// false`.
func snapEndpointToBoundary(inner, endpoint Point, r RectBounds, useApproachSide bool) Point {
	if sameY(inner, endpoint, clipEPS) {
		if endpoint.Y < r.Top-clipEPS || endpoint.Y > r.Bottom+clipEPS {
			return endpoint
		}
		if useApproachSide {
			if inner.X < r.Left-clipEPS {
				return Point{X: r.Left, Y: inner.Y}
			}
			if inner.X > r.Right+clipEPS {
				return Point{X: r.Right, Y: inner.Y}
			}
		}
		toLeft := math.Abs(endpoint.X-r.Left) <= math.Abs(endpoint.X-r.Right)
		if toLeft {
			return Point{X: r.Left, Y: inner.Y}
		}
		return Point{X: r.Right, Y: inner.Y}
	}
	if sameX(inner, endpoint, clipEPS) {
		if endpoint.X < r.Left-clipEPS || endpoint.X > r.Right+clipEPS {
			return endpoint
		}
		if useApproachSide {
			if inner.Y < r.Top-clipEPS {
				return Point{X: inner.X, Y: r.Top}
			}
			if inner.Y > r.Bottom+clipEPS {
				return Point{X: inner.X, Y: r.Bottom}
			}
		}
		toTop := math.Abs(endpoint.Y-r.Top) <= math.Abs(endpoint.Y-r.Bottom)
		if toTop {
			return Point{X: inner.X, Y: r.Top}
		}
		return Point{X: inner.X, Y: r.Bottom}
	}
	return endpoint
}

// firstDistinctAdjacent returns false for undefined.
func firstDistinctAdjacent(points []Point, endpointIndex int, step int) (Point, bool) {
	endpoint := points[endpointIndex]
	for index := endpointIndex + step; index >= 0 && index < len(points); index += step {
		candidate := points[index]
		if !samePoint(candidate, endpoint, clipEPS) {
			return candidate, true
		}
	}
	if at := endpointIndex + step; at >= 0 && at < len(points) {
		return points[at], true
	}
	return Point{}, false
}

func cornerClearanceRange(min, max float64) clipRange {
	lo := min + clipCornerClearance
	hi := max - clipCornerClearance
	if lo <= hi {
		return clipRange{lo: lo, hi: hi}
	}
	return clipRange{lo: (min + max) / 2, hi: (min + max) / 2}
}

func clampToCornerClearance(value, min, max float64) float64 {
	r := cornerClearanceRange(min, max)
	return js.Min(r.hi, js.Max(r.lo, value))
}

// intersectRanges returns false for undefined. ranges is not empty.
func intersectRanges(ranges []clipRange) (clipRange, bool) {
	los := make([]float64, len(ranges))
	his := make([]float64, len(ranges))
	for i, r := range ranges {
		los[i] = r.lo
		his[i] = r.hi
	}
	lo := js.Max(los...)
	hi := js.Min(his...)
	if lo > hi {
		return clipRange{}, false
	}
	return clipRange{lo: lo, hi: hi}, true
}

func clearanceRangeForSide(r RectBounds, side BorderSide) clipRange {
	if side == "left" || side == "right" {
		return cornerClearanceRange(r.Top, r.Bottom)
	}
	return cornerClearanceRange(r.Left, r.Right)
}

// terminalSideForSegment returns "" for undefined.
func terminalSideForSegment(endpoint, adjacent Point, r RectBounds) BorderSide {
	yWithin := endpoint.Y >= r.Top-clipEPS && endpoint.Y <= r.Bottom+clipEPS
	xWithin := endpoint.X >= r.Left-clipEPS && endpoint.X <= r.Right+clipEPS
	if sameY(endpoint, adjacent, clipEPS) && yWithin {
		if math.Abs(endpoint.X-r.Left) < clipEPS {
			return "left"
		}
		if math.Abs(endpoint.X-r.Right) < clipEPS {
			return "right"
		}
	}
	if sameX(endpoint, adjacent, clipEPS) && xWithin {
		if math.Abs(endpoint.Y-r.Top) < clipEPS {
			return "top"
		}
		if math.Abs(endpoint.Y-r.Bottom) < clipEPS {
			return "bottom"
		}
	}
	return ""
}

func isHorizontalSide(side BorderSide) bool {
	return side == "left" || side == "right"
}

// straightClearanceRange returns false for undefined.
func straightClearanceRange(start, end Point, context *endpointContext, horizontal bool) (clipRange, bool) {
	ranges := []clipRange{}
	srcSide := ""
	if context.hasSrcRect {
		srcSide = terminalSideForSegment(start, end, context.srcRect)
	}
	dstSide := ""
	if context.hasDstRect {
		dstSide = terminalSideForSegment(end, start, context.dstRect)
	}

	if context.hasSrcRect && srcSide != "" && isHorizontalSide(srcSide) == horizontal {
		ranges = append(ranges, clearanceRangeForSide(context.srcRect, srcSide))
	}
	if context.hasDstRect && dstSide != "" && isHorizontalSide(dstSide) == horizontal {
		ranges = append(ranges, clearanceRangeForSide(context.dstRect, dstSide))
	}

	if len(ranges) > 0 {
		return intersectRanges(ranges)
	}
	return clipRange{}, false
}

// clearStraightEndpointCornerAxis returns nil for undefined.
func clearStraightEndpointCornerAxis(start, end Point, context *endpointContext, horizontal bool) []Point {
	r, ok := straightClearanceRange(start, end, context, horizontal)
	if !ok {
		return nil
	}

	current := start.X
	if horizontal {
		current = start.Y
	}
	next := js.Min(r.hi, js.Max(r.lo, current))
	if math.Abs(next-current) < clipEPS {
		return nil
	}

	if horizontal {
		return []Point{
			{X: start.X, Y: next},
			{X: end.X, Y: next},
		}
	}
	return []Point{
		{X: next, Y: start.Y},
		{X: next, Y: end.Y},
	}
}

// clearStraightEndpointCornerConnections takes the rects of the two nodes
// from context. It returns points itself, or a new list of two points when
// points has two points.
func clearStraightEndpointCornerConnections(points []Point, context *endpointContext) []Point {
	if len(points) != 2 {
		return points
	}

	start, end := points[0], points[1]
	if sameY(start, end, clipEPS) {
		if cleared := clearStraightEndpointCornerAxis(start, end, context, true); cleared != nil {
			return cleared
		}
		return points
	}

	if sameX(start, end, clipEPS) {
		if cleared := clearStraightEndpointCornerAxis(start, end, context, false); cleared != nil {
			return cleared
		}
		return points
	}

	return points
}

func cornerClearedEndpoint(endpoint Point, r RectBounds, side BorderSide) Point {
	if isHorizontalSide(side) {
		return Point{X: endpoint.X, Y: clampToCornerClearance(endpoint.Y, r.Top, r.Bottom)}
	}
	return Point{X: clampToCornerClearance(endpoint.X, r.Left, r.Right), Y: endpoint.Y}
}

func moveCollinearEndpointRun(points []Point, endpointIndex int, step int, endpoint, adjusted Point, horizontalTerminal bool) []Point {
	next := clonePoints(points)
	for index := endpointIndex; index >= 0 && index < len(points); index += step {
		point := points[index]
		if horizontalTerminal && !sameY(point, endpoint, clipEPS) {
			break
		}
		if !horizontalTerminal && !sameX(point, endpoint, clipEPS) {
			break
		}
		if horizontalTerminal {
			next[index].Y = adjusted.Y
		} else {
			next[index].X = adjusted.X
		}
	}
	return next
}

func clearEndpointCornerConnection(points []Point, r RectBounds, atStart bool) []Point {
	if len(points) < 2 {
		return points
	}

	endpointIndex := len(points) - 1
	step := -1
	if atStart {
		endpointIndex = 0
		step = 1
	}
	endpoint := points[endpointIndex]
	adjacent, ok := firstDistinctAdjacent(points, endpointIndex, step)
	if !ok {
		return points
	}

	side := terminalSideForSegment(endpoint, adjacent, r)
	if side == "" {
		return points
	}

	horizontalTerminal := isHorizontalSide(side)
	adjusted := cornerClearedEndpoint(endpoint, r, side)
	if samePoint(endpoint, adjusted, clipEPS) {
		return points
	}

	return moveCollinearEndpointRun(
		points,
		endpointIndex,
		step,
		endpoint,
		adjusted,
		horizontalTerminal,
	)
}

// borderSideForSegment returns "" for undefined.
func borderSideForSegment(a, b Point, r RectBounds) BorderSide {
	xWithin := js.Min(a.X, b.X) >= r.Left-clipEPS && js.Max(a.X, b.X) <= r.Right+clipEPS
	yWithin := js.Min(a.Y, b.Y) >= r.Top-clipEPS && js.Max(a.Y, b.Y) <= r.Bottom+clipEPS
	if math.Abs(a.Y-r.Top) < clipEPS && math.Abs(b.Y-r.Top) < clipEPS && xWithin {
		return "top"
	}
	if math.Abs(a.Y-r.Bottom) < clipEPS && math.Abs(b.Y-r.Bottom) < clipEPS && xWithin {
		return "bottom"
	}
	if math.Abs(a.X-r.Left) < clipEPS && math.Abs(b.X-r.Left) < clipEPS && yWithin {
		return "left"
	}
	if math.Abs(a.X-r.Right) < clipEPS && math.Abs(b.X-r.Right) < clipEPS && yWithin {
		return "right"
	}
	return ""
}

func leavesOutward(side BorderSide, from, to Point, r RectBounds) bool {
	switch side {
	case "top":
		return sameX(from, to, clipEPS) && to.Y < r.Top-clipEPS
	case "bottom":
		return sameX(from, to, clipEPS) && to.Y > r.Bottom+clipEPS
	case "left":
		return sameY(from, to, clipEPS) && to.X < r.Left-clipEPS
	default: // "right"
		return sameY(from, to, clipEPS) && to.X > r.Right+clipEPS
	}
}

func collapseOwnBorderStub(points []Point, r RectBounds, atStart bool) []Point {
	if len(points) < 3 {
		return points
	}
	if atStart {
		side := borderSideForSegment(points[0], points[1], r)
		if side != "" && leavesOutward(side, points[1], points[2], r) {
			return clonePoints(points[1:])
		}
		return points
	}

	last := len(points) - 1
	side := borderSideForSegment(points[last-1], points[last], r)
	if side != "" && leavesOutward(side, points[last-1], points[last-2], r) {
		return clonePoints(points[:last])
	}
	return points
}

func snapAndCollapseEndpoints(points []Point, context *endpointContext) []Point {
	next := points
	if context.hasSrcRect {
		if adjacent, ok := firstDistinctAdjacent(next, 0, 1); ok {
			// Upstream replaces the endpoint only when the snap returns
			// another object. The same object has the same value.
			snapped := snapEndpointToBoundary(adjacent, next[0], context.srcRect, false)
			next = cat([]Point{snapped}, next[1:])
		}
		next = collapseOwnBorderStub(next, context.srcRect, true)
	}
	if context.hasDstRect {
		last := len(next) - 1
		if adjacent, ok := firstDistinctAdjacent(next, last, -1); ok {
			snapped := snapEndpointToBoundary(adjacent, next[last], context.dstRect, true)
			next = cat(next[:last], []Point{snapped})
		}
		next = collapseOwnBorderStub(next, context.dstRect, false)
	}

	// clearStraightEndpointCornerConnections returns another list only for
	// a list of two points, so `straightCleared !== next || next.length ===
	// 2` is this length check.
	straightCleared := clearStraightEndpointCornerConnections(next, context)
	if len(next) == 2 {
		return straightCleared
	}

	if context.hasSrcRect {
		next = clearEndpointCornerConnection(next, context.srcRect, true)
	}
	if context.hasDstRect {
		next = clearEndpointCornerConnection(next, context.dstRect, false)
	}
	return next
}

// prepareEdgeEndpointsForRenderer snaps the ends of every edge to the
// border of its nodes and doubles the first and the last point, which the
// edge renderer expects.
func prepareEdgeEndpointsForRenderer(edges []*ru.Edge, nodeByIdMap *js.Map[string, *ru.Node]) {
	for _, edge := range edges {
		context := endpointContextFor(edge, nodeByIdMap, 2)
		if context == nil {
			continue
		}

		input := dedupeConsecutivePoints(context.points, clipEPS)
		newPts := snapAndCollapseEndpoints(input, context)
		if len(newPts) < 3 {
			context.edge.Points = newPts
			continue
		}
		last := len(newPts) - 1
		duplicated := cat(
			[]Point{newPts[0], newPts[0]},
			newPts[1:last],
			[]Point{newPts[last], newPts[last]},
		)
		context.edge.Points = duplicated
	}
}
