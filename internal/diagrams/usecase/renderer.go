// Ported from mermaid 12.2.0, diagrams/usecase/usecaseRenderer.ts (MIT).

package usecase

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/markdown"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// usecaseMarkers is USECASE_MARKERS.
var usecaseMarkers = []string{
	"point",
	"circle",
	"cross",
	"extension",
}

var actorShapes = map[string]bool{
	"usecaseActor":        true,
	"usecaseActorHollow":  true,
	"usecaseActorAwesome": true,
	"usecaseActorIcon":    true,
}

var (
	reNotWordOrDash   = regexp.MustCompile(`[^\w-]+`)
	reOuterUnderscore = regexp.MustCompile(`^_+|_+$`)
)

// safeID is `value.replace(/[^\w-]+/g, '_').replace(/^_+|_+$/g, ”) || 'element'`.
func safeID(value string) string {
	safe := reOuterUnderscore.ReplaceAllString(reNotWordOrDash.ReplaceAllString(value, "_"), "")
	if safe == "" {
		return "element"
	}
	return safe
}

// usecaseDomID is usecaseDomId(diagramId, modelId).
func usecaseDomID(diagramID, modelID string) string {
	return "usecase-" + safeID(diagramID) + "-" + safeID(modelID)
}

// usecaseNodeDomID is usecaseNodeDomId(modelId).
func usecaseNodeDomID(modelID string) string {
	return "usecase-" + safeID(modelID)
}

// getAccessibleLabel is getAccessibleLabel(label, labelType).
func getAccessibleLabel(label string, labelType string) string {
	if labelType != "markdown" {
		return label
	}
	lines := markdown.MarkdownToLines(label, nil)
	joined := make([]string, len(lines))
	for i, line := range lines {
		words := make([]string, len(line))
		for j, word := range line {
			words[j] = word.Content
		}
		joined[i] = strings.Join(words, " ")
	}
	return strings.Join(joined, "\n")
}

// nodeLabelOrID is `node.label ?? node.id`.
func nodeLabelOrID(node *ru.Node) string {
	if node.HasLabel {
		return node.Label
	}
	return node.ID
}

// stringProp is an extra property of a node or an edge that is a string.
// The second result is false when the property is undefined.
func stringProp(extra map[string]any, key string) (string, bool) {
	v, ok := extra[key]
	if !ok || v == nil {
		return "", false
	}
	return js.ToString(v), true
}

// getUsecaseNodeAccessibleName is getUsecaseNodeAccessibleName(node). The
// label of the target of a note is a parameter: the caller of upstream
// replaces the property noteTargetLabel in a copy of the node.
func getUsecaseNodeAccessibleName(node *ru.Node, noteTargetLabel string, hasNoteTargetLabel bool) string {
	label := getAccessibleLabel(nodeLabelOrID(node), node.LabelType)
	if actorShapes[node.Shape] {
		variant := ""
		if actorType, ok := stringProp(node.Extra, "actorType"); ok && actorType != "" && actorType != "normal" {
			variant = actorType + " "
		}
		business := ""
		if isBusiness(node) {
			business = "business "
		}
		stereotype := ""
		if node.Stereotype != "" {
			stereotype = ", stereotype " + node.Stereotype
		}
		return business + variant + "actor " + label + stereotype
	}
	if node.Shape == "note" {
		// node.noteTargetLabel ?? node.noteTarget ?? ''
		target := noteTargetLabel
		if !hasNoteTargetLabel {
			target, _ = stringProp(node.Extra, "noteTarget")
		}
		return "Note for " + target + ": " + label
	}
	if node.Shape == "usecaseJsonTable" {
		rows := make([]string, 0)
		for _, row := range jsonRowsOf(node) {
			rows = append(rows, row.AccessibleKey+": "+row.Value)
		}
		joined := strings.Join(rows, "; ")
		if joined != "" {
			return label + ": " + joined
		}
		return label
	}
	stereotype := ""
	if node.Stereotype != "" {
		stereotype = ", stereotype " + node.Stereotype
	}
	business := ""
	if isBusiness(node) {
		business = "business "
	}
	return business + "use case " + label + stereotype
}

// getUsecaseBoundaryAccessibleName is getUsecaseBoundaryAccessibleName(boundary).
func getUsecaseBoundaryAccessibleName(boundary *ru.Node) string {
	boundaryType := "undefined"
	if v, ok := stringProp(boundary.Extra, "boundaryType"); ok {
		boundaryType = v
	}
	return boundaryType + " system boundary " + getAccessibleLabel(
		nodeLabelOrID(boundary),
		boundary.LabelType,
	)
}

// getUsecaseEdgeAccessibleName is getUsecaseEdgeAccessibleName(edge), with
// the source label and the target label of the copy of the edge that the
// caller of upstream makes.
func getUsecaseEdgeAccessibleName(edge *ru.Edge, sourceLabel, targetLabel string) string {
	relationshipType, _ := stringProp(edge.Extra, "relationshipType")
	if relationshipType == "note" {
		return ""
	}
	relation := relationshipType
	if relationshipType == "association" && edge.HasLabel && edge.Label != "" {
		relation = "association " + getAccessibleLabel(edge.Label, edge.LabelType)
	}
	return relation + " from " + sourceLabel + " to " + targetLabel
}

// escapePlainLabel is escapePlainLabel(label).
func escapePlainLabel(label string) string {
	label = strings.ReplaceAll(label, "&", "&amp;")
	label = strings.ReplaceAll(label, "<", "&lt;")
	return strings.ReplaceAll(label, ">", "&gt;")
}

var reMarkdownMarkers = regexp.MustCompile("([*\\[\\\\\\]_`])")

// escapeMarkdownMarkers is escapeMarkdownMarkers(label).
func escapeMarkdownMarkers(label string) string {
	return reMarkdownMarkers.ReplaceAllString(label, `\$1`)
}

// usecaseAccessibleNames is UsecaseAccessibleNames.
type usecaseAccessibleNames struct {
	nodes *js.Map[string, string]
	edges *js.Map[string, string]
}

// prepareUsecaseLayoutData is prepareUsecaseLayoutData(data, diagramId).
// The property hasFoldedStereotype of a node is in node.Extra.
func prepareUsecaseLayoutData(data *ru.LayoutData, diagramID string) *ru.LayoutData {
	data.DiagramID = diagramID
	data.Markers = append([]string{}, usecaseMarkers...)
	for _, node := range data.Nodes {
		node.DomID = usecaseNodeDomID(node.ID)
		if node.HasLabel && node.LabelType == "text" {
			node.Label = escapePlainLabel(node.Label)
		}
		if node.Stereotype != "" {
			node.Stereotype = escapePlainLabel(node.Stereotype)
		}
		if !node.IsGroup && node.Shape == "usecaseJsonTable" {
			if rows, ok := node.Get("jsonRows").([]UsecaseJsonRow); ok {
				escaped := make([]UsecaseJsonRow, len(rows))
				for i, row := range rows {
					escaped[i] = UsecaseJsonRow{
						Key:           escapePlainLabel(row.Key),
						AccessibleKey: row.AccessibleKey,
						Value:         escapePlainLabel(row.Value),
					}
				}
				node.Set("jsonRows", escaped)
			}
		}
		if !node.IsGroup &&
			(node.Shape == "usecaseEllipse" || node.Shape == "rect") &&
			node.Stereotype != "" {
			label := escapePlainLabel(node.ID)
			if node.HasLabel {
				label = node.Label
			}
			if node.LabelType == "text" {
				label = escapeMarkdownMarkers(label)
			}
			node.Label, node.HasLabel = "«"+escapeMarkdownMarkers(node.Stereotype)+"»<br/>"+label, true
			node.LabelType = "markdown"
			node.Set("hasFoldedStereotype", true)
			// delete node.stereotype
			node.Stereotype, node.HasStereotype = "", false
		}
	}
	for _, edge := range data.Edges {
		if edge.HasLabel && edge.LabelType == "text" {
			edge.Label = escapePlainLabel(edge.Label)
		}
	}
	return data
}

// annotateUsecaseElements is annotateUsecaseElements(svg, data, accessibleNames).
func annotateUsecaseElements(svg *dom.Node, data *ru.LayoutData, accessibleNames usecaseAccessibleNames) {
	for _, node := range data.Nodes {
		// typeof node.domId === 'string' ? node.domId : usecaseDomId(data.diagramId, node.id)
		stableDomID := node.DomID
		if stableDomID == "" {
			stableDomID = usecaseDomID(data.DiagramID, node.ID)
		}
		element := svg.Select("#" + stableDomID)
		var kind string
		switch {
		case node.IsGroup:
			kind = "boundary"
		case actorShapes[node.Shape]:
			kind = "actor"
		case node.Shape == "note":
			kind = "note"
		case node.Shape == "usecaseJsonTable":
			kind = "json"
		default:
			kind = "usecase"
		}
		accessibleName, ok := accessibleNames.nodes.Lookup(node.ID)
		if !ok {
			accessibleName = node.ID
		}
		element.
			Attr("data-usecase-id", node.ID).
			Attr("data-usecase-kind", kind).
			Attr("role", "img").
			Attr("aria-label", accessibleName)
		if !node.IsGroup &&
			(node.Shape == "usecaseEllipse" || node.Shape == "rect") &&
			node.Get("hasFoldedStereotype") == true {
			root := element
			htmlLabel := root.Select(".nodeLabel")
			container := htmlLabel.Select("p")
			if container == nil {
				container = htmlLabel
			}
			var firstLabelNode *dom.Node
			if container != nil {
				firstLabelNode = container.FirstChild
			}
			if container != nil && firstLabelNode != nil && firstLabelNode.Type == dom.TextNode {
				stereotype := container.Document().CreateElement("span")
				stereotype.Attr("class", "usecase-stereotype")
				container.InsertBefore(stereotype, firstLabelNode)
				stereotype.AppendChild(firstLabelNode)
			} else {
				root.Select(".label tspan tspan, .label tspan").Classed("usecase-stereotype", true)
			}
		}
	}

	// Index the edges by id so the paths are matched in a single pass instead of
	// re-scanning every path for each edge.
	edgesByID := js.NewMap[string, *ru.Edge]()
	for _, edge := range data.Edges {
		edgesByID.Set(edge.ID, edge)
	}
	for _, path := range svg.SelectAll(`path[data-et="edge"]`) {
		edge := edgesByID.Get(path.AttrOr("data-id", ""))
		if edge == nil {
			continue
		}
		internal := js.Truthy(edge.Get("internal"))
		kind := "relationship"
		if internal {
			kind = "note-connector"
		}
		path.
			Attr("id", usecaseDomID(data.DiagramID, edge.ID)).
			Attr("data-usecase-id", edge.ID).
			Attr("data-usecase-kind", kind)
		if internal {
			path.Attr("aria-hidden", "true")
		} else {
			name, ok := accessibleNames.edges.Lookup(edge.ID)
			if !ok {
				name = edge.ID
			}
			path.Attr("role", "img").Attr("aria-label", name)
		}
	}
}

// styleValue is the value of selection.style(name, value) for a property
// of the layout data: undefined and null remove the property.
func styleValue(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return js.ToString(v)
}

// applyUsecaseFonts is applyUsecaseFonts(svg, data): it publishes the
// configured use-case fonts on the diagram root as the CSS custom properties
// that styles.ts reads through `var(--mermaid-usecase-*, <fallback>)`.
//
// This has to be applied twice. The labels are measured while render runs,
// and the measured width is frozen onto each label foreignObject; the fonts
// therefore have to be in force by then. setupViewPortForSVG then runs
// configureSvgSize, which assigns the whole style attribute and so wipes the
// custom properties again.
func applyUsecaseFonts(svg *dom.Node, data *ru.LayoutData) {
	svg.
		Style("--mermaid-usecase-actor-font-size", js.ToString(data.Extra["actorFontSize"])+"px").
		Style("--mermaid-usecase-actor-font-family", styleValue(data.Extra["actorFontFamily"])).
		Style("--mermaid-usecase-actor-font-weight", styleValue(data.Extra["actorFontWeight"])).
		Style("--mermaid-usecase-font-size", js.ToString(data.Extra["usecaseFontSize"])+"px").
		Style("--mermaid-usecase-font-family", styleValue(data.Extra["usecaseFontFamily"])).
		Style("--mermaid-usecase-font-weight", styleValue(data.Extra["usecaseFontWeight"]))
}

// layoutName is the argument of getRegisteredLayoutAlgorithm for the
// `layout` of the config. Its default parameter is the empty string.
func layoutName(layout any) string {
	if layout == nil {
		return ""
	}
	return js.ToString(layout)
}

// drawing is the state of draw between the layout data and the layout.
type drawing struct {
	data            *ru.LayoutData
	accessibleNames usecaseAccessibleNames
	svg             *dom.Node
}

// prepareDraw is the part of draw before render(data4Layout, svg): it
// builds the layout data and the accessible names.
func prepareDraw(ctx *render.Context, usecaseDB *DB) *drawing {
	layout := ctx.Cfg("layout")

	// The getData method provided in all supported diagrams is used to extract the data from the parsed structure
	// into the Layout data format
	data4Layout := usecaseDB.GetData()
	accessibleLabels := js.NewMap[string, string]()
	for _, node := range data4Layout.Nodes {
		accessibleLabels.Set(node.ID, getAccessibleLabel(nodeLabelOrID(node), node.LabelType))
	}
	accessibleNames := usecaseAccessibleNames{
		nodes: js.NewMap[string, string](),
		edges: js.NewMap[string, string](),
	}
	for _, node := range data4Layout.Nodes {
		var name string
		if node.IsGroup {
			name = getUsecaseBoundaryAccessibleName(node)
		} else if node.Shape == "note" {
			// { ...node, noteTargetLabel: accessibleLabels.get(node.noteTarget ?? '') }
			noteTarget, _ := stringProp(node.Extra, "noteTarget")
			noteTargetLabel, ok := accessibleLabels.Lookup(noteTarget)
			name = getUsecaseNodeAccessibleName(node, noteTargetLabel, ok)
		} else {
			noteTargetLabel, ok := stringProp(node.Extra, "noteTargetLabel")
			name = getUsecaseNodeAccessibleName(node, noteTargetLabel, ok)
		}
		accessibleNames.nodes.Set(node.ID, name)
	}
	for _, edge := range data4Layout.Edges {
		// accessibleLabels.get(edge.source) ?? edge.sourceLabel
		sourceLabel, ok := accessibleLabels.Lookup(edge.Source)
		if !ok {
			sourceLabel = js.ToString(edge.Get("sourceLabel"))
		}
		targetLabel, ok := accessibleLabels.Lookup(edge.Target)
		if !ok {
			targetLabel = js.ToString(edge.Get("targetLabel"))
		}
		accessibleNames.edges.Set(edge.ID, getUsecaseEdgeAccessibleName(edge, sourceLabel, targetLabel))
	}
	svg := ctx.GetDiagramElement()

	data4Layout.LayoutAlgorithm = ru.GetRegisteredLayoutAlgorithm(layoutName(layout), "")
	prepareUsecaseLayoutData(data4Layout, ctx.ID)
	return &drawing{data: data4Layout, accessibleNames: accessibleNames, svg: svg}
}

// draw is draw(text, id, version, diag): the main draw function, which
// uses the unified rendering system.
func draw(ctx *render.Context, usecaseDB *DB) error {
	d := prepareDraw(ctx, usecaseDB)
	data4Layout, svg := d.data, d.svg

	// Set before `render` so the labels are measured in the fonts they will be painted in.
	applyUsecaseFonts(svg, data4Layout)

	if err := ru.Render(ctx, data4Layout, svg); err != nil {
		return err
	}
	// The fonts above are written through the CSSOM. Blink keeps such a
	// style out of the attribute list until something reads the attributes
	// or, for an svg element, until the next style recalculation. The layout
	// measures labels and yields to the event loop, so the browser has
	// recalculated the style before render returns: the style attribute of
	// the root exists before setupViewPortForSVG adds the class attribute.
	// Reading the attribute makes it exist here too.
	//
	// This is what the IIFE build of mermaid gives (mermaid.min.js, the
	// build of the plain reference page). With the ESM build, which
	// mermaid-cli loads, the style attribute does not exist yet when
	// setupViewPortForSVG adds the class attribute: the result has class
	// before style (tools/reference/probes/usecase-attr-order.js).
	if ctx.Page == render.PagePlain {
		svg.GetAttr("style")
	}
	annotateUsecaseElements(svg, data4Layout, d.accessibleNames)

	padding := ru.Undef
	if v, ok := data4Layout.Extra["diagramPadding"]; ok && v != nil {
		padding = js.ToNumber(v)
	}
	render.InsertTitle(
		svg,
		"usecaseDiagramTitleText",
		0, // Default title top margin
		usecaseDB.GetDiagramTitle(),
	)
	render.SetupViewPortForSVG(svg, padding, "usecaseDiagram", js.Truthy(data4Layout.Extra["useMaxWidth"]))
	// Restore the fonts that `setupViewPortForSVG` just dropped from the `style` attribute.
	applyUsecaseFonts(svg, data4Layout)
	return nil
}
