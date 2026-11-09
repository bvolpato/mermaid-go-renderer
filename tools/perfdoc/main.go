// Command perfdoc measures the render time of every diagram of a corpus and
// compares it with the time that mermaid took for the same diagram in
// Chromium. It writes docs/PERFORMANCE.md and docs/performance.json. The
// website and the README take their numbers from the JSON file.
//
// tools/reference/bench.mjs runs the whole measurement and calls this
// command as its last step:
//
//	node tools/reference/bench.mjs
//
// The command alone, with files of "oracle.mjs bench" and "bench-cli.mjs":
//
//	go run ./tools/perfdoc -warm 'DIR/warm-*.json' -cold 'DIR/cold-*.json' -cli DIR/cli.json
//
// With -from-json the command does not measure. It writes the Markdown file
// again from the JSON file.
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
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/perfdata"
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
	warm := flag.String("warm", ".parity/perf/bench/warm-*.json", "files of \"oracle.mjs bench\": mermaid in a page that stays open")
	cold := flag.String("cold", ".parity/perf/bench/cold-*.json", "files of \"oracle.mjs bench --fresh\": mermaid in a new page for each diagram")
	repeat := flag.Int("repeat", 7, "renders of each diagram; the fastest counts")
	systemFonts := flag.Bool("system-fonts", false, "measure with the system fonts")
	cli := flag.String("cli", "", "JSON file of bench-cli.mjs with the comparison of the command-line tools")
	out := flag.String("o", "docs/PERFORMANCE.md", "Markdown file to write")
	jsonOut := flag.String("json", "docs/performance.json", "JSON file to write, for the website and the README")
	fromJSON := flag.Bool("from-json", false, "do not measure: write the Markdown file again from the JSON file")
	child := flag.String("child", "", "render this file one time and exit (the first-render measurement uses it)")
	flag.Parse()

	if *fromJSON {
		data, err := perfdata.Load(*jsonOut)
		if err == nil {
			err = os.WriteFile(*out, []byte(markdown(data)), 0o644)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

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
	// The JSON file has the digits of the tables.
	round := func(v float64, digits int) float64 {
		p := math.Pow(10, float64(digits))
		return math.Round(v*p) / p
	}

	data := &perfdata.Data{
		Measured:      time.Now().Format("2006-01-02"),
		Mermaid:       warmMeta.Mermaid,
		Chromium:      warmMeta.Chromium,
		Platform:      runtime.GOOS + "/" + runtime.GOARCH,
		CPUs:          runtime.NumCPU(),
		Corpus:        filepath.ToSlash(*corpus),
		Diagrams:      len(samples),
		RepeatMermaid: warmMeta.Repeat,
		RepeatMmdg:    *repeat,
		WarmMedian:    perfdata.Pair{Mermaid: round(quantile(mmWarm, 0.5), 2), Mmdg: round(quantile(goWarm, 0.5), 2)},
		WarmP90:       perfdata.Pair{Mermaid: round(quantile(mmWarm, 0.9), 2), Mmdg: round(quantile(goWarm, 0.9), 2)},
		WarmTotal:     perfdata.Pair{Mermaid: round(sum(mmWarm)/1000, 2), Mmdg: round(sum(goWarm)/1000, 2)},
	}
	if len(coldBoth) > 0 {
		data.FirstMedian = perfdata.Pair{Mermaid: round(quantile(mmCold, 0.5), 1), Mmdg: round(quantile(goCold, 0.5), 1)}
		data.FirstP90 = perfdata.Pair{Mermaid: round(quantile(mmCold, 0.9), 1), Mmdg: round(quantile(goCold, 0.9), 1)}
	}
	for _, t := range types {
		list := byType[t]
		g := column(list, func(s sample) float64 { return s.goWarm })
		m := column(list, func(s sample) float64 { return s.mermaidWarm })
		row := perfdata.Type{Type: t, Diagrams: len(list), Mermaid: round(quantile(m, 0.5), 2), Mmdg: round(quantile(g, 0.5), 2), Mmdg90: round(quantile(g, 0.9), 2)}
		if v := column(list, func(s sample) float64 { return s.mermaidCold }); len(v) > 0 {
			row.MermaidFirst = round(quantile(v, 0.5), 1)
		}
		if v := column(list, func(s sample) float64 { return s.goCold }); len(v) > 0 {
			row.MmdgFirst = round(quantile(v, 0.5), 1)
		}
		data.Types = append(data.Types, row)
	}
	if *cli != "" {
		raw, err := os.ReadFile(*cli)
		if err == nil {
			data.CLI = &perfdata.CLI{}
			err = json.Unmarshal(raw, data.CLI)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if err := data.Save(*jsonOut); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, []byte(markdown(data)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s and %s: %d diagrams, warm median: mermaid %.2f ms, mmdg %.2f ms\n", *out, *jsonOut, data.Diagrams, data.WarmMedian.Mermaid, data.WarmMedian.Mmdg)
}

// markdown writes docs/PERFORMANCE.md from the data.
func markdown(d *perfdata.Data) string {
	var b strings.Builder
	b.WriteString("# Performance\n\n")
	b.WriteString("This file is generated. Do not edit it. `node tools/reference/bench.mjs` runs the measurement and writes this file and `docs/performance.json`, which has the same numbers for the website and the README.\n\n")
	fmt.Fprintf(&b, "The numbers compare `mmdg` with mermaid %s in Chromium %s, for %d diagrams of the corpus `%s`: the diagrams that both sides rendered on the day of the measurement. Both ran on the same machine (%s, %d CPUs) on %s, one diagram at a time. Times are in milliseconds.\n\n", d.Mermaid, d.Chromium, d.Diagrams, d.Corpus, d.Platform, d.CPUs, d.Measured)

	b.WriteString("## Summary\n\n")
	b.WriteString("| Measurement | mermaid in Chromium | mmdg | Ratio |\n")
	b.WriteString("|---|---:|---:|---:|\n")
	line := func(name string, p perfdata.Pair, format string) {
		fmt.Fprintf(&b, "| %s | "+format+" | "+format+" | %.0fx |\n", name, p.Mermaid, p.Mmdg, p.Ratio())
	}
	line("Warm render, median", d.WarmMedian, "%.2f")
	line("Warm render, 90th percentile", d.WarmP90, "%.2f")
	line("Warm render, all diagrams, in seconds", d.WarmTotal, "%.2f")
	if d.FirstMedian.Mmdg > 0 {
		line("First render, median", d.FirstMedian, "%.1f")
		line("First render, 90th percentile", d.FirstP90, "%.1f")
	}
	b.WriteString("\n")
	b.WriteString("What each measurement is:\n\n")
	fmt.Fprintf(&b, "- **Warm render, mermaid**: the fastest of %d `mermaid.render()` calls for the diagram, in a page that stays open and renders many diagrams. V8 has compiled the code.\n", d.RepeatMermaid)
	fmt.Fprintf(&b, "- **Warm render, mmdg**: the fastest of %d `mermaid.Render` calls for the diagram, in a process that stays running.\n", d.RepeatMmdg)
	b.WriteString("- **First render, mermaid**: the first `mermaid.render()` call in a new page. The browser is running and the page has loaded mermaid. The start of the browser and the load of the script are not in the time.\n")
	b.WriteString("- **First render, mmdg**: a new process that reads the file, renders it one time and exits. The start and the end of the process are in the time.\n\n")
	b.WriteString("The first-render comparison favors mermaid, because it leaves out the start of the browser. A tool that starts a browser for each run, such as mermaid-cli, pays that start each time.\n\n")

	b.WriteString("## By diagram type\n\n")
	b.WriteString("| Diagram type | Diagrams | mermaid warm | mmdg warm | mmdg warm, 90th percentile | Warm ratio | mermaid first | mmdg first |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|\n")
	first := func(v float64) string {
		if v == 0 {
			return ""
		}
		return fmt.Sprintf("%.1f", v)
	}
	for _, t := range d.Types {
		fmt.Fprintf(&b, "| `%s` | %d | %.2f | %.2f | %.2f | %.0fx | %s | %s |\n", t.Type, t.Diagrams, t.Mermaid, t.Mmdg, t.Mmdg90, t.Mermaid/t.Mmdg, first(t.MermaidFirst), first(t.MmdgFirst))
	}
	fmt.Fprintf(&b, "| **All** | %d | %.2f | %.2f | %.2f | %.0fx | %s | %s |\n", d.Diagrams, d.WarmMedian.Mermaid, d.WarmMedian.Mmdg, d.WarmP90.Mmdg, d.WarmMedian.Ratio(), first(d.FirstMedian.Mermaid), first(d.FirstMedian.Mmdg))
	b.WriteString("\nThe columns have the median of the diagrams of the type, except where the heading says otherwise.\n")

	if d.CLI != nil {
		b.WriteString("\n## Command-line tools\n\n")
		fmt.Fprintf(&b, "The table compares `mmdg` with `mmdc` (mermaid-cli %s). Each run is one new process that reads a diagram file and writes one output file. The time is the wall time of the process in milliseconds: the fastest of %d runs, after %d runs of each tool that do not count. `mmdc` starts a browser in each run.\n\n", d.CLI.Mmdc, d.CLI.Runs, d.CLI.Warmup)
		b.WriteString("| Input | Output | mmdc | mmdg | Ratio |\n")
		b.WriteString("|---|---|---:|---:|---:|\n")
		for _, r := range d.CLI.Rows {
			fmt.Fprintf(&b, "| %s | %s | %.0f | %.1f | %.0fx |\n", r.Input, r.Output, r.Mmdc, r.Mmdg, r.Mmdc/r.Mmdg)
		}
	}
	return b.String()
}
