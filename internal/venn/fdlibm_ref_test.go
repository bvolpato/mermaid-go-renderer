// Ported from v8/src/base/ieee754.cc (V8 14.6): the fdlibm functions sin,
// cos, acos, atan, atan2 and pow, with __kernel_sin, __kernel_cos,
// __ieee754_rem_pio2 and __kernel_rem_pio2.
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

package venn

import "math"

// The functions in this file are the Math functions of the golden file
// testdata/venn.json.gz.
//
// venn.js calls Math.sin, cos, acos, atan2 and pow. Their native results
// depend on the build of V8: node for arm64 contracts a multiplication and an
// addition into one fused operation, node for x64 does not, and Chromium has
// other functions. So tools/reference/gen-venn.mjs installs the fdlibm
// functions of the V8 source code, written in JavaScript
// (tools/reference/elk-fdlibm.mjs), before it runs venn.js in node. This file
// is the same code in Go, a copy of internal/elk/fdlibm_ref_test.go without
// the logarithm. The golden test gives it to the port with Env.Math, and the
// results are then the results of the golden file, bit for bit.
//
// A program gets the Math functions of internal/js, which match Chromium.
//
// Each product that is an operand of an addition or a subtraction has an
// explicit float64 conversion. Without it the Go compiler may fuse the two
// operations into one multiply-add (it does on arm64), and the result is
// then not the result of the C code.

// fdlibmMath is the Math of the golden results, for Env.Math.
var fdlibmMath = &Math{Sin: fdlibmSin, Cos: fdlibmCos, Acos: fdlibmAcos, Atan2: fdlibmAtan2, Pow: fdlibmPow}

func hiWord(x float64) int32  { return int32(math.Float64bits(x) >> 32) }
func loWord(x float64) uint32 { return uint32(math.Float64bits(x)) }

func withLoWord(x float64, lo uint32) float64 {
	return math.Float64frombits(math.Float64bits(x)&0xffffffff00000000 | uint64(lo))
}

func withHiWord(x float64, hi int32) float64 {
	return math.Float64frombits(math.Float64bits(x)&0x00000000ffffffff | uint64(uint32(hi))<<32)
}

func fromWords(hi int32, lo uint32) float64 {
	return math.Float64frombits(uint64(uint32(hi))<<32 | uint64(lo))
}

// two_over_pi: 396 hex digits of 2/pi.
var twoOverPi = [...]int32{
	0xA2F983, 0x6E4E44, 0x1529FC, 0x2757D1, 0xF534DD, 0xC0DB62,
	0x95993C, 0x439041, 0xFE5163, 0xABDEBB, 0xC561B7, 0x246E3A,
	0x424DD2, 0xE00649, 0x2EEA09, 0xD1921C, 0xFE1DEB, 0x1CB129,
	0xA73EE8, 0x8235F5, 0x2EBB44, 0x84E99C, 0x7026B4, 0x5F7E41,
	0x3991D6, 0x398353, 0x39F49C, 0x845F8B, 0xBDF928, 0x3B1FF8,
	0x97FFDE, 0x05980F, 0xEF2F11, 0x8B5A0A, 0x6D1F6D, 0x367ECF,
	0x27CB09, 0xB74F46, 0x3F669E, 0x5FEA2D, 0x7527BA, 0xC7EBE5,
	0xF17B3D, 0x0739F7, 0x8A5292, 0xEA6BFB, 0x5FB11F, 0x8D5D08,
	0x560330, 0x46FC7B, 0x6BABF0, 0xCFBC20, 0x9AF436, 0x1DA9E3,
	0x91615E, 0xE61B08, 0x659985, 0x5F14A0, 0x68408D, 0xFFD880,
	0x4D7327, 0x310606, 0x1556CA, 0x73A8C9, 0x60E27B, 0xC08C6B,
}

// npio2_hw: the high words of n*pi/2 for n = 1 to 32.
var npio2Hw = [...]int32{
	0x3FF921FB, 0x400921FB, 0x4012D97C, 0x401921FB, 0x401F6A7A, 0x4022D97C,
	0x4025FDBB, 0x402921FB, 0x402C463A, 0x402F6A7A, 0x4031475C, 0x4032D97C,
	0x40346B9C, 0x4035FDBB, 0x40378FDB, 0x403921FB, 0x403AB41B, 0x403C463A,
	0x403DD85A, 0x403F6A7A, 0x40407E4C, 0x4041475C, 0x4042106C, 0x4042D97C,
	0x4043A28C, 0x40446B9C, 0x404534AC, 0x4045FDBB, 0x4046C6CB, 0x40478FDB,
	0x404858EB, 0x404921FB,
}

const (
	two24   = 1.67772160000000000000e+07 // 0x41700000, 0x00000000
	twon24  = 5.96046447753906250000e-08 // 0x3E700000, 0x00000000
	invpio2 = 6.36619772367581382433e-01 // 0x3FE45F30, 0x6DC9C883
	pio2_1  = 1.57079632673412561417e+00 // 0x3FF921FB, 0x54400000
	pio2_1t = 6.07710050650619224932e-11 // 0x3DD0B461, 0x1A626331
	pio2_2  = 6.07710050630396597660e-11 // 0x3DD0B461, 0x1A600000
	pio2_2t = 2.02226624879595063154e-21 // 0x3BA3198A, 0x2E037073
	pio2_3  = 2.02226624871116645580e-21 // 0x3BA3198A, 0x2E000000
	pio2_3t = 8.47842766036889956997e-32 // 0x397B839A, 0x252049C1
)

// remPio2 is __ieee754_rem_pio2. It returns n and the remainder y0+y1 of
// x - n*pi/2.
func remPio2(x float64) (n int32, y0, y1 float64) {
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	if ix <= 0x3fe921fb { // |x| ~<= pi/4, no need for reduction
		return 0, x, 0
	}
	if ix < 0x4002d97c { // |x| < 3pi/4, special case with n=+-1
		if hx > 0 {
			z := x - pio2_1
			if ix != 0x3ff921fb { // 33+53 bit pi is good enough
				y0 = z - pio2_1t
				y1 = (z - y0) - pio2_1t
			} else { // near pi/2, use 33+33+53 bit pi
				z -= pio2_2
				y0 = z - pio2_2t
				y1 = (z - y0) - pio2_2t
			}
			return 1, y0, y1
		}
		z := x + pio2_1
		if ix != 0x3ff921fb {
			y0 = z + pio2_1t
			y1 = (z - y0) + pio2_1t
		} else {
			z += pio2_2
			y0 = z + pio2_2t
			y1 = (z - y0) + pio2_2t
		}
		return -1, y0, y1
	}
	if ix <= 0x413921fb { // |x| ~<= 2^19*(pi/2), medium size
		t := math.Abs(x)
		n = int32(float64(t*invpio2) + 0.5)
		fn := float64(n)
		r := t - float64(fn*pio2_1)
		w := float64(fn * pio2_1t) // 1st round good to 85 bit
		if n < 32 && ix != npio2Hw[n-1] {
			y0 = r - w // quick check no cancellation
		} else {
			j := ix >> 20
			y0 = r - w
			i := j - ((hiWord(y0) >> 20) & 0x7ff)
			if i > 16 { // 2nd iteration needed, good to 118
				t = r
				w = float64(fn * pio2_2)
				r = t - w
				w = float64(fn*pio2_2t) - ((t - r) - w)
				y0 = r - w
				i = j - ((hiWord(y0) >> 20) & 0x7ff)
				if i > 49 { // 3rd iteration need, 151 bits acc
					t = r // will cover all possible cases
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
	// All other (large) arguments.
	if ix >= 0x7ff00000 { // x is inf or NaN
		y0 = x - x
		return 0, y0, y0
	}
	// set z = scalbn(|x|,ilogb(x)-23)
	e0 := (ix >> 20) - 1046 // e0 = ilogb(z)-23
	z := fromWords(ix-(e0<<20), loWord(x))
	var tx [3]float64
	for i := 0; i < 2; i++ {
		tx[i] = float64(int32(z))
		z = float64((z - tx[i]) * two24)
	}
	tx[2] = z
	nx := 3
	for tx[nx-1] == 0 { // skip zero term
		nx--
	}
	var y [2]float64
	n = kernelRemPio2(tx[:nx], &y, int(e0))
	if hx < 0 {
		return -n, -y[0], -y[1]
	}
	return n, y[0], y[1]
}

var pio2Table = [...]float64{
	1.57079625129699707031e+00, // 0x3FF921FB, 0x40000000
	7.54978941586159635335e-08, // 0x3E74442D, 0x00000000
	5.39030252995776476554e-15, // 0x3CF84698, 0x80000000
	3.28200341580791294123e-22, // 0x3B78CC51, 0x60000000
	1.27065575308067607349e-29, // 0x39F01B83, 0x80000000
	1.22933308981111328932e-36, // 0x387A2520, 0x40000000
	2.73370053816464559624e-44, // 0x36E38222, 0x80000000
	2.16741683877804819444e-51, // 0x3569F31D, 0x00000000
}

// kernelRemPio2 is __kernel_rem_pio2 for prec = 2 (double precision) and
// ipio2 = two_over_pi.
func kernelRemPio2(x []float64, y *[2]float64, e0 int) int32 {
	const jk = 4 // init_jk[2]
	const jp = jk
	var iq [20]int32
	var f, fq, q [20]float64

	// determine jx,jv,q0, note that 3>q0
	jx := len(x) - 1
	jv := (e0 - 3) / 24
	if jv < 0 {
		jv = 0
	}
	q0 := e0 - 24*(jv+1)

	// set up f[0] to f[jx+jk] where f[jx+jk] = ipio2[jv+jk]
	j := jv - jx
	m := jx + jk
	for i := 0; i <= m; i, j = i+1, j+1 {
		if j < 0 {
			f[i] = 0
		} else {
			f[i] = float64(twoOverPi[j])
		}
	}

	// compute q[0],q[1],...q[jk]
	for i := 0; i <= jk; i++ {
		fw := 0.0
		for j := 0; j <= jx; j++ {
			fw += float64(x[j] * f[jx+i-j])
		}
		q[i] = fw
	}

	jz := jk
	var n int32
	var ih int32
	var z float64
	for {
		// distill q[] into iq[] reversingly
		z = q[jz]
		for i, j := 0, jz; j > 0; i, j = i+1, j-1 {
			fw := float64(int32(float64(twon24 * z)))
			iq[i] = int32(z - float64(two24*fw))
			z = q[j-1] + fw
		}

		// compute n
		z = math.Ldexp(z, q0)                            // actual value of z
		z -= float64(8.0 * math.Floor(float64(z*0.125))) // trim off integer >= 8
		n = int32(z)
		z -= float64(n)
		ih = 0
		if q0 > 0 { // need iq[jz-1] to determine n
			i := iq[jz-1] >> uint(24-q0)
			n += i
			iq[jz-1] -= i << uint(24-q0)
			ih = iq[jz-1] >> uint(23-q0)
		} else if q0 == 0 {
			ih = iq[jz-1] >> 23
		} else if z >= 0.5 {
			ih = 2
		}

		if ih > 0 { // q > 0.5
			n++
			carry := int32(0)
			for i := 0; i < jz; i++ { // compute 1-q
				j := iq[i]
				if carry == 0 {
					if j != 0 {
						carry = 1
						iq[i] = 0x1000000 - j
					}
				} else {
					iq[i] = 0xffffff - j
				}
			}
			if q0 > 0 { // rare case: chance is 1 in 12
				switch q0 {
				case 1:
					iq[jz-1] &= 0x7fffff
				case 2:
					iq[jz-1] &= 0x3fffff
				}
			}
			if ih == 2 {
				z = 1.0 - z
				if carry != 0 {
					z -= math.Ldexp(1.0, q0)
				}
			}
		}

		// check if recomputation is needed
		if z != 0 {
			break
		}
		j := int32(0)
		for i := jz - 1; i >= jk; i-- {
			j |= iq[i]
		}
		if j != 0 {
			break
		}
		// need recomputation
		k := 1
		for iq[jk-k] == 0 { // k = no. of terms needed
			k++
		}
		for i := jz + 1; i <= jz+k; i++ { // add q[jz+1] to q[jz+k]
			f[jx+i] = float64(twoOverPi[jv+i])
			fw := 0.0
			for j := 0; j <= jx; j++ {
				fw += float64(x[j] * f[jx+i-j])
			}
			q[i] = fw
		}
		jz += k
	}

	// chop off zero terms
	if z == 0.0 {
		jz--
		q0 -= 24
		for iq[jz] == 0 {
			jz--
			q0 -= 24
		}
	} else { // break z into 24-bit if necessary
		z = math.Ldexp(z, -q0)
		if z >= two24 {
			fw := float64(int32(float64(twon24 * z)))
			iq[jz] = int32(z - float64(two24*fw))
			jz++
			q0 += 24
			iq[jz] = int32(fw)
		} else {
			iq[jz] = int32(z)
		}
	}

	// convert integer "bit" chunk to floating-point value
	fw := math.Ldexp(1.0, q0)
	for i := jz; i >= 0; i-- {
		q[i] = float64(fw * float64(iq[i]))
		fw *= twon24
	}

	// compute PIo2[0,...,jp]*q[jz,...,0]
	for i := jz; i >= 0; i-- {
		fw = 0.0
		for k := 0; k <= jp && k <= jz-i; k++ {
			fw += float64(pio2Table[k] * q[i+k])
		}
		fq[jz-i] = fw
	}

	// compress fq[] into y[]
	fw = 0.0
	for i := jz; i >= 0; i-- {
		fw += fq[i]
	}
	if ih == 0 {
		y[0] = fw
	} else {
		y[0] = -fw
	}
	fw = fq[0] - fw
	for i := 1; i <= jz; i++ {
		fw += fq[i]
	}
	if ih == 0 {
		y[1] = fw
	} else {
		y[1] = -fw
	}
	return n & 7
}

// kernelSin is __kernel_sin: sin on [-pi/4, pi/4]. y is the tail of x. iy is
// 0 when y is 0.
func kernelSin(x, y float64, iy int) float64 {
	const (
		half = 5.00000000000000000000e-01  // 0x3FE00000, 0x00000000
		s1   = -1.66666666666666324348e-01 // 0xBFC55555, 0x55555549
		s2   = 8.33333333332248946124e-03  // 0x3F811111, 0x1110F8A6
		s3   = -1.98412698298579493134e-04 // 0xBF2A01A0, 0x19C161D5
		s4   = 2.75573137070700676789e-06  // 0x3EC71DE3, 0x57B1FE7D
		s5   = -2.50507602534068634195e-08 // 0xBE5AE5E6, 0x8A2B9CEB
		s6   = 1.58969099521155010221e-10  // 0x3DE5D93A, 0x5ACFD57C
	)
	ix := hiWord(x) & 0x7fffffff
	if ix < 0x3e400000 { // |x| < 2**-27
		if int32(x) == 0 {
			return x
		}
	}
	z := float64(x * x)
	v := float64(z * x)
	r := s2 + float64(z*(s3+float64(z*(s4+float64(z*(s5+float64(z*s6)))))))
	if iy == 0 {
		return x + float64(v*(s1+float64(z*r)))
	}
	return x - ((float64(z*(float64(half*y)-float64(v*r))) - y) - float64(v*s1))
}

// kernelCos is __kernel_cos: cos on [-pi/4, pi/4]. y is the tail of x.
func kernelCos(x, y float64) float64 {
	const (
		c1 = 4.16666666666666019037e-02  // 0x3FA55555, 0x5555554C
		c2 = -1.38888888888741095749e-03 // 0xBF56C16C, 0x16C15177
		c3 = 2.48015872894767294178e-05  // 0x3EFA01A0, 0x19CB1590
		c4 = -2.75573143513906633035e-07 // 0xBE927E4F, 0x809C52AD
		c5 = 2.08757232129817482790e-09  // 0x3E21EE9E, 0xBDB4B1C4
		c6 = -1.13596475577881948265e-11 // 0xBDA8FAE9, 0xBE8838D4
	)
	ix := hiWord(x) & 0x7fffffff
	if ix < 0x3e400000 { // if x < 2**27
		if int32(x) == 0 {
			return 1
		}
	}
	z := float64(x * x)
	r := float64(z * (c1 + float64(z*(c2+float64(z*(c3+float64(z*(c4+float64(z*(c5+float64(z*c6)))))))))))
	if ix < 0x3FD33333 { // if |x| < 0.3
		return 1 - (float64(0.5*z) - (float64(z*r) - float64(x*y)))
	}
	var qx float64
	if ix > 0x3fe90000 { // x > 0.78125
		qx = 0.28125
	} else {
		qx = fromWords(ix-0x00200000, 0) // x/4
	}
	iz := float64(0.5*z) - qx
	a := 1 - qx
	return a - (iz - (float64(z*r) - float64(x*y)))
}

// fdlibmSin is sin of fdlibm.
func fdlibmSin(x float64) float64 {
	ix := hiWord(x) & 0x7fffffff
	if ix <= 0x3fe921fb { // |x| ~< pi/4
		return kernelSin(x, 0, 0)
	}
	if ix >= 0x7ff00000 { // sin(Inf or NaN) is NaN
		return x - x
	}
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

// fdlibmCos is cos of fdlibm.
func fdlibmCos(x float64) float64 {
	ix := hiWord(x) & 0x7fffffff
	if ix <= 0x3fe921fb { // |x| ~< pi/4
		return kernelCos(x, 0)
	}
	if ix >= 0x7ff00000 { // cos(Inf or NaN) is NaN
		return x - x
	}
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

// fdlibmAcos is acos of fdlibm.
func fdlibmAcos(x float64) float64 {
	const (
		pi     = 3.14159265358979311600e+00  // 0x400921FB, 0x54442D18
		pio2Hi = 1.57079632679489655800e+00  // 0x3FF921FB, 0x54442D18
		pio2Lo = 6.12323399573676603587e-17  // 0x3C91A626, 0x33145C07
		pS0    = 1.66666666666666657415e-01  // 0x3FC55555, 0x55555555
		pS1    = -3.25565818622400915405e-01 // 0xBFD4D612, 0x03EB6F7D
		pS2    = 2.01212532134862925881e-01  // 0x3FC9C155, 0x0E884455
		pS3    = -4.00555345006794114027e-02 // 0xBFA48228, 0xB5688F3B
		pS4    = 7.91534994289814532176e-04  // 0x3F49EFE0, 0x7501B288
		pS5    = 3.47933107596021167570e-05  // 0x3F023DE1, 0x0DFDF709
		qS1    = -2.40339491173441421878e+00 // 0xC0033A27, 0x1C8A2D4B
		qS2    = 2.02094576023350569471e+00  // 0x40002AE5, 0x9C598AC8
		qS3    = -6.88283971605453293030e-01 // 0xBFE6066C, 0x1B8D0159
		qS4    = 7.70381505559019352791e-02  // 0x3FB3B8C5, 0xB12E9282
	)
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	if ix >= 0x3ff00000 { // |x| >= 1
		if (uint32(ix-0x3ff00000) | loWord(x)) == 0 { // |x|==1
			if hx > 0 {
				return 0.0 // acos(1) = 0
			}
			return pi + float64(2.0*pio2Lo) // acos(-1)= pi
		}
		return (x - x) / (x - x) // acos(|x|>1) is NaN
	}
	if ix < 0x3fe00000 { // |x| < 0.5
		if ix <= 0x3c600000 { // if |x| < 2**-57
			return float64(pio2Hi) + float64(pio2Lo)
		}
		z := float64(x * x)
		p := float64(z * (pS0 + float64(z*(pS1+float64(z*(pS2+float64(z*(pS3+float64(z*(pS4+float64(z*pS5)))))))))))
		q := 1 + float64(z*(qS1+float64(z*(qS2+float64(z*(qS3+float64(z*qS4)))))))
		r := p / q
		return pio2Hi - (x - (pio2Lo - float64(r*x)))
	}
	if hx < 0 { // x < -0.5
		z := float64((1 + x) * 0.5)
		p := float64(z * (pS0 + float64(z*(pS1+float64(z*(pS2+float64(z*(pS3+float64(z*(pS4+float64(z*pS5)))))))))))
		q := 1 + float64(z*(qS1+float64(z*(qS2+float64(z*(qS3+float64(z*qS4)))))))
		s := math.Sqrt(z)
		r := p / q
		w := float64(r*s) - pio2Lo
		return pi - float64(2.0*(s+w))
	}
	// x > 0.5
	z := float64((1 - x) * 0.5)
	s := math.Sqrt(z)
	df := withLoWord(s, 0)
	c := (z - float64(df*df)) / (s + df)
	p := float64(z * (pS0 + float64(z*(pS1+float64(z*(pS2+float64(z*(pS3+float64(z*(pS4+float64(z*pS5)))))))))))
	q := 1 + float64(z*(qS1+float64(z*(qS2+float64(z*(qS3+float64(z*qS4)))))))
	r := p / q
	w := float64(r*s) + c
	return 2.0 * (df + w)
}

var atanHi = [...]float64{
	4.63647609000806093515e-01, // atan(0.5)hi 0x3FDDAC67, 0x0561BB4F
	7.85398163397448278999e-01, // atan(1.0)hi 0x3FE921FB, 0x54442D18
	9.82793723247329054082e-01, // atan(1.5)hi 0x3FEF730B, 0xD281F69B
	1.57079632679489655800e+00, // atan(inf)hi 0x3FF921FB, 0x54442D18
}

var atanLo = [...]float64{
	2.26987774529616870924e-17, // atan(0.5)lo 0x3C7A2B7F, 0x222F65E2
	3.06161699786838301793e-17, // atan(1.0)lo 0x3C81A626, 0x33145C07
	1.39033110312309984516e-17, // atan(1.5)lo 0x3C700788, 0x7AF0CBBD
	6.12323399573676603587e-17, // atan(inf)lo 0x3C91A626, 0x33145C07
}

var aT = [...]float64{
	3.33333333333329318027e-01,  // 0x3FD55555, 0x5555550D
	-1.99999999998764832476e-01, // 0xBFC99999, 0x9998EBC4
	1.42857142725034663711e-01,  // 0x3FC24924, 0x920083FF
	-1.11111104054623557880e-01, // 0xBFBC71C6, 0xFE231671
	9.09088713343650656196e-02,  // 0x3FB745CD, 0xC54C206E
	-7.69187620504482999495e-02, // 0xBFB3B0F2, 0xAF749A6D
	6.66107313738753120669e-02,  // 0x3FB10D66, 0xA0D03D51
	-5.83357013379057348645e-02, // 0xBFADDE2D, 0x52DEFD9A
	4.97687799461593236017e-02,  // 0x3FA97B4B, 0x24760DEB
	-3.65315727442169155270e-02, // 0xBFA2B444, 0x2C6A6C2F
	1.62858201153657823623e-02,  // 0x3F90AD3A, 0xE322DA11
}

// fdlibmAtan is atan of fdlibm.
func fdlibmAtan(x float64) float64 {
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	var id int
	if ix >= 0x44100000 { // if |x| >= 2^66
		if ix > 0x7ff00000 || (ix == 0x7ff00000 && loWord(x) != 0) {
			return x + x // NaN
		}
		if hx > 0 {
			return atanHi[3] + atanLo[3]
		}
		return -atanHi[3] - atanLo[3]
	}
	if ix < 0x3fdc0000 { // |x| < 0.4375
		if ix < 0x3e400000 { // |x| < 2^-27
			return x
		}
		id = -1
	} else {
		x = math.Abs(x)
		if ix < 0x3ff30000 { // |x| < 1.1875
			if ix < 0x3fe60000 { // 7/16 <=|x|<11/16
				id = 0
				x = (float64(2.0*x) - 1) / (2.0 + x)
			} else { // 11/16<=|x|< 19/16
				id = 1
				x = (x - 1) / (x + 1)
			}
		} else {
			if ix < 0x40038000 { // |x| < 2.4375
				id = 2
				x = (x - 1.5) / (1 + float64(1.5*x))
			} else { // 2.4375 <= |x| < 2^66
				id = 3
				x = -1.0 / x
			}
		}
	}
	// end of argument reduction
	z := float64(x * x)
	w := float64(z * z)
	// break sum from i=0 to 10 aT[i]z**(i+1) into odd and even poly
	s1 := float64(z * (aT[0] + float64(w*(aT[2]+float64(w*(aT[4]+float64(w*(aT[6]+float64(w*(aT[8]+float64(w*aT[10])))))))))))
	s2 := float64(w * (aT[1] + float64(w*(aT[3]+float64(w*(aT[5]+float64(w*(aT[7]+float64(w*aT[9])))))))))
	if id < 0 {
		return x - float64(x*(s1+s2))
	}
	z = atanHi[id] - ((float64(x*(s1+s2)) - atanLo[id]) - x)
	if hx < 0 {
		return -z
	}
	return z
}

// fdlibmAtan2 is atan2 of fdlibm.
func fdlibmAtan2(y, x float64) float64 {
	const (
		tiny  = 1.0e-300
		piO4  = 7.8539816339744827900e-01 // 0x3FE921FB, 0x54442D18
		piO2  = 1.5707963267948965580e+00 // 0x3FF921FB, 0x54442D18
		pi    = 3.1415926535897931160e+00 // 0x400921FB, 0x54442D18
		piLo  = 1.2246467991473531772e-16 // 0x3CA1A626, 0x33145C07
		three = 3.0
	)
	// The sums with tiny are not constant expressions in C: they are
	// rounded to double. A Go constant expression is exact, so the
	// operands are variables here.
	vTiny, vPi, vPiO2, vPiO4 := float64(tiny), float64(pi), float64(piO2), float64(piO4)

	hx, lx := hiWord(x), loWord(x)
	ix := hx & 0x7fffffff
	hy, ly := hiWord(y), loWord(y)
	iy := hy & 0x7fffffff
	if uint32(ix)|((lx|-lx)>>31) > 0x7ff00000 || uint32(iy)|((ly|-ly)>>31) > 0x7ff00000 {
		return x + y // x or y is NaN
	}
	if (uint32(hx-0x3ff00000) | lx) == 0 {
		return fdlibmAtan(y) // x=1.0
	}
	m := ((hy >> 31) & 1) | ((hx >> 30) & 2) // 2*sign(x)+sign(y)

	// when y = 0
	if (uint32(iy) | ly) == 0 {
		switch m {
		case 0, 1:
			return y // atan(+-0,+anything)=+-0
		case 2:
			return vPi + vTiny // atan(+0,-anything) = pi
		case 3:
			return -vPi - vTiny // atan(-0,-anything) =-pi
		}
	}
	// when x = 0
	if (uint32(ix) | lx) == 0 {
		if hy < 0 {
			return -vPiO2 - vTiny
		}
		return vPiO2 + vTiny
	}
	// when x is INF
	if ix == 0x7ff00000 {
		if iy == 0x7ff00000 {
			switch m {
			case 0:
				return vPiO4 + vTiny // atan(+INF,+INF)
			case 1:
				return -vPiO4 - vTiny // atan(-INF,+INF)
			case 2:
				return float64(three*vPiO4) + vTiny // atan(+INF,-INF)
			case 3:
				return float64(-three*vPiO4) - vTiny // atan(-INF,-INF)
			}
		} else {
			switch m {
			case 0:
				return 0 // atan(+...,+INF)
			case 1:
				return math.Copysign(0, -1) // atan(-...,+INF)
			case 2:
				return vPi + vTiny // atan(+...,-INF)
			case 3:
				return -vPi - vTiny // atan(-...,-INF)
			}
		}
	}
	// when y is INF
	if iy == 0x7ff00000 {
		if hy < 0 {
			return -vPiO2 - vTiny
		}
		return vPiO2 + vTiny
	}

	// compute y/x
	var z float64
	k := (iy - ix) >> 20
	if k > 60 { // |y/x| >  2**60
		z = vPiO2 + float64(0.5*piLo)
		m &= 1
	} else if hx < 0 && k < -60 {
		z = 0.0 // 0 > |y|/x > -2**-60
	} else {
		z = fdlibmAtan(math.Abs(y / x)) // safe to do y/x
	}
	switch m {
	case 0:
		return z // atan(+,+)
	case 1:
		return -z // atan(-,+)
	case 2:
		return vPi - (z - piLo) // atan(+,-)
	default: // case 3
		return (z - piLo) - vPi // atan(-,-)
	}
}

// fdlibmPow is Math.pow with __ieee754_pow of fdlibm, after the two cases in
// which ECMAScript differs from C (a NaN exponent, and 1 or -1 to an infinite
// power).
func fdlibmPow(x, y float64) float64 {
	if math.IsNaN(y) {
		return math.NaN()
	}
	if (x == 1 || x == -1) && math.IsInf(y, 0) {
		return math.NaN()
	}
	return ieee754Pow(x, y)
}

func ieee754Pow(x, y float64) float64 {
	const (
		two53 = 9007199254740992.0 // 0x43400000, 0x00000000
		huge  = 1.0e300
		tiny  = 1.0e-300
		// poly coefs for (3/2)*(log(x)-2s-2/3*s**3
		l1 = 5.99999999999994648725e-01  // 0x3FE33333, 0x33333303
		l2 = 4.28571428578550184252e-01  // 0x3FDB6DB6, 0xDB6FABFF
		l3 = 3.33333329818377432918e-01  // 0x3FD55555, 0x518F264D
		l4 = 2.72728123808534006489e-01  // 0x3FD17460, 0xA91D4101
		l5 = 2.30660745775561754067e-01  // 0x3FCD864A, 0x93C9DB65
		l6 = 2.06975017800338417784e-01  // 0x3FCA7E28, 0x4A454EEF
		p1 = 1.66666666666666019037e-01  // 0x3FC55555, 0x5555553E
		p2 = -2.77777777770155933842e-03 // 0xBF66C16C, 0x16BEBD93
		p3 = 6.61375632143793436117e-05  // 0x3F11566A, 0xAF25DE2C
		p4 = -1.65339022054652515390e-06 // 0xBEBBBD41, 0xC5D26BF1
		p5 = 4.13813679705723846039e-08  // 0x3E663769, 0x72BEA4D0
		// lg2 = 0x3FE62E42, 0xFEFA39EF
		lg2    = 6.93147180559945286227e-01
		lg2H   = 6.93147182464599609375e-01  // 0x3FE62E43, 0x00000000
		lg2L   = -1.90465429995776804525e-09 // 0xBE205C61, 0x0CA86C39
		ovt    = 8.0085662595372944372e-17   // -(1024-log2(ovfl+.5ulp))
		cp     = 9.61796693925975554329e-01  // 0x3FEEC709, 0xDC3A03FD =2/(3ln2)
		cpH    = 9.61796700954437255859e-01  // 0x3FEEC709, 0xE0000000 =(float)cp
		cpL    = -7.02846165095275826516e-09 // 0xBE3E2FE0, 0x145B01F5 =tail of cp_h
		ivln2  = 1.44269504088896338700e+00  // 0x3FF71547, 0x652B82FE =1/ln2
		ivln2H = 1.44269502162933349609e+00  // 0x3FF71547, 0x60000000 =24b 1/ln2
		ivln2L = 1.92596299112661746302e-08  // 0x3E54AE0B, 0xF85DDF44 =1/ln2 tail
	)
	bp := [2]float64{1.0, 1.5}
	dpH := [2]float64{0.0, 5.84962487220764160156e-01} // 0x3FE2B803, 0x40000000
	dpL := [2]float64{0.0, 1.35003920212974897128e-08} // 0x3E4CFDEB, 0x43CFD006
	// huge*huge and tiny*tiny overflow and underflow at run time in C.
	vHuge, vTiny := float64(huge), float64(tiny)

	hx, lx := hiWord(x), loWord(x)
	hy, ly := hiWord(y), loWord(y)
	ix := hx & 0x7fffffff
	iy := hy & 0x7fffffff

	// y==zero: x**0 = 1
	if (uint32(iy) | ly) == 0 {
		return 1
	}

	// +-NaN return x+y
	if ix > 0x7ff00000 || (ix == 0x7ff00000 && lx != 0) || iy > 0x7ff00000 || (iy == 0x7ff00000 && ly != 0) {
		return x + y
	}

	// determine if y is an odd int when x < 0
	// yisint = 0	... y is not an integer
	// yisint = 1	... y is an odd int
	// yisint = 2	... y is an even int
	yisint := int32(0)
	if hx < 0 {
		if iy >= 0x43400000 {
			yisint = 2 // even integer y
		} else if iy >= 0x3ff00000 {
			k := (iy >> 20) - 0x3ff // exponent
			if k > 20 {
				j := ly >> uint(52-k)
				if (j << uint(52-k)) == ly {
					yisint = 2 - int32(j&1)
				}
			} else if ly == 0 {
				j := iy >> uint(20-k)
				if (j << uint(20-k)) == iy {
					yisint = 2 - (j & 1)
				}
			}
		}
	}

	// special value of y
	if ly == 0 {
		if iy == 0x7ff00000 { // y is +-inf
			if (uint32(ix-0x3ff00000) | lx) == 0 {
				return y - y // inf**+-1 is NaN
			} else if ix >= 0x3ff00000 { // (|x|>1)**+-inf = inf,0
				if hy >= 0 {
					return y
				}
				return 0
			}
			// (|x|<1)**-,+inf = inf,0
			if hy < 0 {
				return -y
			}
			return 0
		}
		if iy == 0x3ff00000 { // y is  +-1
			if hy < 0 {
				return 1 / x
			}
			return x
		}
		if hy == 0x40000000 {
			return x * x // y is  2
		}
		if hy == 0x3fe00000 { // y is  0.5
			if hx >= 0 { // x >= +0
				return math.Sqrt(x)
			}
		}
	}

	ax := math.Abs(x)
	// special value of x
	if lx == 0 {
		if ix == 0x7ff00000 || ix == 0 || ix == 0x3ff00000 {
			z := ax // x is +-0,+-inf,+-1
			if hy < 0 {
				z = 1 / z // z = (1/|x|)
			}
			if hx < 0 {
				if ((ix - 0x3ff00000) | yisint) == 0 {
					z = (z - z) / (z - z) // (-1)**non-int is NaN
				} else if yisint == 1 {
					z = -z // (x<0)**odd = -(|x|**odd)
				}
			}
			return z
		}
	}

	n := int32(uint32(hx)>>31) - 1

	// (x<0)**(non-int) is NaN
	if (n | yisint) == 0 {
		return (x - x) / (x - x)
	}

	s := 1.0 // s (sign of result -ve**odd) = -1 else = 1
	if (n | (yisint - 1)) == 0 {
		s = -1.0 // (-ve)**(odd int)
	}

	var t1, t2 float64
	// |y| is huge
	if iy > 0x41e00000 { // if |y| > 2**31
		if iy > 0x43f00000 { // if |y| > 2**64, must o/uflow
			if ix <= 0x3fefffff {
				if hy < 0 {
					return vHuge * vHuge
				}
				return vTiny * vTiny
			}
			if ix >= 0x3ff00000 {
				if hy > 0 {
					return vHuge * vHuge
				}
				return vTiny * vTiny
			}
		}
		// over/underflow if x is not close to one
		if ix < 0x3fefffff {
			if hy < 0 {
				return float64(s*vHuge) * vHuge
			}
			return float64(s*vTiny) * vTiny
		}
		if ix > 0x3ff00000 {
			if hy > 0 {
				return float64(s*vHuge) * vHuge
			}
			return float64(s*vTiny) * vTiny
		}
		// now |1-x| is tiny <= 2**-20, suffice to compute
		// log(x) by x-x^2/2+x^3/3-x^4/4
		t := ax - 1 // t has 20 trailing zeros
		w := float64(float64(t*t) * (0.5 - float64(t*(0.3333333333333333333333-float64(t*0.25)))))
		u := float64(ivln2H * t) // ivln2_h has 21 sig. bits
		v := float64(t*ivln2L) - float64(w*ivln2)
		t1 = withLoWord(u+v, 0)
		t2 = v - (t1 - u)
	} else {
		n = 0
		// take care subnormal number
		if ix < 0x00100000 {
			ax *= two53
			n -= 53
			ix = hiWord(ax)
		}
		n += (ix >> 20) - 0x3ff
		j := ix & 0x000fffff
		// determine interval
		ix = j | 0x3ff00000 // normalize ix
		var k int
		if j <= 0x3988E {
			k = 0 // |x|<sqrt(3/2)
		} else if j < 0xBB67A {
			k = 1 // |x|<sqrt(3)
		} else {
			k = 0
			n++
			ix -= 0x00100000
		}
		ax = withHiWord(ax, ix)

		// compute ss = s_h+s_l = (x-1)/(x+1) or (x-1.5)/(x+1.5)
		u := ax - bp[k] // bp[0]=1.0, bp[1]=1.5
		v := 1 / (ax + bp[k])
		ss := float64(u * v)
		sH := withLoWord(ss, 0)
		// t_h=ax+bp[k] High
		tH := fromWords(((ix>>1)|0x20000000)+0x00080000+int32(k<<18), 0)
		tL := ax - (tH - bp[k])
		sL := float64(v * ((u - float64(sH*tH)) - float64(sH*tL)))
		// compute log(ax)
		s2 := float64(ss * ss)
		r := float64(float64(s2*s2) * (l1 + float64(s2*(l2+float64(s2*(l3+float64(s2*(l4+float64(s2*(l5+float64(s2*l6)))))))))))
		r += float64(sL * (sH + ss))
		s2 = float64(sH * sH)
		tH = withLoWord(3.0+s2+r, 0)
		tL = r - ((tH - 3.0) - s2)
		// u+v = ss*(1+...)
		u = float64(sH * tH)
		v = float64(sL*tH) + float64(tL*ss)
		// 2/(3log2)*(ss+...)
		pH := withLoWord(u+v, 0)
		pL := v - (pH - u)
		zH := float64(cpH * pH) // cp_h+cp_l = 2/(3*log2)
		zL := float64(cpL*pH) + float64(pL*cp) + dpL[k]
		// log2(ax) = (ss+..)*2/(3*log2) = n + dp_h + z_h + z_l
		t := float64(n)
		t1 = withLoWord(((zH+zL)+dpH[k])+t, 0)
		t2 = zL - (((t1 - t) - dpH[k]) - zH)
	}

	// split up y into y1+y2 and compute (y1+y2)*(t1+t2)
	y1 := withLoWord(y, 0)
	pL := float64((y-y1)*t1) + float64(y*t2)
	pH := float64(y1 * t1)
	z := pL + pH
	j := hiWord(z)
	i := int32(loWord(z))
	if j >= 0x40900000 { // z >= 1024
		if ((j - 0x40900000) | i) != 0 { // if z > 1024
			return float64(s*vHuge) * vHuge // overflow
		}
		if pL+ovt > z-pH {
			return float64(s*vHuge) * vHuge // overflow
		}
	} else if (j & 0x7fffffff) >= 0x4090cc00 { // z <= -1075
		if (uint32(j-int32(-0x3f6f3400)) | uint32(i)) != 0 { // z < -1075
			return float64(s*vTiny) * vTiny // underflow
		}
		if pL <= z-pH {
			return float64(s*vTiny) * vTiny // underflow
		}
	}
	// compute 2**(p_h+p_l)
	i = j & 0x7fffffff
	k := (i >> 20) - 0x3ff
	n = 0
	if i > 0x3fe00000 { // if |z| > 0.5, set n = [z+0.5]
		n = j + (0x00100000 >> uint(k+1))
		k = ((n & 0x7fffffff) >> 20) - 0x3ff // new k for n
		t := fromWords(n&^(0x000fffff>>uint(k)), 0)
		n = ((n & 0x000fffff) | 0x00100000) >> uint(20-k)
		if j < 0 {
			n = -n
		}
		pH -= t
	}
	t := withLoWord(pL+pH, 0)
	u := float64(t * lg2H)
	v := float64((pL-(t-pH))*lg2) + float64(t*lg2L)
	z = u + v
	w := v - (z - u)
	t = float64(z * z)
	t1 = z - float64(t*(p1+float64(t*(p2+float64(t*(p3+float64(t*(p4+float64(t*p5)))))))))
	r := float64(z*t1)/(t1-2) - (w + float64(z*w))
	z = 1 - (r - z)
	j = hiWord(z)
	j += n << 20
	if (j >> 20) <= 0 {
		z = math.Ldexp(z, int(n)) // subnormal output
	} else {
		z = withHiWord(z, hiWord(z)+(n<<20))
	}
	return s * z
}
