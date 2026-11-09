// This file ports no upstream file. It has the font library: it finds font
// faces by CSS family name, with the choices of Chromium on each platform.

package font

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
	gtfont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// Library finds font faces by CSS family name, as a browser does on the same
// machine. It reads the system font directories. Lookups for common fonts
// touch only the files whose names start with the family name. A full scan
// of the directories happens only when that fails, and its result is cached
// on disk.
type Library struct {
	platform Platform
	embed    embedMode
	dirs     []string

	// The fontconfig of a Linux library. See fontconfig_match.go.
	fcDir          string // directory of the configuration, or ""
	fcSystemConfig bool   // read the configuration of the machine
	fcOnce         sync.Once
	fc             *fcSystem

	mu       sync.Mutex
	files    []string              // every font file path, listed once
	scanned  map[string][]*faceRef // path -> the faces of the file
	families map[string][]*faceRef // lowercase family -> faces found so far
	missing  map[string]bool       // families known to be absent
	index    []indexEntry          // full index, built lazily
	indexed  bool
	chains   map[string]*Chain
	fallback map[fallbackKey]*Face
	// embedded has the faces of the embedded font files. See embeddedRefs.
	embedded []*faceRef
	// fallbackFaces has the faces that are loaded as the fallback font of a
	// character, by file and index. See fcSystem.fallbackFace.
	fallbackFaces map[string]*Face
	// parses counts the fonts that the library parsed. See ParsedFonts.
	parses int
}

// ParsedFonts returns the number of fonts that the library has parsed. A
// font is parsed one time, when a text first uses its face: a later lookup
// of the family, in any goroutine, gets the same face.
func (l *Library) ParsedFonts() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.parses
}

// faceRef is one face of a font file: the family and the style, as a scan
// of the file gives them. The font is parsed when the face is first used.
// So a lookup of a family does not parse the styles that no text has, and
// it does not parse the other files whose names start with the family name.
type faceRef struct {
	family string // the family name of the font
	weight int    // 100 to 900
	italic bool
	path   string // the file, or embeddedPrefix and the name of an embedded file
	index  int    // index in a font collection

	// face is the parsed face. tried is true when the font was parsed: a
	// font that does not parse has no face. Library.mu guards both.
	face  *Face
	tried bool
}

type indexEntry struct {
	Path   string `json:"p"`
	Index  int    `json:"i"`
	Family string `json:"f"`
	Weight int    `json:"w"`
	Italic bool   `json:"s,omitempty"`
	// Version and NoLatin are for the sort of fontconfig on Linux. See
	// fcIndexInfo.
	Version int32 `json:"v,omitempty"`
	NoLatin bool  `json:"n,omitempty"`
}

type fallbackKey struct {
	r      rune
	bold   bool
	italic bool
}

var (
	systemOnce sync.Once
	systemLib  *Library
)

// System returns the library of the fonts installed on this machine, with
// the rules of the platform of this machine. The environment variable
// MMDG_FONT_PATH adds directories (separated like PATH). The embedded
// Liberation fonts stand in for a standard family that is not installed.
func System() *Library {
	systemOnce.Do(func() {
		platform := DefaultPlatform()
		systemLib = newLibrary(platform, defaultEmbedMode(platform), systemFontDirs())
		systemLib.fcSystemConfig = platform == PlatformLinux
	})
	return systemLib
}

// NewLibrary returns a library that reads fonts from the given directories
// and applies the rules of the platform of this machine.
func NewLibrary(dirs ...string) *Library {
	return NewLibraryFor(DefaultPlatform(), dirs...)
}

func newLibrary(platform Platform, embed embedMode, dirs []string) *Library {
	return &Library{
		platform:      platform,
		embed:         embed,
		dirs:          dirs,
		scanned:       map[string][]*faceRef{},
		families:      map[string][]*faceRef{},
		missing:       map[string]bool{},
		chains:        map[string]*Chain{},
		fallback:      map[fallbackKey]*Face{},
		fallbackFaces: map[string]*Face{},
	}
}

func systemFontDirs() []string {
	var dirs []string
	if extra := os.Getenv("MMDG_FONT_PATH"); extra != "" {
		dirs = append(dirs, filepath.SplitList(extra)...)
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		dirs = append(dirs,
			"/System/Library/Fonts",
			"/System/Library/Fonts/Supplemental",
			"/Library/Fonts",
			filepath.Join(home, "Library/Fonts"),
			// PingFang, the CJK fallback of the system, is stored here.
			"/System/Library/PrivateFrameworks/FontServices.framework/Versions/A/Resources/Reserved",
		)
	case "windows":
		windir := os.Getenv("WINDIR")
		if windir == "" {
			windir = `C:\Windows`
		}
		dirs = append(dirs, filepath.Join(windir, "Fonts"), filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Windows\Fonts`))
	default:
		dirs = append(dirs,
			"/usr/share/fonts",
			"/usr/local/share/fonts",
			filepath.Join(home, ".fonts"),
			filepath.Join(home, ".local/share/fonts"),
		)
	}
	return dirs
}

func isFontFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	}
	return false
}

// listFiles lists every font file once. The caller holds l.mu.
func (l *Library) listFiles() []string {
	if l.files != nil {
		return l.files
	}
	l.files = []string{}
	for _, dir := range l.dirs {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() && isFontFile(path) {
				l.files = append(l.files, path)
			}
			return nil
		})
	}
	return l.files
}

func normalizeName(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if r != ' ' && r != '-' && r != '_' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// scanFile lists the faces of a file and registers them under their family
// names. It reads the description of each face and does not parse the
// fonts. The caller holds l.mu.
func (l *Library) scanFile(path string) []*faceRef {
	if refs, ok := l.scanned[path]; ok {
		return refs
	}
	refs, parses := scanFileFor(l.platform, path)
	l.parses += parses
	l.scanned[path] = refs
	for _, ref := range refs {
		key := strings.ToLower(ref.family)
		l.families[key] = append(l.families[key], ref)
	}
	return refs
}

// scanFileFor reads the family and the style of each face of a font file.
//
// The description is the one that a parsed font reports, so that a face is
// chosen as if every font were parsed. The two differ for a font that has no
// usable OS/2 table: go-text then describes a parsed font in another way
// than a font that it only scans. Such a face is parsed here, and a face
// that does not parse is left out.
//
// The second result is the number of fonts that it parsed.
func scanFileFor(platform Platform, path string) (refs []*faceRef, parses int) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0
	}
	defer file.Close()
	loaders, err := ot.NewLoaders(file)
	if err != nil {
		return nil, 0
	}
	var data []byte
	for i, ld := range loaders {
		ref := &faceRef{path: path, index: i}
		if scanIsExact(ld) {
			desc, _ := gtfont.Describe(ld, nil)
			ref.family = desc.Family
			ref.weight = int(desc.Aspect.Weight)
			if ref.weight == 0 {
				ref.weight = 400
			}
			ref.italic = desc.Aspect.Style != gtfont.StyleNormal
			refs = append(refs, ref)
			continue
		}
		if data == nil {
			if data, err = os.ReadFile(path); err != nil {
				return refs, parses
			}
		}
		parses++
		face, err := loadFaceAt(platform, data, i)
		if err != nil {
			continue
		}
		face.Path = path
		ref.family, ref.weight, ref.italic = face.Family, face.Weight, face.Italic
		ref.face, ref.tried = face, true
		refs = append(refs, ref)
	}
	return refs, parses
}

// scanIsExact reports whether the description that go-text reads from the
// tables of a font is the description that the parsed font reports. Both
// read the OS/2 table. A parsed font drops an OS/2 table of version 2 or
// later that is too short, and it keeps the start of a table that the scan
// rejects.
func scanIsExact(ld *ot.Loader) bool {
	raw, err := ld.RawTable(ot.MustNewTag("OS/2"))
	if err != nil {
		return false
	}
	os2, _, err := tables.ParseOs2(raw)
	if err != nil {
		return false
	}
	return os2.Version < 2 || len(os2.HigherVersionData) >= 12
}

// faceOf returns the parsed face of a reference. It parses the font on the
// first call. The result is nil for a font that does not parse: the
// reference then leaves its family, as if the scan had not found it. The
// caller holds l.mu.
func (l *Library) faceOf(ref *faceRef) *Face {
	if ref == nil {
		return nil
	}
	if ref.tried {
		return ref.face
	}
	ref.tried = true
	var data []byte
	var err error
	if name, ok := strings.CutPrefix(ref.path, embeddedPrefix); ok {
		data, err = liberation.Font(name)
	} else {
		data, err = os.ReadFile(ref.path)
	}
	if err == nil {
		l.parses++
		if face, err := loadFaceAt(l.platform, data, ref.index); err == nil {
			face.Path = ref.path
			ref.face = face
		}
	}
	if ref.face == nil {
		l.dropRef(ref)
	}
	return ref.face
}

// dropRef removes a reference whose font does not parse from its family.
// The caller holds l.mu.
func (l *Library) dropRef(ref *faceRef) {
	key := strings.ToLower(ref.family)
	refs := l.families[key]
	for i, r := range refs {
		if r == ref {
			// A new slice: a caller can hold the old one.
			l.families[key] = append(append([]*faceRef(nil), refs[:i]...), refs[i+1:]...)
			return
		}
	}
}

// loadPicked parses and returns the face that pick chooses from the
// references. When the font of the choice does not parse, it chooses again
// from the other references. The caller holds l.mu.
func (l *Library) loadPicked(refs []*faceRef, pick func([]*faceRef) *faceRef) *Face {
	for len(refs) > 0 {
		ref := pick(refs)
		if face := l.faceOf(ref); face != nil {
			return face
		}
		rest := make([]*faceRef, 0, len(refs)-1)
		for _, r := range refs {
			if r != ref {
				rest = append(rest, r)
			}
		}
		refs = rest
	}
	return nil
}

// loadFamily parses and returns the face that pick chooses from the faces
// of a family, or nil for a family that has no usable face. The caller
// holds l.mu.
func (l *Library) loadFamily(family string, pick func([]*faceRef) *faceRef) *Face {
	for {
		refs := l.refsOf(family)
		if len(refs) == 0 {
			return nil
		}
		if face := l.loadPicked(refs, pick); face != nil {
			return face
		}
		// No font of these references parses. They left the family, so the
		// next lookup searches the other files.
	}
}

// refsOf returns the faces of a family, scanning files as needed. The
// caller holds l.mu.
func (l *Library) refsOf(family string) []*faceRef {
	key := strings.ToLower(strings.TrimSpace(family))
	if key == "" {
		return nil
	}
	if l.missing[key] {
		return nil
	}
	if refs := l.families[key]; len(refs) > 0 && l.familyComplete(key) {
		return refs
	}
	// Fast path: files whose name starts with the family name.
	norm := normalizeName(key)
	for _, path := range l.listFiles() {
		base := normalizeName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		if strings.HasPrefix(base, norm) {
			l.scanFile(path)
		}
	}
	if refs := l.families[key]; len(refs) > 0 {
		l.markComplete(key)
		return refs
	}
	// Slow path: the full index.
	l.buildIndex()
	for _, e := range l.index {
		if strings.EqualFold(e.Family, key) {
			l.scanFile(e.Path)
		}
	}
	if refs := l.families[key]; len(refs) > 0 {
		l.markComplete(key)
		return refs
	}
	// The embedded fonts come after the fonts of the directories.
	if refs := l.embeddedFamily(key); len(refs) > 0 {
		l.families[key] = refs
		l.markComplete(key)
		return refs
	}
	l.missing[key] = true
	return nil
}

// A family is complete when every file that can hold it was loaded. Faces of
// a family can be registered as a side effect of loading another family
// ("Arial" loads "Arial Black"), so a separate marker is needed.
func (l *Library) familyComplete(key string) bool { return l.missing["\x00"+key] }
func (l *Library) markComplete(key string)        { l.missing["\x00"+key] = true }

// buildIndex scans every font file, or reads the scan from the disk cache.
// The caller holds l.mu.
func (l *Library) buildIndex() {
	if l.indexed {
		return
	}
	l.indexed = true
	files := l.listFiles()
	if len(files) == 0 {
		// A library without font files does not touch the disk.
		return
	}
	cachePath := ""
	if dir, err := os.UserCacheDir(); err == nil {
		cachePath = filepath.Join(dir, "mmdg", "font-index-v2.json")
	}
	stamp := indexStamp(files)
	if l.platform == PlatformLinux {
		// The index of a Linux library has more fields.
		stamp += "-linux"
	}
	if cachePath != "" {
		if data, err := os.ReadFile(cachePath); err == nil {
			var cached struct {
				Stamp   string       `json:"stamp"`
				Entries []indexEntry `json:"entries"`
			}
			if json.Unmarshal(data, &cached) == nil && cached.Stamp == stamp {
				l.index = cached.Entries
				return
			}
		}
	}
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		loaders, err := ot.NewLoaders(file)
		if err == nil {
			for i, ld := range loaders {
				desc, _ := gtfont.Describe(ld, nil)
				if desc.Family == "" {
					continue
				}
				w := int(desc.Aspect.Weight)
				if w == 0 {
					w = 400
				}
				entry := indexEntry{Path: path, Index: i, Family: desc.Family, Weight: w, Italic: desc.Aspect.Style != gtfont.StyleNormal}
				if l.platform == PlatformLinux {
					version, latin := fcIndexInfo(ld)
					entry.Version, entry.NoLatin = version, !latin
				}
				l.index = append(l.index, entry)
			}
		}
		file.Close()
	}
	if cachePath != "" {
		if data, err := json.Marshal(map[string]any{"stamp": stamp, "entries": l.index}); err == nil {
			_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
			_ = os.WriteFile(cachePath, data, 0o644)
		}
	}
}

func indexStamp(files []string) string {
	var sb strings.Builder
	sorted := append([]string(nil), files...)
	sort.Strings(sorted)
	for _, f := range sorted {
		sb.WriteString(f)
		if st, err := os.Stat(f); err == nil {
			sb.WriteString(st.ModTime().UTC().Format("|20060102150405|"))
		}
	}
	// A short stable digest (FNV-1a) keeps the cache file small.
	var h uint64 = 14695981039346656037
	for i := 0; i < sb.Len(); i++ {
		h ^= uint64(sb.String()[i])
		h *= 1099511628211
	}
	return strings.ToUpper(strings.TrimLeft(string(appendHex(nil, h)), "0"))
}

func appendHex(b []byte, v uint64) []byte {
	const digits = "0123456789abcdef"
	for shift := 60; shift >= 0; shift -= 4 {
		b = append(b, digits[(v>>uint(shift))&15])
	}
	return b
}

// genericFamily maps a CSS generic family to the default font of Chromium on
// the platform of the library. The rules of Linux are in linux_match.go.
func (l *Library) genericFamily(name string) []string {
	switch l.platform {
	case PlatformDarwin:
		switch name {
		case "serif", "ui-serif":
			return []string{"Times"}
		case "sans-serif", "ui-sans-serif", "system-ui":
			return []string{"Helvetica"}
		case "monospace", "ui-monospace":
			return []string{"Courier"}
		case "cursive":
			return []string{"Apple Chancery"}
		case "fantasy":
			return []string{"Papyrus"}
		}
	case PlatformWindows:
		switch name {
		case "serif", "ui-serif":
			return []string{"Times New Roman"}
		case "sans-serif", "ui-sans-serif":
			return []string{"Arial"}
		case "system-ui":
			return []string{"Segoe UI", "Arial"}
		case "monospace", "ui-monospace":
			return []string{"Consolas", "Courier New"}
		case "cursive":
			return []string{"Comic Sans MS"}
		case "fantasy":
			return []string{"Impact"}
		}
	}
	return nil
}

func isGenericFamily(name string) bool {
	switch name {
	case "serif", "sans-serif", "monospace", "cursive", "fantasy", "system-ui",
		"ui-serif", "ui-sans-serif", "ui-monospace", "ui-rounded", "math", "emoji", "fangsong":
		return true
	}
	return false
}

// Match returns the face of a family that is closest to the weight and
// slant, by the CSS font matching rules. It returns nil when the family is
// not installed. On Linux it follows fontconfig as Chromium uses it: a
// family that is not installed gives the font that is metric-compatible
// with it (see linuxFamily).
func (l *Library) Match(family string, weight int, italic bool) *Face {
	if l.platform == PlatformLinux {
		return l.linuxFamily(family, weight, italic)
	}
	l.mu.Lock()
	face := l.loadFamily(family, func(refs []*faceRef) *faceRef {
		return pickRef(refs, weight, italic)
	})
	l.mu.Unlock()
	if face == nil {
		return l.standIn(family, weight, italic)
	}
	return face
}

// pickRef returns the face of a family that is closest to a weight and a
// slant, by the CSS font matching rules. refs is not empty.
func pickRef(refs []*faceRef, weight int, italic bool) *faceRef {
	// Slant first: prefer a face with the wanted slant.
	pool := refs[:0:0]
	for _, r := range refs {
		if r.italic == italic {
			pool = append(pool, r)
		}
	}
	if len(pool) == 0 {
		pool = refs
	}
	best := pool[0]
	bestScore := weightDistance(weight, best.weight)
	for _, r := range pool[1:] {
		if s := weightDistance(weight, r.weight); s < bestScore {
			best, bestScore = r, s
		}
	}
	return best
}

// weightDistance orders candidate weights by the CSS font matching
// algorithm. A lower result is a better match.
func weightDistance(want, have int) int {
	if want == have {
		return 0
	}
	switch {
	case want >= 400 && want <= 500:
		// Try up to 500 first, then lighter, then heavier.
		if have >= want && have <= 500 {
			return have - want
		}
		if have < want {
			return 1000 + (want - have)
		}
		return 2000 + (have - want)
	case want < 400:
		if have < want {
			return want - have
		}
		return 1000 + (have - want)
	default:
		if have > want {
			return have - want
		}
		return 1000 + (want - have)
	}
}

// Chain is the ordered list of faces for one CSS font-family value at one
// weight and slant. A character uses the first face that has a glyph for it.
type Chain struct {
	lib    *Library
	faces  []*Face
	weight int
	italic bool
	// first is the first family name of the font-family list. On Linux
	// the fallback for a bold or italic text tries its regular face.
	first string
}

// Resolve builds the fallback chain for a font-family list.
func (l *Library) Resolve(families []string, weight int, italic bool) *Chain {
	// The weight is in the key as it is: on Linux the weights 549 and 550
	// give different faces.
	key := strings.Join(families, "\x00") + "\x00" + strconv.Itoa(weight)
	if italic {
		key += "i"
	}
	l.mu.Lock()
	if c, ok := l.chains[key]; ok {
		l.mu.Unlock()
		return c
	}
	l.mu.Unlock()

	c := &Chain{lib: l, weight: weight, italic: italic}
	if l.platform == PlatformLinux {
		if len(families) > 0 {
			c.first, _ = unquoteFamily(families[0])
		}
		c.faces = l.linuxChain(families, weight, italic)
		l.mu.Lock()
		l.chains[key] = c
		l.mu.Unlock()
		return c
	}
	seen := map[*Face]bool{}
	add := func(f *Face) {
		if f != nil && !seen[f] {
			seen[f] = true
			c.faces = append(c.faces, f)
		}
	}
	for _, fam := range families {
		fam, _ = unquoteFamily(fam)
		lower := strings.ToLower(fam)
		if isGenericFamily(lower) {
			for _, g := range l.genericFamily(lower) {
				if f := l.Match(g, weight, italic); f != nil {
					add(f)
					break
				}
			}
			continue
		}
		add(l.Match(fam, weight, italic))
	}
	// The last resort is the default standard font of the browser.
	if len(c.faces) == 0 {
		for _, g := range l.genericFamily("serif") {
			if f := l.Match(g, weight, italic); f != nil {
				add(f)
				break
			}
		}
	}
	if len(c.faces) == 0 {
		for _, g := range l.genericFamily("sans-serif") {
			if f := l.Match(g, weight, italic); f != nil {
				add(f)
				break
			}
		}
	}
	if len(c.faces) == 0 {
		// No font of the system: the embedded font is the last resort.
		add(l.lastResort(weight, italic))
	}
	l.mu.Lock()
	l.chains[key] = c
	l.mu.Unlock()
	return c
}

// Primary is the first face of the chain. Line metrics come from it.
func (c *Chain) Primary() *Face {
	if len(c.faces) == 0 {
		return nil
	}
	return c.faces[0]
}

// FaceFor returns the face that renders a character.
func (c *Chain) FaceFor(r rune) *Face {
	if c.lib.platform == PlatformLinux {
		return c.linuxFaceFor(r)
	}
	for _, f := range c.faces {
		if f.HasGlyph(r) {
			return f
		}
	}
	if f := c.lib.systemFallback(r, c.weight >= 600, c.italic); f != nil {
		return f
	}
	return c.Primary()
}

// fallbackFamilies is the order in which system fonts are tried for a
// character that no listed family has. The order approximates the font
// cascade of CoreText that Chromium uses on macOS.
func (l *Library) fallbackFamilies() []string {
	switch l.platform {
	case PlatformDarwin:
		return []string{
			"Helvetica", "Helvetica Neue", "Lucida Grande", "PingFang SC", "Hiragino Sans", "Hiragino Sans GB",
			"Apple SD Gothic Neo", "Apple Color Emoji", "Apple Symbols", "Menlo", "Zapf Dingbats",
			"STIX Two Math", "Arial Unicode MS", "Geneva", "Times",
		}
	case PlatformWindows:
		return []string{"Segoe UI", "Arial", "Segoe UI Symbol", "Segoe UI Emoji", "Microsoft YaHei", "Yu Gothic", "Malgun Gothic", "Arial Unicode MS"}
	}
	// The fallback of Linux is in linux_match.go.
	return nil
}

func (l *Library) systemFallback(r rune, bold, italic bool) *Face {
	key := fallbackKey{r, bold, italic}
	l.mu.Lock()
	if f, ok := l.fallback[key]; ok {
		l.mu.Unlock()
		return f
	}
	l.mu.Unlock()
	weight := 400
	if bold {
		weight = 700
	}
	var found *Face
	// This is the fallback for a character with text presentation: a
	// character with emoji presentation gets the emoji font before (see
	// emojiFaceFor). CoreText knows the presentation of a character and
	// gives the emoji font to a character with text presentation only when
	// no other font has it. It gives a character of the block Dingbats to
	// Zapf Dingbats before the fonts that cover many scripts.
	// internal/browser/testdata/emoji-probe.json.gz has the values of
	// Chromium, and the test that reads it lists the characters for which
	// this order is not the choice of CoreText.
	families := l.fallbackFamilies()
	if isDingbat(r) {
		families = append([]string{"Zapf Dingbats"}, families...)
	}
	for _, fam := range families {
		if fam == appleColorEmoji {
			continue
		}
		if f := l.Match(fam, weight, italic); f != nil && f.HasGlyph(r) {
			found = f
			break
		}
	}
	if found == nil {
		if f := l.Match(appleColorEmoji, weight, italic); f != nil && f.Family == appleColorEmoji && f.HasGlyph(r) {
			found = f
		}
	}
	l.mu.Lock()
	l.fallback[key] = found
	l.mu.Unlock()
	return found
}
