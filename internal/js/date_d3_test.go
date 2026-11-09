package js

import (
	"encoding/base64"
	"encoding/binary"
	"math"
	"testing"
	"time"
	_ "time/tzdata"
)

func decodeFloatList(t *testing.T, s string) []float64 {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	vals := make([]float64, len(raw)/8)
	for i := range vals {
		vals[i] = math.Float64frombits(binary.LittleEndian.Uint64(raw[i*8:]))
	}
	return vals
}

type dateGolden struct {
	TZ     string
	Groups struct {
		Get  []struct{ T, F string }
		Clip []struct{ V, T string }
		Ctor []struct{ A, T string }
		UTC  []struct{ A, T string }
		Set  []struct{ T, M, A, R string }
	}
}

// TestDateAgainstV8 compares the Date port with values recorded from V8 by
// tools/reference/gen-d3-jsdate.mjs, in UTC and in America/New_York.
func TestDateAgainstV8(t *testing.T) {
	for _, file := range []string{"testdata/d3_date_utc.json.gz", "testdata/d3_date_new_york.json.gz"} {
		var g dateGolden
		loadGzipJSON(t, file, &g)
		loc, err := time.LoadLocation(g.TZ)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(g.TZ, func(t *testing.T) {
			n := len(g.Groups.Get) + len(g.Groups.Clip) + len(g.Groups.Ctor) + len(g.Groups.UTC) + len(g.Groups.Set)
			if n < 15000 {
				t.Fatalf("only %d cases", n)
			}
			for _, c := range g.Groups.Get {
				d := NewDate(loc, fromBits(t, c.T))
				got := []float64{d.GetFullYear(), d.GetMonth(), d.GetDate(), d.GetDay(), d.GetHours(), d.GetMinutes(), d.GetSeconds(), d.GetMilliseconds(),
					d.GetTimezoneOffset(), d.GetUTCFullYear(), d.GetUTCMonth(), d.GetUTCDate(), d.GetUTCDay(), d.GetUTCHours(), d.GetUTCMinutes(),
					d.GetUTCSeconds(), d.GetUTCMilliseconds()}
				want := decodeFloatList(t, c.F)
				for i := range want {
					if !sameFloat(got[i], want[i]) {
						t.Errorf("fields of %v: got %v, want %v", d.GetTime(), got, want)
						break
					}
				}
			}
			for _, c := range g.Groups.Clip {
				v := fromBits(t, c.V)
				if got, want := NewDate(loc, v).GetTime(), fromBits(t, c.T); !sameFloat(got, want) {
					t.Errorf("new Date(%v) = %v, want %v", v, got, want)
				}
			}
			for _, c := range g.Groups.Ctor {
				a := decodeFloatList(t, c.A)
				if got, want := NewDateLocal(loc, a[0], a[1], a[2:]...).GetTime(), fromBits(t, c.T); !sameFloat(got, want) {
					t.Errorf("new Date(%v) = %v, want %v", a, got, want)
				}
			}
			for _, c := range g.Groups.UTC {
				a := decodeFloatList(t, c.A)
				if got, want := DateUTC(a[0], a[1:]...), fromBits(t, c.T); !sameFloat(got, want) {
					t.Errorf("Date.UTC(%v) = %v, want %v", a, got, want)
				}
			}
			for _, c := range g.Groups.Set {
				d := NewDate(loc, fromBits(t, c.T))
				a := decodeFloatList(t, c.A)
				var got float64
				switch c.M {
				case "setFullYear":
					got = d.SetFullYear(a[0], a[1:]...)
				case "setMonth":
					got = d.SetMonth(a[0], a[1:]...)
				case "setDate":
					got = d.SetDate(a[0])
				case "setHours":
					got = d.SetHours(a[0], a[1:]...)
				case "setMinutes":
					got = d.SetMinutes(a[0], a[1:]...)
				case "setSeconds":
					got = d.SetSeconds(a[0], a[1:]...)
				case "setMilliseconds":
					got = d.SetMilliseconds(a[0])
				case "setTime":
					got = d.SetTime(a[0])
				case "setUTCFullYear":
					got = d.SetUTCFullYear(a[0], a[1:]...)
				case "setUTCMonth":
					got = d.SetUTCMonth(a[0], a[1:]...)
				case "setUTCDate":
					got = d.SetUTCDate(a[0])
				case "setUTCHours":
					got = d.SetUTCHours(a[0], a[1:]...)
				case "setUTCMinutes":
					got = d.SetUTCMinutes(a[0], a[1:]...)
				case "setUTCSeconds":
					got = d.SetUTCSeconds(a[0], a[1:]...)
				case "setUTCMilliseconds":
					got = d.SetUTCMilliseconds(a[0])
				default:
					t.Fatalf("unknown setter %s", c.M)
				}
				if want := fromBits(t, c.R); !sameFloat(got, want) || !sameFloat(d.GetTime(), want) {
					t.Errorf("new Date(%v).%s(%v) = %v, want %v", fromBits(t, c.T), c.M, a, got, want)
				}
			}
		})
	}
}
