// Package purify is a port of DOMPurify 3.4.12, the version that mermaid
// 12.2.0 bundles. It sanitizes a string of HTML as DOMPurify.sanitize does in
// Chromium, and returns the same string.
//
// The result depends on the HTML parser. dom.ParseHTMLDocument builds the
// tree of Chromium for the SVG that mermaid writes and for common label
// markup. For some unusual markup the parser of golang.org/x/net/html builds
// another tree, and the result then differs from Chromium. TestGoldenParse of
// internal/dom lists those cases by cause.
//
// Ported from dompurify 3.4.12, src/purify.ts (MPL-2.0 OR Apache-2.0).
// DOMPurify gives a choice of the two licenses. The owner of this repository
// has not chosen which of them the port uses (finding F5 of
// docs/LICENSE-AUDIT.md). The files of this package are changed versions of
// the files of DOMPurify: they are Go translations. NOTICE in this directory
// has the copyright line, and the two license texts are next to it.
//
// The port covers the path of sanitize for a string input with the text/html
// parser. It does not cover: a DOM node as input (IN_PLACE), the
// application/xhtml+xml parser, a NAMESPACE other than HTML, Trusted Types,
// and the list DOMPurify.removed.
//
// A Purifier is one DOMPurify instance. It holds the hooks and the parsed
// configuration, as the closure of createDOMPurify does. It is not safe for
// concurrent use. Each render must use its own Purifier.
package purify

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

// Version is the version of DOMPurify that this package ports.
const Version = "3.4.12"

// ErrUnsupported reports a configuration that this port does not implement.
var ErrUnsupported = errors.New("purify: unsupported configuration")

// TypeError is an error that upstream throws as a JavaScript TypeError.
type TypeError struct{ Message string }

func (e *TypeError) Error() string { return "TypeError: " + e.Message }

const sanitizeNamedPropsPrefix = "user-content-"

// whatToShow of _createNodeIterator. A parsed HTML document has no
// processing instruction and no CDATA section.
const iteratorShow = dom.ShowElement | dom.ShowComment | dom.ShowText

// Purifier is a DOMPurify instance.
type Purifier struct {
	hooks  map[HookPoint][]hookEntry
	nextID HookID

	// The variables of the closure of createDOMPurify that _parseConfig sets.
	settings

	// SET_CONFIG and the sets that setConfig saved.
	setConfig            bool
	setConfigAllowedTags set
	setConfigAllowedAttr set
	setConfigTagsShared  bool
	setConfigAttrShared  bool

	// iterators are the active NodeIterators. A browser updates them when a
	// node is removed. Here forceRemove does it.
	iterators []*dom.NodeIterator
}

// settings has the variables that _parseConfig sets from a configuration.
// They are a function of the configuration alone, so parseConfig keeps the
// settings of a configuration (see parsedConfigs). The sets of kept settings
// are shared and are not changed: a Purifier copies a set before it lets a
// hook change it.
type settings struct {
	allowedTags set
	allowedAttr set
	// tagsShared and attrShared are set while allowedTags and allowedAttr are
	// sets that other Purifiers share and none may change.
	tagsShared                  bool
	attrShared                  bool
	customElementHandling       CustomElementHandling
	forbidTags                  set
	forbidAttr                  set
	extraTagCheck               func(tagName string) bool
	extraAttrCheck              func(attributeName, tagName string) bool
	allowAriaAttr               bool
	allowDataAttr               bool
	allowUnknownProtocols       bool
	allowSelfCloseInAttr        bool
	safeForTemplates            bool
	safeForXML                  bool
	wholeDocument               bool
	forceBody                   bool
	returnDOM                   bool
	returnDOMFragment           bool
	sanitizeDOM                 bool
	sanitizeNamedProps          bool
	keepContent                 bool
	forbidContents              set
	dataURITags                 set
	uriSafeAttributes           set
	allowedURI                  func(value string) bool
	allowedNamespaces           set
	mathMLTextIntegrationPoints set
	htmlIntegrationPoints       set
	config                      *Config
}

// New returns a DOMPurify instance with no hooks, as createDOMPurify() does.
func New() *Purifier {
	return &Purifier{hooks: map[HookPoint][]hookEntry{}}
}

// Sanitize is DOMPurify.sanitize(dirty, cfg) on a new instance that has no
// hooks. See Purifier.Sanitize.
func Sanitize(dirty string, cfg Config) string {
	return New().Sanitize(dirty, cfg)
}

// Sanitize is DOMPurify.sanitize(dirty, cfg) for a string, with RETURN_DOM
// false. It returns the sanitized HTML. When upstream would throw, or the
// markup or the configuration is outside of what this port supports, it
// returns the empty string. SanitizeErr reports the cause.
func (p *Purifier) Sanitize(dirty string, cfg Config) string {
	out, err := p.SanitizeErr(dirty, cfg)
	if err != nil {
		return ""
	}
	return out
}

// SanitizeErr is Sanitize with the error.
func (p *Purifier) SanitizeErr(dirty string, cfg Config) (string, error) {
	cfg.ReturnDOM, cfg.ReturnDOMFragment = false, false
	out, _, err := p.sanitize(dirty, &cfg)
	return out, err
}

// SanitizeDOM is DOMPurify.sanitize(dirty, cfg) with RETURN_DOM true. It
// returns the body element of the sanitized document (the html element with
// WHOLE_DOCUMENT), or nil for a document that has no body. With
// RETURN_DOM_FRAGMENT upstream moves the children of the body to a fragment.
// This function returns the body for both options.
func (p *Purifier) SanitizeDOM(dirty string, cfg Config) (*dom.Node, error) {
	cfg.ReturnDOM = true
	_, node, err := p.sanitize(dirty, &cfg)
	return node, err
}

// SetConfig is DOMPurify.setConfig: the configuration stays in effect for
// every later call of Sanitize, which then ignores its own argument.
func (p *Purifier) SetConfig(cfg Config) error {
	if err := p.parseConfig(&cfg); err != nil {
		return err
	}
	p.setConfig = true
	p.setConfigAllowedTags = p.allowedTags
	p.setConfigAllowedAttr = p.allowedAttr
	p.setConfigTagsShared = p.tagsShared
	p.setConfigAttrShared = p.attrShared
	return nil
}

// ClearConfig is DOMPurify.clearConfig.
func (p *Purifier) ClearConfig() {
	p.config = nil
	p.setConfig = false
	p.setConfigAllowedTags = nil
	p.setConfigAllowedAttr = nil
}

// IsValidAttribute is DOMPurify.isValidAttribute.
func (p *Purifier) IsValidAttribute(tag, attr, value string) bool {
	if p.config == nil {
		if err := p.parseConfig(&Config{}); err != nil {
			return false
		}
	}
	return p.isValidAttribute(toLowerCase(tag), toLowerCase(attr), value)
}

// resolveSetOption is _resolveSetOption. base is nil when upstream starts
// from an empty set.
func resolveSetOption(values []string, fallback set, lower bool, base set) set {
	if values == nil {
		return fallback
	}
	var s set
	if base != nil {
		s = base.clone()
	} else {
		s = make(set, len(values))
	}
	addToSet(s, values, lower)
	return s
}

// addToSet is addToSet of utils.ts.
func addToSet(s set, values []string, lower bool) {
	for l := len(values) - 1; l >= 0; l-- {
		element := values[l]
		if lower {
			element = toLowerCase(element)
		}
		s[element] = true
	}
}

// parsedConfigs has the settings of the configurations that were parsed, by
// the content of the configuration (see Config.appendKey). A render parses
// the same few configurations for every label and for the final SVG.
var parsedConfigs = memo.New[*settings](64)

// parseConfig is _parseConfig.
func (p *Purifier) parseConfig(cfg *Config) error {
	var buf [256]byte
	key, hasKey := cfg.appendKey(buf[:0])
	if hasKey {
		if kept, ok := parsedConfigs.GetBytes(key); ok {
			p.settings = *kept
			if len(p.hooks) > 0 {
				// A hook gets the configuration and can change it.
				c := *cfg
				p.config = &c
			}
			return nil
		}
	}
	if err := p.parseConfigSlow(cfg); err != nil {
		return err
	}
	if hasKey && memo.Enabled() {
		// The sets are shared from now on.
		p.tagsShared, p.attrShared = true, true
		kept := p.settings
		c := *cfg
		kept.config = &c
		parsedConfigs.Put(string(key), &kept)
	}
	return nil
}

// parseConfigSlow is _parseConfig without the kept settings.
func (p *Purifier) parseConfigSlow(cfg *Config) error {
	if cfg.ParserMediaType == "application/xhtml+xml" {
		return fmt.Errorf("%w: PARSER_MEDIA_TYPE application/xhtml+xml", ErrUnsupported)
	}
	if cfg.Namespace != "" && cfg.Namespace != htmlNamespace {
		return fmt.Errorf("%w: NAMESPACE %q", ErrUnsupported, cfg.Namespace)
	}
	// transformCaseFunc is stringToLowerCase for text/html.
	const lower = true

	p.allowedTags = resolveSetOption(cfg.AllowedTags, defaultAllowedTags, lower, nil)
	p.tagsShared = cfg.AllowedTags == nil
	p.allowedAttr = resolveSetOption(cfg.AllowedAttr, defaultAllowedAttr, lower, nil)
	p.attrShared = cfg.AllowedAttr == nil
	p.allowedNamespaces = resolveSetOption(cfg.AllowedNamespaces, defaultAllowedNamespaces, false, nil)
	p.uriSafeAttributes = resolveSetOption(cfg.AddURISafeAttr, defaultURISafeAttributes, lower, defaultURISafeAttributes)
	p.dataURITags = resolveSetOption(cfg.AddDataURITags, defaultDataURITags, lower, defaultDataURITags)
	p.forbidContents = resolveSetOption(cfg.ForbidContents, defaultForbidContents, lower, nil)
	p.forbidTags = resolveSetOption(cfg.ForbidTags, set{}, lower, nil)
	p.forbidAttr = resolveSetOption(cfg.ForbidAttr, set{}, lower, nil)
	useProfiles := cfg.UseProfiles
	p.allowAriaAttr = cfg.AllowAriaAttr != False
	p.allowDataAttr = cfg.AllowDataAttr != False
	p.allowUnknownProtocols = cfg.AllowUnknownProtocols
	p.allowSelfCloseInAttr = cfg.AllowSelfCloseInAttr != False
	p.safeForTemplates = cfg.SafeForTemplates
	p.safeForXML = cfg.SafeForXML != False
	p.wholeDocument = cfg.WholeDocument
	p.returnDOM = cfg.ReturnDOM
	p.returnDOMFragment = cfg.ReturnDOMFragment
	p.forceBody = cfg.ForceBody
	p.sanitizeDOM = cfg.SanitizeDOM != False
	p.sanitizeNamedProps = cfg.SanitizeNamedProps
	p.keepContent = cfg.KeepContent != False
	p.allowedURI = cfg.AllowedURI
	if p.allowedURI == nil {
		p.allowedURI = matchAllowedURI
	}
	if cfg.MathMLTextIntegrationPoints != nil {
		p.mathMLTextIntegrationPoints = set(cfg.MathMLTextIntegrationPoints).clone()
	} else {
		p.mathMLTextIntegrationPoints = defaultMathMLTextIntegrationPoints
	}
	if cfg.HTMLIntegrationPoints != nil {
		p.htmlIntegrationPoints = set(cfg.HTMLIntegrationPoints).clone()
	} else {
		p.htmlIntegrationPoints = defaultHTMLIntegrationPoints
	}
	p.customElementHandling = CustomElementHandling{}
	if cfg.CustomElementHandling != nil {
		p.customElementHandling = *cfg.CustomElementHandling
	}

	if p.safeForTemplates {
		p.allowDataAttr = false
	}
	if p.returnDOMFragment {
		p.returnDOM = true
	}

	// Parse profile info.
	if useProfiles != nil {
		p.allowedTags = newSet(textTags)
		p.allowedAttr = set{}
		p.tagsShared, p.attrShared = false, false
		if useProfiles.HTML {
			addToSet(p.allowedTags, htmlTags, true)
			addToSet(p.allowedAttr, htmlAttrs, true)
		}
		if useProfiles.SVG {
			addToSet(p.allowedTags, svgTags, true)
			addToSet(p.allowedAttr, svgAttrs, true)
			addToSet(p.allowedAttr, xmlAttrs, true)
		}
		if useProfiles.SVGFilters {
			addToSet(p.allowedTags, svgFilterTags, true)
			addToSet(p.allowedAttr, svgAttrs, true)
			addToSet(p.allowedAttr, xmlAttrs, true)
		}
		if useProfiles.MathML {
			addToSet(p.allowedTags, mathMLTags, true)
			addToSet(p.allowedAttr, mathMLAttrs, true)
			addToSet(p.allowedAttr, xmlAttrs, true)
		}
	}

	// Merge configuration parameters.
	p.extraTagCheck = nil
	p.extraAttrCheck = nil
	if cfg.AddTagsFunc != nil {
		p.extraTagCheck = cfg.AddTagsFunc
	} else if cfg.AddTags != nil {
		p.ownAllowedTags()
		addToSet(p.allowedTags, cfg.AddTags, lower)
	}
	if cfg.AddAttrFunc != nil {
		p.extraAttrCheck = cfg.AddAttrFunc
	} else if cfg.AddAttr != nil {
		p.ownAllowedAttr()
		addToSet(p.allowedAttr, cfg.AddAttr, lower)
	}
	if cfg.AddURISafeAttr != nil {
		addToSet(p.uriSafeAttributes, cfg.AddURISafeAttr, lower)
	}
	if cfg.ForbidContents != nil {
		addToSet(p.forbidContents, cfg.ForbidContents, lower)
	}
	if cfg.AddForbidContents != nil {
		if cfg.ForbidContents == nil {
			p.forbidContents = p.forbidContents.clone()
		}
		addToSet(p.forbidContents, cfg.AddForbidContents, lower)
	}

	// Add #text in case KEEP_CONTENT is set to true.
	if p.keepContent && !p.allowedTags["#text"] {
		p.ownAllowedTags()
		p.allowedTags["#text"] = true
	}
	// Add html, head and body to ALLOWED_TAGS in case WHOLE_DOCUMENT is true.
	if p.wholeDocument && !(p.allowedTags["html"] && p.allowedTags["head"] && p.allowedTags["body"]) {
		p.ownAllowedTags()
		addToSet(p.allowedTags, []string{"html", "head", "body"}, true)
	}
	// Add tbody to ALLOWED_TAGS in case tables are permitted.
	if p.allowedTags["table"] {
		if !p.allowedTags["tbody"] {
			p.ownAllowedTags()
			p.allowedTags["tbody"] = true
		}
		delete(p.forbidTags, "tbody")
	}

	c := *cfg
	p.config = &c
	return nil
}

// ownAllowedTags makes ALLOWED_TAGS a private copy when it is the shared
// default set. Upstream changes DEFAULT_ALLOWED_TAGS in place in a few
// branches, always with names that the set already has. The copy keeps the
// shared set unchanged in every case.
func (p *Purifier) ownAllowedTags() {
	if p.tagsShared {
		p.allowedTags = p.allowedTags.clone()
		p.tagsShared = false
	}
}

func (p *Purifier) ownAllowedAttr() {
	if p.attrShared {
		p.allowedAttr = p.allowedAttr.clone()
		p.attrShared = false
	}
}

// ---- tree helpers ----
//
// The children of an HTML template element are its contents. They are not
// child nodes in a browser, so the helpers below hide them.

func nodeName(n *dom.Node) string {
	switch n.Type {
	case dom.ElementNode:
		return n.Tag
	case dom.TextNode:
		return "#text"
	case dom.CommentNode:
		return "#comment"
	}
	return "#document"
}

func firstChild(n *dom.Node) *dom.Node {
	if n.IsTemplate() {
		return nil
	}
	return n.FirstChild
}

func hasChildNodes(n *dom.Node) bool { return firstChild(n) != nil }

func firstElementChild(n *dom.Node) *dom.Node {
	for c := firstChild(n); c != nil; c = c.Next {
		if c.Type == dom.ElementNode {
			return c
		}
	}
	return nil
}

// textContent is Node.textContent.
func textContent(n *dom.Node) string {
	switch n.Type {
	case dom.TextNode, dom.CommentNode:
		return n.Data
	}
	c := firstChild(n)
	if c == nil {
		return ""
	}
	if c.Next == nil && c.Type == dom.TextNode {
		return c.Data
	}
	var sb strings.Builder
	var walk func(*dom.Node)
	walk = func(p *dom.Node) {
		for c := firstChild(p); c != nil; c = c.Next {
			switch c.Type {
			case dom.TextNode:
				sb.WriteString(c.Data)
			case dom.ElementNode:
				walk(c)
			}
		}
	}
	walk(n)
	return sb.String()
}

// innerHTML is Element.innerHTML in the document that DOMParser made.
func innerHTML(n *dom.Node) string {
	if n.Type != dom.ElementNode {
		return ""
	}
	return n.InnerHTMLInert()
}

// ---- namespace checks ----

// parentInfo is the parent that _checkValidNamespace sees.
type parentInfo struct {
	namespaceURI string
	tagName      string // in lowercase
}

// checkSvgNamespace is _checkSvgNamespace.
func (p *Purifier) checkSvgNamespace(tagName string, parent parentInfo) bool {
	// The only way to switch from HTML namespace to SVG is via <svg>. If it
	// happens via any other tag, then it should be killed.
	if parent.namespaceURI == htmlNamespace {
		return tagName == "svg"
	}
	// The only way to switch from MathML to SVG is via <svg> if the parent is
	// either <annotation-xml> or a MathML text integration point.
	if parent.namespaceURI == mathMLNamespace {
		return tagName == "svg" && (parent.tagName == "annotation-xml" || p.mathMLTextIntegrationPoints[parent.tagName])
	}
	// We only allow elements that are defined in the SVG spec.
	return allSVGTags[tagName]
}

// checkMathMlNamespace is _checkMathMlNamespace.
func (p *Purifier) checkMathMlNamespace(tagName string, parent parentInfo) bool {
	if parent.namespaceURI == htmlNamespace {
		return tagName == "math"
	}
	if parent.namespaceURI == svgNamespace {
		return tagName == "math" && p.htmlIntegrationPoints[parent.tagName]
	}
	return allMathMLTags[tagName]
}

// checkHtmlNamespace is _checkHtmlNamespace.
func (p *Purifier) checkHtmlNamespace(tagName string, parent parentInfo) bool {
	if parent.namespaceURI == svgNamespace && !p.htmlIntegrationPoints[parent.tagName] {
		return false
	}
	if parent.namespaceURI == mathMLNamespace && !p.mathMLTextIntegrationPoints[parent.tagName] {
		return false
	}
	return !allMathMLTags[tagName] && (commonSVGAndHTMLElements[tagName] || !allSVGTags[tagName])
}

// checkValidNamespace is _checkValidNamespace.
func (p *Purifier) checkValidNamespace(element *dom.Node) bool {
	// In JSDOM, if we're inside shadow DOM, then parentNode can be null. We
	// need to emulate the behavior then. The content fragment of a template
	// has no tagName, so the same fallback applies to its children: here the
	// parent of such a child is the template element, which has the same
	// namespace and tag name as the fallback.
	parent := parentInfo{namespaceURI: htmlNamespace, tagName: "template"}
	if pn := element.Parent; pn != nil && pn.Type == dom.ElementNode {
		parent = parentInfo{namespaceURI: pn.NS, tagName: toLowerCase(pn.Tag)}
	}
	tagName := toLowerCase(element.Tag)
	if !p.allowedNamespaces[element.NS] {
		return false
	}
	switch element.NS {
	case svgNamespace:
		return p.checkSvgNamespace(tagName, parent)
	case mathMLNamespace:
		return p.checkMathMlNamespace(tagName, parent)
	case htmlNamespace:
		return p.checkHtmlNamespace(tagName, parent)
	}
	// The code should never reach this place (this means that the element
	// somehow got a namespace that is not HTML, SVG, MathML or allowed via
	// ALLOWED_NAMESPACES). Return false just in case.
	return false
}

// ---- removal ----

// forceRemove is _forceRemove. It returns the TypeError that upstream throws
// for a node that has no parent.
func (p *Purifier) forceRemove(node *dom.Node) error {
	if node == nil || node.Parent == nil {
		return &TypeError{"a node selected for removal could not be detached from its tree and cannot be safely returned; refusing to sanitize in place"}
	}
	for _, it := range p.iterators {
		it.PreRemove(node)
	}
	node.Remove()
	return nil
}

// RemoveNode detaches a node from its parent, as node.remove() does in a
// hook. A hook must use it and not dom.Node.Remove, so that the walk of the
// sanitizer continues at the right node.
func (p *Purifier) RemoveNode(node *dom.Node) {
	if node == nil || node.Parent == nil {
		return
	}
	for _, it := range p.iterators {
		it.PreRemove(node)
	}
	node.Remove()
}

// abort carries an error out of the walk, as a JavaScript exception does.
type abort struct{ err error }

func (p *Purifier) mustRemove(node *dom.Node) {
	if err := p.forceRemove(node); err != nil {
		panic(abort{err})
	}
}

// removeAttribute is _removeAttribute.
func (p *Purifier) removeAttribute(name string, element *dom.Node) {
	element.RemoveAttribute(name)
	// We void attribute values for unremovable "is" attributes.
	if name == "is" {
		if p.returnDOM || p.returnDOMFragment {
			_ = p.forceRemove(element)
		} else {
			_ = element.SetAttribute(name, "")
		}
	}
}

// ---- document ----

// initDocument is _initDocument. It returns the node that sanitize walks: the
// body element, or the html element for WHOLE_DOCUMENT. The result is nil for
// a document that has no body.
func (p *Purifier) initDocument(dirty string) (*dom.HTMLDocument, *dom.Node, error) {
	leading := ""
	if p.forceBody {
		dirty = "<remove></remove>" + dirty
	} else {
		// If FORCE_BODY isn't used, leading whitespace needs to be preserved manually.
		leading = leadingWhitespace.FindString(dirty)
	}
	doc, err := dom.ParseHTMLDocument(dirty)
	if err != nil {
		return nil, nil, err
	}
	// body = doc.body || doc.documentElement. The body of a frameset document
	// is the frameset element.
	body := doc.Body
	if body == nil {
		body = doc.Frameset
	}
	if body == nil {
		body = doc.Root
	}
	if dirty != "" && leading != "" {
		// The text node is not parsed: a carriage return stays as it is.
		body.InsertBefore(doc.Document.CreateText(leading), body.FirstChild)
	}
	if p.wholeDocument {
		return doc, doc.Root, nil
	}
	return doc, doc.Body, nil
}

// stripTemplateExpressions is _stripTemplateExpressions.
func stripTemplateExpressions(value string) string {
	value = mustacheExpr.ReplaceAllLiteralString(value, " ")
	value = erbExpr.ReplaceAllLiteralString(value, " ")
	value = tmplitExpr.ReplaceAllLiteralString(value, " ")
	return value
}

// scrubTemplateExpressions is _scrubTemplateExpressions: the template scrub
// of a returned DOM tree. It joins adjacent text nodes first, as
// Node.normalize does, and then scrubs every text node.
func scrubTemplateExpressions(node *dom.Node) {
	var walk func(parent *dom.Node)
	walk = func(parent *dom.Node) {
		for c := parent.FirstChild; c != nil; {
			next := c.Next
			switch c.Type {
			case dom.TextNode:
				for next != nil && next.Type == dom.TextNode {
					c.Data += next.Data
					after := next.Next
					next.Remove()
					next = after
				}
				if c.Data == "" {
					c.Remove()
				} else {
					c.Data = stripTemplateExpressions(c.Data)
				}
			case dom.CommentNode:
				c.Data = stripTemplateExpressions(c.Data)
			case dom.ElementNode:
				walk(c)
			}
			c = next
		}
	}
	walk(node)
}

// ---- elements ----

// isUnsafeNode is _isUnsafeNode.
func (p *Purifier) isUnsafeNode(currentNode *dom.Node, tagName string) bool {
	// Detect mXSS attempts abusing namespace confusion.
	if p.safeForXML && hasChildNodes(currentNode) && !hasFirstElementChild(currentNode) &&
		matchElementMarkupProbe(textContent(currentNode)) &&
		matchElementMarkupProbe(readInnerHTML(currentNode)) {
		return true
	}
	// Remove risky CSS construction leading to mXSS.
	if p.safeForXML && currentNode.Type == dom.ElementNode && currentNode.NS == htmlNamespace &&
		tagName == "style" && firstElementChild(currentNode) != nil {
		return true
	}
	// Remove any occurrence of processing instructions: a parsed HTML document
	// has none.
	// Remove any kind of possibly harmful comments.
	if p.safeForXML && currentNode.Type == dom.CommentNode && commentMarkupProbe.MatchString(currentNode.Data) {
		return true
	}
	return false
}

// hasFirstElementChild is _isNode(currentNode.firstElementChild). A form
// returns a control named firstElementChild in place of the DOM property.
// One control is a node. More than one are a RadioNodeList, which is not.
func hasFirstElementChild(n *dom.Node) bool {
	if isHTMLForm(n) {
		switch formNamed(n, "firstElementChild", 2) {
		case 1:
			return true
		case 2:
			return false
		}
	}
	return firstElementChild(n) != nil
}

// readInnerHTML is the string that upstream gets from currentNode.innerHTML.
// A form returns a control named innerHTML in place of the DOM property, and
// the control converts to a string like "[object HTMLInputElement]".
func readInnerHTML(n *dom.Node) string {
	if isHTMLForm(n) && formNamed(n, "innerHTML", 1) > 0 {
		return "[object HTMLElement]"
	}
	return innerHTML(n)
}

// isBasicCustomElement is _isBasicCustomElement.
func isBasicCustomElement(tagName string) bool {
	return !reservedCustomElementNames[toLowerCase(tagName)] && customElement.MatchString(tagName)
}

// sanitizeDisallowedNode is _sanitizeDisallowedNode. It returns false when
// the node is kept as a custom element.
func (p *Purifier) sanitizeDisallowedNode(currentNode *dom.Node, tagName string) bool {
	// Check if we have a custom element to handle.
	if !p.forbidTags[tagName] && isBasicCustomElement(tagName) {
		if check := p.customElementHandling.TagNameCheck; check != nil && check(tagName) {
			return false
		}
	}
	// Keep content except for bad-listed elements.
	if p.keepContent && !p.forbidContents[tagName] {
		parentNode := currentNode.Parent
		if parentNode != nil {
			var childNodes []*dom.Node
			for c := firstChild(currentNode); c != nil; c = c.Next {
				childNodes = append(childNodes, c)
			}
			for i := len(childNodes) - 1; i >= 0; i-- {
				hoisted := childNodes[i].CloneNode(true)
				parentNode.InsertBefore(hoisted, currentNode.Next)
			}
		}
	}
	p.mustRemove(currentNode)
	return true
}

// sanitizeElements is _sanitizeElements. It returns true when the node was
// removed.
func (p *Purifier) sanitizeElements(currentNode, root *dom.Node) bool {
	// Execute a hook if present.
	p.executeHooks(BeforeSanitizeElements, currentNode, nil)
	// A hook may have detached the node.
	if currentNode != root && currentNode.Parent == nil {
		return true
	}
	// Check if element is clobbered or can clobber.
	if p.isClobbered(currentNode) {
		p.mustRemove(currentNode)
		return true
	}
	// Now let's check the element's type and name.
	tagName := toLowerCase(nodeName(currentNode))
	// Execute a hook if present.
	if len(p.hooks[UponSanitizeElement]) > 0 {
		p.executeHooks(UponSanitizeElement, currentNode, &HookEvent{TagName: tagName, AllowedTags: p.allowedTags})
	}
	if currentNode != root && currentNode.Parent == nil {
		return true
	}
	// Detect mXSS attempts abusing namespace confusion.
	if p.isUnsafeNode(currentNode, tagName) {
		p.mustRemove(currentNode)
		return true
	}
	// Remove element if anything forbids its presence.
	if p.forbidTags[tagName] || (!(p.extraTagCheck != nil && p.extraTagCheck(tagName)) && !p.allowedTags[tagName]) {
		removed := p.sanitizeDisallowedNode(currentNode, tagName)
		if !removed {
			p.executeHooks(AfterSanitizeElements, currentNode, nil)
		}
		return removed
	}
	// Check whether element has a valid namespace.
	if currentNode.Type == dom.ElementNode && !p.checkValidNamespace(currentNode) {
		p.mustRemove(currentNode)
		return true
	}
	// Make sure that older browsers don't get fallback-tag mXSS.
	if (tagName == "noscript" || tagName == "noembed" || tagName == "noframes") &&
		fallbackTagClose.MatchString(innerHTML(currentNode)) {
		p.mustRemove(currentNode)
		return true
	}
	// Sanitize element content to be template-safe.
	if p.safeForTemplates && currentNode.Type == dom.TextNode {
		if content := stripTemplateExpressions(currentNode.Data); currentNode.Data != content {
			currentNode.Data = content
		}
	}
	// Execute a hook if present.
	p.executeHooks(AfterSanitizeElements, currentNode, nil)
	return false
}

// ---- attributes ----

// isValidAttribute is _isValidAttribute.
func (p *Purifier) isValidAttribute(lcTag, lcName, value string) bool {
	// FORBID_ATTR must always win, even if ADD_ATTR predicate would allow it.
	if p.forbidAttr[lcName] {
		return false
	}
	if p.safeForXML && lcName == "patchsrc" {
		return false
	}
	if p.safeForXML && lcName == "for" && lcTag != "label" && lcTag != "output" {
		return false
	}
	// Make sure attribute cannot clobber.
	if p.sanitizeDOM && (lcName == "id" || lcName == "name") && clobberingNames[value] {
		return false
	}
	nameIsPermitted := p.allowedAttr[lcName] || (p.extraAttrCheck != nil && p.extraAttrCheck(lcName, lcTag))
	// Allow valid data-* attributes: at least one character after "-". We
	// don't need to check the value; it's always URI safe.
	if p.allowDataAttr && matchDataAttr(lcName) {
		// This attribute is safe.
	} else if p.allowAriaAttr && matchAriaAttr(lcName) {
		// This attribute is safe.
	} else if !nameIsPermitted {
		ceh := &p.customElementHandling
		switch {
		case isBasicCustomElement(lcTag) &&
			ceh.TagNameCheck != nil && ceh.TagNameCheck(lcTag) &&
			ceh.AttributeNameCheck != nil && ceh.AttributeNameCheck(lcName, lcTag):
			// A custom element that the configuration accepts.
		case lcName == "is" && ceh.AllowCustomizedBuiltInElements &&
			ceh.TagNameCheck != nil && ceh.TagNameCheck(value):
			// An is attribute whose value the configuration accepts.
		default:
			return false
		}
	} else if p.uriSafeAttributes[lcName] {
		// This attribute is safe.
	} else if p.allowedURI(stripAttrWhitespace(value)) {
		// This attribute is safe.
	} else if (lcName == "src" || lcName == "xlink:href" || lcName == "href") && lcTag != "script" &&
		strings.HasPrefix(value, "data:") && p.dataURITags[lcTag] {
		// This attribute is safe.
	} else if p.allowUnknownProtocols && !isScriptOrData.MatchString(stripAttrWhitespace(value)) {
		// This attribute is safe.
	} else if value != "" {
		return false
	}
	return true
}

// setAttributeValue is _setAttributeValue.
func (p *Purifier) setAttributeValue(currentNode *dom.Node, name, namespaceURI, value string) {
	var err error
	if namespaceURI != "" {
		err = currentNode.SetAttributeNS(namespaceURI, name, value)
	} else {
		// Fallback to setAttribute() for browser-unrecognized namespaces.
		err = currentNode.SetAttribute(name, value)
	}
	if err == nil && p.isClobbered(currentNode) {
		err = p.forceRemove(currentNode)
	}
	if err != nil {
		p.removeAttribute(name, currentNode)
	}
}

// sanitizeAttributes is _sanitizeAttributes.
func (p *Purifier) sanitizeAttributes(currentNode *dom.Node) {
	// Execute a hook if present.
	p.executeHooks(BeforeSanitizeAttributes, currentNode, nil)
	// Check if we have attributes; if not we might have a text node.
	if currentNode.Type != dom.ElementNode || p.isClobbered(currentNode) {
		return
	}
	hookEvent := &HookEvent{KeepAttr: true, AllowedAttributes: p.allowedAttr}
	hasAttrHooks := len(p.hooks[UponSanitizeAttribute]) > 0
	l := len(currentNode.Attrs)
	lcTag := toLowerCase(nodeName(currentNode))
	// Go backwards over all attributes; safely remove bad ones.
	for l > 0 {
		l--
		if l >= len(currentNode.Attrs) {
			// attributes[l] is undefined: upstream reads a property of it.
			panic(abort{&TypeError{"Cannot read properties of undefined (reading 'name')"}})
		}
		attr := currentNode.Attrs[l]
		name, namespaceURI, attrValue := attr.QName(), attr.NS, attr.Value
		lcName := toLowerCase(name)
		initValue := attrValue
		value := initValue
		if name != "value" {
			value = trimSpace(initValue)
		}
		// Execute a hook if present.
		hookEvent.AttrName = lcName
		hookEvent.AttrValue = value
		hookEvent.KeepAttr = true
		hookEvent.ForceKeepAttr = false
		if hasAttrHooks {
			p.executeHooks(UponSanitizeAttribute, currentNode, hookEvent)
		}
		value = hookEvent.AttrValue
		// Full DOM Clobbering protection via namespace isolation: prefix id
		// and name attributes with user-content-.
		if p.sanitizeNamedProps && (lcName == "id" || lcName == "name") &&
			!strings.HasPrefix(value, sanitizeNamedPropsPrefix) {
			// Remove the attribute with this value.
			p.removeAttribute(name, currentNode)
			// Prefix the value and later re-create the attribute with the sanitized value.
			value = sanitizeNamedPropsPrefix + value
		}
		// Work around a security issue with comments inside attributes.
		if p.safeForXML && matchAttrValueProbe(value) {
			p.removeAttribute(name, currentNode)
			continue
		}
		// Make sure we cannot easily use animated hrefs, even if animations are allowed.
		if lcName == "attributename" && strings.Contains(value, "href") {
			p.removeAttribute(name, currentNode)
			continue
		}
		// Did the hooks approve of the attribute?
		if hookEvent.ForceKeepAttr {
			continue
		}
		// Did the hooks approve of the attribute?
		if !hookEvent.KeepAttr {
			p.removeAttribute(name, currentNode)
			continue
		}
		// Work around a security issue in jQuery 3.0.
		if !p.allowSelfCloseInAttr && strings.Contains(value, "/>") {
			p.removeAttribute(name, currentNode)
			continue
		}
		// Sanitize attribute content to be template-safe.
		if p.safeForTemplates {
			value = stripTemplateExpressions(value)
		}
		// Is `value` valid for this attribute?
		if !p.isValidAttribute(lcTag, lcName, value) {
			p.removeAttribute(name, currentNode)
			continue
		}
		// Handle attributes that require Trusted Types: this port has no
		// policy, and a policy does not change the string.
		// Handle invalid data-* attribute set by try-catching it.
		if value != initValue {
			p.setAttributeValue(currentNode, name, namespaceURI, value)
		}
	}
	// Execute a hook if present.
	p.executeHooks(AfterSanitizeAttributes, currentNode, nil)
}

// sanitizeShadowDOM is _sanitizeShadowDOM for the contents of a template
// element. A parsed HTML document has no other shadow tree.
func (p *Purifier) sanitizeShadowDOM(template *dom.Node) {
	shadowIterator := dom.NewContentIterator(template, iteratorShow)
	p.iterators = append(p.iterators, shadowIterator)
	defer func() { p.iterators = p.iterators[:len(p.iterators)-1] }()
	// Execute a hook if present.
	p.executeHooks(BeforeSanitizeShadowDOM, template, nil)
	for shadowNode := shadowIterator.NextNode(); shadowNode != nil; shadowNode = shadowIterator.NextNode() {
		// Execute a hook if present.
		p.executeHooks(UponSanitizeShadowNode, shadowNode, nil)
		// Sanitize tags and elements.
		p.sanitizeElements(shadowNode, template)
		// Check attributes next.
		p.sanitizeAttributes(shadowNode)
		// Deep shadow DOM detected.
		if shadowNode.IsTemplate() {
			p.sanitizeShadowDOM(shadowNode)
		}
	}
	// Execute a hook if present.
	p.executeHooks(AfterSanitizeShadowDOM, template, nil)
}

// sanitize is DOMPurify.sanitize for a string. The second result is the
// sanitized root when RETURN_DOM is set.
func (p *Purifier) sanitize(dirty string, cfg *Config) (out string, node *dom.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			a, ok := r.(abort)
			if !ok {
				panic(r)
			}
			out, node, err = "", nil, a.err
		}
	}()
	// Make sure we have a string to sanitize. DO NOT return early, as this
	// will return the wrong type if the user has requested a DOM object
	// rather than a string.
	if dirty == "" {
		dirty = "<!-->"
	}
	// Assign config vars.
	if p.setConfig {
		p.allowedTags, p.tagsShared = p.setConfigAllowedTags, p.setConfigTagsShared
		p.allowedAttr, p.attrShared = p.setConfigAllowedAttr, p.setConfigAttrShared
	} else if err := p.parseConfig(cfg); err != nil {
		return "", nil, err
	}
	// Clone the sets so that hooks cannot change the shared ones.
	if len(p.hooks[UponSanitizeElement]) > 0 || len(p.hooks[UponSanitizeAttribute]) > 0 {
		p.allowedTags, p.tagsShared = p.allowedTags.clone(), false
	}
	if len(p.hooks[UponSanitizeAttribute]) > 0 {
		p.allowedAttr, p.attrShared = p.allowedAttr.clone(), false
	}

	// Exit directly if we have nothing to do.
	if !p.returnDOM && !p.safeForTemplates && !p.wholeDocument && !strings.Contains(dirty, "<") {
		return dirty, nil, nil
	}
	// Initialize the document to work on.
	doc, body, err := p.initDocument(dirty)
	if err != nil {
		return "", nil, err
	}
	// Check we have a DOM node from the data.
	if body == nil {
		return "", nil, nil
	}
	// Remove first element node (ours) if FORCE_BODY is set.
	if p.forceBody {
		p.mustRemove(body.FirstChild)
	}

	// Get node iterator.
	nodeIterator := dom.NewNodeIterator(body, iteratorShow)
	p.iterators = append(p.iterators[:0], nodeIterator)
	defer func() { p.iterators = p.iterators[:0] }()
	// Now start iterating over the created document.
	for currentNode := nodeIterator.NextNode(); currentNode != nil; currentNode = nodeIterator.NextNode() {
		// Sanitize tags and elements.
		p.sanitizeElements(currentNode, body)
		// Check attributes next.
		p.sanitizeAttributes(currentNode)
		// Shadow DOM detected, sanitize it.
		if currentNode.IsTemplate() {
			p.sanitizeShadowDOM(currentNode)
		}
	}

	// Return sanitized string or DOM.
	if p.returnDOM {
		if p.safeForTemplates {
			scrubTemplateExpressions(body)
		}
		return "", body, nil
	}
	var serializedHTML string
	if p.wholeDocument {
		serializedHTML = body.OuterHTMLInert()
	} else {
		serializedHTML = body.InnerHTMLInert()
	}
	// Serialize doctype if allowed.
	if p.wholeDocument && p.allowedTags["!doctype"] && doc.HasDoctype && doc.DoctypeName != "" &&
		doctypeName.MatchString(doc.DoctypeName) {
		serializedHTML = "<!DOCTYPE " + doc.DoctypeName + ">\n" + serializedHTML
	}
	// Sanitize final string template-safe.
	if p.safeForTemplates {
		serializedHTML = stripTemplateExpressions(serializedHTML)
	}
	return serializedHTML, nil, nil
}
