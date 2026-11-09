// Ported from skrifa 0.46.2 (src/outline/autohint/metrics/scale.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/aflatin.c: af_latin_metrics_scale_dim, src/autofit/afcjk.c:
// af_cjk_metrics_scale_dim), which has the FreeType License
// (LICENSE-FreeType.txt).

package font

// ahComputeUnscaledStyleMetrics is compute_unscaled_style_metrics: the
// metrics of a style in font units.
func ahComputeUnscaledStyleMetrics(shaper *ahShaper, style *ahStyleClass, quirksAot bool) *ahUnscaledStyleMetrics {
	charmap := &shaper.font.charmap
	m := &ahUnscaledStyleMetrics{classIx: uint16(style.index)}
	m.axes[0].dim = ahHorizontal
	m.axes[1].dim = ahVertical
	// There are no metrics for a font without a Unicode character map.
	if charmap.isSymbol {
		return m
	}
	widths := ahComputeWidths(shaper, style, quirksAot)
	blues := ahComputeUnscaledBlues(shaper, style)
	haveDigitAdvance := false
	var digitAdvance int32
	m.digitsHaveSameWidth = true
	for ch := uint32('0'); ch <= '9'; ch++ {
		gid, ok := charmap.mapChar(ch)
		if !ok {
			continue
		}
		advance := shaper.font.advanceWidth(gid)
		if haveDigitAdvance && digitAdvance != advance {
			m.digitsHaveSameWidth = false
			break
		}
		haveDigitAdvance, digitAdvance = true, advance
	}
	for i := range m.axes {
		m.axes[i].blues = blues[i]
		m.axes[i].widthMetrics = widths[i].metrics
		m.axes[i].widths = widths[i].widths
	}
	return m
}

// ahScaleStyleMetrics is scale_style_metrics: the scaled metrics of a style.
func ahScaleStyleMetrics(unscaledMetrics *ahUnscaledStyleMetrics, scale ahScale, quirksAot bool) *ahScaledStyleMetrics {
	scaleAxis := ahScaleCjkAxisMetrics
	if unscaledMetrics.styleClass().script.group == ahGroupDefault {
		scaleAxis = ahScaleDefaultAxisMetrics
	}
	scaled := &ahScaledStyleMetrics{}
	for i := range unscaledMetrics.axes {
		axis := &unscaledMetrics.axes[i]
		scaled.axes[i] = scaleAxis(axis.dim, axis.widths, axis.widthMetrics, axis.blues, &scale, quirksAot)
	}
	scaled.scale = scale
	return scaled
}

// ahScaleDefaultAxisMetrics is scale_default_axis_metrics: the scaled
// metrics of one axis.
func ahScaleDefaultAxisMetrics(dim ahDimension, widths []int32, widthMetrics ahWidthMetrics, blues []ahUnscaledBlue, scale *ahScale, quirksAot bool) ahScaledAxisMetrics {
	axis := ahScaledAxisMetrics{dim: dim}
	if dim == ahHorizontal {
		axis.scale = scale.xScale
		axis.delta = scale.xDelta
	} else {
		axis.scale = scale.yScale
		axis.delta = scale.yDelta
	}
	// Correct the Y scale to make the alignment better.
	for blueIx := range blues {
		if !blues[blueIx].zones.contains(ahBlueAdjustment) {
			continue
		}
		unscaledBlue := &blues[blueIx]
		scaled := ahFixedMul(axis.scale, unscaledBlue.overshoot)
		fitted := (scaled + 40) &^ 63
		if scaled != fitted && dim == ahVertical {
			newScale := ahFixedMulDiv(axis.scale, fitted, scaled)
			// The scale must not change a height by more than 2 pixels.
			maxHeight := scale.unitsPerEm
			for i := range blues {
				maxHeight = max(max(maxHeight, blues[i].ascender), -blues[i].descender)
			}
			dist := ahAbs(ahFixedMul(maxHeight, newScale-axis.scale))
			dist &^= 127
			if dist == 0 {
				axis.scale = newScale
				scale.yScale = newScale
			}
		}
		break
	}
	// Scale the widths. FreeType has one width at least: the standard
	// width.
	axis.widthMetrics = widthMetrics
	if len(widths) == 0 && quirksAot {
		scaled := ahFixedMul(axis.scale, axis.widthMetrics.standardWidth)
		axis.widths = append(axis.widths, ahScaledWidth{scaled: scaled, fitted: scaled})
	} else {
		for _, unscaledWidth := range widths {
			scaled := ahFixedMul(axis.scale, unscaledWidth)
			axis.widths = append(axis.widths, ahScaledWidth{scaled: scaled, fitted: scaled})
		}
	}
	// The font is extra light when the standard width is less than 5/8
	// pixels.
	axis.widthMetrics.isExtraLight = ahFixedMul(axis.widthMetrics.standardWidth, axis.scale) < (32 + 8)
	if dim == ahVertical {
		// Scale the blue zones.
		for i := range blues {
			unscaledBlue := &blues[i]
			scaledPosition := ahFixedMul(axis.scale, unscaledBlue.position) + axis.delta
			scaledOvershoot := ahFixedMul(axis.scale, unscaledBlue.overshoot) + axis.delta
			blue := ahScaledBlue{
				position:  ahScaledWidth{scaled: scaledPosition, fitted: scaledPosition},
				overshoot: ahScaledWidth{scaled: scaledOvershoot, fitted: scaledOvershoot},
				zones:     unscaledBlue.zones,
				isActive:  false,
			}
			// Only a blue zone that is less than 3/4 pixels tall is
			// active.
			dist := ahFixedMul(unscaledBlue.position-unscaledBlue.overshoot, axis.scale)
			if dist >= -48 && dist <= 48 {
				delta := ahAbs(dist)
				if delta < 32 {
					delta = 0
				} else if delta < 48 {
					delta = 32
				} else {
					delta = 64
				}
				if dist < 0 {
					delta = -delta
				}
				blue.position.fitted = ahPixRound(blue.position.scaled)
				blue.overshoot.fitted = blue.position.fitted - delta
				blue.isActive = true
			}
			axis.blues = append(axis.blues, blue)
		}
		// Use a sub-top blue zone when it does not overlap with another
		// blue zone that is not sub-top.
		for blueIx := range axis.blues {
			blue := axis.blues[blueIx]
			if !blue.zones.isSubTop() || !blue.isActive {
				continue
			}
			for i := range axis.blues {
				blue2 := &axis.blues[i]
				if blue2.zones.isSubTop() || !blue2.isActive {
					continue
				}
				if blue2.position.fitted <= blue.overshoot.fitted && blue2.overshoot.fitted >= blue.position.fitted {
					axis.blues[blueIx].isActive = false
					break
				}
			}
		}
	}
	return axis
}

// ahScaleCjkAxisMetrics is scale_cjk_axis_metrics: the scaled metrics of one
// axis for the CJK script group.
func ahScaleCjkAxisMetrics(dim ahDimension, widths []int32, widthMetrics ahWidthMetrics, blues []ahUnscaledBlue, scaleParams *ahScale, _ bool) ahScaledAxisMetrics {
	axis := ahScaledAxisMetrics{dim: dim}
	if dim == ahHorizontal {
		axis.scale = scaleParams.xScale
		axis.delta = scaleParams.xDelta
	} else {
		axis.scale = scaleParams.yScale
		axis.delta = scaleParams.yDelta
	}
	scale := axis.scale
	// Scale the blue zones.
	for i := range blues {
		unscaledBlue := &blues[i]
		position := ahFixedMul(unscaledBlue.position, scale) + axis.delta
		overshoot := ahFixedMul(unscaledBlue.overshoot, scale) + axis.delta
		blue := ahScaledBlue{
			position:  ahScaledWidth{scaled: position, fitted: position},
			overshoot: ahScaledWidth{scaled: overshoot, fitted: overshoot},
			zones:     unscaledBlue.zones,
			isActive:  false,
		}
		// A blue zone is active only when it is less than 3/4 pixels tall.
		dist := ahFixedMul(unscaledBlue.position-unscaledBlue.overshoot, scale)
		if dist >= -48 && dist <= 48 {
			blue.position.fitted = ahPixRound(blue.position.scaled)
			// For CJK the overshoot is an undershoot.
			delta1 := ahFixedDiv(blue.position.fitted, scale) - unscaledBlue.overshoot
			delta2 := ahFixedMul(ahAbs(delta1), scale)
			if delta2 < 32 {
				delta2 = 0
			} else {
				delta2 = ahPixRound(delta2)
			}
			if delta1 < 0 {
				delta2 = -delta2
			}
			blue.overshoot.fitted = blue.position.fitted - delta2
			blue.isActive = true
		}
		axis.blues = append(axis.blues, blue)
	}
	// FreeType does not compute the scaled widths, and skrifa does the
	// same.
	for range widths {
		axis.widths = append(axis.widths, ahScaledWidth{})
	}
	axis.widthMetrics = widthMetrics
	return axis
}
