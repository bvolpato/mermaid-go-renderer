// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/graphlib/json.js (MIT).

package graphlib

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Write returns the JSON form of g as a dynamic JavaScript value (see
// internal/js/value.go):
//
//	{options: {directed, multigraph, compound}, nodes: [{v, value, parent}],
//	 edges: [{v, w, name, value}], value}
//
// A property is left out where the JavaScript function leaves it out: a nil
// node label, a root parent, an edge without a name, a nil edge label and a
// nil graph label. Labels are not converted. js.StringifyJSON prints the
// result as JSON.stringify does when all labels are dynamic values.
func Write(g *Graph) *js.Obj[any] {
	options := js.NewObj[any]()
	options.Set("directed", g.IsDirected())
	options.Set("multigraph", g.IsMultigraph())
	options.Set("compound", g.IsCompound())

	json := js.NewObj[any]()
	json.Set("options", options)
	json.Set("nodes", writeNodes(g))
	json.Set("edges", writeEdges(g))
	if g.Graph() != nil {
		json.Set("value", clone(g.Graph()))
	}
	return json
}

func writeNodes(g *Graph) []any {
	nodes := []any{}
	for _, v := range g.Nodes() {
		nodeValue := g.Node(v)
		parent, hasParent := g.Parent(v)
		node := js.NewObj[any]()
		node.Set("v", v)
		if nodeValue != nil {
			node.Set("value", nodeValue)
		}
		if hasParent {
			node.Set("parent", parent)
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func writeEdges(g *Graph) []any {
	edges := []any{}
	for _, e := range g.Edges() {
		edgeValue := g.EdgeObj(e)
		edge := js.NewObj[any]()
		edge.Set("v", e.V)
		edge.Set("w", e.W)
		if e.Name != "" {
			edge.Set("name", e.Name)
		}
		if edgeValue != nil {
			edge.Set("value", edgeValue)
		}
		edges = append(edges, edge)
	}
	return edges
}

// clone is the shallow _.clone for dynamic values. Other values are returned
// as they are.
func clone(v any) any {
	switch x := v.(type) {
	case []any:
		return append([]any{}, x...)
	case *js.Obj[any]:
		out := js.NewObj[any]()
		x.Each(func(k string, e any) { out.Set(k, e) })
		return out
	}
	return v
}

// Read builds a graph from the JSON form that Write returns, given as a
// dynamic JavaScript value (for example from js.ParseJSON).
//
// The options "directed", "multigraph" and "compound" are converted to
// booleans. The JavaScript function stores them as they are.
func Read(json any) (*Graph, error) {
	opts := Options{}
	if options := js.AsObj(js.Path(json, "options")); options != nil {
		if directed, ok := options.Lookup("directed"); ok {
			opts.Undirected = !js.Truthy(directed)
		}
		if multigraph, ok := options.Lookup("multigraph"); ok {
			opts.Multigraph = js.Truthy(multigraph)
		}
		if compound, ok := options.Lookup("compound"); ok {
			opts.Compound = js.Truthy(compound)
		}
	}
	g := New(opts).SetGraph(js.Path(json, "value"))

	nodes, _ := js.Path(json, "nodes").([]any)
	for _, entry := range nodes {
		v := js.ToString(js.Path(entry, "v"))
		g.SetNode(v, js.Path(entry, "value"))
		if parent := js.Path(entry, "parent"); js.Truthy(parent) {
			if err := g.SetParent(v, js.ToString(parent)); err != nil {
				return nil, err
			}
		}
	}

	edges, _ := js.Path(json, "edges").([]any)
	for _, entry := range edges {
		v := js.ToString(js.Path(entry, "v"))
		w := js.ToString(js.Path(entry, "w"))
		value := js.Path(entry, "value")
		var err error
		if name := js.Path(entry, "name"); name != nil {
			// Every value except undefined is a name, also null and "".
			err = g.SetEdge(v, w, value, js.ToString(name))
		} else {
			err = g.SetEdge(v, w, value)
		}
		if err != nil {
			return nil, err
		}
	}
	return g, nil
}
