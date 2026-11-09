// Ported from mermaid 12.2.0, diagrams/er/erDb.ts (MIT).

package er

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// The values of ErDB.Cardinality.
const (
	CardinalityZeroOrOne  = "ZERO_OR_ONE"
	CardinalityZeroOrMore = "ZERO_OR_MORE"
	CardinalityOneOrMore  = "ONE_OR_MORE"
	CardinalityOnlyOne    = "ONLY_ONE"
	CardinalityMdParent   = "MD_PARENT"
)

// The values of ErDB.Identification.
const (
	IdentificationNonIdentifying = "NON_IDENTIFYING"
	IdentificationIdentifying    = "IDENTIFYING"
)

// DB is ErDB: the data of a parsed ER diagram.
type DB struct {
	render.CommonDB
	ctx *render.Context

	entities      *js.Map[string, *EntityNode]
	relationships []*Relationship
	classes       *js.Map[string, *EntityClass]
	// SubgraphDepth is public upstream: the actions of the grammar read and
	// write it.
	SubgraphDepth  int
	subGraphs      []*ErSubGraph
	subGraphLookup *js.Map[string, *ErSubGraph]
	subCount       int
	direction      string
}

// NewDB returns a db in the state after the constructor of ErDB.
func NewDB(ctx *render.Context) *DB {
	d := &DB{ctx: ctx, direction: "TB"}
	d.CommonDB.Sanitize = ctx.SanitizeText
	d.Clear()
	return d
}

// getConfig is getConfig() of diagramAPI. The pipeline keeps the current
// config in the context.
func (d *DB) getConfig() *js.Obj[any] {
	return d.ctx.Config
}

// AddEntity is addEntity(name, alias): it adds an entity, or gives an
// alias to an entity that has none. The default of alias is the empty
// string.
func (d *DB) AddEntity(name string, alias ...string) *EntityNode {
	a := ""
	if len(alias) > 0 {
		a = alias[0]
	}
	if !d.entities.Has(name) {
		// getConfig().look ?? 'default'
		look := "default"
		if v := d.getConfig().Get("look"); !js.IsNullish(v) {
			look = js.ToString(v)
		}
		d.entities.Set(name, &EntityNode{
			ID:         "entity-" + name + "-" + js.Str(float64(d.entities.Len())),
			Label:      name,
			Attributes: []*Attribute{},
			Alias:      a,
			Shape:      "erBox",
			Look:       look,
			CSSClasses: "default",
			CSSStyles:  []string{},
			LabelType:  "markdown",
		})
	} else if entity := d.entities.Get(name); entity.Alias == "" && a != "" {
		entity.Alias = a
	}
	return d.entities.Get(name)
}

// GetEntity is getEntity(name). The result is nil when there is no entity
// with the name.
func (d *DB) GetEntity(name string) *EntityNode {
	return d.entities.Get(name)
}

// GetEntities is getEntities().
func (d *DB) GetEntities() *js.Map[string, *EntityNode] {
	return d.entities
}

// GetClasses is getClasses().
func (d *DB) GetClasses() *js.Map[string, *EntityClass] {
	return d.classes
}

// AddAttributes is addAttributes(entityName, attribs).
func (d *DB) AddAttributes(entityName string, attribs []*Attribute) {
	entity := d.AddEntity(entityName) // May do nothing (if entity has already been added)

	// Process attribs in reverse order due to effect of recursive construction (last attribute is first)
	for i := len(attribs) - 1; i >= 0; i-- {
		if attribs[i].Keys == nil {
			attribs[i].Keys = []string{}
		}
		// A comment that is undefined becomes ''. Both are "" here.
		entity.Attributes = append(entity.Attributes, attribs[i])
	}
}

// AddRelationship is addRelationship(entA, rolA, entB, rSpec): entA and
// entB are names of entities or ids of subgraphs, rolA is the role of the
// first entity in relation to the second, and rSpec has the details.
func (d *DB) AddRelationship(entA, rolA, entB string, rSpec *RelSpec) {
	// Check if entA is a subgraph, otherwise treat it as an entity
	var entityAID string
	if d.subGraphLookup.Has(entA) {
		entityAID = entA
	} else {
		entityAID = d.AddEntity(entA).ID
	}

	// Check if entB is a subgraph, otherwise treat it as an entity
	var entityBID string
	if d.subGraphLookup.Has(entB) {
		entityBID = entB
	} else {
		entityBID = d.AddEntity(entB).ID
	}

	d.relationships = append(d.relationships, &Relationship{
		EntityA: entityAID,
		RoleA:   rolA,
		EntityB: entityBID,
		RelSpec: rSpec,
	})
}

// GetRelationships is getRelationships().
func (d *DB) GetRelationships() []*Relationship {
	return d.relationships
}

// GetDirection is getDirection().
func (d *DB) GetDirection() string {
	return d.direction
}

// SetDirection is setDirection(dir).
func (d *DB) SetDirection(dir string) {
	d.direction = dir
}

// trimAll is list.map((s) => s.trim()).
func trimAll(list []string) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = js.TrimSpace(s)
	}
	return out
}

// getCompiledStyles is getCompiledStyles(classDefs): the styles of the
// classes with the given names.
func (d *DB) getCompiledStyles(classDefs []string) []string {
	compiledStyles := []string{}
	for _, customClass := range classDefs {
		cssClass, ok := d.classes.Lookup(customClass)
		if !ok {
			continue
		}
		if cssClass.Styles != nil {
			compiledStyles = trimAll(append(compiledStyles, cssClass.Styles...))
		}
		if cssClass.TextStyles != nil {
			compiledStyles = trimAll(append(compiledStyles, cssClass.TextStyles...))
		}
	}
	return compiledStyles
}

// AddCssStyles is addCssStyles(ids, styles). A nil styles is undefined.
func (d *DB) AddCssStyles(ids []string, styles []string) {
	for _, id := range ids {
		entity, hasEntity := d.entities.Lookup(id)
		subGraph, hasSubGraph := d.subGraphLookup.Lookup(id)

		if styles == nil {
			continue
		}

		if hasEntity {
			entity.CSSStyles = append(entity.CSSStyles, styles...)
		}

		if hasSubGraph {
			if subGraph.CSSStyles == nil {
				subGraph.CSSStyles = []string{}
			}
			subGraph.CSSStyles = append(subGraph.CSSStyles, styles...)
		}
	}
}

// AddClass is addClass(ids, style). A nil style is undefined.
func (d *DB) AddClass(ids []string, style []string) {
	for _, id := range ids {
		classNode, ok := d.classes.Lookup(id)
		if !ok {
			classNode = &EntityClass{ID: id, Styles: []string{}, TextStyles: []string{}}
			d.classes.Set(id, classNode)
		}

		for _, s := range style {
			if strings.Contains(s, "color") {
				newStyle := strings.Replace(s, "fill", "bgFill", 1)
				classNode.TextStyles = append(classNode.TextStyles, newStyle)
			}
			classNode.Styles = append(classNode.Styles, s)
		}
	}
}

// DirStmt is the value of a direction statement in a document:
// { stmt: 'dir', value }.
type DirStmt struct {
	Stmt  string
	Value string
}

// AddSubGraph is addSubGraph(_id, list, _title). id is _id.text and title
// is _title.text. _title has no type, so the label type is the default.
// list is the document of the subgraph: strings and *DirStmt values. The
// result is the id of the subgraph.
func (d *DB) AddSubGraph(idText string, list []any, titleText string) string {
	id := js.TrimSpace(idText)
	title := titleText

	// uniq: the names of the list without the statements, the empty names
	// and the names that came before, and the direction of the last
	// direction statement.
	seen := js.NewSet[string]()
	dir, hasDir := "", false
	nodeList := []string{}
	for _, item := range list {
		if stmt, ok := item.(*DirStmt); ok && stmt != nil {
			// A statement object is truthy when its stmt is not empty.
			if stmt.Stmt != "" {
				if stmt.Stmt == "dir" {
					dir, hasDir = stmt.Value, true
				}
				continue
			}
		}

		name, ok := item.(string)
		if !ok {
			continue
		}

		trimmed := js.TrimSpace(name)
		if trimmed == "" {
			continue
		}

		if seen.Has(trimmed) {
			continue
		}
		seen.Add(trimmed)

		nodeList = append(nodeList, name)
	}

	// id is a string, so `id ?? 'subGraph' + this.subCount` is id.
	title = d.ctx.SanitizeText(title)
	d.subCount = d.subCount + 1

	// If no explicit direction is declared within the subgraph, dir stays
	// undefined so that the layout engine applies its own default direction.
	subGraph := &ErSubGraph{
		ID:        id,
		Nodes:     nodeList,
		Title:     js.TrimSpace(title),
		Classes:   []string{},
		CSSStyles: []string{},
		Dir:       dir,
		HasDir:    hasDir,
		LabelType: sanitizeNodeLabelType(""),
	}

	// Ensure nodes are unique across subgraphs by removing duplicates from the new subgraph
	subGraph.Nodes = d.makeUniq(subGraph, d.subGraphs)
	d.subGraphs = append(d.subGraphs, subGraph)
	d.subGraphLookup.Set(id, subGraph)
	return id
}

// GetSubGraphs is getSubGraphs().
func (d *DB) GetSubGraphs() []*ErSubGraph {
	return d.subGraphs
}

// SetClass is setClass(ids, classNames).
func (d *DB) SetClass(ids []string, classNames []string) {
	for _, id := range ids {
		if entity, ok := d.entities.Lookup(id); ok {
			for _, className := range classNames {
				entity.CSSClasses += " " + className
			}
		}

		if subGraph, ok := d.subGraphLookup.Lookup(id); ok {
			subGraph.Classes = append(subGraph.Classes, classNames...)
		}
	}
}

// subgraphNodeCache builds a lookup for all node ids that are already
// assigned to existing subgraphs.
func subgraphNodeCache(allSubgraphs []*ErSubGraph) *js.Set[string] {
	nodeCache := js.NewSet[string]()
	for _, subGraph := range allSubgraphs {
		for _, id := range subGraph.Nodes {
			nodeCache.Add(id)
		}
	}
	return nodeCache
}

// makeUniq filters out the nodes that are already part of another
// subgraph, so that subgraph membership is unique.
func (d *DB) makeUniq(subGraph *ErSubGraph, allSubgraphs []*ErSubGraph) []string {
	existingNodes := subgraphNodeCache(allSubgraphs)
	res := []string{}
	for _, id := range subGraph.Nodes {
		if !existingNodes.Has(id) {
			res = append(res, id)
		}
	}
	return res
}

// sanitizeNodeLabelType is sanitizeNodeLabelType(labelType). An empty
// string stands for undefined.
func sanitizeNodeLabelType(labelType string) string {
	switch labelType {
	case "markdown", "string", "text":
		return labelType
	}
	return "markdown"
}

// Clear is clear().
func (d *DB) Clear() {
	d.entities = js.NewMap[string, *EntityNode]()
	d.classes = js.NewMap[string, *EntityClass]()
	d.relationships = []*Relationship{}
	d.subGraphs = []*ErSubGraph{}
	d.subGraphLookup = js.NewMap[string, *ErSubGraph]()
	d.subCount = 0
	d.SubgraphDepth = 0
	d.ClearCommon()
}

// lookOf is config.look as the string of a node or an edge. An undefined
// look is "".
func lookOf(cfg *js.Obj[any]) string {
	v := cfg.Get("look")
	if js.IsNullish(v) {
		return ""
	}
	return js.ToString(v)
}

// attributesValue returns the attributes of an entity as JavaScript
// objects: the form of the property `attributes` of a layout node.
func attributesValue(attributes []*Attribute) []any {
	out := make([]any, len(attributes))
	for i, a := range attributes {
		o := js.NewObj[any]()
		o.Set("type", a.Type)
		o.Set("name", a.Name)
		keys := make([]any, len(a.Keys))
		for j, k := range a.Keys {
			keys[j] = k
		}
		o.Set("keys", keys)
		o.Set("comment", a.Comment)
		out[i] = o
	}
	return out
}

// layoutConfig is getConfig() as getData calls it: a copy of the current
// config that the caller can change.
func (d *DB) layoutConfig() *js.Obj[any] {
	if d.ctx.ConfigState != nil {
		return d.ctx.ConfigState.GetConfig()
	}
	cfg, _ := config.AssignWithDepth(js.NewObj[any](), d.ctx.Config, 2).(*js.Obj[any])
	return cfg
}

// GetData is getData(): the nodes and the edges for the unified renderer,
// the config and the direction. An entity node has the properties
// `attributes` and `alias` in Extra.
func (d *DB) GetData() *ru.LayoutData {
	nodes := []*ru.Node{}
	edges := []*ru.Edge{}
	cfg := d.layoutConfig()
	look := lookOf(cfg)

	subGraphs := d.GetSubGraphs()
	parentDB := js.NewMap[string, string]()
	subGraphDB := js.NewMap[string, bool]()

	// Setup the subgraph data for adding nodes
	for i := len(subGraphs) - 1; i >= 0; i-- {
		subGraph := subGraphs[i]
		if len(subGraph.Nodes) > 0 {
			subGraphDB.Set(subGraph.ID, true)
		}
		for _, id := range subGraph.Nodes {
			parentDB.Set(id, subGraph.ID)
		}
	}

	// Add the nodes
	for i := len(subGraphs) - 1; i >= 0; i-- {
		subGraph := subGraphs[i]
		node := ru.NewNode(subGraph.ID)
		node.Label, node.HasLabel = subGraph.Title, true
		node.LabelStyle = ""
		node.LabelType = subGraph.LabelType
		node.ParentID = parentDB.Get(subGraph.ID)
		node.Padding = 8
		node.CSSCompiledStyles = d.getCompiledStyles(subGraph.Classes)
		node.CSSStyles, node.CSSStylesSet = subGraph.CSSStyles, subGraph.CSSStyles != nil
		node.CSSClasses, node.CSSClassesSet = strings.Join(subGraph.Classes, " "), true
		node.Shape = "rect"
		node.Dir = subGraph.Dir
		node.IsGroup = true
		node.Look = look
		nodes = append(nodes, node)
	}

	subGraphIDs := js.NewSet[string]()
	for _, sg := range subGraphs {
		subGraphIDs.Add(sg.ID)
	}
	colorIndex := 0
	for _, entityKey := range d.entities.Keys() {
		if subGraphIDs.Has(entityKey) {
			continue
		}
		entityNode := d.entities.Get(entityKey)
		if entityNode == nil {
			continue
		}
		entityNode.CSSCompiledStyles = d.getCompiledStyles(strings.Split(entityNode.CSSClasses, " "))
		entityNode.ColorIndex, entityNode.HasColorIndex = colorIndex, true
		colorIndex++

		// { ...entityNode, parentId, isGroup: false }
		node := ru.NewNode(entityNode.ID)
		node.Label, node.HasLabel = entityNode.Label, true
		node.Set("attributes", attributesValue(entityNode.Attributes))
		node.Set("alias", entityNode.Alias)
		node.Shape = entityNode.Shape
		node.Look = entityNode.Look
		node.CSSClasses, node.CSSClassesSet = entityNode.CSSClasses, true
		node.CSSStyles, node.CSSStylesSet = entityNode.CSSStyles, true
		node.LabelType = entityNode.LabelType
		node.CSSCompiledStyles = entityNode.CSSCompiledStyles
		node.ColorIndex = float64(entityNode.ColorIndex)
		node.ParentID = parentDB.Get(entityKey)
		node.IsGroup = false
		nodes = append(nodes, node)
	}

	count := 0
	for _, relationship := range d.relationships {
		edge := ru.NewEdge(utils.GetEdgeId(relationship.EntityA, relationship.EntityB, utils.EdgeIdOptions{
			Prefix:  "id",
			Counter: count,
		}, ""))
		count++
		edge.Type = "normal"
		edge.Curve = "basis"
		edge.Start = relationship.EntityA
		edge.End = relationship.EntityB
		edge.Label, edge.HasLabel = relationship.RoleA, true
		edge.Labelpos = "c"
		edge.Thickness = "normal"
		edge.Classes = "relationshipLine"
		edge.ArrowTypeStart = js.ToLowerCase(relationship.RelSpec.CardB)
		edge.ArrowTypeEnd = js.ToLowerCase(relationship.RelSpec.CardA)
		if relationship.RelSpec.RelType == IdentificationIdentifying {
			edge.Pattern = "solid"
		} else {
			edge.Pattern = "dashed"
		}
		edge.Look = look
		edge.LabelType = "markdown"
		// The edge has no style: the layouts and the edge renderer tell an
		// undefined style from an empty one.
		edge.StyleUndefined = true
		edges = append(edges, edge)
	}
	return &ru.LayoutData{Nodes: nodes, Edges: edges, Config: cfg, Direction: d.direction}
}

// GetConfig is getConfig(): the `er` section of the config.
func (d *DB) GetConfig() *js.Obj[any] {
	return js.AsObj(d.getConfig().Get("er"))
}
