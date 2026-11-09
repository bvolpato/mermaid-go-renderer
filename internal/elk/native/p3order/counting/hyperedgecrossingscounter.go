// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/counting/HyperedgeCrossingsCounter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package counting

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// HyperedgeCrossingsCounter is the class HyperedgeCrossingsCounter: a
// crossings counter for hyperedges. For normal edges the number of crossings
// is exact. For hyperedges it is an estimate.
type HyperedgeCrossingsCounter struct {
	// portPos is the port position array for counting the edge crossings.
	portPos []int
}

// NewHyperedgeCrossingsCounter is new HyperedgeCrossingsCounter(
// inLayerEdgeCount, hasNorthSouthPorts, portPos). The Java constructor keeps
// only portPos.
func NewHyperedgeCrossingsCounter(inLayerEdgeCount []int, hasNorthSouthPorts []bool, portPos []int) *HyperedgeCrossingsCounter {
	return &HyperedgeCrossingsCounter{portPos: portPos}
}

// hyperedge is the class Hyperedge.
type hyperedge struct {
	edges      []*lgraph.LEdge
	ports      []*lgraph.LPort
	upperLeft  int
	lowerLeft  int
	upperRight int
	lowerRight int
	// hash stands for the identity hash code of the Java object. GWT gives
	// identity hash codes from a counter when an object is hashed the first
	// time. A hyperedge is hashed when it is made, because it is put into a
	// hash set. So the hash codes of two hyperedges compare as their
	// creation order, and compareTo uses only the sign of the difference.
	hash int
}

// compareTo is Hyperedge.compareTo(other).
func (h *hyperedge) compareTo(other *hyperedge) int {
	if h.upperLeft < other.upperLeft {
		return -1
	} else if h.upperLeft > other.upperLeft {
		return 1
	} else if h.upperRight < other.upperRight {
		return -1
	} else if h.upperRight > other.upperRight {
		return 1
	}
	return h.hash - other.hash
}

// cornerType is the enum HyperedgeCorner.Type.
type cornerType int

const (
	cornerUpper cornerType = iota
	cornerLower
)

// hyperedgeCorner is the class HyperedgeCorner: the upper left, lower left,
// upper right, or lower right corner of a hyperedge.
type hyperedgeCorner struct {
	hyperedge        *hyperedge
	position         int
	oppositePosition int
	typ              cornerType
}

// compareTo is HyperedgeCorner.compareTo(other).
func (c *hyperedgeCorner) compareTo(other *hyperedgeCorner) int {
	if c.position < other.position {
		return -1
	} else if c.position > other.position {
		return 1
	} else if c.oppositePosition < other.oppositePosition {
		return -1
	} else if c.oppositePosition > other.oppositePosition {
		return 1
	} else if c.hyperedge != other.hyperedge {
		return c.hyperedge.hash - other.hyperedge.hash
	} else if c.typ == cornerUpper && other.typ == cornerLower {
		return -1
	} else if c.typ == cornerLower && other.typ == cornerUpper {
		return 1
	}
	return 0
}

// countOpen sorts the corners and counts the crossings that the overlapping
// hyperedge areas of one side cause.
func countOpen(corners []*hyperedgeCorner) int {
	gwt.SortFunc(corners, (*hyperedgeCorner).compareTo)
	crossings := 0
	openHyperedges := 0
	for i := 0; i < len(corners); i++ {
		switch corners[i].typ {
		case cornerUpper:
			openHyperedges++
		case cornerLower:
			openHyperedges--
			crossings += openHyperedges
		}
	}
	return crossings
}

// CountCrossings is countCrossings(leftLayer, rightLayer): the special
// crossings counting method for hyperedges.
func (c *HyperedgeCrossingsCounter) CountCrossings(leftLayer, rightLayer []*lgraph.LNode) int {
	// Assign index values to the ports of the left layer
	sourceCount := 0
	for _, node := range leftLayer {
		// Assign index values in the order north - east - south - west
		for _, port := range node.Ports {
			portEdges := 0
			for _, edge := range port.OutgoingEdges {
				if node.Layer() != edge.Target.Node().Layer() {
					portEdges++
				}
			}
			if portEdges > 0 {
				c.portPos[port.ID] = sourceCount
				sourceCount++
			}
		}
	}

	// Assign index values to the ports of the right layer
	targetCount := 0
	for _, node := range rightLayer {
		// Determine how many input ports there are on the north side (note
		// that the standard port order is north - east - south - west)
		northInputPorts := 0
	northPorts:
		for _, port := range node.Ports {
			if port.Side == core.PortSideNorth {
				for _, edge := range port.IncomingEdges {
					if node.Layer() != edge.Source.Node().Layer() {
						northInputPorts++
						break
					}
				}
			} else {
				break northPorts
			}
		}
		// Assign index values in the order north - west - south - east
		otherInputPorts := 0
		for i := len(node.Ports) - 1; i >= 0; i-- {
			port := node.Ports[i]
			portEdges := 0
			for _, edge := range port.IncomingEdges {
				if node.Layer() != edge.Source.Node().Layer() {
					portEdges++
				}
			}
			if portEdges > 0 {
				if port.Side == core.PortSideNorth {
					c.portPos[port.ID] = targetCount
					targetCount++
				} else {
					c.portPos[port.ID] = targetCount + northInputPorts + otherInputPorts
					otherInputPorts++
				}
			}
		}
		targetCount += otherInputPorts
	}

	// Gather hyperedges
	port2HyperedgeMap := map[*lgraph.LPort]*hyperedge{}
	hyperedgeSet := gwt.NewSet[*hyperedge]()
	nextHash := 0
	for _, node := range leftLayer {
		for _, sourcePort := range node.Ports {
			for _, edge := range sourcePort.OutgoingEdges {
				targetPort := edge.Target
				if node.Layer() != targetPort.Node().Layer() {
					sourceHE := port2HyperedgeMap[sourcePort]
					targetHE := port2HyperedgeMap[targetPort]
					if sourceHE == nil && targetHE == nil {
						nextHash++
						he := &hyperedge{hash: nextHash}
						hyperedgeSet.Add(he)
						he.edges = append(he.edges, edge)
						he.ports = append(he.ports, sourcePort)
						port2HyperedgeMap[sourcePort] = he
						he.ports = append(he.ports, targetPort)
						port2HyperedgeMap[targetPort] = he
					} else if sourceHE == nil {
						targetHE.edges = append(targetHE.edges, edge)
						targetHE.ports = append(targetHE.ports, sourcePort)
						port2HyperedgeMap[sourcePort] = targetHE
					} else if targetHE == nil {
						sourceHE.edges = append(sourceHE.edges, edge)
						sourceHE.ports = append(sourceHE.ports, targetPort)
						port2HyperedgeMap[targetPort] = sourceHE
					} else if sourceHE == targetHE {
						sourceHE.edges = append(sourceHE.edges, edge)
					} else {
						sourceHE.edges = append(sourceHE.edges, edge)
						for _, p := range targetHE.ports {
							port2HyperedgeMap[p] = sourceHE
						}
						sourceHE.edges = append(sourceHE.edges, targetHE.edges...)
						sourceHE.ports = append(sourceHE.ports, targetHE.ports...)
						hyperedgeSet.Remove(targetHE)
					}
				}
			}
		}
	}

	// Determine top and bottom positions for each hyperedge
	hyperedges := hyperedgeSet.Values()
	leftLayerRef := leftLayer[0].Layer()
	rightLayerRef := rightLayer[0].Layer()
	for _, he := range hyperedges {
		he.upperLeft = sourceCount
		he.upperRight = targetCount
		for _, port := range he.ports {
			pos := c.portPos[port.ID]
			if port.Node().Layer() == leftLayerRef {
				if pos < he.upperLeft {
					he.upperLeft = pos
				}
				if pos > he.lowerLeft {
					he.lowerLeft = pos
				}
			} else if port.Node().Layer() == rightLayerRef {
				if pos < he.upperRight {
					he.upperRight = pos
				}
				if pos > he.lowerRight {
					he.lowerRight = pos
				}
			}
		}
	}

	// Determine the sequence of edge target positions sorted by source and
	// target index
	gwt.SortFunc(hyperedges, (*hyperedge).compareTo)
	southSequence := make([]int, len(hyperedges))
	compressDeltas := make([]int, targetCount+1)
	for i := 0; i < len(hyperedges); i++ {
		southSequence[i] = hyperedges[i].upperRight
		compressDeltas[southSequence[i]] = 1
	}
	delta := 0
	for i := 0; i < len(compressDeltas); i++ {
		if compressDeltas[i] == 1 {
			compressDeltas[i] = delta
		} else {
			delta--
		}
	}
	q := 0
	for i := 0; i < len(southSequence); i++ {
		southSequence[i] += compressDeltas[southSequence[i]]
		q = max(q, southSequence[i]+1)
	}

	// Build the accumulator tree
	firstIndex := 1
	for firstIndex < q {
		firstIndex *= 2
	}
	treeSize := 2*firstIndex - 1
	firstIndex -= 1
	tree := make([]int, treeSize)

	// Count the straight-line crossings of the topmost edges
	crossings := 0
	for k := 0; k < len(southSequence); k++ {
		index := southSequence[k] + firstIndex
		tree[index]++
		for index > 0 {
			if index%2 > 0 {
				crossings += tree[index+1]
			}
			index = (index - 1) / 2
			tree[index]++
		}
	}

	// Create corners for the left side
	leftCorners := make([]*hyperedgeCorner, len(hyperedges)*2)
	for i := 0; i < len(hyperedges); i++ {
		leftCorners[2*i] = &hyperedgeCorner{hyperedges[i], hyperedges[i].upperLeft, hyperedges[i].lowerLeft, cornerUpper}
		leftCorners[2*i+1] = &hyperedgeCorner{hyperedges[i], hyperedges[i].lowerLeft, hyperedges[i].upperLeft, cornerLower}
	}

	// Count crossings caused by overlapping hyperedge areas on the left side
	crossings += countOpen(leftCorners)

	// Create corners for the right side
	rightCorners := make([]*hyperedgeCorner, len(hyperedges)*2)
	for i := 0; i < len(hyperedges); i++ {
		rightCorners[2*i] = &hyperedgeCorner{hyperedges[i], hyperedges[i].upperRight, hyperedges[i].lowerRight, cornerUpper}
		rightCorners[2*i+1] = &hyperedgeCorner{hyperedges[i], hyperedges[i].lowerRight, hyperedges[i].upperRight, cornerLower}
	}

	// Count crossings caused by overlapping hyperedge areas on the right
	// side
	crossings += countOpen(rightCorners)

	return crossings
}
