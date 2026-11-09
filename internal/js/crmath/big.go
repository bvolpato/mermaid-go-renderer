// This file ports no upstream file. It has the slow path of the correctly
// rounded functions: a computation with math/big whose precision grows until
// the result rounds to one float64 (the strategy of Ziv). The fast paths call
// it when their rounding test fails, which is rare.
//
// The exact value of each function here is never in the middle of two
// float64 numbers for an argument that the callers pass, so the loop ends.

package crmath

import (
	"math"
	"math/big"
	"sync"
)

// bigGuard is the number of bits that a computation has above the precision
// that the caller asks for.
const bigGuard = 96

// bigConstPrec is the precision of the cached constants. The argument
// reduction of the largest float64 needs 1024 bits above the working
// precision.
const bigConstPrec = 4096

var (
	bigOnce   sync.Once
	bigHalfPi *big.Float // pi/2
	bigLn2    *big.Float // log(2)
)

func bigNew(prec uint) *big.Float { return new(big.Float).SetPrec(prec) }

// bigAtanInv is atan(1/n) with prec bits, by the Taylor series. With
// hyperbolic set, it is atanh(1/n).
func bigAtanInv(n int64, prec uint, hyperbolic bool) *big.Float {
	sum := bigNew(prec)
	term := bigNew(prec).Quo(bigNew(prec).SetInt64(1), bigNew(prec).SetInt64(n))
	n2 := bigNew(prec).SetInt64(n * n)
	t := bigNew(prec)
	d := bigNew(prec)
	for k := int64(0); ; k++ {
		t.Quo(term, d.SetInt64(2*k+1))
		if t.MantExp(nil) < -int(prec) {
			break
		}
		if hyperbolic || k%2 == 0 {
			sum.Add(sum, t)
		} else {
			sum.Sub(sum, t)
		}
		term.Quo(term, n2)
	}
	return sum
}

func bigPiLn2(prec uint) (halfPi, ln2 *big.Float) {
	p := prec + 64
	// The Machin formula: pi/4 = 4 atan(1/5) - atan(1/239).
	a := bigAtanInv(5, p, false)
	a.Mul(a, bigNew(p).SetInt64(8))
	b := bigAtanInv(239, p, false)
	b.Mul(b, bigNew(p).SetInt64(2))
	a.Sub(a, b)
	// log(2) = 2 atanh(1/3)
	l := bigAtanInv(3, p, true)
	l.Mul(l, bigNew(p).SetInt64(2))
	return a.SetPrec(prec), l.SetPrec(prec)
}

// bigConsts returns pi/2 and log(2), rounded to prec bits.
func bigConsts(prec uint) (halfPi, ln2 *big.Float) {
	if prec > bigConstPrec {
		return bigPiLn2(prec)
	}
	bigOnce.Do(func() { bigHalfPi, bigLn2 = bigPiLn2(bigConstPrec) })
	return bigNew(prec).Set(bigHalfPi), bigNew(prec).Set(bigLn2)
}

// bigRound rounds v to a float64. v has a relative error below 2^-prec. ok is
// false when the values within the error bound do not all round to the same
// float64.
func bigRound(v *big.Float, prec uint) (r float64, ok bool) {
	if v.Sign() == 0 {
		return 0, true
	}
	p := v.Prec()
	// 2^-prec times a number above |v|.
	e := bigNew(p).SetMantExp(bigNew(p).SetInt64(1), v.MantExp(nil)-int(prec))
	lo, _ := bigNew(p).Sub(v, e).Float64()
	hi, _ := bigNew(p).Add(v, e).Float64()
	return lo, lo == hi
}

// bigCompute calls f with a growing precision until the result rounds to one
// float64. f returns a value with a relative error below 2^-prec, in a
// number with more than prec bits.
func bigCompute(f func(prec uint) *big.Float) float64 {
	for prec := uint(128); ; prec *= 2 {
		if r, ok := bigRound(f(prec), prec); ok {
			return r
		}
	}
}

// bigSinCos returns sin(x) and cos(x) with a relative error below 2^-prec.
// x is finite and not 0.
func bigSinCos(x float64, prec uint) (sin, cos *big.Float) {
	_, e := math.Frexp(x)
	if e < 0 {
		e = 0
	}
	// The reduction x - k (pi/2) cancels e bits, and the remainder can have
	// about 62 zero bits more for a float64 that is near a multiple of pi/2.
	p := prec + uint(e) + bigGuard
	halfPi, _ := bigConsts(p)
	bx := bigNew(p).SetFloat64(x)
	// k = round(x / (pi/2))
	q := bigNew(p).Quo(bx, halfPi)
	q.Add(q, bigNew(p).SetFloat64(math.Copysign(0.5, x)))
	k, _ := q.Int(nil)
	r := bigNew(p).SetInt(k)
	r.Mul(r, halfPi)
	r.Sub(bx, r)

	// sin and cos of r / 2^halvings by the Taylor series, then the double
	// angle formulas. Each doubling adds a relative error of about 2^-w,
	// which the guard bits cover.
	const halvings = 16
	w := prec + bigGuard
	a := bigNew(w).Set(r)
	a.SetMantExp(a, -halvings)
	a2 := bigNew(w).Mul(a, a)
	s := bigNew(w).Set(a)
	c := bigNew(w).SetInt64(1)
	ts := bigNew(w).Set(a)
	tc := bigNew(w).SetInt64(1)
	d := bigNew(w)
	for i := int64(1); i < 1000; i++ {
		tc.Mul(tc, a2)
		tc.Quo(tc, d.SetInt64((2*i-1)*(2*i)))
		ts.Mul(ts, a2)
		ts.Quo(ts, d.SetInt64((2*i)*(2*i+1)))
		if i%2 == 1 {
			c.Sub(c, tc)
			s.Sub(s, ts)
		} else {
			c.Add(c, tc)
			s.Add(s, ts)
		}
		if tc.Sign() == 0 || tc.MantExp(nil) < -int(w)-8 {
			break
		}
	}
	t := bigNew(w)
	for i := 0; i < halvings; i++ {
		// sin(2a) = 2 sin(a) cos(a), cos(2a) = 1 - 2 sin(a)^2
		t.Mul(s, s)
		s.Mul(s, c)
		s.SetMantExp(s, 1)
		t.SetMantExp(t, 1)
		c.Sub(d.SetInt64(1), t)
	}
	switch new(big.Int).And(k, big.NewInt(3)).Int64() {
	case 0:
		return s, c
	case 1:
		return c, s.Neg(s)
	case 2:
		return s.Neg(s), c.Neg(c)
	}
	return c.Neg(c), s
}

// bigAtan returns atan(y) for a positive y, with a relative error below
// 2^-prec.
func bigAtan(y *big.Float, prec uint) *big.Float {
	w := prec + bigGuard
	one := bigNew(w).SetInt64(1)
	a := bigNew(w).Set(y)
	invert := a.Cmp(one) > 0
	if invert {
		// atan(y) = pi/2 - atan(1/y)
		a.Quo(one, a)
	}
	// atan(a) = 2 atan(a / (1 + sqrt(1 + a^2)))
	const halvings = 6
	t := bigNew(w)
	for i := 0; i < halvings; i++ {
		t.Mul(a, a)
		t.Add(t, one)
		t.Sqrt(t)
		t.Add(t, one)
		a.Quo(a, t)
	}
	// The Taylor series a - a^3/3 + a^5/5 - ...
	a2 := bigNew(w).Mul(a, a)
	sum := bigNew(w).Set(a)
	term := bigNew(w).Set(a)
	d := bigNew(w)
	for i := int64(1); i < 2000; i++ {
		term.Mul(term, a2)
		t.Quo(term, d.SetInt64(2*i+1))
		if i%2 == 1 {
			sum.Sub(sum, t)
		} else {
			sum.Add(sum, t)
		}
		if t.Sign() == 0 || t.MantExp(nil)-sum.MantExp(nil) < -int(w)-8 {
			break
		}
	}
	sum.SetMantExp(sum, halvings)
	if invert {
		halfPi, _ := bigConsts(w)
		sum.Sub(halfPi, sum)
	}
	return sum
}

// bigExp returns exp(x) with a relative error below 2^-prec. |x| is below
// 2^20.
func bigExp(x *big.Float, prec uint) *big.Float {
	e := 0
	if x.Sign() != 0 {
		e = x.MantExp(nil)
	}
	if e < 0 {
		e = 0
	}
	w := prec + uint(e) + bigGuard
	_, ln2 := bigConsts(w)
	// k = round(x / log(2)), r = x - k log(2)
	q := bigNew(w).Quo(x, ln2)
	half := 0.5
	if q.Sign() < 0 {
		half = -0.5
	}
	q.Add(q, bigNew(w).SetFloat64(half))
	k, _ := q.Int64()
	r := bigNew(w).SetInt64(k)
	r.Mul(r, ln2)
	r.Sub(bigNew(w).Set(x), r)
	// exp(r / 2^halvings) by the Taylor series, then squarings. A squaring
	// doubles the relative error, which the guard bits cover.
	const halvings = 12
	r.SetMantExp(r, -halvings)
	sum := bigNew(w).SetInt64(1)
	term := bigNew(w).SetInt64(1)
	d := bigNew(w)
	for i := int64(1); i < 1000; i++ {
		term.Mul(term, r)
		term.Quo(term, d.SetInt64(i))
		if term.Sign() == 0 || term.MantExp(nil) < -int(w)-8 {
			break
		}
		sum.Add(sum, term)
	}
	for i := 0; i < halvings; i++ {
		sum.Mul(sum, sum)
	}
	return sum.SetMantExp(sum, int(k))
}

// bigExpm1 returns exp(x) - 1 with a relative error below 2^-prec. x is
// finite and not 0.
func bigExpm1(x float64, prec uint) *big.Float {
	w := prec + bigGuard
	bx := bigNew(w).SetFloat64(x)
	if math.Abs(x) >= 0.5 {
		// The subtraction cancels less than two bits.
		r := bigExp(bx, prec+8)
		r.SetPrec(r.Prec() + 8)
		return r.Sub(r, bigNew(w).SetInt64(1))
	}
	// The Taylor series x + x^2/2! + x^3/3! + ...
	sum := bigNew(w).Set(bx)
	term := bigNew(w).Set(bx)
	d := bigNew(w)
	for i := int64(2); i < 1000; i++ {
		term.Mul(term, bx)
		term.Quo(term, d.SetInt64(i))
		if term.Sign() == 0 || term.MantExp(nil)-sum.MantExp(nil) < -int(w)-8 {
			break
		}
		sum.Add(sum, term)
	}
	return sum
}

// bigAtanh2 returns 2 atanh(z) = log((1+z)/(1-z)) for |z| < 0.2, with a
// relative error below 2^-(w-16) for a z with w bits.
func bigAtanh2(z *big.Float, w uint) *big.Float {
	z2 := bigNew(w).Mul(z, z)
	sum := bigNew(w).Set(z)
	term := bigNew(w).Set(z)
	t := bigNew(w)
	d := bigNew(w)
	for i := int64(1); i < 100000; i++ {
		term.Mul(term, z2)
		t.Quo(term, d.SetInt64(2*i+1))
		if t.Sign() == 0 || t.MantExp(nil)-sum.MantExp(nil) < -int(w)-8 {
			break
		}
		sum.Add(sum, t)
	}
	return sum.SetMantExp(sum, 1)
}

// bigLog returns log(t) with a relative error below 2^-prec. t is positive
// and not 1.
func bigLog(t *big.Float, prec uint) *big.Float {
	w := prec + bigGuard
	if tp := t.Prec() + 8; w < tp {
		w = tp
	}
	// t = m 2^e with m from sqrt(1/2) to sqrt(2).
	m := bigNew(w)
	e := t.MantExp(m)
	if m.Cmp(bigNew(w).SetFloat64(0.7071067811865476)) < 0 {
		m.SetMantExp(m, 1)
		e--
	}
	// log(m) = 2 atanh((m - 1) / (m + 1)). The subtraction is exact.
	one := bigNew(w).SetInt64(1)
	num := bigNew(w).Sub(m, one)
	den := bigNew(w).Add(m, one)
	sum := bigAtanh2(num.Quo(num, den), w)
	if e != 0 {
		// |e log(2)| is above 2 |log(m)|, so the sum cancels one bit at most.
		_, ln2 := bigConsts(w)
		ln2.Mul(ln2, bigNew(w).SetInt64(int64(e)))
		sum.Add(sum, ln2)
	}
	return sum
}

// bigLog1p returns log(1 + x) with a relative error below 2^-prec. x is
// above -1, finite and not 0.
func bigLog1p(x float64, prec uint) *big.Float {
	w := prec + bigGuard
	bx := bigNew(w).SetFloat64(x)
	if math.Abs(x) < 0.25 {
		// log(1 + x) = 2 atanh(x / (2 + x))
		den := bigNew(w).Add(bx, bigNew(w).SetInt64(2))
		return bigAtanh2(bx.Quo(bx, den), w)
	}
	// 1 + x is exact for x below 2^w, and it is rounded above.
	return bigLog(bx.Add(bx, bigNew(w).SetInt64(1)), prec)
}

// bigCbrt returns the cube root of a positive x with a relative error below
// 2^-prec.
func bigCbrt(x float64, prec uint) *big.Float {
	w := prec + bigGuard
	// x = a 8^q with a from 1/2 to 4.
	frac, e := math.Frexp(x)
	q := e / 3
	if e%3 < 0 {
		q--
	}
	a := math.Ldexp(frac, e-3*q)
	ba := bigNew(w).SetFloat64(a)
	// The iteration of Newton, y = y - (y^3 - a) / (3 y^2), from a float64.
	y := bigNew(w).SetFloat64(math.Cbrt(a))
	y2 := bigNew(w)
	t := bigNew(w)
	three := bigNew(w).SetInt64(3)
	for bits := uint(40); bits < 2*w; bits *= 2 {
		y2.Mul(y, y)
		t.Mul(y2, y)
		t.Sub(t, ba)
		y2.Mul(y2, three)
		t.Quo(t, y2)
		y.Sub(y, t)
	}
	return y.SetMantExp(y, q)
}

// sinSlow is the correctly rounded sine of a finite x that is not 0.
func sinSlow(x float64) float64 {
	return bigCompute(func(prec uint) *big.Float {
		s, _ := bigSinCos(x, prec)
		return s
	})
}

// cosSlow is the correctly rounded cosine of a finite x that is not 0.
func cosSlow(x float64) float64 {
	return bigCompute(func(prec uint) *big.Float {
		_, c := bigSinCos(x, prec)
		return c
	})
}

// tanSlow is the correctly rounded tangent of a finite x that is not 0.
func tanSlow(x float64) float64 {
	return bigCompute(func(prec uint) *big.Float {
		s, c := bigSinCos(x, prec+8)
		return s.Quo(s, c)
	})
}

// asinSlow is the correctly rounded arc sine for 0 < |x| < 1.
func asinSlow(x float64) float64 {
	ax := math.Abs(x)
	r := bigCompute(func(prec uint) *big.Float {
		// asin(x) = atan(x / sqrt((1 - x) (1 + x)))
		w := prec + bigGuard + 64
		bx := bigNew(w).SetFloat64(ax)
		one := bigNew(w).SetInt64(1)
		d := bigNew(w).Sub(one, bx)
		d.Mul(d, bigNew(w).Add(one, bx))
		d.Sqrt(d)
		return bigAtan(bx.Quo(bx, d), prec)
	})
	return math.Copysign(r, x)
}

// acosSlow is the correctly rounded arc cosine for 0 < |x| < 1.
func acosSlow(x float64) float64 {
	ax := math.Abs(x)
	return bigCompute(func(prec uint) *big.Float {
		// acos(x) = atan(sqrt((1 - x) (1 + x)) / x) for x > 0, and pi minus
		// the same expression of -x for x < 0.
		w := prec + bigGuard + 64
		bx := bigNew(w).SetFloat64(ax)
		one := bigNew(w).SetInt64(1)
		d := bigNew(w).Sub(one, bx)
		d.Mul(d, bigNew(w).Add(one, bx))
		d.Sqrt(d)
		a := bigAtan(d.Quo(d, bx), prec+4)
		if x < 0 {
			pi, _ := bigConsts(a.Prec())
			pi.SetMantExp(pi, 1)
			a.Sub(pi, a)
		}
		return a
	})
}

// expSlow is the correctly rounded exponential of a finite x that is not 0.
func expSlow(x float64) float64 {
	return bigCompute(func(prec uint) *big.Float {
		return bigExp(bigNew(64).SetFloat64(x), prec)
	})
}

// expm1Slow is the correctly rounded exp(x) - 1 of a finite x that is not 0.
func expm1Slow(x float64) float64 {
	return bigCompute(func(prec uint) *big.Float { return bigExpm1(x, prec) })
}

// logSlow is the correctly rounded natural logarithm of a positive finite x
// that is not 1.
func logSlow(x float64) float64 {
	return bigCompute(func(prec uint) *big.Float {
		return bigLog(bigNew(64).SetFloat64(x), prec)
	})
}

// log2Slow is the correctly rounded logarithm to base 2 of a positive finite
// x that is not 1.
func log2Slow(x float64) float64 {
	return bigCompute(func(prec uint) *big.Float {
		l := bigLog(bigNew(64).SetFloat64(x), prec+8)
		_, ln2 := bigConsts(l.Prec())
		return l.Quo(l, ln2)
	})
}

// log10Slow is the correctly rounded logarithm to base 10 of a positive
// finite x that is not 1.
func log10Slow(x float64) float64 {
	return bigCompute(func(prec uint) *big.Float {
		l := bigLog(bigNew(64).SetFloat64(x), prec+8)
		ln10 := bigLog(bigNew(64).SetInt64(10), prec+8)
		return l.Quo(l, ln10)
	})
}

// log1pSlow is the correctly rounded log(1 + x) for a finite x above -1 that
// is not 0.
func log1pSlow(x float64) float64 {
	return bigCompute(func(prec uint) *big.Float { return bigLog1p(x, prec) })
}

// cbrtSlow is the correctly rounded cube root of a finite x that is not 0.
func cbrtSlow(x float64) float64 {
	r := bigCompute(func(prec uint) *big.Float { return bigCbrt(math.Abs(x), prec) })
	return math.Copysign(r, x)
}
