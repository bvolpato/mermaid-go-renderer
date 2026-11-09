// Ported from mermaid 12.2.0,
// rendering-util/rendering-elements/shapes/c4LabelHelper.ts (MIT).
//
// The label helper of the unified renderer (ru.LabelHelper) calls this
// function for a node that has a stereotype. The package sets the hook
// ru.C4LabelHelper for it.

package c4

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

const (
	sectionGap   = 3
	minWrapWidth = 32
)

// c4LabelHelper renders a stacked multi-section node label (name,
// stereotype, description lines) as pure SVG text, with one createText call
// for each section, so that each section can be styled through its own CSS
// class.
//
// It is a drop-in for labelHelper on nodes that carry a stereotype: the
// same element structure and return contract. The wrap width is a soft
// limit. An unbreakable word can exceed it, and the calling shape sizes
// itself from the returned bounds.
func c4LabelHelper(ctx *render.Context, parent *dom.Node, node *ru.Node, classes string) (*ru.LabelResult, error) {
	// classes ?? 'node default'. A shape always gives its classes.
	id := node.DomID
	if id == "" {
		id = node.ID
	}
	shapeSvg := parent.
		Insert("g").
		Attr("class", classes).
		Attr("id", id)

	labelEl := shapeSvg.
		Insert("g").
		Attr("class", "label").
		Attr("style", node.LabelStyle)

	// const name = typeof node.label === 'string' ? node.label : (node.label?.[0] ?? '');
	name := ""
	if node.HasLabel {
		name = node.Label
	}
	type section struct {
		text     string
		cssClass string
	}
	var sections []section
	add := func(text, cssClass string) {
		// .filter((section) => section.text)
		if text != "" {
			sections = append(sections, section{text, cssClass})
		}
	}
	add(name, "c4-name")
	add(node.Stereotype, "c4-type")
	for _, line := range node.Description {
		add(line, "c4-descr")
	}

	var wrapWidth float64
	if !ru.IsUndef(node.Width) && node.Width != 0 {
		wrapWidth = js.Max(node.Width-float64(2*ru.NN(node.Padding, 0)), minWrapWidth)
	} else {
		wrapWidth = 200
		if v := ctx.Cfg("flowchart", "wrappingWidth"); !js.IsNullish(v) {
			wrapWidth = js.ToNumber(v)
		}
	}
	// `c4.wrap` (schema default true) is this diagram's own auto-wrap flag, and is what
	// the legacy renderer gated on. The root-level `wrap` has no schema default, so
	// reading it here left every C4 label unwrapped.
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
	rendered := make([]renderedSection, len(sections))
	// Upstream starts all sections and then waits for them: each section is
	// appended and gets its text before the first one continues.
	for i, sec := range sections {
		// Appending before the first await pins the section order in the DOM.
		sectionEl := labelEl.Append("g").Attr("class", sec.cssClass)
		textEl, err := ru.CreateText(
			ctx,
			sectionEl,
			ctx.SanitizeText(render.DecodeEntities(sec.text)),
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
		// Center each wrapped line within the section; the outer tspan elements sit at x=0.
		textEl.SelectAll("tspan.text-outer-tspan").Attr("text-anchor", "middle")
		// Without markdown every word is "normal"; drop the presentation attributes so
		// font weight and style from CSS (section classes, per-element config) inherit.
		textEl.SelectAll("tspan.text-inner-tspan").
			Attr("font-weight", nil).
			Attr("font-style", nil)
		rendered[i].box = rendered[i].el.BBox()
	}

	// Math.max(...rendered.map(({ box }) => box.width), 0)
	totalWidth := math.Inf(-1)
	for _, r := range rendered {
		totalWidth = js.Max(totalWidth, r.box.Width)
	}
	totalWidth = js.Max(totalWidth, 0)
	y := 0.0
	for _, r := range rendered {
		r.el.Attr("transform", "translate("+js.Str(float64(totalWidth/2)-r.box.X-float64(r.box.Width/2))+", "+js.Str(y-r.box.Y)+")")
		y += r.box.Height + sectionGap
	}
	totalHeight := 0.0
	if len(rendered) > 0 {
		totalHeight = y - sectionGap
	}

	labelEl.Insert("rect", ":first-child")
	labelEl.Attr("transform", "translate("+js.Str(-totalWidth/2)+", "+js.Str(-totalHeight/2)+")")

	bbox := labelEl.BBox()
	halfPadding := float64(ru.NN(node.Padding, 0) / 2)
	// The box of getBBox is an SVGRect: it has no left and no top.
	return &ru.LabelResult{
		ShapeSvg:    shapeSvg,
		BBox:        ru.LabelBBox{X: bbox.X, Y: bbox.Y, Width: bbox.Width, Height: bbox.Height},
		HalfPadding: halfPadding,
		Label:       labelEl,
	}, nil
}
