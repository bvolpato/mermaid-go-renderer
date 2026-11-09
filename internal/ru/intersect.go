// Ported from mermaid 12.2.0, rendering-elements/intersect/*.js (MIT).

package ru

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// IntersectNode is intersect.node(node, point).
func IntersectNode(node *Node, point Point) Point {
	return node.Intersect(point)
}

// IntersectRect is intersect.rect(node, point). The node is given by its
// center and size, as a Node or a Bounds.
func IntersectRect(node Bounds, point Point) Point {
	x := node.X
	y := node.Y

	dx := point.X - x
	dy := point.Y - y
	w := node.Width / 2
	h := node.Height / 2

	var sx, sy float64
	if float64(math.Abs(dy)*w) > float64(math.Abs(dx)*h) {
		if dy < 0 {
			h = -h
		}
		if dy == 0 {
			sx = 0
		} else {
			sx = float64(h*dx) / dy
		}
		sy = h
	} else {
		if dx < 0 {
			w = -w
		}
		sx = w
		if dx == 0 {
			sy = 0
		} else {
			sy = float64(w*dy) / dx
		}
	}
	return Point{X: x + sx, Y: y + sy}
}

// IntersectCircle is intersect.circle(node, rx, point).
func IntersectCircle(node Bounds, rx float64, point Point) Point {
	return IntersectEllipse(node, rx, rx, point)
}

// IntersectEllipse is intersect.ellipse(node, rx, ry, point).
func IntersectEllipse(node Bounds, rx, ry float64, point Point) Point {
	cx := node.X
	cy := node.Y

	px := cx - point.X
	py := cy - point.Y

	det := math.Sqrt(float64(rx*rx*py*py) + float64(ry*ry*px*px))

	dx := math.Abs(float64(rx*ry*px) / det)
	if point.X < cx {
		dx = -dx
	}
	dy := math.Abs(float64(rx*ry*py) / det)
	if point.Y < cy {
		dy = -dy
	}
	return Point{X: cx + dx, Y: cy + dy}
}

// IntersectPolygon is intersect.polygon(node, polyPoints, point). When no
// edge of the polygon is hit, upstream returns the node itself: the result
// is then the center of the node.
func IntersectPolygon(node Bounds, polyPoints []Point, point Point) Point {
	x1 := node.X
	y1 := node.Y

	var intersections []Point

	minX := math.Inf(1)
	minY := math.Inf(1)
	for _, entry := range polyPoints {
		minX = js.Min(minX, entry.X)
		minY = js.Min(minY, entry.Y)
	}

	left := x1 - float64(node.Width/2) - minX
	top := y1 - float64(node.Height/2) - minY

	for i := 0; i < len(polyPoints); i++ {
		p1 := polyPoints[i]
		next := 0
		if i < len(polyPoints)-1 {
			next = i + 1
		}
		p2 := polyPoints[next]
		if q, ok := intersectLine(
			Point{node.X, node.Y},
			point,
			Point{X: left + p1.X, Y: top + p1.Y},
			Point{X: left + p2.X, Y: top + p2.Y},
		); ok {
			intersections = append(intersections, q)
		}
	}

	if len(intersections) == 0 {
		return Point{X: node.X, Y: node.Y}
	}

	if len(intersections) > 1 {
		js.SortFunc(intersections, func(p, q Point) float64 {
			pdx := p.X - point.X
			pdy := p.Y - point.Y
			distp := math.Sqrt(float64(pdx*pdx) + float64(pdy*pdy))

			qdx := q.X - point.X
			qdy := q.Y - point.Y
			distq := math.Sqrt(float64(qdx*qdx) + float64(qdy*qdy))

			if distp < distq {
				return -1
			}
			if distp == distq {
				return 0
			}
			return 1
		})
	}
	return intersections[0]
}

// intersectLine returns the point where the line p1-p2 crosses the segment
// q1-q2. The second result is false when they do not intersect.
func intersectLine(p1, p2, q1, q2 Point) (Point, bool) {
	a1 := p2.Y - p1.Y
	b1 := p1.X - p2.X
	c1 := float64(p2.X*p1.Y) - float64(p1.X*p2.Y)

	r3 := float64(a1*q1.X) + float64(b1*q1.Y) + c1
	r4 := float64(a1*q2.X) + float64(b1*q2.Y) + c1

	const epsilon = 1e-6

	if r3 != 0 && r4 != 0 && sameSign(r3, r4) {
		return Point{}, false
	}

	a2 := q2.Y - q1.Y
	b2 := q1.X - q2.X
	c2 := float64(q2.X*q1.Y) - float64(q1.X*q2.Y)

	r1 := float64(a2*p1.X) + float64(b2*p1.Y) + c2
	r2 := float64(a2*p2.X) + float64(b2*p2.Y) + c2

	if math.Abs(r1) < epsilon && math.Abs(r2) < epsilon && sameSign(r1, r2) {
		return Point{}, false
	}

	denom := float64(a1*b2) - float64(a2*b1)
	if denom == 0 {
		return Point{}, false
	}

	x := (float64(b1*c2) - float64(b2*c1)) / denom
	y := (float64(a2*c1) - float64(a1*c2)) / denom
	return Point{X: x, Y: y}, true
}

func sameSign(r1, r2 float64) bool {
	return r1*r2 > 0
}

// NodeBounds returns the center and size of a node as Bounds, for the
// intersect functions that upstream calls with the node itself.
func NodeBounds(n *Node) Bounds {
	return Bounds{X: n.X, Y: n.Y, Width: n.Width, Height: n.Height}
}
