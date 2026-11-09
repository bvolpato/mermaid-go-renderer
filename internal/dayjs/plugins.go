// Ported from dayjs 1.11.21, dayjs/esm/plugin/isoWeek/index.js and
// dayjs/esm/plugin/advancedFormat/index.js (MIT).

package dayjs

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

const isoWeekPrettyUnit = "isoweek"

// getYearFirstThursday is getYearFirstThursday(year, isUtc) of the isoWeek
// plugin.
func (f *Factory) getYearFirstThursday(year float64) *Dayjs {
	yearFirstDay := f.Now().SetYear(year).StartOf(unitY)
	addDiffDays := 4 - yearFirstDay.IsoWeekday()

	if yearFirstDay.IsoWeekday() > 4 {
		addDiffDays += 7
	}

	return yearFirstDay.Add(addDiffDays, unitD)
}

func getCurrentWeekThursday(ins *Dayjs) *Dayjs {
	return ins.Add(4-ins.IsoWeekday(), unitD)
}

// IsoWeekYear is isoWeekYear().
func (d *Dayjs) IsoWeekYear() float64 {
	nowWeekThursday := getCurrentWeekThursday(d)
	return nowWeekThursday.Year()
}

// IsoWeek is isoWeek() as a getter.
func (d *Dayjs) IsoWeek() float64 {
	nowWeekThursday := getCurrentWeekThursday(d)
	diffWeekThursday := d.f.getYearFirstThursday(d.IsoWeekYear())
	return nowWeekThursday.Diff(diffWeekThursday, unitW, false) + 1
}

// SetIsoWeek is isoWeek(week) as a setter.
func (d *Dayjs) SetIsoWeek(week float64) *Dayjs {
	return d.Add((week-d.IsoWeek())*7, unitD)
}

// IsoWeekday is isoWeekday() as a getter: 1 is Monday and 7 is Sunday.
func (d *Dayjs) IsoWeekday() float64 {
	// this.day() || 7
	if d.W == 0 || math.IsNaN(d.W) {
		return 7
	}
	return d.W
}

// SetIsoWeekday is isoWeekday(week) as a setter.
func (d *Dayjs) SetIsoWeekday(week float64) *Dayjs {
	if m := math.Mod(d.Day(), 7); m != 0 && !math.IsNaN(m) {
		return d.SetDay(week)
	}
	return d.SetDay(week - 7)
}

// advancedToken finds the token of the advancedFormat plugin that starts at
// s[i]:
//
//	/\[([^\]]+)]|Q|wo|ww|w|WW|W|zzz|z|gggg|GGGG|Do|X|x|k{1,2}|S/g
//
// It returns the length of the match, or 0.
func advancedToken(s string, i int) int {
	has := func(tok string) bool { return strings.HasPrefix(s[i:], tok) }
	switch s[i] {
	case '[':
		if end := bracketEnd(s, i); end > 0 {
			return end - i
		}
	case 'Q', 'X', 'x', 'S':
		return 1
	case 'w':
		if has("wo") || has("ww") {
			return 2
		}
		return 1
	case 'W':
		if has("WW") {
			return 2
		}
		return 1
	case 'z':
		if has("zzz") {
			return 3
		}
		return 1
	case 'g':
		if has("gggg") {
			return 4
		}
	case 'G':
		if has("GGGG") {
			return 4
		}
	case 'D':
		if has("Do") {
			return 2
		}
	case 'k':
		if has("kk") {
			return 2
		}
		return 1
	}
	return 0
}

// advancedFormat is the first pass of format(formatStr) of the
// advancedFormat plugin: it replaces the tokens of the plugin. The result
// goes to the format function of the core.
func (d *Dayjs) advancedFormat(formatStr string) (string, error) {
	str := formatStr
	if str == "" {
		str = formatDefault
	}
	var sb strings.Builder
	for i := 0; i < len(str); {
		n := advancedToken(str, i)
		if n == 0 {
			sb.WriteByte(str[i])
			i++
			continue
		}
		match := str[i : i+n]
		i += n
		switch match {
		case "Q":
			sb.WriteString(js.Str(math.Ceil((d.M + 1) / 3)))

		case "Do":
			sb.WriteString(enOrdinal(d.D))

		case "gggg":
			// The weekYear plugin is not loaded.
			return "", &TypeError{"this.weekYear is not a function"}

		case "GGGG":
			sb.WriteString(js.Str(d.IsoWeekYear()))

		case "wo", "w", "ww":
			// The weekOfYear plugin is not loaded.
			return "", &TypeError{"this.week is not a function"}

		case "W":
			sb.WriteString(padNum(d.IsoWeek(), 1, "0"))

		case "WW":
			sb.WriteString(padNum(d.IsoWeek(), 2, "0"))

		case "k", "kk":
			h := d.H
			if h == 0 {
				h = 24
			}
			width := 1
			if match == "kk" {
				width = 2
			}
			sb.WriteString(padNum(h, width, "0"))

		case "X":
			sb.WriteString(js.Str(math.Floor(d.d.GetTime() / 1000)))

		case "x":
			sb.WriteString(js.Str(d.d.GetTime()))

		case "z", "zzz":
			// The timezone plugin is not loaded.
			return "", &TypeError{"this.offsetName is not a function"}

		default:
			sb.WriteString(match)
		}
	}
	return sb.String(), nil
}
