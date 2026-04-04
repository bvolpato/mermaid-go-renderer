package dagre

// NormalizeRun breaks long edges into chains of dummy nodes spanning 1 rank each.
func NormalizeRun(g *Graph) {
	g.label.DummyChains = nil
	for _, e := range g.Edges() {
		normalizeEdge(g, e)
	}
}

func normalizeEdge(g *Graph, e Edge) {
	v := e.V
	vRank := g.Node(v).Rank
	w := e.W
	wRank := g.Node(w).Rank
	edgeLabel := g.EdgeByKey(e)

	if wRank == vRank+1 {
		return
	}

	g.RemoveEdge(e)

	vRank++
	for i := 0; vRank < wRank; i++ {
		edgeLabel.Points = nil
		dummy := UniqueID("_d")
		nl := &NodeLabel{
			Width:     0,
			Height:    0,
			Rank:      vRank,
			HasRank:   true,
			Dummy:     "edge",
			EdgeLabel: edgeLabel,
			EdgeObj:   &e,
		}
		if edgeLabel.HasLabelRank && vRank == edgeLabel.LabelRank {
			nl.Width = edgeLabel.Width
			nl.Height = edgeLabel.Height
			nl.Dummy = "edge-label"
			nl.LabelPos = edgeLabel.LabelPos
		}
		g.SetNode(dummy, nl)
		g.SetEdgeNamed(v, dummy, e.Name, &EdgeLabel{Weight: edgeLabel.Weight, MinLen: 1})
		if i == 0 {
			g.label.DummyChains = append(g.label.DummyChains, dummy)
		}
		v = dummy
		vRank++
	}

	g.SetEdgeNamed(v, w, e.Name, &EdgeLabel{Weight: edgeLabel.Weight, MinLen: 1})
}

// NormalizeUndo removes dummy nodes and reconstructs edge points.
func NormalizeUndo(g *Graph) {
	for _, v := range g.label.DummyChains {
		node := g.Node(v)
		if node == nil {
			continue
		}
		origLabel := node.EdgeLabel
		if origLabel == nil || node.EdgeObj == nil {
			continue
		}
		g.SetEdge(*node.EdgeObj, origLabel)
		current := v
		for {
			cn := g.Node(current)
			if cn == nil || cn.Dummy == "" {
				break
			}
			succs := g.Successors(current)
			if len(succs) == 0 {
				break
			}
			w := succs[0]
			g.RemoveNode(current)
			origLabel.Points = append(origLabel.Points, Point{X: cn.X, Y: cn.Y})
			if cn.Dummy == "edge-label" {
				origLabel.X = cn.X
				origLabel.Y = cn.Y
				origLabel.HasX = true
				origLabel.HasY = true
				origLabel.Width = cn.Width
				origLabel.Height = cn.Height
			}
			current = w
		}
	}
}
