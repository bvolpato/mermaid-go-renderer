// Ported from skrifa 0.46.2 (src/outline/autohint/instance.rs,
// src/outline/autohint/metrics/mod.rs: UnscaledStyleMetricsSet, and
// src/outline/hint.rs: HintingInstance with the automatic engine) and from
// Skia at revision aa789d9b (src/ports/SkTypeface_fontations.cpp:
// SkFontationsScalerContext, src/ports/fontations/src/hinting.rs,
// src/ports/fontations/src/verbs_points_pen.rs; BSD 3-clause license,
// LICENSE-Skia.txt).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/afloader.c, src/autofit/afmodule.c), which has the FreeType
// License (LICENSE-FreeType.txt).

package font

import (
	"math"
	"sync"

	ot "github.com/go-text/typesetting/font/opentype"
)

// Headless Chromium on Linux uses slight hinting. For that, the scaler
// context of Skia makes a hinting instance of skrifa that forces the
// automatic hinter for TrueType outlines, with the light smooth target
// (make_hinting_instance with do_light_hinting). Skia draws the outline of a
// glyph through that instance, and the ink bounds of the glyph are the
// bounds of the points of the path.
//
// The files autohint_*.go are a port of the automatic hinter of skrifa
// (src/outline/autohint), which is a port of the autofit module of FreeType.
// The names of skrifa are kept, with the prefix ah.
//
// The hinting style of a font comes from fontconfig. Slight hinting is the
// default. A font with the hint style none gets no hinting instance, and
// Skia draws its outline without hinting (unhintedCBox). The fontconfig
// files of Ubuntu set that style for WenQuanYi Zen Hei, and for IPAGothic up
// to 18px.
//
// Skia does not use the automatic hinter for a font that FreeType calls
// tricky (hinting_reliant in src/ports/fontations/src/hinting.rs). Such a
// font needs the TrueType interpreter, which is not ported. A font without
// TrueType outlines has no automatic hinter here also: Skia uses the hinter
// of the PostScript interpreter for it.

// ahInstance is Instance of skrifa: the state of the automatic hinter for
// one font. Many goroutines can use it at the same time.
type ahInstance struct {
	font *ahFont
	// styles has the style of each glyph (GlyphStyles). It is computed once
	// for a font.
	styles *ahGlyphStyleMap
	// metrics has the metrics of each style of the font in font units. An
	// entry is computed on first use (UnscaledStyleMetricsSet::Lazy).
	metrics []ahLazyMetrics
	target  ahTarget
	// scaled has the scaled metrics of a style at a font size:
	// ahScaledKey -> *ahScaledStyleMetrics.
	scaled sync.Map
	// boxes has the bounds of the path of a glyph at a font size, with the
	// automatic hinter or without hinting: ahBoxKey -> ftCBox.
	boxes sync.Map
}

type ahLazyMetrics struct {
	once    sync.Once
	metrics *ahUnscaledStyleMetrics
}

type ahScaledKey struct {
	classIx uint16
	size    uint32 // the bits of the font size as a float32
}

type ahBoxKey struct {
	gid    uint32
	size   uint32 // the bits of the font size as a float32
	hinted bool
}

// newAHInstance is GlyphStyles::new and Instance::new. It returns nil for a
// font without TrueType outlines.
func newAHInstance(ld *ot.Loader) *ahInstance {
	font := newAHFont(ld)
	if font == nil {
		return nil
	}
	shaper := newAHShaper(font, ahShaperBestEffort)
	styles := newAHGlyphStyleMap(uint32(font.glyphCount), shaper)
	return &ahInstance{
		font:    font,
		styles:  styles,
		metrics: make([]ahLazyMetrics, styles.metricsCount),
		// The hinting target of Skia for slight hinting: Target::Smooth
		// with SmoothMode::Light.
		target: ahTarget{mode: ahSmoothLight, symmetricRendering: true},
	}
}

// unscaledMetrics is UnscaledStyleMetricsSet::get: the metrics of the style
// of a glyph in font units, or nil.
func (inst *ahInstance) unscaledMetrics(style ahGlyphStyle) *ahUnscaledStyleMetrics {
	index, ok := inst.styles.metricsIndex(style)
	if !ok || index >= len(inst.metrics) {
		return nil
	}
	styleClass := style.styleClass()
	if styleClass == nil {
		return nil
	}
	entry := &inst.metrics[index]
	entry.once.Do(func() {
		shaper := newAHShaper(inst.font, ahShaperBestEffort)
		entry.metrics = ahComputeUnscaledStyleMetrics(shaper, styleClass, false)
	})
	return entry.metrics
}

// scaledMetrics returns the metrics of a style for a font size: Scale::new
// and scale_style_metrics. skrifa computes them for each glyph. They depend
// only on the style and the font size.
func (inst *ahInstance) scaledMetrics(metrics *ahUnscaledStyleMetrics, size float32) *ahScaledStyleMetrics {
	key := ahScaledKey{metrics.classIx, math.Float32bits(size)}
	if v, ok := inst.scaled.Load(key); ok {
		return v.(*ahScaledStyleMetrics)
	}
	scale := newAHScale(size, int32(inst.font.unitsPerEm), inst.font.isItalic, inst.target, metrics.styleClass().script.group)
	v, _ := inst.scaled.LoadOrStore(key, ahScaleStyleMetrics(metrics, scale, false))
	return v.(*ahScaledStyleMetrics)
}

// draw is Instance::draw, with the pen of Skia: the bounds of the points of
// the path of the hinted outline of a glyph, in 26.6 px with y up. The box
// is not ok for a glyph without a path.
func (inst *ahInstance) draw(glyphID uint32, size float32) ftCBox {
	outline, ok := inst.hint(glyphID, size)
	if !ok {
		return ftCBox{}
	}
	return outline.pathBounds()
}

// path is Instance::draw or OutlineGlyph::draw_unhinted with the pen of
// Skia (get_path_verbs_points): the segments of the path of the outline of
// a glyph at a font size, in px with y up. hinted selects the outline of the
// automatic hinter. A glyph without a path has no segments.
func (inst *ahInstance) path(glyphID uint32, size float32, hinted bool) []ot.Segment {
	var pen ahPathPen
	if hinted {
		outline, ok := inst.hint(glyphID, size)
		if !ok || !outline.path(&pen) {
			return nil
		}
		return pen.segments
	}
	glyph, ok := inst.font.outline(glyphID)
	if !ok || !inst.font.unhintedPath(glyph, size, &pen) {
		return nil
	}
	return pen.segments
}

// hint is Instance::draw before the pen gets the path: the outline of a
// glyph at a font size after the automatic hinter. The result is false for
// a glyph that skrifa cannot draw.
func (inst *ahInstance) hint(glyphID uint32, size float32) (*ahOutline, bool) {
	font := inst.font
	// get_path_verbs_points of Skia: a glyph that skrifa cannot load has no
	// path.
	glyph, ok := font.outline(glyphID)
	if !ok {
		return nil, false
	}
	style, ok := inst.styles.style(glyphID)
	if !ok {
		return nil, false
	}
	metrics := inst.unscaledMetrics(style)
	if metrics == nil {
		return nil, false
	}
	scaledMetrics := inst.scaledMetrics(metrics, size)
	scale := &scaledMetrics.scale
	var outline ahOutline
	if !outline.fill(font, glyph, false) {
		return nil, false
	}
	hintedMetrics := ahHintOutline(&outline, metrics, scaledMetrics, style, true)
	hAdvance := outline.advance
	var pp1x int32
	pp2x := ahFixedMul(hAdvance, hintedMetrics.xScale)
	isLight := inst.target.isLight() || inst.target.preserveLinear()
	if !isLight {
		if !scale.flags.contains(ahScaleNoAdvance) && hintedMetrics.hasEdgeMetrics {
			edgeMetrics := hintedMetrics.edgeMetrics
			oldRsb := pp2x - edgeMetrics.rightOpos
			oldLsb := edgeMetrics.leftOpos
			newLsb := edgeMetrics.leftPos
			pp1xUh := newLsb - oldLsb
			pp2xUh := edgeMetrics.rightPos + oldRsb
			if oldLsb < 24 {
				pp1xUh -= 8
			}
			if oldRsb < 24 {
				pp2xUh += 8
			}
			pp1x = ahPixRound(pp1xUh)
			pp2x = ahPixRound(pp2xUh)
			if pp1x >= newLsb && oldLsb > 0 {
				pp1x -= 64
			}
			if pp2x <= edgeMetrics.rightPos && oldRsb > 0 {
				pp2x += 64
			}
		} else {
			pp1x = ahPixRound(pp1x)
			pp2x = ahPixRound(pp2x)
		}
	} else {
		pp1x = ahPixRound(pp1x)
		pp2x = ahPixRound(pp2x)
	}
	if pp1x != 0 {
		for i := range outline.points {
			outline.points[i].x -= pp1x
		}
	}
	// The advance of the hinted glyph is not computed here: Skia uses the
	// linear advance for slight hinting.
	_ = pp2x
	return &outline, true
}

// cbox returns the bounds of the path of the outline of a glyph at a font
// size, with the automatic hinter or without hinting.
func (inst *ahInstance) cbox(glyphID uint32, size float32, hinted bool) ftCBox {
	key := ahBoxKey{glyphID, math.Float32bits(size), hinted}
	if v, ok := inst.boxes.Load(key); ok {
		return v.(ftCBox)
	}
	var box ftCBox
	if hinted {
		box = inst.draw(glyphID, size)
	} else if glyph, ok := inst.font.outline(glyphID); ok {
		box = inst.font.unhintedPathBounds(glyph, size)
	}
	inst.boxes.Store(key, box)
	return box
}

// autohinter returns the automatic hinter of the face, or nil for a font
// without TrueType outlines. It is made once for a face.
func (f *Face) autohinter() *ahInstance {
	t := f.ft
	if t == nil {
		return nil
	}
	t.hinterOnce.Do(func() { t.hinter = newAHInstance(t.ld) })
	return t.hinter
}

// hintedCBox returns the bounds of the points of the outline of a glyph at
// a font size, as the automatic hinter of skrifa produces the outline in
// its light mode. The second result is false for a font that has no
// TrueType outlines.
//
// size is the font size in px. skrifa gets it as a float32
// (Size::new(fScale.y()) in make_hinting_instance).
func (f *Face) hintedCBox(gid uint32, size float64) (ftCBox, bool) {
	inst := f.autohinter()
	if inst == nil {
		return ftCBox{}, false
	}
	return inst.cbox(gid, float32(size), true), true
}

// unhintedCBox returns the bounds of the points of the path that Skia makes
// from the outline of a glyph at a font size without hinting
// (DrawSettings::unhinted of skrifa and VerbsPointsPen of Skia). The second
// result is false for a font that has no TrueType outlines.
//
// The scaler of skrifa loads a composite glyph as FreeType does: each
// component is scaled alone and then moved. The pen of Skia starts a
// contour at the first segment that moves, so a contour whose points are
// all at one position is not in the path.
func (f *Face) unhintedCBox(gid uint32, size float64) (ftCBox, bool) {
	inst := f.autohinter()
	if inst == nil {
		return ftCBox{}, false
	}
	return inst.cbox(gid, float32(size), false), true
}
