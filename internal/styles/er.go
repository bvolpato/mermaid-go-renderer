// Ported from mermaid 12.2.0, mermaid/src/diagrams/er/styles.ts (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// erGenColor emits the cycling colour of each entity for the colour themes.
func erGenColor(o *options) string {
	theme, bkgColorArray, borderColorArray := o.Get("theme"), o.Get("bkgColorArray"), o.Get("borderColorArray")
	// isColorTheme covers both halves of the gate: the theme is a colour theme
	// and the border palette is an array that is not empty.
	if !isColorTheme(theme, borderColorArray) {
		return ""
	}
	// look is validated before it reaches the selector. See safeLook.
	look := safeLook(o.Get("look"))
	hasBkgColors := hasPalette(bkgColorArray)
	var sections text

	// One rule for each slot that erBox can stamp.
	palette, _ := borderColorArray.([]any)
	for i := 0; i < paletteSlotCount(borderColorArray); i++ {
		borderColor := s(elem(palette, i))
		fill := ""
		if hasBkgColors {
			fill = `fill: ` + s(paletteAt(bkgColorArray, i)) + `;`
		}
		sections.add(`

    [data-look="` + look + `"][data-color-id="color-` + itoa(i) + `"].node path {
    stroke: ` + borderColor + `;
    ` + fill + `
    }

    [data-look="` + look + `"][data-color-id="color-` + itoa(i) + `"].node  rect {
    stroke: ` + borderColor + `;
    ` + fill + `
     }
    `)
	}
	return sections.String()
}

// erStyles is getStyles of the er diagram: the CSS of the diagram type.
func erStyles(o *options) string {
	look, theme, erEdgeLabelBackground, strokeWidth := o.Get("look"), o.Get("theme"), o.Get("erEdgeLabelBackground"), o.Get("strokeWidth")
	// The template evaluates its expressions in order. genColor comes first.
	genColor := erGenColor(o)
	useLabelBackground := inColorThemes(theme) && js.Truthy(erEdgeLabelBackground)
	labelBkg := ""
	if useLabelBackground {
		labelBkg = s(erEdgeLabelBackground)
	} else {
		labelBkg = fade(o.Get("tertiaryColor"), 0.5)
	}
	edgeLabel := s(cond(useLabelBackground, erEdgeLabelBackground, o.Get("edgeLabelBackground")))
	neoStrokeWidth := s(cond(is(look, "neo"), strokeWidth, "1px"))
	return `
    ` + genColor + `
  .entityBox {
    fill: ` + o.s("mainBkg") + `;
    stroke: ` + o.s("nodeBorder") + `;
  }

  .relationshipLabelBox {
    fill: ` + o.s("tertiaryColor") + `;
    opacity: 0.7;
    background-color: ` + o.s("tertiaryColor") + `;
      rect {
        opacity: 0.5;
      }
  }

  .labelBkg {
    background-color: ` + labelBkg + `;
  }

  .edgeLabel {
    background-color: ` + edgeLabel + `;
  }
  .edgeLabel .label rect {
    fill: ` + edgeLabel + `;
  }
  .edgeLabel .label text {
    fill: ` + o.s("textColor") + `;
  }

  .edgeLabel .label {
    fill: ` + o.s("nodeBorder") + `;
    font-size: 14px;
  }

  .label {
    font-family: ` + o.s("fontFamily") + `;
    color: ` + s(or(o.Get("nodeTextColor"), o.Get("textColor"))) + `;
  }

  .edge-pattern-dashed {
    stroke-dasharray: 8,8;
  }

  .node rect,
  .node circle,
  .node ellipse,
  .node polygon
  {
    fill: ` + o.s("mainBkg") + `;
    stroke: ` + o.s("nodeBorder") + `;
    stroke-width: ` + neoStrokeWidth + `;
  }

  .relationshipLine {
    stroke: ` + o.s("lineColor") + `;
    stroke-width: ` + neoStrokeWidth + `;
    fill: none;
  }

  .marker {
    fill: none !important;
    stroke: ` + o.s("lineColor") + ` !important;
    stroke-width: 1;
  }
  [data-look=neo].labelBkg {
    background-color: ` + fade(o.Get("tertiaryColor"), 0.5) + `;
  }

  .cluster rect {
    fill: ` + s(nn(o.Get("clusterBkg"), o.Get("mainBkg"))) + `;
    stroke: ` + s(nn(o.Get("clusterBorder"), o.Get("nodeBorder"))) + `;
    stroke-width: 1px;
  }

  .cluster text {
    fill: ` + s(nn(o.Get("titleColor"), o.Get("textColor"))) + `;
  }

  .cluster-label text {
    fill: ` + s(nn(o.Get("titleColor"), o.Get("textColor"))) + `;
  }
`
}
