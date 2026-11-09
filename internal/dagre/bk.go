// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/position/bk.js (MIT).

package dagre

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// This file provides coordinate assignment based on Brandes and Köpf, "Fast
// and Simple Horizontal Coordinate Assignment."

// conflicts is the set of conflicting node pairs. JavaScript stores it as
// conflicts[v][w] = true with v <= w.
type conflicts map[[2]string]bool

// findType1Conflicts marks all edges in the graph with a type-1 conflict. A
// type-1 conflict is one where a non-inner segment crosses an inner
// segment. An inner segment is an edge with both incident nodes marked with
// the "dummy" property.
//
// This algorithm scans layer by layer, starting with the second, for type-1
// conflicts between the current layer and the previous layer. For each
// layer it scans the nodes from left to right until it reaches one that is
// incident on an inner segment. It then scans predecessors to determine if
// they have edges that cross that inner segment. At the end a final scan is
// done for all nodes on the current rank to see if they cross the last
// visited inner segment.
//
// This algorithm (safely) assumes that a dummy node will only be incident
// on a single node in the layers being scanned.
func findType1Conflicts(g *graphlib.Graph, layering [][]string, conflicts conflicts) {
	visitLayer := func(prevLayer, layer []string) {
		// k0 is the position of the last visited node in the previous layer
		// that is incident on an inner segment.
		k0 := 0.0
		// scanPos tracks the last node in this layer scanned for crossings
		// with a type-1 segment.
		scanPos := 0
		prevLayerLength := float64(len(prevLayer))
		lastNode := last(layer)

		for i, v := range layer {
			w := findOtherInnerSegmentNode(g, v)
			k1 := prevLayerLength
			if w != "" {
				k1 = nodeOf(g, w).order
			}

			if w != "" || v == lastNode {
				for _, scanNode := range layer[scanPos : i+1] {
					for _, u := range g.Predecessors(scanNode) {
						uLabel := nodeOf(g, u)
						uPos := uLabel.order
						if (uPos < k0 || k1 < uPos) && !(uLabel.dummy != "" && nodeOf(g, scanNode).dummy != "") {
							addConflict(conflicts, u, scanNode)
						}
					}
				}
				scanPos = i + 1
				k0 = k1
			}
		}
	}

	// _.reduce(layering, visitLayer)
	for i := 1; i < len(layering); i++ {
		visitLayer(layering[i-1], layering[i])
	}
}

func findType2Conflicts(g *graphlib.Graph, layering [][]string, conflicts conflicts) {
	scan := func(south []string, southPos, southEnd, prevNorthBorder, nextNorthBorder float64) {
		for _, i := range rangeStartEnd(southPos, southEnd) {
			v := south[int(i)]
			if nodeOf(g, v).dummy != "" {
				for _, u := range g.Predecessors(v) {
					uNode := nodeOf(g, u)
					if uNode.dummy != "" && (uNode.order < prevNorthBorder || uNode.order > nextNorthBorder) {
						addConflict(conflicts, u, v)
					}
				}
			}
		}
	}

	visitLayer := func(north, south []string) {
		prevNorthPos := -1.0
		// nextNorthPos is undefined until the first border node.
		nextNorthPos := nan
		southPos := 0.0

		for southLookahead, v := range south {
			if nodeOf(g, v).dummy == "border" {
				predecessors := g.Predecessors(v)
				if len(predecessors) > 0 {
					nextNorthPos = nodeOf(g, predecessors[0]).order
					scan(south, southPos, float64(southLookahead), prevNorthPos, nextNorthPos)
					southPos = float64(southLookahead)
					prevNorthPos = nextNorthPos
				}
			}
			scan(south, southPos, float64(len(south)), nextNorthPos, float64(len(north)))
		}
	}

	// _.reduce(layering, visitLayer)
	for i := 1; i < len(layering); i++ {
		visitLayer(layering[i-1], layering[i])
	}
}

func findOtherInnerSegmentNode(g *graphlib.Graph, v string) string {
	if nodeOf(g, v).dummy != "" {
		for _, u := range g.Predecessors(v) {
			if nodeOf(g, u).dummy != "" {
				return u
			}
		}
	}
	return ""
}

func addConflict(conflicts conflicts, v, w string) {
	if js.CompareStrings(v, w) > 0 {
		v, w = w, v
	}
	conflicts[[2]string{v, w}] = true
}

func hasConflict(conflicts conflicts, v, w string) bool {
	if js.CompareStrings(v, w) > 0 {
		v, w = w, v
	}
	return conflicts[[2]string{v, w}]
}

// verticalAlignment tries to align nodes into vertical "blocks" where
// possible. This algorithm attempts to align a node with one of its median
// neighbors. If the edge connecting a neighbor is a type-1 conflict then we
// ignore that possibility. If a previous node has already formed a block
// with a node after the node we're trying to form a block with, we also
// ignore that possibility - our blocks would be split in that scenario.
func (l *layout) verticalAlignment(layering [][]string, conflicts conflicts, neighborFn func(v string) []string) (root map[string]string, align *js.Obj[string]) {
	root = map[string]string{}
	align = js.NewObj[string]()
	pos := map[string]float64{}

	// We cache the position here based on the layering because the graph
	// and layering may be out of sync. The layering matrix is manipulated
	// to generate different extreme alignments.
	for _, layer := range layering {
		for order, v := range layer {
			root[v] = v
			align.Set(v, v)
			pos[v] = float64(order)
		}
	}

	posOf := func(w string) sortKey {
		p, ok := pos[w]
		return sortKey{v: p, defined: ok}
	}

	for _, layer := range layering {
		prevIdx := -1.0
		for _, v := range layer {
			ws := neighborFn(v)
			if len(ws) > 0 {
				ws = sortBy(l.state.Sort, ws, posOf)
				mp := (float64(len(ws)) - 1) / 2
				for i, il := math.Floor(mp), math.Ceil(mp); i <= il; i++ {
					w := ws[int(i)]
					posW, okW := pos[w]
					if !okW {
						posW = nan
					}
					if align.Get(v) == v && prevIdx < posW && !hasConflict(conflicts, v, w) {
						align.Set(w, v)
						root[v] = root[w]
						align.Set(v, root[v])
						prevIdx = posW
					}
				}
			}
		}
	}

	return root, align
}

func (l *layout) horizontalCompaction(g *graphlib.Graph, layering [][]string, root map[string]string, align *js.Obj[string], reverseSep bool) *js.Obj[float64] {
	// This portion of the algorithm differs from BK due to a number of
	// problems. Instead of their algorithm we construct a new block graph
	// and do two sweeps. The first sweep places blocks with the smallest
	// possible coordinates. The second sweep removes unused space by moving
	// blocks to the greatest coordinates without violating separation.
	xs := js.NewObj[float64]()
	blockG := buildBlockGraph(g, layering, root, reverseSep)
	borderType := "borderRight"
	if reverseSep {
		borderType = "borderLeft"
	}

	// x is xs[v]. A missing entry is undefined, which is NaN in arithmetic.
	x := func(v string) float64 {
		val, ok := xs.Lookup(v)
		if !ok {
			return nan
		}
		return val
	}

	iterate := func(setXsFunc func(elem string), nextNodesFunc func(v string) []string) {
		stack := blockG.Nodes()
		pop := func() string {
			if len(stack) == 0 {
				return ""
			}
			elem := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			return elem
		}
		elem := pop()
		// visited is a plain object in JavaScript: visited[elem] is truthy
		// for the name of an Object.prototype property.
		visited := map[string]bool{}
		for elem != "" {
			if visited[elem] || isObjectPrototypeKey(elem) {
				setXsFunc(elem)
			} else {
				visited[elem] = true
				stack = append(stack, elem)
				stack = append(stack, nextNodesFunc(elem)...)
			}

			elem = pop()
		}
	}

	// First pass, assign smallest coordinates.
	pass1 := func(elem string) {
		acc := 0.0
		for _, e := range blockG.InEdges(elem) {
			acc = js.Max(acc, x(e.V)+blockEdge(blockG, e))
		}
		xs.Set(elem, acc)
	}

	// Second pass, assign greatest coordinates.
	pass2 := func(elem string) {
		min := math.Inf(1)
		for _, e := range blockG.OutEdges(elem) {
			min = js.Min(min, x(e.W)-blockEdge(blockG, e))
		}

		node := nodeOf(g, elem)
		if !math.IsInf(min, 1) && node.borderType != borderType {
			xs.Set(elem, js.Max(x(elem), min))
		}
	}

	iterate(pass1, blockG.Predecessors)
	iterate(pass2, blockG.Successors)

	// Assign x coordinates to all nodes.
	align.Each(func(_ string, v string) {
		xs.Set(v, x(root[v]))
	})

	return xs
}

// blockEdge returns the label of an edge of the block graph: a separation.
func blockEdge(blockG *graphlib.Graph, e graphlib.Edge) float64 {
	sep, ok := blockG.EdgeObj(e).(float64)
	if !ok {
		return nan
	}
	return sep
}

func buildBlockGraph(g *graphlib.Graph, layering [][]string, root map[string]string, reverseSep bool) *graphlib.Graph {
	blockGraph := graphlib.New(graphlib.Options{})
	graphLabel := graphOf(g)
	sepFn := sep(graphLabel.nodesep, graphLabel.edgesep, reverseSep)

	for _, layer := range layering {
		u := ""
		for _, v := range layer {
			vRoot := root[v]
			blockGraph.SetNode(vRoot)
			if u != "" {
				uRoot := root[u]
				prevMax, _ := blockGraph.Edge(uRoot, vRoot).(float64)
				if !truthy(prevMax) {
					prevMax = 0
				}
				setEdge(blockGraph, uRoot, vRoot, js.Max(sepFn(g, v, u), prevMax), "")
			}
			u = v
		}
	}

	return blockGraph
}

// findSmallestWidthAlignment returns the alignment that has the smallest
// width of the given alignments.
func findSmallestWidthAlignment(g *graphlib.Graph, xss *js.Obj[*js.Obj[float64]]) *js.Obj[float64] {
	result, _ := minBy(xss.Values(), func(xs *js.Obj[float64]) float64 {
		max := math.Inf(-1)
		min := math.Inf(1)

		xs.Each(func(v string, x float64) {
			halfWidth := float64(width(g, v) / 2)

			max = js.Max(x+halfWidth, max)
			min = js.Min(x-halfWidth, min)
		})

		return max - min
	})
	return result
}

// alignCoordinates aligns the coordinates of each of the layout alignments
// such that left-biased alignments have their minimum coordinate at the
// same point as the minimum coordinate of the smallest width alignment and
// right-biased alignments have their maximum coordinate at the same point
// as the maximum coordinate of the smallest width alignment.
func alignCoordinates(xss *js.Obj[*js.Obj[float64]], alignTo *js.Obj[float64]) {
	var alignToVals []float64
	if alignTo != nil {
		alignToVals = alignTo.Values()
	}
	alignToMin := orNaN(lodashMin(alignToVals))
	alignToMax := orNaN(lodashMax(alignToVals))

	for _, vert := range []string{"u", "d"} {
		for _, horiz := range []string{"l", "r"} {
			alignment := vert + horiz
			xs := xss.Get(alignment)
			if xs == alignTo {
				continue
			}

			xsVals := xs.Values()
			var delta float64
			if horiz == "l" {
				delta = alignToMin - orNaN(lodashMin(xsVals))
			} else {
				delta = alignToMax - orNaN(lodashMax(xsVals))
			}

			if truthy(delta) {
				shifted := js.NewObj[float64]()
				xs.Each(func(v string, x float64) {
					shifted.Set(v, x+delta)
				})
				xss.Set(alignment, shifted)
			}
		}
	}
}

func (l *layout) balance(xss *js.Obj[*js.Obj[float64]], align string) *js.Obj[float64] {
	result := js.NewObj[float64]()
	xss.Get("ul").Each(func(v string, _ float64) {
		if align != "" {
			xs := xss.Get(strings.ToLower(align))
			if xs == nil {
				// xss[align.toLowerCase()] is undefined.
				panic(typeError("Cannot read properties of undefined (reading '" + v + "')"))
			}
			x, ok := xs.Lookup(v)
			if !ok {
				x = nan
			}
			result.Set(v, x)
		} else {
			// _.sortBy(_.map(xss, v))
			vals := make([]sortKey, 0, 4)
			xss.Each(func(_ string, xs *js.Obj[float64]) {
				x, ok := xs.Lookup(v)
				vals = append(vals, sortKey{v: x, defined: ok})
			})
			sorted := sortBy(l.state.Sort, vals, func(k sortKey) sortKey { return k })
			a, b := sorted[1], sorted[2]
			if !a.defined {
				a.v = nan
			}
			if !b.defined {
				b.v = nan
			}
			result.Set(v, (a.v+b.v)/2)
		}
	})
	return result
}

func (l *layout) positionX(g *graphlib.Graph) *js.Obj[float64] {
	layering := buildLayerMatrix(g)
	// _.merge(findType1Conflicts(g, layering), findType2Conflicts(g, layering))
	conflicts := conflicts{}
	findType1Conflicts(g, layering, conflicts)
	findType2Conflicts(g, layering, conflicts)

	xss := js.NewObj[*js.Obj[float64]]()
	var adjustedLayering [][]string
	for _, vert := range []string{"u", "d"} {
		if vert == "u" {
			adjustedLayering = layering
		} else {
			adjustedLayering = make([][]string, len(layering))
			for i, layer := range layering {
				adjustedLayering[len(layering)-1-i] = layer
			}
		}
		for _, horiz := range []string{"l", "r"} {
			if horiz == "r" {
				reversed := make([][]string, len(adjustedLayering))
				for i, inner := range adjustedLayering {
					rev := make([]string, len(inner))
					for j, v := range inner {
						rev[len(inner)-1-j] = v
					}
					reversed[i] = rev
				}
				adjustedLayering = reversed
			}

			neighborFn := g.Successors
			if vert == "u" {
				neighborFn = g.Predecessors
			}
			root, align := l.verticalAlignment(adjustedLayering, conflicts, neighborFn)
			xs := l.horizontalCompaction(g, adjustedLayering, root, align, horiz == "r")
			if horiz == "r" {
				negated := js.NewObj[float64]()
				xs.Each(func(v string, x float64) {
					negated.Set(v, -x)
				})
				xs = negated
			}
			xss.Set(vert+horiz, xs)
		}
	}

	smallestWidth := findSmallestWidthAlignment(g, xss)
	alignCoordinates(xss, smallestWidth)
	return l.balance(xss, graphOf(g).align)
}

func sep(nodeSep, edgeSep float64, reverseSep bool) func(g *graphlib.Graph, v, w string) float64 {
	return func(g *graphlib.Graph, v, w string) float64 {
		vLabel := nodeOf(g, v)
		wLabel := nodeOf(g, w)
		sum := 0.0
		// delta is undefined at first, which is falsy as 0 is.
		delta := 0.0

		sum += float64(vLabel.width / 2)
		if vLabel.hasLabelpos {
			switch strings.ToLower(vLabel.labelpos) {
			case "l":
				delta = float64(-vLabel.width / 2)
			case "r":
				delta = float64(vLabel.width / 2)
			}
		}
		if truthy(delta) {
			if reverseSep {
				sum += delta
			} else {
				sum += -delta
			}
		}
		delta = 0

		if vLabel.dummy != "" {
			sum += float64(edgeSep / 2)
		} else {
			sum += float64(nodeSep / 2)
		}
		if wLabel.dummy != "" {
			sum += float64(edgeSep / 2)
		} else {
			sum += float64(nodeSep / 2)
		}

		sum += float64(wLabel.width / 2)
		if wLabel.hasLabelpos {
			switch strings.ToLower(wLabel.labelpos) {
			case "l":
				delta = float64(wLabel.width / 2)
			case "r":
				delta = float64(-wLabel.width / 2)
			}
		}
		if truthy(delta) {
			if reverseSep {
				sum += delta
			} else {
				sum += -delta
			}
		}

		return sum
	}
}

func width(g *graphlib.Graph, v string) float64 {
	return nodeOf(g, v).width
}
