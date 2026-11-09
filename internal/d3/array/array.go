// Ported from d3-array 3.2.4, d3-array/src: ascending.js, descending.js,
// min.js, max.js, extent.js, sum.js, range.js, bisect.js, bisector.js and
// number.js (ISC).

package array

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The d3 functions skip the values null, undefined and NaN. A float64 has
// only NaN for "no value", so NaN stands for all three.

// Ascending is d3.ascending for numbers. It returns NaN when a value is NaN.
func Ascending(a, b float64) float64 {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	case a >= b:
		return 0
	}
	return math.NaN()
}

// Descending is d3.descending for numbers. It returns NaN when a value is NaN.
func Descending(a, b float64) float64 {
	switch {
	case b < a:
		return -1
	case b > a:
		return 1
	case b >= a:
		return 0
	}
	return math.NaN()
}

// Min is d3.min(values). The result is false when no value is comparable,
// which is undefined in JavaScript.
func Min(values []float64) (float64, bool) {
	min, ok := math.NaN(), false
	for _, value := range values {
		if min > value || (!ok && value >= value) {
			min, ok = value, true
		}
	}
	return min, ok
}

// MinFunc is d3.min(values, valueof).
func MinFunc[T any](values []T, valueof func(d T, i int, data []T) float64) (float64, bool) {
	min, ok := math.NaN(), false
	for i, d := range values {
		value := valueof(d, i, values)
		if min > value || (!ok && value >= value) {
			min, ok = value, true
		}
	}
	return min, ok
}

// Max is d3.max(values). The result is false when no value is comparable.
func Max(values []float64) (float64, bool) {
	max, ok := math.NaN(), false
	for _, value := range values {
		if max < value || (!ok && value >= value) {
			max, ok = value, true
		}
	}
	return max, ok
}

// MaxFunc is d3.max(values, valueof).
func MaxFunc[T any](values []T, valueof func(d T, i int, data []T) float64) (float64, bool) {
	max, ok := math.NaN(), false
	for i, d := range values {
		value := valueof(d, i, values)
		if max < value || (!ok && value >= value) {
			max, ok = value, true
		}
	}
	return max, ok
}

// Extent is d3.extent(values). The result is false when no value is
// comparable; d3 returns [undefined, undefined] then.
func Extent(values []float64) (min, max float64, ok bool) {
	min, max = math.NaN(), math.NaN()
	for _, value := range values {
		if !ok {
			if value >= value {
				min, max, ok = value, value, true
			}
		} else {
			if min > value {
				min = value
			}
			if max < value {
				max = value
			}
		}
	}
	return min, max, ok
}

// ExtentFunc is d3.extent(values, valueof).
func ExtentFunc[T any](values []T, valueof func(d T, i int, data []T) float64) (min, max float64, ok bool) {
	min, max = math.NaN(), math.NaN()
	for i, d := range values {
		value := valueof(d, i, values)
		if !ok {
			if value >= value {
				min, max, ok = value, value, true
			}
		} else {
			if min > value {
				min = value
			}
			if max < value {
				max = value
			}
		}
	}
	return min, max, ok
}

// Sum is d3.sum(values). It skips NaN and zero.
func Sum(values []float64) float64 {
	sum := 0.0
	for _, value := range values {
		if value != 0 && !math.IsNaN(value) {
			sum += value
		}
	}
	return sum
}

// SumFunc is d3.sum(values, valueof).
func SumFunc[T any](values []T, valueof func(d T, i int, data []T) float64) float64 {
	sum := 0.0
	for i, d := range values {
		if value := valueof(d, i, values); value != 0 && !math.IsNaN(value) {
			sum += value
		}
	}
	return sum
}

// Range is d3.range(start, stop, step). Call Range(0, stop, 1) for
// d3.range(stop). The result is nil where JavaScript throws a RangeError
// for an invalid array length.
func Range(start, stop, step float64) []float64 {
	n := js.ToInt32(js.Max(0, math.Ceil((stop-start)/step)))
	if n < 0 {
		return nil
	}
	r := make([]float64, n)
	for i := range r {
		r[i] = start + float64(float64(i)*step)
	}
	return r
}

// Bisector is d3.bisector. T is the element type of the sorted array and X
// is the type of the search value.
type Bisector[T, X any] struct {
	compare1 func(a, b X) float64 // tests whether the search value is comparable
	compare2 func(d T, x X) float64
	delta    func(d T, x X) float64
}

// NewBisector is d3.bisector(accessor) for an accessor that returns a number.
func NewBisector[T any](accessor func(d T) float64) *Bisector[T, float64] {
	return &Bisector[T, float64]{
		compare1: Ascending,
		compare2: func(d T, x float64) float64 { return Ascending(accessor(d), x) },
		delta:    func(d T, x float64) float64 { return accessor(d) - x },
	}
}

// NewBisectorFunc is d3.bisector(comparator) for a comparator of an element
// and a search value. d3 cannot test whether the search value is comparable
// in this case, and the port does not test it either.
func NewBisectorFunc[T, X any](comparator func(d T, x X) float64) *Bisector[T, X] {
	return &Bisector[T, X]{
		compare1: func(a, b X) float64 { return 0 },
		compare2: comparator,
		delta:    comparator,
	}
}

// Left is bisector.left(a, x, lo, hi).
func (b *Bisector[T, X]) Left(a []T, x X, lo, hi int) int {
	if lo < hi {
		if b.compare1(x, x) != 0 {
			return hi
		}
		for {
			mid := int(uint32(lo+hi) >> 1)
			if b.compare2(a[mid], x) < 0 {
				lo = mid + 1
			} else {
				hi = mid
			}
			if !(lo < hi) {
				break
			}
		}
	}
	return lo
}

// Right is bisector.right(a, x, lo, hi).
func (b *Bisector[T, X]) Right(a []T, x X, lo, hi int) int {
	if lo < hi {
		if b.compare1(x, x) != 0 {
			return hi
		}
		for {
			mid := int(uint32(lo+hi) >> 1)
			if b.compare2(a[mid], x) <= 0 {
				lo = mid + 1
			} else {
				hi = mid
			}
			if !(lo < hi) {
				break
			}
		}
	}
	return lo
}

// Center is bisector.center(a, x, lo, hi).
func (b *Bisector[T, X]) Center(a []T, x X, lo, hi int) int {
	i := b.Left(a, x, lo, hi-1)
	if i > lo && b.delta(a[i-1], x) > -b.delta(a[i], x) {
		return i - 1
	}
	return i
}

var (
	ascendingBisect = &Bisector[float64, float64]{
		compare1: Ascending,
		compare2: Ascending,
		delta:    Ascending,
	}
	numberBisect = NewBisector(func(d float64) float64 { return d })
)

// BisectRight is d3.bisectRight(a, x, lo, hi), which is also d3.bisect.
func BisectRight(a []float64, x float64, lo, hi int) int {
	return ascendingBisect.Right(a, x, lo, hi)
}

// BisectLeft is d3.bisectLeft(a, x, lo, hi).
func BisectLeft(a []float64, x float64, lo, hi int) int {
	return ascendingBisect.Left(a, x, lo, hi)
}

// BisectCenter is d3.bisectCenter(a, x, lo, hi).
func BisectCenter(a []float64, x float64, lo, hi int) int {
	return numberBisect.Center(a, x, lo, hi)
}
