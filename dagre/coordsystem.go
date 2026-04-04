package dagre

// CoordSystemAdjust swaps width/height for LR/RL rank directions.
func CoordSystemAdjust(g *Graph) {
	rd := g.label.RankDir
	if rd == "LR" || rd == "RL" {
		swapWidthHeight(g)
	}
}

// CoordSystemUndo restores coordinates after layout is complete.
func CoordSystemUndo(g *Graph) {
	rd := g.label.RankDir
	if rd == "BT" || rd == "RL" {
		reverseY(g)
	}
	if rd == "LR" || rd == "RL" {
		swapXY(g)
		swapWidthHeight(g)
	}
}

func swapWidthHeight(g *Graph) {
	for _, v := range g.Nodes() {
		n := g.Node(v)
		n.Width, n.Height = n.Height, n.Width
	}
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		el.Width, el.Height = el.Height, el.Width
	}
}

func reverseY(g *Graph) {
	for _, v := range g.Nodes() {
		n := g.Node(v)
		n.Y = -n.Y
	}
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		for i := range el.Points {
			el.Points[i].Y = -el.Points[i].Y
		}
		if el.HasY {
			el.Y = -el.Y
		}
	}
}

func swapXY(g *Graph) {
	for _, v := range g.Nodes() {
		n := g.Node(v)
		n.X, n.Y = n.Y, n.X
	}
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		for i := range el.Points {
			el.Points[i].X, el.Points[i].Y = el.Points[i].Y, el.Points[i].X
		}
		if el.HasX && el.HasY {
			el.X, el.Y = el.Y, el.X
		}
	}
}
