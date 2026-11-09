// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p1cycles/GreedyModelOrderCycleBreaker.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p1cycles

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("GreedyModelOrderCycleBreaker", func() lgraph.Processor { return NewGreedyModelOrderCycleBreaker() })
}

// GreedyModelOrderCycleBreaker is the phase GreedyModelOrderCycleBreaker: a
// GreedyCycleBreaker that selects the node with the lowest model order
// instead of a random node.
type GreedyModelOrderCycleBreaker struct {
	GreedyCycleBreaker
}

// NewGreedyModelOrderCycleBreaker is new GreedyModelOrderCycleBreaker().
func NewGreedyModelOrderCycleBreaker() *GreedyModelOrderCycleBreaker {
	b := &GreedyModelOrderCycleBreaker{}
	b.chooser = b.chooseNodeWithMaxOutflow
	return b
}

// chooseNodeWithMaxOutflow is chooseNodeWithMaxOutflow(nodes): the node with
// the lowest model order, or a random node when no node has a model order.
func (b *GreedyModelOrderCycleBreaker) chooseNodeWithMaxOutflow(nodes []*lgraph.LNode) *lgraph.LNode {
	var returnNode *lgraph.LNode
	minimumModelOrder := math.MaxInt32
	for _, node := range nodes {
		// In this step nodes without a model order are disregarded.
		// One could of course think of a different strategy regarding this
		// aspect.
		if prop.Has(node, iprops.ModelOrder) && prop.Get(node, iprops.ModelOrder) < minimumModelOrder {
			minimumModelOrder = prop.Get(node, iprops.ModelOrder)
			returnNode = node
		}
	}
	if returnNode == nil {
		return b.GreedyCycleBreaker.chooseNodeWithMaxOutflow(nodes)
	}
	return returnNode
}
