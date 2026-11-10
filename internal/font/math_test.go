// This file ports no upstream implementation. It checks the OpenType MATH
// table format, Chromium 156 MathML fallback formulas, and go-text metrics.

package font

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
)

func mathPut16(d []byte, off int, value int) {
	binary.BigEndian.PutUint16(d[off:], uint16(value))
}

// mathTestTable uses both coverage formats and both stretch directions.
func mathTestTable(gid uint32) []byte {
	d := make([]byte, 332)
	mathPut16(d, 0, 1)
	mathPut16(d, 4, 10)
	mathPut16(d, 6, 224)
	mathPut16(d, 8, 278)
	c := d[10:224]
	mathPut16(c, 0, 80)
	mathPut16(c, 2, 60)
	mathPut16(c, 4, 40000)
	mathPut16(c, 6, 1200)
	mathPut16(c, 12, 250)   // AxisHeight.
	mathPut16(c, 144, 40)   // FractionRuleThickness.
	mathPut16(c, 208, -550) // RadicalKernAfterDegree.
	mathPut16(c, 212, 60)
	info := d[224:278]
	mathPut16(info, 0, 8)
	mathPut16(info, 2, 22)
	mathPut16(info, 4, 44)
	italic := info[8:22]
	mathPut16(italic, 0, 8)
	mathPut16(italic, 2, 1)
	mathPut16(italic, 4, 75)
	mathPut16(italic, 8, 1)
	mathPut16(italic, 10, 1)
	mathPut16(italic, 12, int(gid))
	accent := info[22:44]
	mathPut16(accent, 0, 8)
	mathPut16(accent, 2, 1)
	mathPut16(accent, 4, -125)
	mathPut16(accent, 8, 2)
	mathPut16(accent, 10, 1)
	mathPut16(accent, 12, int(gid))
	mathPut16(accent, 14, int(gid))
	mathPut16(info, 44, 2)
	mathPut16(info, 46, 1)
	mathPut16(info, 48, int(gid))
	mathPut16(info, 50, int(gid))
	v := d[278:]
	mathPut16(v, 0, 20)
	mathPut16(v, 2, 14)
	mathPut16(v, 4, 20)
	mathPut16(v, 6, 1)
	mathPut16(v, 8, 1)
	mathPut16(v, 10, 26)
	mathPut16(v, 12, 42)
	for _, offset := range []int{14, 20} {
		mathPut16(v, offset, 1)
		mathPut16(v, offset+2, 1)
		mathPut16(v, offset+4, int(gid))
	}
	mathPut16(v, 28, 3)
	mathPut16(v, 30, int(gid))
	mathPut16(v, 32, 1000)
	mathPut16(v, 34, int(gid+1))
	mathPut16(v, 36, 2000)
	mathPut16(v, 38, 65535) // Invalid variant glyph, which must be skipped.
	mathPut16(v, 40, 3000)
	mathPut16(v, 44, 1)
	mathPut16(v, 46, int(gid+2))
	mathPut16(v, 48, 1500)
	return d
}

func mathTestFontData(t *testing.T, table []byte) []byte {
	t.Helper()
	base, err := liberation.Font("LiberationSerif-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	ld, err := ot.NewLoader(bytes.NewReader(base))
	if err != nil {
		t.Fatal(err)
	}
	tags := append(ld.Tables(), ot.MustNewTag("MATH"))
	sort.Slice(tags, func(i, j int) bool { return tags[i] < tags[j] })
	d := make([]byte, 12+len(tags)*16)
	binary.BigEndian.PutUint32(d, uint32(ot.TrueType))
	mathPut16(d, 4, len(tags))
	for i, tag := range tags {
		raw := table
		if tag != ot.MustNewTag("MATH") {
			raw, err = ld.RawTable(tag)
			if err != nil {
				t.Fatal(err)
			}
		}
		entry := d[12+i*16:]
		binary.BigEndian.PutUint32(entry, uint32(tag))
		binary.BigEndian.PutUint32(entry[8:], uint32(len(d)))
		binary.BigEndian.PutUint32(entry[12:], uint32(len(raw)))
		d = append(d, raw...)
		for len(d)%4 != 0 {
			d = append(d, 0)
		}
	}
	return d
}

func mathTestFace(t *testing.T, table []byte) *Face {
	t.Helper()
	faces, err := LoadFor(PlatformLinux, mathTestFontData(t, table))
	if err != nil {
		t.Fatal(err)
	}
	return faces[0]
}

func TestMathConstantsAndGlyphInfo(t *testing.T) {
	base := Embedded().Match("Liberation Serif", 400, false)
	id, _ := base.NominalGlyph('x')
	f := mathTestFace(t, mathTestTable(id))
	size := f.UnitsPerEm()
	for _, c := range []struct {
		name string
		want float64
	}{
		{"ScriptPercentScaleDown", .8},
		{"ScriptScriptPercentScaleDown", .6},
		{"DelimitedSubFormulaMinHeight", 40000},
		{"DisplayOperatorMinHeight", 1200},
		{"AxisHeight", 250},
		{"FractionRuleThickness", 40},
		{"RadicalKernAfterDegree", -550},
		{"RadicalDegreeBottomRaisePercent", .6},
	} {
		got, ok := f.MathConstant(c.name, size)
		if !ok || got != c.want {
			t.Errorf("%s = %v, %v; want %v, true", c.name, got, ok, c.want)
		}
	}
	if got, ok := f.MathConstant("AxisHeight", size/2); !ok || got != 125 {
		t.Errorf("scaled axis = %v, %v", got, ok)
	}
	g, ok := f.MathGlyph('x', size)
	if !ok || g.ID != id || !g.HasInk || !g.ExtendedShape || g.ItalicCorrection != 75 || !g.HasTopAccentAttachment || g.TopAccentAttachment != -125 {
		t.Fatalf("glyph metrics = %+v, %v", g, ok)
	}
	private := gtfont.NewFace(f.GoText().Font)
	ext, _ := private.GlyphExtents(gtfont.GID(id))
	if g.Advance != float64(private.HorizontalAdvance(gtfont.GID(id))) || g.Left != float64(ext.XBearing) || g.Top != -float64(ext.YBearing) || g.Right != float64(ext.XBearing+ext.Width) || g.Bottom != -float64(ext.YBearing+ext.Height) {
		t.Errorf("glyph metrics disagree with go-text: %+v, %+v", g, ext)
	}
	vertical := f.MathVariants(id, size, true)
	if len(vertical) != 2 || vertical[0] != (MathGlyphVariant{ID: id, Advance: 1000}) || vertical[1] != (MathGlyphVariant{ID: id + 1, Advance: 2000}) {
		t.Errorf("vertical variants = %+v", vertical)
	}
	horizontal := f.MathVariants(id, size/2, false)
	if len(horizontal) != 1 || horizontal[0] != (MathGlyphVariant{ID: id + 2, Advance: 750}) {
		t.Errorf("horizontal variants = %+v", horizontal)
	}
	if _, ok := f.MathGlyphByID(65535, size); ok {
		t.Error("accepted glyph outside maxp count")
	}
}

func TestMathFallbackAndInvalidInput(t *testing.T) {
	f := Embedded().Match("Liberation Serif", 400, false)
	size := 20.0
	x := f.Metrics(size).XHeight
	rule := f.UnderlineThickness(size)
	for _, c := range []struct {
		name string
		want float64
	}{
		{"AxisHeight", x / 2},
		{"SubscriptShiftDown", x / 3},
		{"SuperscriptShiftUp", x},
		{"FractionRuleThickness", rule},
		{"FractionNumDisplayStyleGapMin", 3 * rule},
		{"RadicalDisplayStyleVerticalGap", rule + float64(x/4)},
		{"ScriptPercentScaleDown", .71},
		{"RadicalDegreeBottomRaisePercent", .6},
		{"NoSuchConstant", 0},
	} {
		got, fromFont := f.MathConstant(c.name, size)
		if fromFont || got != c.want {
			t.Errorf("fallback %s = %v, %v; want %v, false", c.name, got, fromFont, c.want)
		}
	}
	if rule != f.underlineThickness*size/f.upem || rule <= 0 {
		t.Errorf("underline thickness = %v", rule)
	}
	blank, ok := f.MathGlyph(' ', size)
	if !ok || blank.HasInk || blank.Advance <= 0 || blank.ItalicCorrection != 0 || blank.HasTopAccentAttachment || blank.TopAccentAttachment != blank.Advance/2 {
		t.Errorf("blank glyph = %+v, %v", blank, ok)
	}
	for _, size := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if v, ok := f.MathConstant("AxisHeight", size); ok || v != 0 {
			t.Errorf("accepted size %v", size)
		}
		if _, ok := f.MathGlyph('x', size); ok {
			t.Errorf("accepted glyph size %v", size)
		}
		if variants := f.MathVariants(1, size, true); len(variants) != 0 {
			t.Errorf("accepted variant size %v", size)
		}
	}
	for _, r := range []rune{-1, 0x10ffff, 0x110000} {
		if _, ok := f.MathGlyph(r, 16); ok {
			t.Errorf("accepted missing character U+%X", r)
		}
	}
	var absent *Face
	if v, ok := absent.MathConstant("AxisHeight", 16); ok || v != 0 {
		t.Error("accepted nil face")
	}
	if family := Embedded().MathFamily(); family != "Liberation Serif" {
		t.Errorf("embedded math family = %q", family)
	}
	withoutPost := &Face{upem: 1000}
	if v := withoutPost.UnderlineThickness(20); v != 1 {
		t.Errorf("missing thickness fallback = %v", v)
	}
}

func TestMathMalformedTables(t *testing.T) {
	id, _ := Embedded().Match("Liberation Serif", 400, false).NominalGlyph('x')
	full := mathTestTable(id)
	// Each truncation must remain safe, including partial coverage arrays.
	for end := 0; end <= len(full); end++ {
		f := mathTestFace(t, full[:end])
		f.MathConstant("AxisHeight", 16)
		f.MathGlyph('x', 16)
		f.MathVariants(id, 16, true)
		f.MathVariants(id, 16, false)
	}
	for _, offset := range []int{0, 4, 6, 8, 224, 226, 228, 232, 234, 278 + 2, 278 + 6, 278 + 10, 278 + 28} {
		bad := bytes.Clone(full)
		mathPut16(bad, offset, 65535)
		f := mathTestFace(t, bad)
		f.MathConstant("AxisHeight", 16)
		f.MathGlyph('x', 16)
		f.MathVariants(id, 16, true)
	}
}

func TestMathConcurrent(t *testing.T) {
	id, _ := Embedded().Match("Liberation Serif", 400, false).NominalGlyph('x')
	f := mathTestFace(t, mathTestTable(id))
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 30 {
				if _, ok := f.MathConstant("AxisHeight", 16); !ok {
					t.Error("missing MATH constants")
				}
				if g, ok := f.MathGlyph('x', 16); !ok || !g.HasInk {
					t.Error("missing glyph metrics")
				}
				f.MathVariants(id, 16, true)
				f.Shape([]rune("math"), 16)
			}
		})
	}
	wg.Wait()
}

func TestMathLocalSTIX(t *testing.T) {
	const path = "/System/Library/Fonts/Supplemental/STIXTwoMath.otf"
	if _, err := os.Stat(path); err != nil {
		t.Skip("local STIX Two Math is unavailable")
	}
	faces, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f := faces[0]
	for _, name := range []string{"AxisHeight", "FractionRuleThickness", "ScriptPercentScaleDown"} {
		if v, ok := f.MathConstant(name, 16); !ok || v <= 0 {
			t.Errorf("STIX %s = %v, %v", name, v, ok)
		}
	}
	g, ok := f.MathGlyph('\u222b', 16)
	if !ok || !g.HasInk || g.ItalicCorrection <= 0 {
		t.Errorf("STIX integral metrics = %+v, %v", g, ok)
	}
	if v := f.MathVariants(g.ID, 16, true); len(v) < 2 {
		t.Errorf("STIX integral variants = %+v", v)
	}
}

func TestMathFamilyDarwin(t *testing.T) {
	// A dedicated math font must not replace the generic's primary face.
	l := newLibrary(PlatformDarwin, embedNone, nil)
	for _, family := range []string{"Times", "STIX Two Math", "STIXGeneral"} {
		key := strings.ToLower(family)
		l.families[key] = []*faceRef{{family: family, weight: 400, tried: true, face: &Face{Family: family}}}
		l.markComplete(key)
	}
	if got := l.MathFamily(); got != "Times" {
		t.Errorf("macOS math family with STIX installed = %q; want Times", got)
	}
}
