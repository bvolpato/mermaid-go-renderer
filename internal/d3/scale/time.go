// Ported from d3-scale 4.0.2, d3-scale/src/time.js, d3-scale/src/utcTime.js
// and d3-scale/src/nice.js (ISC).

package scale

import (
	"math"
	stdtime "time"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/interpolate"
	d3time "github.com/bvolpato/mermaid-go-renderer/internal/d3/time"
	"github.com/bvolpato/mermaid-go-renderer/internal/d3/timefmt"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Time is d3.scaleTime and d3.scaleUtc. A date is a JavaScript time value: a
// float64 with the milliseconds since the epoch in UTC, or NaN for an invalid
// date.
type Time struct {
	continuous[float64]
	intervals *d3time.Intervals
	// day is the day interval of the tick format: timeDay or utcDay.
	day    *d3time.Interval
	format func(specifier string) func(date float64) string

	formatMillisecond, formatSecond, formatMinute, formatHour func(date float64) string
	formatDay, formatWeek, formatMonth, formatYear            func(date float64) string
}

func newCalendar(intervals *d3time.Intervals, format func(specifier string) func(date float64) string) *Time {
	s := &Time{intervals: intervals, day: intervals.Day, format: format}
	s.domain = []float64{0, 1}
	s.rng = []float64{0, 1}
	s.interpolate = interpolate.Number
	s.unknown = math.NaN()
	s.number = identityNumber
	s.formatMillisecond = format(".%L")
	s.formatSecond = format(":%S")
	s.formatMinute = format("%I:%M")
	s.formatHour = format("%I %p")
	s.formatDay = format("%a %d")
	s.formatWeek = format("%b %d")
	s.formatMonth = format("%B")
	s.formatYear = format("%Y")
	return s
}

// NewTime is d3.scaleTime() for the given local time zone. A browser uses
// the zone of the machine. A nil zone means UTC. The domain is the first day
// of the year 2000 in local time.
func NewTime(loc *stdtime.Location) *Time {
	if loc == nil {
		loc = stdtime.UTC
	}
	s := newCalendar(d3time.Local(loc), func(specifier string) func(float64) string {
		return timefmt.Format(loc, specifier)
	})
	return s.Domain(js.NewDateLocal(loc, 2000, 0, 1).GetTime(), js.NewDateLocal(loc, 2000, 0, 2).GetTime())
}

// NewUTC is d3.scaleUtc(). The domain is the first day of the year 2000 in
// UTC.
func NewUTC() *Time {
	s := newCalendar(d3time.UTC(), timefmt.UTCFormat)
	return s.Domain(js.DateUTC(2000, 0, 1), js.DateUTC(2000, 0, 2))
}

// Domain is scale.domain(dates). Each value is clipped as the Date object
// clips a time value.
func (s *Time) Domain(dates ...float64) *Time {
	values := make([]float64, len(dates))
	for i, t := range dates {
		values[i] = js.TimeClip(t)
	}
	s.setDomain(values)
	return s
}

// DomainValues is scale.domain(): a copy of the domain.
func (s *Time) DomainValues() []float64 { return append([]float64(nil), s.domain...) }

// Range is scale.range(values).
func (s *Time) Range(values ...float64) *Time { s.setRange(values); return s }

// RangeValues is scale.range(): a copy of the range.
func (s *Time) RangeValues() []float64 { return append([]float64(nil), s.rng...) }

// RangeRound is scale.rangeRound(values).
func (s *Time) RangeRound(values ...float64) *Time { s.setRangeRound(values); return s }

// Clamp is scale.clamp(clamp).
func (s *Time) Clamp(clamp bool) *Time { s.clamp = clamp; s.rescale(); return s }

// Clamped is scale.clamp().
func (s *Time) Clamped() bool { return s.clamp }

// Interpolate is scale.interpolate(interpolate).
func (s *Time) Interpolate(interpolate func(a, b float64) func(t float64) float64) *Time {
	s.interpolate = interpolate
	s.rescale()
	return s
}

// Unknown is scale.unknown(value): the output for an invalid date.
func (s *Time) Unknown(value float64) *Time { s.unknown = value; return s }

// Scale is scale(date).
func (s *Time) Scale(date float64) float64 { return s.scale(date) }

// Invert is scale.invert(y): a date.
func (s *Time) Invert(y float64) float64 { return js.TimeClip(s.invert(y)) }

// Intervals returns the time intervals of the scale: the local-time
// intervals of its zone, or the UTC intervals.
func (s *Time) Intervals() *d3time.Intervals { return s.intervals }

// Ticks is scale.ticks(count). Without an argument, the count is 10.
func (s *Time) Ticks(count ...float64) []float64 {
	d := s.domain
	return s.intervals.Ticks(at(d, 0), at(d, len(d)-1), countOr10(count))
}

// TicksInterval is scale.ticks(interval) with a time interval, for example
// scale.ticks(d3.timeDay.every(2)). A nil interval is the null that
// interval.every returns for an invalid step; d3 then uses a count of 10.
func (s *Time) TicksInterval(interval *d3time.Interval) []float64 {
	if interval == nil {
		return s.Ticks()
	}
	d := s.domain
	return s.intervals.TicksInterval(at(d, 0), at(d, len(d)-1), interval)
}

// TickFormat is scale.tickFormat(count, specifier). d3 ignores the count.
// Without a specifier, the format depends on the date: it shows the smallest
// unit that is not zero.
func (s *Time) TickFormat(specifier ...string) func(date float64) string {
	if len(specifier) > 0 {
		return s.format(specifier[0])
	}
	return s.tickFormat
}

func (s *Time) tickFormat(date float64) string {
	t := s.intervals
	// The comparisons are false for an invalid date, so that the year
	// format applies, as in d3.
	switch {
	case t.Second.Floor(date) < date:
		return s.formatMillisecond(date)
	case t.Minute.Floor(date) < date:
		return s.formatSecond(date)
	case t.Hour.Floor(date) < date:
		return s.formatMinute(date)
	case s.day.Floor(date) < date:
		return s.formatHour(date)
	case t.Month.Floor(date) < date:
		if t.Week.Floor(date) < date {
			return s.formatDay(date)
		}
		return s.formatWeek(date)
	case t.Year.Floor(date) < date:
		return s.formatMonth(date)
	}
	return s.formatYear(date)
}

// Nice is scale.nice(count). Without an argument, the count is 10.
func (s *Time) Nice(count ...float64) *Time {
	d := s.domain
	return s.NiceInterval(s.intervals.TickInterval(at(d, 0), at(d, len(d)-1), countOr10(count)))
}

// NiceInterval is scale.nice(interval) with a time interval. A nil interval
// leaves the scale as it is.
func (s *Time) NiceInterval(interval *d3time.Interval) *Time {
	if interval == nil {
		return s
	}
	return s.Domain(nice(s.domain, interval)...)
}

// nice ports nice.js.
func nice(domain []float64, interval *d3time.Interval) []float64 {
	domain = append([]float64(nil), domain...)
	if len(domain) == 0 {
		return domain
	}

	i0 := 0
	i1 := len(domain) - 1
	x0 := domain[i0]
	x1 := domain[i1]

	if x1 < x0 {
		i0, i1 = i1, i0
		x0, x1 = x1, x0
	}

	domain[i0] = interval.Floor(x0)
	domain[i1] = interval.Ceil(x1)
	return domain
}

// Copy is scale.copy().
func (s *Time) Copy() *Time {
	c := newCalendar(s.intervals, s.format)
	s.copyTo(&c.continuous)
	return c
}
