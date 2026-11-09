// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/direction/lrTransform.ts (MIT).

package swimlanes

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

func lrBuildNodeMap(nodes []*ru.Node) map[string]*ru.Node {
	m := make(map[string]*ru.Node, len(nodes))
	for _, node := range nodes {
		m[node.ID] = node
	}
	return m
}

// resolveTopLevelGroupId returns "" for null.
func resolveTopLevelGroupId(node *ru.Node, nodeById map[string]*ru.Node) string {
	parentId := node.ParentID
	topLevelGroupId := ""
	for parentId != "" {
		parent := nodeById[parentId]
		if parent == nil || !parent.IsGroup {
			break
		}
		topLevelGroupId = parent.ID
		parentId = parent.ParentID
	}
	return topLevelGroupId
}

func groupDepth(group *ru.Node, nodeById map[string]*ru.Node) int {
	depth := 0
	parentId := group.ParentID
	for parentId != "" {
		parent := nodeById[parentId]
		if parent == nil || !parent.IsGroup {
			break
		}
		depth++
		parentId = parent.ParentID
	}
	return depth
}

type childBounds struct {
	minX, maxX, minY, maxY float64
}

// boundsForChildren returns false for null.
func boundsForChildren(children []*ru.Node) (childBounds, bool) {
	minX := math.Inf(1)
	maxX := math.Inf(-1)
	minY := math.Inf(1)
	maxY := math.Inf(-1)
	for _, child := range children {
		cx := child.X
		cy := child.Y
		if ru.IsUndef(cx) || ru.IsUndef(cy) {
			continue
		}
		w := ru.NN(child.Width, 0)
		h := ru.NN(child.Height, 0)
		minX = js.Min(minX, cx-float64(w/2))
		maxX = js.Max(maxX, cx+float64(w/2))
		minY = js.Min(minY, cy-float64(h/2))
		maxY = js.Max(maxY, cy+float64(h/2))
	}
	if minX == math.Inf(1) || minY == math.Inf(1) {
		return childBounds{}, false
	}
	return childBounds{minX: minX, maxX: maxX, minY: minY, maxY: maxY}, true
}

func applyGroupBounds(group *ru.Node, bounds childBounds) {
	pad := ru.NN(group.Padding, 20)
	group.X = (bounds.minX + bounds.maxX) / 2
	group.Y = (bounds.minY + bounds.maxY) / 2
	group.Width = js.Max(0, bounds.maxX-bounds.minX) + pad
	group.Height = js.Max(0, bounds.maxY-bounds.minY) + pad
}

func recomputeNestedGroupBounds(nodes []*ru.Node) {
	nodeById := lrBuildNodeMap(nodes)
	var groupsByDepth []*ru.Node
	for _, node := range nodes {
		if node.IsGroup && node.ParentID != "" {
			groupsByDepth = append(groupsByDepth, node)
		}
	}
	sortFunc(groupsByDepth, func(a, b *ru.Node) float64 {
		return float64(groupDepth(b, nodeById) - groupDepth(a, nodeById))
	})

	for _, group := range groupsByDepth {
		var children []*ru.Node
		for _, node := range nodes {
			if node.ParentID == group.ID {
				children = append(children, node)
			}
		}
		if bounds, ok := boundsForChildren(children); ok {
			applyGroupBounds(group, bounds)
		}
	}
}

// mirrorAxis mirrors the layout on the axis "x" or "y".
func mirrorAxis(layout *ru.LayoutData, axis string) bool {
	nodes := layout.Nodes
	edges := layout.Edges
	get := func(node *ru.Node) float64 {
		if axis == "x" {
			return node.X
		}
		return node.Y
	}
	min := math.Inf(1)
	max := math.Inf(-1)
	for _, node := range nodes {
		if node.IsGroup {
			continue
		}
		value := get(node)
		if ru.IsUndef(value) {
			continue
		}
		min = js.Min(min, value)
		max = js.Max(max, value)
	}
	if !isFiniteNumber(min) || !isFiniteNumber(max) {
		return false
	}
	mirror := func(value float64) float64 { return min + max - value }
	for _, node := range nodes {
		value := get(node)
		if !ru.IsUndef(value) {
			if axis == "x" {
				node.X = mirror(value)
			} else {
				node.Y = mirror(value)
			}
		}
		if titleRect := node.GroupTitleRect; titleRect != nil {
			next := *titleRect
			if axis == "x" {
				next.Left = mirror(titleRect.Right)
				next.Right = mirror(titleRect.Left)
			} else {
				next.Top = mirror(titleRect.Bottom)
				next.Bottom = mirror(titleRect.Top)
			}
			node.GroupTitleRect = &next
		}
	}
	for _, edge := range edges {
		for i := range edge.Points {
			if axis == "x" {
				edge.Points[i].X = mirror(edge.Points[i].X)
			} else {
				edge.Points[i].Y = mirror(edge.Points[i].Y)
			}
		}
	}
	return true
}

func applyBtDirectionTransform(layout *ru.LayoutData) bool {
	hasContent := false
	for _, node := range layout.Nodes {
		if !node.IsGroup {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return true
	}

	return mirrorAxis(layout, "y")
}

type lrLaneBound struct {
	lane          *ru.Node
	contentTop    float64
	contentBottom float64
	centerY       float64
}

// applyLrDirectionTransform rotates the layout: the rank progression goes
// onto X. direction is "LR" or "RL".
func applyLrDirectionTransform(layout *ru.LayoutData, direction string) bool {
	nodes := layout.Nodes
	edges := layout.Edges
	var contentNodes []*ru.Node
	for _, n := range nodes {
		if !n.IsGroup {
			contentNodes = append(contentNodes, n)
		}
	}

	minX := math.Inf(1)
	minY := math.Inf(1)
	for _, n := range contentNodes {
		x0 := ru.NN(n.X, 0)
		y0 := ru.NN(n.Y, 0)
		if x0 < minX {
			minX = x0
		}
		if y0 < minY {
			minY = y0
		}
	}

	if !isFiniteNumber(minX) || !isFiniteNumber(minY) {
		return false
	}

	const titleBandSize = 36

	totalWidth := 0.0
	totalHeight := 0.0
	for _, n := range contentNodes {
		totalWidth += ru.NN(n.Width, 0)
		totalHeight += ru.NN(n.Height, 0)
	}
	avgWidth := totalWidth / float64(len(contentNodes))
	avgHeight := totalHeight / float64(len(contentNodes))
	horizontalScaleFactor := 1.0
	if avgHeight > 0 {
		horizontalScaleFactor = js.Max(1, avgWidth/avgHeight)
	}

	for _, n := range contentNodes {
		x0 := ru.NN(n.X, 0)
		y0 := ru.NN(n.Y, 0)
		newX := float64((y0-minY)*horizontalScaleFactor) + titleBandSize
		newY := x0 - minX

		n.X = newX
		n.Y = newY
	}

	for _, e := range edges {
		for i := range e.Points {
			x0 := e.Points[i].X
			y0 := e.Points[i].Y
			newX := float64((y0-minY)*horizontalScaleFactor) + titleBandSize
			newY := x0 - minX
			e.Points[i].X = newX
			e.Points[i].Y = newY
		}
	}

	recomputeNestedGroupBounds(nodes)

	var laneNodes []*ru.Node
	for _, n := range nodes {
		if n.IsGroup && n.ParentID == "" {
			laneNodes = append(laneNodes, n)
		}
	}
	if len(laneNodes) == 0 {
		if direction == "RL" {
			mirrorAxis(layout, "x")
		}
		return true
	}

	nodeById := lrBuildNodeMap(nodes)
	childrenByLane := map[string][]*ru.Node{}

	for _, n := range nodes {
		if n.IsGroup {
			continue
		}
		laneId := resolveTopLevelGroupId(n, nodeById)
		if laneId == "" {
			continue
		}
		childrenByLane[laneId] = append(childrenByLane[laneId], n)
	}

	maxPad := 0.0
	for _, lane := range laneNodes {
		pad := ru.NN(lane.Padding, 0)
		if pad > maxPad {
			maxPad = pad
		}
	}

	var laneBounds []*lrLaneBound
	globalMinXChild := math.Inf(1)
	globalMaxXChild := math.Inf(-1)

	for _, lane := range laneNodes {
		children := childrenByLane[lane.ID]
		bounds, ok := boundsForChildren(children)
		if !ok {
			continue
		}
		globalMinXChild = js.Min(globalMinXChild, bounds.minX)
		globalMaxXChild = js.Max(globalMaxXChild, bounds.maxX)

		laneBounds = append(laneBounds, &lrLaneBound{
			lane:          lane,
			contentTop:    bounds.minY,
			contentBottom: bounds.maxY,
			centerY:       (bounds.minY + bounds.maxY) / 2,
		})
	}

	if globalMinXChild == math.Inf(1) || globalMaxXChild == math.Inf(-1) {
		return true
	}

	fullContentWidth := js.Max(0, globalMaxXChild-globalMinXChild)
	horizontalMargin := js.Max(maxPad, 10)
	bodyWidth := fullContentWidth + float64(2*horizontalMargin)
	laneWidth := titleBandSize + bodyWidth
	bodyCenter := float64((globalMinXChild + globalMaxXChild) / 2)
	bodyLeft := bodyCenter - float64(bodyWidth/2)
	laneLeft := bodyLeft - titleBandSize
	centerX := laneLeft + float64(laneWidth/2)
	verticalMargin := js.Max(maxPad, titleBandSize)

	sortFunc(laneBounds, func(a, b *lrLaneBound) float64 { return a.centerY - b.centerY })

	for i := 0; i < len(laneBounds); i++ {
		curr := laneBounds[i]
		var laneTop float64
		var laneBottom float64

		if i == 0 {
			laneTop = curr.contentTop - verticalMargin
		} else {
			prev := laneBounds[i-1]
			laneTop = (prev.contentBottom + curr.contentTop) / 2
		}

		if i == len(laneBounds)-1 {
			laneBottom = curr.contentBottom + verticalMargin
		} else {
			next := laneBounds[i+1]
			laneBottom = (curr.contentBottom + next.contentTop) / 2
		}

		laneHeight := js.Max(0, laneBottom-laneTop)
		centerY := (laneTop + laneBottom) / 2

		curr.lane.X = centerX
		curr.lane.Y = centerY
		curr.lane.Width = laneWidth
		curr.lane.Height = laneHeight
		curr.lane.Set("swimlaneContentTop", curr.contentTop)
		curr.lane.GroupTitleRect = &ru.GroupTitleRect{
			Left:   laneLeft,
			Right:  laneLeft + titleBandSize,
			Top:    laneTop,
			Bottom: laneBottom,
		}
	}

	if direction == "RL" {
		mirrorAxis(layout, "x")
	}

	return true
}
