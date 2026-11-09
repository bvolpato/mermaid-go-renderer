// Ported from mermaid 12.2.0, mermaid/src/diagrams/class/styles.js (MIT).

package styles

// classGenColor emits the cycling colour of each class box for the colour
// themes.
func classGenColor(o *options) string {
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
		sections.add(`

    [data-look="` + look + `"][data-color-id="color-` + itoa(i) + `"].node .outer-path path {
      stroke: ` + borderColor + `;
      ` + fill + `
    }

    [data-look="` + look + `"][data-color-id="color-` + itoa(i) + `"].node .divider path {
      stroke: ` + borderColor + `;
    }
    `)
	}
	return sections.String()
}

// classStyles is getStyles of the class diagram: the CSS of the diagram type.
func classStyles(o *options) string {
	return classGenColor(o) + `
  g.classGroup text {
  fill: ` + s(or(o.Get("nodeBorder"), o.Get("classText"))) + `;
  stroke: none;
  font-family: ` + o.s("fontFamily") + `;
  font-size: 10px;

  .title {
    font-weight: bolder;
  }

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

.nodeLabel, .edgeLabel {
  color: ` + o.s("classText") + `;
}

.noteLabel .nodeLabel, .noteLabel .edgeLabel {
  color: ` + o.s("noteTextColor") + `;
}
.edgeLabel .label rect {
  fill: ` + o.s("mainBkg") + `;
}
.label text {
  fill: ` + o.s("classText") + `;
}

.labelBkg {
  background: ` + o.s("mainBkg") + `;
}
.edgeLabel .label span {
  background: ` + o.s("mainBkg") + `;
}

.classTitle {
  font-weight: bolder;
}
.node rect,
  .node circle,
  .node ellipse,
  .node polygon,
  .node path {
    fill: ` + o.s("mainBkg") + `;
    stroke: ` + o.s("nodeBorder") + `;
    stroke-width: ` + o.s("strokeWidth") + `;
  }


.divider {
  stroke: ` + o.s("nodeBorder") + `;
  stroke-width: 1;
}

g.clickable {
  cursor: pointer;
}

g.classGroup rect {
  fill: ` + o.s("mainBkg") + `;
  stroke: ` + o.s("nodeBorder") + `;
}

g.classGroup line {
  stroke: ` + o.s("nodeBorder") + `;
  stroke-width: 1;
}

.classLabel .box {
  stroke: none;
  stroke-width: 0;
  fill: ` + o.s("mainBkg") + `;
  opacity: 0.5;
}

.classLabel .label {
  fill: ` + o.s("nodeBorder") + `;
  font-size: 10px;
}

.relation {
  stroke: ` + o.s("lineColor") + `;
  stroke-width: ` + o.s("strokeWidth") + `;
  fill: none;
}

.dashed-line{
  stroke-dasharray: 3;
}

.dotted-line{
  stroke-dasharray: 1 2;
}

[id$="-compositionStart"], .composition {
  fill: ` + o.s("lineColor") + ` !important;
  stroke: ` + o.s("lineColor") + ` !important;
  stroke-width: 1;
}

[id$="-compositionEnd"], .composition {
  fill: ` + o.s("lineColor") + ` !important;
  stroke: ` + o.s("lineColor") + ` !important;
  stroke-width: 1;
}

[id$="-dependencyStart"], .dependency {
  fill: ` + o.s("lineColor") + ` !important;
  stroke: ` + o.s("lineColor") + ` !important;
  stroke-width: 1;
}

[id$="-dependencyEnd"], .dependency {
  fill: ` + o.s("lineColor") + ` !important;
  stroke: ` + o.s("lineColor") + ` !important;
  stroke-width: 1;
}

[id$="-extensionStart"], .extension {
  fill: transparent !important;
  stroke: ` + o.s("lineColor") + ` !important;
  stroke-width: 1;
}

[id$="-extensionEnd"], .extension {
  fill: transparent !important;
  stroke: ` + o.s("lineColor") + ` !important;
  stroke-width: 1;
}

[id$="-aggregationStart"], .aggregation {
  fill: transparent !important;
  stroke: ` + o.s("lineColor") + ` !important;
  stroke-width: 1;
}

[id$="-aggregationEnd"], .aggregation {
  fill: transparent !important;
  stroke: ` + o.s("lineColor") + ` !important;
  stroke-width: 1;
}

[id$="-lollipopStart"], .lollipop {
  fill: ` + o.s("mainBkg") + ` !important;
  stroke: ` + o.s("lineColor") + ` !important;
  stroke-width: 1;
}

[id$="-lollipopEnd"], .lollipop {
  fill: ` + o.s("mainBkg") + ` !important;
  stroke: ` + o.s("lineColor") + ` !important;
  stroke-width: 1;
}

.edgeTerminals {
  font-size: 11px;
  line-height: initial;
}

.classTitleText {
  text-anchor: middle;
  font-size: 18px;
  fill: ` + o.s("textColor") + `;
}

.edgeLabel[data-look="neo"] {
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
  ` + getIconStyles() + `
`
}
