// This file ports no upstream file. Upstream hands the node and edge
// objects of the db to the unified renderer as they are. Here they are
// converted to the layout model of internal/ru.

package state

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// A layout node or edge holds strings, numbers and booleans. A property of
// upstream that comes from the config can hold a value of another type. The
// functions below convert such a value for the field, and keep the
// JavaScript value under the name of the property in Extra, so that the
// value is not lost.

type extraSetter interface{ Set(key string, v any) }

// dynString converts a property value to a string field.
func dynString(x extraSetter, key string, v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	x.Set(key, v)
	return js.ToString(v)
}

// dynNumber converts a property value to a number field.
func dynNumber(x extraSetter, key string, v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	x.Set(key, v)
	return js.ToNumber(v)
}

// dynStrings converts an array of strings to a list field. The result is
// not nil, so that an empty array differs from undefined.
func dynStrings(x extraSetter, key string, v any) []string {
	list, ok := v.([]any)
	if !ok {
		x.Set(key, v)
		return nil
	}
	out := make([]string, len(list))
	for i, e := range list {
		s, isString := e.(string)
		if !isString {
			x.Set(key, v)
			s = js.ToString(e)
		}
		out[i] = s
	}
	return out
}

// layoutNode converts a node object of the db (NodeData of stateDb.ts) to a
// layout node. A property that is undefined keeps the undefined value of
// the field.
func layoutNode(o *js.Obj[any]) *ru.Node {
	id, _ := o.Get("id").(string)
	n := ru.NewNode(id)
	o.Each(func(key string, v any) {
		if v == nil {
			return
		}
		switch key {
		case "id":
			if _, ok := v.(string); !ok {
				n.Set(key, v)
			}
		case "labelStyle":
			n.LabelStyle = dynString(n, key, v)
		case "shape":
			n.Shape = dynString(n, key, v)
		case "label":
			if s, ok := v.(string); ok {
				n.Label, n.HasLabel = s, true
			} else {
				n.Set(key, v)
			}
		case "cssClasses":
			n.CSSClasses, n.CSSClassesSet = dynString(n, key, v), true
		case "cssCompiledStyles":
			n.CSSCompiledStyles = dynStrings(n, key, v)
		case "cssStyles":
			n.CSSStyles, n.CSSStylesSet = dynStrings(n, key, v), true
		case "dir":
			n.Dir = dynString(n, key, v)
		case "domId":
			n.DomID = dynString(n, key, v)
		case "type":
			// The type of a state node is a text ("group" or "node"). The
			// Type field is the number of another diagram.
			n.Set(key, v)
		case "isGroup":
			n.IsGroup = js.Truthy(v)
		case "colorIndex":
			n.ColorIndex = dynNumber(n, key, v)
		case "padding":
			n.Padding = dynNumber(n, key, v)
		case "rx":
			n.Rx = dynNumber(n, key, v)
		case "ry":
			n.Ry = dynNumber(n, key, v)
		case "look":
			n.Look = dynString(n, key, v)
		case "labelType":
			n.LabelType = dynString(n, key, v)
		case "parentId":
			n.ParentID = dynString(n, key, v)
		case "centerLabel":
			n.CenterLabel = js.Truthy(v)
		case "position":
			n.Position = dynString(n, key, v)
		case "description":
			n.Description = dynStrings(n, key, v)
		case "wrappingWidth":
			n.WrappingWidth = dynNumber(n, key, v)
		case "minWidth":
			n.MinWidth = dynNumber(n, key, v)
		default:
			n.Set(key, v)
		}
	})
	return n
}

// layoutEdge converts an edge object of the db (Edge of stateDb.ts) to a
// layout edge.
func layoutEdge(o *js.Obj[any]) *ru.Edge {
	id, _ := o.Get("id").(string)
	e := ru.NewEdge(id)
	e.StyleUndefined = o.Get("style") == nil
	o.Each(func(key string, v any) {
		if v == nil {
			return
		}
		switch key {
		case "id":
			if _, ok := v.(string); !ok {
				e.Set(key, v)
			}
		case "start":
			e.Start = dynString(e, key, v)
		case "end":
			e.End = dynString(e, key, v)
		case "arrowhead":
			e.Arrowhead = dynString(e, key, v)
		case "arrowTypeEnd":
			e.ArrowTypeEnd = dynString(e, key, v)
		case "style":
			// The style is a text. insertEdge makes the array [style] of it.
			e.Style = []string{dynString(e, key, v)}
		case "labelStyle":
			// The label style is a text, and not an array.
			e.LabelStyleText, e.LabelStyleIsText = dynString(e, key, v), true
		case "label":
			e.Label, e.HasLabel = dynString(e, key, v), true
		case "arrowheadStyle":
			e.ArrowheadStyle = dynString(e, key, v)
		case "labelpos":
			e.Labelpos = dynString(e, key, v)
		case "labelType":
			e.LabelType = dynString(e, key, v)
		case "thickness":
			e.Thickness = dynString(e, key, v)
		case "classes":
			e.Classes = dynString(e, key, v)
		case "look":
			e.Look = dynString(e, key, v)
		case "pattern":
			e.Pattern = dynString(e, key, v)
		default:
			e.Set(key, v)
		}
	})
	return e
}
