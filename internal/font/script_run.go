// Ported from Chromium (Blink):
// third_party/blink/renderer/platform/fonts/script_run_iterator.cc
// (BSD-3-Clause, Copyright 2015 The Chromium Authors, LICENSE-Chromium.txt).
// The port does not record the version. Chromium 156.0.8078.4, the
// reference browser, has the file with this license.

package font

import "github.com/go-text/typesetting/language"

// Blink splits a text into runs of one script before it shapes the text
// (RunSegmenter). HarfBuzz shapes each run alone with the script of the run,
// so that a kerning pair or a ligature does not apply across the limit of
// two runs.
//
// The port uses the Script property of a character only. Blink also reads
// the Script_Extensions property, which gives a few characters of the script
// Common (for example the ideographic full stop) a set of scripts that they
// can join. Here such a character joins every run.

// scriptRun is the run text[start:end] of one script.
type scriptRun struct {
	start, end int
	script     language.Script
}

// maxBrackets is kMaxBrackets: the depth of the stack of open brackets.
const maxBrackets = 32

// bracketRec is BracketRec: an open bracket and the script of its run.
type bracketRec struct {
	ch     rune
	script language.Script
}

// bracketPairs lists the paired brackets of Unicode (BidiBrackets.txt): each
// opening bracket and its closing bracket.
var bracketPairs = [...][2]rune{
	{0x28, 0x29}, {0x5b, 0x5d}, {0x7b, 0x7d}, {0xf3a, 0xf3b}, {0xf3c, 0xf3d}, {0x169b, 0x169c},
	{0x2045, 0x2046}, {0x207d, 0x207e}, {0x208d, 0x208e}, {0x2308, 0x2309}, {0x230a, 0x230b},
	{0x2329, 0x232a}, {0x2768, 0x2769}, {0x276a, 0x276b}, {0x276c, 0x276d}, {0x276e, 0x276f},
	{0x2770, 0x2771}, {0x2772, 0x2773}, {0x2774, 0x2775}, {0x27c5, 0x27c6}, {0x27e6, 0x27e7},
	{0x27e8, 0x27e9}, {0x27ea, 0x27eb}, {0x27ec, 0x27ed}, {0x27ee, 0x27ef}, {0x2983, 0x2984},
	{0x2985, 0x2986}, {0x2987, 0x2988}, {0x2989, 0x298a}, {0x298b, 0x298c}, {0x298d, 0x2990},
	{0x298f, 0x298e}, {0x2991, 0x2992}, {0x2993, 0x2994}, {0x2995, 0x2996}, {0x2997, 0x2998},
	{0x29d8, 0x29d9}, {0x29da, 0x29db}, {0x29fc, 0x29fd}, {0x2e22, 0x2e23}, {0x2e24, 0x2e25},
	{0x2e26, 0x2e27}, {0x2e28, 0x2e29}, {0x2e55, 0x2e56}, {0x2e57, 0x2e58}, {0x2e59, 0x2e5a},
	{0x2e5b, 0x2e5c}, {0x3008, 0x3009}, {0x300a, 0x300b}, {0x300c, 0x300d}, {0x300e, 0x300f},
	{0x3010, 0x3011}, {0x3014, 0x3015}, {0x3016, 0x3017}, {0x3018, 0x3019}, {0x301a, 0x301b},
	{0xfe59, 0xfe5a}, {0xfe5b, 0xfe5c}, {0xfe5d, 0xfe5e}, {0xff08, 0xff09}, {0xff3b, 0xff3d},
	{0xff5b, 0xff5d}, {0xff5f, 0xff60}, {0xff62, 0xff63},
}

// pairedBracket returns the kind of a bracket: 1 for an opening bracket and
// 2 for a closing bracket, with the opening bracket that it closes. The kind
// is 0 for another character.
func pairedBracket(ch rune) (kind int, open rune) {
	if ch < 0x80 {
		switch ch {
		case '(', '[', '{':
			return 1, ch
		case ')':
			return 2, '('
		case ']':
			return 2, '['
		case '}':
			return 2, '{'
		}
		return 0, 0
	}
	for _, p := range bracketPairs {
		switch ch {
		case p[0]:
			return 1, ch
		case p[1]:
			return 2, p[0]
		}
	}
	return 0, 0
}

// unresolvedScript reports whether a script is Common or Inherited: a
// character of such a script takes the script of its run.
func unresolvedScript(s language.Script) bool {
	return s == language.Common || s == language.Inherited
}

// scriptRuns is ScriptRunIterator: it splits text into runs of one script.
// The script of a run that has no letter is Common.
func scriptRuns(text []rune) []scriptRun {
	if len(text) == 0 {
		return nil
	}
	var runs []scriptRun
	// The current script starts as Common, so that the first character that
	// has a script gives the script of the first run.
	current := language.Common
	var brackets []bracketRec
	bracketsFixupDepth := 0
	// fixupStack gives the brackets that were opened in the run that ends
	// the script of that run.
	fixupStack := func(resolved language.Script) {
		if bracketsFixupDepth > 0 {
			if bracketsFixupDepth > len(brackets) {
				bracketsFixupDepth = 0
			}
			for i := 0; i < bracketsFixupDepth; i++ {
				brackets[len(brackets)-1-i].script = resolved
			}
			bracketsFixupDepth = 0
		}
	}
	start := 0
	for pos, ch := range text {
		next := language.LookupScript(ch)
		switch kind, open := pairedBracket(ch); kind {
		case 1:
			// OpenBracket
			if len(brackets) == maxBrackets {
				brackets = brackets[1:]
				if bracketsFixupDepth == maxBrackets {
					bracketsFixupDepth--
				}
			}
			brackets = append(brackets, bracketRec{ch: ch, script: language.Common})
			bracketsFixupDepth++
		case 2:
			// CloseBracket: use the script of the matching open bracket, and
			// pop the stack to that bracket.
			for i := len(brackets) - 1; i >= 0; i-- {
				if brackets[i].ch == open {
					next = brackets[i].script
					numPopped := len(brackets) - 1 - i
					brackets = brackets[:len(brackets)-numPopped]
					bracketsFixupDepth = max(bracketsFixupDepth-numPopped, 0)
					break
				}
			}
		}
		// MergeSets, for sets of one script.
		switch {
		case unresolvedScript(next):
			// The character goes with the current run.
		case unresolvedScript(current):
			current = next
		case current != next:
			runs = append(runs, scriptRun{start: start, end: pos, script: current})
			fixupStack(current)
			current = next
			start = pos
		}
	}
	runs = append(runs, scriptRun{start: start, end: len(text), script: current})
	return runs
}
