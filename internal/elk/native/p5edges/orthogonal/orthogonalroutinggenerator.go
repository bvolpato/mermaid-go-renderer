// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/orthogonal/OrthogonalRoutingGenerator.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package orthogonal has the routing generator of the orthogonal edge
// router: it makes hyper edge segments for the edges between two layers,
// orders them into routing slots and sets the bend points.
package orthogonal

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p5edges/orthogonal/direction"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// Tolerance is OrthogonalRoutingGenerator.TOLERANCE: a difference below it is
// zero.
const Tolerance = direction.Tolerance

const (
	// criticalConflictsDetected is the result of countConflicts for a
	// critical conflict.
	criticalConflictsDetected = -1

	// conflictThresholdFactor is the factor of the edge spacing that gives
	// the conflict threshold.
	conflictThresholdFactor = 0.5
	// criticalConflictThresholdFactor is the factor that gives the critical
	// conflict threshold.
	criticalConflictThresholdFactor = 0.2

	// conflictPenalty is the weight penalty for a conflict that is not
	// critical.
	conflictPenalty = 1
	// crossingPenalty is the weight penalty for a crossing.
	crossingPenalty = 16
)

// OrthogonalRoutingGenerator is the class OrthogonalRoutingGenerator: it
// routes the edges between two lists of nodes with orthogonal bend points,
// for one routing direction.
type OrthogonalRoutingGenerator struct {
	// segmentSplitter splits hyper edge segments. It is made when it is
	// needed.
	segmentSplitter *HyperEdgeSegmentSplitter

	// routingStrategy is the routing direction strategy.
	routingStrategy direction.BaseRoutingDirectionStrategy
	// edgeSpacing is the spacing between edges.
	edgeSpacing float64

	// conflictThreshold is the distance below which two horizontal line
	// segments are too close: a fixed part of the edge spacing.
	conflictThreshold float64
	// criticalConflictThreshold is the distance below which two horizontal
	// line segments overlap. It is a part of the smallest distance between
	// two connections of one side, so each pair of layers has its own.
	criticalConflictThreshold float64

	// debugPrefix is the prefix of the debug output files. elkjs has no debug
	// output, so nothing reads it.
	debugPrefix string
}

// NewOrthogonalRoutingGenerator is new OrthogonalRoutingGenerator(direction,
// edgeSpacing, debugPrefix).
func NewOrthogonalRoutingGenerator(dir direction.RoutingDirection, edgeSpacing float64, debugPrefix string) *OrthogonalRoutingGenerator {
	return &OrthogonalRoutingGenerator{
		routingStrategy:   direction.ForRoutingDirection(dir),
		edgeSpacing:       edgeSpacing,
		conflictThreshold: float64(conflictThresholdFactor * edgeSpacing),
		debugPrefix:       debugPrefix,
	}
}

////////////////////////////////////////////////////////////////////////////
// Edge Routing

// RouteEdges is routeEdges(monitor, layeredGraph, sourceLayerNodes,
// sourceLayerIndex, targetLayerNodes, startPos): it routes the edges between
// the two layers and returns the number of routing slots. A layer can be
// nil. startPos is the position of the first routing slot.
//
// The Java method has a progress monitor as its first parameter. It uses the
// monitor and the index of the source layer only for the debug output, which
// elkjs does not have.
func (g *OrthogonalRoutingGenerator) RouteEdges(layeredGraph *lgraph.LGraph, sourceLayerNodes []*lgraph.LNode, sourceLayerIndex int, targetLayerNodes []*lgraph.LNode, startPos float64) int {
	// Keep track of our hyperedge segments, and which ports they were created
	// for
	portToEdgeSegmentMap := gwt.NewMap[*lgraph.LPort, *HyperEdgeSegment]()
	var edgeSegments []*HyperEdgeSegment

	// create hyperedge segments for eastern output ports of the left layer
	// and for western output ports of the right layer
	edgeSegments = g.createHyperEdgeSegments(sourceLayerNodes, g.routingStrategy.SourcePortSide(), edgeSegments, portToEdgeSegmentMap)
	edgeSegments = g.createHyperEdgeSegments(targetLayerNodes, g.routingStrategy.TargetPortSide(), edgeSegments, portToEdgeSegmentMap)

	// Our critical conflict threshold is a fraction of the minimum distance
	// between two horizontal hyperedge segments
	g.criticalConflictThreshold = float64(criticalConflictThresholdFactor * g.minimumHorizontalSegmentDistance(edgeSegments))

	// create dependencies for the hyperedge segment ordering graph and note
	// how many critical dependencies have been created
	criticalDependencyCount := 0
	for firstIdx := 0; firstIdx < len(edgeSegments)-1; firstIdx++ {
		firstSegment := edgeSegments[firstIdx]
		for secondIdx := firstIdx + 1; secondIdx < len(edgeSegments); secondIdx++ {
			criticalDependencyCount += g.createDependencyIfNecessary(firstSegment, edgeSegments[secondIdx])
		}
	}

	// if there are at least two critical dependencies, there may be critical
	// cycles that need to be broken
	random := prop.Get(layeredGraph, iprops.Random)
	if criticalDependencyCount >= 2 {
		edgeSegments = g.breakCriticalCycles(edgeSegments, random)
	}

	// break non-critical cycles
	BreakNonCriticalCycles(edgeSegments, random)

	// assign ranks to the edge segments
	topologicalNumbering(edgeSegments)

	// set bend points with appropriate coordinates
	rankCount := -1
	for _, node := range edgeSegments {
		// edges that are just straight lines don't take up a slot and don't
		// need bend points
		if math.Abs(node.StartCoordinate()-node.EndCoordinate()) < Tolerance {
			continue
		}

		rankCount = max(rankCount, node.RoutingSlot())

		g.routingStrategy.CalculateBendPoints(node, startPos, g.edgeSpacing)
	}

	// release the created resources
	g.routingStrategy.ClearCreatedJunctionPoints()
	return rankCount + 1
}

// minimumHorizontalSegmentDistance is
// minimumHorizontalSegmentDistance(edgeSegments): the smallest distance
// between two adjacent source connections or two adjacent target
// connections.
func (g *OrthogonalRoutingGenerator) minimumHorizontalSegmentDistance(edgeSegments []*HyperEdgeSegment) float64 {
	var incoming []float64
	for _, segment := range edgeSegments {
		incoming = append(incoming, segment.incomingConnectionCoordinates...)
	}
	minIncomingDistance := g.minimumDifference(incoming)

	var outgoing []float64
	for _, segment := range edgeSegments {
		outgoing = append(outgoing, segment.outgoingConnectionCoordinates...)
	}
	minOutgoingDistance := g.minimumDifference(outgoing)

	// Math.min is the builtin min of Go: math.Min gives an infinity where
	// JavaScript gives NaN for a NaN and an infinity.
	return min(minIncomingDistance, minOutgoingDistance)
}

// minimumDifference is minimumDifference(numberStream): the smallest
// difference between two of the numbers, or Double.MAX_VALUE for fewer than
// two numbers. It sorts the given slice.
func (g *OrthogonalRoutingGenerator) minimumDifference(numbers []float64) float64 {
	// The Java stream is sorted().distinct(): a stable merge sort with
	// Double.compareTo, then a filter that keeps the first of the numbers
	// that are equal. Equal numbers are next to each other after the sort.
	gwt.SortFunc(numbers, gwt.DoubleCompare)
	distinct := numbers[:0]
	for i, number := range numbers {
		if i > 0 && number == distinct[len(distinct)-1] {
			continue
		}
		distinct = append(distinct, number)
	}
	numbers = distinct

	minDifference := math.MaxFloat64

	if len(numbers) >= 2 {
		currentNumber := numbers[0]

		for i := 1; i < len(numbers); i++ {
			previousNumber := currentNumber
			currentNumber = numbers[i]

			// This relies on the fact that the numbers are distinct and sorted
			// ascendingly
			minDifference = min(minDifference, currentNumber-previousNumber)
		}
	}

	return minDifference
}

////////////////////////////////////////////////////////////////////////////
// Hyper Edge Graph Creation

// createHyperEdgeSegments is createHyperEdgeSegments(nodes, portSide,
// hyperEdges, portToHyperEdgeSegmentMap): it makes the hyper edge segments
// of a layer, for the output ports of the given side. It returns the list of
// segments, which Java changes in place.
func (g *OrthogonalRoutingGenerator) createHyperEdgeSegments(nodes []*lgraph.LNode, portSide core.PortSide, hyperEdges []*HyperEdgeSegment, portToHyperEdgeSegmentMap *gwt.Map[*lgraph.LPort, *HyperEdgeSegment]) []*HyperEdgeSegment {
	for _, node := range nodes {
		for _, port := range node.PortsByTypeAndSide(lenum.PortTypeOutput, portSide) {
			hyperEdge := portToHyperEdgeSegmentMap.Get(port)
			if hyperEdge == nil {
				hyperEdge = NewHyperEdgeSegment(g.routingStrategy)
				hyperEdges = append(hyperEdges, hyperEdge)
				hyperEdge.AddPortPositions(port, portToHyperEdgeSegmentMap)
			}
		}
	}
	return hyperEdges
}

// createDependencyIfNecessary is createDependencyIfNecessary(he1, he2): it
// makes the dependencies between the two segments, if they need one. It
// returns the number of critical dependencies that it made.
func (g *OrthogonalRoutingGenerator) createDependencyIfNecessary(he1, he2 *HyperEdgeSegment) int {
	// check if at least one of the two nodes is just a straight line; those
	// don't create dependencies since they don't take up a slot
	if math.Abs(he1.StartCoordinate()-he1.EndCoordinate()) < Tolerance ||
		math.Abs(he2.StartCoordinate()-he2.EndCoordinate()) < Tolerance {

		return 0
	}

	// compare number of conflicts for both variants
	conflicts1 := g.countConflicts(he1.outgoingConnectionCoordinates, he2.incomingConnectionCoordinates)
	conflicts2 := g.countConflicts(he2.outgoingConnectionCoordinates, he1.incomingConnectionCoordinates)

	criticalConflictsFound := conflicts1 == criticalConflictsDetected || conflicts2 == criticalConflictsDetected
	criticalDependencyCount := 0

	if criticalConflictsFound {
		// Check which critical dependencies have to be added
		if conflicts1 == criticalConflictsDetected {
			// hyperedge 1 MUST NOT be left of hyperedge 2, since that would
			// cause critical conflicts
			CreateAndAddCritical(he2, he1)
			criticalDependencyCount++
		}

		if conflicts2 == criticalConflictsDetected {
			// hyperedge 2 MUST NOT be left of hyperedge 1, since that would
			// cause critical conflicts
			CreateAndAddCritical(he1, he2)
			criticalDependencyCount++
		}

	} else {
		// we did not detect critical conflicts, so count the number of
		// crossings for both variants
		crossings1 := countCrossings(he1.outgoingConnectionCoordinates, he2.StartCoordinate(), he2.EndCoordinate())
		crossings1 += countCrossings(he2.incomingConnectionCoordinates, he1.StartCoordinate(), he1.EndCoordinate())
		crossings2 := countCrossings(he2.outgoingConnectionCoordinates, he1.StartCoordinate(), he1.EndCoordinate())
		crossings2 += countCrossings(he1.incomingConnectionCoordinates, he2.StartCoordinate(), he2.EndCoordinate())

		// compute the penalty; crossings are deemed worse than (non-critical)
		// conflicts
		depValue1 := conflictPenalty*conflicts1 + crossingPenalty*crossings1
		depValue2 := conflictPenalty*conflicts2 + crossingPenalty*crossings2

		if depValue1 < depValue2 {
			// hyperedge 1 wants to be left of hyperedge 2
			CreateAndAddRegular(he1, he2, depValue2-depValue1)
		} else if depValue1 > depValue2 {
			// hyperedge 2 wants to be left of hyperedge 1
			CreateAndAddRegular(he2, he1, depValue1-depValue2)
		} else if depValue1 > 0 && depValue2 > 0 {
			// create two dependencies with zero weight
			CreateAndAddRegular(he1, he2, 0)
			CreateAndAddRegular(he2, he1, 0)
		}
	}

	return criticalDependencyCount
}

// countConflicts is countConflicts(posis1, posis2): the number of positions
// of the two sorted lists that are too close, or criticalConflictsDetected
// for a critical conflict.
func (g *OrthogonalRoutingGenerator) countConflicts(posis1, posis2 []float64) int {
	conflicts := 0

	if len(posis1) != 0 && len(posis2) != 0 {
		iter1 := 0
		iter2 := 0
		pos1 := posis1[iter1]
		iter1++
		pos2 := posis2[iter2]
		iter2++
		hasMore := true

		for hasMore {
			if pos1 > pos2-g.criticalConflictThreshold && pos1 < pos2+g.criticalConflictThreshold {
				// We're done as soon as we find a single critical conflict
				return criticalConflictsDetected
			} else if pos1 > pos2-g.conflictThreshold && pos1 < pos2+g.conflictThreshold {
				conflicts++
			}

			if pos1 <= pos2 && iter1 < len(posis1) {
				pos1 = posis1[iter1]
				iter1++
			} else if pos2 <= pos1 && iter2 < len(posis2) {
				pos2 = posis2[iter2]
				iter2++
			} else {
				hasMore = false
			}
		}
	}

	return conflicts
}

// countCrossings is countCrossings(posis, start, end): the number of
// positions of the sorted list that are in the critical area.
func countCrossings(posis []float64, start, end float64) int {
	crossings := 0
	for _, pos := range posis {
		if pos > end {
			break
		} else if pos >= start {
			crossings++
		}
	}
	return crossings
}

////////////////////////////////////////////////////////////////////////////
// Cycle Breaking

// breakCriticalCycles is breakCriticalCycles(edgeSegments, random): it finds
// the critical cycles and breaks each one by the split of an edge segment.
// It returns the list of segments with the new segments.
func (g *OrthogonalRoutingGenerator) breakCriticalCycles(edgeSegments []*HyperEdgeSegment, random *gwt.Random) []*HyperEdgeSegment {
	cycleDependencies := DetectCycles(edgeSegments, true, random)

	// Lazy initialisation
	if g.segmentSplitter == nil {
		g.segmentSplitter = NewHyperEdgeSegmentSplitter(g)
	}

	return g.segmentSplitter.SplitSegments(cycleDependencies, edgeSegments, g.criticalConflictThreshold)
}

// BreakNonCriticalCycles is
// OrthogonalRoutingGenerator.breakNonCriticalCycles(edgeSegments, random): it
// finds the cycles that are not critical and breaks each one: it removes a
// dependency of weight zero and reverses each other one. The self loop
// router uses it too.
func BreakNonCriticalCycles(edgeSegments []*HyperEdgeSegment, random *gwt.Random) {
	cycleDependencies := DetectCycles(edgeSegments, false, random)

	for _, cycleDependency := range cycleDependencies {
		if cycleDependency.Weight() == 0 {
			// Simply remove this dependency. This assumes that only two-cycles
			// will have dependency weight 0
			cycleDependency.Remove()
		} else {
			cycleDependency.Reverse()
		}
	}
}

////////////////////////////////////////////////////////////////////////////
// Topological Ordering

// topologicalNumbering is topologicalNumbering(segments): it gives each
// segment a routing slot by a topological numbering.
func topologicalNumbering(segments []*HyperEdgeSegment) {
	// determine sources, targets, incoming count and outgoing count; targets
	// are only added to the list if they only connect westward ports (that
	// is, if all their horizontal segments point to the right)
	var sources []*HyperEdgeSegment
	var rightwardTargets []*HyperEdgeSegment
	for _, node := range segments {
		node.SetInWeight(len(node.incomingSegmentDependencies))
		node.SetOutWeight(len(node.outgoingSegmentDependencies))

		if node.InWeight() == 0 {
			sources = append(sources, node)
		}

		if node.OutWeight() == 0 && len(node.incomingConnectionCoordinates) == 0 {
			rightwardTargets = append(rightwardTargets, node)
		}
	}

	maxRank := -1

	// assign ranks using topological numbering
	for len(sources) != 0 {
		node := sources[0]
		sources = sources[1:]
		for _, dep := range node.outgoingSegmentDependencies {
			target := dep.Target()
			target.SetRoutingSlot(max(target.RoutingSlot(), node.RoutingSlot()+1))
			maxRank = max(maxRank, target.RoutingSlot())

			target.SetInWeight(target.InWeight() - 1)
			if target.InWeight() == 0 {
				sources = append(sources, target)
			}
		}
	}

	// If we stopped here, hyperedge segments that don't have any horizontal
	// segments pointing leftward would be ranked just like every other
	// hyperedge segments. This would move back edges too far away from their
	// target node. To remedy that, we move all hyperedge segments with
	// horizontal segments only pointing rightwards as far right as possible.
	if maxRank > -1 {
		// assign all target nodes with horizontal segments pointing to the
		// right the rightmost rank
		for _, node := range rightwardTargets {
			node.SetRoutingSlot(maxRank)
		}

		// let all other segments with horizontal segments pointing rightwards
		// move as far right as possible
		for len(rightwardTargets) != 0 {
			node := rightwardTargets[0]
			rightwardTargets = rightwardTargets[1:]

			// The node only has connections to western ports
			for _, dep := range node.incomingSegmentDependencies {
				source := dep.Source()
				if len(source.incomingConnectionCoordinates) > 0 {
					continue
				}

				source.SetRoutingSlot(min(source.RoutingSlot(), node.RoutingSlot()-1))

				source.SetOutWeight(source.OutWeight() - 1)
				if source.OutWeight() == 0 {
					rightwardTargets = append(rightwardTargets, source)
				}
			}
		}
	}
}
