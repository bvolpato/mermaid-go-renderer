// Ported from mermaid 12.2.0, diagrams/state/dataFetcher.ts (MIT).

package state

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// colorSlot is an entry of containerColorIndex: a palette slot, or
// undefined for a container that the author has styled.
type colorSlot struct {
	slot    float64
	defined bool
}

// value is the JavaScript value of the slot: a number or undefined.
func (c colorSlot) value() any {
	if !c.defined {
		return nil
	}
	return c.slot
}

// fetcherState is the module-level state of dataFetcher.ts.
type fetcherState struct {
	// nodeDb is the list of nodes created from the parsed diagram statement
	// items.
	nodeDb map[string]*js.Obj[any]
	// graphItemCount is used to construct ids, etc.
	graphItemCount int
	// nextColorIndex is the next palette slot to hand out, and
	// containerColorIndex is the slot each container ended up with.
	nextColorIndex      int
	containerColorIndex map[string]colorSlot
}

// reset is reset().
func (f *fetcherState) reset() {
	f.nodeDb = map[string]*js.Obj[any]{}
	f.graphItemCount = 0
	f.nextColorIndex = 0
	f.containerColorIndex = map[string]colorSlot{}
}

// colorSlotFor is colorSlotFor(shape, itemId, parent, userStyled): the
// palette slot for a container.
//
// dataFetcher recurses depth-first and takes a slot as it inserts each
// container, so the numbering is a pre-order walk of the containment tree.
// Concurrency regions are the exception: they reuse the slot of their
// parent rather than taking one. A container that the author has styled
// resolves to undefined, and its regions inherit that.
func (d *DB) colorSlotFor(shape string, itemID string, parentItem *js.Obj[any], userStyled bool) any {
	f := &d.fetch
	// `has`, not a truthy check: a styled parent records `undefined`
	// deliberately, and that is exactly the value its regions have to
	// inherit.
	if shape == shapeDivider && parentItem != nil && parentItem.Get("id") != nil {
		if inherited, ok := f.containerColorIndex[js.ToString(parentItem.Get("id"))]; ok {
			f.containerColorIndex[itemID] = inherited
			return inherited.value()
		}
	}
	// Everything else takes the next slot. A `--` at the top level lands
	// here too: there is no composite to belong to, so it is its own
	// container.
	slot := f.nextColorIndex
	f.nextColorIndex++
	effective := colorSlot{slot: float64(slot), defined: !userStyled}
	f.containerColorIndex[itemID] = effective
	return effective.value()
}

// stateDomID is stateDomId(itemId, counter, type, typeSpacer): a standard
// string for the dom ID of an item. If a type is given, it is inserted
// before the counter, preceded by the type spacer.
func stateDomID(itemID string, counter int, typ string) string {
	typeStr := ""
	if len(typ) > 0 {
		typeStr = domIDTypeSpacer + typ
	}
	return domIDState + "-" + itemID + typeStr + "-" + strconv.Itoa(counter)
}

// setupDoc is setupDoc(parentParsedItem, doc, diagramStates, nodes, edges,
// altFlag, look, classes). nodes, edges and classes are the fields of the
// db.
func (d *DB) setupDoc(parentParsedItem *js.Obj[any], doc []any, diagramStates *js.Map[string, *State], altFlag bool, look any) error {
	for _, item := range doc {
		stmt, _ := strProp(item, "stmt")
		switch stmt {
		case stmtState, defaultStateType:
			if err := d.dataFetcher(parentParsedItem, item.(*js.Obj[any]), diagramStates, altFlag, look); err != nil {
				return err
			}
		case stmtRelation:
			state1, ok1 := prop(item, "state1").(*js.Obj[any])
			state2, ok2 := prop(item, "state2").(*js.Obj[any])
			if !ok1 || !ok2 {
				return typeError("Cannot read properties of undefined (reading 'id')")
			}
			if err := d.dataFetcher(parentParsedItem, state1, diagramStates, altFlag, look); err != nil {
				return err
			}
			if err := d.dataFetcher(parentParsedItem, state2, diagramStates, altFlag, look); err != nil {
				return err
			}
			isNeo := js.StrictEquals(look, "neo")
			arrowTypeEnd := "arrow_barb"
			if isNeo {
				arrowTypeEnd = "arrow_barb_neo"
			}
			// item.description ?? ''
			description := ""
			if v := prop(item, "description"); !js.IsNullish(v) {
				description = js.ToString(v)
			}
			label, err := d.sanitizeText(description)
			if err != nil {
				return err
			}
			edgeData := newObj(
				"id", "edge"+strconv.Itoa(d.fetch.graphItemCount),
				"start", state1.Get("id"),
				"end", state2.Get("id"),
				"arrowhead", "normal",
				"arrowTypeEnd", arrowTypeEnd,
				"style", gEdgeStyle,
				"labelStyle", "",
				"label", label,
				"arrowheadStyle", gEdgeArrowheadStyle,
				"labelpos", gEdgeLabelpos,
				"labelType", gEdgeLabelType,
				"thickness", gEdgeThickness,
				"classes", cssEdge,
				"look", look,
			)
			d.edges = append(d.edges, edgeData)
			d.fetch.graphItemCount++
		}
	}
	return nil
}

// getDir is getDir(parsedItem, defaultDir): the direction from the
// statement items. It looks through all of the documents (docs) in the
// parsedItems. Because it is a _document_ direction, the default direction
// is not necessarily the same as the overall default _diagram_ direction.
// dataFetcher.ts and stateRenderer-v3-unified.ts have this function each.
func getDir(parsedItem *js.Obj[any], defaultDir string) any {
	var dir any = defaultDir
	if doc, ok := parsedItem.Get("doc").([]any); ok {
		for _, parsedItemDoc := range doc {
			if stmt, _ := strProp(parsedItemDoc, "stmt"); stmt == "dir" {
				dir = prop(parsedItemDoc, "value")
			}
		}
	}
	return dir
}

// insertOrUpdateNode is insertOrUpdateNode(nodes, nodeData, classes).
func (d *DB) insertOrUpdateNode(nodeData *js.Obj[any]) {
	id := nodeData.Get("id")
	if !js.Truthy(id) || id == "</join></fork>" || id == "</choice>" {
		return
	}

	// Populate node style attributes if nodeData has classes defined
	if cssClasses := nodeData.Get("cssClasses"); js.Truthy(cssClasses) {
		if _, ok := nodeData.Get("cssCompiledStyles").([]any); !ok {
			nodeData.Set("cssCompiledStyles", []any{})
		}

		for _, cssClass := range strings.Split(js.ToString(cssClasses), " ") {
			classDef := d.classes.Get(cssClass)
			if classDef != nil {
				// [...(nodeData.cssCompiledStyles ?? []), ...classDef.styles]
				compiled, _ := nodeData.Get("cssCompiledStyles").([]any)
				list := make([]any, 0, len(compiled)+len(classDef.Styles))
				list = append(list, compiled...)
				list = append(list, anyStrings(classDef.Styles)...)
				nodeData.Set("cssCompiledStyles", list)
			}
		}
	}
	var existingNodeData *js.Obj[any]
	for _, node := range d.nodes {
		if js.StrictEquals(node.Get("id"), id) {
			existingNodeData = node
			break
		}
	}
	if existingNodeData != nil {
		// update the existing nodeData: Object.assign(existingNodeData, nodeData)
		nodeData.Each(func(k string, v any) { existingNodeData.Set(k, v) })
	} else {
		d.nodes = append(d.nodes, nodeData)
	}
}

// sanitizeTextOrArray is common.sanitizeTextOrArray(a, config): a string is
// sanitized, and an array is flattened by one level and each element is
// sanitized.
func (d *DB) sanitizeTextOrArray(a any) (any, error) {
	if s, ok := a.(string); ok {
		return d.sanitizeText(s)
	}
	list, ok := a.([]any)
	if !ok {
		return nil, typeError("a.flat is not a function")
	}
	// a.flat()
	flat := make([]any, 0, len(list))
	for _, e := range list {
		if inner, ok := e.([]any); ok {
			flat = append(flat, inner...)
		} else {
			flat = append(flat, e)
		}
	}
	out := make([]any, len(flat))
	for i, x := range flat {
		s, ok := x.(string)
		if !ok {
			// sanitizeText returns a value that is falsy as it is.
			if !js.Truthy(x) {
				out[i] = x
				continue
			}
			return nil, typeError("text.replace is not a function")
		}
		sanitized, err := d.sanitizeText(s)
		if err != nil {
			return nil, err
		}
		out[i] = sanitized
	}
	return out, nil
}

// dataFetcher is dataFetcher(parent, parsedItem, diagramStates, nodes,
// edges, altFlag, look, classes). nodes, edges and classes are the fields
// of the db. parentItem is nil when the parent is undefined.
func (d *DB) dataFetcher(parentItem, parsedItem *js.Obj[any], diagramStates *js.Map[string, *State], altFlag bool, look any) error {
	f := &d.fetch
	itemID, _ := parsedItem.Get("id").(string)
	dbState := diagramStates.Get(itemID)
	// getClassesFromDbInfo(dbState) and getStylesFromDbInfo(dbState)
	classStr := ""
	style := []string{}
	if dbState != nil {
		classStr = strings.Join(dbState.Classes, " ")
		if dbState.Styles != nil {
			style = dbState.Styles
		}
	}
	config := d.config()

	// Whether the author has styled this state themselves, via
	// `classDef`/`class` or a `style` statement. Such a container opts out
	// of the palette entirely.
	userStyled := js.TrimSpace(classStr) != "" || len(style) > 0

	if itemID != "root" {
		var shape any = shapeState
		// The if === true / false can be removed if we can guarantee that
		// the parsedItem.start is always a boolean
		if start, ok := parsedItem.Get("start").(bool); ok {
			if start {
				shape = shapeStart
			} else {
				shape = shapeEnd
			}
		}
		if typ := parsedItem.Get("type"); !js.StrictEquals(typ, defaultStateType) {
			shape = typ
		}

		// Add the node to our list (nodeDb)
		if f.nodeDb[itemID] == nil {
			description, err := d.sanitizeText(itemID)
			if err != nil {
				return err
			}
			f.nodeDb[itemID] = newObj(
				"id", itemID,
				"shape", shape,
				"description", description,
				"cssClasses", classStr+" "+cssDiagramState,
				"cssStyles", anyStrings(style),
			)
		}

		newNode := f.nodeDb[itemID]

		// Save data for description and group so that for instance a
		// statement without description overwrites one with description

		// Build of the array of description strings
		if parsedDescription := parsedItem.Get("description"); js.Truthy(parsedDescription) {
			if list, ok := newNode.Get("description").([]any); ok {
				// There already is an array of strings,add to it
				newNode.Set("shape", shapeStateWithDesc)
				newNode.Set("description", append(list, parsedDescription))
			} else {
				current := newNode.Get("description")
				if n, ok := lengthOf(current); ok && n > 0 {
					// if there is a description already transform it to an array
					newNode.Set("shape", shapeStateWithDesc)
					if js.StrictEquals(current, itemID) {
						// If the previous description was this, remove it
						newNode.Set("description", []any{parsedDescription})
					} else {
						newNode.Set("description", []any{current, parsedDescription})
					}
				} else {
					newNode.Set("shape", shapeState)
					newNode.Set("description", parsedDescription)
				}
			}
			sanitized, err := d.sanitizeTextOrArray(newNode.Get("description"))
			if err != nil {
				return err
			}
			newNode.Set("description", sanitized)
		}

		// If there's only 1 description entry, just use a regular state shape
		if n, ok := lengthOf(newNode.Get("description")); ok && n == 1 && js.StrictEquals(newNode.Get("shape"), shapeStateWithDesc) {
			if js.StrictEquals(newNode.Get("type"), "group") {
				newNode.Set("shape", shapeGroup)
			} else {
				newNode.Set("shape", shapeState)
			}
		}

		// group
		if !js.Truthy(newNode.Get("type")) && js.Truthy(parsedItem.Get("doc")) {
			newNode.Set("type", "group")
			newNode.Set("isGroup", true)
			newNode.Set("dir", getDir(parsedItem, defaultNestedDocDir))
			groupShape := shapeGroup
			if js.StrictEquals(parsedItem.Get("type"), dividerType) {
				groupShape = shapeDivider
			}
			newNode.Set("shape", groupShape)
			// A styled container still spends its slot, so giving one
			// composite a `classDef` does not shift the colour of every
			// composite after it; it simply resolves to `undefined` and
			// goes unstamped. See colorSlotFor.
			newNode.Set("colorIndex", d.colorSlotFor(groupShape, itemID, parentItem, userStyled))
			alt := ""
			if altFlag {
				alt = cssDiagramClusterAlt
			}
			newNode.Set("cssClasses", js.ToString(newNode.Get("cssClasses"))+" "+cssDiagramCluster+" "+alt)
		}

		// This is what will be added to the graph
		nodeData := newObj(
			"labelStyle", "",
			"shape", newNode.Get("shape"),
			"label", newNode.Get("description"),
			"cssClasses", newNode.Get("cssClasses"),
			"cssCompiledStyles", []any{},
			"cssStyles", newNode.Get("cssStyles"),
			"id", itemID,
			"dir", newNode.Get("dir"),
			"domId", stateDomID(itemID, f.graphItemCount, ""),
			"type", newNode.Get("type"),
			"isGroup", js.StrictEquals(newNode.Get("type"), "group"),
			"colorIndex", newNode.Get("colorIndex"),
			"padding", 8.0,
			"rx", 10.0,
			"ry", 10.0,
			"look", look,
			"labelType", "markdown",
		)

		// Clear the label for dividers who have no description
		if js.StrictEquals(nodeData.Get("shape"), shapeDivider) {
			nodeData.Set("label", "")
		}

		if parentItem != nil && !js.StrictEquals(parentItem.Get("id"), "root") {
			nodeData.Set("parentId", parentItem.Get("id"))
		}

		nodeData.Set("centerLabel", true)

		if noteItem := parsedItem.Get("note"); js.Truthy(noteItem) {
			noteText := prop(noteItem, "text")
			notePosition := prop(noteItem, "position")
			noteData := newObj(
				"labelStyle", "",
				"shape", shapeNote,
				"label", noteText,
				"labelType", "markdown",
				"cssClasses", cssDiagramNote,
				"cssStyles", []any{},
				"cssCompiledStyles", []any{},
				"id", itemID+noteID+"-"+strconv.Itoa(f.graphItemCount),
				"domId", stateDomID(itemID, f.graphItemCount, noteName),
				// A note is a leaf: it is placed inside the note group
				// below, alongside the state it annotates.
				"type", "node",
				"isGroup", false,
				"padding", js.Path(config, "flowchart", "padding"),
				"look", look,
				"position", notePosition,
			)
			parentNodeID := itemID + parentID
			groupData := newObj(
				"labelStyle", "",
				"shape", shapeNoteGroup,
				"label", noteText,
				"cssClasses", newNode.Get("cssClasses"),
				"cssStyles", []any{},
				"id", itemID+parentID,
				"domId", stateDomID(itemID, f.graphItemCount, parentName),
				"type", "group",
				"isGroup", true,
				"padding", 16.0,
				"look", look,
				"position", notePosition,
			)
			f.graphItemCount++

			// add parent id to groupData
			groupData.Set("id", parentNodeID)
			// add parent id to noteData
			noteData.Set("parentId", parentNodeID)

			// insert groupData
			d.insertOrUpdateNode(groupData)
			// insert noteData
			d.insertOrUpdateNode(noteData)
			// insert nodeData
			d.insertOrUpdateNode(nodeData)

			from := itemID
			to := noteData.Get("id").(string)

			if js.StrictEquals(notePosition, "left of") {
				from = to
				to = itemID
			}

			d.edges = append(d.edges, newObj(
				"id", from+"-"+to,
				"start", from,
				"end", to,
				"arrowhead", "none",
				"arrowTypeEnd", "",
				"style", gEdgeStyle,
				"labelStyle", "",
				"classes", cssEdgeNoteEdge,
				// The dashes have to be declared on the edge, not only
				// through the `note-edge` class. Under `look: neo`,
				// insertEdge writes an inline `stroke-dasharray`, and it
				// picks that pattern from `edge.pattern`.
				"pattern", "dashed",
				"arrowheadStyle", gEdgeArrowheadStyle,
				"labelpos", gEdgeLabelpos,
				"labelType", gEdgeLabelType,
				"thickness", gEdgeThickness,
				"look", look,
			))
		} else {
			d.insertOrUpdateNode(nodeData)
		}
	}
	if doc := parsedItem.Get("doc"); js.Truthy(doc) {
		list, ok := doc.([]any)
		if !ok {
			return typeError("doc.forEach is not a function")
		}
		return d.setupDoc(parsedItem, list, diagramStates, !altFlag, look)
	}
	return nil
}
