// Ported from skia/src/core/SkDraw.cpp, SkDrawProcs.h, SkPathUtils.cpp and
// SkStrokeRec.cpp (Skia branch chrome/m156): how the software device of
// Skia turns a draw call into scan conversion.

package raster

import "math"

// blendMode is the subset of SkBlendMode that the painter uses.
type blendMode uint8

const (
	blendSrcOver blendMode = iota
	blendMultiply
	blendDstIn
	blendSrc
)

// Paint is the subset of SkPaint that the painter needs.
type Paint struct {
	// Color is a straight alpha color (SkColor).
	R, G, B, A uint8
	shader     shader

	stroke     bool
	width      float32
	miterLimit float32
	cap        strokeCap
	join       strokeJoin
	dash       *dashInfo

	antiAlias bool
	blend     blendMode
}

// device is one raster target: the base bitmap or a layer.
type device struct {
	pm *pixmap
	// clip is the device clip rectangle.
	clip IRect
	// aa is the anti-aliased clip, or nil.
	aa *aaClip
	// dirty is the bounds of everything drawn into the device.
	dirty IRect
	// lcd reports that Skia knows the pixel geometry of the device
	// (SkSurfaceProps): text can have one coverage per color channel.
	lcd bool
	// backdrop is the device below a layer that starts as a copy of that
	// device (SkCanvas::kInitWithPrevious_SaveLayerFlag). It is nil for
	// the base bitmap and for a layer that starts transparent.
	backdrop *device
}

// touch adds a rectangle to the bounds of what is drawn, before the pixels
// are written.
func (d *device) touch(r IRect) {
	if r.intersect(d.clip) {
		d.extend(r)
	}
}

// extend adds a rectangle inside the clip to the dirty bounds. A layer that
// starts as a copy of the device below it copies the new part of the bounds
// first: the copy of the rest of the layer is never read, because a pixel
// that is equal to the pixel below it does not change that pixel when the
// layer is drawn back.
func (d *device) extend(r IRect) {
	n := d.dirty.join(r)
	if n == d.dirty {
		return
	}
	if d.backdrop != nil {
		// The device below can be such a layer too.
		d.backdrop.extend(n)
		old := d.dirty
		src := d.backdrop.pm
		for y := n.T; y < n.B; y++ {
			o := y * d.pm.stride
			if old.isEmpty() || y < old.T || y >= old.B {
				copy(d.pm.pix[o+n.L*4:o+n.R*4], src.pix[o+n.L*4:o+n.R*4])
				continue
			}
			copy(d.pm.pix[o+n.L*4:o+old.L*4], src.pix[o+n.L*4:o+old.L*4])
			copy(d.pm.pix[o+old.R*4:o+n.R*4], src.pix[o+old.R*4:o+n.R*4])
		}
	}
	d.dirty = n
}

// drawer holds the scratch state of drawing. It is not safe for concurrent
// use.
type drawer struct {
	aaa      aaaState
	color    colorBlitter
	rp       rpBlitter
	aaBlit   aaClipBlitter
	stroke   strokeScratch
	dashS    dashScratch
	devPath  Path
	tmpPath  Path
	tmpPath2 Path
	hair     []Point
	nonAA    nonAAState
}

// chooseBlitter is SkBlitter::Choose: a solid color with source-over uses
// the legacy 32-bit blitters. Everything else uses the raster pipeline.
func (dr *drawer) chooseBlitter(dev *device, p *Paint) blitter {
	if p.shader == nil && p.blend == blendSrcOver {
		dr.color.init(dev.pm, p.R, p.G, p.B, p.A)
		return &dr.color
	}
	dr.rp.init(dev.pm, p)
	return &dr.rp
}

// wrapClip applies the anti-aliased clip of the device to a blitter.
func (dr *drawer) wrapClip(dev *device, b blitter) blitter {
	if dev.aa == nil {
		return b
	}
	dr.aaBlit.init(b, dev.aa)
	return &dr.aaBlit
}

func fastLen(v Point) float32 {
	x, y := abs32f(v.X), abs32f(v.Y)
	if x < y {
		x, y = y, x
	}
	return x + float32(y/2)
}

// treatAAStrokeAsHairline is DrawTreatAAStrokeAsHairline.
func treatAAStrokeAsHairline(strokeWidth float32, m *Matrix) (float32, bool) {
	d0 := m.mapVector(Point{strokeWidth, 0})
	d1 := m.mapVector(Point{0, strokeWidth})
	len0 := fastLen(d0)
	len1 := fastLen(d1)
	if len0 <= 1 && len1 <= 1 {
		return midpoint32(len0, len1), true
	}
	return 0, false
}

// treatAsHairline is DrawTreatAsHairline.
func treatAsHairline(p *Paint, m *Matrix) (float32, bool) {
	if !p.stroke {
		return 0, false
	}
	if p.width == 0 {
		return 1, true
	}
	if !p.antiAlias {
		return 0, false
	}
	return treatAAStrokeAsHairline(p.width, m)
}

// resScaleForStroking is SkMatrixPriv::ComputeResScaleForStroking.
func resScaleForStroking(m *Matrix) float32 {
	length := func(dx, dy float32) float32 {
		mag2 := float32(dx*dx) + float32(dy*dy)
		if isFinite32(mag2) {
			return sqrt32(mag2)
		}
		return float32(math.Sqrt(float64(float64(dx)*float64(dx)) + float64(float64(dy)*float64(dy))))
	}
	sx := length(m.sx, m.ky)
	sy := length(m.kx, m.sy)
	if isFinite32(sx) && isFinite32(sy) {
		if scale := max(sx, sy); scale > 0 {
			return scale
		}
	}
	return 1
}

// drawPath is SkDraw::drawPath: path is in local space.
func (dr *drawer) drawPath(dev *device, path *Path, origPaint *Paint, ctm *Matrix) {
	if dev.clip.isEmpty() || path.isEmpty() {
		return
	}
	paint := origPaint
	var modified Paint
	// modifyPaintForHairlines.
	if coverage, ok := treatAsHairline(origPaint, ctm); ok {
		if coverage == 1 {
			modified = *origPaint
			modified.width = 0
			paint = &modified
		} else if origPaint.blend == blendSrcOver {
			scale := int32(floatToInt(float32(coverage * 256)))
			modified = *origPaint
			modified.width = 0
			modified.A = uint8(int32(origPaint.A) * scale >> 8)
			paint = &modified
		}
	}

	doFill := true
	devPath := &dr.devPath
	if paint.stroke {
		// computeConservativeLocalClipBounds.
		var cullRect *Rect
		var cull Rect
		if inv, ok := ctm.invert(); ok {
			cull = inv.mapRect(dev.clip.outset(1, 1).toRect())
			cullRect = &cull
		}
		if !path.isFinite() {
			return
		}
		sp := strokeParams{width: paint.width, miterLimit: paint.miterLimit, cap: paint.cap, join: paint.join, resScale: resScaleForStroking(ctm)}
		hairline := paint.width == 0
		src := path
		fillStyle := false
		if paint.dash != nil {
			dr.tmpPath.Reset()
			ok, fs := dashPath(&dr.tmpPath, path, &sp, hairline, cullRect, paint.dash, &dr.dashS)
			if ok {
				src = &dr.tmpPath
				fillStyle = fs
			}
		}
		out := src
		if !hairline && !fillStyle {
			strokePath(src, &dr.tmpPath2, &sp, &dr.stroke)
			out = &dr.tmpPath2
		}
		if !out.isFinite() {
			return
		}
		doFill = !hairline
		out.transformInto(devPath, ctm)
		devPath.fill = fillWinding
	} else {
		if ctm.isIdentity() {
			devPath = path
		} else {
			path.transformInto(devPath, ctm)
		}
	}
	dr.drawDevPath(dev, devPath, paint, doFill)
}

// tooBigForMath is SkPathPriv::TooBigForMath.
func tooBigForMath(r Rect) bool {
	const big = float32(1 << 22)
	return r.L < -big || r.T < -big || r.R > big || r.B > big
}

// drawDevPath is SkDraw::drawDevPath: path is in device space.
func (dr *drawer) drawDevPath(dev *device, path *Path, paint *Paint, doFill bool) {
	bounds := path.bounds()
	if tooBigForMath(bounds) || !path.isFinite() {
		return
	}
	b := dr.chooseBlitter(dev, paint)
	if doFill {
		dev.touch(bounds.roundOut())
		if paint.antiAlias {
			if dev.aa != nil {
				dr.aaBlit.init(b, dev.aa)
				dr.aaa.antiFillPathAA(path, dev.clip, &dr.aaBlit)
			} else {
				dr.aaa.antiFillPath(path, dev.clip, b)
			}
		} else {
			dr.nonAA.fillPath(path, dev.clip, dr.wrapClip(dev, b))
		}
		return
	}
	dev.touch(bounds.roundOut().outset(2, 2))
	b = dr.wrapClip(dev, b)
	if paint.antiAlias {
		antiHairPath(path, dev.clip, b, paint.cap, &dr.hair)
	} else {
		hairPath(path, dev.clip, b, paint.cap, &dr.hair)
	}
}

// drawRect is SkDraw::drawRect.
func (dr *drawer) drawRect(dev *device, rect Rect, paint *Paint, ctm *Matrix) {
	if dev.clip.isEmpty() {
		return
	}
	const (
		rectFill = iota
		rectStroke
		rectHair
		rectPath
	)
	// ComputeRectType.
	rtype := rectPath
	var strokeSize Point
	switch {
	case paint.dash != nil && paint.stroke, !ctm.rectStaysRect():
		rtype = rectPath
	case !paint.stroke:
		rtype = rectFill
	case paint.width == 0:
		rtype = rectHair
	case !rect.isEmpty() && paint.join == joinMiter && paint.miterLimit >= float32(math.Sqrt2):
		size := ctm.mapVector(Point{paint.width, paint.width})
		strokeSize = Point{abs32f(size.X), abs32f(size.Y)}
		rtype = rectStroke
	}
	if rtype == rectPath {
		p := &dr.tmpPath
		p.Reset()
		p.AddRect(rect, 0)
		// drawPath uses tmpPath as scratch, so the rectangle goes into a
		// path of its own.
		var rp Path
		rp.lastMoveIdx = -1
		rp.AddRect(rect, 0)
		dr.drawPath(dev, &rp, paint, ctm)
		return
	}
	var q [2]Point
	ctm.mapPoints(q[:], []Point{{rect.L, rect.T}, {rect.R, rect.B}})
	devRect := Rect{q[0].X, q[0].Y, q[1].X, q[1].Y}.sorted()

	bbox := devRect
	if paint.stroke {
		if paint.width == 0 {
			bbox = bbox.outset(1, 1)
		} else {
			ssize := strokeSize
			if rtype != rectStroke {
				size := ctm.mapVector(Point{paint.width, paint.width})
				ssize = Point{abs32f(size.X), abs32f(size.Y)}
			}
			bbox = bbox.outset(ssize.X/2, ssize.Y/2)
		}
	}
	if tooBigForMath(bbox) {
		return
	}
	const fixedLimit = float32(32767)
	if (bbox.L < -fixedLimit || bbox.T < -fixedLimit || bbox.R > fixedLimit || bbox.B > fixedLimit) && rtype != rectHair {
		var rp Path
		rp.lastMoveIdx = -1
		rp.AddRect(rect, 0)
		dr.drawPath(dev, &rp, paint, ctm)
		return
	}
	ir := bbox.roundOut()
	if ir.isEmpty() || !intersects(ir, dev.clip) {
		return
	}
	dev.touch(ir)
	b := dr.wrapClip(dev, dr.chooseBlitter(dev, paint))
	switch rtype {
	case rectFill:
		if paint.antiAlias {
			antiFillRect(devRect, dev.clip, b)
		} else {
			fillRect(devRect, dev.clip, b)
		}
	case rectStroke:
		if paint.antiAlias {
			antiFrameRect(devRect, strokeSize, dev.clip, b)
		} else {
			frameRect(devRect, strokeSize, dev.clip, b)
		}
	case rectHair:
		pts := [5]Point{{devRect.L, devRect.T}, {devRect.R, devRect.T}, {devRect.R, devRect.B}, {devRect.L, devRect.B}, {devRect.L, devRect.T}}
		if paint.antiAlias {
			clip := dev.clip
			var clipPtr *IRect
			if !clip.contains(ir.outset(1, 1)) {
				clipPtr = &clip
			}
			antiHairLineRgn(pts[:], clipPtr, b)
		} else {
			hairRect(devRect, dev.clip, b)
		}
	}
}

func intersects(a, b IRect) bool {
	return a.L < b.R && b.L < a.R && a.T < b.B && b.T < a.B
}

// drawOval is SkDraw::drawOval.
func (dr *drawer) drawOval(dev *device, oval Rect, paint *Paint, ctm *Matrix) {
	var p Path
	p.lastMoveIdx = -1
	p.AddOval(oval, 1)
	dr.drawPath(dev, &p, paint, ctm)
}
