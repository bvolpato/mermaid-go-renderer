// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/NodeDimensionCalculation.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package nodespacing

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"

// CalculateLabelAndNodeSizes is
// NodeDimensionCalculation.calculateLabelAndNodeSizes(adapter): it computes
// the node sizes and places the ports and the labels of each node of the
// graph. The port lists must be sorted (see SortPortLists).
func CalculateLabelAndNodeSizes(adapter adapters.GraphAdapter) {
	Process(adapter)
}

// CalculateNodeMargins is NodeDimensionCalculation.calculateNodeMargins(
// adapter): it computes the margins of the nodes of the graph. To leave
// something out of the margins, use GetNodeMarginCalculator.
func CalculateNodeMargins(adapter adapters.GraphAdapter) {
	calcu := newNodeMarginCalculator(adapter)
	calcu.Process()
}

// GetNodeMarginCalculator is
// NodeDimensionCalculation.getNodeMarginCalculator(adapter): a node margin
// calculator that the caller can configure and then run with Process.
func GetNodeMarginCalculator(adapter adapters.GraphAdapter) *NodeMarginCalculator {
	return newNodeMarginCalculator(adapter)
}

// SortPortLists is NodeDimensionCalculation.sortPortLists(adapter): it sorts
// the port list of each node of the graph by side, clockwise from the top
// left corner.
func SortPortLists(adapter adapters.GraphAdapter) {
	// Iterate through the nodes of all layers
	for _, node := range adapter.Nodes() {
		node.SortPortList()
	}
}
