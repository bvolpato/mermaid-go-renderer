// Ported from d3-array 3.2.4, d3-array/src/ticks.js and d3-array/src/nice.js
// (ISC).

// Package array is a port of the parts of d3-array 3.2.4 that mermaid and the
// d3 scales use.
package array

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/jsm"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

var (
	e10 = math.Sqrt(50)
	e5  = math.Sqrt(10)
	e2  = math.Sqrt(2)
)

// maxArrayLength bounds the number of ticks. JavaScript throws a RangeError
// above 2**32-1 elements, and V8 runs out of memory long before that. The
// port returns nil above 2**27 elements.
const maxArrayLength = 1 << 27

func tickSpec(start, stop, count float64) (i1, i2, inc float64) {
	step := (stop - start) / js.Max(0, count)
	power := math.Floor(jsm.Log10(step))
	err := step / jsm.Pow(10, power)
	factor := 1.0
	switch {
	case err >= e10:
		factor = 10
	case err >= e5:
		factor = 5
	case err >= e2:
		factor = 2
	}
	if power < 0 {
		inc = jsm.Pow(10, -power) / factor
		i1 = js.Round(float64(start * inc))
		i2 = js.Round(float64(stop * inc))
		if i1/inc < start {
			i1++
		}
		if i2/inc > stop {
			i2--
		}
		inc = -inc
	} else {
		inc = float64(jsm.Pow(10, power) * factor)
		i1 = js.Round(start / inc)
		i2 = js.Round(stop / inc)
		if i1*inc < start {
			i1++
		}
		if i2*inc > stop {
			i2--
		}
	}
	if i2 < i1 && 0.5 <= count && count < 2 {
		return tickSpec(start, stop, float64(count*2))
	}
	return i1, i2, inc
}

// Ticks is d3.ticks(start, stop, count).
func Ticks(start, stop, count float64) []float64 {
	if !(count > 0) {
		return []float64{}
	}
	if start == stop {
		return []float64{start}
	}
	reverse := stop < start
	var i1, i2, inc float64
	if reverse {
		i1, i2, inc = tickSpec(stop, start, count)
	} else {
		i1, i2, inc = tickSpec(start, stop, count)
	}
	if !(i2 >= i1) {
		return []float64{}
	}
	nf := i2 - i1 + 1
	if !(nf <= maxArrayLength) {
		return nil
	}
	n := int(nf)
	ticks := make([]float64, n)
	if reverse {
		if inc < 0 {
			for i := 0; i < n; i++ {
				ticks[i] = (i2 - float64(i)) / -inc
			}
		} else {
			for i := 0; i < n; i++ {
				ticks[i] = float64((i2 - float64(i)) * inc)
			}
		}
	} else {
		if inc < 0 {
			for i := 0; i < n; i++ {
				ticks[i] = (i1 + float64(i)) / -inc
			}
		} else {
			for i := 0; i < n; i++ {
				ticks[i] = float64((i1 + float64(i)) * inc)
			}
		}
	}
	return ticks
}

// TickIncrement is d3.tickIncrement(start, stop, count).
func TickIncrement(start, stop, count float64) float64 {
	_, _, inc := tickSpec(start, stop, count)
	return inc
}

// TickStep is d3.tickStep(start, stop, count).
func TickStep(start, stop, count float64) float64 {
	reverse := stop < start
	var inc float64
	if reverse {
		inc = TickIncrement(stop, start, count)
	} else {
		inc = TickIncrement(start, stop, count)
	}
	sign := 1.0
	if reverse {
		sign = -1
	}
	if inc < 0 {
		return float64(sign * (1 / -inc))
	}
	return float64(sign * inc)
}

// Nice is d3.nice(start, stop, count). It returns the new start and stop.
func Nice(start, stop, count float64) (float64, float64) {
	prestep := math.NaN() // undefined
	first := true
	for {
		step := TickIncrement(start, stop, count)
		if (!first && step == prestep) || step == 0 || math.IsNaN(step) || math.IsInf(step, 0) {
			return start, stop
		} else if step > 0 {
			start = float64(math.Floor(start/step) * step)
			stop = float64(math.Ceil(stop/step) * step)
		} else if step < 0 {
			start = math.Ceil(start*step) / step
			stop = math.Floor(stop*step) / step
		}
		prestep = step
		first = false
	}
}
