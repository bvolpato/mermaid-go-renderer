// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/dagre/mermaid-graphlib.js (MIT).

package dagre

import (
	"errors"
	"slices"

	dagrelib "github.com/bvolpato/mermaid-go-renderer/internal/dagre"
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// ClusterEntry is a value of the clusterDb Map.
//
// adjustClustersAndEdges stores { id, clusterData } and can add
// externalConnections. The measure phase replaces the entry of a cluster
// that stays in its graph with { id, node }, and the paint phase sets node.
type ClusterEntry struct {
	// ID is the id of the node that stands for the cluster at the end of
	// an edge: a descendant that is not a cluster. Empty stands for
	// undefined.
	ID string
	// ClusterData is the label of the cluster node.
	ClusterData *Node
	// ExternalConnections says that an edge connects a node inside the
	// cluster with a node outside of it.
	ExternalConnections bool
	// Node is the laid-out cluster node, or nil.
	Node *Node
}

// state is the module state of mermaid-graphlib.js (clusterDb, descendants
// and parents) and of lodash (the uniqueId counter that dagre uses), kept
// per render.
type state struct {
	clusterDb   *js.Map[string, *ClusterEntry]
	descendants *js.Map[string, []string]
	parents     *js.Map[string, string]

	// dagre carries the uniqueId counter across the layouts of the nested
	// cluster graphs of one render.
	dagre *dagrelib.State

	// ctx is the render: its scheduler keeps the order of the async work
	// of upstream.
	ctx *render.Context
	// ops are the functions of the other rendering-util modules.
	ops *ops
	// trace, when set, sees the graphs at fixed points. Tests use it.
	trace *trace
}

const stateKey = "ru.layout.dagre"

// getState returns the state of the render.
func getState(ctx *render.Context) *state {
	return ctx.Value(stateKey, func() any {
		return &state{
			clusterDb:   js.NewMap[string, *ClusterEntry](),
			descendants: js.NewMap[string, []string](),
			parents:     js.NewMap[string, string](),
			dagre:       &dagrelib.State{},
			ctx:         ctx,
			ops:         realOps(ctx),
		}
	}).(*state)
}

// ClusterDb returns the clusterDb Map of the render.
func ClusterDb(ctx *render.Context) *js.Map[string, *ClusterEntry] {
	return getState(ctx).clusterDb
}

// Clear is clear(): it empties clusterDb, descendants and parents.
func Clear(ctx *render.Context) {
	getState(ctx).clear()
}

func (st *state) clear() {
	st.descendants.Clear()
	st.parents.Clear()
	st.clusterDb.Clear()
}

// typeError stands for the TypeError that JavaScript throws for a property
// access on undefined.
func typeError(text string) error {
	return errors.New(text)
}

// childCount is graph.children(v).length. The error is the TypeError of
// upstream for a node that the graph does not have.
func childCount(g *graphlib.Graph, v string) (int, error) {
	children := g.Children(v)
	if children == nil {
		return 0, typeError("Cannot read properties of undefined (reading 'length')")
	}
	return len(children), nil
}

func (st *state) isDescendant(id, ancestorID string) bool {
	ancestorDescendants := st.descendants.Get(ancestorID)
	return slices.Contains(ancestorDescendants, id)
}

func (st *state) edgeInCluster(edge graphlib.Edge, clusterID string) bool {
	clusterDescendants := st.descendants.Get(clusterID)
	if edge.V == clusterID || edge.W == clusterID {
		return false
	}

	return slices.Contains(clusterDescendants, edge.V) ||
		st.isDescendant(edge.V, clusterID) ||
		st.isDescendant(edge.W, clusterID) ||
		slices.Contains(clusterDescendants, edge.W)
}

// setParent is graph.setParent(v, parent). ok is false for an undefined
// parent: v becomes a child of the root.
func setParent(g *graphlib.Graph, v, parent string, ok bool) error {
	if ok {
		return g.SetParent(v, parent)
	}
	return g.SetParent(v)
}

func (st *state) copy(clusterID string, graph, newGraph *graphlib.Graph, rootID string) error {
	nodes := graph.Children(clusterID)

	if clusterID != rootID {
		nodes = append(nodes, clusterID)
	}

	for _, node := range nodes {
		n, err := childCount(graph, node)
		if err != nil {
			return err
		}
		if n > 0 {
			if err := st.copy(node, graph, newGraph, rootID); err != nil {
				return err
			}
		} else {
			data := graph.Node(node)
			newGraph.SetNode(node, data)
			if parent, ok := graph.Parent(node); !ok || rootID != parent {
				if err := setParent(newGraph, node, parent, ok); err != nil {
					return err
				}
			}

			if clusterID != rootID && node != clusterID {
				if err := newGraph.SetParent(node, clusterID); err != nil {
					return err
				}
			}
			edges := graph.Edges()
			for _, edge := range edges {
				data := graph.EdgeObj(edge)
				if st.edgeInCluster(edge, rootID) {
					// Upstream logs an error of setEdge and goes on.
					_ = newGraph.SetEdgeObj(edge, data)
				}
			}
		}
		graph.RemoveNode(node)
	}
	return nil
}

// ExtractDescendants is extractDescendants(id, graph): the descendants of
// a node, children first. It records the parent of every descendant.
func ExtractDescendants(ctx *render.Context, id string, graph *graphlib.Graph) []string {
	return getState(ctx).extractDescendants(id, graph)
}

func (st *state) extractDescendants(id string, graph *graphlib.Graph) []string {
	children := graph.Children(id)
	res := slices.Clone(children)

	for _, child := range children {
		st.parents.Set(child, id)
		res = append(res, st.extractDescendants(child, graph)...)
	}

	return res
}

// Validate is validate(graph): false when an edge starts or ends at a node
// that has children.
func Validate(graph *graphlib.Graph) bool {
	edges := graph.Edges()
	for _, edge := range edges {
		if len(graph.Children(edge.V)) > 0 {
			return false
		}
		if len(graph.Children(edge.W)) > 0 {
			return false
		}
	}
	return true
}

type commonEdge struct {
	v, w string
}

// findCommonEdges is findCommonEdges(graph, id1, id2). An empty id stands
// for undefined: no edge has such an end.
func findCommonEdges(graph *graphlib.Graph, id1, id2 string) []commonEdge {
	if id1 == "" || id2 == "" {
		return nil
	}
	var edges1, edges2 []graphlib.Edge
	for _, edge := range graph.Edges() {
		if edge.V == id1 || edge.W == id1 {
			edges1 = append(edges1, edge)
		}
	}
	for _, edge := range graph.Edges() {
		if edge.V == id2 || edge.W == id2 {
			edges2 = append(edges2, edge)
		}
	}
	edges1Prim := make([]commonEdge, len(edges1))
	for i, edge := range edges1 {
		p := commonEdge{v: edge.V, w: edge.W}
		if edge.V == id1 {
			p.v = id2
		}
		if edge.W == id1 {
			p.w = id1
		}
		edges1Prim[i] = p
	}
	edges2Prim := make([]commonEdge, len(edges2))
	for i, edge := range edges2 {
		edges2Prim[i] = commonEdge{v: edge.V, w: edge.W}
	}
	var result []commonEdge
	for _, edgeIn1 := range edges1Prim {
		if slices.ContainsFunc(edges2Prim, func(edge commonEdge) bool { return edgeIn1.v == edge.v && edgeIn1.w == edge.w }) {
			result = append(result, edgeIn1)
		}
	}

	return result
}

// FindNonClusterChild is findNonClusterChild(id, graph, clusterId): a
// descendant of id that has no children, or id itself when it has none. A
// descendant that shares an edge with the cluster is the last choice. An
// empty clusterID stands for undefined. An empty result stands for
// undefined.
func FindNonClusterChild(id string, graph *graphlib.Graph, clusterID string) (string, error) {
	children := graph.Children(id)
	if children == nil {
		return "", typeError("Cannot read properties of undefined (reading 'length')")
	}
	if len(children) < 1 {
		return id, nil
	}
	reserve := ""
	for _, child := range children {
		_id, err := FindNonClusterChild(child, graph, clusterID)
		if err != nil {
			return "", err
		}

		commonEdges := findCommonEdges(graph, clusterID, _id)

		if _id != "" {
			if len(commonEdges) > 0 {
				reserve = _id
			} else {
				return _id, nil
			}
		}
	}
	return reserve, nil
}

func (st *state) getAnchorId(id string) string {
	entry, ok := st.clusterDb.Lookup(id)
	if !ok {
		return id
	}
	if !entry.ExternalConnections {
		return id
	}

	return entry.ID
}

// AdjustClustersAndEdges is adjustClustersAndEdges(graph): it finds the
// clusters of the graph, moves the edges that end at a cluster to a node
// inside the cluster, and moves every cluster without external connections
// into a graph of its own (see Extractor).
//
// The error stands for the TypeError that upstream throws for a graph that
// it cannot handle.
func AdjustClustersAndEdges(ctx *render.Context, graph *graphlib.Graph) error {
	return getState(ctx).adjustClustersAndEdges(graph, 0)
}

func (st *state) adjustClustersAndEdges(graph *graphlib.Graph, depth int) error {
	if graph == nil || depth > 10 {
		return nil
	}

	for _, id := range graph.Nodes() {
		children := graph.Children(id)
		if len(children) > 0 {
			st.descendants.Set(id, st.extractDescendants(id, graph))
			anchor, err := FindNonClusterChild(id, graph, id)
			if err != nil {
				return err
			}
			st.clusterDb.Set(id, &ClusterEntry{ID: anchor, ClusterData: nodeOf(graph, id)})
		}
	}

	for _, id := range graph.Nodes() {
		children := graph.Children(id)
		edges := graph.Edges()
		if len(children) > 0 {
			for _, edge := range edges {
				d1 := st.isDescendant(edge.V, id)
				d2 := st.isDescendant(edge.W, id)

				if d1 != d2 {
					st.clusterDb.Get(id).ExternalConnections = true
				}
			}
		}
	}

	for _, id := range st.clusterDb.Keys() {
		entry := st.clusterDb.Get(id)
		nonClusterChild := entry.ID
		parent, hasParent := graph.Parent(nonClusterChild)

		if hasParent && parent != id && st.clusterDb.Has(parent) && !st.clusterDb.Get(parent).ExternalConnections {
			entry.ID = parent
		}
		// When this cluster has a direct outgoing edge and its anchor is
		// inside a sibling subgraph that will be extracted (collapsed into
		// a clusterNode), the anchor is gone at render time and the end of
		// the edge is undefined. Then anchor to a node that stays.
		hasDirectOutgoingEdge := slices.ContainsFunc(graph.Edges(), func(edge graphlib.Edge) bool { return edge.V == id })
		if nonClusterChild != "" &&
			entry.ExternalConnections &&
			hasDirectOutgoingEdge &&
			st.isNodeInExtractableCluster(graph, nonClusterChild, id) {
			excludedCluster, hasExcluded := graph.Parent(nonClusterChild)
			safeAnchor, err := st.findSafeAnchorNode(graph, id, excludedCluster, hasExcluded)
			if err != nil {
				return err
			}
			if safeAnchor != "" {
				entry.ID = safeAnchor
			}
		}
	}

	for _, e := range graph.Edges() {
		edge := edgeOf(graph, e)

		v := e.V
		w := e.W
		if st.clusterDb.Has(e.V) || st.clusterDb.Has(e.W) {
			v = st.getAnchorId(e.V)
			w = st.getAnchorId(e.W)
			graph.RemoveEdgeObj(e)
			if v != e.V {
				parent, _ := graph.Parent(v)
				entry, ok := st.clusterDb.Lookup(parent)
				if !ok {
					return typeError("Cannot set properties of undefined (setting 'externalConnections')")
				}
				entry.ExternalConnections = true
				if edge == nil {
					return typeError("Cannot set properties of undefined (setting 'fromCluster')")
				}
				edge.FromCluster = e.V
			}
			if w != e.W {
				parent, _ := graph.Parent(w)
				entry, ok := st.clusterDb.Lookup(parent)
				if !ok {
					return typeError("Cannot set properties of undefined (setting 'externalConnections')")
				}
				entry.ExternalConnections = true
				if edge == nil {
					return typeError("Cannot set properties of undefined (setting 'toCluster')")
				}
				edge.ToCluster = e.W
			}
			if err := graph.SetEdgeObj(graphlib.Edge{V: v, W: w, Name: e.Name}, edgeLabelValue(edge)); err != nil {
				return err
			}
		}
	}
	return st.extractor(graph, 0)
}

// edgeLabelValue converts an edge label to a label value of the graph: a
// nil *Edge is the undefined label.
func edgeLabelValue(edge *Edge) any {
	if edge == nil {
		return nil
	}
	return edge
}

// Extractor is extractor(graph, depth): it moves every cluster that has no
// external connections into a graph of its own. The node of the cluster
// stays in the graph with a new label: a cluster node (ClusterNode is
// true) with the old label in ClusterData and the new graph in Graph. It
// then does the same in the new graphs.
func Extractor(ctx *render.Context, graph *graphlib.Graph, depth int) error {
	return getState(ctx).extractor(graph, depth)
}

func (st *state) extractor(graph *graphlib.Graph, depth int) error {
	if depth > 10 {
		return nil
	}
	if graph == nil {
		return typeError("Cannot read properties of undefined (reading 'nodes')")
	}
	nodes := graph.Nodes()
	hasChildren := false
	for _, node := range nodes {
		children := graph.Children(node)
		hasChildren = hasChildren || len(children) > 0
	}

	if !hasChildren {
		return nil
	}
	for _, node := range nodes {
		entry, isCluster := st.clusterDb.Lookup(node)
		if !isCluster {
			// Not a cluster.
		} else if !entry.ExternalConnections && len(graph.Children(node)) > 0 {
			// A cluster without external connections gets its own graph.
			graphSettings := graphOf(graph)
			if graphSettings == nil {
				return typeError("Cannot read properties of undefined (reading 'rankdir')")
			}
			dir := "TB"
			if graphSettings.Rankdir == "TB" {
				dir = "LR"
			}
			if entry.ClusterData != nil && entry.ClusterData.Dir != "" {
				dir = entry.ClusterData.Dir
			}

			clusterGraph := newGraph(newGraphLabel(dir, 50.0, 50.0))

			if err := st.copy(node, graph, clusterGraph, node); err != nil {
				return err
			}
			// The label of a clusterDb entry has no property label, so the
			// label of the cluster node is undefined.
			clusterNode := ru.NewNode(node)
			clusterNode.ClusterNode = true
			graph.SetNode(node, &Node{
				Node:        clusterNode,
				ClusterData: entry.ClusterData,
				Graph:       clusterGraph,
			})
		}
	}

	nodes = graph.Nodes()
	for _, node := range nodes {
		data := nodeOf(graph, node)
		if data != nil && data.ClusterNode {
			if err := st.extractor(data.Graph, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func sorter(graph *graphlib.Graph, nodes []string) []string {
	if len(nodes) == 0 {
		return []string{}
	}
	result := slices.Clone(nodes)
	for _, node := range nodes {
		children := graph.Children(node)
		sorted := sorter(graph, children)
		result = append(result, sorted...)
	}

	return result
}

// SortNodesByHierarchy is sortNodesByHierarchy(graph): the nodes of the
// graph, every node before its descendants. The children of the root come
// first, then the descendants of the first of them, and so on.
func SortNodesByHierarchy(graph *graphlib.Graph) []string {
	return sorter(graph, graph.Children())
}

// isNodeInExtractableCluster checks if a node is inside a cluster that
// will be extracted (a cluster without external connections).
func (st *state) isNodeInExtractableCluster(graph *graphlib.Graph, node, rootID string) bool {
	parent, ok := graph.Parent(node)

	for ok && parent != "" && parent != rootID {
		cluster, isCluster := st.clusterDb.Lookup(parent)
		if isCluster && !cluster.ExternalConnections {
			return true
		}
		parent, ok = graph.Parent(parent)
	}

	return false
}

// findSafeAnchorNode finds another anchor node for a cluster: one that is
// not inside a cluster that will be extracted. hasExcluded is false for an
// undefined excludedCluster. An empty result stands for null.
func (st *state) findSafeAnchorNode(graph *graphlib.Graph, clusterID, excludedCluster string, hasExcluded bool) (string, error) {
	children := graph.Children(clusterID)

	for _, child := range children {
		if hasExcluded && (child == excludedCluster || st.isDescendant(child, excludedCluster)) {
			continue
		}

		// findNonClusterChild returns the leaf itself when child is a leaf,
		// or a descendant of a subgraph that is not a cluster. Skip only
		// when the lookup found nothing.
		candidate, err := FindNonClusterChild(child, graph, clusterID)
		if err != nil {
			return "", err
		}
		if candidate == "" {
			continue
		}

		if !st.isNodeInExtractableCluster(graph, candidate, clusterID) {
			return candidate, nil
		}
	}

	return "", nil
}
