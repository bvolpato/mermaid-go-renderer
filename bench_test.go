package mermaid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// benchFixtures has one representative fixture of the corpus per diagram
// family.
var benchFixtures = []string{
	"pie/nightly-check/default-theme.mmd",
	"sequence",
	"gantt",
	"git",
	"c4",
	"timeline",
	"journey",
	"xychart",
	"sankey",
	"treemap",
	"quadrant-chart",
	"flowchart",
	"class-diagram",
	"state-diagram-v2",
	"er-diagram",
	"mindmap",
}

// firstFixture returns the first fixture (in name order) under a corpus
// directory, or the file itself.
func firstFixture(tb testing.TB, rel string) (string, string) {
	root := filepath.Join("testdata", "corpus", "mermaid-e2e")
	path := filepath.Join(root, rel)
	if strings.HasSuffix(rel, ".mmd") {
		src, err := os.ReadFile(path)
		if err != nil {
			tb.Fatal(err)
		}
		return rel, string(src)
	}
	var found string
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err == nil && found == "" && !d.IsDir() && strings.HasSuffix(p, ".mmd") {
			found = p
		}
		return nil
	})
	if found == "" {
		tb.Skipf("no fixture under %s", rel)
	}
	src, err := os.ReadFile(found)
	if err != nil {
		tb.Fatal(err)
	}
	return strings.TrimPrefix(found, root+string(filepath.Separator)), string(src)
}

// BenchmarkRender renders one fixture per diagram family with the default
// options. Next to the wall time it reports cpu-ns/op: the CPU time of the
// process per render, with the garbage collector. On a machine that runs
// other work, the CPU time and the allocations are the stable numbers.
func BenchmarkRender(b *testing.B) {
	for _, rel := range benchFixtures {
		name, src := firstFixture(b, rel)
		family := strings.SplitN(name, string(filepath.Separator), 2)[0]
		b.Run(family, func(b *testing.B) {
			benchRender(b, name, func() error {
				_, err := Render(src)
				return err
			})
		})
	}
}

// benchRender runs one render function as a benchmark.
func benchRender(b *testing.B, name string, render func() error) {
	if err := render(); err != nil {
		b.Skipf("%s: %v", name, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	cpu := processCPUTime()
	for i := 0; i < b.N; i++ {
		if err := render(); err != nil {
			b.Fatal(err)
		}
	}
	if used := processCPUTime() - cpu; used > 0 {
		b.ReportMetric(float64(used.Nanoseconds())/float64(b.N), "cpu-ns/op")
	}
}

// benchFontFixtures are the fixtures of the benchmarks that select another
// font library or another output.
var benchFontFixtures = []string{
	"pie/nightly-check/default-theme.mmd",
	"sequence",
	"flowchart",
}

// BenchmarkRenderSystemFonts renders with the fonts of this machine.
func BenchmarkRenderSystemFonts(b *testing.B) {
	for _, rel := range benchFontFixtures {
		name, src := firstFixture(b, rel)
		family := strings.SplitN(name, string(filepath.Separator), 2)[0]
		b.Run(family, func(b *testing.B) {
			benchRender(b, name, func() error {
				_, err := Render(src, WithSystemFonts())
				return err
			})
		})
	}
}

// BenchmarkSVGToPNG paints an SVG that Render returned.
func BenchmarkSVGToPNG(b *testing.B) {
	for _, rel := range benchFontFixtures {
		name, src := firstFixture(b, rel)
		family := strings.SplitN(name, string(filepath.Separator), 2)[0]
		b.Run(family, func(b *testing.B) {
			svg, err := Render(src)
			if err != nil {
				b.Skipf("%s: %v", name, err)
			}
			benchRender(b, name, func() error {
				_, err := SVGToPNG(svg)
				return err
			})
		})
	}
}
