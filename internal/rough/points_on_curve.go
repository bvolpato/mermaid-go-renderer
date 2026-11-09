// Ported from points-on-curve/lib/index.js and curve-to-bezier.js
// (points-on-curve 0.2.0).
//
// The lists hold pointers to points, because the JavaScript code shares the
// point arrays between lists and the hachure fill changes them in place.

package rough

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// distance between 2 points
func distance(m *mathFuncs, p1, p2 *Point) float64 {
	return math.Sqrt(distanceSq(m, p1, p2))
}

// distance between 2 points squared
func distanceSq(m *mathFuncs, p1, p2 *Point) float64 {
	return m.pow(p1[0]-p2[0], 2) + m.pow(p1[1]-p2[1], 2)
}

// Distance squared from a point p to the line segment vw
func distanceToSegmentSq(m *mathFuncs, p, v, w *Point) float64 {
	l2 := distanceSq(m, v, w)
	if l2 == 0 {
		return distanceSq(m, p, v)
	}
	t := (float64((p[0]-v[0])*(w[0]-v[0])) + float64((p[1]-v[1])*(w[1]-v[1]))) / l2
	t = js.Max(0, js.Min(1, t))
	return distanceSq(m, p, lerp(v, w, t))
}

func lerp(a, b *Point, t float64) *Point {
	return &Point{
		a[0] + float64((b[0]-a[0])*t),
		a[1] + float64((b[1]-a[1])*t),
	}
}

// Adapted from https://seant23.wordpress.com/2010/11/12/offset-bezier-curves/
func flatness(p1, p2, p3, p4 *Point) float64 {
	ux := float64(3*p2[0]) - float64(2*p1[0]) - p4[0]
	ux = float64(ux * ux)
	uy := float64(3*p2[1]) - float64(2*p1[1]) - p4[1]
	uy = float64(uy * uy)
	vx := float64(3*p3[0]) - float64(2*p4[0]) - p1[0]
	vx = float64(vx * vx)
	vy := float64(3*p3[1]) - float64(2*p4[1]) - p1[1]
	vy = float64(vy * vy)
	if ux < vx {
		ux = vx
	}
	if uy < vy {
		uy = vy
	}
	return ux + uy
}

func getPointsOnBezierCurveWithSplitting(g *Generator, p1, p2, p3, p4 *Point, tolerance float64, outPoints []*Point, depth int) []*Point {
	if depth > maxCallDepth {
		// The curve never becomes flat, for example with a coordinate
		// that is not finite. JavaScript ends with a stack overflow.
		throw("RangeError", "Maximum call stack size exceeded")
	}
	if flatness(p1, p2, p3, p4) < tolerance {
		p0 := p1
		if len(outPoints) > 0 {
			d := distance(&g.m, outPoints[len(outPoints)-1], p0)
			if d > 1 {
				outPoints = append(outPoints, p0)
			}
		} else {
			outPoints = append(outPoints, p0)
		}
		outPoints = append(outPoints, p4)
	} else {
		// subdivide
		t := .5
		q1 := lerp(p1, p2, t)
		q2 := lerp(p2, p3, t)
		q3 := lerp(p3, p4, t)
		r1 := lerp(q1, q2, t)
		r2 := lerp(q2, q3, t)
		red := lerp(r1, r2, t)
		outPoints = getPointsOnBezierCurveWithSplitting(g, p1, q1, r1, red, tolerance, outPoints, depth+1)
		outPoints = getPointsOnBezierCurveWithSplitting(g, red, r2, q3, p4, tolerance, outPoints, depth+1)
	}
	return outPoints
}

func simplify(g *Generator, points []*Point, distance float64) []*Point {
	return simplifyPoints(g, points, 0, len(points), distance, nil, 0)
}

// Ramer–Douglas–Peucker algorithm
// https://en.wikipedia.org/wiki/Ramer%E2%80%93Douglas%E2%80%93Peucker_algorithm
func simplifyPoints(g *Generator, points []*Point, start, end int, epsilon float64, outPoints []*Point, depth int) []*Point {
	if epsilon < 0 || depth > maxSimplifyDepth {
		// With a negative epsilon each call splits again. JavaScript ends
		// with a stack overflow.
		throw("RangeError", "Maximum call stack size exceeded")
	}
	if end-start < 1 || end > len(points) {
		return outPoints
	}
	// find the most distance point from the endpoints
	s := points[start]
	e := points[end-1]
	maxDistSq := 0.0
	maxNdx := 1
	for i := start + 1; i < end-1; i++ {
		distSq := distanceToSegmentSq(&g.m, points[i], s, e)
		if distSq > maxDistSq {
			maxDistSq = distSq
			maxNdx = i
		}
	}
	// if that point is too far, split
	if math.Sqrt(maxDistSq) > epsilon {
		outPoints = simplifyPoints(g, points, start, maxNdx+1, epsilon, outPoints, depth+1)
		outPoints = simplifyPoints(g, points, maxNdx, end, epsilon, outPoints, depth+1)
	} else {
		if len(outPoints) == 0 {
			outPoints = append(outPoints, s)
		}
		outPoints = append(outPoints, e)
	}
	return outPoints
}

// pointsOnBezierCurves returns points on the curves. points has 3n+1
// entries. A distance of 0 or less means that the result is not simplified.
func pointsOnBezierCurves(g *Generator, points []*Point, tolerance, distance float64) []*Point {
	newPoints := []*Point{}
	numSegments := (len(points) - 1) / 3
	for i := 0; i < numSegments; i++ {
		offset := i * 3
		newPoints = getPointsOnBezierCurveWithSplitting(g, points[offset], points[offset+1], points[offset+2], points[offset+3], tolerance, newPoints, 0)
	}
	if truthyNum(distance) && distance > 0 {
		return simplifyPoints(g, newPoints, 0, len(newPoints), distance, nil, 0)
	}
	return newPoints
}

// curveToBezier converts a curve through the points to a list of cubic
// Bezier control points. The caller gives at least 3 points.
func curveToBezier(pointsIn []*Point, curveTightness float64) []*Point {
	n := len(pointsIn)
	if n < 3 {
		throw("Error", "A curve must have at least three points.")
	}
	clone := func(p *Point) *Point { c := *p; return &c }
	out := []*Point{}
	if n == 3 {
		out = append(out, clone(pointsIn[0]), clone(pointsIn[1]), clone(pointsIn[2]), clone(pointsIn[2]))
	} else {
		points := []*Point{}
		points = append(points, pointsIn[0], pointsIn[0])
		for i := 1; i < len(pointsIn); i++ {
			points = append(points, pointsIn[i])
			if i == (len(pointsIn) - 1) {
				points = append(points, pointsIn[i])
			}
		}
		s := 1 - curveTightness
		out = append(out, clone(points[0]))
		for i := 1; (i + 2) < len(points); i++ {
			cachedVertArray := points[i]
			b1 := &Point{
				cachedVertArray[0] + (float64(s*points[i+1][0])-float64(s*points[i-1][0]))/6,
				cachedVertArray[1] + (float64(s*points[i+1][1])-float64(s*points[i-1][1]))/6,
			}
			b2 := &Point{
				points[i+1][0] + (float64(s*points[i][0])-float64(s*points[i+2][0]))/6,
				points[i+1][1] + (float64(s*points[i][1])-float64(s*points[i+2][1]))/6,
			}
			b3 := &Point{points[i+1][0], points[i+1][1]}
			out = append(out, b1, b2, b3)
		}
	}
	return out
}
