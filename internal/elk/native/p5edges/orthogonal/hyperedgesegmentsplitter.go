// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/orthogonal/HyperEdgeSegmentSplitter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package orthogonal

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
)

// HyperEdgeSegmentSplitter is the class HyperEdgeSegmentSplitter: it splits
// hyper edge segments to break the cycles of critical dependencies. The
// comments use the terms of a layout from left to right.
type HyperEdgeSegmentSplitter struct {
	// routingGenerator is the routing generator that made the splitter.
	routingGenerator *OrthogonalRoutingGenerator
}

// NewHyperEdgeSegmentSplitter is new HyperEdgeSegmentSplitter(routingGenerator).
func NewHyperEdgeSegmentSplitter(routingGenerator *OrthogonalRoutingGenerator) *HyperEdgeSegmentSplitter {
	return &HyperEdgeSegmentSplitter{routingGenerator: routingGenerator}
}

// SplitSegments is splitSegments(dependenciesToResolve, segments,
// criticalConflictThreshold): for each dependency it splits one of the two
// segments. It returns the list of segments with the new segments at the
// end, where Java adds them to the given list.
func (s *HyperEdgeSegmentSplitter) SplitSegments(dependenciesToResolve []*HyperEdgeSegmentDependency, segments []*HyperEdgeSegment, criticalConflictThreshold float64) []*HyperEdgeSegment {
	// Only start preparations if there's going to be things to do
	if len(dependenciesToResolve) == 0 {
		return segments
	}

	// Collect all relevant spaces between horizontal segments that are large
	// enough to house another horizontal segment without causing additional
	// conflicts
	freeAreas := s.findFreeAreas(segments, criticalConflictThreshold)

	// For each dependency, choose which segment to split
	segmentsToSplit := s.decideWhichSegmentsToSplit(dependenciesToResolve)

	// Split the segments in order from smallest to largest. The smallest ones
	// need to be split first since they have fewer options for where to put
	// their horizontal connecting segments. The Java stream sorts a copy of
	// the set before it splits the first segment.
	sorted := segmentsToSplit.Values()
	gwt.SortFunc(sorted, func(hes1, hes2 *HyperEdgeSegment) int {
		return gwt.DoubleCompare(hes1.Length(), hes2.Length())
	})
	for _, segment := range sorted {
		segments, freeAreas = s.split(segment, segments, freeAreas, criticalConflictThreshold)
	}
	return segments
}

////////////////////////////////////////////////////////////////////////////
// Finding Space

// findFreeAreas is findFreeAreas(segments, criticalConflictThreshold): the
// areas between horizontal segments that are large enough for one more
// horizontal segment.
func (s *HyperEdgeSegmentSplitter) findFreeAreas(segments []*HyperEdgeSegment, criticalConflictThreshold float64) []*freeArea {
	freeAreas := []*freeArea{}

	// Retrieve all positions where hyperedge segments connect to ports, and
	// sort them
	var sortedCoordinates []float64
	for _, segment := range segments {
		sortedCoordinates = append(sortedCoordinates, segment.incomingConnectionCoordinates...)
	}
	for _, segment := range segments {
		sortedCoordinates = append(sortedCoordinates, segment.outgoingConnectionCoordinates...)
	}
	gwt.SortFloats(sortedCoordinates)

	// Go through each pair of coordinates and create free areas for those
	// that are at least twice the critical threshold
	for i := 1; i < len(sortedCoordinates); i++ {
		if sortedCoordinates[i]-sortedCoordinates[i-1] >= 2*criticalConflictThreshold {
			freeAreas = append(freeAreas, newFreeArea(
				sortedCoordinates[i-1]+criticalConflictThreshold,
				sortedCoordinates[i]-criticalConflictThreshold))
		}
	}

	return freeAreas
}

////////////////////////////////////////////////////////////////////////////
// Split Segment Decisions

// decideWhichSegmentsToSplit is decideWhichSegmentsToSplit(dependencies): the
// segments whose split breaks the critical cycles of the dependencies.
//
// The Java set is a LinkedHashSet, which finds a segment by its hashCode and
// its equals. Both are the mark of the segment, so the set is a map from the
// mark to the segment, in insertion order.
func (s *HyperEdgeSegmentSplitter) decideWhichSegmentsToSplit(dependencies []*HyperEdgeSegmentDependency) *gwt.Map[int, *HyperEdgeSegment] {
	segmentsToSplit := gwt.NewMap[int, *HyperEdgeSegment]()

	for _, dependency := range dependencies {
		sourceSegment := dependency.Source()
		targetSegment := dependency.Target()

		// If either of the involved segments were already selected for
		// splitting because of another dependency, that's sufficient
		if segmentsToSplit.ContainsKey(sourceSegment.mark) || segmentsToSplit.ContainsKey(targetSegment.mark) {
			continue
		}

		// One segment will be split, and the other one will be remembered to
		// have caused the split. The latter will have to stay between the two
		// segments the former will be split into, or else we'll have overlaps
		// again
		segmentToSplit := sourceSegment
		segmentCausingSplit := targetSegment

		// This is perfectly fine unless the segment to split represents a
		// hyperedge and the other does not. We prefer splitting regular edges
		// since hyperedges have a higher chance of causing additional
		// crossings.
		if sourceSegment.RepresentsHyperedge() && !targetSegment.RepresentsHyperedge() {
			segmentToSplit = targetSegment
			segmentCausingSplit = sourceSegment
		}

		segmentsToSplit.Put(segmentToSplit.mark, segmentToSplit)
		segmentToSplit.SetSplitBy(segmentCausingSplit)
	}

	return segmentsToSplit
}

////////////////////////////////////////////////////////////////////////////
// Actual Splitting

// split is split(segment, segments, freeAreas, criticalConflictThreshold): it
// splits the segment at the best position. It returns the two lists, which
// Java changes in place.
func (s *HyperEdgeSegmentSplitter) split(segment *HyperEdgeSegment, segments []*HyperEdgeSegment, freeAreas []*freeArea, criticalConflictThreshold float64) ([]*HyperEdgeSegment, []*freeArea) {
	// Split the segment at the best position and add the new segment to our
	// list
	splitPosition, freeAreas := s.computePositionToSplitAndUpdateFreeAreas(segment, freeAreas, criticalConflictThreshold)
	segments = append(segments, segment.SplitAt(splitPosition))

	// Update the dependencies to reflect the new situation
	s.updateDependencies(segment, segments)
	return segments, freeAreas
}

// updateDependencies is updateDependencies(segment, segments): it adds the
// dependencies of the segment and of its split partner. The segment was
// split, and the two have no dependencies.
func (s *HyperEdgeSegmentSplitter) updateDependencies(segment *HyperEdgeSegment, segments []*HyperEdgeSegment) {
	splitCausingSegment := segment.SplitBy()
	splitPartner := segment.SplitPartner()

	// The segment currently has no dependencies at all. We first need for the
	// segments to be ordered like this:
	//    segment ---> split-causing segment ---> split partner
	CreateAndAddCritical(segment, splitCausingSegment)
	CreateAndAddCritical(splitCausingSegment, splitPartner)

	// Now we just need to re-introduce dependencies to other segments
	for _, otherSegment := range segments {
		// We already have our dependencies between our three segments involved
		// in the conflict settled
		if otherSegment != splitCausingSegment && otherSegment != segment && otherSegment != splitPartner {
			s.routingGenerator.createDependencyIfNecessary(otherSegment, segment)
			s.routingGenerator.createDependencyIfNecessary(otherSegment, splitPartner)
		}
	}
}

////////////////////////////////////////////////////////////////////////////
// Split Position Computation

// computePositionToSplitAndUpdateFreeAreas is
// computePositionToSplitAndUpdateFreeAreas(segment, freeAreas,
// criticalConflictThreshold): a position to split the segment at. It returns
// the list of free areas, where the used area is gone or split into two.
func (s *HyperEdgeSegmentSplitter) computePositionToSplitAndUpdateFreeAreas(segment *HyperEdgeSegment, freeAreas []*freeArea, criticalConflictThreshold float64) (float64, []*freeArea) {
	// We shall iterate over the available areas (which are sorted by
	// ascending positions) and find the index of the first and the last area
	// we could use since they are in our segment's reach
	firstPossibleAreaIndex := -1
	lastPossibleAreaIndex := -1

	for i := 0; i < len(freeAreas); i++ {
		currArea := freeAreas[i]

		if currArea.startPosition > segment.EndCoordinate() {
			// We're past the possible areas, so stop
			break
		} else if currArea.endPosition >= segment.StartCoordinate() {
			// We've found a possible area; it might be the first
			if firstPossibleAreaIndex < 0 {
				firstPossibleAreaIndex = i
			}

			lastPossibleAreaIndex = i
		}
	}

	// Determine the position to split the segment at
	splitPosition := centerOfSegment(segment)

	if firstPossibleAreaIndex >= 0 {
		// There are areas we can use
		bestAreaIndex := s.chooseBestAreaIndex(segment, freeAreas, firstPossibleAreaIndex, lastPossibleAreaIndex)

		// We'll use the best area's centre and update the area list
		splitPosition = centerOfArea(freeAreas[bestAreaIndex])
		freeAreas = s.useArea(freeAreas, bestAreaIndex, criticalConflictThreshold)
	}

	return splitPosition, freeAreas
}

// chooseBestAreaIndex is chooseBestAreaIndex(segment, freeAreas, fromIndex,
// toIndex): the best area for the split of the segment between the two
// indices, which are both inclusive.
func (s *HyperEdgeSegmentSplitter) chooseBestAreaIndex(segment *HyperEdgeSegment, freeAreas []*freeArea, fromIndex, toIndex int) int {
	bestAreaIndex := fromIndex

	if fromIndex < toIndex {
		// We have more areas to choose from, so rate them and find the best
		// one. We need to simulate splitting the segment so that we can count
		// potential crossings
		splitSegment, splitPartner := segment.SimulateSplit()

		bestArea := freeAreas[bestAreaIndex]
		bestRating := s.rateArea(segment, splitSegment, splitPartner, bestArea)

		for i := fromIndex + 1; i <= toIndex; i++ {
			// Determine how good our current area is
			currArea := freeAreas[i]
			currRating := s.rateArea(segment, splitSegment, splitPartner, currArea)

			if s.isBetter(currArea, currRating, bestArea, bestRating) {
				bestArea = currArea
				bestRating = currRating
				bestAreaIndex = i
			}
		}
	}

	return bestAreaIndex
}

// rateArea is rateArea(segment, splitSegment, splitPartner, area): what
// happens when the two split segments are connected through the area.
func (s *HyperEdgeSegmentSplitter) rateArea(segment, splitSegment, splitPartner *HyperEdgeSegment, area *freeArea) *areaRating {
	// The area's centre would be used to link the two split segments, so we
	// need to add that to their incident connections
	areaCentre := centerOfArea(area)

	splitSegment.outgoingConnectionCoordinates = append(splitSegment.outgoingConnectionCoordinates[:0], areaCentre)

	splitPartner.incomingConnectionCoordinates = append(splitPartner.incomingConnectionCoordinates[:0], areaCentre)

	// We need to count the dependencies and crossings that the split partners
	// would cause with the original segments incident dependencies
	rating := &areaRating{0, 0}

	for _, dependency := range segment.incomingSegmentDependencies {
		otherSegment := dependency.Source()

		s.updateConsideringBothOrderings(rating, splitSegment, otherSegment)
		s.updateConsideringBothOrderings(rating, splitPartner, otherSegment)
	}

	for _, dependency := range segment.outgoingSegmentDependencies {
		otherSegment := dependency.Target()

		s.updateConsideringBothOrderings(rating, splitSegment, otherSegment)
		s.updateConsideringBothOrderings(rating, splitPartner, otherSegment)
	}

	// There will be two additional dependencies: splitSegment -->
	// splitBySegment --> splitPartner. The order between the three will not
	// change, so we only have to count crossings for this concrete order
	rating.dependencies += 2

	rating.crossings += s.countCrossingsForSingleOrdering(splitSegment, segment.SplitBy())
	rating.crossings += s.countCrossingsForSingleOrdering(segment.SplitBy(), splitPartner)

	return rating
}

// updateConsideringBothOrderings is updateConsideringBothOrderings(rating,
// s1, s2): it looks at both orders of the two segments and adds the smallest
// number of dependencies and crossings to expect.
func (s *HyperEdgeSegmentSplitter) updateConsideringBothOrderings(rating *areaRating, s1, s2 *HyperEdgeSegment) {
	crossingsS1LeftOfS2 := s.countCrossingsForSingleOrdering(s1, s2)
	crossingsS2LeftOfS1 := s.countCrossingsForSingleOrdering(s2, s1)

	if crossingsS1LeftOfS2 == crossingsS2LeftOfS1 {
		// If the crossings are the same, we're only interested if there are
		// more than 0
		if crossingsS1LeftOfS2 > 0 {
			// Both orderings generate the same number of crossings (more than
			// 0), so we have a two-cycle
			rating.dependencies += 2
			rating.crossings += crossingsS1LeftOfS2
		}
	} else {
		// One order is better than the other, so there will be a single
		// dependency
		rating.dependencies += 1
		rating.crossings += min(crossingsS1LeftOfS2, crossingsS2LeftOfS1)
	}
}

// countCrossingsForSingleOrdering is countCrossingsForSingleOrdering(left,
// right): the crossings between a left and a right segment.
func (s *HyperEdgeSegmentSplitter) countCrossingsForSingleOrdering(left, right *HyperEdgeSegment) int {
	return countCrossings(left.outgoingConnectionCoordinates, right.StartCoordinate(), right.EndCoordinate()) +
		countCrossings(right.incomingConnectionCoordinates, left.StartCoordinate(), left.EndCoordinate())
}

// isBetter is isBetter(currArea, currRating, bestArea, bestRating).
func (s *HyperEdgeSegmentSplitter) isBetter(currArea *freeArea, currRating *areaRating, bestArea *freeArea, bestRating *areaRating) bool {
	if currRating.crossings < bestRating.crossings {
		// First criterion: number of crossings
		return true
	} else if currRating.crossings == bestRating.crossings {
		if currRating.dependencies < bestRating.dependencies {
			// Second criterion: number of dependencies
			return true
		} else if currRating.dependencies == bestRating.dependencies {
			if currArea.size > bestArea.size {
				// Third criterion: size
				return true
			}
		}
	}

	return false
}

// useArea is useArea(freeAreas, usedAreaIndex, criticalConflictThreshold):
// an area that is used falls into two parts, which can be free areas
// themselves. It returns the list, which Java changes in place.
func (s *HyperEdgeSegmentSplitter) useArea(freeAreas []*freeArea, usedAreaIndex int, criticalConflictThreshold float64) []*freeArea {
	oldArea := freeAreas[usedAreaIndex]
	freeAreas = append(freeAreas[:usedAreaIndex], freeAreas[usedAreaIndex+1:]...)

	if oldArea.size/2 >= criticalConflictThreshold {
		// We will probably insert new areas. Keep track of where to insert
		// them
		insertIndex := usedAreaIndex

		// This area is large enough to split and still have enough space to
		// the position we're now using for our latest segment
		oldAreaCentre := centerOfArea(oldArea)

		// Create the two new areas (and be doubly sure that double precision
		// does not bite us)
		newEnd1 := oldAreaCentre - criticalConflictThreshold
		if oldArea.startPosition <= oldAreaCentre-criticalConflictThreshold {
			newArea1 := newFreeArea(oldArea.startPosition, newEnd1)
			freeAreas = insertFreeArea(freeAreas, insertIndex, newArea1)
			insertIndex++
		}

		newStart2 := oldAreaCentre + criticalConflictThreshold
		if newStart2 <= oldArea.endPosition {
			newArea2 := newFreeArea(newStart2, oldArea.endPosition)
			freeAreas = insertFreeArea(freeAreas, insertIndex, newArea2)
		}
	}
	return freeAreas
}

func insertFreeArea(freeAreas []*freeArea, index int, area *freeArea) []*freeArea {
	freeAreas = append(freeAreas, nil)
	copy(freeAreas[index+1:], freeAreas[index:])
	freeAreas[index] = area
	return freeAreas
}

// centerOfSegment is center(HyperEdgeSegment).
func centerOfSegment(s *HyperEdgeSegment) float64 {
	return center(s.StartCoordinate(), s.EndCoordinate())
}

// centerOfArea is center(FreeArea).
func centerOfArea(a *freeArea) float64 {
	return center(a.startPosition, a.endPosition)
}

// center is center(p1, p2).
func center(p1, p2 float64) float64 {
	return float64((p1 + p2) / 2)
}

////////////////////////////////////////////////////////////////////////////
// Data Holders

// freeArea is the class FreeArea: a free area between two horizontal edge
// segments, where a new horizontal segment can connect the two parts of a
// split segment.
type freeArea struct {
	// startPosition is the start of the free area.
	startPosition float64
	// endPosition is the end of the free area.
	endPosition float64
	// size is the size of the area.
	size float64
}

func newFreeArea(startPosition, endPosition float64) *freeArea {
	return &freeArea{
		startPosition: startPosition,
		endPosition:   endPosition,
		size:          endPosition - startPosition,
	}
}

// areaRating is the class AreaRating: what happens when a segment is
// connected to its split partner through an area.
type areaRating struct {
	// dependencies is the number of dependencies that the area causes.
	dependencies int
	// crossings is the number of crossings that the area causes.
	crossings int
}
