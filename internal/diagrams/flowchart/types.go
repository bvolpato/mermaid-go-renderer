// Ported from mermaid 12.2.0, diagrams/flowchart/types.ts (MIT).

package flowchart

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Error is an exception that the upstream JavaScript throws: an Error of
// flowDb.ts, or a TypeError of the JavaScript engine for a value that the
// code does not expect.
type Error struct {
	// Name is the JavaScript error class: "Error" or "TypeError".
	Name string
	// Message is error.message.
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(message string) *Error { return &Error{Name: "Error", Message: message} }

func typeError(message string) *Error { return &Error{Name: "TypeError", Message: message} }

// undef is the value of an optional number that is undefined.
var undef = math.NaN()

// FlowText is FlowText: the text of a node, an edge or a subgraph title, and
// the kind of text ("text", "string" or "markdown").
type FlowText struct {
	Text string
	Type string
}

// FlowVertex is FlowVertex. A property that the YAML metadata of a node can
// set holds a dynamic JavaScript value, because upstream stores the YAML
// value without a check of its type: nil is undefined.
type FlowVertex struct {
	Classes      []string
	Dir          string
	HasDir       bool
	DomID        string
	HaveCallback bool
	ID           string
	LabelType    string
	Link         string
	HasLink      bool
	LinkTarget   string
	// HasLinkTarget is false when linkTarget is undefined.
	HasLinkTarget bool
	// Props is nil when props is undefined.
	Props  *js.Obj[any]
	Styles []string
	// Text is a string, or the value of `label` of the metadata.
	Text any
	// Type is the shape. HasType is false when type is undefined.
	Type    string
	HasType bool
	// Icon, Form, Pos, Img and Constraint are values of the metadata.
	Icon       any
	Form       any
	Pos        any
	Img        any
	Constraint any
	// AssetWidth and AssetHeight are Number(w) and Number(h) of the
	// metadata. The Has field is false when the property is undefined.
	AssetWidth     float64
	HasAssetWidth  bool
	AssetHeight    float64
	HasAssetHeight bool
}

// FlowEdge is FlowEdge.
type FlowEdge struct {
	IsUserDefinedID bool
	Start           string
	End             string
	// Interpolate is undefined (nil), the curve of a linkStyle statement,
	// or the value of `curve` of the metadata.
	Interpolate any
	Type        string
	Stroke      string
	// Style is nil when style is undefined.
	Style []string
	// Length is NaN when length is undefined.
	Length    float64
	Text      string
	LabelType string
	Classes   []string
	ID        string
	// Animation and Animate are values of the metadata, or nil.
	Animation any
	Animate   any
}

// FlowSubGraph is FlowSubGraph.
type FlowSubGraph struct {
	Classes []string
	// Dir is the direction. HasDir is false when dir is undefined.
	Dir       string
	HasDir    bool
	ID        string
	LabelType string
	Nodes     []string
	Title     string
	// Metadata is the `@{ ... }` metadata of a `subGraphId@{ ... }`
	// statement, for example { view: 'collapsed' }. It is nil when no
	// statement set it.
	Metadata *js.Obj[any]
}

// FlowLink is FlowLink: the result of destructLink, and with a text and an
// id the value of a link in the grammar.
type FlowLink struct {
	// Length is NaN when length is undefined.
	Length float64
	Stroke string
	Type   string
	// Text is nil when the link has no text.
	Text *FlowText
	// ID is the edge id with the "@" of the source. HasID is false when the
	// link has no id.
	ID    string
	HasID bool
}

// ClickCallback is a callback that setClickFun registers: upstream adds a
// click listener to the element of the node that calls the function.
type ClickCallback struct {
	ID           string
	FunctionName string
	Args         []string
}

// dirStmt is the value of a direction statement in a subgraph:
// { stmt: 'dir', value }.
type dirStmt struct {
	value string
}

// vertexStmt is the value of a vertexStatement of the grammar:
// { stmt, nodes }.
type vertexStmt struct {
	stmt  []string
	nodes []string
}
