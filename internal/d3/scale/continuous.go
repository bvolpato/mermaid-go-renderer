// Ported from d3-scale 4.0.2, d3-scale/src/continuous.js (ISC).

// Package scale is a port of the scales of d3-scale 4.0.2 that mermaid uses:
// linear, band, point, ordinal, time and UTC.
package scale

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/array"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/interpolate"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// continuous is the scale that transformer()(identity, identity) returns in
// d3. R is the type of the range values.
type continuous[R any] struct {
	domain      []float64
	rng         []R
	interpolate func(a, b R) func(t float64) R
	clamp       bool
	unknown     R
	// number converts a range value to a number, for invert.
	number func(v R) float64
	// The piecewise functions, built when a method needs them.
	output func(x float64) R
	input  func(y float64) float64
}

// normalize(a, b)(x) takes a domain value x in [a,b] and returns the
// corresponding parameter t in [0,1].
func normalize(a, b float64) func(x float64) float64 {
	b -= a
	if b != 0 && !math.IsNaN(b) {
		return func(x float64) float64 { return (x - a) / b }
	}
	c := 0.5
	if math.IsNaN(b) {
		c = math.NaN()
	}
	return func(float64) float64 { return c }
}

func clamper(a, b float64) func(x float64) float64 {
	if a > b {
		a, b = b, a
	}
	return func(x float64) float64 { return js.Max(a, js.Min(b, x)) }
}

// at is a[i] in JavaScript: NaN for an index that does not exist.
func at(a []float64, i int) float64 {
	if i >= 0 && i < len(a) {
		return a[i]
	}
	return math.NaN()
}

// bimap ports bimap. The domain and the range have at least two values.
func bimap[R any](domain []float64, rng []R, interpolate func(a, b R) func(t float64) R) func(x float64) R {
	d0, d1, r0, r1 := domain[0], domain[1], rng[0], rng[1]
	var d func(float64) float64
	var r func(float64) R
	if d1 < d0 {
		d, r = normalize(d1, d0), interpolate(r1, r0)
	} else {
		d, r = normalize(d0, d1), interpolate(r0, r1)
	}
	return func(x float64) R { return r(d(x)) }
}

// polymap ports polymap.
func polymap[R any](domain []float64, rng []R, interpolate func(a, b R) func(t float64) R) func(x float64) R {
	j := len(domain)
	if len(rng) < j {
		j = len(rng)
	}
	j--
	d := make([]func(float64) float64, j)
	r := make([]func(float64) R, j)

	// Reverse descending domains.
	if domain[j] < domain[0] {
		domain = reversed(domain)
		rng = reversed(rng)
	}

	for i := 0; i < j; i++ {
		d[i] = normalize(domain[i], domain[i+1])
		r[i] = interpolate(rng[i], rng[i+1])
	}

	return func(x float64) R {
		i := array.BisectRight(domain, x, 1, j) - 1
		return r[i](d[i](x))
	}
}

func reversed[T any](s []T) []T {
	out := make([]T, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}

func (s *continuous[R]) rescale() {
	s.output, s.input = nil, nil
}

// size is Math.min(domain.length, range.length).
func (s *continuous[R]) size() int {
	n := len(s.domain)
	if len(s.rng) < n {
		n = len(s.rng)
	}
	return n
}

func (s *continuous[R]) clampValue(x float64) float64 {
	if !s.clamp {
		return x
	}
	n := s.size()
	return clamper(at(s.domain, 0), at(s.domain, n-1))(x)
}

// scale ports the scale function. The result is the unknown value for NaN,
// which also stands for null and undefined.
//
// d3 fails or returns undefined for a scale with fewer than two domain or
// range values. The port returns the unknown value.
func (s *continuous[R]) scale(x float64) R {
	if math.IsNaN(x) {
		return s.unknown
	}
	n := s.size()
	if n < 2 {
		return s.unknown
	}
	if s.output == nil {
		if n > 2 {
			s.output = polymap(s.domain, s.rng, s.interpolate)
		} else {
			s.output = bimap(s.domain, s.rng, s.interpolate)
		}
	}
	return s.output(s.clampValue(x))
}

// invert ports scale.invert.
func (s *continuous[R]) invert(y float64) float64 {
	n := s.size()
	if n < 2 {
		return math.NaN()
	}
	if s.input == nil {
		numbers := make([]float64, len(s.rng))
		for i, v := range s.rng {
			numbers[i] = s.number(v)
		}
		if n > 2 {
			s.input = polymap(numbers, s.domain, interpolate.Number)
		} else {
			s.input = bimap(numbers, s.domain, interpolate.Number)
		}
	}
	return s.clampValue(s.input(y))
}

func (s *continuous[R]) setDomain(values []float64) {
	s.domain = append([]float64(nil), values...)
	s.rescale()
}

func (s *continuous[R]) setRange(values []R) {
	s.rng = append([]R(nil), values...)
	s.rescale()
}

// setRangeRound ports rangeRound: it sets the range and the interpolator
// interpolateRound. The interpolator exists only for a range of numbers.
func (s *continuous[R]) setRangeRound(values []R) {
	s.rng = append([]R(nil), values...)
	if f, ok := any(interpolate.Round).(func(a, b R) func(t float64) R); ok {
		s.interpolate = f
	}
	s.rescale()
}

// copyTo ports copy(source, target).
func (s *continuous[R]) copyTo(target *continuous[R]) {
	target.setDomain(s.domain)
	target.setRange(s.rng)
	target.interpolate = s.interpolate
	target.clamp = s.clamp
	target.unknown = s.unknown
	target.number = s.number
	target.rescale()
}

func identityNumber(v float64) float64 { return v }

func notANumber[R any](R) float64 { return math.NaN() }
