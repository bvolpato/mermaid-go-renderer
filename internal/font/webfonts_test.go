package font

import (
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/webfonts"
)

// TestPageFontFaceRules checks the @font-face rules that the library reads
// from the style sheets of internal/webfonts: the families, the segmented
// faces and the ranges. The numbers are those of document.fonts in the
// page of oracle.mjs --webfonts.
func TestPageFontFaceRules(t *testing.T) {
	set := pageWebFonts()
	type want struct {
		family string
		// Each segmented face: slope, weight range and number of rules.
		sets [][4]float64
	}
	regular := [4]float64{0, 400, 400, 1}
	bold := [4]float64{0, 700, 700, 1}
	italic := [4]float64{italicSlope, 400, 400, 1}
	boldItalic := [4]float64{italicSlope, 700, 700, 1}
	wants := []want{
		{"font awesome 7 brands", [][4]float64{{0, 400, 400, 1}}},
		{"font awesome 7 free", [][4]float64{{0, 400, 400, 1}, {0, 900, 900, 1}}},
		{"recursive variable", [][4]float64{{0, 300, 1000, 4}}},
		{"open sans", [][4]float64{{0, 400, 400, 10}, {italicSlope, 400, 400, 10}}},
		{"katex_ams", [][4]float64{regular}},
		{"katex_caligraphic", [][4]float64{bold, regular}},
		{"katex_fraktur", [][4]float64{bold, regular}},
		{"katex_main", [][4]float64{bold, boldItalic, italic, regular}},
		{"katex_math", [][4]float64{boldItalic, italic}},
		{"katex_sansserif", [][4]float64{bold, italic, regular}},
		{"katex_script", [][4]float64{regular}},
		{"katex_size1", [][4]float64{regular}},
		{"katex_size2", [][4]float64{regular}},
		{"katex_size3", [][4]float64{regular}},
		{"katex_size4", [][4]float64{regular}},
		{"katex_typewriter", [][4]float64{regular}},
	}
	if len(set.families) != len(wants) {
		t.Errorf("%d families, want %d", len(set.families), len(wants))
	}
	rules := 0
	for _, w := range wants {
		family := set.families[w.family]
		if family == nil {
			t.Errorf("no family %q", w.family)
			continue
		}
		if len(family.sets) != len(w.sets) {
			t.Errorf("%s: %d segmented faces, want %d", w.family, len(family.sets), len(w.sets))
			continue
		}
		for i, s := range family.sets {
			got := [4]float64{s.slopeMin, s.weightMin, s.weightMax, float64(len(s.rules))}
			if s.slopeMin != s.slopeMax || got != w.sets[i] {
				t.Errorf("%s face %d: slope, weights and rules %v, want %v", w.family, i, got, w.sets[i])
			}
			for _, rule := range s.rules {
				rules++
				if _, err := webfonts.FS.ReadFile(rule.url); err != nil {
					t.Errorf("%s: the source %q is not a file of internal/webfonts", w.family, rule.url)
				}
			}
		}
	}
	if rules != 47 {
		t.Errorf("%d rules, want 47", rules)
	}

	// The ranges of Recursive Variable: the rule of the Latin file comes
	// last in the style sheet, so it is the first font of the face.
	recursive := set.families["recursive variable"].sets[0]
	latin := recursive.rules[len(recursive.rules)-1]
	for _, r := range []rune{' ', 'A', 0xff, 0x131, 0x2013, 0x20ac, 0xfffd} {
		if !latin.ranges.contains(r) {
			t.Errorf("the Latin rule of Recursive does not have U+%04X", r)
		}
	}
	for _, r := range []rune{0x100, 0x400, 0x1ea0, 0x4e2d, 0xf007} {
		if latin.ranges.contains(r) {
			t.Errorf("the Latin rule of Recursive has U+%04X", r)
		}
	}
	// A rule without unicode-range has every character.
	solid := set.families["font awesome 7 free"].sets[1].rules[0]
	if solid.ranges != nil || !solid.ranges.contains(0xf007) {
		t.Errorf("the rule of Font Awesome has a range")
	}
}

// TestFontFaceRuleParser checks the descriptors that the parser reads.
func TestFontFaceRuleParser(t *testing.T) {
	sheet := `/* a comment with @font-face { } */
.a::before { content: "@font-face {"; }
@font-face{font-family:Some  Name;font-style:oblique;font-weight:bold;
  src:local("X"),url(a.svg) format("svg"),url('b.woff2') format("woff2"),url(c.woff) format("woff");
  unicode-range:U+41-5A, u+0061-007a, U+4??, U+20AC}
@media print { @font-face { font-family: "Quoted \22 Name"; src: url("d.ttf"); font-weight: 200 900 } }
@font-face { font-family: NoSource; }
@font-face { src: url(e.woff2); }`
	rules := parseFontFaceRules(sheet)
	if len(rules) != 2 {
		t.Fatalf("%d rules, want 2", len(rules))
	}
	r := rules[0]
	if r.family != "Some Name" || r.slopeMin != italicSlope || r.slopeMax != italicSlope || r.weightMin != 700 || r.weightMax != 700 || r.url != "b.woff2" {
		t.Errorf("first rule: %+v", r)
	}
	wantRanges := [][2]rune{{0x41, 0x5a}, {0x61, 0x7a}, {0x400, 0x4ff}, {0x20ac, 0x20ac}}
	if r.ranges == nil || len(r.ranges.ranges) != len(wantRanges) {
		t.Fatalf("ranges of the first rule: %+v", r.ranges)
	}
	for i, w := range wantRanges {
		if r.ranges.ranges[i] != w {
			t.Errorf("range %d: %x, want %x", i, r.ranges.ranges[i], w)
		}
	}
	r = rules[1]
	if r.family != `Quoted "Name` || r.slopeMin != 0 || r.weightMin != 200 || r.weightMax != 900 || r.url != "d.ttf" || r.ranges != nil {
		t.Errorf("second rule: %+v", r)
	}
	// Ranges that touch or overlap become one range.
	joined := newRangeSet([][2]rune{{0x30, 0x39}, {0x20, 0x2f}, {0x35, 0x50}, {0x100, 0x100}})
	if len(joined.ranges) != 2 || joined.ranges[0] != [2]rune{0x20, 0x50} || !joined.contains(0x100) || joined.contains(0x51) {
		t.Errorf("joined ranges: %x", joined.ranges)
	}
}

// TestWebFontSelection checks the segmented face that a text style gets
// (FontSelectionAlgorithm) and the styles that Skia makes for it.
func TestWebFontSelection(t *testing.T) {
	set := pageWebFonts()
	free := set.families["font awesome 7 free"]
	for weight := 100; weight <= 900; weight += 100 {
		// Up to 500 the regular face is the closest, above it the solid
		// face.
		want := 400.0
		if weight > 500 {
			want = 900
		}
		if got := free.pick(weight, false).weightMin; got != want {
			t.Errorf("Font Awesome 7 Free at %d: the face of weight %v, want %v", weight, got, want)
		}
	}
	sans := set.families["open sans"]
	if sans.pick(700, true).slopeMin != italicSlope || sans.pick(700, false).slopeMin != 0 {
		t.Errorf("Open Sans: the slant of the face is not the slant of the text")
	}

	lib := Embedded().WithWebFonts()
	if lib.WithWebFonts() != lib || Embedded().WithWebFonts() != lib {
		t.Errorf("WithWebFonts does not give one library")
	}
	type style struct {
		family         string
		weight         int
		italic         bool
		wght           float32
		syntheticBold  bool
		syntheticSlant bool
	}
	for _, s := range []style{
		{"Recursive Variable", 400, false, 400, false, false},
		{"Recursive Variable", 100, false, 300, false, false},
		{"Recursive Variable", 700, true, 700, false, true},
		{"Recursive Variable", 950, false, 950, false, false},
		{"Open Sans", 400, false, 0, false, false},
		{"Open Sans", 500, false, 0, false, false},
		{"Open Sans", 600, false, 0, true, false},
		{"Open Sans", 700, true, 0, true, false},
		{"Font Awesome 7 Free", 400, true, 0, false, true},
		{"Font Awesome 7 Free", 700, false, 0, false, false},
		{"Font Awesome 7 Brands", 900, false, 0, true, false},
	} {
		face := lib.Resolve([]string{s.family}, s.weight, s.italic).Primary()
		if face == nil || face.web == nil {
			t.Errorf("%+v: no web font face", s)
			continue
		}
		if face.web.wght != s.wght || face.web.syntheticBold != s.syntheticBold || face.web.syntheticItalic != s.syntheticSlant {
			t.Errorf("%+v: wght %v, synthetic bold %v, synthetic italic %v", s, face.web.wght, face.web.syntheticBold, face.web.syntheticItalic)
		}
	}
	// Text styles with the same variation and the same synthetic styles
	// share a face.
	a := lib.Resolve([]string{"Open Sans"}, 400, false).Primary()
	b := lib.Resolve([]string{"Open Sans"}, 500, false).Primary()
	c := lib.Resolve([]string{"Open Sans"}, 600, false).Primary()
	if a != b || a == c {
		t.Errorf("Open Sans: the faces of the weights 400, 500 and 600 are not shared as they must be")
	}
}

// TestWebFontFallback checks which font draws a character: the first font
// of the family whose range has the character, then the next family of the
// list, then the standard font.
func TestWebFontFallback(t *testing.T) {
	lib := Embedded().WithWebFonts()
	chain := lib.Resolve(css.SplitFontFamily(`"Recursive Variable", arial, sans-serif`), 400, false)
	file := func(text string) string {
		runs := chain.ShapeText([]rune(text), 14)
		if len(runs) != 1 {
			t.Fatalf("%q: %d runs", text, len(runs))
		}
		return runs[0].Face.Path
	}
	const dir = "webfont:files/recursive/files/"
	for text, want := range map[string]string{
		"Hello":  dir + "recursive-latin-wght-normal.woff2",
		"Łł":     dir + "recursive-latin-ext-wght-normal.woff2",
		"ệ":      dir + "recursive-vietnamese-wght-normal.woff2",
		"Привет": "embedded:LiberationSans-Regular.ttf",
		// A character that no font has gets the missing glyph of the
		// first font that the text could use: Arial, because no range of
		// Recursive has the character.
		"\uf007": "embedded:LiberationSans-Regular.ttf",
	} {
		if got := file(text); got != want {
			t.Errorf("%q: the font is %s, want %s", text, got, want)
		}
	}
	// A list of web fonts only ends with the standard font of the browser.
	alone := lib.Resolve([]string{"Recursive Variable"}, 400, false)
	if runs := alone.ShapeText([]rune("Привет"), 14); len(runs) != 1 || runs[0].Face.Path != "embedded:LiberationSerif-Regular.ttf" {
		t.Errorf("Cyrillic text with Recursive alone does not use the standard font")
	}
	if p := alone.Primary(); p == nil || p.Path != dir+"recursive-latin-wght-normal.woff2" {
		t.Errorf("the primary font of Recursive is not its Latin font")
	}
}

// TestWebFontsLoadLazily checks that a library with web fonts parses a
// font file when a text first uses a face of it, and one time.
func TestWebFontsLoadLazily(t *testing.T) {
	base := NewLibraryFor(PlatformLinux)
	lib := base.WithWebFonts()
	if n := lib.ParsedFonts(); n != 0 {
		t.Fatalf("a new library has %d parsed fonts", n)
	}
	chain := lib.Resolve([]string{"Open Sans"}, 400, false)
	if n := lib.webParsing; n != 0 {
		t.Errorf("Resolve parsed %d web fonts", n)
	}
	before := webFontsParsed()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chain.ShapeText([]rune("Hello"), 14)
			lib.Resolve([]string{"Open Sans"}, 400, false).ShapeText([]rune("World"), 16)
		}()
	}
	wg.Wait()
	// Latin text needs the Latin file only. Another test can have parsed
	// it before: the process parses a file one time.
	if n := webFontsParsed() - before; n > 1 {
		t.Errorf("Latin text in Open Sans parsed %d font files", n)
	}
	if n := lib.ParsedFonts(); n > 1 {
		t.Errorf("the library counts %d parsed fonts for one font file", n)
	}
}

// webFontsParsed returns the number of web font files that the process has
// parsed.
func webFontsParsed() int {
	webFontsMu.Lock()
	defer webFontsMu.Unlock()
	n := 0
	for _, wf := range webFontsBy {
		if wf.font != nil {
			n++
		}
	}
	return n
}

// TestWebFontsGenericFamilies checks the generic families of the two
// libraries with web fonts. The page of mermaid-cli runs in the headless
// shell of Chromium, whose generic families are the defaults of Blink. The
// page of oracle.mjs runs in Chromium, which has the generic families of
// the library without web fonts.
func TestWebFontsGenericFamilies(t *testing.T) {
	for name, want := range map[string]string{
		"serif": "Times New Roman", "sans-serif": "Arial", "cursive": "Script", "fantasy": "Impact",
	} {
		for _, platform := range []Platform{PlatformLinux, PlatformDarwin} {
			if got, ok := shellGenericFamily(platform, name); !ok || got != want {
				t.Errorf("%s on %v: %q, want %q", name, platform, got, want)
			}
		}
	}
	if got, _ := shellGenericFamily(PlatformDarwin, "monospace"); got != "Menlo" {
		t.Errorf("monospace on macOS: %q, want Menlo", got)
	}
	if got, _ := shellGenericFamily(PlatformLinux, "monospace"); got != "Courier New" {
		t.Errorf("monospace on Linux: %q, want Courier New", got)
	}
	if _, ok := shellGenericFamily(PlatformDarwin, "system-ui"); ok {
		t.Errorf("system-ui has a family of the headless shell")
	}

	base := Embedded()
	shell, chromium := base.WithWebFonts(), base.WithWebFontsInChromium()
	if shell == chromium || chromium.WithWebFontsInChromium() != chromium || base.WithWebFontsInChromium() != chromium {
		t.Fatalf("the two libraries with web fonts are not two libraries, each made one time")
	}
	// With only the Liberation fonts, every generic family gives the same
	// font in both libraries, and a text without a family gets the standard
	// font, Times New Roman, which is Liberation Serif.
	for _, lib := range []*Library{shell, chromium} {
		for families, want := range map[string]string{
			"sans-serif": "embedded:LiberationSans-Regular.ttf",
			"serif":      "embedded:LiberationSerif-Regular.ttf",
			"monospace":  "embedded:LiberationMono-Regular.ttf",
			"":           "embedded:LiberationSerif-Regular.ttf",
		} {
			var list []string
			if families != "" {
				list = []string{families}
			}
			if p := lib.Resolve(list, 400, false).Primary(); p == nil || p.Path != want {
				t.Errorf("%q: the primary font is %v, want %s", families, p, want)
			}
		}
	}
}
