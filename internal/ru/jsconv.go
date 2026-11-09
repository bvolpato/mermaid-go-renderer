package ru

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// NodeFromJS builds a Node from a JavaScript object with the property
// names of types.ts. Tests use it to feed the same node to the Go code and
// to upstream. Properties that have no field go to Extra.
func NodeFromJS(o *js.Obj[any]) *Node {
	n := NewNode("")
	if o == nil {
		return n
	}
	str := func(v any) string {
		if js.IsNullish(v) {
			return ""
		}
		return js.ToString(v)
	}
	num := func(v any) float64 {
		if js.IsNullish(v) {
			return Undef
		}
		return js.ToNumber(v)
	}
	strs := func(v any) []string {
		arr, ok := v.([]any)
		if !ok {
			return nil
		}
		out := make([]string, len(arr))
		for i, e := range arr {
			out[i] = js.ToString(e)
		}
		return out
	}
	o.Each(func(k string, v any) {
		switch k {
		case "id":
			n.ID = str(v)
		case "label":
			n.Label, n.HasLabel = str(v), !js.IsNullish(v)
		case "description":
			n.Description = strs(v)
		case "stereotype":
			n.Stereotype, n.HasStereotype = str(v), !js.IsNullish(v)
		case "parentId":
			n.ParentID = str(v)
		case "position":
			n.Position = str(v)
		case "cssStyles":
			n.CSSStyles, n.CSSStylesSet = strs(v), !js.IsNullish(v)
		case "cssCompiledStyles":
			n.CSSCompiledStyles = strs(v)
		case "cssClasses":
			n.CSSClasses, n.CSSClassesSet = str(v), !js.IsNullish(v)
		case "labelStyle":
			n.LabelStyle = str(v)
		case "labelType":
			n.LabelType = str(v)
		case "domId":
			n.DomID = str(v)
		case "dir":
			n.Dir = str(v)
		case "haveCallback":
			n.HaveCallback = js.Truthy(v)
		case "link":
			n.Link = str(v)
		case "linkTarget":
			n.LinkTarget = str(v)
		case "tooltip":
			n.Tooltip = str(v)
		case "padding":
			n.Padding = num(v)
		case "isGroup":
			n.IsGroup = js.Truthy(v)
		case "width":
			n.Width = num(v)
		case "height":
			n.Height = num(v)
		case "wrappingWidth":
			n.WrappingWidth = num(v)
		case "minWidth":
			n.MinWidth = num(v)
		case "spreadPorts":
			n.SpreadPorts = js.Truthy(v)
		case "rx":
			n.Rx = num(v)
		case "ry":
			n.Ry = num(v)
		case "useHtmlLabels":
			n.UseHtmlLabels, n.UseHtmlLabelsSet = js.Truthy(v), !js.IsNullish(v)
		case "centerLabel":
			n.CenterLabel = js.Truthy(v)
		case "backgroundColor":
			n.BackgroundColor = str(v)
		case "borderColor":
			n.BorderColor = str(v)
		case "borderStyle":
			n.BorderStyle = str(v)
		case "borderWidth":
			n.BorderWidth = num(v)
		case "labelTextColor":
			n.LabelTextColor = str(v)
		case "labelPaddingX":
			n.LabelPaddingX = num(v)
		case "labelPaddingY":
			n.LabelPaddingY = num(v)
		case "x":
			n.X = num(v)
		case "y":
			n.Y = num(v)
		case "look":
			n.Look = str(v)
		case "icon":
			n.Icon = str(v)
		case "pos":
			n.Pos = str(v)
		case "img":
			n.Img = str(v)
		case "assetWidth":
			n.AssetWidth = num(v)
		case "assetHeight":
			n.AssetHeight = num(v)
		case "defaultWidth":
			n.DefaultWidth = num(v)
		case "imageAspectRatio":
			n.ImageAspectRatio = num(v)
		case "constraint":
			n.Constraint = str(v)
		case "metadata":
			n.Metadata = js.AsObj(v)
		case "layer":
			n.Layer = num(v)
		case "order":
			n.Order = num(v)
		case "children":
			if arr, ok := v.([]any); ok {
				for _, c := range arr {
					n.Children = append(n.Children, NodeFromJS(js.AsObj(c)))
				}
			}
		case "nodeId":
			n.NodeID = str(v)
		case "level":
			n.Level = num(v)
		case "descr":
			n.Descr = str(v)
		case "type":
			n.Type = num(v)
		case "radius":
			n.Radius = num(v)
		case "taper":
			n.Taper = num(v)
		case "stroke":
			n.Stroke = str(v)
		case "colorIndex":
			n.ColorIndex = num(v)
		case "shape":
			n.Shape = str(v)
		case "isEdgeLabel":
			n.IsEdgeLabel = js.Truthy(v)
		case "edgeStart":
			n.EdgeStart = str(v)
		case "edgeEnd":
			n.EdgeEnd = str(v)
		case "isDummy":
			n.IsDummy = js.Truthy(v)
		default:
			n.Set(k, v)
		}
	})
	n.LabelStyleUndefined = js.IsNullish(o.Get("labelStyle"))
	return n
}
