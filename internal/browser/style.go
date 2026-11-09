// Ported from Chromium 156.0.8078.4 (Blink), in part: four parts of the
// font size and line height code follow the functions of Blink that their
// comments name. The rest of the file ports no upstream file. It has the
// CSS cascade and the inherited text properties.
//
//   - FontDescription::IsAbsoluteSize: platform/fonts/font_description.h
//     (LGPL-2.0-or-later).
//   - FontDescription::EffectiveFontSize: platform/fonts/font_description.cc
//     (BSD-3-Clause, with the copyright of Nicholas Shanks and Apple in the
//     file).
//   - CSSToLengthConversionData::FontSizes::Ex:
//     core/css/css_to_length_conversion_data.cc (BSD-3-Clause, with the
//     copyright of Google in the file).
//   - StyleBuilderConverter::ConvertLineHeight, for a math function:
//     core/css/resolver/style_builder_converter.cc (BSD-3-Clause, with the
//     copyright of Google in the file).
//
// The paths are in third_party/blink/renderer. The license of each file is
// the license in its header at that version. See NOTICE in this directory.

package browser

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

// This file is the CSS cascade: which declaration wins for an element, and
// the inherited text properties that layout needs.

type cascadeRule struct {
	sel   *dom.Selector
	decls []css.Decl
	order int
	// ancestorClass is sel.AncestorClass(): the rule can match an element
	// only when an ancestor of the element has this class.
	ancestorClass string
	// pseudo is the pseudo-element that the rule styles, or pseudoNone.
	pseudo pseudoKind
}

// ruleSet has the rules of the style sheets of a document, or of the style
// sheets of the page (pagesheets.go). It does not change after parseRuleSet
// made it, and engines of many renders share it.
type ruleSet struct {
	byID, byClass, byTag map[string][]*cascadeRule
	// idFirst, classFirst and tagFirst have the first bytes of the names
	// in the three maps. A name whose first byte is not there is in no
	// rule, and needs no lookup in the map: an element of a diagram tries
	// no rule of the style sheets of Font Awesome, whose classes start
	// with f.
	idFirst, classFirst, tagFirst firstBytes
	universal                     []*cascadeRule
	// root has the rules whose selector ends in :root. Only the root
	// element tries them.
	root []*cascadeRule
	// pseudo has the rules for the pseudo-elements ::before and ::after.
	// See pseudo.go.
	pseudo []*cascadeRule
	count  int
	// deps is what the selectors of the rules read in the tree besides an
	// element and its ancestors. See cascadeIsCurrent.
	deps dom.SelectorDeps
}

// nodeStyle is cached in dom.Node.Cache.
//
// A drawing changes the document between its measurements: it appends an
// element, sets an attribute, measures, and goes on. The cascade of an
// element reads the element, its ancestors and the rules, and a change in
// another place of the document does not change its result. So the cascade
// and the text style stay while those inputs are the same (see
// cascadeIsCurrent). The layout reads the subtree and more, so it stays only
// until the next change of the document.
type nodeStyle struct {
	// gen is the generation of the document at the last check of the
	// style. In that generation the style is current.
	gen uint64
	// rules and at are the rule set that the cascade read and the
	// generation of the document at that time.
	rules    *ruleSet
	at       uint64
	declared map[string]string // winning declarations on this element
	fromAttr map[string]bool   // the value comes from a presentation attribute
	text     *textStyle
	layout   any // layout cache of the engine (text or HTML layout)
	layoutOK bool
	// pseudo has the boxes of the pseudo-elements of the element, or nil
	// when they are not known yet. They depend on what the cascade of the
	// element depends on. See pseudo.go.
	pseudo *pseudoBoxes
}

// firstBytes is a set of bytes: the first bytes of the names of an index of
// a rule set.
type firstBytes [4]uint64

func (f *firstBytes) add(name string) {
	if name != "" {
		f[name[0]>>6] |= 1 << (name[0] & 63)
	}
}

// has reports whether a name of the index can start as name does.
func (f *firstBytes) has(name string) bool {
	return name != "" && f[name[0]>>6]&(1<<(name[0]&63)) != 0
}

// presentationAttrs are the SVG attributes that act as CSS declarations.
var presentationAttrs = map[string]bool{
	"alignment-baseline": true, "baseline-shift": true, "clip-path": true, "clip-rule": true, "color": true,
	"cursor": true, "direction": true, "display": true, "dominant-baseline": true, "fill": true,
	"fill-opacity": true, "fill-rule": true, "filter": true, "flood-color": true, "flood-opacity": true,
	"font-family": true, "font-size": true, "font-style": true, "font-variant": true, "font-weight": true,
	"letter-spacing": true, "marker-end": true, "marker-mid": true, "marker-start": true, "mask": true,
	"opacity": true, "overflow": true, "paint-order": true, "pointer-events": true, "shape-rendering": true,
	"stop-color": true, "stop-opacity": true, "stroke": true, "stroke-dasharray": true, "stroke-dashoffset": true,
	"stroke-linecap": true, "stroke-linejoin": true, "stroke-miterlimit": true, "stroke-opacity": true,
	"stroke-width": true, "text-anchor": true, "text-decoration": true, "text-rendering": true,
	"transform-origin": true, "unicode-bidi": true, "vector-effect": true, "visibility": true,
	"word-spacing": true, "writing-mode": true, "white-space": true,
}

// inheritedProps are the CSS properties that inherit by default.
var inheritedProps = map[string]bool{
	"color": true, "cursor": true, "direction": true, "fill": true, "fill-opacity": true, "fill-rule": true,
	"font-family": true, "font-size": true, "font-style": true, "font-variant": true, "font-weight": true,
	"letter-spacing": true, "line-height": true, "marker-end": true, "marker-mid": true, "marker-start": true,
	"paint-order": true, "pointer-events": true, "shape-rendering": true, "stroke": true,
	"stroke-dasharray": true, "stroke-dashoffset": true, "stroke-linecap": true, "stroke-linejoin": true,
	"stroke-miterlimit": true, "stroke-opacity": true, "stroke-width": true, "text-align": true,
	"text-anchor": true, "text-rendering": true, "text-transform": true, "visibility": true,
	"white-space": true, "word-break": true, "word-spacing": true, "overflow-wrap": true,
	"dominant-baseline": true, "clip-rule": true, "text-indent": true, "list-style-type": true,
	"border-collapse": true, "writing-mode": true, "font-kerning": true, "tab-size": true,
}

// ruleSets has the rule set of a document by the text of its style
// elements. A rule set is a function of that text alone, and the renders of
// one diagram type with one theme have the same style sheet.
var ruleSets = memo.New[*ruleSet](64)

// collectRules gathers the rules of every <style> element of the document.
// Parsed sheets are cached by their text.
func (e *Engine) collectRules(doc *dom.Document) *ruleSet {
	// The style elements and their text are the same while the style
	// generation of the document is the same.
	styleGen := doc.StyleGeneration()
	if e.rules != nil && e.rulesDoc == doc && e.rulesGen == styleGen {
		return e.rules
	}
	var texts []string
	var walk func(n *dom.Node)
	walk = func(n *dom.Node) {
		for c := n.FirstChild; c != nil; c = c.Next {
			if c.Type != dom.ElementNode {
				continue
			}
			if c.Tag == "style" {
				texts = append(texts, c.TextContent())
				continue
			}
			walk(c)
		}
	}
	walk(doc.Body)
	key := strings.Join(texts, "\x00")
	if e.rules != nil && key == e.rulesKey {
		e.rulesGen, e.rulesDoc = styleGen, doc
		return e.rules
	}
	rs, ok := ruleSets.Get(key)
	if !ok {
		rs = parseRuleSet(texts, true)
		ruleSets.Put(key, rs)
	}
	e.rules, e.rulesKey, e.rulesGen, e.rulesDoc = rs, key, styleGen, doc
	return rs
}

// parseRuleSet makes the rule set of the texts of style sheets. With
// cacheSelectors the selectors go through the selector cache of the dom
// package, which the style sheets of a diagram type fill one time.
func parseRuleSet(texts []string, cacheSelectors bool) *ruleSet {
	rs := &ruleSet{byID: map[string][]*cascadeRule{}, byClass: map[string][]*cascadeRule{}, byTag: map[string][]*cascadeRule{}}
	for _, text := range texts {
		for _, r := range css.ParseStylesheet(text) {
			var list dom.SelectorList
			var err error
			if cacheSelectors {
				list, err = dom.ParseSelector(r.Selector)
			} else {
				list, err = dom.ParseSelectorUncached(r.Selector)
			}
			if err != nil {
				continue
			}
			for _, sel := range list {
				cr := &cascadeRule{sel: sel, decls: r.Decls, order: rs.count, ancestorClass: sel.AncestorClass()}
				rs.count++
				rs.deps |= sel.Deps()
				if kind := pseudoKindOfName(sel.PseudoElement()); kind != pseudoNone {
					// The rule styles a pseudo-element. It matches no
					// element, and the cascade of an element does not try it.
					cr.pseudo = kind
					rs.pseudo = append(rs.pseudo, cr)
					continue
				}
				if sel.NeverMatches() {
					// A rule for a state such as :hover, or for another
					// pseudo-element: no cascade needs it.
					continue
				}
				switch kind, name := sel.Key(); {
				case kind == '#':
					rs.byID[name] = append(rs.byID[name], cr)
					rs.idFirst.add(name)
				case kind == '.':
					rs.byClass[name] = append(rs.byClass[name], cr)
					rs.classFirst.add(name)
				case kind == 't':
					rs.byTag[name] = append(rs.byTag[name], cr)
					rs.tagFirst.add(name)
				case sel.SubjectIsRoot():
					rs.root = append(rs.root, cr)
				default:
					rs.universal = append(rs.universal, cr)
				}
			}
		}
	}
	return rs
}

func (e *Engine) styleOf(n *dom.Node) *nodeStyle {
	doc := n.Document()
	gen := doc.Generation()
	st, ok := n.Cache.(*nodeStyle)
	if ok {
		if st.gen == gen {
			return st
		}
		// The document changed. The layout is not kept. The cascade and
		// the text style are kept when what they read did not change.
		st.layout, st.layoutOK = nil, false
		if e.cascadeIsCurrent(n, st) {
			st.gen = gen
			return st
		}
	}
	st = &nodeStyle{gen: gen, at: gen}
	n.Cache = st
	e.cascade(n, st)
	return st
}

// cascadeIsCurrent reports whether the cascade of n that st has is the
// cascade that the Engine would compute now.
//
// The cascade reads the rule set, and from the tree what the selectors of
// the rules read, plus the custom properties of the ancestors:
//
//   - the element and its ancestors: their names, their attributes, their
//     inline styles and their links. A node has the generation of its last
//     such change (dom.Node.SelfGeneration).
//   - for a rule set with :nth-child and the like, the position of the
//     element and of each ancestor among its siblings. A parent has the
//     generation of the last change of its children that can move a child
//     (ChildOrderGeneration), and of the last change of its children at all
//     (ChildrenGeneration), which the forms that count from the end need.
//   - for a rule set with a sibling combinator or :empty, more of the tree:
//     such a cascade is current only in the generation that computed it.
//
// A text node and a comment have no cascade.
func (e *Engine) cascadeIsCurrent(n *dom.Node, st *nodeStyle) bool {
	if n.Type != dom.ElementNode {
		return true
	}
	rs := e.collectRules(n.Document())
	deps := rs.deps
	if page := e.pageRules(n.Document()); page != nil {
		deps |= page.deps
	}
	if st.rules != rs || deps&dom.DepsOther != 0 {
		return false
	}
	at := st.at
	for cur := n; cur != nil; cur = cur.Parent {
		if cur.SelfGeneration() > at {
			return false
		}
		parent := cur.Parent
		if parent == nil || deps == 0 {
			continue
		}
		if deps&dom.DepsSiblings != 0 {
			if parent.ChildrenGeneration() > at {
				return false
			}
		} else if parent.ChildOrderGeneration() > at {
			return false
		}
	}
	return true
}

type matched struct {
	decl      *css.Decl
	spec      dom.Specificity
	order     int
	important bool
}

func (e *Engine) cascade(n *dom.Node, st *nodeStyle) {
	st.declared = map[string]string{}
	if n.Type != dom.ElementNode {
		return
	}
	if n.NS == pseudoNS {
		e.cascadePseudo(n, st)
		return
	}
	// 1. Presentation attributes: the lowest author level.
	if n.NS == dom.NSSVG {
		for i := range n.Attrs {
			a := &n.Attrs[i]
			if a.NS == "" && presentationAttrs[a.Local] {
				if st.fromAttr == nil {
					st.fromAttr = map[string]bool{}
				}
				st.declared[a.Local] = strings.TrimSpace(a.Value)
				st.fromAttr[a.Local] = true
			}
		}
	}
	// 2. Author rules: the style sheets of the page, then the style
	// elements of the document.
	rs := e.collectRules(n.Document())
	st.rules = rs
	page := e.pageRules(n.Document())
	var ms []matched
	// The classes of the ancestors of n, collected for the first rule that
	// needs one of them. Most rules of a style sheet of mermaid are for the
	// descendants of an element with a class: a rule whose class no ancestor
	// has cannot match, and is not tried.
	ancestorClasses, haveAncestorClasses := e.classBuf[:0], false
	// base is the number of rules in the sheets before the rule set.
	try := func(rules []*cascadeRule, base int) {
		for _, r := range rules {
			if r.ancestorClass != "" {
				if !haveAncestorClasses {
					haveAncestorClasses = true
					for p := n.Parent; p != nil; p = p.Parent {
						ancestorClasses = p.AppendClasses(ancestorClasses)
					}
				}
				if !slices.Contains(ancestorClasses, r.ancestorClass) {
					continue
				}
			}
			if !r.sel.Match(n) {
				continue
			}
			for i := range r.decls {
				d := &r.decls[i]
				ms = append(ms, matched{decl: d, spec: r.sel.Specificity(), order: base + r.order, important: d.Important})
			}
		}
	}
	id, classes := n.ID(), n.ClassList()
	tag := strings.ToLower(n.Tag)
	match := func(set *ruleSet, base int) {
		if set.idFirst.has(id) {
			try(set.byID[id], base)
		}
		for _, cl := range classes {
			if set.classFirst.has(cl) {
				try(set.byClass[cl], base)
			}
		}
		if set.tagFirst.has(tag) {
			try(set.byTag[tag], base)
		}
		try(set.universal, base)
		if n.Parent == nil {
			try(set.root, base)
		}
	}
	if page != nil {
		match(page, 0)
		match(rs, page.count)
	} else {
		match(rs, 0)
	}
	// Keep the storage for the next cascade.
	e.classBuf = ancestorClasses[:0]
	var inline []css.Decl
	if v, ok := n.InlineStyleExact(); ok && v != "" {
		inline = css.ParseDeclarations(v)
	}
	e.applyCascade(n, st, ms, inline)
}

// applyCascade sorts the declarations that matched an element and applies
// them, with the declarations of the style attribute.
func (e *Engine) applyCascade(n *dom.Node, st *nodeStyle, ms []matched, inline []css.Decl) {
	sort.SliceStable(ms, func(i, j int) bool {
		a, b := ms[i], ms[j]
		if a.important != b.important {
			return !a.important
		}
		if a.spec != b.spec {
			return a.spec.Less(b.spec)
		}
		return a.order < b.order
	})
	// The cascade runs two times: first for the custom properties, then for
	// the other properties, whose var() references read the custom
	// properties of the element and of its ancestors.
	custom := true
	apply := func(d *css.Decl) {
		if strings.HasPrefix(d.Name, "--") != custom {
			return
		}
		value := d.Value
		if !custom && hasVar(value) {
			resolved, ok := e.resolveVars(n, value)
			if !ok {
				// Invalid at computed-value time: the property acts as unset.
				for _, name := range longhandsOf(d.Name) {
					st.declared[name] = unsetValue
					delete(st.fromAttr, name)
				}
				return
			}
			value = resolved
		}
		if d.Name == "content" {
			// The CSS parser of a browser drops a declaration whose value
			// is not valid, and an earlier declaration then wins. With a
			// var() reference the declaration is invalid at computed-value
			// time, and the property acts as unset.
			if _, ok := css.ParseContent(value); !ok {
				if value != d.Value {
					st.declared[d.Name] = unsetValue
				}
				return
			}
		}
		expandDecl(d.Name, value, func(name, value string) {
			st.declared[name] = value
			delete(st.fromAttr, name)
		})
	}
	firstImportant := len(ms)
	for i := range ms {
		if ms[i].important {
			firstImportant = i
			break
		}
	}
	// 3. The style attribute, then !important rules, then inline !important.
	for _, custom = range []bool{true, false} {
		if !custom {
			// A presentation attribute can have var() references too.
			for name := range st.fromAttr {
				if value := st.declared[name]; hasVar(value) {
					resolved, ok := e.resolveVars(n, value)
					if !ok {
						resolved = unsetValue
					}
					st.declared[name] = resolved
				}
			}
		}
		for i := 0; i < firstImportant; i++ {
			apply(ms[i].decl)
		}
		for i := range inline {
			if !inline[i].Important {
				apply(&inline[i])
			}
		}
		for i := firstImportant; i < len(ms); i++ {
			apply(ms[i].decl)
		}
		for i := range inline {
			if inline[i].Important {
				apply(&inline[i])
			}
		}
	}
}

// expandDecl expands the shorthands that layout reads.
func expandDecl(name, value string, set func(name, value string)) {
	switch name {
	case "font":
		expandFontShorthand(value, set)
	case "margin", "padding":
		parts := strings.Fields(value)
		var t, r, b, l string
		switch len(parts) {
		case 1:
			t, r, b, l = parts[0], parts[0], parts[0], parts[0]
		case 2:
			t, r, b, l = parts[0], parts[1], parts[0], parts[1]
		case 3:
			t, r, b, l = parts[0], parts[1], parts[2], parts[1]
		case 4:
			t, r, b, l = parts[0], parts[1], parts[2], parts[3]
		default:
			return
		}
		set(name+"-top", t)
		set(name+"-right", r)
		set(name+"-bottom", b)
		set(name+"-left", l)
	case "margin-block", "padding-block", "margin-inline", "padding-inline":
		// The logical shorthands. The engine lays out horizontal text from
		// left to right: block is top and bottom, inline is left and right.
		parts := strings.Fields(value)
		if len(parts) < 1 || len(parts) > 2 {
			return
		}
		prefix, axis, _ := strings.Cut(name, "-")
		start, end := "-top", "-bottom"
		if axis == "inline" {
			start, end = "-left", "-right"
		}
		set(prefix+start, parts[0])
		set(prefix+end, parts[len(parts)-1])
	case "margin-block-start", "padding-block-start":
		set(name[:strings.IndexByte(name, '-')]+"-top", value)
	case "margin-block-end", "padding-block-end":
		set(name[:strings.IndexByte(name, '-')]+"-bottom", value)
	case "margin-inline-start", "padding-inline-start":
		set(name[:strings.IndexByte(name, '-')]+"-left", value)
	case "margin-inline-end", "padding-inline-end":
		set(name[:strings.IndexByte(name, '-')]+"-right", value)
	case "border":
		// The shorthand resets the longhands of the four sides.
		set(name, value)
		for _, side := range borderSides {
			set("border-"+side+"-width", "")
			set("border-"+side+"-style", "")
		}
	case "border-top", "border-right", "border-bottom", "border-left":
		set(name, value)
		set(name+"-width", "")
		set(name+"-style", "")
	case "border-width", "border-style":
		parts := strings.Fields(value)
		var t, r, b, l string
		switch len(parts) {
		case 1:
			t, r, b, l = parts[0], parts[0], parts[0], parts[0]
		case 2:
			t, r, b, l = parts[0], parts[1], parts[0], parts[1]
		case 3:
			t, r, b, l = parts[0], parts[1], parts[2], parts[1]
		case 4:
			t, r, b, l = parts[0], parts[1], parts[2], parts[3]
		default:
			return
		}
		suffix := name[len("border"):]
		set("border-top"+suffix, t)
		set("border-right"+suffix, r)
		set("border-bottom"+suffix, b)
		set("border-left"+suffix, l)
		set(borderLonghandsMark, "1")
	case "border-top-width", "border-right-width", "border-bottom-width", "border-left-width",
		"border-top-style", "border-right-style", "border-bottom-style", "border-left-style":
		set(name, value)
		set(borderLonghandsMark, "1")
	case "background":
		set("background", value)
		// The color is the last component that parses as a color.
		for _, part := range strings.Fields(value) {
			if _, ok := css.ParseColor(part); ok {
				set("background-color", part)
			}
		}
		if _, ok := css.ParseColor(value); ok {
			set("background-color", value)
		}
	case "white-space":
		set(name, strings.ToLower(value))
	default:
		set(name, value)
	}
}

// borderSides are the sides of a box in the order of a shorthand.
var borderSides = [...]string{"top", "right", "bottom", "left"}

// borderLonghandsMark is a name in the declarations of an element that has
// a declaration of the width or the style of one border side. It is not a
// CSS property. borderEdges reads the eight properties of the sides only
// for an element that has it.
const borderLonghandsMark = "\x00border-longhands"

func expandFontShorthand(value string, set func(name, value string)) {
	// [style] [variant] [weight] [stretch] size[/line-height] family
	fields := strings.Fields(value)
	i := 0
	for ; i < len(fields); i++ {
		f := strings.ToLower(fields[i])
		switch f {
		case "normal":
			continue
		case "italic", "oblique":
			set("font-style", f)
			continue
		case "bold", "bolder", "lighter":
			set("font-weight", f)
			continue
		case "small-caps":
			continue
		}
		if _, err := strconv.Atoi(f); err == nil {
			set("font-weight", f)
			continue
		}
		break
	}
	if i >= len(fields) {
		return
	}
	size := fields[i]
	if slash := strings.IndexByte(size, '/'); slash >= 0 {
		set("line-height", size[slash+1:])
		size = size[:slash]
	} else {
		set("line-height", "normal")
	}
	set("font-size", size)
	if i+1 < len(fields) {
		set("font-family", strings.Join(fields[i+1:], " "))
	}
}

// declared returns the cascaded value of a property on the element itself.
func (e *Engine) declared(n *dom.Node, prop string) (string, bool) {
	if n == nil || n.Type != dom.ElementNode {
		return "", false
	}
	v, ok := e.styleOf(n).declared[prop]
	if ok {
		lower := strings.ToLower(v)
		if lower == "initial" || lower == "unset" || lower == "revert" {
			return "", false
		}
	}
	return v, ok
}

// lookup returns the value of a property with inheritance, or "" when no
// element in the chain declares it.
func (e *Engine) lookup(n *dom.Node, prop string) string {
	inherits := inheritedProps[prop]
	for cur := n; cur != nil && cur.Type == dom.ElementNode; cur = cur.Parent {
		if v, ok := e.declared(cur, prop); ok {
			if strings.EqualFold(v, "inherit") {
				continue
			}
			return v
		}
		if !inherits {
			return ""
		}
	}
	return ""
}

// ---- text style ----

type lineHeightKind uint8

const (
	lhNormal lineHeightKind = iota
	lhNumber
	lhLength
)

// textStyle is the resolved set of inherited text properties of an element.
type textStyle struct {
	families      []string
	size          float64 // px
	weight        int
	italic        bool
	lhKind        lineHeightKind
	lhValue       float64
	whiteSpace    string
	letterSpacing float64
	textAnchor    string
	textAlign     string
	textTransform string
	visibility    string
	chain         *font.Chain
	// sizeKeyword is the keyword of the font size (1 for xx-small to 8 for
	// xxx-large), or 0. sizeAbsolute is FontDescription::IsAbsoluteSize.
	// See fontmono.go.
	sizeKeyword  int
	sizeAbsolute bool
}

// fontSize is FontDescription::EffectiveFontSize in Blink: the size of the
// platform font that shapes the text and gives the metrics. It is the
// computed font size cut to the precision of the font cache, 1/100 px, in
// float32 arithmetic. So 33.46875px gives the font of 33.46px, and 16.8px,
// which is 16.799999 as a float32, gives the font of 16.79px. Lengths in em
// and the computed style use the full size.
func (ts *textStyle) fontSize() float64 {
	const precisionMultiplier = 100
	scaled := float32(ts.size) * precisionMultiplier
	return float64(float32(math.Floor(float64(scaled))) / precisionMultiplier)
}

func (ts *textStyle) lineHeightPx(m font.Metrics) float64 {
	switch ts.lhKind {
	case lhNumber:
		return float64(ts.lhValue * ts.size)
	case lhLength:
		return ts.lhValue
	}
	return m.LineSpacing
}

var uaBold = map[string]bool{"b": true, "strong": true, "th": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true}
var uaItalic = map[string]bool{"i": true, "em": true, "cite": true, "var": true, "dfn": true, "address": true}
var uaMono = map[string]bool{"code": true, "kbd": true, "samp": true, "tt": true, "pre": true}
var uaHeadingSize = map[string]float64{"h1": 2, "h2": 1.5, "h3": 1.17, "h4": 1, "h5": 0.83, "h6": 0.67}

// textStyleOf resolves the inherited text properties of an element.
func (e *Engine) textStyleOf(n *dom.Node) *textStyle {
	if n == nil || n.Type != dom.ElementNode {
		// The initial values: the default font of the browser.
		return &textStyle{families: []string{"serif"}, size: 16, weight: 400, whiteSpace: "normal", textAnchor: "start", textAlign: "start", visibility: "visible", sizeKeyword: fontSizeKeywordIndex["medium"]}
	}
	st := e.styleOf(n)
	if st.text != nil {
		return st.text
	}
	parent := e.textStyleOf(n.Parent)
	ts := *parent
	ts.chain = nil
	inherit := func(v string) bool { return strings.EqualFold(v, "inherit") }

	// User agent defaults for HTML elements.
	if n.NS == dom.NSHTML {
		switch {
		case n.Tag == "b" || n.Tag == "strong":
			// The style sheet of the user agent has font-weight: bolder
			// for b and strong: in a bold parent they have the weight 900.
			ts.weight = resolveFontWeight("bolder", parent.weight)
		case uaBold[n.Tag]:
			ts.weight = 700
		case uaItalic[n.Tag]:
			ts.italic = true
		case uaMono[n.Tag]:
			ts.families = []string{"monospace"}
		}
		if f, ok := uaHeadingSize[n.Tag]; ok {
			ts.size = float64(parent.size * f)
			ts.sizeKeyword = 0
		}
		switch n.Tag {
		case "small":
			ts.size = parent.size * 5 / 6
			ts.sizeKeyword = 0
		case "sub", "sup":
			ts.size = parent.size * 5 / 6
			ts.sizeKeyword = 0
		case "pre":
			ts.whiteSpace = "pre"
		case "center":
			ts.textAlign = "center"
		case "th":
			ts.textAlign = "center"
		}
	}

	fromAttr := st.fromAttr
	if v, ok := e.declared(n, "font-family"); ok && !inherit(v) {
		ts.families = css.SplitFontFamily(v)
	}
	if v, ok := e.declared(n, "font-size"); ok && !inherit(v) {
		if f, ok := exValue(v); ok {
			// The ex of font-size is the x-height of the font of the
			// parent. The computed font size is a float32.
			ts.size = float64(float32(f * e.exSize(parent)))
			ts.sizeKeyword = 0
		} else if px, ok := e.calcFontSize(v, parent); ok {
			ts.size = px
			ts.sizeKeyword, ts.sizeAbsolute = 0, true
		} else if px, ok := resolveFontSize(v, parent.size, fromAttr["font-size"]); ok {
			ts.size = px
			if keyword := fontSizeKeywordIndex[strings.ToLower(strings.TrimSpace(v))]; keyword != 0 {
				// A keyword is not an absolute size, and its size is that
				// of the kind of family.
				ts.sizeKeyword, ts.sizeAbsolute = keyword, false
				ts.size = keywordFontSize(keyword, isMonospace(ts.families))
			} else {
				ts.sizeKeyword = 0
				ts.sizeAbsolute = parent.sizeAbsolute || !fontSizeIsRelative(v)
			}
		}
	}
	// A change between monospace and another family scales a size that is
	// not absolute. See fontmono.go.
	genericFamilyChange(&ts, parent)
	if v, ok := e.declared(n, "font-weight"); ok && !inherit(v) {
		ts.weight = resolveFontWeight(v, parent.weight)
	}
	if v, ok := e.declared(n, "font-style"); ok && !inherit(v) {
		lower := strings.ToLower(v)
		ts.italic = lower == "italic" || strings.HasPrefix(lower, "oblique")
	}
	if v, ok := e.declared(n, "line-height"); ok && !inherit(v) {
		lower := strings.ToLower(strings.TrimSpace(v))
		switch {
		case lower == "normal":
			ts.lhKind = lhNormal
		case strings.HasSuffix(lower, "%"):
			if f, err := strconv.ParseFloat(lower[:len(lower)-1], 64); err == nil {
				ts.lhKind, ts.lhValue = lhLength, float64(f/100*ts.size)
			}
		default:
			if f, err := strconv.ParseFloat(lower, 64); err == nil {
				ts.lhKind, ts.lhValue = lhNumber, f
			} else if css.IsMathFunction(lower) {
				// ConvertLineHeight: a math function that is a number is a
				// number. One with a percentage is a length: the
				// percentage is of the font size, as a LayoutUnit.
				if c, ok := e.evalCalc(lower, &ts); ok {
					switch {
					case c.IsNumber:
						ts.lhKind, ts.lhValue = lhNumber, c.Number
					case c.HasPercent:
						fontSize := luFromFloat(float32(ts.size)).f32()
						value := luFromFloat(float32(c.Px) + float32(fontSize*float32(c.Percent)/100))
						ts.lhKind, ts.lhValue = lhLength, value.px()
					default:
						ts.lhKind, ts.lhValue = lhLength, clampCalcPx(c.Px)
					}
				}
			} else if px, ok := e.cssLength(lower, &ts, false); ok {
				ts.lhKind, ts.lhValue = lhLength, px
			}
		}
	} else if ts.lhKind == lhLength && parent.lhKind == lhLength {
		ts.lhValue = parent.lhValue
	}
	if v, ok := e.declared(n, "white-space"); ok && !inherit(v) {
		ts.whiteSpace = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := n.GetAttr("xml:space"); ok && v == "preserve" {
		if _, declared := e.declared(n, "white-space"); !declared {
			ts.whiteSpace = "pre"
		}
	}
	if v, ok := e.declared(n, "letter-spacing"); ok && !inherit(v) {
		if strings.EqualFold(v, "normal") {
			ts.letterSpacing = 0
		} else if px, ok := e.cssLength(v, &ts, fromAttr["letter-spacing"]); ok {
			ts.letterSpacing = px
		}
	}
	if v, ok := e.declared(n, "text-anchor"); ok && !inherit(v) {
		ts.textAnchor = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := e.declared(n, "text-align"); ok && !inherit(v) {
		ts.textAlign = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := e.declared(n, "text-transform"); ok && !inherit(v) {
		ts.textTransform = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := e.declared(n, "visibility"); ok && !inherit(v) {
		ts.visibility = strings.ToLower(strings.TrimSpace(v))
	}
	st.text = &ts
	return st.text
}

// exSize is the length of the CSS unit ex for a text style: the x-height of
// its primary font (CSSToLengthConversionData::FontSizes::Ex in Blink). It
// is half of the font size for a font that has no x-height.
func (e *Engine) exSize(ts *textStyle) float64 {
	if face := e.chainOf(ts).Primary(); face != nil && face.HasXHeight() {
		return face.Metrics(ts.fontSize()).XHeight
	}
	return ts.size / 2
}

// exValue parses a length in the unit ex and returns its number.
func exValue(v string) (float64, bool) {
	s := strings.ToLower(strings.TrimSpace(v))
	if !strings.HasSuffix(s, "ex") {
		return 0, false
	}
	f, err := strconv.ParseFloat(s[:len(s)-len("ex")], 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, false
	}
	return f, true
}

// cssLength is resolveLength for a property of an element with the text
// style ts. It resolves the unit ex with the font of the element, which
// resolveLength cannot do.
func (e *Engine) cssLength(v string, ts *textStyle, unitlessOK bool) (float64, bool) {
	if f, ok := exValue(v); ok {
		return float64(f * e.exSize(ts)), true
	}
	if css.IsMathFunction(v) {
		c, ok := e.evalCalc(v, ts)
		if !ok || c.IsNumber || c.HasPercent {
			return 0, false
		}
		return clampCalcPx(c.Px), true
	}
	return resolveLength(v, ts.size, unitlessOK)
}

func (e *Engine) chainOf(ts *textStyle) *font.Chain {
	if ts.chain == nil {
		ts.chain = e.Fonts.Resolve(ts.families, ts.weight, ts.italic)
	}
	return ts.chain
}

var fontSizeKeywords = map[string]float64{
	"xx-small": 9, "x-small": 10, "small": 13, "medium": 16, "large": 18, "x-large": 24, "xx-large": 32, "xxx-large": 48,
}

func resolveFontSize(v string, parentSize float64, unitlessOK bool) (float64, bool) {
	lower := strings.ToLower(strings.TrimSpace(v))
	if px, ok := fontSizeKeywords[lower]; ok {
		return px, true
	}
	switch lower {
	case "smaller":
		return parentSize / 1.2, true
	case "larger":
		return float64(parentSize * 1.2), true
	}
	if strings.HasSuffix(lower, "%") {
		f, err := strconv.ParseFloat(lower[:len(lower)-1], 64)
		return parentSize * f / 100, err == nil
	}
	return resolveLength(lower, parentSize, unitlessOK)
}

func resolveFontWeight(v string, parent int) int {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "normal":
		return 400
	case "bold":
		return 700
	case "bolder":
		switch {
		case parent < 350:
			return 400
		case parent < 550:
			return 700
		default:
			return 900
		}
	case "lighter":
		switch {
		case parent < 550:
			return 100
		case parent < 750:
			return 400
		default:
			return 700
		}
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && f >= 1 && f <= 1000 {
		return int(f)
	}
	return parent
}

// resolveLength converts a CSS length to px. emSize is the font size that em
// refers to. A unitless number is valid only where the caller allows it.
func resolveLength(v string, emSize float64, unitlessOK bool) (float64, bool) {
	s := strings.ToLower(strings.TrimSpace(v))
	if s == "" {
		return 0, false
	}
	if s[len(s)-1] == ')' && css.IsMathFunction(s) {
		// A caller that knows the font of the element uses cssLength: here
		// the unit ex has the fallback of unitLength.
		c, ok := css.EvalCalc(s, func(number float64, unit string) (float64, bool) {
			return unitLength(number, unit, emSize)
		})
		if !ok || c.IsNumber || c.HasPercent {
			return 0, false
		}
		return clampCalcPx(c.Px), true
	}
	end := len(s)
	for end > 0 && (s[end-1] < '0' || s[end-1] > '9') && s[end-1] != '.' {
		end--
	}
	num, unit := s[:end], s[end:]
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, false
	}
	if unit == "" {
		if unitlessOK || f == 0 {
			return f, true
		}
		return 0, false
	}
	return unitLength(f, unit, emSize)
}

// unitLength converts a number with a unit of length to px. emSize is the
// font size that em refers to. The unit is in lower case.
func unitLength(f float64, unit string, emSize float64) (float64, bool) {
	switch unit {
	case "px":
		return f, true
	case "em":
		return float64(f * emSize), true
	case "rem":
		return float64(f * 16), true
	case "ex":
		// The fallback of Blink for a font without an x-height. A caller
		// that knows the font of the element uses cssLength.
		return float64(f * emSize * 0.5), true
	case "ch":
		return float64(f * emSize * 0.5), true
	case "pt":
		return f * 96 / 72, true
	case "pc":
		return float64(f * 16), true
	case "in":
		return float64(f * 96), true
	case "cm":
		return f * 96 / 2.54, true
	case "mm":
		return f * 96 / 25.4, true
	case "q":
		return f * 96 / 101.6, true
	}
	return 0, false
}

// display returns the computed display of an element.
func (e *Engine) display(n *dom.Node) string {
	if v, ok := e.declared(n, "display"); ok {
		return strings.ToLower(strings.TrimSpace(v))
	}
	if n.NS == dom.NSHTML {
		switch n.Tag {
		case "div", "p", "body", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "pre", "blockquote",
			"center", "hr", "section", "article", "header", "footer", "address", "dl", "dt", "dd",
			"figure", "figcaption", "form", "fieldset", "nav", "aside", "main", "details", "summary":
			return "block"
		case "li":
			return "list-item"
		case "table":
			return "table"
		case "tr":
			return "table-row"
		case "td", "th":
			return "table-cell"
		case "tbody":
			return "table-row-group"
		case "thead":
			return "table-header-group"
		case "head", "script", "style", "title", "meta", "link", "template", "noscript":
			return "none"
		}
		return "inline"
	}
	return "inline"
}
