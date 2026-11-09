// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/orthogonal/direction/BaseRoutingDirectionStrategy.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package direction

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// Tolerance is OrthogonalRoutingGenerator.TOLERANCE: a difference below it is
// zero. The package orthogonal imports this package, so the constant is
// declared here, and orthogonal.Tolerance has the same value.
const Tolerance = 0.001

// HyperEdgeSegment is the part of the class HyperEdgeSegment that a strategy
// reads. The Java strategies take the class itself. The class is in the
// package orthogonal, which imports this package, so the strategies take
// this interface. *orthogonal.HyperEdgeSegment implements it.
type HyperEdgeSegment interface {
	// IsDummy is isDummy().
	IsDummy() bool
	// RoutingSlot is getRoutingSlot().
	RoutingSlot() int
	// Ports is getPorts().
	Ports() []*lgraph.LPort
	// StartCoordinate is getStartCoordinate().
	StartCoordinate() float64
	// EndCoordinate is getEndCoordinate().
	EndCoordinate() float64
	// IncomingConnectionCoordinates is getIncomingConnectionCoordinates().
	IncomingConnectionCoordinates() []float64
	// OutgoingConnectionCoordinates is getOutgoingConnectionCoordinates().
	OutgoingConnectionCoordinates() []float64
	// SplitPartnerSegment is getSplitPartner() as this interface. It is nil
	// for a segment that was not split.
	SplitPartnerSegment() HyperEdgeSegment
}

// BaseRoutingDirectionStrategy is the abstract class
// BaseRoutingDirectionStrategy: the methods that each strategy has.
// ForRoutingDirection returns the strategy of a direction.
type BaseRoutingDirectionStrategy interface {
	// PortPositionOnHyperNode is getPortPositionOnHyperNode(port): the
	// position of the port on the axis of the hyper edge segments.
	PortPositionOnHyperNode(port *lgraph.LPort) float64
	// SourcePortSide is getSourcePortSide(): the side of the ports of a
	// source layer that the router handles.
	SourcePortSide() core.PortSide
	// TargetPortSide is getTargetPortSide(): the side of the ports of a
	// target layer that the router handles.
	TargetPortSide() core.PortSide
	// CalculateBendPoints is calculateBendPoints(hyperNode, startPos,
	// edgeSpacing): it adds the bend points of the edges of the segment.
	CalculateBendPoints(hyperNode HyperEdgeSegment, startPos, edgeSpacing float64)
	// CreatedJunctionPoints is getCreatedJunctionPoints(): the junction
	// points made so far, in the iteration order of the Java set.
	CreatedJunctionPoints() []*emath.KVector
	// ClearCreatedJunctionPoints is clearCreatedJunctionPoints().
	ClearCreatedJunctionPoints()
}

// ForRoutingDirection is forRoutingDirection(direction). It stops the layout
// for a value that is not a direction, where Java throws an
// IllegalArgumentException.
func ForRoutingDirection(direction RoutingDirection) BaseRoutingDirectionStrategy {
	switch direction {
	case RoutingDirectionWestToEast:
		return NewWestToEastRoutingStrategy()
	case RoutingDirectionNorthToSouth:
		return NewNorthToSouthRoutingStrategy()
	case RoutingDirectionSouthToNorth:
		return NewSouthToNorthRoutingStrategy()
	}
	lgraph.Bail("BaseRoutingDirectionStrategy: no strategy for the routing direction %d", int(direction))
	return nil
}

// baseStrategy has the state and the methods that the Java base class gives
// to each strategy.
type baseStrategy struct {
	// createdJunctionPoints is the set of the junction points made so far: a
	// HashSet of KVector, with the hashCode and the equals of KVector.
	createdJunctionPoints *gwt.HashMap[*emath.KVector, struct{}]
}

func newJunctionPointSet() *gwt.HashMap[*emath.KVector, struct{}] {
	return gwt.NewHashMap[*emath.KVector, struct{}](
		func(v *emath.KVector) int32 { return v.HashCode() },
		func(a, b *emath.KVector) bool { return a.Equals(b) },
	)
}

func newBaseStrategy() baseStrategy {
	return baseStrategy{createdJunctionPoints: newJunctionPointSet()}
}

// addJunctionPointIfNecessary is addJunctionPointIfNecessary(edge, segment,
// pos, vertical): it adds a junction point to the edge when the bend point
// is not at one of the two ends of the hyper edge segment.
func (b *baseStrategy) addJunctionPointIfNecessary(edge *lgraph.LEdge, segment HyperEdgeSegment, pos *emath.KVector, vertical bool) {
	p := pos.X
	if vertical {
		p = pos.Y
	}

	// If we already have this junction point, don't bother
	if b.createdJunctionPoints.ContainsKey(pos) {
		return
	}

	// Whether the point lies somewhere inside the edge segment (without
	// boundaries)
	pointInsideEdgeSegment := p > segment.StartCoordinate() && p < segment.EndCoordinate()

	// Check if the point lies somewhere at the segment's boundary
	pointAtSegmentBoundary := false
	incoming := segment.IncomingConnectionCoordinates()
	outgoing := segment.OutgoingConnectionCoordinates()
	if len(incoming) != 0 && len(outgoing) != 0 {
		// Is the bend point at the start and joins another edge at the same
		// position?
		pointAtSegmentBoundary = pointAtSegmentBoundary ||
			(math.Abs(p-incoming[0]) < Tolerance && math.Abs(p-outgoing[0]) < Tolerance)

		// Is the bend point at the end and joins another edge at the same
		// position?
		pointAtSegmentBoundary = pointAtSegmentBoundary ||
			(math.Abs(p-incoming[len(incoming)-1]) < Tolerance && math.Abs(p-outgoing[len(outgoing)-1]) < Tolerance)
	}

	if pointInsideEdgeSegment || pointAtSegmentBoundary {
		// create a new junction point for the edge at the bend point's
		// position
		junctionPoints := prop.Get(edge, lopts.JunctionPoints)
		if junctionPoints == nil {
			junctionPoints = emath.NewKVectorChain()
			prop.Set(edge, lopts.JunctionPoints, junctionPoints)
		}

		jpoint := pos.Clone()
		junctionPoints.Add(jpoint)
		b.createdJunctionPoints.Put(jpoint, struct{}{})
	}
}

// CreatedJunctionPoints is getCreatedJunctionPoints().
func (b *baseStrategy) CreatedJunctionPoints() []*emath.KVector {
	return b.createdJunctionPoints.Keys()
}

// ClearCreatedJunctionPoints is clearCreatedJunctionPoints().
func (b *baseStrategy) ClearCreatedJunctionPoints() {
	if b.createdJunctionPoints.Len() != 0 {
		b.createdJunctionPoints = newJunctionPointSet()
	}
}

// edgeTarget returns the target port of an edge of a segment. Java fails
// with a NullPointerException for an edge without a target or for a port
// without a node, so the layout stops.
func edgeTarget(edge *lgraph.LEdge) *lgraph.LPort {
	target := edge.Target
	if target == nil || target.Node() == nil {
		lgraph.Bail("BaseRoutingDirectionStrategy: an edge of a hyper edge segment has no target node")
	}
	return target
}

// splitCoordinate is splitPartner.getIncomingConnectionCoordinates().get(0).
// Java fails for an empty list, so the layout stops.
func splitCoordinate(splitPartner HyperEdgeSegment) float64 {
	incoming := splitPartner.IncomingConnectionCoordinates()
	if len(incoming) == 0 {
		lgraph.Bail("BaseRoutingDirectionStrategy: a split partner has no incoming connection")
	}
	return incoming[0]
}
