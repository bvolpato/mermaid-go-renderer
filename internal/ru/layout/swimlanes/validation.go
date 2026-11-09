// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/validation.ts (MIT).

package swimlanes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// ValidationIssue is a quality issue of a finished layout. Type is
// "edge-node-overlap" or "edge-edge-crossing". TargetId is the second edge
// id (for crossings) or the node id (for overlaps).
type ValidationIssue struct {
	Type     string
	EdgeId   string
	TargetId string
	Detail   string
}

// segmentsIntersect checks if two line segments intersect. It returns true
// only for proper intersections: touching endpoints or collinear segments
// return false.
func segmentsIntersect(p1, p2, p3, p4 Point) bool {
	d1x := p2.X - p1.X
	d1y := p2.Y - p1.Y
	d2x := p4.X - p3.X
	d2y := p4.Y - p3.Y

	cross := float64(d1x*d2y) - float64(d1y*d2x)
	if math.Abs(cross) < 1e-10 {
		return false // parallel or collinear
	}

	dx := p3.X - p1.X
	dy := p3.Y - p1.Y
	t := (float64(dx*d2y) - float64(dy*d2x)) / cross
	u := (float64(dx*d1y) - float64(dy*d1x)) / cross

	const eps = 0.01
	return t > eps && t < 1-eps && u > eps && u < 1-eps
}

// validateSwimlanesLayout is the final validation pass: it scans the
// completed layout for remaining quality issues. It does not attempt
// fixes. Upstream logs the issues; here they are only returned.
//
// Checks:
//  1. Edge segments that still pass through non-endpoint nodes
//  2. Edge segments that cross other edge segments
func validateSwimlanesLayout(layout *ru.LayoutData) []ValidationIssue {
	nodes := layout.Nodes
	edges := layout.Edges
	issues := []ValidationIssue{}

	if len(edges) == 0 || len(nodes) == 0 {
		return issues
	}

	nodeRects := collectLayoutNodeRects(nodes)

	const epsilon = 1 // tighter than the fix pass: catch marginal overlaps
	type edgeSegment struct {
		edgeId string
		start  string
		end    string
		p1, p2 Point
	}
	edgeSegments := []edgeSegment{}

	for _, edge := range edges {
		if edge.IsLayoutOnly {
			continue
		}
		points := edge.Points
		if len(points) < 2 {
			continue
		}
		edgeStart := edge.Start
		edgeEnd := edge.End
		ownLabelId := edge.LabelNodeID
		edgeId := edge.ID

		for _, rect := range nodeRects {
			if rect.NodeId == edgeStart || rect.NodeId == edgeEnd {
				continue
			}
			if ownLabelId != "" && rect.NodeId == ownLabelId {
				continue
			}
			for i := 0; i < len(points)-1; i++ {
				if segmentBoundsOverlapRect(points[i], points[i+1], rect.RectBounds, -epsilon) {
					issues = append(issues, ValidationIssue{
						Type:     "edge-node-overlap",
						EdgeId:   edgeId,
						TargetId: rect.NodeId,
						Detail:   "segment " + js.Str(float64(i)) + " passes through node \"" + rect.NodeId + "\"",
					})
					break
				}
			}
		}

		for i := 0; i < len(points)-1; i++ {
			edgeSegments = append(edgeSegments, edgeSegment{
				edgeId: edgeId,
				start:  edgeStart,
				end:    edgeEnd,
				p1:     points[i],
				p2:     points[i+1],
			})
		}
	}

	crossingPairs := map[string]bool{}
	for i := 0; i < len(edgeSegments); i++ {
		for j := i + 1; j < len(edgeSegments); j++ {
			a := edgeSegments[i]
			b := edgeSegments[j]
			if a.edgeId == b.edgeId {
				continue
			}

			if a.start == b.start || a.start == b.end || a.end == b.start || a.end == b.end {
				continue
			}

			if segmentsIntersect(a.p1, a.p2, b.p1, b.p2) {
				pairKey := b.edgeId + "|" + a.edgeId
				if js.CompareStrings(a.edgeId, b.edgeId) < 0 {
					pairKey = a.edgeId + "|" + b.edgeId
				}
				if !crossingPairs[pairKey] {
					crossingPairs[pairKey] = true
					issues = append(issues, ValidationIssue{
						Type:     "edge-edge-crossing",
						EdgeId:   a.edgeId,
						TargetId: b.edgeId,
						Detail:   "edges \"" + a.edgeId + "\" and \"" + b.edgeId + "\" cross",
					})
				}
			}
		}
	}

	// Upstream logs a summary and every issue with log.warn.

	return issues
}
