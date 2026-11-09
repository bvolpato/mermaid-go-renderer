// Ported from java.util.Random of the GWT runtime that elkjs 0.9.3 is
// compiled with (the emulation class java/util/Random.java of GWT), as it
// appears in elk-worker.js (Apache-2.0).
//
// GWT has the Apache License 2.0. elk-worker.js has no notice for this
// code. The Java source of GWT for it is
// user/super/com/google/gwt/emul/java/util/Random.java. Its notices, read
// from the main branch of https://github.com/gwtproject/gwt on 2026-11-09:
//
//	Copyright 2009 Google Inc.
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with this
//	work for additional information regarding copyright ownership. The ASF
//	licenses this file to You under the Apache License, Version 2.0 (the
//	"License"); you may not use this file except in compliance with the License.
//
//	INCLUDES MODIFICATIONS BY RICHARD ZSCHECH AS WELL AS GOOGLE.
//
// This file is a Go translation of that code. It is a changed version of
// it. LICENSE-GWT.txt in this directory has the license, and NOTICE in this
// directory has the license notices of the GWT files.

// Package gwt has the parts of the GWT Java runtime whose behavior decides the
// layout of elkjs: the random number generator, the iteration order of the
// hash collections, the sort algorithm and the number conversions.
//
// elkjs is Eclipse ELK compiled from Java to JavaScript by GWT. The emulated
// classes of GWT do not behave like the classes of a Java VM in each detail,
// and the layout depends on the details. Ported ELK code must use this
// package where the Java code uses the matching class.
package gwt

import "math"

const (
	multiplierHi = 1502
	multiplierLo = 15525485
	twoTo24      = 16777216
	twoToMinus24 = 5.9604644775390625e-8
	twoToMinus26 = 1.4901161193847656e-8
	twoToMinus31 = 4.6566128730773926e-10
	twoToMinus53 = 1.1102230246251565e-16
)

// Random is java.util.Random of GWT: a linear congruential generator with a
// state of 48 bits in two doubles.
type Random struct {
	// SeedHi and SeedLo are the upper and the lower 24 bits of the state.
	SeedHi, SeedLo float64
}

// NewRandom is new Random(seed) for a long seed.
func NewRandom(seed int64) *Random {
	r := &Random{}
	r.SetSeed(seed)
	return r
}

// NewRandomFromDouble is new Random() of GWT, where seed is the value of
// uniqueSeed++ + Date.now(). The Context of a layout supplies that value.
func NewRandomFromDouble(seed float64) *Random {
	hi := RoundInt(math.Floor(seed*twoToMinus24)) & 16777215
	lo := RoundInt(seed - float64(float64(hi)*twoTo24))
	return &Random{SeedHi: float64(hi ^ multiplierHi), SeedLo: float64(lo ^ multiplierLo)}
}

// SetSeed is setSeed(long).
func (r *Random) SetSeed(seed int64) {
	hi := int32((seed >> 24) & 16777215)
	lo := int32(seed & 16777215)
	r.SeedHi = float64(hi ^ multiplierHi)
	r.SeedLo = float64(lo ^ multiplierLo)
}

// Clone returns a generator with the same state.
func (r *Random) Clone() *Random {
	c := *r
	return &c
}

// nextInternal is Random.nextInternal(bits).
func (r *Random) nextInternal(bits int) float64 {
	hi := float64(r.SeedHi*multiplierLo) + float64(r.SeedLo*multiplierHi)
	lo := float64(r.SeedLo*multiplierLo) + 11
	carry := math.Floor(lo * twoToMinus24)
	hi += carry
	lo -= float64(carry * twoTo24)
	hi = math.Mod(hi, twoTo24)
	r.SeedHi = hi
	r.SeedLo = lo
	if bits <= 24 {
		return math.Floor(r.SeedHi * math.Ldexp(1, bits-24))
	}
	h := float64(r.SeedHi * float64(int(1)<<(bits-24)))
	l := math.Floor(r.SeedLo * math.Ldexp(1, bits-48))
	dval := h + l
	if dval >= 2147483648 {
		dval -= 4294967296
	}
	return dval
}

// NextInt is nextInt(n). n must be positive.
func (r *Random) NextInt(n int) int {
	if n <= 0 {
		panic("gwt: Random.NextInt: n must be positive")
	}
	if n&-n == n {
		return RoundInt(float64(n) * r.nextInternal(31) * twoToMinus31)
	}
	for {
		bits := r.nextInternal(31)
		val := math.Mod(bits, float64(n))
		// The sum does not overflow in JavaScript, so the loop runs one time.
		if bits-val+float64(n-1) >= 0 {
			return RoundInt(val)
		}
	}
}

// NextInt32 is nextInt().
func (r *Random) NextInt32() int {
	return RoundInt(r.nextInternal(32))
}

// NextDouble is nextDouble().
func (r *Random) NextDouble() float64 {
	a := r.nextInternal(26)
	b := r.nextInternal(27)
	return float64(a*twoToMinus26) + float64(b*twoToMinus53)
}

// NextFloat is nextFloat(). The result is a double, as each float of GWT.
func (r *Random) NextFloat() float64 {
	return r.nextInternal(24) * twoToMinus24
}

// NextBoolean is nextBoolean().
func (r *Random) NextBoolean() bool {
	return r.nextInternal(1) != 0
}

// NextLong is nextLong().
func (r *Random) NextLong() int64 {
	hi := int64(r.nextInternal(32))
	lo := int64(r.nextInternal(32))
	return hi<<32 + lo
}
