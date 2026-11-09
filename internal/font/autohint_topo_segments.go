// Ported from skrifa 0.46.2 (src/outline/autohint/topo/segments.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/aflatin.c: af_latin_hints_compute_segments and
// af_latin_hints_link_segments, src/autofit/afcjk.c:
// af_cjk_hints_link_segments), which has the FreeType License
// (LICENSE-FreeType.txt).

package font

// A segment is a series of at least two consecutive points that are aligned
// along a coordinate axis. The linking stage joins pairs of segments to
// stems, and it finds serifs in a pass after that.

// The bounds of the score, the position and the coordinate values.
const (
	ahMaxScore int32 = 32000
	ahMinScore int32 = -32000
)

// ahComputeSegments is compute_segments: the segments of an outline for the
// Latin writing system. It returns false for an outline with too many
// segments.
func ahComputeSegments(outline *ahOutline, axis *ahAxis, _ ahScriptGroup) bool {
	ahAssignPointUVs(outline, axis.dim)
	if !ahBuildSegments(outline, axis) {
		return false
	}
	ahAdjustSegmentHeights(outline, axis)
	// The detection of round segments for CJK is never executed, because of
	// a bug in FreeType.
	return true
}

// ahLinkSegments is link_segments: it links segments to stems and serifs. A
// maxWidth that is not 0 refines the scoring function.
func ahLinkSegments(outline *ahOutline, axis *ahAxis, scale int32, group ahScriptGroup, maxWidth int32) {
	if group == ahGroupDefault {
		ahLinkSegmentsDefault(outline, axis, maxWidth)
	} else {
		ahLinkSegmentsCjk(outline, axis, scale)
	}
}

// ahLinkSegmentsDefault is link_segments_default.
func ahLinkSegmentsDefault(outline *ahOutline, axis *ahAxis, maxWidth int32) {
	// A heuristic value: the least overlap.
	lenThreshold := max(ahDerivedConstant(outline.unitsPerEm, 8), 1)
	// A heuristic value: the weight of lengths.
	lenScore := ahDerivedConstant(outline.unitsPerEm, 6000)
	// A heuristic value: the weight of distances. It is not a Latin
	// constant, because it works on multiples of the stem width.
	const distScore = 3000
	// Compare each segment with the others.
	segments := axis.segments
	for ix1 := range segments {
		if segments[ix1].dir != axis.majorDir {
			continue
		}
		pos1 := int32(segments[ix1].pos)
		// Search for stems that have the opposite direction, with seg1 to
		// the left of seg2.
		for ix2 := range segments {
			seg1 := segments[ix1]
			seg2 := segments[ix2]
			pos2 := int32(seg2.pos)
			if seg1.dir.isOpposite(seg2.dir) && pos2 > pos1 {
				// The distance between the segments. The functions min and
				// max are as intended.
				minCoord := int32(max(seg1.minCoord, seg2.minCoord))
				maxCoord := int32(min(seg1.maxCoord, seg2.maxCoord))
				// The overlap of the segments.
				length := maxCoord - minCoord
				if length >= lenThreshold {
					// From FreeType: the score is the sum of two demerits
					// that tell how bad a fit is, measured along the main
					// axis of the segments and orthogonal to it. Less
					// overlap along the main axis gives a larger demerit.
					// An orthogonal distance near a stem width gives a
					// smaller demerit. The demerit grows only for values
					// that exceed the largest stem width.
					dist := pos2 - pos1
					var distDemerit int32
					if maxWidth != 0 {
						// The demerits of distances are multiples of
						// maxWidth.
						delta := (dist<<10)/maxWidth - (1 << 10)
						if delta > 10_000 {
							distDemerit = ahMaxScore
						} else if delta > 0 {
							distDemerit = delta * delta / distScore
						} else {
							distDemerit = 0
						}
					} else {
						distDemerit = dist
					}
					score := distDemerit + lenScore/length
					if score < seg1.score {
						segments[ix1].score = score
						segments[ix1].linkIx = ahSome(ix2)
					}
					if score < seg2.score {
						segments[ix2].score = score
						segments[ix2].linkIx = ahSome(ix1)
					}
				}
			}
		}
	}
	// Compute the serif segments.
	for ix1 := range segments {
		if !segments[ix1].linkIx.ok {
			continue
		}
		ix2 := int(segments[ix1].linkIx.ix)
		seg2Link := segments[ix2].linkIx
		if seg2Link != ahSome(ix1) {
			segments[ix1].linkIx = ahOptIx{}
			segments[ix1].serifIx = seg2Link
		}
	}
}

// ahLinkSegmentsCjk is link_segments_cjk: it links segments to stems and
// serifs for the CJK script group.
func ahLinkSegmentsCjk(outline *ahOutline, axis *ahAxis, scale int32) {
	// A heuristic value: the least overlap.
	lenThreshold := ahDerivedConstant(outline.unitsPerEm, 8)
	distThreshold := ahFixedDiv(64*3, scale)
	// Compare each segment with the others.
	segments := axis.segments
	for ix1 := range segments {
		if segments[ix1].dir != axis.majorDir {
			continue
		}
		pos1 := int32(segments[ix1].pos)
		// Search for stems that have the opposite direction, with seg1 to
		// the left of seg2.
		for ix2 := range segments {
			seg1 := segments[ix1]
			seg2 := segments[ix2]
			if ix1 == ix2 || !seg1.dir.isOpposite(seg2.dir) {
				continue
			}
			pos2 := int32(seg2.pos)
			dist := pos2 - pos1
			if dist < 0 {
				continue
			}
			// The distance between the segments. The functions min and max
			// are as intended.
			minCoord := int32(max(seg1.minCoord, seg2.minCoord))
			maxCoord := int32(min(seg1.maxCoord, seg2.maxCoord))
			// The overlap of the segments.
			length := maxCoord - minCoord
			if length >= lenThreshold {
				// Some more heuristics of FreeType.
				checkSeg := func(seg *ahSegment) bool {
					return (dist*8 < seg.score*9) && (dist*8 < seg.score*7 || seg.len < length)
				}
				if checkSeg(&seg1) {
					seg := &segments[ix1]
					seg.score = dist
					seg.len = length
					seg.linkIx = ahSome(ix2)
				}
				if checkSeg(&seg2) {
					seg := &segments[ix2]
					seg.score = dist
					seg.len = length
					seg.linkIx = ahSome(ix1)
				}
			}
		}
	}
	// Compute the serif segments.
	for ix1 := range segments {
		seg1 := segments[ix1]
		if seg1.score >= distThreshold {
			continue
		}
		if !seg1.linkIx.ok || int(seg1.linkIx.ix) >= len(segments) {
			continue
		}
		link1Ix := int(seg1.linkIx.ix)
		link1 := segments[link1Ix]
		if link1.linkIx != ahSome(ix1) || link1.pos <= seg1.pos {
			continue
		}
		for ix2 := range segments {
			seg2 := segments[ix2]
			if seg2.pos > seg1.pos || ix1 == ix2 {
				continue
			}
			if !seg2.linkIx.ok || int(seg2.linkIx.ix) >= len(segments) {
				continue
			}
			link2 := segments[seg2.linkIx.ix]
			if link2.linkIx != ahSome(ix2) || link2.pos < link1.pos {
				continue
			}
			if seg1.pos == seg2.pos && link1.pos == link2.pos {
				continue
			}
			if seg2.score <= seg1.score || seg1.score*4 <= seg2.score {
				continue
			}
			if seg1.len >= seg2.len*3 {
				link2Ix := int(seg2.linkIx.ix)
				for i := range segments {
					seg := &segments[i]
					linkIx := seg.linkIx
					if linkIx == ahSome(ix2) {
						seg.linkIx = ahOptIx{}
						seg.serifIx = ahSome(link1Ix)
					} else if linkIx == ahSome(link2Ix) {
						seg.linkIx = ahOptIx{}
						seg.serifIx = ahSome(ix1)
					}
				}
			} else {
				segments[ix1].linkIx = ahOptIx{}
				segments[link1Ix].linkIx = ahOptIx{}
				break
			}
		}
	}
	for ix1 := range segments {
		seg1 := segments[ix1]
		if !seg1.linkIx.ok || int(seg1.linkIx.ix) >= len(segments) {
			continue
		}
		seg2 := segments[seg1.linkIx.ix]
		if seg2.linkIx != ahSome(ix1) {
			segments[ix1].linkIx = ahOptIx{}
			if seg2.score < distThreshold || seg1.score < seg2.score*4 {
				segments[ix1].serifIx = seg2.linkIx
			}
		}
	}
}

// ahAssignPointUVs is assign_point_uvs: it sets the values u and v of each
// point to the coordinates in font units, for the dimension of the axis.
func ahAssignPointUVs(outline *ahOutline, dim ahDimension) {
	if dim == ahHorizontal {
		for i := range outline.points {
			point := &outline.points[i]
			point.u = point.fx
			point.v = point.fy
		}
	} else {
		for i := range outline.points {
			point := &outline.points[i]
			point.u = point.fy
			point.v = point.fx
		}
	}
}

// ahSegmentState is State: the current and the previous state while the
// segments are computed. The values along a segment (v of a point) are
// coordinates, and the values orthogonal to it (u of a point) are positions.
type ahSegmentState struct {
	minPos, maxPos         int32
	minCoord, maxCoord     int32
	minFlags, maxFlags     uint8
	minOnCoord, maxOnCoord int32
}

func newAHSegmentState() ahSegmentState {
	return ahSegmentState{
		minPos:     ahMaxScore,
		maxPos:     ahMinScore,
		minCoord:   ahMaxScore,
		maxCoord:   ahMinScore,
		minOnCoord: ahMaxScore,
		maxOnCoord: ahMinScore,
	}
}

// applyToSegment is State::apply_to_segment.
func (s *ahSegmentState) applyToSegment(segment *ahSegment, flatThreshold int32) {
	segment.pos = ahComputeMidPos(s.minPos, s.maxPos)
	segment.delta = ahComputeMidDelta(s.minPos, s.maxPos)
	// A segment is round when one end point is a control point, and the
	// length of the points on the curve between them is in a heuristic
	// limit.
	if (s.minFlags&ahPointOnCurve == 0 || s.maxFlags&ahPointOnCurve == 0) &&
		(s.maxOnCoord-s.minOnCoord) < flatThreshold {
		segment.flags |= ahTopoRound
	}
	segment.minCoord = int16(s.minCoord)
	segment.maxCoord = int16(s.maxCoord)
	segment.height = int16(s.maxCoord - s.minCoord)
}

// ahComputeMidPos is compute_mid_pos: the position in the middle of two
// values.
func ahComputeMidPos(minPos, maxPos int32) int16 { return int16((minPos + maxPos) >> 1) }

// ahComputeMidDelta is compute_mid_delta: half the distance of two values.
func ahComputeMidDelta(minPos, maxPos int32) int16 { return int16((maxPos - minPos) >> 1) }

// ahBuildSegments is build_segments: the segments of each contour. It
// returns false for an outline with too many segments.
func ahBuildSegments(outline *ahOutline, axis *ahAxis) bool {
	flatThreshold := outline.unitsPerEm / 14
	axis.segments = axis.segments[:0]
	majorDir := axis.majorDir.normalize()
	segmentDir := majorDir
	points := outline.points
	for _, contour := range outline.contours {
		isSinglePointContour := contour.last()+1-contour.first() == 1
		pointIx := contour.first()
		lastIx := contour.prev(pointIx)
		state := newAHSegmentState()
		prevState := state
		// prevSegmentIx is the segment before, or -1.
		prevSegmentIx := -1
		segmentIx := 0
		// When the contour starts on an edge, find the starting point.
		if points[pointIx].outDir.isSameAxis(majorDir) && points[lastIx].outDir.isSameAxis(majorDir) {
			lastIx = pointIx
			for {
				pointIx = contour.prev(pointIx)
				if !points[pointIx].outDir.isSameAxis(majorDir) {
					pointIx = contour.next(pointIx)
					break
				}
				if pointIx == lastIx {
					break
				}
			}
		}
		lastIx = pointIx
		onEdge := false
		passed := false
		for {
			if onEdge {
				// The least and the largest position.
				point := points[pointIx]
				state.minPos = min(state.minPos, point.u)
				state.maxPos = max(state.maxPos, point.u)
				// The least and the largest coordinate, and the flags.
				v := point.v
				if v < state.minCoord {
					state.minCoord = v
					state.minFlags = point.flags
				}
				if v > state.maxCoord {
					state.maxCoord = v
					state.maxFlags = point.flags
				}
				// The least and the largest coordinate of the points on the
				// curve.
				if point.isOnCurve() {
					state.minOnCoord = min(state.minOnCoord, point.v)
					state.maxOnCoord = max(state.maxOnCoord, point.v)
				}
				if point.outDir != segmentDir || pointIx == lastIx {
					if prevSegmentIx >= 0 && axis.segments[segmentIx].firstIx != axis.segments[prevSegmentIx].lastIx {
						prevSegmentIx = -1
					}
					if prevSegmentIx >= 0 {
						// The points are the same, so merge the segments.
						prevSegment := &axis.segments[prevSegmentIx]
						if points[prevSegment.last()].inDir == point.inDir {
							// The directions are identical: unify the
							// segments and update the constraints.
							state.minPos = min(prevState.minPos, state.minPos)
							state.maxPos = max(prevState.maxPos, state.maxPos)
							if prevState.minCoord < state.minCoord {
								state.minCoord = prevState.minCoord
								state.minFlags = prevState.minFlags
							}
							if prevState.maxCoord > state.maxCoord {
								state.maxCoord = prevState.maxCoord
								state.maxFlags = prevState.maxFlags
							}
							state.minOnCoord = min(prevState.minOnCoord, state.minOnCoord)
							state.maxOnCoord = max(prevState.maxOnCoord, state.maxOnCoord)
							prevSegment.lastIx = uint16(pointIx)
							state.applyToSegment(prevSegment, flatThreshold)
						} else {
							// The directions are different: use the
							// properties of the longer segment.
							if ahAbs(prevState.maxCoord-prevState.minCoord) > ahAbs(state.maxCoord-state.minCoord) {
								// Discard the current segment.
								prevState.minPos = min(prevState.minPos, state.minPos)
								prevState.maxPos = max(prevState.maxPos, state.maxPos)
								prevSegment.lastIx = uint16(pointIx)
								prevSegment.pos = ahComputeMidPos(prevState.minPos, prevState.maxPos)
								prevSegment.delta = ahComputeMidDelta(prevState.minPos, prevState.maxPos)
							} else {
								// Discard the previous segment.
								state.minPos = min(state.minPos, prevState.minPos)
								state.maxPos = max(state.maxPos, prevState.maxPos)
								segment := axis.segments[segmentIx]
								segment.lastIx = uint16(pointIx)
								state.applyToSegment(&segment, flatThreshold)
								axis.segments[prevSegmentIx] = segment
								prevState = state
							}
						}
						axis.segments = axis.segments[:len(axis.segments)-1]
					} else {
						// The points are different: the contour leaves an
						// edge, so there is a new segment.
						segment := &axis.segments[segmentIx]
						segment.lastIx = uint16(pointIx)
						state.applyToSegment(segment, flatThreshold)
						prevSegmentIx = segmentIx
						prevState = state
					}
					onEdge = false
				}
			}
			if pointIx == lastIx {
				if passed {
					break
				}
				passed = true
			}
			point := points[pointIx]
			if !onEdge && (point.outDir.isSameAxis(majorDir) || isSinglePointContour) {
				if len(axis.segments) > 1000 {
					axis.segments = axis.segments[:0]
					return false
				}
				segmentIx = len(axis.segments)
				segmentDir = point.outDir
				segment := ahSegment{
					dir:     segmentDir,
					firstIx: uint16(pointIx),
					lastIx:  uint16(pointIx),
					score:   ahMaxScore,
				}
				state.minPos = point.u
				state.maxPos = point.u
				state.minCoord = point.v
				state.maxCoord = point.v
				state.minFlags = point.flags
				state.maxFlags = point.flags
				if !point.isOnCurve() {
					state.minOnCoord = ahMaxScore
					state.maxOnCoord = ahMinScore
				} else {
					state.minOnCoord = point.v
					state.maxOnCoord = point.v
				}
				onEdge = true
				if isSinglePointContour {
					segment.pos = int16(state.minPos)
					if !point.isOnCurve() {
						segment.flags |= ahTopoRound
					}
					segment.minCoord = int16(point.v)
					segment.maxCoord = int16(point.v)
					segment.height = 0
					onEdge = false
				}
				axis.segments = append(axis.segments, segment)
			}
			pointIx = contour.next(pointIx)
		}
	}
	return true
}

// ahAdjustSegmentHeights is adjust_segment_heights: it makes the height of
// segments a little larger when that helps to find and ignore serifs.
func ahAdjustSegmentHeights(outline *ahOutline, axis *ahAxis) {
	points := outline.points
	for i := range axis.segments {
		segment := &axis.segments[i]
		first := &points[segment.first()]
		last := &points[segment.last()]
		prev := &points[first.prev()]
		next := &points[last.next()]
		if first.v < last.v {
			if prev.v < first.v {
				segment.adjustHeight(first.v, prev.v)
			}
			if next.v > last.v {
				segment.adjustHeight(next.v, last.v)
			}
		} else {
			if prev.v > first.v {
				segment.adjustHeight(prev.v, first.v)
			}
			if next.v < last.v {
				segment.adjustHeight(last.v, next.v)
			}
		}
	}
}
