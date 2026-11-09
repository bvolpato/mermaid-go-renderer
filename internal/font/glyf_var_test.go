// This file ports no upstream file. It compares glyf_var.go, and the scaler
// of autohint_glyf.go, with skrifa 0.46.2, read-fonts 0.43.2 and font-types
// 0.12.5: it replays the golden files that tools/reference/skrifa-oracle
// writes.

package font

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	ot "github.com/go-text/typesetting/font/opentype"

	"github.com/bvolpato/mermaid-go-renderer/internal/font/webfontdata"
)

// The tests of this file replay the golden files of
// tools/reference/skrifa-oracle against glyf_var.go. The oracle runs skrifa
// 0.46.2, read-fonts 0.43.2 and font-types 0.12.5, the crates of Skia in
// Chromium 156, with a copy of the pen of Skia. gen.sh in that directory
// writes the files. Every comparison is exact: integers, and the bits of
// f32 numbers.

// skrifaOracleFonts has the web fonts of the golden files:
// testdata/skrifa-<name>.json.gz.
//
// The font Recursive is variable: it has the axis wght. The interpreter of
// its hinting instance has the backward compatibility mode on. The prep
// program of Open Sans turns the mode off (INSTCTRL with the selector 3).
// TestSkrifaBackwardCompat shows both.
var skrifaOracleFonts = []struct {
	name, path               string
	variable, backwardCompat bool
}{
	{"recursive-latin", "files/recursive/files/recursive-latin-wght-normal.woff2", true, true},
	{"recursive-latin-ext", "files/recursive/files/recursive-latin-ext-wght-normal.woff2", true, true},
	{"recursive-vietnamese", "files/recursive/files/recursive-vietnamese-wght-normal.woff2", true, true},
	{"recursive-cyrillic-ext", "files/recursive/files/recursive-cyrillic-ext-wght-normal.woff2", true, true},
	{"open-sans-latin-400-normal", "files/open-sans/files/open-sans-latin-400-normal.woff2", false, false},
	{"open-sans-latin-400-italic", "files/open-sans/files/open-sans-latin-400-italic.woff2", false, false},
}

// skrifaGolden is a golden file of the oracle. The header of
// tools/reference/skrifa-oracle/src/main.rs describes the fields.
type skrifaGolden struct {
	Font          string    `json:"font"`
	GlyphCount    int       `json:"glyphCount"`
	Sizes         []string  `json:"sizes"`
	SizeBits      []uint32  `json:"sizeBits"`
	FullGlyphs    []uint32  `json:"fullGlyphs"`
	FullSizes     []int     `json:"fullSizes"`
	HasHinting    string    `json:"hasHinting"`
	Normalize     [][]int64 `json:"normalize"`
	EnabledByPpem *struct {
		From   int    `json:"from"`
		Values string `json:"values"`
	} `json:"enabledByPpem"`
	EnabledBySize [][2]uint32 `json:"enabledBySize"`
	LsbProbe      *struct {
		Delta  int16      `json:"delta"`
		Glyphs []uint32   `json:"glyphs"`
		Rows   [][6]int64 `json:"rows"`
	} `json:"lsbProbe"`
	Locations []skrifaLocation `json:"locations"`
}

type skrifaLocation struct {
	Wght            *float32          `json:"wght"`
	Coords          []int16           `json:"coords"`
	Mvar            map[string]*int32 `json:"mvar"`
	Metrics         [][]*uint32       `json:"metrics"`
	UnscaledAdvance []*int32          `json:"unscaledAdvance"`
	Instance        []bool            `json:"instance"`
	Enabled         []bool            `json:"enabled"`
	LinHash         string            `json:"linHash"`
	Unhinted        []string          `json:"unhinted"`
	Hinted          []string          `json:"hinted"`
	HintedDetail    [][][]int32       `json:"hintedDetail"`
	Paths           []skrifaPath      `json:"paths"`
}

type skrifaPath struct {
	G   uint32     `json:"g"`
	S   int        `json:"s"`
	Lin *uint32    `json:"lin"`
	U   skrifaDraw `json:"u"`
	H   skrifaDraw `json:"h"`
}

type skrifaDraw struct {
	Err bool    `json:"err"`
	Adv *uint32 `json:"adv"`
	Lsb *uint32 `json:"lsb"`
	V   string  `json:"v"`
	P   []int32 `json:"p"`
}

// name returns the name of a location for a message.
func (l *skrifaLocation) name() string {
	if l.Wght == nil {
		return "default"
	}
	return fmt.Sprintf("wght=%v", *l.Wght)
}

// skrifaFont has a web font and its golden file.
type skrifaFont struct {
	name     string
	variable bool
	// backwardCompat is HintInstance::backward_compatibility of the hinting
	// instance of the font.
	backwardCompat bool
	data           []byte
	ld             *ot.Loader
	font           *ahFont
	golden         skrifaGolden
}

// loadSkrifaFonts reads the web fonts of the oracle and their golden files.
func loadSkrifaFonts(t *testing.T) []*skrifaFont {
	t.Helper()
	var fonts []*skrifaFont
	for _, f := range skrifaOracleFonts {
		data, err := webfontdata.Font(f.path)
		if err != nil {
			t.Fatal(err)
		}
		loaders, err := ot.NewLoaders(bytes.NewReader(data))
		if err != nil || len(loaders) != 1 {
			t.Fatalf("%s: %v", f.name, err)
		}
		sf := &skrifaFont{name: f.name, variable: f.variable, backwardCompat: f.backwardCompat, data: data, ld: loaders[0], font: newAHFont(loaders[0])}
		if sf.font == nil {
			t.Fatalf("%s: no TrueType outlines", f.name)
		}
		readGzipJSON(t, "skrifa-"+f.name+".json.gz", &sf.golden)
		if sf.golden.GlyphCount != int(sf.font.glyphCount) {
			t.Fatalf("%s: %d glyphs, the golden file has %d", f.name, sf.font.glyphCount, sf.golden.GlyphCount)
		}
		fonts = append(fonts, sf)
	}
	return fonts
}

// skiaPath is the path of a glyph as the pen of Skia records it: the verbs
// of SkPathVerb and the points with y down.
type skiaPath struct {
	verbs  []byte
	points [][2]float32
}

// newSkiaPath converts the segments of ahPathPen. That pen keeps y up and
// has no verb for the end of a contour. The pen of Skia changes the sign of
// y and closes each contour.
func newSkiaPath(segments []ot.Segment) skiaPath {
	var p skiaPath
	for _, seg := range segments {
		n := 1
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			if len(p.verbs) != 0 {
				p.verbs = append(p.verbs, 5)
			}
			p.verbs = append(p.verbs, 0)
		case ot.SegmentOpLineTo:
			p.verbs = append(p.verbs, 1)
		case ot.SegmentOpQuadTo:
			p.verbs, n = append(p.verbs, 2), 2
		case ot.SegmentOpCubeTo:
			p.verbs, n = append(p.verbs, 4), 3
		}
		for _, a := range seg.Args[:n] {
			p.points = append(p.points, [2]float32{a.X, -a.Y})
		}
	}
	if len(p.verbs) != 0 {
		p.verbs = append(p.verbs, 5)
	}
	return p
}

// skrifaDrawRecord returns the bytes that the hash of a draw covers:
// draw_record of the oracle.
func skrifaDrawRecord(d ahDraw, ok bool) []byte {
	if !ok {
		return []byte{0xEE}
	}
	p := newSkiaPath(d.segments)
	r := []byte{0x01, 1}
	r = binary.BigEndian.AppendUint32(r, math.Float32bits(d.advance))
	r = append(r, 1)
	r = binary.BigEndian.AppendUint32(r, math.Float32bits(d.lsb))
	r = binary.BigEndian.AppendUint32(r, uint32(len(p.verbs)))
	r = append(r, p.verbs...)
	r = binary.BigEndian.AppendUint32(r, uint32(len(p.points)))
	for _, pt := range p.points {
		r = binary.BigEndian.AppendUint32(r, math.Float32bits(pt[0]))
		r = binary.BigEndian.AppendUint32(r, math.Float32bits(pt[1]))
	}
	return r
}

func skrifaFNV1a64(b []byte) uint64 {
	h := uint64(0xcbf29ce484222325)
	for _, c := range b {
		h ^= uint64(c)
		h *= 0x100000001b3
	}
	return h
}

// skrifaRow returns a row of a golden file for the hashes of the draws of a
// glyph: the hash of the row, and a tag of 16 bits for each draw.
func skrifaRow(hashes []uint64) string {
	var all []byte
	for _, h := range hashes {
		all = binary.BigEndian.AppendUint64(all, h)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%016x", skrifaFNV1a64(all))
	for _, h := range hashes {
		fmt.Fprintf(&b, "%04x", uint16(h^h>>16^h>>32^h>>48))
	}
	return b.String()
}

// skrifaDrawMode is one of the two draws of the oracle.
type skrifaDrawMode struct {
	name   string
	hinted bool
}

var (
	skrifaUnhinted = skrifaDrawMode{"unhinted", false}
	skrifaHinted   = skrifaDrawMode{"hinted", true}
)

// draw draws a glyph as the oracle does: DrawSettings::unhinted, or
// DrawSettings::hinted with the hinting instance of Skia. Without a
// hinting instance Skia draws without hinting.
func (m skrifaDrawMode) draw(f *ahFont, loc *skrifaLocation, gid uint32, si int, size float32, backwardCompat bool) (ahDraw, bool) {
	o, ok := f.outline(gid)
	if !ok {
		return ahDraw{}, false
	}
	hinted := m.hinted && loc.Instance[si]
	return f.drawAt(o, size, loc.Coords, hinted, loc.Enabled[si], backwardCompat)
}

// skrifaRowResult has the result of the comparison of the rows of a
// location.
type skrifaRowResult struct {
	cases, pass int
	// rows and rowPass count the 64 bit hashes of the rows.
	rows, rowPass int
	fails         []string
}

// compareRows draws every glyph at every size of a location, and compares
// the hashes with the rows of the golden file. only selects the glyphs. A
// nil only selects all glyphs.
func (sf *skrifaFont) compareRows(loc *skrifaLocation, mode skrifaDrawMode, backwardCompat bool, only func(gid uint32) bool) skrifaRowResult {
	var res skrifaRowResult
	rows := loc.Unhinted
	if mode.hinted {
		rows = loc.Hinted
	}
	for gid := uint32(0); gid < uint32(sf.golden.GlyphCount); gid++ {
		if only != nil && !only(gid) {
			continue
		}
		hashes := make([]uint64, len(sf.golden.SizeBits))
		for si, bits := range sf.golden.SizeBits {
			d, ok := mode.draw(sf.font, loc, gid, si, math.Float32frombits(bits), backwardCompat)
			hashes[si] = skrifaFNV1a64(skrifaDrawRecord(d, ok))
		}
		got, want := skrifaRow(hashes), rows[gid]
		res.rows++
		if got == want {
			res.rowPass++
		}
		for si := range hashes {
			res.cases++
			if got[16+4*si:20+4*si] == want[16+4*si:20+4*si] {
				res.pass++
			} else if len(res.fails) < 8 {
				res.fails = append(res.fails, fmt.Sprintf("glyph %d size %s", gid, sf.golden.Sizes[si]))
			}
		}
	}
	return res
}

// TestSkrifaNormalizedCoords compares the normalized coordinates of user
// values of the axis wght with AxisCollection::location of skrifa: the
// values of the locations of the golden files, and 217 values from 250 to
// 1050, which are on all segments of the avar map and outside of the axis.
func TestSkrifaNormalizedCoords(t *testing.T) {
	wght := ahTag("wght")
	for _, sf := range loadSkrifaFonts(t) {
		total, pass := 0, 0
		check := func(value float32, want []int16) {
			total++
			got := ahNormalizedCoords(sf.ld, map[uint32]float32{wght: value})
			if fmt.Sprint(got) == fmt.Sprint(want) {
				pass++
			} else if total-pass <= 5 {
				t.Errorf("%s: wght %v: coords %v, want %v", sf.name, value, got, want)
			}
		}
		for i := range sf.golden.Locations {
			loc := &sf.golden.Locations[i]
			if loc.Wght != nil {
				check(*loc.Wght, loc.Coords)
			}
		}
		for _, pair := range sf.golden.Normalize {
			want := make([]int16, len(pair)-1)
			for i, c := range pair[1:] {
				want[i] = int16(c)
			}
			check(math.Float32frombits(uint32(pair[0])), want)
		}
		if !sf.variable {
			// A font without fvar has no coordinates.
			if got := ahNormalizedCoords(sf.ld, map[uint32]float32{wght: 700}); got != nil {
				t.Errorf("%s: coords %v for a font without fvar", sf.name, got)
			}
			continue
		}
		// An axis without a value has its default, and a tag that is not
		// an axis is not used.
		if got := ahNormalizedCoords(sf.ld, map[uint32]float32{ahTag("wdth"): 80}); len(got) != 1 || got[0] != 0 {
			t.Errorf("%s: coords %v without a value for wght", sf.name, got)
		}
		if got := ahEffectiveCoords(ahNormalizedCoords(sf.ld, nil)); got != nil {
			t.Errorf("%s: effective coords %v for the default instance", sf.name, got)
		}
		if pass != total {
			t.Errorf("%s: %d of %d normalized coordinates match", sf.name, pass, total)
		}
		t.Logf("%s: %d of %d normalized coordinates match", sf.name, pass, total)
	}
}

// skrifaFixedToF64 is Fixed::to_f64: the whole part and the fraction. The
// conversion keeps the quotient apart from the sum.
func skrifaFixedToF64(bits int32) float64 {
	return float64(bits>>16) + float64(float64(bits&0xFFFF)/65536.0)
}

// skrifaMetrics is Metrics::new of skrifa for the metrics that the golden
// files have: ascent, descent, leading, cap height, x height, the offset
// and the thickness of the underline, and the offset and the thickness of
// the strikeout. A value is nil when skrifa has none. The deltas of MVAR
// come from ahFont.mvarDelta.
func skrifaMetrics(ld *ot.Loader, f *ahFont, size float32, coords []int16) [9]*uint32 {
	// Size::linear_scale.
	scale := float32(1.0)
	if f.unitsPerEm != 0 {
		scale = size / float32(f.unitsPerEm)
	}
	var ascent, descent, leading float32
	var capHeight, xHeight, underlineOffset, underlineThickness, strikeoutOffset, strikeoutThickness *float32
	scaled := func(v int32) *float32 {
		s := float32(v) * scale
		return &s
	}
	if post, ok := ahRawTable(ld, "post"); ok && len(post) >= 32 {
		underlineOffset = scaled(int32(post.i16(8)))
		underlineThickness = scaled(int32(post.i16(10)))
	}
	os2, hasOS2 := ahRawTable(ld, "OS/2")
	hasOS2 = hasOS2 && len(os2) >= 78
	usedTypoMetrics := false
	if hasOS2 {
		// Bit 7 of fsSelection is USE_TYPO_METRICS.
		if os2.u16(62)&0x80 != 0 {
			ascent = float32(os2.i16(68)) * scale
			descent = float32(os2.i16(70)) * scale
			leading = float32(os2.i16(72)) * scale
			usedTypoMetrics = true
		}
		// sxHeight and sCapHeight are in version 2 and later.
		if os2.u16(0) >= 2 && len(os2) >= 90 {
			capHeight = scaled(int32(os2.i16(88)))
			xHeight = scaled(int32(os2.i16(86)))
		}
		strikeoutOffset = scaled(int32(os2.i16(28)))
		strikeoutThickness = scaled(int32(os2.i16(26)))
	}
	if !usedTypoMetrics {
		if hhea, ok := ahRawTable(ld, "hhea"); ok && len(hhea) >= 36 {
			ascent = float32(hhea.i16(4)) * scale
			descent = float32(hhea.i16(6)) * scale
			leading = float32(hhea.i16(8)) * scale
		}
		if ascent == 0 && descent == 0 && hasOS2 {
			if os2.i16(68) != 0 || os2.i16(70) != 0 {
				ascent = float32(os2.i16(68)) * scale
				descent = float32(os2.i16(70)) * scale
				leading = float32(os2.i16(72)) * scale
			} else {
				ascent = float32(os2.u16(74)) * scale
				descent = -(float32(os2.u16(76)) * scale)
			}
		}
	}
	// The delta of a metric: delta.to_f64() as f32 * scale. The conversion
	// to float32 keeps the product apart from the sum.
	metricDelta := func(tag string) float32 {
		return float32(float32(skrifaFixedToF64(f.mvarDelta(ahTag(tag), coords))) * scale)
	}
	add := func(v *float32, tag string) {
		if v != nil {
			*v += metricDelta(tag)
		}
	}
	add(&ascent, "hasc")
	add(&descent, "hdsc")
	add(&leading, "hlgp")
	add(capHeight, "cpht")
	add(xHeight, "xhgt")
	add(underlineOffset, "undo")
	add(underlineThickness, "unds")
	add(strikeoutOffset, "stro")
	add(strikeoutThickness, "strs")
	var out [9]*uint32
	for i, v := range []*float32{&ascent, &descent, &leading, capHeight, xHeight, underlineOffset, underlineThickness, strikeoutOffset, strikeoutThickness} {
		if v != nil {
			bits := math.Float32bits(*v)
			out[i] = &bits
		}
	}
	return out
}

// TestSkrifaMetrics compares the deltas of MVAR with Mvar::metric_delta of
// read-fonts for eleven tags at each location, and the metrics of the font
// with Metrics::new of skrifa at each location and font size, by the bits
// of the f32 numbers.
func TestSkrifaMetrics(t *testing.T) {
	for _, sf := range loadSkrifaFonts(t) {
		deltas, deltaPass, metrics, metricPass := 0, 0, 0, 0
		for i := range sf.golden.Locations {
			loc := &sf.golden.Locations[i]
			for tag, want := range loc.Mvar {
				deltas++
				var got *int32
				if v := sf.font.vari; v != nil && v.mvar != nil {
					if delta, ok := v.mvar.metricDelta(ahTag(tag), loc.Coords); ok {
						got = &delta
					}
				}
				if (got == nil) == (want == nil) && (got == nil || *got == *want) {
					deltaPass++
				} else {
					t.Errorf("%s %s: MVAR %s: delta %v, want %v", sf.name, loc.name(), tag, deref(got), deref(want))
				}
			}
			for si, bits := range sf.golden.SizeBits {
				got := skrifaMetrics(sf.ld, sf.font, math.Float32frombits(bits), loc.Coords)
				for k, want := range loc.Metrics[si] {
					metrics++
					if (got[k] == nil) == (want == nil) && (want == nil || *got[k] == *want) {
						metricPass++
					} else {
						t.Errorf("%s %s size %s: metric %d: bits %v, want %v", sf.name, loc.name(), sf.golden.Sizes[si], k, deref(got[k]), deref(want))
					}
				}
			}
		}
		t.Logf("%s: %d of %d MVAR deltas and %d of %d metrics match", sf.name, deltaPass, deltas, metricPass, metrics)
	}
}

// deref returns the value of a pointer for a message.
func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// TestSkrifaAdvanceWidth compares the advance of every glyph in font units
// at each location with GlyphMetrics::advance_width of skrifa for a font
// without a size, and the advance in px at each font size with the hash of
// the f32 bits of GlyphMetrics::advance_width.
func TestSkrifaAdvanceWidth(t *testing.T) {
	for _, sf := range loadSkrifaFonts(t) {
		units, unitPass, hashes, hashPass, linear := 0, 0, 0, 0, 0
		for i := range sf.golden.Locations {
			loc := &sf.golden.Locations[i]
			var lin []byte
			for gid := uint32(0); gid < uint32(sf.golden.GlyphCount); gid++ {
				units++
				if want := loc.UnscaledAdvance[gid]; want != nil && sf.font.advanceWidthAt(gid, loc.Coords) == *want {
					unitPass++
				} else {
					t.Errorf("%s %s glyph %d: advance %d, want %v", sf.name, loc.name(), gid, sf.font.advanceWidthAt(gid, loc.Coords), deref(want))
				}
				for _, bits := range sf.golden.SizeBits {
					linear++
					lin = append(lin, 1)
					lin = binary.BigEndian.AppendUint32(lin, math.Float32bits(sf.font.linearAdvance(gid, math.Float32frombits(bits), loc.Coords)))
				}
			}
			hashes++
			if got := fmt.Sprintf("%016x", skrifaFNV1a64(lin)); got == loc.LinHash {
				hashPass++
			} else {
				t.Errorf("%s %s: hash of the linear advances %s, want %s", sf.name, loc.name(), got, loc.LinHash)
			}
		}
		// A glyph that the font does not have has the advance 0.
		if got := sf.font.advanceWidthAt(uint32(sf.golden.GlyphCount), sf.golden.Locations[0].Coords); got != 0 {
			t.Errorf("%s: advance %d for a glyph after the last glyph", sf.name, got)
		}
		t.Logf("%s: %d of %d advances in font units match, %d of %d hashes of %d linear advances match", sf.name, unitPass, units, hashPass, hashes, linear)
	}
}

// TestSkrifaHasInstructions compares hasInstructions with
// OutlineGlyph::has_hinting of skrifa for every glyph.
func TestSkrifaHasInstructions(t *testing.T) {
	for _, sf := range loadSkrifaFonts(t) {
		with, pass := 0, 0
		for gid := uint32(0); gid < uint32(sf.golden.GlyphCount); gid++ {
			o, ok := sf.font.outline(gid)
			want := sf.golden.HasHinting[gid]
			if ok && (want == '1') == sf.font.hasInstructions(o) && want != '2' {
				pass++
			} else {
				t.Errorf("%s glyph %d: hasInstructions %v, want %c", sf.name, gid, sf.font.hasInstructions(o), want)
			}
			if want == '1' {
				with++
			}
		}
		t.Logf("%s: %d of %d glyphs match, %d glyphs have instructions", sf.name, pass, sf.golden.GlyphCount, with)
	}
}

// TestSkrifaUnhinted compares the draw without hinting of every glyph, at
// every location and font size, with OutlineGlyph::draw of skrifa for
// DrawSettings::unhinted: the advance and the left side bearing of
// AdjustedMetrics, and the verbs and the points that the pen of Skia
// records.
func TestSkrifaUnhinted(t *testing.T) {
	for _, sf := range loadSkrifaFonts(t) {
		var total skrifaRowResult
		for i := range sf.golden.Locations {
			loc := &sf.golden.Locations[i]
			res := sf.compareRows(loc, skrifaUnhinted, true, nil)
			if res.pass != res.cases || res.rowPass != res.rows {
				t.Errorf("%s %s: %d of %d draws without hinting match (%d of %d rows): %s", sf.name, loc.name(), res.pass, res.cases, res.rowPass, res.rows, strings.Join(res.fails, ", "))
			}
			total.cases, total.pass = total.cases+res.cases, total.pass+res.pass
			total.rows, total.rowPass = total.rows+res.rows, total.rowPass+res.rowPass
		}
		t.Logf("%s: %d of %d draws without hinting match, %d of %d rows", sf.name, total.pass, total.cases, total.rowPass, total.rows)
	}
}

// TestSkrifaHinted compares the hinted draw of every glyph without
// instructions, at every location and font size, with OutlineGlyph::draw of
// skrifa for DrawSettings::hinted and the hinting instance of Skia for the
// hinting mode "normal". The hinted mode of the scaler is exact for these
// glyphs: all glyphs of Recursive, and the few glyphs of Open Sans without
// instructions.
//
// The golden file says for each font size whether the hinting instance is
// enabled. The prep program of the font decides that, and the interpreter
// is not ported. The prep program decides also whether the backward
// compatibility mode is on: see TestSkrifaBackwardCompat.
func TestSkrifaHinted(t *testing.T) {
	for _, sf := range loadSkrifaFonts(t) {
		noInstructions := func(gid uint32) bool { return sf.golden.HasHinting[gid] == '0' }
		var total skrifaRowResult
		for i := range sf.golden.Locations {
			loc := &sf.golden.Locations[i]
			res := sf.compareRows(loc, skrifaHinted, sf.backwardCompat, noInstructions)
			if res.pass != res.cases || res.rowPass != res.rows {
				t.Errorf("%s %s: %d of %d hinted draws match (%d of %d rows): %s", sf.name, loc.name(), res.pass, res.cases, res.rowPass, res.rows, strings.Join(res.fails, ", "))
			}
			total.cases, total.pass = total.cases+res.cases, total.pass+res.pass
			total.rows, total.rowPass = total.rows+res.rows, total.rowPass+res.rowPass
		}
		t.Logf("%s: %d of %d hinted draws of the glyphs without instructions match, %d of %d rows", sf.name, total.pass, total.cases, total.rowPass, total.rows)
	}
}

// skrifaDrawText returns a draw as text for a message: the advance and the
// left side bearing as f32 bits, the verbs, and the points in 26.6 units.
func skrifaDrawText(adv, lsb uint32, verbs string, points []int32) string {
	return fmt.Sprintf("adv=%d lsb=%d verbs=%s points=%v", adv, lsb, verbs, points)
}

// TestSkrifaFullPaths compares the draws that the golden files have in
// full: about 20 glyphs of each font at two font sizes, and for Recursive
// at three values of wght. A difference shows the verbs and the points. The
// hinted draw is compared for the glyphs without instructions.
func TestSkrifaFullPaths(t *testing.T) {
	for _, sf := range loadSkrifaFonts(t) {
		total, pass := 0, 0
		for i := range sf.golden.Locations {
			loc := &sf.golden.Locations[i]
			for _, p := range loc.Paths {
				size := math.Float32frombits(sf.golden.SizeBits[p.S])
				if p.Lin != nil {
					total++
					if got := math.Float32bits(sf.font.linearAdvance(p.G, size, loc.Coords)); got == *p.Lin {
						pass++
					} else {
						t.Errorf("%s %s glyph %d size %s: linear advance bits %d, want %d", sf.name, loc.name(), p.G, sf.golden.Sizes[p.S], got, *p.Lin)
					}
				}
				for _, m := range []struct {
					mode skrifaDrawMode
					want skrifaDraw
				}{{skrifaUnhinted, p.U}, {skrifaHinted, p.H}} {
					if m.mode.hinted && sf.golden.HasHinting[p.G] != '0' {
						continue
					}
					total++
					d, ok := m.mode.draw(sf.font, loc, p.G, p.S, size, sf.backwardCompat)
					// pathAt gives the path of the same draw.
					if o, outlineOK := sf.font.outline(p.G); outlineOK {
						hinted := m.mode.hinted && loc.Instance[p.S] && loc.Enabled[p.S]
						if segments, pathOK := sf.font.pathAt(o, size, loc.Coords, hinted, sf.backwardCompat); pathOK != ok || fmt.Sprint(segments) != fmt.Sprint(d.segments) {
							t.Errorf("%s %s glyph %d size %s %s: pathAt differs from drawAt", sf.name, loc.name(), p.G, sf.golden.Sizes[p.S], m.mode.name)
						}
					}
					got, want := "error", "error"
					if ok {
						sp := newSkiaPath(d.segments)
						var verbs strings.Builder
						for _, v := range sp.verbs {
							verbs.WriteByte('0' + v)
						}
						var points []int32
						for _, pt := range sp.points {
							points = append(points, int32(pt[0]*64), int32(pt[1]*64))
						}
						got = skrifaDrawText(math.Float32bits(d.advance), math.Float32bits(d.lsb), verbs.String(), points)
					}
					if !m.want.Err {
						want = skrifaDrawText(*m.want.Adv, *m.want.Lsb, m.want.V, m.want.P)
					}
					if got == want {
						pass++
					} else if total-pass <= 6 {
						t.Errorf("%s %s glyph %d size %s %s:\n got %s\nwant %s", sf.name, loc.name(), p.G, sf.golden.Sizes[p.S], m.mode.name, got, want)
					}
				}
			}
		}
		if pass != total {
			t.Errorf("%s: %d of %d full records match", sf.name, pass, total)
		}
		t.Logf("%s: %d of %d full records match", sf.name, pass, total)
	}
}

// skrifaBounds returns the bounds of the points of a path in 26.6 units,
// and the number of points.
func skrifaBounds(p skiaPath) (n int, xMin, yMin, xMax, yMax int32) {
	for i, pt := range p.points {
		x, y := int32(pt[0]*64), int32(pt[1]*64)
		if i == 0 {
			xMin, yMin, xMax, yMax = x, y, x, y
			continue
		}
		xMin, xMax = min(xMin, x), max(xMax, x)
		yMin, yMax = min(yMin, y), max(yMax, y)
	}
	return len(p.points), xMin, yMin, xMax, yMax
}

// TestSkrifaHintedWithInstructions measures how far the hinted mode of the
// scaler is from skrifa for the glyphs with instructions, which are most
// glyphs of Open Sans. The interpreter of TrueType instructions is not
// ported, so the points of these glyphs are the points before the program
// of the glyph.
//
// The test compares, for every glyph with instructions and every font size:
// the advance of the hinted draw, the left side bearing, and the bounds of
// the points of the path rounded out to whole pixels, for x and for y. The
// advance and the bounds in x must match: the programs of these fonts move
// the points in y only, and they do not move the phantom points after the
// scaler rounds them. The bounds in y are a measurement: the test logs how
// many match.
func TestSkrifaHintedWithInstructions(t *testing.T) {
	for _, sf := range loadSkrifaFonts(t) {
		loc := &sf.golden.Locations[0]
		if loc.HintedDetail == nil {
			continue
		}
		type count struct{ cases, advance, lsb, withPoints, pointCount, xBounds, yBounds, xExact, yExact, hash int }
		var all count
		bySize := make([]count, len(sf.golden.SizeBits))
		var advanceDiffs, xDiffs []string
		yDelta := map[int32]int{}
		for gid := uint32(0); gid < uint32(sf.golden.GlyphCount); gid++ {
			if sf.golden.HasHinting[gid] != '1' {
				continue
			}
			hashes := make([]uint64, len(sf.golden.SizeBits))
			for si, bits := range sf.golden.SizeBits {
				want := loc.HintedDetail[gid][si]
				d, ok := skrifaHinted.draw(sf.font, loc, gid, si, math.Float32frombits(bits), sf.backwardCompat)
				hashes[si] = skrifaFNV1a64(skrifaDrawRecord(d, ok))
				c := count{cases: 1}
				if !ok || want == nil {
					t.Errorf("%s glyph %d size %s: draw %v, the golden file has %v", sf.name, gid, sf.golden.Sizes[si], ok, want)
					continue
				}
				if int32(d.advance*64) == want[0] {
					c.advance = 1
				} else {
					advanceDiffs = append(advanceDiffs, fmt.Sprintf("glyph %d size %s: %v, want %v", gid, sf.golden.Sizes[si], d.advance, float32(want[0])/64))
				}
				if int32(d.lsb*64) == want[1] {
					c.lsb = 1
				}
				n, xMin, yMin, xMax, yMax := skrifaBounds(newSkiaPath(d.segments))
				if want[2] != 0 {
					c.withPoints = 1
					if n == int(want[2]) {
						c.pointCount = 1
					}
					// The bounds rounded out to whole pixels.
					out := func(lo, hi int32) [2]int32 { return [2]int32{lo >> 6, (hi + 63) >> 6} }
					if out(xMin, xMax) == out(want[3], want[5]) {
						c.xBounds = 1
					} else {
						xDiffs = append(xDiffs, fmt.Sprintf("glyph %d size %s: x %d..%d, want %d..%d", gid, sf.golden.Sizes[si], xMin, xMax, want[3], want[5]))
					}
					if out(yMin, yMax) == out(want[4], want[6]) {
						c.yBounds = 1
					}
					if xMin == want[3] && xMax == want[5] {
						c.xExact = 1
					}
					if yMin == want[4] && yMax == want[6] {
						c.yExact = 1
					}
					// The largest distance of an edge in y, in whole
					// pixels after the rounding.
					dy := max(ahAbs(out(yMin, yMax)[0]-out(want[4], want[6])[0]), ahAbs(out(yMin, yMax)[1]-out(want[4], want[6])[1]))
					yDelta[dy]++
				} else if n != 0 {
					t.Errorf("%s glyph %d size %s: %d points, the golden file has none", sf.name, gid, sf.golden.Sizes[si], n)
				}
				for _, sum := range []*count{&all, &bySize[si]} {
					sum.cases += c.cases
					sum.advance += c.advance
					sum.lsb += c.lsb
					sum.withPoints += c.withPoints
					sum.pointCount += c.pointCount
					sum.xBounds += c.xBounds
					sum.yBounds += c.yBounds
					sum.xExact += c.xExact
					sum.yExact += c.yExact
				}
			}
			// The draws that are equal to the draws of skrifa in every
			// bit.
			row := skrifaRow(hashes)
			for si := range hashes {
				if row[16+4*si:20+4*si] == loc.Hinted[gid][16+4*si:20+4*si] {
					all.hash++
					bySize[si].hash++
				}
			}
		}
		t.Logf("%s: glyphs with instructions, %d draws: advance %d, left side bearing %d match", sf.name, all.cases, all.advance, all.lsb)
		t.Logf("%s: %d draws with a path: number of points %d, bounds in whole px: x %d, y %d; bounds in 26.6: x %d, y %d; all bits of the draw: %d of %d",
			sf.name, all.withPoints, all.pointCount, all.xBounds, all.yBounds, all.xExact, all.yExact, all.hash, all.cases)
		for si, c := range bySize {
			t.Logf("%s size %s (hinting instance enabled: %v): %d draws: advance %d; %d with a path: x %d, y %d in whole px; x %d, y %d in 26.6; all bits %d",
				sf.name, sf.golden.Sizes[si], loc.Enabled[si], c.cases, c.advance, c.withPoints, c.xBounds, c.yBounds, c.xExact, c.yExact, c.hash)
		}
		var deltas []string
		keys := make([]int, 0, len(yDelta))
		for k := range yDelta {
			keys = append(keys, int(k))
		}
		sort.Ints(keys)
		for _, k := range keys {
			deltas = append(deltas, fmt.Sprintf("%d px: %d", k, yDelta[int32(k)]))
		}
		t.Logf("%s: largest distance of an edge of the bounds in y: %s", sf.name, strings.Join(deltas, ", "))
		if len(advanceDiffs) != 0 {
			t.Errorf("%s: %d hinted advances differ: %s", sf.name, len(advanceDiffs), strings.Join(advanceDiffs[:min(len(advanceDiffs), 20)], "; "))
		}
		if len(xDiffs) != 0 {
			t.Errorf("%s: %d bounds in x differ: %s", sf.name, len(xDiffs), strings.Join(xDiffs[:min(len(xDiffs), 20)], "; "))
		}
	}
}

// skrifaVariants is a golden file of the variants of a font: fonts that the
// oracle makes from a web font, with other tables or with changed bytes
// (tools/reference/skrifa-oracle/src/variants.rs).
type skrifaVariants struct {
	Base string    `json:"base"`
	Wght []float32 `json:"wght"`
	// NormWght has the values of wght of Norm of each variant.
	NormWght []float32 `json:"normWght"`
	Sizes    []string  `json:"sizes"`
	SizeBits []uint32  `json:"sizeBits"`
	// Tables has the tables that the oracle made, as hex digits.
	Tables   []string        `json:"tables"`
	Variants []skrifaVariant `json:"variants"`
}

type skrifaVariant struct {
	Name           string `json:"name"`
	BackwardCompat bool   `json:"backwardCompat"`
	// Set has the tables that replace a table of the base font: the tag,
	// and the index in Tables.
	Set map[string]int `json:"set"`
	// Edits has the bytes that change: the tag of the table, the position
	// and the value.
	Edits          [][]any `json:"edits"`
	FontHash       string  `json:"fontHash"`
	Panic          bool    `json:"panic"`
	Font           *bool   `json:"font"`
	GlyphCount     int     `json:"glyphCount"`
	HasHintingHash string  `json:"hasHintingHash"`
	// Norm has the normalized coordinates of the values of NormWght.
	Norm      [][]int16 `json:"norm"`
	Locations []struct {
		Coords       []int16  `json:"coords"`
		Mvar         []*int32 `json:"mvar"`
		AdvHash      string   `json:"advHash"`
		Instance     []bool   `json:"instance"`
		Enabled      []bool   `json:"enabled"`
		Unhinted     string   `json:"unhinted"`
		Hinted       string   `json:"hinted"`
		ChangedCount int      `json:"changedCount"`
		// Changed has the first draws that differ from the base font: the
		// glyph, the index of the size, the mode and the tag of the draw.
		Changed [][4]int `json:"changed"`
	} `json:"locations"`
}

// sfntTables returns the tables of an sfnt file by tag, and the version of
// the file.
func sfntTables(data []byte) (version []byte, tables map[string][]byte) {
	tables = map[string][]byte{}
	for i := 0; i < int(binary.BigEndian.Uint16(data[4:])); i++ {
		record := data[12+16*i:]
		offset, length := binary.BigEndian.Uint32(record[8:]), binary.BigEndian.Uint32(record[12:])
		tables[string(record[:4])] = data[offset : offset+length]
	}
	return data[:4], tables
}

// buildSfnt writes an sfnt file as the oracle does: the records of the
// tables in the order of their tags with the checksum 0, and the tables in
// the same order, each padded to a multiple of four bytes.
func buildSfnt(version []byte, tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for tag := range tables {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	n := len(tags)
	entrySelector := 0
	for 1<<(entrySelector+1) <= n {
		entrySelector++
	}
	searchRange := 16 << entrySelector
	out := append([]byte{}, version...)
	out = binary.BigEndian.AppendUint16(out, uint16(n))
	out = binary.BigEndian.AppendUint16(out, uint16(searchRange))
	out = binary.BigEndian.AppendUint16(out, uint16(entrySelector))
	out = binary.BigEndian.AppendUint16(out, uint16(16*n-searchRange))
	offset := 12 + 16*n
	for _, tag := range tags {
		out = append(out, tag...)
		out = binary.BigEndian.AppendUint32(out, 0)
		out = binary.BigEndian.AppendUint32(out, uint32(offset))
		out = binary.BigEndian.AppendUint32(out, uint32(len(tables[tag])))
		offset += (len(tables[tag]) + 3) &^ 3
	}
	for _, tag := range tags {
		out = append(out, tables[tag]...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	return out
}

// skrifaMvarTags has the tags of the MVAR deltas of the golden files, in
// the order of the oracle.
var skrifaMvarTags = []string{"hasc", "hdsc", "hlgp", "cpht", "xhgt", "undo", "unds", "stro", "strs", "hcrs", "sbxo"}

// TestSkrifaVariants compares the port with skrifa for variants of two of
// the web fonts. The oracle makes each variant from the web font, and the
// test makes the same font from the recipe in the golden file.
//
// The variants have what the web fonts do not have: an HVAR table with and
// without a map of delta sets, an hdmx table, a prep program that turns the
// backward compatibility mode of the interpreter off, bit 3 of head.flags,
// and gvar tables with random tuples: peaks in the header, intermediate
// regions, deltas for some points, point numbers of the glyph, deltas of 32
// bits, and deltas for the phantom points. The other variants have one to
// four bytes of a table changed, so that the data is often not valid: the
// port must return what skrifa returns, and it must not panic.
//
// For each variant the test compares the normalized coordinates of 33
// values of wght. For two values of wght it compares the deltas of MVAR for
// eleven tags, and the advance of every glyph in font units. For the first
// of the two it compares also the draw without hinting of every glyph at
// two font sizes, and the hinted draw of every glyph without instructions.
func TestSkrifaVariants(t *testing.T) {
	for _, sf := range loadSkrifaFonts(t) {
		if sf.name != "recursive-latin" && sf.name != "recursive-vietnamese" {
			continue
		}
		t.Run(sf.name, func(t *testing.T) {
			t.Parallel()
			testSkrifaVariants(t, sf)
		})
	}
}

func testSkrifaVariants(t *testing.T, sf *skrifaFont) {
	wght := ahTag("wght")
	{
		var golden skrifaVariants
		readGzipJSON(t, "skrifa-variants-"+sf.name+".json.gz", &golden)
		var base []byte
		for _, f := range skrifaOracleFonts {
			if f.name == sf.name {
				base, _ = webfontdata.Font(f.path)
			}
		}
		version, baseTables := sfntTables(base)
		made := make([][]byte, len(golden.Tables))
		for i, table := range golden.Tables {
			if _, err := fmt.Sscanf(table, "%x", &made[i]); err != nil {
				t.Fatal(err)
			}
		}
		type counts struct{ total, pass int }
		var variants, panics, coords, norm, mvar, advance, unhinted, hinted, hasHinting counts
		changed := 0
		// baseDraws has the hashes of the draws of the base font, by
		// location and mode.
		var baseDraws [][2][]uint64
		for vi := range golden.Variants {
			v := &golden.Variants[vi]
			// The font of the variant.
			tables := map[string][]byte{}
			for tag, data := range baseTables {
				tables[tag] = data
			}
			for tag, id := range v.Set {
				tables[tag] = made[id]
			}
			for _, edit := range v.Edits {
				tag := edit[0].(string)
				table := append([]byte{}, tables[tag]...)
				table[int(edit[1].(float64))] = byte(edit[2].(float64))
				tables[tag] = table
			}
			data := buildSfnt(version, tables)
			if got := fmt.Sprintf("%016x", skrifaFNV1a64(data)); got != v.FontHash {
				t.Fatalf("%s %s: the font has the hash %s, the oracle had %s", sf.name, v.Name, got, v.FontHash)
			}
			if v.Panic || v.Font != nil {
				// skrifa has no result for the font.
				panics.total++
				continue
			}
			variants.total++
			loaders, err := ot.NewLoaders(bytes.NewReader(data))
			if err != nil || len(loaders) != 1 {
				t.Errorf("%s %s: %v", sf.name, v.Name, err)
				continue
			}
			ld := loaders[0]
			f := newAHFont(ld)
			if f == nil {
				t.Errorf("%s %s: no font, skrifa has one", sf.name, v.Name)
				continue
			}
			ok := true
			fail := func(format string, args ...any) {
				ok = false
				t.Errorf("%s %s: %s", sf.name, v.Name, fmt.Sprintf(format, args...))
			}
			if int(f.glyphCount) != v.GlyphCount {
				fail("%d glyphs, want %d", f.glyphCount, v.GlyphCount)
				continue
			}
			// OutlineGlyph::has_hinting of each glyph.
			outlines := make([]ahGlyfOutline, v.GlyphCount)
			outlineOK := make([]bool, v.GlyphCount)
			has := make([]byte, v.GlyphCount)
			for gid := range outlines {
				outlines[gid], outlineOK[gid] = f.outline(uint32(gid))
				switch {
				case !outlineOK[gid]:
					has[gid] = '2'
				case f.hasInstructions(outlines[gid]):
					has[gid] = '1'
				default:
					has[gid] = '0'
				}
			}
			hasHinting.total++
			if fmt.Sprintf("%016x", skrifaFNV1a64(has)) == v.HasHintingHash {
				hasHinting.pass++
			} else {
				fail("hasInstructions differs")
			}
			for k, want := range v.Norm {
				norm.total++
				if got := ahNormalizedCoords(ld, map[uint32]float32{wght: golden.NormWght[k]}); fmt.Sprint(got) == fmt.Sprint(want) {
					norm.pass++
				} else {
					fail("wght %v: coords %v, want %v", golden.NormWght[k], got, want)
				}
			}
			for li := range v.Locations {
				loc := &v.Locations[li]
				got := ahNormalizedCoords(ld, map[uint32]float32{wght: golden.Wght[li]})
				coords.total++
				if fmt.Sprint(got) == fmt.Sprint(loc.Coords) {
					coords.pass++
				} else {
					fail("wght %v: coords %v, want %v", golden.Wght[li], got, loc.Coords)
				}
				// The other comparisons use the coordinates of skrifa.
				for k, tag := range skrifaMvarTags {
					mvar.total++
					var delta *int32
					if f.vari != nil && f.vari.mvar != nil {
						if d, ok := f.vari.mvar.metricDelta(ahTag(tag), loc.Coords); ok {
							delta = &d
						}
					}
					if want := loc.Mvar[k]; (delta == nil) == (want == nil) && (delta == nil || *delta == *want) {
						mvar.pass++
					} else {
						fail("wght %v: MVAR %s: delta %v, want %v", golden.Wght[li], tag, deref(delta), deref(want))
					}
				}
				// GlyphMetrics::advance_width without a size, as f32 bits.
				var adv []byte
				for gid := 0; gid < v.GlyphCount; gid++ {
					const scaleFactor = 1.0 / 65536.0
					units := float32(ahFixedMulDiv(0x10000*64, f.advanceWidthAt(uint32(gid), loc.Coords), 64)) * scaleFactor
					adv = append(adv, 1)
					adv = binary.BigEndian.AppendUint32(adv, math.Float32bits(units))
				}
				advance.total++
				if fmt.Sprintf("%016x", skrifaFNV1a64(adv)) == loc.AdvHash {
					advance.pass++
				} else {
					fail("wght %v: the advances differ", golden.Wght[li])
				}
				if loc.Unhinted == "" {
					continue
				}
				// The draws of every glyph at every size.
				var draws [2][]uint64
				for gid := 0; gid < v.GlyphCount; gid++ {
					for si, bits := range golden.SizeBits {
						size := math.Float32frombits(bits)
						d, drawOK := ahDraw{}, false
						if outlineOK[gid] {
							d, drawOK = f.drawAt(outlines[gid], size, loc.Coords, false, false, false)
						}
						draws[0] = append(draws[0], skrifaFNV1a64(skrifaDrawRecord(d, drawOK)))
						// The hash has a marker for the hinted draw of a
						// glyph with instructions.
						record := []byte{0xDD}
						if has[gid] != '1' {
							d, drawOK = ahDraw{}, false
							if outlineOK[gid] {
								d, drawOK = f.drawAt(outlines[gid], size, loc.Coords, loc.Instance[si], loc.Enabled[si], v.BackwardCompat)
							}
							record = skrifaDrawRecord(d, drawOK)
						}
						draws[1] = append(draws[1], skrifaFNV1a64(record))
					}
				}
				if vi == 0 {
					baseDraws = append(baseDraws, draws)
				}
				if loc.ChangedCount != 0 {
					changed++
				}
				for mode, want := range []string{loc.Unhinted, loc.Hinted} {
					c := &unhinted
					if mode == 1 {
						c = &hinted
					}
					c.total++
					var all []byte
					for _, h := range draws[mode] {
						all = binary.BigEndian.AppendUint64(all, h)
					}
					if fmt.Sprintf("%016x", skrifaFNV1a64(all)) == want {
						c.pass++
						continue
					}
					// The first draws that differ. The golden file has the
					// tags of the draws that differ from the base font. The
					// other draws are the draws of the base font.
					fold := func(h uint64) int { return int(uint16(h ^ h>>16 ^ h>>32 ^ h>>48)) }
					expect := map[int]int{}
					for _, ch := range loc.Changed {
						if ch[2] == mode {
							expect[ch[0]*len(golden.SizeBits)+ch[1]] = ch[3]
						}
					}
					var where []string
					for i, h := range draws[mode] {
						want, listed := expect[i]
						if !listed {
							if loc.ChangedCount > len(loc.Changed) || i >= len(baseDraws[li][mode]) {
								continue
							}
							want = fold(baseDraws[li][mode][i])
						}
						if fold(h) != want && len(where) < 6 {
							where = append(where, fmt.Sprintf("glyph %d size %s", i/len(golden.SizeBits), golden.Sizes[i%len(golden.SizeBits)]))
						}
					}
					fail("wght %v: the draws of mode %d differ: %s", golden.Wght[li], mode, strings.Join(where, ", "))
				}
			}
			if ok {
				variants.pass++
			}
		}
		t.Logf("%s: %d of %d variants match in all results (%d with draws that differ from the base font); skrifa has no result for %d more variants",
			sf.name, variants.pass, variants.total, changed, panics.total)
		t.Logf("%s: coords %d of %d, MVAR deltas %d of %d, advances %d of %d, draws without hinting %d of %d, hinted draws %d of %d, hasInstructions %d of %d (hashes over all glyphs)",
			sf.name, coords.pass+norm.pass, coords.total+norm.total, mvar.pass, mvar.total, advance.pass, advance.total, unhinted.pass, unhinted.total, hinted.pass, hinted.total, hasHinting.pass, hasHinting.total)
	}
}

// TestSkrifaBackwardCompat finds the backward compatibility mode of the
// interpreter for each font (HintInstance::backward_compatibility), and
// compares the font size that the prep program sees.
//
// skrifa does not return the mode. The oracle makes a copy of the font in
// which some glyphs have a left side bearing in hmtx that is 7 units less,
// so that their first phantom point is between two pixels. With the mode
// off the scaler rounds the phantom points in the hinted mode, and the left
// side bearing of the hinted draw is a whole number of pixels. The test
// makes the same copy and draws the glyphs with the mode on and off: only
// one of the two gives the left side bearing and the advance of skrifa.
//
// The golden file has also HintingInstance::is_enabled for each whole font
// size from 1 to 300, and for some sizes between whole numbers. The test
// compares the second list with the first one at hintedPpem.
func TestSkrifaBackwardCompat(t *testing.T) {
	for _, sf := range loadSkrifaFonts(t) {
		probe := sf.golden.LsbProbe
		if probe == nil {
			t.Fatalf("%s: no probe in the golden file", sf.name)
		}
		// The copy of the font with the other left side bearings.
		data := append([]byte{}, sf.data...)
		tableOffset := func(tag string) int {
			for i := 0; i < int(binary.BigEndian.Uint16(data[4:])); i++ {
				if record := data[12+16*i:]; string(record[:4]) == tag {
					return int(binary.BigEndian.Uint32(record[8:]))
				}
			}
			t.Fatalf("%s: no table %s", sf.name, tag)
			return 0
		}
		hhea, hmtx := tableOffset("hhea"), tableOffset("hmtx")
		metrics := uint32(binary.BigEndian.Uint16(data[hhea+34:]))
		for _, gid := range probe.Glyphs {
			pos := hmtx + 4*int(gid) + 2
			if gid >= metrics {
				pos = hmtx + 4*int(metrics) + 2*int(gid-metrics)
			}
			lsb := int16(binary.BigEndian.Uint16(data[pos:])) + probe.Delta
			binary.BigEndian.PutUint16(data[pos:], uint16(lsb))
		}
		loaders, err := ot.NewLoaders(bytes.NewReader(data))
		if err != nil || len(loaders) != 1 {
			t.Fatalf("%s: %v", sf.name, err)
		}
		f := newAHFont(loaders[0])
		loc := &sf.golden.Locations[0]
		// match counts the rows that a value of the mode gives.
		match := func(backwardCompat bool) (n, rounded int) {
			for _, row := range probe.Rows {
				gid, si := uint32(row[0]), int(row[1])
				size := math.Float32frombits(sf.golden.SizeBits[si])
				o, ok := f.outline(gid)
				if !ok {
					continue
				}
				unhinted, ok := f.drawAt(o, size, nil, false, false, false)
				if !ok || int64(math.Float32bits(unhinted.lsb)) != row[2] {
					t.Errorf("%s glyph %d size %s: left side bearing without hinting %v", sf.name, gid, sf.golden.Sizes[si], unhinted.lsb)
					continue
				}
				d, ok := f.drawAt(o, size, nil, loc.Instance[si], loc.Enabled[si], backwardCompat)
				same := ok && int64(math.Float32bits(d.lsb)) == row[3] && int64(math.Float32bits(d.advance)) == row[4]
				if same && sf.golden.HasHinting[gid] == '0' {
					// Without instructions the draw is exact.
					h := skrifaFNV1a64(skrifaDrawRecord(d, ok))
					same = int64(uint16(h^h>>16^h>>32^h>>48)) == row[5]
				}
				if same {
					n++
				}
				if row[2] != row[3] {
					rounded++
				}
			}
			return n, rounded
		}
		on, rounded := match(true)
		off, _ := match(false)
		t.Logf("%s: %d draws of %d glyphs: the mode on gives %d, the mode off gives %d; skrifa rounds the left side bearing in %d draws",
			sf.name, len(probe.Rows), len(probe.Glyphs), on, off, rounded)
		want, other := on, off
		if !sf.backwardCompat {
			want, other = off, on
		}
		if want != len(probe.Rows) || other == len(probe.Rows) {
			t.Errorf("%s: backward compatibility %v gives %d of %d draws, the other value gives %d", sf.name, sf.backwardCompat, want, len(probe.Rows), other)
		}

		// HintingInstance::is_enabled by the font size that the prep
		// program sees.
		byPpem := sf.golden.EnabledByPpem
		enabledAt := func(size float32) (byte, bool) {
			ix := int(sf.font.hintedPpem(size)) - byPpem.From
			if ix < 0 || ix >= len(byPpem.Values) {
				return 0, false
			}
			return byPpem.Values[ix], true
		}
		total, pass := 0, 0
		check := func(size float32, want byte) {
			if got, ok := enabledAt(size); ok {
				total++
				if got == want {
					pass++
				} else {
					t.Errorf("%s size %v: enabled %c at the ppem %d, want %c", sf.name, size, got, sf.font.hintedPpem(size), want)
				}
			}
		}
		for _, pair := range sf.golden.EnabledBySize {
			check(math.Float32frombits(pair[0]), byte('0'+pair[1]))
		}
		for si, bits := range sf.golden.SizeBits {
			want := byte('2')
			if loc.Instance[si] {
				want = '0'
				if loc.Enabled[si] {
					want = '1'
				}
			}
			check(math.Float32frombits(bits), want)
		}
		notEnabled := 0
		for _, c := range byPpem.Values {
			if c != '1' {
				notEnabled++
			}
		}
		t.Logf("%s: is_enabled at %d font sizes matches the table by whole ppem at hintedPpem: %d; the hinting instance is not enabled at %d of the ppem %d to %d",
			sf.name, total, pass, notEnabled, byPpem.From, byPpem.From+len(byPpem.Values)-1)
	}
}

// TestScaleOutlineAtDefault compares the scaler for an instance with the
// scaler that the package had before it, for every glyph of the fonts of
// the oracle: without coordinates and without hinting, scaleOutlineAt gives
// the points, the contours and the phantom points of unhintedOutline, and
// pathAt gives the segments of unhintedPath.
func TestScaleOutlineAtDefault(t *testing.T) {
	for _, sf := range loadSkrifaFonts(t) {
		total, pass := 0, 0
		for gid := uint32(0); gid < uint32(sf.golden.GlyphCount); gid++ {
			o, ok := sf.font.outline(gid)
			if !ok {
				t.Fatalf("%s glyph %d: no outline", sf.name, gid)
			}
			for si, bits := range sf.golden.SizeBits {
				size := math.Float32frombits(bits)
				total++
				old, oldOK := sf.font.unhintedOutline(o, size)
				at, atOK := sf.font.scaleOutlineAt(o, size, nil, false, false)
				var oldPen ahPathPen
				oldPath := sf.font.unhintedPath(o, size, &oldPen)
				segments, pathOK := sf.font.pathAt(o, size, nil, false, false)
				same := oldOK == atOK && oldPath == pathOK && fmt.Sprint(oldPen.segments) == fmt.Sprint(segments)
				if same && oldOK {
					same = fmt.Sprint(old.scaled[:old.pointCount], old.flags[:old.pointCount], old.contours[:old.contourCount], old.phantom) ==
						fmt.Sprint(at.scaled[:at.pointCount], at.flags[:at.pointCount], at.contours[:at.contourCount], at.phantom)
				}
				if same {
					pass++
				} else if total-pass <= 5 {
					t.Errorf("%s glyph %d size %s: scaleOutlineAt differs from unhintedOutline", sf.name, gid, sf.golden.Sizes[si])
				}
			}
		}
		if pass != total {
			t.Errorf("%s: %d of %d outlines are the same", sf.name, pass, total)
		}
	}
}
