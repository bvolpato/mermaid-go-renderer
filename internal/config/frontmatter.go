// Ported from packages/mermaid/src/diagram-api/frontmatter.ts (mermaid 12.2.0).

package config

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

var (
	frontMatterYAMLMu    sync.RWMutex
	frontMatterYAMLCache = map[string]any{}
)

const (
	maxCachedFrontMatterYAMLSize    = 4096
	maxCachedFrontMatterYAMLEntries = 256
)

func loadFrontMatterYAML(yamlBody string) (any, error) {
	if len(yamlBody) <= maxCachedFrontMatterYAMLSize {
		frontMatterYAMLMu.RLock()
		cached, ok := frontMatterYAMLCache[yamlBody]
		frontMatterYAMLMu.RUnlock()
		if ok {
			return js.Clone(cached), nil
		}
	}
	loaded, err := loadYAML(yamlBody)
	if err != nil {
		return nil, err
	}
	if len(yamlBody) <= maxCachedFrontMatterYAMLSize {
		frozen := js.FreezeValue(js.Clone(loaded))
		frontMatterYAMLMu.Lock()
		if len(frontMatterYAMLCache) >= maxCachedFrontMatterYAMLEntries {
			clear(frontMatterYAMLCache)
		}
		frontMatterYAMLCache[yamlBody] = frozen
		frontMatterYAMLMu.Unlock()
	}
	return loaded, nil
}

func isASCIIHorizSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\v' || c == '\f'
}

// matchFrontMatterFast matches the common UTF-8 LF frontmatter case directly
// on bytes without converting the entire diagram text to []rune. When handled
// is false, the caller falls back to matchFrontMatter([]rune(text)).
func matchFrontMatterFast(text string) (indent, body string, restOffset int, matched, handled bool) {
	if !utf8.ValidString(text) {
		return "", "", 0, false, false
	}
	n := len(text)
	cursor := 0
	for cursor < n && isASCIIHorizSpace(text[cursor]) {
		cursor++
	}
	if cursor >= n || text[cursor] >= utf8.RuneSelf {
		return "", "", 0, false, false
	}
	if !strings.HasPrefix(text[cursor:], "---") {
		return "", "", 0, false, true
	}
	indent = text[:cursor]
	i := cursor + 3
	for i < n && isASCIIHorizSpace(text[i]) {
		i++
	}
	if i >= n || text[i] != '\n' {
		return "", "", 0, false, false
	}
	bodyStart := i + 1
	// Require exactly one line break in the opening whitespace run so that
	// openingEnd is uniquely determined without scanning to the end of text.
	if bodyStart < n {
		c := text[bodyStart]
		if c >= utf8.RuneSelf || isASCIIHorizSpace(c) || c == '\n' || c == '\r' {
			return "", "", 0, false, false
		}
	}
	// Scan line by line for the first closing fence "\n<indent>---" followed
	// by ASCII whitespace containing a '\n'.
	for pos := bodyStart; pos < n; pos++ {
		c := text[pos]
		if c == '\r' || (c == 0xe2 && pos+2 < n && text[pos+1] == 0x80 && (text[pos+2] == 0xa8 || text[pos+2] == 0xa9)) {
			// CR or Unicode line/paragraph separator: fall back to rune matcher.
			return "", "", 0, false, false
		}
		if c != '\n' {
			continue
		}
		fenceStart := pos + 1
		if !strings.HasPrefix(text[fenceStart:], indent) {
			continue
		}
		afterIndent := fenceStart + len(indent)
		if !strings.HasPrefix(text[afterIndent:], "---") {
			continue
		}
		j := afterIndent + 3
		lastNL := -1
		for j < n {
			cj := text[j]
			if cj == '\n' {
				lastNL = j
				j++
			} else if isASCIIHorizSpace(cj) {
				j++
			} else if cj == '\r' || cj >= utf8.RuneSelf {
				return "", "", 0, false, false
			} else {
				break
			}
		}
		if lastNL >= 0 {
			return indent, text[bodyStart:pos], lastNL + 1, true, true
		}
	}
	return "", "", 0, false, false
}

// frontMatterMetadata is the FrontMatterMetadata interface.
type frontMatterMetadata struct {
	title          string
	hasTitle       bool
	displayMode    string
	hasDisplayMode bool
	// config is undefined (nil) when the front matter has no truthy config.
	config any
}

// toStringMethod is value.toString() for a truthy value.
func toStringMethod(value any, what string) (string, error) {
	if o, ok := value.(*js.Obj[any]); ok && o.Has("toString") {
		// The own property shadows Object.prototype.toString, and YAML cannot
		// produce a function.
		return "", typeError("parsed.%s.toString is not a function", what)
	}
	return js.ToString(value), nil
}

// hasFrontMatterPrefix reports whether text starts with horizontal JS
// whitespace followed by "---", which frontMatterRegex requires.
func hasFrontMatterPrefix(text string) bool {
	i := 0
	for i < len(text) {
		c := text[i]
		if c < utf8.RuneSelf {
			if c == ' ' || c == '\t' || c == '\v' || c == '\f' {
				i++
				continue
			}
			return strings.HasPrefix(text[i:], "---")
		}
		r, sz := utf8.DecodeRuneInString(text[i:])
		if isWhitespace(r) && !isLineBreak(r) {
			i += sz
			continue
		}
		return false
	}
	return false
}

// extractFrontMatter extracts and parses frontmatter from text, if present
// (extractFrontMatter). It returns the text with the frontmatter stripped
// out, and the supported properties of the frontmatter.
func extractFrontMatter(text string) (string, frontMatterMetadata, error) {
	var metadata frontMatterMetadata
	if !hasFrontMatterPrefix(text) {
		// No opening fence: no front matter.
		return text, metadata, nil
	}
	var indent, body, rest string
	if fi, fb, restOff, matched, handled := matchFrontMatterFast(text); handled {
		if !matched {
			return text, metadata, nil
		}
		indent, body, rest = fi, fb, text[restOff:]
	} else {
		runes := []rune(text)
		match, ok := matchFrontMatter(runes)
		if !ok {
			return text, metadata, nil
		}
		indent, body, rest = match.indent, match.body, string(runes[match.length:])
	}

	// Dedent by the captured indent: js-yaml rejects tab-indented documents.
	yamlBody := body
	if indent != "" {
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimPrefix(line, indent)
		}
		yamlBody = strings.Join(lines, "\n")
	}

	// To support config, we need JSON schema.
	// https://www.yaml.org/spec/1.2/spec.html#id2803231
	loaded, err := loadFrontMatterYAML(yamlBody)
	if err != nil {
		return "", metadata, err
	}

	// To handle runtime data type changes
	parsed, _ := loaded.(*js.Obj[any])
	if parsed == nil {
		parsed = js.NewObj[any]()
	}

	// Only add properties that are explicitly supported, if they exist
	if displayMode := parsed.Get("displayMode"); js.Truthy(displayMode) {
		s, err := toStringMethod(displayMode, "displayMode")
		if err != nil {
			return "", metadata, err
		}
		metadata.displayMode, metadata.hasDisplayMode = s, true
	}
	if title := parsed.Get("title"); js.Truthy(title) {
		s, err := toStringMethod(title, "title")
		if err != nil {
			return "", metadata, err
		}
		metadata.title, metadata.hasTitle = s, true
	}
	if config := parsed.Get("config"); js.Truthy(config) {
		metadata.config = config
	}

	return rest, metadata, nil
}
