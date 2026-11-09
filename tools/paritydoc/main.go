// Command paritydoc renders every fixture of the corpus and writes the
// parity table of docs/PARITY.md.
//
// Usage:
//
//	go run ./tools/paritydoc -manifest testdata/reference/macos.json -o docs/PARITY.md
//
// It compares the SHA-256 of each rendered SVG with the hash in a reference
// manifest (see tools/manifest), as TestCorpusParity does.
//
// A manifest of the page of mermaid-cli (tools/reference/mmdc-oracle.mjs)
// has the hashes of the two .svg files that mermaid-cli writes. The
// command then renders in the page mode mermaid-cli and compares both
// files. -manifest takes several manifests, with commas between them: the
// document then has one section for each. docs/PARITY-MMDC.md is:
//
//	go run ./tools/paritydoc -title "Parity with mermaid-cli" -o docs/PARITY-MMDC.md -manifest testdata/reference/linux-mmdc.json,testdata/reference/linux-mmdc-docs.json,testdata/reference/linux-mmdc-large.json,testdata/reference/macos-mmdc.json,testdata/reference/macos-mmdc-docs.json,testdata/reference/macos-mmdc-large.json
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
	name string
	// exactSVG counts the diagrams of a mermaid-cli manifest whose file
	// without fonts is byte-exact and whose file with fonts is not.
	n, exact, exactSVG, agree, different, unsupported, failed int
}

// sources says where the diagrams of each corpus come from.
var sources = map[string]string{
	"mermaid-e2e":   "the end-to-end test suite of mermaid",
	"mermaid-docs":  "the examples of the mermaid documentation, a corpus that no part of this package was developed against",
	"mermaid-large": "a seeded generator (tools/reference/gen-large-corpus.mjs): flowcharts, class, state and ER diagrams with 15 to 150 nodes",
}

func main() {
	manifestPaths := flag.String("manifest", "testdata/reference/macos.json", "reference manifest, or several with commas between them")
	corpusRoot := flag.String("corpus", "testdata/corpus", "directory that holds the corpus directories")
	title := flag.String("title", "Parity with mermaid", "title of the document")
	source := flag.String("source", "", "where the diagrams of the corpus come from (default: known for the corpora of this repository)")
	out := flag.String("o", "", "Markdown file to write (default: standard output)")
	flag.Parse()

	paths := strings.Split(*manifestPaths, ",")
	var b strings.Builder
	b.WriteString("# " + *title + "\n\n")
	b.WriteString("This file is generated. Do not edit it. The command `go run ./tools/paritydoc` writes it.\n\n")
	var summary []string
	for _, path := range paths {
		m, err := parity.LoadManifest(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if m.Corpus == "" {
			m.Corpus = "mermaid-e2e"
		}
		from := *source
		if from == "" {
			from = sources[m.Corpus]
		}
		if from == "" {
			from = "the corpus " + m.Corpus
		}
		if len(paths) > 1 {
			fmt.Fprintf(&b, "## %s, corpus `%s`\n\n", m.Environment, m.Corpus)
		}
		summary = append(summary, section(&b, m, path, *corpusRoot, from))
		if len(paths) > 1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\nThe test `TestCorpusParity` makes the same comparison. It fails when a diagram that passed stops passing.\n")

	if *out == "" {
		fmt.Print(b.String())
		return
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, line := range summary {
		fmt.Printf("%s: %s\n", *out, line)
	}
}

// section renders the corpus of one manifest and writes its paragraph and
// its table. It returns one line with the result.
func section(b *strings.Builder, m *parity.Manifest, manifestPath, corpusRoot, source string) string {
	corpusDir := filepath.Join(corpusRoot, m.Corpus)
	page := m.Page == parity.PageMermaidCLI
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
			status[i] = render(corpusDir, name, m.Fixtures[name], m.Fonts == "system", page)
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
			case "exact-svg":
				t.exactSVG++
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

	pass := total.exact + total.agree
	if page {
		fmt.Fprintf(b, "The corpus `%s` has %d diagrams from %s. Each diagram was rendered with mermaid-cli %s and mermaid %s in the reference environment `%s`. `%s` has the SHA-256 of the two `.svg` files that mermaid-cli writes for it: the file without the fonts of its page (`--no-font-embed`) and the file with them (the default). The table compares the output of this package in the page mode `mermaid-cli` with those hashes.\n\n", filepath.ToSlash(corpusDir), total.n, source, m.MermaidCLI, m.Mermaid, m.Environment, filepath.ToSlash(manifestPath))
		b.WriteString("- **Byte-exact**: both files have the same bytes as the files of mermaid-cli.\n")
		b.WriteString("- **Without fonts only**: the file without fonts has the same bytes. The file with fonts differs.\n")
		b.WriteString("- **Same error**: mermaid-cli fails on the diagram, and this package fails too.\n")
		b.WriteString("- **Different**: the diagram renders, and the file without fonts differs.\n")
		b.WriteString("- **Not supported**: the diagram type, or a feature that the diagram uses, is not ported.\n\n")
		fmt.Fprintf(b, "Result: %d of %d diagrams pass (%.1f%%). %d are byte-exact and %d give the same error.\n\n", pass, total.n, 100*float64(pass)/float64(total.n), total.exact, total.agree)
		b.WriteString("| Diagram type | Diagrams | Byte-exact | Without fonts only | Same error | Different | Not supported |\n")
		b.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")
	} else {
		fmt.Fprintf(b, "The corpus `%s` has %d diagrams from %s. Each diagram was rendered with mermaid %s in the reference environment `%s`, and the SHA-256 of the SVG that `mermaid.render()` returned is in `%s`. The table compares the output of this package with those hashes.\n\n", filepath.ToSlash(corpusDir), total.n, source, m.Mermaid, m.Environment, filepath.ToSlash(manifestPath))
		b.WriteString("- **Byte-exact**: the SVG text has the same bytes as the SVG of mermaid.\n")
		b.WriteString("- **Same error**: mermaid fails on the diagram, and this package fails too.\n")
		b.WriteString("- **Different**: the diagram renders, and the SVG text differs.\n")
		b.WriteString("- **Not supported**: the diagram type, or a feature that the diagram uses, is not ported.\n\n")
		fmt.Fprintf(b, "Result: %d of %d diagrams pass (%.1f%%). %d are byte-exact and %d give the same error.\n\n", pass, total.n, 100*float64(pass)/float64(total.n), total.exact, total.agree)
		b.WriteString("| Diagram type | Diagrams | Byte-exact | Same error | Different | Not supported |\n")
		b.WriteString("|---|---:|---:|---:|---:|---:|\n")
	}
	line := func(r *row, bold bool) {
		name := "`" + r.name + "`"
		if bold || strings.HasPrefix(r.name, "(") {
			name = r.name
		}
		if bold {
			name = "**" + name + "**"
		}
		if page {
			fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %d | %d |\n", name, r.n, r.exact, r.exactSVG, r.agree, r.different, r.unsupported+r.failed)
			return
		}
		fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %d |\n", name, r.n, r.exact, r.agree, r.different, r.unsupported+r.failed)
	}
	for _, r := range list {
		line(r, false)
	}
	line(total, true)
	return fmt.Sprintf("%s: %d of %d pass, %d byte-exact", filepath.Base(manifestPath), pass, total.n, total.exact)
}

func render(corpus, name string, ref parity.ManifestEntry, systemFonts, page bool) string {
	src, err := os.ReadFile(filepath.Join(corpus, filepath.FromSlash(name)+".mmd"))
	if err != nil {
		return "missing"
	}
	opts := []mermaid.Option{mermaid.WithImageSize(parity.CorpusImageSize)}
	if systemFonts {
		opts = append(opts, mermaid.WithSystemFonts())
	}
	if page {
		opts = append(opts, mermaid.WithPage(mermaid.PageMermaidCLI))
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
		if strings.Contains(err.Error(), "not supported") || strings.Contains(err.Error(), "not ported") || strings.Contains(err.Error(), "UnknownDiagramError") {
			return "unsupported"
		}
		return "error"
	case !page && parity.HashSVG(svg) == ref.SHA256:
		return "exact"
	case !page:
		return "different"
	}
	// The two .svg files of mermaid-cli, which sets the background "white".
	opts = append(opts, mermaid.WithBackgroundColor("white"))
	file, err := mermaid.StandaloneSVG(svg, opts...)
	if err != nil || parity.HashSVG(file) != ref.SHA256 {
		return "different"
	}
	want := ref.EmbedSHA256
	if want == "" {
		want = ref.SHA256
	}
	embedded, err := mermaid.StandaloneSVG(svg, append(opts, mermaid.WithFontEmbed(true))...)
	if err != nil || parity.HashSVG(embedded) != want {
		return "exact-svg"
	}
	return "exact"
}
