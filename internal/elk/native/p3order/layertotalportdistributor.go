// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/LayerTotalPortDistributor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// LayerTotalPortDistributor is the class LayerTotalPortDistributor: it
// calculates port ranks in a layer-total way. Each node i gets the range of
// values from i to i+x for its ports, where x is the number of its input
// ports or output ports. So a node with many ports gets a broader range of
// ranks than a node with few ports.
type LayerTotalPortDistributor struct {
	AbstractBarycenterPortDistributor
}

// NewLayerTotalPortDistributor is new LayerTotalPortDistributor(numLayers).
func NewLayerTotalPortDistributor(numLayers int) *LayerTotalPortDistributor {
	d := &LayerTotalPortDistributor{}
	d.initAbstractBarycenterPortDistributor(numLayers)
	d.calculateNodePortRanks = d.calculatePortRanks
	return d
}

// calculatePortRanks is calculatePortRanks(node, rankSum, type).
func (d *LayerTotalPortDistributor) calculatePortRanks(node *lgraph.LNode, rankSum float64, portType lenum.PortType) float64 {
	portRanks := d.PortRanks()

	switch portType {
	case lenum.PortTypeInput:
		// Count the number of input ports, and additionally the north-side
		// input ports
		inputCount, northInputCount := 0, 0
		for _, port := range node.Ports {
			if len(port.IncomingEdges) != 0 {
				inputCount++
				if port.Side == core.PortSideNorth {
					northInputCount++
				}
			}
		}

		// Assign port ranks in the order north - west - south - east
		northPos := rankSum + float64(northInputCount)
		restPos := rankSum + float64(inputCount)
		for _, port := range node.PortsByType(lenum.PortTypeInput) {
			if port.Side == core.PortSideNorth {
				portRanks[port.ID] = northPos
				northPos--
			} else {
				portRanks[port.ID] = restPos
				restPos--
			}
		}

		// the consumed rank corresponds to the number of input ports
		return float64(inputCount)

	case lenum.PortTypeOutput:
		// Iterate output ports in their natural order, that is north - east
		// - south - west
		pos := 0
		for _, port := range node.PortsByType(lenum.PortTypeOutput) {
			pos++
			portRanks[port.ID] = rankSum + float64(pos)
		}
		return float64(pos)
	}

	// this means illegal input to the method
	lgraph.Bail("LayerTotalPortDistributor: the port type is undefined")
	return 0
}
