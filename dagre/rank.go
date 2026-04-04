package dagre

import "math"

// Rank assigns ranks to nodes using the network simplex algorithm.
func Rank(g *Graph) {
	switch g.label.Ranker {
	case "tight-tree":
		tightTreeRanker(g)
	case "longest-path":
		longestPath(g)
	case "none":
		// no-op
	default: // "network-simplex" or unset
		networkSimplexRanker(g)
	}
}

func longestPath(g *Graph) {
	visited := make(map[string]bool)

	var dfs func(string) int
	dfs = func(v string) int {
		n := g.Node(v)
		if visited[v] {
			return n.Rank
		}
		visited[v] = true

		outEdges := g.OutEdges(v)
		if len(outEdges) == 0 {
			n.Rank = 0
			n.HasRank = true
			return 0
		}

		minRank := math.MaxInt64
		for _, e := range outEdges {
			el := g.EdgeByKey(e)
			r := dfs(e.W) - el.MinLen
			if r < minRank {
				minRank = r
			}
		}

		n.Rank = minRank
		n.HasRank = true
		return minRank
	}

	for _, v := range g.Sources() {
		dfs(v)
	}
}

func tightTreeRanker(g *Graph) {
	longestPath(g)
	feasibleTree(g)
}

func networkSimplexRanker(g *Graph) {
	networkSimplex(g)
}

// Slack returns the slack (rank difference minus minlen) for an edge.
func slack(g *Graph, e Edge) int {
	return g.Node(e.W).Rank - g.Node(e.V).Rank - g.EdgeByKey(e).MinLen
}

// --- Feasible Tree ---

type treeNodeLabel struct {
	Low    int
	Lim    int
	Parent string
	HasParent bool
}

type treeEdgeLabel struct {
	Cutvalue    int
	HasCutvalue bool
}

// feasibleTree builds a spanning tree of tight edges and adjusts ranks.
func feasibleTree(g *Graph) *Graph {
	tree := NewUndirectedGraph()

	nodes := g.Nodes()
	if len(nodes) == 0 {
		return tree
	}
	start := nodes[0]
	tree.SetNode(start, &NodeLabel{})

	for tightTreeDFS(tree, g) < g.NodeCount() {
		e, found := findMinSlackEdge(tree, g)
		if !found {
			break
		}
		var delta int
		if tree.HasNode(e.V) {
			delta = slack(g, e)
		} else {
			delta = -slack(g, e)
		}
		shiftRanks(tree, g, delta)
	}

	return tree
}

func tightTreeDFS(tree, g *Graph) int {
	var dfs func(string)
	dfs = func(v string) {
		for _, e := range g.NodeEdges(v) {
			w := e.W
			if w == v {
				w = e.V
			}
			if !tree.HasNode(w) && slack(g, e) == 0 {
				tree.SetNode(w, &NodeLabel{})
				tree.SetEdgeVW(v, w, &EdgeLabel{})
				dfs(w)
			}
		}
	}
	for _, v := range tree.Nodes() {
		dfs(v)
	}
	return tree.NodeCount()
}

func findMinSlackEdge(tree, g *Graph) (Edge, bool) {
	bestSlack := math.MaxInt64
	var bestEdge Edge
	found := false
	for _, e := range g.Edges() {
		if tree.HasNode(e.V) != tree.HasNode(e.W) {
			s := slack(g, e)
			if s < 0 {
				s = -s
			}
			if s < bestSlack {
				bestSlack = s
				bestEdge = e
				found = true
			}
		}
	}
	return bestEdge, found
}

func shiftRanks(tree, g *Graph, delta int) {
	for _, v := range tree.Nodes() {
		n := g.Node(v)
		if n != nil {
			n.Rank += delta
		}
	}
}

// --- Network Simplex ---

// treeGraph wraps the spanning tree with separate node/edge label maps.
type treeGraph struct {
	g          *Graph
	nodeLabels map[string]*treeNodeLabel
	edgeLabels map[string]*treeEdgeLabel
}

func newTreeGraph(g *Graph) *treeGraph {
	return &treeGraph{
		g:          g,
		nodeLabels: make(map[string]*treeNodeLabel),
		edgeLabels: make(map[string]*treeEdgeLabel),
	}
}

func (t *treeGraph) nodeLabel(v string) *treeNodeLabel {
	if l, ok := t.nodeLabels[v]; ok {
		return l
	}
	l := &treeNodeLabel{}
	t.nodeLabels[v] = l
	return l
}

func (t *treeGraph) edgeLabel(v, w string) *treeEdgeLabel {
	k := v + "\x00" + w
	if l, ok := t.edgeLabels[k]; ok {
		return l
	}
	// try reverse for undirected
	k2 := w + "\x00" + v
	if l, ok := t.edgeLabels[k2]; ok {
		return l
	}
	l := &treeEdgeLabel{}
	t.edgeLabels[k] = l
	return l
}

func (t *treeGraph) setEdgeLabel(v, w string, l *treeEdgeLabel) {
	k := v + "\x00" + w
	t.edgeLabels[k] = l
}

func networkSimplex(g *Graph) {
	sg := Simplify(g)
	longestPath(sg)

	tree := feasibleTree(sg)
	tg := newTreeGraph(tree)

	initLowLimValues(tg, "")
	initCutValues(tg, sg)

	for {
		e, found := leaveEdge(tg)
		if !found {
			break
		}
		f := enterEdge(tg, sg, e)
		exchangeEdges(tg, sg, e, f)
	}

	// Copy ranks back to original graph
	for _, v := range sg.Nodes() {
		n := g.Node(v)
		if n != nil {
			sn := sg.Node(v)
			n.Rank = sn.Rank
			n.HasRank = true
		}
	}
}

func initCutValues(t *treeGraph, g *Graph) {
	visited := Postorder(t.g, t.g.Nodes())
	if len(visited) > 0 {
		visited = visited[:len(visited)-1] // skip root
	}
	for _, v := range visited {
		assignCutValue(t, g, v)
	}
}

func assignCutValue(t *treeGraph, g *Graph, child string) {
	childLab := t.nodeLabel(child)
	if !childLab.HasParent {
		return
	}
	parent := childLab.Parent
	el := t.edgeLabel(child, parent)
	el.Cutvalue = calcCutValue(t, g, child)
	el.HasCutvalue = true
}

func calcCutValue(t *treeGraph, g *Graph, child string) int {
	childLab := t.nodeLabel(child)
	parent := childLab.Parent

	childIsTail := true
	graphEdge := g.EdgeVW(child, parent)
	if graphEdge == nil {
		childIsTail = false
		graphEdge = g.EdgeVW(parent, child)
	}
	if graphEdge == nil {
		return 0
	}

	cutValue := graphEdge.Weight

	for _, edge := range g.NodeEdges(child) {
		isOutEdge := edge.V == child
		other := edge.W
		if !isOutEdge {
			other = edge.V
		}

		if other != parent {
			pointsToHead := isOutEdge == childIsTail
			otherWeight := g.EdgeByKey(edge).Weight

			if pointsToHead {
				cutValue += otherWeight
			} else {
				cutValue -= otherWeight
			}

			if isTreeEdge(t, child, other) {
				treeEdge := t.edgeLabel(child, other)
				otherCutValue := treeEdge.Cutvalue
				if pointsToHead {
					cutValue -= otherCutValue
				} else {
					cutValue += otherCutValue
				}
			}
		}
	}

	return cutValue
}

func isTreeEdge(t *treeGraph, u, v string) bool {
	return t.g.HasEdge(u, v) || t.g.HasEdge(v, u)
}

func initLowLimValues(t *treeGraph, root string) {
	if root == "" {
		nodes := t.g.Nodes()
		if len(nodes) == 0 {
			return
		}
		root = nodes[0]
	}
	dfsAssignLowLim(t, make(map[string]bool), 1, root, "")
}

func dfsAssignLowLim(t *treeGraph, visited map[string]bool, nextLim int, v, parent string) int {
	low := nextLim
	label := t.nodeLabel(v)

	visited[v] = true
	for _, w := range t.g.Neighbors(v) {
		if !visited[w] {
			nextLim = dfsAssignLowLim(t, visited, nextLim, w, v)
		}
	}

	label.Low = low
	label.Lim = nextLim
	nextLim++
	if parent != "" {
		label.Parent = parent
		label.HasParent = true
	} else {
		label.HasParent = false
	}

	return nextLim
}

func leaveEdge(t *treeGraph) (Edge, bool) {
	for _, e := range t.g.Edges() {
		el := t.edgeLabel(e.V, e.W)
		if el.HasCutvalue && el.Cutvalue < 0 {
			return e, true
		}
	}
	return Edge{}, false
}

func enterEdge(t *treeGraph, g *Graph, edge Edge) Edge {
	v := edge.V
	w := edge.W

	if !g.HasEdge(v, w) {
		v, w = w, v
	}

	vLabel := t.nodeLabel(v)
	wLabel := t.nodeLabel(w)
	tailLabel := vLabel
	flip := false

	if vLabel.Lim > wLabel.Lim {
		tailLabel = wLabel
		flip = true
	}

	bestSlack := math.MaxInt64
	var bestEdge Edge

	for _, e := range g.Edges() {
		evLabel := t.nodeLabel(e.V)
		ewLabel := t.nodeLabel(e.W)
		evDesc := isDescendant(evLabel, tailLabel)
		ewDesc := isDescendant(ewLabel, tailLabel)
		if flip == evDesc && flip != ewDesc {
			s := slack(g, e)
			if s < bestSlack {
				bestSlack = s
				bestEdge = e
			}
		}
	}

	return bestEdge
}

func isDescendant(vLabel, rootLabel *treeNodeLabel) bool {
	return rootLabel.Low <= vLabel.Lim && vLabel.Lim <= rootLabel.Lim
}

func exchangeEdges(t *treeGraph, g *Graph, e, f Edge) {
	t.g.RemoveEdge(e)
	// also try reverse since undirected
	t.g.RemoveEdge(Edge{V: e.W, W: e.V})
	t.g.SetEdgeVW(f.V, f.W, &EdgeLabel{})
	// Remove old edge labels
	k1 := e.V + "\x00" + e.W
	k2 := e.W + "\x00" + e.V
	delete(t.edgeLabels, k1)
	delete(t.edgeLabels, k2)

	initLowLimValues(t, "")
	initCutValues(t, g)
	updateRanks(t, g)
}

func updateRanks(t *treeGraph, g *Graph) {
	// Find root (no parent)
	var root string
	for _, v := range t.g.Nodes() {
		nl := t.nodeLabel(v)
		if !nl.HasParent {
			root = v
			break
		}
	}
	if root == "" {
		return
	}

	vs := Preorder(t.g, []string{root})
	if len(vs) > 0 {
		vs = vs[1:] // skip root
	}

	for _, v := range vs {
		tn := t.nodeLabel(v)
		parent := tn.Parent
		edge := g.EdgeVW(v, parent)
		flipped := false
		if edge == nil {
			edge = g.EdgeVW(parent, v)
			flipped = true
		}
		if edge == nil {
			continue
		}

		parentNode := g.Node(parent)
		vNode := g.Node(v)
		if flipped {
			vNode.Rank = parentNode.Rank + edge.MinLen
		} else {
			vNode.Rank = parentNode.Rank - edge.MinLen
		}
		vNode.HasRank = true
	}
}
