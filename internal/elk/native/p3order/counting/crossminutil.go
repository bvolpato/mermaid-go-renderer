// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/counting/CrossMinUtil.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package counting

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// InNorthSouthEastWestOrder is CrossMinUtil.inNorthSouthEastWestOrder(node,
// side): the ports of the node on the given side, from north to south or
// from east to west. The result is for reading only: for the sides EAST and
// NORTH it shares the memory of the port list.
func InNorthSouthEastWestOrder(node *lgraph.LNode, side core.PortSide) []*lgraph.LPort {
	switch side {
	case core.PortSideEast, core.PortSideNorth:
		return node.PortSideView(side)
	case core.PortSideSouth, core.PortSideWest:
		return Reverse(node.PortSideView(side))
	}
	return nil
}

// Reverse is Lists.reverse(list) for a list that is only read: a copy in
// the reverse order.
func Reverse(ports []*lgraph.LPort) []*lgraph.LPort {
	out := make([]*lgraph.LPort, len(ports))
	for i, p := range ports {
		out[len(ports)-1-i] = p
	}
	return out
}
