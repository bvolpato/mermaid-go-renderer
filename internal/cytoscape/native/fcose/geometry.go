// Ported from layout-base 2.0.1 (MIT license): src/util/IGeometry.js,
// src/util/IMath.js, src/util/RectangleD.js, src/util/PointD.js and
// src/util/DimensionD.js.

package fcose

import "math"

// nan stands for a value that JavaScript leaves undefined and then uses in
// arithmetic.
var nan = math.NaN()

// PointD is PointD, and Point for the places that hold numbers.
type PointD struct {
	x, y float64
}

// RectangleD is RectangleD.
type RectangleD struct {
	x, y, width, height float64
}

func (r *RectangleD) getX() float64      { return r.x }
func (r *RectangleD) getY() float64      { return r.y }
func (r *RectangleD) getRight() float64  { return r.x + r.width }
func (r *RectangleD) getBottom() float64 { return r.y + r.height }

func (r *RectangleD) intersects(a *RectangleD) bool {
	if r.getRight() < a.x {
		return false
	}

	if r.getBottom() < a.y {
		return false
	}

	if a.getRight() < r.x {
		return false
	}

	if a.getBottom() < r.y {
		return false
	}

	return true
}

func (r *RectangleD) getCenterX() float64    { return r.x + float64(r.width/2) }
func (r *RectangleD) getCenterY() float64    { return r.y + float64(r.height/2) }
func (r *RectangleD) getWidthHalf() float64  { return r.width / 2 }
func (r *RectangleD) getHeightHalf() float64 { return r.height / 2 }

// iMathSign is IMath.sign.
func iMathSign(value float64) float64 {
	if value > 0 {
		return 1
	} else if value < 0 {
		return -1
	}
	return 0
}

// calcSeparationAmount is IGeometry.calcSeparationAmount: the amount by
// which the two rectangles that overlap must move apart.
func calcSeparationAmount(rectA, rectB *RectangleD, overlapAmount *[2]float64, separationBuffer float64) {
	if !rectA.intersects(rectB) {
		panic("assert failed")
	}

	var directions [2]float64

	decideDirectionsForOverlappingNodes(rectA, rectB, &directions)

	overlapAmount[0] = min(rectA.getRight(), rectB.getRight()) - max(rectA.x, rectB.x)
	overlapAmount[1] = min(rectA.getBottom(), rectB.getBottom()) - max(rectA.y, rectB.y)

	// update the overlapping amounts for the following cases:
	if rectA.getX() <= rectB.getX() && rectA.getRight() >= rectB.getRight() {
		// Case x.1: rectB is inside the horizontal extent of rectA.
		overlapAmount[0] += min(rectB.getX()-rectA.getX(), rectA.getRight()-rectB.getRight())
	} else if rectB.getX() <= rectA.getX() && rectB.getRight() >= rectA.getRight() {
		// Case x.2: rectA is inside the horizontal extent of rectB.
		overlapAmount[0] += min(rectA.getX()-rectB.getX(), rectB.getRight()-rectA.getRight())
	}
	if rectA.getY() <= rectB.getY() && rectA.getBottom() >= rectB.getBottom() {
		// Case y.1: rectB is inside the vertical extent of rectA.
		overlapAmount[1] += min(rectB.getY()-rectA.getY(), rectA.getBottom()-rectB.getBottom())
	} else if rectB.getY() <= rectA.getY() && rectB.getBottom() >= rectA.getBottom() {
		// Case y.2: rectA is inside the vertical extent of rectB.
		overlapAmount[1] += min(rectA.getY()-rectB.getY(), rectB.getBottom()-rectA.getBottom())
	}

	// find slope of the line passes two centers
	slope := math.Abs((rectB.getCenterY() - rectA.getCenterY()) / (rectB.getCenterX() - rectA.getCenterX()))
	// if centers are overlapped
	if rectB.getCenterY() == rectA.getCenterY() && rectB.getCenterX() == rectA.getCenterX() {
		// assume the slope is 1 (45 degree)
		slope = 1.0
	}

	moveByY := slope * overlapAmount[0]
	moveByX := overlapAmount[1] / slope
	if overlapAmount[0] < moveByX {
		moveByX = overlapAmount[0]
	} else {
		moveByY = overlapAmount[1]
	}
	// return half the amount so that if each rectangle is moved by these
	// amounts in opposite directions, overlap will be resolved
	overlapAmount[0] = -1 * directions[0] * (float64(moveByX/2) + separationBuffer)
	overlapAmount[1] = -1 * directions[1] * (float64(moveByY/2) + separationBuffer)
}

// decideDirectionsForOverlappingNodes is
// IGeometry.decideDirectionsForOverlappingNodes.
func decideDirectionsForOverlappingNodes(rectA, rectB *RectangleD, directions *[2]float64) {
	if rectA.getCenterX() < rectB.getCenterX() {
		directions[0] = -1
	} else {
		directions[0] = 1
	}

	if rectA.getCenterY() < rectB.getCenterY() {
		directions[1] = -1
	} else {
		directions[1] = 1
	}
}

// getIntersection2 is IGeometry.getIntersection2: the points where the line
// between the centers leaves the two rectangles. result[0] and result[1] are
// the clip point of rectA, result[2] and result[3] the clip point of rectB.
// It returns true when the rectangles intersect. A member that the JavaScript
// does not assign stays as the caller set it: NaN stands for undefined.
func getIntersection2(rectA, rectB *RectangleD, result *[4]float64) bool {
	p1x := rectA.getCenterX()
	p1y := rectA.getCenterY()
	p2x := rectB.getCenterX()
	p2y := rectB.getCenterY()

	// if two rectangles intersect, then clipping points are centers
	if rectA.intersects(rectB) {
		result[0] = p1x
		result[1] = p1y
		result[2] = p2x
		result[3] = p2y
		return true
	}
	// variables for rectA
	topLeftAx := rectA.getX()
	topLeftAy := rectA.getY()
	topRightAx := rectA.getRight()
	bottomLeftAx := rectA.getX()
	bottomLeftAy := rectA.getBottom()
	bottomRightAx := rectA.getRight()
	halfWidthA := rectA.getWidthHalf()
	halfHeightA := rectA.getHeightHalf()
	// variables for rectB
	topLeftBx := rectB.getX()
	topLeftBy := rectB.getY()
	topRightBx := rectB.getRight()
	bottomLeftBx := rectB.getX()
	bottomLeftBy := rectB.getBottom()
	bottomRightBx := rectB.getRight()
	halfWidthB := rectB.getWidthHalf()
	halfHeightB := rectB.getHeightHalf()

	// flag whether clipping points are found
	clipPointAFound := false
	clipPointBFound := false

	// line is vertical
	if p1x == p2x {
		if p1y > p2y {
			result[0] = p1x
			result[1] = topLeftAy
			result[2] = p2x
			result[3] = bottomLeftBy
			return false
		} else if p1y < p2y {
			result[0] = p1x
			result[1] = bottomLeftAy
			result[2] = p2x
			result[3] = topLeftBy
			return false
		}
		// not line, return null;
	} else if p1y == p2y {
		// line is horizontal
		if p1x > p2x {
			result[0] = topLeftAx
			result[1] = p1y
			result[2] = topRightBx
			result[3] = p2y
			return false
		} else if p1x < p2x {
			result[0] = topRightAx
			result[1] = p1y
			result[2] = topLeftBx
			result[3] = p2y
			return false
		}
		// not valid line, return null;
	} else {
		// slopes of rectA's and rectB's diagonals
		slopeA := rectA.height / rectA.width
		slopeB := rectB.height / rectB.width

		// slope of line between center of rectA and center of rectB
		slopePrime := (p2y - p1y) / (p2x - p1x)
		cardinalDirectionA := 0
		cardinalDirectionB := 0
		var tempPointAx, tempPointAy, tempPointBx, tempPointBy float64

		// determine whether clipping point is the corner of nodeA
		if -slopeA == slopePrime {
			if p1x > p2x {
				result[0] = bottomLeftAx
				result[1] = bottomLeftAy
				clipPointAFound = true
			} else {
				result[0] = topRightAx
				result[1] = topLeftAy
				clipPointAFound = true
			}
		} else if slopeA == slopePrime {
			if p1x > p2x {
				result[0] = topLeftAx
				result[1] = topLeftAy
				clipPointAFound = true
			} else {
				result[0] = bottomRightAx
				result[1] = bottomLeftAy
				clipPointAFound = true
			}
		}

		// determine whether clipping point is the corner of nodeB
		if -slopeB == slopePrime {
			if p2x > p1x {
				result[2] = bottomLeftBx
				result[3] = bottomLeftBy
				clipPointBFound = true
			} else {
				result[2] = topRightBx
				result[3] = topLeftBy
				clipPointBFound = true
			}
		} else if slopeB == slopePrime {
			if p2x > p1x {
				result[2] = topLeftBx
				result[3] = topLeftBy
				clipPointBFound = true
			} else {
				result[2] = bottomRightBx
				result[3] = bottomLeftBy
				clipPointBFound = true
			}
		}

		// if both clipping points are corners
		if clipPointAFound && clipPointBFound {
			return false
		}

		// determine Cardinal Direction of rectangles
		if p1x > p2x {
			if p1y > p2y {
				cardinalDirectionA = getCardinalDirection(slopeA, slopePrime, 4)
				cardinalDirectionB = getCardinalDirection(slopeB, slopePrime, 2)
			} else {
				cardinalDirectionA = getCardinalDirection(-slopeA, slopePrime, 3)
				cardinalDirectionB = getCardinalDirection(-slopeB, slopePrime, 1)
			}
		} else {
			if p1y > p2y {
				cardinalDirectionA = getCardinalDirection(-slopeA, slopePrime, 1)
				cardinalDirectionB = getCardinalDirection(-slopeB, slopePrime, 3)
			} else {
				cardinalDirectionA = getCardinalDirection(slopeA, slopePrime, 2)
				cardinalDirectionB = getCardinalDirection(slopeB, slopePrime, 4)
			}
		}
		// calculate clipping Point if it is not found before
		if !clipPointAFound {
			switch cardinalDirectionA {
			case 1:
				tempPointAy = topLeftAy
				tempPointAx = p1x + -halfHeightA/slopePrime
				result[0] = tempPointAx
				result[1] = tempPointAy
			case 2:
				tempPointAx = bottomRightAx
				tempPointAy = p1y + float64(halfWidthA*slopePrime)
				result[0] = tempPointAx
				result[1] = tempPointAy
			case 3:
				tempPointAy = bottomLeftAy
				tempPointAx = p1x + halfHeightA/slopePrime
				result[0] = tempPointAx
				result[1] = tempPointAy
			case 4:
				tempPointAx = bottomLeftAx
				tempPointAy = p1y + float64(-halfWidthA*slopePrime)
				result[0] = tempPointAx
				result[1] = tempPointAy
			}
		}
		if !clipPointBFound {
			switch cardinalDirectionB {
			case 1:
				tempPointBy = topLeftBy
				tempPointBx = p2x + -halfHeightB/slopePrime
				result[2] = tempPointBx
				result[3] = tempPointBy
			case 2:
				tempPointBx = bottomRightBx
				tempPointBy = p2y + float64(halfWidthB*slopePrime)
				result[2] = tempPointBx
				result[3] = tempPointBy
			case 3:
				tempPointBy = bottomLeftBy
				tempPointBx = p2x + halfHeightB/slopePrime
				result[2] = tempPointBx
				result[3] = tempPointBy
			case 4:
				tempPointBx = bottomLeftBx
				tempPointBy = p2y + float64(-halfWidthB*slopePrime)
				result[2] = tempPointBx
				result[3] = tempPointBy
			}
		}
	}
	return false
}

// getCardinalDirection is IGeometry.getCardinalDirection.
func getCardinalDirection(slope, slopePrime float64, line int) int {
	if slope > slopePrime {
		return line
	}
	return 1 + line%4
}
