package gantt

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// oracleDate is a task date in the dump of the oracle.
type oracleDate struct {
	Undefined bool
	Null      bool
	Invalid   bool
	MS        *float64
	ISO       string
	Other     string
}

type oracleTask struct {
	ID            *string
	HasID         bool
	Section       string
	Type          string
	Task          string
	Classes       []string
	Active        bool
	Done          bool
	Crit          bool
	Milestone     bool
	Vert          bool
	Order         float64
	Processed     bool
	ManualEndTime bool
	StartTime     oracleDate
	EndTime       oracleDate
	RenderEndTime oracleDate
	PrevTaskID    *string
	HasPrevTaskID bool
	Raw           string
}

type oracleDB struct {
	ParseError        string
	TasksError        string
	DateFormat        string
	AxisFormat        string
	TickInterval      *string
	TodayMarker       string
	Excludes          []string
	Includes          []string
	Sections          []string
	Links             [][2]string
	Title             string
	AccTitle          string
	AccDescription    string
	DisplayMode       string
	Weekday           string
	InclusiveEndDates bool
	TopAxis           bool
	Tasks             []oracleTask
}

type oracleCase struct {
	DB    oracleDB
	SVG   *string
	Error *string
}

type oracleFile struct {
	Now      string
	Timezone string
	Cases    map[string]oracleCase
}

func loadOracle(t testing.TB) *oracleFile {
	t.Helper()
	f, err := os.Open("testdata/oracle.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var o oracleFile
	if err := json.Unmarshal(data, &o); err != nil {
		t.Fatal(err)
	}
	if o.Now != "2026-01-15T12:00:00.000Z" || o.Timezone != "UTC" {
		t.Fatalf("the oracle ran at %s in zone %s", o.Now, o.Timezone)
	}
	return &o
}

// caseText reads the text of an oracle case: a fixture of the corpus or an
// extra case of this package.
func caseText(t testing.TB, name string) string {
	t.Helper()
	var path string
	if rest, ok := strings.CutPrefix(name, "corpus/"); ok {
		path = filepath.Join("..", "..", "..", "testdata", "corpus", "mermaid-e2e", filepath.FromSlash(rest)+".mmd")
	} else {
		path = filepath.Join("testdata", filepath.FromSlash(name)+".mmd")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func caseNames(o *oracleFile) []string {
	names := make([]string, 0, len(o.Cases))
	for name := range o.Cases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// parseLikeMermaid does what mermaid.parse(text) does: it resolves the
// config of the text and parses the diagram into a new db.
func parseLikeMermaid(text string) (*DB, error) {
	return parseIn(text, nil, false)
}

// parseIn parses a text into a new db whose local time zone is loc. With
// withTitle, the db gets the title of the front matter first, as in
// mermaid.render.
func parseIn(text string, loc *time.Location, withTitle bool) (*DB, error) {
	resolved, err := config.Resolve(text, nil)
	if err != nil {
		return nil, err
	}
	if _, err := config.DetectType(resolved.Source.Cleaned, resolved.Config); err != nil {
		return nil, err
	}
	ctx := render.NewContext(render.ContextOptions{
		Config:      resolved.Config,
		ConfigState: resolved.State,
		Location:    loc,
		RandomSkip:  1,
	})
	db := NewDB(ctx)
	if withTitle && resolved.Title != "" {
		db.SetDiagramTitle(resolved.Title)
	}
	if err := Parse(db, render.EncodeEntities(resolved.Source.Cleaned)+"\n"); err != nil {
		return nil, err
	}
	return db, nil
}

func sameDate(got Date, want oracleDate, isRenderEnd bool) bool {
	switch {
	case want.Undefined, want.Null:
		// renderEndTime is null when it is not set, and the other two dates
		// are undefined.
		return !got.Defined && want.Null == isRenderEnd
	case want.Invalid:
		return got.Defined && math.IsNaN(got.T)
	case want.MS != nil:
		return got.Defined && got.T == *want.MS
	}
	return false
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// checkDB compares a db after the parse, or the error of the parse, with
// the dump of the db of mermaid. It adds the number of tasks that it
// compared to tasks.
func checkDB(t *testing.T, db *DB, err error, want oracleDB, tasks *int) {
	t.Helper()
	if want.ParseError != "" {
		if err == nil {
			t.Fatalf("the parse succeeded; mermaid fails: %s", want.ParseError)
		}
		if strings.HasPrefix(want.ParseError, "Parse error") || strings.HasPrefix(want.ParseError, "Lexical error") {
			if err.Error() != want.ParseError {
				t.Errorf("error:\n got  %q\n want %q", err.Error(), want.ParseError)
			}
		}
		return
	}
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	check := func(field string, got, want any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v, want %#v", field, got, want)
		}
	}
	check("dateFormat", db.GetDateFormat(), want.DateFormat)
	check("axisFormat", db.GetAxisFormat(), want.AxisFormat)
	tickInterval, hasTickInterval := db.GetTickInterval()
	if (want.TickInterval != nil) != hasTickInterval || (hasTickInterval && tickInterval != *want.TickInterval) {
		t.Errorf("tickInterval: got %q, %v", tickInterval, hasTickInterval)
	}
	check("todayMarker", db.GetTodayMarker(), want.TodayMarker)
	check("excludes", orEmpty(db.GetExcludes()), want.Excludes)
	check("includes", orEmpty(db.GetIncludes()), want.Includes)
	check("sections", orEmpty(db.GetSections()), want.Sections)
	links := [][2]string{}
	db.GetLinks().Each(func(k, v string) { links = append(links, [2]string{k, v}) })
	check("links", links, want.Links)
	check("title", db.GetDiagramTitle(), want.Title)
	check("accTitle", db.GetAccTitle(), want.AccTitle)
	check("accDescription", db.GetAccDescription(), want.AccDescription)
	check("displayMode", db.GetDisplayMode(), want.DisplayMode)
	check("weekday", db.GetWeekday(), want.Weekday)
	check("inclusiveEndDates", db.EndDatesAreInclusive(), want.InclusiveEndDates)
	check("topAxis", db.TopAxisEnabled(), want.TopAxis)

	got, err := db.GetTasks()
	if want.TasksError != "" {
		if err == nil {
			t.Fatalf("getTasks succeeded; mermaid fails: %s", want.TasksError)
		}
		if strings.HasPrefix(want.TasksError, "Invalid date:") || strings.HasPrefix(want.TasksError, "Failed to find") {
			if err.Error() != want.TasksError {
				t.Errorf("error: got %q, want %q", err.Error(), want.TasksError)
			}
		}
		return
	}
	if err != nil {
		t.Fatalf("getTasks: %v", err)
	}
	if len(got) != len(want.Tasks) {
		t.Fatalf("%d tasks, want %d", len(got), len(want.Tasks))
	}
	for i, w := range want.Tasks {
		g := got[i]
		*tasks++
		id := ""
		if w.ID != nil {
			id = *w.ID
		}
		prev := ""
		if w.PrevTaskID != nil {
			prev = *w.PrevTaskID
		}
		if g.ID != id || g.HasID != w.HasID || g.PrevTaskID != prev || g.HasPrevTaskID != w.HasPrevTaskID {
			t.Errorf("task %d: id %q (%v), prev %q (%v); want %q (%v), %q (%v)", i, g.ID, g.HasID, g.PrevTaskID, g.HasPrevTaskID, id, w.HasID, prev, w.HasPrevTaskID)
		}
		if g.Section != w.Section || g.Type != w.Type || g.Task != w.Task || g.Raw != w.Raw {
			t.Errorf("task %d: section %q, type %q, task %q, raw %q; want %q, %q, %q, %q", i, g.Section, g.Type, g.Task, g.Raw, w.Section, w.Type, w.Task, w.Raw)
		}
		if !reflect.DeepEqual(orEmpty(g.Classes), w.Classes) {
			t.Errorf("task %d: classes %v, want %v", i, g.Classes, w.Classes)
		}
		if g.Active != w.Active || g.Done != w.Done || g.Crit != w.Crit || g.Milestone != w.Milestone || g.Vert != w.Vert {
			t.Errorf("task %d: tags %v %v %v %v %v, want %v %v %v %v %v", i, g.Active, g.Done, g.Crit, g.Milestone, g.Vert, w.Active, w.Done, w.Crit, w.Milestone, w.Vert)
		}
		if g.Order != w.Order || g.Processed != w.Processed || g.ManualEndTime != w.ManualEndTime {
			t.Errorf("task %d: order %v, processed %v, manualEndTime %v; want %v, %v, %v", i, g.Order, g.Processed, g.ManualEndTime, w.Order, w.Processed, w.ManualEndTime)
		}
		if !sameDate(g.StartTime, w.StartTime, false) {
			t.Errorf("task %d (%s): startTime %+v, want %+v %s", i, w.Task, g.StartTime, w.StartTime, w.StartTime.ISO)
		}
		if !sameDate(g.EndTime, w.EndTime, false) {
			t.Errorf("task %d (%s): endTime %+v, want %+v %s", i, w.Task, g.EndTime, w.EndTime, w.EndTime.ISO)
		}
		if !sameDate(g.RenderEndTime, w.RenderEndTime, true) {
			t.Errorf("task %d (%s): renderEndTime %+v, want %+v %s", i, w.Task, g.RenderEndTime, w.RenderEndTime, w.RenderEndTime.ISO)
		}
		// The ISO text of the dump must be the time value: this
		// guards the reading of the golden file.
		if w.StartTime.MS != nil && *w.StartTime.MS >= -62135596800000 && *w.StartTime.MS < 253402300800000 {
			if iso := time.UnixMilli(int64(*w.StartTime.MS)).UTC().Format("2006-01-02T15:04:05.000Z"); iso != w.StartTime.ISO {
				t.Errorf("task %d: the dump has %v and %s", i, *w.StartTime.MS, w.StartTime.ISO)
			}
		}
	}
}

// TestDBAgainstMermaid compares the db after the parse with the db of real
// mermaid 12.2.0 in Chromium, for every gantt fixture of the corpus and for
// the extra cases in testdata/cases. tools/reference/gen-gantt-oracle.mjs
// and gantt-oracle-page.js made the golden file.
func TestDBAgainstMermaid(t *testing.T) {
	o := loadOracle(t)
	names := caseNames(o)
	corpus, tasks := 0, 0
	for _, name := range names {
		if strings.HasPrefix(name, "corpus/") {
			corpus++
		}
	}
	if corpus != 52 || len(names) < 140 {
		t.Fatalf("%d cases, %d of the corpus", len(names), corpus)
	}
	for _, name := range names {
		want := o.Cases[name].DB
		t.Run(name, func(t *testing.T) {
			db, err := parseLikeMermaid(caseText(t, name))
			checkDB(t, db, err, want, &tasks)
		})
	}
	if tasks < 500 {
		t.Errorf("only %d tasks compared", tasks)
	}
}

// knownDifferences lists the cases whose SVG differs from Chromium, with
// the reason.
var knownDifferences = map[string]string{
	// The cause is outside of this package.
	"cases/emoji-labels": "the measurement engine gives another width than Chromium for emoji: the class of the task text has width-76.875 where Chromium has width-82.875",
	// The renderer reads the numbers of the gantt config as numbers.
	// Upstream adds them with +, which joins text when a value is a string.
	"cases/config-string-numbers": `a number of the gantt config that is a string: upstream computes "30" + 4 = "304" for barHeight + barGap, and the viewBox is 708 px high where this port has 168 px`,
}

// TestRenderAgainstMermaid renders the extra cases in testdata/cases and
// compares each SVG, byte for byte, with the result of mermaid.render of
// real mermaid 12.2.0 in Chromium. Where mermaid fails, the render must
// fail too. The fixtures of the corpus have the parity command for this.
func TestRenderAgainstMermaid(t *testing.T) {
	o := loadOracle(t)
	// The SVG has the text widths of Chromium on the macOS reference
	// machine. A machine with other fonts compares only the errors.
	reference := fonttest.IsMacOSReference()
	rendered, failed := 0, 0
	for _, name := range caseNames(o) {
		c := o.Cases[name]
		if strings.HasPrefix(name, "corpus/") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if reason, ok := knownDifferences[name]; ok {
				t.Skip(reason)
			}
			res, err := render.Render(caseText(t, name), render.Options{})
			if c.Error != nil {
				failed++
				if err == nil {
					t.Fatalf("the render succeeded; mermaid fails: %s", *c.Error)
				}
				for _, prefix := range []string{"Parse error", "Lexical error", "Invalid date:", "Failed to find"} {
					if strings.HasPrefix(*c.Error, prefix) && err.Error() != *c.Error {
						t.Errorf("error:\n got  %q\n want %q", err.Error(), *c.Error)
					}
				}
				return
			}
			rendered++
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if reference && res.SVG != *c.SVG {
				i := 0
				for i < len(res.SVG) && i < len(*c.SVG) && res.SVG[i] == (*c.SVG)[i] {
					i++
				}
				from := max(0, i-120)
				t.Errorf("the SVG differs at byte %d:\n got  ...%s\n want ...%s", i, clip(res.SVG[from:], 400), clip((*c.SVG)[from:], 400))
			}
		})
	}
	if rendered < 70 || failed < 8 {
		t.Errorf("%d cases rendered, %d expected failures", rendered, failed)
	}
	fonttest.RequireMacOSReference(t)
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

type tzFile struct {
	TZ    string
	Now   string
	Cases []struct {
		Name string
		Text *string
		DB   oracleDB
	}
}

// TestDBInNewYork compares the db after the parse with the db of real
// mermaid 12.2.0 when the local time zone is America/New_York. The browser
// oracle runs in UTC only, so tools/reference/gen-gantt-db-tz.mjs made this
// golden file with the gantt code of mermaid in node. The cases are the
// corpus fixtures, the extra cases, and cases for local time near the
// changes of the offset.
func TestDBInNewYork(t *testing.T) {
	f, err := os.Open("testdata/db_new_york.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var golden tzFile
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.TZ != "America/New_York" || golden.Now != "2026-01-15T12:00:00.000Z" {
		t.Fatalf("the generator ran at %s in zone %s", golden.Now, golden.TZ)
	}
	loc, err := time.LoadLocation(golden.TZ)
	if err != nil {
		t.Fatal(err)
	}
	if len(golden.Cases) < 150 {
		t.Fatalf("only %d cases", len(golden.Cases))
	}
	tasks := 0
	for _, c := range golden.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var text string
			if c.Text != nil {
				text = *c.Text
			} else {
				text = caseText(t, c.Name)
			}
			db, err := parseIn(text, loc, true)
			checkDB(t, db, err, c.DB, &tasks)
		})
	}
	if tasks < 500 {
		t.Errorf("only %d tasks compared", tasks)
	}
}
