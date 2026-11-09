// Ported from Arm Optimized Routines v20.02 (math/sinf.c, math/cosf.c,
// math/sincosf.h, math/sincosf_data.c). Copyright (c) 2018, Arm Limited.
// MIT License: see LICENSE-ArmOptimizedRoutines.txt in this directory.
//
// Ported from the single-precision functions of fdlibm as NetBSD has them
// (lib/libm/src/k_tanf.c 1.7, s_tanf.c 1.8, s_atanf.c 1.9, e_atan2f.c 1.7:
// conversion to float by Ian Lance Taylor, Cygnus Support), under this
// notice:
//
//	Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
//	Developed at SunPro, a Sun Microsystems, Inc. business.
//	Permission to use, copy, modify, and distribute this
//	software is freely granted, provided that this notice
//	is preserved.

package browser

import "math"

// Skia and Blink compute arcs and points on a path with sinf, cosf, tanf
// and atan2f of the C library. These functions do not always return the
// nearest float32, and each C library has its own results. This file has
// the results of the C library of the reference Linux environment (the
// Playwright image for arm64).
//
// That library has the algorithms of the two sources named above: the Arm
// routines for sinf and cosf, and the float functions of fdlibm for tanf,
// atanf and atan2f. Its results differ from a plain evaluation of those
// sources in the points below. They come from the behavior of the compiled
// library in the image, and the test checks them:
// testdata/linux-libm.json.gz has results of the library
// (tools/reference/gen-browser-linux-libm.mjs), and every bit must agree.
//
//   - Fused operations. The compiler of the library joins a product and the
//     sum that follows it into one operation, which rounds one time. The
//     float32 polynomials of tanf and atanf depend on that. The calls of
//     fmaf and math.FMA below are at the places of the fused operations.
//   - tanf reduces its argument in float64, with the reduction of sinf, and
//     gives the kernel the float32 of the result and the float32 of the
//     rest. The kernel returns x for |x| < 2^-13, and 1 - 2x for an
//     argument that is nearer than 2^-13 to pi/4.
//   - atanf returns pi/2 for |x| >= 2^25.
//   - atan2f has pi as the nearest float32 and the rest of pi.

// fmaf is the fused multiply-add of float32 values: a*b + c with one
// rounding.
//
// The product is exact in float64. The sum is rounded to odd in float64
// (the last bit records that the sum was not exact), and that value is
// rounded to float32. With 53 bits for a result of 24 bits the two
// roundings together are one correct rounding.
func fmaf(a, b, c float32) float32 {
	p := float64(float64(a) * float64(b))
	cc := float64(c)
	s := p + cc
	// The error of the sum (TwoSum).
	bb := s - p
	e := (p - (s - bb)) + (cc - bb)
	if e != 0 && !math.IsInf(s, 0) {
		bits := math.Float64bits(s)
		if bits&1 == 0 {
			if (e > 0) == (s > 0) {
				// The exact sum is further from zero.
				bits |= 1
			} else {
				bits--
			}
			s = math.Float64frombits(bits)
		}
	}
	return float32(s)
}

func f32bits(x float32) uint32     { return math.Float32bits(x) }
func f32frombits(b uint32) float32 { return math.Float32frombits(b) }

// ---- sinf and cosf: Arm Optimized Routines ----

// abstop12 is the top 12 bits of a float32 without the sign bit.
func abstop12(x float32) uint32 { return (f32bits(x) >> 20) & 0x7ff }

// sincosTable is sincos_t: the constants of the sine and cosine
// polynomials.
type sincosTable struct {
	sign               [4]float64 // sign of the sine in the quadrants 0 to 3
	c0, c1, c2, c3, c4 float64    // cosine polynomial
	s1, s2, s3         float64    // sine polynomial
}

// sincosfTable is __sincosf_table. The second entry computes -cos(x), so
// that the negation is free.
var sincosfTable = [2]sincosTable{
	{
		sign: [4]float64{1, -1, -1, 1},
		c0:   0x1p0, c1: -0x1.ffffffd0c621cp-2, c2: 0x1.55553e1068f19p-5, c3: -0x1.6c087e89a359dp-10, c4: 0x1.99343027bf8c3p-16,
		s1: -0x1.555545995a603p-3, s2: 0x1.1107605230bc4p-7, s3: -0x1.994eb3774cf24p-13,
	},
	{
		sign: [4]float64{1, -1, -1, 1},
		c0:   -0x1p0, c1: 0x1.ffffffd0c621cp-2, c2: -0x1.55553e1068f19p-5, c3: 0x1.6c087e89a359dp-10, c4: -0x1.99343027bf8c3p-16,
		s1: -0x1.555545995a603p-3, s2: 0x1.1107605230bc4p-7, s3: -0x1.994eb3774cf24p-13,
	},
}

const (
	hpiInv = 0x1.45F306DC9C883p-1  // 2 / pi
	hpi    = 0x1.921FB54442D18p0   // pi / 2
	pi63   = 0x1.921FB54442D18p-62 // 2 pi * 2^-64
)

// invPio4 is __inv_pio4: 4/pi to 192 bits. Each entry adds 8 new bits.
var invPio4 = [24]uint32{
	0xa2, 0xa2f9, 0xa2f983, 0xa2f9836e,
	0xf9836e4e, 0x836e4e44, 0x6e4e4415, 0x4e441529,
	0x441529fc, 0x1529fc27, 0x29fc2757, 0xfc2757d1,
	0x2757d1f5, 0x57d1f534, 0xd1f534dd, 0xf534ddc0,
	0x34ddc0db, 0xddc0db62, 0xc0db6295, 0xdb629599,
	0x6295993c, 0x95993c43, 0x993c4390, 0x3c439041,
}

// reduceFast is reduce_fast: x modulo pi/2 as a value between -pi/4 and
// pi/4, and the quadrant. It is accurate for |x| <= 120. The library rounds
// the quotient to the nearest integer with a half away from zero, and
// computes x - n*hpi as one fused operation.
func reduceFast(x float64) (float64, int) {
	r := x * hpiInv
	n := math.Round(r)
	return math.FMA(-n, hpi, x), int(n)
}

// reduceLarge is reduce_large: the reduction of a large value with integer
// arithmetic. xi is the bits of the float32, whose sign is ignored.
func reduceLarge(xi uint32) (float64, int) {
	arr := invPio4[(xi>>26)&15:]
	shift := (xi >> 23) & 7
	xi = (xi & 0xffffff) | 0x800000
	xi <<= shift
	res0 := uint64(xi * arr[0])
	res1 := uint64(xi) * uint64(arr[4])
	res2 := uint64(xi) * uint64(arr[8])
	res0 = (res2 >> 32) | (res0 << 32)
	res0 += res1
	n := (res0 + (1 << 61)) >> 62
	res0 -= n << 62
	x := float64(int64(res0))
	return x * pi63, int(n)
}

// sinfPoly is sinf_poly: the sine of x (x2 is x*x) with the polynomial p.
// n is the quadrant: for an odd quadrant the cosine polynomial is used.
// Every sum of a product is a fused operation in the library.
func sinfPoly(x, x2 float64, p *sincosTable, n int) float32 {
	if n&1 == 0 {
		x3 := x * x2
		s1 := math.FMA(x2, p.s3, p.s2)
		x7 := x2 * x3
		s := math.FMA(x3, p.s1, x)
		return float32(math.FMA(s1, x7, s))
	}
	x4 := x2 * x2
	c2 := math.FMA(x2, p.c4, p.c3)
	c1 := math.FMA(x2, p.c1, p.c0)
	x6 := x2 * x4
	c := math.FMA(x4, p.c2, c1)
	return float32(math.FMA(c2, x6, c))
}

// sincosfReduce is the part that sinf and cosf share: the reduced argument
// with its sign, its square, the polynomial table and the quadrant. ok is
// false for an infinity and for a NaN.
func sincosfReduce(y float32) (x, x2 float64, p *sincosTable, n int, ok bool) {
	p = &sincosfTable[0]
	top := abstop12(y)
	if top < 0x42f { // |y| < 120
		x, n = reduceFast(float64(y))
		s := p.sign[n&3]
		if n&2 != 0 {
			p = &sincosfTable[1]
		}
		return x * s, x * x, p, n, true
	}
	if top < 0x7f8 {
		xi := f32bits(y)
		sign := int(xi >> 31)
		x, n = reduceLarge(xi)
		// The signs include the sign of y.
		s := p.sign[(n+sign)&3]
		if (n+sign)&2 != 0 {
			p = &sincosfTable[1]
		}
		return x * s, x * x, p, n, true
	}
	return 0, 0, p, 0, false
}

// linuxSinf is sinf of the C library of the reference Linux environment.
func linuxSinf(y float32) float32 {
	top := abstop12(y)
	if top < 0x3f4 { // |y| < pi/4
		if top < 0x398 { // |y| < 2^-12
			return y
		}
		x := float64(y)
		return sinfPoly(x, x*x, &sincosfTable[0], 0)
	}
	x, x2, p, n, ok := sincosfReduce(y)
	if !ok {
		return float32(math.NaN())
	}
	return sinfPoly(x, x2, p, n)
}

// linuxCosf is cosf of the C library of the reference Linux environment.
func linuxCosf(y float32) float32 {
	top := abstop12(y)
	if top < 0x3f4 { // |y| < pi/4
		if top < 0x398 { // |y| < 2^-12
			return 1
		}
		x := float64(y)
		return sinfPoly(x, x*x, &sincosfTable[0], 1)
	}
	x, x2, p, n, ok := sincosfReduce(y)
	if !ok {
		return float32(math.NaN())
	}
	return sinfPoly(x, x2, p, n^1)
}

// ---- tanf, atanf and atan2f: fdlibm ----

// tanT is the table T of k_tanf.c.
var tanT = [13]float32{
	f32frombits(0x3eaaaaab), // 3.3333334327e-01
	f32frombits(0x3e088889), // 1.3333334029e-01
	f32frombits(0x3d5d0dd1), // 5.3968254477e-02
	f32frombits(0x3cb327a4), // 2.1869488060e-02
	f32frombits(0x3c11371f), // 8.8632395491e-03
	f32frombits(0x3b6b6916), // 3.5920790397e-03
	f32frombits(0x3abede48), // 1.4562094584e-03
	f32frombits(0x3a1a26c8), // 5.8804126456e-04
	f32frombits(0x398137b9), // 2.4646313977e-04
	f32frombits(0x38a3f445), // 7.8179444245e-05
	f32frombits(0x3895c07a), // 7.1407252108e-05
	f32frombits(0xb79bae5f), // -1.8558637748e-05
	f32frombits(0x37d95384), // 2.5907305826e-05
}

// kernelTanf is __kernel_tanf: the tangent of x + y for |x| <= pi/4, where
// y is the tail of x. For iy = -1 it returns -1/tan(x + y).
func kernelTanf(x, y float32, iy int32) float32 {
	T := &tanT
	pio4 := f32frombits(0x3f490fda)
	pio4lo := f32frombits(0x33222168)
	hx := int32(f32bits(x))
	ix := hx & 0x7fffffff // high word of |x|
	if ix < 0x39000000 {  // |x| < 2^-13 in the reference library
		if int32(x) == 0 {
			if ix|(iy+1) == 0 {
				return 1 / float32(math.Abs(float64(x)))
			}
			if iy == 1 {
				return x
			}
			return -1 / x
		}
	}
	// sign is 1 - ((hx >> 30) & 2): -1 for a negative x.
	sign := float32(1 - ((hx >> 30) & 2))
	if ix >= 0x3f2ca140 { // |x| >= 0.6744
		if hx < 0 {
			x, y = -x, -y
		}
		z := pio4 - x
		w := pio4lo - y
		x = z + w
		y = 0
		if float32(math.Abs(float64(x))) < 0x1p-13 {
			// The reference library: tan(pi/4 - x) is 1 - 2x here.
			return float32(sign*float32(iy)) * (1 - float32(float32(2*iy)*x))
		}
	}
	z := x * x
	w := z * z
	// Break x^5*(T[1]+x^2*T[2]+...) into
	//   x^5(T[1]+x^4*T[3]+...+x^20*T[11]) +
	//   x^5(x^2*(T[2]+x^4*T[4]+...+x^22*[T12]))
	r := fmaf(w, T[11], T[9])
	r = fmaf(r, w, T[7])
	r = fmaf(r, w, T[5])
	r = fmaf(r, w, T[3])
	r = fmaf(r, w, T[1])
	v := fmaf(w, T[12], T[10])
	v = fmaf(v, w, T[8])
	v = fmaf(v, w, T[6])
	v = fmaf(v, w, T[4])
	v = fmaf(v, w, T[2])
	s := z * x
	// r = y + z*(s*(r+v)+y), where v = z*(...) joins the sum r+v.
	r = fmaf(fmaf(fmaf(v, z, r), s, y), z, y)
	// r += T[0]*s
	r = fmaf(s, T[0], r)
	w = x + r
	if ix >= 0x3f2ca140 {
		v := float32(iy)
		return sign * (v - 2*(x-(w*w/(w+v)-r)))
	}
	if iy == 1 {
		return w
	}
	// Compute -1.0/(x+r) accurately.
	z = f32frombits(f32bits(w) & 0xfffff000)
	v = r - (z - x) // z+v = r+x
	a := -1 / w
	t := f32frombits(f32bits(a) & 0xfffff000)
	s = fmaf(z, t, 1)
	return fmaf(fmaf(v, t, s), a, t)
}

// linuxTanf is tanf of the C library of the reference Linux environment.
func linuxTanf(x float32) float32 {
	ix := f32bits(x) & 0x7fffffff
	switch {
	case ix <= 0x3f490fda: // |x| ~<= pi/4
		return kernelTanf(x, 0, 1)
	case ix >= 0x7f800000: // tan(Inf or NaN) is NaN
		return float32(math.NaN())
	}
	// Argument reduction. The reference library reduces in float64 and
	// splits the result into a float32 and a tail.
	var dx float64
	var n int
	if abstop12(x) < 0x42f { // |x| < 120
		dx, n = reduceFast(float64(x))
	} else {
		xi := f32bits(x)
		dx, n = reduceLarge(xi)
		if xi>>31 != 0 {
			dx = -dx
		}
	}
	y0 := float32(dx)
	y1 := float32(dx - float64(y0))
	// 1 for an even n, -1 for an odd n.
	return kernelTanf(y0, y1, int32(1-((n&1)<<1)))
}

// linuxAtanf is atanf of the C library of the reference Linux environment.
func linuxAtanf(x float32) float32 {
	atanhi := [4]float32{
		f32frombits(0x3eed6338), // atan(0.5)hi
		f32frombits(0x3f490fda), // atan(1.0)hi
		f32frombits(0x3f7b985e), // atan(1.5)hi
		f32frombits(0x3fc90fda), // atan(inf)hi
	}
	atanlo := [4]float32{
		f32frombits(0x31ac3769), // atan(0.5)lo
		f32frombits(0x33222168), // atan(1.0)lo
		f32frombits(0x33140fb4), // atan(1.5)lo
		f32frombits(0x33a22168), // atan(inf)lo
	}
	aT := [11]float32{
		f32frombits(0x3eaaaaab), f32frombits(0xbe4ccccd), f32frombits(0x3e124925), f32frombits(0xbde38e38),
		f32frombits(0x3dba2e6e), f32frombits(0xbd9d8795), f32frombits(0x3d886b35), f32frombits(0xbd6ef16b),
		f32frombits(0x3d4bda59), f32frombits(0xbd15a221), f32frombits(0x3c8569d7),
	}
	hx := int32(f32bits(x))
	ix := hx & 0x7fffffff
	if ix >= 0x4c000000 { // |x| >= 2^25 in the reference library
		if ix > 0x7f800000 {
			return x + x // NaN
		}
		if hx > 0 {
			return atanhi[3] + atanlo[3]
		}
		return -atanhi[3] - atanlo[3]
	}
	id := -1
	if ix < 0x3ee00000 { // |x| < 0.4375
		if ix < 0x31000000 { // |x| < 2^-29
			return x
		}
	} else {
		x = float32(math.Abs(float64(x)))
		switch {
		case ix < 0x3f300000: // 7/16 <= |x| < 11/16
			id = 0
			x = fmaf(x, 2, -1) / (x + 2)
		case ix < 0x3f980000: // 11/16 <= |x| < 19/16
			id = 1
			x = (x - 1) / (x + 1)
		case ix < 0x401c0000: // |x| < 2.4375
			id = 2
			x = (x - 1.5) / fmaf(x, 1.5, 1)
		default: // 2.4375 <= |x| < 2^25
			id = 3
			x = -1 / x
		}
	}
	// End of the argument reduction.
	z := x * x
	w := z * z
	// Break the sum from i=0 to 10 of aT[i]z**(i+1) into an odd and an
	// even polynomial.
	even := fmaf(w, aT[10], aT[8])
	even = fmaf(even, w, aT[6])
	even = fmaf(even, w, aT[4])
	even = fmaf(even, w, aT[2])
	even = fmaf(even, w, aT[0])
	odd := fmaf(w, aT[9], aT[7])
	odd = fmaf(odd, w, aT[5])
	odd = fmaf(odd, w, aT[3])
	odd = fmaf(odd, w, aT[1])
	s2 := odd * w
	// x*(s1+s2), where s1 = z*(...) joins the sum. The product by x and
	// the subtraction that follows are two operations.
	t := float32(x * fmaf(even, z, s2))
	if id < 0 {
		return x - t
	}
	r := atanhi[id] - ((t - atanlo[id]) - x)
	if hx < 0 {
		return -r
	}
	return r
}

// linuxAtan2f is atan2f of the C library of the reference Linux
// environment.
func linuxAtan2f(y, x float32) float32 {
	const tiny = float32(1.0e-30)
	piO4 := f32frombits(0x3f490fdb)
	piO2 := f32frombits(0x3fc90fdb)
	// The reference library: the nearest float32 to pi, and the rest.
	pi := f32frombits(0x40490fdb)
	piLo := f32frombits(0xb3bbbd2e)
	hx := int32(f32bits(x))
	ix := hx & 0x7fffffff
	hy := int32(f32bits(y))
	iy := hy & 0x7fffffff
	if ix > 0x7f800000 || iy > 0x7f800000 { // x or y is NaN
		return x + y
	}
	if hx == 0x3f800000 { // x = 1.0
		return linuxAtanf(y)
	}
	m := ((hy >> 31) & 1) | ((hx >> 30) & 2) // 2*sign(x)+sign(y)
	// When y = 0.
	if iy == 0 {
		switch m {
		case 0, 1:
			return y // atan(+-0,+anything)=+-0
		case 2:
			return pi + tiny // atan(+0,-anything) = pi
		default:
			return -pi - tiny // atan(-0,-anything) =-pi
		}
	}
	// When x = 0.
	if ix == 0 {
		if hy < 0 {
			return -piO2 - tiny
		}
		return piO2 + tiny
	}
	// When x is INF.
	if ix == 0x7f800000 {
		if iy == 0x7f800000 {
			switch m {
			case 0:
				return piO4 + tiny // atan(+INF,+INF)
			case 1:
				return -piO4 - tiny // atan(-INF,+INF)
			case 2:
				return float32(3*piO4) + tiny // atan(+INF,-INF)
			default:
				return float32(-3*piO4) - tiny // atan(-INF,-INF)
			}
		}
		switch m {
		case 0:
			return 0 // atan(+...,+INF)
		case 1:
			return float32(math.Copysign(0, -1)) // atan(-...,+INF)
		case 2:
			return pi + tiny // atan(+...,-INF)
		default:
			return -pi - tiny // atan(-...,-INF)
		}
	}
	// When y is INF.
	if iy == 0x7f800000 {
		if hy < 0 {
			return -piO2 - tiny
		}
		return piO2 + tiny
	}
	// Compute y/x.
	k := (iy - ix) >> 23
	var z float32
	switch {
	case k > 60: // |y/x| > 2**60
		z = piO2 + float32(0.5*piLo)
	case hx < 0 && k < -60: // |y|/x < -2**60
		z = 0
	default: // safe to do y/x
		z = linuxAtanf(float32(math.Abs(float64(y / x))))
	}
	switch m {
	case 0:
		return z // atan(+,+)
	case 1:
		return -z // atan(-,+)
	case 2:
		return pi - (z - piLo) // atan(+,-)
	default:
		return (z - piLo) - pi // atan(-,-)
	}
}
