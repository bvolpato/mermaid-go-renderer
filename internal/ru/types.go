// Ported from mermaid 12.2.0, rendering-util/types.ts (MIT).
//
// JavaScript objects have optional properties. In these structs an optional
// number that is undefined holds NaN (see Undef). This keeps the arithmetic
// of upstream: undefined + 1 is NaN there too. An optional string that is
// undefined is "", and an optional boolean that is undefined is false. Where
// upstream tells undefined from an empty string or from false, a separate
// Has field says so.

// Package ru is the port of mermaid's rendering-util: the unified renderer
// that flowchart, class, state, ER, requirement and other diagrams share.
// It defines the layout data model (types.ts), the registries for shapes,
// cluster shapes and layout algorithms, and the helpers that shapes use.
//
// Shape handlers, edges, clusters and layout algorithms live in sub
// packages and register themselves here.
package ru

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// Undef is the value of an optional number that is undefined.
var Undef = math.NaN()

// IsUndef reports whether an optional number is undefined.
func IsUndef(v float64) bool { return v != v }

// Or is `v || d` for numbers: d when v is undefined, NaN or 0.
func Or(v, d float64) float64 {
	if v != v || v == 0 {
		return d
	}
	return v
}

// NN is `v ?? d` for numbers: d when v is undefined.
func NN(v, d float64) float64 {
	if v != v {
		return d
	}
	return v
}

// NumAttr is handleUndefinedAttr for a number: nil (no attribute) when the
// value is undefined.
func NumAttr(v float64) any {
	if v != v {
		return nil
	}
	return v
}

// StrAttr is handleUndefinedAttr for a string: nil (no attribute) when the
// string is undefined. Use it only where "" stands for undefined.
func StrAttr(v string, defined bool) any {
	if !defined {
		return nil
	}
	return v
}

// Point is a point of an edge or a shape.
type Point struct {
	X, Y float64
}

// Bounds is a rectangle given by its center or corner, as the caller says.
type Bounds struct {
	X, Y, Width, Height float64
}

// GroupTitleRect is the rendered title region of a group.
type GroupTitleRect struct {
	Left, Right, Top, Bottom float64
}

// Node is Node of types.ts (ClusterNode and NonClusterNode together).
type Node struct {
	ID          string
	Label       string
	HasLabel    bool // false when label is undefined
	Description []string
	// Stereotype is the line between the label and the description.
	Stereotype    string
	HasStereotype bool
	ParentID      string
	Position      string
	CSSStyles     []string
	// CSSStylesSet is false when cssStyles is undefined.
	CSSStylesSet      bool
	CSSCompiledStyles []string
	CSSClasses        string
	// CSSClassesSet is false when cssClasses is undefined. Upstream then
	// prints the text "undefined" in class attributes.
	CSSClassesSet bool
	LabelStyle    string
	LabelType     string

	DomID        string
	Dir          string
	HaveCallback bool
	Link         string
	LinkTarget   string
	Tooltip      string
	Padding      float64
	IsGroup      bool
	Width        float64
	Height       float64
	// WrappingWidth and MinWidth are the label wrapping settings.
	WrappingWidth float64
	MinWidth      float64
	SpreadPorts   bool
	LabelBBox     *Bounds
	// GroupTitleRect is the title region of a group-like container.
	GroupTitleRect *GroupTitleRect
	Intersect      func(point Point) Point
	CalcIntersect  func(bounds Bounds, point Point) Point

	Rx, Ry float64

	UseHtmlLabels bool
	CenterLabel   bool

	BackgroundColor string
	BorderColor     string
	BorderStyle     string
	BorderWidth     float64
	LabelTextColor  string
	LabelPaddingX   float64
	LabelPaddingY   float64

	X, Y float64

	Look             string
	Icon             string
	Pos              string
	Img              string
	AssetWidth       float64
	AssetHeight      float64
	DefaultWidth     float64
	ImageAspectRatio float64
	Constraint       string
	Metadata         *js.Obj[any]
	Layer            float64
	Order            float64
	Children         []*Node
	NodeID           string
	Level            float64
	Descr            string
	Type             float64
	Radius           float64
	Taper            float64
	Stroke           string
	ColorIndex       float64

	Shape       string
	IsEdgeLabel bool
	EdgeStart   string
	EdgeEnd     string
	IsDummy     bool

	// Extra holds the properties that one diagram type or one layout adds
	// to a node (for example memberData of a class node). It stands for
	// the index signature of the JavaScript object.
	Extra map[string]any

	// The fields below are the properties that the cluster shapes
	// (internal/ru/clusters) write and that the layouts and positionNode
	// read. types.ts does not list them.

	// Diff is node.diff: how far a cluster grew to fit its title, minus the
	// padding. It is undefined (NaN) until a cluster shape or the dagre
	// layout sets it.
	Diff float64
	// OffsetX and OffsetY are node.offsetX and node.offsetY of a cluster.
	OffsetX float64
	OffsetY float64
	// ClusterNode is node.clusterNode: the dagre layout marks a cluster
	// that it renders as a nested graph. positionNode and the common
	// layout paint read it.
	ClusterNode bool

	// UseHtmlLabelsSet is false when useHtmlLabels is undefined. The
	// collapsedGroup shape reads `node.useHtmlLabels ?? ...`, which tells
	// undefined from false.
	UseHtmlLabelsSet bool
	// LabelStyleUndefined is true when the diagram made the node with no
	// labelStyle. labelHelper then prints no style attribute on the label
	// group (handleUndefinedAttr). Most shapes assign LabelStyle before
	// they make the label, and they do not clear this field: only a shape
	// that does not assign a label style reads it.
	LabelStyleUndefined bool
}

// NewNode returns a node whose optional numbers are undefined.
func NewNode(id string) *Node {
	u := Undef
	return &Node{
		ID: id, Padding: u, Width: u, Height: u, WrappingWidth: u, MinWidth: u, Rx: u, Ry: u,
		BorderWidth: u, LabelPaddingX: u, LabelPaddingY: u, X: u, Y: u, AssetWidth: u, AssetHeight: u,
		DefaultWidth: u, ImageAspectRatio: u, Layer: u, Order: u, Level: u, Type: u, Radius: u, Taper: u,
		ColorIndex: u,
		Diff:       u, OffsetX: u, OffsetY: u,
	}
}

// Get reads an extra property. It returns nil when the property is unset.
func (n *Node) Get(key string) any { return n.Extra[key] }

// Set writes an extra property.
func (n *Node) Set(key string, v any) {
	if n.Extra == nil {
		n.Extra = map[string]any{}
	}
	n.Extra[key] = v
}

// TerminalLabelSize is the measured size of a terminal label of an edge.
type TerminalLabelSize struct {
	Width, Height float64
}

// Edge is Edge of types.ts.
type Edge struct {
	ID                 string
	Label              string
	HasLabel           bool
	Classes            string
	Style              []string
	Animate            bool
	HasAnimate         bool
	Animation          string
	Metadata           *js.Obj[any]
	Arrowhead          string
	ArrowheadStyle     string
	ArrowTypeEnd       string
	ArrowTypeStart     string
	CSSCompiledStyles  []string
	DefaultInterpolate string
	End                string
	Interpolate        string
	LabelType          string
	Length             float64
	Start              string
	Stroke             string
	Text               string
	Type               string
	StartLabelRight    string
	EndLabelLeft       string
	// TerminalLabelSizes and TerminalLabelCenters are keyed by startLeft,
	// startRight, endLeft and endRight.
	TerminalLabelSizes   map[string]TerminalLabelSize
	TerminalLabelCenters map[string]Point
	Curve                string
	Labelpos             string
	LabelStyle           []string
	Minlen               float64
	Pattern              string
	Thickness            string
	Look                 string
	IsUserDefinedID      bool
	ShowPoints           bool
	Width                float64
	Height               float64
	X, Y                 float64
	Points               []Point
	ParentID             string
	Dir                  string
	Source               string
	Target               string
	Depth                float64
	IsLabelEdge          bool
	LabelNodeID          string
	IsLayoutOnly         bool

	// Extra holds the properties that one diagram type or one layout adds.
	Extra map[string]any

	// The fields below are the properties that rendering-elements/edges.js
	// reads and that types.ts does not list (internal/ru/edges).

	// StartLabelLeft and EndLabelRight are the other two terminal labels.
	StartLabelLeft string
	EndLabelRight  string
	// ToCluster and FromCluster are the ids of the clusters at whose border
	// the dagre layout cuts the edge.
	ToCluster   string
	FromCluster string
	// StyleUndefined is true when style is undefined. Upstream then prints
	// the text "undefined" in the style attribute of the path. A style that
	// is a string is a Style with one element.
	StyleUndefined bool
	// LabelStyleText is labelStyle when it is a string: insertEdgeLabel
	// replaces the array with a string. LabelStyleIsText says that it is.
	LabelStyleText   string
	LabelStyleIsText bool
	// CSSStyles is cssStyles, which styles2String reads from an edge too.
	CSSStyles []string
	// ClassesUndefined is true when classes is undefined. The ELK layout
	// (internal/ru/layout/elk) then sets the classes "flowchart-link
	// LS_<start> LE_<end>". An empty Classes with a false ClassesUndefined
	// is the empty string, which the layout keeps.
	ClassesUndefined bool
}

// NewEdge returns an edge whose optional numbers are undefined.
func NewEdge(id string) *Edge {
	u := Undef
	return &Edge{ID: id, Length: u, Minlen: u, Width: u, Height: u, X: u, Y: u, Depth: u}
}

// Get reads an extra property. It returns nil when the property is unset.
func (e *Edge) Get(key string) any { return e.Extra[key] }

// Set writes an extra property.
func (e *Edge) Set(key string, v any) {
	if e.Extra == nil {
		e.Extra = map[string]any{}
	}
	e.Extra[key] = v
}

// LayoutData is LayoutData of types.ts: what a diagram db hands to render.
type LayoutData struct {
	Nodes  []*Node
	Edges  []*Edge
	Config *js.Obj[any]
	// DiagramID is the id of the svg element.
	DiagramID string

	// The fields below are the additional properties that the diagram
	// renderers set before they call Render.
	Type            string
	LayoutAlgorithm string
	Direction       string
	NodeSpacing     float64
	RankSpacing     float64
	Markers         []string

	// Extra holds other additional properties.
	Extra map[string]any
}

// ShapeRenderOptions is ShapeRenderOptions of types.ts.
type ShapeRenderOptions struct {
	Config *js.Obj[any]
	// Dir is the direction of the diagram. Some shapes use it.
	Dir     string
	Padding float64
}

// ShapeHandler draws one node shape into parent and returns the group of
// the shape. It stores the size and the intersection functions in the node.
type ShapeHandler func(ctx *render.Context, parent *dom.Node, node *Node, options *ShapeRenderOptions) (*dom.Node, error)

// ShapeDef is ShapeDefinition of shapes.ts, without the handler function.
type ShapeDef struct {
	SemanticName    string
	Name            string
	ShortName       string
	Aliases         []string
	InternalAliases []string
	// Handler is the name under which the handler is registered.
	Handler string
}
