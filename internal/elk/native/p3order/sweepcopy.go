// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/SweepCopy.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// SweepCopy is the class SweepCopy: it stores the node order and the port
// orders of a sweep.
type SweepCopy struct {
	// nodeOrder is a copy of the node order.
	nodeOrder [][]*lgraph.LNode
	// portOrders has a copy of the order of the ports of each node, because
	// they are reordered in each sweep.
	portOrders [][][]*lgraph.LPort
}

// NewSweepCopy is new SweepCopy(nodeOrderIn). It copies on construction.
func NewSweepCopy(nodeOrderIn [][]*lgraph.LNode) *SweepCopy {
	sc := &SweepCopy{nodeOrder: deepCopy(nodeOrderIn)}
	for _, lNodes := range nodeOrderIn {
		layer := make([][]*lgraph.LPort, 0, len(lNodes))
		for _, node := range lNodes {
			layer = append(layer, append([]*lgraph.LPort(nil), node.Ports...))
		}
		sc.portOrders = append(sc.portOrders, layer)
	}
	return sc
}

// NewSweepCopyOf is new SweepCopy(sc). The copy shares the port orders of
// each layer with sc.
func NewSweepCopyOf(sc *SweepCopy) *SweepCopy {
	return &SweepCopy{
		nodeOrder:  deepCopy(sc.nodeOrder),
		portOrders: append([][][]*lgraph.LPort(nil), sc.portOrders...),
	}
}

func deepCopy(currentlyBestNodeOrder [][]*lgraph.LNode) [][]*lgraph.LNode {
	if currentlyBestNodeOrder == nil {
		return nil
	}
	result := make([][]*lgraph.LNode, len(currentlyBestNodeOrder))
	for i := range result {
		result[i] = append([]*lgraph.LNode{}, currentlyBestNodeOrder[i]...)
	}
	return result
}

// Nodes is nodes(): the copy of the node orders. Do not change it.
func (sc *SweepCopy) Nodes() [][]*lgraph.LNode { return sc.nodeOrder }

// TransferNodeAndPortOrdersToGraph is transferNodeAndPortOrdersToGraph(
// lGraph, setPortContstraints).
func (sc *SweepCopy) TransferNodeAndPortOrdersToGraph(lGraph *lgraph.LGraph, setPortContstraints bool) {
	// The option ALLOW_NON_FLOW_PORTS_TO_SWITCH_SIDES allows the crossing
	// minimizer to decide the side on which a north south port dummy is
	// placed, to reduce the number of crossings. So the configured port side
	// may not be valid anymore, and it must be corrected.
	var northSouthPortDummies []*lgraph.LNode
	updatePortOrder := gwt.NewSet[*lgraph.LNode]()

	// iterate the layers
	layers := lGraph.Layers
	for i := 0; i < len(layers); i++ {
		nodes := layers[i].Nodes
		northSouthPortDummies = northSouthPortDummies[:0]

		// iterate and order the nodes within the layer
		for j := 0; j < len(nodes); j++ {
			node := sc.nodeOrder[i][j]
			// use the id field to remember the order within the layer
			node.ID = j
			if node.Type == lgraph.NodeTypeNorthSouthPort {
				northSouthPortDummies = append(northSouthPortDummies, node)
			}

			lGraph.Layers[i].Nodes[j] = node
			// order ports as computed
			node.Ports = append(node.Ports[:0], sc.portOrders[i][j]...)
			if setPortContstraints {
				if !prop.Get(node, lopts.PortConstraints).IsOrderFixed() {
					prop.Set(node, lopts.PortConstraints, core.PortConstraintsFixedOrder)
				}
			}
		}

		// assert that the port side is set properly
		for _, dummy := range northSouthPortDummies {
			origin := sc.assertCorrectPortSides(dummy)
			updatePortOrder.Add(origin)
			updatePortOrder.Add(dummy)
		}
	}

	// since the side of certain ports may have changed at this point, the
	// list of ports must be re-sorted (see PortListSorter) and the port
	// list views must be re-cached.
	for _, node := range updatePortOrder.Values() {
		gwt.SortFunc(node.Ports, cmpCombined)
		node.CachePortSides()
	}
}

// assertCorrectPortSides corrects the port side of the origin of the dummy.
// It returns the node whose port the dummy represents.
func (sc *SweepCopy) assertCorrectPortSides(dummy *lgraph.LNode) *lgraph.LNode {
	origin := prop.Get(dummy, iprops.InLayerLayoutUnit)

	// a north south port dummy has exactly one port
	dummyPorts := dummy.Ports
	dummyPort := dummyPorts[0]

	// find the corresponding port on the regular node
	for _, port := range origin.Ports {
		if any(port) == prop.Get(dummyPort, iprops.Origin) {
			// switch the port's side if necessary
			if port.Side == core.PortSideNorth && dummy.ID > origin.ID {
				port.SetSide(core.PortSideSouth)
				if port.ExplicitlySuppliedPortAnchor {
					// Set new coordinates for port anchor since it was
					// switched from NORTH to SOUTH. The y coordinate is
					// updated by mirroring the y coordinate
					portHeight := port.Size.Y
					anchorY := port.Anchor.Y
					port.Anchor.Y = portHeight - anchorY
				}
			} else if port.Side == core.PortSideSouth && origin.ID > dummy.ID {
				port.SetSide(core.PortSideNorth)
				if port.ExplicitlySuppliedPortAnchor {
					// Set new coordinates for port anchor since it was
					// switched from SOUTH to NORTH. The y coordinate is
					// updated by mirroring the y coordinate
					portHeight := port.Size.Y
					anchorY := port.Anchor.Y
					port.Anchor.Y = -(portHeight - anchorY)
				}
			}
			break
		}
	}
	return origin
}

// TODO(intermediate): cmpPortSide, cmpFixedOrderAndFixedPos and cmpCombined
// are the comparators CMP_PORT_SIDE, CMP_FIXED_ORDER_AND_FIXED_POS and
// CMP_COMBINED of intermediate/PortListSorter.java of Eclipse ELK 0.9.1.
// Use the ones of the package that ports PortListSorter instead.

// cmpPortSide is PortListSorter.CMP_PORT_SIDE: it sorts the ports of a node
// by their side, north - east - south - west.
func cmpPortSide(p1, p2 *lgraph.LPort) int {
	// Sort by side first (if the comparison ends here, the ports were on
	// different sides; otherwise, the ports must be on the same side)
	ordinalDifference := int(p1.Side) - int(p2.Side)
	if ordinalDifference != 0 {
		return ordinalDifference
	}

	return 0
}

// cmpFixedOrderAndFixedPos is PortListSorter.CMP_FIXED_ORDER_AND_FIXED_POS:
// it handles the order of ports of the same side when the port constraints
// are FIXED_ORDER or FIXED_POS.
func cmpFixedOrderAndFixedPos(p1, p2 *lgraph.LPort) int {
	portConstraints := prop.Get(p1.Node(), lopts.PortConstraints)

	// Do nothing if not at least the port order is fixed or the ports are
	// on different sides
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
	}
	// Port sides should not be undefined
	lgraph.Bail("Port side is undefined")
	return 0
}

// cmpCombined is PortListSorter.CMP_COMBINED: CMP_PORT_SIDE, then
// CMP_FIXED_ORDER_AND_FIXED_POS.
func cmpCombined(p1, p2 *lgraph.LPort) int {
	if c := cmpPortSide(p1, p2); c != 0 {
		return c
	}
	return cmpFixedOrderAndFixedPos(p1, p2)
}
