// Command paritydoc renders every fixture of the corpus and writes the
// parity table of docs/PARITY.md.
//
// Usage:
//
//	go run ./tools/paritydoc -manifest testdata/reference/macos.json -o docs/PARITY.md
//
// It compares the SHA-256 of each rendered SVG with the hash in a reference
// manifest (see tools/manifest), as TestCorpusParity does.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
)

type row struct {
	name                                            string
	n, exact, agree, different, unsupported, failed int
}

func main() {
	manifestPath := flag.String("manifest", "testdata/reference/macos.json", "reference manifest")
	corpusRoot := flag.String("corpus", "testdata/corpus", "directory that holds the corpus directories")
	title := flag.String("title", "Parity with mermaid", "title of the document")
	source := flag.String("source", "the end-to-end test suite of mermaid", "where the diagrams of the corpus come from")
	out := flag.String("o", "", "Markdown file to write (default: standard output)")
	flag.Parse()

	m, err := parity.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if m.Corpus == "" {
		m.Corpus = "mermaid-e2e"
	}
	corpusDir := filepath.Join(*corpusRoot, m.Corpus)
	corpus := &corpusDir
	names := m.Names()
	status := make([]string, len(names))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, name := range names {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			status[i] = render(*corpus, name, m.Fixtures[name], m.Fonts == "system")
		}()
	}
	wg.Wait()

	rows := map[string]*row{}
	total := &row{name: "Total"}
	for i, name := range names {
		entry := m.Fixtures[name]
		key := entry.Type
		if entry.Error {
			key = "(mermaid fails)"
		}
		r := rows[key]
		if r == nil {
			r = &row{name: key}
			rows[key] = r
		}
		for _, t := range []*row{r, total} {
			t.n++
			switch status[i] {
			case "exact":
				t.exact++
			case "agree-error":
				t.agree++
			case "different":
				t.different++
			case "unsupported":
				t.unsupported++
			default:
				t.failed++
			}
		}
	}
	list := make([]*row, 0, len(rows))
	for _, r := range rows {
		list = append(list, r)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].name < list[j].name
	})

	var b strings.Builder
	b.WriteString("# " + *title + "\n\n")
	b.WriteString("This file is generated. Do not edit it. The command `go run ./tools/paritydoc` writes it.\n\n")
	fmt.Fprintf(&b, "The corpus `%s` has %d diagrams from %s. Each diagram was rendered with mermaid %s in the reference environment `%s`, and the SHA-256 of the SVG that `mermaid.render()` returned is in `%s`. The table compares the output of this package with those hashes.\n\n", filepath.ToSlash(corpusDir), total.n, *source, m.Mermaid, m.Environment, filepath.ToSlash(*manifestPath))
	b.WriteString("- **Byte-exact**: the SVG text has the same bytes as the SVG of mermaid.\n")
	b.WriteString("- **Same error**: mermaid fails on the diagram, and this package fails too.\n")
	b.WriteString("- **Different**: the diagram renders, and the SVG text differs.\n")
	b.WriteString("- **Not supported**: the diagram type, or a feature that the diagram uses, is not ported.\n\n")
	pass := total.exact + total.agree
	fmt.Fprintf(&b, "Result: %d of %d diagrams pass (%.1f%%). %d are byte-exact and %d give the same error.\n\n", pass, total.n, 100*float64(pass)/float64(total.n), total.exact, total.agree)
	b.WriteString("| Diagram type | Diagrams | Byte-exact | Same error | Different | Not supported |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|\n")
	line := func(r *row, bold bool) {
		name := "`" + r.name + "`"
		if bold || strings.HasPrefix(r.name, "(") {
			name = r.name
		}
		if bold {
			name = "**" + name + "**"
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d |\n", name, r.n, r.exact, r.agree, r.different, r.unsupported+r.failed)
	}
	for _, r := range list {
		line(r, false)
	}
	line(total, true)
	b.WriteString("\nThe test `TestCorpusParity` makes the same comparison. It fails when a diagram that passed stops passing.\n")

	if *out == "" {
		fmt.Print(b.String())
		return
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d of %d pass, %d byte-exact\n", *out, pass, total.n, total.exact)
}

func render(corpus, name string, ref parity.ManifestEntry, systemFonts bool) string {
	src, err := os.ReadFile(filepath.Join(corpus, filepath.FromSlash(name)+".mmd"))
	if err != nil {
		return "missing"
	}
	opts := []mermaid.Option{mermaid.WithImageSize(parity.CorpusImageSize)}
	if systemFonts {
		opts = append(opts, mermaid.WithSystemFonts())
	}
	svg, err := mermaid.Render(string(src), opts...)
	var unsupported *mermaid.UnsupportedDiagramError
	switch {
	case ref.Error && err != nil:
		return "agree-error"
	case ref.Error:
		return "different"
	case errors.As(err, &unsupported):
		return "unsupported"
	case err != nil:
		if strings.Contains(err.Error(), "not supported") || strings.Contains(err.Error(), "not ported") {
			return "unsupported"
		}
		return "error"
	case parity.HashSVG(svg) == ref.SHA256:
		return "exact"
	}
	return "different"
}
