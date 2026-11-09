// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/SelfHyperLoop.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package loops

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// PortSideCount is PortSide.values().length: the length of an array that
// has one entry for each ordinal of PortSide.
const PortSideCount = 5

// SLPortsBySide is the ListMultimap<PortSide, SelfLoopPort> of a hyper loop:
// the ports of the loop on each side. It is an ArrayListMultimap of Guava on
// a HashMap of GWT, so the sides iterate in the order of their first
// insertion.
type SLPortsBySide struct {
	m *gwt.Map[core.PortSide, []*SelfLoopPort]
}

// NewSLPortsBySide is ArrayListMultimap.create().
func NewSLPortsBySide() *SLPortsBySide {
	return &SLPortsBySide{m: gwt.NewMap[core.PortSide, []*SelfLoopPort]()}
}

// Put is put(portSide, slPort).
func (m *SLPortsBySide) Put(portSide core.PortSide, slPort *SelfLoopPort) {
	m.m.Put(portSide, append(m.m.Get(portSide), slPort))
}

// Get is get(portSide): the ports on the side in insertion order. It is
// empty for a side without ports.
func (m *SLPortsBySide) Get(portSide core.PortSide) []*SelfLoopPort {
	return m.m.Get(portSide)
}

// KeySet is keySet(): the sides that have ports, in iteration order.
func (m *SLPortsBySide) KeySet() []core.PortSide { return m.m.Keys() }

// ContainsKey is containsKey(portSide).
func (m *SLPortsBySide) ContainsKey(portSide core.PortSide) bool { return m.m.ContainsKey(portSide) }

// SelfHyperLoop is the class SelfHyperLoop: a self loop hyperedge that has
// one self loop edge or more.
//
// The edges of a hyper loop are drawn as one trunk that runs around the
// node, with branches to the ports. The trunk leaves the leftmost port to
// the right and reaches the rightmost port from the left. The routing
// director sets both ports when the port order is known.
type SelfHyperLoop struct {
	// SLHolder is the holder that owns this instance (getSLHolder()).
	SLHolder *SelfLoopHolder
	// SLPorts has the ports of the hyper loop (getSLPorts()). The routing
	// director sorts it by the order of the ports of the node.
	SLPorts []*SelfLoopPort
	// SLEdges is the set of the edges of the hyper loop (getSLEdges()), in
	// the iteration order of the HashSet of GWT: the order of insertion.
	SLEdges []*SelfLoopEdge
	// SLLabels has the labels of the hyper loop (getSLLabels()), or nil.
	SLLabels *SelfHyperLoopLabels

	// SelfLoopType is the type of the loop (getSelfLoopType()). It has a
	// value when SLPortsBySide is not nil.
	SelfLoopType SelfLoopType
	// SLPortsBySide has the ports of each side (getSLPortsBySide()). It is
	// nil until ComputePortsPerSide.
	SLPortsBySide *SLPortsBySide
	// LeftmostPort is the leftmost port of the trunk (getLeftmostPort()).
	LeftmostPort *SelfLoopPort
	// RightmostPort is the rightmost port of the trunk (getRightmostPort()).
	RightmostPort *SelfLoopPort
	// OccupiedPortSides is the set of the sides that the loop is routed
	// along (getOccupiedPortSides()).
	OccupiedPortSides *prop.EnumSet[core.PortSide]
	// RoutingSlot is the routing slot of the loop on each side, by the
	// ordinal of the side. Set an entry with SetRoutingSlot.
	RoutingSlot [PortSideCount]int
}

// NewSelfHyperLoop is new SelfHyperLoop(slHolder).
func NewSelfHyperLoop(slHolder *SelfLoopHolder) *SelfHyperLoop {
	return &SelfHyperLoop{SLHolder: slHolder, OccupiedPortSides: prop.NoneOf[core.PortSide]()}
}

// ComputePortsPerSide is computePortsPerSide(): it fills SLPortsBySide and
// sets the type of the loop.
func (l *SelfHyperLoop) ComputePortsPerSide() {
	// Remember the ports of each side.
	l.SLPortsBySide = NewSLPortsBySide()
	for _, slPort := range l.SLPorts {
		portSide := slPort.LPort.Side
		l.SLPortsBySide.Put(portSide, slPort)
	}

	// Find the type of the loop.
	l.SelfLoopType = SelfLoopTypeFromPortSides(l.SLPortsBySide.KeySet())
}

// AddSelfLoopEdge is addSelfLoopEdge(slEdge): it adds the edge, its ports
// and its labels to this instance, unless the edge is in it.
func (l *SelfHyperLoop) AddSelfLoopEdge(slEdge *SelfLoopEdge) {
	for _, e := range l.SLEdges {
		if e == slEdge {
			return
		}
	}
	l.SLEdges = append(l.SLEdges, slEdge)
	slEdge.SetSLHyperLoop(l)

	slSource := slEdge.SLSource
	if !l.containsSLPort(slSource) {
		l.SLPorts = append(l.SLPorts, slSource)
	}

	slTarget := slEdge.SLTarget
	if !l.containsSLPort(slTarget) {
		l.SLPorts = append(l.SLPorts, slTarget)
	}

	// Take the labels of the edge.
	lLabels := slEdge.LEdge.Labels
	if len(lLabels) != 0 {
		if l.SLLabels == nil {
			l.SLLabels = NewSelfHyperLoopLabels(l)
		}

		l.SLLabels.AddLLabels(lLabels)
	}
}

func (l *SelfHyperLoop) containsSLPort(slPort *SelfLoopPort) bool {
	for _, p := range l.SLPorts {
		if p == slPort {
			return true
		}
	}
	return false
}

// SLPortsOnSide is getSLPortsBySide(portSide): the ports of the loop on the
// given side. The layout stops when the ports for each side are not
// computed, where Java fails with a NullPointerException.
func (l *SelfHyperLoop) SLPortsOnSide(portSide core.PortSide) []*SelfLoopPort {
	return l.PortSides().Get(portSide)
}

// PortSides is getSLPortsBySide(). The layout stops when the ports for each
// side are not computed, where Java fails with a NullPointerException on
// the first use of the map.
func (l *SelfHyperLoop) PortSides() *SLPortsBySide {
	if l.SLPortsBySide == nil {
		lgraph.Bail("SelfHyperLoop: the ports for each side of a self loop are not computed")
	}
	return l.SLPortsBySide
}

// GetSelfLoopType is getSelfLoopType(). The layout stops when the loop has
// no type, where Java fails with a NullPointerException on the first use of
// the type.
func (l *SelfHyperLoop) GetSelfLoopType() SelfLoopType {
	if l.SLPortsBySide == nil {
		lgraph.Bail("SelfHyperLoop: a self loop has no type")
	}
	return l.SelfLoopType
}

// SetLeftmostPort is setLeftmostPort(leftmostPort).
func (l *SelfHyperLoop) SetLeftmostPort(leftmostPort *SelfLoopPort) { l.LeftmostPort = leftmostPort }

// SetRightmostPort is setRightmostPort(rightmostPort).
func (l *SelfHyperLoop) SetRightmostPort(rightmostPort *SelfLoopPort) {
	l.RightmostPort = rightmostPort
}

// GetRoutingSlot is getRoutingSlot(portSide).
func (l *SelfHyperLoop) GetRoutingSlot(portSide core.PortSide) int {
	return l.RoutingSlot[portSide]
}

// SetRoutingSlot is setRoutingSlot(portSide, slot): it sets the routing slot
// of the loop on the given side, and makes the number of routing slots of
// the holder on that side large enough.
func (l *SelfHyperLoop) SetRoutingSlot(portSide core.PortSide, slot int) {
	l.RoutingSlot[portSide] = slot

	slotCount := &l.SLHolder.RoutingSlotCount
	slotCount[portSide] = max(slotCount[portSide], slot+1)
}
