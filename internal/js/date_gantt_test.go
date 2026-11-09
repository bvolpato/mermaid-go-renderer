package js

import (
	"testing"
	"time"
	_ "time/tzdata"
)

type dateParseGolden struct {
	TZ     string
	Valid  int
	Groups struct {
		Parse []struct{ S, T string }
	}
}

// TestDateParseAgainstV8 compares DateParse with Date.parse of V8, recorded
// by tools/reference/gen-gantt-jsdate.mjs in UTC and in America/New_York.
func TestDateParseAgainstV8(t *testing.T) {
	for _, file := range []string{"testdata/gantt_dateparse_utc.json.gz", "testdata/gantt_dateparse_new_york.json.gz"} {
		var g dateParseGolden
		loadGzipJSON(t, file, &g)
		loc, err := time.LoadLocation(g.TZ)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(g.TZ, func(t *testing.T) {
			if len(g.Groups.Parse) < 20000 || g.Valid < 8000 {
				t.Fatalf("only %d cases, %d valid", len(g.Groups.Parse), g.Valid)
			}
			bad := 0
			for _, c := range g.Groups.Parse {
				want := fromBits(t, c.T)
				got := DateParse(loc, c.S)
				if !sameFloat(got, want) {
					bad++
					if bad <= 20 {
						t.Errorf("Date.parse(%q) = %v, want %v", c.S, got, want)
					}
				}
				if d := NewDateString(loc, c.S); !sameFloat(d.GetTime(), want) {
					t.Errorf("new Date(%q) = %v, want %v", c.S, d.GetTime(), want)
				}
			}
			if bad > 0 {
				t.Errorf("%d of %d cases differ", bad, len(g.Groups.Parse))
			}
		})
	}
}
