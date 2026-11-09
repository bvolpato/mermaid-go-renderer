package font

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
)

type chromeTextCase struct {
	Text   string  `json:"text"`
	Family string  `json:"family"`
	Size   float64 `json:"size"`
	Weight string  `json:"weight"`
	Style  string  `json:"style"`
	HTMLW  float64 `json:"htmlW"`
	HTMLH  float64 `json:"htmlH"`
	SVGLen float64 `json:"svgLen"`
	BX     float64 `json:"bx"`
	BY     float64 `json:"by"`
	BW     float64 `json:"bw"`
	BH     float64 `json:"bh"`
}

// TestCalibrateAgainstChromium compares text widths and vertical metrics
// with measurements taken in Chromium on this machine. The data file is
// produced by .parity/probe/text.js. The test is skipped when it is absent.
func TestCalibrateAgainstChromium(t *testing.T) {
	path := os.Getenv("MMDG_TEXT_PROBE")
	if path == "" {
		path = "../../.parity/probe/text-results.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("no Chromium text probe data:", err)
	}
	var cases []chromeTextCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	lib := System()
	type stat struct{ total, wOK, hOK, bboxOK int }
	stats := map[string]*stat{}
	var fails []string
	for _, c := range cases {
		weight := 400
		if c.Weight == "bold" {
			weight = 700
		}
		chain := lib.Resolve(css.SplitFontFamily(c.Family), weight, c.Style == "italic")
		runs := chain.ShapeText([]rune(c.Text), c.Size)
		w := SnapCeil(TextWidth(runs))
		m := chain.Primary().Metrics(c.Size)
		key := c.Family
		if !isASCII(c.Text) {
			key = "non-ascii"
		}
		st := stats[key]
		if st == nil {
			st = &stat{}
			stats[key] = st
		}
		st.total++
		wOK := w == c.HTMLW && w == c.SVGLen
		if wOK {
			st.wOK++
		}
		hOK := m.LineSpacing == c.HTMLH
		if hOK {
			st.hOK++
		}
		bOK := -m.Ascent == c.BY && m.Ascent+m.Descent == c.BH
		if bOK {
			st.bboxOK++
		}
		verbose := os.Getenv("MMDG_TEXT_PROBE_VERBOSE")
		if (verbose == "width" && !wOK && len(fails) < 400) || (verbose != "width" && !(wOK && hOK && bOK) && len(fails) < 60) {
			fails = append(fails, fmt.Sprintf("%q %s %vpx %s %s: width go=%v html=%v svg=%v | lineH go=%v html=%v | bbox go=(%v,%v) chrome=(%v,%v) face=%s",
				c.Text, c.Family, c.Size, c.Weight, c.Style, w, c.HTMLW, c.SVGLen, m.LineSpacing, c.HTMLH, -m.Ascent, m.Ascent+m.Descent, c.BY, c.BH, chain.Primary().Family))
		}
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var total, okAll int
	for _, k := range keys {
		s := stats[k]
		t.Logf("%-48s n=%4d width=%4d lineHeight=%4d bbox=%4d", k, s.total, s.wOK, s.hOK, s.bboxOK)
		total += s.total
		okAll += s.wOK
	}
	t.Logf("width exact: %d / %d", okAll, total)
	if os.Getenv("MMDG_TEXT_PROBE_VERBOSE") != "" {
		t.Log("first failures:\n" + strings.Join(fails, "\n"))
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
