// Ported from d3-scale 4.0.2, d3-scale/src/linear.js and
// d3-scale/src/tickFormat.js (ISC).

package scale

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/array"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/format"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/interpolate"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Linear is d3.scaleLinear. R is the type of the range values: float64 for
// NewLinear, and for example string for a range of colors.
type Linear[R any] struct {
	continuous[R]
}

// NewLinear is d3.scaleLinear(): a scale with the domain [0, 1] and the
// range [0, 1]. The unknown value is NaN, which stands for undefined.
func NewLinear() *Linear[float64] {
	s := &Linear[float64]{}
	s.domain = []float64{0, 1}
	s.rng = []float64{0, 1}
	s.interpolate = interpolate.Number
	s.unknown = math.NaN()
	s.number = identityNumber
	return s
}

// NewLinearOf is d3.scaleLinear().interpolate(interpolate) for a range that
// does not hold numbers, for example colors with interpolate.HCL. The range
// is empty until Range sets it. The unknown value is the zero value of R.
func NewLinearOf[R any](interpolate func(a, b R) func(t float64) R) *Linear[R] {
	s := &Linear[R]{}
	s.domain = []float64{0, 1}
	s.interpolate = interpolate
	s.number = notANumber[R]
	return s
}

// Domain is scale.domain(values).
func (s *Linear[R]) Domain(values ...float64) *Linear[R] { s.setDomain(values); return s }

// DomainValues is scale.domain(): a copy of the domain.
func (s *Linear[R]) DomainValues() []float64 { return append([]float64(nil), s.domain...) }

// Range is scale.range(values).
func (s *Linear[R]) Range(values ...R) *Linear[R] { s.setRange(values); return s }

// RangeValues is scale.range(): a copy of the range.
func (s *Linear[R]) RangeValues() []R { return append([]R(nil), s.rng...) }

// RangeRound is scale.rangeRound(values): it sets the range and rounds the
// output to integers. It needs a range of numbers.
func (s *Linear[R]) RangeRound(values ...R) *Linear[R] { s.setRangeRound(values); return s }

// Clamp is scale.clamp(clamp).
func (s *Linear[R]) Clamp(clamp bool) *Linear[R] { s.clamp = clamp; s.rescale(); return s }

// Clamped is scale.clamp().
func (s *Linear[R]) Clamped() bool { return s.clamp }

// Interpolate is scale.interpolate(interpolate).
func (s *Linear[R]) Interpolate(interpolate func(a, b R) func(t float64) R) *Linear[R] {
	s.interpolate = interpolate
	s.rescale()
	return s
}

// Unknown is scale.unknown(value): the output for NaN.
func (s *Linear[R]) Unknown(value R) *Linear[R] { s.unknown = value; return s }

// Scale is scale(x).
func (s *Linear[R]) Scale(x float64) R { return s.scale(x) }

// Invert is scale.invert(y). The result is NaN when the range does not hold
// numbers.
func (s *Linear[R]) Invert(y float64) float64 { return s.invert(y) }

// Copy is scale.copy().
func (s *Linear[R]) Copy() *Linear[R] {
	c := &Linear[R]{}
	s.copyTo(&c.continuous)
	return c
}

// countOr10 is the JavaScript expression count == null ? 10 : count for an
// optional argument.
func countOr10(count []float64) float64 {
	if len(count) == 0 {
		return 10
	}
	return count[0]
}

// Ticks is scale.ticks(count). Without an argument, the count is 10.
func (s *Linear[R]) Ticks(count ...float64) []float64 {
	d := s.domain
	return array.Ticks(at(d, 0), at(d, len(d)-1), countOr10(count))
}

// TickFormat is scale.tickFormat(count, specifier). Without a specifier, d3
// uses ",f" with a precision that fits the tick step.
func (s *Linear[R]) TickFormat(count float64, specifier ...string) (format.Formatter, error) {
	d := s.domain
	return TickFormat(at(d, 0), at(d, len(d)-1), count, specifier...)
}

// Nice is scale.nice(count). Without an argument, the count is 10.
func (s *Linear[R]) Nice(count ...float64) *Linear[R] {
	c := countOr10(count)

	d := s.DomainValues()
	if len(d) == 0 {
		return s
	}
	i0 := 0
	i1 := len(d) - 1
	start := d[i0]
	stop := d[i1]
	prestep := math.NaN() // undefined
	hasPrestep := false

	if stop < start {
		start, stop = stop, start
		i0, i1 = i1, i0
	}

	for maxIter := 10; maxIter > 0; maxIter-- {
		step := array.TickIncrement(start, stop, c)
		if hasPrestep && step == prestep {
			d[i0] = start
			d[i1] = stop
			s.setDomain(d)
			return s
		} else if step > 0 {
			start = float64(math.Floor(start/step) * step)
			stop = float64(math.Ceil(stop/step) * step)
		} else if step < 0 {
			start = math.Ceil(start*step) / step
			stop = math.Floor(stop*step) / step
		} else {
			break
		}
		prestep, hasPrestep = step, true
	}

	return s
}

// TickFormat is the tickFormat function of d3-scale (tickFormat.js): a
// number format for the ticks of a domain from start to stop. Without a
// specifier, the format is ",f".
func TickFormat(start, stop, count float64, specifier ...string) (format.Formatter, error) {
	step := array.TickStep(start, stop, count)
	text := ",f"
	if len(specifier) > 0 {
		text = specifier[0]
	}
	spec, err := format.ParseSpecifier(text)
	if err != nil {
		return nil, err
	}
	switch spec.Type {
	case "s":
		value := js.Max(math.Abs(start), math.Abs(stop))
		if math.IsNaN(spec.Precision) {
			if precision := format.PrecisionPrefix(step, value); !math.IsNaN(precision) {
				spec.Precision = precision
			}
		}
		return format.FormatPrefixSpecifier(spec, value)
	case "", "e", "g", "p", "r":
		if math.IsNaN(spec.Precision) {
			if precision := format.PrecisionRound(step, js.Max(math.Abs(start), math.Abs(stop))); !math.IsNaN(precision) {
				spec.Precision = precision
				if spec.Type == "e" {
					spec.Precision = precision - 1
				}
			}
		}
	case "f", "%":
		if math.IsNaN(spec.Precision) {
			if precision := format.PrecisionFixed(step); !math.IsNaN(precision) {
				spec.Precision = precision
				if spec.Type == "%" {
					spec.Precision = precision - 2
				}
			}
		}
	}
	return format.FormatSpecifier(spec)
}
