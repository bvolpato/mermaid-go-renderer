package set1

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The helpers of this file stand for JavaScript expressions that several
// shapes of this package share. They port no upstream function.

// hasCSSStyles is the truth value of `node.cssStyles`: an array is truthy,
// also when it is empty.
func hasCSSStyles(node *ru.Node) bool {
	return node.CSSStylesSet || len(node.CSSStyles) > 0
}

// cssStylesString is `node.cssStyles` as an attribute value: the elements
// joined with commas.
func cssStylesString(node *ru.Node) string {
	return strings.Join(node.CSSStyles, ",")
}

// selectChildren is selection.selectChildren(tag): the child elements with
// the given tag name.
func selectChildren(n *dom.Node, tag string) dom.Nodes {
	var out dom.Nodes
	for _, c := range n.Children() {
		if c.Tag == tag {
			out = append(out, c)
		}
	}
	return out
}

// ownerSVGElementID is the id of parent.node()?.ownerSVGElement, the nearest
// svg ancestor, or an empty string.
func ownerSVGElementID(parent *dom.Node) string {
	if parent == nil {
		return ""
	}
	for p := parent.Parent; p != nil; p = p.Parent {
		if p.Type == dom.ElementNode && p.NS == dom.NSSVG && p.Tag == "svg" {
			return p.AttrOr("id", "")
		}
	}
	return ""
}

// spread is `{ ...options }`: a shallow copy of an options object.
func spread(options *js.Obj[any]) *js.Obj[any] {
	out := js.NewObj[any]()
	options.Each(func(k string, v any) { out.Set(k, v) })
	return out
}

// nodeID is `node.domId || node.id`.
func nodeID(node *ru.Node) string {
	if node.DomID != "" {
		return node.DomID
	}
	return node.ID
}

// themeVariable is `themeVariables?.[name]` of the render options.
func themeVariable(options *ru.ShapeRenderOptions, name string) any {
	if options == nil {
		return nil
	}
	return js.Path(options.Config, "themeVariables", name)
}

// accentColor is `themeVariables?.nodeBorder ?? themeVariables?.lineColor ??
// 'currentColor'`, as text.
func accentColor(options *ru.ShapeRenderOptions) string {
	accent := themeVariable(options, "nodeBorder")
	if js.IsNullish(accent) {
		accent = themeVariable(options, "lineColor")
	}
	if js.IsNullish(accent) {
		accent = "currentColor"
	}
	return js.ToString(accent)
}

// labelX is `-(bbox.width / 2) - (bbox.x - (bbox.left ?? 0))`.
func labelX(bbox ru.LabelBBox) float64 {
	return -float64(bbox.Width/2) - (bbox.X - bbox.LeftOr0())
}

// polygonIntersect sets node.intersect to intersect.polygon(node, points,
// point).
func polygonIntersect(node *ru.Node, points []ru.Point) {
	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectPolygon(ru.NodeBounds(node), points, point)
	}
}

// rectIntersect sets node.intersect to intersect.rect(node, point).
func rectIntersect(node *ru.Node) {
	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.NodeBounds(node), point)
	}
}
