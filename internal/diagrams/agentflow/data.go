// Ported from mermaid 12.2.0, diagrams/agentflow/agentflowDb.ts (getData and
// its helpers) (MIT).

package agentflow

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
// An empty string is not undefined, so Extra keeps it.
func dynString(x extraSetter, key string, v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case string:
		if s != "" {
			return s
		}
	}
	x.Set(key, v)
	return js.ToString(v)
}

// toNumber is Number(v). An array or an object converts to a string first.
func toNumber(v any) float64 {
	switch v.(type) {
	case []any, *js.Obj[any]:
		return js.Number(js.ToString(v))
	}
	return js.ToNumber(v)
}

// dynNumber converts a property value to a number field. Undefined is NaN.
// The number NaN is not undefined, so Extra keeps it.
func dynNumber(x extraSetter, key string, v any) float64 {
	switch n := v.(type) {
	case nil:
		return ru.Undef
	case float64:
		if n == n {
			return n
		}
	}
	x.Set(key, v)
	return toNumber(v)
}

// getTypeFromVertex is getTypeFromVertex(vertex).
func getTypeFromVertex(vertex *FlowVertex) string {
	if !vertex.HasType {
		return "squareRect"
	}
	// Resolve v0.8.1 alias (e.g. `task` to `roundedRect`, `action` to `hexagon`)
	// before mapping to the canonical Mermaid shape id.
	resolved := resolveShapeAlias(vertex.Type)
	switch resolved {
	case "square":
		return "squareRect"
	case "round":
		return "roundedRect"
	case "ellipse":
		// Ellipses are broken, see https://github.com/mermaid-js/mermaid/issues/5976
		return "ellipse"
	default:
		return resolved
	}
}

// destructEdgeType is destructEdgeType(type).
func destructEdgeType(typ string) (arrowTypeStart, arrowTypeEnd string) {
	arrowTypeStart = "none"
	arrowTypeEnd = "arrow_point"
	switch typ {
	case "arrow_point", "arrow_circle", "arrow_cross", "arrow_hierarchy":
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
	parentID, hasParent := parentDB.Lookup(vertex.ID)
	isGroup, _ := subGraphDB.Lookup(vertex.ID)

	if node := nodes.byID[vertex.ID]; node != nil {
		node.CSSStyles, node.CSSStylesSet = vertex.Styles, true
		node.CSSCompiledStyles = d.getCompiledStyles(vertex.Classes)
		node.CSSClasses, node.CSSClassesSet = strings.Join(vertex.Classes, " "), true
		return
	}
	node := ru.NewNode(vertex.ID)
	if vertex.HasText {
		node.Label, node.HasLabel = vertex.Text, true
	}
	node.LabelType = vertex.LabelType
	node.LabelStyle = ""
	setParent(node, parentID, hasParent)
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
		node.Dir = dynString(node, "dir", vertex.Dir)
	}
	node.DomID = vertex.DomID
	node.Look = dynString(node, "look", look)
	if vertex.HasLink {
		node.Link = dynString(node, "link", vertex.Link)
	}
	if vertex.HasLinkTarget {
		node.LinkTarget = dynString(node, "linkTarget", vertex.LinkTarget)
	}
	if tooltip, ok := d.GetTooltip(vertex.ID); ok {
		node.Tooltip = dynString(node, "tooltip", tooltip)
	}
	node.Metadata = vertex.Metadata
	if isGroup {
		node.IsGroup = true
		node.Shape = "rect"
	} else {
		node.IsGroup = false
		node.Shape = getTypeFromVertex(vertex)
		node.WrappingWidth = dynNumber(node, "wrappingWidth", js.Path(config, "agentflow", "wrappingWidth"))
		node.MinWidth = dynNumber(node, "minWidth", js.Path(config, "agentflow", "minNodeWidth"))
	}
	nodes.push(node)
}

// setParent sets the parentId of a node. The id of a parent can be the
// empty string, which is not undefined, so Extra keeps it.
func setParent(node *ru.Node, parentID string, hasParent bool) {
	if !hasParent {
		return
	}
	node.ParentID = dynString(node, "parentId", parentID)
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

// runPostParseValidators is runPostParseValidators(): the hook that runs
// one time after the parse. The checks of the metadata and of the semantics
// are not part of the parser since v0.8.2, so the hook does nothing but
// keep its run-once guard.
func (d *DB) runPostParseValidators() {
	if d.postParseValidationRun {
		return
	}
	d.postParseValidationRun = true
}

// isCollapsed is sg.metadata?.view === 'collapsed'.
func isCollapsed(sg *FlowSubGraph) bool {
	return sg.Metadata != nil && js.StrictEquals(sg.Metadata.Get("view"), "collapsed")
}

// LayoutData is the result of getData(): the layout data for the unified
// renderer, and the connectors, which upstream returns beside the nodes and
// the edges.
type LayoutData struct {
	*ru.LayoutData
	// Connectors is the `connectors` property: getConnectors().
	Connectors []*FlowVertex
}

// GetData is getData(): the nodes and the edges for the unified renderer,
// the config and the connectors.
func (d *DB) GetData() *LayoutData {
	d.runPostParseValidators()
	config := d.getConfig()
	nodes := &nodeList{byID: map[string]*ru.Node{}}
	edges := []*ru.Edge{}

	subGraphs := d.GetSubGraphs()
	parentDB := js.NewMap[string, string]()
	subGraphDB := js.NewMap[string, bool]()

	// Collect IDs hidden by collapsed subgraphs (the subgraph's descendants).
	// `collapsedAncestorMap` records the *outermost* collapsed ancestor for
	// each hidden id so that cross-boundary edges can be redirected to the
	// visible collapsed node (issue #53). Nested collapses resolve to the
	// outermost ancestor because we process collapsed subgraphs in
	// declaration order (outer first) and the ancestor parameter is held
	// constant during recursion.
	hiddenIDs := map[string]bool{}
	collapsedAncestorMap := js.NewMap[string, string]()
	//
	// Containment can be cyclic: an edge written inside one container that
	// names another container folds that container into the first one's member
	// list, so `A` can contain `B` while `B` contains `A`. Without the `seen`
	// guard the recursion never terminated and `getData()` blew the stack,
	// blanking the diagram. The collapse root itself is excluded so a back-edge
	// cannot hide the very node the collapse is supposed to draw.
	//
	// The recursion of upstream is as deep as the nesting of the containers.
	// Here an explicit stack does the same walk, so that a deep nesting does
	// not use the call stack.
	collectDescendants := func(root, ancestor string) {
		seen := map[string]bool{}
		type frame struct {
			nodes []string
			next  int
		}
		var stack []frame
		enter := func(sgID string) {
			if seen[sgID] {
				return
			}
			seen[sgID] = true
			sg, ok := d.subGraphLookup.Lookup(sgID)
			if !ok {
				return
			}
			stack = append(stack, frame{nodes: sg.Nodes})
		}
		enter(root)
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.next >= len(top.nodes) {
				stack = stack[:len(stack)-1]
				continue
			}
			childID := top.nodes[top.next]
			top.next++
			if childID == ancestor || seen[childID] {
				continue
			}
			hiddenIDs[childID] = true
			collapsedAncestorMap.Set(childID, ancestor)
			// Recurse into child subgraphs
			enter(childID)
		}
	}
	for _, sg := range subGraphs {
		if isCollapsed(sg) && !hiddenIDs[sg.ID] {
			collectDescendants(sg.ID, sg.ID)
		}
	}

	// Setup the subgraph data for adding nodes
	//
	// A parent assignment that closes a containment cycle (`A` contains `B`
	// and `B` contains `A`, which two cross-boundary edges are enough to
	// produce) is refused: graphlib rejects `setParent` on a cycle and the
	// whole diagram would fail to render. The first assignment wins and the
	// one that would close the loop is reported as a containment violation.
	wouldCloseContainmentCycle := func(childID, parentID string) bool {
		seen := map[string]bool{}
		current, ok := parentID, true
		for ok && !seen[current] {
			if current == childID {
				return true
			}
			seen[current] = true
			current, ok = parentDB.Lookup(current)
		}
		return false
	}
	for i := len(subGraphs) - 1; i >= 0; i-- {
		subGraph := subGraphs[i]
		if hiddenIDs[subGraph.ID] {
			continue
		}
		if len(subGraph.Nodes) > 0 {
			subGraphDB.Set(subGraph.ID, true)
		}
		for _, id := range subGraph.Nodes {
			if wouldCloseContainmentCycle(id, subGraph.ID) {
				d.EmitWarning(
					WarningContainmentViolation,
					`Container "`+subGraph.ID+`" cannot contain "`+id+`" because "`+id+`" already contains it. The nesting that would close the loop is dropped.`,
					nodeContext(id),
				)
				continue
			}
			parentDB.Set(id, subGraph.ID)
		}
	}

	// Data is setup, add the nodes
	for i := len(subGraphs) - 1; i >= 0; i-- {
		subGraph := subGraphs[i]
		if hiddenIDs[subGraph.ID] {
			continue
		}

		node := ru.NewNode(subGraph.ID)
		node.Label, node.HasLabel = subGraph.Title, true
		node.LabelStyle = ""
		node.LabelType = subGraph.LabelType
		parentID, hasParent := parentDB.Lookup(subGraph.ID)
		setParent(node, parentID, hasParent)
		node.Padding = 8
		node.CSSCompiledStyles = d.getCompiledStyles(subGraph.Classes)
		node.CSSClasses, node.CSSClassesSet = strings.Join(subGraph.Classes, " "), true
		node.Dir = dynString(node, "dir", subGraph.dirJS())
		node.Look = dynString(node, "look", config.Get("look"))

		if isCollapsed(subGraph) {
			// Collapsed: render as a compact node with container-type styling and collapse indicator
			collapsedMetadata := spread(metadataOrNil(subGraph.Metadata))
			collapsedMetadata.Set("containerType", subGraph.Type)
			node.Shape = "collapsedGroup"
			node.IsGroup = false
			node.Metadata = collapsedMetadata
		} else {
			// SUBGRAPH_TYPE_TO_SHAPE[subGraph.type ?? 'flow']
			typ := subGraph.Type
			if typ == "" {
				typ = "flow"
			}
			node.Shape = subgraphTypeToShape[typ]
			node.IsGroup = true
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
		// Skip vertices hidden by collapsed subgraphs
		if hiddenIDs[vertex.ID] {
			continue
		}
		d.addNodeFromVertex(vertex, nodes, parentDB, subGraphDB, config, look)
	}

	for index, rawEdge := range d.GetEdges() {
		// Redirect cross-boundary edges to the outermost visible collapsed
		// ancestor (issue #53). When two *distinct* endpoints collapse to the
		// same ancestor the edge would be a self-loop on the collapsed node and
		// is dropped instead. Genuine self-loops authored as `a --> a` are kept
		// (issue #58): only collapse-induced self-loops are dropped.
		startID, ok := collapsedAncestorMap.Lookup(rawEdge.Start)
		if !ok {
			startID = rawEdge.Start
		}
		endID, ok := collapsedAncestorMap.Lookup(rawEdge.End)
		if !ok {
			endID = rawEdge.End
		}
		if startID == endID && rawEdge.Start != rawEdge.End {
			continue
		}
		arrowTypeStart, arrowTypeEnd := destructEdgeType(rawEdge.Type)
		styles := append([]string{}, d.defaultStyle...)

		if rawEdge.Style != nil {
			styles = append(styles, rawEdge.Style...)
		}
		edge := ru.NewEdge(utils.GetEdgeId(startID, endID, utils.EdgeIdOptions{Counter: index, Prefix: "L"}, rawEdge.ID))
		edge.IsUserDefinedID = rawEdge.IsUserDefinedID
		edge.Start = startID
		edge.End = endID
		// rawEdge.type ?? 'normal'
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
		// Inert: `Edge.arrowheadStyle` is declared but never read by the
		// renderer. Kept at flowchart parity rather than theme-derived.
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
		// Edge metadata of section 5.3 (e.g. `instruction`): carried to the IR so
		// downstream tooling (dispatch compile) can read it (issue #62).
		edge.Metadata = rawEdge.Metadata

		edges = append(edges, edge)
	}

	// Shape validation runs here, not only at render time. `getDiagnostics()`
	// is built for editor tooling, and tooling calls `parse()`. It never
	// renders, so leaving the only two live diagnostics behind `renderer.draw`
	// meant `mermaid.parse(text)` followed by `getDiagnostics()` always
	// returned `[]`.
	normaliseNodeShapes(nodes.nodes, d)

	// Palette slots. Wired here rather than in `transformData` because this is the path the
	// renderer actually takes -- `getData()` calls `normaliseNodeShapes` directly, so a hook
	// added to `transformData` never runs on a real render.
	//
	// The kind comes from the vertex record, not from the resolved shape: a `connector` and a
	// `task` are both `roundedRect`, so reading it off the shape paints every connector as a
	// task.
	// Declaration order for the container counter. Neither `nodes` nor `subGraphs` carries
	// it: the loops above walk `subGraphs` in REVERSE, and `subGraphs` is itself in
	// completion order, because the grammar reduces a container when it closes and a
	// nested one therefore lands before its parent. Un-reversing would fix the flat case
	// and still get nesting wrong.
	//
	// A pre-order walk of the containment forest recovers source order: roots complete in
	// source order relative to each other, and a parent is always declared before the
	// children it holds. `flowDb` builds its `declarationIndex` the same way.
	containerOrder := js.NewMap[string, int]()
	childContainers := js.NewMap[string, []string]()
	for _, sg := range subGraphs {
		if parent, ok := parentDB.Lookup(sg.ID); ok {
			children, _ := childContainers.Lookup(parent)
			childContainers.Set(parent, append(append(make([]string, 0, len(children)+1), children...), sg.ID))
		}
	}
	nextContainer := 0
	// The walk of upstream recurses. An explicit stack does the same
	// pre-order walk here.
	walkContainers := func(root string) {
		type frame struct {
			children []string
			next     int
		}
		var stack []frame
		enter := func(id string) {
			if containerOrder.Has(id) {
				return // a containment cycle was refused above; do not loop on its remnant
			}
			containerOrder.Set(id, nextContainer)
			nextContainer++
			children, _ := childContainers.Lookup(id)
			stack = append(stack, frame{children: children})
		}
		enter(root)
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.next >= len(top.children) {
				stack = stack[:len(stack)-1]
				continue
			}
			child := top.children[top.next]
			top.next++
			enter(child)
		}
	}
	for _, sg := range subGraphs {
		if !parentDB.Has(sg.ID) {
			walkContainers(sg.ID)
		}
	}

	connectors := d.GetConnectors()
	connectorIDs := map[string]bool{}
	for _, c := range connectors {
		connectorIDs[c.ID] = true
	}
	assignColorSlots(
		config,
		nodes.nodes,
		func(id string) string {
			if connectorIDs[id] {
				return "connector"
			}
			v, ok := d.vertices.Lookup(id)
			if !ok {
				return ""
			}
			return d.deriveVertexKind(v, resolvedType(v))
		},
		containerOrder,
	)

	return &LayoutData{
		LayoutData: &ru.LayoutData{Nodes: nodes.nodes, Edges: edges, Config: config},
		Connectors: connectors,
	}
}

// resolvedType is resolveShapeAlias(v.type) for a vertex: the shape id of
// mermaid, or "" when the type is undefined.
func resolvedType(v *FlowVertex) string {
	if !v.HasType {
		return ""
	}
	return resolveShapeAlias(v.Type)
}
