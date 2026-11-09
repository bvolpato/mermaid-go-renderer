// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/portSwap.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

const portSwapEPS = 1e-6

// δ_s, the Kandinsky port-spacing constant. This pass shifts only the
// single edge being swapped, so it moves the full δ_s.
const portSwapMinPortSpacing = 8
const portSwapPortShift = portSwapMinPortSpacing

var portSwapTryDeltas = []float64{0, portSwapPortShift, -portSwapPortShift, 2 * portSwapPortShift, -2 * portSwapPortShift}

// portSwapToLShape port-swaps a 4-point H-V-H / V-H-V edge to a 3-point
// L-shape when the current src port is "straight-through" (parallel to the
// incoming edge) but a perpendicular src face permits a one-bend reach to
// the existing dst port.
//
// Rewrite (H-V-H):
//
//	new_src_port = (src.cx + δ, dst-below ? src.bottom : src.top)
//	new_polyline = [ new_src_port, (src.cx + δ, p3.y), p3 ]
//
// (V-H-V is symmetric across axes.) A candidate is rejected when a new
// segment hits a node, or crosses or overlaps a segment of another edge.
func portSwapToLShape(edges []*ru.Edge, nodes []*ru.Node) {
	nodeInfoById, realNodeRects := collectRealNodeBounds(nodes)

	for _, edge := range edges {
		if edge.IsLayoutOnly {
			continue
		}
		pts := edge.Points
		if len(pts) < 4 {
			continue
		}

		// Dedupe consecutive identical points (the router and the endpoint
		// clip can produce duplicates).
		route := classifyThreeSegmentRoute(dedupeConsecutivePoints(pts, portSwapEPS), portSwapEPS)
		if route == nil {
			continue
		}

		p3 := route.P3
		isHVH := route.Kind == "HVH"

		nodePair := getNodePairGeometry(edge, nodeInfoById, portSwapEPS)
		if nodePair == nil {
			continue
		}
		srcId, dstId, srcInfo, dstInfo := nodePair.SrcId, nodePair.DstId, nodePair.SrcInfo, nodePair.DstInfo

		// Skip collinear src/dst: straightenCollinearSiblingDetours handles
		// those.
		if nodePair.CollinearX || nodePair.CollinearY {
			continue
		}

		// Build the candidate new polyline.
		// H-V-H: swap src E/W face → N/S face. Preserve the dst port (p3).
		var newPts []Point
		srcRect := srcInfo.Rect

		for _, delta := range portSwapTryDeltas {
			var np0, np1, np2 Point

			if isHVH {
				dstBelow := dstInfo.Cy > srcInfo.Cy
				newSrcY := srcRect.Top
				if dstBelow {
					newSrcY = srcRect.Bottom
				}
				newSrcX := srcInfo.Cx + delta
				// Must lie strictly within the src face span (exclusive of
				// corners).
				if newSrcX <= srcRect.Left+portSwapEPS || newSrcX >= srcRect.Right-portSwapEPS {
					continue
				}
				np0 = Point{X: newSrcX, Y: newSrcY}
				np1 = Point{X: newSrcX, Y: p3.Y}
				np2 = Point{X: p3.X, Y: p3.Y}
			} else {
				// isVHV
				dstEast := dstInfo.Cx > srcInfo.Cx
				newSrcX := srcRect.Left
				if dstEast {
					newSrcX = srcRect.Right
				}
				newSrcY := srcInfo.Cy + delta
				if newSrcY <= srcRect.Top+portSwapEPS || newSrcY >= srcRect.Bottom-portSwapEPS {
					continue
				}
				np0 = Point{X: newSrcX, Y: newSrcY}
				np1 = Point{X: p3.X, Y: newSrcY}
				np2 = Point{X: p3.X, Y: p3.Y}
			}

			// Degenerate: if np1 === np2, the "L" collapses to a straight line.
			// Accept it, but keep it as 2 points.
			firstSegDegenerate := samePoint(np0, np1, portSwapEPS)
			secondSegDegenerate := samePoint(np1, np2, portSwapEPS)
			if firstSegDegenerate && secondSegDegenerate {
				continue
			}

			// No edge-node collisions. Seg 1 may touch src; seg 2 may touch
			// dst.
			if !firstSegDegenerate && segmentHitsAnyRect(np0, np1, realNodeRects, []string{srcId}, 1) {
				continue
			}
			if !secondSegDegenerate && segmentHitsAnyRect(np1, np2, realNodeRects, []string{dstId}, 1) {
				continue
			}

			// Check every other edge's every segment for a perpendicular
			// crossing or a collinear-axis overlap with either of the new
			// segments.
			firstSegConflicts := !firstSegDegenerate &&
				segmentConflictsWithAnyEdge(np0, np1, edges, edge, portSwapEPS, true)
			secondSegConflicts := !secondSegDegenerate &&
				segmentConflictsWithAnyEdge(np1, np2, edges, edge, portSwapEPS, true)
			if firstSegConflicts || secondSegConflicts {
				continue
			}

			// All guards pass. Build the final polyline.
			if firstSegDegenerate {
				newPts = []Point{np1, np2}
			} else if secondSegDegenerate {
				newPts = []Point{np0, np1}
			} else {
				newPts = []Point{np0, np1, np2}
			}
			break
		}

		if newPts != nil {
			edge.Points = newPts
		}
	}
}
