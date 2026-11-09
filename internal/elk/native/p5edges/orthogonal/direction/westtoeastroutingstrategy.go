// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/orthogonal/direction/WestToEastRoutingStrategy.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package direction

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// WestToEastRoutingStrategy is the class WestToEastRoutingStrategy: the
// routing strategy for layers that go from west to east.
type WestToEastRoutingStrategy struct {
	baseStrategy
}

// NewWestToEastRoutingStrategy is new WestToEastRoutingStrategy().
func NewWestToEastRoutingStrategy() *WestToEastRoutingStrategy {
	return &WestToEastRoutingStrategy{newBaseStrategy()}
}

// PortPositionOnHyperNode is getPortPositionOnHyperNode(port).
func (s *WestToEastRoutingStrategy) PortPositionOnHyperNode(port *lgraph.LPort) float64 {
	return port.Node().Pos.Y + port.Pos.Y + port.Anchor.Y
}

// SourcePortSide is getSourcePortSide().
func (s *WestToEastRoutingStrategy) SourcePortSide() core.PortSide { return core.PortSideEast }

// TargetPortSide is getTargetPortSide().
func (s *WestToEastRoutingStrategy) TargetPortSide() core.PortSide { return core.PortSideWest }

// CalculateBendPoints is calculateBendPoints(segment, startPos, edgeSpacing).
func (s *WestToEastRoutingStrategy) CalculateBendPoints(segment HyperEdgeSegment, startPos, edgeSpacing float64) {
	// We don't do anything with dummy segments; they are dealt with when
	// their partner is processed
	if segment.IsDummy() {
		return
	}

	// Calculate coordinates for each port's bend points
	segmentX := startPos + float64(float64(segment.RoutingSlot())*edgeSpacing)

	for _, port := range segment.Ports() {
		sourceY := port.AbsoluteAnchor().Y

		for _, edge := range port.OutgoingEdges {
			if !edge.IsSelfLoop() {
				target := edgeTarget(edge)
				targetY := target.AbsoluteAnchor().Y

				if math.Abs(sourceY-targetY) > Tolerance {
					// We'll update these if we find that the segment was split
					currentX := segmentX
					currentSegment := segment

					bend := emath.NewKVector(currentX, sourceY)
					edge.BendPoints.Add(bend)
					s.addJunctionPointIfNecessary(edge, currentSegment, bend, true)

					// If this segment was split, we need two additional bend
					// points
					splitPartner := segment.SplitPartnerSegment()
					if splitPartner != nil {
						splitY := splitCoordinate(splitPartner)

						bend = emath.NewKVector(currentX, splitY)
						edge.BendPoints.Add(bend)
						s.addJunctionPointIfNecessary(edge, currentSegment, bend, true)

						// Advance to the split partner's routing slot
						currentX = startPos + float64(float64(splitPartner.RoutingSlot())*edgeSpacing)
						currentSegment = splitPartner

						bend = emath.NewKVector(currentX, splitY)
						edge.BendPoints.Add(bend)
						s.addJunctionPointIfNecessary(edge, currentSegment, bend, true)
					}

					bend = emath.NewKVector(currentX, targetY)
					edge.BendPoints.Add(bend)
					s.addJunctionPointIfNecessary(edge, currentSegment, bend, true)
				}
			}
		}
	}
}
