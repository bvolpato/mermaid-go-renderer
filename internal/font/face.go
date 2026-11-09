// Ported from Chromium (Blink), in part: the vertical metrics of a face
// follow FontMetrics::AscentDescentWithHacks and
// SimpleFontData::PlatformInit, which the comments name
// (third_party/blink/renderer/platform/fonts/font_metrics.cc and
// simple_font_data.cc). The two files have the BSD-3-Clause license, with
// the terms in the file and the copyright of Apple and Alexey Proskuryakov,
// at Chromium 156.0.8078.80 and 156.0.8078.4. NOTICE-Blink.txt in this
// directory has their headers. The rest of the file ports no upstream
// file. It loads a font face and shapes text with go-text/typesetting.

// Package font loads fonts and measures text the way Chromium does.
//
// Layout in mermaid depends on text measurements from the browser. To match
// them, this package uses the same font files, shapes text with a port of
// HarfBuzz (the shaper inside Chromium), and applies the same number
// handling: 16.16 fixed-point glyph advances, float32 accumulation, and
// integer-rounded ascent and descent.
package font

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"

	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/harfbuzz"
	"github.com/go-text/typesetting/language"
)

// Face is one font face: a family at one weight and slant.
type Face struct {
	Family string
	Weight int // 100 to 900
	Italic bool
	Path   string
	Index  int // index in a font collection

	upem                                  float64
	hheaAscent, hheaDescent, hheaLineGap  float64 // font units, descent is positive
	typoAscent, typoDescent, typoLineGap  float64
	winAscent, winDescent                 float64
	useTypoMetrics                        bool
	xHeight, capHeight                    float64
	underlinePosition, underlineThickness float64

	// gt is the go-text face. Many goroutines share it, so only its
	// methods that do not write are called on it. See concurrent.go.
	gt     *gtfont.Face
	glyphs glyphCache // character -> glyph id
	shaper sync.Pool  // *shaper
	bounds sync.Map   // glyph id -> [5]float32 outline bounds

	// platform selects the rules of the font rasterizer: CoreText on
	// macOS, the Fontations backend of Skia on Linux.
	platform Platform
	// ft has the tables that the rules of Linux read. See
	// linux_scaler.go.
	ft *ftFace
	// defaultStyle marks a face that Blink found as the fallback font for
	// a character on Linux. Such a font has the default font settings of
	// the browser and not the settings of fontconfig. See
	// linuxAdvance16.
	defaultStyle bool

	// ascentHack is the 15% ascent adjustment that Blink applies on macOS
	// to Times, Helvetica and Courier.
	ascentHack bool
}

type shaper struct {
	font *harfbuzz.Font
	buf  *harfbuzz.Buffer
}

// LoadFile reads the faces of a font file (.ttf, .otf, .ttc, .otc). The
// faces apply the rules of the platform of this machine.
func LoadFile(path string) ([]*Face, error) {
	return loadFileFor(DefaultPlatform(), path)
}

func loadFileFor(platform Platform, path string) ([]*Face, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	faces, err := LoadFor(platform, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for _, f := range faces {
		f.Path = path
	}
	return faces, nil
}

// Load parses font data. A collection yields several faces. The faces
// apply the rules of the platform of this machine.
func Load(data []byte) ([]*Face, error) {
	return LoadFor(DefaultPlatform(), data)
}

// LoadFor parses font data. The faces apply the rules of the given
// platform.
func LoadFor(platform Platform, data []byte) ([]*Face, error) {
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var faces []*Face
	for i, ld := range loaders {
		f, err := newFace(platform, ld, i)
		if err != nil {
			continue
		}
		faces = append(faces, f)
	}
	if len(faces) == 0 {
		return nil, fmt.Errorf("no usable face")
	}
	return faces, nil
}

// loadFaceAt parses one face of font data: the face at an index of a
// collection, or the face 0 of a single font. The other faces of a
// collection are not parsed.
func loadFaceAt(platform Platform, data []byte, index int) (*Face, error) {
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(loaders) {
		return nil, fmt.Errorf("no face %d", index)
	}
	return newFace(platform, loaders[index], index)
}

// newFace parses the face of a loader.
func newFace(platform Platform, ld *ot.Loader, index int) (*Face, error) {
	ft, err := newGoTextFont(ld)
	if err != nil {
		return nil, err
	}
	f := &Face{Index: index, gt: gtfont.NewFace(ft), platform: platform}
	desc := ft.Describe()
	f.Family = desc.Family
	f.Weight = int(desc.Aspect.Weight)
	if f.Weight == 0 {
		f.Weight = 400
	}
	f.Italic = desc.Aspect.Style != gtfont.StyleNormal
	f.upem = float64(ft.Upem())
	f.readTables(ld)
	f.ft = newFTFace(ld, f)
	f.shaper.New = func() any { return f.newShaper() }
	return f, nil
}

func (f *Face) readTables(ld *ot.Loader) {
	if b, err := ld.RawTable(ot.MustNewTag("hhea")); err == nil && len(b) >= 10 {
		f.hheaAscent = float64(int16(binary.BigEndian.Uint16(b[4:])))
		f.hheaDescent = -float64(int16(binary.BigEndian.Uint16(b[6:])))
		f.hheaLineGap = float64(int16(binary.BigEndian.Uint16(b[8:])))
	}
	if b, err := ld.RawTable(ot.MustNewTag("OS/2")); err == nil && len(b) >= 78 {
		f.typoAscent = float64(int16(binary.BigEndian.Uint16(b[68:])))
		f.typoDescent = -float64(int16(binary.BigEndian.Uint16(b[70:])))
		f.typoLineGap = float64(int16(binary.BigEndian.Uint16(b[72:])))
		f.winAscent = float64(binary.BigEndian.Uint16(b[74:]))
		f.winDescent = float64(binary.BigEndian.Uint16(b[76:]))
		f.useTypoMetrics = binary.BigEndian.Uint16(b[62:])&(1<<7) != 0
		if version := binary.BigEndian.Uint16(b[0:]); version >= 2 && len(b) >= 90 {
			f.xHeight = float64(int16(binary.BigEndian.Uint16(b[86:])))
			f.capHeight = float64(int16(binary.BigEndian.Uint16(b[88:])))
		}
	}
	if b, err := ld.RawTable(ot.MustNewTag("post")); err == nil && len(b) >= 12 {
		f.underlinePosition = float64(int16(binary.BigEndian.Uint16(b[8:])))
		f.underlineThickness = float64(int16(binary.BigEndian.Uint16(b[10:])))
	}
	if f.platform == PlatformDarwin {
		switch strings.ToLower(f.Family) {
		case "times", "helvetica", "courier":
			f.ascentHack = true
		}
	}
}

// UnitsPerEm is the em size of the font in font units.
func (f *Face) UnitsPerEm() float64 { return f.upem }

// GoText returns the underlying go-text face, for glyph outlines. The face
// is shared by all goroutines: call only its methods that do not write
// (the glyph data and the advances). NominalGlyph and GlyphExtents of the
// go-text face write caches: use Face.NominalGlyph in place of the first.
func (f *Face) GoText() *gtfont.Face { return f.gt }

// HasGlyph reports whether the font maps the character to a glyph.
func (f *Face) HasGlyph(r rune) bool {
	_, ok := f.NominalGlyph(r)
	return ok
}

// Metrics are the vertical metrics of a face at one size, in px.
type Metrics struct {
	Ascent, Descent float64 // rounded to integers, as Blink does
	LineGap         float64
	// LineSpacing is the height of a line box for line-height: normal.
	LineSpacing float64
	XHeight     float64
	CapHeight   float64
	// FloatAscent and FloatDescent are the values before rounding.
	FloatAscent, FloatDescent float64
}

func f32(x float64) float64 { return float64(float32(x)) }

// roundScalar is SkScalarRoundToScalar: floor(x + 0.5) in float32.
func roundScalar(x float64) float64 {
	return float64(float32(math.Floor(float64(float32(x) + 0.5))))
}

// Metrics returns the vertical metrics that Blink computes for this face
// (FontMetrics::AscentDescentWithHacks and SimpleFontData::PlatformInit).
func (f *Face) Metrics(size float64) Metrics {
	if f.platform == PlatformLinux {
		return f.linuxMetrics(size)
	}
	size = f.colorEmojiMetricsSize(size)
	scale := size / f.upem
	asc := f32(float64(f.hheaAscent * scale))
	desc := f32(float64(f.hheaDescent * scale))
	gap := f32(float64(f.hheaLineGap * scale))
	m := Metrics{FloatAscent: asc, FloatDescent: desc, LineGap: gap}
	m.Ascent = roundScalar(asc)
	m.Descent = roundScalar(desc)
	if f.ascentHack {
		m.Ascent += float64(float32(math.Floor(float64(float32((m.Ascent+m.Descent)*0.15) + 0.5))))
	}
	m.LineSpacing = math.Round(m.Ascent) + math.Round(m.Descent) + math.Round(gap)
	m.XHeight = f.xHeightPx(size, asc)
	if f.capHeight > 0 {
		m.CapHeight = f32(float64(f.capHeight * scale))
	} else {
		m.CapHeight = m.Ascent
	}
	return m
}

// xHeightPx is the x-height that Blink uses. On macOS Blink measures the
// glyph of 'x': the distance from the baseline to the top of its outline.
// On other platforms it reads the OS/2 table.
func (f *Face) xHeightPx(size, ascent float64) float64 {
	scale := size / f.upem
	if f.platform == PlatformDarwin {
		if gid, ok := f.NominalGlyph('x'); ok {
			if maxY, ok := f.tightTop(gid); ok {
				return f32(float64(float64(maxY) * scale))
			}
		}
	}
	if f.xHeight > 0 {
		return f32(float64(f.xHeight * scale))
	}
	return f32(float64(ascent * 0.56))
}

// OutlineBounds returns the bounds of all outline points of a glyph in font
// units, y up. Control points are included, as in SkPath::getBounds. The
// last result is false for a glyph without an outline.
func (f *Face) OutlineBounds(gid uint32) (minX, minY, maxX, maxY float32, ok bool) {
	if v, hit := f.bounds.Load(gid); hit {
		b := v.([5]float32)
		return b[0], b[1], b[2], b[3], b[4] != 0
	}
	var res [5]float32
	if outline, has := f.gt.GlyphDataOutline(gtfont.GID(gid)); has && len(outline.Segments) > 0 {
		first := true
		add := func(x, y float32) {
			if first {
				res[0], res[1], res[2], res[3] = x, y, x, y
				first = false
				return
			}
			res[0], res[2] = min(res[0], x), max(res[2], x)
			res[1], res[3] = min(res[1], y), max(res[3], y)
		}
		var cur gtfont.SegmentPoint
		for _, seg := range outline.Segments {
			switch seg.Op {
			case ot.SegmentOpQuadTo:
				c, p := seg.Args[0], seg.Args[1]
				if TightGlyphBounds {
					quadExtrema(cur.X, c.X, p.X, cur.Y, c.Y, p.Y, add)
				} else {
					add(c.X, c.Y)
				}
				add(p.X, p.Y)
				cur = p
			case ot.SegmentOpCubeTo:
				c1, c2, p := seg.Args[0], seg.Args[1], seg.Args[2]
				add(c1.X, c1.Y)
				add(c2.X, c2.Y)
				add(p.X, p.Y)
				cur = p
			default:
				add(seg.Args[0].X, seg.Args[0].Y)
				cur = seg.Args[0]
			}
		}
		if !first {
			res[4] = 1
		}
	}
	f.bounds.Store(gid, res)
	return res[0], res[1], res[2], res[3], res[4] != 0
}

// tightTop returns the highest point of the curve of a glyph, in font
// units. Unlike OutlineBounds it does not count control points that are
// off the curve.
func (f *Face) tightTop(gid uint32) (float32, bool) {
	outline, has := f.gt.GlyphDataOutline(gtfont.GID(gid))
	if !has || len(outline.Segments) == 0 {
		return 0, false
	}
	var top float32
	first := true
	add := func(_, y float32) {
		if first || y > top {
			top, first = y, false
		}
	}
	var cur gtfont.SegmentPoint
	for _, seg := range outline.Segments {
		switch seg.Op {
		case ot.SegmentOpQuadTo:
			c, p := seg.Args[0], seg.Args[1]
			quadExtrema(cur.X, c.X, p.X, cur.Y, c.Y, p.Y, add)
			add(p.X, p.Y)
			cur = p
		case ot.SegmentOpCubeTo:
			for _, p := range seg.Args {
				add(p.X, p.Y)
			}
			cur = seg.Args[2]
		default:
			add(seg.Args[0].X, seg.Args[0].Y)
			cur = seg.Args[0]
		}
	}
	return top, !first
}

// TightGlyphBounds selects the bounds of the curve instead of the bounds of
// the control points for quadratic outlines.
var TightGlyphBounds = false

// quadExtrema reports the extreme points of a quadratic curve that are
// inside the curve.
func quadExtrema(x0, x1, x2, y0, y1, y2 float32, add func(x, y float32)) {
	eval := func(t float32) (float32, float32) {
		mt := 1 - t
		return float32(float32(mt*mt)*x0) + float32(float32(2*mt*t)*x1) + float32(float32(t*t)*x2), float32(float32(mt*mt)*y0) + float32(float32(2*mt*t)*y1) + float32(float32(t*t)*y2)
	}
	if d := x0 - float32(2*x1) + x2; d != 0 {
		if t := (x0 - x1) / d; t > 0 && t < 1 {
			add(eval(t))
		}
	}
	if d := y0 - float32(2*y1) + y2; d != 0 {
		if t := (y0 - y1) / d; t > 0 && t < 1 {
			add(eval(t))
		}
	}
}

// Glyph is one shaped glyph.
type Glyph struct {
	ID      uint32
	Cluster int     // index of the first rune of the cluster in the input
	Advance float64 // px
	XOffset float64 // px
	YOffset float64 // px, positive is up
}

// Shape shapes a run of text that uses this face only, left to right, with
// the default OpenType features (kerning and standard ligatures on).
// Advances are in px, quantized as in Blink: the nominal advance of a glyph
// is truncated to 16.16 fixed point from a float32, and kerning is scaled by
// HarfBuzz with rounding.
func (f *Face) Shape(text []rune, size float64) []Glyph {
	return f.shape(text, size, scriptOf(text))
}

// shape is Shape with the script that HarfBuzz gets for the run.
func (f *Face) shape(text []rune, size float64, script language.Script) []Glyph {
	if len(text) == 0 {
		return nil
	}
	if f.platform == PlatformLinux {
		text = f.linuxShapeText(text)
	}
	sh := f.shaper.Get().(*shaper)
	defer f.shaper.Put(sh)

	// Shape in font units. The scale to px is applied below with the exact
	// arithmetic of Blink, which HarfBuzz scaling alone cannot reproduce.
	sh.font.XScale = int32(f.upem)
	sh.font.YScale = int32(f.upem)
	sh.font.Ptem = float32(size) // enables size-specific tracking ('trak')
	sh.buf.Clear()
	sh.buf.AddRunes(text, 0, -1)
	sh.buf.Props.Direction = harfbuzz.LeftToRight
	sh.buf.Props.Script = script
	sh.buf.Props.Language = language.NewLanguage("en")
	sh.buf.Shape(sh.font, nil)

	hbScale := int64(float32(size) * 65536) // SkiaScalarToHarfBuzzPosition(size)
	upem := int64(f.upem)
	// scale16 is hb_font_t::em_mult: (v * mult + 32768) >> 16, with
	// mult = (scale << 16) / upem. The result is in 16.16 fixed point.
	mult := (hbScale << 16) / upem
	scale16 := func(units int32) int64 { return (int64(units)*mult + 32768) >> 16 }
	scalePos := func(units int32) float64 { return float64(scale16(units)) / 65536 }
	out := make([]Glyph, len(sh.buf.Info))
	nominals := make([]int32, len(sh.buf.Info))
	for i, info := range sh.buf.Info {
		nominals[i] = int32(f.gt.HorizontalAdvance(gtfont.GID(info.Glyph)))
	}
	// The kern machine of HarfBuzz halves the kerning of a pair after it
	// scaled the value. See kern_split.go.
	kernAdv, kernOff, machine := f.machineKerning(sh.buf.Info, sh.buf.Pos, nominals, scale16)
	for i, info := range sh.buf.Info {
		pos := sh.buf.Pos[i]
		nominal := nominals[i]
		extra := pos.XAdvance - nominal
		// Skia returns the advance as a float32 in px. Blink truncates it
		// to 16.16. Kerning is added in fixed point, so the sum is exact.
		adv16 := int64(float32(float64(nominal)*size/f.upem) * 65536)
		if f.platform == PlatformLinux {
			// See linux_scaler.go.
			adv16 = f.linuxAdvance16(nominal, size)
		}
		xOffset := scalePos(pos.XOffset)
		if machine {
			adv16 += kernAdv[i]
			xOffset = float64(kernOff[i]) / 65536
		} else if extra != 0 {
			adv16 += scale16(extra)
		}
		adv := float64(adv16) / 65536
		if f.platform == PlatformLinux && pos.XAdvance == 0 {
			// HarfBuzz gave the glyph no advance: a character that is not
			// drawn, such as a soft hyphen.
			adv = 0
		}
		if f.isAppleColorEmoji() && nominal != 0 {
			// CoreText scales the glyphs of this font. See emoji.go.
			if pos.XAdvance == 0 {
				// A glyph that HarfBuzz hides, such as a variation
				// selector.
				adv = 0
			} else {
				adv = f.colorEmojiAdvance(nominal, size)
				if extra != 0 {
					adv += float64(float64(scale16(extra)) / 65536)
				}
			}
		}
		out[i] = Glyph{
			ID:      uint32(info.Glyph),
			Cluster: info.Cluster,
			Advance: adv,
			XOffset: xOffset,
			YOffset: scalePos(pos.YOffset),
		}
	}
	return out
}

// Width returns the width of a shaped run as Blink accumulates it: a sum of
// float32 advances.
func Width(glyphs []Glyph) float64 {
	var w float32
	for i := range glyphs {
		w += float32(glyphs[i].Advance)
	}
	return float64(w)
}

func scriptOf(text []rune) language.Script {
	for _, r := range text {
		s := language.LookupScript(r)
		if s != language.Common && s != language.Inherited && s != language.Unknown {
			return s
		}
	}
	return language.Latin
}

// newGoTextFont parses a font with go-text. The parser of go-text panics on
// some damaged tables (a loca table that points outside the glyf table, for
// example). A font file is input from outside, so the panic becomes an
// error and the face is skipped.
func newGoTextFont(ld *ot.Loader) (ft *gtfont.Font, err error) {
	defer func() {
		if p := recover(); p != nil {
			ft, err = nil, fmt.Errorf("damaged font: %v", p)
		}
	}()
	return gtfont.NewFont(ld)
}
