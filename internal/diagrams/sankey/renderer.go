// Ported from mermaid 12.2.0, diagrams/sankey/sankeyRenderer.ts and
// rendering-util/uid.ts (MIT).

package sankey

import (
	"fmt"

	d3sankey "github.com/bvolpato/mermaid-go-renderer/internal/d3/sankey"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/scale"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// uid is Uid of rendering-util/uid.ts: an id of an element and the url
// reference to it.
type uid struct {
	id   string
	href string
}

// nextUID is Uid.next(name). The counter of upstream is a static field, so
// it lives in the render context.
func nextUID(ctx *render.Context, name string) *uid {
	count := ctx.Counter("Uid") + 1
	id := name + js.Str(float64(count))
	return &uid{id: id, href: "#" + id}
}

// String is Uid.toString().
func (u *uid) String() string {
	return "url(" + u.href + ")"
}

// alignmentsMap maps the config options to the alignment functions.
var alignmentsMap = map[string]d3sankey.Align{
	"left":    d3sankey.Left,
	"right":   d3sankey.Right,
	"center":  d3sankey.Center,
	"justify": d3sankey.Justify,
}

// objectPrototype holds the properties that a plain JavaScript object
// inherits, with the result of String() for each. A read of nodeColors[id]
// finds them when the id of a node is one of these names.
var objectPrototype = map[string]string{
	"constructor":          "function Object() { [native code] }",
	"__defineGetter__":     "function __defineGetter__() { [native code] }",
	"__defineSetter__":     "function __defineSetter__() { [native code] }",
	"hasOwnProperty":       "function hasOwnProperty() { [native code] }",
	"__lookupGetter__":     "function __lookupGetter__() { [native code] }",
	"__lookupSetter__":     "function __lookupSetter__() { [native code] }",
	"isPrototypeOf":        "function isPrototypeOf() { [native code] }",
	"propertyIsEnumerable": "function propertyIsEnumerable() { [native code] }",
	"toString":             "function toString() { [native code] }",
	"valueOf":              "function valueOf() { [native code] }",
	"__proto__":            "[object Object]",
	"toLocaleString":       "function toLocaleString() { [native code] }",
}

// nullish is the ?? operator for two operands.
func nullish(a, b any) any {
	if js.IsNullish(a) {
		return b
	}
	return a
}

// findCentralNodeLayer finds the layer of the central node (the node with
// the maximum value). It is used for the label positions.
func findCentralNodeLayer(nodes []*d3sankey.Node) int {
	maxValue := 0.0
	centralLayer := 0

	for _, node := range nodes {
		value := node.Value
		if value > maxValue {
			maxValue = value
			centralLayer = node.Layer
		}
	}

	return centralLayer
}

// drawSankey is draw(text, id, _version, diagObj).
func drawSankey(ctx *render.Context, db *DB) error {
	// Get Sankey config
	conf := ctx.Cfg("sankey")
	defaultSankeyConfig := ctx.ConfigState.DefaultSection("sankey")
	get := func(key string) any {
		if js.IsNullish(conf) {
			return nil
		}
		v, _ := js.GetProperty(conf, key)
		return v
	}
	def := func(key string) any {
		v, _ := js.GetProperty(defaultSankeyConfig, key)
		return v
	}

	svg := ctx.GetDiagramElement()

	// Establish svg dimensions and get width and height. Upstream takes the
	// default of the height from the default width.
	width := nullish(get("width"), def("width"))
	height := nullish(get("height"), def("width"))
	useMaxWidth := nullish(get("useMaxWidth"), def("useMaxWidth"))
	nodeAlignment := nullish(get("nodeAlignment"), def("nodeAlignment"))
	prefix := nullish(get("prefix"), def("prefix"))
	suffix := nullish(get("suffix"), def("suffix"))
	showValues := js.Truthy(nullish(get("showValues"), def("showValues")))

	// New config options with defaults
	nodeWidth := nullish(nullish(get("nodeWidth"), def("nodeWidth")), 10.0)
	nodePadding := nullish(nullish(get("nodePadding"), def("nodePadding")), 12.0)
	labelStyle := nullish(nullish(get("labelStyle"), def("labelStyle")), "legacy")
	nodeColors := nullish(get("nodeColors"), js.NewObj[any]())

	// Prepare data for construction based on the db.
	dbGraph := db.GetGraph()
	graph := &d3sankey.Graph{}
	for _, node := range dbGraph.Nodes {
		graph.Nodes = append(graph.Nodes, &d3sankey.Node{Data: node.ID})
	}
	for _, link := range dbGraph.Links {
		graph.Links = append(graph.Links, &d3sankey.Link{SourceID: link.Source, TargetID: link.Target, Value: link.Value})
	}
	nodeID := func(d *d3sankey.Node) string { return d.Data.(string) }

	// Get alignment function
	alignmentName, _ := nodeAlignment.(string)
	nodeAlign, ok := alignmentsMap[alignmentName]
	if !ok {
		// alignmentsMap[nodeAlignment] is not one of the four functions.
		// d3-sankey then gets a column that is not a number for each node,
		// and the layout throws this TypeError.
		return fmt.Errorf("Cannot read properties of undefined (reading 'length')")
	}

	// Construct and configure a Sankey generator. That will be a function
	// that calculates nodes and links dimensions.
	extraPadding := 0.0
	if showValues {
		extraPadding = 15
	}
	sankey := d3sankey.New().
		NodeID(func(d *d3sankey.Node, _ int, _ []*d3sankey.Node) any { return d.Data }). // we use 'id' property to identify node
		NodeWidth(js.ToNumber(nodeWidth)).
		NodePadding(js.ToNumber(js.Add(nodePadding, extraPadding))).
		NodeAlign(nodeAlign).
		Extent(0, 0, js.ToNumber(width), js.ToNumber(height))

	// Compute the Sankey layout: calculate nodes and links positions. Our
	// graph object will be mutated by this and enriched with other
	// properties.
	if err := sankey.Layout(graph); err != nil {
		return err
	}

	// Find central node layer for smart label positioning
	centralNodeLayer := findCentralNodeLayer(graph.Nodes)

	// Get color scheme for the graph (fallback for nodes without custom
	// colors)
	colorScheme := scale.NewOrdinal[string, string]().Range(schemeTableau10...)

	// getNodeColor gets the color for a node, using the custom nodeColors
	// config if available, otherwise falling back to the default color
	// scheme.
	getNodeColor := func(nodeID string) any {
		color, _ := js.GetProperty(nodeColors, nodeID)
		if color == nil && !js.HasOwn(nodeColors, nodeID) {
			if _, plain := nodeColors.(*js.Obj[any]); plain {
				if inherited, ok := objectPrototype[nodeID]; ok {
					return inherited
				}
			}
		}
		if js.IsNullish(color) {
			return colorScheme.Scale(nodeID)
		}
		return color
	}

	// Create rectangles for nodes
	nodesGroup := svg.Append("g").Attr("class", "nodes")
	for _, d := range graph.Nodes {
		nodesGroup.Append("g").
			Attr("class", "node").
			Attr("id", nextUID(ctx, "node-").id).
			Attr("transform", "translate("+js.Str(d.X0)+","+js.Str(d.Y0)+")").
			Attr("x", d.X0).
			Attr("y", d.Y0).
			Append("rect").
			Attr("height", d.Y1-d.Y0).
			Attr("width", d.X1-d.X0).
			Attr("fill", getNodeColor(nodeID(d)))
	}

	getText := func(d *d3sankey.Node) string {
		if !showValues {
			return nodeID(d)
		}
		return nodeID(d) + "\n" + js.ToString(prefix) + js.Str(js.Round(d.Value*100)/100) + js.ToString(suffix)
	}

	// getLabelPosition determines the label position based on the node
	// position or layer. For the 'outlined' style it uses the layer of the
	// node relative to the central node. For the 'legacy' style it uses the
	// position of the node.
	getLabelPosition := func(d *d3sankey.Node) (x float64, anchor string) {
		if js.StrictEquals(labelStyle, "outlined") {
			// Layer-based: left-of-center nodes get labels on the left
			nodeLayer := d.Layer
			if nodeLayer < centralNodeLayer {
				return d.X0 - 6, "end"
			}
			return d.X1 + 6, "start"
		}
		// Legacy: position-based (original behavior)
		if d.X0 < js.ToNumber(width)/2 {
			return d.X1 + 6, "start"
		}
		return d.X0 - 6, "end"
	}

	// Create labels for nodes
	labelsGroup := svg.Append("g").Attr("class", "node-labels").Attr("font-size", 14)

	dy := "0.35em"
	if showValues {
		dy = "0em"
	}
	appendLabel := func(className string) {
		for _, d := range graph.Nodes {
			x, anchor := getLabelPosition(d)
			label := labelsGroup.Append("text")
			if className != "" {
				label.Attr("class", className)
			}
			label.
				Attr("x", x).
				Attr("y", (d.Y1+d.Y0)/2).
				Attr("dy", dy).
				Attr("text-anchor", anchor).
				Text(getText(d))
		}
	}

	if js.StrictEquals(labelStyle, "outlined") {
		appendLabel("sankey-label-bg")
		appendLabel("sankey-label-fg")
	} else {
		appendLabel("")
	}

	// Creates the paths that represent the links.
	linksGroup := svg.Append("g").
		Attr("class", "links").
		Attr("fill", "none").
		Attr("stroke-opacity", 0.5)

	linkColor := nullish(get("linkColor"), "gradient")
	gradient := js.StrictEquals(linkColor, "gradient")

	for _, d := range graph.Links {
		link := linksGroup.Append("g").
			Attr("class", "link").
			Style("mix-blend-mode", "multiply")

		var linkUID *uid
		if gradient {
			linkUID = nextUID(ctx, "linearGradient-")
			gradient := link.Append("linearGradient").
				Attr("id", linkUID.id).
				Attr("gradientUnits", "userSpaceOnUse").
				Attr("x1", d.Source.X1).
				Attr("x2", d.Target.X0)

			gradient.Append("stop").
				Attr("offset", "0%").
				Attr("stop-color", getNodeColor(nodeID(d.Source)))

			gradient.Append("stop").
				Attr("offset", "100%").
				Attr("stop-color", getNodeColor(nodeID(d.Target)))
		}

		var coloring any
		switch {
		case gradient:
			coloring = linkUID.String()
		case js.StrictEquals(linkColor, "source"):
			coloring = getNodeColor(nodeID(d.Source))
		case js.StrictEquals(linkColor, "target"):
			coloring = getNodeColor(nodeID(d.Target))
		default:
			coloring = linkColor
		}

		link.Append("path").
			Attr("d", d3sankey.LinkHorizontal(d)).
			Attr("stroke", coloring).
			Attr("stroke-width", js.Max(1, d.Width))
	}

	render.SetupGraphViewbox(svg, 0, js.Truthy(useMaxWidth))
	return nil
}
