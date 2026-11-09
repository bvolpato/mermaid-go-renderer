// This file ports no mermaid file. It adds Number.prototype.toString(radix),
// which utils.ts calls as Math.random().toString(36). The digits follow
// DoubleToRadixCString of V8 (src/numbers/conversions.cc).

package js

import "math"

const radixDigits = "0123456789abcdefghijklmnopqrstuvwxyz"

// ToStringRadix is Number.prototype.toString(radix) for a radix from 2 to
// 36. A radix of 10 gives Str(x). Another radix outside the range gives "",
// where JavaScript throws a RangeError.
func ToStringRadix(x float64, radix int) string {
	if radix == 10 {
		return Str(x)
	}
	if radix < 2 || radix > 36 {
		return ""
	}
	switch {
	case math.IsNaN(x):
		return "NaN"
	case math.IsInf(x, 1):
		return "Infinity"
	case math.IsInf(x, -1):
		return "-Infinity"
	case x == 0:
		return "0"
	}

	// The buffer of V8 has 2200 characters. The integer digits grow to the
	// left of the middle and the fraction digits grow to the right.
	const bufferSize = 2200
	var buffer [bufferSize]byte
	integerCursor := bufferSize / 2
	fractionCursor := integerCursor
	value := x
	negative := value < 0
	if negative {
		value = -value
	}

	// Split the value into an integer part and a fractional part.
	integer := math.Floor(value)
	fraction := value - integer
	// Only compute fractional digits up to the precision of the input.
	delta := 0.5 * (math.Nextafter(value, math.Inf(1)) - value)
	delta = max(math.SmallestNonzeroFloat64, delta)
	if fraction >= delta {
		// Insert decimal point.
		buffer[fractionCursor] = '.'
		fractionCursor++
		for {
			// Shift up by one digit.
			fraction = float64(fraction * float64(radix))
			delta = float64(delta * float64(radix))
			// Write digit.
			digit := int(fraction)
			buffer[fractionCursor] = radixDigits[digit]
			fractionCursor++
			// Calculate remainder.
			fraction -= float64(digit)
			// Round to even.
			if fraction > 0.5 || (fraction == 0.5 && digit&1 != 0) {
				if fraction+delta > 1 {
					// Back trace the digits that are already written, in
					// case of a carry.
					for {
						fractionCursor--
						if fractionCursor == bufferSize/2 {
							// Carry over to the integer part.
							integer++
							break
						}
						c := buffer[fractionCursor]
						// Reconstruct digit.
						d := int(c - '0')
						if c > '9' {
							d = int(c-'a') + 10
						}
						if d+1 < radix {
							buffer[fractionCursor] = radixDigits[d+1]
							fractionCursor++
							break
						}
					}
					break
				}
			}
			if !(fraction >= delta) {
				break
			}
		}
	}

	// Compute integer digits. Fill unrepresented digits with zero. V8 tests
	// Double(integer / radix).Exponent() > 0. That exponent counts from a
	// 53-bit integer significand, so it is positive from 2^53.
	for integer/float64(radix) >= 9007199254740992 {
		integer /= float64(radix)
		integerCursor--
		buffer[integerCursor] = '0'
	}
	for {
		remainder := math.Mod(integer, float64(radix))
		integerCursor--
		buffer[integerCursor] = radixDigits[int(remainder)]
		integer = (integer - remainder) / float64(radix)
		if !(integer > 0) {
			break
		}
	}

	// Add sign.
	if negative {
		integerCursor--
		buffer[integerCursor] = '-'
	}
	return string(buffer[integerCursor:fractionCursor])
}
