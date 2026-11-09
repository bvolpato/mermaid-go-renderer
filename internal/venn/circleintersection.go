// Ported from @upsetjs/venn.js 2.0.0, src/circleintersection.js (the part
// of build/venn.esm.js that mermaid 12.2.0 bundles).

package venn

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// small is SMALL: 1e-10.
const small = 1e-10

// intersectionPoint is an intersection point of two circles:
// { x, y, parentIndex, angle }.
type intersectionPoint struct {
	x, y        float64
	parentIndex [2]int
	angle       float64
}

// Arc is one arc of the border of an intersection area:
// { circle, width, p1, p2, large, sweep }.
type Arc struct {
	Circle *Circle
	Width  float64
	P1, P2 Point
	Large  bool
	Sweep  bool
}

// areaStats is the stats object that intersectionArea fills.
type areaStats struct {
	area               float64
	arcArea            float64
	polygonArea        float64
	arcs               []Arc
	innerPoints        []*intersectionPoint
	intersectionPoints []*intersectionPoint
}

// intersectionArea is intersectionArea(circles, stats).
//
// Returns the intersection area of a bunch of circles (where each circle
// is an object having an x,y and radius property)
func (e *engine) intersectionArea(circles []*Circle, stats *areaStats) float64 {
	// get all the intersection points of the circles
	intersectionPoints := getIntersectionPoints(circles)

	// filter out points that aren't included in all the circles
	innerPoints := []*intersectionPoint{}
	for _, p := range intersectionPoints {
		if containedInCircles(p, circles) {
			innerPoints = append(innerPoints, p)
		}
	}

	arcArea := 0.0
	polygonArea := 0.0
	arcs := []Arc{}

	// if we have intersection points that are within all the circles,
	// then figure out the area contained by them
	if len(innerPoints) > 1 {
		// sort the points by angle from the center of the polygon, which lets
		// us just iterate over points to get the edges
		center := Point{}
		for _, point := range innerPoints {
			center.X += point.x
			center.Y += point.y
		}
		center.X /= float64(len(innerPoints))
		center.Y /= float64(len(innerPoints))
		for i := 0; i < len(innerPoints); i++ {
			p := innerPoints[i]
			p.angle = e.m.Atan2(p.x-center.X, p.y-center.Y)
		}
		sortBy(e, innerPoints, func(a, b *intersectionPoint) float64 { return b.angle - a.angle })

		// iterate over all points, get arc between the points
		// and update the areas
		p2 := innerPoints[len(innerPoints)-1]
		for i := 0; i < len(innerPoints); i++ {
			p1 := innerPoints[i]

			// polygon area updates easily ...
			polygonArea += float64((p2.x + p1.x) * (p1.y - p2.y))

			// updating the arc area is a little more involved
			midPoint := Point{X: float64((p1.x + p2.x) / 2), Y: float64((p1.y + p2.y) / 2)}
			var arc *Arc

			for j := 0; j < len(p1.parentIndex); j++ {
				if p2.parentIndex[0] == p1.parentIndex[j] || p2.parentIndex[1] == p1.parentIndex[j] {
					// figure out the angle halfway between the two points
					// on the current circle
					circle := circles[p1.parentIndex[j]]
					a1 := e.m.Atan2(p1.x-circle.X, p1.y-circle.Y)
					a2 := e.m.Atan2(p2.x-circle.X, p2.y-circle.Y)

					angleDiff := a2 - a1
					if angleDiff < 0 {
						angleDiff += 2 * pi
					}

					// and use that angle to figure out the width of the
					// arc
					a := a2 - float64(angleDiff/2)
					width := distance(
						midPoint.X, midPoint.Y,
						circle.X+float64(circle.Radius*e.m.Sin(a)),
						circle.Y+float64(circle.Radius*e.m.Cos(a)),
					)

					// clamp the width to the largest is can actually be
					// (sometimes slightly overflows because of FP errors)
					if width > circle.Radius*2 {
						width = circle.Radius * 2
					}

					// pick the circle whose arc has the smallest width
					if arc == nil || arc.Width > width {
						arc = &Arc{
							Circle: circle,
							Width:  width,
							P1:     Point{X: p1.x, Y: p1.y},
							P2:     Point{X: p2.x, Y: p2.y},
							Large:  width > circle.Radius,
							Sweep:  true,
						}
					}
				}
			}

			if arc != nil {
				arcs = append(arcs, *arc)
				arcArea += e.circleArea(arc.Circle.Radius, arc.Width)
				p2 = p1
			}
		}
	} else {
		// no intersection points, is either disjoint - or is completely
		// overlapped. figure out which by examining the smallest circle
		if len(circles) == 0 {
			throwUndefined("radius")
		}
		smallest := circles[0]
		for i := 1; i < len(circles); i++ {
			if circles[i].Radius < smallest.Radius {
				smallest = circles[i]
			}
		}

		// make sure the smallest circle is completely contained in all
		// the other circles
		disjoint := false
		for i := 0; i < len(circles); i++ {
			if distance(circles[i].X, circles[i].Y, smallest.X, smallest.Y) > math.Abs(smallest.Radius-circles[i].Radius) {
				disjoint = true
				break
			}
		}

		if disjoint {
			arcArea = 0
			polygonArea = 0
		} else {
			arcArea = float64(smallest.Radius * smallest.Radius * pi)
			arcs = append(arcs, Arc{
				Circle: smallest,
				P1:     Point{X: smallest.X, Y: smallest.Y + smallest.Radius},
				P2:     Point{X: smallest.X - small, Y: smallest.Y + smallest.Radius},
				Width:  smallest.Radius * 2,
				Large:  true,
				Sweep:  true,
			})
		}
	}

	polygonArea = float64(polygonArea / 2)

	if stats != nil {
		stats.area = arcArea + polygonArea
		stats.arcArea = arcArea
		stats.polygonArea = polygonArea
		stats.arcs = arcs
		stats.innerPoints = innerPoints
		stats.intersectionPoints = intersectionPoints
	}

	return arcArea + polygonArea
}

// containedInCircles is containedInCircles(point, circles): it returns
// whether a point is contained by all of a list of circles.
func containedInCircles(point *intersectionPoint, circles []*Circle) bool {
	for _, circle := range circles {
		if !(distance(point.x, point.y, circle.X, circle.Y) < circle.Radius+small) {
			return false
		}
	}
	return true
}

// getIntersectionPoints is getIntersectionPoints(circles): all intersection
// points between a bunch of circles.
func getIntersectionPoints(circles []*Circle) []*intersectionPoint {
	ret := []*intersectionPoint{}
	for i := 0; i < len(circles); i++ {
		for j := i + 1; j < len(circles); j++ {
			intersect := circleCircleIntersection(circles[i], circles[j])
			for _, p := range intersect {
				ret = append(ret, &intersectionPoint{x: p.X, y: p.Y, parentIndex: [2]int{i, j}})
			}
		}
	}
	return ret
}

// circleArea is circleArea(r, width): the circular segment area
// calculation. See http://mathworld.wolfram.com/CircularSegment.html
func (e *engine) circleArea(r, width float64) float64 {
	return float64(r*r*e.m.Acos(1-width/r)) - float64((r-width)*math.Sqrt(width*(float64(2*r)-width)))
}

// distance is distance(p1, p2): the euclidean distance between two points.
func distance(x1, y1, x2, y2 float64) float64 {
	return math.Sqrt(float64((x1-x2)*(x1-x2)) + float64((y1-y2)*(y1-y2)))
}

// circleOverlap is circleOverlap(r1, r2, d).
//
// Returns the overlap area of two circles of radius r1 and r2 - that
// have their centers separated by distance d. Simpler faster
// circle intersection for only two circles
func (e *engine) circleOverlap(r1, r2, d float64) float64 {
	// no overlap
	if d >= r1+r2 {
		return 0
	}

	// completely overlapped
	if d <= math.Abs(r1-r2) {
		return pi * js.Min(r1, r2) * js.Min(r1, r2)
	}

	w1 := r1 - (float64(d*d)-float64(r2*r2)+float64(r1*r1))/(2*d)
	w2 := r2 - (float64(d*d)-float64(r1*r1)+float64(r2*r2))/(2*d)
	return e.circleArea(r1, w1) + e.circleArea(r2, w2)
}

// circleCircleIntersection is circleCircleIntersection(p1, p2).
//
// Given two circles (containing a x/y/radius attributes),
// returns the intersecting points if possible
// note: doesn't handle cases where there are infinitely many
// intersection points (circles are equivalent):, or only one intersection point
func circleCircleIntersection(p1, p2 *Circle) []Point {
	d := distance(p1.X, p1.Y, p2.X, p2.Y)
	r1 := p1.Radius
	r2 := p2.Radius

	// if to far away, or self contained - can't be done
	if d >= r1+r2 || d <= math.Abs(r1-r2) {
		return nil
	}

	a := (float64(r1*r1) - float64(r2*r2) + float64(d*d)) / (2 * d)
	h := math.Sqrt(float64(r1*r1) - float64(a*a))
	x0 := p1.X + (a*(p2.X-p1.X))/d
	y0 := p1.Y + (a*(p2.Y-p1.Y))/d
	rx := float64(-(p2.Y - p1.Y) * (h / d))
	ry := float64(-(p2.X - p1.X) * (h / d))

	return []Point{
		{X: x0 + rx, Y: y0 - ry},
		{X: x0 - rx, Y: y0 + ry},
	}
}

// getCenter is getCenter(points): the center of a bunch of points.
func getCenter(points []Point) Point {
	center := Point{X: 0, Y: 0}
	for _, point := range points {
		center.X += point.X
		center.Y += point.Y
	}
	center.X /= float64(len(points))
	center.Y /= float64(len(points))
	return center
}
