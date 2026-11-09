// Ported from khroma/dist/methods/*.js (khroma 2.1.0).

package khroma

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// A color argument is a dynamic JavaScript value (see internal/js/value.go),
// because mermaid passes theme variables that a user can set to anything. It
// is a string in every normal case.
//
// khroma returns a value that is not a string unchanged from Color.parse, and
// then reads channel properties from it. This port follows that for reads: a
// missing channel is undefined, which is NaN in arithmetic. A function that
// writes a channel returns a TypeError for such a value. khroma does the same
// for undefined, null, numbers and booleans. For an object or an array khroma
// first adds properties to the caller's value; this port does not.

// ChannelValue is one entry of the channels object that adjust, change and
// scale take, for example { h: 180, l: 5 }. The order of the entries is the
// property order of the JavaScript object.
type ChannelValue struct {
	Channel byte // one of 'r', 'g', 'b', 'h', 's', 'l', 'a'
	Value   float64
}

func R(v float64) ChannelValue { return ChannelValue{'r', v} }
func G(v float64) ChannelValue { return ChannelValue{'g', v} }
func B(v float64) ChannelValue { return ChannelValue{'b', v} }
func H(v float64) ChannelValue { return ChannelValue{'h', v} }
func S(v float64) ChannelValue { return ChannelValue{'s', v} }
func L(v float64) ChannelValue { return ChannelValue{'l', v} }
func A(v float64) ChannelValue { return ChannelValue{'a', v} }

// parse is Color.parse for a dynamic value.
func parse(color any) (*channels, bool, error) {
	switch v := color.(type) {
	case string:
		ch, err := parseColor(v)
		return ch, false, err
	case *channels:
		return v, false, nil
	case nil:
		return nil, true, &Error{Message: "Cannot read properties of undefined", TypeError: true}
	case *js.Obj[any]:
		if v == nil {
			return nil, true, &Error{Message: "Cannot read properties of undefined", TypeError: true}
		}
		prop := func(k string) float64 { return js.ToNumber(v.Get(k)) }
		ch := &channels{
			r: prop("r"), g: prop("g"), b: prop("b"),
			h: prop("h"), s: prop("s"), l: prop("l"), a: prop("a"),
			hasR: true, hasG: true, hasB: true, hasH: true, hasS: true, hasL: true,
		}
		return ch, true, nil
	}
	if js.IsNullish(color) {
		return nil, true, &Error{Message: "Cannot read properties of null", TypeError: true}
	}
	nan := math.NaN()
	ch := &channels{
		r: nan, g: nan, b: nan, h: nan, s: nan, l: nan, a: nan,
		hasR: true, hasG: true, hasB: true, hasH: true, hasS: true, hasL: true,
	}
	return ch, true, nil
}

// parseWritable is Color.parse for a function that sets a channel.
func parseWritable(color any) (*channels, error) {
	ch, foreign, err := parse(color)
	if err != nil {
		return nil, err
	}
	if foreign {
		return nil, &Error{Message: "Cannot create property on a value that is not a color string", TypeError: true}
	}
	return ch, nil
}

// AdjustChannel is adjustChannel(color, channel, amount).
func AdjustChannel(color any, channel byte, amount float64) (string, error) {
	ch, err := parseWritable(color)
	if err != nil {
		return "", err
	}
	amountCurrent := ch.get(channel)
	amountNext := clampChannel(channel, amountCurrent+amount)
	if amountCurrent != amountNext { // also true for NaN, as in JavaScript
		if err := ch.set(channel, amountNext); err != nil {
			return "", err
		}
	}
	return stringifyColor(ch), nil
}

// Change is change(color, channels): it sets channels to absolute values.
func Change(color any, values ...ChannelValue) (string, error) {
	ch, err := parseWritable(color)
	if err != nil {
		return "", err
	}
	for _, c := range values {
		if err := ch.set(c.Channel, clampChannel(c.Channel, c.Value)); err != nil {
			return "", err
		}
	}
	return stringifyColor(ch), nil
}

// Adjust is adjust(color, channels): it adds amounts to channels.
func Adjust(color any, amounts ...ChannelValue) (string, error) {
	ch, err := parseWritable(color)
	if err != nil {
		return "", err
	}
	changes := make([]ChannelValue, 0, len(amounts))
	for _, c := range amounts {
		if c.Value == 0 || math.IsNaN(c.Value) {
			continue
		}
		changes = append(changes, ChannelValue{c.Channel, ch.get(c.Channel) + c.Value})
	}
	return Change(color, changes...)
}

// Scale is scale(color, channels): it moves channels by a percentage of the
// distance to their limit.
func Scale(color any, weights ...ChannelValue) (string, error) {
	ch, err := parseWritable(color)
	if err != nil {
		return "", err
	}
	delta := func(amount, weight, max float64) float64 {
		if weight > 0 {
			return float64((max-amount)*weight) / 100
		}
		return float64(amount*weight) / 100
	}
	adjustments := make([]ChannelValue, 0, len(weights))
	for _, c := range weights {
		adjustments = append(adjustments, ChannelValue{c.Channel, delta(ch.get(c.Channel), c.Value, channelMax(c.Channel))})
	}
	return Adjust(color, adjustments...)
}

// Lighten is lighten(color, amount). Pass NaN for an omitted amount.
func Lighten(color any, amount float64) (string, error) {
	return AdjustChannel(color, 'l', amount)
}

// Darken is darken(color, amount). Pass NaN for an omitted amount.
func Darken(color any, amount float64) (string, error) {
	return AdjustChannel(color, 'l', -amount)
}

// Saturate is saturate(color, amount).
func Saturate(color any, amount float64) (string, error) {
	return AdjustChannel(color, 's', amount)
}

// Desaturate is desaturate(color, amount).
func Desaturate(color any, amount float64) (string, error) {
	return AdjustChannel(color, 's', -amount)
}

// Opacify is opacify(color, amount), also named fadeIn.
func Opacify(color any, amount float64) (string, error) {
	return AdjustChannel(color, 'a', amount)
}

// Transparentize is transparentize(color, amount), also named fadeOut.
func Transparentize(color any, amount float64) (string, error) {
	return AdjustChannel(color, 'a', -amount)
}

// Complement is complement(color).
func Complement(color any) (string, error) {
	return AdjustChannel(color, 'h', 180)
}

// Grayscale is grayscale(color).
func Grayscale(color any) (string, error) {
	return Change(color, S(0))
}

// RGBA is rgba(r, g, b, a) with numbers. khroma also exports it as rgb and hex.
func RGBA(r, g, b, a float64) string {
	return stringifyColor(newRGB(clamp255(r), clamp255(g), clamp255(b), clampA(a)))
}

// WithAlpha is rgba(color, a) with a color: it sets the alpha channel.
func WithAlpha(color any, a float64) (string, error) {
	return Change(color, A(a))
}

// HSLA is hsla(h, s, l, a). khroma also exports it as hsl, where a is 1.
func HSLA(h, s, l, a float64) string {
	return stringifyColor(newHSL(clampH(h), clamp100(s), clamp100(l), clampA(a)))
}

// Mix is mix(color1, color2, weight). The weight is 50 when it is omitted.
func Mix(color1, color2 any, weight ...float64) (string, error) {
	w := 50.0
	if len(weight) > 0 {
		w = weight[0]
	}
	c1, _, err := parse(color1)
	if err != nil {
		return "", err
	}
	r1, g1, b1, a1 := c1.getR(), c1.getG(), c1.getB(), c1.a
	c2, _, err := parse(color2)
	if err != nil {
		return "", err
	}
	r2, g2, b2, a2 := c2.getR(), c2.getG(), c2.getB(), c2.a
	weightScale := w / 100
	weightNormalized := float64(weightScale*2) - 1
	alphaDelta := a1 - a2
	var weight1combined float64
	if float64(weightNormalized*alphaDelta) == -1 {
		weight1combined = weightNormalized
	} else {
		weight1combined = (weightNormalized + alphaDelta) / (1 + float64(weightNormalized*alphaDelta))
	}
	// The conversion keeps the compiler from fusing the division, which it
	// turns into a product, with the subtraction of the next line.
	weight1 := float64((weight1combined + 1) / 2)
	weight2 := 1 - weight1
	r := float64(r1*weight1) + float64(r2*weight2)
	g := float64(g1*weight1) + float64(g2*weight2)
	b := float64(b1*weight1) + float64(b2*weight2)
	a := float64(a1*weightScale) + float64(a2*(1-weightScale))
	return RGBA(r, g, b, a), nil
}

// Invert is invert(color, weight). The weight is 100 when it is omitted.
func Invert(color any, weight ...float64) (string, error) {
	w := 100.0
	if len(weight) > 0 {
		w = weight[0]
	}
	inverse, err := parseWritable(color)
	if err != nil {
		return "", err
	}
	if err := inverse.set('r', 255-inverse.getR()); err != nil {
		return "", err
	}
	if err := inverse.set('g', 255-inverse.getG()); err != nil {
		return "", err
	}
	if err := inverse.set('b', 255-inverse.getB()); err != nil {
		return "", err
	}
	return Mix(inverse, color, w)
}

// Channel is channel(color, channel): the value of one channel, rounded to
// ten decimal digits.
func Channel(color any, channel byte) (float64, error) {
	ch, _, err := parse(color)
	if err != nil {
		return 0, err
	}
	return langRound(ch.get(channel)), nil
}

func Red(color any) (float64, error)        { return Channel(color, 'r') }
func Green(color any) (float64, error)      { return Channel(color, 'g') }
func Blue(color any) (float64, error)       { return Channel(color, 'b') }
func Hue(color any) (float64, error)        { return Channel(color, 'h') }
func Saturation(color any) (float64, error) { return Channel(color, 's') }
func Lightness(color any) (float64, error)  { return Channel(color, 'l') }

// Alpha is alpha(color), also named opacity.
func Alpha(color any) (float64, error) { return Channel(color, 'a') }

// Luminance is luminance(color).
func Luminance(color any) (float64, error) {
	ch, _, err := parse(color)
	if err != nil {
		return 0, err
	}
	r, g, b := ch.getR(), ch.getG(), ch.getB()
	luminance := float64(.2126*toLinear(r)) + float64(.7152*toLinear(g)) + float64(.0722*toLinear(b))
	return langRound(luminance), nil
}

// Contrast is contrast(color1, color2).
func Contrast(color1, color2 any) (float64, error) {
	luminance1, err := Luminance(color1)
	if err != nil {
		return 0, err
	}
	luminance2, err := Luminance(color2)
	if err != nil {
		return 0, err
	}
	const epsilon = 2.220446049250313e-16 // Number.EPSILON
	max := js.Max(luminance1, luminance2)
	min := js.Min(luminance1, luminance2)
	ratio := (max + epsilon) / (min + epsilon)
	return langRound(langClamp(ratio, 1, 10)), nil
}

// IsLight is isLight(color).
func IsLight(color any) (bool, error) {
	l, err := Luminance(color)
	return l >= .5, err
}

// IsDark is isDark(color).
func IsDark(color any) (bool, error) {
	light, err := IsLight(color)
	return !light, err
}

// IsTransparent is isTransparent(color).
func IsTransparent(color any) (bool, error) {
	a, err := Alpha(color)
	return a == 0 || math.IsNaN(a), err
}

// IsValid is isValid(color).
func IsValid(color any) bool {
	s, ok := color.(string)
	if !ok {
		return true // Color.parse returns such a value as is
	}
	_, err := parseColor(s)
	return err == nil
}

// ToHex is toHex(color).
func ToHex(color any) (string, error) {
	ch, _, err := parse(color)
	if err != nil {
		return "", err
	}
	return stringifyHex(ch), nil
}

// ToRgba is toRgba(color).
func ToRgba(color any) (string, error) {
	ch, _, err := parse(color)
	if err != nil {
		return "", err
	}
	return stringifyRGB(ch), nil
}

// ToHsla is toHsla(color).
func ToHsla(color any) (string, error) {
	ch, _, err := parse(color)
	if err != nil {
		return "", err
	}
	return stringifyHSL(ch), nil
}

// ToKeyword is toKeyword(color). The second result is false when no CSS
// keyword has this color; khroma returns undefined then.
func ToKeyword(color any) (string, bool, error) {
	ch, _, err := parse(color)
	if err != nil {
		return "", false, err
	}
	name, ok := stringifyKeyword(ch)
	return name, ok, nil
}
