// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/PortSideProcessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("PortSideProcessor", func() lgraph.Processor { return NewPortSideProcessor() })
}

// PortSideProcessor is the processor PortSideProcessor: it gives a side to
// each port that has none, and makes the port sides of each node fixed.
type PortSideProcessor struct{}

// NewPortSideProcessor is new PortSideProcessor().
func NewPortSideProcessor() *PortSideProcessor { return &PortSideProcessor{} }

// Process is process(layeredGraph, monitor).
func (p *PortSideProcessor) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	// IF USED BEFORE PHASE 1
	// Iterate through the unlayered nodes of the graph
	for _, node := range layeredGraph.LayerlessNodes {
		p.processNode(node)
	}

	// IF USED BEFORE PHASE 3
	// Iterate through the nodes of all layers
	for _, layer := range layeredGraph.Layers {
		for _, node := range layer.Nodes {
			p.processNode(node)
		}
	}
}

// processNode is process(LNode): it processes the ports of the node.
func (p *PortSideProcessor) processNode(node *lgraph.LNode) {
	if prop.Get(node, lopts.PortConstraints).IsSideFixed() {
		// Check whether there are ports with undefined side
		for _, port := range node.Ports {
			if port.Side == core.PortSideUndefined {
				SetPortSide(port)
			}
		}
	} else {
		// Distribute all ports and change constraints to fixed side
		for _, port := range node.Ports {
			SetPortSide(port)
		}
		prop.Set(node, lopts.PortConstraints, core.PortConstraintsFixedSide)
	}
}

// SetPortSide is PortSideProcessor.setPortSide(port): it places an input
// port on the left side and an output port on the right side of its node.
func SetPortSide(port *lgraph.LPort) {
	portDummy := prop.Get(port, iprops.PortDummy)
	if portDummy != nil {
		port.SetSide(prop.Get(portDummy, iprops.ExtPortSide))
	} else if port.NetFlow() < 0 {
		port.SetSide(core.PortSideEast)
	} else {
		port.SetSide(core.PortSideWest)
	}
}
