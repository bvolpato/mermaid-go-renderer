package marked

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

// These tests cover the code that saves time and that is not a port: the
// prefilters of the rules, the plain text path of Lexer, and the functions
// that read a rule by hand. Each test compares the code with the compiled
// rule of marked.

// goldenInputs returns the inputs of lexer.json.gz as code units.
func goldenInputs(t testing.TB) [][]rune {
	t.Helper()
	var cases []goldenCase
	loadGolden(t, "lexer.json.gz", &cases)
	out := make([][]rune, len(cases))
	for i := range cases {
		out[i] = units(cases[i].input(t))
	}
	return out
}

// checker runs a check on many strings in several goroutines. The check
// returns a message when it fails.
type checker struct {
	t      *testing.T
	check  func(s []rune) string
	n      atomic.Int64
	failed atomic.Int64
	wg     sync.WaitGroup
}

func (c *checker) one(s []rune) {
	c.n.Add(1)
	if msg := c.check(s); msg != "" && c.failed.Add(1) <= 5 {
		c.t.Error(msg)
	}
}

// words checks each string of 0 to maxLen parts of the alphabet.
func (c *checker) words(alphabet []string, maxLen int) {
	parts := make([][]rune, len(alphabet))
	for i, a := range alphabet {
		parts[i] = []rune(a)
	}
	var walk func(prefix []rune, depth int)
	walk = func(prefix []rune, depth int) {
		c.one(prefix)
		if depth == maxLen {
			return
		}
		for _, p := range parts {
			walk(append(prefix[:len(prefix):len(prefix)], p...), depth+1)
		}
	}
	c.one(nil)
	for _, p := range parts {
		c.wg.Go(func() { walk(p, 1) })
	}
}

// each checks the strings that visit gives, for each input.
func (c *checker) each(inputs [][]rune, visit func(s []rune, check func(s []rune))) {
	const workers = 8
	for w := range workers {
		c.wg.Go(func() {
			for i := w; i < len(inputs); i += workers {
				visit(inputs[i], c.one)
			}
		})
	}
}

// wait returns the number of strings after all checks.
func (c *checker) wait() int64 {
	c.wg.Wait()
	if n := c.failed.Load(); n > 0 {
		c.t.Errorf("%d of %d strings failed", n, c.n.Load())
	}
	return c.n.Load()
}

// namedRegexes returns each compiled rule with its name.
func namedRegexes() map[string]*regex {
	out := map[string]*regex{}
	// The fields are not exported, so each value is read through its address.
	readable := func(v reflect.Value) reflect.Value {
		return reflect.NewAt(v.Type(), v.Addr().UnsafePointer()).Elem()
	}
	var visit func(name string, v reflect.Value)
	visit = func(name string, v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if re, ok := v.Interface().(*regex); ok && re != nil {
				out[name] = re
			}
		case reflect.Struct:
			for i := range v.NumField() {
				visit(name+"."+v.Type().Field(i).Name, readable(v.Field(i)))
			}
		case reflect.Array:
			for i := range v.Len() {
				visit(name+"."+string(rune('0'+i)), readable(v.Index(i)))
			}
		case reflect.Map:
			for _, k := range v.MapKeys() {
				visit(name+"."+string(rune(k.Int())), v.MapIndex(k))
			}
		}
	}
	visit("rules", reflect.ValueOf(getRules()).Elem())
	return out
}

// TestPrefilters checks the fields first and needs of each rule: when they
// exclude an input, the rule itself does not match it.
func TestPrefilters(t *testing.T) {
	inputs := goldenInputs(t)
	// The start characters: each ASCII character, and code units of other
	// kinds.
	starts := []rune{0x80, 0xa0, 0xe9, 0x2028, 0x3000, 0xfeff, 0xd83d, 0xde00, 0x4e2d}
	for c := rune(0); c < 128; c++ {
		starts = append(starts, c)
	}
	tails := [][]rune{nil}
	for i := 0; i < len(inputs) && len(tails) < 300; i += 11 {
		tails = append(tails, inputs[i])
	}

	regexes := namedRegexes()
	if len(regexes) < 60 {
		t.Fatalf("found %d rules", len(regexes))
	}
	withFirst, withNeeds := 0, 0
	for name, re := range regexes {
		if re.first != nil {
			withFirst++
			for _, c := range starts {
				if c < 128 && re.first[c] {
					continue
				}
				for _, tail := range tails {
					s := append([]rune{c}, tail...)
					if ok, _ := re.re.MatchRunes(s); ok {
						t.Fatalf("%s matches %q, and first excludes it", name, string(s))
					}
				}
			}
		}
		if re.needs != 0 {
			withNeeds++
			for _, s := range inputs {
				if indexOf(s, re.needs) >= 0 {
					continue
				}
				if ok, _ := re.re.MatchRunes(s); ok {
					t.Fatalf("%s matches %q, and needs excludes it", name, string(s))
				}
			}
		}
	}
	if withFirst < 20 || withNeeds < 3 {
		t.Fatalf("%d rules have first and %d have needs", withFirst, withNeeds)
	}
}

// TestInlineTextLen compares inlineTextLen with the inline text rule.
func TestInlineTextLen(t *testing.T) {
	re := getRules().inline.text
	c := &checker{t: t, check: func(s []rune) string {
		if len(s) == 0 {
			return ""
		}
		m := re.exec(s)
		if m == nil || m.Index != 0 {
			return "the text rule does not match " + string(s)
		}
		if got := inlineTextLen(s); got != m.Length {
			return "inlineTextLen(" + string(s) + ") differs from the rule"
		}
		return ""
	}}
	// Each string of up to 5 characters of the rule, and of up to 4 parts
	// that the rule names.
	c.words([]string{"`", "~", " ", "\n", "a", "@", "_", "!", "\\", "é", ".", "h", "w"}, 5)
	c.words([]string{"http", "s", "://", "ftp", "www", ".", "a", "@", " ", "  ", "\n", "_", "`", "~", "*", "<", "[", "{", "中", "😀"}, 4)
	// Each suffix of the inputs. Most inputs are plain text, and one in ten
	// of them is enough.
	inputs := goldenInputs(t)
	var sample [][]rune
	for i, s := range inputs {
		if len(s) <= 300 && (i%10 == 0 || !plainText(str(s))) {
			sample = append(sample, s)
		}
	}
	c.each(sample, func(s []rune, check func(s []rune)) {
		for j := range s {
			check(s[j:])
		}
	})
	if n := c.wait(); n < 1000000 {
		t.Fatalf("only %d strings", n)
	}
}

// TestURLStart checks that the url rule does not match when urlStart
// returns false.
func TestURLStart(t *testing.T) {
	re := getRules().inline.url
	var matches atomic.Int64
	c := &checker{t: t, check: func(s []rune) string {
		if matched, _ := re.re.MatchRunes(s); matched {
			matches.Add(1)
			if !urlStart(s) {
				return "the url rule matches " + string(s) + ", and urlStart excludes it"
			}
		}
		return ""
	}}
	c.words([]string{"h", "t", "p", "s", "f", "w", "W", "H", ":", "/", ".", "@", "a", "_", "+", "-", " ", "1"}, 4)
	c.words([]string{"http", "HTTP", "hTtPs", "s", "://", "ftp", "FTP", "www", "WwW", ".", "a", "@", " ", "a.b", "é", "-", "_", "K", "ſ"}, 4)
	c.each(goldenInputs(t), func(s []rune, check func(s []rune)) {
		for j := 0; j < len(s) && j < 20; j++ {
			check(s[j:])
		}
	})
	c.wait()
	if matches.Load() < 1000 {
		t.Fatalf("the url rule matched only %d strings", matches.Load())
	}
}

// TestSetextUnderline checks that the lheading rule does not match when
// setextUnderline returns false.
func TestSetextUnderline(t *testing.T) {
	re := getRules().block.lheading
	var matches atomic.Int64
	c := &checker{t: t, check: func(s []rune) string {
		if matched, _ := re.re.MatchRunes(s); matched {
			matches.Add(1)
			if !setextUnderline(s) {
				return "the lheading rule matches " + string(s) + ", and setextUnderline excludes it"
			}
		}
		return ""
	}}
	c.words([]string{"a", "\n", " ", "=", "-", "  ", "    ", "\t", "x y", "\n\n", "#", ">", "\xc2\xa0"}, 5)
	c.each(goldenInputs(t), func(s []rune, check func(s []rune)) {
		check(s)
		// The rule runs on the rest of the text after each block.
		for j, r := range s {
			if r == '\n' && j+1 < len(s) && j < 400 {
				check(s[j+1:])
			}
		}
	})
	c.wait()
	if matches.Load() < 1000 {
		t.Fatalf("the lheading rule matched only %d strings", matches.Load())
	}
}

// TestPlainText checks the plain text path of Lexer: for each text that
// plainText accepts, the lexer of marked returns the tokens of plainTokens.
func TestPlainText(t *testing.T) {
	var plain atomic.Int64
	c := &checker{t: t, check: func(s []rune) string {
		src := str(s)
		if !plainText(src) {
			return ""
		}
		plain.Add(1)
		tokens, err := lexSlow(src)
		if err != nil {
			return "lexSlow(" + src + "): " + err.Error()
		}
		if got, want := JSON(plainTokens(src)), JSON(tokens); got != want {
			return "plainText accepts " + src + "\n got " + got + "\nwant " + want
		}
		return ""
	}}
	// Each string of up to 4 parts. The parts are the characters that
	// plainText reads, the characters next to them in the rules, and
	// characters that have no meaning.
	c.words([]string{
		" ", "\t", "a", "1", ".", ")", "!", ":", "/", "w", "W", "-", "+", "#", ">", "=", "|", "(", "&", "é",
		"\n", "\r", "\\", "<", "[", "]", "`", "*", "~", "_", "@", "h", "t", "p", "s", "f", "www", "://", "😀", "\xc2\xa0",
	}, 4)
	c.words([]string{"http", "s", "ftp", "://", ":", "/", "www", "WWW", "wWw", ".", "w", "a", " ", "!", "1", "1.", "2)", "- ", "é", "x@y.z"}, 4)
	// The underscore between letters and digits, and next to other characters.
	c.words([]string{"_", "__", "a", "B", "1", "é", "中", " ", "!", ".", "-", "(", "\"", "😀"}, 5)
	c.each(goldenInputs(t), func(s []rune, check func(s []rune)) { check(s) })
	c.wait()
	if plain.Load() < 100000 {
		t.Fatalf("plainText accepted only %d strings", plain.Load())
	}
}
