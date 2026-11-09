// Ported from dayjs 1.11.21, dayjs/esm/plugin/customParseFormat/index.js,
// dayjs/esm/plugin/localizedFormat/utils.js and the function parseDate of
// dayjs/esm/index.js (MIT).

package dayjs

import (
	"errors"
	"math"

	"github.com/dlclark/regexp2"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// isJSSpaceUnit reports whether a UTF-16 code unit matches \s of a
// JavaScript regular expression.
func isJSSpaceUnit(c rune) bool {
	switch c {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return c >= 0x2000 && c <= 0x200A
}

func isDigitUnit(c rune) bool { return c >= '0' && c <= '9' }

// isSeparatorUnit matches the class [-_:/.,()\s] of formattingTokens.
func isSeparatorUnit(c rune) bool {
	switch c {
	case '-', '_', ':', '/', '.', ',', '(', ')':
		return true
	}
	return isJSSpaceUnit(c)
}

// isWordUnit matches the class [^-_:/,()\s\d] of matchWord.
func isWordUnit(c rune) bool {
	switch c {
	case '-', '_', ':', '/', ',', '(', ')':
		return false
	}
	return !isJSSpaceUnit(c) && !isDigitUnit(c)
}

// The regular expressions of the plugin. Each function finds the first
// match in part and returns its start and end, or ok false.
type matcher func(part []rune) (start, end int, ok bool)

// matchDigits is /\d{n}/ for n digits in sequence.
func matchDigits(n int) matcher {
	return func(part []rune) (int, int, bool) {
		run := 0
		for i, c := range part {
			if isDigitUnit(c) {
				run++
				if run == n {
					return i - n + 1, i + 1, true
				}
			} else {
				run = 0
			}
		}
		return 0, 0, false
	}
}

var (
	match1 = matchDigits(1) // 0 - 9
	match2 = matchDigits(2) // 00 - 99
	match3 = matchDigits(3) // 000 - 999
	match4 = matchDigits(4) // 0000 - 9999
)

// match1to2 is /\d\d?/: 0 - 99.
func match1to2(part []rune) (int, int, bool) {
	for i, c := range part {
		if isDigitUnit(c) {
			if i+1 < len(part) && isDigitUnit(part[i+1]) {
				return i, i + 2, true
			}
			return i, i + 1, true
		}
	}
	return 0, 0, false
}

// matchSigned is /[+-]?\d+/: -inf - inf.
func matchSigned(part []rune) (int, int, bool) {
	for i, c := range part {
		j := i
		if c == '+' || c == '-' {
			j++
		}
		if j < len(part) && isDigitUnit(part[j]) {
			for j < len(part) && isDigitUnit(part[j]) {
				j++
			}
			return i, j, true
		}
	}
	return 0, 0, false
}

// matchOffset is /[+-]\d\d:?(\d\d)?|Z/: +00:00 -00:00 +0000 or -0000 +00 or Z.
func matchOffset(part []rune) (int, int, bool) {
	for i, c := range part {
		if (c == '+' || c == '-') && i+2 < len(part) && isDigitUnit(part[i+1]) && isDigitUnit(part[i+2]) {
			j := i + 3
			if j < len(part) && part[j] == ':' {
				j++
			}
			if j+1 < len(part) && isDigitUnit(part[j]) && isDigitUnit(part[j+1]) {
				j += 2
			}
			return i, j, true
		}
		if c == 'Z' {
			return i, i + 1, true
		}
	}
	return 0, 0, false
}

// matchWord is /\d*[^-_:/,()\s\d]+/: a word.
func matchWord(part []rune) (int, int, bool) {
	for i := 0; i < len(part); i++ {
		j := i
		for j < len(part) && isDigitUnit(part[j]) {
			j++
		}
		if j < len(part) && isWordUnit(part[j]) {
			for j < len(part) && isWordUnit(part[j]) {
				j++
			}
			return i, j, true
		}
		// No word follows these digits. A start inside the run of digits
		// fails in the same way.
		if j > i {
			i = j - 1
		}
	}
	return 0, 0, false
}

// parsedTime is the object that the token parsers fill ("time" in
// makeParser). A field is undefined until its flag is set.
type parsedTime struct {
	year, month, day, hours, minutes, seconds, milliseconds, week                         float64
	hasYear, hasMonth, hasDay, hasHours, hasMinutes, hasSeconds, hasMilliseconds, hasWeek bool
	// dayString is true when day came from the token Do without a match in
	// the ordinals: the value is then a string of digits, which is truthy
	// also for "0".
	dayString bool

	afternoon, hasAfternoon bool

	zoneOffset float64
	hasZone    bool
}

func truthyNum(v float64, has bool) bool {
	return has && v != 0 && !math.IsNaN(v)
}

type tokenParser func(time *parsedTime, input string) error

// offsetFromString is offsetFromString.
func offsetFromString(s string) float64 {
	if s == "" {
		return 0
	}
	if s == "Z" {
		return 0
	}
	// string.match(/([+-]|\d\d)/g)
	var parts []string
	for i := 0; i < len(s); {
		if s[i] == '+' || s[i] == '-' {
			parts = append(parts, s[i:i+1])
			i++
		} else if i+1 < len(s) && isDigitUnit(rune(s[i])) && isDigitUnit(rune(s[i+1])) {
			parts = append(parts, s[i:i+2])
			i += 2
		} else {
			i++
		}
	}
	part := func(i int) float64 {
		if i < len(parts) {
			return js.Number(parts[i])
		}
		return math.NaN()
	}
	p2 := part(2)
	if p2 == 0 || math.IsNaN(p2) {
		p2 = 0
	}
	minutes := float64(part(1)*60) + p2
	if minutes == 0 {
		return 0
	}
	if parts[0] == "+" {
		return -minutes
	}
	return minutes
}

func zoneExpression(time *parsedTime, input string) error {
	time.zoneOffset = offsetFromString(input)
	time.hasZone = true
	return nil
}

// meridiemMatch is meridiemMatch for a locale without a meridiem function.
func meridiemMatch(input string, isLowerCase bool) bool {
	if isLowerCase {
		return input == "pm"
	}
	return input == "PM"
}

type expression struct {
	regex  matcher
	parser tokenParser
}

func addInput(set func(time *parsedTime, v float64)) tokenParser {
	return func(time *parsedTime, input string) error {
		set(time, js.Number(input))
		return nil
	}
}

func setSeconds(t *parsedTime, v float64) { t.seconds, t.hasSeconds = v, true }
func setMinutes(t *parsedTime, v float64) { t.minutes, t.hasMinutes = v, true }
func setHours(t *parsedTime, v float64)   { t.hours, t.hasHours = v, true }
func setDay(t *parsedTime, v float64)     { t.day, t.hasDay, t.dayString = v, true, false }
func setWeek(t *parsedTime, v float64)    { t.week, t.hasWeek = v, true }
func setMonth(t *parsedTime, v float64)   { t.month, t.hasMonth = v, true }
func setYear(t *parsedTime, v float64)    { t.year, t.hasYear = v, true }

// parseTwoDigitYear is parseTwoDigitYear.
func parseTwoDigitYear(input string) float64 {
	v := js.Number(input)
	if v > 68 {
		return v + 1900
	}
	return v + 2000
}

func indexOfString(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

var enMonthsShort = func() []string {
	out := make([]string, len(enMonths))
	for i, m := range enMonths {
		out[i] = m[:3]
	}
	return out
}()

func namedMonth(names []string) tokenParser {
	return func(time *parsedTime, input string) error {
		matchIndex := indexOfString(names, input) + 1
		if matchIndex < 1 {
			return errInvalidValue
		}
		// matchIndex % 12 || matchIndex
		setMonth(time, float64(matchIndex))
		if r := matchIndex % 12; r != 0 {
			setMonth(time, float64(r))
		}
		return nil
	}
}

// errInvalidValue stands for an exception inside parseFormattedInput. The
// plugin catches it and returns an invalid date.
var errInvalidValue = errors.New("dayjs: invalid input")

// expressions is the table of the tokens that parse a part of the input.
var expressions = map[string]expression{
	"A": {matchWord, func(time *parsedTime, input string) error {
		time.afternoon, time.hasAfternoon = meridiemMatch(input, false), true
		return nil
	}},
	"a": {matchWord, func(time *parsedTime, input string) error {
		time.afternoon, time.hasAfternoon = meridiemMatch(input, true), true
		return nil
	}},
	"Q": {match1, func(time *parsedTime, input string) error {
		setMonth(time, float64((js.Number(input)-1)*3)+1)
		return nil
	}},
	"S": {match1, func(time *parsedTime, input string) error {
		time.milliseconds, time.hasMilliseconds = js.Number(input)*100, true
		return nil
	}},
	"SS": {match2, func(time *parsedTime, input string) error {
		time.milliseconds, time.hasMilliseconds = js.Number(input)*10, true
		return nil
	}},
	"SSS": {match3, func(time *parsedTime, input string) error {
		time.milliseconds, time.hasMilliseconds = js.Number(input), true
		return nil
	}},
	"s":  {match1to2, addInput(setSeconds)},
	"ss": {match1to2, addInput(setSeconds)},
	"m":  {match1to2, addInput(setMinutes)},
	"mm": {match1to2, addInput(setMinutes)},
	"H":  {match1to2, addInput(setHours)},
	"h":  {match1to2, addInput(setHours)},
	"HH": {match1to2, addInput(setHours)},
	"hh": {match1to2, addInput(setHours)},
	"D":  {match1to2, addInput(setDay)},
	"DD": {match2, addInput(setDay)},
	"Do": {matchWord, func(time *parsedTime, input string) error {
		// input.match(/\d+/)
		units := js.UTF16Units(input)
		start := -1
		end := 0
		for i, c := range units {
			if isDigitUnit(c) {
				if start < 0 {
					start = i
				}
				end = i + 1
			} else if start >= 0 {
				break
			}
		}
		if start < 0 {
			return errInvalidValue
		}
		// The day is a string here.
		time.day, time.hasDay, time.dayString = js.Number(js.FromUTF16Units(units[start:end])), true, true

		for i := 1; i <= 31; i++ {
			o := enOrdinal(float64(i))
			if o[1:len(o)-1] == input {
				setDay(time, float64(i))
			}
		}
		return nil
	}},
	"w":    {match1to2, addInput(setWeek)},
	"ww":   {match2, addInput(setWeek)},
	"M":    {match1to2, addInput(setMonth)},
	"MM":   {match2, addInput(setMonth)},
	"MMM":  {matchWord, namedMonth(enMonthsShort)},
	"MMMM": {matchWord, namedMonth(enMonths)},
	"Y":    {matchSigned, addInput(setYear)},
	"YY": {match2, func(time *parsedTime, input string) error {
		setYear(time, parseTwoDigitYear(input))
		return nil
	}},
	"YYYY": {match4, addInput(setYear)},
	"Z":    {matchOffset, zoneExpression},
	"ZZ":   {matchOffset, zoneExpression},
}

// correctHours is correctHours.
func correctHours(time *parsedTime) {
	if time.hasAfternoon {
		if time.afternoon {
			// undefined < 12 is false.
			if time.hasHours && time.hours < 12 {
				time.hours += 12
			}
		} else if time.hasHours && time.hours == 12 {
			time.hours = 0
		}

		time.hasAfternoon = false
	}
}

// parseToken is one element of the array that makeParser builds: a literal
// text, or a token with a parser.
type parseToken struct {
	literal int // the length of a literal text in UTF-16 code units
	expr    *expression
}

// usesLocalizedFormat reports whether the function u of the localizedFormat
// plugin finds a token (LT, LTS, L, LL, l, ll, ...) in the format. The
// locale "en" has no formats object, so such a token throws upstream.
func usesLocalizedFormat(format []rune) bool {
	// /(\[[^\]]+])|(LTS?|l{1,4}|L{1,4})/g
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c == '[' {
			j := i + 1
			for j < len(format) && format[j] != ']' {
				j++
			}
			if j < len(format) && j > i+1 {
				i = j
			}
			continue
		}
		if c == 'L' || c == 'l' {
			return true
		}
	}
	return false
}

// formattingTokens splits a format as format.match(formattingTokens) does:
//
//	/(\[[^[]*\])|([-_:/.,()\s]+)|(A|a|Q|YYYY|YY?|ww?|MM?M?M?|Do|DD?|hh?|HH?|mm?|ss?|S{1,3}|z|ZZ?)/g
//
// Text that matches no alternative is not in the result.
func formattingTokens(format []rune) [][]rune {
	var out [][]rune
	n := len(format)
	run := func(i int, c rune, max int) int {
		k := 0
		for k < max && i+k < n && format[i+k] == c {
			k++
		}
		return k
	}
	for i := 0; i < n; {
		c := format[i]
		length := 0
		switch {
		case c == '[':
			// \[[^[]*\]: up to the last ']' before the next '['.
			j := i + 1
			last := -1
			for j < n && format[j] != '[' {
				if format[j] == ']' {
					last = j
				}
				j++
			}
			if last >= 0 {
				length = last + 1 - i
			}
		case isSeparatorUnit(c):
			j := i
			for j < n && isSeparatorUnit(format[j]) {
				j++
			}
			length = j - i
		case c == 'A' || c == 'a' || c == 'Q' || c == 'z':
			length = 1
		case c == 'Y':
			if run(i, 'Y', 4) == 4 {
				length = 4
			} else {
				length = run(i, 'Y', 2)
			}
		case c == 'M':
			length = run(i, 'M', 4)
		case c == 'S':
			length = run(i, 'S', 3)
		case c == 'D':
			if i+1 < n && format[i+1] == 'o' {
				length = 2
			} else {
				length = run(i, 'D', 2)
			}
		case c == 'w' || c == 'h' || c == 'H' || c == 'm' || c == 's' || c == 'Z':
			length = run(i, c, 2)
		}
		if length == 0 {
			i++
			continue
		}
		out = append(out, format[i:i+length])
		i += length
	}
	return out
}

// makeParser is makeParser(format). The second result is false when the
// function throws.
func makeParser(format string) ([]parseToken, bool) {
	units := js.UTF16Units(format)
	if usesLocalizedFormat(units) {
		return nil, false
	}
	array := formattingTokens(units)
	if len(array) == 0 {
		// format.match returns null, and null has no length.
		return nil, false
	}
	tokens := make([]parseToken, len(array))
	for i, token := range array {
		if expr, ok := expressions[js.FromUTF16Units(token)]; ok {
			e := expr
			tokens[i] = parseToken{expr: &e}
		} else {
			// token.replace(/^\[|\]$/g, '')
			length := len(token)
			if token[0] == '[' {
				length--
			}
			if token[len(token)-1] == ']' {
				length--
			}
			tokens[i] = parseToken{literal: length}
		}
	}
	return tokens, true
}

// runParser is the function that makeParser returns.
func runParser(tokens []parseToken, inputStr string) (*parsedTime, error) {
	time := &parsedTime{}
	input := js.UTF16Units(inputStr)
	start := 0
	for _, token := range tokens {
		if token.expr == nil {
			start += token.literal
			continue
		}
		var part []rune
		if start < len(input) {
			part = input[start:]
		}
		s, e, ok := token.expr.regex(part)
		if !ok {
			// match is null.
			return nil, errInvalidValue
		}
		value := part[s:e]
		if err := token.expr.parser(time, js.FromUTF16Units(value)); err != nil {
			return nil, err
		}
		input = removeFirst(input, value)
	}
	correctHours(time)
	return time, nil
}

// removeFirst is input.replace(value, ”): it removes the first occurrence
// of value in input.
func removeFirst(input, value []rune) []rune {
	n := len(value)
	for i := 0; i+n <= len(input); i++ {
		match := true
		for k := 0; k < n; k++ {
			if input[i+k] != value[k] {
				match = false
				break
			}
		}
		if match {
			out := make([]rune, 0, len(input)-n)
			out = append(out, input[:i]...)
			return append(out, input[i+n:]...)
		}
	}
	return input
}

// parseFormattedInput is parseFormattedInput(input, format, utc, dayjs)
// without the UTC mode. The result is a time value: NaN for an invalid date.
func (f *Factory) parseFormattedInput(input, format string) *js.Date {
	invalid := js.NewDate(f.loc, math.NaN())
	if format == "x" || format == "X" {
		scale := 1.0
		if format == "X" {
			scale = 1000
		}
		return js.NewDate(f.loc, scale*js.Number(input))
	}
	tokens, ok := makeParser(format)
	if !ok {
		return invalid
	}
	t, err := runParser(tokens, input)
	if err != nil {
		return invalid
	}

	now := f.newDate()
	// var d = day || (!year && !month ? now.getDate() : 1);
	var d float64
	if t.dayString || truthyNum(t.day, t.hasDay) {
		d = t.day
	} else if !truthyNum(t.year, t.hasYear) && !truthyNum(t.month, t.hasMonth) {
		d = now.GetDate()
	} else {
		d = 1
	}
	// var y = year || now.getFullYear();
	y := t.year
	if !truthyNum(t.year, t.hasYear) {
		y = now.GetFullYear()
	}
	M := 0.0

	if !(truthyNum(t.year, t.hasYear) && !truthyNum(t.month, t.hasMonth)) {
		if t.hasMonth && t.month > 0 {
			M = t.month - 1
		} else {
			M = now.GetMonth()
		}
	}

	orZero := func(v float64, has bool) float64 {
		if truthyNum(v, has) {
			return v
		}
		return 0
	}
	h := orZero(t.hours, t.hasHours)
	m := orZero(t.minutes, t.hasMinutes)
	s := orZero(t.seconds, t.hasSeconds)
	ms := orZero(t.milliseconds, t.hasMilliseconds)

	if t.hasZone {
		return js.NewDate(f.loc, js.DateUTC(y, M, d, h, m, s, ms+float64(float64(t.zoneOffset*60)*1000)))
	}

	newDate := js.NewDateLocal(f.loc, y, M, d, h, m, s, ms)

	if truthyNum(t.week, t.hasWeek) {
		// dayjs(newDate).week(week): the weekOfYear plugin is not loaded,
		// so this throws, and the plugin catches the exception.
		return invalid
	}

	return newDate
}

// Parse is dayjs(date, format) with a string format, and dayjs(date,
// format, true) for strict parsing. Strict parsing requires that the result
// formats back to the input.
func (f *Factory) Parse(date, format string, strict bool) (*Dayjs, error) {
	d := f.wrap(f.parseFormattedInput(date, format))
	if strict {
		formatted, err := d.Format(format)
		if err != nil {
			return nil, err
		}
		if date != formatted {
			// The fields $y to $ms keep the values of the parsed date, as
			// upstream: the plugin does not call init again.
			d.d = js.NewDate(f.loc, math.NaN())
		}
	}
	return d, nil
}

// regexParse is REGEX_PARSE of constant.js.
var regexParse = regexp2.MustCompile(`^(\d{4})[-/]?(\d{1,2})?[-/]?(\d{0,2})[Tt\s]*(\d{1,2})?:?(\d{1,2})?:?(\d{1,2})?[.:]?(\d+)?$`, regexp2.ECMAScript)

// FromString is dayjs(date) with a string and no format: parseDate.
func (f *Factory) FromString(date string) *Dayjs {
	// !/Z$/i.test(date)
	if n := len(date); n == 0 || (date[n-1] != 'Z' && date[n-1] != 'z') {
		if m, err := regexParse.FindStringMatch(date); err == nil && m != nil {
			group := func(i int) (string, bool) {
				g := m.GroupByNumber(i)
				if len(g.Captures) == 0 {
					return "", false
				}
				return g.String(), true
			}
			// A missing or empty group is falsy.
			numOr := func(i int, def float64) float64 {
				if s, ok := group(i); ok && s != "" {
					return js.Number(s)
				}
				return def
			}
			y, _ := group(1)
			// d[2] - 1 || 0
			mo := 0.0
			if s, ok := group(2); ok {
				if v := js.Number(s) - 1; v != 0 && !math.IsNaN(v) {
					mo = v
				}
			}
			msStr := "0"
			if s, ok := group(7); ok && s != "" {
				msStr = s
			}
			msStr = js.StringSlice(msStr, 0, 3)
			return f.wrap(js.NewDateLocal(f.loc, js.Number(y), mo, numOr(3, 1), numOr(4, 0), numOr(5, 0), numOr(6, 0), js.Number(msStr)))
		}
	}
	return f.wrap(js.NewDateString(f.loc, date))
}
