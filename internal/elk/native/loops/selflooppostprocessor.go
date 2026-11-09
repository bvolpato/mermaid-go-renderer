// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/SelfLoopPostProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package loops

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("SelfLoopPostProcessor", func() lgraph.Processor { return NewSelfLoopPostProcessor() })
}

// SelfLoopPostProcessor is the processor SelfLoopPostProcessor: it puts the
// routed self loops back on their ports, and moves their bend points and
// their labels to the position of the node.
type SelfLoopPostProcessor struct{}

// NewSelfLoopPostProcessor is new SelfLoopPostProcessor().
func NewSelfLoopPostProcessor() *SelfLoopPostProcessor { return &SelfLoopPostProcessor{} }

// Process is process(graph, progressMonitor).
func (p *SelfLoopPostProcessor) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	for _, layer := range graph.Layers {
		for _, lNode := range layer.Nodes {
			if lNode.Type == lgraph.NodeTypeNormal && prop.Has(lNode, iprops.SelfLoopHolder) {
				p.processNode(lNode)
			}
		}
	}
}

func (p *SelfLoopPostProcessor) processNode(lNode *lgraph.LNode) {
	slHolder := Of(lNode)

	for _, slLoop := range slHolder.SLHyperLoops {
		for _, slEdge := range slLoop.SLEdges {
			p.restoreEdge(lNode, slEdge)
		}
	}

	for _, slLoop := range slHolder.SLHyperLoops {
		if slLoop.SLLabels != nil {
			slLoop.SLLabels.ApplyPlacement(&lNode.Pos)
		}
	}
}

func (p *SelfLoopPostProcessor) restoreEdge(lNode *lgraph.LNode, slEdge *SelfLoopEdge) {
	lEdge := slEdge.LEdge
	lEdge.SetSource(slEdge.SLSource.LPort)
	lEdge.SetTarget(slEdge.SLTarget.LPort)

	lEdge.BendPoints.Offset(&lNode.Pos)
}
