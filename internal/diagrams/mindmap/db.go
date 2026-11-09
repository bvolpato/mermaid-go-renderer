// Ported from mermaid 12.2.0, diagrams/mindmap/mindmapDb.ts and
// mindmapTypes.ts (MIT).

package mindmap

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// maxSections is MAX_SECTIONS of svgDraw.ts.
const maxSections = 12

// The values of nodeType.
const (
	nodeTypeDefault     = 0
	nodeTypeNoBorder    = 0
	nodeTypeRoundedRect = 1
	nodeTypeRect        = 2
	nodeTypeCircle      = 3
	nodeTypeCloud       = 4
	nodeTypeBang        = 5
	nodeTypeHexagon     = 6
)

// Node is MindmapNode of mindmapTypes.ts. An optional number that is
// undefined is NaN (ru.Undef).
type Node struct {
	ID       int
	NodeID   string
	Level    int
	Descr    string
	Type     int
	Children []*Node
	// Width and Padding are config values: numbers, unless the config has
	// another type there.
	Width   any
	Padding any
	// Section is undefined for the root and before getData.
	Section float64
	Height  float64
	// Class and Icon are set by decorateNode. HasClass and HasIcon are
	// false while the property is undefined.
	Class    string
	HasClass bool
	Icon     string
	HasIcon  bool
	X, Y     float64
	IsRoot   bool
}

// DB is MindmapDB.
type DB struct {
	ctx   *render.Context
	nodes []*Node
	count int
	// baseLevel is undefined until the first node.
	baseLevel    int
	hasBaseLevel bool
}

// NewDB returns an empty db: new MindmapDB().
func NewDB(ctx *render.Context) *DB {
	return &DB{ctx: ctx}
}

// Clear is clear().
func (d *DB) Clear() {
	d.nodes = nil
	d.count = 0
	d.baseLevel = 0
	d.hasBaseLevel = false
}

// GetParent is getParent(level): the last node with a smaller level.
func (d *DB) GetParent(level int) *Node {
	for i := len(d.nodes) - 1; i >= 0; i-- {
		if d.nodes[i].Level < level {
			return d.nodes[i]
		}
	}
	return nil
}

// GetMindmap is getMindmap(): the root node, or nil.
func (d *DB) GetMindmap() *Node {
	if len(d.nodes) > 0 {
		return d.nodes[0]
	}
	return nil
}

// mindmapConfig is `conf.mindmap?.<key> ?? defaultConfig.mindmap.<key>`.
func mindmapConfig(ctx *render.Context, conf *js.Obj[any], key string) any {
	if v := js.Path(conf, "mindmap", key); !js.IsNullish(v) {
		return v
	}
	return js.Path(ctx.ConfigState.DefaultSection("mindmap"), key)
}

// AddNode is addNode(level, id, descr, type).
func (d *DB) AddNode(level int, id, descr string, typ int) error {
	isRoot := false

	if len(d.nodes) == 0 {
		d.baseLevel = level
		d.hasBaseLevel = true
		level = 0
		isRoot = true
	} else if d.hasBaseLevel {
		level = level - d.baseLevel
		isRoot = false
	}

	conf := d.ctx.Config
	padding := mindmapConfig(d.ctx, conf, "padding")

	switch typ {
	case nodeTypeRoundedRect, nodeTypeRect, nodeTypeHexagon:
		// padding *= 2
		padding = float64(js.ToNumber(padding) * 2)
	}

	node := &Node{
		ID:      d.count,
		NodeID:  d.ctx.SanitizeText(id),
		Level:   level,
		Descr:   d.ctx.SanitizeText(descr),
		Type:    typ,
		Width:   mindmapConfig(d.ctx, conf, "maxNodeWidth"),
		Padding: padding,
		Section: ru.Undef,
		Height:  ru.Undef,
		X:       ru.Undef,
		Y:       ru.Undef,
		IsRoot:  isRoot,
	}
	d.count++

	parent := d.GetParent(level)
	if parent != nil {
		parent.Children = append(parent.Children, node)
		d.nodes = append(d.nodes, node)
	} else {
		if isRoot {
			d.nodes = append(d.nodes, node)
		} else {
			return &Error{Message: `There can be only one root. No parent could be found for ("` + node.Descr + `")`}
		}
	}
	return nil
}

// GetType is getType(startStr, endStr).
func (d *DB) GetType(startStr, endStr string) int {
	switch startStr {
	case "[":
		return nodeTypeRect
	case "(":
		if endStr == ")" {
			return nodeTypeRoundedRect
		}
		return nodeTypeCloud
	case "((":
		return nodeTypeCircle
	case ")":
		return nodeTypeCloud
	case "))":
		return nodeTypeBang
	case "{{":
		return nodeTypeHexagon
	default:
		return nodeTypeDefault
	}
}

// Decoration is the argument of decorateNode.
type Decoration struct {
	Class string
	Icon  string
}

// DecorateNode is decorateNode(decoration).
func (d *DB) DecorateNode(decoration Decoration) error {
	if len(d.nodes) == 0 {
		// node is undefined: the assignment throws.
		if decoration.Icon != "" {
			return &Error{Name: "TypeError", Message: "Cannot set properties of undefined (setting 'icon')"}
		}
		if decoration.Class != "" {
			return &Error{Name: "TypeError", Message: "Cannot set properties of undefined (setting 'class')"}
		}
		return nil
	}
	node := d.nodes[len(d.nodes)-1]
	if decoration.Icon != "" {
		node.Icon = d.ctx.SanitizeText(decoration.Icon)
		node.HasIcon = true
	}
	if decoration.Class != "" {
		node.Class = d.ctx.SanitizeText(decoration.Class)
		node.HasClass = true
	}
	return nil
}

// Type2Str is type2Str(type).
func (d *DB) Type2Str(typ int) string {
	switch typ {
	case nodeTypeDefault:
		return "no-border"
	case nodeTypeRect:
		return "rect"
	case nodeTypeRoundedRect:
		return "rounded-rect"
	case nodeTypeCircle:
		return "circle"
	case nodeTypeCloud:
		return "cloud"
	case nodeTypeBang:
		return "bang"
	case nodeTypeHexagon:
		return "hexgon" // cspell: disable-line
	default:
		return "no-border"
	}
}

// AssignSections is assignSections(node, sectionNumber, depth,
// colorByDepth): it assigns color sections by top-level branch (default) or
// structural depth. sectionNumber is undefined (NaN) for the root.
func (d *DB) AssignSections(node *Node, sectionNumber float64, depth int, colorByDepth bool) {
	// For root node, section should be undefined (not -1)
	if node.Level == 0 {
		node.Section = ru.Undef
	} else {
		// For non-root nodes, assign the section number
		if colorByDepth {
			node.Section = float64((depth - 1) % (maxSections - 1))
		} else {
			node.Section = sectionNumber
		}
	}
	// For root node's children, assign section numbers based on their index
	// For other nodes, inherit parent's section number
	for index, child := range node.Children {
		childSectionNumber := sectionNumber
		if node.Level == 0 {
			childSectionNumber = float64(index % (maxSections - 1))
		}
		d.AssignSections(child, childSectionNumber, depth+1, colorByDepth)
	}
}

// numOrUndef converts a config value to an optional number of ru.Node.
func numOrUndef(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	if js.IsNullish(v) {
		return ru.Undef
	}
	return js.ToNumber(v)
}

// FlattenNodes is flattenNodes(node, processedNodes): it converts the
// mindmap tree structure to a flat array of nodes.
func (d *DB) FlattenNodes(node *Node, processedNodes *[]*ru.Node) {
	conf := d.ctx.Config
	// Build CSS classes for the node
	cssClasses := []string{"mindmap-node"}

	if node.IsRoot {
		// Root node gets special classes
		cssClasses = append(cssClasses, "section-root", "section--1")
	} else if !ru.IsUndef(node.Section) {
		// Child nodes get section class based on their section number
		cssClasses = append(cssClasses, "section-"+js.Str(node.Section))
	}

	// Add any custom classes from the node
	if node.Class != "" {
		cssClasses = append(cssClasses, node.Class)
	}

	classes := strings.Join(cssClasses, " ")

	// Map mindmap node type to valid shape name
	getShapeFromType := func(typ int) string {
		// conf.theme?.toLowerCase() ?? ''
		theme := ""
		if t := conf.Get("theme"); !js.IsNullish(t) {
			theme = strings.ToLower(js.ToString(t))
		}
		isReduxTheme := strings.Contains(theme, "redux")
		switch typ {
		case nodeTypeCircle:
			return "mindmapCircle"
		case nodeTypeRect:
			return "rect"
		case nodeTypeRoundedRect:
			return "rounded"
		case nodeTypeCloud:
			return "cloud"
		case nodeTypeBang:
			return "bang"
		case nodeTypeHexagon:
			return "hexagon"
		case nodeTypeDefault:
			if isReduxTheme {
				return "rounded"
			}
			return "defaultMindmapNode"
		default:
			return "rect"
		}
	}

	id := strconv.Itoa(node.ID)
	processedNode := ru.NewNode(id)
	processedNode.DomID = "node_" + id
	processedNode.Label = node.Descr
	processedNode.HasLabel = true
	processedNode.LabelType = "markdown"
	processedNode.IsGroup = false
	processedNode.Shape = getShapeFromType(node.Type)
	processedNode.Width = numOrUndef(node.Width)
	processedNode.Height = ru.NN(node.Height, 0)
	processedNode.Padding = numOrUndef(node.Padding)
	processedNode.CSSClasses = classes
	processedNode.CSSClassesSet = true
	processedNode.CSSStyles = []string{}
	processedNode.CSSStylesSet = true
	// Mindmap styles don't set text-anchor: middle, so SVG-text labels must be
	// offset by half their width to be centred (issue #8387)
	processedNode.CenterLabel = true
	if look := conf.Get("look"); !js.IsNullish(look) {
		processedNode.Look = js.ToString(look)
	}
	processedNode.Icon = node.Icon
	processedNode.X = node.X
	processedNode.Y = node.Y
	// Mindmap-specific properties
	processedNode.Level = float64(node.Level)
	processedNode.NodeID = node.NodeID
	processedNode.Type = float64(node.Type)
	if !ru.IsUndef(node.Section) {
		processedNode.Set("section", node.Section)
	}
	// The node is made with no labelStyle.
	processedNode.LabelStyleUndefined = true

	*processedNodes = append(*processedNodes, processedNode)

	// Recursively process children
	for _, child := range node.Children {
		d.FlattenNodes(child, processedNodes)
	}
}

// GenerateEdges is generateEdges(node, edges): it generates the edges from
// the parent-child relationships in the mindmap tree.
func (d *DB) GenerateEdges(node *Node, edges *[]*ru.Edge) {
	conf := d.ctx.Config
	for _, child := range node.Children {
		// Build CSS classes for the edge
		edgeClasses := "edge"

		// Add section-specific classes based on the child's section
		if !ru.IsUndef(child.Section) {
			edgeClasses += " section-edge-" + js.Str(child.Section)
		}

		// Add depth class based on the parent's level + 1 (depth of the edge)
		edgeDepth := node.Level + 1
		edgeClasses += " edge-depth-" + strconv.Itoa(edgeDepth)

		edge := ru.NewEdge("edge_" + strconv.Itoa(node.ID) + "_" + strconv.Itoa(child.ID))
		edge.Start = strconv.Itoa(node.ID)
		edge.End = strconv.Itoa(child.ID)
		edge.Type = "normal"
		edge.Curve = "basis"
		edge.Thickness = "normal"
		if look := conf.Get("look"); !js.IsNullish(look) {
			edge.Look = js.ToString(look)
		}
		edge.Classes = edgeClasses
		// Store mindmap-specific data
		edge.Depth = float64(node.Level)
		if !ru.IsUndef(child.Section) {
			edge.Set("section", child.Section)
		}
		// The edge has no style and no label.
		edge.StyleUndefined = true

		*edges = append(*edges, edge)

		// Recursively process child edges
		d.GenerateEdges(child, edges)
	}
}

// placeholderUUID stands for v4() of the package uuid in the diagramId of
// getData. The id is random upstream, and the renderer replaces it with the
// id of the svg before anything reads it.
const placeholderUUID = "00000000-0000-4000-8000-000000000000"

// GetData is getData(): the structured data for layout algorithms.
func (d *DB) GetData() *ru.LayoutData {
	mindmapRoot := d.GetMindmap()
	// getConfig() returns a copy of the config, and getData changes it.
	config := js.Clone(d.ctx.Config).(*js.Obj[any])

	userDefinedConfig := d.ctx.ConfigState.GetUserDefinedConfig()
	hasUserDefinedLayout := userDefinedConfig.Get("layout") != nil

	finalConfig := config
	if !hasUserDefinedLayout {
		finalConfig.Set("layout", "cose-bilkent")
	}

	if mindmapRoot == nil {
		return &ru.LayoutData{
			Nodes:       []*ru.Node{},
			Edges:       []*ru.Edge{},
			Config:      finalConfig,
			NodeSpacing: ru.Undef,
			RankSpacing: ru.Undef,
		}
	}

	// Assign section numbers to all nodes based on their position relative to root
	d.AssignSections(mindmapRoot, ru.Undef, 0, js.StrictEquals(js.Path(config, "mindmap", "colorBy"), "depth"))

	// Convert tree structure to flat arrays
	processedNodes := []*ru.Node{}
	processedEdges := []*ru.Edge{}

	d.FlattenNodes(mindmapRoot, &processedNodes)
	d.GenerateEdges(mindmapRoot, &processedEdges)

	// Create shapes map for ELK compatibility
	shapes := js.NewObj[any]()
	for _, node := range processedNodes {
		shape := js.NewObj[any]()
		shape.Set("shape", node.Shape)
		shape.Set("width", optNum(node.Width))
		shape.Set("height", optNum(node.Height))
		shape.Set("padding", optNum(node.Padding))
		shapes.Set(node.ID, shape)
	}

	return &ru.LayoutData{
		Nodes:  processedNodes,
		Edges:  processedEdges,
		Config: finalConfig,
		// Properties required by dagre layout algorithm
		Markers:     []string{"point"}, // Mindmaps don't use markers
		Direction:   "TB",              // Top-to-bottom direction for mindmaps
		NodeSpacing: 50,                // Default spacing between nodes
		RankSpacing: 50,                // Default spacing between ranks
		// Additional properties that layout algorithms might expect
		Type:      "mindmap",
		DiagramID: "mindmap-" + placeholderUUID,
		Extra: map[string]any{
			// Store the root node for mindmap-specific layout algorithms
			"rootNode": mindmapRoot,
			// Add shapes for ELK compatibility
			"shapes": shapes,
		},
	}
}

// optNum is an optional number as a dynamic value: nil when undefined.
func optNum(v float64) any {
	if ru.IsUndef(v) {
		return nil
	}
	return v
}

// Error is an error that the db or the parser of upstream throws.
type Error struct {
	// Name is the name of the JavaScript error. Empty for Error.
	Name    string
	Message string
}

func (e *Error) Error() string { return e.Message }
