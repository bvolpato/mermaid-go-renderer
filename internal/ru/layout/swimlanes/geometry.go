// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/geometry.ts (MIT).

package swimlanes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// geometryEPS is EPS of geometry.ts: the default epsilon of its functions.
const geometryEPS = 1e-3

// epsOf is the value of a default parameter `epsilon = EPS`.
func epsOf(epsilon []float64) float64 {
	if len(epsilon) > 0 {
		return epsilon[0]
	}
	return geometryEPS
}

// RectSide is "top", "bottom", "left" or "right".
type RectSide = string

// RectBounds is a rectangle given by its edges.
type RectBounds struct {
	Left, Right, Top, Bottom float64
}

// RectEntry is the rectangle of a node.
type RectEntry struct {
	ID   string
	Rect RectBounds
}

// NodeBoundsInfo is the rectangle of a node with its center.
type NodeBoundsInfo struct {
	ID   string
	Rect RectBounds
	Cx   float64
	Cy   float64
}

// NodePairGeometry is the geometry of the two nodes of an edge.
type NodePairGeometry struct {
	SrcId      string
	DstId      string
	SrcInfo    *NodeBoundsInfo
	DstInfo    *NodeBoundsInfo
	CollinearX bool
	CollinearY bool
}

// LayoutNodeRect is a rectangle with the id of its node.
type LayoutNodeRect struct {
	RectBounds
	NodeId string
}

// ThreeSegmentRoute is a route of four points: HVH or VHV.
type ThreeSegmentRoute struct {
	Kind           string
	P0, P1, P2, P3 Point
}

// OrthogonalSegment is a horizontal or vertical segment of a polyline.
type OrthogonalSegment struct {
	Index      int
	A, B       Point
	Horizontal bool
	Vertical   bool
}

type measuredRect struct {
	cx, cy float64
	rect   RectBounds
}

func measuredNodeRect(node *ru.Node) (measuredRect, bool) {
	cx := ru.NN(node.X, 0)
	cy := ru.NN(node.Y, 0)
	width := ru.NN(node.Width, 0)
	height := ru.NN(node.Height, 0)
	if width > 0 && height > 0 {
		return measuredRect{cx: cx, cy: cy, rect: rectFromCenterSize(cx, cy, width, height)}, true
	}
	return measuredRect{}, false
}

func nodeBoundsInfoFor(node *ru.Node) *NodeBoundsInfo {
	if node.IsGroup {
		return nil
	}
	measured, ok := measuredNodeRect(node)
	if !ok {
		return nil
	}
	return &NodeBoundsInfo{
		ID:   node.ID,
		Cx:   measured.cx,
		Cy:   measured.cy,
		Rect: measured.rect,
	}
}

func samePoint(a, b Point, epsilon ...float64) bool {
	e := epsOf(epsilon)
	return math.Abs(a.X-b.X) < e && math.Abs(a.Y-b.Y) < e
}

func sameX(a, b Point, epsilon ...float64) bool {
	return math.Abs(a.X-b.X) < epsOf(epsilon)
}

func sameY(a, b Point, epsilon ...float64) bool {
	return math.Abs(a.Y-b.Y) < epsOf(epsilon)
}

func isHorizontalSegment(a, b Point, epsilon ...float64) bool {
	e := epsOf(epsilon)
	return sameY(a, b, e) && math.Abs(a.X-b.X) > e
}

func isVerticalSegment(a, b Point, epsilon ...float64) bool {
	e := epsOf(epsilon)
	return sameX(a, b, e) && math.Abs(a.Y-b.Y) > e
}

func overlapLength(a1, a2, b1, b2 float64) float64 {
	return js.Max(
		0,
		js.Min(js.Max(a1, a2), js.Max(b1, b2))-js.Max(js.Min(a1, a2), js.Min(b1, b2)),
	)
}

func sameAxisSegmentOverlapLength(a, b *OrthogonalSegment, epsilon ...float64) float64 {
	e := epsOf(epsilon)
	if a.Horizontal && b.Horizontal && sameY(a.A, b.A, e) {
		return overlapLength(a.A.X, a.B.X, b.A.X, b.B.X)
	}
	if a.Vertical && b.Vertical && sameX(a.A, b.A, e) {
		return overlapLength(a.A.Y, a.B.Y, b.A.Y, b.B.Y)
	}
	return 0
}

// orthogonalSegmentsForPoints returns the horizontal and vertical segments
// of a polyline. Each segment is a new object, as upstream: some passes
// compare segments by identity.
func orthogonalSegmentsForPoints(points []Point, epsilon ...float64) []*OrthogonalSegment {
	n := max(len(points)-1, 0)
	// One allocation holds every segment. It never grows, so the pointers
	// stay valid.
	_, result := appendOrthogonalSegments(make([]OrthogonalSegment, 0, n), make([]*OrthogonalSegment, 0, n), points, epsOf(epsilon))
	return result
}

// appendOrthogonalSegments adds the segments of the points to segments, and
// a pointer to each of them to result. segments must have room for one
// segment for each pair of points, so that it does not move.
func appendOrthogonalSegments(segments []OrthogonalSegment, result []*OrthogonalSegment, points []Point, e float64) ([]OrthogonalSegment, []*OrthogonalSegment) {
	for i := 0; i < len(points)-1; i++ {
		a := points[i]
		b := points[i+1]
		horizontal := isHorizontalSegment(a, b, e)
		vertical := isVerticalSegment(a, b, e)
		if horizontal || vertical {
			segments = append(segments, OrthogonalSegment{Index: i, A: a, B: b, Horizontal: horizontal, Vertical: vertical})
			result = append(result, &segments[len(segments)-1])
		}
	}
	return segments, result
}

// orthogonalSegmentCount is orthogonalSegmentsForPoints(points).length. It
// does not make the list.
func orthogonalSegmentCount(points []Point, epsilon ...float64) int {
	e := epsOf(epsilon)
	count := 0
	for i := 0; i < len(points)-1; i++ {
		if isHorizontalSegment(points[i], points[i+1], e) || isVerticalSegment(points[i], points[i+1], e) {
			count++
		}
	}
	return count
}

// segmentScratch has the segments of one path at a time. It is for a caller
// that reads the segments of a path and drops them before it asks for the
// segments of the next path: the next call writes over them.
type segmentScratch struct {
	segments []OrthogonalSegment
	result   []*OrthogonalSegment
}

// of is orthogonalSegmentsForPoints(points), in the memory of the scratch.
func (s *segmentScratch) of(points []Point) []*OrthogonalSegment {
	if n := max(len(points)-1, 0); cap(s.segments) < n {
		s.segments = make([]OrthogonalSegment, 0, n)
		s.result = make([]*OrthogonalSegment, 0, n)
	}
	s.segments, s.result = appendOrthogonalSegments(s.segments[:0], s.result[:0], points, geometryEPS)
	return s.result
}

func countOrthogonalBends(points []Point, epsilon ...float64) int {
	segments := orthogonalSegmentsForPoints(points, epsOf(epsilon))
	bends := 0
	for i := 1; i < len(segments); i++ {
		if segments[i-1].Horizontal != segments[i].Horizontal {
			bends++
		}
	}
	return bends
}

// dedupeConsecutivePoints returns a new slice without the points that are
// equal to the point before them.
func dedupeConsecutivePoints(points []Point, epsilon ...float64) []Point {
	e := epsOf(epsilon)
	result := make([]Point, 0, len(points))
	for _, point := range points {
		if len(result) == 0 || !samePoint(result[len(result)-1], point, e) {
			result = append(result, Point{X: point.X, Y: point.Y})
		}
	}
	return result
}

// classifyThreeSegmentRoute returns nil when the points are not a route of
// three segments.
func classifyThreeSegmentRoute(points []Point, epsilon ...float64) *ThreeSegmentRoute {
	e := epsOf(epsilon)
	if len(points) != 4 {
		return nil
	}
	p0, p1, p2, p3 := points[0], points[1], points[2], points[3]
	isHVH := isHorizontalSegment(p0, p1, e) &&
		isVerticalSegment(p1, p2, e) &&
		isHorizontalSegment(p2, p3, e)
	if isHVH {
		return &ThreeSegmentRoute{Kind: "HVH", P0: p0, P1: p1, P2: p2, P3: p3}
	}
	isVHV := isVerticalSegment(p0, p1, e) &&
		isHorizontalSegment(p1, p2, e) &&
		isVerticalSegment(p2, p3, e)
	if isVHV {
		return &ThreeSegmentRoute{Kind: "VHV", P0: p0, P1: p1, P2: p2, P3: p3}
	}
	return nil
}

// segmentBoundsOverlapRect has the default parameter `buffer = 0`.
func segmentBoundsOverlapRect(a, b Point, rect RectBounds, buffer ...float64) bool {
	buf := 0.0
	if len(buffer) > 0 {
		buf = buffer[0]
	}
	segMinX := js.Min(a.X, b.X)
	segMaxX := js.Max(a.X, b.X)
	segMinY := js.Min(a.Y, b.Y)
	segMaxY := js.Max(a.Y, b.Y)
	return segMaxX > rect.Left-buf &&
		segMinX < rect.Right+buf &&
		segMaxY > rect.Top-buf &&
		segMinY < rect.Bottom+buf
}

// pointInsideRect has the default parameter `buffer = 0`.
func pointInsideRect(point Point, rect RectBounds, buffer ...float64) bool {
	buf := 0.0
	if len(buffer) > 0 {
		buf = buffer[0]
	}
	return point.X > rect.Left+buf &&
		point.X < rect.Right-buf &&
		point.Y > rect.Top+buf &&
		point.Y < rect.Bottom-buf
}

func rectContainsRect(outer, inner RectBounds) bool {
	return outer.Left <= inner.Left &&
		outer.Right >= inner.Right &&
		outer.Top <= inner.Top &&
		outer.Bottom >= inner.Bottom
}

func rectsOverlap(a, b RectBounds) bool {
	return a.Left < b.Right && a.Right > b.Left && a.Top < b.Bottom && a.Bottom > b.Top
}

func inflateRect(rect RectBounds, margin float64) RectBounds {
	return RectBounds{
		Left:   rect.Left - margin,
		Right:  rect.Right + margin,
		Top:    rect.Top - margin,
		Bottom: rect.Bottom + margin,
	}
}

func rectFromCenterSize(cx, cy, width, height float64) RectBounds {
	return RectBounds{
		Left:   cx - float64(width/2),
		Right:  cx + float64(width/2),
		Top:    cy - float64(height/2),
		Bottom: cy + float64(height/2),
	}
}

// rectOfNodeBounds returns false when the node has no measured size.
func rectOfNodeBounds(node *ru.Node) (RectBounds, bool) {
	measured, ok := measuredNodeRect(node)
	return measured.rect, ok
}

// portForRectSide is portForRectSide(node, side) for an object with cx, cy
// and rect.
func portForRectSide(cx, cy float64, rect RectBounds, side RectSide) Point {
	switch side {
	case "top":
		return Point{X: cx, Y: rect.Top}
	case "bottom":
		return Point{X: cx, Y: rect.Bottom}
	case "left":
		return Point{X: rect.Left, Y: cy}
	default: // "right"
		return Point{X: rect.Right, Y: cy}
	}
}

// buildOrthogonalPortPath returns nil for undefined.
func buildOrthogonalPortPath(src Point, srcSide RectSide, dst Point, dstSide RectSide, anchor float64, epsilon ...float64) []Point {
	e := epsOf(epsilon)
	srcH := srcSide == "left" || srcSide == "right"
	dstH := dstSide == "left" || dstSide == "right"

	if srcH && dstH {
		opposingDir := (srcSide == "right" && dstSide == "left" && src.X < dst.X) ||
			(srcSide == "left" && dstSide == "right" && src.X > dst.X)
		if opposingDir {
			if sameY(src, dst, e) {
				return []Point{src, dst}
			}
			midX := (src.X + dst.X) / 2
			return []Point{src, {X: midX, Y: src.Y}, {X: midX, Y: dst.Y}, dst}
		}
		if srcSide == dstSide {
			if sameY(src, dst, e) {
				return nil
			}
			var intX float64
			if srcSide == "left" {
				intX = js.Min(src.X, dst.X) - anchor
			} else {
				intX = js.Max(src.X, dst.X) + anchor
			}
			return []Point{src, {X: intX, Y: src.Y}, {X: intX, Y: dst.Y}, dst}
		}
		return nil
	}

	if !srcH && !dstH {
		if srcSide == dstSide {
			if sameX(src, dst, e) {
				return nil
			}
			var intY float64
			if srcSide == "top" {
				intY = js.Min(src.Y, dst.Y) - anchor
			} else {
				intY = js.Max(src.Y, dst.Y) + anchor
			}
			return []Point{src, {X: src.X, Y: intY}, {X: dst.X, Y: intY}, dst}
		}
		sameDir := (srcSide == "bottom" && dstSide == "top" && src.Y < dst.Y) ||
			(srcSide == "top" && dstSide == "bottom" && src.Y > dst.Y)
		if !sameDir {
			return nil
		}
		if sameX(src, dst, e) {
			return []Point{src, dst}
		}
		midY := (src.Y + dst.Y) / 2
		return []Point{src, {X: src.X, Y: midY}, {X: dst.X, Y: midY}, dst}
	}

	if srcH && !dstH {
		sameDirSrc := (srcSide == "right" && dst.X > src.X) || (srcSide == "left" && dst.X < src.X)
		sameDirDst := (dstSide == "top" && src.Y < dst.Y) || (dstSide == "bottom" && src.Y > dst.Y)
		if sameDirSrc && sameDirDst {
			return []Point{src, {X: dst.X, Y: src.Y}, dst}
		}
		return nil
	}

	sameDirSrc := (srcSide == "bottom" && dst.Y > src.Y) || (srcSide == "top" && dst.Y < src.Y)
	sameDirDst := (dstSide == "left" && src.X < dst.X) || (dstSide == "right" && src.X > dst.X)
	if sameDirSrc && sameDirDst {
		return []Point{src, {X: src.X, Y: dst.Y}, dst}
	}
	return nil
}

func buildSameSideTrackPath(src Point, side RectSide, dst Point, track float64) []Point {
	if side == "left" || side == "right" {
		return []Point{src, {X: track, Y: src.Y}, {X: track, Y: dst.Y}, dst}
	}
	return []Point{src, {X: src.X, Y: track}, {X: dst.X, Y: track}, dst}
}

// collectRealNodeBounds returns the bounds of the nodes that are neither
// groups nor edge labels, by id and as a list in the order of the nodes.
func collectRealNodeBounds(nodes []*ru.Node) (nodeInfoById map[string]*NodeBoundsInfo, realNodeRects []RectEntry) {
	nodeInfoById = map[string]*NodeBoundsInfo{}
	realNodeRects = []RectEntry{}
	for _, node := range nodes {
		if node.IsEdgeLabel {
			continue
		}
		info := nodeBoundsInfoFor(node)
		if info == nil {
			continue
		}
		nodeInfoById[info.ID] = info
		realNodeRects = append(realNodeRects, RectEntry{ID: info.ID, Rect: info.Rect})
	}
	return nodeInfoById, realNodeRects
}

// collectNodeRectEntries returns the rectangles of the real nodes and of
// the edge label nodes.
func collectNodeRectEntries(nodes []*ru.Node) (realNodeRects, labelNodeRects []RectEntry) {
	realNodeRects = []RectEntry{}
	labelNodeRects = []RectEntry{}
	for _, node := range nodes {
		info := nodeBoundsInfoFor(node)
		if info == nil {
			continue
		}
		entry := RectEntry{ID: info.ID, Rect: info.Rect}
		if node.IsEdgeLabel {
			labelNodeRects = append(labelNodeRects, entry)
		} else {
			realNodeRects = append(realNodeRects, entry)
		}
	}
	return realNodeRects, labelNodeRects
}

// collectLayoutNodeRects has the option includeEdgeLabels, which defaults
// to true.
func collectLayoutNodeRects(nodes []*ru.Node, includeEdgeLabels ...bool) []LayoutNodeRect {
	include := true
	if len(includeEdgeLabels) > 0 {
		include = includeEdgeLabels[0]
	}
	result := []LayoutNodeRect{}
	for _, node := range nodes {
		if node.IsGroup || (!include && node.IsEdgeLabel) {
			continue
		}
		cx := ru.NN(node.X, 0)
		cy := ru.NN(node.Y, 0)
		width := ru.NN(node.Width, 0)
		height := ru.NN(node.Height, 0)
		result = append(result, LayoutNodeRect{
			NodeId:     node.ID,
			RectBounds: rectFromCenterSize(cx, cy, width, height),
		})
	}
	return result
}

// getNodePairGeometry returns nil for undefined.
func getNodePairGeometry(edge *ru.Edge, nodeInfoById map[string]*NodeBoundsInfo, epsilon ...float64) *NodePairGeometry {
	e := epsOf(epsilon)
	srcId := edge.Start
	dstId := edge.End
	if srcId == "" || dstId == "" {
		return nil
	}
	srcInfo := nodeInfoById[srcId]
	dstInfo := nodeInfoById[dstId]
	if srcInfo == nil || dstInfo == nil {
		return nil
	}
	return &NodePairGeometry{
		SrcId:      srcId,
		DstId:      dstId,
		SrcInfo:    srcInfo,
		DstInfo:    dstInfo,
		CollinearX: math.Abs(srcInfo.Cx-dstInfo.Cx) < e,
		CollinearY: math.Abs(srcInfo.Cy-dstInfo.Cy) < e,
	}
}

// segmentHitsAnyRect is segmentHitsAnyRect(a, b, rects, excludeIds, shrink).
// The defaults of upstream are excludeIds = [] and shrink = 0.
func segmentHitsAnyRect(a, b Point, rects []RectEntry, excludeIds []string, shrink float64) bool {
	for _, entry := range rects {
		if containsString(excludeIds, entry.ID) {
			continue
		}
		if segmentBoundsOverlapRect(a, b, entry.Rect, -shrink) {
			return true
		}
	}
	return false
}

// orthogonalSegmentsCross has the default parameters `epsilon = EPS` and
// `endpointTolerance = 1e-6`, in this order.
func orthogonalSegmentsCross(a1, b1, a2, b2 Point, opts ...float64) bool {
	epsilon := geometryEPS
	endpointTolerance := 1e-6
	if len(opts) > 0 {
		epsilon = opts[0]
	}
	if len(opts) > 1 {
		endpointTolerance = opts[1]
	}
	s1H := sameY(a1, b1, epsilon)
	s1V := sameX(a1, b1, epsilon)
	s2H := sameY(a2, b2, epsilon)
	s2V := sameX(a2, b2, epsilon)
	if (s1H && s2H) || (s1V && s2V) {
		return false
	}
	if !(s1H || s1V) || !(s2H || s2V) {
		return false
	}

	horizA, horizB := a2, b2
	if s1H {
		horizA, horizB = a1, b1
	}
	vertA, vertB := a2, b2
	if s1V {
		vertA, vertB = a1, b1
	}
	hY := horizA.Y
	hX1 := js.Min(horizA.X, horizB.X)
	hX2 := js.Max(horizA.X, horizB.X)
	vX := vertA.X
	vY1 := js.Min(vertA.Y, vertB.Y)
	vY2 := js.Max(vertA.Y, vertB.Y)
	if vX < hX1 || vX > hX2 || hY < vY1 || hY > vY2 {
		return false
	}

	matchesHorizEndpoint := (math.Abs(vX-horizA.X) < endpointTolerance &&
		math.Abs(hY-horizA.Y) < endpointTolerance) ||
		(math.Abs(vX-horizB.X) < endpointTolerance && math.Abs(hY-horizB.Y) < endpointTolerance)
	matchesVertEndpoint := (math.Abs(vX-vertA.X) < endpointTolerance && math.Abs(hY-vertA.Y) < endpointTolerance) ||
		(math.Abs(vX-vertB.X) < endpointTolerance && math.Abs(hY-vertB.Y) < endpointTolerance)
	return !(matchesHorizEndpoint && matchesVertEndpoint)
}

func sameAxisSegmentsOverlap(a1, b1, a2, b2 Point, epsilon ...float64) bool {
	e := epsOf(epsilon)
	s1H := sameY(a1, b1, e)
	s1V := sameX(a1, b1, e)
	s2H := sameY(a2, b2, e)
	s2V := sameX(a2, b2, e)
	if s1V && s2V && sameX(a1, a2, e) {
		return overlapLength(a1.Y, b1.Y, a2.Y, b2.Y) > e
	}
	if s1H && s2H && sameY(a1, a2, e) {
		return overlapLength(a1.X, b1.X, a2.X, b2.X) > e
	}
	return false
}

// segmentConflictsWithAnyEdge is segmentConflictsWithAnyEdge(a, b, edges,
// excludeEdge, { epsilon, skipDegenerateOther }). The defaults of upstream
// are epsilon = EPS and skipDegenerateOther = false.
func segmentConflictsWithAnyEdge(a, b Point, edges []*ru.Edge, excludeEdge *ru.Edge, epsilon float64, skipDegenerateOther bool) bool {
	for _, other := range edges {
		if other == excludeEdge || other.IsLayoutOnly {
			continue
		}
		points := other.Points
		if len(points) < 2 {
			continue
		}
		for i := 0; i < len(points)-1; i++ {
			oa := points[i]
			ob := points[i+1]
			if skipDegenerateOther && samePoint(oa, ob, epsilon) {
				continue
			}
			if orthogonalSegmentsCross(a, b, oa, ob, epsilon) ||
				sameAxisSegmentsOverlap(a, b, oa, ob, epsilon) {
				return true
			}
		}
	}
	return false
}

func orthogonalSegmentsStrictlyCross(a1, b1, a2, b2 Point, epsilon ...float64) bool {
	e := epsOf(epsilon)
	aHoriz := sameY(a1, b1, e)
	aVert := sameX(a1, b1, e)
	bHoriz := sameY(a2, b2, e)
	bVert := sameX(a2, b2, e)
	if !((aHoriz && bVert) || (aVert && bHoriz)) {
		return false
	}

	horizA, horizB := a2, b2
	vertA, vertB := a1, b1
	if aHoriz {
		horizA, horizB = a1, b1
		vertA, vertB = a2, b2
	}
	hY := horizA.Y
	hXmin := js.Min(horizA.X, horizB.X)
	hXmax := js.Max(horizA.X, horizB.X)
	vX := vertA.X
	vYmin := js.Min(vertA.Y, vertB.Y)
	vYmax := js.Max(vertA.Y, vertB.Y)
	return vX > hXmin+e && vX < hXmax-e && hY > vYmin+e && hY < vYmax-e
}

func strictlyBetween(value, a, b float64) bool {
	lo := js.Min(a, b)
	hi := js.Max(a, b)
	return value > lo+geometryEPS && value < hi-geometryEPS
}

func isCollinearIntermediate(prev, cur, next Point) bool {
	if sameX(prev, cur) && sameX(cur, next) {
		return strictlyBetween(cur.Y, prev.Y, next.Y)
	}

	if sameY(prev, cur) && sameY(cur, next) {
		return strictlyBetween(cur.X, prev.X, next.X)
	}

	return false
}

func simplifyPolylineOnce(points []Point) (out []Point, changed bool) {
	// The result has no more points than the input.
	out = make([]Point, 0, len(points))

	for i := 0; i < len(points); i++ {
		cur := points[i]
		if len(out) > 0 && i+1 < len(points) {
			prev := out[len(out)-1]
			next := points[i+1]
			if samePoint(prev, next) {
				i++
				changed = true
				continue
			}

			if isCollinearIntermediate(prev, cur, next) {
				changed = true
				continue
			}
		}
		out = append(out, cur)
	}

	// The result has no spare room, so an append to it always makes a copy.
	return out[:len(out):len(out)], changed
}

// orthogonalizePolyline inserts orthogonal L-bends and removes consecutive
// duplicate points. pts must not be empty.
func orthogonalizePolyline(pts []Point) []Point {
	cleaned := []Point{pts[0]}
	for i := 1; i < len(pts); i++ {
		prev := cleaned[len(cleaned)-1]
		curr := pts[i]
		if !sameX(prev, curr) && !sameY(prev, curr) {
			incomingVertical := false
			if len(cleaned) >= 2 {
				prevPrev := cleaned[len(cleaned)-2]
				incomingVertical = sameX(prevPrev, prev)
			}
			corner := Point{X: curr.X, Y: prev.Y}
			if incomingVertical {
				corner = Point{X: prev.X, Y: curr.Y}
			}
			cleaned = append(cleaned, corner)
		}
		cleaned = append(cleaned, curr)
	}
	deduped := []Point{}
	for _, p := range cleaned {
		if len(deduped) == 0 || !samePoint(deduped[len(deduped)-1], p) {
			deduped = append(deduped, p)
		}
	}
	return deduped
}

// simplifyPolyline returns pts itself when it has fewer than three points,
// and a new slice otherwise.
func simplifyPolyline(pts []Point) []Point {
	if len(pts) < 3 {
		return pts
	}
	// Upstream starts with a copy of pts. simplifyPolylineOnce only reads its
	// input and returns a new slice, so the copy is not necessary.
	work := pts
	for guard := 0; guard < 32; guard++ {
		result, changed := simplifyPolylineOnce(work)
		work = result
		if !changed {
			break
		}
	}
	return work
}
