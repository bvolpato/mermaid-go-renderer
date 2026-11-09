// Ported from mermaid 12.2.0, rendering-util/layout-algorithms/elk/geometry.ts
// (MIT).

package elk

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// P is a point.
type P = ru.Point

// RectLike is a rectangle given by its center and its size.
type RectLike struct {
	X       float64 // center x
	Y       float64 // center y
	Width   float64
	Height  float64
	Padding float64 // NaN when undefined
}

// NodeLike is the part of a node that the geometry functions use: its
// intersect function. A nil function stands for a node without one.
type NodeLike interface {
	intersectFunc() func(point P) P
}

const (
	EPS      = 1.0
	PUSH_OUT = 10.0
)

// onBorder reports whether p is on the border of bounds, within tol.
// Upstream has the default tol = 0.5.
func onBorder(bounds RectLike, p P, tol float64) bool {
	halfW := float64(bounds.Width / 2)
	halfH := float64(bounds.Height / 2)
	left := bounds.X - halfW
	right := bounds.X + halfW
	top := bounds.Y - halfH
	bottom := bounds.Y + halfH

	onLeft := math.Abs(p.X-left) <= tol && p.Y >= top-tol && p.Y <= bottom+tol
	onRight := math.Abs(p.X-right) <= tol && p.Y >= top-tol && p.Y <= bottom+tol
	onTop := math.Abs(p.Y-top) <= tol && p.X >= left-tol && p.X <= right+tol
	onBottom := math.Abs(p.Y-bottom) <= tol && p.X >= left-tol && p.X <= right+tol
	return onLeft || onRight || onTop || onBottom
}

// intersection computes the intersection between a rectangle (center x/y,
// width/height) and the line segment from insidePoint to outsidePoint. It
// returns the point on the border of the rectangle.
//
// This version avoids snapping to outsidePoint when certain variables
// evaluate to 0. It only enforces axis-constant behavior for purely
// vertical or horizontal approaches.
func intersection(node RectLike, outsidePoint, insidePoint P) P {
	x := node.X
	y := node.Y

	dx := math.Abs(x - insidePoint.X)
	w := float64(node.Width / 2)
	var r float64
	if insidePoint.X < outsidePoint.X {
		r = w - dx
	} else {
		r = w + dx
	}
	h := float64(node.Height / 2)

	Q := math.Abs(outsidePoint.Y - insidePoint.Y)
	R := math.Abs(outsidePoint.X - insidePoint.X)

	if math.Abs(y-outsidePoint.Y)*w > math.Abs(x-outsidePoint.X)*h {
		// Intersection is top or bottom of rect.
		var q float64
		if insidePoint.Y < outsidePoint.Y {
			q = outsidePoint.Y - h - y
		} else {
			q = y - h - outsidePoint.Y
		}
		r = (R * q) / Q
		var res P
		if insidePoint.X < outsidePoint.X {
			res.X = insidePoint.X + r
		} else {
			res.X = insidePoint.X - R + r
		}
		if insidePoint.Y < outsidePoint.Y {
			res.Y = insidePoint.Y + Q - q
		} else {
			res.Y = insidePoint.Y - Q + q
		}

		// Keep axis-constant special-cases only
		if R == 0 {
			res.X = outsidePoint.X
		}
		if Q == 0 {
			res.Y = outsidePoint.Y
		}
		return res
	}
	// Intersection on sides of rect
	if insidePoint.X < outsidePoint.X {
		r = outsidePoint.X - w - x
	} else {
		r = x - w - outsidePoint.X
	}
	q := (Q * r) / R
	var _x, _y float64
	if insidePoint.X < outsidePoint.X {
		_x = insidePoint.X + R - r
	} else {
		_x = insidePoint.X - R + r
	}
	if insidePoint.Y < outsidePoint.Y {
		_y = insidePoint.Y + q
	} else {
		_y = insidePoint.Y - q
	}

	// Only handle axis-constant cases
	if R == 0 {
		_x = outsidePoint.X
	}
	if Q == 0 {
		_y = outsidePoint.Y
	}

	return P{X: _x, Y: _y}
}

// outsideNode reports whether point is on or outside the border of node.
func outsideNode(node RectLike, point P) bool {
	x := node.X
	y := node.Y
	dx := math.Abs(point.X - x)
	dy := math.Abs(point.Y - y)
	w := node.Width / 2
	h := node.Height / 2
	return dx >= w || dy >= h
}

// ensureTrulyOutside moves a point that is on the border of bounds away
// from the center by push. Upstream has the default push = PUSH_OUT.
func ensureTrulyOutside(bounds RectLike, p P, push float64) P {
	dx := math.Abs(p.X - bounds.X)
	dy := math.Abs(p.Y - bounds.Y)
	w := float64(bounds.Width / 2)
	h := float64(bounds.Height / 2)
	if math.Abs(dx-w) < EPS || math.Abs(dy-h) < EPS {
		dirX := p.X - bounds.X
		dirY := p.Y - bounds.Y
		length := math.Sqrt(float64(dirX*dirX) + float64(dirY*dirY))
		if length > 0 {
			return P{
				X: bounds.X + float64((dirX/length)*(length+push)),
				Y: bounds.Y + float64((dirY/length)*(length+push)),
			}
		}
	}
	return p
}

func makeInsidePoint(bounds RectLike, outside, center P) P {
	isVertical := math.Abs(outside.X-bounds.X) < EPS
	isHorizontal := math.Abs(outside.Y-bounds.Y) < EPS
	var res P
	if isVertical {
		res.X = outside.X
	} else if outside.X < bounds.X {
		res.X = bounds.X - float64(bounds.Width/4)
	} else {
		res.X = bounds.X + float64(bounds.Width/4)
	}
	if isHorizontal {
		res.Y = outside.Y
	} else {
		res.Y = center.Y
	}
	return res
}

// tryNodeIntersect is the intersection of the shape of the node with the
// ray from its center to outside. The second result is false where upstream
// returns null: the node has no intersect function, the result is on the
// wrong side of the node, or it is too close to outside.
func tryNodeIntersect(node NodeLike, bounds RectLike, outside P) (P, bool) {
	var intersect func(point P) P
	if node != nil {
		intersect = node.intersectFunc()
	}
	if intersect == nil {
		return P{}, false
	}
	res := intersect(outside)
	wrongSide := (outside.X < bounds.X && res.X > bounds.X) || (outside.X > bounds.X && res.X < bounds.X)
	if wrongSide {
		return P{}, false
	}
	dist := js.Hypot(outside.X-res.X, outside.Y-res.Y)
	if dist <= EPS {
		return P{}, false
	}
	return res, true
}

func fallbackIntersection(bounds RectLike, outside, center P) P {
	inside := makeInsidePoint(bounds, outside, center)
	return intersection(bounds, outside, inside)
}

// OUTLINE_RAY_STEPS is the number of bisection steps used to walk a ray onto
// the node outline. 20 steps take a 200px starting bracket to about 2e-4px.
const OUTLINE_RAY_STEPS = 20

// DEPARTURE_AXIS_EPS is how far off an axis a departure may sit and still
// count as axis-aligned.
const DEPARTURE_AXIS_EPS = 1e-6

// insideOutline reports whether a point lies inside the outline of the
// node. It is derived from intersect alone: the intersect of the shape
// returns where the ray from the node center through the probe leaves the
// outline, so the probe is inside exactly when it is no further from the
// center than that crossing is.
func insideOutline(intersect func(point P) P, centre, probe P) bool {
	if intersect == nil {
		return false
	}
	crossing := intersect(probe)
	probeDist := js.Hypot(probe.X-centre.X, probe.Y-centre.Y)
	outlineDist := js.Hypot(crossing.X-centre.X, crossing.Y-centre.Y)
	return probeDist <= outlineDist+1e-9
}

// outlineAttachPoint is where the outline of the node meets the ray that
// runs into the node from port, against the direction the edge departs in.
//
// ELK routes to ports on the bounding box of the node, and always leaves
// one perpendicular to the side it sits on. For a shape whose outline is
// inside the box the attachment has to move inwards. Moving along the
// departure axis keeps the attachment collinear with the stub of ELK.
//
// The second result is false where upstream returns null: no intersect, a
// departure direction that is not axis-aligned, or an interior sample that
// is not inside.
func outlineAttachPoint(node NodeLike, bounds RectLike, port, next P) (P, bool) {
	var intersect func(point P) P
	if node != nil {
		intersect = node.intersectFunc()
	}
	if intersect == nil {
		return P{}, false
	}

	dx := next.X - port.X
	dy := next.Y - port.Y
	if dx == 0 && dy == 0 {
		return P{}, false
	}

	// A diagonal departure has no single axis to preserve.
	if math.Abs(dx) > DEPARTURE_AXIS_EPS && math.Abs(dy) > DEPARTURE_AXIS_EPS {
		return P{}, false
	}

	centre := P{X: bounds.X, Y: bounds.Y}
	horizontal := math.Abs(dx) > math.Abs(dy)
	along := func(t float64) P {
		if horizontal {
			return P{X: t, Y: port.Y}
		}
		return P{X: port.X, Y: t}
	}

	// Walk in from the centre-line towards the port: inside at one end, on
	// or outside the outline at the other.
	var inner, outer float64
	if horizontal {
		inner = centre.X
		outer = port.X
	} else {
		inner = centre.Y
		outer = port.Y
	}
	if !insideOutline(intersect, centre, along(inner)) {
		return P{}, false
	}
	if insideOutline(intersect, centre, along(outer)) {
		// The port itself is on or inside the outline: it is the attachment.
		return port, true
	}

	for step := 0; step < OUTLINE_RAY_STEPS; step++ {
		mid := (inner + outer) / 2
		if insideOutline(intersect, centre, along(mid)) {
			inner = mid
		} else {
			outer = mid
		}
	}
	return along(inner), true
}

func computeNodeIntersection(node NodeLike, bounds RectLike, outside, center P) P {
	outside2 := ensureTrulyOutside(bounds, outside, PUSH_OUT)
	if res, ok := tryNodeIntersect(node, bounds, outside2); ok {
		return res
	}
	return fallbackIntersection(bounds, outside2, center)
}

// replaceEndpoint replaces the first or the last point by value. When the
// endpoint is within tol of value already, it is removed. Upstream has the
// default tol = 0.1. A false ok stands for a null or undefined value.
func replaceEndpoint(points *[]P, which string, value P, ok bool, tol float64) {
	if !ok || len(*points) == 0 {
		return
	}

	pts := *points
	if which == "start" {
		if len(pts) > 0 &&
			math.Abs(pts[0].X-value.X) < tol &&
			math.Abs(pts[0].Y-value.Y) < tol {
			// duplicate start remove it
			*points = pts[1:]
		} else {
			pts[0] = value
		}
	} else {
		last := len(pts) - 1
		if len(pts) > 0 &&
			math.Abs(pts[last].X-value.X) < tol &&
			math.Abs(pts[last].Y-value.Y) < tol {
			// duplicate end remove it
			*points = pts[:last]
		} else {
			pts[last] = value
		}
	}
}
