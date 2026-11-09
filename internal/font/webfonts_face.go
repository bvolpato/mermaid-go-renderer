// Ported from Chromium 156.0.8078.80 (Blink) and from Skia at revision
// aa789d9b, in part: a face of a web font gets its variation, its synthetic
// styles and its advances as in
// third_party/blink/renderer/platform/fonts/font_custom_platform_data.cc
// (FontCustomPlatformData::GetFontPlatformData),
// third_party/blink/renderer/core/css/css_segmented_font_face.cc
// (CSSSegmentedFontFace::GetFontData),
// third_party/blink/renderer/platform/fonts/skia/skia_text_metrics.cc
// (SkFontGetGlyphWidthForHarfBuzz),
// third_party/blink/renderer/platform/fonts/shaping/harfbuzz_shaper.cc
// (RoundHarfBuzzBufferPositions) and Skia src/ports/SkTypeface_fontations.cpp
// (SkFontationsScalerContext::generateMetrics; BSD 3-clause license,
// LICENSE-Skia.txt), with the scaling of skrifa 0.46.2
// (src/outline/glyf/mod.rs, src/outline/cff/mod.rs, src/metrics.rs; MIT OR
// Apache-2.0, LICENSE-skrifa.txt).
//
// The license of each Blink file is the license in its header:
//
//   - skia_text_metrics.cc: BSD-3-Clause, Copyright 2016 The Chromium
//     Authors (LICENSE-Chromium.txt).
//   - font_custom_platform_data.cc: BSD-3-Clause, with the terms in the
//     file. Copyright (C) 2007 Apple Computer, Inc. Copyright (c) 2007,
//     2008, 2009, Google Inc. All rights reserved. Copyright (C) 2010
//     Company 100, Inc.
//   - css_segmented_font_face.cc: BSD-2-Clause, with the terms in the file.
//     Copyright (C) 2008 Apple Inc. All rights reserved.
//   - harfbuzz_shaper.cc: BSD-3-Clause, with the terms in the file.
//     Copyright (c) 2012 Google Inc. All rights reserved. Copyright (C)
//     2013 BlackBerry Limited. All rights reserved.
//
// NOTICE-Blink.txt in this directory has the headers of the files that have
// the terms in the file.

package font

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/font/webfontdata"
	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/harfbuzz"
	"github.com/go-text/typesetting/language"
)

// This file has the faces of web fonts.
//
// Blink makes the font of a web font face for a text style in two steps.
// CSSSegmentedFontFace decides whether the style needs a synthetic bold or
// a synthetic italic: the weights of the rule end below 600 and the text
// has 600 or more, or the rule has no italic and the text is italic. Then
// FontCustomPlatformData sets the wght axis of a variable font to the
// weight of the text, limited to the range of the rule, and drops a
// synthetic style that the font has by itself.
//
// On Linux the font of a web font has the default font settings of the
// browser, because the browser asks fontconfig for the settings of a font
// by family name and a web font has no name there
// (FontPlatformData::QuerySystemRenderStyle). Headless Chromium has full
// hinting and no subpixel positioning as its default. So, unlike a system
// font, a web font has whole pixel advances: Skia rounds the advance of the
// hinted glyph, and Blink rounds what Skia gives and the positions that
// HarfBuzz returns. On macOS a web font is a CoreText font as every other
// font.

// webRefKey names the face of a rule for a text style.
type webRefKey struct {
	url    string
	weight int
	italic bool
	// boldCapable and italicCapable say what the segmented face of the
	// rule has, which decides the synthetic styles.
	boldCapable, italicCapable bool
	weightMin, weightMax       float64
}

// webFaceKey names a face of a web font: a font file with a value of the
// wght axis and the synthetic styles.
type webFaceKey struct {
	url string
	// wght is the value of the wght axis, or 0 for a font without the
	// axis.
	wght                           float32
	syntheticBold, syntheticItalic bool
}

// webFaceRef is the face of a web font for a text style. The font is read
// when a text first needs the face.
type webFaceRef struct {
	lib  *Library
	key  webRefKey
	once sync.Once
	face *Face
}

// webFaceRef returns the reference for a rule and a text style. The caller
// holds no lock.
func (l *Library) webFaceRef(rule *webRule, seg *webSegmented, weight int, italic bool) *webFaceRef {
	key := webRefKey{
		url: rule.url, weight: weight, italic: italic,
		boldCapable:   !(seg.weightMax < boldThreshold),
		italicCapable: !(seg.slopeMax < italicSlope),
		weightMin:     seg.weightMin, weightMax: seg.weightMax,
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.webRefs == nil {
		l.webRefs = map[webRefKey]*webFaceRef{}
	}
	ref := l.webRefs[key]
	if ref == nil {
		ref = &webFaceRef{lib: l, key: key}
		l.webRefs[key] = ref
	}
	return ref
}

// get returns the face, or nil for a font that does not load.
func (r *webFaceRef) get() *Face {
	r.once.Do(func() { r.face = r.lib.loadWebFace(r.key) })
	return r.face
}

// webFont is a web font file that was parsed. All libraries and all faces
// share it.
type webFont struct {
	once sync.Once
	data []byte
	font *gtfont.Font
	err  error
	// axes are the axes of the fvar table.
	axes []webAxis
	// weight and italic are the style of the font itself: the style of the
	// typeface of Skia (isBold, isItalic).
	weight int
	italic bool
	// glyf is true for a font with TrueType outlines.
	glyf bool
}

// webAxis is an axis of variation of a font, in the units of the user.
type webAxis struct {
	tag                 uint32
	minimum, def, maxim float32
}

const tagWght = 'w'<<24 | 'g'<<16 | 'h'<<8 | 't'

var (
	webFontsMu sync.Mutex
	webFontsBy = map[string]*webFont{}
)

// parsedWebFont returns the parsed font of a file of internal/webfonts. It
// parses a file one time for the process. parsed is true when this call
// parsed the font.
func parsedWebFont(url string) (wf *webFont, parsed bool) {
	webFontsMu.Lock()
	wf = webFontsBy[url]
	if wf == nil {
		wf = &webFont{}
		webFontsBy[url] = wf
	}
	webFontsMu.Unlock()
	wf.once.Do(func() {
		parsed = true
		wf.data, wf.err = webfontdata.Font(url)
		if wf.err != nil {
			return
		}
		loaders, err := ot.NewLoaders(bytes.NewReader(wf.data))
		if err != nil || len(loaders) == 0 {
			wf.err = fmt.Errorf("web font %s: no font", url)
			return
		}
		ld := loaders[0]
		if wf.font, wf.err = newGoTextFont(ld); wf.err != nil {
			return
		}
		desc := wf.font.Describe()
		wf.weight = int(desc.Aspect.Weight)
		if wf.weight == 0 {
			wf.weight = 400
		}
		wf.italic = desc.Aspect.Style != gtfont.StyleNormal
		_, errGlyf := ld.RawTable(ot.MustNewTag("glyf"))
		wf.glyf = errGlyf == nil
		wf.axes = readFvarAxes(ld)
	})
	return wf, parsed
}

// readFvarAxes reads the axes of the fvar table.
func readFvarAxes(ld *ot.Loader) []webAxis {
	b, err := ld.RawTable(ot.MustNewTag("fvar"))
	if err != nil || len(b) < 16 {
		return nil
	}
	offset := int(binary.BigEndian.Uint16(b[4:]))
	count := int(binary.BigEndian.Uint16(b[8:]))
	size := int(binary.BigEndian.Uint16(b[10:]))
	if size < 20 {
		return nil
	}
	fixed := func(off int) float32 { return float32(int32(binary.BigEndian.Uint32(b[off:]))) / 65536 }
	var axes []webAxis
	for i := 0; i < count; i++ {
		off := offset + i*size
		if off+20 > len(b) {
			break
		}
		axes = append(axes, webAxis{
			tag:     binary.BigEndian.Uint32(b[off:]),
			minimum: fixed(off + 4), def: fixed(off + 8), maxim: fixed(off + 12),
		})
	}
	return axes
}

// webFace is the state of a face of a web font.
type webFace struct {
	font *webFont
	// wght is the value of the wght axis, or 0.
	wght float32
	// coords are the normalized coordinates that HarfBuzz computes for the
	// variation, or nil for the default instance and for a static font.
	coords []tables.Coord
	// skCoords are the normalized coordinates that skrifa computes, or
	// nil.
	skCoords []int16
	// syntheticBold and syntheticItalic say that Skia makes the style: it
	// strokes the outline, and it skews it.
	syntheticBold, syntheticItalic bool

	// glyf reads the TrueType outlines of the font for the rules of Linux.
	glyfOnce sync.Once
	glyf     *ahFont
	// advances has the advance that Skia gives for a glyph at a font
	// size: webAdvanceKey -> float32. draws has the draws of skrifa
	// (webDrawKey -> *webDrawn) and bounds the bounds of Skia (webDrawKey
	// -> [5]float32). See webfonts_scaler.go.
	advances sync.Map
	draws    sync.Map
	bounds   sync.Map
}

type webAdvanceKey struct {
	gid  uint32
	size uint32 // the bits of the font size as a float32
}

// loadWebFace returns the face of a web font for a text style
// (FontCustomPlatformData::GetFontPlatformData), or nil for a font that
// does not load.
func (l *Library) loadWebFace(ref webRefKey) *Face {
	wf, parsed := parsedWebFont(ref.url)
	if parsed {
		l.mu.Lock()
		l.webParsing++
		l.mu.Unlock()
	}
	if wf.err != nil {
		return nil
	}
	// CSSSegmentedFontFace::GetFontData.
	bold := !ref.boldCapable && ref.weight >= boldThreshold
	italic := !ref.italicCapable && ref.italic
	key := webFaceKey{url: ref.url}
	if len(wf.axes) > 0 {
		// A variable font: the wght axis has the weight of the text in the
		// range of the rule. Skia limits it to the range of the axis.
		weight := float32(min(max(float64(ref.weight), ref.weightMin), ref.weightMax))
		for _, axis := range wf.axes {
			if axis.tag == tagWght {
				key.wght = min(max(weight, axis.minimum), axis.maxim)
			}
		}
	}
	// The font itself can have the style.
	key.syntheticBold = bold && !(wf.weight >= boldThreshold)
	key.syntheticItalic = italic && !wf.italic

	l.mu.Lock()
	if f := l.webFaces[key]; f != nil {
		l.mu.Unlock()
		return f
	}
	l.mu.Unlock()
	f := newWebFace(l.platform, wf, key)
	if f == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if kept := l.webFaces[key]; kept != nil {
		return kept
	}
	l.webFaces[key] = f
	return f
}

// newWebFace makes the face of a parsed web font.
func newWebFace(platform Platform, wf *webFont, key webFaceKey) *Face {
	// Each face has a loader of its own: a loader reads its tables with a
	// seek, so two goroutines must not share one.
	loaders, err := ot.NewLoaders(bytes.NewReader(wf.data))
	if err != nil || len(loaders) == 0 {
		return nil
	}
	ld := loaders[0]
	f := newFaceOfFont(platform, ld, wf.font, 0)
	f.Path = "webfont:" + key.url
	f.web = &webFace{font: wf, wght: key.wght, syntheticBold: key.syntheticBold, syntheticItalic: key.syntheticItalic}
	if len(wf.axes) > 0 {
		design := make([]float32, len(wf.axes))
		user := map[uint32]float32{}
		atDefault := true
		for i, axis := range wf.axes {
			design[i] = axis.def
			if axis.tag == tagWght && key.wght != 0 {
				design[i] = key.wght
			}
			user[axis.tag] = design[i]
			atDefault = atDefault && design[i] == axis.def
		}
		if !atDefault {
			// HarfBuzz and skrifa each compute the normalized coordinates
			// from the values of the user, and they round in different
			// ways.
			f.web.coords = wf.font.NormalizeVariations(design)
			f.gt.SetCoords(f.web.coords)
			f.web.skCoords = webSkrifaCoords(ld, user)
		}
		if key.wght != 0 {
			f.Weight = int(key.wght)
		}
	}
	return f
}

// isWebLinux reports whether the rules of a web font on Linux apply to the
// face.
func (f *Face) isWebLinux() bool { return f.web != nil && f.platform == PlatformLinux }

// SubpixelPositioning reports whether Skia places the glyphs of the face at
// fractions of a pixel. A web font on Linux has the default font settings
// of headless Chromium, which have no subpixel positioning: its glyphs are
// at whole pixels.
func (f *Face) SubpixelPositioning() bool { return !f.isWebLinux() }

// webNominal is the advance of a glyph in font units at the instance of
// the face, as HarfBuzz has it before it positions the glyphs.
func (f *Face) webNominal(gid uint32) int32 {
	return int32(math.Floor(float64(f.gt.HorizontalAdvance(gtfont.GID(gid))) + 0.5))
}

// webSkiaAdvance is the advance that Skia gives Blink for a glyph of a web
// font on Linux (SkFontationsScalerContext::generateMetrics and
// SkFont::getWidthsBounds).
//
// A web font has normal hinting, so the advance is the advance of the
// hinted glyph, rounded to a whole pixel. Skia keeps the linear advance
// when the hinted advance is zero. A font with a synthetic italic has a
// skew, and Skia does not hint a font that is not along an axis: it gives
// the linear advance. Above 256px Skia measures a font of 64px without
// hinting and scales the value.
func (f *Face) webSkiaAdvance(gid uint32, size float64) float32 {
	key := webAdvanceKey{gid, math.Float32bits(float32(size))}
	if v, ok := f.web.advances.Load(key); ok {
		return v.(float32)
	}
	adv := f.webSkiaAdvanceUncached(gid, size)
	f.web.advances.Store(key, adv)
	return adv
}

func (f *Face) webSkiaAdvanceUncached(gid uint32, size float64) float32 {
	units := f.webLinearUnits(gid)
	if float32(size) > canonicalLimit {
		linear := ftMulDiv(int64(units), f.ft.scale(canonicalSize), 64)
		advance := float32(linear) / 65536
		return float32(advance * float32(float32(size)/canonicalSize))
	}
	linear := float32(ftMulDiv(int64(units), f.ft.scale(size), 64)) / 65536 // GlyphMetrics::advance_width
	if f.web.syntheticItalic {
		return linear
	}
	adjusted, ok := f.webHintedAdvance(gid, size)
	if !ok {
		return linear
	}
	// The hinted draw of skrifa rounds the advance to a whole pixel, and
	// Skia rounds the float again (roundf).
	hinted := float32(math.Round(float64(adjusted)))
	if hinted != linear && hinted != 0 {
		return hinted
	}
	return linear
}

// shapeWebLinux shapes a run of text with a face of a web font on Linux.
// It is Face.shape with the advances and the rounding of a font that has
// no subpixel positioning.
func (f *Face) shapeWebLinux(text []rune, size float64, script language.Script) []Glyph {
	text = f.linuxShapeText(text)
	sh := f.shaper.Get().(*shaper)
	defer f.shaper.Put(sh)

	sh.font.XScale = int32(f.upem)
	sh.font.YScale = int32(f.upem)
	sh.font.Ptem = float32(size)
	sh.buf.Clear()
	sh.buf.AddRunes(text, 0, -1)
	sh.buf.Props.Direction = harfbuzz.LeftToRight
	sh.buf.Props.Script = script
	sh.buf.Props.Language = language.NewLanguage("en")
	sh.buf.Shape(sh.font, nil)

	hbScale := int64(float32(size) * 65536)
	upem := int64(f.upem)
	mult := (hbScale << 16) / upem
	scale16 := func(units int32) int64 { return (int64(units)*mult + 32768) >> 16 }
	out := make([]Glyph, len(sh.buf.Info))
	nominals := make([]int32, len(sh.buf.Info))
	for i, info := range sh.buf.Info {
		nominals[i] = f.webNominal(uint32(info.Glyph))
	}
	kernAdv, kernOff, machine := f.machineKerning(sh.buf.Info, sh.buf.Pos, nominals, scale16)
	for i, info := range sh.buf.Info {
		pos := sh.buf.Pos[i]
		extra := pos.XAdvance - nominals[i]
		// SkFontGetGlyphWidthForHarfBuzz: the font has no subpixel
		// positioning, so the advance of Skia is rounded to a whole pixel
		// (SkScalarRoundToScalar).
		sk := f.webSkiaAdvance(uint32(info.Glyph), size)
		adv16 := int64(float32(math.Floor(float64(sk+0.5))) * 65536)
		xOff16 := scale16(pos.XOffset)
		if machine {
			adv16 += kernAdv[i]
			xOff16 = kernOff[i]
		} else if extra != 0 {
			adv16 += scale16(extra)
		}
		if pos.XAdvance == 0 {
			// HarfBuzz gave the glyph no advance: a character that is not
			// drawn, such as a soft hyphen, or a mark.
			adv16 = 0
		}
		out[i] = Glyph{
			ID:      uint32(info.Glyph),
			Cluster: info.Cluster,
			Advance: roundHarfBuzzPosition(adv16),
			XOffset: roundHarfBuzzPosition(xOff16),
			YOffset: roundHarfBuzzPosition(scale16(pos.YOffset)),
		}
	}
	return out
}

// roundHarfBuzzPosition is RoundHarfBuzzPosition of Blink: a 16.16 value
// becomes a whole number of pixels. The value goes through a float32, and
// a half rounds away from zero.
func roundHarfBuzzPosition(v int64) float64 {
	if v&0xffff == 0 {
		return float64(v >> 16)
	}
	return math.Round(float64(float32(v) / 65536))
}

// webCoreTextAdvance is the advance in px that CoreText gives Skia for a
// glyph of an instance of a variable font on macOS, for an advance in font
// units. For an instance that is not the default instance, CoreText keeps
// the advance as a 16.16 fraction of the em and cuts the rest, then
// multiplies by the font size: at 1000px the letter a of Recursive, whose
// advance is 600 units of 1000, measures 599.9908447265625px at the weight
// 400 and 600px at the weight 300, the default.
func (f *Face) webCoreTextAdvance(nominal int32, size float64) float32 {
	em := math.Floor(float64(nominal) * 65536 / f.upem)
	return float32(float64(em/65536) * size)
}
