// Ported from mermaid 12.2.0, diagrams/agentflow/shapes.ts and
// transformData.ts (MIT).

package agentflow

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// shapeAliases is SHAPE_ALIASES: the shape aliases of v0.8.1. The names of
// the authors map to the shape ids of mermaid. "round" is a legacy alias of
// "rect", which the render pass then changes to defaultShape.
var shapeAliases = map[string]string{
	"task":     "roundedRect",
	"tool":     "subroutine",
	"input":    "lean-right",
	"decision": "diamond",
	"refdoc":   "lin-doc",
	"action":   "hexagon",
	"round":    "rect",
}

// removedShapes is REMOVED_SHAPES: the shapes that v0.8.1 removed. A node
// with such a shape is an error (SHAPE_REMOVED), and the node gets
// defaultShape.
var removedShapes = map[string]bool{
	"doc":             true,
	"stadium":         true,
	"terminal":        true,
	"circle":          true,
	"trapezoid":       true,
	"inv_trapezoid":   true,
	"inv-trapezoid":   true,
	"doublecircle":    true,
	"double-circle":   true,
	"typeDeclaration": true,
	"procs":           true,
	"lean_left":       true,
	"lean-left":       true,
	"in-out":          true,
	"cylinder":        true,
	"ellipse":         true,
	"odd":             true,
	// Instance shapes: the instancing mechanism is removed.
	"tag-rect":               true,
	"tagged-rectangle":       true,
	"delay":                  true,
	"half-rounded-rectangle": true,
	"lin-rect":               true,
	"lined-rectangle":        true,
	"win-pane":               true,
	"window-pane":            true,
	"curv-trap":              true,
	"curved-trapezoid":       true,
}

// allowedShapes is ALLOWED_SHAPES: the shapes that the v0.8.1 spec allows,
// as shape ids of mermaid after the alias resolution. "rect" and
// "squareRect" become defaultShape before this set is read.
var allowedShapes = map[string]bool{
	"roundedRect":      true,
	"subroutine":       true,
	"subprocess":       true,
	"subproc":          true,
	"framed-rectangle": true,
	"lean-right":       true,
	"diamond":          true,
	"lin-doc":          true,
	"lined-document":   true,
	"hexagon":          true,
	"hex":              true,
	"connector":        true,
	// Collapsed flow container
	"collapsedGroup": true,
}

// defaultShape is DEFAULT_SHAPE.
const defaultShape = "roundedRect"

// resolveShapeAlias is resolveShapeAlias(shape): the shape id of mermaid
// for a shape name. A name with no alias stays as it is.
func resolveShapeAlias(shape string) string {
	if shape == "" {
		return shape
	}
	if resolved, ok := shapeAliases[shape]; ok {
		return resolved
	}
	return shape
}

// shapeDiagnosticSink is ShapeDiagnosticSink: the receiver of the shape
// diagnostics. The db implements it.
type shapeDiagnosticSink interface {
	EmitError(id, message string, ctx DiagnosticContext)
	EmitWarning(id, message string, ctx DiagnosticContext)
}

// normaliseNodeShapes is normaliseNodeShapes(nodes, sink): it changes the
// shape of every node to a supported shape id, and reports the removed and
// the unsupported shapes to the sink. A group node keeps its cluster shape.
// With a nil sink upstream writes the message to the log.
func normaliseNodeShapes(nodes []*ru.Node, sink shapeDiagnosticSink) {
	for _, node := range nodes {
		// Group/cluster nodes use specific cluster shapes: don't override
		if node.IsGroup {
			continue
		}

		// Map rect/squareRect (incl. the `round` to `rect` alias) to the canonical
		// rounded default.
		if node.Shape == "" || node.Shape == "squareRect" || node.Shape == "rect" {
			node.Shape = defaultShape
		}

		if removedShapes[node.Shape] {
			msg := `shape "` + node.Shape + `" was removed in v0.8.1, using "` + defaultShape + `"`
			if sink != nil {
				sink.EmitError(WarningShapeRemoved, msg, nodeContext(node.ID))
			}
			node.Shape = defaultShape
			continue
		}

		if !allowedShapes[node.Shape] {
			msg := `shape "` + node.Shape + `" is not supported, using "` + defaultShape + `"`
			if sink != nil {
				sink.EmitWarning(WarningShapeUnsupported, msg, nodeContext(node.ID))
			}
			node.Shape = defaultShape
		}
	}
}

// TransformData is transformData(data, db): it changes the layout data of a
// flowchart to the form of agentflow. GetData already does this, so the
// renderer gets normalised shapes. The function stays for a caller that has
// layout data from another source. db can be nil.
func TransformData(data *ru.LayoutData, db *DB) {
	if db == nil {
		normaliseNodeShapes(data.Nodes, nil)
		return
	}
	normaliseNodeShapes(data.Nodes, db)
}
