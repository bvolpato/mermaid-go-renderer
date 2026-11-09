// Ported from dagre-d3-es 7.0.14,
// dagre-d3-es/src/dagre/order/build-layer-graph.js (MIT).

package dagre

import "github.com/bvolpato/mermaid-go-renderer/internal/graphlib"

// relationship selects the edges that buildLayerGraph copies.
type relationship int

const (
	inEdges relationship = iota
	outEdges
)

// layerGraphLabel is the graph label {root} of a layer graph.
type layerGraphLabel struct {
	root string
}

// layerSubgraphLabel is the node label {borderLeft, borderRight} of a
// subgraph in a layer graph.
type layerSubgraphLabel struct {
	borderLeft  string
	borderRight string
}

// layerEdge is the edge label {weight} of a layer graph.
type layerEdge struct {
	weight float64
}

// layerNodeOf returns the label of a node of a layer graph that is not a
// subgraph. The label is shared with the layout graph.
func layerNodeOf(lg *graphlib.Graph, v string) *nodeLabel {
	n, _ := lg.Node(v).(*nodeLabel)
	return n
}

// buildLayerGraph constructs a graph that can be used to sort a layer of
// nodes. The graph contains all base and subgraph nodes from the requested
// layer in their original hierarchy and any edges that are incident on
// these nodes and are of the type requested by the "relationship" parameter.
//
// Nodes from the requested rank that do not have parents are assigned a
// root node in the output graph, which is set in the root graph attribute.
// This makes it easy to walk the hierarchy of movable nodes during ordering.
//
// Pre-conditions:
//
//  1. Input graph is a DAG.
//  2. Base nodes in the input graph have a rank attribute.
//  3. Subgraph nodes in the input graph have minRank and maxRank attributes.
//  4. Edges have an assigned weight.
//
// Post-conditions:
//
//  1. Output graph has all nodes in the movable rank with preserved
//     hierarchy.
//  2. Root nodes in the movable layer are made children of the node
//     indicated by the root attribute of the graph.
//  3. Non-movable nodes incident on movable nodes, selected by the
//     relationship parameter, are included in the graph (without hierarchy).
//  4. Edges incident on movable nodes, selected by the relationship
//     parameter, are added to the output graph.
//  5. The weights for copied edges are aggregated as need, since the output
//     graph is not a multi-graph.
func (l *layout) buildLayerGraph(g *graphlib.Graph, rank float64, relationship relationship) *graphlib.Graph {
	root := l.createRootNode(g)
	result := graphlib.New(graphlib.Options{Compound: true}).
		SetGraph(&layerGraphLabel{root: root}).
		SetDefaultNodeLabel(func(v string) any {
			return g.Node(v)
		})

	for _, v := range g.Nodes() {
		node := nodeOf(g, v)
		parent, hasParent := g.Parent(v)

		if (node.hasRank && node.rank == rank) || (node.minRank <= rank && rank <= node.maxRank) {
			result.SetNode(v)
			if hasParent && parent != "" {
				setParent(result, v, parent, true)
			} else {
				setParent(result, v, root, true)
			}

			// This assumes we have only short edges!
			var edges []graphlib.Edge
			if relationship == inEdges {
				edges = g.InEdges(v)
			} else {
				edges = g.OutEdges(v)
			}
			for _, e := range edges {
				u := e.V
				if e.V == v {
					u = e.W
				}
				edge, _ := result.Edge(u, v).(*layerEdge)
				weight := 0.0
				if edge != nil {
					weight = edge.weight
				}
				setEdge(result, u, v, &layerEdge{weight: edgeOf(g, e).weight + weight}, "")
			}

			if node.hasMinRank {
				result.SetNode(v, &layerSubgraphLabel{
					borderLeft:  sparseGet(node.borderLeft, rank),
					borderRight: sparseGet(node.borderRight, rank),
				})
			}
		}
	}

	return result
}

func (l *layout) createRootNode(g *graphlib.Graph) string {
	for {
		v := l.state.UniqueID("_root")
		if !g.HasNode(v) {
			return v
		}
	}
}
