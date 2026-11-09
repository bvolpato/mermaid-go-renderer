// Ported from mermaid 12.2.0, diagrams/class/classDb.ts (MIT).

package class

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

const mermaidDomIDPrefix = "classId-"

// classCounterName is the name of the counter that stands for the
// module-level classCounter of classDb.ts. Upstream never resets it, so it
// grows for as long as the page lives. A render has its own counter.
const classCounterName = "class.classCounter"

// ClickCallback is one click handler that bindFunctions would install.
type ClickCallback struct {
	ID           string
	FunctionName string
	Args         []string
}

// DB is ClassDB: the data of a parsed class diagram.
type DB struct {
	render.CommonDB
	ctx *render.Context

	relations        []*ClassRelation
	classes          *js.Map[string, *ClassNode]
	styleClasses     *js.Map[string, *StyleClass]
	notes            *js.Map[string, *ClassNote]
	interfaces       []*Interface
	namespaces       *js.Map[string, *NamespaceNode]
	namespaceCounter int
	namespaceStack   []string
	diagramID        string
	direction        string

	// callbacks stands for the functions that upstream runs after the
	// render to add the click listeners (functions, without the first one,
	// which sets up the tooltips).
	callbacks []ClickCallback

	// err is the first exception that a db function threw. The parser
	// stops with it after the action that called the function.
	err error
}

// NewDB returns a db in the state after the constructor of ClassDB.
func NewDB(ctx *render.Context) *DB {
	d := &DB{ctx: ctx, styleClasses: js.NewMap[string, *StyleClass]()}
	d.CommonDB.Sanitize = d.sanitizeText
	d.Clear()
	return d
}

// fail records an exception of a db function.
func (d *DB) fail(err error) {
	if d.err == nil {
		d.err = err
	}
}

// getConfig is getConfig() of diagramAPI. The pipeline keeps the current
// config in the context.
func (d *DB) getConfig() *js.Obj[any] {
	return d.ctx.Config
}

// sanitizeText is sanitizeText(txt): common.sanitizeText(txt, getConfig()).
func (d *DB) sanitizeText(txt string) string {
	out, err := d.ctx.Sanitizer.SanitizeTextErr(txt, d.getConfig())
	if err != nil {
		d.fail(err)
		return ""
	}
	return out
}

// splitClassNameAndType is splitClassNameAndType(_id).
func (d *DB) splitClassNameAndType(_id string) (className, genericType string) {
	id := d.sanitizeText(_id)
	genericType = ""
	className = id

	if strings.Index(id, "~") > 0 {
		split := strings.Split(id, "~")
		className = d.sanitizeText(split[0])
		genericType = d.sanitizeText(split[1])
	}

	return className, genericType
}

// SetClassLabel is setClassLabel(_id, label).
func (d *DB) SetClassLabel(_id, label string) {
	id := d.sanitizeText(_id)
	if label != "" {
		label = d.sanitizeText(label)
	}

	className, _ := d.splitClassNameAndType(id)
	c, ok := d.classes.Lookup(className)
	if !ok {
		d.fail(undefinedSet("label"))
		return
	}
	c.Label = label
	c.Text = label
	if c.Type != "" {
		c.Text = label + "<" + c.Type + ">"
	}
}

// AddClass is addClass(_id): the parser calls it when it finds a class.
func (d *DB) AddClass(_id string) {
	id := d.sanitizeText(_id)
	className, typ := d.splitClassNameAndType(id)
	// Only add class if not exists
	if d.classes.Has(className) {
		return
	}
	name := d.sanitizeText(className)
	text := name
	if typ != "" {
		text = name + "&lt;" + typ + "&gt;"
	}
	d.classes.Set(name, &ClassNode{
		ID:          name,
		Type:        typ,
		Label:       name,
		Text:        text,
		Shape:       "classBox",
		CSSClasses:  "default",
		Methods:     []*ClassMember{},
		Members:     []*ClassMember{},
		Annotations: []string{},
		Styles:      []string{},
		DomID:       mermaidDomIDPrefix + name + "-" + strconv.Itoa(d.ctx.Counter(classCounterName)),
	})
}

// addInterface is addInterface(label, classId).
func (d *DB) addInterface(label, classID string) {
	d.interfaces = append(d.interfaces, &Interface{
		ID:      "interface" + strconv.Itoa(len(d.interfaces)),
		Label:   label,
		ClassID: classID,
	})
}

// SetDiagramId is setDiagramId(svgElementId): the id of the svg element is
// the prefix of the DOM ids, so that they are unique across the diagrams of
// a page.
func (d *DB) SetDiagramId(svgElementID string) {
	d.diagramID = svgElementID
}

// LookUpDomId is lookUpDomId(_id): the DOM id of a class, with the prefix
// of the diagram when it is set. The error is the error that upstream
// throws for an unknown class.
func (d *DB) LookUpDomId(_id string) (string, error) {
	id := d.sanitizeText(_id)
	if c, ok := d.classes.Lookup(id); ok {
		if d.diagramID != "" {
			return d.diagramID + "-" + c.DomID, nil
		}
		return c.DomID, nil
	}
	return "", &Error{Message: "Class not found: " + id}
}

// Error is a JavaScript Error that the db throws.
type Error struct {
	Message string
}

func (e *Error) Error() string { return e.Message }

// Clear is clear().
func (d *DB) Clear() {
	d.relations = []*ClassRelation{}
	d.classes = js.NewMap[string, *ClassNode]()
	d.notes = js.NewMap[string, *ClassNote]()
	d.interfaces = []*Interface{}
	d.callbacks = nil
	d.namespaces = js.NewMap[string, *NamespaceNode]()
	d.namespaceCounter = 0
	d.namespaceStack = nil
	d.diagramID = ""
	d.direction = "TB"
	d.ClearCommon()
}

// GetClass is getClass(id). The result is nil for an unknown class.
func (d *DB) GetClass(id string) *ClassNode {
	return d.classes.Get(id)
}

// GetClasses is getClasses().
func (d *DB) GetClasses() *js.Map[string, *ClassNode] {
	return d.classes
}

// GetRelations is getRelations().
func (d *DB) GetRelations() []*ClassRelation {
	return d.relations
}

// GetNote is getNote(id) for a string id. The result is nil for an unknown
// note.
func (d *DB) GetNote(id string) *ClassNote {
	return d.notes.Get(id)
}

// GetNotes is getNotes().
func (d *DB) GetNotes() *js.Map[string, *ClassNote] {
	return d.notes
}

// GetInterfaces returns the interfaces: the ends of the lollipop relations.
func (d *DB) GetInterfaces() []*Interface {
	return d.interfaces
}

// GetStyleClasses returns the style classes: the classDef statements.
func (d *DB) GetStyleClasses() *js.Map[string, *StyleClass] {
	return d.styleClasses
}

// isInvalidType reports whether a relation type is one of invalidTypes.
func isInvalidType(t int) bool {
	switch t {
	case relLollipop, relAggregation, relComposition, relDependency, relExtension:
		return true
	}
	return false
}

// AddRelation is addRelation(classRelation).
func (d *DB) AddRelation(classRelation *ClassRelation) {
	// Due to relationType cannot just check if it is equal to 'none' or it complains, can fix this later
	if classRelation.Relation.Type1 == relLollipop && !isInvalidType(classRelation.Relation.Type2) {
		d.AddClass(classRelation.ID2)
		d.addInterface(classRelation.ID1, classRelation.ID2)
		classRelation.ID1 = "interface" + strconv.Itoa(len(d.interfaces)-1)
	} else if classRelation.Relation.Type2 == relLollipop && !isInvalidType(classRelation.Relation.Type1) {
		d.AddClass(classRelation.ID1)
		d.addInterface(classRelation.ID2, classRelation.ID1)
		classRelation.ID2 = "interface" + strconv.Itoa(len(d.interfaces)-1)
	} else {
		d.AddClass(classRelation.ID1)
		d.AddClass(classRelation.ID2)
	}

	classRelation.ID1, _ = d.splitClassNameAndType(classRelation.ID1)
	classRelation.ID2, _ = d.splitClassNameAndType(classRelation.ID2)

	classRelation.RelationTitle1 = d.sanitizeText(js.TrimSpace(classRelation.RelationTitle1))

	classRelation.RelationTitle2 = d.sanitizeText(js.TrimSpace(classRelation.RelationTitle2))

	d.relations = append(d.relations, classRelation)
}

// AddAnnotation is addAnnotation(className, annotation). Annotations mark
// special properties of the class, like 'interface' or 'service'.
func (d *DB) AddAnnotation(className, annotation string) {
	validatedClassName, _ := d.splitClassNameAndType(className)
	c, ok := d.classes.Lookup(validatedClassName)
	if !ok {
		d.fail(undefinedRead("annotations"))
		return
	}
	c.Annotations = append(c.Annotations, annotation)
}

// AddMember is addMember(className, member). A member in `<<brackets>>` is
// an annotation, a member with a closing bracket ) is a method, and other
// members are attributes.
func (d *DB) AddMember(className, member string) {
	d.AddClass(className)

	validatedClassName, _ := d.splitClassNameAndType(className)
	theClass, ok := d.classes.Lookup(validatedClassName)

	// Member can contain white spaces, we trim them out
	memberString := js.TrimSpace(member)

	if strings.HasPrefix(memberString, "<<") && strings.HasSuffix(memberString, ">>") {
		// its an annotation
		if !ok {
			d.fail(undefinedRead("annotations"))
			return
		}
		// A member that starts with << and ends with >> has 4 bytes or more.
		theClass.Annotations = append(theClass.Annotations, d.sanitizeText(memberString[2:len(memberString)-2]))
	} else if strings.Index(memberString, ")") > 0 {
		//its a method
		if !ok {
			d.fail(undefinedRead("methods"))
			return
		}
		m, err := NewClassMember(memberString, "method", d.sanitizeText)
		if err != nil {
			d.fail(err)
			return
		}
		theClass.Methods = append(theClass.Methods, m)
	} else if memberString != "" {
		if !ok {
			d.fail(undefinedRead("members"))
			return
		}
		m, err := NewClassMember(memberString, "attribute", d.sanitizeText)
		if err != nil {
			d.fail(err)
			return
		}
		theClass.Members = append(theClass.Members, m)
	}
}

// AddMembers is addMembers(className, members). It reverses the members in
// place, as upstream does.
func (d *DB) AddMembers(className string, members []string) {
	for i, j := 0, len(members)-1; i < j; i, j = i+1, j-1 {
		members[i], members[j] = members[j], members[i]
	}
	for _, member := range members {
		d.AddMember(className, member)
		if d.err != nil {
			return
		}
	}
}

// AddNote is addNote(text, className). hasClass is false when the note has
// no class.
func (d *DB) AddNote(text, className string, hasClass bool) string {
	index := d.notes.Len()
	note := &ClassNote{
		ID:       "note" + strconv.Itoa(index),
		Class:    className,
		HasClass: hasClass,
		Text:     text,
		Index:    index,
	}
	d.notes.Set(note.ID, note)
	return note.ID
}

// CleanupLabel is cleanupLabel(label).
func (d *DB) CleanupLabel(label string) string {
	if strings.HasPrefix(label, ":") {
		label = label[1:]
	}
	return d.sanitizeText(js.TrimSpace(label))
}

// startsWithDigit is /\d/.exec(_id[0]).
func startsWithDigit(id string) bool {
	return id != "" && id[0] >= '0' && id[0] <= '9'
}

// SetCssClass is setCssClass(ids, className): the parser calls it when it
// assigns a CSS class to a class. ids is a comma separated list.
func (d *DB) SetCssClass(ids, className string) {
	for _, _id := range strings.Split(ids, ",") {
		id := _id
		if startsWithDigit(_id) {
			id = mermaidDomIDPrefix + id
		}
		id, _ = d.splitClassNameAndType(id)
		if classNode, ok := d.classes.Lookup(id); ok {
			classNode.CSSClasses += " " + className
		}
	}
}

// DefineClass is defineClass(ids, style).
func (d *DB) DefineClass(ids []string, style []string) {
	for _, id := range ids {
		styleClass, ok := d.styleClasses.Lookup(id)
		if !ok {
			styleClass = &StyleClass{ID: id, Styles: []string{}, TextStyles: []string{}}
			d.styleClasses.Set(id, styleClass)
		}

		for _, s := range style {
			if strings.Contains(s, "color") {
				newStyle := strings.Replace(s, "fill", "bgFill", 1) // .replace('color', 'fill');
				styleClass.TextStyles = append(styleClass.TextStyles, newStyle)
			}
			styleClass.Styles = append(styleClass.Styles, s)
		}

		d.classes.Each(func(_ string, value *ClassNode) {
			if strings.Contains(value.CSSClasses, id) {
				for _, s := range style {
					value.Styles = append(value.Styles, strings.Split(s, ",")...)
				}
			}
		})
	}
}

// SetTooltip is setTooltip(ids, tooltip): the parser calls it when it finds
// a tooltip. The parser always gives a tooltip.
func (d *DB) SetTooltip(ids, tooltip string) {
	for _, id := range strings.Split(ids, ",") {
		className, _ := d.splitClassNameAndType(id)
		if classNode, ok := d.classes.Lookup(className); ok {
			classNode.Tooltip, classNode.HasTooltip = d.sanitizeText(tooltip), true
		}
	}
}

// GetTooltip is getTooltip(id) without a namespace. The second result is
// false when the class has no tooltip, and the error is the TypeError of
// upstream for an unknown class.
func (d *DB) GetTooltip(id string) (string, bool, error) {
	c, ok := d.classes.Lookup(id)
	if !ok {
		return "", false, undefinedRead("tooltip")
	}
	return c.Tooltip, c.HasTooltip, nil
}

// SetLink is setLink(ids, linkStr, target): the parser calls it when it
// finds a link. target is nil when it is undefined.
func (d *DB) SetLink(ids, linkStr string, target *string) {
	config := d.getConfig()
	for _, _id := range strings.Split(ids, ",") {
		id := _id
		if startsWithDigit(_id) {
			id = mermaidDomIDPrefix + id
		}
		id, _ = d.splitClassNameAndType(id)
		if theClass, ok := d.classes.Lookup(id); ok {
			theClass.Link, theClass.LinkDefined = utils.FormatUrl(linkStr, config)
			theClass.HasLinkTarget = true
			if js.StrictEquals(config.Get("securityLevel"), "sandbox") {
				theClass.LinkTarget = "_top"
			} else if target != nil {
				theClass.LinkTarget = d.sanitizeText(*target)
			} else {
				theClass.LinkTarget = "_blank"
			}
		}
	}
	d.SetCssClass(ids, "clickable")
}

// SetClickEvent is setClickEvent(ids, functionName, functionArgs): the
// parser calls it when it finds a click definition. functionArgs is nil
// when it is undefined.
func (d *DB) SetClickEvent(ids, functionName string, functionArgs *string) {
	for _, id := range strings.Split(ids, ",") {
		d.setClickFunc(id, functionName, functionArgs)
		className, _ := d.splitClassNameAndType(id)
		if classNode, ok := d.classes.Lookup(className); ok {
			classNode.HaveCallback = true
		}
	}
	d.SetCssClass(ids, "clickable")
}

// splitArgs is functionArgs.split(/,(?=(?:(?:[^"]*"){2})*[^"]*$)/): it
// splits at each "," that an even number of double quotes follows.
func splitArgs(functionArgs string) []string {
	quotesAfter := make([]int, len(functionArgs)+1)
	for i := len(functionArgs) - 1; i >= 0; i-- {
		quotesAfter[i] = quotesAfter[i+1]
		if functionArgs[i] == '"' {
			quotesAfter[i]++
		}
	}
	var out []string
	start := 0
	for i := 0; i < len(functionArgs); i++ {
		if functionArgs[i] == ',' && quotesAfter[i+1]%2 == 0 {
			out = append(out, functionArgs[start:i])
			start = i + 1
		}
	}
	return append(out, functionArgs[start:])
}

// setClickFunc is setClickFunc(_domId, functionName, functionArgs).
func (d *DB) setClickFunc(_domID, functionName string, functionArgs *string) {
	domID := d.sanitizeText(_domID)
	config := d.getConfig()
	if !js.StrictEquals(config.Get("securityLevel"), "loose") {
		return
	}

	id, _ := d.splitClassNameAndType(domID)
	if d.classes.Has(id) {
		var argList []string
		if functionArgs != nil {
			/* Splits functionArgs by ',', ignoring all ',' in double quoted strings */
			argList = splitArgs(*functionArgs)
			for i := range argList {
				item := js.TrimSpace(argList[i])
				/* Removes all double quotes at the start and end of an argument */
				/* This preserves all starting and ending whitespace inside */
				if strings.HasPrefix(item, `"`) && strings.HasSuffix(item, `"`) {
					item = js.Substr(item, 1, js.UTF16Len(item)-2)
				}
				argList[i] = item
			}
		}

		/* if no arguments passed into callback, default to passing in id */
		if len(argList) == 0 {
			argList = append(argList, id)
		}

		d.callbacks = append(d.callbacks, ClickCallback{ID: id, FunctionName: functionName, Args: argList})
	}
}

// Callbacks returns the click callbacks that bindFunctions would install.
// A static SVG has no event listeners, so this port only records them.
func (d *DB) Callbacks() []ClickCallback {
	return d.callbacks
}

// GetDirection is getDirection().
func (d *DB) GetDirection() string {
	return d.direction
}

// SetDirection is setDirection(dir).
func (d *DB) SetDirection(dir string) {
	d.direction = dir
}

// resolveQualifiedId is ClassDB.resolveQualifiedId(id, stack).
func resolveQualifiedId(id string, stack []string) string {
	if len(stack) > 0 {
		if prefix := stack[len(stack)-1]; prefix != "" {
			return prefix + "." + id
		}
	}
	return id
}

// getAncestorIds is ClassDB.getAncestorIds(qualifiedId).
func getAncestorIds(qualifiedID string) []string {
	parts := strings.Split(qualifiedID, ".")
	ids := make([]string, len(parts))
	ids[0] = parts[0]
	for i := 1; i < len(parts); i++ {
		ids[i] = ids[i-1] + "." + parts[i]
	}
	return ids
}

// createNamespaceNode is createNamespaceNode(id, label, parentId, explicit).
func (d *DB) createNamespaceNode(id, label, parentID string, hasParent, explicit bool) *NamespaceNode {
	node := &NamespaceNode{
		ID:        id,
		Label:     label,
		Classes:   js.NewMap[string, *ClassNode](),
		Notes:     js.NewMap[string, *ClassNote](),
		Children:  js.NewMap[string, *NamespaceNode](),
		DomID:     mermaidDomIDPrefix + id + "-" + strconv.Itoa(d.namespaceCounter),
		Parent:    parentID,
		HasParent: hasParent,
		Explicit:  explicit,
	}
	d.namespaceCounter++
	return node
}

// linkParentChild is linkParentChild(parentId, childId).
func (d *DB) linkParentChild(parentID, childID string) {
	parent, ok1 := d.namespaces.Lookup(parentID)
	child, ok2 := d.namespaces.Lookup(childID)
	if !ok1 || !ok2 {
		return
	}
	if !parent.Children.Has(childID) {
		parent.Children.Set(childID, child)
	}
	if !child.HasParent {
		child.Parent, child.HasParent = parentID, true
	}
}

// AddNamespace is addNamespace(id, label). An empty label stands for no
// label: upstream tests the label for truthiness.
func (d *DB) AddNamespace(id, label string) string {
	qualifiedID := resolveQualifiedId(id, d.namespaceStack)
	// Push first: the grammar guarantees a matching popNamespace in all
	// cases, also for a namespace that is declared again.
	d.namespaceStack = append(d.namespaceStack, qualifiedID)

	if existing, ok := d.namespaces.Lookup(qualifiedID); ok {
		// Re-declaration promotes an auto-created ancestor to explicit
		existing.Explicit = true
		if label != "" {
			existing.Label = label
		}
		return qualifiedID
	}

	parts := strings.Split(qualifiedID, ".")
	ancestorIDs := getAncestorIds(qualifiedID)
	for i := 0; i < len(ancestorIDs); i++ {
		currentID := ancestorIDs[i]
		parentID, hasParent := "", false
		if i > 0 {
			parentID, hasParent = ancestorIDs[i-1], true
		}
		isLeaf := i == len(ancestorIDs)-1
		nodeLabel := parts[i]
		if isLeaf && label != "" {
			nodeLabel = label
		}

		if !d.namespaces.Has(currentID) {
			d.namespaces.Set(currentID, d.createNamespaceNode(currentID, nodeLabel, parentID, hasParent, isLeaf))
		} else if isLeaf {
			d.namespaces.Get(currentID).Explicit = true
		}
		// `if (parentId)`: an empty parent id is falsy.
		if parentID != "" {
			d.linkParentChild(parentID, currentID)
		}
	}

	return qualifiedID
}

// PopNamespace is popNamespace().
func (d *DB) PopNamespace() {
	if n := len(d.namespaceStack); n > 0 {
		d.namespaceStack = d.namespaceStack[:n-1]
	}
}

// GetNamespace is getNamespace(name). The result is nil for an unknown
// namespace.
func (d *DB) GetNamespace(name string) *NamespaceNode {
	return d.namespaces.Get(name)
}

// GetNamespaces is getNamespaces().
func (d *DB) GetNamespaces() *js.Map[string, *NamespaceNode] {
	return d.namespaces
}

// AddClassesToNamespace is addClassesToNamespace(id, classNames, noteNames):
// the parser calls it when it finds a namespace.
func (d *DB) AddClassesToNamespace(id string, classNames, noteNames []string) {
	ns, ok := d.namespaces.Lookup(id)
	if !ok {
		return
	}
	for _, name := range classNames {
		className, _ := d.splitClassNameAndType(name)
		classNode := d.GetClass(className)
		if classNode == nil {
			d.fail(undefinedSet("parent"))
			return
		}
		classNode.Parent, classNode.HasParent = id, true
		ns.Classes.Set(className, classNode)
	}
	for _, noteName := range noteNames {
		noteNode := d.GetNote(noteName)
		if noteNode == nil {
			d.fail(undefinedSet("parent"))
			return
		}
		noteNode.Parent, noteNode.HasParent = id, true
		ns.Notes.Set(noteName, noteNode)
	}
}

// SetCssStyle is setCssStyle(id, styles).
func (d *DB) SetCssStyle(id string, styles []string) {
	thisClass, ok := d.classes.Lookup(id)
	if styles == nil || !ok {
		return
	}
	for _, s := range styles {
		if strings.Contains(s, ",") {
			thisClass.Styles = append(thisClass.Styles, strings.Split(s, ",")...)
		} else {
			thisClass.Styles = append(thisClass.Styles, s)
		}
	}
}

// getArrowMarker is getArrowMarker(type): the arrow marker for a type.
func getArrowMarker(typ int) string {
	switch typ {
	case 0:
		return "aggregation"
	case 1:
		return "extension"
	case 2:
		return "composition"
	case 3:
		return "dependency"
	case 4:
		return "lollipop"
	default:
		return "none"
	}
}

// resolveExplicitAncestor is resolveExplicitAncestor(id): it walks up the
// namespace tree and returns the nearest namespace that the user declared,
// the id itself included. The compact rendering mode gives the children to
// that namespace. has is false when the id is undefined.
func (d *DB) resolveExplicitAncestor(id string, has bool) (string, bool) {
	current, ok := id, has
	// `while (current)`: an empty id is falsy.
	for ok && current != "" {
		ns, found := d.namespaces.Lookup(current)
		if !found {
			return "", false
		}
		if ns.Explicit {
			return current, true
		}
		current, ok = ns.Parent, ns.HasParent
	}
	return "", false
}

// GetData is getData(): the layout data of the diagram.
func (d *DB) GetData() *ru.LayoutData {
	nodes := []*ru.Node{}
	edges := []*ru.Edge{}
	config := d.getConfig()
	classConfig := js.AsObj(config.Get("class"))
	hierarchical := true
	if classConfig != nil {
		if v := classConfig.Get("hierarchicalNamespaces"); !js.IsNullish(v) {
			hierarchical = js.Truthy(v)
		}
	}
	// config.class!.padding ?? def
	padding := func(def float64) float64 {
		if classConfig != nil {
			if v := classConfig.Get("padding"); !js.IsNullish(v) {
				return js.ToNumber(v)
			}
		}
		return def
	}
	look := ""
	if v := config.Get("look"); !js.IsNullish(v) {
		look = js.ToString(v)
	}

	d.namespaces.Each(func(_ string, namespace *NamespaceNode) {
		if !hierarchical && !namespace.Explicit {
			return
		}
		node := ru.NewNode(namespace.ID)
		node.Label, node.HasLabel = namespace.ID, true
		if hierarchical {
			node.Label = namespace.Label
		}
		node.IsGroup = true
		node.Padding = padding(16)
		// parent node must be one of [rect, roundedWithTitle, noteGroup, divider]
		node.Shape = "rect"
		node.CSSStyles, node.CSSStylesSet = []string{}, true
		node.Look = look
		if hierarchical && namespace.HasParent {
			node.ParentID = namespace.Parent
		}
		nodes = append(nodes, node)
	})

	// Only classes consume a colour slot -- namespaces are containers and notes have
	// their own fixed note colour, so neither should shift the cycle.
	classColorIndex := 0
	d.classes.Each(func(_ string, classNode *ClassNode) {
		parentID, hasParentID := classNode.Parent, classNode.HasParent
		if !hierarchical {
			parentID, hasParentID = d.resolveExplicitAncestor(classNode.Parent, classNode.HasParent)
		}
		node := ru.NewNode(classNode.ID)
		// ...classNode
		node.Label, node.HasLabel = classNode.Label, true
		node.Set("text", classNode.Text)
		node.Shape = classNode.Shape
		node.CSSClasses, node.CSSClassesSet = classNode.CSSClasses, true
		node.Set("methods", classNode.Methods)
		node.Set("members", classNode.Members)
		node.Set("annotations", classNode.Annotations)
		node.Set("styles", classNode.Styles)
		node.DomID = classNode.DomID
		if classNode.HasParent {
			node.Set("parent", classNode.Parent)
		}
		if classNode.LinkDefined {
			node.Link = classNode.Link
		}
		if classNode.HasLinkTarget {
			node.LinkTarget = classNode.LinkTarget
		}
		node.HaveCallback = classNode.HaveCallback
		if classNode.HasTooltip {
			node.Tooltip = classNode.Tooltip
		}
		// type: undefined
		node.IsGroup = false
		if hasParentID {
			node.ParentID = parentID
		}
		node.Look = look
		node.ColorIndex = float64(classColorIndex)
		classColorIndex++
		nodes = append(nodes, node)
	})

	themeVariables := js.AsObj(config.Get("themeVariables"))
	themeVariable := func(name string) string {
		if themeVariables == nil {
			return "undefined"
		}
		return js.ToString(themeVariables.Get(name))
	}
	d.notes.Each(func(_ string, note *ClassNote) {
		noteParentID, hasNoteParentID := note.Parent, note.HasParent
		if !hierarchical {
			noteParentID, hasNoteParentID = d.resolveExplicitAncestor(note.Parent, note.HasParent)
		}
		noteNode := ru.NewNode(note.ID)
		noteNode.Label, noteNode.HasLabel = note.Text, true
		noteNode.IsGroup = false
		noteNode.Shape = "note"
		noteNode.Padding = padding(6)
		noteNode.CSSStyles, noteNode.CSSStylesSet = []string{
			"text-align: left",
			"white-space: nowrap",
			"fill: " + themeVariable("noteBkgColor"),
			"stroke: " + themeVariable("noteBorderColor"),
		}, true
		noteNode.Look = look
		if hasNoteParentID {
			noteNode.ParentID = noteParentID
		}
		noteNode.LabelType = "markdown"
		nodes = append(nodes, noteNode)

		// this.classes.get(note.class)?.id
		var noteClass *ClassNode
		if note.HasClass {
			noteClass = d.classes.Get(note.Class)
		}

		// `if (noteClassId)`: an empty id is falsy.
		if noteClass != nil && noteClass.ID != "" {
			edge := ru.NewEdge("edgeNote" + strconv.Itoa(note.Index))
			edge.Start = note.ID
			edge.End = noteClass.ID
			edge.Type = "normal"
			edge.Thickness = "normal"
			edge.Classes = "relation"
			edge.ArrowTypeStart = "none"
			edge.ArrowTypeEnd = "none"
			edge.ArrowheadStyle = ""
			edge.LabelStyle = []string{""}
			edge.Style = []string{"fill: none"}
			edge.Pattern = "dotted"
			edge.Look = look
			edges = append(edges, edge)
		}
	})

	for _, _interface := range d.interfaces {
		interfaceNode := ru.NewNode(_interface.ID)
		interfaceNode.Label, interfaceNode.HasLabel = _interface.Label, true
		interfaceNode.IsGroup = false
		interfaceNode.Shape = "rect"
		interfaceNode.CSSStyles, interfaceNode.CSSStylesSet = []string{"opacity: 0;"}, true
		interfaceNode.Look = look
		nodes = append(nodes, interfaceNode)
	}

	cnt := 0
	for _, classRelation := range d.relations {
		cnt++
		edge := ru.NewEdge(utils.GetEdgeId(classRelation.ID1, classRelation.ID2, utils.EdgeIdOptions{
			Prefix:  "id",
			Counter: cnt,
		}, ""))
		edge.Start = classRelation.ID1
		edge.End = classRelation.ID2
		edge.Type = "normal"
		edge.Label, edge.HasLabel = classRelation.Title, classRelation.HasTitle
		edge.Labelpos = "c"
		edge.Thickness = "normal"
		edge.Classes = "relation"
		edge.ArrowTypeStart = getArrowMarker(classRelation.Relation.Type1)
		edge.ArrowTypeEnd = getArrowMarker(classRelation.Relation.Type2)
		if classRelation.RelationTitle1 != "none" {
			edge.StartLabelRight = classRelation.RelationTitle1
		}
		if classRelation.RelationTitle2 != "none" {
			edge.EndLabelLeft = classRelation.RelationTitle2
		}
		edge.ArrowheadStyle = ""
		edge.LabelStyle = []string{"display: inline-block"}
		// style: classRelation.style || ''. The relation has no style, so
		// the style is the empty string: an array with one element for the
		// edge code.
		edge.Style = []string{""}
		edge.Pattern = "solid"
		if classRelation.Relation.LineType == 1 {
			edge.Pattern = "dashed"
		}
		edge.Look = look
		edge.LabelType = "markdown"
		edges = append(edges, edge)
	}

	return &ru.LayoutData{Nodes: nodes, Edges: edges, Config: config, Direction: d.GetDirection()}
}

// GetClassConfig is getConfig() of the db: the class part of the config.
func (d *DB) GetClassConfig() *js.Obj[any] {
	return js.AsObj(d.getConfig().Get("class"))
}
