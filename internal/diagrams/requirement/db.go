// Ported from mermaid 12.2.0, diagrams/requirement/requirementDb.ts (MIT).

package requirement

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// DB is RequirementDB: the data of a parsed requirement diagram.
type DB struct {
	render.CommonDB
	ctx *render.Context

	relations         []*Relation
	latestRequirement *Requirement
	requirements      *js.Map[string, *Requirement]
	latestElement     *Element
	elements          *js.Map[string, *Element]
	classes           *js.Map[string, *RequirementClass]
	direction         string
}

// NewDB returns a db in the state after the constructor of RequirementDB.
func NewDB(ctx *render.Context) *DB {
	d := &DB{ctx: ctx, direction: "TB"}
	d.CommonDB.Sanitize = ctx.SanitizeText
	d.Clear()
	return d
}

// getConfig is getConfig() of diagramAPI: a copy of the current config.
func (d *DB) getConfig() *js.Obj[any] {
	if d.ctx.ConfigState != nil {
		return d.ctx.ConfigState.GetConfig()
	}
	cfg, _ := config.AssignWithDepth(js.NewObj[any](), d.ctx.Config, 2).(*js.Obj[any])
	return cfg
}

// GetDirection is getDirection().
func (d *DB) GetDirection() string {
	return d.direction
}

// SetDirection is setDirection(dir).
func (d *DB) SetDirection(dir string) {
	d.direction = dir
}

func (d *DB) resetLatestRequirement() {
	d.latestRequirement = d.getInitialRequirement()
}

func (d *DB) resetLatestElement() {
	d.latestElement = d.getInitialElement()
}

func (d *DB) getInitialRequirement() *Requirement {
	return &Requirement{
		RequirementID: "",
		Text:          "",
		Risk:          "",
		VerifyMethod:  "",
		Name:          "",
		Type:          "",
		CSSStyles:     []string{},
		Classes:       []string{"default"},
	}
}

func (d *DB) getInitialElement() *Element {
	return &Element{
		Name:      "",
		Type:      "",
		DocRef:    "",
		CSSStyles: []string{},
		Classes:   []string{"default"},
	}
}

// AddRequirement is addRequirement(name, type): it adds a requirement with
// the values of the body that the parser read before, unless a requirement
// with the name exists.
func (d *DB) AddRequirement(name, typ string) *Requirement {
	if !d.requirements.Has(name) {
		d.requirements.Set(name, &Requirement{
			Name:          name,
			Type:          typ,
			RequirementID: d.latestRequirement.RequirementID,
			Text:          d.latestRequirement.Text,
			Risk:          d.latestRequirement.Risk,
			VerifyMethod:  d.latestRequirement.VerifyMethod,
			CSSStyles:     []string{},
			Classes:       []string{"default"},
		})
	}
	d.resetLatestRequirement()

	return d.requirements.Get(name)
}

// GetRequirements is getRequirements().
func (d *DB) GetRequirements() *js.Map[string, *Requirement] {
	return d.requirements
}

// SetNewReqId is setNewReqId(id).
func (d *DB) SetNewReqId(id string) {
	if d.latestRequirement != nil {
		d.latestRequirement.RequirementID = id
	}
}

// SetNewReqText is setNewReqText(text).
func (d *DB) SetNewReqText(text string) {
	if d.latestRequirement != nil {
		d.latestRequirement.Text = text
	}
}

// SetNewReqRisk is setNewReqRisk(risk).
func (d *DB) SetNewReqRisk(risk string) {
	if d.latestRequirement != nil {
		d.latestRequirement.Risk = risk
	}
}

// SetNewReqVerifyMethod is setNewReqVerifyMethod(verifyMethod).
func (d *DB) SetNewReqVerifyMethod(verifyMethod string) {
	if d.latestRequirement != nil {
		d.latestRequirement.VerifyMethod = verifyMethod
	}
}

// AddElement is addElement(name): it adds an element with the values of the
// body that the parser read before, unless an element with the name exists.
func (d *DB) AddElement(name string) *Element {
	if !d.elements.Has(name) {
		d.elements.Set(name, &Element{
			Name:      name,
			Type:      d.latestElement.Type,
			DocRef:    d.latestElement.DocRef,
			CSSStyles: []string{},
			Classes:   []string{"default"},
		})
	}
	d.resetLatestElement()

	return d.elements.Get(name)
}

// GetElements is getElements().
func (d *DB) GetElements() *js.Map[string, *Element] {
	return d.elements
}

// SetNewElementType is setNewElementType(type).
func (d *DB) SetNewElementType(typ string) {
	if d.latestElement != nil {
		d.latestElement.Type = typ
	}
}

// SetNewElementDocRef is setNewElementDocRef(docRef).
func (d *DB) SetNewElementDocRef(docRef string) {
	if d.latestElement != nil {
		d.latestElement.DocRef = docRef
	}
}

// AddRelationship is addRelationship(type, src, dst).
func (d *DB) AddRelationship(typ, src, dst string) {
	d.relations = append(d.relations, &Relation{
		Type: typ,
		Src:  src,
		Dst:  dst,
	})
}

// GetRelationships is getRelationships().
func (d *DB) GetRelationships() []*Relation {
	return d.relations
}

// Clear is clear().
func (d *DB) Clear() {
	d.relations = []*Relation{}
	d.resetLatestRequirement()
	d.requirements = js.NewMap[string, *Requirement]()
	d.resetLatestElement()
	d.elements = js.NewMap[string, *Element]()
	d.classes = js.NewMap[string, *RequirementClass]()
	d.ClearCommon()
}

// styled is the part of a requirement or an element that the style
// functions change: its cssStyles and its classes.
type styled struct {
	cssStyles *[]string
	classes   *[]string
}

// lookup is `this.requirements.get(id) ?? this.elements.get(id)`.
func (d *DB) lookup(id string) (styled, bool) {
	if r, ok := d.requirements.Lookup(id); ok && r != nil {
		return styled{cssStyles: &r.CSSStyles, classes: &r.Classes}, true
	}
	if e, ok := d.elements.Lookup(id); ok && e != nil {
		return styled{cssStyles: &e.CSSStyles, classes: &e.Classes}, true
	}
	return styled{}, false
}

// SetCssStyle is setCssStyle(ids, styles). It stops at the first id that
// is not a requirement or an element.
func (d *DB) SetCssStyle(ids []string, styles []string) {
	for _, id := range ids {
		node, ok := d.lookup(id)
		if styles == nil || !ok {
			return
		}
		for _, s := range styles {
			if strings.Contains(s, ",") {
				*node.cssStyles = append(*node.cssStyles, strings.Split(s, ",")...)
			} else {
				*node.cssStyles = append(*node.cssStyles, s)
			}
		}
	}
}

// SetClass is setClass(ids, classNames).
func (d *DB) SetClass(ids []string, classNames []string) {
	for _, id := range ids {
		node, ok := d.lookup(id)
		if ok {
			for _, class := range classNames {
				*node.classes = append(*node.classes, class)
				if styleClass := d.classes.Get(class); styleClass != nil {
					// A styles array is truthy, also when it is empty.
					*node.cssStyles = append(*node.cssStyles, styleClass.Styles...)
				}
			}
		}
	}
}

// includes is Array.prototype.includes for strings.
func includes(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// DefineClass is defineClass(ids, style).
func (d *DB) DefineClass(ids []string, style []string) {
	for _, id := range ids {
		styleClass := d.classes.Get(id)
		if styleClass == nil {
			styleClass = &RequirementClass{ID: id, Styles: []string{}, TextStyles: []string{}}
			d.classes.Set(id, styleClass)
		}

		for _, s := range style {
			if strings.Contains(s, "color") {
				newStyle := strings.Replace(s, "fill", "bgFill", 1) // .replace('color', 'fill');
				styleClass.TextStyles = append(styleClass.TextStyles, newStyle)
			}
			styleClass.Styles = append(styleClass.Styles, s)
		}

		// style.flatMap((s) => s.split(','))
		flat := func() []string {
			var out []string
			for _, s := range style {
				out = append(out, strings.Split(s, ",")...)
			}
			return out
		}
		d.requirements.Each(func(_ string, value *Requirement) {
			if includes(value.Classes, id) {
				value.CSSStyles = append(value.CSSStyles, flat()...)
			}
		})
		d.elements.Each(func(_ string, value *Element) {
			if includes(value.Classes, id) {
				value.CSSStyles = append(value.CSSStyles, flat()...)
			}
		})
	}
}

// GetClasses is getClasses().
func (d *DB) GetClasses() *js.Map[string, *RequirementClass] {
	return d.classes
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

// stringsValue returns a list of strings as a JavaScript array.
func stringsValue(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// nodeName is `this.requirements.get(id)?.name ?? this.elements.get(id)?.name`.
// The result is "" when the value is undefined: a name is never empty.
func (d *DB) nodeName(id string) string {
	if r := d.requirements.Get(id); r != nil {
		return r.Name
	}
	if e := d.elements.Get(id); e != nil {
		return e.Name
	}
	return ""
}

// GetData is getData(): the nodes and the edges for the unified renderer,
// the config and the direction. Upstream makes each requirement and each
// element a layout node by adding properties to it, so a node has the
// properties of the requirement or of the element in Extra: name, type,
// requirementId, text, risk, verifyMethod and classes, or name, type,
// docRef and classes.
func (d *DB) GetData() *ru.LayoutData {
	cfg := d.getConfig()
	look := lookOf(cfg)
	nodes := []*ru.Node{}
	edges := []*ru.Edge{}
	d.requirements.Each(func(_ string, requirement *Requirement) {
		node := ru.NewNode(requirement.Name)
		node.Set("name", requirement.Name)
		node.Set("type", requirement.Type)
		node.Set("requirementId", requirement.RequirementID)
		node.Set("text", requirement.Text)
		node.Set("risk", requirement.Risk)
		node.Set("verifyMethod", requirement.VerifyMethod)
		node.Set("classes", stringsValue(requirement.Classes))
		node.CSSStyles, node.CSSStylesSet = requirement.CSSStyles, true
		node.CSSClasses, node.CSSClassesSet = strings.Join(requirement.Classes, " "), true
		node.Shape = "requirementBox"
		node.Look = look
		node.ColorIndex = float64(len(nodes))
		nodes = append(nodes, node)
	})

	d.elements.Each(func(_ string, element *Element) {
		node := ru.NewNode(element.Name)
		node.Set("name", element.Name)
		node.Set("type", element.Type)
		node.Set("docRef", element.DocRef)
		node.Set("classes", stringsValue(element.Classes))
		node.Shape = "requirementBox"
		node.Look = look
		node.CSSStyles, node.CSSStylesSet = element.CSSStyles, true
		node.CSSClasses, node.CSSClassesSet = strings.Join(element.Classes, " "), true
		node.ColorIndex = float64(len(nodes))
		nodes = append(nodes, node)
	})

	counter := 0
	for _, relation := range d.relations {
		isContains := relation.Type == RelContains
		edge := ru.NewEdge(js.Concat(relation.Src, "-", relation.Dst, "-", js.Str(float64(counter))))
		counter++
		// An undefined start or end is "".
		edge.Start = d.nodeName(relation.Src)
		edge.End = d.nodeName(relation.Dst)
		edge.Label, edge.HasLabel = "&lt;&lt;"+relation.Type+"&gt;&gt;", true
		edge.Classes = "relationshipLine"
		if isContains {
			edge.Style = []string{"fill:none", ""}
		} else {
			edge.Style = []string{"fill:none", "stroke-dasharray: 10,7"}
		}
		edge.Labelpos = "c"
		edge.Thickness = "normal"
		edge.Type = "normal"
		if isContains {
			edge.Pattern = "normal"
			edge.ArrowTypeStart = "requirement_contains"
			edge.ArrowTypeEnd = ""
		} else {
			edge.Pattern = "dashed"
			edge.ArrowTypeStart = ""
			edge.ArrowTypeEnd = "requirement_arrow"
		}
		edge.Look = look
		edge.LabelType = "markdown"

		edges = append(edges, edge)
	}

	return &ru.LayoutData{
		Nodes:     nodes,
		Edges:     edges,
		Config:    cfg,
		Direction: d.GetDirection(),
		Extra:     map[string]any{"other": js.NewObj[any]()},
	}
}

// GetConfig is getConfig(): the `requirement` section of the config.
func (d *DB) GetConfig() any {
	return d.ctx.Config.Get("requirement")
}
