// Ported from fontconfig 2.15.0, in part: this file follows src/fcmatch.c
// (FcFontSort and the order of its criteria) and src/fcdir.c and
// src/fccfg.c (the order of the fonts) (HPND-sell-variant, license of
// fontconfig, LICENSE-fontconfig.txt). It also follows Chromium
// 156.0.8078.80 (ui/gfx/linux/fontconfig_util.cc and
// ui/gfx/font_fallback_linux.cc: the request for a font that has a
// character; BSD 3-clause license, LICENSE-Chromium.txt).

package font

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
)

// fontconfigDefault is the configuration of the reference environment, for
// a machine that has no fontconfig configuration.
// tools/reference/gen-fontconfig-default.mjs writes it.
//
//go:embed fontconfig-default.xml.gz
var fontconfigDefault []byte

// fcFont is a font file of a library, as fontconfig lists it.
type fcFont struct {
	family  string // the family name of the font
	key     string // fcFamilyKey(family)
	weight  int    // CSS weight
	italic  bool
	version int32 // the font revision of the head table
	noLatin bool  // the font does not have the Latin letters
	path    string
	index   int
}

// fcSystem is the fontconfig of a Linux library: the configuration and the
// fonts in the order in which fontconfig has them.
type fcSystem struct {
	lib    *Library
	config *fcConfig
	fonts  []*fcFont

	mu      sync.Mutex
	matches map[fcMatchKey]fcMatchResult
	chars   map[fcCharKey]*Face
}

type fcMatchKey struct {
	name   string
	weight int
	italic bool
}

type fcMatchResult struct {
	font       *fcFont
	postConfig string
}

type fcCharKey struct {
	r     rune
	emoji bool
}

// fontconfig returns the fontconfig of the library, or nil for a library
// that has only the embedded fonts: the rules for those are in linux.go.
func (l *Library) fontconfig() *fcSystem {
	if len(l.dirs) == 0 {
		return nil
	}
	l.fcOnce.Do(func() {
		s := &fcSystem{lib: l, matches: map[fcMatchKey]fcMatchResult{}, chars: map[fcCharKey]*Face{}}
		s.config = l.loadFontconfig()
		s.fonts = l.fontconfigFonts()
		l.fc = s
	})
	return l.fc
}

// loadFontconfig reads the configuration of the library: the directory
// that was given for it, else the configuration of the machine
// (FONTCONFIG_FILE, FONTCONFIG_PATH, /etc/fonts), else the configuration of
// the reference environment.
func (l *Library) loadFontconfig() *fcConfig {
	var file string
	switch {
	case l.fcDir != "":
		file = filepath.Join(l.fcDir, "fonts.conf")
	case l.fcSystemConfig:
		if f := os.Getenv("FONTCONFIG_FILE"); f != "" {
			file = f
		} else if d := os.Getenv("FONTCONFIG_PATH"); d != "" {
			file = filepath.Join(d, "fonts.conf")
		} else {
			file = "/etc/fonts/fonts.conf"
		}
	}
	if file != "" {
		if _, err := os.Stat(file); err == nil {
			open, list := fcFileSystem()
			return loadFontconfig(file, open, list)
		}
	}
	open := func(string) (io.ReadCloser, error) {
		return gzip.NewReader(bytes.NewReader(fontconfigDefault))
	}
	return loadFontconfig("fontconfig-default.xml", open, func(string) []string { return nil })
}

// fcPathLess orders two font files as fontconfig lists them. fontconfig
// adds the fonts of a directory in the order of the file names, and puts
// the subdirectories at the end of the list of directories that it still
// has to read (FcConfigAddDirList and FcConfigAddCache). So the fonts of a
// directory come before the fonts of every directory that is deeper, and
// directories of the same depth are in the order of their names.
func fcPathLess(a, b string) bool {
	as := strings.Split(filepath.ToSlash(a), "/")
	bs := strings.Split(filepath.ToSlash(b), "/")
	if len(as) != len(bs) {
		return len(as) < len(bs)
	}
	for i := range as {
		if as[i] != bs[i] {
			return as[i] < bs[i]
		}
	}
	return false
}

// fontconfigFonts lists the fonts of the library in the order of
// fontconfig: the directories in their order, and the embedded fonts last.
func (l *Library) fontconfigFonts() []*fcFont {
	l.mu.Lock()
	l.buildIndex()
	entries := append([]indexEntry(nil), l.index...)
	l.mu.Unlock()
	dirOf := func(path string) int {
		for i, dir := range l.dirs {
			if strings.HasPrefix(path, dir) {
				return i
			}
		}
		return len(l.dirs)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if da, db := dirOf(a.Path), dirOf(b.Path); da != db {
			return da < db
		}
		if a.Path != b.Path {
			return fcPathLess(a.Path, b.Path)
		}
		return a.Index < b.Index
	})
	var fonts []*fcFont
	have := map[string]bool{}
	for _, e := range entries {
		f := &fcFont{family: e.Family, key: fcFamilyKey(e.Family), weight: e.Weight, italic: e.Italic, version: e.Version, noLatin: e.NoLatin, path: e.Path, index: e.Index}
		have[f.key] = true
		fonts = append(fonts, f)
	}
	if l.embed == embedFonts {
		// The embedded fonts come last. A family that a directory has is
		// not listed two times.
		l.mu.Lock()
		for _, family := range []string{"liberation mono", "liberation sans", "liberation serif"} {
			if have[fcFamilyKey(family)] {
				continue
			}
			for _, ref := range l.embeddedRefs(family) {
				// Liberation 2.1.5 has the font revision 2.1.
				fonts = append(fonts, &fcFont{family: ref.family, key: fcFamilyKey(ref.family), weight: ref.weight, italic: ref.italic, version: 0x2199a, path: ref.path, index: ref.index})
			}
		}
		l.mu.Unlock()
	}
	return fonts
}

// face loads the face of a font.
func (s *fcSystem) face(f *fcFont) *Face {
	l := s.lib
	l.mu.Lock()
	defer l.mu.Unlock()
	if strings.HasPrefix(f.path, embeddedPrefix) {
		for _, ref := range l.embedded {
			if ref.path == f.path && ref.index == f.index {
				return l.faceOf(ref)
			}
		}
		return nil
	}
	for _, ref := range l.scanFile(f.path) {
		if ref.index == f.index {
			return l.faceOf(ref)
		}
	}
	return nil
}

// fcNoMatch is the family score of a font that has none of the families of
// a request.
const fcNoMatch = 1 << 30

// sorted returns the fonts in the order of FcFontSort for a request. The
// criteria are, in this order: the family values with a strong binding,
// the language, the family values with a weak binding, the slant, the
// weight, the font version (the newer font first), and the order of the
// font files. An earlier family of the request is a better match than a
// later one.
//
// The language of the requests of Chromium is the language of the page. A
// font supports it when it has the Latin letters: a font of symbols or of
// emoji does not, and comes after the other fonts. For a request in the
// language of emoji (emoji is true) the language is not compared: the
// default configuration gives the emoji fonts as strong families.
//
// internal/font/testdata/linux-image-*.json.gz has what Chromium chooses in
// the Playwright image.
func (s *fcSystem) sorted(p *fcPattern, weight int, italic, emoji bool) []*fcFont {
	type scored struct {
		font         *fcFont
		strong, weak int
		lang         int
		style        int
		order        int
	}
	list := make([]scored, len(s.fonts))
	wantWeight := fcWeight(weight)
	for i, f := range s.fonts {
		sc := scored{font: f, strong: fcNoMatch, weak: fcNoMatch, order: i}
		for j, v := range p.family {
			if fcFamilyKey(v.s) != f.key {
				continue
			}
			if v.strong {
				sc.strong = min(sc.strong, j)
			} else {
				sc.weak = min(sc.weak, j)
			}
		}
		if f.noLatin && !emoji {
			sc.lang = 1
		}
		slant := 0
		if f.italic != italic {
			slant = 100
		}
		d := wantWeight - fcWeight(f.weight)
		if d < 0 {
			d = -d
		}
		sc.style = slant*1000 + d
		list[i] = sc
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.strong != b.strong {
			return a.strong < b.strong
		}
		if a.lang != b.lang {
			return a.lang < b.lang
		}
		if a.weak != b.weak {
			return a.weak < b.weak
		}
		if a.style != b.style {
			return a.style < b.style
		}
		if a.font.version != b.font.version {
			return a.font.version > b.font.version
		}
		return a.order < b.order
	})
	out := make([]*fcFont, len(list))
	for i := range list {
		out[i] = list[i].font
	}
	return out
}

// match is the first font of FcFontSort for a family name, a weight and a
// slant, as SkFontConfigInterfaceDirect::matchFamilyName asks for it. The
// second result is the first family of the request after the rules of the
// configuration.
func (s *fcSystem) match(name string, weight int, italic bool) (*fcFont, string) {
	key := fcMatchKey{name, weight, italic}
	s.mu.Lock()
	res, ok := s.matches[key]
	s.mu.Unlock()
	if ok {
		return res.font, res.postConfig
	}
	p := &fcPattern{}
	if name != "" {
		p.family = []fcValue{{name, true}}
	}
	s.config.substitute(p)
	if len(p.family) > 0 {
		res.postConfig = p.family[0].s
	}
	if fonts := s.sorted(p, weight, italic, false); len(fonts) > 0 {
		res.font = fonts[0]
	}
	s.mu.Lock()
	s.matches[key] = res
	s.mu.Unlock()
	return res.font, res.postConfig
}

// charFallback is gfx::GetFallbackFontForChar: the first font of
// FcFontSort that has a character, for a request with the language of the
// page and no family. emoji selects the language that Blink gives for a
// character with emoji presentation (und-Zsye), for which the default
// configuration prefers the emoji fonts.
func (s *fcSystem) charFallback(r rune, emoji bool) *Face {
	key := fcCharKey{r, emoji}
	s.mu.Lock()
	face, ok := s.chars[key]
	s.mu.Unlock()
	if ok {
		return face
	}
	lang := "en-us"
	if emoji {
		lang = "und-zsye"
	}
	p := &fcPattern{lang: []fcValue{{lang, true}}}
	s.config.substitute(p)
	for _, f := range s.sorted(p, 400, false, emoji) {
		if candidate := s.face(f); candidate != nil && candidate.HasGlyph(r) {
			face = s.fallbackFace(f)
			break
		}
	}
	s.mu.Lock()
	s.chars[key] = face
	s.mu.Unlock()
	return face
}

// matchFace is SkFontConfigInterfaceDirect::matchFamilyName: the face that
// fontconfig gives for a family name, or nil when Skia does not accept it.
// name is in lower case.
//
// Skia accepts the font when the request allows any font (the names sans,
// serif and monospace), when the font has the name of the request or the
// first family of the request after the rules of the configuration, or
// when the two names are in one class of metric-compatible fonts.
func (s *fcSystem) matchFace(name string, weight int, italic bool) *Face {
	font, postConfig := s.match(name, weight, italic)
	if font == nil {
		return nil
	}
	if !linuxFallbackAllowed(name) {
		family := strings.ToLower(font.family)
		class := linuxEquivClass(name)
		if !strings.EqualFold(postConfig, font.family) && name != family && (class < 0 || class != linuxEquivClass(family)) {
			return nil
		}
	}
	return s.face(font)
}

// systemFont returns the face of the system font: the family that
// fontconfig gives for a request without a family, at the weight and the
// slant.
func (s *fcSystem) systemFont(weight int, italic bool) *Face {
	font, _ := s.match("", 400, false)
	if font == nil {
		return nil
	}
	styled, _ := s.match(strings.ToLower(font.family), weight, italic)
	if styled == nil {
		styled = font
	}
	return s.face(styled)
}

// fcIndexInfo reads from a font what the sort of fontconfig needs beyond
// the family and the style: the font revision of the head table (the
// property fontversion: fontconfig prefers the newer font), and whether the
// font has the Latin letters (the fonts without them do not support the
// language of the page).
func fcIndexInfo(ld *ot.Loader) (version int32, latin bool) {
	if head, err := ld.RawTable(ot.MustNewTag("head")); err == nil && len(head) >= 8 {
		version = int32(binary.BigEndian.Uint32(head[4:]))
	}
	if ft, err := gtfont.NewFont(ld); err == nil {
		face := gtfont.NewFace(ft)
		_, a := face.NominalGlyph('a')
		_, z := face.NominalGlyph('Z')
		latin = a && z
	}
	return version, latin
}

// fallbackFace loads the face of a font for use as the fallback font of a
// character. It is a face of its own, because Blink gives a fallback font
// other font settings than a font that the page names (see
// Face.defaultStyle).
func (s *fcSystem) fallbackFace(f *fcFont) *Face {
	l := s.lib
	key := f.path + "\x00" + strconv.Itoa(f.index)
	l.mu.Lock()
	defer l.mu.Unlock()
	face, ok := l.fallbackFaces[key]
	if !ok {
		var data []byte
		var err error
		if name, embedded := strings.CutPrefix(f.path, embeddedPrefix); embedded {
			data, err = liberation.Font(name)
		} else {
			data, err = os.ReadFile(f.path)
		}
		if err == nil {
			l.parses++
			face, _ = loadFaceAt(l.platform, data, f.index)
		}
		if face != nil {
			face.Path = f.path
			face.defaultStyle = true
		}
		l.fallbackFaces[key] = face
	}
	return face
}
