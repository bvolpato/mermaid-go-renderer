// Ported from mermaid 12.2.0, mermaid/src/diagrams/agentflow/styles.ts and
// from the slot functions of mermaid/src/diagrams/agentflow/colorSlots.ts
// (MIT).

package styles

// agentflowKinds are the node kinds in palette order: KINDS and KIND_SLOT of
// colorSlots.ts. The slot of a kind is its index.
var agentflowKinds = []string{"tool", "task", "decision", "input", "refdoc", "connector", "action"}

// agentflowKindClass is kindClass: the class that marks the kind of a node.
func agentflowKindClass(kind string) string {
	return "af-kind-" + kind
}

// agentflowContainerSlotCount is containerSlotCount: how many slots the
// containers can cycle through. It is at least one.
func agentflowContainerSlotCount(paletteLength int) int {
	return max(1, paletteLength-len(agentflowKinds))
}

// agentflowContainerSlot is containerSlot: the palette slot of the nth
// container.
func agentflowContainerSlot(n, paletteLength int) int {
	slot := len(agentflowKinds) + (n % agentflowContainerSlotCount(paletteLength))
	if paletteLength > 0 {
		return slot % paletteLength
	}
	return slot
}

// agentflowGenColor emits the palette rules for the colour themes: one colour
// for each node kind from a fixed slot, and a counter over the containers
// from the slots above them.
func agentflowGenColor(o *options) string {
	theme, bkgColorArray, borderColorArray := o.Get("theme"), o.Get("bkgColorArray"), o.Get("borderColorArray")
	if !isColorTheme(theme, borderColorArray) {
		return ""
	}
	look := safeLook(o.Get("look"))
	hasBkgColors := hasPalette(bkgColorArray)
	paletteLength := colorSlotCount(o.Get("THEME_COLOR_LIMIT"), borderColorArray)
	border := func(slot int) string {
		return s(paletteAt(borderColorArray, slot))
	}
	fill := func(slot int) string {
		if hasBkgColors {
			return `fill: ` + s(paletteAt(bkgColorArray, slot)) + `;`
		}
		return ""
	}

	var sections text

	// One rule for each kind.
	for slot, kind := range agentflowKinds {
		sel := `[data-look="` + look + `"].node.` + agentflowKindClass(kind)
		sections.add(`

    ` + sel + ` rect,
    ` + sel + ` path,
    ` + sel + ` polygon {
      stroke: ` + border(slot) + `;
      ` + fill(slot) + `
    }
`)
	}

	// The containers cycle the slots above the kind range.
	for i := 0; i < agentflowContainerSlotCount(paletteLength); i++ {
		slot := agentflowContainerSlot(i, paletteLength)
		expanded := `[data-look="` + look + `"][data-color-id="color-` + itoa(slot) + `"].cluster`
		collapsed := `[data-look="` + look + `"][data-color-id="color-` + itoa(slot) + `"].node`
		rule := func(suffix string) string {
			return expanded + ` ` + suffix + `, ` + collapsed + ` ` + suffix
		}
		sections.add(`

    ` + rule("rect") + `,
    ` + rule("path") + ` {
      stroke: ` + border(slot) + `;
      ` + fill(slot) + `
    }
`)
	}
	return sections.String()
}

// agentflowStyles is getStyles of the agentflow diagram: the CSS of the diagram type.
func agentflowStyles(o *options) string {
	return agentflowGenColor(o) + `
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
    fill: ` + s(or(o.Get("nodeTextColor"), o.Get("textColor"))) + `;
    stroke: ` + s(or(o.Get("nodeTextColor"), o.Get("textColor"))) + `;
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
  }

  .cluster rect {
    fill: ` + o.s("clusterBkg") + `;
    stroke: ` + o.s("clusterBorder") + `;
    stroke-width: 1px;
  }

  .flow-cluster rect {
    fill: none;
    stroke: ` + o.s("clusterBorder") + `;
    stroke-width: 0.75px;
  }

  .node .collapsed-indicator {
    fill: ` + o.s("nodeBorder") + `;
    stroke: none;
    stroke-width: 0;
    opacity: 0.5;
  }

  .node .collapsed-separator {
    stroke-width: 0.75px;
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

  .agentflowTitleText {
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
