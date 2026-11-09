// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase2.multitree.order.ts (MIT).

package swimlanes

// createChildOrderer creates a function that orders children by crossing
// counts and minimum layers. Children in future layers are ordered by their
// minimum layer, while children in the current layer are ordered by
// crossing counts.
func createChildOrderer(children map[NodeId][]NodeId, rankOf map[NodeId]int, crossCounts map[NodeId]map[NodeId]int, minLayer map[NodeId]int) func(node NodeId) []NodeId {
	return func(node NodeId) []NodeId {
		raw := children[node]
		if len(raw) == 0 {
			return []NodeId{}
		}
		layer := rankOr0(rankOf, node)
		type futureItem struct {
			child NodeId
			min   int
		}
		future := []futureItem{}
		present := []NodeId{}
		crossMap := crossCounts[node]

		// minOr is `minLayer.get(child) ?? layer`.
		minOr := func(child NodeId) int {
			if v, ok := minLayer[child]; ok {
				return v
			}
			return layer
		}

		for _, child := range raw {
			minL := minOr(child)
			if minL > layer {
				future = append(future, futureItem{child: child, min: minL})
			} else {
				present = append(present, child)
			}
		}

		sortFunc(future, func(a, b futureItem) float64 {
			if a.min == b.min {
				return localeCompare(a.child, b.child)
			}
			return float64(a.min - b.min)
		})

		sortFunc(present, func(a, b NodeId) float64 {
			ca := crossMap[a]
			cb := crossMap[b]
			if ca != cb {
				return float64(ca - cb)
			}
			ma := minOr(a)
			mb := minOr(b)
			if ma != mb {
				return float64(ma - mb)
			}
			return localeCompare(a, b)
		})

		out := make([]NodeId, 0, len(raw))
		for _, item := range future {
			out = append(out, item.child)
		}
		return append(out, present...)
	}
}

// buildMultitreeLayerOrder builds a layering (array of layers) by
// traversing the driving tree in a multitree order that tries to minimize
// crossings.
func buildMultitreeLayerOrder(g *Graph, rankOf map[NodeId]int, laneOf func(id NodeId) string) [][]NodeId {
	tree := buildDrivingTree(g, &DrivingTreeBuildOptions{
		RankHint: rankOf,
		LaneOf:   laneOf,
	})
	children, roots := tree.Children, tree.Roots

	// Ensure all nodes have a children entry.
	for _, node := range g.Nodes {
		if _, ok := children[node]; !ok {
			children[node] = []NodeId{}
		}
	}

	crossCounts := computeSubtreeCrossCounts(g, rankOf, tree)

	rootsSorted := append([]NodeId{}, roots...)
	sortFunc(rootsSorted, compareByRankThenId(rankOf))

	// Annotate each node with the minimum layer in its subtree.
	minLayer := annotateMinimumLayers(rootsSorted, children, rankOf)

	// Create a function to order children by crossing counts.
	orderChildren := createChildOrderer(children, rankOf, crossCounts, minLayer)

	// Emit nodes in tree order.
	layers := emitNodesInTreeOrder(rootsSorted, g.Nodes, rankOf, orderChildren)

	// Deduplicate each layer.
	layers = deduplicateLayers(layers)

	return layers
}
