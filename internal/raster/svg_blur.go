// The Gaussian blur of image filters on the CPU. Ported from
// skia/src/core/SkBlurEngine.cpp (Raster8888BlurAlgorithm, eval_blur_passes,
// Pass and ThreeBoxApproxPass) and src/base/SkVx.h (ScaledDividerU32) of
// the Skia branch chrome/m156. Chromium defines
// SK_AVOID_SLOW_RASTER_PIPELINE_BLURS, so every sigma uses three box blurs.

package raster

import "math"

// blurIdentity is SkBlurEngine::IsEffectivelyIdentity.
func blurIdentity(sigma float32) bool { return sigma <= 0.03 }

// sigmaToRadius is SkBlurEngine::SigmaToRadius.
func sigmaToRadius(sigma float32) int {
	if blurIdentity(sigma) {
		return 0
	}
	return int(math.Ceil(float64(float32(3 * sigma))))
}

// boxBlurWindow is SkBlurEngine::BoxBlurWindow.
func boxBlurWindow(sigma float32) int {
	w := int(math.Floor(float64(float64(sigma)*3*math.Sqrt(2*float64(float32(math.Pi)))/4) + 0.5))
	return max(1, w)
}

// boxParams holds the constants of ThreeBoxApproxPass for a window: three
// box blurs in one pass over a line, with three ring buffers for the
// trailing edges.
type boxParams struct {
	// border is the distance in pixels between the first source pixel and
	// the first destination pixel that it reaches.
	border int
	// factor and half are ScaledDividerU32: 2^32 / divisor rounded, and
	// half the divisor.
	factor uint64
	half   uint32
	// bufLen is the length of the three ring buffers together. The second
	// starts at b1 and the third at b2.
	bufLen, b1, b2 int
}

func newBoxParams(window int) boxParams {
	var p boxParams
	passSize := window - 1
	p.bufLen = 3 * passSize
	if window&1 == 0 {
		// An even window has one more element in the last pass.
		p.bufLen++
	}
	p.b1, p.b2 = passSize, 2*passSize
	if window&1 == 1 {
		p.border = 3 * ((window - 1) / 2)
	} else {
		p.border = 3*(window/2) - 1
	}
	w2 := window * window
	w3 := w2 * window
	divisor := w3
	if window&1 == 0 {
		divisor = w3 + w2
	}
	p.factor = uint64(math.Round(1.0 / float64(divisor) * float64(uint64(1)<<32)))
	p.half = uint32(divisor+1) >> 1
	return p
}

// alphaPass is ThreeBoxApproxPass for one channel: a drop shadow needs only
// the blurred alpha. Skia blurs the four channels with the same
// arithmetic.
type alphaPass struct {
	border     int
	factor     uint64
	half       uint32
	buf        []uint32
	b1, b2     int
	c0, c1, c2 int
	s0, s1, s2 uint32
}

func newAlphaPass(window int) *alphaPass {
	ref := newBoxParams(window)
	return &alphaPass{border: ref.border, factor: ref.factor, half: ref.half, buf: make([]uint32, ref.bufLen), b1: ref.b1, b2: ref.b2}
}

func (p *alphaPass) start() {
	p.s0, p.s1, p.s2 = 0, 0, p.half
	clear(p.buf)
	p.c0, p.c1, p.c2 = 0, p.b1, p.b2
}

func (p *alphaPass) process(lead uint32) uint8 {
	p.s0 += lead
	p.s1 += p.s0
	p.s2 += p.s1
	out := uint8((uint64(p.s2) * p.factor) >> 32)
	p.s2 -= p.buf[p.c2]
	p.buf[p.c2] = p.s1
	if p.c2++; p.c2 >= len(p.buf) {
		p.c2 = p.b2
	}
	p.s1 -= p.buf[p.c1]
	p.buf[p.c1] = p.s0
	if p.c1++; p.c1 >= p.b2 {
		p.c1 = p.b1
	}
	p.s0 -= p.buf[p.c0]
	p.buf[p.c0] = lead
	if p.c0++; p.c0 >= p.b1 {
		p.c0 = 0
	}
	return out
}

// blur is Pass::blur for one line of single values. The source has n
// values that start at srcLeft in the coordinates of the destination.
func (p *alphaPass) blur(srcLeft int, src []uint8, srcStride, n int, dst []uint8, dstStride, dstCount int) {
	buf := p.buf
	clear(buf)
	bufLen := len(buf)
	b1, b2 := p.b1, p.b2
	factor := p.factor
	var s0, s1 uint32
	s2 := p.half
	c0, c1, c2 := 0, b1, b2

	srcIdx := srcLeft - p.border
	srcEnd := srcLeft + n - p.border
	dstIdx := 0
	si, di := 0, 0
	if dstIdx < srcIdx {
		end := min(srcIdx, dstCount)
		for dstIdx < end {
			dst[di] = 0
			di += dstStride
			dstIdx++
		}
	} else if srcIdx < dstIdx {
		end := min(dstIdx, srcEnd)
		for srcIdx < end {
			lead := uint32(src[si])
			s0 += lead
			s1 += s0
			s2 += s1
			s2 -= buf[c2]
			buf[c2] = s1
			if c2++; c2 >= bufLen {
				c2 = b2
			}
			s1 -= buf[c1]
			buf[c1] = s0
			if c1++; c1 >= b2 {
				c1 = b1
			}
			s0 -= buf[c0]
			buf[c0] = lead
			if c0++; c0 >= b1 {
				c0 = 0
			}
			si += srcStride
			srcIdx++
		}
		for srcIdx < dstIdx {
			s1 += s0
			s2 += s1
			s2 -= buf[c2]
			buf[c2] = s1
			if c2++; c2 >= bufLen {
				c2 = b2
			}
			s1 -= buf[c1]
			buf[c1] = s0
			if c1++; c1 >= b2 {
				c1 = b1
			}
			s0 -= buf[c0]
			buf[c0] = 0
			if c0++; c0 >= b1 {
				c0 = 0
			}
			srcIdx++
		}
	}
	for end := min(dstCount, srcEnd); dstIdx < end; dstIdx++ {
		lead := uint32(src[si])
		s0 += lead
		s1 += s0
		s2 += s1
		dst[di] = uint8((uint64(s2) * factor) >> 32)
		s2 -= buf[c2]
		buf[c2] = s1
		if c2++; c2 >= bufLen {
			c2 = b2
		}
		s1 -= buf[c1]
		buf[c1] = s0
		if c1++; c1 >= b2 {
			c1 = b1
		}
		s0 -= buf[c0]
		buf[c0] = lead
		if c0++; c0 >= b1 {
			c0 = 0
		}
		si += srcStride
		di += dstStride
	}
	for ; dstIdx < dstCount; dstIdx++ {
		s1 += s0
		s2 += s1
		dst[di] = uint8((uint64(s2) * factor) >> 32)
		s2 -= buf[c2]
		buf[c2] = s1
		if c2++; c2 >= bufLen {
			c2 = b2
		}
		s1 -= buf[c1]
		buf[c1] = s0
		if c1++; c1 >= b2 {
			c1 = b1
		}
		s0 -= buf[c0]
		buf[c0] = 0
		if c0++; c0 >= b1 {
			c0 = 0
		}
		di += dstStride
	}
}

// alphaImage is the alpha channel of an image for a rectangle of the
// device.
type alphaImage struct {
	pix    []uint8
	bounds IRect
	stride int
}

func (a *alphaImage) release() {
	if a == nil || a.pix == nil {
		return
	}
	putFilterBuf(a.pix)
	a.pix = nil
}

func (a *alphaImage) at(x, y int) uint8 {
	if x < a.bounds.L || x >= a.bounds.R || y < a.bounds.T || y >= a.bounds.B {
		return 0
	}
	return a.pix[(y-a.bounds.T)*a.stride+(x-a.bounds.L)]
}

// blurAlpha returns the alpha channel of an image after a Gaussian blur
// with the given sigma per axis (eval_blur_passes). The result is larger
// than the source by three sigma on each side.
func blurAlpha(src *filterImage, sigmaX, sigmaY float32) *alphaImage {
	sb := src.bounds
	sw, sh := sb.width(), sb.height()
	wx, wy := 1, 1
	if !blurIdentity(sigmaX) {
		wx = boxBlurWindow(sigmaX)
	}
	if !blurIdentity(sigmaY) {
		wy = boxBlurWindow(sigmaY)
	}
	if (wx <= 1 && wy <= 1) || wx >= 255 || wy >= 255 {
		out := &alphaImage{pix: getFilterBuf(sw * sh), bounds: sb, stride: sw}
		for y := 0; y < sh; y++ {
			row := src.pix[y*src.stride:]
			for x := 0; x < sw; x++ {
				out.pix[y*sw+x] = row[x*4+3]
			}
		}
		return out
	}
	rx, ry := sigmaToRadius(sigmaX), sigmaToRadius(sigmaY)
	db := IRect{sb.L - rx, sb.T - ry, sb.R + rx, sb.B + ry}
	dw, dh := db.width(), db.height()
	out := &alphaImage{pix: getFilterBuf(dw * dh), bounds: db, stride: dw}
	// The rows of the source land at this row offset of the destination.
	top := sb.T - db.T
	if wx > 1 {
		pass := newAlphaPass(wx)
		for y := 0; y < sh; y++ {
			row := src.pix[y*src.stride : y*src.stride+sw*4]
			hasAlpha := false
			i := 3
			for ; i+12 < len(row); i += 16 {
				if row[i]|row[i+4]|row[i+8]|row[i+12] != 0 {
					hasAlpha = true
					break
				}
			}
			if !hasAlpha {
				for ; i < len(row); i += 4 {
					if row[i] != 0 {
						hasAlpha = true
						break
					}
				}
			}
			if !hasAlpha {
				continue
			}
			pass.blur(sb.L-db.L, row[3:], 4, sw, out.pix[(top+y)*dw:], 1, dw)
		}
	} else {
		for y := 0; y < sh; y++ {
			row := src.pix[y*src.stride:]
			o := (top+y)*dw + (sb.L - db.L)
			for x := 0; x < sw; x++ {
				out.pix[o+x] = row[x*4+3]
			}
		}
	}
	if wy > 1 {
		pass := newAlphaPass(wy)
		col := getFilterBuf(sh)
		defer putFilterBuf(col)
		for x := 0; x < dw; x++ {
			var anyByte byte
			idx := top*dw + x
			for y := 0; y < sh; y++ {
				v := out.pix[idx]
				col[y] = v
				anyByte |= v
				idx += dw
			}
			if anyByte == 0 {
				continue
			}
			pass.blur(top, col, 1, sh, out.pix[x:], dw, dh)
		}
	}
	return out
}
