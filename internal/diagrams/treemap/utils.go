// Ported from mermaid 12.2.0, diagrams/treemap/utils.ts (MIT).

package treemap

// hierarchyItem is an element of the flat list that buildHierarchy reads.
type hierarchyItem struct {
	level int
	name  string
	// leaf is true for the type "Leaf".
	leaf bool
	// value is the value of a Leaf.
	value             float64
	classSelector     string
	cssCompiledStyles []string
}

// buildHierarchy converts a flat list of treemap items into a hierarchical
// structure: an item is a child of the nearest section above it that has a
// smaller level.
func buildHierarchy(items []hierarchyItem) []*Node {
	if len(items) == 0 {
		return []*Node{}
	}

	root := []*Node{}
	type entry struct {
		node  *Node
		level int
	}
	var stack []entry

	for _, item := range items {
		node := &Node{Name: item.name}
		if !item.leaf {
			node.Children = []*Node{}
		}
		node.ClassSelector = item.classSelector
		if len(item.cssCompiledStyles) > 0 {
			node.CSSCompiledStyles = item.cssCompiledStyles
		}

		if item.leaf {
			// A Leaf of the grammar always has a value.
			node.Value = item.value
			node.HasValue = true
		}

		// Find the right parent for this node
		for len(stack) > 0 && stack[len(stack)-1].level >= item.level {
			stack = stack[:len(stack)-1]
		}

		if len(stack) == 0 {
			// This is a root node
			root = append(root, node)
		} else {
			// Add as child to the parent
			parent := stack[len(stack)-1].node
			parent.Children = append(parent.Children, node)
		}

		// Only add to stack if it can have children
		if !item.leaf {
			stack = append(stack, entry{node: node, level: item.level})
		}
	}

	return root
}
