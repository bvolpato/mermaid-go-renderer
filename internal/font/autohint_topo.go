// Ported from skrifa 0.46.2 (src/outline/autohint/topo/mod.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/afhints.h, src/autofit/afhints.c), which has the FreeType
// License (LICENSE-FreeType.txt).

package font

// ahOptIx is an index that can be absent: Option<u16> of skrifa. The zero
// value is the absent index.
type ahOptIx struct {
	ix uint16
	ok bool
}

// ahSome returns a present index.
func ahSome(ix int) ahOptIx { return ahOptIx{ix: uint16(ix), ok: true} }

// Flags of segments and edges (TopoFlags).
const (
	// ahTopoNormal is a regular segment or edge.
	ahTopoNormal uint8 = 0
	// ahTopoRound says that the segment or edge has round geometry.
	ahTopoRound uint8 = 1
	// ahTopoSerif says that the segment or edge is a serif.
	ahTopoSerif uint8 = 2
	// ahTopoDone says that the segment or edge is processed.
	ahTopoDone uint8 = 4
	// ahTopoNeutral says that the segment or edge aligns to a neutral blue
	// zone.
	ahTopoNeutral uint8 = 8
)

// ahAxis is Axis: the segments and the edges of one dimension of an
// outline.
type ahAxis struct {
	// dim is horizontal or vertical.
	dim ahDimension
	// majorDir depends on the dimension and on the orientation of the
	// outline.
	majorDir ahDirection
	segments []ahSegment
	edges    []ahEdge
}

// reset is Axis::reset.
func (a *ahAxis) reset(dim ahDimension, orientation ahOrientation) {
	a.dim = dim
	switch {
	case dim == ahHorizontal && orientation == ahOrientationClockwise:
		a.majorDir = ahDirDown
	case dim == ahVertical && orientation == ahOrientationClockwise:
		a.majorDir = ahDirRight
	case dim == ahHorizontal:
		a.majorDir = ahDirUp
	default:
		a.majorDir = ahDirLeft
	}
	a.segments = a.segments[:0]
	a.edges = a.edges[:0]
}

// insertEdge is Axis::insert_edge: it puts an edge into the sorted list of
// edges.
func (a *ahAxis) insertEdge(edge ahEdge, topToBottomHinting bool) {
	a.edges = append(a.edges, edge)
	edges := a.edges
	// The first edge is in place.
	if len(edges) == 1 {
		return
	}
	// Move the edge into place.
	ix := len(edges) - 1
	for ix > 0 {
		prevIx := ix - 1
		prevFpos := edges[prevIx].fpos
		if (topToBottomHinting && prevFpos > edge.fpos) || (!topToBottomHinting && prevFpos < edge.fpos) {
			break
		}
		// Of the edges at one position, the edges with the minor direction
		// come before those with the major direction.
		if prevFpos == edge.fpos && edge.dir == a.majorDir {
			break
		}
		edges[ix] = edges[prevIx]
		ix--
	}
	edges[ix] = edge
}

// appendSegmentToEdge is Axis::append_segment_to_edge: it links a segment
// and an edge.
func (a *ahAxis) appendSegmentToEdge(segmentIx, edgeIx int) {
	edge := &a.edges[edgeIx]
	firstIx := edge.firstIx
	lastIx := edge.lastIx
	edge.lastIx = uint16(segmentIx)
	a.segments[segmentIx].edgeNextIx = ahSome(int(firstIx))
	a.segments[lastIx].edgeNextIx = ahSome(segmentIx)
}

// ahSegment is Segment: a sequence of points with one dominant direction.
type ahSegment struct {
	// flags has the properties of the segment.
	flags uint8
	// dir is the dominant direction of the segment.
	dir ahDirection
	// pos is the position of the segment.
	pos int16
	// delta is the deviation from the position.
	delta int16
	// minCoord and maxCoord are the least and the largest coordinate.
	minCoord, maxCoord int16
	// height is the hinted height of the segment.
	height int16
	// score and len are used to match stems.
	score, len int32
	// linkIx is the best candidate for a stem link.
	linkIx ahOptIx
	// serifIx is the best candidate for a serif link.
	serifIx ahOptIx
	// firstIx and lastIx are the first and the last point in the outline.
	firstIx, lastIx uint16
	// edgeIx is the edge of the segment.
	edgeIx ahOptIx
	// edgeNextIx is the next segment in the list of segments of the edge.
	edgeNextIx ahOptIx
}

func (s *ahSegment) first() int { return int(s.firstIx) }

func (s *ahSegment) last() int { return int(s.lastIx) }

// adjustHeight is Segment::adjust_height: it changes the height of the
// segment by half the distance between v1 and v2.
func (s *ahSegment) adjustHeight(v1, v2 int32) {
	s.height = int16(int32(s.height) + ((v1 - v2) >> 1))
}

// ahEdge is Edge: a sequence of segments that is used for grid fitting.
type ahEdge struct {
	// fpos is the position in font units.
	fpos int16
	// opos is the scaled position.
	opos int32
	// pos is the current position.
	pos   int32
	flags uint8
	dir   ahDirection
	// blueEdge is the blue zone of the edge, when hasBlueEdge is true.
	blueEdge    ahScaledWidth
	hasBlueEdge bool
	// linkIx is the linked edge.
	linkIx ahOptIx
	// serifIx is the primary edge for a serif.
	serifIx ahOptIx
	// scale makes the interpolation of edges faster.
	scale int32
	// firstIx and lastIx are the first and the last segment of the edge.
	firstIx, lastIx uint16
}
