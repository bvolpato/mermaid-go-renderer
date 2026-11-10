// This file reads the OpenType MATH table as specified in
// https://learn.microsoft.com/en-us/typography/opentype/spec/math.
// It ports no upstream implementation and uses go-text/typesetting v0.3.5
// (Unlicense OR BSD-3-Clause) to read sfnt tables.

package font

import (
	"bytes"
	"os"
	"sort"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
	ot "github.com/go-text/typesetting/font/opentype"
)

type mathFontSource struct {
	table     ahData
	numGlyphs uint32
}

// This bounded cache holds immutable table bytes, including absent tables.
var mathFontSources = memo.NewMap[*Face, mathFontSource](128)

func (f *Face) mathSource() mathFontSource {
	if source, ok := mathFontSources.Get(f); ok {
		return source
	}
	var source mathFontSource
	if ld := f.mathLoader(); ld != nil {
		if raw, err := ld.RawTable(ot.MustNewTag("maxp")); err == nil && len(raw) >= 6 {
			source.numGlyphs = uint32(ahData(raw).u16(4))
		}
		if raw, err := ld.RawTable(ot.MustNewTag("MATH")); err == nil && len(raw) >= 10 && ahData(raw).u16(0) == 1 {
			source.table = raw
		}
	}
	mathFontSources.Put(f, source)
	return source
}

func (f *Face) mathLoader() *ot.Loader {
	if f.ft != nil {
		return f.ft.ld
	}
	var data []byte
	if f.web != nil && f.web.font != nil {
		data = f.web.font.data
	} else if name, ok := strings.CutPrefix(f.Path, embeddedPrefix); ok {
		data, _ = liberation.Font(name)
	} else if f.Path != "" {
		data, _ = os.ReadFile(f.Path)
	}
	// Non-Linux faces loaded from bytes do not retain their original data.
	// Such faces can still provide glyph metrics and fallback constants.
	if len(data) == 0 {
		return nil
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || f.Index < 0 || f.Index >= len(loaders) {
		return nil
	}
	return loaders[f.Index]
}

// mathCoverageIndex supports both OpenType coverage formats. Every array
// is checked before access, including arrays in malformed optional tables.
func mathCoverageIndex(d ahData, gid uint32) (int, bool) {
	if len(d) < 4 || gid > 0xffff {
		return 0, false
	}
	n := int(d.u16(2))
	switch d.u16(0) {
	case 1:
		a := d.array(4, n, 2)
		if len(a) != n*2 {
			return 0, false
		}
		i := sort.Search(n, func(i int) bool { return uint32(a.u16(i*2)) >= gid })
		return i, i < n && uint32(a.u16(i*2)) == gid
	case 2:
		a := d.array(4, n, 6)
		if len(a) != n*6 {
			return 0, false
		}
		i := sort.Search(n, func(i int) bool { return uint32(a.u16(i*6+2)) >= gid })
		if i < n {
			start, end := uint32(a.u16(i*6)), uint32(a.u16(i*6+2))
			if start <= gid && gid <= end {
				return int(a.u16(i*6+4)) + int(gid-start), true
			}
		}
	}
	return 0, false
}

func mathGlyphInfo(d ahData) ahData {
	if len(d) < 10 {
		return nil
	}
	info, _ := d.resolve(int(d.u16(6)), 8)
	return info
}

func mathGlyphValue(info ahData, field int, gid uint32) (int16, bool) {
	if len(info) < 8 {
		return 0, false
	}
	t, ok := info.resolve(int(info.u16(field)), 4)
	if !ok {
		return 0, false
	}
	coverage, ok := t.resolve(int(t.u16(0)), 4)
	if !ok {
		return 0, false
	}
	i, ok := mathCoverageIndex(coverage, gid)
	n := int(t.u16(2))
	if !ok || i >= n || len(t.array(4, n, 4)) != n*4 {
		return 0, false
	}
	return t.i16(4 + i*4), true
}

// MathGlyphVariant is a ready-made size variant from the MATH table.
// Advance is its full extent along the requested stretch axis, in pixels.
// It is not the horizontal text advance. Use MathGlyphByID for that value.
type MathGlyphVariant struct {
	ID      uint32
	Advance float64
}

// MathVariants returns the size variants of gid in their font table order.
// vertical selects vertical stretching. False selects horizontal stretching.
// The result is owned by the caller. The base glyph is not added implicitly.
// Glyph assemblies, device adjustments, and variation adjustments are omitted.
func (f *Face) MathVariants(gid uint32, size float64, vertical bool) []MathGlyphVariant {
	if f == nil || f.gt == nil || f.upem <= 0 || !mathValidSize(size) || gid > 0xffff {
		return nil
	}
	source := f.mathSource()
	d := source.table
	if len(d) < 10 || (source.numGlyphs != 0 && gid >= source.numGlyphs) {
		return nil
	}
	v, ok := d.resolve(int(d.u16(8)), 10)
	if !ok {
		return nil
	}
	vertCount, horizCount := int(v.u16(6)), int(v.u16(8))
	if len(v.array(10, vertCount+horizCount, 2)) != (vertCount+horizCount)*2 {
		return nil
	}
	coverageOffset, count, first := int(v.u16(4)), horizCount, vertCount
	if vertical {
		coverageOffset, count, first = int(v.u16(2)), vertCount, 0
	}
	coverage, ok := v.resolve(coverageOffset, 4)
	if !ok {
		return nil
	}
	i, ok := mathCoverageIndex(coverage, gid)
	if !ok || i >= count {
		return nil
	}
	construction, ok := v.resolve(int(v.u16(10+(first+i)*2)), 4)
	if !ok {
		return nil
	}
	n := int(construction.u16(2))
	records := construction.array(4, n, 4)
	if len(records) != n*4 {
		return nil
	}
	var out []MathGlyphVariant
	for i := 0; i < n; i++ {
		id := uint32(records.u16(i * 4))
		if source.numGlyphs != 0 && id >= source.numGlyphs {
			continue
		}
		out = append(out, MathGlyphVariant{ID: id, Advance: float64(records.u16(i*4+2)) * size / f.upem})
	}
	return out
}
