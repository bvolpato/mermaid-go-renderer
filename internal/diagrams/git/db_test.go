package git

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

var errStopAfterParse = errors.New("stop after parse")

// parseOnly installs a gitGraph definition that keeps the db and stops
// before the draw step. It returns a function that runs the pipeline of
// render.Render up to the end of the parse step. The function can run in
// several goroutines: the id of the svg tells the renders apart, and the id
// has no effect on the db.
func parseOnly(t testing.TB) func(id, text string, site *js.Obj[any]) (*DB, error) {
	t.Helper()
	def, ok := render.Lookup("gitGraph")
	if !ok {
		t.Fatal("gitGraph is not registered")
	}
	origParse, origDraw := def.Parse, def.Draw
	t.Cleanup(func() { def.Parse, def.Draw = origParse, origDraw })
	var captured sync.Map
	def.Parse = func(ctx *render.Context, db render.DB, text string) error {
		captured.Store(ctx.ID, db.(*DB))
		return origParse(ctx, db, text)
	}
	def.Draw = func(*render.Context, render.DB, string) error { return errStopAfterParse }
	return func(id, text string, site *js.Obj[any]) (*DB, error) {
		_, err := render.Render(text, render.Options{ID: id, Config: site})
		if errors.Is(err, errStopAfterParse) {
			err = nil
		}
		var d *DB
		if v, ok := captured.LoadAndDelete(id); ok {
			d = v.(*DB)
		}
		return d, err
	}
}

// commitJSON is a commit as JSON.stringify prints it.
func commitJSON(c *Commit) map[string]any {
	m := map[string]any{
		"id":      c.ID,
		"message": c.Message,
		"seq":     c.Seq,
		"type":    c.Type,
		"tags":    append([]string{}, c.Tags...),
		"parents": append([]string{}, c.Parents...),
		"branch":  c.Branch,
	}
	if c.CustomType != nil {
		m["customType"] = *c.CustomType
	}
	if c.HasCustomID {
		m["customId"] = c.CustomID
	}
	return m
}

// dbJSON reads the db in the form of the golden files.
func dbJSON(t testing.TB, d *DB) map[string]any {
	t.Helper()
	commits := []any{}
	d.GetCommits().Each(func(key string, c *Commit) {
		m := commitJSON(c)
		m["key"] = key
		commits = append(commits, m)
	})
	branchHeads := []any{}
	d.GetBranches().Each(func(name, id string) {
		var head any
		if id != "" {
			head = id
		}
		branchHeads = append(branchHeads, []any{name, head})
	})
	var head any
	if h := d.GetHead(); h != nil {
		head = commitJSON(h)
	}
	dump := map[string]any{
		"commits":       commits,
		"branches":      d.GetBranchesAsObjArray(),
		"branchHeads":   branchHeads,
		"direction":     d.GetDirection(),
		"currentBranch": d.GetCurrentBranch(),
		"head":          head,
		"title":         d.GetDiagramTitle(),
		"accTitle":      d.GetAccTitle(),
		"accDescr":      d.GetAccDescription(),
	}
	// A round trip through JSON gives the types of the golden file.
	raw, err := json.Marshal(dump)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

type dbCase struct {
	Name string `json:"name"`
	Text string `json:"text"`
	// Config is the site config: the argument of mermaid.initialize.
	Config     json.RawMessage `json:"config"`
	DB         map[string]any  `json:"db"`
	Error      string          `json:"error"`
	ParseError bool            `json:"parseError"`
}

// checkDBCase runs one case. It returns "" when the case passes, and the
// description of the difference otherwise.
func checkDBCase(t testing.TB, run func(id, text string, site *js.Obj[any]) (*DB, error), i int, c dbCase) string {
	t.Helper()
	var site *js.Obj[any]
	if len(c.Config) > 0 {
		v, err := js.ParseJSON(c.Config)
		if err != nil {
			return fmt.Sprintf("case %d: config: %v", i, err)
		}
		site = v.(*js.Obj[any])
	}
	d, err := run(fmt.Sprintf("case-%d", i), c.Text, site)
	label := c.Name
	if label == "" {
		label = c.Text
	}
	switch {
	case c.ParseError:
		if err == nil || !strings.HasPrefix(err.Error(), "Parsing failed") {
			return fmt.Sprintf("case %d: upstream reports a parse error, got %v\ncase: %q", i, err, label)
		}
		return ""
	case c.Error != "":
		if err == nil || err.Error() != c.Error {
			return fmt.Sprintf("case %d: error\n got: %v\nwant: %s\ncase: %q", i, err, c.Error, label)
		}
	default:
		if err != nil {
			return fmt.Sprintf("case %d: upstream parses, got error %v\ncase: %q", i, err, label)
		}
	}
	if d == nil {
		return fmt.Sprintf("case %d: the parse step did not run\ncase: %q", i, label)
	}
	if got := dbJSON(t, d); !reflect.DeepEqual(got, c.DB) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(c.DB)
		return fmt.Sprintf("case %d: db\n got: %s\nwant: %s\ncase: %q", i, gotJSON, wantJSON, label)
	}
	return ""
}

// checkDBCases runs the cases on all processors and reports the failures in
// the order of the cases.
func checkDBCases(t *testing.T, cases []dbCase) {
	t.Helper()
	run := parseOnly(t)
	failures := make([]string, len(cases))
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				failures[i] = checkDBCase(t, run, i, cases[i])
			}
		}()
	}
	for i := range cases {
		next <- i
	}
	close(next)
	wg.Wait()
	failed := 0
	for _, f := range failures {
		if f == "" {
			continue
		}
		failed++
		if failed <= 20 {
			t.Error(f)
		}
	}
	if failed > 20 {
		t.Errorf("%d more failures", failed-20)
	}
}

// TestDBNodeGolden compares the db after the parse step with the db of
// upstream mermaid, for the corpus, for seeded mutations of it and for the
// error cases of the db. The golden file comes from
// tools/reference/gen-gitgraph-db.mjs. That generator runs in node, where
// DOMPurify is the identity function: no input has markup.
func TestDBNodeGolden(t *testing.T) {
	var golden struct {
		Cases []dbCase `json:"cases"`
	}
	readGolden(t, "db_node.json.gz", &golden)
	if len(golden.Cases) < 4000 {
		t.Fatalf("golden file has %d cases", len(golden.Cases))
	}
	checkDBCases(t, golden.Cases)
}

// TestDBChromiumGolden compares the db after the parse step with the db of
// upstream mermaid in Chromium, for every fixture of the corpus family and
// for cases with markup, which the real DOMPurify changes. The golden file
// comes from tools/reference/gitgraph-db-oracle.mjs. Each case ran in a new
// page, so the generated commit ids are those of a reference render.
func TestDBChromiumGolden(t *testing.T) {
	var golden struct {
		Cases []dbCase `json:"cases"`
	}
	readGolden(t, "db_chromium.json.gz", &golden)
	corpus := 0
	for _, c := range golden.Cases {
		if strings.HasPrefix(c.Name, "corpus/") {
			corpus++
		}
	}
	if corpus != 135 {
		t.Fatalf("golden file has %d corpus fixtures", corpus)
	}
	checkDBCases(t, golden.Cases)
}
