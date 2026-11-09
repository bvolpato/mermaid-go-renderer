package timefmt

import (
	"fmt"
	"testing"
	stdtime "time"
	_ "time/tzdata"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
)

type golden struct {
	TZ     string
	Math   d3test.Math
	Groups struct {
		Formats []struct {
			S         string
			T         d3test.FS
			Time, UTC []string
		}
		Parses []struct {
			S, Text   string
			Time, UTC *d3test.F
		}
		Locales []struct {
			S           string
			T           d3test.F
			Time, UTC   string
			Text        string
			Ptime, Putc *d3test.F
		}
	}
}

// mathExceptions lists, for each build mode (see d3test.Mode), the number of
// golden cases that differ from Node.js only because a Math function returns
// another last bit. The test proves each one: the case passes when it runs
// again with the results that Node.js returned for the same calls. Measured
// with go1.27.3.
//
// There are none: d3-time-format has no such call.
var mathExceptions = map[string]map[string]int{
	"arm64":       {},
	"arm64/nofma": {},
	"amd64":       {},
}

func checkParse(what string, got float64, ok bool, want *d3test.F) string {
	if want == nil {
		if ok {
			return fmt.Sprintf("%s: got %v, want null", what, got)
		}
		return ""
	}
	if !ok {
		return fmt.Sprintf("%s: got null, want %v", what, want.V())
	}
	if !d3test.Same(got, want.V()) {
		return fmt.Sprintf("%s: got %v, want %v", what, got, want.V())
	}
	return ""
}

// TestAgainstD3 compares with values that tools/reference/gen-d3-timefmt.mjs
// recorded from d3-time-format, in UTC and in America/New_York.
func TestAgainstD3(t *testing.T) {
	fr := NewLocale(LocaleDefinition{
		DateTime: "%A %e %B %Y à %X", Date: "%d/%m/%Y", Time: "%H:%M:%S", Periods: [2]string{"AM", "PM"},
		Days:        [7]string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"},
		ShortDays:   [7]string{"dim.", "lun.", "mar.", "mer.", "jeu.", "ven.", "sam."},
		Months:      [12]string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"},
		ShortMonths: [12]string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."},
	})
	for _, file := range []string{"testdata/timefmt_utc.json.gz", "testdata/timefmt_new_york.json.gz"} {
		var g golden
		d3test.Load(t, file, &g)
		loc, err := stdtime.LoadLocation(g.TZ)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(g.TZ, func(t *testing.T) {
			c := d3test.NewChecker(t, g.Math)

			for i, tc := range g.Groups.Formats {
				c.Check("timeFormat/utcFormat", fmt.Sprintf("#%d %q %v", i, tc.S, tc.T), func() string {
					f, u := Format(loc, tc.S), UTCFormat(tc.S)
					for j, x := range tc.T {
						if got := f(x); got != tc.Time[j] {
							return fmt.Sprintf("timeFormat(%v): got %q, want %q", x, got, tc.Time[j])
						}
						if got := u(x); got != tc.UTC[j] {
							return fmt.Sprintf("utcFormat(%v): got %q, want %q", x, got, tc.UTC[j])
						}
					}
					return ""
				})
			}

			for i, tc := range g.Groups.Parses {
				c.Check("timeParse/utcParse", fmt.Sprintf("#%d %q %q", i, tc.S, tc.Text), func() string {
					got, ok := Parse(loc, tc.S)(tc.Text)
					if s := checkParse("timeParse", got, ok, tc.Time); s != "" {
						return s
					}
					got, ok = UTCParse(tc.S)(tc.Text)
					return checkParse("utcParse", got, ok, tc.UTC)
				})
			}

			for i, tc := range g.Groups.Locales {
				c.Check("timeFormatLocale", fmt.Sprintf("#%d %q %v %q", i, tc.S, tc.T.V(), tc.Text), func() string {
					if got := fr.Format(loc, tc.S)(tc.T.V()); got != tc.Time {
						return fmt.Sprintf("format: got %q, want %q", got, tc.Time)
					}
					if got := fr.UTCFormat(tc.S)(tc.T.V()); got != tc.UTC {
						return fmt.Sprintf("utcFormat: got %q, want %q", got, tc.UTC)
					}
					got, ok := fr.Parse(loc, tc.S)(tc.Text)
					if s := checkParse("parse", got, ok, tc.Ptime); s != "" {
						return s
					}
					got, ok = fr.UTCParse(tc.S)(tc.Text)
					return checkParse("utcParse", got, ok, tc.Putc)
				})
			}

			c.Report()
			c.CheckExceptions(mathExceptions)
		})
	}
}
