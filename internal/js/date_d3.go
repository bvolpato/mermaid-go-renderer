// This file ports no upstream file. It adds the JavaScript Date object that
// the d3 port needs. It follows V8 (src/date/date.cc,
// src/builtins/builtins-date.cc) and the ECMAScript abstract operations
// MakeDay, MakeTime, MakeDate and TimeClip.
//
// The local time zone is a parameter. A browser uses the zone of the machine;
// the port passes a *time.Location instead.

package js

import (
	"math"
	"time"
)

const (
	msPerSecond = 1000
	msPerMinute = 60 * msPerSecond
	msPerHour   = 60 * msPerMinute
	msPerDay    = 24 * msPerHour

	// The time value of a Date is at most 8.64e15 ms from the epoch.
	maxTimeMs = 8.64e15
	// A local time may be this far out before the conversion to UTC
	// (kMaxTimeBeforeUTCInMs in V8).
	maxTimeBeforeUTCMs = maxTimeMs + 10*msPerDay
)

// Date is a JavaScript Date: a time value in milliseconds since the epoch in
// UTC, or NaN for an invalid date. The methods that JavaScript defines in
// local time use the location of the date.
type Date struct {
	t   float64
	loc *time.Location
}

// NewDate is new Date(value). A nil location means UTC.
func NewDate(loc *time.Location, value float64) *Date {
	if loc == nil {
		loc = time.UTC
	}
	return &Date{t: TimeClip(value), loc: loc}
}

// NewDateLocal is new Date(year, month, date, hours, minutes, seconds, ms)
// with two to seven arguments. The fields are in local time.
func NewDateLocal(loc *time.Location, year, month float64, rest ...float64) *Date {
	if loc == nil {
		loc = time.UTC
	}
	date, hours, minutes, seconds, ms := 1.0, 0.0, 0.0, 0.0, 0.0
	switch len(rest) {
	default:
		ms = rest[4]
		fallthrough
	case 4:
		seconds = rest[3]
		fallthrough
	case 3:
		minutes = rest[2]
		fallthrough
	case 2:
		hours = rest[1]
		fallthrough
	case 1:
		date = rest[0]
	case 0:
	}
	year = twoDigitYear(year)
	t := makeDate(makeDay(year, month, date), makeTime(hours, minutes, seconds, ms))
	return &Date{t: TimeClip(localToUTC(loc, t)), loc: loc}
}

// DateUTC is Date.UTC(year, month, date, hours, minutes, seconds, ms) with
// one to seven arguments.
func DateUTC(year float64, rest ...float64) float64 {
	month, date, hours, minutes, seconds, ms := 0.0, 1.0, 0.0, 0.0, 0.0, 0.0
	switch len(rest) {
	default:
		ms = rest[5]
		fallthrough
	case 5:
		seconds = rest[4]
		fallthrough
	case 4:
		minutes = rest[3]
		fallthrough
	case 3:
		hours = rest[2]
		fallthrough
	case 2:
		date = rest[1]
		fallthrough
	case 1:
		month = rest[0]
	case 0:
	}
	year = twoDigitYear(year)
	return TimeClip(makeDate(makeDay(year, month, date), makeTime(hours, minutes, seconds, ms)))
}

// twoDigitYear maps the years 0 to 99 to 1900 to 1999, as the Date
// constructor and Date.UTC do.
func twoDigitYear(year float64) float64 {
	if !math.IsNaN(year) {
		y := math.Trunc(year)
		if 0 <= y && y <= 99 {
			return 1900 + y
		}
	}
	return year
}

// TimeClip is the TimeClip abstract operation.
func TimeClip(t float64) float64 {
	if -maxTimeMs <= t && t <= maxTimeMs {
		return math.Trunc(t) + 0
	}
	return math.NaN()
}

func makeTime(hour, min, sec, ms float64) float64 {
	if isFinite(hour) && isFinite(min) && isFinite(sec) && isFinite(ms) {
		h := math.Trunc(hour)
		m := math.Trunc(min)
		s := math.Trunc(sec)
		milli := math.Trunc(ms)
		return float64(h*msPerHour) + float64(m*msPerMinute) + float64(s*msPerSecond) + milli
	}
	return math.NaN()
}

func makeDate(day, time float64) float64 {
	if isFinite(day) && isFinite(time) {
		return time + float64(day*msPerDay)
	}
	return math.NaN()
}

func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}

// makeDay is MakeDay. V8 accepts years within one million of year zero and
// months within ten million.
func makeDay(year, month, date float64) float64 {
	if -1000000 <= year && year <= 1000000 && -10000000 <= month && month <= 10000000 && isFinite(date) {
		y := int64(year)
		m := int64(month)
		y += m / 12
		m %= 12
		if m < 0 {
			m += 12
			y--
		}
		return float64(daysFromCivil(y, m+1, 1)-1) + math.Trunc(date)
	}
	return math.NaN()
}

// daysFromCivil returns the number of days from 1970-01-01 to the given date
// of the proleptic Gregorian calendar. The month is 1 to 12.
func daysFromCivil(y, m, d int64) int64 {
	if m <= 2 {
		y--
	}
	era := floorDiv(y, 400)
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// civilFromDays is the inverse of daysFromCivil. The month is 0 to 11, as in
// JavaScript.
func civilFromDays(days int64) (year, month, day int64) {
	z := days + 719468
	era := floorDiv(z, 146097)
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := doy - (153*mp+2)/5 + 1
	m := mp + 3
	if m > 12 {
		m -= 12
	}
	if m <= 2 {
		y++
	}
	return y, m - 1, d
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// offsetMs returns the offset of local time from UTC at the UTC time t.
func offsetMs(loc *time.Location, t int64) int64 {
	_, off := time.UnixMilli(t).In(loc).Zone()
	return int64(off) * msPerSecond
}

// localToUTC converts a local time value to UTC. A local time that a
// transition skips or repeats uses the offset before the transition, as
// ECMAScript requires. The result is NaN when the local time is out of range.
func localToUTC(loc *time.Location, local float64) float64 {
	if !(local >= -maxTimeBeforeUTCMs && local <= maxTimeBeforeUTCMs) {
		return math.NaN()
	}
	l := int64(local)
	// A zone offset is between -12 and +14 hours. The two probes are on
	// both sides of each transition that can make the local time ambiguous.
	before := offsetMs(loc, l-14*msPerHour)
	after := offsetMs(loc, l+12*msPerHour)
	if before != after {
		validBefore := offsetMs(loc, l-before) == before
		validAfter := offsetMs(loc, l-after) == after
		if !validBefore && validAfter {
			return float64(l - after)
		}
	}
	return float64(l - before)
}

// local returns the local time value and true, or false for an invalid date.
func (d *Date) local() (int64, bool) {
	if math.IsNaN(d.t) {
		return 0, false
	}
	t := int64(d.t)
	return t + offsetMs(d.loc, t), true
}

func (d *Date) utc() (int64, bool) {
	if math.IsNaN(d.t) {
		return 0, false
	}
	return int64(d.t), true
}

// split returns the day number and the time within the day.
func split(ms int64) (days, timeInDay int64) {
	days = floorDiv(ms, msPerDay)
	return days, ms - days*msPerDay
}

func weekday(days int64) int64 {
	r := (days + 4) % 7
	if r < 0 {
		r += 7
	}
	return r
}

// Location returns the local time zone of the date.
func (d *Date) Location() *time.Location { return d.loc }

// Clone is new Date(+date).
func (d *Date) Clone() *Date { return &Date{t: d.t, loc: d.loc} }

// GetTime is getTime and valueOf: the time value, or NaN.
func (d *Date) GetTime() float64 { return d.t }

// SetTime is setTime.
func (d *Date) SetTime(t float64) float64 {
	d.t = TimeClip(t)
	return d.t
}

type dateField int

const (
	fieldYear dateField = iota
	fieldMonth
	fieldDate
	fieldDay
	fieldHours
	fieldMinutes
	fieldSeconds
	fieldMilliseconds
)

func getField(ms int64, ok bool, f dateField) float64 {
	if !ok {
		return math.NaN()
	}
	days, tid := split(ms)
	switch f {
	case fieldDay:
		return float64(weekday(days))
	case fieldHours:
		return float64(tid / msPerHour)
	case fieldMinutes:
		return float64((tid / msPerMinute) % 60)
	case fieldSeconds:
		return float64((tid / msPerSecond) % 60)
	case fieldMilliseconds:
		return float64(tid % 1000)
	}
	y, m, dd := civilFromDays(days)
	switch f {
	case fieldYear:
		return float64(y)
	case fieldMonth:
		return float64(m)
	}
	return float64(dd)
}

func (d *Date) localField(f dateField) float64 {
	ms, ok := d.local()
	return getField(ms, ok, f)
}

func (d *Date) utcField(f dateField) float64 {
	ms, ok := d.utc()
	return getField(ms, ok, f)
}

// The getters return NaN for an invalid date.

func (d *Date) GetFullYear() float64     { return d.localField(fieldYear) }
func (d *Date) GetMonth() float64        { return d.localField(fieldMonth) }
func (d *Date) GetDate() float64         { return d.localField(fieldDate) }
func (d *Date) GetDay() float64          { return d.localField(fieldDay) }
func (d *Date) GetHours() float64        { return d.localField(fieldHours) }
func (d *Date) GetMinutes() float64      { return d.localField(fieldMinutes) }
func (d *Date) GetSeconds() float64      { return d.localField(fieldSeconds) }
func (d *Date) GetMilliseconds() float64 { return d.localField(fieldMilliseconds) }

func (d *Date) GetUTCFullYear() float64     { return d.utcField(fieldYear) }
func (d *Date) GetUTCMonth() float64        { return d.utcField(fieldMonth) }
func (d *Date) GetUTCDate() float64         { return d.utcField(fieldDate) }
func (d *Date) GetUTCDay() float64          { return d.utcField(fieldDay) }
func (d *Date) GetUTCHours() float64        { return d.utcField(fieldHours) }
func (d *Date) GetUTCMinutes() float64      { return d.utcField(fieldMinutes) }
func (d *Date) GetUTCSeconds() float64      { return d.utcField(fieldSeconds) }
func (d *Date) GetUTCMilliseconds() float64 { return d.utcField(fieldMilliseconds) }

// GetTimezoneOffset is getTimezoneOffset: UTC minus local time in minutes.
// V8 truncates the result to an integer, which matters for the zones that
// have an offset with seconds (local mean time).
func (d *Date) GetTimezoneOffset() float64 {
	if math.IsNaN(d.t) {
		return math.NaN()
	}
	t := int64(d.t)
	return float64(-offsetMs(d.loc, t) / msPerMinute)
}

// setValue stores a time value that the setters computed. A local value is
// converted to UTC first.
func (d *Date) setValue(t float64, local bool) float64 {
	if local {
		t = localToUTC(d.loc, t)
	}
	d.t = TimeClip(t)
	return d.t
}

func (d *Date) fields(local bool) (int64, bool) {
	if local {
		return d.local()
	}
	return d.utc()
}

func (d *Date) setFullYear(local bool, year float64, rest []float64) float64 {
	m, dt := 0.0, 1.0
	var tid int64
	if ms, ok := d.fields(local); ok {
		var days int64
		days, tid = split(ms)
		_, mi, di := civilFromDays(days)
		m, dt = float64(mi), float64(di)
	}
	if len(rest) >= 1 {
		m = rest[0]
		if len(rest) >= 2 {
			dt = rest[1]
		}
	}
	return d.setValue(makeDate(makeDay(year, m, dt), float64(tid)), local)
}

func (d *Date) setMonth(local bool, month float64, rest []float64) float64 {
	ms, ok := d.fields(local)
	if !ok {
		return d.t
	}
	days, tid := split(ms)
	y, _, di := civilFromDays(days)
	dt := float64(di)
	if len(rest) >= 1 {
		dt = rest[0]
	}
	return d.setValue(makeDate(makeDay(float64(y), month, dt), float64(tid)), local)
}

func (d *Date) setDate(local bool, date float64) float64 {
	ms, ok := d.fields(local)
	if !ok {
		return d.t
	}
	days, tid := split(ms)
	y, m, _ := civilFromDays(days)
	return d.setValue(makeDate(makeDay(float64(y), float64(m), date), float64(tid)), local)
}

// setClock implements setHours, setMinutes, setSeconds and setMilliseconds.
// The argument first is the index of the first field that args sets: 0 for
// hours, 1 for minutes, 2 for seconds, 3 for milliseconds.
func (d *Date) setClock(local bool, first int, args []float64) float64 {
	ms, ok := d.fields(local)
	if !ok {
		return d.t
	}
	days, tid := split(ms)
	f := [4]float64{
		float64(tid / msPerHour),
		float64((tid / msPerMinute) % 60),
		float64((tid / msPerSecond) % 60),
		float64(tid % 1000),
	}
	for i, v := range args {
		if first+i > 3 {
			break
		}
		f[first+i] = v
	}
	return d.setValue(makeDate(float64(days), makeTime(f[0], f[1], f[2], f[3])), local)
}

// The setters take the same optional arguments as in JavaScript and return
// the new time value.

// SetFullYear is setFullYear(year, month, date).
func (d *Date) SetFullYear(year float64, rest ...float64) float64 {
	return d.setFullYear(true, year, rest)
}

// SetMonth is setMonth(month, date).
func (d *Date) SetMonth(month float64, rest ...float64) float64 {
	return d.setMonth(true, month, rest)
}

// SetDate is setDate(date).
func (d *Date) SetDate(date float64) float64 { return d.setDate(true, date) }

// SetHours is setHours(hours, minutes, seconds, ms).
func (d *Date) SetHours(hours float64, rest ...float64) float64 {
	return d.setClock(true, 0, append([]float64{hours}, rest...))
}

// SetMinutes is setMinutes(minutes, seconds, ms).
func (d *Date) SetMinutes(minutes float64, rest ...float64) float64 {
	return d.setClock(true, 1, append([]float64{minutes}, rest...))
}

// SetSeconds is setSeconds(seconds, ms).
func (d *Date) SetSeconds(seconds float64, rest ...float64) float64 {
	return d.setClock(true, 2, append([]float64{seconds}, rest...))
}

// SetMilliseconds is setMilliseconds(ms).
func (d *Date) SetMilliseconds(ms float64) float64 {
	return d.setClock(true, 3, []float64{ms})
}

// SetUTCFullYear is setUTCFullYear(year, month, date).
func (d *Date) SetUTCFullYear(year float64, rest ...float64) float64 {
	return d.setFullYear(false, year, rest)
}

// SetUTCMonth is setUTCMonth(month, date).
func (d *Date) SetUTCMonth(month float64, rest ...float64) float64 {
	return d.setMonth(false, month, rest)
}

// SetUTCDate is setUTCDate(date).
func (d *Date) SetUTCDate(date float64) float64 { return d.setDate(false, date) }

// SetUTCHours is setUTCHours(hours, minutes, seconds, ms).
func (d *Date) SetUTCHours(hours float64, rest ...float64) float64 {
	return d.setClock(false, 0, append([]float64{hours}, rest...))
}

// SetUTCMinutes is setUTCMinutes(minutes, seconds, ms).
func (d *Date) SetUTCMinutes(minutes float64, rest ...float64) float64 {
	return d.setClock(false, 1, append([]float64{minutes}, rest...))
}

// SetUTCSeconds is setUTCSeconds(seconds, ms).
func (d *Date) SetUTCSeconds(seconds float64, rest ...float64) float64 {
	return d.setClock(false, 2, append([]float64{seconds}, rest...))
}

// SetUTCMilliseconds is setUTCMilliseconds(ms).
func (d *Date) SetUTCMilliseconds(ms float64) float64 {
	return d.setClock(false, 3, []float64{ms})
}
