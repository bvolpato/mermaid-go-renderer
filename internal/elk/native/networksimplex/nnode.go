// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/networksimplex/NNode.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package networksimplex

// NNode is the class NNode: a node of the graph of the network simplex
// algorithm.
type NNode struct {
	// ID is the public field id. The algorithm does not use it.
	ID int
	// internalID is the index of the node in the arrays of the algorithm.
	internalID int
	// Origin is the object that the node was made for.
	Origin any
	// Type is a text for debugging. It has no meaning for the algorithm.
	Type string
	// Layer is the layer of the node. NetworkSimplex.Execute sets it. The
	// lowest layer of a graph is 0 when Execute returns.
	Layer int

	// The node caches the list of its incoming and outgoing edges together.
	// The two modification counts tell when the cache is out of date.
	outgoingEdges       changeAwareArrayList
	outgoingEdgesModCnt int
	incomingEdges       changeAwareArrayList
	incomingEdgesModCnt int

	// allEdges is the cached list of all edges.
	allEdges []*NEdge

	// treeNode tells that the node is in the spanning tree.
	treeNode bool
	// unknownCutvalues has the tree edges of the node whose cut values are
	// not known yet.
	unknownCutvalues []*NEdge
}

// NNodeOf is NNode.of(): a builder for a new node.
func NNodeOf() *NNodeBuilder {
	return &NNodeBuilder{node: &NNode{outgoingEdgesModCnt: -1, incomingEdgesModCnt: -1}}
}

// GetOutgoingEdges is getOutgoingEdges(). The caller must not change the
// result: the list changes only through NEdgeBuilder.Create and the
// algorithm.
func (n *NNode) GetOutgoingEdges() []*NEdge { return n.outgoingEdges.list }

// GetIncomingEdges is getIncomingEdges(). The caller must not change the
// result.
func (n *NNode) GetIncomingEdges() []*NEdge { return n.incomingEdges.list }

// GetConnectedEdges is getConnectedEdges(): the incoming edges, then the
// outgoing edges. The list is cached: a later call returns the same memory
// until the incoming or the outgoing edges change. The caller must not change
// the result.
func (n *NNode) GetConnectedEdges() []*NEdge {
	if n.incomingEdgesModCnt != n.incomingEdges.modCount || n.outgoingEdgesModCnt != n.outgoingEdges.modCount {
		n.allEdges = n.allEdges[:0]
		n.allEdges = append(n.allEdges, n.incomingEdges.list...)
		n.allEdges = append(n.allEdges, n.outgoingEdges.list...)

		n.incomingEdgesModCnt = n.incomingEdges.modCount
		n.outgoingEdgesModCnt = n.outgoingEdges.modCount
	}
	return n.allEdges
}

// NNodeBuilder is the class NNode.NNodeBuilder.
type NNodeBuilder struct {
	node *NNode
}

// ID is id(id): it sets the field ID.
func (b *NNodeBuilder) ID(id int) *NNodeBuilder {
	b.node.ID = id
	return b
}

// Origin is origin(origin): it sets the field Origin.
func (b *NNodeBuilder) Origin(origin any) *NNodeBuilder {
	b.node.Origin = origin
	return b
}

// Type is type(type): it sets the field Type.
func (b *NNodeBuilder) Type(nodeType string) *NNodeBuilder {
	b.node.Type = nodeType
	return b
}

// Create is create(graph): it adds the node to the graph and returns it.
func (b *NNodeBuilder) Create(graph *NGraph) *NNode {
	graph.Nodes = append(graph.Nodes, b.node)
	return b.node
}

// changeAwareArrayList is the class NNode.ChangeAwareArrayList: a list that
// counts its changes. It has the methods that the package calls.
type changeAwareArrayList struct {
	list     []*NEdge
	modCount int
}

// add is add(e).
func (l *changeAwareArrayList) add(e *NEdge) {
	l.modCount++
	l.list = append(l.list, e)
}

// remove is remove(o): it removes the first occurrence of the edge. The
// count changes also when the list does not have the edge.
func (l *changeAwareArrayList) remove(e *NEdge) bool {
	l.modCount++
	for i, x := range l.list {
		if x == e {
			copy(l.list[i:], l.list[i+1:])
			l.list[len(l.list)-1] = nil
			l.list = l.list[:len(l.list)-1]
			return true
		}
	}
	return false
}
