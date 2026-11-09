// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/InLayerConstraintProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("InLayerConstraintProcessor", func() lgraph.Processor { return NewInLayerConstraintProcessor() })
}

// InLayerConstraintProcessor is the processor InLayerConstraintProcessor: it
// moves the nodes with the in-layer constraint TOP to the start of their
// layer, and the nodes with the constraint BOTTOM to the end.
type InLayerConstraintProcessor struct{}

// NewInLayerConstraintProcessor is new InLayerConstraintProcessor().
func NewInLayerConstraintProcessor() *InLayerConstraintProcessor {
	return &InLayerConstraintProcessor{}
}

// Process is process(layeredGraph, monitor).
func (*InLayerConstraintProcessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// Iterate through each layer
	for _, layer := range layeredGraph.Layers {
		// We'll go through the layer's nodes, remembering two things:
		//  1. Once we reach the first non-top-constrained node, we remember
		//     its index. Top-constrained nodes encountered afterwards must be
		//     inserted at that point.
		//  2. A list of bottom-constrained nodes we have encountered. They
		//     will afterwards be moved to the end of the list, keeping the
		//     order in which we've encountered them.

		topInsertionIndex := -1
		var bottomConstrainedNodes []*lgraph.LNode

		// Iterate through an array of its nodes
		nodes := append([]*lgraph.LNode(nil), layer.Nodes...)

		for i := 0; i < len(nodes); i++ {
			constraint := prop.Get(nodes[i], iprops.InLayerConstraint)

			if topInsertionIndex == -1 {
				// See if this node is the first non-top-constrained node
				if constraint != lenum.InLayerConstraintTop {
					topInsertionIndex = i
				}
			} else {
				// We have already encountered non-top-constrained nodes
				// before
				if constraint == lenum.InLayerConstraintTop {
					// Move the node to the top insertion point
					nodes[i].SetLayer(nil)
					nodes[i].SetLayerAt(topInsertionIndex, layer)
					topInsertionIndex++
				}
			}

			// Put BOTTOM-constrained nodes into the corresponding list
			if constraint == lenum.InLayerConstraintBottom {
				bottomConstrainedNodes = append(bottomConstrainedNodes, nodes[i])
			}
		}

		// Append the bottom-constrained nodes
		for _, node := range bottomConstrainedNodes {
			node.SetLayer(nil)
			node.SetLayer(layer)
		}
	}
}
