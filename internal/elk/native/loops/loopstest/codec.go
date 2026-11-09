// Package loopstest is the test support of the self loop packages (loops,
// loops/ordering and loops/routing). It gives the test harness (nativetest)
// a codec for the property selfLoopHolder, whose value is the SelfLoopHolder
// of a node. A test that reads or writes states with that property links
// this package:
//
//	import _ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops/loopstest"
//
// The package imports nativetest, so only tests can import it.
//
// The stored form of a holder is the form of the dump of elkjs
// (tools/reference/gen-elk-phases.mjs): each Java object is
// {"$o": i, "c": class, "f": fields} at its first reference and {"$o": i}
// later, and the fields are the own properties of the JavaScript object in
// the order of their first assignment:
//
//	SelfLoopHolder       slHyperLoops, slPorts, routingSlotCount, lNode,
//	                     arePortsHidden
//	SelfHyperLoop        slPorts, slEdges, occupiedPortSides, routingSlot,
//	                     slHolder, slLabels, slPortsBySide, selfLoopType,
//	                     leftmostPort, rightmostPort
//	SelfLoopPort         incomingSLEdges, outgoingSLEdges, lPort,
//	                     hadOnlySelfLoops, isHidden
//	SelfLoopEdge         lEdge, slSource, slTarget, slHyperLoop
//	SelfHyperLoopLabels  lLabels, size_0, position, layoutDirection,
//	                     labelLabelSpacing, side, alignment,
//	                     alignmentReferenceSLPort, id_0
//
// A field that is null is not stored. The fields arePortsHidden, isHidden
// and id_0 are stored only after their first assignment, because the GWT
// compiler keeps their initial value on the prototype.
//
// The order matters: the dump numbers the objects, and the ports, edges and
// labels that only the holder still knows, in the order in which it meets
// them. The encoder walks the fields in the order above, and the decoder
// checks that each stored object has its fields in that order.
package loopstest

import (
	"fmt"
	"math"
	"slices"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// PropertyID is the id of the property InternalProperties.SELF_LOOP_HOLDER.
const PropertyID = "selfLoopHolder"

// The class names of the stored objects.
const (
	classHolder = "alg.layered.intermediate.loops.SelfLoopHolder"
	classLoop   = "alg.layered.intermediate.loops.SelfHyperLoop"
	classLabels = "alg.layered.intermediate.loops.SelfHyperLoopLabels"
	classPort   = "alg.layered.intermediate.loops.SelfLoopPort"
	classEdge   = "alg.layered.intermediate.loops.SelfLoopEdge"
)

// maxDepth is the depth below which the dump of elkjs writes the fields of
// an object. A deeper object is stored as {"$o": i, "c": class} only.
const maxDepth = 12

func init() {
	nativetest.RegisterCodec(PropertyID, Codec())
}

// Codec returns the codec of the property selfLoopHolder.
func Codec() nativetest.Codec {
	return nativetest.Codec{Decode: decode, Encode: encode}
}

// orderKey is the member that nativetest.ParseState adds to an object with
// two members or more: the names of the members in the order of the JSON
// text.
const orderKey = "\x00order"

// fail stops a decode or an encode. Build and Dump of the harness turn the
// panic into an error.
func fail(format string, args ...any) {
	panic(fmt.Sprintf("loopstest: "+format, args...))
}

////////////////////////////////////////////////////////////////////////////
// Stored values

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

func items(v any, key string) []any {
	list, ok := tagged(v, key).([]any)
	if !ok {
		fail("%v is not a %s value", v, key)
	}
	return list
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

func integer(v any) int {
	f := number(v)
	i := int(f)
	if float64(i) != f {
		fail("%v is not an integer", v)
	}
	return i
}

func boolean(v any) bool {
	b, ok := v.(bool)
	if !ok {
		fail("%v is not a boolean", v)
	}
	return b
}

func index(v any, key string, count int) int {
	i := integer(tagged(v, key))
	if i < 0 || i >= count {
		fail("%v does not name an element of the state", v)
	}
	return i
}

func enum[E ~int](v any) E {
	name, _ := v.(string)
	e, ok := prop.ParseEnum[E](name)
	if !ok {
		fail("%v is not a constant of the enum %T", v, e)
	}
	return e
}

func vector(v any) emath.KVector {
	a, ok := tagged(v, "$v").([]any)
	if !ok || len(a) != 2 {
		fail("%v is not a vector", v)
	}
	return emath.KVector{X: number(a[0]), Y: number(a[1])}
}

func slots(v any) [loops.PortSideCount]int {
	a := items(v, "$arr")
	if len(a) != loops.PortSideCount {
		fail("%v is not an array with an entry for each port side", v)
	}
	var out [loops.PortSideCount]int
	for i, x := range a {
		out[i] = integer(x)
	}
	return out
}

////////////////////////////////////////////////////////////////////////////
// Decode

// decoder builds the objects of one stored holder.
type decoder struct {
	b *nativetest.Built
	// objects has the Go object of each stored object, by its number.
	objects map[float64]any
	// fields has the stored fields of each object, by its number.
	fields map[float64]map[string]any
	// numbers has the numbers in the order of the first references.
	numbers []float64
}

func decode(raw any, b *nativetest.Built) (v any, err error) {
	defer func() {
		if r := recover(); r != nil {
			v, err = nil, fmt.Errorf("%v", r)
		}
	}()
	d := &decoder{b: b, objects: map[float64]any{}, fields: map[float64]map[string]any{}}
	d.collect(raw)
	for _, n := range d.numbers {
		d.fill(n)
	}
	holder, ok := d.ref(raw).(*loops.SelfLoopHolder)
	if !ok {
		fail("the value of the property %s is not a SelfLoopHolder: %v", PropertyID, raw)
	}
	return holder, nil
}

// collect makes the Go object of each stored object below v.
func (d *decoder) collect(v any) {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			d.collect(e)
		}
	case map[string]any:
		if n, ok := x["$o"].(float64); ok {
			class, full := x["c"].(string)
			if !full {
				return
			}
			f, ok := x["f"].(map[string]any)
			if !ok {
				fail("the object %v of the class %s has no fields: the dump of elkjs cuts an object below the depth %d", n, class, maxDepth)
			}
			if _, dup := d.objects[n]; dup {
				fail("the object %v has its fields two times", n)
			}
			switch class {
			case classHolder:
				d.objects[n] = &loops.SelfLoopHolder{SLPortMap: gwt.NewMap[*lgraph.LPort, *loops.SelfLoopPort]()}
			case classLoop:
				d.objects[n] = &loops.SelfHyperLoop{OccupiedPortSides: prop.NoneOf[core.PortSide]()}
			case classLabels:
				d.objects[n] = &loops.SelfHyperLoopLabels{}
			case classPort:
				d.objects[n] = &loops.SelfLoopPort{}
			case classEdge:
				d.objects[n] = &loops.SelfLoopEdge{}
			default:
				fail("an object of the class %s in a self loop holder", class)
			}
			d.fields[n] = f
			d.numbers = append(d.numbers, n)
			d.collect(f)
			return
		}
		for k, e := range x {
			if k != orderKey {
				d.collect(e)
			}
		}
	}
}

// ref returns the Go object that the stored reference v names.
func (d *decoder) ref(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		fail("%v is not a reference to an object", v)
	}
	n, ok := m["$o"].(float64)
	if !ok {
		fail("%v is not a reference to an object", v)
	}
	obj, ok := d.objects[n]
	if !ok {
		fail("the object %v has its fields outside of the self loop holder", n)
	}
	return obj
}

func (d *decoder) holder(v any) *loops.SelfLoopHolder {
	o, ok := d.ref(v).(*loops.SelfLoopHolder)
	if !ok {
		fail("%v is not a SelfLoopHolder", v)
	}
	return o
}

func (d *decoder) loop(v any) *loops.SelfHyperLoop {
	o, ok := d.ref(v).(*loops.SelfHyperLoop)
	if !ok {
		fail("%v is not a SelfHyperLoop", v)
	}
	return o
}

func (d *decoder) labels(v any) *loops.SelfHyperLoopLabels {
	o, ok := d.ref(v).(*loops.SelfHyperLoopLabels)
	if !ok {
		fail("%v is not a SelfHyperLoopLabels", v)
	}
	return o
}

func (d *decoder) port(v any) *loops.SelfLoopPort {
	o, ok := d.ref(v).(*loops.SelfLoopPort)
	if !ok {
		fail("%v is not a SelfLoopPort", v)
	}
	return o
}

func (d *decoder) edge(v any) *loops.SelfLoopEdge {
	o, ok := d.ref(v).(*loops.SelfLoopEdge)
	if !ok {
		fail("%v is not a SelfLoopEdge", v)
	}
	return o
}

func (d *decoder) ports(v any, key string) []*loops.SelfLoopPort {
	list := items(v, key)
	out := make([]*loops.SelfLoopPort, len(list))
	for i, x := range list {
		out[i] = d.port(x)
	}
	return out
}

func (d *decoder) edges(v any, key string) []*loops.SelfLoopEdge {
	list := items(v, key)
	out := make([]*loops.SelfLoopEdge, len(list))
	for i, x := range list {
		out[i] = d.edge(x)
	}
	return out
}

// fill sets the fields of the object with the number n.
func (d *decoder) fill(n float64) {
	f := d.fields[n]
	var keys []string
	// get returns a field that each object of the class has.
	get := func(key string) any {
		v, ok := f[key]
		if !ok {
			fail("the object %v (%T) has no field %s", n, d.objects[n], key)
		}
		keys = append(keys, key)
		return v
	}
	// opt returns a field that an object has from its first assignment on.
	opt := func(key string) (any, bool) {
		v, ok := f[key]
		if ok {
			keys = append(keys, key)
		}
		return v, ok
	}

	switch o := d.objects[n].(type) {
	case *loops.SelfLoopHolder:
		for _, x := range items(get("slHyperLoops"), "$list") {
			o.SLHyperLoops = append(o.SLHyperLoops, d.loop(x))
		}
		for _, rv := range items(get("slPorts"), "$map") {
			row, ok := rv.([]any)
			if !ok || len(row) != 2 {
				fail("%v is not an entry of a map", rv)
			}
			o.SLPortMap.Put(d.b.Ports[index(row[0], "$p", len(d.b.Ports))], d.port(row[1]))
		}
		o.RoutingSlotCount = slots(get("routingSlotCount"))
		o.LNode = d.b.Nodes[index(get("lNode"), "$n", len(d.b.Nodes))]
		if v, ok := opt("arePortsHidden"); ok {
			o.PortsHidden, o.PortsHiddenAssigned = boolean(v), true
		}

	case *loops.SelfHyperLoop:
		o.SLPorts = d.ports(get("slPorts"), "$list")
		o.SLEdges = d.edges(get("slEdges"), "$set")
		for _, name := range items(get("occupiedPortSides"), "$es") {
			o.OccupiedPortSides.Add(enum[core.PortSide](name))
		}
		o.RoutingSlot = slots(get("routingSlot"))
		o.SLHolder = d.holder(get("slHolder"))
		if v, ok := opt("slLabels"); ok {
			o.SLLabels = d.labels(v)
		}
		if v, ok := opt("slPortsBySide"); ok {
			o.SLPortsBySide = loops.NewSLPortsBySide()
			for _, rv := range items(v, "$mm") {
				row, ok := rv.([]any)
				if !ok || len(row) != 2 {
					fail("%v is not an entry of a multimap", rv)
				}
				side := enum[core.PortSide](row[0])
				values, ok := row[1].([]any)
				if !ok {
					fail("%v is not an entry of a multimap", rv)
				}
				for _, x := range values {
					o.SLPortsBySide.Put(side, d.port(x))
				}
			}
			o.SelfLoopType = enum[loops.SelfLoopType](get("selfLoopType"))
		}
		if v, ok := opt("leftmostPort"); ok {
			o.LeftmostPort = d.port(v)
		}
		if v, ok := opt("rightmostPort"); ok {
			o.RightmostPort = d.port(v)
		}

	case *loops.SelfLoopPort:
		o.IncomingSLEdges = d.edges(get("incomingSLEdges"), "$list")
		o.OutgoingSLEdges = d.edges(get("outgoingSLEdges"), "$list")
		o.LPort = d.b.Ports[index(get("lPort"), "$p", len(d.b.Ports))]
		o.HadOnlySelfLoops = boolean(get("hadOnlySelfLoops"))
		if v, ok := opt("isHidden"); ok {
			o.Hidden, o.HiddenAssigned = boolean(v), true
		}

	case *loops.SelfLoopEdge:
		o.LEdge = d.b.Edges[index(get("lEdge"), "$e", len(d.b.Edges))]
		o.SLSource = d.port(get("slSource"))
		o.SLTarget = d.port(get("slTarget"))
		if v, ok := opt("slHyperLoop"); ok {
			o.SLHyperLoop = d.loop(v)
		}

	case *loops.SelfHyperLoopLabels:
		for _, x := range items(get("lLabels"), "$list") {
			o.LLabels = append(o.LLabels, d.b.Labels[index(x, "$l", len(d.b.Labels))])
		}
		o.Size = vector(get("size_0"))
		o.Position = vector(get("position"))
		o.LayoutDirection = enum[core.Direction](get("layoutDirection"))
		o.LabelLabelSpacing = number(get("labelLabelSpacing"))
		if v, ok := opt("side"); ok {
			o.Placed = true
			o.Side = enum[core.PortSide](v)
			o.Alignment = enum[loops.Alignment](get("alignment"))
			if v, ok := opt("alignmentReferenceSLPort"); ok {
				o.AlignmentReferenceSLPort = d.port(v)
			}
		}
		if v, ok := opt("id_0"); ok {
			o.ID, o.IDAssigned = integer(v), true
		}
	}

	// Each stored field must be known, and the fields must have the order
	// in which the encoder writes them.
	for key := range f {
		if key != orderKey && !slices.Contains(keys, key) {
			fail("the object %v (%T) has the field %s, which the codec does not know", n, d.objects[n], key)
		}
	}
	if order, ok := f[orderKey].([]string); ok && !slices.Equal(order, keys) {
		fail("the object %v (%T) has its fields in the order %v, and the codec writes them in the order %v", n, d.objects[n], order, keys)
	}
}

////////////////////////////////////////////////////////////////////////////
// Encode

// encoder writes one holder in the order of the dump of elkjs.
type encoder struct {
	d *nativetest.Dumper
}

func encode(v any, d *nativetest.Dumper) any {
	holder, ok := v.(*loops.SelfLoopHolder)
	if !ok {
		fail("the property %s has a value of the type %T", PropertyID, v)
	}
	e := &encoder{d: d}
	return e.holder(holder, 0)
}

// object writes an object at the given depth of the value. fields adds the
// fields in the order of the JavaScript object.
func (e *encoder) object(obj any, class string, depth int, fields func(f map[string]any)) any {
	id, first := e.d.ObjectID(obj)
	if !first {
		return map[string]any{"$o": float64(id)}
	}
	out := map[string]any{"$o": float64(id), "c": class}
	if depth > maxDepth {
		return out
	}
	f := map[string]any{}
	fields(f)
	out["f"] = f
	return out
}

func list[T any](key string, values []T, item func(T) any) any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = item(v)
	}
	return map[string]any{key: out}
}

func slotArray(values [loops.PortSideCount]int) any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = float64(v)
	}
	return map[string]any{"$arr": out}
}

func (e *encoder) holder(h *loops.SelfLoopHolder, depth int) any {
	return e.object(h, classHolder, depth, func(f map[string]any) {
		f["slHyperLoops"] = list("$list", h.SLHyperLoops, func(l *loops.SelfHyperLoop) any { return e.loop(l, depth+2) })
		rows := []any{}
		h.SLPortMap.Each(func(lPort *lgraph.LPort, slPort *loops.SelfLoopPort) bool {
			// The key first: the dump meets the port before the object.
			key := e.d.Value(lPort)
			rows = append(rows, []any{key, e.port(slPort, depth+2)})
			return true
		})
		f["slPorts"] = map[string]any{"$map": rows}
		f["routingSlotCount"] = slotArray(h.RoutingSlotCount)
		f["lNode"] = e.d.Value(h.LNode)
		if h.PortsHiddenAssigned {
			f["arePortsHidden"] = h.PortsHidden
		}
	})
}

func (e *encoder) loop(l *loops.SelfHyperLoop, depth int) any {
	return e.object(l, classLoop, depth, func(f map[string]any) {
		f["slPorts"] = list("$list", l.SLPorts, func(p *loops.SelfLoopPort) any { return e.port(p, depth+2) })
		f["slEdges"] = list("$set", l.SLEdges, func(x *loops.SelfLoopEdge) any { return e.edge(x, depth+2) })
		f["occupiedPortSides"] = e.d.Value(l.OccupiedPortSides)
		f["routingSlot"] = slotArray(l.RoutingSlot)
		f["slHolder"] = e.holder(l.SLHolder, depth+1)
		if l.SLLabels != nil {
			f["slLabels"] = e.labels(l.SLLabels, depth+1)
		}
		if l.SLPortsBySide != nil {
			rows := []any{}
			for _, side := range l.SLPortsBySide.KeySet() {
				key := side.String()
				values := []any{}
				for _, p := range l.SLPortsBySide.Get(side) {
					values = append(values, e.port(p, depth+2))
				}
				rows = append(rows, []any{key, values})
			}
			f["slPortsBySide"] = map[string]any{"$mm": rows}
			f["selfLoopType"] = l.SelfLoopType.String()
		}
		if l.LeftmostPort != nil {
			f["leftmostPort"] = e.port(l.LeftmostPort, depth+1)
		}
		if l.RightmostPort != nil {
			f["rightmostPort"] = e.port(l.RightmostPort, depth+1)
		}
	})
}

func (e *encoder) port(p *loops.SelfLoopPort, depth int) any {
	return e.object(p, classPort, depth, func(f map[string]any) {
		f["incomingSLEdges"] = list("$list", p.IncomingSLEdges, func(x *loops.SelfLoopEdge) any { return e.edge(x, depth+2) })
		f["outgoingSLEdges"] = list("$list", p.OutgoingSLEdges, func(x *loops.SelfLoopEdge) any { return e.edge(x, depth+2) })
		f["lPort"] = e.d.Value(p.LPort)
		f["hadOnlySelfLoops"] = p.HadOnlySelfLoops
		if p.HiddenAssigned {
			f["isHidden"] = p.Hidden
		}
	})
}

func (e *encoder) edge(x *loops.SelfLoopEdge, depth int) any {
	return e.object(x, classEdge, depth, func(f map[string]any) {
		f["lEdge"] = e.d.Value(x.LEdge)
		f["slSource"] = e.port(x.SLSource, depth+1)
		f["slTarget"] = e.port(x.SLTarget, depth+1)
		if x.SLHyperLoop != nil {
			f["slHyperLoop"] = e.loop(x.SLHyperLoop, depth+1)
		}
	})
}

func (e *encoder) labels(l *loops.SelfHyperLoopLabels, depth int) any {
	return e.object(l, classLabels, depth, func(f map[string]any) {
		f["lLabels"] = list("$list", l.LLabels, func(lLabel *lgraph.LLabel) any { return e.d.Value(lLabel) })
		f["size_0"] = e.d.Value(&l.Size)
		f["position"] = e.d.Value(&l.Position)
		f["layoutDirection"] = l.LayoutDirection.String()
		f["labelLabelSpacing"] = e.d.Value(l.LabelLabelSpacing)
		if l.Placed {
			f["side"] = l.Side.String()
			f["alignment"] = l.Alignment.String()
			if l.AlignmentReferenceSLPort != nil {
				f["alignmentReferenceSLPort"] = e.port(l.AlignmentReferenceSLPort, depth+1)
			}
		}
		if l.IDAssigned {
			f["id_0"] = float64(l.ID)
		}
	})
}
