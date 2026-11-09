// Ported from the parser runtime of chevrotain 13.2.0 (Apache-2.0), for a
// CstParser with the default config:
// parse/parser/traits/recognizer_engine.ts, lexer_adapter.ts,
// tree_builder.ts, error_handler.ts, parse/grammar/lookahead.ts (the
// lookahead functions), parse/errors_public.ts and parse/cst/cst.ts.
//
// Error recovery is off in that config. So the first recognition exception
// ends the parse: the rule that raised it and its callers return at once.
// This port raises the exception with panic, and parseTokens recovers it.
//
// chevrotain has the Apache License 2.0. Its files have no copyright line,
// and the package has no NOTICE file. This file is a Go translation of the
// files above. It is a changed version of them. LICENSE-chevrotain.txt in
// this directory has the license.

package usecase

import (
	"strconv"
	"strings"
)

// prodKind is the kind of a production that needs a lookahead.
type prodKind int

const (
	prodOr prodKind = iota
	prodOption
	prodMany
	prodAtLeastOne
)

// decision names a production of a rule: its kind and its occurrence index
// (OR is 0, OR2 is 2, and so on).
type decision struct {
	rule string
	kind prodKind
	idx  int
}

// cstElement is one child of a CST node: a token or a node.
type cstElement struct {
	tok  *token
	node *cstNode
}

// cstNode is CstNode of chevrotain. The children are kept by key in the
// order of the first use of each key, as the properties of the JavaScript
// object are. The location of a node (nodeLocationTracking) is not kept:
// the visitor of upstream reads it only to sort the lines of a system
// boundary, which are in source order already (see visitor.go).
type cstNode struct {
	name     string
	keys     []string
	children map[string][]cstElement
}

func (n *cstNode) add(key string, el cstElement) {
	if _, ok := n.children[key]; !ok {
		n.keys = append(n.keys, key)
	}
	n.children[key] = append(n.children[key], el)
}

// The names of the recognition exceptions of chevrotain.
const (
	mismatchedTokenException   = "MismatchedTokenException"
	noViableAltException       = "NoViableAltException"
	earlyExitException         = "EarlyExitException"
	notAllInputParsedException = "NotAllInputParsedException"
)

// recognitionException is IRecognitionException of chevrotain.
type recognitionException struct {
	name    string
	message string
	// token is the token at which the parser stopped.
	token *token
}

// alternative is IOrAlt: one alternative of an OR, with its optional GATE.
type alternative struct {
	gate func() bool
	alt  func()
}

// recognizer is the parser state of chevrotain: the token vector, the
// position in it, and the stacks of the rules and of the CST nodes.
type recognizer struct {
	tokVector []*token
	// currIdx is the index of the last consumed token.
	currIdx   int
	ruleStack []string
	cstStack  []*cstNode
}

// la is LA(howMuch): the token at a distance from the last consumed token.
// LA(1) is the next token and LA(0) is the previous token. It is the
// end-of-file token outside the token vector.
func (p *recognizer) la(howMuch int) *token {
	soughtIdx := p.currIdx + howMuch
	if soughtIdx < 0 || len(p.tokVector) <= soughtIdx {
		return endOfFile
	}
	return p.tokVector[soughtIdx]
}

func (p *recognizer) currRule() string { return p.ruleStack[len(p.ruleStack)-1] }

func (p *recognizer) currCst() *cstNode { return p.cstStack[len(p.cstStack)-1] }

// rule is invokeRuleWithTry: it runs the implementation of a rule and
// returns the CST node of the rule.
func (p *recognizer) rule(name string, impl func()) *cstNode {
	// ruleInvocationStateUpdate and cstInvocationStateUpdate
	p.ruleStack = append(p.ruleStack, name)
	cst := &cstNode{name: name, children: map[string][]cstElement{}}
	p.cstStack = append(p.cstStack, cst)

	impl()

	// ruleFinallyStateUpdate and cstFinallyStateUpdate
	p.ruleStack = p.ruleStack[:len(p.ruleStack)-1]
	p.cstStack = p.cstStack[:len(p.cstStack)-1]
	return cst
}

// subrule is SUBRULE(ruleToCall): it calls a rule and adds its CST node to
// the node of the current rule.
func (p *recognizer) subrule(ruleToCall func() *cstNode) *cstNode {
	ruleResult := ruleToCall()
	// cstPostNonTerminal
	p.currCst().add(ruleResult.name, cstElement{node: ruleResult})
	return ruleResult
}

// consume is CONSUME(tokType): it takes the next token when it has the
// token type, and raises a MismatchedTokenException otherwise.
func (p *recognizer) consume(tokType tokenType) *token {
	nextToken := p.la(1)
	if !tokenMatcher(nextToken, tokType) {
		// buildMismatchTokenMessage
		msg := "Expecting token of type --> " + tokenTypeNames[tokType] + " <-- but found --> '" + nextToken.image + "' <--"
		panic(&recognitionException{name: mismatchedTokenException, message: msg, token: nextToken})
	}
	p.currIdx++
	// cstPostTerminal
	p.currCst().add(tokenTypeNames[tokType], cstElement{tok: nextToken})
	return nextToken
}

// matchesPath reports whether the next tokens have the token types of a
// lookahead path.
func (p *recognizer) matchesPath(path []tokenType) bool {
	for i, tokType := range path {
		if !tokenMatcher(p.la(i+1), tokType) {
			return false
		}
	}
	return true
}

// matchesAnyPath is the lookahead function of one alternative: whether
// one of its paths matches the next tokens.
func (p *recognizer) matchesAnyPath(paths [][]tokenType) bool {
	for _, path := range paths {
		if p.matchesPath(path) {
			return true
		}
	}
	return false
}

// enters is the lookahead function of an OPTION, a MANY or an AT_LEAST_ONE
// of the current rule, with the GATE of the production applied first.
func (p *recognizer) enters(kind prodKind, idx int, gate func() bool) bool {
	if gate != nil && !gate() {
		return false
	}
	return p.matchesAnyPath(lookaheadPaths[decision{p.currRule(), kind, idx}][0])
}

// option is OPTION<idx>({ GATE, DEF }).
func (p *recognizer) option(idx int, gate func() bool, def func()) {
	if p.enters(prodOption, idx, gate) {
		def()
	}
}

// doSingleRepetition reports whether one iteration consumed a token.
func (p *recognizer) doSingleRepetition(action func()) bool {
	beforeIteration := p.currIdx
	action()
	afterIteration := p.currIdx
	return afterIteration > beforeIteration
}

// many is MANY<idx>(DEF).
func (p *recognizer) many(idx int, def func()) {
	notStuck := true
	for p.enters(prodMany, idx, nil) && notStuck {
		notStuck = p.doSingleRepetition(def)
	}
}

// atLeastOne is AT_LEAST_ONE<idx>(DEF).
func (p *recognizer) atLeastOne(idx int, def func()) {
	if p.enters(prodAtLeastOne, idx, nil) {
		notStuck := p.doSingleRepetition(def)
		for p.enters(prodAtLeastOne, idx, nil) && notStuck {
			notStuck = p.doSingleRepetition(def)
		}
	} else {
		p.raiseEarlyExitException(idx)
	}
}

// or is OR<idx>(alternatives): it runs the first alternative whose gate
// holds and that has a lookahead path that matches the next tokens.
func (p *recognizer) or(idx int, alts []alternative) {
	paths := lookaheadPaths[decision{p.currRule(), prodOr, idx}]
	for t := range paths {
		if alts[t].gate != nil && !alts[t].gate() {
			// if the predicate does not match there is no point in checking the paths
			continue
		}
		if p.matchesAnyPath(paths[t]) {
			alts[t].alt()
			return
		}
	}
	p.raiseNoAltException(idx)
}

func pathLabels(path []tokenType, sep string) string {
	names := make([]string, len(path))
	for i, tokType := range path {
		names[i] = tokenTypeNames[tokType]
	}
	return "[" + strings.Join(names, sep) + "]"
}

// raiseNoAltException raises the NoViableAltException of an OR, with the
// message of buildNoViableAltMessage.
func (p *recognizer) raiseNoAltException(occurrence int) {
	lookAheadPathsPerAlternative := lookaheadPaths[decision{p.currRule(), prodOr, occurrence}]
	var items []string
	for _, currAltPaths := range lookAheadPathsPerAlternative {
		for _, currPath := range currAltPaths {
			items = append(items, "  "+strconv.Itoa(len(items)+1)+". "+pathLabels(currPath, ", "))
		}
	}
	actual := p.la(1)
	errMsg := "Expecting: one of these possible Token sequences:\n" + strings.Join(items, "\n") +
		"\nbut found: '" + actual.image + "'"
	panic(&recognitionException{name: noViableAltException, message: errMsg, token: actual})
}

// raiseEarlyExitException raises the EarlyExitException of an AT_LEAST_ONE,
// with the message of buildEarlyExitMessage.
func (p *recognizer) raiseEarlyExitException(occurrence int) {
	insideProdPaths := lookaheadPaths[decision{p.currRule(), prodAtLeastOne, occurrence}][0]
	sequences := make([]string, len(insideProdPaths))
	for i, currPath := range insideProdPaths {
		sequences[i] = pathLabels(currPath, ",")
	}
	actual := p.la(1)
	msg := "Expecting: expecting at least one iteration which starts with one of these possible Token sequences::\n  " +
		"<" + strings.Join(sequences, " ,") + ">" +
		"\nbut found: '" + actual.image + "'"
	panic(&recognitionException{name: earlyExitException, message: msg, token: actual})
}

// parseTokens runs a top rule on a token vector. It returns the CST, or
// the first recognition exception: the one that errors[0] of the real
// parser holds.
func parseTokens(tokens []*token, top func(p *usecaseParser) *cstNode) (cst *cstNode, exception *recognitionException) {
	p := &usecaseParser{recognizer: recognizer{tokVector: tokens, currIdx: -1}}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*recognitionException)
			if !ok {
				panic(r)
			}
			cst, exception = nil, e
		}
	}()
	cst = top(p)
	// onAfterParse
	if firstRedundantTok := p.la(1); !tokenMatcher(firstRedundantTok, tokEof) {
		// buildNotAllInputParsedMessage
		errMsg := "Redundant input, expecting EOF but found: " + firstRedundantTok.image
		return nil, &recognitionException{name: notAllInputParsedException, message: errMsg, token: firstRedundantTok}
	}
	return cst, nil
}
