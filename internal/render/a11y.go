// Ported from mermaid 12.2.0, accessibility.ts (MIT).

package render

import "github.com/bvolpato/mermaid-go-renderer/internal/dom"

const svgRole = "graphics-document document"

// setA11yDiagramInfo sets the role and the role description of the svg.
func setA11yDiagramInfo(svg *dom.Node, diagramType string) {
	svg.Attr("role", svgRole)
	if diagramType != "" {
		svg.Attr("aria-roledescription", diagramType)
	}
}

// addSVGa11yTitleDescription inserts the accessible title and description.
func addSVGa11yTitleDescription(svg *dom.Node, a11yTitle, a11yDesc, baseID string) {
	if a11yDesc != "" {
		descID := "chart-desc-" + baseID
		svg.Attr("aria-describedby", descID)
		svg.Insert("desc", ":first-child").Attr("id", descID).Text(a11yDesc)
	}
	if a11yTitle != "" {
		titleID := "chart-title-" + baseID
		svg.Attr("aria-labelledby", titleID)
		svg.Insert("title", ":first-child").Attr("id", titleID).Text(a11yTitle)
	}
}
