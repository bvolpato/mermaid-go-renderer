// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/greedyswitch/NorthSouthEdgeNeighbouringNodeCrossingsCounter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package greedyswitch

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// NorthSouthEdgeNeighbouringNodeCrossingsCounter is the class
// NorthSouthEdgeNeighbouringNodeCrossingsCounter: it counts the crossings
// that the order of north south port dummies causes when their normal node
// in the same layer has a fixed port order. It also counts the crossings
// between north south edges and long edge dummies.
type NorthSouthEdgeNeighbouringNodeCrossingsCounter struct {
	upperLowerCrossings int
	lowerUpperCrossings int
	portPositions       map[*lgraph.LPort]int
	layer               []*lgraph.LNode
}

// NewNorthSouthEdgeNeighbouringNodeCrossingsCounter is new
// NorthSouthEdgeNeighbouringNodeCrossingsCounter(nodes).
func NewNorthSouthEdgeNeighbouringNodeCrossingsCounter(nodes []*lgraph.LNode) *NorthSouthEdgeNeighbouringNodeCrossingsCounter {
	c := &NorthSouthEdgeNeighbouringNodeCrossingsCounter{
		layer:         nodes,
		portPositions: map[*lgraph.LPort]int{},
	}
	c.initializePortPositions()
	return c
}

// initializePortPositions saves the positions of the ports. Ports are
// numbered as they are in the port list.
func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) initializePortPositions() {
	for _, node := range c.layer {
		c.setPortIdsOn(node, core.PortSideSouth)
		c.setPortIdsOn(node, core.PortSideNorth)
	}
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) setPortIdsOn(node *lgraph.LNode, side core.PortSide) {
	ports := counting.InNorthSouthEastWestOrder(node, side)
	portID := 0
	for _, port := range ports {
		c.portPositions[port] = portID
		portID++
	}
}

// CountCrossings is countCrossings(upperNode, lowerNode): it counts north
// south port crossings, and crossings between north south ports and dummy
// nodes, for the two nodes.
func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) CountCrossings(upperNode, lowerNode *lgraph.LNode) {
	c.upperLowerCrossings = 0
	c.lowerUpperCrossings = 0

	c.processIfTwoNorthSouthNodes(upperNode, lowerNode)

	c.processIfNorthSouthLongEdgeDummyCrossing(upperNode, lowerNode)

	c.processIfNormalNodeWithNSPortsAndLongEdgeDummy(upperNode, lowerNode)
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) processIfTwoNorthSouthNodes(upperNode, lowerNode *lgraph.LNode) {
	if c.isNorthSouth(upperNode) && c.isNorthSouth(lowerNode) && !c.haveDifferentOrigins(upperNode, lowerNode) {
		if c.isNorthOfNormalNode(upperNode) {
			c.countCrossingsOfTwoNorthSouthDummies(upperNode, lowerNode)
		} else {
			c.countCrossingsOfTwoNorthSouthDummies(lowerNode, upperNode)
		}
	}
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) countCrossingsOfTwoNorthSouthDummies(furtherFromNormalNode, closerToNormalNode *lgraph.LNode) {
	if c.originPortPositionOf(furtherFromNormalNode) > c.originPortPositionOf(closerToNormalNode) {
		closerEastPorts := closerToNormalNode.PortSideView(core.PortSideEast)
		c.upperLowerCrossings = firstDegree(closerEastPorts)
		furtherWestPorts := furtherFromNormalNode.PortSideView(core.PortSideWest)
		c.lowerUpperCrossings = firstDegree(furtherWestPorts)
	} else {
		closerWestPorts := closerToNormalNode.PortSideView(core.PortSideWest)
		c.upperLowerCrossings = firstDegree(closerWestPorts)
		furtherEastPorts := furtherFromNormalNode.PortSideView(core.PortSideEast)
		c.lowerUpperCrossings = firstDegree(furtherEastPorts)
	}
}

// firstDegree returns the degree of the first port, or 0 without ports.
func firstDegree(ports []*lgraph.LPort) int {
	if len(ports) == 0 {
		return 0
	}
	return ports[0].Degree()
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) processIfNorthSouthLongEdgeDummyCrossing(upperNode, lowerNode *lgraph.LNode) {
	if c.isNorthSouth(upperNode) && c.isLongEdgeDummy(lowerNode) {
		if c.isNorthOfNormalNode(upperNode) {
			c.upperLowerCrossings = 1
		} else {
			c.lowerUpperCrossings = 1
		}
	} else if c.isNorthSouth(lowerNode) && c.isLongEdgeDummy(upperNode) {
		if c.isNorthOfNormalNode(lowerNode) {
			c.lowerUpperCrossings = 1
		} else {
			c.upperLowerCrossings = 1
		}
	}
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) processIfNormalNodeWithNSPortsAndLongEdgeDummy(upperNode, lowerNode *lgraph.LNode) {
	if c.isNormal(upperNode) && c.isLongEdgeDummy(lowerNode) {
		c.upperLowerCrossings = c.numberOfNorthSouthEdges(upperNode, core.PortSideSouth)
		c.lowerUpperCrossings = c.numberOfNorthSouthEdges(upperNode, core.PortSideNorth)
	}
	if c.isNormal(lowerNode) && c.isLongEdgeDummy(upperNode) {
		c.upperLowerCrossings = c.numberOfNorthSouthEdges(lowerNode, core.PortSideNorth)
		c.lowerUpperCrossings = c.numberOfNorthSouthEdges(lowerNode, core.PortSideSouth)
	}
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) numberOfNorthSouthEdges(node *lgraph.LNode, side core.PortSide) int {
	numberOfEdges := 0
	for _, port := range node.PortSideView(side) {
		if c.hasConnectedNorthSouthEdge(port) {
			numberOfEdges++
		}
	}
	return numberOfEdges
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) hasConnectedNorthSouthEdge(port *lgraph.LPort) bool {
	return prop.Get(port, iprops.PortDummy) != nil
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) haveDifferentOrigins(upperNode, lowerNode *lgraph.LNode) bool {
	return c.originOf(upperNode) != c.originOf(lowerNode)
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) originPortPositionOf(node *lgraph.LNode) int {
	origin := c.originPortOf(node)
	position, ok := c.portPositions[origin]
	if !ok {
		// The Java code unboxes null here.
		lgraph.Bail("greedyswitch: the origin port of a north south dummy has no position")
	}
	return position
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) originPortOf(node *lgraph.LNode) *lgraph.LPort {
	port := node.Ports[0]
	return counting.OriginPort(port)
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) isNorthOfNormalNode(upperNode *lgraph.LNode) bool {
	return c.originPortOf(upperNode).Side == core.PortSideNorth
}

// originOf is the cast (LNode) node.getProperty(ORIGIN). The layout stops
// for an origin that is not a node, where the Java code throws.
func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) originOf(node *lgraph.LNode) *lgraph.LNode {
	v := prop.Get(node, iprops.Origin)
	if v == nil {
		return nil
	}
	origin, ok := v.(*lgraph.LNode)
	if !ok {
		lgraph.Bail("greedyswitch: the origin of a north south dummy is not a node")
	}
	return origin
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) isLongEdgeDummy(node *lgraph.LNode) bool {
	return node.Type == lgraph.NodeTypeLongEdge
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) isNorthSouth(node *lgraph.LNode) bool {
	return node.Type == lgraph.NodeTypeNorthSouthPort
}

func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) isNormal(node *lgraph.LNode) bool {
	return node.Type == lgraph.NodeTypeNormal
}

// UpperLowerCrossings is getUpperLowerCrossings(): the crossings for the
// order upper - lower.
func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) UpperLowerCrossings() int {
	return c.upperLowerCrossings
}

// LowerUpperCrossings is getLowerUpperCrossings(): the crossings for the
// order lower - upper.
func (c *NorthSouthEdgeNeighbouringNodeCrossingsCounter) LowerUpperCrossings() int {
	return c.lowerUpperCrossings
}
