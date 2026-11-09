// This file ports no upstream file.

// Command sitegen builds the website of the project and the images of the
// README.
//
//	go run ./tools/sitegen -out dist                 # the website
//	go run ./tools/sitegen -readme docs/assets       # the images and the numbers of the README
//
// The website is static. Its sources are in the directory website: the page
// template and the style sheet and scripts in website/static, and the
// diagrams of the gallery in website/diagrams. The generator adds what comes
// from the repository:
//
//   - The diagrams of the gallery, which this module renders.
//   - The result of the corpus comparison. The generator renders every
//     diagram of the three corpora and compares the hashes with the
//     manifests in testdata/reference, as the test TestCorpusParity does.
//   - The numbers of docs/performance.json, which
//     tools/reference/bench.mjs writes.
//   - The web fonts of internal/webfonts, which the diagrams use.
//   - The renderer as a WebAssembly module, for the playground.
//
// Run it from the root of the repository.
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/perfdata"
	"github.com/bvolpato/mermaid-go-renderer/internal/webfonts"
)

func main() {
	out := flag.String("out", "", "directory to write the website to")
	readme := flag.String("readme", "", "directory to write the images of the README to")
	wasm := flag.Bool("wasm", true, "build the WebAssembly module of the playground")
	flag.Parse()
	if *out == "" && *readme == "" {
		fmt.Fprintln(os.Stderr, "sitegen: give -out or -readme")
		os.Exit(2)
	}
	if err := run(*out, *readme, *wasm); err != nil {
		fmt.Fprintln(os.Stderr, "sitegen:", err)
		os.Exit(1)
	}
}

func run(out, readme string, wasm bool) error {
	perf, err := perfdata.Load(filepath.Join("docs", "performance.json"))
	if err != nil {
		return err
	}
	if perf.CLI == nil || len(perf.CLI.Rows) == 0 {
		return errors.New("docs/performance.json has no comparison of the command-line tools")
	}
	gallery, err := loadGallery(perf)
	if err != nil {
		return err
	}
	par, err := loadParity()
	if err != nil {
		return err
	}
	if readme != "" {
		if err := writeReadmeImages(readme, gallery, perf, par); err != nil {
			return err
		}
		if err := writeGeneratedText(perf); err != nil {
			return err
		}
	}
	if out == "" {
		return nil
	}
	return writeSite(out, gallery, perf, par, wasm)
}

// ---- The gallery ----

// galleryItem is one diagram of website/diagrams.
type galleryItem struct {
	File   string `json:"file"`
	Title  string `json:"title"`
	Note   string `json:"note"`
	Source string `json:"-"`
	// SVG is the markup for the page: what Render returns for the page of
	// mermaid-cli, with an id of its own.
	SVG string `json:"-"`
}

// galleryData is the data of a diagram of the gallery that shows numbers of
// the repository. Such a diagram is a template (text/template).
type galleryData struct {
	// WarmMmdg is the median time of a warm render of mmdg in ms.
	WarmMmdg string
	// CorpusTypes has the number of diagrams of the largest diagram types
	// of the end-to-end corpus, and of the others together.
	CorpusTypes []namedCount
	// Chart has the median render times of some diagram types, as the
	// lists of an xychart.
	Chart struct{ Labels, Mermaid, Mmdg, Max string }
}

type namedCount struct {
	Name  string
	Count int
}

// typeLabels has the names of diagram types for a reader.
var typeLabels = map[string]string{
	"flowchart-v2": "Flowchart", "classDiagram": "Class", "er": "ER", "sequence": "Sequence",
	"requirement": "Requirement", "stateDiagram": "State", "gitGraph": "Git graph", "gantt": "Gantt", "pie": "Pie",
}

func loadGalleryData(perf *perfdata.Data) (*galleryData, error) {
	d := &galleryData{WarmMmdg: fmt.Sprintf("%.1f", perf.WarmMedian.Mmdg)}
	m, err := parity.LoadManifest(filepath.Join("testdata", "reference", "linux.json"))
	if err != nil {
		return nil, err
	}
	count := map[string]int{}
	for _, entry := range m.Fixtures {
		if entry.Type != "" {
			count[entry.Type]++
		}
	}
	var all []namedCount
	for t, n := range count {
		all = append(all, namedCount{t, n})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Count != all[j].Count {
			return all[i].Count > all[j].Count
		}
		return all[i].Name < all[j].Name
	})
	other := 0
	for i, c := range all {
		if i >= 6 {
			other += c.Count
			continue
		}
		if label, ok := typeLabels[c.Name]; ok {
			c.Name = label
		}
		d.CorpusTypes = append(d.CorpusTypes, c)
	}
	d.CorpusTypes = append(d.CorpusTypes, namedCount{"Other", other})

	byType := map[string]perfdata.Type{}
	for _, t := range perf.Types {
		byType[t.Type] = t
	}
	var labels, mermaidMs, mmdgMs []string
	top := 0.0
	for _, id := range []string{"flowchart-v2", "classDiagram", "er", "sequence", "stateDiagram", "gitGraph", "gantt", "pie"} {
		t, ok := byType[id]
		if !ok {
			continue
		}
		labels = append(labels, strings.ToLower(strings.ReplaceAll(typeLabels[id], " ", "")))
		mermaidMs = append(mermaidMs, fmt.Sprintf("%.1f", t.Mermaid))
		mmdgMs = append(mmdgMs, fmt.Sprintf("%.2f", t.Mmdg))
		top = max(top, t.Mermaid)
	}
	d.Chart.Labels, d.Chart.Mermaid, d.Chart.Mmdg = strings.Join(labels, ", "), strings.Join(mermaidMs, ", "), strings.Join(mmdgMs, ", ")
	d.Chart.Max = fmt.Sprintf("%.0f", math.Ceil(top*1.05))
	return d, nil
}

func loadGallery(perf *perfdata.Data) ([]galleryItem, error) {
	data, err := loadGalleryData(perf)
	if err != nil {
		return nil, err
	}
	gallery, err := loadGalleryWith(data)
	if err != nil {
		return nil, err
	}
	return gallery, nil
}

func loadGalleryWith(numbers *galleryData) ([]galleryItem, error) {
	dir := filepath.Join("website", "diagrams")
	data, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return nil, err
	}
	var items []galleryItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	// The diagram of the section "How it works" is not in the list.
	items = append(items, galleryItem{File: "pipeline", Title: "Pipeline"})
	for i := range items {
		src, err := os.ReadFile(filepath.Join(dir, items[i].File+".mmd"))
		if err != nil {
			return nil, err
		}
		// A diagram with numbers of the repository is a template.
		// The actions are between {% and %}, because {{ and }} are a
		// node shape of mermaid.
		tmpl, err := template.New(items[i].File).Delims("{%", "%}").Parse(string(src))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", items[i].File, err)
		}
		var text bytes.Buffer
		if err := tmpl.Execute(&text, numbers); err != nil {
			return nil, fmt.Errorf("%s: %w", items[i].File, err)
		}
		items[i].Source = strings.TrimRight(text.String(), "\n")
		svg, err := mermaid.Render(items[i].Source, mermaid.WithPage(mermaid.PageMermaidCLI), mermaid.WithID("d-"+items[i].File))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", items[i].File, err)
		}
		items[i].SVG = svg
	}
	return items, nil
}

// ---- The corpus comparison ----

// The status of a fixture, as one letter in the data of the page.
const (
	statusExact     = 'e' // the same bytes as the reference
	statusSameError = 's' // the reference fails, and this module fails too
	statusOther     = 'x' // every other result
)

// corpusResult is the comparison of one corpus with one manifest.
type corpusResult struct {
	Corpus    string `json:"corpus"`
	Label     string `json:"label"`
	Total     int    `json:"total"`
	Exact     int    `json:"exact"`
	SameError int    `json:"sameError"`
	Other     int    `json:"other"`
	// Status has one letter for each fixture, in the order of the names of
	// the corpus.
	Status string `json:"status"`
	// Reasons says why a fixture with the status x does not pass, by its
	// index.
	Reasons map[string]string `json:"reasons,omitempty"`
}

func (c corpusResult) Pass() int { return c.Exact + c.SameError }

// paritySet is the comparison with one kind of reference.
type paritySet struct {
	ID      string         `json:"id"`
	Label   string         `json:"label"`
	Note    string         `json:"note"`
	Corpora []corpusResult `json:"corpora"`
}

// corpusNames has the names and the types of the fixtures of one corpus.
type corpusNames struct {
	Names []string `json:"names"`
	Types []string `json:"types"`
}

type parityData struct {
	Mermaid     string                 `json:"mermaid"`
	MermaidCLI  string                 `json:"mermaidCLI"`
	Environment string                 `json:"environment"`
	Sets        []paritySet            `json:"sets"`
	Fixtures    map[string]corpusNames `json:"fixtures"`
}

var corpusLabels = map[string]string{
	"mermaid-e2e":   "The end-to-end tests of mermaid",
	"mermaid-docs":  "The examples of the mermaid documentation",
	"mermaid-large": "Generated large diagrams",
}

func loadParity() (*parityData, error) {
	data := &parityData{Fixtures: map[string]corpusNames{}}
	sets := []struct {
		id, label, note string
		files           []string
	}{
		{"plain", "mermaid.render()", "The string that mermaid.render() returns in a plain Chromium page.",
			[]string{"linux.json", "linux-docs.json", "linux-large.json"}},
		{"mmdc", "mmdc 12", "Both files that the mmdc command writes: with the embedded fonts and without.",
			[]string{"linux-mmdc.json", "linux-mmdc-docs.json", "linux-mmdc-large.json"}},
	}
	for _, s := range sets {
		set := paritySet{ID: s.id, Label: s.label, Note: s.note}
		for _, file := range s.files {
			m, err := parity.LoadManifest(filepath.Join("testdata", "reference", file))
			if err != nil {
				return nil, err
			}
			data.Mermaid, data.Environment = m.Mermaid, m.Environment
			if m.MermaidCLI != "" {
				data.MermaidCLI = m.MermaidCLI
			}
			names := m.Names()
			if _, ok := data.Fixtures[m.Corpus]; !ok {
				types := make([]string, len(names))
				for i, name := range names {
					types[i] = m.Fixtures[name].Type
				}
				data.Fixtures[m.Corpus] = corpusNames{names, types}
			}
			set.Corpora = append(set.Corpora, compare(m, names))
		}
		data.Sets = append(data.Sets, set)
	}
	return data, nil
}

// compare renders every fixture of a manifest and compares it with the
// reference, as the test TestCorpusParity does.
func compare(m *parity.Manifest, names []string) corpusResult {
	res := corpusResult{Corpus: m.Corpus, Label: corpusLabels[m.Corpus], Total: len(names), Reasons: map[string]string{}}
	status := make([]byte, len(names))
	reasons := make([]string, len(names))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, name := range names {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			status[i], reasons[i] = fixtureStatus(m, name)
		}()
	}
	wg.Wait()
	for i, st := range status {
		switch st {
		case statusExact:
			res.Exact++
		case statusSameError:
			res.SameError++
		default:
			res.Other++
			res.Reasons[strconv.Itoa(i)] = reasons[i]
		}
	}
	res.Status = string(status)
	return res
}

func fixtureStatus(m *parity.Manifest, name string) (byte, string) {
	ref := m.Fixtures[name]
	src, err := os.ReadFile(filepath.Join("testdata", "corpus", m.Corpus, filepath.FromSlash(name)+".mmd"))
	if err != nil {
		return statusOther, "the fixture is missing"
	}
	opts := []mermaid.Option{mermaid.WithImageSize(parity.CorpusImageSize)}
	page := m.Page == parity.PageMermaidCLI
	if page {
		opts = append(opts, mermaid.WithPage(mermaid.PageMermaidCLI))
	}
	svg, err := mermaid.Render(string(src), opts...)
	switch {
	case ref.Error && err != nil:
		return statusSameError, ""
	case err != nil:
		return statusOther, notSupported(string(src), err)
	case ref.Error:
		return statusOther, "the reference fails, and mmdg renders"
	}
	if !page {
		if parity.HashSVG(svg) == ref.SHA256 {
			return statusExact, ""
		}
		return statusOther, "the output differs"
	}
	opts = append(opts, mermaid.WithBackgroundColor("white"))
	file, err := mermaid.StandaloneSVG(svg, opts...)
	if err != nil || parity.HashSVG(file) != ref.SHA256 {
		return statusOther, "the output differs"
	}
	want := ref.EmbedSHA256
	if want == "" {
		want = ref.SHA256
	}
	embedded, err := mermaid.StandaloneSVG(svg, append(opts, mermaid.WithFontEmbed(true))...)
	if err != nil || parity.HashSVG(embedded) != want {
		return statusOther, "the file with fonts differs"
	}
	return statusExact, ""
}

// notSupported names the feature that a fixture needs and this module has
// not.
func notSupported(src string, err error) string {
	var unsupported *mermaid.UnsupportedDiagramError
	switch {
	case strings.Contains(src, "$$"):
		return "KaTeX math is not supported"
	case errors.As(err, &unsupported), strings.Contains(src, "zenuml"):
		return "the ZenUML diagram is not supported"
	}
	return "mmdg fails: " + err.Error()
}

// ---- The website ----

type siteData struct {
	Gallery     []galleryItem
	Pipeline    galleryItem
	Perf        *perfdata.Data
	Parity      *parityData
	Plain       paritySet
	Mmdc        paritySet
	Warm        perfdata.Pair
	CLI         perfdata.CLIRow
	CLIRatio    string
	WasmMB      string
	Year        int
	Version     string
	ParityJSON  string
	PerfJSON    string
	GalleryJSON string
}

func writeSite(out string, gallery []galleryItem, perf *perfdata.Data, par *parityData, wasm bool) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	static := filepath.Join("website", "static")
	// The files of website/static, except the template of the page.
	err := filepath.WalkDir(static, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(path) == "index.html" {
			return err
		}
		rel, _ := filepath.Rel(static, path)
		return copyFile(path, filepath.Join(out, rel))
	})
	if err != nil {
		return err
	}
	if err := copyFile(filepath.Join("assets", "logo.svg"), filepath.Join(out, "logo.svg")); err != nil {
		return err
	}
	// The web fonts and their style sheets, with the layout of the
	// packages, so that the url() of a style sheet finds its font.
	err = fs.WalkDir(webfonts.FS, "files", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := webfonts.FS.ReadFile(path)
		if err != nil {
			return err
		}
		return writeFile(filepath.Join(out, "fonts", strings.TrimPrefix(path, "files/")), data)
	})
	if err != nil {
		return err
	}

	data := siteData{Perf: perf, Parity: par, Year: time.Now().Year(), Version: mermaid.MermaidVersion}
	for _, item := range gallery {
		if item.File == "pipeline" {
			data.Pipeline = item
		} else {
			data.Gallery = append(data.Gallery, item)
		}
	}
	data.Plain, data.Mmdc = par.Sets[0], par.Sets[1]
	data.Warm = perf.WarmMedian
	data.CLI = perf.CLI.Rows[0]
	data.CLIRatio = fmt.Sprintf("%.0f", data.CLI.Mmdc/data.CLI.Mmdg)

	if wasm {
		size, err := buildWasm(out)
		if err != nil {
			return err
		}
		data.WasmMB = fmt.Sprintf("%.0f", float64(size)/1e6)
	}

	sources := map[string]string{}
	for _, item := range data.Gallery {
		sources[item.File] = item.Source
	}
	for name, v := range map[string]any{"ParityJSON": par, "PerfJSON": perf, "GalleryJSON": sources} {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		// The text goes into a script element of the page.
		s := strings.ReplaceAll(string(b), "</", `<\/`)
		switch name {
		case "ParityJSON":
			data.ParityJSON = s
		case "PerfJSON":
			data.PerfJSON = s
		default:
			data.GalleryJSON = s
		}
	}

	tmplText, err := os.ReadFile(filepath.Join(static, "index.html"))
	if err != nil {
		return err
	}
	tmpl, err := template.New("index").Funcs(template.FuncMap{
		"ms": func(v float64) string {
			if v >= 100 {
				return fmt.Sprintf("%.0f", v)
			}
			if v >= 10 {
				return fmt.Sprintf("%.1f", v)
			}
			return fmt.Sprintf("%.2f", v)
		},
		"round":   func(v float64) string { return fmt.Sprintf("%.0f", v) },
		"escape":  template.HTMLEscapeString,
		"percent": func(a, b int) string { return fmt.Sprintf("%.1f", 100*float64(a)/float64(b)) },
	}).Parse(string(tmplText))
	if err != nil {
		return err
	}
	var page bytes.Buffer
	if err := tmpl.Execute(&page, data); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(out, "index.html"), page.Bytes()); err != nil {
		return err
	}
	// GitHub Pages must not run Jekyll on the files.
	return writeFile(filepath.Join(out, ".nojekyll"), nil)
}

// buildWasm builds cmd/wasm and writes it compressed, with the JavaScript
// glue of the Go release. A browser cannot ask GitHub Pages for a
// compressed .wasm file, so the page reads the gzip file and expands it.
func buildWasm(out string) (int, error) {
	dir := filepath.Join(out, "play")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	tmp, err := os.MkdirTemp("", "mmdg-wasm-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tmp)
	bin := filepath.Join(tmp, "mmdg.wasm")
	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", bin, "./cmd/wasm")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("go build ./cmd/wasm: %w", err)
	}
	raw, err := os.ReadFile(bin)
	if err != nil {
		return 0, err
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if _, err := zw.Write(raw); err != nil {
		return 0, err
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}
	if err := writeFile(filepath.Join(dir, "mmdg.wasm.gz"), buf.Bytes()); err != nil {
		return 0, err
	}
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return 0, err
	}
	glue := filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js")
	return buf.Len(), copyFile(glue, filepath.Join(dir, "wasm_exec.js"))
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return writeFile(to, data)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
