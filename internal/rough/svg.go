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
// DrawEach streams each generated <path> element's attributes to fn without
// allocating intermediate PathElement or Attr slices.
func (g *Generator) DrawEach(drawable *Drawable, fn func(attrs []Attr)) {
	sets := drawable.Sets
	o := drawable.Options
	if o == nil {
		o = g.defaultOptions
	}
	precision := o.fixedDecimalPlaceDigits
	var buf [8]Attr
	for i := range sets {
		drawing := &sets[i]
		attrs := buf[:0]
		switch drawing.Type {
		case SetPath:
			attrs = append(attrs,
				Attr{Name: "d", Value: g.OpsToPath(drawing, precision)},
				Attr{Name: "stroke", Value: toString(o.stroke)},
				Attr{Name: "stroke-width", Value: toString(o.strokeWidth)},
				Attr{Name: "fill", Value: "none"},
			)
			if truthy(o.strokeLineDash) {
				if s, ok := joinArray(o.strokeLineDash, " "); ok {
					attrs = append(attrs, Attr{Name: "stroke-dasharray", Value: js.TrimSpace(s)})
				}
			}
			if truthy(o.strokeLineDashOffset) {
				attrs = append(attrs, Attr{Name: "stroke-dashoffset", Value: toString(o.strokeLineDashOffset)})
			}
			fn(attrs)
		case SetFillPath:
			attrs = append(attrs,
				Attr{Name: "d", Value: g.OpsToPath(drawing, precision)},
				Attr{Name: "stroke", Value: "none"},
				Attr{Name: "stroke-width", Value: "0"},
				Attr{Name: "fill", Value: orEmpty(o.fill)},
			)
			if drawable.Shape == "curve" || drawable.Shape == "polygon" {
				attrs = append(attrs, Attr{Name: "fill-rule", Value: "evenodd"})
			}
			fn(attrs)
		case SetFillSketch:
			fweight := o.fillWeight
			if o.fillWeightNum() < 0 {
				fweight = o.strokeWidthNum() / 2
			}
			attrs = append(attrs,
				Attr{Name: "d", Value: g.OpsToPath(drawing, precision)},
				Attr{Name: "stroke", Value: orEmpty(o.fill)},
				Attr{Name: "stroke-width", Value: toString(fweight)},
				Attr{Name: "fill", Value: "none"},
			)
			if truthy(o.fillLineDash) {
				if s, ok := joinArray(o.fillLineDash, " "); ok {
					attrs = append(attrs, Attr{Name: "stroke-dasharray", Value: js.TrimSpace(s)})
				}
			}
			if truthy(o.fillLineDashOffset) {
				attrs = append(attrs, Attr{Name: "stroke-dashoffset", Value: toString(o.fillLineDashOffset)})
			}
			fn(attrs)
		}
	}
}

func (g *Generator) Draw(drawable *Drawable) []PathElement {
	paths := make([]PathElement, 0, len(drawable.Sets))
	g.DrawEach(drawable, func(attrs []Attr) {
		cp := make([]Attr, len(attrs))
		copy(cp, attrs)
		paths = append(paths, PathElement{Attrs: cp})
	})
	return paths
}

// orEmpty is the value that setAttribute gets for the expression o.fill || "".
func orEmpty(v any) string {
	if truthy(v) {
		return toString(v)
	}
	return ""
}
