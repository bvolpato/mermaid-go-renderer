// Ported from skrifa 0.46.2 (src/outline/glyf/mod.rs: FreeTypeScaler with
// variations and with the hinted mode, Outlines::compute_hinted_scale and
// Outlines::hdmx_width, src/outline/glyf/deltas.rs,
// src/outline/glyf/outline.rs: ScaledOutline, src/outline/mod.rs:
// OutlineGlyph::draw, src/outline/hint.rs: HintingInstance::draw,
// src/metrics.rs: GlyphMetrics::advance_width, src/variation.rs:
// AxisCollection::location, src/instance.rs: LocationRef and Size), from
// read-fonts 0.43.2 (src/tables/fvar.rs, src/tables/avar.rs,
// src/tables/gvar.rs, src/tables/variations.rs, src/tables/mvar.rs,
// src/tables/hvar.rs, src/tables/hdmx.rs, src/tables/glyf.rs:
// CompositeGlyph::count_and_instructions, src/font_data.rs: Cursor, and the
// generated readers of the tables) and from font-types 0.12.5
// (src/fixed.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported the scaler from FreeType 2.13.2
// (src/truetype/ttgload.c, src/truetype/ttgxvar.c, src/truetype/ttobjs.c),
// which has the FreeType License (LICENSE-FreeType.txt).

package font

import (
	"math"

	ot "github.com/go-text/typesetting/font/opentype"
)

// This file adds two things to the scaler of autohint_glyf.go.
//
// The first is a variable font: the normalized coordinates of an instance
// (fvar and avar), the deltas of the points of a glyph (gvar), and the
// deltas of the metrics of the font (MVAR and HVAR).
//
// The second is the hinted mode of the scaler (FreeTypeScaler::hinted). The
// interpreter of TrueType instructions is not ported. So the hinted mode is
// exact for a glyph without instructions, and an approximation for a glyph
// with instructions: the points do not move, and the phantom points are the
// points before the program of the glyph. hasInstructions says which glyphs
// that affects.
//
// The numbers are fixed-point numbers in 32 bit integers, as in skrifa:
// Fixed is 16.16, F26Dot6 is 26.6 and F2Dot14 is 2.14 in 16 bits. The
// arithmetic wraps on overflow, as a release build of skrifa does.

// ---- fixed-point numbers (font-types, src/fixed.rs) ----

// ahFixedToI32 is Fixed::to_i32: a 16.16 number rounded to a whole number.
func ahFixedToI32(a int32) int32 { return (a + 0x8000) >> 16 }

// ahFixedToF26Dot6 is Fixed::to_f26dot6.
func ahFixedToF26Dot6(a int32) int32 { return (a + 0x200) >> 10 }

// ahFixedToF2Dot14 is Fixed::to_f2dot14.
func ahFixedToF2Dot14(a int32) int16 { return int16((a + 2) >> 2) }

// ahF2Dot14ToFixed is F2Dot14::to_fixed.
func ahF2Dot14ToFixed(a int16) int32 { return int32(a) * 4 }

// ahF26Dot6ToI32 is F26Dot6::to_i32: a 26.6 number rounded to a whole
// number.
func ahF26Dot6ToI32(a int32) int32 { return (a + 32) >> 6 }

// ahSaturatingSub is saturating_sub of a 32 bit integer.
func ahSaturatingSub(a, b int32) int32 {
	d := int64(a) - int64(b)
	return int32(min(max(d, math.MinInt32), math.MaxInt32))
}

// ahF64ToI32 is the cast of an f64 to an i32 in Rust: it saturates, and a
// NaN gives 0.
func ahF64ToI32(v float64) int32 {
	switch {
	case v != v:
		return 0
	case v >= 2147483648:
		return math.MaxInt32
	case v <= -2147483648:
		return math.MinInt32
	}
	return int32(v)
}

// ahFloatFrac is the term that from_f64 of font-types adds before the cast:
// a half with the sign of the number.
func ahFloatFrac(x float64) float64 {
	if math.Signbit(x) {
		return -0.5
	}
	return 0.5
}

// ahFixedFromF64 is Fixed::from_f64: the nearest 16.16 number.
func ahFixedFromF64(x float64) int32 {
	return ahF64ToI32(float64(x*65536.0) + ahFloatFrac(x))
}

// ahF26Dot6FromF64 is F26Dot6::from_f64: the nearest 26.6 number.
func ahF26Dot6FromF64(x float64) int32 {
	return ahF64ToI32(float64(x*64.0) + ahFloatFrac(x))
}

// ---- Cursor (read-fonts, src/font_data.rs) ----

// ahCursor is Cursor of read-fonts: a position in font data. A read moves
// the position also when it fails.
type ahCursor struct {
	data ahData
	pos  int
}

// advanceBy is Cursor::advance_by. The position saturates.
func (c *ahCursor) advanceBy(n int) {
	if n > math.MaxInt-c.pos {
		c.pos = math.MaxInt
		return
	}
	c.pos += n
}

// readArray is Cursor::read_array for n items of size bytes.
func (c *ahCursor) readArray(n, size int) (ahData, bool) {
	length := n * size
	pos := c.pos
	c.advanceBy(length)
	if length > len(c.data) || pos > len(c.data)-length {
		return nil, false
	}
	return c.data[pos : pos+length], true
}

func (c *ahCursor) readU8() (uint8, bool) {
	b, ok := c.readArray(1, 1)
	if !ok {
		return 0, false
	}
	return b[0], true
}

func (c *ahCursor) readU16() (uint16, bool) {
	b, ok := c.readArray(1, 2)
	if !ok {
		return 0, false
	}
	return b.u16(0), true
}

func (c *ahCursor) readU32() (uint32, bool) {
	b, ok := c.readArray(1, 4)
	if !ok {
		return 0, false
	}
	return b.u32(0), true
}

// splitOff is FontData::split_off: the data from a position. It fails for a
// position after the data.
func (d ahData) splitOff(pos int) (ahData, bool) {
	if pos < 0 || pos > len(d) {
		return nil, false
	}
	return d[pos:], true
}

// resolveOffset is ResolveOffset::resolve for a table without a minimum
// size: the data at an offset. It fails for a null offset and for an offset
// after the data.
func (d ahData) resolveOffset(off uint32) (ahData, bool) {
	if off == 0 {
		return nil, false
	}
	if uint64(off) > uint64(len(d)) {
		return nil, false
	}
	return d[off:], true
}

// ---- normalized coordinates (fvar, avar) ----

// ahNormalizedCoords returns the normalized coordinates (F2Dot14 bits, one
// per axis of fvar) for user values by axis tag. Axes without a value get
// their default. The result is nil for a font without fvar.
//
// It is AxisCollection::location of skrifa with Fvar::user_to_normalized of
// read-fonts.
//
// The delta sets of an avar table of version 2 are not ported: such a font
// gets the coordinates of the segment maps only.
func ahNormalizedCoords(ld *ot.Loader, user map[uint32]float32) []int16 {
	fvar, ok := ahRawTable(ld, "fvar")
	if !ok || len(fvar) < 16 {
		return nil
	}
	var avar ahData
	if a, ok := ahRawTable(ld, "avar"); ok && len(a) >= 8 {
		avar = a
	}
	return ahUserToNormalized(fvar, avar, user)
}

// ahUserToNormalized is Fvar::user_to_normalized for a location that has
// one coordinate for each axis that the header of fvar counts. avar is nil
// for a font without the table.
func ahUserToNormalized(fvar, avar ahData, user map[uint32]float32) []int16 {
	// Location::new: one coordinate for each axis of the header.
	normalizedCoords := make([]int16, int(fvar.u16(8)))
	// Fvar::axes: the records of the axes. An array that does not fit in
	// the table reads as an empty array.
	var axes ahData
	if arrays, ok := fvar.resolveOffset(uint32(fvar.u16(4))); ok {
		axes = arrays.array(0, len(normalizedCoords), 20)
	}
	actualLen := min(len(axes)/20, len(normalizedCoords))
	fixedCoords := make([]int32, actualLen)
	// normalize_user_coords: a value is for every axis that has its tag.
	for i := range fixedCoords {
		axis := axes[20*i : 20*i+20]
		if value, ok := user[axis.u32(0)]; ok {
			fixedCoords[i] = ahAxisNormalize(axis, ahFixedFromF64(float64(value)))
		}
	}
	// apply_avar_mappings: the segment maps follow the header of avar, one
	// for each axis. The array ends at the first map that does not fit.
	if avar != nil {
		maps := avar[8:]
		for i := range fixedCoords {
			if len(maps) < 2 {
				break
			}
			itemLen := int(maps.u16(0))*4 + 2
			if itemLen > len(maps) {
				break
			}
			fixedCoords[i] = ahSegmentMapsApply(maps[2:itemLen], fixedCoords[i])
			maps = maps[itemLen:]
		}
	}
	// to_normalized_coords.
	for i, coord := range fixedCoords {
		normalizedCoords[i] = ahFixedToF2Dot14(coord)
	}
	return normalizedCoords
}

// ahAxisNormalize is VariationAxisRecord::normalize: the normalized 16.16
// coordinate of a user value, before avar. axis is the record of the axis.
func ahAxisNormalize(axis ahData, value int32) int32 {
	minValue := int32(axis.u32(4))
	defaultValue := int32(axis.u32(8))
	// The maximum is not less than the minimum.
	maxValue := max(int32(axis.u32(12)), minValue)
	value = min(max(value, minValue), maxValue)
	switch {
	case value < defaultValue:
		value = -ahFixedDiv(ahSaturatingSub(defaultValue, value), ahSaturatingSub(defaultValue, minValue))
	case value > defaultValue:
		value = ahFixedDiv(ahSaturatingSub(value, defaultValue), ahSaturatingSub(maxValue, defaultValue))
	default:
		value = 0
	}
	return min(max(value, -0x10000), 0x10000)
}

// ahSegmentMapsApply is SegmentMaps::apply: the piecewise linear map of an
// axis for a 16.16 coordinate. maps has the pairs of the map: the from and
// the to coordinate, both F2Dot14. The special cases are those of HarfBuzz.
func ahSegmentMapsApply(maps ahData, coord int32) int32 {
	n := len(maps) / 4
	from := func(i int) int32 { return ahF2Dot14ToFixed(maps.i16(4 * i)) }
	to := func(i int) int32 { return ahF2Dot14ToFixed(maps.i16(4*i + 2)) }
	if n < 2 {
		if n == 0 {
			return coord
		}
		// One pair: the coordinate moves by the delta of the pair.
		return coord - from(0) + to(0)
	}
	const neg1, pos1 = -0x10000, 0x10000
	start, end := 0, n
	// A second pair for -1 at the start, or for +1 at the end, is cut.
	if from(start) == neg1 && to(start) == neg1 && from(start+1) == neg1 {
		start++
	}
	if from(end-1) == pos1 && to(end-1) == pos1 && from(end-2) == pos1 {
		end--
	}
	// An exact match comes first. Several pairs can have the same from
	// coordinate.
	i := start
	for i < end {
		if coord == from(i) {
			break
		}
		i++
	}
	if i < end {
		j := i
		for j+1 < end && coord == from(j+1) {
			j++
		}
		// The pairs i to j match.
		if i == j {
			return to(i)
		}
		// With three matches the middle one counts.
		if i+2 == j {
			return to(i + 1)
		}
		// Else the pair that is closer to 0 on the from side.
		if coord < 0 {
			return to(j)
		}
		if coord > 0 {
			return to(i)
		}
		ti, tj := to(i), to(j)
		if ahAbs(ti) < ahAbs(tj) {
			return ti
		}
		return tj
	}
	// No exact match: the segment that has the coordinate.
	k := start
	for k < end {
		if coord < from(k) {
			break
		}
		k++
	}
	if k == start {
		return coord - from(start) + to(start)
	}
	if k == end {
		return coord - from(end-1) + to(end-1)
	}
	bf, bt := from(k-1), to(k-1)
	af, at := from(k), to(k)
	denom := af - bf
	return bt + ahFixedMulDiv(at-bt, coord-bf, denom)
}

// ahEffectiveCoords is LocationRef::effective_coords: nil when every
// coordinate is zero.
func ahEffectiveCoords(coords []int16) []int16 {
	for _, c := range coords {
		if c != 0 {
			return coords
		}
	}
	return nil
}

// ---- ItemVariationStore (read-fonts, src/tables/variations.rs) ----

// ahItemVariationStore is ItemVariationStore: the deltas of MVAR and HVAR.
type ahItemVariationStore struct {
	data ahData
}

// newAHItemVariationStore reads the store at data. It fails for data that
// is shorter than the fixed fields.
func newAHItemVariationStore(data ahData) (ahItemVariationStore, bool) {
	if len(data) < 8 {
		return ahItemVariationStore{}, false
	}
	return ahItemVariationStore{data}, true
}

// ahRegionArray is the array of the regions of a store
// (ComputedArray<VariationRegion>). A region has three F2Dot14 numbers for
// each axis: start, peak and end.
type ahRegionArray struct {
	data      ahData
	itemLen   int
	axisCount int
}

// get is ComputedArray::get: the axes of a region.
func (a ahRegionArray) get(idx int) (ahData, bool) {
	d, ok := a.data.splitOff(idx * a.itemLen)
	if !ok || 6*a.axisCount > len(d) {
		return nil, false
	}
	return d[:6*a.axisCount], true
}

// variationRegions is VariationRegionList::variation_regions of the region
// list of the store. A list whose regions do not fit in the data reads as
// the default array of read-fonts: every region of it has no axes.
func (s ahItemVariationStore) variationRegions() (ahRegionArray, bool) {
	list, ok := s.data.resolveOffset(s.data.u32(2))
	if !ok || len(list) < 4 {
		return ahRegionArray{}, false
	}
	axisCount, regionCount := int(list.u16(0)), int(list.u16(2))
	itemLen := 6 * axisCount
	end := 4 + regionCount*itemLen
	if end > len(list) {
		return ahRegionArray{}, true
	}
	return ahRegionArray{data: list[4:end], itemLen: itemLen, axisCount: axisCount}, true
}

// ahRegionComputeScalar is VariationRegion::compute_scalar: the scalar of a
// region at normalized coordinates, a 16.16 number.
func ahRegionComputeScalar(regionAxes ahData, coords []int16) int32 {
	scalar := int32(0x10000)
	for i := 0; 6*i+6 <= len(regionAxes); i++ {
		peakCoord := regionAxes.i16(6*i + 2)
		// An axis with the peak 0 does not count.
		if peakCoord == 0 {
			continue
		}
		peak := ahF2Dot14ToFixed(peakCoord)
		start := ahF2Dot14ToFixed(regionAxes.i16(6 * i))
		end := ahF2Dot14ToFixed(regionAxes.i16(6*i + 4))
		if start > peak || peak > end || start < 0 && end > 0 {
			continue
		}
		coord := int32(0)
		if i < len(coords) {
			coord = ahF2Dot14ToFixed(coords[i])
		}
		switch {
		case coord < start || coord > end:
			return 0
		case coord == peak:
			continue
		case coord < peak:
			scalar = ahFixedMulDiv(scalar, coord-start, peak-start)
		default:
			scalar = ahFixedMulDiv(scalar, end-coord, end-peak)
		}
	}
	return scalar
}

// computeDelta is ItemVariationStore::compute_delta: the delta of the item
// with an outer and an inner index at normalized coordinates, rounded to a
// whole number. ok is false for a store that is not valid.
func (s ahItemVariationStore) computeDelta(outer, inner uint16, coords []int16) (delta int32, ok bool) {
	if len(coords) == 0 || outer == 0xFFFF && inner == 0xFFFF {
		return 0, true
	}
	// ItemVariationStore::item_variation_data: an array of offsets. The
	// array is empty when it does not fit in the data. A null offset gives
	// no delta.
	offsets := s.data.array(8, int(s.data.u16(6)), 4)
	if 4*int(outer)+4 > len(offsets) {
		return 0, false
	}
	offset := offsets.u32(4 * int(outer))
	if offset == 0 {
		return 0, true
	}
	data, ok := s.data.resolveOffset(offset)
	if !ok || len(data) < 6 {
		return 0, false
	}
	regions, ok := s.variationRegions()
	if !ok {
		return 0, false
	}
	// ItemVariationData: item_count, word_delta_count, region_index_count,
	// the indexes of the regions, and one row of deltas for each item.
	itemCount := int(data.u16(0))
	wordDeltaCount := data.u16(2)
	regionIndexCount := int(data.u16(4))
	regionIndexes := data.array(6, regionIndexCount, 2)
	// ItemVariationData::delta_row_len.
	longWords := wordDeltaCount&0x8000 != 0
	wordSize, smallSize := 2, 1
	if longWords {
		wordSize, smallSize = 4, 2
	}
	longDeltaCount := int(wordDeltaCount & 0x7FFF)
	shortDeltaCount := max(regionIndexCount-longDeltaCount, 0)
	bytesPerRow := longDeltaCount*wordSize + shortDeltaCount*smallSize
	// The rows read as empty when they do not all fit in the data.
	deltaSets := data.array(6+2*regionIndexCount, bytesPerRow*itemCount, 1)
	// ItemVariationData::delta_set: the row of the item. It ends at the
	// first delta that is not in the data.
	row, _ := deltaSets.splitOff(bytesPerRow * int(inner))
	cursor := ahCursor{data: row}
	// The sum has 64 bits, as in FreeType.
	accum := int64(0)
	for i := 0; i < regionIndexCount; i++ {
		var regionDelta int32
		switch {
		case (i >= longDeltaCount) == longWords:
			v, ok := cursor.readU16()
			if !ok {
				return ahItemDeltaRound(accum), true
			}
			regionDelta = int32(int16(v))
		case i >= longDeltaCount:
			v, ok := cursor.readU8()
			if !ok {
				return ahItemDeltaRound(accum), true
			}
			regionDelta = int32(int8(v))
		default:
			v, ok := cursor.readU32()
			if !ok {
				return ahItemDeltaRound(accum), true
			}
			regionDelta = int32(v)
		}
		if 2*i+2 > len(regionIndexes) {
			return 0, false
		}
		region, ok := regions.get(int(regionIndexes.u16(2 * i)))
		if !ok {
			return 0, false
		}
		scalar := ahRegionComputeScalar(region, coords)
		accum += int64(regionDelta) * int64(scalar)
	}
	return ahItemDeltaRound(accum), true
}

// ahItemDeltaRound is the end of compute_delta: the 48.16 sum as a whole
// number.
func ahItemDeltaRound(accum int64) int32 { return int32((accum + 0x8000) >> 16) }

// ahDeltaSetIndexMapGet is DeltaSetIndexMap::get: the outer and the inner
// index of the delta set of an entry of the map at data.
func ahDeltaSetIndexMapGet(data ahData, index uint32) (outer, inner uint16, ok bool) {
	if len(data) < 1 {
		return 0, 0, false
	}
	var mapCount uint32
	var mapStart int
	switch data[0] {
	case 0:
		if len(data) < 4 {
			return 0, 0, false
		}
		mapCount, mapStart = uint32(data.u16(2)), 4
	case 1:
		if len(data) < 6 {
			return 0, 0, false
		}
		mapCount, mapStart = data.u32(2), 6
	default:
		return 0, 0, false
	}
	if mapCount == 0 {
		return uint16(index >> 16), uint16(index), true
	}
	// EntryFormat: the size of an entry and the number of bits of the
	// inner index.
	entryFormat := data[1] & 0x3F
	entrySize := int((entryFormat&0x30)>>4) + 1
	// The map reads as empty when it does not fit in the data.
	mapEnd := uint64(mapStart) + uint64(entrySize)*uint64(mapCount)
	var mapData ahData
	if mapEnd <= uint64(len(data)) {
		mapData = data[mapStart:mapEnd]
	}
	// An index after the map uses the last entry.
	index = min(index, mapCount-1)
	offset := uint64(index) * uint64(entrySize)
	if offset+uint64(entrySize) > uint64(len(mapData)) {
		return 0, 0, false
	}
	var entry uint32
	for _, b := range mapData[offset : offset+uint64(entrySize)] {
		entry = entry<<8 | uint32(b)
	}
	bitCount := uint32(entryFormat&0x0F) + 1
	return uint16(entry >> bitCount), uint16(entry & (1<<bitCount - 1)), true
}

// ---- MVAR (read-fonts, src/tables/mvar.rs) ----

// ahMvar is Mvar: the deltas of the metrics of the font.
type ahMvar struct {
	data ahData
}

// metricDelta is Mvar::metric_delta: the delta of the metric with a tag at
// normalized coordinates, as Fixed 16.16 bits. ok is false for a tag that
// the table does not have, and for a table that is not valid.
func (m *ahMvar) metricDelta(tag uint32, coords []int16) (delta int32, ok bool) {
	// The records read as empty when they do not fit in the table.
	records := m.data.array(12, int(m.data.u16(8)), 8)
	lo, hi := 0, len(records)/8
	for lo < hi {
		i := (lo + hi) / 2
		recordTag := records.u32(8 * i)
		switch {
		case tag < recordTag:
			hi = i
		case tag > recordTag:
			lo = i + 1
		default:
			storeData, ok := m.data.resolveOffset(uint32(m.data.u16(10)))
			if !ok {
				return 0, false
			}
			store, ok := newAHItemVariationStore(storeData)
			if !ok {
				return 0, false
			}
			d, ok := store.computeDelta(records.u16(8*i+4), records.u16(8*i+6), coords)
			if !ok {
				return 0, false
			}
			return d << 16, true
		}
	}
	return 0, false
}

// ---- HVAR (read-fonts, src/tables/hvar.rs) ----

// ahHvar is Hvar: the deltas of the advances of the glyphs.
type ahHvar struct {
	data ahData
}

// advanceWidthDelta is Hvar::advance_width_delta (advance_delta of
// variations.rs): the delta of the advance of a glyph at normalized
// coordinates, as Fixed 16.16 bits.
func (h *ahHvar) advanceWidthDelta(gid uint32, coords []int16) (delta int32, ok bool) {
	if len(coords) == 0 {
		return 0, true
	}
	// Without a map that reads, the glyph is the inner index.
	outer, inner := uint16(0), uint16(gid)
	if off := h.data.u32(8); off != 0 {
		if mapData, ok := h.data.resolveOffset(off); ok && ahDeltaSetIndexMapReads(mapData) {
			if outer, inner, ok = ahDeltaSetIndexMapGet(mapData, gid); !ok {
				return 0, false
			}
		}
	}
	storeData, ok := h.data.resolveOffset(h.data.u32(4))
	if !ok {
		return 0, false
	}
	store, ok := newAHItemVariationStore(storeData)
	if !ok {
		return 0, false
	}
	d, ok := store.computeDelta(outer, inner, coords)
	if !ok {
		return 0, false
	}
	return d << 16, true
}

// ahDeltaSetIndexMapReads reports whether DeltaSetIndexMap::read accepts
// the data: it has a format that is known and the fixed fields of the
// format.
func ahDeltaSetIndexMapReads(data ahData) bool {
	if len(data) < 1 {
		return false
	}
	switch data[0] {
	case 0:
		return len(data) >= 4
	case 1:
		return len(data) >= 6
	}
	return false
}

// ---- hdmx (read-fonts, src/tables/hdmx.rs) ----

// ahHdmx is Hdmx: the widths of the glyphs in whole pixels for some font
// sizes.
type ahHdmx struct {
	data      ahData
	numGlyphs int
}

// width is Outlines::hdmx_width: the width of a glyph for a font size that
// is a whole number and that the table has.
func (h *ahHdmx) width(ppem float32, gid uint32) (uint8, bool) {
	if h == nil {
		return 0, false
	}
	// The cast of an f32 to an u8 in Rust saturates.
	var ppemU8 uint8
	switch {
	case ppem != ppem || ppem <= 0:
		ppemU8 = 0
	case ppem >= 255:
		ppemU8 = 255
	default:
		ppemU8 = uint8(ppem)
	}
	if float32(ppemU8) != ppem {
		return 0, false
	}
	// Hdmx::record_for_size: a binary search in the records. The records
	// read as the default array of read-fonts when they do not fit in the
	// table: an array without items.
	sizeDeviceRecord := uint64(h.data.u32(4))
	numRecords := uint64(h.data.u16(2))
	if sizeDeviceRecord == 0 || 8+numRecords*sizeDeviceRecord > uint64(len(h.data)) {
		return 0, false
	}
	records := h.data[8 : 8+numRecords*sizeDeviceRecord]
	lo, hi := uint64(0), numRecords
	for lo < hi {
		mid := (lo + hi) / 2
		// DeviceRecord::read: the pixel size, the largest width, and one
		// width for each glyph.
		record := records[mid*sizeDeviceRecord:]
		if 2+h.numGlyphs > len(record) {
			return 0, false
		}
		switch pixelSize := record[0]; {
		case pixelSize < ppemU8:
			lo = mid + 1
		case pixelSize > ppemU8:
			hi = mid
		default:
			if gid >= uint32(h.numGlyphs) {
				return 0, false
			}
			return record[2+gid], true
		}
	}
	return 0, false
}

// ---- gvar (read-fonts, src/tables/gvar.rs and src/tables/variations.rs) ----

// The flags of the tuple index and of the tuple count (TupleIndex and
// TupleVariationCount), and the flags of a run of packed deltas.
const (
	ahTupleEmbeddedPeak        = 0x8000
	ahTupleIntermediateRegion  = 0x4000
	ahTuplePrivatePointNumbers = 0x2000
	ahTupleIndexMask           = 0x0FFF
	ahTupleSharedPointNumbers  = 0x8000
	ahTupleCountMask           = 0x0FFF
	ahDeltasAreZero            = 0x80
	ahDeltasAreWords           = 0x40
	ahDeltaRunCountMask        = 0x3F
)

// ahMarkerHasDelta is PointMarker::HAS_DELTA: a point that a tuple gives a
// delta.
const ahMarkerHasDelta uint8 = 0x04

// ahGvar is Gvar: the deltas of the points of the glyphs.
type ahGvar struct {
	data      ahData
	axisCount int
}

// The results of glyphVariationData.
const (
	// ahVarDataErr: the data of the glyph is not valid.
	ahVarDataErr = iota
	// ahVarDataNone: the glyph has no variation data.
	ahVarDataNone
	// ahVarDataSome: the glyph has variation data.
	ahVarDataSome
)

// ahTupleArray is an array of tuples (ComputedArray<Tuple>): the shared
// tuples of gvar. A tuple has one F2Dot14 number for each axis.
type ahTupleArray struct {
	data      ahData
	itemLen   int
	axisCount int
}

// get is ComputedArray::get: the values of a tuple.
func (a ahTupleArray) get(idx int) (ahData, bool) {
	d, ok := a.data.splitOff(idx * a.itemLen)
	if !ok || 2*a.axisCount > len(d) {
		return nil, false
	}
	return d[:2*a.axisCount], true
}

// ahTupleVariationData is TupleVariationData: the tuples of one glyph.
type ahTupleVariationData struct {
	axisCount    int
	sharedTuples ahTupleArray
	// sharedPointNumbers has the packed point numbers that the tuples
	// share.
	sharedPointNumbers    ahPackedPointNumbers
	hasSharedPointNumbers bool
	tupleCount            uint16
	// headerData has the headers of the tuples, and serializedData has the
	// data of the tuples.
	headerData     ahData
	serializedData ahData
}

// glyphVariationData is Gvar::glyph_variation_data: the tuples of a glyph.
func (g *ahGvar) glyphVariationData(gid uint32) (ahTupleVariationData, int) {
	var d ahTupleVariationData
	// Gvar::shared_tuples.
	sharedData, ok := g.data.resolveOffset(g.data.u32(8))
	if !ok {
		return d, ahVarDataErr
	}
	// Gvar::data_for_gid: the offsets of the data of the glyphs are 16 bit
	// numbers of two bytes, or 32 bit numbers.
	long := g.data.u16(14)&1 != 0
	offsetSize := 2
	if long {
		offsetSize = 4
	}
	offsets := g.data.array(20, int(g.data.u16(12))+1, offsetSize)
	offsetAt := func(idx uint64) (uint32, bool) {
		if (idx+1)*uint64(offsetSize) > uint64(len(offsets)) {
			return 0, false
		}
		if long {
			return offsets.u32(int(idx) * 4), true
		}
		return uint32(offsets.u16(int(idx)*2)) * 2, true
	}
	dataStart := uint64(g.data.u32(16))
	startOff, ok := offsetAt(uint64(gid))
	if !ok {
		return d, ahVarDataErr
	}
	endOff, ok := offsetAt(uint64(gid) + 1)
	if !ok {
		return d, ahVarDataErr
	}
	start, end := dataStart+uint64(startOff), dataStart+uint64(endOff)
	if start > math.MaxUint32 || end > math.MaxUint32 {
		return d, ahVarDataErr
	}
	if start >= end {
		return d, ahVarDataNone
	}
	if end > uint64(len(g.data)) {
		return d, ahVarDataErr
	}
	data := g.data[start:end]
	// GlyphVariationData::new: the header has the count of the tuples and
	// the offset of the serialized data.
	if len(data) < 4 {
		return d, ahVarDataErr
	}
	d.headerData = data[4:]
	d.tupleCount = data.u16(0)
	serialized, ok := data.resolveOffset(uint32(data.u16(2)))
	if !ok {
		return d, ahVarDataErr
	}
	if d.tupleCount&ahTupleSharedPointNumbers != 0 {
		d.sharedPointNumbers, serialized = ahSplitOffFront(serialized)
		d.hasSharedPointNumbers = true
	}
	d.serializedData = serialized
	d.axisCount = g.axisCount
	// SharedTuples::tuples: the tuples read as the default array of
	// read-fonts when they do not fit in the table. A tuple of that array
	// has no values.
	sharedTupleCount := int(g.data.u16(6))
	if end := sharedTupleCount * 2 * g.axisCount; end <= len(sharedData) {
		d.sharedTuples = ahTupleArray{data: sharedData[:end], itemLen: 2 * g.axisCount, axisCount: g.axisCount}
	}
	return d, ahVarDataSome
}

// ahTupleVariationHeader is TupleVariationHeader: the header of a tuple.
type ahTupleVariationHeader struct {
	data      ahData
	axisCount int
}

func (h ahTupleVariationHeader) variationDataSize() int { return int(h.data.u16(0)) }

func (h ahTupleVariationHeader) tupleIndex() uint16 { return h.data.u16(2) }

// byteLen is TupleVariationHeader::byte_len.
func (h ahTupleVariationHeader) byteLen() int {
	n := 4
	index := h.tupleIndex()
	if index&ahTupleEmbeddedPeak != 0 {
		n += 2 * h.axisCount
	}
	if index&ahTupleIntermediateRegion != 0 {
		n += 4 * h.axisCount
	}
	return n
}

// peakTuple is TupleVariationHeader::peak_tuple: the peak tuple in the
// header.
func (h ahTupleVariationHeader) peakTuple() (ahData, bool) {
	if h.tupleIndex()&ahTupleEmbeddedPeak == 0 {
		return nil, false
	}
	return h.data[4 : 4+2*h.axisCount], true
}

// intermediateTuples is TupleVariationHeader::intermediate_tuples: the start
// and the end tuple of an intermediate region.
func (h ahTupleVariationHeader) intermediateTuples() (start, end ahData, ok bool) {
	index := h.tupleIndex()
	if index&ahTupleIntermediateRegion == 0 {
		return nil, nil, false
	}
	p := 4
	if index&ahTupleEmbeddedPeak != 0 {
		p += 2 * h.axisCount
	}
	return h.data[p : p+2*h.axisCount], h.data[p+2*h.axisCount : p+4*h.axisCount], true
}

// ahComputeScalar is compute_scalar of variations.rs: the 16.16 scalar of a
// tuple at normalized coordinates. ok is false for a tuple that does not
// apply at the coordinates.
func ahComputeScalar(header ahTupleVariationHeader, axisCount int, sharedTuples ahTupleArray, coords []int16) (int32, bool) {
	scalar := int32(0x10000)
	var peak ahData
	var ok bool
	if tupleIdx := header.tupleIndex(); tupleIdx&ahTupleEmbeddedPeak == 0 {
		peak, ok = sharedTuples.get(int(tupleIdx & ahTupleIndexMask))
	} else {
		peak, ok = header.peakTuple()
	}
	if !ok || len(peak)/2 != axisCount {
		return 0, false
	}
	interStart, interEnd, hasIntermediate := header.intermediateTuples()
	for i := 0; i < axisCount; i++ {
		peakCoord := peak.i16(2 * i)
		if peakCoord == 0 {
			continue
		}
		var coord int16
		if i < len(coords) {
			coord = coords[i]
		}
		if coord == 0 {
			return 0, false
		}
		if peakCoord == coord {
			continue
		}
		if hasIntermediate {
			start, end := interStart.i16(2*i), interEnd.i16(2*i)
			if coord <= start || coord >= end {
				return 0, false
			}
			c, p := ahF2Dot14ToFixed(coord), ahF2Dot14ToFixed(peakCoord)
			if c < p {
				s := ahF2Dot14ToFixed(start)
				scalar = ahFixedMulDiv(scalar, c-s, p-s)
			} else {
				e := ahF2Dot14ToFixed(end)
				scalar = ahFixedMulDiv(scalar, e-c, e-p)
			}
		} else {
			if coord < min(peakCoord, 0) || coord > max(peakCoord, 0) {
				return 0, false
			}
			scalar = ahFixedMulDiv(scalar, ahF2Dot14ToFixed(coord), ahF2Dot14ToFixed(peakCoord))
		}
	}
	return scalar, scalar != 0
}

// ahTupleVariation is TupleVariation: the data of one tuple of a glyph.
type ahTupleVariation struct {
	header                ahTupleVariationHeader
	serializedData        ahData
	sharedPointNumbers    ahPackedPointNumbers
	hasSharedPointNumbers bool
}

// activeTuplesAt is TupleVariationData::active_tuples_at: it calls fn for
// each tuple that applies at the coordinates, with the scalar of the tuple.
// The iteration ends when fn returns false.
func (d *ahTupleVariationData) activeTuplesAt(coords []int16, fn func(tuple *ahTupleVariation, scalar int32) bool) {
	headerData := d.headerData
	dataOffset := 0
	for current, n := 0, int(d.tupleCount&ahTupleCountMask); current < n; current++ {
		// TupleVariationHeaderIter::next: the header needs its fixed
		// fields, and the data must have all bytes of the header.
		header := ahTupleVariationHeader{data: headerData, axisCount: d.axisCount}
		headerOK := len(headerData) >= 4
		nextLen := 0
		if headerOK {
			nextLen = header.byteLen()
		}
		if nextLen > len(headerData) {
			return
		}
		headerData = headerData[nextLen:]
		if !headerOK {
			return
		}
		dataStart := dataOffset
		dataEnd := dataStart + header.variationDataSize()
		dataOffset = dataEnd
		scalar, ok := ahComputeScalar(header, d.axisCount, d.sharedTuples, coords)
		if !ok {
			continue
		}
		if dataEnd > len(d.serializedData) {
			return
		}
		tuple := &ahTupleVariation{
			header:                header,
			serializedData:        d.serializedData[dataStart:dataEnd],
			sharedPointNumbers:    d.sharedPointNumbers,
			hasSharedPointNumbers: d.hasSharedPointNumbers,
		}
		if !fn(tuple, scalar) {
			return
		}
	}
}

// hasDeltasForAllPoints is TupleVariation::has_deltas_for_all_points.
func (t *ahTupleVariation) hasDeltasForAllPoints() bool {
	if t.header.tupleIndex()&ahTuplePrivatePointNumbers != 0 {
		return ahPackedPointNumbers{t.serializedData}.count() == 0
	}
	if t.hasSharedPointNumbers {
		return t.sharedPointNumbers.count() == 0
	}
	return false
}

// pointNumbersAndPackedDeltas is
// TupleVariation::point_numbers_and_packed_deltas. Without point numbers of
// its own and without shared point numbers, a tuple has the default point
// numbers of read-fonts: all points.
func (t *ahTupleVariation) pointNumbersAndPackedDeltas() (ahPackedPointNumbers, ahData) {
	if t.header.tupleIndex()&ahTuplePrivatePointNumbers != 0 {
		return ahSplitOffFront(t.serializedData)
	}
	return t.sharedPointNumbers, t.serializedData
}

// ahPackedPointNumbers is PackedPointNumbers: the numbers of the points
// that a tuple has deltas for.
type ahPackedPointNumbers struct {
	data ahData
}

// ahSplitOffFront is PackedPointNumbers::split_off_front: the point numbers
// at the start of data, and the data after them.
func ahSplitOffFront(data ahData) (ahPackedPointNumbers, ahData) {
	p := ahPackedPointNumbers{data}
	remainder, _ := data.splitOff(p.totalLen())
	return p, remainder
}

// count is PackedPointNumbers::count. The count 0 says that the tuple has
// deltas for all points.
func (p ahPackedPointNumbers) count() uint16 {
	n, _ := p.countAndCountBytes()
	return n
}

// countAndCountBytes is PackedPointNumbers::count_and_count_bytes: the
// count, and the number of bytes of the count.
func (p ahPackedPointNumbers) countAndCountBytes() (uint16, int) {
	var first uint8
	if len(p.data) > 0 {
		first = p.data[0]
	}
	switch {
	case first == 0:
		return 0, 1
	case first < 128:
		return uint16(first), 1
	}
	// With the high bit, the count has two bytes.
	return p.data.u16(0) & 0x7FFF, 2
}

// totalLen is PackedPointNumbers::total_len: the number of bytes of the
// point numbers.
func (p ahPackedPointNumbers) totalLen() int {
	nPoints, nBytes := p.countAndCountBytes()
	if nPoints == 0 {
		return nBytes
	}
	cursor := ahCursor{data: p.data}
	cursor.advanceBy(nBytes)
	nSeen := uint16(0)
	for nSeen < nPoints {
		count, twoBytes, ok := ahReadControlByte(&cursor)
		if !ok {
			return nBytes
		}
		wordSize := 1
		if twoBytes {
			wordSize = 2
		}
		runSize := wordSize * int(count)
		nBytes += runSize + 1
		cursor.advanceBy(runSize)
		nSeen += uint16(count)
	}
	return nBytes
}

// ahReadControlByte is read_control_byte: the count of a run of point
// numbers, and whether a number of the run has two bytes.
func ahReadControlByte(cursor *ahCursor) (count uint8, twoBytes, ok bool) {
	control, ok := cursor.readU8()
	if !ok {
		return 0, false, false
	}
	return control&0x7F + 1, control&0x80 != 0, true
}

// ahPackedPointNumbersIter is PackedPointNumbersIter.
type ahPackedPointNumbersIter struct {
	count, seen, lastVal uint16
	// The state of PointRunIter.
	remaining uint8
	twoBytes  bool
	cursor    ahCursor
}

// iter is PackedPointNumbers::iter.
func (p ahPackedPointNumbers) iter() *ahPackedPointNumbersIter {
	count, nBytes := p.countAndCountBytes()
	it := &ahPackedPointNumbersIter{count: count, cursor: ahCursor{data: p.data}}
	it.cursor.advanceBy(nBytes)
	return it
}

// next is the next point number. With the count 0 the numbers are 0, 1, 2
// and so on.
func (it *ahPackedPointNumbersIter) next() (uint16, bool) {
	if it.count == 0 {
		result := it.lastVal
		if it.lastVal == math.MaxUint16 {
			return 0, false
		}
		it.lastVal++
		return result, true
	}
	if it.count == it.seen {
		return 0, false
	}
	it.seen++
	// PointRunIter::next: a number is the difference to the number before
	// it.
	for it.remaining == 0 {
		var ok bool
		if it.remaining, it.twoBytes, ok = ahReadControlByte(&it.cursor); !ok {
			return 0, false
		}
	}
	it.remaining--
	var delta uint16
	if it.twoBytes {
		v, ok := it.cursor.readU16()
		if !ok {
			return 0, false
		}
		delta = v
	} else {
		v, ok := it.cursor.readU8()
		if !ok {
			return 0, false
		}
		delta = uint16(v)
	}
	sum := uint32(it.lastVal) + uint32(delta)
	if sum > math.MaxUint16 {
		return 0, false
	}
	it.lastVal = uint16(sum)
	return it.lastVal, true
}

// ahDeltaRunSize is DeltaRunType::new: the number of bytes of a delta of a
// run with a control byte. It is 0 for a run of zeros.
func ahDeltaRunSize(control uint8) int {
	areZero, areWords := control&ahDeltasAreZero != 0, control&ahDeltasAreWords != 0
	switch {
	case areZero && areWords:
		return 4
	case areZero:
		return 0
	case areWords:
		return 2
	}
	return 1
}

// ahDeltaRunIter is DeltaRunIter: the packed deltas of a tuple.
type ahDeltaRunIter struct {
	// limit is the number of deltas that are left. Without a limit the
	// deltas go to the end of the data.
	limit          int
	hasLimit       bool
	remainingInRun uint8
	valueSize      int
	cursor         ahCursor
}

func newAHDeltaRunIter(data ahData, limit int, hasLimit bool) ahDeltaRunIter {
	return ahDeltaRunIter{limit: limit, hasLimit: hasLimit, valueSize: 1, cursor: ahCursor{data: data}}
}

// readNextControl is DeltaRunIter::read_next_control.
func (it *ahDeltaRunIter) readNextControl() bool {
	it.remainingInRun = 0
	control, ok := it.cursor.readU8()
	if !ok {
		return false
	}
	it.valueSize = ahDeltaRunSize(control)
	it.remainingInRun = control&ahDeltaRunCountMask + 1
	return true
}

// skipFast is DeltaRunIter::skip_fast: it steps over n deltas.
func (it *ahDeltaRunIter) skipFast(n int) {
	wanted := n
	remaining := int(it.remainingInRun)
	valueSize := it.valueSize
	for {
		if wanted > remaining {
			// The rest of this run, and then the next run.
			it.cursor.advanceBy(remaining * valueSize)
			wanted -= remaining
			if !it.readNextControl() {
				it.limit, it.hasLimit = 0, true
				return
			}
			remaining = int(it.remainingInRun)
			valueSize = it.valueSize
			continue
		}
		consumed := wanted
		it.remainingInRun -= uint8(consumed)
		it.cursor.advanceBy(consumed * valueSize)
		if it.hasLimit {
			it.limit = max(it.limit-n, 0)
		}
		return
	}
}

// next is the next delta.
func (it *ahDeltaRunIter) next() (int32, bool) {
	if it.hasLimit {
		if it.limit == 0 {
			return 0, false
		}
		it.limit--
	}
	if it.remainingInRun == 0 {
		if !it.readNextControl() {
			return 0, false
		}
	}
	it.remainingInRun--
	switch it.valueSize {
	case 0:
		return 0, true
	case 1:
		v, ok := it.cursor.readU8()
		return int32(int8(v)), ok
	case 2:
		v, ok := it.cursor.readU16()
		return int32(int16(v)), ok
	}
	v, ok := it.cursor.readU32()
	return int32(v), ok
}

// ahCountAllDeltas is count_all_deltas: the number of deltas in data that
// is packed deltas to its end.
func ahCountAllDeltas(data ahData) int {
	count, offset := 0, 0
	for offset < len(data) {
		control := data[offset]
		runCount := int(control&ahDeltaRunCountMask) + 1
		count += runCount
		offset += runCount*ahDeltaRunSize(control) + 1
	}
	return count
}

// ahTupleDeltaIter is TupleDeltaIter for the deltas of points: the deltas
// that a tuple has in its data. It does not infer deltas.
type ahTupleDeltaIter struct {
	cur int
	// points is nil when every point has a delta.
	points    *ahPackedPointNumbersIter
	nextPoint int
	x, y      ahDeltaRunIter
}

// deltas is TupleVariation::deltas.
func (t *ahTupleVariation) deltas() *ahTupleDeltaIter {
	pointNumbers, packedDeltas := t.pointNumbersAndPackedDeltas()
	// PackedDeltas::x_deltas and y_deltas: the first half of the deltas is
	// for x and the second half for y. Without a count of points the
	// deltas go to the end of the data.
	count := 2 * int(pointNumbers.count())
	if count == 0 {
		count = ahCountAllDeltas(packedDeltas)
	}
	it := &ahTupleDeltaIter{
		x: newAHDeltaRunIter(packedDeltas, count/2, true),
		y: newAHDeltaRunIter(packedDeltas, count, true),
	}
	it.y.skipFast(count / 2)
	// TupleDeltaIter::new.
	points := pointNumbers.iter()
	if nextPoint, ok := points.next(); ok {
		it.points = points
		it.nextPoint = int(nextPoint)
	}
	return it
}

// next is the next delta: the number of its point, and the delta of x and
// of y.
func (it *ahTupleDeltaIter) next() (position uint16, dx, dy int32, ok bool) {
	for {
		pos := it.cur
		if it.points != nil {
			// Only some points have deltas.
			if it.cur > it.nextPoint {
				nextPoint, ok := it.points.next()
				if !ok {
					return 0, 0, 0, false
				}
				it.nextPoint = int(nextPoint)
			}
			pos = it.nextPoint
		}
		if pos == it.cur {
			if dx, ok = it.x.next(); !ok {
				return 0, 0, 0, false
			}
			if dy, ok = it.y.next(); !ok {
				return 0, 0, 0, false
			}
			it.cur++
			return uint16(pos), dx, dy, true
		}
		it.cur++
	}
}

// ahApplyScalar is GlyphDelta::apply_scalar for 16.16 deltas: a delta of a
// tuple, as whole font units, times the scalar of the tuple.
func ahApplyScalar(dx, dy, scalar int32) ahFPoint {
	return ahFPoint{ahFixedMul(dx<<16, scalar), ahFixedMul(dy<<16, scalar)}
}

// ahScaledDelta is the delta that accumulate_dense_deltas and
// accumulate_sparse_deltas add for a delta of a tuple: the delta as a
// 16.16 number, times the scalar. skrifa does not multiply for the scalar
// 1.
func ahScaledDelta(delta, scalar int32) int32 {
	if scalar == 0x10000 {
		return delta << 16
	}
	return ahFixedMul(delta<<16, scalar)
}

// ahPackedDelta returns the delta k of the values of a run of packed deltas
// whose deltas have valueSize bytes.
func ahPackedDelta(packed ahData, valueSize, k int) int32 {
	switch valueSize {
	case 1:
		return int32(int8(packed[k]))
	case 2:
		return int32(packed.i16(2 * k))
	}
	return int32(packed.u32(4 * k))
}

// ahReadDenseDeltas is read_dense_deltas: the deltas of one coordinate of n
// points. fn gets the number of a point and its delta. A run of zeros has
// no values, and fn does not get its points. The result is false for data
// that is not valid.
func ahReadDenseDeltas(cursor *ahCursor, n int, fn func(i int, delta int32)) bool {
	cur := 0
	for cur < n {
		control, ok := cursor.readU8()
		if !ok {
			return false
		}
		valueSize := ahDeltaRunSize(control)
		runCount := int(control&ahDeltaRunCountMask) + 1
		// The run must be inside the points.
		if cur+runCount > n {
			return false
		}
		if valueSize != 0 {
			packed, ok := cursor.readArray(runCount, valueSize)
			if !ok {
				return false
			}
			for k := 0; k < runCount; k++ {
				fn(cur+k, ahPackedDelta(packed, valueSize, k))
			}
		}
		cur += runCount
	}
	return true
}

// ahReadSparseDeltas is read_sparse_deltas: the deltas of one coordinate of
// the count points of the point numbers. fn gets the number of a point and
// its delta. The result is false for data that is not valid.
func ahReadSparseDeltas(cursor *ahCursor, pointNumbers ahPackedPointNumbers, count int, fn func(ix int, delta int32)) bool {
	cur := 0
	points := pointNumbers.iter()
	for cur < count {
		control, ok := cursor.readU8()
		if !ok {
			return false
		}
		valueSize := ahDeltaRunSize(control)
		runCount := int(control&ahDeltaRunCountMask) + 1
		if valueSize == 0 {
			// A run of zeros must have a point number for each delta.
			for k := 0; k < runCount; k++ {
				pointIx, ok := points.next()
				if !ok {
					return false
				}
				fn(int(pointIx), 0)
			}
		} else {
			packed, ok := cursor.readArray(runCount, valueSize)
			if !ok {
				return false
			}
			// A run with values can have more deltas than there are
			// point numbers left. The deltas without a point are not
			// used.
			for k := 0; k < runCount; k++ {
				pointIx, ok := points.next()
				if !ok {
					break
				}
				fn(int(pointIx), ahPackedDelta(packed, valueSize, k))
			}
		}
		cur += runCount
	}
	return true
}

// accumulateDenseDeltas is TupleVariation::accumulate_dense_deltas: it adds
// the deltas of a tuple that has deltas for all points to deltas, which are
// 16.16 numbers.
func (t *ahTupleVariation) accumulateDenseDeltas(deltas []ahFPoint, scalar int32) bool {
	_, packedDeltas := t.pointNumbersAndPackedDeltas()
	cursor := ahCursor{data: packedDeltas}
	if !ahReadDenseDeltas(&cursor, len(deltas), func(i int, delta int32) {
		deltas[i].x += ahScaledDelta(delta, scalar)
	}) {
		return false
	}
	return ahReadDenseDeltas(&cursor, len(deltas), func(i int, delta int32) {
		deltas[i].y += ahScaledDelta(delta, scalar)
	})
}

// accumulateSparseDeltas is TupleVariation::accumulate_sparse_deltas: it
// adds the deltas of a tuple that has deltas for some points to deltas,
// and it marks those points in flags.
func (t *ahTupleVariation) accumulateSparseDeltas(deltas []ahFPoint, flags []uint8, scalar int32) bool {
	pointNumbers, packedDeltas := t.pointNumbersAndPackedDeltas()
	cursor := ahCursor{data: packedDeltas}
	count := int(pointNumbers.count())
	if !ahReadSparseDeltas(&cursor, pointNumbers, count, func(ix int, delta int32) {
		if ix < len(deltas) && ix < len(flags) {
			deltas[ix].x += ahScaledDelta(delta, scalar)
			flags[ix] |= ahMarkerHasDelta
		}
	}) {
		return false
	}
	return ahReadSparseDeltas(&cursor, pointNumbers, count, func(ix int, delta int32) {
		if ix < len(deltas) {
			deltas[ix].y += ahScaledDelta(delta, scalar)
		}
	})
}

// ahComponentGlyphIDFlagsIter is ComponentGlyphIdFlagsIter of read-fonts:
// the glyph and the flags of each component of a composite glyph.
type ahComponentGlyphIDFlagsIter struct {
	curFlags uint16
	done     bool
	cursor   ahCursor
}

// newAHComponentGlyphIDFlagsIter starts at the components of the data of a
// composite glyph.
func newAHComponentGlyphIDFlagsIter(glyph ahData) *ahComponentGlyphIDFlagsIter {
	return &ahComponentGlyphIDFlagsIter{cursor: ahCursor{data: glyph.rest(10, 1)}}
}

func (it *ahComponentGlyphIDFlagsIter) next() (glyph, flags uint16, ok bool) {
	if it.done {
		return 0, 0, false
	}
	if flags, ok = it.cursor.readU16(); !ok {
		return 0, 0, false
	}
	it.curFlags = flags
	if glyph, ok = it.cursor.readU16(); !ok {
		return 0, 0, false
	}
	if flags&ahCompArg1And2AreWords != 0 {
		it.cursor.advanceBy(4)
	} else {
		it.cursor.advanceBy(2)
	}
	switch {
	case flags&ahCompWeHaveAScale != 0:
		it.cursor.advanceBy(2)
	case flags&ahCompWeHaveAnXAndYScale != 0:
		it.cursor.advanceBy(4)
	case flags&ahCompWeHaveATwoByTwo != 0:
		it.cursor.advanceBy(8)
	}
	it.done = flags&ahCompMoreComponents == 0
	return glyph, flags, true
}

// ahCountAndInstructions is CompositeGlyph::count_and_instructions: the
// number of components of a composite glyph, and the instructions that
// follow the components.
func ahCountAndInstructions(glyph ahData) (count int, instructions ahData) {
	it := newAHComponentGlyphIDFlagsIter(glyph)
	for {
		if _, _, ok := it.next(); !ok {
			break
		}
		count++
	}
	if it.curFlags&ahCompWeHaveInstructions != 0 {
		if n, ok := it.cursor.readU16(); ok {
			instructions, _ = it.cursor.readArray(int(n), 1)
		}
	}
	return count, instructions
}

// More flags of a component of a composite glyph (CompositeGlyphFlags).
const (
	ahCompRoundXYToGrid      = 0x0004
	ahCompWeHaveInstructions = 0x0100
)

// findGlyphAndPointCount is find_glyph_and_point_count of gvar.rs: the
// glyph that has the metrics of a glyph, and the number of its points. For
// a simple glyph that is the glyph. For a composite glyph it is the last
// component with the flag USE_MY_METRICS, or the glyph with the number of
// its components. The deltas of the phantom points start at that number.
func (f *ahFont) findGlyphAndPointCount(gid uint32, recurseDepth int) (uint32, int, bool) {
	// The limit of HarfBuzz for nested glyphs.
	const recursionLimit = 64
	if recurseDepth > recursionLimit {
		return 0, 0, false
	}
	glyph, ok := f.getGlyf(gid)
	if !ok {
		return 0, 0, false
	}
	if len(glyph) == 0 {
		// A glyph without an outline can have deltas for its phantom
		// points.
		return gid, 0, true
	}
	if glyph.i16(0) >= 0 {
		return gid, newAHSimpleGlyph(glyph).numPoints(), true
	}
	count := 0
	inheritMetrics, hasInherit := uint16(0), false
	for it := newAHComponentGlyphIDFlagsIter(glyph); ; {
		componentID, flags, ok := it.next()
		if !ok {
			break
		}
		if flags&ahCompUseMyMetrics != 0 {
			inheritMetrics, hasInherit = componentID, true
		}
		count++
	}
	if hasInherit {
		return f.findGlyphAndPointCount(uint32(inheritMetrics), recurseDepth+1)
	}
	return gid, count, true
}

// phantomPointDeltas is Gvar::phantom_point_deltas: the deltas of the four
// phantom points of a glyph at normalized coordinates, as 16.16 numbers. ok
// is false for a glyph without variation data and for data that is not
// valid.
func (g *ahGvar) phantomPointDeltas(f *ahFont, coords []int16, gid uint32) (phantomDeltas [4]ahFPoint, ok bool) {
	gid, pointCount, ok := f.findGlyphAndPointCount(gid, 0)
	if !ok {
		return phantomDeltas, false
	}
	varData, state := g.glyphVariationData(gid)
	if state != ahVarDataSome {
		return phantomDeltas, false
	}
	// A phantom point is not in a contour, so no delta is inferred for it.
	varData.activeTuplesAt(coords, func(tuple *ahTupleVariation, scalar int32) bool {
		for it := tuple.deltas(); ; {
			position, dx, dy, ok := it.next()
			if !ok {
				break
			}
			if ix := int(position); ix >= pointCount && ix < pointCount+4 {
				delta := ahApplyScalar(dx, dy, scalar)
				phantomDeltas[ix-pointCount].x += delta.x
				phantomDeltas[ix-pointCount].y += delta.y
			}
		}
		return true
	})
	return phantomDeltas, true
}

// ---- deltas of a glyph (skrifa, src/outline/glyf/deltas.rs) ----

// computeDeltasForGlyph is compute_deltas_for_glyph: the sum of the deltas
// of the tuples of a glyph at normalized coordinates, as 16.16 numbers.
// applyTupleMissingDeltas adds the deltas of a tuple that does not have
// deltas for all points. The result is false for data that is not valid.
func (g *ahGvar) computeDeltasForGlyph(varData *ahTupleVariationData, state int, coords []int16, deltas []ahFPoint, applyTupleMissingDeltas func(scalar int32, tuple *ahTupleVariation) bool) bool {
	clear(deltas)
	if state != ahVarDataSome {
		// A glyph without variation data is not an error.
		return true
	}
	result := true
	varData.activeTuplesAt(coords, func(tuple *ahTupleVariation, scalar int32) bool {
		if tuple.hasDeltasForAllPoints() {
			// The deltas of the tuple are the deltas of the points.
			result = tuple.accumulateDenseDeltas(deltas, scalar)
		} else {
			result = applyTupleMissingDeltas(scalar, tuple)
		}
		return result
	})
	return result
}

// compositeGlyph is composite_glyph: the deltas of the offsets of the
// components of a composite glyph, and of its phantom points. No delta is
// inferred for a component.
func (g *ahGvar) compositeGlyph(gid uint32, coords []int16, deltas []ahFPoint) bool {
	varData, state := g.glyphVariationData(gid)
	return g.computeDeltasForGlyph(&varData, state, coords, deltas, func(scalar int32, tuple *ahTupleVariation) bool {
		for it := tuple.deltas(); ; {
			position, dx, dy, ok := it.next()
			if !ok {
				break
			}
			if ix := int(position); ix < len(deltas) {
				delta := ahApplyScalar(dx, dy, scalar)
				deltas[ix].x += delta.x
				deltas[ix].y += delta.y
			}
		}
		return true
	})
}

// simpleGlyph is simple_glyph: the deltas of the points of a simple glyph,
// and of its phantom points. points has the points in font units, flags
// their flags and contours the last point of each contour. A tuple that has
// deltas for some points gives the other points a delta by interpolation.
// iupBuffer is the buffer for that.
//
// The result says that the glyph has deltas. For variation data that does
// not read, skrifa returns without an error before it clears deltas, so
// the deltas are then what the buffer had.
func (g *ahGvar) simpleGlyph(gid uint32, coords []int16, points []ahFPoint, flags []uint8, contours []uint16, iupBuffer, deltas []ahFPoint) bool {
	if len(iupBuffer) < len(points) || len(points) < ahPhantomPointCount {
		return false
	}
	varData, state := g.glyphVariationData(gid)
	if state == ahVarDataErr {
		return true
	}
	return g.computeDeltasForGlyph(&varData, state, coords, deltas, func(scalar int32, tuple *ahTupleVariation) bool {
		// The buffer starts with the points as 16.16 numbers, and no point
		// has a delta yet.
		n := min(len(flags), len(points), len(iupBuffer))
		for i := 0; i < n; i++ {
			iupBuffer[i] = ahFPoint{points[i].x << 16, points[i].y << 16}
			flags[i] &^= ahMarkerHasDelta
		}
		if !tuple.accumulateSparseDeltas(iupBuffer, flags, scalar) {
			return false
		}
		if !ahInterpolateDeltas(points, flags, contours, iupBuffer) {
			return false
		}
		n = min(len(deltas), len(points), len(iupBuffer))
		for i := 0; i < n; i++ {
			deltas[i].x += iupBuffer[i].x - points[i].x<<16
			deltas[i].y += iupBuffer[i].y - points[i].y<<16
		}
		return true
	})
}

// ahInterpolateDeltas is interpolate_deltas: it gives the points without a
// delta a position between the points with a delta, as the instruction IUP
// does. points has the points in font units, and outPoints the points with
// the deltas of the tuple as 16.16 numbers.
func ahInterpolateDeltas(points []ahFPoint, flags []uint8, contours []uint16, outPoints []ahFPoint) bool {
	jiggler := ahJiggler{points, outPoints}
	hasDelta := func(ix int) (has, ok bool) {
		if ix >= len(flags) {
			return false, false
		}
		return flags[ix]&ahMarkerHasDelta != 0, true
	}
	pointIx := 0
	for _, c := range contours {
		endPointIx := int(c)
		firstPointIx := pointIx
		// The first point with a delta.
		for pointIx <= endPointIx {
			has, ok := hasDelta(pointIx)
			if !ok {
				return false
			}
			if has {
				break
			}
			pointIx++
		}
		// A contour without a delta does not change.
		if pointIx > endPointIx {
			continue
		}
		firstDeltaIx := pointIx
		curDeltaIx := pointIx
		pointIx++
		// The next point with a delta, and the points between the two.
		for pointIx <= endPointIx {
			has, ok := hasDelta(pointIx)
			if !ok {
				return false
			}
			if has {
				if !jiggler.interpolate(curDeltaIx+1, pointIx-1, curDeltaIx, pointIx) {
					return false
				}
				curDeltaIx = pointIx
			}
			pointIx++
		}
		if curDeltaIx == firstDeltaIx {
			// One delta moves the contour.
			if !jiggler.shift(firstPointIx, endPointIx, curDeltaIx) {
				return false
			}
		} else {
			// The points after the last delta and before the first one.
			if !jiggler.interpolate(curDeltaIx+1, endPointIx, curDeltaIx, firstDeltaIx) {
				return false
			}
			if firstDeltaIx > 0 {
				if !jiggler.interpolate(firstPointIx, firstDeltaIx-1, curDeltaIx, firstDeltaIx) {
					return false
				}
			}
		}
	}
	return true
}

// ahJiggler is Jiggler: the points of a glyph in font units, and the points
// with deltas as 16.16 numbers.
type ahJiggler struct {
	points    []ahFPoint
	outPoints []ahFPoint
}

// shift is Jiggler::shift: it moves the points first to last by the delta
// of the point ref. The point ref has its delta already.
func (j *ahJiggler) shift(first, last, refIx int) bool {
	if refIx >= len(j.points) || refIx >= len(j.outPoints) {
		return false
	}
	deltaX := j.outPoints[refIx].x - j.points[refIx].x<<16
	deltaY := j.outPoints[refIx].y - j.points[refIx].y<<16
	if deltaX == 0 && deltaY == 0 {
		return true
	}
	if first > refIx || refIx > len(j.outPoints) {
		return false
	}
	for i := first; i < refIx; i++ {
		j.outPoints[i].x += deltaX
		j.outPoints[i].y += deltaY
	}
	if refIx+1 > last+1 || last+1 > len(j.outPoints) {
		return false
	}
	for i := refIx + 1; i <= last; i++ {
		j.outPoints[i].x += deltaX
		j.outPoints[i].y += deltaY
	}
	return true
}

// interpolate is Jiggler::interpolate: it places the points first to last
// between the points ref1 and ref2, which have deltas. Each coordinate is
// done alone.
func (j *ahJiggler) interpolate(first, last, ref1, ref2 int) bool {
	if first > last {
		return true
	}
	return j.interpolateCoord(first, last, ref1, ref2, func(p *ahFPoint) *int32 { return &p.x }) &&
		j.interpolateCoord(first, last, ref1, ref2, func(p *ahFPoint) *int32 { return &p.y })
}

func (j *ahJiggler) interpolateCoord(first, last, ref1Ix, ref2Ix int, coord func(p *ahFPoint) *int32) bool {
	if ref1Ix >= len(j.points) || ref2Ix >= len(j.points) {
		return false
	}
	if *coord(&j.points[ref1Ix]) > *coord(&j.points[ref2Ix]) {
		ref1Ix, ref2Ix = ref2Ix, ref1Ix
	}
	in1 := *coord(&j.points[ref1Ix]) << 16
	in2 := *coord(&j.points[ref2Ix]) << 16
	if ref1Ix >= len(j.outPoints) || ref2Ix >= len(j.outPoints) {
		return false
	}
	out1 := *coord(&j.outPoints[ref1Ix])
	out2 := *coord(&j.outPoints[ref2Ix])
	// Two points with the same coordinate and with deltas that differ give
	// no delta.
	if in1 != in2 || out1 == out2 {
		scale := int32(0)
		if in1 != in2 {
			scale = ahFixedDiv(out2-out1, in2-in1)
		}
		d1 := out1 - in1
		d2 := out2 - in2
		if last >= len(j.points) || last >= len(j.outPoints) {
			return false
		}
		for i := first; i <= last; i++ {
			out := *coord(&j.points[i]) << 16
			switch {
			case out <= in1:
				out += d1
			case out >= in2:
				out += d2
			default:
				out = out1 + ahFixedMul(out-in1, scale)
			}
			*coord(&j.outPoints[i]) = out
		}
	}
	return true
}

// ---- the variation tables of a font ----

// ahVarFont has the variation tables of a font: it is nil for a font
// without gvar, MVAR and HVAR.
type ahVarFont struct {
	// gvar is nil for a font without the table.
	gvar *ahGvar
	// hvar is nil for a font without the table.
	hvar *ahHvar
	// mvar is nil for a font without the table.
	mvar *ahMvar
}

// newAHVarFont reads the variation tables of a font. A table that is
// shorter than its fixed fields is no table, as in read-fonts.
func newAHVarFont(ld *ot.Loader) *ahVarFont {
	var v ahVarFont
	if gvar, ok := ahRawTable(ld, "gvar"); ok && len(gvar) >= 20 {
		v.gvar = &ahGvar{data: gvar, axisCount: int(gvar.u16(4))}
	}
	if hvar, ok := ahRawTable(ld, "HVAR"); ok && len(hvar) >= 20 {
		v.hvar = &ahHvar{data: hvar}
	}
	if mvar, ok := ahRawTable(ld, "MVAR"); ok && len(mvar) >= 12 {
		v.mvar = &ahMvar{data: mvar}
	}
	if v.gvar == nil && v.hvar == nil && v.mvar == nil {
		return nil
	}
	return &v
}

// newAHHdmx reads the hdmx table of a font. It returns nil for a font
// without the table, and for a font without maxp: the table has one width
// for each glyph.
func newAHHdmx(ld *ot.Loader, f *ahFont) *ahHdmx {
	if !f.hasMaxp {
		return nil
	}
	hdmx, ok := ahRawTable(ld, "hdmx")
	if !ok || len(hdmx) < 8 {
		return nil
	}
	return &ahHdmx{data: hdmx, numGlyphs: int(f.glyphCount)}
}

// ---- the scaler for an instance and for the hinted mode ----

// computeScale is Scale26Dot6::new for a font size in px: the 16.16 scale
// from font units to 26.6 px. isScaled is false for a font without units
// per em.
func (f *ahFont) computeScale(size float32) (scale int32, isScaled bool) {
	if f.unitsPerEm > 0 {
		return ahFixedDiv(ahF32ToI32(size*64.0), int32(f.unitsPerEm)), true
	}
	return 0x10000, false
}

// computeHintedScale is Outlines::compute_hinted_scale: the scale of the
// hinted mode. A font with bit 3 of head.flags gets the font size rounded
// to a whole number of px, in 26.6, as in FreeType.
func (f *ahFont) computeHintedScale(size float32) (scale int32, isScaled bool) {
	if f.forceIntegerPpem {
		rounded := ahPixRound(ahF26Dot6FromF64(float64(size)))
		const scaleFactor = 1.0 / 64.0
		return f.computeScale(float32(rounded) * scaleFactor)
	}
	return f.computeScale(size)
}

// hintedPpem is the font size in whole px that the programs of the font
// see in the hinted mode (the instruction MPPEM): the rounded ppem of
// HintingInstance::reconfigure. FreeType rounds the product of the scale
// and the units per em in 26.6.
//
// The prep program of a font can turn hinting off for some sizes
// (HintingInstance::is_enabled). It decides that with this number.
func (f *ahFont) hintedPpem(size float32) int32 {
	scale, _ := f.computeHintedScale(size)
	return (ahFixedMul(scale, int32(f.unitsPerEm)) + 32) >> 6
}

// scaleOutlineAt is FreeTypeScaler::unhinted (hinted false) or
// FreeTypeScaler::hinted without the interpreter (hinted true) followed by
// FreeTypeScaler::scale, at a font size in px and normalized coordinates
// (nil for the default instance). backwardCompat is
// HintInstance::backward_compatibility of the hinting instance. Only the
// hinted mode reads it.
//
// The coordinates can have zeros only: the scaler takes the effective
// coordinates, as skrifa does.
//
// The hinted mode is exact for a glyph without instructions
// (hasInstructions). For a glyph with instructions the points are the points
// before the program of the glyph, at the scale of the hinted mode.
func (f *ahFont) scaleOutlineAt(o ahGlyfOutline, size float32, coords []int16, hinted, backwardCompat bool) (*ahGlyfScaler, bool) {
	if o.points > ahMaxGlyfPoints {
		return nil, false
	}
	s := &ahGlyfScaler{
		font:     f,
		coords:   ahEffectiveCoords(coords),
		scaled:   make([]ahFPoint, o.points),
		flags:    make([]uint8, o.points),
		contours: make([]uint16, o.contours),
	}
	if hinted {
		s.scale, s.isScaled = f.computeHintedScale(size)
		// An outline without a size is not hinted.
		s.isHinted = s.isScaled
		s.backwardCompat = backwardCompat
	} else {
		s.scale, s.isScaled = f.computeScale(size)
	}
	// FreeTypeOutlineMemory::new: the buffers for the deltas.
	if s.gvar() != nil {
		s.deltas = make([]ahFPoint, o.maxSimplePoints)
		s.iupBuffer = make([]ahFPoint, o.maxSimplePoints)
		s.compositeDeltas = make([]ahFPoint, o.maxComponentDeltaStack)
	}
	if !s.load(o.glyph, o.glyphID, 0) {
		return nil, false
	}
	// The width of the hdmx table is for the hinted mode without the
	// backward compatibility mode.
	if s.isHinted && !backwardCompat {
		if width, ok := f.hdmx.width(size, o.glyphID); ok {
			s.hdmxWidth, s.hasHdmxWidth = int32(width), true
		}
	}
	// ScaledOutline::new: the origin of the glyph is at its first phantom
	// point.
	if xShift := s.phantom[0].x; xShift != 0 {
		for i := 0; i < s.pointCount; i++ {
			s.scaled[i].x -= xShift
		}
	}
	return s, true
}

// adjustedAdvance is ScaledOutline::adjusted_advance_width in 26.6 px: the
// width of the hdmx table, or the distance of the first two phantom points.
// The hinted draw rounds it to a whole pixel (ahPixRound).
func (s *ahGlyfScaler) adjustedAdvance() int32 {
	if s.hasHdmxWidth {
		return s.hdmxWidth << 6
	}
	return s.phantom[1].x - s.phantom[0].x
}

// adjustedLsb is ScaledOutline::adjusted_lsb in 26.6 px: the x of the first
// phantom point.
func (s *ahGlyfScaler) adjustedLsb() int32 { return s.phantom[0].x }

// toPath is ScaledOutline::to_path with the pen of Skia. The result is
// false for an outline that has no valid path.
func (s *ahGlyfScaler) toPath(pen *ahPathPen) bool {
	points, flags := s.scaled[:s.pointCount], s.flags[:s.pointCount]
	// to_path: a contour that ends before it starts, or after the last
	// point, is an error.
	startIx := 0
	for _, c := range s.contours[:s.contourCount] {
		endIx := int(c)
		if endIx < startIx || endIx >= len(points) {
			return false
		}
		contour, contourFlags := points[startIx:endIx+1], flags[startIx:endIx+1]
		ok := ahContourToPath(len(contour), func(i int) ahPathPoint {
			return ahPathPoint{x: contour[i].x, y: contour[i].y, flags: contourFlags[i]}
		}, pen)
		if !ok {
			return false
		}
		startIx = endIx + 1
	}
	return true
}

// pathAt is the path that the pen of Skia records for the outline of a
// glyph (the segments that ahInstance.path returns), for the same arguments
// as scaleOutlineAt. The result is false for a glyph that has no valid
// path. A glyph without a path has no segments.
func (f *ahFont) pathAt(o ahGlyfOutline, size float32, coords []int16, hinted, backwardCompat bool) ([]ot.Segment, bool) {
	s, ok := f.scaleOutlineAt(o, size, coords, hinted, backwardCompat)
	if !ok {
		return nil, false
	}
	var pen ahPathPen
	if !s.toPath(&pen) {
		return nil, false
	}
	return pen.segments, true
}

// ahDraw is the result of OutlineGlyph::draw with the pen of Skia: the path
// and the AdjustedMetrics.
type ahDraw struct {
	segments []ot.Segment
	// advance and lsb are the advance width and the left side bearing of
	// AdjustedMetrics in px.
	advance, lsb float32
}

// drawAt is OutlineGlyph::draw with the pen of Skia, at a font size in px
// and normalized coordinates.
//
// hinted false is DrawSettings::unhinted. hinted true is
// DrawSettings::hinted with a hinting instance that uses the interpreter:
// enabled is HintingInstance::is_enabled and backwardCompat is
// HintInstance::backward_compatibility of that instance. An instance that
// is not enabled draws without hinting and rounds the advance. An instance
// that is enabled draws in the hinted mode of the scaler and rounds the
// advance in 26.6.
//
// The result is false for a glyph that skrifa does not draw.
func (f *ahFont) drawAt(o ahGlyfOutline, size float32, coords []int16, hinted, enabled, backwardCompat bool) (ahDraw, bool) {
	var d ahDraw
	scalerHinted := hinted && enabled
	s, ok := f.scaleOutlineAt(o, size, coords, scalerHinted, backwardCompat)
	if !ok {
		return d, false
	}
	var pen ahPathPen
	if !s.toPath(&pen) {
		return d, false
	}
	d.segments = pen.segments
	// F26Dot6::to_f32.
	const scaleFactor = 1.0 / 64.0
	d.lsb = float32(s.adjustedLsb()) * scaleFactor
	switch {
	case scalerHinted:
		// HintingInstance::draw: FreeType rounds the advance of a hinted
		// glyph.
		d.advance = float32(ahPixRound(s.adjustedAdvance())) * scaleFactor
	case hinted:
		// OutlineGlyph::draw: the advance is rounded also when the
		// instance is not enabled. A half rounds away from zero.
		d.advance = float32(math.Round(float64(float32(s.adjustedAdvance()) * scaleFactor)))
	default:
		d.advance = float32(s.adjustedAdvance()) * scaleFactor
	}
	return d, true
}

// hasInstructions reports whether the glyph, or a component of it, has
// bytecode (Outline::has_hinting). The hinted mode of the scaler is exact
// for a glyph without instructions only.
func (f *ahFont) hasInstructions(o ahGlyfOutline) bool { return o.hasHinting }

// advanceWidthAt is GlyphMetrics::advance_width before scaling: the advance
// of a glyph in font units with the HVAR delta, or without HVAR the gvar
// delta of the phantom points (metric_deltas_from_gvar), at normalized
// coordinates. The result is 0 for a glyph that the font does not have:
// GlyphMetrics::advance_width returns nothing for it, and Skia takes 0
// (unhinted_advance_width_or_zero).
func (f *ahFont) advanceWidthAt(gid uint32, coords []int16) int32 {
	if gid >= uint32(f.glyphCount) {
		return 0
	}
	coords = ahEffectiveCoords(coords)
	advance := f.advanceWidth(gid)
	v := f.vari
	if v == nil {
		return advance
	}
	if v.hvar != nil {
		// FreeType cuts the fraction of a delta of a metric. The delta of
		// HVAR is a whole number in a 16.16 number, and the cast goes
		// through that number.
		if delta, ok := v.hvar.advanceWidthDelta(gid, coords); ok {
			advance += delta >> 16
		}
	} else if v.gvar != nil {
		// metric_deltas_from_gvar: the delta of the advance is the
		// difference of the deltas of the first two phantom points.
		if deltas, ok := v.gvar.phantomPointDeltas(f, coords, gid); ok {
			advance += ahFixedToI32(deltas[1].x - deltas[0].x)
		}
	}
	return advance
}

// linearAdvance is GlyphMetrics::advance_width at a font size in px: the
// advance of a glyph without hinting, in px. FreeType scales a metric with
// the 16.16 scale that gives 26.6 (Size::fixed_linear_scale and
// FixedScaleFactor::apply). The result is 0 for a glyph that the font does
// not have.
func (f *ahFont) linearAdvance(gid uint32, size float32, coords []int16) float32 {
	if gid >= uint32(f.glyphCount) {
		return 0
	}
	// The scale 0x10000 * 64 keeps the font units.
	scale := int32(0x10000 * 64)
	if f.unitsPerEm > 0 {
		scale = ahFixedDiv(ahF32ToI32(size*64.0), int32(f.unitsPerEm))
	}
	// Fixed::to_f32.
	const scaleFactor = 1.0 / 65536.0
	return float32(ahFixedMulDiv(scale, f.advanceWidthAt(gid, coords), 64)) * scaleFactor
}

// mvarDelta is the delta that Metrics::new of skrifa adds to a metric of
// the font: Mvar::metric_delta for a tag at normalized coordinates, as
// Fixed 16.16 bits. It is 0 without MVAR, for a tag that MVAR does not
// have, and for the default instance (coordinates that are all zero).
func (f *ahFont) mvarDelta(tag uint32, coords []int16) int32 {
	coords = ahEffectiveCoords(coords)
	if f.vari == nil || f.vari.mvar == nil || len(coords) == 0 {
		return 0
	}
	delta, _ := f.vari.mvar.metricDelta(tag, coords)
	return delta
}
