// Ported from mermaid 12.2.0, mermaid/src/diagrams/swimlanes/styles.ts (MIT).

package styles

// swimlanesStyles is the flowchart styles plus the rules of a lane.
func swimlanesStyles(o *options) string {
	return flowchartStyles(o) + `
  .swimlane.cluster:not([data-color-id]) rect {
    stroke: ` + o.s("clusterBorder") + ` !important;
  }
  [data-look="neo"].cluster rect {
    filter: none;
  }
`
}
