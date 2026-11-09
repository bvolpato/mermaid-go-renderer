// Ported from d3-scale 4.0.2, d3-scale/src/band.js and
// d3-scale/src/ordinal.js (ISC), and from internmap 2.0.3,
// internmap/src/index.js (ISC).

package scale

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/array"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

type nanKey struct{}

// internKey gives a domain value the identity that it has as a key of a
// JavaScript Map: every NaN is the same key, and -0 is the same key as 0.
func internKey[D comparable](d D) any {
	if f, ok := any(d).(float64); ok {
		if math.IsNaN(f) {
			return nanKey{}
		}
		if f == 0 {
			return float64(0)
		}
	}
	return d
}

// Ordinal is d3.scaleOrdinal. D is the type of the domain values and R the
// type of the range values.
type Ordinal[D comparable, R any] struct {
	index  map[any]int
	domain []D
	rng    []R
	// implicit is true while the scale adds an unknown value to the domain.
	implicit bool
	unknown  R
}

// NewOrdinal is d3.scaleOrdinal(): a scale with an empty domain and an empty
// range. A value that is not in the domain is added to the domain.
func NewOrdinal[D comparable, R any]() *Ordinal[D, R] {
	return &Ordinal[D, R]{index: map[any]int{}, implicit: true}
}

// Scale is scale(d). The result is the zero value of R where d3 returns
// undefined: for an empty range.
func (s *Ordinal[D, R]) Scale(d D) R {
	key := internKey(d)
	i, ok := s.index[key]
	if !ok {
		if !s.implicit {
			return s.unknown
		}
		s.domain = append(s.domain, d)
		i = len(s.domain) - 1
		s.index[key] = i
	}
	if len(s.rng) == 0 {
		var zero R
		return zero
	}
	return s.rng[i%len(s.rng)]
}

// Domain is scale.domain(values). A value that is in the list twice counts
// once.
func (s *Ordinal[D, R]) Domain(values ...D) *Ordinal[D, R] {
	s.domain, s.index = nil, map[any]int{}
	for _, value := range values {
		key := internKey(value)
		if _, ok := s.index[key]; ok {
			continue
		}
		s.domain = append(s.domain, value)
		s.index[key] = len(s.domain) - 1
	}
	return s
}

// DomainValues is scale.domain(): a copy of the domain.
func (s *Ordinal[D, R]) DomainValues() []D { return append([]D(nil), s.domain...) }

// Range is scale.range(values).
func (s *Ordinal[D, R]) Range(values ...R) *Ordinal[D, R] {
	s.rng = append([]R(nil), values...)
	return s
}

// RangeValues is scale.range(): a copy of the range.
func (s *Ordinal[D, R]) RangeValues() []R { return append([]R(nil), s.rng...) }

// Unknown is scale.unknown(value): the output for a value that is not in the
// domain. The scale then does not add such a value to the domain.
func (s *Ordinal[D, R]) Unknown(value R) *Ordinal[D, R] {
	s.unknown, s.implicit = value, false
	return s
}

// Copy is scale.copy().
func (s *Ordinal[D, R]) Copy() *Ordinal[D, R] {
	c := NewOrdinal[D, R]().Domain(s.domain...).Range(s.rng...)
	c.unknown, c.implicit = s.unknown, s.implicit
	return c
}

// Band is d3.scaleBand. D is the type of the domain values.
type Band[D comparable] struct {
	ordinal      *Ordinal[D, float64]
	r0, r1       float64
	step         float64
	bandwidth    float64
	round        bool
	paddingInner float64
	paddingOuter float64
	align        float64
}

// NewBand is d3.scaleBand(): a scale with an empty domain and the range
// [0, 1].
func NewBand[D comparable]() *Band[D] {
	s := &Band[D]{
		ordinal: NewOrdinal[D, float64]().Unknown(math.NaN()),
		r0:      0,
		r1:      1,
		align:   0.5,
	}
	s.rescale()
	return s
}

func (s *Band[D]) rescale() *Band[D] {
	n := float64(len(s.ordinal.domain))
	reverse := s.r1 < s.r0
	start, stop := s.r0, s.r1
	if reverse {
		start, stop = s.r1, s.r0
	}
	s.step = (stop - start) / js.Max(1, n-s.paddingInner+float64(s.paddingOuter*2))
	if s.round {
		s.step = math.Floor(s.step)
	}
	start += float64((stop - start - float64(s.step*(n-s.paddingInner))) * s.align)
	s.bandwidth = float64(s.step * (1 - s.paddingInner))
	if s.round {
		start = js.Round(start)
		s.bandwidth = js.Round(s.bandwidth)
	}
	values := array.Range(0, n, 1)
	for i, v := range values {
		values[i] = start + float64(s.step*v)
	}
	if reverse {
		values = reversed(values)
	}
	s.ordinal.Range(values...)
	return s
}

// Scale is scale(d): the start of the band of d. The result is NaN, which
// stands for undefined, for a value that is not in the domain.
func (s *Band[D]) Scale(d D) float64 {
	if len(s.ordinal.rng) == 0 {
		return math.NaN()
	}
	return s.ordinal.Scale(d)
}

// Domain is scale.domain(values).
func (s *Band[D]) Domain(values ...D) *Band[D] {
	s.ordinal.Domain(values...)
	return s.rescale()
}

// DomainValues is scale.domain(): a copy of the domain.
func (s *Band[D]) DomainValues() []D { return s.ordinal.DomainValues() }

// Range is scale.range([r0, r1]).
func (s *Band[D]) Range(r0, r1 float64) *Band[D] {
	s.r0, s.r1 = r0, r1
	return s.rescale()
}

// RangeValues is scale.range().
func (s *Band[D]) RangeValues() (r0, r1 float64) { return s.r0, s.r1 }

// RangeRound is scale.rangeRound([r0, r1]).
func (s *Band[D]) RangeRound(r0, r1 float64) *Band[D] {
	s.r0, s.r1 = r0, r1
	s.round = true
	return s.rescale()
}

// Bandwidth is scale.bandwidth().
func (s *Band[D]) Bandwidth() float64 { return s.bandwidth }

// Step is scale.step().
func (s *Band[D]) Step() float64 { return s.step }

// Round is scale.round(round).
func (s *Band[D]) Round(round bool) *Band[D] {
	s.round = round
	return s.rescale()
}

// Rounded is scale.round().
func (s *Band[D]) Rounded() bool { return s.round }

// Padding is scale.padding(padding): it sets the inner and the outer padding.
func (s *Band[D]) Padding(padding float64) *Band[D] {
	s.paddingOuter = padding
	s.paddingInner = js.Min(1, padding)
	return s.rescale()
}

// PaddingInner is scale.paddingInner(padding).
func (s *Band[D]) PaddingInner(padding float64) *Band[D] {
	s.paddingInner = js.Min(1, padding)
	return s.rescale()
}

// PaddingInnerValue is scale.paddingInner(), which is also scale.padding().
func (s *Band[D]) PaddingInnerValue() float64 { return s.paddingInner }

// PaddingOuter is scale.paddingOuter(padding).
func (s *Band[D]) PaddingOuter(padding float64) *Band[D] {
	s.paddingOuter = padding
	return s.rescale()
}

// PaddingOuterValue is scale.paddingOuter().
func (s *Band[D]) PaddingOuterValue() float64 { return s.paddingOuter }

// Align is scale.align(align).
func (s *Band[D]) Align(align float64) *Band[D] {
	s.align = js.Max(0, js.Min(1, align))
	return s.rescale()
}

// AlignValue is scale.align().
func (s *Band[D]) AlignValue() float64 { return s.align }

// Copy is scale.copy().
func (s *Band[D]) Copy() *Band[D] {
	return NewBand[D]().Domain(s.ordinal.domain...).Range(s.r0, s.r1).
		Round(s.round).PaddingInner(s.paddingInner).PaddingOuter(s.paddingOuter).Align(s.align)
}

// Point is d3.scalePoint: a band scale with an inner padding of 1, so that
// the bandwidth is zero.
type Point[D comparable] struct {
	band *Band[D]
}

// NewPoint is d3.scalePoint().
func NewPoint[D comparable]() *Point[D] {
	return &Point[D]{band: NewBand[D]().PaddingInner(1)}
}

// Scale is scale(d): the position of d. The result is NaN, which stands for
// undefined, for a value that is not in the domain.
func (s *Point[D]) Scale(d D) float64 { return s.band.Scale(d) }

// Domain is scale.domain(values).
func (s *Point[D]) Domain(values ...D) *Point[D] { s.band.Domain(values...); return s }

// DomainValues is scale.domain().
func (s *Point[D]) DomainValues() []D { return s.band.DomainValues() }

// Range is scale.range([r0, r1]).
func (s *Point[D]) Range(r0, r1 float64) *Point[D] { s.band.Range(r0, r1); return s }

// RangeValues is scale.range().
func (s *Point[D]) RangeValues() (r0, r1 float64) { return s.band.RangeValues() }

// RangeRound is scale.rangeRound([r0, r1]).
func (s *Point[D]) RangeRound(r0, r1 float64) *Point[D] { s.band.RangeRound(r0, r1); return s }

// Bandwidth is scale.bandwidth(), which is zero.
func (s *Point[D]) Bandwidth() float64 { return s.band.Bandwidth() }

// Step is scale.step().
func (s *Point[D]) Step() float64 { return s.band.Step() }

// Round is scale.round(round).
func (s *Point[D]) Round(round bool) *Point[D] { s.band.Round(round); return s }

// Padding is scale.padding(padding): the outer padding.
func (s *Point[D]) Padding(padding float64) *Point[D] { s.band.PaddingOuter(padding); return s }

// PaddingValue is scale.padding().
func (s *Point[D]) PaddingValue() float64 { return s.band.PaddingOuterValue() }

// Align is scale.align(align).
func (s *Point[D]) Align(align float64) *Point[D] { s.band.Align(align); return s }

// Copy is scale.copy().
func (s *Point[D]) Copy() *Point[D] { return &Point[D]{band: s.band.Copy()} }
