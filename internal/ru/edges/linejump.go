// Ported from mermaid 12.2.0, rendering-util/rendering-elements/lineJump.ts
// (MIT).

package edges

import (
	"encoding/base64"
	"math"
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// Line jumps ("hops") for edge crossings. The code detects true segment
// crossings between edge polylines and rewrites the SVG path of one of the
// edges, so that the crossing renders as a small arc (jumpStyle "arc") or
// as a visible break (jumpStyle "gap").

// roundedCornerRadius is the radius used by generateRoundedPath of
// edges.js. It is kept in sync so that rewritten paths look like the
// originals at bends.
const roundedCornerRadius = 5

// cornerEpsilon: skip the jump if its clamped radius falls below this.
const cornerEpsilon = 1e-5

// cornerJumpClearance is the straight run kept between a hop and the bend
// next to it.
const cornerJumpClearance = 2

// minUsefulRadiusRatio is the smallest share of the requested radius a hop
// may shrink to before it is dropped instead of drawn.
const minUsefulRadiusRatio = 0.6

// EdgeGeom is EdgeGeom of lineJump.ts: the geometry of one edge.
type EdgeGeom struct {
	ID     string
	Points []ru.Point
	// Curve is the curve hint (edge.curve). An empty string stands for
	// undefined.
	Curve string
	// ArrowTypeStart and ArrowTypeEnd give the marker offsets of the ends.
	ArrowTypeStart string
	ArrowTypeEnd   string
}

// LineJumpConfig is LineJumpConfig of lineJump.ts.
type LineJumpConfig struct {
	Enabled    bool
	JumpRadius float64
	// JumpStyle is "arc" or "gap".
	JumpStyle string
}

// Crossing is Crossing of lineJump.ts: one place where an edge hops
// another.
type Crossing struct {
	JumpEdgeID  string
	OtherEdgeID string
	// SegIndex is the index of the segment in the polyline of the jumping
	// edge.
	SegIndex int
	// T is the position of the crossing along that segment, 0..1.
	T     float64
	Point ru.Point
}

const endpointEpsilon = 1e-6

type segment struct {
	a, b ru.Point
}

func buildSegmentList(points []ru.Point) []segment {
	segments := []segment{}
	for i := 0; i < len(points)-1; i++ {
		segments = append(segments, segment{a: points[i], b: points[i+1]})
	}
	return segments
}

type segmentHit struct {
	point  ru.Point
	tA, tB float64
}

// segmentIntersection is the parametric segment-segment intersection. ok
// is false if the segments are parallel, do not intersect, or only meet at
// one of their endpoints (within endpointEpsilon).
func segmentIntersection(a1, a2, b1, b2 ru.Point) (hit segmentHit, ok bool) {
	dxA := a2.X - a1.X
	dyA := a2.Y - a1.Y
	dxB := b2.X - b1.X
	dyB := b2.Y - b1.Y

	denom := float64(dxA*dyB) - float64(dyA*dxB)
	if denom == 0 {
		return hit, false
	}

	dx := b1.X - a1.X
	dy := b1.Y - a1.Y

	tA := (float64(dx*dyB) - float64(dy*dxB)) / denom
	tB := (float64(dx*dyA) - float64(dy*dxA)) / denom

	if tA <= endpointEpsilon ||
		tA >= 1-endpointEpsilon ||
		tB <= endpointEpsilon ||
		tB >= 1-endpointEpsilon {
		return hit, false
	}
	// A NaN parameter passes the comparisons above, as it does upstream.

	return segmentHit{
		point: ru.Point{X: a1.X + float64(tA*dxA), Y: a1.Y + float64(tA*dyA)},
		tA:    tA,
		tB:    tB,
	}, true
}

// isHorizontalSeg reports whether the segment is horizontally dominant
// (abs(dx) is at least abs(dy)).
func isHorizontalSeg(seg segment) bool {
	return math.Abs(seg.b.X-seg.a.X) >= math.Abs(seg.b.Y-seg.a.Y)
}

// crossingSitsInRoundedCorner reports whether a crossing on the segment
// segIndex of edge at parameter t falls inside the stretch where the drawn
// stroke has left the polyline to round a bend. Only rounded edges have
// such a stretch.
func crossingSitsInRoundedCorner(edge *EdgeGeom, segIndex int, t float64) bool {
	if edge.Curve != "rounded" {
		return false
	}
	pts := edge.Points
	if segIndex < 0 || segIndex+1 >= len(pts) {
		return false
	}
	a := pts[segIndex]
	b := pts[segIndex+1]
	segLen := js.Hypot(b.X-a.X, b.Y-a.Y)
	d := float64(t * segLen)

	if segIndex > 0 {
		if entering, ok := computeRoundedCorner(pts[segIndex-1], a, b, roundedCornerRadius); ok && d < entering.cutLen {
			return true
		}
	}

	if segIndex+2 < len(pts) {
		if leaving, ok := computeRoundedCorner(a, b, pts[segIndex+2], roundedCornerRadius); ok {
			return segLen-d < leaving.cutLen
		}
	}
	return false
}

// FindEdgeIntersections is findEdgeIntersections(edges): the true segment
// crossings between the polylines of the edges, in upstream's order.
func FindEdgeIntersections(edges []EdgeGeom) []Crossing {
	crossings := []Crossing{}

	for i := 0; i < len(edges); i++ {
		edgeA := &edges[i]
		segmentsA := buildSegmentList(edgeA.Points)
		for j := i + 1; j < len(edges); j++ {
			edgeB := &edges[j]
			segmentsB := buildSegmentList(edgeB.Points)

			for si, segA := range segmentsA {
				for sj, segB := range segmentsB {
					hit, ok := segmentIntersection(segA.a, segA.b, segB.a, segB.b)
					if !ok {
						continue
					}

					// Either edge rounding a bend here means the polyline is not where
					// the stroke is, so there is nothing trustworthy to hop over.
					if crossingSitsInRoundedCorner(edgeA, si, hit.tA) ||
						crossingSitsInRoundedCorner(edgeB, sj, hit.tB) {
						continue
					}

					// Orthogonal-orientation rule: when one segment is horizontal-
					// dominant and the other vertical-dominant, the HORIZONTAL one
					// gets the jump. Falls back to later-index-wins when both
					// segments share an orientation.
					aHoriz := isHorizontalSeg(segA)
					bHoriz := isHorizontalSeg(segB)
					orthogonalPair := aHoriz != bHoriz
					jumpOnA := false
					if orthogonalPair {
						jumpOnA = aHoriz
					}

					if jumpOnA {
						crossings = append(crossings, Crossing{
							JumpEdgeID:  edgeA.ID,
							OtherEdgeID: edgeB.ID,
							SegIndex:    si,
							T:           hit.tA,
							Point:       hit.point,
						})
					} else {
						crossings = append(crossings, Crossing{
							JumpEdgeID:  edgeB.ID,
							OtherEdgeID: edgeA.ID,
							SegIndex:    sj,
							T:           hit.tB,
							Point:       hit.point,
						})
					}
				}
			}
		}
	}

	return crossings
}

// fmtNum is fmt(n): the number with at most 3 decimals.
func fmtNum(n float64) string {
	rounded := js.Round(n*1000) / 1000
	return js.Str(rounded)
}

func pointToString(p ru.Point) string {
	return fmtNum(p.X) + "," + fmtNum(p.Y)
}

// getArcSweepFlag determines the SVG arc sweep flag so that the jump bumps
// in the conventional direction: horizontal segments bump up, vertical
// segments bump right.
func getArcSweepFlag(seg segment) int {
	dx := seg.b.X - seg.a.X
	dy := seg.b.Y - seg.a.Y
	if math.Abs(dx) >= math.Abs(dy) {
		// Horizontal-dominant: bump up (smaller y in SVG's y-down frame).
		if dx >= 0 {
			return 1
		}
		return 0
	}
	// Vertical-dominant: bump right (positive x).
	if dy >= 0 {
		return 1
	}
	return 0
}

type jumpOnSegment struct {
	t     float64
	point ru.Point
	// d is the distance from the segment start along the segment direction.
	d float64
	// r is the effective radius after boundary and adjacency clamping.
	r float64
}

// applyMarkerOffsets shifts the first and the last point inward along the
// edge direction by the amount required for their arrow markers, matching
// applyMarkerOffsetsToPoints of edges.js.
func applyMarkerOffsets(points []ru.Point, edge *EdgeGeom) []ru.Point {
	out := append([]ru.Point{}, points...)
	if len(points) < 2 {
		return out
	}
	if startOff, ok := markerOffset(edge.ArrowTypeStart); ok {
		a := points[0]
		b := points[1]
		ang := atan2(b.Y-a.Y, b.X-a.X)
		out[0].X = a.X + float64(startOff*cos(ang))
		out[0].Y = a.Y + float64(startOff*sin(ang))
	}
	if endOff, ok := markerOffset(edge.ArrowTypeEnd); ok {
		n := len(points)
		a := points[n-2]
		b := points[n-1]
		ang := atan2(b.Y-a.Y, b.X-a.X)
		out[n-1].X = b.X - float64(endOff*cos(ang))
		out[n-1].Y = b.Y - float64(endOff*sin(ang))
	}
	return out
}

// emitJump emits the arc or gap command for a crossing, in the direction
// of the segment. It returns the part strings; the caller inserts them in
// order.
func emitJump(jump *jumpOnSegment, ux, uy float64, sweep int, style string) []string {
	cx := jump.point.X
	cy := jump.point.Y
	pre := ru.Point{X: cx - float64(ux*jump.r), Y: cy - float64(uy*jump.r)}
	post := ru.Point{X: cx + float64(ux*jump.r), Y: cy + float64(uy*jump.r)}
	out := []string{"L" + pointToString(pre)}
	if style == "arc" {
		out = append(out, "A"+fmtNum(jump.r)+","+fmtNum(jump.r)+" 0 0 "+js.Str(float64(sweep))+" "+pointToString(post))
	} else {
		out = append(out, "M"+pointToString(post))
	}
	return out
}

// roundedCorner mirrors the corner-rounding logic of generateRoundedPath
// of edges.js.
type roundedCorner struct {
	startX, startY float64
	endX, endY     float64
	ctrlX, ctrlY   float64
	// cutLen is how much the start of the rounded corner eats into the
	// incoming segment.
	cutLen float64
}

// computeRoundedCorner computes the rounded corner of the bend at curr
// between the segments prev-curr and curr-next. ok is false if the angle
// is degenerate, and the caller then emits a straight line to curr.
func computeRoundedCorner(prev, curr, next ru.Point, radius float64) (corner roundedCorner, ok bool) {
	dx1 := curr.X - prev.X
	dy1 := curr.Y - prev.Y
	dx2 := next.X - curr.X
	dy2 := next.Y - curr.Y
	len1 := js.Hypot(dx1, dy1)
	len2 := js.Hypot(dx2, dy2)
	if len1 < cornerEpsilon || len2 < cornerEpsilon {
		return corner, false
	}
	nx1 := dx1 / len1
	ny1 := dy1 / len1
	nx2 := dx2 / len2
	ny2 := dy2 / len2
	dot := float64(nx1*nx2) + float64(ny1*ny2)
	clamped := js.Max(-1, js.Min(1, dot))
	angle := acos(clamped)
	if angle < cornerEpsilon || math.Abs(math.Pi-angle) < cornerEpsilon {
		return corner, false
	}
	cutLen := js.Min(radius/sin(angle/2), len1/2, len2/2)
	return roundedCorner{
		startX: curr.X - float64(nx1*cutLen),
		startY: curr.Y - float64(ny1*cutLen),
		endX:   curr.X + float64(nx2*cutLen),
		endY:   curr.Y + float64(ny2*cutLen),
		ctrlX:  curr.X,
		ctrlY:  curr.Y,
		cutLen: cutLen,
	}, true
}

// pointOr is points[i] ?? points[j].
func pointOr(points []ru.Point, i, j int) ru.Point {
	if i < len(points) {
		return points[i]
	}
	return points[j]
}

func rewriteEdgePath(edge *EdgeGeom, jumps []Crossing, config LineJumpConfig) string {
	rawPoints := edge.Points
	if len(rawPoints) < 2 {
		return ""
	}

	// Match edges.js: shift the first/last point inward so arrow markers line up.
	points := applyMarkerOffsets(rawPoints, edge)
	rounded := edge.Curve == "rounded"

	// Jumps are indexed into the ORIGINAL (un-offset) segment list.
	segments := buildSegmentList(points)
	bySeg := map[int][]*jumpOnSegment{}
	for _, j := range jumps {
		if j.SegIndex < 0 || j.SegIndex >= len(segments) {
			continue
		}
		seg := segments[j.SegIndex]
		segLen := js.Hypot(seg.b.X-seg.a.X, seg.b.Y-seg.a.Y)
		bySeg[j.SegIndex] = append(bySeg[j.SegIndex], &jumpOnSegment{
			t:     j.T,
			point: j.Point,
			d:     float64(j.T * segLen),
			r:     config.JumpRadius,
		})
	}

	parts := []string{"M" + pointToString(points[0])}
	// Running cursor along the current segment measured from seg.a.
	for i := 0; i < len(segments); i++ {
		seg := segments[i]
		segLen := js.Hypot(seg.b.X-seg.a.X, seg.b.Y-seg.a.Y)
		ux, uy := 0.0, 0.0
		if segLen != 0 {
			ux = (seg.b.X - seg.a.X) / segLen
			uy = (seg.b.Y - seg.a.Y) / segLen
		}
		sweep := getArcSweepFlag(seg)

		// How much of the front of this segment was consumed by the previous
		// corner's Q end-point (endX,endY). Default 0.
		segStartConsumed := 0.0
		if rounded && i > 0 {
			if corner, ok := computeRoundedCorner(points[i-1], points[i], pointOr(points, i+1, i), roundedCornerRadius); ok {
				segStartConsumed = corner.cutLen
			}
		}

		// Rounded: if there's a next corner ahead, we stop short of it by cutLen.
		segEndStop := segLen
		var upcomingCorner roundedCorner
		hasUpcomingCorner := false
		if rounded && i < len(segments)-1 {
			upcomingCorner, hasUpcomingCorner = computeRoundedCorner(points[i], points[i+1], pointOr(points, i+2, i+1), roundedCornerRadius)
			if hasUpcomingCorner {
				segEndStop = segLen - upcomingCorner.cutLen
			}
		}

		// Clamp each jump to the room between the bends at either end of the
		// segment, then drop the ones with too little room to be worth drawing.
		// Dropping happens BEFORE the adjacency pass below so that a hop being
		// squeezed out by a corner does not also shrink its neighbours.
		minUsefulRadius := config.JumpRadius * minUsefulRadiusRatio
		sorted := append([]*jumpOnSegment{}, bySeg[i]...)
		js.SortFunc(sorted, func(a, b *jumpOnSegment) float64 { return a.t - b.t })
		segJumps := []*jumpOnSegment{}
		for _, j := range sorted {
			room := js.Min(j.d-segStartConsumed, segEndStop-j.d) - cornerJumpClearance
			j.r = js.Min(j.r, room)
			if j.r >= minUsefulRadius {
				segJumps = append(segJumps, j)
			}
		}
		for k := 0; k < len(segJumps)-1; k++ {
			gap := segJumps[k+1].d - segJumps[k].d
			if segJumps[k].r+segJumps[k+1].r > gap {
				half := gap / 2
				segJumps[k].r = js.Min(segJumps[k].r, half)
				segJumps[k+1].r = js.Min(segJumps[k+1].r, half)
			}
		}

		for _, j := range segJumps {
			// Checked AGAIN after the adjacency pass, not only before it.
			if j.r < minUsefulRadius {
				continue
			}
			parts = append(parts, emitJump(j, ux, uy, sweep, config.JumpStyle)...)
		}

		// End of segment: either a straight L to seg.b (last segment or linear),
		// or a Q-corner into seg.b's neighborhood (rounded, middle).
		if rounded && hasUpcomingCorner {
			parts = append(parts, "L"+fmtNum(upcomingCorner.startX)+","+fmtNum(upcomingCorner.startY))
			parts = append(parts,
				"Q"+fmtNum(upcomingCorner.ctrlX)+","+fmtNum(upcomingCorner.ctrlY)+" "+fmtNum(upcomingCorner.endX)+","+fmtNum(upcomingCorner.endY))
		} else {
			parts = append(parts, "L"+pointToString(seg.b))
		}
	}

	return strings.Join(parts, " ")
}

func plainPath(points []ru.Point) string {
	if len(points) == 0 {
		return ""
	}
	parts := []string{"M" + pointToString(points[0])}
	for i := 1; i < len(points); i++ {
		parts = append(parts, "L"+pointToString(points[i]))
	}
	return strings.Join(parts, " ")
}

// jumpsByEdge groups the crossings by the id of the jumping edge, each
// group in the order of the crossings.
func jumpsByEdge(crossings []Crossing) map[string][]Crossing {
	byEdge := map[string][]Crossing{}
	for _, c := range crossings {
		byEdge[c.JumpEdgeID] = append(byEdge[c.JumpEdgeID], c)
	}
	return byEdge
}

// ProcessEdgesWithJumps is processEdgesWithJumps(edges, config): the path
// data of every edge, by edge id, with a hop at each crossing.
func ProcessEdgesWithJumps(edges []EdgeGeom, config LineJumpConfig) *js.Map[string, string] {
	result := js.NewMap[string, string]()

	if !config.Enabled {
		for i := range edges {
			result.Set(edges[i].ID, plainPath(edges[i].Points))
		}
		return result
	}

	crossings := FindEdgeIntersections(edges)
	byEdge := jumpsByEdge(crossings)

	for i := range edges {
		edge := &edges[i]
		jumps := byEdge[edge.ID]
		if len(jumps) == 0 {
			result.Set(edge.ID, plainPath(edge.Points))
		} else {
			result.Set(edge.ID, rewriteEdgePath(edge, jumps, config))
		}
	}

	return result
}

// jsSpace is the character class \s of a JavaScript regular expression.
const jsSpace = `\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

var reStraightPath = regexp.MustCompile(`^[\d` + jsSpace + `+,.LMelm-]*$`)

// IsStraightPath is isStraightPath(d): whether the path data has only
// move and line commands with their numbers.
func IsStraightPath(d string) bool {
	return reStraightPath.MatchString(d)
}

// CurveSupportsLineHops is curveSupportsLineHops(curve): whether a curve
// makes segments that line hops can be drawn on. An empty string stands
// for undefined.
func CurveSupportsLineHops(curve string) bool {
	if curve == "" {
		return true
	}
	return curve == "linear" ||
		curve == "rounded" ||
		curve == "step" ||
		curve == "stepBefore" ||
		curve == "stepAfter"
}

// decodeDataPoints decodes the data-points attribute that InsertEdge sets.
// It gives the exact point list that the rendered path was made from,
// before the marker offsets. ok is false when the attribute does not hold
// at least two points.
func decodeDataPoints(raw string, has bool) (pts []ru.Point, ok bool) {
	if !has || raw == "" {
		return nil, false
	}
	// atob accepts unpadded input and ASCII white space.
	cleaned := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\f", "", "\r", "").Replace(raw)
	cleaned = strings.TrimRight(cleaned, "=")
	jsonBytes, err := base64.RawStdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, false
	}
	parsed, err := js.ParseJSON(jsonBytes)
	if err != nil {
		return nil, false
	}
	arr, isArray := parsed.([]any)
	if !isArray {
		return nil, false
	}
	for _, p := range arr {
		o := js.AsObj(p)
		if o == nil {
			continue
		}
		x, xok := o.Get("x").(float64)
		y, yok := o.Get("y").(float64)
		if xok && yok {
			pts = append(pts, ru.Point{X: x, Y: y})
		}
	}
	if len(pts) >= 2 {
		return pts, true
	}
	return nil, false
}

var (
	reDasharray      = regexp.MustCompile(`stroke-dasharray[` + jsSpace + `]*:[` + jsSpace + `]*0[` + jsSpace + `]+([\d.]+)[` + jsSpace + `]+[\d.]+[` + jsSpace + `]+([\d.]+)`)
	reDasharrayDecl  = regexp.MustCompile(`stroke-dasharray[` + jsSpace + `]*:[^;]*;?`)
	reEmptySemicolon = regexp.MustCompile(`;[` + jsSpace + `]*;+`)
)

// ApplyLineJumpsToSvg is applyLineJumpsToSvg(edgePathsGroup, edges,
// config): it rewrites the d attribute (and the neo dash array) of the
// rendered paths of the edges that cross another edge. It reads the true
// geometry from the data-points attribute that InsertEdge wrote. Edges
// whose curve is a true smoothing curve are skipped.
func ApplyLineJumpsToSvg(edgePathsGroup *dom.Node, edges []EdgeGeom, config LineJumpConfig) {
	if !config.Enabled {
		return
	}

	groupNode := edgePathsGroup
	if groupNode == nil {
		return
	}

	// Build a metadata lookup so per-edge properties (curve, arrow types)
	// survive the DOM round-trip. A later edge with the same id wins.
	edgeMeta := map[string]*EdgeGeom{}
	for i := range edges {
		edgeMeta[edges[i].ID] = &edges[i]
	}

	// Collect geometry from each path's data-points, preferring that over the
	// incoming `edges[].points` which came from pre-render layout state.
	pathByDataID := map[string]*dom.Node{}
	for _, el := range groupNode.SelectAll("path[data-id]") {
		id, _ := el.GetAttr("data-id")
		if _, has := pathByDataID[id]; !has {
			pathByDataID[id] = el
		}
	}

	renderedEdges := []EdgeGeom{}
	for i := range edges {
		e := edges[i]
		pathEl := pathByDataID[e.ID]
		if pathEl == nil {
			continue
		}
		raw, has := pathEl.GetAttr("data-points")
		if decoded, ok := decodeDataPoints(raw, has); ok {
			e.Points = decoded
		}
		renderedEdges = append(renderedEdges, e)
	}

	crossings := FindEdgeIntersections(renderedEdges)
	if len(crossings) == 0 {
		return
	}

	byEdge := jumpsByEdge(crossings)

	for i := range renderedEdges {
		renderedEdge := &renderedEdges[i]
		jumps := byEdge[renderedEdge.ID]
		if len(jumps) == 0 {
			continue
		}
		curveHint := ""
		if meta := edgeMeta[renderedEdge.ID]; meta != nil {
			curveHint = meta.Curve
		}
		if curveHint != "" && !CurveSupportsLineHops(curveHint) {
			continue
		}

		pathEl := pathByDataID[renderedEdge.ID]
		if pathEl == nil {
			continue
		}

		if curveHint == "" {
			currentD := pathEl.AttrOr("d", "")
			if !IsStraightPath(currentD) {
				continue
			}
		}

		// Read the ORIGINAL stroke-dasharray before rewriting so we can
		// recompute it against the new total length. The `neo` look emits:
		//   stroke-dasharray: 0 <oValueS> <len - oValueS - oValueE> <oValueE>;
		// Our rewritten path has a different length.
		originalStyle := pathEl.AttrOr("style", "")
		dasharrayMatch := reDasharray.FindStringSubmatch(originalStyle)
		var preservedOValueS, preservedOValueE float64
		if dasharrayMatch != nil {
			preservedOValueS = js.ParseFloat(dasharrayMatch[1])
			preservedOValueE = js.ParseFloat(dasharrayMatch[2])
		}

		newD := rewriteEdgePath(renderedEdge, jumps, config)
		pathEl.Attr("d", newD)

		if dasharrayMatch != nil {
			newLen := pathEl.TotalLength()
			onLen := js.Max(0, newLen-preservedOValueS-preservedOValueE)
			newDasharray := "0 " + js.Str(preservedOValueS) + " " + js.Str(onLen) + " " + js.Str(preservedOValueE)
			cleaned := reDasharrayDecl.ReplaceAllLiteralString(originalStyle, "stroke-dasharray: "+newDasharray+";")
			cleaned = reEmptySemicolon.ReplaceAllLiteralString(cleaned, ";")
			pathEl.Attr("style", cleaned)
		}
	}
}
