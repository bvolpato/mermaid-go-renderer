package sankey

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// renderCase is one case of testdata/render.json, which
// tools/reference/gen-sankey-treemap-render.mjs wrote with mermaid 12.2.0
// in Chromium: a text and the SVG that mermaid.render returns for it, or the
// error. All cases render in one page, and UidsBefore is the value of the
// id counter of the page (Uid.count) before the render.
type renderCase struct {
	Name       string `json:"name"`
	Text       string `json:"text"`
	SVG        string `json:"svg"`
	Error      string `json:"error"`
	UidsBefore int    `json:"uidsBefore"`
}

var reUID = regexp.MustCompile(`( id="(?:node|linearGradient)-|url\(#linearGradient-)(\d+)`)

// renumber gives the ids of a reference SVG the numbers that a page has
// for its first render: it subtracts the ids that earlier renders took.
func renumber(svg string, uidsBefore int) string {
	return reUID.ReplaceAllStringFunc(svg, func(m string) string {
		sub := reUID.FindStringSubmatch(m)
		n, _ := strconv.Atoi(sub[2])
		return sub[1] + strconv.Itoa(n-uidsBefore)
	})
}

// TestRenderAgainstMermaid renders texts with config options and inputs
// that the corpus does not have, and requires the SVG text of mermaid.
func TestRenderAgainstMermaid(t *testing.T) {
	data, err := os.ReadFile("testdata/render.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []renderCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 30 {
		t.Fatalf("only %d cases", len(cases))
	}
	// The SVG has the text widths of Chromium on the macOS reference
	// machine. A machine with other fonts compares only the errors.
	reference := fonttest.IsMacOSReference()
	same, rejected := 0, 0
	for _, c := range cases {
		result, err := render.Render(c.Text, render.Options{})
		if c.Error != "" {
			rejected++
			if err == nil {
				t.Errorf("%s: no error, mermaid fails with %q\n%s", c.Name, c.Error, c.Text)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: error %v, mermaid renders the text\n%s", c.Name, err, c.Text)
			continue
		}
		if !reference {
			continue
		}
		want := renumber(c.SVG, c.UidsBefore)
		if result.SVG != want {
			i := 0
			for i < len(want) && i < len(result.SVG) && want[i] == result.SVG[i] {
				i++
			}
			lo, hi := max(0, i-60), func(s string) int { return min(len(s), i+80) }
			t.Errorf("%s: the SVG differs at byte %d\n got  …%s…\n want …%s…\n%s", c.Name, i, result.SVG[lo:hi(result.SVG)], want[lo:hi(want)], c.Text)
			continue
		}
		same++
	}
	t.Logf("%d cases: %d with the same SVG, %d that mermaid and the port reject", len(cases), same, rejected)
	fonttest.RequireMacOSReference(t)
}
