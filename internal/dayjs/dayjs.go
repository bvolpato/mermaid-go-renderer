// Package dayjs is a port of the parts of dayjs 1.11.21 that the gantt
// diagram of mermaid uses: the core, and the plugins isoWeek,
// customParseFormat, advancedFormat and duration. It has the locale "en"
// only, and it has no UTC mode.
//
// Ported from dayjs 1.11.21, dayjs/esm/index.js, dayjs/esm/utils.js,
// dayjs/esm/constant.js and dayjs/esm/locale/en.js (MIT).
//
// A JavaScript exception is a Go error here. Only Format and the functions
// that call it can fail: mermaid does not load the plugins that some format
// tokens need (weekOfYear, weekYear, timezone), so those tokens throw a
// TypeError upstream.
package dayjs

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Constants of constant.js.
const (
	secondsAMinute      = 60
	secondsAnHour       = secondsAMinute * 60
	secondsADay         = secondsAnHour * 24
	secondsAWeek        = secondsADay * 7
	millisecondsASecond = 1e3
	millisecondsAMinute = secondsAMinute * millisecondsASecond
	millisecondsAnHour  = secondsAnHour * millisecondsASecond
	millisecondsADay    = secondsADay * millisecondsASecond
	millisecondsAWeek   = secondsAWeek * millisecondsASecond

	unitMS   = "millisecond"
	unitS    = "second"
	unitMin  = "minute"
	unitH    = "hour"
	unitD    = "day"
	unitW    = "week"
	unitM    = "month"
	unitQ    = "quarter"
	unitY    = "year"
	unitDate = "date"

	formatDefault     = "YYYY-MM-DDTHH:mm:ssZ"
	invalidDateString = "Invalid Date"
)

// The locale "en".
var (
	enWeekdays = strings.Split("Sunday_Monday_Tuesday_Wednesday_Thursday_Friday_Saturday", "_")
	enMonths   = strings.Split("January_February_March_April_May_June_July_August_September_October_November_December", "_")
)

// enOrdinal is the ordinal function of the locale "en". The result has
// square brackets, which escape the text in a format string.
func enOrdinal(n float64) string {
	s := [4]string{"th", "st", "nd", "rd"}
	v := math.Mod(n, 100)
	suffix := s[0]
	if i := math.Mod(v-20, 10); i == 1 || i == 2 || i == 3 {
		suffix = s[int(i)]
	} else if i == 0 {
		// s[0], also for -0.
	} else if v == 1 || v == 2 || v == 3 {
		suffix = s[int(v)]
	}
	return "[" + js.Str(n) + suffix + "]"
}

// TypeError is a JavaScript TypeError that dayjs throws.
type TypeError struct{ Message string }

func (e *TypeError) Error() string { return "TypeError: " + e.Message }

// Factory is the dayjs function with its environment: the local time zone
// and the clock of the browser.
type Factory struct {
	loc *time.Location
	now float64
}

// New returns the dayjs function for a local time zone and a current time.
// The time is a JavaScript time value: milliseconds since the epoch. A nil
// location means UTC.
func New(loc *time.Location, now time.Time) *Factory {
	if loc == nil {
		loc = time.UTC
	}
	return &Factory{loc: loc, now: float64(now.UnixMilli())}
}

// Location returns the local time zone.
func (f *Factory) Location() *time.Location { return f.loc }

// newDate is new Date().
func (f *Factory) newDate() *js.Date { return js.NewDate(f.loc, f.now) }

// Dayjs is a dayjs object. It is immutable, as in dayjs.
type Dayjs struct {
	f *Factory
	// d is $d, and the other fields are $y, $M, $D, $W, $H, $m, $s and $ms.
	d                       *js.Date
	y, M, D, W, H, m, s, ms float64
}

// wrap is the constructor: new Dayjs({date}) with a Date object, which the
// constructor copies.
func (f *Factory) wrap(date *js.Date) *Dayjs {
	d := &Dayjs{f: f, d: date}
	d.init()
	return d
}

// Now is dayjs(): the current time.
func (f *Factory) Now() *Dayjs { return f.wrap(f.newDate()) }

// FromTime is dayjs(date) with a Date object, and dayjs(number). The
// argument is a time value; NaN is an invalid date.
func (f *Factory) FromTime(t float64) *Dayjs { return f.wrap(js.NewDate(f.loc, t)) }

func (d *Dayjs) init() {
	d.y, d.M, d.D, d.W, d.H, d.m, d.s, d.ms = d.d.LocalFields()
}

// IsValid is isValid().
func (d *Dayjs) IsValid() bool { return !math.IsNaN(d.d.GetTime()) }

// ValueOf is valueOf(): the time value, or NaN.
func (d *Dayjs) ValueOf() float64 { return d.d.GetTime() }

// ToDate is toDate(): the time value of a new Date object.
func (d *Dayjs) ToDate() float64 { return d.d.GetTime() }

// Unix is unix().
func (d *Dayjs) Unix() float64 { return math.Floor(d.ValueOf() / 1000) }

// Clone is clone().
func (d *Dayjs) Clone() *Dayjs { return d.f.wrap(d.d.Clone()) }

// Year is year() as a getter. The other getters follow.
func (d *Dayjs) Year() float64 { return d.y }

// Month is month(): 0 is January.
func (d *Dayjs) Month() float64 { return d.M }

// Date is date(): the day of the month.
func (d *Dayjs) Date() float64 { return d.D }

// Day is day(): 0 is Sunday.
func (d *Dayjs) Day() float64 { return d.W }

// Hour is hour().
func (d *Dayjs) Hour() float64 { return d.H }

// Minute is minute().
func (d *Dayjs) Minute() float64 { return d.m }

// Second is second().
func (d *Dayjs) Second() float64 { return d.s }

// Millisecond is millisecond().
func (d *Dayjs) Millisecond() float64 { return d.ms }

// padStart is Utils.s for a string value.
func padStart(s string, length int, pad string) string {
	n := js.UTF16Len(s)
	if s == "" || n >= length {
		return s
	}
	return strings.Repeat(pad, length-n) + s
}

// numStr is String(v). The fields of a date are small integers, which
// print as their decimal digits.
func numStr(v float64) string {
	if v == math.Trunc(v) && v > -1e15 && v < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return js.Str(v)
}

// padNum is Utils.s for a number.
func padNum(v float64, length int, pad string) string {
	return padStart(numStr(v), length, pad)
}

// padZoneStr is Utils.z.
func padZoneStr(instance *Dayjs) string {
	negMinutes := -instance.UtcOffset()
	minutes := math.Abs(negMinutes)
	hourOffset := math.Floor(minutes / 60)
	minuteOffset := math.Mod(minutes, 60)
	sign := "-"
	if negMinutes <= 0 {
		sign = "+"
	}
	return sign + padNum(hourOffset, 2, "0") + ":" + padNum(minuteOffset, 2, "0")
}

// monthDiff is Utils.m.
func monthDiff(a, b *Dayjs) float64 {
	// function from moment.js in order to keep the same result
	if a.Date() < b.Date() {
		return -monthDiff(b, a)
	}
	wholeMonthDiff := float64((b.Year()-a.Year())*12) + (b.Month() - a.Month())
	anchor := a.Clone().Add(wholeMonthDiff, unitM)
	c := b.ValueOf()-anchor.ValueOf() < 0
	step := 1.0
	if c {
		step = -1
	}
	anchor2 := a.Clone().Add(wholeMonthDiff+step, unitM)
	var den float64
	if c {
		den = anchor.ValueOf() - anchor2.ValueOf()
	} else {
		den = anchor2.ValueOf() - anchor.ValueOf()
	}
	r := -(wholeMonthDiff + (b.ValueOf()-anchor.ValueOf())/den)
	// +(x || 0)
	if r == 0 || math.IsNaN(r) {
		return 0
	}
	return r
}

// absFloor is Utils.a.
func absFloor(n float64) float64 {
	if n < 0 {
		r := math.Ceil(n)
		if r == 0 {
			return 0
		}
		return r
	}
	return math.Floor(n)
}

// prettyUnit is Utils.p.
func prettyUnit(u string) string {
	switch u {
	case "M":
		return unitM
	case "y":
		return unitY
	case "w":
		return unitW
	case "d":
		return unitD
	case "D":
		return unitDate
	case "h":
		return unitH
	case "m":
		return unitMin
	case "s":
		return unitS
	case "ms":
		return unitMS
	case "Q":
		return unitQ
	}
	return strings.TrimSuffix(js.ToLowerCase(u), "s")
}

// IsSame is isSame(that, units).
func (d *Dayjs) IsSame(that *Dayjs, units string) bool {
	other := that.Clone()
	return d.StartOf(units).ValueOf() <= other.ValueOf() && other.ValueOf() <= d.EndOf(units).ValueOf()
}

// IsAfter is isAfter(that, units).
func (d *Dayjs) IsAfter(that *Dayjs, units string) bool {
	return that.Clone().ValueOf() < d.StartOf(units).ValueOf()
}

// IsBefore is isBefore(that, units).
func (d *Dayjs) IsBefore(that *Dayjs, units string) bool {
	return d.EndOf(units).ValueOf() < that.Clone().ValueOf()
}

// StartOf is startOf(units).
func (d *Dayjs) StartOf(units string) *Dayjs { return d.startOf(units, true) }

// EndOf is endOf(units).
func (d *Dayjs) EndOf(units string) *Dayjs { return d.startOf(units, false) }

func (d *Dayjs) startOf(units string, isStartOf bool) *Dayjs {
	unit := prettyUnit(units)

	// The isoWeek plugin wraps startOf.
	if unit == isoWeekPrettyUnit {
		if isStartOf {
			return d.SetDate(d.Date() - (d.IsoWeekday() - 1)).StartOf("day")
		}
		return d.SetDate(d.Date() - 1 - (d.IsoWeekday() - 1) + 7).EndOf("day")
	}

	instanceFactory := func(day, m float64) *Dayjs {
		ins := d.f.wrap(js.NewDateLocal(d.f.loc, d.y, m, day))
		if isStartOf {
			return ins
		}
		return ins.EndOf(unitD)
	}

	instanceFactorySet := func(slice int) *Dayjs {
		args := []float64{0, 0, 0, 0}
		if !isStartOf {
			args = []float64{23, 59, 59, 999}
		}
		args = args[slice:]
		date := d.d.Clone()
		var t float64
		switch slice {
		case 0:
			t = date.SetHours(args[0], args[1:]...)
		case 1:
			t = date.SetMinutes(args[0], args[1:]...)
		case 2:
			t = date.SetSeconds(args[0], args[1:]...)
		default:
			t = date.SetMilliseconds(args[0])
		}
		return d.f.FromTime(t)
	}

	switch unit {
	case unitY:
		if isStartOf {
			return instanceFactory(1, 0)
		}
		return instanceFactory(31, 11)

	case unitM:
		if isStartOf {
			return instanceFactory(1, d.M)
		}
		return instanceFactory(0, d.M+1)

	case unitW:
		weekStart := 0.0
		gap := d.W
		if d.W < weekStart {
			gap = d.W + 7
		}
		gap -= weekStart
		if isStartOf {
			return instanceFactory(d.D-gap, d.M)
		}
		return instanceFactory(d.D+(6-gap), d.M)

	case unitD, unitDate:
		return instanceFactorySet(0)

	case unitH:
		return instanceFactorySet(1)

	case unitMin:
		return instanceFactorySet(2)

	case unitS:
		return instanceFactorySet(3)
	}
	return d.Clone()
}

// set is $set(units, int): the private set, which changes the receiver.
func (d *Dayjs) set(units string, v float64) *Dayjs {
	unit := prettyUnit(units)
	arg := v
	if unit == unitD {
		arg = d.D + (v - d.W)
	}

	if unit == unitM || unit == unitY {
		date := d.Clone().Set(unitDate, 1)
		if unit == unitM {
			date.d.SetMonth(arg)
		} else {
			date.d.SetFullYear(arg)
		}
		date.init()
		d.d = date.Set(unitDate, js.Min(d.D, date.DaysInMonth())).d
	} else {
		switch unit {
		case unitD, unitDate:
			d.d.SetDate(arg)
		case unitH:
			d.d.SetHours(arg)
		case unitMin:
			d.d.SetMinutes(arg)
		case unitS:
			d.d.SetSeconds(arg)
		case unitMS:
			d.d.SetMilliseconds(arg)
		}
	}

	d.init()
	return d
}

// Set is set(unit, value).
func (d *Dayjs) Set(units string, v float64) *Dayjs {
	return d.Clone().set(units, v)
}

// SetDate is date(value) as a setter: the day of the month.
func (d *Dayjs) SetDate(v float64) *Dayjs { return d.Set(unitDate, v) }

// SetDay is day(value) as a setter: the day of the week.
func (d *Dayjs) SetDay(v float64) *Dayjs { return d.Set(unitD, v) }

// SetYear is year(value) as a setter.
func (d *Dayjs) SetYear(v float64) *Dayjs { return d.Set(unitY, v) }

// SetMonth is month(value) as a setter.
func (d *Dayjs) SetMonth(v float64) *Dayjs { return d.Set(unitM, v) }

// Add is add(number, units). The units are the names that dayjs accepts,
// for example "d", "day", "days", "M", "ms".
func (d *Dayjs) Add(number float64, units string) *Dayjs {
	unit := prettyUnit(units)

	instanceFactorySet := func(n float64) *Dayjs {
		// Upstream makes a copy before and after: var d = dayjs(this);
		// Utils.w(d.date(...), this). The copies have the same value.
		return d.SetDate(d.Date() + js.Round(n*number))
	}

	if unit == unitM {
		return d.Set(unitM, d.M+number)
	}

	if unit == unitY {
		return d.Set(unitY, d.y+number)
	}

	if unit == unitD {
		return instanceFactorySet(1)
	}

	if unit == unitW {
		return instanceFactorySet(7)
	}

	step := 1.0 // ms
	switch unit {
	case unitMin:
		step = millisecondsAMinute
	case unitH:
		step = millisecondsAnHour
	case unitS:
		step = millisecondsASecond
	}

	nextTimeStamp := d.d.GetTime() + float64(number*step)
	return d.f.FromTime(nextTimeStamp)
}

// Subtract is subtract(number, units).
func (d *Dayjs) Subtract(number float64, units string) *Dayjs {
	return d.Add(number*-1, units)
}

// UtcOffset is utcOffset(): the offset of local time from UTC in minutes,
// rounded to 15 minutes.
func (d *Dayjs) UtcOffset() float64 {
	return -js.Round(d.d.GetTimezoneOffset()/15) * 15
}

// Diff is diff(input, units, float).
func (d *Dayjs) Diff(input *Dayjs, units string, float bool) float64 {
	unit := prettyUnit(units)
	that := input.Clone()
	zoneDelta := float64((that.UtcOffset() - d.UtcOffset()) * millisecondsAMinute)
	diff := d.ValueOf() - that.ValueOf()

	getMonth := func() float64 { return monthDiff(d, that) }

	var result float64

	switch unit {
	case unitY:
		result = getMonth() / 12

	case unitM:
		result = getMonth()

	case unitQ:
		result = getMonth() / 3

	case unitW:
		result = (diff - zoneDelta) / millisecondsAWeek

	case unitD:
		result = (diff - zoneDelta) / millisecondsADay

	case unitH:
		result = diff / millisecondsAnHour

	case unitMin:
		result = diff / millisecondsAMinute

	case unitS:
		result = diff / millisecondsASecond

	default:
		result = diff // milliseconds
	}

	if float {
		return result
	}
	return absFloor(result)
}

// DaysInMonth is daysInMonth().
func (d *Dayjs) DaysInMonth() float64 {
	return d.EndOf(unitM).D
}

// formatToken finds the token of REGEX_FORMAT that starts at s[i]:
//
//	/\[([^\]]+)]|YYYY|YY|M{1,4}|D{1,2}|d{1,4}|H{1,2}|h{1,2}|a|A|m{1,2}|s{1,2}|Z{1,2}|SSS/g
//
// It returns the length of the match, or 0. For the bracket form, escaped
// is the text between the brackets.
func formatToken(s string, i int) (n int, escaped string, isEscape bool) {
	run := func(c byte, max int) int {
		k := 0
		for k < max && i+k < len(s) && s[i+k] == c {
			k++
		}
		return k
	}
	switch c := s[i]; c {
	case '[':
		if end := bracketEnd(s, i); end > 0 {
			return end - i, s[i+1 : end-1], true
		}
	case 'Y':
		if k := run('Y', 4); k == 4 {
			return 4, "", false
		} else if k >= 2 {
			return 2, "", false
		}
	case 'M', 'd':
		return run(c, 4), "", false
	case 'D', 'H', 'h', 'm', 's', 'Z':
		return run(c, 2), "", false
	case 'a', 'A':
		return 1, "", false
	case 'S':
		if run('S', 3) == 3 {
			return 3, "", false
		}
	}
	return 0, "", false
}

// bracketEnd matches /\[[^\]]+]/ at s[i] and returns the index after the
// match, or 0.
func bracketEnd(s string, i int) int {
	if s[i] != '[' {
		return 0
	}
	j := strings.IndexByte(s[i+1:], ']')
	if j <= 0 {
		return 0
	}
	return i + 1 + j + 1
}

// Format is format(formatStr), with the advancedFormat plugin. An empty
// format is the default format.
func (d *Dayjs) Format(formatStr string) (string, error) {
	if !d.IsValid() {
		return invalidDateString, nil
	}
	result, err := d.advancedFormat(formatStr)
	if err != nil {
		return "", err
	}
	return d.format(result), nil
}

// format is the format function of the core.
func (d *Dayjs) format(formatStr string) string {
	if !d.IsValid() {
		return invalidDateString
	}
	str := formatStr
	if str == "" {
		str = formatDefault
	}
	// Upstream computes the zone text first. Here it is computed when a
	// token needs it, because it needs a lookup in the time zone data.
	zoneStr := func() string { return padZoneStr(d) }
	H, m, M := d.H, d.m, d.M

	getH := func(num int) string {
		h := math.Mod(H, 12)
		if h == 0 {
			h = 12
		}
		return padNum(h, num, "0")
	}

	meridiemFunc := func(hour, _ float64, isLowercase bool) string {
		m := "PM"
		if hour < 12 {
			m = "AM"
		}
		if isLowercase {
			return strings.ToLower(m)
		}
		return m
	}

	matches := func(match string) (string, bool) {
		switch match {
		case "YY":
			y := numStr(d.y)
			return js.StringSliceFrom(y, -2), true
		case "YYYY":
			return padNum(d.y, 4, "0"), true
		case "M":
			return numStr(M + 1), true
		case "MM":
			return padNum(M+1, 2, "0"), true
		case "MMM":
			return js.StringSlice(enMonths[int(M)], 0, 3), true
		case "MMMM":
			return enMonths[int(M)], true
		case "D":
			return numStr(d.D), true
		case "DD":
			return padNum(d.D, 2, "0"), true
		case "d":
			return numStr(d.W), true
		case "dd":
			return js.StringSlice(enWeekdays[int(d.W)], 0, 2), true
		case "ddd":
			return js.StringSlice(enWeekdays[int(d.W)], 0, 3), true
		case "dddd":
			return enWeekdays[int(d.W)], true
		case "H":
			return numStr(H), true
		case "HH":
			return padNum(H, 2, "0"), true
		case "h":
			return getH(1), true
		case "hh":
			return getH(2), true
		case "a":
			return meridiemFunc(H, m, true), true
		case "A":
			return meridiemFunc(H, m, false), true
		case "m":
			return numStr(m), true
		case "mm":
			return padNum(m, 2, "0"), true
		case "s":
			return numStr(d.s), true
		case "ss":
			return padNum(d.s, 2, "0"), true
		case "SSS":
			return padNum(d.ms, 3, "0"), true
		case "Z":
			return zoneStr(), true
		}
		return "", false
	}

	var sb strings.Builder
	for i := 0; i < len(str); {
		n, escaped, isEscape := formatToken(str, i)
		if n == 0 {
			sb.WriteByte(str[i])
			i++
			continue
		}
		if isEscape {
			sb.WriteString(escaped)
		} else if r, ok := matches(str[i : i+n]); ok {
			sb.WriteString(r)
		} else {
			// 'ZZ'
			sb.WriteString(strings.Replace(zoneStr(), ":", "", 1))
		}
		i += n
	}
	return sb.String()
}
