// Ported from mermaid 12.2.0, diagrams/git/gitGraphParser.ts and
// diagrams/common/populateCommonDb.ts (MIT).

package git

// parse is parser.parse(input): it parses the text and fills the db.
func parse(db *DB, input string) error {
	ast, err := parseGitGraphText(input)
	if err != nil {
		return err
	}
	return populate(ast, db)
}

// populateCommonDb is populateCommonDb(ast, db).
func populateCommonDb(ast *gitGraphAST, db *DB) {
	if ast.accDescr != nil && *ast.accDescr != "" {
		db.SetAccDescription(*ast.accDescr)
	}
	if ast.accTitle != nil && *ast.accTitle != "" {
		db.SetAccTitle(*ast.accTitle)
	}
	if ast.title != nil && *ast.title != "" {
		db.SetDiagramTitle(*ast.title)
	}
}

// populate is populate(ast, db).
func populate(ast *gitGraphAST, db *DB) error {
	populateCommonDb(ast, db)
	if ast.dir != nil && *ast.dir != "" {
		db.SetDirection(*ast.dir)
	}
	for _, statement := range ast.statements {
		if err := parseStatement(statement, db); err != nil {
			return err
		}
	}
	return nil
}

// parseStatement is parseStatement(statement, db).
func parseStatement(statement statement, db *DB) error {
	switch stmt := statement.(type) {
	case *commitAST:
		return db.Commit(parseCommit(stmt))
	case *branchAST:
		return db.Branch(parseBranch(stmt))
	case *mergeAST:
		return db.Merge(parseMerge(stmt))
	case *checkoutAST:
		return db.Checkout(parseCheckout(stmt))
	case *cherryPickingAST:
		return db.CherryPick(parseCherryPicking(stmt))
	}
	return nil
}

// orEmpty is `value ?? ""`. An undefined id, message or parent acts as the
// empty string in the db: sanitizeText returns both unchanged, and the db
// tests them for truthiness only.
func orEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// commitTypeOf is commitType[name].
func commitTypeOf(name string) int {
	return commitTypeByName[name]
}

// parseCommit is parseCommit(commit).
func parseCommit(commit *commitAST) CommitDB {
	typ := commitTypeNormal
	if commit.typ != nil {
		typ = commitTypeOf(*commit.typ)
	}
	return CommitDB{
		ID:   orEmpty(commit.id),
		Msg:  orEmpty(commit.message),
		Type: typ,
		Tags: commit.tags,
	}
}

// parseBranch is parseBranch(branch).
func parseBranch(branch *branchAST) BranchDB {
	order := 0.0
	if branch.order != nil {
		order = *branch.order
	}
	return BranchDB{
		Name:  branch.name,
		Order: order,
	}
}

// parseMerge is parseMerge(merge).
func parseMerge(merge *mergeAST) MergeDB {
	var typ *int
	if merge.typ != nil {
		t := commitTypeOf(*merge.typ)
		typ = &t
	}
	return MergeDB{
		Branch: merge.branch,
		ID:     orEmpty(merge.id),
		Type:   typ,
		Tags:   merge.tags,
	}
}

// parseCheckout is parseCheckout(checkout).
func parseCheckout(checkout *checkoutAST) string {
	return checkout.branch
}

// parseCherryPicking is parseCherryPicking(cherryPicking).
func parseCherryPicking(cherryPicking *cherryPickingAST) CherryPickDB {
	tags := cherryPicking.tags
	if len(tags) == 0 {
		tags = nil
	}
	return CherryPickDB{
		ID:       orEmpty(cherryPicking.id),
		TargetID: "",
		Tags:     tags,
		Parent:   orEmpty(cherryPicking.parent),
	}
}
