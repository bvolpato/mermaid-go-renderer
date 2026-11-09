// Ported from d3-time 3.1.0, d3-time/src: millisecond.js, second.js,
// minute.js, hour.js, day.js, week.js, month.js, year.js and ticks.js (ISC).

package time

import (
	"math"
	stdtime "time"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/array"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Intervals is one family of d3 intervals: the local-time intervals
// (timeSecond to timeYear) of a zone, or the UTC intervals (utcSecond to
// utcYear).
type Intervals struct {
	Millisecond *Interval
	Second      *Interval
	Minute      *Interval
	Hour        *Interval
	Day         *Interval
	// Week is the same interval as Sunday.
	Week      *Interval
	Sunday    *Interval
	Monday    *Interval
	Tuesday   *Interval
	Wednesday *Interval
	Thursday  *Interval
	Friday    *Interval
	Saturday  *Interval
	Month     *Interval
	Year      *Interval
	// UnixDay is d3.unixDay. It is nil in a local-time family.
	UnixDay *Interval

	// tickDay is the day interval of the tick computation: timeDay for local
	// time and unixDay for UTC.
	tickDay       *Interval
	tickIntervals []tickInterval
	loc           *stdtime.Location
}

type tickInterval struct {
	interval *Interval
	step     float64
	duration float64
}

func newMillisecond(loc *stdtime.Location) *Interval {
	millisecond := NewInterval(loc, func(date *js.Date) {
		// noop
	}, func(date *js.Date, step float64) {
		date.SetTime(date.GetTime() + step)
	}, func(start, end *js.Date) float64 {
		return end.GetTime() - start.GetTime()
	}, nil)
	// An optimized implementation for this simple case.
	millisecond.every = func(k float64) *Interval {
		k = math.Floor(k)
		if math.IsNaN(k) || math.IsInf(k, 0) || !(k > 0) {
			return nil
		}
		if !(k > 1) {
			return millisecond
		}
		return NewInterval(loc, func(date *js.Date) {
			date.SetTime(float64(math.Floor(date.GetTime()/k) * k))
		}, func(date *js.Date, step float64) {
			date.SetTime(date.GetTime() + float64(step*k))
		}, func(start, end *js.Date) float64 {
			return (end.GetTime() - start.GetTime()) / k
		}, nil)
	}
	return millisecond
}

func newSecond(loc *stdtime.Location) *Interval {
	return NewInterval(loc, func(date *js.Date) {
		date.SetTime(date.GetTime() - date.GetMilliseconds())
	}, func(date *js.Date, step float64) {
		date.SetTime(date.GetTime() + float64(step*DurationSecond))
	}, func(start, end *js.Date) float64 {
		return (end.GetTime() - start.GetTime()) / DurationSecond
	}, func(date *js.Date) float64 {
		return date.GetUTCSeconds()
	})
}

// localOffsetDiff is (end.getTimezoneOffset() - start.getTimezoneOffset()) * durationMinute.
func localOffsetDiff(start, end *js.Date) float64 {
	return float64((end.GetTimezoneOffset() - start.GetTimezoneOffset()) * DurationMinute)
}

func timeWeekday(loc *stdtime.Location, i float64) *Interval {
	return NewInterval(loc, func(date *js.Date) {
		date.SetDate(date.GetDate() - math.Mod(date.GetDay()+7-i, 7))
		date.SetHours(0, 0, 0, 0)
	}, func(date *js.Date, step float64) {
		date.SetDate(date.GetDate() + float64(step*7))
	}, func(start, end *js.Date) float64 {
		return (end.GetTime() - start.GetTime() - localOffsetDiff(start, end)) / DurationWeek
	}, nil)
}

func utcWeekday(i float64) *Interval {
	return NewInterval(stdtime.UTC, func(date *js.Date) {
		date.SetUTCDate(date.GetUTCDate() - math.Mod(date.GetUTCDay()+7-i, 7))
		date.SetUTCHours(0, 0, 0, 0)
	}, func(date *js.Date, step float64) {
		date.SetUTCDate(date.GetUTCDate() + float64(step*7))
	}, func(start, end *js.Date) float64 {
		return (end.GetTime() - start.GetTime()) / DurationWeek
	}, nil)
}

// Local returns the local-time intervals of d3 (timeMillisecond to timeYear)
// for the given zone. A browser uses the zone of the machine. A nil zone
// means UTC.
func Local(loc *stdtime.Location) *Intervals {
	if loc == nil {
		loc = stdtime.UTC
	}
	s := &Intervals{loc: loc}
	s.Millisecond = newMillisecond(loc)
	s.Second = newSecond(loc)

	s.Minute = NewInterval(loc, func(date *js.Date) {
		date.SetTime(date.GetTime() - date.GetMilliseconds() - float64(date.GetSeconds()*DurationSecond))
	}, func(date *js.Date, step float64) {
		date.SetTime(date.GetTime() + float64(step*DurationMinute))
	}, func(start, end *js.Date) float64 {
		return (end.GetTime() - start.GetTime()) / DurationMinute
	}, func(date *js.Date) float64 {
		return date.GetMinutes()
	})

	s.Hour = NewInterval(loc, func(date *js.Date) {
		date.SetTime(date.GetTime() - date.GetMilliseconds() - float64(date.GetSeconds()*DurationSecond) - float64(date.GetMinutes()*DurationMinute))
	}, func(date *js.Date, step float64) {
		date.SetTime(date.GetTime() + float64(step*DurationHour))
	}, func(start, end *js.Date) float64 {
		return (end.GetTime() - start.GetTime()) / DurationHour
	}, func(date *js.Date) float64 {
		return date.GetHours()
	})

	s.Day = NewInterval(loc, func(date *js.Date) {
		date.SetHours(0, 0, 0, 0)
	}, func(date *js.Date, step float64) {
		date.SetDate(date.GetDate() + step)
	}, func(start, end *js.Date) float64 {
		return (end.GetTime() - start.GetTime() - localOffsetDiff(start, end)) / DurationDay
	}, func(date *js.Date) float64 {
		return date.GetDate() - 1
	})

	s.Sunday = timeWeekday(loc, 0)
	s.Monday = timeWeekday(loc, 1)
	s.Tuesday = timeWeekday(loc, 2)
	s.Wednesday = timeWeekday(loc, 3)
	s.Thursday = timeWeekday(loc, 4)
	s.Friday = timeWeekday(loc, 5)
	s.Saturday = timeWeekday(loc, 6)
	s.Week = s.Sunday

	s.Month = NewInterval(loc, func(date *js.Date) {
		date.SetDate(1)
		date.SetHours(0, 0, 0, 0)
	}, func(date *js.Date, step float64) {
		date.SetMonth(date.GetMonth() + step)
	}, func(start, end *js.Date) float64 {
		return end.GetMonth() - start.GetMonth() + float64((end.GetFullYear()-start.GetFullYear())*12)
	}, func(date *js.Date) float64 {
		return date.GetMonth()
	})

	s.Year = NewInterval(loc, func(date *js.Date) {
		date.SetMonth(0, 1)
		date.SetHours(0, 0, 0, 0)
	}, func(date *js.Date, step float64) {
		date.SetFullYear(date.GetFullYear() + step)
	}, func(start, end *js.Date) float64 {
		return end.GetFullYear() - start.GetFullYear()
	}, func(date *js.Date) float64 {
		return date.GetFullYear()
	})
	// An optimized implementation for this simple case.
	s.Year.every = func(k float64) *Interval {
		k = math.Floor(k)
		if math.IsNaN(k) || math.IsInf(k, 0) || !(k > 0) {
			return nil
		}
		return NewInterval(loc, func(date *js.Date) {
			date.SetFullYear(float64(math.Floor(date.GetFullYear()/k) * k))
			date.SetMonth(0, 1)
			date.SetHours(0, 0, 0, 0)
		}, func(date *js.Date, step float64) {
			date.SetFullYear(date.GetFullYear() + float64(step*k))
		}, nil, nil)
	}

	s.tickDay = s.Day
	s.initTicker()
	return s
}

// UTC returns the UTC intervals of d3 (utcMillisecond to utcYear).
func UTC() *Intervals {
	loc := stdtime.UTC
	s := &Intervals{loc: loc}
	s.Millisecond = newMillisecond(loc)
	s.Second = newSecond(loc)

	s.Minute = NewInterval(loc, func(date *js.Date) {
		date.SetUTCSeconds(0, 0)
	}, func(date *js.Date, step float64) {
		date.SetTime(date.GetTime() + float64(step*DurationMinute))
	}, func(start, end *js.Date) float64 {
		return (end.GetTime() - start.GetTime()) / DurationMinute
	}, func(date *js.Date) float64 {
		return date.GetUTCMinutes()
	})

	s.Hour = NewInterval(loc, func(date *js.Date) {
		date.SetUTCMinutes(0, 0, 0)
	}, func(date *js.Date, step float64) {
		date.SetTime(date.GetTime() + float64(step*DurationHour))
	}, func(start, end *js.Date) float64 {
		return (end.GetTime() - start.GetTime()) / DurationHour
	}, func(date *js.Date) float64 {
		return date.GetUTCHours()
	})

	utcDayFloor := func(date *js.Date) {
		date.SetUTCHours(0, 0, 0, 0)
	}
	utcDayOffset := func(date *js.Date, step float64) {
		date.SetUTCDate(date.GetUTCDate() + step)
	}
	utcDayCount := func(start, end *js.Date) float64 {
		return (end.GetTime() - start.GetTime()) / DurationDay
	}
	s.Day = NewInterval(loc, utcDayFloor, utcDayOffset, utcDayCount, func(date *js.Date) float64 {
		return date.GetUTCDate() - 1
	})
	s.UnixDay = NewInterval(loc, utcDayFloor, utcDayOffset, utcDayCount, func(date *js.Date) float64 {
		return math.Floor(date.GetTime() / DurationDay)
	})

	s.Sunday = utcWeekday(0)
	s.Monday = utcWeekday(1)
	s.Tuesday = utcWeekday(2)
	s.Wednesday = utcWeekday(3)
	s.Thursday = utcWeekday(4)
	s.Friday = utcWeekday(5)
	s.Saturday = utcWeekday(6)
	s.Week = s.Sunday

	s.Month = NewInterval(loc, func(date *js.Date) {
		date.SetUTCDate(1)
		date.SetUTCHours(0, 0, 0, 0)
	}, func(date *js.Date, step float64) {
		date.SetUTCMonth(date.GetUTCMonth() + step)
	}, func(start, end *js.Date) float64 {
		return end.GetUTCMonth() - start.GetUTCMonth() + float64((end.GetUTCFullYear()-start.GetUTCFullYear())*12)
	}, func(date *js.Date) float64 {
		return date.GetUTCMonth()
	})

	s.Year = NewInterval(loc, func(date *js.Date) {
		date.SetUTCMonth(0, 1)
		date.SetUTCHours(0, 0, 0, 0)
	}, func(date *js.Date, step float64) {
		date.SetUTCFullYear(date.GetUTCFullYear() + step)
	}, func(start, end *js.Date) float64 {
		return end.GetUTCFullYear() - start.GetUTCFullYear()
	}, func(date *js.Date) float64 {
		return date.GetUTCFullYear()
	})
	// An optimized implementation for this simple case.
	s.Year.every = func(k float64) *Interval {
		k = math.Floor(k)
		if math.IsNaN(k) || math.IsInf(k, 0) || !(k > 0) {
			return nil
		}
		return NewInterval(loc, func(date *js.Date) {
			date.SetUTCFullYear(float64(math.Floor(date.GetUTCFullYear()/k) * k))
			date.SetUTCMonth(0, 1)
			date.SetUTCHours(0, 0, 0, 0)
		}, func(date *js.Date, step float64) {
			date.SetUTCFullYear(date.GetUTCFullYear() + float64(step*k))
		}, nil, nil)
	}

	s.tickDay = s.UnixDay
	s.initTicker()
	return s
}

// Location returns the zone of a local-time family, or UTC.
func (s *Intervals) Location() *stdtime.Location { return s.loc }

func (s *Intervals) initTicker() {
	s.tickIntervals = []tickInterval{
		{s.Second, 1, DurationSecond},
		{s.Second, 5, 5 * DurationSecond},
		{s.Second, 15, 15 * DurationSecond},
		{s.Second, 30, 30 * DurationSecond},
		{s.Minute, 1, DurationMinute},
		{s.Minute, 5, 5 * DurationMinute},
		{s.Minute, 15, 15 * DurationMinute},
		{s.Minute, 30, 30 * DurationMinute},
		{s.Hour, 1, DurationHour},
		{s.Hour, 3, 3 * DurationHour},
		{s.Hour, 6, 6 * DurationHour},
		{s.Hour, 12, 12 * DurationHour},
		{s.tickDay, 1, DurationDay},
		{s.tickDay, 2, 2 * DurationDay},
		{s.Week, 1, DurationWeek},
		{s.Month, 1, DurationMonth},
		{s.Month, 3, 3 * DurationMonth},
		{s.Year, 1, DurationYear},
	}
}

var tickBisector = array.NewBisector(func(d tickInterval) float64 { return d.duration })

// Ticks is d3.timeTicks(start, stop, count) for a local-time family and
// d3.utcTicks(start, stop, count) for the UTC family.
func (s *Intervals) Ticks(start, stop, count float64) []float64 {
	reverse := stop < start
	if reverse {
		start, stop = stop, start
	}
	var interval *Interval
	// A count of zero or NaN is falsy in d3; tickInterval handles it too.
	interval = s.TickInterval(start, stop, count)
	return s.ticksIn(interval, start, stop, reverse)
}

// TicksInterval is d3.timeTicks(start, stop, interval) with a time interval
// in place of the count.
func (s *Intervals) TicksInterval(start, stop float64, interval *Interval) []float64 {
	reverse := stop < start
	if reverse {
		start, stop = stop, start
	}
	return s.ticksIn(interval, start, stop, reverse)
}

func (s *Intervals) ticksIn(interval *Interval, start, stop float64, reverse bool) []float64 {
	ticks := []float64{}
	if interval != nil {
		ticks = interval.Range(start, stop+1, 1) // inclusive stop
	}
	if reverse {
		for a, b := 0, len(ticks)-1; a < b; a, b = a+1, b-1 {
			ticks[a], ticks[b] = ticks[b], ticks[a]
		}
	}
	return ticks
}

// TickInterval is d3.timeTickInterval(start, stop, count) for a local-time
// family and d3.utcTickInterval(start, stop, count) for the UTC family. The
// result is nil where d3 returns null.
func (s *Intervals) TickInterval(start, stop, count float64) *Interval {
	target := math.Abs(stop-start) / count
	i := tickBisector.Right(s.tickIntervals, target, 0, len(s.tickIntervals))
	if i == len(s.tickIntervals) {
		return s.Year.Every(array.TickStep(start/DurationYear, stop/DurationYear, count))
	}
	if i == 0 {
		return s.Millisecond.Every(js.Max(array.TickStep(start, stop, count), 1))
	}
	t := s.tickIntervals[i]
	if target/s.tickIntervals[i-1].duration < s.tickIntervals[i].duration/target {
		t = s.tickIntervals[i-1]
	}
	return t.interval.Every(t.step)
}
