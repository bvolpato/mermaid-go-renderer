// Ported from mermaid 12.2.0, diagrams/ishikawa/ishikawaDb.ts and
// ishikawaTypes.ts (MIT).

package ishikawa

import "github.com/bvolpato/mermaid-go-renderer/internal/render"

// Node is IshikawaNode: the effect, a cause or a sub-cause.
type Node struct {
	Text     string
	Children []*Node
}

// stackEntry is StackEntry.
type stackEntry struct {
	level int
	node  *Node
}

// DB is IshikawaDB. Upstream makes a new one for each diagram.
type DB struct {
	render.CommonDB

	root  *Node
	stack []stackEntry
	// baseLevel is undefined upstream until the first cause sets it.
	baseLevel    int
	hasBaseLevel bool

	// sanitizeText is common.sanitizeText(text, getConfig()).
	sanitizeText func(text string) string
}

// NewDB returns an empty db. It sanitizes texts with the sanitizer and the
// config of ctx at the time of each call, as ishikawaDb.ts does with
// getConfig().
func NewDB(ctx *render.Context) *DB {
	db := &DB{}
	db.sanitizeText = ctx.SanitizeText
	db.Sanitize = ctx.SanitizeText
	return db
}

// Clear is clear().
func (db *DB) Clear() {
	db.root = nil
	db.stack = nil
	db.baseLevel, db.hasBaseLevel = 0, false
	db.ClearCommon()
}

// GetRoot is getRoot(). It returns nil when the diagram has no node.
func (db *DB) GetRoot() *Node { return db.root }

// AddNode is addNode(rawLevel, text).
func (db *DB) AddNode(rawLevel int, text string) {
	label := text
	if db.sanitizeText != nil {
		label = db.sanitizeText(text)
	}

	if db.root == nil {
		db.root = &Node{Text: label, Children: []*Node{}}
		db.stack = []stackEntry{{level: 0, node: db.root}}
		db.SetDiagramTitle(label)
		return
	}

	// Set baseLevel from the first cause (not the effect/root line),
	// so that relative indentation between causes is preserved
	// even when the effect line is indented more than the causes.
	if !db.hasBaseLevel {
		db.baseLevel, db.hasBaseLevel = rawLevel, true
	}

	level := rawLevel - db.baseLevel + 1
	if level <= 0 {
		level = 1
	}

	// Pop stack until the top has a strictly lower level (= parent)
	for len(db.stack) > 1 && db.stack[len(db.stack)-1].level >= level {
		db.stack = db.stack[:len(db.stack)-1]
	}

	parent := db.stack[len(db.stack)-1].node
	node := &Node{Text: label, Children: []*Node{}}
	parent.Children = append(parent.Children, node)
	db.stack = append(db.stack, stackEntry{level: level, node: node})
}
