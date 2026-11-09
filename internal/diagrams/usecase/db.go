// Ported from mermaid 12.2.0, diagrams/usecase/usecaseDb.ts (MIT).

package usecase

import (
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// DB is the db of the usecase diagram. The module-level state of upstream
// is the field state.
type DB struct {
	render.CommonDB
	ctx   *render.Context
	state *fields
}

// NewDB returns an empty db: the state after clear().
func NewDB(ctx *render.Context) *DB {
	d := &DB{ctx: ctx}
	d.Sanitize = ctx.SanitizeText
	d.state = d.createModel()
	return d
}

// defaultUsecaseConfig is DEFAULT_USECASE_CONFIG: DEFAULT_CONFIG.usecase. It
// is read only: createModel clones it.
var defaultUsecaseConfig = sync.OnceValue(func() *js.Obj[any] {
	section, _ := config.DefaultSection("usecase").(*js.Obj[any])
	if section == nil {
		section = js.NewObj[any]()
	}
	return section
})

// createModel is createModel().
func (d *DB) createModel() *fields {
	modelConfig, _ := js.Clone(defaultUsecaseConfig()).(*js.Obj[any])
	return &fields{
		actors:              js.NewMap[string, *Actor](),
		useCases:            js.NewMap[string, *UseCase](),
		systemBoundaries:    js.NewMap[string, *SystemBoundary](),
		relationships:       []*Relationship{},
		notes:               js.NewMap[string, *UsecaseNote](),
		jsonNodes:           js.NewMap[string, *UsecaseJsonNode](),
		classDefs:           js.NewMap[string, *ClassDef](),
		symbols:             js.NewMap[string, string](),
		direction:           defaultDirection,
		relationshipCounter: 0,
		noteCounter:         0,
		accTitle:            "",
		accDescription:      "",
		config:              modelConfig,
	}
}

// commit is commit(model). The model of upstream is a structured clone;
// here the builder that made the model does not use it again.
func (d *DB) commit(model *fields) {
	d.SetAccTitle(model.accTitle)
	d.SetAccDescription(model.accDescription)
	d.state = model
}

// clear is clear().
func (d *DB) clear() {
	d.state = d.createModel()
	d.ClearCommon()
}

// GetConfig is getConfig().
func (d *DB) GetConfig() *js.Obj[any] {
	c, _ := js.Clone(d.state.config).(*js.Obj[any])
	return c
}

// GetActors is getActors().
func (d *DB) GetActors() *js.Map[string, *Actor] { return d.state.actors }

// GetUseCases is getUseCases().
func (d *DB) GetUseCases() *js.Map[string, *UseCase] { return d.state.useCases }

// GetSystemBoundaries is getSystemBoundaries().
func (d *DB) GetSystemBoundaries() *js.Map[string, *SystemBoundary] { return d.state.systemBoundaries }

// GetRelationships is getRelationships().
func (d *DB) GetRelationships() []*Relationship { return d.state.relationships }

// GetNotes is getNotes().
func (d *DB) GetNotes() *js.Map[string, *UsecaseNote] { return d.state.notes }

// GetJsonNodes is getJsonNodes().
func (d *DB) GetJsonNodes() *js.Map[string, *UsecaseJsonNode] { return d.state.jsonNodes }

// GetClassDefs is getClassDefs().
func (d *DB) GetClassDefs() *js.Map[string, *ClassDef] { return d.state.classDefs }

// GetDirection is getDirection().
func (d *DB) GetDirection() string { return d.state.direction }

// getCompiledStyles is getCompiledStyles(classNames).
func (d *DB) getCompiledStyles(classNames []string) []string {
	compiled := js.NewMap[string, string]()
	for _, className := range append([]string{"default"}, classNames...) {
		definition := d.state.classDefs.Get(className)
		if definition == nil {
			continue
		}
		for _, rawStyle := range definition.Styles {
			style := js.TrimSpace(rawStyle)
			separator := strings.Index(style, ":")
			property := style
			if separator != -1 {
				property = style[:separator]
			}
			property = js.TrimSpace(property)
			if property != "" {
				compiled.Set(property, style)
			}
		}
	}
	return compiled.Values()
}

// escapeJSONPointerPart is escapeJsonPointerPart(part).
func escapeJSONPointerPart(part string) string {
	return escapePointerPart(part)
}

// displayJSONScalar is displayJsonScalar(value).
func displayJSONScalar(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	if js.IsNullish(value) {
		return "null"
	}
	return js.ToString(value)
}

// flattenJSONRows is flattenJsonRows(value, propertyOrder, sanitize): it
// flattens a JSON object once into renderer-ready rows.
func flattenJSONRows(value *js.Obj[any], propertyOrder *js.Obj[[]string], sanitize func(string) string) []UsecaseJsonRow {
	rows := []UsecaseJsonRow{}

	appendRow := func(key, accessibleKey, cellValue string) {
		rows = append(rows, UsecaseJsonRow{
			Key:           sanitize(key),
			AccessibleKey: sanitize(accessibleKey),
			Value:         sanitize(cellValue),
		})
	}

	var visit func(current any, path, pointer string)
	visit = func(current any, path, pointer string) {
		if array, ok := current.([]any); ok {
			if len(array) == 0 {
				appendRow(path, path, "[]")
				return
			}

			// item === null || ['string', 'number', 'boolean'].includes(typeof item)
			scalarArray := true
			for _, item := range array {
				switch item.(type) {
				case string, float64, bool:
				default:
					if !js.IsNullish(item) {
						scalarArray = false
					}
				}
			}
			if scalarArray {
				for index, element := range array {
					key := ""
					if index == 0 {
						key = path
					}
					appendRow(
						key,
						path,
						displayJSONScalar(element),
					)
				}
				return
			}

			for index, element := range array {
				visit(element, path+"["+strconv.Itoa(index)+"]", pointer+"/"+strconv.Itoa(index))
			}
			return
		}

		if object, ok := current.(*js.Obj[any]); ok && object != nil {
			keys, found := propertyOrder.Lookup(pointer)
			if !found {
				keys = object.Keys()
			}
			if len(keys) == 0 {
				appendRow(path, path, "{}")
				return
			}

			for _, key := range keys {
				childPath := key
				if path != "" {
					childPath = path + "." + key
				}
				visit(object.Get(key), childPath, pointer+"/"+escapeJSONPointerPart(key))
			}
			return
		}

		appendRow(path, path, displayJSONScalar(current))
	}

	visit(value, "", "")
	return rows
}

// actorShape is actorShape(actor).
func actorShape(actor *Actor) string {
	switch actor.Type {
	case "hollow":
		return "usecaseActorHollow"
	case "awesome":
		return "usecaseActorAwesome"
	case "icon":
		return "usecaseActorIcon"
	}
	return "usecaseActor"
}

// useCaseShape is useCaseShape(useCase).
func useCaseShape(useCase *UseCase) string {
	if useCase.Shape == "ellipse" {
		if useCase.Business {
			return "usecaseBusiness"
		}
		return "usecaseEllipse"
	}
	return useCase.Shape
}

// associationMarkers is associationMarkers(arrowType).
func associationMarkers(arrowType int) (arrowTypeStart, arrowTypeEnd string) {
	switch arrowType {
	case arrowSolidArrow:
		return "none", "arrow_point"
	case arrowBackArrow:
		return "arrow_point", "none"
	case arrowCircleArrow:
		return "none", "arrow_circle"
	case arrowCrossArrow:
		return "none", "arrow_cross"
	case arrowCircleArrowReversed:
		return "arrow_circle", "none"
	case arrowCrossArrowReversed:
		return "arrow_cross", "none"
	}
	// ARROW_TYPE.LINE_SOLID
	return "none", "none"
}

// relationshipVisual is the result of relationshipVisuals(relationship).
type relationshipVisual struct {
	arrowTypeStart string
	arrowTypeEnd   string
	pattern        string
	// label is set when hasLabel is true.
	label    string
	hasLabel bool
	// labelType is "" for undefined.
	labelType string
}

// relationshipVisuals is relationshipVisuals(relationship).
func relationshipVisuals(relationship *Relationship) relationshipVisual {
	switch relationship.Type {
	case "include", "extend":
		return relationshipVisual{
			arrowTypeStart: "none",
			arrowTypeEnd:   "arrow_point",
			pattern:        "dotted",
			label:          relationship.Type,
			hasLabel:       true,
			labelType:      "text",
		}
	case "generalization":
		return relationshipVisual{
			arrowTypeStart: "none",
			arrowTypeEnd:   "extension",
			pattern:        "solid",
		}
	}
	// association
	visual := relationshipVisual{pattern: "solid"}
	visual.arrowTypeStart, visual.arrowTypeEnd = associationMarkers(relationship.ArrowType)
	if relationship.Label != "" {
		visual.label, visual.hasLabel = relationship.Label, true
	}
	if relationship.LabelType != "" {
		visual.labelType = relationship.LabelType
	}
	return visual
}

// animationClasses is animationClasses(relationship).
func animationClasses(relationship *Relationship) []string {
	if relationship.Animate || relationship.Animation != "" {
		animation := relationship.Animation
		if animation == "" {
			animation = "fast"
		}
		return []string{"edge-animation-" + animation}
	}
	return []string{}
}

// classNames is classNames(...names): the names that are not empty, joined
// with a space.
func classNames(names ...string) string {
	var kept []string
	for _, name := range names {
		if name != "" {
			kept = append(kept, name)
		}
	}
	return strings.Join(kept, " ")
}

// orEmpty is `condition && name` as an argument of classNames.
func orEmpty(condition bool, name string) string {
	if condition {
		return name
	}
	return ""
}

// numberOf is the number in a number field of the layout model for a
// config value: undefined for a missing value. The config can have a value
// that is not a number, for example a string from the front matter. Then
// the caller keeps the JavaScript value in Extra under the name of the
// property, and the field has its numeric value.
func numberOf(v any) (n float64, isNumber bool) {
	if n, ok := v.(float64); ok {
		return n, true
	}
	if v == nil {
		return ru.Undef, false
	}
	return js.ToNumber(v), false
}

// GetData is getData(): it converts the committed use-case model to the
// unified renderer contract without semantic re-parsing.
//
// The properties of the nodes, of the edges and of the layout data that
// rendering-util/types.ts does not have are in Extra, with their JavaScript
// values: actorType, business, jsonRows, noteTarget, noteTargetLabel and
// boundaryType of a node, sourceLabel, targetLabel, relationshipType and
// internal of an edge, and the fonts, diagramPadding and useMaxWidth of the
// layout data.
func (d *DB) GetData() *ru.LayoutData {
	globalConfig := d.ctx.Config
	// const config = { ...state.config, ...globalConfig.usecase }
	cfg := js.NewObj[any]()
	d.state.config.Each(func(k string, v any) { cfg.Set(k, v) })
	if usecase := js.AsObj(globalConfig.Get("usecase")); usecase != nil {
		usecase.Each(func(k string, v any) { cfg.Set(k, v) })
	}
	sanitize := func(value string) string { return d.ctx.SanitizeText(value) }
	state := d.state
	endpointLabel := func(id string) string {
		if actor := state.actors.Get(id); actor != nil {
			return sanitize(actor.Label)
		}
		if useCase := state.useCases.Get(id); useCase != nil {
			return sanitize(useCase.Label)
		}
		if json := state.jsonNodes.Get(id); json != nil {
			return sanitize(json.ID)
		}
		if note := state.notes.Get(id); note != nil {
			return sanitize(note.Label)
		}
		return sanitize(id)
	}
	nodes := []*ru.Node{}
	edges := []*ru.Edge{}
	look := lookOf(globalConfig)

	// Two counters, because the two kinds of slot mean different things.
	//
	// boundaryColorIndex numbers the system boundaries from zero in declaration order, the
	// way flowDb numbers flowchart subgraphs. colorIndex numbers actors and use cases
	// together, and is consumed only by the opt-in usecase.colorScheme: 'rotate'. Notes and
	// JSON tables are deliberately not counted.
	colorIndex := 0.0
	boundaryColorIndex := 0.0

	newNode := func(id, label, labelType, shape string, isGroup bool, padding float64, cssClasses string, cssStyles, cssCompiledStyles []string) *ru.Node {
		node := ru.NewNode(id)
		node.Label, node.HasLabel = label, true
		node.LabelType = labelType
		node.Shape = shape
		node.IsGroup = isGroup
		node.Padding = padding
		node.Look = look
		node.CSSClasses, node.CSSClassesSet = cssClasses, true
		node.CSSStyles, node.CSSStylesSet = cssStyles, true
		node.CSSCompiledStyles = cssCompiledStyles
		return node
	}

	for _, actor := range state.actors.Values() {
		node := newNode(
			actor.ID,
			sanitize(actor.Label),
			actor.LabelType,
			actorShape(actor),
			false,
			10,
			classNames(append([]string{
				"default",
				"usecase-actor",
				"usecase-actor-" + actor.Type,
				orEmpty(actor.Business, "usecase-business"),
			}, actor.Classes...)...),
			slices.Clone(actor.Styles),
			d.getCompiledStyles(actor.Classes),
		)
		node.ColorIndex = colorIndex
		colorIndex++
		node.Set("actorType", actor.Type)
		node.Set("business", actor.Business)
		if actor.Icon != "" {
			node.Icon = actor.Icon
		}
		if actor.Stereotype != "" {
			node.Stereotype, node.HasStereotype = sanitize(actor.Stereotype), true
		}
		if actor.ParentID != "" {
			node.ParentID = actor.ParentID
		}
		nodes = append(nodes, node)
	}

	for _, useCase := range state.useCases.Values() {
		padding := 10.0
		if useCase.Shape == "ellipse" {
			padding = 20
		}
		node := newNode(
			useCase.ID,
			sanitize(useCase.Label),
			useCase.LabelType,
			useCaseShape(useCase),
			false,
			padding,
			classNames(append([]string{
				"default",
				"usecase-element",
				"usecase-" + useCase.Shape,
				orEmpty(useCase.Business, "usecase-business"),
			}, useCase.Classes...)...),
			slices.Clone(useCase.Styles),
			d.getCompiledStyles(useCase.Classes),
		)
		node.ColorIndex = colorIndex
		colorIndex++
		node.Set("business", useCase.Business)
		if useCase.Stereotype != "" {
			node.Stereotype, node.HasStereotype = sanitize(useCase.Stereotype), true
		}
		if useCase.ParentID != "" {
			node.ParentID = useCase.ParentID
		}
		nodes = append(nodes, node)
	}

	for _, note := range state.notes.Values() {
		node := newNode(
			note.ID,
			sanitize(note.Label),
			note.LabelType,
			"note",
			false,
			10,
			"default usecase-note",
			[]string{},
			d.getCompiledStyles(nil),
		)
		node.Set("noteTarget", note.Target)
		var noteTargetLabel string
		if actor := state.actors.Get(note.Target); actor != nil {
			noteTargetLabel = actor.Label
		} else if useCase := state.useCases.Get(note.Target); useCase != nil {
			noteTargetLabel = useCase.Label
		} else if json := state.jsonNodes.Get(note.Target); json != nil {
			noteTargetLabel = json.ID
		} else {
			noteTargetLabel = note.Target
		}
		node.Set("noteTargetLabel", sanitize(noteTargetLabel))
		nodes = append(nodes, node)
	}

	for _, json := range state.jsonNodes.Values() {
		node := newNode(
			json.ID,
			sanitize(json.ID),
			"text",
			"usecaseJsonTable",
			false,
			10,
			classNames(append([]string{"default", "usecase-json-table"}, json.Classes...)...),
			slices.Clone(json.Styles),
			d.getCompiledStyles(json.Classes),
		)
		node.Set("jsonRows", flattenJSONRows(json.Value, json.PropertyOrder, sanitize))
		nodes = append(nodes, node)
	}

	for _, boundary := range state.systemBoundaries.Values() {
		node := newNode(
			boundary.ID,
			sanitize(boundary.Label),
			boundary.LabelType,
			"usecaseSystemBoundary",
			true,
			20,
			classNames(append([]string{
				"default",
				"system-boundary",
				"system-boundary-" + boundary.Type,
			}, boundary.Classes...)...),
			slices.Clone(boundary.Styles),
			d.getCompiledStyles(boundary.Classes),
		)
		node.ColorIndex = boundaryColorIndex
		boundaryColorIndex++
		node.Set("boundaryType", boundary.Type)
		nodes = append(nodes, node)
	}

	for _, relationship := range state.relationships {
		visual := relationshipVisuals(relationship)
		edge := ru.NewEdge(relationship.ID)
		edge.Start = relationship.Source
		edge.End = relationship.Target
		edge.Source = relationship.Source
		edge.Target = relationship.Target
		edge.Set("sourceLabel", endpointLabel(relationship.Source))
		edge.Set("targetLabel", endpointLabel(relationship.Target))
		edge.Type = "edge"
		edge.Set("relationshipType", relationship.Type)
		edge.Set("internal", false)
		edge.ArrowTypeStart = visual.arrowTypeStart
		edge.ArrowTypeEnd = visual.arrowTypeEnd
		edge.Pattern = visual.pattern
		edge.LabelType = visual.labelType
		if visual.hasLabel {
			edge.Label, edge.HasLabel = sanitize(visual.label), true
		}
		edge.Labelpos = "c"
		edge.Classes = classNames(append(append([]string{
			"default",
			"relationship",
			"relationship-" + relationship.Type,
		}, relationship.Classes...), animationClasses(relationship)...)...)
		edge.Style = slices.Clone(relationship.Styles)
		edge.CSSCompiledStyles = d.getCompiledStyles(relationship.Classes)
		edge.Animate, edge.HasAnimate = relationship.Animate, true
		if relationship.Animation != "" {
			edge.Animation = relationship.Animation
		}
		edge.Look = look
		edge.Thickness = "normal"
		edge.Minlen = relationship.Minlen
		edge.IsUserDefinedID = relationship.ExplicitID
		edges = append(edges, edge)
	}

	for _, note := range state.notes.Values() {
		edge := ru.NewEdge(note.ID + "-edge")
		edge.Start = note.ID
		edge.End = note.Target
		edge.Source = note.ID
		edge.Target = note.Target
		edge.Type = "edge"
		edge.Set("relationshipType", "note")
		edge.Set("sourceLabel", endpointLabel(note.ID))
		edge.Set("targetLabel", endpointLabel(note.Target))
		edge.Set("internal", true)
		edge.Pattern = "dotted"
		edge.ArrowTypeStart = "none"
		edge.ArrowTypeEnd = "none"
		edge.Labelpos = "c"
		edge.Classes = "default relationship relationship-note"
		edge.Style = []string{}
		edge.CSSCompiledStyles = d.getCompiledStyles(nil)
		edge.Animate, edge.HasAnimate = false, true
		edge.Look = look
		edge.Thickness = "normal"
		edge.Minlen = 1
		edge.IsUserDefinedID = false
		edges = append(edges, edge)
	}

	wrappingWidth, wrappingWidthIsNumber := numberOf(cfg.Get("wrappingWidth"))
	minNodeWidth, minNodeWidthIsNumber := numberOf(cfg.Get("minNodeWidth"))
	for _, node := range nodes {
		// node.wrappingWidth ??= config.wrappingWidth
		node.WrappingWidth = wrappingWidth
		if !wrappingWidthIsNumber {
			node.Set("wrappingWidth", cfg.Get("wrappingWidth"))
		}
		// Actors are glyphs with a caption, not boxes -- a minimum width would only
		// pad their layout footprint.
		if !node.IsGroup && !strings.HasPrefix(node.Shape, "usecaseActor") {
			// node.minWidth ??= config.minNodeWidth
			node.MinWidth = minNodeWidth
			if !minNodeWidthIsNumber {
				node.Set("minWidth", cfg.Get("minNodeWidth"))
			}
		}
		// Several association edges can end on one side of a use case, each with its
		// own marker; spread their attachment points so the markers stay apart.
		if node.Shape == "usecaseEllipse" || node.Shape == "usecaseBusiness" {
			node.SpreadPorts = true
		}
	}

	data := &ru.LayoutData{Extra: map[string]any{}}
	data.Nodes = nodes
	data.Edges = edges
	data.Config = globalConfig
	data.Type = "usecase"
	data.LayoutAlgorithm = "dagre"
	data.Direction = d.GetDirection()
	var isNumber bool
	if data.NodeSpacing, isNumber = numberOf(cfg.Get("nodeSpacing")); !isNumber {
		data.Extra["nodeSpacing"] = cfg.Get("nodeSpacing")
	}
	if data.RankSpacing, isNumber = numberOf(cfg.Get("rankSpacing")); !isNumber {
		data.Extra["rankSpacing"] = cfg.Get("rankSpacing")
	}
	data.Extra["actorFontSize"] = cfg.Get("actorFontSize")
	data.Extra["actorFontFamily"] = cfg.Get("actorFontFamily")
	data.Extra["actorFontWeight"] = cfg.Get("actorFontWeight")
	data.Extra["usecaseFontSize"] = cfg.Get("usecaseFontSize")
	data.Extra["usecaseFontFamily"] = cfg.Get("usecaseFontFamily")
	data.Extra["usecaseFontWeight"] = cfg.Get("usecaseFontWeight")
	data.Extra["diagramPadding"] = cfg.Get("diagramPadding")
	data.Extra["useMaxWidth"] = cfg.Get("useMaxWidth")
	data.Markers = []string{"point", "circle", "cross", "extension"}
	return data
}

// lookOf is globalConfig.look as the look of a node: "" for undefined.
func lookOf(globalConfig *js.Obj[any]) string {
	look := globalConfig.Get("look")
	if look == nil {
		return ""
	}
	return js.ToString(look)
}
