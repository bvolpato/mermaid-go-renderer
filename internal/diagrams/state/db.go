// Ported from mermaid 12.2.0, diagrams/state/stateDb.ts (MIT).

package state

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// CONSTANTS of stateDb.ts.
const (
	startNode     = "[*]"
	startType     = "start"
	endNode       = "[*]"
	endType       = "end"
	colorKeyword  = "color"
	fillKeyword   = "fill"
	bgFill        = "bgFill"
	styleClassSep = ","
)

// State is the StateStmt that addState creates: the entry of a state in a
// document. The statements of the parser are dynamic objects (see stmt.go).
type State struct {
	// Stmt is always "state".
	Stmt         string
	ID           string
	Descriptions []string
	Type         string
	// Doc is the document of a composite state: a []any, or nil when the
	// property is undefined. It is the array of the statement.
	Doc any
	// Note is the note object of the statement, or nil. The db shares it
	// with the statement and changes its text in place.
	Note       *js.Obj[any]
	Classes    []string
	Styles     []string
	TextStyles []string
}

// Relation is DiagramEdge: a relation of a document.
type Relation struct {
	ID1, ID2      string
	RelationTitle string
	// HasRelationTitle is false when relationTitle is undefined.
	HasRelationTitle bool
}

// Link is the value of the links map: { url, tooltip }. The parser gives
// strings.
type Link struct {
	URL     any
	Tooltip any
}

// document is Document: the relations and the states of a document. Upstream
// has a `documents` record too, which nothing fills.
type document struct {
	relations []*Relation
	states    *js.Map[string, *State]
}

// newDoc is newDoc().
func newDoc() *document {
	return &document{states: js.NewMap[string, *State]()}
}

// DB is the class StateDB. It also holds the module-level state of
// dataFetcher.ts, which upstream resets at the start of every extract.
type DB struct {
	render.CommonDB
	ctx *render.Context
	// version is 1 for the old renderer and 2 for the unified renderer.
	// Mermaid 12.2.0 creates the db with version 2 only.
	version int

	// nodes and edges are the layout data that dataFetcher builds. They are
	// JavaScript objects: see NodeData and Edge of stateDb.ts.
	nodes   []*js.Obj[any]
	edges   []*js.Obj[any]
	rootDoc []any
	classes *js.Map[string, *render.ClassDef]
	// documents is { root: newDoc() }, and currentDocument is its root.
	currentDocument *document
	// startEndCount is the number of start and end nodes. It is used to
	// construct ids.
	startEndCount int
	dividerCnt    int
	// links maps a state to its link. The key is the value that addLink
	// receives: the parser gives the id statement, an object, and a Map
	// compares objects by identity.
	links *js.Map[any, *Link]

	fetch fetcherState

	// initErr is the exception of the init hook of the diagram definition.
	initErr error
}

// NewDB is new StateDB(version) for a render.
func NewDB(ctx *render.Context, version int) *DB {
	d := &DB{ctx: ctx, version: version, rootDoc: []any{}}
	d.CommonDB.Sanitize = ctx.SanitizeText
	d.links = js.NewMap[any, *Link]()
	d.clear(false)
	return d
}

// config is getConfig().
func (d *DB) config() *js.Obj[any] { return d.ctx.Config }

// sanitizeText is common.sanitizeText(text, getConfig()).
func (d *DB) sanitizeText(text string) (string, error) {
	return d.ctx.Sanitizer.SanitizeTextErr(text, d.config())
}

// Extract is extract(statements).
//
// It converts all of the statements (stmts) that were parsed into states and
// relationships. This is done because a state diagram may have nested
// sections, where each section is a 'document' and has its own set of
// statements. statements is a document or an object with a doc.
func (d *DB) Extract(statements any) error {
	d.clear(true)
	// Array.isArray(statements) ? statements : statements.doc
	items, ok := statements.([]any)
	if !ok {
		if items, ok = prop(statements, "doc").([]any); !ok {
			return typeError("(intermediate value) is not iterable")
		}
	}
	for _, item := range items {
		stmt, _ := strProp(item, "stmt")
		switch stmt {
		case stmtState:
			id, ok := strProp(item, "id")
			if !ok {
				return typeError("Cannot read properties of undefined (reading 'trim')")
			}
			if err := d.addState(js.TrimSpace(id), prop(item, "type"), prop(item, "doc"), prop(item, "description"), prop(item, "note"), nil, nil, nil); err != nil {
				return err
			}
		case stmtRelation:
			if err := d.AddRelation(prop(item, "state1"), prop(item, "state2"), prop(item, "description")); err != nil {
				return err
			}
		case stmtClassDef:
			id, ok := strProp(item, "id")
			if !ok {
				return typeError("Cannot read properties of undefined (reading 'trim')")
			}
			d.addStyleClass(js.TrimSpace(id), prop(item, "classes"))
		case stmtStyleDef:
			if err := d.handleStyleDef(item); err != nil {
				return err
			}
		case stmtApplyClass:
			id, ok := strProp(item, "id")
			if !ok {
				return typeError("Cannot read properties of undefined (reading 'trim')")
			}
			styleClass, _ := strProp(item, "styleClass")
			if err := d.setCssClass(js.TrimSpace(id), styleClass); err != nil {
				return err
			}
		case "click":
			d.AddLink(prop(item, "id"), prop(item, "url"), prop(item, "tooltip"))
		}
	}
	diagramStates := d.GetStates()
	config := d.config()

	d.fetch.reset()
	root, err := d.getRootDocV2()
	if err != nil {
		return err
	}
	if err := d.dataFetcher(nil, root, diagramStates, true, config.Get("look")); err != nil {
		return err
	}

	// Process node labels
	for _, node := range d.nodes {
		label, ok := node.Get("label").([]any)
		if !ok {
			continue
		}

		// node.label.slice(1)
		description := []any{}
		if len(label) > 1 {
			description = append(description, label[1:]...)
		}
		node.Set("description", description)
		if js.Truthy(node.Get("isGroup")) && len(description) > 0 {
			return newError("Group nodes can only have label. Remove the additional description for node [" + js.ToString(node.Get("id")) + "]")
		}
		if len(label) > 0 {
			node.Set("label", label[0])
		} else {
			node.Set("label", nil)
		}
	}
	return nil
}

// handleStyleDef is handleStyleDef(item).
func (d *DB) handleStyleDef(item any) error {
	itemID, ok := strProp(item, "id")
	if !ok {
		return typeError("Cannot read properties of undefined (reading 'trim')")
	}
	styleClass, ok := strProp(item, "styleClass")
	if !ok {
		return typeError("Cannot read properties of undefined (reading 'split')")
	}
	ids := strings.Split(js.TrimSpace(itemID), ",")
	styles := strings.Split(styleClass, ",")

	for _, id := range ids {
		state := d.GetState(id)
		if state == nil {
			trimmedID := js.TrimSpace(id)
			if err := d.addState(trimmedID, nil, nil, nil, nil, nil, nil, nil); err != nil {
				return err
			}
			state = d.GetState(trimmedID)
		}
		if state != nil {
			// styles.map((s) => s.replace(/;/g, '')?.trim())
			list := make([]string, len(styles))
			for i, s := range styles {
				list[i] = js.TrimSpace(strings.ReplaceAll(s, ";", ""))
			}
			state.Styles = list
		}
	}
	return nil
}

// SetRootDoc is setRootDoc(o).
func (d *DB) SetRootDoc(o []any) error {
	d.rootDoc = o
	if d.version == 1 {
		return d.Extract(o)
	}
	root, err := d.getRootDocV2()
	if err != nil {
		return err
	}
	return d.Extract(root)
}

// docTranslator is docTranslator(parent, node, first). It gives the start
// and end states their ids, and splits a document at its concurrency
// dividers. node is a line of a document: a statement or a string.
func (d *DB) docTranslator(parentNode *js.Obj[any], node any, first bool) error {
	n, ok := node.(*js.Obj[any])
	if !ok || n == nil {
		// A string has no stmt and no doc.
		return nil
	}
	stmt, _ := n.Get("stmt").(string)
	if stmt == stmtRelation {
		if err := d.docTranslator(parentNode, n.Get("state1"), true); err != nil {
			return err
		}
		return d.docTranslator(parentNode, n.Get("state2"), false)
	}

	if stmt == stmtState {
		id, isString := n.Get("id").(string)
		if isString && id == startNode {
			suffix := "_end"
			if first {
				suffix = "_start"
			}
			n.Set("id", js.ToString(parentNode.Get("id"))+suffix)
			n.Set("start", first)
		} else {
			// This is just a plain state, not a start or end
			if !isString {
				return typeError("node.id.trim is not a function")
			}
			n.Set("id", js.TrimSpace(id))
		}
	}

	if (stmt != stmtRoot && stmt != stmtState) || !js.Truthy(n.Get("doc")) {
		return nil
	}
	nodeDoc, ok := n.Get("doc").([]any)
	if !ok {
		return typeError("node.doc is not iterable")
	}

	doc := []any{}
	// Check for concurrency
	currentDoc := []any{}
	for _, docStmt := range nodeDoc {
		if typ, _ := strProp(docStmt, "type"); typ == dividerType {
			newNode := clone(docStmt).(*js.Obj[any])
			newNode.Set("doc", clone(currentDoc))
			doc = append(doc, newNode)
			currentDoc = []any{}
		} else {
			currentDoc = append(currentDoc, docStmt)
		}
	}

	// If any divider was encountered
	if len(doc) > 0 && len(currentDoc) > 0 {
		newNode := newObj(
			"stmt", stmtState,
			"id", utils.GenerateId(d.ctx),
			"type", "divider",
			"doc", clone(currentDoc),
		)
		doc = append(doc, clone(newNode))
		n.Set("doc", doc)
		nodeDoc = doc
	}

	for _, docNode := range nodeDoc {
		if err := d.docTranslator(n, docNode, true); err != nil {
			return err
		}
	}
	return nil
}

// getRootDocV2 is getRootDocV2().
func (d *DB) getRootDocV2() (*js.Obj[any], error) {
	err := d.docTranslator(
		newObj("id", stmtRoot, "stmt", stmtRoot),
		newObj("id", stmtRoot, "stmt", stmtRoot, "doc", d.rootDoc),
		true,
	)
	if err != nil {
		return nil, err
	}
	return newObj("id", stmtRoot, "doc", d.rootDoc), nil
}

// eachTrimmed calls fn for a value or for every element of an array:
// (Array.isArray(v) ? v : [v]).forEach((x) => fn(x.trim())). name is the
// name of x upstream, for the message of the exception.
func eachTrimmed(v any, name string, fn func(s string) error) error {
	list, ok := v.([]any)
	if !ok {
		list = []any{v}
	}
	for _, x := range list {
		s, ok := x.(string)
		if !ok {
			if js.IsNullish(x) {
				return typeError("Cannot read properties of " + js.ToString(x) + " (reading 'trim')")
			}
			return typeError(name + ".trim is not a function")
		}
		if err := fn(js.TrimSpace(s)); err != nil {
			return err
		}
	}
	return nil
}

// addState is addState(id, type, doc, descr, note, classes, styles,
// textStyles): the parser found a node definition. An argument that is
// undefined is nil.
//
// descr is the description for the state: a string or a list of strings.
// classes, styles and textStyles are a string or a list of strings each.
func (d *DB) addState(id string, typ, doc, descr, note, classes, styles, textStyles any) error {
	// type = DEFAULT_STATE_TYPE
	stateType := defaultStateType
	if typ != nil {
		stateType = js.ToString(typ)
	}
	trimmedID := js.TrimSpace(id)
	states := d.currentDocument.states
	if !states.Has(trimmedID) {
		states.Set(trimmedID, &State{
			Stmt:         stmtState,
			ID:           trimmedID,
			Descriptions: []string{},
			Type:         stateType,
			Doc:          doc,
			Note:         js.AsObj(note),
			Classes:      []string{},
			Styles:       []string{},
			TextStyles:   []string{},
		})
	} else {
		state := states.Get(trimmedID)
		if state == nil {
			return newError("State not found: " + trimmedID)
		}
		if !js.Truthy(state.Doc) {
			state.Doc = doc
		}
		if state.Type == "" {
			state.Type = stateType
		}
	}

	if js.Truthy(descr) {
		err := eachTrimmed(descr, "des", func(des string) error { return d.addDescription(trimmedID, des) })
		if err != nil {
			return err
		}
	}

	if js.Truthy(note) {
		doc2 := states.Get(trimmedID)
		if doc2 == nil {
			return newError("State not found: " + trimmedID)
		}
		noteObj := js.AsObj(note)
		if noteObj == nil {
			return typeError("Cannot create property 'text' on " + js.TypeOf(note) + " '" + js.ToString(note) + "'")
		}
		doc2.Note = noteObj
		if text, ok := noteObj.Get("text").(string); ok {
			sanitized, err := d.sanitizeText(text)
			if err != nil {
				return err
			}
			noteObj.Set("text", sanitized)
		}
	}

	if js.Truthy(classes) {
		err := eachTrimmed(classes, "cssClass", func(cssClass string) error { return d.setCssClass(trimmedID, cssClass) })
		if err != nil {
			return err
		}
	}

	if js.Truthy(styles) {
		err := eachTrimmed(styles, "style", func(style string) error {
			d.setStyle(trimmedID, style)
			return nil
		})
		if err != nil {
			return err
		}
	}

	if js.Truthy(textStyles) {
		err := eachTrimmed(textStyles, "textStyle", func(textStyle string) error {
			d.setTextStyle(trimmedID, textStyle)
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// clear is clear(saveCommon).
func (d *DB) clear(saveCommon bool) {
	d.nodes = []*js.Obj[any]{}
	d.edges = []*js.Obj[any]{}
	d.currentDocument = newDoc()

	// number of start and end nodes; used to construct ids
	d.startEndCount = 0
	d.classes = js.NewMap[string, *render.ClassDef]()
	if !saveCommon {
		d.links = js.NewMap[any, *Link]()
		d.ClearCommon()
	}
}

// GetState is getState(id).
func (d *DB) GetState(id string) *State {
	return d.currentDocument.states.Get(id)
}

// GetStates is getStates().
func (d *DB) GetStates() *js.Map[string, *State] {
	return d.currentDocument.states
}

// GetRelations is getRelations().
func (d *DB) GetRelations() []*Relation {
	return d.currentDocument.relations
}

// AddLink is addLink(stateId, url, tooltip): it adds a clickable link to a
// state.
func (d *DB) AddLink(stateID, url, tooltip any) {
	d.links.Set(stateID, &Link{URL: url, Tooltip: tooltip})
}

// GetLinks is getLinks(): all registered links.
func (d *DB) GetLinks() *js.Map[any, *Link] {
	return d.links
}

// startIDIfNeeded is startIdIfNeeded(id). If the id is a start node ( [*] ),
// it returns a new id constructed from the start node name and the current
// start node count. Else it returns the given id.
func (d *DB) startIDIfNeeded(id string) string {
	if id == startNode {
		d.startEndCount++
		return startType + strconv.Itoa(d.startEndCount)
	}
	return id
}

// startTypeIfNeeded is startTypeIfNeeded(id, type). If the id is a start
// node ( [*] ), it returns the start type ('start'). Else it returns the
// given type, which is nil when it is undefined.
func (d *DB) startTypeIfNeeded(id string, typ any) any {
	if id == startNode {
		return startType
	}
	if typ == nil {
		return defaultStateType
	}
	return typ
}

// endIDIfNeeded is endIdIfNeeded(id). If the id is an end node ( [*] ), it
// returns a new id constructed from the end node name and the current
// start_end node count. Else it returns the given id.
func (d *DB) endIDIfNeeded(id string) string {
	if id == endNode {
		d.startEndCount++
		return endType + strconv.Itoa(d.startEndCount)
	}
	return id
}

// endTypeIfNeeded is endTypeIfNeeded(id, type). If the id is an end node
// ( [*] ), it returns the end type. Else it returns the given type.
func (d *DB) endTypeIfNeeded(id string, typ any) any {
	if id == endNode {
		return endType
	}
	if typ == nil {
		return defaultStateType
	}
	return typ
}

// addRelationObjs is addRelationObjs(item1, item2, relationTitle).
func (d *DB) addRelationObjs(item1, item2 *js.Obj[any], relationTitle string) error {
	itemID1, ok1 := item1.Get("id").(string)
	itemID2, ok2 := item2.Get("id").(string)
	if !ok1 || !ok2 {
		return typeError("Cannot read properties of undefined (reading 'trim')")
	}
	id1 := d.startIDIfNeeded(js.TrimSpace(itemID1))
	type1 := d.startTypeIfNeeded(js.TrimSpace(itemID1), item1.Get("type"))
	id2 := d.startIDIfNeeded(js.TrimSpace(itemID2))
	type2 := d.startTypeIfNeeded(js.TrimSpace(itemID2), item2.Get("type"))
	err := d.addState(
		id1,
		type1,
		item1.Get("doc"),
		item1.Get("description"),
		item1.Get("note"),
		item1.Get("classes"),
		item1.Get("styles"),
		item1.Get("textStyles"),
	)
	if err != nil {
		return err
	}
	err = d.addState(
		id2,
		type2,
		item2.Get("doc"),
		item2.Get("description"),
		item2.Get("note"),
		item2.Get("classes"),
		item2.Get("styles"),
		item2.Get("textStyles"),
	)
	if err != nil {
		return err
	}
	title, err := d.sanitizeText(relationTitle)
	if err != nil {
		return err
	}
	d.currentDocument.relations = append(d.currentDocument.relations, &Relation{
		ID1:              id1,
		ID2:              id2,
		RelationTitle:    title,
		HasRelationTitle: true,
	})
	return nil
}

// AddRelation is addRelation(item1, item2, title): it adds a relation
// between two items. The items may be full objects or just the string id of
// a state.
func (d *DB) AddRelation(item1, item2, title any) error {
	obj1, isObj1 := item1.(*js.Obj[any])
	obj2, isObj2 := item2.(*js.Obj[any])
	str1, isStr1 := item1.(string)
	str2, isStr2 := item2.(string)
	if isObj1 && isObj2 {
		// relationTitle = ''
		relationTitle := ""
		if title != nil {
			relationTitle = js.ToString(title)
		}
		return d.addRelationObjs(obj1, obj2, relationTitle)
	} else if isStr1 && isStr2 {
		id1 := d.startIDIfNeeded(js.TrimSpace(str1))
		type1 := d.startTypeIfNeeded(str1, nil)
		id2 := d.endIDIfNeeded(js.TrimSpace(str2))
		type2 := d.endTypeIfNeeded(str2, nil)

		if err := d.addState(id1, type1, nil, nil, nil, nil, nil, nil); err != nil {
			return err
		}
		if err := d.addState(id2, type2, nil, nil, nil, nil, nil, nil); err != nil {
			return err
		}
		relation := &Relation{ID1: id1, ID2: id2}
		if js.Truthy(title) {
			sanitized, err := d.sanitizeText(js.ToString(title))
			if err != nil {
				return err
			}
			relation.RelationTitle, relation.HasRelationTitle = sanitized, true
		}
		d.currentDocument.relations = append(d.currentDocument.relations, relation)
	}
	return nil
}

// addDescription is addDescription(id, descr).
func (d *DB) addDescription(id, descr string) error {
	theState := d.currentDocument.states.Get(id)
	_descr := descr
	if strings.HasPrefix(descr, ":") {
		_descr = js.TrimSpace(strings.Replace(descr, ":", "", 1))
	}
	if theState == nil {
		return nil
	}
	sanitized, err := d.sanitizeText(_descr)
	if err != nil {
		return err
	}
	theState.Descriptions = append(theState.Descriptions, sanitized)
	return nil
}

// cleanupLabel is cleanupLabel(label).
func (d *DB) cleanupLabel(label string) string {
	if strings.HasPrefix(label, ":") {
		return js.TrimSpace(js.StringSliceFrom(label, 2))
	}
	return js.TrimSpace(label)
}

// getDividerID is getDividerId().
func (d *DB) getDividerID() string {
	d.dividerCnt++
	return "divider-id-" + strconv.Itoa(d.dividerCnt)
}

// addStyleClass is addStyleClass(id, styleAttributes): the parser came
// across a (style) class definition, for example
//
//	classDef my-style fill:#f96;
//
// id is the id of this (style) class. styleAttributes is the string with 1
// or more style attributes (each separated by a comma).
func (d *DB) addStyleClass(id string, styleAttributes any) {
	// styleAttributes = ''
	attributes := ""
	if styleAttributes != nil {
		attributes = js.ToString(styleAttributes)
	}
	// create a new style class object with this id
	if !d.classes.Has(id) {
		d.classes.Set(id, &render.ClassDef{ID: id, Styles: []string{}, TextStyles: []string{}})
	}
	foundClass := d.classes.Get(id)
	if attributes != "" && foundClass != nil {
		for _, attrib := range strings.Split(attributes, styleClassSep) {
			// attrib.replace(/([^;]*);/, '$1'): it removes the first ";".
			fixedAttrib := js.TrimSpace(strings.Replace(attrib, ";", "", 1))
			if strings.Contains(attrib, colorKeyword) {
				newStyle1 := strings.Replace(fixedAttrib, fillKeyword, bgFill, 1)
				newStyle2 := strings.Replace(newStyle1, colorKeyword, fillKeyword, 1)
				foundClass.TextStyles = append(foundClass.TextStyles, newStyle2)
			}
			foundClass.Styles = append(foundClass.Styles, fixedAttrib)
		}
	}
}

// GetClasses is getClasses().
func (d *DB) GetClasses() *js.Map[string, *render.ClassDef] {
	return d.classes
}

// setCssClass is setCssClass(itemIds, cssClassName): it adds a (style)
// class or css class to a state with the given id. If the state isn't
// already in the list of known states, it adds it. itemIds is the id or a
// list of ids of the item(s) to apply the css class to.
func (d *DB) setCssClass(itemIDs, cssClassName string) error {
	for _, id := range strings.Split(itemIDs, ",") {
		foundState := d.GetState(id)
		if foundState == nil {
			trimmedID := js.TrimSpace(id)
			if err := d.addState(trimmedID, nil, nil, nil, nil, nil, nil, nil); err != nil {
				return err
			}
			foundState = d.GetState(trimmedID)
		}
		if foundState != nil {
			foundState.Classes = append(foundState.Classes, cssClassName)
		}
	}
	return nil
}

// setStyle is setStyle(itemId, styleText): it adds a style to a state with
// the given id, for example
//
//	style stateId fill:#f9f,stroke:#333,stroke-width:4px
func (d *DB) setStyle(itemID, styleText string) {
	if state := d.GetState(itemID); state != nil {
		state.Styles = append(state.Styles, styleText)
	}
}

// setTextStyle is setTextStyle(itemId, cssClassName): it adds a text style
// to a state with the given id.
func (d *DB) setTextStyle(itemID, cssClassName string) {
	if state := d.GetState(itemID); state != nil {
		state.TextStyles = append(state.TextStyles, cssClassName)
	}
}

// getDirectionStatement is getDirectionStatement(): it finds the direction
// statement in the root document.
func (d *DB) getDirectionStatement() *js.Obj[any] {
	for _, doc := range d.rootDoc {
		if stmt, _ := strProp(doc, "stmt"); stmt == stmtDirection {
			return doc.(*js.Obj[any])
		}
	}
	return nil
}

// GetDirection is getDirection().
func (d *DB) GetDirection() any {
	if doc := d.getDirectionStatement(); doc != nil {
		if value := doc.Get("value"); !js.IsNullish(value) {
			return value
		}
	}
	return defaultDiagramDirection
}

// SetDirection is setDirection(dir).
func (d *DB) SetDirection(dir string) {
	doc := d.getDirectionStatement()
	if doc != nil {
		doc.Set("value", dir)
	} else {
		// this.rootDoc.unshift({ stmt: STMT_DIRECTION, value: dir })
		rootDoc := make([]any, 0, len(d.rootDoc)+1)
		rootDoc = append(rootDoc, newObj("stmt", stmtDirection, "value", dir))
		d.rootDoc = append(rootDoc, d.rootDoc...)
	}
}

// trimColon is trimColon(str).
func (d *DB) trimColon(str string) string {
	if strings.HasPrefix(str, ":") {
		return js.TrimSpace(js.StringSliceFrom(str, 1))
	}
	return js.TrimSpace(str)
}

// GetData is getData(): the layout data of the diagram.
func (d *DB) GetData() (*ru.LayoutData, error) {
	config := d.config()
	for _, node := range d.nodes {
		// node.wrappingWidth ??= config.state?.wrappingWidth
		if js.IsNullish(node.Get("wrappingWidth")) {
			node.Set("wrappingWidth", js.Path(config, "state", "wrappingWidth"))
		}
		if !js.Truthy(node.Get("isGroup")) {
			// node.minWidth ??= config.state?.minNodeWidth
			if js.IsNullish(node.Get("minWidth")) {
				node.Set("minWidth", js.Path(config, "state", "minNodeWidth"))
			}
		}
	}
	root, err := d.getRootDocV2()
	if err != nil {
		return nil, err
	}
	data := &ru.LayoutData{
		Nodes:  make([]*ru.Node, len(d.nodes)),
		Edges:  make([]*ru.Edge, len(d.edges)),
		Config: config,
	}
	for i, node := range d.nodes {
		data.Nodes[i] = layoutNode(node)
	}
	for i, edge := range d.edges {
		data.Edges[i] = layoutEdge(edge)
	}
	if direction := getDir(root, defaultNestedDocDir); direction != nil {
		data.Direction = js.ToString(direction)
	}
	return data, nil
}

// GetConfig is getConfig(): the state section of the config.
func (d *DB) GetConfig() any {
	return d.config().Get("state")
}

// Upstream has two more members, setupToolTips and bindFunctions. They add
// event listeners for the tooltips after mermaid.run inserted the SVG into
// the page. mermaid.render does not call them, so they have no port.
