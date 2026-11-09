// Ported from skia/src/core/SkBlitter.h, SkBlitter.cpp, SkBlitter_ARGB32.cpp,
// SkColorData.h, SkColorPriv.h, src/opts/SkBlitRow_opts.h and
// src/opts/SkBlitMask_opts.h (Skia branch chrome/m156).

package raster

// pixmap is a premultiplied RGBA bitmap: four bytes per pixel in the order
// R, G, B, A. It has the layout of image.RGBA.
type pixmap struct {
	pix    []uint8
	stride int
	w, h   int
}

func newPixmap(w, h int) *pixmap {
	return &pixmap{pix: make([]uint8, w*h*4), stride: w * 4, w: w, h: h}
}

func (p *pixmap) bounds() IRect { return IRect{0, 0, p.w, p.h} }

// maskA8 is SkMask in the A8 format.
type maskA8 struct {
	image  []uint8
	bounds IRect
	rowB   int
}

func (m *maskA8) row(y int) []uint8 {
	o := (y - m.bounds.T) * m.rowB
	return m.image[o : o+m.bounds.width()]
}

// blitter is SkBlitter. Runs are run-length encoded: runs[i] is the length
// of the run that starts at i, and a zero ends the scanline. aa[i] is the
// coverage of that run.
type blitter interface {
	blitH(x, y, width int)
	blitAntiH(x, y int, aa []uint8, runs []int16)
	blitV(x, y, height int, alpha uint8)
	blitRect(x, y, width, height int)
	blitAntiRect(x, y, width, height int, leftAlpha, rightAlpha uint8)
	blitMask(m *maskA8, clip IRect)
	blitAntiH2(x, y int, a0, a1 uint8)
	blitAntiV2(x, y int, a0, a1 uint8)
}

// ---- default implementations of SkBlitter ----

func defaultBlitAntiRect(b blitter, x, y, width, height int, leftAlpha, rightAlpha uint8) {
	if leftAlpha > 0 {
		b.blitV(x, y, height, leftAlpha)
	}
	x++
	if width > 0 {
		b.blitRect(x, y, width, height)
		x += width
	}
	if rightAlpha > 0 {
		b.blitV(x, y, height, rightAlpha)
	}
}

func defaultBlitAntiH2(b blitter, x, y int, a0, a1 uint8) {
	runs := [3]int16{1, 1, 0}
	aa := [2]uint8{a0, a1}
	b.blitAntiH(x, y, aa[:], runs[:])
}

func defaultBlitAntiV2(b blitter, x, y int, a0, a1 uint8) {
	runs := [2]int16{1, 0}
	aa := [1]uint8{a0}
	b.blitAntiH(x, y, aa[:], runs[:])
	runs[0], runs[1] = 1, 0
	aa[0] = a1
	b.blitAntiH(x, y+1, aa[:], runs[:])
}

func scalarToAlpha(a float32) uint8 {
	alpha := uint8(floatToInt(float32(a * 255)))
	if alpha > 247 {
		return 0xFF
	}
	if alpha < 8 {
		return 0
	}
	return alpha
}

// blitFatAntiRect is SkBlitter::blitFatAntiRect.
func blitFatAntiRect(b blitter, rect Rect, scratch *runScratch) {
	bounds := rect.roundOut()
	if bounds.height() == 0 {
		return
	}
	w := bounds.width()
	runs, alphas := scratch.get(w + 1)
	runs[0] = 1
	runs[1] = int16(w - 2)
	runs[w-1] = 1
	runs[w] = 0

	partialL := float32(bounds.L+1) - rect.L
	partialR := rect.R - float32(bounds.R-1)
	partialT := float32(bounds.T+1) - rect.T
	partialB := rect.B - float32(bounds.B-1)
	if bounds.height() == 1 {
		partialT = rect.B - rect.T
	}
	alphas[0] = scalarToAlpha(float32(partialL * partialT))
	alphas[1] = scalarToAlpha(partialT)
	alphas[w-1] = scalarToAlpha(float32(partialR * partialT))
	b.blitAntiH(bounds.L, bounds.T, alphas, runs)

	if bounds.height() > 2 {
		b.blitAntiRect(bounds.L, bounds.T+1, w-2, bounds.height()-2, scalarToAlpha(partialL), scalarToAlpha(partialR))
	}
	if bounds.height() > 1 {
		// The clip blitter may have changed the runs.
		runs[0] = 1
		runs[1] = int16(w - 2)
		runs[w-1] = 1
		runs[w] = 0
		alphas[0] = scalarToAlpha(float32(partialL * partialB))
		alphas[1] = scalarToAlpha(partialB)
		alphas[w-1] = scalarToAlpha(float32(partialR * partialB))
		b.blitAntiH(bounds.L, bounds.B-1, alphas, runs)
	}
}

// runScratch holds reusable run and alpha buffers.
type runScratch struct {
	runs   []int16
	alphas []uint8
}

func (s *runScratch) get(n int) ([]int16, []uint8) {
	if cap(s.runs) < n {
		s.runs = make([]int16, n*2)
		s.alphas = make([]uint8, n*2)
	}
	return s.runs[:n], s.alphas[:n]
}

// ---- SkRectClipBlitter ----

type rectClipBlitter struct {
	b    blitter
	clip IRect
}

func computeAntiWidth(runs []int16) int {
	width := 0
	for {
		count := int(runs[width])
		if count == 0 {
			break
		}
		width += count
	}
	return width
}

// breakRunsAt is SkAlphaRuns::BreakAt.
func breakRunsAt(runs []int16, alpha []uint8, x int) {
	i := 0
	for x > 0 {
		n := int(runs[i])
		if x < n {
			alpha[i+x] = alpha[i]
			runs[i] = int16(x)
			runs[i+x] = int16(n - x)
			break
		}
		i += n
		x -= n
	}
}

func (c *rectClipBlitter) blitH(left, y, width int) {
	if y < c.clip.T || y >= c.clip.B {
		return
	}
	right := left + width
	left = max(left, c.clip.L)
	right = min(right, c.clip.R)
	if right > left {
		c.b.blitH(left, y, right-left)
	}
}

func (c *rectClipBlitter) blitAntiH(left, y int, aa []uint8, runs []int16) {
	if y < c.clip.T || y >= c.clip.B || left >= c.clip.R {
		return
	}
	x0 := left
	x1 := left + computeAntiWidth(runs)
	if x1 <= c.clip.L {
		return
	}
	if x0 < c.clip.L {
		dx := c.clip.L - x0
		breakRunsAt(runs, aa, dx)
		runs = runs[dx:]
		aa = aa[dx:]
		x0 = c.clip.L
	}
	if x1 > c.clip.R {
		x1 = c.clip.R
		breakRunsAt(runs, aa, x1-x0)
		runs[x1-x0] = 0
	}
	c.b.blitAntiH(x0, y, aa, runs)
}

func (c *rectClipBlitter) blitV(x, y, height int, alpha uint8) {
	if x < c.clip.L || x >= c.clip.R {
		return
	}
	y0, y1 := max(y, c.clip.T), min(y+height, c.clip.B)
	if y0 < y1 {
		c.b.blitV(x, y0, y1-y0, alpha)
	}
}

func (c *rectClipBlitter) blitRect(left, y, width, height int) {
	r := IRect{left, y, left + width, y + height}
	if r.intersect(c.clip) {
		c.b.blitRect(r.L, r.T, r.width(), r.height())
	}
}

func (c *rectClipBlitter) blitAntiRect(left, y, width, height int, leftAlpha, rightAlpha uint8) {
	r := IRect{left, y, left + width + 2, y + height}
	if r.intersect(c.clip) {
		if r.L != left {
			leftAlpha = 255
		}
		if r.R != left+width+2 {
			rightAlpha = 255
		}
		if leftAlpha == 255 && rightAlpha == 255 {
			c.b.blitRect(r.L, r.T, r.width(), r.height())
		} else if r.width() == 1 {
			if r.L == left {
				c.b.blitV(r.L, r.T, r.height(), leftAlpha)
			} else {
				c.b.blitV(r.L, r.T, r.height(), rightAlpha)
			}
		} else {
			c.b.blitAntiRect(r.L, r.T, r.width()-2, r.height(), leftAlpha, rightAlpha)
		}
	}
}

func (c *rectClipBlitter) blitMask(m *maskA8, clip IRect) {
	r := clip
	if r.intersect(c.clip) {
		c.b.blitMask(m, r)
	}
}

func (c *rectClipBlitter) blitAntiH2(x, y int, a0, a1 uint8) { defaultBlitAntiH2(c, x, y, a0, a1) }
func (c *rectClipBlitter) blitAntiV2(x, y int, a0, a1 uint8) { defaultBlitAntiV2(c, x, y, a0, a1) }

// ---- pixel math ----

// mulDiv255Round is SkMulDiv255Round.
func mulDiv255Round(a, b uint32) uint32 {
	prod := a*b + 128
	return (prod + (prod >> 8)) >> 8
}

// premul is a premultiplied color, one value per channel.
type premul struct{ r, g, b, a uint8 }

// premultiply is SkPreMultiplyColor.
func premultiply(r, g, b, a uint8) premul {
	if a != 255 {
		r = uint8(mulDiv255Round(uint32(r), uint32(a)))
		g = uint8(mulDiv255Round(uint32(g), uint32(a)))
		b = uint8(mulDiv255Round(uint32(b), uint32(a)))
	}
	return premul{r, g, b, a}
}

// alphaMulQ is SkAlphaMulQ on a color: each channel is (c * scale) >> 8.
func (c premul) alphaMulQ(scale uint32) premul {
	return premul{
		uint8(uint32(c.r) * scale >> 8),
		uint8(uint32(c.g) * scale >> 8),
		uint8(uint32(c.b) * scale >> 8),
		uint8(uint32(c.a) * scale >> 8),
	}
}

// color32 is SkBlitRow::Color32: it blends the premultiplied color over
// count pixels.
func color32(dst []uint8, count int, c premul) {
	switch c.a {
	case 0:
		return
	case 255:
		fill32(dst, count, c)
		return
	}
	invA := uint32(256 - uint32(c.a))
	d := dst[: count*4 : count*4]
	for i := 0; i+3 < len(d); i += 4 {
		d[i] = uint8(uint32(d[i])*invA>>8) + c.r
		d[i+1] = uint8(uint32(d[i+1])*invA>>8) + c.g
		d[i+2] = uint8(uint32(d[i+2])*invA>>8) + c.b
		d[i+3] = uint8(uint32(d[i+3])*invA>>8) + c.a
	}
}

// fill32 is SkOpts::memset32.
func fill32(dst []uint8, count int, c premul) {
	if count <= 0 {
		return
	}
	d := dst[: count*4 : count*4]
	d[0], d[1], d[2], d[3] = c.r, c.g, c.b, c.a
	for n := 4; n < len(d); n *= 2 {
		copy(d[n:], d[:n])
	}
}

// blendARGB32 is SkBlendARGB32 on one pixel.
func blendARGB32(d []uint8, src premul, aa uint32) {
	srcScale := aa + 1
	prod := 0xFFFF - uint32(src.a)*srcScale
	dstScale := (prod + (prod >> 8)) >> 8
	d[0] = uint8((uint32(src.r)*srcScale + uint32(d[0])*dstScale) >> 8)
	d[1] = uint8((uint32(src.g)*srcScale + uint32(d[1])*dstScale) >> 8)
	d[2] = uint8((uint32(src.b)*srcScale + uint32(d[2])*dstScale) >> 8)
	d[3] = uint8((uint32(src.a)*srcScale + uint32(d[3])*dstScale) >> 8)
}

// fastFourByteInterp is SkFastFourByteInterp on one pixel.
func fastFourByteInterp(d []uint8, src premul, srcWeight uint32) {
	scale := srcWeight + (srcWeight >> 7)
	inv := 256 - scale
	d[0] = uint8((uint32(src.r)*scale + inv*uint32(d[0])) >> 8)
	d[1] = uint8((uint32(src.g)*scale + inv*uint32(d[1])) >> 8)
	d[2] = uint8((uint32(src.b)*scale + inv*uint32(d[2])) >> 8)
	d[3] = uint8((uint32(src.a)*scale + inv*uint32(d[3])) >> 8)
}

// ---- solid color blitters: SkARGB32_Blitter and its subclasses ----

type colorKind uint8

const (
	colorGeneral colorKind = iota // SkARGB32_Blitter: alpha below 255
	colorOpaque                   // SkARGB32_Opaque_Blitter
	colorBlack                    // SkARGB32_Black_Blitter
)

// colorBlitter draws one color with the source-over blend mode.
type colorBlitter struct {
	dst  *pixmap
	pm   premul
	srcA uint8
	kind colorKind
}

// init sets the color from a straight alpha color (SkColor).
func (b *colorBlitter) init(dst *pixmap, r, g, bl, a uint8) {
	b.dst = dst
	b.srcA = a
	b.pm = premultiply(r, g, bl, a)
	switch {
	case a == 255 && r == 0 && g == 0 && bl == 0:
		b.kind = colorBlack
	case a == 255:
		b.kind = colorOpaque
	default:
		b.kind = colorGeneral
	}
}

func (b *colorBlitter) addr(x, y int) []uint8 {
	o := y*b.dst.stride + x*4
	return b.dst.pix[o:]
}

func (b *colorBlitter) blitH(x, y, width int) {
	color32(b.addr(x, y), width, b.pm)
}

func (b *colorBlitter) blitAntiH(x, y int, aa []uint8, runs []int16) {
	if b.kind == colorGeneral && b.srcA == 0 {
		return
	}
	device := b.addr(x, y)
	i := 0
	for {
		count := int(runs[i])
		if count <= 0 {
			return
		}
		a := uint32(aa[i])
		if a != 0 {
			d := device[i*4:]
			switch b.kind {
			case colorBlack:
				if a == 255 {
					fill32(d, count, premul{0, 0, 0, 255})
				} else {
					dstScale := 256 - a
					d = d[: count*4 : count*4]
					for j := 0; j+3 < len(d); j += 4 {
						d[j] = uint8(uint32(d[j]) * dstScale >> 8)
						d[j+1] = uint8(uint32(d[j+1]) * dstScale >> 8)
						d[j+2] = uint8(uint32(d[j+2]) * dstScale >> 8)
						d[j+3] = uint8(a) + uint8(uint32(d[j+3])*dstScale>>8)
					}
				}
			case colorOpaque:
				if a == 255 {
					fill32(d, count, b.pm)
				} else {
					color32(d, count, b.pm.alphaMulQ(a+1))
				}
			default:
				color32(d, count, b.pm.alphaMulQ(a+1))
			}
		}
		i += count
	}
}

func (b *colorBlitter) blitAntiH2(x, y int, a0, a1 uint8) {
	d := b.addr(x, y)
	b.blend1(d, uint32(a0))
	b.blend1(d[4:], uint32(a1))
}

func (b *colorBlitter) blitAntiV2(x, y int, a0, a1 uint8) {
	b.blend1(b.addr(x, y), uint32(a0))
	b.blend1(b.addr(x, y+1), uint32(a1))
}

// blend1 is the per-pixel blend of blitAntiH2 and blitAntiV2.
func (b *colorBlitter) blend1(d []uint8, a uint32) {
	switch b.kind {
	case colorBlack:
		s := 256 - a
		d[0] = uint8(uint32(d[0]) * s >> 8)
		d[1] = uint8(uint32(d[1]) * s >> 8)
		d[2] = uint8(uint32(d[2]) * s >> 8)
		d[3] = uint8(a) + uint8(uint32(d[3])*s>>8)
	case colorOpaque:
		fastFourByteInterp(d, b.pm, a)
	default:
		blendARGB32(d, b.pm, a)
	}
}

func (b *colorBlitter) blitV(x, y, height int, alpha uint8) {
	if alpha == 0 || (b.kind == colorGeneral && b.srcA == 0) {
		return
	}
	c := b.pm
	if alpha != 255 {
		c = c.alphaMulQ(uint32(alpha) + 1)
	}
	dstScale := uint32(256 - uint32(c.a))
	o := y*b.dst.stride + x*4
	for ; height > 0; height-- {
		d := b.dst.pix[o : o+4 : o+4]
		d[0] = c.r + uint8(uint32(d[0])*dstScale>>8)
		d[1] = c.g + uint8(uint32(d[1])*dstScale>>8)
		d[2] = c.b + uint8(uint32(d[2])*dstScale>>8)
		d[3] = c.a + uint8(uint32(d[3])*dstScale>>8)
		o += b.dst.stride
	}
}

func (b *colorBlitter) blitRect(x, y, width, height int) {
	if b.kind == colorGeneral && b.srcA == 0 {
		return
	}
	o := y*b.dst.stride + x*4
	for ; height > 0; height-- {
		color32(b.dst.pix[o:], width, b.pm)
		o += b.dst.stride
	}
}

func (b *colorBlitter) blitAntiRect(x, y, width, height int, leftAlpha, rightAlpha uint8) {
	defaultBlitAntiRect(b, x, y, width, height, leftAlpha, rightAlpha)
}

// blitMask is SkARGB32_Blitter::blitMask for an A8 mask
// (SkOpts::blit_mask_d32_a8).
func (b *colorBlitter) blitMask(m *maskA8, clip IRect) {
	if b.kind == colorGeneral && b.srcA == 0 {
		return
	}
	width := clip.width()
	for y := clip.T; y < clip.B; y++ {
		mrow := m.row(y)[clip.L-m.bounds.L:]
		d := b.addr(clip.L, y)
		d = d[: width*4 : width*4]
		switch b.kind {
		case colorBlack:
			for i := 0; i < width; i++ {
				vmask := uint32(mrow[i])
				s := 256 - vmask
				p := d[i*4 : i*4+4 : i*4+4]
				p[0] = uint8(uint32(p[0]) * s >> 8)
				p[1] = uint8(uint32(p[1]) * s >> 8)
				p[2] = uint8(uint32(p[2]) * s >> 8)
				p[3] = uint8(uint32(p[3])*s>>8) + uint8(vmask)
			}
		case colorOpaque:
			for i := 0; i < width; i++ {
				vmask := uint32(mrow[i])
				m256 := vmask + 1
				s := 256 - vmask
				p := d[i*4 : i*4+4 : i*4+4]
				p[0] = uint8(uint32(b.pm.r)*m256>>8) + uint8(uint32(p[0])*s>>8)
				p[1] = uint8(uint32(b.pm.g)*m256>>8) + uint8(uint32(p[1])*s>>8)
				p[2] = uint8(uint32(b.pm.b)*m256>>8) + uint8(uint32(p[2])*s>>8)
				p[3] = uint8(uint32(b.pm.a)*m256>>8) + uint8(uint32(p[3])*s>>8)
			}
		default:
			ca := uint32(b.pm.a)
			for i := 0; i < width; i++ {
				vmask := uint32(mrow[i])
				m256 := vmask + 1
				s := 256 - (ca * m256 >> 8)
				p := d[i*4 : i*4+4 : i*4+4]
				p[0] = uint8(uint32(b.pm.r)*m256>>8) + uint8(uint32(p[0])*s>>8)
				p[1] = uint8(uint32(b.pm.g)*m256>>8) + uint8(uint32(p[1])*s>>8)
				p[2] = uint8(uint32(b.pm.b)*m256>>8) + uint8(uint32(p[2])*s>>8)
				p[3] = uint8(uint32(b.pm.a)*m256>>8) + uint8(uint32(p[3])*s>>8)
			}
		}
	}
}
