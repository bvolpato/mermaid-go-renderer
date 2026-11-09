package mermaid

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

// TestCachesDoNotChangeRenders renders diagrams with many site configs,
// themes and directives. It first renders each one with all caches off, so
// that every value is computed for that render alone. Then it turns the
// caches on and renders all of them again, in mixed order, from many
// goroutines and for several rounds. Each result must be the result of the
// render without caches: a cache must not change a byte, and a render must
// not depend on the renders before it. With -race the test also finds a
// cached value that changes.
func TestCachesDoNotChangeRenders(t *testing.T) {
	families := benchFixtures
	if testing.Short() {
		families = []string{"pie/nightly-check/default-theme.mmd", "sequence", "flowchart", "class-diagram", "xychart"}
	}
	options := []struct {
		name string
		opts []Option
	}{
		{"default", nil},
		{"dark", []Option{WithTheme("dark")}},
		{"forest", []Option{WithTheme("forest")}},
		{"base and variables", []Option{WithConfigJSON([]byte(`{"theme":"base","themeVariables":{"primaryColor":"#ff8800","lineColor":"#0000ff"}}`))}},
		{"hand drawn", []Option{WithConfigJSON([]byte(`{"look":"handDrawn","fontFamily":"courier"}`))}},
		{"sections", []Option{WithConfigJSON([]byte(`{"flowchart":{"curve":"linear","htmlLabels":false},"sequence":{"mirrorActors":false,"showSequenceNumbers":true},"themeVariables":{"xyChart":{"titleColor":"#123456"}}}`))}},
		{"id", []Option{WithID("other-id"), WithTheme("neutral")}},
		// A config that is not kept as a template: an object below the
		// levels that a copy of a config makes new.
		{"deep", []Option{WithConfigJSON([]byte(`{"c4":{"flowchart":{"subGraphTitleMargin":{"top":3,"more":{"x":1}}}},"theme":"dark"}`))}},
		{"unknown theme", []Option{WithTheme("no-such-theme")}},
	}
	directives := []string{
		"",
		"---\nconfig:\n  theme: neutral\n---\n",
		"---\nconfig:\n  theme: base\n  themeVariables:\n    primaryColor: '#00aa00'\n  look: neo\n---\n",
		"%%{init: {\"theme\": \"dark\", \"themeVariables\": {\"fontSize\": \"20px\"}}}%%\n",
		"%%{init: {\"flowchart\": {\"curve\": \"basis\"}, \"sequence\": {\"wrap\": true}}}%%\n",
	}
	// Diagrams without front matter, so that each directive can go in
	// front of them.
	plain := []string{
		"flowchart TD\n  A[Start] --> B{Choice}\n  B -- yes --> C[Done]\n  B -- no --> A\n",
		"sequenceDiagram\n  Alice->>Bob: Hello\n  Bob-->>Alice: Hi\n",
		"pie title Pets\n  \"Dogs\" : 10\n  \"Cats\" : 5\n",
		"classDiagram\n  class Animal {\n    +int age\n  }\n  Animal <|-- Dog\n",
		"stateDiagram-v2\n  [*] --> Still\n  Still --> Moving\n  Moving --> [*]\n",
		"erDiagram\n  CUSTOMER ||--o{ ORDER : places\n",
		"gantt\n  title Plan\n  dateFormat YYYY-MM-DD\n  section One\n  Task :a1, 2024-01-01, 3d\n",
		"xychart-beta\n  x-axis [a, b, c]\n  bar [1, 2, 3]\n",
		"timeline\n  title History\n  2020 : first\n  2021 : second\n",
		"journey\n  title Day\n  section Morning\n    Wake up: 5: Me\n",
		"gitGraph\n  commit\n  branch develop\n  commit\n  checkout main\n  merge develop\n",
		"quadrantChart\n  x-axis Low --> High\n  y-axis Low --> High\n  Point A: [0.3, 0.6]\n",
		"mindmap\n  root\n    one\n    two\n",
	}
	if testing.Short() {
		plain = plain[:5]
	}
	type renderCase struct {
		name string
		src  string
		opts []Option
		want string
	}
	var cases []renderCase
	for _, rel := range families {
		// The fixtures of the corpus have their config in front matter.
		name, src := firstFixture(t, rel)
		for _, o := range options {
			cases = append(cases, renderCase{name: fmt.Sprintf("%s, %s", name, o.name), src: src, opts: o.opts})
		}
	}
	for n, src := range plain {
		for _, o := range options {
			for i, d := range directives {
				cases = append(cases, renderCase{name: fmt.Sprintf("diagram %d, %s, directive %d", n, o.name, i), src: d + src, opts: o.opts})
			}
		}
	}
	result := func(c *renderCase) string {
		svg, err := Render(c.src, c.opts...)
		if err != nil {
			return "error: " + err.Error()
		}
		return svg
	}

	memo.SetEnabled(false)
	defer memo.SetEnabled(true)
	rendered := 0
	for i := range cases {
		cases[i].want = result(&cases[i])
		if len(cases[i].want) > 7 && cases[i].want[:7] != "error: " {
			rendered++
		}
	}
	if rendered < len(cases)/2 {
		t.Fatalf("only %d of %d cases rendered", rendered, len(cases))
	}

	memo.SetEnabled(true)
	memo.Purge()
	const workers = 8
	rounds := 3
	if testing.Short() {
		rounds = 2
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := 0
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w)))
			for round := 0; round < rounds; round++ {
				for _, i := range rng.Perm(len(cases)) {
					// Each worker renders a part of the cases in each
					// round, in its own order.
					if (i+w+round)%workers >= 3 {
						continue
					}
					c := &cases[i]
					if got := result(c); got != c.want {
						mu.Lock()
						failed++
						if failed <= 5 {
							t.Errorf("%s: the render with caches differs from the render without caches (%d and %d bytes)", c.name, len(got), len(c.want))
						}
						mu.Unlock()
					}
				}
			}
		}(w)
	}
	wg.Wait()
	if failed > 0 {
		t.Errorf("%d renders differ", failed)
	}
	t.Logf("%d cases (%d render), %d workers, %d rounds", len(cases), rendered, workers, rounds)
}
