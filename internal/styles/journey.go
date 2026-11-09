// Ported from mermaid 12.2.0, mermaid/src/diagrams/user-journey/styles.js
// (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// journeyFill is the template expression
// options[test] ? `fill: ${options[value]}` : fallback.
func journeyFill(o *options, test, value, fallback string) string {
	if js.Truthy(o.Get(test)) {
		return "fill: " + o.s(value)
	}
	return fallback
}

// journeyStyles is getStyles of the user journey diagram: the CSS of the diagram type.
func journeyStyles(o *options) string {
	return `.label {
    font-family: ` + o.s("fontFamily") + `;
    color: ` + o.s("textColor") + `;
  }
  .mouth {
    stroke: #666;
  }

  line {
    stroke: ` + o.s("textColor") + `
  }

  .legend {
    fill: ` + o.s("textColor") + `;
    font-family: ` + o.s("fontFamily") + `;
  }

  .label text {
    fill: #333;
  }
  .label {
    color: ` + o.s("textColor") + `
  }

  .face {
    ` + journeyFill(o, "faceColor", "faceColor", "fill: #FFF8DC") + `;
    stroke: #999;
  }

  .node rect,
  .node circle,
  .node ellipse,
  .node polygon,
  .node path {
    fill: ` + o.s("mainBkg") + `;
    stroke: ` + o.s("nodeBorder") + `;
    stroke-width: 1px;
  }

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
    stroke-width: 1.5px;
  }

  .flowchart-link {
    stroke: ` + o.s("lineColor") + `;
    fill: none;
  }

  .edgeLabel {
    background-color: ` + o.s("edgeLabelBackground") + `;
    rect {
      opacity: 0.5;
    }
    text-align: center;
  }

  .cluster rect {
  }

  .cluster text {
    fill: ` + o.s("titleColor") + `;
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

  .task-type-0, .section-type-0  {
    ` + journeyFill(o, "fillType0", "fillType0", "") + `;
  }
  .task-type-1, .section-type-1  {
    ` + journeyFill(o, "fillType0", "fillType1", "") + `;
  }
  .task-type-2, .section-type-2  {
    ` + journeyFill(o, "fillType0", "fillType2", "") + `;
  }
  .task-type-3, .section-type-3  {
    ` + journeyFill(o, "fillType0", "fillType3", "") + `;
  }
  .task-type-4, .section-type-4  {
    ` + journeyFill(o, "fillType0", "fillType4", "") + `;
  }
  .task-type-5, .section-type-5  {
    ` + journeyFill(o, "fillType0", "fillType5", "") + `;
  }
  .task-type-6, .section-type-6  {
    ` + journeyFill(o, "fillType0", "fillType6", "") + `;
  }
  .task-type-7, .section-type-7  {
    ` + journeyFill(o, "fillType0", "fillType7", "") + `;
  }

  .actor-0 {
    ` + journeyFill(o, "actor0", "actor0", "") + `;
  }
  .actor-1 {
    ` + journeyFill(o, "actor1", "actor1", "") + `;
  }
  .actor-2 {
    ` + journeyFill(o, "actor2", "actor2", "") + `;
  }
  .actor-3 {
    ` + journeyFill(o, "actor3", "actor3", "") + `;
  }
  .actor-4 {
    ` + journeyFill(o, "actor4", "actor4", "") + `;
  }
  .actor-5 {
    ` + journeyFill(o, "actor5", "actor5", "") + `;
  }
  ` + getIconStyles() + `
`
}
