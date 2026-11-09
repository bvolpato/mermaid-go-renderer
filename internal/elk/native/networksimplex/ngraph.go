// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/networksimplex/NGraph.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package networksimplex

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"

// NGraph is the class NGraph: the graph of the network simplex algorithm. It
// has a list of nodes.
//
// writeDebugGraph and isAcyclic of Java are not ported: elkjs does not have
// them, so no reference result exists for them.
type NGraph struct {
	// Nodes has the nodes of the graph. NetworkSimplex.Execute changes the
	// order when it removes subtrees.
	Nodes []*NNode
}

// NewNGraph is new NGraph().
func NewNGraph() *NGraph { return &NGraph{} }

// MakeConnected is makeConnected(): for a graph that is not connected, it
// makes a new root node and connects it to one node of each connected
// component. It returns the root node, or nil for a connected graph.
func (g *NGraph) MakeConnected() *NNode {
	id := 0
	for _, n := range g.Nodes {
		n.internalID = id
		id++
	}
	ccRep := g.findConCompRepresentatives()
	var root *NNode
	if len(ccRep) > 1 {
		root = g.createArtificialRootAndConnect(ccRep)
	}
	return root
}

// createArtificialRootAndConnect makes a new node and connects it to each of
// the given nodes with an edge of weight 0 and length 0.
func (g *NGraph) createArtificialRootAndConnect(nodesToConnect []*NNode) *NNode {
	root := NNodeOf().Create(g)
	for _, src := range nodesToConnect {
		NEdgeOf().
			Delta(0).
			Weight(0).
			Source(root).
			Target(src).
			Create()
	}
	return root
}

// findConCompRepresentatives returns one node of each connected component.
func (g *NGraph) findConCompRepresentatives() []*NNode {
	var ccRep []*NNode
	mark := make([]bool, len(g.Nodes))
	for _, node := range g.Nodes {
		if !mark[node.internalID] {
			ccRep = append(ccRep, node)
			g.dfs(node, mark)
		}
	}
	return ccRep
}

// dfs marks each node that a path connects to the given node.
func (g *NGraph) dfs(node *NNode, mark []bool) {
	if node.internalID < 0 || node.internalID >= len(mark) {
		lgraph.Bail("network simplex: an edge leads to a node that is not in the graph")
	}
	if mark[node.internalID] {
		return
	}
	mark[node.internalID] = true
	for _, edge := range node.GetConnectedEdges() {
		other := edge.GetOther(node)
		g.dfs(other, mark)
	}
}
