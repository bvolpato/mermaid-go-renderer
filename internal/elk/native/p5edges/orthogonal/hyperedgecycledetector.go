// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/orthogonal/HyperEdgeCycleDetector.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package orthogonal

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// DetectCycles is HyperEdgeCycleDetector.detectCycles(segments, criticalOnly,
// random): it returns dependencies whose reversal or removal makes the graph
// of the segments acyclic. With criticalOnly it looks at the critical
// dependencies only. Without it, it expects that the critical dependencies
// have no cycle, and does not return a critical dependency.
//
// random can be nil. The layout then stops when the method needs a random
// number, where Java fails with a NullPointerException.
func DetectCycles(segments []*HyperEdgeSegment, criticalOnly bool, random *gwt.Random) []*HyperEdgeSegmentDependency {
	result := []*HyperEdgeSegmentDependency{}

	var sources, sinks []*HyperEdgeSegment

	// initialize values for the algorithm
	sources, sinks = initialize(segments, sources, sinks, criticalOnly)

	// assign marks to all nodes
	computeLinearOrderingMarks(segments, sources, sinks, criticalOnly, random)

	// process edges that point left: remove those of zero weight, reverse the
	// others
	for _, source := range segments {
		for _, outDependency := range source.outgoingSegmentDependencies {
			// Only consider critical dependencies, if required
			if !criticalOnly || outDependency.typ == DependencyTypeCritical {
				if source.mark > outDependency.target.mark {
					result = append(result, outDependency)
				}
			}
		}
	}

	return result
}

// initialize is initialize(segments, sources, sinks, criticalOnly): it sets
// the mark and the weights of each segment, and adds the segments without
// outgoing weight to the sinks and the segments without incoming weight to
// the sources. After it the marks are -1 to -len(segments).
func initialize(segments, sources, sinks []*HyperEdgeSegment, criticalOnly bool) (newSources, newSinks []*HyperEdgeSegment) {
	nextMark := -1
	for _, segment := range segments {
		segment.mark = nextMark
		nextMark--

		// Sum up the weights of our critical dependencies
		criticalInWeight := 0
		for _, dep := range segment.incomingSegmentDependencies {
			if dep.typ == DependencyTypeCritical {
				criticalInWeight += dep.weight
			}
		}

		criticalOutWeight := 0
		for _, dep := range segment.outgoingSegmentDependencies {
			if dep.typ == DependencyTypeCritical {
				criticalOutWeight += dep.weight
			}
		}

		// If we're only considering critical dependencies, we'll ignore the
		// others
		inWeight := criticalInWeight
		outWeight := criticalOutWeight

		if !criticalOnly {
			// Just sum up everything
			inWeight = 0
			for _, dep := range segment.incomingSegmentDependencies {
				inWeight += dep.weight
			}

			outWeight = 0
			for _, dep := range segment.outgoingSegmentDependencies {
				outWeight += dep.weight
			}
		}

		// Apply the weight
		segment.SetInWeight(inWeight)
		segment.SetCriticalInWeight(criticalInWeight)
		segment.SetOutWeight(outWeight)
		segment.SetCriticalOutWeight(criticalOutWeight)

		// Add the segment to either sources or sinks if the corresponding
		// weight is zero
		if outWeight == 0 {
			sinks = append(sinks, segment)
		} else if inWeight == 0 {
			sources = append(sources, segment)
		}
	}
	return sources, sinks
}

// markSet is the TreeSet of the segments that are not processed. The Java
// set orders the segments by compareTo, which is the order of the marks. A
// segment leaves the set before its mark changes, so each segment of the set
// has the mark that initialize gave to it, and the marks of the set are all
// different.
type markSet struct {
	// segments has the segments of the set by ascending mark.
	segments []*HyperEdgeSegment
}

// newMarkSet is Sets.newTreeSet(segments). A segment whose mark is in the set
// is not added.
func newMarkSet(segments []*HyperEdgeSegment) *markSet {
	set := &markSet{}
	for _, segment := range segments {
		i, found := set.find(segment.mark)
		if found {
			continue
		}
		set.segments = append(set.segments, nil)
		copy(set.segments[i+1:], set.segments[i:])
		set.segments[i] = segment
	}
	return set
}

// find returns the index of the segment with the mark, or the index where a
// segment with the mark belongs.
func (s *markSet) find(mark int) (index int, found bool) {
	lo, hi := 0, len(s.segments)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if s.segments[mid].mark < mark {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, lo < len(s.segments) && s.segments[lo].mark == mark
}

// remove is remove(segment): it removes the segment of the set whose mark is
// the mark of the given segment. That can be another segment, as in Java.
func (s *markSet) remove(segment *HyperEdgeSegment) {
	if i, found := s.find(segment.mark); found {
		s.segments = append(s.segments[:i], s.segments[i+1:]...)
	}
}

// computeLinearOrderingMarks is computeLinearOrderingMarks(segments, sources,
// sinks, criticalOnly, random): it gives each segment a mark from a linear
// order of the segments. The marks are all different, and the sinks have
// higher marks than the sources.
func computeLinearOrderingMarks(segments, sources, sinks []*HyperEdgeSegment, criticalOnly bool, random *gwt.Random) {
	unprocessed := newMarkSet(segments)
	maxSegments := []*HyperEdgeSegment{}

	// We'll mark sinks with marks < markBase and sources with marks >
	// markBase. Sink marks will later be offset to be higher than the source
	// marks, but this way, the sink marks will reflect the order in which
	// the sinks were discovered and added.
	markBase := len(segments)
	nextSinkMark := markBase - 1
	nextSourceMark := markBase + 1

	for len(unprocessed.segments) != 0 {
		for len(sinks) != 0 {
			sink := sinks[0]
			sinks = sinks[1:]
			unprocessed.remove(sink)
			sink.mark = nextSinkMark
			nextSinkMark--
			sources, sinks = updateNeighbors(sink, sources, sinks, criticalOnly)
		}

		for len(sources) != 0 {
			source := sources[0]
			sources = sources[1:]
			unprocessed.remove(source)
			source.mark = nextSourceMark
			nextSourceMark++
			sources, sinks = updateNeighbors(source, sources, sinks, criticalOnly)
		}

		// If any segments are still unprocessed, they are neither source nor
		// sink. Assemble the list of segments with the highest out flow (out
		// weight - in weight), to be placed among the sources. If we're
		// looking at both, critical and non-critical dependencies, we must be
		// sure that critical dependencies will always point rightwards; we
		// thus stop immediately once we find one
		maxOutflow := math.MinInt32
		for _, segment := range unprocessed.segments {
			// Once we find a single segment that still has an outgoing
			// critical dependency and no incoming ones, we'll take that and
			// leave!
			if !criticalOnly && segment.CriticalOutWeight() > 0 && segment.CriticalInWeight() <= 0 {
				maxSegments = maxSegments[:0]
				maxSegments = append(maxSegments, segment)
				break
			}

			outflow := segment.OutWeight() - segment.InWeight()
			if outflow >= maxOutflow {
				if outflow > maxOutflow {
					maxSegments = maxSegments[:0]
					maxOutflow = outflow
				}
				maxSegments = append(maxSegments, segment)
			}
		}

		// If there are segments with maximal out flow, select one randomly;
		// this might yield new sources and sinks
		if len(maxSegments) != 0 {
			if random == nil {
				lgraph.Bail("HyperEdgeCycleDetector: the graph has no random number generator")
			}
			maxNode := maxSegments[random.NextInt(len(maxSegments))]
			unprocessed.remove(maxNode)
			maxNode.mark = nextSourceMark
			nextSourceMark++
			sources, sinks = updateNeighbors(maxNode, sources, sinks, criticalOnly)
			maxSegments = maxSegments[:0]
		}
	}

	// shift ranks that are left of the mark base so that sinks now have
	// higher marks than sources
	shiftBase := len(segments) + 1
	for _, node := range segments {
		if node.mark < markBase {
			node.mark = node.mark + shiftBase
		}
	}
}

// updateNeighbors is updateNeighbors(node, sources, sinks, criticalOnly): it
// updates the weights of the neighbors of the node as if the node left the
// graph, and adds the neighbors that become sources or sinks to the lists.
func updateNeighbors(node *HyperEdgeSegment, sources, sinks []*HyperEdgeSegment, criticalOnly bool) (newSources, newSinks []*HyperEdgeSegment) {
	// process following nodes
	for _, dep := range node.outgoingSegmentDependencies {
		if !criticalOnly || dep.typ == DependencyTypeCritical {
			target := dep.target
			if target.mark < 0 && dep.weight > 0 {
				// Remove weight (and possibly critical weight) from the target
				target.SetInWeight(target.InWeight() - dep.weight)
				if dep.typ == DependencyTypeCritical {
					target.SetCriticalInWeight(target.CriticalInWeight() - dep.weight)
				}

				if target.InWeight() <= 0 && target.OutWeight() > 0 {
					sources = append(sources, target)
				}
			}
		}
	}

	// process preceding nodes
	for _, dep := range node.incomingSegmentDependencies {
		if !criticalOnly || dep.typ == DependencyTypeCritical {
			source := dep.source
			if source.mark < 0 && dep.weight > 0 {
				// Remove weight (and possibly critical weight) from the source
				source.SetOutWeight(source.OutWeight() - dep.weight)
				if dep.typ == DependencyTypeCritical {
					source.SetCriticalOutWeight(source.CriticalOutWeight() - dep.weight)
				}

				if source.OutWeight() <= 0 && source.InWeight() > 0 {
					sinks = append(sinks, source)
				}
			}
		}
	}
	return sources, sinks
}
