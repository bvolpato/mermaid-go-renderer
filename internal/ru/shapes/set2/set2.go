// Package set2 holds the second group of node shapes of the unified
// renderer. Each handler registers itself under the name that it has in
// rendering-elements/shapes.ts.
package set2

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/rough"
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

// solidOptions is userNodeOverrides(node, overrides) followed by the block
// that most shapes have: a look that is not handDrawn draws with roughness
// 0 and a solid fill.
func solidOptions(ctx *render.Context, node *ru.Node, overrides *js.Obj[any]) *js.Obj[any] {
	options := ru.UserNodeOverrides(ctx, node, overrides)
	if node.Look != "handDrawn" {
		options.Set("roughness", 0.0)
		options.Set("fillStyle", "solid")
	}
	return options
}

// truthy is the truthiness of a number: false for undefined, NaN and 0.
func truthy(v float64) bool {
	return v == v && v != 0
}

// tern is `cond ? a : b` for numbers.
func tern(cond bool, a, b float64) float64 {
	if cond {
		return a
	}
	return b
}

// insertShapeGroup is the start of the shapes that draw no label:
//
//	parent.insert('g').attr('class', getNodeClasses(node)).attr('id', node.domId ?? node.id)
func insertShapeGroup(parent *dom.Node, node *ru.Node) *dom.Node {
	// An id that is undefined gives no attribute.
	var id any
	if node.DomID != "" {
		id = node.DomID
	} else if node.ID != "" {
		id = node.ID
	}
	return parent.Insert("g").Attr("class", ru.GetNodeClasses(node)).Attr("id", id)
}

// hasCSSStyles is the truthiness of node.cssStyles. An array is truthy,
// also when it is empty.
func hasCSSStyles(node *ru.Node) bool {
	return node.CSSStylesSet || len(node.CSSStyles) > 0
}

// childPaths is selection.selectChildren('path').
func childPaths(n *dom.Node) dom.Nodes {
	var out dom.Nodes
	for _, c := range n.Children() {
		if c.Matches("path") {
			out = append(out, c)
		}
	}
	return out
}

// stylePaths is the block that follows the drawing of a shape:
//
//	if (cssStyles && node.look !== 'handDrawn') { paths.attr('style', cssStyles); }
//	if (nodeStyles && node.look !== 'handDrawn') { paths.attr('style', nodeStyles); }
func stylePaths(paths dom.Nodes, node *ru.Node, nodeStyles string) {
	if hasCSSStyles(node) && node.Look != "handDrawn" {
		paths.Attr("style", ru.CSSStylesAttr(node))
	}
	if nodeStyles != "" && node.Look != "handDrawn" {
		paths.Attr("style", nodeStyles)
	}
}

// roughPoints is points.map((p) => [p.x, p.y]).
func roughPoints(points []ru.Point) []rough.Point {
	out := make([]rough.Point, len(points))
	for i, p := range points {
		out[i] = rough.Point{p.X, p.Y}
	}
	return out
}

// polygonIntersect is the intersect function of the shapes that call
// intersect.polygon(node, points, point).
func polygonIntersect(node *ru.Node, points []ru.Point) func(ru.Point) ru.Point {
	return func(point ru.Point) ru.Point {
		return ru.IntersectPolygon(ru.NodeBounds(node), points, point)
	}
}
