// Ported from mermaid 12.2.0,
// mermaid/src/diagrams/architecture/architectureStyles.ts (MIT).

package styles

// architectureStyles is getStyles of the architecture diagram: the CSS of the diagram type.
func architectureStyles(o *options) string {
	return `
  .edge {
    stroke-width: ` + o.s("archEdgeWidth") + `;
    stroke: ` + o.s("archEdgeColor") + `;
    fill: none;
  }

  .arrow {
    fill: ` + o.s("archEdgeArrowColor") + `;
  }

  .node-bkg {
    fill: none;
    stroke: ` + o.s("archGroupBorderColor") + `;
    stroke-width: ` + o.s("archGroupBorderWidth") + `;
    stroke-dasharray: 8;
  }
  .node-icon-text {
    display: flex; 
    align-items: center;
  }
  
  .node-icon-text > div {
    color: #fff;
    margin: 1px;
    height: fit-content;
    text-align: center;
    overflow: hidden;
    display: -webkit-box;
    -webkit-box-orient: vertical;
  }
`
}
