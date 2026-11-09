// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/NoCrossingMinimizer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"

func init() {
	lgraph.Register("NoCrossingMinimizer", func() lgraph.Processor { return NewNoCrossingMinimizer() })
}

// NoCrossingMinimizer is the class NoCrossingMinimizer: it does nothing
// instead of minimizing crossings. The order of the nodes and of the ports
// remains as it was before this phase.
type NoCrossingMinimizer struct{}

// NewNoCrossingMinimizer is new NoCrossingMinimizer().
func NewNoCrossingMinimizer() *NoCrossingMinimizer { return &NoCrossingMinimizer{} }

// Process is process(graph, progressMonitor).
func (*NoCrossingMinimizer) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {}
