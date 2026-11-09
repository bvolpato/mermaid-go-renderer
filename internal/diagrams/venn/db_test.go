package venn

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

// goldenSubset is one element of db.getSubsetData() in the golden file. The
// size is the text of the number, because a size can be Infinity. The label
// is null when it is undefined.
type goldenSubset struct {
	Sets  []string `json:"sets"`
	Size  string   `json:"size"`
	Label *string  `json:"label"`
}

// goldenText is one element of db.getTextData().
type goldenText struct {
	Sets  []string `json:"sets"`
	ID    string   `json:"id"`
	Label *string  `json:"label"`
}

// goldenStyle is one element of db.getStyleData(). Styles has the entries
// of the styles object in the order of its keys.
type goldenStyle struct {
	Targets []string    `json:"targets"`
	Styles  [][2]string `json:"styles"`
}

// goldenDB is the content of the venn db of real mermaid.
type goldenDB struct {
	Subsets        []goldenSubset `json:"subsets"`
	TextNodes      []goldenText   `json:"textNodes"`
	Styles         []goldenStyle  `json:"styles"`
	CurrentSets    *[]string      `json:"currentSets"`
	IndentMode     bool           `json:"indentMode"`
	Title          string         `json:"title"`
	AccTitle       string         `json:"accTitle"`
	AccDescription string         `json:"accDescription"`
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
	if resolved.DiagramType != "venn" {
		return nil, &config.Error{Message: "detected type " + resolved.DiagramType}
	}
	ctx := &render.Context{Config: resolved.Config, ConfigState: resolved.State, Sanitizer: sanitize.New()}
	db := NewDB(ctx)
	if err := parseText(db, render.EncodeEntities(resolved.Code)+"\n"); err != nil {
		return nil, err
	}
	return db, nil
}

func optional(s string, has bool) *string {
	if !has {
		return nil
	}
	return &s
}

// dump is the content of db in the form of the golden file.
func dump(db *DB) goldenDB {
	got := goldenDB{
		Subsets:        []goldenSubset{},
		TextNodes:      []goldenText{},
		Styles:         []goldenStyle{},
		IndentMode:     db.GetIndentMode(),
		Title:          db.GetDiagramTitle(),
		AccTitle:       db.GetAccTitle(),
		AccDescription: db.GetAccDescription(),
	}
	for _, s := range db.GetSubsetData() {
		got.Subsets = append(got.Subsets, goldenSubset{Sets: s.Sets, Size: js.Str(s.Size), Label: optional(s.Label, s.HasLabel)})
	}
	for _, n := range db.GetTextData() {
		got.TextNodes = append(got.TextNodes, goldenText{Sets: n.Sets, ID: n.ID, Label: optional(n.Label, n.HasLabel)})
	}
	for _, s := range db.GetStyleData() {
		entries := [][2]string{}
		s.Styles.Each(func(k, v string) { entries = append(entries, [2]string{k, v}) })
		got.Styles = append(got.Styles, goldenStyle{Targets: s.Targets, Styles: entries})
	}
	if cs, ok := db.GetCurrentSets(); ok {
		got.CurrentSets = &cs
	}
	return got
}

// TestDBAgainstMermaid compares the db after a parse with the db of real
// mermaid in Chromium, for the corpus fixtures, mutations of them, random
// documents and hand-written inputs. Regenerate the golden file with
// node tools/reference/gen-venn-ishikawa-db.mjs.
func TestDBAgainstMermaid(t *testing.T) {
	cases := loadGolden(t)
	if len(cases) < 400 {
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
		got := dump(db)
		if !reflect.DeepEqual(got, *c.DB) {
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(c.DB)
			t.Errorf("%s: db differs\n got  %s\n want %s\n%s", c.Name, gotJSON, wantJSON, c.Text)
		}
	}
	if parsed < 150 {
		t.Fatalf("only %d cases parse", parsed)
	}
	t.Logf("%d cases: %d parse and have the db of mermaid, %d fail as in mermaid", len(cases), parsed, len(cases)-parsed)
}

// TestErrorMessagesAgainstMermaid compares the messages of the errors that
// come from the parser, from its semantic actions and from the db.
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
	if checked < 100 {
		t.Fatalf("only %d error messages checked", checked)
	}
	t.Logf("%d error messages are the messages of mermaid", checked)
}
