package crmath

// This file has the arguments of the Chromium golden test of the Math
// functions (golden_test.go). The arguments come from a seeded generator, so
// the golden file stores only the results of the browser.
//
// tools/reference/gen-jsmath.mjs runs TestMathWriteInputs to get the
// arguments, evaluates them in Chromium and writes the golden file.
//
// The generator uses integer operations and exact float64 operations only.
// It gives the same arguments on every platform. A change of the generator
// changes the arguments: generate the golden file again after a change.

import (
	"encoding/binary"
	"math"
	"os"
	"strconv"
	"testing"
)

// mathGen is a splitmix64 generator.
type mathGen struct{ s uint64 }

func (g *mathGen) u64() uint64 {
	g.s += 0x9e3779b97f4a7c15
	z := g.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (g *mathGen) intn(n int) int { return int(g.u64() % uint64(n)) }

// unit returns a number in [0, 1).
func (g *mathGen) unit() float64 { return float64(float64(g.u64()>>11) * 0x1p-53) }

func (g *mathGen) sign(x float64) float64 {
	if g.u64()&1 != 0 {
		return -x
	}
	return x
}

// raw returns a float64 with random bits. It can be NaN or infinite.
func (g *mathGen) raw() float64 { return math.Float64frombits(g.u64()) }

// exp returns a positive number with a random mantissa and a binary exponent
// from lo to hi. The limits are -1022 and 1023.
func (g *mathGen) exp(lo, hi int) float64 {
	e := uint64(lo + g.intn(hi-lo+1) + 1023)
	return math.Float64frombits(e<<52 | g.u64()>>12)
}

// subnormal returns a positive subnormal number, or zero.
func (g *mathGen) subnormal() float64 {
	return math.Float64frombits(g.u64() >> (12 + uint(g.intn(52))))
}

// uniform returns a number from lo to hi.
func (g *mathGen) uniform(lo, hi float64) float64 {
	return lo + float64(g.unit()*(hi-lo))
}

// near returns a number that is at most span float64 steps away from c.
func (g *mathGen) near(c float64, span int) float64 {
	return math.Float64frombits(uint64(int64(math.Float64bits(c)) + int64(g.intn(2*span+1)-span)))
}

// short returns a positive number with a binary exponent from lo to hi and a
// mantissa of at most bits random bits.
func (g *mathGen) short(lo, hi, bits int) float64 {
	e := uint64(lo + g.intn(hi-lo+1) + 1023)
	m := (g.u64() >> 12) &^ (1<<(52-uint(bits)) - 1)
	return math.Float64frombits(e<<52 | m)
}

var mathGenPow10 = [...]float64{1, 10, 100, 1000, 10000, 100000}

// decimal returns a number with at most 5 decimal places, such as -37.25.
func (g *mathGen) decimal() float64 {
	k := g.intn(2000001) - 1000000
	return float64(k) / mathGenPow10[g.intn(len(mathGenPow10))]
}

// integer returns an integer from -max to max.
func (g *mathGen) integer(max int) float64 { return float64(g.intn(2*max+1) - max) }

// any returns a number of a random class.
func (g *mathGen) any() float64 {
	switch g.intn(8) {
	case 0:
		return g.raw()
	case 1:
		return g.sign(g.exp(-1022, 1023))
	case 2:
		return g.sign(g.exp(-40, 40))
	case 3:
		return g.decimal()
	case 4:
		return g.integer(1 << uint(g.intn(30)+1))
	case 5:
		return g.sign(g.subnormal())
	case 6:
		return g.sign(g.short(-64, 64, 1+g.intn(8)))
	}
	return mathEdgeValues[g.intn(len(mathEdgeValues))]
}

// mathEdgeValues are arguments that every function gets.
var mathEdgeValues = func() []float64 {
	inf := math.Inf(1)
	v := []float64{
		0, math.Copysign(0, -1), 5e-324, -5e-324, 1e-323,
		2.225073858507201e-308, -2.225073858507201e-308, // the largest subnormal number
		2.2250738585072014e-308, -2.2250738585072014e-308,
		1, -1, 2, -2, 0.5, -0.5, 3, 10, 0.1, 0.2, 0.3, 1.5, 2.5, 100, 1000, 255, 360, 180, 90, 45, 30, 60,
		math.MaxFloat64, -math.MaxFloat64, inf, -inf, math.NaN(),
		1e22, -1e22, 1e300, -1e300, 1e-300, 1e-22, 1e21, 1e-7, 1e-5,
		math.Pi, math.Pi / 2, math.Pi / 4, math.Pi / 3, math.Pi / 6, 2 * math.Pi, 3 * math.Pi / 2,
		-math.Pi, -math.Pi / 2, math.E, math.Ln2, math.Ln10, math.Log2E, math.Log10E, math.Sqrt2, math.SqrtE,
		0x1p53, 0x1p53 - 1, 0x1p53 + 2, -0x1p53, 0x1p52, 0x1p52 + 1, 0x1p52 + 0.5, 0x1p51 + 0.5,
		0x1p31, 0x1p31 - 1, 0x1p32, 0x1p32 - 1, 0x1p32 + 1, -0x1p31, -0x1p31 - 1, 0x1p63, 0x1p64, -0x1p63,
		0.49999999999999994, 0.5000000000000001, -0.49999999999999994, 1.0000000000000002, 0.9999999999999999,
		-1.0000000000000002, -0.9999999999999999, 4503599627370495.5, 4503599627370497, 9007199254740993,
		709.782712893384, 709.7827128933841, 710.4758600739439, 710.475860073944, -745.1332191019411,
		-745.1332191019412, -708.3964185322641, 88.72283905206835, -103.97207708399179, 0.6931471805599453,
		22, 22.000000000000004, 3.725290298461914e-09, 0.34657359027997264, 1.1102230246251565e-16,
		6381956970095103 * 0x1p797, 0x1.921fb54442d18p+1, 0x1.921fb54442d19p+1, 0x1.921fb54442d17p+1,
	}
	for k := 1; k <= 256; k++ {
		v = append(v, float64(k)*math.Pi/2, -float64(k)*math.Pi/128, float64(k)*math.Ln2, float64(k)/64)
	}
	for e := -1074; e <= 1023; e++ {
		v = append(v, math.Ldexp(1, e))
	}
	v = append(v, mathPowersOfTen...)
	for k := -40; k <= 40; k++ {
		v = append(v, float64(k), float64(k)+0.5, float64(k)/10, float64(k*k*k))
	}
	return v
}()

// mathPowersOfTen are the powers of ten from 1e-323 to 1e308.
var mathPowersOfTen = func() []float64 {
	var v []float64
	for e := -323; e <= 308; e++ {
		f, _ := strconv.ParseFloat("1e"+strconv.Itoa(e), 64)
		v = append(v, f)
	}
	return v
}()

// mathPairValues are the arguments of the functions of two arguments: each
// pair of two values is a case.
var mathPairValues = func() []float64 {
	inf := math.Inf(1)
	return []float64{
		0, math.Copysign(0, -1), 1, -1, 2, -2, 0.5, -0.5, 3, -3, 10, 1.5, 2.5, -1.5, 0.25, 1.0 / 3, 0.1,
		inf, -inf, math.NaN(), 5e-324, -5e-324, math.MaxFloat64, -math.MaxFloat64,
		2.2250738585072014e-308, 1.0000000000000002, 0.9999999999999999, -0.9999999999999999,
		1e300, -1e300, 1e-300, 1074, -1074, 1023, 1024, -1075, 0x1p53, 0x1p53 + 2, 0x1p52 + 1, -0x1p52 - 1,
		0x1p63, 0x1p64, 0x1p-65, 0x1p-66, 4, 7, 0.99, 1e-5, 100, 1e22,
	}
}()

// pow returns arguments for Math.pow.
func (g *mathGen) pow() (x, y float64) {
	switch k := g.intn(40); {
	case k < 8:
		return g.exp(-5, 5), g.uniform(-25, 25)
	case k < 10:
		return g.decimal(), g.decimal()
	case k < 12:
		// A result that is near the overflow or the underflow limit.
		e := g.intn(2046) - 1022
		x = math.Float64frombits(uint64(e+1023)<<52 | g.u64()>>12)
		t := g.uniform(-1100, 1050)
		return x, t / (float64(e) + 0.5)
	case k < 14:
		return g.near(1, 1<<uint(g.intn(40))), g.sign(g.exp(0, 62))
	case k < 16:
		return g.integer(40), g.integer(40)
	case k < 18:
		return 10, g.integer(340)
	case k < 19:
		return 2, g.uniform(-1080, 1030)
	case k < 20:
		return 2, g.integer(1080)
	case k < 22:
		return g.sign(g.exp(-3, 3)), g.integer(60) + float64(float64(g.intn(2))/2)
	case k < 24:
		return g.sign(g.short(-10, 10, 1+g.intn(6))), g.short(-4, 4, 1+g.intn(4))
	case k < 26:
		return g.exp(-1022, 1023), g.sign(g.exp(-70, 12))
	case k < 28:
		return g.raw(), g.raw()
	case k < 29:
		return g.subnormal(), g.uniform(-2, 2)
	case k < 30:
		return g.any(), g.any()
	case k < 32:
		// Powers that are exact: a short base and a small exponent.
		return g.sign(float64(1+g.intn(1<<uint(1+g.intn(12))))) * math.Ldexp(1, g.intn(21)-10), float64(g.intn(12)) / 2
	case k < 34:
		return g.uniform(0, 1), g.uniform(0, 4)
	case k < 35:
		return g.exp(-1, 0), 2.4 // the gamma of sRGB
	case k < 36:
		return g.uniform(0, 300), 1 / float64(1+g.intn(5))
	case k < 37:
		return g.sign(g.exp(-1022, 1023)), mathPairValues[g.intn(len(mathPairValues))]
	case k < 38:
		return mathPairValues[g.intn(len(mathPairValues))], g.sign(g.exp(-80, 80))
	}
	return g.exp(-40, 40), g.uniform(-30, 30)
}

// pair returns arguments for Math.atan2 and Math.hypot.
func (g *mathGen) pair() (y, x float64) {
	switch k := g.intn(20); {
	case k < 6:
		return g.sign(g.exp(-10, 10)), g.sign(g.exp(-10, 10))
	case k < 8:
		return g.decimal(), g.decimal()
	case k < 10:
		return g.integer(2000), g.integer(2000)
	case k < 12:
		return g.raw(), g.raw()
	case k < 14:
		return g.sign(g.exp(-1022, 1023)), g.sign(g.exp(-1022, 1023))
	case k < 15:
		return g.sign(g.subnormal()), g.sign(g.subnormal())
	case k < 16:
		return g.sign(g.subnormal()), g.sign(g.exp(-1022, -900))
	case k < 17:
		// A ratio that is near k/64.
		x = g.sign(g.exp(-3, 3))
		return g.near(x*float64(g.intn(129))/64, 4), x
	case k < 18:
		x = g.sign(g.exp(-200, 200))
		return g.near(x, 1<<uint(g.intn(30))), x
	case k < 19:
		return g.any(), g.any()
	}
	// Exponents that are far apart.
	e := g.intn(1900) - 950
	d := g.intn(130) - 65
	return g.sign(math.Ldexp(g.exp(0, 0), e)), g.sign(math.Ldexp(g.exp(0, 0), e+d))
}

// one returns an argument for a function of one argument.
func (g *mathGen) one(name string) float64 {
	k := g.intn(20)
	switch name {
	case "sin", "cos", "tan":
		switch {
		case k < 6:
			return g.sign(g.exp(-6, 9))
		case k < 8:
			return g.uniform(-7, 7)
		case k < 10:
			return g.raw()
		case k < 12:
			return g.sign(g.exp(10, 1023))
		case k < 14:
			return g.sign(g.exp(-60, -7))
		case k < 16:
			// A number that is near a multiple of pi/2.
			return g.sign(g.near(float64(g.intn(1<<uint(1+g.intn(40))))*(math.Pi/2), 1<<uint(g.intn(12))))
		case k < 17:
			return g.integer(720) * math.Pi / 180
		case k < 18:
			return g.decimal()
		case k < 19:
			return g.sign(g.exp(-1022, -61))
		}
		return g.integer(4096) * (math.Pi / float64(int(1)<<uint(g.intn(8))))
	case "asin", "acos", "atanh":
		switch {
		case k < 6:
			return g.uniform(-1, 1)
		case k < 9:
			return g.sign(g.near(1, 1<<uint(g.intn(50))))
		case k < 12:
			return g.sign(g.exp(-60, -1))
		case k < 13:
			return g.sign(g.exp(-1022, -61))
		case k < 14:
			return g.raw()
		case k < 16:
			return g.sign(g.short(-12, -1, 1+g.intn(10)))
		case k < 18:
			return g.sign(g.near(0.5, 1<<uint(g.intn(50))))
		case k < 19:
			return g.decimal() / 1e6
		}
		return g.sign(g.exp(-1, 0))
	case "atan", "asinh":
		switch {
		case k < 6:
			return g.sign(g.exp(-8, 8))
		case k < 8:
			return g.raw()
		case k < 10:
			return g.sign(g.exp(-60, -9))
		case k < 12:
			return g.sign(g.exp(9, 70))
		case k < 14:
			return g.decimal()
		case k < 15:
			return g.integer(100000)
		case k < 16:
			return g.sign(g.exp(-1022, 1023))
		case k < 18:
			// A number that is near k/64 or near its inverse.
			c := float64(1+g.intn(128)) / 64
			if g.intn(2) == 0 {
				c = 1 / c
			}
			return g.sign(g.near(c, 1<<uint(g.intn(20))))
		case k < 19:
			return g.sign(g.near(1, 1<<uint(g.intn(50))))
		}
		return g.sign(g.short(-30, 30, 1+g.intn(10)))
	case "exp", "expm1", "sinh", "cosh", "tanh":
		switch {
		case k < 5:
			return g.uniform(-746, 711)
		case k < 9:
			return g.sign(g.exp(-10, 5))
		case k < 11:
			return g.sign(g.exp(-60, -11))
		case k < 12:
			return g.raw()
		case k < 13:
			return g.sign(g.exp(-1022, -61))
		case k < 14:
			return g.integer(750)
		case k < 15:
			return g.decimal()
		case k < 16:
			return g.sign(g.near(float64(g.intn(1100))*math.Ln2, 1<<uint(g.intn(16))))
		case k < 17:
			return g.sign(g.near([]float64{709.782712893384, 745.1332191019411, 710.4758600739439, 22, 1, 0.5, 0.34657359027997264, 708.3964185322641}[g.intn(8)], 1<<uint(g.intn(12))))
		case k < 18:
			return g.sign(g.exp(5, 10))
		case k < 19:
			return g.uniform(-25, 25)
		}
		return g.sign(g.short(-20, 9, 1+g.intn(10)))
	case "log", "log2", "log10":
		switch {
		case k < 5:
			return g.exp(-1022, 1023)
		case k < 9:
			return g.exp(-6, 6)
		case k < 12:
			return g.near(1, 1<<uint(g.intn(52)))
		case k < 13:
			return g.raw()
		case k < 14:
			return g.subnormal()
		case k < 15:
			return float64(1 + g.intn(1<<uint(1+g.intn(52))))
		case k < 16:
			return g.uniform(0, 2)
		case k < 17:
			f := g.decimal()
			if f < 0 {
				f = -f
			}
			return f
		case k < 18:
			return g.short(-1022, 1023, 1+g.intn(10))
		case k < 19:
			// A number that is near a power of ten.
			return g.near(mathPowersOfTen[g.intn(len(mathPowersOfTen))], 1<<uint(g.intn(8)))
		}
		return g.near(math.Ldexp(1, g.intn(2046)-1022), 1<<uint(g.intn(20)))
	case "log1p":
		switch {
		case k < 5:
			return g.sign(g.exp(-60, -1))
		case k < 8:
			return g.exp(-1, 60)
		case k < 10:
			return g.uniform(-1, 1)
		case k < 12:
			return g.near(-1, 1<<uint(g.intn(52)))
		case k < 13:
			return g.raw()
		case k < 14:
			return g.sign(g.exp(-1022, -61))
		case k < 15:
			return g.exp(60, 1023)
		case k < 16:
			return g.decimal()
		case k < 18:
			return g.sign(g.near([]float64{0.5, 1, 0.25, 0.41421356237309503, 0.2928932188134524, 0.125, 2}[g.intn(7)], 1<<uint(g.intn(30))))
		case k < 19:
			return g.sign(g.short(-40, 10, 1+g.intn(10)))
		}
		return g.sign(g.subnormal())
	case "acosh":
		switch {
		case k < 5:
			return g.uniform(1, 2)
		case k < 8:
			return g.near(1, 1<<uint(g.intn(52)))
		case k < 11:
			return g.exp(0, 30)
		case k < 13:
			return g.exp(30, 1023)
		case k < 14:
			return g.raw()
		case k < 15:
			f := g.decimal()
			if f < 0 {
				f = -f
			}
			return f
		case k < 17:
			return g.near([]float64{2, 0x1p28, 1.5, 4}[g.intn(4)], 1<<uint(g.intn(30)))
		case k < 18:
			return g.exp(-3, 0)
		}
		return 1 + g.short(-52, 2, 1+g.intn(10))
	case "cbrt", "sqrt":
		switch {
		case k < 6:
			return g.sign(g.exp(-1022, 1023))
		case k < 10:
			return g.sign(g.exp(-10, 10))
		case k < 12:
			return g.raw()
		case k < 13:
			return g.sign(g.subnormal())
		case k < 15:
			// A cube or a square of a short number, and its neighbors.
			f := g.short(-100, 100, 1+g.intn(16))
			if name == "cbrt" {
				f = f * f * f
			} else {
				f = f * f
			}
			return g.sign(g.near(f, g.intn(3)))
		case k < 17:
			return g.decimal()
		case k < 18:
			return g.integer(1000000)
		}
		return g.sign(g.short(-1022, 1023, 1+g.intn(10)))
	case "fround":
		switch {
		case k < 5:
			return g.raw()
		case k < 9:
			return g.sign(g.exp(-150, 128))
		case k < 14:
			// A number that is near the middle of two float32 numbers.
			f := float64(math.Float32frombits(uint32(g.u64()) & 0x7fffffff))
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return f
			}
			h := float64(math.Float32frombits(math.Float32bits(float32(f)) + 1))
			if math.IsInf(h, 0) {
				h = 0x1p128
			}
			return g.sign(g.near(f+float64((h-f)/2), g.intn(3)))
		case k < 16:
			return g.sign(g.near(float64(math.MaxFloat32), 1<<uint(g.intn(40))))
		case k < 17:
			return g.sign(g.near(0x1p-149, 1<<uint(g.intn(50))))
		case k < 18:
			return g.decimal()
		}
		return g.sign(g.exp(-1022, 1023))
	case "round", "trunc", "sign", "clz32":
		switch {
		case k < 4:
			return g.integer(1<<uint(1+g.intn(52))) + 0.5
		case k < 7:
			return g.sign(g.near(float64(g.intn(1<<uint(1+g.intn(52))))+0.5, 2))
		case k < 9:
			return g.raw()
		case k < 12:
			return g.sign(g.exp(-5, 54))
		case k < 14:
			return g.decimal()
		case k < 15:
			return g.sign(g.exp(-1022, 1023))
		case k < 16:
			return g.sign(g.near(0.5, 3))
		case k < 18:
			return g.sign(g.exp(28, 36))
		case k < 19:
			return g.integer(1 << uint(1+g.intn(33)))
		}
		return g.sign(g.subnormal())
	}
	panic("no arguments for " + name)
}

// imul returns arguments for Math.imul.
func (g *mathGen) imul() (a, b float64) {
	one := func() float64 {
		switch g.intn(8) {
		case 0:
			return g.any()
		case 1:
			return g.sign(g.exp(28, 70))
		case 2:
			return g.decimal()
		case 3:
			return g.sign(g.exp(-5, 34))
		}
		return float64(int64(g.u64()) >> uint(31+g.intn(22)))
	}
	return one(), one()
}

// mathArity is the number of arguments of a function of the golden file.
func mathArity(name string) int {
	switch name {
	case "atan2", "pow", "hypot", "imul":
		return 2
	case "hypot3":
		return 3
	case "hypot4":
		return 4
	}
	return 1
}

// mathInputs returns the arguments of n cases of a function. The arguments
// of a case are consecutive. The first cases are the edge values.
func mathInputs(name string, n int) []float64 { return mathInputsSalt(name, n, 0) }

// mathInputsSalt is mathInputs with another sequence of random arguments for
// each salt. The golden file has the salt 0.
func mathInputsSalt(name string, n int, salt uint64) []float64 {
	arity := mathArity(name)
	seed := uint64(0x6d6d6467) + salt*0x9e3779b97f4a7c15
	for i := 0; i < len(name); i++ {
		seed = seed*1099511628211 ^ uint64(name[i])
	}
	g := &mathGen{s: seed}
	in := make([]float64, 0, n*arity)
	switch arity {
	case 1:
		in = append(in, mathEdgeValues...)
	case 2:
		for _, a := range mathPairValues {
			for _, b := range mathPairValues {
				in = append(in, a, b)
			}
		}
	}
	if len(in) > n*arity {
		in = in[:n*arity]
	}
	for len(in) < n*arity {
		switch name {
		case "pow":
			x, y := g.pow()
			in = append(in, x, y)
		case "atan2", "hypot":
			y, x := g.pair()
			in = append(in, y, x)
		case "imul":
			a, b := g.imul()
			in = append(in, a, b)
		case "hypot1":
			in = append(in, g.any())
		case "hypot3", "hypot4":
			// Arguments of a similar size, so that each one has an effect.
			a, b := g.pair()
			in = append(in, a, b)
			for k := 2; k < arity; k++ {
				switch g.intn(4) {
				case 0:
					in = append(in, g.any())
				case 1:
					in = append(in, g.near(a, 1<<uint(g.intn(50))))
				default:
					in = append(in, g.sign(a*g.uniform(0, 4)))
				}
			}
		default:
			in = append(in, g.one(name))
		}
	}
	return in
}

// mathInputHash is the FNV-1a hash of the bits of the arguments.
func mathInputHash(in []float64) uint64 {
	h := uint64(14695981039346656037)
	for _, f := range in {
		b := math.Float64bits(f)
		for i := 0; i < 8; i++ {
			h ^= b >> (8 * uint(i)) & 0xff
			h *= 1099511628211
		}
	}
	return h
}

// TestMathWriteInputs writes the arguments of all functions to the file that
// the environment variable JSMATH_INPUTS names. JSMATH_N is the number of
// cases of a transcendental function, and JSMATH_N_EXACT is the number of
// cases of the other functions. tools/reference/gen-jsmath.mjs runs this
// test.
func TestMathWriteInputs(t *testing.T) {
	path := os.Getenv("JSMATH_INPUTS")
	if path == "" {
		t.Skip("JSMATH_INPUTS is not set")
	}
	count := func(env string, def int) int {
		if s := os.Getenv(env); s != "" {
			v, err := strconv.Atoi(s)
			if err != nil {
				t.Fatal(err)
			}
			return v
		}
		return def
	}
	n := count("JSMATH_N", 20000)
	nExact := count("JSMATH_N_EXACT", 5000)
	buf := []byte("JSMATHIN")
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(mathFuncs)))
	for _, f := range mathFuncs {
		c := n
		if f.exact {
			c = nExact
		}
		in := mathInputs(f.name, c)
		buf = append(buf, byte(len(f.name)))
		buf = append(buf, f.name...)
		buf = append(buf, byte(mathArity(f.name)))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(c))
		buf = binary.LittleEndian.AppendUint64(buf, mathInputHash(in))
		for _, x := range in {
			buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(x))
		}
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}
