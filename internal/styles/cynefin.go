// Ported from mermaid 12.2.0, mermaid/src/diagrams/cynefin/styles.ts (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// cynefinGetTheme is getCynefinTheme: the cynefin block of the theme
// variables of the config, merged over the default theme. The result is any
// value, because a user can replace the block.
func cynefinGetTheme(in *options) any {
	defaultThemeVariables := getThemeVariables()
	currentConfig := in.config
	// cleanAndMerge(defaultThemeVariables, currentConfig.themeVariables).cynefin
	return mergedProperty(defaultThemeVariables, currentConfig.Get("themeVariables"), "cynefin")
}

// cynefinStyles is getStyles of the cynefin diagram: the CSS of the diagram type.
func cynefinStyles(in *options) string {
	theme := cynefinGetTheme(in)
	// t reads a property of the block. JavaScript throws when the block is
	// undefined or null.
	t := func(key string) any {
		return prop(theme, key)
	}
	return `
	.cynefinDomain {
		stroke: none;
	}
	.cynefinDomainLabel {
		font-size: ` + s(t("domainFontSize")) + `px;
		font-weight: bold;
		fill: ` + s(t("labelColor")) + `;
	}
	.cynefinSubtitle {
		font-size: ` + js.Str(js.ToNumber(t("itemFontSize"))-1) + `px;
		fill: ` + s(t("textColor")) + `;
		font-style: italic;
	}
	.cynefinItem {
		fill-opacity: 0.95;
		stroke: ` + s(t("boundaryColor")) + `;
		stroke-width: 1;
	}
	.cynefinItemText {
		font-size: ` + s(t("itemFontSize")) + `px;
		fill: ` + s(t("textColor")) + `;
	}
	.cynefinItemOverflow {
		fill-opacity: 0.6;
		stroke: ` + s(t("boundaryColor")) + `;
		stroke-width: 1;
		stroke-dasharray: 3 2;
	}
	.cynefinBoundary {
		stroke: ` + s(t("boundaryColor")) + `;
		stroke-width: ` + s(t("boundaryWidth")) + `;
		stroke-dasharray: 6 3;
	}
	.cynefinCliff {
		stroke: ` + s(t("cliffColor")) + `;
		stroke-width: ` + s(t("cliffWidth")) + `;
	}
	.cynefinConfusion {
		stroke: ` + s(t("boundaryColor")) + `;
		stroke-width: 1.5;
		stroke-dasharray: 4 2;
	}
	.cynefinArrowLine {
		stroke: ` + s(t("arrowColor")) + `;
		stroke-width: ` + s(t("arrowWidth")) + `;
		fill: none;
	}
	.cynefinArrowHead {
		fill: ` + s(t("arrowColor")) + `;
		stroke: none;
	}
	.cynefinArrowLabel {
		font-size: ` + js.Str(js.ToNumber(t("itemFontSize"))-1) + `px;
		fill: ` + s(t("textColor")) + `;
	}
	.cynefinTitle {
		font-size: ` + add(t("domainFontSize"), 2) + `px;
		font-weight: bold;
		fill: ` + s(t("labelColor")) + `;
	}
	`
}
