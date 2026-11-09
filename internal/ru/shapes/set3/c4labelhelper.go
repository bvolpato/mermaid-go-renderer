// Ported from mermaid 12.2.0, rendering-elements/shapes/c4LabelHelper.ts
// (MIT).

package set3

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	// labelHelper calls c4LabelHelper for a node that has a stereotype.
	ru.C4LabelHelper = c4LabelHelper
}

const (
	c4SectionGap   = 3.0
	c4MinWrapWidth = 32.0
)

// c4LabelHelper renders the label of a node that has several sections (the
// name, the stereotype and the description lines) as SVG text. Each section
// is one createText call, so that it can be styled through its own CSS
// class.
//
// It stands in for labelHelper on nodes that have a stereotype: it makes
// the same element structure and returns the same values. The wrap width is
// a soft limit. A word that cannot break can be wider, and the calling
// shape takes its size from the returned box.
func c4LabelHelper(ctx *render.Context, parent *dom.Node, node *ru.Node, classes string) (*ru.LabelResult, error) {
	// classes ?? 'node default'. An empty text stands for undefined: every
	// shape gives its classes.
	if classes == "" {
		classes = "node default"
	}
	shapeSvg := parent.
		Insert("g").
		Attr("class", classes).
		Attr("id", nodeID(node))

	labelEl := shapeSvg.
		Insert("g").
		Attr("class", "label").
		Attr("style", node.LabelStyle)

	name := node.Label
	type section struct {
		text     string
		cssClass string
	}
	all := []section{
		{text: name, cssClass: "c4-name"},
		{text: node.Stereotype, cssClass: "c4-type"},
	}
	for _, line := range node.Description {
		all = append(all, section{text: line, cssClass: "c4-descr"})
	}
	// .filter((section) => section.text)
	var sections []section
	for _, s := range all {
		if s.text != "" {
			sections = append(sections, s)
		}
	}

	var wrapWidth float64
	if js.Truthy(node.Width) {
		wrapWidth = js.Max(node.Width-float64(2*ru.NN(node.Padding, 0)), c4MinWrapWidth)
	} else {
		wrapWidth = 200
		if v := ctx.Cfg("flowchart", "wrappingWidth"); !js.IsNullish(v) {
			wrapWidth = js.ToNumber(v)
		}
	}
	// c4.wrap is the flag of the C4 diagram for automatic wrapping. Its
	// default in the schema is true.
	var shouldWrap any = true
	if v := ctx.Cfg("c4", "wrap"); !js.IsNullish(v) {
		shouldWrap = v
	}
	width := math.Inf(1)
	if js.Truthy(shouldWrap) {
		width = wrapWidth
	}

	type renderedSection struct {
		el     *dom.Node
		textEl *dom.Node
		box    dom.Rect
	}
	// Upstream starts the sections one after the other and then waits for
	// all of them: every section is in the DOM with its text before the
	// first section goes on.
	rendered := make([]renderedSection, len(sections))
	for i, s := range sections {
		// The section is appended before the first await, which fixes the
		// order of the sections in the DOM.
		sectionEl := labelEl.Append("g").Attr("class", s.cssClass)
		textEl, err := ru.CreateText(
			ctx,
			sectionEl,
			ctx.SanitizeText(render.DecodeEntities(s.text)),
			ru.TextOptions{
				UseHtmlLabels: false,
				Markdown:      false,
				IsNode:        true,
				Width:         width,
				Style:         node.LabelStyle,
			},
		)
		if err != nil {
			return nil, err
		}
		rendered[i] = renderedSection{el: sectionEl, textEl: textEl}
	}
	for i := range rendered {
		textEl := rendered[i].textEl
		// Center each wrapped line in the section. The outer tspan
		// elements are at x=0.
		textEl.SelectAll("tspan.text-outer-tspan").Attr("text-anchor", "middle")
		// Without markdown every word is "normal". Remove the presentation
		// attributes, so that the font weight and the font style of the
		// CSS apply.
		textEl.
			SelectAll("tspan.text-inner-tspan").
			Attr("font-weight", nil).
			Attr("font-style", nil)
		rendered[i].box = rendered[i].el.BBox()
	}

	// Math.max(...rendered.map(({ box }) => box.width), 0)
	widths := make([]float64, 0, len(rendered)+1)
	for _, r := range rendered {
		widths = append(widths, r.box.Width)
	}
	totalWidth := js.Max(append(widths, 0)...)
	y := 0.0
	for _, r := range rendered {
		r.el.Attr("transform", "translate("+js.Str(float64(totalWidth/2)-r.box.X-float64(r.box.Width/2))+", "+js.Str(y-r.box.Y)+")")
		y += r.box.Height + c4SectionGap
	}
	totalHeight := 0.0
	if len(rendered) > 0 {
		totalHeight = y - c4SectionGap
	}

	labelEl.Insert("rect", ":first-child")
	labelEl.Attr("transform", "translate("+js.Str(-totalWidth/2)+", "+js.Str(-totalHeight/2)+")")

	bbox := labelEl.BBox()
	halfPadding := ru.NN(node.Padding, 0) / 2
	// The box of getBBox is an SVGRect: it has no left and no top.
	return &ru.LabelResult{
		ShapeSvg:    shapeSvg,
		BBox:        ru.LabelBBox{X: bbox.X, Y: bbox.Y, Width: bbox.Width, Height: bbox.Height},
		HalfPadding: halfPadding,
		Label:       labelEl,
	}, nil
}
