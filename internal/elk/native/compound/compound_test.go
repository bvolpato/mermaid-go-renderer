package compound_test

import (
	"path/filepath"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/compound"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/compound/compoundtest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

func TestCompoundGraphPreprocessor(t *testing.T) {
	nativetest.RunProcessor(t, "CompoundGraphPreprocessor", func() lgraph.Processor { return compound.NewCompoundGraphPreprocessor() })
}

func TestCompoundGraphPostprocessor(t *testing.T) {
	nativetest.RunProcessor(t, "CompoundGraphPostprocessor", func() lgraph.Processor { return compound.NewCompoundGraphPostprocessor() })
}

// TestHierarchyGoldens checks the compound graph processors against the
// extra goldens of tools/reference/gen-elk-hierarchy.mjs: graphs with ports
// on compound nodes, port constraints, merged hierarchy edges, edge labels
// and junction points on cross-hierarchy edges, which mermaid does not make.
func TestHierarchyGoldens(t *testing.T) {
	cases, err := compoundtest.LoadCases(filepath.Join("testdata", "compound-steps.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	compoundtest.RunCases(t, cases, map[string]compoundtest.CaseFactory{
		"CompoundGraphPreprocessor":  compoundtest.Factory(func() lgraph.Processor { return compound.NewCompoundGraphPreprocessor() }),
		"CompoundGraphPostprocessor": compoundtest.Factory(func() lgraph.Processor { return compound.NewCompoundGraphPostprocessor() }),
	})
}
