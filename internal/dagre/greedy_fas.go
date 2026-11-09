// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/dagre/greedy-fas.js (MIT).

package dagre

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// greedyFAS is a greedy heuristic that finds a feedback arc set of a graph:
// a set of edges that can be removed to make the graph acyclic. The
// algorithm comes from P. Eades, X. Lin and W. F. Smyth, "A fast and
// effective heuristic for the feedback arc set problem", with weighted edges.
func greedyFAS(g *graphlib.Graph, weightFn func(e graphlib.Edge) float64) []graphlib.Edge {
	if g.NodeCount() <= 1 {
		return []graphlib.Edge{}
	}
	fasGraph, buckets, zeroIdx := buildState(g, weightFn)
	results := doGreedyFAS(fasGraph, buckets, zeroIdx)

	// Expand multi-edges.
	out := []graphlib.Edge{}
	for _, e := range results {
		out = append(out, g.OutEdges(e.V, e.W)...)
	}
	return out
}

func doGreedyFAS(g *graphlib.Graph, buckets []*list, zeroIdx float64) []graphlib.Edge {
	results := []graphlib.Edge{}
	sources := buckets[len(buckets)-1]
	sinks := buckets[0]

	for g.NodeCount() > 0 {
		for entry := sinks.dequeue(); entry != nil; entry = sinks.dequeue() {
			removeNode(g, buckets, zeroIdx, entry, false)
		}
		for entry := sources.dequeue(); entry != nil; entry = sources.dequeue() {
			removeNode(g, buckets, zeroIdx, entry, false)
		}
		if g.NodeCount() > 0 {
			for i := len(buckets) - 2; i > 0; i-- {
				entry := buckets[i].dequeue()
				if entry != nil {
					results = append(results, removeNode(g, buckets, zeroIdx, entry, true)...)
					break
				}
			}
		}
	}

	return results
}

func removeNode(g *graphlib.Graph, buckets []*list, zeroIdx float64, entry *fasEntry, collectPredecessors bool) []graphlib.Edge {
	var results []graphlib.Edge

	for _, edge := range g.InEdges(entry.v) {
		weight := fasWeight(g.EdgeObj(edge))
		uEntry := g.Node(edge.V).(*fasEntry)

		if collectPredecessors {
			results = append(results, graphlib.Edge{V: edge.V, W: edge.W})
		}

		uEntry.out -= weight
		assignBucket(buckets, zeroIdx, uEntry)
	}

	for _, edge := range g.OutEdges(entry.v) {
		weight := fasWeight(g.EdgeObj(edge))
		w := edge.W
		wEntry := g.Node(w).(*fasEntry)
		wEntry.in -= weight
		assignBucket(buckets, zeroIdx, wEntry)
	}

	g.RemoveNode(entry.v)

	return results
}

func fasWeight(label any) float64 {
	w, ok := label.(float64)
	if !ok {
		return nan
	}
	return w
}

func buildState(g *graphlib.Graph, weightFn func(e graphlib.Edge) float64) (*graphlib.Graph, []*list, float64) {
	fasGraph := graphlib.New(graphlib.Options{})
	maxIn := 0.0
	maxOut := 0.0

	for _, v := range g.Nodes() {
		fasGraph.SetNode(v, &fasEntry{v: v, in: 0, out: 0})
	}

	// Aggregate weights on nodes, but also sum the weights across
	// multi-edges into a single edge for the fasGraph.
	for _, e := range g.Edges() {
		prevWeight := fasWeight(fasGraph.Edge(e.V, e.W))
		if !truthy(prevWeight) {
			prevWeight = 0
		}
		weight := weightFn(e)
		edgeWeight := prevWeight + weight
		setEdge(fasGraph, e.V, e.W, edgeWeight, "")
		vEntry := fasGraph.Node(e.V).(*fasEntry)
		vEntry.out += weight
		maxOut = js.Max(maxOut, vEntry.out)
		wEntry := fasGraph.Node(e.W).(*fasEntry)
		wEntry.in += weight
		maxIn = js.Max(maxIn, wEntry.in)
	}

	count := rangeEnd(maxOut + maxIn + 3)
	buckets := make([]*list, len(count))
	for i := range buckets {
		buckets[i] = newList()
	}
	zeroIdx := maxIn + 1

	for _, v := range fasGraph.Nodes() {
		assignBucket(buckets, zeroIdx, fasGraph.Node(v).(*fasEntry))
	}

	return fasGraph, buckets, zeroIdx
}

func assignBucket(buckets []*list, zeroIdx float64, entry *fasEntry) {
	if !truthy(entry.out) {
		buckets[0].enqueue(entry)
	} else if !truthy(entry.in) {
		buckets[len(buckets)-1].enqueue(entry)
	} else {
		i, ok := arrayIndex(entry.out - entry.in + zeroIdx)
		if !ok || i >= len(buckets) {
			// buckets[i] is undefined in JavaScript.
			panic(typeError("Cannot read properties of undefined (reading 'enqueue')"))
		}
		buckets[i].enqueue(entry)
	}
}
