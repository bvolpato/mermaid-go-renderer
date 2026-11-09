// Package treeview is the treeView diagram: a tree of files and directories.
//
// Ported from mermaid 12.2.0, diagrams/treeView/diagram.ts (MIT). The other
// files of the package port db.ts, parser.ts, boxDrawingPreprocessor.ts,
// icons.ts, renderer.ts and types.ts, and the grammar
// parser/src/language/treeView/treeView.langium with its value converter.
package treeview

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "treeView",
		New:   func(ctx *render.Context) render.DB { return newDB(ctx) },
		Parse: parseDiagram,
		Draw:  draw,
	})
}
