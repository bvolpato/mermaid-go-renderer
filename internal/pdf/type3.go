// This file implements original Type 3 outline embedding under ISO 32000-1.
// It does not port an upstream implementation.
package pdf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
	"github.com/bvolpato/mermaid-go-renderer/internal/font/webfontdata"
	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
)

// Each resource contains at most 255 glyph/text pairs. A new resource handles
// later pairs, since a Type 3 font uses single-byte character codes.
type type3Resource struct {
	face       *font.Face
	glyphCount uint32
	codes      map[fontGlyphKey]uint16
	keys       []fontGlyphKey
	outlines   map[uint32]*type3Glyph
}

type type3Glyph struct {
	width float64
	bbox  [4]float64
	path  string
}

func newType3Resource(face *font.Face) (*type3Resource, error) {
	if face == nil || face.GoText() == nil || face.Index < 0 {
		return nil, fmt.Errorf("pdf: invalid Type 3 font face")
	}
	var data []byte
	var err error
	switch {
	case strings.HasPrefix(face.Path, "embedded:"):
		data, err = liberation.Font(strings.TrimPrefix(face.Path, "embedded:"))
	case strings.HasPrefix(face.Path, "webfont:"):
		data, err = webfontdata.Font(strings.TrimPrefix(face.Path, "webfont:"))
	default:
		data, err = os.ReadFile(face.Path)
	}
	if err != nil {
		return nil, fmt.Errorf("pdf: load Type 3 font: %w", err)
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || face.Index >= len(loaders) {
		return nil, fmt.Errorf("pdf: invalid Type 3 font source or collection index")
	}
	ld := loaders[face.Index]
	if ld.HasTable(ot.MustNewTag("OS/2")) {
		os2, err := ld.RawTable(ot.MustNewTag("OS/2"))
		if err != nil {
			return nil, fmt.Errorf("pdf: read Type 3 embedding rights: %w", err)
		}
		if err := type3EmbeddingRights(os2); err != nil {
			return nil, err
		}
	}
	// A monochrome outline cannot preserve color, SVG, or bitmap font content.
	for _, tag := range []string{"COLR", "CPAL", "CBDT", "CBLC", "sbix", "SVG ", "EBDT", "EBLC", "bdat", "bloc"} {
		if ld.HasTable(ot.MustNewTag(tag)) {
			return nil, fmt.Errorf("pdf: Type 3 does not support font table %q", tag)
		}
	}
	if !ld.HasTable(ot.MustNewTag("glyf")) && !ld.HasTable(ot.MustNewTag("CFF ")) && !ld.HasTable(ot.MustNewTag("CFF2")) {
		return nil, fmt.Errorf("pdf: Type 3 font has no supported outlines")
	}
	maxp, err := ld.RawTable(ot.MustNewTag("maxp"))
	if err != nil || len(maxp) < 6 || binary.BigEndian.Uint16(maxp[4:]) == 0 {
		return nil, fmt.Errorf("pdf: invalid Type 3 glyph count")
	}
	if upem := face.UnitsPerEm(); !finite(upem) || upem <= 0 {
		return nil, fmt.Errorf("pdf: invalid Type 3 units per em")
	}
	return &type3Resource{
		face: face, glyphCount: uint32(binary.BigEndian.Uint16(maxp[4:])),
		codes: make(map[fontGlyphKey]uint16), outlines: make(map[uint32]*type3Glyph),
	}, nil
}

// Type 3 embeds a subset of converted outlines. It cannot satisfy a prohibition
// on subsetting, even when embedding the complete original font would be allowed.
func type3EmbeddingRights(os2 []byte) error {
	if len(os2) < 78 {
		return fmt.Errorf("pdf: truncated Type 3 OS/2 table")
	}
	version := binary.BigEndian.Uint16(os2)
	if version > 5 || version >= 1 && len(os2) < 86 || version >= 2 && len(os2) < 96 || version >= 5 && len(os2) < 100 {
		return fmt.Errorf("pdf: unsupported or truncated Type 3 OS/2 version %d", version)
	}
	fsType := binary.BigEndian.Uint16(os2[8:])
	permissions := fsType & 0x0e
	if permissions == 2 || version >= 3 && permissions != 0 && permissions != 4 && permissions != 8 || version >= 2 && fsType&0x300 != 0 {
		return fmt.Errorf("pdf: Type 3 embedding is restricted by OS/2 fsType")
	}
	return nil
}

func (f *type3Resource) glyphCode(cid uint16) string { return fmt.Sprintf("%02X", cid) }

func (f *type3Resource) encodeGlyph(gid uint32, text string) (uint16, bool) {
	if gid >= f.glyphCount || !utf8.ValidString(text) {
		return 0, false
	}
	// A ToUnicode destination may contain at most 512 bytes of UTF-16BE.
	units := 0
	for _, r := range text {
		units++
		if r > 0xffff {
			units++
		}
		if units > 256 {
			return 0, false
		}
	}
	key := fontGlyphKey{gid, text}
	if code, ok := f.codes[key]; ok {
		return code, true
	}
	if len(f.keys) == 255 {
		return 0, false
	}
	if _, ok := f.outlines[gid]; !ok {
		glyph, ok := f.outline(gid)
		if !ok {
			return 0, false
		}
		f.outlines[gid] = glyph
	}
	code := uint16(len(f.keys) + 1)
	f.keys = append(f.keys, key)
	f.codes[key] = code
	return code, true
}

func (f *type3Resource) outline(gid uint32) (glyph *type3Glyph, ok bool) {
	// Some malformed glyphs pass the source loader and fail during lazy outline
	// decoding. Reject those glyphs instead of panicking during PDF generation.
	defer func() {
		if recover() != nil {
			glyph, ok = nil, false
		}
	}()
	// Use the same canonical unhinted outlines as the vector fallback. These
	// include the selected variation and synthetic styles of Linux web fonts.
	outline, ok := f.face.ScalerOutlineUnhinted(gid, 64)
	scale := 1000.0 / 64
	if !ok {
		outline, ok = f.face.GoText().GlyphDataOutline(gtfont.GID(gid))
		scale = 1000 / f.face.UnitsPerEm()
	}
	if !ok {
		return nil, false
	}
	width := float64(f.face.GoText().HorizontalAdvance(gtfont.GID(gid))) * 1000 / f.face.UnitsPerEm()
	if !finite(width) {
		return nil, false
	}
	path, bbox, ok := type3Outline(outline.Segments, scale)
	if !ok {
		return nil, false
	}
	return &type3Glyph{type3Round(width), bbox, path}, true
}

func type3Round(value float64) float64 { return math.Round(value*1000) / 1000 }

func type3Outline(segments []ot.Segment, scale float64) (string, [4]float64, bool) {
	var b strings.Builder
	var bbox [4]float64
	var current ot.SegmentPoint
	open, bounded := false, false
	point := func(p ot.SegmentPoint) bool {
		x, y := type3Round(float64(p.X)*scale), type3Round(float64(p.Y)*scale)
		if !finite(x) || !finite(y) {
			return false
		}
		if !bounded {
			bbox, bounded = [4]float64{x, y, x, y}, true
		} else {
			bbox = [4]float64{min(bbox[0], x), min(bbox[1], y), max(bbox[2], x), max(bbox[3], y)}
		}
		fmt.Fprintf(&b, "%s %s ", number(x), number(y))
		return true
	}
	for _, seg := range segments {
		a := seg.Args
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			if open {
				b.WriteString("h\n")
			}
			if !point(a[0]) {
				return "", bbox, false
			}
			b.WriteString("m\n")
			current, open = a[0], true
		case ot.SegmentOpLineTo:
			if !open || !point(a[0]) {
				return "", bbox, false
			}
			b.WriteString("l\n")
			current = a[0]
		case ot.SegmentOpQuadTo:
			control1 := ot.SegmentPoint{X: current.X + (a[0].X-current.X)*2/3, Y: current.Y + (a[0].Y-current.Y)*2/3}
			control2 := ot.SegmentPoint{X: a[1].X + (a[0].X-a[1].X)*2/3, Y: a[1].Y + (a[0].Y-a[1].Y)*2/3}
			if !open || !point(control1) || !point(control2) || !point(a[1]) {
				return "", bbox, false
			}
			b.WriteString("c\n")
			current = a[1]
		case ot.SegmentOpCubeTo:
			if !open || !point(a[0]) || !point(a[1]) || !point(a[2]) {
				return "", bbox, false
			}
			b.WriteString("c\n")
			current = a[2]
		default:
			return "", bbox, false
		}
	}
	if open {
		b.WriteString("h\nf\n")
	}
	return b.String(), bbox, true
}

func (f *type3Resource) writePDF(w *objectWriter) int {
	var procs, widths, differences strings.Builder
	var bbox [4]float64
	refs := make(map[uint32]int)
	for i, key := range f.keys {
		glyph := f.outlines[key.gid]
		ref, found := refs[key.gid]
		if !found {
			var data strings.Builder
			fmt.Fprintf(&data, "%s 0 %s %s %s %s d1\n%s", number(glyph.width), number(glyph.bbox[0]), number(glyph.bbox[1]), number(glyph.bbox[2]), number(glyph.bbox[3]), glyph.path)
			ref = w.addStream("", []byte(data.String()))
			refs[key.gid] = ref
			bbox = [4]float64{min(bbox[0], glyph.bbox[0]), min(bbox[1], glyph.bbox[1]), max(bbox[2], glyph.bbox[2]), max(bbox[3], glyph.bbox[3])}
		}
		fmt.Fprintf(&procs, "/g%d %d 0 R ", i+1, ref)
		fmt.Fprintf(&widths, "%s ", number(glyph.width))
		fmt.Fprintf(&differences, "/g%d ", i+1)
	}
	toUnicode := ""
	if cmap := f.toUnicode(); len(cmap) != 0 {
		toUnicode = fmt.Sprintf(" /ToUnicode %d 0 R", w.addStream("", cmap))
	}
	return w.addObject(fmt.Sprintf("<< /Type /Font /Subtype /Type3 /FontBBox [%s %s %s %s] /FontMatrix [0.001 0 0 0.001 0 0] /CharProcs << %s>> /Encoding << /Type /Encoding /Differences [1 %s] >> /FirstChar 1 /LastChar %d /Widths [%s] /Resources << >>%s >>", number(bbox[0]), number(bbox[1]), number(bbox[2]), number(bbox[3]), procs.String(), differences.String(), len(f.keys), widths.String(), toUnicode))
}

func (f *type3Resource) toUnicode() []byte {
	var entries []string
	for i, key := range f.keys {
		if key.text == "" {
			continue
		}
		var entry strings.Builder
		fmt.Fprintf(&entry, "<%02X> <", i+1)
		for _, unit := range utf16.Encode([]rune(key.text)) {
			fmt.Fprintf(&entry, "%04X", unit)
		}
		entry.WriteString(">\n")
		entries = append(entries, entry.String())
	}
	if len(entries) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n/CMapName /Type3-UCS def\n/CMapType 2 def\n1 begincodespacerange\n<00> <FF>\nendcodespacerange\n")
	for start := 0; start < len(entries); start += 100 {
		end := min(start+100, len(entries))
		fmt.Fprintf(&b, "%d beginbfchar\n", end-start)
		for _, entry := range entries[start:end] {
			b.WriteString(entry)
		}
		b.WriteString("endbfchar\n")
	}
	b.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return []byte(b.String())
}
