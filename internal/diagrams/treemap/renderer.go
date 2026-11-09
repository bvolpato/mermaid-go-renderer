// Ported from mermaid 12.2.0, diagrams/treemap/renderer.ts (MIT).

package treemap

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/format"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/hierarchy"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/scale"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

const (
	defaultInnerPadding  = 10 // Default for inner padding between cells/sections
	sectionInnerPadding  = 10 // Default for inner padding between cells/sections
	sectionHeaderHeight  = 25
	widthTooLargeMessage = ": the width is too large"
)

// rePrecision is /\.\d+/.
var rePrecision = regexp.MustCompile(`\.[0-9]+`)

// valueFormatter formats a value. The error is an error that d3.format
// throws when the function runs.
type valueFormatter func(value float64) (string, error)

// d3Format is format(specifier)(value) with the specifier read at the time
// of the call, as the closures of upstream do. The state is the module
// variable prefixExponent of d3-format, which the formats of one page share:
// with the type "s", an infinite value takes the SI prefix of the value
// before it.
func d3Format(state *format.State, specifier string) valueFormatter {
	return func(value float64) (string, error) {
		f, err := state.Format(specifier)
		if err != nil {
			return "", err
		}
		return f(value), nil
	}
}

// newValueFormat makes the format function for the values from the config
// option valueFormat.
func newValueFormat(valueFormat any) valueFormatter {
	// One render is one page for d3-format.
	state := format.NewState()
	// Handle special format patterns
	var formatStr any = ","
	if js.Truthy(valueFormat) {
		formatStr = valueFormat
	}
	s, isString := formatStr.(string)
	if !isString {
		// formatStr.startsWith is not a function. Upstream catches the
		// TypeError and falls back to the default format.
		return d3Format(state, ",")
	}

	// Handle special cases that aren't directly supported by D3 format
	switch {
	case s == "$0,0":
		// Currency with thousands separator
		f := d3Format(state, ",")
		return func(value float64) (string, error) {
			text, err := f(value)
			return "$" + text, err
		}
	case strings.HasPrefix(s, "$") && strings.Contains(s, ","):
		// Other dollar formats with commas
		precisionStr := rePrecision.FindString(s)
		f := d3Format(state, ","+precisionStr)
		return func(value float64) (string, error) {
			text, err := f(value)
			return "$" + text, err
		}
	case strings.HasPrefix(s, "$"):
		// Simple dollar sign prefix
		restOfFormat := js.StringSliceFrom(s, 1)
		f := d3Format(state, restOfFormat)
		return func(value float64) (string, error) {
			text, err := f(value)
			return "$" + text, err
		}
	}
	// Standard D3 format. Upstream makes this function inside a try block:
	// an invalid specifier gives the default format. A width that is too
	// large makes d3 throw later, when the function runs.
	if _, err := format.Format(s); err != nil && !strings.HasSuffix(err.Error(), widthTooLargeMessage) {
		// Fallback to default format
		return d3Format(state, ",")
	}
	return d3Format(state, s)
}

// treemapNode is a node of the d3 hierarchy of the treemap.
type treemapNode = hierarchy.Node[*Node]

// hasChildren is d.children && d.children.length > 0 for a hierarchy node.
func hasChildren(d *treemapNode) bool {
	return len(d.Children) > 0
}

// styles2String is styles2String({ cssCompiledStyles }).
func styles2String(d *treemapNode) ru.NodeStyles {
	return ru.Styles2String(&ru.Node{CSSCompiledStyles: d.Data.CSSCompiledStyles})
}

// styleOf is selection.style(name) of d3 as a getter: the inline style, or
// the computed style when the inline style does not have the property.
func styleOf(n *dom.Node, property string) string {
	if v := n.StyleValue(property); v != "" {
		return v
	}
	return n.ComputedStyle(property)
}

// drawTreemap is draw(_text, id, _version, diagram): it draws the treemap
// diagram.
func drawTreemap(ctx *render.Context, treemapDb *DB) error {
	id := ctx.ID
	config := treemapDb.GetConfig()
	treemapInnerPadding := config.Get("padding")
	if js.IsNullish(treemapInnerPadding) {
		treemapInnerPadding = float64(defaultInnerPadding)
	}
	title := treemapDb.GetDiagramTitle()
	root := treemapDb.GetRoot()
	themeVariables := ctx.Cfg("themeVariables")
	themeVariable := func(name string) any {
		v, _ := js.GetProperty(themeVariables, name)
		return v
	}

	// Define dimensions
	titleHeight := 0.0
	if title != "" {
		titleHeight = 30
	}

	svg := ctx.SelectSvgElement()
	// Use config dimensions or defaults
	width := 960.0
	if nodeWidth := config.Get("nodeWidth"); js.Truthy(nodeWidth) {
		width = js.ToNumber(nodeWidth) * sectionInnerPadding
	}
	height := 500.0
	if nodeHeight := config.Get("nodeHeight"); js.Truthy(nodeHeight) {
		height = js.ToNumber(nodeHeight) * sectionInnerPadding
	}

	svgWidth := width
	svgHeight := height + titleHeight

	// Set the SVG size
	svg.Attr("viewBox", "0 0 "+js.Str(svgWidth)+" "+js.Str(svgHeight))
	render.ConfigureSvgSize(svg, svgHeight, svgWidth, js.Truthy(config.Get("useMaxWidth")))

	// Format for displaying values
	valueFormat := newValueFormat(config.Get("valueFormat"))

	// Create color scale
	colorScale := scale.NewOrdinal[string, any]().Range(
		"transparent",
		themeVariable("cScale0"),
		themeVariable("cScale1"),
		themeVariable("cScale2"),
		themeVariable("cScale3"),
		themeVariable("cScale4"),
		themeVariable("cScale5"),
		themeVariable("cScale6"),
		themeVariable("cScale7"),
		themeVariable("cScale8"),
		themeVariable("cScale9"),
		themeVariable("cScale10"),
		themeVariable("cScale11"),
	)
	colorScalePeer := scale.NewOrdinal[string, any]().Range(
		"transparent",
		themeVariable("cScalePeer0"),
		themeVariable("cScalePeer1"),
		themeVariable("cScalePeer2"),
		themeVariable("cScalePeer3"),
		themeVariable("cScalePeer4"),
		themeVariable("cScalePeer5"),
		themeVariable("cScalePeer6"),
		themeVariable("cScalePeer7"),
		themeVariable("cScalePeer8"),
		themeVariable("cScalePeer9"),
		themeVariable("cScalePeer10"),
		themeVariable("cScalePeer11"),
	)
	colorScaleLabel := scale.NewOrdinal[string, any]().Range(
		themeVariable("cScaleLabel0"),
		themeVariable("cScaleLabel1"),
		themeVariable("cScaleLabel2"),
		themeVariable("cScaleLabel3"),
		themeVariable("cScaleLabel4"),
		themeVariable("cScaleLabel5"),
		themeVariable("cScaleLabel6"),
		themeVariable("cScaleLabel7"),
		themeVariable("cScaleLabel8"),
		themeVariable("cScaleLabel9"),
		themeVariable("cScaleLabel10"),
		themeVariable("cScaleLabel11"),
	)

	// Draw the title if it exists
	if title != "" {
		svg.Append("text").
			Attr("x", svgWidth/2).
			Attr("y", titleHeight/2).
			Attr("class", "treemapTitle").
			Attr("text-anchor", "middle").
			Attr("dominant-baseline", "middle").
			Text(title)
	}

	// Create a main container for the treemap, translated below the title
	g := svg.Append("g").
		Attr("transform", "translate(0, "+js.Str(titleHeight)+")").
		Attr("class", "treemapContainer")

	// Create the hierarchical structure
	hierarchyRoot := hierarchy.New(root, func(d *Node) []*Node { return d.Children }).
		Sum(func(d *Node) float64 {
			// d.value ?? 0
			if !d.HasValue {
				return 0
			}
			return d.Value
		}).
		Sort(func(a, b *treemapNode) float64 { return b.Value - a.Value })

	// Create treemap layout
	branchPadding := func(padding float64) func(d *treemapNode) float64 {
		return func(d *treemapNode) float64 {
			if hasChildren(d) {
				return padding
			}
			return 0
		}
	}
	treemapLayout := hierarchy.NewTreemap[*Node]().
		Size(width, height).
		PaddingTopFunc(branchPadding(sectionHeaderHeight + sectionInnerPadding)).
		PaddingInner(js.ToNumber(treemapInnerPadding)).
		PaddingLeftFunc(branchPadding(sectionInnerPadding)).
		PaddingRightFunc(branchPadding(sectionInnerPadding)).
		PaddingBottomFunc(branchPadding(sectionInnerPadding)).
		Round(true)

	// Apply the treemap layout to the hierarchy
	treemapData := treemapLayout.Layout(hierarchyRoot)

	showValues := !js.StrictEquals(config.Get("showValues"), false)

	// Draw section nodes (branches - nodes with children)
	var branchNodes []*treemapNode
	for _, d := range treemapData.Descendants() {
		if hasChildren(d) {
			branchNodes = append(branchNodes, d)
		}
	}
	sections := make([]*dom.Node, len(branchNodes))
	for i, d := range branchNodes {
		sections[i] = g.Append("g").
			Attr("class", "treemapSection").
			Attr("transform", "translate("+js.Str(d.X0)+","+js.Str(d.Y0)+")")
	}

	// Add section header background
	for i, d := range branchNodes {
		style := ""
		// Hide the label for the root section
		if d.Depth == 0 {
			style = "display: none;"
		}
		sections[i].Append("rect").
			Attr("width", d.X1-d.X0).
			Attr("height", sectionHeaderHeight).
			Attr("class", "treemapSectionHeader").
			Attr("fill", "none").
			Attr("fill-opacity", 0.6).
			Attr("stroke-width", 0.6).
			Attr("style", style)
	}

	// Add clip paths for section headers to prevent text overflow
	for i, d := range branchNodes {
		sections[i].Append("clipPath").
			Attr("id", "clip-section-"+id+"-"+js.Str(float64(i))).
			Append("rect").
			Attr("width", js.Max(0, d.X1-d.X0-12)). // 6px padding on each side
			Attr("height", sectionHeaderHeight)
	}

	for i, d := range branchNodes {
		fill := colorScale.Scale(d.Data.Name)
		stroke := colorScalePeer.Scale(d.Data.Name)
		var style string
		if d.Depth == 0 {
			// Hide the label for the root section
			style = "display: none;"
		} else {
			styles := styles2String(d)
			style = styles.NodeStyles + ";" + strings.Join(styles.BorderStyles, ";")
		}
		sections[i].Append("rect").
			Attr("width", d.X1-d.X0).
			Attr("height", d.Y1-d.Y0).
			Attr("class", "treemapSection section"+js.Str(float64(i))).
			Attr("fill", fill).
			Attr("fill-opacity", 0.6).
			Attr("stroke", stroke).
			Attr("stroke-width", 2.0).
			Attr("stroke-opacity", 0.4).
			Attr("style", style)
	}

	// Add section labels
	sectionLabels := make([]*dom.Node, len(branchNodes))
	for i, d := range branchNodes {
		text := ""
		if d.Depth != 0 {
			// Skip label for root section
			text = d.Data.Name
		}
		var style string
		if d.Depth == 0 {
			// Hide the label for the root section
			style = "display: none;"
		} else {
			labelStyles := "dominant-baseline: middle; font-size: 12px; fill:" +
				js.ToString(colorScaleLabel.Scale(d.Data.Name)) +
				"; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;"
			styles := styles2String(d)
			style = labelStyles + strings.Replace(styles.LabelStyles, "color:", "fill:", 1)
		}
		sectionLabels[i] = sections[i].Append("text").
			Attr("class", "treemapSectionLabel").
			Attr("x", 6). // Keep original left padding
			Attr("y", sectionHeaderHeight/2.0).
			Attr("dominant-baseline", "middle").
			Text(text).
			Attr("font-weight", "bold").
			Attr("clip-path", "url(#clip-section-"+id+"-"+js.Str(float64(i))+")"). // Apply clip-path to prevent overflow
			Attr("style", style)
	}
	for i, d := range branchNodes {
		// Skip processing for root section
		if d.Depth == 0 {
			continue
		}
		self := sectionLabels[i]
		originalText := d.Data.Name
		self.Text(originalText)
		totalHeaderWidth := d.X1 - d.X0
		const labelXPosition = 6
		var spaceForTextContent float64
		if showValues && js.Truthy(d.Value) {
			valueEndsAtXRelative := totalHeaderWidth - 10
			const estimatedValueTextActualWidth = 30
			const gapBetweenLabelAndValue = 10
			labelMustEndBeforeX := valueEndsAtXRelative - estimatedValueTextActualWidth - gapBetweenLabelAndValue
			spaceForTextContent = labelMustEndBeforeX - labelXPosition
		} else {
			const labelOwnRightPadding = 6
			spaceForTextContent = totalHeaderWidth - labelXPosition - labelOwnRightPadding
		}
		const minimumWidthToDisplay = 15
		actualAvailableWidth := js.Max(minimumWidthToDisplay, spaceForTextContent)
		textNode := self
		currentTextContentLength := textNode.ComputedTextLength()
		if currentTextContentLength > actualAvailableWidth {
			const ellipsis = "..."
			// The lengths count UTF-16 code units, as in JavaScript.
			currentTruncatedLength := js.UTF16Len(originalText)
			for currentTruncatedLength > 0 {
				currentTruncatedText := js.StringSlice(originalText, 0, currentTruncatedLength-1)
				currentTruncatedLength = js.UTF16Len(currentTruncatedText)
				if currentTruncatedLength == 0 {
					self.Text(ellipsis)
					if textNode.ComputedTextLength() > actualAvailableWidth {
						self.Text("")
					}
					break
				}
				self.Text(currentTruncatedText + ellipsis)
				if textNode.ComputedTextLength() <= actualAvailableWidth {
					break
				}
			}
		}
	}

	// Add section values if enabled
	if showValues {
		for i, d := range branchNodes {
			text := ""
			if js.Truthy(d.Value) {
				var err error
				if text, err = valueFormat(d.Value); err != nil {
					return err
				}
			}
			var style string
			if d.Depth == 0 {
				// Hide the value for the root section
				style = "display: none;"
			} else {
				labelStyles := "text-anchor: end; dominant-baseline: middle; font-size: 10px; fill:" +
					js.ToString(colorScaleLabel.Scale(d.Data.Name)) +
					"; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;"
				styles := styles2String(d)
				style = labelStyles + strings.Replace(styles.LabelStyles, "color:", "fill:", 1)
			}
			sections[i].Append("text").
				Attr("class", "treemapSectionValue").
				Attr("x", d.X1-d.X0-10).
				Attr("y", sectionHeaderHeight/2.0).
				Attr("text-anchor", "end").
				Attr("dominant-baseline", "middle").
				Text(text).
				Attr("font-style", "italic").
				Attr("style", style)
		}
	}

	// Draw the leaf nodes
	leafNodes := treemapData.Leaves()

	isComplexTreemap := len(leafNodes) > 20

	pick := func(complex, simple float64) float64 {
		if isComplexTreemap {
			return complex
		}
		return simple
	}
	baseLabelFontSize := pick(16, 38)
	baseValueFontSize := pick(14, 28)
	minLabelFontSize := pick(4, 8)
	minValueFontSize := pick(4, 6)
	labelPadding := pick(2, 4)
	minDisplayThreshold := pick(8, 10)
	spacingBetweenLabelAndValue := pick(1, 2)

	cell := make([]*dom.Node, len(leafNodes))
	for i, d := range leafNodes {
		class := "treemapNode treemapLeafGroup leaf" + js.Str(float64(i))
		if d.Data.ClassSelector != "" {
			class += " " + d.Data.ClassSelector
		}
		class += "x"
		cell[i] = g.Append("g").
			Attr("class", class).
			Attr("transform", "translate("+js.Str(d.X0)+","+js.Str(d.Y0)+")")
	}

	// leafColor is the color of a leaf. Leaves inherit color from their
	// immediate parent section's name. If a leaf is the root itself (no
	// parent), it uses its own name.
	leafColor := func(d *treemapNode) any {
		if d.Parent != nil {
			return colorScale.Scale(d.Parent.Data.Name)
		}
		return colorScale.Scale(d.Data.Name)
	}

	// Add rectangle for each leaf node
	for i, d := range leafNodes {
		cell[i].Append("rect").
			Attr("width", d.X1-d.X0).
			Attr("height", d.Y1-d.Y0).
			Attr("class", "treemapLeaf").
			Attr("fill", leafColor(d)).
			Attr("style", styles2String(d).NodeStyles).
			Attr("fill-opacity", 0.3).
			Attr("stroke", leafColor(d)).
			Attr("stroke-width", 3.0)
	}

	// Add clip paths to prevent text from extending outside nodes
	for i, d := range leafNodes {
		cell[i].Append("clipPath").
			Attr("id", "clip-"+id+"-"+js.Str(float64(i))).
			Append("rect").
			Attr("width", js.Max(0, d.X1-d.X0-4)).
			Attr("height", js.Max(0, d.Y1-d.Y0-4))
	}

	// Add node labels with clipping
	leafLabels := make([]*dom.Node, len(leafNodes))
	for i, d := range leafNodes {
		labelStyles := "text-anchor: middle; dominant-baseline: middle; font-size: " + js.Str(baseLabelFontSize) + "px;fill:" +
			js.ToString(colorScaleLabel.Scale(d.Data.Name)) +
			";"
		styles := styles2String(d)
		leafLabels[i] = cell[i].Append("text").
			Attr("class", "treemapLabel").
			Attr("x", (d.X1-d.X0)/2).
			Attr("y", (d.Y1-d.Y0)/2).
			Attr("style", labelStyles+strings.Replace(styles.LabelStyles, "color:", "fill:", 1)).
			Attr("clip-path", "url(#clip-"+id+"-"+js.Str(float64(i))+")").
			Text(d.Data.Name)
	}

	// valueFontSize is the font size of a value for the font size of its
	// label.
	const valueScaleFactor = 0.6 // Value font size as a factor of label font size
	valueFontSize := func(labelFontSize float64) float64 {
		return js.Max(minValueFontSize, js.Min(baseValueFontSize, js.Round(labelFontSize*valueScaleFactor)))
	}

	for i, d := range leafNodes {
		self := leafLabels[i]
		nodeWidth := d.X1 - d.X0
		nodeHeight := d.Y1 - d.Y0
		textNode := self

		availableWidth := nodeWidth - float64(2*labelPadding)
		availableHeight := nodeHeight - float64(2*labelPadding)

		if availableWidth < minDisplayThreshold || availableHeight < minDisplayThreshold {
			self.Style("display", "none")
			continue
		}

		currentLabelFontSize := js.ParseInt(styleOf(self, "font-size"), 10)

		// 1. Adjust label font size to fit width
		for textNode.ComputedTextLength() > availableWidth && currentLabelFontSize > minLabelFontSize {
			currentLabelFontSize--
			self.Style("font-size", js.Str(currentLabelFontSize)+"px")
		}

		// 2. Adjust both label and prospective value font size to fit combined height
		prospectiveValueFontSize := valueFontSize(currentLabelFontSize)
		combinedHeight := currentLabelFontSize + spacingBetweenLabelAndValue + prospectiveValueFontSize

		for combinedHeight > availableHeight && currentLabelFontSize > minLabelFontSize {
			currentLabelFontSize--
			prospectiveValueFontSize = valueFontSize(currentLabelFontSize)
			if prospectiveValueFontSize < minValueFontSize && currentLabelFontSize == minLabelFontSize {
				break
			} // Avoid shrinking label if value is already at min
			self.Style("font-size", js.Str(currentLabelFontSize)+"px")
			combinedHeight = currentLabelFontSize + spacingBetweenLabelAndValue + prospectiveValueFontSize
		}

		// Update label font size based on height adjustment
		self.Style("font-size", js.Str(currentLabelFontSize)+"px")

		// 3. Final visibility check for the label
		if isComplexTreemap {
			if currentLabelFontSize < minLabelFontSize || availableHeight < minLabelFontSize {
				self.Style("display", "none")
			}
		} else {
			if textNode.ComputedTextLength() > availableWidth ||
				currentLabelFontSize < minLabelFontSize ||
				availableHeight < currentLabelFontSize {
				self.Style("display", "none")
			}
		}
	}

	// Add node values with clipping
	if showValues {
		leafValues := make([]*dom.Node, len(leafNodes))
		for i, d := range leafNodes {
			labelStyles := "text-anchor: middle; dominant-baseline: hanging; font-size: " + js.Str(baseValueFontSize) + "px;fill:" +
				js.ToString(colorScaleLabel.Scale(d.Data.Name)) +
				";"
			styles := styles2String(d)
			text := ""
			if js.Truthy(d.Value) {
				var err error
				if text, err = valueFormat(d.Value); err != nil {
					return err
				}
			}
			leafValues[i] = cell[i].Append("text").
				Attr("class", "treemapValue").
				Attr("x", (d.X1-d.X0)/2).
				// Y position calculated dynamically below based on final label metrics
				Attr("y", (d.Y1-d.Y0)/2). // Placeholder, will be overwritten
				Attr("style", labelStyles+strings.Replace(styles.LabelStyles, "color:", "fill:", 1)).
				Attr("clip-path", "url(#clip-"+id+"-"+js.Str(float64(i))+")").
				Text(text)
		}

		for i, d := range leafNodes {
			valueTextElement := leafValues[i]
			parentCellNode := valueTextElement.Parent

			if parentCellNode == nil {
				valueTextElement.Style("display", "none")
				continue
			}

			labelElement := parentCellNode.Select(".treemapLabel")

			if labelElement == nil || styleOf(labelElement, "display") == "none" {
				valueTextElement.Style("display", "none")
				continue
			}

			finalLabelFontSize := js.ParseFloat(styleOf(labelElement, "font-size"))

			actualValueFontSize := valueFontSize(finalLabelFontSize)
			valueTextElement.Style("font-size", js.Str(actualValueFontSize)+"px")

			labelCenterY := float64((d.Y1 - d.Y0) / 2)
			valueTopActualY := labelCenterY + float64(finalLabelFontSize/2) + spacingBetweenLabelAndValue
			valueTextElement.Attr("y", valueTopActualY)

			nodeWidth := d.X1 - d.X0
			nodeTotalHeight := d.Y1 - d.Y0
			const cellBottomPadding = 4
			maxValueBottomY := nodeTotalHeight - cellBottomPadding
			availableWidthForValue := nodeWidth - float64(2*labelPadding)

			if valueTextElement.ComputedTextLength() > availableWidthForValue ||
				valueTopActualY+actualValueFontSize > maxValueBottomY ||
				actualValueFontSize < minValueFontSize {
				valueTextElement.Style("display", "none")
			} else {
				valueTextElement.Style("display", nil)
			}
		}
	}
	diagramPadding := config.Get("diagramPadding")
	if js.IsNullish(diagramPadding) {
		diagramPadding = 8.0
	}
	render.SetupViewPortForSVG(svg, js.ToNumber(diagramPadding), "flowchart", js.Truthy(config.Get("useMaxWidth")))
	return nil
}
