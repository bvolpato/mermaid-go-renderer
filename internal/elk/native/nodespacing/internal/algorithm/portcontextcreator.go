// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/internal/algorithm/PortContextCreator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package algorithm

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/cellsystem"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nodespacing/internal"
)

// CreatePortContexts is PortContextCreator.createPortContexts(nodeContext,
// ignoreInsidePortLabels): it makes a port context for each port of the
// node and gives each port a volatile id. Unless the port labels are fixed,
// it adds the labels of a port to the label cell of its context.
func CreatePortContexts(nodeContext *internal.NodeContext, ignoreInsidePortLabels bool) {
	// Hue hue hue... gimme color?
	imPortLabels := !ignoreInsidePortLabels || !nodeContext.PortLabelsPlacement.Contains(core.PortLabelPlacementInside)

	volatileID := 0
	for _, port := range nodeContext.Node.Ports() {
		if port.Side() == core.PortSideUndefined {
			lgraph.Bail("Label and node size calculator can only be used with ports that have port sides assigned.")
		}

		port.SetVolatileID(volatileID)
		volatileID++
		createPortContext(nodeContext, port, imPortLabels)
	}
}

// createPortContext makes the port context of a port.
func createPortContext(nodeContext *internal.NodeContext, port adapters.PortAdapter, imPortLabels bool) {
	portContext := internal.NewPortContext(nodeContext, port)
	nodeContext.PortContexts.Put(port.Side(), portContext)

	// If the port has labels and if port labels are to be placed, we need to
	// remember them
	if imPortLabels && !internal.PortLabelPlacementIsFixed(nodeContext.PortLabelsPlacement) {
		portContext.PortLabelCell = cellsystem.NewLabelCell(nodeContext.LabelLabelSpacing)
		for _, label := range port.Labels() {
			portContext.PortLabelCell.AddLabel(label)
		}
	}
}
