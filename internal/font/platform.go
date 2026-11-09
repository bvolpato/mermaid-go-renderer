// This file ports no upstream file. It selects the rules of Chromium for one
// operating system, and makes the font library of each platform.

package font

import (
	"runtime"
	"strings"
	"sync"
)

// Platform selects the rules of Chromium for one operating system. The
// rules differ in three areas: which font a family name gives, how the font
// rasterizer of the system scales the metrics and the advances of a font,
// and how it computes the bounds of a glyph.
//
// The platform is a property of a Library and of the faces that it loads.
// So the rules of one platform can run on a machine of another platform.
type Platform uint8

const (
	// PlatformDarwin is macOS: CoreText gives the fonts and the metrics.
	PlatformDarwin Platform = iota + 1
	// PlatformLinux is Linux: fontconfig gives the fonts, and the font
	// backend of Skia (skrifa) gives the metrics.
	PlatformLinux
	// PlatformWindows is Windows: DirectWrite gives the fonts.
	PlatformWindows
)

// DefaultPlatform returns the platform of the machine that runs the
// program.
func DefaultPlatform() Platform {
	switch runtime.GOOS {
	case "darwin", "ios":
		return PlatformDarwin
	case "windows":
		return PlatformWindows
	}
	return PlatformLinux
}

// String returns the name of the platform.
func (p Platform) String() string {
	switch p {
	case PlatformDarwin:
		return "darwin"
	case PlatformLinux:
		return "linux"
	case PlatformWindows:
		return "windows"
	}
	return "unknown"
}

// embedMode says how a library uses the embedded Liberation fonts.
type embedMode uint8

const (
	// embedNone: the library has only the fonts of its directories.
	embedNone embedMode = iota
	// embedFonts: the embedded fonts are fonts of the library. A family of
	// the same name in a directory comes first.
	embedFonts
	// embedLastResort: an embedded font stands in for a standard family
	// (Arial, Helvetica, Times New Roman, Times, Courier New, Courier) that
	// is not installed, and for a font-family list that gives no font.
	embedLastResort
)

// defaultEmbedMode is the mode of the system library of a platform. On
// Linux the Liberation fonts are the fonts that Chromium uses for the
// standard families, so they are fonts of the library. On the other
// platforms they are the last resort only.
func defaultEmbedMode(p Platform) embedMode {
	if p == PlatformLinux {
		return embedFonts
	}
	return embedLastResort
}

// NewLibraryFor returns a library that reads fonts from the given
// directories and applies the rules of a platform. It has no embedded
// fonts.
func NewLibraryFor(platform Platform, dirs ...string) *Library {
	return newLibrary(platform, embedNone, dirs)
}

var (
	embeddedOnce sync.Once
	embeddedLib  *Library
)

// Embedded returns the hermetic library: the rules of Linux and only the
// embedded Liberation fonts. It does not read the file system, so it gives
// the same result on every machine. The result is the result of Chromium on
// Linux with only the Liberation fonts installed.
func Embedded() *Library {
	embeddedOnce.Do(func() {
		embeddedLib = newLibrary(PlatformLinux, embedFonts, nil)
	})
	return embeddedLib
}

// EmbeddedWith returns a library with the rules of Linux that has the fonts
// of the given directories and the embedded Liberation fonts. A family in a
// directory comes before the embedded family of the same name.
func EmbeddedWith(dirs ...string) *Library {
	return newLibrary(PlatformLinux, embedFonts, dirs)
}

// Platform returns the platform whose rules the library applies.
func (l *Library) Platform() Platform { return l.platform }

// Platform returns the platform whose rules the face applies.
func (f *Face) Platform() Platform { return f.platform }

// embeddedPrefix starts the path of a face of an embedded font file.
const embeddedPrefix = "embedded:"

// embeddedFamilies maps the lowercase name of an embedded family to the
// start of the names of its files.
var embeddedFamilies = map[string]string{
	"liberation sans":  "LiberationSans-",
	"liberation serif": "LiberationSerif-",
	"liberation mono":  "LiberationMono-",
}

// embeddedStandIn maps the lowercase name of a standard family to the
// embedded family that has the same metrics. Chromium on Linux uses the
// Liberation fonts for these names (see linuxFamily). On the other
// platforms the stand-in applies only when the family is not installed.
var embeddedStandIn = map[string]string{
	"arial":           "liberation sans",
	"helvetica":       "liberation sans",
	"times new roman": "liberation serif",
	"times":           "liberation serif",
	"courier new":     "liberation mono",
	"courier":         "liberation mono",
}

// embeddedFile is the description of an embedded font file: what a scan of
// the file gives. TestEmbeddedFiles compares the table with the files.
type embeddedFile struct {
	name   string
	family string
	weight int
	italic bool
}

// embeddedFiles lists the embedded font files in the order of their names,
// which is the order of liberation.Names.
var embeddedFiles = [...]embeddedFile{
	{"LiberationMono-Bold.ttf", "Liberation Mono", 700, false},
	{"LiberationMono-BoldItalic.ttf", "Liberation Mono", 700, true},
	{"LiberationMono-Italic.ttf", "Liberation Mono", 400, true},
	{"LiberationMono-Regular.ttf", "Liberation Mono", 400, false},
	{"LiberationSans-Bold.ttf", "Liberation Sans", 700, false},
	{"LiberationSans-BoldItalic.ttf", "Liberation Sans", 700, true},
	{"LiberationSans-Italic.ttf", "Liberation Sans", 400, true},
	{"LiberationSans-Regular.ttf", "Liberation Sans", 400, false},
	{"LiberationSerif-Bold.ttf", "Liberation Serif", 700, false},
	{"LiberationSerif-BoldItalic.ttf", "Liberation Serif", 700, true},
	{"LiberationSerif-Italic.ttf", "Liberation Serif", 400, true},
	{"LiberationSerif-Regular.ttf", "Liberation Serif", 400, false},
}

// embeddedRefs returns the faces of an embedded family in the order of the
// files. key is the lowercase family name. A file is decompressed and
// parsed when its face is used (see faceOf), so a text in one style does
// not load the other styles. The caller holds l.mu.
func (l *Library) embeddedRefs(key string) []*faceRef {
	prefix, ok := embeddedFamilies[key]
	if !ok {
		return nil
	}
	if l.embedded == nil {
		l.embedded = make([]*faceRef, len(embeddedFiles))
		for i, f := range embeddedFiles {
			l.embedded[i] = &faceRef{family: f.family, weight: f.weight, italic: f.italic, path: embeddedPrefix + f.name}
		}
	}
	var refs []*faceRef
	for i, f := range embeddedFiles {
		// A font that did not parse is not a face of the family.
		if ref := l.embedded[i]; strings.HasPrefix(f.name, prefix) && !(ref.tried && ref.face == nil) {
			refs = append(refs, ref)
		}
	}
	return refs
}

// embeddedFamily returns the faces of an embedded family when the library
// has the embedded fonts as fonts (embedFonts). The caller holds l.mu.
func (l *Library) embeddedFamily(key string) []*faceRef {
	if l.embed != embedFonts {
		return nil
	}
	return l.embeddedRefs(key)
}

// standIn returns the embedded face that stands in for a standard family
// that is not installed, or nil. It applies to the platforms that have the
// embedded fonts as the last resort.
func (l *Library) standIn(family string, weight int, italic bool) *Face {
	if l.embed != embedLastResort {
		return nil
	}
	key, ok := embeddedStandIn[strings.ToLower(strings.TrimSpace(family))]
	if !ok {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loadPicked(l.embeddedRefs(key), func(refs []*faceRef) *faceRef {
		return pickRef(refs, weight, italic)
	})
}

// lastResort returns the embedded face for a font-family list that gives no
// font: Liberation Serif, which has the metrics of Times New Roman, the
// standard font of Chromium.
func (l *Library) lastResort(weight int, italic bool) *Face {
	if l.embed == embedNone {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loadPicked(l.embeddedRefs("liberation serif"), func(refs []*faceRef) *faceRef {
		return pickRef(refs, weight, italic)
	})
}

// isAppleColorEmoji reports whether the rules of Apple Color Emoji apply to
// the face: CoreText scales the glyphs of this font (see emoji.go).
func (f *Face) isAppleColorEmoji() bool {
	return f.platform == PlatformDarwin && f.Family == appleColorEmoji
}

// unquoteFamily splits a name of a font-family list. css.SplitFontFamily
// keeps the quotes of a quoted name that is also the keyword of a generic
// family, such as "serif": the second result is true for such a name, and
// the first result is the name without the quotes.
func unquoteFamily(name string) (string, bool) {
	if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
		return name[1 : len(name)-1], true
	}
	return name, false
}

// NewLinuxLibrary returns a library with the rules of Linux that reads
// fonts from the given directories and follows the fontconfig
// configuration in fontconfigDir (the directory that has fonts.conf, such
// as /etc/fonts). With an empty fontconfigDir the library follows the
// default configuration of the reference environment. The library has no
// embedded fonts.
func NewLinuxLibrary(fontconfigDir string, dirs ...string) *Library {
	l := newLibrary(PlatformLinux, embedNone, dirs)
	l.fcDir = fontconfigDir
	return l
}

// linuxEmojiFace returns the font that Chromium on Linux uses for a
// character with emoji presentation: the font that fontconfig gives for
// the emoji U+1F46A in the language und-Zsye
// (FontCache::PlatformFallbackFontForCharacter), or nil.
func (l *Library) linuxEmojiFace() *Face {
	s := l.fontconfig()
	if s == nil {
		return nil
	}
	return s.charFallback(0x1f46a, true)
}
