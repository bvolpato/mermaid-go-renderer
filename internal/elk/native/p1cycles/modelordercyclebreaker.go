// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p1cycles/ModelOrderCycleBreaker.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p1cycles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("ModelOrderCycleBreaker", func() lgraph.Processor { return NewModelOrderCycleBreaker() })
}

// ModelOrderCycleBreaker is the phase ModelOrderCycleBreaker: it reverses
// each edge whose target has a lower model order than its source.
type ModelOrderCycleBreaker struct {
	firstSeparateModelOrder int
	lastSeparateModelOrder  int
}

// NewModelOrderCycleBreaker is new ModelOrderCycleBreaker().
func NewModelOrderCycleBreaker() *ModelOrderCycleBreaker { return &ModelOrderCycleBreaker{} }

// Process is process(layeredGraph, monitor).
func (b *ModelOrderCycleBreaker) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// Reset FIRST_SEPARATE and LAST_SEPARATE counters.
	b.firstSeparateModelOrder = 0
	b.lastSeparateModelOrder = 0

	// gather edges that point to the wrong direction
	var revEdges []*lgraph.LEdge

	// One needs an offset to make sure that the model order of nodes with
	// port constraints is always lower/higher than that of other nodes.
	// E.g. A node with the LAST constraint needs to have a model order m =
	// modelOrder + offset such that m > m(n) with m(n) being the model order
	// of a normal node n (without constraints).
	// Such that the highest model order has to be used as an offset
	offset := len(layeredGraph.LayerlessNodes)
	for _, node := range layeredGraph.LayerlessNodes {
		if prop.Has(node, iprops.ModelOrder) {
			offset = max(offset, prop.Get(node, iprops.ModelOrder)+1)
		}
	}

	for _, source := range layeredGraph.LayerlessNodes {
		modelOrderSource := b.computeConstraintModelOrder(source, offset)

		// source.getPorts(PortType.OUTPUT): the ports with outgoing edges.
		for _, port := range source.Ports {
			if len(port.OutgoingEdges) == 0 {
				continue
			}
			for _, edge := range port.OutgoingEdges {
				target := edge.Target.Node()
				modelOrderTarget := b.computeConstraintModelOrder(target, offset)
				if modelOrderTarget < modelOrderSource {
					revEdges = append(revEdges, edge)
				}
			}
		}
	}

	// reverse the gathered edges
	for _, edge := range revEdges {
		edge.Reverse(layeredGraph, true)
		prop.Set(layeredGraph, iprops.Cyclic, true)
	}
}

// computeConstraintModelOrder is computeConstraintModelOrder(node, offset):
// the model order of the node, moved by its layer constraint. Each call for
// a node with the constraint FIRST_SEPARATE or LAST_SEPARATE counts.
func (b *ModelOrderCycleBreaker) computeConstraintModelOrder(node *lgraph.LNode, offset int) int {
	modelOrder := 0
	switch prop.Get(node, lopts.LayeringLayerConstraint) {
	case lenum.LayerConstraintFirstSeparate:
		modelOrder = 2*-offset + b.firstSeparateModelOrder
		b.firstSeparateModelOrder++
	case lenum.LayerConstraintFirst:
		modelOrder = -offset
	case lenum.LayerConstraintLast:
		modelOrder = offset
	case lenum.LayerConstraintLastSeparate:
		modelOrder = 2*offset + b.lastSeparateModelOrder
		b.lastSeparateModelOrder++
	}
	if prop.Has(node, iprops.ModelOrder) {
		modelOrder += prop.Get(node, iprops.ModelOrder)
	}
	return modelOrder
}
