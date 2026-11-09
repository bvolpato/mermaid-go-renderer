// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/phase2.options.ts (MIT).

package swimlanes

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// optBool is an optional boolean: undefined, false or true.
type optBool uint8

const (
	optUndefined optBool = iota
	optFalse
	optTrue
)

// optOf is a boolean that is defined.
func optOf(b bool) optBool {
	if b {
		return optTrue
	}
	return optFalse
}

// or is `o ?? d`.
func (o optBool) or(d bool) bool {
	if o == optUndefined {
		return d
	}
	return o == optTrue
}

// truthy is the truthiness of o: undefined is false.
func (o optBool) truthy() bool { return o == optTrue }

// LayeringOptions are the options that control how layers are assigned in
// the Sugiyama pipeline.
type LayeringOptions struct {
	// CompactSingleInput: a node with exactly one incoming edge inherits the
	// layer of its predecessor.
	CompactSingleInput optBool
	// IgnoreCrossLaneEdges: ignore edges from other lanes when calculating
	// layer positions.
	IgnoreCrossLaneEdges optBool
	// OptimizeRanksByCrossings: try to lift nodes to reduce crossings between
	// layers.
	OptimizeRanksByCrossings optBool
	// Direction is the diagram direction ("TB", "LR", "BT", "RL"), or ""
	// when it is undefined.
	Direction string
}

// errMaxCallStack is the error of JavaScript for a recursion without end.
// Upstream recurses over the parent chain of a node, so a cycle of parents
// ends in this error there.
const errMaxCallStack = jsError("Maximum call stack size exceeded")

// buildTopLaneMap computes the "top lane" (outermost group container) of
// every node. The lane is "" (null upstream) for a node that does not
// belong to any lane.
func buildTopLaneMap(g *Graph) map[NodeId]string {
	cache := map[NodeId]string{}
	visiting := map[NodeId]bool{}

	var resolve func(id NodeId) string
	resolve = func(id NodeId) string {
		if lane, ok := cache[id]; ok {
			return lane
		}
		node := g.NodeById.Get(id)
		if node == nil {
			cache[id] = ""
			return ""
		}
		parentId := node.ParentID
		if parentId == "" {
			cache[id] = ""
			return ""
		}
		if visiting[id] {
			panic(errMaxCallStack)
		}
		visiting[id] = true
		parentLane := resolve(parentId)
		delete(visiting, id)
		lane := parentLane
		if lane == "" {
			lane = parentId
		}
		cache[id] = lane
		return lane
	}

	for _, id := range g.Nodes {
		resolve(id)
	}
	return cache
}

func createTopLaneResolver(g *Graph) func(id NodeId) string {
	topLaneMap := buildTopLaneMap(g)
	return func(id NodeId) string { return topLaneMap[id] }
}

func buildTopLaneOrder(g *Graph) []string {
	lanes := js.NewSet[string]()
	for _, node := range g.Layout.Nodes {
		if node.IsGroup && node.ParentID == "" {
			lanes.Add(node.ID)
		}
	}
	values := lanes.Values()
	out := make([]string, 0, len(values))
	for i := len(values) - 1; i >= 0; i-- {
		out = append(out, values[i])
	}
	return out
}

// resolveTopLaneOrder returns the order of the lanes. preferredOrder is
// nil when it is undefined.
func resolveTopLaneOrder(g *Graph, preferredOrder []string) []string {
	sourceOrder := buildTopLaneOrder(g)
	if len(preferredOrder) == 0 {
		return sourceOrder
	}

	sourceLaneIds := map[string]bool{}
	for _, laneId := range sourceOrder {
		sourceLaneIds[laneId] = true
	}
	seen := map[string]bool{}
	resolved := []string{}
	for _, laneId := range preferredOrder {
		if !sourceLaneIds[laneId] || seen[laneId] {
			continue
		}
		seen[laneId] = true
		resolved = append(resolved, laneId)
	}
	for _, laneId := range sourceOrder {
		if seen[laneId] {
			continue
		}
		resolved = append(resolved, laneId)
	}
	return resolved
}
