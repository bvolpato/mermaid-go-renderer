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
	for _, r := range text {
		if r == '\n' || r == '\r' {
			continue
		}
		if r == '\t' {
			width += fallbackAdvance * 2.0
			continue
		}
		glyphIdx, err := face.GlyphIndex(&buf, r)
		if err != nil || glyphIdx == 0 {
			width += fallbackAdvance
			continue
		}
		advance, err := face.GlyphAdvance(&buf, glyphIdx, ppem, font.HintingNone)
		if err != nil {
			width += fallbackAdvance
			continue
		}
		width += float64(advance) / 64.0
	}
	return width, true
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
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
	}

	index := make([]indexedFontFile, 0, 256)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if ext != ".ttf" && ext != ".otf" && ext != ".ttc" {
				continue
			}
			base := strings.TrimSuffix(name, filepath.Ext(name))
			norm := normalizeFontToken(base)
			if norm == "" {
				continue
			}
			index = append(index, indexedFontFile{
				Name: norm,
				Path: filepath.Join(dir, name),
			})
		}
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
		case "sansserif", "uisansserif", "systemui", "applesystem":
			out = append(out, "arial", "helvetica")
		case "serif":
			out = append(out, "timesnewroman", "times")
		case "monospace", "uimonospace":
			out = append(out, "couriernew", "menlo", "monaco")
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
