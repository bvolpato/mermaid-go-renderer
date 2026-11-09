// Ported from v8/src/base/ieee754.cc at the tag 14.6.202 (the fdlibm
// functions __ieee754_rem_pio2, __kernel_rem_pio2, __kernel_sin,
// __kernel_cos, __kernel_tan, sin, cos, tan, atan and asin).
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunSoft, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// The original source code covered by the above license above has been
// modified significantly by Google Inc.
// Copyright 2016 the V8 project authors. All rights reserved.

// This file is test support. The golden generator gen-rough.mjs replaces
// Math.sin, Math.cos, Math.tan, Math.atan and Math.asin with these functions
// in JavaScript (tools/reference/rough-fdlibm.mjs), and TestGolden gives the
// port the Go versions below. Both sides then get the same result for each
// call, on every platform.
//
// These are not the functions of the reference browser. V8 used the fdlibm
// functions until version 14.8. Chromium 156 has V8 15, which takes the
// transcendental functions from LLVM libc. Node.js 26 has the fdlibm
// functions, and its build for darwin/arm64 has fused multiply-adds in them.
//
// Each product that is an operand of an addition or a subtraction has an
// explicit float64 conversion, so the compiler does not fuse the two
// operations.

package rough

import "math"

func highWord(x float64) int32 { return int32(math.Float64bits(x) >> 32) }

func lowWord(x float64) uint32 { return uint32(math.Float64bits(x)) }

func setLowWord(x float64, lo uint32) float64 {
	return math.Float64frombits(math.Float64bits(x)&0xffffffff00000000 | uint64(lo))
}

func setHighWord(x float64, hi int32) float64 {
	return math.Float64frombits(math.Float64bits(x)&0x00000000ffffffff | uint64(uint32(hi))<<32)
}

func insertWords(hi int32, lo uint32) float64 {
	return math.Float64frombits(uint64(uint32(hi))<<32 | uint64(lo))
}

// Table of constants for 2/pi, 396 Hex digits (476 decimal) of 2/pi
var twoOverPi = [...]int32{
	0xA2F983, 0x6E4E44, 0x1529FC, 0x2757D1, 0xF534DD, 0xC0DB62, 0x95993C,
	0x439041, 0xFE5163, 0xABDEBB, 0xC561B7, 0x246E3A, 0x424DD2, 0xE00649,
	0x2EEA09, 0xD1921C, 0xFE1DEB, 0x1CB129, 0xA73EE8, 0x8235F5, 0x2EBB44,
	0x84E99C, 0x7026B4, 0x5F7E41, 0x3991D6, 0x398353, 0x39F49C, 0x845F8B,
	0xBDF928, 0x3B1FF8, 0x97FFDE, 0x05980F, 0xEF2F11, 0x8B5A0A, 0x6D1F6D,
	0x367ECF, 0x27CB09, 0xB74F46, 0x3F669E, 0x5FEA2D, 0x7527BA, 0xC7EBE5,
	0xF17B3D, 0x0739F7, 0x8A5292, 0xEA6BFB, 0x5FB11F, 0x8D5D08, 0x560330,
	0x46FC7B, 0x6BABF0, 0xCFBC20, 0x9AF436, 0x1DA9E3, 0x91615E, 0xE61B08,
	0x659985, 0x5F14A0, 0x68408D, 0xFFD880, 0x4D7327, 0x310606, 0x1556CA,
	0x73A8C9, 0x60E27B, 0xC08C6B,
}

var npio2Hw = [...]int32{
	0x3FF921FB, 0x400921FB, 0x4012D97C, 0x401921FB, 0x401F6A7A, 0x4022D97C,
	0x4025FDBB, 0x402921FB, 0x402C463A, 0x402F6A7A, 0x4031475C, 0x4032D97C,
	0x40346B9C, 0x4035FDBB, 0x40378FDB, 0x403921FB, 0x403AB41B, 0x403C463A,
	0x403DD85A, 0x403F6A7A, 0x40407E4C, 0x4041475C, 0x4042106C, 0x4042D97C,
	0x4043A28C, 0x40446B9C, 0x404534AC, 0x4045FDBB, 0x4046C6CB, 0x40478FDB,
	0x404858EB, 0x404921FB,
}

// remPio2 is __ieee754_rem_pio2: it returns the remainder of x rem pi/2 in
// y0+y1 and the quadrant n.
func remPio2(x float64) (n int32, y0, y1 float64) {
	const (
		half    = 5.00000000000000000000e-01 /* 0x3FE00000, 0x00000000 */
		two24   = 1.67772160000000000000e+07 /* 0x41700000, 0x00000000 */
		invpio2 = 6.36619772367581382433e-01 /* 0x3FE45F30, 0x6DC9C883 */
		pio2_1  = 1.57079632673412561417e+00 /* 0x3FF921FB, 0x54400000 */
		pio2_1t = 6.07710050650619224932e-11 /* 0x3DD0B461, 0x1A626331 */
		pio2_2  = 6.07710050630396597660e-11 /* 0x3DD0B461, 0x1A600000 */
		pio2_2t = 2.02226624879595063154e-21 /* 0x3BA3198A, 0x2E037073 */
		pio2_3  = 2.02226624871116645580e-21 /* 0x3BA3198A, 0x2E000000 */
		pio2_3t = 8.47842766036889956997e-32 /* 0x397B839A, 0x252049C1 */
	)
	var z, w, t, r, fn float64
	hx := highWord(x) /* high word of x */
	ix := hx & 0x7FFFFFFF
	if ix <= 0x3FE921FB { /* |x| ~<= pi/4 , no need for reduction */
		return 0, x, 0
	}
	if ix < 0x4002D97C { /* |x| < 3pi/4, special case with n=+-1 */
		if hx > 0 {
			z = x - pio2_1
			if ix != 0x3FF921FB { /* 33+53 bit pi is good enough */
				y0 = z - pio2_1t
				y1 = (z - y0) - pio2_1t
			} else { /* near pi/2, use 33+33+53 bit pi */
				z -= pio2_2
				y0 = z - pio2_2t
				y1 = (z - y0) - pio2_2t
			}
			return 1, y0, y1
		}
		/* negative x */
		z = x + pio2_1
		if ix != 0x3FF921FB { /* 33+53 bit pi is good enough */
			y0 = z + pio2_1t
			y1 = (z - y0) + pio2_1t
		} else { /* near pi/2, use 33+33+53 bit pi */
			z += pio2_2
			y0 = z + pio2_2t
			y1 = (z - y0) + pio2_2t
		}
		return -1, y0, y1
	}
	if ix <= 0x413921FB { /* |x| ~<= 2^19*(pi/2), medium size */
		t = math.Abs(x)
		n = int32(float64(t*invpio2) + half)
		fn = float64(n)
		r = t - float64(fn*pio2_1)
		w = float64(fn * pio2_1t) /* 1st round good to 85 bit */
		if n < 32 && ix != npio2Hw[n-1] {
			y0 = r - w /* quick check no cancellation */
		} else {
			j := ix >> 20
			y0 = r - w
			high := uint32(highWord(y0))
			i := j - int32((high>>20)&0x7FF)
			if i > 16 { /* 2nd iteration needed, good to 118 */
				t = r
				w = float64(fn * pio2_2)
				r = t - w
				w = float64(fn*pio2_2t) - ((t - r) - w)
				y0 = r - w
				high = uint32(highWord(y0))
				i = j - int32((high>>20)&0x7FF)
				if i > 49 { /* 3rd iteration need, 151 bits acc */
					t = r /* will cover all possible cases */
					w = float64(fn * pio2_3)
					r = t - w
					w = float64(fn*pio2_3t) - ((t - r) - w)
					y0 = r - w
				}
			}
		}
		y1 = (r - y0) - w
		if hx < 0 {
			return -n, -y0, -y1
		}
		return n, y0, y1
	}
	/*
	 * all other (large) arguments
	 */
	if ix >= 0x7FF00000 { /* x is inf or NaN */
		y0 = x - x
		return 0, y0, y0
	}
	/* set z = scalbn(|x|,ilogb(x)-23) */
	z = setLowWord(0, lowWord(x))
	e0 := (ix >> 20) - 1046 /* e0 = ilogb(z)-23; */
	z = setHighWord(z, ix-int32(uint32(e0)<<20))
	var tx [3]float64
	for i := 0; i < 2; i++ {
		tx[i] = float64(int32(z))
		z = float64((z - tx[i]) * two24)
	}
	tx[2] = z
	nx := 3
	for tx[nx-1] == 0 {
		nx-- /* skip zero term */
	}
	var y [3]float64
	n = kernelRemPio2(tx[:], y[:], int(e0), nx, 2)
	if hx < 0 {
		return -n, -y[0], -y[1]
	}
	return n, y[0], y[1]
}

var initJk = [...]int{2, 3, 4, 6} /* initial value for jk */

var pio2Table = [...]float64{
	1.57079625129699707031e+00, /* 0x3FF921FB, 0x40000000 */
	7.54978941586159635335e-08, /* 0x3E74442D, 0x00000000 */
	5.39030252995776476554e-15, /* 0x3CF84698, 0x80000000 */
	3.28200341580791294123e-22, /* 0x3B78CC51, 0x60000000 */
	1.27065575308067607349e-29, /* 0x39F01B83, 0x80000000 */
	1.22933308981111328932e-36, /* 0x387A2520, 0x40000000 */
	2.73370053816464559624e-44, /* 0x36E38222, 0x80000000 */
	2.16741683877804819444e-51, /* 0x3569F31D, 0x00000000 */
}

// kernelRemPio2 is __kernel_rem_pio2 with ipio2 = two_over_pi.
func kernelRemPio2(x, y []float64, e0, nx, prec int) int32 {
	const (
		two24  = 1.67772160000000000000e+07 /* 0x41700000, 0x00000000 */
		twon24 = 5.96046447753906250000e-08 /* 0x3E700000, 0x00000000 */
	)
	var (
		jz, jx, jv, jp, jk, carry, n, i, j, k, m, q0, ih int
		iq                                               [20]int32
		z, fw                                            float64
		f, fq, q                                         [20]float64
	)

	/* initialize jk*/
	jk = initJk[prec]
	jp = jk

	/* determine jx,jv,q0, note that 3>q0 */
	jx = nx - 1
	jv = (e0 - 3) / 24
	if jv < 0 {
		jv = 0
	}
	q0 = e0 - 24*(jv+1)

	/* set up f[0] to f[jx+jk] where f[jx+jk] = ipio2[jv+jk] */
	j = jv - jx
	m = jx + jk
	for i = 0; i <= m; i, j = i+1, j+1 {
		if j < 0 {
			f[i] = 0
		} else {
			f[i] = float64(twoOverPi[j])
		}
	}

	/* compute q[0],q[1],...q[jk] */
	for i = 0; i <= jk; i++ {
		fw = 0.0
		for j = 0; j <= jx; j++ {
			fw += float64(x[j] * f[jx+i-j])
		}
		q[i] = fw
	}

	jz = jk
recompute:
	/* distill q[] into iq[] reversingly */
	for i, j, z = 0, jz, q[jz]; j > 0; i, j = i+1, j-1 {
		fw = float64(int32(float64(twon24 * z)))
		iq[i] = int32(z - float64(two24*fw))
		z = q[j-1] + fw
	}

	/* compute n */
	z = math.Ldexp(z, q0)                   /* actual value of z */
	z -= float64(8.0 * math.Floor(z*0.125)) /* trim off integer >= 8 */
	n = int(int32(z))
	z -= float64(n)
	ih = 0
	if q0 > 0 { /* need iq[jz-1] to determine n */
		i = int(iq[jz-1] >> (24 - q0))
		n += i
		iq[jz-1] -= int32(i << (24 - q0))
		ih = int(iq[jz-1] >> (23 - q0))
	} else if q0 == 0 {
		ih = int(iq[jz-1] >> 23)
	} else if z >= 0.5 {
		ih = 2
	}

	if ih > 0 { /* q > 0.5 */
		n += 1
		carry = 0
		for i = 0; i < jz; i++ { /* compute 1-q */
			j = int(iq[i])
			if carry == 0 {
				if j != 0 {
					carry = 1
					iq[i] = int32(0x1000000 - j)
				}
			} else {
				iq[i] = int32(0xFFFFFF - j)
			}
		}
		if q0 > 0 { /* rare case: chance is 1 in 12 */
			switch q0 {
			case 1:
				iq[jz-1] &= 0x7FFFFF
			case 2:
				iq[jz-1] &= 0x3FFFFF
			}
		}
		if ih == 2 {
			z = 1 - z
			if carry != 0 {
				z -= math.Ldexp(1, q0)
			}
		}
	}

	/* check if recomputation is needed */
	if z == 0 {
		j = 0
		for i = jz - 1; i >= jk; i-- {
			j |= int(iq[i])
		}
		if j == 0 { /* need recomputation */
			for k = 1; jk >= k && iq[jk-k] == 0; k++ {
				/* k = no. of terms needed */
			}

			for i = jz + 1; i <= jz+k; i++ { /* add q[jz+1] to q[jz+k] */
				f[jx+i] = float64(twoOverPi[jv+i])
				fw = 0.0
				for j = 0; j <= jx; j++ {
					fw += float64(x[j] * f[jx+i-j])
				}
				q[i] = fw
			}
			jz += k
			goto recompute
		}
	}

	/* chop off zero terms */
	if z == 0.0 {
		jz -= 1
		q0 -= 24
		for iq[jz] == 0 {
			jz--
			q0 -= 24
		}
	} else { /* break z into 24-bit if necessary */
		z = math.Ldexp(z, -q0)
		if z >= two24 {
			fw = float64(int32(float64(twon24 * z)))
			iq[jz] = int32(z - float64(two24*fw))
			jz += 1
			q0 += 24
			iq[jz] = int32(fw)
		} else {
			iq[jz] = int32(z)
		}
	}

	/* convert integer "bit" chunk to floating-point value */
	fw = math.Ldexp(1, q0)
	for i = jz; i >= 0; i-- {
		q[i] = float64(fw * float64(iq[i]))
		fw *= twon24
	}

	/* compute PIo2[0,...,jp]*q[jz,...,0] */
	for i = jz; i >= 0; i-- {
		fw = 0.0
		for k = 0; k <= jp && k <= jz-i; k++ {
			fw += float64(pio2Table[k] * q[i+k])
		}
		fq[jz-i] = fw
	}

	/* compress fq[] into y[] */
	switch prec {
	case 0:
		fw = 0.0
		for i = jz; i >= 0; i-- {
			fw += fq[i]
		}
		if ih == 0 {
			y[0] = fw
		} else {
			y[0] = -fw
		}
	case 1, 2:
		fw = 0.0
		for i = jz; i >= 0; i-- {
			fw += fq[i]
		}
		if ih == 0 {
			y[0] = fw
		} else {
			y[0] = -fw
		}
		fw = fq[0] - fw
		for i = 1; i <= jz; i++ {
			fw += fq[i]
		}
		if ih == 0 {
			y[1] = fw
		} else {
			y[1] = -fw
		}
	case 3: /* painful */
		for i = jz; i > 0; i-- {
			fw = fq[i-1] + fq[i]
			fq[i] += fq[i-1] - fw
			fq[i-1] = fw
		}
		for i = jz; i > 1; i-- {
			fw = fq[i-1] + fq[i]
			fq[i] += fq[i-1] - fw
			fq[i-1] = fw
		}
		fw = 0.0
		for i = jz; i >= 2; i-- {
			fw += fq[i]
		}
		if ih == 0 {
			y[0] = fq[0]
			y[1] = fq[1]
			y[2] = fw
		} else {
			y[0] = -fq[0]
			y[1] = -fq[1]
			y[2] = -fw
		}
	}
	return int32(n & 7)
}

// kernelCos is __kernel_cos: cos on [-pi/4, pi/4]. y is the tail of x.
func kernelCos(x, y float64) float64 {
	const (
		one = 1.00000000000000000000e+00  /* 0x3FF00000, 0x00000000 */
		C1  = 4.16666666666666019037e-02  /* 0x3FA55555, 0x5555554C */
		C2  = -1.38888888888741095749e-03 /* 0xBF56C16C, 0x16C15177 */
		C3  = 2.48015872894767294178e-05  /* 0x3EFA01A0, 0x19CB1590 */
		C4  = -2.75573143513906633035e-07 /* 0xBE927E4F, 0x809C52AD */
		C5  = 2.08757232129817482790e-09  /* 0x3E21EE9E, 0xBDB4B1C4 */
		C6  = -1.13596475577881948265e-11 /* 0xBDA8FAE9, 0xBE8838D4 */
	)
	ix := highWord(x) & 0x7FFFFFFF /* ix = |x|'s high word*/
	if ix < 0x3E400000 {           /* if x < 2**27 */
		if int32(x) == 0 {
			return one /* generate inexact */
		}
	}
	z := float64(x * x)
	r := float64(z * (C1 + float64(z*(C2+float64(z*(C3+float64(z*(C4+float64(z*(C5+float64(z*C6)))))))))))
	if ix < 0x3FD33333 { /* if |x| < 0.3 */
		return one - (float64(0.5*z) - (float64(z*r) - float64(x*y)))
	}
	var qx float64
	if ix > 0x3FE90000 { /* x > 0.78125 */
		qx = 0.28125
	} else {
		qx = insertWords(ix-0x00200000, 0) /* x/4 */
	}
	iz := float64(0.5*z) - qx
	a := one - qx
	return a - (iz - (float64(z*r) - float64(x*y)))
}

// kernelSin is __kernel_sin: sin on [-pi/4, pi/4]. y is the tail of x, and
// iy is 0 when y is 0.
func kernelSin(x, y float64, iy int) float64 {
	const (
		half = 5.00000000000000000000e-01  /* 0x3FE00000, 0x00000000 */
		S1   = -1.66666666666666324348e-01 /* 0xBFC55555, 0x55555549 */
		S2   = 8.33333333332248946124e-03  /* 0x3F811111, 0x1110F8A6 */
		S3   = -1.98412698298579493134e-04 /* 0xBF2A01A0, 0x19C161D5 */
		S4   = 2.75573137070700676789e-06  /* 0x3EC71DE3, 0x57B1FE7D */
		S5   = -2.50507602534068634195e-08 /* 0xBE5AE5E6, 0x8A2B9CEB */
		S6   = 1.58969099521155010221e-10  /* 0x3DE5D93A, 0x5ACFD57C */
	)
	ix := highWord(x) & 0x7FFFFFFF /* high word of x */
	if ix < 0x3E400000 {           /* |x| < 2**-27 */
		if int32(x) == 0 {
			return x
		}
	} /* generate inexact */
	z := float64(x * x)
	v := float64(z * x)
	r := S2 + float64(z*(S3+float64(z*(S4+float64(z*(S5+float64(z*S6)))))))
	if iy == 0 {
		return x + float64(v*(S1+float64(z*r)))
	}
	return x - ((float64(z*(float64(half*y)-float64(v*r))) - y) - float64(v*S1))
}

var tanT = [...]float64{
	3.33333333333334091986e-01,  /* 3FD55555, 55555563 */
	1.33333333333201242699e-01,  /* 3FC11111, 1110FE7A */
	5.39682539762260521377e-02,  /* 3FABA1BA, 1BB341FE */
	2.18694882948595424599e-02,  /* 3F9664F4, 8406D637 */
	8.86323982359930005737e-03,  /* 3F8226E3, E96E8493 */
	3.59207910759131235356e-03,  /* 3F6D6D22, C9560328 */
	1.45620945432529025516e-03,  /* 3F57DBC8, FEE08315 */
	5.88041240820264096874e-04,  /* 3F4344D8, F2F26501 */
	2.46463134818469906812e-04,  /* 3F3026F7, 1A8D1068 */
	7.81794442939557092300e-05,  /* 3F147E88, A03792A6 */
	7.14072491382608190305e-05,  /* 3F12B80F, 32F0A7E9 */
	-1.85586374855275456654e-05, /* BEF375CB, DB605373 */
	2.59073051863633712884e-05,  /* 3EFB2A70, 74BF7AD4 */
}

// kernelTan is __kernel_tan: tan on [-pi/4, pi/4]. y is the tail of x. It
// returns tan when iy is 1, and -1/tan when iy is -1.
func kernelTan(x, y float64, iy int) float64 {
	const (
		one    = 1.00000000000000000000e+00 /* 3FF00000, 00000000 */
		pio4   = 7.85398163397448278999e-01 /* 3FE921FB, 54442D18 */
		pio4lo = 3.06161699786838301793e-17 /* 3C81A626, 33145C07 */
	)
	T := &tanT
	var z, r, v, w, s float64

	hx := highWord(x)     /* high word of x */
	ix := hx & 0x7FFFFFFF /* high word of |x| */
	if ix < 0x3E300000 {  /* x < 2**-28 */
		if int32(x) == 0 { /* generate inexact */
			low := lowWord(x)
			if ((uint32(ix) | low) | uint32(iy+1)) == 0 {
				return one / math.Abs(x)
			}
			if iy == 1 {
				return x
			}
			/* compute -1 / (x+y) carefully */
			w = x + y
			z = setLowWord(w, 0)
			v = y - (z - x)
			a := -one / w
			t := setLowWord(a, 0)
			s = one + float64(t*z)
			return t + float64(a*(s+float64(t*v)))
		}
	}
	if ix >= 0x3FE59428 { /* |x| >= 0.6744 */
		if hx < 0 {
			x = -x
			y = -y
		}
		z = pio4 - x
		w = pio4lo - y
		x = z + w
		y = 0.0
	}
	z = float64(x * x)
	w = float64(z * z)
	/*
	 * Break x^5*(T[1]+x^2*T[2]+...) into
	 * x^5(T[1]+x^4*T[3]+...+x^20*T[11]) +
	 * x^5(x^2*(T[2]+x^4*T[4]+...+x^22*[T12]))
	 */
	r = T[1] + float64(w*(T[3]+float64(w*(T[5]+float64(w*(T[7]+float64(w*(T[9]+float64(w*T[11])))))))))
	v = float64(z * (T[2] + float64(w*(T[4]+float64(w*(T[6]+float64(w*(T[8]+float64(w*(T[10]+float64(w*T[12])))))))))))
	s = float64(z * x)
	r = y + float64(z*(float64(s*(r+v))+y))
	r += float64(T[0] * s)
	w = x + r
	if ix >= 0x3FE59428 {
		v = float64(iy)
		return float64(float64(1-((hx>>30)&2)) * (v - float64(2.0*(x-(float64(w*w)/(w+v)-r)))))
	}
	if iy == 1 {
		return w
	}
	/*
	 * if allow error up to 2 ulp, simply return
	 * -1.0 / (x+r) here
	 */
	/* compute -1.0 / (x+r) accurately */
	z = setLowWord(w, 0)
	v = r - (z - x) /* z+v = r+x */
	a := -1.0 / w   /* a = -1.0/w */
	t := setLowWord(a, 0)
	s = 1.0 + float64(t*z)
	return t + float64(a*(s+float64(t*v)))
}

// fdlibmSin is the function sin of fdlibm.
func fdlibmSin(x float64) float64 {
	/* High word of x. */
	ix := highWord(x)

	/* |x| ~< pi/4 */
	ix &= 0x7FFFFFFF
	if ix <= 0x3FE921FB {
		return kernelSin(x, 0, 0)
	} else if ix >= 0x7FF00000 {
		/* sin(Inf or NaN) is NaN */
		return x - x
	}
	/* argument reduction needed */
	n, y0, y1 := remPio2(x)
	switch n & 3 {
	case 0:
		return kernelSin(y0, y1, 1)
	case 1:
		return kernelCos(y0, y1)
	case 2:
		return -kernelSin(y0, y1, 1)
	default:
		return -kernelCos(y0, y1)
	}
}

// fdlibmCos is the function cos of fdlibm.
func fdlibmCos(x float64) float64 {
	/* High word of x. */
	ix := highWord(x)

	/* |x| ~< pi/4 */
	ix &= 0x7FFFFFFF
	if ix <= 0x3FE921FB {
		return kernelCos(x, 0)
	} else if ix >= 0x7FF00000 {
		/* cos(Inf or NaN) is NaN */
		return x - x
	}
	/* argument reduction needed */
	n, y0, y1 := remPio2(x)
	switch n & 3 {
	case 0:
		return kernelCos(y0, y1)
	case 1:
		return -kernelSin(y0, y1, 1)
	case 2:
		return -kernelCos(y0, y1)
	default:
		return kernelSin(y0, y1, 1)
	}
}

// fdlibmTan is the function tan of fdlibm.
func fdlibmTan(x float64) float64 {
	/* High word of x. */
	ix := highWord(x)

	/* |x| ~< pi/4 */
	ix &= 0x7FFFFFFF
	if ix <= 0x3FE921FB {
		return kernelTan(x, 0, 1)
	} else if ix >= 0x7FF00000 {
		/* tan(Inf or NaN) is NaN */
		return x - x /* NaN */
	}
	/* argument reduction needed */
	n, y0, y1 := remPio2(x)
	/* 1 -> n even, -1 -> n odd */
	return kernelTan(y0, y1, int(1-((n&1)<<1)))
}

var atanhi = [...]float64{
	4.63647609000806093515e-01, /* atan(0.5)hi 0x3FDDAC67, 0x0561BB4F */
	7.85398163397448278999e-01, /* atan(1.0)hi 0x3FE921FB, 0x54442D18 */
	9.82793723247329054082e-01, /* atan(1.5)hi 0x3FEF730B, 0xD281F69B */
	1.57079632679489655800e+00, /* atan(inf)hi 0x3FF921FB, 0x54442D18 */
}

var atanlo = [...]float64{
	2.26987774529616870924e-17, /* atan(0.5)lo 0x3C7A2B7F, 0x222F65E2 */
	3.06161699786838301793e-17, /* atan(1.0)lo 0x3C81A626, 0x33145C07 */
	1.39033110312309984516e-17, /* atan(1.5)lo 0x3C700788, 0x7AF0CBBD */
	6.12323399573676603587e-17, /* atan(inf)lo 0x3C91A626, 0x33145C07 */
}

var aT = [...]float64{
	3.33333333333329318027e-01,  /* 0x3FD55555, 0x5555550D */
	-1.99999999998764832476e-01, /* 0xBFC99999, 0x9998EBC4 */
	1.42857142725034663711e-01,  /* 0x3FC24924, 0x920083FF */
	-1.11111104054623557880e-01, /* 0xBFBC71C6, 0xFE231671 */
	9.09088713343650656196e-02,  /* 0x3FB745CD, 0xC54C206E */
	-7.69187620504482999495e-02, /* 0xBFB3B0F2, 0xAF749A6D */
	6.66107313738753120669e-02,  /* 0x3FB10D66, 0xA0D03D51 */
	-5.83357013379057348645e-02, /* 0xBFADDE2D, 0x52DEFD9A */
	4.97687799461593236017e-02,  /* 0x3FA97B4B, 0x24760DEB */
	-3.65315727442169155270e-02, /* 0xBFA2B444, 0x2C6A6C2F */
	1.62858201153657823623e-02,  /* 0x3F90AD3A, 0xE322DA11 */
}

// fdlibmAtan is the function atan of fdlibm.
func fdlibmAtan(x float64) float64 {
	const one, huge = 1.0, 1.0e300
	var w, s1, s2, z float64
	var id int

	hx := highWord(x)
	ix := hx & 0x7FFFFFFF
	if ix >= 0x44100000 { /* if |x| >= 2^66 */
		low := lowWord(x)
		if ix > 0x7FF00000 || (ix == 0x7FF00000 && (low != 0)) {
			return x + x /* NaN */
		}
		if hx > 0 {
			return atanhi[3] + atanlo[3]
		}
		return -atanhi[3] - atanlo[3]
	}
	if ix < 0x3FDC0000 { /* |x| < 0.4375 */
		if ix < 0x3E400000 { /* |x| < 2^-27 */
			if huge+x > one {
				return x /* raise inexact */
			}
		}
		id = -1
	} else {
		x = math.Abs(x)
		if ix < 0x3FF30000 { /* |x| < 1.1875 */
			if ix < 0x3FE60000 { /* 7/16 <=|x|<11/16 */
				id = 0
				x = (float64(2.0*x) - one) / (2.0 + x)
			} else { /* 11/16<=|x|< 19/16 */
				id = 1
				x = (x - one) / (x + one)
			}
		} else {
			if ix < 0x40038000 { /* |x| < 2.4375 */
				id = 2
				x = (x - 1.5) / (one + float64(1.5*x))
			} else { /* 2.4375 <= |x| < 2^66 */
				id = 3
				x = -1.0 / x
			}
		}
	}
	/* end of argument reduction */
	z = float64(x * x)
	w = float64(z * z)
	/* break sum from i=0 to 10 aT[i]z**(i+1) into odd and even poly */
	s1 = float64(z * (aT[0] + float64(w*(aT[2]+float64(w*(aT[4]+float64(w*(aT[6]+float64(w*(aT[8]+float64(w*aT[10])))))))))))
	s2 = float64(w * (aT[1] + float64(w*(aT[3]+float64(w*(aT[5]+float64(w*(aT[7]+float64(w*aT[9])))))))))
	if id < 0 {
		return x - float64(x*(s1+s2))
	}
	z = atanhi[id] - ((float64(x*(s1+s2)) - atanlo[id]) - x)
	if hx < 0 {
		return -z
	}
	return z
}

// fdlibmAsin is the function asin of fdlibm.
func fdlibmAsin(x float64) float64 {
	const (
		one     = 1.00000000000000000000e+00 /* 0x3FF00000, 0x00000000 */
		huge    = 1.000e+300
		pio2_hi = 1.57079632679489655800e+00 /* 0x3FF921FB, 0x54442D18 */
		pio2_lo = 6.12323399573676603587e-17 /* 0x3C91A626, 0x33145C07 */
		pio4_hi = 7.85398163397448278999e-01 /* 0x3FE921FB, 0x54442D18 */
		/* coefficient for R(x^2) */
		pS0 = 1.66666666666666657415e-01  /* 0x3FC55555, 0x55555555 */
		pS1 = -3.25565818622400915405e-01 /* 0xBFD4D612, 0x03EB6F7D */
		pS2 = 2.01212532134862925881e-01  /* 0x3FC9C155, 0x0E884455 */
		pS3 = -4.00555345006794114027e-02 /* 0xBFA48228, 0xB5688F3B */
		pS4 = 7.91534994289814532176e-04  /* 0x3F49EFE0, 0x7501B288 */
		pS5 = 3.47933107596021167570e-05  /* 0x3F023DE1, 0x0DFDF709 */
		qS1 = -2.40339491173441421878e+00 /* 0xC0033A27, 0x1C8A2D4B */
		qS2 = 2.02094576023350569471e+00  /* 0x40002AE5, 0x9C598AC8 */
		qS3 = -6.88283971605453293030e-01 /* 0xBFE6066C, 0x1B8D0159 */
		qS4 = 7.70381505559019352791e-02  /* 0x3FB3B8C5, 0xB12E9282 */
	)
	var t, w, p, q, c, r, s float64

	hx := highWord(x)
	ix := hx & 0x7FFFFFFF
	if ix >= 0x3FF00000 { /* |x|>= 1 */
		lx := lowWord(x)
		if (uint32(ix-0x3FF00000) | lx) == 0 { /* asin(1)=+-pi/2 with inexact */
			return float64(x*pio2_hi) + float64(x*pio2_lo)
		}
		return math.NaN() // asin(|x|>1) is NaN
	} else if ix < 0x3FE00000 { /* |x|<0.5 */
		if ix < 0x3E400000 { /* if |x| < 2**-27 */
			if huge+x > one {
				return x /* return x with inexact if x!=0*/
			}
		} else {
			t = float64(x * x)
		}
		p = float64(t * (pS0 + float64(t*(pS1+float64(t*(pS2+float64(t*(pS3+float64(t*(pS4+float64(t*pS5)))))))))))
		q = one + float64(t*(qS1+float64(t*(qS2+float64(t*(qS3+float64(t*qS4)))))))
		w = p / q
		return x + float64(x*w)
	}
	/* 1> |x|>= 0.5 */
	w = one - math.Abs(x)
	t = float64(w * 0.5)
	p = float64(t * (pS0 + float64(t*(pS1+float64(t*(pS2+float64(t*(pS3+float64(t*(pS4+float64(t*pS5)))))))))))
	q = one + float64(t*(qS1+float64(t*(qS2+float64(t*(qS3+float64(t*qS4)))))))
	s = math.Sqrt(t)
	if ix >= 0x3FEF3333 { /* if |x| > 0.975 */
		w = p / q
		t = pio2_hi - (float64(2.0*(s+float64(s*w))) - pio2_lo)
	} else {
		w = setLowWord(s, 0)
		c = (t - float64(w*w)) / (s + w)
		r = p / q
		p = float64(2.0*s*r) - (pio2_lo - float64(2.0*c))
		q = pio4_hi - float64(2.0*w)
		t = pio4_hi - (p - q)
	}
	if hx > 0 {
		return t
	}
	return -t
}
