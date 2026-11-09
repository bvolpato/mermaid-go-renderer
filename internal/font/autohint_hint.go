// Ported from skrifa 0.46.2 (src/outline/autohint/hint/mod.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/aflatin.c, src/autofit/afcjk.c, src/autofit/afloader.c),
// which has the FreeType License (LICENSE-FreeType.txt).

package font

// ahEdgeMetrics is EdgeMetrics: the positions of the outer edges, for the
// adjustment of the horizontal metrics.
type ahEdgeMetrics struct {
	leftOpos, leftPos   int32
	rightOpos, rightPos int32
}

// ahHintedMetrics is HintedMetrics: the adjusted horizontal scale, and the
// positions of the outer edges. hasEdgeMetrics is false for an outline with
// fewer than two horizontal edges.
type ahHintedMetrics struct {
	xScale         int32
	edgeMetrics    ahEdgeMetrics
	hasEdgeMetrics bool
}

// ahHintOutline is hint_outline: it applies the complete hinting process to
// an outline. scaledMetrics are the metrics of the style of the glyph for
// the scale (scale_style_metrics). skrifa computes them in this function.
// They depend only on the style and the scale, so the caller keeps them.
// hasGlyphStyle is false for an outline without a glyph style.
func ahHintOutline(outline *ahOutline, metrics *ahUnscaledStyleMetrics, scaledMetrics *ahScaledStyleMetrics, glyphStyle ahGlyphStyle, hasGlyphStyle bool) ahHintedMetrics {
	scale := &scaledMetrics.scale
	var axis ahAxis
	hintTopToBottom := metrics.styleClass().script.hintTopToBottom
	outline.scale(&scaledMetrics.scale)
	hintedMetrics := ahHintedMetrics{xScale: scale.xScale}
	group := metrics.styleClass().script.group
	// The default script group does not hint an outline when the alignment
	// zones are missing. FreeType uses a dummy hinter there.
	if group == ahGroupDefault && len(scaledMetrics.axes[1].blues) == 0 {
		return hintedMetrics
	}
	for dim := ahHorizontal; dim <= ahVertical; dim++ {
		if (dim == ahHorizontal && scale.flags.contains(ahScaleNoHorizontal)) ||
			(dim == ahVertical && scale.flags.contains(ahScaleNoVertical)) {
			continue
		}
		axis.reset(dim, outline.orientation)
		ahComputeSegments(outline, &axis, group)
		maxWidth, _ := metrics.axes[dim].maxWidth()
		ahLinkSegments(outline, &axis, scaledMetrics.axes[dim].scale, group, maxWidth)
		ahComputeEdges(&axis, &scaledMetrics.axes[dim], hintTopToBottom, scaledMetrics.scale.yScale, group)
		if dim == ahVertical {
			if group != ahGroupDefault || !hasGlyphStyle || !glyphStyle.isNonBase() {
				ahComputeBlueEdges(&axis, scale, metrics.axes[dim].blues, scaledMetrics.axes[dim].blues, group)
			}
		} else {
			hintedMetrics.xScale = scaledMetrics.axes[0].scale
		}
		ahHintEdges(&axis, &scaledMetrics.axes[dim], group, scale, hintTopToBottom)
		ahAlignEdgePoints(outline, &axis, group, scale)
		ahAlignStrongPoints(outline, &axis)
		ahAlignWeakPoints(outline, dim)
		if dim == ahHorizontal && len(axis.edges) > 1 {
			left := &axis.edges[0]
			right := &axis.edges[len(axis.edges)-1]
			hintedMetrics.edgeMetrics = ahEdgeMetrics{
				leftPos:   left.pos,
				leftOpos:  left.opos,
				rightPos:  right.pos,
				rightOpos: right.opos,
			}
			hintedMetrics.hasEdgeMetrics = true
		}
	}
	return hintedMetrics
}
