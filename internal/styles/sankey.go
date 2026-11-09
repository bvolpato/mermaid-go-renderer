// Ported from mermaid 12.2.0, mermaid/src/diagrams/sankey/styles.js (MIT).

package styles

// sankeyStyles is getStyles of the sankey diagram: the CSS of the diagram type.
func sankeyStyles(o *options) string {
	return `.label {
    font-family: ` + o.s("fontFamily") + `;
  }

  .node-labels {
    font-family: ` + o.s("fontFamily") + `;
  }

  /* Outlined label style - background stroke for better readability */
  .sankey-label-bg {
    stroke: ` + s(or(or(o.Get("mainBkg"), o.Get("background")), "#fff")) + `;
    stroke-width: 4px;
    stroke-linejoin: round;
    paint-order: stroke;
  }

  /* Foreground label text */
  .sankey-label-fg {
    fill: ` + o.s("textColor") + `;
  }

  /* Node styling */
  .node rect {
    shape-rendering: crispEdges;
  }

  /* Link styling */
  .link {
    fill: none;
    stroke-opacity: 0.5;
    mix-blend-mode: multiply;
  }
`
}
