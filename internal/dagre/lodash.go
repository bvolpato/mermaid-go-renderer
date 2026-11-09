// Ported from lodash-es 4.18.1: range.js (_createRange.js, _baseRange.js,
// toFinite.js), min.js, max.js, minBy.js (_baseExtremum.js) and sortBy.js
// (_baseOrderBy.js, _compareMultiple.js, _compareAscending.js).

package dagre

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// toFinite is lodash toFinite for a number. NaN stands for undefined.
func toFinite(value float64) float64 {
	if value == 0 || value != value {
		if value == 0 {
			return value
		}
		return 0
	}
	if math.IsInf(value, 0) {
		if value < 0 {
			return -math.MaxFloat64
		}
		return math.MaxFloat64
	}
	return value
}

func baseRange(start, end, step float64) []float64 {
	d := step
	if !truthy(d) {
		d = 1
	}
	length := max(math.Ceil((end-start)/d), 0)
	if !(length < 1<<32) {
		// new Array(length) throws a RangeError.
		panic(typeError("Invalid array length"))
	}
	result := make([]float64, int(length))
	for i := range result {
		result[i] = start
		start += step
	}
	return result
}

// rangeEnd is _.range(end).
func rangeEnd(end float64) []float64 {
	start := toFinite(end)
	end = start
	start = 0
	step := -1.0
	if start < end {
		step = 1
	}
	return baseRange(start, end, step)
}

// rangeStartEnd is _.range(start, end).
func rangeStartEnd(start, end float64) []float64 {
	start = toFinite(start)
	end = toFinite(end)
	step := -1.0
	if start < end {
		step = 1
	}
	return baseRange(start, end, step)
}

// rangeStartEndStep is _.range(start, end, step).
func rangeStartEndStep(start, end, step float64) []float64 {
	return baseRange(toFinite(start), toFinite(end), toFinite(step))
}

// lodashMin is _.min for numbers. NaN stands for undefined or NaN: lodash
// skips both. ok is false when the result is undefined. The first of equal
// minimums wins, which matters for the sign of zero.
func lodashMin(values []float64) (result float64, ok bool) {
	for _, current := range values {
		if current != current {
			continue
		}
		if !ok || current < result {
			result, ok = current, true
		}
	}
	return result, ok
}

// lodashMax is _.max for numbers. See lodashMin.
func lodashMax(values []float64) (result float64, ok bool) {
	for _, current := range values {
		if current != current {
			continue
		}
		if !ok || current > result {
			result, ok = current, true
		}
	}
	return result, ok
}

// orNaN turns an undefined result into NaN for use in arithmetic.
func orNaN(v float64, ok bool) float64 {
	if !ok {
		return nan
	}
	return v
}

// minBy is _.minBy(array, iteratee). The iteratee returns NaN for undefined.
// The first minimum wins. ok is false when the result is undefined.
func minBy[T any](array []T, iteratee func(T) float64) (result T, ok bool) {
	var computed float64
	for _, value := range array {
		current := iteratee(value)
		if current != current {
			continue
		}
		if !ok || current < computed {
			computed, result, ok = current, value, true
		}
	}
	return result, ok
}

// sortKey is a sort criterion of _.sortBy: a number or undefined.
type sortKey struct {
	v       float64
	defined bool
}

func numKey(v float64) sortKey { return sortKey{v: v, defined: true} }

// compareAscending is lodash compareAscending for numbers and undefined.
// It is not consistent for NaN: NaN is greater than every other value, also
// than another NaN.
func compareAscending(value, other sortKey) float64 {
	same := value.defined == other.defined && (!value.defined || value.v == other.v)
	if !same {
		valIsDefined := value.defined
		valIsReflexive := !value.defined || value.v == value.v

		othIsDefined := other.defined
		othIsReflexive := !other.defined || other.v == other.v

		if (valIsDefined && othIsDefined && value.v > other.v) ||
			(!valIsDefined && othIsReflexive) ||
			!valIsReflexive {
			return 1
		}
		if (valIsDefined && othIsDefined && value.v < other.v) ||
			(!othIsDefined && valIsReflexive) ||
			!othIsReflexive {
			return -1
		}
	}
	return 0
}

// sortBy is _.sortBy(collection, iteratee) for one criterion. It returns a
// new slice. The sort is Array.prototype.sort of V8 with the lodash
// comparator: the criterion first, then the original index.
func sortBy[T any](mode js.V8Sort, collection []T, iteratee func(T) sortKey) []T {
	type item struct {
		criteria sortKey
		index    int
		value    T
	}
	result := make([]item, len(collection))
	for i, value := range collection {
		result[i] = item{criteria: iteratee(value), index: i, value: value}
	}
	js.SortV8(mode, result, func(object, other item) float64 {
		if r := compareAscending(object.criteria, other.criteria); r != 0 {
			return r
		}
		return float64(object.index - other.index)
	})
	out := make([]T, len(result))
	for i := range result {
		out[i] = result[i].value
	}
	return out
}

// arrayIndex converts a number to the index of an array element. ok is
// false when the number is not an array index: JavaScript then reads or
// writes a property that is not an element.
func arrayIndex(x float64) (int, bool) {
	if x >= 0 && x < 4294967295 && x == math.Trunc(x) {
		return int(x), true
	}
	return 0, false
}
