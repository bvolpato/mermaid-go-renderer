// Glyph masks.
//
// Chromium on macOS asks CoreText for the mask of each glyph: an 8-bit
// coverage image at one of four horizontal subpixel offsets (Skia packs the
// offset into the glyph id). This package has no CoreText. It builds the
// mask from the outline of the glyph: the outline grows as the font
// smoothing of CoreGraphics makes it grow (see glyphModel), and an exact
// area fill gives the coverage (glyph_raster.go).

package raster

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/png" // sbix strikes are PNG images
	"math"
	"sync"

	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// glyphModel holds the parameters of the model of CoreGraphics glyph
// rasterization with font smoothing:
//
//   - CoreGraphics grows the outline of a glyph by an amount that is
//     proportional to the font size, up to a limit. The growth is the sum
//     with a rectangle: an edge moves by the growth in x and in y, with the
//     signs of its normal (growOutline). It keeps the bottom of the glyph
//     in place, in the device space.
//   - It flattens curves coarsely (flattenTolerance) and fills with exact
//     area coverage.
//   - For hinted fonts at whole sizes up to 18 px it fits the heights of
//     the small letters and of the capitals to the pixel grid
//     (glyph_zones.go).
//
// The values come from fits against Chromium screenshots
// (glyph_calibrate_test.go): TestFitGrowth and TestFitZones on the
// calibration sheets. TestGlyphModel states the residual error on the
// validation sheets.
type glyphModel struct {
	// GrowX and GrowY are the growth of the outline on each side, as a
	// fraction of the font size in device pixels.
	GrowX, GrowY float32
	// MaxGrowX and MaxGrowY are the largest growth on each side in device
	// pixels.
	MaxGrowX, MaxGrowY float32
	// ShiftY moves the grown outline up, as a fraction of the vertical
	// growth: 1 keeps the bottom edges in place.
	ShiftY float32
	// Fit turns on the vertical fitting of small text (glyph_zones.go).
	Fit bool
	// Transfer is the mapping from coverage to mask values.
	Transfer transferMode
}

// model is the glyph model in use.
var model = glyphModel{
	GrowX: 0.0058, GrowY: 0.0058,
	MaxGrowX: 0.3, MaxGrowY: 0.3,
	ShiftY:   1,
	Fit:      true,
	Transfer: transferCG,
}

// growth returns the growth of the outline on each side at a device font
// size.
func (m *glyphModel) growth(size float32) (dx, dy float32) {
	return min(float32(m.GrowX*size), m.MaxGrowX), min(float32(m.GrowY*size), m.MaxGrowY)
}

type glyphKey struct {
	face *font.Face
	gid  uint32
	// size is the font size of the scaler in device pixels.
	size float32
	// sub and subY are the subpixel offsets in quarters of a pixel.
	sub, subY uint8
	// lum selects the coverage transfer table.
	lum uint8
	// lcd asks for a mask with one coverage value per color channel
	// (SkMask::kLCD16_Format). lumG and lumB then select the tables of
	// green and blue, and lum the table of red.
	lcd        bool
	lumG, lumB uint8
	// rot is the matrix from the text space at the device font size to
	// the device, without the translation: a, b, c, d. It is zero for text
	// along the x axis.
	rot [4]float32
	// unhinted asks for the outline of a Linux font without hinting, also
	// for text along an axis.
	unhinted bool
}

// glyphMask is the mask of one glyph. The mask is placed with its origin
// at the integer device position of the glyph.
type glyphMask struct {
	// lcd holds one RGB565 coverage value per pixel for an LCD mask, with
	// the bounds of mask. It is nil for other masks.
	lcd  []uint16
	mask maskA8
	// color holds premultiplied RGBA pixels for a color glyph, with the
	// same bounds as mask. It is nil for an outline glyph.
	color []uint8
	empty bool
}

type glyphCache struct {
	mu    sync.Mutex
	masks map[glyphKey]*glyphMask
	// paths holds outlines in font units, y up.
	paths map[pathKey]*Path
	// zoneCache holds the zone heights of each face.
	zoneCache map[*font.Face]faceZones
	bytes     int

	scratch glyphScratch
}

type pathKey struct {
	face *font.Face
	gid  uint32
}

type glyphScratch struct {
	path   Path
	raster areaRaster
	cov    []float32
	aaa    aaaState
}

var (
	glyphCacheOnce sync.Once
	theGlyphCache  *glyphCache
)

// sharedGlyphCache returns the process-wide glyph cache. Masks depend only
// on the font and the size, so every render shares them.
func sharedGlyphCache() *glyphCache {
	glyphCacheOnce.Do(func() {
		theGlyphCache = &glyphCache{masks: map[glyphKey]*glyphMask{}, paths: map[pathKey]*Path{}}
	})
	return theGlyphCache
}

const maxGlyphCacheBytes = 64 << 20

// outline returns the outline of a glyph in font units with y up, or nil.
// The caller holds c.mu.
func (c *glyphCache) outline(face *font.Face, gid uint32) *Path {
	key := pathKey{face, gid}
	if p, ok := c.paths[key]; ok {
		return p
	}
	var p *Path
	if o, ok := face.GoText().GlyphDataOutline(gtfont.GID(gid)); ok && len(o.Segments) > 0 {
		p = NewPath()
		for _, seg := range o.Segments {
			a := seg.Args
			switch seg.Op {
			case ot.SegmentOpMoveTo:
				if !p.isEmpty() {
					p.Close()
				}
				p.MoveTo(a[0].X, a[0].Y)
			case ot.SegmentOpLineTo:
				p.LineTo(a[0].X, a[0].Y)
			case ot.SegmentOpQuadTo:
				p.QuadTo(a[0].X, a[0].Y, a[1].X, a[1].Y)
			case ot.SegmentOpCubeTo:
				p.CubicTo(a[0].X, a[0].Y, a[1].X, a[1].Y, a[2].X, a[2].Y)
			}
		}
		p.Close()
	}
	c.paths[key] = p
	return p
}

// devicePath writes the outline of a glyph in device pixels into dst: the
// origin of the glyph is at (sub/4, 0) and y points down.
func (c *glyphCache) devicePath(dst *Path, face *font.Face, gid uint32, size float32, subX float32) bool {
	src := c.outline(face, gid)
	if src == nil {
		return false
	}
	scale := size / float32(face.UnitsPerEm())
	m := Matrix{sx: scale, sy: -scale, tx: subX}
	src.transformInto(dst, &m)
	return true
}

// get returns the mask of a glyph at a device font size and a subpixel
// offset. lum is the table index of the text color (lumBucket). rot maps
// the text space at the device font size to the device: it is zero for
// upright text. unhinted is glyphKey.unhinted.
func (c *glyphCache) get(face *font.Face, gid uint32, size float32, rot [4]float32, subX, subY uint8, lum uint8, unhinted bool) *glyphMask {
	return c.getKey(glyphKey{face: face, gid: gid, size: size, sub: subX, subY: subY, lum: lum, rot: rot, unhinted: unhinted})
}

// getLCD returns the LCD mask of a glyph of a Linux font. lum holds the
// table indexes of red, green and blue.
func (c *glyphCache) getLCD(face *font.Face, gid uint32, size float32, rot [4]float32, subX, subY uint8, lum [3]uint8, unhinted bool) *glyphMask {
	return c.getKey(glyphKey{face: face, gid: gid, size: size, sub: subX, subY: subY, lum: lum[0], lumG: lum[1], lumB: lum[2], lcd: true, rot: rot, unhinted: unhinted})
}

func (c *glyphCache) getKey(key glyphKey) *glyphMask {
	c.mu.Lock()
	defer c.mu.Unlock()
	if g, ok := c.masks[key]; ok {
		return g
	}
	g := c.build(&key)
	if c.bytes > maxGlyphCacheBytes {
		c.masks = map[glyphKey]*glyphMask{}
		c.bytes = 0
	}
	c.masks[key] = g
	c.bytes += len(g.mask.image) + len(g.color) + 2*len(g.lcd) + 64
	return g
}

func (c *glyphCache) build(key *glyphKey) *glyphMask {
	face, gid, lum, size := key.face, key.gid, key.lum, key.size
	rotated := key.rot != [4]float32{}
	if bm, ok := face.GoText().GlyphData(gtfont.GID(gid)).(gtfont.GlyphBitmap); ok && !rotated {
		if g := c.buildColor(face, gid, bm, size); g != nil {
			return g
		}
	}
	s := &c.scratch
	if !c.devicePath(&s.path, face, gid, size, 0) {
		return &glyphMask{empty: true}
	}
	path := &s.path
	if face.Platform() != font.PlatformDarwin {
		// The outline that the scaler of the platform draws. Skia keeps
		// the hinting for text along an axis, which includes a quarter
		// turn and a flip, and turns it off for other text
		// (SkTypeface_Fontations::onFilterRec). The matrix of the glyph
		// applies to the outline after the hinter.
		r := key.rot
		axisAligned := !rotated || (r[1] == 0 && r[2] == 0) || (r[0] == 0 && r[3] == 0)
		if axisAligned && !key.unhinted {
			if o, ok := face.ScalerOutline(gid, float64(size)); ok {
				appendOutline(path, o.Segments)
			}
		} else if o, ok := face.ScalerOutlineUnhinted(gid, float64(size)); ok {
			appendOutline(path, o.Segments)
		}
		return c.buildSkia(key, path)
	}
	dx, dy := model.growth(size)
	if rotated {
		// The growth gets smaller as the baseline turns away from the axes
		// and is zero on a diagonal: the factor is sqrt(|cos 2a|) for the
		// angle a of the baseline. The screenshots give 1 at 0 and 90
		// degrees, about 0.72 at 30 degrees and 0 at 45 degrees.
		bx, by := key.rot[0], key.rot[1]
		if n := float32(bx*bx) + float32(by*by); n > 0 {
			c2 := (float32(bx*bx) - float32(by*by)) / n
			f := float32(math.Sqrt(float64(fabs32(c2))))
			dx, dy = float32(dx*f), float32(dy*f)
		}
	}
	grow := func() {
		if dx > 0 || dy > 0 {
			growOutline(path, dx, dy)
			if shift := float32(model.ShiftY * dy); shift != 0 {
				for i := range path.pts {
					path.pts[i].Y -= shift
				}
			}
		}
	}
	if !rotated {
		grow()
		zones := c.zones(face)
		if fit := zones.fitFor(size, float32(dy*(1+model.ShiftY))); fit.active {
			// The path has y down and the baseline at 0.
			for i := range path.pts {
				path.pts[i].Y = -fit.apply(-path.pts[i].Y)
			}
		}
	} else {
		// CoreGraphics grows the outline in the device space: the growth
		// keeps the lowest edges of the rotated glyph in place.
		r := key.rot
		for i := range path.pts {
			x, y := path.pts[i].X, path.pts[i].Y
			path.pts[i].X = float32(r[0]*x) + float32(r[2]*y)
			path.pts[i].Y = float32(r[1]*x) + float32(r[3]*y)
		}
		grow()
	}
	if ox, oy := float32(float32(key.sub)*0.25), float32(float32(key.subY)*0.25); ox != 0 || oy != 0 {
		for i := range path.pts {
			path.pts[i].X += ox
			path.pts[i].Y += oy
		}
	}
	bounds := path.bounds().roundOut()
	if bounds.isEmpty() || bounds.width() > 4096 || bounds.height() > 4096 {
		return &glyphMask{empty: true}
	}
	// One pixel of room on each side, as Skia gives CoreGraphics.
	bounds = bounds.outset(1, 1)
	w, h := bounds.width(), bounds.height()
	s.raster.reset(w, h)
	s.raster.addPath(path, float32(bounds.L), float32(bounds.T))
	if cap(s.cov) < w*h {
		s.cov = make([]float32, w*h)
	}
	cov := s.cov[:w*h]
	s.raster.coverage(cov)
	image := make([]uint8, w*h)
	coverageToMask(image, cov, lum)
	return &glyphMask{mask: maskA8{image: image, bounds: bounds, rowB: w}}
}

// appendOutline replaces a path by an outline in pixels with y up.
func appendOutline(p *Path, segments []ot.Segment) {
	p.Reset()
	for _, seg := range segments {
		a := seg.Args
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			if !p.isEmpty() {
				p.Close()
			}
			p.MoveTo(a[0].X, -a[0].Y)
		case ot.SegmentOpLineTo:
			p.LineTo(a[0].X, -a[0].Y)
		case ot.SegmentOpQuadTo:
			p.QuadTo(a[0].X, -a[0].Y, a[1].X, -a[1].Y)
		case ot.SegmentOpCubeTo:
			p.CubicTo(a[0].X, -a[0].Y, a[1].X, -a[1].Y, a[2].X, -a[2].Y)
		}
	}
	p.Close()
}

// buildSkia builds the mask of a glyph as Chromium does on Linux: the scan
// converter of Skia fills the outline (SkScalerContext::
// GenerateImageFromPath), and the table of the text luminance applies. path
// is the outline at the device font size.
//
// The Fontations backend of Skia hints the outline of text along an axis
// with the automatic hinter in its light mode. The caller takes that
// outline from internal/font (Face.ScalerOutline), and the outline without
// hinting for other text (Face.ScalerOutlineUnhinted).
func (c *glyphCache) buildSkia(key *glyphKey, path *Path) *glyphMask {
	if key.rot != [4]float32{} {
		r := key.rot
		for i := range path.pts {
			x, y := path.pts[i].X, path.pts[i].Y
			path.pts[i].X = float32(r[0]*x) + float32(r[2]*y)
			path.pts[i].Y = float32(r[1]*x) + float32(r[3]*y)
		}
		path.convex = convexityUnknown
	}
	if ox, oy := float32(float32(key.sub)*0.25), float32(float32(key.subY)*0.25); ox != 0 || oy != 0 {
		for i := range path.pts {
			path.pts[i].X += ox
			path.pts[i].Y += oy
		}
	}
	bounds := path.bounds().roundOut()
	if bounds.isEmpty() || bounds.width() > 4096 || bounds.height() > 4096 {
		return &glyphMask{empty: true}
	}
	path.fill = fillWinding
	if key.lcd {
		return c.buildLCD(key, path, bounds)
	}
	clip := newAAClip(bounds)
	b := maskBuilderBlitter{clip: clip}
	c.scratch.aaa.antiFillPath(path, bounds, &b)
	applyLinuxPreBlend(clip.mask, key.lum)
	return &glyphMask{mask: maskA8{image: clip.mask, bounds: bounds, rowB: clip.stride}}
}

// lcdCoefficients are the filters of pack4xHToMask: one for each of red,
// green and blue, over 12 samples at four samples per pixel.
var lcdCoefficients = [3][12]int{
	{0x03, 0x0b, 0x1c, 0x33, 0x40, 0x39, 0x24, 0x10, 0x05, 0x01, 0x00, 0x00},
	{0x00, 0x02, 0x08, 0x16, 0x2b, 0x3d, 0x3d, 0x2b, 0x16, 0x08, 0x02, 0x00},
	{0x00, 0x00, 0x01, 0x05, 0x10, 0x24, 0x39, 0x40, 0x33, 0x1c, 0x0b, 0x03},
}

// buildLCD builds an LCD mask: SkScalerContext::GenerateImageFromPath for
// SkMask::kLCD16_Format and pack4xHToMask. The outline is filled at four
// times the horizontal resolution, and a filter per color channel reduces
// it. The mask is one pixel wider than the outline on each side.
func (c *glyphCache) buildLCD(key *glyphKey, path *Path, pathBounds IRect) *glyphMask {
	bounds := pathBounds.outset(1, 0)
	w, h := bounds.width(), bounds.height()
	sw := 4 * (w - 2)
	// The outline in the space of the samples.
	origin := float32(bounds.L + 1)
	for i := range path.pts {
		path.pts[i].X = float32((path.pts[i].X - origin) * 4)
	}
	path.convex = convexityUnknown
	sb := IRect{0, bounds.T, sw, bounds.B}
	clip := newAAClip(sb)
	b := maskBuilderBlitter{clip: clip}
	c.scratch.aaa.antiFillPath(path, sb, &b)
	transferOnce.Do(buildTransferTables)
	tables := [3]*[256]uint8{&linuxTables[key.lum&7], &linuxTables[key.lumG&7], &linuxTables[key.lumB&7]}
	lcd := make([]uint16, w*h)
	alpha := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		src := clip.mask[y*clip.stride : y*clip.stride+sw]
		for px, sx := 0, -4; sx < sw+4; px, sx = px+1, sx+4 {
			var fir [3]int
			for si := max(0, sx-4); si < min(sx+8, sw); si++ {
				v := int(src[si])
				ci := si - (sx - 4)
				fir[0] += lcdCoefficients[0][ci] * v
				fir[1] += lcdCoefficients[1][ci] * v
				fir[2] += lcdCoefficients[2][ci] * v
			}
			var rgb [3]uint16
			for k := 0; k < 3; k++ {
				rgb[k] = uint16(tables[k][min(fir[k]/0x100, 255)])
			}
			// SkPack888ToRGB16.
			lcd[y*w+px] = (rgb[0]>>3)<<11 | (rgb[1]>>2)<<5 | rgb[2]>>3
			alpha[y*w+px] = uint8(max(rgb[0], rgb[1], rgb[2]))
		}
	}
	return &glyphMask{lcd: lcd, mask: maskA8{image: alpha, bounds: bounds, rowB: w}}
}

// buildColor builds the mask of a color bitmap glyph (Apple Color Emoji):
// the PNG strike scaled to the size that CoreText draws.
func (c *glyphCache) buildColor(face *font.Face, gid uint32, bm gtfont.GlyphBitmap, size float32) *glyphMask {
	if bm.Format != gtfont.PNG {
		return nil
	}
	left, top, right, bottom, ok := face.ColorGlyphBounds(gid, float64(size), false)
	if !ok || !(right > left && bottom > top) {
		return nil
	}
	src, _, err := image.Decode(bytes.NewReader(bm.Data))
	if err != nil {
		return nil
	}
	// The bounds of Skia have one pixel of room on each side.
	bounds := IRect{int(left) + 1, int(top) + 1, int(right) - 1, int(bottom) - 1}
	w, h := bounds.width(), bounds.height()
	if w <= 0 || h <= 0 {
		return nil
	}
	nrgba := image.NewNRGBA(src.Bounds())
	draw.Draw(nrgba, nrgba.Bounds(), src, src.Bounds().Min, draw.Src)
	out := make([]uint8, w*h*4)
	scaleImagePremul(out, w, h, nrgba)
	alpha := make([]uint8, w*h)
	for i := range alpha {
		alpha[i] = out[i*4+3]
	}
	return &glyphMask{mask: maskA8{image: alpha, bounds: bounds, rowB: w}, color: out}
}

// scaleImagePremul resamples an image to w by h premultiplied RGBA pixels
// with a box filter in premultiplied space.
func scaleImagePremul(dst []uint8, w, h int, src *image.NRGBA) {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return
	}
	fx := float64(sw) / float64(w)
	fy := float64(sh) / float64(h)
	for y := 0; y < h; y++ {
		y0, y1 := float64(float64(y)*fy), float64(float64(y+1)*fy)
		for x := 0; x < w; x++ {
			x0, x1 := float64(float64(x)*fx), float64(float64(x+1)*fx)
			var r, g, b, a, wsum float64
			for sy := int(y0); sy < sh && float64(sy) < y1; sy++ {
				wy := min(y1, float64(sy+1)) - max(y0, float64(sy))
				for sx := int(x0); sx < sw && float64(sx) < x1; sx++ {
					wx := min(x1, float64(sx+1)) - max(x0, float64(sx))
					wt := float64(wx * wy)
					o := sy*src.Stride + sx*4
					pa := float64(src.Pix[o+3]) / 255
					r += float64(float64(src.Pix[o]) * pa * wt)
					g += float64(float64(src.Pix[o+1]) * pa * wt)
					b += float64(float64(src.Pix[o+2]) * pa * wt)
					a += float64(float64(src.Pix[o+3]) * wt)
					wsum += wt
				}
			}
			if wsum > 0 {
				o := (y*w + x) * 4
				dst[o] = uint8(r/wsum + 0.5)
				dst[o+1] = uint8(g/wsum + 0.5)
				dst[o+2] = uint8(b/wsum + 0.5)
				dst[o+3] = uint8(a/wsum + 0.5)
			}
		}
	}
}
