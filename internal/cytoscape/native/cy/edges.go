// Ported from cytoscape 3.34.0 (MIT license):
// src/extensions/renderer/base/coord-ele-math/edge-control-points.mjs,
// edge-endpoints.mjs and edge-projection.mjs.

package cy

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/internal/bail"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Point is a position.
type Point struct {
	X, Y float64
}

// EdgeScratch has the members of the renderer scratch of an edge
// (edge._private.rscratch) that mermaid reads, and the ones that they are
// computed from.
type EdgeScratch struct {
	StartX, StartY           float64
	EndX, EndY               float64
	ArrowStartX, ArrowStartY float64
	ArrowEndX, ArrowEndY     float64
	MidX, MidY               float64
}

// StraightEdge is findEdgeControlPoints for an edge of the type straight
// between two nodes of the shape ellipse, with the default endpoints
// (outside-to-node) and without arrows: findStraightEdgePoints,
// findEndpoints and storeAllpts. srcW, srcH, tgtW and tgtH are outerWidth()
// and outerHeight() of the two nodes.
func StraightEdge(srcPos, tgtPos Point, srcW, srcH, tgtW, tgtH float64) EdgeScratch {
	var rs EdgeScratch

	// findEndpoints. The manual endpoints have no value: [0, 0].
	// p1 is the last known point of the edge on the target side, p2 the one
	// on the source side.
	srcArrowFromPt := Point{tgtPos.X + 0, tgtPos.Y + 0}
	tgtArrowFromPt := Point{srcPos.X + 0, srcPos.Y + 0}
	p1 := tgtArrowFromPt
	p2 := srcArrowFromPt

	// tgtManEndptVal === 'outside-to-node': intersect p1 with the target
	// shape.
	ix, iy := IntersectLineEllipse(p1.X, p1.Y, tgtPos.X, tgtPos.Y, float64(tgtW/2)+0, float64(tgtH/2)+0)
	// The arrow shape none has no spacing and no gap, and the distance from
	// the node is 0.
	arrowEndX, arrowEndY := ShortenIntersection(ix, iy, p1.X, p1.Y, 0+0)
	edgeEndX, edgeEndY := ShortenIntersection(ix, iy, p1.X, p1.Y, 0+0)
	rs.EndX = edgeEndX
	rs.EndY = edgeEndY
	rs.ArrowEndX = arrowEndX
	rs.ArrowEndY = arrowEndY

	ix, iy = IntersectLineEllipse(p2.X, p2.Y, srcPos.X, srcPos.Y, float64(srcW/2)+0, float64(srcH/2)+0)
	arrowStartX, arrowStartY := ShortenIntersection(ix, iy, p2.X, p2.Y, 0+0)
	edgeStartX, edgeStartY := ShortenIntersection(ix, iy, p2.X, p2.Y, 0+0)
	rs.StartX = edgeStartX
	rs.StartY = edgeStartY
	rs.ArrowStartX = arrowStartX
	rs.ArrowStartY = arrowStartY

	// storeAllpts: the default midpoint for labels.
	rs.MidX = (rs.StartX + rs.EndX + rs.ArrowStartX + rs.ArrowEndX) / 4
	rs.MidY = (rs.StartY + rs.EndY + rs.ArrowStartY + rs.ArrowEndY) / 4
	return rs
}

// findEdgeControlPoints is BRp.findEdgeControlPoints for edges of the curve
// styles straight and segments with manual endpoints, between nodes without
// children: the renderer computes the points of each edge, and its label.
//
// The JavaScript groups the edges by the pair of their nodes, and computes
// for each pair the points where the line between the two nodes leaves their
// shapes. An edge of these two styles with manual endpoints does not read
// those points, so the port does not compute them. The order of the edges
// stays: by pair in the order of the first edge of each pair.
func (cy *Instance) findEdgeControlPoints(edges []*Element) {
	if len(edges) == 0 {
		return
	}
	type pairID struct{ a, b int }
	var pairIDs []pairID
	table := map[pairID][]*Element{}

	// create a table of edge (src, tgt) => list of edges between them
	for _, edge := range edges {
		// ignore edges who are not to be displayed
		// they shouldn't take up space
		if !edge.source.takesUpSpace() || !edge.target.takesUpSpace() || !edge.takesUpSpace() {
			bail.Out("cytoscape: an edge of a node that takes up no space")
		}
		src, tgt := edge.source, edge.target
		if src.IsParent() || tgt.IsParent() || src == tgt {
			bail.Out("cytoscape: an edge of a compound node, or from a node to itself")
		}
		// pairId is [srcIndex, tgtIndex].sort(): the sort without a
		// comparator compares the texts of the two indexes.
		id := pairID{src.poolIndex, tgt.poolIndex}
		if js.Str(float64(id.a)) > js.Str(float64(id.b)) {
			id.a, id.b = id.b, id.a
		}
		if _, ok := table[id]; !ok {
			pairIDs = append(pairIDs, id)
		}
		table[id] = append(table[id], edge)
	}

	// for each pair (src, tgt), create the ctrl pts
	for _, id := range pairIDs {
		for _, edge := range table[id] {
			if edge.EdgeStyle.Segments {
				cy.findSegmentsPoints(edge)
			}
			// findStraightEdgePoints only sets the edge type.
			cy.findEndpoints(edge)
			cy.storeAllpts(edge)
			cy.storeEdgeProjections(edge)
			cy.recalculateEdgeLabelProjections(edge)
		}
	}
}

// manualEndptToPx is BRp.manualEndptToPx for an endpoint with two values.
func manualEndptToPx(node *Element, prop Endpoint) (float64, float64) {
	npos := node.Position()
	w := node.OuterWidth()
	h := node.OuterHeight()
	p0, p1 := prop.PfValue[0], prop.PfValue[1]
	if prop.Percent[0] {
		p0 = float64(p0 * w)
	}
	if prop.Percent[1] {
		p1 = float64(p1 * h)
	}
	p0 += npos.X
	p1 += npos.Y
	return p0, p1
}

// findSegmentsPoints is BRp.findSegmentsPoints with the edge distances
// endpoints (findMidptPtsEtc): the points of the segments, by the weights
// and the distances of the style.
func (cy *Instance) findSegmentsPoints(edge *Element) {
	segmentWs := edge.EdgeStyle.SegmentWeights
	segmentDs := edge.EdgeStyle.SegmentDistances
	segmentsN := min(len(segmentWs), len(segmentDs))
	edge.segpts = edge.segpts[:0]
	for s := 0; s < segmentsN; s++ {
		w := segmentWs[s]
		d := segmentDs[s]
		w1 := 1 - w
		w2 := w
		// findMidptPtsEtc: the manual endpoints are the ends of the line.
		x1, y1 := manualEndptToPx(edge.source, edge.EdgeStyle.SourceEndpoint)
		x2, y2 := manualEndptToPx(edge.target, edge.EdgeStyle.TargetEndpoint)
		// recalcVectorNormInverse
		dy := y2 - y1
		dx := x2 - x1
		l := math.Sqrt(float64(dx*dx) + float64(dy*dy))
		vectorNormInverseX := -dy / l
		vectorNormInverseY := dx / l
		adjustedMidptX := float64(x1*w1) + float64(x2*w2)
		adjustedMidptY := float64(y1*w1) + float64(y2*w2)
		edge.segpts = append(edge.segpts, adjustedMidptX+float64(vectorNormInverseX*d), adjustedMidptY+float64(vectorNormInverseY*d))
	}
}

// findEndpoints is BRp.findEndpoints for an edge of the type straight or
// segments with manual endpoints, without arrows.
func (cy *Instance) findEndpoints(edge *Element) {
	source := edge.source
	target := edge.target
	srcPos := source.Position()
	tgtPos := target.Position()
	rs := &edge.rs
	segments := edge.EdgeStyle.Segments
	srcManEndpt := edge.EdgeStyle.SourceEndpoint
	tgtManEndpt := edge.EdgeStyle.TargetEndpoint

	// p1 is the last known point of the edge on the target side, p2 the one
	// on the source side.
	var p1, p2 Point
	// The JavaScript adds the pfValue of the manual endpoint to the position
	// of the node here: the numbers of the style, not their pixel values.
	tgtManEndptPt := tgtManEndpt.PfValue
	srcManEndptPt := srcManEndpt.PfValue
	if !segments {
		srcArrowFromPt := Point{tgtPos.X + tgtManEndptPt[0], tgtPos.Y + tgtManEndptPt[1]}
		tgtArrowFromPt := Point{srcPos.X + srcManEndptPt[0], srcPos.Y + srcManEndptPt[1]}
		p1 = tgtArrowFromPt
		p2 = srcArrowFromPt
	} else {
		if len(edge.segpts) < 2 {
			bail.Out("cytoscape: an edge of the style segments without a segment")
		}
		srcArrowFromPt := Point{edge.segpts[0], edge.segpts[1]}
		tgtArrowFromPt := Point{edge.segpts[len(edge.segpts)-2], edge.segpts[len(edge.segpts)-1]}
		p1 = tgtArrowFromPt
		p2 = srcArrowFromPt
	}

	// tgtManEndpt.units: the endpoint is the manual one.
	ix, iy := manualEndptToPx(target, tgtManEndpt)
	arrowEndX, arrowEndY := ShortenIntersection(ix, iy, p1.X, p1.Y, 0+0)
	edgeEndX, edgeEndY := ShortenIntersection(ix, iy, p1.X, p1.Y, 0+0)
	rs.EndX = edgeEndX
	rs.EndY = edgeEndY
	rs.ArrowEndX = arrowEndX
	rs.ArrowEndY = arrowEndY

	ix, iy = manualEndptToPx(source, srcManEndpt)
	arrowStartX, arrowStartY := ShortenIntersection(ix, iy, p2.X, p2.Y, 0+0)
	edgeStartX, edgeStartY := ShortenIntersection(ix, iy, p2.X, p2.Y, 0+0)
	rs.StartX = edgeStartX
	rs.StartY = edgeStartY
	rs.ArrowStartX = arrowStartX
	rs.ArrowStartY = arrowStartY
}

// storeAllpts is BRp.storeAllpts: the midpoint of the edge.
func (cy *Instance) storeAllpts(edge *Element) {
	rs := &edge.rs
	if !edge.EdgeStyle.Segments {
		// default midpt for labels etc
		rs.MidX = (rs.StartX + rs.EndX + rs.ArrowStartX + rs.ArrowEndX) / 4
		rs.MidY = (rs.StartY + rs.EndY + rs.ArrowStartY + rs.ArrowEndY) / 4
		return
	}
	segpts := edge.segpts
	if len(segpts)%4 == 0 {
		i2 := len(segpts) / 2
		i1 := i2 - 2
		rs.MidX = (segpts[i1] + segpts[i2]) / 2
		rs.MidY = (segpts[i1+1] + segpts[i2+1]) / 2
	} else {
		i := len(segpts)/2 - 1
		rs.MidX = segpts[i]
		rs.MidY = segpts[i+1]
	}
}

// storeEdgeProjections is BRp.storeEdgeProjections: the points of the lines
// of an edge of the type segments.
func (cy *Instance) storeEdgeProjections(edge *Element) {
	rstyle := &edge.rstyle
	rstyle.linePts = rstyle.linePts[:0]
	rstyle.hasLinePts = false
	if edge.EdgeStyle.Segments {
		rs := &edge.rs
		rstyle.hasLinePts = true
		rstyle.linePts = append(rstyle.linePts, Point{rs.StartX, rs.StartY})
		for i := 0; i+1 < len(edge.segpts); i += 2 {
			rstyle.linePts = append(rstyle.linePts, Point{edge.segpts[i], edge.segpts[i+1]})
		}
		rstyle.linePts = append(rstyle.linePts, Point{rs.EndX, rs.EndY})
	}
}

// recalculateEdgeLabelProjections is BRp.recalculateEdgeLabelProjections
// for an edge without a source label and a target label: the label is at
// the midpoint.
func (cy *Instance) recalculateEdgeLabelProjections(edge *Element) {
	if edge.label() == "" {
		return // no labels => no calcs
	}
	rs := &edge.rs
	edge.rstyle.labelX = rs.MidX
	edge.rstyle.labelY = rs.MidY
	cy.applyLabelDimensions(edge)
}

// SetSegments is the two calls edge.style('segment-distances', distance) and
// edge.style('segment-weights', weight): a style bypass, which drops the
// cached bounds of the edge.
func (e *Element) SetSegments(distance, weight float64) {
	// parseFloat of a number: the sign of a zero goes away.
	if distance == 0 {
		distance = 0
	}
	if weight == 0 {
		weight = 0
	}
	// checkBoundsTrigger of each call. The bounds of the compound nodes
	// around an edge do not change.
	e.EdgeStyle.SegmentDistances = []float64{distance}
	e.dirtyBoundingBoxCache()
	e.EdgeStyle.SegmentWeights = []float64{weight}
	e.dirtyBoundingBoxCache()
}
