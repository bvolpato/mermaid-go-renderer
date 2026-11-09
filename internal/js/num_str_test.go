package js

import (
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

// strBySteps is the first implementation of Str: the steps of
// Number::toString written out one by one. TestAgainstV8 compares Str with
// V8, and this test compares Str with the steps for many more numbers.
func strBySteps(x float64) string {
	switch {
	case math.IsNaN(x):
		return "NaN"
	case x == 0:
		return "0"
	case math.IsInf(x, 1):
		return "Infinity"
	case math.IsInf(x, -1):
		return "-Infinity"
	}
	var buf [32]byte
	b := strconv.AppendFloat(buf[:0], math.Abs(x), 'e', -1, 64)
	ePos := -1
	for i, c := range b {
		if c == 'e' {
			ePos = i
			break
		}
	}
	exp, _ := strconv.Atoi(string(b[ePos+1:]))
	digits := make([]byte, 0, 20)
	for _, c := range b[:ePos] {
		if c != '.' {
			digits = append(digits, c)
		}
	}
	k := len(digits)
	n := exp + 1
	var sb strings.Builder
	if x < 0 {
		sb.WriteByte('-')
	}
	switch {
	case k <= n && n <= 21:
		sb.Write(digits)
		for i := k; i < n; i++ {
			sb.WriteByte('0')
		}
	case 0 < n && n <= 21:
		sb.Write(digits[:n])
		sb.WriteByte('.')
		sb.Write(digits[n:])
	case -6 < n && n <= 0:
		sb.WriteString("0.")
		for i := n; i < 0; i++ {
			sb.WriteByte('0')
		}
		sb.Write(digits)
	default:
		sb.WriteByte(digits[0])
		if k > 1 {
			sb.WriteByte('.')
			sb.Write(digits[1:])
		}
		sb.WriteByte('e')
		e := n - 1
		if e < 0 {
			sb.WriteByte('-')
			e = -e
		} else {
			sb.WriteByte('+')
		}
		sb.WriteString(strconv.Itoa(e))
	}
	return sb.String()
}

func TestStrAgainstSteps(t *testing.T) {
	check := func(x float64) {
		if got, want := Str(x), strBySteps(x); got != want {
			t.Fatalf("Str(%v) = %q, want %q", x, got, want)
		}
	}
	for _, x := range []float64{
		0, math.Copysign(0, -1), 1, -1, 0.5, 1e21, 1e-7, 123456789012345680000, 1 << 53, 1<<53 - 1, 1<<53 + 2,
		-(1 << 53), -(1<<53 - 1), 1e20, 1e22, 5e-324, math.MaxFloat64, math.SmallestNonzeroFloat64, 0.1 + 0.2,
		999999999999999900000, 1e300, 1.5e-10, 100, 1000000, 0.000001, 0.0000001, 4294967296, 9007199254740000,
		math.NaN(), math.Inf(1), math.Inf(-1),
	} {
		check(x)
		check(-x)
	}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 300000; i++ {
		switch i % 6 {
		case 0:
			check(math.Float64frombits(rng.Uint64()))
		case 1:
			check(float64(rng.Int63n(1<<54) - 1<<53))
		case 2:
			check(float64(rng.Intn(4000)-2000) / float64(1+rng.Intn(64)))
		case 3:
			check(float64(rng.Int63()) * math.Pow(10, float64(rng.Intn(40)-25)))
		case 4:
			check(math.Round(rng.Float64()*1e6) / 1e3)
		case 5:
			// Integers on both sides of 2^53.
			check(float64(int64(1)<<53 + rng.Int63n(1<<12) - 1<<11))
		}
	}
}
