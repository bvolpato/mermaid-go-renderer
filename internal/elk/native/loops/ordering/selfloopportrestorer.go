// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/SelfLoopPortRestorer.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package ordering

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("SelfLoopPortRestorer", func() lgraph.Processor { return NewSelfLoopPortRestorer() })
}

// SelfLoopPortRestorer is the processor SelfLoopPortRestorer: it puts the
// hidden self loop ports back on their nodes and computes the types of the
// self loops. It does not restore the self loops.
type SelfLoopPortRestorer struct {
	// portSideAssigner gives hidden ports a side when the port constraints
	// were free.
	portSideAssigner *PortSideAssigner
	// portRestorer restores the ports when the sides are known.
	portRestorer *PortRestorer
}

// NewSelfLoopPortRestorer is new SelfLoopPortRestorer().
func NewSelfLoopPortRestorer() *SelfLoopPortRestorer {
	return &SelfLoopPortRestorer{portSideAssigner: NewPortSideAssigner(), portRestorer: NewPortRestorer()}
}

// Process is process(graph, progressMonitor).
func (p *SelfLoopPortRestorer) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	// Process each node that has self loops.
	for _, layer := range graph.Layers {
		for _, lNode := range layer.Nodes {
			if lNode.Type == lgraph.NodeTypeNormal && prop.Has(lNode, iprops.SelfLoopHolder) {
				p.processNode(loops.Of(lNode))
			}
		}
	}
}

func (p *SelfLoopPortRestorer) processNode(slHolder *loops.SelfLoopHolder) {
	// Restore and order the ports that have self loops only, if they were
	// hidden.
	if slHolder.ArePortsHidden() {
		if !prop.Has(slHolder.LNode, iprops.OriginalPortConstraints) {
			// The Java code fails with a NullPointerException.
			lgraph.Bail("SelfLoopPortRestorer: a node with hidden self loop ports has no original port constraints")
		}
		switch prop.Get(slHolder.LNode, iprops.OriginalPortConstraints) {
		case core.PortConstraintsUndefined, core.PortConstraintsFree:
			// Assign the port sides first, then restore the ports.
			p.portSideAssigner.AssignPortSides(slHolder)
			p.computeSelfLoopTypes(slHolder)
			p.portRestorer.RestorePorts(slHolder)

		case core.PortConstraintsFixedSide:
			// Restore the ports, which have port sides now. After this call
			// arePortsHidden() is false.
			p.computeSelfLoopTypes(slHolder)
			p.portRestorer.RestorePorts(slHolder)

		default:
			// This must not occur: ports are hidden only when their order
			// is not fixed. elkjs does nothing here and fails in the self
			// loop router.
			lgraph.Bail("SelfLoopPortRestorer: hidden self loop ports with the original port constraints %v",
				prop.Get(slHolder.LNode, iprops.OriginalPortConstraints))
		}
	} else {
		// The types of the self loops are still needed.
		p.computeSelfLoopTypes(slHolder)
	}
}

func (p *SelfLoopPortRestorer) computeSelfLoopTypes(slHolder *loops.SelfLoopHolder) {
	for _, slLoop := range slHolder.SLHyperLoops {
		slLoop.ComputePortsPerSide()
	}
}
