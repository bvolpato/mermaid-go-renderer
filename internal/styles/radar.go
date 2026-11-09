// Ported from mermaid 12.2.0, mermaid/src/diagrams/radar/styles.ts (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// radarGenIndexStyles is genIndexStyles: the rules of each curve and legend box.
func radarGenIndexStyles(themeVariables, radarOptions *js.Obj[any]) string {
	var sections text
	for i, n := 0, loopCount(themeVariables.Get("THEME_COLOR_LIMIT")); i < n; i++ {
		indexColor := themeVariables.Get("cScale" + itoa(i))
		sections.add(`
		.radarCurve-` + itoa(i) + ` {
			color: ` + s(indexColor) + `;
			fill: ` + s(indexColor) + `;
			fill-opacity: ` + s(radarOptions.Get("curveOpacity")) + `;
			stroke: ` + s(indexColor) + `;
			stroke-width: ` + s(radarOptions.Get("curveStrokeWidth")) + `;
		}
		.radarLegendBox-` + itoa(i) + ` {
			fill: ` + s(indexColor) + `;
			fill-opacity: ` + s(radarOptions.Get("curveOpacity")) + `;
			stroke: ` + s(indexColor) + `;
		}
		`)
	}
	return sections.String()
}

// radarBuildStyleOptions is buildRadarStyleOptions: the theme variables of
// the config merged over the default theme, and the radar options.
func radarBuildStyleOptions(in *options, radar any) (themeVariables, radarOptions *js.Obj[any]) {
	defaultThemeVariables := getThemeVariables()
	currentConfig := in.config

	themeVariables = cleanAndMerge(defaultThemeVariables, currentConfig.Get("themeVariables"))
	radarOptions = cleanAndMerge(themeVariables.Get("radar"), radar)

	return themeVariables, radarOptions
}

// radarStyles reads its options from the radar theme variable.
func radarStyles(in *options) string {
	themeVariables, radarOptions := radarBuildStyleOptions(in, in.Get("radar"))
	return `
	.radarTitle {
		font-size: ` + s(themeVariables.Get("fontSize")) + `;
		color: ` + s(themeVariables.Get("titleColor")) + `;
		dominant-baseline: hanging;
		text-anchor: middle;
	}
	.radarAxisLine {
		stroke: ` + s(radarOptions.Get("axisColor")) + `;
		stroke-width: ` + s(radarOptions.Get("axisStrokeWidth")) + `;
	}
	.radarAxisLabel {
		font-size: ` + s(radarOptions.Get("axisLabelFontSize")) + `px;
		color: ` + s(radarOptions.Get("axisColor")) + `;
	}
	.radarGraticule {
		fill: ` + s(radarOptions.Get("graticuleColor")) + `;
		fill-opacity: ` + s(radarOptions.Get("graticuleOpacity")) + `;
		stroke: ` + s(radarOptions.Get("graticuleColor")) + `;
		stroke-width: ` + s(radarOptions.Get("graticuleStrokeWidth")) + `;
	}
	.radarLegendText {
		text-anchor: start;
		font-size: ` + s(radarOptions.Get("legendFontSize")) + `px;
		dominant-baseline: hanging;
	}
	` + radarGenIndexStyles(themeVariables, radarOptions) + `
	`
}
