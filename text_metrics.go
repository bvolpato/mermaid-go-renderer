package mermaid

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

const defaultMetricFontFamily = "'trebuchet ms', verdana, arial, sans-serif"

type indexedFontFile struct {
	Name string
	Path string
}

var (
	fontIndexOnce sync.Once
	fontIndex     []indexedFontFile

	fontFaceCache sync.Map // map[string]*sfnt.Font
)

func measureNativeTextWidth(text string, fontSize float64, fontFamily string) (float64, bool) {
	if text == "" || fontSize <= 0 {
		return 0, true
	}
	path := resolveFontPath(fontFamily)
	if path == "" {
		return 0, false
	}
	face := loadFontFace(path)
	if face == nil {
		return 0, false
	}

	var buf sfnt.Buffer
	ppem := fixed.Int26_6(math.Round(fontSize * 64.0))
	fallbackAdvance := fontSize * 0.56
	width := 0.0
	var prevGlyph sfnt.GlyphIndex

	for _, r := range text {
		if r == '\n' || r == '\r' {
			prevGlyph = 0
			continue
		}
		if r == '\t' {
			width += fallbackAdvance * 2.0
			prevGlyph = 0
			continue
		}
		glyphIdx, err := face.GlyphIndex(&buf, r)
		if err != nil || glyphIdx == 0 {
			width += fallbackAdvance
			prevGlyph = 0
			continue
		}

		if prevGlyph != 0 {
			kern, err := face.Kern(&buf, prevGlyph, glyphIdx, ppem, font.HintingNone)
			if err == nil {
				width += float64(kern) / 64.0
			}
		}

		advance, err := face.GlyphAdvance(&buf, glyphIdx, ppem, font.HintingNone)
		if err != nil {
			width += fallbackAdvance
			prevGlyph = 0
			continue
		}
		charWidth := float64(advance) / 64.0
		charWidth *= browserCharScale(r)
		width += charWidth
		prevGlyph = glyphIdx
	}
	return width, true
}

// browserCharScale returns a per-character scale factor that calibrates native
// sfnt glyph advance (DejaVu Sans) to match the browser reference renderer
// (Chrome/Skia Trebuchet MS). Derived from empirical measurement of mmdc vs
// mmdg foreignObject widths for repeated single-character labels at 16px.
//
// Uppercase letters are nearly matched (avg 1.02x), but lowercase letters
// and digits are systematically ~16% wider in DejaVu Sans than in browser
// Trebuchet MS rendering. Key outliers: 't' (0.71), 'f' (0.74), 'J' (1.70).
func browserCharScale(r rune) float64 {
	switch r {
	case 't':
		return 0.71
	case 'f':
		return 0.74
	case '1':
		return 0.77
	case 'i', 'j', 'l':
		return 0.80
	case 'r', 'v', 'x', 'y':
		return 0.84
	case 'm':
		return 0.85
	case 'k':
		return 0.86
	case '0', '2', '3', '4', '5', '6', '7', '8', '9':
		return 0.87
	case 'b', 'd', 'g', 'h', 'n', 'p', 'q', 'u', 'w':
		return 0.88
	case 'Z':
		return 0.89
	case 'e':
		return 0.90
	case 'a', 'c', 'o':
		return 0.91
	case 'A', 'D', 'I':
		return 0.94
	case 'W', 'z':
		return 0.95
	case 'H', 'M', 's':
		return 0.96
	case 'B', 'N', 'V', 'X':
		return 0.97
	case 'O', 'Q', 'U':
		return 0.99
	case 'G', 'L':
		return 1.00
	case 'K':
		return 1.02
	case 'C', 'T':
		return 1.03
	case 'R':
		return 1.04
	case 'S':
		return 1.05
	case 'E', 'F':
		return 1.06
	case 'Y':
		return 1.09
	case 'P':
		return 1.11
	case 'J':
		return 1.70
	default:
		return 0.93
	}
}

func resolveFontPath(fontFamily string) string {
	fontIndexOnce.Do(buildFontIndex)
	if strings.TrimSpace(fontFamily) == "" {
		fontFamily = defaultMetricFontFamily
	}
	candidates := parseFontFamilyCandidates(fontFamily)
	if len(candidates) == 0 {
		candidates = parseFontFamilyCandidates(defaultMetricFontFamily)
	}

	// Exact match first.
	for _, candidate := range candidates {
		for _, file := range fontIndex {
			if file.Name == candidate {
				return file.Path
			}
		}
	}
	// Then fuzzy contains match.
	for _, candidate := range candidates {
		bestPath := ""
		bestLen := int(^uint(0) >> 1)
		for _, file := range fontIndex {
			if strings.Contains(file.Name, candidate) || strings.Contains(candidate, file.Name) {
				if l := len(file.Name); l < bestLen {
					bestLen = l
					bestPath = file.Path
				}
			}
		}
		if bestPath != "" {
			return bestPath
		}
	}

	return ""
}

func buildFontIndex() {
	dirs := []string{
		"/System/Library/Fonts/Supplemental",
		"/System/Library/Fonts",
		"/Library/Fonts",
		"/usr/share/fonts",
		"/usr/local/share/fonts",
		"~/.fonts",
		"~/.local/share/fonts",
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
		dirs = append(dirs, filepath.Join(home, ".fonts"))
		dirs = append(dirs, filepath.Join(home, ".local", "share", "fonts"))
	}

	index := make([]indexedFontFile, 0, 256)
	for _, dir := range dirs {
		if strings.HasPrefix(dir, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			dir = filepath.Join(home, dir[2:])
		}

		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}

			name := d.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if ext != ".ttf" && ext != ".otf" && ext != ".ttc" {
				return nil
			}
			base := strings.TrimSuffix(name, filepath.Ext(name))
			norm := normalizeFontToken(base)
			if norm == "" {
				return nil
			}
			index = append(index, indexedFontFile{
				Name: norm,
				Path: path,
			})
			return nil
		})
	}
	sort.Slice(index, func(i, j int) bool {
		if index[i].Name == index[j].Name {
			return index[i].Path < index[j].Path
		}
		return len(index[i].Name) < len(index[j].Name)
	})
	fontIndex = index
}

func parseFontFamilyCandidates(fontFamily string) []string {
	parts := strings.Split(fontFamily, ",")
	out := make([]string, 0, len(parts)+2)
	for _, part := range parts {
		token := strings.TrimSpace(part)
		token = strings.Trim(token, "\"'")
		if token == "" {
			continue
		}
		norm := normalizeFontToken(token)
		if norm == "" {
			continue
		}
		switch norm {
		case "sansserif", "uisansserif", "systemui", "applesystem", "sans":
			out = append(out, "arial", "helvetica", "dejavusans", "liberationsans", "notosans", "freesans", "ubuntu")
		case "serif":
			out = append(out, "timesnewroman", "times", "dejavuserif", "liberationserif", "freeserif")
		case "monospace", "uimonospace", "mono":
			out = append(out, "couriernew", "menlo", "monaco", "dejavusansmono", "liberationmono", "freemono", "ubuntu mono")
		default:
			out = append(out, norm)
		}
	}
	return dedupeStrings(out)
}

func normalizeFontToken(value string) string {
	lower := strings.ToLower(value)
	var b strings.Builder
	b.Grow(len(lower))
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func loadFontFace(path string) *sfnt.Font {
	if cached, ok := fontFaceCache.Load(path); ok {
		if face, ok := cached.(*sfnt.Font); ok {
			return face
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	if strings.EqualFold(filepath.Ext(path), ".ttc") {
		collection, err := sfnt.ParseCollection(data)
		if err == nil {
			for i := 0; i < collection.NumFonts(); i++ {
				face, err := collection.Font(i)
				if err == nil {
					fontFaceCache.Store(path, face)
					return face
				}
			}
		}
	}

	face, err := sfnt.Parse(data)
	if err != nil {
		return nil
	}
	fontFaceCache.Store(path, face)
	return face
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
