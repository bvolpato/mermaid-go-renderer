// Ported from d3-sankey/src/sankeyLinkHorizontal.js and from
// d3-shape/src/link/index.js (linkHorizontal, curveHorizontal) of
// d3-shape 1.3.7, the version that d3-sankey depends on.

package sankey

import "github.com/bvolpato/mermaid-go-renderer/internal/d3/path"

func horizontalSource(d *Link) (x, y float64) {
	return d.Source.X1, d.Y0
}

func horizontalTarget(d *Link) (x, y float64) {
	return d.Target.X0, d.Y1
}

func curveHorizontal(context *path.Path, x0, y0, x1, y1 float64) {
	context.MoveTo(x0, y0)
	x0 = (x0 + x1) / 2
	context.BezierCurveTo(x0, y0, x0, y1, x1, y1)
}

// LinkHorizontal is d3.sankeyLinkHorizontal()(d): the path data of a link
// between two nodes, as a horizontal cubic curve from the right side of the
// source to the left side of the target.
func LinkHorizontal(d *Link) string {
	buffer := path.New()
	x0, y0 := horizontalSource(d)
	x1, y1 := horizontalTarget(d)
	curveHorizontal(buffer, x0, y0, x1, y1)
	return buffer.String()
}
