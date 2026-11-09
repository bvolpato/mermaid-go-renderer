// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/ReversedEdgeRestorer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("ReversedEdgeRestorer", func() lgraph.Processor { return NewReversedEdgeRestorer() })
}

// ReversedEdgeRestorer is the processor ReversedEdgeRestorer: it restores
// the direction of each edge that an earlier processor reversed.
type ReversedEdgeRestorer struct{}

// NewReversedEdgeRestorer is new ReversedEdgeRestorer().
func NewReversedEdgeRestorer() *ReversedEdgeRestorer { return &ReversedEdgeRestorer{} }

// Process is process(layeredGraph, monitor).
func (*ReversedEdgeRestorer) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	for _, layer := range layeredGraph.Layers {
		for _, node := range layer.Nodes {
			// Reversing an edge appends it to the incoming edges of this
			// port or of a later port, so the walk is over indices and over
			// a copy of the outgoing edges, as in the Java code.
			for portIndex := 0; portIndex < len(node.Ports); portIndex++ {
				port := node.Ports[portIndex]
				edgeArray := append([]*lgraph.LEdge(nil), port.OutgoingEdges...)
				for _, edge := range edgeArray {
					if prop.Get(edge, iprops.Reversed) {
						edge.Reverse(layeredGraph, false)
					}
				}
			}
		}
	}
}
