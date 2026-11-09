package set3

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

var reIconID = regexp.MustCompile(`IconifyId[0-9a-f]{17}([0-9]+)`)

// registeredSetup registers the icon packs of testdata/registered in the
// context of a case, as the probe registers them in the page with
// mermaid.registerIconPacks.
//
// The page numbers the ids of all icons with one counter, which goes on
// from case to case. Each case here has its own context, so the set-up
// starts the counter at the first id of the expected markup.
func registeredSetup(t *testing.T) rutest.ShapeSetup {
	t.Helper()
	data, err := os.ReadFile("testdata/registered/icon-packs.json")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := js.ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var packs []ru.IconLoader
	for _, p := range parsed.([]any) {
		pack := js.AsObj(p)
		packs = append(packs, ru.IconLoader{Name: js.ToString(pack.Get("name")), Icons: js.AsObj(pack.Get("icons"))})
	}
	return func(ctx *render.Context, c rutest.ShapeCase) error {
		if m := reIconID.FindStringSubmatch(c.HTML); m != nil {
			first, err := strconv.Atoi(m[1])
			if err != nil {
				return err
			}
			ru.SkipIconIDs(ctx, first)
		}
		return ru.RegisterIconPacks(ctx, packs...)
	}
}

// TestRegisteredIconShapes replays the icon shape cases that have
// registered icon packs.
func TestRegisteredIconShapes(t *testing.T) {
	rutest.RunShapeGoldenSetup(t, "testdata/registered/icons.json.gz", registeredSetup(t))
}

var reLabelSpan = regexp.MustCompile(`<span class="nodeLabel[^"]*">.*?</span>`)

// TestRegisteredLabelIcons replays cases with a registered icon in the
// text of an HTML label: createText puts the svg element of the icon in
// the label. The content of the label must be the same as upstream.
//
// The sizes are a known gap, and not a failure of this test: the layout
// engine (internal/browser) does not give an svg element in an HTML label
// its size yet, so the label is too narrow. The test logs how many cases
// are the same in full.
func TestRegisteredLabelIcons(t *testing.T) {
	setup := registeredSetup(t)
	cases := rutest.LoadShapeCases(t, "testdata/registered/label-icons.json.gz")
	counts := map[string]int{}
	for _, c := range cases {
		r, got := rutest.RunShapeCaseSetup(c, setup)
		counts[r.Status]++
		if r.Status == "error" {
			t.Errorf("%s: %s", c.Name, r.Detail)
			continue
		}
		want := reLabelSpan.FindString(c.HTML)
		if want == "" {
			t.Errorf("%s: no label in the expected markup", c.Name)
		} else if label := reLabelSpan.FindString(got); label != want {
			t.Errorf("%s: label\n got  %s\n want %s", c.Name, label, want)
		}
	}
	t.Logf("%d cases: label content the same in all; in full: exact=%d equivalent=%d different=%d (the sizes: a known gap of the layout engine)",
		len(cases), counts["exact"], counts["equivalent"], counts["different"])
}
