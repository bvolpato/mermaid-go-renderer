// Ported from mermaid 12.2.0, diagrams/cynefin/cynefinRenderer.ts (MIT).

package cynefin

import (
	"errors"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/theme"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

type domainMeta struct {
	model    string
	practice string
}

// domainMetas is DOMAIN_META.
var domainMetas = map[string]domainMeta{
	"complex":     {model: "Probe → Sense → Respond", practice: "Emergent Practices"},
	"complicated": {model: "Sense → Analyse → Respond", practice: "Good Practices"},
	"clear":       {model: "Sense → Categorise → Respond", practice: "Best Practices"},
	"chaotic":     {model: "Act → Sense → Respond", practice: "Novel Practices"},
	"confusion":   {model: "", practice: "Disorder"},
}

type domainLayout struct {
	cx, cy, x, y, w, h float64
}

// getDomainLayouts is getDomainLayouts(width, height).
func getDomainLayouts(width, height float64) map[string]domainLayout {
	hw := float64(width / 2)
	hh := float64(height / 2)
	return map[string]domainLayout{
		"complex":     {cx: hw / 2, cy: hh / 2, x: 0, y: 0, w: hw, h: hh},
		"complicated": {cx: hw + float64(hw/2), cy: hh / 2, x: hw, y: 0, w: hw, h: hh},
		"chaotic":     {cx: hw / 2, cy: hh + float64(hh/2), x: 0, y: hh, w: hw, h: hh},
		"clear":       {cx: hw + float64(hw/2), cy: hh + float64(hh/2), x: hw, y: hh, w: hw, h: hh},
		"confusion":   {cx: hw, cy: hh, x: hw * 0.7, y: hh * 0.7, w: hw * 0.6, h: hh * 0.6},
	}
}

// errNullThemeBlock is the TypeError of upstream for a cynefin theme block
// that is null: the renderer reads its properties.
var errNullThemeBlock = errors.New("Cannot read properties of null (reading 'complexBg')")

// getCynefinDomainColors is getCynefinDomainColors(): it resolves only the
// domain background colors from the cynefin theme block. The result reads a
// color by its key. A color that the block does not have is nil.
func getCynefinDomainColors(ctx *render.Context) (func(key string) any, error) {
	defaultThemeVariables, err := theme.GetThemeVariables("default", nil)
	if err != nil {
		return nil, err
	}
	currentConfig := ctx.Config
	themeVariables := utils.CleanAndMerge(defaultThemeVariables, currentConfig.Get("themeVariables"))
	block := themeVariables.Get("cynefin")
	if js.IsNullish(block) {
		return nil, errNullThemeBlock
	}
	colors := js.AsObj(block)
	return func(key string) any {
		if colors == nil {
			// A property of a string, a number or an array that is not a
			// color key is undefined.
			return nil
		}
		return colors.Get(key)
	}, nil
}

// maxConfusionItems is MAX_CONFUSION_ITEMS: the maximum of items inside the
// confusion ellipse before the overflow badge is shown.
const maxConfusionItems = 3

// attrValue is a value for selection.attr: null removes the attribute, as
// undefined does.
func attrValue(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// add is the + operator for a config value and a number: a string, an
// array or an object on the left gives a text.
func add(a any, b float64) any {
	switch a.(type) {
	case string, []any, *js.Obj[any]:
		return js.ToString(a) + js.Str(b)
	}
	return js.ToNumber(a) + b
}

// configureSvgSize is configureSvgSize(svgElem, height, width, useMaxWidth)
// for sizes that can be texts.
func configureSvgSize(svg *dom.Node, height, width any, useMaxWidth bool) {
	if useMaxWidth {
		svg.Attr("width", "100%")
		svg.Attr("style", "max-width: "+js.ToString(width)+"px;")
	} else {
		svg.Attr("height", height)
		svg.Attr("width", width)
	}
}

// upperFirst is name.charAt(0).toUpperCase() + name.slice(1) for a domain
// name.
func upperFirst(name string) string {
	return string(name[0]-'a'+'A') + name[1:]
}

// draw is draw(_text, id, _version, diagram).
func draw(ctx *render.Context, rdb render.DB, _ string) error {
	db := rdb.(*DB)
	id := ctx.ID
	domains := db.GetDomains()
	transitions := db.GetTransitions()
	title := db.GetDiagramTitle()
	accTitle := db.GetAccTitle()
	accDescription := db.GetAccDescription()
	config := db.GetConfig()
	domainColors, err := getCynefinDomainColors(ctx)
	if err != nil {
		return err
	}

	rawWidth := config.Get("width")
	rawHeight := config.Get("height")
	rawPadding := config.Get("padding")
	width := js.ToNumber(rawWidth)
	height := js.ToNumber(rawHeight)
	padding := js.ToNumber(rawPadding)
	showDomainDescriptions := js.Truthy(config.Get("showDomainDescriptions"))
	boundaryAmplitude := config.Get("boundaryAmplitude")
	totalWidth := add(rawWidth, padding*2)
	totalHeight := add(rawHeight, padding*2)

	domainBg := map[string]any{
		"complex":     attrValue(domainColors("complexBg")),
		"complicated": attrValue(domainColors("complicatedBg")),
		"clear":       attrValue(domainColors("clearBg")),
		"chaotic":     attrValue(domainColors("chaoticBg")),
		"confusion":   attrValue(domainColors("confusionBg")),
	}

	svg := ctx.SelectSvgElement()

	// config.useMaxWidth ?? true
	useMaxWidth := true
	if v := config.Get("useMaxWidth"); !js.IsNullish(v) {
		useMaxWidth = js.Truthy(v)
	}
	configureSvgSize(svg, totalHeight, totalWidth, useMaxWidth)
	svg.Attr("viewBox", "0 0 "+js.ToString(totalWidth)+" "+js.ToString(totalHeight))

	// Accessibility: use <title> element (W3C recommendation for SVG)
	if accTitle != "" {
		svg.Append("title").Text(accTitle)
	}
	if accDescription != "" {
		svg.Append("desc").Text(accDescription)
	}

	root := svg.Append("g").Attr("transform", "translate("+js.ToString(rawPadding)+", "+js.ToString(rawPadding)+")")

	layouts := getDomainLayouts(width, height)
	seed := resolveSeed(config.Get("seed"), id)

	// 1. Domain background rectangles
	bgGroup := root.Append("g").Attr("class", "cynefin-backgrounds")
	quadrantDomains := []string{"complex", "complicated", "chaotic", "clear"}
	for _, domainName := range quadrantDomains {
		layout := layouts[domainName]
		bgGroup.
			Append("rect").
			Attr("class", "cynefinDomain").
			Attr("x", layout.x).
			Attr("y", layout.y).
			Attr("width", layout.w).
			Attr("height", layout.h).
			Attr("fill", domainBg[domainName]).
			Attr("fill-opacity", 0.4).
			Attr("stroke", "none")
	}

	// 2. Wavy boundaries. The stroke comes from the CSS class cynefinBoundary.
	boundaryGroup := root.Append("g").Attr("class", "cynefin-boundaries")

	boundaryGroup.
		Append("path").
		Attr("class", "cynefinBoundary").
		Attr("d", generateFoldPath(width, height, seed, boundaryAmplitude)).
		Attr("fill", "none")

	boundaryGroup.
		Append("path").
		Attr("class", "cynefinBoundary").
		Attr("d", generateHorizontalBoundary(width, height, seed+100, boundaryAmplitude)).
		Attr("fill", "none")

	// 3. The cliff (thicker, between Clear and Chaotic). The stroke comes
	// from the CSS class cynefinCliff.
	boundaryGroup.
		Append("path").
		Attr("class", "cynefinCliff").
		Attr("d", generateCliffPath(width, height, js.ToString(rawHeight))).
		Attr("fill", "none")

	// 4. Confusion ellipse (center overlay). The stroke comes from the CSS
	// class cynefinConfusion.
	// Using width*0.15 and height*0.15 gives enough room for up to ~3 item badges
	confusionRx := float64(width * 0.15)
	confusionRy := float64(height * 0.15)
	root.
		Append("path").
		Attr("class", "cynefinConfusion").
		Attr("d", generateConfusionPath(float64(width/2), float64(height/2), confusionRx, confusionRy)).
		Attr("fill", domainBg["confusion"]).
		Attr("fill-opacity", 0.5)

	// 5. Domain name labels. The text styling comes from the CSS class
	// cynefinDomainLabel.
	labelGroup := root.Append("g").Attr("class", "cynefin-labels")
	for _, domainName := range quadrantDomains {
		layout := layouts[domainName]
		y := layout.cy
		if showDomainDescriptions {
			y = layout.cy - 30
		}
		labelGroup.
			Append("text").
			Attr("class", "cynefinDomainLabel").
			Attr("x", layout.cx).
			Attr("y", y).
			Attr("text-anchor", "middle").
			Attr("dominant-baseline", "middle").
			Text(upperFirst(domainName))
	}

	// Confusion label
	confusionY := height / 2
	if showDomainDescriptions {
		confusionY = float64(height/2) - 10
	}
	labelGroup.
		Append("text").
		Attr("class", "cynefinDomainLabel").
		Attr("x", width/2).
		Attr("y", confusionY).
		Attr("text-anchor", "middle").
		Attr("dominant-baseline", "middle").
		Text("Confusion")

	// 6. Domain description subtitles. The text styling comes from the CSS
	// class cynefinSubtitle.
	if showDomainDescriptions {
		subtitleGroup := root.Append("g").Attr("class", "cynefin-subtitles")
		for _, domainName := range quadrantDomains {
			layout := layouts[domainName]
			meta := domainMetas[domainName]

			subtitleGroup.
				Append("text").
				Attr("class", "cynefinSubtitle").
				Attr("x", layout.cx).
				Attr("y", layout.cy-10).
				Attr("text-anchor", "middle").
				Attr("dominant-baseline", "middle").
				Text(meta.model)

			subtitleGroup.
				Append("text").
				Attr("class", "cynefinSubtitle").
				Attr("x", layout.cx).
				Attr("y", layout.cy+5).
				Attr("text-anchor", "middle").
				Attr("dominant-baseline", "middle").
				Text(meta.practice)
		}

		// Confusion subtitle
		subtitleGroup.
			Append("text").
			Attr("class", "cynefinSubtitle").
			Attr("x", width/2).
			Attr("y", float64(height/2)+8).
			Attr("text-anchor", "middle").
			Attr("dominant-baseline", "middle").
			Text(domainMetas["confusion"].practice)
	}

	// 7. Items as text badges within each domain
	itemGroup := root.Append("g").Attr("class", "cynefin-items")
	const itemHeight = 26.0
	const itemPaddingX = 10.0

	// drawBadge draws one badge: a text, and a rect behind it that has the
	// measured width of the text.
	drawBadge := func(label, rectClass string, layout domainLayout, itemY float64, fill any, fillOpacity float64) {
		g := itemGroup.Append("g")

		// Append text first to measure actual rendered width
		textEl := g.
			Append("text").
			Attr("class", "cynefinItemText").
			Attr("x", 0).
			Attr("y", itemHeight/2).
			Attr("text-anchor", "middle").
			Attr("dominant-baseline", "central").
			Text(label)

		// Measure rendered text width; fall back to character-count heuristic if unavailable
		measuredWidth := float64(js.UTF16Len(label) * 7)
		if bbox := textEl.BBox(); bbox.Width > 0 {
			measuredWidth = bbox.Width
		}

		badgeWidth := measuredWidth + itemPaddingX*2
		itemX := layout.cx - float64(badgeWidth/2)

		g.Attr("transform", "translate("+js.Str(itemX)+", "+js.Str(itemY)+")")

		// Insert rect behind the text, sized to measured badge width
		g.Insert("rect", "text").
			Attr("class", rectClass).
			Attr("x", 0).
			Attr("y", 0).
			Attr("width", badgeWidth).
			Attr("height", itemHeight).
			Attr("rx", 4).
			Attr("ry", 4).
			Attr("fill", fill).
			Attr("fill-opacity", fillOpacity)

		// Centre text within badge
		textEl.Attr("x", badgeWidth/2).Attr("y", itemHeight/2)
	}

	allDomains := []string{"complex", "complicated", "chaotic", "clear", "confusion"}
	for _, domainName := range allDomains {
		domain := domains.Get(domainName)
		if domain == nil || len(domain.Items) == 0 {
			continue
		}

		layout := layouts[domainName]
		isConfusion := domainName == "confusion"

		// For confusion: cap items and center the block around the ellipse center.
		// For quadrant domains: start below the label/subtitle area.
		itemsToRender := domain.Items
		overflowCount := 0
		if isConfusion && len(domain.Items) > maxConfusionItems {
			overflowCount = len(domain.Items) - maxConfusionItems
			itemsToRender = domain.Items[:maxConfusionItems]
		}

		var startY float64
		if isConfusion {
			// Center the item block below the label within the ellipse
			labelOffset := 14.0
			if showDomainDescriptions {
				labelOffset = 22
			}
			startY = layout.cy + labelOffset
		} else if showDomainDescriptions {
			startY = layout.cy + 25
		} else {
			startY = layout.cy + 15
		}

		// Render item badges using getBBox() for accurate width measurement
		for idx, item := range itemsToRender {
			itemY := startY + float64(float64(idx)*(itemHeight+4))
			drawBadge(item.Label, "cynefinItem", layout, itemY, domainBg[domainName], 0.95)
		}

		// Overflow badge: "+N more" when confusion has more items than MAX_CONFUSION_ITEMS
		if overflowCount > 0 {
			overflowY := startY + float64(float64(len(itemsToRender))*(itemHeight+4))
			overflowLabel := "+" + js.Str(float64(overflowCount)) + " more"
			drawBadge(overflowLabel, "cynefinItemOverflow", layout, overflowY, domainBg[domainName], 0.6)
		}
	}

	// 8. Transition arrows between domain centers
	if len(transitions) > 0 {
		defs := svg.Select("defs")
		if defs == nil {
			defs = svg.Append("defs")
		}
		markerID := "cynefin-arrow-" + id

		defs.
			Append("marker").
			Attr("id", markerID).
			Attr("viewBox", "0 0 10 10").
			Attr("refX", 9).
			Attr("refY", 5).
			Attr("markerWidth", 6).
			Attr("markerHeight", 6).
			Attr("orient", "auto-start-reverse").
			Append("path").
			Attr("d", "M 0 0 L 10 5 L 0 10 z").
			Attr("class", "cynefinArrowHead")

		arrowGroup := root.Append("g").Attr("class", "cynefin-arrows")

		for _, transition := range transitions {
			fromLayout, hasFrom := layouts[transition.From]
			toLayout, hasTo := layouts[transition.To]
			if !hasFrom || !hasTo {
				continue
			}

			// Self-loops are filtered at the DB level; guard here handles any edge case
			if transition.From == transition.To {
				continue
			}

			// All math in root-local (unpadded) coordinates to match the <g> transform.
			x1 := fromLayout.cx
			y1 := fromLayout.cy
			x2 := toLayout.cx
			y2 := toLayout.cy

			// Quadratic bezier with perpendicular offset
			mx := float64((x1 + x2) / 2)
			my := float64((y1 + y2) / 2)
			dx := x2 - x1
			dy := y2 - y1
			length := math.Sqrt(float64(dx*dx) + float64(dy*dy))
			offsetAmount := length * 0.15
			nx := -dy / length
			ny := dx / length
			cpx := mx + float64(nx*offsetAmount)
			cpy := my + float64(ny*offsetAmount)

			arrowGroup.
				Append("path").
				Attr("class", "cynefinArrowLine").
				Attr("d", "M"+js.Str(x1)+","+js.Str(y1)+" Q"+js.Str(cpx)+","+js.Str(cpy)+" "+js.Str(x2)+","+js.Str(y2)).
				Attr("fill", "none").
				Attr("marker-end", "url(#"+markerID+")")

			// Optional label. The text styling comes from the CSS class
			// cynefinArrowLabel.
			if transition.Label != "" {
				arrowGroup.
					Append("text").
					Attr("class", "cynefinArrowLabel").
					Attr("x", cpx).
					Attr("y", cpy-6).
					Attr("text-anchor", "middle").
					Attr("dominant-baseline", "auto").
					Text(transition.Label)
			}
		}
	}

	// Title. The text styling comes from the CSS class cynefinTitle.
	if title != "" {
		root.
			Append("text").
			Attr("class", "cynefinTitle").
			Attr("x", width/2).
			Attr("y", -padding/2).
			Attr("text-anchor", "middle").
			Attr("dominant-baseline", "middle").
			Text(title)
	}
	return nil
}
