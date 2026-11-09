// Ported from mermaid 12.2.0,
// diagrams/xychart/chartBuilder/components/axis/bandAxis.ts (MIT).

package xychart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/scale"
)

// bandAxis is BandAxis.
type bandAxis struct {
	*baseAxis
	scale      *scale.Band[string]
	categories []string
	// known is the set of the categories: the values for which the scale
	// does not return undefined.
	known map[string]bool
}

func newBandAxis(
	axisConfig any,
	axisThemeConfig xyChartAxisThemeConfig,
	categories []string,
	title string,
	textDimensionCalculator textDimensionCalculator,
) (*bandAxis, error) {
	a := &bandAxis{}
	base, err := newBaseAxis(a, axisConfig, title, textDimensionCalculator, axisThemeConfig)
	if err != nil {
		return nil, err
	}
	a.baseAxis = base
	a.categories = categories
	a.known = make(map[string]bool, len(categories))
	for _, c := range categories {
		a.known[c] = true
	}
	rng := a.getRange()
	a.scale = scale.NewBand[string]().Domain(a.categories...).Range(rng[0], rng[1])
	return a, nil
}

func (a *bandAxis) recalculateScale() {
	rng := a.getRange()
	a.scale = scale.NewBand[string]().
		Domain(a.categories...).
		Range(rng[0], rng[1]).
		PaddingInner(1).
		PaddingOuter(0).
		Align(0.5)
}

func (a *bandAxis) getTickValues() []any {
	ticks := make([]any, len(a.categories))
	for i, c := range a.categories {
		ticks[i] = c
	}
	return ticks
}

func (a *bandAxis) getScaleValue(value any) any {
	// this.scale(value) ?? this.getRange()[0]. The scale returns undefined
	// for a value that is not in its domain.
	if s, ok := value.(string); ok && a.known[s] {
		return a.scale.Scale(s)
	}
	return a.getRange()[0]
}
