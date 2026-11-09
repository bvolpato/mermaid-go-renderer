// Ported from mermaid 12.2.0, diagrams/usecase/parser/usecaseModelBuilder.ts
// (MIT).
//
// The builder of upstream also keeps the statements of the source and
// builds the graph AST (usecaseAst.ts) that db.getAST() returns. The
// renderer does not read the AST, and building it cannot fail, so this port
// leaves it out.

package usecase

import (
	"errors"
	"slices"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// span is Span: the start offset and the end offset of a source range, in
// UTF-16 code units.
type span [2]int

// draftLocation is DraftLocation.
type draftLocation struct {
	span   span
	line   int
	column int
}

// draftLabel is DraftLabel. typ is the label type: text or markdown.
type draftLabel struct {
	text string
	typ  string
	span span
}

// draftMetadataProperty is DraftMetadataProperty. value is a string or a
// bool.
type draftMetadataProperty struct {
	key      string
	value    any
	location draftLocation
}

// draftMetadata is DraftMetadata.
type draftMetadata struct {
	properties []draftMetadataProperty
}

// draftElement is DraftElement.
type draftElement struct {
	id string
	// kind is actor or usecase.
	kind      string
	label     draftLabel
	location  draftLocation
	generated bool
	// parentID and parentLocation are set when hasParent is true: the
	// element is inside a system boundary. The id of a boundary can be the
	// empty string, which upstream tells from undefined.
	parentID       string
	hasParent      bool
	parentLocation draftLocation
	// shape is ellipse or rect, or "" for undefined.
	shape    string
	metadata *draftMetadata
	// stereotype is "" for undefined: the text of a stereotype is not empty.
	stereotype string
	classes    []string
}

// draftBoundary is DraftBoundary.
type draftBoundary struct {
	id        string
	label     draftLabel
	location  draftLocation
	generated bool
	classes   []string
}

// draftJSON is DraftJson.
type draftJSON struct {
	id            string
	value         *js.Obj[any]
	propertyOrder *js.Obj[[]string]
	location      draftLocation
	classes       []string
}

// draftEndpoint is DraftEndpoint.
type draftEndpoint struct {
	id                 string
	label              draftLabel
	location           draftLocation
	generated          bool
	declaration        bool
	classesOnReference bool
}

// draftRelationship is DraftRelationship.
type draftRelationship struct {
	source   draftEndpoint
	target   draftEndpoint
	location draftLocation
	// explicitID is "" for undefined: an explicit id is an identifier.
	explicitID         string
	explicitIDLocation draftLocation
	// typ is association, include, extend or generalization.
	typ       string
	arrowType int
	label     *draftLabel
	minlen    float64
}

// draftNote is DraftNote.
type draftNote struct {
	target         string
	targetLocation draftLocation
	label          draftLabel
}

type draftMetadataAssignment struct {
	target         string
	targetLocation draftLocation
	metadata       *draftMetadata
}

type classTarget struct {
	id       string
	location draftLocation
}

type draftClassAssignment struct {
	targets []classTarget
	classes []string
}

type draftStyleAssignment struct {
	target   string
	location draftLocation
	styles   []string
}

type classDefinition struct {
	ids    []string
	styles []string
}

// symbolOrigin is SymbolOrigin. kind is actor, usecase, boundary, json or
// edge.
type symbolOrigin struct {
	kind      string
	location  draftLocation
	generated bool
	// label is the source label a generated ID was derived from. hasLabel is
	// false when the ID was written explicitly.
	label    string
	hasLabel bool
}

// elementState is ElementState.
type elementState struct {
	kind      string
	id        string
	label     draftLabel
	location  draftLocation
	generated bool
	// parentID is defined when hasParent is true.
	parentID  string
	hasParent bool
	// parentLocation is nil for undefined.
	parentLocation *draftLocation
	// shape is "" for undefined.
	shape string
	// stereotype is "" for undefined.
	stereotype         string
	stereotypeLocation *draftLocation
	classes            []string
	// actorType is normal, hollow or awesome, or "" for undefined.
	actorType string
	// icon is defined when hasIcon is true.
	icon    string
	hasIcon bool
	// business is defined when hasBusiness is true.
	business    bool
	hasBusiness bool
}

// boundaryState is BoundaryState.
type boundaryState struct {
	id        string
	label     draftLabel
	location  draftLocation
	generated bool
	// typ is rect or package, or "" for undefined.
	typ     string
	classes []string
	styles  []string
	members []string
}

// edgeState is EdgeState.
type edgeState struct {
	draft        *draftRelationship
	relationship *Relationship
}

func locationText(location draftLocation) string {
	return "line " + strconv.Itoa(location.line) + ", column " + strconv.Itoa(location.column) +
		" [" + strconv.Itoa(location.span[0]) + "," + strconv.Itoa(location.span[1]) + ")"
}

// optLabel is the optional label of labelSuffix and generatedFrom.
type optLabel struct {
	text string
	ok   bool
}

// labelSuffix names the source label a generated ID was derived from, so
// that a collision between two different labels that normalize to the same
// ID points at both originals.
func labelSuffix(label optLabel) string {
	if !label.ok {
		return ""
	}
	return ` (label "` + label.text + `")`
}

// generatedFrom is generatedFrom(origin).
func generatedFrom(generated bool, label string, hasLabel bool) optLabel {
	if generated {
		return optLabel{text: label, ok: hasLabel}
	}
	return optLabel{}
}

func pushUnique(target *[]string, values []string) {
	for _, value := range values {
		if !slices.Contains(*target, value) {
			*target = append(*target, value)
		}
	}
}

// modelBuilder is UsecaseModelBuilder: it collects detached CST drafts and
// publishes a fully resolved model.
type modelBuilder struct {
	db                  *DB
	model               *fields
	elements            []*draftElement
	boundaries          []*draftBoundary
	jsonDrafts          []*draftJSON
	relationshipDrafts  []*draftRelationship
	noteDrafts          []*draftNote
	metadataAssignments []*draftMetadataAssignment
	classAssignments    []*draftClassAssignment
	styleAssignments    []*draftStyleAssignment
	classDefinitions    []*classDefinition
	directions          []string
}

func newModelBuilder(db *DB) *modelBuilder {
	return &modelBuilder{db: db, model: db.createModel()}
}

func (b *modelBuilder) addElement(value *draftElement)       { b.elements = append(b.elements, value) }
func (b *modelBuilder) addBoundary(value *draftBoundary)     { b.boundaries = append(b.boundaries, value) }
func (b *modelBuilder) addJSON(value *draftJSON)             { b.jsonDrafts = append(b.jsonDrafts, value) }
func (b *modelBuilder) addNote(value *draftNote)             { b.noteDrafts = append(b.noteDrafts, value) }
func (b *modelBuilder) setAccTitle(title string)             { b.model.accTitle = title }
func (b *modelBuilder) setAccDescription(description string) { b.model.accDescription = description }

func (b *modelBuilder) addRelationship(value *draftRelationship) {
	b.relationshipDrafts = append(b.relationshipDrafts, value)
}

func (b *modelBuilder) addMetadataAssignment(target string, targetLocation draftLocation, metadata *draftMetadata) {
	b.metadataAssignments = append(b.metadataAssignments, &draftMetadataAssignment{target, targetLocation, metadata})
}

func (b *modelBuilder) addClassDef(ids []string, styles []string) {
	b.classDefinitions = append(b.classDefinitions, &classDefinition{ids, styles})
}

func (b *modelBuilder) addClassAssignment(targets []classTarget, classes []string) {
	b.classAssignments = append(b.classAssignments, &draftClassAssignment{targets, classes})
}

func (b *modelBuilder) addStyleAssignment(target string, location draftLocation, styles []string) {
	b.styleAssignments = append(b.styleAssignments, &draftStyleAssignment{target, location, styles})
}

func (b *modelBuilder) setDirection(direction string) {
	if direction == "TD" {
		direction = "TB"
	}
	b.directions = append(b.directions, direction)
}

// declaration is one entry of the declarations list of finalize.
type declaration struct {
	offset   int
	element  *draftElement
	boundary *draftBoundary
	json     *draftJSON
	edge     *draftRelationship
}

// finalize resolves the drafts and commits the model to the db.
func (b *modelBuilder) finalize() error {
	symbols := js.NewMap[string, *symbolOrigin]()
	elements := js.NewMap[string, *elementState]()
	boundaries := js.NewMap[string, *boundaryState]()
	first := js.NewMap[string, int]()
	for _, relation := range b.relationshipDrafts {
		b.recordFirst(first, relation.source.id, relation.source.location.span[0])
		b.recordFirst(first, relation.target.id, relation.target.location.span[0])
	}
	for _, draft := range b.elements {
		b.recordFirst(first, draft.id, draft.location.span[0])
	}
	for _, draft := range b.boundaries {
		b.recordFirst(first, draft.id, draft.location.span[0])
	}
	for _, draft := range b.jsonDrafts {
		b.recordFirst(first, draft.id, draft.location.span[0])
	}
	var declarations []declaration
	for _, value := range b.elements {
		declarations = append(declarations, declaration{offset: value.location.span[0], element: value})
	}
	for _, value := range b.boundaries {
		declarations = append(declarations, declaration{offset: value.location.span[0], boundary: value})
	}
	for _, value := range b.jsonDrafts {
		declarations = append(declarations, declaration{offset: value.location.span[0], json: value})
	}
	for _, value := range b.relationshipDrafts {
		if value.explicitID != "" {
			declarations = append(declarations, declaration{offset: value.explicitIDLocation.span[0], edge: value})
		}
	}
	js.SortFunc(declarations, func(a, b declaration) float64 { return float64(a.offset - b.offset) })
	for _, item := range declarations {
		var err error
		switch {
		case item.element != nil:
			err = b.collectElement(item.element, symbols, elements)
		case item.boundary != nil:
			err = b.collectBoundary(item.boundary, symbols, boundaries)
		case item.json != nil:
			err = b.registerUnique(symbols, item.json.id, "json", item.json.location, false)
		default:
			err = b.registerUnique(symbols, item.edge.explicitID, "edge", item.edge.explicitIDLocation, false)
		}
		if err != nil {
			return err
		}
	}
	b.materializeElements(elements, first)
	b.materializeBoundaries(boundaries, first)
	b.materializeJSON(first)
	edges, err := b.materializeRelationships(symbols, elements, first)
	if err != nil {
		return err
	}
	b.reorderElements(elements, first)
	if err := b.applyMetadataAssignments(symbols, elements, boundaries, edges); err != nil {
		return err
	}
	if err := b.validateAndRefreshElements(elements); err != nil {
		return err
	}
	if err := b.refreshBoundaries(boundaries, elements); err != nil {
		return err
	}
	if err := b.materializeNotes(symbols); err != nil {
		return err
	}
	b.applyClassDefinitions()
	if err := b.applyClassesAndStyles(symbols, edges); err != nil {
		return err
	}
	if len(b.directions) > 0 {
		b.model.direction = b.directions[len(b.directions)-1]
	}
	b.model.symbols = js.NewMap[string, string]()
	symbols.Each(func(id string, origin *symbolOrigin) { b.model.symbols.Set(id, origin.kind) })
	b.db.commit(b.model)
	return nil
}

func (b *modelBuilder) collectElement(
	draft *draftElement,
	symbols *js.Map[string, *symbolOrigin],
	states *js.Map[string, *elementState],
) error {
	origin := symbols.Get(draft.id)
	draftLabel := generatedFrom(draft.generated, draft.label.text, true)
	if origin != nil && origin.kind != draft.kind {
		return b.conflict(
			"ID '"+draft.id+"' is declared as both "+origin.kind+" and "+draft.kind,
			draft.location,
			origin.location,
			draftLabel,
			generatedFrom(origin.generated, origin.label, origin.hasLabel),
		)
	}
	if origin != nil && (origin.generated || draft.generated) {
		return b.conflict(
			"Generated ID '"+draft.id+"' collides with another declaration",
			draft.location,
			origin.location,
			draftLabel,
			generatedFrom(origin.generated, origin.label, origin.hasLabel),
		)
	}
	existing := states.Get(draft.id)
	if existing == nil {
		state := &elementState{
			kind:      draft.kind,
			id:        draft.id,
			label:     draft.label,
			location:  draft.location,
			generated: draft.generated,
			classes:   slices.Clone(draft.classes),
		}
		if draft.parentID != "" {
			state.parentID, state.hasParent = draft.parentID, true
			if draft.hasParent {
				parentLocation := draft.parentLocation
				state.parentLocation = &parentLocation
			}
		}
		if draft.shape != "" {
			state.shape = draft.shape
		}
		if draft.stereotype != "" {
			state.stereotype = draft.stereotype
			stereotypeLocation := draft.location
			state.stereotypeLocation = &stereotypeLocation
		}
		if err := b.applyDeclarationMetadata(state, draft.metadata); err != nil {
			return err
		}
		states.Set(draft.id, state)
		symbols.Set(draft.id, &symbolOrigin{
			kind:      draft.kind,
			location:  draft.location,
			generated: draft.generated,
			label:     draft.label.text,
			hasLabel:  true,
		})
		return nil
	}
	if existing.label.text != draft.label.text || existing.label.typ != draft.label.typ {
		return b.conflict("ID '"+draft.id+"' has conflicting labels", draft.location, existing.location, optLabel{}, optLabel{})
	}
	if draft.shape != "" && existing.shape != "" && draft.shape != existing.shape {
		return b.conflict(
			"Use case '"+draft.id+"' has conflicting shapes",
			draft.location,
			existing.location,
			optLabel{}, optLabel{},
		)
	}
	if draft.parentID != "" && existing.parentID != "" && draft.parentID != existing.parentID {
		current := draft.location
		if draft.hasParent {
			current = draft.parentLocation
		}
		previous := existing.location
		if existing.parentLocation != nil {
			previous = *existing.parentLocation
		}
		return b.conflict(
			"Element '"+draft.id+"' belongs to more than one system boundary",
			current,
			previous,
			optLabel{}, optLabel{},
		)
	}
	if draft.stereotype != "" && existing.stereotype != "" && draft.stereotype != existing.stereotype {
		previous := existing.location
		if existing.stereotypeLocation != nil {
			previous = *existing.stereotypeLocation
		}
		return b.conflict(
			"Element '"+draft.id+"' has conflicting stereotypes",
			draft.location,
			previous,
			optLabel{}, optLabel{},
		)
	}
	// existing.shape ??= draft.shape
	if existing.shape == "" {
		existing.shape = draft.shape
	}
	// existing.parentId ??= draft.parentId
	if !existing.hasParent && draft.hasParent {
		existing.parentID, existing.hasParent = draft.parentID, true
	}
	// existing.parentLocation ??= draft.parentLocation
	if existing.parentLocation == nil && draft.hasParent {
		parentLocation := draft.parentLocation
		existing.parentLocation = &parentLocation
	}
	// existing.stereotype ??= draft.stereotype
	if existing.stereotype == "" {
		existing.stereotype = draft.stereotype
	}
	// existing.stereotypeLocation ??= draft.stereotype ? draft.location : undefined
	if existing.stereotypeLocation == nil && draft.stereotype != "" {
		stereotypeLocation := draft.location
		existing.stereotypeLocation = &stereotypeLocation
	}
	pushUnique(&existing.classes, draft.classes)
	return b.applyDeclarationMetadata(existing, draft.metadata)
}

func (b *modelBuilder) collectBoundary(
	draft *draftBoundary,
	symbols *js.Map[string, *symbolOrigin],
	states *js.Map[string, *boundaryState],
) error {
	origin := symbols.Get(draft.id)
	draftLabel := generatedFrom(draft.generated, draft.label.text, true)
	if origin != nil && origin.kind != "boundary" {
		return b.conflict(
			"ID '"+draft.id+"' is declared as both "+origin.kind+" and boundary",
			draft.location,
			origin.location,
			draftLabel,
			generatedFrom(origin.generated, origin.label, origin.hasLabel),
		)
	}
	if origin != nil && (origin.generated || draft.generated) {
		return b.conflict(
			"Generated ID '"+draft.id+"' collides with another declaration",
			draft.location,
			origin.location,
			draftLabel,
			generatedFrom(origin.generated, origin.label, origin.hasLabel),
		)
	}
	existing := states.Get(draft.id)
	if existing != nil {
		if existing.label.text != draft.label.text || existing.label.typ != draft.label.typ {
			return b.conflict(
				"Boundary '"+draft.id+"' has conflicting titles",
				draft.location,
				existing.location,
				optLabel{}, optLabel{},
			)
		}
		pushUnique(&existing.classes, draft.classes)
		return nil
	}
	states.Set(draft.id, &boundaryState{
		id:        draft.id,
		label:     draft.label,
		location:  draft.location,
		generated: draft.generated,
		classes:   slices.Clone(draft.classes),
		styles:    []string{},
		members:   []string{},
	})
	symbols.Set(draft.id, &symbolOrigin{
		kind:      "boundary",
		location:  draft.location,
		generated: draft.generated,
		label:     draft.label.text,
		hasLabel:  true,
	})
	return nil
}

// sortedByFirst returns the values of a map sorted by the first offset of
// their ids: [...states.values()].sort((a, b) => (first.get(a.id) ?? 0) - (first.get(b.id) ?? 0)).
func sortedByFirst[T any](values []T, id func(T) string, first *js.Map[string, int]) []T {
	sorted := slices.Clone(values)
	js.SortFunc(sorted, func(a, b T) float64 {
		// first.get(id) ?? 0: a missing id counts as 0.
		return float64(first.Get(id(a)) - first.Get(id(b)))
	})
	return sorted
}

func (b *modelBuilder) materializeElements(states *js.Map[string, *elementState], first *js.Map[string, int]) {
	for _, state := range sortedByFirst(states.Values(), func(s *elementState) string { return s.id }, first) {
		b.setElementModel(state)
	}
}

func (b *modelBuilder) materializeBoundaries(states *js.Map[string, *boundaryState], first *js.Map[string, int]) {
	for _, state := range sortedByFirst(states.Values(), func(s *boundaryState) string { return s.id }, first) {
		typ := state.typ
		if typ == "" {
			typ = "rect"
		}
		b.model.systemBoundaries.Set(state.id, &SystemBoundary{
			ID:        state.id,
			Label:     state.label.text,
			LabelType: state.label.typ,
			Type:      typ,
			Members:   []string{},
			Classes:   slices.Clone(state.classes),
			Styles:    slices.Clone(state.styles),
		})
	}
}

func (b *modelBuilder) materializeJSON(first *js.Map[string, int]) {
	for _, draft := range sortedByFirst(b.jsonDrafts, func(d *draftJSON) string { return d.id }, first) {
		b.model.jsonNodes.Set(draft.id, &UsecaseJsonNode{
			ID:            draft.id,
			Value:         draft.value,
			PropertyOrder: draft.propertyOrder,
			Classes:       slices.Clone(draft.classes),
			Styles:        []string{},
		})
	}
}

func (b *modelBuilder) materializeRelationships(
	symbols *js.Map[string, *symbolOrigin],
	states *js.Map[string, *elementState],
	first *js.Map[string, int],
) (*js.Map[string, *edgeState], error) {
	edges := js.NewMap[string, *edgeState]()
	anonymous := 0
	for _, draft := range b.relationshipDrafts {
		sourceKind, err := b.resolveEndpoint(&draft.source, symbols, states, first)
		if err != nil {
			return nil, err
		}
		targetKind, err := b.resolveEndpoint(&draft.target, symbols, states, first)
		if err != nil {
			return nil, err
		}
		if err := b.validateRelationship(draft, sourceKind, targetKind, symbols); err != nil {
			return nil, err
		}
		id := draft.explicitID
		if id == "" {
			id = "edge-" + strconv.Itoa(anonymous)
			anonymous++
		}
		relationship := &Relationship{
			ID:         id,
			ExplicitID: draft.explicitID != "",
			Source:     draft.source.id,
			Target:     draft.target.id,
			Type:       draft.typ,
			ArrowType:  draft.arrowType,
			Minlen:     draft.minlen,
			Classes:    []string{},
			Styles:     []string{},
			Animate:    false,
		}
		if draft.label != nil {
			relationship.Label, relationship.LabelType, relationship.HasLabel = draft.label.text, draft.label.typ, true
		}
		b.model.relationships = append(b.model.relationships, relationship)
		edges.Set(id, &edgeState{draft: draft, relationship: relationship})
	}
	b.model.relationshipCounter = anonymous
	return edges, nil
}

func (b *modelBuilder) resolveEndpoint(
	endpoint *draftEndpoint,
	symbols *js.Map[string, *symbolOrigin],
	states *js.Map[string, *elementState],
	first *js.Map[string, int],
) (string, error) {
	if endpoint.classesOnReference && !endpoint.declaration {
		return "", errors.New(
			"Relationship endpoint '" + endpoint.id + "' uses ::: without declaring the node at " + locationText(endpoint.location),
		)
	}
	origin := symbols.Get(endpoint.id)
	if origin != nil {
		return origin.kind, nil
	}
	states.Set(endpoint.id, &elementState{
		kind:      "usecase",
		id:        endpoint.id,
		label:     endpoint.label,
		location:  endpoint.location,
		generated: endpoint.generated,
		shape:     "ellipse",
		classes:   []string{},
	})
	symbols.Set(endpoint.id, &symbolOrigin{
		kind:      "usecase",
		location:  endpoint.location,
		generated: endpoint.generated,
		label:     endpoint.label.text,
		hasLabel:  true,
	})
	b.recordFirst(first, endpoint.id, endpoint.location.span[0])
	return "usecase", nil
}

func (b *modelBuilder) reorderElements(states *js.Map[string, *elementState], first *js.Map[string, int]) {
	actors := js.NewMap[string, *Actor]()
	b.model.actors.Each(func(id string, actor *Actor) { actors.Set(id, actor) })
	useCases := js.NewMap[string, *UseCase]()
	b.model.useCases.Each(func(id string, useCase *UseCase) { useCases.Set(id, useCase) })
	b.model.actors.Clear()
	b.model.useCases.Clear()
	for _, state := range sortedByFirst(states.Values(), func(s *elementState) string { return s.id }, first) {
		if !actors.Has(state.id) && !useCases.Has(state.id) {
			b.setElementModel(state)
		}
		actor := actors.Get(state.id)
		if actor == nil {
			actor = b.model.actors.Get(state.id)
		}
		useCase := useCases.Get(state.id)
		if useCase == nil {
			useCase = b.model.useCases.Get(state.id)
		}
		if actor != nil {
			b.model.actors.Set(state.id, actor)
		} else if useCase != nil {
			b.model.useCases.Set(state.id, useCase)
		}
	}
}

func (b *modelBuilder) applyMetadataAssignments(
	symbols *js.Map[string, *symbolOrigin],
	elements *js.Map[string, *elementState],
	boundaries *js.Map[string, *boundaryState],
	edges *js.Map[string, *edgeState],
) error {
	for _, assignment := range b.metadataAssignments {
		origin := symbols.Get(assignment.target)
		if origin == nil {
			inferred := b.inferMetadataKind(assignment.metadata)
			implies := ""
			if inferred != "" {
				implies = " (metadata implies " + inferred + ")"
			}
			return errors.New(
				"Metadata target '" + assignment.target + "' is unresolved" + implies + " at " + locationText(assignment.targetLocation),
			)
		}
		if origin.kind == "actor" || origin.kind == "usecase" {
			if err := b.applyStandaloneElementMetadata(elements.Get(assignment.target), assignment.metadata); err != nil {
				return err
			}
		} else if origin.kind == "boundary" {
			boundary := boundaries.Get(assignment.target)
			for i := range assignment.metadata.properties {
				property := &assignment.metadata.properties[i]
				if property.key != "type" ||
					(property.value != "rect" && property.value != "package") {
					return b.invalidMetadata(assignment.target, origin.kind, property)
				}
				boundary.typ = property.value.(string)
			}
		} else if origin.kind == "edge" {
			edge := edges.Get(assignment.target)
			if edge == nil {
				return errors.New(
					"Metadata target '" + assignment.target + "' is not an explicit edge at " + locationText(assignment.targetLocation),
				)
			}
			for i := range assignment.metadata.properties {
				property := &assignment.metadata.properties[i]
				if value, isBool := property.value.(bool); property.key == "animate" && isBool {
					edge.relationship.Animate = value
				} else if property.key == "animation" &&
					(property.value == "fast" || property.value == "slow") {
					edge.relationship.Animation = property.value.(string)
					edge.relationship.Animate = true
				} else {
					return b.invalidMetadata(assignment.target, origin.kind, property)
				}
			}
		} else {
			for i := range assignment.metadata.properties {
				return b.invalidMetadata(assignment.target, origin.kind, &assignment.metadata.properties[i])
			}
		}
	}
	edges.Each(func(_ string, edge *edgeState) {
		if edge.relationship.Animation != "" {
			edge.relationship.Animate = true
		}
	})
	return nil
}

func (b *modelBuilder) applyStandaloneElementMetadata(state *elementState, metadata *draftMetadata) error {
	for i := range metadata.properties {
		property := &metadata.properties[i]
		if state.kind == "actor" {
			if err := b.applyActorProperty(state, property, true); err != nil {
				return err
			}
		} else if value, isBool := property.value.(bool); property.key == "business" && isBool {
			state.business, state.hasBusiness = value, true
		} else {
			return b.invalidMetadata(state.id, state.kind, property)
		}
	}
	return nil
}

func (b *modelBuilder) applyDeclarationMetadata(state *elementState, metadata *draftMetadata) error {
	if metadata == nil {
		return nil
	}
	for i := range metadata.properties {
		property := &metadata.properties[i]
		if state.kind == "actor" {
			if err := b.applyActorProperty(state, property, false); err != nil {
				return err
			}
		} else if value, isBool := property.value.(bool); property.key == "business" && isBool {
			if state.hasBusiness && state.business != value {
				return b.conflict(
					"Use case '"+state.id+"' has conflicting business metadata",
					property.location,
					state.location,
					optLabel{}, optLabel{},
				)
			}
			state.business, state.hasBusiness = value, true
		} else {
			return b.invalidMetadata(state.id, state.kind, property)
		}
	}
	return nil
}

func (b *modelBuilder) applyActorProperty(state *elementState, property *draftMetadataProperty, replace bool) error {
	stringValue, isString := property.value.(string)
	boolValue, isBool := property.value.(bool)
	if property.key == "type" &&
		isString && (stringValue == "normal" || stringValue == "hollow" || stringValue == "awesome") {
		if !replace && state.actorType != "" && state.actorType != stringValue {
			return b.conflict(
				"Actor '"+state.id+"' has conflicting type metadata",
				property.location,
				state.location,
				optLabel{}, optLabel{},
			)
		}
		state.actorType = stringValue
	} else if property.key == "icon" && isString {
		if !replace && state.hasIcon && state.icon != stringValue {
			return b.conflict(
				"Actor '"+state.id+"' has conflicting icon metadata",
				property.location,
				state.location,
				optLabel{}, optLabel{},
			)
		}
		state.icon, state.hasIcon = stringValue, true
	} else if property.key == "business" && isBool {
		if !replace && state.hasBusiness && state.business != boolValue {
			return b.conflict(
				"Actor '"+state.id+"' has conflicting business metadata",
				property.location,
				state.location,
				optLabel{}, optLabel{},
			)
		}
		state.business, state.hasBusiness = boolValue, true
	} else {
		return b.invalidMetadata(state.id, state.kind, property)
	}
	return nil
}

// actorTypeOf is `state.icon ? 'icon' : (state.actorType ?? 'normal')`.
func actorTypeOf(state *elementState) string {
	if state.icon != "" {
		return "icon"
	}
	if state.actorType != "" {
		return state.actorType
	}
	return "normal"
}

func (b *modelBuilder) validateAndRefreshElements(states *js.Map[string, *elementState]) error {
	for _, state := range states.Values() {
		typ := actorTypeOf(state)
		if state.kind == "actor" {
			if state.icon != "" && state.actorType != "" && state.actorType != "normal" {
				return errors.New(
					"Actor '" + state.id + "' cannot combine icon with type '" + state.actorType + "' at " + locationText(state.location),
				)
			}
			if state.business && (typ == "icon" || typ == "awesome") {
				return errors.New(
					"Business actor '" + state.id + "' must use normal or hollow geometry at " + locationText(state.location),
				)
			}
		} else if (state.shape == "rect") && state.business {
			return errors.New(
				"Rectangular use case '" + state.id + "' cannot be a business use case at " + locationText(state.location),
			)
		}
		b.setElementModel(state)
	}
	return nil
}

func (b *modelBuilder) refreshBoundaries(
	boundaries *js.Map[string, *boundaryState],
	elements *js.Map[string, *elementState],
) error {
	for _, boundary := range boundaries.Values() {
		boundary.members = boundary.members[:0]
	}
	sorted := slices.Clone(b.elements)
	js.SortFunc(sorted, func(a, b *draftElement) float64 { return float64(a.location.span[0] - b.location.span[0]) })
	for _, draft := range sorted {
		if draft.parentID == "" {
			continue
		}
		boundary := boundaries.Get(draft.parentID)
		if boundary == nil {
			// draft.parentLocation ?? draft.location: a draft with a parent id
			// has a parent location.
			return errors.New(
				"Parent boundary '" + draft.parentID + "' for '" + draft.id + "' is unresolved at " + locationText(draft.parentLocation),
			)
		}
		if !slices.Contains(boundary.members, draft.id) {
			boundary.members = append(boundary.members, draft.id)
		}
	}
	for _, state := range elements.Values() {
		if state.parentID != "" && !boundaries.Has(state.parentID) {
			location := state.location
			if state.parentLocation != nil {
				location = *state.parentLocation
			}
			return errors.New(
				"Parent boundary '" + state.parentID + "' for '" + state.id + "' is unresolved at " + locationText(location),
			)
		}
	}
	for _, state := range boundaries.Values() {
		model := b.model.systemBoundaries.Get(state.id)
		model.Type = state.typ
		if model.Type == "" {
			model.Type = "rect"
		}
		model.Members = slices.Clone(state.members)
		model.Classes = slices.Clone(state.classes)
		model.Styles = slices.Clone(state.styles)
	}
	return nil
}

func (b *modelBuilder) materializeNotes(symbols *js.Map[string, *symbolOrigin]) error {
	counter := 0
	for _, draft := range b.noteDrafts {
		origin := symbols.Get(draft.target)
		if origin == nil {
			return errors.New(
				"Note target '" + draft.target + "' is unresolved at " + locationText(draft.targetLocation),
			)
		}
		if origin.kind != "actor" && origin.kind != "usecase" {
			return b.conflict(
				"Note target '"+draft.target+"' must be an actor or use case, not "+origin.kind,
				draft.targetLocation,
				origin.location,
				optLabel{}, optLabel{},
			)
		}
		id := "note-" + strconv.Itoa(counter)
		counter++
		b.model.notes.Set(id, &UsecaseNote{
			ID:        id,
			Target:    draft.target,
			Label:     draft.label.text,
			LabelType: draft.label.typ,
		})
	}
	b.model.noteCounter = counter
	return nil
}

func (b *modelBuilder) applyClassDefinitions() {
	for _, definition := range b.classDefinitions {
		for _, id := range definition.ids {
			b.model.classDefs.Set(id, &ClassDef{ID: id, Styles: slices.Clone(definition.styles)})
		}
	}
}

func (b *modelBuilder) applyClassesAndStyles(
	symbols *js.Map[string, *symbolOrigin],
	edges *js.Map[string, *edgeState],
) error {
	for _, assignment := range b.classAssignments {
		for _, target := range assignment.targets {
			classes, _, err := b.getStylable(target.id, symbols, edges, target.location)
			if err != nil {
				return err
			}
			pushUnique(classes, assignment.classes)
		}
	}
	for _, assignment := range b.styleAssignments {
		_, styles, err := b.getStylable(assignment.target, symbols, edges, assignment.location)
		if err != nil {
			return err
		}
		*styles = append(*styles, assignment.styles...)
	}
	return nil
}

// getStylable returns the classes and the styles of the model object of an
// id.
func (b *modelBuilder) getStylable(
	id string,
	symbols *js.Map[string, *symbolOrigin],
	edges *js.Map[string, *edgeState],
	location draftLocation,
) (classes, styles *[]string, err error) {
	kind := ""
	if origin := symbols.Get(id); origin != nil {
		kind = origin.kind
	}
	switch kind {
	case "actor":
		if target := b.model.actors.Get(id); target != nil {
			return &target.Classes, &target.Styles, nil
		}
	case "usecase":
		if target := b.model.useCases.Get(id); target != nil {
			return &target.Classes, &target.Styles, nil
		}
	case "boundary":
		if target := b.model.systemBoundaries.Get(id); target != nil {
			return &target.Classes, &target.Styles, nil
		}
	case "json":
		if target := b.model.jsonNodes.Get(id); target != nil {
			return &target.Classes, &target.Styles, nil
		}
	case "edge":
		if edge := edges.Get(id); edge != nil {
			return &edge.relationship.Classes, &edge.relationship.Styles, nil
		}
	}
	return nil, nil, errors.New(
		"Class/style target '" + id + "' is unresolved or anonymous at " + locationText(location),
	)
}

func (b *modelBuilder) validateRelationship(
	draft *draftRelationship,
	sourceKind string,
	targetKind string,
	symbols *js.Map[string, *symbolOrigin],
) error {
	allowed := func(kind string) bool {
		return kind == "actor" || kind == "usecase" || kind == "json"
	}
	if !allowed(sourceKind) {
		return b.conflict(
			"Relationship source '"+draft.source.id+"' cannot be "+sourceKind,
			draft.source.location,
			symbols.Get(draft.source.id).location,
			optLabel{}, optLabel{},
		)
	}
	if !allowed(targetKind) {
		return b.conflict(
			"Relationship target '"+draft.target.id+"' cannot be "+targetKind,
			draft.target.location,
			symbols.Get(draft.target.id).location,
			optLabel{}, optLabel{},
		)
	}
	if (draft.typ == "include" || draft.typ == "extend") &&
		(sourceKind != "usecase" || targetKind != "usecase") {
		return errors.New(
			draft.typ + " relationship requires use-case endpoints at " + locationText(draft.location),
		)
	}
	if draft.typ == "generalization" &&
		((sourceKind != "actor" && sourceKind != "usecase") || sourceKind != targetKind) {
		return errors.New(
			"Generalization requires actor-to-actor or use-case-to-use-case endpoints at " + locationText(draft.location),
		)
	}
	if draft.typ == "association" &&
		(sourceKind == "json" || targetKind == "json") &&
		!(draft.arrowType == 0 || draft.arrowType == 1 || draft.arrowType == 2) {
		return errors.New(
			"JSON relationship '" + draft.source.id + "' to '" + draft.target.id +
				"' permits only point, reversed-point, or markerless solid association at " + locationText(draft.location),
		)
	}
	return nil
}

func (b *modelBuilder) setElementModel(state *elementState) {
	if state.kind == "actor" {
		typ := actorTypeOf(state)
		b.model.useCases.Delete(state.id)
		// styles: this.model.actors.get(state.id)?.styles ?? []
		styles := []string{}
		if previous := b.model.actors.Get(state.id); previous != nil {
			styles = previous.Styles
		}
		b.model.actors.Set(state.id, &Actor{
			ID:         state.id,
			Label:      state.label.text,
			LabelType:  state.label.typ,
			Type:       typ,
			Icon:       state.icon,
			Business:   state.business,
			Stereotype: state.stereotype,
			ParentID:   state.parentID,
			Classes:    slices.Clone(state.classes),
			Styles:     styles,
		})
	} else {
		b.model.actors.Delete(state.id)
		shape := state.shape
		if shape == "" {
			shape = "ellipse"
		}
		styles := []string{}
		if previous := b.model.useCases.Get(state.id); previous != nil {
			styles = previous.Styles
		}
		b.model.useCases.Set(state.id, &UseCase{
			ID:         state.id,
			Label:      state.label.text,
			LabelType:  state.label.typ,
			Shape:      shape,
			Business:   state.business,
			Stereotype: state.stereotype,
			ParentID:   state.parentID,
			Classes:    slices.Clone(state.classes),
			Styles:     styles,
		})
	}
}

// inferMetadataKind returns the one kind that the metadata implies, or ""
// for undefined.
func (b *modelBuilder) inferMetadataKind(metadata *draftMetadata) string {
	possible := js.NewSet("actor", "usecase", "boundary", "edge")
	for i := range metadata.properties {
		property := &metadata.properties[i]
		if property.key == "icon" {
			possible = js.NewSet("actor")
		} else if property.key == "animate" || property.key == "animation" {
			possible = js.NewSet("edge")
		} else if property.key == "type" {
			possible = js.NewSet[string]()
			if property.value == "rect" || property.value == "package" {
				possible.Add("boundary")
			} else if property.value == "normal" ||
				property.value == "hollow" ||
				property.value == "awesome" {
				possible.Add("actor")
			}
		} else if property.key == "business" {
			possible.Delete("boundary")
			possible.Delete("edge")
		} else {
			return ""
		}
	}
	if possible.Len() == 1 {
		return possible.Values()[0]
	}
	return ""
}

func (b *modelBuilder) invalidMetadata(id string, kind string, property *draftMetadataProperty) error {
	return errors.New(
		"Metadata property '" + property.key + "' is invalid for " + kind + " '" + id + "' at " + locationText(property.location),
	)
}

func (b *modelBuilder) registerUnique(
	symbols *js.Map[string, *symbolOrigin],
	id string,
	kind string,
	location draftLocation,
	generated bool,
) error {
	previous := symbols.Get(id)
	if previous != nil {
		return b.conflict(
			"ID '"+id+"' is declared more than once ("+previous.kind+" and "+kind+")",
			location,
			previous.location,
			optLabel{},
			generatedFrom(previous.generated, previous.label, previous.hasLabel),
		)
	}
	symbols.Set(id, &symbolOrigin{kind: kind, location: location, generated: generated})
	return nil
}

func (b *modelBuilder) recordFirst(m *js.Map[string, int], id string, offset int) {
	previous, ok := m.Lookup(id)
	if !ok || offset < previous {
		m.Set(id, offset)
	}
}

func (b *modelBuilder) conflict(
	message string,
	current draftLocation,
	previous draftLocation,
	currentLabel optLabel,
	previousLabel optLabel,
) error {
	return errors.New(
		message + " at " + locationText(current) + labelSuffix(currentLabel) +
			"; previous declaration at " + locationText(previous) + labelSuffix(previousLabel),
	)
}
