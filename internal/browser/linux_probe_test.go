package browser

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// The tests in this file compare the engine with Chromium on Linux that has
// only the Liberation fonts. The engine uses font.Embedded(), so the tests
// read no font of the machine and give the same result on every platform.
//
// tools/reference/gen-browser-linux-probes.mjs writes the expected values:
// it measures the case files of the macOS probes, and two case sets of its
// own, with `oracle-linux.mjs --fonts liberation`.

// readLinuxJSON reads a JSON file of testdata, with or without gzip
// compression. The second result is false when the file does not exist.
func readLinuxJSON(t *testing.T, name string, v any) bool {
	t.Helper()
	file, err := os.Open("testdata/" + name)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var r io.Reader = file
	if strings.HasSuffix(name, ".gz") {
		zr, err := gzip.NewReader(file)
		if err != nil {
			t.Fatal(err)
		}
		r = zr
	}
	if err := json.NewDecoder(r).Decode(v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return true
}

// linuxProbeSets lists the case sets: the name of the set and the file
// with its cases.
var linuxProbeSets = []struct{ name, cases string }{
	{"arc", "arc-cases.json"},
	{"baseline", "baseline-cases.json"},
	{"boundschange", "boundschange-cases.json"},
	{"brspace", "brspace-cases.json"},
	{"cssvar", "cssvar-cases.json"},
	{"emptytext", "emptytext-cases.json"},
	{"ex", "ex-cases.json"},
	{"fontsize", "fontsize-cases.json"},
	{"htmlbox", "htmlbox-cases.json"},
	{"kernsplit", "kernsplit-cases.json"},
	{"pathbounds", "pathbounds-cases.json"},
	{"scriptrun", "scriptrun-cases.json"},
	{"union", "union-cases.json"},
	{"linuxtext", "linux-linuxtext-cases.json.gz"},
	{"linuxdom", "linux-linuxdom-cases.json.gz"},
}

// linuxKnownGap returns the reason why a value that differs from Chromium
// on Linux does not fail the test, or "".
//
// html: the layout of HTML boxes in a foreignObject is not complete on any
// platform. The macOS probes have the same gaps (the place of an
// inline-block box on the first line, margins that collapse through a box).
// A case fails on Linux only when its fonts make the gap visible.
//
// float: one case of the baseline set differs in the last bit on macOS too.
func linuxKnownGap(set, name, verdict string) string {
	switch {
	case set == "htmlbox" || strings.HasPrefix(name, "html-"):
		return "html"
	case verdict == "close" && set == "baseline":
		return "float"
	}
	return ""
}

// linuxReplay is the result of the replay of one case set.
type linuxReplay struct {
	cases   int
	exact   int            // values that are the numbers of Chromium
	summary string         // exact, close and different values per query kind
	gaps    map[string]int // known gaps by kind
	bad     int            // other values that differ
	fails   []string       // the first of those
}

// replayLinuxSet replays the cases of a file with a font library and
// compares with the results of a file. ok is false when a file is absent.
func replayLinuxSet(t *testing.T, setName, casesFile, resultsFile string, lib *font.Library) (res linuxReplay, ok bool) {
	t.Helper()
	var cases struct{ Cases []probeCase }
	var results []probeResult
	if !readLinuxJSON(t, casesFile, &cases) || !readLinuxJSON(t, resultsFile, &results) {
		return res, false
	}
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	filter := os.Getenv("MMDG_PROBE_FILTER")
	type stat struct{ exact, close, diff int }
	stats := map[string]*stat{}
	res.cases = len(cases.Cases)
	res.gaps = map[string]int{}
	for i, c := range cases.Cases {
		if filter != "" && !strings.Contains(c.Name, filter) {
			continue
		}
		doc := NewDocumentWithFonts(lib)
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		for qi, q := range c.Queries {
			st := stats[q.Op]
			if st == nil {
				st = &stat{}
				stats[q.Op] = st
			}
			var got any
			if q.All {
				vals := []any{}
				for _, el := range container.SelectAll(q.Sel) {
					vals = append(vals, runQuery(el, q))
				}
				got = vals
			} else {
				got = runQuery(container.Select(q.Sel), q)
			}
			want := results[i].Results[qi]
			verdict := compare(got, want, 0.01)
			switch verdict {
			case "exact":
				st.exact++
				res.exact++
				continue
			case "close":
				st.close++
			default:
				st.diff++
			}
			if gap := linuxKnownGap(setName, c.Name, verdict); gap != "" && os.Getenv("MMDG_LINUX_GAPS") == "" {
				res.gaps[gap]++
				continue
			}
			res.bad++
			if len(res.fails) < 25 {
				gotJSON, _ := json.Marshal(got)
				var compact any
				_ = json.Unmarshal(want, &compact)
				wantJSON, _ := json.Marshal(compact)
				html := c.HTML
				if j := strings.Index(html, "</style>"); j >= 0 {
					html = "<svg…>" + html[j+8:]
				}
				if len(html) > 300 {
					html = html[:300] + "…"
				}
				res.fails = append(res.fails, fmt.Sprintf("%s %s(%s): %s\n      got  %s\n      want %s\n      %s", c.Name, q.Op, q.Sel, verdict, gotJSON, wantJSON, html))
			}
		}
	}
	ops := make([]string, 0, len(stats))
	for op := range stats {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	var parts []string
	for _, op := range ops {
		st := stats[op]
		parts = append(parts, fmt.Sprintf("%s exact=%d close=%d diff=%d", op, st.exact, st.close, st.diff))
	}
	res.summary = strings.Join(parts, "; ")
	return res, true
}

// TestLinuxProbesAgainstChromium replays every case set with the embedded
// fonts and requires the numbers of Chromium on Linux.
func TestLinuxProbesAgainstChromium(t *testing.T) {
	for _, set := range linuxProbeSets {
		t.Run(set.name, func(t *testing.T) {
			res, ok := replayLinuxSet(t, set.name, set.cases, "linux-"+set.name+"-results.json.gz", font.Embedded())
			if !ok {
				t.Skip("no data for this set")
			}
			t.Logf("%d cases: %s", res.cases, res.summary)
			if len(res.gaps) > 0 {
				t.Logf("known gaps that do not fail the test (set MMDG_LINUX_GAPS=1 to see them): %v", res.gaps)
			}
			if res.bad > 0 {
				t.Errorf("%d values differ from Chromium on Linux; the first:\n%s", res.bad, strings.Join(res.fails, "\n"))
			}
		})
	}
}

// TestLinuxImageProbesAgainstChromium replays three case sets against
// Chromium in the Playwright image as it is, with a Linux library that has
// the font files and the fontconfig configuration of the image. The files
// are not in the repository: the test is skipped without them.
//
// The fonts of a page that names Arial, Times New Roman or Courier New are
// the Liberation fonts in this environment too, so most values are those of
// the embedded library. The values that differ are text in the default
// fonts of fontconfig (monospace, system-ui: WenQuanYi Zen Hei, for which
// fontconfig turns hinting off, which the library does not follow yet).
// The test requires the counts that the library reaches.
func TestLinuxImageProbesAgainstChromium(t *testing.T) {
	const fonts, config = "../../.parity/linux/image/fonts", "../../.parity/linux/image/etc-fonts"
	if _, err := os.Stat(fonts); err != nil {
		t.Skip("no fonts of the Playwright image:", err)
	}
	lib := font.NewLinuxLibrary(config, fonts)
	for _, set := range []struct {
		name, cases string
		want        int // the least number of exact values
	}{
		{"linuxtext", "linux-linuxtext-cases.json.gz", 6672},
		{"baseline", "baseline-cases.json", 1055},
		{"fontsize", "fontsize-cases.json", 1218},
	} {
		t.Run(set.name, func(t *testing.T) {
			res, ok := replayLinuxSet(t, set.name, set.cases, "linux-image-"+set.name+"-results.json.gz", lib)
			if !ok {
				t.Skip("no data for this set")
			}
			t.Logf("%d cases: %s; known gaps %v; other differences %d", res.cases, res.summary, res.gaps, res.bad)
			if res.exact < set.want {
				t.Errorf("%d exact values, want at least %d; the first differences:\n%s", res.exact, set.want, strings.Join(res.fails, "\n"))
			}
		})
	}
}

// TestLinuxTransformAgainstChromium replays the cases of
// tools/reference/gen-browser-transform-probe.mjs against Chromium on
// Linux: the matrix of a transform attribute and the box that it maps. The
// expected values come from
//
//	cd tools/reference && node oracle-linux.mjs eval browser-transform-probe.js \
//	  --args ../../internal/browser/testdata/transform-cases.json --out out.json
//
// A matrix without a sine, a cosine or a tangent of the C library must be
// the same number. The other matrices depend on the C library of Linux
// (glibc), which does not always round to the nearest number. The engine
// rounds to the nearest number, so a few of these matrices differ in the
// last bit: the test counts them and requires that they are close.
func TestLinuxTransformAgainstChromium(t *testing.T) {
	var cases struct {
		Cases []struct {
			Name      string     `json:"name"`
			Transform string     `json:"transform"`
			Rect      [4]float64 `json:"rect"`
		}
	}
	var results []struct {
		Matrix [6]float64 `json:"matrix"`
		BBox   [4]float64 `json:"bbox"`
	}
	if !readLinuxJSON(t, "transform-cases.json", &cases) || !readLinuxJSON(t, "linux-transform-results.json.gz", &results) {
		t.Skip("no data")
	}
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	closeTo := func(a, b float64) bool {
		return math.Abs(a-b) <= 1e-12*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
	}
	matrices, boxes, libm, libmExact := 0, 0, 0, 0
	for i, c := range cases.Cases {
		want := results[i]
		m := parseTransformList(c.Transform)
		got := [6]float64{m.a, m.b, m.c, m.d, m.e, m.f}
		ok := true
		for k := range got {
			// Zero has no sign in the matrix that getCTM returns.
			ok = ok && (got[k] == want.Matrix[k] || math.Float64bits(got[k]) == math.Float64bits(want.Matrix[k]))
		}
		if usesLibm(c.Transform) {
			libm++
			if ok {
				libmExact++
				continue
			}
			for k := range got {
				if !closeTo(got[k], want.Matrix[k]) {
					t.Errorf("%s matrix of %q\n got  %v\n want %v", c.Name, c.Transform, got, want.Matrix)
					break
				}
			}
			continue
		}
		if !ok {
			t.Errorf("%s matrix of %q\n got  %v\n want %v", c.Name, c.Transform, got, want.Matrix)
			continue
		}
		matrices++
		doc := NewDocumentWithFonts(font.Embedded())
		container := doc.Body.Append("div").Attr("id", "container")
		svg := container.Append("svg").Attr("id", "my-svg").Attr("width", "100%")
		group := svg.Append("g").Attr("id", "p")
		group.Append("g").Attr("transform", c.Transform).Append("rect").
			Attr("x", c.Rect[0]).Attr("y", c.Rect[1]).Attr("width", c.Rect[2]).Attr("height", c.Rect[3])
		b := group.BBox()
		if box := [4]float64{b.X, b.Y, b.Width, b.Height}; box != want.BBox {
			t.Errorf("%s bbox with %q of rect %v\n got  %v\n want %v", c.Name, c.Transform, c.Rect, box, want.BBox)
			continue
		}
		boxes++
	}
	t.Logf("exact matrices=%d and boxes=%d of %d lists; %d more lists depend on the C library, %d of them exact",
		matrices, boxes, len(cases.Cases)-libm, libm, libmExact)
}
