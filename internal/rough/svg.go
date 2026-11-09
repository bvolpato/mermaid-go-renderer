// Ported from roughjs 4.6.6, roughjs/bin/svg.js (MIT).

package rough

import "github.com/bvolpato/mermaid-go-renderer/internal/js"

// Draw is RoughSVG.draw. It returns the <path> elements of the <g> element
// that roughjs makes, in order. Each element has its attributes in the order
// that roughjs sets them. The caller makes the <g> element and the DOM nodes.
//
// A dash option (strokeLineDash, fillLineDash) is an array: []any or
// []float64. roughjs throws a TypeError for a truthy value of another type.
// Draw does not set the dash attribute in that case.
func (g *Generator) Draw(drawable *Drawable) []PathElement {
	sets := drawable.Sets
	o := drawable.Options
	if o == nil {
		o = g.defaultOptions
	}
	paths := []PathElement{}
	precision := o.fixedDecimalPlaceDigits
	for i := range sets {
		drawing := &sets[i]
		switch drawing.Type {
		case SetPath:
			path := PathElement{}
			path.setAttribute("d", g.OpsToPath(drawing, precision))
			path.setAttribute("stroke", toString(o.stroke))
			path.setAttribute("stroke-width", toString(o.strokeWidth))
			path.setAttribute("fill", "none")
			if truthy(o.strokeLineDash) {
				if s, ok := joinArray(o.strokeLineDash, " "); ok {
					path.setAttribute("stroke-dasharray", js.TrimSpace(s))
				}
			}
			if truthy(o.strokeLineDashOffset) {
				path.setAttribute("stroke-dashoffset", toString(o.strokeLineDashOffset))
			}
			paths = append(paths, path)
		case SetFillPath:
			path := PathElement{}
			path.setAttribute("d", g.OpsToPath(drawing, precision))
			path.setAttribute("stroke", "none")
			path.setAttribute("stroke-width", "0")
			path.setAttribute("fill", orEmpty(o.fill))
			if drawable.Shape == "curve" || drawable.Shape == "polygon" {
				path.setAttribute("fill-rule", "evenodd")
			}
			paths = append(paths, path)
		case SetFillSketch:
			paths = append(paths, g.fillSketchElement(drawing, o))
		}
	}
	return paths
}

// fillSketchElement is RoughSVG.fillSketch.
func (g *Generator) fillSketchElement(drawing *OpSet, o *ResolvedOptions) PathElement {
	fweight := o.fillWeight
	if o.fillWeightNum() < 0 {
		fweight = o.strokeWidthNum() / 2
	}
	path := PathElement{}
	path.setAttribute("d", g.OpsToPath(drawing, o.fixedDecimalPlaceDigits))
	path.setAttribute("stroke", orEmpty(o.fill))
	path.setAttribute("stroke-width", toString(fweight))
	path.setAttribute("fill", "none")
	if truthy(o.fillLineDash) {
		if s, ok := joinArray(o.fillLineDash, " "); ok {
			path.setAttribute("stroke-dasharray", js.TrimSpace(s))
		}
	}
	if truthy(o.fillLineDashOffset) {
		path.setAttribute("stroke-dashoffset", toString(o.fillLineDashOffset))
	}
	return path
}

func (p *PathElement) setAttribute(name, value string) {
	p.Attrs = append(p.Attrs, Attr{Name: name, Value: value})
}

// orEmpty is the value that setAttribute gets for the expression o.fill || "".
func orEmpty(v any) string {
	if truthy(v) {
		return toString(v)
	}
	return ""
}
