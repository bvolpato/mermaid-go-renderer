package dayjs

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"math"
	"os"
	"strconv"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// fixedNow is the clock of the generator: 2026-01-15T12:00:00Z.
var fixedNow = time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

type durationResult struct {
	MS  string
	As  []string
	Get []string
	ISO string
	Fmt []string
}

type golden struct {
	TZ     string
	Now    string
	Groups struct {
		Format []struct {
			T, F, S string
			Err     bool
		}
		Parse []struct {
			I, F   string
			Strict bool
			T, Y   string
			Err    bool
		}
		FromString []struct{ I, T string }
		Ops        []struct {
			Op, T, T2, U, N string
			Fl              bool
			R               json.RawMessage
		}
		Info []struct {
			T     string
			R     []string
			Valid bool
		}
		Walk []struct {
			T, N, U string
			R       []string
		}
		Durations []struct {
			Kind, MS, V, U, S string
			Keys              [][2]string
			R                 durationResult
			Add               *struct {
				V, U string
				Sub  bool
				R    durationResult
			}
			Apply *struct {
				T   string
				Sub bool
				R   string
			}
		}
	}
}

func loadGolden(t *testing.T, path string) *golden {
	t.Helper()
	f, err := os.Open(path)
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
	var g golden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return &g
}

func bits(t *testing.T, s string) float64 {
	t.Helper()
	u, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return math.Float64frombits(u)
}

// same compares two numbers by their bits. All NaN values are equal.
func same(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return math.Float64bits(a) == math.Float64bits(b)
}

// The units of the lists "as" and "get" of a duration result, in the order
// of the generator.
var durationUnits = []string{"milliseconds", "seconds", "minutes", "hours", "days", "weeks", "months", "years", "millisecond", "second", "minute", "hour", "day", "week", "month", "year",
	"ms", "s", "m", "h", "d", "w", "M", "y", "Q", "foo"}

// TestAgainstDayjs compares the port with values recorded from dayjs by
// tools/reference/gen-dayjs.mjs, in UTC and in America/New_York. All
// numbers are compared by bits.
func TestAgainstDayjs(t *testing.T) {
	for _, file := range []string{"testdata/dayjs_utc.json.gz", "testdata/dayjs_new_york.json.gz"} {
		g := loadGolden(t, file)
		loc, err := time.LoadLocation(g.TZ)
		if err != nil {
			t.Fatal(err)
		}
		f := New(loc, fixedNow)
		t.Run(g.TZ, func(t *testing.T) {
			gr := &g.Groups
			if n := len(gr.Format) + len(gr.Parse) + len(gr.FromString) + len(gr.Ops) + len(gr.Info) + len(gr.Walk) + len(gr.Durations); n < 25000 {
				t.Fatalf("only %d cases", n)
			}
			if got, want := f.Now().ValueOf(), bits(t, g.Now); !same(got, want) {
				t.Errorf("dayjs() = %v, want %v", got, want)
			}
			fails := 0
			failf := func(format string, args ...any) {
				fails++
				if fails <= 40 {
					t.Errorf(format, args...)
				}
			}

			for _, c := range gr.Format {
				tv := bits(t, c.T)
				got, err := f.FromTime(tv).Format(c.F)
				if (err != nil) != c.Err || (err == nil && got != c.S) {
					failf("dayjs(%v).format(%q) = %q, %v; want %q, error %v", tv, c.F, got, err, c.S, c.Err)
				}
			}

			for _, c := range gr.Parse {
				d, err := f.Parse(c.I, c.F, c.Strict)
				if (err != nil) != c.Err {
					failf("dayjs(%q, %q, %v): error %v, want error %v", c.I, c.F, c.Strict, err, c.Err)
					continue
				}
				if err != nil {
					continue
				}
				if want := bits(t, c.T); !same(d.ValueOf(), want) {
					failf("dayjs(%q, %q, %v) = %v, want %v", c.I, c.F, c.Strict, d.ValueOf(), want)
				} else if wantY := bits(t, c.Y); !same(d.Year(), wantY) {
					failf("dayjs(%q, %q, %v).$y = %v, want %v", c.I, c.F, c.Strict, d.Year(), wantY)
				}
			}

			for _, c := range gr.FromString {
				if got, want := f.FromString(c.I).ValueOf(), bits(t, c.T); !same(got, want) {
					failf("dayjs(%q) = %v, want %v", c.I, got, want)
				}
			}

			for _, c := range gr.Ops {
				tv := bits(t, c.T)
				d := f.FromTime(tv)
				var got float64
				switch c.Op {
				case "add":
					got = d.Add(bits(t, c.N), c.U).ValueOf()
				case "subtract":
					got = d.Subtract(bits(t, c.N), c.U).ValueOf()
				case "startOf":
					got = d.StartOf(c.U).ValueOf()
				case "endOf":
					got = d.EndOf(c.U).ValueOf()
				case "set":
					got = d.Set(c.U, bits(t, c.N)).ValueOf()
				case "diff":
					got = d.Diff(f.FromTime(bits(t, c.T2)), c.U, c.Fl)
				case "isoWeekday":
					got = d.SetIsoWeekday(bits(t, c.N)).ValueOf()
				case "isoWeek":
					got = d.SetIsoWeek(bits(t, c.N)).ValueOf()
				case "cmp":
					var want [3]bool
					if err := json.Unmarshal(c.R, &want); err != nil {
						t.Fatal(err)
					}
					o := f.FromTime(bits(t, c.T2))
					if r := [3]bool{d.IsSame(o, c.U), d.IsBefore(o, c.U), d.IsAfter(o, c.U)}; r != want {
						failf("cmp(%v, %v, %q) = %v, want %v", tv, o.ValueOf(), c.U, r, want)
					}
					continue
				default:
					t.Fatalf("unknown op %q", c.Op)
				}
				var r string
				if err := json.Unmarshal(c.R, &r); err != nil {
					t.Fatal(err)
				}
				if want := bits(t, r); !same(got, want) {
					n := math.NaN()
					if c.N != "" {
						n = bits(t, c.N)
					}
					failf("dayjs(%v).%s(%v, %q) = %v, want %v", tv, c.Op, n, c.U, got, want)
				}
			}

			for _, c := range gr.Info {
				tv := bits(t, c.T)
				d := f.FromTime(tv)
				got := []float64{d.Year(), d.Month(), d.Date(), d.Day(), d.Hour(), d.Minute(), d.Second(), d.Millisecond(), d.IsoWeekday(), d.DaysInMonth(), d.UtcOffset(), d.Unix(),
					d.IsoWeek(), d.IsoWeekYear(), d.ValueOf(), d.ToDate()}
				for i, w := range c.R {
					if want := bits(t, w); !same(got[i], want) {
						failf("info %d of %v = %v, want %v", i, tv, got[i], want)
					}
				}
				if d.IsValid() != c.Valid {
					failf("isValid of %v = %v", tv, d.IsValid())
				}
			}

			for _, c := range gr.Walk {
				d := f.FromTime(bits(t, c.T))
				n := bits(t, c.N)
				for j, w := range c.R {
					d = d.Add(n, c.U)
					if want := bits(t, w); !same(d.ValueOf(), want) {
						failf("walk from %v step %d (%v %s) = %v, want %v", bits(t, c.T), j, n, c.U, d.ValueOf(), want)
						break
					}
				}
			}

			checkDuration := func(name string, d *Duration, want *durationResult) {
				if got := d.AsMilliseconds(); !same(got, bits(t, want.MS)) {
					failf("%s: asMilliseconds = %v, want %v", name, got, bits(t, want.MS))
				}
				if got := d.ValueOf(); !same(got, bits(t, want.MS)) {
					failf("%s: valueOf = %v", name, got)
				}
				for i, u := range durationUnits {
					if got, w := d.As(u), bits(t, want.As[i]); !same(got, w) {
						failf("%s: as(%q) = %v, want %v", name, u, got, w)
					}
					if got, w := d.Get(u), bits(t, want.Get[i]); !same(got, w) {
						failf("%s: get(%q) = %v, want %v", name, u, got, w)
					}
				}
				if got := d.ToISOString(); got != want.ISO {
					failf("%s: toISOString = %q, want %q", name, got, want.ISO)
				}
				for i, layout := range []string{"", "Y-M-D H:m:s.SSS", "YYYY YY [x] MM DD HH mm ss YYY"} {
					if got := d.Format(layout); got != want.Fmt[i] {
						failf("%s: format(%q) = %q, want %q", name, layout, got, want.Fmt[i])
					}
				}
			}
			for _, c := range gr.Durations {
				var d *Duration
				var name string
				switch c.Kind {
				case "ms":
					d = f.DurationMs(bits(t, c.MS))
					name = "duration(" + js.Str(bits(t, c.MS)) + ")"
				case "unit":
					d = f.DurationUnit(bits(t, c.V), c.U)
					name = "duration(" + js.Str(bits(t, c.V)) + ", " + c.U + ")"
				case "obj":
					obj := js.NewObj[float64]()
					name = "duration({"
					for _, kv := range c.Keys {
						obj.Set(kv[0], bits(t, kv[1]))
						name += kv[0] + ": " + js.Str(bits(t, kv[1])) + ", "
					}
					name += "})"
					d = f.DurationOf(obj)
				case "iso":
					d = f.DurationISO(c.S)
					name = "duration(" + c.S + ")"
				default:
					t.Fatalf("unknown duration kind %q", c.Kind)
				}
				checkDuration(name, d, &c.R)
				if c.Add != nil {
					var r *Duration
					if c.Add.Sub {
						r = d.Subtract(bits(t, c.Add.V), c.Add.U)
					} else {
						r = d.Add(bits(t, c.Add.V), c.Add.U)
					}
					checkDuration(name+".add", r, &c.Add.R)
				}
				if c.Apply != nil {
					base := f.FromTime(bits(t, c.Apply.T))
					var r *Dayjs
					if c.Apply.Sub {
						r = base.SubtractDuration(d)
					} else {
						r = base.AddDuration(d)
					}
					if want := bits(t, c.Apply.R); !same(r.ValueOf(), want) {
						failf("dayjs(%v).add(%s) = %v, want %v", base.ValueOf(), name, r.ValueOf(), want)
					}
				}
			}
			if fails > 0 {
				t.Errorf("%d checks failed", fails)
			}
		})
	}
}
