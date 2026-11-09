// Ported from mermaid 12.2.0, diagrams/git/gitGraphAst.ts and gitGraphTypes.ts
// (MIT).

package git

import (
	"errors"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// The values of commitType.
const (
	commitTypeNormal     = 0
	commitTypeReverse    = 1
	commitTypeHighlight  = 2
	commitTypeMerge      = 3
	commitTypeCherryPick = 4
)

// commitTypeByName is the commitType object, read by name as the parser
// adapter does: commitType[name].
var commitTypeByName = map[string]int{
	"NORMAL":      commitTypeNormal,
	"REVERSE":     commitTypeReverse,
	"HIGHLIGHT":   commitTypeHighlight,
	"MERGE":       commitTypeMerge,
	"CHERRY_PICK": commitTypeCherryPick,
}

// CommitDB is the argument of commit.
type CommitDB struct {
	Msg  string
	ID   string // "" is undefined
	Type int
	Tags []string // nil is undefined
}

// BranchDB is the argument of branch.
type BranchDB struct {
	Name  string
	Order float64
}

// MergeDB is the argument of merge.
type MergeDB struct {
	Branch string
	ID     string
	Type   *int     // nil is undefined
	Tags   []string // nil is undefined
}

// CherryPickDB is the argument of cherryPick.
type CherryPickDB struct {
	ID       string // "" is undefined
	TargetID string
	Parent   string   // "" is undefined
	Tags     []string // nil is undefined
}

// Commit is one commit of the graph.
type Commit struct {
	ID      string
	Message string
	Seq     int
	Type    int
	Tags    []string
	Parents []string
	Branch  string
	// CustomType is the type that a merge statement gives. nil is undefined.
	CustomType *int
	// CustomID is customId. Only a merge commit has it: HasCustomID is true
	// then.
	CustomID    bool
	HasCustomID bool
}

// branchConfig is one value of the branchConfig map. order follows the
// dynamic value model: nil is undefined.
type branchConfig struct {
	name  string
	order any
}

// DB is the state of gitGraphAst.ts: the records of the ImperativeState and
// the common db.
type DB struct {
	render.CommonDB

	ctx       *render.Context
	sanitizer *sanitize.Sanitizer
	// gitGraphConfig caches getConfig(). The config does not change while a
	// diagram is parsed and drawn.
	gitGraphConfig *js.Obj[any]

	// mainBranchName is the mainBranchName of the config. badMainBranchName
	// is set when the config value is not a string.
	mainBranchName    string
	badMainBranchName bool

	commits      *js.Map[string, *Commit]
	head         *Commit
	branchConfig *js.Map[string, *branchConfig]
	// branches maps a branch name to the id of its last commit. "" is null:
	// a commit id is never empty.
	branches   *js.Map[string, string]
	currBranch string
	direction  string
	seq        int
	options    any
}

// newDB is the init function of the ImperativeState, which clear() runs.
func newDB(ctx *render.Context) *DB {
	// The DOMPurify instance of the page: the hooks that an earlier
	// sanitize call installed change the later calls.
	d := &DB{ctx: ctx, sanitizer: ctx.Sanitizer}
	if d.sanitizer == nil {
		d.sanitizer = sanitize.New()
	}
	d.CommonDB.Sanitize = func(text string) string {
		return d.sanitizer.SanitizeText(text, ctx.Config)
	}
	cfg := d.GetConfig()
	mainBranchName, ok := cfg.Get("mainBranchName").(string)
	if !ok {
		// Upstream keeps the value as the map key. drawText then fails on
		// it, so the name only has to be stable here.
		mainBranchName = js.ToString(cfg.Get("mainBranchName"))
		d.badMainBranchName = true
	}
	mainBranchOrder := cfg.Get("mainBranchOrder")
	d.mainBranchName = mainBranchName
	d.commits = js.NewMap[string, *Commit]()
	d.head = nil
	d.branchConfig = js.NewMap[string, *branchConfig]()
	d.branchConfig.Set(mainBranchName, &branchConfig{name: mainBranchName, order: mainBranchOrder})
	d.branches = js.NewMap[string, string]()
	d.branches.Set(mainBranchName, "")
	d.currBranch = mainBranchName
	d.direction = "LR"
	d.seq = 0
	d.options = js.NewObj[any]()
	return d
}

// GetConfig is getConfig(): cleanAndMerge({...DEFAULT_CONFIG.gitGraph,
// ...getConfig().gitGraph}). The spread is shallow, and cleanAndMerge drops
// the properties that are undefined.
func (d *DB) GetConfig() *js.Obj[any] {
	if d.gitGraphConfig != nil {
		return d.gitGraphConfig
	}
	merged := js.NewObj[any]()
	spread := func(v any) {
		o, ok := v.(*js.Obj[any])
		if !ok || o == nil {
			return
		}
		o.Each(func(key string, value any) {
			merged.Set(key, value)
		})
	}
	spread(config.DefaultSection("gitGraph"))
	if d.ctx != nil && d.ctx.Config != nil {
		spread(d.ctx.Config.Get("gitGraph"))
	}
	out := js.NewObj[any]()
	merged.Each(func(key string, value any) {
		if value == nil {
			return
		}
		out.Set(key, js.Clone(value))
	})
	d.gitGraphConfig = out
	return out
}

// sanitizeText is common.sanitizeText(text, getConfig()), with the gitGraph
// config as upstream passes it.
func (d *DB) sanitizeText(text string) (string, error) {
	return d.sanitizer.SanitizeTextErr(text, d.GetConfig())
}

// sanitizeTags is tags?.map((tag) => common.sanitizeText(tag, config)).
func (d *DB) sanitizeTags(tags []string) ([]string, error) {
	if tags == nil {
		return nil, nil
	}
	out := make([]string, len(tags))
	for i, tag := range tags {
		s, err := d.sanitizeText(tag)
		if err != nil {
			return nil, err
		}
		out[i] = s
	}
	return out, nil
}

// getID is getID(): random({ length: 7 }).
func (d *DB) getID() string {
	return utils.Random(d.ctx, 7)
}

// SetDirection is setDirection(dir).
func (d *DB) SetDirection(dir string) {
	d.direction = dir
}

// SetOptions is setOptions(rawOptString).
func (d *DB) SetOptions(rawOptString string) {
	rawOptString = js.TrimSpace(rawOptString)
	if rawOptString == "" {
		rawOptString = "{}"
	}
	options, err := js.ParseJSON([]byte(rawOptString))
	if err != nil {
		// Upstream logs the error and keeps the old options.
		return
	}
	d.options = options
}

// GetOptions is getOptions().
func (d *DB) GetOptions() any {
	return d.options
}

// Commit is commit(commitDB).
func (d *DB) Commit(commitDB CommitDB) error {
	msg := commitDB.Msg
	id := commitDB.ID
	typ := commitDB.Type
	tags := commitDB.Tags

	var err error
	if id, err = d.sanitizeText(id); err != nil {
		return err
	}
	if msg, err = d.sanitizeText(msg); err != nil {
		return err
	}
	if tags, err = d.sanitizeTags(tags); err != nil {
		return err
	}
	if id == "" {
		id = js.Str(float64(d.seq)) + "-" + d.getID()
	}
	if tags == nil {
		tags = []string{}
	}
	parents := []string{}
	if d.head != nil {
		parents = []string{d.head.ID}
	}
	newCommit := &Commit{
		ID:      id,
		Message: msg,
		Seq:     d.seq,
		Type:    typ,
		Tags:    tags,
		Parents: parents,
		Branch:  d.currBranch,
	}
	d.seq++
	d.head = newCommit
	d.commits.Set(newCommit.ID, newCommit)
	d.branches.Set(d.currBranch, newCommit.ID)
	return nil
}

// Branch is branch(branchDB).
func (d *DB) Branch(branchDB BranchDB) error {
	name := branchDB.Name
	order := branchDB.Order
	name, err := d.sanitizeText(name)
	if err != nil {
		return err
	}
	if d.branches.Has(name) {
		return errors.New(`Trying to create an existing branch. (Help: Either use a new name if you want create a new branch or try using "checkout ` + name + `")`)
	}

	headID := ""
	if d.head != nil {
		headID = d.head.ID
	}
	d.branches.Set(name, headID)
	d.branchConfig.Set(name, &branchConfig{name: name, order: order})
	return d.Checkout(name)
}

// Merge is merge(mergeDB).
func (d *DB) Merge(mergeDB MergeDB) error {
	otherBranch := mergeDB.Branch
	customID := mergeDB.ID
	overrideType := mergeDB.Type
	customTags := mergeDB.Tags
	otherBranch, err := d.sanitizeText(otherBranch)
	if err != nil {
		return err
	}
	if customID != "" {
		if customID, err = d.sanitizeText(customID); err != nil {
			return err
		}
	}
	currentBranchCheck := d.branches.Get(d.currBranch)
	otherBranchCheck := d.branches.Get(otherBranch)
	var currentCommit, otherCommit *Commit
	if currentBranchCheck != "" {
		currentCommit = d.commits.Get(currentBranchCheck)
	}
	if otherBranchCheck != "" {
		otherCommit = d.commits.Get(otherBranchCheck)
	}
	if currentCommit != nil && otherCommit != nil && currentCommit.Branch == otherBranch {
		return errors.New("Cannot merge branch '" + otherBranch + "' into itself.")
	}
	if d.currBranch == otherBranch {
		return errors.New(`Incorrect usage of "merge". Cannot merge a branch to itself`)
	}
	if currentCommit == nil {
		return errors.New(`Incorrect usage of "merge". Current branch (` + d.currBranch + `)has no commits`)
	}
	if !d.branches.Has(otherBranch) {
		return errors.New(`Incorrect usage of "merge". Branch to be merged (` + otherBranch + `) does not exist`)
	}
	if otherCommit == nil {
		return errors.New(`Incorrect usage of "merge". Branch to be merged (` + otherBranch + `) has no commits`)
	}
	if currentCommit == otherCommit {
		return errors.New(`Incorrect usage of "merge". Both branches have same head`)
	}
	if customID != "" && d.commits.Has(customID) {
		return errors.New(`Incorrect usage of "merge". Commit with id:` + customID + ` already exists, use different custom id`)
	}

	verifiedBranch := otherBranchCheck // figure out a cleaner way to do this

	id := customID
	if id == "" {
		id = js.Str(float64(d.seq)) + "-" + d.getID()
	}
	parents := []string{}
	if d.head != nil {
		parents = []string{d.head.ID, verifiedBranch}
	}
	if customTags == nil {
		customTags = []string{}
	}
	commit := &Commit{
		ID:          id,
		Message:     "merged branch " + otherBranch + " into " + d.currBranch,
		Seq:         d.seq,
		Parents:     parents,
		Branch:      d.currBranch,
		Type:        commitTypeMerge,
		CustomType:  overrideType,
		CustomID:    customID != "",
		HasCustomID: true,
		Tags:        customTags,
	}
	d.seq++
	d.head = commit
	d.commits.Set(commit.ID, commit)
	d.branches.Set(d.currBranch, commit.ID)
	return nil
}

// CherryPick is cherryPick(cherryPickDB).
func (d *DB) CherryPick(cherryPickDB CherryPickDB) error {
	sourceID := cherryPickDB.ID
	targetID := cherryPickDB.TargetID
	tags := cherryPickDB.Tags
	parentCommitID := cherryPickDB.Parent
	var err error
	if sourceID, err = d.sanitizeText(sourceID); err != nil {
		return err
	}
	if targetID, err = d.sanitizeText(targetID); err != nil {
		return err
	}

	if tags, err = d.sanitizeTags(tags); err != nil {
		return err
	}

	if parentCommitID, err = d.sanitizeText(parentCommitID); err != nil {
		return err
	}

	if sourceID == "" || !d.commits.Has(sourceID) {
		return errors.New(`Incorrect usage of "cherryPick". Source commit id should exist and provided`)
	}

	sourceCommit := d.commits.Get(sourceID)
	if sourceCommit == nil {
		return errors.New(`Incorrect usage of "cherryPick". Source commit id should exist and provided`)
	}
	if parentCommitID != "" && !includes(sourceCommit.Parents, parentCommitID) {
		return errors.New("Invalid operation: The specified parent commit is not an immediate parent of the cherry-picked commit.")
	}
	sourceCommitBranch := sourceCommit.Branch
	if sourceCommit.Type == commitTypeMerge && parentCommitID == "" {
		return errors.New("Incorrect usage of cherry-pick: If the source commit is a merge commit, an immediate parent commit must be specified.")
	}
	if targetID == "" || !d.commits.Has(targetID) {
		// cherry-pick source commit to current branch

		if sourceCommitBranch == d.currBranch {
			return errors.New(`Incorrect usage of "cherryPick". Source commit is already on current branch`)
		}
		currentCommitID := d.branches.Get(d.currBranch)
		if currentCommitID == "" {
			return errors.New(`Incorrect usage of "cherry-pick". Current branch (` + d.currBranch + `)has no commits`)
		}

		currentCommit := d.commits.Get(currentCommitID)
		if currentCommit == nil {
			return errors.New(`Incorrect usage of "cherry-pick". Current branch (` + d.currBranch + `)has no commits`)
		}
		id := js.Str(float64(d.seq)) + "-" + d.getID()
		parents := []string{}
		if d.head != nil {
			parents = []string{d.head.ID, sourceCommit.ID}
		}
		var commitTags []string
		if tags != nil {
			// tags.filter(Boolean)
			commitTags = []string{}
			for _, tag := range tags {
				if tag != "" {
					commitTags = append(commitTags, tag)
				}
			}
		} else {
			tag := "cherry-pick:" + sourceCommit.ID
			if sourceCommit.Type == commitTypeMerge {
				tag += "|parent:" + parentCommitID
			}
			commitTags = []string{tag}
		}
		commit := &Commit{
			ID:      id,
			Message: "cherry-picked " + sourceCommit.Message + " into " + d.currBranch,
			Seq:     d.seq,
			Parents: parents,
			Branch:  d.currBranch,
			Type:    commitTypeCherryPick,
			Tags:    commitTags,
		}
		d.seq++

		d.head = commit
		d.commits.Set(commit.ID, commit)
		d.branches.Set(d.currBranch, commit.ID)
	}
	return nil
}

// Checkout is checkout(branch).
func (d *DB) Checkout(branch string) error {
	branch, err := d.sanitizeText(branch)
	if err != nil {
		return err
	}
	if !d.branches.Has(branch) {
		return errors.New(`Trying to checkout branch which is not yet created. (Help try using "branch ` + branch + `")`)
	}
	d.currBranch = branch
	id := d.branches.Get(d.currBranch)
	if id == "" {
		d.head = nil
	} else {
		d.head = d.commits.Get(id)
	}
	return nil
}

func includes(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// upsert is upsert(arr, key, newVal).
func upsert(arr []*Commit, key, newVal *Commit) []*Commit {
	index := -1
	for i, c := range arr {
		if c == key {
			index = i
			break
		}
	}
	if index == -1 {
		return append(arr, newVal)
	}
	arr[index] = newVal
	return arr
}

// uniqBy is uniqBy(list, (c) => c.id).
func uniqBy(list []*Commit) []*Commit {
	recordMap := map[string]bool{}
	out := []*Commit{}
	for _, item := range list {
		key := item.ID
		if !recordMap[key] {
			recordMap[key] = true
			out = append(out, item)
		}
	}
	return out
}

// prettyPrintCommitHistory is prettyPrintCommitHistory(commitArr). Upstream
// writes each line to the debug log. Here the lines are returned.
func (d *DB) prettyPrintCommitHistory(commitArr []*Commit, lines []string) []string {
	// Upstream recurses once per commit. The loop does the same steps.
	for steps := 0; steps <= d.commits.Len()*2+1; steps++ {
		if len(commitArr) == 0 || commitArr[0] == nil {
			return lines
		}
		commit := commitArr[0]
		for _, c := range commitArr {
			if c == nil {
				return lines
			}
			if commit.Seq > c.Seq {
				continue
			}
			commit = c
		}
		var line strings.Builder
		for _, c := range commitArr {
			if c == commit {
				line.WriteString("\t*")
			} else {
				line.WriteString("\t|")
			}
		}
		// The branch names are not added: upstream enumerates the Map with
		// for...in, which yields nothing.
		lines = append(lines, line.String()+" "+commit.ID+" "+js.Str(float64(commit.Seq)))
		switch {
		case len(commit.Parents) == 2 && commit.Parents[0] != "" && commit.Parents[1] != "":
			newCommit := d.commits.Get(commit.Parents[0])
			commitArr = upsert(commitArr, commit, newCommit)
			commitArr = append(commitArr, d.commits.Get(commit.Parents[1]))
		case len(commit.Parents) == 0:
			return lines
		default:
			if commit.Parents[0] != "" {
				newCommit := d.commits.Get(commit.Parents[0])
				commitArr = upsert(commitArr, commit, newCommit)
			}
		}
		for _, c := range commitArr {
			if c == nil {
				return lines
			}
		}
		commitArr = uniqBy(commitArr)
	}
	return lines
}

// PrettyPrint is prettyPrint(). It returns the lines that upstream logs.
func (d *DB) PrettyPrint() []string {
	commits := d.GetCommitsArray()
	if len(commits) == 0 {
		return nil
	}
	return d.prettyPrintCommitHistory([]*Commit{commits[0]}, nil)
}

// GetBranchesAsObjArray is getBranchesAsObjArray(). It returns the branch
// names in drawing order.
func (d *DB) GetBranchesAsObjArray() []string {
	type entry struct {
		name  string
		order any
	}
	values := d.branchConfig.Values()
	branchesArray := make([]entry, len(values))
	for i, bc := range values {
		if !js.IsNullish(bc.order) {
			branchesArray[i] = entry{name: bc.name, order: bc.order}
			continue
		}
		branchesArray[i] = entry{name: bc.name, order: js.ParseFloat("0." + js.Str(float64(i)))}
	}
	orZero := func(order any) float64 {
		if js.IsNullish(order) {
			return 0
		}
		return js.ToNumber(order)
	}
	js.SortFunc(branchesArray, func(a, b entry) float64 {
		return orZero(a.order) - orZero(b.order)
	})
	names := make([]string, len(branchesArray))
	for i, e := range branchesArray {
		names[i] = e.name
	}
	return names
}

// GetBranches is getBranches(). A value of "" is null.
func (d *DB) GetBranches() *js.Map[string, string] {
	return d.branches
}

// GetCommits is getCommits().
func (d *DB) GetCommits() *js.Map[string, *Commit] {
	return d.commits
}

// GetCommitsArray is getCommitsArray().
func (d *DB) GetCommitsArray() []*Commit {
	commitArr := d.commits.Values()
	js.SortFunc(commitArr, func(a, b *Commit) float64 {
		return float64(a.Seq - b.Seq)
	})
	return commitArr
}

// GetCurrentBranch is getCurrentBranch().
func (d *DB) GetCurrentBranch() string {
	return d.currBranch
}

// GetDirection is getDirection().
func (d *DB) GetDirection() string {
	return d.direction
}

// GetHead is getHead(). The result is nil when the branch has no commit.
func (d *DB) GetHead() *Commit {
	return d.head
}
