// Ported from packages/mermaid/src/preprocess.ts (mermaid 12.2.0).

package config

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

var (
	crRe = regexp.MustCompile(`\r\n?`)
	// htmlTagRe is /<(\w+)([^>]*)>/g.
	htmlTagRe = regexp.MustCompile(`<(\w+)([^>]*)>`)
	// attributeRe is /="([^"]*)"/g.
	attributeRe = regexp.MustCompile(`="([^"]*)"`)
)

func cleanupText(code string) string {
	// parser problems on CRLF ignore all CR and leave LF;;
	if strings.IndexByte(code, '\r') >= 0 {
		code = crRe.ReplaceAllString(code, "\n")
	}
	// clean up html tags so that all attributes use single quotes, parser
	// throws error on double quotes
	if strings.IndexByte(code, '<') < 0 {
		// No tag: the replace would copy the text.
		return code
	}
	return htmlTagRe.ReplaceAllStringFunc(code, func(match string) string {
		m := htmlTagRe.FindStringSubmatch(match)
		tag, attributes := m[1], m[2]
		return "<" + tag + attributeRe.ReplaceAllString(attributes, "='${1}'") + ">"
	})
}

// DiagramCode is the DiagramCode interface of diagram-api/types.ts: the
// text of a diagram at the stages of preprocessing.
type DiagramCode struct {
	// Raw is the original source text, untouched.
	Raw string
	// Cleaned is the fully processed text: CRLF normalised, frontmatter
	// removed, directives removed, comments stripped.
	Cleaned string
	// WithComments is the text after frontmatter/directive removal but before
	// comment cleanup.
	WithComments string
	// FrontmatterLineOffset is the number of lines occupied by YAML
	// frontmatter (0 if none).
	FrontmatterLineOffset int
}

// Preprocessed is the result of PreprocessDiagram.
type Preprocessed struct {
	Code DiagramCode
	// Title is the title of the front matter, or "" when there is none.
	Title string
	// DisplayMode is the displayMode of the front matter, or "". Upstream
	// does not return it: it stores it in Config as gantt.displayMode.
	DisplayMode string
	// Config is the config of the front matter merged with the init
	// directive. State.AddDirective sanitizes it in place.
	Config *js.Obj[any]
}

type frontmatterResult struct {
	title       string
	displayMode string
	config      any
	text        string
}

func processFrontmatter(code string) (frontmatterResult, error) {
	text, metadata, err := extractFrontMatter(code)
	if err != nil {
		return frontmatterResult{}, err
	}
	var config any = metadata.config
	if config == nil {
		config = js.NewObj[any]()
	}
	if js.Truthy(metadata.displayMode) {
		// Needs to be supported for legacy reasons
		gantt, _ := js.GetProperty(config, "gantt")
		if !js.Truthy(gantt) {
			gantt = js.NewObj[any]()
			if err := assignProperty(config, "gantt", gantt); err != nil {
				return frontmatterResult{}, err
			}
		}
		if err := assignProperty(gantt, "displayMode", metadata.displayMode); err != nil {
			return frontmatterResult{}, err
		}
	}

	return frontmatterResult{title: metadata.title, displayMode: metadata.displayMode, config: config, text: text}, nil
}

// assignProperty is target[key] = value in strict mode. Creating a property
// on a primitive is a TypeError.
func assignProperty(target any, key string, value any) error {
	switch t := target.(type) {
	case *js.Obj[any]:
		t.Set(key, value)
		return nil
	case []any:
		// A property of an array that is not an index. The value model has no
		// place for it, and nothing upstream reads it back.
		return nil
	}
	return typeError("Cannot create property '%s' on %s '%s'", key, js.TypeOf(target), js.ToString(target))
}

func processDirectives(code string, includeZen bool) (string, any, error) {
	scan := &directiveScan{includeZen: includeZen}
	initDirective, err := scan.detectInit(code)
	if err != nil {
		return "", nil, err
	}
	if js.IsNullish(initDirective) {
		initDirective = js.NewObj[any]()
	}
	wrapDirectives, isArray := scan.detectDirective(code, wrapDirectiveType)
	if isArray {
		some := false
		for _, d := range wrapDirectives {
			if d.hasType && d.typ == "wrap" {
				some = true
				break
			}
		}
		if err := assignProperty(initDirective, "wrap", some); err != nil {
			return "", nil, err
		}
	} else if d := wrapDirectives[0]; d.hasType && d.typ == "wrap" {
		if err := assignProperty(initDirective, "wrap", true); err != nil {
			return "", nil, err
		}
	}
	return scan.removeDirectives(code), initDirective, nil
}

// PreprocessDiagram preprocesses the given code by cleaning it up, extracting
// front matter and directives, cleaning and merging configuration, and
// removing comments (preprocessDiagram).
//
// The error is a *Error when upstream throws: for front matter that is not
// valid YAML, and for a text that has an init directive and no known diagram
// type.
func PreprocessDiagram(code string) (*Preprocessed, error) {
	return preprocessDiagram(code, false)
}

func preprocessDiagram(code string, includeZen bool) (*Preprocessed, error) {
	rawCode := code
	normalizedCode := cleanupText(code)
	frontMatterResult, err := processFrontmatter(normalizedCode)
	if err != nil {
		return nil, err
	}
	text, directive, err := processDirectives(frontMatterResult.text, includeZen)
	if err != nil {
		return nil, err
	}
	config, err := cleanAndMerge(frontMatterResult.config, directive)
	if err != nil {
		return nil, err
	}
	withComments := text
	cleanedCode := CleanupComments(withComments)

	// Compute the number of lines occupied by frontmatter so that parser
	// positions (which are relative to the post-frontmatter text) can be
	// mapped back to the original source shown in the editor.
	frontmatterLineOffset := 0
	if len(normalizedCode) > len(frontMatterResult.text) {
		frontmatterLineOffset = strings.Count(normalizedCode[:len(normalizedCode)-len(frontMatterResult.text)], "\n")
	}

	return &Preprocessed{
		Code: DiagramCode{
			Raw:                   rawCode,
			Cleaned:               cleanedCode,
			WithComments:          withComments,
			FrontmatterLineOffset: frontmatterLineOffset,
		},
		Title:       frontMatterResult.title,
		DisplayMode: frontMatterResult.displayMode,
		Config:      config,
	}, nil
}
