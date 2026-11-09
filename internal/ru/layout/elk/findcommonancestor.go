// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/elk/find-common-ancestor.ts (MIT).

package elk

// TreeData is the subgraph parent lookup: the parent of each node that is
// in a subgraph, and the children of each subgraph in declaration order.
//
// Upstream uses plain objects. An id that is the name of a member of
// Object.prototype (for example "constructor") finds that member there.
// These maps do not have that behavior.
type TreeData struct {
	ParentByID   map[string]string
	ChildrenByID map[string][]string
}

func newTreeData() *TreeData {
	return &TreeData{ParentByID: map[string]string{}, ChildrenByID: map[string][]string{}}
}

// errParentCycle is the panic value for a parent chain that has a cycle.
// Upstream does not terminate on such a chain, or fails with a stack
// overflow. The entry points of the layout recover the panic and return
// the error.
type errParentCycle struct{}

func (errParentCycle) Error() string {
	return "elk layout: the parents of the nodes form a cycle"
}

// walkGuard counts the steps of a walk along the parent chain. A walk that
// takes more steps than there are nodes with a parent is in a cycle.
type walkGuard struct {
	steps, limit int
}

func newWalkGuard(tree *TreeData) walkGuard {
	return walkGuard{limit: len(tree.ParentByID) + 2}
}

func (g *walkGuard) step() {
	g.steps++
	if g.steps > g.limit {
		panic(errParentCycle{})
	}
}

// findCommonAncestor returns the id of the lowest subgraph that contains
// both nodes, or "root". An empty id stands for undefined.
func findCommonAncestor(id1, id2 string, tree *TreeData) string {
	parentById := tree.ParentByID
	visited := map[string]bool{}
	currentId := id1

	// Edge case with self edges
	if id1 == id2 {
		if p := parentById[id1]; p != "" {
			return p
		}
		return "root"
	}

	guard := newWalkGuard(tree)
	for currentId != "" {
		guard.step()
		visited[currentId] = true
		if currentId == id2 {
			return currentId
		}
		currentId = parentById[currentId]
	}

	currentId = id2
	guard = newWalkGuard(tree)
	for currentId != "" {
		guard.step()
		if visited[currentId] {
			return currentId
		}
		currentId = parentById[currentId]
	}

	return "root"
}
