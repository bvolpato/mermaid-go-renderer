package usecase

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// shapeGolden is testdata/shapes.json.gz: the cases of the shape probe
// (tools/reference/probes/shapes.js) for the shapes of this package, as
// tools/reference/gen-shape-cases-usecase.mjs records them from the real
// insertNode of mermaid in Chromium. The config and the style sheet of a
// case are indexes into the lists of the file.
type shapeGolden struct {
	Configs []json.RawMessage `json:"configs"`
	CSS     []string          `json:"css"`
	Cases   []struct {
		rutest.ShapeCase
		Config *int `json:"config"`
		CSS    *int `json:"css"`
	} `json:"cases"`
}

func loadShapeGolden(t *testing.T) []rutest.ShapeCase {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "shapes.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var g shapeGolden
	if err := json.NewDecoder(zr).Decode(&g); err != nil {
		t.Fatal(err)
	}
	cases := make([]rutest.ShapeCase, len(g.Cases))
	for i, c := range g.Cases {
		cases[i] = c.ShapeCase
		if c.Config != nil {
			cases[i].Config = g.Configs[*c.Config]
		}
		if c.CSS != nil {
			cases[i].CSS = g.CSS[*c.CSS]
		}
	}
	return cases
}

// TestShapesAgainstMermaid replays the golden file of the shapes of this
// package: usecaseActor, usecaseActorHollow, usecaseActorAwesome,
// usecaseActorIcon, usecaseEllipse, usecaseBusiness and usecaseJsonTable.
// It compares the markup, the size of the node and the intersections with
// what the real insertNode of mermaid made in Chromium.
func TestShapesAgainstMermaid(t *testing.T) {
	cases := loadShapeGolden(t)
	counts := map[string]map[string]int{}
	skipped := 0
	for _, c := range cases {
		r := rutest.RunShapeCase(c)
		shape, _, _ := strings.Cut(c.Name, "/")
		if counts[shape] == nil {
			counts[shape] = map[string]int{}
		}
		counts[shape][r.Status]++
		counts[shape]["cases"]++
		if rutest.SkipComparison(c.HTML) {
			// The case has text, and this machine does not have the fonts
			// of the reference machine: only an error fails the test.
			skipped++
			if r.Status == "error" || r.Status == "not-ported" {
				t.Errorf("%s: %s\n    %s", r.Name, r.Status, r.Detail)
			}
			continue
		}
		if r.Status != "exact" && r.Status != "equivalent" {
			t.Errorf("%s: %s\n    %s", r.Name, r.Status, r.Detail)
		}
	}
	shapes := make([]string, 0, len(counts))
	for shape := range counts {
		shapes = append(shapes, shape)
	}
	sort.Strings(shapes)
	for _, shape := range shapes {
		n := counts[shape]
		t.Logf("%s: %d cases: exact=%d equivalent=%d different=%d error=%d not-ported=%d",
			shape, n["cases"], n["exact"], n["equivalent"], n["different"], n["error"], n["not-ported"])
	}
	rutest.RequireFonts(t, skipped)
}
