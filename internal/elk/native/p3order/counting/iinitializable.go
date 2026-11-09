// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/counting/IInitializable.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package counting has the crossing counters of the layer sweep crossing
// minimizer, and the interface that initializes the helper objects of the
// minimizer in one walk over a graph.
package counting

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"

// IInitializable is the interface IInitializable: an object whose data is
// set up before the layer sweep runs. Init walks the layers, nodes, ports
// and edges in the input order and calls the methods at each level.
//
// The Java interface has empty default methods. Embed DefaultInit to get
// them.
type IInitializable interface {
	// InitAtLayerLevel is initAtLayerLevel(l, nodeOrder).
	InitAtLayerLevel(l int, nodeOrder [][]*lgraph.LNode)
	// InitAtNodeLevel is initAtNodeLevel(l, n, nodeOrder).
	InitAtNodeLevel(l, n int, nodeOrder [][]*lgraph.LNode)
	// InitAtPortLevel is initAtPortLevel(l, n, p, nodeOrder).
	InitAtPortLevel(l, n, p int, nodeOrder [][]*lgraph.LNode)
	// InitAtEdgeLevel is initAtEdgeLevel(l, n, p, e, edge, nodeOrder).
	InitAtEdgeLevel(l, n, p, e int, edge *lgraph.LEdge, nodeOrder [][]*lgraph.LNode)
	// InitAfterTraversal is initAfterTraversal().
	InitAfterTraversal()
}

// DefaultInit has the empty default methods of IInitializable.
type DefaultInit struct{}

// InitAtLayerLevel does nothing.
func (DefaultInit) InitAtLayerLevel(l int, nodeOrder [][]*lgraph.LNode) {}

// InitAtNodeLevel does nothing.
func (DefaultInit) InitAtNodeLevel(l, n int, nodeOrder [][]*lgraph.LNode) {}

// InitAtPortLevel does nothing.
func (DefaultInit) InitAtPortLevel(l, n, p int, nodeOrder [][]*lgraph.LNode) {}

// InitAtEdgeLevel does nothing.
func (DefaultInit) InitAtEdgeLevel(l, n, p, e int, edge *lgraph.LEdge, nodeOrder [][]*lgraph.LNode) {}

// InitAfterTraversal does nothing.
func (DefaultInit) InitAfterTraversal() {}

// Init is IInitializable.init(initializables, order): it walks the graph
// and calls the methods of each object at each level.
func Init(initializables []IInitializable, order [][]*lgraph.LNode) {
	for l := 0; l < len(order); l++ {
		for _, i := range initializables {
			i.InitAtLayerLevel(l, order)
		}
		for n := 0; n < len(order[l]); n++ {
			for _, i := range initializables {
				i.InitAtNodeLevel(l, n, order)
			}
			node := order[l][n]
			for p := 0; p < len(node.Ports); p++ {
				for _, i := range initializables {
					i.InitAtPortLevel(l, n, p, order)
				}
				port := node.Ports[p]
				e := 0
				for _, edge := range port.ConnectedEdges() {
					for _, i := range initializables {
						i.InitAtEdgeLevel(l, n, p, e, edge, order)
						e++
					}
				}
			}
		}
	}
	for _, i := range initializables {
		i.InitAfterTraversal()
	}
}
