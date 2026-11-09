// Ported from mermaid 12.2.0, diagrams/state/stateCommon.ts (MIT).

package state

// Constants common to all state diagram code.
const (
	// defaultDiagramDirection is the default diagram direction.
	defaultDiagramDirection = "TB"
	// defaultNestedDocDir is the default direction for any nested
	// documents (composites).
	defaultNestedDocDir = "TB"

	// stmtDirection is the parsed statement type for a direction.
	stmtDirection = "dir"
	// stmtState is the parsed statement type for a state.
	stmtState = "state"
	// stmtRoot is the parsed statement type for a root.
	stmtRoot = "root"
	// stmtRelation is the parsed statement type for a relation.
	stmtRelation = "relation"
	// stmtClassDef is the parsed statement type for a classDef.
	stmtClassDef = "classDef"
	stmtStyleDef = "style"
	// stmtApplyClass is the parsed statement type for applyClass.
	stmtApplyClass = "applyClass"

	defaultStateType = "default"
	dividerType      = "divider"

	// Graph edge settings.
	gEdgeStyle          = "fill:none"
	gEdgeArrowheadStyle = "fill: #333"
	gEdgeLabelpos       = "c"
	gEdgeLabelType      = "markdown"
	gEdgeThickness      = "normal"

	shapeState         = "rect"
	shapeStateWithDesc = "rectWithTitle"
	shapeStart         = "stateStart"
	shapeEnd           = "stateEnd"
	shapeDivider       = "divider"
	shapeGroup         = "roundedWithTitle"
	shapeNote          = "note"
	shapeNoteGroup     = "noteGroup"

	// CSS classes.
	cssDiagram           = "statediagram"
	cssState             = "state"
	cssDiagramState      = cssDiagram + "-" + cssState
	cssEdge              = "transition"
	cssNote              = "note"
	cssNoteEdge          = "note-edge"
	cssEdgeNoteEdge      = cssEdge + " " + cssNoteEdge
	cssDiagramNote       = cssDiagram + "-" + cssNote
	cssCluster           = "cluster"
	cssDiagramCluster    = cssDiagram + "-" + cssCluster
	cssClusterAlt        = "cluster-alt"
	cssDiagramClusterAlt = cssDiagram + "-" + cssClusterAlt

	// parentName and noteName are PARENT and NOTE.
	parentName      = "parent"
	noteName        = "note"
	domIDState      = "state"
	domIDTypeSpacer = "----"
	noteID          = domIDTypeSpacer + noteName
	parentID        = domIDTypeSpacer + parentName
)
