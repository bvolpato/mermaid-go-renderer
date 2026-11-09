// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/orthogonalRouter/router.ts (MIT).

package swimlanes

import (
	"math"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// Orthogonal edge router for the swimlanes layout.
//
// Each edge is routed as an axis-aligned (Manhattan) polyline between ports
// on the node borders, with port distribution and nudging to reduce
// overlaps and crossings. "Raykov" in the comments is the informal name
// under which upstream developed this router.

// routerEPS is EPS of router.ts.
const routerEPS = PRECISION_EPSILON

const (
	routerNodePadding          = 8
	routerHorizontalPipeMargin = 15
	routerVerticalPipeMargin   = 15
	routerRoutingMargin        = 25
	routerAnchorOffset         = 20
	routerTrackSpacing         = 10
)

// routerPipe is Pipe: a line of the routing grid. orientation is
// "horizontal" or "vertical". coord is y for a horizontal pipe and x for a
// vertical pipe.
type routerPipe struct {
	id          string
	orientation string
	coord       float64
	spanMin     float64
	spanMax     float64
	tracks      []*routerTrack
}

// routerTrack is Track: one lane of a pipe.
type routerTrack struct {
	index int
	// coord is the x or y of this track.
	coord float64
	// segments are the references to the segments of this track.
	segments []routerSegmentRef
}

// routerSegmentRef is SegmentRef. from and to are the minimum and the
// maximum coordinate along the axis.
type routerSegmentRef struct {
	edgeIndex    int
	segmentIndex int
	from         float64
	to           float64
}

// routerRoutedSegment is RoutedSegment. The segments are shared objects:
// the passes compare and change them through pointers.
type routerRoutedSegment struct {
	edgeIndex    int
	segmentIndex int
	orientation  string
	pipe         *routerPipe
	trackIndex   int
	from         float64
	to           float64
	// allIndex is the index of the segment in allRoutedSegments. It stands
	// for allRoutedSegments.indexOf(seg): every segment is added once.
	allIndex int
}

// routerRoutedLine is RoutedLine.
type routerRoutedLine struct {
	orient string
	coord  float64
	from   float64
	to     float64
}

func routerChooseOrthogonalSide(node *ru.Node, target Point, fallback string) string {
	cx := ru.NN(node.X, 0)
	cy := ru.NN(node.Y, 0)
	dx := target.X - cx
	dy := target.Y - cy
	absDx := math.Abs(dx)
	absDy := math.Abs(dy)

	if absDx < routerEPS && absDy < routerEPS {
		return fallback
	}

	const verticalBias = 3.0
	if absDy > routerEPS && absDy*verticalBias >= absDx {
		if dy > 0 {
			return "bottom"
		}
		return "top"
	}
	if absDx > routerEPS {
		if dx > 0 {
			return "right"
		}
		return "left"
	}
	return fallback
}

func routerSharedLineEndpointCoord(line, nextLine routerRoutedLine) float64 {
	if math.Abs(line.to-nextLine.from) < routerEPS || math.Abs(line.to-nextLine.to) < routerEPS {
		return line.to
	}
	return line.from
}

func routerPointOnLine(line routerRoutedLine, along float64) Point {
	if line.orient == "vertical" {
		return Point{X: line.coord, Y: along}
	}
	return Point{X: along, Y: line.coord}
}

// routerTrackAt is pipe.tracks[index], or nil when there is no such track.
func routerTrackAt(pipe *routerPipe, index int) *routerTrack {
	if index < 0 || index >= len(pipe.tracks) {
		return nil
	}
	return pipe.tracks[index]
}

// routeEdgesOrthogonal routes every edge of the layout data that is not
// layout-only and writes the polylines to the edges. It removes the
// layout-only edges from the layout data. An empty direction stands for
// undefined.
func routeEdgesOrthogonal(data *ru.LayoutData, direction string) *ru.LayoutData {
	nodes := data.Nodes
	originalEdges := data.Edges

	// A local routing view of the edge list. Layout-only virtual edges exist
	// only for the Sugiyama layering and are never routed. Every other edge,
	// labelled or not, is routed as one unbroken polyline. The labels are
	// anchored on the polylines after the routing (anchorLabelsToPolyline).
	//
	// The view is the shallow copy { ...oe, __originalEdge: oe } of upstream,
	// with the properties that the router reads.
	type InternalRoutingEdge struct {
		id           string
		start        string
		end          string
		points       []Point
		labelNodeId  string
		originalEdge *ru.Edge
	}
	edges := []*InternalRoutingEdge{}
	for _, oe := range originalEdges {
		if oe.IsLayoutOnly {
			continue
		}
		edges = append(edges, &InternalRoutingEdge{
			id:           oe.ID,
			start:        oe.Start,
			end:          oe.End,
			points:       oe.Points,
			labelNodeId:  oe.LabelNodeID,
			originalEdge: oe,
		})
	}

	nodeById := map[string]*ru.Node{}
	// laneByNodeId has the id of the lane (LaneInfo.id) of a node.
	laneByNodeId := map[string]string{}
	pipes := []*routerPipe{}
	isLR := direction == "LR"

	// 1. Initialize helpers and lookups.
	for _, n := range nodes {
		nodeById[n.ID] = n
	}

	// Identify the lanes: the top-level groups.
	for _, group := range nodes {
		if !(group.IsGroup && group.ParentID == "") {
			continue
		}
		lane := group.ID

		// Assign this lane to all descendants. A node that is its own
		// descendant (duplicate ids) recurses without end upstream, which
		// throws a RangeError there.
		visiting := map[*ru.Node]bool{}
		var assignLane func(n *ru.Node)
		assignLane = func(n *ru.Node) {
			if visiting[n] {
				panic(jsError("Maximum call stack size exceeded"))
			}
			visiting[n] = true
			laneByNodeId[n.ID] = lane
			for _, child := range nodes {
				if child.ParentID != "" && child.ParentID == n.ID {
					assignLane(child)
				}
			}
			delete(visiting, n)
		}
		assignLane(group)
	}

	// The obstacle rects of the nodes that are not groups. Edge-label nodes
	// are not obstacles during the routing: the labels are placed on the
	// polylines afterwards.
	type ObstacleRect struct {
		nodeId string
		minX   float64
		minY   float64
		maxX   float64
		maxY   float64
		// For the LR direction the visual extent after the transform counts:
		// the x of TB becomes the y of LR, so it uses the height.
		visualXHalfExtent float64
	}
	obstacles := []ObstacleRect{}
	for _, n := range nodes {
		if n.IsGroup || n.IsEdgeLabel {
			continue
		}
		w := ru.NN(n.Width, 10)
		h := ru.NN(n.Height, 10)
		x := ru.NN(n.X, 0)
		y := ru.NN(n.Y, 0)
		// Inflate by the configured padding.
		const padding = routerNodePadding

		visualXHalfExtent := float64(w/2) + padding
		if isLR {
			visualXHalfExtent = float64(h/2) + padding
		}
		obstacles = append(obstacles, ObstacleRect{
			nodeId:            n.ID,
			minX:              x - float64(w/2) - padding,
			maxX:              x + float64(w/2) + padding,
			minY:              y - float64(h/2) - padding,
			maxY:              y + float64(h/2) + padding,
			visualXHalfExtent: visualXHalfExtent,
		})
	}

	// Find or create a pipe.
	getOrAddPipe := func(orientation string, coord, spanMin, spanMax float64) *routerPipe {
		var pipe *routerPipe
		for _, p := range pipes {
			if p.orientation == orientation && math.Abs(p.coord-coord) < 1 {
				pipe = p
				break
			}
		}
		if pipe == nil {
			pipe = &routerPipe{
				id:          "pipe-" + orientation + "-" + toFixed(coord, 0),
				orientation: orientation,
				coord:       coord,
				spanMin:     spanMin,
				spanMax:     spanMax,
				tracks:      []*routerTrack{},
			}
			pipes = append(pipes, pipe)
		}
		// Extend the span.
		pipe.spanMin = js.Min(pipe.spanMin, spanMin)
		pipe.spanMax = js.Max(pipe.spanMax, spanMax)
		return pipe
	}

	// The port at the center of a side. Step 6.2 uses it so that the main
	// routing loop can honor a side that is not the natural choice of
	// getOrthogonalPort.
	portForSide := func(node *ru.Node, side string) Point {
		w := ru.NN(node.Width, 10)
		h := ru.NN(node.Height, 10)
		cx := ru.NN(node.X, 0)
		cy := ru.NN(node.Y, 0)
		switch side {
		case "top":
			return Point{X: cx, Y: cy - float64(h/2)}
		case "bottom":
			return Point{X: cx, Y: cy + float64(h/2)}
		case "left":
			return Point{X: cx - float64(w/2), Y: cy}
		default: // "right"
			return Point{X: cx + float64(w/2), Y: cy}
		}
	}

	// getOrthogonalPort returns the center of a cardinal side, so that the
	// edge starts or ends with a horizontal or vertical segment. target
	// decides the side. isSource changes the side for nodes in the same row
	// or column.
	getOrthogonalPort := func(node *ru.Node, target Point, isSource bool) Point {
		fallback := "top"
		if isSource {
			fallback = "bottom"
		}
		return portForSide(node, routerChooseOrthogonalSide(node, target, fallback))
	}

	// The list of all routed segments, for the crossing reduction.
	allRoutedSegments := []*routerRoutedSegment{}
	// edgeIndex -> [routedSegmentIndex, ...]
	edgeSegmentIndices := make([][]int, len(edges))
	// Centered straight-line fast-path edges keep their pipe coord.
	straightIntraLaneEdges := map[int]bool{}
	const CROSSING_PENALTY = 1000

	crossingPenalty := func(edgeIdx int, from, to Point) float64 {
		if len(allRoutedSegments) == 0 {
			return 0
		}
		isHorizontal := math.Abs(from.Y-to.Y) < routerEPS
		isVertical := math.Abs(from.X-to.X) < routerEPS
		if !isHorizontal && !isVertical {
			return 0
		}

		penalties := 0.0
		if isHorizontal {
			y := from.Y
			minX := js.Min(from.X, to.X) - routerEPS
			maxX := js.Max(from.X, to.X) + routerEPS
			if maxX <= minX {
				return 0
			}
			for _, seg := range allRoutedSegments {
				if seg.edgeIndex == edgeIdx || seg.orientation != "vertical" {
					continue
				}
				if seg.pipe.coord < minX || seg.pipe.coord > maxX {
					continue
				}
				if seg.from-routerEPS <= y && seg.to+routerEPS >= y {
					penalties += CROSSING_PENALTY
				}
			}
		} else if isVertical {
			x := from.X
			minY := js.Min(from.Y, to.Y) - routerEPS
			maxY := js.Max(from.Y, to.Y) + routerEPS
			if maxY <= minY {
				return 0
			}
			for _, seg := range allRoutedSegments {
				if seg.edgeIndex == edgeIdx || seg.orientation != "horizontal" {
					continue
				}
				if seg.pipe.coord < minY || seg.pipe.coord > maxY {
					continue
				}
				if seg.from-routerEPS <= x && seg.to+routerEPS >= x {
					penalties += CROSSING_PENALTY
				}
			}
		}
		return penalties
	}

	// -----------------------------------------------------------------------
	// Phase 1: Initial Routing
	// -----------------------------------------------------------------------
	type routingEntry struct {
		idx       int
		crossLane float64
		dx        float64
		dy        float64
	}
	routingEntries := make([]routingEntry, 0, len(edges))
	for idx, edge := range edges {
		if edge.start == "" || edge.end == "" {
			routingEntries = append(routingEntries, routingEntry{idx: idx, crossLane: 0, dx: 0, dy: 0})
			continue
		}
		srcNode := nodeById[edge.start]
		dstNode := nodeById[edge.end]
		srcLane, hasSrcLane := laneByNodeId[edge.start]
		dstLane, hasDstLane := laneByNodeId[edge.end]
		crossLane := 0.0
		if hasSrcLane && hasDstLane && srcLane != dstLane {
			crossLane = 1
		}
		dx := 0.0
		dy := 0.0
		if srcNode != nil && dstNode != nil {
			dx = math.Abs(ru.NN(dstNode.X, 0) - ru.NN(srcNode.X, 0))
			dy = math.Abs(ru.NN(dstNode.Y, 0) - ru.NN(srcNode.Y, 0))
		}
		routingEntries = append(routingEntries, routingEntry{idx: idx, crossLane: crossLane, dx: dx, dy: dy})
	}
	sortFunc(routingEntries, func(a, b routingEntry) float64 {
		// Route the cross-lane edges first, so that they claim their preferred
		// straight path before flexible intra-lane detours can block them. The
		// intra-lane edges then adapt through the crossing penalty.
		if a.crossLane != b.crossLane {
			return b.crossLane - a.crossLane
		}
		// Shorter edges first: they need less room to maneuver.
		aDist := a.dx + a.dy
		bDist := b.dx + b.dy
		if math.Abs(aDist-bDist) > 1 {
			return aDist - bDist
		}
		return float64(a.idx - b.idx)
	})
	routingOrder := make([]int, len(routingEntries))
	for k, entry := range routingEntries {
		routingOrder[k] = entry.idx
	}

	// isSegmentBlocked says whether an obstacle blocks a segment.
	// excludeStart and excludeEnd are node ids whose obstacles do not count.
	isSegmentBlocked := func(p1, p2 Point, excludeStart, excludeEnd string) bool {
		segMinX := js.Min(p1.X, p2.X)
		segMaxX := js.Max(p1.X, p2.X)
		segMinY := js.Min(p1.Y, p2.Y)
		segMaxY := js.Max(p1.Y, p2.Y)

		for _, obs := range obstacles {
			if excludeStart != "" && obs.nodeId == excludeStart {
				continue
			}
			if excludeEnd != "" && obs.nodeId == excludeEnd {
				continue
			}
			var blocks bool
			if math.Abs(p1.X-p2.X) > routerEPS {
				// Horizontal segment
				blocks = obs.minY < p1.Y && obs.maxY > p1.Y && obs.maxX > segMinX && obs.minX < segMaxX
			} else {
				// Vertical segment
				blocks = obs.minX < p1.X && obs.maxX > p1.X && obs.maxY > segMinY && obs.minY < segMaxY
			}
			if blocks {
				return true
			}
		}

		return false
	}

	// ---- Step 6: Port pre-assignment ----
	// When several edges connect to the same side of a node, their ports are
	// spread over that side and ordered by the coordinate of the other end,
	// so that the edges do not cross at the node boundary.
	//
	// Key: "nodeId:side:role". side is top, bottom, left or right. role is
	// "src" (the edge leaves this node) or "dst" (the edge arrives here).
	// Value: the list of { edgeIdx, oppositeCoord }, sorted by oppositeCoord.
	type portGroupEntry struct {
		edgeIdx       int
		oppositeCoord float64
	}
	portGroups := js.NewMap[string, []portGroupEntry]()
	incidentEdgeTotals := map[string]int{}
	for _, edge := range edges {
		if edge.start == "" || edge.end == "" || edge.start == edge.end {
			continue
		}
		incidentEdgeTotals[edge.start]++
		incidentEdgeTotals[edge.end]++
	}

	// First pass: the side on which each edge connects to each node.
	determineSide := func(node *ru.Node, target Point) string {
		return routerChooseOrthogonalSide(node, target, "bottom")
	}

	// ----- Step 6.1: compute the initial sides for every edge -----
	//
	// determineSide runs first for both ends of every edge, so that the
	// sibling side-splitting pass (6.2) sees the full picture before the
	// port-group build (6.3) locks each edge into a side.
	type EdgeSideInfo struct {
		edgeIdx int
		srcId   string
		dstId   string
		srcSide string
		dstSide string
		absDx   float64
		absDy   float64
		dxSign  float64
		dySign  float64
	}
	sideInfoByIdx := js.NewMap[int, *EdgeSideInfo]()
	for i, e := range edges {
		if e.start == "" || e.end == "" || e.start == e.end {
			continue
		}
		if len(e.points) > 0 {
			continue
		}
		src := nodeById[e.start]
		dst := nodeById[e.end]
		if src == nil || dst == nil {
			continue
		}
		dx := ru.NN(dst.X, 0) - ru.NN(src.X, 0)
		dy := ru.NN(dst.Y, 0) - ru.NN(src.Y, 0)
		sideInfoByIdx.Set(i, &EdgeSideInfo{
			edgeIdx: i,
			srcId:   e.start,
			dstId:   e.end,
			srcSide: determineSide(src, Point{X: ru.NN(dst.X, 0), Y: ru.NN(dst.Y, 0)}),
			dstSide: determineSide(dst, Point{X: ru.NN(src.X, 0), Y: ru.NN(src.Y, 0)}),
			absDx:   math.Abs(dx),
			absDy:   math.Abs(dy),
			dxSign:  js.Sign(dx),
			dySign:  js.Sign(dy),
		})
	}

	// ----- Step 6.2: sibling side-splitting -----
	//
	// When two or more edges leave a source node from the same side, the
	// ones with the weaker preference move to their secondary side. This
	// spreads the edges over several sides of the node.
	//
	// This pass must run before the port-group build (6.3) and the anchor
	// computation: a side that changes later would corrupt the port groups.
	//
	// The preference strength is the dy/dx ratio for an edge that prefers a
	// vertical side, and the dx/dy ratio otherwise. A higher ratio is a
	// stronger preference for the current side.
	preferenceStrength := func(info *EdgeSideInfo) float64 {
		if info.srcSide == "top" || info.srcSide == "bottom" {
			if info.absDx == 0 {
				return math.Inf(1)
			}
			return info.absDy / info.absDx
		}
		if info.absDy == 0 {
			return math.Inf(1)
		}
		return info.absDx / info.absDy
	}
	secondarySide := func(info *EdgeSideInfo) string {
		if info.srcSide == "top" || info.srcSide == "bottom" {
			if info.dxSign >= 0 {
				return "right"
			}
			return "left"
		}
		if info.dySign >= 0 {
			return "bottom"
		}
		return "top"
	}

	// Group by (src, srcSide) to find the sides that two or more edges share.
	sourceSideGroups := js.NewMap[string, []*EdgeSideInfo]()
	for _, info := range sideInfoByIdx.Values() {
		key := info.srcId + ":" + info.srcSide
		sourceSideGroups.Set(key, append(sourceSideGroups.Get(key), info))
	}

	// The load of every (nodeId, side): the number of edges committed to
	// that side as source or as destination.
	sideLoad := map[string]int{}
	loadKey := func(nodeId string, side string) string { return nodeId + ":" + side }
	for _, info := range sideInfoByIdx.Values() {
		sideLoad[loadKey(info.srcId, info.srcSide)]++
		sideLoad[loadKey(info.dstId, info.dstSide)]++
	}

	for _, group := range sourceSideGroups.Values() {
		if len(group) < 2 {
			continue
		}
		// Sort by preference strength, strongest first. The strongest sibling
		// keeps its side. The edge index breaks ties.
		sortFunc(group, func(a, b *EdgeSideInfo) float64 {
			sa := preferenceStrength(a)
			sb := preferenceStrength(b)
			if math.Abs(sa-sb) > 1e-9 {
				return sb - sa
			}
			return float64(a.edgeIdx - b.edgeIdx)
		})
		// Each sibling but the first is considered for its secondary side.
		for g := 1; g < len(group); g++ {
			info := group[g]
			secondary := secondarySide(info)
			primaryLoad := sideLoad[loadKey(info.srcId, info.srcSide)]
			secondaryLoad := sideLoad[loadKey(info.srcId, secondary)]
			// Move only when the secondary side has strictly less load than
			// the primary side.
			if secondaryLoad >= primaryLoad {
				continue
			}
			// Update the counters and the side.
			sideLoad[loadKey(info.srcId, info.srcSide)] = primaryLoad - 1
			sideLoad[loadKey(info.srcId, secondary)] = secondaryLoad + 1
			info.srcSide = secondary
		}
	}

	// ----- Step 6.2b: bimodal in/out de-collision for diamond nodes -----
	//
	// The incoming and outgoing edges of a node should use distinct sides.
	// Step 6.2 balances only siblings with the same role, so an out-edge can
	// still be on the side of an in-edge. On a diamond each side is a single
	// pin at the vertex, so both edges would get the same pin. The out-edge
	// moves to its free secondary side. Only diamonds are changed.
	isDiamondNode := func(node *ru.Node) bool {
		if node == nil {
			return false
		}
		shape := node.Shape
		return shape == "question" || shape == "diamond"
	}
	inSidesByNode := map[string]map[string]bool{}
	for _, info := range sideInfoByIdx.Values() {
		if _, ok := inSidesByNode[info.dstId]; !ok {
			inSidesByNode[info.dstId] = map[string]bool{}
		}
		inSidesByNode[info.dstId][info.dstSide] = true
	}
	for _, info := range sideInfoByIdx.Values() {
		if !isDiamondNode(nodeById[info.srcId]) {
			continue
		}
		inSides, ok := inSidesByNode[info.srcId]
		if !ok || !inSides[info.srcSide] {
			continue // no in-edge shares the side of this out-edge
		}
		secondary := secondarySide(info)
		// Move only to a side that no in-edge uses and that has no load.
		if inSides[secondary] || sideLoad[loadKey(info.srcId, secondary)] > 0 {
			continue
		}
		primaryLoad := sideLoad[loadKey(info.srcId, info.srcSide)]
		if primaryLoad-1 > 0 {
			sideLoad[loadKey(info.srcId, info.srcSide)] = primaryLoad - 1
		} else {
			sideLoad[loadKey(info.srcId, info.srcSide)] = 0
		}
		sideLoad[loadKey(info.srcId, secondary)] = 1
		info.srcSide = secondary
	}

	// ----- Step 6.3: port-group build (with the sides of 6.2) -----
	for _, info := range sideInfoByIdx.Values() {
		i, srcId, dstId, srcSide, dstSide := info.edgeIdx, info.srcId, info.dstId, info.srcSide, info.dstSide
		src := nodeById[srcId]
		dst := nodeById[dstId]

		srcKey := srcId + ":" + srcSide + ":src"
		dstCoord := ru.NN(dst.Y, 0)
		if srcSide == "top" || srcSide == "bottom" {
			dstCoord = ru.NN(dst.X, 0)
		}
		portGroups.Set(srcKey, append(portGroups.Get(srcKey), portGroupEntry{edgeIdx: i, oppositeCoord: dstCoord}))

		dstKey := dstId + ":" + dstSide + ":dst"
		srcCoord := ru.NN(src.Y, 0)
		if dstSide == "top" || dstSide == "bottom" {
			srcCoord = ru.NN(src.X, 0)
		}
		portGroups.Set(dstKey, append(portGroups.Get(dstKey), portGroupEntry{edgeIdx: i, oppositeCoord: srcCoord}))
	}

	// The port offsets of each group with two or more edges.
	// Key: "edgeIdx:src" or "edgeIdx:dst". Value: the offset from the center.
	portOffsets := map[string]float64{}
	const MIN_PORT_SPACING = 8 // minimum pixels between adjacent ports

	for _, key := range portGroups.Keys() {
		group := portGroups.Get(key)
		if len(group) < 2 {
			continue
		}
		// Sort by the opposite coordinate, ascending. The sort is in place:
		// the list of the Map shares its elements with group.
		sortFunc(group, func(a, b portGroupEntry) float64 { return a.oppositeCoord - b.oppositeCoord })

		// Parse the node id and the side from the key.
		parts := strings.Split(key, ":")
		nodeId := strings.Join(parts[:len(parts)-2], ":") // handle ids with colons
		side := parts[len(parts)-2]
		role := parts[len(parts)-1] // "src" or "dst"
		node := nodeById[nodeId]
		if node == nil {
			continue
		}

		// The span along the side. On the pointed sides of a diamond the port
		// area is much smaller than the bounding box: a fraction of the side.
		isVerticalSide := side == "left" || side == "right"
		sideLength := ru.NN(node.Width, 10)
		if isVerticalSide {
			sideLength = ru.NN(node.Height, 10)
		}
		shape := node.Shape
		isDiamond := shape == "question" || shape == "diamond"
		effectiveLength := sideLength
		if isDiamond {
			effectiveLength = sideLength * 0.3
		}
		const MAX_PORT_SPACING = 20 // cap the spacing to avoid large detours

		// Spread the ports evenly, clamped by MIN and MAX.
		spacing := js.Min(
			MAX_PORT_SPACING,
			js.Max(MIN_PORT_SPACING, effectiveLength/float64(len(group)+1)),
		)
		totalSpan := float64(spacing * float64(len(group)-1))
		startOffset := float64(-totalSpan / 2)

		for j, element := range group {
			offset := startOffset + float64(float64(j)*spacing)
			offsetKey := strconv.Itoa(element.edgeIdx) + ":" + role
			portOffsets[offsetKey] = offset
		}
	}

	edgeHasLabelNode := func(edgeIdx int) bool {
		if edgeIdx < 0 || edgeIdx >= len(edges) {
			return false
		}
		return edges[edgeIdx].labelNodeId != ""
	}

	faceHasLabelNode := func(nodeId string, side string) bool {
		if nodeId == "" {
			return false
		}
		for _, entry := range portGroups.Get(nodeId + ":" + side + ":src") {
			if edgeHasLabelNode(entry.edgeIdx) {
				return true
			}
		}
		for _, entry := range portGroups.Get(nodeId + ":" + side + ":dst") {
			if edgeHasLabelNode(entry.edgeIdx) {
				return true
			}
		}
		return false
	}

	// Apply a port offset to a center port.
	applyPortOffset := func(basePort Point, side string, offset float64) Point {
		if side == "top" || side == "bottom" {
			// Offset along the X axis
			return Point{X: basePort.X + offset, Y: basePort.Y}
		}
		// Offset along the Y axis
		return Point{X: basePort.X, Y: basePort.Y + offset}
	}

	portsForEdge := func(edgeIndex int, src, dst *ru.Node) (pSrcPort, pDstPort Point, srcSide, dstSide string) {
		sideInfo, hasSideInfo := sideInfoByIdx.Lookup(edgeIndex)
		srcTarget := Point{X: ru.NN(dst.X, 0), Y: ru.NN(dst.Y, 0)}
		dstTarget := Point{X: ru.NN(src.X, 0), Y: ru.NN(src.Y, 0)}
		if hasSideInfo {
			srcSide = sideInfo.srcSide
			dstSide = sideInfo.dstSide
			pSrcPort = portForSide(src, sideInfo.srcSide)
			pDstPort = portForSide(dst, sideInfo.dstSide)
		} else {
			srcSide = determineSide(src, srcTarget)
			dstSide = determineSide(dst, dstTarget)
			pSrcPort = getOrthogonalPort(src, srcTarget, true)
			pDstPort = getOrthogonalPort(dst, dstTarget, false)
		}

		if srcOffset, ok := portOffsets[strconv.Itoa(edgeIndex)+":src"]; ok {
			pSrcPort = applyPortOffset(pSrcPort, srcSide, srcOffset)
		}
		if dstOffset, ok := portOffsets[strconv.Itoa(edgeIndex)+":dst"]; ok {
			pDstPort = applyPortOffset(pDstPort, dstSide, dstOffset)
		}
		return pSrcPort, pDstPort, srcSide, dstSide
	}

	for _, i := range routingOrder {
		e := edges[i]
		edgeSegmentIndices[i] = []int{}

		if e.start == "" || e.end == "" {
			continue
		}
		if len(e.points) > 0 {
			continue
		}
		// Skip self-loops
		if e.start == e.end {
			continue
		}

		src := nodeById[e.start]
		dst := nodeById[e.end]
		if src == nil || dst == nil {
			continue
		}

		// 2. Compute the ports, with the side assignment of Step 6.2.
		pSrcPort, pDstPort, srcPortSide, dstPortSide := portsForEdge(i, src, dst)

		// 3. Compute the anchors. An anchor extends from its port in the
		// direction of the port: below a port on the bottom side, and so on.
		pSrcAnchor := pSrcPort
		pDstAnchor := pDstPort

		// Whether the ports are on a vertical axis (top or bottom) or on a
		// horizontal axis (left or right).
		srcPortIsVertical := srcPortSide == "top" || srcPortSide == "bottom"
		dstPortIsVertical := dstPortSide == "top" || dstPortSide == "bottom"

		// Source anchor
		if srcPortIsVertical {
			isBottom := pSrcPort.Y > ru.NN(src.Y, 0)
			if isBottom {
				pSrcAnchor.Y = pSrcPort.Y + routerAnchorOffset
			} else {
				pSrcAnchor.Y = pSrcPort.Y - routerAnchorOffset
			}
		} else {
			isRight := pSrcPort.X > ru.NN(src.X, 0)
			if isRight {
				pSrcAnchor.X = pSrcPort.X + routerAnchorOffset
			} else {
				pSrcAnchor.X = pSrcPort.X - routerAnchorOffset
			}
		}

		// Target anchor
		if dstPortIsVertical {
			isBottom := pDstPort.Y > ru.NN(dst.Y, 0)
			if isBottom {
				pDstAnchor.Y = pDstPort.Y + routerAnchorOffset
			} else {
				pDstAnchor.Y = pDstPort.Y - routerAnchorOffset
			}
		} else {
			isRight := pDstPort.X > ru.NN(dst.X, 0)
			if isRight {
				pDstAnchor.X = pDstPort.X + routerAnchorOffset
			} else {
				pDstAnchor.X = pDstPort.X - routerAnchorOffset
			}
		}

		// isPointInObstacle says whether a point is inside an obstacle, apart
		// from the obstacles of the given nodes.
		isPointInObstacle := func(pt Point, excludeNodeIds []string) (inside bool, obstacle ObstacleRect) {
			for _, obs := range obstacles {
				if containsString(excludeNodeIds, obs.nodeId) {
					continue
				}
				if pt.X > obs.minX && pt.X < obs.maxX && pt.Y > obs.minY && pt.Y < obs.maxY {
					return true, obs
				}
			}
			return false, ObstacleRect{}
		}

		obstacleDetour := func(port Point, node, opposite *ru.Node, obs ObstacleRect, portIsVertical bool) (x, y float64, leavesPositiveSide bool) {
			if portIsVertical {
				leavesPositiveSide = port.Y > ru.NN(node.Y, 0)
				goRight := ru.NN(opposite.X, 0) >= port.X
				x = obs.minX - routerHorizontalPipeMargin
				if goRight {
					x = obs.maxX + routerHorizontalPipeMargin
				}
				y = obs.minY - routerVerticalPipeMargin
				if leavesPositiveSide {
					y = obs.maxY + routerVerticalPipeMargin
				}
				return x, y, leavesPositiveSide
			}

			leavesPositiveSide = port.X > ru.NN(node.X, 0)
			goDown := ru.NN(opposite.Y, 0) >= port.Y
			x = obs.minX - routerHorizontalPipeMargin
			if leavesPositiveSide {
				x = obs.maxX + routerHorizontalPipeMargin
			}
			y = obs.minY - routerVerticalPipeMargin
			if goDown {
				y = obs.maxY + routerVerticalPipeMargin
			}
			return x, y, leavesPositiveSide
		}

		// Push the source anchor out when it is inside an obstacle. This
		// happens when the node below the source is too close. Waypoints then
		// route around the obstacle.
		//
		// The first point after the port must be in the orthogonal direction
		// (the same x for a vertical port, the same y for a horizontal port):
		// insertEdge intersects the node with the first inner point.
		srcHandleWaypoints := []Point{}
		endpointIds := []string{e.start, e.end}
		if srcInside, obs := isPointInObstacle(pSrcAnchor, endpointIds); srcInside {
			if srcPortIsVertical {
				// Vertical port (top or bottom): route around the obstacle
				// horizontally.
				detourX, detourY, leavesPositiveSide := obstacleDetour(pSrcPort, src, dst, obs, true)

				pSrcAnchor.X = detourX
				pSrcAnchor.Y = detourY

				// A small orthogonal step in the gap between the source node
				// and the obstacle.
				var gapY float64
				if leavesPositiveSide {
					gapY = js.Min(obs.minY-2, pSrcPort.Y+routerAnchorOffset) // just before the obstacle
				} else {
					gapY = js.Max(obs.maxY+2, pSrcPort.Y-routerAnchorOffset) // just after the obstacle
				}

				srcHandleWaypoints = []Point{
					{X: pSrcPort.X, Y: gapY}, // orthogonal step in the gap
					{X: detourX, Y: gapY},    // horizontal detour
					{X: detourX, Y: detourY}, // down past the obstacle
				}
			} else {
				// Horizontal port (left or right): route around vertically.
				detourX, detourY, leavesPositiveSide := obstacleDetour(pSrcPort, src, dst, obs, false)

				var gapX float64
				if leavesPositiveSide {
					gapX = js.Min(obs.minX-2, pSrcPort.X+routerAnchorOffset)
				} else {
					gapX = js.Max(obs.maxX+2, pSrcPort.X-routerAnchorOffset)
				}

				pSrcAnchor.X = detourX
				pSrcAnchor.Y = detourY

				srcHandleWaypoints = []Point{
					{X: gapX, Y: pSrcPort.Y}, // orthogonal step in the gap
					{X: gapX, Y: detourY},    // vertical detour
					{X: detourX, Y: detourY}, // horizontal past the obstacle
				}
			}
		}

		// Push the destination anchor out when it is inside an obstacle, with
		// the same logic as for the source.
		dstHandleWaypoints := []Point{}
		if dstInside, obs := isPointInObstacle(pDstAnchor, endpointIds); dstInside {
			if dstPortIsVertical {
				detourX, detourY, _ := obstacleDetour(pDstPort, dst, src, obs, true)

				pDstAnchor.X = detourX
				pDstAnchor.Y = detourY

				// From the anchor, sideways, then orthogonally to the port. The
				// last waypoint before the port has the x of the port.
				dstHandleWaypoints = []Point{
					{X: detourX, Y: detourY},    // from the anchor position
					{X: pDstPort.X, Y: detourY}, // sideways to the x of the port
				}
			} else {
				detourX, detourY, _ := obstacleDetour(pDstPort, dst, src, obs, false)

				pDstAnchor.X = detourX
				pDstAnchor.Y = detourY

				dstHandleWaypoints = []Point{
					{X: detourX, Y: detourY},    // from the anchor position
					{X: detourX, Y: pDstPort.Y}, // vertically to the y of the port
				}
			}
		}

		// ----- Centered straight-line fast path -----
		//
		// When the two port sides face each other and the segment from anchor
		// to anchor is already axis-aligned, the straight polyline from port
		// to port is emitted directly if no foreign obstacle blocks it.
		//
		// Conditions:
		//   1. No obstacle-avoidance waypoints were added.
		//   2. The anchors share one coordinate within the pipe margin.
		//   3. The edge is not in a distributed port group (a port offset
		//      would move the port off the face center).
		//   4. The endpoint faces are uncontested, or every contested endpoint
		//      is a degree-2 chain node.
		//   5. The direct segment from port to port is free of obstacles.
		//
		// The points are then set directly and the rest of the loop body is
		// skipped. Phases 2 and 3 skip edges without segment indices, so the
		// straight polyline reaches the output unchanged.
		if len(srcHandleWaypoints) == 0 && len(dstHandleWaypoints) == 0 {
			const hpMargin = routerHorizontalPipeMargin
			anchorsSameX := math.Abs(pSrcAnchor.X-pDstAnchor.X) < hpMargin
			anchorsSameY := math.Abs(pSrcAnchor.Y-pDstAnchor.Y) < hpMargin
			_, hasSrcOffset := portOffsets[strconv.Itoa(i)+":src"]
			_, hasDstOffset := portOffsets[strconv.Itoa(i)+":dst"]
			hasPortOffset := hasSrcOffset || hasDstOffset
			// The number of edges (of any role) on each facing side. More than
			// one means that the face is contested.
			srcFaceTotal := len(portGroups.Get(e.start+":"+srcPortSide+":src")) +
				len(portGroups.Get(e.start+":"+srcPortSide+":dst"))
			dstFaceTotal := len(portGroups.Get(e.end+":"+dstPortSide+":src")) +
				len(portGroups.Get(e.end+":"+dstPortSide+":dst"))
			faceContested := srcFaceTotal > 1 || dstFaceTotal > 1
			srcIncidentTotal := incidentEdgeTotals[e.start]
			dstIncidentTotal := incidentEdgeTotals[e.end]
			contestedFaceHasLabel := (srcFaceTotal > 1 && faceHasLabelNode(e.start, srcPortSide)) ||
				(dstFaceTotal > 1 && faceHasLabelNode(e.end, dstPortSide))
			srcContestAllowsCenteredStraight := srcFaceTotal <= 1 || srcIncidentTotal <= 2
			dstContestAllowsCenteredStraight := dstFaceTotal <= 1 || dstIncidentTotal <= 2
			canPreserveSimpleContestedStraight := faceContested &&
				!contestedFaceHasLabel &&
				srcContestAllowsCenteredStraight &&
				dstContestAllowsCenteredStraight
			if (anchorsSameX || anchorsSameY) &&
				!hasPortOffset &&
				(!faceContested || canPreserveSimpleContestedStraight) {
				directBlocked := isSegmentBlocked(pSrcPort, pDstPort, e.start, e.end)
				if !directBlocked {
					// Emit the shape port, anchor, anchor, port. The four points
					// are collinear, so the post-processing collapses them to a
					// straight line of two points.
					e.points = []Point{pSrcPort, pSrcAnchor, pDstAnchor, pDstPort}
					straightIntraLaneEdges[i] = true
					// Register the line from port to port as a routed segment,
					// so that later A* searches see it through crossingPenalty.
					// The index is not added to edgeSegmentIndices[i]: the
					// track assignment of phases 2 and 3 must keep skipping
					// fast-path edges.
					fastPathOrientation := "vertical"
					fastPathCoord := pSrcPort.X
					fastPathFrom := js.Min(pSrcPort.Y, pDstPort.Y)
					fastPathTo := js.Max(pSrcPort.Y, pDstPort.Y)
					if anchorsSameY {
						fastPathOrientation = "horizontal"
						fastPathCoord = pSrcPort.Y
						fastPathFrom = js.Min(pSrcPort.X, pDstPort.X)
						fastPathTo = js.Max(pSrcPort.X, pDstPort.X)
					}
					fastPathPipe := &routerPipe{
						id:          "fast-path-" + fastPathOrientation + "-" + toFixed(fastPathCoord, 0) + "-" + strconv.Itoa(i),
						orientation: fastPathOrientation,
						coord:       fastPathCoord,
						spanMin:     fastPathFrom,
						spanMax:     fastPathTo,
						tracks:      []*routerTrack{},
					}
					allRoutedSegments = append(allRoutedSegments, &routerRoutedSegment{
						edgeIndex:    i,
						segmentIndex: 0,
						orientation:  fastPathOrientation,
						pipe:         fastPathPipe,
						trackIndex:   0,
						from:         fastPathFrom,
						to:           fastPathTo,
						allIndex:     len(allRoutedSegments),
					})
					continue
				}
			}
		}

		// Snap the anchors to the nearest pipe (created lazily).
		srcPipe := getOrAddPipe("vertical", pSrcAnchor.X, pSrcAnchor.Y, pSrcAnchor.Y)
		pSrcAnchor.X = srcPipe.coord
		dstPipe := getOrAddPipe("vertical", pDstAnchor.X, pDstAnchor.Y, pDstAnchor.Y)
		pDstAnchor.X = dstPipe.coord

		// 4. Build the visibility graph and find a path.
		// The bounding box starts with the anchor points.
		bbMinX := js.Min(pSrcAnchor.X, pDstAnchor.X) - 50
		bbMaxX := js.Max(pSrcAnchor.X, pDstAnchor.X) + 50
		bbMinY := js.Min(pSrcAnchor.Y, pDstAnchor.Y) - 50
		bbMaxY := js.Max(pSrcAnchor.Y, pDstAnchor.Y) + 50

		// Expand the bounding box so that it has room for detours around the
		// obstacles that block the direct path.
		for _, obs := range obstacles {
			// Whether the obstacle overlaps the corridor of the direct path.
			pathMinX := js.Min(pSrcAnchor.X, pDstAnchor.X)
			pathMaxX := js.Max(pSrcAnchor.X, pDstAnchor.X)
			pathMinY := js.Min(pSrcAnchor.Y, pDstAnchor.Y)
			pathMaxY := js.Max(pSrcAnchor.Y, pDstAnchor.Y)

			obsBlocksPath := obs.minX < pathMaxX && obs.maxX > pathMinX && obs.minY < pathMaxY && obs.maxY > pathMinY

			if obsBlocksPath {
				bbMinX = js.Min(bbMinX, obs.minX-routerRoutingMargin)
				bbMaxX = js.Max(bbMaxX, obs.maxX+routerRoutingMargin)
				bbMinY = js.Min(bbMinY, obs.minY-routerRoutingMargin)
				bbMaxY = js.Max(bbMaxY, obs.maxY+routerRoutingMargin)
			}
		}

		// Add the pipe grid lines around the obstacles.
		for _, obs := range obstacles {
			// Whether the obstacle is relevant to the bounding box of this edge.
			if obs.maxX < bbMinX || obs.minX > bbMaxX || obs.maxY < bbMinY || obs.minY > bbMaxY {
				continue
			}
			// Horizontal pipes around the obstacle, only at the safe positions
			// with a margin. A pipe on the exact boundary would let edges hug
			// the nodes.
			const hMargin = routerHorizontalPipeMargin
			getOrAddPipe("horizontal", obs.minY-hMargin, bbMinX, bbMaxX) // above the obstacle
			getOrAddPipe("horizontal", obs.maxY+hMargin, bbMinX, bbMaxX) // below the obstacle

			// Vertical pipes around the obstacle, with a margin.
			const vMargin = routerVerticalPipeMargin
			getOrAddPipe("vertical", obs.minX-vMargin, bbMinY, bbMaxY) // left of the obstacle
			getOrAddPipe("vertical", obs.maxX+vMargin, bbMinY, bbMaxY) // right of the obstacle
		}

		// The horizontal pipes of the start and the end must exist.
		getOrAddPipe("horizontal", pSrcAnchor.Y, bbMinX, bbMaxX)
		getOrAddPipe("horizontal", pDstAnchor.Y, bbMinX, bbMaxX)

		// Collect the relevant pipes.
		hPipes := []*routerPipe{}
		vPipes := []*routerPipe{}
		for _, p := range pipes {
			if p.orientation == "horizontal" && p.coord >= bbMinY && p.coord <= bbMaxY {
				hPipes = append(hPipes, p)
			}
		}
		for _, p := range pipes {
			if p.orientation == "vertical" && p.coord >= bbMinX && p.coord <= bbMaxX {
				vPipes = append(vPipes, p)
			}
		}

		// The vertices are all intersections of hPipes and vPipes. A* runs on
		// these vertices.

		getKey := func(x, y float64) string { return toFixed(x, 1) + "," + toFixed(y, 1) }
		startKey := getKey(pSrcAnchor.X, pSrcAnchor.Y)
		endKey := getKey(pDstAnchor.X, pDstAnchor.Y)

		// A* data structures
		gScore := map[string]float64{}
		cameFrom := map[string]Point{}
		// The direction in which each vertex was reached: 'h' horizontal,
		// 'v' vertical, 'n' none (the start).
		arrivalDir := map[string]byte{}
		openSet := map[string]bool{}
		type openEntry struct {
			key string
			f   float64
			pt  Point
		}
		openList := []openEntry{}

		gScore[startKey] = 0
		arrivalDir[startKey] = 'n' // the start has no arrival direction
		openList = append(openList, openEntry{
			key: startKey,
			f:   js.Hypot(pDstAnchor.X-pSrcAnchor.X, pDstAnchor.Y-pSrcAnchor.Y),
			pt:  pSrcAnchor,
		})
		openSet[startKey] = true

		foundPath := []Point{}

		// Whether a segment is blocked for this edge (src and dst excluded).
		checkSegmentBlocked := func(p1, p2 Point) bool {
			return isSegmentBlocked(p1, p2, e.start, e.end)
		}

		// Try the direct L-shaped paths first (much simpler than A*).
		// Option 1: horizontal first, then vertical.
		cornerHV := Point{X: pDstAnchor.X, Y: pSrcAnchor.Y}
		seg1HV_blocked := checkSegmentBlocked(pSrcAnchor, cornerHV)
		seg2HV_blocked := checkSegmentBlocked(cornerHV, pDstAnchor)
		pathHV_blocked := seg1HV_blocked || seg2HV_blocked

		// Option 2: vertical first, then horizontal.
		cornerVH := Point{X: pSrcAnchor.X, Y: pDstAnchor.Y}
		seg1VH_blocked := checkSegmentBlocked(pSrcAnchor, cornerVH)
		seg2VH_blocked := checkSegmentBlocked(cornerVH, pDstAnchor)
		pathVH_blocked := seg1VH_blocked || seg2VH_blocked

		if !pathHV_blocked {
			// Use the horizontal-first L-path.
			if math.Abs(pSrcAnchor.Y-pDstAnchor.Y) < routerEPS ||
				math.Abs(pSrcAnchor.X-pDstAnchor.X) < routerEPS {
				// Same y or same x: a straight line (the corner would be a
				// duplicate point).
				foundPath = []Point{pSrcAnchor, pDstAnchor}
			} else {
				foundPath = []Point{pSrcAnchor, cornerHV, pDstAnchor}
			}
		} else if !pathVH_blocked {
			// Use the vertical-first L-path.
			if math.Abs(pSrcAnchor.X-pDstAnchor.X) < routerEPS {
				// Same x: a straight vertical line.
				foundPath = []Point{pSrcAnchor, pDstAnchor}
			} else {
				foundPath = []Point{pSrcAnchor, cornerVH, pDstAnchor}
			}
		}

		// Without a direct path, use A*.
		if len(foundPath) == 0 {
			for len(openList) > 0 {
				sortFunc(openList, func(a, b openEntry) float64 { return a.f - b.f })
				current := openList[0]
				openList = openList[1:]
				delete(openSet, current.key)

				if current.key == endKey {
					// Reconstruct the path. A chain of cameFrom entries cannot
					// be longer than the Map, so the walk stops there: a cycle
					// would not end upstream.
					currKey := endKey
					foundPath = []Point{pDstAnchor}
					for steps := 0; steps <= len(cameFrom); steps++ {
						prev, ok := cameFrom[currKey]
						if !ok {
							break
						}
						foundPath = append([]Point{prev}, foundPath...)
						currKey = getKey(prev.X, prev.Y)
					}
					break
				}

				// Neighbors: move along the current horizontal pipe or the
				// current vertical pipe.
				cx := current.pt.X
				cy := current.pt.Y

				// Find the vPipes next to cx. The sort is in place.
				sortFunc(vPipes, func(a, b *routerPipe) float64 { return a.coord - b.coord })
				sortedVPipes := vPipes
				vIdx := -1
				for k, p := range sortedVPipes {
					if math.Abs(p.coord-cx) < 1 {
						vIdx = k
						break
					}
				}

				sortFunc(hPipes, func(a, b *routerPipe) float64 { return a.coord - b.coord })
				hPipesSorted := hPipes
				hIdx := -1
				for k, p := range hPipesSorted {
					if math.Abs(p.coord-cy) < 1 {
						hIdx = k
						break
					}
				}

				neighbors := []Point{}

				// The horizontal neighbors (along the hPipe at cy)
				if vIdx > 0 {
					neighbors = append(neighbors, Point{X: sortedVPipes[vIdx-1].coord, Y: cy})
				}
				if vIdx >= 0 && vIdx < len(sortedVPipes)-1 {
					neighbors = append(neighbors, Point{X: sortedVPipes[vIdx+1].coord, Y: cy})
				}

				// The vertical neighbors (along the vPipe at cx)
				if hIdx > 0 {
					neighbors = append(neighbors, Point{X: cx, Y: hPipesSorted[hIdx-1].coord})
				}
				if hIdx >= 0 && hIdx < len(hPipesSorted)-1 {
					neighbors = append(neighbors, Point{X: cx, Y: hPipesSorted[hIdx+1].coord})
				}

				for _, neighbor := range neighbors {
					// Check the obstacles. The source and destination nodes do
					// not block: the edge must reach them.
					minX := js.Min(cx, neighbor.X)
					maxX := js.Max(cx, neighbor.X)
					minY := js.Min(cy, neighbor.Y)
					maxY := js.Max(cy, neighbor.Y)

					blocked := false
					for _, obs := range obstacles {
						if obs.nodeId == e.start || obs.nodeId == e.end {
							continue
						}
						var blocks bool
						if minX != maxX {
							// Horizontal
							blocks = obs.minY < cy && obs.maxY > cy && obs.maxX > minX && obs.minX < maxX
						} else {
							// Vertical
							blocks = obs.minX < cx && obs.maxX > cx && obs.maxY > minY && obs.minY < maxY
						}
						if blocks {
							blocked = true
							break
						}
					}

					if blocked {
						continue
					}

					nKey := getKey(neighbor.X, neighbor.Y)
					dist := math.Abs(neighbor.X-cx) + math.Abs(neighbor.Y-cy)
					penalty := crossingPenalty(i, current.pt, neighbor)

					// Directional penalty: strongly discourage a move opposite
					// to the direction of the destination. The penalty must be
					// higher than the crossing penalty.
					dirPenalty := 0.0
					destDx := pDstAnchor.X - pSrcAnchor.X
					destDy := pDstAnchor.Y - pSrcAnchor.Y
					moveDx := neighbor.X - cx
					moveDy := neighbor.Y - cy

					if (destDy > 10 && moveDy < -5) || (destDy < -10 && moveDy > 5) {
						dirPenalty = float64(math.Abs(moveDy) * 100)
					}
					// The same for the horizontal direction.
					if (destDx > 10 && moveDx < -5) || (destDx < -10 && moveDx > 5) {
						dirPenalty += float64(math.Abs(moveDx) * 50)
					}

					// Bend penalty: a change of direction costs, so that
					// straighter paths with fewer bends win.
					bendPenalty := 0.0
					currentDir, ok := arrivalDir[current.key]
					if !ok {
						currentDir = 'n'
					}
					moveDir := byte('v')
					if math.Abs(moveDx) > routerEPS {
						moveDir = 'h'
					}
					// A change of direction, and not at the start.
					if currentDir != 'n' && currentDir != moveDir {
						bendPenalty = 50
					}

					stepCost := dist + penalty + dirPenalty + bendPenalty
					currentG, ok := gScore[current.key]
					if !ok {
						currentG = math.Inf(1)
					}
					tentativeG := currentG + stepCost
					h := math.Abs(pDstAnchor.X-neighbor.X) + math.Abs(pDstAnchor.Y-neighbor.Y)

					neighborG, ok := gScore[nKey]
					if !ok {
						neighborG = math.Inf(1)
					}
					if tentativeG < neighborG {
						cameFrom[nKey] = current.pt
						gScore[nKey] = tentativeG
						arrivalDir[nKey] = moveDir // how this vertex was reached
						if !openSet[nKey] {
							openList = append(openList, openEntry{key: nKey, f: tentativeG + h, pt: neighbor})
							openSet[nKey] = true
						} else {
							for idx := range openList {
								if openList[idx].key == nKey {
									openList[idx].f = tentativeG + h
									break
								}
							}
						}
					}
				}
			}
		} // end of the A* block

		if len(foundPath) == 0 {
			foundPath = []Point{pSrcAnchor, {X: pSrcAnchor.X, Y: pDstAnchor.Y}, pDstAnchor}
		}

		// Path simplification: find the smallest x-extent and y-extent that
		// route around the obstacles, and rebuild the path with only those.

		if len(foundPath) > 4 {
			start := foundPath[0]
			end := foundPath[len(foundPath)-1]

			// The extreme x and y values of the path: how far the path had to
			// go to clear the obstacles.
			minX := js.Min(start.X, end.X)
			maxX := js.Max(start.X, end.X)
			minY := js.Min(start.Y, end.Y)
			maxY := js.Max(start.Y, end.Y)

			for _, pt := range foundPath {
				minX = js.Min(minX, pt.X)
				maxX = js.Max(maxX, pt.X)
				minY = js.Min(minY, pt.Y)
				maxY = js.Max(maxY, pt.Y)
			}
			_, _ = minY, maxY

			// Whether the path goes left or right (or both).
			wentRight := maxX > js.Max(start.X, end.X)
			wentLeft := minX < js.Min(start.X, end.X)

			// For the LR direction the detour x uses the visual extent: the x
			// of TB becomes the y of LR, so the extent is the height.
			if isLR {
				const margin = routerVerticalPipeMargin
				// The obstacles that block the direct vertical path: the
				// x-range has the x of the path and the y-range overlaps.
				if wentRight {
					pathX := js.Max(start.X, end.X)
					pathMinY := js.Min(start.Y, end.Y)
					pathMaxY := js.Max(start.Y, end.Y)
					detourObstacles := []ObstacleRect{}
					for _, obs := range obstacles {
						if obs.minX < pathX &&
							obs.maxX > pathX && // the x-range of the obstacle has the path x
							obs.minY < pathMaxY &&
							obs.maxY > pathMinY { // the y-ranges overlap
							detourObstacles = append(detourObstacles, obs)
						}
					}
					if len(detourObstacles) > 0 {
						// The obstacle that needs the largest detour by its
						// visual extent.
						visualMaxX := js.Max(start.X, end.X)
						for _, obs := range detourObstacles {
							obsCenterX := float64((obs.minX + obs.maxX) / 2)
							// Skip obstacles without a valid visualXHalfExtent.
							if math.IsNaN(obs.visualXHalfExtent) {
								continue
							}
							visualRight := obsCenterX + obs.visualXHalfExtent + margin
							visualMaxX = js.Max(visualMaxX, visualRight)
						}

						// The visual maxX can be smaller than the original maxX:
						// the route is closer to the obstacles.
						if !math.IsNaN(visualMaxX) {
							maxX = visualMaxX
						}
					}
				}
				// The obstacles that caused the left detour.
				if wentLeft {
					detourObstacles := []ObstacleRect{}
					for _, obs := range obstacles {
						if obs.minX < js.Min(start.X, end.X)+margin && // the obstacle extends past the direct path
							obs.minY < js.Max(start.Y, end.Y) &&
							obs.maxY > js.Min(start.Y, end.Y) { // the obstacle is in the y range
							detourObstacles = append(detourObstacles, obs)
						}
					}
					if len(detourObstacles) > 0 {
						// The obstacle that needs the smallest x by its visual
						// extent.
						visualMinX := js.Min(start.X, end.X)
						for _, obs := range detourObstacles {
							obsCenterX := float64((obs.minX + obs.maxX) / 2)
							visualLeft := obsCenterX - obs.visualXHalfExtent - margin
							visualMinX = js.Min(visualMinX, visualLeft)
						}
						minX = visualMinX
					}
				}
			}

			// The best y for the horizontal return segment: closest to the
			// obstacles and not to the destination, so that the route hugs the
			// obstacles.
			findBestReturnY := func(detourX float64) float64 {
				goingDown := end.Y > start.Y
				// The obstacles that the route goes around: between start and
				// end, blocking the direct path.
				relevantObs := []ObstacleRect{}
				for _, obs := range obstacles {
					obsInXRange := js.Min(start.X, end.X) < obs.maxX && js.Max(start.X, end.X) > obs.minX
					obsInYRange := js.Min(start.Y, end.Y) < obs.maxY && js.Max(start.Y, end.Y) > obs.minY
					if obsInXRange && obsInYRange {
						relevantObs = append(relevantObs, obs)
					}
				}

				// For LR the obstacles that the detour column (detourX) hits
				// count most. Obstacles off to the side should not make the
				// detour deeper.
				filteredObs := relevantObs
				if isLR && len(relevantObs) > 0 {
					obsAtDetourX := []ObstacleRect{}
					for _, obs := range relevantObs {
						if obs.minX < detourX && obs.maxX > detourX {
							obsAtDetourX = append(obsAtDetourX, obs)
						}
					}
					if len(obsAtDetourX) > 0 {
						filteredObs = obsAtDetourX
					}
				}

				if len(filteredObs) == 0 {
					return end.Y
				}

				// The obstacle edge closest to the destination in the direction
				// of travel.
				const margin = routerHorizontalPipeMargin
				if goingDown {
					// Going down: the bottom of the lowest obstacle to clear.
					lowestObsBottom := math.Inf(-1)
					for _, obs := range filteredObs {
						lowestObsBottom = js.Max(lowestObsBottom, obs.maxY)
					}
					bestY := lowestObsBottom + margin
					// Use it only if it is closer than end.y.
					if bestY < end.Y-routerEPS {
						return bestY
					}
				} else {
					// Going up: the top of the highest obstacle to clear.
					highestObsTop := math.Inf(1)
					for _, obs := range filteredObs {
						highestObsTop = js.Min(highestObsTop, obs.minY)
					}
					bestY := highestObsTop - margin
					// Use it only if it is closer than end.y.
					if bestY > end.Y+routerEPS {
						return bestY
					}
				}
				return end.Y
			}

			// Build a minimal path with only the extreme coordinates.
			// A U-shaped detour to the right:
			//   start, (maxX, start.y), (maxX, bestY), (end.x, bestY), end
			// A U-shaped detour to the left uses minX. nil stands for null.
			trySimplifyWithDetourX := func(detourX float64) []Point {
				bestY := findBestReturnY(detourX)
				corner1 := Point{X: detourX, Y: start.Y}
				corner2 := Point{X: detourX, Y: bestY}
				corner3 := Point{X: end.X, Y: bestY}
				seg1Blocked := checkSegmentBlocked(start, corner1)
				seg2Blocked := checkSegmentBlocked(corner1, corner2)
				seg3Blocked := checkSegmentBlocked(corner2, corner3)
				seg4Blocked := false
				if bestY != end.Y {
					seg4Blocked = checkSegmentBlocked(corner3, end)
				}

				if !seg1Blocked && !seg2Blocked && !seg3Blocked && !seg4Blocked {
					if math.Abs(bestY-end.Y) < routerEPS {
						return []Point{start, corner1, corner2, end}
					}
					return []Point{start, corner1, corner2, corner3, end}
				}
				return nil
			}

			var simplified []Point
			if wentRight && !wentLeft {
				simplified = trySimplifyWithDetourX(maxX)
			} else if wentLeft && !wentRight {
				simplified = trySimplifyWithDetourX(minX)
			}

			if simplified != nil {
				foundPath = simplified
			}
		}

		// 5. Collapse the collinear points and generate the segments. The
		// handle waypoints route around the obstacles at the source and at
		// the destination. The destination waypoints are stored from the
		// anchor to the port, so they are reversed.
		fullPoints := make([]Point, 0, 2+len(srcHandleWaypoints)+len(foundPath)+len(dstHandleWaypoints))
		fullPoints = append(fullPoints, pSrcPort)
		fullPoints = append(fullPoints, srcHandleWaypoints...)
		fullPoints = append(fullPoints, foundPath...)
		for k := len(dstHandleWaypoints) - 1; k >= 0; k-- {
			fullPoints = append(fullPoints, dstHandleWaypoints[k])
		}
		fullPoints = append(fullPoints, pDstPort)

		// Remove a hook or an overshoot at the end. When the last three points
		// A, B, C are collinear and B is beyond C, B is removed.
		if len(fullPoints) >= 3 {
			C := fullPoints[len(fullPoints)-1]
			B := fullPoints[len(fullPoints)-2]
			A := fullPoints[len(fullPoints)-3]

			// Collinear: horizontal or vertical.
			isHoriz := math.Abs(A.Y-B.Y) < routerEPS && math.Abs(B.Y-C.Y) < routerEPS
			isVert := math.Abs(A.X-B.X) < routerEPS && math.Abs(B.X-C.X) < routerEPS

			if isHoriz {
				// B is an overshoot when it is further from A than C is, in
				// the same direction.
				signAB := js.Sign(B.X - A.X)
				signAC := js.Sign(C.X - A.X)
				if signAB != 0 && signAB == signAC && math.Abs(B.X-A.X) > math.Abs(C.X-A.X) {
					// Remove B: fullPoints = [..., A, C]
					fullPoints = append(fullPoints[:len(fullPoints)-2], fullPoints[len(fullPoints)-1])
				}
			} else if isVert {
				signAB := js.Sign(B.Y - A.Y)
				signAC := js.Sign(C.Y - A.Y)
				if signAB != 0 && signAB == signAC && math.Abs(B.Y-A.Y) > math.Abs(C.Y-A.Y) {
					fullPoints = append(fullPoints[:len(fullPoints)-2], fullPoints[len(fullPoints)-1])
				}
			}
		}

		simplified := []Point{fullPoints[0]}
		for k := 1; k < len(fullPoints)-1; k++ {
			// Keep the anchor point (k = 1): Z and U shapes have four points.
			if k == 1 {
				simplified = append(simplified, fullPoints[k])
				continue
			}
			prev := simplified[len(simplified)-1]
			curr := fullPoints[k]
			next := fullPoints[k+1]

			// Collinear points are skipped, but not where the direction
			// reverses.
			// Horizontal: the y is the same.
			if math.Abs(prev.Y-curr.Y) < routerEPS && math.Abs(curr.Y-next.Y) < routerEPS {
				// Check a reversal of the direction.
				dir1 := curr.X > prev.X
				dir2 := next.X > curr.X
				if dir1 != dir2 {
					simplified = append(simplified, curr)
					continue
				}
				// The same direction: skip.
				continue
			}
			// Vertical: the x is the same.
			if math.Abs(prev.X-curr.X) < routerEPS && math.Abs(curr.X-next.X) < routerEPS {
				dir1 := curr.Y > prev.Y
				dir2 := next.Y > curr.Y
				if dir1 != dir2 {
					simplified = append(simplified, curr)
					continue
				}
				continue
			}

			simplified = append(simplified, curr)
		}
		simplified = append(simplified, fullPoints[len(fullPoints)-1])

		// Create the segments.
		for k := 0; k < len(simplified)-1; k++ {
			p1 := simplified[k]
			p2 := simplified[k+1]
			orientation := "horizontal"
			if math.Abs(p1.X-p2.X) < routerEPS {
				orientation = "vertical"
			}
			coord := p1.Y
			from := js.Min(p1.X, p2.X)
			to := js.Max(p1.X, p2.X)
			if orientation == "vertical" {
				coord = p1.X
				from = js.Min(p1.Y, p2.Y)
				to = js.Max(p1.Y, p2.Y)
			}

			pipe := getOrAddPipe(orientation, coord, from, to)

			rSeg := &routerRoutedSegment{
				edgeIndex:    i,
				segmentIndex: k,
				orientation:  orientation,
				pipe:         pipe,
				trackIndex:   0, // the initial track
				from:         from,
				to:           to,
				allIndex:     len(allRoutedSegments),
			}

			allRoutedSegments = append(allRoutedSegments, rSeg)
			edgeSegmentIndices[i] = append(edgeSegmentIndices[i], len(allRoutedSegments)-1)

			if len(pipe.tracks) == 0 {
				pipe.tracks = append(pipe.tracks, nil)
			}
			if pipe.tracks[0] == nil {
				pipe.tracks[0] = &routerTrack{index: 0, coord: pipe.coord, segments: []routerSegmentRef{}}
			}
			pipe.tracks[0].segments = append(pipe.tracks[0].segments, routerSegmentRef{
				edgeIndex:    i,
				segmentIndex: k,
				from:         from,
				to:           to,
			})
		}
	}

	// -----------------------------------------------------------------------
	// Phase 2: Crossing Reduction
	// -----------------------------------------------------------------------

	// segmentsOverlap says whether two intervals intersect. [a, b] and [c, d]
	// overlap if a < d and c < b. from and to are sorted.
	segmentsOverlap := func(s1From, s1To, s2From, s2To float64) bool {
		return s1From < s2To && s2From < s1To
	}

	// withoutSegment is segments.filter((r) => r is not the segment).
	withoutSegment := func(segments []routerSegmentRef, edgeIndex, segmentIndex int) []routerSegmentRef {
		out := make([]routerSegmentRef, 0, len(segments)+1)
		for _, r := range segments {
			if r.edgeIndex != edgeIndex || r.segmentIndex != segmentIndex {
				out = append(out, r)
			}
		}
		return out
	}

	trySwapSegmentsAcrossTracks := func(s1, s2 *routerRoutedSegment, t1, t2 *routerTrack) bool {
		if t1 == nil || t2 == nil {
			panic(jsError("Cannot read properties of undefined (reading 'segments')"))
		}
		canS1GoT2 := true
		for _, r := range t2.segments {
			if (r.edgeIndex != s2.edgeIndex || r.segmentIndex != s2.segmentIndex) &&
				segmentsOverlap(r.from, r.to, s1.from, s1.to) {
				canS1GoT2 = false
				break
			}
		}
		canS2GoT1 := true
		for _, r := range t1.segments {
			if (r.edgeIndex != s1.edgeIndex || r.segmentIndex != s1.segmentIndex) &&
				segmentsOverlap(r.from, r.to, s2.from, s2.to) {
				canS2GoT1 = false
				break
			}
		}

		if canS1GoT2 && canS2GoT1 {
			s1.trackIndex = t2.index
			s2.trackIndex = t1.index
			// t1 and t2 can be the same track: the second assignment reads the
			// result of the first, as upstream.
			t1.segments = append(withoutSegment(t1.segments, s1.edgeIndex, s1.segmentIndex), routerSegmentRef{
				edgeIndex:    s2.edgeIndex,
				segmentIndex: s2.segmentIndex,
				from:         s2.from,
				to:           s2.to,
			})
			t2.segments = append(withoutSegment(t2.segments, s2.edgeIndex, s2.segmentIndex), routerSegmentRef{
				edgeIndex:    s1.edgeIndex,
				segmentIndex: s1.segmentIndex,
				from:         s1.from,
				to:           s1.to,
			})
			return true
		}
		return false
	}

	createNewTrack := func(pipe *routerPipe) int {
		idx := len(pipe.tracks)
		pipe.tracks = append(pipe.tracks, &routerTrack{index: idx, coord: pipe.coord, segments: []routerSegmentRef{}})
		return idx
	}

	moveSegmentToTrack := func(seg *routerRoutedSegment, trackIdx int) {
		oldTrack := routerTrackAt(seg.pipe, seg.trackIndex)
		if oldTrack == nil {
			panic(jsError("Cannot read properties of undefined (reading 'segments')"))
		}
		oldTrack.segments = withoutSegment(oldTrack.segments, seg.edgeIndex, seg.segmentIndex)
		seg.trackIndex = trackIdx
		newTrack := routerTrackAt(seg.pipe, trackIdx)
		if newTrack == nil {
			panic(jsError("Cannot read properties of undefined (reading 'segments')"))
		}
		newTrack.segments = append(newTrack.segments, routerSegmentRef{
			edgeIndex:    seg.edgeIndex,
			segmentIndex: seg.segmentIndex,
			from:         seg.from,
			to:           seg.to,
		})
	}

	moveSegmentChainToTrack := func(seg *routerRoutedSegment, trackIdx int) {
		// Move all segments of this edge that are on the same pipe, not only
		// the consecutive ones.
		indices := edgeSegmentIndices[seg.edgeIndex]
		for _, idx := range indices {
			s := allRoutedSegments[idx]
			if s.pipe == seg.pipe {
				moveSegmentToTrack(s, trackIdx)
			}
		}
	}

	getAdjacentSegmentsAlongEdge := func(seg *routerRoutedSegment) []*routerRoutedSegment {
		indices := edgeSegmentIndices[seg.edgeIndex]
		// indices.indexOf(allRoutedSegments.indexOf(seg))
		idxInList := -1
		for k, idx := range indices {
			if idx == seg.allIndex {
				idxInList = k
				break
			}
		}
		adj := []*routerRoutedSegment{}
		if idxInList > 0 {
			adj = append(adj, allRoutedSegments[indices[idxInList-1]])
		}
		if idxInList < len(indices)-1 {
			adj = append(adj, allRoutedSegments[indices[idxInList+1]])
		}
		return adj
	}

	haveAnyCrossing := func(segA, segB *routerRoutedSegment) bool {
		if segA.orientation == segB.orientation {
			return false
		}
		h, v := segB, segA
		if segA.orientation == "horizontal" {
			h, v = segA, segB
		}
		return v.pipe.coord > h.from && v.pipe.coord < h.to && h.pipe.coord > v.from && h.pipe.coord < v.to
	}

	findAvailableTrack := func(pipe *routerPipe, seg *routerRoutedSegment) int {
		for _, track := range pipe.tracks {
			if track == nil {
				continue
			}
			overlap := false
			for _, r := range track.segments {
				if (r.edgeIndex != seg.edgeIndex || r.segmentIndex != seg.segmentIndex) &&
					segmentsOverlap(r.from, r.to, seg.from, seg.to) {
					overlap = true
					break
				}
			}
			if !overlap {
				return track.index
			}
		}
		return -1
	}

	segmentsConflict := func(s1, s2 *routerRoutedSegment) bool {
		if s1.trackIndex == s2.trackIndex {
			return segmentsOverlap(s1.from, s1.to, s2.from, s2.to)
		}

		adj1 := getAdjacentSegmentsAlongEdge(s1)
		adj2 := getAdjacentSegmentsAlongEdge(s2)
		for _, a1 := range adj1 {
			for _, a2 := range adj2 {
				if haveAnyCrossing(a1, a2) {
					return true
				}
			}
		}
		return false
	}

	resolveTrackConflict := func(s1, s2 *routerRoutedSegment, move func(seg *routerRoutedSegment, trackIdx int)) {
		if trySwapSegmentsAcrossTracks(
			s1,
			s2,
			routerTrackAt(s1.pipe, s1.trackIndex),
			routerTrackAt(s2.pipe, s2.trackIndex),
		) {
			return
		}

		avail := findAvailableTrack(s1.pipe, s2)
		if avail != -1 {
			move(s2, avail)
		} else {
			move(s2, createNewTrack(s1.pipe))
		}
	}

	resolveHandleConflicts := func(handles []*routerRoutedSegment) int {
		crossings := 0
		for i := 0; i < len(handles); i++ {
			for j := i + 1; j < len(handles); j++ {
				h1 := handles[i]
				h2 := handles[j]
				if h1.pipe != h2.pipe {
					continue
				}

				if segmentsConflict(h1, h2) {
					crossings++
					resolveTrackConflict(h1, h2, moveSegmentChainToTrack)
				}
			}
		}
		return crossings
	}

	type DestInfo struct {
		dest      float64
		deviation float64
		base      float64
		delta     float64
	}
	destInfoCache := map[int]DestInfo{}
	getDestInfo := func(edgeIdx int) DestInfo {
		if info, ok := destInfoCache[edgeIdx]; ok {
			return info
		}
		indices := edgeSegmentIndices[edgeIdx]
		if len(indices) == 0 {
			info := DestInfo{dest: 0, deviation: 0, base: 0, delta: 0}
			destInfoCache[edgeIdx] = info
			return info
		}
		firstSeg := allRoutedSegments[indices[0]]
		base := firstSeg.pipe.coord
		dest := base
		for idx := 1; idx < len(indices); idx++ {
			seg := allRoutedSegments[indices[idx]]
			if seg.orientation == "horizontal" {
				candidateA := seg.from
				candidateB := seg.to
				if math.Abs(candidateA-base) > math.Abs(candidateB-base) {
					dest = candidateA
				} else {
					dest = candidateB
				}
				break
			}
		}
		deviation := math.Abs(dest - base)
		info := DestInfo{dest: dest, deviation: deviation, base: base, delta: dest - base}
		destInfoCache[edgeIdx] = info
		return info
	}

	fixSourceHandleCrossings := func() int {
		crossings := 0
		edgesBySource := js.NewMap[string, []int]()
		for i, e := range edges {
			if len(edgeSegmentIndices[i]) == 0 {
				continue
			}
			if e.start == "" {
				continue
			}
			edgesBySource.Set(e.start, append(edgesBySource.Get(e.start), i))
		}

		getEdgeDistance := func(edgeIdx int) float64 {
			edge := edges[edgeIdx]
			if edge.start == "" || edge.end == "" {
				return 0
			}
			srcNode := nodeById[edge.start]
			dstNode := nodeById[edge.end]
			if srcNode == nil || dstNode == nil {
				return 0
			}
			dx := ru.NN(dstNode.X, 0) - ru.NN(srcNode.X, 0)
			dy := ru.NN(dstNode.Y, 0) - ru.NN(srcNode.Y, 0)
			return math.Abs(dx) + math.Abs(dy)
		}

		for _, grp := range edgesBySource.Values() {
			// Sort by continuity: prefer the edges that continue straight from
			// the previous segment.
			sortFunc(grp, func(a, b int) float64 {
				// 0. Destination-aware ordering: keep the near-center edges on
				// the center track.
				infoA := getDestInfo(a)
				infoB := getDestInfo(b)
				if math.Abs(infoA.deviation-infoB.deviation) > 1 {
					return infoA.deviation - infoB.deviation
				}
				if math.Abs(infoA.dest-infoB.dest) > 1 {
					return infoA.dest - infoB.dest
				}

				// 0. Prefer longer spans: edges that travel further keep
				// straighter paths.
				distA := getEdgeDistance(a)
				distB := getEdgeDistance(b)
				if math.Abs(distA-distB) > 1 {
					return distB - distA
				}

				// 1. Segment count: prefer simpler paths.
				lenA := len(edgeSegmentIndices[a])
				lenB := len(edgeSegmentIndices[b])
				if lenA != lenB {
					return float64(lenA - lenB)
				}

				// 2. For single-segment edges: prefer the shorter edge, so
				// that it stays centered.
				if lenA == 1 {
					idxA := edgeSegmentIndices[a][0]
					idxB := edgeSegmentIndices[b][0]
					// Make sure that the indices exist.
					if idxA >= 0 && idxA < len(allRoutedSegments) && idxB >= 0 && idxB < len(allRoutedSegments) {
						segA := allRoutedSegments[idxA]
						segB := allRoutedSegments[idxB]
						distA := math.Abs(segA.to - segA.from)
						distB := math.Abs(segB.to - segB.from)
						if math.Abs(distA-distB) > 1 {
							// The shorter distance comes first and gets the
							// center track.
							return distA - distB
						}
					}
				}

				return 0
			})

			handles := make([]*routerRoutedSegment, len(grp))
			for k, ei := range grp {
				handles[k] = allRoutedSegments[edgeSegmentIndices[ei][0]]
			}
			crossings += resolveHandleConflicts(handles)
		}
		return crossings
	}

	fixTargetHandleCrossings := func() int {
		crossings := 0
		edgesByTarget := js.NewMap[string, []int]()
		for i, e := range edges {
			indices := edgeSegmentIndices[i]
			if len(indices) == 0 {
				continue
			}
			if e.end == "" {
				continue
			}
			edgesByTarget.Set(e.end, append(edgesByTarget.Get(e.end), i))
		}

		for _, grp := range edgesByTarget.Values() {
			// Sort by continuity: prefer the edges that align with their
			// previous segment.
			sortFunc(grp, func(a, b int) float64 {
				getDist := func(edgeIdx int) float64 {
					indices := edgeSegmentIndices[edgeIdx]
					if len(indices) < 2 {
						return 0
					}
					// A shorter perpendicular connector is a better alignment
					// of the target handle.
					prev := allRoutedSegments[indices[len(indices)-2]]
					return math.Abs(prev.to - prev.from)
				}

				scoreA := getDist(a)
				scoreB := getDist(b)
				if math.Abs(scoreA-scoreB) > 0.1 {
					return scoreA - scoreB // ascending length (shorter first)
				}
				return float64(a - b) // stable fallback
			})

			handles := make([]*routerRoutedSegment, len(grp))
			for k, ei := range grp {
				handles[k] = allRoutedSegments[edgeSegmentIndices[ei][len(edgeSegmentIndices[ei])-1]]
			}
			crossings += resolveHandleConflicts(handles)
		}
		return crossings
	}

	fixPipeCrossings := func() int {
		crossings := 0
		for _, pipe := range pipes {
			// Collect all segments in the pipe.
			pipeSegments := []*routerRoutedSegment{}
			for _, t := range pipe.tracks {
				if t == nil {
					continue
				}
				for _, ref := range t.segments {
					// Find the RoutedSegment object.
					for _, ix := range edgeSegmentIndices[ref.edgeIndex] {
						if allRoutedSegments[ix].segmentIndex == ref.segmentIndex {
							pipeSegments = append(pipeSegments, allRoutedSegments[ix])
							break
						}
					}
				}
			}

			sortFunc(pipeSegments, func(a, b *routerRoutedSegment) float64 {
				if d := a.edgeIndex - b.edgeIndex; d != 0 {
					return float64(d)
				}
				return float64(a.segmentIndex - b.segmentIndex)
			})

			for i := 0; i < len(pipeSegments); i++ {
				for j := i + 1; j < len(pipeSegments); j++ {
					s1 := pipeSegments[i]
					s2 := pipeSegments[j]

					if segmentsConflict(s1, s2) {
						crossings++
						resolveTrackConflict(s1, s2, moveSegmentToTrack)
					}
				}
			}
		}
		return crossings
	}

	// Main reduction loop
	iterations := 0
	const MAX_ITER = 10
	for iterations < MAX_ITER {
		changed := 0
		changed += fixSourceHandleCrossings()
		changed += fixTargetHandleCrossings()
		changed += fixPipeCrossings()
		if changed == 0 {
			break
		}
		iterations++
	}

	// -----------------------------------------------------------------------
	// Phase 3: Rebuild Geometry
	// -----------------------------------------------------------------------
	// {edgeIndex, segmentIndex} -> coord. Upstream uses the string key
	// `${edgeIndex}-${segmentIndex}`.
	segmentCoords := map[[2]int]float64{}

	for _, pipe := range pipes {
		// Find the clusters of connected segments (an interval graph).
		type SegmentInfo struct {
			edgeIndex    int
			segmentIndex int
			trackIndex   int
			from         float64
			to           float64
		}

		segments := []SegmentInfo{}
		for _, t := range pipe.tracks {
			if t == nil {
				continue
			}
			for _, s := range t.segments {
				segments = append(segments, SegmentInfo{
					edgeIndex:    s.edgeIndex,
					segmentIndex: s.segmentIndex,
					trackIndex:   t.index,
					from:         s.from,
					to:           s.to,
				})
			}
		}

		sortFunc(segments, func(a, b SegmentInfo) float64 { return a.from - b.from })

		clusters := [][]SegmentInfo{}
		if len(segments) > 0 {
			currentCluster := []SegmentInfo{segments[0]}
			clusterEnd := segments[0].to

			for k := 1; k < len(segments); k++ {
				s := segments[k]
				if s.from < clusterEnd {
					currentCluster = append(currentCluster, s)
					clusterEnd = js.Max(clusterEnd, s.to)
				} else {
					clusters = append(clusters, currentCluster)
					currentCluster = []SegmentInfo{s}
					clusterEnd = s.to
				}
			}
			clusters = append(clusters, currentCluster)
		}

		// Assign the local coordinates of each cluster.
		for _, cluster := range clusters {
			usedTracks := js.NewSet[int]()
			for _, s := range cluster {
				usedTracks.Add(s.trackIndex)
			}
			trackScores := map[int]float64{}
			for _, s := range cluster {
				info := getDestInfo(s.edgeIndex)
				trackScores[s.trackIndex] += info.delta
			}

			leftTracks := []int{}
			rightTracks := []int{}
			neutralTracks := []int{}
			for _, t := range usedTracks.Values() {
				if trackScores[t] < -1 {
					leftTracks = append(leftTracks, t)
				}
			}
			for _, t := range usedTracks.Values() {
				if trackScores[t] > 1 {
					rightTracks = append(rightTracks, t)
				}
			}
			for _, t := range usedTracks.Values() {
				if math.Abs(trackScores[t]) <= 1 {
					neutralTracks = append(neutralTracks, t)
				}
			}

			sortFunc(leftTracks, func(a, b int) float64 { return trackScores[b] - trackScores[a] })
			sortFunc(rightTracks, func(a, b int) float64 { return trackScores[a] - trackScores[b] })

			assignCoord := func(trackIndex int, coord float64) {
				for _, s := range cluster {
					if s.trackIndex != trackIndex {
						continue
					}
					// Straight intra-lane edges keep the pipe coord: they are
					// not spread.
					effectiveCoord := coord
					if straightIntraLaneEdges[s.edgeIndex] {
						effectiveCoord = pipe.coord
					}
					segmentCoords[[2]int{s.edgeIndex, s.segmentIndex}] = effectiveCoord
				}
			}

			leftCount := 0
			for _, trackIndex := range leftTracks {
				leftCount++
				assignCoord(trackIndex, pipe.coord-float64(float64(leftCount)*routerTrackSpacing))
			}

			if len(neutralTracks) == 0 && usedTracks.Len() > 0 {
				// Without a neutral track, the track closest to the center
				// becomes neutral.
				byScore := usedTracks.Values()
				sortFunc(byScore, func(a, b int) float64 {
					return math.Abs(trackScores[a]) - math.Abs(trackScores[b])
				})
				bestTrack := byScore[0]
				for k, t := range leftTracks {
					if t == bestTrack {
						leftTracks = append(leftTracks[:k], leftTracks[k+1:]...)
						break
					}
				}
				for k, t := range rightTracks {
					if t == bestTrack {
						rightTracks = append(rightTracks[:k], rightTracks[k+1:]...)
						break
					}
				}
				neutralTracks = append(neutralTracks, bestTrack)
			}

			neutralAssigned := 0
			for _, trackIndex := range neutralTracks {
				if neutralAssigned == 0 {
					assignCoord(trackIndex, pipe.coord)
				} else {
					dir := -1.0
					if neutralAssigned%2 == 1 {
						dir = 1
					}
					magnitude := math.Ceil(float64(neutralAssigned) / 2)
					assignCoord(trackIndex, pipe.coord+float64(dir*magnitude*routerTrackSpacing*0.5))
				}
				neutralAssigned++
			}

			rightCount := 0
			for _, trackIndex := range rightTracks {
				rightCount++
				assignCoord(trackIndex, pipe.coord+float64(float64(rightCount)*routerTrackSpacing))
			}
		}
	}

	// No fan-out nudge for sibling labels is needed: a labelled edge routes
	// as one edge and shares the track assignment with every other edge.

	for i, e := range edges {
		indices := edgeSegmentIndices[i]
		if len(indices) == 0 {
			continue
		}

		newPoints := []Point{}

		// Recompute the ports, with the side assignment of Step 6.2.
		src := nodeById[e.start]
		dst := nodeById[e.end]
		if src == nil || dst == nil {
			// An edge with segments has both nodes.
			continue
		}
		pSrcPort, pDstPort, _, _ := portsForEdge(i, src, dst)

		lines := make([]routerRoutedLine, len(indices))
		for k, idx := range indices {
			s := allRoutedSegments[idx]
			coord, ok := segmentCoords[[2]int{s.edgeIndex, s.segmentIndex}]
			if !ok {
				coord = s.pipe.coord
			}
			lines[k] = routerRoutedLine{
				orient: s.orientation,
				coord:  coord,
				from:   s.from,
				to:     s.to,
			}
		}

		newPoints = append(newPoints, pSrcPort)

		for k := 0; k < len(lines); k++ {
			line := lines[k]
			prevPt := newPoints[len(newPoints)-1]
			prevAlong := prevPt.X
			prevTrackCoord := prevPt.Y
			if line.orient == "vertical" {
				prevAlong = prevPt.Y
				prevTrackCoord = prevPt.X
			}
			hasNextLine := k < len(lines)-1
			var nextLine routerRoutedLine
			if hasNextLine {
				nextLine = lines[k+1]
			}

			if math.Abs(prevTrackCoord-line.coord) > routerEPS {
				newPoints = append(newPoints, routerPointOnLine(line, prevAlong))
			}

			if hasNextLine && nextLine.orient == line.orient {
				if math.Abs(line.coord-nextLine.coord) > routerEPS {
					var junction float64
					if line.orient == "vertical" {
						junction = (prevAlong + nextLine.from) / 2
					} else {
						junction = routerSharedLineEndpointCoord(line, nextLine)
					}
					newPoints = append(newPoints, routerPointOnLine(line, junction), routerPointOnLine(nextLine, junction))
				} else if k == 0 || k == len(lines)-2 {
					newPoints = append(newPoints, routerPointOnLine(line, routerSharedLineEndpointCoord(line, nextLine)))
				}
			} else if hasNextLine {
				newPoints = append(newPoints, routerPointOnLine(line, nextLine.coord))
			} else {
				endAlong := line.from
				if math.Abs(line.from-prevAlong) < math.Abs(line.to-prevAlong) {
					endAlong = line.to
				}
				newPoints = append(newPoints, routerPointOnLine(line, endAlong))
			}
		}

		// End at pDstPort, so that the intersection logic of the renderer
		// does not eat the last anchor.
		last := newPoints[len(newPoints)-1]
		if math.Abs(last.X-pDstPort.X) > routerEPS || math.Abs(last.Y-pDstPort.Y) > routerEPS {
			newPoints = append(newPoints, pDstPort)
		}

		filtered := []Point{}
		if len(newPoints) > 0 {
			filtered = append(filtered, newPoints[0])
		}
		for k := 1; k < len(newPoints); k++ {
			p := newPoints[k]
			prev := filtered[len(filtered)-1]
			if math.Abs(p.X-prev.X) > routerEPS || math.Abs(p.Y-prev.Y) > routerEPS {
				filtered = append(filtered, p)
			}
		}

		e.points = filtered
	}

	// Copy the polyline of the routing view back to the original edge. Each
	// labelled original routes as one unbroken polyline.
	for _, re := range edges {
		orig := re.originalEdge
		if orig != nil && re.points != nil {
			orig.Points = re.points
		}
	}

	// Remove the layout-only virtual edges from the layout. They gave the
	// Sugiyama phases their layering constraints through the label nodes and
	// must not reach rendering, validation or scoring.
	kept := make([]*ru.Edge, 0, len(data.Edges))
	for _, e := range data.Edges {
		if !e.IsLayoutOnly {
			kept = append(kept, e)
		}
	}
	data.Edges = kept

	// Snap the edge endpoints onto the rectangular boundary of their nodes.
	// The port and anchor logic can leave an end of the polyline inside the
	// node body. The renderer clips to the node boundary, but the validator
	// sees the raw points. The snap keeps the orientation of the first and
	// the last segment.
	nodeBoundaryClamp := func(p Point, node *ru.Node) Point {
		cx := ru.NN(node.X, 0)
		cy := ru.NN(node.Y, 0)
		w := ru.NN(node.Width, 0)
		h := ru.NN(node.Height, 0)
		if w <= 0 || h <= 0 {
			return p
		}
		left := cx - float64(w/2)
		right := cx + float64(w/2)
		top := cy - float64(h/2)
		bottom := cy + float64(h/2)
		// Already outside the rect: leave it alone.
		if p.X < left || p.X > right || p.Y < top || p.Y > bottom {
			return p
		}
		// Inside or on the rect: project onto the nearest edge. The order of
		// the checks decides ties.
		dLeft := p.X - left
		dRight := right - p.X
		dTop := p.Y - top
		dBottom := bottom - p.Y
		minD := js.Min(dLeft, dRight, dTop, dBottom)
		if minD == dLeft {
			return Point{X: left, Y: p.Y}
		}
		if minD == dRight {
			return Point{X: right, Y: p.Y}
		}
		if minD == dTop {
			return Point{X: p.X, Y: top}
		}
		return Point{X: p.X, Y: bottom}
	}

	for _, edge := range data.Edges {
		pts := edge.Points
		if len(pts) < 2 {
			continue
		}
		srcId := edge.Start
		dstId := edge.End
		var src, dst *ru.Node
		if srcId != "" {
			src = nodeById[srcId]
		}
		if dstId != "" {
			dst = nodeById[dstId]
		}
		if src != nil {
			pts[0] = nodeBoundaryClamp(pts[0], src)
		}
		if dst != nil {
			pts[len(pts)-1] = nodeBoundaryClamp(pts[len(pts)-1], dst)
		}
	}

	return data
}
