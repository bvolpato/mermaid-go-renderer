// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/orthogonal/direction/SouthToNorthRoutingStrategy.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package direction

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// SouthToNorthRoutingStrategy is the class SouthToNorthRoutingStrategy: the
// routing strategy for layers that go from south to north.
type SouthToNorthRoutingStrategy struct {
	baseStrategy
}

// NewSouthToNorthRoutingStrategy is new SouthToNorthRoutingStrategy().
func NewSouthToNorthRoutingStrategy() *SouthToNorthRoutingStrategy {
	return &SouthToNorthRoutingStrategy{newBaseStrategy()}
}

// PortPositionOnHyperNode is getPortPositionOnHyperNode(port).
func (s *SouthToNorthRoutingStrategy) PortPositionOnHyperNode(port *lgraph.LPort) float64 {
	return port.Node().Pos.X + port.Pos.X + port.Anchor.X
}

// SourcePortSide is getSourcePortSide().
func (s *SouthToNorthRoutingStrategy) SourcePortSide() core.PortSide { return core.PortSideNorth }

// TargetPortSide is getTargetPortSide().
func (s *SouthToNorthRoutingStrategy) TargetPortSide() core.PortSide { return core.PortSideSouth }

// CalculateBendPoints is calculateBendPoints(segment, startPos, edgeSpacing).
func (s *SouthToNorthRoutingStrategy) CalculateBendPoints(segment HyperEdgeSegment, startPos, edgeSpacing float64) {
	// We don't do anything with dummy segments; they are dealt with when
	// their partner is processed
	if segment.IsDummy() {
		return
	}

	// Calculate coordinates for each port's bend points
	segmentY := startPos - float64(float64(segment.RoutingSlot())*edgeSpacing)

	for _, port := range segment.Ports() {
		sourceX := port.AbsoluteAnchor().X

		for _, edge := range port.OutgoingEdges {
			if !edge.IsSelfLoop() {
				target := edgeTarget(edge)
				targetX := target.AbsoluteAnchor().X

				if math.Abs(sourceX-targetX) > Tolerance {
					// We'll update these if we find that the segment was split
					currentY := segmentY
					currentSegment := segment

					bend := emath.NewKVector(sourceX, currentY)
					edge.BendPoints.Add(bend)
					s.addJunctionPointIfNecessary(edge, currentSegment, bend, false)

					// If this segment was split, we need two additional bend
					// points
					splitPartner := segment.SplitPartnerSegment()
					if splitPartner != nil {
						splitX := splitCoordinate(splitPartner)

						bend = emath.NewKVector(splitX, currentY)
						edge.BendPoints.Add(bend)
						s.addJunctionPointIfNecessary(edge, currentSegment, bend, false)

						// Advance to the split partner's routing slot
						currentY = startPos - float64(float64(splitPartner.RoutingSlot())*edgeSpacing)
						currentSegment = splitPartner

						bend = emath.NewKVector(splitX, currentY)
						edge.BendPoints.Add(bend)
						s.addJunctionPointIfNecessary(edge, currentSegment, bend, false)
					}

					bend = emath.NewKVector(targetX, currentY)
					edge.BendPoints.Add(bend)
					s.addJunctionPointIfNecessary(edge, currentSegment, bend, false)
				}
			}
		}
	}
}
