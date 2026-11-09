package edges_test

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// The golden files hold cases with the results of upstream mermaid in
// Chromium. tools/reference/gen-edges.mjs writes them:
//
//	node tools/reference/gen-edges.mjs all
var goldenFiles = []string{
	"pure",
	"edges_synthetic",
	"labels",
	"linejumps",
	"corpus_flowchart_elk",
	"corpus_linejumps",
	"corpus_flowchart_dagre",
	"corpus_other_elk",
	"corpus_other_dagre",
}

func TestGolden(t *testing.T) {
	for _, name := range goldenFiles {
		t.Run(name, func(t *testing.T) {
			rutest.RunEdgeGolden(t, "testdata/"+name+".json.gz")
		})
	}
}
