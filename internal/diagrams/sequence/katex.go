// Ported from mermaid 12.2.0, diagrams/sequence/svgDraw.js and
// diagrams/common/common.ts (MIT).

package sequence

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

func (r *renderer) calculateLabelDimensions(text string, font utils.TextDimensionConfig) (utils.TextDimensions, error) {
	if !sanitize.HasKatex(text) {
		return r.calculateTextDimensions(text, font)
	}
	dim, err := ru.CalculateMathMLDimensions(r.ctx, text)
	return utils.TextDimensions{Width: dim.Width, Height: dim.Height}, err
}

type mathPlacement struct {
	startx, stopx, starty float64
}

func (r *renderer) drawTextOrKatex(elem *dom.Node, textData *textObj, placement *mathPlacement) ([]*dom.Node, error) {
	if !sanitize.HasKatex(textData.text) {
		return drawText(elem, textData), nil
	}
	return r.drawKatex(elem, textData, placement)
}

// drawKatex keeps the foreignObject placement of Mermaid's sequence labels.
func (r *renderer) drawKatex(elem *dom.Node, textData *textObj, placement *mathPlacement) ([]*dom.Node, error) {
	textElem := elem.Append("foreignObject")
	markup, err := ru.RenderKatexSanitized(r.ctx, textData.text)
	if err != nil {
		return nil, err
	}
	div := textElem.Append("xhtml:div").
		Attr("style", "width: fit-content;").
		Attr("xmlns", "http://www.w3.org/1999/xhtml").HTML(markup)
	dim := div.BoundingClientRect()
	textElem.Attr("height", js.Round(dim.Height)).Attr("width", js.Round(dim.Width))
	if textData.class == "noteText" {
		rect := elem.FirstElementChild()
		rect.Attr("height", dim.Height+float64(2*textData.textMargin))
		box := rect.BBox()
		textElem.Attr("x", js.Round(box.X+float64(box.Width/2)-float64(dim.Width/2))).
			Attr("y", js.Round(box.Y+float64(box.Height/2)-float64(dim.Height/2)))
	} else if placement != nil {
		startx, stopx := placement.startx, placement.stopx
		if startx > stopx {
			startx, stopx = stopx, startx
		}
		textElem.Attr("x", js.Round(startx+float64(math.Abs(startx-stopx)/2)-float64(dim.Width/2)))
		y := placement.starty - dim.Height
		if textData.class == "loopText" {
			y = placement.starty
		}
		textElem.Attr("y", js.Round(y))
	}
	r.ctx.Await()
	return []*dom.Node{textElem}, nil
}

// byKatex includes the SVG text fallback that Mermaid puts after the math.
func (r *renderer) byKatex(content string, g *dom.Node, x, y, width, height float64, textAttrs []textAttr) error {
	dim, err := ru.CalculateMathMLDimensions(r.ctx, content)
	if err != nil {
		return err
	}
	s := g.Append("switch")
	f := s.Append("foreignObject").
		Attr("x", x+float64(width/2)-float64(dim.Width/2)).
		Attr("y", y+float64(height/2)-float64(dim.Height/2)).
		Attr("width", dim.Width).Attr("height", dim.Height)
	text := f.Append("xhtml:div").Style("height", "100%").Style("width", "100%")
	markup, err := ru.RenderKatexSanitized(r.ctx, content)
	if err != nil {
		return err
	}
	text.Append("div").Style("text-align", "center").Style("vertical-align", "middle").HTML(markup)
	r.byTspan(content, s, x, y, width, height, textAttrs)
	setTextAttrs(text, textAttrs)
	r.ctx.Await()
	return nil
}
