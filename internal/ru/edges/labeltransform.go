// Ported from mermaid 12.2.0, rendering-util/labelTransform.ts (MIT).

package edges

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// ComputeLabelTransform is computeLabelTransform(bbox, useHtmlLabels): the
// translate transform that centers a label element at the origin. For HTML
// labels only the size of the box counts.
func ComputeLabelTransform(bbox dom.Rect, useHtmlLabels bool) string {
	if useHtmlLabels {
		return "translate(" + js.Str(-bbox.Width/2) + ", " + js.Str(-bbox.Height/2) + ")"
	}
	x := bbox.X
	y := bbox.Y
	return "translate(" + js.Str(-(x + float64(bbox.Width/2))) + ", " + js.Str(-(y + float64(bbox.Height/2))) + ")"
}
