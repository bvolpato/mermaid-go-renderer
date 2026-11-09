// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/PortListSorter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package intermediate

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("PortListSorter", func() lgraph.Processor { return NewPortListSorter() })
}

// PortListSorter is the processor PortListSorter: it sorts the port list of
// each node by side, and by position or index where the order is fixed.
type PortListSorter struct{}

// NewPortListSorter is new PortListSorter().
func NewPortListSorter() *PortListSorter { return &PortListSorter{} }

// Process is process(layeredGraph, monitor).
func (s *PortListSorter) Process(layeredGraph *lgraph.LGraph, ctx *lgraph.Context) {
	pss := prop.Get(layeredGraph, lopts.PortSortingStrategy)

	// Iterate through the nodes of all layers
	for _, layer := range layeredGraph.Layers {
		for _, node := range layer.Nodes {
			portConstraints := prop.Get(node, lopts.PortConstraints)
			ports := node.Ports

			// We need to sort the port list according to port constraints
			if portConstraints.IsOrderFixed() {
				gwt.SortFunc(ports, CmpCombined)
			} else if portConstraints.IsSideFixed() {
				// by default try to preserve the input order
				gwt.SortFunc(ports, CmpPortSide)
				s.reverseWestAndSouthSide(ports)

				if pss == lenum.PortSortingStrategyPortDegree {
					gwt.SortFunc(ports, CmpPortDegreeEastWest)
				}
			}
			node.CachePortSides()
		}
	}
}

func (s *PortListSorter) reverseWestAndSouthSide(ports []*lgraph.LPort) {
	if len(ports) <= 1 {
		return
	}

	southLow, southHigh := s.findPortSideRange(ports, core.PortSideSouth)
	s.reverse(ports, southLow, southHigh)

	westLow, westHigh := s.findPortSideRange(ports, core.PortSideWest)
	s.reverse(ports, westLow, westHigh)
}

// findPortSideRange returns indices [i, j) of the sorted port list. The Java
// comment says that i is the first index of a port on the side, and that j
// is the index of the first port of another side or the size of the list.
// The second loop of the Java code reads the side at lowIdx, not at highIdx,
// so j is lowIdx or the last index of the list. The port keeps that.
func (s *PortListSorter) findPortSideRange(ports []*lgraph.LPort, side core.PortSide) (int, int) {
	if len(ports) == 0 {
		return 0, 0
	}

	currentSide := ports[0].Side
	lowIdx := 0
	lb := int(side)
	hb := int(side) + 1

	for lowIdx < len(ports)-1 && int(currentSide) < lb {
		lowIdx++
		currentSide = ports[lowIdx].Side
	}
	highIdx := lowIdx
	for highIdx < len(ports)-1 && int(currentSide) < hb {
		highIdx++
		currentSide = ports[lowIdx].Side
	}

	return lowIdx, highIdx
}

// reverse reverses the port list between lowIdx (inclusive) and highIdx
// (exclusive).
func (s *PortListSorter) reverse(ports []*lgraph.LPort, lowIdx, highIdx int) {
	if highIdx <= lowIdx+2 {
		return
	}
	n := (highIdx - lowIdx) / 2
	for i := 0; i < n; i++ {
		tmp := ports[lowIdx+i]
		ports[lowIdx+i] = ports[highIdx-i-1]
		ports[highIdx-i-1] = tmp
	}
}

// CmpPortSide is PortListSorter.CMP_PORT_SIDE: it orders ports by side,
// north - east - south - west. It does not change the order of the ports of
// one side.
func CmpPortSide(p1, p2 *lgraph.LPort) int {
	// Sort by side first (if the comparison ends here, the ports were on
	// different sides; otherwise, the ports must be on the same side)
	ordinalDifference := int(p1.Side) - int(p2.Side)
	if ordinalDifference != 0 {
		return ordinalDifference
	}

	return 0
}

// CmpPortDegreeEastWest is PortListSorter.CMP_PORT_DEGREE_EAST_WEST: on the
// east side it orders ports by their out-degree in the input graph, and on
// the west side by their in-degree. The ports must be sorted by side.
func CmpPortDegreeEastWest(p1, p2 *lgraph.LPort) int {
	ordinalDifference := int(p1.Side) - int(p2.Side)
	if ordinalDifference != 0 {
		return 0 // ports on different sides -- not our job
	}

	switch p1.Side {
	case core.PortSideEast:
		return realDegree(p2.OutgoingEdges) - realDegree(p1.OutgoingEdges)

	case core.PortSideWest:
		return realDegree(p1.IncomingEdges) - realDegree(p2.IncomingEdges)
	}

	return 0
}

// realDegree counts the edges that are not reversed. The Java method takes
// the port and a function that returns its incoming or outgoing edges.
func realDegree(edges []*lgraph.LEdge) int {
	realDegree := 0
	for _, e := range edges {
		if !prop.Get(e, iprops.Reversed) {
			realDegree++
		}
	}
	return realDegree
}

// CmpFixedOrderAndFixedPos is PortListSorter.CMP_FIXED_ORDER_AND_FIXED_POS:
// it orders the ports of one side of a node whose port constraints fix the
// order. The ports must be sorted by side.
func CmpFixedOrderAndFixedPos(p1, p2 *lgraph.LPort) int {
	portConstraints := prop.Get(p1.Node(), lopts.PortConstraints)

	// Do nothing if not at least the port order is fixed or the ports are on
	// different sides
	ordinalDifference := int(p1.Side) - int(p2.Side)
	if ordinalDifference != 0 || !portConstraints.IsOrderFixed() {
		return 0
	}

	// If the ports are on the same side and the node has FIXED_ORDER port
	// constraints (that is, the coordinates of the ports don't necessarily
	// make sense), we check if the port index has been explicitly set
	if portConstraints == core.PortConstraintsFixedOrder {
		// In case of equal sides, sort by port index property
		index1, has1 := prop.Lookup(p1, lopts.PortIndex)
		index2, has2 := prop.Lookup(p2, lopts.PortIndex)
		if has1 && has2 {
			// GWT subtracts the two ints as doubles, so the difference does
			// not wrap around as it does in the JVM.
			indexDifference := index1 - index2
			if indexDifference != 0 {
				return indexDifference
			}
		}
	}

	// In case of equal index (or FIXED_POS), sort by position
	switch p1.Side {
	case core.PortSideNorth:
		// Compare x coordinates
		return gwt.DoubleCompare(p1.Pos.X, p2.Pos.X)

	case core.PortSideEast:
		// Compare y coordinates
		return gwt.DoubleCompare(p1.Pos.Y, p2.Pos.Y)

	case core.PortSideSouth:
		// Compare x coordinates in reversed order
		return gwt.DoubleCompare(p2.Pos.X, p1.Pos.X)

	case core.PortSideWest:
		// Compare y coordinates in reversed order
		return gwt.DoubleCompare(p2.Pos.Y, p1.Pos.Y)

	default:
		// Port sides should not be undefined. Java throws an
		// IllegalStateException.
		lgraph.Bail("Port side is undefined")
		return 0
	}
}

// CmpCombined is PortListSorter.CMP_COMBINED: CMP_PORT_SIDE, then
// CMP_FIXED_ORDER_AND_FIXED_POS.
func CmpCombined(p1, p2 *lgraph.LPort) int {
	if c := CmpPortSide(p1, p2); c != 0 {
		return c
	}
	return CmpFixedOrderAndFixedPos(p1, p2)
}
