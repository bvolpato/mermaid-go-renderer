// A blitter for the cases that Skia hands to SkRasterPipelineBlitter:
// shaders and blend modes other than source-over.
//
// Skia runs these through its raster pipeline. The pipeline computes in
// normalized values and stores round(v * 255). This file does the same
// arithmetic per pixel.

package raster

// shader computes premultiplied colors for a span of device pixels.
type shader interface {
	// shadeSpan writes n premultiplied colors, as float32 in [0, 1], for
	// the pixels (x, y) to (x+n-1, y).
	shadeSpan(x, y, n int, out []rgbaF)
}

// rgbaF is a premultiplied color in float32.
type rgbaF struct{ r, g, b, a float32 }

type rpBlitter struct {
	dst    *pixmap
	shader shader
	color  rgbaF
	alpha  float32
	blend  blendMode
	span   []rgbaF
}

func (b *rpBlitter) init(dst *pixmap, p *Paint) {
	b.dst = dst
	b.shader = p.shader
	b.blend = p.blend
	b.alpha = float32(float32(p.A) * (1.0 / 255))
	a := b.alpha
	b.color = rgbaF{
		float32(float32(float32(p.R)*(1.0/255)) * a),
		float32(float32(float32(p.G)*(1.0/255)) * a),
		float32(float32(float32(p.B)*(1.0/255)) * a),
		a,
	}
}

func toUnorm(v float32) uint8 {
	if !(v > 0) {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(float32(v*255) + 0.5)
}

func (b *rpBlitter) src(x, y, n int) []rgbaF {
	if cap(b.span) < n {
		b.span = make([]rgbaF, n*2)
	}
	out := b.span[:n]
	if b.shader == nil {
		for i := range out {
			out[i] = b.color
		}
		return out
	}
	b.shader.shadeSpan(x, y, n, out)
	if b.alpha != 1 {
		a := b.alpha
		for i := range out {
			out[i].r *= a
			out[i].g *= a
			out[i].b *= a
			out[i].a *= a
		}
	}
	return out
}

// blendSpan blends n source colors into the destination with a coverage
// from 0 to 255.
func (b *rpBlitter) blendSpan(x, y, n int, cov uint8) {
	if n <= 0 || cov == 0 {
		return
	}
	src := b.src(x, y, n)
	o := y*b.dst.stride + x*4
	d := b.dst.pix[o : o+n*4 : o+n*4]
	c := float32(float32(cov) * (1.0 / 255))
	const inv = float32(1.0 / 255)
	for i := 0; i < n; i++ {
		s := src[i]
		p := d[i*4 : i*4+4 : i*4+4]
		dr, dg, db, da := float32(float32(p[0])*inv), float32(float32(p[1])*inv), float32(float32(p[2])*inv), float32(float32(p[3])*inv)
		var r, g, bl, a float32
		switch b.blend {
		case blendSrcOver:
			k := 1 - s.a
			r, g, bl, a = s.r+float32(dr*k), s.g+float32(dg*k), s.b+float32(db*k), s.a+float32(da*k)
		case blendMultiply:
			// s*(1-da) + d*(1-sa) + s*d
			isa, ida := 1-s.a, 1-da
			r = float32(s.r*ida) + float32(dr*isa) + float32(s.r*dr)
			g = float32(s.g*ida) + float32(dg*isa) + float32(s.g*dg)
			bl = float32(s.b*ida) + float32(db*isa) + float32(s.b*db)
			a = float32(s.a*ida) + float32(da*isa) + float32(s.a*da)
		case blendDstIn:
			r, g, bl, a = float32(dr*s.a), float32(dg*s.a), float32(db*s.a), float32(da*s.a)
		case blendSrc:
			r, g, bl, a = s.r, s.g, s.b, s.a
		}
		if cov != 255 {
			// lerp(dst, blended, coverage)
			r = dr + float32((r-dr)*c)
			g = dg + float32((g-dg)*c)
			bl = db + float32((bl-db)*c)
			a = da + float32((a-da)*c)
		}
		p[0], p[1], p[2], p[3] = toUnorm(r), toUnorm(g), toUnorm(bl), toUnorm(a)
	}
}

func (b *rpBlitter) blitH(x, y, width int) { b.blendSpan(x, y, width, 255) }

func (b *rpBlitter) blitAntiH(x, y int, aa []uint8, runs []int16) {
	i := 0
	for {
		count := int(runs[i])
		if count <= 0 {
			return
		}
		b.blendSpan(x+i, y, count, aa[i])
		i += count
	}
}

func (b *rpBlitter) blitV(x, y, height int, alpha uint8) {
	for ; height > 0; height-- {
		b.blendSpan(x, y, 1, alpha)
		y++
	}
}

func (b *rpBlitter) blitRect(x, y, width, height int) {
	for ; height > 0; height-- {
		b.blendSpan(x, y, width, 255)
		y++
	}
}

func (b *rpBlitter) blitAntiRect(x, y, width, height int, leftAlpha, rightAlpha uint8) {
	defaultBlitAntiRect(b, x, y, width, height, leftAlpha, rightAlpha)
}

func (b *rpBlitter) blitMask(m *maskA8, clip IRect) {
	for y := clip.T; y < clip.B; y++ {
		row := m.row(y)[clip.L-m.bounds.L:]
		for x := clip.L; x < clip.R; {
			c := row[x-clip.L]
			n := 1
			for x+n < clip.R && row[x+n-clip.L] == c {
				n++
			}
			b.blendSpan(x, y, n, c)
			x += n
		}
	}
}

func (b *rpBlitter) blitAntiH2(x, y int, a0, a1 uint8) {
	b.blendSpan(x, y, 1, a0)
	b.blendSpan(x+1, y, 1, a1)
}

func (b *rpBlitter) blitAntiV2(x, y int, a0, a1 uint8) {
	b.blendSpan(x, y, 1, a0)
	b.blendSpan(x, y+1, 1, a1)
}
