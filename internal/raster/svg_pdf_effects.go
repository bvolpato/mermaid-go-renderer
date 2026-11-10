// This file paints original flat PDF shadows from the filter operations
// resolved by svg_filter.go. It does not port another upstream file.
package raster

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/pdf"
)

// paintVectorFiltered paints offset source silhouettes before the source.
// Blur is intentionally omitted. Unrecognized filter primitives pass through.
func (p *painter) paintVectorFiltered(n *dom.Node, st *svgStyle, ctm Affine, ops []filterOp, paint func(*svgStyle, Affine)) {
	var shadows []filterOp
	for _, op := range ops {
		switch op.kind {
		case filterDropShadow:
			shadows = append(shadows, op)
		case filterReference:
			for prim := op.node.FirstChild; prim != nil; prim = prim.Next {
				if prim.Type != dom.ElementNode {
					continue
				}
				if !strings.EqualFold(prim.Tag, "feDropShadow") {
					// Other filter graphs need image-filter evaluation.
					paint(st, ctm)
					return
				}
				s := filterOp{kind: filterDropShadow, dx: attrNumber(prim, "dx", 2), dy: attrNumber(prim, "dy", 2), color: css.Color{A: 1}}
				if v, ok := p.eng.RasterDeclared(prim, "flood-color"); ok {
					if c, ok := parseColor(v); ok {
						s.color = c
					}
				}
				if v, ok := p.eng.RasterDeclared(prim, "flood-opacity"); ok {
					if a, ok := parseOpacity(v); ok {
						s.color.A *= float64(a)
					}
				}
				if strings.TrimSpace(op.node.AttrOr("primitiveUnits", "userSpaceOnUse")) == "objectBoundingBox" {
					if _, _, w, h, ok := p.eng.RasterBBox(n); ok {
						s.dx, s.dy = float32(s.dx*w), float32(s.dy*h)
					}
				}
				shadows = append(shadows, s)
			}
		}
	}
	for _, s := range shadows {
		if !(s.color.A > 0) {
			continue
		}
		c := pdf.Color{R: s.color.R, G: s.color.G, B: s.color.B, A: 255}
		p.vector.doc.BeginGroup(s.color.A, false)
		p.vector.shadow = &c
		paint(st, ctm.Translate(float64(s.dx), float64(s.dy)))
		p.vector.shadow = nil
		p.vector.doc.EndGroup()
	}
	paint(st, ctm)
}
