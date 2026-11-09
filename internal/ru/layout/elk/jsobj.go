// The JavaScript object form of the layout data.
//
// Upstream builds the ELK graph from copies of its node and edge objects
// ({ ...node }, { ...edge }), so every property of a node or an edge goes to
// ELK, in the property order of the object. The functions of render.go that
// build the graph work on such objects. This file makes them from the Go
// structs of internal/ru, and has the helpers for dynamic values.

package elk

import (
	"sort"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// layoutObjects is the part of data4Layout that the graph is built from, as
// JavaScript values.
type layoutObjects struct {
	// nodes and edges are data4Layout.nodes and data4Layout.edges.
	nodes []*js.Obj[any]
	edges []*js.Obj[any]
	// config is data4Layout.config.
	config *js.Obj[any]
	// direction is data4Layout.direction. nil stands for undefined.
	direction any
}

// prop is v?.[key]: nil when v is not an object or has no such property.
func prop(v any, key string) any {
	o, ok := v.(*js.Obj[any])
	if !ok || o == nil {
		return nil
	}
	return o.Get(key)
}

// nn is v ?? d.
func nn(v, d any) any {
	if js.IsNullish(v) {
		return d
	}
	return v
}

// num is the value of a dynamic number in arithmetic.
func num(v any) float64 { return js.ToNumber(v) }

// idOf is the string form of an id. The second result is false for
// undefined and null. Ids are strings in every diagram db.
func idOf(v any) (string, bool) {
	if js.IsNullish(v) {
		return "", false
	}
	return js.ToString(v), true
}

// strictEq is a === b for the dynamic values that ids and flags have.
func strictEq(a, b any) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case string:
		y, ok := b.(string)
		return ok && x == y
	case float64:
		y, ok := b.(float64)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case *js.Obj[any]:
		y, ok := b.(*js.Obj[any])
		return ok && x == y
	}
	if js.IsNullish(a) {
		return a == b
	}
	return false
}

// spread is { ...o }: a shallow copy with the same property order.
func spread(o *js.Obj[any]) *js.Obj[any] {
	out := js.NewObj[any]()
	if o != nil {
		o.Each(func(k string, v any) { out.Set(k, v) })
	}
	return out
}

// assign is Object.assign(dst, src).
func assign(dst, src *js.Obj[any]) {
	if src == nil {
		return
	}
	src.Each(func(k string, v any) { dst.Set(k, v) })
}

// obj builds an object from keys and values, in order.
func obj(kv ...any) *js.Obj[any] {
	o := js.NewObj[any]()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1])
	}
	return o
}

// push is o[key].push(v).
func push(o *js.Obj[any], key string, v any) {
	arr, _ := o.Get(key).([]any)
	o.Set(key, append(arr, v))
}

func strs(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// setStr sets a property when the string is not empty: an empty string
// stands for undefined in the structs of internal/ru.
func setStr(o *js.Obj[any], key, v string) {
	if v != "" {
		o.Set(key, v)
	}
}

// setNum sets a property when the number is defined.
func setNum(o *js.Obj[any], key string, v float64) {
	if !ru.IsUndef(v) {
		o.Set(key, v)
	}
}

func setTrue(o *js.Obj[any], key string, v bool) {
	if v {
		o.Set(key, true)
	}
}

// setExtra adds the extra properties in the order of their names. A value
// that is not a JSON value (a function, a Go struct) is left out, as
// JSON.stringify leaves out a function.
func setExtra(o *js.Obj[any], extra map[string]any) {
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if o.Has(k) {
			continue
		}
		switch v := extra[k].(type) {
		case bool, float64, string, []any, *js.Obj[any]:
			o.Set(k, v)
		case int:
			o.Set(k, float64(v))
		default:
			if v != nil && js.IsNullish(v) {
				o.Set(k, v)
			}
		}
	}
}

// nodeToObj is the JavaScript object of a layout node: the properties of
// types.ts that are defined, in the order of types.ts, and then the extra
// properties. The functions intersect and calcIntersect are left out: JSON
// has no functions.
func nodeToObj(n *ru.Node) *js.Obj[any] {
	o := js.NewObj[any]()
	o.Set("id", n.ID)
	if n.HasLabel || n.Label != "" {
		o.Set("label", n.Label)
	}
	if n.Description != nil {
		o.Set("description", strs(n.Description))
	}
	if n.HasStereotype || n.Stereotype != "" {
		o.Set("stereotype", n.Stereotype)
	}
	setStr(o, "parentId", n.ParentID)
	setStr(o, "position", n.Position)
	if n.CSSStylesSet || n.CSSStyles != nil {
		o.Set("cssStyles", strs(n.CSSStyles))
	}
	if n.CSSCompiledStyles != nil {
		o.Set("cssCompiledStyles", strs(n.CSSCompiledStyles))
	}
	if n.CSSClassesSet || n.CSSClasses != "" {
		o.Set("cssClasses", n.CSSClasses)
	}
	setStr(o, "labelStyle", n.LabelStyle)
	setStr(o, "labelType", n.LabelType)
	setStr(o, "domId", n.DomID)
	setStr(o, "dir", n.Dir)
	setTrue(o, "haveCallback", n.HaveCallback)
	setStr(o, "link", n.Link)
	setStr(o, "linkTarget", n.LinkTarget)
	setStr(o, "tooltip", n.Tooltip)
	setNum(o, "padding", n.Padding)
	o.Set("isGroup", n.IsGroup)
	setNum(o, "width", n.Width)
	setNum(o, "height", n.Height)
	setNum(o, "wrappingWidth", n.WrappingWidth)
	setNum(o, "minWidth", n.MinWidth)
	setTrue(o, "spreadPorts", n.SpreadPorts)
	if n.LabelBBox != nil {
		b := js.NewObj[any]()
		setNum(b, "x", n.LabelBBox.X)
		setNum(b, "y", n.LabelBBox.Y)
		b.Set("width", n.LabelBBox.Width)
		b.Set("height", n.LabelBBox.Height)
		o.Set("labelBBox", b)
	}
	if r := n.GroupTitleRect; r != nil {
		o.Set("groupTitleRect", obj("left", r.Left, "right", r.Right, "top", r.Top, "bottom", r.Bottom))
	}
	setNum(o, "rx", n.Rx)
	setNum(o, "ry", n.Ry)
	setTrue(o, "useHtmlLabels", n.UseHtmlLabels)
	setTrue(o, "centerLabel", n.CenterLabel)
	setStr(o, "backgroundColor", n.BackgroundColor)
	setStr(o, "borderColor", n.BorderColor)
	setStr(o, "borderStyle", n.BorderStyle)
	setNum(o, "borderWidth", n.BorderWidth)
	setStr(o, "labelTextColor", n.LabelTextColor)
	setNum(o, "labelPaddingX", n.LabelPaddingX)
	setNum(o, "labelPaddingY", n.LabelPaddingY)
	setNum(o, "x", n.X)
	setNum(o, "y", n.Y)
	setStr(o, "look", n.Look)
	setStr(o, "icon", n.Icon)
	setStr(o, "pos", n.Pos)
	setStr(o, "img", n.Img)
	setNum(o, "assetWidth", n.AssetWidth)
	setNum(o, "assetHeight", n.AssetHeight)
	setNum(o, "defaultWidth", n.DefaultWidth)
	setNum(o, "imageAspectRatio", n.ImageAspectRatio)
	setStr(o, "constraint", n.Constraint)
	if n.Metadata != nil {
		o.Set("metadata", n.Metadata)
	}
	setNum(o, "layer", n.Layer)
	setNum(o, "order", n.Order)
	if n.Children != nil {
		children := make([]any, len(n.Children))
		for i, c := range n.Children {
			children[i] = nodeToObj(c)
		}
		o.Set("children", children)
	}
	setStr(o, "nodeId", n.NodeID)
	setNum(o, "level", n.Level)
	setStr(o, "descr", n.Descr)
	setNum(o, "type", n.Type)
	setNum(o, "radius", n.Radius)
	setNum(o, "taper", n.Taper)
	setStr(o, "stroke", n.Stroke)
	setNum(o, "colorIndex", n.ColorIndex)
	setStr(o, "shape", n.Shape)
	setTrue(o, "isEdgeLabel", n.IsEdgeLabel)
	setStr(o, "edgeStart", n.EdgeStart)
	setStr(o, "edgeEnd", n.EdgeEnd)
	setTrue(o, "isDummy", n.IsDummy)
	setNum(o, "diff", n.Diff)
	setNum(o, "offsetX", n.OffsetX)
	setNum(o, "offsetY", n.OffsetY)
	setTrue(o, "clusterNode", n.ClusterNode)
	setExtra(o, n.Extra)
	return o
}

// terminalLabelKeys is the order in which insertEdgeLabel records the sizes
// of the terminal labels, and so the property order of terminalLabelSizes
// and terminalLabelCenters.
var terminalLabelKeys = [4]string{"startLeft", "startRight", "endLeft", "endRight"}

// terminalKeys returns the keys of a terminal label map in property order:
// the four known keys first, then other keys by name.
func terminalKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for _, k := range terminalLabelKeys {
		if _, ok := m[k]; ok {
			keys = append(keys, k)
		}
	}
	if len(keys) == len(m) {
		return keys
	}
	var other []string
	for k := range m {
		known := false
		for _, t := range terminalLabelKeys {
			if k == t {
				known = true
			}
		}
		if !known {
			other = append(other, k)
		}
	}
	sort.Strings(other)
	return append(keys, other...)
}

// edgeToObj is the JavaScript object of a layout edge: the properties of
// types.ts that are defined, in the order of types.ts, and then the extra
// properties. The curve is left out when it is not a string: upstream has
// a function there after the prepare phase.
func edgeToObj(e *ru.Edge) *js.Obj[any] {
	o := js.NewObj[any]()
	o.Set("id", e.ID)
	if e.HasLabel || e.Label != "" {
		o.Set("label", e.Label)
	}
	if !e.ClassesUndefined || e.Classes != "" {
		o.Set("classes", e.Classes)
	}
	if !e.StyleUndefined {
		o.Set("style", strs(e.Style))
	}
	if e.HasAnimate || e.Animate {
		o.Set("animate", e.Animate)
	}
	setStr(o, "animation", e.Animation)
	if e.Metadata != nil {
		o.Set("metadata", e.Metadata)
	}
	setStr(o, "arrowhead", e.Arrowhead)
	setStr(o, "arrowheadStyle", e.ArrowheadStyle)
	setStr(o, "arrowTypeEnd", e.ArrowTypeEnd)
	setStr(o, "arrowTypeStart", e.ArrowTypeStart)
	if e.CSSCompiledStyles != nil {
		o.Set("cssCompiledStyles", strs(e.CSSCompiledStyles))
	}
	setStr(o, "defaultInterpolate", e.DefaultInterpolate)
	setStr(o, "end", e.End)
	setStr(o, "interpolate", e.Interpolate)
	setStr(o, "labelType", e.LabelType)
	setNum(o, "length", e.Length)
	setStr(o, "start", e.Start)
	setStr(o, "stroke", e.Stroke)
	setStr(o, "text", e.Text)
	setStr(o, "type", e.Type)
	setStr(o, "startLabelRight", e.StartLabelRight)
	setStr(o, "endLabelLeft", e.EndLabelLeft)
	if e.TerminalLabelSizes != nil {
		sizes := js.NewObj[any]()
		for _, k := range terminalKeys(e.TerminalLabelSizes) {
			s := e.TerminalLabelSizes[k]
			sizes.Set(k, obj("width", s.Width, "height", s.Height))
		}
		o.Set("terminalLabelSizes", sizes)
	}
	if e.TerminalLabelCenters != nil {
		centers := js.NewObj[any]()
		for _, k := range terminalKeys(e.TerminalLabelCenters) {
			c := e.TerminalLabelCenters[k]
			centers.Set(k, obj("x", c.X, "y", c.Y))
		}
		o.Set("terminalLabelCenters", centers)
	}
	setStr(o, "curve", e.Curve)
	setStr(o, "labelpos", e.Labelpos)
	if e.LabelStyleIsText {
		o.Set("labelStyle", e.LabelStyleText)
	} else if e.LabelStyle != nil {
		o.Set("labelStyle", strs(e.LabelStyle))
	}
	setNum(o, "minlen", e.Minlen)
	setStr(o, "pattern", e.Pattern)
	setStr(o, "thickness", e.Thickness)
	setStr(o, "look", e.Look)
	setTrue(o, "isUserDefinedId", e.IsUserDefinedID)
	setTrue(o, "showPoints", e.ShowPoints)
	setNum(o, "width", e.Width)
	setNum(o, "height", e.Height)
	setNum(o, "x", e.X)
	setNum(o, "y", e.Y)
	if e.Points != nil {
		points := make([]any, len(e.Points))
		for i, p := range e.Points {
			points[i] = obj("x", p.X, "y", p.Y)
		}
		o.Set("points", points)
	}
	setStr(o, "parentId", e.ParentID)
	setStr(o, "dir", e.Dir)
	setStr(o, "source", e.Source)
	setStr(o, "target", e.Target)
	setNum(o, "depth", e.Depth)
	setTrue(o, "isLabelEdge", e.IsLabelEdge)
	setStr(o, "labelNodeId", e.LabelNodeID)
	setTrue(o, "isLayoutOnly", e.IsLayoutOnly)
	setStr(o, "startLabelLeft", e.StartLabelLeft)
	setStr(o, "endLabelRight", e.EndLabelRight)
	setStr(o, "toCluster", e.ToCluster)
	setStr(o, "fromCluster", e.FromCluster)
	if e.CSSStyles != nil {
		o.Set("cssStyles", strs(e.CSSStyles))
	}
	setExtra(o, e.Extra)
	return o
}

// newLayoutObjects is the JavaScript form of the layout data.
func newLayoutObjects(data4Layout *ru.LayoutData) *layoutObjects {
	lo := &layoutObjects{config: data4Layout.Config}
	if data4Layout.Direction != "" {
		lo.direction = data4Layout.Direction
	}
	lo.nodes = make([]*js.Obj[any], len(data4Layout.Nodes))
	for i, n := range data4Layout.Nodes {
		lo.nodes[i] = nodeToObj(n)
	}
	lo.edges = make([]*js.Obj[any], len(data4Layout.Edges))
	for i, e := range data4Layout.Edges {
		lo.edges[i] = edgeToObj(e)
	}
	return lo
}
