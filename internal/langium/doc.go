// Package langium is the shared base of the parsers that upstream writes as
// Langium grammars (packages/parser/src/language of mermaid 12.2.0): info,
// pie, packet, radar, gitGraph, architecture, treemap, treeView, wardley,
// cynefin, eventmodeling and the railroad grammars.
//
// Langium generates two parts from a grammar. The lexer is a Chevrotain
// lexer. This package has an exact port of it, with the token types of every
// grammar taken from the real token builders (vocab_gen.go, which
// tools/reference/gen-langium.mjs writes). The parser is a Chevrotain ALL(*)
// parser. This package does not generate parsers: a diagram package writes a
// small recursive descent parser by hand on top of Parser, and compares it
// with the real parser in a golden test.
//
// # The lexer
//
// NewLexer(grammar) returns the lexer of a grammar, by the name that
// parse(diagramType, text) of the parser package takes: "pie", "gitGraph",
// "architecture" and so on. At each position the lexer tries the token types
// in the order of the vocabulary and takes the first one that matches, not
// the longest one. The order is: the keywords, longest first, then the
// terminals of the grammar file, then the terminals of the imported files.
// A keyword that a terminal can also match (the keyword "axis" and the
// terminal ID for the text "axis1") names that terminal as its longer
// alternative: the terminal wins when its match is longer. The keywords that
// the token builder of the grammar lists ("pie", "showData", "gitGraph")
// match only in front of white space, "%%" or the end of the text.
//
// A token type has a group. Tokens of the Skipped group (white space) are
// dropped. Tokens of the Hidden group (comments, directives, front matter)
// go to a separate list and the parser does not see them. A position where no
// token type matches is a lexer error. Offsets and lengths count UTF-16 code
// units, as in JavaScript.
//
// # Writing a parser
//
// Read the .langium file and write one function for each parser rule:
//
//	var lexer = langium.MustLexer("pie")
//
//	func parse(text string) (*Pie, error) {
//		p, err := lexer.Parse(text)
//		if err != nil {
//			return nil, err
//		}
//		ast := &Pie{}
//		p.Skip("NEWLINE")                        // NEWLINE*
//		if _, err := p.Expect("pie"); err != nil { // "pie"
//			return nil, err
//		}
//		ast.ShowData = p.Accept("showData") != nil // showData?="showData"?
//		for !p.At(langium.EOF) {                   // ( ... )*
//			switch {
//			case p.AtTitleAndAccessibilities():
//				if err := p.TitleAndAccessibilities(&ast.Common); err != nil {
//					return nil, err
//				}
//			case p.At("STRING"):
//				...
//			case p.At("NEWLINE"):
//				p.Next()
//			default:
//				return nil, p.Unexpected()
//			}
//		}
//		return ast, p.End()
//	}
//
// A token type is named by its keyword text ("pie", ":", "{") or by its
// terminal name ("STRING", "NEWLINE"). EOF is the name of the end of the
// text. The methods of Parser are:
//
//   - At(names...) reports whether the next token has one of the types.
//     LA(k) returns the k-th next token, for a decision that needs more than
//     one token.
//   - Accept(name) consumes the next token when it has the type, and returns
//     nil otherwise: an optional element. Skip(name) consumes every next
//     token of the type: a repetition of one token, such as NEWLINE*.
//   - Expect(name) consumes the next token, or returns the error of the real
//     parser for a token of another type.
//   - EOL() is the rule EOL of common.langium: NEWLINE+ | EOF.
//   - AtTitleAndAccessibilities() and TitleAndAccessibilities(&common) are
//     the fragment TitleAndAccessibilities. Common holds the three
//     properties, and Common.Populate is populateCommonDb.
//   - End() returns an error when tokens are left: the real parser reports
//     "Expecting end of file".
//
// The real parser recovers from errors, but mermaid throws when the result
// has a lexer error or a parser error. So a hand-written parser stops at the
// first error. The error texts follow the real ones where that is cheap. They
// are not part of the contract: only "error or no error" is.
//
// The real parser chooses between alternatives with unlimited lookahead and
// takes the first alternative when more than one fits. A loop therefore
// continues as long as the next tokens fit its body. Keep that in mind where
// two alternatives start with the same tokens, and check the result with a
// golden test: tools/reference/gen-langium.mjs shows how to dump the AST of
// the real parser for corpus fixtures and mutated inputs.
//
// # Values
//
// The value converter of Langium turns the text of a token into the value of
// an AST property. Use the function for the terminal rule of the token:
// ConvertString for STRING (it removes the quotes and reads the escapes),
// ConvertInt for INT, ConvertNumber for a terminal that returns number,
// ConvertBoolean for BOOLEAN, ConvertID for ID, and ConvertAccDescr,
// ConvertAccTitle and ConvertTitle for the three common terminals. A keyword
// and a terminal that returns string without a converter keep their text.
package langium
