// Ported from skrifa 0.46.2 (src/outline/autohint/metrics/blues.rs, and
// find_last_contour of src/outline/unscaled.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/aflatin.c: af_latin_metrics_init_blues, src/autofit/afcjk.c:
// af_cjk_metrics_init_blues, src/autofit/afblue.h), which has the FreeType
// License (LICENSE-FreeType.txt).

package font

import (
	"math"
	"sort"
	"strings"
)

// ahMaxBlues is MAX_BLUES: the largest number of blue values.
const ahMaxBlues = 8

// ahBlueStringMaxLen is BLUE_STRING_MAX_LEN.
const ahBlueStringMaxLen = 51

// ahBlueZones is BlueZones: the zones of a blue value.
type ahBlueZones uint16

const (
	ahBlueNone       ahBlueZones = 0
	ahBlueTop        ahBlueZones = 1 << 1
	ahBlueSubTop     ahBlueZones = 1 << 2
	ahBlueNeutral    ahBlueZones = 1 << 3
	ahBlueAdjustment ahBlueZones = 1 << 4
	ahBlueXHeight    ahBlueZones = 1 << 5
	ahBlueLong       ahBlueZones = 1 << 6
	ahBlueHorizontal ahBlueZones = 1 << 2
	ahBlueRight                  = ahBlueTop
)

func (z ahBlueZones) contains(other ahBlueZones) bool { return z&other == other }

func (z ahBlueZones) isTopLike() bool { return z&(ahBlueTop|ahBlueSubTop) != ahBlueNone }

func (z ahBlueZones) isTop() bool { return z.contains(ahBlueTop) }

func (z ahBlueZones) isSubTop() bool { return z.contains(ahBlueSubTop) }

func (z ahBlueZones) isNeutral() bool { return z.contains(ahBlueNeutral) }

func (z ahBlueZones) isXHeight() bool { return z.contains(ahBlueXHeight) }

func (z ahBlueZones) isLong() bool { return z.contains(ahBlueLong) }

func (z ahBlueZones) isHorizontal() bool { return z.contains(ahBlueHorizontal) }

func (z ahBlueZones) isRight() bool { return z.contains(ahBlueRight) }

func (z ahBlueZones) retainTopLikeOrNeutral() ahBlueZones {
	return z & (ahBlueTop | ahBlueSubTop | ahBlueNeutral)
}

// ahUnscaledBlue is UnscaledBlue: an alignment zone in font units.
type ahUnscaledBlue struct {
	// position is the position of the blue.
	position int32
	// overshoot is the overshoot value of the blue.
	overshoot int32
	// ascender and descender are the extents of the outlines that the blue
	// comes from.
	ascender, descender int32
	// zones has the active zones of the blue.
	zones ahBlueZones
}

// ahScaledBlue is ScaledBlue: a scaled alignment zone.
type ahScaledBlue struct {
	position  ahScaledWidth
	overshoot ahScaledWidth
	zones     ahBlueZones
	isActive  bool
}

// ahComputeUnscaledBlues is compute_unscaled_blues: the blue values of each
// axis in font units.
func ahComputeUnscaledBlues(shaper *ahShaper, style *ahStyleClass) [2][]ahUnscaledBlue {
	switch style.script.group {
	case ahGroupDefault:
		// The default group has no horizontal blues.
		return [2][]ahUnscaledBlue{nil, ahComputeDefaultBlues(shaper, style)}
	case ahGroupCjk:
		return ahComputeCjkBlues(shaper, style)
	}
	// The Indic group does not use blue values.
	return [2][]ahUnscaledBlue{}
}

// ahFindLastContour is UnscaledOutlineRef::find_last_contour: the range of
// the contour and the index in that contour of the last point for which f
// returns true.
func ahFindLastContour(points []ahUnscaledPoint, f func(point *ahUnscaledPoint) bool) (start, end, bestPoint int, ok bool) {
	if len(points) == 0 {
		return 0, 0, 0, false
	}
	bestStart, bestEnd := 0, 0
	curStart, curEnd := 0, 0
	foundBestInCurContour := false
	for pointIx := range points {
		point := &points[pointIx]
		if point.isContourStart {
			if foundBestInCurContour {
				bestStart, bestEnd = curStart, curEnd
			}
			curStart, curEnd = pointIx, pointIx
			foundBestInCurContour = false
			// A contour with one point is ignored.
			if pointIx+1 >= len(points) || points[pointIx+1].isContourStart {
				continue
			}
		}
		curEnd++
		if f(point) {
			bestPoint = pointIx - curStart
			foundBestInCurContour = true
		}
	}
	if foundBestInCurContour {
		bestStart, bestEnd = curStart, curEnd
	}
	if bestStart < bestEnd {
		return bestStart, bestEnd, bestPoint, true
	}
	return 0, 0, 0, false
}

func ahSortInt32(v []int32) {
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
}

// ahComputeDefaultBlues is compute_default_blues: the blue values of a
// script of the default group in font units.
func ahComputeDefaultBlues(shaper *ahShaper, style *ahStyleClass) []ahUnscaledBlue {
	var blues []ahUnscaledBlue
	var flats, rounds [ahBlueStringMaxLen]int32
	font := shaper.font
	unitsPerEm := int32(font.unitsPerEm)
	flatThreshold := unitsPerEm / 14
	clusterShaper := shaper.clusterShaper(style)
	var shapedCluster []ahShapedGlyph
	// Walk over the sets of blue characters of the script.
	for _, blue := range style.script.blues {
		blueZones := blue.zones
		ascender := int32(math.MinInt32)
		descender := int32(math.MaxInt32)
		nFlats := 0
		nRounds := 0
		if len(blues) >= ahMaxBlues {
			continue
		}
		for _, cluster := range strings.Split(blue.str, " ") {
			bestYExtremum := int32(math.MaxInt32)
			if blueZones.isTop() {
				bestYExtremum = math.MinInt32
			}
			bestIsRound := false
			shapedCluster = clusterShaper.shape(cluster, shapedCluster)
			for _, g := range shapedCluster {
				if g.id == 0 {
					continue
				}
				glyph, ok := font.outline(g.id)
				if !ok {
					continue
				}
				yOffset := g.yOffset
				points, _, ok := font.drawUnscaled(glyph)
				if !ok {
					continue
				}
				// Reject a glyph that cannot produce a rendering.
				if len(points) <= 2 {
					continue
				}
				haveBestY := false
				var bestY16 int16
				// Find the extreme point for a top blue or for a bottom
				// blue.
				var start, end, bestPointIx int
				if blueZones.isTopLike() {
					start, end, bestPointIx, ok = ahFindLastContour(points, func(point *ahUnscaledPoint) bool {
						if !haveBestY || point.y > bestY16 {
							haveBestY, bestY16 = true, point.y
							ascender = max(ascender, int32(point.y)+yOffset)
							return true
						}
						descender = min(descender, int32(point.y)+yOffset)
						return false
					})
				} else {
					start, end, bestPointIx, ok = ahFindLastContour(points, func(point *ahUnscaledPoint) bool {
						if !haveBestY || point.y < bestY16 {
							haveBestY, bestY16 = true, point.y
							descender = min(descender, int32(point.y)+yOffset)
							return true
						}
						ascender = max(ascender, int32(point.y)+yOffset)
						return false
					})
				}
				if !ok {
					continue
				}
				bestContour := points[start:end]
				if !haveBestY {
					continue
				}
				bestY := int32(bestY16)
				bestX := int32(bestContour[bestPointIx].x)
				// Find out whether the point is on a straight or a round
				// segment: look at the points before and after it. An
				// index of -1 says that there is no point.
				onPointFirst, onPointLast := -1, -1
				if bestContour[bestPointIx].isOnCurve() {
					onPointFirst, onPointLast = bestPointIx, bestPointIx
				}
				segmentFirst := bestPointIx
				segmentLast := bestPointIx
				n := len(bestContour)
				// Look for the points before and after on the contour that
				// are not on the same Y coordinate, and allow a small
				// distance or angle (20 is about 2.9 degrees).
				for k := n - 1; k >= 0; k-- {
					ix := (k + bestPointIx) % n
					prev := bestContour[ix]
					dist := ahAbs(int32(prev.y) - bestY)
					if dist > 5 && ahAbs(int32(prev.x)-bestX) <= 20*dist {
						break
					}
					segmentFirst = ix
					if prev.isOnCurve() {
						onPointFirst = ix
						if onPointLast < 0 {
							onPointLast = ix
						}
					}
				}
				nextIx := 0
				for k := 0; k < n; k++ {
					ix := (k + bestPointIx + 1) % n
					next := bestContour[ix]
					// The long blue computation uses nextIx later.
					nextIx = ix
					dist := ahAbs(int32(next.y) - bestY)
					if dist > 5 && ahAbs(int32(next.x)-bestX) <= 20*dist {
						break
					}
					segmentLast = ix
					if next.isOnCurve() {
						onPointLast = ix
						if onPointFirst < 0 {
							onPointFirst = ix
						}
					}
				}
				if blueZones.isLong() {
					// From FreeType: with this flag there is one more
					// constraint for the distance of the blue zone. Find a
					// segment of the topmost (or bottommost) contour that is
					// longer than a heuristic threshold. Small bumps in the
					// outline are ignored then (the vertical serifs of many
					// Hebrew designs).
					//
					// When this segment is long enough, that is all. Else
					// search the segment next to the extremum that is long
					// enough, has the same direction and a vertical
					// distance from the extremum that is not too large.
					lengthThreshold := unitsPerEm / 25
					dist := ahAbs(int32(bestContour[segmentLast].x) - int32(bestContour[segmentFirst].x))
					if dist < lengthThreshold && ahSatisfiesMinLongSegmentLen(segmentFirst, segmentLast, n-1) {
						heightThreshold := unitsPerEm / 4
						// Find the point before with a different x value.
						prevIx := bestPointIx
						for k := n - 1; k >= 0; k-- {
							ix := (k + bestPointIx) % n
							if int32(bestContour[ix].x) != bestX {
								prevIx = ix
								break
							}
						}
						// Skip the degenerate case.
						if prevIx == bestPointIx {
							continue
						}
						isLtr := int32(bestContour[prevIx].x) < bestX
						first := segmentLast
						last := first
						pFirst, pLast := -1, -1
						hit := false
						for {
							if !hit {
								// No hit: adjust the first point, and the
								// first and last point on the curve.
								first = last
								if bestContour[first].isOnCurve() {
									pFirst, pLast = first, first
								} else {
									pFirst, pLast = -1, -1
								}
								hit = true
							}
							if last < n-1 {
								last++
							} else {
								last = 0
							}
							if ahAbs(bestY-int32(bestContour[first].y)) > heightThreshold {
								// The vertical distance is too large.
								hit = false
								if last == segmentFirst {
									break
								}
								continue
							}
							dist := ahAbs(int32(bestContour[last].y) - int32(bestContour[first].y))
							if dist > 5 && ahAbs(int32(bestContour[last].x)-int32(bestContour[first].x)) <= 20*dist {
								hit = false
								if last == segmentFirst {
									break
								}
								continue
							}
							if bestContour[last].isOnCurve() {
								pLast = last
								if pFirst < 0 {
									pFirst = last
								}
							}
							firstX := int32(bestContour[first].x)
							lastX := int32(bestContour[last].x)
							isCurLtr := firstX < lastX
							dx := ahAbs(lastX - firstX)
							if isCurLtr == isLtr && dx >= lengthThreshold {
								for {
									if last < n-1 {
										last++
									} else {
										last = 0
									}
									dy := ahAbs(int32(bestContour[last].y) - int32(bestContour[first].y))
									if dy > 5 && ahAbs(int32(bestContour[nextIx].x)-int32(bestContour[first].x)) <= 20*dist {
										if last > 0 {
											last--
										} else {
											last = n - 1
										}
										break
									}
									pLast = last
									if bestContour[last].isOnCurve() {
										pLast = last
										if pFirst < 0 {
											pFirst = last
										}
									}
									if last == segmentFirst {
										break
									}
								}
								bestY = int32(bestContour[first].y)
								segmentFirst = first
								segmentLast = last
								onPointFirst = pFirst
								onPointLast = pLast
								break
							}
							if last == segmentFirst {
								break
							}
						}
					}
				}
				bestY += yOffset
				// Is the segment round? It is flat when the horizontal
				// distance of the first and the last point on the curve is
				// larger than a heuristic threshold. Else it is round when
				// the first or the last point of the segment is not on the
				// curve.
				var isRound bool
				if onPointFirst >= 0 && onPointLast >= 0 &&
					ahAbs(int32(bestContour[onPointLast].x)-int32(bestContour[onPointFirst].x)) > flatThreshold {
					isRound = false
				} else {
					isRound = !bestContour[segmentFirst].isOnCurve() || !bestContour[segmentLast].isOnCurve()
				}
				if isRound && blueZones.isNeutral() {
					// A neutral zone ignores round segments.
					continue
				}
				if blueZones.isTop() {
					if bestY > bestYExtremum {
						bestYExtremum = bestY
						bestIsRound = isRound
					}
				} else if bestY < bestYExtremum {
					bestYExtremum = bestY
					bestIsRound = isRound
				}
			}
			if bestYExtremum != math.MinInt32 && bestYExtremum != math.MaxInt32 {
				if bestIsRound {
					if nRounds < len(rounds) {
						rounds[nRounds] = bestYExtremum
						nRounds++
					}
				} else if nFlats < len(flats) {
					flats[nFlats] = bestYExtremum
					nFlats++
				}
			}
		}
		if nFlats == 0 && nRounds == 0 {
			continue
		}
		ahSortInt32(rounds[:nRounds])
		ahSortInt32(flats[:nFlats])
		var blueRef, blueShoot int32
		switch {
		case nFlats == 0:
			blueRef = rounds[nRounds/2]
			blueShoot = blueRef
		case nRounds == 0:
			blueRef = flats[nFlats/2]
			blueShoot = blueRef
		default:
			blueRef, blueShoot = flats[nFlats/2], rounds[nRounds/2]
		}
		if blueShoot != blueRef {
			overRef := blueShoot > blueRef
			if blueZones.isTopLike() != overRef {
				val := (blueShoot + blueRef) / 2
				blueRef = val
				blueShoot = val
			}
		}
		unscaled := ahUnscaledBlue{
			position:  blueRef,
			overshoot: blueShoot,
			ascender:  ascender,
			descender: descender,
			zones:     blueZones.retainTopLikeOrNeutral(),
		}
		if blueZones.isXHeight() {
			unscaled.zones |= ahBlueAdjustment
		}
		blues = append(blues, unscaled)
	}
	// Sort the bottoms.
	var sortedIndices [ahMaxBlues]int
	for i := range sortedIndices {
		sortedIndices[i] = i
	}
	blueValues := blues
	n := len(blueValues)
	if n == 0 {
		return blues
	}
	// Sort from the bottom to the top.
	for i := 1; i < n; i++ {
		for j := i; j >= 1; j-- {
			first := &blueValues[sortedIndices[j-1]]
			second := &blueValues[sortedIndices[j]]
			a := first.overshoot
			if first.zones.isTopLike() {
				a = first.position
			}
			b := second.overshoot
			if second.zones.isTopLike() {
				b = second.position
			}
			if b >= a {
				break
			}
			sortedIndices[j], sortedIndices[j-1] = sortedIndices[j-1], sortedIndices[j]
		}
	}
	// And adjust the tops.
	for i := 0; i < n-1; i++ {
		index1 := sortedIndices[i]
		index2 := sortedIndices[i+1]
		first := &blueValues[index1]
		second := &blueValues[index2]
		a := first.position
		if first.zones.isTopLike() {
			a = first.overshoot
		}
		b := second.position
		if second.zones.isTopLike() {
			b = second.overshoot
		}
		if a > b {
			if first.zones.isTopLike() {
				blueValues[index1].overshoot = b
			} else {
				blueValues[index1].position = b
			}
		}
	}
	return blues
}

// ahSatisfiesMinLongSegmentLen is satisfies_min_long_segment_len: for
// inclusive indices and the last index of a contour, it says that the
// segment is large enough to test for bumps in the long alignment zones of
// Hebrew.
func ahSatisfiesMinLongSegmentLen(firstIx, lastIx, contourLast int) bool {
	var inclusiveDiff int
	if firstIx <= lastIx {
		inclusiveDiff = lastIx - firstIx
	} else {
		// The sum of the ranges [firstIx, contourLast] and [0, lastIx].
		inclusiveDiff = contourLast - firstIx + 1 + lastIx
	}
	// The +2 is from FreeType.
	return inclusiveDiff+2 <= contourLast
}

// ahComputeCjkBlues is compute_cjk_blues: the blue values of the CJK script
// group in font units, for the horizontal and the vertical zones. The
// horizontal set is not generated, as in FreeType.
func ahComputeCjkBlues(shaper *ahShaper, style *ahStyleClass) [2][]ahUnscaledBlue {
	var blues [2][]ahUnscaledBlue
	var flats, fills [ahBlueStringMaxLen]int16
	font := shaper.font
	clusterShaper := shaper.clusterShaper(style)
	var shapedCluster []ahShapedGlyph
	// Walk over the sets of blue characters of the script.
	for _, blue := range style.script.blues {
		blueZones := blue.zones
		isHorizontal := blueZones.isHorizontal()
		// Horizontal blue zones are disabled in FreeType.
		if isHorizontal {
			continue
		}
		isRight := blueZones.isRight()
		isTop := blueZones.isTop()
		axisBlues := &blues[1]
		if len(*axisBlues) >= ahMaxBlues {
			continue
		}
		nFlats := 0
		nFills := 0
		isFill := true
		for _, cluster := range strings.Split(blue.str, " ") {
			// The character '|' in the blue string switches to the
			// characters that define the flat values.
			if cluster == "|" {
				isFill = false
				continue
			}
			shapedCluster = clusterShaper.shape(cluster, shapedCluster)
			for _, g := range shapedCluster {
				if g.id == 0 {
					continue
				}
				glyph, ok := font.outline(g.id)
				if !ok {
					continue
				}
				points, _, ok := font.drawUnscaled(glyph)
				if !ok {
					continue
				}
				// Reject a glyph that cannot produce a rendering.
				if len(points) <= 2 {
					continue
				}
				// Find an extremum.
				takeMax := (isHorizontal && isRight) || (!isHorizontal && isTop)
				var bestPos int16
				for i, p := range points {
					pos := p.y
					if isHorizontal {
						pos = p.x
					}
					switch {
					case i == 0:
						bestPos = pos
					case takeMax:
						bestPos = max(bestPos, pos)
					default:
						bestPos = min(bestPos, pos)
					}
				}
				if isFill {
					if nFills < len(fills) {
						fills[nFills] = bestPos
						nFills++
					}
				} else if nFlats < len(flats) {
					flats[nFlats] = bestPos
					nFlats++
				}
			}
		}
		if nFlats == 0 && nFills == 0 {
			continue
		}
		// The reference and the overshoot of the blue are the medians
		// after a sort.
		sort.Slice(fills[:nFills], func(i, j int) bool { return fills[i] < fills[j] })
		sort.Slice(flats[:nFlats], func(i, j int) bool { return flats[i] < flats[j] })
		var blueRef, blueShoot int32
		switch {
		case nFlats == 0:
			blueRef = int32(fills[nFills/2])
			blueShoot = blueRef
		case nFills == 0:
			blueRef = int32(flats[nFlats/2])
			blueShoot = blueRef
		default:
			blueRef, blueShoot = int32(fills[nFills/2]), int32(flats[nFlats/2])
		}
		// Make sure that the reference is not below the overshoot for the
		// top and right zones, and not above it for the bottom and left
		// zones.
		if blueShoot != blueRef {
			underRef := blueShoot < blueRef
			if blueZones.isTop() != underRef {
				blueRef = (blueShoot + blueRef) / 2
				blueShoot = blueRef
			}
		}
		*axisBlues = append(*axisBlues, ahUnscaledBlue{
			position:  blueRef,
			overshoot: blueShoot,
			zones:     blueZones & ahBlueTop,
		})
	}
	return blues
}
