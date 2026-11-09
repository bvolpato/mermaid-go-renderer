// Ported from packages/mermaid/src/diagram-api/frontmatter.ts (mermaid 12.2.0).

package config

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

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

// extractFrontMatter extracts and parses frontmatter from text, if present
// (extractFrontMatter). It returns the text with the frontmatter stripped
// out, and the supported properties of the frontmatter.
func extractFrontMatter(text string) (string, frontMatterMetadata, error) {
	var metadata frontMatterMetadata
	if !strings.Contains(text, "---") {
		// No fence: no front matter.
		return text, metadata, nil
	}
	runes := []rune(text)
	match, ok := matchFrontMatter(runes)
	if !ok {
		return text, metadata, nil
	}

	// Dedent by the captured indent: js-yaml rejects tab-indented documents.
	indent := match.indent
	yamlBody := match.body
	if indent != "" {
		lines := strings.Split(match.body, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimPrefix(line, indent)
		}
		yamlBody = strings.Join(lines, "\n")
	}

	// To support config, we need JSON schema.
	// https://www.yaml.org/spec/1.2/spec.html#id2803231
	loaded, err := loadYAML(yamlBody)
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

	return string(runes[match.length:]), metadata, nil
}
