// Ported from mermaid 12.2.0, mermaid/src/diagrams/requirement/styles.js
// (MIT).

package styles

// requirementGenColor emits the cycling colour of each box for the colour
// themes. It reads the theme and the palettes from the config.
func requirementGenColor(o *options) string {
	config := o.config

	theme, themeVariables := config.Get("theme"), config.Get("themeVariables")
	bkgColorArray, borderColorArray := prop(themeVariables, "bkgColorArray"), prop(themeVariables, "borderColorArray")
	// The gate is on the theme and on the palette, as in every other style sheet.
	if !isColorTheme(theme, borderColorArray) {
		return ""
	}
	// look is validated before it reaches the selector. See safeLook.
	look := safeLook(config.Get("look"))
	var sections text

	hasBkgColors := hasPalette(bkgColorArray)

	// One rule for each slot that requirementBox can stamp.
	palette, _ := borderColorArray.([]any)
	for i := 0; i < paletteSlotCount(borderColorArray); i++ {
		// The declaration is left out when there is no fill palette.
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

// requirementStyles is getStyles of the requirement diagram: the CSS of the diagram type.
func requirementStyles(o *options) string {
	config := o.config
	look, themeVariables := config.Get("look"), config.Get("themeVariables")
	requirementEdgeLabelBackground := prop(themeVariables, "requirementEdgeLabelBackground")
	return `
  ` + requirementGenColor(o) + `
  marker {
    fill: ` + o.s("relationColor") + `;
    stroke: ` + o.s("relationColor") + `;
  }

  marker.cross {
    stroke: ` + o.s("lineColor") + `;
  }

  svg {
    font-family: ` + o.s("fontFamily") + `;
    font-size: ` + o.s("fontSize") + `;
  }

  .reqBox {
    fill: ` + o.s("requirementBackground") + `;
    fill-opacity: 1.0;
    stroke: ` + o.s("requirementBorderColor") + `;
    stroke-width: ` + o.s("requirementBorderSize") + `;
  }
  
  .reqTitle, .reqLabel{
    fill:  ` + o.s("requirementTextColor") + `;
  }
  .reqLabelBox {
    fill: ` + o.s("relationLabelBackground") + `;
    fill-opacity: 1.0;
  }

  .req-title-line {
    stroke: ` + o.s("requirementBorderColor") + `;
    stroke-width: ` + o.s("requirementBorderSize") + `;
  }
  .relationshipLine {
    stroke: ` + o.s("relationColor") + `;
    stroke-width: ` + s(cond(is(look, "neo"), o.Get("strokeWidth"), "1px")) + `;
  }
  .relationshipLabel {
    fill: ` + o.s("relationLabelColor") + `;
  }
    .edgeLabel {
    background-color: ` + o.s("edgeLabelBackground") + `;
  }
  .edgeLabel .label rect {
    fill: ` + o.s("edgeLabelBackground") + `;
  }
  .edgeLabel .label text {
    fill: ` + o.s("relationLabelColor") + `;
  }
  .divider {
    stroke: ` + o.s("nodeBorder") + `;
    stroke-width: 1;
  }
  .label {
    font-family: ` + o.s("fontFamily") + `;
    color: ` + s(or(o.Get("nodeTextColor"), o.Get("textColor"))) + `;
  }
  .label text,span {
    fill: ` + s(or(o.Get("nodeTextColor"), o.Get("textColor"))) + `;
    color: ` + s(or(o.Get("nodeTextColor"), o.Get("textColor"))) + `;
  }
  .labelBkg {
    background-color: ` + s(nn(requirementEdgeLabelBackground, o.Get("edgeLabelBackground"))) + `;
  }

`
}
