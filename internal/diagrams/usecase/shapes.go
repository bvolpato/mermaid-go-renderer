// Ported from mermaid 12.2.0,
// rendering-util/rendering-elements/shapes/usecaseActor.ts,
// usecaseActorHollow.ts, usecaseActorAwesome.ts, usecaseActorIcon.ts,
// usecaseEllipse.ts, usecaseBusiness.ts and usecaseJsonTable.ts: the node
// shapes that only the usecase diagram uses (MIT). Each handler registers
// itself under the name that it has in rendering-elements/shapes.ts.
//
// The properties of a node that rendering-util/types.ts does not have are
// in node.Extra: business (bool), jsonRows ([]UsecaseJsonRow) and
// accessibleName (string).

package usecase

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func init() {
	ru.RegisterShape("usecaseActor", usecaseActor)
	ru.RegisterShape("usecaseActorHollow", usecaseActorHollow)
	ru.RegisterShape("usecaseActorAwesome", usecaseActorAwesome)
	ru.RegisterShape("usecaseActorIcon", usecaseActorIcon)
	ru.RegisterShape("usecaseBusiness", usecaseBusiness)
	ru.RegisterShape("usecaseEllipse", usecaseEllipse)
	ru.RegisterShape("usecaseJsonTable", usecaseJSONTable)
}

const (
	actorFigureWidth  = 56.0
	actorFigureHeight = 72.0

	actorLabelGap       = 8.0
	stereotypeLabelGap  = 2.0
	defaultActorPadding = 8.0
)

var variantClass = map[string]string{
	"normal":  "usecase-actor-shape usecase-actor-normal",
	"hollow":  "usecase-actor-hollow",
	"awesome": "usecase-actor-awesome",
	"icon":    "usecase-actor-icon",
}

// businessMarkerAngle is Math.PI / 3. The division is done on the float64
// value of pi, as in JavaScript: a constant expression would be exact.
var businessMarkerAngle = func() float64 {
	pi := math.Pi
	return pi / 3
}()

const businessMarkerOffsetRatio = 0.6

func businessMarkerPathForCircle(centerY, radius float64) string {
	centerOffset := radius * businessMarkerOffsetRatio
	halfChord := radius * math.Sqrt(1-js.Pow(businessMarkerOffsetRatio, 2))
	directionX := js.Cos(businessMarkerAngle)
	directionY := -js.Sin(businessMarkerAngle)
	centerX := float64(centerOffset * -directionY)
	markerCenterY := centerY + float64(centerOffset*directionX)
	deltaX := float64(halfChord * directionX)
	deltaY := float64(halfChord * directionY)

	return "M " + js.Str(centerX-deltaX) + " " + js.Str(markerCenterY-deltaY) + " L " + js.Str(centerX+deltaX) + " " +
		js.Str(markerCenterY+deltaY)
}

var businessMarkerPath = map[string]string{
	"normal":  businessMarkerPathForCircle(-24, 12),
	"hollow":  businessMarkerPathForCircle(-23, 9),
	"awesome": businessMarkerPathForCircle(-21, 13),
	"icon":    "M 12 -8 L 26 -26",
}

// actorGlyphRenderer is ActorGlyphRenderer: it draws the figure of an
// actor into a group.
type actorGlyphRenderer func(ctx *render.Context, group *dom.Node, node *ru.Node) error

// positionLabel is positionLabel(label, bbox, centerY) of usecaseActor.ts
// and usecaseBusiness.ts: it centers a label at x 0 and at centerY.
func positionLabel(label *dom.Node, bbox ru.LabelBBox, centerY float64) {
	originX := bbox.X - bbox.LeftOr0()
	originY := bbox.Y - bbox.TopOr0()
	label.Attr(
		"transform",
		"translate("+js.Str(float64(-bbox.Width/2)-originX)+","+js.Str(centerY-float64(bbox.Height/2)-originY)+")",
	)
}

// roughFill is the options object { fill: 'none' } for a hollow figure, or
// an empty object.
func roughFill(hollow bool) *js.Obj[any] {
	o := js.NewObj[any]()
	if hollow {
		o.Set("fill", "none")
	}
	return o
}

// appendActorPath is appendActorPath(group, node, pathData, className, hollow).
func appendActorPath(ctx *render.Context, group *dom.Node, node *ru.Node, pathData, className string, hollow bool) (*dom.Node, error) {
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, group)
		path, err := rc.Path(pathData, ru.UserNodeOverrides(ctx, node, roughFill(hollow)))
		if err != nil {
			return nil, err
		}
		return group.InsertNode(path, ":first-child").Attr("class", className), nil
	}

	path := group.Append("path").Attr("class", className).Attr("d", pathData)
	if hollow {
		path.Attr("fill", "none")
	}
	return path, nil
}

// appendActorCircle is appendActorCircle(group, node, cx, cy, radius, className, hollow).
func appendActorCircle(ctx *render.Context, group *dom.Node, node *ru.Node, cx, cy, radius float64, className string, hollow bool) (*dom.Node, error) {
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, group)
		circle, err := rc.Circle(
			cx,
			cy,
			radius*2,
			ru.UserNodeOverrides(ctx, node, roughFill(hollow)),
		)
		if err != nil {
			return nil, err
		}
		return group.InsertNode(circle, ":first-child").Attr("class", className), nil
	}

	circle := group.
		Append("circle").
		Attr("class", className).
		Attr("cx", cx).
		Attr("cy", cy).
		Attr("r", radius)
	if hollow {
		circle.Attr("fill", "none")
	}
	return circle, nil
}

// isBusiness is node.business, with JavaScript truthiness.
func isBusiness(node *ru.Node) bool {
	return js.Truthy(node.Get("business"))
}

func appendBusinessActorMarker(group *dom.Node, node *ru.Node, variant string) {
	if !isBusiness(node) {
		return
	}

	group.
		Append("path").
		Attr("class", "usecase-business-marker usecase-actor-business-marker").
		Attr("d", businessMarkerPath[variant]).
		Attr("fill", "none").
		Attr("style", "stroke: inherit !important; stroke-width: inherit !important")
}

// accessibleNameOf is `node.accessibleName ?? node.label`. The second
// result is false when the value is undefined.
func accessibleNameOf(node *ru.Node) (string, bool) {
	if v := node.Get("accessibleName"); !js.IsNullish(v) {
		return js.ToString(v), true
	}
	return node.Label, node.HasLabel
}

// stereotypeLabelOptions are the options of the stereotype label of an
// actor and of a business use case.
func stereotypeLabelOptions(node *ru.Node, labelStyles string) ru.InsertLabelOptions {
	return ru.InsertLabelOptions{
		LabelStyle:       labelStyles,
		UseHtmlLabels:    node.UseHtmlLabels,
		UseHtmlLabelsSet: node.UseHtmlLabelsSet,
		Padding:          0,
		CenterLabel:      true,
		Width:            node.WrappingWidth,
	}
}

// renderUsecaseActor is renderUsecaseActor(parent, node, variant, drawGlyph).
// The four actor shapes return its promise, so it has no await of a caller
// at its end: see actorShapeHandler.
func renderUsecaseActor(ctx *render.Context, parent *dom.Node, node *ru.Node, variant string, drawGlyph actorGlyphRenderer) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	// Actor icons render their own frame, and actor stereotypes render in a dedicated label below.
	// Remove those fields before measurement so the generic helper does not add icon decoration or
	// switch to the C4 multi-section label renderer.
	labelNode := node
	if variant == "icon" || node.HasStereotype {
		c := *node
		if variant == "icon" {
			c.Icon = ""
		}
		c.Stereotype, c.HasStereotype = "", false
		labelNode = &c
	}
	res, err := ru.LabelHelper(
		ctx,
		parent,
		labelNode,
		ru.GetNodeClasses(node, "usecase-actor-variant usecase-actor-"+variant),
	)
	if err != nil {
		return nil, err
	}
	shapeSvg, labelBox, label := res.ShapeSvg, res.BBox, res.Label

	// `role="img"` without a name is worse than no role at all, so fall back to the
	// label and skip the role entirely when neither is available.
	if accessibleName, _ := accessibleNameOf(node); accessibleName != "" {
		shapeSvg.Attr("role", "img").Attr("aria-label", accessibleName)
	}

	label.Attr("class", "label actor-label usecase-actor-label")

	// Per-item colour slot, stamped once here so all four actor variants share it.
	ru.StampColorSlot(shapeSvg, node.ColorIndex, ctx.Cfg("theme"), ctx.Cfg("themeVariables", "borderColorArray"))

	var stereotypeLabel *dom.Node
	var stereotypeBox ru.LabelBBox
	if node.Stereotype != "" {
		stereotype, err := ru.InsertLabel(ctx, shapeSvg, "«"+node.Stereotype+"»", stereotypeLabelOptions(node, labelStyles))
		if err != nil {
			return nil, err
		}
		stereotypeLabel = stereotype.Label.Attr("class", "label usecase-stereotype")
		stereotypeBox = stereotype.BBox
	}

	var actorStyle any
	if nodeStyles != "" {
		actorStyle = nodeStyles
	}
	actorGroup := shapeSvg.
		Append("g").
		Attr("class", "usecase-actor-glyph "+variantClass[variant]).
		Attr("style", actorStyle)
	if err := drawGlyph(ctx, actorGroup, node); err != nil {
		return nil, err
	}
	appendBusinessActorMarker(actorGroup, node, variant)

	padding := ru.NN(node.Padding, defaultActorPadding)
	stereotypeHeight := 0.0
	stereotypeWidth := 0.0
	stereotypeContent := 0.0
	if stereotypeLabel != nil {
		stereotypeHeight = stereotypeBox.Height
		stereotypeWidth = stereotypeBox.Width
		stereotypeContent = stereotypeHeight + stereotypeLabelGap
	}
	contentHeight := actorFigureHeight +
		actorLabelGap +
		labelBox.Height +
		stereotypeContent
	totalWidth := js.Max(actorFigureWidth, labelBox.Width, stereotypeWidth) + float64(padding*2)
	totalHeight := contentHeight + float64(padding*2)
	contentTop := float64(-contentHeight / 2)

	actorGroup.Attr("transform", "translate(0,"+js.Str(contentTop+float64(actorFigureHeight/2))+")")

	nextLabelCenter := contentTop + actorFigureHeight + actorLabelGap
	if stereotypeLabel != nil {
		nextLabelCenter += float64(stereotypeHeight / 2)
		positionLabel(stereotypeLabel, stereotypeBox, nextLabelCenter)
		nextLabelCenter += float64(stereotypeHeight/2) + stereotypeLabelGap
	}
	positionLabel(label, labelBox, nextLabelCenter+float64(labelBox.Height/2))

	outline := shapeSvg.
		Insert("rect", ":first-child").
		Attr("class", "usecase-actor-outline").
		Attr("x", -totalWidth/2).
		Attr("y", -totalHeight/2).
		Attr("width", totalWidth).
		Attr("height", totalHeight).
		Attr("opacity", 0).
		Attr("aria-hidden", "true")

	ru.UpdateNodeBounds(node, outline)
	node.Intersect = func(point ru.Point) ru.Point { return ru.IntersectRect(ru.NodeBounds(node), point) }

	return shapeSvg, nil
}

// actorShapeHandler is the body that the four actor shapes share:
//
//	return renderUsecaseActor(parent, node, variant, drawGlyph);
//
// An async function that returns the promise of another async function
// takes two more turns of the microtask queue.
func actorShapeHandler(variant string, drawGlyph actorGlyphRenderer) ru.ShapeHandler {
	return func(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
		shapeSvg, err := renderUsecaseActor(ctx, parent, node, variant, drawGlyph)
		if err != nil {
			return nil, err
		}
		ctx.AwaitN(2)
		return shapeSvg, nil
	}
}

// syncGlyph is a glyph renderer that is not async: renderUsecaseActor
// awaits its result, which takes one turn of the microtask queue.
func syncGlyph(draw func(ctx *render.Context, group *dom.Node, node *ru.Node) error) actorGlyphRenderer {
	return func(ctx *render.Context, group *dom.Node, node *ru.Node) error {
		if err := draw(ctx, group, node); err != nil {
			return err
		}
		ctx.Await()
		return nil
	}
}

var stickFigurePath = strings.Join([]string{
	"M 0 -12",
	"C 6.627 -12 12 -17.373 12 -24",
	"C 12 -30.627 6.627 -36 0 -36",
	"C -6.627 -36 -12 -30.627 -12 -24",
	"C -12 -17.373 -6.627 -12 0 -12 Z",
	"M 0 -12 V 8",
	"M -17 -5 H 17",
	"M 0 8 L -15 28",
	"M 0 8 L 15 28",
}, " ")

// usecaseActor is usecaseActor(parent, node).
var usecaseActor = actorShapeHandler("normal", syncGlyph(func(ctx *render.Context, group *dom.Node, actorNode *ru.Node) error {
	_, err := appendActorPath(ctx, group, actorNode, stickFigurePath, "usecase-actor-stick", false)
	return err
}))

var hollowBodyPath = strings.Join([]string{
	"M -22 -10",
	"H 22",
	"V 0",
	"H 6",
	"L 22 17",
	"L 13 28",
	"L 0 13",
	"L -13 28",
	"L -22 17",
	"L -6 0",
	"H -22",
	"Z",
}, " ")

// usecaseActorHollow is usecaseActorHollow(parent, node).
var usecaseActorHollow = actorShapeHandler("hollow", syncGlyph(func(ctx *render.Context, group *dom.Node, actorNode *ru.Node) error {
	if _, err := appendActorCircle(ctx, group, actorNode, 0, -23, 9, "usecase-actor-hollow-head", true); err != nil {
		return err
	}
	_, err := appendActorPath(ctx, group, actorNode, hollowBodyPath, "usecase-actor-hollow-body", true)
	return err
}))

// A bundled user silhouette. Keeping the path local makes this variant independent of icon packs.
var awesomeSilhouettePath = strings.Join([]string{
	"M 0 -34",
	"C 7.18 -34 13 -28.18 13 -21",
	"C 13 -13.82 7.18 -8 0 -8",
	"C -7.18 -8 -13 -13.82 -13 -21",
	"C -13 -28.18 -7.18 -34 0 -34 Z",
	"M -24 25",
	"C -24 7 -14 -3 0 -3",
	"C 14 -3 24 7 24 25",
	"C 24 28 21 30 18 30",
	"H -18",
	"C -21 30 -24 28 -24 25 Z",
}, " ")

// usecaseActorAwesome is usecaseActorAwesome(parent, node).
var usecaseActorAwesome = actorShapeHandler("awesome", syncGlyph(func(ctx *render.Context, group *dom.Node, actorNode *ru.Node) error {
	_, err := appendActorPath(ctx, group, actorNode, awesomeSilhouettePath, "usecase-actor-awesome-silhouette", false)
	return err
}))

const (
	iconFrameSize = 52.0
	iconSize      = 42.0
	iconCenterY   = -2.0
)

// drawIconActor is drawIconActor(group, node) of usecaseActorIcon.ts.
func drawIconActor(ctx *render.Context, group *dom.Node, node *ru.Node) error {
	frameX := -iconFrameSize / 2
	frameY := iconCenterY - iconFrameSize/2

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, group)
		frame, err := rc.Rectangle(
			frameX,
			frameY,
			iconFrameSize,
			iconFrameSize,
			ru.UserNodeOverrides(ctx, node, js.NewObj[any]()),
		)
		if err != nil {
			return err
		}
		group.InsertNode(frame, ":first-child").Attr("class", "usecase-actor-icon-frame")
	} else {
		group.
			Append("rect").
			Attr("class", "usecase-actor-icon-frame").
			Attr("x", frameX).
			Attr("y", frameY).
			Attr("width", iconFrameSize).
			Attr("height", iconFrameSize).
			Attr("rx", 4).
			Attr("ry", 4)
	}

	iconName := node.Icon
	registeredName := iconName
	if !strings.Contains(iconName, ":") {
		registeredName = "fa:" + iconName
	}
	available := ru.IsIconAvailable(ctx, registeredName)
	// await isIconAvailable(...): the await inside it and the await here.
	ctx.AwaitN(2)
	iconSvg, err := ru.GetIconSVG(ctx, iconName, map[string]any{
		"height":         iconSize,
		"width":          iconSize,
		"fallbackPrefix": "fa",
	}, nil)
	if err != nil {
		return err
	}
	// await getIconSVG(...): the await inside it and the await here.
	ctx.AwaitN(2)
	symbolClass := "usecase-actor-icon-symbol"
	if !available {
		symbolClass += " usecase-actor-icon-fallback"
	}
	iconGroup := group.
		Append("g").
		Attr("class", symbolClass).
		Attr("aria-hidden", "true").
		HTML("<g>" + iconSvg + "</g>")

	iconBox := iconGroup.BBox()
	iconGroup.Attr(
		"transform",
		"translate("+js.Str(float64(-iconBox.Width/2)-iconBox.X)+","+
			js.Str(iconCenterY-float64(iconBox.Height/2)-iconBox.Y)+
			")",
	)
	// The await of the caller.
	ctx.Await()
	return nil
}

// usecaseActorIcon is usecaseActorIcon(parent, node).
var usecaseActorIcon = actorShapeHandler("icon", drawIconActor)

// usecaseEllipse is usecaseEllipse(parent, node).
func usecaseEllipse(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node))
	if err != nil {
		return nil, err
	}
	shapeSvg, bbox, halfPadding := res.ShapeSvg, res.BBox, res.HalfPadding

	// Per-item colour slot. A no-op unless the active theme carries a palette; `usecase/styles.ts`
	// defines the matching `[data-color-id]` rules.
	ru.StampColorSlot(shapeSvg, node.ColorIndex, ctx.Cfg("theme"), ctx.Cfg("themeVariables", "borderColorArray"))

	// Calculate ellipse dimensions with padding
	padding := halfPadding
	// Same air above and below the label as beside it: a flatter oval leaves no
	// room on its sides for the markers of several associations.
	radiusX := float64(bbox.Width/2) + float64(padding*2)
	radiusY := float64(bbox.Height/2) + float64(padding*2)

	var ellipseElem *dom.Node

	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		options := ru.UserNodeOverrides(ctx, node, js.NewObj[any]())
		roughNode, err := rc.Ellipse(0, 0, radiusX*2, radiusY*2, options)
		if err != nil {
			return nil, err
		}

		ellipseElem = shapeSvg.InsertNode(roughNode, ":first-child")
		ellipseElem.Attr("class", "basic label-container")

		// if (cssStyles) { ellipseElem.attr('style', cssStyles); }
		if cssStyles := ru.CSSStylesAttr(node); cssStyles != nil {
			ellipseElem.Attr("style", cssStyles)
		}
	} else {
		ellipseElem = shapeSvg.
			Insert("ellipse", ":first-child").
			Attr("class", "basic label-container").
			Attr("style", nodeStyles).
			Attr("rx", radiusX).
			Attr("ry", radiusY).
			Attr("cx", 0).
			Attr("cy", 0)
	}

	node.Width = radiusX * 2
	node.Height = radiusY * 2

	ru.UpdateNodeBounds(node, ellipseElem)

	node.CalcIntersect = func(bounds ru.Bounds, point ru.Point) ru.Point {
		rx := bounds.Width / 2
		ry := bounds.Height / 2
		return ru.IntersectEllipse(bounds, rx, ry, point)
	}

	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectEllipse(ru.NodeBounds(node), radiusX, radiusY, point)
	}

	return shapeSvg, nil
}

// usecaseBusiness is usecaseBusiness(parent, node).
func usecaseBusiness(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	node.LabelStyle = labelStyles
	// const labelNode = businessNode.stereotype ? { ...node, stereotype: undefined } : node
	labelNode := node
	if node.Stereotype != "" {
		c := *node
		c.Stereotype, c.HasStereotype = "", false
		labelNode = &c
	}
	res, err := ru.LabelHelper(ctx, parent, labelNode, ru.GetNodeClasses(node, "usecase-business-shape"))
	if err != nil {
		return nil, err
	}
	shapeSvg, labelBox, halfPadding, label := res.ShapeSvg, res.BBox, res.HalfPadding, res.Label

	label.Attr("class", "label usecase-label")

	// Per-item colour slot -- see `usecaseEllipse`.
	ru.StampColorSlot(shapeSvg, node.ColorIndex, ctx.Cfg("theme"), ctx.Cfg("themeVariables", "borderColorArray"))
	var stereotypeLabel *dom.Node
	var stereotypeBox ru.LabelBBox
	if node.Stereotype != "" {
		stereotype, err := ru.InsertLabel(ctx, shapeSvg, "«"+node.Stereotype+"»", stereotypeLabelOptions(node, labelStyles))
		if err != nil {
			return nil, err
		}
		stereotypeLabel = stereotype.Label.Attr("class", "label usecase-stereotype")
		stereotypeBox = stereotype.BBox
	}

	labelGap := 0.0
	stereotypeHeight := 0.0
	stereotypeWidth := 0.0
	if stereotypeLabel != nil {
		labelGap = 2
		stereotypeHeight = stereotypeBox.Height
		stereotypeWidth = stereotypeBox.Width
	}
	labelHeight := labelBox.Height + stereotypeHeight + labelGap
	labelWidth := js.Max(labelBox.Width, stereotypeWidth)
	padding := halfPadding
	// Same vertical air as `usecaseEllipse`, so both ovals share one silhouette.
	radiusX := float64(labelWidth/2) + float64(padding*2)
	radiusY := float64(labelHeight/2) + float64(padding*2)
	markerInset := js.Min(js.Max(padding/5, 1), padding/2)
	markerStartX := float64(labelWidth/2) + markerInset
	markerEndX := radiusX - markerInset
	normalizedStartX := markerStartX / radiusX
	normalizedEndX := markerEndX / radiusX
	markerStartY := radiusY * math.Sqrt(js.Max(0, 1-float64(normalizedStartX*normalizedStartX)))
	markerEndY := -radiusY * math.Sqrt(js.Max(0, 1-float64(normalizedEndX*normalizedEndX)))

	if stereotypeLabel != nil {
		positionLabel(stereotypeLabel, stereotypeBox, float64(-labelHeight/2)+float64(stereotypeBox.Height/2))
	}
	positionLabel(label, labelBox, float64(labelHeight/2)-float64(labelBox.Height/2))

	var ellipseElement *dom.Node
	if node.Look == "handDrawn" {
		rc := ru.NewRoughSVG(ctx, shapeSvg)
		roughEllipse, err := rc.Ellipse(0, 0, radiusX*2, radiusY*2, ru.UserNodeOverrides(ctx, node, js.NewObj[any]()))
		if err != nil {
			return nil, err
		}
		ellipseElement = shapeSvg.
			InsertNode(roughEllipse, ":first-child").
			Attr("class", "basic label-container usecase-business-ellipse")
	} else {
		ellipseElement = shapeSvg.
			Insert("ellipse", ":first-child").
			Attr("class", "basic label-container usecase-business-ellipse").
			Attr("style", nodeStyles).
			Attr("rx", radiusX).
			Attr("ry", radiusY).
			Attr("cx", 0).
			Attr("cy", 0)
	}

	var markerStyle any
	if nodeStyles != "" {
		markerStyle = nodeStyles
	}
	shapeSvg.
		Append("path").
		Attr("class", "usecase-business-marker").
		Attr("d", "M "+js.Str(markerStartX)+" "+js.Str(markerStartY)+" L "+js.Str(markerEndX)+" "+js.Str(markerEndY)).
		Attr("fill", "none").
		Attr("style", markerStyle)

	// `role="img"` without a name is worse than no role at all, so fall back to the
	// label and skip the role entirely when neither is available.
	if accessibleName, _ := accessibleNameOf(node); accessibleName != "" {
		shapeSvg.Attr("role", "img").Attr("aria-label", accessibleName)
	}

	ru.UpdateNodeBounds(node, ellipseElement)
	node.CalcIntersect = func(bounds ru.Bounds, point ru.Point) ru.Point {
		return ru.IntersectEllipse(bounds, bounds.Width/2, bounds.Height/2, point)
	}
	node.Intersect = func(point ru.Point) ru.Point {
		return ru.IntersectEllipse(ru.NodeBounds(node), radiusX, radiusY, point)
	}
	return shapeSvg, nil
}

const (
	cellPaddingX       = 8.0
	cellPaddingY       = 4.0
	defaultBorderWidth = 1.0
)

// positionCellLabel is positionLabel(label, bbox, centerX, centerY) of
// usecaseJsonTable.ts.
func positionCellLabel(label *dom.Node, bbox ru.LabelBBox, centerX, centerY float64) {
	originX := bbox.X - bbox.LeftOr0()
	originY := bbox.Y - bbox.TopOr0()
	label.Attr(
		"transform",
		"translate("+js.Str(centerX-float64(bbox.Width/2)-originX)+","+js.Str(centerY-float64(bbox.Height/2)-originY)+")",
	)
}

// numericStrokeWidth is numericStrokeWidth(value). hasValue is false for
// undefined.
func numericStrokeWidth(value string, hasValue bool) float64 {
	if !hasValue {
		value = ""
	}
	parsed := js.ParseFloat(value)
	if !math.IsNaN(parsed) && !math.IsInf(parsed, 0) && parsed >= 0 {
		return parsed
	}
	return defaultBorderWidth
}

type measuredCell struct {
	label *dom.Node
	bbox  ru.LabelBBox
}

type measuredRow struct {
	key   measuredCell
	value measuredCell
}

// jsonRowsOf is `tableNode.jsonRows ?? []`. The db stores the rows as
// []UsecaseJsonRow. A node that was made from a JavaScript object has them
// as a dynamic value: an array of objects.
func jsonRowsOf(node *ru.Node) []UsecaseJsonRow {
	switch rows := node.Get("jsonRows").(type) {
	case []UsecaseJsonRow:
		return rows
	case []any:
		out := make([]UsecaseJsonRow, len(rows))
		for i, row := range rows {
			o := js.AsObj(row)
			text := func(key string) string {
				if v := js.Path(o, key); !js.IsNullish(v) {
					return js.ToString(v)
				}
				return ""
			}
			out[i] = UsecaseJsonRow{Key: text("key"), AccessibleKey: text("accessibleKey"), Value: text("value")}
		}
		return out
	}
	return nil
}

// usecaseJSONTable is usecaseJsonTable(parent, node).
func usecaseJSONTable(ctx *render.Context, parent *dom.Node, node *ru.Node, _ *ru.ShapeRenderOptions) (*dom.Node, error) {
	styles := ru.Styles2String(node)
	labelStyles, nodeStyles := styles.LabelStyles, styles.NodeStyles
	stylesMap := ru.CompileStyles(node)
	node.LabelStyle = labelStyles

	res, err := ru.LabelHelper(ctx, parent, node, ru.GetNodeClasses(node, "usecase-json-table"))
	if err != nil {
		return nil, err
	}
	shapeSvg, titleBox, titleLabel := res.ShapeSvg, res.BBox, res.Label
	shapeSvg.Attr("role", "img")
	if accessibleName := node.Get("accessibleName"); js.Truthy(accessibleName) {
		shapeSvg.Attr("aria-label", js.ToString(accessibleName))
	}
	titleLabel.Attr("class", "label usecase-json-title")

	tableGroup := shapeSvg.Append("g").Attr("class", "usecase-json-table-grid")
	var measuredRows []measuredRow
	cellOptions := ru.InsertLabelOptions{
		LabelStyle:       labelStyles,
		UseHtmlLabels:    node.UseHtmlLabels,
		UseHtmlLabelsSet: node.UseHtmlLabelsSet,
		Padding:          0,
		CenterLabel:      true,
		Width:            ru.Undef,
	}
	for _, row := range jsonRowsOf(node) {
		key, err := ru.InsertLabel(ctx, tableGroup, row.Key, cellOptions)
		if err != nil {
			return nil, err
		}
		value, err := ru.InsertLabel(ctx, tableGroup, row.Value, cellOptions)
		if err != nil {
			return nil, err
		}
		measuredRows = append(measuredRows, measuredRow{
			key: measuredCell{
				label: key.Label.Attr("class", "label usecase-json-key"),
				bbox:  key.BBox,
			},
			value: measuredCell{
				label: value.Label.Attr("class", "label usecase-json-value"),
				bbox:  value.BBox,
			},
		})
	}

	keyWidths := []float64{0}
	valueWidths := []float64{0}
	for _, row := range measuredRows {
		keyWidths = append(keyWidths, row.key.bbox.Width)
		valueWidths = append(valueWidths, row.value.bbox.Width)
	}
	keyWidth := js.Max(keyWidths...) + cellPaddingX*2
	measuredValueWidth := js.Max(valueWidths...) + cellPaddingX*2
	titleWidth := titleBox.Width + cellPaddingX*2
	innerWidth := js.Max(titleWidth, keyWidth+measuredValueWidth)
	valueWidth := measuredValueWidth + js.Max(0, innerWidth-keyWidth-measuredValueWidth)
	titleHeight := titleBox.Height + cellPaddingY*2
	rowHeights := make([]float64, len(measuredRows))
	rowHeightSum := 0.0
	for i, row := range measuredRows {
		rowHeights[i] = js.Max(row.key.bbox.Height, row.value.bbox.Height) + cellPaddingY*2
		rowHeightSum = rowHeightSum + rowHeights[i]
	}
	innerHeight := titleHeight + rowHeightSum
	strokeWidth, hasStrokeWidth := stylesMap.Get("stroke-width")
	borderWidth := numericStrokeWidth(strokeWidth, hasStrokeWidth)
	totalWidth := innerWidth + float64(borderWidth*2)
	totalHeight := innerHeight + float64(borderWidth*2)
	left := float64(-innerWidth / 2)
	top := float64(-innerHeight / 2)

	var roughSvg *ru.RoughSVG
	var roughOptions *js.Obj[any]
	if node.Look == "handDrawn" {
		roughSvg = ru.NewRoughSVG(ctx, shapeSvg)
		roughOptions = ru.UserNodeOverrides(ctx, node, js.NewObj[any]())
	}
	insertCell := func(container *dom.Node, className string, x, y, width, height float64) (*dom.Node, error) {
		if roughSvg != nil {
			roughCell, err := roughSvg.Rectangle(x, y, width, height, roughOptions)
			if err != nil {
				return nil, err
			}
			return container.InsertNode(roughCell, ":first-child").Attr("class", className), nil
		}
		return container.
			Insert("rect", ":first-child").
			Attr("class", className).
			Attr("x", x).
			Attr("y", y).
			Attr("width", width).
			Attr("height", height).
			Attr("style", nodeStyles), nil
	}

	outerBorder, err := insertCell(
		shapeSvg,
		"label-container usecase-json-border",
		-totalWidth/2,
		-totalHeight/2,
		totalWidth,
		totalHeight,
	)
	if err != nil {
		return nil, err
	}

	if _, err := insertCell(
		tableGroup,
		"usecase-json-cell usecase-json-title-cell",
		left,
		top,
		innerWidth,
		titleHeight,
	); err != nil {
		return nil, err
	}
	tableGroup.AppendChild(titleLabel)
	positionCellLabel(titleLabel, titleBox, 0, top+float64(titleHeight/2))

	rowTop := top + titleHeight
	for index, row := range measuredRows {
		key, value := row.key, row.value
		rowHeight := rowHeights[index]
		rowGroup := tableGroup.
			Append("g").
			Attr("class", "usecase-json-row").
			Attr("data-row-index", index).
			Attr("transform", "translate(0,"+js.Str(rowTop)+")")

		if _, err := insertCell(rowGroup, "usecase-json-cell usecase-json-key-cell", left, 0, keyWidth, rowHeight); err != nil {
			return nil, err
		}
		if _, err := insertCell(
			rowGroup,
			"usecase-json-cell usecase-json-value-cell",
			left+keyWidth,
			0,
			valueWidth,
			rowHeight,
		); err != nil {
			return nil, err
		}

		rowGroup.AppendChild(key.label)
		rowGroup.AppendChild(value.label)
		positionCellLabel(key.label, key.bbox, left+float64(keyWidth/2), rowHeight/2)
		positionCellLabel(value.label, value.bbox, left+keyWidth+float64(valueWidth/2), rowHeight/2)
		rowTop += rowHeight
	}

	ru.UpdateNodeBounds(node, outerBorder)
	node.Intersect = func(point ru.Point) ru.Point { return ru.IntersectRect(ru.NodeBounds(node), point) }
	return shapeSvg, nil
}
