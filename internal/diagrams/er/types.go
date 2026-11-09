// Ported from mermaid 12.2.0, diagrams/er/erTypes.ts (MIT).

package er

import "github.com/bvolpato/mermaid-go-renderer/internal/render"

// EntityNode is EntityNode: one entity of the diagram.
type EntityNode struct {
	ID         string
	Label      string
	Attributes []*Attribute
	Alias      string
	Shape      string
	// Look is `getConfig().look ?? 'default'`.
	Look       string
	CSSClasses string
	CSSStyles  []string
	// CSSCompiledStyles is undefined (nil) until getData sets it.
	CSSCompiledStyles []string
	LabelType         string
	// ColorIndex is undefined until getData sets it: HasColorIndex says so.
	ColorIndex    int
	HasColorIndex bool
}

// Attribute is Attribute: one row of an entity.
type Attribute struct {
	Type string
	Name string
	// Keys is undefined (nil) for an attribute without keys until
	// addAttributes makes it an empty list.
	Keys    []string
	Comment string
}

// Relationship is Relationship. EntityA and EntityB are the ids of the
// entities, or the id of a subgraph.
type Relationship struct {
	EntityA string
	RoleA   string
	EntityB string
	RelSpec *RelSpec
}

// RelSpec is RelSpec: the cardinalities and the identification of a
// relationship.
type RelSpec struct {
	CardA   string
	CardB   string
	RelType string
}

// EntityClass is EntityClass: a class of a classDef statement. It has the
// fields of DiagramStyleClassDef.
type EntityClass = render.ClassDef

// ErSubGraph is ErSubGraph: a group of entities.
type ErSubGraph struct {
	Classes   []string
	CSSStyles []string
	// Dir is the direction of a direction statement in the subgraph. HasDir
	// is false when dir is undefined.
	Dir       string
	HasDir    bool
	ID        string
	LabelType string
	Nodes     []string
	Title     string
}
