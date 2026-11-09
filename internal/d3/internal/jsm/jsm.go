// Package jsm is the entry point for the transcendental Math functions that
// the d3 port calls. Each function goes to internal/js, as the project
// requires.
//
// The reference of the project is Chromium, and Chromium returns the
// correctly rounded value for Math.sin and Math.cos: the reference renders
// show it (internal/js/testdata/d3_chromium_trig.json). So Sin, Cos, Tan,
// Asin, Acos, Atan and Atan2 are the correctly rounded functions of
// internal/js. For the other functions internal/js has only the wrappers of
// Go's math package, which do not always agree with a browser in the last
// bit.
//
// The functions are variables for one reason. The golden files of the d3
// port come from Node.js, whose Math functions are not correctly rounded and
// are not those of Go either. The golden tests replace these variables with
// the results that Node.js returned. A golden case that fails with the
// functions below and passes with the recorded results differs only because
// of a Math function. Code outside of tests must not assign to these
// variables.
package jsm

import (
	"math"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

var (
	Sin   = js.SinCR
	Cos   = js.CosCR
	Tan   = js.TanCR
	Asin  = js.AsinCR
	Atan  = js.Atan
	Acos  = js.AcosCR
	Atan2 = js.Atan2
	Exp   = js.Exp
	Log   = js.Log
	Log2  = js.Log2
	Cbrt  = js.Cbrt
	Log10 = log10
	Pow   = pow
)

// The tick computation of d3 depends on powers of ten. Node.js returns the
// correctly rounded value for Math.pow(10, n) with an integer n, and it
// returns exactly n for Math.log10 of a power of ten from 1e-311 up. Go's
// math.Pow(10, n) is off by some units in the last place for n < -22 and
// n > 32, and math.Log10 is not exact for powers such as 0.1 and 1e-4. The
// table below gives the exact results for these arguments.
const (
	pow10Min = -330
	pow10Max = 310
	// Node.js is not exact below this exponent, because the powers are
	// subnormal.
	log10Min = -311
)

var (
	pow10tab [pow10Max - pow10Min + 1]float64
	log10tab = map[uint64]float64{}
)

func init() {
	for n := pow10Min; n <= pow10Max; n++ {
		v, _ := strconv.ParseFloat("1e"+strconv.Itoa(n), 64)
		pow10tab[n-pow10Min] = v
		if n >= log10Min && n <= 308 {
			log10tab[math.Float64bits(v)] = float64(n)
		}
	}
}

// pow is Math.pow. ECMAScript returns NaN for a base of 1 or -1 with an
// exponent that is NaN or infinite. Go's math.Pow returns 1 in these cases.
func pow(x, y float64) float64 {
	if math.IsNaN(y) || (math.IsInf(y, 0) && math.Abs(x) == 1) {
		return math.NaN()
	}
	if x == 10 && y >= pow10Min && y <= pow10Max && y == math.Trunc(y) {
		return pow10tab[int(y)-pow10Min]
	}
	return js.Pow(x, y)
}

// log10 is Math.log10.
func log10(x float64) float64 {
	if n, ok := log10tab[math.Float64bits(x)]; ok {
		return n
	}
	return js.Log10(x)
}

// Pow10 is 10 ** n for an integer n from -330 to 310, correctly rounded.
func Pow10(n int) float64 {
	return pow10tab[n-pow10Min]
}
