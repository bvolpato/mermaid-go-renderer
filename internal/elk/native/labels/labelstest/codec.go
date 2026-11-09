// Package labelstest is the test support of the package labels. It gives the
// test harness (nativetest) a codec for the property END_LABELS, whose values
// are maps from a port to a label cell. A test that reads or writes states
// with that property links this package with a blank import.
//
// The stored form of a value is the form of the dump of elkjs
// (tools/reference/gen-elk-phases.mjs):
//
//	{"$map": [[{"$p": port}, {"$o": i, "c": "alg.common.nodespacing.cellsystem.LabelCell", "f": {
//	    "padding": {"$sp": [...]},
//	    "cellRectangle": {"$o": i, "c": "core.math.ElkRectangle", "f": {"x_0", "y_0", "width_0", "height"}},
//	    "horizontalAlignment": "LEFT", "verticalAlignment": "TOP",
//	    "labels": {"$list": [{"$o": i, "c": "alg.layered.graph.LGraphAdapters$LLabelAdapter", "f": {"element": {"$l": label}}}]},
//	    "minimumContentAreaSize": {"$v": [x, y]}, "gap": 0, "horizontalLayoutMode": true}}]]}
package labelstest

import (
	"fmt"
	"math"
	"slices"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/labels"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// EndLabelsID is the id of the property END_LABELS.
const EndLabelsID = "endLabels"

const (
	classLabelCell     = "alg.common.nodespacing.cellsystem.LabelCell"
	classElkRectangle  = "core.math.ElkRectangle"
	classLLabelAdapter = "alg.layered.graph.LGraphAdapters$LLabelAdapter"
)

func init() {
	nativetest.RegisterCodec(EndLabelsID, nativetest.Codec{Decode: decodeEndLabels, Encode: encodeEndLabels})
}

// ObjectNumber returns the number ("$o") of the next Java object that a
// codec writes in the dump of d. The dump of elkjs numbers the objects of a
// state in the order in which it finds them, from 0. The numbers of this
// function are equal to those of elkjs when each object of the state gets
// its number here: the label cells of this package, and the opaque values of
// the properties of RegisterOpaque.
//
// The harness owns the counter (nativetest.Dumper.NewObjectID), so the
// codecs of all packages and the opaque values share it.
func ObjectNumber(d *nativetest.Dumper) float64 {
	return float64(d.NewObjectID())
}

// orderKey is the member that nativetest.ParseState adds to an object: the
// names of the members in the order of the JSON text.
const orderKey = "\x00order"

// fail stops a decode or an encode. Build and Dump of the harness turn the
// panic into an error.
func fail(format string, args ...any) {
	panic(fmt.Sprintf("labelstest: "+format, args...))
}

func object(v any, class string) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		fail("%v is not an object of the class %s", v, class)
	}
	if c, _ := m["c"].(string); c != class {
		fail("an object of the class %q where the class %s is expected", m["c"], class)
	}
	f, ok := m["f"].(map[string]any)
	if !ok {
		fail("an object of the class %s without fields", class)
	}
	return f
}

// tagged returns the value of an object with one member, for example the
// list of {"$list": [...]}.
func tagged(v any, key string) any {
	m, ok := v.(map[string]any)
	if !ok || len(m) != 1 {
		fail("%v is not a %s value", v, key)
	}
	x, ok := m[key]
	if !ok {
		fail("%v is not a %s value", v, key)
	}
	return x
}

func number(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case map[string]any:
		if s, ok := x["$f"].(string); ok && len(x) == 1 {
			switch s {
			case "-0":
				return math.Copysign(0, -1)
			case "NaN":
				return math.NaN()
			case "Infinity":
				return math.Inf(1)
			case "-Infinity":
				return math.Inf(-1)
			}
		}
	}
	fail("%v is not a number", v)
	return 0
}

func numbers(v any, n int) []float64 {
	list, ok := v.([]any)
	if !ok || len(list) != n {
		fail("%v is not a list of %d numbers", v, n)
	}
	out := make([]float64, n)
	for i, x := range list {
		out[i] = number(x)
	}
	return out
}

func index(v any, key string, count int) int {
	f, ok := tagged(v, key).(float64)
	i := int(f)
	if !ok || float64(i) != f || i < 0 || i >= count {
		fail("%v does not name an element of the state", v)
	}
	return i
}

func decodeEndLabels(raw any, b *nativetest.Built) (any, error) {
	rows, ok := tagged(raw, "$map").([]any)
	if !ok {
		fail("%v is not a map", raw)
	}
	cells := gwt.NewMap[*lgraph.LPort, *cellsystem.LabelCell]()
	for _, rv := range rows {
		row, ok := rv.([]any)
		if !ok || len(row) != 2 {
			fail("%v is not an entry of a map", rv)
		}
		cells.Put(b.Ports[index(row[0], "$p", len(b.Ports))], decodeLabelCell(row[1], b))
	}
	return cells, nil
}

// labelCellFields has the fields of the JavaScript object of a label cell:
// those that the constructor of the Java class sets. The fields
// contributesToMinimumWidth and contributesToMinimumHeight are in the stored
// form only for a cell that got them, and no end label cell does.
var labelCellFields = []string{"padding", "cellRectangle", "horizontalAlignment", "verticalAlignment", "labels", "minimumContentAreaSize", "gap", "horizontalLayoutMode"}

func decodeLabelCell(v any, b *nativetest.Built) *cellsystem.LabelCell {
	f := object(v, classLabelCell)
	for _, key := range labelCellFields {
		if _, ok := f[key]; !ok {
			fail("a label cell without the field %s", key)
		}
	}
	for key := range f {
		if key != orderKey && !slices.Contains(labelCellFields, key) {
			fail("a label cell with the field %s, which the codec does not know", key)
		}
	}

	horizontalLayoutMode, ok := f["horizontalLayoutMode"].(bool)
	if !ok {
		fail("the field horizontalLayoutMode of a label cell is %v", f["horizontalLayoutMode"])
	}
	cell := cellsystem.NewLabelCellMode(number(f["gap"]), horizontalLayoutMode)

	name, _ := f["horizontalAlignment"].(string)
	horizontalAlignment, ok := prop.ParseEnum[cellsystem.HorizontalLabelAlignment](name)
	if !ok {
		fail("the horizontal alignment %v of a label cell", f["horizontalAlignment"])
	}
	cell.SetHorizontalAlignment(horizontalAlignment)
	name, _ = f["verticalAlignment"].(string)
	verticalAlignment, ok := prop.ParseEnum[cellsystem.VerticalLabelAlignment](name)
	if !ok {
		fail("the vertical alignment %v of a label cell", f["verticalAlignment"])
	}
	cell.SetVerticalAlignment(verticalAlignment)

	padding := numbers(tagged(f["padding"], "$sp"), 4)
	cell.Padding().Set(padding[0], padding[1], padding[2], padding[3])

	rect := object(f["cellRectangle"], classElkRectangle)
	cell.CellRectangle().SetRect(number(rect["x_0"]), number(rect["y_0"]), number(rect["width_0"]), number(rect["height"]))

	size := numbers(tagged(f["minimumContentAreaSize"], "$v"), 2)
	cell.MinimumContentAreaSize().SetXY(size[0], size[1])

	list, ok := tagged(f["labels"], "$list").([]any)
	if !ok {
		fail("the labels %v of a label cell", f["labels"])
	}
	cellLabels := make([]adapters.LabelAdapter, 0, len(list))
	for _, lv := range list {
		adapter := object(lv, classLLabelAdapter)
		cellLabels = append(cellLabels, labels.AdaptLabel(b.Labels[index(adapter["element"], "$l", len(b.Labels))]))
	}
	// SetLabels keeps the minimum size of the cell.
	cell.SetLabels(cellLabels)
	return cell
}

func encodeEndLabels(v any, d *nativetest.Dumper) any {
	cells, ok := v.(*labels.EndLabelCells)
	if !ok {
		fail("the property %s has a value of the type %T", EndLabelsID, v)
	}
	rows := []any{}
	cells.Each(func(port *lgraph.LPort, cell *cellsystem.LabelCell) bool {
		// The key first: the dump finds the port before the labels.
		key := d.Value(port)
		rows = append(rows, []any{key, encodeLabelCell(cell, d)})
		return true
	})
	return map[string]any{"$map": rows}
}

// encodeLabelCell numbers the objects in the order of the fields of the
// JavaScript objects: the cell, its rectangle, then the label adapters.
func encodeLabelCell(cell *cellsystem.LabelCell, d *nativetest.Dumper) any {
	if cell.IsContributingToMinimumWidth() || cell.IsContributingToMinimumHeight() {
		fail("a label cell that contributes to the minimum size of a container: the codec does not know the fields")
	}
	id := ObjectNumber(d)
	rect := cell.CellRectangle()
	f := map[string]any{
		"padding": d.Value(cell.Padding()),
		"cellRectangle": map[string]any{"$o": ObjectNumber(d), "c": classElkRectangle, "f": map[string]any{
			"x_0":     d.Value(rect.X),
			"y_0":     d.Value(rect.Y),
			"width_0": d.Value(rect.Width),
			"height":  d.Value(rect.Height),
		}},
		"horizontalAlignment": d.Value(cell.HorizontalAlignment()),
		"verticalAlignment":   d.Value(cell.VerticalAlignment()),
	}
	list := []any{}
	for _, label := range cell.Labels() {
		element, ok := label.Holder().(*lgraph.LLabel)
		if !ok {
			fail("a label cell with a label adapter of the type %T", label)
		}
		list = append(list, map[string]any{"$o": ObjectNumber(d), "c": classLLabelAdapter, "f": map[string]any{"element": d.Value(element)}})
	}
	f["labels"] = map[string]any{"$list": list}
	f["minimumContentAreaSize"] = d.Value(cell.MinimumContentAreaSize())
	f["gap"] = d.Value(cell.Gap())
	f["horizontalLayoutMode"] = cell.IsHorizontalLayoutMode()
	return map[string]any{"$o": id, "c": classLabelCell, "f": f}
}

// RegisterOpaque registers a codec for properties whose values the harness
// keeps in their stored form (nativetest.Opaque), because no Go package has
// their classes. The codec writes a value back with new numbers for its Java
// objects: the numbers of ObjectNumber, in the order of the dump.
//
// Without it the harness writes the numbers of the state before a step. They
// are wrong when the step adds or removes an object, of any property, that
// the dump finds before the value.
//
// The codec does not move the fields of an object to another reference. So a
// state must keep the element that has the first reference to each object.
//
// TODO(core): move to nativetest (number the objects of each opaque value in
// Dump).
func RegisterOpaque(propertyIDs ...string) {
	for _, id := range propertyIDs {
		nativetest.RegisterCodec(id, nativetest.Codec{
			Decode: func(raw any, b *nativetest.Built) (any, error) {
				return &opaqueValue{raw: raw, b: b}, nil
			},
			Encode: func(v any, d *nativetest.Dumper) any {
				o, ok := v.(*opaqueValue)
				if !ok {
					fail("the property %s has a value of the type %T", id, v)
				}
				return o.encode(o.raw, d)
			},
		})
	}
}

// opaqueValue is a stored value with the graph that its element numbers
// name.
type opaqueValue struct {
	raw any
	b   *nativetest.Built
}

// encode writes the stored form again. It visits the members of an object
// in the order of the JSON text, as the dump of elkjs visits the fields of a
// Java object.
func (o *opaqueValue) encode(v any, d *nativetest.Dumper) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = o.encode(e, d)
		}
		return out
	case map[string]any:
		if len(x) == 1 {
			for key, ref := range x {
				i, isIndex := ref.(float64)
				switch {
				case key == "$g" && isIndex:
					return d.Value(o.b.Graphs[int(i)])
				case key == "$n" && isIndex:
					return d.Value(o.b.Nodes[int(i)])
				case key == "$p" && isIndex:
					return d.Value(o.b.Ports[int(i)])
				case key == "$e" && isIndex:
					return d.Value(o.b.Edges[int(i)])
				case key == "$l" && isIndex:
					return d.Value(o.b.Labels[int(i)])
				case key == "$layer":
					// The layer keeps its index, as in the harness.
					if a, ok := ref.([]any); ok && len(a) == 2 {
						if g, ok := a[0].(float64); ok {
							return map[string]any{key: []any{float64(d.Ref(o.b.Graphs[int(g)])), a[1]}}
						}
					}
				}
			}
		}
		keys, _ := x[orderKey].([]string)
		if keys == nil {
			for k := range x {
				if k != orderKey {
					keys = append(keys, k)
				}
			}
		}
		out := make(map[string]any, len(x))
		for _, k := range keys {
			if k == "$o" {
				if old, ok := x[k].(float64); ok {
					out[k] = opaqueNumber(old, o.b, d)
					continue
				}
			}
			out[k] = o.encode(x[k], d)
		}
		return out
	}
	return v
}

// opaqueNumber returns the number that the object with the stored number old
// has in the dump of d.
func opaqueNumber(old float64, b *nativetest.Built, d *nativetest.Dumper) float64 {
	return float64(d.OpaqueObjectID(b, int(old)))
}
