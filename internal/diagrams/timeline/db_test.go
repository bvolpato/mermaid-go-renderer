package timeline

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// goldenTask is one task of db.getTasks() in the golden file.
type goldenTask struct {
	ID      int      `json:"id"`
	Section string   `json:"section"`
	Type    string   `json:"type"`
	Task    string   `json:"task"`
	Score   any      `json:"score"`
	Events  []string `json:"events"`
}

// goldenDB is the content of the timeline db of real mermaid.
type goldenDB struct {
	Direction      string       `json:"direction"`
	Sections       []string     `json:"sections"`
	Tasks          []goldenTask `json:"tasks"`
	Title          string       `json:"title"`
	AccTitle       string       `json:"accTitle"`
	AccDescription string       `json:"accDescription"`
}

type goldenCase struct {
	Name  string    `json:"name"`
	Type  string    `json:"type"`
	Text  string    `json:"text"`
	Error *string   `json:"error"`
	DB    *goldenDB `json:"db"`
}

func loadGolden(t testing.TB) []goldenCase {
	t.Helper()
	f, err := os.Open("testdata/db.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases []goldenCase `json:"cases"`
	}
	if err := json.NewDecoder(zr).Decode(&golden); err != nil {
		t.Fatal(err)
	}
	return golden.Cases
}

// parseLikeMermaid runs the steps of mermaid.parse(text): the config of the
// front matter and of the directives, the type detection, and the parser on
// the cleaned text.
func parseLikeMermaid(text string) (*DB, error) {
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)
	resolved, err := config.Resolve(text, site)
	if err != nil {
		return nil, err
	}
	if resolved.DiagramType != "timeline" {
		return nil, &config.Error{Message: "detected type " + resolved.DiagramType}
	}
	ctx := &render.Context{Config: resolved.Config, ConfigState: resolved.State, Sanitizer: sanitize.New()}
	db := NewDB(ctx)
	if err := parseText(db, render.EncodeEntities(resolved.Code)+"\n"); err != nil {
		return nil, err
	}
	return db, nil
}

// TestDBAgainstMermaid compares the db after a parse with the db of real
// mermaid in Chromium, for the corpus fixtures, mutations of them, random
// documents and hand-written inputs. Regenerate the golden file with
// node tools/reference/gen-timeline-journey-db.mjs.
func TestDBAgainstMermaid(t *testing.T) {
	cases := loadGolden(t)
	if len(cases) < 300 {
		t.Fatalf("only %d golden cases", len(cases))
	}
	parsed := 0
	for _, c := range cases {
		db, err := parseLikeMermaid(c.Text)
		if c.Error != nil {
			if err == nil {
				t.Errorf("%s: no error, mermaid has %q\n%s", c.Name, *c.Error, c.Text)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: error %q, mermaid has none\n%s", c.Name, err, c.Text)
			continue
		}
		parsed++
		got := goldenDB{
			Direction:      db.GetDirection(),
			Sections:       append([]string{}, db.GetSections()...),
			Tasks:          []goldenTask{},
			Title:          db.GetCommonDb().GetDiagramTitle(),
			AccTitle:       db.GetCommonDb().GetAccTitle(),
			AccDescription: db.GetCommonDb().GetAccDescription(),
		}
		for _, task := range db.GetTasks() {
			got.Tasks = append(got.Tasks, goldenTask{
				ID:      task.ID,
				Section: task.Section,
				Type:    task.Type,
				Task:    task.Task,
				Score:   task.Score,
				Events:  append([]string{}, task.Events...),
			})
		}
		// The oracle parses all cases in one page, and clear() of the db
		// does not reset currentTaskId. So the ids of the oracle continue
		// from one case to the next. Compare them from the first task.
		want := *c.DB
		want.Tasks = append([]goldenTask{}, want.Tasks...)
		if len(want.Tasks) > 0 {
			base := want.Tasks[0].ID
			for i := range want.Tasks {
				want.Tasks[i].ID -= base
			}
		}
		if !reflect.DeepEqual(got, want) {
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(want)
			t.Errorf("%s: db differs\n got  %s\n want %s\n%s", c.Name, gotJSON, wantJSON, c.Text)
		}
	}
	if parsed < 200 {
		t.Fatalf("only %d cases parse", parsed)
	}
}

// TestErrorMessagesAgainstMermaid compares the messages of the errors that
// come from the parser and from the db.
func TestErrorMessagesAgainstMermaid(t *testing.T) {
	checked := 0
	for _, c := range loadGolden(t) {
		if c.Error == nil {
			continue
		}
		_, err := parseLikeMermaid(c.Text)
		if err == nil {
			continue // TestDBAgainstMermaid reports it.
		}
		if _, ok := err.(*config.Error); ok {
			continue // The config package has its own golden tests.
		}
		checked++
		if err.Error() != *c.Error {
			t.Errorf("%s: error message differs\n got  %q\n want %q", c.Name, err.Error(), *c.Error)
		}
	}
	if checked < 50 {
		t.Fatalf("only %d error messages checked", checked)
	}
}
