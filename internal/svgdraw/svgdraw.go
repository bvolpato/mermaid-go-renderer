// Ported from mermaid 12.2.0, mermaid/src/diagrams/common/svgDrawCommon.ts and
// mermaid/src/diagrams/common/commonTypes.ts (MIT).

// Package svgdraw is the port of svgDrawCommon.ts: the functions that
// several diagram renderers use to draw rectangles, texts and images.
//
// A field of type any is a dynamic value that goes to selection.attr: nil
// is undefined, and d3 writes no attribute for it.
package svgdraw

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// RectData is the RectData interface.
type RectData struct {
	X float64
	Y float64
	// Fill is a string, or nil for undefined.
	Fill   any
	Width  float64
	Height float64
	// Stroke is a string, or nil for undefined.
	Stroke any
	// Class is the class attribute. The empty string writes no attribute.
	Class string
	Color string
	// Rx and Ry are written when they are not zero.
	Rx float64
	Ry float64
	// Attrs is Record<string, string | number>: more attributes, written in
	// the property order of the object.
	Attrs  *js.Obj[any]
	Anchor string
	// Name is the name attribute. The empty string writes no attribute.
	Name string
}

// Bound is the Bound interface.
type Bound struct {
	Startx float64
	Stopx  float64
	Starty float64
	Stopy  float64
	// Fill and Stroke are strings, or nil for undefined.
	Fill   any
	Stroke any
}

// TextData is the TextData interface.
type TextData struct {
	X float64
	Y float64
	// Anchor is the text-anchor style. The empty string sets no style.
	Anchor     string
	Text       string
	TextMargin float64
	// Class replaces the class "legend" when it is not empty.
	Class string
}

// TextObject is the TextObject interface.
type TextObject struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
	// Fill, Anchor and Valign are strings, or nil for undefined.
	Fill   any
	Anchor any
	// TextAnchor is the property 'text-anchor'.
	TextAnchor string
	Style      string
	TextMargin float64
	Rx         float64
	Ry         float64
	Tspan      bool
	Valign     any

	// The fields below are not in the TextObject interface. The renderers
	// of upstream (sequence, journey, timeline) set them on the object that
	// getTextObj returns before they draw it. A dynamic field is nil until
	// a renderer sets it, as the property is undefined upstream.
	Text              string
	Class             any
	Dy                any
	FontFamily        any
	FontSize          any
	FontWeight        any
	Wrap              bool
	LabelMargin       float64
	DominantBaseline  any
	AlignmentBaseline any
}

// truthy is the JavaScript conversion of a number to a boolean.
func truthy(x float64) bool {
	return x != 0 && !math.IsNaN(x)
}

// nullish maps undefined and null to nil, which removes an attribute, as
// d3 does for a value == null.
func nullish(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// DrawRect is drawRect(element, rectData): it appends a rect element.
func DrawRect(element *dom.Node, rectData RectData) *dom.Node {
	rectElement := element.Append("rect")
	rectElement.Attr("x", rectData.X)
	rectElement.Attr("y", rectData.Y)
	rectElement.Attr("fill", nullish(rectData.Fill))
	rectElement.Attr("stroke", nullish(rectData.Stroke))
	rectElement.Attr("width", rectData.Width)
	rectElement.Attr("height", rectData.Height)
	if rectData.Name != "" {
		rectElement.Attr("name", rectData.Name)
	}
	if truthy(rectData.Rx) {
		rectElement.Attr("rx", rectData.Rx)
	}
	if truthy(rectData.Ry) {
		rectElement.Attr("ry", rectData.Ry)
	}

	if rectData.Attrs != nil {
		rectData.Attrs.Each(func(attrKey string, value any) {
			rectElement.Attr(attrKey, nullish(value))
		})
	}

	if rectData.Class != "" {
		rectElement.Attr("class", rectData.Class)
	}

	return rectElement
}

// DrawBackgroundRect draws a background rectangle. element is the diagram
// (the reference for the bounds), and bounds is the shape of the rectangle.
func DrawBackgroundRect(element *dom.Node, bounds Bound) {
	rectData := RectData{
		X:      bounds.Startx,
		Y:      bounds.Starty,
		Width:  bounds.Stopx - bounds.Startx,
		Height: bounds.Stopy - bounds.Starty,
		Fill:   bounds.Fill,
		Stroke: bounds.Stroke,
		Class:  "rect",
	}
	rectElement := DrawRect(element, rectData)
	rectElement.Lower()
}

// DrawText is drawText(element, textData): it appends a text element with
// one tspan.
func DrawText(element *dom.Node, textData TextData) *dom.Node {
	nText := sanitize.LineBreakRegex.ReplaceAllLiteralString(textData.Text, " ")

	textElem := element.Append("text")
	textElem.Attr("x", textData.X)
	textElem.Attr("y", textData.Y)
	textElem.Attr("class", "legend")

	textElem.Style("text-anchor", textData.Anchor)
	if textData.Class != "" {
		textElem.Attr("class", textData.Class)
	}

	tspan := textElem.Append("tspan")
	tspan.Attr("x", textData.X+float64(textData.TextMargin*2))
	tspan.Text(nText)

	return textElem
}

// DrawImage is drawImage(elem, x, y, link): it appends an image element.
func DrawImage(elem *dom.Node, x, y float64, link string) {
	imageElement := elem.Append("image")
	imageElement.Attr("x", x)
	imageElement.Attr("y", y)
	sanitizedLink := sanitize.SanitizeUrl(link)
	imageElement.Attr("xlink:href", sanitizedLink)
}

// DrawEmbeddedImage is drawEmbeddedImage(element, x, y, link): it appends a
// use element that refers to the id in link.
func DrawEmbeddedImage(element *dom.Node, x, y float64, link string) {
	imageElement := element.Append("use")
	imageElement.Attr("x", x)
	imageElement.Attr("y", y)
	sanitizedLink := sanitize.SanitizeUrl(link)
	imageElement.Attr("xlink:href", "#"+sanitizedLink)
}

// GetNoteRect is getNoteRect().
func GetNoteRect() RectData {
	noteRectData := RectData{
		X:      0,
		Y:      0,
		Width:  100,
		Height: 100,
		Fill:   "#EDF2AE",
		Stroke: "#666",
		Anchor: "start",
		Rx:     0,
		Ry:     0,
	}
	return noteRectData
}

// GetTextObj is getTextObj() of svgDrawCommon.ts. The function with the
// same name in utils.ts is utils.GetTextObj.
func GetTextObj() TextObject {
	testObject := TextObject{
		X:          0,
		Y:          0,
		Width:      100,
		Height:     100,
		TextAnchor: "start",
		Style:      "#666",
		TextMargin: 0,
		Rx:         0,
		Ry:         0,
		Tspan:      true,
	}
	return testObject
}

// CreateTooltip is createTooltip(): it returns the tooltip element of the
// document, and appends it to the body when there is none.
func CreateTooltip(ctx *render.Context) *dom.Node {
	tooltipElem := ctx.Body.Select(".mermaidTooltip")
	if tooltipElem == nil {
		tooltipElem = ctx.Body.
			Append("div").
			Attr("class", "mermaidTooltip").
			Style("opacity", 0).
			Style("position", "absolute").
			Style("text-align", "center").
			Style("max-width", "200px").
			Style("padding", "2px").
			Style("font-size", "12px").
			Style("background", "#ffffde").
			Style("border", "1px solid #333").
			Style("border-radius", "2px").
			Style("pointer-events", "none").
			Style("z-index", "100")
	}
	return tooltipElem
}
