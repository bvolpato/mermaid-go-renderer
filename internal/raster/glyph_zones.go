// The vertical fitting of small text.
//
// This file is not a port: CoreText is closed source. Chromium screenshots
// show that CoreText moves the horizontal features of hinted TrueType fonts
// when the font size in device pixels is a whole number up to 18: the top
// of the small letters and the top of the capitals go to pixel borders, and
// what is between them scales. Sizes above 18 and sizes with a fraction
// have no fitting. The model is a piecewise linear map of the height above
// the baseline (see zoneFit). glyph_calibrate_test.go measures it.

package raster

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// maxFitSize is the largest font size in device pixels with fitting.
const maxFitSize = 18

// fitSizeTolerance is the largest distance of a font size from a whole
// number that still gets the fitting.
var fitSizeTolerance = float32(1.0 / 128)

// fitRound is the fraction from which the top of the small letters rounds
// up to the next pixel.
const fitRound = 0.46

// capStem is the height of the band below the top of the capitals that
// moves with it, as a fraction of the font size: the thickness of a
// horizontal stem.
const capStem = 0.115

// faceZones holds the heights of a font that the fitting uses, as
// fractions of the em.
type faceZones struct {
	// xTop is the top of the letter o, with its overshoot.
	xTop float32
	// capTop is the top of the letter H.
	capTop float32
	// table has the measured fits of the family, or nil for no fitting.
	table *zoneTable
	ok    bool
}

// zoneTable holds the measured height in pixels of the top of the
// capitals of a font family, per font size. 0 means that CoreText leaves
// the capitals in place.
type zoneTable struct {
	caps [maxFitSize + 1]int8
}

// zoneTables are the fits of the font families that the themes of mermaid
// use, and of some common web fonts. The values come from TestFitZones. A
// family without a table gets no fitting: the fitting depends on the
// hinting data of the font.
var zoneTables = map[string]*zoneTable{
	"arial":        {caps: [maxFitSize + 1]int8{10: 7, 12: 9, 15: 11, 16: 12, 17: 12, 18: 13}},
	"helvetica":    {caps: [maxFitSize + 1]int8{11: 8, 12: 9, 16: 12, 17: 12, 18: 13}},
	"trebuchet ms": {caps: [maxFitSize + 1]int8{11: 8, 12: 9, 15: 11, 16: 12, 17: 12, 18: 13}},
	"courier":      {caps: [maxFitSize + 1]int8{9: 5, 10: 6, 14: 9, 15: 9, 16: 10, 17: 10, 18: 11}},
	"times":        {caps: [maxFitSize + 1]int8{9: 6, 10: 7, 12: 8, 13: 9, 14: 10, 15: 10, 16: 11, 17: 12, 18: 12}},
	"verdana":      {caps: [maxFitSize + 1]int8{11: 8, 12: 9, 13: 9, 17: 13, 18: 13}},
	"menlo":        {caps: [maxFitSize + 1]int8{10: 8, 11: 8, 12: 9, 13: 9, 16: 12, 17: 13, 18: 13}},
	"georgia":      {caps: [maxFitSize + 1]int8{9: 7, 10: 7, 11: 8, 14: 10, 15: 11, 17: 12, 18: 13}},
}

// zones returns the zone heights of a face. The caller holds c.mu.
func (c *glyphCache) zones(face *font.Face) faceZones {
	if z, ok := c.zoneCache[face]; ok {
		return z
	}
	var z faceZones
	top := func(r rune) (float32, bool) {
		// NominalGlyph of the go-text face writes a cache, and the face is
		// shared by all goroutines. The method of font.Face is safe.
		gid, ok := face.NominalGlyph(r)
		if !ok {
			return 0, false
		}
		p := c.outline(face, gid)
		if p == nil {
			return 0, false
		}
		return outlineTop(p) / float32(face.UnitsPerEm()), true
	}
	x, okX := top('o')
	h, okH := top('H')
	if okX && okH && x > 0 && h > x {
		z = faceZones{xTop: x, capTop: h, ok: true, table: zoneTables[strings.ToLower(face.Family)]}
	}
	if c.zoneCache == nil {
		c.zoneCache = map[*font.Face]faceZones{}
	}
	c.zoneCache[face] = z
	return z
}

// outlineTop returns the largest y of the curves of an outline.
func outlineTop(p *Path) float32 {
	top := float32(math.Inf(-1))
	var cur Point
	pi := 0
	for _, v := range p.verbs {
		switch v {
		case verbMove, verbLine:
			cur = p.pts[pi]
			top = max(top, cur.Y)
			pi++
		case verbQuad, verbConic:
			c, e := p.pts[pi], p.pts[pi+1]
			top = max(top, e.Y)
			// The extreme of the curve, when it is inside the curve.
			if d := cur.Y - float32(2*c.Y) + e.Y; d != 0 {
				if t := (cur.Y - c.Y) / d; t > 0 && t < 1 {
					pts := [3]Point{cur, c, e}
					top = max(top, evalQuadAt(pts[:], t).Y)
				}
			}
			cur = e
			pi += 2
		case verbCubic:
			pts := [4]Point{cur, p.pts[pi], p.pts[pi+1], p.pts[pi+2]}
			for i := 1; i <= 8; i++ {
				top = max(top, evalCubicPosAt(pts[:], float32(i)/8).Y)
			}
			cur = pts[3]
			pi += 3
		}
	}
	return top
}

// zoneFit is the map of the height above the baseline for one font and
// size. Heights up to x scale by xScale. Heights from k up move by
// capShift. Heights between x and k go linearly from one to the other.
type zoneFit struct {
	active   bool
	x, k     float32
	xScale   float32
	capShift float32
}

// fitFor returns the fitting of a face at a font size in device pixels.
// grow is the vertical growth of the outline: the top of a flat stem moves
// up by grow.
func (z *faceZones) fitFor(size, grow float32) zoneFit {
	if !model.Fit || !z.ok || z.table == nil {
		return zoneFit{}
	}
	n := int(size + 0.5)
	if n < 9 || n > maxFitSize || fabs32(size-float32(n)) > fitSizeTolerance {
		return zoneFit{}
	}
	x := float32(z.xTop*size) + grow
	fitted := floor32(x + (1 - fitRound))
	if fitted < 1 {
		return zoneFit{}
	}
	f := zoneFit{active: true, x: x, xScale: fitted / x}
	capTop := float32(z.capTop*size) + grow
	f.k = max(x, capTop-float32(capStem*size))
	if z.table.caps[n] != 0 {
		f.capShift = float32(z.table.caps[n]) - capTop
	}
	return f
}

// apply maps a height above the baseline.
func (f *zoneFit) apply(y float32) float32 {
	switch {
	case y <= 0:
		return y
	case y <= f.x:
		return float32(y * f.xScale)
	case y >= f.k:
		return y + f.capShift
	}
	lo := float32(f.x * f.xScale)
	hi := f.k + f.capShift
	t := (y - f.x) / (f.k - f.x)
	return lo + float32(t*(hi-lo))
}
