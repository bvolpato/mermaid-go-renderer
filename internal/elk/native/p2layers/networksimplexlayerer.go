// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p2layers/NetworkSimplexLayerer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package p2layers has the layerers of the layered algorithm: the second
// phase, which puts each node in a layer so that each edge points to a later
// layer.
package p2layers

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/networksimplex"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("NetworkSimplexLayerer", func() lgraph.Processor { return NewNetworkSimplexLayerer() })
}

// iterLimitFactor is ITER_LIMIT_FACTOR: the factor of the maximal number of
// iterations.
const iterLimitFactor = 4

// NetworkSimplexLayerer is the phase NetworkSimplexLayerer: it finds the
// layering with the minimal sum of edge lengths with the network simplex
// algorithm, for each connected component of the graph.
type NetworkSimplexLayerer struct {
	// layeredGraph is the graph that the methods work on.
	layeredGraph *lgraph.LGraph
	// componentNodes has the nodes of the connected component that
	// connectedComponents finds at the moment.
	componentNodes []*lgraph.LNode
	// nodeVisited tells for each node that a depth-first search visited it.
	nodeVisited []bool
}

// NewNetworkSimplexLayerer is new NetworkSimplexLayerer().
func NewNetworkSimplexLayerer() *NetworkSimplexLayerer { return &NetworkSimplexLayerer{} }

// connectedComponents finds the connected components of the graph with the
// given nodes. A component that is larger than the first one of the result
// goes to the front, and each other one to the end.
func (l *NetworkSimplexLayerer) connectedComponents(theNodes []*lgraph.LNode) [][]*lgraph.LNode {
	// initialize required attributes
	if l.nodeVisited == nil || len(l.nodeVisited) < len(theNodes) {
		l.nodeVisited = make([]bool, len(theNodes))
	} else {
		clear(l.nodeVisited)
	}
	l.componentNodes = nil

	// re-index nodes
	counter := 0
	for _, node := range theNodes {
		node.ID = counter
		counter++
	}
	// determine connected components
	var components [][]*lgraph.LNode
	for _, node := range theNodes {
		if !l.nodeVisited[node.ID] {
			l.connectedComponentsDFS(node)
			// connected component with the most nodes should be layered first
			// to guarantee reusability of attribute instances
			if len(components) == 0 || len(components[0]) < len(l.componentNodes) {
				components = append(components, nil)
				copy(components[1:], components)
				components[0] = l.componentNodes
			} else {
				components = append(components, l.componentNodes)
			}
			l.componentNodes = nil
		}
	}
	return components
}

// connectedComponentsDFS adds the node and each node that a path connects to
// it to componentNodes.
func (l *NetworkSimplexLayerer) connectedComponentsDFS(node *lgraph.LNode) {
	l.nodeVisited[node.ID] = true

	// node is part of the current connected component
	l.componentNodes = append(l.componentNodes, node)

	// continue with next nodes, if not already visited
	for _, port := range node.Ports {
		// port.getConnectedEdges(): the incoming edges, then the outgoing
		// edges.
		for pass := 0; pass < 2; pass++ {
			edges := port.IncomingEdges
			if pass == 1 {
				edges = port.OutgoingEdges
			}
			for _, edge := range edges {
				opposite := getOpposite(port, edge).Node()
				if opposite == nil || opposite.ID < 0 || opposite.ID >= len(l.nodeVisited) {
					// The JavaScript code extends its array for such a node.
					lgraph.Bail("NetworkSimplexLayerer: an edge leads to a node that is not a layerless node of the graph")
				}
				if !l.nodeVisited[opposite.ID] {
					l.connectedComponentsDFS(opposite)
				}
			}
		}
	}
}

// initialize makes the graph of the network simplex algorithm for the nodes
// of one connected component.
func (l *NetworkSimplexLayerer) initialize(theNodes []*lgraph.LNode) *networksimplex.NGraph {
	nodeMap := make(map[*lgraph.LNode]*networksimplex.NNode, len(theNodes))

	// transform nodes
	graph := networksimplex.NewNGraph()
	for _, lNode := range theNodes {
		nNode := networksimplex.NNodeOf().
			Origin(lNode).
			Create(graph)
		nodeMap[lNode] = nNode
	}

	// transform edges
	for _, lNode := range theNodes {
		for _, port := range lNode.Ports {
			for _, lEdge := range port.OutgoingEdges {
				// ignore self-loops
				if lEdge.IsSelfLoop() {
					continue
				}

				networksimplex.NEdgeOf().
					Weight(float64(1 * max(1, prop.Get(lEdge, lopts.PriorityShortness)))).
					Delta(1).
					Source(nodeMap[lEdge.Source.Node()]).
					Target(nodeMap[lEdge.Target.Node()]).
					Create()
			}
		}
	}

	return graph
}

// dispose releases the data of one run.
func (l *NetworkSimplexLayerer) dispose() {
	l.componentNodes = nil
	l.layeredGraph = nil
	l.nodeVisited = nil
}

// Process is process(theLayeredGraph, monitor).
func (l *NetworkSimplexLayerer) Process(theLayeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	l.layeredGraph = theLayeredGraph
	thoroughness := prop.Get(theLayeredGraph, lopts.Thoroughness) * iterLimitFactor

	theNodes := l.layeredGraph.LayerlessNodes
	if len(theNodes) < 1 {
		return
	}

	// layer graph, each connected component separately
	connectedComponents := l.connectedComponents(theNodes)
	var previousLayeringNodeCounts []int
	for _, connComp := range connectedComponents {
		// determine a limit on the number of iterations
		iterLimit := thoroughness * gwt.RoundInt(math.Sqrt(float64(len(connComp))))

		graph := l.initialize(connComp)

		// execute the network simplex algorithm on the (sub-)graph
		networksimplex.ForGraph(graph).WithIterationLimit(iterLimit).
			WithPreviousLayering(previousLayeringNodeCounts).
			WithBalancing(true).
			Execute()

		// the layers are store in the NNode's layer field.
		for _, nNode := range graph.Nodes {
			// add additional layers to match required number
			for len(l.layeredGraph.Layers) <= nNode.Layer {
				l.layeredGraph.Layers = append(l.layeredGraph.Layers, lgraph.NewLayer(l.layeredGraph))
			}
			lNode := nNode.Origin.(*lgraph.LNode)
			lNode.SetLayer(l.layeredGraph.Layers[nNode.Layer])
		}

		if len(connectedComponents) > 1 {
			previousLayeringNodeCounts = make([]int, len(l.layeredGraph.Layers))
			layerIdx := 0
			for _, layer := range l.layeredGraph.Layers {
				previousLayeringNodeCounts[layerIdx] = len(layer.Nodes)
				layerIdx++
			}
		}
	}

	// empty the list of unlayered nodes
	l.layeredGraph.LayerlessNodes = theNodes[:0]

	// release the created resources
	l.dispose()
}

// getOpposite returns the port at the other end of the edge. Java throws
// when the edge is not connected to the port.
func getOpposite(port *lgraph.LPort, edge *lgraph.LEdge) *lgraph.LPort {
	if edge.Source == port {
		return edge.Target
	} else if edge.Target == port {
		return edge.Source
	}
	lgraph.Bail("NetworkSimplexLayerer: Input edge is not connected to the input port.")
	return nil
}
