// Package set3 holds the third group of node shapes of the unified
// renderer: the shapes that shapes.ts does not document (states, notes,
// block shapes, the collapsed subgraph, icon and image nodes), and the
// label helper of C4 nodes. Each handler registers itself under the name
// that it has in rendering-elements/shapes.ts.
package set3

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// This file ports no upstream file. It has the JavaScript idioms that the
// shape files of this package repeat.

// roughOptions is an object literal of rough options: key, value, key,
// value and so on.
func roughOptions(kv ...any) *js.Obj[any] {
	o := js.NewObj[any]()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1])
	}
	return o
}

// spread is { ...options, key: value, ... }: a copy of the options with
// the given properties assigned on top.
func spread(options *js.Obj[any], kv ...any) *js.Obj[any] {
	o := js.NewObj[any]()
	options.Each(func(k string, v any) { o.Set(k, v) })
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1])
	}
	return o
}

// hasCSSStyles is the truthiness of node.cssStyles. An array is truthy,
// also when it is empty.
func hasCSSStyles(node *ru.Node) bool {
	return node.CSSStylesSet || len(node.CSSStyles) > 0
}

// cssStylesString is node.cssStyles as an attribute value: the elements
// joined with commas.
func cssStylesString(node *ru.Node) string {
	return strings.Join(node.CSSStyles, ",")
}

// nodeID is `node.domId || node.id`.
func nodeID(node *ru.Node) string {
	if node.DomID != "" {
		return node.DomID
	}
	return node.ID
}

// attrValue is a dynamic value as the argument of selection.attr:
// undefined and null remove the attribute (nil), and other values are
// converted to a string.
func attrValue(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return js.ToString(v)
}

// applyPathStyles is the block that the rough shapes repeat:
//
//	if (cssStyles && node.look !== 'handDrawn') {
//	  shape.selectAll('path').attr('style', cssStyles);
//	}
//	if (nodeStyles && node.look !== 'handDrawn') {
//	  shape.selectAll('path').attr('style', nodeStyles);
//	}
func applyPathStyles(shape *dom.Node, node *ru.Node, nodeStyles string) {
	if hasCSSStyles(node) && node.Look != "handDrawn" {
		shape.SelectAll("path").Attr("style", cssStylesString(node))
	}
	if nodeStyles != "" && node.Look != "handDrawn" {
		shape.SelectAll("path").Attr("style", nodeStyles)
	}
}

// labelOffsetX is `-bbox.width / 2 - (bbox.x - (bbox.left ?? 0))`.
func labelOffsetX(bbox ru.LabelBBox) float64 {
	return float64(-bbox.Width/2) - (bbox.X - bbox.LeftOr0())
}

// labelHelper is labelHelper(parent, node, classes) for the shapes that do
// not assign node.labelStyle before they make the label: labelRect,
// composite, block_arrow and collapsedGroup. For a node that the diagram
// made with no label style, upstream prints no style attribute on the
// label group (handleUndefinedAttr). ru.LabelHelper always prints it, so
// the attribute is removed here.
func labelHelper(ctx *render.Context, parent *dom.Node, node *ru.Node, classes string) (*ru.LabelResult, error) {
	res, err := ru.LabelHelper(ctx, parent, node, classes)
	if err != nil {
		return nil, err
	}
	if node.LabelStyleUndefined {
		res.Label.Attr("style", nil)
	}
	return res, nil
}
