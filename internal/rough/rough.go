// Ported from roughjs 4.6.6, roughjs/bin/core.d.ts and
// roughjs/bin/geometry.d.ts (MIT).

// Package rough is a port of roughjs 4.6.6, the library that mermaid uses to
// draw node shapes, both for the handDrawn look and, with roughness 0, for
// ordinary shapes.
//
// The port produces the same operations and the same strings as the
// JavaScript code. The entry point is a Generator. A shape method (Rectangle,
// Path and the others) returns a Drawable, and Draw returns the path elements
// that RoughSVG.draw appends to its <g> element.
//
// Options are plain JavaScript objects (*js.Obj[any]), because mermaid builds
// them with Object.assign and passes values of dynamic type. A numeric option
// is a float64 or an int. An absent key keeps the default. A key with a nil
// value is undefined, as it is in JavaScript.
//
// Random numbers come from Config.Random when the seed option is 0, in the
// order and the number in which roughjs calls Math.random. With another seed
// they come from the port of the Random class of roughjs.
//
// The port calls the wrappers of package js for Math.sin, Math.cos,
// Math.tan, Math.atan, Math.asin and Math.pow. The output has the numbers of
// the browser only when the wrappers return the values of the browser.
//
// roughjs throws for some inputs, and for other inputs it does not
// terminate. The shape methods that can fail return an *Error for them.
package rough

import "github.com/bvolpato/mermaid-go-renderer/internal/js"

// Point is the roughjs Point: [x, y].
type Point [2]float64

// OpType is the type of one drawing operation.
type OpType string

// The operations that roughjs emits.
const (
	OpMove     OpType = "move"
	OpBcurveTo OpType = "bcurveTo"
	OpLineTo   OpType = "lineTo"
)

// Op is one drawing operation. Data has 2 numbers for move and lineTo, and 6
// numbers for bcurveTo.
type Op struct {
	Op   OpType
	Data []float64
}

// OpSetType is the type of a set of operations.
type OpSetType string

// The set types that roughjs emits.
const (
	SetPath       OpSetType = "path"
	SetFillPath   OpSetType = "fillPath"
	SetFillSketch OpSetType = "fillSketch"
)

// OpSet is one set of operations. RoughSVG.draw makes one <path> for each set.
type OpSet struct {
	Type OpSetType
	Ops  []Op
}

// Drawable is the result of a shape method.
type Drawable struct {
	Shape   string
	Options *ResolvedOptions
	Sets    []OpSet
}

// PathInfo is one entry of the result of ToPaths. Stroke, StrokeWidth and
// Fill are dynamic JavaScript values, because roughjs copies them from the
// options without a conversion.
type PathInfo struct {
	D           string
	Stroke      any
	StrokeWidth any
	Fill        any
}

// Attr is one attribute of a path element.
type Attr struct {
	Name  string
	Value string
}

// PathElement is one <path> element that RoughSVG.draw appends to its <g>
// element. Attrs has the attributes in the order that roughjs sets them.
type PathElement struct {
	Attrs []Attr
}

// Options is an options object of roughjs. A nil value means that the caller
// gives no options.
type Options = *js.Obj[any]

// Config is the configuration of a Generator.
type Config struct {
	// Options are the default options (config.options in roughjs).
	Options Options
	// Random replaces Math.random. roughjs calls Math.random when the seed
	// option is 0, and for some fills with any seed. When Random is nil, the
	// generator uses a random source that is not deterministic.
	Random func() float64
}

// Error reports that roughjs fails for the given input. Name is the class of
// the JavaScript error ("Error", "TypeError" or "RangeError"). Name is "Hang"
// when the JavaScript code does not terminate, or needs more steps than a
// browser can run. Name is "InternalError" for an unexpected failure of the
// port.
type Error struct {
	Name    string
	Message string
}

func (e *Error) Error() string {
	return "rough: " + e.Name + ": " + e.Message
}
