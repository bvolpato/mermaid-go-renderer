// Ported from d3-time 3.1.0, d3-time/src/interval.js and
// d3-time/src/duration.js (ISC).

// Package time is a port of d3-time 3.1.0.
//
// A date is a JavaScript time value: a float64 with the milliseconds since
// the epoch in UTC, or NaN for an invalid date. d3 uses the local time zone
// of the machine for the intervals timeDay, timeMonth and so on. The port
// takes the zone as a *time.Location instead (see Local).
package time

import (
	"math"
	stdtime "time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The durations in milliseconds.
const (
	DurationSecond = 1000
	DurationMinute = DurationSecond * 60
	DurationHour   = DurationMinute * 60
	DurationDay    = DurationHour * 24
	DurationWeek   = DurationDay * 7
	DurationMonth  = DurationDay * 30
	DurationYear   = DurationDay * 365
)

// Interval is a d3 time interval, for example timeDay.
type Interval struct {
	loc     *stdtime.Location
	floori  func(date *js.Date)
	offseti func(date *js.Date, step float64)
	count   func(start, end *js.Date) float64
	field   func(date *js.Date) float64
	// every replaces the generic implementation of interval.every.
	every func(k float64) *Interval
}

// NewInterval is d3.timeInterval(floor, offset, count, field). The count and
// field functions may be nil. The dates that the functions receive are in
// the given zone; nil means UTC.
func NewInterval(loc *stdtime.Location, floor func(date *js.Date), offset func(date *js.Date, step float64),
	count func(start, end *js.Date) float64, field func(date *js.Date) float64) *Interval {
	if loc == nil {
		loc = stdtime.UTC
	}
	return &Interval{loc: loc, floori: floor, offseti: offset, count: count, field: field}
}

// Floor is interval(date) and interval.floor(date).
func (iv *Interval) Floor(date float64) float64 {
	d := js.NewDate(iv.loc, date)
	iv.floori(d)
	return d.GetTime()
}

// Ceil is interval.ceil(date).
func (iv *Interval) Ceil(date float64) float64 {
	return iv.ceil(date).GetTime()
}

func (iv *Interval) ceil(date float64) *js.Date {
	d := js.NewDate(iv.loc, date-1)
	iv.floori(d)
	iv.offseti(d, 1)
	iv.floori(d)
	return d
}

// Round is interval.round(date).
func (iv *Interval) Round(date float64) float64 {
	d0, d1 := iv.Floor(date), iv.Ceil(date)
	if date-d0 < d1-date {
		return d0
	}
	return d1
}

// Offset is interval.offset(date, step). d3 uses a step of 1 when the
// argument is missing.
func (iv *Interval) Offset(date, step float64) float64 {
	d := js.NewDate(iv.loc, date)
	iv.offseti(d, math.Floor(step))
	return d.GetTime()
}

// Range is interval.range(start, stop, step). d3 uses a step of 1 when the
// argument is missing.
func (iv *Interval) Range(start, stop, step float64) []float64 {
	r := []float64{}
	s := iv.ceil(start)
	step = math.Floor(step)
	if !(s.GetTime() < stop) || !(step > 0) {
		return r // also handles Invalid Date
	}
	for {
		previous := s.GetTime()
		r = append(r, previous)
		iv.offseti(s, step)
		iv.floori(s)
		if !(previous < s.GetTime() && s.GetTime() < stop) {
			break
		}
	}
	return r
}

// Filter is interval.filter(test).
//
// d3 loops forever when no date passes the test before the date becomes
// invalid at the end of the range of the Date object. The port stops there
// and leaves the date invalid.
func (iv *Interval) Filter(test func(date *js.Date) bool) *Interval {
	valid := func(date *js.Date) bool { return !math.IsNaN(date.GetTime()) }
	return NewInterval(iv.loc, func(date *js.Date) {
		if valid(date) {
			for {
				iv.floori(date)
				if test(date) || !valid(date) {
					break
				}
				date.SetTime(date.GetTime() - 1)
			}
		}
	}, func(date *js.Date, step float64) {
		if valid(date) {
			if step < 0 {
				for step++; step <= 0; step++ {
					for {
						iv.offseti(date, -1)
						if test(date) || !valid(date) {
							break
						}
					}
				}
			} else {
				for step--; step >= 0; step-- {
					for {
						iv.offseti(date, +1)
						if test(date) || !valid(date) {
							break
						}
					}
				}
			}
		}
	}, nil, nil)
}

// HasCount reports whether the interval has the methods count and every.
// The intervals that Filter and Every return do not have them.
func (iv *Interval) HasCount() bool { return iv.count != nil }

// Count is interval.count(start, end). The result is NaN for an interval
// that has no count method.
func (iv *Interval) Count(start, end float64) float64 {
	if iv.count == nil {
		return math.NaN()
	}
	t0, t1 := js.NewDate(iv.loc, start), js.NewDate(iv.loc, end)
	iv.floori(t0)
	iv.floori(t1)
	return math.Floor(iv.count(t0, t1))
}

// Every is interval.every(step). It returns nil where d3 returns null, and
// for an interval that has no every method.
func (iv *Interval) Every(step float64) *Interval {
	if iv.every != nil {
		return iv.every(step)
	}
	if iv.count == nil {
		return nil
	}
	step = math.Floor(step)
	if math.IsNaN(step) || math.IsInf(step, 0) || !(step > 0) {
		return nil
	}
	if !(step > 1) {
		return iv
	}
	if iv.field != nil {
		return iv.Filter(func(d *js.Date) bool { return math.Mod(iv.field(d), step) == 0 })
	}
	return iv.Filter(func(d *js.Date) bool { return math.Mod(iv.Count(0, d.GetTime()), step) == 0 })
}
