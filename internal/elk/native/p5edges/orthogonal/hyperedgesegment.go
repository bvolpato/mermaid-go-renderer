// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/orthogonal/HyperEdgeSegment.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package orthogonal

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p5edges/orthogonal/direction"
)

// HyperEdgeSegment is the class HyperEdgeSegment: the trunk of a hyper edge.
// In a layout from left to right it is the vertical segment of the hyper
// edge. It has a sorted list of coordinates for each of its two sides, where
// the incoming and the outgoing connections meet the segment.
//
// The Java class compares, hashes and tests equality by the value of mark.
// CompareTo, Equals and HashCode do the same.
type HyperEdgeSegment struct {
	// routingStrategy decides how the edges are routed. It can be nil for a
	// segment that AddPortPositions is not called on.
	routingStrategy direction.BaseRoutingDirectionStrategy
	// ports has the ports that the segment represents.
	ports []*lgraph.LPort

	// mark is the mark value of the cycle breaking.
	mark int

	// routingSlot decides the distance to the preceding layer.
	routingSlot int

	// startPosition and endPosition are the ends of the segment. They are
	// NaN for a segment without a connection.
	startPosition float64
	endPosition   float64

	// incomingConnectionCoordinates and outgoingConnectionCoordinates are the
	// sorted lists of the coordinates of the connections.
	incomingConnectionCoordinates []float64
	outgoingConnectionCoordinates []float64

	outgoingSegmentDependencies []*HyperEdgeSegmentDependency
	outDepWeight                int
	criticalOutDepWeight        int
	incomingSegmentDependencies []*HyperEdgeSegmentDependency
	inDepWeight                 int
	criticalInDepWeight         int

	// splitPartner is the other segment of a split, or nil.
	splitPartner *HyperEdgeSegment
	// splitBy is the segment that caused the split. One of the two split
	// partners has it.
	splitBy *HyperEdgeSegment
}

// NewHyperEdgeSegment is new HyperEdgeSegment(routingStrategy). The strategy
// can be nil when AddPortPositions is not called.
func NewHyperEdgeSegment(routingStrategy direction.BaseRoutingDirectionStrategy) *HyperEdgeSegment {
	return &HyperEdgeSegment{
		routingStrategy: routingStrategy,
		startPosition:   math.NaN(),
		endPosition:     math.NaN(),
	}
}

// AddPortPositions is addPortPositions(port, hyperEdgeSegmentMap): it adds
// the positions of the port and of each port that is connected to it.
func (s *HyperEdgeSegment) AddPortPositions(port *lgraph.LPort, hyperEdgeSegmentMap *gwt.Map[*lgraph.LPort, *HyperEdgeSegment]) {
	// Java fails with a NullPointerException in these cases.
	if s.routingStrategy == nil {
		lgraph.Bail("HyperEdgeSegment: addPortPositions needs a routing strategy")
	}
	if port == nil || port.Node() == nil {
		lgraph.Bail("HyperEdgeSegment: a port of a hyper edge has no node")
	}

	hyperEdgeSegmentMap.Put(port, s)
	s.ports = append(s.ports, port)
	portPos := s.routingStrategy.PortPositionOnHyperNode(port)

	// add the new port position to the respective list
	if port.Side == s.routingStrategy.SourcePortSide() {
		s.incomingConnectionCoordinates = insertSorted(s.incomingConnectionCoordinates, portPos)
	} else {
		s.outgoingConnectionCoordinates = insertSorted(s.outgoingConnectionCoordinates, portPos)
	}

	// update start and end coordinates
	s.RecomputeExtent()

	// add connected ports
	for _, otherPort := range port.ConnectedPorts() {
		if !hyperEdgeSegmentMap.ContainsKey(otherPort) {
			s.AddPortPositions(otherPort, hyperEdgeSegmentMap)
		}
	}
}

// insertSorted is insertSorted(list, value): it puts the value before the
// first larger value, and does nothing when the list has the value.
func insertSorted(list []float64, value float64) []float64 {
	i := 0
	for i < len(list) {
		next := list[i]
		i++
		if next == value {
			// an exactly equal value is already present in the list
			return list
		} else if next > value {
			i--
			break
		}
	}
	list = append(list, 0)
	copy(list[i+1:], list[i:])
	list[i] = value
	return list
}

// Ports is getPorts(): the ports incident to the segment.
func (s *HyperEdgeSegment) Ports() []*lgraph.LPort { return s.ports }

// RoutingSlot is getRoutingSlot().
func (s *HyperEdgeSegment) RoutingSlot() int { return s.routingSlot }

// SetRoutingSlot is setRoutingSlot(slot).
func (s *HyperEdgeSegment) SetRoutingSlot(slot int) { s.routingSlot = slot }

// StartCoordinate is getStartCoordinate().
func (s *HyperEdgeSegment) StartCoordinate() float64 { return s.startPosition }

// EndCoordinate is getEndCoordinate().
func (s *HyperEdgeSegment) EndCoordinate() float64 { return s.endPosition }

// IncomingConnectionCoordinates is getIncomingConnectionCoordinates(): the
// sorted coordinates where incoming connections enter the segment. The
// result is the list of the segment: do not change it.
func (s *HyperEdgeSegment) IncomingConnectionCoordinates() []float64 {
	return s.incomingConnectionCoordinates
}

// OutgoingConnectionCoordinates is getOutgoingConnectionCoordinates(): the
// sorted coordinates where outgoing connections leave the segment. The
// result is the list of the segment: do not change it.
func (s *HyperEdgeSegment) OutgoingConnectionCoordinates() []float64 {
	return s.outgoingConnectionCoordinates
}

// OutgoingSegmentDependencies is getOutgoingSegmentDependencies(). The
// result is the list of the segment: do not change it.
func (s *HyperEdgeSegment) OutgoingSegmentDependencies() []*HyperEdgeSegmentDependency {
	return s.outgoingSegmentDependencies
}

// OutWeight is getOutWeight().
func (s *HyperEdgeSegment) OutWeight() int { return s.outDepWeight }

// SetOutWeight is setOutWeight(outWeight).
func (s *HyperEdgeSegment) SetOutWeight(outWeight int) { s.outDepWeight = outWeight }

// CriticalOutWeight is getCriticalOutWeight().
func (s *HyperEdgeSegment) CriticalOutWeight() int { return s.criticalOutDepWeight }

// SetCriticalOutWeight is setCriticalOutWeight(outWeight).
func (s *HyperEdgeSegment) SetCriticalOutWeight(outWeight int) { s.criticalOutDepWeight = outWeight }

// IncomingSegmentDependencies is getIncomingSegmentDependencies(). The
// result is the list of the segment: do not change it.
func (s *HyperEdgeSegment) IncomingSegmentDependencies() []*HyperEdgeSegmentDependency {
	return s.incomingSegmentDependencies
}

// InWeight is getInWeight().
func (s *HyperEdgeSegment) InWeight() int { return s.inDepWeight }

// SetInWeight is setInWeight(inWeight).
func (s *HyperEdgeSegment) SetInWeight(inWeight int) { s.inDepWeight = inWeight }

// CriticalInWeight is getCriticalInWeight().
func (s *HyperEdgeSegment) CriticalInWeight() int { return s.criticalInDepWeight }

// SetCriticalInWeight is setCriticalInWeight(inWeight).
func (s *HyperEdgeSegment) SetCriticalInWeight(inWeight int) { s.criticalInDepWeight = inWeight }

// SplitPartner is getSplitPartner(): the other segment of a split, or nil.
func (s *HyperEdgeSegment) SplitPartner() *HyperEdgeSegment { return s.splitPartner }

// SplitPartnerSegment is getSplitPartner() for the routing direction
// strategies. It is a nil interface for a segment that was not split.
func (s *HyperEdgeSegment) SplitPartnerSegment() direction.HyperEdgeSegment {
	if s.splitPartner == nil {
		return nil
	}
	return s.splitPartner
}

// SetSplitPartner is setSplitPartner(splitPartner).
func (s *HyperEdgeSegment) SetSplitPartner(splitPartner *HyperEdgeSegment) {
	s.splitPartner = splitPartner
}

// SplitBy is getSplitBy(): the segment that caused the split, or nil.
func (s *HyperEdgeSegment) SplitBy() *HyperEdgeSegment { return s.splitBy }

// SetSplitBy is setSplitBy(splitBy).
func (s *HyperEdgeSegment) SetSplitBy(splitBy *HyperEdgeSegment) { s.splitBy = splitBy }

// Length is getLength(): the end coordinate minus the start coordinate.
func (s *HyperEdgeSegment) Length() float64 {
	return s.EndCoordinate() - s.StartCoordinate()
}

// RepresentsHyperedge is representsHyperedge(): the segment connects more
// than two ports.
func (s *HyperEdgeSegment) RepresentsHyperedge() bool {
	return len(s.incomingConnectionCoordinates)+len(s.outgoingConnectionCoordinates) > 2
}

// IsDummy is isDummy(): the segment was made by the split of another
// segment.
func (s *HyperEdgeSegment) IsDummy() bool {
	return s.splitPartner != nil && s.splitBy == nil
}

// RecomputeExtent is recomputeExtent(): it sets the start and the end
// coordinate from the connection coordinates.
func (s *HyperEdgeSegment) RecomputeExtent() {
	s.startPosition = math.NaN()
	s.endPosition = math.NaN()

	s.recomputeExtentFor(s.incomingConnectionCoordinates)
	s.recomputeExtentFor(s.outgoingConnectionCoordinates)
}

// recomputeExtentFor is recomputeExtent(positions). Math.min and Math.max
// are the builtins min and max of Go: math.Min and math.Max give an infinity
// where JavaScript gives NaN for a NaN and an infinity.
func (s *HyperEdgeSegment) recomputeExtentFor(positions []float64) {
	// this code assumes that the positions are sorted ascendingly
	if len(positions) != 0 {
		// set new start position
		if s.startPosition != s.startPosition {
			s.startPosition = positions[0]
		} else {
			s.startPosition = min(s.startPosition, positions[0])
		}

		// set new end position
		if s.endPosition != s.endPosition {
			s.endPosition = positions[len(positions)-1]
		} else {
			s.endPosition = max(s.endPosition, positions[len(positions)-1])
		}
	}
}

// SimulateSplit is simulateSplit(): two new segments that show what a split
// does. The first stands for this segment and the second for the split
// partner. They have the connection coordinates of this segment, without
// the connection between the two.
func (s *HyperEdgeSegment) SimulateSplit() (newSplit, newSplitPartner *HyperEdgeSegment) {
	newSplit = NewHyperEdgeSegment(s.routingStrategy)
	newSplitPartner = NewHyperEdgeSegment(s.routingStrategy)

	newSplit.incomingConnectionCoordinates = append(newSplit.incomingConnectionCoordinates, s.incomingConnectionCoordinates...)
	newSplit.splitBy = s.splitBy
	newSplit.splitPartner = newSplitPartner
	newSplit.RecomputeExtent()

	newSplitPartner.outgoingConnectionCoordinates = append(newSplitPartner.outgoingConnectionCoordinates, s.outgoingConnectionCoordinates...)
	newSplitPartner.splitPartner = newSplit
	newSplitPartner.RecomputeExtent()

	return newSplit, newSplitPartner
}

// SplitAt is splitAt(splitPosition): it splits the segment into two and
// returns the new segment. This segment keeps its incoming connections, and
// its outgoing connections move to the new segment. The two are connected at
// the split position. Each dependency of this segment is removed.
func (s *HyperEdgeSegment) SplitAt(splitPosition float64) *HyperEdgeSegment {
	s.splitPartner = NewHyperEdgeSegment(s.routingStrategy)
	s.splitPartner.SetSplitPartner(s)

	// Move all target positions over to the new segment
	s.splitPartner.outgoingConnectionCoordinates = append(s.splitPartner.outgoingConnectionCoordinates, s.outgoingConnectionCoordinates...)
	s.outgoingConnectionCoordinates = nil

	// Link the two
	s.outgoingConnectionCoordinates = append(s.outgoingConnectionCoordinates, splitPosition)
	s.splitPartner.incomingConnectionCoordinates = append(s.splitPartner.incomingConnectionCoordinates, splitPosition)

	// Recompute their outer coordinates
	s.RecomputeExtent()
	s.splitPartner.RecomputeExtent()

	// Clear dependencies so they can be regenerated later.
	for len(s.incomingSegmentDependencies) != 0 {
		s.incomingSegmentDependencies[0].Remove()
	}

	for len(s.outgoingSegmentDependencies) != 0 {
		s.outgoingSegmentDependencies[0].Remove()
	}

	return s.splitPartner
}

// CompareTo is compareTo(other): the difference of the marks.
func (s *HyperEdgeSegment) CompareTo(other *HyperEdgeSegment) int {
	return s.mark - other.mark
}

// Equals is equals(object): the two segments have the same mark.
func (s *HyperEdgeSegment) Equals(other *HyperEdgeSegment) bool {
	return other != nil && s.mark == other.mark
}

// HashCode is hashCode(): the mark.
func (s *HyperEdgeSegment) HashCode() int { return s.mark }

// String is toString().
func (s *HyperEdgeSegment) String() string {
	var builder strings.Builder
	builder.WriteByte('{')
	for i, port := range s.ports {
		// LNode.getDesignation does not return null, so the Java branch for a
		// null name does not run.
		builder.WriteString(port.Node().Designation())
		if i+1 < len(s.ports) {
			builder.WriteByte(',')
		}
	}
	builder.WriteByte('}')
	return builder.String()
}
