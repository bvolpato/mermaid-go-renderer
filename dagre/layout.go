package dagre

import "math"

// Layout runs the dagre layout algorithm on the given graph.
// After layout, every node has X/Y coordinates and every edge has Points.
func Layout(g *Graph) {
	runLayout(g)
}

func runLayout(g *Graph) {
	makeSpaceForEdgeLabels(g)
	removeSelfEdges(g)
	AcyclicRun(g)
	if g.isCompound {
		NestingGraphRun(g)
	}
	Rank(AsNonCompoundGraph(g))
	injectEdgeLabelProxies(g)
	removeEmptyRanks(g)
	if g.isCompound {
		NestingGraphCleanup(g)
	}
	normalizeRanks(g)
	assignRankMinMax(g)
	removeEdgeLabelProxies(g)
	NormalizeRun(g)
	if g.isCompound {
		ParentDummyChains(g)
		AddBorderSegments(g)
	}
	Order(g)
	insertSelfEdges(g)
	CoordSystemAdjust(g)
	Position(g)
	positionSelfEdges(g)
	removeBorderNodes(g)
	NormalizeUndo(g)
	fixupEdgeLabelCoords(g)
	CoordSystemUndo(g)
	translateGraph(g)
	assignNodeIntersects(g)
	reversePointsForReversedEdges(g)
	AcyclicUndo(g)
}

// makeSpaceForEdgeLabels halves ranksep and doubles minlen to create room
// for edge labels between ranks.
func makeSpaceForEdgeLabels(g *Graph) {
	g.label.RankSep /= 2
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		el.MinLen *= 2
		if el.LabelPos != "c" && el.LabelPos != "C" {
			rd := g.label.RankDir
			if rd == "TB" || rd == "BT" || rd == "" {
				el.Width += el.LabelOffset
			} else {
				el.Height += el.LabelOffset
			}
		}
	}
}

func removeSelfEdges(g *Graph) {
	for _, e := range g.Edges() {
		if e.V == e.W {
			node := g.Node(e.V)
			label := g.EdgeByKey(e)
			node.SelfEdges = append(node.SelfEdges, SelfEdge{E: e, Label: *label})
			g.RemoveEdge(e)
		}
	}
}

func insertSelfEdges(g *Graph) {
	layers := BuildLayerMatrix(g)
	for _, layer := range layers {
		orderShift := 0
		for i, v := range layer {
			node := g.Node(v)
			if node == nil {
				continue
			}
			node.Order = i + orderShift
			node.HasOrder = true
			for _, se := range node.SelfEdges {
				orderShift++
				dummy := AddDummyNode(g, "selfedge", "", node.Rank)
				dn := g.Node(dummy)
				dn.Width = se.Label.Width
				dn.Height = se.Label.Height
				dn.Rank = node.Rank
				dn.HasRank = true
				dn.Order = i + orderShift
				dn.HasOrder = true
				dn.Dummy = "selfedge"
				dn.EdgeObj = &Edge{V: se.E.V, W: se.E.W, Name: se.E.Name}
				dn.EdgeLabel = &se.Label
			}
			node.SelfEdges = nil
		}
	}
}

func positionSelfEdges(g *Graph) {
	for _, v := range g.Nodes() {
		node := g.Node(v)
		if node.Dummy != "selfedge" {
			continue
		}
		if node.EdgeObj == nil || node.EdgeLabel == nil {
			continue
		}
		selfNode := g.Node(node.EdgeObj.V)
		if selfNode == nil {
			continue
		}
		x := selfNode.X + selfNode.Width/2
		y := selfNode.Y
		dx := node.X - x
		dy := selfNode.Height / 2

		label := node.EdgeLabel
		g.SetEdge(*node.EdgeObj, label)
		g.RemoveNode(v)
		label.Points = []Point{
			{X: x + 2*dx/3, Y: y - dy},
			{X: x + 5*dx/6, Y: y - dy},
			{X: x + dx, Y: y},
			{X: x + 5*dx/6, Y: y + dy},
			{X: x + 2*dx/3, Y: y + dy},
		}
		label.X = node.X
		label.Y = node.Y
		label.HasX = true
		label.HasY = true
	}
}

func injectEdgeLabelProxies(g *Graph) {
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		if el.Width > 0 && el.Height > 0 {
			vn := g.Node(e.V)
			wn := g.Node(e.W)
			if vn == nil || wn == nil {
				continue
			}
			proxyRank := (wn.Rank-vn.Rank)/2 + vn.Rank
			dummy := AddDummyNode(g, "edge-proxy", "_ep", proxyRank)
			dn := g.Node(dummy)
			dn.Dummy = "edge-proxy"
			dn.Rank = proxyRank
			dn.HasRank = true
			dn.EdgeObj = &e
		}
	}
}

func removeEdgeLabelProxies(g *Graph) {
	for _, v := range g.Nodes() {
		node := g.Node(v)
		if node.Dummy == "edge-proxy" && node.EdgeObj != nil {
			el := g.EdgeByKey(*node.EdgeObj)
			if el != nil {
				el.LabelRank = node.Rank
				el.HasLabelRank = true
			}
			g.RemoveNode(v)
		}
	}
}

func assignRankMinMax(g *Graph) {
	maxRank := 0
	for _, v := range g.Nodes() {
		node := g.Node(v)
		if node.BorderTop != "" {
			bt := g.Node(node.BorderTop)
			bb := g.Node(node.BorderBot)
			if bt != nil && bb != nil {
				node.MinRank = bt.Rank
				node.MaxRank = bb.Rank
				node.HasMinRank = true
				node.HasMaxRank = true
				if bb.Rank > maxRank {
					maxRank = bb.Rank
				}
			}
		}
	}
	g.label.MaxRank = maxRank
}

func normalizeRanks(g *Graph) {
	minRank := math.MaxInt64
	for _, v := range g.Nodes() {
		n := g.Node(v)
		if n.HasRank && n.Rank < minRank {
			minRank = n.Rank
		}
	}
	if minRank == math.MaxInt64 {
		return
	}
	for _, v := range g.Nodes() {
		n := g.Node(v)
		if n.HasRank {
			n.Rank -= minRank
		}
	}
}

func removeEmptyRanks(g *Graph) {
	nrf := g.label.NodeRankFactor

	// Find min/max rank
	minRank := math.MaxInt64
	maxRank := math.MinInt64
	for _, v := range g.Nodes() {
		n := g.Node(v)
		if n.HasRank {
			if n.Rank < minRank {
				minRank = n.Rank
			}
			if n.Rank > maxRank {
				maxRank = n.Rank
			}
		}
	}
	if minRank == math.MaxInt64 {
		return
	}

	// Find occupied ranks
	occupied := make(map[int]bool)
	for _, v := range g.Nodes() {
		n := g.Node(v)
		if n.HasRank {
			occupied[n.Rank] = true
		}
	}

	// Build rank map to compress
	rankMap := make(map[int]int)
	newRank := 0
	for r := minRank; r <= maxRank; r++ {
		if occupied[r] || (nrf > 0 && r%nrf == 0) {
			rankMap[r] = newRank
			newRank++
		}
	}

	// Apply new ranks
	for _, v := range g.Nodes() {
		n := g.Node(v)
		if n.HasRank {
			if nr, ok := rankMap[n.Rank]; ok {
				n.Rank = nr
			}
		}
	}
}

func translateGraph(g *Graph) {
	minX := math.Inf(1)
	maxX := 0.0
	minY := math.Inf(1)
	maxY := 0.0
	marginX := g.label.MarginX
	marginY := g.label.MarginY

	for _, v := range g.Nodes() {
		n := g.Node(v)
		x, y, w, h := n.X, n.Y, n.Width, n.Height
		if x-w/2 < minX {
			minX = x - w/2
		}
		if x+w/2 > maxX {
			maxX = x + w/2
		}
		if y-h/2 < minY {
			minY = y - h/2
		}
		if y+h/2 > maxY {
			maxY = y + h/2
		}
	}

	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		if el.HasX {
			x, y, w, h := el.X, el.Y, el.Width, el.Height
			if x-w/2 < minX {
				minX = x - w/2
			}
			if x+w/2 > maxX {
				maxX = x + w/2
			}
			if y-h/2 < minY {
				minY = y - h/2
			}
			if y+h/2 > maxY {
				maxY = y + h/2
			}
		}
	}

	minX -= marginX
	minY -= marginY

	for _, v := range g.Nodes() {
		n := g.Node(v)
		n.X -= minX
		n.Y -= minY
	}

	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		for i := range el.Points {
			el.Points[i].X -= minX
			el.Points[i].Y -= minY
		}
		if el.HasX {
			el.X -= minX
		}
		if el.HasY {
			el.Y -= minY
		}
	}

	g.label.Width = maxX - minX + marginX
	g.label.Height = maxY - minY + marginY
}

func assignNodeIntersects(g *Graph) {
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		nodeV := g.Node(e.V)
		nodeW := g.Node(e.W)
		if nodeV == nil || nodeW == nil {
			continue
		}

		var p1, p2 Point
		if len(el.Points) == 0 {
			el.Points = nil
			p1 = Point{X: nodeW.X, Y: nodeW.Y}
			p2 = Point{X: nodeV.X, Y: nodeV.Y}
		} else {
			p1 = el.Points[0]
			p2 = el.Points[len(el.Points)-1]
		}

		v1 := IntersectRect(nodeV, p1)
		v2 := IntersectRect(nodeW, p2)

		// Prepend v1, append v2
		newPts := make([]Point, 0, len(el.Points)+2)
		newPts = append(newPts, v1)
		newPts = append(newPts, el.Points...)
		newPts = append(newPts, v2)
		el.Points = newPts
	}
}

func fixupEdgeLabelCoords(g *Graph) {
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		if el.HasX {
			lp := el.LabelPos
			if lp == "l" || lp == "r" {
				el.Width -= el.LabelOffset
			}
			switch lp {
			case "l":
				el.X -= el.Width/2 + el.LabelOffset
			case "r":
				el.X += el.Width/2 + el.LabelOffset
			}
		}
	}
}

func reversePointsForReversedEdges(g *Graph) {
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		if el.Reversed && len(el.Points) > 1 {
			for i, j := 0, len(el.Points)-1; i < j; i, j = i+1, j-1 {
				el.Points[i], el.Points[j] = el.Points[j], el.Points[i]
			}
		}
	}
}

func removeBorderNodes(g *Graph) {
	// First pass: compute dimensions for compound nodes
	for _, v := range g.Nodes() {
		if len(g.Children(v)) > 0 {
			node := g.Node(v)
			if node == nil {
				continue
			}
			var t, b, l, r *NodeLabel
			if node.BorderTop != "" {
				t = g.Node(node.BorderTop)
			}
			if node.BorderBot != "" {
				b = g.Node(node.BorderBot)
			}
			if node.BorderLeft != nil && len(node.BorderLeft) > 0 {
				last := node.BorderLeft[len(node.BorderLeft)-1]
				if last != "" {
					l = g.Node(last)
				}
			}
			if node.BorderRight != nil && len(node.BorderRight) > 0 {
				last := node.BorderRight[len(node.BorderRight)-1]
				if last != "" {
					r = g.Node(last)
				}
			}

			if l != nil && r != nil {
				node.Width = math.Abs(r.X - l.X)
				node.X = l.X + node.Width/2
			}
			if t != nil && b != nil {
				node.Height = math.Abs(b.Y - t.Y)
				node.Y = t.Y + node.Height/2
			}
		}
	}

	// Second pass: remove border nodes
	for _, v := range g.Nodes() {
		n := g.Node(v)
		if n != nil && n.Dummy == "border" {
			g.RemoveNode(v)
		}
	}
}
