// Ported from dayjs 1.11.21, dayjs/esm/plugin/duration/index.js (MIT).

package dayjs

import (
	"math"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

const (
	millisecondsAYear  = millisecondsADay * 365
	millisecondsAMonth = millisecondsAYear / 12
)

var durationRegexParse = regexp2.MustCompile(`^(-|\+)?P(?:([-+]?[0-9,.]*)Y)?(?:([-+]?[0-9,.]*)M)?(?:([-+]?[0-9,.]*)W)?(?:([-+]?[0-9,.]*)D)?(?:T(?:([-+]?[0-9,.]*)H)?(?:([-+]?[0-9,.]*)M)?(?:([-+]?[0-9,.]*)S)?)?$`, regexp2.ECMAScript)

// unitToMS is unitToMS. The result is NaN for a name that is not a unit, as
// the undefined of the lookup gives upstream.
func unitToMS(unit string) float64 {
	switch unit {
	case "years":
		return millisecondsAYear
	case "months":
		return millisecondsAMonth
	case "days":
		return millisecondsADay
	case "hours":
		return millisecondsAnHour
	case "minutes":
		return millisecondsAMinute
	case "seconds":
		return millisecondsASecond
	case "milliseconds":
		return 1
	case "weeks":
		return millisecondsAWeek
	}
	return math.NaN()
}

func durationPrettyUnit(unit string) string {
	return prettyUnit(unit) + "s"
}

func roundNumber(number float64) float64 {
	if number < 0 {
		return math.Ceil(number)
	}
	return math.Floor(number)
}

// Duration is a dayjs duration object.
type Duration struct {
	// d is $d: the parts of the duration by unit name, in insertion order.
	d *js.Obj[float64]
	// ms is $ms.
	ms float64
}

// DurationMs is dayjs.duration(number): a duration in milliseconds.
func (f *Factory) DurationMs(ms float64) *Duration {
	d := &Duration{d: js.NewObj[float64](), ms: ms}
	d.parseFromMilliseconds()
	return d
}

// DurationUnit is dayjs.duration(number, unit).
func (f *Factory) DurationUnit(input float64, unit string) *Duration {
	return f.DurationMs(input * unitToMS(durationPrettyUnit(unit)))
}

// DurationOf is dayjs.duration(object), for example {days: 2, hours: 3}.
// The keys are unit names in any form that dayjs accepts.
func (f *Factory) DurationOf(input *js.Obj[float64]) *Duration {
	d := &Duration{d: js.NewObj[float64]()}
	input.Each(func(k string, v float64) {
		d.d.Set(durationPrettyUnit(k), v)
	})
	d.calMilliseconds()
	return d
}

// DurationISO is dayjs.duration(string) with an ISO 8601 duration, for
// example "P1DT2H". A string that does not match gives an empty duration,
// whose value is NaN (undefined upstream).
func (f *Factory) DurationISO(input string) *Duration {
	d := &Duration{d: js.NewObj[float64](), ms: math.NaN()}
	m, err := durationRegexParse.FindStringMatch(input)
	if err != nil || m == nil {
		return d
	}
	names := []string{"years", "months", "weeks", "days", "hours", "minutes", "seconds"}
	for i, name := range names {
		g := m.GroupByNumber(i + 2)
		v := 0.0
		if len(g.Captures) > 0 {
			v = js.Number(g.String())
		}
		d.d.Set(name, v)
	}
	d.calMilliseconds()
	return d
}

func (d *Duration) calMilliseconds() {
	total := 0.0
	d.d.Each(func(unit string, v float64) {
		// (this.$d[unit] || 0) * unitToMS[unit]
		if v == 0 || math.IsNaN(v) {
			v = 0
		}
		total = total + float64(v*unitToMS(unit))
	})
	d.ms = total
}

func (d *Duration) parseFromMilliseconds() {
	ms := d.ms
	d.d.Set("years", roundNumber(ms/millisecondsAYear))
	ms = math.Mod(ms, millisecondsAYear)
	d.d.Set("months", roundNumber(ms/millisecondsAMonth))
	ms = math.Mod(ms, millisecondsAMonth)
	d.d.Set("days", roundNumber(ms/millisecondsADay))
	ms = math.Mod(ms, millisecondsADay)
	d.d.Set("hours", roundNumber(ms/millisecondsAnHour))
	ms = math.Mod(ms, millisecondsAnHour)
	d.d.Set("minutes", roundNumber(ms/millisecondsAMinute))
	ms = math.Mod(ms, millisecondsAMinute)
	d.d.Set("seconds", roundNumber(ms/millisecondsASecond))
	ms = math.Mod(ms, millisecondsASecond)
	d.d.Set("milliseconds", ms)
}

// part reads this.$d[name]: the second result is false for undefined.
func (d *Duration) part(name string) (float64, bool) {
	return d.d.Lookup(name)
}

type numberUnitFormat struct {
	negative bool
	format   string
}

func getNumberUnitFormat(number float64, has bool, unit string) numberUnitFormat {
	if !has || number == 0 || math.IsNaN(number) {
		return numberUnitFormat{}
	}
	if number < 0 {
		return numberUnitFormat{negative: true, format: js.Str(math.Abs(number)) + unit}
	}
	return numberUnitFormat{format: js.Str(number) + unit}
}

// ToISOString is toISOString().
func (d *Duration) ToISOString() string {
	years, hasYears := d.part("years")
	Y := getNumberUnitFormat(years, hasYears, "Y")
	months, hasMonths := d.part("months")
	M := getNumberUnitFormat(months, hasMonths, "M")
	// +this.$d.days || 0
	days, _ := d.part("days")
	if math.IsNaN(days) {
		days = 0
	}

	if weeks, ok := d.part("weeks"); ok && weeks != 0 && !math.IsNaN(weeks) {
		days += float64(weeks * 7)
	}

	D := getNumberUnitFormat(days, true, "D")
	hours, hasHours := d.part("hours")
	H := getNumberUnitFormat(hours, hasHours, "H")
	minutes, hasMinutes := d.part("minutes")
	m := getNumberUnitFormat(minutes, hasMinutes, "M")
	// this.$d.seconds || 0
	seconds, _ := d.part("seconds")
	if math.IsNaN(seconds) {
		seconds = 0
	}

	if milliseconds, ok := d.part("milliseconds"); ok && milliseconds != 0 && !math.IsNaN(milliseconds) {
		seconds += milliseconds / 1000
		seconds = js.Round(seconds*1000) / 1000
	}

	S := getNumberUnitFormat(seconds, true, "S")
	negativeMode := Y.negative || M.negative || D.negative || H.negative || m.negative || S.negative
	T := ""
	if H.format != "" || m.format != "" || S.format != "" {
		T = "T"
	}
	P := ""
	if negativeMode {
		P = "-"
	}
	result := P + "P" + Y.format + M.format + D.format + T + H.format + m.format + S.format
	if result == "P" || result == "-P" {
		return "P0D"
	}
	return result
}

// durationToken finds the token of DURATION_REGEX_FORMAT at s[i]:
//
//	/\[([^\]]+)]|YYYY|YY|Y|M{1,2}|D{1,2}|H{1,2}|m{1,2}|s{1,2}|SSS/g
func durationToken(s string, i int) (n int, escaped string, isEscape bool) {
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
		k := run('Y', 4)
		if k == 3 {
			k = 2
		}
		return k, "", false
	case 'M', 'D', 'H', 'm', 's':
		return run(c, 2), "", false
	case 'S':
		if run('S', 3) == 3 {
			return 3, "", false
		}
	}
	return 0, "", false
}

// Format is format(formatStr). An empty format is the default format.
func (d *Duration) Format(formatStr string) string {
	str := formatStr
	if str == "" {
		str = "YYYY-MM-DDTHH:mm:ss"
	}
	// String(undefined) is "undefined", also after Utils.s.
	val := func(name string, width int) string {
		v, ok := d.part(name)
		if !ok {
			return padStart("undefined", width, "0")
		}
		return padNum(v, width, "0")
	}
	var sb strings.Builder
	for i := 0; i < len(str); {
		n, escaped, isEscape := durationToken(str, i)
		if n == 0 {
			sb.WriteByte(str[i])
			i++
			continue
		}
		if isEscape {
			sb.WriteString(escaped)
		} else {
			switch str[i : i+n] {
			case "Y":
				sb.WriteString(val("years", 0))
			case "YY":
				sb.WriteString(val("years", 2))
			case "YYYY":
				sb.WriteString(val("years", 4))
			case "M":
				sb.WriteString(val("months", 0))
			case "MM":
				sb.WriteString(val("months", 2))
			case "D":
				sb.WriteString(val("days", 0))
			case "DD":
				sb.WriteString(val("days", 2))
			case "H":
				sb.WriteString(val("hours", 0))
			case "HH":
				sb.WriteString(val("hours", 2))
			case "m":
				sb.WriteString(val("minutes", 0))
			case "mm":
				sb.WriteString(val("minutes", 2))
			case "s":
				sb.WriteString(val("seconds", 0))
			case "ss":
				sb.WriteString(val("seconds", 2))
			case "SSS":
				sb.WriteString(val("milliseconds", 3))
			}
		}
		i += n
	}
	return sb.String()
}

// As is as(unit): the duration in the unit, with a fraction.
func (d *Duration) As(unit string) float64 {
	return d.ms / unitToMS(durationPrettyUnit(unit))
}

// Get is get(unit): one part of the duration.
func (d *Duration) Get(unit string) float64 {
	base := d.ms
	pUnit := durationPrettyUnit(unit)

	if pUnit == "milliseconds" {
		base = math.Mod(base, 1000)
	} else if pUnit == "weeks" {
		base = roundNumber(base / unitToMS(pUnit))
	} else {
		v, ok := d.part(pUnit)
		if !ok {
			return 0
		}
		base = v
	}

	// base || 0
	if base == 0 || math.IsNaN(base) {
		return 0
	}
	return base
}

// Add is add(number, unit).
func (d *Duration) Add(input float64, unit string) *Duration {
	return d.add(input*unitToMS(durationPrettyUnit(unit)), false)
}

// AddDuration is add(duration).
func (d *Duration) AddDuration(input *Duration) *Duration { return d.add(input.ms, false) }

// Subtract is subtract(number, unit).
func (d *Duration) Subtract(input float64, unit string) *Duration {
	return d.add(input*unitToMS(durationPrettyUnit(unit)), true)
}

// SubtractDuration is subtract(duration).
func (d *Duration) SubtractDuration(input *Duration) *Duration { return d.add(input.ms, true) }

func (d *Duration) add(another float64, isSubtract bool) *Duration {
	k := 1.0
	if isSubtract {
		k = -1
	}
	out := &Duration{d: js.NewObj[float64](), ms: d.ms + float64(another*k)}
	out.parseFromMilliseconds()
	return out
}

// Clone is clone().
func (d *Duration) Clone() *Duration {
	out := &Duration{d: js.NewObj[float64](), ms: d.ms}
	out.parseFromMilliseconds()
	return out
}

// ValueOf is valueOf().
func (d *Duration) ValueOf() float64 { return d.AsMilliseconds() }

// The accessors of the parts and of the total in a unit.

func (d *Duration) Milliseconds() float64   { return d.Get("milliseconds") }
func (d *Duration) AsMilliseconds() float64 { return d.As("milliseconds") }
func (d *Duration) Seconds() float64        { return d.Get("seconds") }
func (d *Duration) AsSeconds() float64      { return d.As("seconds") }
func (d *Duration) Minutes() float64        { return d.Get("minutes") }
func (d *Duration) AsMinutes() float64      { return d.As("minutes") }
func (d *Duration) Hours() float64          { return d.Get("hours") }
func (d *Duration) AsHours() float64        { return d.As("hours") }
func (d *Duration) Days() float64           { return d.Get("days") }
func (d *Duration) AsDays() float64         { return d.As("days") }
func (d *Duration) Weeks() float64          { return d.Get("weeks") }
func (d *Duration) AsWeeks() float64        { return d.As("weeks") }
func (d *Duration) Months() float64         { return d.Get("months") }
func (d *Duration) AsMonths() float64       { return d.As("months") }
func (d *Duration) Years() float64          { return d.Get("years") }
func (d *Duration) AsYears() float64        { return d.As("years") }

// AddDuration is add(duration) of a dayjs object: manipulateDuration.
func (d *Dayjs) AddDuration(duration *Duration) *Dayjs {
	return manipulateDuration(d, duration, 1)
}

// SubtractDuration is subtract(duration) of a dayjs object.
func (d *Dayjs) SubtractDuration(duration *Duration) *Dayjs {
	return manipulateDuration(d, duration, -1)
}

func manipulateDuration(date *Dayjs, duration *Duration, k float64) *Dayjs {
	return date.Add(duration.Years()*k, "y").Add(duration.Months()*k, "M").Add(duration.Days()*k, "d").
		Add(duration.Hours()*k, "h").Add(duration.Minutes()*k, "m").Add(duration.Seconds()*k, "s").
		Add(duration.Milliseconds()*k, "ms")
}
