// This file ports no upstream file. It converts the layout data to the
// JavaScript objects of rendering-util/types.ts, for the comparison with
// the db of upstream.

package flowchart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// jsonObj builds a JavaScript object. A property that is undefined is absent.
type jsonObj struct{ o *js.Obj[any] }

func (j jsonObj) str(key, v string) {
	if v != "" {
		j.o.Set(key, v)
	}
}

func (j jsonObj) strSet(key, v string, set bool) {
	if set {
		j.o.Set(key, v)
	}
}

func (j jsonObj) num(key string, v float64) {
	if v == v {
		j.o.Set(key, v)
	}
}

func (j jsonObj) flag(key string, v bool) {
	if v {
		j.o.Set(key, true)
	}
}

func (j jsonObj) list(key string, v []string) {
	if v == nil {
		return
	}
	arr := make([]any, len(v))
	for i, s := range v {
		arr[i] = s
	}
	j.o.Set(key, arr)
}

// extra adds the properties of Extra. They win over the fields: Extra has
// the JavaScript value of a property that the field cannot hold.
func (j jsonObj) extra(extra map[string]any) {
	for _, k := range sortedKeys(extra) {
		j.o.Set(k, extra[k])
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	js.SortStrings(keys)
	return keys
}

// NodeJSON returns a layout node as a JavaScript object with the property
// names of Node in rendering-util/types.ts.
//
// A property that is undefined is absent. The node model has no separate
// state for "undefined": an empty string, a false boolean, a NaN number
// and a nil list stand for it, except for label, cssStyles and cssClasses,
// which have a field that says whether the property is set. So a property
// that upstream sets to "" or to false is absent here too.
func NodeJSON(n *ru.Node) *js.Obj[any] {
	j := jsonObj{js.NewObj[any]()}
	j.o.Set("id", n.ID)
	j.strSet("label", n.Label, n.HasLabel)
	j.list("description", n.Description)
	j.strSet("stereotype", n.Stereotype, n.HasStereotype)
	j.str("parentId", n.ParentID)
	j.str("position", n.Position)
	if n.CSSStylesSet {
		list := n.CSSStyles
		if list == nil {
			list = []string{}
		}
		j.list("cssStyles", list)
	}
	j.list("cssCompiledStyles", n.CSSCompiledStyles)
	j.strSet("cssClasses", n.CSSClasses, n.CSSClassesSet)
	j.str("labelStyle", n.LabelStyle)
	j.str("labelType", n.LabelType)
	j.str("domId", n.DomID)
	j.str("dir", n.Dir)
	j.flag("haveCallback", n.HaveCallback)
	j.str("link", n.Link)
	j.str("linkTarget", n.LinkTarget)
	j.str("tooltip", n.Tooltip)
	j.num("padding", n.Padding)
	j.flag("isGroup", n.IsGroup)
	j.num("width", n.Width)
	j.num("height", n.Height)
	j.num("wrappingWidth", n.WrappingWidth)
	j.num("minWidth", n.MinWidth)
	j.flag("spreadPorts", n.SpreadPorts)
	j.num("rx", n.Rx)
	j.num("ry", n.Ry)
	j.flag("useHtmlLabels", n.UseHtmlLabels)
	j.flag("centerLabel", n.CenterLabel)
	j.str("backgroundColor", n.BackgroundColor)
	j.str("borderColor", n.BorderColor)
	j.str("borderStyle", n.BorderStyle)
	j.num("borderWidth", n.BorderWidth)
	j.str("labelTextColor", n.LabelTextColor)
	j.num("labelPaddingX", n.LabelPaddingX)
	j.num("labelPaddingY", n.LabelPaddingY)
	j.num("x", n.X)
	j.num("y", n.Y)
	j.str("look", n.Look)
	j.str("icon", n.Icon)
	j.str("pos", n.Pos)
	j.str("img", n.Img)
	j.num("assetWidth", n.AssetWidth)
	j.num("assetHeight", n.AssetHeight)
	j.num("defaultWidth", n.DefaultWidth)
	j.num("imageAspectRatio", n.ImageAspectRatio)
	j.str("constraint", n.Constraint)
	if n.Metadata != nil {
		j.o.Set("metadata", n.Metadata)
	}
	j.num("layer", n.Layer)
	j.num("order", n.Order)
	if n.Children != nil {
		children := make([]any, len(n.Children))
		for i, c := range n.Children {
			children[i] = NodeJSON(c)
		}
		j.o.Set("children", children)
	}
	j.str("nodeId", n.NodeID)
	j.num("level", n.Level)
	j.str("descr", n.Descr)
	j.num("type", n.Type)
	j.num("radius", n.Radius)
	j.num("taper", n.Taper)
	j.str("stroke", n.Stroke)
	j.num("colorIndex", n.ColorIndex)
	j.str("shape", n.Shape)
	j.flag("isEdgeLabel", n.IsEdgeLabel)
	j.str("edgeStart", n.EdgeStart)
	j.str("edgeEnd", n.EdgeEnd)
	j.flag("isDummy", n.IsDummy)
	j.extra(n.Extra)
	return j.o
}

// EdgeJSON returns a layout edge as a JavaScript object with the property
// names of Edge in rendering-util/types.ts. See NodeJSON for the properties
// that are undefined. The properties with a field that says whether they
// are set are label and animate.
func EdgeJSON(e *ru.Edge) *js.Obj[any] {
	j := jsonObj{js.NewObj[any]()}
	j.o.Set("id", e.ID)
	j.strSet("label", e.Label, e.HasLabel)
	j.str("classes", e.Classes)
	j.list("style", e.Style)
	if e.HasAnimate {
		j.o.Set("animate", e.Animate)
	}
	j.str("animation", e.Animation)
	if e.Metadata != nil {
		j.o.Set("metadata", e.Metadata)
	}
	j.str("arrowhead", e.Arrowhead)
	j.str("arrowheadStyle", e.ArrowheadStyle)
	j.str("arrowTypeEnd", e.ArrowTypeEnd)
	j.str("arrowTypeStart", e.ArrowTypeStart)
	j.list("cssCompiledStyles", e.CSSCompiledStyles)
	j.str("defaultInterpolate", e.DefaultInterpolate)
	j.str("end", e.End)
	j.str("interpolate", e.Interpolate)
	j.str("labelType", e.LabelType)
	j.num("length", e.Length)
	j.str("start", e.Start)
	j.str("stroke", e.Stroke)
	j.str("text", e.Text)
	j.str("type", e.Type)
	j.str("startLabelRight", e.StartLabelRight)
	j.str("endLabelLeft", e.EndLabelLeft)
	j.str("curve", e.Curve)
	j.str("labelpos", e.Labelpos)
	j.list("labelStyle", e.LabelStyle)
	j.num("minlen", e.Minlen)
	j.str("pattern", e.Pattern)
	j.str("thickness", e.Thickness)
	j.str("look", e.Look)
	j.flag("isUserDefinedId", e.IsUserDefinedID)
	j.flag("showPoints", e.ShowPoints)
	j.num("width", e.Width)
	j.num("height", e.Height)
	j.num("x", e.X)
	j.num("y", e.Y)
	if e.Points != nil {
		points := make([]any, len(e.Points))
		for i, p := range e.Points {
			o := js.NewObj[any]()
			o.Set("x", p.X)
			o.Set("y", p.Y)
			points[i] = o
		}
		j.o.Set("points", points)
	}
	j.str("parentId", e.ParentID)
	j.str("dir", e.Dir)
	j.str("source", e.Source)
	j.str("target", e.Target)
	j.num("depth", e.Depth)
	j.flag("isLabelEdge", e.IsLabelEdge)
	j.str("labelNodeId", e.LabelNodeID)
	j.flag("isLayoutOnly", e.IsLayoutOnly)
	j.extra(e.Extra)
	return j.o
}
