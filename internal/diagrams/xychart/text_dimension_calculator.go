// Ported from mermaid 12.2.0,
// diagrams/xychart/chartBuilder/textDimensionCalculator.ts (MIT).

package xychart

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// textDimensionCalculator is TextDimensionCalculator.
type textDimensionCalculator interface {
	getMaxDimension(texts []string, fontSize any) dimension
}

// textDimensionCalculatorWithFont is TextDimensionCalculatorWithFont.
type textDimensionCalculatorWithFont struct {
	parentGroup *dom.Node
}

// screenCTM is the part of the measurement engine that getScreenCTM needs.
// dom.Engine does not have it.
type screenCTM interface {
	// ScreenCTM is SVGGraphicsElement.getScreenCTM(): the matrix a, b, c, d,
	// e, f. ok is false where the browser returns null.
	ScreenCTM(n *dom.Node) (m [6]float64, ok bool)
}

func (c *textDimensionCalculatorWithFont) getMaxDimension(texts []string, fontSize any) dimension {
	if c.parentGroup == nil {
		acc := 0.0
		for _, cur := range texts {
			acc = js.Max(float64(js.UTF16Len(cur)), acc)
		}
		return dimension{
			width:  acc * num(fontSize),
			height: num(fontSize),
		}
	}

	dim := dimension{
		width:  0,
		height: 0,
	}

	elem := c.parentGroup.
		Append("g").
		Attr("visibility", "hidden").
		Attr("font-size", attrValue(fontSize))

	// computeDimensionOfText measures in screen pixels. When the SVG is scaled
	// to fit its container (useMaxWidth), convert back to SVG user units so
	// layout matches drawing.
	toScale := func(s float64, ok bool) float64 {
		if ok && s > 0 {
			return s
		}
		return 1
	}
	var ctm [6]float64
	hasCTM := false
	if engine, ok := elem.Document().Engine.(screenCTM); ok {
		ctm, hasCTM = engine.ScreenCTM(elem)
	}
	scaleX := toScale(ctm[0], hasCTM)
	scaleY := toScale(ctm[3], hasCTM)

	for _, t := range texts {
		bbox := ru.ComputeDimensionOfText(elem, 1, t)
		width := bbox.Width / scaleX
		height := bbox.Height / scaleY
		dim.width = js.Max(dim.width, width)
		dim.height = js.Max(dim.height, height)
	}
	elem.Remove()
	return dim
}
