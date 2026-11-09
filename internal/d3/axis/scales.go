// This file ports no upstream file. It adapts the scales of
// internal/d3/scale to the Scale interface of the axis: the methods that
// d3-axis calls on a scale object.

package axis

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/scale"
	d3time "github.com/bvolpato/mermaid-go-renderer/internal/d3/time"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// count reads the first tick argument as the count of scale.ticks(count).
// ok is false when the argument is absent or null.
func count(args []any) (float64, bool) {
	if len(args) == 0 || args[0] == nil {
		return 0, false
	}
	switch v := args[0].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}

// specifier reads the second tick argument as the specifier of
// scale.tickFormat(count, specifier).
func specifier(args []any) (string, bool) {
	if len(args) < 2 || args[1] == nil {
		return "", false
	}
	s, ok := args[1].(string)
	return s, ok
}

type timeScale struct{ s *scale.Time }

// TimeScale adapts d3.scaleTime and d3.scaleUtc. A domain value is a time
// value: milliseconds since the epoch. The first tick argument is a count
// (float64) or a time interval (*time.Interval of internal/d3/time, where a
// nil pointer is the null of interval.every). The second tick argument is a
// format specifier.
func TimeScale(s *scale.Time) Scale[float64] { return timeScale{s} }

func (t timeScale) Ticks(args []any) ([]float64, bool) {
	if len(args) > 0 {
		if iv, ok := args[0].(*d3time.Interval); ok {
			return t.s.TicksInterval(iv), true
		}
	}
	if c, ok := count(args); ok {
		return t.s.Ticks(c), true
	}
	return t.s.Ticks(), true
}

func (t timeScale) Domain() []float64 { return t.s.DomainValues() }

func (t timeScale) TickFormat(args []any) (func(float64) string, bool, error) {
	if spec, ok := specifier(args); ok {
		return t.s.TickFormat(spec), true, nil
	}
	return t.s.TickFormat(), true, nil
}

func (t timeScale) Range() []float64           { return t.s.RangeValues() }
func (t timeScale) Bandwidth() (float64, bool) { return 0, false }
func (t timeScale) Round() bool                { return false }
func (t timeScale) Copy() Scale[float64]       { return timeScale{t.s.Copy()} }
func (t timeScale) Apply(d float64) float64    { return t.s.Scale(d) }
func (t timeScale) String(d float64) string    { return js.Str(d) }

type linearScale struct{ s *scale.Linear[float64] }

// LinearScale adapts d3.scaleLinear. The first tick argument is the count
// and the second is a format specifier.
func LinearScale(s *scale.Linear[float64]) Scale[float64] { return linearScale{s} }

func (l linearScale) Ticks(args []any) ([]float64, bool) {
	if c, ok := count(args); ok {
		return l.s.Ticks(c), true
	}
	return l.s.Ticks(), true
}

func (l linearScale) Domain() []float64 { return l.s.DomainValues() }

func (l linearScale) TickFormat(args []any) (func(float64) string, bool, error) {
	c, ok := count(args)
	if !ok {
		c = 10
	}
	var spec []string
	if s, ok := specifier(args); ok {
		spec = []string{s}
	}
	f, err := l.s.TickFormat(c, spec...)
	if err != nil {
		return nil, true, err
	}
	return f, true, nil
}

func (l linearScale) Range() []float64           { return l.s.RangeValues() }
func (l linearScale) Bandwidth() (float64, bool) { return 0, false }
func (l linearScale) Round() bool                { return false }
func (l linearScale) Copy() Scale[float64]       { return linearScale{l.s.Copy()} }
func (l linearScale) Apply(d float64) float64    { return l.s.Scale(d) }
func (l linearScale) String(d float64) string    { return js.Str(d) }

type bandScale[D comparable] struct {
	s   *scale.Band[D]
	str func(D) string
}

// BandScale adapts d3.scaleBand. str is String(d) for a domain value.
func BandScale[D comparable](s *scale.Band[D], str func(D) string) Scale[D] {
	return bandScale[D]{s, str}
}

func (b bandScale[D]) Ticks([]any) ([]D, bool) { return nil, false }
func (b bandScale[D]) Domain() []D             { return b.s.DomainValues() }
func (b bandScale[D]) TickFormat([]any) (func(D) string, bool, error) {
	return nil, false, nil
}
func (b bandScale[D]) Range() []float64 {
	r0, r1 := b.s.RangeValues()
	return []float64{r0, r1}
}
func (b bandScale[D]) Bandwidth() (float64, bool) { return b.s.Bandwidth(), true }
func (b bandScale[D]) Round() bool                { return b.s.Rounded() }
func (b bandScale[D]) Copy() Scale[D]             { return bandScale[D]{b.s.Copy(), b.str} }
func (b bandScale[D]) Apply(d D) float64          { return b.s.Scale(d) }
func (b bandScale[D]) String(d D) string          { return b.str(d) }

type pointScale[D comparable] struct {
	s     *scale.Point[D]
	str   func(D) string
	round bool
}

// PointScale adapts d3.scalePoint. str is String(d) for a domain value.
// round is scale.round(): scale.Point has no getter for it, so the caller
// passes the value that it gave to Round.
func PointScale[D comparable](s *scale.Point[D], str func(D) string, round bool) Scale[D] {
	return pointScale[D]{s, str, round}
}

func (p pointScale[D]) Ticks([]any) ([]D, bool) { return nil, false }
func (p pointScale[D]) Domain() []D             { return p.s.DomainValues() }
func (p pointScale[D]) TickFormat([]any) (func(D) string, bool, error) {
	return nil, false, nil
}
func (p pointScale[D]) Range() []float64 {
	r0, r1 := p.s.RangeValues()
	return []float64{r0, r1}
}
func (p pointScale[D]) Bandwidth() (float64, bool) { return p.s.Bandwidth(), true }
func (p pointScale[D]) Round() bool                { return p.round }
func (p pointScale[D]) Copy() Scale[D]             { return pointScale[D]{p.s.Copy(), p.str, p.round} }
func (p pointScale[D]) Apply(d D) float64          { return p.s.Scale(d) }
func (p pointScale[D]) String(d D) string          { return p.str(d) }
