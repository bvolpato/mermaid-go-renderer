// Command perfdoc measures the render time of every diagram of a corpus and
// compares it with the time that mermaid took for the same diagram in
// Chromium. It writes docs/PERFORMANCE.md.
//
// Usage:
//
//	cd tools/reference
//	node oracle.mjs bench --base ../../testdata/corpus/mermaid-e2e --list FILES --out warm.json
//	node oracle.mjs bench --base ../../testdata/corpus/mermaid-e2e --list FILES --out cold.json --fresh --repeat 1
//	cd ../..
//	go run ./tools/perfdoc -warm warm.json -cold cold.json -o docs/PERFORMANCE.md
//
// The comparison has two parts, and each part measures both sides in the
// same state.
//
//   - Warm. mermaid: the fastest of several mermaid.render() calls in a page
//     that stays open. This renderer: the fastest of several Render calls in
//     a process that stays running.
//   - First render. mermaid: the first mermaid.render() call of a new page
//     that has mermaid loaded, in a browser that is running. This renderer:
//     a new process that reads the file, renders it and exits.
//
// The -warm and -cold flags take file name patterns, so that a benchmark
// that ran in batches can be read from several files.
package main

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
	"time"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
)

// benchFile is what "oracle.mjs bench" writes.
type benchFile struct {
	Mermaid  string `json:"mermaid"`
	Chromium string `json:"chromium"`
	Mode     string `json:"mode"`
	Repeat   int    `json:"repeat"`
	Results  map[string]struct {
		DiagramType string    `json:"diagramType"`
		Times       []float64 `json:"times"`
		Error       string    `json:"error"`
	} `json:"results"`
}

type benchResult struct {
	typ   string
	times []float64
}

// readBench reads the files that match the pattern into one table. The key
// is the path of a fixture relative to the corpus.
func readBench(pattern string) (results map[string]benchResult, meta benchFile, err error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, meta, err
	}
	if len(files) == 0 {
		return nil, meta, fmt.Errorf("no file matches %s", pattern)
	}
	results = map[string]benchResult{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, meta, err
		}
		var b benchFile
		if err := json.Unmarshal(data, &b); err != nil {
			return nil, meta, fmt.Errorf("%s: %w", file, err)
		}
		meta.Mermaid, meta.Chromium, meta.Mode, meta.Repeat = b.Mermaid, b.Chromium, b.Mode, b.Repeat
		for rel, r := range b.Results {
			if r.Error == "" && len(r.Times) > 0 {
				results[rel] = benchResult{r.DiagramType, r.Times}
			}
		}
	}
	return results, meta, nil
}

type sample struct {
	typ string
	// Warm times in milliseconds.
	goWarm, mermaidWarm float64
	// First-render times in milliseconds. Zero when there is no value.
	goCold, mermaidCold float64
}

func fastest(v []float64) float64 {
	best := v[0]
	for _, x := range v[1:] {
		best = min(best, x)
	}
	return best
}

func quantile(v []float64, p float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[int(p*float64(len(s)-1)+0.5)]
}

func column(list []sample, get func(sample) float64) []float64 {
	out := make([]float64, 0, len(list))
	for _, s := range list {
		if v := get(s); v > 0 {
			out = append(out, v)
		}
	}
	return out
}

func renderOptions(systemFonts bool) []mermaid.Option {
	opts := []mermaid.Option{mermaid.WithImageSize(parity.CorpusImageSize)}
	if systemFonts {
		opts = append(opts, mermaid.WithSystemFonts())
	}
	return opts
}

func main() {
	corpus := flag.String("corpus", "testdata/corpus/mermaid-e2e", "directory of .mmd fixtures")
	warm := flag.String("warm", ".parity/perf/chunks/shared.*.json", "files of \"oracle.mjs bench\": mermaid in a page that stays open")
	cold := flag.String("cold", ".parity/perf/chunks/fresh.*.json", "files of \"oracle.mjs bench --fresh\": mermaid in a new page for each diagram")
	repeat := flag.Int("repeat", 7, "renders of each diagram; the fastest counts")
	systemFonts := flag.Bool("system-fonts", false, "measure with the system fonts")
	cli := flag.String("cli", "", "Markdown file with the comparison of the command-line tools, added as a section")
	out := flag.String("o", "", "Markdown file to write (default: standard output)")
	child := flag.String("child", "", "render this file one time and exit (the first-render measurement uses it)")
	flag.Parse()

	if *child != "" {
		src, err := os.ReadFile(*child)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if _, err := mermaid.Render(string(src), renderOptions(*systemFonts)...); err != nil {
			os.Exit(3)
		}
		return
	}

	warmResults, warmMeta, err := readBench(*warm)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	coldResults, _, err := readBench(*cold)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	opts := renderOptions(*systemFonts)

	rels := make([]string, 0, len(warmResults))
	for rel := range warmResults {
		rels = append(rels, rel)
	}
	sort.Strings(rels)

	// Warm: the fastest of several renders in this process.
	var samples []sample
	var files []string
	for _, rel := range rels {
		file := filepath.Join(*corpus, filepath.FromSlash(rel))
		src, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		times := make([]float64, 0, *repeat)
		for i := 0; i < *repeat; i++ {
			start := time.Now()
			if _, err := mermaid.Render(string(src), opts...); err != nil {
				times = nil
				break
			}
			times = append(times, float64(time.Since(start).Nanoseconds())/1e6)
		}
		if len(times) == 0 {
			continue
		}
		s := sample{typ: warmResults[rel].typ, goWarm: fastest(times), mermaidWarm: fastest(warmResults[rel].times)}
		if c, ok := coldResults[rel]; ok {
			s.mermaidCold = c.times[0]
		}
		samples = append(samples, s)
		files = append(files, file)
	}

	// First render: a new process for each diagram. The time is the wall
	// time of the process, with its start and its end.
	for i, file := range files {
		if samples[i].mermaidCold == 0 {
			continue
		}
		args := []string{"-child", file}
		if *systemFonts {
			args = append(args, "-system-fonts")
		}
		start := time.Now()
		if err := exec.Command(self, args...).Run(); err != nil {
			continue
		}
		samples[i].goCold = float64(time.Since(start).Nanoseconds()) / 1e6
	}

	byType := map[string][]sample{}
	for _, s := range samples {
		byType[s.typ] = append(byType[s.typ], s)
	}
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		if len(byType[types[i]]) != len(byType[types[j]]) {
			return len(byType[types[i]]) > len(byType[types[j]])
		}
		return types[i] < types[j]
	})

	goWarm := column(samples, func(s sample) float64 { return s.goWarm })
	mmWarm := column(samples, func(s sample) float64 { return s.mermaidWarm })
	var coldBoth []sample
	for _, s := range samples {
		if s.goCold > 0 && s.mermaidCold > 0 {
			coldBoth = append(coldBoth, s)
		}
	}
	goCold := column(coldBoth, func(s sample) float64 { return s.goCold })
	mmCold := column(coldBoth, func(s sample) float64 { return s.mermaidCold })
	sum := func(v []float64) float64 {
		t := 0.0
		for _, x := range v {
			t += x
		}
		return t
	}

	var b strings.Builder
	b.WriteString("# Performance\n\n")
	b.WriteString("This file is generated. Do not edit it. The command `go run ./tools/perfdoc` writes it.\n\n")
	fmt.Fprintf(&b, "The numbers compare `mmdg` with mermaid %s in Chromium %s, for the %d diagrams of the corpus `%s` that both sides render. Both ran on the same machine (%s/%s, %d CPUs), one diagram at a time. Times are in milliseconds.\n\n", warmMeta.Mermaid, warmMeta.Chromium, len(samples), filepath.ToSlash(*corpus), runtime.GOOS, runtime.GOARCH, runtime.NumCPU())

	b.WriteString("## Summary\n\n")
	b.WriteString("| Measurement | mermaid in Chromium | mmdg | Ratio |\n")
	b.WriteString("|---|---:|---:|---:|\n")
	line := func(name string, mm, g float64, format string) {
		fmt.Fprintf(&b, "| %s | "+format+" | "+format+" | %.0fx |\n", name, mm, g, mm/g)
	}
	line("Warm render, median", quantile(mmWarm, 0.5), quantile(goWarm, 0.5), "%.2f")
	line("Warm render, 90th percentile", quantile(mmWarm, 0.9), quantile(goWarm, 0.9), "%.2f")
	line("Warm render, all diagrams, in seconds", sum(mmWarm)/1000, sum(goWarm)/1000, "%.2f")
	if len(coldBoth) > 0 {
		line("First render, median", quantile(mmCold, 0.5), quantile(goCold, 0.5), "%.1f")
		line("First render, 90th percentile", quantile(mmCold, 0.9), quantile(goCold, 0.9), "%.1f")
	}
	b.WriteString("\n")
	b.WriteString("What each measurement is:\n\n")
	fmt.Fprintf(&b, "- **Warm render, mermaid**: the fastest of %d `mermaid.render()` calls for the diagram, in a page that stays open and renders many diagrams. V8 has compiled the code.\n", warmMeta.Repeat)
	fmt.Fprintf(&b, "- **Warm render, mmdg**: the fastest of %d `mermaid.Render` calls for the diagram, in a process that stays running.\n", *repeat)
	b.WriteString("- **First render, mermaid**: the first `mermaid.render()` call in a new page. The browser is running and the page has loaded mermaid. The start of the browser and the load of the script are not in the time.\n")
	b.WriteString("- **First render, mmdg**: a new process that reads the file, renders it one time and exits. The start and the end of the process are in the time.\n\n")
	b.WriteString("The first-render comparison favors mermaid, because it leaves out the start of the browser. A tool that starts a browser for each run, such as mermaid-cli, pays that start each time.\n\n")

	b.WriteString("## By diagram type\n\n")
	b.WriteString("| Diagram type | Diagrams | mermaid warm | mmdg warm | mmdg warm, 90th percentile | Warm ratio | mermaid first | mmdg first |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|\n")
	row := func(name string, list []sample) {
		g := column(list, func(s sample) float64 { return s.goWarm })
		m := column(list, func(s sample) float64 { return s.mermaidWarm })
		gm, mm := quantile(g, 0.5), quantile(m, 0.5)
		first := func(get func(sample) float64) string {
			v := column(list, get)
			if len(v) == 0 {
				return ""
			}
			return fmt.Sprintf("%.1f", quantile(v, 0.5))
		}
		fmt.Fprintf(&b, "| %s | %d | %.2f | %.2f | %.2f | %.0fx | %s | %s |\n", name, len(list), mm, gm, quantile(g, 0.9), mm/gm,
			first(func(s sample) float64 { return s.mermaidCold }), first(func(s sample) float64 { return s.goCold }))
	}
	for _, t := range types {
		row("`"+t+"`", byType[t])
	}
	row("**All**", samples)
	b.WriteString("\nThe columns have the median of the diagrams of the type, except where the heading says otherwise.\n")

	if *cli != "" {
		text, err := os.ReadFile(*cli)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(string(text)))
		b.WriteString("\n")
	}

	if *out == "" {
		fmt.Print(b.String())
		return
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d diagrams, warm median: mermaid %.2f ms, mmdg %.2f ms\n", *out, len(samples), quantile(mmWarm, 0.5), quantile(goWarm, 0.5))
}
