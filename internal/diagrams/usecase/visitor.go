// Ported from mermaid 12.2.0, diagrams/usecase/parser/usecase.visitor.ts
// (MIT).
//
// The visitor of upstream does two things in one pass: it hands drafts to
// the model builder, and it returns a GraphStatement for each line, from
// which the builder makes the graph AST of db.getAST(). The renderer does
// not read the AST and its construction cannot fail, so this port keeps the
// first part only. The order of the calls to the builder is the order of
// upstream.

package usecase

import (
	"errors"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parsedStereotype is ParsedStereotype.
type parsedStereotype struct {
	value string
}

// parsedItem is ParsedActorItem and ParsedEntity.
type parsedItem struct {
	id         string
	label      draftLabel
	location   draftLocation
	generated  bool
	metadata   *draftMetadata
	stereotype *parsedStereotype
	classes    []string
	// shape is ellipse or rect, or "" for undefined. Only an entity has it.
	shape string
	// explicitDeclaration is the property of ParsedEntity.
	explicitDeclaration bool
}

// parsedArrow is ParsedArrow.
type parsedArrow struct {
	typ       string
	arrowType int
	label     *draftLabel
	minlen    float64
}

// parsedRelationTail is ParsedRelationTail.
type parsedRelationTail struct {
	// explicitID is "" for undefined.
	explicitID         string
	explicitIDLocation draftLocation
	arrow              parsedArrow
	target             *parsedItem
}

// parentBoundary is the system boundary that the visitor is inside of.
type parentBoundary struct {
	id       string
	location draftLocation
}

// usecaseVisitor is UsecaseVisitor.
type usecaseVisitor struct {
	builder *modelBuilder
	// source is the diagram text as UTF-16 code units.
	source         []rune
	parentBoundary *parentBoundary
}

// build is build(cst, source): it fills the builder from the CST and
// commits the model.
func (v *usecaseVisitor) build(cst *cstNode) error {
	return v.start(cst)
}

func (v *usecaseVisitor) start(ctx *cstNode) error {
	for _, line := range v.nodes(ctx, "line") {
		if err := v.line(line); err != nil {
			return err
		}
	}
	return v.builder.finalize()
}

func (v *usecaseVisitor) line(ctx *cstNode) error {
	child, err := v.firstNode(ctx, "blankLine", "commentLine", "statement")
	if err != nil {
		return err
	}
	if child.name == "statement" {
		return v.statement(child)
	}
	// A blank line and a comment line add nothing to the model.
	return nil
}

func (v *usecaseVisitor) statement(ctx *cstNode) error {
	child, err := v.firstNode(
		ctx,
		"accTitleStatement",
		"accDescrStatement",
		"directionStatement",
		"actorStatement",
		"systemBoundaryStatement",
		"noteStatement",
		"jsonStatement",
		"classDefStatement",
		"classStatement",
		"styleStatement",
		"metadataAssignmentStatement",
		"entityStatement",
	)
	if err != nil {
		return err
	}
	switch child.name {
	case "accTitleStatement":
		v.accTitleStatement(child)
	case "accDescrStatement":
		v.accDescrStatement(child)
	case "directionStatement":
		v.directionStatement(child)
	case "actorStatement":
		v.actorStatement(child)
	case "systemBoundaryStatement":
		v.systemBoundaryStatement(child)
	case "noteStatement":
		v.noteStatement(child)
	case "jsonStatement":
		return v.jsonStatement(child)
	case "classDefStatement":
		v.classDefStatement(child)
	case "classStatement":
		v.classStatement(child)
	case "styleStatement":
		v.styleStatement(child)
	case "metadataAssignmentStatement":
		v.metadataAssignmentStatement(child)
	case "entityStatement":
		v.entityStatement(child)
	}
	return nil
}

func (v *usecaseVisitor) accTitleStatement(ctx *cstNode) {
	image := v.tokens(ctx, "ACC_TITLE_LINE")[0].image
	v.builder.setAccTitle(js.TrimSpace(image[strings.Index(image, ":")+1:]))
}

func (v *usecaseVisitor) accDescrStatement(ctx *cstNode) {
	lines := v.tokens(ctx, "ACC_DESCR_LINE")
	var description string
	if len(lines) > 0 {
		image := lines[0].image
		description = js.TrimSpace(image[strings.Index(image, ":")+1:])
	} else {
		image := v.tokens(ctx, "ACC_DESCR_BLOCK")[0].image
		// image.slice(image.indexOf('{') + 1, image.lastIndexOf('}')): the
		// image of the block has both braces.
		description = js.TrimSpace(image[strings.Index(image, "{")+1 : strings.LastIndex(image, "}")])
	}
	v.builder.setAccDescription(description)
}

func (v *usecaseVisitor) actorStatement(ctx *cstNode) {
	nodes := v.nodes(ctx, "actorItem")
	items := make([]*parsedItem, len(nodes))
	for i, node := range nodes {
		items[i] = v.actorItem(node)
	}
	relationNodes := v.nodes(ctx, "relationTail")
	for _, item := range items {
		v.builder.addElement(v.actorDraft(item))
	}
	if len(relationNodes) == 0 {
		return
	}
	relationNode := relationNodes[0]
	relation := v.relationTail(relationNode)
	if relation.target.explicitDeclaration {
		v.builder.addElement(v.entityDraft(relation.target))
	}
	draft := v.relationshipDraft(items[0], relation, v.nodeLocation(relationNode))
	v.builder.addRelationship(draft)
}

func (v *usecaseVisitor) actorItem(ctx *cstNode) *parsedItem {
	base := v.actorName(v.nodes(ctx, "actorName")[0])
	metadataNodes := v.nodes(ctx, "metadata")
	stereotypeNodes := v.nodes(ctx, "stereotype")
	classNodes := v.nodes(ctx, "classSuffix")
	classes := []string{}
	if len(classNodes) > 0 {
		classes = v.classSuffix(classNodes[0])
	}
	if len(metadataNodes) > 0 {
		base.metadata = v.metadata(metadataNodes[0])
	}
	if len(stereotypeNodes) > 0 {
		base.stereotype = v.stereotype(stereotypeNodes[0])
	}
	base.classes = classes
	return base
}

func (v *usecaseVisitor) actorName(ctx *cstNode) *parsedItem {
	identifiers := v.tokens(ctx, "IDENTIFIER")
	if len(identifiers) > 0 {
		identifier := identifiers[0]
		labelNodes := v.nodes(ctx, "nodeLabel")
		var label draftLabel
		if len(labelNodes) > 0 {
			label = v.nodeLabel(labelNodes[0])
		} else {
			label = v.tokenLabel(identifier)
		}
		return &parsedItem{
			id:        identifier.image,
			label:     label,
			location:  v.tokenLocation(identifier),
			generated: false,
			classes:   []string{},
		}
	}
	str := v.stringToken(ctx)
	label := v.tokenLabel(str)
	return &parsedItem{
		id:        v.generateID(label.text),
		label:     label,
		location:  v.tokenLocation(str),
		generated: true,
		classes:   []string{},
	}
}

// stringToken is `this.tokens(ctx, 'PLAIN_STRING')[0] ?? this.tokens(ctx, 'MARKDOWN_STRING')[0]`.
func (v *usecaseVisitor) stringToken(ctx *cstNode) *token {
	if toks := v.tokens(ctx, "PLAIN_STRING"); len(toks) > 0 {
		return toks[0]
	}
	return v.tokens(ctx, "MARKDOWN_STRING")[0]
}

func (v *usecaseVisitor) actorDeclarationOnly(ctx *cstNode) {
	nodes := v.nodes(ctx, "actorItem")
	items := make([]*parsedItem, len(nodes))
	for i, node := range nodes {
		items[i] = v.actorItem(node)
	}
	for _, item := range items {
		v.builder.addElement(v.actorDraft(item))
	}
}

func (v *usecaseVisitor) entityStatement(ctx *cstNode) {
	entityNodes := v.nodes(ctx, "entityName")
	source := v.entityName(entityNodes[0])
	relationNodes := v.nodes(ctx, "relationTail")
	if len(relationNodes) == 0 {
		source.explicitDeclaration = true
		if source.shape == "" {
			source.shape = "ellipse"
		}
		v.builder.addElement(v.entityDraft(source))
		return
	}
	relationNode := relationNodes[0]
	if source.explicitDeclaration {
		v.builder.addElement(v.entityDraft(source))
	}
	relation := v.relationTail(relationNode)
	if relation.target.explicitDeclaration {
		v.builder.addElement(v.entityDraft(relation.target))
	}
	draft := &draftRelationship{
		source:   v.endpoint(source, source.explicitDeclaration),
		target:   v.endpoint(relation.target, relation.target.explicitDeclaration),
		location: v.nodeLocation(relationNode),
	}
	if relation.explicitID != "" {
		draft.explicitID, draft.explicitIDLocation = relation.explicitID, relation.explicitIDLocation
	}
	draft.typ, draft.arrowType, draft.label, draft.minlen = relation.arrow.typ, relation.arrow.arrowType, relation.arrow.label, relation.arrow.minlen
	v.builder.addRelationship(draft)
}

func (v *usecaseVisitor) entityName(ctx *cstNode) *parsedItem {
	identifiers := v.tokens(ctx, "IDENTIFIER")
	labelNodes := v.nodes(ctx, "nodeLabel")
	metadataNodes := v.nodes(ctx, "useCaseMetadata")
	stereotypeNodes := v.nodes(ctx, "stereotype")
	classNodes := v.nodes(ctx, "classSuffix")
	classes := []string{}
	if len(classNodes) > 0 {
		classes = v.classSuffix(classNodes[0])
	}
	if len(identifiers) > 0 {
		identifier := identifiers[0]
		var label draftLabel
		shape := ""
		if len(labelNodes) > 0 {
			label = v.nodeLabel(labelNodes[0])
			if len(v.tokens(ctx, "LBRACKET")) > 0 {
				shape = "rect"
			} else {
				shape = "ellipse"
			}
		} else {
			label = v.tokenLabel(identifier)
		}
		item := &parsedItem{
			id:        identifier.image,
			label:     label,
			location:  v.tokenLocation(identifier),
			generated: false,
			shape:     shape,
		}
		if len(metadataNodes) > 0 {
			item.metadata = v.useCaseMetadata(metadataNodes[0])
		}
		if len(stereotypeNodes) > 0 {
			item.stereotype = v.stereotype(stereotypeNodes[0])
		}
		item.classes = classes
		item.explicitDeclaration = shape != "" || len(metadataNodes) > 0 || len(stereotypeNodes) > 0
		return item
	}
	str := v.stringToken(ctx)
	label := v.tokenLabel(str)
	item := &parsedItem{
		id:        v.generateID(label.text),
		label:     label,
		location:  v.tokenLocation(str),
		generated: true,
	}
	if len(metadataNodes) > 0 {
		item.metadata = v.useCaseMetadata(metadataNodes[0])
	}
	if len(stereotypeNodes) > 0 {
		item.stereotype = v.stereotype(stereotypeNodes[0])
	}
	item.classes = classes
	item.explicitDeclaration = len(metadataNodes) > 0 || len(stereotypeNodes) > 0
	return item
}

func (v *usecaseVisitor) nodeLabel(ctx *cstNode) draftLabel {
	tokens := v.allTokens(ctx)
	if len(tokens) == 1 &&
		(tokens[0].typ == tokPlainString ||
			tokens[0].typ == tokMarkdownString) {
		return v.tokenLabel(tokens[0])
	}
	s := span{
		tokens[0].startOffset,
		tokens[len(tokens)-1].endOffset + 1,
	}
	return draftLabel{text: v.slice(s[0], s[1]), typ: "text", span: s}
}

func (v *usecaseVisitor) useCaseMetadata(ctx *cstNode) *draftMetadata {
	return v.metadata(v.nodes(ctx, "metadata")[0])
}

func (v *usecaseVisitor) relationTail(ctx *cstNode) *parsedRelationTail {
	tail := &parsedRelationTail{}
	if explicitIDs := v.tokens(ctx, "IDENTIFIER"); len(explicitIDs) > 0 {
		tail.explicitID = explicitIDs[0].image
		tail.explicitIDLocation = v.tokenLocation(explicitIDs[0])
	}
	tail.arrow = v.arrow(v.nodes(ctx, "arrow")[0])
	tail.target = v.entityName(v.nodes(ctx, "entityName")[0])
	return tail
}

func (v *usecaseVisitor) arrow(ctx *cstNode) parsedArrow {
	// firstNode: the arrow rule has one of these children.
	for _, key := range []string{
		"semanticRelation",
		"forwardSolidOperator",
		"backwardSolidOperator",
		"markerlessSolidOperator",
		"forwardCircleOperator",
		"backwardCircleOperator",
		"forwardCrossOperator",
		"backwardCrossOperator",
	} {
		nodes := v.nodes(ctx, key)
		if len(nodes) == 0 {
			continue
		}
		child := nodes[0]
		switch key {
		case "semanticRelation":
			return v.semanticRelation(child)
		case "forwardSolidOperator":
			return v.forwardSolidOperator(child)
		case "backwardSolidOperator":
			return v.backwardSolidOperator(child)
		case "markerlessSolidOperator":
			return v.markerlessSolidOperator(child)
		case "forwardCircleOperator":
			return v.forwardCircleOperator(child)
		case "backwardCircleOperator":
			return v.backwardCircleOperator(child)
		case "forwardCrossOperator":
			return v.forwardCrossOperator(child)
		default:
			return v.backwardCrossOperator(child)
		}
	}
	return parsedArrow{}
}

func (v *usecaseVisitor) edgeLabel(ctx *cstNode) draftLabel {
	return v.nodeLabel(ctx)
}

func (v *usecaseVisitor) semanticRelation(ctx *cstNode) parsedArrow {
	if len(v.tokens(ctx, "GENERALIZATION")) > 0 {
		return parsedArrow{typ: "generalization", arrowType: arrowSolidArrow, minlen: 1}
	}
	typ := "extend"
	if len(v.tokens(ctx, "INCLUDE")) > 0 {
		typ = "include"
	}
	key := "EXTEND"
	if typ == "include" {
		key = "INCLUDE"
	}
	tok := v.tokens(ctx, key)[0]
	return parsedArrow{
		typ:       typ,
		arrowType: arrowSolidArrow,
		label:     &draftLabel{text: typ, typ: "text", span: v.tokenSpan(tok, 0)},
		minlen:    1,
	}
}

func (v *usecaseVisitor) metadata(ctx *cstNode) *draftMetadata {
	nodes := v.nodes(ctx, "metadataProperty")
	properties := make([]draftMetadataProperty, len(nodes))
	for i, node := range nodes {
		properties[i] = v.metadataProperty(node)
	}
	return &draftMetadata{properties: properties}
}

func (v *usecaseVisitor) metadataProperty(ctx *cstNode) draftMetadataProperty {
	var tokens []*token
	for _, tok := range v.allTokens(ctx) {
		if tok.typ != tokColon {
			tokens = append(tokens, tok)
		}
	}
	keyToken := tokens[0]
	valueToken := tokens[1]
	var value any
	switch valueToken.typ {
	case tokTrue:
		value = true
	case tokFalse:
		value = false
	default:
		value = v.decodePlain(valueToken)
	}
	return draftMetadataProperty{
		key:      v.decodePlain(keyToken),
		value:    value,
		location: v.tokenLocation(keyToken),
	}
}

func (v *usecaseVisitor) systemBoundaryStatement(ctx *cstNode) {
	boundary := v.systemBoundaryName(v.nodes(ctx, "systemBoundaryName")[0])
	classNodes := v.nodes(ctx, "classSuffix")
	classes := []string{}
	if len(classNodes) > 0 {
		classes = v.classSuffix(classNodes[0])
	}
	boundary.classes = classes
	v.builder.addBoundary(boundary)
	previous := v.parentBoundary
	v.parentBoundary = &parentBoundary{id: boundary.id, location: boundary.location}
	if contentNodes := v.nodes(ctx, "systemBoundaryContent"); len(contentNodes) > 0 {
		v.systemBoundaryContent(contentNodes[0])
	}
	v.parentBoundary = previous
	metadataNodes := v.nodes(ctx, "metadata")
	if len(metadataNodes) > 0 {
		metadata := v.metadata(metadataNodes[0])
		// Inline metadata resolves exactly like the standalone `id@{ … }` statement, so
		// both forms share one validation path and a later statement can still override.
		v.builder.addMetadataAssignment(boundary.id, boundary.location, metadata)
	}
}

func (v *usecaseVisitor) systemBoundaryName(ctx *cstNode) *draftBoundary {
	if identifiers := v.tokens(ctx, "IDENTIFIER"); len(identifiers) > 0 {
		identifier := identifiers[0]
		var label draftLabel
		if labelNodes := v.nodes(ctx, "nodeLabel"); len(labelNodes) > 0 {
			label = v.nodeLabel(labelNodes[0])
		} else {
			label = v.tokenLabel(identifier)
		}
		return &draftBoundary{
			id:        identifier.image,
			label:     label,
			location:  v.tokenLocation(identifier),
			generated: false,
			classes:   []string{},
		}
	}
	tok := v.stringToken(ctx)
	label := v.tokenLabel(tok)
	return &draftBoundary{
		id:        v.generateID(label.text),
		label:     label,
		location:  v.tokenLocation(tok),
		generated: true,
		classes:   []string{},
	}
}

// systemBoundaryContent visits the lines of a system boundary. Upstream
// sorts the blank lines, the comment lines and the elements by their start
// offset and visits them in that order. Only an element changes the
// builder, and the elements are in source order already.
func (v *usecaseVisitor) systemBoundaryContent(ctx *cstNode) {
	for _, node := range v.nodes(ctx, "boundaryElement") {
		v.boundaryElement(node)
	}
}

func (v *usecaseVisitor) boundaryElement(ctx *cstNode) {
	if actorNodes := v.nodes(ctx, "actorDeclarationOnly"); len(actorNodes) > 0 {
		v.actorDeclarationOnly(actorNodes[0])
		return
	}
	entityNode := v.nodes(ctx, "entityName")[0]
	entity := v.entityName(entityNode)
	entity.explicitDeclaration = true
	if entity.shape == "" {
		entity.shape = "ellipse"
	}
	v.builder.addElement(v.entityDraft(entity))
}

func (v *usecaseVisitor) metadataAssignmentStatement(ctx *cstNode) {
	id, location := v.metadataAssignmentTarget(v.nodes(ctx, "metadataAssignmentTarget")[0])
	metadata := v.metadata(v.nodes(ctx, "metadata")[0])
	v.builder.addMetadataAssignment(id, location, metadata)
}

func (v *usecaseVisitor) metadataAssignmentTarget(ctx *cstNode) (string, draftLocation) {
	tok := v.allTokens(ctx)[0]
	label := v.tokenLabel(tok)
	id := v.generateID(label.text)
	if tok.typ == tokIdentifier {
		id = tok.image
	}
	return id, v.tokenLocation(tok)
}

func (v *usecaseVisitor) noteStatement(ctx *cstNode) {
	target := v.tokens(ctx, "IDENTIFIER")[0]
	label := v.nodeLabel(v.nodes(ctx, "nodeLabel")[0])
	v.builder.addNote(&draftNote{
		target:         target.image,
		targetLocation: v.tokenLocation(target),
		label:          label,
	})
}

func (v *usecaseVisitor) stereotype(ctx *cstNode) *parsedStereotype {
	tok := v.tokens(ctx, "STEREOTYPE_TEXT")[0]
	return &parsedStereotype{value: js.TrimSpace(tok.image)}
}

func (v *usecaseVisitor) classSuffix(ctx *cstNode) []string {
	tokens := v.tokens(ctx, "IDENTIFIER")
	classes := make([]string, len(tokens))
	for i, tok := range tokens {
		classes[i] = tok.image
	}
	return classes
}

func (v *usecaseVisitor) jsonStatement(ctx *cstNode) error {
	start := v.tokens(ctx, "JSON_DECLARATION_START")[0]
	literal := v.tokens(ctx, "JSON_OBJECT_LITERAL")[0]
	// const match = /^json[\t ]+(\w+)/.exec(start.image)!
	image := js.UTF16Units(start.image)
	matchLength := len("json")
	matchLength += countWhile(image, matchLength, isBlank)
	idLength := countWhile(image, matchLength, isWordChar)
	matchLength += idLength
	id := js.FromUTF16Units(image[matchLength-idLength : matchLength])
	// The match is anchored at the start of the image, so the id begins where the
	// matched prefix ends. A text search would instead find ids like `o` or `son`
	// inside the leading `json` keyword.
	relative := matchLength - idLength
	idLocation := draftLocation{
		span:   span{start.startOffset + relative, start.startOffset + relative + idLength},
		line:   start.startLine,
		column: start.startColumn + relative,
	}
	parsed, err := parseOrderedJSONObject(
		literal.image,
		literal.startLine,
		literal.startColumn,
	)
	if err != nil {
		return err
	}
	classNodes := v.nodes(ctx, "classSuffix")
	classes := []string{}
	if len(classNodes) > 0 {
		classes = v.classSuffix(classNodes[0])
	}
	v.builder.addJSON(&draftJSON{
		id:            id,
		value:         parsed.value,
		propertyOrder: parsed.propertyOrder,
		location:      idLocation,
		classes:       classes,
	})
	return nil
}

func (v *usecaseVisitor) directionStatement(ctx *cstNode) {
	for _, value := range v.allTokens(ctx) {
		switch value.typ {
		case tokTd, tokTb, tokBt, tokRl, tokLr:
			v.builder.setDirection(value.image)
			return
		}
	}
}

func (v *usecaseVisitor) classDefStatement(ctx *cstNode) {
	ids := v.tokens(ctx, "IDENTIFIER")
	styles := v.styles(v.nodes(ctx, "styles")[0])
	images := make([]string, len(ids))
	for i, tok := range ids {
		images[i] = tok.image
	}
	v.builder.addClassDef(images, styles)
}

func (v *usecaseVisitor) classStatement(ctx *cstNode) {
	ids := v.tokens(ctx, "IDENTIFIER")
	split := 1
	for ; split < len(ids); split++ {
		between := v.slice(
			ids[split-1].endOffset+1,
			ids[split].startOffset,
		)
		if !strings.Contains(between, ",") {
			break
		}
	}
	// The grammar has a class name after the targets with no comma between
	// them, so split is an index of ids.
	targets := make([]classTarget, 0, split)
	for _, tok := range ids[:split] {
		targets = append(targets, classTarget{id: tok.image, location: v.tokenLocation(tok)})
	}
	classes := make([]string, 0, len(ids)-split)
	for _, tok := range ids[split:] {
		classes = append(classes, tok.image)
	}
	v.builder.addClassAssignment(targets, classes)
}

func (v *usecaseVisitor) styleStatement(ctx *cstNode) {
	target := v.tokens(ctx, "IDENTIFIER")[0]
	styles := v.styles(v.nodes(ctx, "styles")[0])
	v.builder.addStyleAssignment(target.image, v.tokenLocation(target), styles)
}

func (v *usecaseVisitor) styles(ctx *cstNode) []string {
	nodes := v.nodes(ctx, "styleValue")
	styles := make([]string, len(nodes))
	for i, node := range nodes {
		styles[i] = v.styleValue(node)
	}
	return styles
}

func (v *usecaseVisitor) styleValue(ctx *cstNode) string {
	tokens := v.allTokens(ctx)
	// WhiteSpace is Lexer.SKIPPED, so joining the token images would collapse the
	// separators inside multi-word values such as `border:1px solid red`. Commas are
	// consumed by the `styles` rule, so the source slice never spans two values.
	s := span{
		tokens[0].startOffset,
		tokens[len(tokens)-1].endOffset + 1,
	}
	return strings.ReplaceAll(v.slice(s[0], s[1]), `\,`, ",")
}

func (v *usecaseVisitor) forwardSolidOperator(ctx *cstNode) parsedArrow {
	tok := v.tokens(ctx, "FORWARD_SOLID")[0]
	return parsedArrow{
		typ:       "association",
		arrowType: arrowSolidArrow,
		minlen:    v.solidMinlen(tok),
	}
}

func (v *usecaseVisitor) backwardSolidOperator(ctx *cstNode) parsedArrow {
	tok := v.tokens(ctx, "BACKWARD_SOLID")[0]
	labelNodes := v.nodes(ctx, "edgeLabel")
	lengthToken := tok
	if len(labelNodes) > 0 {
		markerless := v.tokens(ctx, "MARKERLESS_SOLID")
		lengthToken = markerless[len(markerless)-1]
	}
	arrow := parsedArrow{
		typ:       "association",
		arrowType: arrowBackArrow,
		minlen:    v.solidMinlen(lengthToken),
	}
	if len(labelNodes) > 0 {
		label := v.edgeLabel(labelNodes[0])
		arrow.label = &label
	}
	return arrow
}

func (v *usecaseVisitor) markerlessSolidOperator(ctx *cstNode) parsedArrow {
	labelNodes := v.nodes(ctx, "edgeLabel")
	tokens := v.allTokens(ctx)
	if len(labelNodes) == 0 {
		return parsedArrow{
			typ:       "association",
			arrowType: arrowLineSolid,
			minlen:    v.solidMinlen(v.tokens(ctx, "MARKERLESS_SOLID")[0]),
		}
	}
	last := tokens[len(tokens)-1]
	arrowType := arrowLineSolid
	switch last.typ {
	case tokForwardSolid:
		arrowType = arrowSolidArrow
	case tokForwardCircle:
		arrowType = arrowCircleArrow
	case tokForwardCross:
		arrowType = arrowCrossArrow
	}
	label := v.edgeLabel(labelNodes[0])
	minlen := 1.0
	if arrowType == arrowSolidArrow || arrowType == arrowLineSolid {
		minlen = v.solidMinlen(last)
	}
	return parsedArrow{
		typ:       "association",
		arrowType: arrowType,
		label:     &label,
		minlen:    minlen,
	}
}

func (v *usecaseVisitor) forwardCircleOperator(_ *cstNode) parsedArrow {
	return parsedArrow{typ: "association", arrowType: arrowCircleArrow, minlen: 1}
}

func (v *usecaseVisitor) backwardCircleOperator(ctx *cstNode) parsedArrow {
	arrow := parsedArrow{
		typ:       "association",
		arrowType: arrowCircleArrowReversed,
		minlen:    1,
	}
	if labelNodes := v.nodes(ctx, "edgeLabel"); len(labelNodes) > 0 {
		label := v.edgeLabel(labelNodes[0])
		arrow.label = &label
	}
	return arrow
}

func (v *usecaseVisitor) forwardCrossOperator(_ *cstNode) parsedArrow {
	return parsedArrow{typ: "association", arrowType: arrowCrossArrow, minlen: 1}
}

func (v *usecaseVisitor) backwardCrossOperator(ctx *cstNode) parsedArrow {
	arrow := parsedArrow{
		typ:       "association",
		arrowType: arrowCrossArrowReversed,
		minlen:    1,
	}
	if labelNodes := v.nodes(ctx, "edgeLabel"); len(labelNodes) > 0 {
		label := v.edgeLabel(labelNodes[0])
		arrow.label = &label
	}
	return arrow
}

func (v *usecaseVisitor) actorDraft(item *parsedItem) *draftElement {
	draft := &draftElement{
		id:        item.id,
		kind:      "actor",
		label:     item.label,
		location:  item.location,
		generated: item.generated,
	}
	if v.parentBoundary != nil {
		draft.parentID, draft.hasParent, draft.parentLocation = v.parentBoundary.id, true, v.parentBoundary.location
	}
	if item.metadata != nil {
		draft.metadata = item.metadata
	}
	if item.stereotype != nil {
		draft.stereotype = item.stereotype.value
	}
	draft.classes = item.classes
	return draft
}

func (v *usecaseVisitor) entityDraft(entity *parsedItem) *draftElement {
	draft := &draftElement{
		id:        entity.id,
		kind:      "usecase",
		label:     entity.label,
		location:  entity.location,
		generated: entity.generated,
	}
	if v.parentBoundary != nil {
		draft.parentID, draft.hasParent, draft.parentLocation = v.parentBoundary.id, true, v.parentBoundary.location
	}
	if entity.shape != "" {
		draft.shape = entity.shape
	}
	if entity.metadata != nil {
		draft.metadata = entity.metadata
	}
	if entity.stereotype != nil {
		draft.stereotype = entity.stereotype.value
	}
	draft.classes = entity.classes
	return draft
}

// endpoint is endpoint(entity, declaration). The default of declaration,
// `entity.explicitDeclaration ?? true`, is the argument of the caller.
func (v *usecaseVisitor) endpoint(entity *parsedItem, declaration bool) draftEndpoint {
	return draftEndpoint{
		id:                 entity.id,
		label:              entity.label,
		location:           entity.location,
		generated:          entity.generated,
		declaration:        declaration,
		classesOnReference: len(entity.classes) > 0,
	}
}

func (v *usecaseVisitor) relationshipDraft(source *parsedItem, tail *parsedRelationTail, location draftLocation) *draftRelationship {
	draft := &draftRelationship{
		source:   v.endpoint(source, true),
		target:   v.endpoint(tail.target, tail.target.explicitDeclaration),
		location: location,
	}
	if tail.explicitID != "" {
		draft.explicitID, draft.explicitIDLocation = tail.explicitID, tail.explicitIDLocation
	}
	draft.typ, draft.arrowType, draft.label, draft.minlen = tail.arrow.typ, tail.arrow.arrowType, tail.arrow.label, tail.arrow.minlen
	return draft
}

func (v *usecaseVisitor) tokenLabel(tok *token) draftLabel {
	typ := "text"
	if tok.typ == tokMarkdownString {
		typ = "markdown"
	}
	trim := 0
	if typ == "markdown" {
		trim = 2
	} else if tok.typ == tokPlainString {
		trim = 1
	}
	s := v.tokenSpan(tok, trim)
	return draftLabel{text: v.slice(s[0], s[1]), typ: typ, span: s}
}

func (v *usecaseVisitor) decodePlain(tok *token) string {
	if tok.typ == tokPlainString {
		// token.image.slice(1, -1): the image has both quotes.
		return tok.image[1 : len(tok.image)-1]
	}
	return tok.image
}

func (v *usecaseVisitor) generateID(label string) string {
	// label.replace(/\W/g, '_'), on UTF-16 code units.
	units := js.UTF16Units(label)
	for i, c := range units {
		if !isWordChar(c) {
			units[i] = '_'
		}
	}
	return js.FromUTF16Units(units)
}

func (v *usecaseVisitor) solidMinlen(tok *token) float64 {
	// Math.max(1, (token.image.match(/-/g)?.length ?? 2) - 1)
	dashes := strings.Count(tok.image, "-")
	if dashes == 0 {
		dashes = 2
	}
	return float64(max(1, dashes-1))
}

func (v *usecaseVisitor) nodeLocation(node *cstNode) draftLocation {
	first := v.allTokens(node)[0]
	return draftLocation{
		span:   v.nodeSpan(node),
		line:   first.startLine,
		column: first.startColumn,
	}
}

func (v *usecaseVisitor) tokenLocation(tok *token) draftLocation {
	trim := 0
	switch tok.typ {
	case tokPlainString:
		trim = 1
	case tokMarkdownString:
		trim = 2
	}
	return draftLocation{
		span:   v.tokenSpan(tok, trim),
		line:   tok.startLine,
		column: tok.startColumn + trim,
	}
}

// nodeSpan is nodeSpan(node). The nodes that the visitor asks for have a
// source token.
func (v *usecaseVisitor) nodeSpan(node *cstNode) span {
	var tokens []*token
	for _, tok := range v.allTokens(node) {
		if tok.typ != tokNewline && tok.typ != tokEof {
			tokens = append(tokens, tok)
		}
	}
	if len(tokens) == 0 {
		return span{}
	}
	first := tokens[0]
	last := tokens[len(tokens)-1]
	return span{
		first.startOffset,
		min(len(v.source), last.endOffset+1),
	}
}

func (v *usecaseVisitor) tokenSpan(tok *token, trim int) span {
	return span{
		tok.startOffset + trim,
		min(len(v.source), tok.endOffset+1-trim),
	}
}

// slice is this.source.slice(start, end).
func (v *usecaseVisitor) slice(start, end int) string {
	start = max(0, min(start, len(v.source)))
	end = max(0, min(end, len(v.source)))
	if start >= end {
		return ""
	}
	return js.FromUTF16Units(v.source[start:end])
}

func (v *usecaseVisitor) nodes(ctx *cstNode, key string) []*cstNode {
	var out []*cstNode
	for _, item := range ctx.children[key] {
		if item.node != nil {
			out = append(out, item.node)
		}
	}
	return out
}

func (v *usecaseVisitor) tokens(ctx *cstNode, key string) []*token {
	var out []*token
	for _, item := range ctx.children[key] {
		if item.tok != nil {
			out = append(out, item.tok)
		}
	}
	return out
}

func (v *usecaseVisitor) firstNode(ctx *cstNode, keys ...string) (*cstNode, error) {
	for _, key := range keys {
		if nodes := v.nodes(ctx, key); len(nodes) > 0 {
			return nodes[0], nil
		}
	}
	return nil, errors.New("Usecase CST is missing one of: " + strings.Join(keys, ", "))
}

// allTokens returns the tokens of a node and of its descendants, sorted by
// their start offset.
func (v *usecaseVisitor) allTokens(ctx *cstNode) []*token {
	var result []*token
	var collect func(ctx *cstNode)
	collect = func(ctx *cstNode) {
		for _, key := range ctx.keys {
			for _, value := range ctx.children[key] {
				if value.tok != nil {
					result = append(result, value.tok)
				} else {
					collect(value.node)
				}
			}
		}
	}
	collect(ctx)
	js.SortFunc(result, func(a, b *token) float64 { return float64(a.startOffset - b.startOffset) })
	return result
}
