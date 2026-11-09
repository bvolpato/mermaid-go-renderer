// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/postProcessing.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// postProcessSwimlaneLayout applies the direction transforms and the
// post-routing cleanup to a swimlane layout. direction is "" when it is
// undefined.
func postProcessSwimlaneLayout(layout *ru.LayoutData, direction string) {
	postProcessSwimlaneLayoutTraced(layout, direction, nil)
}

// postProcessSwimlaneLayoutTraced calls trace after each step with the
// name of the step. The tests compare the layout after each step with
// upstream.
func postProcessSwimlaneLayoutTraced(layout *ru.LayoutData, direction string, trace func(step string)) {
	step := func(name string) {
		if trace != nil {
			trace(name)
		}
	}
	nodes := layout.Nodes
	edges := layout.Edges
	hasContentNodes := false
	for _, n := range nodes {
		if !n.IsGroup {
			hasContentNodes = true
			break
		}
	}

	// TB is the canonical orientation. LR/RL rotate rank progression onto X;
	// BT mirrors the canonical Y progression. The cleanup passes below
	// operate in whichever coordinate system this step leaves behind.
	if (direction == "LR" || direction == "RL") &&
		hasContentNodes &&
		!applyLrDirectionTransform(layout, direction) {
		return
	}

	if direction == "BT" && hasContentNodes && !applyBtDirectionTransform(layout) {
		return
	}
	step("direction")

	for _, edge := range edges {
		if edge.IsLayoutOnly {
			continue
		}
		pts := edge.Points
		if len(pts) < 2 {
			continue
		}
		edge.Points = simplifyPolyline(orthogonalizePolyline(pts))
	}
	step("orthogonalize")

	simplifyDetouredEdges(edges, nodes)
	step("simplifyDetouredEdges")

	// Prefer shorter sibling routes when a local port shift keeps them
	// clear.
	straightenCollinearSiblingDetours(edges, nodes)
	step("straightenCollinearSiblingDetours")

	// Swap a source port only when it produces a clear L-shape with fewer
	// bends.
	portSwapToLShape(edges, nodes)
	step("portSwapToLShape")

	nodeByIdMap := js.NewMap[string, *ru.Node]()
	for _, n := range nodes {
		nodeByIdMap.Set(n.ID, n)
	}
	// Initial label anchoring against the routed polylines. The cleanup
	// passes below can still reshape these polylines, so labels are
	// re-anchored once more at the end.
	anchorLabelsToPolyline(edges, nodeByIdMap)
	step("anchorLabelsToPolyline")

	clipEdgeEndpointsToNodeBoundaries(edges, nodeByIdMap)
	step("clipEdgeEndpointsToNodeBoundaries")

	// Retarget short terminal stubs that are hidden by endpoint clipping.
	collapseShortTerminalStub(edges, nodeByIdMap)
	step("collapseShortTerminalStub")

	// Shared-track nudge. The router may bundle connectors onto the same
	// rail, but before rendering those coincident middle rails must be
	// separated into nearby parallel tracks.
	nudgeSharedInteriorSubpaths(edges, nodeByIdMap)
	step("nudgeSharedInteriorSubpaths")

	// Materialized-render terminal lane split: split the visible terminal
	// rails that share a node face before the endpoint-duplication handoff
	// pins them.
	separateSharedRenderedTerminalLanes(edges, nodeByIdMap)
	step("separateSharedRenderedTerminalLanes")

	// Once terminal lanes have been separated, some earlier same-track
	// detours become unnecessary. Remove only provably redundant rectangular
	// doglegs.
	collapseRedundantRectangularDoglegs(edges, nodeByIdMap)
	step("collapseRedundantRectangularDoglegs")

	// Same-side rails can be routed just inside a taller intervening node's
	// border. Lift those rails outside the blocker before crossing cleanup.
	liftObstacleHuggingSameSideRails(edges, nodeByIdMap)
	step("liftObstacleHuggingSameSideRails")

	// Some shared-destination crossings only improve when two terminal
	// ports exchange places together.
	swapDestinationTerminalTailsToReduceCrossings(edges, nodeByIdMap)
	step("swapDestinationTerminalTailsToReduceCrossings")

	finalizeRenderedEdges := func(round string) {
		resolveRenderedOrthogonalCrossings(edges, nodeByIdMap)
		step(round + ".resolveRenderedOrthogonalCrossings")
		// Crossing cleanup can bring a route onto a face another edge already
		// leaves or enters from, so split shared terminal lanes again before
		// endpoints are pinned.
		separateSharedRenderedTerminalLanes(edges, nodeByIdMap)
		step(round + ".separateSharedRenderedTerminalLanes")
		reassignCrossingExternalRailChannels(edges, nodeByIdMap)
		step(round + ".reassignCrossingExternalRailChannels")
		shortcutRedundantOrthogonalJogs(edges, nodeByIdMap)
		step(round + ".shortcutRedundantOrthogonalJogs")
		anchorLabelsToPolyline(edges, nodeByIdMap)
		step(round + ".anchorLabelsToPolyline1")
		prepareEdgeEndpointsForRenderer(edges, nodeByIdMap)
		step(round + ".prepareEdgeEndpointsForRenderer1")
		liftObstacleHuggingSameSideRails(edges, nodeByIdMap)
		step(round + ".liftObstacleHuggingSameSideRails")
		anchorLabelsToPolyline(edges, nodeByIdMap)
		step(round + ".anchorLabelsToPolyline2")
		prepareEdgeEndpointsForRenderer(edges, nodeByIdMap)
		step(round + ".prepareEdgeEndpointsForRenderer2")
	}

	// Crossing cleanup for the materialized render geometry. It tries
	// bounded port-pair and outer-channel candidates and accepts only
	// candidates that reduce the rendered crossing count or shorten an
	// equally crossing route.
	finalizeRenderedEdges("finalize1")

	// Renderer endpoint preparation materializes the visible endpoint
	// geometry. Run the shared-track pass once more on that visible shape.
	nudgeSharedInteriorSubpaths(edges, nodeByIdMap)
	step("nudgeSharedInteriorSubpaths2")

	// The materialized stubs can reveal a crossing that was not present in
	// the pre-materialized route, so run the bounded cleanup once more and
	// then re-prepare the endpoints it reshaped.
	finalizeRenderedEdges("finalize2")

	// Swimlane title bands are visual headers, not routing obstacles. After
	// all edge geometry is final, move the aligned title bands out of any
	// clear rail that still crosses the title section.
	liftTopLaneTitleBandsAboveRails(edges, nodeByIdMap, direction)
	step("liftTopLaneTitleBandsAboveRails1")
	shiftLeftLaneTitleBandsLeftOfRails(edges, nodeByIdMap, direction)
	step("shiftLeftLaneTitleBandsLeftOfRails1")
	// Moving one aligned title band can expose a second title/rail
	// interaction after the group bounds settle. The passes are idempotent,
	// so one bounded repeat keeps headers out of late crossing-cleanup
	// routes.
	liftTopLaneTitleBandsAboveRails(edges, nodeByIdMap, direction)
	step("liftTopLaneTitleBandsAboveRails2")
	shiftLeftLaneTitleBandsLeftOfRails(edges, nodeByIdMap, direction)
	step("shiftLeftLaneTitleBandsLeftOfRails2")
}
