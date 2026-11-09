// Ported from skrifa 0.46.2 (src/outline/autohint/topo/edges.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/aflatin.c: af_latin_hints_compute_edges and
// af_latin_hints_compute_blue_edges, src/autofit/afcjk.c:
// af_cjk_hints_compute_edges), which has the FreeType License
// (LICENSE-FreeType.txt).

package font

import "math"

// An edge is a set of segments that lie within a threshold that depends on
// the stem widths. The edges are computed from the list of segments, they
// get properties (round, serif, links), and then they are associated with
// blue zones.

// ahComputeEdges is compute_edges: it links segments to edges, and it uses
// the analysis of the features for the selection.
func ahComputeEdges(axis *ahAxis, metrics *ahScaledAxisMetrics, topToBottomHinting bool, yScale int32, group ahScriptGroup) {
	axis.edges = axis.edges[:0]
	scale := metrics.scale
	// The functions of CJK that take the hinting direction always get 0.
	if axis.dim == ahHorizontal || group != ahGroupDefault {
		topToBottomHinting = false
	}
	// Ignore horizontal segments that are less than 1 pixel long.
	var segmentLengthThreshold int32
	if axis.dim == ahHorizontal {
		segmentLengthThreshold = ahFixedDiv(64, yScale)
	}
	// Ignore segments with a width delta of more than 0.5 pixels.
	segmentWidthThreshold := ahFixedDiv(32, scale)
	// The edge distance threshold is at most 0.25 pixels.
	initialThreshold := metrics.widthMetrics.edgeDistanceThreshold
	const edgeDistanceThresholdMax = 64 / 4
	var edgeDistanceThreshold int32
	if group == ahGroupDefault {
		edgeDistanceThreshold = ahFixedDiv(min(ahFixedMul(initialThreshold, scale), edgeDistanceThresholdMax), scale)
	} else {
		// CJK has another computation here.
		threshold := ahFixedMul(initialThreshold, scale)
		if threshold > edgeDistanceThresholdMax {
			edgeDistanceThreshold = ahFixedDiv(edgeDistanceThresholdMax, scale)
		} else {
			edgeDistanceThreshold = initialThreshold
		}
	}
	// Build the sorted table of edges: for each segment, find an edge that
	// matches, and add a new edge when there is none.
	for segmentIx := range axis.segments {
		segment := &axis.segments[segmentIx]
		if group == ahGroupDefault {
			// Ignore segments that are too short, too wide or without a
			// direction.
			if int32(segment.height) < segmentLengthThreshold ||
				int32(segment.delta) > segmentWidthThreshold ||
				segment.dir == ahDirNone {
				continue
			}
			// Ignore serif edges that are smaller than 1.5 pixels.
			if segment.serifIx.ok && 2*int32(segment.height) < 3*segmentLengthThreshold {
				continue
			}
		}
		// Look for an edge for this segment.
		bestDist := int32(math.MaxInt32)
		bestEdgeIx := -1
	edges:
		for edgeIx := range axis.edges {
			edge := &axis.edges[edgeIx]
			dist := ahAbs(int32(segment.pos) - int32(edge.fpos))
			if dist < edgeDistanceThreshold && edge.dir == segment.dir && dist < bestDist {
				if group == ahGroupDefault {
					bestEdgeIx = edgeIx
					break
				}
				// CJK has some more checks.
				if segment.linkIx.ok && int(segment.linkIx.ix) < len(axis.segments) {
					link := axis.segments[segment.linkIx.ix]
					// Check that all linked segments of the candidate edge
					// can make one edge.
					firstIx := int(edge.firstIx)
					seg1 := &axis.segments[firstIx]
					var dist2 int32
					for {
						if seg1.linkIx.ok && int(seg1.linkIx.ix) < len(axis.segments) {
							link1 := axis.segments[seg1.linkIx.ix]
							dist2 = ahAbs(int32(link.pos) - int32(link1.pos))
							if dist2 >= edgeDistanceThreshold {
								break
							}
						}
						if seg1.edgeNextIx == ahSome(firstIx) {
							break
						}
						if seg1.edgeNextIx.ok && int(seg1.edgeNextIx.ix) < len(axis.segments) {
							seg1 = &axis.segments[seg1.edgeNextIx.ix]
						} else {
							break
						}
					}
					if dist2 >= edgeDistanceThreshold {
						continue edges
					}
				}
				bestDist = dist
				bestEdgeIx = edgeIx
			}
		}
		if bestEdgeIx >= 0 {
			axis.appendSegmentToEdge(segmentIx, bestEdgeIx)
		} else {
			// There is no edge, so add a new edge for this segment.
			opos := ahFixedMul(int32(segment.pos), scale)
			edge := ahEdge{
				fpos:    segment.pos,
				opos:    opos,
				pos:     opos,
				dir:     segment.dir,
				firstIx: uint16(segmentIx),
				lastIx:  uint16(segmentIx),
			}
			axis.insertEdge(edge, topToBottomHinting)
			axis.segments[segmentIx].edgeNextIx = ahSome(segmentIx)
		}
	}
	if group == ahGroupDefault {
		// Loop again to find the segments of one point without a direction,
		// and associate them with an edge when that is possible.
		for segmentIx := range axis.segments {
			segment := &axis.segments[segmentIx]
			if segment.dir != ahDirNone {
				continue
			}
			// Find an edge that is at the position of this segment, within
			// the threshold.
			for edgeIx := range axis.edges {
				if ahAbs(int32(segment.pos)-int32(axis.edges[edgeIx].fpos)) < edgeDistanceThreshold {
					// Link the segment and the edge.
					axis.appendSegmentToEdge(segmentIx, edgeIx)
					break
				}
			}
		}
	}
	ahLinkSegmentsToEdges(axis)
	ahComputeEdgeProperties(axis)
}

// ahLinkSegmentsToEdges is link_segments_to_edges. The edges change their
// order while they are built, so the segments get the index of their edge
// in a second pass.
func ahLinkSegmentsToEdges(axis *ahAxis) {
	segments := axis.segments
	for edgeIx := range axis.edges {
		edge := &axis.edges[edgeIx]
		ix := int(edge.firstIx)
		lastIx := int(edge.lastIx)
		for {
			segment := &segments[ix]
			segment.edgeIx = ahSome(edgeIx)
			if ix == lastIx {
				break
			}
			if segment.edgeNextIx.ok {
				ix = int(segment.edgeNextIx.ix)
			} else {
				ix = lastIx
			}
		}
	}
}

// ahComputeEdgeProperties is compute_edge_properties: the properties of an
// edge come from the segments of the edge.
func ahComputeEdgeProperties(axis *ahAxis) {
	edges := axis.edges
	segments := axis.segments
	for edgeIx := range edges {
		roundness := 0
		straightness := 0
		segmentIx := int(edges[edgeIx].firstIx)
		lastSegmentIx := int(edges[edgeIx].lastIx)
		for {
			// This loop can change the current edge, so load it again.
			edge := edges[edgeIx]
			segment := &segments[segmentIx]
			nextSegmentIx := segment.edgeNextIx
			// Check the roundness.
			if segment.flags&ahTopoRound != 0 {
				roundness++
			} else {
				straightness++
			}
			// Check for serifs.
			isSerif := false
			if segment.serifIx.ok {
				serif := &segments[segment.serifIx.ix]
				isSerif = serif.edgeIx.ok && serif.edgeIx != ahSome(edgeIx)
			}
			// Check for links.
			if isSerif || (segment.linkIx.ok && segments[segment.linkIx.ix].edgeIx.ok) {
				var edge2Ix, segment2Ix ahOptIx
				if isSerif {
					edge2Ix, segment2Ix = edge.serifIx, segment.serifIx
				} else {
					edge2Ix, segment2Ix = edge.linkIx, segment.linkIx
				}
				if edge2Ix.ok && segment2Ix.ok {
					edge2 := &edges[edge2Ix.ix]
					edgeDelta := ahAbs(int32(edge.fpos) - int32(edge2.fpos))
					segment2 := &segments[segment2Ix.ix]
					segmentDelta := ahAbs(int32(segment.pos) - int32(segment2.pos))
					if segmentDelta < edgeDelta {
						edge2Ix = segment2.edgeIx
					}
				} else if segment2Ix.ok {
					edge2Ix = segments[segment2Ix.ix].edgeIx
				}
				if isSerif {
					edges[edgeIx].serifIx = edge2Ix
					edges[edge2Ix.ix].flags |= ahTopoSerif
				} else {
					edges[edgeIx].linkIx = edge2Ix
				}
			}
			if segmentIx == lastSegmentIx {
				break
			}
			if nextSegmentIx.ok {
				segmentIx = int(nextSegmentIx.ix)
			} else {
				segmentIx = lastSegmentIx
			}
		}
		edge := &edges[edgeIx]
		edge.flags = ahTopoNormal
		if roundness > 0 && roundness >= straightness {
			edge.flags |= ahTopoRound
		}
		// Drop the serif of a linked edge.
		if edge.serifIx.ok && edge.linkIx.ok {
			edge.serifIx = ahOptIx{}
		}
	}
}

// ahComputeBlueEdges is compute_blue_edges: the edges that lie in blue
// zones. For Latin this is done for the vertical axis only.
func ahComputeBlueEdges(axis *ahAxis, scale *ahScale, unscaledBlues []ahUnscaledBlue, blues []ahScaledBlue, group ahScriptGroup) {
	// The default script group has no blues in the horizontal direction.
	if axis.dim != ahVertical && group == ahGroupDefault {
		return
	}
	axisScale := scale.yScale
	if axis.dim == ahHorizontal {
		axisScale = scale.xScale
	}
	// The first threshold: a fraction of the em size, and at most 0.5
	// pixels.
	initialBestDest := min(ahFixedMul(scale.unitsPerEm/40, axisScale), 64/2)
	for edgeIx := range axis.edges {
		edge := &axis.edges[edgeIx]
		haveBestBlue := false
		var bestBlue ahScaledWidth
		bestIsNeutral := false
		bestDist := initialBestDest
		for blueIx := 0; blueIx < len(unscaledBlues) && blueIx < len(blues); blueIx++ {
			unscaledBlue := &unscaledBlues[blueIx]
			blue := &blues[blueIx]
			// Ignore the blue zones that are not active.
			if !blue.isActive {
				continue
			}
			isTop := blue.zones.isTopLike()
			isNeutral := blue.zones.isNeutral()
			isMajorDir := edge.dir == axis.majorDir
			// A neutral blue handles both directions.
			if isTop != isMajorDir || isNeutral {
				// Compare with the reference position.
				refPos, matchingBlue := unscaledBlue.position, blue.position
				if group != ahGroupDefault {
					// CJK takes the blue with the smallest delta from the
					// edge.
					if ahAbs(int32(edge.fpos)-unscaledBlue.position) > ahAbs(int32(edge.fpos)-unscaledBlue.overshoot) {
						refPos, matchingBlue = unscaledBlue.overshoot, blue.overshoot
					}
				}
				dist := ahFixedMul(ahAbs(int32(edge.fpos)-refPos), axisScale)
				if dist < bestDist {
					bestDist = dist
					haveBestBlue, bestBlue = true, matchingBlue
					bestIsNeutral = isNeutral
				}
				if group == ahGroupDefault {
					// The default script group compares with the overshoot
					// position also.
					if edge.flags&ahTopoRound != 0 && dist != 0 && !isNeutral {
						isUnderRef := int32(edge.fpos) < unscaledBlue.position
						if isTop != isUnderRef {
							dist := ahFixedMul(ahAbs(int32(edge.fpos)-unscaledBlue.overshoot), axisScale)
							if dist < bestDist {
								bestDist = dist
								haveBestBlue, bestBlue = true, blue.overshoot
								bestIsNeutral = isNeutral
							}
						}
					}
				}
			}
		}
		if haveBestBlue {
			edge.blueEdge, edge.hasBlueEdge = bestBlue, true
			if bestIsNeutral {
				edge.flags |= ahTopoNeutral
			}
		}
	}
}
