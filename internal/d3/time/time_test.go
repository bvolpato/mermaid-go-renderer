package time

import (
	"fmt"
	"math"
	"testing"
	stdtime "time"
	_ "time/tzdata"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

type golden struct {
	TZ     string
	Math   d3test.Math
	Groups struct {
		Intervals []struct {
			Fam, Iv             string
			T, St, T2, Rs, Stop d3test.F
			R, Range            d3test.FS
			K                   d3test.F
			EvSkip              bool
			Ev                  *d3test.FS
			EvSt, EvStop        d3test.F
			EvRange             d3test.FS
			EvHasCount          bool
		}
		Filters []struct {
			Fam, Iv     string
			Mod         float64
			T, St, Stop d3test.F
			R, Range    d3test.FS
		}
		Ticks []struct {
			A, Time, UTC d3test.FS
			Probe        d3test.F
			Ti, Ui       *d3test.FS
		}
		IntervalTicks []struct {
			Fam, Iv string
			K       float64
			A, R    d3test.FS
		}
	}
}

// mathExceptions lists, for each build mode (see d3test.Mode), the number of
// golden cases that differ from Node.js only because a Math function returns
// another last bit. The test proves each one: the case passes when it runs
// again with the results that Node.js returned for the same calls. Measured
// with go1.27.3.
//
// There are none.
var mathExceptions = map[string]map[string]int{
	"arm64":       {},
	"arm64/nofma": {},
	"amd64":       {},
}

func byName(s *Intervals, name string) *Interval {
	switch name {
	case "millisecond":
		return s.Millisecond
	case "second":
		return s.Second
	case "minute":
		return s.Minute
	case "hour":
		return s.Hour
	case "day":
		return s.Day
	case "unixDay":
		return s.UnixDay
	case "sunday":
		return s.Sunday
	case "monday":
		return s.Monday
	case "tuesday":
		return s.Tuesday
	case "wednesday":
		return s.Wednesday
	case "thursday":
		return s.Thursday
	case "friday":
		return s.Friday
	case "saturday":
		return s.Saturday
	case "month":
		return s.Month
	case "year":
		return s.Year
	}
	return nil
}

func diff(what string, got, want []float64) string {
	if d3test.SameSlice(got, want) {
		return ""
	}
	return fmt.Sprintf("%s: got %v, want %v", what, got, want)
}

// TestAgainstD3 compares with values that tools/reference/gen-d3-time.mjs
// recorded from d3-time, in UTC and in America/New_York.
func TestAgainstD3(t *testing.T) {
	for _, file := range []string{"testdata/time_utc.json.gz", "testdata/time_new_york.json.gz"} {
		var g golden
		d3test.Load(t, file, &g)
		loc, err := stdtime.LoadLocation(g.TZ)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(g.TZ, func(t *testing.T) {
			c := d3test.NewChecker(t, g.Math)
			families := map[string]*Intervals{"time": Local(loc), "utc": UTC()}

			for i, tc := range g.Groups.Intervals {
				iv := byName(families[tc.Fam], tc.Iv)
				name := fmt.Sprintf("#%d %s %s t=%v", i, tc.Fam, tc.Iv, tc.T.V())
				c.Check(tc.Fam+" interval", name, func() string {
					x := tc.T.V()
					got := []float64{iv.Floor(x), iv.Ceil(x), iv.Round(x), iv.Offset(x, tc.St.V()), iv.Count(x, tc.T2.V())}
					if s := diff("floor/ceil/round/offset/count", got, tc.R); s != "" {
						return s
					}
					return diff("range", iv.Range(x, tc.Stop.V(), tc.Rs.V()), tc.Range)
				})
				if tc.EvSkip {
					continue
				}
				c.Check(tc.Fam+" interval.every", name+fmt.Sprintf(" k=%v", tc.K.V()), func() string {
					ev := iv.Every(tc.K.V())
					if tc.Ev == nil {
						if ev != nil {
							return "got an interval, want null"
						}
						return ""
					}
					if ev == nil {
						return "got null"
					}
					x := tc.T.V()
					got := []float64{ev.Floor(x), ev.Ceil(x), ev.Round(x), ev.Offset(x, tc.EvSt.V())}
					if s := diff("floor/ceil/round/offset", got, *tc.Ev); s != "" {
						return s
					}
					if ev.HasCount() != tc.EvHasCount {
						return fmt.Sprintf("HasCount() = %v, want %v", ev.HasCount(), tc.EvHasCount)
					}
					return diff("range", ev.Range(x, tc.EvStop.V(), 1), tc.EvRange)
				})
			}

			for i, tc := range g.Groups.Filters {
				iv := byName(families[tc.Fam], tc.Iv)
				c.Check("interval.filter", fmt.Sprintf("#%d %s %s t=%v", i, tc.Fam, tc.Iv, tc.T.V()), func() string {
					field := func(d *js.Date) float64 {
						switch {
						case tc.Iv == "month" && tc.Fam == "time":
							return d.GetMonth()
						case tc.Iv == "month":
							return d.GetUTCMonth()
						case tc.Fam == "time":
							return d.GetDate()
						}
						return d.GetUTCDate()
					}
					f := iv.Filter(func(d *js.Date) bool { return math.Mod(field(d), tc.Mod) == 0 })
					x := tc.T.V()
					got := []float64{f.Floor(x), f.Ceil(x), f.Offset(x, tc.St.V())}
					if s := diff("floor/ceil/offset", got, tc.R); s != "" {
						return s
					}
					return diff("range", f.Range(x, tc.Stop.V(), 1), tc.Range)
				})
			}

			for i, tc := range g.Groups.Ticks {
				start, stop, count := tc.A[0], tc.A[1], tc.A[2]
				c.Check("timeTicks/utcTicks", fmt.Sprintf("#%d (%v, %v, %v)", i, start, stop, count), func() string {
					for _, p := range []struct {
						name   string
						s      *Intervals
						ticks  []float64
						probed *d3test.FS
					}{{"timeTicks", families["time"], tc.Time, tc.Ti}, {"utcTicks", families["utc"], tc.UTC, tc.Ui}} {
						if s := diff(p.name, p.s.Ticks(start, stop, count), p.ticks); s != "" {
							return s
						}
						iv := p.s.TickInterval(start, stop, count)
						if p.probed == nil {
							if iv != nil {
								return p.name + ": tickInterval: got an interval, want null"
							}
							continue
						}
						if iv == nil {
							return p.name + ": tickInterval: got null"
						}
						got := []float64{iv.Floor(tc.Probe.V()), iv.Offset(tc.Probe.V(), 1)}
						if s := diff(p.name+" tickInterval", got, *p.probed); s != "" {
							return s
						}
					}
					return ""
				})
			}

			for i, tc := range g.Groups.IntervalTicks {
				s := families[tc.Fam]
				c.Check("ticks(interval)", fmt.Sprintf("#%d %s %s.every(%v) %v", i, tc.Fam, tc.Iv, tc.K, tc.A), func() string {
					return diff("ticks", s.TicksInterval(tc.A[0], tc.A[1], byName(s, tc.Iv).Every(tc.K)), tc.R)
				})
			}

			c.Report()
			c.CheckExceptions(mathExceptions)
		})
	}
}

// TestFilterStopsAtInvalidDate checks the one place where the port does not
// follow d3: d3 loops forever when no date passes the test of a filter.
func TestFilterStopsAtInvalidDate(t *testing.T) {
	never := UTC().Year.Filter(func(*js.Date) bool { return false })
	if got := never.Floor(0); !math.IsNaN(got) {
		t.Errorf("Floor = %v, want an invalid date", got)
	}
}
