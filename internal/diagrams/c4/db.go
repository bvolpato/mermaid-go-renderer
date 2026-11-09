// Ported from mermaid 12.2.0, diagrams/c4/c4Db.ts (MIT).

package c4

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// A C4 shape, boundary or relationship is a bag of properties upstream: the
// parser can set every key with a `$key="value"` attribute. The port keeps
// each bag as a *js.Obj[any], so that the keys, their order and the types of
// the values are the same as in JavaScript. The values follow the model of
// internal/js/value.go. A text field (label, descr, techn, type) is an object
// with the key "text".
//
// A parser attribute is a string, or an object with one entry for a named
// attribute. A missing attribute is nil (undefined).

// errNotIterable is the TypeError of `const [key, value] = undefined`. It
// happens when a named attribute has no own property, as with `$__proto__`.
var errNotIterable = errors.New("TypeError: undefined is not iterable (cannot read property Symbol(Symbol.iterator))")

// textFields is TEXT_FIELDS: the fields that a C4 element, boundary or
// relationship stores as `{ text }`.
var textFields = map[string]bool{"label": true, "descr": true, "techn": true, "type": true}

// db is the state of c4Db.ts.
type db struct {
	render.CommonDB
	ctx *render.Context

	c4ShapeArray         []*js.Obj[any]
	boundaryParseStack   []any
	currentBoundaryParse any
	parentBoundaryParse  any
	boundaries           []*js.Obj[any]
	rels                 []*js.Obj[any]
	title                string
	wrapEnabled          any
	c4ShapeInRow         float64
	c4BoundaryInRow      float64
	c4Type               any

	// rs is the state of the renderer module (c4Renderer.ts).
	rs rendererState
}

// newDB returns a db in the state after clear().
func newDB(ctx *render.Context) *db {
	d := &db{ctx: ctx, rs: newRendererState()}
	d.CommonDB.Sanitize = d.sanitizeText
	d.clear()
	return d
}

// sanitizeText is sanitizeText(text, getConfig()).
func (d *db) sanitizeText(text string) string {
	return d.ctx.SanitizeText(text)
}

// text is `{ text: value }`.
func text(value any) *js.Obj[any] {
	o := js.NewObj[any]()
	o.Set("text", value)
	return o
}

// setKey is `bag[key] = value`. An assignment to __proto__ adds no own
// property: the value becomes the prototype, or is ignored.
func setKey(bag *js.Obj[any], key string, value any) {
	if key == "__proto__" {
		return
	}
	bag.Set(key, value)
}

// firstEntry is `Object.entries(value)[0]`. The second result is false when
// the object has no entry.
func firstEntry(value *js.Obj[any]) (key string, val any, ok bool) {
	keys := value.Keys()
	if len(keys) == 0 {
		return "", nil, false
	}
	return keys[0], value.Get(keys[0]), true
}

// isObject is `typeof value === 'object'` for a parser attribute.
func isObject(value any) (*js.Obj[any], bool) {
	o, ok := value.(*js.Obj[any])
	return o, ok && o != nil
}

// createGlobalBoundary is createGlobalBoundary().
func createGlobalBoundary() *js.Obj[any] {
	b := js.NewObj[any]()
	b.Set("alias", "global")
	b.Set("label", text("global"))
	b.Set("type", text("global"))
	b.Set("tags", js.Null)
	b.Set("link", js.Null)
	b.Set("parentBoundary", "")
	return b
}

// attrSlot is one entry of the object that assignAttributes gets.
type attrSlot struct {
	field string
	value any
}

// assignAttributes applies optional C4 attributes to bag. A value is a raw
// positional value or one named override. Undefined is skipped, so that an
// earlier value stays.
func assignAttributes(bag *js.Obj[any], attrs ...attrSlot) error {
	for _, a := range attrs {
		if a.value == nil {
			continue
		}
		if named, ok := isObject(a.value); ok {
			key, val, ok := firstEntry(named)
			if !ok {
				return errNotIterable
			}
			if textFields[key] {
				setKey(bag, key, text(val))
			} else {
				setKey(bag, key, val)
			}
		} else {
			bag.Set(a.field, a.value)
		}
	}
	return nil
}

// setTextSlot ports the block that every add function has for a text
// argument (techn, descr):
//
//	if (value === undefined || value === null) { bag[field] ??= { text: '' } }
//	else if (typeof value === 'object') { bag[key] = TEXT_FIELDS.has(key) ? { text: v } : v }
//	else { bag[field] = { text: value } }
func setTextSlot(bag *js.Obj[any], field string, value any) error {
	if js.IsNullish(value) {
		if js.IsNullish(bag.Get(field)) {
			bag.Set(field, text(""))
		}
		return nil
	}
	if named, ok := isObject(value); ok {
		key, val, ok := firstEntry(named)
		if !ok {
			return errNotIterable
		}
		if textFields[key] {
			setKey(bag, key, text(val))
		} else {
			setKey(bag, key, val)
		}
		return nil
	}
	bag.Set(field, text(value))
	return nil
}

// setTypeSlot ports the block for the type argument of a boundary. def is
// the type when the argument is missing.
func setTypeSlot(bag *js.Obj[any], value any, def string) error {
	if js.IsNullish(value) {
		bag.Set("type", text(def))
		return nil
	}
	if named, ok := isObject(value); ok {
		key, val, ok := firstEntry(named)
		if !ok {
			return errNotIterable
		}
		if textFields[key] {
			setKey(bag, key, text(val))
		} else {
			setKey(bag, key, val)
		}
		return nil
	}
	bag.Set("type", text(value))
	return nil
}

// setLabel ports the label block: an undefined label becomes the empty text.
func setLabel(bag *js.Obj[any], label any) {
	if js.IsNullish(label) {
		bag.Set("label", text(""))
	} else {
		bag.Set("label", text(label))
	}
}

// findByAlias is `list.find((e) => e.alias === alias)`.
func findByAlias(list []*js.Obj[any], alias any) *js.Obj[any] {
	for _, e := range list {
		if js.StrictEquals(e.Get("alias"), alias) {
			return e
		}
	}
	return nil
}

// findRel is `rels.find((rel) => rel.from === from && rel.to === to)`.
func (d *db) findRel(from, to any) *js.Obj[any] {
	for _, rel := range d.rels {
		if js.StrictEquals(rel.Get("from"), from) && js.StrictEquals(rel.Get("to"), to) {
			return rel
		}
	}
	return nil
}

// getC4Type is getC4Type().
func (d *db) getC4Type() any { return d.c4Type }

// setC4Type is setC4Type(c4TypeParam).
func (d *db) setC4Type(c4TypeParam string) {
	d.c4Type = d.sanitizeText(c4TypeParam)
}

// addRel is addRel(type, from, to, label, techn, descr, sprite, tags, link).
func (d *db) addRel(typ, from, to, label, techn, descr, sprite, tags, link any) error {
	// Don't allow label nulling
	if js.IsNullish(typ) || js.IsNullish(from) || js.IsNullish(to) || js.IsNullish(label) {
		return nil
	}

	rel := d.findRel(from, to)
	if rel == nil {
		rel = js.NewObj[any]()
		d.rels = append(d.rels, rel)
	}

	rel.Set("type", typ)
	rel.Set("from", from)
	rel.Set("to", to)
	rel.Set("label", text(label))

	if err := setTextSlot(rel, "techn", techn); err != nil {
		return err
	}
	if err := setTextSlot(rel, "descr", descr); err != nil {
		return err
	}

	if err := assignAttributes(rel, attrSlot{"sprite", sprite}, attrSlot{"tags", tags}, attrSlot{"link", link}); err != nil {
		return err
	}
	rel.Set("wrap", d.autoWrap())
	return nil
}

// shapeFor returns the shape with the alias, or adds a new one.
func (d *db) shapeFor(alias any) *js.Obj[any] {
	if old := findByAlias(d.c4ShapeArray, alias); old != nil {
		return old
	}
	shape := js.NewObj[any]()
	shape.Set("alias", alias)
	d.c4ShapeArray = append(d.c4ShapeArray, shape)
	return shape
}

// boundaryFor returns the boundary with the alias, or adds a new one.
func (d *db) boundaryFor(alias any) *js.Obj[any] {
	if old := findByAlias(d.boundaries, alias); old != nil {
		return old
	}
	boundary := js.NewObj[any]()
	boundary.Set("alias", alias)
	d.boundaries = append(d.boundaries, boundary)
	return boundary
}

// addPersonOrSystem is addPersonOrSystem(typeC4Shape, alias, label, descr,
// sprite, tags, link).
func (d *db) addPersonOrSystem(typeC4Shape string, alias, label, descr, sprite, tags, link any) error {
	// Don't allow label nulling
	if isNull(alias) || isNull(label) {
		return nil
	}

	personOrSystem := d.shapeFor(alias)

	// Don't allow null labels, either
	setLabel(personOrSystem, label)

	if err := setTextSlot(personOrSystem, "descr", descr); err != nil {
		return err
	}

	if err := assignAttributes(personOrSystem, attrSlot{"sprite", sprite}, attrSlot{"tags", tags}, attrSlot{"link", link}); err != nil {
		return err
	}
	personOrSystem.Set("typeC4Shape", text(typeC4Shape))
	personOrSystem.Set("parentBoundary", d.currentBoundaryParse)
	personOrSystem.Set("wrap", d.autoWrap())
	return nil
}

// addContainer is addContainer(typeC4Shape, alias, label, techn, descr,
// sprite, tags, link).
func (d *db) addContainer(typeC4Shape string, alias, label, techn, descr, sprite, tags, link any) error {
	// Don't allow label nulling
	if isNull(alias) || isNull(label) {
		return nil
	}

	container := d.shapeFor(alias)

	// Don't allow null labels, either
	setLabel(container, label)

	if err := setTextSlot(container, "techn", techn); err != nil {
		return err
	}
	if err := setTextSlot(container, "descr", descr); err != nil {
		return err
	}

	if err := assignAttributes(container, attrSlot{"sprite", sprite}, attrSlot{"tags", tags}, attrSlot{"link", link}); err != nil {
		return err
	}
	container.Set("wrap", d.autoWrap())
	container.Set("typeC4Shape", text(typeC4Shape))
	container.Set("parentBoundary", d.currentBoundaryParse)
	return nil
}

// addComponent is addComponent(typeC4Shape, alias, label, techn, descr,
// sprite, tags, link). It has the same body as addContainer upstream.
func (d *db) addComponent(typeC4Shape string, alias, label, techn, descr, sprite, tags, link any) error {
	// Don't allow label nulling
	if isNull(alias) || isNull(label) {
		return nil
	}

	component := d.shapeFor(alias)

	// Don't allow null labels, either
	setLabel(component, label)

	if err := setTextSlot(component, "techn", techn); err != nil {
		return err
	}
	if err := setTextSlot(component, "descr", descr); err != nil {
		return err
	}

	if err := assignAttributes(component, attrSlot{"sprite", sprite}, attrSlot{"tags", tags}, attrSlot{"link", link}); err != nil {
		return err
	}
	component.Set("wrap", d.autoWrap())
	component.Set("typeC4Shape", text(typeC4Shape))
	component.Set("parentBoundary", d.currentBoundaryParse)
	return nil
}

// enterBoundary ports the end of every boundary function: the new boundary
// becomes the current one.
func (d *db) enterBoundary(alias any) {
	d.parentBoundaryParse = d.currentBoundaryParse
	d.currentBoundaryParse = alias
	d.boundaryParseStack = append(d.boundaryParseStack, d.parentBoundaryParse)
}

// addPersonOrSystemBoundary is addPersonOrSystemBoundary(alias, label, type,
// tags, link).
func (d *db) addPersonOrSystemBoundary(alias, label, typ, tags, link any) error {
	// Don't allow label nulling
	if isNull(alias) || isNull(label) {
		return nil
	}

	boundary := d.boundaryFor(alias)

	// Don't allow null labels, either
	setLabel(boundary, label)

	if err := setTypeSlot(boundary, typ, "system"); err != nil {
		return err
	}

	if err := assignAttributes(boundary, attrSlot{"tags", tags}, attrSlot{"link", link}); err != nil {
		return err
	}
	boundary.Set("parentBoundary", d.currentBoundaryParse)
	boundary.Set("wrap", d.autoWrap())

	d.enterBoundary(alias)
	return nil
}

// addContainerBoundary is addContainerBoundary(alias, label, type, tags, link).
func (d *db) addContainerBoundary(alias, label, typ, tags, link any) error {
	// Don't allow label nulling
	if isNull(alias) || isNull(label) {
		return nil
	}

	boundary := d.boundaryFor(alias)

	// Don't allow null labels, either
	setLabel(boundary, label)

	if err := setTypeSlot(boundary, typ, "container"); err != nil {
		return err
	}

	if err := assignAttributes(boundary, attrSlot{"tags", tags}, attrSlot{"link", link}); err != nil {
		return err
	}
	boundary.Set("parentBoundary", d.currentBoundaryParse)
	boundary.Set("wrap", d.autoWrap())

	d.enterBoundary(alias)
	return nil
}

// addDeploymentNode is addDeploymentNode(nodeType, alias, label, type, descr,
// sprite, tags, link).
func (d *db) addDeploymentNode(nodeType string, alias, label, typ, descr, sprite, tags, link any) error {
	// Don't allow label nulling
	if isNull(alias) || isNull(label) {
		return nil
	}

	boundary := d.boundaryFor(alias)

	// Don't allow null labels, either
	setLabel(boundary, label)

	if err := setTypeSlot(boundary, typ, "node"); err != nil {
		return err
	}

	if err := setTextSlot(boundary, "descr", descr); err != nil {
		return err
	}

	// Upstream does not pass the sprite to assignAttributes here.
	_ = sprite
	if err := assignAttributes(boundary, attrSlot{"tags", tags}, attrSlot{"link", link}); err != nil {
		return err
	}
	boundary.Set("nodeType", nodeType)
	boundary.Set("parentBoundary", d.currentBoundaryParse)
	boundary.Set("wrap", d.autoWrap())

	d.enterBoundary(alias)
	return nil
}

// popBoundaryParseStack is popBoundaryParseStack().
func (d *db) popBoundaryParseStack() {
	d.currentBoundaryParse = d.parentBoundaryParse
	d.popStack()
	d.parentBoundaryParse = d.popStack()
	d.boundaryParseStack = append(d.boundaryParseStack, d.parentBoundaryParse)
}

// popStack is boundaryParseStack.pop(): undefined for an empty array.
func (d *db) popStack() any {
	n := len(d.boundaryParseStack)
	if n == 0 {
		return nil
	}
	v := d.boundaryParseStack[n-1]
	d.boundaryParseStack = d.boundaryParseStack[:n-1]
	return v
}

// setStyleSlot ports one block of updateElStyle and of updateRelStyle:
//
//	if (value !== undefined && value !== null) {
//	  if (typeof value === 'object') { old[key] = v } else { old[field] = value }
//	}
//
// conv converts the stored value (parseInt for the offsets).
func setStyleSlot(old *js.Obj[any], field string, value any, conv func(any) any) error {
	if js.IsNullish(value) {
		return nil
	}
	if named, ok := isObject(value); ok {
		key, val, ok := firstEntry(named)
		if !ok {
			return errNotIterable
		}
		if conv != nil {
			val = conv(val)
		}
		setKey(old, key, val)
		return nil
	}
	if conv != nil {
		value = conv(value)
	}
	old.Set(field, value)
	return nil
}

// updateElStyle is updateElStyle(typeC4Shape, elementName, bgColor,
// fontColor, borderColor, shadowing, shape, sprite, techn, legendText,
// legendSprite).
func (d *db) updateElStyle(_ string, elementName, bgColor, fontColor, borderColor, shadowing, shape, sprite, techn, legendText, legendSprite any) error {
	old := findByAlias(d.c4ShapeArray, elementName)
	if old == nil {
		old = findByAlias(d.boundaries, elementName)
		if old == nil {
			return nil
		}
	}
	for _, slot := range []attrSlot{
		{"bgColor", bgColor},
		{"fontColor", fontColor},
		{"borderColor", borderColor},
		{"shadowing", shadowing},
		{"shape", shape},
		{"sprite", sprite},
		// The parser hands over a plain string here, even though techn is a
		// `{ text: string }` object everywhere else.
		{"techn", techn},
		{"legendText", legendText},
		{"legendSprite", legendSprite},
	} {
		if err := setStyleSlot(old, slot.field, slot.value, nil); err != nil {
			return err
		}
	}
	return nil
}

// parseIntValue is the global parseInt(value).
func parseIntValue(value any) any {
	return js.ParseIntAuto(js.ToString(value))
}

// updateRelStyle is updateRelStyle(typeC4Shape, from, to, textColor,
// lineColor, offsetX, offsetY).
func (d *db) updateRelStyle(_ string, from, to, textColor, lineColor, offsetX, offsetY any) error {
	old := d.findRel(from, to)
	if old == nil {
		return nil
	}
	if err := setStyleSlot(old, "textColor", textColor, nil); err != nil {
		return err
	}
	if err := setStyleSlot(old, "lineColor", lineColor, nil); err != nil {
		return err
	}
	if err := setStyleSlot(old, "offsetX", offsetX, parseIntValue); err != nil {
		return err
	}
	return setStyleSlot(old, "offsetY", offsetY, parseIntValue)
}

// layoutValue ports the reading of one argument of updateLayoutConfig:
// `typeof param === 'object' ? parseInt(Object.values(param)[0]) : parseInt(param)`.
func layoutValue(param any) float64 {
	if named, ok := isObject(param); ok {
		_, val, _ := firstEntry(named)
		return js.ParseIntAuto(js.ToString(val))
	}
	return js.ParseIntAuto(js.ToString(param))
}

// updateLayoutConfig is updateLayoutConfig(typeC4Shape, c4ShapeInRowParam,
// c4BoundaryInRowParam).
func (d *db) updateLayoutConfig(_ string, c4ShapeInRowParam, c4BoundaryInRowParam any) {
	c4ShapeInRowValue := layoutValue(c4ShapeInRowParam)
	c4BoundaryInRowValue := layoutValue(c4BoundaryInRowParam)

	if c4ShapeInRowValue >= 1 {
		d.c4ShapeInRow = c4ShapeInRowValue
	}
	if c4BoundaryInRowValue >= 1 {
		d.c4BoundaryInRow = c4BoundaryInRowValue
	}
}

// getC4ShapeInRow is getC4ShapeInRow().
func (d *db) getC4ShapeInRow() float64 { return d.c4ShapeInRow }

// getC4BoundaryInRow is getC4BoundaryInRow().
func (d *db) getC4BoundaryInRow() float64 { return d.c4BoundaryInRow }

// getCurrentBoundaryParse is getCurrentBoundaryParse().
func (d *db) getCurrentBoundaryParse() any { return d.currentBoundaryParse }

// getParentBoundaryParse is getParentBoundaryParse().
func (d *db) getParentBoundaryParse() any { return d.parentBoundaryParse }

// getC4ShapeArray is getC4ShapeArray() with no argument.
func (d *db) getC4ShapeArray() []*js.Obj[any] { return d.c4ShapeArray }

// getC4ShapeArrayOf is getC4ShapeArray(parentBoundary). It also stands for
// getC4ShapeKeys(parentBoundary), which returns the indexes of this array.
func (d *db) getC4ShapeArrayOf(parentBoundary any) []*js.Obj[any] {
	var out []*js.Obj[any]
	for _, personOrSystem := range d.c4ShapeArray {
		if js.StrictEquals(personOrSystem.Get("parentBoundary"), parentBoundary) {
			out = append(out, personOrSystem)
		}
	}
	return out
}

// getC4Shape is getC4Shape(alias). Relations can target a boundary as well
// as a shape, so it falls back to the boundaries.
func (d *db) getC4Shape(alias any) *js.Obj[any] {
	if shape := findByAlias(d.c4ShapeArray, alias); shape != nil {
		return shape
	}
	return findByAlias(d.boundaries, alias)
}

// getBoundaries is getBoundaries() with no argument.
func (d *db) getBoundaries() []*js.Obj[any] { return d.boundaries }

// getBoundariesOf is getBoundaries(parentBoundary).
func (d *db) getBoundariesOf(parentBoundary any) []*js.Obj[any] {
	var out []*js.Obj[any]
	for _, boundary := range d.boundaries {
		if js.StrictEquals(boundary.Get("parentBoundary"), parentBoundary) {
			out = append(out, boundary)
		}
	}
	return out
}

// getRels is getRels().
func (d *db) getRels() []*js.Obj[any] { return d.rels }

// getTitle is getTitle().
func (d *db) getTitle() string { return d.title }

// setWrap is setWrap(wrapSetting).
func (d *db) setWrap(wrapSetting any) { d.wrapEnabled = wrapSetting }

// autoWrap is autoWrap().
func (d *db) autoWrap() any { return d.wrapEnabled }

// clear is clear().
func (d *db) clear() {
	d.c4ShapeArray = nil
	d.boundaries = []*js.Obj[any]{createGlobalBoundary()}
	d.parentBoundaryParse = ""
	d.currentBoundaryParse = "global"
	d.boundaryParseStack = []any{""}
	d.rels = nil

	d.title = ""
	d.wrapEnabled = false
	d.c4ShapeInRow = 4
	d.c4BoundaryInRow = 2
}

// setTitle is setTitle(txt).
func (d *db) setTitle(txt string) {
	d.title = d.sanitizeText(txt)
}

// isNull is `value === null`.
func isNull(value any) bool {
	return value != nil && js.IsNullish(value)
}
