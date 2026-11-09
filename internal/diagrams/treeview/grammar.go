// Ported from mermaid 12.2.0,
// packages/parser/src/language/treeView/treeView.langium and valueConverter.ts
// (MIT).

package treeview

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// TreeView is the AST of the entry rule TreeView.
type TreeView struct {
	langium.Common
	Nodes []TreeNode
}

// TreeNode is the AST of the rule TreeNode. A nil pointer is a property
// that the text does not set: undefined in the AST.
type TreeNode struct {
	Indent          *int
	Name            string
	ClassAnnotation *string
	IconAnnotation  *string
	DescAnnotation  *string
}

var lexer = langium.MustLexer("treeView")

// parse is parse('treeView', text) of the parser package. The grammar is:
//
//	entry TreeView returns TreeView:
//	  "treeView-beta"
//	  TitleAndAccessibilities?
//	  nodes+=TreeNode*
//	;
//	fragment TitleAndAccessibilities:
//	  ((accDescr=ACC_DESCR | accTitle=ACC_TITLE | title=TITLE))+
//	;
//	TreeNode:
//	  indent=INDENTATION?
//	  (name=QUOTED_NAME | name=BARE_NAME)
//	  (classAnnotation=CLASS_ANNOTATION | iconAnnotation=ICON_ANNOTATION | descAnnotation=DESC_ANNOTATION)*
//	;
//
// The line breaks are hidden tokens. White space is the token INDENTATION
// in every place, because that terminal comes first.
func parse(text string) (*TreeView, error) {
	p, err := lexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &TreeView{Nodes: []TreeNode{}}
	if _, err := p.Expect("treeView-beta"); err != nil {
		return nil, err
	}
	// The fragment of this grammar has no EOL after a title.
	for p.AtTitleAndAccessibilities() {
		t := p.Next()
		var value string
		var ok bool
		var target **string
		switch t.Type.Name {
		case "ACC_DESCR":
			value, ok = langium.ConvertAccDescr(t.Image)
			target = &ast.AccDescr
		case "ACC_TITLE":
			value, ok = langium.ConvertAccTitle(t.Image)
			target = &ast.AccTitle
		default:
			value, ok = langium.ConvertTitle(t.Image)
			target = &ast.Title
		}
		if !ok {
			// The common converter returns undefined, the converter of
			// this grammar has no case for the rule, and the default
			// converter of Langium then keeps the text of the token.
			value = t.Image
		}
		*target = &value
	}
	for p.At("INDENTATION", "QUOTED_NAME", "BARE_NAME") {
		node, err := treeNode(p)
		if err != nil {
			return nil, err
		}
		ast.Nodes = append(ast.Nodes, node)
	}
	return ast, p.End()
}

// treeNode is the rule TreeNode.
func treeNode(p *langium.Parser) (TreeNode, error) {
	node := TreeNode{}
	if t := p.Accept("INDENTATION"); t != nil {
		// input?.length || 0
		indent := js.UTF16Len(t.Image)
		node.Indent = &indent
	}
	switch {
	case p.At("QUOTED_NAME"):
		// Strip surrounding quotes: "name" or 'name' → name
		image := p.Next().Image
		node.Name = image[1 : len(image)-1]
	case p.At("BARE_NAME"):
		// Trim trailing whitespace only. Internal runs of spaces are part
		// of the name.
		node.Name = strings.TrimRight(p.Next().Image, "\t ")
	default:
		return node, p.Unexpected("QUOTED_NAME", "BARE_NAME")
	}
	for {
		switch {
		case p.At("CLASS_ANNOTATION"):
			// " :::className" → className (strip leading whitespace and :::)
			trimmed := js.TrimSpace(p.Next().Image)
			value := js.TrimSpace(trimmed[3:])
			node.ClassAnnotation = &value
		case p.At("ICON_ANNOTATION"):
			// " icon(name)" → name, " icon()" → empty string
			trimmed := js.TrimSpace(p.Next().Image)
			value := trimmed[5 : len(trimmed)-1]
			node.IconAnnotation = &value
		case p.At("DESC_ANNOTATION"):
			// " ## description text" → description text
			trimmed := js.TrimSpace(p.Next().Image)
			value := js.TrimSpace(trimmed[2:])
			node.DescAnnotation = &value
		default:
			return node, nil
		}
	}
}
