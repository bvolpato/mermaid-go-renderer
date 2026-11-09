// Ported from skrifa 0.46.2 (src/outline/mod.rs: draw_unscaled and
// draw_unhinted, src/outline/unscaled.rs, src/outline/glyf/mod.rs:
// Outlines::outline and FreeTypeScaler without hinting,
// src/outline/glyf/outline.rs, src/outline/glyf/memory.rs,
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
}

func newAHSimpleGlyph(data ahData) ahSimpleGlyph {
	numberOfContours := int(data.i16(0))
	var g ahSimpleGlyph
	g.endPtsOfContours = data.array(10, numberOfContours, 2)
	off := 10 + 2*numberOfContours
	instructionLength := int(data.u16(off))
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
		if !f.outlineRec(glyph, &o, 0, &totalComponents) {
			return o, false
		}
	}
	o.points += ahPhantomPointCount
	o.glyph = glyph
	return o, true
}

// outlineRec is Outlines::outline_rec.
func (f *ahFont) outlineRec(glyph ahData, o *ahGlyfOutline, recurseDepth int, totalComponents *int) bool {
	if recurseDepth > ahGlyfCompositeRecursionLimit {
		return false
	}
	if glyph.i16(0) >= 0 {
		simple := newAHSimpleGlyph(glyph)
		o.points += simple.numPoints()
		if o.points > ahMaxGlyfPoints {
			return false
		}
		o.contours += len(simple.endPtsOfContours) / 2
		return true
	}
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
		if !f.outlineRec(componentGlyph, o, recurseDepth+1, totalComponents) {
			return false
		}
	}
	return true
}

// ahGlyfScaler is FreeTypeScaler of skrifa without hinting. The points are
// 26.6 numbers. Without a size (isScaled is false) the outline is in font
// units, which is what the automatic hinter starts from.
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
		// load_empty.
		for i := range s.phantom {
			s.phantom[i] = s.applyPoint(s.phantom[i])
		}
		return true
	case glyph.i16(0) >= 0:
		return s.loadSimple(newAHSimpleGlyph(glyph))
	}
	return s.loadComposite(glyph, recurseDepth)
}

// loadSimple is load_simple.
func (s *ahGlyfScaler) loadSimple(glyph ahSimpleGlyph) bool {
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
	// An outline without a size is in 26.6 also.
	for i := range scaled {
		scaled[i] = s.applyPoint(unscaled[i])
	}
	copy(s.phantom[:], scaled[phantomStart:])
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
func (s *ahGlyfScaler) loadComposite(glyph ahData, recurseDepth int) bool {
	pointBase := s.pointCount
	for i := range s.phantom {
		s.phantom[i] = s.applyPoint(s.phantom[i])
	}
	for _, component := range ahComponents(glyph) {
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
			for i := startPoint; i < endPoint; i++ {
				if s.isScaled {
					px, py := s.scaled[i].x, s.scaled[i].y
					s.scaled[i] = ahFPoint{
						ahFixedMul(px, xx) + ahFixedMul(py, xy),
						ahFixedMul(px, yx) + ahFixedMul(py, yy),
					}
					continue
				}
				// Without a size the point goes back to whole font units,
				// and the result is in whole font units.
				ux, uy := (s.scaled[i].x+32)>>6, (s.scaled[i].y+32)>>6
				x := ahFixedMul(ux, xx) + ahFixedMul(uy, xy)
				y := ahFixedMul(ux, yx) + ahFixedMul(uy, yy)
				s.scaled[i] = ahFPoint{x << 6, y << 6}
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
			// Without hinting the offset is not rounded.
			anchorOffset = s.applyPoint(ahFPoint{x, y})
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
			for i := startPoint; i < endPoint; i++ {
				s.scaled[i].x += anchorOffset.x
				s.scaled[i].y += anchorOffset.y
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
