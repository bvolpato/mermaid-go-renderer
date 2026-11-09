package compoundtest_test

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/compound/compoundtest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

// TestHierarchyGraphsEndToEnd lays out each ELK graph of the extra goldens
// of tools/reference/gen-elk-hierarchy.mjs with the native driver and
// compares the result with the result of elkjs, byte for byte. A step test
// builds the graph of each step from a stored state, which does not keep
// the vectors that two elements share. This test runs the processors of the
// packages compound and hports on the graph of the real importer.
//
// A layout that falls back to elkjs is not a failure: the graphs use
// features that other processors do not port. The test fails for a result
// that differs, and when no graph is laid out at all.
//
// A case without a result is a graph for which elkjs fails in one of the
// processors of the two packages. The native driver must fall back for it.
func TestHierarchyGraphsEndToEnd(t *testing.T) {
	var cases []*nativetest.Case
	seen := map[string]bool{}
	for _, file := range []string{
		filepath.Join("..", "testdata", "compound-steps.json.gz"),
		filepath.Join("..", "..", "hports", "testdata", "hports-steps.json.gz"),
	} {
		list, err := compoundtest.LoadCases(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range list {
			if !seen[c.Name] {
				seen[c.Name] = true
				cases = append(cases, c)
			}
		}
	}

	equal, differ, failing := 0, 0, 0
	reasons := map[string]int{}
	for _, c := range cases {
		out, reason := native.LayoutReason(c.In)
		if len(c.Out) == 0 {
			failing++
			if reason == "" {
				t.Errorf("%s: elkjs fails for the graph, and the native driver gives the layout %s", c.Name, clip(out))
			}
			continue
		}
		switch {
		case reason != "":
			if i := strings.IndexByte(reason, '\n'); i >= 0 {
				reason = reason[:i]
			}
			reasons[reason]++
		case string(out) != string(c.Out):
			differ++
			if differ <= 5 {
				t.Errorf("%s: the layout differs from elkjs:\n   got  %s\n   want %s", c.Name, clip(out), clip(c.Out))
			}
		default:
			equal++
		}
	}

	fallbacks := 0
	keys := make([]string, 0, len(reasons))
	for reason, n := range reasons {
		fallbacks += n
		keys = append(keys, reason)
	}
	sort.Strings(keys)
	t.Logf("%d graphs: %d laid out and equal to elkjs, %d fall back, %d differ; %d more graphs fail in elkjs", len(cases)-failing, equal, fallbacks, differ, failing)
	for _, reason := range keys {
		t.Logf("%4d fall back: %s", reasons[reason], reason)
	}
	if differ > 0 {
		t.Errorf("%d of %d layouts differ from elkjs", differ, len(cases)-failing)
	}
	if equal == 0 {
		t.Errorf("no graph is laid out by the native driver")
	}
}

func clip(b []byte) string {
	if len(b) > 700 {
		return string(b[:700]) + "…"
	}
	return string(b)
}
