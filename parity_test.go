package mermaid

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
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
)

var updateParity = flag.Bool("update-parity", false, "rewrite the expected results of TestCorpusParity")

// TestCorpusParity renders every fixture of the corpus and compares the
// SHA-256 of the SVG with the hash of the SVG that mermaid produced in
// Chromium (testdata/reference/*.json, written by tools/manifest from the
// output of tools/reference/oracle.mjs).
//
// A fixture is "exact" when the hashes are equal, so that the output is
// byte for byte the output of mermaid. The fixtures that are not exact yet
// are listed, with their status, in the .expected.txt file next to the
// manifest. The test fails when a fixture gets worse. When fixtures get
// better, run the test with -update-parity to shorten the list.
//
// A manifest with a page (linux-mmdc.json and the like) is a reference of
// the page of mermaid-cli. See corpusPageParity.
func TestCorpusParity(t *testing.T) {
	manifests, _ := filepath.Glob(filepath.Join("testdata", "reference", "*.json"))
	if len(manifests) == 0 {
		t.Skip("no reference manifest")
	}
	for _, path := range manifests {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		t.Run(name, func(t *testing.T) {
			m, err := parity.LoadManifest(path)
			if err != nil {
				t.Fatal(err)
			}
			if m.Corpus == "" {
				m.Corpus = "mermaid-e2e"
			}
			if m.Mermaid != MermaidVersion {
				t.Fatalf("the manifest is for mermaid %s, this package ports %s", m.Mermaid, MermaidVersion)
			}
			if m.Fonts == "system" {
				if fp := parity.SystemFontFingerprint(); fp != m.FontFingerprint {
					t.Skipf("the reference %s uses the system fonts of another machine (fingerprint %s, here %q)", m.Environment, m.FontFingerprint, fp)
				}
			}
			if m.Page == parity.PageMermaidCLI {
				corpusPageParity(t, m, strings.TrimSuffix(path, ".json")+".passing.txt")
				return
			}
			corpusParity(t, m, strings.TrimSuffix(path, ".json")+".expected.txt")
		})
	}
}

// corpusPageParity is corpusParity for a reference of the page of
// mermaid-cli (tools/reference/mmdc-oracle.mjs). The reference of a fixture
// is the .svg file that mermaid-cli writes, in two variants: without the
// fonts of the page (--no-font-embed) and with them (the default).
//
// The mode is new, so the list next to the manifest is the other way
// around: the .passing.txt file lists the fixtures that pass, with "pass"
// when both files are the files of mermaid-cli and "pass-svg" when only the
// file without fonts is. The test fails when a fixture of the list gets
// worse. When more fixtures pass, run the test with -update-parity to add
// them to the list.
func corpusPageParity(t *testing.T, m *parity.Manifest, passingPath string) {
	names := m.Names()
	if testing.Short() {
		var sample []string
		for i := 0; i < len(names); i += 10 {
			sample = append(sample, names[i])
		}
		names = sample
	}
	status := make([]string, len(names))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, name := range names {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			status[i] = pageFixtureStatus(m, name)
		}()
	}
	wg.Wait()

	rank := map[string]int{"pass": 2, "pass-svg": 1}
	listed := map[string]string{}
	if data, err := os.ReadFile(passingPath); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if st, name, ok := strings.Cut(line, "\t"); ok {
				listed[name] = st
			}
		}
	}
	tally := map[string]int{}
	var worse, better []string
	for i, name := range names {
		got := status[i]
		tally[got]++
		want := listed[name]
		switch {
		case rank[got] < rank[want]:
			worse = append(worse, fmt.Sprintf("%s: %s, the list has %s", name, got, want))
		case rank[got] > rank[want]:
			better = append(better, name)
		}
	}
	keys := make([]string, 0, len(tally))
	for k := range tally {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, tally[k]))
	}
	t.Logf("%s, page %s: %d fixtures: %s", m.Environment, m.Page, len(names), strings.Join(parts, ", "))

	if *updateParity && !testing.Short() {
		var b strings.Builder
		for i, name := range names {
			if rank[status[i]] > 0 {
				b.WriteString(status[i] + "\t" + name + "\n")
			}
		}
		if err := os.WriteFile(passingPath, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if len(better) > 0 {
		t.Logf("%d fixtures are better than %s says; run go test -run TestCorpusParity -update-parity .", len(better), filepath.Base(passingPath))
	}
	if len(worse) > 0 {
		if len(worse) > 25 {
			worse = append(worse[:25], fmt.Sprintf("and %d more", len(worse)-25))
		}
		t.Errorf("fixtures that got worse:\n  %s", strings.Join(worse, "\n  "))
	}
}

// pageFixtureStatus renders one fixture in the page of mermaid-cli, makes
// the two .svg files of mermaid-cli from it, and compares them with the
// reference. "pass" means the same bytes as mermaid-cli for both files, or
// an error where mermaid-cli also fails. "pass-svg" means the same bytes
// for the file without fonts only.
func pageFixtureStatus(m *parity.Manifest, name string) (status string) {
	ref := m.Fixtures[name]
	src, err := os.ReadFile(filepath.Join("testdata", "corpus", m.Corpus, filepath.FromSlash(name)+".mmd"))
	if err != nil {
		return "missing"
	}
	opts := []Option{WithImageSize(parity.CorpusImageSize), WithPage(PageMermaidCLI)}
	if m.Fonts == "system" {
		opts = append(opts, WithSystemFonts())
	}
	svg, err := Render(string(src), opts...)
	var unsupported *UnsupportedDiagramError
	var internal *InternalError
	switch {
	case errors.As(err, &internal):
		return "panic"
	case ref.Error && err != nil:
		return "pass"
	case ref.Error:
		return "renders-where-mermaid-fails"
	case errors.As(err, &unsupported):
		return "unsupported"
	case err != nil:
		return "error"
	}
	// mermaid-cli sets the background "white" by default.
	opts = append(opts, WithBackgroundColor("white"))
	file, err := StandaloneSVG(svg, opts...)
	if err != nil {
		return "error"
	}
	if parity.HashSVG(file) != ref.SHA256 {
		return "different"
	}
	// The file with fonts is the same file when the diagram uses no font
	// of the page.
	want := ref.EmbedSHA256
	if want == "" {
		want = ref.SHA256
	}
	embedded, err := StandaloneSVG(svg, append(opts, WithFontEmbed(true))...)
	if err != nil || parity.HashSVG(embedded) != want {
		return "pass-svg"
	}
	return "pass"
}

func corpusParity(t *testing.T, m *parity.Manifest, expectedPath string) {
	names := m.Names()
	if testing.Short() {
		var sample []string
		for i := 0; i < len(names); i += 10 {
			sample = append(sample, names[i])
		}
		names = sample
	}
	status := make([]string, len(names))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i, name := range names {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			status[i] = fixtureStatus(m, name)
		}()
	}
	wg.Wait()

	expected := map[string]string{}
	if data, err := os.ReadFile(expectedPath); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if st, name, ok := strings.Cut(line, "\t"); ok {
				expected[name] = st
			}
		}
	}

	tally := map[string]int{}
	var worse, better []string
	for i, name := range names {
		got := status[i]
		tally[got]++
		want, listed := expected[name]
		if !listed {
			want = "pass"
		}
		switch {
		case got == want:
		case got == "pass":
			better = append(better, name)
		default:
			worse = append(worse, fmt.Sprintf("%s: %s, expected %s", name, got, want))
		}
	}
	keys := make([]string, 0, len(tally))
	for k := range tally {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, tally[k]))
	}
	t.Logf("%s: %d fixtures: %s", m.Environment, len(names), strings.Join(parts, ", "))

	if *updateParity && !testing.Short() {
		var b strings.Builder
		for i, name := range names {
			if status[i] != "pass" {
				b.WriteString(status[i] + "\t" + name + "\n")
			}
		}
		if err := os.WriteFile(expectedPath, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if len(better) > 0 {
		t.Logf("%d fixtures are better than %s says; run go test -run TestCorpusParity -update-parity .", len(better), filepath.Base(expectedPath))
	}
	if len(worse) > 0 {
		if len(worse) > 25 {
			worse = append(worse[:25], fmt.Sprintf("and %d more", len(worse)-25))
		}
		t.Errorf("fixtures that got worse:\n  %s", strings.Join(worse, "\n  "))
	}
}

// fixtureStatus renders one fixture and compares it with its reference.
// "pass" means the same bytes as mermaid, or an error where mermaid also
// fails.
func fixtureStatus(m *parity.Manifest, name string) (status string) {
	ref := m.Fixtures[name]
	src, err := os.ReadFile(filepath.Join("testdata", "corpus", m.Corpus, filepath.FromSlash(name)+".mmd"))
	if err != nil {
		return "missing"
	}
	// A reference from system fonts needs the same fonts. The other
	// references use the fonts of this module, which are the default.
	opts := []Option{WithImageSize(parity.CorpusImageSize)}
	if m.Fonts == "system" {
		opts = append(opts, WithSystemFonts())
	}
	svg, err := Render(string(src), opts...)
	var unsupported *UnsupportedDiagramError
	var internal *InternalError
	switch {
	case errors.As(err, &internal):
		return "panic"
	case ref.Error && err != nil:
		return "pass"
	case ref.Error:
		return "renders-where-mermaid-fails"
	case errors.As(err, &unsupported):
		return "unsupported"
	case err != nil:
		return "error"
	case parity.HashSVG(svg) == ref.SHA256:
		return "pass"
	}
	return "different"
}
