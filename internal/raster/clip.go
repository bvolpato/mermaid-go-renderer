// Clipping with a coverage mask: SkAAClip and SkAAClipBlitter of
// skia/src/core/SkAAClip.cpp (Skia branch chrome/m156). Skia stores the
// clip as run-length encoded rows. This port keeps a dense mask. The
// blitter makes the same calls on the wrapped blitter.

package raster

// aaClip is a clip with coverage: SkAAClip. A clip without anti-aliasing
// that is not a rectangle (SkRegion) uses the same type with the values 0
// and 255.
type aaClip struct {
	bounds IRect
	stride int
	mask   []uint8
}

func newAAClip(bounds IRect) *aaClip {
	w, h := bounds.width(), bounds.height()
	return &aaClip{bounds: bounds, stride: w, mask: make([]uint8, w*h)}
}

// rowAt returns the coverage of row y. Index 0 is the left edge of the
// bounds.
func (c *aaClip) rowAt(y int) []uint8 {
	o := (y - c.bounds.T) * c.stride
	return c.mask[o : o+c.stride]
}

func (c *aaClip) at(x, y int) uint8 {
	if x < c.bounds.L || x >= c.bounds.R || y < c.bounds.T || y >= c.bounds.B {
		return 0
	}
	return c.mask[(y-c.bounds.T)*c.stride+(x-c.bounds.L)]
}

// quickContains reports whether the clip has full coverage over a
// rectangle (SkAAClip::quickContains).
func (c *aaClip) quickContains(r IRect) bool {
	if r.isEmpty() || !c.bounds.contains(r) {
		return false
	}
	for y := r.T; y < r.B; y++ {
		for _, v := range c.rowAt(y)[r.L-c.bounds.L : r.R-c.bounds.L] {
			if v != 0xFF {
				return false
			}
		}
	}
	return true
}

// intersect returns the product of two clips.
func (c *aaClip) intersect(o *aaClip) *aaClip {
	b := c.bounds
	if !b.intersect(o.bounds) {
		return newAAClip(IRect{})
	}
	out := newAAClip(b)
	for y := b.T; y < b.B; y++ {
		ra := c.rowAt(y)[b.L-c.bounds.L:]
		rb := o.rowAt(y)[b.L-o.bounds.L:]
		dst := out.rowAt(y)
		for i := range dst {
			dst[i] = uint8(mulDiv255Round(uint32(ra[i]), uint32(rb[i])))
		}
	}
	return out
}

// aaClipBlitter is SkAAClipBlitter.
type aaClipBlitter struct {
	b    blitter
	clip *aaClip
	runs []int16
	aa   []uint8
	tmp  maskA8
}

func (c *aaClipBlitter) init(b blitter, clip *aaClip) {
	c.b, c.clip = b, clip
	need := clip.bounds.width() + 2
	if cap(c.runs) < need {
		c.runs = make([]int16, need)
		c.aa = make([]uint8, need)
	}
}

// span returns the clip coverage of [x, x+width) on row y, or nil when the
// row is outside the clip.
func (c *aaClipBlitter) span(x, y, width int) []uint8 {
	b := c.clip.bounds
	if y < b.T || y >= b.B || x < b.L || x+width > b.R {
		return nil
	}
	o := (y-b.T)*c.clip.stride + (x - b.L)
	return c.clip.mask[o : o+width]
}

// clampSpan cuts a span to the clip bounds.
func (c *aaClipBlitter) clampSpan(x, width int) (int, int) {
	b := c.clip.bounds
	if x < b.L {
		width -= b.L - x
		x = b.L
	}
	if x+width > b.R {
		width = b.R - x
	}
	return x, width
}

func uniform(s []uint8) (uint8, bool) {
	v := s[0]
	for _, x := range s[1:] {
		if x != v {
			return 0, false
		}
	}
	return v, true
}

func (c *aaClipBlitter) blitH(x, y, width int) {
	x, width = c.clampSpan(x, width)
	if width <= 0 {
		return
	}
	row := c.span(x, y, width)
	if row == nil {
		return
	}
	if v, ok := uniform(row); ok {
		if v == 0 {
			return
		}
		if v == 0xFF {
			c.b.blitH(x, y, width)
			return
		}
	}
	// expandToRuns.
	runs, aa := c.runs[:width+1], c.aa[:width+1]
	for i := 0; i < width; {
		v := row[i]
		n := 1
		for i+n < width && row[i+n] == v {
			n++
		}
		runs[i] = int16(n)
		aa[i] = v
		i += n
	}
	runs[width] = 0
	c.b.blitAntiH(x, y, aa, runs)
}

func (c *aaClipBlitter) blitAntiH(x, y int, srcAA []uint8, srcRuns []int16) {
	b := c.clip.bounds
	if y < b.T || y >= b.B {
		return
	}
	runs, aa := c.runs, c.aa
	crow := c.clip.rowAt(y)
	outX, end := -1, 0
	// merge: the product of the coverage and the clip, in runs.
	for i := 0; ; {
		n := int(srcRuns[i])
		if n <= 0 {
			break
		}
		a := uint32(srcAA[i])
		s0, s1 := max(x+i, b.L), min(x+i+n, b.R)
		if s0 < s1 {
			if outX < 0 {
				outX = s0
			}
			for k := s0; k < s1; {
				cv := crow[k-b.L]
				m := 1
				for k+m < s1 && crow[k+m-b.L] == cv {
					m++
				}
				runs[k-outX] = int16(m)
				aa[k-outX] = uint8(mulDiv255Round(a, uint32(cv)))
				k += m
			}
			end = s1
		}
		i += n
	}
	if outX >= 0 {
		runs[end-outX] = 0
		c.b.blitAntiH(outX, y, aa, runs)
	}
}

func (c *aaClipBlitter) blitV(x, y, height int, alpha uint8) {
	b := c.clip.bounds
	if x < b.L || x >= b.R {
		return
	}
	y0, y1 := max(y, b.T), min(y+height, b.B)
	if y0 >= y1 {
		return
	}
	full := true
	for yy := y0; yy < y1; yy++ {
		if c.clip.at(x, yy) != 0xFF {
			full = false
			break
		}
	}
	if full {
		c.b.blitV(x, y0, y1-y0, alpha)
		return
	}
	for yy := y0; yy < y1; {
		cv := c.clip.at(x, yy)
		n := 1
		for yy+n < y1 && c.clip.at(x, yy+n) == cv {
			n++
		}
		if newAlpha := uint8(mulDiv255Round(uint32(alpha), uint32(cv))); newAlpha != 0 {
			c.b.blitV(x, yy, n, newAlpha)
		}
		yy += n
	}
}

func (c *aaClipBlitter) blitRect(x, y, width, height int) {
	r := IRect{x, y, x + width, y + height}
	if !r.intersect(c.clip.bounds) {
		return
	}
	full := true
	for yy := r.T; yy < r.B && full; yy++ {
		row := c.span(r.L, yy, r.width())
		for _, v := range row {
			if v != 0xFF {
				full = false
				break
			}
		}
	}
	if full {
		c.b.blitRect(r.L, r.T, r.width(), r.height())
		return
	}
	for yy := r.T; yy < r.B; yy++ {
		c.blitH(r.L, yy, r.width())
	}
}

func (c *aaClipBlitter) blitAntiRect(x, y, width, height int, leftAlpha, rightAlpha uint8) {
	defaultBlitAntiRect(c, x, y, width, height, leftAlpha, rightAlpha)
}

func (c *aaClipBlitter) blitMask(m *maskA8, clip IRect) {
	r := clip
	if !r.intersect(c.clip.bounds) {
		return
	}
	w, h := r.width(), r.height()
	if cap(c.tmp.image) < w*h {
		c.tmp.image = make([]uint8, w*h)
	}
	c.tmp.image = c.tmp.image[:w*h]
	c.tmp.bounds = r
	c.tmp.rowB = w
	for yy := r.T; yy < r.B; yy++ {
		src := m.row(yy)[r.L-m.bounds.L:]
		crow := c.span(r.L, yy, w)
		dst := c.tmp.image[(yy-r.T)*w : (yy-r.T+1)*w]
		for i := range dst {
			switch cv := crow[i]; cv {
			case 0xFF:
				dst[i] = src[i]
			case 0:
				dst[i] = 0
			default:
				dst[i] = uint8(mulDiv255Round(uint32(src[i]), uint32(cv)))
			}
		}
	}
	// Skia blits the merged mask one row at a time.
	for yy := r.T; yy < r.B; yy++ {
		c.b.blitMask(&c.tmp, IRect{r.L, yy, r.R, yy + 1})
	}
}

func (c *aaClipBlitter) blitAntiH2(x, y int, a0, a1 uint8) { defaultBlitAntiH2(c, x, y, a0, a1) }
func (c *aaClipBlitter) blitAntiV2(x, y int, a0, a1 uint8) { defaultBlitAntiV2(c, x, y, a0, a1) }

// maskBuilderBlitter writes coverage into a clip mask: the blitter of
// SkAAClip::Builder. A scan converter draws into it to turn a path into a
// clip.
type maskBuilderBlitter struct {
	clip *aaClip
}

func (m *maskBuilderBlitter) set(x, y, width int, alpha uint8) {
	b := m.clip.bounds
	if y < b.T || y >= b.B {
		return
	}
	x0, x1 := max(x, b.L), min(x+width, b.R)
	if x0 >= x1 {
		return
	}
	row := m.clip.rowAt(y)[x0-b.L : x1-b.L]
	for i := range row {
		// Coverage accumulates as in a source-over blit of white.
		v := uint32(row[i]) + uint32(alpha) - mulDiv255Round(uint32(row[i]), uint32(alpha))
		if v > 255 {
			v = 255
		}
		row[i] = uint8(v)
	}
}

func (m *maskBuilderBlitter) blitH(x, y, width int) { m.set(x, y, width, 0xFF) }
func (m *maskBuilderBlitter) blitAntiH(x, y int, aa []uint8, runs []int16) {
	i := 0
	for {
		n := int(runs[i])
		if n <= 0 {
			return
		}
		if aa[i] != 0 {
			m.set(x+i, y, n, aa[i])
		}
		i += n
	}
}
func (m *maskBuilderBlitter) blitV(x, y, height int, alpha uint8) {
	for i := 0; i < height; i++ {
		m.set(x, y+i, 1, alpha)
	}
}
func (m *maskBuilderBlitter) blitRect(x, y, width, height int) {
	for i := 0; i < height; i++ {
		m.set(x, y+i, width, 0xFF)
	}
}
func (m *maskBuilderBlitter) blitAntiRect(x, y, width, height int, l, r uint8) {
	defaultBlitAntiRect(m, x, y, width, height, l, r)
}
func (m *maskBuilderBlitter) blitMask(mk *maskA8, clip IRect) {
	for y := clip.T; y < clip.B; y++ {
		row := mk.row(y)[clip.L-mk.bounds.L:]
		for x := clip.L; x < clip.R; x++ {
			if v := row[x-clip.L]; v != 0 {
				m.set(x, y, 1, v)
			}
		}
	}
}
func (m *maskBuilderBlitter) blitAntiH2(x, y int, a0, a1 uint8) { defaultBlitAntiH2(m, x, y, a0, a1) }
func (m *maskBuilderBlitter) blitAntiV2(x, y int, a0, a1 uint8) { defaultBlitAntiV2(m, x, y, a0, a1) }

// antiFillPathAA is SkScan::AntiFillPath for a clip with coverage: the scan
// converter must blit in runs (forceRLE).
func (s *aaaState) antiFillPathAA(p *Path, clip IRect, b blitter) {
	if clip.isEmpty() || p.isEmpty() {
		return
	}
	ir := safeRoundOut(p.bounds())
	if ir.isEmpty() {
		return
	}
	clipped := ir
	if !clipped.intersect(clip) {
		return
	}
	if !clip.contains(ir) && (clip.L > ir.L || clip.R < ir.R) {
		s.rectClip = rectClipBlitter{b: b, clip: clip}
		b = &s.rectClip
	}
	s.aaaFillPath(p, b, ir, clip, true)
}
