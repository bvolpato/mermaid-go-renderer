// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/networksimplex/NEdge.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package networksimplex

import (
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// NEdge is the class NEdge: an edge of the graph of the network simplex
// algorithm. It has a source node, a target node, a weight and a minimum
// length (Delta).
//
// reverse() of Java is not ported: elkjs does not have it, so no reference
// result exists for it.
type NEdge struct {
	// ID is the public field id. The algorithm does not use it.
	ID int
	// internalID is the index of the edge in the arrays of the algorithm.
	internalID int
	// Origin is the object that the edge was made for.
	Origin any
	// Source is the source node.
	Source *NNode
	// Target is the target node.
	Target *NNode
	// Weight is the weight of the edge.
	Weight float64
	// Delta is the minimum length of the edge. A new edge has the length 1.
	Delta int

	// treeEdge tells that the edge is in the spanning tree.
	treeEdge bool
}

// NEdgeOf is NEdge.of(): a builder for a new edge. NEdge.of(origin) of Java
// drops its argument, so it is NEdgeOf too.
func NEdgeOf() *NEdgeBuilder {
	return &NEdgeBuilder{edge: &NEdge{Delta: 1}}
}

// GetOther is getOther(some): the end of the edge that is not some. Java
// throws when some is not an end of the edge.
func (e *NEdge) GetOther(some *NNode) *NNode {
	if some == e.Source {
		return e.Target
	} else if some == e.Target {
		return e.Source
	}
	lgraph.Bail("network simplex: a node is not part of the edge %s", e)
	return nil
}

// String is toString().
func (e *NEdge) String() string {
	return "NEdge[id=" + strconv.Itoa(e.ID) + " w=" + js.Str(e.Weight) + " d=" + strconv.Itoa(e.Delta) + "]"
}

// NEdgeBuilder is the class NEdge.NEdgeBuilder.
type NEdgeBuilder struct {
	edge *NEdge
}

// ID is id(id): it sets the field ID.
func (b *NEdgeBuilder) ID(id int) *NEdgeBuilder {
	b.edge.ID = id
	return b
}

// Origin is origin(origin): it sets the field Origin.
func (b *NEdgeBuilder) Origin(origin any) *NEdgeBuilder {
	b.edge.Origin = origin
	return b
}

// Weight is weight(weight): it sets the weight of the edge.
func (b *NEdgeBuilder) Weight(weight float64) *NEdgeBuilder {
	b.edge.Weight = weight
	return b
}

// Delta is delta(delta): it sets the minimum length of the edge.
func (b *NEdgeBuilder) Delta(delta int) *NEdgeBuilder {
	b.edge.Delta = delta
	return b
}

// Source is source(source): it sets the source node.
func (b *NEdgeBuilder) Source(source *NNode) *NEdgeBuilder {
	b.edge.Source = source
	return b
}

// Target is target(target): it sets the target node.
func (b *NEdgeBuilder) Target(target *NNode) *NEdgeBuilder {
	b.edge.Target = target
	return b
}

// Create is create(): it adds the edge to the outgoing edges of the source
// and to the incoming edges of the target, and returns it. Java throws for an
// edge without a source or a target, and for a self loop.
func (b *NEdgeBuilder) Create() *NEdge {
	if b.edge.Source == nil || b.edge.Target == nil {
		lgraph.Bail("network simplex: NEdge must have a source and target NNode specified.")
	}
	if b.edge.Source == b.edge.Target {
		lgraph.Bail("network simplex: Network simplex does not support self-loops: %s", b.edge)
	}

	b.edge.Source.outgoingEdges.add(b.edge)
	b.edge.Target.incomingEdges.add(b.edge)

	return b.edge
}
