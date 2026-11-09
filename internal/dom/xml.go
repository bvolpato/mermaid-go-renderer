// Ported from Chromium, third_party/blink/renderer/core/editing/serializers:
// markup_accumulator.cc and markup_formatter.cc, in XML mode (BSD-2-Clause).
// The port does not record the version of Chromium. Chromium 156.0.8078.4,
// the reference browser, has the two files. At that version they have the
// BSD terms in the file, with these copyright lines:
//
//	Copyright (C) 2004, 2005, 2006, 2007, 2008, 2009, 2012 Apple Inc. All rights
//	reserved.
//	Copyright (C) 2009, 2010 Google Inc. All rights reserved.
//
// NOTICE-Blink.txt in this directory has the headers of the two files, with
// the conditions and the disclaimer. This file is a Go translation.

package dom

import (
	"strconv"
	"strings"
)

// This file implements XMLSerializer.serializeToString as Chromium does. It
// follows the XML serialization algorithm of the DOM Parsing and
// Serialization specification (https://w3c.github.io/DOM-Parsing/), with the
// differences of Chromium. The comments quote the step numbers that the
// source of Chromium quotes.
//
// The result is a well-formed XML document for the trees that mermaid
// makes, where the HTML serialization (InnerHTML) is not: an element without
// children is closed in its start tag, every namespace has a declaration,
// and U+00A0 is written as the character, not as &nbsp;.
//
// A Node has no prefix: CreateElementNS drops it, and the HTML parser makes
// no element with a prefix. So the steps of the algorithm that read the
// prefix of an element see an empty prefix here.

// XML is new XMLSerializer().serializeToString(n).
func (n *Node) XML() string {
	if n == nil {
		return ""
	}
	// SerializeNodes. Step 3: the prefix map starts with the XML namespace.
	// Step 4: the generated namespace prefix index starts at 1.
	a := &markupAccumulator{prefixIndex: 1}
	root := &namespaceContext{}
	root.add("xml", NSXML)
	a.stack = append(a.stack, root)
	a.serializeNodesWithNamespaces(n)
	return a.markup.String()
}

// markupAccumulator is MarkupAccumulator with SerializationType::kXML.
type markupAccumulator struct {
	markup      strings.Builder
	stack       []*namespaceContext
	prefixIndex int
}

// namespaceContext is MarkupAccumulator::NamespaceContext: the namespace
// prefix map and the context namespace of one element. Chromium copies the
// whole context for every element. Here an element shares the maps of its
// parent until it adds a prefix.
type namespaceContext struct {
	// prefixNS maps a prefix to its namespace.
	prefixNS map[string]string
	// nsPrefixes maps a namespace to its prefixes, in the order of Add.
	nsPrefixes map[string][]string
	// shared is true while the maps belong to the context of the parent.
	shared bool
	// contextNS is the context namespace. "" is the null namespace.
	contextNS string
}

func (c *namespaceContext) push() *namespaceContext {
	return &namespaceContext{prefixNS: c.prefixNS, nsPrefixes: c.nsPrefixes, shared: true, contextNS: c.contextNS}
}

// add is NamespaceContext::Add.
func (c *namespaceContext) add(prefix, namespace string) {
	if c.shared || c.prefixNS == nil {
		prefixNS := make(map[string]string, len(c.prefixNS)+1)
		for k, v := range c.prefixNS {
			prefixNS[k] = v
		}
		nsPrefixes := make(map[string][]string, len(c.nsPrefixes)+1)
		for k, v := range c.nsPrefixes {
			nsPrefixes[k] = v
		}
		c.prefixNS, c.nsPrefixes, c.shared = prefixNS, nsPrefixes, false
	}
	c.prefixNS[prefix] = namespace
	// A list can belong to the context of the parent: append to a copy.
	list := c.nsPrefixes[namespace]
	c.nsPrefixes[namespace] = append(list[:len(list):len(list)], prefix)
}

// recordNamespaceInformation is NamespaceContext::RecordNamespaceInformation.
// It returns the value of the default namespace declaration of the element.
// has is false when the element has no such attribute.
//
// Chromium does not look at the namespace of the attribute: the xmlns
// attribute of an HTML element has no namespace in an HTML document.
func (c *namespaceContext) recordNamespaceInformation(n *Node) (localDefault string, has bool) {
	for i := range n.Attrs {
		attr := &n.Attrs[i]
		if attr.Prefix == "" && attr.Local == "xmlns" {
			localDefault, has = attr.Value, true
		} else if attr.Prefix == "xmlns" {
			c.add(attr.Local, attr.Value)
		}
	}
	return localDefault, has
}

// lookupNamespaceURI is NamespaceContext::LookupNamespaceURI. ok is false
// for a prefix that the map does not have.
func (c *namespaceContext) lookupNamespaceURI(prefix string) (namespace string, ok bool) {
	namespace, ok = c.prefixNS[prefix]
	return namespace, ok
}

// inheritLocalDefaultNamespace is
// NamespaceContext::InheritLocalDefaultNamespace.
func (c *namespaceContext) inheritLocalDefaultNamespace(localDefault string, has bool) {
	if !has {
		return
	}
	c.contextNS = localDefault
}

func (a *markupAccumulator) context() *namespaceContext { return a.stack[len(a.stack)-1] }

// serializeNodesWithNamespaces is
// MarkupAccumulator::SerializeNodesWithNamespaces with kIncludeNode.
func (a *markupAccumulator) serializeNodesWithNamespaces(n *Node) {
	if n.Type != ElementNode {
		a.appendStartMarkup(n)
		for c := n.FirstChild; c != nil; c = c.Next {
			a.serializeNodesWithNamespaces(c)
		}
		return
	}
	// PushNamespaces.
	a.stack = append(a.stack, a.context().push())
	prefixOverride := a.appendElement(n)
	if !shouldSelfClose(n) {
		for c := n.FirstChild; c != nil; c = c.Next {
			a.serializeNodesWithNamespaces(c)
		}
		// AppendEndTag.
		a.markup.WriteString("</")
		if prefixOverride != "" {
			a.markup.WriteString(prefixOverride)
			a.markup.WriteByte(':')
		}
		a.markup.WriteString(n.Tag)
		a.markup.WriteByte('>')
	}
	// PopNamespaces.
	a.stack = a.stack[:len(a.stack)-1]
}

// shouldSelfClose is MarkupFormatter::ShouldSelfClose in XML mode: an
// element without children has no end tag, except an HTML element that can
// have one.
func shouldSelfClose(n *Node) bool {
	if n.FirstChild != nil {
		return false
	}
	if n.NS == NSHTML && !voidElements[n.Tag] {
		return false
	}
	return true
}

// appendStartMarkup is MarkupAccumulator::AppendStartMarkup for the nodes
// that are not elements.
func (a *markupAccumulator) appendStartMarkup(n *Node) {
	switch n.Type {
	case TextNode:
		// MarkupFormatter::AppendText with kEntityMaskInPCDATA. Text in a
		// style or script element is escaped as other text is.
		appendXMLText(&a.markup, n.Data)
	case CommentNode:
		a.markup.WriteString("<!--")
		a.markup.WriteString(n.Data)
		a.markup.WriteString("-->")
	}
}

// appendElement is MarkupAccumulator::AppendElement. It returns the prefix
// that the start tag has, which the end tag repeats.
func (a *markupAccumulator) appendElement(n *Node) string {
	// Element::Attributes synchronizes the style attribute.
	n.syncStyle()
	prefix, ignoreNamespaceDefinitionAttribute := a.appendStartTagOpen(n)
	for i := range n.Attrs {
		attr := &n.Attrs[i]
		if ignoreNamespaceDefinitionAttribute && attr.NS == NSXMLNS && attr.Prefix == "" {
			// Drop xmlns= only if it is inconsistent with the namespace of
			// the element. https://github.com/w3c/DOM-Parsing/issues/47
			if attr.Value != n.NS {
				continue
			}
		}
		a.appendAttributeAsXMLWithNamespace(attr)
	}
	// MarkupFormatter::AppendStartTagClose.
	if shouldSelfClose(n) {
		if n.NS == NSHTML {
			// XHTML 1.0 <-> HTML compatibility.
			a.markup.WriteByte(' ')
		}
		a.markup.WriteByte('/')
	}
	a.markup.WriteByte('>')
	return prefix
}

// appendStartTagOpen is MarkupAccumulator::AppendStartTagOpen: "XML
// serializing an element node" up to the attributes.
func (a *markupAccumulator) appendStartTagOpen(n *Node) (serializedPrefix string, ignoreNamespaceDefinitionAttribute bool) {
	ctx := a.context()
	// 8. Let local default namespace be the result of recording the
	// namespace information for node given map and local prefixes map.
	localDefault, hasLocalDefault := ctx.recordNamespaceInformation(n)
	// 9. Let inherited ns be a copy of namespace.
	inheritedNS := ctx.contextNS
	// 10. Let ns be the value of node's namespaceURI attribute.
	ns := n.NS

	// 11. If inherited ns is equal to ns, then:
	if inheritedNS == ns {
		// 11.1. If local default namespace is not null, then set ignore
		// namespace definition attribute to true.
		// 11.3. Append to qualified name the value of node's localName.
		a.appendTagName("", n.Tag)
		return "", hasLocalDefault
	}

	// 12. Otherwise, inherited ns is not equal to ns.
	// 12.2. Let candidate prefix be the result of retrieving a preferred
	// prefix string prefix from map given namespace ns.
	candidatePrefix, hasCandidate := "", false
	if ns != "" && !(hasLocalDefault && ns == localDefault) {
		candidatePrefix, hasCandidate = a.retrievePreferredPrefixString(ns, "")
	}
	// 12.4. If candidate prefix is not null (a namespace prefix is defined
	// which maps to ns), then:
	if hasCandidate {
		if _, ok := ctx.lookupNamespaceURI(candidatePrefix); ok {
			// 12.4.1. Append to qualified name the concatenation of
			// candidate prefix, ":", and node's localName.
			a.appendTagName(candidatePrefix, n.Tag)
			// 12.4.2. If the local default namespace is not null and its
			// value is not the XML namespace, then let inherited ns get the
			// value of local default namespace.
			if !(hasLocalDefault && localDefault == NSXML) {
				ctx.inheritLocalDefaultNamespace(localDefault, hasLocalDefault)
			}
			return candidatePrefix, false
		}
	}

	// 12.5 is for a node that has a prefix.

	// 12.6. Otherwise, if local default namespace is null, or local default
	// namespace is not null and its value is not equal to ns, then:
	if !hasLocalDefault || localDefault != ns {
		// 12.6.1. Set the ignore namespace definition attribute flag to true.
		// 12.6.3. Let the value of inherited ns be ns.
		ctx.contextNS = ns
		// 12.6.4. Append the value of qualified name to markup.
		a.appendTagName("", n.Tag)
		// 12.6.5. Append a default namespace declaration for ns.
		appendXMLAttribute(&a.markup, "", "xmlns", ns)
		return "", true
	}

	// 12.7. Otherwise, the node has a local default namespace that matches
	// ns. Let the value of inherited ns be ns.
	ctx.contextNS = ns
	a.appendTagName("", n.Tag)
	return "", false
}

func (a *markupAccumulator) appendTagName(prefix, local string) {
	a.markup.WriteByte('<')
	if prefix != "" {
		a.markup.WriteString(prefix)
		a.markup.WriteByte(':')
	}
	a.markup.WriteString(local)
}

// appendAttributeAsXMLWithNamespace is
// MarkupAccumulator::AppendAttributeAsXMLWithNamespace: "serializing an
// element's attributes" for one attribute.
func (a *markupAccumulator) appendAttributeAsXMLWithNamespace(attr *Attr) {
	// 3.3. Let attribute namespace be the value of attr's namespaceURI.
	attributeNamespace := attr.NS
	if attributeNamespace == "" {
		appendXMLAttribute(&a.markup, "", attr.Local, attr.Value)
		return
	}
	// 3.5.1. Let candidate prefix be the result of retrieving a preferred
	// prefix string from map given namespace attribute namespace with
	// preferred prefix being attr's prefix value.
	candidatePrefix, hasCandidate := a.retrievePreferredPrefixString(attributeNamespace, attr.Prefix)
	if attributeNamespace == NSXMLNS {
		// 3.5.2. The attribute namespace is the XMLNS namespace.
		if attr.Prefix == "" && attr.Local != "xmlns" {
			candidatePrefix, hasCandidate = "xmlns", true
		}
	} else if a.shouldAddNamespaceAttribute(attr, candidatePrefix, hasCandidate) {
		// 3.5.3. The attribute namespace is not the XMLNS namespace.
		_, inUse := a.context().lookupNamespaceURI(candidatePrefix)
		if !hasCandidate || inUse {
			// 3.5.3.1. Let candidate prefix be the result of generating a
			// prefix providing map, attribute namespace, and prefix index.
			candidatePrefix = a.generatePrefix(attributeNamespace)
			// 3.5.3.2. Append the declaration of the prefix.
			appendXMLAttribute(&a.markup, "xmlns", candidatePrefix, attributeNamespace)
		} else {
			a.appendNamespace(candidatePrefix, attributeNamespace)
		}
	}
	appendXMLAttribute(&a.markup, candidatePrefix, attr.Local, attr.Value)
}

// shouldAddNamespaceAttribute is
// MarkupAccumulator::ShouldAddNamespaceAttribute.
func (a *markupAccumulator) shouldAddNamespaceAttribute(attr *Attr, candidatePrefix string, hasCandidate bool) bool {
	// An attribute without a prefix needs a generated prefix, and an xmlns
	// attribute for that prefix.
	if !hasCandidate {
		return true
	}
	namespace, _ := a.context().lookupNamespaceURI(candidatePrefix)
	return namespace != attr.NS
}

// appendNamespace is MarkupAccumulator::AppendNamespace for a prefix that
// is not empty.
func (a *markupAccumulator) appendNamespace(prefix, namespace string) {
	if found, _ := a.context().lookupNamespaceURI(prefix); found != namespace {
		a.context().add(prefix, namespace)
		appendXMLAttribute(&a.markup, "xmlns", prefix, namespace)
	}
}

// retrievePreferredPrefixString is
// MarkupAccumulator::RetrievePreferredPrefixString. ok is false for the
// null prefix.
func (a *markupAccumulator) retrievePreferredPrefixString(ns, preferredPrefix string) (prefix string, ok bool) {
	ctx := a.context()
	nsForPreferred, preferredInUse := ctx.lookupNamespaceURI(preferredPrefix)
	// Keep the prefix if it is used in the scope and its namespace is the
	// namespace of the node.
	if preferredPrefix != "" && preferredInUse && nsForPreferred == ns {
		return preferredPrefix, true
	}
	// Get the last effective prefix.
	list := ctx.nsPrefixes[ns]
	for i := len(list) - 1; i >= 0; i-- {
		if nsForCandidate, _ := ctx.lookupNamespaceURI(list[i]); nsForCandidate == ns {
			return list[i], true
		}
	}
	// No prefix for ns. Keep the prefix if the scope does not use it.
	if preferredPrefix != "" && !preferredInUse {
		return preferredPrefix, true
	}
	// Without a prefix, or with a prefix that maps to another namespace,
	// the caller generates a prefix.
	return "", false
}

// generatePrefix is MarkupAccumulator::GeneratePrefix: "generating a
// prefix".
func (a *markupAccumulator) generatePrefix(namespace string) string {
	for {
		// 1. Let generated prefix be the concatenation of the string "ns"
		// and the current numerical value of prefix index.
		generated := "ns" + strconv.Itoa(a.prefixIndex)
		// 2. Let the value of prefix index be incremented by one.
		a.prefixIndex++
		if _, inUse := a.context().lookupNamespaceURI(generated); !inUse {
			// 3. Add to map the generated prefix given the new namespace.
			a.context().add(generated, namespace)
			return generated
		}
	}
}

// appendXMLAttribute is MarkupFormatter::AppendAttribute for a document
// that is not HTML.
func appendXMLAttribute(sb *strings.Builder, prefix, local, value string) {
	sb.WriteByte(' ')
	if prefix != "" {
		sb.WriteString(prefix)
		sb.WriteByte(':')
	}
	sb.WriteString(local)
	sb.WriteString(`="`)
	appendXMLAttributeValue(sb, value)
	sb.WriteByte('"')
}

// appendXMLText escapes with kEntityMaskInPCDATA: & < >. U+00A0 stays a
// character, because XML has no entity for it.
func appendXMLText(sb *strings.Builder, s string) {
	last := 0
	for i := 0; i < len(s); i++ {
		var rep string
		switch s[i] {
		case '&':
			rep = "&amp;"
		case '<':
			rep = "&lt;"
		case '>':
			rep = "&gt;"
		default:
			continue
		}
		sb.WriteString(s[last:i])
		sb.WriteString(rep)
		last = i + 1
	}
	sb.WriteString(s[last:])
}

// appendXMLAttributeValue escapes with kEntityMaskInAttributeValue: & < > "
// and tab, line feed and carriage return, which an XML parser would
// otherwise turn into spaces.
func appendXMLAttributeValue(sb *strings.Builder, s string) {
	last := 0
	for i := 0; i < len(s); i++ {
		var rep string
		switch s[i] {
		case '&':
			rep = "&amp;"
		case '<':
			rep = "&lt;"
		case '>':
			rep = "&gt;"
		case '"':
			rep = "&quot;"
		case '\t':
			rep = "&#9;"
		case '\n':
			rep = "&#10;"
		case '\r':
			rep = "&#13;"
		default:
			continue
		}
		sb.WriteString(s[last:i])
		sb.WriteString(rep)
		last = i + 1
	}
	sb.WriteString(s[last:])
}
