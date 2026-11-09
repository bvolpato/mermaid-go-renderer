// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/NodeRelativePortDistributor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// NodeRelativePortDistributor is the class NodeRelativePortDistributor: it
// calculates port ranks in a node-relative way. Each node i gets the range
// of values from i to i+1 for its ports. So a node with many ports has the
// same weight as a node with few ports.
type NodeRelativePortDistributor struct {
	AbstractBarycenterPortDistributor
}

// NewNodeRelativePortDistributor is new NodeRelativePortDistributor(
// numLayers).
func NewNodeRelativePortDistributor(numLayers int) *NodeRelativePortDistributor {
	d := &NodeRelativePortDistributor{}
	d.initAbstractBarycenterPortDistributor(numLayers)
	d.calculateNodePortRanks = d.calculatePortRanks
	return d
}

// calculatePortRanks is calculatePortRanks(node, rankSum, type).
func (d *NodeRelativePortDistributor) calculatePortRanks(node *lgraph.LNode, rankSum float64, portType lenum.PortType) float64 {
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
		incr := 1.0 / float64(inputCount+1)
		northPos := rankSum + float64(float64(northInputCount)*incr)
		restPos := rankSum + 1 - incr
		for _, port := range node.PortsByType(lenum.PortTypeInput) {
			if port.Side == core.PortSideNorth {
				portRanks[port.ID] = northPos
				northPos -= incr
			} else {
				portRanks[port.ID] = restPos
				restPos -= incr
			}
		}

	case lenum.PortTypeOutput:
		// Count the number of output ports
		outputCount := 0
		for _, port := range node.Ports {
			if len(port.OutgoingEdges) != 0 {
				outputCount++
			}
		}

		// Iterate output ports in their natural order, that is north - east
		// - south - west
		incr := 1.0 / float64(outputCount+1)
		pos := rankSum + incr
		for _, port := range node.PortsByType(lenum.PortTypeOutput) {
			portRanks[port.ID] = pos
			pos += incr
		}

	default:
		// this means illegal input to the method
		lgraph.Bail("Port type is undefined")
	}

	// the consumed rank is always 1
	return 1
}
