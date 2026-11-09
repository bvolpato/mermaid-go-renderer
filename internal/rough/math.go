// Ported from roughjs 4.6.6, roughjs/bin/math.js (MIT).

package rough

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// mathFuncs are the transcendental functions that the port calls. A
// generator uses the wrappers of package js. The tests replace them, so that
// the port and the JavaScript code get the same value for each call. A test
// can then separate a difference in the port from a difference between two
// math libraries.
type mathFuncs struct {
	sin, cos, tan, atan, asin func(float64) float64
	pow                       func(x, y float64) float64
}

func jsMathFuncs() mathFuncs {
	return mathFuncs{sin: js.Sin, cos: js.Cos, tan: js.Tan, atan: js.Atan, asin: js.Asin, pow: js.Pow}
}

// random is the Random class of roughjs. The seed is a dynamic value in
// JavaScript. seed holds its numeric value and truthy holds its boolean value.
type random struct {
	seed   float64
	truthy bool
	gen    *Generator
}

// newRandom is new Random(seed).
func newRandom(seed any, gen *Generator) *random {
	return &random{seed: toNumber(seed), truthy: truthy(seed), gen: gen}
}

// next returns the next number in [0, 1). With a seed of 0 it calls
// Math.random.
func (r *random) next() float64 {
	if r.truthy {
		// ((2 ** 31 - 1) & (this.seed = Math.imul(48271, this.seed))) / 2 ** 31
		s := int32(uint32(48271) * js.ToUint32(r.seed))
		r.seed = float64(s)
		r.truthy = s != 0
		return float64(0x7fffffff&s) / 2147483648
	}
	return r.gen.mathRandom()
}

// mathRound is Math.round. It returns -0 for an argument in [-0.5, -0], as
// JavaScript does. js.Round returns +0 for an argument in [-0.5, 0).
func mathRound(x float64) float64 {
	r := js.Round(x)
	if r == 0 && (x < 0 || (x == 0 && math.Signbit(x))) {
		return math.Copysign(0, -1)
	}
	return r
}
