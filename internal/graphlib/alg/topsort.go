// Ported from dagre-d3-es 7.0.14, dagre-d3-es/src/graphlib/alg/topsort.js and
// is-acyclic.js (MIT).

package alg

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
)

// ErrCycle is the CycleException of topsort: the graph has a cycle.
var ErrCycle = errors.New("graph has a cycle")

// Topsort returns the nodes of g in a topological order. It returns
// ErrCycle when g has a cycle.
func Topsort(g *graphlib.Graph) ([]string, error) {
	visited := map[string]bool{}
	stack := map[string]bool{}
	results := []string{}

	var visit func(node string) error
	visit = func(node string) error {
		if stack[node] {
			return ErrCycle
		}

		if !visited[node] {
			stack[node] = true
			visited[node] = true
			for _, u := range g.Predecessors(node) {
				if err := visit(u); err != nil {
					return err
				}
			}
			delete(stack, node)
			results = append(results, node)
		}
		return nil
	}

	for _, v := range g.Sinks() {
		if err := visit(v); err != nil {
			return nil, err
		}
	}

	if len(visited) != g.NodeCount() {
		return nil, ErrCycle
	}

	return results, nil
}

// IsAcyclic reports whether g has no cycle.
func IsAcyclic(g *graphlib.Graph) bool {
	_, err := Topsort(g)
	return err == nil
}
