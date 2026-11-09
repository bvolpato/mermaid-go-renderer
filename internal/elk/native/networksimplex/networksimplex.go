// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/networksimplex/NetworkSimplex.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package networksimplex has the network simplex algorithm of ELK
// (org.eclipse.elk.alg.common.networksimplex): it gives each node of an
// acyclic graph a layer, so that the weighted sum of the edge lengths is
// minimal. The layered algorithm uses it for the layer assignment and for the
// node placement.
//
// The use follows the Java classes:
//
//	graph := networksimplex.NewNGraph()
//	a := networksimplex.NNodeOf().Origin(x).Create(graph)
//	b := networksimplex.NNodeOf().Origin(y).Create(graph)
//	networksimplex.NEdgeOf().Weight(1).Delta(1).Source(a).Target(b).Create()
//	networksimplex.ForGraph(graph).WithIterationLimit(100).WithBalancing(true).Execute()
//	// a.Layer and b.Layer have the result.
//
// Where the Java code throws an exception, the package calls lgraph.Bail.
package networksimplex

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

const (
	// removeSubtreesThresh is REMOVE_SUBTREES_THRESH: the number of nodes
	// from which the algorithm removes subtrees first.
	removeSubtreesThresh = 40

	// fuzzyStZero is FUZZY_ST_ZERO: a value a little below zero. A cut value
	// below it counts as negative.
	fuzzyStZero = -1e-10
)

// leafy is the Pair<NNode, NEdge> of the stack of removed subtree nodes.
type leafy struct {
	node *NNode
	edge *NEdge
}

// NetworkSimplex is the class NetworkSimplex. Make one with ForGraph.
type NetworkSimplex struct {
	// previousLayeringNodeCounts has the number of nodes in each layer of an
	// earlier layering, for normalize and balance.
	previousLayeringNodeCounts []int
	// balance tells to balance the layering.
	balance bool
	// iterationLimit is the limit on the number of iterations.
	iterationLimit int

	// graph is the graph that the methods work on.
	graph *NGraph
	// edges has all edges of the graph.
	edges []*NEdge
	// treeEdges has the edges of the spanning tree, in insertion order.
	treeEdges *gwt.Set[*NEdge]
	// sources has the nodes without incoming edges.
	sources []*NNode
	// edgeVisited tells for each edge that a depth-first search visited it.
	edgeVisited []bool
	// postOrder is the next number of the postorder traversal.
	postOrder int
	// poID has the postorder number of each node.
	poID []int
	// lowestPoID has for each node the lowest postorder number of a node
	// below it in the traversal tree.
	lowestPoID []int
	// cutvalue has the cut value of each edge.
	cutvalue []float64
	// subtreeNodesStack has the removed subtree nodes. The last removed node
	// is at the end.
	subtreeNodesStack []leafy
}

// ForGraph is NetworkSimplex.forGraph(graph).
func ForGraph(graph *NGraph) *NetworkSimplex {
	return &NetworkSimplex{graph: graph, iterationLimit: math.MaxInt32}
}

// WithBalancing is withBalancing(doBalance). A balanced layering moves each
// node that can move without a longer edge to the layer with the fewest
// nodes.
func (ns *NetworkSimplex) WithBalancing(doBalance bool) *NetworkSimplex {
	ns.balance = doBalance
	return ns
}

// WithPreviousLayering is withPreviousLayering(considerPreviousLayering):
// the number of nodes that an earlier layering put in each layer. The
// balancing counts them. A nil slice is the Java null.
func (ns *NetworkSimplex) WithPreviousLayering(considerPreviousLayering []int) *NetworkSimplex {
	ns.previousLayeringNodeCounts = considerPreviousLayering
	return ns
}

// WithIterationLimit is withIterationLimit(limit).
func (ns *NetworkSimplex) WithIterationLimit(limit int) *NetworkSimplex {
	ns.iterationLimit = limit
	return ns
}

// initialize makes the data of one run: it numbers the nodes and the edges
// and finds the source nodes.
func (ns *NetworkSimplex) initialize() {
	// initialize node attributes
	numNodes := len(ns.graph.Nodes)
	for _, n := range ns.graph.Nodes {
		n.treeNode = false
	}
	ns.poID = make([]int, numNodes)
	ns.lowestPoID = make([]int, numNodes)
	ns.sources = nil

	// determine edges and re-index nodes
	index := 0
	var theEdges []*NEdge
	for _, node := range ns.graph.Nodes {
		node.internalID = index
		index++
		// add node to sinks, resp. sources
		if len(node.incomingEdges.list) == 0 {
			ns.sources = append(ns.sources, node)
		}
		theEdges = append(theEdges, node.outgoingEdges.list...)
	}
	// re-index edges
	counter := 0
	for _, edge := range theEdges {
		edge.internalID = counter
		counter++
		edge.treeEdge = false
	}
	// initialize edge attributes
	numEdges := len(theEdges)
	if ns.cutvalue == nil || len(ns.cutvalue) < numEdges {
		ns.cutvalue = make([]float64, numEdges)
		ns.edgeVisited = make([]bool, numEdges)
	} else {
		clear(ns.edgeVisited)
	}
	ns.edges = theEdges
	// we iterate over this set, thus we have to use a linked hash set
	// to get a deterministic iteration order
	ns.treeEdges = gwt.NewSet[*NEdge]()
	ns.postOrder = 1

	ns.checkGraph()
}

// checkGraph stops the layout when an edge of a node of the graph has an end
// that is not a node of the graph, or is not in the edge list. The Java code
// does not expect such a graph: it reads the arrays at the index of another
// graph. No caller of ELK makes one.
func (ns *NetworkSimplex) checkGraph() {
	nodes := ns.graph.Nodes
	inGraph := func(n *NNode) bool {
		return n != nil && n.internalID >= 0 && n.internalID < len(nodes) && nodes[n.internalID] == n
	}
	inEdges := func(e *NEdge) bool {
		return e.internalID >= 0 && e.internalID < len(ns.edges) && ns.edges[e.internalID] == e
	}
	for _, node := range nodes {
		for _, edge := range node.outgoingEdges.list {
			if edge.Source != node || !inGraph(edge.Target) {
				lgraph.Bail("network simplex: an edge leads to a node that is not in the graph")
			}
		}
		for _, edge := range node.incomingEdges.list {
			if edge.Target != node || !inGraph(edge.Source) || !inEdges(edge) {
				lgraph.Bail("network simplex: an edge comes from a node that is not in the graph")
			}
		}
	}
}

// dispose releases the data of one run.
func (ns *NetworkSimplex) dispose() {
	ns.cutvalue = nil
	ns.edges = nil
	ns.treeEdges = nil
	ns.edgeVisited = nil
	ns.lowestPoID = nil
	ns.poID = nil
	ns.sources = nil
	ns.subtreeNodesStack = nil
}

// Execute is execute(): it finds the optimal layering and stores it in the
// field Layer of each node.
func (ns *NetworkSimplex) Execute() {
	if len(ns.graph.Nodes) < 1 {
		return
	}

	// reset any old layering
	for _, node := range ns.graph.Nodes {
		node.Layer = 0
	}

	// remove leafs
	removeSubtrees := len(ns.graph.Nodes) >= removeSubtreesThresh
	if removeSubtrees {
		ns.removeSubtrees()
	}

	// init all the data structures we use
	ns.initialize()
	// determine an initial feasible layering
	ns.feasibleTree()
	// improve the initial layering until it is optimal
	e := ns.leaveEdge()
	iter := 0
	for e != nil && iter < ns.iterationLimit {
		// current layering is not optimal
		ns.exchange(e, ns.enterEdge(e))
		e = ns.leaveEdge()
		iter++
	}

	// re-attach leafs
	if removeSubtrees {
		ns.reattachSubtrees()
	}

	// normalize and, if desired, balance
	//   both methods must work on the NNode#layer field
	if ns.balance {
		ns.balanceLayering(ns.normalize())
	} else {
		ns.normalize()
	}

	// release the created resources
	ns.dispose()
}

// removeSubtrees removes leafs from the graph until no more leafs are
// present.
func (ns *NetworkSimplex) removeSubtrees() {
	ns.subtreeNodesStack = nil

	// find initial leafs
	var leafs []*NNode
	for _, node := range ns.graph.Nodes {
		if len(node.GetConnectedEdges()) == 1 {
			leafs = append(leafs, node)
		}
	}

	// remove them from the graph like there's no tomorrow
	for head := 0; head < len(leafs); head++ {
		node := leafs[head]
		// was the edge already removed?
		if len(node.GetConnectedEdges()) == 0 {
			continue
		}
		edge := node.GetConnectedEdges()[0]
		isOutEdge := len(node.outgoingEdges.list) > 0

		other := edge.GetOther(node)
		if isOutEdge {
			other.incomingEdges.remove(edge)
		} else {
			other.outgoingEdges.remove(edge)
		}

		if len(other.GetConnectedEdges()) == 1 {
			leafs = append(leafs, other)
		}

		ns.subtreeNodesStack = append(ns.subtreeNodesStack, leafy{node, edge})
		// remove the node from the graph's nodes
		ns.graph.Nodes = removeNode(ns.graph.Nodes, node)
	}
}

// removeNode is List.remove(Object): it removes the first occurrence.
func removeNode(list []*NNode, node *NNode) []*NNode {
	for i, x := range list {
		if x == node {
			copy(list[i:], list[i+1:])
			list[len(list)-1] = nil
			return list[:len(list)-1]
		}
	}
	return list
}

// removeEdge is List.remove(Object): it removes the first occurrence.
func removeEdge(list []*NEdge, edge *NEdge) []*NEdge {
	for i, x := range list {
		if x == edge {
			copy(list[i:], list[i+1:])
			list[len(list)-1] = nil
			return list[:len(list)-1]
		}
	}
	return list
}

// reattachSubtrees puts the removed nodes back, in the opposite order of
// their removal. The nodes go to the end of the node list.
func (ns *NetworkSimplex) reattachSubtrees() {
	for len(ns.subtreeNodesStack) > 0 {
		last := len(ns.subtreeNodesStack) - 1
		leafy := ns.subtreeNodesStack[last]
		ns.subtreeNodesStack = ns.subtreeNodesStack[:last]
		node := leafy.node
		edge := leafy.edge

		placed := edge.GetOther(node)

		if edge.Target == node {
			placed.outgoingEdges.add(edge)
			node.Layer = placed.Layer + edge.Delta
		} else {
			placed.incomingEdges.add(edge)
			node.Layer = placed.Layer - edge.Delta
		}

		ns.graph.Nodes = append(ns.graph.Nodes, node)
	}
}

// feasibleTree finds an initial feasible spanning tree whose edges are all
// tight, and the initial cut value of each tree edge.
func (ns *NetworkSimplex) feasibleTree() {
	// determine initial layering
	ns.layeringTopologicalNumbering(ns.sources)

	if len(ns.edges) > 0 {
		clear(ns.edgeVisited)
		for ns.tightTreeDFS(ns.graph.Nodes[0]) < len(ns.graph.Nodes) {
			// some nodes are still not part of the tree
			e := ns.minimalSlack()
			if e == nil {
				// The JavaScript code fails here with a TypeError: the
				// graph is not connected.
				lgraph.Bail("network simplex: the graph is not connected")
			}
			slack := e.Target.Layer - e.Source.Layer - e.Delta
			if e.Target.treeNode {
				slack = -slack
			}

			// update tree
			for _, node := range ns.graph.Nodes {
				if node.treeNode {
					node.Layer += slack
				}
			}
			clear(ns.edgeVisited)
		}
		// update tree-related attributes
		clear(ns.edgeVisited)
		ns.postorderTraversal(ns.graph.Nodes[0])
		ns.cutvalues()
	}
}

// layeringTopologicalNumbering finds an initial feasible layering: it walks
// the graph in a topological order from the given root nodes along the
// outgoing edges.
func (ns *NetworkSimplex) layeringTopologicalNumbering(initialRootNodes []*NNode) {
	// initialize the number of incident edges for each node
	incident := make([]int, len(ns.graph.Nodes))
	for _, node := range ns.graph.Nodes {
		incident[node.internalID] += len(node.incomingEdges.list)
	}

	// The Java list is a LinkedList that is polled at the head.
	roots := append([]*NNode(nil), initialRootNodes...)
	for head := 0; head < len(roots); head++ {
		node := roots[head]

		for _, edge := range node.outgoingEdges.list {
			target := edge.Target
			target.Layer = max(target.Layer, node.Layer+edge.Delta)
			incident[target.internalID]--
			if incident[target.internalID] == 0 {
				roots = append(roots, target)
			}
		}
	}
}

// minimalSpan returns the length of the shortest incoming edge and of the
// shortest outgoing edge of the node, or -1 where the node has no such edge.
//
// An incoming edge that is not shorter than the shortest incoming edge so
// far counts for the outgoing edges, as in the Java code.
func minimalSpan(node *NNode) (minSpanIn, minSpanOut int) {
	minSpanOut = math.MaxInt32
	minSpanIn = math.MaxInt32

	for _, edge := range node.GetConnectedEdges() {
		currentSpan := edge.Target.Layer - edge.Source.Layer
		if edge.Target == node && currentSpan < minSpanIn {
			minSpanIn = currentSpan
		} else if currentSpan < minSpanOut {
			minSpanOut = currentSpan
		}
	}

	if minSpanIn == math.MaxInt32 {
		minSpanIn = -1
	}
	if minSpanOut == math.MaxInt32 {
		minSpanOut = -1
	}

	return minSpanIn, minSpanOut
}

// tightTreeDFS walks the tight edges from the node (the edges whose length
// is their minimum length), adds them to the tree and returns the number of
// nodes that it reached. When that is the number of nodes of the graph, the
// tree is a tight spanning tree.
func (ns *NetworkSimplex) tightTreeDFS(node *NNode) int {
	nodeCount := 1
	node.treeNode = true
	for _, edge := range node.GetConnectedEdges() {
		if !ns.edgeVisited[edge.internalID] {
			ns.edgeVisited[edge.internalID] = true
			opposite := edge.GetOther(node)
			if edge.treeEdge {
				// edge is a tree edge already: follow this path
				nodeCount += ns.tightTreeDFS(opposite)
			} else if !opposite.treeNode && edge.Delta == edge.Target.Layer-edge.Source.Layer {
				// edge is a tight non-tree edge
				edge.treeEdge = true
				ns.treeEdges.Add(edge)
				nodeCount += ns.tightTreeDFS(opposite)
			}
		}
	}
	return nodeCount
}

// minimalSlack returns the edge between a tree node and a node outside the
// tree that has the least slack (the least difference between its length and
// its minimum length), or nil without such an edge.
func (ns *NetworkSimplex) minimalSlack() *NEdge {
	minSlack := math.MaxInt32
	var minSlackEdge *NEdge
	for _, edge := range ns.edges {
		if edge.Source.treeNode != edge.Target.treeNode {
			// edge is non-tree edge and incident on the tree
			curSlack := edge.Target.Layer - edge.Source.Layer - edge.Delta
			if curSlack < minSlack {
				minSlack = curSlack
				minSlackEdge = edge
			}
		}
	}
	return minSlackEdge
}

// postorderTraversal walks the tree from the node in postorder. It gives
// each node its number in poID and the lowest number below it in lowestPoID,
// and returns the lowest number of the node.
func (ns *NetworkSimplex) postorderTraversal(node *NNode) int {
	lowest := math.MaxInt32
	for _, edge := range node.GetConnectedEdges() {
		if edge.treeEdge && !ns.edgeVisited[edge.internalID] {
			ns.edgeVisited[edge.internalID] = true
			lowest = min(lowest, ns.postorderTraversal(edge.GetOther(node)))
		}
	}
	ns.poID[node.internalID] = ns.postOrder
	ns.lowestPoID[node.internalID] = min(lowest, ns.postOrder)
	ns.postOrder++
	return ns.lowestPoID[node.internalID]
}

// isInHead tells that the node is in the head component of the tree edge:
// without the edge the tree has two components, and the head component has
// the target of the edge.
func (ns *NetworkSimplex) isInHead(node *NNode, edge *NEdge) bool {
	source := edge.Source
	target := edge.Target

	if ns.lowestPoID[source.internalID] <= ns.poID[node.internalID] &&
		ns.poID[node.internalID] <= ns.poID[source.internalID] &&
		ns.lowestPoID[target.internalID] <= ns.poID[node.internalID] &&
		ns.poID[node.internalID] <= ns.poID[target.internalID] {
		// node is in a descending path in the DFS-Tree
		if ns.poID[source.internalID] < ns.poID[target.internalID] {
			// root is in the head component
			return false
		}
		return true
	}
	if ns.poID[source.internalID] < ns.poID[target.internalID] {
		// root is in the head component
		return true
	}
	return false
}

// cutvalues finds the cut value of each tree edge: the sum of the weights of
// the edges from the tail component to the head component, with the tree
// edge, minus the sum of the weights of the edges from the head component to
// the tail component.
func (ns *NetworkSimplex) cutvalues() {
	// determine incident tree edges for each node
	var leafs []*NNode
	for _, node := range ns.graph.Nodes {
		treeEdgeCount := 0
		node.unknownCutvalues = node.unknownCutvalues[:0]
		for _, edge := range node.GetConnectedEdges() {
			if edge.treeEdge {
				node.unknownCutvalues = append(node.unknownCutvalues, edge)
				treeEdgeCount++
			}
		}
		if treeEdgeCount == 1 {
			leafs = append(leafs, node)
		}
	}

	// determine cut values
	for _, node := range leafs {
		for len(node.unknownCutvalues) == 1 {
			// one tree edge with undetermined cut value is incident
			toDetermine := node.unknownCutvalues[0]
			ns.cutvalue[toDetermine.internalID] = toDetermine.Weight
			source := toDetermine.Source
			target := toDetermine.Target
			for _, edge := range node.GetConnectedEdges() {
				if edge != toDetermine {
					if edge.treeEdge {
						// edge is tree edge
						if source == edge.Source || target == edge.Target {
							// edge has not the same direction as toDetermine
							ns.cutvalue[toDetermine.internalID] -= ns.cutvalue[edge.internalID] - edge.Weight
						} else {
							ns.cutvalue[toDetermine.internalID] += ns.cutvalue[edge.internalID] - edge.Weight
						}
					} else {
						// edge is non-tree edge
						if node == source {
							if edge.Source == node {
								ns.cutvalue[toDetermine.internalID] += edge.Weight
							} else {
								ns.cutvalue[toDetermine.internalID] -= edge.Weight
							}
						} else {
							if edge.Source == node {
								ns.cutvalue[toDetermine.internalID] -= edge.Weight
							} else {
								ns.cutvalue[toDetermine.internalID] += edge.Weight
							}
						}
					}
				}
			}

			// remove edge from 'unknownCutvalues'
			source.unknownCutvalues = removeEdge(source.unknownCutvalues, toDetermine)
			target.unknownCutvalues = removeEdge(target.unknownCutvalues, toDetermine)

			// proceed with next node
			if source == node {
				node = toDetermine.Target
			} else {
				node = toDetermine.Source
			}
		}
	}
}

// leaveEdge returns the first tree edge with a negative cut value, or nil
// when the layering is optimal.
func (ns *NetworkSimplex) leaveEdge() *NEdge {
	var found *NEdge
	ns.treeEdges.Each(func(edge *NEdge) bool {
		if edge.treeEdge && ns.cutvalue[edge.internalID] < fuzzyStZero {
			found = edge
			return false
		}
		return true
	})
	return found
}

// enterEdge returns the edge that replaces the given tree edge: the edge
// from the head component to the tail component with the least slack. Java
// throws when the given edge is not a tree edge.
func (ns *NetworkSimplex) enterEdge(leave *NEdge) *NEdge {
	if !leave.treeEdge {
		lgraph.Bail("network simplex: The input edge is not a tree edge.")
	}

	var replace *NEdge
	repSlack := math.MaxInt32
	for _, edge := range ns.edges {
		source := edge.Source
		target := edge.Target
		if ns.isInHead(source, leave) && !ns.isInHead(target, leave) {
			// edge is to consider
			slack := target.Layer - source.Layer - edge.Delta
			if slack < repSlack {
				repSlack = slack
				replace = edge
			}
		}
	}
	return replace
}

// exchange replaces the tree edge leave with the edge enter, moves the nodes
// of the tail component, and finds the postorder numbers and the cut values
// again. Java throws when leave is not a tree edge or enter is one.
func (ns *NetworkSimplex) exchange(leave, enter *NEdge) {
	if !leave.treeEdge {
		lgraph.Bail("network simplex: Given leave edge is no tree edge.")
	}
	if enter == nil {
		// The JavaScript code fails here with a TypeError.
		lgraph.Bail("network simplex: no edge can enter the tree")
	}
	if enter.treeEdge {
		lgraph.Bail("network simplex: Given enter edge is a tree edge already.")
	}

	// update tree
	leave.treeEdge = false
	ns.treeEdges.Remove(leave)
	enter.treeEdge = true
	ns.treeEdges.Add(enter)
	delta := enter.Target.Layer - enter.Source.Layer - enter.Delta
	if !ns.isInHead(enter.Target, leave) {
		delta = -delta
	}
	for _, node := range ns.graph.Nodes {
		if !ns.isInHead(node, leave) {
			node.Layer += delta
		}
	}

	// update tree-based values
	ns.postOrder = 1
	clear(ns.edgeVisited)
	ns.postorderTraversal(ns.graph.Nodes[0])
	ns.cutvalues()
}

// normalize moves the layering so that the lowest layer is 0. It returns the
// number of nodes in each layer, with the nodes of the earlier layering.
func (ns *NetworkSimplex) normalize() []int {
	// determine lowest assigned layer and layer count
	highest := math.MinInt32
	lowest := math.MaxInt32
	for _, node := range ns.graph.Nodes {
		lowest = min(lowest, node.Layer)
		highest = max(highest, node.Layer)
	}
	// normalize and determine layer filling
	filling := make([]int, highest-lowest+1)
	for _, node := range ns.graph.Nodes {
		node.Layer -= lowest
		filling[node.Layer]++
	}

	// also consider nodes of already layered connected components
	layerID := 0
	if ns.previousLayeringNodeCounts != nil {
		for _, nodeCntInLayer := range ns.previousLayeringNodeCounts {
			filling[layerID] += nodeCntInLayer
			layerID++
			if len(filling) == layerID {
				break
			}
		}
	}
	return filling
}

// balanceLayering is balance(filling): it moves each node with as many
// incoming edges as outgoing edges to the layer with the fewest nodes among
// the layers that keep its edges as long as they are. filling has the number
// of nodes in each layer.
func (ns *NetworkSimplex) balanceLayering(filling []int) {
	// determine possible layers
	for _, node := range ns.graph.Nodes {
		if len(node.incomingEdges.list) == len(node.outgoingEdges.list) {
			// node might get shifted
			newLayer := node.Layer
			rangeFirst, rangeSecond := minimalSpan(node)
			for i := node.Layer - rangeFirst + 1; i < node.Layer+rangeSecond; i++ {
				// A layer outside the array is undefined in JavaScript, and
				// the comparison with it is false.
				if i >= 0 && i < len(filling) && filling[i] < filling[newLayer] {
					newLayer = i
				}
			}
			// assign new layer
			if filling[newLayer] < filling[node.Layer] {
				filling[node.Layer]--
				filling[newLayer]++
				node.Layer = newLayer
			}
		}
	}
}
