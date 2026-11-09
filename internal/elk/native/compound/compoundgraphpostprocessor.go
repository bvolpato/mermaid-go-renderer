// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/compound/CompoundGraphPostprocessor.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package compound

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func init() {
	lgraph.Register("CompoundGraphPostprocessor", func() lgraph.Processor { return NewCompoundGraphPostprocessor() })
}

// tolerance is OrthogonalRoutingGenerator.TOLERANCE: a difference below it
// is zero.
const tolerance = 1e-3

// CompoundGraphPostprocessor is the processor CompoundGraphPostprocessor: it
// restores the cross-hierarchy edges that the CompoundGraphPreprocessor
// split. Each original edge gets the bend points, the junction points and
// the labels of its segments, in the coordinate system of one reference
// graph.
type CompoundGraphPostprocessor struct{}

// NewCompoundGraphPostprocessor is new CompoundGraphPostprocessor().
func NewCompoundGraphPostprocessor() *CompoundGraphPostprocessor {
	return &CompoundGraphPostprocessor{}
}

// hasJunctionPoints is HAS_JUNCTION_POINTS_PREDICATE: it checks if a given
// cross hierarchy edge has junction points.
func hasJunctionPoints(chEdge *CrossHierarchyEdge) bool {
	jps := prop.Get(chEdge.Edge(), lopts.JunctionPoints)
	return jps != nil && !jps.IsEmpty()
}

// Process is process(graph, monitor).
func (*CompoundGraphPostprocessor) Process(graph *lgraph.LGraph, ctx *lgraph.Context) {
	// whether bend points should be added whenever crossing a hierarchy
	// boundary
	addUnnecessaryBendpoints := prop.Get(graph, lopts.UnnecessaryBendpoints)

	// restore the cross-hierarchy map that was built by the preprocessor
	crossHierarchyMap, isMap := prop.Get(graph, iprops.CrossHierarchyMap).(*CrossHierarchyMap)
	if !isMap || crossHierarchyMap == nil {
		lgraph.Bail("CompoundGraphPostprocessor: NullPointerException, the graph has no cross hierarchy map")
	}

	// remember all dummy edges we encounter; these need to be removed at the
	// end
	dummyEdges := gwt.NewSet[*lgraph.LEdge]()

	comparator := NewCrossHierarchyEdgeComparator(graph)

	// iterate over all original edges
	for _, origEdge := range crossHierarchyMap.KeySet() {
		// find all cross-hierarchy edges the original edge was split into,
		// and sort them from source to target
		crossHierarchyEdges := crossHierarchyMap.Get(origEdge)
		gwt.SortFunc(crossHierarchyEdges, comparator.Compare)
		if len(crossHierarchyEdges) == 0 {
			lgraph.Bail("CompoundGraphPostprocessor: IndexOutOfBoundsException, an original edge has no segments")
		}

		// find the original source and target ports for the original edge
		sourcePort := crossHierarchyEdges[0].ActualSource()
		targetPort := crossHierarchyEdges[len(crossHierarchyEdges)-1].ActualTarget()
		if sourcePort == nil || targetPort == nil {
			lgraph.Bail("CompoundGraphPostprocessor: NullPointerException, an external port dummy has no origin")
		}

		// determine the reference graph for all bend points
		referenceNode := sourcePort.Node()
		var referenceGraph *lgraph.LGraph
		if lgraph.IsDescendant(targetPort.Node(), referenceNode) {
			referenceGraph = referenceNode.NestedGraph
		} else {
			referenceGraph = referenceNode.Graph()
		}

		// check whether there are any junction points
		junctionPoints := clearJunctionPoints(origEdge, crossHierarchyEdges)

		// reset bend points (we have computed new ones anyway)
		origEdge.BendPoints.Clear()

		// apply the computed layouts to the cross-hierarchy edge
		var lastPoint *emath.KVector
		for _, chEdge := range crossHierarchyEdges {
			// transform all coordinates from the graph of the dummy edge to
			// the reference graph
			offset := &emath.KVector{}
			lgraph.ChangeCoordSystem(offset, chEdge.Graph(), referenceGraph)

			ledge := chEdge.Edge()
			bendPoints := emath.NewKVectorChain()
			bendPoints.AddAllAsCopies(0, ledge.BendPoints.Points)
			bendPoints.Offset(offset)

			// Note: if an NPE occurs here, that means ELK Layered has
			// replaced the original edge
			if ledge.Source == nil || ledge.Target == nil {
				lgraph.Bail("CompoundGraphPostprocessor: NullPointerException, a dummy edge has lost a port")
			}
			sourcePoint := ledge.Source.AbsoluteAnchor()
			targetPoint := ledge.Target.AbsoluteAnchor()
			sourcePoint.Add(offset)
			targetPoint.Add(offset)

			if lastPoint != nil {
				var nextPoint *emath.KVector
				if bendPoints.IsEmpty() {
					nextPoint = targetPoint
				} else {
					nextPoint = bendPoints.First()
				}

				// we add the source point as a bend point to properly connect
				// the hierarchy levels either if the last point of the
				// previous hierarchy edge segment is a certain level of
				// tolerance away or if we are required to add unnecessary
				// bend points
				xDiffEnough := math.Abs(lastPoint.X-nextPoint.X) > tolerance
				yDiffEnough := math.Abs(lastPoint.Y-nextPoint.Y) > tolerance

				if (!addUnnecessaryBendpoints && xDiffEnough && yDiffEnough) ||
					(addUnnecessaryBendpoints && (xDiffEnough || yDiffEnough)) {

					origEdge.BendPoints.Add(sourcePoint)
				}
			}

			origEdge.BendPoints.AddAll(bendPoints.Points...)

			if bendPoints.IsEmpty() {
				lastPoint = sourcePoint
			} else {
				lastPoint = bendPoints.Last()
			}

			// copy junction points
			copyJunctionPoints(ledge, junctionPoints, offset)

			// add offset to target port with a special property
			if chEdge.ActualTarget() == targetPort {
				if targetPort.Node().Graph() != chEdge.Graph() {
					// the target port is in a different coordinate system --
					// recompute the offset
					offset = &emath.KVector{}
					lgraph.ChangeCoordSystem(offset, targetPort.Node().Graph(), referenceGraph)
				}
				prop.Set(origEdge, iprops.TargetOffset, offset)
			}

			// copy labels back to the original edge
			copyLabelsBack(ledge, origEdge, referenceGraph)

			// remember the dummy edge for later removal (dummy edges may be
			// in use by several different original edges, which is why we
			// cannot just go and remove it now)
			dummyEdges.Add(ledge)
		}

		// restore the original source port and target port
		origEdge.SetSource(sourcePort)
		origEdge.SetTarget(targetPort)
	}

	// remove the dummy edges from the graph (dummy ports and dummy nodes are
	// retained)
	for _, dummyEdge := range dummyEdges.Values() {
		dummyEdge.SetSource(nil)
		dummyEdge.SetTarget(nil)
	}
}

// clearJunctionPoints clears an original edge's list of junction points and
// returns them. The returned vector chain is already set on the original
// edge, so all the calling code needs to do to update the edge's junction
// points is to update the returned vector chain.
func clearJunctionPoints(origEdge *lgraph.LEdge, crossHierarchyEdges []*CrossHierarchyEdge) *emath.KVectorChain {
	junctionPoints := prop.Get(origEdge, lopts.JunctionPoints)
	anyJunctionPoints := false
	for _, chEdge := range crossHierarchyEdges {
		if hasJunctionPoints(chEdge) {
			anyJunctionPoints = true
			break
		}
	}
	if anyJunctionPoints {
		// if so, make sure the original edge has an empty non-null junction
		// point list
		if junctionPoints == nil {
			junctionPoints = emath.NewKVectorChain()
			prop.Set(origEdge, lopts.JunctionPoints, junctionPoints)
		} else {
			junctionPoints.Clear()
		}
	} else if junctionPoints != nil {
		prop.Unset(origEdge, lopts.JunctionPoints)
	}

	return junctionPoints
}

// copyJunctionPoints copies the junction points of the source to the target,
// adding the given offset.
func copyJunctionPoints(source *lgraph.LEdge, target *emath.KVectorChain, offset *emath.KVector) {
	ledgeJPs := prop.Get(source, lopts.JunctionPoints)
	if ledgeJPs != nil {
		jpCopies := emath.NewKVectorChain()
		jpCopies.AddAllAsCopies(0, ledgeJPs.Points)
		jpCopies.Offset(offset)

		if target == nil {
			lgraph.Bail("CompoundGraphPostprocessor: NullPointerException, the original edge has no junction point list")
		}
		target.AddAll(jpCopies.Points...)
	}
}

// copyLabelsBack copies the labels from the given hierarchy segment back to
// the original hierarchical edge.
func copyLabelsBack(hierarchySegment, origEdge *lgraph.LEdge, referenceGraph *lgraph.LGraph) {
	for labelIndex := 0; labelIndex < len(hierarchySegment.Labels); {
		currLabel := hierarchySegment.Labels[labelIndex]
		if prop.Get(currLabel, iprops.OriginalLabelEdge) != origEdge {
			labelIndex++
			continue
		}

		lgraph.ChangeCoordSystem(&currLabel.Pos, hierarchySegment.Source.Node().Graph(), referenceGraph)
		hierarchySegment.Labels = append(hierarchySegment.Labels[:labelIndex], hierarchySegment.Labels[labelIndex+1:]...)
		origEdge.Labels = append(origEdge.Labels, currLabel)
	}
}
