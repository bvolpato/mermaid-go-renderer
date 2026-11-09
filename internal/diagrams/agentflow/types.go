// Ported from mermaid 12.2.0, diagrams/agentflow/types.ts (MIT).

package agentflow

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Error is an exception that the upstream JavaScript throws: an Error of
// agentflowDb.ts, a YAMLException of js-yaml for the metadata of a node, or
// a TypeError of the JavaScript engine for a value that the code does not
// expect.
type Error struct {
	// Name is the JavaScript error class: "Error", "TypeError" or
	// "YAMLException".
	Name string
	// Message is error.message.
	Message string
	// Hash is error.hash: the position of the error in the source, in the
	// form of the hash of a jison parse error. It is nil when the error has
	// no position.
	Hash *ErrorHash
	// Mark is error.mark of a YAMLException after the translation to source
	// coordinates. It is nil for the other errors.
	Mark *ErrorMark
}

func (e *Error) Error() string { return e.Message }

func newError(message string) *Error { return &Error{Name: "Error", Message: message} }

func typeError(message string) *Error { return &Error{Name: "TypeError", Message: message} }

// ErrorHash is the hash that positionedShapeError and
// rethrowMetadataYamlError attach to an error. Upstream gives it the form
// of the hash of a jison parse error: an empty text, a null token, the
// line, the location and an empty list of expected tokens.
type ErrorHash struct {
	// Line is the line of the error, counted from 0.
	Line int
	// Loc is the location in the source. Lines start at 1 and columns at 0.
	Loc ErrorLoc
}

// ErrorLoc is hash.loc.
type ErrorLoc struct {
	FirstLine   int
	LastLine    int
	FirstColumn int
	LastColumn  int
}

// ErrorMark is the line and the column of the mark of a YAMLException, both
// counted from 0.
type ErrorMark struct {
	Line   int
	Column int
}

// undef is the value of an optional number that is undefined.
var undef = math.NaN()

// FlowText is FlowText: the text of a node, an edge or a container title,
// and the kind of text ("text", "string" or "markdown").
type FlowText struct {
	Text string
	Type string
}

// VertexKind is the derived semantic kind of a vertex: "tool", "action",
// "input", "refdoc", "decision", "connector" or "task".
type VertexKind = string

// FlowVertex is FlowVertex.
type FlowVertex struct {
	Classes []string
	// Dir is the direction. HasDir is false when dir is undefined.
	Dir          string
	HasDir       bool
	DomID        string
	HaveCallback bool
	ID           string
	LabelType    string
	// Link is the URL. HasLink is false when link is undefined.
	Link    string
	HasLink bool
	// LinkTarget is the target. HasLinkTarget is false when linkTarget is
	// undefined.
	LinkTarget    string
	HasLinkTarget bool
	// Props is nil when props is undefined.
	Props  *js.Obj[any]
	Styles []string
	// Text is the label. HasText is false when text is undefined.
	Text    string
	HasText bool
	// Type is the shape. HasType is false when type is undefined.
	Type    string
	HasType bool
	// Metadata is the `@{ ... }` metadata, or nil.
	Metadata *js.Obj[any]
	// IsConnector marks a node declared with the `connector` keyword.
	IsConnector bool
}

// FlowEdge is FlowEdge.
type FlowEdge struct {
	IsUserDefinedID bool
	Start           string
	End             string
	// Interpolate is undefined (nil), the curve of a linkStyle statement,
	// or the value of `curve` of the metadata.
	Interpolate any
	// Type is the arrow type. It is "" when type is undefined.
	Type string
	// Stroke is "" when stroke is undefined.
	Stroke string
	// EdgeSemantic is the canonical semantic of the operator: "sequence",
	// "reference" or "failure". It is "" when undefined.
	EdgeSemantic string
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
	// Metadata is the metadata of the edge, or nil.
	Metadata *js.Obj[any]
}

// FlowSubGraph is FlowSubGraph.
type FlowSubGraph struct {
	Classes []string
	// Dir is the direction. HasDir is false when dir is undefined.
	Dir    string
	HasDir bool
	// DirValue is the direction when it is not a string, and nil for a
	// string: a container can inherit the `direction` of the config, which
	// can have any type.
	DirValue  any
	ID        string
	LabelType string
	Nodes     []string
	Title     string
	// Type is the container kind: "flow".
	Type string
	// Metadata is the `@{ ... }` metadata of the container, or nil.
	Metadata *js.Obj[any]
}

// dirJS is the JavaScript value of dir. The result is nil when dir is
// undefined.
func (sg *FlowSubGraph) dirJS() any {
	switch {
	case !sg.HasDir:
		return nil
	case sg.DirValue != nil:
		return sg.DirValue
	}
	return sg.Dir
}

// FlowLink is the value of a link in the grammar: the result of
// destructLink with the text and the id of the link.
type FlowLink struct {
	// Length is NaN when length is undefined.
	Length float64
	Stroke string
	Type   string
	// EdgeSemantic is "" when edgeSemantic is undefined.
	EdgeSemantic string
	// Text is nil when the link has no text.
	Text *FlowText
	// ID is the edge id with the "@" of the source. HasID is false when the
	// link has no id.
	ID    string
	HasID bool
}

// ElementPosition is the position of an element in the diagram source.
type ElementPosition struct {
	StartLine   int
	StartColumn int
	EndLine     int
	EndColumn   int
	StartIndex  int
	EndIndex    int
}

// ElementMapping is AgentflowElementMapping: an element and its position.
// Type is "vertex", "edge", "subgraph", "connector" or "attachment".
type ElementMapping struct {
	ID       string
	Type     string
	Position *ElementPosition
}

// SemanticVertex is a vertex of the semantic model.
type SemanticVertex struct {
	ID         string
	Label      string
	HasLabel   bool
	Shape      string
	HasShape   bool
	VertexKind VertexKind
	Metadata   *js.Obj[any]
}

// SemanticEdge is an edge of the semantic model.
type SemanticEdge struct {
	Start        string
	End          string
	ID           string
	Label        string
	HasLabel     bool
	Type         string
	Stroke       string
	EdgeSemantic string
	// Length is NaN when length is undefined.
	Length   float64
	Metadata *js.Obj[any]
}

// SemanticSubGraph is a container of the semantic model.
type SemanticSubGraph struct {
	ID       string
	Type     string
	Title    string
	Nodes    []string
	Metadata *js.Obj[any]
	// Direction is the direction of the container: a string, or the value
	// of the config that the container inherits. It is nil when undefined.
	Direction any
}

// SemanticConnector is a connector of the semantic model.
type SemanticConnector struct {
	ID       string
	Title    string
	HasTitle bool
	Metadata *js.Obj[any]
}

// SemanticModel is AgentflowSemanticModel: the state of the db without the
// presentation fields.
type SemanticModel struct {
	Direction    string
	HasDirection bool
	Vertices     []*SemanticVertex
	Edges        []*SemanticEdge
	SubGraphs    []*SemanticSubGraph
	Connectors   []*SemanticConnector
	Diagnostics  []*Diagnostic
}

// ClickCallback is a callback that setClickFun registers: upstream adds a
// click listener to the element of the node that calls the function.
type ClickCallback struct {
	ID           string
	FunctionName string
	Args         []string
}

// dirStmt is the value of a direction statement in a container:
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
