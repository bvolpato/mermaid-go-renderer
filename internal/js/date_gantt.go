// Ported from v8/src/date/dateparser.h, dateparser-inl.h and dateparser.cc,
// and from the function ParseDateTimeString of v8/src/date/date.cc
// (BSD-3-Clause, Copyright 2011 and 2012 the V8 project authors,
// LICENSE-V8.txt). The port does not record the version of V8. V8 15.6.75,
// the version of the reference browser, has the four files and the function.
//
// This file adds Date.parse and new Date(string). V8 first reads the string
// as an ES5 date-time string. Where that stops, the legacy parser of V8
// continues, which accepts many more forms ("2019-09-1", "Jan 5 2020 10:00").

package js

import (
	"math"
	"time"
	"unicode"
)

// DateParse is Date.parse(s): the time value of the date that the string
// names, or NaN. A string without a zone is in the zone loc, except a
// date-only ISO string, which is in UTC. A nil location means UTC.
func DateParse(loc *time.Location, s string) float64 {
	if loc == nil {
		loc = time.UTC
	}
	var out [dateOutputSize]float64
	if !dateParserParse(UTF16Units(s), &out) {
		return math.NaN()
	}
	day := makeDay(out[dateYear], out[dateMonth], out[dateDay])
	tm := makeTime(out[dateHour], out[dateMinute], out[dateSecond], out[dateMillisecond])
	date := makeDate(day, tm)
	if math.IsNaN(out[dateUTCOffset]) {
		if date >= -maxTimeBeforeUTCMs && date <= maxTimeBeforeUTCMs {
			date = localToUTC(loc, date)
		} else {
			return math.NaN()
		}
	} else {
		date -= float64(out[dateUTCOffset] * 1000.0)
	}
	return TimeClip(date)
}

// NewDateString is new Date(s) with a string argument.
func NewDateString(loc *time.Location, s string) *Date {
	if loc == nil {
		loc = time.UTC
	}
	return &Date{t: DateParse(loc, s), loc: loc}
}

// The fields of the output of the parser.
const (
	dateYear = iota
	dateMonth
	dateDay
	dateHour
	dateMinute
	dateSecond
	dateMillisecond
	dateUTCOffset
	dateOutputSize
)

// dateNone indicates a missing value (kNone).
const dateNone = math.MaxInt32

// dateMaxSignificantDigits is the number of digits that build the value of
// a numeral. The parser ignores the other digits.
const dateMaxSignificantDigits = 9

// dateSmiMax is Smi::kMaxValue of a V8 build with pointer compression, as
// Chromium has.
const dateSmiMax = 1<<30 - 1

func dateBetween(x, lo, hi int) bool { return lo <= x && x <= hi }

// dateInputReader is InputReader: string parsing and character
// classification on UTF-16 code units.
type dateInputReader struct {
	index  int
	buffer []rune
	ch     rune
}

func newDateInputReader(s []rune) *dateInputReader {
	in := &dateInputReader{buffer: s}
	in.next()
	return in
}

func (in *dateInputReader) position() int { return in.index }

// next advances to the next character of the string.
func (in *dateInputReader) next() {
	if in.index < len(in.buffer) {
		in.ch = in.buffer[in.index]
	} else {
		in.ch = 0
	}
	in.index++
}

// readUnsignedNumeral reads a string of digits as an unsigned number.
func (in *dateInputReader) readUnsignedNumeral() int {
	n := 0
	i := 0
	// First, skip leading zeros.
	for in.ch == '0' {
		in.next()
	}
	// And then, do the conversion.
	for in.isASCIIDigit() {
		if i < dateMaxSignificantDigits {
			n = n*10 + int(in.ch-'0')
		}
		i++
		in.next()
	}
	return n
}

// readWord reads a word (a sequence of characters >= 'A'), fills the prefix
// with its lower-case start, and returns the length of the word.
func (in *dateInputReader) readWord(prefix *[3]rune) int {
	length := 0
	for ; in.isASCIIAlphaOrAbove() && !in.isWhiteSpaceChar(); length++ {
		if length < len(prefix) {
			prefix[length] = in.ch | 0x20
		}
		in.next()
	}
	for i := length; i < len(prefix); i++ {
		prefix[i] = 0
	}
	return length
}

func (in *dateInputReader) skip(c rune) bool {
	if in.ch == c {
		in.next()
		return true
	}
	return false
}

func (in *dateInputReader) skipWhiteSpace() bool {
	if dateIsWhiteSpaceOrLineTerminator(in.ch) {
		in.next()
		return true
	}
	return false
}

func (in *dateInputReader) skipParentheses() bool {
	if in.ch != '(' {
		return false
	}
	balance := 0
	for {
		if in.ch == ')' {
			balance--
		} else if in.ch == '(' {
			balance++
		}
		in.next()
		if !(balance > 0 && in.ch != 0) {
			break
		}
	}
	return true
}

func (in *dateInputReader) isEnd() bool               { return in.ch == 0 }
func (in *dateInputReader) isASCIIDigit() bool        { return in.ch >= '0' && in.ch <= '9' }
func (in *dateInputReader) isASCIIAlphaOrAbove() bool { return in.ch >= 'A' }
func (in *dateInputReader) isWhiteSpaceChar() bool    { return dateIsWhiteSpace(in.ch) }

// dateIsWhiteSpace is IsWhiteSpace of V8: the WhiteSpace production of
// ECMAScript, without the line terminators.
func dateIsWhiteSpace(c rune) bool {
	switch c {
	case 0x09, 0x0B, 0x0C, 0x20, 0xA0, 0xFEFF:
		return true
	}
	return c > 0xFF && unicode.Is(unicode.Zs, c)
}

func dateIsWhiteSpaceOrLineTerminator(c rune) bool {
	return dateIsWhiteSpace(c) || c == '\n' || c == '\r' || c == 0x2028 || c == 0x2029
}

// The types of a keyword (KeywordType). They are also the tags of the
// keyword tokens.
const (
	dateKwInvalid = iota
	dateKwMonthName
	dateKwTimeZoneName
	dateKwTimeSeparator
	dateKwAmPm
)

// The tags of the other tokens.
const (
	dateTagInvalid    = -6
	dateTagUnknown    = -5
	dateTagWhiteSpace = -4
	dateTagNumber     = -3
	dateTagSymbol     = -2
	dateTagEndOfInput = -1
)

// dateToken is DateToken.
type dateToken struct {
	tag    int
	length int // the number of characters
	value  int
}

func (t dateToken) isInvalid() bool    { return t.tag == dateTagInvalid }
func (t dateToken) isNumber() bool     { return t.tag == dateTagNumber }
func (t dateToken) isWhiteSpace() bool { return t.tag == dateTagWhiteSpace }
func (t dateToken) isEndOfInput() bool { return t.tag == dateTagEndOfInput }
func (t dateToken) isKeyword() bool    { return t.tag >= 0 }
func (t dateToken) isSymbol(symbol rune) bool {
	return t.tag == dateTagSymbol && t.value == int(symbol)
}
func (t dateToken) isKeywordType(tag int) bool { return t.tag == tag }
func (t dateToken) isFixedLengthNumber(length int) bool {
	return t.isNumber() && t.length == length
}
func (t dateToken) isASCIISign() bool {
	return t.tag == dateTagSymbol && (t.value == '-' || t.value == '+')
}

// asciiSign returns 1 for '+' and -1 for '-'.
func (t dateToken) asciiSign() int { return 44 - t.value }
func (t dateToken) isKeywordZ() bool {
	return t.isKeywordType(dateKwTimeZoneName) && t.length == 1 && t.value == 0
}

func dateSymbolToken(symbol rune) dateToken { return dateToken{dateTagSymbol, 1, int(symbol)} }

var (
	dateEndOfInputToken = dateToken{dateTagEndOfInput, 0, -1}
	dateUnknownToken    = dateToken{dateTagUnknown, 1, -1}
	dateInvalidToken    = dateToken{dateTagInvalid, 0, -1}
)

// dateKeywords is KeywordTable::array: names of months, time zones and
// am/pm, with their type and value.
var dateKeywords = [...]struct {
	prefix [3]rune
	typ    int
	value  int
}{
	{[3]rune{'j', 'a', 'n'}, dateKwMonthName, 1},
	{[3]rune{'f', 'e', 'b'}, dateKwMonthName, 2},
	{[3]rune{'m', 'a', 'r'}, dateKwMonthName, 3},
	{[3]rune{'a', 'p', 'r'}, dateKwMonthName, 4},
	{[3]rune{'m', 'a', 'y'}, dateKwMonthName, 5},
	{[3]rune{'j', 'u', 'n'}, dateKwMonthName, 6},
	{[3]rune{'j', 'u', 'l'}, dateKwMonthName, 7},
	{[3]rune{'a', 'u', 'g'}, dateKwMonthName, 8},
	{[3]rune{'s', 'e', 'p'}, dateKwMonthName, 9},
	{[3]rune{'o', 'c', 't'}, dateKwMonthName, 10},
	{[3]rune{'n', 'o', 'v'}, dateKwMonthName, 11},
	{[3]rune{'d', 'e', 'c'}, dateKwMonthName, 12},
	{[3]rune{'a', 'm', 0}, dateKwAmPm, 0},
	{[3]rune{'p', 'm', 0}, dateKwAmPm, 12},
	{[3]rune{'u', 't', 0}, dateKwTimeZoneName, 0},
	{[3]rune{'u', 't', 'c'}, dateKwTimeZoneName, 0},
	{[3]rune{'z', 0, 0}, dateKwTimeZoneName, 0},
	{[3]rune{'g', 'm', 't'}, dateKwTimeZoneName, 0},
	{[3]rune{'c', 'd', 't'}, dateKwTimeZoneName, -5},
	{[3]rune{'c', 's', 't'}, dateKwTimeZoneName, -6},
	{[3]rune{'e', 'd', 't'}, dateKwTimeZoneName, -4},
	{[3]rune{'e', 's', 't'}, dateKwTimeZoneName, -5},
	{[3]rune{'m', 'd', 't'}, dateKwTimeZoneName, -6},
	{[3]rune{'m', 's', 't'}, dateKwTimeZoneName, -7},
	{[3]rune{'p', 'd', 't'}, dateKwTimeZoneName, -7},
	{[3]rune{'p', 's', 't'}, dateKwTimeZoneName, -8},
	{[3]rune{'t', 0, 0}, dateKwTimeSeparator, 0},
}

// dateKeywordLookup is KeywordTable::Lookup. It returns the type and the
// value of the word, or the type dateKwInvalid.
func dateKeywordLookup(pre *[3]rune, length int) (typ, value int) {
	for i := range dateKeywords {
		k := &dateKeywords[i]
		// A word longer than the keyword is only allowed for month names.
		if *pre == k.prefix && (length <= len(pre) || k.typ == dateKwMonthName) {
			return k.typ, k.value
		}
	}
	return dateKwInvalid, 0
}

// dateTokenizer is DateStringTokenizer.
type dateTokenizer struct {
	in    *dateInputReader
	nextT dateToken
}

func newDateTokenizer(in *dateInputReader) *dateTokenizer {
	s := &dateTokenizer{in: in}
	s.nextT = s.scan()
	return s
}

func (s *dateTokenizer) next() dateToken {
	result := s.nextT
	s.nextT = s.scan()
	return result
}

func (s *dateTokenizer) peek() dateToken { return s.nextT }

func (s *dateTokenizer) skipSymbol(symbol rune) bool {
	if s.nextT.isSymbol(symbol) {
		s.nextT = s.scan()
		return true
	}
	return false
}

func (s *dateTokenizer) scan() dateToken {
	in := s.in
	prePos := in.position()
	if in.isEnd() {
		return dateEndOfInputToken
	}
	if in.isASCIIDigit() {
		n := in.readUnsignedNumeral()
		length := in.position() - prePos
		return dateToken{dateTagNumber, length, n}
	}
	if in.skip(':') {
		return dateSymbolToken(':')
	}
	if in.skip('-') {
		return dateSymbolToken('-')
	}
	if in.skip('+') {
		return dateSymbolToken('+')
	}
	if in.skip('.') {
		return dateSymbolToken('.')
	}
	if in.skip(')') {
		return dateSymbolToken(')')
	}
	if in.isASCIIAlphaOrAbove() && !in.isWhiteSpaceChar() {
		var buffer [3]rune
		length := in.readWord(&buffer)
		typ, value := dateKeywordLookup(&buffer, length)
		return dateToken{typ, length, value}
	}
	if in.skipWhiteSpace() {
		return dateToken{dateTagWhiteSpace, in.position() - prePos, -1}
	}
	if in.skipParentheses() {
		return dateUnknownToken
	}
	in.next()
	return dateUnknownToken
}

// dateTimeZoneComposer is TimeZoneComposer.
type dateTimeZoneComposer struct {
	sign, hour, minute int
}

func (tz *dateTimeZoneComposer) set(offsetInHours int) {
	if offsetInHours < 0 {
		tz.sign = -1
	} else {
		tz.sign = 1
	}
	tz.hour = offsetInHours * tz.sign
	tz.minute = 0
}

func (tz *dateTimeZoneComposer) setSign(sign int) {
	if sign < 0 {
		tz.sign = -1
	} else {
		tz.sign = 1
	}
}

func (tz *dateTimeZoneComposer) isExpecting(n int) bool {
	return tz.hour != dateNone && tz.minute == dateNone && dateIsMinute(n)
}
func (tz *dateTimeZoneComposer) isUTC() bool   { return tz.hour == 0 && tz.minute == 0 }
func (tz *dateTimeZoneComposer) isEmpty() bool { return tz.hour == dateNone }

func (tz *dateTimeZoneComposer) write(output *[dateOutputSize]float64) bool {
	if tz.sign != dateNone {
		if tz.hour == dateNone {
			tz.hour = 0
		}
		if tz.minute == dateNone {
			tz.minute = 0
		}
		// V8 does this in unsigned 32-bit arithmetic.
		totalSecondsUnsigned := uint32(tz.hour)*3600 + uint32(tz.minute)*60
		if totalSecondsUnsigned > dateSmiMax {
			return false
		}
		totalSeconds := int(totalSecondsUnsigned)
		if tz.sign < 0 {
			totalSeconds = -totalSeconds
		}
		output[dateUTCOffset] = float64(totalSeconds)
	} else {
		output[dateUTCOffset] = math.NaN()
	}
	return true
}

func dateIsMinute(x int) bool      { return dateBetween(x, 0, 59) }
func dateIsHour(x int) bool        { return dateBetween(x, 0, 23) }
func dateIsSecond(x int) bool      { return dateBetween(x, 0, 59) }
func dateIsHour12(x int) bool      { return dateBetween(x, 0, 12) }
func dateIsMillisecond(x int) bool { return dateBetween(x, 0, 999) }
func dateIsMonth(x int) bool       { return dateBetween(x, 1, 12) }
func dateIsDay(x int) bool         { return dateBetween(x, 1, 31) }

// dateTimeComposer is TimeComposer.
type dateTimeComposer struct {
	comp       [4]int
	index      int
	hourOffset int
}

func (tc *dateTimeComposer) isEmpty() bool { return tc.index == 0 }

func (tc *dateTimeComposer) isExpecting(n int) bool {
	return (tc.index == 1 && dateIsMinute(n)) || (tc.index == 2 && dateIsSecond(n)) ||
		(tc.index == 3 && dateIsMillisecond(n))
}

func (tc *dateTimeComposer) add(n int) bool {
	if tc.index < len(tc.comp) {
		tc.comp[tc.index] = n
		tc.index++
		return true
	}
	return false
}

func (tc *dateTimeComposer) addFinal(n int) bool {
	if !tc.add(n) {
		return false
	}
	for tc.index < len(tc.comp) {
		tc.comp[tc.index] = 0
		tc.index++
	}
	return true
}

func (tc *dateTimeComposer) write(output *[dateOutputSize]float64) bool {
	// All time slots default to 0.
	for tc.index < len(tc.comp) {
		tc.comp[tc.index] = 0
		tc.index++
	}

	hour := &tc.comp[0]
	minute := tc.comp[1]
	second := tc.comp[2]
	millisecond := tc.comp[3]

	if tc.hourOffset != dateNone {
		if !dateIsHour12(*hour) {
			return false
		}
		*hour %= 12
		*hour += tc.hourOffset
	}

	if !dateIsHour(*hour) || !dateIsMinute(minute) || !dateIsSecond(second) ||
		!dateIsMillisecond(millisecond) {
		// A 24th hour is allowed if minutes, seconds, and milliseconds are 0.
		if *hour != 24 || minute != 0 || second != 0 || millisecond != 0 {
			return false
		}
	}

	output[dateHour] = float64(*hour)
	output[dateMinute] = float64(minute)
	output[dateSecond] = float64(second)
	output[dateMillisecond] = float64(millisecond)
	return true
}

// dateDayComposer is DayComposer.
type dateDayComposer struct {
	comp       [3]int
	index      int
	namedMonth int
	// isISODate makes the order of the numbers year, month, day.
	isISODate bool
}

func (dc *dateDayComposer) isEmpty() bool { return dc.index == 0 }

func (dc *dateDayComposer) add(n int) bool {
	if dc.index < len(dc.comp) {
		dc.comp[dc.index] = n
		dc.index++
		return true
	}
	return false
}

func (dc *dateDayComposer) write(output *[dateOutputSize]float64) bool {
	if dc.index < 1 {
		return false
	}
	// Day and month defaults to 1.
	for dc.index < len(dc.comp) {
		dc.comp[dc.index] = 1
		dc.index++
	}

	year := 0 // The default year is 0 (=> 2000) for KJS compatibility.
	month := dateNone
	day := dateNone

	if dc.namedMonth == dateNone {
		if dc.isISODate || (dc.index == 3 && !dateIsDay(dc.comp[0])) {
			// YMD
			year = dc.comp[0]
			month = dc.comp[1]
			day = dc.comp[2]
		} else {
			// MD(Y)
			month = dc.comp[0]
			day = dc.comp[1]
			if dc.index == 3 {
				year = dc.comp[2]
			}
		}
	} else {
		month = dc.namedMonth
		if dc.index == 1 {
			// MD or DM
			day = dc.comp[0]
		} else if !dateIsDay(dc.comp[0]) {
			// YMD, MYD, or YDM
			year = dc.comp[0]
			day = dc.comp[1]
		} else {
			// DMY, MDY, or DYM
			day = dc.comp[0]
			year = dc.comp[1]
		}
	}

	if !dc.isISODate {
		if dateBetween(year, 0, 49) {
			year += 2000
		} else if dateBetween(year, 50, 99) {
			year += 1900
		}
	}

	if year > dateSmiMax || year < -dateSmiMax-1 || !dateIsMonth(month) || !dateIsDay(day) {
		return false
	}

	output[dateYear] = float64(year)
	output[dateMonth] = float64(month - 1) // 0-based
	output[dateDay] = float64(day)
	return true
}

// dateReadMilliseconds is DateParser::ReadMilliseconds: the first three
// significant digits of the numeral.
func dateReadMilliseconds(token dateToken) int {
	number := token.value
	length := token.length
	if length < 3 {
		// Less than three digits. Multiply to put the most significant digit
		// in the hundreds position.
		if length == 1 {
			number *= 100
		} else if length == 2 {
			number *= 10
		}
	} else if length > 3 {
		if length > dateMaxSignificantDigits {
			length = dateMaxSignificantDigits
		}
		// More than three digits. Divide by 10^(length - 3) to get the three
		// most significant digits.
		factor := 1
		for {
			factor *= 10
			length--
			if !(length > 3) {
				break
			}
		}
		number /= factor
	}
	return number
}

// dateParserParse is DateParser::Parse. The input is a string as UTF-16
// code units.
func dateParserParse(str []rune, out *[dateOutputSize]float64) bool {
	in := newDateInputReader(str)
	scanner := newDateTokenizer(in)
	tz := &dateTimeZoneComposer{sign: dateNone, hour: dateNone, minute: dateNone}
	tm := &dateTimeComposer{hourOffset: dateNone}
	day := &dateDayComposer{namedMonth: dateNone}

	// First try getting as far as possible with as ES5 Date Time String.
	nextUnhandledToken := dateParseES5DateTime(scanner, day, tm, tz)
	if nextUnhandledToken.isInvalid() {
		return false
	}
	hasReadNumber := !day.isEmpty()
	// If there's anything left, continue with the legacy parser.
	for token := nextUnhandledToken; !token.isEndOfInput(); token = scanner.next() {
		if token.isNumber() {
			hasReadNumber = true
			n := token.value
			if scanner.skipSymbol(':') {
				if scanner.skipSymbol(':') {
					// n + "::"
					if !tm.isEmpty() {
						return false
					}
					tm.add(n)
					tm.add(0)
				} else {
					// n + ":"
					if !tm.add(n) {
						return false
					}
					if scanner.peek().isSymbol('.') {
						scanner.next()
					}
				}
			} else if scanner.skipSymbol('.') && tm.isExpecting(n) {
				tm.add(n)
				if !scanner.peek().isNumber() {
					return false
				}
				ms := dateReadMilliseconds(scanner.next())
				if ms < 0 {
					return false
				}
				tm.addFinal(ms)
			} else if tz.isExpecting(n) {
				tz.minute = n
			} else if tm.isExpecting(n) {
				tm.addFinal(n)
				// Require end, white space, "Z", "+" or "-" immediately after
				// finalizing time.
				peek := scanner.peek()
				if !peek.isEndOfInput() && !peek.isWhiteSpace() &&
					!peek.isKeywordZ() && !peek.isASCIISign() {
					return false
				}
			} else {
				if !day.add(n) {
					return false
				}
				scanner.skipSymbol('-')
			}
		} else if token.isKeyword() {
			// Parse a "word" (a sequence of characters >= 'A').
			typ := token.tag
			value := token.value
			if typ == dateKwAmPm && !tm.isEmpty() {
				tm.hourOffset = value
			} else if typ == dateKwMonthName {
				day.namedMonth = value
				scanner.skipSymbol('-')
			} else if typ == dateKwTimeZoneName && hasReadNumber {
				tz.set(value)
			} else {
				// Garbage words are illegal if a number has been read.
				if hasReadNumber {
					return false
				}
				// The first number has to be separated from garbage words by
				// whitespace or other separators.
				if scanner.peek().isNumber() {
					return false
				}
			}
		} else if token.isASCIISign() && (tz.isUTC() || !tm.isEmpty()) {
			// Parse UTC offset (only after UTC or time).
			tz.setSign(token.asciiSign())
			// The following number may be empty.
			n := 0
			length := 0
			if scanner.peek().isNumber() {
				nextToken := scanner.next()
				length = nextToken.length
				n = nextToken.value
			}
			hasReadNumber = true

			if scanner.peek().isSymbol(':') {
				tz.hour = n
				tz.minute = dateNone
			} else if length == 2 || length == 1 {
				// Handle time zones like GMT-8.
				tz.hour = n
				tz.minute = 0
			} else if length == 4 || length == 3 {
				// Looks like the hhmm format.
				tz.hour = n / 100
				tz.minute = n % 100
			} else {
				// No need to accept time zones like GMT-12345.
				return false
			}
		} else if (token.isASCIISign() || token.isSymbol(')')) && hasReadNumber {
			// Extra sign or ')' is illegal if a number has been read.
			return false
		}
		// Ignore other characters and whitespace.
	}

	return day.write(out) && tm.write(out) && tz.write(out)
}

// dateParseES5DateTime is DateParser::ParseES5DateTime. It returns the next
// token for the legacy parser, the end-of-input token when the string is a
// complete ES5 date-time string, or the invalid token.
func dateParseES5DateTime(scanner *dateTokenizer, day *dateDayComposer, tm *dateTimeComposer, tz *dateTimeZoneComposer) dateToken {
	// Parse mandatory date string: [('-'|'+')yy]yyyy[':'MM[':'DD]]
	if scanner.peek().isASCIISign() {
		// Keep the sign token, so we can pass it back to the legacy
		// parser if we don't use it.
		signToken := scanner.next()
		if !scanner.peek().isFixedLengthNumber(6) {
			return signToken
		}
		sign := signToken.asciiSign()
		year := scanner.next().value
		if sign < 0 && year == 0 {
			return signToken
		}
		day.add(sign * year)
	} else if scanner.peek().isFixedLengthNumber(4) {
		day.add(scanner.next().value)
	} else {
		return scanner.next()
	}
	if scanner.skipSymbol('-') {
		if !scanner.peek().isFixedLengthNumber(2) || !dateIsMonth(scanner.peek().value) {
			return scanner.next()
		}
		day.add(scanner.next().value)
		if scanner.skipSymbol('-') {
			if !scanner.peek().isFixedLengthNumber(2) || !dateIsDay(scanner.peek().value) {
				return scanner.next()
			}
			day.add(scanner.next().value)
		}
	}
	// Check for optional time string: 'T'HH':'mm[':'ss['.'sss]]Z
	if !scanner.peek().isKeywordType(dateKwTimeSeparator) {
		if !scanner.peek().isEndOfInput() {
			return scanner.next()
		}
	} else {
		// ES5 Date Time String time part is present.
		scanner.next()
		if !scanner.peek().isFixedLengthNumber(2) || !dateBetween(scanner.peek().value, 0, 24) {
			return dateInvalidToken
		}
		// Allow 24:00[:00[.000]], but no other time starting with 24.
		hourIs24 := scanner.peek().value == 24
		tm.add(scanner.next().value)
		if !scanner.skipSymbol(':') {
			return dateInvalidToken
		}
		if !scanner.peek().isFixedLengthNumber(2) || !dateIsMinute(scanner.peek().value) ||
			(hourIs24 && scanner.peek().value > 0) {
			return dateInvalidToken
		}
		tm.add(scanner.next().value)
		if scanner.skipSymbol(':') {
			if !scanner.peek().isFixedLengthNumber(2) || !dateIsSecond(scanner.peek().value) ||
				(hourIs24 && scanner.peek().value > 0) {
				return dateInvalidToken
			}
			tm.add(scanner.next().value)
			if scanner.skipSymbol('.') {
				if !scanner.peek().isNumber() || (hourIs24 && scanner.peek().value > 0) {
					return dateInvalidToken
				}
				// Allow more or less than the mandated three digits.
				tm.add(dateReadMilliseconds(scanner.next()))
			}
		}
		// Check for optional timezone designation: 'Z' | ('+'|'-')hh':'mm
		if scanner.peek().isKeywordZ() {
			scanner.next()
			tz.set(0)
		} else if scanner.peek().isSymbol('+') || scanner.peek().isSymbol('-') {
			if scanner.next().value == '+' {
				tz.setSign(1)
			} else {
				tz.setSign(-1)
			}
			if scanner.peek().isFixedLengthNumber(4) {
				// hhmm extension syntax.
				hourmin := scanner.next().value
				hour := hourmin / 100
				min := hourmin % 100
				if !dateIsHour(hour) || !dateIsMinute(min) {
					return dateInvalidToken
				}
				tz.hour = hour
				tz.minute = min
			} else {
				// hh:mm standard syntax.
				if !scanner.peek().isFixedLengthNumber(2) || !dateIsHour(scanner.peek().value) {
					return dateInvalidToken
				}
				tz.hour = scanner.next().value
				if !scanner.skipSymbol(':') {
					return dateInvalidToken
				}
				if !scanner.peek().isFixedLengthNumber(2) || !dateIsMinute(scanner.peek().value) {
					return dateInvalidToken
				}
				tz.minute = scanner.next().value
			}
		}
		if !scanner.peek().isEndOfInput() {
			return dateInvalidToken
		}
	}
	// Successfully parsed ES5 Date Time String. When the time zone offset is
	// absent, date-only forms are interpreted as a UTC time and date-time
	// forms are interpreted as a local time.
	if tz.isEmpty() && tm.isEmpty() {
		tz.set(0)
	}
	day.isISODate = true
	return dateEndOfInputToken
}

// LocalFields returns the results of getFullYear, getMonth, getDate, getDay,
// getHours, getMinutes, getSeconds and getMilliseconds with one conversion
// to local time. All of them are NaN for an invalid date.
func (d *Date) LocalFields() (year, month, date, day, hours, minutes, seconds, milliseconds float64) {
	ms, ok := d.local()
	if !ok {
		nan := math.NaN()
		return nan, nan, nan, nan, nan, nan, nan, nan
	}
	days, tid := split(ms)
	y, m, dd := civilFromDays(days)
	return float64(y), float64(m), float64(dd), float64(weekday(days)),
		float64(tid / msPerHour), float64((tid / msPerMinute) % 60), float64((tid / msPerSecond) % 60), float64(tid % 1000)
}
