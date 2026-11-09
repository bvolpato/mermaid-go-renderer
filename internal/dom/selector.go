// This file ports no upstream file. It has CSS selectors for querySelector
// and for the style cascade, as the Selectors specification defines them.
// One comment names a rule of Blink for the letter case of a tag name
// (SelectorChecker::TagMatches). Chromium 156.0.8078.4 has this rule as
// MatchesTagName in third_party/blink/renderer/core/css/selector_checker.cc
// (LGPL-2.0-or-later). See NOTICE in this directory.

package dom

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// This file implements CSS selectors for querySelector and for the style
// cascade. It covers Selectors Level 3, plus :not(), :is() and :where() with
// selector lists. State pseudo-classes (:hover, :focus) parse and never
// match. Pseudo-elements parse and never match.

// Selector is one complex selector, for example `#a .b > c`.
type Selector struct {
	// compounds are in source order. combinators[i] joins compounds[i] and
	// compounds[i+1].
	compounds   []compound
	combinators []byte // ' ', '>', '+', '~'
	spec        Specificity
	never       bool // has a pseudo-element or a state pseudo-class
	deps        SelectorDeps
	// ancestorClass is a class that an ancestor of an element must have
	// for the selector to match the element, or "". See AncestorClass.
	ancestorClass string
}

// SelectorDeps says what a selector reads in the tree besides the element
// and its ancestors: their names, their attributes and their links. The
// style cascade keeps a result while everything that its selectors read is
// the same.
type SelectorDeps uint8

const (
	// DepsEarlierSiblings: the position of an element among the siblings
	// in front of it (:nth-child, :first-child, :nth-of-type,
	// :first-of-type).
	DepsEarlierSiblings SelectorDeps = 1 << iota
	// DepsSiblings: the siblings behind an element too (:nth-last-child,
	// :last-child, :only-child and the forms with of-type).
	DepsSiblings
	// DepsOther: more of the tree: the attributes of siblings (the
	// combinators + and ~), the children (:empty).
	DepsOther
)

// Deps returns what the selector reads besides the element and its
// ancestors.
func (s *Selector) Deps() SelectorDeps { return s.deps }

// AncestorClass returns a class name that an ancestor of an element must
// have for the selector to match the element, or "" when the selector names
// no such class. A caller that knows the classes of the ancestors can leave
// out a selector whose class is not among them, without a call of Match.
//
// The result is "" for a selector that tests the style attribute: Match
// reads that attribute, which creates it from the inline style, so such a
// selector must be tried.
func (s *Selector) AncestorClass() string { return s.ancestorClass }

// computeAncestorClass finds the class of AncestorClass: a class of a
// compound that a descendant or child combinator joins to the subject.
func (s *Selector) computeAncestorClass() string {
	if s.never || s.readsStyleAttr() {
		return ""
	}
	for i := len(s.compounds) - 2; i >= 0; i-- {
		if c := s.combinators[i]; c != ' ' && c != '>' {
			// The compound in front of a sibling combinator is a sibling.
			return ""
		}
		if classes := s.compounds[i].classes; len(classes) > 0 {
			return classes[0]
		}
	}
	return ""
}

// readsStyleAttr reports whether the selector has an attribute selector for
// the style attribute, also inside :not() and :is().
func (s *Selector) readsStyleAttr() bool {
	for i := range s.compounds {
		c := &s.compounds[i]
		for j := range c.attrs {
			if c.attrs[j].nameLower == "style" {
				return true
			}
		}
		for j := range c.pseudos {
			for _, inner := range c.pseudos[j].list {
				if inner.readsStyleAttr() {
					return true
				}
			}
		}
	}
	return false
}

// computeDeps finds the dependencies of a selector, with the selectors
// inside :not() and :is().
func (s *Selector) computeDeps() SelectorDeps {
	if s.never {
		// The selector matches nothing.
		return 0
	}
	var d SelectorDeps
	for _, c := range s.combinators {
		if c == '+' || c == '~' {
			d |= DepsOther
		}
	}
	for i := range s.compounds {
		for j := range s.compounds[i].pseudos {
			ps := &s.compounds[i].pseudos[j]
			switch ps.kind {
			case pseudoNthChild, pseudoNthOfType:
				d |= DepsEarlierSiblings
			case pseudoNthLastChild, pseudoNthLastOfType, pseudoOnlyChild, pseudoOnlyOfType:
				d |= DepsSiblings
			case pseudoEmpty:
				if ps.a != -1 {
					d |= DepsOther
				}
			case pseudoNot, pseudoIs:
				for _, inner := range ps.list {
					d |= inner.computeDeps()
				}
			}
		}
	}
	return d
}

// Specificity is (ids, classes, types).
type Specificity [3]int

// Less orders specificities.
func (s Specificity) Less(o Specificity) bool {
	for i := 0; i < 3; i++ {
		if s[i] != o[i] {
			return s[i] < o[i]
		}
	}
	return false
}

type compound struct {
	tag      string // "" means any element
	tagLower string
	id       string
	classes  []string
	attrs    []attrSel
	pseudos  []pseudo
}

type attrSel struct {
	name      string
	nameLower string
	op        byte // 0 presence, '=', '~', '|', '^', '$', '*'
	val       string
	ci        bool
}

type pseudoKind uint8

const (
	pseudoNthChild pseudoKind = iota + 1
	pseudoNthLastChild
	pseudoNthOfType
	pseudoNthLastOfType
	pseudoOnlyChild
	pseudoOnlyOfType
	pseudoEmpty
	pseudoRoot
	pseudoNot
	pseudoIs // also :where and :matches
)

type pseudo struct {
	kind pseudoKind
	a, b int
	list []*Selector
}

// SelectorList is a comma-separated list of selectors.
type SelectorList []*Selector

var selectorCache sync.Map // string -> selectorCacheEntry
var selectorCacheSize int64
var selectorCacheMu sync.Mutex

type selectorCacheEntry struct {
	list SelectorList
	err  error
}

// ParseSelector parses a selector list. Results are cached.
func ParseSelector(s string) (SelectorList, error) {
	if e, ok := selectorCache.Load(s); ok {
		ent := e.(selectorCacheEntry)
		return ent.list, ent.err
	}
	p := &selParser{s: s}
	list, err := p.parseList(false)
	if err == nil && p.i < len(p.s) {
		err = fmt.Errorf("selector %q: unexpected %q", s, p.s[p.i:])
	}
	if err != nil {
		list = nil
	}
	selectorCacheMu.Lock()
	// The key can contain element ids that are different for every diagram.
	// Drop the cache when it grows, so that a long-running service is safe.
	if selectorCacheSize >= 8192 {
		selectorCache.Range(func(k, _ any) bool { selectorCache.Delete(k); return true })
		selectorCacheSize = 0
	}
	selectorCacheSize++
	selectorCacheMu.Unlock()
	selectorCache.Store(s, selectorCacheEntry{list, err})
	return list, err
}

type selParser struct {
	s string
	i int
}

func (p *selParser) skipSpace() bool {
	start := p.i
	for p.i < len(p.s) && isASCIISpace(rune(p.s[p.i])) {
		p.i++
	}
	return p.i > start
}

func (p *selParser) parseList(nested bool) (SelectorList, error) {
	var list SelectorList
	for {
		p.skipSpace()
		sel, err := p.parseComplex(nested)
		if err != nil {
			return nil, err
		}
		sel.deps = sel.computeDeps()
		sel.ancestorClass = sel.computeAncestorClass()
		list = append(list, sel)
		p.skipSpace()
		if p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
			continue
		}
		return list, nil
	}
}

func (p *selParser) parseComplex(nested bool) (*Selector, error) {
	sel := &Selector{}
	for {
		c, err := p.parseCompound(sel)
		if err != nil {
			return nil, err
		}
		sel.compounds = append(sel.compounds, c)
		hadSpace := p.skipSpace()
		if p.i >= len(p.s) || p.s[p.i] == ',' || (nested && p.s[p.i] == ')') {
			break
		}
		comb := byte(' ')
		switch p.s[p.i] {
		case '>', '+', '~':
			comb = p.s[p.i]
			p.i++
			p.skipSpace()
		default:
			if !hadSpace {
				return nil, fmt.Errorf("selector %q: unexpected %q", p.s, p.s[p.i:])
			}
		}
		sel.combinators = append(sel.combinators, comb)
	}
	for i := range sel.compounds {
		c := &sel.compounds[i]
		if c.id != "" {
			sel.spec[0]++
		}
		sel.spec[1] += len(c.classes) + len(c.attrs)
		if c.tag != "" {
			sel.spec[2]++
		}
		for _, ps := range c.pseudos {
			switch ps.kind {
			case pseudoNot, pseudoIs:
				var max Specificity
				for _, s := range ps.list {
					if max.Less(s.spec) {
						max = s.spec
					}
				}
				if ps.a == 0 { // a == 1 marks :where, which has no specificity
					for k := 0; k < 3; k++ {
						sel.spec[k] += max[k]
					}
				}
			default:
				sel.spec[1]++
			}
		}
	}
	return sel, nil
}

func isNameChar(c byte) bool {
	return c == '-' || c == '_' || c >= 0x80 || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func (p *selParser) parseName() string {
	var sb strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '\\' && p.i+1 < len(p.s) {
			// A CSS escape: \HHHHHH with an optional space, or \c.
			j := p.i + 1
			k := j
			for k < len(p.s) && k-j < 6 && isHexDigit(p.s[k]) {
				k++
			}
			if k > j {
				v, _ := strconv.ParseUint(p.s[j:k], 16, 32)
				sb.WriteRune(rune(v))
				if k < len(p.s) && p.s[k] == ' ' {
					k++
				}
				p.i = k
			} else {
				sb.WriteByte(p.s[j])
				p.i = j + 1
			}
			continue
		}
		if !isNameChar(c) {
			break
		}
		sb.WriteByte(c)
		p.i++
	}
	return sb.String()
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func (p *selParser) parseCompound(sel *Selector) (compound, error) {
	var c compound
	start := p.i
	if p.i < len(p.s) && p.s[p.i] == '*' {
		p.i++
	} else if p.i < len(p.s) && (isNameChar(p.s[p.i]) || p.s[p.i] == '\\') {
		c.tag = p.parseName()
		c.tagLower = strings.ToLower(c.tag)
	}
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case '#':
			p.i++
			c.id = p.parseName()
			if c.id == "" {
				return c, fmt.Errorf("selector %q: empty id", p.s)
			}
		case '.':
			p.i++
			name := p.parseName()
			if name == "" {
				return c, fmt.Errorf("selector %q: empty class", p.s)
			}
			c.classes = append(c.classes, name)
		case '[':
			a, err := p.parseAttr()
			if err != nil {
				return c, err
			}
			c.attrs = append(c.attrs, a)
		case ':':
			if err := p.parsePseudo(&c, sel); err != nil {
				return c, err
			}
		default:
			if p.i == start {
				return c, fmt.Errorf("selector %q: unexpected %q", p.s, p.s[p.i:])
			}
			return c, nil
		}
	}
	if p.i == start {
		return c, fmt.Errorf("selector %q: empty compound", p.s)
	}
	return c, nil
}

func (p *selParser) parseAttr() (attrSel, error) {
	var a attrSel
	p.i++ // [
	p.skipSpace()
	a.name = p.parseName()
	a.nameLower = strings.ToLower(a.name)
	if a.name == "" {
		return a, fmt.Errorf("selector %q: bad attribute selector", p.s)
	}
	p.skipSpace()
	if p.i < len(p.s) && p.s[p.i] == ']' {
		p.i++
		return a, nil
	}
	if p.i < len(p.s) && p.s[p.i] == '=' {
		a.op = '='
		p.i++
	} else if p.i+1 < len(p.s) && p.s[p.i+1] == '=' && strings.IndexByte("~|^$*", p.s[p.i]) >= 0 {
		a.op = p.s[p.i]
		p.i += 2
	} else {
		return a, fmt.Errorf("selector %q: bad attribute operator", p.s)
	}
	p.skipSpace()
	if p.i < len(p.s) && (p.s[p.i] == '"' || p.s[p.i] == '\'') {
		q := p.s[p.i]
		p.i++
		var sb strings.Builder
		for p.i < len(p.s) && p.s[p.i] != q {
			if p.s[p.i] == '\\' && p.i+1 < len(p.s) {
				p.i++
			}
			sb.WriteByte(p.s[p.i])
			p.i++
		}
		if p.i >= len(p.s) {
			return a, fmt.Errorf("selector %q: unterminated string", p.s)
		}
		p.i++
		a.val = sb.String()
	} else {
		a.val = p.parseName()
	}
	p.skipSpace()
	if p.i < len(p.s) && (p.s[p.i] == 'i' || p.s[p.i] == 'I') {
		a.ci = true
		p.i++
		p.skipSpace()
	} else if p.i < len(p.s) && (p.s[p.i] == 's' || p.s[p.i] == 'S') {
		p.i++
		p.skipSpace()
	}
	if p.i >= len(p.s) || p.s[p.i] != ']' {
		return a, fmt.Errorf("selector %q: missing ]", p.s)
	}
	p.i++
	return a, nil
}

func (p *selParser) parsePseudo(c *compound, sel *Selector) error {
	p.i++ // :
	if p.i < len(p.s) && p.s[p.i] == ':' {
		// A pseudo-element. It never matches a real element.
		p.i++
		p.parseName()
		if p.i < len(p.s) && p.s[p.i] == '(' {
			p.skipParens()
		}
		sel.never = true
		return nil
	}
	name := strings.ToLower(p.parseName())
	arg := ""
	hasArg := false
	if p.i < len(p.s) && p.s[p.i] == '(' {
		hasArg = true
		switch name {
		case "not", "is", "where", "matches", "-webkit-any":
			p.i++
			list, err := p.parseList(true)
			if err != nil {
				return err
			}
			p.skipSpace()
			if p.i >= len(p.s) || p.s[p.i] != ')' {
				return fmt.Errorf("selector %q: missing )", p.s)
			}
			p.i++
			ps := pseudo{kind: pseudoIs, list: list}
			if name == "not" {
				ps.kind = pseudoNot
			}
			if name == "where" {
				ps.a = 1
			}
			c.pseudos = append(c.pseudos, ps)
			return nil
		}
		start := p.i + 1
		p.skipParens()
		arg = strings.TrimSpace(p.s[start : p.i-1])
	}
	nth := func(kind pseudoKind) error {
		a, b, err := parseNth(arg)
		if err != nil {
			return fmt.Errorf("selector %q: %v", p.s, err)
		}
		c.pseudos = append(c.pseudos, pseudo{kind: kind, a: a, b: b})
		return nil
	}
	switch name {
	case "first-child":
		c.pseudos = append(c.pseudos, pseudo{kind: pseudoNthChild, a: 0, b: 1})
	case "last-child":
		c.pseudos = append(c.pseudos, pseudo{kind: pseudoNthLastChild, a: 0, b: 1})
	case "first-of-type":
		c.pseudos = append(c.pseudos, pseudo{kind: pseudoNthOfType, a: 0, b: 1})
	case "last-of-type":
		c.pseudos = append(c.pseudos, pseudo{kind: pseudoNthLastOfType, a: 0, b: 1})
	case "only-child":
		c.pseudos = append(c.pseudos, pseudo{kind: pseudoOnlyChild})
	case "only-of-type":
		c.pseudos = append(c.pseudos, pseudo{kind: pseudoOnlyOfType})
	case "empty":
		c.pseudos = append(c.pseudos, pseudo{kind: pseudoEmpty})
	case "root":
		c.pseudos = append(c.pseudos, pseudo{kind: pseudoRoot})
	case "nth-child":
		return nth(pseudoNthChild)
	case "nth-last-child":
		return nth(pseudoNthLastChild)
	case "nth-of-type":
		return nth(pseudoNthOfType)
	case "nth-last-of-type":
		return nth(pseudoNthLastOfType)
	case "before", "after", "first-line", "first-letter":
		sel.never = true
	default:
		// A state or unknown pseudo-class (:hover, :focus, :has(...)). It
		// never matches in a static render.
		_ = hasArg
		sel.never = true
		c.pseudos = append(c.pseudos, pseudo{kind: pseudoEmpty, a: -1})
	}
	return nil
}

func (p *selParser) skipParens() {
	depth := 0
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				p.i++
				return
			}
		}
		p.i++
	}
}

// parseNth parses the An+B microsyntax.
func parseNth(s string) (a, b int, err error) {
	s = strings.ToLower(strings.ReplaceAll(s, " ", ""))
	switch s {
	case "odd":
		return 2, 1, nil
	case "even":
		return 2, 0, nil
	}
	n := strings.IndexByte(s, 'n')
	if n < 0 {
		b, err = strconv.Atoi(s)
		return 0, b, err
	}
	switch s[:n] {
	case "", "+":
		a = 1
	case "-":
		a = -1
	default:
		if a, err = strconv.Atoi(s[:n]); err != nil {
			return 0, 0, err
		}
	}
	if rest := s[n+1:]; rest != "" {
		if b, err = strconv.Atoi(rest); err != nil {
			return 0, 0, err
		}
	}
	return a, b, nil
}

// ---- matching ----

// Specificity returns the specificity of the selector.
func (s *Selector) Specificity() Specificity { return s.spec }

// Key returns a property of the rightmost compound that an element must
// have to match: kind is '#' (id), '.' (class), 't' (tag) or 0 (none).
// The style cascade uses it to index rules.
func (s *Selector) Key() (kind byte, name string) {
	c := &s.compounds[len(s.compounds)-1]
	switch {
	case c.id != "":
		return '#', c.id
	case len(c.classes) > 0:
		return '.', c.classes[0]
	case c.tag != "":
		return 't', c.tagLower
	}
	return 0, ""
}

// Match reports whether the element matches the selector.
func (s *Selector) Match(n *Node) bool {
	if s.never || n == nil || n.Type != ElementNode {
		return false
	}
	return s.matchFrom(n, len(s.compounds)-1)
}

func (s *Selector) matchFrom(n *Node, idx int) bool {
	if !matchCompound(n, &s.compounds[idx]) {
		return false
	}
	if idx == 0 {
		return true
	}
	switch s.combinators[idx-1] {
	case '>':
		p := n.Parent
		return p != nil && p.Type == ElementNode && s.matchFrom(p, idx-1)
	case ' ':
		for p := n.Parent; p != nil && p.Type == ElementNode; p = p.Parent {
			if s.matchFrom(p, idx-1) {
				return true
			}
		}
	case '+':
		if prev := prevElement(n); prev != nil {
			return s.matchFrom(prev, idx-1)
		}
	case '~':
		for prev := prevElement(n); prev != nil; prev = prevElement(prev) {
			if s.matchFrom(prev, idx-1) {
				return true
			}
		}
	}
	return false
}

func prevElement(n *Node) *Node {
	for p := n.Prev; p != nil; p = p.Prev {
		if p.Type == ElementNode {
			return p
		}
	}
	return nil
}

func nextElement(n *Node) *Node {
	for p := n.Next; p != nil; p = p.Next {
		if p.Type == ElementNode {
			return p
		}
	}
	return nil
}

// equalFoldASCII reports whether a and b are equal when the ASCII letters
// are compared without regard to case.
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 'a' - 'A'
		}
		if y >= 'A' && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

func matchCompound(n *Node, c *compound) bool {
	if c.tag != "" {
		// A type selector is case-insensitive in an HTML document. For an
		// HTML element Blink compares the lowercase selector name. For an
		// element of another namespace, whose name can be camel case, it
		// compares the ASCII uppercase forms (SelectorChecker::TagMatches),
		// so "foreignobject" matches <foreignObject>.
		if n.NS == NSHTML {
			if n.Tag != c.tagLower {
				return false
			}
		} else if n.Tag != c.tag && !equalFoldASCII(n.Tag, c.tag) {
			return false
		}
	}
	if c.id != "" && n.ID() != c.id {
		return false
	}
	for _, cl := range c.classes {
		if !n.HasClass(cl) {
			return false
		}
	}
	for i := range c.attrs {
		if !matchAttr(n, &c.attrs[i]) {
			return false
		}
	}
	for i := range c.pseudos {
		if !matchPseudo(n, &c.pseudos[i]) {
			return false
		}
	}
	return true
}

func matchAttr(n *Node, a *attrSel) bool {
	name := a.name
	if n.NS == NSHTML {
		name = a.nameLower
	}
	if name == "style" {
		// Matching an attribute selector reads the attribute: Blink then
		// creates the style attribute from the CSSOM state.
		n.syncStyle()
	}
	val, ok := "", false
	for i := range n.Attrs {
		if n.Attrs[i].QName() == name {
			val, ok = n.Attrs[i].Value, true
			break
		}
	}
	if !ok {
		return false
	}
	want := a.val
	if a.ci {
		val, want = strings.ToLower(val), strings.ToLower(want)
	}
	switch a.op {
	case 0:
		return true
	case '=':
		return val == want
	case '~':
		if want == "" || strings.IndexFunc(want, isASCIISpace) >= 0 {
			return false
		}
		for _, f := range strings.FieldsFunc(val, isASCIISpace) {
			if f == want {
				return true
			}
		}
		return false
	case '|':
		return val == want || strings.HasPrefix(val, want+"-")
	case '^':
		return want != "" && strings.HasPrefix(val, want)
	case '$':
		return want != "" && strings.HasSuffix(val, want)
	case '*':
		return want != "" && strings.Contains(val, want)
	}
	return false
}

func matchPseudo(n *Node, ps *pseudo) bool {
	switch ps.kind {
	case pseudoNthChild, pseudoNthLastChild, pseudoNthOfType, pseudoNthLastOfType:
		ofType := ps.kind == pseudoNthOfType || ps.kind == pseudoNthLastOfType
		fromEnd := ps.kind == pseudoNthLastChild || ps.kind == pseudoNthLastOfType
		if n.Parent == nil {
			return false
		}
		pos := 1
		step := prevElement
		if fromEnd {
			step = nextElement
		}
		for s := step(n); s != nil; s = step(s) {
			if !ofType || (s.Tag == n.Tag && s.NS == n.NS) {
				pos++
			}
		}
		if ps.a == 0 {
			return pos == ps.b
		}
		d := pos - ps.b
		return d%ps.a == 0 && d/ps.a >= 0
	case pseudoOnlyChild:
		return n.Parent != nil && prevElement(n) == nil && nextElement(n) == nil
	case pseudoOnlyOfType:
		if n.Parent == nil {
			return false
		}
		for s := n.Parent.FirstChild; s != nil; s = s.Next {
			if s != n && s.Type == ElementNode && s.Tag == n.Tag && s.NS == n.NS {
				return false
			}
		}
		return true
	case pseudoEmpty:
		if ps.a == -1 {
			return false // unknown pseudo-class
		}
		for c := n.FirstChild; c != nil; c = c.Next {
			if c.Type == ElementNode || (c.Type == TextNode && c.Data != "") {
				return false
			}
		}
		return true
	case pseudoRoot:
		// The document element. A diagram is built under <body>, so the root
		// is the body's parent, which this DOM does not model. A body with no
		// parent stands in for it.
		return n.Parent == nil
	case pseudoNot:
		for _, s := range ps.list {
			if s.Match(n) {
				return false
			}
		}
		return true
	case pseudoIs:
		for _, s := range ps.list {
			if s.Match(n) {
				return true
			}
		}
		return false
	}
	return false
}

// Match reports whether the element matches any selector of the list.
func (l SelectorList) Match(n *Node) bool {
	for _, s := range l {
		if s.Match(n) {
			return true
		}
	}
	return false
}

// Matches is Element.matches(selector). An invalid selector matches nothing.
func (n *Node) Matches(selector string) bool {
	list, err := ParseSelector(selector)
	return err == nil && list.Match(n)
}

// Select is selection.select(selector), that is querySelector: the first
// descendant in document order that matches. It returns nil for no match.
func (n *Node) Select(selector string) *Node {
	if n == nil {
		return nil
	}
	list, err := ParseSelector(selector)
	if err != nil {
		return nil
	}
	var found *Node
	n.walkElements(func(e *Node) bool {
		if list.Match(e) {
			found = e
			return false
		}
		return true
	})
	return found
}

// Nodes is a list of nodes, as selection.selectAll returns.
type Nodes []*Node

// SelectAll is selection.selectAll(selector), that is querySelectorAll.
func (n *Node) SelectAll(selector string) Nodes {
	if n == nil {
		return nil
	}
	list, err := ParseSelector(selector)
	if err != nil {
		return nil
	}
	var out Nodes
	n.walkElements(func(e *Node) bool {
		if list.Match(e) {
			out = append(out, e)
		}
		return true
	})
	return out
}

// walkElements visits the element descendants of n in document order until
// fn returns false.
func (n *Node) walkElements(fn func(*Node) bool) bool {
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type != ElementNode {
			continue
		}
		if !fn(c) || !c.walkElements(fn) {
			return false
		}
	}
	return true
}

// Closest is Element.closest(selector).
func (n *Node) Closest(selector string) *Node {
	list, err := ParseSelector(selector)
	if err != nil {
		return nil
	}
	for e := n; e != nil && e.Type == ElementNode; e = e.Parent {
		if list.Match(e) {
			return e
		}
	}
	return nil
}

// Attr sets an attribute on every node.
func (ns Nodes) Attr(name string, value any) Nodes {
	for _, n := range ns {
		n.Attr(name, value)
	}
	return ns
}

// Style sets a style property on every node.
func (ns Nodes) Style(property string, value any) Nodes {
	for _, n := range ns {
		n.Style(property, value)
	}
	return ns
}

// Classed sets or removes classes on every node.
func (ns Nodes) Classed(names string, on bool) Nodes {
	for _, n := range ns {
		n.Classed(names, on)
	}
	return ns
}

// Remove detaches every node.
func (ns Nodes) Remove() {
	for _, n := range ns {
		n.Remove()
	}
}
