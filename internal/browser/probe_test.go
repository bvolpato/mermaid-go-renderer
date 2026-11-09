package browser

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

type probeQuery struct {
	Sel string          `json:"sel"`
	Op  string          `json:"op"`
	Arg json.RawMessage `json:"arg"`
	All bool            `json:"all"`
}

type probeCase struct {
	Name    string       `json:"name"`
	HTML    string       `json:"html"`
	Queries []probeQuery `json:"queries"`
}

type probeResult struct {
	Name    string            `json:"name"`
	Results []json.RawMessage `json:"results"`
}

func runQuery(el *dom.Node, q probeQuery) any {
	if el == nil {
		return nil
	}
	switch q.Op {
	case "bbox":
		b := el.BBox()
		return []float64{b.X, b.Y, b.Width, b.Height}
	case "rect":
		b := el.BoundingClientRect()
		return []float64{b.X, b.Y, b.Width, b.Height}
	case "textlen":
		return el.ComputedTextLength()
	case "len":
		return el.TotalLength()
	case "pt":
		var d float64
		_ = json.Unmarshal(q.Arg, &d)
		p := el.PointAtLength(d)
		return []float64{p.X, p.Y}
	case "style":
		var prop string
		_ = json.Unmarshal(q.Arg, &prop)
		return el.ComputedStyle(prop)
	case "outer":
		return el.OuterHTML()
	case "html":
		return el.InnerHTML()
	case "setstyle":
		var pairs [][2]string
		_ = json.Unmarshal(q.Arg, &pairs)
		for _, p := range pairs {
			el.Style(p[0], p[1])
		}
		v, ok := el.GetAttr("style")
		if !ok {
			return nil
		}
		return v
	}
	return nil
}

func closeEnough(a, b, tol float64) bool {
	if a == b {
		return true
	}
	return math.Abs(a-b) <= tol
}

// compare returns "exact", "close" (within tol) or "diff".
func compare(got any, want json.RawMessage, tol float64) string {
	gotJSON, _ := json.Marshal(got)
	var g, w any
	_ = json.Unmarshal(gotJSON, &g)
	_ = json.Unmarshal(want, &w)
	exact := true
	var cmp func(a, b any) bool
	cmp = func(a, b any) bool {
		switch av := a.(type) {
		case float64:
			bv, ok := b.(float64)
			if !ok {
				return false
			}
			if av != bv {
				exact = false
			}
			return closeEnough(av, bv, tol)
		case []any:
			bv, ok := b.([]any)
			if !ok || len(av) != len(bv) {
				return false
			}
			for i := range av {
				if !cmp(av[i], bv[i]) {
					return false
				}
			}
			return true
		default:
			return fmt.Sprint(a) == fmt.Sprint(b)
		}
	}
	if !cmp(g, w) {
		return "diff"
	}
	if exact {
		return "exact"
	}
	return "close"
}

var caseKind = regexp.MustCompile(`^[a-z]+(-[a-z]+)?`)

// TestAgainstChromium runs the DOM probe cases through the engine and
// compares with what Chromium returned. It is skipped without probe data.
func TestAgainstChromium(t *testing.T) {
	casesData, err := os.ReadFile("../../.parity/probe/dom-cases.json")
	if err != nil {
		t.Skip("no probe cases:", err)
	}
	resultsData, err := os.ReadFile("../../.parity/probe/dom-results.json")
	if err != nil {
		t.Skip("no probe results:", err)
	}
	var cases struct{ Cases []probeCase }
	var results []probeResult
	if err := json.Unmarshal(casesData, &cases); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(resultsData, &results); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("MMDG_TIGHT") != "" {
		font.TightGlyphBounds = true
	}
	filter := os.Getenv("MMDG_PROBE_FILTER")
	type stat struct{ exact, close, diff int }
	stats := map[string]*stat{}
	var fails []string
	for i, c := range cases.Cases {
		if filter != "" && !strings.Contains(c.Name, filter) {
			continue
		}
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		for qi, q := range c.Queries {
			kind := caseKind.FindString(c.Name) + "/" + q.Op
			st := stats[kind]
			if st == nil {
				st = &stat{}
				stats[kind] = st
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
			switch compare(got, want, 0.01) {
			case "exact":
				st.exact++
			case "close":
				st.close++
			default:
				st.diff++
				if len(fails) < 400 {
					gj, _ := json.Marshal(got)
					html := c.HTML
					if j := strings.Index(html, "</style>"); j >= 0 {
						html = "<svg…>" + html[j+8:]
					}
					if len(html) > 260 {
						html = html[:260] + "…"
					}
					var compact any
					_ = json.Unmarshal(want, &compact)
					wj, _ := json.Marshal(compact)
					if sel := os.Getenv("MMDG_PROBE_SEL"); sel != "" && sel != q.Sel {
						continue
					}
					fails = append(fails, fmt.Sprintf("%s %s(%s)\n      got  %s\n      want %s\n      %s", c.Name, q.Op, q.Sel, gj, wj, html))
				}
			}
		}
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var exact, close, diff int
	for _, k := range keys {
		s := stats[k]
		t.Logf("%-22s exact=%4d close=%4d diff=%4d", k, s.exact, s.close, s.diff)
		exact, close, diff = exact+s.exact, close+s.close, diff+s.diff
	}
	t.Logf("TOTAL exact=%d close(<=0.01)=%d diff=%d", exact, close, diff)
	if os.Getenv("MMDG_PROBE_VERBOSE") != "" {
		n := 60
		if len(fails) < n {
			n = len(fails)
		}
		t.Log("failures:\n" + strings.Join(fails[:n], "\n"))
	}
}
