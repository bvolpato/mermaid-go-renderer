// This file adds an original PDF target to the existing SVG walker.
// Path conversion uses the existing Skia conic subdivision from chrome/m156.
package raster

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/pdf"
	gtfont "github.com/go-text/typesetting/font"
)

// PDFPage describes the paper and diagram placement in CSS pixels.
// Zero dimensions use the scaled SVG size. A nil Background means white.
// Pages defaults to one. Each later page shows the next vertical slice.
type PDFPage struct {
	Width, Height, X, Y float64
	Background          color.Color
	Pages               int
}

// RenderPDF paints the SVG with vector paths and embedded text where supported.
// Options.Background paints the diagram box. PDFPage.Background paints the paper.
// Drop shadows use flat silhouettes. Other filters and external images are omitted.
// Unsupported font faces use outlines.
func RenderPDF(svg string, opts Options, page PDFPage) ([]byte, error) {
	doc, err := dom.ParseBody(svg)
	if err != nil {
		return nil, err
	}
	doc.PageSheets = opts.PageSheets
	eng := browser.NewWithFonts(opts.Fonts)
	eng.ViewportWidth, eng.BodyMargin = pageWidth, 0
	doc.Engine = eng
	var root *dom.Node
	for c := doc.Body.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.ElementNode && c.NS == dom.NSSVG && c.Tag == "svg" {
			root = c
			break
		}
	}
	if root == nil {
		return nil, errors.New("raster: no svg element")
	}
	scale := opts.Scale
	if scale == 0 {
		scale = 1
	}
	if !finitePDF(scale) || scale <= 0 {
		return nil, errors.New("raster: invalid PDF scale")
	}
	sz := rootSize(eng, root)
	w, h := float64(sz.cssW)*scale, float64(sz.cssH)*scale
	// The walker uses integer clip bounds. Keep conversion within that range.
	if !finitePDF(w) || !finitePDF(h) || w <= 0 || h <= 0 || w > 1<<30 || h > 1<<30 {
		return nil, errors.New("raster: empty or oversized PDF svg")
	}
	if page.Width == 0 {
		page.Width = w
	}
	if page.Height == 0 {
		page.Height = h
	}
	if !finitePDF(page.Width) || !finitePDF(page.Height) || page.Width <= 0 || page.Height <= 0 ||
		!finitePDF(page.X) || !finitePDF(page.Y) {
		return nil, errors.New("raster: invalid PDF page geometry")
	}
	if page.Pages == 0 {
		page.Pages = 1
	}
	if page.Pages < 1 || page.Pages > 10000 {
		return nil, errors.New("raster: invalid PDF page count")
	}
	out := pdf.New(page.Width, page.Height)
	v := &vectorPainter{doc: out, unicode: make(map[*font.Face]map[uint32]string)}
	for i := 0; i < page.Pages; i++ {
		if i != 0 {
			out.NewPage(page.Width, page.Height)
		}
		v.background(page.Background, page.Width, page.Height)
		out.Save()
		out.Transform(pdf.Matrix{1, 0, 0, 1, page.X, page.Y - float64(float64(i)*page.Height)})
		cv := &canvas{w: int(math.Ceil(w)), h: int(math.Ceil(h))}
		cv.dr.vector = v
		cv.stack = []*device{{clip: IRect{0, 0, cv.w, cv.h}}}
		p := &painter{eng: eng, doc: doc, cv: cv, scale: scale, glyphs: sharedGlyphCache(), vector: v}
		v.background(opts.Background, w, h)
		p.paintRoot(root, sz)
		out.Restore()
	}
	return out.Bytes()
}

func finitePDF(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

type vectorPainter struct {
	doc     *pdf.Document
	unicode map[*font.Face]map[uint32]string
	names   map[*font.Face]map[string]string
	shadow  *pdf.Color
}

func (v *vectorPainter) background(c color.Color, w, h float64) {
	r, g, b, a := uint8(255), uint8(255), uint8(255), uint8(255)
	if c != nil {
		r, g, b, a = colorToNRGBA(c)
	}
	if a == 0 {
		return
	}
	v.doc.Draw(fmt.Sprintf("0 0 %s %s re\n", pdfNum(w), pdfNum(h)), false, pdf.Style{Color: pdf.Color{R: r, G: g, B: b, A: a}})
}

func pdfNum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
func pdfMatrix(m *Matrix) pdf.Matrix {
	return pdf.Matrix{float64(m.sx), float64(m.ky), float64(m.kx), float64(m.sy), float64(m.tx), float64(m.ty)}
}

func pdfStyle(p *Paint) pdf.Style {
	s := pdf.Style{Color: pdf.Color{R: p.R, G: p.G, B: p.B, A: p.A}, Stroke: p.stroke, Width: float64(p.width), Miter: float64(p.miterLimit), Cap: int(p.cap), Join: int(p.join)}
	if p.dash != nil {
		for _, d := range p.dash.intervals {
			s.Dash = append(s.Dash, float64(d))
		}
		s.DashOffset = float64(p.dash.phase)
	}
	return s
}

// pdfPath converts quadratic curves to cubic curves without flattening lines.
// Rational conics use the same bounded subdivision as the raster path code.
func pdfPath(p *Path) string {
	if p == nil || !p.isFinite() {
		return ""
	}
	var b strings.Builder
	valid := true
	xy := func(x, y float64) {
		if !finitePDF(x) || !finitePDF(y) {
			valid = false
			return
		}
		b.WriteString(pdfNum(x))
		b.WriteByte(' ')
		b.WriteString(pdfNum(y))
		b.WriteByte(' ')
	}
	pt := func(p Point) { xy(float64(p.X), float64(p.Y)) }
	quad := func(a, c, z Point) {
		x, y := float64(a.X), float64(a.Y)
		cx, cy, zx, zy := float64(c.X), float64(c.Y), float64(z.X), float64(z.Y)
		xy(x+float64((cx-x)*(2.0/3)), y+float64((cy-y)*(2.0/3)))
		xy(zx+float64((cx-zx)*(2.0/3)), zy+float64((cy-zy)*(2.0/3)))
		pt(z)
		b.WriteString("c\n")
	}
	var cur, first Point
	pi, wi := 0, 0
	for _, verb := range p.verbs {
		switch verb {
		case verbMove:
			cur = p.pts[pi]
			first = cur
			pi++
			pt(cur)
			b.WriteString("m\n")
		case verbLine:
			cur = p.pts[pi]
			pi++
			pt(cur)
			b.WriteString("l\n")
		case verbQuad:
			quad(cur, p.pts[pi], p.pts[pi+1])
			cur = p.pts[pi+1]
			pi += 2
		case verbConic:
			var buf [1 + 2*(1<<5)]Point
			q := conicToQuads(cur, p.pts[pi], p.pts[pi+1], p.conics[wi], 0.01, buf[:])
			for k := 0; k+2 < len(q); k += 2 {
				quad(q[k], q[k+1], q[k+2])
			}
			cur = p.pts[pi+1]
			pi += 2
			wi++
		case verbCubic:
			pt(p.pts[pi])
			pt(p.pts[pi+1])
			pt(p.pts[pi+2])
			b.WriteString("c\n")
			cur = p.pts[pi+2]
			pi += 3
		case verbClose:
			b.WriteString("h\n")
			cur = first
		}
	}
	if !valid {
		return ""
	}
	return b.String()
}

func (v *vectorPainter) drawPath(dev *device, path *Path, paint *Paint, m *Matrix) {
	if dev.clip.isEmpty() || path.isEmpty() || !path.isFinite() {
		return
	}
	if paint.shader != nil && v.shadow == nil {
		v.drawGradient(path, paint, m)
		return
	}
	v.doc.Save()
	defer v.doc.Restore()
	v.doc.Transform(pdfMatrix(m))
	v.doc.Draw(pdfPath(path), path.fill == fillEvenOdd, v.style(paint))
}

func (v *vectorPainter) style(paint *Paint) pdf.Style {
	s := pdfStyle(paint)
	if v.shadow != nil {
		s.Color.R, s.Color.G, s.Color.B = v.shadow.R, v.shadow.G, v.shadow.B
	}
	return s
}

func (v *vectorPainter) clipPath(path *Path) bool {
	if path.isEmpty() {
		v.doc.Clip("0 0 0 0 re\n", false)
		return false
	}
	v.doc.Clip(pdfPath(path), path.fill == fillEvenOdd)
	return true
}

func (v *vectorPainter) glyphText(face *font.Face, gid uint32) string {
	chars, ok := v.unicode[face]
	if !ok {
		chars = make(map[uint32]string)
		names := make(map[string]string)
		it := face.GoText().Cmap.Iter()
		for it.Next() {
			r, g := it.Char()
			if r >= 32 && g != 0 {
				if _, has := chars[uint32(g)]; !has {
					chars[uint32(g)] = string(r)
					if name := face.GoText().GlyphName(g); name != "" {
						names[name] = string(r)
					}
				}
			}
		}
		v.unicode[face] = chars
		if v.names == nil {
			v.names = make(map[*font.Face]map[string]string)
		}
		v.names[face] = names
	}
	if chars[gid] == "" && gid != 0 {
		// Recursive substitutes l.sans for l. Its nominal glyph name retains
		// the exact Unicode mapping when the alternate has no cmap entry.
		name := face.GoText().GlyphName(gtfont.GID(gid))
		if base, suffix, ok := strings.Cut(name, "."); ok && suffix == "sans" {
			chars[gid] = v.names[face][base]
		}
	}
	return chars[gid]
}

func (p *painter) drawVectorGlyphRun(run *glyphRun, paint *Paint, ctm Affine) {
	var advance float32
	for _, g := range run.glyphs {
		x, y := run.x+advance+float32(g.XOffset), run.y-float32(g.YOffset)
		advance += float32(g.Advance)
		if g.Face == nil {
			continue
		}
		m := deviceMatrix(ctm)
		p.vector.doc.Save()
		p.vector.doc.Transform(pdfMatrix(&m))
		written := false
		if paint.shader == nil && p.vector.shadow == nil {
			written = p.vector.doc.Glyph(g.Face, g.ID, p.vector.glyphText(g.Face, g.ID), float64(run.fontSize), float64(x), float64(y), pdfStyle(paint))
		}
		p.vector.doc.Restore()
		if written {
			continue
		}
		// Outlines retain variable font instances and unsupported font formats.
		o, ok := g.Face.ScalerOutlineUnhinted(g.ID, 64)
		var local Path
		if ok && len(o.Segments) != 0 {
			path := NewPath()
			appendOutline(path, o.Segments)
			scale := run.fontSize / 64
			sm := Matrix{sx: scale, sy: scale, tx: x, ty: y}
			path.transformInto(&local, &sm)
		} else {
			// Faces outside the Linux scaler expose outlines in font units.
			p.glyphs.mu.Lock()
			path := p.glyphs.outline(g.Face, g.ID)
			if path != nil {
				scale := float32(float64(run.fontSize) / g.Face.UnitsPerEm())
				sm := Matrix{sx: scale, sy: -scale, tx: x, ty: y}
				path.transformInto(&local, &sm)
			}
			p.glyphs.mu.Unlock()
			if path == nil {
				continue
			}
		}
		p.cv.dr.drawPath(p.cv.dev(), &local, paint, &m)
	}
}

func (v *vectorPainter) gradientColor(g *gradientShader, t float32, alpha uint8) pdf.Color {
	switch g.spread {
	case 1:
		t = float32(math.Mod(float64(t), 2))
		if t < 0 {
			t += 2
		}
		if t > 1 {
			t = 2 - t
		}
	case 2:
		t -= float32(math.Floor(float64(t)))
	default:
		t = min(max(t, 0), 1)
	}
	c := g.colorAt(t)
	if !(c.a > 0) {
		return pdf.Color{}
	}
	toByte := func(f float32) uint8 { return uint8(min(max(float32(f*255)+0.5, 0), 255)) }
	return pdf.Color{R: toByte(c.r / c.a), G: toByte(c.g / c.a), B: toByte(c.b / c.a), A: toByte(float32(c.a*float32(alpha)) / 255)}
}

// drawGradient approximates continuous gradients with clipped vector bands.
func (v *vectorPainter) drawGradient(path *Path, paint *Paint, m *Matrix) {
	g, ok := paint.shader.(*gradientShader)
	if !ok {
		return
	}
	src := path
	var stroked, dashed Path
	if paint.stroke {
		sp := strokeParams{width: paint.width, miterLimit: paint.miterLimit, cap: paint.cap, join: paint.join, resScale: resScaleForStroking(m)}
		if paint.dash != nil {
			var scratch dashScratch
			if good, _ := dashPath(&dashed, src, &sp, false, nil, paint.dash, &scratch); good {
				src = &dashed
			}
		}
		var scratch strokeScratch
		strokePath(src, &stroked, &sp, &scratch)
		src = &stroked
	}
	var devicePath Path
	src.transformInto(&devicePath, m)
	if !devicePath.isFinite() {
		return
	}
	gm, ok := g.inv.invert()
	if !ok {
		return
	}
	bounds := g.inv.mapRect(devicePath.bounds())
	if bounds.isEmpty() {
		return
	}
	v.doc.Save()
	defer v.doc.Restore()
	v.clipPath(&devicePath)
	v.doc.Transform(pdfMatrix(&gm))
	stops := make([]pdf.GradientStop, len(g.stops))
	for i, s := range g.stops {
		toByte := func(c float32) uint8 { return uint8(min(max(float32(c*255)+0.5, 0), 255)) }
		stops[i] = pdf.GradientStop{Offset: float64(s.offset), Color: pdf.Color{R: toByte(s.r), G: toByte(s.g), B: toByte(s.b), A: toByte(s.a)}}
	}
	if g.spread == 0 {
		coords := [6]float64{float64(g.x0), float64(g.y0)}
		if g.radial {
			coords[3], coords[4], coords[5] = float64(g.x0), float64(g.y0), float64(g.r)
		} else {
			den := float32(g.dx*g.dx) + float32(g.dy*g.dy)
			if den > 0 {
				coords[2], coords[3] = float64(g.x0)+float64(g.dx/den), float64(g.y0)+float64(g.dy/den)
			}
		}
		if v.doc.Gradient(coords, g.radial, stops, float64(paint.A)/255) {
			return
		}
	} else {
		var coords [6]float64
		lo, hi := float64(0), float64(0)
		if g.radial {
			for _, q := range []Point{{bounds.L, bounds.T}, {bounds.R, bounds.T}, {bounds.R, bounds.B}, {bounds.L, bounds.B}} {
				hi = max(hi, math.Hypot(float64(q.X-g.x0), float64(q.Y-g.y0))/float64(g.r))
			}
			hi = math.Ceil(hi)
			coords = [6]float64{float64(g.x0), float64(g.y0), 0, float64(g.x0), float64(g.y0), float64(hi * float64(g.r))}
		} else {
			den := float32(g.dx*g.dx) + float32(g.dy*g.dy)
			if den > 0 {
				vx, vy := g.dx/den, g.dy/den
				basis := Matrix{sx: vx, ky: vy, kx: -vy, sy: vx, tx: g.x0, ty: g.y0}
				if inv, ok := basis.invert(); ok {
					box := inv.mapRect(bounds)
					lo, hi = math.Floor(float64(box.L)), math.Ceil(float64(box.R))
					coords = [6]float64{float64(g.x0) + float64(lo*float64(vx)), float64(g.y0) + float64(lo*float64(vy)), float64(g.x0) + float64(hi*float64(vx)), float64(g.y0) + float64(hi*float64(vy))}
				}
			}
		}
		if expanded := repeatPDFStops(stops, lo, hi, g.spread == 1); len(expanded) > 0 && v.doc.Gradient(coords, g.radial, expanded, float64(paint.A)/255) {
			return
		}
	}
	const bands = 256
	if g.radial {
		radius := float32(0)
		for _, q := range []Point{{bounds.L, bounds.T}, {bounds.R, bounds.T}, {bounds.R, bounds.B}, {bounds.L, bounds.B}} {
			radius = max(radius, float32(math.Hypot(float64(q.X-g.x0), float64(q.Y-g.y0))))
		}
		for i := 0; i < bands; i++ {
			a := float32(float32(float32(i)*radius) / bands)
			b := float32(float32(float32(i+1)*radius) / bands)
			ring := NewPath()
			ring.AddOval(Rect{g.x0 - b, g.y0 - b, g.x0 + b, g.y0 + b}, 1)
			if a > 0 {
				ring.AddOval(Rect{g.x0 - a, g.y0 - a, g.x0 + a, g.y0 + a}, 1)
			}
			v.doc.Draw(pdfPath(ring), true, pdf.Style{Color: v.gradientColor(g, float32((a+b)/2)/g.r, paint.A)})
		}
		return
	}
	den := float32(g.dx*g.dx) + float32(g.dy*g.dy)
	if !(den > 0) {
		v.doc.Draw(pdfRect(bounds), false, pdf.Style{Color: v.gradientColor(g, 1, paint.A)})
		return
	}
	// This basis maps a band coordinate t to the gradient direction.
	vx, vy := g.dx/den, g.dy/den
	basis := Matrix{sx: vx, ky: vy, kx: -vy, sy: vx, tx: g.x0, ty: g.y0}
	inv, ok := basis.invert()
	if !ok {
		return
	}
	box := inv.mapRect(bounds)
	v.doc.Transform(pdfMatrix(&basis))
	for i := 0; i < bands; i++ {
		a := box.L + float32(float32(box.width()*float32(i))/bands)
		b := box.L + float32(float32(box.width()*float32(i+1))/bands)
		v.doc.Draw(pdfRect(Rect{a, box.T, b, box.B}), false, pdf.Style{Color: v.gradientColor(g, float32((a+b)/2), paint.A)})
	}
}

// repeatPDFStops limits expansion to the visible gradient range.
func repeatPDFStops(stops []pdf.GradientStop, lo, hi float64, reflect bool) []pdf.GradientStop {
	if !finitePDF(lo) || !finitePDF(hi) || !(hi > lo) || hi-lo > 256 || len(stops) == 0 || float64(len(stops))*(hi-lo) > 4096 {
		return nil
	}
	base := make([]pdf.GradientStop, 0, len(stops)+2)
	if stops[0].Offset > 0 {
		base = append(base, pdf.GradientStop{Offset: 0, Color: stops[0].Color})
	}
	base = append(base, stops...)
	if base[len(base)-1].Offset < 1 {
		base = append(base, pdf.GradientStop{Offset: 1, Color: base[len(base)-1].Color})
	}
	var out []pdf.GradientStop
	add := func(s pdf.GradientStop) {
		if n := len(out); n > 0 && s.Offset <= out[n-1].Offset {
			if s.Color == out[n-1].Color {
				return
			}
			// A tiny interval preserves the jump between repeated periods.
			out[n-1].Offset = s.Offset - 1e-7
			if n > 1 && out[n-1].Offset <= out[n-2].Offset {
				out = out[:n-1]
			}
		}
		out = append(out, s)
	}
	for cycle := lo; cycle < hi; cycle++ {
		reversed := reflect && math.Mod(math.Abs(cycle), 2) == 1
		for i := range base {
			s := base[i]
			if reversed {
				s = base[len(base)-1-i]
				s.Offset = 1 - s.Offset
			}
			s.Offset = (cycle + s.Offset - lo) / (hi - lo)
			add(s)
		}
	}
	return out
}

func pdfRect(r Rect) string {
	return fmt.Sprintf("%s %s %s %s re\n", pdfNum(float64(r.L)), pdfNum(float64(r.T)), pdfNum(float64(r.width())), pdfNum(float64(r.height())))
}
