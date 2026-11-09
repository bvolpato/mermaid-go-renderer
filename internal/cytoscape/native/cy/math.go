// Package cy is a Go port of the parts of cytoscape 3.34.0 (MIT license,
// dist/cytoscape.esm.mjs as mermaid 12.2.0 bundles it) that decide the
// numbers that mermaid reads after a layout: the geometry of the edges that
// the canvas renderer keeps in its scratch, the bounds of compound nodes and
// the bounding boxes.
//
// This file is ported from src/math.mjs.
package cy

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// nan stands for a member of an array that JavaScript leaves undefined and
// then uses in arithmetic.
var nan = math.NaN()

// IntersectLineEllipse is intersectLineEllipse: the point where the line
// from (x, y) to the centre of the ellipse enters the ellipse. The
// JavaScript returns an empty array when (x, y) is inside the ellipse: the
// result is then (NaN, NaN), which is what the callers compute with.
func IntersectLineEllipse(x, y, centerX, centerY, ellipseWradius, ellipseHradius float64) (float64, float64) {
	dispX := centerX - x
	dispY := centerY - y
	dispX /= ellipseWradius
	dispY /= ellipseHradius
	length := math.Sqrt(float64(dispX*dispX) + float64(dispY*dispY))
	newLength := length - 1
	if newLength < 0 {
		return nan, nan
	}
	lenProportion := newLength / length
	return float64((centerX-x)*lenProportion) + x, float64((centerY-y)*lenProportion) + y
}

// CheckInEllipse is checkInEllipse.
func CheckInEllipse(x, y, width, height, centerX, centerY, padding float64) bool {
	x -= centerX
	y -= centerY
	x /= float64(width/2) + padding
	y /= float64(height/2) + padding
	return float64(x*x)+float64(y*y) <= 1
}

// ShortenIntersection is shortenIntersection: the point on the line from
// offset to intersection that is amount before the intersection.
func ShortenIntersection(intersectionX, intersectionY, offsetX, offsetY, amount float64) (float64, float64) {
	dispX, dispY := intersectionX-offsetX, intersectionY-offsetY
	length := math.Sqrt(float64(dispX*dispX) + float64(dispY*dispY))
	lenRatio := (length - amount) / length
	if lenRatio < 0 {
		lenRatio = 0.00001
	}
	return offsetX + float64(lenRatio*dispX), offsetY + float64(lenRatio*dispY)
}

// makeBoundingBox is makeBoundingBox() without an argument.
func makeBoundingBox() *BB {
	return &BB{X1: math.Inf(1), Y1: math.Inf(1), X2: math.Inf(-1), Y2: math.Inf(-1)}
}

// expandBoundingBox is expandBoundingBox.
func expandBoundingBox(bb *BB, padding float64) {
	bb.X1 -= padding
	bb.X2 += padding
	bb.Y1 -= padding
	bb.Y2 += padding
	bb.W = bb.X2 - bb.X1
	bb.H = bb.Y2 - bb.Y1
}

// assignBoundingBox assigns the values of bb2 into bb1.
func assignBoundingBox(bb1, bb2 *BB) {
	bb1.X1 = bb2.X1
	bb1.Y1 = bb2.Y1
	bb1.X2 = bb2.X2
	bb1.Y2 = bb2.Y2
	bb1.W = bb1.X2 - bb1.X1
	bb1.H = bb1.Y2 - bb1.Y1
}

// hashIntsArray is hashIntsArray with the default seed: the sdbm string
// hash over numbers.
func hashIntsArray(ints []float64) int32 {
	return hashIterable(defaultHashSeed, ints)
}

const (
	defaultHashSeed = 9261
	hashK           = 65599
)

func hashIterable(seed int32, values []float64) int32 {
	hash := seed
	for _, v := range values {
		// hash = hash * K + entry.value | 0
		hash = js.ToInt32(float64(float64(hash)*hashK) + v)
	}
	return hash
}
