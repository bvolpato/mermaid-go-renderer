package dagre

import (
	"math"
	"sort"
)

// Position assigns x and y coordinates to all nodes.
func Position(g *Graph) {
	ng := AsNonCompoundGraph(g)
	positionY(ng)
	xs := positionX(ng)
	for v, x := range xs {
		n := g.Node(v)
		if n != nil {
			n.X = x
			n.HasX = true
		}
	}
	// Copy y from ng to g
	for _, v := range ng.Nodes() {
		nn := ng.Node(v)
		gn := g.Node(v)
		if gn != nil && nn != nil {
			gn.Y = nn.Y
			gn.HasY = true
		}
	}
}

func positionY(g *Graph) {
	layering := BuildLayerMatrix(g)
	rankSep := g.label.RankSep
	prevY := 0.0
	for _, layer := range layering {
		maxHeight := 0.0
		for _, v := range layer {
			h := g.Node(v).Height
			if h > maxHeight {
				maxHeight = h
			}
		}
		for _, v := range layer {
			n := g.Node(v)
			n.Y = prevY + maxHeight/2
			n.HasY = true
		}
		prevY += maxHeight + rankSep
	}
}

type conflicts map[string]map[string]bool
type positionMap map[string]float64

func addConflict(c conflicts, v, w string) {
	if v > w {
		v, w = w, v
	}
	if c[v] == nil {
		c[v] = make(map[string]bool)
	}
	c[v][w] = true
}

func hasConflict(c conflicts, v, w string) bool {
	if v > w {
		v, w = w, v
	}
	cv, ok := c[v]
	if !ok {
		return false
	}
	return cv[w]
}

func findType1Conflicts(g *Graph, layering [][]string) conflicts {
	c := make(conflicts)
	if len(layering) == 0 {
		return c
	}

	prev := layering[0]
	for li := 1; li < len(layering); li++ {
		layer := layering[li]
		k0 := 0
		scanPos := 0
		prevLayerLength := len(prev)
		lastNode := ""
		if len(layer) > 0 {
			lastNode = layer[len(layer)-1]
		}

		for i, v := range layer {
			w := findOtherInnerSegmentNode(g, v)
			k1 := prevLayerLength
			if w != "" {
				wn := g.Node(w)
				if wn != nil && wn.HasOrder {
					k1 = wn.Order
				}
			}

			if w != "" || v == lastNode {
				for _, scanNode := range layer[scanPos : i+1] {
					preds := g.Predecessors(scanNode)
					for _, u := range preds {
						uLabel := g.Node(u)
						uPos := 0
						if uLabel != nil && uLabel.HasOrder {
							uPos = uLabel.Order
						}
						scanLabel := g.Node(scanNode)
						if (uPos < k0 || k1 < uPos) &&
							!(uLabel != nil && uLabel.Dummy != "" && scanLabel != nil && scanLabel.Dummy != "") {
							addConflict(c, u, scanNode)
						}
					}
				}
				scanPos = i + 1
				k0 = k1
			}
		}

		prev = layer
	}
	return c
}

func findOtherInnerSegmentNode(g *Graph, v string) string {
	n := g.Node(v)
	if n != nil && n.Dummy != "" {
		preds := g.Predecessors(v)
		for _, u := range preds {
			un := g.Node(u)
			if un != nil && un.Dummy != "" {
				return u
			}
		}
	}
	return ""
}

type alignmentResult struct {
	root  map[string]string
	align map[string]string
}

func verticalAlignment(g *Graph, layering [][]string, c conflicts, neighborFn func(string) []string) alignmentResult {
	root := make(map[string]string)
	alignMap := make(map[string]string)
	pos := make(map[string]int)

	for _, layer := range layering {
		for order, v := range layer {
			root[v] = v
			alignMap[v] = v
			pos[v] = order
		}
	}

	for _, layer := range layering {
		prevIdx := -1
		for _, v := range layer {
			ws := neighborFn(v)
			if len(ws) == 0 {
				continue
			}
			// Sort by position
			sort.Slice(ws, func(i, j int) bool {
				return pos[ws[i]] < pos[ws[j]]
			})
			mp := float64(len(ws)-1) / 2
			for i := int(math.Floor(mp)); i <= int(math.Ceil(mp)); i++ {
				if i >= len(ws) {
					continue
				}
				w := ws[i]
				posW, ok := pos[w]
				if !ok {
					continue
				}
				if alignMap[v] == v && prevIdx < posW && !hasConflict(c, v, w) {
					rootW, ok := root[w]
					if ok {
						alignMap[w] = v
						root[v] = rootW
						alignMap[v] = rootW
						prevIdx = posW
					}
				}
			}
		}
	}

	return alignmentResult{root: root, align: alignMap}
}

func horizontalCompaction(g *Graph, layering [][]string, root, alignMap map[string]string, reverseSep bool) positionMap {
	xs := make(positionMap)
	blockG := buildBlockGraph(g, layering, root, reverseSep)

	// First pass: assign smallest coordinates via topological sweep
	pass1Stack := make([]string, len(blockG.Nodes()))
	copy(pass1Stack, blockG.Nodes())
	visited := make(map[string]bool)

	for i := len(pass1Stack) - 1; i >= 0; {
		elem := pass1Stack[i]
		pass1Stack = pass1Stack[:i]
		if visited[elem] {
			// Process: xs[elem] = max of predecessors
			inEdges := blockG.InEdges(elem)
			if len(inEdges) > 0 {
				maxVal := 0.0
				for _, e := range inEdges {
					xv, ok := xs[e.V]
					if !ok {
						xv = 0
					}
					el := blockG.EdgeByKey(e)
					w := 0.0
					if el != nil {
						w = el.Width // storing sep as width
					}
					val := xv + w
					if val > maxVal {
						maxVal = val
					}
				}
				xs[elem] = maxVal
			} else {
				xs[elem] = 0
			}
		} else {
			visited[elem] = true
			pass1Stack = append(pass1Stack, elem)
			preds := blockG.Predecessors(elem)
			for _, p := range preds {
				pass1Stack = append(pass1Stack, p)
			}
		}
		i = len(pass1Stack) - 1
	}

	// Second pass: compact by pulling toward successors
	borderType := "borderRight"
	if reverseSep {
		borderType = "borderLeft"
	}

	visited2 := make(map[string]bool)
	pass2Stack := make([]string, len(blockG.Nodes()))
	copy(pass2Stack, blockG.Nodes())

	for i := len(pass2Stack) - 1; i >= 0; {
		elem := pass2Stack[i]
		pass2Stack = pass2Stack[:i]
		if visited2[elem] {
			outEdges := blockG.OutEdges(elem)
			minVal := math.Inf(1)
			if len(outEdges) > 0 {
				for _, e := range outEdges {
					xw, ok := xs[e.W]
					if !ok {
						xw = 0
					}
					el := blockG.EdgeByKey(e)
					w := 0.0
					if el != nil {
						w = el.Width
					}
					val := xw - w
					if val < minVal {
						minVal = val
					}
				}
			}
			n := g.Node(elem)
			if minVal != math.Inf(1) && (n == nil || n.BorderType != borderType) {
				curX, ok := xs[elem]
				if !ok {
					curX = 0
				}
				if minVal > curX {
					xs[elem] = minVal
				}
			}
		} else {
			visited2[elem] = true
			pass2Stack = append(pass2Stack, elem)
			succs := blockG.Successors(elem)
			for _, s := range succs {
				pass2Stack = append(pass2Stack, s)
			}
		}
		i = len(pass2Stack) - 1
	}

	// Assign x coordinates to all aligned nodes
	for v := range alignMap {
		rootV := root[v]
		if rootV != "" {
			if rx, ok := xs[rootV]; ok {
				xs[v] = rx
			}
		}
	}

	return xs
}

func buildBlockGraph(g *Graph, layering [][]string, root map[string]string, reverseSep bool) *Graph {
	bg := NewSimpleGraph()
	nodeSep := g.label.NodeSep
	edgeSep := g.label.EdgeSep

	for _, layer := range layering {
		var u string
		hasU := false
		for _, v := range layer {
			vRoot, ok := root[v]
			if !ok {
				continue
			}
			bg.SetNode(vRoot, &NodeLabel{})
			if hasU {
				uRoot := root[u]
				s := sep(g, nodeSep, edgeSep, reverseSep, v, u)
				existing := bg.EdgeVW(uRoot, vRoot)
				if existing != nil {
					if s > existing.Width {
						existing.Width = s
					}
				} else {
					bg.SetEdgeVW(uRoot, vRoot, &EdgeLabel{Width: s})
				}
			}
			u = v
			hasU = true
		}
	}

	return bg
}

func sep(g *Graph, nodeSep, edgeSep float64, reverseSep bool, v, w string) float64 {
	vLabel := g.Node(v)
	wLabel := g.Node(w)
	sum := 0.0

	if vLabel != nil {
		sum += vLabel.Width / 2
	}
	if vLabel != nil && vLabel.Dummy != "" {
		sum += edgeSep / 2
	} else {
		sum += nodeSep / 2
	}
	if wLabel != nil && wLabel.Dummy != "" {
		sum += edgeSep / 2
	} else {
		sum += nodeSep / 2
	}
	if wLabel != nil {
		sum += wLabel.Width / 2
	}

	return sum
}

func positionX(g *Graph) positionMap {
	layering := BuildLayerMatrix(g)
	c := findType1Conflicts(g, layering)

	type xssEntry struct {
		key string
		xs  positionMap
	}
	xss := make(map[string]positionMap)

	for _, vert := range []string{"u", "d"} {
		adjustedLayering := layering
		if vert == "d" {
			adjustedLayering = reverseLayers(layering)
		}
		for _, horiz := range []string{"l", "r"} {
			al := adjustedLayering
			if horiz == "r" {
				al = reverseEachLayer(al)
			}

			neighborFn := func(v string) []string {
				if vert == "u" {
					return g.Predecessors(v)
				}
				return g.Successors(v)
			}

			alignment := verticalAlignment(g, al, c, neighborFn)
			xs := horizontalCompaction(g, al, alignment.root, alignment.align, horiz == "r")
			if horiz == "r" {
				for k, v := range xs {
					xs[k] = -v
				}
			}
			xss[vert+horiz] = xs
		}
	}

	smallest := findSmallestWidthAlignment(g, xss)
	alignCoordinates(xss, smallest)
	return balance(xss, g.label.Align)
}

func reverseLayers(layers [][]string) [][]string {
	result := make([][]string, len(layers))
	for i := range layers {
		result[i] = layers[len(layers)-1-i]
	}
	return result
}

func reverseEachLayer(layers [][]string) [][]string {
	result := make([][]string, len(layers))
	for i, layer := range layers {
		r := make([]string, len(layer))
		for j, v := range layer {
			r[len(layer)-1-j] = v
		}
		result[i] = r
	}
	return result
}

func findSmallestWidthAlignment(g *Graph, xss map[string]positionMap) positionMap {
	minWidth := math.Inf(1)
	var best positionMap
	for _, xs := range xss {
		maxX := math.Inf(-1)
		minX := math.Inf(1)
		for v, x := range xs {
			hw := 0.0
			n := g.Node(v)
			if n != nil {
				hw = n.Width / 2
			}
			if x+hw > maxX {
				maxX = x + hw
			}
			if x-hw < minX {
				minX = x - hw
			}
		}
		w := maxX - minX
		if w < minWidth {
			minWidth = w
			best = xs
		}
	}
	return best
}

func alignCoordinates(xss map[string]positionMap, alignTo positionMap) {
	if alignTo == nil {
		return
	}
	alignToMin := math.Inf(1)
	alignToMax := math.Inf(-1)
	for _, v := range alignTo {
		if v < alignToMin {
			alignToMin = v
		}
		if v > alignToMax {
			alignToMax = v
		}
	}

	for _, vert := range []string{"u", "d"} {
		for _, horiz := range []string{"l", "r"} {
			key := vert + horiz
			xs := xss[key]
			if xs == nil || sameMap(xs, alignTo) {
				continue
			}

			xsMin := math.Inf(1)
			xsMax := math.Inf(-1)
			for _, v := range xs {
				if v < xsMin {
					xsMin = v
				}
				if v > xsMax {
					xsMax = v
				}
			}

			delta := alignToMin - xsMin
			if horiz != "l" {
				delta = alignToMax - xsMax
			}

			if delta != 0 {
				newXs := make(positionMap)
				for k, v := range xs {
					newXs[k] = v + delta
				}
				xss[key] = newXs
			}
		}
	}
}

func sameMap(a, b positionMap) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func balance(xss map[string]positionMap, align string) positionMap {
	ul := xss["ul"]
	if ul == nil {
		return make(positionMap)
	}

	result := make(positionMap)
	for v := range ul {
		if align != "" {
			key := align
			if xs, ok := xss[key]; ok {
				if val, ok := xs[v]; ok {
					result[v] = val
					continue
				}
			}
		}
		// Take median of the 4 alignments
		vals := make([]float64, 0, 4)
		for _, xs := range xss {
			if val, ok := xs[v]; ok {
				vals = append(vals, val)
			}
		}
		sort.Float64s(vals)
		if len(vals) >= 4 {
			result[v] = (vals[1] + vals[2]) / 2
		} else if len(vals) > 0 {
			result[v] = vals[len(vals)/2]
		}
	}
	return result
}
