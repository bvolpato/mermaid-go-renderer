// Ported from mermaid 12.2.0, mermaid/src/diagrams/block/styles.ts (MIT).

package styles

// blockGenColor emits the palette rules of each composite block for the
// colour themes.
func blockGenColor(o *options) string {
	theme, bkgColorArray, borderColorArray := o.Get("theme"), o.Get("bkgColorArray"), o.Get("borderColorArray")
	if !isColorTheme(theme, borderColorArray) {
		return ""
	}
	look := safeLook(o.Get("look"))
	hasBkgColors := hasPalette(bkgColorArray)
	var sections text

	for i := 0; i < colorSlotCount(o.Get("THEME_COLOR_LIMIT"), borderColorArray); i++ {
		borderColor := s(paletteAt(borderColorArray, i))
		fill := ""
		if hasBkgColors {
			fill = `fill: ` + s(paletteAt(bkgColorArray, i)) + `;`
		}
		slot := `[data-look="` + look + `"][data-color-id="color-` + itoa(i) + `"]`

		sections.add(`

    ` + slot + `.node rect.composite {
      stroke: ` + borderColor + `;
      ` + fill + `
    }
`)
	}
	return sections.String()
}

// blockStyles is getStyles of the block diagram: the CSS of the diagram type.
func blockStyles(o *options) string {
	return blockGenColor(o) + `
  .label {
    font-family: ` + o.s("fontFamily") + `;
    color: ` + s(or(o.Get("nodeTextColor"), o.Get("textColor"))) + `;
  }
  .cluster-label text {
    fill: ` + o.s("titleColor") + `;
  }
  .cluster-label span {
    color: ` + o.s("titleColor") + `;
  }



  .label text,span {
    fill: ` + s(or(o.Get("nodeTextColor"), o.Get("textColor"))) + `;
    color: ` + s(or(o.Get("nodeTextColor"), o.Get("textColor"))) + `;
  }

  .node rect,
  .node circle,
  .node ellipse,
  .node polygon,
  .node path {
    fill: ` + o.s("mainBkg") + `;
    stroke: ` + o.s("nodeBorder") + `;
    /* From the theme, as every other diagram does. Pinned at 1px here, so block drew a
       thinner border than an identical flowchart node under any theme asking for more --
       the neo theme asks for 2. Nothing downstream corrected it: the neo rules set stroke
       and filter, never stroke-width. */
    stroke-width: ` + s(nn(o.Get("strokeWidth"), float64(1))) + `px;
  }
  .flowchart-label text {
    text-anchor: middle;
  }
  // .flowchart-label .text-outer-tspan {
  //   text-anchor: middle;
  // }
  // .flowchart-label .text-inner-tspan {
  //   text-anchor: start;
  // }

  .node .label {
    text-align: center;
  }
  .node.clickable {
    cursor: pointer;
  }

  .arrowheadPath {
    fill: ` + o.s("arrowheadColor") + `;
  }

  .edgePaths .path {
    stroke: ` + o.s("lineColor") + `;
    stroke-width: 2.0px;
  }

  .flowchart-link {
    stroke: ` + o.s("lineColor") + `;
    fill: none;
  }

  .edgeLabel {
    background-color: ` + o.s("edgeLabelBackground") + `;
    /*
     * This is for backward compatibility with existing code that didn't
     * add a ` + "`" + `<p>` + "`" + ` around edge labels.
     *
     * TODO: We should probably remove this in a future release.
     */
    p {
      margin: 0;
      padding: 0;
      display: inline;
    }
    rect {
      opacity: 0.5;
      background-color: ` + o.s("edgeLabelBackground") + `;
      fill: ` + o.s("edgeLabelBackground") + `;
    }
    text-align: center;
  }

  /* For html labels only */
  .labelBkg {
    background-color: ` + o.s("edgeLabelBackground") + `;
  }

  .node .cluster {
    // fill: ` + fade(o.Get("mainBkg"), 0.5) + `;
    fill: ` + fade(o.Get("clusterBkg"), 0.5) + `;
    stroke: ` + fade(o.Get("clusterBorder"), 0.2) + `;
    box-shadow: rgba(50, 50, 93, 0.25) 0px 13px 27px -5px, rgba(0, 0, 0, 0.3) 0px 8px 16px -8px;
    stroke-width: 1px;
  }

  .cluster text {
    fill: ` + o.s("titleColor") + `;
  }

  .cluster span {
    color: ` + o.s("titleColor") + `;
  }
  /* .cluster div {
    color: ` + o.s("titleColor") + `;
  } */

  div.mermaidTooltip {
    position: absolute;
    text-align: center;
    max-width: 200px;
    padding: 2px;
    font-family: ` + o.s("fontFamily") + `;
    font-size: 12px;
    background: ` + o.s("tertiaryColor") + `;
    border: 1px solid ` + o.s("border2") + `;
    border-radius: 2px;
    pointer-events: none;
    z-index: 100;
  }

  .flowchartTitleText {
    text-anchor: middle;
    font-size: 18px;
    fill: ` + o.s("textColor") + `;
  }
  ` + getIconStyles() + `
`
}
