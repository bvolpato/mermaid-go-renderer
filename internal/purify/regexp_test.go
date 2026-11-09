package purify

import (
	"math/rand"
	"testing"
)

// TestFastPaths compares the hand-written tests of regexp.go with the
// regular expressions that they stand for.
func TestFastPaths(t *testing.T) {
	pieces := []string{
		"", "a", "Z", "http", "HTTPS", "ftp", "ftps", "mailto", "tel", "callto", "sms", "cid", "xmpp",
		"matrix", "javascript", "data", "foo", ":", "://", "/", ".", "+", "-", "#", "?", " ", "\t", "1",
		"<", ">", "</", "-->", "--!>", "]>", "--", "!", "]", "</style", "</SCRIPT", "</title>", "</xmp",
		"</textarea", "</noscript", "</iframe", "</noembed", "</noframes", "</b", "<b>", "<!",
		"data-", "aria-", "data-x", "aria-label", "_", "x.y", "\u00e9", "\u00b7", "\u00b6", "\U0001F600",
		"\u017f", "\u212a", "\n", "matrixx", "htt", "ttps",
	}
	rng := rand.New(rand.NewSource(1))
	var values []string
	for _, a := range pieces {
		for _, b := range pieces {
			values = append(values, a+b, a+":"+b)
		}
	}
	for i := 0; i < 20000; i++ {
		s := ""
		for n := rng.Intn(5); n >= 0; n-- {
			s += pieces[rng.Intn(len(pieces))]
		}
		values = append(values, s)
	}
	checks := []struct {
		name   string
		fast   func(string) bool
		regexp func(string) bool
	}{
		{"IS_ALLOWED_URI", matchAllowedURI, isAllowedURI.MatchString},
		{"attribute value probe", matchAttrValueProbe, attrValueProbe.MatchString},
		{"DATA_ATTR", matchDataAttr, dataAttr.MatchString},
		{"ARIA_ATTR", matchAriaAttr, ariaAttr.MatchString},
		{"ELEMENT_MARKUP_PROBE", matchElementMarkupProbe, elementMarkupProbe.MatchString},
	}
	for _, c := range checks {
		for _, v := range values {
			if got, want := c.fast(v), c.regexp(v); got != want {
				t.Errorf("%s: %q: fast path %v, expression %v", c.name, v, got, want)
				break
			}
		}
	}
	t.Logf("%d values for each of %d expressions", len(values), len(checks))
}

// TestStripAttrWhitespace checks the class of ATTR_WHITESPACE: U+0000 to
// U+0020, U+00A0, U+1680, U+180E, U+2000 to U+2029, U+205F and U+3000.
func TestStripAttrWhitespace(t *testing.T) {
	cases := [][2]string{
		{"", ""},
		{"abc", "abc"},
		{" a\tb\nc\x00d ", "abcd"},
		{"java\u00a0script", "javascript"},
		{"a\u1680b\u180ec\u2000d", "abcd"},
		{"a\u2029b\u205fc\u3000d", "abcd"},
		// U+202A and U+FEFF are not in the class. U+200B is.
		{"a\u202ab\u200bc\ufeff", "a\u202abc\ufeff"},
		{"a\u00e9\U0001F600", "a\u00e9\U0001F600"},
		// An invalid byte is kept.
		{"\xff a", "\xffa"},
		{"a\u2028b\u2010c", "abc"},
	}
	for _, c := range cases {
		if got := stripAttrWhitespace(c[0]); got != c[1] {
			t.Errorf("stripAttrWhitespace(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}
