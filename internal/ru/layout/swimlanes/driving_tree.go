// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/driving-tree.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// DrivingTreeBlock is a biconnected block of the graph.
type DrivingTreeBlock struct {
	ID    int
	Nodes []NodeId
	Edges [][2]NodeId
}

// DrivingTree is the spanning forest and the traversal metadata that the
// tree ordering uses. Parent has "" for a root (null upstream).
type DrivingTree struct {
	Parent           map[NodeId]NodeId
	Children         map[NodeId][]NodeId
	Roots            []NodeId
	ComponentOf      map[NodeId]int
	Blocks           []*DrivingTreeBlock
	NodeBlocks       map[NodeId][]int
	Adjacency        *js.Map[NodeId, []NodeId]
	Preorder         []NodeId
	Postorder        []NodeId
	TopologicalOrder []NodeId
}

// DrivingTreeBuildOptions are the options of buildDrivingTree. RankHint and
// LaneOf can be nil.
type DrivingTreeBuildOptions struct {
	RankHint map[NodeId]int
	LaneOf   func(id NodeId) string
}

// buildDrivingTree builds the spanning forest and traversal metadata used
// by tree ordering.
func buildDrivingTree(graph *Graph, opts *DrivingTreeBuildOptions) *DrivingTree {
	g := normalizeGraph(graph)
	laneOf := func(NodeId) string { return "" }
	var rankHint map[NodeId]int
	if opts != nil {
		if opts.LaneOf != nil {
			laneOf = opts.LaneOf
		}
		rankHint = opts.RankHint
	}

	preds, _ := buildPredecessorSuccessorMaps(g, nil)
	for _, arr := range preds {
		sortFunc(arr, func(a, b NodeId) float64 { return localeCompare(a, b) })
	}

	topoOrder := topoSortIfAcyclic(g)
	if topoOrder == nil {
		topoOrder = append([]NodeId{}, g.Nodes...)
		sortFunc(topoOrder, func(a, b NodeId) float64 { return localeCompare(a, b) })
	}
	topoIndex := make(map[NodeId]int, len(topoOrder))
	for idx, id := range topoOrder {
		topoIndex[id] = idx
	}

	parent := map[NodeId]NodeId{}
	children := make(map[NodeId][]NodeId, len(g.Nodes))
	for _, node := range g.Nodes {
		children[node] = []NodeId{}
	}

	for _, node := range topoOrder {
		var candidates []NodeId
		for _, p := range preds[node] {
			if _, ok := parent[p]; ok {
				candidates = append(candidates, p)
			}
		}
		if len(candidates) > 0 {
			chosen := chooseParent(node, candidates, &parentSelectionContext{
				laneOf:    laneOf,
				rankHint:  rankHint,
				topoIndex: topoIndex,
			})
			parent[node] = chosen
			children[chosen] = append(children[chosen], node)
		} else if _, ok := parent[node]; !ok {
			parent[node] = ""
		}
	}

	for _, node := range g.Nodes {
		if _, ok := parent[node]; !ok {
			parent[node] = ""
		}
	}

	rootSet := []NodeId{}
	for _, node := range g.Nodes {
		if parent[node] == "" {
			rootSet = append(rootSet, node)
		}
	}
	roots := rootSet
	sortFunc(roots, func(a, b NodeId) float64 {
		ta := topoIndex[a]
		tb := topoIndex[b]
		if ta == tb {
			return localeCompare(a, b)
		}
		return float64(ta - tb)
	})

	adjacency := buildAdjacency(g)
	adjacencyList := js.NewMap[NodeId, []NodeId]()
	adjacency.Each(func(node NodeId, set *js.Set[NodeId]) {
		list := set.Values()
		sortFunc(list, func(a, b NodeId) float64 { return localeCompare(a, b) })
		adjacencyList.Set(node, list)
	})

	componentOf := assignComponents(adjacencyList)
	blocks := computeBlocks(adjacencyList)
	nodeBlocks := make(map[NodeId][]int, len(g.Nodes))
	for _, node := range g.Nodes {
		nodeBlocks[node] = []int{}
	}
	for _, block := range blocks {
		for _, node := range block.Nodes {
			nodeBlocks[node] = append(nodeBlocks[node], block.ID)
		}
	}

	preorder := []NodeId{}
	postorder := []NodeId{}
	seen := map[NodeId]bool{}

	var walk func(node NodeId)
	walk = func(node NodeId) {
		if seen[node] {
			return
		}
		seen[node] = true
		preorder = append(preorder, node)
		for _, child := range children[node] {
			walk(child)
		}
		postorder = append(postorder, node)
	}

	for _, root := range roots {
		walk(root)
	}
	for _, node := range topoOrder {
		walk(node)
	}

	return &DrivingTree{
		Parent:           parent,
		Children:         children,
		Roots:            roots,
		ComponentOf:      componentOf,
		Blocks:           blocks,
		NodeBlocks:       nodeBlocks,
		Adjacency:        adjacencyList,
		Preorder:         preorder,
		Postorder:        postorder,
		TopologicalOrder: topoOrder,
	}
}

type parentSelectionContext struct {
	laneOf    func(id NodeId) string
	rankHint  map[NodeId]int
	topoIndex map[NodeId]int
}

func chooseParent(node NodeId, candidates []NodeId, ctx *parentSelectionContext) NodeId {
	laneNode := ctx.laneOf(node)
	sorted := append([]NodeId{}, candidates...)
	sortFunc(sorted, func(a, b NodeId) float64 {
		laneA := ctx.laneOf(a)
		laneB := ctx.laneOf(b)
		sameLaneA := laneA != "" && laneA == laneNode
		sameLaneB := laneB != "" && laneB == laneNode
		if sameLaneA != sameLaneB {
			if sameLaneA {
				return -1
			}
			return 1
		}

		rankA, hasRankA := ctx.rankHint[a]
		rankB, hasRankB := ctx.rankHint[b]
		if hasRankA && hasRankB && rankA != rankB {
			return float64(rankB - rankA)
		}

		idxA := ctx.topoIndex[a]
		idxB := ctx.topoIndex[b]
		if idxA != idxB {
			return float64(idxA - idxB)
		}

		return localeCompare(a, b)
	})
	return sorted[0]
}

func buildAdjacency(g *Graph) *js.Map[NodeId, *js.Set[NodeId]] {
	adjacency := js.NewMap[NodeId, *js.Set[NodeId]]()
	for _, node := range g.Nodes {
		adjacency.Set(node, js.NewSet[NodeId]())
	}
	for _, e := range g.Edges {
		adjacency.Get(e.Src).Add(e.Dst)
		adjacency.Get(e.Dst).Add(e.Src)
	}
	return adjacency
}

func assignComponents(adjacency *js.Map[NodeId, []NodeId]) map[NodeId]int {
	componentOf := map[NodeId]int{}
	componentId := 0
	for _, node := range adjacency.Keys() {
		if _, ok := componentOf[node]; ok {
			continue
		}
		stack := []NodeId{node}
		for len(stack) > 0 {
			cur := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if _, ok := componentOf[cur]; ok {
				continue
			}
			componentOf[cur] = componentId
			for _, next := range adjacency.Get(cur) {
				if _, ok := componentOf[next]; !ok {
					stack = append(stack, next)
				}
			}
		}
		componentId++
	}
	return componentOf
}

func computeBlocks(adjacency *js.Map[NodeId, []NodeId]) []*DrivingTreeBlock {
	discovery := map[NodeId]int{}
	low := map[NodeId]int{}
	edgeStack := [][2]NodeId{}
	blocks := []*DrivingTreeBlock{}
	time := 0

	// lowOr is `low.get(node) ?? time`, and discoveryOr is
	// `discovery.get(node) ?? d`.
	lowOr := func(node NodeId) int {
		if v, ok := low[node]; ok {
			return v
		}
		return time
	}
	discoveryOr := func(node NodeId, d int) int {
		if v, ok := discovery[node]; ok {
			return v
		}
		return d
	}

	// parent is "" for null, with hasParent false.
	var visit func(node NodeId, parent NodeId, hasParent bool)
	visit = func(node NodeId, parent NodeId, hasParent bool) {
		time++
		discovery[node] = time
		low[node] = time

		for _, next := range adjacency.Get(node) {
			if hasParent && next == parent {
				continue
			}
			if _, ok := discovery[next]; !ok {
				edgeStack = append(edgeStack, [2]NodeId{node, next})
				visit(next, node, true)
				low[node] = min(lowOr(node), lowOr(next))
				if low[next] >= discovery[node] {
					blocks = append(blocks, popBlock(node, next, &edgeStack, len(blocks)))
				}
			} else if discovery[next] < discovery[node] {
				edgeStack = append(edgeStack, [2]NodeId{node, next})
				low[node] = min(lowOr(node), discoveryOr(next, time))
			}
		}
	}

	for _, node := range adjacency.Keys() {
		if _, ok := discovery[node]; !ok {
			visit(node, "", false)
		}
	}

	return blocks
}

func popBlock(u, v NodeId, stack *[][2]NodeId, id int) *DrivingTreeBlock {
	edges := [][2]NodeId{}
	nodes := js.NewSet[NodeId]()
	for len(*stack) > 0 {
		edge := (*stack)[len(*stack)-1]
		*stack = (*stack)[:len(*stack)-1]
		edges = append(edges, edge)
		nodes.Add(edge[0])
		nodes.Add(edge[1])
		if (edge[0] == u && edge[1] == v) || (edge[0] == v && edge[1] == u) {
			break
		}
	}
	return &DrivingTreeBlock{ID: id, Edges: edges, Nodes: nodes.Values()}
}
