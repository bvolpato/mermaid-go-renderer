// The stored state of an LGraph: the format of the phase goldens that
// tools/reference/gen-elk-phases.mjs writes from elkjs.
//
// A state is a JSON object with five tables: "g" (graphs), "n" (nodes), "p"
// (ports), "e" (edges) and "l" (labels). An element refers to another one by
// its index in the table of its kind, and -1 is null. The tables have each
// element that can be reached from the root graph, also through property
// values, in the order in which a walk finds them: the walk starts at the
// root graph and handles the elements in the order of their discovery. For
// each element it visits the references in the order of the fields below,
// and the properties in the order of their ids.
//
// A field with the default value is left out.
//
//	graph  id, sz [x,y] (size), pad [top,right,bottom,left], off [x,y]
//	       (offset), ll (layerless nodes), ly (layers: id, sz, n (nodes),
//	       pr), pn (parent node), pr (properties)
//	node   id, t (type), pos, sz, mg (margin), pad (padding), g (the field
//	       graph), ly [graph, index] (the layer), po (ports), lb (labels),
//	       ng (nested graph), psc (port sides cached), psi (the cached port
//	       side ranges: [side, first, last]), pr
//	port   id, o (owner), sd (side), pos, sz, an (anchor), ea (explicitly
//	       supplied port anchor), nx (not connected to external nodes), lb,
//	       in (incoming edges), out (outgoing edges), pr
//	edge   id, s (source port), t (target port), bp (bend points), lb, pr
//	label  id, tx (text), pos, sz, pr
//
// A number is a JSON number, or {"$f": "-0" | "NaN" | "Infinity" |
// "-Infinity"}. A property value is one of:
//
//	true, false       a Boolean
//	a number          an Integer or a Double
//	"NAME"            an enum constant
//	{"$s": text}      a String
//	{"$es": [names]}  an EnumSet
//	{"$v": [x, y]}    a KVector
//	{"$vc": [[x, y]]} a KVectorChain
//	{"$sp": [t,r,b,l]} an ElkPadding or an ElkMargin
//	{"$g": i}, {"$n": i}, {"$p": i}, {"$e": i}, {"$l": i}
//	                  an element of the LGraph
//	{"$layer": [g, i]} a Layer
//	{"$elk": ref}     an element of the ELK graph: "n:id", "p:id", "e:id" or
//	                  "l:<parent ref>#<index>"
//	{"$rnd": [hi, lo]} a java.util.Random with its state
//	{"$spacings": 1}  the Spacings of the graph
//	{"$algo": id}     a LayoutAlgorithmData
//	{"$list": [...]}, {"$set": [...]}, {"$map": [[k, v]]},
//	{"$mm": [[k, [v]]]}, {"$pair": [a, b]}, {"$arr": [...]}
//	                  collections, in iteration order
//	{"$o": i, "c": class, "f": {field: value}}
//	                  any other Java object, with the fields of the
//	                  JavaScript object. A second reference to the same
//	                  object is {"$o": i}. The numbers count the objects of
//	                  a state in the order of the walk. Dump numbers the
//	                  objects of opaque values and of codecs the same way:
//	                  a codec takes its numbers from Dumper.ObjectID.

package nativetest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// ElkRef stands for an element of the ELK graph in a graph that Build makes.
// It is the value of the property origin.
type ElkRef string

// ElkRef returns the reference text.
func (r ElkRef) ElkRef() string { return string(r) }

// ElkIdentifier returns the identifier of a node, a port or an edge.
func (r ElkRef) ElkIdentifier() string {
	s := string(r)
	if len(s) > 2 && s[0] != 'l' {
		return s[2:]
	}
	return ""
}

// AlgoRef stands for a LayoutAlgorithmData in a graph that Build makes. It
// is the value of the property resolvedAlgorithm.
type AlgoRef string

// AlgorithmID returns the id of the algorithm.
func (a AlgoRef) AlgorithmID() string { return string(a) }

// Opaque is a property value that the harness does not understand. Build
// keeps it and Dump writes it back. Raw is the stored form, where each
// reference to an element of the LGraph is an ElementRef or a LayerRef.
type Opaque struct {
	Raw any
}

// ElementRef is a reference to an element of the LGraph inside an Opaque
// value.
type ElementRef struct {
	// Element is a *lgraph.LGraph, *lgraph.LNode, *lgraph.LPort,
	// *lgraph.LEdge or *lgraph.LLabel.
	Element any
}

// LayerRef is a reference to a layer inside an Opaque value.
type LayerRef struct {
	Graph *lgraph.LGraph
	Index int
}

// ObjectRef is a reference to a Java object ({"$o": id}) inside an Opaque
// value. The stored number of the object is its number in the dump that the
// state came from. A dump numbers the objects again, in the order in which
// it meets them, as the dump of elkjs does. The class and the fields of the
// object are kept one time for the state, and the dump writes them where it
// meets the object first.
type ObjectRef struct {
	// ID is the stored number.
	ID      int
	objects *objectTable
}

// Class returns the class name of the object, as the state has it.
func (r ObjectRef) Class() string {
	c, _ := r.objects.class[r.ID].(string)
	return c
}

// Fields returns the fields of the object: a map from the names of the
// JavaScript fields to stored values, where each reference to an element is
// an ElementRef, a LayerRef or an ObjectRef.
func (r ObjectRef) Fields() map[string]any {
	f, _ := r.objects.fields[r.ID].(map[string]any)
	return f
}

// objectTable has the class and the fields of the Java objects of one
// state, by their stored numbers.
type objectTable struct {
	class  map[int]any
	fields map[int]any
}

// objectKey names an object of a state for Dumper.ObjectID.
type objectKey struct {
	objects *objectTable
	id      int
}

// opaque makes an Opaque value from a stored form: it replaces the element
// indices with the elements.
func (b *Built) opaque(raw any) Opaque { return Opaque{b.resolveRefs(raw)} }

// Opaque makes an Opaque value from the stored form of a property value. A
// codec that only writes its type can use it to read a state.
func (b *Built) Opaque(raw any) Opaque { return b.opaque(raw) }

func (b *Built) resolveRefs(raw any) any {
	switch x := raw.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = b.resolveRefs(e)
		}
		return out
	case map[string]any:
		if len(x) == 1 {
			for key, v := range x {
				i, isIndex := index(v)
				switch key {
				case "$g":
					if isIndex && i >= 0 && i < len(b.Graphs) {
						return ElementRef{b.Graphs[i]}
					}
				case "$n":
					if isIndex && i >= 0 && i < len(b.Nodes) {
						return ElementRef{b.Nodes[i]}
					}
				case "$p":
					if isIndex && i >= 0 && i < len(b.Ports) {
						return ElementRef{b.Ports[i]}
					}
				case "$e":
					if isIndex && i >= 0 && i < len(b.Edges) {
						return ElementRef{b.Edges[i]}
					}
				case "$l":
					if isIndex && i >= 0 && i < len(b.Labels) {
						return ElementRef{b.Labels[i]}
					}
				case "$layer":
					if a, ok := v.([]any); ok && len(a) == 2 {
						g, _ := index(a[0])
						li, _ := index(a[1])
						if g >= 0 && g < len(b.Graphs) {
							return LayerRef{b.Graphs[g], li}
						}
					}
				}
			}
		}
		if idv, isObject := x["$o"]; isObject {
			if id, ok := index(idv); ok {
				if b.objects == nil {
					b.objects = &objectTable{class: map[int]any{}, fields: map[int]any{}}
				}
				if c, has := x["c"]; has {
					b.objects.class[id] = c
				}
				if f, has := x["f"]; has {
					b.objects.fields[id] = b.resolveRefs(f)
				}
				return ObjectRef{ID: id, objects: b.objects}
			}
		}
		out := make(map[string]any, len(x))
		for k, v := range x {
			if k == orderKey {
				out[k] = v
				continue
			}
			out[k] = b.resolveRefs(v)
		}
		return out
	}
	return raw
}

// unresolveRefs is the reverse of resolveRefs for a dump. It visits the
// members of an object in the order of the JSON text, as the dump of elkjs
// visits the fields of a Java object.
func (d *Dumper) unresolveRefs(v any) any {
	switch x := v.(type) {
	case ElementRef:
		return map[string]any{"$" + kindOf(x.Element): float64(d.Ref(x.Element))}
	case LayerRef:
		return map[string]any{"$layer": []any{float64(d.Ref(x.Graph)), float64(x.Index)}}
	case ObjectRef:
		id, first := d.ObjectID(objectKey{x.objects, x.ID})
		out := map[string]any{"$o": float64(id)}
		if first {
			if c, has := x.objects.class[x.ID]; has {
				out["c"] = c
			}
			if f, has := x.objects.fields[x.ID]; has {
				out["f"] = d.unresolveRefs(f)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = d.unresolveRefs(e)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for _, k := range orderedKeys(x) {
			out[k] = d.unresolveRefs(x[k])
		}
		return out
	}
	return v
}

// Codec converts the values of one property between the stored form and the
// Go type of a port. Register a codec for a property whose values are
// objects of the package under test.
type Codec struct {
	// Decode builds the value from its stored form.
	Decode func(raw any, b *Built) (any, error)
	// Encode returns the stored form. It must give references to elements
	// through d.Ref.
	Encode func(v any, d *Dumper) any
}

var codecs = map[string]Codec{}

// RegisterCodec sets the codec of the property with the given id. Call it
// from an init function of a test.
func RegisterCodec(propertyID string, c Codec) { codecs[propertyID] = c }

// Built is an LGraph that Build made from a state, with its elements by
// index.
type Built struct {
	Graphs []*lgraph.LGraph
	Nodes  []*lgraph.LNode
	Ports  []*lgraph.LPort
	Edges  []*lgraph.LEdge
	Labels []*lgraph.LLabel
	// objects has the Java objects of the opaque values of the state.
	objects *objectTable
}

// Root is the root graph of the state.
func (b *Built) Root() *lgraph.LGraph { return b.Graphs[0] }

// orderKey is a member that ParseState adds to each object with two members
// or more: the names of the members in the order of the JSON text. A dump
// visits the fields of a stored Java object in that order. Diff ignores the
// member.
const orderKey = "\x00order"

// ParseState reads the JSON text of a state.
func ParseState(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseOrdered(dec)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("a state must be an object")
	}
	return m, nil
}

// parseOrdered reads one JSON value. A number becomes a float64.
func parseOrdered(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			m := map[string]any{}
			var order []string
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := kt.(string)
				v, err := parseOrdered(dec)
				if err != nil {
					return nil, err
				}
				if _, dup := m[key]; !dup {
					order = append(order, key)
				}
				m[key] = v
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			if len(order) > 1 {
				m[orderKey] = order
			}
			return m, nil
		case '[':
			a := []any{}
			for dec.More() {
				v, err := parseOrdered(dec)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return a, nil
		}
		return nil, fmt.Errorf("unexpected %v", t)
	case json.Number:
		f, err := strconv.ParseFloat(string(t), 64)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	return tok, nil
}

// orderedKeys returns the member names of a parsed object in the order of
// the JSON text.
func orderedKeys(m map[string]any) []string {
	if order, ok := m[orderKey].([]string); ok {
		return order
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		if k != orderKey {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func num(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case map[string]any:
		if s, ok := x["$f"].(string); ok {
			switch s {
			case "-0":
				return math.Copysign(0, -1), nil
			case "NaN":
				return math.NaN(), nil
			case "Infinity":
				return math.Inf(1), nil
			case "-Infinity":
				return math.Inf(-1), nil
			}
		}
	}
	return 0, fmt.Errorf("not a number: %v", v)
}

func encNum(x float64) any {
	switch {
	case x != x:
		return map[string]any{"$f": "NaN"}
	case math.IsInf(x, 1):
		return map[string]any{"$f": "Infinity"}
	case math.IsInf(x, -1):
		return map[string]any{"$f": "-Infinity"}
	case x == 0 && math.Signbit(x):
		return map[string]any{"$f": "-0"}
	}
	return x
}

func vec(v any, out *emath.KVector) error {
	a, ok := v.([]any)
	if !ok || len(a) != 2 {
		return fmt.Errorf("not a vector: %v", v)
	}
	x, err := num(a[0])
	if err != nil {
		return err
	}
	y, err := num(a[1])
	if err != nil {
		return err
	}
	out.X, out.Y = x, y
	return nil
}

func spacing(v any, out *emath.Spacing) error {
	a, ok := v.([]any)
	if !ok || len(a) != 4 {
		return fmt.Errorf("not a spacing: %v", v)
	}
	var f [4]float64
	for i := range f {
		x, err := num(a[i])
		if err != nil {
			return err
		}
		f[i] = x
	}
	out.Top, out.Right, out.Bottom, out.Left = f[0], f[1], f[2], f[3]
	return nil
}

func index(v any) (int, bool) {
	f, ok := v.(float64)
	return int(f), ok
}

func indices(v any) ([]int, error) {
	a, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("not a list of references: %v", v)
	}
	out := make([]int, len(a))
	for i, e := range a {
		n, ok := index(e)
		if !ok {
			return nil, fmt.Errorf("not a reference: %v", e)
		}
		out[i] = n
	}
	return out, nil
}

// Build makes the LGraph of a state.
func Build(state map[string]any) (b *Built, err error) {
	defer func() {
		if r := recover(); r != nil {
			b, err = nil, fmt.Errorf("build a graph from a state: %v", r)
		}
	}()
	table := func(key string) []map[string]any {
		rows, _ := state[key].([]any)
		out := make([]map[string]any, len(rows))
		for i, r := range rows {
			out[i], _ = r.(map[string]any)
		}
		return out
	}
	gs, ns, ps, es, ls := table("g"), table("n"), table("p"), table("e"), table("l")
	b = &Built{}
	for range gs {
		b.Graphs = append(b.Graphs, lgraph.NewLGraph())
	}
	for range ns {
		b.Nodes = append(b.Nodes, lgraph.NewLNode(nil))
	}
	for range ps {
		b.Ports = append(b.Ports, lgraph.NewLPort())
	}
	for range es {
		b.Edges = append(b.Edges, lgraph.NewLEdge())
	}
	for range ls {
		b.Labels = append(b.Labels, &lgraph.LLabel{})
	}
	node := func(v any) *lgraph.LNode {
		if i, ok := index(v); ok && i >= 0 {
			return b.Nodes[i]
		}
		return nil
	}
	graph := func(v any) *lgraph.LGraph {
		if i, ok := index(v); ok && i >= 0 {
			return b.Graphs[i]
		}
		return nil
	}
	port := func(v any) *lgraph.LPort {
		if i, ok := index(v); ok && i >= 0 {
			return b.Ports[i]
		}
		return nil
	}
	labels := func(v any) ([]*lgraph.LLabel, error) {
		if v == nil {
			return nil, nil
		}
		idx, err := indices(v)
		if err != nil {
			return nil, err
		}
		out := make([]*lgraph.LLabel, len(idx))
		for i, k := range idx {
			out[i] = b.Labels[k]
		}
		return out, nil
	}
	id := func(row map[string]any) int {
		n, _ := index(row["id"])
		return n
	}
	optVec := func(row map[string]any, key string, out *emath.KVector) error {
		if v, ok := row[key]; ok {
			return vec(v, out)
		}
		return nil
	}
	optSpacing := func(row map[string]any, key string, out *emath.Spacing) error {
		if v, ok := row[key]; ok {
			return spacing(v, out)
		}
		return nil
	}
	check := func(what string, i int, errs ...error) error {
		for _, e := range errs {
			if e != nil {
				return fmt.Errorf("%s %d: %w", what, i, e)
			}
		}
		return nil
	}

	// The structure first, then the properties: a property value can refer
	// to each element.
	for i, row := range gs {
		g := b.Graphs[i]
		g.ID = id(row)
		if err := check("graph", i, optVec(row, "sz", &g.Size), optSpacing(row, "pad", &g.Padding.Spacing), optVec(row, "off", &g.Offset)); err != nil {
			return nil, err
		}
		if v, ok := row["ll"]; ok {
			idx, err := indices(v)
			if err != nil {
				return nil, check("graph", i, err)
			}
			for _, k := range idx {
				g.LayerlessNodes = append(g.LayerlessNodes, b.Nodes[k])
			}
		}
		layers, _ := row["ly"].([]any)
		for _, lv := range layers {
			lrow, _ := lv.(map[string]any)
			layer := lgraph.NewLayer(g)
			layer.ID = id(lrow)
			if err := check("layer of graph", i, optVec(lrow, "sz", &layer.Size)); err != nil {
				return nil, err
			}
			idx, err := indices(lrow["n"])
			if err != nil {
				return nil, check("layer of graph", i, err)
			}
			for _, k := range idx {
				layer.Nodes = append(layer.Nodes, b.Nodes[k])
			}
			g.Layers = append(g.Layers, layer)
		}
		g.ParentNode = node(row["pn"])
	}
	for i, row := range ns {
		n := b.Nodes[i]
		n.ID = id(row)
		if t, ok := row["t"].(string); ok {
			nt, ok := prop.ParseEnum[lgraph.NodeType](t)
			if !ok {
				return nil, fmt.Errorf("node %d: unknown type %q", i, t)
			}
			n.Type = nt
		}
		if err := check("node", i, optVec(row, "pos", &n.Pos), optVec(row, "sz", &n.Size), optSpacing(row, "mg", &n.Margin.Spacing), optSpacing(row, "pad", &n.Padding.Spacing)); err != nil {
			return nil, err
		}
		n.SetGraph(graph(row["g"]))
		if ly, ok := row["ly"].([]any); ok && len(ly) == 2 {
			lg := graph(ly[0])
			li, _ := index(ly[1])
			switch {
			case lg == nil || li >= len(lg.Layers):
				return nil, fmt.Errorf("node %d: its layer is not in a graph of the state", i)
			case li < 0:
				// The node still names a layer that its graph dropped. The
				// state does not tell which nodes share such a layer.
				n.SetLayerField(lgraph.NewLayer(lg))
			default:
				n.SetLayerField(lg.Layers[li])
			}
		}
		if v, ok := row["po"]; ok {
			idx, err := indices(v)
			if err != nil {
				return nil, check("node", i, err)
			}
			for _, k := range idx {
				n.Ports = append(n.Ports, b.Ports[k])
			}
		}
		var err error
		if n.Labels, err = labels(row["lb"]); err != nil {
			return nil, check("node", i, err)
		}
		n.NestedGraph = graph(row["ng"])
		n.PortSidesCached = row["psc"] != nil
		if psi, ok := row["psi"].([]any); ok {
			n.PortSideIndices = []lgraph.PortSideRange{}
			for _, rv := range psi {
				r, _ := rv.([]any)
				if len(r) != 3 {
					return nil, fmt.Errorf("node %d: bad port side range", i)
				}
				name, _ := r[0].(string)
				side, ok := prop.ParseEnum[core.PortSide](name)
				if !ok {
					return nil, fmt.Errorf("node %d: unknown port side %q", i, name)
				}
				first, _ := index(r[1])
				last, _ := index(r[2])
				n.PortSideIndices = append(n.PortSideIndices, lgraph.PortSideRange{Side: side, First: first, Last: last})
			}
		}
	}
	for i, row := range ps {
		p := b.Ports[i]
		p.ID = id(row)
		p.SetNodeField(node(row["o"]))
		if s, ok := row["sd"].(string); ok {
			side, ok := prop.ParseEnum[core.PortSide](s)
			if !ok {
				return nil, fmt.Errorf("port %d: unknown side %q", i, s)
			}
			p.Side = side
		}
		if err := check("port", i, optVec(row, "pos", &p.Pos), optVec(row, "sz", &p.Size), optVec(row, "an", &p.Anchor)); err != nil {
			return nil, err
		}
		p.ExplicitlySuppliedPortAnchor = row["ea"] != nil
		p.ConnectedToExternalNodes = row["nx"] == nil
		var err error
		if p.Labels, err = labels(row["lb"]); err != nil {
			return nil, check("port", i, err)
		}
		for _, key := range []string{"in", "out"} {
			v, ok := row[key]
			if !ok {
				continue
			}
			idx, err := indices(v)
			if err != nil {
				return nil, check("port", i, err)
			}
			for _, k := range idx {
				if key == "in" {
					p.IncomingEdges = append(p.IncomingEdges, b.Edges[k])
				} else {
					p.OutgoingEdges = append(p.OutgoingEdges, b.Edges[k])
				}
			}
		}
	}
	for i, row := range es {
		e := b.Edges[i]
		e.ID = id(row)
		e.Source = port(row["s"])
		e.Target = port(row["t"])
		if bp, ok := row["bp"].([]any); ok {
			for _, pv := range bp {
				v := &emath.KVector{}
				if err := vec(pv, v); err != nil {
					return nil, check("edge", i, err)
				}
				e.BendPoints.Add(v)
			}
		}
		var err error
		if e.Labels, err = labels(row["lb"]); err != nil {
			return nil, check("edge", i, err)
		}
	}
	for i, row := range ls {
		l := b.Labels[i]
		l.ID = id(row)
		if tx, ok := row["tx"].(string); ok {
			l.Text, l.HasText = tx, true
		}
		if err := check("label", i, optVec(row, "pos", &l.Pos), optVec(row, "sz", &l.Size)); err != nil {
			return nil, err
		}
	}

	setProps := func(what string, i int, h prop.Holder, g *lgraph.LGraph, v any) error {
		props, _ := v.(map[string]any)
		ids := make([]string, 0, len(props))
		for id := range props {
			if id != orderKey {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		for _, pid := range ids {
			if err := b.setProperty(h, g, pid, props[pid]); err != nil {
				return fmt.Errorf("%s %d: property %s: %w", what, i, pid, err)
			}
		}
		return nil
	}
	for i, row := range gs {
		if err := setProps("graph", i, b.Graphs[i], b.Graphs[i], row["pr"]); err != nil {
			return nil, err
		}
		layers, _ := row["ly"].([]any)
		for k, lv := range layers {
			lrow, _ := lv.(map[string]any)
			if err := setProps("layer of graph", i, b.Graphs[i].Layers[k], b.Graphs[i], lrow["pr"]); err != nil {
				return nil, err
			}
		}
	}
	for i, row := range ns {
		if err := setProps("node", i, b.Nodes[i], nil, row["pr"]); err != nil {
			return nil, err
		}
	}
	for i, row := range ps {
		if err := setProps("port", i, b.Ports[i], nil, row["pr"]); err != nil {
			return nil, err
		}
	}
	for i, row := range es {
		if err := setProps("edge", i, b.Edges[i], nil, row["pr"]); err != nil {
			return nil, err
		}
	}
	for i, row := range ls {
		if err := setProps("label", i, b.Labels[i], nil, row["pr"]); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// setProperty decodes one stored property value and sets it.
func (b *Built) setProperty(h prop.Holder, g *lgraph.LGraph, id string, raw any) error {
	if c, ok := codecs[id]; ok {
		v, err := c.Decode(raw, b)
		if err != nil {
			return err
		}
		prop.SetByID(h, id, v)
		return nil
	}
	decl := prop.Declared(id)
	if len(decl) == 0 {
		// No Go package declares the property. Keep the stored form.
		prop.SetByID(h, id, b.opaque(raw))
		return nil
	}
	p := decl[0]
	v, err := b.decode(raw, p.Type(), g)
	if err != nil {
		return err
	}
	prop.SetAny(h, p, v)
	return nil
}

var (
	typeKVector      = reflect.TypeFor[*emath.KVector]()
	typeKVectorChain = reflect.TypeFor[*emath.KVectorChain]()
	typePadding      = reflect.TypeFor[*emath.ElkPadding]()
	typeMargin       = reflect.TypeFor[*emath.ElkMargin]()
	typeGraph        = reflect.TypeFor[*lgraph.LGraph]()
	typeNode         = reflect.TypeFor[*lgraph.LNode]()
	typePort         = reflect.TypeFor[*lgraph.LPort]()
	typeEdge         = reflect.TypeFor[*lgraph.LEdge]()
	typeLabel        = reflect.TypeFor[*lgraph.LLabel]()
	typeRandom       = reflect.TypeFor[*gwt.Random]()
	typeSpacings     = reflect.TypeFor[*lgraph.Spacings]()
	typeEnumSet      = reflect.TypeFor[prop.EnumSetValue]()
	typeNodeIntMap   = reflect.TypeFor[*gwt.Map[*lgraph.LNode, int]]()
)

func tag(raw any, key string) (any, bool) {
	m, ok := raw.(map[string]any)
	if !ok || len(m) != 1 {
		return nil, false
	}
	v, ok := m[key]
	return v, ok
}

// decode converts a stored value to the Go type t.
func (b *Built) decode(raw any, t reflect.Type, g *lgraph.LGraph) (any, error) {
	bad := func() (any, error) { return nil, fmt.Errorf("cannot read %v as %v", raw, t) }
	ref := func(key string, n int) (int, bool) {
		v, ok := tag(raw, key)
		if !ok {
			return 0, false
		}
		i, ok := index(v)
		return i, ok && i >= 0 && i < n
	}
	switch t {
	case typeKVector:
		v, ok := tag(raw, "$v")
		out := &emath.KVector{}
		if !ok || vec(v, out) != nil {
			return bad()
		}
		return out, nil
	case typeKVectorChain:
		v, ok := tag(raw, "$vc")
		pts, isList := v.([]any)
		if !ok || !isList {
			return bad()
		}
		out := emath.NewKVectorChain()
		for _, pv := range pts {
			p := &emath.KVector{}
			if vec(pv, p) != nil {
				return bad()
			}
			out.Add(p)
		}
		return out, nil
	case typePadding:
		v, ok := tag(raw, "$sp")
		out := &emath.ElkPadding{}
		if !ok || spacing(v, &out.Spacing) != nil {
			return bad()
		}
		return out, nil
	case typeMargin:
		v, ok := tag(raw, "$sp")
		out := &emath.ElkMargin{}
		if !ok || spacing(v, &out.Spacing) != nil {
			return bad()
		}
		return out, nil
	case typeGraph:
		if i, ok := ref("$g", len(b.Graphs)); ok {
			return b.Graphs[i], nil
		}
		return bad()
	case typeNode:
		if i, ok := ref("$n", len(b.Nodes)); ok {
			return b.Nodes[i], nil
		}
		return bad()
	case typePort:
		if i, ok := ref("$p", len(b.Ports)); ok {
			return b.Ports[i], nil
		}
		return bad()
	case typeEdge:
		if i, ok := ref("$e", len(b.Edges)); ok {
			return b.Edges[i], nil
		}
		return bad()
	case typeLabel:
		if i, ok := ref("$l", len(b.Labels)); ok {
			return b.Labels[i], nil
		}
		return bad()
	case typeRandom:
		v, ok := tag(raw, "$rnd")
		a, isList := v.([]any)
		if !ok || !isList || len(a) != 2 {
			return bad()
		}
		hi, _ := a[0].(float64)
		lo, _ := a[1].(float64)
		return &gwt.Random{SeedHi: hi, SeedLo: lo}, nil
	case typeSpacings:
		if _, ok := tag(raw, "$spacings"); !ok || g == nil {
			return bad()
		}
		return lgraph.NewSpacings(g), nil
	case typeNodeIntMap:
		v, ok := tag(raw, "$map")
		rows, isList := v.([]any)
		if !ok || !isList {
			return bad()
		}
		out := gwt.NewMap[*lgraph.LNode, int]()
		for _, rv := range rows {
			r, _ := rv.([]any)
			if len(r) != 2 {
				return bad()
			}
			k, err := b.decode(r[0], typeNode, g)
			if err != nil {
				return nil, err
			}
			n, _ := r[1].(float64)
			out.Put(k.(*lgraph.LNode), int(n))
		}
		return out, nil
	}
	if t.Implements(typeEnumSet) && t.Kind() == reflect.Pointer {
		v, ok := tag(raw, "$es")
		names, isList := v.([]any)
		if !ok || !isList {
			return bad()
		}
		set := reflect.New(t.Elem()).Interface().(prop.EnumSetValue)
		info := prop.EnumOf(set.EnumType())
		if info == nil {
			return nil, fmt.Errorf("the enum %v is not registered", set.EnumType())
		}
		var bits uint64
		for _, nv := range names {
			name, _ := nv.(string)
			o := info.Ordinal(name)
			if o < 0 {
				return nil, fmt.Errorf("unknown constant %q of %s", name, info.Class)
			}
			bits |= 1 << uint(o)
		}
		set.SetBits(bits)
		return set, nil
	}
	switch t.Kind() {
	case reflect.Bool:
		if v, ok := raw.(bool); ok {
			return v, nil
		}
		return bad()
	case reflect.Float64:
		f, err := num(raw)
		if err != nil {
			return bad()
		}
		return f, nil
	case reflect.String:
		if v, ok := tag(raw, "$s"); ok {
			if s, ok := v.(string); ok {
				return s, nil
			}
		}
		return bad()
	case reflect.Int:
		if info := prop.EnumOf(t); info != nil {
			name, ok := raw.(string)
			if !ok {
				return bad()
			}
			o := info.Ordinal(name)
			if o < 0 {
				return nil, fmt.Errorf("unknown constant %q of %s", name, info.Class)
			}
			return reflect.ValueOf(o).Convert(t).Interface(), nil
		}
		if f, ok := raw.(float64); ok {
			return int(f), nil
		}
		return bad()
	case reflect.Slice:
		v, ok := tag(raw, "$list")
		items, isList := v.([]any)
		if !ok || !isList {
			return bad()
		}
		out := reflect.MakeSlice(t, 0, len(items))
		for _, item := range items {
			e, err := b.decode(item, t.Elem(), g)
			if err != nil {
				return nil, err
			}
			out = reflect.Append(out, reflect.ValueOf(e))
		}
		return out.Interface(), nil
	case reflect.Interface:
		// An untyped property: an element, a reference to the ELK graph, or
		// a value that only a codec understands.
		if v, ok := tag(raw, "$elk"); ok {
			if s, ok := v.(string); ok {
				return ElkRef(s), nil
			}
		}
		if v, ok := tag(raw, "$algo"); ok {
			if s, ok := v.(string); ok {
				return AlgoRef(s), nil
			}
		}
		for key, et := range map[string]reflect.Type{"$g": typeGraph, "$n": typeNode, "$p": typePort, "$e": typeEdge, "$l": typeLabel} {
			if _, ok := tag(raw, key); ok {
				return b.decode(raw, et, g)
			}
		}
		return b.opaque(raw), nil
	}
	return bad()
}

// Dumper writes the state of an LGraph. See Dump.
type Dumper struct {
	tables  map[string][]any
	ids     map[any]int
	queue   []queued
	objects map[any]int
	// nextObject is the number of the next Java object of the dump.
	nextObject int
	err        error
}

// ObjectID returns the number of a Java object that is not an element of
// the LGraph, for the stored form {"$o": id, "c": class, "f": fields}. The
// dump of elkjs numbers such objects in the order in which it meets them.
// first is true for the first call with the object: a codec then writes the
// class and the fields, and only {"$o": id} later.
func (d *Dumper) ObjectID(obj any) (id int, first bool) {
	if id, ok := d.objects[obj]; ok {
		return id, false
	}
	id = d.nextObject
	d.nextObject++
	d.objects[obj] = id
	return id, true
}

// NewObjectID returns the next object number of the dump, for an object
// that the dump meets one time only.
func (d *Dumper) NewObjectID() int {
	id := d.nextObject
	d.nextObject++
	return id
}

// OpaqueObjectID returns the number that the Java object with the stored
// number old, of the state that b was built from, has in the dump. A codec
// that keeps the stored form of a value uses it for each "$o".
func (d *Dumper) OpaqueObjectID(b *Built, old int) int {
	if b.objects == nil {
		b.objects = &objectTable{class: map[int]any{}, fields: map[int]any{}}
	}
	id, _ := d.ObjectID(objectKey{b.objects, old})
	return id
}

type queued struct {
	kind string
	id   int
	el   any
}

func kindOf(el any) string {
	switch el.(type) {
	case *lgraph.LGraph:
		return "g"
	case *lgraph.LNode:
		return "n"
	case *lgraph.LPort:
		return "p"
	case *lgraph.LEdge:
		return "e"
	case *lgraph.LLabel:
		return "l"
	}
	return ""
}

func isNilElement(el any) bool {
	switch x := el.(type) {
	case *lgraph.LGraph:
		return x == nil
	case *lgraph.LNode:
		return x == nil
	case *lgraph.LPort:
		return x == nil
	case *lgraph.LEdge:
		return x == nil
	case *lgraph.LLabel:
		return x == nil
	}
	return el == nil
}

// Ref returns the index of an element (a graph, a node, a port, an edge or a
// label) in the table of its kind, and -1 for nil. An element that the dump
// did not see before gets the next index.
func (d *Dumper) Ref(el any) int {
	if isNilElement(el) {
		return -1
	}
	if id, ok := d.ids[el]; ok {
		return id
	}
	kind := kindOf(el)
	id := len(d.tables[kind])
	d.tables[kind] = append(d.tables[kind], nil)
	d.ids[el] = id
	d.queue = append(d.queue, queued{kind, id, el})
	return id
}

func refs[T any](d *Dumper, list []T) []any {
	out := make([]any, len(list))
	for i, el := range list {
		out[i] = float64(d.Ref(el))
	}
	return out
}

func encVec(v *emath.KVector) any { return []any{encNum(v.X), encNum(v.Y)} }

func isZeroVec(v *emath.KVector) bool {
	return v.X == 0 && !math.Signbit(v.X) && v.Y == 0 && !math.Signbit(v.Y)
}

func encSpacing(s *emath.Spacing) any {
	return []any{encNum(s.Top), encNum(s.Right), encNum(s.Bottom), encNum(s.Left)}
}

func isZeroSpacing(s *emath.Spacing) bool {
	for _, x := range []float64{s.Top, s.Right, s.Bottom, s.Left} {
		if x != 0 || math.Signbit(x) {
			return false
		}
	}
	return true
}

// Dump returns the state of the LGraph hierarchy below root, in the form of
// ParseState.
func Dump(root *lgraph.LGraph) (state map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			state, err = nil, fmt.Errorf("dump a graph: %v", r)
		}
	}()
	d := &Dumper{tables: map[string][]any{"g": {}, "n": {}, "p": {}, "e": {}, "l": {}}, ids: map[any]int{}, objects: map[any]int{}}
	d.Ref(root)
	for len(d.queue) > 0 {
		q := d.queue[0]
		d.queue = d.queue[1:]
		d.tables[q.kind][q.id] = d.fill(q.el)
	}
	if d.err != nil {
		return nil, d.err
	}
	out := map[string]any{}
	for k, v := range d.tables {
		out[k] = v
	}
	return out, nil
}

func (d *Dumper) fill(el any) map[string]any {
	out := map[string]any{}
	putID := func(id int) {
		if id != 0 {
			out["id"] = float64(id)
		}
	}
	putVec := func(key string, v *emath.KVector) {
		if !isZeroVec(v) {
			out[key] = encVec(v)
		}
	}
	putSpacing := func(key string, s *emath.Spacing) {
		if !isZeroSpacing(s) {
			out[key] = encSpacing(s)
		}
	}
	putProps := func(target map[string]any, h prop.Holder) {
		if pr := d.props(h); pr != nil {
			target["pr"] = pr
		}
	}
	switch o := el.(type) {
	case *lgraph.LGraph:
		putID(o.ID)
		putVec("sz", &o.Size)
		putSpacing("pad", &o.Padding.Spacing)
		putVec("off", &o.Offset)
		if len(o.LayerlessNodes) != 0 {
			out["ll"] = refs(d, o.LayerlessNodes)
		}
		if len(o.Layers) != 0 {
			layers := make([]any, len(o.Layers))
			for i, layer := range o.Layers {
				lo := map[string]any{}
				if layer.ID != 0 {
					lo["id"] = float64(layer.ID)
				}
				if !isZeroVec(&layer.Size) {
					lo["sz"] = encVec(&layer.Size)
				}
				lo["n"] = refs(d, layer.Nodes)
				putProps(lo, layer)
				layers[i] = lo
			}
			out["ly"] = layers
		}
		if o.ParentNode != nil {
			out["pn"] = float64(d.Ref(o.ParentNode))
		}
		putProps(out, o)
	case *lgraph.LNode:
		putID(o.ID)
		if o.Type != lgraph.NodeTypeNormal {
			out["t"] = o.Type.String()
		}
		putVec("pos", &o.Pos)
		putVec("sz", &o.Size)
		putSpacing("mg", &o.Margin.Spacing)
		putSpacing("pad", &o.Padding.Spacing)
		out["g"] = float64(d.Ref(o.GraphField()))
		if layer := o.Layer(); layer != nil {
			out["ly"] = []any{float64(d.Ref(layer.Graph())), float64(layer.Index())}
		}
		if len(o.Ports) != 0 {
			out["po"] = refs(d, o.Ports)
		}
		if len(o.Labels) != 0 {
			out["lb"] = refs(d, o.Labels)
		}
		if o.NestedGraph != nil {
			out["ng"] = float64(d.Ref(o.NestedGraph))
		}
		if o.PortSidesCached {
			out["psc"] = float64(1)
		}
		if o.PortSideIndices != nil {
			rows := make([]any, len(o.PortSideIndices))
			for i, r := range o.PortSideIndices {
				rows[i] = []any{r.Side.String(), float64(r.First), float64(r.Last)}
			}
			out["psi"] = rows
		}
		putProps(out, o)
	case *lgraph.LPort:
		putID(o.ID)
		out["o"] = float64(d.Ref(o.Node()))
		if o.Side != core.PortSideUndefined {
			out["sd"] = o.Side.String()
		}
		putVec("pos", &o.Pos)
		putVec("sz", &o.Size)
		putVec("an", &o.Anchor)
		if o.ExplicitlySuppliedPortAnchor {
			out["ea"] = float64(1)
		}
		if !o.ConnectedToExternalNodes {
			out["nx"] = float64(1)
		}
		if len(o.Labels) != 0 {
			out["lb"] = refs(d, o.Labels)
		}
		if len(o.IncomingEdges) != 0 {
			out["in"] = refs(d, o.IncomingEdges)
		}
		if len(o.OutgoingEdges) != 0 {
			out["out"] = refs(d, o.OutgoingEdges)
		}
		putProps(out, o)
	case *lgraph.LEdge:
		putID(o.ID)
		out["s"] = float64(d.Ref(o.Source))
		out["t"] = float64(d.Ref(o.Target))
		if o.BendPoints.Len() != 0 {
			bends := make([]any, o.BendPoints.Len())
			for i, v := range o.BendPoints.Points {
				bends[i] = encVec(v)
			}
			out["bp"] = bends
		}
		if len(o.Labels) != 0 {
			out["lb"] = refs(d, o.Labels)
		}
		putProps(out, o)
	case *lgraph.LLabel:
		putID(o.ID)
		if o.HasText {
			out["tx"] = o.Text
		}
		putVec("pos", &o.Pos)
		putVec("sz", &o.Size)
		putProps(out, o)
	}
	return out
}

// skipProps are the properties that are not part of a state.
var skipProps = map[string]bool{"processors": true}

func (d *Dumper) props(h prop.Holder) map[string]any {
	type row struct {
		id string
		v  any
	}
	var rows []row
	prop.Each(h, func(id string, v any) {
		if !skipProps[id] {
			rows = append(rows, row{id, v})
		}
	})
	if len(rows) == 0 {
		return nil
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	out := map[string]any{}
	for _, r := range rows {
		if c, ok := codecs[r.id]; ok {
			out[r.id] = c.Encode(r.v, d)
			continue
		}
		out[r.id] = d.Value(r.v)
	}
	return out
}

// Value returns the stored form of a property value.
func (d *Dumper) Value(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case Opaque:
		return d.unresolveRefs(x.Raw)
	case bool:
		return x
	case int:
		return float64(x)
	case float64:
		return encNum(x)
	case string:
		return map[string]any{"$s": x}
	case *emath.KVector:
		return map[string]any{"$v": encVec(x)}
	case *emath.KVectorChain:
		pts := make([]any, x.Len())
		for i, p := range x.Points {
			pts[i] = encVec(p)
		}
		return map[string]any{"$vc": pts}
	case *emath.ElkPadding:
		return map[string]any{"$sp": encSpacing(&x.Spacing)}
	case *emath.ElkMargin:
		return map[string]any{"$sp": encSpacing(&x.Spacing)}
	case *lgraph.LGraph, *lgraph.LNode, *lgraph.LPort, *lgraph.LEdge, *lgraph.LLabel:
		return map[string]any{"$" + kindOf(x): float64(d.Ref(x))}
	case *lgraph.Layer:
		return map[string]any{"$layer": []any{float64(d.Ref(x.Graph())), float64(x.Index())}}
	case lgraph.OriginRef:
		return map[string]any{"$elk": x.ElkRef()}
	case interface{ AlgorithmID() string }:
		return map[string]any{"$algo": x.AlgorithmID()}
	case *gwt.Random:
		return map[string]any{"$rnd": []any{x.SeedHi, x.SeedLo}}
	case *lgraph.Spacings:
		return map[string]any{"$spacings": float64(1)}
	case prop.EnumSetValue:
		info := prop.EnumOf(x.EnumType())
		names := []any{}
		for b, i := x.Bits(), 0; b != 0; b, i = b>>1, i+1 {
			if b&1 != 0 {
				if info == nil {
					d.fail("the enum %v is not registered", x.EnumType())
					return nil
				}
				names = append(names, prop.EnumName(info.Names, i))
			}
		}
		return map[string]any{"$es": names}
	case interface{ EachAny(func(key, value any)) }:
		rows := []any{}
		x.EachAny(func(k, val any) { rows = append(rows, []any{d.Value(k), d.Value(val)}) })
		return map[string]any{"$map": rows}
	case interface{ EachAny(func(key any)) }:
		items := []any{}
		x.EachAny(func(k any) { items = append(items, d.Value(k)) })
		return map[string]any{"$set": items}
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int:
		if info := prop.EnumOf(rv.Type()); info != nil {
			return prop.EnumName(info.Names, int(rv.Int()))
		}
		return float64(rv.Int())
	case reflect.Slice:
		items := make([]any, rv.Len())
		for i := range items {
			items[i] = d.Value(rv.Index(i).Interface())
		}
		return map[string]any{"$list": items}
	}
	d.fail("cannot store a property value of the type %T. Register a codec for the property with nativetest.RegisterCodec", v)
	return nil
}

func (d *Dumper) fail(format string, args ...any) {
	if d.err == nil {
		d.err = fmt.Errorf(format, args...)
	}
}

// Diff compares two states, or two parts of states. It returns a description
// of each difference, at most max of them. Numbers must be equal bit for
// bit.
func Diff(got, want any, max int) []string {
	var out []string
	diff("", got, want, &out, max)
	return out
}

func diff(path string, got, want any, out *[]string, max int) {
	if len(*out) >= max {
		return
	}
	add := func(format string, args ...any) {
		*out = append(*out, path+": "+fmt.Sprintf(format, args...))
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			add("got %s, want %s", show(got), show(want))
			return
		}
		keys := map[string]bool{}
		for k := range w {
			if k != orderKey {
				keys[k] = true
			}
		}
		for k := range g {
			if k != orderKey {
				keys[k] = true
			}
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			gv, gok := g[k]
			wv, wok := w[k]
			sub := path + "." + k
			switch {
			case !gok:
				if len(*out) < max {
					*out = append(*out, sub+": missing, want "+show(wv))
				}
			case !wok:
				if len(*out) < max {
					*out = append(*out, sub+": got "+show(gv)+", want nothing")
				}
			default:
				diff(sub, gv, wv, out, max)
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok {
			add("got %s, want %s", show(got), show(want))
			return
		}
		if len(g) != len(w) {
			add("got %d elements %s, want %d elements %s", len(g), show(got), len(w), show(want))
			return
		}
		for i := range w {
			diff(path+"["+strconv.Itoa(i)+"]", g[i], w[i], out, max)
		}
	case float64:
		g, ok := got.(float64)
		if !ok || math.Float64bits(g) != math.Float64bits(w) {
			add("got %s, want %s", show(got), show(want))
		}
	default:
		if !reflect.DeepEqual(got, want) {
			add("got %s, want %s", show(got), show(want))
		}
	}
}

func show(v any) string {
	if f, ok := v.(float64); ok {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	data, err := json.Marshal(stripOrder(v))
	if err != nil {
		return fmt.Sprint(v)
	}
	if len(data) > 300 {
		return string(data[:300]) + "…"
	}
	return string(data)
}

// stripOrder returns v without the order members of ParseState.
func stripOrder(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = stripOrder(e)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			if k != orderKey {
				out[k] = stripOrder(e)
			}
		}
		return out
	}
	return v
}
