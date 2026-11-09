// Package cli is the parity command: it renders the corpus with this
// renderer and compares each result with the reference output of upstream
// mermaid in Chromium.
//
// Usage:
//
//	parity [-filter text] [-type id] [-v] [-out dir] [-report file] [-ref dir] [-fonts system|embedded] [-page plain|mermaid-cli]
//
// With -page mermaid-cli the renderer reproduces the page of mermaid-cli,
// and the reference is the output of tools/reference/mmdc-oracle.mjs
// (tools/reference/render-mmdc-ref.mjs writes it to
// .parity/ref-mmdc/linux and .parity/ref-mmdc/macos):
//
//	parity -page mermaid-cli -fonts embedded -ref .parity/ref-mmdc/linux/mermaid-e2e
//
// With -fonts embedded the renderer measures text with the embedded
// Liberation fonts and the rules of Chromium on Linux. The reference for
// that is Chromium on Linux with only the Liberation fonts
// (tools/reference/render-linux-ref.mjs writes it to
// .parity/ref-linux/mermaid-e2e).
//
// The reference output comes from tools/reference/oracle.mjs. The program
// that calls Main decides which diagram types are linked in: cmd/parity
// links all of them, and a diagram package can have its own small main
// that links only itself.
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/mmdc"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

type refMeta struct {
	DiagramType string `json:"diagramType"`
	Error       string `json:"error"`
}

type outcome struct {
	Fixture  string  `json:"fixture"`
	Family   string  `json:"family"`
	Type     string  `json:"type,omitempty"`
	Status   string  `json:"status"`
	Detail   string  `json:"detail,omitempty"`
	MaxDelta float64 `json:"maxDelta,omitempty"`
	Micros   int64   `json:"micros,omitempty"`
}

// Statuses, from best to worst.
var statuses = []string{"exact", "equivalent", "agree-error", "different", "go-error", "unsupported", "panic", "ref-error"}

// Main runs the parity command with the arguments of the process.
func Main() {
	corpus := flag.String("corpus", "testdata/corpus/mermaid-e2e", "directory of .mmd fixtures")
	ref := flag.String("ref", ".parity/ref/mermaid-e2e", "directory of reference .svg and .json files")
	filter := flag.String("filter", "", "only fixtures whose path contains this text")
	typ := flag.String("type", "", "only fixtures of this diagram type id")
	out := flag.String("out", "", "write the rendered SVG files to this directory")
	report := flag.String("report", "", "write a JSON report to this file")
	verbose := flag.Bool("v", false, "print every fixture that is not exact")
	tol := flag.Float64("tol", 0.01, "numeric tolerance in px")
	pixels := flag.Bool("pixels", false, "for results that are not byte-exact, render both SVGs in Chromium and compare the pixels (needs node and -out)")
	limit := flag.Int("show", 15, "number of differences to print")
	fontsFlag := flag.String("fonts", "system", "font library: system (the fonts and the rules of this machine) or embedded (the Liberation fonts and the rules of Linux)")
	pageFlag := flag.String("page", "plain", "page that the render reproduces: plain (a page with only mermaid, the reference of oracle.mjs) or mermaid-cli (the page of mmdc, the reference of mmdc-oracle.mjs)")
	flag.Parse()

	var fonts *font.Library
	switch *fontsFlag {
	case "system":
	case "embedded":
		fonts = font.Embedded()
	default:
		fmt.Fprintln(os.Stderr, "-fonts must be system or embedded")
		os.Exit(2)
	}
	page := render.PagePlain
	switch *pageFlag {
	case "plain":
	case "mermaid-cli":
		page = render.PageMermaidCLI
		if fonts == nil {
			fonts = font.System()
		}
		// The web fonts of the page come before the fonts of the machine.
		fonts = fonts.WithWebFonts()
	default:
		fmt.Fprintln(os.Stderr, "-page must be plain or mermaid-cli")
		os.Exit(2)
	}

	var fixtures []string
	_ = filepath.WalkDir(*corpus, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".mmd") && strings.Contains(path, *filter) {
			fixtures = append(fixtures, path)
		}
		return nil
	})
	sort.Strings(fixtures)

	results := make([]outcome, len(fixtures))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, fixture := range fixtures {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, fixture string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = run(fixture, *corpus, *ref, *out, *typ, *tol, fonts, page)
		}(i, fixture)
	}
	wg.Wait()

	type tally map[string]int
	families := map[string]tally{}
	total := tally{}
	var shown []outcome
	for _, r := range results {
		if r.Status == "skip" {
			continue
		}
		if families[r.Family] == nil {
			families[r.Family] = tally{}
		}
		families[r.Family][r.Status]++
		families[r.Family]["n"]++
		total[r.Status]++
		total["n"]++
		if r.Status != "exact" && r.Status != "agree-error" {
			shown = append(shown, r)
		}
	}
	names := make([]string, 0, len(families))
	for name := range families {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Printf("%-22s %5s %6s %6s %6s %6s %6s %6s %6s\n", "family", "n", "exact", "equiv", "agree", "diff", "error", "unsup", "panic")
	row := func(name string, t tally) {
		fmt.Printf("%-22s %5d %6d %6d %6d %6d %6d %6d %6d\n", name, t["n"], t["exact"], t["equivalent"], t["agree-error"], t["different"], t["go-error"]+t["ref-error"], t["unsupported"], t["panic"])
	}
	for _, name := range names {
		row(name, families[name])
	}
	row("TOTAL", total)
	pass := total["exact"] + total["equivalent"] + total["agree-error"]
	if total["n"] > 0 {
		fmt.Printf("\nparity: %d / %d (%.1f%%), byte-exact: %d\n", pass, total["n"], 100*float64(pass)/float64(total["n"]), total["exact"])
	}

	if *pixels {
		if *out == "" {
			fmt.Fprintln(os.Stderr, "-pixels needs -out")
			os.Exit(2)
		}
		pixelProof(results, *ref, *out)
	}

	sort.SliceStable(shown, func(i, j int) bool { return shown[i].Status < shown[j].Status })
	n := 0
	for _, r := range shown {
		if r.Status == "unsupported" && !*verbose {
			continue
		}
		if !*verbose && n >= *limit {
			break
		}
		if r.Status == "equivalent" && !*verbose {
			continue
		}
		n++
		fmt.Printf("\n[%s] %s (%s)\n    %s\n", r.Status, r.Fixture, r.Type, r.Detail)
	}
	if *report != "" {
		data, _ := json.MarshalIndent(map[string]any{"total": total, "families": families, "results": results}, "", " ")
		_ = os.WriteFile(*report, data, 0o644)
	}
}

// pixelProof renders, in Chromium, the output of this renderer and the
// reference for every fixture that is equivalent but not byte-exact, and
// compares the two screenshots pixel by pixel. A byte-exact result needs no
// screenshot: identical markup gives identical pixels.
func pixelProof(results []outcome, ref, out string) {
	var gotFiles, wantFiles []string
	var names []string
	exact := 0
	for _, r := range results {
		base := strings.TrimSuffix(r.Fixture, ".mmd")
		switch r.Status {
		case "exact":
			exact++
		case "equivalent", "different":
			gotFiles = append(gotFiles, filepath.Join(out, base+".svg"))
			wantFiles = append(wantFiles, filepath.Join(ref, base+".svg"))
			names = append(names, r.Fixture+" ["+r.Status+"]")
		}
	}
	fmt.Printf("\npixel proof in Chromium: %d byte-exact (identical pixels by definition), %d to compare\n", exact, len(names))
	if len(names) == 0 {
		return
	}
	shoot := func(files []string, dir string) error {
		list := filepath.Join(os.TempDir(), fmt.Sprintf("mmdg-shoot-%d.list", os.Getpid()))
		defer os.Remove(list)
		if err := os.WriteFile(list, []byte(strings.Join(files, "\n")+"\n"), 0o644); err != nil {
			return err
		}
		cmd := exec.Command("node", "tools/reference/oracle.mjs", "shoot", "--quiet", "--list", list)
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if err := shoot(append(append([]string{}, gotFiles...), wantFiles...), ""); err != nil {
		fmt.Fprintln(os.Stderr, "pixel proof: the oracle failed:", err)
		return
	}
	identical := 0
	for i := range names {
		d, err := parity.ComparePNG(strings.TrimSuffix(gotFiles[i], ".svg")+".png", strings.TrimSuffix(wantFiles[i], ".svg")+".png")
		if err != nil {
			fmt.Printf("  %s: %v\n", names[i], err)
			continue
		}
		if d.Identical() {
			identical++
		} else {
			fmt.Printf("  %s: %s\n", names[i], d)
		}
	}
	fmt.Printf("pixel-identical in Chromium: %d of %d compared; with the byte-exact ones: %d of %d\n", identical, len(names), identical+exact, len(names)+exact)
}

func run(fixture, corpus, ref, out, typ string, tol float64, fonts *font.Library, page render.Page) (res outcome) {
	rel, _ := filepath.Rel(corpus, fixture)
	base := strings.TrimSuffix(rel, ".mmd")
	res = outcome{Fixture: rel, Family: strings.Split(rel, string(filepath.Separator))[0]}
	var meta refMeta
	if data, err := os.ReadFile(filepath.Join(ref, base+".json")); err == nil {
		_ = json.Unmarshal(data, &meta)
	} else {
		res.Status = "skip"
		return res
	}
	res.Type = meta.DiagramType
	if typ != "" && meta.DiagramType != typ {
		res.Status = "skip"
		return res
	}
	text, err := os.ReadFile(fixture)
	if err != nil {
		res.Status, res.Detail = "go-error", err.Error()
		return res
	}
	defer func() {
		if p := recover(); p != nil {
			buf := make([]byte, 2048)
			buf = buf[:runtime.Stack(buf, false)]
			res.Status, res.Detail = "panic", fmt.Sprintf("%v\n%s", p, buf)
		}
	}()
	start := time.Now()
	// The images that the reference renderer loaded from the network
	// have their natural sizes here: a render reads no network.
	got, err := render.Render(string(text), render.Options{Fonts: fonts, ImageSize: parity.CorpusImageSize, Page: page})
	if err == nil && page == render.PageMermaidCLI {
		// The reference is the file of mmdc --no-font-embed: the XML of
		// the svg element with the background that mmdc sets.
		var file *mmdc.Page
		if file, err = mmdc.Load(got.SVG, mmdc.Options{Background: "white", SetBackground: true}); err == nil {
			got.SVG = file.XML()
		}
	}
	res.Micros = time.Since(start).Microseconds()
	if meta.Error != "" {
		if err != nil {
			res.Status = "agree-error"
		} else {
			res.Status, res.Detail = "ref-error", "mermaid failed: "+meta.Error
		}
		return res
	}
	if err != nil {
		if _, ok := err.(*render.UnsupportedError); ok {
			res.Status, res.Detail = "unsupported", err.Error()
		} else {
			res.Status, res.Detail = "go-error", err.Error()
		}
		return res
	}
	if out != "" {
		path := filepath.Join(out, base+".svg")
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte(got.SVG), 0o644)
	}
	want, err := os.ReadFile(filepath.Join(ref, base+".svg"))
	if err != nil {
		res.Status, res.Detail = "go-error", err.Error()
		return res
	}
	verdict, diff := parity.CompareSVG(got.SVG, string(want), tol)
	res.Status = verdict.String()
	if diff != nil {
		res.MaxDelta = diff.MaxDelta
		if verdict == parity.Different {
			res.Detail = diff.String()
		}
	}
	return res
}
