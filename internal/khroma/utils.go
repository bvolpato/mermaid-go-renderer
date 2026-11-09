// Ported from khroma/dist/utils/channel.js, utils/lang.js, utils/unit.js and
// constants.js (khroma 2.1.0).

// Package khroma is a port of the khroma color library, version 2.1.0.
//
// Mermaid computes its theme variables and many CSS values with khroma. The
// output strings of this package are byte for byte the strings that khroma
// returns in V8.
//
// Floating-point expressions keep the shape of the JavaScript source. Every
// product is wrapped in float64(...). Without the conversion the Go compiler
// may fuse a multiplication with a later addition or subtraction into one FMA
// instruction (on arm64, for example), which rounds differently from V8. The
// fusion also happens across inlined calls: js.Round(a*255) returns 76 for
// a = 0.3 on arm64, and js.Round(float64(a*255)) returns 77, as V8 does.
package khroma

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Channel type flags (constants.js TYPE).
const (
	typeAll = 0
	typeRGB = 1
	typeHSL = 2
)

// dec2hex is Unit.dec2hex.
func dec2hex(dec float64) string {
	const digits = "0123456789abcdef"
	n := int(js.Round(dec))
	if n < 16 {
		return "0" + string(digits[n])
	}
	return string(digits[n>>4]) + string(digits[n&15])
}

// dec2hexTable is the DEC2HEX lookup. A key that is not an integer from 0 to
// 255 is a missing property, and prints as "undefined" in a template string.
func dec2hexTable(x float64) string {
	if x >= 0 && x <= 255 && x == math.Trunc(x) {
		return dec2hex(x)
	}
	return "undefined"
}

// channelMax is Channel.max.
func channelMax(c byte) float64 {
	switch c {
	case 'r', 'g', 'b':
		return 255
	case 'h':
		return 360
	case 's', 'l':
		return 100
	case 'a':
		return 1
	}
	return math.NaN()
}

func clamp255(v float64) float64 {
	if v >= 255 {
		return 255
	}
	if v < 0 {
		return 0
	}
	return v
}

func clamp100(v float64) float64 {
	if v >= 100 {
		return 100
	}
	if v < 0 {
		return 0
	}
	return v
}

func clampA(a float64) float64 {
	if a >= 1 {
		return 1
	}
	if a < 0 {
		return 0
	}
	return a
}

// clampH is Channel.clamp.h. The JavaScript % operator is fmod.
func clampH(h float64) float64 { return math.Mod(h, 360) }

// clampChannel is Channel.clamp[c](v).
func clampChannel(c byte, v float64) float64 {
	switch c {
	case 'r', 'g', 'b':
		return clamp255(v)
	case 'h':
		return clampH(v)
	case 's', 'l':
		return clamp100(v)
	}
	return clampA(v)
}

// toLinear is Channel.toLinear.
func toLinear(c float64) float64 {
	n := c / 255
	if c > .03928 {
		return js.Pow((n+.055)/1.055, 2.4)
	}
	return n / 12.92
}

// hue2rgb is Channel.hue2rgb.
func hue2rgb(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	if t < 1.0/6 {
		return p + float64(float64((q-p)*6)*t)
	}
	if t < 1.0/2 {
		return q
	}
	if t < 2.0/3 {
		return p + float64(float64((q-p)*(2.0/3-t))*6)
	}
	return p
}

// hsl2rgb is Channel.hsl2rgb. An undefined channel is passed as NaN.
func hsl2rgb(h, s, l float64, channel byte) float64 {
	if s == 0 || math.IsNaN(s) {
		return float64(l * 2.55) // Achromatic
	}
	h /= 360
	s /= 100
	l /= 100
	var q float64
	if l < .5 {
		q = float64(l * (1 + s))
	} else {
		q = (l + s) - float64(l*s)
	}
	p := float64(2*l) - q
	switch channel {
	case 'r':
		return float64(hue2rgb(p, q, h+1.0/3) * 255)
	case 'g':
		return float64(hue2rgb(p, q, h) * 255)
	}
	return float64(hue2rgb(p, q, h-1.0/3) * 255)
}

// rgb2hsl is Channel.rgb2hsl. An undefined channel is passed as NaN.
func rgb2hsl(r, g, b float64, channel byte) float64 {
	r /= 255
	g /= 255
	b /= 255
	max := js.Max(r, g, b)
	min := js.Min(r, g, b)
	l := (max + min) / 2
	if channel == 'l' {
		return float64(l * 100)
	}
	if max == min {
		return 0 // Achromatic
	}
	d := max - min
	var s float64
	if l > .5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	if channel == 's' {
		return float64(s * 100)
	}
	switch max {
	case r:
		k := 0.0
		if g < b {
			k = 6
		}
		return float64(((g-b)/d + k) * 60)
	case g:
		return float64(((b-r)/d + 2) * 60)
	case b:
		return float64(((r-g)/d + 4) * 60)
	}
	return -1
}

// langClamp is Lang.clamp.
func langClamp(number, lower, upper float64) float64 {
	if lower > upper {
		return js.Min(lower, js.Max(upper, number))
	}
	return js.Min(upper, js.Max(lower, number))
}

// langRound is Lang.round: it keeps ten decimal digits.
func langRound(number float64) float64 {
	return js.Round(float64(number*10000000000)) / 10000000000
}
