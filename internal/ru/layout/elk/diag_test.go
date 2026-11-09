//go:build elkdiag

package elk

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/class"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/er"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/flowchart"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/requirement"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/state"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/dagre"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

// TestDiagElkInput is a tool for the work on whole-diagram parity. It is
// not a test of this package, and it builds only with the tag elkdiag,
// because it links the diagram packages:
//
//	DIAG_FILTER=flowchart/ go test -tags elkdiag -run TestDiagElkInput -v ./internal/ru/layout/elk/
//
// For each corpus fixture whose name has DIAG_FILTER in it, it renders the
// fixture with the Go renderer and compares the graph that goes to ELK
// with the graph that upstream passed to elk.layout for the same fixture
// (testdata/fixtures.jsonl.gz), on the properties that ELK and the layout
// read. A difference means that the cause of a wrong SVG is before the
// layout: in the diagram db or in the measured sizes. With the same graph
// the cause is after it.
func TestDiagElkInput(t *testing.T) {
	filter := os.Getenv("DIAG_FILTER")
	if filter == "" {
		t.Skip("set DIAG_FILTER to a part of the fixture names")
	}
	calls := loadCalls(t)
	same, differ, noCall, renderErr := 0, 0, 0, 0
	defer func(orig func([]byte) ([]byte, error)) { elkLayout = orig }(elkLayout)
	engine := elkLayout
	for _, c := range calls {
		if c.isRandom() || !strings.Contains(c.fixture, filter) {
			continue
		}
		text, err := os.ReadFile(filepath.Join("../../../../testdata/corpus/mermaid-e2e", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		var captured string
		elkLayout = func(graphJSON []byte) ([]byte, error) {
			captured = string(graphJSON)
			return engine(graphJSON)
		}
		if _, err = render.Render(string(text), render.Options{}); err != nil {
			renderErr++
			t.Logf("%s: render error: %v", c.fixture, err)
			continue
		}
		if captured == "" {
			noCall++
			t.Logf("%s: no ELK layout", c.fixture)
			continue
		}
		if projectedJSON(t, captured) == projectedJSON(t, c.in) {
			same++
			continue
		}
		differ++
		var paths []string
		diffPaths(canonical(project(mustParseJSON(t, captured), "root")), canonical(project(mustParseJSON(t, c.in), "root")), "", &paths)
		t.Logf("%s: ELK input differs in %d places:\n  %s", c.fixture, len(paths), strings.Join(paths[:min(len(paths), 12)], "\n  "))
	}
	t.Logf("ELK input: %d same, %d differ, %d without an ELK layout, %d render errors", same, differ, noCall, renderErr)
}

func mustParseJSON(t *testing.T, text string) any {
	t.Helper()
	v, err := js.ParseJSON([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// diffPaths lists the places where two JSON values differ: the path, the
// value of the Go renderer and the value of upstream.
func diffPaths(got, want any, path string, out *[]string) {
	switch g := got.(type) {
	case []any:
		w, ok := want.([]any)
		if !ok || len(g) != len(w) {
			*out = append(*out, path+": "+short(got)+", upstream "+short(want))
			return
		}
		for i := range g {
			name := strconv.Itoa(i)
			if id, ok := prop(g[i], "id").(string); ok {
				name = id
			}
			diffPaths(g[i], w[i], path+"/"+name, out)
		}
	case *js.Obj[any]:
		w, ok := want.(*js.Obj[any])
		if !ok {
			*out = append(*out, path+": "+short(got)+", upstream "+short(want))
			return
		}
		keys := g.Keys()
		for _, k := range w.Keys() {
			if !g.Has(k) {
				keys = append(keys, k)
			}
		}
		for _, k := range keys {
			diffPaths(g.Get(k), w.Get(k), path+"/"+k, out)
		}
	default:
		if js.StringifyJSON(got) != js.StringifyJSON(want) || (got == nil) != (want == nil) {
			*out = append(*out, path+": "+short(got)+", upstream "+short(want))
		}
	}
}

func short(v any) string {
	if v == nil {
		return "undefined"
	}
	text := js.StringifyJSON(v)
	if len(text) > 60 {
		text = text[:60] + "..."
	}
	return text
}
