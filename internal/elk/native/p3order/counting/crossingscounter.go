// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/counting/CrossingsCounter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package counting

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// CrossingsCounter is the class CrossingsCounter. It counts three types of
// edge crossings: crossings between two layers, crossings of in-layer edges,
// and crossings of the implied edges between north/south port dummies and
// their nodes. Each problem is turned into counting in-layer crossings: the
// ports get positions along one line, each edge end is added to a sorted
// list, and the number of crossings of an edge is the index of its end in
// that list.
//
// Each port must have an id that is unique in the graph, and each node an id
// that is unique in its layer.
type CrossingsCounter struct {
	portPositions []int

	indexTree *BinaryIndexedTree
	// ends is the Deque of the Java code, used as a stack.
	ends []int

	nodeCardinalities []int
}

// NewCrossingsCounter is new CrossingsCounter(portPositions). The counter
// writes to the given slice.
func NewCrossingsCounter(portPositions []int) *CrossingsCounter {
	return &CrossingsCounter{portPositions: portPositions}
}

// CountCrossingsBetweenLayers is countCrossingsBetweenLayers(leftLayerNodes,
// rightLayerNodes): the in-layer and between-layer crossings between the two
// layers.
func (c *CrossingsCounter) CountCrossingsBetweenLayers(leftLayerNodes, rightLayerNodes []*lgraph.LNode) int {
	ports := c.initPortPositionsCounterClockwise(leftLayerNodes, rightLayerNodes)
	c.indexTree = NewBinaryIndexedTree(len(ports))
	return c.countCrossingsOnPorts(ports)
}

// CountInLayerCrossingsOnSide is countInLayerCrossingsOnSide(nodes, side):
// the in-layer crossings on the given side.
func (c *CrossingsCounter) CountInLayerCrossingsOnSide(nodes []*lgraph.LNode, side core.PortSide) int {
	ports := c.InitPortPositionsForInLayerCrossings(nodes, side)
	return c.countInLayerCrossingsOnPorts(ports)
}

// CountNorthSouthPortCrossingsInLayer is
// countNorthSouthPortCrossingsInLayer(layer): the crossings between edges
// connected to north/south ports of the nodes of the layer, and of these
// edges with long edges that span the layer.
func (c *CrossingsCounter) CountNorthSouthPortCrossingsInLayer(layer []*lgraph.LNode) int {
	ports := c.initPositionsForNorthSouthCounting(layer)
	c.indexTree = NewBinaryIndexedTree(len(ports))
	return c.countNorthSouthCrossingsOnPorts(ports)
}

// CountCrossingsBetweenPortsInBothOrders is
// countCrossingsBetweenPortsInBothOrders(upperPort, lowerPort): the
// crossings between the edges of the two ports in the given order, and in
// the switched order. Initialize the counter first with
// InitForCountingBetween, or with InitPortPositionsForInLayerCrossings at
// either end of a graph.
func (c *CrossingsCounter) CountCrossingsBetweenPortsInBothOrders(upperPort, lowerPort *lgraph.LPort) (upperLowerCrossings, lowerUpperCrossings int) {
	ports := c.connectedPortsSortedByPosition(upperPort, lowerPort)
	upperLowerCrossings = c.countCrossingsOnPorts(ports)
	// Since we might add endpositions of ports which are not in the ports
	// list, we need to explicitly clear the index tree.
	c.indexTree.Clear()
	c.SwitchPorts(upperPort, lowerPort)
	gwt.SortFunc(ports, func(a, b *lgraph.LPort) int { return gwt.IntCompare(c.positionOf(a), c.positionOf(b)) })
	lowerUpperCrossings = c.countCrossingsOnPorts(ports)
	c.indexTree.Clear()
	c.SwitchPorts(lowerPort, upperPort)
	return upperLowerCrossings, lowerUpperCrossings
}

// CountInLayerCrossingsBetweenNodesInBothOrders is
// countInLayerCrossingsBetweenNodesInBothOrders(upperNode, lowerNode, side):
// the in-layer crossings between the edges of the two nodes in the given
// order, and in the switched order. Initialize the counter first with
// InitPortPositionsForInLayerCrossings.
func (c *CrossingsCounter) CountInLayerCrossingsBetweenNodesInBothOrders(upperNode, lowerNode *lgraph.LNode, side core.PortSide) (upperLowerCrossings, lowerUpperCrossings int) {
	ports := c.connectedInLayerPortsSortedByPosition(upperNode, lowerNode, side)
	upperLowerCrossings = c.countInLayerCrossingsOnPorts(ports)
	c.SwitchNodes(upperNode, lowerNode, side)
	// Since we might add endpositions of ports which are not in the ports
	// list, we need to explicitly clear the index tree.
	c.indexTree.Clear()
	gwt.SortFunc(ports, func(a, b *lgraph.LPort) int { return gwt.IntCompare(c.positionOf(a), c.positionOf(b)) })
	lowerUpperCrossings = c.countInLayerCrossingsOnPorts(ports)
	c.SwitchNodes(lowerNode, upperNode, side)
	c.indexTree.Clear()
	return upperLowerCrossings, lowerUpperCrossings
}

// InitForCountingBetween is initForCountingBetween(leftLayerNodes,
// rightLayerNodes): it prepares the counter for counting crossings between
// two layers with CountCrossingsBetweenPortsInBothOrders.
func (c *CrossingsCounter) InitForCountingBetween(leftLayerNodes, rightLayerNodes []*lgraph.LNode) {
	ports := c.initPortPositionsCounterClockwise(leftLayerNodes, rightLayerNodes)
	c.indexTree = NewBinaryIndexedTree(len(ports))
}

// InitPortPositionsForInLayerCrossings is
// initPortPositionsForInLayerCrossings(nodes, side): it prepares the counter
// for counting in-layer crossings on one side of one layer. It returns the
// ports to count crossings on.
func (c *CrossingsCounter) InitPortPositionsForInLayerCrossings(nodes []*lgraph.LNode, side core.PortSide) []*lgraph.LPort {
	var ports []*lgraph.LPort
	c.initPositions(nodes, &ports, side, true, true)
	c.indexTree = NewBinaryIndexedTree(len(ports))
	return ports
}

// SwitchPorts is switchPorts(topPort, bottomPort): it tells the counter
// that two ports were switched.
func (c *CrossingsCounter) SwitchPorts(topPort, bottomPort *lgraph.LPort) {
	topPortPos := c.portPositions[topPort.ID]
	c.portPositions[topPort.ID] = c.portPositions[bottomPort.ID]
	c.portPositions[bottomPort.ID] = topPortPos
}

// SwitchNodes is switchNodes(wasUpperNode, wasLowerNode, side): it tells the
// counter that two neighboring nodes were switched. wasUpperNode is the node
// that was the upper one before the switch.
func (c *CrossingsCounter) SwitchNodes(wasUpperNode, wasLowerNode *lgraph.LNode, side core.PortSide) {
	ports := InNorthSouthEastWestOrder(wasUpperNode, side)
	for _, port := range ports {
		c.portPositions[port.ID] = c.positionOf(port) + c.nodeCardinalities[wasLowerNode.ID]
	}

	ports = InNorthSouthEastWestOrder(wasLowerNode, side)
	for _, port := range ports {
		c.portPositions[port.ID] = c.positionOf(port) - c.nodeCardinalities[wasUpperNode.ID]
	}
}

// treeSetAdd is add(port) of the TreeSet that the Java code sorts by port
// position: a port whose position the set has already is not added.
func (c *CrossingsCounter) treeSetAdd(ports []*lgraph.LPort, port *lgraph.LPort) []*lgraph.LPort {
	pos := c.positionOf(port)
	lo, hi := 0, len(ports)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if c.positionOf(ports[mid]) < pos {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(ports) && c.positionOf(ports[lo]) == pos {
		return ports
	}
	ports = append(ports, nil)
	copy(ports[lo+1:], ports[lo:])
	ports[lo] = port
	return ports
}

func (c *CrossingsCounter) connectedInLayerPortsSortedByPosition(upperNode, lowerNode *lgraph.LNode, side core.PortSide) []*lgraph.LPort {
	var ports []*lgraph.LPort
	for _, node := range [2]*lgraph.LNode{upperNode, lowerNode} {
		for _, port := range InNorthSouthEastWestOrder(node, side) {
			for _, edge := range port.ConnectedEdges() {
				if !edge.IsSelfLoop() {
					ports = c.treeSetAdd(ports, port)
					if c.isInLayer(edge) {
						ports = c.treeSetAdd(ports, c.otherEndOf(edge, port))
					}
				}
			}
		}
	}
	return ports
}

func (c *CrossingsCounter) connectedPortsSortedByPosition(upperPort, lowerPort *lgraph.LPort) []*lgraph.LPort {
	var ports []*lgraph.LPort
	for _, port := range [2]*lgraph.LPort{upperPort, lowerPort} {
		ports = c.treeSetAdd(ports, port)
		for _, edge := range port.ConnectedEdges() {
			if !c.isPortSelfLoop(edge) {
				ports = c.treeSetAdd(ports, c.otherEndOf(edge, port))
			}
		}
	}
	return ports
}

// addEnds adds the collected end points to the index tree, last one first.
func (c *CrossingsCounter) addEnds() {
	for len(c.ends) != 0 {
		last := len(c.ends) - 1
		end := c.ends[last]
		c.ends = c.ends[:last]
		c.indexTree.Add(end)
	}
}

func (c *CrossingsCounter) countCrossingsOnPorts(ports []*lgraph.LPort) int {
	crossings := 0
	for _, port := range ports {
		c.indexTree.RemoveAll(c.positionOf(port))
		// First get crossings for all edges.
		for _, edge := range port.ConnectedEdges() {
			endPosition := c.positionOf(c.otherEndOf(edge, port))
			if endPosition > c.positionOf(port) {
				crossings += c.indexTree.Rank(endPosition)
				c.ends = append(c.ends, endPosition)
			}
		}
		// Then add end points.
		c.addEnds()
	}

	return crossings
}

func (c *CrossingsCounter) countInLayerCrossingsOnPorts(ports []*lgraph.LPort) int {
	crossings := 0
	for _, port := range ports {
		c.indexTree.RemoveAll(c.positionOf(port))
		numBetweenLayerEdges := 0
		// First get crossings for all edges.
		for _, edge := range port.ConnectedEdges() {
			if c.isInLayer(edge) {
				endPosition := c.positionOf(c.otherEndOf(edge, port))
				if endPosition > c.positionOf(port) {
					crossings += c.indexTree.Rank(endPosition)
					c.ends = append(c.ends, endPosition)
				}
			} else {
				numBetweenLayerEdges++
			}
		}
		crossings += c.indexTree.Size() * numBetweenLayerEdges
		// Then add end points.
		c.addEnds()
	}
	return crossings
}

// targetAndDegree is the Pair<LPort, Integer> of the Java code.
type targetAndDegree struct {
	target *lgraph.LPort
	degree int
}

func (c *CrossingsCounter) countNorthSouthCrossingsOnPorts(ports []*lgraph.LPort) int {
	crossings := 0
	var targetsAndDegrees []targetAndDegree

	for _, port := range ports {
		c.indexTree.RemoveAll(c.positionOf(port))
		targetsAndDegrees = targetsAndDegrees[:0]

		// collect the edges that are incident to the port, which is a bit
		// tedious since north/south ports have no physical edge within the
		// graph at this point
		switch port.Node().Type {
		case lgraph.NodeTypeNormal:
			dummy := prop.Get(port, iprops.PortDummy)
			// western and eastern
			for _, p := range dummy.Ports {
				targetsAndDegrees = append(targetsAndDegrees, targetAndDegree{p, p.Degree()})
			}

		case lgraph.NodeTypeLongEdge:
			// add an edge to the dummy's other port
			for _, p := range port.Node().Ports {
				if p != port {
					targetsAndDegrees = append(targetsAndDegrees, targetAndDegree{p, p.Degree()})
					break
				}
			}

		case lgraph.NodeTypeNorthSouthPort:
			dummyPort := OriginPort(port)
			targetsAndDegrees = append(targetsAndDegrees, targetAndDegree{dummyPort, port.Degree()})
		}

		// First get crossings for all edges.
		for _, td := range targetsAndDegrees {
			endPosition := c.positionOf(td.target)
			if endPosition > c.positionOf(port) {
				crossings += c.indexTree.Rank(endPosition) * td.degree
				c.ends = append(c.ends, endPosition)
			}
		}

		// Then add end points.
		c.addEnds()
	}

	return crossings
}

func (c *CrossingsCounter) initPositions(nodes []*lgraph.LNode, ports *[]*lgraph.LPort, side core.PortSide, topDown, getCardinalities bool) {
	numPorts := len(*ports)
	if getCardinalities {
		c.nodeCardinalities = make([]int, len(nodes))
	}
	i, step := 0, 1
	if !topDown {
		i, step = len(nodes)-1, -1
	}
	for ; i >= 0 && i < len(nodes); i += step {
		node := nodes[i]
		nodePorts := c.getPorts(node, side, topDown)
		if getCardinalities {
			c.nodeCardinalities[node.ID] = len(nodePorts)
		}
		for _, port := range nodePorts {
			c.portPositions[port.ID] = numPorts
			numPorts++
		}
		*ports = append(*ports, nodePorts...)
	}
}

func (c *CrossingsCounter) initPortPositionsCounterClockwise(leftLayerNodes, rightLayerNodes []*lgraph.LNode) []*lgraph.LPort {
	var ports []*lgraph.LPort
	c.initPositions(leftLayerNodes, &ports, core.PortSideEast, true, false)
	c.initPositions(rightLayerNodes, &ports, core.PortSideWest, false, false)
	return ports
}

const (
	indexingSide = core.PortSideWest
	stackSide    = core.PortSideEast
)

func (c *CrossingsCounter) initPositionsForNorthSouthCounting(nodes []*lgraph.LNode) []*lgraph.LPort {
	var ports []*lgraph.LPort
	// stack is the Deque of the Java code, used as a stack.
	var stack []*lgraph.LNode

	var lastLayoutUnit *lgraph.LNode
	index := 0
	for i := 0; i < len(nodes); i++ {
		current := nodes[i]

		if c.isLayoutUnitChanged(lastLayoutUnit, current) {
			// work the stack (filled with southern dummies)
			index = c.emptyStack(&stack, &ports, stackSide, index)
		}
		if prop.Has(current, iprops.InLayerLayoutUnit) {
			lastLayoutUnit = prop.Get(current, iprops.InLayerLayoutUnit)
		}

		switch current.Type {
		// what we consider normal
		case lgraph.NodeTypeNormal:
			// index the northern ports west-to-east
			for _, p := range c.getNorthSouthPortsWithIncidentEdges(current, core.PortSideNorth) {
				c.portPositions[p.ID] = index
				index++
				ports = append(ports, p)
			}

			// work the stack (filled with northern dummies)
			index = c.emptyStack(&stack, &ports, stackSide, index)

			// index the southern ports in regular clock-wise order
			for _, p := range c.getNorthSouthPortsWithIncidentEdges(current, core.PortSideSouth) {
				c.portPositions[p.ID] = index
				index++
				ports = append(ports, p)
			}

		case lgraph.NodeTypeNorthSouthPort:
			if len(current.PortSideView(indexingSide)) != 0 {
				// should be only one
				p := current.PortSideView(indexingSide)[0]
				c.portPositions[p.ID] = index
				index++
				ports = append(ports, p)
			}
			if len(current.PortSideView(stackSide)) != 0 {
				stack = append(stack, current)
			}

		case lgraph.NodeTypeLongEdge:
			for _, p := range current.PortSideView(core.PortSideWest) {
				c.portPositions[p.ID] = index
				index++
				ports = append(ports, p)
			}
			for range current.PortSideView(core.PortSideEast) {
				stack = append(stack, current)
			}

		default: // nothing to do here
		}
	}

	// are there any southern dummy nodes left on the stack?
	c.emptyStack(&stack, &ports, stackSide, index)

	return ports
}

func (c *CrossingsCounter) emptyStack(stack *[]*lgraph.LNode, ports *[]*lgraph.LPort, side core.PortSide, startIndex int) int {
	index := startIndex
	for len(*stack) != 0 {
		last := len(*stack) - 1
		dummy := (*stack)[last]
		*stack = (*stack)[:last]
		// dummy is either a north/south port dummy or a long edge dummy both
		// of which have only a single port on the west and/or east side
		p := dummy.PortSideView(side)[0]
		c.portPositions[p.ID] = index
		index++
		*ports = append(*ports, p)
	}
	return index
}

func (c *CrossingsCounter) getPorts(node *lgraph.LNode, side core.PortSide, topDown bool) []*lgraph.LPort {
	if side == core.PortSideEast {
		if topDown {
			return node.PortSideView(side)
		}
		return Reverse(node.PortSideView(side))
	} else if topDown {
		return Reverse(node.PortSideView(side))
	}
	return node.PortSideView(side)
}

func (c *CrossingsCounter) getNorthSouthPortsWithIncidentEdges(node *lgraph.LNode, side core.PortSide) []*lgraph.LPort {
	var out []*lgraph.LPort
	for _, p := range node.PortSideView(side) {
		if prop.Has(p, iprops.PortDummy) {
			out = append(out, p)
		}
	}
	return out
}

func (c *CrossingsCounter) isInLayer(edge *lgraph.LEdge) bool {
	sourceLayer := edge.Source.Node().Layer()
	targetLayer := edge.Target.Node().Layer()
	return sourceLayer == targetLayer
}

func (c *CrossingsCounter) positionOf(port *lgraph.LPort) int {
	return c.portPositions[port.ID]
}

func (c *CrossingsCounter) otherEndOf(edge *lgraph.LEdge, fromPort *lgraph.LPort) *lgraph.LPort {
	if fromPort == edge.Source {
		return edge.Target
	}
	return edge.Source
}

func (c *CrossingsCounter) isPortSelfLoop(edge *lgraph.LEdge) bool {
	return edge.Source == edge.Target
}

func (c *CrossingsCounter) isLayoutUnitChanged(lastUnit, node *lgraph.LNode) bool {
	if lastUnit == nil || lastUnit == node || !prop.Has(node, iprops.InLayerLayoutUnit) {
		return false
	}
	unit := prop.Get(node, iprops.InLayerLayoutUnit)
	return unit != lastUnit
}

// OriginPort is the cast (LPort) element.getProperty(ORIGIN). It returns nil
// for an element without an origin. The layout stops for an origin that is
// not a port, where the Java code throws.
func OriginPort(h prop.Holder) *lgraph.LPort {
	v := prop.Get(h, iprops.Origin)
	if v == nil {
		return nil
	}
	port, ok := v.(*lgraph.LPort)
	if !ok {
		lgraph.Bail("counting: the origin of an element is not a port")
	}
	return port
}
