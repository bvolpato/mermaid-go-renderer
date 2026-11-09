// Paint servers: linear and radial gradients. The attributes follow
// blink/renderer/core/svg/svg_gradient_element.cc and
// core/layout/svg/layout_svg_resource_gradient.cc (Chromium 156). The
// colors are interpolated without premultiplication, as Blink asks Skia
// for SVG gradients.
//
// The two Blink files have the license LGPL-2.0-or-later in their headers
// at Chromium 156.0.8078.4. See NOTICE in this directory and finding F1 of
// docs/LICENSE-AUDIT.md.

package raster

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

type gradientStop struct {
	offset     float32
	r, g, b, a float32 // straight alpha, 0 to 1
}

// gradientShader is a linear or radial gradient in device space.
type gradientShader struct {
	// inv maps device pixel centers to the gradient space.
	inv    Matrix
	radial bool
	// Linear: the start point and the direction divided by its squared
	// length. Radial: the center and the radius.
	x0, y0, dx, dy float32
	r              float32
	spread         uint8 // 0 pad, 1 reflect, 2 repeat
	stops          []gradientStop
}

func (g *gradientShader) shadeSpan(x, y, n int, out []rgbaF) {
	fy := float32(y) + 0.5
	for i := 0; i < n; i++ {
		fx := float32(x+i) + 0.5
		lx := float32(g.inv.sx*fx) + float32(g.inv.kx*fy) + g.inv.tx
		ly := float32(g.inv.ky*fx) + float32(g.inv.sy*fy) + g.inv.ty
		var t float32
		if g.radial {
			ddx, ddy := lx-g.x0, ly-g.y0
			t = sqrt32(float32(ddx*ddx)+float32(ddy*ddy)) / g.r
		} else {
			t = float32((lx-g.x0)*g.dx) + float32((ly-g.y0)*g.dy)
		}
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
			if !(t > 0) {
				t = 0
			} else if t > 1 {
				t = 1
			}
		}
		out[i] = g.colorAt(t)
	}
}

func (g *gradientShader) colorAt(t float32) rgbaF {
	stops := g.stops
	var c gradientStop
	switch {
	case t <= stops[0].offset:
		c = stops[0]
	case t >= stops[len(stops)-1].offset:
		c = stops[len(stops)-1]
	default:
		for i := 1; i < len(stops); i++ {
			if t <= stops[i].offset {
				a, b := stops[i-1], stops[i]
				span := b.offset - a.offset
				if span <= 0 {
					c = b
					break
				}
				f := (t - a.offset) / span
				c = gradientStop{
					r: a.r + float32((b.r-a.r)*f), g: a.g + float32((b.g-a.g)*f),
					b: a.b + float32((b.b-a.b)*f), a: a.a + float32((b.a-a.a)*f),
				}
				break
			}
		}
	}
	return rgbaF{float32(c.r * c.a), float32(c.g * c.a), float32(c.b * c.a), c.a}
}

// gradientAttr looks up an attribute on a gradient element and on the
// elements that it references with href.
func (p *painter) gradientAttr(g *dom.Node, name string) (string, bool) {
	for depth := 0; g != nil && depth < 16; depth++ {
		if v, ok := g.GetAttr(name); ok {
			return v, true
		}
		g = p.gradientRef(g)
	}
	return "", false
}

func (p *painter) gradientRef(g *dom.Node) *dom.Node {
	href, ok := g.GetAttr("href")
	if !ok {
		href, ok = g.GetAttr("xlink:href")
	}
	if !ok || !strings.HasPrefix(href, "#") {
		return nil
	}
	t := p.ids[href[1:]]
	if t == nil || t == g || t.NS != dom.NSSVG {
		return nil
	}
	switch t.Tag {
	case "linearGradient", "lineargradient", "radialGradient", "radialgradient":
		return t
	}
	return nil
}

// gradientStops collects the stops of a gradient, or of the first
// referenced gradient that has stops.
func (p *painter) gradientStops(g *dom.Node) []gradientStop {
	for depth := 0; g != nil && depth < 16; depth++ {
		var stops []gradientStop
		last := float32(0)
		for c := g.FirstChild; c != nil; c = c.Next {
			if c.Type != dom.ElementNode || c.NS != dom.NSSVG || c.Tag != "stop" {
				continue
			}
			off := float32(0)
			if v, ok := c.GetAttr("offset"); ok {
				v = strings.TrimSpace(v)
				if strings.HasSuffix(v, "%") {
					if f, ok := parseFloat(v[:len(v)-1]); ok {
						off = float32(f / 100)
					}
				} else if f, ok := parseFloat(v); ok {
					off = float32(f)
				}
			}
			off = min(max(off, 0), 1)
			off = max(off, last)
			last = off
			col := css.Color{A: 1}
			if v, ok := p.eng.RasterDeclared(c, "stop-color"); ok {
				if strings.EqualFold(strings.TrimSpace(v), "currentcolor") {
					col = p.htmlColor(c)
				} else if pc, ok := parseColor(v); ok {
					col = pc
				}
			}
			opacity := float32(1)
			if v, ok := p.eng.RasterDeclared(c, "stop-opacity"); ok {
				if f, ok := parseOpacity(v); ok {
					opacity = f
				}
			}
			stops = append(stops, gradientStop{
				offset: off,
				r:      float32(col.R) / 255, g: float32(col.G) / 255, b: float32(col.B) / 255,
				a: float32(float32(col.A) * opacity),
			})
		}
		if len(stops) > 0 {
			return stops
		}
		g = p.gradientRef(g)
	}
	return nil
}

// paintServer resolves url(#id) to a shader for an element. It returns nil
// when the reference is not a usable paint server.
func (p *painter) paintServer(n *dom.Node, id string, ctm Affine) shader {
	g := p.ids[id]
	if g == nil || g.NS != dom.NSSVG {
		return nil
	}
	radial := false
	switch g.Tag {
	case "linearGradient", "lineargradient":
	case "radialGradient", "radialgradient":
		radial = true
	default:
		return nil
	}
	stops := p.gradientStops(g)
	if len(stops) == 0 {
		return nil
	}
	units, _ := p.gradientAttr(g, "gradientUnits")
	bbox := strings.TrimSpace(units) != "userSpaceOnUse"
	m := ctm
	if bbox {
		x, y, w, h, ok := p.eng.RasterBBox(n)
		if !ok || !(w > 0 && h > 0) {
			return nil
		}
		m = m.Translate(float64(x), float64(y)).Scale(float64(w), float64(h))
	}
	if v, ok := p.gradientAttr(g, "gradientTransform"); ok {
		t := browser.RasterParseTransform(v, false)
		m = m.Mul(Affine{A: t[0], B: t[1], C: t[2], D: t[3], E: t[4], F: t[5]})
	}
	inv, ok := m.Invert()
	if !ok {
		return nil
	}
	sh := &gradientShader{inv: inv.toMatrix(), radial: radial, stops: stops}
	if v, ok := p.gradientAttr(g, "spreadMethod"); ok {
		switch strings.TrimSpace(v) {
		case "reflect":
			sh.spread = 1
		case "repeat":
			sh.spread = 2
		}
	}
	// coord resolves a gradient coordinate: a fraction of the bounding
	// box, or a length in user space.
	coord := func(name, def string, mode lengthMode) float32 {
		v, ok := p.gradientAttr(g, name)
		if !ok {
			v = def
		}
		v = strings.TrimSpace(v)
		if bbox {
			if strings.HasSuffix(v, "%") {
				f, _ := parseFloat(v[:len(v)-1])
				return float32(f / 100)
			}
			f, _ := parseFloat(v)
			return float32(f)
		}
		f, _ := p.length(g, v, mode)
		return f
	}
	if radial {
		sh.x0 = coord("cx", "50%", lengthWidth)
		sh.y0 = coord("cy", "50%", lengthHeight)
		sh.r = coord("r", "50%", lengthOther)
		if !(sh.r > 0) {
			// A zero radius paints the last stop.
			last := stops[len(stops)-1]
			sh.stops = []gradientStop{last}
			sh.r = 1
		}
		return sh
	}
	x1 := coord("x1", "0%", lengthWidth)
	y1 := coord("y1", "0%", lengthHeight)
	x2 := coord("x2", "100%", lengthWidth)
	y2 := coord("y2", "0%", lengthHeight)
	dx, dy := x2-x1, y2-y1
	l2 := float32(dx*dx) + float32(dy*dy)
	if !(l2 > 0) {
		last := stops[len(stops)-1]
		sh.stops = []gradientStop{last}
		sh.dx, sh.dy = 0, 0
		return sh
	}
	sh.x0, sh.y0 = x1, y1
	sh.dx, sh.dy = dx/l2, dy/l2
	return sh
}
