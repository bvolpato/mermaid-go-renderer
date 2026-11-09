// Ported from khroma/dist/channels/index.js and channels/type.js (khroma 2.1.0).

package khroma

import "math"

// channels is the Channels class. It holds a color as RGB or HSL data and
// converts lazily between the two. A channel that the JavaScript object does
// not define has its "has" flag cleared.
//
// khroma reuses one global instance for every parse. This port allocates one
// per parse, so that concurrent callers are safe. No khroma function depends
// on the sharing.
type channels struct {
	color    string // the source string, or "" when there is none
	hasColor bool
	changed  bool
	typ      int

	r, g, b, h, s, l, a float64
	hasR, hasG, hasB    bool
	hasH, hasS, hasL    bool
}

func newRGB(r, g, b, a float64) *channels {
	return &channels{r: r, g: g, b: b, a: a, hasR: true, hasG: true, hasB: true}
}

func newHSL(h, s, l, a float64) *channels {
	return &channels{h: h, s: s, l: l, a: a, hasH: true, hasS: true, hasL: true}
}

func (c *channels) withColor(color string) *channels {
	c.color = color
	c.hasColor = true
	return c
}

// undef returns the value of a data field, or NaN for an undefined field.
func undef(v float64, has bool) float64 {
	if has {
		return v
	}
	return math.NaN()
}

func (c *channels) ensureHSL() {
	r, g, b := undef(c.r, c.hasR), undef(c.g, c.hasG), undef(c.b, c.hasB)
	if !c.hasH {
		c.h, c.hasH = rgb2hsl(r, g, b, 'h'), true
	}
	if !c.hasS {
		c.s, c.hasS = rgb2hsl(r, g, b, 's'), true
	}
	if !c.hasL {
		c.l, c.hasL = rgb2hsl(r, g, b, 'l'), true
	}
}

func (c *channels) ensureRGB() {
	h, s, l := undef(c.h, c.hasH), undef(c.s, c.hasS), undef(c.l, c.hasL)
	if !c.hasR {
		c.r, c.hasR = hsl2rgb(h, s, l, 'r'), true
	}
	if !c.hasG {
		c.g, c.hasG = hsl2rgb(h, s, l, 'g'), true
	}
	if !c.hasB {
		c.b, c.hasB = hsl2rgb(h, s, l, 'b'), true
	}
}

func (c *channels) getR() float64 {
	if c.typ != typeHSL && c.hasR {
		return c.r
	}
	c.ensureHSL()
	return hsl2rgb(c.h, c.s, c.l, 'r')
}

func (c *channels) getG() float64 {
	if c.typ != typeHSL && c.hasG {
		return c.g
	}
	c.ensureHSL()
	return hsl2rgb(c.h, c.s, c.l, 'g')
}

func (c *channels) getB() float64 {
	if c.typ != typeHSL && c.hasB {
		return c.b
	}
	c.ensureHSL()
	return hsl2rgb(c.h, c.s, c.l, 'b')
}

func (c *channels) getH() float64 {
	if c.typ != typeRGB && c.hasH {
		return c.h
	}
	c.ensureRGB()
	return rgb2hsl(c.r, c.g, c.b, 'h')
}

func (c *channels) getS() float64 {
	if c.typ != typeRGB && c.hasS {
		return c.s
	}
	c.ensureRGB()
	return rgb2hsl(c.r, c.g, c.b, 's')
}

func (c *channels) getL() float64 {
	if c.typ != typeRGB && c.hasL {
		return c.l
	}
	c.ensureRGB()
	return rgb2hsl(c.r, c.g, c.b, 'l')
}

// get is channels[channel].
func (c *channels) get(channel byte) float64 {
	switch channel {
	case 'r':
		return c.getR()
	case 'g':
		return c.getG()
	case 'b':
		return c.getB()
	case 'h':
		return c.getH()
	case 's':
		return c.getS()
	case 'l':
		return c.getL()
	}
	return c.a
}

// setType is Type.set.
func (c *channels) setType(typ int) error {
	if c.typ != typeAll && c.typ != typ {
		return &Error{Message: "Cannot change both RGB and HSL channels at the same time"}
	}
	c.typ = typ
	return nil
}

// set is channels[channel] = v.
func (c *channels) set(channel byte, v float64) error {
	switch channel {
	case 'r', 'g', 'b':
		if err := c.setType(typeRGB); err != nil {
			return err
		}
	case 'h', 's', 'l':
		if err := c.setType(typeHSL); err != nil {
			return err
		}
	}
	c.changed = true
	switch channel {
	case 'r':
		c.r, c.hasR = v, true
	case 'g':
		c.g, c.hasG = v, true
	case 'b':
		c.b, c.hasB = v, true
	case 'h':
		c.h, c.hasH = v, true
	case 's':
		c.s, c.hasS = v, true
	case 'l':
		c.l, c.hasL = v, true
	default:
		c.a = v
	}
	return nil
}
