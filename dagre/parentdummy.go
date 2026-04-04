package dagre

// ParentDummyChains sets the parent of dummy nodes in long-edge chains
// based on the lowest common ancestor of the edge endpoints.
func ParentDummyChains(g *Graph) {
	postorderNums := computePostorderNums(g)

	for _, v := range g.label.DummyChains {
		node := g.Node(v)
		if node == nil || node.EdgeObj == nil {
			continue
		}
		edgeObj := *node.EdgeObj
		pathData := findPath(g, postorderNums, edgeObj.V, edgeObj.W)
		path := pathData.path
		lca := pathData.lca
		pathIdx := 0
		ascending := true
		current := v

		for current != edgeObj.W {
			node = g.Node(current)
			if node == nil {
				break
			}

			if ascending {
				for pathIdx < len(path) {
					pathV := path[pathIdx]
					if pathV == lca {
						break
					}
					pvNode := g.Node(pathV)
					if pvNode != nil && pvNode.HasMaxRank && pvNode.MaxRank < node.Rank {
						pathIdx++
					} else {
						break
					}
				}
				if pathIdx < len(path) && path[pathIdx] == lca {
					ascending = false
				}
			}

			if !ascending {
				for pathIdx < len(path)-1 {
					nextNode := g.Node(path[pathIdx+1])
					if nextNode != nil && nextNode.HasMinRank && nextNode.MinRank <= node.Rank {
						pathIdx++
					} else {
						break
					}
				}
			}

			if pathIdx < len(path) && path[pathIdx] != "" {
				g.SetParent(current, path[pathIdx])
			}

			succs := g.Successors(current)
			if len(succs) == 0 {
				break
			}
			current = succs[0]
		}
	}
}

type postorderNum struct {
	low int
	lim int
}

type pathData struct {
	path []string
	lca  string
}

func computePostorderNums(g *Graph) map[string]postorderNum {
	result := make(map[string]postorderNum)
	lim := 0

	var dfs func(string)
	dfs = func(v string) {
		low := lim
		for _, child := range g.Children(v) {
			dfs(child)
		}
		result[v] = postorderNum{low: low, lim: lim}
		lim++
	}

	for _, v := range g.Children("") {
		dfs(v)
	}
	return result
}

func findPath(g *Graph, nums map[string]postorderNum, v, w string) pathData {
	var vPath []string
	var wPath []string

	vNum := nums[v]
	wNum := nums[w]
	low := vNum.low
	if wNum.low < low {
		low = wNum.low
	}
	lim := vNum.lim
	if wNum.lim > lim {
		lim = wNum.lim
	}

	// Traverse up from v
	parent := v
	for {
		parent = g.Parent(parent)
		if parent == "" {
			break
		}
		vPath = append(vPath, parent)
		pNum := nums[parent]
		if pNum.low <= low && lim <= pNum.lim {
			break
		}
	}
	lca := parent

	// Traverse up from w
	wParent := w
	for {
		wParent = g.Parent(wParent)
		if wParent == "" || wParent == lca {
			break
		}
		wPath = append(wPath, wParent)
	}

	// Reverse wPath and concatenate
	for i, j := 0, len(wPath)-1; i < j; i, j = i+1, j-1 {
		wPath[i], wPath[j] = wPath[j], wPath[i]
	}

	path := append(vPath, wPath...)
	return pathData{path: path, lca: lca}
}
