// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/elk/subgraphFeedbackEdges.ts (MIT).

package elk

import "github.com/bvolpato/mermaid-go-renderer/internal/js"

// EdgeEnds is an edge as far as cycle detection is concerned: the ids of
// its two ends.
type EdgeEnds struct {
	Source string
	Target string
}

const ROOT = "root"

// childOnPath returns the ancestor of id that is a direct child of
// ancestorId (or id itself).
func childOnPath(id, ancestorId string, tree *TreeData) string {
	parentById := tree.ParentByID
	current := id
	guard := newWalkGuard(tree)
	for {
		parent, ok := parentById[current]
		if !ok {
			parent = ROOT
		}
		if parent == ancestorId {
			break
		}
		guard.step()
		current = parentById[current]
	}
	return current
}

type feedbackLevelEdge struct {
	index     int
	from, to  string
	collapsed bool
}

// findSubgraphFeedbackEdges finds the edges that close a cycle only because
// a subgraph is laid out as one node.
//
// With INCLUDE_CHILDREN, ELK lays out each subgraph as a single node of its
// parent graph. A node outside a subgraph that both receives an edge from it
// and sends one into it therefore forms a cycle there, even when the nodes
// inside do not. ELK breaks that cycle by reversing one of the edges and
// routes it into the input side of the subgraph, around the outside of the
// subgraph.
//
// Each edge is looked at on the graph of its lowest common subgraph, with
// every subgraph below that collapsed. A depth-first search from the
// sources, in declaration order, marks an edge that points at a node still
// on the stack as a feedback edge. Only feedback edges with at least one end
// inside a collapsed subgraph are returned.
//
// The result has one flag per edge, true where the edge should be laid out
// reversed.
func findSubgraphFeedbackEdges(edges []EdgeEnds, tree *TreeData) []bool {
	flags := make([]bool, len(edges))
	// lowest common subgraph -> edges between its direct children
	levels := js.NewMap[string, []*feedbackLevelEdge]()

	for index, e := range edges {
		source, target := e.Source, e.Target
		if source == target {
			continue
		}
		ancestor := findCommonAncestor(source, target, tree)
		if ancestor == source || ancestor == target {
			// an edge between a subgraph and its own member crosses no boundary of the parent graph
			continue
		}
		from := childOnPath(source, ancestor, tree)
		to := childOnPath(target, ancestor, tree)
		level := levels.Get(ancestor)
		level = append(level, &feedbackLevelEdge{index: index, from: from, to: to, collapsed: from != source || to != target})
		levels.Set(ancestor, level)
	}

	for _, level := range levels.Values() {
		successors := map[string][]*feedbackLevelEdge{}
		hasSuccessors := map[string]bool{}
		nodes := []string{}
		inDegree := map[string]int{}
		for _, edge := range level {
			for _, node := range [2]string{edge.from, edge.to} {
				if !hasSuccessors[node] {
					hasSuccessors[node] = true
					successors[node] = nil
					nodes = append(nodes, node)
					inDegree[node] = 0
				}
			}
			successors[edge.from] = append(successors[edge.from], edge)
			inDegree[edge.to] = inDegree[edge.to] + 1
		}

		const (
			ON_STACK = 1
			DONE     = 2
		)
		state := map[string]int{}
		roots := []string{}
		for _, node := range nodes {
			if inDegree[node] == 0 {
				roots = append(roots, node)
			}
		}
		roots = append(roots, nodes...)
		type frameT struct {
			node string
			next int
		}
		for _, root := range roots {
			if _, has := state[root]; has {
				continue
			}
			state[root] = ON_STACK
			stack := []*frameT{{node: root, next: 0}}
			for len(stack) > 0 {
				frame := stack[len(stack)-1]
				out := successors[frame.node]
				if frame.next >= len(out) {
					state[frame.node] = DONE
					stack = stack[:len(stack)-1]
					continue
				}
				edge := out[frame.next]
				frame.next++
				seen, has := state[edge.to]
				if has && seen == ON_STACK {
					flags[edge.index] = edge.collapsed
				} else if !has {
					state[edge.to] = ON_STACK
					stack = append(stack, &frameT{node: edge.to, next: 0})
				}
			}
		}
	}

	return flags
}
