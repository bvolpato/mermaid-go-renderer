// Ported from skrifa 0.46.2 (src/outline/mod.rs: draw_unscaled and
// draw_unhinted, src/outline/unscaled.rs, src/outline/glyf/mod.rs:
// Outlines::outline and FreeTypeScaler without the interpreter of TrueType
// instructions, src/outline/glyf/outline.rs, src/outline/glyf/memory.rs,
// src/outline/path.rs) and from read-fonts 0.43.3 (src/tables/glyf.rs,
// src/tables/loca.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/truetype/ttgload.c, src/truetype/ttobjs.c, src/base/ftoutln.c), which
// has the FreeType License (LICENSE-FreeType.txt).

package font

// The automatic hinter of skrifa starts from the outline of a glyph in font
// units (draw_unscaled). The scaler of skrifa loads that outline with the
// same code as a scaled outline, and with the scale 1: a composite glyph has
// the transforms and the offsets of its components in font units, rounded to
// whole units, and the outline moves so that the origin of the glyph is at
// its first phantom point. So the points differ from the points of the
// scaled outline that loadGlyph in linux_scaler.go makes.

// Flags of a point (PointFlags of read-fonts) and the markers that the
// automatic hinter sets (PointMarker).
const (
	ahPointOnCurve            uint8 = 0x01
	ahPointOffCurveCubic      uint8 = 0x80
	ahPointCurveMask                = ahPointOnCurve | ahPointOffCurveCubic
	ahMarkerWeakInterpolation uint8 = 0x02
	ahMarkerNear              uint8 = 0x08
	ahMarkerTouchedX          uint8 = 0x10
	ahMarkerTouchedY          uint8 = 0x20
)

// Limits of skrifa (src/lib.rs).
const (
	ahMaxGlyfPoints               = 0xFFFF
	ahGlyfCompositeRecursionLimit = 32
	ahMaxGraphEdges               = 2048
	ahPhantomPointCount           = 4
)

// Flags of a component of a composite glyph (CompositeGlyphFlags).
const (
	ahCompArg1And2AreWords        = 0x0001
	ahCompArgsAreXYValues         = 0x0002
	ahCompWeHaveAScale            = 0x0008
	ahCompMoreComponents          = 0x0020
	ahCompWeHaveAnXAndYScale      = 0x0040
	ahCompWeHaveATwoByTwo         = 0x0080
	ahCompUseMyMetrics            = 0x0200
	ahCompScaledComponentOffset   = 0x0800
	ahCompUnscaledComponentOffset = 0x1000
)

// ahUnscaledPoint is UnscaledPoint: a point of an outline in font units.
type ahUnscaledPoint struct {
	x, y           int16
	flags          uint8
	isContourStart bool
}

func (p ahUnscaledPoint) isOnCurve() bool { return p.flags&ahPointOnCurve != 0 }

// ahFPoint is a point in 26.6 fixed point.
type ahFPoint struct{ x, y int32 }

// getGlyf is Loca::get_glyf: the data of a glyph. The data is empty for a
// glyph without an outline. ok is false for a glyph that the font does not
// have, and for data that is too short for a glyph.
func (f *ahFont) getGlyf(gid uint32) (data ahData, ok bool) {
	getRaw := func(idx int) (uint32, bool) {
		if f.locaLong {
			if 4*idx+4 > len(f.loca) {
				return 0, false
			}
			return f.loca.u32(4 * idx), true
		}
		if 2*idx+2 > len(f.loca) {
			return 0, false
		}
		return uint32(f.loca.u16(2*idx)) * 2, true
	}
	idx := int(gid)
	start, ok := getRaw(idx)
	if !ok {
		return nil, false
	}
	end, ok := getRaw(idx + 1)
	if !ok {
		return nil, false
	}
	if start == end {
		return nil, true
	}
	if start > end || int64(end) > int64(len(f.glyf)) {
		return nil, false
	}
	data = f.glyf[start:end]
	// A simple glyph has 12 bytes of fixed fields, and a composite glyph
	// has 10.
	if len(data) < 2 {
		return nil, false
	}
	if data.i16(0) >= 0 {
		if len(data) < 12 {
			return nil, false
		}
	} else if len(data) < 10 {
		return nil, false
	}
	return data, true
}

// ahSimpleGlyph has the parts of a simple glyph (SimpleGlyph of read-fonts).
type ahSimpleGlyph struct {
	endPtsOfContours ahData
	glyphData        ahData
	// instructionLength is the length field of the instructions, and
	// instructions has the instructions. They are empty when they do not
	// fit in the data.
	instructionLength int
	instructions      ahData
}

func newAHSimpleGlyph(data ahData) ahSimpleGlyph {
	numberOfContours := int(data.i16(0))
	var g ahSimpleGlyph
	g.endPtsOfContours = data.array(10, numberOfContours, 2)
	off := 10 + 2*numberOfContours
	instructionLength := int(data.u16(off))
	g.instructionLength = instructionLength
	g.instructions = data.array(off+2, instructionLength, 1)
	off += 2 + instructionLength
	g.glyphData = data.rest(off, 1)
	return g
}

// numPoints is SimpleGlyph::num_points.
func (g ahSimpleGlyph) numPoints() int {
	n := len(g.endPtsOfContours) / 2
	if n == 0 {
		return 0
	}
	return int(g.endPtsOfContours.u16(2*(n-1))) + 1
}

// readPointsFast is SimpleGlyph::read_points_fast: the points in font units
// and the flags. Only the flag for a point on the curve stays.
func (g ahSimpleGlyph) readPointsFast(points []ahFPoint, flags []uint8) bool {
	nPoints := len(points)
	if nPoints == 0 {
		return true
	}
	data := g.glyphData
	readFlagsBytes := 0
	for i := 0; i < nPoints && readFlagsBytes < len(data); {
		flagBits := data[readFlagsBytes]
		readFlagsBytes++
		if flagBits&glyfRepeat != 0 {
			if readFlagsBytes >= len(data) {
				return false
			}
			count := min(int(data[readFlagsBytes])+1, nPoints-i)
			readFlagsBytes++
			for k := 0; k < count; k++ {
				flags[i+k] = flagBits
			}
			i += count
		} else {
			flags[i] = flagBits
			i++
		}
	}
	p := readFlagsBytes
	var x int32
	for i := range points {
		var delta int32
		flag := flags[i]
		if flag&glyfXShort != 0 {
			if p >= len(data) {
				return false
			}
			delta = int32(data[p])
			p++
			if flag&glyfXSame == 0 {
				delta = -delta
			}
		} else if flag&glyfXSame == 0 {
			if p+2 > len(data) {
				return false
			}
			delta = int32(data.i16(p))
			p += 2
		}
		x += delta
		points[i].x = x
	}
	var y int32
	for i := range points {
		var delta int32
		flag := flags[i]
		if flag&glyfYShort != 0 {
			if p >= len(data) {
				return false
			}
			delta = int32(data[p])
			p++
			if flag&glyfYSame == 0 {
				delta = -delta
			}
		} else if flag&glyfYSame == 0 {
			if p+2 > len(data) {
				return false
			}
			delta = int32(data.i16(p))
			p += 2
		}
		y += delta
		points[i].y = y
		flags[i] &= ahPointOnCurve
	}
	return true
}

// ahComponent is a component of a composite glyph (Component of read-fonts).
type ahComponent struct {
	flags uint16
	glyph uint16
	// anchorIsOffset selects the meaning of the anchor: an offset (x, y), or
	// the numbers of a point of the glyph so far and of a point of the
	// component.
	anchorIsOffset  bool
	x, y            int16
	base, component uint16
	xx, yx, xy, yy  int16 // 2.14
}

// components is CompositeGlyph::components. It stops without an error at
// data that is too short.
func ahComponents(data ahData) []ahComponent {
	d := data.rest(10, 1)
	var out []ahComponent
	p := 0
	for {
		if p+4 > len(d) {
			return out
		}
		c := ahComponent{flags: d.u16(p), glyph: d.u16(p + 2), xx: 0x4000, yy: 0x4000}
		p += 4
		words := c.flags&ahCompArg1And2AreWords != 0
		c.anchorIsOffset = c.flags&ahCompArgsAreXYValues != 0
		switch {
		case c.anchorIsOffset && words:
			if p+4 > len(d) {
				return out
			}
			c.x, c.y = d.i16(p), d.i16(p+2)
			p += 4
		case c.anchorIsOffset:
			if p+2 > len(d) {
				return out
			}
			c.x, c.y = int16(int8(d[p])), int16(int8(d[p+1]))
			p += 2
		case words:
			if p+4 > len(d) {
				return out
			}
			c.base, c.component = d.u16(p), d.u16(p+2)
			p += 4
		default:
			if p+2 > len(d) {
				return out
			}
			c.base, c.component = uint16(d[p]), uint16(d[p+1])
			p += 2
		}
		switch {
		case c.flags&ahCompWeHaveAScale != 0:
			if p+2 > len(d) {
				return out
			}
			c.xx = d.i16(p)
			c.yy = c.xx
			p += 2
		case c.flags&ahCompWeHaveAnXAndYScale != 0:
			if p+4 > len(d) {
				return out
			}
			c.xx, c.yy = d.i16(p), d.i16(p+2)
			p += 4
		case c.flags&ahCompWeHaveATwoByTwo != 0:
			if p+8 > len(d) {
				return out
			}
			c.xx, c.yx, c.xy, c.yy = d.i16(p), d.i16(p+2), d.i16(p+4), d.i16(p+6)
			p += 8
		}
		out = append(out, c)
		if c.flags&ahCompMoreComponents == 0 {
			return out
		}
	}
}

// ahComponentGlyphs is CompositeGlyph::component_glyphs_and_flags: the glyph
// of each component. It reads the flags and the glyph of a component only,
// and it steps over the other fields.
func ahComponentGlyphs(data ahData) []uint16 {
	d := data.rest(10, 1)
	var out []uint16
	p := 0
	for {
		if p+4 > len(d) {
			return out
		}
		flags := d.u16(p)
		out = append(out, d.u16(p+2))
		p += 4
		if flags&ahCompArg1And2AreWords != 0 {
			p += 4
		} else {
			p += 2
		}
		switch {
		case flags&ahCompWeHaveAScale != 0:
			p += 2
		case flags&ahCompWeHaveAnXAndYScale != 0:
			p += 4
		case flags&ahCompWeHaveATwoByTwo != 0:
			p += 8
		}
		if flags&ahCompMoreComponents == 0 {
			return out
		}
	}
}

// ahGlyfOutline is Outline of src/outline/glyf/outline.rs: the glyph and the
// number of points and contours that it needs.
type ahGlyfOutline struct {
	glyphID  uint32
	glyph    ahData // empty for a glyph without an outline
	points   int
	contours int
	// maxSimplePoints is the largest number of points of a simple glyph of
	// the outline, with its phantom points: the size of the buffers for
	// the deltas of a variable font.
	maxSimplePoints int
	// maxComponentDeltaStack is the size of the stack for the deltas of
	// the components of the composite glyphs of the outline.
	maxComponentDeltaStack int
	// hasHinting says that the glyph or a component of it has
	// instructions.
	hasHinting bool
}

// outline is Outlines::outline. ok is false for a glyph that skrifa does not
// load: a glyph that the font does not have, a glyph with too many points,
// and a composite glyph that is too deep.
func (f *ahFont) outline(gid uint32) (ahGlyfOutline, bool) {
	o := ahGlyfOutline{glyphID: gid}
	glyph, ok := f.getGlyf(gid)
	if !ok {
		return o, false
	}
	if len(glyph) != 0 {
		totalComponents := 0
		if !f.outlineRec(glyph, &o, 0, 0, &totalComponents) {
			return o, false
		}
	}
	o.points += ahPhantomPointCount
	o.glyph = glyph
	return o, true
}

// outlineRec is Outlines::outline_rec. componentDepth is the number of
// deltas of the composite glyphs that have the glyph as a component.
func (f *ahFont) outlineRec(glyph ahData, o *ahGlyfOutline, componentDepth, recurseDepth int, totalComponents *int) bool {
	if recurseDepth > ahGlyfCompositeRecursionLimit {
		return false
	}
	if glyph.i16(0) >= 0 {
		simple := newAHSimpleGlyph(glyph)
		numPoints := simple.numPoints()
		o.maxSimplePoints = max(o.maxSimplePoints, numPoints+ahPhantomPointCount)
		o.points += numPoints
		if o.points > ahMaxGlyfPoints {
			return false
		}
		o.contours += len(simple.endPtsOfContours) / 2
		o.hasHinting = o.hasHinting || simple.instructionLength != 0
		return true
	}
	// A composite glyph has one delta for each component and for each
	// phantom point.
	count, instructions := ahCountAndInstructions(glyph)
	count += ahPhantomPointCount
	for _, component := range ahComponentGlyphs(glyph) {
		componentGlyph, ok := f.getGlyf(uint32(component))
		if !ok {
			return false
		}
		if len(componentGlyph) == 0 {
			continue
		}
		*totalComponents++
		if *totalComponents > ahMaxGraphEdges {
			return false
		}
		if !f.outlineRec(componentGlyph, o, componentDepth+count, recurseDepth+1, totalComponents) {
			return false
		}
	}
	o.maxComponentDeltaStack = max(o.maxComponentDeltaStack, componentDepth+count)
	o.hasHinting = o.hasHinting || len(instructions) != 0
	return true
}

// ahGlyfScaler is FreeTypeScaler of skrifa without the interpreter of
// TrueType instructions. The points are 26.6 numbers. Without a size
// (isScaled is false) the outline is in font units, which is what the
// automatic hinter starts from.
//
// scaleOutline makes a scaler without variations and without hinting.
// scaleOutlineAt in glyf_var.go makes a scaler for an instance of a
// variable font, and for the hinted mode.
type ahGlyfScaler struct {
	font *ahFont
	// scale converts font units to 26.6 px (Scale26Dot6).
	scale        int32
	isScaled     bool
	scaled       []ahFPoint
	flags        []uint8
	contours     []uint16
	pointCount   int
	contourCount int
	phantom      [ahPhantomPointCount]ahFPoint

	// coords has the normalized coordinates of the instance. It is empty
	// for the default instance, and then the scaler reads no deltas.
	coords []int16
	// isHinted is FreeTypeScaler::is_hinted: the hinted mode with a size.
	isHinted bool
	// backwardCompat is HintInstance::backward_compatibility of the
	// hinting instance of the hinted mode.
	backwardCompat bool
	// deltas and iupBuffer are the buffers for the deltas of the points of
	// a simple glyph, as 16.16 numbers. compositeDeltas is the stack of
	// the deltas of the components of the composite glyphs, and
	// componentDeltaCount is the number of deltas on it.
	deltas, iupBuffer   []ahFPoint
	compositeDeltas     []ahFPoint
	componentDeltaCount int
	// hdmxWidth is the width of the glyph in the hdmx table, in whole px.
	hdmxWidth    int32
	hasHdmxWidth bool
}

// gvar returns the gvar table when the scaler reads deltas: the font has
// the table and the instance is not the default instance.
func (s *ahGlyfScaler) gvar() *ahGvar {
	if len(s.coords) == 0 || s.font.vari == nil {
		return nil
	}
	return s.font.vari.gvar
}

// applyPoint is Scale26Dot6::apply_point: a point in font units as a 26.6
// point. Without a size the units stay, and only the format changes.
func (s *ahGlyfScaler) applyPoint(p ahFPoint) ahFPoint {
	if s.isScaled {
		return ahFPoint{ahFixedMul(p.x, s.scale), ahFixedMul(p.y, s.scale)}
	}
	return ahFPoint{p.x << 6, p.y << 6}
}

// load is Scaler::load.
func (s *ahGlyfScaler) load(glyph ahData, gid uint32, recurseDepth int) bool {
	if recurseDepth > ahGlyfCompositeRecursionLimit {
		return false
	}
	var xMin, yMax int32
	if len(glyph) != 0 {
		xMin, yMax = int32(glyph.i16(2)), int32(glyph.i16(8))
	}
	lsb := s.font.lsb(gid)
	advance := s.font.advanceWidth(gid)
	ascent, descent := int32(s.font.os2VMetrics[0]), int32(s.font.os2VMetrics[1])
	tsb := ascent - yMax
	vadvance := ascent - descent
	// setup_phantom_points: the four phantom points as FreeType computes
	// them.
	s.phantom[0] = ahFPoint{xMin - lsb, 0}
	s.phantom[1] = ahFPoint{s.phantom[0].x + advance, 0}
	s.phantom[2] = ahFPoint{0, yMax + tsb}
	s.phantom[3] = ahFPoint{0, s.phantom[2].y - vadvance}
	switch {
	case len(glyph) == 0:
		// load_empty: a glyph without an outline can have deltas for its
		// first two phantom points.
		unscaled := s.phantom
		if gvar := s.gvar(); gvar != nil {
			if deltas, ok := gvar.phantomPointDeltas(s.font, s.coords, gid); ok {
				for i := 0; i < 2; i++ {
					unscaled[i].x += ahFixedToI32(deltas[i].x)
					unscaled[i].y += ahFixedToI32(deltas[i].y)
				}
			}
		}
		for i := range s.phantom {
			s.phantom[i] = s.applyPoint(unscaled[i])
		}
		return true
	case glyph.i16(0) >= 0:
		return s.loadSimple(newAHSimpleGlyph(glyph), gid)
	}
	return s.loadComposite(glyph, gid, recurseDepth)
}

// loadSimple is load_simple.
func (s *ahGlyfScaler) loadSimple(glyph ahSimpleGlyph, gid uint32) bool {
	pointsStart := s.pointCount
	pointCount := glyph.numPoints()
	phantomStart := pointCount
	pointsEnd := pointsStart + pointCount + ahPhantomPointCount
	if pointsEnd > len(s.scaled) {
		return false
	}
	scaled := s.scaled[pointsStart:pointsEnd]
	flags := s.flags[pointsStart:pointsEnd]
	unscaled := make([]ahFPoint, pointCount+ahPhantomPointCount)
	if !glyph.readPointsFast(unscaled[:pointCount], flags[:pointCount]) {
		return false
	}
	contoursStart := s.contourCount
	contourCount := len(glyph.endPtsOfContours) / 2
	contoursEnd := contoursStart + contourCount
	if contoursEnd > len(s.contours) {
		return false
	}
	contours := s.contours[contoursStart:contoursEnd]
	// The end points of the contours must not go down.
	lastEndPt := uint16(0)
	for i := range contours {
		endPt := glyph.endPtsOfContours.u16(2 * i)
		if endPt < lastEndPt {
			return false
		}
		lastEndPt = endPt
		contours[i] = endPt
	}
	s.pointCount += pointCount
	s.contourCount += contourCount
	// The phantom points come after the points of the outline.
	for i, phantom := range s.phantom {
		unscaled[phantomStart+i] = phantom
		flags[phantomStart+i] = 0
	}
	// The deltas of the points and of the phantom points, for an instance
	// of a variable font.
	haveDeltas := false
	if gvar := s.gvar(); gvar != nil {
		n := pointCount + ahPhantomPointCount
		if n > len(s.deltas) || n > len(s.iupBuffer) {
			return false
		}
		haveDeltas = gvar.simpleGlyph(gid, s.coords, unscaled, flags, contours, s.iupBuffer[:n], s.deltas[:n])
	}
	switch {
	case haveDeltas && s.isScaled:
		for i := range scaled {
			// The point in 26.6 font units with its delta, times the
			// scale. The scale gives a number with 12 fraction bits, which
			// is rounded to 26.6.
			delta := s.deltas[i]
			x := ahFixedMul(unscaled[i].x<<6+ahFixedToF26Dot6(delta.x), s.scale)
			y := ahFixedMul(unscaled[i].y<<6+ahFixedToF26Dot6(delta.y), s.scale)
			scaled[i] = ahFPoint{ahF26Dot6ToI32(x), ahF26Dot6ToI32(y)}
		}
		// FreeType rounds the deltas of HVAR to whole font units. The
		// deltas of the phantom points get the same rounding for a font
		// with an HVAR table.
		if s.font.vari.hvar != nil {
			for i := phantomStart; i < len(scaled); i++ {
				delta := s.deltas[i]
				x := ahFixedMul(unscaled[i].x<<6+ahFixedToI32(delta.x)<<6, s.scale)
				y := ahFixedMul(unscaled[i].y<<6+ahFixedToI32(delta.y)<<6, s.scale)
				scaled[i] = ahFPoint{ahF26Dot6ToI32(x), ahF26Dot6ToI32(y)}
			}
		}
		// In the hinted mode skrifa adds the rounded deltas to the points
		// in font units also. Only the interpreter reads those points.
	case haveDeltas:
		// An outline without a size has the deltas rounded to whole font
		// units.
		for i := range scaled {
			delta := s.deltas[i]
			scaled[i] = ahFPoint{(unscaled[i].x + ahFixedToI32(delta.x)) << 6, (unscaled[i].y + ahFixedToI32(delta.y)) << 6}
		}
	default:
		// An outline without a size is in 26.6 also.
		for i := range scaled {
			scaled[i] = s.applyPoint(unscaled[i])
		}
	}
	copy(s.phantom[:], scaled[phantomStart:])
	// The hinted mode. For a glyph with instructions skrifa rounds the
	// phantom points and runs the program of the glyph. The program can
	// move the phantom points, and skrifa takes the moved points when the
	// backward compatibility mode is off. The interpreter is not ported:
	// the phantom points are then the rounded points. For a glyph without
	// instructions the phantom points are the rounded points also when the
	// mode is off. With the mode on the phantom points stay as they are.
	if s.isHinted && !s.backwardCompat {
		for i := range s.phantom {
			p := scaled[phantomStart+i]
			s.phantom[i] = ahFPoint{ahPixRound(p.x), ahPixRound(p.y)}
		}
	}
	if pointsStart != 0 {
		// The end points of a component count from the first point of the
		// glyph.
		for i := range contours {
			contours[i] += uint16(pointsStart)
		}
	}
	return true
}

// loadComposite is load_composite.
func (s *ahGlyfScaler) loadComposite(glyph ahData, gid uint32, recurseDepth int) bool {
	pointBase := s.pointCount
	components := ahComponents(glyph)
	// The deltas of the offsets of the components and of the phantom
	// points, for an instance of a variable font. A composite glyph can
	// have a composite glyph as a component, so the deltas are on a stack.
	haveDeltas := false
	deltaBase := s.componentDeltaCount
	if gvar := s.gvar(); gvar != nil {
		count := len(components) + ahPhantomPointCount
		if deltaBase+count > len(s.compositeDeltas) {
			return false
		}
		deltas := s.compositeDeltas[deltaBase : deltaBase+count]
		if gvar.compositeGlyph(gid, s.coords, deltas) {
			// The deltas of the phantom points, rounded to whole font
			// units.
			for i := range s.phantom {
				delta := deltas[len(deltas)-ahPhantomPointCount+i]
				s.phantom[i].x += ahFixedToI32(delta.x)
				s.phantom[i].y += ahFixedToI32(delta.y)
			}
			haveDeltas = true
		}
		s.componentDeltaCount += count
	}
	for i := range s.phantom {
		s.phantom[i] = s.applyPoint(s.phantom[i])
	}
	for i, component := range components {
		// The component changes the phantom points. They come back,
		// unless the component has the flag USE_MY_METRICS.
		phantom := s.phantom
		startPoint := s.pointCount
		componentGlyph, ok := s.font.getGlyf(uint32(component.glyph))
		if !ok {
			return false
		}
		if !s.load(componentGlyph, uint32(component.glyph), recurseDepth+1) {
			return false
		}
		endPoint := s.pointCount
		if component.flags&ahCompUseMyMetrics == 0 {
			s.phantom = phantom
		}
		// The transform is 2.14 in the font and 16.16 here.
		xx, yx := int32(component.xx)*4, int32(component.yx)*4
		xy, yy := int32(component.xy)*4, int32(component.yy)*4
		haveXform := component.flags&(ahCompWeHaveAScale|ahCompWeHaveAnXAndYScale|ahCompWeHaveATwoByTwo) != 0
		if haveXform {
			for k := startPoint; k < endPoint; k++ {
				if s.isScaled {
					px, py := s.scaled[k].x, s.scaled[k].y
					s.scaled[k] = ahFPoint{
						ahFixedMul(px, xx) + ahFixedMul(py, xy),
						ahFixedMul(px, yx) + ahFixedMul(py, yy),
					}
					continue
				}
				// Without a size the point goes back to whole font units,
				// and the result is in whole font units.
				ux, uy := (s.scaled[k].x+32)>>6, (s.scaled[k].y+32)>>6
				x := ahFixedMul(ux, xx) + ahFixedMul(uy, xy)
				y := ahFixedMul(ux, yx) + ahFixedMul(uy, yy)
				s.scaled[k] = ahFPoint{x << 6, y << 6}
			}
		}
		var anchorOffset ahFPoint
		if component.anchorIsOffset {
			x, y := int32(component.x), int32(component.y)
			if haveXform && component.flags&(ahCompScaledComponentOffset|ahCompUnscaledComponentOffset) == ahCompScaledComponentOffset {
				// The length of a column of the transform, as FreeType
				// estimates it.
				hypot := func(a, b int32) int32 {
					a, b = ahAbs(a), ahAbs(b)
					if a > b {
						return a + ((3 * b) >> 3)
					}
					return b + ((3 * a) >> 3)
				}
				x = ahFixedMul(x, hypot(xx, xy))
				y = ahFixedMul(y, hypot(yy, yx))
			}
			if haveDeltas {
				// The delta of the offset, rounded to whole font units as
				// in FreeType. A delta outside of the stack is zero.
				var delta ahFPoint
				if ix := deltaBase + i; ix < len(s.compositeDeltas) {
					delta = s.compositeDeltas[ix]
				}
				x += ahFixedToI32(delta.x)
				y += ahFixedToI32(delta.y)
			}
			// Without hinting the offset is not rounded.
			anchorOffset = s.applyPoint(ahFPoint{x, y})
			if s.isHinted && component.flags&ahCompRoundXYToGrid != 0 {
				// The hinted mode rounds y only, as FreeType does.
				anchorOffset.y = ahPixRound(anchorOffset.y)
			}
		} else {
			// The anchor is a pair of point numbers: a point of the
			// component goes to a point of the glyph so far.
			baseIx := pointBase + int(component.base)
			componentIx := startPoint + int(component.component)
			if baseIx >= len(s.scaled) || componentIx >= len(s.scaled) {
				return false
			}
			basePoint, componentPoint := s.scaled[baseIx], s.scaled[componentIx]
			anchorOffset = ahFPoint{basePoint.x - componentPoint.x, basePoint.y - componentPoint.y}
		}
		if anchorOffset.x != 0 || anchorOffset.y != 0 {
			for k := startPoint; k < endPoint; k++ {
				s.scaled[k].x += anchorOffset.x
				s.scaled[k].y += anchorOffset.y
			}
		}
	}
	if haveDeltas {
		s.componentDeltaCount = deltaBase
	}
	// The hinted mode. For a composite glyph with instructions skrifa
	// rounds the phantom points and runs the program of the glyph, and it
	// takes the phantom points of the program when the backward
	// compatibility mode is off. The interpreter is not ported: the
	// phantom points are then the rounded points. A composite glyph
	// without instructions keeps its phantom points.
	if s.isHinted && !s.backwardCompat {
		if _, instructions := ahCountAndInstructions(glyph); len(instructions) != 0 {
			for i := range s.phantom {
				s.phantom[i] = ahFPoint{ahPixRound(s.phantom[i].x), ahPixRound(s.phantom[i].y)}
			}
		}
	}
	return true
}

// scaleOutline is FreeTypeScaler::unhinted and FreeTypeScaler::scale: it
// loads the outline of a glyph. The outline is moved by the x of the first
// phantom point, so that the origin of the glyph is at that point
// (ScaledOutline::new).
func (f *ahFont) scaleOutline(o ahGlyfOutline, scale int32, isScaled bool) (*ahGlyfScaler, bool) {
	if o.points > ahMaxGlyfPoints {
		return nil, false
	}
	s := &ahGlyfScaler{
		font:     f,
		scale:    scale,
		isScaled: isScaled,
		scaled:   make([]ahFPoint, o.points),
		flags:    make([]uint8, o.points),
		contours: make([]uint16, o.contours),
	}
	if !s.load(o.glyph, o.glyphID, 0) {
		return nil, false
	}
	if xShift := s.phantom[0].x; xShift != 0 {
		for i := 0; i < s.pointCount; i++ {
			s.scaled[i].x -= xShift
		}
	}
	return s, true
}

// drawUnscaled is OutlineGlyph::draw_unscaled for a TrueType outline: the
// points of the outline in font units, and the advance of the glyph.
func (f *ahFont) drawUnscaled(o ahGlyfOutline) (points []ahUnscaledPoint, advance int32, ok bool) {
	s, ok := f.scaleOutline(o, 0x10000, false)
	if !ok {
		return nil, 0, false
	}
	scaled := s.scaled[:s.pointCount]
	points = make([]ahUnscaledPoint, 0, len(scaled))
	contourStart := 0
	for _, c := range s.contours[:s.contourCount] {
		contourEnd := int(c)
		if contourEnd >= contourStart && contourEnd < len(scaled) {
			for ix := contourStart; ix <= contourEnd; ix++ {
				points = append(points, ahUnscaledPoint{
					x:              int16(scaled[ix].x >> 6),
					y:              int16(scaled[ix].y >> 6),
					flags:          s.flags[ix] & ahPointCurveMask,
					isContourStart: ix == contourStart,
				})
			}
		}
		contourStart = contourEnd + 1
	}
	return points, (s.phantom[1].x - s.phantom[0].x) >> 6, true
}

// ahPathBounds accumulates the bounds of the points of a path that skrifa
// and the pen of Skia make from the contours of an outline
// (src/outline/path.rs and VerbsPointsPen of Skia), in 26.6 px with y up. The
// control points count, as in SkPath::getBounds.
//
// The pen of Skia starts a contour at the first segment that moves, so a
// contour whose points are all at one position adds nothing to the path.
// Every other contour has all its points in the path: the points that the
// path adds between two control points are inside the bounds of the two.
type ahPathBounds struct {
	box ftCBox
}

// addContour adds the n points of a contour. at returns a point of the
// contour.
func (b *ahPathBounds) addContour(n int, at func(i int) (x, y int32)) {
	if n == 0 {
		return
	}
	x0, y0 := at(0)
	moves := false
	for i := 1; i < n; i++ {
		if x, y := at(i); x != x0 || y != y0 {
			moves = true
			break
		}
	}
	if !moves {
		return
	}
	for i := 0; i < n; i++ {
		px, py := at(i)
		x, y := int64(px), int64(py)
		if !b.box.ok {
			b.box = ftCBox{xMin: x, yMin: y, xMax: x, yMax: y, ok: true}
			continue
		}
		b.box.xMin, b.box.xMax = min(b.box.xMin, x), max(b.box.xMax, x)
		b.box.yMin, b.box.yMax = min(b.box.yMin, y), max(b.box.yMax, y)
	}
}

// unhintedPathBounds is OutlineGlyph::draw_unhinted with the pen of Skia:
// the bounds of the points of the path of the outline of a glyph at a font
// size without hinting. The box is not ok for a glyph without a path.
func (f *ahFont) unhintedPathBounds(o ahGlyfOutline, size float32) ftCBox {
	s, ok := f.unhintedOutline(o, size)
	if !ok {
		return ftCBox{}
	}
	points := s.scaled[:s.pointCount]
	var bounds ahPathBounds
	// to_path: a contour that ends before it starts, or after the last
	// point, is an error, and then the glyph has no path.
	startIx := 0
	for _, c := range s.contours[:s.contourCount] {
		endIx := int(c)
		if endIx < startIx || endIx >= len(points) {
			return ftCBox{}
		}
		contour := points[startIx : endIx+1]
		bounds.addContour(len(contour), func(i int) (int32, int32) { return contour[i].x, contour[i].y })
		startIx = endIx + 1
	}
	return bounds.box
}

// Flags of the points of a simple glyph in the glyf table.
const (
	glyfXShort = 0x02
	glyfYShort = 0x04
	glyfRepeat = 0x08
	glyfXSame  = 0x10
	glyfYSame  = 0x20
)
