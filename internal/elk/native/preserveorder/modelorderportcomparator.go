// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/preserveorder/ModelOrderPortComparator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package preserveorder

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// ModelOrderPortComparator is the class ModelOrderPortComparator: it orders
// the ports of one node by the model order of their edges, or by the order
// of the nodes that they connect to in the previous layer. Ports that
// connect to the same long edge target are ordered next to each other.
// Outgoing ports are ordered before incoming ports.
//
// The comparator keeps the results of earlier comparisons, so one comparator
// serves one sort.
type ModelOrderPortComparator struct {
	// targetNodeModelOrder has the model order of each target node of the
	// node. It can be nil.
	targetNodeModelOrder *gwt.Map[*lgraph.LNode, int]
	// portModelOrder tells that ports have a model order.
	portModelOrder bool
	// previousLayer has the nodes of the previous layer.
	previousLayer []*lgraph.LNode
	strategy      lenum.OrderingStrategy
	// biggerThan has, for each port, the ports that it is bigger than.
	biggerThan *gwt.Map[*lgraph.LPort, *gwt.Set[*lgraph.LPort]]
	// smallerThan has, for each port, the ports that it is smaller than.
	smallerThan *gwt.Map[*lgraph.LPort, *gwt.Set[*lgraph.LPort]]
}

// NewModelOrderPortComparator is new ModelOrderPortComparator(Layer,
// OrderingStrategy, Map, boolean). It copies the node list of the layer.
func NewModelOrderPortComparator(previousLayer *lgraph.Layer, strategy lenum.OrderingStrategy,
	targetNodeModelOrder *gwt.Map[*lgraph.LNode, int], portModelOrder bool) *ModelOrderPortComparator {
	return NewModelOrderPortComparatorOfNodes(append([]*lgraph.LNode(nil), previousLayer.Nodes...), strategy,
		targetNodeModelOrder, portModelOrder)
}

// NewModelOrderPortComparatorOfNodes is new ModelOrderPortComparator(LNode[],
// OrderingStrategy, Map, boolean). It keeps the slice.
func NewModelOrderPortComparatorOfNodes(previousLayer []*lgraph.LNode, strategy lenum.OrderingStrategy,
	targetNodeModelOrder *gwt.Map[*lgraph.LNode, int], portModelOrder bool) *ModelOrderPortComparator {
	return &ModelOrderPortComparator{
		targetNodeModelOrder: targetNodeModelOrder,
		portModelOrder:       portModelOrder,
		previousLayer:        previousLayer,
		strategy:             strategy,
		biggerThan:           gwt.NewMap[*lgraph.LPort, *gwt.Set[*lgraph.LPort]](),
		smallerThan:          gwt.NewMap[*lgraph.LPort, *gwt.Set[*lgraph.LPort]](),
	}
}

// Compare is compare(originalP1, originalP2).
func (c *ModelOrderPortComparator) Compare(originalP1, originalP2 *lgraph.LPort) int {
	p1 := originalP1
	p2 := originalP2

	if c.portModelOrder && p1.Side == core.PortSideWest && p2.Side == core.PortSideWest {
		// Both nodes have the same port side. Hence I need to determine
		// their ordering based on the side. WEST sort descending. EAST,
		// NORTH, SOUTH sort ascending. Hence I exchange p1 and p2 in the
		// WEST case.
		p1, p2 = p2, p1
	}
	if !c.biggerThan.ContainsKey(p1) {
		c.biggerThan.Put(p1, gwt.NewSet[*lgraph.LPort]())
	} else if c.biggerThan.Get(p1).Contains(p2) {
		return 1
	}
	if !c.biggerThan.ContainsKey(p2) {
		c.biggerThan.Put(p2, gwt.NewSet[*lgraph.LPort]())
	} else if c.biggerThan.Get(p2).Contains(p1) {
		return -1
	}
	if !c.smallerThan.ContainsKey(p1) {
		c.smallerThan.Put(p1, gwt.NewSet[*lgraph.LPort]())
	} else if c.smallerThan.Get(p1).Contains(p2) {
		return -1
	}
	if !c.smallerThan.ContainsKey(p2) {
		c.smallerThan.Put(p2, gwt.NewSet[*lgraph.LPort]())
	} else if c.biggerThan.Get(p2).Contains(p1) {
		// The Java code reads biggerThan here too.
		return 1
	}
	// Sort nodes by their ports sides NORTH < EAST < SOUTH < WEST.
	if p1.Side != p2.Side {
		result := gwt.IntCompare(int(p1.Side), int(p2.Side))

		if result == -1 {
			c.updateBiggerAndSmallerAssociations(p2, p1)
		} else {
			c.updateBiggerAndSmallerAssociations(p1, p2)
		}
		return result
	}

	// Sort incoming edges by sorting their ports by the order of the nodes
	// they connect to.
	if len(p1.IncomingEdges) != 0 && len(p2.IncomingEdges) != 0 {
		// If port order is used instead of edge order, consult it to make
		// decisions. If not both ports have a model order fall back to the
		// edge order.
		if c.portModelOrder {
			result := c.CheckPortModelOrder(p1, p2)
			if result != 0 {
				if result == -1 {
					c.updateBiggerAndSmallerAssociations(p2, p1)
				} else if result == 1 {
					c.updateBiggerAndSmallerAssociations(p1, p2)
				}
				return result
			}
		}
		p1Node := p1.IncomingEdges[0].Source.Node()
		p2Node := p2.IncomingEdges[0].Source.Node()
		if p1Node == p2Node {
			p1MO := modelOrder(p1.IncomingEdges[0])
			p2MO := modelOrder(p2.IncomingEdges[0])
			if p1MO > p2MO {
				c.updateBiggerAndSmallerAssociations(p1, p2)
			} else {
				c.updateBiggerAndSmallerAssociations(p2, p1)
			}
			// In this case both incoming edges must have a model order set.
			// Check it.
			return gwt.IntCompare(p1MO, p2MO)
		}
		for _, previousNode := range c.previousLayer {
			if previousNode == p1Node {
				c.updateBiggerAndSmallerAssociations(p1, p2)
				return 1
			} else if previousNode == p2Node {
				c.updateBiggerAndSmallerAssociations(p2, p1)
				return -1
			}
		}
	}

	// Sort outgoing edges by sorting their ports based on the model order of
	// their edges.
	if len(p1.OutgoingEdges) != 0 && len(p2.OutgoingEdges) != 0 {
		p1TargetNode := prop.Get(p1, iprops.LongEdgeTargetNode)
		p2TargetNode := prop.Get(p2, iprops.LongEdgeTargetNode)

		// If node order is preferred and both edges have a target node use
		// the model order of the long edge target.
		if c.strategy == lenum.OrderingStrategyPreferNodes && p1TargetNode != nil && p2TargetNode != nil &&
			prop.Has(p1TargetNode, iprops.ModelOrder) && prop.Has(p2TargetNode, iprops.ModelOrder) {
			p1MO := modelOrder(p1TargetNode)
			p2MO := modelOrder(p2TargetNode)
			if p1MO > p2MO {
				c.updateBiggerAndSmallerAssociations(p1, p2)
			} else {
				c.updateBiggerAndSmallerAssociations(p2, p1)
			}
			return gwt.IntCompare(p1MO, p2MO)
		}

		// If port order is used instead of edge order, consult it to make
		// decisions. If not both ports have a model order fall back to the
		// edge order.
		if c.portModelOrder {
			result := c.CheckPortModelOrder(p1, p2)
			if result != 0 {
				if result == -1 {
					c.updateBiggerAndSmallerAssociations(p2, p1)
				} else if result == 1 {
					c.updateBiggerAndSmallerAssociations(p1, p2)
				}
				return result
			}
		}

		p1Order := 0
		p2Order := 0
		if prop.Has(p1.OutgoingEdges[0], iprops.ModelOrder) {
			p1Order = modelOrder(p1.OutgoingEdges[0])
		}
		if prop.Has(p2.OutgoingEdges[0], iprops.ModelOrder) {
			// The Java code reads the edge of p1 here too.
			p2Order = modelOrder(p1.OutgoingEdges[0])
		}

		// Same target node
		if p1TargetNode != nil && p1TargetNode == p2TargetNode {
			// Backward edges below
			if prop.Get(p1.OutgoingEdges[0], iprops.Reversed) && !prop.Get(p2.OutgoingEdges[0], iprops.Reversed) {
				c.updateBiggerAndSmallerAssociations(p1, p2)
				return 1
			} else if !prop.Get(p1.OutgoingEdges[0], iprops.Reversed) && prop.Get(p2.OutgoingEdges[0], iprops.Reversed) {
				c.updateBiggerAndSmallerAssociations(p2, p1)
				return -1
			}
			if p1Order > p2Order {
				c.updateBiggerAndSmallerAssociations(p1, p2)
			} else {
				c.updateBiggerAndSmallerAssociations(p2, p1)
			}
			return gwt.IntCompare(p1Order, p2Order)
		}
		if c.targetNodeModelOrder != nil {
			// A map of GWT does not have the key null.
			if p1TargetNode != nil && c.targetNodeModelOrder.ContainsKey(p1TargetNode) {
				p1Order = c.targetNodeModelOrder.Get(p1TargetNode)
			}
			if p2TargetNode != nil && c.targetNodeModelOrder.ContainsKey(p2TargetNode) {
				p2Order = c.targetNodeModelOrder.Get(p2TargetNode)
			}
		}
		if p1Order > p2Order {
			c.updateBiggerAndSmallerAssociations(p1, p2)
		} else {
			c.updateBiggerAndSmallerAssociations(p2, p1)
		}
		return gwt.IntCompare(p1Order, p2Order)
	}
	// Sort outgoing ports before incoming ports.
	if len(p1.IncomingEdges) != 0 && len(p2.OutgoingEdges) != 0 {
		c.updateBiggerAndSmallerAssociations(p1, p2)
		return 1
	} else if len(p1.OutgoingEdges) != 0 && len(p2.IncomingEdges) != 0 {
		c.updateBiggerAndSmallerAssociations(p2, p1)
		return -1
	} else if prop.Has(p1, iprops.ModelOrder) && prop.Has(p2, iprops.ModelOrder) {
		// The ports have no edges. Use the port model order to compare them.
		p1MO := modelOrder(p1)
		p2MO := modelOrder(p2)
		if p1MO > p2MO {
			c.updateBiggerAndSmallerAssociations(p1, p2)
		} else {
			c.updateBiggerAndSmallerAssociations(p2, p1)
		}
		return gwt.IntCompare(p1MO, p2MO)
	}
	c.updateBiggerAndSmallerAssociations(p2, p1)
	return -1
}

// CheckPortModelOrder is checkPortModelOrder(p1, p2): it compares two ports
// by their model order. It returns 0 when a port has no model order.
func (c *ModelOrderPortComparator) CheckPortModelOrder(p1, p2 *lgraph.LPort) int {
	if prop.Has(p1, iprops.ModelOrder) && prop.Has(p2, iprops.ModelOrder) {
		return gwt.IntCompare(modelOrder(p1), modelOrder(p2))
	}
	return 0
}

func (c *ModelOrderPortComparator) updateBiggerAndSmallerAssociations(bigger, smaller *lgraph.LPort) {
	UpdateBiggerAndSmallerAssociations(c.biggerThan, c.smallerThan, bigger, smaller)
}
