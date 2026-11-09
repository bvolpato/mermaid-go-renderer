// Ported from mermaid 12.2.0, diagrams/block/blockTypes.ts (MIT).

package block

import (
	"math"
	"slices"

	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The values of Block.Type.
const (
	typeNA            = "na"
	typeColumnSetting = "column-setting"
	typeEdge          = "edge"
	typeRound         = "round"
	typeBlockArrow    = "block_arrow"
	typeSpace         = "space"
	typeSquare        = "square"
	typeDiamond       = "diamond"
	typeHexagon       = "hexagon"
	typeOdd           = "odd"
	typeLeanRight     = "lean_right"
	typeLeanLeft      = "lean_left"
	typeTrapezoid     = "trapezoid"
	typeInvTrapezoid  = "inv_trapezoid"
	typeRectLeftInv   = "rect_left_inv_arrow"
	typeCircle        = "circle"
	typeEllipse       = "ellipse"
	typeStadium       = "stadium"
	typeSubroutine    = "subroutine"
	typeCylinder      = "cylinder"
	typeGroup         = "group"
	typeDoubleCircle  = "doublecircle"
	typeClassDef      = "classDef"
	typeApplyClass    = "applyClass"
	typeApplyStyles   = "applyStyles"
	typeComposite     = "composite"
)

// undef is the value of an optional number that is undefined.
var undef = math.NaN()

// isUndef reports whether an optional number is undefined.
func isUndef(v float64) bool { return v != v }

// nn is `v ?? d` for an optional number.
func nn(v, d float64) float64 {
	if v != v {
		return d
	}
	return v
}

// Size is the size property of a block: the size of the block and the
// position of its center.
type Size struct {
	Width, Height, X, Y float64
}

// Block is Block of blockTypes.ts. The parser makes a block for each
// statement: a node, an edge, a space, a composite block, a columns
// statement, a classDef statement, a class statement or a style statement.
//
// A JavaScript object can lack a property. An optional number that is
// undefined holds NaN. An optional string that is undefined is "", and an
// optional list that is undefined is nil: an empty list is not nil. Where
// upstream tells undefined from the empty string, a separate field says so.
type Block struct {
	// ID is the id. Two forms of the grammar give a block without an id:
	// the first block of a chain of links (A --> B --> C) and a composite
	// block whose head is a link (block:A --> B). IDUndefined is true then,
	// and ID is "".
	ID          string
	IDUndefined bool
	// Label is the label. HasLabel is false when the label is undefined.
	Label    string
	HasLabel bool
	// Type is the block type, or "" when it is undefined.
	Type string

	// The properties of an edge. Start and End are the ids of the source
	// and the target block. Start is "" when it is undefined.
	Start          string
	End            string
	ArrowTypeEnd   string
	ArrowTypeStart string
	Thickness      string
	Pattern        string

	// Width is the number of columns of a space statement.
	Width float64
	// Children are the blocks inside the block. They are nil for a block
	// without the property.
	Children []*Block
	// Size is nil until the renderer measures the block.
	Size *Size
	// Columns is the number of columns of a composite block: the value of
	// its columns statement, and -1 for automatic columns.
	Columns    float64
	Classes    []string
	Directions []string
	// CSS, StyleClass and StylesStr are the texts of a classDef, a class
	// and a style statement.
	CSS        string
	StyleClass string
	StylesStr  string
	Styles     []string
	// WidthInColumns is the number of columns that the block spans.
	WidthInColumns float64
	// ColorIndex is the palette slot of a composite block, assigned in
	// declaration order by populateBlockDatabase.
	ColorIndex float64

	// Intersect is the intersect function of the drawn node.
	Intersect func(point ru.Point) ru.Point
	// intersectUsesThis says that Intersect reads a property of the object
	// on which it is called (see blockIntersect).
	intersectUsesThis bool

	// document is the children property as the parser sets it: the
	// statements of a composite block, where a statement with links is a
	// list of blocks. populateBlockDatabase replaces it with Children.
	document    []any
	hasDocument bool
	// spread holds the properties "0", "1" and "2" of a composite block
	// whose head is a link: upstream spreads the list of the three blocks
	// of the link into the composite block. Nothing reads them.
	spread []*Block
	// props holds the names of the properties that the parser and the db
	// gave to the JavaScript object, in the order in which they were added.
	// A property in the list can be undefined. Only the dump of the db for
	// the tests reads the list: JSON.stringify prints the properties in
	// this order.
	props []string
}

// newBlock returns a block whose optional numbers are undefined, with the
// properties of the object literal that makes the block upstream.
func newBlock(props ...string) *Block {
	return &Block{Width: undef, Columns: undef, WidthInColumns: undef, ColorIndex: undef, props: props}
}

// define records an assignment to properties of the block: a property that
// the block does not have yet is added after the others.
func (b *Block) define(names ...string) {
	for _, name := range names {
		if !slices.Contains(b.props, name) {
			b.props = append(b.props, name)
		}
	}
}

// clone is a shallow copy of the block: { ...block } and clone(block).
func (b *Block) clone() *Block {
	c := *b
	c.props = slices.Clone(b.props)
	return &c
}

// idString is the id in a string concatenation or a template literal.
func (b *Block) idString() string {
	if b.IDUndefined {
		return "undefined"
	}
	return b.ID
}

// key is the id as a key of the block database.
func (b *Block) key() blockKey {
	return blockKey{id: b.ID, undefined: b.IDUndefined}
}

// hasChildren is the truthiness of block.children.
func (b *Block) hasChildren() bool {
	return b.hasDocument || b.Children != nil
}

// labelEqualsID is block.label === block.id.
func (b *Block) labelEqualsID() bool {
	if !b.HasLabel || b.IDUndefined {
		return !b.HasLabel && b.IDUndefined
	}
	return b.Label == b.ID
}

// ClassDef is ClassDef of blockTypes.ts: a class of a classDef statement.
type ClassDef = render.ClassDef
