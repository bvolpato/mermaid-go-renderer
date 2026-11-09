// Ported from mermaid 12.2.0, the diagram registrations of mermaid:
// addStylesForDiagram in mermaid/src/styles.ts, registerDiagram in
// diagram-api/diagramAPI.ts, addDiagrams in
// diagram-api/diagram-orchestration.ts and the "styles" field of each diagram
// definition (MIT).

package styles

import "slices"

// stylesProvider is DiagramStylesProvider: the function that makes the CSS
// of one diagram type. It can throw. See throw in jsutil.go.
type stylesProvider func(o *options) string

// noStyles is the style function of a diagram that returns an empty string
// (quadrantChart and eventmodeling).
func noStyles(*options) string {
	return ""
}

// themes maps a diagram type to its style function. The keys are the ids of
// the diagram detectors.
//
// Mermaid fills this table when it loads a diagram, and a render loads only
// the diagram that it draws. The port has the full table from the start. The
// result is the same, because getStyles reads only the entry of its type.
//
// The types error, info and xychart have no style function, so they have no
// entry: their style sheet has only the common rules.
var themes = map[string]stylesProvider{
	// The "---" diagram has an object where the function must be, so mermaid
	// throws a TypeError when it makes the styles.
	"---": func(*options) string {
		throwf("TypeError: themes[type] is not a function")
		return ""
	},

	"flowchart-elk": flowchartStyles,
	"mindmap":       mindmapStyles,
	"architecture":  architectureStyles,

	"agentflow":     agentflowStyles,
	"c4":            c4Styles,
	"kanban":        kanbanStyles,
	"classDiagram":  classStyles,
	"er":            erStyles,
	"gantt":         ganttStyles,
	"pie":           pieStyles,
	"requirement":   requirementStyles,
	"sequence":      sequenceStyles,
	"swimlane":      swimlanesStyles,
	"flowchart-v2":  flowchartStyles,
	"timeline":      timelineStyles,
	"gitGraph":      gitStyles,
	"stateDiagram":  stateStyles,
	"journey":       journeyStyles,
	"quadrantChart": noStyles,
	"sankey":        sankeyStyles,
	"packet":        packetStyles,
	"block":         blockStyles,
	"eventmodeling": noStyles,
	"treeView":      treeViewStyles,
	"radar":         radarStyles,
	"ishikawa":      ishikawaStyles,
	"treemap":       treemapStyles,
	"railroad":      railroadStyles,
	"railroadEbnf":  railroadStyles,
	"railroadAbnf":  railroadStyles,
	"railroadPeg":   railroadStyles,
	"venn":          vennStyles,
	"wardley":       wardleyStyles,
	"cynefin":       cynefinStyles,
	"usecase":       usecaseStyles,
}

// DiagramTypes returns the diagram types of mermaid 12.2.0, sorted. A type
// that is not in the list gets only the common rules.
func DiagramTypes() []string {
	types := []string{"error", "info", "xychart"}
	for typ := range themes {
		if typ != "---" {
			types = append(types, typ)
		}
	}
	slices.Sort(types)
	return types
}
