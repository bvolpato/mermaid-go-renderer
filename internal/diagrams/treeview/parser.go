// Ported from mermaid 12.2.0, diagrams/treeView/parser.ts (MIT).

package treeview

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// populate is populate(ast).
func populate(ctx *render.Context, ast *TreeView, db *DB) {
	ast.Populate(db)
	for _, node := range ast.Nodes {
		level := 0
		if node.Indent != nil {
			level = *node.Indent
		}

		// Name comes pre-cleaned from value converter (quotes stripped, etc.)
		name := node.Name

		// Detect directory: trailing / on the name
		isDirectory := strings.HasSuffix(name, "/")
		if isDirectory {
			name = name[:len(name)-1]
		}
		nodeType := fileNode
		if isDirectory {
			nodeType = directoryNode
		}

		// Read annotations directly from AST fields (cleaned by value converter)
		cssClass := ""
		if node.ClassAnnotation != nil {
			cssClass = *node.ClassAnnotation
		}

		// Icon: value converter extracts the iconify name from icon(name).
		// Empty string from icon() means suppress icon. Without an
		// annotation the icon stays undefined. The defaults are resolved at
		// render time (showIcons).
		icon := ""
		if node.IconAnnotation != nil {
			icon = *node.IconAnnotation
			if icon == "" {
				icon = "none"
			}
		}

		// Description comes pre-trimmed from value converter; sanitize for defense in depth
		description, hasDescription := "", false
		if node.DescAnnotation != nil && *node.DescAnnotation != "" {
			description, hasDescription = ctx.SanitizeText(*node.DescAnnotation), true
		}

		db.AddNode(level, name, nodeType, cssClass, icon, description, hasDescription)
	}
}

// parseError is an error of the parser with the line numbers of the text
// before the box-drawing characters were replaced.
type parseError struct {
	message string
	err     error
}

func (e *parseError) Error() string { return e.message }
func (e *parseError) Unwrap() error { return e.err }

// parseDiagram is parser.parse(input).
func parseDiagram(ctx *render.Context, rdb render.DB, input string) error {
	db := rdb.(*DB)
	text, lineMap, err := preprocessBoxDrawing(input)
	if err != nil {
		return err
	}
	ast, err := parse(text)
	if err != nil {
		if lineMap.Len() > 0 {
			return &parseError{message: remapErrorLines(err.Error(), lineMap), err: err}
		}
		return err
	}
	populate(ctx, ast, db)
	return nil
}
