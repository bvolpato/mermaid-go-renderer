// Ported from skrifa 0.46.2 (src/outline/autohint/metrics/widths.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/aflatin.c: af_latin_metrics_init_widths, src/autofit/afcjk.c:
// af_cjk_metrics_init_widths), which has the FreeType License
// (LICENSE-FreeType.txt).

package font

import "strings"

// ahWidths has the width metrics and the widths in font units of one
// dimension.
type ahWidths struct {
	metrics ahWidthMetrics
	widths  []int32
}

// ahComputeWidths is compute_widths: all stem widths, and the standard
// width and height of a script, for each dimension.
func ahComputeWidths(shaper *ahShaper, style *ahStyleClass, quirksAot bool) [2]ahWidths {
	var result [2]ahWidths
	font := shaper.font
	unitsPerEm := int32(font.unitsPerEm)
	var outline ahOutline
	var axis ahAxis
	clusterShaper := shaper.clusterShaper(style)
	var shapedCluster []ahShapedGlyph
	// The first glyph of the set of standard characters that the font has.
	var glyph ahGlyfOutline
	haveGlyph := false
	for _, cluster := range strings.Split(style.script.stdChars, " ") {
		shapedCluster = clusterShaper.shape(cluster, shapedCluster)
		// Reject input that maps to more than one glyph.
		if len(shapedCluster) != 1 || shapedCluster[0].id == 0 {
			continue
		}
		if g, ok := font.outline(shapedCluster[0].id); ok {
			glyph, haveGlyph = g, true
			break
		}
	}
	if haveGlyph {
		if outline.fill(font, glyph, quirksAot) && len(outline.points) != 0 {
			// Process each dimension.
			for dim := ahHorizontal; dim <= ahVertical; dim++ {
				widths := &result[dim].widths
				axis.reset(dim, outline.orientation)
				// The segments for the widths always use the default
				// script group.
				ahComputeSegments(&outline, &axis, ahGroupDefault)
				ahLinkSegments(&outline, &axis, 0, ahGroupDefault, 0)
				segments := axis.segments
				for segmentIx := range segments {
					segment := &segments[segmentIx]
					if !segment.linkIx.ok {
						continue
					}
					linkIx := int(segment.linkIx.ix)
					link := &segments[linkIx]
					if linkIx > segmentIx && link.linkIx == ahSome(segmentIx) {
						dist := ahAbs(int32(segment.pos) - int32(link.pos))
						if len(*widths) < ahMaxWidths {
							*widths = append(*widths, dist)
						} else {
							break
						}
					}
				}
				// FreeType always has one width, even when no width was
				// found.
				if len(*widths) == 0 {
					*widths = append(*widths, 0)
				}
				// The value 100 is a heuristic value.
				*widths = ahSortAndQuantizeWidths(*widths, unitsPerEm/100)
			}
		}
	}
	for i := range result {
		// Set the derived values.
		stdw := ahDerivedConstant(unitsPerEm, 50)
		if len(result[i].widths) != 0 {
			stdw = result[i].widths[0]
		}
		// A heuristic value: 20% of the smallest width.
		result[i].metrics.edgeDistanceThreshold = stdw / 5
		result[i].metrics.standardWidth = stdw
		result[i].metrics.isExtraLight = false
	}
	return result
}
