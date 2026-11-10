// Ported from mermaid 12.2.0, rendering-util/createText.ts and
// rendering-elements/createLabel.js (MIT).

package ru

import (
	"math"
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/markdown"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// maxSafeSizeForWidth: nobody creates labels wider than 16384 px.
const maxSafeSizeForWidth = 16384

func applyStyle(node *dom.Node, style string) {
	if style != "" {
		node.Attr("style", style)
	}
}

type htmlSpanNode struct {
	label      string
	labelStyle string
	isNode     bool
}

func addHtmlSpan(ctx *render.Context, element *dom.Node, node htmlSpanNode, width float64, classes string, addBackground bool, minWidth float64) (*dom.Node, error) {
	fo := element.Append("foreignObject")
	// This is not the final width. The final width comes from the div.
	fo.Attr("width", js.Str(js.Min(float64(10*width), maxSafeSizeForWidth))+"px")
	fo.Attr("height", js.Str(js.Min(float64(10*width), maxSafeSizeForWidth))+"px")

	div := fo.Append("xhtml:div")
	var sanitizedLabel string
	if sanitize.HasKatex(node.label) {
		var err error
		sanitizedLabel, err = RenderKatexSanitized(ctx, sanitize.LineBreakRegex.ReplaceAllString(node.label, "\n"))
		if err != nil {
			return nil, err
		}
	} else {
		sanitizedLabel = ctx.SanitizeText(node.label)
	}
	labelClass := "edgeLabel"
	if node.isNode {
		labelClass = "nodeLabel"
	}
	span := div.Append("span")
	span.HTML(sanitizedLabel)
	applyStyle(span, node.labelStyle)
	span.Attr("class", labelClass+" "+classes)

	applyStyle(div, node.labelStyle)
	div.Style("display", "table-cell")
	div.Style("white-space", "nowrap")
	div.Style("line-height", "1.5")
	if !math.IsInf(width, 1) {
		div.Style("max-width", js.Str(width)+"px")
		div.Style("text-align", "center")
	}
	div.Attr("xmlns", "http://www.w3.org/1999/xhtml")
	if addBackground {
		div.Attr("class", "labelBkg")
	}

	ctx.AwaitMeasure()
	bbox := div.BoundingClientRect()
	if bbox.Width == width {
		div.Style("display", "table")
		div.Style("white-space", "break-spaces")
		div.Style("width", js.Str(width)+"px")
	} else if bbox.Width < minWidth {
		div.Style("display", "table")
		div.Style("width", js.Str(minWidth)+"px")
		div.Style("text-align", "center")
	}
	// The await of the caller.
	ctx.Await()
	return fo, nil
}

// createTspan creates a tspan with the attributes for text positioning.
func createTspan(textElement *dom.Node, lineIndex int, lineHeight float64, centerText bool) *dom.Node {
	tspan := textElement.Append("tspan").
		Attr("class", "text-outer-tspan").
		Attr("x", 0).
		Attr("y", js.Str(float64(float64(lineIndex)*lineHeight)-0.1)+"em").
		Attr("dy", js.Str(lineHeight)+"em")
	if centerText {
		tspan.Attr("text-anchor", "middle")
	}
	return tspan
}

func computeWidthOfText(parentNode *dom.Node, lineHeight float64, line []markdown.Word) float64 {
	testElement := parentNode.Append("text")
	testSpan := createTspan(testElement, 1, lineHeight, false)
	updateTextContentAndStyles(testSpan, line)
	textLength := testSpan.ComputedTextLength()
	testElement.Remove()
	return textLength
}

// ComputeDimensionOfText is computeDimensionOfText(parentNode, lineHeight, text).
func ComputeDimensionOfText(parentNode *dom.Node, lineHeight float64, text string) dom.Rect {
	testElement := parentNode.Append("text")
	testSpan := createTspan(testElement, 1, lineHeight, false)
	updateTextContentAndStyles(testSpan, []markdown.Word{{Content: text, Type: "normal"}})
	textDimension := testSpan.BoundingClientRect()
	testElement.Remove()
	return textDimension
}

// createFormattedText creates an SVG text element: it breaks lines and
// applies the styles of the structured text.
func createFormattedText(width float64, g *dom.Node, structuredText [][]markdown.Word, addBackground, centerText bool) (*dom.Node, error) {
	const lineHeight = 1.1
	labelGroup := g.Append("g")
	bkg := labelGroup.Insert("rect").Attr("class", "background").Attr("style", "stroke: none")
	textElement := labelGroup.Append("text").Attr("y", "-10.1")
	if centerText {
		textElement.Attr("text-anchor", "middle")
	}
	lineIndex := 0
	for _, line := range structuredText {
		checkWidth := func(line []markdown.Word) bool {
			return computeWidthOfText(labelGroup, lineHeight, line) <= width
		}
		linesUnderWidth := [][]markdown.Word{line}
		if !checkWidth(line) {
			var err error
			if linesUnderWidth, err = markdown.SplitLineToFitWidth(line, checkWidth); err != nil {
				return nil, err
			}
		}
		for _, preparedLine := range linesUnderWidth {
			tspan := createTspan(textElement, lineIndex, lineHeight, centerText)
			updateTextContentAndStyles(tspan, preparedLine)
			lineIndex++
		}
	}
	if addBackground {
		bbox := textElement.BBox()
		const padding = 2
		bkg.
			Attr("x", bbox.X-padding).
			Attr("y", bbox.Y-padding).
			Attr("width", bbox.Width+2*padding).
			Attr("height", bbox.Height+2*padding)
		return labelGroup, nil
	}
	return textElement, nil
}

var reDecodeHTMLEntities = regexp.MustCompile(`&(amp|lt|gt);`)

// decodeHTMLEntities decodes the entities that sanitizeText encodes. The
// SVG code sets text with textContent, which does not decode entities.
func decodeHTMLEntities(text string) string {
	return reDecodeHTMLEntities.ReplaceAllStringFunc(text, func(match string) string {
		switch match {
		case "&amp;":
			return "&"
		case "&lt;":
			return "<"
		default:
			return ">"
		}
	})
}

// updateTextContentAndStyles fills a tspan with one inner tspan per word.
func updateTextContentAndStyles(tspan *dom.Node, wrappedLine []markdown.Word) {
	tspan.Text("")
	for index, word := range wrappedLine {
		fontStyle := "normal"
		if word.Type == "em" {
			fontStyle = "italic"
		}
		fontWeight := "normal"
		if word.Type == "strong" {
			fontWeight = "bold"
		}
		innerTspan := tspan.Append("tspan").
			Attr("font-style", fontStyle).
			Attr("class", "text-inner-tspan").
			Attr("font-weight", fontWeight)
		if index == 0 {
			innerTspan.Text(decodeHTMLEntities(word.Content))
		} else {
			innerTspan.Text(" " + decodeHTMLEntities(word.Content))
		}
	}
}

var reIconSubstring = regexp.MustCompile(`(fa[bklrs]?):fa-([\w-]+)`)

// ReplaceIconSubstring is replaceIconSubstring(text, config): it converts
// fontawesome labels to icons, or to <i> tags when the icon is not
// registered.
func ReplaceIconSubstring(ctx *render.Context, text string) string {
	// Upstream starts one async replacement per icon and awaits them with
	// Promise.all. The number of turns of the microtask queue depends on the
	// slowest one: an icon that is registered takes four turns
	// (isIconAvailable, its await, getIconSVG, its await), an icon that is
	// not registered takes two. Promise.all and its await add two. With no
	// icon, await Promise.all([]) is one turn.
	turns := 1
	out := reIconSubstring.ReplaceAllStringFunc(text, func(fullMatch string) string {
		m := reIconSubstring.FindStringSubmatch(fullMatch)
		registeredIconName := m[1] + ":" + m[2]
		if IsIconAvailable(ctx, registeredIconName) {
			turns = 6
			svg, _ := GetIconSVG(ctx, registeredIconName, nil, IconAttrs("class", "label-icon"))
			return svg
		}
		if turns < 4 {
			turns = 4
		}
		return "<i class='" + strings.Replace(ctx.SanitizeText(fullMatch), ":", " ", 1) + "'></i>"
	})
	// One more turn for the await of the caller.
	ctx.AwaitN(turns + 1)
	return out
}

// TextOptions are the options of createText. A zero value is not the
// default: start from DefaultTextOptions.
type TextOptions struct {
	Style         string
	IsTitle       bool
	Classes       string
	UseHtmlLabels bool
	Markdown      bool
	IsNode        bool
	// Width is the width to wrap the text within. Use +Inf for no wrapping.
	Width float64
	// MinWidth widens a narrower label. 0 disables it. Only for HTML labels.
	MinWidth         float64
	AddSvgBackground bool
}

// DefaultTextOptions returns the defaults of the options of createText.
func DefaultTextOptions() TextOptions {
	return TextOptions{UseHtmlLabels: true, Markdown: true, IsNode: true, Width: 200}
}

var (
	reBR              = regexp.MustCompile(`<br\s*/?>`)
	reStrokeColon     = regexp.MustCompile(`stroke:`)
	reStrokeDecl      = regexp.MustCompile(`stroke:[^;]+;?`)
	reStrokeWidth     = regexp.MustCompile(`stroke-width:[^;]+;?`)
	reFillDecl        = regexp.MustCompile(`fill:[^;]+;?`)
	reDoubleBackslash = regexp.MustCompile(`\\\\`)
)

// CreateText is createText(el, text, options, config): it creates a text
// element in the SVG group. The result is a foreignObject for HTML labels,
// else a text element or a group with a background.
func CreateText(ctx *render.Context, el *dom.Node, text string, o TextOptions) (*dom.Node, error) {
	// A width that is undefined takes the default of the destructuring.
	if IsUndef(o.Width) {
		o.Width = 200
	}
	if IsUndef(o.MinWidth) {
		o.MinWidth = 0
	}
	if o.UseHtmlLabels {
		var htmlText string
		if o.Markdown {
			htmlText = markdown.MarkdownToHTML(text, ctx.Config)
		} else {
			htmlText = markdown.NonMarkdownToHTML(text)
		}
		decodedReplacedText := ReplaceIconSubstring(ctx, render.DecodeEntities(htmlText))

		node := htmlSpanNode{
			isNode:     o.IsNode,
			label:      decodedReplacedText,
			labelStyle: strings.Replace(o.Style, "fill:", "color:", 1),
		}
		if sanitize.HasKatex(text) {
			// For KaTeX the text can hold escaped characters.
			node.label = reDoubleBackslash.ReplaceAllString(text, `\`)
		}
		vertexNode, err := addHtmlSpan(ctx, el, node, o.Width, o.Classes, o.AddSvgBackground, o.MinWidth)
		// The await of the caller.
		ctx.Await()
		return vertexNode, err
	}
	// The user can write br tags with spaces: make them <br/>.
	sanitizeBR := render.DecodeEntities(reBR.ReplaceAllString(text, "<br/>"))
	var structuredText [][]markdown.Word
	if o.Markdown {
		structuredText = markdown.MarkdownToLines(strings.Replace(sanitizeBR, "<br>", "<br/>", 1), ctx.Config)
	} else {
		structuredText = markdown.NonMarkdownToLines(sanitizeBR)
	}
	svgLabel, err := createFormattedText(o.Width, el, structuredText, text != "" && o.AddSvgBackground, !o.IsNode)
	if err != nil {
		return nil, err
	}
	style := o.Style
	if o.IsNode {
		if reStrokeColon.MatchString(style) {
			style = strings.Replace(style, "stroke:", "lineColor:", 1)
		}
		nodeLabelTextStyle := reStrokeDecl.ReplaceAllString(style, "")
		nodeLabelTextStyle = reStrokeWidth.ReplaceAllString(nodeLabelTextStyle, "")
		nodeLabelTextStyle = reFillDecl.ReplaceAllString(nodeLabelTextStyle, "")
		nodeLabelTextStyle = strings.ReplaceAll(nodeLabelTextStyle, "color:", "fill:")
		svgLabel.Attr("style", nodeLabelTextStyle)
	} else {
		// On an edge, stroke and stroke-width are for the path: remove
		// them. background becomes the fill of the label rect.
		edgeLabelRectStyle := reStrokeDecl.ReplaceAllString(style, "")
		edgeLabelRectStyle = reStrokeWidth.ReplaceAllString(edgeLabelRectStyle, "")
		edgeLabelRectStyle = reFillDecl.ReplaceAllString(edgeLabelRectStyle, "")
		edgeLabelRectStyle = strings.ReplaceAll(edgeLabelRectStyle, "background:", "fill:")
		svgLabel.Select("rect").Attr("style", strings.ReplaceAll(edgeLabelRectStyle, "background:", "fill:"))

		edgeLabelTextStyle := reStrokeDecl.ReplaceAllString(style, "")
		edgeLabelTextStyle = reStrokeWidth.ReplaceAllString(edgeLabelTextStyle, "")
		edgeLabelTextStyle = reFillDecl.ReplaceAllString(edgeLabelTextStyle, "")
		edgeLabelTextStyle = strings.ReplaceAll(edgeLabelTextStyle, "color:", "fill:")
		svgLabel.Select("text").Attr("style", edgeLabelTextStyle)
	}
	if o.IsTitle {
		svgLabel.SelectAll("tspan.text-outer-tspan").Classed("title-row", true)
	} else {
		svgLabel.SelectAll("tspan.text-outer-tspan").Classed("row", true)
	}
	// The await of the caller.
	ctx.Await()
	return svgLabel, nil
}

// CreateLabel is createLabel(element, vertexText, style, isTitle, isNode)
// of createLabel.js.
func CreateLabel(ctx *render.Context, element *dom.Node, vertexText string, style string, isTitle, isNode bool) (*dom.Node, error) {
	label, err := CreateText(ctx, element, vertexText, TextOptions{
		Style:         style,
		IsTitle:       isTitle,
		UseHtmlLabels: sanitize.GetEffectiveHtmlLabels(ctx.Config),
		Markdown:      false,
		IsNode:        isNode,
		Width:         math.Inf(1),
	})
	// The await of the caller.
	ctx.Await()
	return label, err
}
