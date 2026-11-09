package journey

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

// goldenTask is one task of db.getTasks() in the golden file. The score is
// a number, or the name of a number that JSON does not have ("NaN").
type goldenTask struct {
	Section string   `json:"section"`
	Type    string   `json:"type"`
	People  []string `json:"people"`
	Task    string   `json:"task"`
	Score   any      `json:"score"`
}

// goldenDB is the content of the journey db of real mermaid.
type goldenDB struct {
	Sections       []string     `json:"sections"`
	Tasks          []goldenTask `json:"tasks"`
	Actors         []string     `json:"actors"`
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
// front matter and of the directives, the type detection, the init function
// of the diagram and the parser on the cleaned text.
func parseLikeMermaid(text string) (*DB, error) {
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)
	resolved, err := config.Resolve(text, site)
	if err != nil {
		return nil, err
	}
	if resolved.DiagramType != "journey" {
		return nil, &config.Error{Message: "detected type " + resolved.DiagramType}
	}
	ctx := &render.Context{Config: resolved.Config, ConfigState: resolved.State, Sanitizer: sanitize.New()}
	db := NewDB(ctx)
	initDiagram(ctx, db)
	if err := parse(ctx, db, render.EncodeEntities(resolved.Code)+"\n"); err != nil {
		return nil, err
	}
	return db, nil
}

// score converts a score to the form of the golden file.
func score(v float64) any {
	if v != v || v-v != 0 {
		return js.Str(v)
	}
	return v
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
			Sections:       append([]string{}, db.GetSections()...),
			Tasks:          []goldenTask{},
			Title:          db.GetDiagramTitle(),
			AccTitle:       db.GetAccTitle(),
			AccDescription: db.GetAccDescription(),
		}
		for _, task := range db.GetTasks() {
			got.Tasks = append(got.Tasks, goldenTask{
				Section: task.Section,
				Type:    task.Type,
				People:  append([]string{}, task.People...),
				Task:    task.Task,
				Score:   score(task.Score),
			})
		}
		got.Actors = append([]string{}, db.GetActors()...)
		if !reflect.DeepEqual(got, *c.DB) {
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(*c.DB)
			t.Errorf("%s: db differs\n got  %s\n want %s\n%s", c.Name, gotJSON, wantJSON, c.Text)
		}
	}
	if parsed < 150 {
		t.Fatalf("only %d cases parse", parsed)
	}
}

// TestErrorMessagesAgainstMermaid compares the messages of the errors that
// come from the parser.
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
