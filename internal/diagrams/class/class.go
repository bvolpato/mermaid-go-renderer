// Package class is the class diagram.
//
// Ported from mermaid 12.2.0, diagrams/class/classDiagram-v2.ts, classDb.ts,
// classTypes.ts, classRenderer-v3-unified.ts, shapeUtil.ts, the semantic
// actions of parser/classDiagram.jison and
// rendering-elements/shapes/classBox.ts (MIT).
//
// The package registers the diagram type "classDiagram" and the node shape
// "classBox". Upstream has two definitions, classDiagram.ts and
// classDiagram-v2.ts, with the same content, and one detector for both
// keywords (classDetector-V2.ts).
package class

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	render.Register(&render.Definition{
		ID:  "classDiagram",
		New: func(ctx *render.Context) render.DB { return NewDB(ctx) },
		// The init(cnf) hook of upstream sets cnf.class.arrowMarkerAbsolute
		// on the object that getConfig() returned. That object is a copy,
		// so the hook changes nothing that a later getConfig() returns.
		Parse: func(_ *render.Context, d render.DB, text string) error {
			cdb, ok := d.(*DB)
			if !ok {
				return fmt.Errorf("class: unexpected db type %T", d)
			}
			return parse(cdb, text)
		},
		Draw: func(ctx *render.Context, d render.DB, _ string) error {
			cdb, ok := d.(*DB)
			if !ok {
				return fmt.Errorf("class: unexpected db type %T", d)
			}
			return draw(ctx, cdb)
		},
		Classes: func(_ *render.Context, d render.DB, _ string) *js.Map[string, *render.ClassDef] {
			cdb, ok := d.(*DB)
			if !ok {
				return nil
			}
			return getClasses(cdb)
		},
	})
	ru.RegisterShape("classBox", classBox)
}
