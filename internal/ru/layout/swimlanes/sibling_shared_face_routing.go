// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/siblingSharedFaceRouting.ts
// (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

const siblingEPS = 1e-6
const siblingMinPortSpacing = 8
const siblingPortShift = siblingMinPortSpacing / 2.0
const siblingLabelClearanceBuffer = 3

type siblingLabelDim struct {
	w, h float64
}

// pairKey is the key of an unordered pair of ids. The comparison is the <
// of JavaScript strings.
func pairKey(a, b string) string {
	if js.CompareStrings(a, b) < 0 {
		return a + "::" + b
	}
	return b + "::" + a
}

// straightenCollinearSiblingDetours is the co-route sibling straight-line
// rescue.
//
// It fires only on the narrow "4-point U-detour around a collinear blocker
// where the obvious straight line is geometrically clear" shape. For each
// eligible edge, it shifts the source and destination attach points along
// the shared face and replaces the polyline with a 2-point straight line.
// The shift is the first of 0, +delta, -delta that does not introduce a new
// edge crossing or leave the face span of the node.
func straightenCollinearSiblingDetours(edges []*ru.Edge, nodes []*ru.Node) {
	nodeInfoById, realNodeRects := collectRealNodeBounds(nodes)
	labelDimById := map[string]siblingLabelDim{}
	for _, n := range nodes {
		id := n.ID
		if n.IsGroup {
			continue
		}
		if n.IsEdgeLabel {
			labelDimById[id] = siblingLabelDim{
				w: ru.NN(n.Width, 0),
				h: ru.NN(n.Height, 0),
			}
			continue
		}
	}

	labelClearanceFor := func(thisEdge *ru.Edge, thisSrcId, thisDstId string, axis string) float64 {
		targetPair := pairKey(thisSrcId, thisDstId)
		maxHalf := 0.0
		consider := func(labelId string) {
			if labelId == "" {
				return
			}
			dim, ok := labelDimById[labelId]
			if !ok {
				return
			}
			half := dim.h / 2
			if axis == "x" {
				half = dim.w / 2
			}
			if half > maxHalf {
				maxHalf = half
			}
		}
		consider(thisEdge.LabelNodeID)
		for _, other := range edges {
			if other == thisEdge {
				continue
			}
			if other.IsLayoutOnly {
				continue
			}
			oSrc := other.Start
			oDst := other.End
			if oSrc == "" || oDst == "" {
				continue
			}
			if pairKey(oSrc, oDst) != targetPair {
				continue
			}
			consider(other.LabelNodeID)
		}
		if maxHalf > 0 {
			return maxHalf + siblingLabelClearanceBuffer
		}
		return 0
	}

	for _, edge := range edges {
		if edge.IsLayoutOnly {
			continue
		}
		pts := edge.Points
		if classifyThreeSegmentRoute(pts, siblingEPS) == nil {
			continue
		}

		nodePair := getNodePairGeometry(edge, nodeInfoById, siblingEPS)
		if nodePair == nil {
			continue
		}
		srcId, dstId, srcInfo, dstInfo := nodePair.SrcId, nodePair.DstId, nodePair.SrcInfo, nodePair.DstInfo
		collinearX, collinearY := nodePair.CollinearX, nodePair.CollinearY
		if collinearX == collinearY {
			continue
		}

		var targetSrc, targetDst Point
		if collinearX {
			dstBelow := dstInfo.Cy > srcInfo.Cy
			if dstBelow {
				targetSrc = Point{X: srcInfo.Cx, Y: srcInfo.Rect.Bottom}
				targetDst = Point{X: dstInfo.Cx, Y: dstInfo.Rect.Top}
			} else {
				targetSrc = Point{X: srcInfo.Cx, Y: srcInfo.Rect.Top}
				targetDst = Point{X: dstInfo.Cx, Y: dstInfo.Rect.Bottom}
			}
		} else {
			dstEast := dstInfo.Cx > srcInfo.Cx
			if dstEast {
				targetSrc = Point{X: srcInfo.Rect.Right, Y: srcInfo.Cy}
				targetDst = Point{X: dstInfo.Rect.Left, Y: dstInfo.Cy}
			} else {
				targetSrc = Point{X: srcInfo.Rect.Left, Y: srcInfo.Cy}
				targetDst = Point{X: dstInfo.Rect.Right, Y: dstInfo.Cy}
			}
		}

		if segmentHitsAnyRect(targetSrc, targetDst, realNodeRects, []string{srcId, dstId}, 1) {
			continue
		}

		shiftAxis := "y"
		if collinearX {
			shiftAxis = "x"
		}
		labelShift := labelClearanceFor(edge, srcId, dstId, shiftAxis)
		effectiveShift := float64(siblingPortShift)
		if labelShift > siblingPortShift {
			effectiveShift = labelShift
		}
		deltas := []float64{0, effectiveShift, -effectiveShift}
		for _, delta := range deltas {
			shiftedSrc := targetSrc
			shiftedDst := targetDst
			if collinearX {
				shiftedSrc.X += delta
				shiftedDst.X += delta
				if shiftedSrc.X <= srcInfo.Rect.Left || shiftedSrc.X >= srcInfo.Rect.Right {
					continue
				}
				if shiftedDst.X <= dstInfo.Rect.Left || shiftedDst.X >= dstInfo.Rect.Right {
					continue
				}
			} else {
				shiftedSrc.Y += delta
				shiftedDst.Y += delta
				if shiftedSrc.Y <= srcInfo.Rect.Top || shiftedSrc.Y >= srcInfo.Rect.Bottom {
					continue
				}
				if shiftedDst.Y <= dstInfo.Rect.Top || shiftedDst.Y >= dstInfo.Rect.Bottom {
					continue
				}
			}

			if segmentHitsAnyRect(shiftedSrc, shiftedDst, realNodeRects, []string{srcId, dstId}, 1) {
				continue
			}

			if segmentConflictsWithAnyEdge(shiftedSrc, shiftedDst, edges, edge, siblingEPS, false) {
				continue
			}

			edge.Points = []Point{shiftedSrc, shiftedDst}
			break
		}
	}
}
