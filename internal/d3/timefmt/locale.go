// Ported from d3-time-format 4.1.0, d3-time-format/src/locale.js and
// d3-time-format/src/defaultLocale.js (ISC).

// Package timefmt is a port of d3-time-format 4.1.0.
//
// A date is a JavaScript time value: a float64 with the milliseconds since
// the epoch in UTC, or NaN for an invalid date. d3.timeFormat and
// d3.timeParse use the local time zone of the machine; the port takes the
// zone as a *time.Location instead.
package timefmt

import (
	"math"
	"strings"
	stdtime "time"

	d3time "github.com/bvolpato/mermaid-go-renderer/internal/d3/time"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// LocaleDefinition is the argument of d3.timeFormatLocale.
type LocaleDefinition struct {
	DateTime    string // the format of %c
	Date        string // the format of %x
	Time        string // the format of %X
	Periods     [2]string
	Days        [7]string
	ShortDays   [7]string
	Months      [12]string
	ShortMonths [12]string
}

// Locale is the result of d3.timeFormatLocale.
type Locale struct {
	def LocaleDefinition
}

// NewLocale is d3.timeFormatLocale(definition).
func NewLocale(def LocaleDefinition) *Locale {
	return &Locale{def: def}
}

// defaultLocale is the en-US locale of d3-time-format.
var defaultLocale = NewLocale(LocaleDefinition{
	DateTime:    "%x, %X",
	Date:        "%-m/%-d/%Y",
	Time:        "%-I:%M:%S %p",
	Periods:     [2]string{"AM", "PM"},
	Days:        [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
	ShortDays:   [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
	Months:      [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	ShortMonths: [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
})

// Format is d3.timeFormat(specifier) with the default locale. The fields of
// a date are in the given zone; nil means UTC.
func Format(loc *stdtime.Location, specifier string) func(date float64) string {
	return defaultLocale.Format(loc, specifier)
}

// Parse is d3.timeParse(specifier) with the default locale. The second result
// of the parser is false where d3 returns null.
func Parse(loc *stdtime.Location, specifier string) func(s string) (float64, bool) {
	return defaultLocale.Parse(loc, specifier)
}

// UTCFormat is d3.utcFormat(specifier) with the default locale.
func UTCFormat(specifier string) func(date float64) string {
	return defaultLocale.UTCFormat(specifier)
}

// UTCParse is d3.utcParse(specifier) with the default locale.
func UTCParse(specifier string) func(s string) (float64, bool) {
	return defaultLocale.UTCParse(specifier)
}

// Format is locale.format(specifier).
func (l *Locale) Format(loc *stdtime.Location, specifier string) func(date float64) string {
	if loc == nil {
		loc = stdtime.UTC
	}
	f := &formatter{locale: l, loc: loc, time: d3time.Local(loc)}
	spec := []rune(specifier)
	return func(date float64) string {
		return f.format(spec, js.NewDate(loc, date))
	}
}

// UTCFormat is locale.utcFormat(specifier).
func (l *Locale) UTCFormat(specifier string) func(date float64) string {
	f := &formatter{locale: l, loc: stdtime.UTC, utc: true, time: d3time.UTC()}
	spec := []rune(specifier)
	return func(date float64) string {
		return f.format(spec, js.NewDate(stdtime.UTC, date))
	}
}

// formatter holds the directive functions of one family: local time or UTC.
type formatter struct {
	locale *Locale
	loc    *stdtime.Location
	utc    bool
	time   *d3time.Intervals
}

// The getters below select the local or the UTC method of the date.

func (f *formatter) fullYear(d *js.Date) float64 {
	if f.utc {
		return d.GetUTCFullYear()
	}
	return d.GetFullYear()
}

func (f *formatter) month(d *js.Date) float64 {
	if f.utc {
		return d.GetUTCMonth()
	}
	return d.GetMonth()
}

func (f *formatter) date(d *js.Date) float64 {
	if f.utc {
		return d.GetUTCDate()
	}
	return d.GetDate()
}

func (f *formatter) day(d *js.Date) float64 {
	if f.utc {
		return d.GetUTCDay()
	}
	return d.GetDay()
}

func (f *formatter) hours(d *js.Date) float64 {
	if f.utc {
		return d.GetUTCHours()
	}
	return d.GetHours()
}

func (f *formatter) minutes(d *js.Date) float64 {
	if f.utc {
		return d.GetUTCMinutes()
	}
	return d.GetMinutes()
}

func (f *formatter) seconds(d *js.Date) float64 {
	if f.utc {
		return d.GetUTCSeconds()
	}
	return d.GetSeconds()
}

func (f *formatter) milliseconds(d *js.Date) float64 {
	if f.utc {
		return d.GetUTCMilliseconds()
	}
	return d.GetMilliseconds()
}

// floor is interval(d) as a new date.
func (f *formatter) floor(iv *d3time.Interval, d *js.Date) *js.Date {
	return js.NewDate(f.loc, iv.Floor(d.GetTime()))
}

func (f *formatter) ceil(iv *d3time.Interval, d *js.Date) *js.Date {
	return js.NewDate(f.loc, iv.Ceil(d.GetTime()))
}

var pads = map[rune]string{'-': "", '_': " ", '0': "0"}

// format ports the function that newFormat returns.
func (f *formatter) format(specifier []rune, date *js.Date) string {
	var sb strings.Builder
	n := len(specifier)
	j := 0
	i := -1
	for i++; i < n; i++ {
		if specifier[i] == '%' {
			sb.WriteString(string(specifier[j:i]))
			// charAt gives the empty string past the end.
			var c rune
			hasC := false
			i++
			if i < n {
				c, hasC = specifier[i], true
			}
			pad, isPad := "", false
			if hasC {
				pad, isPad = pads[c]
			}
			if isPad {
				i++
				hasC = false
				if i < n {
					c, hasC = specifier[i], true
				}
			} else if hasC && c == 'e' {
				pad = " "
			} else {
				pad = "0"
			}
			if hasC {
				if s, ok := f.directive(c, date, pad); ok {
					sb.WriteString(s)
				} else {
					sb.WriteRune(c)
				}
			}
			j = i + 1
		}
	}
	if j < n {
		sb.WriteString(string(specifier[j:n]))
	}
	return sb.String()
}

// pad ports the pad function.
func pad(value float64, fill string, width int) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	s := js.Str(value)
	if length := len(s); length < width {
		s = strings.Repeat(fill, width-length) + s
	}
	return sign + s
}

// name is names[i] in JavaScript: the empty string after join for an index
// that does not exist.
func name(names []string, i float64) string {
	if i >= 0 && i < float64(len(names)) && i == math.Trunc(i) {
		return names[int(i)]
	}
	return ""
}

// truncInt32 is the JavaScript expression x | 0 (and ~~x) as a number.
func truncInt32(x float64) float64 {
	return float64(js.ToInt32(x))
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// directive applies a format directive. The result is false for a character
// that is not a directive.
func (f *formatter) directive(c rune, d *js.Date, p string) (string, bool) {
	l := &f.locale.def
	t := f.time
	switch c {
	case 'a': // formatShortWeekday
		return name(l.ShortDays[:], f.day(d)), true
	case 'A': // formatWeekday
		return name(l.Days[:], f.day(d)), true
	case 'b': // formatShortMonth
		return name(l.ShortMonths[:], f.month(d)), true
	case 'B': // formatMonth
		return name(l.Months[:], f.month(d)), true
	case 'c':
		return f.format([]rune(l.DateTime), d), true
	case 'd', 'e': // formatDayOfMonth
		return pad(f.date(d), p, 2), true
	case 'f': // formatMicroseconds
		return pad(f.milliseconds(d), p, 3) + "000", true
	case 'g': // formatYearISO
		d = f.dISO(d)
		return pad(math.Mod(f.fullYear(d), 100), p, 2), true
	case 'G': // formatFullYearISO
		d = f.dISO(d)
		return pad(math.Mod(f.fullYear(d), 10000), p, 4), true
	case 'H': // formatHour24
		return pad(f.hours(d), p, 2), true
	case 'I': // formatHour12
		h := math.Mod(f.hours(d), 12)
		if h == 0 || math.IsNaN(h) {
			h = 12
		}
		return pad(h, p, 2), true
	case 'j': // formatDayOfYear
		return pad(1+t.Day.Count(t.Year.Floor(d.GetTime()), d.GetTime()), p, 3), true
	case 'L': // formatMilliseconds
		return pad(f.milliseconds(d), p, 3), true
	case 'm': // formatMonthNumber
		return pad(f.month(d)+1, p, 2), true
	case 'M': // formatMinutes
		return pad(f.minutes(d), p, 2), true
	case 'p': // formatPeriod
		return name(l.Periods[:], b2f(f.hours(d) >= 12)), true
	case 'q': // formatQuarter
		return js.Str(1 + truncInt32(f.month(d)/3)), true
	case 'Q': // formatUnixTimestamp
		return js.Str(d.GetTime()), true
	case 's': // formatUnixTimestampSeconds
		return js.Str(math.Floor(d.GetTime() / 1000)), true
	case 'S': // formatSeconds
		return pad(f.seconds(d), p, 2), true
	case 'u': // formatWeekdayNumberMonday
		day := f.day(d)
		if day == 0 {
			day = 7
		}
		return js.Str(day), true
	case 'U': // formatWeekNumberSunday
		return pad(t.Sunday.Count(t.Year.Floor(d.GetTime())-1, d.GetTime()), p, 2), true
	case 'V': // formatWeekNumberISO
		d = f.dISO(d)
		year := f.floor(t.Year, d)
		return pad(t.Thursday.Count(year.GetTime(), d.GetTime())+b2f(f.day(year) == 4), p, 2), true
	case 'w': // formatWeekdayNumberSunday
		return js.Str(f.day(d)), true
	case 'W': // formatWeekNumberMonday
		return pad(t.Monday.Count(t.Year.Floor(d.GetTime())-1, d.GetTime()), p, 2), true
	case 'x':
		return f.format([]rune(l.Date), d), true
	case 'X':
		return f.format([]rune(l.Time), d), true
	case 'y': // formatYear
		return pad(math.Mod(f.fullYear(d), 100), p, 2), true
	case 'Y': // formatFullYear
		return pad(math.Mod(f.fullYear(d), 10000), p, 4), true
	case 'Z': // formatZone
		if f.utc {
			return "+0000", true
		}
		z := d.GetTimezoneOffset()
		sign := "-"
		if !(z > 0) {
			z = float64(z * -1)
			sign = "+"
		}
		return sign + pad(truncInt32(z/60), "0", 2) + pad(math.Mod(z, 60), "0", 2), true
	case '%': // formatLiteralPercent
		return "%", true
	}
	return "", false
}

// dISO ports dISO and UTCdISO: the Thursday of the ISO week of the date.
func (f *formatter) dISO(d *js.Date) *js.Date {
	day := f.day(d)
	if day >= 4 || day == 0 {
		return f.floor(f.time.Thursday, d)
	}
	return f.ceil(f.time.Thursday, d)
}
