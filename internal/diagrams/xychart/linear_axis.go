// Ported from mermaid 12.2.0,
// diagrams/xychart/chartBuilder/components/axis/linearAxis.ts (MIT).

package xychart

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/scale"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// linearAxis is LinearAxis.
type linearAxis struct {
	*baseAxis
	scale  *scale.Linear[float64]
	domain [2]float64
}

func newLinearAxis(
	axisConfig any,
	axisThemeConfig xyChartAxisThemeConfig,
	domain [2]float64,
	title string,
	textDimensionCalculator textDimensionCalculator,
) (*linearAxis, error) {
	a := &linearAxis{}
	base, err := newBaseAxis(a, axisConfig, title, textDimensionCalculator, axisThemeConfig)
	if err != nil {
		return nil, err
	}
	a.baseAxis = base
	a.domain = domain
	rng := a.getRange()
	a.scale = scale.NewLinear().Domain(a.domain[0], a.domain[1]).Range(rng[0], rng[1])
	return a, nil
}

func (a *linearAxis) getTickValues() []any {
	values := a.scale.Ticks()
	ticks := make([]any, len(values))
	for i, v := range values {
		ticks[i] = v
	}
	return ticks
}

func (a *linearAxis) recalculateScale() {
	// Copy the array so if reverse is called two times it should not cancel
	// the reverse effect.
	domain := a.domain
	if a.axisPosition == "left" {
		// Since y-axis in svg start from top.
		domain[0], domain[1] = domain[1], domain[0]
	}
	rng := a.getRange()
	a.scale = scale.NewLinear().Domain(domain[0], domain[1]).Range(rng[0], rng[1])
}

func (a *linearAxis) getScaleValue(value any) any {
	// The scale of d3 returns its unknown value, which is undefined, for
	// null, for undefined and for a value that is not a number.
	if js.IsNullish(value) {
		return nil
	}
	x := num(value)
	if math.IsNaN(x) {
		return nil
	}
	return a.scale.Scale(x)
}
