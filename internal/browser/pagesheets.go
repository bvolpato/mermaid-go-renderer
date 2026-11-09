// This file ports no upstream file. It has the rule set of the style sheets
// of the page: the link elements in the head of the page of mermaid-cli.

package browser

import (
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

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

// pageRules returns the rule set of the page sheets of a document, or nil
// when the document has none.
func (e *Engine) pageRules(doc *dom.Document) *ruleSet {
	if len(doc.PageSheets) == 0 {
		return nil
	}
	if e.pageDoc != doc || e.page == nil {
		e.page, e.pageDoc = pageRuleSetOf(doc.PageSheets), doc
	}
	return e.page
}
