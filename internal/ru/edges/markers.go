// Ported from mermaid 12.2.0, rendering-util/rendering-elements/markers.js
// (MIT).

package edges

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// The marker functions keep the statement order and the attribute order of
// upstream.

// InsertMarkers is insertMarkers(elem, markerArray, type, id): it appends
// the marker definitions that the diagram needs to elem. typ is the
// diagram type and id is the id of the svg; both are part of the marker
// ids. A marker name that upstream does not know is an error (upstream
// throws a TypeError).
func InsertMarkers(ctx *render.Context, elem *dom.Node, markerArray []string, typ, id string) error {
	// Only add the number of markers that the diagram needs
	for _, markerName := range markerArray {
		marker, ok := markers[markerName]
		if !ok {
			return fmt.Errorf("TypeError: markers[markerName] is not a function (%q)", markerName)
		}
		marker(ctx, elem, typ, id)
	}
	return nil
}

// orValue is `v ?? d` for an attribute value: the value as a string, or d
// when it is undefined or null.
func orValue(v any, d string) string {
	if js.IsNullish(v) {
		return d
	}
	return js.ToString(v)
}

func extension(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-extensionStart").
		Attr("class", "marker extension "+typ).
		Attr("refX", 18).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M 1,7 L18,13 V 1 Z")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-extensionEnd").
		Attr("class", "marker extension "+typ).
		Attr("refX", 1).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M 1,1 V 13 L18,7 Z")

	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-extensionStart-margin").
		Attr("class", "marker extension "+typ).
		Attr("refX", 18).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Attr("viewBox", "0 0 20 14").
		Append("polygon").
		Attr("points", "10,7 18,13 18,1").
		Style("stroke-width", 2).
		Style("stroke-dasharray", "0")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-extensionEnd-margin").
		Attr("class", "marker extension "+typ).
		Attr("refX", 9).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Attr("viewBox", "0 0 20 14").
		Append("polygon").
		Attr("points", "10,1 10,13 18,7").
		Style("stroke-width", 2).
		Style("stroke-dasharray", "0")
}

func composition(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-compositionStart").
		Attr("class", "marker composition "+typ).
		Attr("refX", 18).
		Attr("refY", 7).
		Attr("markerWidth", 190).
		Attr("markerHeight", 240).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M 18,7 L9,13 L1,7 L9,1 Z")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-compositionEnd").
		Attr("class", "marker composition "+typ).
		Attr("refX", 1).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M 18,7 L9,13 L1,7 L9,1 Z")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-compositionStart-margin").
		Attr("class", "marker composition "+typ).
		Attr("refX", 15).
		Attr("refY", 7).
		Attr("markerWidth", 190).
		Attr("markerHeight", 240).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Style("stroke-width", 0).
		Attr("viewBox", "0 0 15 15").
		Attr("d", "M 18,7 L9,13 L1,7 L9,1 Z")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-compositionEnd-margin").
		Attr("class", "marker composition "+typ).
		Attr("refX", 3.5).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Style("stroke-width", 0).
		Attr("d", "M 18,7 L9,13 L1,7 L9,1 Z")
}
func aggregation(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-aggregationStart").
		Attr("class", "marker aggregation "+typ).
		Attr("refX", 18).
		Attr("refY", 7).
		Attr("markerWidth", 190).
		Attr("markerHeight", 240).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M 18,7 L9,13 L1,7 L9,1 Z")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-aggregationEnd").
		Attr("class", "marker aggregation "+typ).
		Attr("refX", 1).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M 18,7 L9,13 L1,7 L9,1 Z")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-aggregationStart-margin").
		Attr("class", "marker aggregation "+typ).
		Attr("refX", 15).
		Attr("refY", 7).
		Attr("markerWidth", 190).
		Attr("markerHeight", 240).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Style("stroke-width", 2).
		Attr("d", "M 18,7 L9,13 L1,7 L9,1 Z")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-aggregationEnd-margin").
		Attr("class", "marker aggregation "+typ).
		Attr("refX", 1).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Style("stroke-width", 2).
		Attr("d", "M 18,7 L9,13 L1,7 L9,1 Z")
}
func dependency(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-dependencyStart").
		Attr("class", "marker dependency "+typ).
		Attr("refX", 6).
		Attr("refY", 7).
		Attr("markerWidth", 190).
		Attr("markerHeight", 240).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M 5,7 L9,13 L1,7 L9,1 Z")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-dependencyEnd").
		Attr("class", "marker dependency "+typ).
		Attr("refX", 13).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M 18,7 L9,13 L14,7 L9,1 Z")
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-dependencyStart-margin").
		Attr("class", "marker dependency "+typ).
		Attr("refX", 4).
		Attr("refY", 7).
		Attr("markerWidth", 190).
		Attr("markerHeight", 240).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Style("stroke-width", 0).
		Attr("d", "M 5,7 L9,13 L1,7 L9,1 Z")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-dependencyEnd-margin").
		Attr("class", "marker dependency "+typ).
		Attr("refX", 16).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 28).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Style("stroke-width", 0).
		Attr("d", "M 18,7 L9,13 L14,7 L9,1 Z")
}
func lollipop(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-lollipopStart").
		Attr("class", "marker lollipop "+typ).
		Attr("refX", 13).
		Attr("refY", 7).
		Attr("markerWidth", 190).
		Attr("markerHeight", 240).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("circle").
		Attr("fill", "transparent").
		Attr("cx", 7).
		Attr("cy", 7).
		Attr("r", 6)

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-lollipopEnd").
		Attr("class", "marker lollipop "+typ).
		Attr("refX", 1).
		Attr("refY", 7).
		Attr("markerWidth", 190).
		Attr("markerHeight", 240).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("circle").
		Attr("fill", "transparent").
		Attr("cx", 7).
		Attr("cy", 7).
		Attr("r", 6)
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-lollipopStart-margin").
		Attr("class", "marker lollipop "+typ).
		Attr("refX", 13).
		Attr("refY", 7).
		Attr("markerWidth", 190).
		Attr("markerHeight", 240).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("circle").
		Attr("fill", "transparent").
		Attr("cx", 7).
		Attr("cy", 7).
		Attr("r", 6).
		Attr("stroke-width", 2)

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-lollipopEnd-margin").
		Attr("class", "marker lollipop "+typ).
		Attr("refX", 1).
		Attr("refY", 7).
		Attr("markerWidth", 190).
		Attr("markerHeight", 240).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("circle").
		Attr("fill", "transparent").
		Attr("cx", 7).
		Attr("cy", 7).
		Attr("r", 6).
		Attr("stroke-width", 2)
}
func point(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-pointEnd").
		Attr("class", "marker "+typ).
		Attr("viewBox", "0 0 10 10").
		Attr("refX", 5).
		Attr("refY", 5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 8).
		Attr("markerHeight", 8).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 0 0 L 10 5 L 0 10 z").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 1).
		Style("stroke-dasharray", "1,0")
	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-pointStart").
		Attr("class", "marker "+typ).
		Attr("viewBox", "0 0 10 10").
		Attr("refX", 4.5).
		Attr("refY", 5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 8).
		Attr("markerHeight", 8).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 0 5 L 10 10 L 10 0 z").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 1).
		Style("stroke-dasharray", "1,0")
	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-pointEnd-margin").
		Attr("class", "marker "+typ).
		Attr("viewBox", "0 0 11.5 14").
		Attr("refX", 11.5).
		Attr("refY", 7).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 10.5).
		Attr("markerHeight", 14).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 0 0 L 11.5 7 L 0 14 z").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 0).
		Style("stroke-dasharray", "1,0")
	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-pointStart-margin").
		Attr("class", "marker "+typ).
		Attr("viewBox", "0 0 11.5 14").
		Attr("refX", 1).
		Attr("refY", 7).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 11.5).
		Attr("markerHeight", 14).
		Attr("orient", "auto").
		Append("polygon").
		Attr("points", "0,7 11.5,14 11.5,0").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 0).
		Style("stroke-dasharray", "1,0")
}
func circle(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-circleEnd").
		Attr("class", "marker "+typ).
		Attr("viewBox", "0 0 10 10").
		Attr("refX", 11).
		Attr("refY", 5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 11).
		Attr("markerHeight", 11).
		Attr("orient", "auto").
		Append("circle").
		Attr("cx", "5").
		Attr("cy", "5").
		Attr("r", "5").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 1).
		Style("stroke-dasharray", "1,0")

	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-circleStart").
		Attr("class", "marker "+typ).
		Attr("viewBox", "0 0 10 10").
		Attr("refX", -1).
		Attr("refY", 5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 11).
		Attr("markerHeight", 11).
		Attr("orient", "auto").
		Append("circle").
		Attr("cx", "5").
		Attr("cy", "5").
		Attr("r", "5").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 1).
		Style("stroke-dasharray", "1,0")
	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-circleEnd-margin").
		Attr("class", "marker "+typ).
		Attr("viewBox", "0 0 10 10").
		Attr("refY", 5).
		Attr("refX", 12.25).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 14).
		Attr("markerHeight", 14).
		Attr("orient", "auto").
		Append("circle").
		Attr("cx", "5").
		Attr("cy", "5").
		Attr("r", "5").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 0).
		Style("stroke-dasharray", "1,0")

	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-circleStart-margin").
		Attr("class", "marker "+typ).
		Attr("viewBox", "0 0 10 10").
		Attr("refX", -2).
		Attr("refY", 5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 14).
		Attr("markerHeight", 14).
		Attr("orient", "auto").
		Append("circle").
		Attr("cx", "5").
		Attr("cy", "5").
		Attr("r", "5").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 0).
		Style("stroke-dasharray", "1,0")
}
func cross(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-crossEnd").
		Attr("class", "marker cross "+typ).
		Attr("viewBox", "0 0 11 11").
		Attr("refX", 12).
		Attr("refY", 5.2).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 11).
		Attr("markerHeight", 11).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 1,1 l 9,9 M 10,1 l -9,9").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 2).
		Style("stroke-dasharray", "1,0")

	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-crossStart").
		Attr("class", "marker cross "+typ).
		Attr("viewBox", "0 0 11 11").
		Attr("refX", -1).
		Attr("refY", 5.2).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 11).
		Attr("markerHeight", 11).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 1,1 l 9,9 M 10,1 l -9,9").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 2).
		Style("stroke-dasharray", "1,0")
	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-crossEnd-margin").
		Attr("class", "marker cross "+typ).
		Attr("viewBox", "0 0 15 15").
		Attr("refX", 17.7).
		Attr("refY", 7.5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 12).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 1,1 L 14,14 M 1,14 L 14,1").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 2.5)

	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-crossStart-margin").
		Attr("class", "marker cross "+typ).
		Attr("viewBox", "0 0 15 15").
		Attr("refX", -3.5).
		Attr("refY", 7.5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 12).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 1,1 L 14,14 M 1,14 L 14,1").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 2.5).
		Style("stroke-dasharray", "1,0")
}
func barb(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-barbEnd").
		Attr("refX", 19).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 14).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 19,7 L9,13 L14,7 L9,1 Z")
}
func barbNeo(ctx *render.Context, elem *dom.Node, typ, id string) {
	transitionColor := ctx.Cfg("themeVariables", "transitionColor")
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-barbEnd").
		Attr("refX", 19).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 14).
		Attr("markerUnits", "strokeWidth").
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 19,7 L11,14 L13,7 L11,0 Z")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-barbEnd-margin").
		Attr("refX", 17).
		Attr("refY", 7).
		Attr("markerWidth", 20).
		Attr("markerHeight", 14).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 19,7 L11,14 L13,7 L11,0 Z").
		Attr("fill", js.ToString(transitionColor))
}

// erDiagram specific markers
func onlyOne(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-onlyOneStart").
		Attr("class", "marker onlyOne "+typ).
		Attr("refX", 0).
		Attr("refY", 9).
		Attr("markerWidth", 18).
		Attr("markerHeight", 18).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M9,0 L9,18 M15,0 L15,18")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-onlyOneEnd").
		Attr("class", "marker onlyOne "+typ).
		Attr("refX", 18).
		Attr("refY", 9).
		Attr("markerWidth", 18).
		Attr("markerHeight", 18).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M3,0 L3,18 M9,0 L9,18")
}

func zeroOrOne(ctx *render.Context, elem *dom.Node, typ, id string) {
	startMarker := elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-zeroOrOneStart").
		Attr("class", "marker zeroOrOne "+typ).
		Attr("refX", 0).
		Attr("refY", 9).
		Attr("markerWidth", 30).
		Attr("markerHeight", 18).
		Attr("orient", "auto")
	startMarker.
		Append("circle").
		Attr("fill", "white").
		Attr("cx", 21).
		Attr("cy", 9).
		Attr("r", 6)
	startMarker.Append("path").Attr("d", "M9,0 L9,18")

	endMarker := elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-zeroOrOneEnd").
		Attr("class", "marker zeroOrOne "+typ).
		Attr("refX", 30).
		Attr("refY", 9).
		Attr("markerWidth", 30).
		Attr("markerHeight", 18).
		Attr("orient", "auto")
	endMarker.
		Append("circle").
		Attr("fill", "white").
		Attr("cx", 9).
		Attr("cy", 9).
		Attr("r", 6)
	endMarker.Append("path").Attr("d", "M21,0 L21,18")
}

func oneOrMore(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-oneOrMoreStart").
		Attr("class", "marker oneOrMore "+typ).
		Attr("refX", 18).
		Attr("refY", 18).
		Attr("markerWidth", 45).
		Attr("markerHeight", 36).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M0,18 Q 18,0 36,18 Q 18,36 0,18 M42,9 L42,27")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-oneOrMoreEnd").
		Attr("class", "marker oneOrMore "+typ).
		Attr("refX", 27).
		Attr("refY", 18).
		Attr("markerWidth", 45).
		Attr("markerHeight", 36).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M3,9 L3,27 M9,18 Q27,0 45,18 Q27,36 9,18")
}

func zeroOrMore(ctx *render.Context, elem *dom.Node, typ, id string) {
	startMarker := elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-zeroOrMoreStart").
		Attr("class", "marker zeroOrMore "+typ).
		Attr("refX", 18).
		Attr("refY", 18).
		Attr("markerWidth", 57).
		Attr("markerHeight", 36).
		Attr("orient", "auto")
	startMarker.Append("circle").Attr("fill", "white").Attr("cx", 48).Attr("cy", 18).Attr("r", 6)
	startMarker.Append("path").Attr("d", "M0,18 Q18,0 36,18 Q18,36 0,18")

	endMarker := elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-zeroOrMoreEnd").
		Attr("class", "marker zeroOrMore "+typ).
		Attr("refX", 39).
		Attr("refY", 18).
		Attr("markerWidth", 57).
		Attr("markerHeight", 36).
		Attr("orient", "auto")
	endMarker.Append("circle").Attr("fill", "white").Attr("cx", 9).Attr("cy", 18).Attr("r", 6)
	endMarker.Append("path").Attr("d", "M21,18 Q39,0 57,18 Q39,36 21,18")
}

func onlyOneNeo(ctx *render.Context, elem *dom.Node, typ, id string) {
	strokeWidth := ctx.Cfg("themeVariables", "strokeWidth")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-onlyOneStart").
		Attr("class", "marker onlyOne "+typ).
		Attr("refX", 0).
		Attr("refY", 9).
		Attr("markerWidth", 18).
		Attr("markerHeight", 18).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M9,0 L9,18 M15,0 L15,18").
		Attr("stroke-width", js.ToString(strokeWidth))

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-onlyOneEnd").
		Attr("class", "marker onlyOne "+typ).
		Attr("refX", 18).
		Attr("refY", 9).
		Attr("markerWidth", 18).
		Attr("markerHeight", 18).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M3,0 L3,18 M9,0 L9,18").
		Attr("stroke-width", js.ToString(strokeWidth))
}

func zeroOrOneNeo(ctx *render.Context, elem *dom.Node, typ, id string) {
	strokeWidth := ctx.Cfg("themeVariables", "strokeWidth")
	mainBkg := ctx.Cfg("themeVariables", "mainBkg")
	startMarker := elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-zeroOrOneStart").
		Attr("class", "marker zeroOrOne "+typ).
		Attr("refX", 0).
		Attr("refY", 9).
		Attr("markerWidth", 30).
		Attr("markerHeight", 18).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse")
	startMarker.
		Append("circle").
		Attr("fill", orValue(mainBkg, "white")).
		Attr("cx", 21).
		Attr("cy", 9).
		Attr("stroke-width", js.ToString(strokeWidth)).
		Attr("r", 6)
	startMarker.Append("path").Attr("d", "M9,0 L9,18").Attr("stroke-width", js.ToString(strokeWidth))

	endMarker := elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-zeroOrOneEnd").
		Attr("class", "marker zeroOrOne "+typ).
		Attr("refX", 30).
		Attr("refY", 9).
		Attr("markerWidth", 30).
		Attr("markerHeight", 18).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("orient", "auto")
	endMarker.
		Append("circle").
		Attr("fill", orValue(mainBkg, "white")).
		Attr("cx", 9).
		Attr("cy", 9).
		Attr("stroke-width", js.ToString(strokeWidth)).
		Attr("r", 6)
	endMarker.Append("path").Attr("d", "M21,0 L21,18").Attr("stroke-width", js.ToString(strokeWidth))
}

func oneOrMoreNeo(ctx *render.Context, elem *dom.Node, typ, id string) {
	strokeWidth := ctx.Cfg("themeVariables", "strokeWidth")
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-oneOrMoreStart").
		Attr("class", "marker oneOrMore "+typ).
		Attr("refX", 18).
		Attr("refY", 18).
		Attr("markerWidth", 45).
		Attr("markerHeight", 36).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("path").
		Attr("d", "M0,18 Q 18,0 36,18 Q 18,36 0,18 M42,9 L42,27").
		Attr("stroke-width", js.ToString(strokeWidth))

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-oneOrMoreEnd").
		Attr("class", "marker oneOrMore "+typ).
		Attr("refX", 27).
		Attr("refY", 18).
		Attr("markerWidth", 45).
		Attr("markerHeight", 36).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M3,9 L3,27 M9,18 Q27,0 45,18 Q27,36 9,18").
		Attr("stroke-width", js.ToString(strokeWidth))
}

func zeroOrMoreNeo(ctx *render.Context, elem *dom.Node, typ, id string) {
	strokeWidth := ctx.Cfg("themeVariables", "strokeWidth")
	mainBkg := ctx.Cfg("themeVariables", "mainBkg")
	startMarker := elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-zeroOrMoreStart").
		Attr("class", "marker zeroOrMore "+typ).
		Attr("refX", 18).
		Attr("refY", 18).
		Attr("markerWidth", 57).
		Attr("markerHeight", 36).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("orient", "auto")
	startMarker.
		Append("circle").
		Attr("fill", orValue(mainBkg, "white")).
		Attr("cx", 45.5).
		Attr("cy", 18).
		Attr("r", 6).
		Attr("stroke-width", js.ToString(strokeWidth))
	startMarker.
		Append("path").
		Attr("d", "M0,18 Q18,0 36,18 Q18,36 0,18").
		Attr("stroke-width", js.ToString(strokeWidth))

	endMarker := elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-zeroOrMoreEnd").
		Attr("class", "marker zeroOrMore "+typ).
		Attr("refX", 39).
		Attr("refY", 18).
		Attr("markerWidth", 57).
		Attr("markerHeight", 36).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse")
	endMarker.
		Append("circle").
		Attr("fill", orValue(mainBkg, "white")).
		Attr("cx", 11).
		Attr("cy", 18).
		Attr("r", 6).
		Attr("stroke-width", js.ToString(strokeWidth))
	endMarker.
		Append("path").
		Attr("d", "M21,18 Q39,0 57,18 Q39,36 21,18").
		Attr("stroke-width", js.ToString(strokeWidth))
}

func requirementArrow(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-requirement_arrowEnd").
		Attr("refX", 20).
		Attr("refY", 10).
		Attr("markerWidth", 20).
		Attr("markerHeight", 20).
		Attr("orient", "auto").
		Append("path").
		Attr(
			"d",
			"M0,0\n      L20,10\n      M20,10\n      L0,20",
		)
}

func requirementArrowNeo(ctx *render.Context, elem *dom.Node, typ, id string) {
	strokeWidth := ctx.Cfg("themeVariables", "strokeWidth")

	elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-requirement_arrowEnd").
		Attr("refX", 20).
		Attr("refY", 10).
		Attr("markerWidth", 20).
		Attr("markerHeight", 20).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Attr("stroke-width", js.ToString(strokeWidth)).
		Attr("viewBox", "0 0 25 20").
		Append("path").
		Attr(
			"d",
			"M0,0\n      L20,10\n      M20,10\n      L0,20",
		).
		Attr("stroke-linejoin", "miter")
}

func requirementContains(ctx *render.Context, elem *dom.Node, typ, id string) {
	containsNode := elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-requirement_containsStart").
		Attr("refX", 0).
		Attr("refY", 10).
		Attr("markerWidth", 20).
		Attr("markerHeight", 20).
		Attr("orient", "auto").
		Append("g")

	containsNode.Append("circle").Attr("cx", 10).Attr("cy", 10).Attr("r", 9).Attr("fill", "none")

	containsNode.Append("line").Attr("x1", 1).Attr("x2", 19).Attr("y1", 10).Attr("y2", 10)

	containsNode.Append("line").Attr("y1", 1).Attr("y2", 19).Attr("x1", 10).Attr("x2", 10)
}

func requirementContainsNeo(ctx *render.Context, elem *dom.Node, typ, id string) {
	strokeWidth := ctx.Cfg("themeVariables", "strokeWidth")
	containsNode := elem.
		Append("defs").
		Append("marker").
		Attr("id", id+"_"+typ+"-requirement_containsStart").
		Attr("refX", 0).
		Attr("refY", 10).
		Attr("markerWidth", 20).
		Attr("markerHeight", 20).
		Attr("orient", "auto").
		Attr("markerUnits", "userSpaceOnUse").
		Append("g")

	containsNode.Append("circle").Attr("cx", 10).Attr("cy", 10).Attr("r", 9).Attr("fill", "none")

	containsNode.Append("line").Attr("x1", 1).Attr("x2", 19).Attr("y1", 10).Attr("y2", 10)

	containsNode.Append("line").Attr("y1", 1).Attr("y2", 19).Attr("x1", 10).Attr("x2", 10)
	for _, n := range containsNode.SelectAll("*") {
		n.Attr("stroke-width", js.ToString(strokeWidth))
	}
}
func hierarchy(ctx *render.Context, elem *dom.Node, typ, id string) {
	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-hierarchyEnd").
		Attr("class", "marker hierarchy "+typ).
		Attr("viewBox", "0 0 12 10").
		Attr("refX", 10).
		Attr("refY", 5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 10).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 0 0 L 6 5 L 0 10 M 4 0 L 10 5 L 4 10").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 1).
		Style("stroke-dasharray", "1,0").
		Style("fill", "none")
	elem.
		Append("marker").
		Attr("id", id+"_"+typ+"-hierarchyStart").
		Attr("class", "marker hierarchy "+typ).
		Attr("viewBox", "0 0 12 10").
		Attr("refX", 2).
		Attr("refY", 5).
		Attr("markerUnits", "userSpaceOnUse").
		Attr("markerWidth", 12).
		Attr("markerHeight", 10).
		Attr("orient", "auto").
		Append("path").
		Attr("d", "M 12 0 L 6 5 L 12 10 M 8 0 L 2 5 L 8 10").
		Attr("class", "arrowMarkerPath").
		Style("stroke-width", 1).
		Style("stroke-dasharray", "1,0").
		Style("fill", "none")
}

// markers is the table of the marker functions, by the name that the
// diagrams list in data4Layout.markers.
var markers = map[string]func(ctx *render.Context, elem *dom.Node, typ, id string){
	"extension":                extension,
	"composition":              composition,
	"aggregation":              aggregation,
	"dependency":               dependency,
	"lollipop":                 lollipop,
	"point":                    point,
	"circle":                   circle,
	"cross":                    cross,
	"barb":                     barb,
	"barbNeo":                  barbNeo,
	"hierarchy":                hierarchy,
	"only_one":                 onlyOne,
	"zero_or_one":              zeroOrOne,
	"one_or_more":              oneOrMore,
	"zero_or_more":             zeroOrMore,
	"only_one_neo":             onlyOneNeo,
	"zero_or_one_neo":          zeroOrOneNeo,
	"one_or_more_neo":          oneOrMoreNeo,
	"zero_or_more_neo":         zeroOrMoreNeo,
	"requirement_arrow":        requirementArrow,
	"requirement_contains":     requirementContains,
	"requirement_arrow_neo":    requirementArrowNeo,
	"requirement_contains_neo": requirementContainsNeo,
}
