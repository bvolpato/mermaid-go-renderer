// Ported from mermaid 12.2.0,
// rendering-util/rendering-elements/shapes/mindmapCircle.ts and
// defaultMindmapNode.ts (MIT).

package mindmap

import (
	"errors"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The two shapes that only the mindmap uses are not in the shape packages
// yet. The copies below are registered when the shape packages of the
// program did not register them, so a port in internal/ru/shapes wins.

var localShapes sync.Once

// registerLocalShapes registers the local copies of the shapes that no
// linked package has registered. It runs at the first draw, after the init
// functions of every package.
func registerLocalShapes() {
	localShapes.Do(func() {
		for _, s := range []struct {
			name    string
			handler ru.ShapeHandler
		}{
			{"mindmapCircle", mindmapCircle},
			{"defaultMindmapNode", defaultMindmapNode},
		} {
			var notPorted *ru.NotPortedError
			if _, err := ru.LookupShape(s.name); errors.As(err, &notPorted) {
				ru.RegisterShape(s.name, s.handler)
			}
		}
	})
}

// Local copy of mindmapCircle (rendering-elements/shapes/mindmapCircle.ts);
// replace with internal/ru/shapes.
//
// mindmapCircle is circle(parent, node, { padding: node.padding ?? 0 }).
func mindmapCircle(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	circle, err := ru.LookupShape("circle")
	if err != nil {
		return nil, err
	}
	options := &ru.ShapeRenderOptions{Padding: ru.NN(node.Padding, 0)}
	return circle(ctx, parent, node, options)
}

// Local copy of defaultMindmapNode
// (rendering-elements/shapes/defaultMindmapNode.ts); replace with
// internal/ru/shapes.
func defaultMindmapNode(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	node.LabelStyle = styles.LabelStyles

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, halfPadding, label := res.ShapeSvg, res.BBox, res.HalfPadding, res.Label

	w := bbox.Width + float64(8*halfPadding)
	h := bbox.Height + float64(2*halfPadding)
	const rd = 5.0

	n := js.Str
	// Neo look: rounded top corners only, flat bottom edge (tab-like shape).
	var rectPath string
	if node.Look == "neo" {
		rectPath = "\n" +
			"    M" + n(-w/2) + " " + n(float64(h/2)-rd) + "\n" +
			"    v" + n(-h+float64(2*rd)) + "\n" +
			"    q0,-" + n(rd) + " " + n(rd) + ",-" + n(rd) + "\n" +
			"    h" + n(w-float64(2*rd)) + "\n" +
			"    q" + n(rd) + ",0 " + n(rd) + "," + n(rd) + "\n" +
			"    v" + n(h-rd) + "\n" +
			"    H" + n(-w/2) + "\n" +
			"    Z\n" +
			"  "
	} else {
		rectPath = "\n" +
			"    M" + n(-w/2) + " " + n(float64(h/2)-rd) + "\n" +
			"    v" + n(-h+float64(2*rd)) + "\n" +
			"    q0,-" + n(rd) + " " + n(rd) + ",-" + n(rd) + "\n" +
			"    h" + n(w-float64(2*rd)) + "\n" +
			"    q" + n(rd) + ",0 " + n(rd) + "," + n(rd) + "\n" +
			"    v" + n(h-float64(2*rd)) + "\n" +
			"    q0," + n(rd) + " " + n(-rd) + "," + n(rd) + "\n" +
			"    h" + n(-(w - float64(2*rd))) + "\n" +
			"    q" + n(-rd) + ",0 " + n(-rd) + "," + n(-rd) + "\n" +
			"    Z\n" +
			"  "
	}

	if node.DomID == "" {
		return nil, errors.New(`defaultMindmapNode: node "` + node.ID + `" is missing a domId — was render.ts domId prefixing skipped?`)
	}

	bg := shapeSvg.
		Append("path").
		Attr("id", node.DomID).
		Attr("class", "node-bkg node-"+n(node.Type)).
		Attr("style", styles.NodeStyles).
		Attr("d", rectPath)

	shapeSvg.
		Append("line").
		Attr("class", "node-line-").
		Attr("x1", -w/2).
		Attr("y1", h/2).
		Attr("x2", w/2).
		Attr("y2", h/2)

	label.Attr("transform", "translate("+n(-bbox.Width/2)+", "+n(-bbox.Height/2)+")")
	shapeSvg.AppendNode(label)

	ru.UpdateNodeBounds(node, bg)
	node.CalcIntersect = func(bounds ru.Bounds, point ru.Point) ru.Point {
		return ru.IntersectRect(bounds, point)
	}
	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectRect(ru.Bounds{X: node.X, Y: node.Y, Width: node.Width, Height: node.Height}, point)
	}

	return shapeSvg, nil
}
