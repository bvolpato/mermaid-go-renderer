// Ported from skrifa 0.46.2 (src/charmap.rs, src/attribute.rs,
// src/outline/metrics.rs) and from read-fonts 0.43.3 (src/font_data.rs,
// src/offset.rs, src/tables/cmap.rs, src/tables/hmtx.rs, src/tables/loca.rs,
// and the generated readers of the tables).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt).

package font

import (
	"encoding/binary"

	ot "github.com/go-text/typesetting/font/opentype"
)

// ahData is a view of font data: FontData of read-fonts. A reader of
// read-fonts accepts a table when the data has the fixed fields of the
// table. An array that does not fit in the data reads as an empty array.
type ahData []byte

// u16 returns the 16 bit number at an offset, or 0 when it is outside of the
// data.
func (d ahData) u16(off int) uint16 {
	if off < 0 || off+2 > len(d) {
		return 0
	}
	return binary.BigEndian.Uint16(d[off:])
}

// i16 returns the signed 16 bit number at an offset, or 0 when it is outside
// of the data.
func (d ahData) i16(off int) int16 { return int16(d.u16(off)) }

// u32 returns the 32 bit number at an offset, or 0 when it is outside of the
// data.
func (d ahData) u32(off int) uint32 {
	if off < 0 || off+4 > len(d) {
		return 0
	}
	return binary.BigEndian.Uint32(d[off:])
}

// array returns the bytes of an array of count items of size bytes at an
// offset. The result is empty when the array does not fit in the data
// (read_array, and the accessors that replace an error with an empty array).
func (d ahData) array(off, count, size int) ahData {
	end := off + count*size
	if off < 0 || end < off || end > len(d) {
		return nil
	}
	return d[off:end]
}

// rest returns the items of size bytes from an offset to the end of the data.
func (d ahData) rest(off, size int) ahData {
	if off < 0 || off > len(d) {
		return nil
	}
	n := (len(d) - off) / size
	return d[off : off+n*size]
}

// resolve returns the table at an offset (ResolveOffset::resolve). It fails
// for a null offset, for an offset after the data, and for a table that is
// shorter than its fixed fields.
func (d ahData) resolve(off, minSize int) (ahData, bool) {
	if off == 0 || off > len(d) {
		return nil, false
	}
	t := d[off:]
	if len(t) < minSize {
		return nil, false
	}
	return t, true
}

// ahTag returns a tag of four characters as a number.
func ahTag(s string) uint32 {
	return uint32(s[0])<<24 | uint32(s[1])<<16 | uint32(s[2])<<8 | uint32(s[3])
}

// ahBinarySearchBy is binary_search_by of the Rust standard library for a
// slice of n items. cmp compares the item at an index with the target: it
// returns a negative number when the item is less, zero when it is equal and
// a positive number when it is greater. The result is the index of an equal
// item, or the index at which the target can be inserted with found false.
func ahBinarySearchBy(n int, cmp func(i int) int) (index int, found bool) {
	size := n
	if size == 0 {
		return 0, false
	}
	base := 0
	for size > 1 {
		half := size / 2
		mid := base + half
		if cmp(mid) <= 0 {
			base = mid
		}
		size -= half
	}
	c := cmp(base)
	if c == 0 {
		return base, true
	}
	if c < 0 {
		return base + 1, false
	}
	return base, false
}

// ahFont has the tables of a font that the automatic hinter reads: FontRef
// of read-fonts with the tables that Outlines::new of skrifa requires.
type ahFont struct {
	glyf, loca ahData
	locaLong   bool
	unitsPerEm uint16
	// glyphCount is the number of glyphs in the maxp table. It is 0 for a
	// font without the table.
	glyphCount uint16
	hasMaxp    bool
	// hMetrics has the records of the hmtx table, and leftSideBearings has
	// the bearings after the records.
	hMetrics, leftSideBearings ahData
	// os2VMetrics has sTypoAscender and sTypoDescender of the OS/2 table.
	os2VMetrics [2]int16
	charmap     ahCharmap
	gsub        ahData
	hasGsub     bool
	// isItalic says that the style of the font is not normal
	// (Attributes::new).
	isItalic bool
	// isFixedWidth is isFixedPitch of the post table.
	isFixedWidth bool
	// vari has the variation tables gvar, HVAR and MVAR (glyf_var.go). It
	// is nil for a font without them.
	vari *ahVarFont
	// forceIntegerPpem is bit 3 of the flags of the head table: the hinted
	// mode of the scaler rounds the font size to a whole number. It is the
	// opposite of Outlines::fractional_size_hinting.
	forceIntegerPpem bool
	// hdmx is nil for a font without the table.
	hdmx *ahHdmx
}

// ahRawTable returns the data of a table.
func ahRawTable(ld *ot.Loader, tag string) (ahData, bool) {
	b, err := ld.RawTable(ot.MustNewTag(tag))
	if err != nil {
		return nil, false
	}
	return b, true
}

// newAHFont reads the tables of a font. It returns nil for a font without
// TrueType outlines: Outlines::new of skrifa (src/outline/glyf/mod.rs) needs
// the tables head, loca, glyf, hhea and hmtx.
func newAHFont(ld *ot.Loader) *ahFont {
	f := &ahFont{}
	head, ok := ahRawTable(ld, "head")
	if !ok || len(head) < 54 {
		return nil
	}
	f.unitsPerEm = head.u16(18)
	f.locaLong = head.i16(50) == 1
	loca, ok := ahRawTable(ld, "loca")
	if !ok {
		return nil
	}
	if f.locaLong {
		if len(loca)%4 != 0 {
			return nil
		}
	} else if len(loca)%2 != 0 {
		return nil
	}
	f.loca = loca
	if f.glyf, ok = ahRawTable(ld, "glyf"); !ok {
		return nil
	}
	hhea, ok := ahRawTable(ld, "hhea")
	if !ok || len(hhea) < 36 {
		return nil
	}
	hmtx, ok := ahRawTable(ld, "hmtx")
	if !ok {
		return nil
	}
	numberOfHMetrics := int(hhea.u16(34))
	f.hMetrics = hmtx.array(0, numberOfHMetrics, 4)
	f.leftSideBearings = hmtx.rest(4*numberOfHMetrics, 2)
	if maxp, ok := ahRawTable(ld, "maxp"); ok && len(maxp) >= 6 {
		f.glyphCount = maxp.u16(4)
		f.hasMaxp = true
	}
	os2, hasOS2 := ahRawTable(ld, "OS/2")
	hasOS2 = hasOS2 && len(os2) >= 78
	if hasOS2 {
		f.os2VMetrics = [2]int16{os2.i16(68), os2.i16(70)}
		// Bits 0 and 9 of fsSelection are italic and oblique.
		f.isItalic = os2.u16(62)&(1|1<<9) != 0
	} else {
		// Bit 1 of macStyle is italic.
		f.isItalic = head.u16(44)&2 != 0
	}
	if post, ok := ahRawTable(ld, "post"); ok && len(post) >= 32 {
		f.isFixedWidth = post.u32(12) != 0
	}
	if gsub, ok := ahRawTable(ld, "GSUB"); ok && len(gsub) >= 10 {
		f.gsub, f.hasGsub = gsub, true
	}
	if cmap, ok := ahRawTable(ld, "cmap"); ok && len(cmap) >= 4 {
		f.charmap = newAHCharmap(cmap, f)
	}
	f.vari = newAHVarFont(ld)
	f.forceIntegerPpem = head.u16(16)&8 != 0
	f.hdmx = newAHHdmx(ld, f)
	return f
}

// advanceWidth is GlyphHMetrics::advance_width for a font without
// variations: the advance of a glyph in font units.
func (f *ahFont) advanceWidth(gid uint32) int32 {
	n := len(f.hMetrics) / 4
	if n == 0 {
		return 0
	}
	ix := int(gid)
	if ix >= n {
		ix = n - 1
	}
	return int32(f.hMetrics.u16(4 * ix))
}

// lsb is GlyphHMetrics::lsb for a font without variations: the left side
// bearing of a glyph in font units.
func (f *ahFont) lsb(gid uint32) int32 {
	n := len(f.hMetrics) / 4
	ix := int(gid)
	if ix < n {
		return int32(f.hMetrics.i16(4*ix + 2))
	}
	ix -= n
	if 2*ix+2 > len(f.leftSideBearings) {
		return 0
	}
	return int32(f.leftSideBearings.i16(2 * ix))
}

// ---- cmap ----

// The kinds of a mapping, in the order of preference (MappingKind).
const (
	ahMappingNone = iota
	ahMappingUnicodeBmp
	ahMappingUnicodeFull
	ahMappingSymbol
)

// ahCharmap is Charmap of skrifa: the mapping of characters to nominal
// glyphs with the cmap subtable that skrifa selects.
type ahCharmap struct {
	// format is the format of the subtable: 4, 12 or 13. It is 0 for a
	// font without a usable subtable.
	format   int
	isSymbol bool
	// The arrays of a subtable of format 4.
	endCode, startCode, idDelta, idRangeOffsets, glyphIDArray ahData
	// groups has the groups of a subtable of format 12 or 13.
	groups ahData
	// The limits of the iteration (CmapIterLimits::default_for_font).
	maxChar, glyphCount uint32
}

// newAHCharmap is Charmap::new and MappingSelection::new: it searches the
// encoding records from the last to the first. A symbol subtable wins over a
// subtable for all of Unicode, and that wins over a subtable for the basic
// plane.
func newAHCharmap(cmap ahData, f *ahFont) ahCharmap {
	var c ahCharmap
	kind := ahMappingNone
	records := cmap.array(4, int(cmap.u16(2)), 8)
	for i := len(records)/8 - 1; i >= 0; i-- {
		platformID, encodingID := records.u16(8*i), records.u16(8*i+2)
		var k int
		switch {
		case platformID == 0 && encodingID == 5:
			// Unicode variation sequences are not used.
			continue
		case platformID == 3 && encodingID == 0:
			k = ahMappingSymbol
		case platformID == 3 && encodingID == 10, platformID == 0 && encodingID == 4:
			k = ahMappingUnicodeFull
		case platformID == 2, platformID == 0, platformID == 3 && encodingID == 1:
			k = ahMappingUnicodeBmp
		default:
			continue
		}
		sub, ok := newAHCmapSubtable(cmap, int(records.u32(8*i+4)))
		if !ok {
			continue
		}
		if k > kind {
			kind = k
			sub.isSymbol = k == ahMappingSymbol
			c = sub
		}
	}
	c.maxChar = 0x10FFFF
	c.glyphCount = 0xFFFF
	if f.hasMaxp {
		c.glyphCount = uint32(f.glyphCount)
	}
	return c
}

// newAHCmapSubtable reads the subtable at an offset of the cmap table. It
// fails for a format other than 4, 12 and 13.
func newAHCmapSubtable(cmap ahData, off int) (ahCharmap, bool) {
	var c ahCharmap
	if off == 0 || off > len(cmap) {
		return c, false
	}
	d := cmap[off:]
	if len(d) < 2 {
		return c, false
	}
	switch d.u16(0) {
	case 4:
		if len(d) < 16 {
			return c, false
		}
		segCount := int(d.u16(6)) / 2
		c.format = 4
		c.endCode = d.array(14, segCount, 2)
		start := 14 + 2*segCount + 2
		c.startCode = d.array(start, segCount, 2)
		start += 2 * segCount
		c.idDelta = d.array(start, segCount, 2)
		start += 2 * segCount
		c.idRangeOffsets = d.array(start, segCount, 2)
		start += 2 * segCount
		c.glyphIDArray = d.rest(start, 2)
	case 12, 13:
		if len(d) < 16 {
			return c, false
		}
		c.format = int(d.u16(0))
		numGroups := d.u32(12)
		if uint64(numGroups)*12 <= uint64(len(d)) {
			c.groups = d.array(16, int(numGroups), 12)
		}
	default:
		return c, false
	}
	return c, true
}

// mapChar is Charmap::map: the nominal glyph of a character. ok is false for
// a character without a glyph.
func (c *ahCharmap) mapChar(ch uint32) (gid uint32, ok bool) {
	if c.format == 0 {
		return 0, false
	}
	if gid, ok = c.mapImpl(ch); ok {
		return gid, true
	}
	if c.isSymbol && ch <= 0x00FF {
		// A symbol font has the characters U+F000 to U+F0FF at U+0000 to
		// U+00FF also.
		return c.mapImpl(ch + 0xF000)
	}
	return 0, false
}

func (c *ahCharmap) mapImpl(ch uint32) (uint32, bool) {
	var gid uint32
	var ok bool
	if c.format == 4 {
		gid, ok = c.cmap4MapCodepoint(ch)
	} else {
		gid, ok = c.cmap1213MapCodepoint(ch)
	}
	if !ok || gid == 0 {
		return 0, false
	}
	return gid, true
}

// cmap4MapCodepoint is Cmap4::map_codepoint.
func (c *ahCharmap) cmap4MapCodepoint(codepoint uint32) (uint32, bool) {
	if codepoint > 0xFFFF {
		return 0, false
	}
	cp := uint16(codepoint)
	lo, hi := 0, len(c.endCode)/2
	// The length of the binary search is segCountX2 / 2 in read-fonts. The
	// arrays have that length, or they are empty.
	for lo < hi {
		i := (lo + hi) / 2
		if 2*i+2 > len(c.startCode) {
			return 0, false
		}
		startCode := c.startCode.u16(2 * i)
		if cp < startCode {
			hi = i
		} else if cp > c.endCode.u16(2*i) {
			lo = i + 1
		} else {
			return c.cmap4LookupGlyphID(cp, i, startCode)
		}
	}
	return 0, false
}

// cmap4LookupGlyphID is Cmap4::lookup_glyph_id.
func (c *ahCharmap) cmap4LookupGlyphID(codepoint uint16, index int, startCode uint16) (uint32, bool) {
	if 2*index+2 > len(c.idDelta) || 2*index+2 > len(c.idRangeOffsets) {
		return 0, false
	}
	delta := int32(c.idDelta.i16(2 * index))
	rangeOffset := int(c.idRangeOffsets.u16(2 * index))
	if rangeOffset == 0 {
		return uint32(uint16(int32(codepoint) + delta)), true
	}
	offset := rangeOffset/2 + int(uint16(codepoint-startCode))
	offset -= len(c.idRangeOffsets)/2 - index
	if offset < 0 {
		offset = 0
	}
	if 2*offset+2 > len(c.glyphIDArray) {
		return 0, false
	}
	gid := c.glyphIDArray.u16(2 * offset)
	if gid == 0 {
		return 0, false
	}
	return uint32(uint16(int32(gid) + delta)), true
}

// cmap1213MapCodepoint is cmap1213_map_codepoint.
func (c *ahCharmap) cmap1213MapCodepoint(codepoint uint32) (uint32, bool) {
	lo, hi := 0, len(c.groups)/12
	for lo < hi {
		i := (lo + hi) / 2
		start, end, ref := c.groups.u32(12*i), c.groups.u32(12*i+4), c.groups.u32(12*i+8)
		if codepoint < start {
			hi = i
		} else if codepoint > end {
			lo = i + 1
		} else {
			if c.format == 13 {
				return ref, true
			}
			return ref + (codepoint - start), true
		}
	}
	return 0, false
}

// mappings is Charmap::mappings: it calls fn for every character of the
// subtable that has a glyph, in the order of the subtable.
func (c *ahCharmap) mappings(fn func(ch, gid uint32)) {
	emit := func(ch, gid uint32) {
		if gid != 0 {
			fn(ch, gid)
		}
	}
	switch c.format {
	case 4:
		c.cmap4Iter(emit)
	case 12, 13:
		c.cmap1213Iter(emit)
	}
}

// cmap4Iter is Cmap4Iter. A range that starts before the end of the range
// before it is cut, so that no character comes twice.
func (c *ahCharmap) cmap4Iter(fn func(ch, gid uint32)) {
	codeRange := func(index int) (start, end uint32, ok bool) {
		if 2*index+2 > len(c.startCode) || 2*index+2 > len(c.endCode) {
			return 0, 0, false
		}
		return uint32(c.startCode.u16(2 * index)), uint32(c.endCode.u16(2*index)) + 1, true
	}
	curStart, curEnd, _ := codeRange(0)
	curStartCode := uint16(curStart)
	curRangeIx := 0
	for {
		for cp := curStart; cp < curEnd; cp++ {
			if gid, ok := c.cmap4LookupGlyphID(uint16(cp), curRangeIx, curStartCode); ok {
				fn(cp, gid)
			}
		}
		// The range is used up. Its end stays as the least start of the
		// next range.
		if curStart < curEnd {
			curStart = curEnd
		}
		curRangeIx++
		nextStart, nextEnd, ok := codeRange(curRangeIx)
		if !ok {
			return
		}
		curStartCode = uint16(nextStart)
		curStart, curEnd = max(nextStart, curEnd), max(nextEnd, curEnd)
	}
}

// cmap1213Iter is Cmap1213Iter with the limits of the font.
func (c *ahCharmap) cmap1213Iter(fn func(ch, gid uint32)) {
	constant := c.format == 13
	group := func(index int) (start, end uint64, startCode, ref uint32, ok bool) {
		if 12*index+12 > len(c.groups) {
			return 0, 0, 0, 0, false
		}
		startCode = c.groups.u32(12 * index)
		endCode := uint64(c.groups.u32(12*index+4)) + 1
		ref = c.groups.u32(12*index + 8)
		if constant {
			endCode = min(endCode, uint64(c.maxChar))
		} else {
			limit := uint64(0)
			if uint64(c.glyphCount) > uint64(ref) {
				limit = uint64(c.glyphCount) - uint64(ref)
			}
			limit += uint64(startCode)
			endCode = min(limit, min(endCode, uint64(c.maxChar)))
		}
		return uint64(startCode), endCode, startCode, ref, true
	}
	start, end, startCode, ref, ok := group(0)
	if !ok {
		return
	}
	ix := 0
	for {
		for cp := start; cp < end; cp++ {
			if constant {
				fn(uint32(cp), ref)
			} else {
				fn(uint32(cp), ref+(uint32(cp)-startCode))
			}
		}
		if start < end {
			start = end
		}
		ix++
		nextStart, nextEnd, nextStartCode, nextRef, ok := group(ix)
		if !ok {
			return
		}
		if nextStart < end {
			nextStart = end
		}
		nextEnd = max(nextEnd, end)
		start, end, startCode, ref = nextStart, nextEnd, nextStartCode, nextRef
	}
}
