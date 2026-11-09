// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/AbstractBarycenterPortDistributor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// AbstractBarycenterPortDistributor is the abstract class
// AbstractBarycenterPortDistributor: it calculates port ranks and
// distributes ports. The rank of a port is a number that stands for its
// position in its layer. It depends on the node order of the layer and on
// the port constraints of the nodes. The barycenter heuristic uses the port
// ranks, and this class uses them to distribute the ports of a node whose
// port order is not fixed.
//
// The Java type of the ranks is float. A float of GWT is a double, so the
// ranks are float64 here.
type AbstractBarycenterPortDistributor struct {
	counting.DefaultInit

	// portRanks is the array in which the results of the rank calculation
	// are stored.
	portRanks      []float64
	minBarycenter  float64
	maxBarycenter  float64
	nodePositions  [][]int
	portBarycenter []float64
	inLayerPorts   []*lgraph.LPort
	nPorts         int

	// calculateNodePortRanks is the abstract method calculatePortRanks(node,
	// rankSum, type) of the subclass: it assigns port ranks to the input or
	// output ports of the node and returns the rank that the node consumed.
	calculateNodePortRanks func(node *lgraph.LNode, rankSum float64, portType lenum.PortType) float64
}

// barycenterPortDistributor is implemented by the subclasses of
// AbstractBarycenterPortDistributor. It stands for the Java cast to the
// abstract class.
type barycenterPortDistributor interface {
	abstractBarycenterPortDistributor() *AbstractBarycenterPortDistributor
}

func (d *AbstractBarycenterPortDistributor) abstractBarycenterPortDistributor() *AbstractBarycenterPortDistributor {
	return d
}

// initAbstractBarycenterPortDistributor is the constructor
// AbstractBarycenterPortDistributor(numLayers).
func (d *AbstractBarycenterPortDistributor) initAbstractBarycenterPortDistributor(numLayers int) {
	d.nodePositions = make([][]int, numLayers)
}

// PortRanks is getPortRanks().
func (d *AbstractBarycenterPortDistributor) PortRanks() []float64 { return d.portRanks }

// DistributePortsWhileSweeping is distributePortsWhileSweeping(nodeOrder,
// currentIndex, isForwardSweep).
func (d *AbstractBarycenterPortDistributor) DistributePortsWhileSweeping(nodeOrder [][]*lgraph.LNode, currentIndex int, isForwardSweep bool) bool {
	d.updateNodePositions(nodeOrder, currentIndex)
	freeLayer := nodeOrder[currentIndex]
	side := core.PortSideEast
	if isForwardSweep {
		side = core.PortSideWest
	}
	if d.isNotFirstLayer(len(nodeOrder), currentIndex, isForwardSweep) {
		var fixedLayer []*lgraph.LNode
		if isForwardSweep {
			fixedLayer = nodeOrder[currentIndex-1]
		} else {
			fixedLayer = nodeOrder[currentIndex+1]
		}
		d.CalculatePortRanks(fixedLayer, d.portTypeFor(isForwardSweep))
		for _, node := range freeLayer {
			d.distributePorts(node, side)
		}

		d.CalculatePortRanks(freeLayer, d.portTypeFor(!isForwardSweep))
		for _, node := range fixedLayer {
			if !d.hasNestedGraph(node) {
				d.distributePorts(node, side.Opposed())
			}
		}
	} else {
		for _, node := range freeLayer {
			d.distributePorts(node, side)
		}
	}
	// Barycenter port distributor can not be used with always improving
	// crossing minimization heuristics which do not need to count.
	return false
}

// CalculatePortRanks is calculatePortRanks(layer, portType): it determines
// the ranks of all ports of one type in the given layer. It writes the
// ranks to the array of port ranks.
func (d *AbstractBarycenterPortDistributor) CalculatePortRanks(layer []*lgraph.LNode, portType lenum.PortType) {
	consumedRank := 0.0
	for nodeIx := 0; nodeIx < len(layer); nodeIx++ {
		consumedRank += d.calculateNodePortRanks(layer[nodeIx], consumedRank, portType)
	}
}

func (d *AbstractBarycenterPortDistributor) distributePorts(node *lgraph.LNode, side core.PortSide) {
	if !prop.Get(node, lopts.PortConstraints).IsOrderFixed() {
		// distribute ports in sweep direction and on north south side of
		// node.
		d.distributePortsOf(node, node.PortsBySide(side))
		d.distributePortsOf(node, node.PortsBySide(core.PortSideSouth))
		d.distributePortsOf(node, node.PortsBySide(core.PortSideNorth))
		// sort the ports by considering the side, type, and barycenter
		// values
		d.sortPorts(node)
	}
}

// distributePortsOf is distributePorts(node, ports): it distributes the
// given ports of the node by their sides, connected ports, and input or
// output type.
func (d *AbstractBarycenterPortDistributor) distributePortsOf(node *lgraph.LNode, ports []*lgraph.LPort) {
	d.inLayerPorts = d.inLayerPorts[:0]
	d.iteratePortsAndCollectInLayerPorts(node, ports)

	if len(d.inLayerPorts) != 0 {
		d.calculateInLayerPortsBarycenterValues(node)
	}
}

func (d *AbstractBarycenterPortDistributor) iteratePortsAndCollectInLayerPorts(node *lgraph.LNode, ports []*lgraph.LPort) {
	d.minBarycenter = 0.0
	d.maxBarycenter = 0.0

	// a float value large enough to ensure that barycenters of south ports
	// work fine
	absurdlyLargeFloat := float64(2*len(node.Layer().Nodes) + 1)
	// calculate barycenter values for the ports of the node
portIteration:
	for _, port := range ports {
		northSouthPort := port.Side == core.PortSideNorth || port.Side == core.PortSideSouth
		sum := 0.0

		if northSouthPort {
			// Find the dummy node created for the port
			portDummy := prop.Get(port, iprops.PortDummy)
			if portDummy == nil {
				continue
			}

			sum += d.dealWithNorthSouthPorts(absurdlyLargeFloat, port, portDummy)
		} else {
			// add up all ranks of connected ports
			for _, outgoingEdge := range port.OutgoingEdges {
				connectedPort := outgoingEdge.Target
				if connectedPort.Node().Layer() == node.Layer() {
					d.inLayerPorts = append(d.inLayerPorts, port)
					continue portIteration
				}
				// outgoing edges go to the subsequent layer and are seen
				// clockwise
				sum += d.portRanks[connectedPort.ID]
			}
			for _, incomingEdge := range port.IncomingEdges {
				connectedPort := incomingEdge.Source
				if connectedPort.Node().Layer() == node.Layer() {
					d.inLayerPorts = append(d.inLayerPorts, port)
					continue portIteration
				}
				// incoming edges go to the preceding layer and are seen
				// counter-clockwise
				sum -= d.portRanks[connectedPort.ID]
			}
		}

		if port.Degree() > 0 {
			d.portBarycenter[port.ID] = sum / float64(port.Degree())
			d.minBarycenter = min(d.minBarycenter, d.portBarycenter[port.ID])
			d.maxBarycenter = max(d.maxBarycenter, d.portBarycenter[port.ID])
		} else if northSouthPort {
			// For northern and southern ports, the sum directly corresponds
			// to the barycenter value to be used.
			d.portBarycenter[port.ID] = sum
		}
	}
}

func (d *AbstractBarycenterPortDistributor) calculateInLayerPortsBarycenterValues(node *lgraph.LNode) {
	// go through the list of in-layer ports and calculate their barycenter
	// values
	nodeIndexInLayer := float64(d.positionOf(node) + 1)
	layerSize := float64(len(node.Layer().Nodes) + 1)
	for _, inLayerPort := range d.inLayerPorts {
		// add the indices of all connected in-layer ports
		sum := 0
		inLayerConnections := 0

		for _, connectedPort := range inLayerPort.ConnectedPorts() {
			if connectedPort.Node().Layer() == node.Layer() {
				sum += d.positionOf(connectedPort.Node()) + 1
				inLayerConnections++
			}
		}
		// The port's barycenter value is the mean index of connected nodes.
		// If that value is lower than the node's index, most in-layer edges
		// point upwards, so we want the port to be placed at the top of the
		// side. If the value is higher than the nodes's index, most
		// in-layer edges point downwards, so we want the port to be placed
		// at the bottom of the side.
		barycenter := float64(sum) / float64(inLayerConnections)

		portSide := inLayerPort.Side

		if portSide == core.PortSideEast {
			if barycenter < nodeIndexInLayer {
				// take a low value in order to have the port above
				d.portBarycenter[inLayerPort.ID] = d.minBarycenter - barycenter
			} else {
				// take a high value in order to have the port below
				d.portBarycenter[inLayerPort.ID] = d.maxBarycenter + (layerSize - barycenter)
			}
		} else if portSide == core.PortSideWest {
			if barycenter < nodeIndexInLayer {
				// take a high value in order to have the port above
				d.portBarycenter[inLayerPort.ID] = d.maxBarycenter + barycenter
			} else {
				// take a low value in order to have the port below
				d.portBarycenter[inLayerPort.ID] = d.minBarycenter - (layerSize - barycenter)
			}
		}
	}
}

func (d *AbstractBarycenterPortDistributor) dealWithNorthSouthPorts(absurdlyLargeFloat float64, port *lgraph.LPort, portDummy *lgraph.LNode) float64 {
	// Find out if it's an input port, an output port, or both
	input := false
	output := false
	for _, portDummyPort := range portDummy.Ports {
		if prop.Get(portDummyPort, iprops.Origin) == any(port) {
			if len(portDummyPort.OutgoingEdges) != 0 {
				output = true
			} else if len(portDummyPort.IncomingEdges) != 0 {
				input = true
			}
		}
	}
	sum := 0.0
	if input && input != output {
		// It's an input port; the index of its dummy node is its inverted
		// sortkey (for southern input ports, the key must be larger than
		// the ones assigned to output ports or input&&output ports)
		if port.Side == core.PortSideNorth {
			sum = -float64(d.positionOf(portDummy))
		} else {
			sum = absurdlyLargeFloat - float64(d.positionOf(portDummy))
		}
	} else if output && input != output {
		// It's an output port; the index of its dummy node is its sort key
		// (for northern output ports, the key must be larger than the ones
		// assigned to input ports or input&&output ports, which are
		// negative and 0, respectively)
		sum = float64(d.positionOf(portDummy)) + 1.0
	} else if input && output {
		// It's both, an input and an output port; it must sit between input
		// and output ports. North: input ports < 0.0, output ports > 0.0.
		// South: input ports > FLOAT_MAX / 2, output ports near zero
		if port.Side == core.PortSideNorth {
			sum = 0.0
		} else {
			sum = absurdlyLargeFloat / 2
		}
	}
	return sum
}

func (d *AbstractBarycenterPortDistributor) positionOf(node *lgraph.LNode) int {
	return d.nodePositions[node.Layer().ID][node.ID]
}

func (d *AbstractBarycenterPortDistributor) updateNodePositions(nodeOrder [][]*lgraph.LNode, currentIndex int) {
	layer := nodeOrder[currentIndex]
	for i := 0; i < len(layer); i++ {
		node := layer[i]
		d.nodePositions[node.Layer().ID][node.ID] = i
	}
}

func (d *AbstractBarycenterPortDistributor) hasNestedGraph(node *lgraph.LNode) bool {
	return node.NestedGraph != nil
}

func (d *AbstractBarycenterPortDistributor) isNotFirstLayer(length, currentIndex int, isForwardSweep bool) bool {
	if isForwardSweep {
		return currentIndex != 0
	}
	return currentIndex != length-1
}

func (d *AbstractBarycenterPortDistributor) portTypeFor(isForwardSweep bool) lenum.PortType {
	if isForwardSweep {
		return lenum.PortTypeOutput
	}
	return lenum.PortTypeInput
}

// sortPorts sorts the ports of a node by the relative position values.
// These values are a hint for the clockwise order of the ports.
func (d *AbstractBarycenterPortDistributor) sortPorts(node *lgraph.LNode) {
	gwt.SortFunc(node.Ports, func(port1, port2 *lgraph.LPort) int {
		side1 := port1.Side
		side2 := port2.Side

		if side1 != side2 {
			// sort according to the node side
			return int(side1) - int(side2)
		}
		port1Bary := d.portBarycenter[port1.ID]
		port2Bary := d.portBarycenter[port2.ID]
		if port1Bary == 0 && port2Bary == 0 {
			return 0
		} else if port1Bary == 0 {
			return -1
		} else if port2Bary == 0 {
			return 1
		}
		// sort according to the position value
		return gwt.DoubleCompare(port1Bary, port2Bary)
	})
}

// InitAtLayerLevel is initAtLayerLevel(l, nodeOrder).
func (d *AbstractBarycenterPortDistributor) InitAtLayerLevel(l int, nodeOrder [][]*lgraph.LNode) {
	d.nodePositions[l] = make([]int, len(nodeOrder[l]))
}

// InitAtNodeLevel is initAtNodeLevel(l, n, nodeOrder). It sets the id of
// the node.
func (d *AbstractBarycenterPortDistributor) InitAtNodeLevel(l, n int, nodeOrder [][]*lgraph.LNode) {
	node := nodeOrder[l][n]
	node.ID = n
	d.nodePositions[l][n] = n
}

// InitAtPortLevel is initAtPortLevel(l, n, p, nodeOrder). It sets the id of
// the port.
func (d *AbstractBarycenterPortDistributor) InitAtPortLevel(l, n, p int, nodeOrder [][]*lgraph.LNode) {
	nodeOrder[l][n].Ports[p].ID = d.nPorts
	d.nPorts++
}

// InitAfterTraversal is initAfterTraversal().
func (d *AbstractBarycenterPortDistributor) InitAfterTraversal() {
	d.portRanks = make([]float64, d.nPorts)
	d.portBarycenter = make([]float64, d.nPorts)
}
