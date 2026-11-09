// Ported from packages/mermaid/src/diagram-api/detectType.ts,
// diagram-api/diagram-orchestration.ts and the detector of every diagram that
// addDiagrams registers (mermaid 12.2.0).

package config

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// detector is one entry of the `detectors` record: the diagram id and the
// function that recognizes the diagram text.
type detector struct {
	id     string
	detect func(text string) bool
	// pattern is the regular expression of a detector that tests the start
	// of the text before it asks the expression. See starts.
	pattern func(text string) bool
}

// starts returns the detector for a pattern of the form ^\s*word...: the
// pattern can match only a text that starts, after white space, with one of
// the words. The detector tests that first, without regard to the case of
// letters, and asks the pattern only then. Most detectors fail on the first
// test, which does not start the regular expression.
func starts(id string, pattern func(string) bool, words ...string) detector {
	return detector{id: id, pattern: pattern, detect: func(text string) bool {
		rest := js.TrimStart(text)
		for _, word := range words {
			if len(rest) >= len(word) && strings.EqualFold(rest[:len(word)], word) {
				return pattern(text)
			}
		}
		return false
	}}
}

// re compiles the pattern of a detector. `\s` becomes the JavaScript class.
// Detectors with the i flag pass their literal through ci instead of using
// (?i), which folds more characters than JavaScript does.
func re(pattern string) func(string) bool {
	compiled := regexp.MustCompile(strings.ReplaceAll(pattern, `\s`, "["+jsSpaceChars+"]"))
	return compiled.MatchString
}

// ci spells out both cases of every ASCII letter of a literal.
func ci(literal string) string {
	var sb strings.Builder
	for _, r := range literal {
		switch {
		case r >= 'a' && r <= 'z':
			sb.WriteString("[" + string(r) + string(r-'a'+'A') + "]")
		case r >= 'A' && r <= 'Z':
			sb.WriteString("[" + string(r-'A'+'a') + string(r) + "]")
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// detectors is the `detectors` record after addDiagrams, in registration
// order. Ordering of detectors is important. The first one to return true
// will be used.
//
// The list is the one of the full mermaid build (injected.includeLargeFeatures
// is true), which registers flowchart-elk, mindmap and architecture before
// the rest.
var detectors = []detector{
	// text.toLowerCase().trim() === 'error'. No letter of another script
	// becomes e, r or o in lower case, so the trimmed text is compared as
	// it is, without a copy in lower case.
	{id: "error", detect: func(text string) bool {
		trimmed := js.TrimSpace(text)
		return len(trimmed) == len("error") && strings.ToLower(trimmed) == "error"
	}},
	// --- diagram type may appear if YAML front-matter is not parsed correctly
	// text.toLowerCase().trimStart().startsWith('---'): the case does not
	// change a '-' or white space.
	{id: "---", detect: func(text string) bool {
		return strings.HasPrefix(js.TrimStart(text), "---")
	}},
	// Only the explicit `flowchart-elk` keyword. A flowchart that wants ELK
	// asks for it with `layout: elk`, which needs no detector of its own.
	// Upstream also sets config.layout = 'elk' on the config that detectType
	// receives; see DetectType.
	starts("flowchart-elk", re(`^\s*flowchart-elk`), "flowchart-elk"),
	starts("mindmap", re(`^\s*mindmap`), "mindmap"),
	starts("architecture", re(`^\s*architecture`), "architecture"),
	starts("agentflow", re(`^\s*agentflow-beta\b`), "agentflow-beta"),
	{id: "c4", detect: re(`^\s*C4Context|C4Container|C4Component|C4Dynamic|C4Deployment`)},
	starts("kanban", re(`^\s*kanban`), "kanban"),
	// Both `classDiagram` and `classDiagram-v2` render with the unified class diagram.
	starts("classDiagram", re(`^\s*classDiagram`), "classDiagram"),
	starts("er", re(`^\s*erDiagram`), "erDiagram"),
	starts("gantt", re(`^\s*gantt`), "gantt"),
	starts("info", re(`^\s*info`), "info"),
	starts("pie", re(`^\s*pie`), "pie"),
	starts("requirement", re(`^\s*requirement(Diagram)?`), "requirement"),
	starts("sequence", re(`^\s*sequenceDiagram`), "sequenceDiagram"),
	starts("swimlane", re(`^\s*swimlane-beta\b`), "swimlane-beta"),
	// `graph` and `flowchart` both render with the unified flowchart. Which
	// layout runs is decided by the `layout` option, not by the diagram id.
	starts("flowchart-v2", re(`^\s*(graph|flowchart)`), "graph", "flowchart"),
	starts("timeline", re(`^\s*timeline`), "timeline"),
	starts("gitGraph", re(`^\s*gitGraph`), "gitGraph"),
	// Both `stateDiagram` and `stateDiagram-v2` render with the unified state diagram.
	starts("stateDiagram", re(`^\s*stateDiagram`), "stateDiagram"),
	starts("journey", re(`^\s*journey`), "journey"),
	starts("quadrantChart", re(`^\s*quadrantChart`), "quadrantChart"),
	starts("sankey", re(`^\s*sankey(-beta)?`), "sankey"),
	starts("packet", re(`^\s*packet(-beta)?`), "packet"),
	starts("xychart", re(`^\s*xychart(-beta)?`), "xychart"),
	starts("block", re(`^\s*block(-beta)?`), "block"),
	starts("eventmodeling", re(`^\s*eventmodeling`), "eventmodeling"),
	starts("treeView", re(`^\s*treeView-beta`), "treeView-beta"),
	starts("radar", re(`^\s*radar-beta`), "radar-beta"),
	starts("ishikawa", re(`^\s*`+ci("ishikawa")+`(`+ci("-beta")+`)?\b`), "ishikawa"),
	starts("treemap", re(`^\s*treemap`), "treemap"),
	starts("railroad", re(`^\s*`+ci("railroad-beta")), "railroad-beta"),
	starts("railroadEbnf", re(`^\s*`+ci("railroad-ebnf-beta")), "railroad-ebnf-beta"),
	starts("railroadAbnf", re(`^\s*`+ci("railroad-abnf-beta")), "railroad-abnf-beta"),
	starts("railroadPeg", re(`^\s*`+ci("railroad-peg-beta")), "railroad-peg-beta"),
	starts("venn", re(`^\s*venn-beta`), "venn-beta"),
	starts("wardley", re(`^\s*`+ci("wardley-beta")), "wardley-beta"),
	// /^\s*cynefin-beta(?:[\s:]|$)/
	starts("cynefin", re(`^\s*cynefin-beta(?:\s|:|$)`), "cynefin-beta"),
	starts("usecase", re(`^\s*usecase-beta(?:\s|$)`), "usecase-beta"),
}

// DetectorIDs returns the ids of the registered diagram detectors in
// registration order (Object.keys(detectors)).
func DetectorIDs() []string {
	ids := make([]string, len(detectors))
	for i, d := range detectors {
		ids[i] = d.id
	}
	return ids
}

// DetectType detects the type of the graph text (detectType). It takes into
// consideration the possible existence of front matter, of an `%%init`
// directive and of comments. The result is a graph definition key, for
// example "flowchart-v2".
//
// config is the mermaid config that upstream hands to the detectors. It can
// be nil. Only the flowchart-elk detector uses it: it sets config.layout to
// "elk", as upstream does, so that the flowchart init function can pass the
// layout on.
//
// The error is an UnknownDiagramError when no detector matches.
func DetectType(text string, config *js.Obj[any]) (string, error) {
	// A text without a fence has no front matter.
	if strings.Contains(text, "---") {
		runes := []rune(text)
		if frontMatter, ok := matchFrontMatter(runes); ok {
			text = string(runes[frontMatter.length:])
		}
	}
	text = stripAnyComments(replaceAll(directiveRegex, directiveMarker, text))
	for _, d := range detectors {
		if d.detect(text) {
			if d.id == "flowchart-elk" && config != nil {
				config.Set("layout", "elk")
			}
			return d.id, nil
		}
	}

	return "", unknownDiagramError("No diagram type detected matching given configuration for text: " + text)
}
