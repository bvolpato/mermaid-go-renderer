package mermaid

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/mmdc"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
)

// The golden files of these tests come from Chromium: the script
// tools/reference/gen-xml-serialize.mjs puts an SVG into a page as
// mermaid-cli does, runs the steps of mermaid-cli, and records
// new XMLSerializer().serializeToString(svg).

// standaloneVariant is one set of steps of mermaid-cli.
type standaloneVariant struct {
	Name string `json:"name"`
	// Background is the value of svg.style.backgroundColor, when it is set.
	Background *string `json:"background"`
	// Size is the value of --size of mermaid-cli 12.
	Size int `json:"size"`
	// CSS is the text of the style element that mermaid-cli 11 appends for
	// its option --cssFile.
	CSS string `json:"css"`
}

// apply runs the steps of the variant on an SVG that mermaid.render()
// returned.
func (v standaloneVariant) apply(svg string) (string, error) {
	if v.Size == 0 && v.CSS == "" {
		// The public function covers these variants.
		if v.Background == nil {
			return StandaloneSVG(svg)
		}
		return StandaloneSVG(svg, WithBackgroundColor(*v.Background))
	}
	opts := mmdc.Options{Size: v.Size}
	if v.Background != nil {
		opts.Background, opts.SetBackground = *v.Background, true
	}
	page, err := mmdc.Load(svg, opts)
	if err != nil {
		return "", err
	}
	if v.CSS != "" {
		doc := page.SVG.Document()
		style := doc.CreateElementNS(dom.NSSVG, "style")
		style.AppendChild(doc.CreateText(v.CSS))
		page.SVG.AppendChild(style)
	}
	return page.XML(), nil
}

// hash16 is the hash of the golden files: the first 16 hex digits of the
// SHA-256.
func hash16(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:16]
}

// TestStandaloneSVGBackground compares the style of the svg element after
// svg.style.backgroundColor = value, for many values and several style
// attributes.
func TestStandaloneSVGBackground(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "standalone", "backgrounds.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		SVG        string `json:"svg"`
		Background string `json:"background"`
		XML        string `json:"xml"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 400 {
		t.Fatalf("only %d cases", len(cases))
	}
	failed, gaps := 0, map[string]bool{}
	for _, c := range cases {
		got, err := StandaloneSVG(c.SVG, WithBackgroundColor(c.Background))
		if err != nil {
			t.Errorf("%q: %v", c.Background, err)
			continue
		}
		if cssColorGaps[c.Background] {
			if got != c.XML {
				gaps[c.Background] = true
			}
			continue
		}
		if got != c.XML {
			failed++
			if failed <= 20 {
				t.Errorf("background %q on %s\n got: %s\nwant: %s", c.Background, c.SVG, got, c.XML)
			}
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d cases differ", failed, len(cases))
	}
	for value := range cssColorGaps {
		if !gaps[value] {
			t.Errorf("background %q now gives the style of Chromium: remove it from cssColorGaps", value)
		}
	}
	t.Logf("%d cases, %d values that the CSS parser of this module does not read", len(cases), len(cssColorGaps))
}

// cssColorGaps are values of --backgroundColor that Chromium accepts as a
// color and that internal/css does not read: the svg element then keeps its
// style, where Chromium adds the background. They are color syntaxes other
// than names, hex colors, rgb() and hsl(), and CSS comments, escapes and
// calc().
var cssColorGaps = map[string]bool{
	"hwb(120 10% 20%)":              true,
	"lab(50% 40 30)":                true,
	"lch(50% 40 30)":                true,
	"oklab(0.5 0.1 0.1)":            true,
	"oklch(0.5 0.1 30)":             true,
	"color(display-p3 1 0 0)":       true,
	"color(srgb 1 0.5 0 / 0.5)":     true,
	"color-mix(in srgb, red, blue)": true,
	"light-dark(red, blue)":         true,
	"black/**/":                     true,
	`\72 ed`:                        true,
	"rgb(calc(1 + 2), 0, 0)":        true,
	"WINDOW":                        true,
}

// TestStandaloneSVGSample compares the full text for a sample of the
// corpus: the smallest reference SVG of every diagram type and of every
// kind of label. The golden file has the input and the output, so the test
// needs no renderer and no fonts.
func TestStandaloneSVGSample(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "standalone", "sample.json.gz"))
	if err != nil {
		t.Skip("no sample: run tools/reference/gen-xml-serialize.mjs sample")
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Variants []standaloneVariant `json:"variants"`
		Sample   []struct {
			Name   string            `json:"name"`
			Covers []string          `json:"covers"`
			SVG    string            `json:"svg"`
			Out    map[string]string `json:"out"`
		} `json:"sample"`
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Sample) < 30 || len(golden.Variants) < 5 {
		t.Fatalf("%d references and %d variants", len(golden.Sample), len(golden.Variants))
	}
	covered := map[string]bool{}
	for _, s := range golden.Sample {
		for _, c := range s.Covers {
			covered[c] = true
		}
		for _, v := range golden.Variants {
			got, err := v.apply(s.SVG)
			if err != nil {
				t.Errorf("%s (%s): %v", s.Name, v.Name, err)
				continue
			}
			if want := s.Out[v.Name]; got != want {
				t.Errorf("%s (%s) differs at byte %d\n got: %s\nwant: %s", s.Name, v.Name, firstDiff(got, want), around(got, firstDiff(got, want)), around(want, firstDiff(got, want)))
			}
		}
	}
	types, labels := 0, 0
	for c := range covered {
		if strings.HasPrefix(c, "type:") {
			types++
		} else {
			labels++
		}
	}
	t.Logf("%d references: %d diagram types, %d kinds of label, %d variants", len(golden.Sample), types, labels, len(golden.Variants))
}

func firstDiff(a, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func around(s string, i int) string {
	return s[max(0, i-80):min(len(s), i+80)]
}

// standaloneGolden is testdata/standalone/<environment>.json: for every
// reference SVG of the corpus, the hash of the SVG and the hash of the
// output of every variant.
type standaloneGolden struct {
	Environment string                       `json:"environment"`
	Variants    []standaloneVariant          `json:"variants"`
	Fixtures    map[string]map[string]string `json:"fixtures"`
}

// TestStandaloneSVGCorpus runs every variant on the reference SVG of every
// fixture of the corpus and compares the hash of the result with the hash
// of what Chromium returned.
//
// The input is the reference SVG itself, when the directory of the
// reference exists (.parity, which is not part of the repository). Without
// it the input is what Render returns for the fixture, if that is the
// reference byte for byte. The test counts the fixtures that it cannot
// check in this way.
func TestStandaloneSVGCorpus(t *testing.T) {
	for _, env := range []struct{ golden, manifest, refDir string }{
		{"macos", "macos", filepath.Join(".parity", "ref", "mermaid-e2e")},
		{"linux", "linux", filepath.Join(".parity", "ref-linux", "mermaid-e2e")},
	} {
		t.Run(env.golden, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "standalone", env.golden+".json"))
			if err != nil {
				t.Skip("no golden file: run tools/reference/gen-xml-serialize.mjs corpus")
			}
			var golden standaloneGolden
			if err := json.Unmarshal(data, &golden); err != nil {
				t.Fatal(err)
			}
			m, err := parity.LoadManifest(filepath.Join("testdata", "reference", env.manifest+".json"))
			if err != nil {
				t.Fatal(err)
			}
			canRender := m.Fonts != "system" || parity.SystemFontFingerprint() == m.FontFingerprint
			names := make([]string, 0, len(golden.Fixtures))
			for name := range golden.Fixtures {
				names = append(names, name)
			}
			sort.Strings(names)
			if testing.Short() {
				var sample []string
				for i := 0; i < len(names); i += 10 {
					sample = append(sample, names[i])
				}
				names = sample
			}

			// status is "" for a fixture that passes, "skip" for one that
			// has no input, and a message for one that fails.
			status := make([]string, len(names))
			var wg sync.WaitGroup
			sem := make(chan struct{}, runtime.NumCPU())
			for i, name := range names {
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					want := golden.Fixtures[name]
					input, ok := standaloneInput(m, env.refDir, name, want["in"], canRender)
					if !ok {
						status[i] = "skip"
						return
					}
					for _, v := range golden.Variants {
						got, err := v.apply(input)
						if err != nil {
							status[i] = fmt.Sprintf("%s: %v", v.Name, err)
							return
						}
						if hash16(got) != want[v.Name] {
							status[i] = fmt.Sprintf("variant %s differs", v.Name)
							return
						}
					}
				}()
			}
			wg.Wait()

			passed, skipped := 0, 0
			var failed []string
			for i, st := range status {
				switch st {
				case "":
					passed++
				case "skip":
					skipped++
				default:
					failed = append(failed, names[i]+": "+st)
				}
			}
			t.Logf("%s: %d reference SVGs, %d variants each: %d pass, %d fail, %d without input", golden.Environment, len(names), len(golden.Variants), passed, len(failed), skipped)
			if len(failed) > 0 {
				if len(failed) > 25 {
					failed = append(failed[:25], fmt.Sprintf("and %d more", len(failed)-25))
				}
				t.Errorf("fixtures that differ from Chromium:\n  %s", strings.Join(failed, "\n  "))
			}
			if passed == 0 {
				t.Skip("no fixture has an input on this machine")
			}
		})
	}
}

// standaloneInput returns the reference SVG of a fixture: the file of the
// reference directory, or the output of Render when it has the hash of the
// reference.
func standaloneInput(m *parity.Manifest, refDir, name, wantHash string, canRender bool) (string, bool) {
	if data, err := os.ReadFile(filepath.Join(refDir, filepath.FromSlash(name)+".svg")); err == nil && hash16(string(data)) == wantHash {
		return string(data), true
	}
	if !canRender {
		return "", false
	}
	src, err := os.ReadFile(filepath.Join("testdata", "corpus", m.Corpus, filepath.FromSlash(name)+".mmd"))
	if err != nil {
		return "", false
	}
	opts := []Option{WithImageSize(parity.CorpusImageSize)}
	if m.Fonts == "system" {
		opts = append(opts, WithSystemFonts())
	}
	svg, err := Render(string(src), opts...)
	if err != nil || hash16(svg) != wantHash {
		return "", false
	}
	return svg, true
}
