// Ported from Chromium 156.0.8078.80 (Blink), in part: the descriptors of an
// @font-face rule become the family, the capabilities and the ranges of a
// font face as in third_party/blink/renderer/core/css/font_face.cc
// (FontFace::GetFontSelectionCapabilities, FontFace::InitCSSFontFace) and
// third_party/blink/renderer/platform/fonts/unicode_range_set.cc. The
// scanner of the style sheet text ports no upstream file.
//
// The license of each Blink file is the license in its header:
//
//   - font_face.cc: BSD-3-Clause, with the terms in the file.
//     Copyright (C) 2013 Google Inc. All rights reserved.
//   - unicode_range_set.cc: BSD-2-Clause, with the terms in the file.
//     Copyright (C) 2007, 2008, 2011 Apple Inc. All rights reserved.
//
// NOTICE-Blink.txt in this directory has the headers of the two files.

package font

import (
	"sort"
	"strconv"
	"strings"
)

// webRule is one @font-face rule of a style sheet of the page: what Blink
// keeps of it in a FontFace.
type webRule struct {
	// family is the font-family descriptor without quotes.
	family string
	// slopeMin and slopeMax are the range of the font-style descriptor in
	// degrees: 0 for normal, 14 for italic and for oblique without an
	// angle.
	slopeMin, slopeMax float64
	// weightMin and weightMax are the range of the font-weight descriptor.
	weightMin, weightMax float64
	// ranges is the unicode-range descriptor, or nil for a rule without
	// the descriptor: the rule then has every character.
	ranges *rangeSet
	// url is the url of the first source that Chromium can read, relative
	// to the style sheet.
	url string
}

// rangeSet is UnicodeRangeSet of Blink: sorted ranges of characters that
// do not touch.
type rangeSet struct {
	ranges [][2]rune
}

// newRangeSet is the constructor of UnicodeRangeSet: it sorts the ranges
// and joins the ranges that overlap or touch.
func newRangeSet(ranges [][2]rune) *rangeSet {
	if len(ranges) == 0 {
		return nil
	}
	sorted := append([][2]rune(nil), ranges...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i][0] < sorted[j][0] })
	out := sorted[:1]
	for _, r := range sorted[1:] {
		last := &out[len(out)-1]
		if r[0] <= last[1]+1 {
			if r[1] > last[1] {
				last[1] = r[1]
			}
			continue
		}
		out = append(out, r)
	}
	return &rangeSet{ranges: out}
}

// contains is UnicodeRangeSet::Contains. A nil set has every character.
func (s *rangeSet) contains(r rune) bool {
	if s == nil {
		return true
	}
	i := sort.Search(len(s.ranges), func(i int) bool { return s.ranges[i][1] >= r })
	return i < len(s.ranges) && s.ranges[i][0] <= r
}

// parseFontFaceRules returns the @font-face rules of a style sheet in the
// order of the text. A rule without a family or without a source that
// Chromium can read is left out, as Blink makes no font face for it.
func parseFontFaceRules(sheet string) []webRule {
	sheet = stripCSSComments(sheet)
	var rules []webRule
	for i := 0; i < len(sheet); {
		switch c := sheet[i]; {
		case c == '"' || c == '\'':
			i = skipCSSString(sheet, i)
		case c == '@' && hasCSSKeyword(sheet[i+1:], "font-face"):
			open := strings.IndexByte(sheet[i:], '{')
			if open < 0 {
				return rules
			}
			body, end := cssBlock(sheet, i+open)
			if rule, ok := parseFontFaceRule(body); ok {
				rules = append(rules, rule)
			}
			i = end
		default:
			i++
		}
	}
	return rules
}

// hasCSSKeyword reports whether text starts with a keyword, without regard
// to case, that is not the start of a longer name.
func hasCSSKeyword(text, keyword string) bool {
	if len(text) < len(keyword) || !strings.EqualFold(text[:len(keyword)], keyword) {
		return false
	}
	if len(text) == len(keyword) {
		return true
	}
	c := text[len(keyword)]
	return !(c == '-' || c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80)
}

func stripCSSComments(s string) string {
	if !strings.Contains(s, "/*") {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); {
		switch {
		case s[i] == '"' || s[i] == '\'':
			end := skipCSSString(s, i)
			sb.WriteString(s[i:end])
			i = end
		case strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return sb.String()
			}
			sb.WriteByte(' ')
			i += end + 4
		default:
			sb.WriteByte(s[i])
			i++
		}
	}
	return sb.String()
}

// skipCSSString returns the index after the string that starts at i.
func skipCSSString(s string, i int) int {
	quote := s[i]
	for i++; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		}
	}
	return len(s)
}

// cssBlock returns the text between the brace at open and the brace that
// closes it, and the index after the block.
func cssBlock(s string, open int) (body string, end int) {
	depth := 0
	for i := open; i < len(s); {
		switch s[i] {
		case '"', '\'':
			i = skipCSSString(s, i)
			continue
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[open+1 : i], i + 1
			}
		}
		i++
	}
	return s[open+1:], len(s)
}

// splitCSSTop splits text at a separator that is not in a string and not
// in parentheses.
func splitCSSTop(s string, sep byte) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '"' || c == '\'':
			i = skipCSSString(s, i)
			continue
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case c == sep && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
		i++
	}
	return append(parts, s[start:])
}

// parseFontFaceRule reads the descriptors of one rule.
func parseFontFaceRule(body string) (webRule, bool) {
	rule := webRule{weightMin: 400, weightMax: 400}
	for _, decl := range splitCSSTop(body, ';') {
		colon := strings.IndexByte(decl, ':')
		if colon < 0 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(decl[:colon]))
		value := strings.TrimSpace(decl[colon+1:])
		switch name {
		case "font-family":
			rule.family = cssFamilyName(value)
		case "font-style":
			fields := strings.Fields(strings.ToLower(value))
			if len(fields) == 0 {
				continue
			}
			switch fields[0] {
			case "normal":
				rule.slopeMin, rule.slopeMax = 0, 0
			case "italic":
				rule.slopeMin, rule.slopeMax = italicSlope, italicSlope
			case "oblique":
				rule.slopeMin, rule.slopeMax = italicSlope, italicSlope
				var angles []float64
				for _, f := range fields[1:] {
					if v, err := strconv.ParseFloat(strings.TrimSuffix(f, "deg"), 64); err == nil {
						angles = append(angles, v)
					}
				}
				switch len(angles) {
				case 1:
					rule.slopeMin, rule.slopeMax = angles[0], angles[0]
				case 2:
					// A range that goes down is turned around.
					rule.slopeMin, rule.slopeMax = min(angles[0], angles[1]), max(angles[0], angles[1])
				}
			}
		case "font-weight":
			var weights []float64
			for _, f := range strings.Fields(strings.ToLower(value)) {
				switch f {
				case "normal":
					weights = append(weights, 400)
				case "bold":
					weights = append(weights, 700)
				default:
					if v, err := strconv.ParseFloat(f, 64); err == nil && v >= 1 && v <= 1000 {
						weights = append(weights, v)
					}
				}
			}
			switch len(weights) {
			case 1:
				rule.weightMin, rule.weightMax = weights[0], weights[0]
			case 2:
				rule.weightMin, rule.weightMax = min(weights[0], weights[1]), max(weights[0], weights[1])
			}
		case "src":
			rule.url = cssFontSource(value)
		case "unicode-range":
			rule.ranges = newRangeSet(cssUnicodeRanges(value))
		}
	}
	return rule, rule.family != "" && rule.url != ""
}

// cssFamilyName returns the name of a font-family descriptor: the text of
// a string, or the identifiers with one space between them.
func cssFamilyName(value string) string {
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return cssUnescape(value[1 : len(value)-1])
	}
	return strings.Join(strings.Fields(value), " ")
}

func cssUnescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			sb.WriteByte(s[i])
			continue
		}
		j := i + 1
		for j < len(s) && j < i+7 && isHexDigit(s[j]) {
			j++
		}
		if j == i+1 {
			// An escaped character that is not a hexadecimal digit.
			sb.WriteByte(s[j])
			i = j
			continue
		}
		if v, err := strconv.ParseUint(s[i+1:j], 16, 32); err == nil {
			sb.WriteRune(rune(v))
		}
		if j < len(s) && s[j] == ' ' {
			j++
		}
		i = j - 1
	}
	return sb.String()
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// cssFontSource returns the url of the first source of a src descriptor
// that Chromium loads: a url without a format, or with a format that it
// supports. A local() source names an installed font, which the page of
// mermaid-cli does not have.
func cssFontSource(value string) string {
	for _, item := range splitCSSTop(value, ',') {
		item = strings.TrimSpace(item)
		if len(item) < 4 || !strings.EqualFold(item[:4], "url(") {
			continue
		}
		end := closeParen(item, 3)
		if end < 0 {
			continue
		}
		url := strings.TrimSpace(item[4:end])
		if len(url) >= 2 && (url[0] == '"' || url[0] == '\'') && url[len(url)-1] == url[0] {
			url = cssUnescape(url[1 : len(url)-1])
		}
		rest := strings.ToLower(item[end+1:])
		if at := strings.Index(rest, "format("); at >= 0 {
			stop := closeParen(rest, at+6)
			if stop < 0 {
				continue
			}
			format := strings.Trim(strings.TrimSpace(rest[at+7:stop]), `"'`)
			if !supportedFontFormat(format) {
				continue
			}
		}
		return url
	}
	return ""
}

// closeParen returns the index of the parenthesis that closes the one at
// open, or -1.
func closeParen(s string, open int) int {
	depth := 0
	for i := open; i < len(s); {
		switch s[i] {
		case '"', '\'':
			i = skipCSSString(s, i)
			continue
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return -1
}

// supportedFontFormat is FontCustomPlatformData::SupportsFormat with the
// format names of the variation fonts.
func supportedFontFormat(format string) bool {
	switch format {
	case "woff2", "woff2-variations":
		return true
	case "woff", "woff-variations", "truetype", "truetype-variations", "opentype", "opentype-variations", "collection":
		// Chromium reads these too. The module has only the WOFF2 files
		// that the style sheets of the page name first.
		return true
	}
	return false
}

// cssUnicodeRanges reads a unicode-range descriptor: a list of single
// characters, ranges and patterns with question marks.
func cssUnicodeRanges(value string) [][2]rune {
	var ranges [][2]rune
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if len(item) < 3 || (item[0] != 'U' && item[0] != 'u') || item[1] != '+' {
			continue
		}
		item = item[2:]
		var lo, hi string
		if dash := strings.IndexByte(item, '-'); dash >= 0 {
			lo, hi = item[:dash], item[dash+1:]
		} else if strings.Contains(item, "?") {
			lo, hi = strings.ReplaceAll(item, "?", "0"), strings.ReplaceAll(item, "?", "F")
		} else {
			lo, hi = item, item
		}
		from, err1 := strconv.ParseUint(lo, 16, 32)
		to, err2 := strconv.ParseUint(hi, 16, 32)
		if err1 != nil || err2 != nil || from > to || from > 0x10ffff {
			continue
		}
		if to > 0x10ffff {
			to = 0x10ffff
		}
		ranges = append(ranges, [2]rune{rune(from), rune(to)})
	}
	return ranges
}
