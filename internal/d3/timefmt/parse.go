// Ported from d3-time-format 4.1.0, d3-time-format/src/locale.js (the parser)
// (ISC).

package timefmt

import (
	"math"
	"regexp"
	"strings"
	stdtime "time"

	d3time "github.com/bvolpato/mermaid-go-renderer/internal/d3/time"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// fields is the object that the parse directives fill. A has* flag is the
// JavaScript test "X" in d.
type fields struct {
	y, m, d, H, M, S, L float64
	hasM                bool // false while d.m is undefined
	Z, p, q, Q, s       float64
	u, U, V, w, W       float64

	hasZ, hasP, hasQuarter, hasQ, hasS bool
	hasu, hasU, hasV, hasw, hasW       bool
}

func newDate(y, m, d float64) fields {
	return fields{y: y, m: m, hasM: true, d: d}
}

// localDate ports localDate.
func localDate(loc *stdtime.Location, d fields) *js.Date {
	m := d.m
	if !d.hasM {
		m = math.NaN()
	}
	if 0 <= d.y && d.y < 100 {
		date := js.NewDateLocal(loc, -1, m, d.d, d.H, d.M, d.S, d.L)
		date.SetFullYear(d.y)
		return date
	}
	return js.NewDateLocal(loc, d.y, m, d.d, d.H, d.M, d.S, d.L)
}

// utcDate ports utcDate.
func utcDate(d fields) *js.Date {
	m := d.m
	if !d.hasM {
		m = math.NaN()
	}
	if 0 <= d.y && d.y < 100 {
		date := js.NewDate(stdtime.UTC, js.DateUTC(-1, m, d.d, d.H, d.M, d.S, d.L))
		date.SetUTCFullYear(d.y)
		return date
	}
	return js.NewDate(stdtime.UTC, js.DateUTC(d.y, m, d.d, d.H, d.M, d.S, d.L))
}

// Parse is locale.parse(specifier). The second result of the parser is false
// where d3 returns null.
func (l *Locale) Parse(loc *stdtime.Location, specifier string) func(s string) (float64, bool) {
	if loc == nil {
		loc = stdtime.UTC
	}
	return l.newParse(loc, []rune(specifier), false)
}

// UTCParse is locale.utcParse(specifier).
func (l *Locale) UTCParse(specifier string) func(s string) (float64, bool) {
	return l.newParse(stdtime.UTC, []rune(specifier), true)
}

// newParse ports newParse.
func (l *Locale) newParse(loc *stdtime.Location, specifier []rune, Z bool) func(s string) (float64, bool) {
	local := d3time.Local(loc)
	utc := d3time.UTC()
	return func(text string) (float64, bool) {
		str := []rune(text)
		d := fields{y: 1900, d: 1}
		i := l.parseSpecifier(&d, specifier, str, 0)
		if i != len(str) {
			return 0, false
		}

		// If a UNIX timestamp is specified, return it.
		if d.hasQ {
			return js.TimeClip(d.Q), true
		}
		if d.hasS {
			return js.TimeClip(float64(d.s*1000) + d.L), true
		}

		// If this is utcParse, never use the local timezone.
		if Z && !d.hasZ {
			d.Z, d.hasZ = 0, true
		}

		// The am-pm flag is 0 for AM, and 1 for PM.
		if d.hasP {
			d.H = math.Mod(d.H, 12) + float64(d.p*12)
		}

		// If the month was not specified, inherit from the quarter.
		if !d.hasM {
			if d.hasQuarter {
				d.m = d.q
			} else {
				d.m = 0
			}
			d.hasM = true
		}

		// Convert day-of-week and week-of-year to day-of-year.
		if d.hasV {
			if d.V < 1 || d.V > 53 {
				return 0, false
			}
			if !d.hasw {
				d.w, d.hasw = 1, true
			}
			if d.hasZ {
				week := utcDate(newDate(d.y, 0, 1))
				day := week.GetUTCDay()
				var w float64
				if day > 4 || day == 0 {
					w = utc.Monday.Ceil(week.GetTime())
				} else {
					w = utc.Monday.Floor(week.GetTime())
				}
				w = utc.Day.Offset(w, float64((d.V-1)*7))
				wd := js.NewDate(stdtime.UTC, w)
				d.y = wd.GetUTCFullYear()
				d.m = wd.GetUTCMonth()
				d.d = wd.GetUTCDate() + math.Mod(d.w+6, 7)
			} else {
				week := localDate(loc, newDate(d.y, 0, 1))
				day := week.GetDay()
				var w float64
				if day > 4 || day == 0 {
					w = local.Monday.Ceil(week.GetTime())
				} else {
					w = local.Monday.Floor(week.GetTime())
				}
				w = local.Day.Offset(w, float64((d.V-1)*7))
				wd := js.NewDate(loc, w)
				d.y = wd.GetFullYear()
				d.m = wd.GetMonth()
				d.d = wd.GetDate() + math.Mod(d.w+6, 7)
			}
		} else if d.hasW || d.hasU {
			if !d.hasw {
				switch {
				case d.hasu:
					d.w = math.Mod(d.u, 7)
				case d.hasW:
					d.w = 1
				default:
					d.w = 0
				}
				d.hasw = true
			}
			var day float64
			if d.hasZ {
				day = utcDate(newDate(d.y, 0, 1)).GetUTCDay()
			} else {
				day = localDate(loc, newDate(d.y, 0, 1)).GetDay()
			}
			d.m = 0
			if d.hasW {
				d.d = math.Mod(d.w+6, 7) + float64(d.W*7) - math.Mod(day+5, 7)
			} else {
				d.d = d.w + float64(d.U*7) - math.Mod(day+6, 7)
			}
		}

		// If a time zone is specified, all fields are interpreted as UTC and then
		// offset according to the specified time zone.
		if d.hasZ {
			d.H += float64(js.ToInt32(d.Z / 100))
			d.M += math.Mod(d.Z, 100)
			return utcDate(d).GetTime(), true
		}

		// Otherwise, all fields are in local time.
		return localDate(loc, d).GetTime(), true
	}
}

// parseSpecifier ports parseSpecifier. It returns the index after the parsed
// text, or -1.
func (l *Locale) parseSpecifier(d *fields, specifier, str []rune, j int) int {
	i, n, m := 0, len(specifier), len(str)
	for i < n {
		if j >= m {
			return -1
		}
		c := specifier[i]
		i++
		if c == '%' {
			if i >= n {
				return -1
			}
			c = specifier[i]
			i++
			if _, isPad := pads[c]; isPad {
				if i >= n {
					return -1
				}
				c = specifier[i]
				i++
			}
			j = l.parseDirective(c, d, str, j)
			if j < 0 {
				return -1
			}
		} else {
			if c != str[j] {
				return -1
			}
			j++
		}
	}
	return j
}

func isSpace(c rune) bool {
	switch c {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return c >= 0x2000 && c <= 0x200a
}

// number ports numberRe.exec(string.slice(i, i + width)) with the expression
// /^\s*\d+/. A width below zero means no limit. It returns the number and the
// length of the match, or a length of -1.
func number(str []rune, i, width int) (float64, int) {
	end := len(str)
	if width >= 0 && i+width < end {
		end = i + width
	}
	k := i
	for k < end && isSpace(str[k]) {
		k++
	}
	start := k
	for k < end && str[k] >= '0' && str[k] <= '9' {
		k++
	}
	if k == start {
		return 0, -1
	}
	return js.Number(string(str[start:k])), k - i
}

var zoneRe = regexp.MustCompile(`^(Z)|([+-]\d\d)(?::?(\d\d))?`)

// matchName ports the name expressions: /^(?:name0|name1|...)/i followed by a
// lookup of the lower-case match. It returns the index of the name and the
// length of the match, or a length of -1.
func matchName(names []string, str []rune, i int) (float64, int) {
	for _, name := range names {
		r := []rune(name)
		if i+len(r) > len(str) {
			continue
		}
		if !strings.EqualFold(string(str[i:i+len(r)]), name) {
			continue
		}
		// formatLookup is a Map: the last name with the same key wins.
		key := strings.ToLower(string(str[i : i+len(r)]))
		index := -1
		for k, other := range names {
			if strings.ToLower(other) == key {
				index = k
			}
		}
		if index < 0 {
			return math.NaN(), len(r)
		}
		return float64(index), len(r)
	}
	return 0, -1
}

// parseDirective applies a parse directive at index i. It returns the index
// after the parsed text, or -1.
func (l *Locale) parseDirective(c rune, d *fields, str []rune, i int) int {
	def := &l.def
	num := func(width int, set func(v float64)) int {
		v, n := number(str, i, width)
		if n < 0 {
			return -1
		}
		set(v)
		return i + n
	}
	names := func(list []string, set func(v float64)) int {
		v, n := matchName(list, str, i)
		if n < 0 {
			return -1
		}
		set(v)
		return i + n
	}
	switch c {
	case 'a': // parseShortWeekday
		return names(def.ShortDays[:], func(v float64) { d.w, d.hasw = v, true })
	case 'A': // parseWeekday
		return names(def.Days[:], func(v float64) { d.w, d.hasw = v, true })
	case 'b': // parseShortMonth
		return names(def.ShortMonths[:], func(v float64) { d.m, d.hasM = v, true })
	case 'B': // parseMonth
		return names(def.Months[:], func(v float64) { d.m, d.hasM = v, true })
	case 'c': // parseLocaleDateTime
		return l.parseSpecifier(d, []rune(def.DateTime), str, i)
	case 'd', 'e': // parseDayOfMonth
		return num(2, func(v float64) { d.d = v })
	case 'f': // parseMicroseconds
		return num(6, func(v float64) { d.L = math.Floor(v / 1000) })
	case 'g', 'y': // parseYear
		return num(2, func(v float64) {
			if v > 68 {
				d.y = v + 1900
			} else {
				d.y = v + 2000
			}
		})
	case 'G', 'Y': // parseFullYear
		return num(4, func(v float64) { d.y = v })
	case 'H', 'I': // parseHour24
		return num(2, func(v float64) { d.H = v })
	case 'j': // parseDayOfYear
		return num(3, func(v float64) { d.m, d.hasM, d.d = 0, true, v })
	case 'L': // parseMilliseconds
		return num(3, func(v float64) { d.L = v })
	case 'm': // parseMonthNumber
		return num(2, func(v float64) { d.m, d.hasM = v-1, true })
	case 'M': // parseMinutes
		return num(2, func(v float64) { d.M = v })
	case 'p': // parsePeriod
		return names(def.Periods[:], func(v float64) { d.p, d.hasP = v, true })
	case 'q': // parseQuarter
		return num(1, func(v float64) { d.q, d.hasQuarter = float64(v*3)-3, true })
	case 'Q': // parseUnixTimestamp
		return num(-1, func(v float64) { d.Q, d.hasQ = v, true })
	case 's': // parseUnixTimestampSeconds
		return num(-1, func(v float64) { d.s, d.hasS = v, true })
	case 'S': // parseSeconds
		return num(2, func(v float64) { d.S = v })
	case 'u': // parseWeekdayNumberMonday
		return num(1, func(v float64) { d.u, d.hasu = v, true })
	case 'U': // parseWeekNumberSunday
		return num(2, func(v float64) { d.U, d.hasU = v, true })
	case 'V': // parseWeekNumberISO
		return num(2, func(v float64) { d.V, d.hasV = v, true })
	case 'w': // parseWeekdayNumberSunday
		return num(1, func(v float64) { d.w, d.hasw = v, true })
	case 'W': // parseWeekNumberMonday
		return num(2, func(v float64) { d.W, d.hasW = v, true })
	case 'x': // parseLocaleDate
		return l.parseSpecifier(d, []rune(def.Date), str, i)
	case 'X': // parseLocaleTime
		return l.parseSpecifier(d, []rune(def.Time), str, i)
	case 'Z': // parseZone
		end := i + 6
		if end > len(str) {
			end = len(str)
		}
		n := zoneRe.FindStringSubmatch(string(str[i:end]))
		if n == nil {
			return -1
		}
		if n[1] != "" {
			d.Z = 0
		} else {
			minutes := n[3]
			if minutes == "" {
				minutes = "00"
			}
			d.Z = -js.Number(n[2] + minutes)
		}
		d.hasZ = true
		return i + len([]rune(n[0]))
	case '%': // parseLiteralPercent
		if i < len(str) && str[i] == '%' {
			return i + 1
		}
		return -1
	}
	return -1
}
