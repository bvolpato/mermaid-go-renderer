// Ported from mermaid 12.2.0, diagrams/flowchart/flowDb.ts (getData and its
// helpers) (MIT).

package flowchart

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// A layout node or edge holds strings, numbers and booleans. A property of
// upstream that comes from the YAML metadata or from the config can hold a
// value of another type. The functions below convert such a value for the
// field, and keep the JavaScript value under the name of the property in
// Extra, so that the value is not lost. NodeJSON and EdgeJSON print it.

type extraSetter interface{ Set(key string, v any) }

// dynString converts a property value to a string field. Undefined is "".
func dynString(x extraSetter, key string, v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case string:
		return s
	}
	x.Set(key, v)
	return js.ToString(v)
}

// dynNumber converts a property value to a number field. Undefined is NaN.
func dynNumber(x extraSetter, key string, v any) float64 {
	switch n := v.(type) {
	case nil:
		return ru.Undef
	case float64:
		return n
	}
	x.Set(key, v)
	return toNumber(v)
}

// labelOf converts the text of a vertex to the label of a node. Upstream
// draws `label[0]` for a label that is not a string, so the label of an
// array is its first element, and another value has no text to draw.
func labelOf(node *ru.Node, text any) {
	switch t := text.(type) {
	case nil:
		return
	case string:
		node.Label, node.HasLabel = t, true
		return
	case []any:
		if len(t) > 0 {
			if first, ok := t[0].(string); ok {
				node.Label, node.HasLabel = first, true
			}
		}
	}
	node.Set("label", text)
}

// getTypeFromVertex is getTypeFromVertex(vertex).
func getTypeFromVertex(vertex *FlowVertex) string {
	if js.Truthy(vertex.Img) {
		return "imageSquare"
	}
	if js.Truthy(vertex.Icon) {
		if js.StrictEquals(vertex.Form, "circle") {
			return "iconCircle"
		}
		if js.StrictEquals(vertex.Form, "square") {
			return "iconSquare"
		}
		if js.StrictEquals(vertex.Form, "rounded") {
			return "iconRounded"
		}
		return "icon"
	}
	if !vertex.HasType {
		return "squareRect"
	}
	switch vertex.Type {
	case "square":
		return "squareRect"
	case "round":
		return "roundedRect"
	case "ellipse":
		// Ellipses are broken, see https://github.com/mermaid-js/mermaid/issues/5976
		return "ellipse"
	default:
		return vertex.Type
	}
}

// destructEdgeType is destructEdgeType(type).
func destructEdgeType(typ string) (arrowTypeStart, arrowTypeEnd string) {
	arrowTypeStart = "none"
	arrowTypeEnd = "arrow_point"
	switch typ {
	case "arrow_point", "arrow_circle", "arrow_cross":
		arrowTypeEnd = typ

	case "double_arrow_point", "double_arrow_circle", "double_arrow_cross":
		arrowTypeStart = strings.Replace(typ, "double_", "", 1)
		arrowTypeEnd = arrowTypeStart
	}
	return arrowTypeStart, arrowTypeEnd
}

// nodeList is the nodes array of getData, with an index for findNode.
type nodeList struct {
	nodes []*ru.Node
	// byID has the first node of each id: nodes.find((node) => node.id === id).
	byID map[string]*ru.Node
}

func (l *nodeList) push(n *ru.Node) {
	l.nodes = append(l.nodes, n)
	if _, ok := l.byID[n.ID]; !ok {
		l.byID[n.ID] = n
	}
}

// addNodeFromVertex is addNodeFromVertex(vertex, nodes, parentDB,
// subGraphDB, config, look).
func (d *DB) addNodeFromVertex(vertex *FlowVertex, nodes *nodeList, parentDB *js.Map[string, string], subGraphDB *js.Map[string, bool], config *js.Obj[any], look any) {
	parentID, _ := parentDB.Lookup(vertex.ID)
	isGroup, _ := subGraphDB.Lookup(vertex.ID)

	if node := nodes.byID[vertex.ID]; node != nil {
		node.CSSStyles, node.CSSStylesSet = vertex.Styles, true
		node.CSSCompiledStyles = d.getCompiledStyles(vertex.Classes)
		node.CSSClasses, node.CSSClassesSet = strings.Join(vertex.Classes, " "), true
		return
	}
	node := ru.NewNode(vertex.ID)
	labelOf(node, vertex.Text)
	node.LabelType = vertex.LabelType
	node.LabelStyle = ""
	node.ParentID = parentID
	// config.flowchart?.padding || 8
	var padding any = 8.0
	if v := js.Path(config, "flowchart", "padding"); js.Truthy(v) {
		padding = v
	}
	node.Padding = dynNumber(node, "padding", padding)
	node.CSSStyles, node.CSSStylesSet = vertex.Styles, true
	node.CSSCompiledStyles = d.getCompiledStyles(append([]string{"default", "node"}, vertex.Classes...))
	node.CSSClasses, node.CSSClassesSet = "default "+strings.Join(vertex.Classes, " "), true
	if vertex.HasDir {
		node.Dir = vertex.Dir
	}
	node.DomID = vertex.DomID
	node.Look = dynString(node, "look", look)
	if vertex.HasLink {
		node.Link = vertex.Link
	}
	if vertex.HasLinkTarget {
		node.LinkTarget = vertex.LinkTarget
	}
	node.Tooltip, _ = d.GetTooltip(vertex.ID)
	node.Icon = dynString(node, "icon", vertex.Icon)
	node.Pos = dynString(node, "pos", vertex.Pos)
	node.Img = dynString(node, "img", vertex.Img)
	if vertex.HasAssetWidth {
		node.AssetWidth = vertex.AssetWidth
		if node.AssetWidth != node.AssetWidth {
			// NaN stands for undefined in the field. Keep the number.
			node.Set("assetWidth", vertex.AssetWidth)
		}
	}
	if vertex.HasAssetHeight {
		node.AssetHeight = vertex.AssetHeight
		if node.AssetHeight != node.AssetHeight {
			node.Set("assetHeight", vertex.AssetHeight)
		}
	}
	node.Constraint = dynString(node, "constraint", vertex.Constraint)
	if isGroup {
		node.IsGroup = true
		node.Shape = "rect"
	} else {
		node.IsGroup = false
		node.Shape = getTypeFromVertex(vertex)
		node.MinWidth = dynNumber(node, "minWidth", js.Path(config, "flowchart", "minNodeWidth"))
	}
	nodes.push(node)
}

// getCompiledStyles is getCompiledStyles(classDefs).
func (d *DB) getCompiledStyles(classDefs []string) []string {
	compiledStyles := []string{}
	for _, customClass := range classDefs {
		cssClass, ok := d.classes.Lookup(customClass)
		if !ok {
			continue
		}
		if cssClass.Styles != nil {
			compiledStyles = trimAll(append(compiledStyles, cssClass.Styles...))
		}
		if cssClass.TextStyles != nil {
			compiledStyles = trimAll(append(compiledStyles, cssClass.TextStyles...))
		}
	}
	return compiledStyles
}

// trimAll is list.map((s) => s.trim()), in place.
func trimAll(list []string) []string {
	for i, s := range list {
		list[i] = js.TrimSpace(s)
	}
	return list
}

// GetData is getData(): the nodes and the edges for the unified renderer,
// and the config.
func (d *DB) GetData() *ru.LayoutData {
	config := d.getConfig()
	nodes := &nodeList{byID: map[string]*ru.Node{}}
	edges := []*ru.Edge{}

	subGraphs := d.GetSubGraphs()
	parentDB := js.NewMap[string, string]()
	subGraphDB := js.NewMap[string, bool]()

	// Collapsible subgraphs (issue #7784).
	// A subgraph carrying `@{ view: collapsed }` is drawn as a single compact
	// node; its descendants are hidden and any edge that crosses the boundary
	// is redirected to the outermost collapsed ancestor.
	//
	// `subGraphParent` maps a subgraph id to the subgraph that directly
	// contains it, so we can walk up the containment chain to find the
	// outermost collapsed ancestor regardless of declaration order.
	subGraphParent := js.NewMap[string, string]()
	for _, sg := range subGraphs {
		for _, childID := range sg.Nodes {
			if d.subGraphLookup.Has(childID) {
				subGraphParent.Set(childID, sg.ID)
			}
		}
	}
	// Colour slots follow the order the subgraphs appear in the source.
	//
	// `subGraphs` is not in that order: the grammar reduces a subgraph when it *closes*,
	// so a nested one lands before its parent -- `Outer { InnerOne, InnerTwo }, Sibling`
	// arrives as [InnerOne, InnerTwo, Outer, Sibling]. Taking the array index directly
	// would hand Outer slot 2 while its own children took 0 and 1.
	//
	// A pre-order walk of the containment forest recovers source order: roots complete in
	// source order relative to each other, and a parent is always declared before the
	// children it contains. Assigned once here so the collapsed and expanded branches
	// below cannot drift apart.
	declarationIndex := js.NewMap[string, int]()
	childrenOf := js.NewMap[string, []string]()
	for _, sg := range subGraphs {
		if parent, ok := subGraphParent.Lookup(sg.ID); ok {
			children, _ := childrenOf.Lookup(parent)
			childrenOf.Set(parent, append(append(make([]string, 0, len(children)+1), children...), sg.ID))
		}
	}
	nextDeclarationIndex := 0
	// The walk of upstream recurses without a guard. A subgraph cannot be
	// its own ancestor here: a subgraph with a parent is never a root, and
	// a child is listed under one parent only. The depth limit keeps a
	// malformed input from overflowing the stack.
	var walk func(sgID string, depth int)
	walk = func(sgID string, depth int) {
		declarationIndex.Set(sgID, nextDeclarationIndex)
		nextDeclarationIndex++
		if depth > len(subGraphs) {
			return
		}
		children, _ := childrenOf.Lookup(sgID)
		for _, childID := range children {
			walk(childID, depth+1)
		}
	}
	for _, sg := range subGraphs {
		if !subGraphParent.Has(sg.ID) {
			walk(sg.ID, 0)
		}
	}

	isCollapsed := func(sgID string) bool {
		sg, ok := d.subGraphLookup.Lookup(sgID)
		if !ok || sg.Metadata == nil {
			return false
		}
		return js.StrictEquals(sg.Metadata.Get("view"), "collapsed")
	}
	outermostCollapsed := func(sgID string) (string, bool) {
		result, found := "", false
		seen := map[string]bool{}
		current, ok := sgID, true
		for ok && !seen[current] {
			seen[current] = true
			if isCollapsed(current) {
				result, found = current, true
			}
			current, ok = subGraphParent.Lookup(current)
		}
		return result, found
	}

	// `hiddenIds` are nodes/subgraphs that are not drawn; `collapsedAncestorMap`
	// maps each hidden id to the visible collapsed node that replaces it.
	hiddenIDs := map[string]bool{}
	collapsedAncestorMap := js.NewMap[string, string]()
	for _, sg := range subGraphs {
		ancestor, ok := outermostCollapsed(sg.ID)
		if !ok {
			continue
		}
		// Hide the subgraph itself unless it is the visible collapsed node.
		if sg.ID != ancestor {
			hiddenIDs[sg.ID] = true
			collapsedAncestorMap.Set(sg.ID, ancestor)
		}
		// Hide every member, redirecting it to the visible collapsed node.
		for _, childID := range sg.Nodes {
			if childID == ancestor {
				continue
			}
			hiddenIDs[childID] = true
			collapsedAncestorMap.Set(childID, ancestor)
		}
	}

	// Setup the subgraph data for adding nodes
	for i := len(subGraphs) - 1; i >= 0; i-- {
		subGraph := subGraphs[i]
		if hiddenIDs[subGraph.ID] {
			continue
		}
		if len(subGraph.Nodes) > 0 {
			subGraphDB.Set(subGraph.ID, true)
		}
		for _, id := range subGraph.Nodes {
			parentDB.Set(id, subGraph.ID)
		}
	}

	// Data is setup, add the nodes
	for i := len(subGraphs) - 1; i >= 0; i-- {
		subGraph := subGraphs[i]
		if hiddenIDs[subGraph.ID] {
			continue
		}
		collapsed := subGraph.Metadata != nil && js.StrictEquals(subGraph.Metadata.Get("view"), "collapsed")
		node := ru.NewNode(subGraph.ID)
		node.Label, node.HasLabel = subGraph.Title, true
		node.LabelStyle = ""
		node.LabelType = subGraph.LabelType
		node.ParentID, _ = parentDB.Lookup(subGraph.ID)
		node.Padding = 8
		node.CSSCompiledStyles = d.getCompiledStyles(subGraph.Classes)
		node.CSSClasses, node.CSSClassesSet = strings.Join(subGraph.Classes, " "), true
		if subGraph.HasDir {
			node.Dir = subGraph.Dir
		}
		node.Look = dynString(node, "look", config.Get("look"))
		// A collapsed subgraph still consumes its slot, so the cycle does not shift
		// when one is collapsed.
		if index, ok := declarationIndex.Lookup(subGraph.ID); ok {
			node.ColorIndex = float64(index)
		}
		if collapsed {
			// Collapsed: draw as a single compact node instead of a container.
			node.Shape = "collapsedGroup"
			node.IsGroup = false
		} else {
			node.Shape = "rect"
			node.IsGroup = true
			// Forwarded so layout engines can read per-container settings such as
			// `@{ algorithm: elk.box }`. `view` is consumed above; everything else
			// is opaque here and simply passed through.
			node.Metadata = subGraph.Metadata
		}
		nodes.push(node)
	}

	// config.look || 'classic'
	var look any = "classic"
	if v := config.Get("look"); js.Truthy(v) {
		look = v
	}
	for _, vertex := range d.GetVertices().Values() {
		// Skip vertices hidden inside a collapsed subgraph
		if hiddenIDs[vertex.ID] {
			continue
		}
		d.addNodeFromVertex(vertex, nodes, parentDB, subGraphDB, config, look)
	}

	for index, rawEdge := range d.GetEdges() {
		arrowTypeStart, arrowTypeEnd := destructEdgeType(rawEdge.Type)
		styles := append([]string{}, d.defaultStyle...)

		// Redirect boundary-crossing edges to the visible collapsed node. An
		// edge that becomes a self-loop purely because both endpoints collapsed
		// into the same node (i.e. it was internal to a collapsed subgraph) is
		// dropped. Self-loops on nodes that are not collapsed are preserved.
		start, startCollapsed := collapsedAncestorMap.Lookup(rawEdge.Start)
		if !startCollapsed {
			start = rawEdge.Start
		}
		end, endCollapsed := collapsedAncestorMap.Lookup(rawEdge.End)
		if !endCollapsed {
			end = rawEdge.End
		}
		if start == end && (startCollapsed || endCollapsed) {
			continue
		}

		if rawEdge.Style != nil {
			styles = append(styles, rawEdge.Style...)
		}
		edge := ru.NewEdge(utils.GetEdgeId(start, end, utils.EdgeIdOptions{Counter: index, Prefix: "L"}, rawEdge.ID))
		edge.IsUserDefinedID = rawEdge.IsUserDefinedID
		edge.Start = start
		edge.End = end
		edge.Type = rawEdge.Type
		if edge.Type == "" {
			edge.Type = "normal"
		}
		edge.Label, edge.HasLabel = rawEdge.Text, true
		edge.LabelType = rawEdge.LabelType
		edge.Labelpos = "c"
		edge.Thickness = rawEdge.Stroke
		edge.Minlen = rawEdge.Length
		invisible := rawEdge.Stroke == "invisible"
		if invisible {
			edge.Classes = ""
		} else {
			edge.Classes = "edge-thickness-normal edge-pattern-solid flowchart-link"
		}
		if invisible || rawEdge.Type == "arrow_open" {
			edge.ArrowTypeStart = "none"
			edge.ArrowTypeEnd = "none"
		} else {
			edge.ArrowTypeStart = arrowTypeStart
			edge.ArrowTypeEnd = arrowTypeEnd
		}
		edge.ArrowheadStyle = "fill: #333"
		edge.CSSCompiledStyles = d.getCompiledStyles(rawEdge.Classes)
		// labelStyle and style are the same array upstream.
		edge.LabelStyle = styles
		edge.Style = styles
		edge.Pattern = rawEdge.Stroke
		edge.Look = dynString(edge, "look", config.Get("look"))
		if rawEdge.Animate != nil {
			edge.HasAnimate = true
			if b, ok := rawEdge.Animate.(bool); ok {
				edge.Animate = b
			} else {
				edge.Animate = js.Truthy(rawEdge.Animate)
				edge.Set("animate", rawEdge.Animate)
			}
		}
		edge.Animation = dynString(edge, "animation", rawEdge.Animation)
		// rawEdge.interpolate || this.edges.defaultInterpolate || config.flowchart?.curve
		curve := rawEdge.Interpolate
		if !js.Truthy(curve) {
			curve = nil
			if d.hasDefaultInterpolate {
				curve = d.defaultInterpolate
			}
		}
		if !js.Truthy(curve) {
			curve = js.Path(config, "flowchart", "curve")
		}
		edge.Curve = dynString(edge, "curve", curve)

		edges = append(edges, edge)
	}

	return &ru.LayoutData{Nodes: nodes.nodes, Edges: edges, Config: config}
}
