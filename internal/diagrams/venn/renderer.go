// Ported from mermaid 12.2.0, diagrams/venn/vennRenderer.ts (MIT).

package venn

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/khroma"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	vennjs "github.com/bvolpato/mermaid-go-renderer/internal/venn"
)

// buildStyleByKey is buildStyleByKey(styleData).
func buildStyleByKey(styleData []*StyleData) *js.Map[string, *js.Obj[string]] {
	m := js.NewMap[string, *js.Obj[string]]()
	for _, entry := range styleData {
		key := strings.Join(entry.Targets, "|")
		existing, ok := m.Lookup(key)
		if ok {
			// Object.assign(existing, entry.styles)
			entry.Styles.Each(func(k, v string) { existing.Set(k, v) })
		} else {
			// { ...entry.styles }
			copied := js.NewObj[string]()
			entry.Styles.Each(func(k, v string) { copied.Set(k, v) })
			m.Set(key, copied)
		}
	}
	return m
}

// or is a || b.
func or(a, b any) any {
	if js.Truthy(a) {
		return a
	}
	return b
}

// nullish is a ?? b.
func nullish(a, b any) any {
	if js.IsNullish(a) {
		return b
	}
	return a
}

// styleProp is customStyle?.[key]: nil stands for undefined.
func styleProp(customStyle *js.Obj[string], key string) any {
	if customStyle == nil {
		return nil
	}
	if v, ok := customStyle.Lookup(key); ok {
		return v
	}
	return nil
}

// sortedSetsKey is stableSetsKey([...sets].sort()).
func sortedSetsKey(sets []string) string {
	sorted := append([]string{}, sets...)
	js.SortStrings(sorted)
	return stableSetsKey(sorted)
}

// configureSvgSize is configureSvgSize(svgElem, height, width, useMaxWidth)
// of setupGraphViewbox.js for a height and a width that are values of the
// config: a number, or another value that prints as JavaScript prints it.
func configureSvgSize(svgElem *dom.Node, height, width any, useMaxWidth bool) {
	h, hOK := height.(float64)
	w, wOK := width.(float64)
	if hOK && wOK {
		render.ConfigureSvgSize(svgElem, h, w, useMaxWidth)
		return
	}
	if useMaxWidth {
		svgElem.Attr("width", "100%")
		svgElem.Attr("style", "max-width: "+js.ToString(width)+"px;")
	} else {
		svgElem.Attr("height", height)
		svgElem.Attr("width", width)
	}
}

// drawVenn is draw.
func drawVenn(ctx *render.Context, db *DB) error {
	config := db.GetConfig()
	themeVariables := ctx.Config.Get("themeVariables")
	look := ctx.Config.Get("look")
	handDrawnSeed := ctx.Config.Get("handDrawnSeed")
	themeVariable := func(key string) any { return js.Path(themeVariables, key) }
	isHandDrawn := js.StrictEquals(look, "handDrawn")
	themeColors := []any{}
	for _, key := range []string{"venn1", "venn2", "venn3", "venn4", "venn5", "venn6", "venn7", "venn8"} {
		if v := themeVariable(key); js.Truthy(v) {
			themeColors = append(themeColors, v)
		}
	}
	title := db.GetDiagramTitle()
	sets := db.GetSubsetData()
	textNodes := db.GetTextData()
	styleByKey := buildStyleByKey(db.GetStyleData())

	// For layout purposes, ensure all pairwise subsets exist for any N-set union (N >= 3).
	// The venn.js layout algorithm requires explicit pairwise intersection entries
	// to correctly position overlapping circles for 3+ set unions. We add these synthetic
	// entries only for rendering. The stored data model (sets) is not mutated so all
	// tests that check getSubsetData() continue to pass unchanged.
	renderSets := ensurePairwiseSubsets(sets)

	// Configurable viewBox size with scale factor for proportional rendering
	svgWidth := nullish(config.Get("width"), 800.0)
	svgHeight := nullish(config.Get("height"), 450.0)
	const referenceWidth = 1600
	scale := js.ToNumber(svgWidth) / referenceWidth
	titleHeight := 0.0
	if title != "" {
		titleHeight = 48 * scale
	}

	defaultTextColor := nullish(themeVariable("primaryTextColor"), themeVariable("textColor"))

	// Prepare the target viewBox
	svg := ctx.SelectSvgElement()
	svg.Attr("viewBox", "0 0 "+js.ToString(svgWidth)+" "+js.ToString(svgHeight))

	if title != "" {
		svg.
			Append("text").
			Text(title).
			Attr("class", "venn-title").
			Attr("font-size", js.Str(32*scale)+"px").
			Attr("text-anchor", "middle").
			Attr("dominant-baseline", "middle").
			Attr("x", "50%").
			Attr("y", 32*scale).
			Style("fill", or(themeVariable("vennTitleTextColor"), themeVariable("titleColor")))
	}

	// The data of venn.js: each area keeps the index of its subset.
	areas := make([]*vennjs.Area, len(renderSets))
	for i, s := range renderSets {
		areas[i] = &vennjs.Area{Sets: s.Sets, Size: s.Size, Label: s.Label, HasLabel: s.HasLabel}
	}
	env := &vennjs.Env{Random: ctx.Random}
	chartHeight := js.ToNumber(svgHeight) - titleHeight

	// Get the original SVG output of Venn.js from a dummy root
	dummyD3root := ctx.Doc.CreateElement("div")
	chart, err := vennjs.Diagram(env, dummyD3root, areas, vennjs.DiagramOptions{Width: svgWidth, Height: chartHeight})
	if err != nil {
		return err
	}

	var roughSvg *ru.RoughSVG
	if isHandDrawn {
		roughSvg = ru.NewRoughSVG(ctx, dummyD3root.Select("svg"))
	}

	// Compute layout areas so we can position additional text nodes
	layoutAreas, err := vennjs.Layout(env, areas, vennjs.LayoutOptions{
		Width:   js.ToNumber(svgWidth),
		Height:  chartHeight,
		Padding: js.ToNumber(nullish(config.Get("padding"), 15.0)),
	})
	if err != nil {
		return err
	}

	// Build a lookup table from set key to layout area
	layoutByKey := js.NewMap[string, *vennjs.LayoutArea]()
	for _, area := range layoutAreas {
		key := sortedSetsKey(area.Data.Sets)
		layoutByKey.Set(key, area)
	}

	if len(textNodes) > 0 {
		if err := renderTextNodes(config, layoutByKey, dummyD3root, textNodes, scale, styleByKey); err != nil {
			return err
		}
	}

	// Style the set circles with theme colors
	themeDark, err := khroma.IsDark(or(themeVariable("background"), "#f4f4f4"))
	if err != nil {
		return err
	}
	for i, group := range dummyD3root.SelectAll(".venn-circle") {
		data := chart.Datum(group)
		if data == nil {
			continue
		}
		setsKey := sortedSetsKey(data.Sets)
		customStyle := styleByKey.Get(setsKey)
		// Empty for themes that define no `venn*`; the flat fallback is intended for those.
		// Spelled out because the old route to it was `i % 0` being NaN.
		var paletteColor any
		if len(themeColors) > 0 {
			paletteColor = themeColors[i%len(themeColors)]
		}
		baseColor := or(or(styleProp(customStyle, "fill"), paletteColor), themeVariable("primaryColor"))
		group.Classed("venn-set-"+js.Str(float64(i%8)), true)
		fillOpacity := nullish(styleProp(customStyle, "fill-opacity"), 0.1)
		strokeColor := or(styleProp(customStyle, "stroke"), baseColor)
		strokeWidthVal := or(styleProp(customStyle, "stroke-width"), js.Str(5*scale))

		drawn := false
		if isHandDrawn && roughSvg != nil {
			layoutArea := layoutByKey.Get(setsKey)
			if layoutArea != nil && len(layoutArea.Circles) > 0 {
				c := layoutArea.Circles[0]
				if c == nil {
					return &vennjs.Error{Name: "TypeError", Message: "Cannot read properties of undefined (reading 'x')"}
				}
				fill, err := khroma.Transparentize(baseColor, 0.7)
				if err != nil {
					return err
				}
				o := js.NewObj[any]()
				o.Set("roughness", 0.7)
				o.Set("seed", handDrawnSeed)
				o.Set("fill", fill)
				o.Set("fillStyle", "hachure")
				o.Set("fillWeight", 2.0)
				o.Set("hachureGap", 8.0)
				o.Set("hachureAngle", -41+float64(i*60))
				o.Set("stroke", strokeColor)
				o.Set("strokeWidth", js.ParseFloat(js.ToString(strokeWidthVal)))
				roughNode, err := roughSvg.Circle(c.X, c.Y, c.Radius*2, o)
				if err != nil {
					return err
				}
				group.Select("path").Remove()
				group.InsertBefore(roughNode, group.Select("text"))
			}
			drawn = true
		}
		if !drawn {
			group.
				Select("path").
				Style("fill", baseColor).
				Style("fill-opacity", fillOpacity).
				Style("stroke", strokeColor).
				Style("stroke-width", strokeWidthVal).
				Style("stroke-opacity", 0.95)
		}
		// Blend border color toward black (light theme) or white (dark theme) for readable text
		textColor := styleProp(customStyle, "color")
		if !js.Truthy(textColor) {
			var blended string
			if themeDark {
				blended, err = khroma.Lighten(baseColor, 30)
			} else {
				blended, err = khroma.Darken(baseColor, 30)
			}
			if err != nil {
				return err
			}
			textColor = blended
		}
		group.
			Select("text").
			Style("font-size", js.Str(48*scale)+"px").
			Style("fill", textColor)
	}

	// intersectionTextColor is
	// styleByKey.get(setsKey)?.color ?? themeVariables.vennSetTextColor ?? defaultTextColor.
	intersectionTextColor := func(customStyle *js.Obj[string]) any {
		return nullish(nullish(styleProp(customStyle, "color"), themeVariable("vennSetTextColor")), defaultTextColor)
	}

	// Style the union labels
	if isHandDrawn && roughSvg != nil {
		for _, group := range dummyD3root.SelectAll(".venn-intersection") {
			data := chart.Datum(group)
			if data == nil {
				continue
			}
			setsKey := sortedSetsKey(data.Sets)
			customStyle := styleByKey.Get(setsKey)
			customFill := styleProp(customStyle, "fill")

			if js.Truthy(customFill) {
				pathEl := group.Select("path")
				pathD, _ := pathEl.GetAttr("d")
				if pathD != "" {
					fill, err := khroma.Transparentize(customFill, 0.3)
					if err != nil {
						return err
					}
					o := js.NewObj[any]()
					o.Set("roughness", 0.7)
					o.Set("seed", handDrawnSeed)
					o.Set("fill", fill)
					o.Set("fillStyle", "cross-hatch")
					o.Set("fillWeight", 2.0)
					o.Set("hachureGap", 6.0)
					o.Set("hachureAngle", 60.0)
					o.Set("stroke", "none")
					roughNode, err := roughSvg.Path(pathD, o)
					if err != nil {
						return err
					}
					if existingPath := pathEl; existingPath != nil && existingPath.Parent != nil {
						existingPath.Parent.InsertBefore(roughNode, existingPath)
					}
					pathEl.Remove()
				}
			} else {
				group.Select("path").Style("fill-opacity", 0.0)
			}

			group.
				Select("text").
				Style("font-size", js.Str(48*scale)+"px").
				Style("fill", intersectionTextColor(customStyle))
		}
	} else {
		texts := dummyD3root.SelectAll(".venn-intersection text")
		for _, text := range texts {
			text.Style("font-size", js.Str(48*scale)+"px")
		}
		for _, text := range texts {
			data := chart.Datum(text)
			if data == nil {
				continue
			}
			text.Style("fill", intersectionTextColor(styleByKey.Get(sortedSetsKey(data.Sets))))
		}
		paths := dummyD3root.SelectAll(".venn-intersection path")
		for _, path := range paths {
			data := chart.Datum(path)
			if data == nil {
				continue
			}
			opacity := 0.0
			if js.Truthy(styleProp(styleByKey.Get(sortedSetsKey(data.Sets)), "fill")) {
				opacity = 1
			}
			path.Style("fill-opacity", opacity)
		}
		for _, path := range paths {
			data := chart.Datum(path)
			if data == nil {
				continue
			}
			path.Style("fill", nullish(styleProp(styleByKey.Get(sortedSetsKey(data.Sets)), "fill"), "transparent"))
		}
	}

	// Transfer the dummy SVG contents into the real SVG group
	vennGroup := svg.Append("g").Attr("transform", "translate(0, "+js.Str(titleHeight)+")")
	if dummySvg := dummyD3root.Select("svg"); dummySvg != nil {
		children := []*dom.Node{}
		for child := dummySvg.FirstChild; child != nil; child = child.Next {
			children = append(children, child)
		}
		for _, child := range children {
			vennGroup.AppendChild(child)
		}
	}
	configureSvgSize(svg, svgHeight, svgWidth, js.Truthy(nullish(config.Get("useMaxWidth"), true)))
	return nil
}

// stableSetsKey is stableSetsKey(setIds).
func stableSetsKey(setIDs []string) string {
	return strings.Join(setIDs, "|")
}

// renderTextNodes is renderTextNodes(config, layoutByKey, dummyD3root,
// textNodes, scale, styleByKey).
func renderTextNodes(
	config *js.Obj[any],
	layoutByKey *js.Map[string, *vennjs.LayoutArea],
	dummyD3root *dom.Node,
	textNodes []*TextData,
	scale float64,
	styleByKey *js.Map[string, *js.Obj[string]],
) error {
	useDebugLayout := js.Truthy(nullish(config.Get("useDebugLayout"), false))
	vennSvg := dummyD3root.Select("svg")
	textGroup := vennSvg.Append("g").Attr("class", "venn-text-nodes")

	// Group text nodes by the set key they belong to
	nodesByArea := js.NewMap[string, *[]*TextData]()
	for _, node := range textNodes {
		key := stableSetsKey(node.Sets)
		existing, ok := nodesByArea.Lookup(key)
		if ok {
			*existing = append(*existing, node)
		} else {
			nodesByArea.Set(key, &[]*TextData{node})
		}
	}

	// For each area, compute a text box and place nodes in a grid.
	for _, key := range nodesByArea.Keys() {
		nodes := *nodesByArea.Get(key)
		area := layoutByKey.Get(key)
		if area == nil || area.Text == nil {
			continue
		}
		// Calculate the center point and a safe inner radius for text.
		centerX := area.Text.X
		centerY := area.Text.Y
		minCircleRadius := math.Inf(1)
		innerRadiusRaw := math.Inf(1)
		for _, c := range area.Circles {
			if c == nil {
				return &vennjs.Error{Name: "TypeError", Message: "Cannot read properties of undefined (reading 'radius')"}
			}
		}
		for _, c := range area.Circles {
			minCircleRadius = js.Min(minCircleRadius, c.Radius)
		}
		for _, c := range area.Circles {
			innerRadiusRaw = js.Min(innerRadiusRaw, c.Radius-js.Hypot(centerX-c.X, centerY-c.Y))
		}
		innerRadius := 0.0
		if isFinite(innerRadiusRaw) {
			innerRadius = js.Max(0, innerRadiusRaw)
		}
		if innerRadius == 0 && isFinite(minCircleRadius) {
			innerRadius = minCircleRadius * 0.6
		}

		// Render text area container
		areaGroup := textGroup.
			Append("g").
			Attr("class", "venn-text-area").
			Attr("font-size", js.Str(40*scale)+"px")
		if useDebugLayout {
			areaGroup.
				Append("circle").
				Attr("class", "venn-text-debug-circle").
				Attr("cx", centerX).
				Attr("cy", centerY).
				Attr("r", innerRadius).
				Attr("fill", "none").
				Attr("stroke", "purple").
				Attr("stroke-width", 1.5*scale).
				Attr("stroke-dasharray", js.Str(6*scale)+" "+js.Str(4*scale))
		}

		// Compute a grid within the area for placing text nodes
		innerWidth := js.Max(80*scale, innerRadius*2*0.95)
		innerHeight := js.Max(60*scale, innerRadius*2*0.95)
		hasLabel := area.Data.HasLabel && js.UTF16Len(area.Data.Label) > 0
		labelOffsetBase := 0.0
		if hasLabel {
			labelOffsetBase = js.Min(32*scale, innerRadius*0.25)
		}
		labelOffset := labelOffsetBase
		if len(nodes) <= 2 {
			labelOffset = labelOffsetBase + float64(30*scale)
		} else {
			labelOffset = labelOffsetBase + 0
		}
		startX := centerX - float64(innerWidth/2)
		startY := centerY - float64(innerHeight/2) + labelOffset
		cols := js.Max(1, math.Ceil(math.Sqrt(float64(len(nodes)))))
		rows := js.Max(1, math.Ceil(float64(len(nodes))/cols))
		cellWidth := innerWidth / cols
		cellHeight := innerHeight / rows

		// Place each node into a grid cell
		for i, node := range nodes {
			col := math.Mod(float64(i), cols)
			row := math.Floor(float64(i) / cols)
			x := startX + float64(cellWidth*(col+0.5))
			y := startY + float64(cellHeight*(row+0.5))

			if useDebugLayout {
				areaGroup.
					Append("rect").
					Attr("class", "venn-text-debug-cell").
					Attr("x", startX+float64(cellWidth*col)).
					Attr("y", startY+float64(cellHeight*row)).
					Attr("width", cellWidth).
					Attr("height", cellHeight).
					Attr("fill", "none").
					Attr("stroke", "teal").
					Attr("stroke-width", 1*scale).
					Attr("stroke-dasharray", js.Str(4*scale)+" "+js.Str(3*scale))
			}

			boxWidth := float64(cellWidth * 0.9)
			boxHeight := float64(cellHeight * 0.9)

			// foreignObject lets us use HTML styling for auto-wrap
			container := areaGroup.
				Append("foreignObject").
				Attr("class", "venn-text-node-fo").
				Attr("width", boxWidth).
				Attr("height", boxHeight).
				Attr("x", x-float64(boxWidth/2)).
				Attr("y", y-float64(boxHeight/2)).
				Attr("overflow", "visible")

			textColor := styleProp(styleByKey.Get(node.ID), "color")
			label := node.ID
			if node.HasLabel {
				label = node.Label
			}
			text := container.
				Append("xhtml:span").
				Attr("class", "venn-text-node").
				Style("display", "flex").
				Style("width", "100%").
				Style("height", "100%").
				Style("white-space", "normal").
				Style("align-items", "center").
				Style("justify-content", "center").
				Style("text-align", "center").
				Style("overflow-wrap", "normal").
				Style("word-break", "normal").
				Text(label)

			if js.Truthy(textColor) {
				text.Style("color", textColor)
			}
		}
	}
	return nil
}

// isFinite is Number.isFinite for a number.
func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// ensurePairwiseSubsets is ensurePairwiseSubsets(subsets).
//
// Ensures that for every N-set union (N >= 3) in the subset list, all pairwise
// (2-set) intersections are present in the array passed to the layout engine.
//
// The venn.js layout algorithm needs explicit pairwise subset entries to
// correctly overlap circles when three or more sets share a region. Without them,
// `union A,B,C["Innovation"]` renders without the shared centre intersection.
//
// This function returns a *new* array. The original `subsets` from the DB is never
// mutated, so all parser/DB tests that assert on `getSubsetData()` continue to pass.
//
// The default size for pairwise intersections is chosen as 1/4 of the smaller
// individual set size. This ratio ensures the overlap is visually distinct but
// smaller than either contributing set, maintaining a balanced representation.
// When set sizes are unknown, a fallback of 2.5 (the parser's default for 2-set unions)
// is used.
func ensurePairwiseSubsets(subsets []*SubsetData) []*SubsetData {
	// Build a set of all existing subset keys (sorted, joined) for fast lookup.
	existingKeys := map[string]bool{}
	for _, s := range subsets {
		existingKeys[sortedSetsKey(s.Sets)] = true
	}

	// Pre-compute a map of individual set sizes for O(1) lookup
	individualSetSizes := map[string]float64{}
	for _, s := range subsets {
		if len(s.Sets) == 1 {
			individualSetSizes[s.Sets[0]] = s.Size
		}
	}

	synthetic := []*SubsetData{}

	for _, subset := range subsets {
		if len(subset.Sets) < 3 {
			continue
		}
		// For an N-set union, enumerate all pairs and add any that are missing.
		members := append([]string{}, subset.Sets...)
		js.SortStrings(members)
		for i := 0; i < len(members)-1; i++ {
			for j := i + 1; j < len(members); j++ {
				pair := []string{members[i], members[j]}
				key := strings.Join(pair, "|")
				if !existingKeys[key] {
					existingKeys[key] = true // avoid duplicates if multiple N-set unions share pairs
					// Use a size that visually represents a meaningful pairwise overlap.
					// We default to 1/4 of the smaller of the two individual set sizes,
					// falling back to 2.5 (the default for a 2-set union in the parser).
					// This ratio was chosen so the pairwise overlap is visible but smaller
					// than either contributing set, providing a balanced visual representation
					// when the actual size relationship between sets is not specified.
					sizeA, hasA := individualSetSizes[pair[0]]
					sizeB, hasB := individualSetSizes[pair[1]]
					pairSize := 2.5
					if hasA && hasB {
						pairSize = js.Min(sizeA, sizeB) / 4
					}
					synthetic = append(synthetic, &SubsetData{Sets: pair, Size: pairSize, Label: "", HasLabel: true})
				}
			}
		}
	}

	if len(synthetic) > 0 {
		return append(append([]*SubsetData{}, subsets...), synthetic...)
	}
	return subsets
}
