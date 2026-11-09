// Ported from mermaid 12.2.0, mermaid/src/diagrams/flowchart/styles.ts (MIT).

package styles

// flowchartGenColor emits the cycling colour of each subgraph for the colour
// themes. Only the containers are painted. The rules have the rect form
// (classic and neo) and the path form (handDrawn).
func flowchartGenColor(o *options) string {
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
		// A collapsed subgraph has the class rough-node and not node for the
		// handDrawn look. Each descendant goes after both prefixes.
		collapsedRule := func(suffix string) string {
			return slot + `.node ` + suffix + `, ` + slot + `.rough-node ` + suffix
		}
		// Both halves of a lane. Each carries the whole prefix.
		laneRule := func(suffix string) string {
			return slot + `.swimlane.cluster .swimlane-title` + suffix + `, ` + slot + `.swimlane.cluster .swimlane-body` + suffix
		}
		laneFill := ""
		if hasBkgColors {
			laneFill = `
    /* A roughjs fill is drawn as lines, so the lane fill is a stroke here. */
    ` + laneRule(" path:first-of-type") + ` {
      stroke: ` + s(paletteAt(bkgColorArray, i)) + `;
    }
`
		}
		sections.add(`

    ` + slot + `.cluster:not(.swimlane) rect {
      stroke: ` + borderColor + `;
      ` + fill + `
    }

    ` + slot + `.cluster:not(.swimlane) path {
      stroke: ` + borderColor + `;
      ` + fill + `
    }

    /* Lane, classic and neo. */
    ` + slot + `.swimlane.cluster rect.swimlane-title,
    ` + slot + `.swimlane.cluster rect.swimlane-body {
      stroke: ` + borderColor + `;
      ` + fill + `
    }

    /* Lane, handDrawn: roughjs emits the hachure fill first, then the outline. */
    ` + laneRule(" path:nth-of-type(2)") + ` {
      stroke: ` + borderColor + `;
    }
` + laneFill + `
    ` + collapsedRule(".collapsed-group") + `,
    ` + collapsedRule(".collapsed-group path") + ` {
      stroke: ` + borderColor + `;
      ` + fill + `
    }

    /* The ellipsis dots and the separator take clusterBorder further down, so without
       these the container is palette-coloured while its own markers are not. */
    ` + collapsedRule(".collapsed-indicator") + ` {
      fill: ` + borderColor + `;
    }

    ` + collapsedRule(".collapsed-separator") + ` {
      stroke: ` + borderColor + `;
    }
    `)
	}
	return sections.String()
}

// flowchartStyles is getStyles of the flowchart diagram: the CSS of the diagram type.
func flowchartStyles(o *options) string {
	return flowchartGenColor(o) + `
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
  .cluster-label span p {
    background-color: transparent;
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
    stroke-width: ` + s(nn(o.Get("strokeWidth"), float64(1))) + `px;
  }
  .rough-node .label text , .node .label text, .image-shape .label, .icon-shape .label {
    text-anchor: middle;
  }

  .node .katex path {
    fill: #000;
    stroke: #000;
    stroke-width: 1px;
  }

  .rough-node .label,.node .label, .image-shape .label, .icon-shape .label {
    text-align: center;
  }
  .node.clickable {
    cursor: pointer;
  }


  .root .anchor path {
    fill: ` + o.s("lineColor") + ` !important;
    stroke-width: 0;
    stroke: ` + o.s("lineColor") + `;
  }

  .arrowheadPath {
    fill: ` + o.s("arrowheadColor") + `;
  }

  .edgePaths .path {
    stroke: ` + o.s("lineColor") + `;
    stroke-width: ` + s(nn(o.Get("strokeWidth"), float64(2))) + `px;
  }

  .flowchart-link {
    stroke: ` + o.s("lineColor") + `;
    fill: none;
  }

  .edgeLabel {
    background-color: ` + o.s("edgeLabelBackground") + `;
    p {
      background-color: ` + o.s("edgeLabelBackground") + `;
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
    background-color: ` + fade(o.Get("edgeLabelBackground"), 0.5) + `;
    // background-color:
  }

  .cluster rect {
    fill: ` + o.s("clusterBkg") + `;
    stroke: ` + o.s("clusterBorder") + `;
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

  /* Collapsed subgraph node (@{ view: collapsed }) */
  .node .collapsed-indicator {
    fill: ` + o.s("clusterBorder") + `;
    stroke: none;
    opacity: 0.6;
  }

  .node .collapsed-separator {
    stroke: ` + o.s("clusterBorder") + `;
    stroke-width: 0.75px;
  }

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

  rect.text {
    fill: none;
    stroke-width: 0;
  }

  .icon-shape, .image-shape {
    background-color: ` + o.s("edgeLabelBackground") + `;
    p {
      background-color: ` + o.s("edgeLabelBackground") + `;
      padding: 2px;
    }
    .label rect {
      opacity: 0.5;
      background-color: ` + o.s("edgeLabelBackground") + `;
      fill: ` + o.s("edgeLabelBackground") + `;
    }
    text-align: center;
  }
  ` + getIconStyles() + `
`
}
