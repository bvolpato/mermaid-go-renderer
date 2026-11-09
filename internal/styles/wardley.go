// Ported from mermaid 12.2.0, mermaid/src/diagrams/wardley/styles.ts (MIT).

package styles

// wardleyStyles reads its options from the wardley theme variable, merged
// over the wardley block of the theme.
func wardleyStyles(in *options) string {
	defaultThemeVariables := getThemeVariables()
	currentConfig := in.config
	// cleanAndMerge(defaultThemeVariables, currentConfig.themeVariables).wardley
	wardley := mergedProperty(defaultThemeVariables, currentConfig.Get("themeVariables"), "wardley")
	w := cleanAndMerge(wardley, in.Get("wardley"))

	return `
  .wardley-background {
    fill: ` + s(w.Get("backgroundColor")) + `;
  }
  .wardley-axes line, .wardley-axes path {
    stroke: ` + s(w.Get("axisColor")) + `;
  }
  .wardley-axis-label {
    fill: ` + s(w.Get("axisTextColor")) + `;
  }
  .wardley-stage-label {
    fill: ` + s(w.Get("axisTextColor")) + `;
  }
  .wardley-grid line {
    stroke: ` + s(w.Get("gridColor")) + `;
  }
  .wardley-node circle {
    fill: ` + s(w.Get("componentFill")) + `;
    stroke: ` + s(w.Get("componentStroke")) + `;
  }
  .wardley-node-label {
    fill: ` + s(w.Get("componentLabelColor")) + `;
  }
  .wardley-link {
    stroke: ` + s(w.Get("linkStroke")) + `;
  }
  .wardley-link--dashed {
    stroke-dasharray: 4 4;
  }
  .wardley-link-label {
    fill: ` + s(w.Get("axisTextColor")) + `;
  }
  .wardley-trend line {
    stroke: ` + s(w.Get("evolutionStroke")) + `;
  }
  .wardley-annotation-line {
    stroke: ` + s(w.Get("annotationStroke")) + `;
  }
  .wardley-annotation circle {
    fill: ` + s(w.Get("annotationFill")) + `;
    stroke: ` + s(w.Get("annotationStroke")) + `;
  }
  .wardley-annotation text {
    fill: ` + s(w.Get("annotationTextColor")) + `;
  }
  .wardley-annotations-box rect {
    fill: ` + s(w.Get("annotationFill")) + `;
    stroke: ` + s(w.Get("annotationStroke")) + `;
  }
  .wardley-annotations-box text {
    fill: ` + s(w.Get("annotationTextColor")) + `;
  }
  .wardley-pipeline-box {
    stroke: ` + s(w.Get("componentStroke")) + `;
  }
  .wardley-notes text {
    fill: ` + s(w.Get("axisTextColor")) + `;
  }
  `
}
