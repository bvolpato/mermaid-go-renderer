// Ported from mermaid 12.2.0, mermaid/src/diagrams/state/styles.js (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// stateGenColor emits the cycling colour of each composite state and
// concurrency region for the colour themes. Only the containers are painted.
func stateGenColor(o *options) string {
	theme, bkgColorArray, borderColorArray := o.Get("theme"), o.Get("bkgColorArray"), o.Get("borderColorArray")
	if !isColorTheme(theme, borderColorArray) {
		return ""
	}
	// look is validated before it reaches the selector. See safeLook.
	look := safeLook(o.Get("look"))
	hasBkgColors := hasPalette(bkgColorArray)
	var sections text

	// One rule for each slot that the renderer can hand out.
	palette, _ := borderColorArray.([]any)
	for i := 0; i < paletteSlotCount(borderColorArray); i++ {
		borderColor := s(elem(palette, i))
		tint := ""
		if hasBkgColors {
			tint = `fill: ` + s(paletteAt(bkgColorArray, i)) + `;`
		}
		slot := `[data-look="` + look + `"][data-color-id="color-` + itoa(i) + `"]`
		sections.add(`

    /* The title strip: ` + "`" + `rect.outer` + "`" + ` spans the whole composite and ` + "`" + `rect.inner` + "`" + ` covers
       the body, so what stays visible of ` + "`" + `outer` + "`" + ` is the band behind the label. */
    ` + slot + `.statediagram-cluster rect.outer {
      stroke: ` + borderColor + `;
      ` + tint + `
    }

    ` + slot + `.statediagram-cluster rect.inner {
      stroke: ` + borderColor + `;
    }

    /* Concurrency regions. Siblings of one composite share a slot, so a divided composite
       reads as one thing split into parts rather than as several composites. */
    ` + slot + `.statediagram-cluster rect.divider {
      stroke: ` + borderColor + `;
      ` + tint + `
    }

    /* handDrawn draws the same container as roughjs shapes rather than plain rects, so it
       needs its own rules. ` + "`" + `roundedWithTitle` + "`" + ` and ` + "`" + `divider` + "`" + ` name those groups ` + "`" + `outer` + "`" + `,
       ` + "`" + `inner` + "`" + ` and ` + "`" + `divider` + "`" + ` to match the classic branch, which is what lets these
       discriminate -- a bare ` + "`" + `.statediagram-cluster path` + "`" + ` rule reached the body as well and
       tinted the whole composite, losing ` + "`" + `compositeBackground` + "`" + ` and diverging from what
       classic and neo do.

       roughjs emits two paths per shape and marks them: the filled shape carries
       ` + "`" + `stroke="none"` + "`" + ` and the sketched outline carries ` + "`" + `fill="none"` + "`" + `. Splitting on that is
       what keeps ` + "`" + `fill` + "`" + ` off the outline -- a rough outline is open squiggles, not a closed
       region, so filling it produces smears -- and keeps ` + "`" + `stroke` + "`" + ` off the fill shape, which
       would otherwise gain an edge it was drawn without. */
    ` + slot + `.statediagram-cluster .outer path[stroke='none'] {
      ` + tint + `
    }

    ` + slot + `.statediagram-cluster .outer path[fill='none'] {
      stroke: ` + borderColor + `;
    }

    /* No ` + "`" + `.inner` + "`" + ` rule on purpose. The body shape is left entirely alone under handDrawn,
       where a rect's ` + "`" + `inner` + "`" + ` counterpart cannot be recoloured safely: roughjs draws a
       hachure fill as *stroked* lines, so its fill paths carry ` + "`" + `fill="none"` + "`" + ` exactly like
       the outline and no selector separates them. An ` + "`" + `.inner` + "`" + ` stroke rule therefore
       repainted the hatching of every alt composite in the palette colour instead of
       leaving it on ` + "`" + `altBackground` + "`" + `. The container still reads as palette-coloured: the
       ` + "`" + `outer` + "`" + ` shape spans the whole composite, so its outline already frames the body. */

    /* Regions split the same way, which is why ` + "`" + `divider` + "`" + ` fills solid rather than taking
       roughjs's default hachure -- see the note on that call. Hatched, both of its paths
       carried ` + "`" + `fill="none"` + "`" + ` and these two rules degenerated: the tint matched nothing and
       the border rule repainted the hatching. */
    ` + slot + `.statediagram-cluster .divider path[stroke='none'] {
      ` + tint + `
    }

    ` + slot + `.statediagram-cluster .divider path[fill='none'] {
      stroke: ` + borderColor + `;
    }
    `)
	}
	return sections.String()
}

// stateStyles is getStyles of the state diagram: the CSS of the diagram type.
func stateStyles(o *options) string {
	altBackground := s(or(o.Get("altBackground"), "#efefef"))
	neoStroke := s(or(o.Get("stateBorder"), o.Get("nodeBorder")))
	if js.Truthy(o.Get("useGradient")) {
		neoStroke = "url(" + o.s("svgId") + "-gradient)"
	}
	neoFilter := "none"
	if dropShadow := o.Get("dropShadow"); js.Truthy(dropShadow) {
		neoFilter = replace(dropShadow, "url(#drop-shadow)", "url("+o.s("svgId")+"-drop-shadow)")
	}
	return `
` + stateGenColor(o) + `
defs [id$="-barbEnd"] {
    fill: ` + o.s("transitionColor") + `;
    stroke: ` + o.s("transitionColor") + `;
  }
g.stateGroup text {
  fill: ` + o.s("nodeBorder") + `;
  stroke: none;
  font-size: 10px;
}
g.stateGroup text {
  fill: ` + o.s("textColor") + `;
  stroke: none;
  font-size: 10px;

}
g.stateGroup .state-title {
  font-weight: bolder;
  fill: ` + o.s("stateLabelColor") + `;
}

g.stateGroup rect {
  fill: ` + o.s("mainBkg") + `;
  stroke: ` + o.s("nodeBorder") + `;
}

g.stateGroup line {
  stroke: ` + o.s("lineColor") + `;
  stroke-width: ` + s(or(o.Get("strokeWidth"), float64(1))) + `;
}

.transition {
  stroke: ` + o.s("transitionColor") + `;
  stroke-width: ` + s(or(o.Get("strokeWidth"), float64(1))) + `;
  fill: none;
}

.stateGroup .composit {
  fill: ` + o.s("background") + `;
  border-bottom: 1px
}

.stateGroup .alt-composit {
  fill: #e0e0e0;
  border-bottom: 1px
}

.state-note {
  stroke: ` + o.s("noteBorderColor") + `;
  fill: ` + o.s("noteBkgColor") + `;

  text {
    fill: ` + o.s("noteTextColor") + `;
    stroke: none;
    font-size: 10px;
  }
}

.stateLabel .box {
  stroke: none;
  stroke-width: 0;
  fill: ` + o.s("mainBkg") + `;
  opacity: 0.5;
}

.edgeLabel .label rect {
  fill: ` + o.s("labelBackgroundColor") + `;
  opacity: 0.5;
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
.edgeLabel .label text {
  fill: ` + s(or(o.Get("transitionLabelColor"), o.Get("tertiaryTextColor"))) + `;
}
.label div .edgeLabel {
  color: ` + s(or(o.Get("transitionLabelColor"), o.Get("tertiaryTextColor"))) + `;
}

.stateLabel text {
  fill: ` + o.s("stateLabelColor") + `;
  font-size: 10px;
  font-weight: bold;
}

.node circle.state-start {
  fill: ` + o.s("specialStateColor") + `;
  stroke: ` + o.s("specialStateColor") + `;
}

.node .fork-join {
  fill: ` + o.s("specialStateColor") + `;
  stroke: ` + o.s("specialStateColor") + `;
}

.node circle.state-end {
  fill: ` + o.s("innerEndBackground") + `;
  stroke: ` + o.s("background") + `;
  stroke-width: 1.5
}
.end-state-inner {
  fill: ` + s(or(o.Get("compositeBackground"), o.Get("background"))) + `;
  // stroke: ` + o.s("background") + `;
  stroke-width: 1.5
}

.node rect {
  fill: ` + s(or(o.Get("stateBkg"), o.Get("mainBkg"))) + `;
  stroke: ` + s(or(o.Get("stateBorder"), o.Get("nodeBorder"))) + `;
  stroke-width: ` + s(or(o.Get("strokeWidth"), float64(1))) + `px;
}
.node polygon {
  fill: ` + o.s("mainBkg") + `;
  stroke: ` + s(or(o.Get("stateBorder"), o.Get("nodeBorder"))) + `;;
  stroke-width: ` + s(or(o.Get("strokeWidth"), float64(1))) + `px;
}
[id$="-barbEnd"] {
  fill: ` + o.s("lineColor") + `;
}

.statediagram-cluster rect {
  fill: ` + o.s("compositeTitleBackground") + `;
  stroke: ` + s(or(o.Get("stateBorder"), o.Get("nodeBorder"))) + `;
  stroke-width: ` + s(or(o.Get("strokeWidth"), float64(1))) + `px;
}

.cluster-label, .nodeLabel {
  color: ` + o.s("stateLabelColor") + `;
  // line-height: 1;
}

.statediagram-cluster rect.outer {
  rx: 5px;
  ry: 5px;
}
.statediagram-state .divider {
  stroke: ` + s(or(o.Get("stateBorder"), o.Get("nodeBorder"))) + `;
}

.statediagram-state .title-state {
  rx: 5px;
  ry: 5px;
}
.statediagram-cluster.statediagram-cluster .inner {
  fill: ` + s(or(o.Get("compositeBackground"), o.Get("background"))) + `;
}
.statediagram-cluster.statediagram-cluster-alt .inner {
  fill: ` + altBackground + `;
}

.statediagram-cluster .inner {
  rx:0;
  ry:0;
}

.statediagram-state rect.basic {
  rx: 5px;
  ry: 5px;
}
.statediagram-state rect.divider {
  stroke-dasharray: 10,10;
  fill: ` + altBackground + `;
}

.note-edge {
  stroke-dasharray: 5;
}

.statediagram-note rect {
  fill: ` + o.s("noteBkgColor") + `;
  stroke: ` + o.s("noteBorderColor") + `;
  stroke-width: 1px;
  rx: 0;
  ry: 0;
}
.statediagram-note rect {
  fill: ` + o.s("noteBkgColor") + `;
  stroke: ` + o.s("noteBorderColor") + `;
  stroke-width: 1px;
  rx: 0;
  ry: 0;
}

.statediagram-note text {
  fill: ` + o.s("noteTextColor") + `;
}

.statediagram-note .nodeLabel {
  color: ` + o.s("noteTextColor") + `;
}
.statediagram .edgeLabel {
  color: red; // ` + o.s("noteTextColor") + `;
}

[id$="-dependencyStart"], [id$="-dependencyEnd"] {
  fill: ` + o.s("lineColor") + `;
  stroke: ` + o.s("lineColor") + `;
  stroke-width: ` + s(or(o.Get("strokeWidth"), float64(1))) + `;
}

.statediagramTitleText {
  text-anchor: middle;
  font-size: 18px;
  fill: ` + o.s("textColor") + `;
}

[data-look="neo"].statediagram-cluster rect {
  fill: ` + o.s("mainBkg") + `;
  stroke: ` + neoStroke + `;
  stroke-width: ` + s(nn(o.Get("strokeWidth"), float64(1))) + `;
}
[data-look="neo"].statediagram-cluster rect.outer {
  rx: ` + o.s("radius") + `px;
  ry: ` + o.s("radius") + `px;
  filter: ` + neoFilter + `
}
`
}
