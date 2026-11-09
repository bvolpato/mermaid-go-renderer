// Ported from mermaid 12.2.0, diagrams/kanban/kanbanDb.ts and kanbanTypes.ts
// (MIT).

package kanban

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// The values of nodeType.
const (
	nodeTypeDefault     = 0
	nodeTypeNoBorder    = 0
	nodeTypeRoundedRect = 1
	nodeTypeRect        = 2
	nodeTypeCircle      = 3
	nodeTypeCloud       = 4
	nodeTypeBang        = 5
	nodeTypeHexagon     = 6
)

// Node is the KanbanNode that addNode makes. A property that is undefined
// has its Has field false, or is nil for a dynamic value.
type Node struct {
	ID    string
	Level int
	// Label is a string, or the value of the label key of the shape data,
	// which can have any type.
	Label any
	// Width and Padding are config values: numbers, unless the config has
	// another type there.
	Width   any
	Padding any
	IsGroup bool

	Shape       string
	HasShape    bool
	Icon        string
	HasIcon     bool
	Assigned    string
	HasAssigned bool
	Ticket      string
	HasTicket   bool
	// Priority is the value of the priority key of the shape data, or nil.
	Priority any

	ParentID    string
	HasParentID bool
	CSSClasses  string
	HasClasses  bool
}

// DB is the db of kanbanDb.ts: its module-level state and its functions.
type DB struct {
	ctx      *render.Context
	nodes    []*Node
	sections []*Node
	cnt      int
}

// NewDB returns an empty db: the state after clear().
func NewDB(ctx *render.Context) *DB {
	return &DB{ctx: ctx}
}

// Clear is clear().
func (d *DB) Clear() {
	d.nodes = nil
	d.sections = nil
	d.cnt = 0
}

// Error is an error that the db or the renderer of upstream throws.
type Error struct {
	// Name is the name of the JavaScript error. Empty for Error.
	Name    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func typeError(message string) error {
	return &Error{Name: "TypeError", Message: message}
}

// sanitizeText is sanitizeText(text, getConfig()) for a string.
func (d *DB) sanitizeText(text string) (string, error) {
	return d.ctx.Sanitizer.SanitizeTextErr(text, d.ctx.Config)
}

// toStringMethod is v.toString() for a value that is not nullish. An object
// of the shape data can have a toString key of its own, which is then not a
// function.
func toStringMethod(v any, expr string) (string, error) {
	if o, ok := v.(*js.Obj[any]); ok && o.Has("toString") {
		return "", typeError(expr + ".toString is not a function")
	}
	return js.ToString(v), nil
}

// sanitizeValue is sanitizeText(text, getConfig()) for a label, which is a
// string or a value of the shape data. A value that is not a string is
// truthy here. DOMPurify converts it to a string (stringifyValue): it calls
// the toString function that it finds on the prototype chain, so an object
// is "[object Object]" also when it has a toString key of its own. With a
// security level that is not one of the known ones, sanitizeMore calls a
// string method on the value first.
func (d *DB) sanitizeValue(text any) (string, error) {
	if s, ok := text.(string); ok {
		return d.sanitizeText(s)
	}
	conf := d.ctx.Config
	if sanitize.GetEffectiveHtmlLabels(conf) {
		switch conf.Get("securityLevel") {
		case "antiscript", "strict", "sandbox", "loose":
		default:
			// breakToPlaceholder calls s.replace.
			return "", typeError("s.replace is not a function")
		}
	}
	s := js.ToString(text)
	if s == "" {
		// DOMPurify returns a text without "<" as it is.
		return "", nil
	}
	return d.sanitizeText(s)
}

// getSection is getSection(level): nil when the level is the section level,
// otherwise the current section.
func (d *DB) getSection(level int) (*Node, error) {
	if len(d.nodes) == 0 {
		return nil, nil
	}
	sectionLevel := d.nodes[0].Level
	var lastSection *Node
	for i := len(d.nodes) - 1; i >= 0; i-- {
		if d.nodes[i].Level == sectionLevel && lastSection == nil {
			lastSection = d.nodes[i]
		}
		if d.nodes[i].Level < sectionLevel {
			label, err := concatValue(d.nodes[i].Label)
			if err != nil {
				return nil, err
			}
			return nil, &Error{Message: `Items without section detected, found section ("` + label + `")`}
		}
	}
	if lastSection != nil && level == lastSection.Level {
		return nil, nil
	}

	// No found
	return lastSection, nil
}

// concatValue is the value in a string concatenation. An object of the
// shape data can have a toString key of its own, which is then not a
// function. The valueOf of Object.prototype returns the object, which is
// not a primitive.
func concatValue(v any) (string, error) {
	if o, ok := v.(*js.Obj[any]); ok && o.Has("toString") {
		return "", typeError("Cannot convert object to primitive value")
	}
	return js.ToString(v), nil
}

// GetSections is getSections().
func (d *DB) GetSections() []*Node {
	return d.sections
}

// Data is the result of getData(): the nodes for the renderer. The edges
// are an empty array, `other` is an empty object and `config` is
// getConfig().
type Data struct {
	Nodes  []*ru.Node
	Config *js.Obj[any]
}

// GetData is getData().
func (d *DB) GetData() (*Data, error) {
	nodes := []*ru.Node{}

	sections := d.GetSections()
	conf := d.ctx.Config

	for _, section := range sections {
		label, err := d.sanitizeValue(orEmpty(section.Label))
		if err != nil {
			return nil, err
		}
		node := ru.NewNode(section.ID)
		node.Label = label
		node.HasLabel = true
		node.LabelType = "markdown"
		node.IsGroup = true
		if section.HasTicket {
			node.Set("ticket", section.Ticket)
		}
		node.Shape = "kanbanSection"
		node.Level = float64(section.Level)
		if look := conf.Get("look"); !js.IsNullish(look) {
			node.Look = js.ToString(look)
		}
		nodes = append(nodes, node)

		for _, item := range d.nodes {
			// nodes.filter((n) => n.parentId === section.id)
			if !item.HasParentID || item.ParentID != section.ID {
				continue
			}
			label, err := d.sanitizeValue(orEmpty(item.Label))
			if err != nil {
				return nil, err
			}
			childNode := ru.NewNode(item.ID)
			childNode.ParentID = section.ID
			childNode.Label = label
			childNode.HasLabel = true
			childNode.LabelType = "markdown"
			childNode.IsGroup = false
			if item.HasTicket {
				childNode.Set("ticket", item.Ticket)
			}
			if item.Priority != nil {
				childNode.Set("priority", item.Priority)
			}
			if item.HasAssigned {
				childNode.Set("assigned", item.Assigned)
			}
			childNode.Icon = item.Icon
			childNode.Shape = "kanbanItem"
			childNode.Level = float64(item.Level)
			childNode.Rx = 5
			childNode.Ry = 5
			childNode.CSSStyles = []string{"text-align: left"}
			childNode.CSSStylesSet = true
			nodes = append(nodes, childNode)
		}
	}

	return &Data{Nodes: nodes, Config: conf}, nil
}

// orEmpty is the label with the empty string for a nullish value.
func orEmpty(label any) any {
	if js.IsNullish(label) {
		return ""
	}
	return label
}

// mindmapConfig is `conf.mindmap?.<key> ?? defaultConfig.mindmap.<key>`.
func mindmapConfig(ctx *render.Context, conf *js.Obj[any], key string) any {
	if v := js.Path(conf, "mindmap", key); !js.IsNullish(v) {
		return v
	}
	return js.Path(ctx.ConfigState.DefaultSection("mindmap"), key)
}

// prop is doc?.key: undefined for a doc that is nullish.
func prop(doc any, key string) any {
	v, _ := js.GetProperty(doc, key)
	return v
}

// AddNode is addNode(level, id, descr, type, shapeData). shapeData is nil
// when the argument is undefined.
func (d *DB) AddNode(level int, id, descr string, typ int, shapeData *string) error {
	conf := d.ctx.Config
	padding := mindmapConfig(d.ctx, conf, "padding")
	switch typ {
	case nodeTypeRoundedRect, nodeTypeRect, nodeTypeHexagon:
		// padding *= 2
		padding = float64(js.ToNumber(padding) * 2)
	}

	nodeID, err := d.sanitizeText(id)
	if err != nil {
		return err
	}
	if nodeID == "" {
		nodeID = "kbn" + strconv.Itoa(d.cnt)
		d.cnt++
	}
	label, err := d.sanitizeText(descr)
	if err != nil {
		return err
	}
	node := &Node{
		ID:      nodeID,
		Level:   level,
		Label:   label,
		Width:   mindmapConfig(d.ctx, conf, "maxNodeWidth"),
		Padding: padding,
		IsGroup: false,
	}

	if shapeData != nil {
		var yamlData string
		// detect if shapeData contains a newline character
		if !strings.Contains(*shapeData, "\n") {
			yamlData = "{\n" + *shapeData + "\n}"
		} else {
			yamlData = *shapeData + "\n"
		}
		doc, err := config.LoadYAML(yamlData)
		if err != nil {
			return err
		}
		shape, ok := js.GetProperty(doc, "shape")
		if !ok {
			return typeError("Cannot read properties of " + js.ToString(doc) + " (reading 'shape')")
		}
		if js.Truthy(shape) {
			s, isString := shape.(string)
			if !isString {
				return typeError("doc.shape.toLowerCase is not a function")
			}
			if s != js.ToLowerCase(s) || strings.Contains(s, "_") {
				return &Error{Message: "No such shape: " + s + ". Shape names should be lowercase."}
			}
			// if shape is defined in the yaml data, use it if it is a valid shape kanbanItem
			if s == "kanbanItem" {
				node.Shape, node.HasShape = s, true
			}
		}
		if v := prop(doc, "label"); js.Truthy(v) {
			node.Label = v
		}
		if v := prop(doc, "icon"); js.Truthy(v) {
			s, err := toStringMethod(v, "doc?.icon")
			if err != nil {
				return err
			}
			node.Icon, node.HasIcon = s, true
		}
		if v := prop(doc, "assigned"); js.Truthy(v) {
			s, err := toStringMethod(v, "doc?.assigned")
			if err != nil {
				return err
			}
			node.Assigned, node.HasAssigned = s, true
		}
		if v := prop(doc, "ticket"); js.Truthy(v) {
			s, err := toStringMethod(v, "doc?.ticket")
			if err != nil {
				return err
			}
			node.Ticket, node.HasTicket = s, true
		}

		if v := prop(doc, "priority"); js.Truthy(v) {
			node.Priority = v
		}
	}

	section, err := d.getSection(level)
	if err != nil {
		return err
	}
	if section != nil {
		// section.id || 'kbn' + cnt++
		if section.ID != "" {
			node.ParentID = section.ID
		} else {
			node.ParentID = "kbn" + strconv.Itoa(d.cnt)
			d.cnt++
		}
		node.HasParentID = true
	} else {
		d.sections = append(d.sections, node)
	}
	d.nodes = append(d.nodes, node)
	return nil
}

// GetType is getType(startStr, endStr).
func (d *DB) GetType(startStr, endStr string) int {
	switch startStr {
	case "[":
		return nodeTypeRect
	case "(":
		if endStr == ")" {
			return nodeTypeRoundedRect
		}
		return nodeTypeCloud
	case "((":
		return nodeTypeCircle
	case ")":
		return nodeTypeCloud
	case "))":
		return nodeTypeBang
	case "{{":
		return nodeTypeHexagon
	default:
		return nodeTypeDefault
	}
}

// Decoration is the argument of decorateNode.
type Decoration struct {
	Class string
	Icon  string
}

// DecorateNode is decorateNode(decoration).
func (d *DB) DecorateNode(decoration Decoration) error {
	var node *Node
	if len(d.nodes) > 0 {
		node = d.nodes[len(d.nodes)-1]
	}
	if decoration.Icon != "" {
		icon, err := d.sanitizeText(decoration.Icon)
		if err != nil {
			return err
		}
		if node == nil {
			return typeError("Cannot set properties of undefined (setting 'icon')")
		}
		node.Icon, node.HasIcon = icon, true
	}
	if decoration.Class != "" {
		class, err := d.sanitizeText(decoration.Class)
		if err != nil {
			return err
		}
		if node == nil {
			return typeError("Cannot set properties of undefined (setting 'cssClasses')")
		}
		node.CSSClasses, node.HasClasses = class, true
	}
	return nil
}

// Type2Str is type2Str(type).
func (d *DB) Type2Str(typ int) string {
	switch typ {
	case nodeTypeDefault:
		return "no-border"
	case nodeTypeRect:
		return "rect"
	case nodeTypeRoundedRect:
		return "rounded-rect"
	case nodeTypeCircle:
		return "circle"
	case nodeTypeCloud:
		return "cloud"
	case nodeTypeBang:
		return "bang"
	case nodeTypeHexagon:
		return "hexgon" // cspell: disable-line
	default:
		return "no-border"
	}
}
