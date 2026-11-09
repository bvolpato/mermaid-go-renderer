// Ported from skrifa 0.46.2 (src/outline/autohint/hint/outline.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/afhints.c: af_glyph_hints_align_edge_points,
// af_glyph_hints_align_strong_points, af_glyph_hints_align_weak_points),
// which has the FreeType License (LICENSE-FreeType.txt).

package font

// The edge hints are applied to an outline in three passes:
//
//  1. Align the points that are attached to edges. These are the points
//     that made the edge. They can include points that were classified as
//     weak.
//  2. Interpolate the points that are not weak and that the pass before did
//     not touch. The pass searches the edges that enclose the point, and it
//     interpolates the coordinate from the adjustment of those edges.
//  3. Interpolate the points that remain. These are the weak points: the
//     points that are very near other points, or that have no dominant
//     direction inward or outward.

// ahAlignEdgePoints is align_edge_points: it aligns all points of an edge to
// the same coordinate value.
func ahAlignEdgePoints(outline *ahOutline, axis *ahAxis, group ahScriptGroup, scale *ahScale) {
	edges := axis.edges
	segments := axis.segments
	points := outline.points
	// The snapping is an option for CJK.
	snap := group == ahGroupDefault ||
		(axis.dim == ahHorizontal && scale.flags.contains(ahScaleHorizontalSnap)) ||
		(axis.dim == ahVertical && scale.flags.contains(ahScaleVerticalSnap))
	for i := range segments {
		segment := &segments[i]
		if !segment.edgeIx.ok || int(segment.edgeIx.ix) >= len(edges) {
			continue
		}
		edge := &edges[segment.edgeIx.ix]
		delta := edge.pos - edge.opos
		pointIx := segment.first()
		lastIx := segment.last()
		for {
			if pointIx >= len(points) {
				return
			}
			point := &points[pointIx]
			if axis.dim == ahHorizontal {
				if snap {
					point.x = edge.pos
				} else {
					point.x += delta
				}
				point.setMarker(ahMarkerTouchedX)
			} else {
				if snap {
					point.y = edge.pos
				} else {
					point.y += delta
				}
				point.setMarker(ahMarkerTouchedY)
			}
			if pointIx == lastIx {
				break
			}
			pointIx = point.next()
		}
	}
}

// ahAlignStrongPoints is align_strong_points: it aligns the strong points,
// as the TrueType instruction IP does.
func ahAlignStrongPoints(outline *ahOutline, axis *ahAxis) {
	if len(axis.edges) == 0 {
		return
	}
	dim := axis.dim
	touchFlag := ahMarkerTouchedY
	if dim == ahHorizontal {
		touchFlag = ahMarkerTouchedX
	}
	points := outline.points
	edges := axis.edges
points:
	for pointIx := range points {
		point := &points[pointIx]
		// Skip the points that are touched already. The weak points are
		// interpolated in the next pass.
		if point.hasMarker(touchFlag | ahMarkerWeakInterpolation) {
			continue
		}
		u, ou := point.fx, point.ox
		if dim == ahVertical {
			u, ou = point.fy, point.oy
		}
		// Is the point before the first edge?
		edge := &edges[0]
		delta := int32(edge.fpos) - u
		if delta >= 0 {
			ahStorePoint(point, dim, edge.pos-(edge.opos-ou))
			continue
		}
		// Is the point after the last edge?
		edge = &edges[len(edges)-1]
		delta = u - int32(edge.fpos)
		if delta >= 0 {
			ahStorePoint(point, dim, edge.pos+(ou-edge.opos))
			continue
		}
		// Find the edges that enclose the point. A small number of edges
		// uses a linear search. This is critical when more than one edge has
		// the same position in font units: then the linear and the binary
		// search can give different results.
		minIx := 0
		if len(edges) <= 8 {
			for ix := range edges {
				if int32(edges[ix].fpos) >= u {
					if int32(edges[ix].fpos) == u {
						ahStorePoint(point, dim, edges[ix].pos)
						continue points
					}
					minIx = ix
					break
				}
			}
		} else {
			maxIx := len(edges)
			for minIx < maxIx {
				midIx := (minIx + maxIx) >> 1
				edge := &edges[midIx]
				fpos := int32(edge.fpos)
				if u < fpos {
					maxIx = midIx
				} else if u > fpos {
					minIx = midIx + 1
				} else {
					// The point is on an edge.
					ahStorePoint(point, dim, edge.pos)
					continue points
				}
			}
		}
		// The point is not on an edge.
		if minIx >= 1 {
			beforeIx := minIx - 1
			edgeBefore := &edges[beforeIx]
			beforePos := edgeBefore.pos
			beforeFpos := int32(edgeBefore.fpos)
			var scale int32
			if edgeBefore.scale == 0 {
				if minIx >= len(edges) {
					return
				}
				edgeAfter := &edges[minIx]
				scale = ahFixedDiv(edgeAfter.pos-edgeBefore.pos, int32(edgeAfter.fpos)-beforeFpos)
				edges[beforeIx].scale = scale
			} else {
				scale = edgeBefore.scale
			}
			ahStorePoint(point, dim, beforePos+ahFixedMul(u-beforeFpos, scale))
		}
	}
}

// ahAlignWeakPoints is align_weak_points: it aligns the weak points, as the
// TrueType instruction IUP does.
func ahAlignWeakPoints(outline *ahOutline, dim ahDimension) {
	var touchMarker uint8
	if dim == ahHorizontal {
		for i := range outline.points {
			point := &outline.points[i]
			point.u = point.x
			point.v = point.ox
		}
		touchMarker = ahMarkerTouchedX
	} else {
		for i := range outline.points {
			point := &outline.points[i]
			point.u = point.y
			point.v = point.oy
		}
		touchMarker = ahMarkerTouchedY
	}
	for _, contour := range outline.contours {
		if contour.last() >= len(outline.points) || contour.first() > contour.last() {
			return
		}
		points := outline.points[contour.first() : contour.last()+1]
		// Find the first touched point.
		firstTouchedIx := -1
		for i := range points {
			if points[i].hasMarker(touchMarker) {
				firstTouchedIx = i
				break
			}
		}
		if firstTouchedIx < 0 {
			continue
		}
		lastIx := len(points) - 1
		pointIx := firstTouchedIx
		var lastTouchedIx int
	outer:
		for {
			// Skip the touched neighbors.
			for pointIx < lastIx && points[pointIx+1].hasMarker(touchMarker) {
				pointIx++
			}
			lastTouchedIx = pointIx
			// Find the next touched point.
			pointIx++
			for {
				if pointIx > lastIx {
					break outer
				}
				if points[pointIx].hasMarker(touchMarker) {
					break
				}
				pointIx++
			}
			ahIupInterpolate(points, lastTouchedIx+1, pointIx-1, lastTouchedIx, pointIx)
		}
		if lastTouchedIx == firstTouchedIx {
			// A special case: only one point was touched.
			ahIupShift(points, 0, lastIx, firstTouchedIx)
		} else {
			// Interpolate the points that remain.
			if lastTouchedIx < lastIx {
				ahIupInterpolate(points, lastTouchedIx+1, lastIx, lastTouchedIx, firstTouchedIx)
			}
			if firstTouchedIx > 0 {
				ahIupInterpolate(points, 0, firstTouchedIx-1, lastTouchedIx, firstTouchedIx)
			}
		}
	}
	// Save the interpolated values.
	if dim == ahHorizontal {
		for i := range outline.points {
			outline.points[i].x = outline.points[i].u
		}
	} else {
		for i := range outline.points {
			outline.points[i].y = outline.points[i].u
		}
	}
}

// ahStorePoint is store_point.
func ahStorePoint(point *ahPoint, dim ahDimension, u int32) {
	if dim == ahHorizontal {
		point.x = u
		point.setMarker(ahMarkerTouchedX)
	} else {
		point.y = u
		point.setMarker(ahMarkerTouchedY)
	}
}

// ahIupShift is iup_shift: it shifts the original coordinates of all points
// from p1Ix to p2Ix to get hinted coordinates, with the difference of the
// point at refIx. The fields u and v are the current and the original
// coordinate.
func ahIupShift(points []ahPoint, p1Ix, p2Ix, refIx int) {
	if refIx >= len(points) {
		return
	}
	refPoint := &points[refIx]
	delta := refPoint.u - refPoint.v
	if delta == 0 {
		return
	}
	if p1Ix > refIx {
		return
	}
	for i := p1Ix; i < refIx; i++ {
		points[i].u = points[i].v + delta
	}
	if refIx+1 > p2Ix+1 || p2Ix >= len(points) {
		return
	}
	for i := refIx + 1; i <= p2Ix; i++ {
		points[i].u = points[i].v + delta
	}
}

// ahIupInterpolate is iup_interpolate: it interpolates the original
// coordinates of all points from p1Ix to p2Ix to get hinted coordinates,
// with the points at ref1Ix and ref2Ix as the reference points. The fields
// u and v are the current and the original coordinate.
func ahIupInterpolate(points []ahPoint, p1Ix, p2Ix, ref1Ix, ref2Ix int) {
	if p1Ix > p2Ix {
		return
	}
	if ref1Ix >= len(points) || ref2Ix >= len(points) || p2Ix >= len(points) {
		return
	}
	refPoint1 := &points[ref1Ix]
	refPoint2 := &points[ref2Ix]
	if refPoint1.v > refPoint2.v {
		refPoint1, refPoint2 = refPoint2, refPoint1
	}
	u1, v1 := refPoint1.u, refPoint1.v
	u2, v2 := refPoint2.u, refPoint2.v
	d1 := u1 - v1
	d2 := u2 - v2
	if u1 == u2 || v1 == v2 {
		for i := p1Ix; i <= p2Ix; i++ {
			point := &points[i]
			if point.v <= v1 {
				point.u = point.v + d1
			} else if point.v >= v2 {
				point.u = point.v + d2
			} else {
				point.u = u1
			}
		}
	} else {
		scale := ahFixedDiv(u2-u1, v2-v1)
		for i := p1Ix; i <= p2Ix; i++ {
			point := &points[i]
			if point.v <= v1 {
				point.u = point.v + d1
			} else if point.v >= v2 {
				point.u = point.v + d2
			} else {
				point.u = u1 + ahFixedMul(point.v-v1, scale)
			}
		}
	}
}
