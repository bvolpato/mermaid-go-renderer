// Ported from mermaid 12.2.0, diagrams/c4/c4Renderer.ts (MIT).

package c4

import (
	"errors"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// rendererState holds the module-level variables of c4Renderer.ts.
type rendererState struct {
	globalBoundaryMaxX float64
	globalBoundaryMaxY float64

	c4ShapeInRow    float64
	c4BoundaryInRow float64

	// conf is the c4 config at render time. Upstream starts with an empty
	// object, and draw replaces it with the c4 section of the config.
	conf *js.Obj[any]
}

func newRendererState() rendererState {
	return rendererState{c4ShapeInRow: 4, c4BoundaryInRow: 2, conf: js.NewObj[any]()}
}

// renderer is the state of one draw call.
type renderer struct {
	ctx *render.Context
	db  *db
	rs  *rendererState
}

// num is `Number(obj[key])` for the layout fields that the renderer keeps
// in the bags of the db. A missing key is NaN, as undefined is in the
// arithmetic of upstream.
func num(obj *js.Obj[any], key string) float64 {
	if obj == nil {
		return math.NaN()
	}
	return js.ToNumber(obj.Get(key))
}

// confNum reads a number of the c4 config.
//
// Upstream uses the config values in arithmetic without a conversion. A
// config value that is a string would concatenate there. This port converts
// each value to a number.
func (r *renderer) confNum(key string) float64 {
	return js.ToNumber(r.rs.conf.Get(key))
}

// boundsData is BoundsData.
type boundsData struct {
	startx, stopx, starty, stopy float64
	widthLimit                   float64
}

// nextBoundsData is NextBoundsData.
type nextBoundsData struct {
	startx, stopx, starty, stopy float64
	cnt                          float64
}

// bounds is the class Bounds. The fields of data and nextData are undefined
// upstream until setData runs. The renderer calls setData before every
// read, so the port keeps plain numbers.
type bounds struct {
	r        *renderer
	name     any
	data     boundsData
	nextData nextBoundsData
}

// newBounds is `new Bounds(diagObj)`.
func (r *renderer) newBounds() *bounds {
	nan := math.NaN()
	b := &bounds{
		r:        r,
		name:     "",
		data:     boundsData{nan, nan, nan, nan, nan},
		nextData: nextBoundsData{nan, nan, nan, nan, 0},
	}
	r.setConf(r.dbConfig())
	return b
}

// setData is Bounds.setData(startx, stopx, starty, stopy).
func (b *bounds) setData(startx, stopx, starty, stopy float64) {
	b.data.startx = startx
	b.nextData.startx = startx
	b.data.stopx = stopx
	b.nextData.stopx = stopx
	b.data.starty = starty
	b.nextData.starty = starty
	b.data.stopy = stopy
	b.nextData.stopy = stopy
}

// updateVal is Bounds.updateVal(obj, key, val, fun). The value is never
// undefined here: setData seeds the bounds before any insert call.
func updateVal(field *float64, val float64, fun func(...float64) float64) {
	*field = fun(val, *field)
}

// insert is Bounds.insert(c4Shape).
func (b *bounds) insert(c4Shape *js.Obj[any]) {
	margin := num(c4Shape, "margin")
	width := num(c4Shape, "width")
	height := num(c4Shape, "height")

	b.nextData.cnt = b.nextData.cnt + 1
	// `setData()` seeds the bounds before any `insert()` call.
	nextStopx := b.nextData.stopx
	widthLimit := b.data.widthLimit
	var startx float64
	if b.nextData.startx == b.nextData.stopx {
		startx = nextStopx + margin
	} else {
		startx = nextStopx + float64(margin*2)
	}
	stopx := startx + width
	starty := b.nextData.starty + float64(margin*2)
	stopy := starty + height
	if startx >= widthLimit || stopx >= widthLimit || b.nextData.cnt > b.r.rs.c4ShapeInRow {
		startx = b.nextData.startx + margin + b.r.confNum("nextLinePaddingX")
		starty = b.nextData.stopy + float64(margin*2)

		stopx = startx + width
		b.nextData.stopx = stopx
		b.nextData.starty = b.nextData.stopy
		stopy = starty + height
		b.nextData.stopy = stopy
		b.nextData.cnt = 1
	}

	c4Shape.Set("x", startx)
	c4Shape.Set("y", starty)

	updateVal(&b.data.startx, startx, js.Min)
	updateVal(&b.data.starty, starty, js.Min)
	updateVal(&b.data.stopx, stopx, js.Max)
	updateVal(&b.data.stopy, stopy, js.Max)

	updateVal(&b.nextData.startx, startx, js.Min)
	updateVal(&b.nextData.starty, starty, js.Min)
	updateVal(&b.nextData.stopx, stopx, js.Max)
	updateVal(&b.nextData.stopy, stopy, js.Max)
}

// bumpLastMargin is Bounds.bumpLastMargin(margin).
func (b *bounds) bumpLastMargin(margin float64) {
	b.data.stopx += margin
	b.data.stopy += margin
}

// dbConfig is db.getConfig(): getRequiredConfig('c4'), a copy of the c4
// section of the current config.
func (r *renderer) dbConfig() *js.Obj[any] {
	c4, _ := js.Clone(r.ctx.Cfg("c4")).(*js.Obj[any])
	return c4
}

// setConf is setConf(cnf).
func (rs *rendererState) setConf(cnf *js.Obj[any]) {
	var src any
	if cnf != nil {
		src = cnf
	}
	if merged, ok := config.AssignWithDepth(rs.conf, src, 2).(*js.Obj[any]); ok && merged != nil {
		rs.conf = merged
	}

	if cnf == nil {
		return
	}
	if v := cnf.Get("fontFamily"); js.Truthy(v) {
		rs.conf.Set("messageFontFamily", v)
		rs.conf.Set("systemFontFamily", v)
		rs.conf.Set("personFontFamily", v)
	}
	if v := cnf.Get("fontSize"); js.Truthy(v) {
		rs.conf.Set("messageFontSize", v)
		rs.conf.Set("systemFontSize", v)
		rs.conf.Set("personFontSize", v)
	}
	if v := cnf.Get("fontWeight"); js.Truthy(v) {
		rs.conf.Set("messageFontWeight", v)
		rs.conf.Set("systemFontWeight", v)
		rs.conf.Set("personFontWeight", v)
	}
}

func (r *renderer) setConf(cnf *js.Obj[any]) { r.rs.setConf(cnf) }

// font is the object that boundaryFont(cnf) and messageFont(cnf) return:
// the font family, size and weight of the config keys with the prefix.
func (r *renderer) font(prefix string) *js.Obj[any] {
	conf := r.rs.conf
	f := js.NewObj[any]()
	f.Set("fontFamily", conf.Get(prefix+"FontFamily"))
	f.Set("fontSize", conf.Get(prefix+"FontSize"))
	f.Set("fontWeight", conf.Get(prefix+"FontWeight"))
	return f
}

// boundaryFont is boundaryFont(conf).
func (r *renderer) boundaryFont() *js.Obj[any] { return r.font("boundary") }

// messageFont is messageFont(conf).
func (r *renderer) messageFont() *js.Obj[any] { return r.font("message") }

// boundaryFontOf is conf.boundaryFont(): the function of the default config.
func (r *renderer) boundaryFontOf() c4Font {
	conf := r.rs.conf
	return c4Font{
		fontFamily: conf.Get("boundaryFontFamily"),
		fontSize:   conf.Get("boundaryFontSize"),
		fontWeight: conf.Get("boundaryFontWeight"),
	}
}

// messageFontOf is conf.messageFont(): the function of the default config.
func (r *renderer) messageFontOf() c4Font {
	conf := r.rs.conf
	return c4Font{
		fontFamily: conf.Get("messageFontFamily"),
		fontSize:   conf.Get("messageFontSize"),
		fontWeight: conf.Get("messageFontWeight"),
	}
}

// textElementOf is `c4Shape[textType]` for a text field that the renderer
// measures. Upstream throws a TypeError when the field is not an object.
func textElementOf(c4Shape *js.Obj[any], textType string) (*js.Obj[any], error) {
	textElement, ok := c4Shape.Get(textType).(*js.Obj[any])
	if !ok || textElement == nil {
		return nil, errTypeError("Cannot read properties of undefined (reading 'split')")
	}
	return textElement, nil
}

// textOf is `textElement.text` for a text that upstream calls split on.
func textOf(textElement *js.Obj[any]) (string, error) {
	text, ok := textElement.Get("text").(string)
	if !ok {
		return "", errTypeError("textElement.text.split is not a function")
	}
	return text, nil
}

// calculateTextWidth is calculateTextWidth(text, textConf) for a text of
// any type. Upstream returns 0 for a falsy text and throws a TypeError for
// a truthy text that is not a string.
func (r *renderer) calculateTextWidth(text any, textConf *js.Obj[any]) (float64, error) {
	if !js.Truthy(text) {
		return 0, nil
	}
	s, ok := text.(string)
	if !ok {
		return 0, errTypeError("text.split is not a function")
	}
	return utils.CalculateTextWidth(r.ctx, s, utils.TextDimensionConfigOf(textConf))
}

// calcC4ShapeTextWH is calcC4ShapeTextWH(textType, c4Shape, c4ShapeTextWrap,
// textConf, textLimitWidth). It measures a text field and stores the layout
// fields in it.
func (r *renderer) calcC4ShapeTextWH(textType string, c4Shape *js.Obj[any], c4ShapeTextWrap bool, textConf *js.Obj[any], textLimitWidth float64) (*js.Obj[any], error) {
	// `textType` is always one of the `C4Text` valued fields of `c4Shape`, and
	// the layout fields are populated below (or by an earlier measurement).
	textElement, err := textElementOf(c4Shape, textType)
	if err != nil {
		return nil, err
	}
	if !js.Truthy(textElement.Get("width")) {
		dimConf := utils.TextDimensionConfigOf(textConf)
		if c4ShapeTextWrap {
			var wrapped string
			if label := textElement.Get("text"); !js.Truthy(label) {
				// wrapLabel returns a falsy label as it is. The split below
				// then fails for a label that is not the empty string.
				if label != "" {
					return nil, errTypeError("Cannot read properties of undefined (reading 'split')")
				}
			} else {
				s, ok := label.(string)
				if !ok {
					return nil, errTypeError("label.split is not a function")
				}
				if wrapped, err = utils.WrapLabel(r.ctx, s, textLimitWidth, utils.WrapLabelConfigOf(textConf)); err != nil {
					return nil, err
				}
			}
			textElement.Set("text", wrapped)
			textElement.Set("textLines", float64(len(sanitize.LineBreakRegex.Split(wrapped, -1))))
			// textElement.width = calculateTextWidth(textElement.text, textConf);
			textElement.Set("width", textLimitWidth)
			// textElement.height = textElement.textLines * textConf.fontSize;
			height, err := utils.CalculateTextHeight(r.ctx, wrapped, dimConf)
			if err != nil {
				return nil, err
			}
			textElement.Set("height", height)
		} else {
			text, err := textOf(textElement)
			if err != nil {
				return nil, err
			}
			lines := sanitize.LineBreakRegex.Split(text, -1)
			textElement.Set("textLines", float64(len(lines)))
			lineHeight := 0.0
			textHeight := 0.0
			textWidth := 0.0
			textElement.Set("height", textHeight)
			textElement.Set("width", textWidth)
			for _, line := range lines {
				lineWidth, err := utils.CalculateTextWidth(r.ctx, line, dimConf)
				if err != nil {
					return nil, err
				}
				textWidth = js.Max(lineWidth, textWidth)
				textElement.Set("width", textWidth)
				if lineHeight, err = utils.CalculateTextHeight(r.ctx, line, dimConf); err != nil {
					return nil, err
				}
				textHeight = textHeight + lineHeight
				textElement.Set("height", textHeight)
			}
			// c4Shapes[textType].height = c4Shapes[textType].textLines * textConf.fontSize;
		}
	}
	return textElement, nil
}

// wrapOf is `bag.wrap && conf.wrap` as a condition.
func (r *renderer) wrapOf(bag *js.Obj[any]) bool {
	return js.Truthy(bag.Get("wrap")) && js.Truthy(r.rs.conf.Get("wrap"))
}

// intersectFunc is the type of the intersect property of a shape or a
// boundary.
type intersectFunc = func(point ru.Point) ru.Point

// drawBoundary is drawBoundary(diagram, boundary, bounds).
func (r *renderer) drawBoundary(diagram *dom.Node, boundary *js.Obj[any], b *bounds) error {
	// The bounds are seeded via `setData()` before a boundary is drawn.
	startx := b.data.startx
	starty := b.data.starty
	width := b.data.stopx - startx
	height := b.data.stopy - starty
	boundary.Set("x", startx)
	boundary.Set("y", starty)
	boundary.Set("width", width)
	boundary.Set("height", height)
	// A boundary can be a relationship endpoint, and its box is a plain rectangle.
	// intersect.rect measures from the centre, while the C4 grid stores the top-left corner.
	boundary.Set("intersect", intersectFunc(func(point ru.Point) ru.Point {
		x, y := num(boundary, "x"), num(boundary, "y")
		width, height := num(boundary, "width"), num(boundary, "height")
		return ru.IntersectRect(
			ru.Bounds{
				X:      x + float64(width/2),
				Y:      y + float64(height/2),
				Width:  width,
				Height: height,
			},
			point,
		)
	}))

	label, err := textElementOf(boundary, "label")
	if err != nil {
		return err
	}
	label.Set("y", r.confNum("c4ShapeMargin")-35)

	boundaryTextWrap := r.wrapOf(boundary)
	boundaryLabelConf := r.boundaryFont()
	boundaryLabelConf.Set("fontSize", js.Add(boundaryLabelConf.Get("fontSize"), 2.0))
	boundaryLabelConf.Set("fontWeight", "bold")
	textLimitWidth, err := r.calculateTextWidth(label.Get("text"), boundaryLabelConf)
	if err != nil {
		return err
	}
	if _, err := r.calcC4ShapeTextWH("label", boundary, boundaryTextWrap, boundaryLabelConf, textLimitWidth); err != nil {
		return err
	}

	r.svgDrawBoundary(diagram, boundary)
	return nil
}

// shapeHandlerFor is shapeHandlerFor of drawC4ShapeArray: the handler of
// the shape of the node, from the shapes of the unified renderer.
func shapeHandlerFor(node *ru.Node) (ru.ShapeHandler, error) {
	if node.Shape == "" || !ru.IsValidShape(node.Shape) {
		return nil, errors.New(`C4: no shape handler for "` + node.Shape + `"`)
	}
	return ru.LookupShape(node.Shape)
}

// drawC4ShapeArray is drawC4ShapeArray(currentBounds, diagram,
// c4ShapeArray, c4ShapeKeys). c4Shapes is the array of the shapes of the
// current boundary: upstream gets the array and its indexes.
func (r *renderer) drawC4ShapeArray(currentBounds *bounds, diagram *dom.Node, c4Shapes []*js.Obj[any]) error {
	mermaidConfig := r.ctx.Config
	look := "classic"
	if v := mermaidConfig.Get("look"); !js.IsNullish(v) {
		look = js.ToString(v)
	}
	renderOptions := &ru.ShapeRenderOptions{Config: mermaidConfig}
	// Namespace the shape DOM ids with the diagram id so two diagrams on one page don't
	// collide (the unified shapes use node.domId for the element id).
	diagramID := diagram.AttrOr("id", "")
	conf := r.rs.conf

	buildNode := func(c4Shape *js.Obj[any]) (*ru.Node, error) {
		node, err := buildC4Node(c4Shape, conf, r.confNum("c4ShapePadding"), look, r.confNum("width"))
		if err != nil {
			return nil, err
		}
		node.DomID = diagramID + "-" + node.ID
		return node, nil
	}

	// Pass 1 (measure): render each shape, read its self-sized dimensions onto the legacy
	// c4Shape, then discard the rendering. The shape is drawn again in pass 2 directly at its
	// final position, so its label (and any composited sub-element) lays out under the final
	// transform - drawing at the origin and translating afterwards leaves composited layers
	// (e.g. anything with opacity) painting at the stale origin.
	//
	// Upstream starts all shapes and waits for them. Each shape measures only
	// its own elements, so the port renders one shape after the other.
	for _, c4Shape := range c4Shapes {
		node, err := buildNode(c4Shape)
		if err != nil {
			return err
		}
		handler, err := shapeHandlerFor(node)
		if err != nil {
			return err
		}
		measured, err := handler(r.ctx, diagram, node, renderOptions)
		if err != nil {
			return err
		}
		c4Shape.Set("width", ru.NN(node.Width, r.confNum("width")))
		c4Shape.Set("height", ru.NN(node.Height, r.confNum("height")))
		c4Shape.Set("margin", r.confNum("c4ShapeMargin"))
		measured.Remove()
	}

	// Position with the legacy grid.
	for _, c4Shape := range c4Shapes {
		currentBounds.insert(c4Shape)
	}

	// Pass 2 (draw): render each shape into a group already translated to its grid position
	// (unified shapes are centred at the origin; legacy x/y is the top-left corner).
	for _, c4Shape := range c4Shapes {
		node, err := buildNode(c4Shape)
		if err != nil {
			return err
		}
		x, y := num(c4Shape, "x"), num(c4Shape, "y")
		width, height := num(c4Shape, "width"), num(c4Shape, "height")
		// Needed to properly calculate the intersection points.
		node.X = x + float64(width/2)
		node.Y = y + float64(height/2)
		// Appending before the await keeps the shapes' stacking order deterministic.
		positioned := diagram.
			Append("g").
			Attr(
				"transform",
				"translate("+js.Str(x+float64(width/2))+", "+js.Str(y+float64(height/2))+")",
			)
		handler, err := shapeHandlerFor(node)
		if err != nil {
			return err
		}
		if _, err := handler(r.ctx, positioned, node, renderOptions); err != nil {
			return err
		}
		if node.Intersect != nil {
			c4Shape.Set("intersect", intersectFunc(node.Intersect))
		} else {
			c4Shape.Set("intersect", nil)
		}
	}

	currentBounds.bumpLastMargin(r.confNum("c4ShapeMargin"))
	return nil
}

// getIntersectPoint gets the intersection of the line between the center
// point of a node and a point outside of the node.
func getIntersectPoint(fromNode *js.Obj[any], endPoint ru.Point) (ru.Point, error) {
	intersect := fromNode.Get("intersect")
	if !js.Truthy(intersect) {
		return ru.Point{}, errors.New(
			`C4 element "` + js.ToString(fromNode.Get("alias")) + `" has no intersect function. Please report this to https://github.com/mermaid-js/mermaid/issues`,
		)
	}
	fn, ok := intersect.(intersectFunc)
	if !ok {
		return ru.Point{}, errTypeError("fromNode.intersect is not a function")
	}
	return fn(endPoint), nil
}

// getIntersectPoints is getIntersectPoints(fromNode, endNode).
func getIntersectPoints(fromNode, endNode *js.Obj[any]) (startPoint, endPoint ru.Point, err error) {
	endIntersectPoint := ru.Point{X: 0, Y: 0}
	endIntersectPoint.X = num(endNode, "x") + float64(num(endNode, "width")/2)
	endIntersectPoint.Y = num(endNode, "y") + float64(num(endNode, "height")/2)
	if startPoint, err = getIntersectPoint(fromNode, endIntersectPoint); err != nil {
		return
	}

	endIntersectPoint.X = num(fromNode, "x") + float64(num(fromNode, "width")/2)
	endIntersectPoint.Y = num(fromNode, "y") + float64(num(fromNode, "height")/2)
	endPoint, err = getIntersectPoint(endNode, endIntersectPoint)
	return
}

// drawRels is drawRels(diagram, rels, getC4ShapeObj, diagObj, diagramId).
func (r *renderer) drawRels(diagram *dom.Node, rels []*js.Obj[any], diagramID string) error {
	diagramType := r.db.getC4Type()
	i := 0
	for _, rel := range rels {
		i = i + 1
		relTextWrap := r.wrapOf(rel)
		relConf := r.messageFont()
		label, err := textElementOf(rel, "label")
		if err != nil {
			return err
		}
		if diagramType == "C4Dynamic" {
			label.Set("text", js.Str(float64(i))+": "+js.ToString(label.Get("text")))
		}
		textLimitWidth, err := r.calculateTextWidth(label.Get("text"), relConf)
		if err != nil {
			return err
		}
		if _, err := r.calcC4ShapeTextWH("label", rel, relTextWrap, relConf, textLimitWidth); err != nil {
			return err
		}

		for _, field := range []string{"techn", "descr"} {
			// if (rel.techn && rel.techn.text !== '')
			value := rel.Get(field)
			if !js.Truthy(value) || js.Path(value, "text") == "" {
				continue
			}
			if textLimitWidth, err = r.calculateTextWidth(js.Path(value, "text"), relConf); err != nil {
				return err
			}
			if _, err := r.calcC4ShapeTextWH(field, rel, relTextWrap, relConf, textLimitWidth); err != nil {
				return err
			}
		}

		fromNode := r.db.getC4Shape(rel.Get("from"))
		endNode := r.db.getC4Shape(rel.Get("to"))
		if fromNode == nil || endNode == nil {
			return errors.New(
				`C4 rel "` + js.ToString(rel.Get("from")) + `" -> "` + js.ToString(rel.Get("to")) + `" references an unknown shape or boundary`,
			)
		}
		startPoint, endPoint, err := getIntersectPoints(fromNode, endNode)
		if err != nil {
			return err
		}
		rel.Set("startPoint", startPoint)
		rel.Set("endPoint", endPoint)
	}
	return r.svgDrawRels(diagram, rels, diagramID)
}

// drawInsideBoundary is drawInsideBoundary(diagram, parentBoundaryAlias,
// parentBounds, currentBoundaries, diagObj).
func (r *renderer) drawInsideBoundary(diagram *dom.Node, parentBounds *bounds, currentBoundaries []*js.Obj[any]) error {
	rs := r.rs
	currentBounds := r.newBounds()
	// Calculate the width limit of the boundary.  label/type 的长度，
	currentBounds.data.widthLimit = parentBounds.data.widthLimit / js.Min(rs.c4BoundaryInRow, float64(len(currentBoundaries)))
	// Math.min(
	//   conf.width * conf.c4ShapeInRow + conf.c4ShapeMargin * conf.c4ShapeInRow * 2,
	//   parentBounds.data.widthLimit / Math.min(conf.c4BoundaryInRow, currentBoundaries.length)
	// );
	for i, currentBoundary := range currentBoundaries {
		Y := 0.0
		image := js.NewObj[any]()
		image.Set("width", 0.0)
		image.Set("height", 0.0)
		image.Set("Y", 0.0)
		currentBoundary.Set("image", image)
		if js.Truthy(currentBoundary.Get("sprite")) {
			image.Set("width", 48.0)
			image.Set("height", 48.0)
			image.Set("Y", Y)
			Y = Y + 48
		}

		currentBoundaryTextWrap := r.wrapOf(currentBoundary)

		currentBoundaryLabelConf := r.boundaryFont()
		currentBoundaryLabelConf.Set("fontSize", js.Add(currentBoundaryLabelConf.Get("fontSize"), 2.0))
		currentBoundaryLabelConf.Set("fontWeight", "bold")
		label, err := r.calcC4ShapeTextWH(
			"label",
			currentBoundary,
			currentBoundaryTextWrap,
			currentBoundaryLabelConf,
			currentBounds.data.widthLimit,
		)
		if err != nil {
			return err
		}
		label.Set("Y", Y+8)
		Y = Y + 8 + num(label, "height")

		if typ := currentBoundary.Get("type"); js.Truthy(typ) && js.Path(typ, "text") != "" {
			typeObj, ok := typ.(*js.Obj[any])
			if !ok {
				// The assignment to a property of a primitive throws in strict mode.
				return errTypeError("Cannot create property 'text' on " + js.TypeOf(typ) + " '" + js.ToString(typ) + "'")
			}
			typeObj.Set("text", "["+js.ToString(typeObj.Get("text"))+"]")
			currentBoundaryTypeConf := r.boundaryFont()
			typeElement, err := r.calcC4ShapeTextWH(
				"type",
				currentBoundary,
				currentBoundaryTextWrap,
				currentBoundaryTypeConf,
				currentBounds.data.widthLimit,
			)
			if err != nil {
				return err
			}
			typeElement.Set("Y", Y+5)
			Y = Y + 5 + num(typeElement, "height")
		}

		if descr := currentBoundary.Get("descr"); js.Truthy(descr) && js.Path(descr, "text") != "" {
			currentBoundaryDescrConf := r.boundaryFont()
			currentBoundaryDescrConf.Set("fontSize", js.ToNumber(currentBoundaryDescrConf.Get("fontSize"))-2)
			descrElement, err := r.calcC4ShapeTextWH(
				"descr",
				currentBoundary,
				currentBoundaryTextWrap,
				currentBoundaryDescrConf,
				currentBounds.data.widthLimit,
			)
			if err != nil {
				return err
			}
			descrElement.Set("Y", Y+20)
			Y = Y + 20 + num(descrElement, "height")
		}

		if i == 0 || math.Mod(float64(i), rs.c4BoundaryInRow) == 0 {
			// Calculate the drawing start point of the currentBoundaries.
			x := parentBounds.data.startx + r.confNum("diagramMarginX")
			y := parentBounds.data.stopy + r.confNum("diagramMarginY") + Y

			currentBounds.setData(x, x, y, y)
		} else {
			// Calculate the drawing start point of the currentBoundaries.
			var x float64
			if currentBounds.data.stopx != currentBounds.data.startx {
				x = currentBounds.data.stopx + r.confNum("diagramMarginX")
			} else {
				x = currentBounds.data.startx
			}
			y := currentBounds.data.starty

			currentBounds.setData(x, x, y, y)
		}
		alias := currentBoundary.Get("alias")
		currentBounds.name = alias
		currentPersonOrSystemArray := r.db.getC4ShapeArrayOf(alias)

		if len(currentPersonOrSystemArray) > 0 {
			if err := r.drawC4ShapeArray(currentBounds, diagram, currentPersonOrSystemArray); err != nil {
				return err
			}
		}
		nextCurrentBoundaries := r.db.getBoundariesOf(alias)

		if len(nextCurrentBoundaries) > 0 {
			// draw boundary inside currentBoundary
			if err := r.drawInsideBoundary(diagram, currentBounds, nextCurrentBoundaries); err != nil {
				return err
			}
		}
		// draw boundary
		if alias != "global" {
			if err := r.drawBoundary(diagram, currentBoundary, currentBounds); err != nil {
				return err
			}
		}
		parentBounds.data.stopy = js.Max(
			currentBounds.data.stopy+r.confNum("c4ShapeMargin"),
			parentBounds.data.stopy,
		)
		parentBounds.data.stopx = js.Max(
			currentBounds.data.stopx+r.confNum("c4ShapeMargin"),
			parentBounds.data.stopx,
		)
		rs.globalBoundaryMaxX = js.Max(rs.globalBoundaryMaxX, parentBounds.data.stopx)
		rs.globalBoundaryMaxY = js.Max(rs.globalBoundaryMaxY, parentBounds.data.stopy)
	}
	return nil
}

// draw draws the C4 diagram in the svg element of the render.
func draw(ctx *render.Context, d render.DB, _ string) error {
	database := d.(*db)
	rs := &database.rs
	r := &renderer{ctx: ctx, db: database, rs: rs}

	// conf = getRequiredConfig('c4')
	rs.conf = r.dbConfig()
	if rs.conf == nil {
		return errTypeError("Cannot read properties of undefined (reading 'wrap')")
	}
	securityLevel, _ := ctx.Config.Get("securityLevel").(string)
	// Handle root and Document for when rendering in sandbox mode
	root, err := utils.GetDiagramRoot(ctx, securityLevel)
	if err != nil {
		return err
	}

	database.setWrap(rs.conf.Get("wrap"))

	rs.c4ShapeInRow = database.getC4ShapeInRow()
	rs.c4BoundaryInRow = database.getC4BoundaryInRow()

	id := ctx.ID
	diagram := root.Select(`[id="` + id + `"]`)

	insertComputerIcon(diagram, id)
	insertDatabaseIcon(diagram, id)
	insertClockIcon(diagram, id)

	screenBounds := r.newBounds()

	screenBounds.setData(
		r.confNum("diagramMarginX"),
		r.confNum("diagramMarginX"),
		r.confNum("diagramMarginY"),
		r.confNum("diagramMarginY"),
	)

	// screen.availWidth. The page of the reference has the size of the
	// viewport as its screen.
	screenBounds.data.widthLimit = ctx.Engine.ViewportWidth
	rs.globalBoundaryMaxX = r.confNum("diagramMarginX")
	rs.globalBoundaryMaxY = r.confNum("diagramMarginY")

	title := database.getTitle()
	currentBoundaries := database.getBoundariesOf("")
	// switch (c4type) {
	//   case 'C4Context':
	if err := r.drawInsideBoundary(diagram, screenBounds, currentBoundaries); err != nil {
		return err
	}
	//     break;
	// }

	// The arrow head definition is attached to the svg once
	insertArrowHead(diagram, id)
	insertArrowEnd(diagram, id)
	insertArrowCrossHead(diagram, id)
	insertArrowFilledHead(diagram, id)

	if err := r.drawRels(diagram, database.getRels(), id); err != nil {
		return err
	}

	screenBounds.data.stopx = rs.globalBoundaryMaxX
	screenBounds.data.stopy = rs.globalBoundaryMaxY

	box := screenBounds.data
	// `setData()` above seeded the start coordinates.
	boxStartx := box.startx
	boxStarty := box.starty

	// Make sure the height of the diagram supports long menus.
	boxHeight := rs.globalBoundaryMaxY - boxStarty

	height := boxHeight + float64(2*r.confNum("diagramMarginY"))

	// Make sure the width of the diagram supports wide menus.
	boxWidth := rs.globalBoundaryMaxX - boxStartx
	width := boxWidth + float64(2*r.confNum("diagramMarginX"))

	if title != "" {
		diagram.
			Append("text").
			Text(title).
			Attr("x", float64(boxWidth/2)-float64(4*r.confNum("diagramMarginX"))).
			Attr("y", boxStarty+r.confNum("diagramMarginY"))
	}

	render.ConfigureSvgSize(diagram, height, width, js.Truthy(rs.conf.Get("useMaxWidth")))

	extraVertForTitle := 0.0
	if title != "" {
		extraVertForTitle = 60
	}
	diagram.Attr(
		"viewBox",
		js.Str(boxStartx-r.confNum("diagramMarginX"))+
			" -"+
			js.Str(r.confNum("diagramMarginY")+extraVertForTitle)+
			" "+
			js.Str(width)+
			" "+
			js.Str(height+extraVertForTitle),
	)
	return nil
}
