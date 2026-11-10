// The canvas of the painter: a base bitmap and a stack of layers, as
// SkCanvas::saveLayer makes them. The layer restore math is ported from
// skia/src/core/SkSpriteBlitter_ARGB32.cpp, SkBlitRow_D32.cpp and
// src/opts/SkBlitRow_opts.h (Skia branch chrome/m156).

package raster

import "sync"

// canvas holds the devices of one render. A layer has the size of the base
// bitmap and the same coordinate system. Each device tracks the bounds of
// what was drawn into it, so that clearing and compositing a layer cost
// only as much as its content.
type canvas struct {
	w, h  int
	stack []*device
	pool  []*pixmap
	dr    drawer
}

func newCanvas(w, h int) *canvas {
	c := &canvas{w: w, h: h}
	base := &device{pm: newPixmap(w, h), clip: IRect{0, 0, w, h}}
	c.stack = append(c.stack, base)
	return c
}

func (c *canvas) dev() *device { return c.stack[len(c.stack)-1] }

func (c *canvas) getPixmap() *pixmap {
	if n := len(c.pool); n > 0 {
		pm := c.pool[n-1]
		c.pool = c.pool[:n-1]
		return pm
	}
	// A layer bitmap of an earlier render, when it is large enough. The
	// bitmaps in the pool are clear over their whole capacity.
	if pm, ok := layerPool.Get().(*pixmap); ok {
		if need := c.w * c.h * 4; cap(pm.pix) >= need {
			pm.pix = pm.pix[:need]
			pm.stride, pm.w, pm.h = c.w*4, c.w, c.h
			return pm
		}
	}
	return newPixmap(c.w, c.h)
}

// layerPool keeps layer bitmaps between renders.
var layerPool sync.Pool

// release gives the layer bitmaps of a finished render to the pool.
func (c *canvas) release() {
	for _, pm := range c.pool {
		layerPool.Put(pm)
	}
	c.pool = nil
}

// putPixmap returns a layer bitmap to the pool. dirty is the region that
// was drawn: it is cleared.
func (c *canvas) putPixmap(pm *pixmap, dirty IRect) {
	if dirty.intersect(pm.bounds()) {
		for y := dirty.T; y < dirty.B; y++ {
			row := pm.pix[y*pm.stride+dirty.L*4 : y*pm.stride+dirty.R*4]
			for i := range row {
				row[i] = 0
			}
		}
	}
	c.pool = append(c.pool, pm)
}

// pushLayer is SkCanvas::saveLayer: drawing goes to a new transparent
// layer until popLayer. The layer inherits the clip. Skia does not know the
// pixel geometry of such a layer, so its text has no LCD masks.
func (c *canvas) pushLayer() *device {
	cur := c.dev()
	if c.dr.vector != nil {
		c.dr.vector.doc.BeginGroup(1, false)
		layer := &device{clip: cur.clip}
		c.stack = append(c.stack, layer)
		return layer
	}
	layer := &device{pm: c.getPixmap(), clip: cur.clip, aa: cur.aa}
	c.stack = append(c.stack, layer)
	return layer
}

// pushLayerWithPrevious is SkCanvas::saveLayer with the flags
// kPreserveLCDText_SaveLayerFlag and kInitWithPrevious_SaveLayerFlag, as
// SaveLayerAlphaOp::Raster of cc/paint/paint_op.cc sets them: the layer has
// the pixel geometry of the canvas, and it starts as a copy of the device
// below it. Text in the layer is then drawn over the pixels that are behind
// it, which LCD masks need.
func (c *canvas) pushLayerWithPrevious() *device {
	if c.dr.vector != nil {
		return c.pushLayer()
	}
	cur := c.dev()
	layer := &device{pm: c.getPixmap(), clip: cur.clip, aa: cur.aa, lcd: c.stack[0].lcd, backdrop: cur}
	c.stack = append(c.stack, layer)
	return layer
}

// popLayerRaw removes the top layer and returns it without compositing.
// The caller composites it and gives the bitmap back with putPixmap.
func (c *canvas) popLayerRaw() *device {
	n := len(c.stack)
	layer := c.stack[n-1]
	c.stack = c.stack[:n-1]
	return layer
}

// popLayer composites the top layer into the device below it with an alpha
// and a blend mode (SkCanvas::restore).
func (c *canvas) popLayer(alpha float32, blend blendMode) {
	if c.dr.vector != nil {
		c.popLayerRaw()
		c.dr.vector.doc.EndGroupWithOpacity(float64(alpha), blend == blendMultiply)
		return
	}
	layer := c.popLayerRaw()
	c.compositeLayer(c.dev(), layer.pm, layer.dirty, alpha, blend)
	c.putPixmap(layer.pm, layer.dirty)
}

// alphaToByte is SkPaint::getAlpha for a float alpha: sk_float_round2int of
// alpha * 255.
func alphaToByte(alpha float32) uint8 {
	if !(alpha > 0) {
		return 0
	}
	if alpha >= 1 {
		return 255
	}
	return uint8(float32(alpha*255) + 0.5)
}

// compositeLayer draws the region of a layer into a device.
func (c *canvas) compositeLayer(dst *device, src *pixmap, region IRect, alpha float32, blend blendMode) {
	if !region.intersect(dst.clip) {
		return
	}
	if dst.aa != nil && !region.intersect(dst.aa.bounds) {
		return
	}
	dst.extend(region)
	a := alphaToByte(alpha)
	if a == 0 && blend == blendSrcOver {
		return
	}
	width := region.width()
	for y := region.T; y < region.B; y++ {
		so := y*src.stride + region.L*4
		s := src.pix[so : so+width*4 : so+width*4]
		d := dst.pm.pix[so : so+width*4 : so+width*4]
		var clipRow []uint8
		if dst.aa != nil {
			clipRow = dst.aa.rowAt(y)[region.L-dst.aa.bounds.L : region.R-dst.aa.bounds.L]
		}
		switch {
		case blend != blendSrcOver || clipRow != nil:
			blendRowGeneral(d, s, a, blend, clipRow)
		case a == 255:
			srcOverRow(d, s)
		default:
			srcOverRowAlpha(d, s, uint32(a))
		}
	}
}

// srcOverRow is blit_row_s32a_opaque for NEON: source-over with exact
// rounding of the destination term (SkMulDiv255Round).
func srcOverRow(d, s []uint8) {
	for i := 0; i+3 < len(s); i += 4 {
		sa := s[i+3]
		if sa == 0 {
			continue
		}
		if sa == 255 {
			d[i], d[i+1], d[i+2], d[i+3] = s[i], s[i+1], s[i+2], 255
			continue
		}
		na := uint32(255 - sa)
		d[i] = satAdd8(s[i], uint8(mulDiv255Round(na, uint32(d[i]))))
		d[i+1] = satAdd8(s[i+1], uint8(mulDiv255Round(na, uint32(d[i+1]))))
		d[i+2] = satAdd8(s[i+2], uint8(mulDiv255Round(na, uint32(d[i+2]))))
		d[i+3] = satAdd8(s[i+3], uint8(mulDiv255Round(na, uint32(d[i+3]))))
	}
}

func satAdd8(a, b uint8) uint8 {
	s := uint32(a) + uint32(b)
	if s > 255 {
		return 255
	}
	return uint8(s)
}

// srcOverRowAlpha is blit_row_s32a_blend: SkBlendARGB32 per pixel.
func srcOverRowAlpha(d, s []uint8, alpha uint32) {
	srcScale := alpha + 1
	for i := 0; i+3 < len(s); i += 4 {
		prod := 0xFFFF - uint32(s[i+3])*srcScale
		dstScale := (prod + (prod >> 8)) >> 8
		d[i] = uint8((uint32(s[i])*srcScale + uint32(d[i])*dstScale) >> 8)
		d[i+1] = uint8((uint32(s[i+1])*srcScale + uint32(d[i+1])*dstScale) >> 8)
		d[i+2] = uint8((uint32(s[i+2])*srcScale + uint32(d[i+2])*dstScale) >> 8)
		d[i+3] = uint8((uint32(s[i+3])*srcScale + uint32(d[i+3])*dstScale) >> 8)
	}
}

// blendRowGeneral composites a layer row with a blend mode other than
// source-over, or through an anti-aliased clip, in the normalized
// arithmetic of the raster pipeline.
func blendRowGeneral(d, s []uint8, alpha uint8, blend blendMode, clipRow []uint8) {
	const inv = float32(1.0 / 255)
	ga := float32(float32(alpha) * inv)
	for i := 0; i+3 < len(s); i += 4 {
		sr, sg, sb, sa := float32(float32(s[i])*inv*ga), float32(float32(s[i+1])*inv*ga), float32(float32(s[i+2])*inv*ga), float32(float32(s[i+3])*inv*ga)
		dr, dg, db, da := float32(float32(d[i])*inv), float32(float32(d[i+1])*inv), float32(float32(d[i+2])*inv), float32(float32(d[i+3])*inv)
		var r, g, b, a float32
		switch blend {
		case blendMultiply:
			isa, ida := 1-sa, 1-da
			r = float32(sr*ida) + float32(dr*isa) + float32(sr*dr)
			g = float32(sg*ida) + float32(dg*isa) + float32(sg*dg)
			b = float32(sb*ida) + float32(db*isa) + float32(sb*db)
			a = float32(sa*ida) + float32(da*isa) + float32(sa*da)
		case blendDstIn:
			r, g, b, a = float32(dr*sa), float32(dg*sa), float32(db*sa), float32(da*sa)
		case blendSrc:
			r, g, b, a = sr, sg, sb, sa
		default:
			k := 1 - sa
			r, g, b, a = sr+float32(dr*k), sg+float32(dg*k), sb+float32(db*k), sa+float32(da*k)
		}
		if clipRow != nil {
			cv := float32(float32(clipRow[i/4]) * inv)
			r = dr + float32((r-dr)*cv)
			g = dg + float32((g-dg)*cv)
			b = db + float32((b-db)*cv)
			a = da + float32((a-da)*cv)
		}
		d[i], d[i+1], d[i+2], d[i+3] = toUnorm(r), toUnorm(g), toUnorm(b), toUnorm(a)
	}
}
