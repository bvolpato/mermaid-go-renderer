// Ported from skrifa 0.46.2 (src/outline/autohint/outline.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/afhints.c: af_glyph_hints_reload, src/autofit/afhints.h),
// which has the FreeType License (LICENSE-FreeType.txt).

package font

// ahDirection is Direction: a hinting direction. The value of skrifa for no
// direction is 4. Here it is the zero value, and raw gives the value of
// skrifa.
type ahDirection int8

const (
	ahDirNone  ahDirection = 0
	ahDirRight ahDirection = 1
	ahDirLeft  ahDirection = -1
	ahDirUp    ahDirection = 2
	ahDirDown  ahDirection = -2
)

// raw returns the number of the direction in skrifa. Two directions are
// opposite when the sum of their numbers is zero.
func (d ahDirection) raw() int8 {
	if d == ahDirNone {
		return 4
	}
	return int8(d)
}

// newAHDirection is Direction::new: the direction of a vector.
func newAHDirection(dx, dy int32) ahDirection {
	var dir ahDirection
	var longArm, shortArm int32
	if dy >= dx {
		if dy >= -dx {
			dir, longArm, shortArm = ahDirUp, dy, dx
		} else {
			dir, longArm, shortArm = ahDirLeft, -dx, dy
		}
	} else if dy >= -dx {
		dir, longArm, shortArm = ahDirRight, dx, dy
	} else {
		dir, longArm, shortArm = ahDirDown, -dy, dx
	}
	// There is no direction when the lengths of the arms do not differ
	// enough.
	if longArm <= 14*ahAbs(shortArm) {
		return ahDirNone
	}
	return dir
}

func (d ahDirection) isOpposite(other ahDirection) bool {
	return d.raw()+other.raw() == 0
}

func (d ahDirection) isSameAxis(other ahDirection) bool {
	a, b := d.raw(), other.raw()
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	return a == b
}

func (d ahDirection) normalize() ahDirection {
	switch d {
	case ahDirLeft:
		return ahDirRight
	case ahDirDown:
		return ahDirUp
	}
	return d
}

// ahOrientation is the overall orientation of an outline. The zero value
// says that the outline has no orientation.
type ahOrientation uint8

const (
	ahOrientationNone ahOrientation = iota
	ahOrientationClockwise
	ahOrientationCounterClockwise
)

// ahPoint is Point: an outline point with the context for hinting.
type ahPoint struct {
	// flags has the type of the point and its hinting state.
	flags uint8
	// fx and fy are the coordinates in font units.
	fx, fy int32
	// ox and oy are the scaled coordinates.
	ox, oy int32
	// x and y are the hinted coordinates.
	x, y int32
	// inDir and outDir are the directions of the vectors into the point
	// and out of it.
	inDir, outDir ahDirection
	// u and v are coordinates that depend on the context.
	u, v int32
	// nextIx and prevIx are the indices of the next and the previous point
	// of the contour.
	nextIx, prevIx uint16
}

func (p *ahPoint) isOnCurve() bool { return p.flags&ahPointOnCurve != 0 }

func (p *ahPoint) hasMarker(marker uint8) bool { return p.flags&marker != 0 }

func (p *ahPoint) setMarker(marker uint8) { p.flags |= marker }

func (p *ahPoint) next() int { return int(p.nextIx) }

func (p *ahPoint) prev() int { return int(p.prevIx) }

// ahContour is Contour: the first and the last point of a contour.
type ahContour struct {
	firstIx, lastIx uint16
}

func (c ahContour) first() int { return int(c.firstIx) }

func (c ahContour) last() int { return int(c.lastIx) }

func (c ahContour) next(index int) int {
	if index >= int(c.lastIx) {
		return int(c.firstIx)
	}
	return index + 1
}

func (c ahContour) prev(index int) int {
	if index <= int(c.firstIx) {
		return int(c.lastIx)
	}
	return index - 1
}

// ahOutline is Outline: the outline of a glyph for the automatic hinter.
type ahOutline struct {
	unitsPerEm  int32
	orientation ahOrientation
	points      []ahPoint
	contours    []ahContour
	advance     int32
}

// fill is Outline::fill: it loads the outline of a glyph in font units and
// analyzes it. quirksAot selects the rules of the hinter that works ahead
// of time. skrifa uses the other rules for drawing.
func (o *ahOutline) fill(font *ahFont, glyph ahGlyfOutline, quirksAot bool) bool {
	o.clear()
	o.unitsPerEm = int32(font.unitsPerEm)
	points, advance, ok := font.drawUnscaled(glyph)
	if !ok {
		return false
	}
	for _, point := range points {
		if !o.push(point) {
			return false
		}
	}
	o.advance = advance
	return o.analyzeAndValidate(quirksAot)
}

// push is the push of UnscaledOutlineSink for Outline.
func (o *ahOutline) push(point ahUnscaledPoint) bool {
	newPoint := ahPoint{flags: point.flags, fx: int32(point.x), fy: int32(point.y)}
	if len(o.points) > 0xFFFF {
		return false
	}
	newPointIx := uint16(len(o.points))
	if point.isContourStart || len(o.contours) == 0 {
		o.contours = append(o.contours, ahContour{firstIx: newPointIx, lastIx: newPointIx})
	} else {
		o.contours[len(o.contours)-1].lastIx++
	}
	o.points = append(o.points, newPoint)
	return true
}

func (o *ahOutline) analyzeAndValidate(quirksAot bool) bool {
	const maxLen = 0xFFFF + 1
	if len(o.points) > maxLen || len(o.contours) > maxLen {
		return false
	}
	// The limit is a heuristic value.
	nearLimit := 20 * o.unitsPerEm / 2048
	o.linkPoints()
	o.markNearPoints(nearLimit)
	o.computeDirections(nearLimit)
	o.simplifyTopology()
	if quirksAot {
		o.checkRemainingWeakPoints(ahIsCornerFlatAot)
	} else {
		o.checkRemainingWeakPoints(ahIsCornerFlatJit)
	}
	o.computeOrientation()
	return true
}

// scale is Outline::scale: it applies the scale and the delta of each
// dimension to each point.
func (o *ahOutline) scale(scale *ahScale) {
	for i := range o.points {
		point := &o.points[i]
		x := ahFixedMul(point.fx, scale.xScale) + scale.xDelta
		y := ahFixedMul(point.fy, scale.yScale) + scale.yDelta
		point.ox = x
		point.x = x
		point.oy = y
		point.y = y
	}
}

func (o *ahOutline) clear() {
	o.unitsPerEm = 0
	o.points = o.points[:0]
	o.contours = o.contours[:0]
	o.advance = 0
}

// linkPoints is link_points: it sets the next and the previous index of each
// point.
func (o *ahOutline) linkPoints() {
	points := o.points
	for _, contour := range o.contours {
		firstIx := contour.first()
		if contour.last() >= len(points) || firstIx > contour.last() {
			continue
		}
		prevIx := contour.lastIx
		for ix := firstIx; ix <= contour.last(); ix++ {
			point := &points[ix]
			point.prevIx = prevIx
			prevIx = uint16(ix)
			point.nextIx = uint16(ix) + 1
		}
		points[contour.last()].nextIx = contour.firstIx
	}
}

// markNearPoints is mark_near_points: it marks a point when the distance to
// the next point is very small.
func (o *ahOutline) markNearPoints(nearLimit int32) {
	points := o.points
	for _, contour := range o.contours {
		prevIx := contour.last()
		for ix := contour.first(); ix <= contour.last(); ix++ {
			point := points[ix]
			prev := &points[prevIx]
			outX := point.fx - prev.fx
			outY := point.fy - prev.fy
			if ahAbs(outX)+ahAbs(outY) < nearLimit {
				prev.setMarker(ahMarkerNear)
			}
			prevIx = ix
		}
	}
}

// computeDirections is compute_directions: the directions of the vectors
// into each point and out of it.
func (o *ahOutline) computeDirections(nearLimit int32) {
	nearLimit2 := 2*nearLimit - 1
	points := o.points
	for _, contour := range o.contours {
		// Walk back to find the first point that is not near.
		firstIx := contour.first()
		ix := firstIx
		prevIx := contour.prev(firstIx)
		point := points[firstIx]
		for prevIx != firstIx {
			prev := points[prevIx]
			outX := point.fx - prev.fx
			outY := point.fy - prev.fy
			if ahAbs(outX)+ahAbs(outY) >= nearLimit2 {
				break
			}
			point = prev
			ix = prevIx
			prevIx = contour.prev(prevIx)
		}
		firstIx = ix
		// The fields u and v hold the indices of the next and the previous
		// point that is not near.
		first := &points[firstIx]
		first.u = int32(firstIx)
		first.v = int32(firstIx)
		nextIx := firstIx
		ix = firstIx
		// Loop over all points of the contour to compute the directions.
		var outX, outY int32
		for {
			pointIx := nextIx
			nextIx = contour.next(pointIx)
			point := points[pointIx]
			next := &points[nextIx]
			// Add the deltas until they pass the limit.
			outX += next.fx - point.fx
			outY += next.fy - point.fy
			if ahAbs(outX)+ahAbs(outY) < nearLimit {
				next.setMarker(ahMarkerWeakInterpolation)
				if nextIx == firstIx {
					break
				}
				continue
			}
			outDir := newAHDirection(outX, outY)
			next.inDir = outDir
			next.v = int32(ix)
			cur := &points[ix]
			cur.u = int32(nextIx)
			cur.outDir = outDir
			// Set the directions of all points between.
			for interIx := contour.next(ix); interIx != nextIx; interIx = contour.next(interIx) {
				point := &points[interIx]
				point.inDir = outDir
				point.outDir = outDir
			}
			ix = nextIx
			points[ix].u = int32(firstIx)
			points[firstIx].v = int32(ix)
			outX = 0
			outY = 0
			if nextIx == firstIx {
				break
			}
		}
	}
}

// simplifyTopology is simplify_topology: it makes points weak, so that the
// local extrema are found more reliably.
func (o *ahOutline) simplifyTopology() {
	points := o.points
	for i := range points {
		point := points[i]
		if point.hasMarker(ahMarkerWeakInterpolation) {
			continue
		}
		if point.inDir == ahDirNone && point.outDir == ahDirNone {
			uIndex := int(point.u)
			vIndex := int(point.v)
			nextU := points[uIndex]
			prevV := points[vIndex]
			inX := point.fx - prevV.fx
			inY := point.fy - prevV.fy
			outX := nextU.fx - point.fx
			outY := nextU.fy - point.fy
			if (inX^outX) >= 0 && (inY^outY) >= 0 {
				// Both vectors point into the same quadrant.
				points[i].setMarker(ahMarkerWeakInterpolation)
				points[vIndex].u = int32(uIndex)
				points[uIndex].v = int32(vIndex)
			}
		}
	}
}

// checkRemainingWeakPoints is check_remaining_weak_points.
func (o *ahOutline) checkRemainingWeakPoints(isCornerFlat func(inX, inY, outX, outY int32) bool) {
	points := o.points
	for i := range points {
		point := points[i]
		makeWeak := false
		if point.hasMarker(ahMarkerWeakInterpolation) {
			// The point is weak already.
			continue
		}
		if !point.isOnCurve() {
			// Control points are always weak.
			makeWeak = true
		} else if point.outDir == point.inDir {
			if point.outDir != ahDirNone {
				// The point is on a vertical or horizontal segment, but not
				// at its start or end.
				makeWeak = true
			} else {
				uIndex := int(point.u)
				vIndex := int(point.v)
				nextU := points[uIndex]
				prevV := points[vIndex]
				if isCornerFlat(point.fx-prevV.fx, point.fy-prevV.fy, nextU.fx-point.fx, nextU.fy-point.fy) {
					// One of the vectors is more dominant.
					makeWeak = true
					points[vIndex].u = int32(uIndex)
					points[uIndex].v = int32(vIndex)
				}
			}
		} else if point.inDir.isOpposite(point.outDir) {
			// The point forms a spike.
			makeWeak = true
		}
		if makeWeak {
			points[i].setMarker(ahMarkerWeakInterpolation)
		}
	}
}

// computeOrientation is compute_orientation: the overall winding order of
// the outline.
func (o *ahOutline) computeOrientation() {
	o.orientation = ahOrientationNone
	points := o.points
	if len(points) == 0 {
		return
	}
	var area int64
	for _, contour := range o.contours {
		lastIx := contour.last()
		firstIx := contour.first()
		prevX, prevY := int64(points[lastIx].fx), int64(points[lastIx].fy)
		for i := firstIx; i <= lastIx; i++ {
			x, y := int64(points[i].fx), int64(points[i].fy)
			area += (y - prevY) * (x + prevX)
			prevX, prevY = x, y
		}
	}
	switch {
	case area < 0:
		o.orientation = ahOrientationCounterClockwise
	case area > 0:
		o.orientation = ahOrientationClockwise
	}
}

// ahIsCornerFlatAot is is_corner_flat_aot: the version of ttfautohint.
func ahIsCornerFlatAot(inX, inY, outX, outY int32) bool {
	dIn := ahAbs(inX) + ahAbs(inY)
	dOut := ahAbs(outX) + ahAbs(outY)
	dCorner := ahAbs(inX+outX) + ahAbs(inY+outY)
	return (dIn + dOut - dCorner) < (dCorner >> 4)
}

// ahIsCornerFlatJit is is_corner_flat_jit: the version of FreeType.
func ahIsCornerFlatJit(inX, inY, outX, outY int32) bool {
	ax := inX + outX
	ay := inY + outY
	hypot := func(x, y int32) int32 {
		x = ahAbs(x)
		y = ahAbs(y)
		if x > y {
			return x + ((3 * y) >> 3)
		}
		return y + ((3 * x) >> 3)
	}
	dIn := hypot(inX, inY)
	dOut := hypot(outX, outY)
	dHypot := hypot(ax, ay)
	return (dIn + dOut - dHypot) < (dHypot >> 4)
}

// pathBounds returns the bounds of the points of the path that skrifa and
// the pen of Skia make from the hinted outline (Outline::to_path), in 26.6
// px with y up.
func (o *ahOutline) pathBounds() ftCBox {
	var bounds ahPathBounds
	for _, contour := range o.contours {
		first, last := contour.first(), contour.last()
		if last >= len(o.points) || first > last {
			continue
		}
		points := o.points[first : last+1]
		bounds.addContour(len(points), func(i int) (int32, int32) { return points[i].x, points[i].y })
	}
	return bounds.box
}
