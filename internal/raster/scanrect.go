// Ported from the rectangle functions of skia/src/core/SkScan_Antihair.cpp
// and SkScan.cpp (Skia branch chrome/m156): SkScan::AntiFillRect,
// SkScan::AntiFrameRect, SkScan::FillRect and SkScan::FrameRect.

package raster

const hlineStackBuffer = 100

// callHLineBlitter is call_hline_blitter.
func callHLineBlitter(b blitter, x, y, count int, alpha uint8) {
	var runs [hlineStackBuffer + 1]int16
	var aa [hlineStackBuffer]uint8
	for count > 0 {
		aa[0] = alpha
		n := min(count, hlineStackBuffer)
		runs[0] = int16(n)
		runs[n] = 0
		b.blitAntiH(x, y, aa[:], runs[:])
		x += n
		count -= n
	}
}

// fdot8 is FDot8: a 24.8 fixed-point number.
type fdot8 = int32

func fixedToFDot8(x fixed) fdot8 { return (x + 0x80) >> 8 }

func alphaMul(value, alpha256 int32) uint8 { return uint8((value * alpha256) >> 8) }

func doScanline(l fdot8, top int, r fdot8, alpha int32, b blitter) {
	if (l >> 8) == ((r - 1) >> 8) {
		b.blitV(int(l>>8), top, 1, alphaMul(alpha, r-l))
		return
	}
	left := int(l >> 8)
	if l&0xFF != 0 {
		b.blitV(left, top, 1, alphaMul(alpha, 256-(l&0xFF)))
		left++
	}
	rite := int(r >> 8)
	width := rite - left
	if width > 0 {
		callHLineBlitter(b, left, top, width, uint8(alpha))
	}
	if r&0xFF != 0 {
		b.blitV(rite, top, 1, alphaMul(alpha, r&0xFF))
	}
}

func antiFillDot8(l, t, r, bt fdot8, b blitter, fillInner bool) {
	if l >= r || t >= bt {
		return
	}
	top := int(t >> 8)
	if top == int((bt-1)>>8) {
		doScanline(l, top, r, bt-t-1, b)
		return
	}
	if t&0xFF != 0 {
		doScanline(l, top, r, 256-(t&0xFF), b)
		top++
	}
	bot := int(bt >> 8)
	height := bot - top
	if height > 0 {
		left := int(l >> 8)
		if left == int((r-1)>>8) {
			b.blitV(left, top, height, uint8(r-l-1))
		} else {
			if l&0xFF != 0 {
				b.blitV(left, top, height, uint8(256-(l&0xFF)))
				left++
			}
			rite := int(r >> 8)
			width := rite - left
			if width > 0 && fillInner {
				b.blitRect(left, top, width, height)
			}
			if r&0xFF != 0 {
				b.blitV(rite, top, height, uint8(r&0xFF))
			}
		}
	}
	if bt&0xFF != 0 {
		doScanline(l, bot, r, bt&0xFF, b)
	}
}

// antiFillRect is SkScan::AntiFillRect for a rectangular clip.
func antiFillRect(r Rect, clip IRect, b blitter) {
	newR := clip.toRect()
	if !newR.intersect(r) {
		return
	}
	antiFillDot8(fixedToFDot8(floatToFixed(newR.L)), fixedToFDot8(floatToFixed(newR.T)),
		fixedToFDot8(floatToFixed(newR.R)), fixedToFDot8(floatToFixed(newR.B)), b, true)
}

func fillCheckRect(l, t, r, bt int, b blitter) {
	if l < r && t < bt {
		b.blitRect(l, t, r-l, bt-t)
	}
}

func scalarToFDot8(x float32) fdot8 { return floatToInt(float32(x * 256)) }
func fdot8Floor(x fdot8) int        { return int(x >> 8) }
func fdot8Ceil(x fdot8) int         { return int((x + 0xFF) >> 8) }

// invAlphaMul is InvAlphaMul: 1 - (1 - a)*(1 - b).
func invAlphaMul(a, b int32) uint8 {
	return uint8(a + b - int32(mulDiv255Round(uint32(a), uint32(b))))
}

func innerScanline(l fdot8, top int, r fdot8, alpha int32, b blitter) {
	if (l >> 8) == ((r - 1) >> 8) {
		widClamp := r - l
		widClamp = widClamp - (widClamp >> 8)
		b.blitV(int(l>>8), top, 1, invAlphaMul(alpha, widClamp))
		return
	}
	left := int(l >> 8)
	if l&0xFF != 0 {
		b.blitV(left, top, 1, invAlphaMul(alpha, l&0xFF))
		left++
	}
	rite := int(r >> 8)
	width := rite - left
	if width > 0 {
		callHLineBlitter(b, left, top, width, uint8(alpha))
	}
	if r&0xFF != 0 {
		b.blitV(rite, top, 1, invAlphaMul(alpha, ^r&0xFF))
	}
}

func innerStrokeDot8(l, t, r, bt fdot8, b blitter) {
	top := int(t >> 8)
	if top == int((bt-1)>>8) {
		alpha := 256 - (bt - t)
		if alpha != 0 {
			innerScanline(l, top, r, alpha, b)
		}
		return
	}
	if t&0xFF != 0 {
		innerScanline(l, top, r, t&0xFF, b)
		top++
	}
	bot := int(bt >> 8)
	height := bot - top
	if height > 0 {
		if l&0xFF != 0 {
			b.blitV(int(l>>8), top, height, uint8(l&0xFF))
		}
		if r&0xFF != 0 {
			b.blitV(int(r>>8), top, height, uint8(^r&0xFF))
		}
	}
	if bt&0xFF != 0 {
		innerScanline(l, bot, r, ^bt&0xFF, b)
	}
}

func alignThinStroke(edge1, edge2 *fdot8) {
	if fdot8Floor(*edge1) == fdot8Floor(*edge2) {
		*edge2 -= *edge1 & 0xFF
		*edge1 &^= 0xFF
	}
}

// antiFrameRect is SkScan::AntiFrameRect for a rectangular clip: the stroke
// of a rectangle with miter joins.
func antiFrameRect(r Rect, strokeSize Point, clip IRect, b blitter) {
	rx := strokeSize.X / 2
	ry := strokeSize.Y / 2
	if r.width() == 0 {
		ry = 0
	}
	if r.height() == 0 {
		rx = 0
	}
	outerL := scalarToFDot8(r.L - rx)
	outerT := scalarToFDot8(r.T - ry)
	outerR := scalarToFDot8(r.R + rx)
	outerB := scalarToFDot8(r.B + ry)

	outer := IRect{fdot8Floor(outerL), fdot8Floor(outerT), fdot8Ceil(outerR), fdot8Ceil(outerB)}
	// SkRegion::quickReject.
	if outer.isEmpty() || clip.isEmpty() || outer.L >= clip.R || clip.L >= outer.R || outer.T >= clip.B || clip.T >= outer.B {
		return
	}
	var clipper rectClipBlitter
	if !clip.contains(outer) {
		clipper = rectClipBlitter{b: b, clip: clip}
		b = &clipper
	}

	rx = strokeSize.X - rx
	ry = strokeSize.Y - ry

	innerL := scalarToFDot8(r.L + rx)
	innerT := scalarToFDot8(r.T + ry)
	innerR := scalarToFDot8(r.R - rx)
	innerB := scalarToFDot8(r.B - ry)

	if strokeSize.X < 1 || strokeSize.Y < 1 {
		alignThinStroke(&outerL, &innerL)
		alignThinStroke(&outerT, &innerT)
		alignThinStroke(&innerR, &outerR)
		alignThinStroke(&innerB, &outerB)
	}

	antiFillDot8(outerL, outerT, outerR, outerB, b, false)

	outer = IRect{fdot8Ceil(outerL), fdot8Ceil(outerT), fdot8Floor(outerR), fdot8Floor(outerB)}

	if innerL >= innerR || innerT >= innerB {
		fillCheckRect(outer.L, outer.T, outer.R, outer.B, b)
	} else {
		inner := IRect{fdot8Floor(innerL), fdot8Floor(innerT), fdot8Ceil(innerR), fdot8Ceil(innerB)}
		fillCheckRect(outer.L, outer.T, outer.R, inner.T, b)
		fillCheckRect(outer.L, inner.T, inner.L, inner.B, b)
		fillCheckRect(inner.R, inner.T, outer.R, inner.B, b)
		fillCheckRect(outer.L, inner.B, outer.R, outer.B, b)
		innerStrokeDot8(innerL, innerT, innerR, innerB, b)
	}
}

// fillIRect is SkScan::FillIRect for a rectangular clip.
func fillIRect(r IRect, clip IRect, b blitter) {
	if r.isEmpty() {
		return
	}
	if r.intersect(clip) {
		b.blitRect(r.L, r.T, r.width(), r.height())
	}
}

// fillRect is SkScan::FillRect: a rectangle without anti-aliasing. The
// edges round to the nearest pixel.
func fillRect(r Rect, clip IRect, b blitter) {
	if r.isEmpty() {
		return
	}
	fillIRect(r.round(), clip, b)
}
