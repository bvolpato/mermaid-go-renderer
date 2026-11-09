// Ported from mermaid 12.2.0, mermaid/src/diagrams/treemap/styles.ts (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// defaultTreemapStyleOptions is the constant of the same name.
func defaultTreemapStyleOptions() *js.Obj[any] {
	return objOf(
		"sectionStrokeColor", "black",
		"sectionStrokeWidth", "1",
		"sectionFillColor", "#efefef",
		"leafStrokeColor", "black",
		"leafStrokeWidth", "1",
		"leafFillColor", "#efefef",
		"labelFontSize", "12px",
		"valueFontSize", "10px",
		"titleFontSize", "14px",
	)
}

// treemapStyles reads its options from the treemap theme variable. The
// colours that the options do not set come from the theme variables of the
// config, merged over the default theme.
func treemapStyles(in *options) string {
	defaultThemeVariables := getThemeVariables()
	currentConfig := in.config
	// themeVariables is cleanAndMerge(defaultThemeVariables,
	// currentConfig.themeVariables). The template reads two of them.
	themeVariable := func(key string) any {
		return mergedProperty(defaultThemeVariables, currentConfig.Get("themeVariables"), key)
	}

	o := &options{base: cleanAndMerge(defaultTreemapStyleOptions(), in.Get("treemap"))}

	titleColor := s(nn(o.Get("titleColor"), themeVariable("titleColor")))
	labelColor := s(nn(o.Get("labelColor"), themeVariable("textColor")))
	valueColor := s(nn(o.Get("valueColor"), themeVariable("textColor")))

	return `
  .treemapNode.section {
    stroke: ` + o.s("sectionStrokeColor") + `;
    stroke-width: ` + o.s("sectionStrokeWidth") + `;
    fill: ` + o.s("sectionFillColor") + `;
  }
  .treemapNode.leaf {
    stroke: ` + o.s("leafStrokeColor") + `;
    stroke-width: ` + o.s("leafStrokeWidth") + `;
    fill: ` + o.s("leafFillColor") + `;
  }
  .treemapLabel {
    fill: ` + labelColor + `;
    font-size: ` + o.s("labelFontSize") + `;
  }
  .treemapValue {
    fill: ` + valueColor + `;
    font-size: ` + o.s("valueFontSize") + `;
  }
  .treemapTitle {
    fill: ` + titleColor + `;
    font-size: ` + o.s("titleFontSize") + `;
  }
  `
}
