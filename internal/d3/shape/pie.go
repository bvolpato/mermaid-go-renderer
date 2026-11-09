// Ported from d3-shape 3.2.0, d3-shape/src/pie.js and
// d3-shape/src/descending.js (ISC).

package shape

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// PieArcDatum is one arc of the result of a pie generator.
type PieArcDatum[T any] struct {
	Data       T
	Index      int
	Value      float64
	StartAngle float64
	EndAngle   float64
	PadAngle   float64
}

// ArcAngles implements AngleDatum, so that an Arc reads the angles of a pie
// arc by default.
func (d PieArcDatum[T]) ArcAngles() (startAngle, endAngle, padAngle float64) {
	return d.StartAngle, d.EndAngle, d.PadAngle
}

// descending is the comparator of descending.js. It returns NaN when a value
// is NaN.
func descending(a, b float64) float64 {
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

// Pie is the pie generator of d3 (d3.pie). T is the type of a datum.
type Pie[T any] struct {
	value      func(d T, i int, data []T) float64
	sortValues func(a, b float64) float64 // nil is the null of d3
	sort       func(a, b T) float64       // nil is the null of d3
	startAngle float64
	endAngle   float64
	padAngle   float64
	sortMode   js.V8Sort
}

// NewPie is d3.pie(). The default value accessor is the datum itself, when
// the datum is a float64, and NaN otherwise. The default order is descending
// by value.
func NewPie[T any]() *Pie[T] {
	return &Pie[T]{
		value: func(d T, _ int, _ []T) float64 {
			if v, ok := any(d).(float64); ok {
				return v
			}
			return math.NaN()
		},
		sortValues: descending,
		startAngle: 0,
		endAngle:   tau,
		padAngle:   0,
		sortMode:   js.V8PowerSort,
	}
}

// Value is pie.value(accessor).
func (p *Pie[T]) Value(value func(d T, i int, data []T) float64) *Pie[T] {
	p.value = value
	return p
}

// SortValues is pie.sortValues(comparator): it orders the arcs by value and
// clears the data comparator. A nil comparator is pie.sortValues(null).
func (p *Pie[T]) SortValues(compare func(a, b float64) float64) *Pie[T] {
	p.sortValues, p.sort = compare, nil
	return p
}

// Sort is pie.sort(comparator): it orders the arcs by datum and clears the
// value comparator. A nil comparator is pie.sort(null): the arcs keep the
// order of the data.
func (p *Pie[T]) Sort(compare func(a, b T) float64) *Pie[T] {
	p.sort, p.sortValues = compare, nil
	return p
}

// StartAngle is pie.startAngle(number).
func (p *Pie[T]) StartAngle(angle float64) *Pie[T] { p.startAngle = angle; return p }

// EndAngle is pie.endAngle(number).
func (p *Pie[T]) EndAngle(angle float64) *Pie[T] { p.endAngle = angle; return p }

// PadAngle is pie.padAngle(number).
func (p *Pie[T]) PadAngle(angle float64) *Pie[T] { p.padAngle = angle; return p }

// SortAlgorithm selects the Array.prototype.sort of a V8 version. The
// versions order the arcs differently only when the comparator is not
// consistent, for example when a value is NaN. The default is the algorithm
// of current Chromium (js.V8PowerSort).
func (p *Pie[T]) SortAlgorithm(mode js.V8Sort) *Pie[T] { p.sortMode = mode; return p }

// Arcs is pie(data). The arcs are in the order of the data.
func (p *Pie[T]) Arcs(data []T) []PieArcDatum[T] {
	n := len(data)
	sum := 0.0
	index := make([]int, n)
	values := make([]float64, n)
	arcs := make([]PieArcDatum[T], n)
	a0 := p.startAngle
	da := js.Min(tau, js.Max(-tau, p.endAngle-a0))
	pad := js.Min(math.Abs(da)/float64(n), p.padAngle)
	pa := pad
	if da < 0 {
		pa = float64(pad * -1)
	}

	for i := 0; i < n; i++ {
		index[i] = i
		v := p.value(data[i], i, data)
		values[i] = v
		if v > 0 {
			sum += v
		}
	}

	// Optionally sort the arcs by previously-computed values or by data.
	if p.sortValues != nil {
		js.SortV8(p.sortMode, index, func(i, j int) float64 { return p.sortValues(values[i], values[j]) })
	} else if p.sort != nil {
		js.SortV8(p.sortMode, index, func(i, j int) float64 { return p.sort(data[i], data[j]) })
	}

	// Compute the arcs! They are stored in the original data's order.
	k := 0.0
	if truthy(sum) {
		k = (da - float64(float64(n)*pa)) / sum
	}
	for i := 0; i < n; i++ {
		j := index[i]
		v := values[j]
		a1 := a0
		if v > 0 {
			a1 += float64(v * k)
		} else {
			a1 += 0
		}
		a1 += pa
		arcs[j] = PieArcDatum[T]{
			Data:       data[j],
			Index:      i,
			Value:      v,
			StartAngle: a0,
			EndAngle:   a1,
			PadAngle:   pad,
		}
		a0 = a1
	}

	return arcs
}
