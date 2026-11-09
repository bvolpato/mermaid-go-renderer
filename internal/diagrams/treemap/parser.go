// Ported from mermaid 12.2.0, diagrams/treemap/parser.ts (MIT).

package treemap

// populate fills the db with the data of the treemap AST.
func populate(ast *treemapAST, db *DB) {
	ast.Populate(db)

	var items []hierarchyItem

	// Extract classes and styles from the treemap
	for _, row := range ast.rows {
		if row.classDef != nil {
			db.AddClass(row.classDef.className, row.classDef.styleText)
		}
	}

	// Extract data from each row in the treemap
	for _, row := range ast.rows {
		item := row.item

		if item == nil {
			continue
		}

		level := 0
		if row.indent != nil {
			level = *row.indent
		}
		name := item.name

		// Get styles as a string if they exist
		var styles []string
		if item.classSelector != "" {
			styles = db.GetStylesForClass(item.classSelector)
		}
		var cssCompiledStyles []string
		if len(styles) > 0 {
			cssCompiledStyles = styles
		}

		items = append(items, hierarchyItem{
			level:             level,
			name:              name,
			leaf:              item.leaf,
			value:             item.value,
			classSelector:     item.classSelector,
			cssCompiledStyles: cssCompiledStyles,
		})
	}

	// Convert flat structure to hierarchical
	hierarchyNodes := buildHierarchy(items)

	// Add all nodes to the database
	var addNodesRecursively func(nodes []*Node, level int)
	addNodesRecursively = func(nodes []*Node, level int) {
		for _, node := range nodes {
			db.AddNode(node, level)
			if len(node.Children) > 0 {
				addNodesRecursively(node.Children, level+1)
			}
		}
	}

	addNodesRecursively(hierarchyNodes, 0)
}

// parseText is parser.parse(text): it parses the text and fills the db.
func parseText(db *DB, text string) error {
	ast, err := parseAST(text)
	if err != nil {
		return err
	}
	populate(ast, db)
	return nil
}
