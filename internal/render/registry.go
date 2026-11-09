// This file ports no upstream file. It has the registry of the diagram types
// and the interfaces of the data of a parsed diagram.

package render

import (
	"sort"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// DB is the data of a parsed diagram: the `db` object of a diagram
// definition upstream. The pipeline reads optional capabilities through the
// interfaces below.
type DB any

// AccTitler is implemented by a DB that has an accessible title.
type AccTitler interface{ GetAccTitle() string }

// AccDescriber is implemented by a DB that has an accessible description.
type AccDescriber interface{ GetAccDescription() string }

// TitleSetter is implemented by a DB that accepts the front matter title.
type TitleSetter interface{ SetDiagramTitle(title string) }

// ClassDef is DiagramStyleClassDef: a class defined in the diagram text.
type ClassDef struct {
	ID         string
	Styles     []string
	TextStyles []string
}

// Definition is a diagram type: DiagramDefinition upstream.
type Definition struct {
	// ID is the diagram type id that detectType returns.
	ID string
	// New returns an empty db. It stands in for db.clear().
	New func(ctx *Context) DB
	// Init is the optional init(config) hook.
	Init func(ctx *Context, db DB)
	// Parse fills the db from the diagram text.
	Parse func(ctx *Context, db DB, text string) error
	// Draw renders the diagram into ctx.SVG.
	Draw func(ctx *Context, db DB, text string) error
	// Classes is renderer.getClasses: the classDefs that become CSS rules.
	Classes func(ctx *Context, db DB, text string) *js.Map[string, *ClassDef]
}

var (
	registryMu sync.RWMutex
	registry   = map[string]*Definition{}
)

// Register adds a diagram type. Diagram packages call it from init.
func Register(def *Definition) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[def.ID] = def
}

// Lookup returns the definition of a diagram type.
func Lookup(id string) (*Definition, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	def, ok := registry[id]
	return def, ok
}

// Types returns the ids of the registered diagram types.
func Types() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// CommentPreserver is a db that sets preserveCommentsWhenParsing: its
// diagram is parsed from the text with comments.
type CommentPreserver interface {
	PreserveCommentsWhenParsing() bool
}

// FrontmatterOffsetSetter is a db with setFrontmatterLineOffset: it gets the
// number of lines of the front matter, to report positions of the original
// source.
type FrontmatterOffsetSetter interface {
	SetFrontmatterLineOffset(offset int)
}
