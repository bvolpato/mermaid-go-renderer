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

// Precomputed 4096-bit values of pi/2 and log(2), bit-exact with bigPiLn2(4096).
const (
	bigHalfPi4096Hex = "0x1.921fb54442d18469898cc51701b839a252049c1114cf98e804177d4c76273644a29410f31c6809bbdf2a33679a748636605614dbe4be286e9fc26adadaa3848bc90b6aecc4bcfd8de89885d34c6fdad617feb96de80d6fdbdc70d7f6b5133f4b5d3e4822f8963fcc9250cca3d9c8b67b8400f97142c77e0b31b4906c38aba734d22c7f51fa499ebf06caba47b9475b2c38c5e6ac410aa5773daa520ee12d2cdace186a9c95793009e2e8d811943042f86520bc8c5c6d9c77c73cee58301d0c07364f0745d80f451f6b8abbe0de98a593bc5797ed2ab02e30732a92f9d52ad5ca2ba44c3131f40a202ae51cb51555885b5a662e1a08a0f46750aa4357be3974c9d9f70a08b1b7de1515d4e2aeba0c18fb672e1f0b4dc3c98f57eb5d19b61267ae3d1929c0944ac33b9dc7a44c35a5dcd7e25ff40db31410c9b0ec04e67d90d4c8a43e56302ef6401977c22eaef4c2bad8ee13118175b28dc411c49f40e9cb566287b6b7f9c1fa211c9705a2415242100234e478254f0fcdaf10e334217b74b64d33864e30d5e9c4783528d0696c2a17b44b07d39455a899d1b77785b609bd1df25d1df8283f7d954c50f8b28e9cd780bb33652c9f412187444677430ca2b7cfda3ec252e19dc5af5f7037baec42e09039a00d224fab60b5532769d5311b1fbb830dff6fb9214d811e9be86b92680509246d87f569a4f8e04d83a9b964c04c8dbep+00"
	bigLn24096Hex    = "0x1.62e42fefa39ef35793c7673007e5ed5e81e6864ce5316c5b141a2eb71755f457cf70ec40dbd75930ab2aa5f695f43621da5d5c6b827042884eae765222d3704a7d2d942c4495d18a3597b42262f870fd73d53787626cc0764adf41d8ecafee96e59d0f633aca9195ebbf4d7a70606490cabf430e5e41c745b45b2f8a1e7fab8c0fe99423f6b7f720c21a61f11fcaa345dcad3adbf83df42afa5c47bc2801672c2e8c0eebb71321cb9287ce6568f39a6799989ccb2726a29898343c17a3ac12ba4acd36666ac9466ed538ff14bc291d040e9b6c02b9fcf546189014a82e6a1a592abaa2f363c2f73b5c6279b6d8c0d9620f1ee6ba365b6636bea16a30a0c983169a2c5b6766cb0a7aeb31432a35c4e7dcaae16d8d1f2d30692da9cda6615f113689404aa8e639b91d42e527a245149df31adea2eff79e0eaa4d14b83f2a717304c35ffa88d639479ebd24457118cda78a844307db9328421217762df5e7b293e46dc056419dd10d720b8251aa7a17a5f2c426c632d5ea060400c1c93210723418ae6737457d74fa0a558b6c3989d240f9de5e19c5ae6e72b1aec44cb1203cc8d52a3088c1b89ce90e2adc18524827abc6c382d2dba4955d7a8e704dfb41847172156223777acf8e492e59a317f77b3a84d88e412dcec22b80bedef9d7593e8b5d9d96e5e33870673b1ed04c4a1bd5123de0f5ffe751246e9c2ebd695f91b55bb2p-01"
)

// bigConsts returns pi/2 and log(2), rounded to prec bits.
func bigConsts(prec uint) (halfPi, ln2 *big.Float) {
	if prec > bigConstPrec {
		return bigPiLn2(prec)
	}
	bigOnce.Do(func() {
		var err1, err2 error
		bigHalfPi, _, err1 = big.ParseFloat(bigHalfPi4096Hex, 0, bigConstPrec, big.ToNearestEven)
		bigLn2, _, err2 = big.ParseFloat(bigLn24096Hex, 0, bigConstPrec, big.ToNearestEven)
		if err1 != nil || err2 != nil {
			bigHalfPi, bigLn2 = bigPiLn2(bigConstPrec)
		}
	})
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

var (
	sinSlowMu    sync.RWMutex
	sinSlowCache = map[uint64]float64{}
	cosSlowMu    sync.RWMutex
	cosSlowCache = map[uint64]float64{}
)

// sinSlow is the correctly rounded sine of a finite x that is not 0.
func sinSlow(x float64) float64 {
	bits := math.Float64bits(x)
	sinSlowMu.RLock()
	if v, ok := sinSlowCache[bits]; ok {
		sinSlowMu.RUnlock()
		return v
	}
	sinSlowMu.RUnlock()
	v := bigCompute(func(prec uint) *big.Float {
		s, _ := bigSinCos(x, prec)
		return s
	})
	sinSlowMu.Lock()
	if len(sinSlowCache) < 1024 {
		sinSlowCache[bits] = v
	}
	sinSlowMu.Unlock()
	return v
}

// cosSlow is the correctly rounded cosine of a finite x that is not 0.
func cosSlow(x float64) float64 {
	bits := math.Float64bits(math.Abs(x))
	cosSlowMu.RLock()
	if v, ok := cosSlowCache[bits]; ok {
		cosSlowMu.RUnlock()
		return v
	}
	cosSlowMu.RUnlock()
	v := bigCompute(func(prec uint) *big.Float {
		_, c := bigSinCos(x, prec)
		return c
	})
	cosSlowMu.Lock()
	if len(cosSlowCache) < 1024 {
		cosSlowCache[bits] = v
	}
	cosSlowMu.Unlock()
	return v
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
