// This file ports no upstream file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/perfdata"
)

// The README and docs/CLI.md have blocks that this command writes: the
// tables and the sentences with numbers. A block is between two comments:
//
//	<!-- generated:NAME start -->
//	<!-- generated:NAME end -->
//
// The numbers come from files of the repository, not from a render: the
// manifests in testdata/reference with their lists, and
// docs/performance.json. So the command gives the same text on every
// machine, also for the reference of macOS. The test TestCorpusParity keeps
// the lists true.

// listCounts is the result of one manifest, as its list says it.
type listCounts struct {
	Total     int
	Exact     int // the same bytes as the reference
	SameError int // the reference fails, and this module fails too
}

func (c listCounts) Pass() int { return c.Exact + c.SameError }

func (c listCounts) percent() string {
	return fmt.Sprintf("%.1f%%", 100*float64(c.Pass())/float64(c.Total))
}

// countManifest reads a manifest and its list. A manifest of a plain page
// has a list of the fixtures that do not pass (.expected.txt). A manifest
// of the page of mermaid-cli has a list of the fixtures that pass
// (.passing.txt).
func countManifest(name string) (listCounts, error) {
	path := filepath.Join("testdata", "reference", name+".json")
	m, err := parity.LoadManifest(path)
	if err != nil {
		return listCounts{}, err
	}
	listed := map[string]string{}
	listFile := filepath.Join("testdata", "reference", name+".expected.txt")
	passing := m.Page == parity.PageMermaidCLI
	if passing {
		listFile = filepath.Join("testdata", "reference", name+".passing.txt")
	}
	data, err := os.ReadFile(listFile)
	if err != nil {
		return listCounts{}, err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if status, fixture, ok := strings.Cut(line, "\t"); ok {
			listed[fixture] = status
		}
	}
	c := listCounts{Total: len(m.Fixtures)}
	for _, fixture := range m.Names() {
		status, inList := listed[fixture]
		pass := !inList
		if passing {
			pass = status == "pass"
		}
		switch {
		case !pass:
			continue
		case m.Fixtures[fixture].Error:
			c.SameError++
		default:
			c.Exact++
		}
	}
	return c, nil
}

// corpora lists the three corpora with their manifest names and labels.
var corpora = []struct{ suffix, label string }{
	{"", "The end-to-end tests of mermaid"},
	{"-docs", "The examples of the mermaid documentation"},
	{"-large", "Generated large diagrams, 15 to 150 nodes"},
}

// generatedText returns the blocks of each file.
func generatedText(perf *perfdata.Data) (map[string]map[string]string, error) {
	var linux, macos, linuxCLI, macosCLI []listCounts
	for _, c := range corpora {
		for _, set := range []struct {
			name string
			into *[]listCounts
		}{
			{"linux" + c.suffix, &linux}, {"macos" + c.suffix, &macos},
			{"linux-mmdc" + c.suffix, &linuxCLI}, {"macos-mmdc" + c.suffix, &macosCLI},
		} {
			counts, err := countManifest(set.name)
			if err != nil {
				return nil, err
			}
			*set.into = append(*set.into, counts)
		}
	}
	sum := func(list []listCounts, get func(listCounts) int) int {
		n := 0
		for _, c := range list {
			n += get(c)
		}
		return n
	}
	warm, cli := perf.WarmMedian, perf.CLI.Rows[0]
	cliRatio := cli.Mmdc / cli.Mmdg

	// ---- README: the chart of the speed ----
	speed := fmt.Sprintf(`<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/speed-dark.svg">
    <img src="docs/assets/speed-light.svg" alt="mmdg is %.0f times faster than mermaid in a warm Chromium page, and %.0f times faster than mmdc as a command" width="860">
  </picture>
</p>
`, warm.Ratio(), cliRatio)

	// ---- README: the result ----
	var r strings.Builder
	fmt.Fprintf(&r, `<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/parity-dark.svg">
    <img src="docs/assets/parity-light.svg" alt="One square for each of the %d diagrams of the end-to-end tests of mermaid. %d give the output of mermaid, byte for byte." width="860">
  </picture>
</p>

`, linux[0].Total, linux[0].Exact)
	r.WriteString("`mmdg` can replace two things, and each has its own proof.\n\n")
	r.WriteString("**`mermaid.render()` in a web page.** This is the default of the library.\n\n")
	r.WriteString("| Corpus | Diagrams | Pass | Byte-exact | Same error as mermaid |\n|---|---:|---:|---:|---:|\n")
	for i, c := range linux {
		fmt.Fprintf(&r, "| %s | %d | %d (%s) | %d | %d |\n", corpora[i].label, c.Total, c.Pass(), c.percent(), c.Exact, c.SameError)
	}
	r.WriteString("\n- \"Pass\" means byte-exact output, or matching errors when the reference also rejects a diagram.\n")
	r.WriteString("- The corpora cover Mermaid's end-to-end tests, its documentation examples, and generated large diagrams.\n")
	same := true
	for i := range linux {
		same = same && linux[i].Pass() == macos[i].Pass() && linux[i].Exact == macos[i].Exact
	}
	if same {
		r.WriteString("- Each corpus passes with the same counts against two references: Chromium 156 on Linux with the Liberation fonts, and Chromium 156 on macOS with the system fonts.\n")
	} else {
		fmt.Fprintf(&r, "- The table is for Chromium 156 on Linux with the Liberation fonts. Against Chromium 156 on macOS with the system fonts, %d, %d and %d diagrams pass.\n", macos[0].Pass(), macos[1].Pass(), macos[2].Pass())
	}
	r.WriteString("\n**The `mmdc` command of mermaid-cli 12.** This is the default of the `mmdg` command. `mmdc` renders in a page of its own, with web fonts that a plain page has not (see [Pages](#pages)), and it writes the SVG as XML with the fonts embedded.\n\n")
	r.WriteString("| Corpus | Diagrams | Pass on Linux | Pass on macOS |\n|---|---:|---:|---:|\n")
	for i, c := range linuxCLI {
		fmt.Fprintf(&r, "| %s | %d | %d (%s) | %d (%s) |\n", corpora[i].label, c.Total, c.Pass(), c.percent(), macosCLI[i].Pass(), macosCLI[i].percent())
	}
	r.WriteString("\n- \"Pass\" means that both files of `mmdc` have the same bytes, or that both tools report an error for a diagram.\n")
	otherOutcomes := func(c listCounts) int { return c.Total - c.Pass() }
	if more := sum(macosCLI, otherOutcomes) - sum(linuxCLI, otherOutcomes); more > 0 {
		fmt.Fprintf(&r, "- On macOS, %d more diagrams do not match both mmdc output files.\n", more)
	}

	// ---- README: the performance ----
	var p strings.Builder
	p.WriteString("| Measurement | mermaid in Chromium | mmdg | Ratio |\n|---|---:|---:|---:|\n")
	fmt.Fprintf(&p, "| One render in a warm process, median of %d diagrams | %s ms | %s ms | %.0fx |\n", perf.Diagrams, trim(warm.Mermaid, 2), trim(warm.Mmdg, 2), warm.Ratio())
	fmt.Fprintf(&p, "| The same, 90th percentile | %s ms | %s ms | %.0fx |\n", trim(perf.WarmP90.Mermaid, 2), trim(perf.WarmP90.Mmdg, 2), perf.WarmP90.Ratio())
	for _, row := range perf.CLI.Rows {
		if row.Input != cli.Input {
			break
		}
		name := "Command-line run, flowchart to " + row.Output
		if row.Output == cli.Output {
			name += " (`mmdc` and `mmdg`)"
		}
		fmt.Fprintf(&p, "| %s | %.0f ms | %s ms | %.0fx |\n", name, row.Mmdc, trim(row.Mmdg, 1), row.Mmdc/row.Mmdg)
	}
	p.WriteString("\n- The first two rows compare `mermaid.render()` in a Chromium page that stays open with `mermaid.Render` in a Go process that stays running. Both sides are warm.\n")
	p.WriteString("- The last two rows compare whole processes. `mmdc` starts a browser in each run.\n")
	fmt.Fprintf(&p, "- All numbers are from one machine, measured on %s. [docs/PERFORMANCE.md](docs/PERFORMANCE.md) has the numbers for each diagram type and says how they were measured.\n", perf.Measured)

	// ---- docs/CLI.md: the corpus table ----
	var c strings.Builder
	c.WriteString("| Corpus | Diagrams | Pass | Other outcomes |\n| --- | ---: | ---: | ---: |\n")
	for i, counts := range linuxCLI {
		fmt.Fprintf(&c, "| %s | %d | %d | %d |\n", corpora[i].label, counts.Total, counts.Pass(), counts.Total-counts.Pass())
	}
	c.WriteString("\nPass requires both mmdc output files to match, or both tools to return an error. Other outcomes include partial matches and different output. See [What differs from mmdc](#what-differs-from-mmdc).\n\n")
	fmt.Fprintf(&c, "With `--fonts system` on macOS, the reference is Chromium on macOS with its system fonts. There %d of %d, %d of %d and %d of %d diagrams pass.", macosCLI[0].Pass(), macosCLI[0].Total, macosCLI[1].Pass(), macosCLI[1].Total, macosCLI[2].Pass(), macosCLI[2].Total)
	if more := macosCLI[0].Total - macosCLI[0].Pass() - (linuxCLI[0].Total - linuxCLI[0].Pass()); more > 0 {
		fmt.Fprintf(&c, " %d more diagrams of the first corpus do not match both output files.", more)
	}
	c.WriteString(" [PARITY-MMDC-MACOS.md](PARITY-MMDC-MACOS.md) has the tables.\n")

	return map[string]map[string]string{
		"README.md":                     {"speed": speed, "result": r.String(), "performance": p.String()},
		filepath.Join("docs", "CLI.md"): {"corpus": c.String()},
	}, nil
}

// trim prints a number with at most the given digits after the point, and
// without zeros at the end.
func trim(v float64, digits int) string {
	s := fmt.Sprintf("%.*f", digits, v)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

// replaceBlocks puts the text of each block between its two comments.
func replaceBlocks(text string, blocks map[string]string) (string, error) {
	for name, body := range blocks {
		start := "<!-- generated:" + name + " start -->\n"
		end := "<!-- generated:" + name + " end -->"
		i := strings.Index(text, start)
		j := strings.Index(text, end)
		if i < 0 || j < i {
			return "", fmt.Errorf("the block %q has no start and end comment", name)
		}
		text = text[:i+len(start)] + body + text[j:]
	}
	return text, nil
}

// writeGeneratedText writes the blocks into the files.
func writeGeneratedText(perf *perfdata.Data) error {
	files, err := generatedText(perf)
	if err != nil {
		return err
	}
	for path, blocks := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text, err := replaceBlocks(string(data), blocks)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return err
		}
	}
	return nil
}
