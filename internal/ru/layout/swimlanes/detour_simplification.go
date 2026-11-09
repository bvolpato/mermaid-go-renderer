// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/detourSimplification.ts
// (MIT).

package swimlanes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

const detourEPS = 1e-3
const detourMinShared = 8

// simplifyDetouredEdges replaces the route of an edge with four or more
// bends by a route between the centers of two free faces of its nodes,
// when that route has fewer bends or fewer conflicts.
func simplifyDetouredEdges(edges []*ru.Edge, nodes []*ru.Node) {
	nodeInfoById, realNodeRects := collectRealNodeBounds(nodes)

	sides := []RectSide{"top", "bottom", "left", "right"}

	// Anchor offset for port exit. The first and the last segment of a port
	// must extend in the perpendicular direction of the port by at least
	// this many units before turning. Matches ANCHOR_OFFSET of the router.
	const ANCHOR = 20

	tops := make([]float64, len(realNodeRects))
	bottoms := make([]float64, len(realNodeRects))
	lefts := make([]float64, len(realNodeRects))
	rights := make([]float64, len(realNodeRects))
	for i, node := range realNodeRects {
		tops[i] = node.Rect.Top
		bottoms[i] = node.Rect.Bottom
		lefts[i] = node.Rect.Left
		rights[i] = node.Rect.Right
	}
	outsideTracks := map[RectSide]float64{
		"top":    js.Min(tops...) - ANCHOR,
		"bottom": js.Max(bottoms...) + ANCHOR,
		"left":   js.Min(lefts...) - ANCHOR,
		"right":  js.Max(rights...) + ANCHOR,
	}

	buildOrthogonalPathCandidates := func(src Point, srcSide RectSide, dst Point, dstSide RectSide) [][]Point {
		paths := [][]Point{}
		base := buildOrthogonalPortPath(src, srcSide, dst, dstSide, ANCHOR, detourEPS)
		if base != nil {
			paths = append(paths, base)
		}

		// Crossing-reduction extension of the same-side detour rule: when the
		// local "just outside these two ports" track still crosses an
		// existing connector, also try the corresponding global outer
		// channel.
		if srcSide == dstSide {
			paths = append(paths, buildSameSideTrackPath(src, srcSide, dst, outsideTracks[srcSide]))
		}

		return paths
	}

	pathHitsNode := func(pts []Point, excludeIds []string) bool {
		for i := 0; i < len(pts)-1; i++ {
			a := pts[i]
			b := pts[i+1]
			if segmentHitsAnyRect(a, b, realNodeRects, excludeIds, 1) {
				return true
			}
		}
		return false
	}

	pathConflictCount := func(path []Point, currentEdge *ru.Edge, includeIncidentEdges bool) int {
		conflicts := 0
		pathSegments := orthogonalSegmentsForPoints(path, detourEPS)
		currentStart := currentEdge.Start
		currentEnd := currentEdge.End
		for _, other := range edges {
			if other == currentEdge || other.IsLayoutOnly {
				continue
			}
			otherStart := other.Start
			otherEnd := other.End
			if !includeIncidentEdges &&
				currentStart != "" &&
				currentEnd != "" &&
				(otherStart == currentStart ||
					otherStart == currentEnd ||
					otherEnd == currentStart ||
					otherEnd == currentEnd) {
				continue
			}
			otherPts := other.Points
			if len(otherPts) < 2 {
				continue
			}
			for _, pathSegment := range pathSegments {
				for _, otherSegment := range orthogonalSegmentsForPoints(otherPts, detourEPS) {
					if orthogonalSegmentsCross(
						pathSegment.A,
						pathSegment.B,
						otherSegment.A,
						otherSegment.B,
						detourEPS,
						detourEPS,
					) {
						conflicts++
						continue
					}
					if sameAxisSegmentOverlapLength(pathSegment, otherSegment, detourEPS) >= detourMinShared {
						conflicts++
					}
				}
			}
		}
		return conflicts
	}

	const BEND_THRESHOLD = 4

	// Collect which node faces are already claimed by other edges, so that
	// the rewrite loop below can reject a candidate port pair whose face is
	// contested: two edges claiming the same node face must be
	// feasibility-checked as a set, never accepted as a sequential patch.
	//
	// The face detection picks whichever of the 4 rect edges the point is
	// closest to. It works regardless of whether the point is on, just
	// outside, or a few units inside the rect.
	nearestSideOfRect := func(pt Point, info *NodeBoundsInfo) RectSide {
		dTop := math.Abs(pt.Y - info.Rect.Top)
		dBottom := math.Abs(pt.Y - info.Rect.Bottom)
		dLeft := math.Abs(pt.X - info.Rect.Left)
		dRight := math.Abs(pt.X - info.Rect.Right)
		best := "top"
		bestDist := dTop
		if dBottom < bestDist {
			best = "bottom"
			bestDist = dBottom
		}
		if dLeft < bestDist {
			best = "left"
			bestDist = dLeft
		}
		if dRight < bestDist {
			best = "right"
		}
		return best
	}

	type FaceClaim struct {
		side   RectSide
		edgeId string
	}
	faceClaims := map[string][]FaceClaim{}
	addFaceClaim := func(nodeId string, side RectSide, edgeId string) {
		faceClaims[nodeId] = append(faceClaims[nodeId], FaceClaim{side: side, edgeId: edgeId})
	}
	for _, e := range edges {
		if e.IsLayoutOnly {
			continue
		}
		pts := e.Points
		if len(pts) < 1 {
			continue
		}
		eId := e.ID
		startId := e.Start
		endId := e.End
		if startId != "" {
			if info := nodeInfoById[startId]; info != nil {
				addFaceClaim(startId, nearestSideOfRect(pts[0], info), eId)
			}
		}
		if endId != "" {
			if info := nodeInfoById[endId]; info != nil {
				addFaceClaim(endId, nearestSideOfRect(pts[len(pts)-1], info), eId)
			}
		}
	}

	faceIsClaimed := func(nodeId string, side RectSide, ignoreEdgeId string) bool {
		for _, c := range faceClaims[nodeId] {
			if c.edgeId != ignoreEdgeId && c.side == side {
				return true
			}
		}
		return false
	}

	// withoutEdge is claims.filter((c) => c.edgeId !== edgeId).
	withoutEdge := func(claims []FaceClaim, edgeId string) []FaceClaim {
		out := []FaceClaim{}
		for _, c := range claims {
			if c.edgeId != edgeId {
				out = append(out, c)
			}
		}
		return out
	}

	for _, edge := range edges {
		if edge.IsLayoutOnly {
			continue
		}
		pts := edge.Points
		if len(pts) < 2 {
			continue
		}
		currentBends := countOrthogonalBends(pts, detourEPS)
		if currentBends < BEND_THRESHOLD {
			continue
		}
		srcId := edge.Start
		dstId := edge.End
		if srcId == "" || dstId == "" {
			continue
		}
		srcInfo := nodeInfoById[srcId]
		dstInfo := nodeInfoById[dstId]
		if srcInfo == nil || dstInfo == nil {
			continue
		}
		edgeId := edge.ID
		currentCrossingConflicts := pathConflictCount(pts, edge, true)
		currentNonIncidentConflicts := pathConflictCount(pts, edge, false)

		var bestPath []Point
		bestCrossingConflicts := currentCrossingConflicts
		bestBends := currentBends

		for _, srcSide := range sides {
			if faceIsClaimed(srcId, srcSide, edgeId) {
				continue
			}
			srcPort := portForRectSide(srcInfo.Cx, srcInfo.Cy, srcInfo.Rect, srcSide)
			for _, dstSide := range sides {
				if faceIsClaimed(dstId, dstSide, edgeId) {
					continue
				}
				dstPort := portForRectSide(dstInfo.Cx, dstInfo.Cy, dstInfo.Rect, dstSide)
				for _, path := range buildOrthogonalPathCandidates(srcPort, srcSide, dstPort, dstSide) {
					if pathHitsNode(path, []string{srcId, dstId}) {
						continue
					}

					pathBends := countOrthogonalBends(path, detourEPS)
					if currentCrossingConflicts > 0 {
						pathCrossingConflicts := pathConflictCount(path, edge, true)
						if pathCrossingConflicts > bestCrossingConflicts ||
							(pathCrossingConflicts == bestCrossingConflicts && pathBends >= bestBends) {
							continue
						}
						bestCrossingConflicts = pathCrossingConflicts
						bestBends = pathBends
						bestPath = path
						continue
					}

					if pathConflictCount(path, edge, false) > currentNonIncidentConflicts {
						continue
					}
					if pathBends < bestBends {
						bestBends = pathBends
						bestPath = path
					}
				}
			}
		}

		if bestPath != nil {
			edge.Points = bestPath
			// Refresh the face claims of this edge so that the later
			// iterations see the new attach sides.
			if refreshSrc, ok := faceClaims[srcId]; ok {
				faceClaims[srcId] = withoutEdge(refreshSrc, edgeId)
			}
			if refreshDst, ok := faceClaims[dstId]; ok {
				faceClaims[dstId] = withoutEdge(refreshDst, edgeId)
			}
			addFaceClaim(srcId, nearestSideOfRect(bestPath[0], srcInfo), edgeId)
			addFaceClaim(dstId, nearestSideOfRect(bestPath[len(bestPath)-1], dstInfo), edgeId)
		}
	}
}
