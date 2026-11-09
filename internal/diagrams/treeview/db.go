// Ported from mermaid 12.2.0, diagrams/treeView/db.ts and types.ts (MIT).

package treeview

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// NodeType is "file" or "directory".
type NodeType string

const (
	fileNode      NodeType = "file"
	directoryNode NodeType = "directory"
)

// BBox is the box of a node, which the renderer sets.
type BBox struct {
	X, Y, Width, Height float64
}

// Node is a node of the tree. Icon and CSSClass are "" when they are
// undefined upstream: the parser never stores an empty text for them.
// Description can be an empty text, so HasDescription tells it apart from
// undefined.
type Node struct {
	ID       int
	Level    int
	Name     string
	NodeType NodeType
	// Icon is an iconify icon reference (`pack:name`), a built-in icon
	// (`file`/`folder`), or `none`.
	Icon           string
	CSSClass       string
	Description    string
	HasDescription bool
	BBox           *BBox
	Children       []*Node
}

// DB is the db of the treeView diagram. cnt and stack are the records of
// the ImperativeState of upstream.
type DB struct {
	render.CommonDB
	ctx   *render.Context
	cnt   int
	stack []*Node
}

func newDB(ctx *render.Context) *DB {
	db := &DB{
		ctx: ctx,
		cnt: 1,
		stack: []*Node{
			{
				ID:       0,
				Level:    -1,
				Name:     "/",
				NodeType: directoryNode,
				Children: []*Node{},
			},
		},
	}
	db.Sanitize = ctx.SanitizeText
	return db
}

// GetRoot is getRoot().
func (db *DB) GetRoot() *Node { return db.stack[0] }

// GetCount is getCount().
func (db *DB) GetCount() int { return db.cnt }

// GetConfig is getConfig(): cleanAndMerge(DEFAULT_CONFIG.treeView,
// getConfig().treeView).
func (db *DB) GetConfig() *js.Obj[any] {
	return utils.CleanAndMerge(config.DefaultSection("treeView"), db.ctx.Config.Get("treeView"))
}

// AddNode is addNode(level, name, nodeType, cssClass, icon, description).
func (db *DB) AddNode(level int, name string, nodeType NodeType, cssClass, icon, description string, hasDescription bool) {
	for level <= db.stack[len(db.stack)-1].Level {
		db.stack = db.stack[:len(db.stack)-1]
	}
	node := &Node{
		ID:             db.cnt,
		Level:          level,
		Name:           name,
		NodeType:       nodeType,
		Icon:           icon,
		CSSClass:       cssClass,
		Description:    description,
		HasDescription: hasDescription,
		Children:       []*Node{},
	}
	db.cnt++
	parent := db.stack[len(db.stack)-1]
	parent.Children = append(parent.Children, node)
	db.stack = append(db.stack, node)
}
