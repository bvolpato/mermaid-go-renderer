// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/preserveorder/ModelOrderNodeComparator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package preserveorder has the comparators that order nodes and ports by
// the order of the input model, and the processor SortByInputModelProcessor
// that sorts a layered graph with them.
package preserveorder

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// modelOrder is getProperty(MODEL_ORDER) where the Java code unboxes the
// value. elkjs throws for an element without a model order, so the layout
// stops here.
func modelOrder(h prop.Holder) int {
	v, ok := prop.Lookup(h, iprops.ModelOrder)
	if !ok {
		lgraph.Bail("preserveorder: an element without a model order is compared by model order")
	}
	return v
}

// returnValue is LongEdgeOrderingStrategy.returnValue().
func returnValue(s lenum.LongEdgeOrderingStrategy) int {
	switch s {
	case lenum.LongEdgeOrderingStrategyDummyNodeOver:
		return 2147483647
	case lenum.LongEdgeOrderingStrategyDummyNodeUnder:
		return -1
	}
	return 0
}

// ModelOrderNodeComparator is the class ModelOrderNodeComparator: it orders
// the nodes of one layer by their model order, or by the order of the nodes
// that they connect to in the previous layer.
//
// The comparator keeps the results of earlier comparisons, so one comparator
// serves one sort.
type ModelOrderNodeComparator struct {
	// previousLayer has the nodes of the previous layer.
	previousLayer []*lgraph.LNode
	// orderingStrategy is the ordering strategy.
	orderingStrategy lenum.OrderingStrategy
	// biggerThan has, for each node, the nodes that it is bigger than.
	biggerThan *gwt.Map[*lgraph.LNode, *gwt.Set[*lgraph.LNode]]
	// smallerThan has, for each node, the nodes that it is smaller than.
	smallerThan *gwt.Map[*lgraph.LNode, *gwt.Set[*lgraph.LNode]]
	// longEdgeNodeOrder is the strategy for a dummy node that is compared
	// to a node without a connection to the previous layer.
	longEdgeNodeOrder lenum.LongEdgeOrderingStrategy
}

// NewModelOrderNodeComparator is new ModelOrderNodeComparator(Layer,
// OrderingStrategy, LongEdgeOrderingStrategy). It copies the node list of
// the layer.
func NewModelOrderNodeComparator(thePreviousLayer *lgraph.Layer, orderingStrategy lenum.OrderingStrategy,
	longEdgeOrderingStrategy lenum.LongEdgeOrderingStrategy) *ModelOrderNodeComparator {
	return NewModelOrderNodeComparatorOfNodes(append([]*lgraph.LNode(nil), thePreviousLayer.Nodes...),
		orderingStrategy, longEdgeOrderingStrategy)
}

// NewModelOrderNodeComparatorOfNodes is new ModelOrderNodeComparator(LNode[],
// OrderingStrategy, LongEdgeOrderingStrategy). It keeps the slice.
func NewModelOrderNodeComparatorOfNodes(previousLayer []*lgraph.LNode, orderingStrategy lenum.OrderingStrategy,
	longEdgeOrderingStrategy lenum.LongEdgeOrderingStrategy) *ModelOrderNodeComparator {
	return &ModelOrderNodeComparator{
		previousLayer:     previousLayer,
		orderingStrategy:  orderingStrategy,
		biggerThan:        gwt.NewMap[*lgraph.LNode, *gwt.Set[*lgraph.LNode]](),
		smallerThan:       gwt.NewMap[*lgraph.LNode, *gwt.Set[*lgraph.LNode]](),
		longEdgeNodeOrder: longEdgeOrderingStrategy,
	}
}

// Compare is compare(n1, n2).
func (c *ModelOrderNodeComparator) Compare(n1, n2 *lgraph.LNode) int {
	if !c.biggerThan.ContainsKey(n1) {
		c.biggerThan.Put(n1, gwt.NewSet[*lgraph.LNode]())
	} else if c.biggerThan.Get(n1).Contains(n2) {
		return 1
	}
	if !c.biggerThan.ContainsKey(n2) {
		c.biggerThan.Put(n2, gwt.NewSet[*lgraph.LNode]())
	} else if c.biggerThan.Get(n2).Contains(n1) {
		return -1
	}
	if !c.smallerThan.ContainsKey(n1) {
		c.smallerThan.Put(n1, gwt.NewSet[*lgraph.LNode]())
	} else if c.smallerThan.Get(n1).Contains(n2) {
		return -1
	}
	if !c.smallerThan.ContainsKey(n2) {
		c.smallerThan.Put(n2, gwt.NewSet[*lgraph.LNode]())
	} else if c.biggerThan.Get(n2).Contains(n1) {
		// The Java code reads biggerThan here too.
		return 1
	}
	// If no model order is set, the one node is a dummy node and the nodes
	// should be ordered by the connected edges. This kind of ordering should
	// be preferred, if the order of the edges has priority.
	if c.orderingStrategy == lenum.OrderingStrategyPreferEdges || !prop.Has(n1, iprops.ModelOrder) ||
		!prop.Has(n2, iprops.ModelOrder) {
		// In this case the order of the connected nodes in the previous
		// layer should be respected
		var p1SourcePort *lgraph.LPort
		for _, p := range n1.Ports {
			// Get the last port that actually connects to a previous layer.
			if len(p.IncomingEdges) != 0 {
				if p.IncomingEdges[0].Source.Node().Layer() != n1.Layer() {
					p1SourcePort = p.IncomingEdges[0].Source
				}
			}
		}

		var p2SourcePort *lgraph.LPort
		for _, p := range n2.Ports {
			if len(p.IncomingEdges) != 0 {
				if p.IncomingEdges[0].Source.Node().Layer() != n2.Layer() {
					p2SourcePort = p.IncomingEdges[0].Source
				}
			}
		}

		// Case both nodes have connections to the previous layer.
		if p1SourcePort != nil && p2SourcePort != nil {
			p1Node := p1SourcePort.Node()
			p2Node := p2SourcePort.Node()

			// If both nodes connect to the same node the order of their
			// corresponding ports in the previous layer should be used to
			// order them.
			if p1Node != nil && p1Node == p2Node {
				// We are not allowed to look at the model order of the edges
				// but we have to look at the actual port ordering.
				for _, port := range p1Node.Ports {
					if port == p1SourcePort {
						// Case the port is the one connecting to n1,
						// therefore, n1 has a smaller model order
						c.updateBiggerAndSmallerAssociations(n2, n1)
						return -1
					} else if port == p2SourcePort {
						// Case the port is the one connecting to n2,
						// therefore, n1 has a bigger model order
						c.updateBiggerAndSmallerAssociations(n1, n2)
						return 1
					}
				}
				// Cannot happen, since both nodes have a connection to the
				// previous layer.
				return gwt.IntCompare(c.getModelOrderFromConnectedEdges(n1), c.getModelOrderFromConnectedEdges(n2))
			}

			// Else the nodes are ordered by the nodes they connect to. One
			// can disregard the model order here since the ordering in the
			// previous layer does already reflect it.
			for _, previousNode := range c.previousLayer {
				if previousNode == p1Node {
					c.updateBiggerAndSmallerAssociations(n2, n1)
					return -1
				} else if previousNode == p2Node {
					c.updateBiggerAndSmallerAssociations(n1, n2)
					return 1
				}
			}
		}

		// One node has no source port
		if !prop.Has(n1, iprops.ModelOrder) || !prop.Has(n2, iprops.ModelOrder) {
			n1ModelOrder := c.getModelOrderFromConnectedEdges(n1)
			n2ModelOrder := c.getModelOrderFromConnectedEdges(n2)
			if n1ModelOrder > n2ModelOrder {
				c.updateBiggerAndSmallerAssociations(n1, n2)
			} else {
				c.updateBiggerAndSmallerAssociations(n2, n1)
			}
			return gwt.IntCompare(n1ModelOrder, n2ModelOrder)
		}
		// Fall through case. Both nodes are not connected to the previous
		// layer. Therefore, they must be normal nodes. The model order shall
		// be used to order them.
	}
	// Order nodes by their order in the model.
	n1ModelOrder := modelOrder(n1)
	n2ModelOrder := modelOrder(n2)
	if n1ModelOrder > n2ModelOrder {
		c.updateBiggerAndSmallerAssociations(n1, n2)
	} else {
		c.updateBiggerAndSmallerAssociations(n2, n1)
	}
	return gwt.IntCompare(n1ModelOrder, n2ModelOrder)
}

// getModelOrderFromConnectedEdges returns the model order of the first
// incoming edge of the first port of n that has incoming edges. Without such
// an edge it returns the value of the long edge strategy.
func (c *ModelOrderNodeComparator) getModelOrderFromConnectedEdges(n *lgraph.LNode) int {
	var sourcePort *lgraph.LPort
	for _, p := range n.Ports {
		if len(p.IncomingEdges) != 0 {
			sourcePort = p
			break
		}
	}
	if sourcePort != nil {
		edge := sourcePort.IncomingEdges[0]
		if edge != nil {
			return modelOrder(edge)
		}
	}
	// Set to -1 to sort dummy nodes under nodes without a connection to the
	// previous layer. Set to MAX_INT to sort dummy nodes over nodes without a
	// connection to the previous layer. Set to 0 if you do not care about
	// their order.
	return returnValue(c.longEdgeNodeOrder)
}

func (c *ModelOrderNodeComparator) updateBiggerAndSmallerAssociations(bigger, smaller *lgraph.LNode) {
	UpdateBiggerAndSmallerAssociations(c.biggerThan, c.smallerThan, bigger, smaller)
}

// UpdateBiggerAndSmallerAssociations is the method
// updateBiggerAndSmallerAssociations of ModelOrderNodeComparator,
// ModelOrderPortComparator and ModelOrderBarycenterHeuristic: it records
// that bigger is bigger than smaller, with the transitive results.
//
// The sets are walked as the iterators of GWT walk them: an element that the
// walk adds to the walked set is visited too.
func UpdateBiggerAndSmallerAssociations[T comparable](biggerThan, smallerThan *gwt.Map[T, *gwt.Set[T]], bigger, smaller T) {
	biggerNodeBiggerThan := biggerThan.Get(bigger)
	smallerNodeBiggerThan := biggerThan.Get(smaller)
	biggerNodeSmallerThan := smallerThan.Get(bigger)
	smallerNodeSmallerThan := smallerThan.Get(smaller)
	biggerNodeBiggerThan.Add(smaller)
	smallerNodeSmallerThan.Add(bigger)
	smallerNodeBiggerThan.Each(func(verySmall T) bool {
		biggerNodeBiggerThan.Add(verySmall)
		smallerThan.Get(verySmall).Add(bigger)
		addAll(smallerThan.Get(verySmall), biggerNodeSmallerThan)
		return true
	})

	biggerNodeSmallerThan.Each(func(veryBig T) bool {
		smallerNodeSmallerThan.Add(veryBig)
		biggerThan.Get(veryBig).Add(smaller)
		addAll(biggerThan.Get(veryBig), smallerNodeBiggerThan)
		return true
	})
}

// addAll is dst.addAll(src).
func addAll[T comparable](dst, src *gwt.Set[T]) {
	src.Each(func(v T) bool {
		dst.Add(v)
		return true
	})
}
