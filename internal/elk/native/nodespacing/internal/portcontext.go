// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/PortContext.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package internal

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
)

// PortContext is the class PortContext: the data of one port during the
// node size calculation. The algorithm keeps the position of the port in
// PortPosition and applies it at the end, if required.
type PortContext struct {
	// ParentNodeContext is the context of the node of the port.
	ParentNodeContext *NodeContext
	// Port is the port.
	Port adapters.PortAdapter
	// PortPosition is the position of the port, to be changed by the
	// algorithm and possibly applied later.
	PortPosition *emath.KVector
	// LabelsNextToPort tells that the labels of the port are placed next to
	// the port.
	LabelsNextToPort bool
	// PortMargin is the margin around the port to assume when the port is
	// placed. With port labels it includes the label cell.
	PortMargin emath.ElkMargin
	// PortLabelCell is the cell for the labels of the port, or nil.
	PortLabelCell *cellsystem.LabelCell
}

// NewPortContext is new PortContext(parentNodeContext, port).
func NewPortContext(parentNodeContext *NodeContext, port adapters.PortAdapter) *PortContext {
	c := &PortContext{
		ParentNodeContext: parentNodeContext,
		Port:              port,
		PortPosition:      port.Position().Clone(),
	}

	portLabelsNextToPort := parentNodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementNextToPortIfPossible)

	// Whether labels are supposed to be placed next to their port is
	// determined differently depending on whether they are to be placed
	// inside or outside
	if parentNodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementInside) {
		if parentNodeContext.TreatAsCompoundNode {
			// There might be connections to the inside. That means that we
			// may want to place port labels next to their port, if possible
			c.LabelsNextToPort = portLabelsNextToPort && !port.HasCompoundConnections()
		} else {
			c.LabelsNextToPort = true
		}
	} else if parentNodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementOutside) {
		if portLabelsNextToPort {
			// We can place a label next to an outside port if that port
			// doesn't have incident edges
			c.LabelsNextToPort = !(len(port.IncomingEdges()) > 0 || len(port.OutgoingEdges()) > 0)
		} else {
			// We are not to place labels next to the port anyway
			c.LabelsNextToPort = false
		}
	} else {
		c.LabelsNextToPort = false
	}
	return c
}

// ApplyPortPosition is applyPortPosition(): it sets the position of the port
// to PortPosition.
func (c *PortContext) ApplyPortPosition() {
	c.Port.SetPosition(c.PortPosition)
}
