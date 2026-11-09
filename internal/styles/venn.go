// Ported from mermaid 12.2.0, mermaid/src/diagrams/venn/styles.ts (MIT).

package styles

// vennStyles is getStyles of the venn diagram: the CSS of the diagram type.
func vennStyles(o *options) string {
	return `
  .venn-title {
    font-size: 32px;
    fill: ` + o.s("vennTitleTextColor") + `;
    font-family: ` + o.s("fontFamily") + `;
  }

  .venn-circle text {
    font-size: 48px;
    font-family: ` + o.s("fontFamily") + `;
  }

  .venn-intersection text {
    font-size: 48px;
    fill: ` + o.s("vennSetTextColor") + `;
    font-family: ` + o.s("fontFamily") + `;
  }

  .venn-text-node {
    font-family: ` + o.s("fontFamily") + `;
    color: ` + o.s("vennSetTextColor") + `;
  }
`
}
