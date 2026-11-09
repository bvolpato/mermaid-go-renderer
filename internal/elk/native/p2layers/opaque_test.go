package p2layers

import (
	"fmt"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

// The dump of elkjs numbers the Java objects that the harness does not know
// ({"$o": i, ...}) in the order in which its walk finds them. A layerer
// changes the order in which the walk finds the nodes, so the self loop
// holders of the nodes get other numbers in the state after the step. The
// harness writes an opaque value back with the numbers of the state before.
// This codec keeps the stored form and numbers the objects again when the
// harness dumps the graph. It does not move the fields of an object to the
// first reference of a new order: each object of these states has one
// owner.
//
// TODO(core): move to nativetest (number the objects of each opaque value in
// Dump).

// orderKey is the member that nativetest.ParseState adds to an object: the
// names of the members in the order of the JSON text.
const orderKey = "\x00order"

type opaqueValue struct {
	raw any
	b   *nativetest.Built
}

var (
	opaqueMu sync.Mutex
	// opaqueDumper is the dumper of the current dump, and opaqueNumbers maps
	// the object numbers of the stored form to the numbers of that dump.
	opaqueDumper  *nativetest.Dumper
	opaqueNumbers map[float64]float64
)

func init() {
	// The two properties of these states with objects that the harness does
	// not know. They share the numbers of one dump.
	for _, id := range []string{"selfLoopHolder", "crossHierarchyMap"} {
		nativetest.RegisterCodec(id, nativetest.Codec{
			Decode: func(raw any, b *nativetest.Built) (any, error) {
				return &opaqueValue{raw: raw, b: b}, nil
			},
			Encode: func(v any, d *nativetest.Dumper) any {
				o, ok := v.(*opaqueValue)
				if !ok {
					panic(fmt.Sprintf("%s: unexpected value %T", id, v))
				}
				opaqueMu.Lock()
				defer opaqueMu.Unlock()
				if opaqueDumper != d {
					opaqueDumper = d
					opaqueNumbers = map[float64]float64{}
				}
				return o.encode(o.raw, d)
			},
		})
	}
}

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
				switch key {
				case "$g":
					if isIndex {
						return map[string]any{key: float64(d.Ref(o.b.Graphs[int(i)]))}
					}
				case "$n":
					if isIndex {
						return map[string]any{key: float64(d.Ref(o.b.Nodes[int(i)]))}
					}
				case "$p":
					if isIndex {
						return map[string]any{key: float64(d.Ref(o.b.Ports[int(i)]))}
					}
				case "$e":
					if isIndex {
						return map[string]any{key: float64(d.Ref(o.b.Edges[int(i)]))}
					}
				case "$l":
					if isIndex {
						return map[string]any{key: float64(d.Ref(o.b.Labels[int(i)]))}
					}
				case "$layer":
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
				keys = append(keys, k)
			}
		}
		out := make(map[string]any, len(x))
		for _, k := range keys {
			if k == "$o" {
				old, _ := x[k].(float64)
				number, ok := opaqueNumbers[old]
				if !ok {
					number = float64(len(opaqueNumbers))
					opaqueNumbers[old] = number
				}
				out[k] = number
				continue
			}
			out[k] = o.encode(x[k], d)
		}
		return out
	}
	return v
}
