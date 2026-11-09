// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/SelfLoopPreProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package loops

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("SelfLoopPreProcessor", func() lgraph.Processor { return NewSelfLoopPreProcessor() })
}

// SelfLoopPreProcessor is the processor SelfLoopPreProcessor: it finds the
// regular nodes with self loops and gives each one a SelfLoopHolder. It
// removes the self loops from their ports. Unless the port order is fixed,
// it also removes each port whose edges are all self loops from its node.
type SelfLoopPreProcessor struct{}

// NewSelfLoopPreProcessor is new SelfLoopPreProcessor().
func NewSelfLoopPreProcessor() *SelfLoopPreProcessor { return &SelfLoopPreProcessor{} }

// Process is process(graph, progressMonitor).
func (p *SelfLoopPreProcessor) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	for _, lnode := range graph.LayerlessNodes {
		if NeedsSelfLoopProcessing(lnode) {
			slHolder := Install(lnode)
			p.hideSelfLoops(slHolder)
			p.hidePorts(slHolder)
		}
	}
}

// hideSelfLoops is hideSelfLoops(slHolder): it hides each self loop edge.
// The edges would confuse the later phases. The post processor restores
// them when the edge routing is done.
func (p *SelfLoopPreProcessor) hideSelfLoops(slHolder *SelfLoopHolder) {
	for _, slLoop := range slHolder.SLHyperLoops {
		for _, slEdge := range slLoop.SLEdges {
			p.hideSelfLoop(slEdge.LEdge)
		}
	}
}

// hideSelfLoop is hideSelfLoop(lEdge): it removes the edge from its ports.
func (p *SelfLoopPreProcessor) hideSelfLoop(lEdge *lgraph.LEdge) {
	lEdge.SetSource(nil)
	lEdge.SetTarget(nil)
}

// hidePorts is hidePorts(slHolder): it hides each port whose edges are all
// self loops, unless the port constraints are at least FIXED_ORDER.
func (p *SelfLoopPreProcessor) hidePorts(slHolder *SelfLoopHolder) {
	lNode := slHolder.LNode
	nestedGraph := lNode.NestedGraph

	// The ports stay on the node in two cases:
	// 1. The port order is fixed. No later code computes a port order, so
	//    the self loop ports cannot confuse it.
	// 2. The node has a graph inside that has external ports. The crossing
	//    minimization then computes the port order. The node gets that port
	//    order and the port constraints FIXED_POS, so this becomes the first
	//    case.
	orderFixed := prop.Get(lNode, lopts.PortConstraints).IsOrderFixed()
	hierarchyMode := nestedGraph != nil && prop.Get(nestedGraph, iprops.GraphProperties).Contains(lenum.GraphPropertiesExternalPorts)

	if orderFixed || hierarchyMode {
		// No port must be hidden.
		return
	}

	for _, slPort := range slHolder.SLPortMap.Values() {
		if slPort.HadOnlySelfLoops {
			// Hide the port.
			lPort := slPort.LPort
			lPort.SetNode(nil)

			// Remember that the port is hidden.
			slPort.SetHidden(true)
			slHolder.SetPortsHidden(true)
		}
	}
}
