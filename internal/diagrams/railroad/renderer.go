// Ported from mermaid 12.2.0, diagrams/railroad/railroadRenderer.ts (MIT).

package railroad

import (
	"errors"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// pathBuilder is the class PathBuilder: it builds the text of an SVG path.
type pathBuilder struct {
	d strings.Builder
}

func (b *pathBuilder) moveTo(x, y float64) *pathBuilder {
	b.d.WriteString("M " + js.Str(x) + " " + js.Str(y) + " ")
	return b
}

func (b *pathBuilder) lineTo(x, y float64) *pathBuilder {
	b.d.WriteString("L " + js.Str(x) + " " + js.Str(y) + " ")
	return b
}

func (b *pathBuilder) horizontalTo(x float64) *pathBuilder {
	b.d.WriteString("H " + js.Str(x) + " ")
	return b
}

func (b *pathBuilder) verticalTo(y float64) *pathBuilder {
	b.d.WriteString("V " + js.Str(y) + " ")
	return b
}

func (b *pathBuilder) arcTo(rx, ry, rotation float64, largeArc, sweep bool, x, y float64) *pathBuilder {
	flag := func(on bool) string {
		if on {
			return "1"
		}
		return "0"
	}
	b.d.WriteString("A " + js.Str(rx) + " " + js.Str(ry) + " " + js.Str(rotation) + " " + flag(largeArc) + " " + flag(sweep) + " " + js.Str(x) + " " + js.Str(y) + " ")
	return b
}

func (b *pathBuilder) build() string {
	return js.TrimSpace(b.d.String())
}

// diagramDimensions is DiagramDimensions of railroadTypes.ts.
type diagramDimensions struct {
	width  float64
	height float64
	// up is the height above the center line.
	up float64
	// down is the height below the center line.
	down float64
}

// renderResult is RenderResult of railroadTypes.ts: the SVG element and the
// dimensions.
type renderResult struct {
	element    *dom.Node
	dimensions diagramDimensions
}

// textDimensions is the value of measureText.
type textDimensions struct {
	width, height float64
}

// railroadRenderer is the class RailroadRenderer.
type railroadRenderer struct {
	svg       *dom.Node
	config    styleOptions
	textCache map[string]textDimensions
}

// measureText is measureText(text): the dimensions of a text.
func (r *railroadRenderer) measureText(text string) textDimensions {
	if dimensions, ok := r.textCache[text]; ok {
		return dimensions
	}

	// Create temporary text element to measure
	tempText := r.svg.
		Append("text").
		Attr("font-family", r.config.fontFamily).
		Attr("font-size", r.config.fontSize).
		Text(text)

	bbox := tempText.BBox()
	dimensions := textDimensions{width: bbox.Width, height: bbox.Height}

	tempText.Remove()
	r.textCache[text] = dimensions

	return dimensions
}

// renderTerminal is renderTerminal(parent, value): a terminal symbol, a
// rounded rectangle.
func (r *railroadRenderer) renderTerminal(parent *dom.Node, value string) renderResult {
	textDim := r.measureText(value)
	width := textDim.width + float64(r.config.padding*2)
	height := textDim.height + float64(r.config.padding*2)

	group := parent.Append("g").Attr("class", "railroad-terminal")

	// Rounded rectangle
	group.
		Append("rect").
		Attr("x", 0).
		Attr("y", 0).
		Attr("width", width).
		Attr("height", height).
		Attr("rx", 10).
		Attr("ry", 10)

	// Text
	group.
		Append("text").
		Attr("x", width/2).
		Attr("y", height/2).
		Text(value)

	return renderResult{
		element: group,
		dimensions: diagramDimensions{
			width:  width,
			height: height,
			up:     height / 2,
			down:   height / 2,
		},
	}
}

// renderNonTerminal is renderNonTerminal(parent, name): a non-terminal
// symbol, a rectangle.
func (r *railroadRenderer) renderNonTerminal(parent *dom.Node, name string) renderResult {
	textDim := r.measureText(name)
	width := textDim.width + float64(r.config.padding*2)
	height := textDim.height + float64(r.config.padding*2)

	group := parent.Append("g").Attr("class", "railroad-nonterminal")

	// Rectangle
	group.Append("rect").Attr("x", 0).Attr("y", 0).Attr("width", width).Attr("height", height)

	// Text
	group.
		Append("text").
		Attr("x", width/2).
		Attr("y", height/2).
		Text(name)

	return renderResult{
		element: group,
		dimensions: diagramDimensions{
			width:  width,
			height: height,
			up:     height / 2,
			down:   height / 2,
		},
	}
}

// renderSequence is renderSequence(parent, elements): a horizontal
// concatenation.
func (r *railroadRenderer) renderSequence(parent *dom.Node, elements []*Node) (renderResult, error) {
	rendered := make([]renderResult, len(elements))
	for i, e := range elements {
		result, err := r.renderExpression(parent, e)
		if err != nil {
			return renderResult{}, err
		}
		rendered[i] = result
	}

	totalWidth := 0.0
	maxUp := 0.0
	maxDown := 0.0

	for _, item := range rendered {
		totalWidth += item.dimensions.width
		maxUp = js.Max(maxUp, item.dimensions.up)
		maxDown = js.Max(maxDown, item.dimensions.down)
	}

	totalWidth += float64(float64(len(rendered)-1) * r.config.horizontalSeparation)

	group := parent.Append("g").Attr("class", "railroad-sequence")
	x := 0.0

	for i, item := range rendered {
		y := maxUp - item.dimensions.up

		// Position element
		elem := group.AppendChild(item.element)
		elem.Attr("transform", "translate("+js.Str(x)+", "+js.Str(y)+")")

		// Draw connecting line to next element
		if i < len(rendered)-1 {
			lineX1 := x + item.dimensions.width
			lineX2 := lineX1 + r.config.horizontalSeparation
			lineY := maxUp

			group.
				Append("path").
				Attr("class", "railroad-line").
				Attr("d", new(pathBuilder).moveTo(lineX1, lineY).lineTo(lineX2, lineY).build())
		}

		x += item.dimensions.width + r.config.horizontalSeparation
	}

	return renderResult{
		element: group,
		dimensions: diagramDimensions{
			width:  totalWidth,
			height: maxUp + maxDown,
			up:     maxUp,
			down:   maxDown,
		},
	}, nil
}

// renderChoice is renderChoice(parent, alternatives): vertical
// alternatives.
func (r *railroadRenderer) renderChoice(parent *dom.Node, alternatives []*Node) (renderResult, error) {
	rendered := make([]renderResult, len(alternatives))
	for i, a := range alternatives {
		result, err := r.renderExpression(parent, a)
		if err != nil {
			return renderResult{}, err
		}
		rendered[i] = result
	}

	maxWidth := 0.0
	totalHeight := 0.0

	for _, item := range rendered {
		maxWidth = js.Max(maxWidth, item.dimensions.width)
		totalHeight += item.dimensions.height
	}

	totalHeight += float64(float64(len(rendered)-1) * r.config.verticalSeparation)

	arcRadius := r.config.arcRadius
	arcWidth := float64(arcRadius * 4)
	totalWidth := maxWidth + arcWidth

	group := parent.Append("g").Attr("class", "railroad-choice")
	y := 0.0
	centerY := float64(totalHeight / 2)

	// sign is isBelowCenter ? arcRadius : -arcRadius, or the opposite.
	sign := func(positive bool) float64 {
		if positive {
			return arcRadius
		}
		return -arcRadius
	}

	for _, item := range rendered {
		elemY := y
		elemCenterY := elemY + item.dimensions.up

		// Position element in the center
		elemX := float64(arcRadius*2) + float64((maxWidth-item.dimensions.width)/2)
		elem := group.AppendChild(item.element)
		elem.Attr("transform", "translate("+js.Str(elemX)+", "+js.Str(elemY)+")")

		// Left arc from center to this alternative
		leftPath := new(pathBuilder)
		isBelowCenter := elemCenterY > centerY
		if elemCenterY == centerY {
			leftPath.moveTo(0, centerY).lineTo(elemX, elemCenterY)
		} else {
			leftPath.
				moveTo(0, centerY).
				arcTo(
					arcRadius,
					arcRadius,
					0,
					false,
					isBelowCenter,
					arcRadius,
					centerY+sign(isBelowCenter),
				).
				lineTo(arcRadius, elemCenterY-sign(isBelowCenter)).
				arcTo(arcRadius, arcRadius, 0, false, !isBelowCenter, float64(arcRadius*2), elemCenterY).
				lineTo(elemX, elemCenterY)
		}

		group.Append("path").Attr("class", "railroad-line").Attr("d", leftPath.build())

		// Right arc from this alternative to center
		rightPath := new(pathBuilder)
		rightStart := elemX + item.dimensions.width
		rightLaneX := totalWidth - float64(arcRadius*2)
		if elemCenterY == centerY {
			rightPath.moveTo(rightStart, elemCenterY).lineTo(totalWidth, centerY)
		} else {
			rightPath.
				moveTo(rightStart, elemCenterY).
				lineTo(rightLaneX, elemCenterY).
				arcTo(
					arcRadius,
					arcRadius,
					0,
					false,
					!isBelowCenter,
					totalWidth-arcRadius,
					elemCenterY+sign(!isBelowCenter),
				).
				lineTo(totalWidth-arcRadius, centerY+sign(isBelowCenter)).
				arcTo(arcRadius, arcRadius, 0, false, isBelowCenter, totalWidth, centerY)
		}

		group.Append("path").Attr("class", "railroad-line").Attr("d", rightPath.build())

		y += item.dimensions.height + r.config.verticalSeparation
	}

	return renderResult{
		element: group,
		dimensions: diagramDimensions{
			width:  totalWidth,
			height: totalHeight,
			up:     centerY,
			down:   totalHeight - centerY,
		},
	}, nil
}

// renderOptional is renderOptional(parent, element): the element with a
// bypass path above.
func (r *railroadRenderer) renderOptional(parent *dom.Node, element *Node) (renderResult, error) {
	inner, err := r.renderExpression(parent, element)
	if err != nil {
		return renderResult{}, err
	}
	arcRadius := r.config.arcRadius
	arcHeight := float64(arcRadius * 2)
	totalWidth := inner.dimensions.width + float64(arcRadius*4)
	totalHeight := inner.dimensions.height + arcHeight

	group := parent.Append("g").Attr("class", "railroad-optional")

	// Position main element
	elemX := float64(arcRadius * 2)
	elemY := arcHeight
	elem := group.AppendChild(inner.element)
	elem.Attr("transform", "translate("+js.Str(elemX)+", "+js.Str(elemY)+")")

	centerY := elemY + inner.dimensions.up

	// Lower path (through element)
	lowerPath := new(pathBuilder).moveTo(0, centerY).lineTo(float64(arcRadius*2), centerY)
	group.Append("path").Attr("class", "railroad-line").Attr("d", lowerPath.build())

	lowerPath2 := new(pathBuilder).
		moveTo(elemX+inner.dimensions.width, centerY).
		lineTo(totalWidth, centerY)
	group.Append("path").Attr("class", "railroad-line").Attr("d", lowerPath2.build())

	// Upper bypass path
	bypassPath := new(pathBuilder).
		moveTo(0, centerY).
		arcTo(arcRadius, arcRadius, 0, false, false, arcRadius, centerY-arcRadius).
		lineTo(arcRadius, arcRadius).
		arcTo(arcRadius, arcRadius, 0, false, true, float64(arcRadius*2), 0).
		lineTo(totalWidth-float64(arcRadius*2), 0).
		arcTo(arcRadius, arcRadius, 0, false, true, totalWidth-arcRadius, arcRadius).
		lineTo(totalWidth-arcRadius, centerY-arcRadius).
		arcTo(arcRadius, arcRadius, 0, false, false, totalWidth, centerY)

	group.Append("path").Attr("class", "railroad-line").Attr("d", bypassPath.build())

	return renderResult{
		element: group,
		dimensions: diagramDimensions{
			width:  totalWidth,
			height: totalHeight,
			up:     centerY,
			down:   totalHeight - centerY,
		},
	}, nil
}

// renderRepetition is renderRepetition(parent, element, min): the element
// with a loop back path.
func (r *railroadRenderer) renderRepetition(parent *dom.Node, element *Node, min float64) (renderResult, error) {
	inner, err := r.renderExpression(parent, element)
	if err != nil {
		return renderResult{}, err
	}
	arcRadius := r.config.arcRadius
	arcHeight := float64(arcRadius * 2)
	totalWidth := inner.dimensions.width + float64(arcRadius*4)

	// For *, add bypass on top; for +, no bypass
	hasBypass := min == 0
	bypassHeight := 0.0
	if hasBypass {
		bypassHeight = arcHeight
	}
	totalHeight := inner.dimensions.height + arcHeight + bypassHeight

	group := parent.Append("g").Attr("class", "railroad-repetition")

	// Position main element
	elemX := float64(arcRadius * 2)
	elemY := 0.0
	if hasBypass {
		elemY = arcHeight
	}
	elem := group.AppendChild(inner.element)
	elem.Attr("transform", "translate("+js.Str(elemX)+", "+js.Str(elemY)+")")

	centerY := elemY + inner.dimensions.up

	// Forward path
	group.
		Append("path").
		Attr("class", "railroad-line").
		Attr(
			"d",
			new(pathBuilder).
				moveTo(0, centerY).
				lineTo(float64(arcRadius*2), centerY).
				build(),
		)

	group.
		Append("path").
		Attr("class", "railroad-line").
		Attr(
			"d",
			new(pathBuilder).
				moveTo(elemX+inner.dimensions.width, centerY).
				lineTo(totalWidth, centerY).
				build(),
		)

	// Loop back path (below)
	loopY := elemY + inner.dimensions.height + arcRadius
	loopPath := new(pathBuilder).
		moveTo(elemX+inner.dimensions.width, centerY).
		arcTo(
			arcRadius,
			arcRadius,
			0,
			false,
			true,
			elemX+inner.dimensions.width+arcRadius,
			centerY+arcRadius,
		).
		lineTo(elemX+inner.dimensions.width+arcRadius, loopY).
		arcTo(
			arcRadius,
			arcRadius,
			0,
			false,
			true,
			elemX+inner.dimensions.width,
			loopY+arcRadius,
		).
		lineTo(float64(arcRadius*2), loopY+arcRadius).
		arcTo(arcRadius, arcRadius, 0, false, true, arcRadius, loopY).
		lineTo(arcRadius, centerY+arcRadius).
		arcTo(arcRadius, arcRadius, 0, false, true, float64(arcRadius*2), centerY)

	group.Append("path").Attr("class", "railroad-line").Attr("d", loopPath.build())

	// Optional bypass path (for *)
	if hasBypass {
		bypassPath := new(pathBuilder).
			moveTo(0, centerY).
			arcTo(arcRadius, arcRadius, 0, false, false, arcRadius, centerY-arcRadius).
			lineTo(arcRadius, arcRadius).
			arcTo(arcRadius, arcRadius, 0, false, true, float64(arcRadius*2), 0).
			lineTo(totalWidth-float64(arcRadius*2), 0).
			arcTo(arcRadius, arcRadius, 0, false, true, totalWidth-arcRadius, arcRadius).
			lineTo(totalWidth-arcRadius, centerY-arcRadius).
			arcTo(arcRadius, arcRadius, 0, false, false, totalWidth, centerY)

		group.Append("path").Attr("class", "railroad-line").Attr("d", bypassPath.build())
	}

	return renderResult{
		element: group,
		dimensions: diagramDimensions{
			width:  totalWidth,
			height: totalHeight,
			up:     centerY,
			down:   totalHeight - centerY,
		},
	}, nil
}

// renderSpecial is renderSpecial(parent, text): a special sequence.
func (r *railroadRenderer) renderSpecial(parent *dom.Node, text string) renderResult {
	textDim := r.measureText("? " + text + " ?")
	width := textDim.width + float64(r.config.padding*2)
	height := textDim.height + float64(r.config.padding*2)

	group := parent.Append("g").Attr("class", "railroad-special")

	// Rectangle with dashed border
	group.Append("rect").Attr("x", 0).Attr("y", 0).Attr("width", width).Attr("height", height)

	// Text
	group.
		Append("text").
		Attr("x", width/2).
		Attr("y", height/2).
		Text("? " + text + " ?")

	return renderResult{
		element: group,
		dimensions: diagramDimensions{
			width:  width,
			height: height,
			up:     height / 2,
			down:   height / 2,
		},
	}
}

// renderExpression is renderExpression(parent, node): it renders an
// expression, with its parts.
func (r *railroadRenderer) renderExpression(parent *dom.Node, node *Node) (renderResult, error) {
	switch node.Type {
	case typeTerminal:
		return r.renderTerminal(parent, node.Value), nil

	case typeNonTerminal:
		return r.renderNonTerminal(parent, node.Name), nil

	case typeSequence:
		return r.renderSequence(parent, node.Elements)

	case typeChoice:
		return r.renderChoice(parent, node.Alternatives)

	case typeOptional:
		return r.renderOptional(parent, node.Element)

	case typeRepetition:
		return r.renderRepetition(parent, node.Element, node.Min)

	case typeSpecial:
		return r.renderSpecial(parent, node.Text), nil

	default:
		return renderResult{}, errors.New("Unknown node type: " + node.Type)
	}
}

// ruleSize is the value of renderRule.
type ruleSize struct {
	height, width float64
}

// renderRule is renderRule(rule, y): it renders a single rule.
func (r *railroadRenderer) renderRule(rule *Rule, y float64) (ruleSize, error) {
	group := r.svg.
		Append("g").
		Attr("class", "railroad-rule").
		Attr("transform", "translate(0, "+js.Str(y)+")")

	ruleName := rule.Name + " ="
	nameWidth := r.measureText(ruleName).width + 20
	definitionX := nameWidth + 20

	// Render definition
	defGroup := group.Append("g")
	result, err := r.renderExpression(defGroup, rule.Definition)
	if err != nil {
		return ruleSize{}, err
	}
	baselineY := js.Max(20, result.dimensions.up)
	definitionY := baselineY - result.dimensions.up
	defGroup.Attr("transform", "translate("+js.Str(definitionX)+", "+js.Str(definitionY)+")")

	nameGroup := group.Append("g").Attr("class", "railroad-rule-name-group")
	nameGroup.
		Append("text").
		Attr("class", "railroad-rule-name").
		Attr("x", 0).
		Attr("y", baselineY).
		Text(ruleName)

	// Start marker
	startMarker := group.Append("g").Attr("class", "railroad-start")
	startMarker.
		Append("circle").
		Attr("cx", nameWidth).
		Attr("cy", baselineY).
		Attr("r", r.config.markerRadius)

	// End marker
	endMarker := group.Append("g").Attr("class", "railroad-end")
	endMarker.
		Append("circle").
		Attr("cx", definitionX+result.dimensions.width+10).
		Attr("cy", baselineY).
		Attr("r", r.config.markerRadius)

	// Line from start to definition
	group.
		Append("path").
		Attr("class", "railroad-line").
		Attr(
			"d",
			new(pathBuilder).
				moveTo(nameWidth+r.config.markerRadius, baselineY).
				lineTo(definitionX, baselineY).
				build(),
		)

	// Line from definition to end
	group.
		Append("path").
		Attr("class", "railroad-line").
		Attr(
			"d",
			new(pathBuilder).
				moveTo(definitionX+result.dimensions.width, baselineY).
				lineTo(definitionX+result.dimensions.width+10-r.config.markerRadius, baselineY).
				build(),
		)

	return ruleSize{
		height: js.Max(40, definitionY+result.dimensions.height+float64(r.config.padding*2)),
		width:  definitionX + result.dimensions.width + 10 + r.config.markerRadius,
	}, nil
}

// diagramSize is the value of renderDiagram.
type diagramSize struct {
	width, height float64
}

// renderDiagram is renderDiagram(rules): it renders all rules.
func (r *railroadRenderer) renderDiagram(rules []*Rule) (diagramSize, error) {
	y := r.config.padding
	maxWidth := 0.0

	for _, rule := range rules {
		result, err := r.renderRule(rule, y)
		if err != nil {
			return diagramSize{}, err
		}
		y += result.height + r.config.verticalSeparation
		maxWidth = js.Max(maxWidth, result.width)
	}

	return diagramSize{
		width:  maxWidth + float64(r.config.padding*2),
		height: y + r.config.padding,
	}, nil
}

// configureRailroadSvgSize is configureRailroadSvgSize(svg, dimensions,
// useMaxWidth).
func configureRailroadSvgSize(svg *dom.Node, dimensions diagramSize, useMaxWidth bool) {
	render.ConfigureSvgSize(svg, dimensions.height, dimensions.width, useMaxWidth)
	svg.Attr("viewBox", "0 0 "+js.Str(dimensions.width)+" "+js.Str(dimensions.height))
}

// draw is draw(text, id, _version): the main draw function.
func draw(ctx *render.Context, db *DB) error {
	svg := ctx.SelectSvgElement()
	svg.Attr("class", "railroad-diagram")
	railroadConfig := ctx.Cfg("railroad")
	// const useMaxWidth = railroadConfig?.useMaxWidth ?? true;
	useMaxWidth := true
	if v := js.Path(railroadConfig, "useMaxWidth"); !js.IsNullish(v) {
		useMaxWidth = js.Truthy(v)
	}

	rules := db.GetRules()

	if len(rules) == 0 {
		configureRailroadSvgSize(svg, diagramSize{height: 100, width: 200}, useMaxWidth)
		return nil
	}

	config, err := buildRailroadStyleOptions(ctx)
	if err != nil {
		return err
	}
	renderer := &railroadRenderer{svg: svg, config: config, textCache: map[string]textDimensions{}}
	dimensions, err := renderer.renderDiagram(rules)
	if err != nil {
		return err
	}

	configureRailroadSvgSize(svg, dimensions, useMaxWidth)
	return nil
}
