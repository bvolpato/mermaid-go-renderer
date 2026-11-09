//go:build recorddocs

// This file renders diagrams with the whole renderer, so it is behind a
// build tag: the tests of this package then build without the packages of
// the diagrams.
//
//	go test -tags recorddocs ./internal/cytoscape -run TestRecordDocsInputs

package cytoscape_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape"
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native"
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/nativetest"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// docsCorpora are the corpora whose diagrams are not in the record of the
// layouts of Chromium (testdata/layouts.json.gz has the corpus mermaid-e2e).
var docsCorpora = []string{"mermaid-docs", "mermaid-large"}

// TestRecordDocsInputs renders the mindmap and architecture diagrams of the
// documentation corpora and writes what they give to cytoscape to
// native/testdata/docs-inputs.json.gz. The golden files of the native
// layouts have these inputs as cases. Run it on a machine with the fonts of
// the reference.
func TestRecordDocsInputs(t *testing.T) {
	var mu sync.Mutex
	var current string
	out := &nativetest.DocsInputs{}
	cytoscape.SetObserver(&cytoscape.Observer{
		CoseBilkent: func(env cytoscape.Env, nodes []cytoscape.Node, edges []cytoscape.Edge) cytoscape.Env {
			d := nativetest.DocsCoseBilkent{Name: current}
			for _, n := range nodes {
				d.Nodes = append(d.Nodes, nativetest.GoldenNode{ID: n.ID, W: nativetest.Num(n.Width), H: nativetest.Num(n.Height), X: nativetest.Num(n.X), Y: nativetest.Num(n.Y)})
			}
			for _, e := range edges {
				d.Edges = append(d.Edges, [3]string{e.ID, e.Start, e.End})
			}
			mu.Lock()
			out.CoseBilkent = append(out.CoseBilkent, d)
			mu.Unlock()
			return env
		},
		Architecture: func(env cytoscape.Env, in *cytoscape.ArchitectureInput) cytoscape.Env {
			mu.Lock()
			at := len(out.Architecture)
			out.Architecture = append(out.Architecture, nativetest.DocsArchitecture{Name: current, Input: nativeInput(in)})
			mu.Unlock()
			measure := env.MeasureText
			if measure != nil {
				env.MeasureText = func(font, text string) float64 {
					w := measure(font, text)
					mu.Lock()
					out.Architecture[at].Measure = append(out.Architecture[at].Measure, nativetest.Measure{Font: font, Text: text, Width: nativetest.Num(w)})
					mu.Unlock()
					return w
				}
			}
			return env
		},
	})
	defer cytoscape.SetObserver(nil)

	rendered := 0
	for _, corpus := range docsCorpora {
		root := filepath.Join("..", "..", "testdata", "corpus", corpus)
		var files []string
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".mmd") {
				files = append(files, path)
			}
			return nil
		})
		sort.Strings(files)
		for _, file := range files {
			text, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			// Only the diagrams that use cytoscape: the others cost time.
			if !strings.Contains(string(text), "mindmap") && !strings.Contains(string(text), "architecture") {
				continue
			}
			rel, _ := filepath.Rel(filepath.Join("..", "..", "testdata", "corpus"), file)
			current = strings.TrimSuffix(filepath.ToSlash(rel), ".mmd")
			before := len(out.CoseBilkent) + len(out.Architecture)
			if _, err := render.Render(string(text), render.Options{}); err != nil {
				t.Logf("%s: %v", current, err)
			}
			if len(out.CoseBilkent)+len(out.Architecture) > before {
				rendered++
			}
		}
	}
	if rendered == 0 {
		t.Fatal("no diagram used cytoscape")
	}
	file := filepath.Join("native", "testdata", "docs-inputs.json.gz")
	if err := nativetest.WriteGzipJSON(file, out); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d diagrams: %d cose-bilkent layouts and %d fcose layouts in %s", rendered, len(out.CoseBilkent), len(out.Architecture), file)
}

// nativeInput converts the input of Architecture to the type of the golden
// files.
func nativeInput(in *cytoscape.ArchitectureInput) native.ArchitectureInput {
	out := native.ArchitectureInput{
		Config:    native.ArchitectureConfig(in.Config),
		Alignment: native.AlignmentConstraint(in.Alignment),
	}
	for _, n := range in.Groups {
		out.Groups = append(out.Groups, native.ArchitectureNode(n))
	}
	for _, n := range in.Services {
		out.Services = append(out.Services, native.ArchitectureNode(n))
	}
	for _, n := range in.Junctions {
		out.Junctions = append(out.Junctions, native.ArchitectureNode(n))
	}
	for _, e := range in.Edges {
		out.Edges = append(out.Edges, native.ArchitectureEdge(e))
	}
	for _, r := range in.RelativePlacement {
		out.RelativePlacement = append(out.RelativePlacement, native.RelativePlacement(r))
	}
	return out
}
