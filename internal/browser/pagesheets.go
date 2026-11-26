// This file ports no upstream file. It has the rule set of the style sheets
// of the page: the link elements in the head of the page of mermaid-cli.

package browser

import (
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
	"github.com/bvolpato/mermaid-go-renderer/internal/webfonts"
)

// StandardPageSheets returns the raw texts of webfonts.Sheets (the page sheets
// of mermaid-cli), shared across callers so embed.FS is read only once and
// slice identity comparison succeeds in O(1).
func StandardPageSheets() []string {
	return standardPageSheets()
}

// standardPageSheets returns the raw texts of webfonts.Sheets (the page sheets
// of mermaid-cli). Every style rule in those sheets other than the three
// ":root, :host" custom-property blocks and "body" requires a ".fa*" or
// ".katex*" class on the element or an ancestor.
var standardPageSheets = sync.OnceValue(func() []string {
	out := make([]string, len(webfonts.Sheets))
	for i, sheet := range webfonts.Sheets {
		out[i] = sheet.Text
	}
	return out
})

// standardBaseRuleSet parses only the non-.fa / non-.katex rules of
// webfonts.Sheets (the ":root, :host" custom properties of Font Awesome and
// the "body" rule of KaTeX). Diagrams without Font Awesome or KaTeX classes
// never need to parse the 175 KiB of icon and math rules.
var standardBaseRuleSet = sync.OnceValue(func() *ruleSet {
	sheets := standardPageSheets()
	if len(sheets) == 0 {
		return nil
	}
	var baseTexts []string
	for i, s := range sheets {
		// Only brands.css, regular.css, solid.css, and katex.css have top-level
		// ":root, :host" or "body" rules; skip 111 KiB fontawesome.css and pure
		// @font-face sheets.
		p := webfonts.Sheets[i].Path
		if strings.HasSuffix(p, "fontawesome.css") || strings.Contains(p, "recursive/") || strings.Contains(p, "open-sans/") {
			continue
		}
		// Extract any top-level ":root, :host { ... }" or "body { ... }" rules
		// anchored at the start of a line so class suffixes like ".accent-body {"
		// do not match.
		for _, prefix := range []string{":root,:host{", ":root, :host{", ":root, :host {", "body{", "body {"} {
			for pos := 0; pos < len(s); {
				rel := strings.Index(s[pos:], prefix)
				if rel < 0 {
					break
				}
				idx := pos + rel
				if idx == 0 || s[idx-1] == '\n' || s[idx-1] == '\r' || s[idx-1] == '}' {
					if end := strings.IndexByte(s[idx:], '}'); end >= 0 {
						baseTexts = append(baseTexts, s[idx:idx+end+1])
						pos = idx + end + 1
						continue
					}
				}
				pos = idx + len(prefix)
			}
		}
	}
	return parseRuleSet(baseTexts, false)
})

func hasFAOrKatexClass(cls string) bool {
	for i := 0; i < len(cls); {
		for i < len(cls) && (cls[i] == ' ' || cls[i] == '\t' || cls[i] == '\n' || cls[i] == '\r' || cls[i] == '\f') {
			i++
		}
		start := i
		for i < len(cls) && cls[i] != ' ' && cls[i] != '\t' && cls[i] != '\n' && cls[i] != '\r' && cls[i] != '\f' {
			i++
		}
		if i > start {
			tok := cls[start:i]
			if (len(tok) >= 2 && tok[0] == 'f' && tok[1] == 'a' && (len(tok) == 2 || tok[2] == '-' || len(tok) == 3)) ||
				strings.Contains(tok, "katex") {
				return true
			}
		}
	}
	return false
}

func nodeNeedsFullPageSheets(n *dom.Node) bool {
	for cur := n; cur != nil; cur = cur.Parent {
		if cur.Type != dom.ElementNode {
			continue
		}
		if cls, ok := cur.GetAttr("class"); ok && cls != "" {
			if hasFAOrKatexClass(cls) {
				return true
			}
		}
	}
	return false
}

// The page of mermaid-cli has style sheets in its head (internal/webfonts):
// the four sheets of Font Awesome and the sheets with the @font-face rules
// of the web fonts. dom.Document.PageSheets has their text. They are author
// style sheets that come before every style element of the body, so a rule
// of them loses against a rule of the body with the same specificity.
//
// The sheets are large: the Font Awesome sheets have a rule for each icon
// class. So their rule set is made one time for the process, and every
// document with the same sheets shares it. It does not change after that.
// The rules are in the same index as the rules of a document, by id, class
// and tag: an element without a class of the sheets tries none of the icon
// rules.

// pageRuleSets has the rule sets of the page sheets that documents had. A
// process has one list of page sheets, or two, so the list is short.
var pageRuleSets struct {
	sync.Mutex
	list []pageRuleSetEntry
}

type pageRuleSetEntry struct {
	sheets []string
	rules  *ruleSet
}

// maxPageRuleSets bounds the list of pageRuleSets.
const maxPageRuleSets = 4

// sameSheets reports whether two lists of sheets have the same text. Two
// strings with the same data compare in constant time, which is the case
// for the list that internal/render gives to every document.
func sameSheets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) > 0 && &a[0] == &b[0] {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// pageRuleSetOf returns the rule set of a list of page sheets, or nil for
// no sheets.
func pageRuleSetOf(sheets []string) *ruleSet {
	if len(sheets) == 0 {
		return nil
	}
	if !memo.Enabled() {
		return parseRuleSet(sheets, false)
	}
	pageRuleSets.Lock()
	defer pageRuleSets.Unlock()
	for i := range pageRuleSets.list {
		if sameSheets(pageRuleSets.list[i].sheets, sheets) {
			return pageRuleSets.list[i].rules
		}
	}
	// The selectors do not go into the selector cache of the dom package:
	// the rule set keeps them, and they would fill that cache.
	rs := parseRuleSet(sheets, false)
	if len(pageRuleSets.list) >= maxPageRuleSets {
		pageRuleSets.list = pageRuleSets.list[1:]
	}
	// The entry has its own list: the caller can change its slice later.
	pageRuleSets.list = append(pageRuleSets.list, pageRuleSetEntry{sheets: append([]string(nil), sheets...), rules: rs})
	return rs
}

// pageRules returns the full rule set of the page sheets of a document, or nil
// when the document has none.
func (e *Engine) pageRules(doc *dom.Document) *ruleSet {
	if len(doc.PageSheets) == 0 {
		return nil
	}
	if e.pageDoc != doc || !e.pageFull || e.page == nil {
		e.page, e.pageDoc, e.pageStd, e.pageFull = pageRuleSetOf(doc.PageSheets), doc, false, true
	}
	return e.page
}

// pageRulesFor returns the rule set of the page sheets needed to cascade n.
// For the standard mermaid-cli page sheets, the full 175 KiB of Font Awesome
// and KaTeX rules are deferred until an element or ancestor carries a class
// containing "fa" or "katex".
func (e *Engine) pageRulesFor(doc *dom.Document, n *dom.Node) *ruleSet {
	if len(doc.PageSheets) == 0 {
		return nil
	}
	if e.pageDoc != doc {
		e.pageDoc = doc
		if memo.Enabled() && sameSheets(doc.PageSheets, standardPageSheets()) {
			e.pageStd = true
			e.pageFull = false
			e.page = standardBaseRuleSet()
		} else {
			e.pageStd = false
			e.pageFull = true
			e.page = pageRuleSetOf(doc.PageSheets)
		}
	}
	if e.pageStd && !e.pageFull && nodeNeedsFullPageSheets(n) {
		e.pageFull = true
		e.page = pageRuleSetOf(doc.PageSheets)
	}
	return e.page
}
