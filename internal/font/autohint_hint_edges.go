// Ported from skrifa 0.46.2 (src/outline/autohint/hint/edges.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/aflatin.c: af_latin_hint_edges, src/autofit/afcjk.c:
// af_cjk_hint_edges), which has the FreeType License (LICENSE-FreeType.txt).

package font

// The edges are aligned to the pixel grid here. This is the last step before
// the adjustments of the edges are applied to the points of the outline.

// ahHintEdges is hint_edges: the main grid fitting routine of the Latin
// writing system.
func ahHintEdges(axis *ahAxis, metrics *ahScaledAxisMetrics, group ahScriptGroup, scale *ahScale, topToBottomHinting bool) {
	if axis.dim != ahVertical {
		topToBottomHinting = false
	}
	// First align the horizontal edges to the blue zones.
	anchorIx := ahAlignEdgesToBlues(axis, metrics, group, scale)
	// Then align the stem edges.
	serifCount, anchorIx := ahAlignStemEdges(axis, metrics, group, scale, topToBottomHinting, anchorIx)
	edges := axis.edges
	// A special case for the small letter m.
	if axis.dim == ahHorizontal && (len(edges) == 6 || len(edges) == 12) {
		ahHintLowercaseM(edges, group)
	}
	// Handle the serifs and the edges of one segment.
	if serifCount > 0 || anchorIx < 0 {
		ahAlignRemainingEdges(axis, group, topToBottomHinting, serifCount, anchorIx)
	}
}

// ahAlignEdgesToBlues is align_edges_to_blues: it aligns the horizontal
// edges to the blue zones. The result is the index of the anchor edge, or
// -1.
func ahAlignEdgesToBlues(axis *ahAxis, metrics *ahScaledAxisMetrics, group ahScriptGroup, scale *ahScale) int {
	anchorIx := -1
	// The default script group has vertical blues only.
	if group == ahGroupDefault && axis.dim != ahVertical {
		return anchorIx
	}
	for edgeIx := range axis.edges {
		edges := axis.edges
		edge := &edges[edgeIx]
		if edge.flags&ahTopoDone != 0 {
			continue
		}
		edge2Ix := -1
		if edge.linkIx.ok {
			edge2Ix = int(edge.linkIx.ix)
		}
		// With two neutral zones, skip one of them.
		if edge.hasBlueEdge && edge2Ix >= 0 {
			edge2 := &edges[edge2Ix]
			if edge2.hasBlueEdge {
				skipIx := -1
				if edge2.flags&ahTopoNeutral != 0 {
					skipIx = edge2Ix
				} else if edge.flags&ahTopoNeutral != 0 {
					skipIx = edgeIx
				}
				if skipIx >= 0 {
					skipEdge := &edges[skipIx]
					skipEdge.hasBlueEdge = false
					skipEdge.blueEdge = ahScaledWidth{}
					skipEdge.flags &^= ahTopoNeutral
				}
			}
		}
		// Flip the edges when the other edge is aligned to a blue zone.
		var blue ahScaledWidth
		edge1Ix := -1
		if edges[edgeIx].hasBlueEdge {
			blue = edges[edgeIx].blueEdge
			edge1Ix = edgeIx
		} else if edge2Ix >= 0 && edges[edge2Ix].hasBlueEdge {
			blue = edges[edge2Ix].blueEdge
			edge1Ix, edge2Ix = edge2Ix, edgeIx
		} else {
			edge2Ix = -1
		}
		if edge1Ix < 0 {
			continue
		}
		// Skip the edge when an iteration before positioned it already: the
		// edge has no blue, its linked edge has one, and the loop visited
		// the linked edge before.
		if edges[edge1Ix].flags&ahTopoDone != 0 {
			continue
		}
		edge1 := &edges[edge1Ix]
		edge1.pos = blue.fitted
		edge1.flags |= ahTopoDone
		if edge2Ix >= 0 && !edges[edge2Ix].hasBlueEdge {
			edges[edge2Ix].flags |= ahTopoDone
			ahAlignLinkedEdge(axis, metrics, group, scale, edge1Ix, edge2Ix)
		}
		if anchorIx < 0 {
			anchorIx = edgeIx
		}
	}
	return anchorIx
}

// ahAlignStemEdges is align_stem_edges: it aligns the stem edges, and it
// tries to keep the order of the stems in the glyph. The results are the
// number of serifs, and the index of the anchor edge or -1.
func ahAlignStemEdges(axis *ahAxis, metrics *ahScaledAxisMetrics, group ahScriptGroup, scale *ahScale, topToBottomHinting bool, anchorIx int) (int, int) {
	serifCount := 0
	haveLastStemPos := false
	var lastStemPos int32
	var delta int32
	// Align all other stem edges.
	for edgeIx := range axis.edges {
		// Read the values for the early exits.
		edges := axis.edges
		edge := &edges[edgeIx]
		edgeFlags := edge.flags
		edgePos := edge.pos
		edge2Ix := -1
		var edge2PosForCjk int32
		edge2HasBlue := false
		if edge.linkIx.ok {
			edge2Ix = int(edge.linkIx.ix)
			edge2PosForCjk = edges[edge2Ix].pos
			edge2HasBlue = edges[edge2Ix].hasBlueEdge
		}
		if edgeFlags&ahTopoDone != 0 {
			continue
		}
		// Skip all edges that are not stems.
		if edge2Ix < 0 {
			serifCount++
			continue
		}
		// CJK skips the stems that are too close. They come later.
		if group != ahGroupDefault {
			if haveLastStemPos {
				if edgePos < lastStemPos+64 || edge2PosForCjk < lastStemPos+64 {
					serifCount++
					continue
				}
			}
		}
		// This must not occur. It is the fallback of the C code.
		if edge2HasBlue {
			ahAlignLinkedEdge(axis, metrics, group, scale, edge2Ix, edgeIx)
			axis.edges[edgeIx].flags |= ahTopoDone
			continue
		}
		if group == ahGroupDefault {
			// Align the stem. The branches are in the other order than in
			// FreeType.
			if anchorIx >= 0 {
				anchor := &edges[anchorIx]
				edge := edges[edgeIx]
				edge2 := edges[edge2Ix]
				originalPos := anchor.pos + (edge.opos - anchor.opos)
				originalLen := edge2.opos - edge.opos
				originalCenter := originalPos + (originalLen >> 1)
				curLen := ahStemWidth(metrics, group, scale, originalLen, 0, edge.flags, edge2.flags)
				if edge2.flags&ahTopoDone != 0 {
					newPos := edge2.pos - curLen
					edges[edgeIx].pos = newPos
				} else if curLen < 96 {
					curPos1 := ahPixRound(originalCenter)
					uOff, dOff := int32(38), int32(26)
					if curLen <= 64 {
						uOff, dOff = 32, 32
					}
					delta1 := ahAbs(originalCenter - (curPos1 - uOff))
					delta2 := ahAbs(originalCenter - (curPos1 + dOff))
					if delta1 < delta2 {
						curPos1 = curPos1 - uOff
					} else {
						curPos1 = curPos1 + dOff
					}
					edges[edgeIx].pos = curPos1 - curLen/2
					edges[edge2Ix].pos = curPos1 + curLen/2
				} else {
					curPos1 := ahPixRound(originalPos)
					delta1 := ahAbs(curPos1 + (curLen >> 1) - originalCenter)
					curPos2 := ahPixRound(originalPos+originalLen) - curLen
					delta2 := ahAbs(curPos2 + (curLen >> 1) - originalCenter)
					newPos := curPos2
					if delta1 < delta2 {
						newPos = curPos1
					}
					newPos2 := newPos + curLen
					edges[edgeIx].pos = newPos
					edges[edge2Ix].pos = newPos2
				}
				edges[edgeIx].flags |= ahTopoDone
				edges[edge2Ix].flags |= ahTopoDone
				if edgeIx > 0 {
					ahAdjustLink(edges, edgeIx, ahLinkPrev, topToBottomHinting)
				}
			} else {
				// No stem is aligned yet.
				edge := edges[edgeIx]
				edge2 := edges[edge2Ix]
				originalLen := edge2.opos - edge.opos
				curLen := ahStemWidth(metrics, group, scale, originalLen, 0, edge.flags, edge2.flags)
				// The edges of small stem widths get a special rounding:
				// for a width of at most 1px, and for a width between 1px
				// and 1.5px.
				uOff, dOff := int32(38), int32(26)
				if curLen <= 64 {
					uOff, dOff = 32, 32
				}
				if curLen < 96 {
					originalCenter := edge.opos + (originalLen >> 1)
					curPos1 := ahPixRound(originalCenter)
					error1 := ahAbs(originalCenter - (curPos1 - uOff))
					error2 := ahAbs(originalCenter - (curPos1 + dOff))
					if error1 < error2 {
						curPos1 -= uOff
					} else {
						curPos1 += dOff
					}
					edgePos := curPos1 - curLen/2
					edges[edgeIx].pos = edgePos
					edges[edge2Ix].pos = edgePos + curLen
				} else {
					edges[edgeIx].pos = ahPixRound(edge.opos)
				}
				edges[edgeIx].flags |= ahTopoDone
				ahAlignLinkedEdge(axis, metrics, group, scale, edgeIx, edge2Ix)
				anchorIx = edgeIx
			}
		} else {
			// CJK differs more here.
			if edge2Ix < edgeIx {
				haveLastStemPos, lastStemPos = true, edgePos
				edges[edgeIx].flags |= ahTopoDone
				ahAlignLinkedEdge(axis, metrics, group, scale, edge2Ix, edgeIx)
				continue
			}
			if axis.dim != ahVertical && anchorIx < 0 {
				delta = ahHintNormalStemCjk(axis, metrics, group, scale, edgeIx, edge2Ix, delta)
			} else {
				ahHintNormalStemCjk(axis, metrics, group, scale, edgeIx, edge2Ix, delta)
			}
			anchorIx = edgeIx
			axis.edges[edgeIx].flags |= ahTopoDone
			edge2 := &axis.edges[edge2Ix]
			edge2.flags |= ahTopoDone
			haveLastStemPos, lastStemPos = true, edge2.pos
		}
	}
	return serifCount, anchorIx
}

// ahHintLowercaseM is hint_lowercase_m: it keeps the symmetry of the small
// letter m.
func ahHintLowercaseM(edges []ahEdge, group ahScriptGroup) {
	edge1Ix, edge2Ix, edge3Ix := 1, 5, 9
	if len(edges) == 6 {
		edge1Ix, edge2Ix, edge3Ix = 0, 2, 4
	}
	edge1 := &edges[edge1Ix]
	edge2 := &edges[edge2Ix]
	edge3 := &edges[edge3Ix]
	dist1 := edge2.opos - edge1.opos
	dist2 := edge3.opos - edge2.opos
	span := ahAbs(dist1 - dist2)
	if group != ahGroupDefault {
		// CJK has more conditions.
		for _, ix := range [3]int{edge1Ix, edge2Ix, edge3Ix} {
			if edges[ix].linkIx != ahSome(ix+1) {
				return
			}
		}
	}
	if span < 8 {
		delta := edge3.pos - (2*edge2.pos - edge1.pos)
		linkIx := edge3.linkIx
		edge3.pos -= delta
		edge3.flags |= ahTopoDone
		if linkIx.ok {
			link := &edges[linkIx.ix]
			link.pos -= delta
			link.flags |= ahTopoDone
		}
		// Move the serifs along with the stem.
		if len(edges) == 12 {
			edges[8].pos -= delta
			edges[11].pos -= delta
		}
	}
}

// ahAlignRemainingEdges is align_remaining_edges: it aligns the serifs and
// the edges of one segment.
func ahAlignRemainingEdges(axis *ahAxis, group ahScriptGroup, topToBottomHinting bool, serifCount int, anchorIx int) {
	if group == ahGroupDefault {
		for edgeIx := range axis.edges {
			edges := axis.edges
			edge := &edges[edgeIx]
			delta := int32(1000)
			if edge.serifIx.ok && int(edge.serifIx.ix) < len(edges) {
				delta = ahAbs(edges[edge.serifIx.ix].opos - edge.opos)
			}
			edgeFlags, edgeOpos, edgeSerifIx := edge.flags, edge.opos, edge.serifIx
			if edgeFlags&ahTopoDone != 0 {
				continue
			}
			if delta < 64+16 {
				// The delta is less than 1000 only for an edge with a
				// serif.
				serifIx := int(edgeSerifIx.ix)
				ahAlignSerifEdge(axis, serifIx, edgeIx)
			} else if anchorIx >= 0 {
				beforeIx, afterIx := ahFindBoundingCompletedEdges(edges, edgeIx)
				if beforeIx >= 0 && afterIx >= 0 {
					before := &edges[beforeIx]
					after := &edges[afterIx]
					var newPos int32
					if after.opos == before.opos {
						newPos = before.pos
					} else {
						newPos = before.pos + ahFixedMulDiv(edgeOpos-before.opos, after.pos-before.pos, after.opos-before.opos)
					}
					edges[edgeIx].pos = newPos
				} else {
					anchor := &edges[anchorIx]
					newPos := anchor.pos + ((edgeOpos - anchor.opos + 16) &^ 31)
					edges[edgeIx].pos = newPos
				}
			} else {
				anchorIx = edgeIx
				newPos := ahPixRound(edgeOpos)
				edges[edgeIx].pos = newPos
			}
			edges[edgeIx].flags |= ahTopoDone
			ahAdjustLink(edges, edgeIx, ahLinkPrev, topToBottomHinting)
			ahAdjustLink(edges, edgeIx, ahLinkNext, topToBottomHinting)
		}
	} else {
		for edgeIx := range axis.edges {
			edge := &axis.edges[edgeIx]
			if edge.flags&ahTopoDone != 0 {
				continue
			}
			if edge.serifIx.ok {
				serifIx := int(edge.serifIx.ix)
				edge.flags |= ahTopoDone
				ahAlignSerifEdge(axis, serifIx, edgeIx)
				if serifCount > 0 {
					serifCount--
				}
			}
		}
		if serifCount == 0 {
			return
		}
		for edgeIx := range axis.edges {
			edges := axis.edges
			edge := &edges[edgeIx]
			if edge.flags&ahTopoDone != 0 {
				continue
			}
			beforeIx, afterIx := ahFindBoundingCompletedEdges(edges, edgeIx)
			switch {
			case beforeIx >= 0 && afterIx < 0:
				ahAlignSerifEdge(axis, beforeIx, edgeIx)
			case beforeIx < 0 && afterIx >= 0:
				ahAlignSerifEdge(axis, afterIx, edgeIx)
			case beforeIx >= 0 && afterIx >= 0:
				before := edges[beforeIx]
				after := edges[afterIx]
				if after.fpos == before.fpos {
					edges[edgeIx].pos = before.pos
				} else {
					edges[edgeIx].pos = before.pos +
						ahFixedMulDiv(int32(edge.fpos)-int32(before.fpos), after.pos-before.pos, int32(after.fpos)-int32(before.fpos))
				}
			}
		}
	}
}

// The directions of adjust_link (LinkDir).
const (
	ahLinkPrev = iota
	ahLinkNext
)

// ahAdjustLink is adjust_link: it adjusts an edge that passed the edge
// before or after it, for the hinting direction.
func ahAdjustLink(edges []ahEdge, edgeIx int, linkDir int, topToBottomHinting bool) {
	edge := &edges[edgeIx]
	var edge2, prevEdge *ahEdge
	if linkDir == ahLinkNext {
		if edgeIx+1 >= len(edges) {
			return
		}
		edge2 = &edges[edgeIx+1]
		// Do not adjust for the next edge when it is not done yet.
		if edge2.flags&ahTopoDone == 0 {
			return
		}
		if edgeIx < 1 {
			return
		}
		prevEdge = &edges[edgeIx-1]
	} else {
		if edgeIx < 1 {
			return
		}
		edge2 = &edges[edgeIx-1]
		prevEdge = edge2
	}
	pos1 := edge.pos
	pos2 := edge2.pos
	var orderCheck bool
	if (linkDir == ahLinkPrev) == topToBottomHinting {
		orderCheck = pos1 > pos2
	} else {
		orderCheck = pos1 < pos2
	}
	if !orderCheck {
		return
	}
	if !edge.linkIx.ok || int(edge.linkIx.ix) >= len(edges) {
		return
	}
	link := &edges[edge.linkIx.ix]
	if ahAbs(link.pos-prevEdge.pos) > 16 {
		edges[edgeIx].pos = edge2.pos
	}
}

// ahFindBoundingCompletedEdges is find_bounding_completed_edges: the
// indices of the done edges before and after an edge, or -1.
func ahFindBoundingCompletedEdges(edges []ahEdge, ix int) (beforeIx, afterIx int) {
	beforeIx, afterIx = -1, -1
	for i := min(ix, len(edges)) - 1; i >= 0; i-- {
		if edges[i].flags&ahTopoDone != 0 {
			beforeIx = i
			break
		}
	}
	for i := ix + 1; i < len(edges); i++ {
		if edges[i].flags&ahTopoDone != 0 {
			afterIx = i
			break
		}
	}
	return beforeIx, afterIx
}

// ahSnapWidth is snap_width: it snaps a scaled width to one of the standard
// widths.
func ahSnapWidth(widths []ahScaledWidth, width int32) int32 {
	bestDist := int32(64 + 32 + 2)
	refWidth := width
	for _, candidate := range widths {
		dist := ahAbs(width - candidate.scaled)
		if dist < bestDist {
			bestDist, refWidth = dist, candidate.scaled
		}
	}
	scaled := ahPixRound(refWidth)
	if width >= refWidth {
		if width < scaled+48 {
			return refWidth
		}
		return width
	}
	if width > scaled-48 {
		return refWidth
	}
	return width
}

// ahStemWidth is stem_width: the snapped width of a stem.
func ahStemWidth(metrics *ahScaledAxisMetrics, group ahScriptGroup, scale *ahScale, width, baseDelta int32, baseFlags, stemFlags uint8) int32 {
	if !scale.flags.contains(ahScaleStemAdjust) || (group == ahGroupDefault && metrics.widthMetrics.isExtraLight) {
		return width
	}
	isVertical := metrics.dim == ahVertical
	sign := int32(1)
	if width < 0 {
		sign = -1
	}
	dist := ahAbs(width)
	if (isVertical && !scale.flags.contains(ahScaleVerticalSnap)) || (!isVertical && !scale.flags.contains(ahScaleHorizontalSnap)) {
		// Smooth hinting.
		if group == ahGroupDefault {
			if stemFlags&ahTopoSerif != 0 && isVertical && dist < 3*64 {
				// The widths of serifs stay as they are.
				return dist * sign
			} else if baseFlags&ahTopoRound != 0 {
				if dist < 80 {
					dist = 64
				}
			} else if dist < 56 {
				dist = 56
			}
		}
		if len(metrics.widths) != 0 {
			// Compare with the standard width.
			minWidth := metrics.widths[0].scaled
			delta := ahAbs(dist - minWidth)
			if delta < 40 {
				dist = max(minWidth, 48)
				return dist * sign
			}
			if group == ahGroupDefault {
				// The behavior of the default group and of Latin.
				if dist < 3*64 {
					delta := dist & 63
					dist &= -64
					if delta < 10 {
						dist += delta
					} else if delta < 32 {
						dist += 10
					} else if delta < 54 {
						dist += 54
					} else {
						dist += delta
					}
				} else {
					var newBaseDelta int32
					if (width > 0 && baseDelta > 0) || (width < 0 && baseDelta < 0) {
						if scale.size < 10.0 {
							newBaseDelta = baseDelta
						} else if scale.size < 30.0 {
							newBaseDelta = (baseDelta * ahF32ToI32(30.0-scale.size)) / 20
						}
					}
					dist = (dist - ahAbs(newBaseDelta) + 32) &^ 63
				}
			}
		}
		if group != ahGroupDefault {
			// The behavior of CJK.
			if dist < 54 {
				dist += (54 - dist) / 2
			} else if dist < 3*64 {
				delta := dist & 63
				dist &= -64
				if delta < 10 {
					dist += delta
				} else if delta < 22 {
					dist += 10
				} else if delta < 42 {
					dist += delta
				} else if delta < 54 {
					dist += 54
				} else {
					dist += delta
				}
			}
		}
	} else {
		// Strong hinting: snap to whole pixels.
		originalDist := dist
		dist = ahSnapWidth(metrics.widths, dist)
		if isVertical {
			// The vertical case always rounds to whole pixels.
			if dist >= 64 {
				dist = (dist + 16) &^ 63
			} else {
				dist = 64
			}
		} else if scale.flags.contains(ahScaleMono) {
			// Horizontal hinting without antialiasing: snap to whole
			// pixels with another threshold.
			if dist < 64 {
				dist = 64
			} else {
				dist = (dist + 32) &^ 63
			}
		} else {
			// Smooth horizontal hinting: make small stems stronger, and
			// round stems whose size is between 1 and 2 pixels.
			if dist < 48 {
				dist = (dist + 64) >> 1
			} else if dist < 128 {
				// Round to whole pixels only when the distortion is less
				// than 1/4 pixel.
				dist = (dist + 22) &^ 63
				if group == ahGroupDefault {
					delta := ahAbs(dist - originalDist)
					if delta >= 16 {
						dist = originalDist
						if dist < 48 {
							dist = (dist + 64) >> 1
						}
					}
				}
			} else {
				// Round, to prevent color fringes in the LCD mode.
				dist = (dist + 32) &^ 63
			}
		}
	}
	return dist * sign
}

// ahAlignLinkedEdge is align_linked_edge: it aligns one stem edge relative
// to the stem edge before it.
func ahAlignLinkedEdge(axis *ahAxis, metrics *ahScaledAxisMetrics, group ahScriptGroup, scale *ahScale, baseEdgeIx, stemEdgeIx int) {
	edges := axis.edges
	baseEdge := &edges[baseEdgeIx]
	stemEdge := &edges[stemEdgeIx]
	width := stemEdge.opos - baseEdge.opos
	baseDelta := baseEdge.pos - baseEdge.opos
	fittedWidth := ahStemWidth(metrics, group, scale, width, baseDelta, baseEdge.flags, stemEdge.flags)
	edges[stemEdgeIx].pos = baseEdge.pos + fittedWidth
}

// ahAlignSerifEdge is align_serif_edge: it moves the serif edge by the
// adjustment of the base edge.
func ahAlignSerifEdge(axis *ahAxis, baseEdgeIx, serifEdgeIx int) {
	edges := axis.edges
	baseEdge := &edges[baseEdgeIx]
	serifEdge := &edges[serifEdgeIx]
	edges[serifEdgeIx].pos = baseEdge.pos + (serifEdge.opos - baseEdge.opos)
}

// ahHintNormalStemCjk is hint_normal_stem_cjk: it adjusts both edges of a
// stem, and it returns the delta.
func ahHintNormalStemCjk(axis *ahAxis, metrics *ahScaledAxisMetrics, group ahScriptGroup, scale *ahScale, edgeIx, edge2Ix int, anchor int32) int32 {
	const (
		maxHorizontalGap = 9
		maxVerticalGap   = 15
		maxDeltaAbs      = 14
	)
	edge := axis.edges[edgeIx]
	edge2 := axis.edges[edge2Ix]
	doStemAdjust := scale.flags.contains(ahScaleStemAdjust)
	var thresholdDelta int32
	if !doStemAdjust {
		delta := int32(maxVerticalGap)
		if axis.dim == ahVertical {
			delta = maxHorizontalGap
		}
		if edge.flags&ahTopoRound != 0 && edge2.flags&ahTopoRound != 0 {
			thresholdDelta = delta
		} else {
			thresholdDelta = delta / 3
		}
	}
	threshold := 64 - thresholdDelta
	originalLen := edge2.opos - edge.opos
	curLen := ahStemWidth(metrics, group, scale, originalLen, 0, edge.flags, edge2.flags)
	originalCenter := (edge.opos+edge2.opos)/2 + anchor
	curPos1 := originalCenter - curLen/2
	curPos2 := curPos1 + curLen
	finish := func(delta int32) int32 {
		if !doStemAdjust {
			delta = min(max(delta, -maxDeltaAbs), maxDeltaAbs)
		}
		adjustment := curPos1 + delta
		if edge.opos < edge2.opos {
			axis.edges[edgeIx].pos = adjustment
			axis.edges[edge2Ix].pos = adjustment + curLen
		} else {
			axis.edges[edge2Ix].pos = adjustment
			axis.edges[edgeIx].pos = adjustment + curLen
		}
		return delta
	}
	dOff1 := curPos1 - ahPixFloor(curPos1)
	dOff2 := curPos2 - ahPixFloor(curPos2)
	var delta int32
	if dOff1 == 0 || dOff2 == 0 {
		return finish(delta)
	}
	uOff1 := 64 - dOff1
	uOff2 := 64 - dOff2
	if curLen <= threshold {
		if dOff2 < curLen {
			if uOff1 <= dOff2 {
				delta = uOff1
			} else {
				delta = -dOff2
			}
		}
		return finish(delta)
	}
	if threshold < 64 && (dOff1 >= threshold || uOff1 >= threshold || dOff2 >= threshold || uOff2 >= threshold) {
		return finish(delta)
	}
	offset := curLen & 63
	if offset < 32 {
		if uOff1 <= offset || dOff2 <= offset {
			return finish(delta)
		}
	} else {
		offset = 64 - threshold
	}
	dOff1 = threshold - uOff1
	uOff1 -= offset
	uOff2 = threshold - dOff2
	dOff2 -= offset
	if dOff1 <= uOff1 {
		uOff1 = -dOff1
	}
	if dOff2 <= uOff2 {
		uOff2 = -dOff2
	}
	if ahAbs(uOff1) <= ahAbs(uOff2) {
		delta = uOff1
	} else {
		delta = uOff2
	}
	return finish(delta)
}
