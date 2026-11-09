// Ported from mermaid 12.2.0, diagrams/treemap/db.ts and types.ts (MIT).

package treemap

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// Node is TreemapNode: a section or a leaf of the treemap.
type Node struct {
	Name string
	// Children is nil for a leaf: the property is undefined upstream. A
	// section has a list, which can be empty.
	Children []*Node
	// Value is the value of a leaf. HasValue is false when the property is
	// undefined.
	Value    float64
	HasValue bool
	// ClassSelector is "" when the property is undefined.
	ClassSelector string
	// CSSCompiledStyles is nil when the property is undefined.
	CSSCompiledStyles []string
}

// DB is TreeMapDB: the db of the treemap diagram.
type DB struct {
	render.CommonDB

	ctx        *render.Context
	nodes      []*Node
	levels     map[*Node]int
	outerNodes []*Node
	classes    *js.Map[string, *render.ClassDef]
	root       *Node
}

// NewDB returns an empty db. It reads the config of ctx at the time of each
// call, as db.ts does with getConfig().
func NewDB(ctx *render.Context) *DB {
	db := &DB{
		ctx:     ctx,
		levels:  map[*Node]int{},
		classes: js.NewMap[string, *render.ClassDef](),
	}
	sanitizer := sanitize.New()
	db.Sanitize = func(text string) string {
		return sanitizer.SanitizeText(text, ctx.Config)
	}
	return db
}

// GetNodes is getNodes().
func (db *DB) GetNodes() []*Node {
	return db.nodes
}

// GetConfig is getConfig(): the treemap config of the default config with
// the treemap config of the current config on top.
func (db *DB) GetConfig() *js.Obj[any] {
	merged := js.NewObj[any]()
	spread := func(v any) {
		// A spread of undefined or null adds nothing.
		keys, ok := js.OwnKeys(v)
		if !ok {
			return
		}
		for _, key := range keys {
			value, _ := js.GetProperty(v, key)
			merged.Set(key, value)
		}
	}
	spread(db.ctx.ConfigState.DefaultSection("treemap"))
	// ...(userConfig.treemap ?? {})
	spread(db.ctx.Cfg("treemap"))
	return utils.CleanAndMerge(merged, nil)
}

// AddNode is addNode(node, level).
func (db *DB) AddNode(node *Node, level int) {
	db.nodes = append(db.nodes, node)
	db.levels[node] = level
	if level == 0 {
		db.outerNodes = append(db.outerNodes, node)
		if db.root == nil {
			db.root = node
		}
	}
}

// GetRoot is getRoot(): a node without a name that holds the outer nodes.
func (db *DB) GetRoot() *Node {
	children := db.outerNodes
	if children == nil {
		children = []*Node{}
	}
	return &Node{Name: "", Children: children}
}

// AddClass is addClass(id, _style).
func (db *DB) AddClass(id, style string) {
	styleClass, ok := db.classes.Lookup(id)
	if !ok {
		styleClass = &render.ClassDef{ID: id, Styles: []string{}, TextStyles: []string{}}
	}
	// _style.replace(/\\,/g, '§§§').replace(/,/g, ';').replace(/§§§/g, ',').split(';')
	styles := strings.ReplaceAll(style, `\,`, "§§§")
	styles = strings.ReplaceAll(styles, ",", ";")
	styles = strings.ReplaceAll(styles, "§§§", ",")
	for _, s := range strings.Split(styles, ";") {
		if ru.IsLabelStyle(s) {
			styleClass.TextStyles = append(styleClass.TextStyles, s)
		}
		styleClass.Styles = append(styleClass.Styles, s)
	}
	db.classes.Set(id, styleClass)
}

// GetClasses is getClasses().
func (db *DB) GetClasses() *js.Map[string, *render.ClassDef] {
	return db.classes
}

// GetStylesForClass is getStylesForClass(classSelector).
func (db *DB) GetStylesForClass(classSelector string) []string {
	if styleClass, ok := db.classes.Lookup(classSelector); ok && styleClass.Styles != nil {
		return styleClass.Styles
	}
	return []string{}
}

// Clear is clear().
func (db *DB) Clear() {
	db.ClearCommon()
	db.nodes = nil
	db.levels = map[*Node]int{}
	db.outerNodes = nil
	db.classes = js.NewMap[string, *render.ClassDef]()
	db.root = nil
}
