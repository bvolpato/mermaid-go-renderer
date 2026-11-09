package axis

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/format"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/scale"
	d3time "github.com/bvolpato/mermaid-go-renderer/internal/d3/time"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/timefmt"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

type scaleCase struct {
	Type         string
	Domain       []json.RawMessage
	Range        []float64
	RangeRound   bool
	Round        bool
	Align        float64
	PaddingInner float64
	PaddingOuter float64
	Padding      float64
}

type axisCase struct {
	Orient        string
	Scale         scaleCase
	Ticks         *[]json.RawMessage
	TickValues    *[]float64
	TickFormat    *struct{ Kind, Spec string }
	TickSize      *float64
	TickSizeInner *float64
	TickSizeOuter *float64
	TickPadding   *float64
	Offset        *float64
	Steps         []struct{ Scale scaleCase }
}

type axisGolden struct {
	DevicePixelRatio float64
	Timezone         string
	Cases            []axisCase
	Results          []struct {
		HTML  []string
		Error string
	}
}

func numbers(t *testing.T, raw []json.RawMessage) []float64 {
	t.Helper()
	out := make([]float64, len(raw))
	for i, r := range raw {
		if err := json.Unmarshal(r, &out[i]); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func stringsOf(t *testing.T, raw []json.RawMessage) []string {
	t.Helper()
	out := make([]string, len(raw))
	for i, r := range raw {
		if err := json.Unmarshal(r, &out[i]); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func numericScale(t *testing.T, s scaleCase) Scale[float64] {
	t.Helper()
	domain := numbers(t, s.Domain)
	if s.Type == "linear" {
		l := scale.NewLinear().Domain(domain...)
		if s.RangeRound {
			return LinearScale(l.RangeRound(s.Range...))
		}
		return LinearScale(l.Range(s.Range...))
	}
	ts := scale.NewTime(time.UTC).Domain(domain...)
	if s.RangeRound {
		return TimeScale(ts.RangeRound(s.Range...))
	}
	return TimeScale(ts.Range(s.Range...))
}

func ordinalScale(t *testing.T, s scaleCase) Scale[string] {
	t.Helper()
	domain := stringsOf(t, s.Domain)
	identity := func(d string) string { return d }
	if s.Type == "band" {
		b := scale.NewBand[string]().Domain(domain...).Round(s.Round).PaddingInner(s.PaddingInner).PaddingOuter(s.PaddingOuter).Align(s.Align)
		return BandScale(b.Range(s.Range[0], s.Range[1]), identity)
	}
	p := scale.NewPoint[string]().Domain(domain...).Round(s.Round).Padding(s.Padding).Align(s.Align)
	return PointScale(p.Range(s.Range[0], s.Range[1]), identity, s.Round)
}

func newAxisOf[D any](orient string, s Scale[D]) *Axis[D] {
	switch orient {
	case "top":
		return Top(s)
	case "right":
		return Right(s)
	case "bottom":
		return Bottom(s)
	}
	return Left(s)
}

func configure[D any](t *testing.T, a *Axis[D], c *axisCase) {
	t.Helper()
	if c.Ticks != nil {
		intervals := d3time.Local(time.UTC)
		byName := map[string]*d3time.Interval{
			"millisecond": intervals.Millisecond, "second": intervals.Second, "minute": intervals.Minute, "hour": intervals.Hour,
			"day": intervals.Day, "week": intervals.Week, "monday": intervals.Monday, "thursday": intervals.Thursday,
			"month": intervals.Month, "year": intervals.Year,
		}
		var args []any
		for _, raw := range *c.Ticks {
			var v any
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatal(err)
			}
			if m, ok := v.(map[string]any); ok {
				args = append(args, byName[m["interval"].(string)].Every(m["every"].(float64)))
			} else {
				args = append(args, v)
			}
		}
		a.Ticks(args...)
	}
	if c.TickSize != nil {
		a.TickSize(*c.TickSize)
	}
	if c.TickSizeInner != nil {
		a.TickSizeInner(*c.TickSizeInner)
	}
	if c.TickSizeOuter != nil {
		a.TickSizeOuter(*c.TickSizeOuter)
	}
	if c.TickPadding != nil {
		a.TickPadding(*c.TickPadding)
	}
	if c.Offset != nil {
		a.Offset(*c.Offset)
	}
}

// run renders one case and returns the outerHTML of the container after
// each call of the axis.
func run(t *testing.T, c *axisCase) []string {
	t.Helper()
	doc := dom.NewDocument()
	g := doc.Body.Append("svg").Append("g")
	var html []string
	if c.Scale.Type == "linear" || c.Scale.Type == "time" {
		a := newAxisOf(c.Orient, numericScale(t, c.Scale))
		configure(t, a, c)
		if c.TickValues != nil {
			a.TickValues(append([]float64{}, *c.TickValues...))
		}
		if f := c.TickFormat; f != nil {
			switch f.Kind {
			case "time":
				a.TickFormat(timefmt.Format(time.UTC, f.Spec))
			case "number":
				nf, err := format.Format(f.Spec)
				if err != nil {
					t.Fatal(err)
				}
				a.TickFormat(nf)
			default:
				a.TickFormat(func(d float64) string { return "<" + js.Str(d) + ">" })
			}
		}
		if err := a.Call(g); err != nil {
			t.Fatal(err)
		}
		html = append(html, g.OuterHTML())
		for _, step := range c.Steps {
			a.Scale(numericScale(t, step.Scale))
			if err := a.Call(g); err != nil {
				t.Fatal(err)
			}
			html = append(html, g.OuterHTML())
		}
		return html
	}
	a := newAxisOf(c.Orient, ordinalScale(t, c.Scale))
	configure(t, a, c)
	if c.TickFormat != nil {
		a.TickFormat(func(d string) string { return "<" + d + ">" })
	}
	if err := a.Call(g); err != nil {
		t.Fatal(err)
	}
	html = append(html, g.OuterHTML())
	for _, step := range c.Steps {
		a.Scale(ordinalScale(t, step.Scale))
		if err := a.Call(g); err != nil {
			t.Fatal(err)
		}
		html = append(html, g.OuterHTML())
	}
	return html
}

// TestAgainstD3AxisInChromium compares the port with the outerHTML that the
// real d3-axis produced in the browser oracle (tools/reference/
// gen-d3-axis-cases.mjs and d3-axis-page.js). The comparison is exact.
func TestAgainstD3AxisInChromium(t *testing.T) {
	f, err := os.Open("testdata/axis_chromium.json.gz")
	if err != nil {
		t.Fatal(err)
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
	var g axisGolden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if g.DevicePixelRatio != 1 || g.Timezone != "UTC" {
		t.Fatalf("the oracle ran with devicePixelRatio %v in zone %q", g.DevicePixelRatio, g.Timezone)
	}
	if len(g.Cases) < 200 || len(g.Cases) != len(g.Results) {
		t.Fatalf("%d cases, %d results", len(g.Cases), len(g.Results))
	}
	steps, bad := 0, 0
	for i := range g.Cases {
		c := &g.Cases[i]
		want := g.Results[i]
		if want.Error != "" {
			t.Fatalf("case %d: the oracle failed: %s", i, want.Error)
		}
		got := run(t, c)
		if len(got) != len(want.HTML) {
			t.Fatalf("case %d: %d calls, want %d", i, len(got), len(want.HTML))
		}
		for j := range got {
			steps++
			if got[j] != want.HTML[j] {
				bad++
				if bad <= 5 {
					t.Errorf("case %d (%s, %s), call %d:\n got  %s\n want %s", i, c.Scale.Type, c.Orient, j, got[j], want.HTML[j])
				}
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d outputs differ", bad, steps)
	}
	t.Logf("%d cases, %d axis calls", len(g.Cases), steps)
}
