package dagre

import (
	"math"
	"sort"
)

// Order applies heuristics to minimize edge crossings and assigns order to nodes.
func Order(g *Graph) {
	maxR := MaxRank(g)
	downLayerGraphs := buildLayerGraphs(g, RangeInts(1, maxR+1), "inEdges")
	upLayerGraphs := buildLayerGraphs(g, RangeIntsStep(maxR-1, -1, -1), "outEdges")

	layering := initOrder(g)
	assignOrder(g, layering)

	bestCC := math.MaxFloat64
	var best [][]string

	for i, lastBest := 0, 0; lastBest < 4; i, lastBest = i+1, lastBest+1 {
		var lgs []*Graph
		if i%2 != 0 {
			lgs = downLayerGraphs
		} else {
			lgs = upLayerGraphs
		}
		sweepLayerGraphs(lgs, i%4 >= 2)

		layering = BuildLayerMatrix(g)
		cc := float64(crossCount(g, layering))
		if cc < bestCC {
			lastBest = 0
			best = deepCopyLayers(layering)
			bestCC = cc
		}
	}

	if best != nil {
		assignOrder(g, best)
	}
}

func deepCopyLayers(layers [][]string) [][]string {
	result := make([][]string, len(layers))
	for i, layer := range layers {
		result[i] = make([]string, len(layer))
		copy(result[i], layer)
	}
	return result
}

func initOrder(g *Graph) [][]string {
	visited := make(map[string]bool)
	simpleNodes := []string{}
	for _, v := range g.Nodes() {
		if len(g.Children(v)) == 0 {
			simpleNodes = append(simpleNodes, v)
		}
	}

	maxR := 0
	for _, v := range simpleNodes {
		n := g.Node(v)
		if n.HasRank && n.Rank > maxR {
			maxR = n.Rank
		}
	}

	layers := make([][]string, maxR+1)
	for i := range layers {
		layers[i] = []string{}
	}

	// Sort by rank
	sort.Slice(simpleNodes, func(i, j int) bool {
		return g.Node(simpleNodes[i]).Rank < g.Node(simpleNodes[j]).Rank
	})

	var dfs func(string)
	dfs = func(v string) {
		if visited[v] {
			return
		}
		visited[v] = true
		n := g.Node(v)
		if n.HasRank && n.Rank >= 0 && n.Rank <= maxR {
			layers[n.Rank] = append(layers[n.Rank], v)
		}
		for _, w := range g.Successors(v) {
			dfs(w)
		}
	}

	for _, v := range simpleNodes {
		dfs(v)
	}

	return layers
}

func assignOrder(g *Graph, layering [][]string) {
	for _, layer := range layering {
		for i, v := range layer {
			n := g.Node(v)
			if n != nil {
				n.Order = i
				n.HasOrder = true
			}
		}
	}
}

func buildLayerGraphs(g *Graph, ranks []int, relationship string) []*Graph {
	// Build index: rank -> nodes
	nodesByRank := make(map[int][]string)
	for _, v := range g.Nodes() {
		n := g.Node(v)
		if n.HasRank {
			nodesByRank[n.Rank] = append(nodesByRank[n.Rank], v)
		}
		if n.HasMinRank && n.HasMaxRank {
			for r := n.MinRank; r <= n.MaxRank; r++ {
				if r != n.Rank {
					nodesByRank[r] = append(nodesByRank[r], v)
				}
			}
		}
	}

	result := make([]*Graph, len(ranks))
	for i, rank := range ranks {
		result[i] = buildLayerGraph(g, rank, relationship, nodesByRank[rank])
	}
	return result
}

func buildLayerGraph(g *Graph, rank int, relationship string, nodesWithRank []string) *Graph {
	root := UniqueID("_root")
	result := NewGraph()
	result.label.NestingRoot = root // store root in graph

	for _, v := range nodesWithRank {
		n := g.Node(v)
		if n.HasRank && n.Rank == rank || (n.HasMinRank && n.MinRank <= rank && rank <= n.MaxRank) {
			result.SetNode(v, n)
			parent := g.Parent(v)
			if parent != "" {
				result.SetParent(v, parent)
			} else {
				result.SetParent(v, root)
			}

			var edges []Edge
			if relationship == "inEdges" {
				edges = g.InEdges(v)
			} else {
				edges = g.OutEdges(v)
			}

			for _, e := range edges {
				u := e.V
				if u == v {
					u = e.W
				}
				existing := result.EdgeVW(u, v)
				w := 0
				if existing != nil {
					w = existing.Weight
				}
				result.SetEdgeVW(u, v, &EdgeLabel{Weight: g.EdgeByKey(e).Weight + w, MinLen: 1})
			}

			if n.HasMinRank {
				// compound node spanning ranks
				blKey := ""
				brKey := ""
				if n.BorderLeft != nil && rank < len(n.BorderLeft) {
					blKey = n.BorderLeft[rank]
				}
				if n.BorderRight != nil && rank < len(n.BorderRight) {
					brKey = n.BorderRight[rank]
				}
				compoundLabel := &NodeLabel{
					Width:   n.Width,
					Height:  n.Height,
					HasRank: true,
					Rank:    rank,
				}
				if blKey != "" {
					compoundLabel.BorderTop = blKey
				}
				if brKey != "" {
					compoundLabel.BorderBot = brKey
				}
				result.SetNode(v, compoundLabel)
			}
		}
	}

	return result
}

func sweepLayerGraphs(layerGraphs []*Graph, biasRight bool) {
	cg := NewSimpleGraph()
	for _, lg := range layerGraphs {
		root := lg.label.NestingRoot
		sorted := sortSubgraph(lg, root, cg, biasRight)
		for i, v := range sorted.vs {
			n := lg.Node(v)
			if n != nil {
				n.Order = i
				n.HasOrder = true
			}
		}
		addSubgraphConstraints(lg, cg, sorted.vs)
	}
}

// --- Barycenter ---

type barycenterEntry struct {
	v          string
	barycenter float64
	weight     float64
	hasBC      bool
}

func barycenter(g *Graph, movable []string) []barycenterEntry {
	result := make([]barycenterEntry, len(movable))
	for i, v := range movable {
		inEdges := g.InEdges(v)
		if len(inEdges) == 0 {
			result[i] = barycenterEntry{v: v}
			continue
		}
		sumVal := 0.0
		weightVal := 0.0
		for _, e := range inEdges {
			el := g.EdgeByKey(e)
			nu := g.Node(e.V)
			order := 0
			if nu != nil && nu.HasOrder {
				order = nu.Order
			}
			sumVal += float64(el.Weight) * float64(order)
			weightVal += float64(el.Weight)
		}
		result[i] = barycenterEntry{
			v:          v,
			barycenter: sumVal / weightVal,
			weight:     weightVal,
			hasBC:      true,
		}
	}
	return result
}

// --- Sort subgraph ---

type subgraphResult struct {
	vs         []string
	barycenter float64
	weight     float64
	hasBC      bool
}

func sortSubgraph(g *Graph, v string, cg *Graph, biasRight bool) subgraphResult {
	movable := g.Children(v)
	node := g.Node(v)
	bl := ""
	br := ""
	if node != nil {
		bl = node.BorderTop // used as borderLeft in layer graph context
		br = node.BorderBot // used as borderRight
	}

	subgraphs := make(map[string]subgraphResult)

	if bl != "" {
		newMovable := make([]string, 0, len(movable))
		for _, w := range movable {
			if w != bl && w != br {
				newMovable = append(newMovable, w)
			}
		}
		movable = newMovable
	}

	bcs := barycenter(g, movable)
	for i := range bcs {
		entry := &bcs[i]
		children := g.Children(entry.v)
		if len(children) > 0 {
			sub := sortSubgraph(g, entry.v, cg, biasRight)
			subgraphs[entry.v] = sub
			if sub.hasBC {
				mergeBarycenters(entry, sub)
			}
		}
	}

	entries := resolveConflicts(bcs, cg)
	expandSubgraphs(entries, subgraphs)

	result := sortEntries(entries, biasRight)

	if bl != "" && br != "" {
		vs := make([]string, 0, len(result.vs)+2)
		vs = append(vs, bl)
		vs = append(vs, result.vs...)
		vs = append(vs, br)
		result.vs = vs
	}

	return result
}

func mergeBarycenters(target *barycenterEntry, other subgraphResult) {
	if target.hasBC {
		target.barycenter = (target.barycenter*target.weight + other.barycenter*other.weight) / (target.weight + other.weight)
		target.weight += other.weight
	} else {
		target.barycenter = other.barycenter
		target.weight = other.weight
		target.hasBC = true
	}
}

// --- Resolve conflicts ---

type resolvedEntry struct {
	vs         []string
	i          int
	barycenter float64
	weight     float64
	hasBC      bool
}

type mappedEntry struct {
	indegree int
	in       []*mappedEntry
	out      []*mappedEntry
	vs       []string
	i        int
	barycenter float64
	weight   float64
	hasBC    bool
	merged   bool
}

func resolveConflicts(entries []barycenterEntry, cg *Graph) []resolvedEntry {
	mapped := make(map[string]*mappedEntry)
	for i, entry := range entries {
		me := &mappedEntry{
			vs:   []string{entry.v},
			i:    i,
			hasBC: entry.hasBC,
			barycenter: entry.barycenter,
			weight: entry.weight,
		}
		mapped[entry.v] = me
	}

	for _, e := range cg.Edges() {
		entryV := mapped[e.V]
		entryW := mapped[e.W]
		if entryV != nil && entryW != nil {
			entryW.indegree++
			entryV.out = append(entryV.out, entryW)
		}
	}

	var sourceSet []*mappedEntry
	for _, me := range mapped {
		if me.indegree == 0 {
			sourceSet = append(sourceSet, me)
		}
	}

	return doResolveConflicts(sourceSet)
}

func doResolveConflicts(sourceSet []*mappedEntry) []resolvedEntry {
	var entries []*mappedEntry

	for len(sourceSet) > 0 {
		entry := sourceSet[len(sourceSet)-1]
		sourceSet = sourceSet[:len(sourceSet)-1]
		entries = append(entries, entry)

		// handle in (reverse order)
		for i := len(entry.in) - 1; i >= 0; i-- {
			uEntry := entry.in[i]
			if uEntry.merged {
				continue
			}
			if !uEntry.hasBC || !entry.hasBC || uEntry.barycenter >= entry.barycenter {
				mergeEntries(entry, uEntry)
			}
		}

		// handle out
		for _, wEntry := range entry.out {
			wEntry.in = append(wEntry.in, entry)
			wEntry.indegree--
			if wEntry.indegree == 0 {
				sourceSet = append(sourceSet, wEntry)
			}
		}
	}

	var result []resolvedEntry
	for _, e := range entries {
		if !e.merged {
			result = append(result, resolvedEntry{
				vs:         e.vs,
				i:          e.i,
				barycenter: e.barycenter,
				weight:     e.weight,
				hasBC:      e.hasBC,
			})
		}
	}
	return result
}

func mergeEntries(target, source *mappedEntry) {
	sum := 0.0
	weight := 0.0
	if target.weight > 0 {
		sum += target.barycenter * target.weight
		weight += target.weight
	}
	if source.weight > 0 {
		sum += source.barycenter * source.weight
		weight += source.weight
	}
	target.vs = append(source.vs, target.vs...)
	target.barycenter = sum / weight
	target.weight = weight
	target.hasBC = true
	if source.i < target.i {
		target.i = source.i
	}
	source.merged = true
}

func expandSubgraphs(entries []resolvedEntry, subgraphs map[string]subgraphResult) {
	for i := range entries {
		var newVs []string
		for _, v := range entries[i].vs {
			if sg, ok := subgraphs[v]; ok {
				newVs = append(newVs, sg.vs...)
			} else {
				newVs = append(newVs, v)
			}
		}
		entries[i].vs = newVs
	}
}

func sortEntries(entries []resolvedEntry, biasRight bool) subgraphResult {
	sortable, unsortable := Partition(entries, func(e resolvedEntry) bool { return e.hasBC })

	sort.Slice(sortable, func(i, j int) bool {
		if sortable[i].barycenter < sortable[j].barycenter {
			return true
		}
		if sortable[i].barycenter > sortable[j].barycenter {
			return false
		}
		if !biasRight {
			return sortable[i].i < sortable[j].i
		}
		return sortable[i].i > sortable[j].i
	})

	// Sort unsortable by i descending
	sort.Slice(unsortable, func(i, j int) bool {
		return unsortable[i].i > unsortable[j].i
	})

	var vs []string
	sumVal := 0.0
	weight := 0.0
	vsIndex := 0

	// consume unsortable at start
	vsIndex = consumeUnsortable(&vs, &unsortable, vsIndex)

	for _, entry := range sortable {
		vsIndex += len(entry.vs)
		vs = append(vs, entry.vs...)
		sumVal += entry.barycenter * entry.weight
		weight += entry.weight
		vsIndex = consumeUnsortable(&vs, &unsortable, vsIndex)
	}

	result := subgraphResult{vs: vs}
	if weight > 0 {
		result.barycenter = sumVal / weight
		result.weight = weight
		result.hasBC = true
	}
	return result
}

func consumeUnsortable(vs *[]string, unsortable *[]resolvedEntry, index int) int {
	for len(*unsortable) > 0 {
		last := (*unsortable)[len(*unsortable)-1]
		if last.i > index {
			break
		}
		*unsortable = (*unsortable)[:len(*unsortable)-1]
		*vs = append(*vs, last.vs...)
		index++
	}
	return index
}

// --- Cross count ---

func crossCount(g *Graph, layering [][]string) int {
	cc := 0
	for i := 1; i < len(layering); i++ {
		cc += twoLayerCrossCount(g, layering[i-1], layering[i])
	}
	return cc
}

type southEntry struct {
	pos    int
	weight int
}

func twoLayerCrossCount(g *Graph, northLayer, southLayer []string) int {
	southPos := make(map[string]int)
	for i, v := range southLayer {
		southPos[v] = i
	}

	var entries []southEntry
	for _, v := range northLayer {
		outEdges := g.OutEdges(v)
		for _, e := range outEdges {
			if pos, ok := southPos[e.W]; ok {
				entries = append(entries, southEntry{pos: pos, weight: g.EdgeByKey(e).Weight})
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].pos < entries[j].pos
	})

	firstIndex := 1
	for firstIndex < len(southLayer) {
		firstIndex <<= 1
	}
	treeSize := 2*firstIndex - 1
	firstIndex--
	tree := make([]int, treeSize)

	cc := 0
	for _, entry := range entries {
		index := entry.pos + firstIndex
		if index < treeSize {
			tree[index] += entry.weight
		}
		weightSum := 0
		for index > 0 {
			if index%2 != 0 {
				if index+1 < treeSize {
					weightSum += tree[index+1]
				}
			}
			index = (index - 1) >> 1
			tree[index] += entry.weight
		}
		cc += entry.weight * weightSum
	}

	return cc
}

// --- Add subgraph constraints ---

func addSubgraphConstraints(g, cg *Graph, vs []string) {
	prev := make(map[string]string)
	rootPrev := ""

	for _, v := range vs {
		child := g.Parent(v)
		for child != "" {
			parent := g.Parent(child)
			var prevChild string
			if parent != "" {
				prevChild = prev[parent]
				prev[parent] = child
			} else {
				prevChild = rootPrev
				rootPrev = child
			}
			if prevChild != "" && prevChild != child {
				cg.SetEdgeVW(prevChild, child, &EdgeLabel{MinLen: 1})
				return
			}
			child = parent
		}
	}
}
