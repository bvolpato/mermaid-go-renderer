// Ported from mermaid 12.2.0, mermaid/src/diagrams/globalStyles.ts (MIT).

package styles

// getIconStyles gives the rules for Font Awesome icons in labels.
func getIconStyles() string {
	return `
  /* Font Awesome icon styling - consolidated */
  .label-icon {
    display: inline-block;
    height: 1em;
    overflow: visible;
    vertical-align: -0.125em;
  }
  
  .node .label-icon path {
    fill: currentColor;
    stroke: revert;
    stroke-width: revert;
  }
`
}
