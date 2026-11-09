// Package compoundtest lets a test store and compare the property
// crossHierarchyMap of the compound graph processors. A test that dumps a
// graph after the CompoundGraphPreprocessor imports this package for its
// init function:
//
//	import _ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/compound/compoundtest"
//
// The package imports nativetest, so only tests can import it.
package compoundtest

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/compound"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// PropertyID is the id of the property InternalProperties.CROSS_HIERARCHY_MAP.
const PropertyID = "crossHierarchyMap"

// className is the class name of a CrossHierarchyEdge in a stored state.
const className = "alg.layered.compound.CrossHierarchyEdge"

func init() {
	nativetest.RegisterCodec(PropertyID, Codec())
}

// Codec returns the codec of the property crossHierarchyMap. The stored form
// is a multimap from an original edge to its segments:
//
//	{"$mm": [[{"$e": i}, [{"$o": n, "c": class, "f": {"newEdge": {"$e": i},
//	"graph_0": {"$g": i}, "type_0": "INPUT"}}]]]}
//
// The dump of elkjs numbers the stored objects of a state in the order in
// which it finds them. The codec takes the number of each segment from
// Dumper.ObjectID.
func Codec() nativetest.Codec {
	return nativetest.Codec{Decode: decode, Encode: encode}
}

func member(raw any, key string) (any, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := m[key]
	return v, ok
}

func index(raw any, key string, n int) (int, error) {
	v, ok := member(raw, key)
	f, isNumber := v.(float64)
	if !ok || !isNumber || int(f) < 0 || int(f) >= n {
		return 0, fmt.Errorf("not a reference %s to one of %d elements: %v", key, n, raw)
	}
	return int(f), nil
}

func decode(raw any, b *nativetest.Built) (any, error) {
	entries, ok := member(raw, "$mm")
	rows, isList := entries.([]any)
	if !ok || !isList {
		return nil, fmt.Errorf("not a multimap: %v", raw)
	}
	out := compound.NewCrossHierarchyMap()
	for _, rowValue := range rows {
		row, _ := rowValue.([]any)
		if len(row) != 2 {
			return nil, fmt.Errorf("not an entry of a multimap: %v", rowValue)
		}
		key, err := index(row[0], "$e", len(b.Edges))
		if err != nil {
			return nil, err
		}
		values, isList := row[1].([]any)
		if !isList {
			return nil, fmt.Errorf("not the values of an entry of a multimap: %v", row[1])
		}
		for _, value := range values {
			fields, ok := member(value, "f")
			if class, _ := member(value, "c"); !ok || class != className {
				return nil, fmt.Errorf("not a CrossHierarchyEdge: %v", value)
			}
			var newEdge *lgraph.LEdge
			if ref, has := member(fields, "newEdge"); has {
				i, err := index(ref, "$e", len(b.Edges))
				if err != nil {
					return nil, err
				}
				newEdge = b.Edges[i]
			}
			var graph *lgraph.LGraph
			if ref, has := member(fields, "graph_0"); has {
				i, err := index(ref, "$g", len(b.Graphs))
				if err != nil {
					return nil, err
				}
				graph = b.Graphs[i]
			}
			portType := lenum.PortTypeUndefined
			if name, has := member(fields, "type_0"); has {
				text, _ := name.(string)
				portType, ok = prop.ParseEnum[lenum.PortType](text)
				if !ok {
					return nil, fmt.Errorf("unknown port type %v", name)
				}
			}
			out.Put(b.Edges[key], compound.NewCrossHierarchyEdge(newEdge, graph, portType))
		}
	}
	return out, nil
}

func encode(v any, d *nativetest.Dumper) any {
	m, ok := v.(*compound.CrossHierarchyMap)
	if !ok {
		// Another value, for example one that a test set: store it as it is.
		return d.Value(v)
	}
	rows := []any{}
	for _, origEdge := range m.KeySet() {
		// The dump of elkjs visits the key, and then for each value its
		// number and its fields in the order newEdge, graph, type.
		key := map[string]any{"$e": float64(d.Ref(origEdge))}
		values := []any{}
		for _, chEdge := range m.Get(origEdge) {
			id, first := d.ObjectID(chEdge)
			if !first {
				values = append(values, map[string]any{"$o": float64(id)})
				continue
			}
			fields := map[string]any{}
			if chEdge.Edge() != nil {
				fields["newEdge"] = map[string]any{"$e": float64(d.Ref(chEdge.Edge()))}
			}
			if chEdge.Graph() != nil {
				fields["graph_0"] = map[string]any{"$g": float64(d.Ref(chEdge.Graph()))}
			}
			fields["type_0"] = chEdge.Type().String()
			values = append(values, map[string]any{"$o": float64(id), "c": className, "f": fields})
		}
		rows = append(rows, []any{key, values})
	}
	return map[string]any{"$mm": rows}
}
