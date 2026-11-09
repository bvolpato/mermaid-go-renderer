// Ported from mermaid 12.2.0, mermaid/src/diagrams/pie/pieStyles.ts (MIT).

package styles

// pieStyles is getStyles of the pie diagram: the CSS of the diagram type.
func pieStyles(o *options) string {
	return `
  .pieCircle{
    stroke: ` + o.s("pieStrokeColor") + `;
    stroke-width : ` + o.s("pieStrokeWidth") + `;
    opacity : ` + o.s("pieOpacity") + `;
  }
  .pieCircle.highlighted{
    scale: 1.05;
    opacity: 1;
  }
  .pieCircle.highlightedOnHover:hover{
    transition-duration: 250ms;
    scale: 1.05;
    opacity: 1;
  }
  .pieOuterCircle{
    stroke: ` + o.s("pieOuterStrokeColor") + `;
    stroke-width: ` + o.s("pieOuterStrokeWidth") + `;
    fill: none;
  }
  .pieTitleText {
    text-anchor: middle;
    font-size: ` + o.s("pieTitleTextSize") + `;
    fill: ` + o.s("pieTitleTextColor") + `;
    font-family: ` + o.s("fontFamily") + `;
  }
  .slice {
    font-family: ` + o.s("fontFamily") + `;
    fill: ` + o.s("pieSectionTextColor") + `;
    font-size:` + o.s("pieSectionTextSize") + `;
    // fill: white;
  }
  .legend text {
    fill: ` + o.s("pieLegendTextColor") + `;
    font-family: ` + o.s("fontFamily") + `;
    font-size: ` + o.s("pieLegendTextSize") + `;
  }
`
}
