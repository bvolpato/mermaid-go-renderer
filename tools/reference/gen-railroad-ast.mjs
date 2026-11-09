#!/usr/bin/env node
// Writes internal/diagrams/railroad/testdata/ast.json.gz: parser goldens for
// the four railroad grammars from the real @mermaid-js/parser (the Langium
// grammars packages/parser/src/language/railroad, railroad-ebnf,
// railroad-abnf and railroad-peg).
//
// Each case is a text with the AST of the real parser, or the fact that the
// parser fails (a lexer error or a parser error, the condition on which the
// parsers of mermaid throw). The parsers are the ones that the diagram code
// of mermaid calls: create<Name>Services().<Name>.parser.LangiumParser. The
// texts are the corpus fixtures, hand-written inputs for the branches of the
// grammars and of the value converters, random sentences of each grammar, and
// seeded mutations of all of these.
//
// Run it with tools/reference as the working directory:
//
//   node gen-railroad-ast.mjs
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import * as P from '@mermaid-js/parser';
import { prng } from './jison-lib.mjs';
import { internals } from './lib-mermaid-internals.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');
const CORPUS = path.join(ROOT, 'testdata/corpus/mermaid-e2e/railroad');
const REF = path.join(ROOT, '.parity/ref/mermaid-e2e/railroad');
const OUT = path.join(ROOT, 'internal/diagrams/railroad/testdata/ast.json.gz');
const MUTANTS = 1500;

const GRAMMARS = {
  railroad: P.createRailroadServices().Railroad,
  railroadEbnf: P.createRailroadEbnfServices().RailroadEbnf,
  railroadAbnf: P.createRailroadAbnfServices().RailroadAbnf,
  railroadPeg: P.createRailroadPegServices().RailroadPeg,
};

// plain turns an AST into JSON: it drops the links to the CST and to the
// container, and the properties that are undefined.
function plain(node) {
  if (Array.isArray(node)) {
    return node.map(plain);
  }
  if (node === null || typeof node !== 'object') {
    return node;
  }
  const out = {};
  for (const key of Object.keys(node)) {
    if (key.startsWith('$') && key !== '$type') {
      continue;
    }
    if (node[key] !== undefined) {
      out[key] = plain(node[key]);
    }
  }
  return out;
}

function run(name, text) {
  const result = GRAMMARS[name].parser.LangiumParser.parse(text);
  if (result.lexerErrors.length > 0 || result.parserErrors.length > 0) {
    return { text, error: true };
  }
  return { text, ast: plain(result.value) };
}

// encodeEntities is the function of utils.ts.
function encodeEntities(text) {
  let txt = text;
  txt = txt.replace(/style.*:\S*#.*;/g, (s) => s.substring(0, s.length - 1));
  txt = txt.replace(/classDef.*:\S*#.*;/g, (s) => s.substring(0, s.length - 1));
  txt = txt.replace(/#\w+;/g, (s) => {
    const innerTxt = s.substring(1, s.length - 1);
    const isInt = /^\+?\d+$/.test(innerTxt);
    return isInt ? 'ﬂ°°' + innerTxt + '¶ß' : 'ﬂ°' + innerTxt + '¶ß';
  });
  return txt;
}

// ---------------------------------------------------------------------------
// Hand-written inputs, by grammar.

const ir = (body) => 'railroad-beta\n' + body;
const ebnf = (body) => 'railroad-ebnf-beta\n' + body;
const abnf = (body) => 'railroad-abnf-beta\n' + body;
const peg = (body) => 'railroad-peg-beta\n' + body;

// The header, the title and the accessibility statements are the same in the
// four grammars. head gives them for a keyword and one valid rule.
const head = (kw, rule) => [
  kw,
  kw + '\n',
  kw + '\n\n\n',
  ' ' + kw + '\n',
  '\n\n\t ' + kw + '  \n',
  kw + 'X\n' + rule,
  kw + '-x\n' + rule,
  kw + '%% comment\n' + rule,
  kw + ' %% comment\n' + rule,
  kw + ';\n' + rule,
  kw + ' ' + rule,
  kw + '\r\n' + rule + '\r\n',
  kw.toUpperCase() + '\n' + rule,
  kw.replace('-beta', '') + '\n' + rule,
  kw + '\n' + kw + '\n' + rule,
  '',
  '\n',
  rule,
  rule + '\n' + kw + '\n',
  kw + '\n' + rule,
  kw + '\n' + rule + '\n' + rule,
  kw + '\n' + rule + rule,
  kw + '\n\n\n' + rule + '\n\n\n',
  '%% first\n' + kw + '\n' + rule,
  '%%{init: {"theme": "dark"}}%%\n' + kw + '\n' + rule,
  '---\ntitle: T\n---\n' + kw + '\n' + rule,
  kw + '\n---\ntitle: T\n---\n' + rule,
  kw + '\n%%{init: {}}%%\n' + rule,
  kw + '\n%% comment\n' + rule + ' %% trailing\n',
  // title
  kw + '\ntitle My Grammar\n' + rule,
  kw + '\ntitle   spaced    out   title  \n' + rule,
  kw + '\ntitle\n' + rule,
  kw + '\ntitle \n' + rule,
  kw + '\ntitle\t\n' + rule,
  kw + '\ntitle' + rule,
  kw + '\ntitleX\n' + rule,
  kw + ' title T\n' + rule,
  kw + '\ntitle "Quoted title"\n' + rule,
  kw + "\ntitle 'Single quoted'\n" + rule,
  kw + '\ntitle "a \\"b\\" c"\n' + rule,
  kw + '\ntitle "line\\nbreak\\ttab\\rcr\\\\slash\\x"\n' + rule,
  kw + '\ntitle "trailing\\"\n' + rule,
  kw + '\ntitle "\n' + rule,
  kw + '\ntitle ""\n' + rule,
  kw + "\ntitle '\n" + rule,
  kw + '\ntitle "open\n' + rule,
  kw + '\ntitle close"\n' + rule,
  kw + '\ntitle "mixed\'\n' + rule,
  kw + '\ntitle "a" and "b"\n' + rule,
  kw + '\ntitle   "  padded  "  \n' + rule,
  kw + '\ntitle T %% comment\n' + rule,
  kw + '\ntitle "T" %% comment\n' + rule,
  kw + '\ntitle %% only comment\n' + rule,
  kw + '\ntitle T%%c\n' + rule,
  kw + '\ntitle Ünïcödé 日本語 😀\n' + rule,
  kw + '\ntitle "\\😀"\n' + rule,
  kw + '\ntitle <b>bold</b> & <script>alert(1)</script>\n' + rule,
  kw + '\ntitle One\ntitle Two\n' + rule,
  kw + '\ntitle One\naccTitle: A\ntitle Two\naccDescr: D\naccTitle: B\n' + rule,
  kw + '\n' + rule + '\ntitle After\n',
  kw + '\n' + rule + '\naccTitle: After\n',
  kw + '\n' + rule + '\naccDescr: After\n',
  kw + '\nTITLE upper\n' + rule,
  kw + '\ntitle a\u00a0b \u2028 c\n' + rule,
  // accTitle, accDescr
  kw + '\naccTitle: Acc title\n' + rule,
  kw + '\naccTitle:\n' + rule,
  kw + '\naccTitle :   spaced   out  \n' + rule,
  kw + '\naccTitle\n' + rule,
  kw + '\naccTitle: "quoted"\n' + rule,
  kw + '\naccTitle: T %% comment\n' + rule,
  kw + '\naccTitle: a\naccTitle: b\n' + rule,
  kw + '\naccDescr: Acc description\n' + rule,
  kw + '\naccDescr:\n' + rule,
  kw + '\naccDescr :  spaced   out \n' + rule,
  kw + '\naccDescr: D %% comment\n' + rule,
  kw + '\naccDescr {\n  multi line\n    description   here\n\n\n  end\n}\n' + rule,
  kw + '\naccDescr{one}\n' + rule,
  kw + '\naccDescr { one } ' + rule,
  kw + '\naccDescr {}\n' + rule,
  kw + '\naccDescr {\n unclosed\n' + rule,
  kw + '\naccDescr\n' + rule,
  kw + '\naccDescr\n\n{ x }\n' + rule,
  kw + '\naccDescr: a\naccDescr { b }\n' + rule,
  kw + '\naccDescr { a %% not a comment\n b }\n' + rule,
  kw + '\ntitle T\naccTitle: A\naccDescr: D\n',
  // characters that no token type matches
  kw + '\n' + rule + '\n@\n',
  kw + '\n' + rule + '\nﬂ°35¶ß\n',
  kw + '\n\u00a0' + rule,
];

const IR_RULE = 'digit = terminal("0") ;\n';
const EBNF_RULE = 'digit = "0" ;\n';
const ABNF_RULE = 'digit = "0" ;\n';
const PEG_RULE = 'Digit <- "0" ;\n';

const HAND = {
  railroad: [
    ...head('railroad-beta', IR_RULE),
    // every expression type
    ir('a = terminal("x") ;'),
    ir('a = nonterminal("x") ;'),
    ir('a = special("x") ;'),
    ir('a = sequence(terminal("x")) ;'),
    ir('a = sequence(terminal("x"), nonterminal("y")) ;'),
    ir('a = sequence(terminal("x"), nonterminal("y"), special("z")) ;'),
    ir('a = choice(terminal("x")) ;'),
    ir('a = choice(terminal("x"), nonterminal("y")) ;'),
    ir('a = optional(terminal("x")) ;'),
    ir('a = oneOrMore(terminal("x")) ;'),
    ir('a = zeroOrMore(terminal("x")) ;'),
    ir('a = optional(optional(optional(terminal("x")))) ;'),
    ir('a = zeroOrMore(oneOrMore(optional(choice(sequence(terminal("x")))))) ;'),
    ir('a = sequence(choice(terminal("1"), terminal("2")), choice(sequence(terminal("3"), terminal("4")), terminal("5"))) ;'),
    ir('a = sequence(sequence(sequence(terminal("x")))) ;'),
    ir('a = choice(choice(choice(terminal("x")), terminal("y")), terminal("z")) ;'),
    // layout
    ir('a=terminal("x");'),
    ir('a\n=\nterminal\n(\n"x"\n)\n;\n'),
    ir('a = terminal("x") ; b = terminal("y") ;'),
    ir('a = terminal("x");b = terminal("y");c=nonterminal("a");'),
    ir('\ta\t=\tsequence(\tterminal("x")\t,\tterminal("y")\t)\t;\t'),
    ir('a = terminal("x") ;\r\nb = terminal("y") ;\r\n'),
    ir('a = terminal("x") ; %% comment\nb = terminal("y") ;'),
    ir('a = /* block */ terminal("x") ;'),
    ir('a = terminal( /* in\nside */ "x") ;'),
    ir('/* a = terminal("x") ; */'),
    ir('/* unclosed\na = terminal("x") ;'),
    ir('a = terminal("x") ; // not a comment'),
    ir('a = terminal("x") ; # not a comment'),
    ir('a = terminal("x") ; (* not a comment *)'),
    ir('%% c\n%% d\na = terminal("x") ;\n%% e'),
    // errors of the rule
    ir('a = terminal("x")'),
    ir('a terminal("x") ;'),
    ir('= terminal("x") ;'),
    ir('a = ;'),
    ir('a = "x" ;'),
    ir('a = x ;'),
    ir('a = terminal ;'),
    ir('a = terminal() ;'),
    ir('a = terminal(x) ;'),
    ir('a = terminal("x" ;'),
    ir('a = terminal "x") ;'),
    ir('a = terminal("x", "y") ;'),
    ir('a = terminal(terminal("x")) ;'),
    ir('a = nonterminal() ;'),
    ir('a = nonterminal(b) ;'),
    ir('a = special() ;'),
    ir('a = sequence() ;'),
    ir('a = sequence(,) ;'),
    ir('a = sequence(terminal("x"),) ;'),
    ir('a = sequence(,terminal("x")) ;'),
    ir('a = sequence(terminal("x") terminal("y")) ;'),
    ir('a = sequence(terminal("x"),, terminal("y")) ;'),
    ir('a = sequence("x") ;'),
    ir('a = choice() ;'),
    ir('a = choice(terminal("x"),) ;'),
    ir('a = optional() ;'),
    ir('a = optional(terminal("x"), terminal("y")) ;'),
    ir('a = oneOrMore() ;'),
    ir('a = oneOrMore(terminal("x"), terminal("y")) ;'),
    ir('a = zeroOrMore() ;'),
    ir('a = zeroOrMore(terminal("x"), terminal("y")) ;'),
    ir('a = terminal("x") ;;'),
    ir('a = terminal("x") ; ;'),
    ir(';'),
    ir('a = terminal("x") terminal("y") ;'),
    ir('a = b = terminal("x") ;'),
    ir('a = unknown("x") ;'),
    ir('a = Terminal("x") ;'),
    ir('a = SEQUENCE(terminal("x")) ;'),
    ir('a = oneormore(terminal("x")) ;'),
    ir('a = (terminal("x")) ;'),
    ir('a := terminal("x") ;'),
    ir('a ::= terminal("x") ;'),
    ir('a <- terminal("x") ;'),
    // rule names
    ir('a-b = terminal("x") ;'),
    ir('_a = terminal("x") ;'),
    ir('A_1-b_ = terminal("x") ;'),
    ir('a- = terminal("x") ;'),
    ir('-a = terminal("x") ;'),
    ir('1a = terminal("x") ;'),
    ir('a1 = terminal("x") ;'),
    ir('a b = terminal("x") ;'),
    ir('a.b = terminal("x") ;'),
    ir('é = terminal("x") ;'),
    ir('"a" = terminal("x") ;'),
    ir('sequence = terminal("x") ;'),
    ir('terminal = terminal("x") ;'),
    ir('choice = terminal("x") ;'),
    ir('optional = terminal("x") ;'),
    ir('nonterminal = terminal("x") ;'),
    ir('special = terminal("x") ;'),
    ir('oneOrMore = terminal("x") ;'),
    ir('zeroOrMore = terminal("x") ;'),
    ir('sequenceX = terminal("x") ;'),
    ir('terminals = terminal("x") ;'),
    ir('choice-1 = terminal("x") ;'),
    ir('sequence_ = sequence(terminal("x")) ;'),
    ir('a = terminalX("x") ;'),
    ir('title = terminal("x") ;'),
    ir('titles = terminal("x") ;'),
    ir('subtitle = terminal("x") ;'),
    ir('accTitle = terminal("x") ;'),
    ir('accDescr = terminal("x") ;'),
    ir('accTitles = terminal("x") ;'),
    ir('railroad-beta = terminal("x") ;'),
    ir('railroad-betas = terminal("x") ;'),
    ir('a = nonterminal("sequence") ;'),
    ir('a = terminal("x") ;\na = terminal("y") ;'),
    // strings
    ir("a = terminal('x') ;"),
    ir('a = terminal("") ;'),
    ir("a = terminal('') ;"),
    ir('a = terminal(" ") ;'),
    ir('a = terminal("a b  c") ;'),
    ir('a = terminal("it\'s") ;'),
    ir("a = terminal('say \"hi\"') ;"),
    ir('a = terminal("say \\"hi\\"") ;'),
    ir("a = terminal('it\\'s') ;"),
    ir('a = terminal("back\\\\slash") ;'),
    ir('a = terminal("\\n\\r\\t\\b\\f\\v\\0\\x\\u0041") ;'),
    ir('a = terminal("line\nbreak") ;'),
    ir('a = terminal("tab\there") ;'),
    ir('a = terminal("\\\n") ;'),
    ir('a = terminal("\\") ;'),
    ir('a = terminal("\\\\") ;'),
    ir('a = terminal("\\\\\\") ;'),
    ir('a = terminal("unclosed) ;'),
    ir("a = terminal('unclosed) ;"),
    ir('a = terminal("mixed\') ;'),
    ir('a = terminal("Ünïcödé 日本語 😀") ;'),
    ir('a = terminal("\\😀") ;'),
    ir('a = terminal("<b>bold</b>") ;'),
    ir('a = terminal("%% not a comment") ;'),
    ir('a = terminal("/* not a comment */") ;'),
    ir('a = terminal("a;b") ;'),
    ir('a = terminal("x")terminal("y") ;'),
    ir('a = nonterminal("a b") ;'),
    ir('a = special("any char") ;'),
    ir('a = terminal("x""y") ;'),
    ir('a = terminal("#35;") ;'),
    ir('a = terminal("ﬂ°°35¶ß") ;'),
    // larger texts
    ir('json = nonterminal("element") ;\nelement = choice(nonterminal("object"), nonterminal("array"), terminal("true")) ;\nobject = sequence(terminal("{"), optional(sequence(nonterminal("member"), zeroOrMore(sequence(terminal(","), nonterminal("member"))))), terminal("}")) ;\n'),
    ir('title Numbers\naccTitle: Number grammar\naccDescr {\n  A grammar\n  of numbers\n}\nnumber = sequence(optional(terminal("-")), oneOrMore(nonterminal("digit")), optional(sequence(terminal("."), oneOrMore(nonterminal("digit"))))) ;\ndigit = choice(terminal("0"), terminal("1")) ;\n'),
  ],
  railroadEbnf: [
    ...head('railroad-ebnf-beta', EBNF_RULE),
    // the rule and its two definition signs
    ebnf('a = b ;'),
    ebnf('a ::= b ;'),
    ebnf('a := b ;'),
    ebnf('a : b ;'),
    ebnf('a :: b ;'),
    ebnf('a == b ;'),
    ebnf('a ::== b ;'),
    ebnf('a <- b ;'),
    ebnf('a=b;'),
    ebnf('a::=b;'),
    ebnf('a\n=\nb\n;\n'),
    ebnf('a = b'),
    ebnf('a = ;'),
    ebnf('a b ;'),
    ebnf('= b ;'),
    ebnf('a = b ;;'),
    ebnf(';'),
    ebnf('a = b ; c = d ;'),
    ebnf('a = b ;c ::= d;e=f;'),
    ebnf('a = b c = d ;'),
    ebnf('a = b ;\r\nc = d ;\r\n'),
    // primaries
    ebnf('a = "x" ;'),
    ebnf("a = 'x' ;"),
    ebnf('a = "" ;'),
    ebnf('a = "say \\"hi\\"\\n\\t\\r\\\\\\x" ;'),
    ebnf('a = "line\nbreak" ;'),
    ebnf('a = "unclosed ;'),
    ebnf('a = "Ünïcödé 日本語 😀" ;'),
    ebnf('a = x ;'),
    ebnf('a = x-y_z1 ;'),
    ebnf('a = _x ;'),
    ebnf('a = 1x ;'),
    ebnf('a = x1 ;'),
    ebnf('a = ( b ) ;'),
    ebnf('a = [ b ] ;'),
    ebnf('a = { b } ;'),
    ebnf('a = (b);'),
    ebnf('a = [b];'),
    ebnf('a = {b};'),
    ebnf('a = ( b | c ) ;'),
    ebnf('a = [ b , c ] ;'),
    ebnf('a = { b | c , d } ;'),
    ebnf('a = ((b)) ;'),
    ebnf('a = [[b]] ;'),
    ebnf('a = {{b}} ;'),
    ebnf('a = ( [ { b } ] ) ;'),
    ebnf('a = () ;'),
    ebnf('a = [] ;'),
    ebnf('a = {} ;'),
    ebnf('a = ( b ;'),
    ebnf('a = b ) ;'),
    ebnf('a = [ b ;'),
    ebnf('a = { b ;'),
    ebnf('a = ( b ] ;'),
    ebnf('a = [ b } ;'),
    ebnf('a = { b ) ;'),
    // special sequences
    ebnf('a = ? special ? ;'),
    ebnf('a = ?special? ;'),
    ebnf('a = ?   padded   text   ? ;'),
    ebnf('a = ? any character ? , b ;'),
    ebnf('a = b , ? x ? ;'),
    ebnf('a = ? x ? ? y ? ;'),
    ebnf('a = ?x??y? ;'),
    ebnf('a = ?? ;'),
    ebnf('a = ? ? ;'),
    ebnf('a = ? ;'),
    ebnf('a = ? x ;'),
    ebnf('a = ? x ; y ? ;'),
    ebnf('a = ? line\nbreak ? ;'),
    ebnf('a = ? "quoted" | ( x ) ? ;'),
    ebnf('a = ? Ünïcödé 😀 ? ;'),
    ebnf('a = ?\u00a0? ;'),
    ebnf('a = ? x ?? ;'),
    ebnf('a = ? x ? ? ;'),
    ebnf('a = ? x ?* ;'),
    ebnf('a = ? x ? - ? y ? ;'),
    // postfixes
    ebnf('a = b? ;'),
    ebnf('a = b* ;'),
    ebnf('a = b+ ;'),
    ebnf('a = b ? ;'),
    ebnf('a = b * ;'),
    ebnf('a = b + ;'),
    ebnf('a = b?*+ ;'),
    ebnf('a = b+*? ;'),
    ebnf('a = b** ;'),
    ebnf('a = b++ ;'),
    ebnf('a = b?? ;'),
    ebnf('a = b? c? ;'),
    ebnf('a = b ? c ? ;'),
    ebnf('a = b? | c? ;'),
    ebnf('a = b ? | c ? ;'),
    ebnf('a = b?\nc = d? ;'),
    ebnf('a = b? ;\nc = d? ;'),
    ebnf('a = "x"? ;'),
    ebnf('a = ( b c )* ;'),
    ebnf('a = [ b ]+ ;'),
    ebnf('a = { b }? ;'),
    ebnf('a = ? ;'),
    ebnf('a = * ;'),
    ebnf('a = + ;'),
    ebnf('a = *b ;'),
    ebnf('a = +b ;'),
    ebnf('a = b - c ;'),
    ebnf('a = b-c ;'),
    ebnf('a = b -c ;'),
    ebnf('a = b- c ;'),
    ebnf('a = b - "x" ;'),
    ebnf('a = "x" - "y" ;'),
    ebnf('a = b - c - d ;'),
    ebnf('a = b - ( c | d ) ;'),
    ebnf('a = b - [ c ] ;'),
    ebnf('a = b - { c } ;'),
    ebnf('a = b - ? x ? ;'),
    ebnf('a = b - c* ;'),
    ebnf('a = b* - c ;'),
    ebnf('a = b - c? - d+ ;'),
    ebnf('a = b - ;'),
    ebnf('a = - b ;'),
    ebnf('a = b - - c ;'),
    ebnf('a = b -- c ;'),
    ebnf('a = b --- c ;'),
    ebnf('a = b - | c ;'),
    ebnf('a = b - , c ;'),
    ebnf('a = b - c , d - e ;'),
    ebnf('a = letter - "x" , digit ;'),
    // sequences and choices
    ebnf('a = b , c ;'),
    ebnf('a = b c ;'),
    ebnf('a = b,c,d ;'),
    ebnf('a = b c d ;'),
    ebnf('a = b , c d , e ;'),
    ebnf('a = b , , c ;'),
    ebnf('a = b ,, c ;'),
    ebnf('a = , b ;'),
    ebnf('a = b , ;'),
    ebnf('a = b , | c ;'),
    ebnf('a = b | c ;'),
    ebnf('a = b|c|d ;'),
    ebnf('a = b | c , d | e f ;'),
    ebnf('a = b | ;'),
    ebnf('a = | b ;'),
    ebnf('a = b || c ;'),
    ebnf('a = b | | c ;'),
    ebnf('a = b / c ;'),
    ebnf('a = "a" "b" "c" ;'),
    ebnf('a = "a""b" ;'),
    ebnf('a = b"x"c ;'),
    ebnf('a = b(c)[d]{e} ;'),
    // comments
    ebnf('(* iso comment *)\na = b ;'),
    ebnf('a = b (* iso *) c ;'),
    ebnf('a = (* iso *) ;'),
    ebnf('a = ( * b ) ;'),
    ebnf('a = (*b*) ;'),
    ebnf('a = (* unclosed\nb ;'),
    ebnf('a = (* multi\nline *) b ;'),
    ebnf('a = b ; (* after *)'),
    ebnf('a = /* block */ b ;'),
    ebnf('a = b ; %% comment\nc = d ;'),
    ebnf('a = b %% comment ;\n;'),
    ebnf('a = (*) b ;'),
    ebnf('a = (**) b ;'),
    ebnf('a = b *) ;'),
    // names
    ebnf('title = b ;'),
    ebnf('titles = b ;'),
    ebnf('a = title ;'),
    ebnf('a = b , title ;'),
    ebnf('a = accTitle ;'),
    ebnf('accDescr = b ;'),
    ebnf('railroad-ebnf-beta = b ;'),
    ebnf('a = railroad-ebnf-beta ;'),
    ebnf('a = railroad-ebnf-betaX ;'),
    ebnf('a = b ;\na = c ;'),
    ebnf('é = b ;'),
    ebnf('a = é ;'),
    ebnf('a = b . c ;'),
    ebnf('a = b ! c ;'),
    ebnf('a = b & c ;'),
    ebnf('a = 3 * b ;'),
    ebnf('a = b = c ;'),
    ebnf('a = "#35;" ;'),
    // larger texts
    ebnf('title "EBNF"\naccTitle: A\naccDescr: D\nexpression = term , { ( "+" | "-" ) , term } ;\nterm = factor , { ( "*" | "/" ) , factor } ;\nfactor = number | "(" , expression , ")" | ? any ? ;\nnumber = digit+ ;\ndigit = "0" | "1" ;\n'),
    ebnf('identifier = letter , { letter | digit | "_" } ;\nletter = "a" | "b" - "c" ;\n'),
  ],
  railroadAbnf: [
    ...head('railroad-abnf-beta', ABNF_RULE),
    // the rule and its end: ";" in front of other text on the line is a comment
    abnf('a = b ;'),
    abnf('a = b ;\n'),
    abnf('a = b;\n'),
    abnf('a=b;\n'),
    abnf('a = b ; comment\n'),
    abnf('a = b ; \n'),
    abnf('a = b ;\t\n'),
    abnf('a = b ;\r\n'),
    abnf('a = b ;;\n'),
    abnf('a = b ;\n;\n'),
    abnf('a = b\n;\n'),
    abnf('a = b ; comment\n;\n'),
    abnf('a = b ; c = d ;\n'),
    abnf('a = b ;\nc = d ;\n'),
    abnf('a = b ;\n; a comment line\nc = d ;\n'),
    abnf('; a comment line\na = b ;\n'),
    abnf('a = b\n'),
    abnf('a = ;\n'),
    abnf('a b ;\n'),
    abnf('= b ;\n'),
    abnf(';\n'),
    abnf(';'),
    abnf('a == b ;\n'),
    abnf('a =/ b ;\n'),
    abnf('a ::= b ;\n'),
    abnf('a := b ;\n'),
    abnf('a = b = c ;\n'),
    abnf('a\n=\nb\n;\n'),
    abnf('a = b ;\r\nc = d ;\r\n'),
    abnf('a = b ; %% comment\n'),
    abnf('a = b ;%% comment\n'),
    abnf('a = b %% comment\n;\n'),
    // primaries
    abnf('a = "x" ;\n'),
    abnf('a = "" ;\n'),
    abnf('a = "a b  c" ;\n'),
    abnf('a = "back\\slash" ;\n'),
    abnf('a = "line\nbreak" ;\n'),
    abnf('a = "; not a comment" ;\n'),
    abnf('a = "unclosed ;\n'),
    abnf("a = 'x' ;\n"),
    abnf('a = "Ünïcödé 日本語 😀" ;\n'),
    abnf('a = "x""y" ;\n'),
    abnf('a = %x41 ;\n'),
    abnf('a = %X41 ;\n'),
    abnf('a = %d65 ;\n'),
    abnf('a = %D65 ;\n'),
    abnf('a = %b1010 ;\n'),
    abnf('a = %B1010 ;\n'),
    abnf('a = %x41-5A ;\n'),
    abnf('a = %x41-5a ;\n'),
    abnf('a = %d13.10 ;\n'),
    abnf('a = %d13.10.32 ;\n'),
    abnf('a = %x41-5A.61-7A ;\n'),
    abnf('a = %x41- ;\n'),
    abnf('a = %x41. ;\n'),
    abnf('a = %x41-5A- ;\n'),
    abnf('a = %x ;\n'),
    abnf('a = % ;\n'),
    abnf('a = %q11 ;\n'),
    abnf('a = %xZZ ;\n'),
    abnf('a = %x41G ;\n'),
    abnf('a = %x41 %x42 ;\n'),
    abnf('a = %x41%x42 ;\n'),
    abnf('a = b ;\n'),
    abnf('a = B-c1 ;\n'),
    abnf('a = b- ;\n'),
    abnf('a = b_c ;\n'),
    abnf('a = _b ;\n'),
    abnf('a = 1b ;\n'),
    abnf('a = b1 ;\n'),
    abnf('a = <b> ;\n'),
    abnf('a = ( b ) ;\n'),
    abnf('a = [ b ] ;\n'),
    abnf('a = (b) ;\n'),
    abnf('a = [b] ;\n'),
    abnf('a = ( b / c ) ;\n'),
    abnf('a = [ b c / d ] ;\n'),
    abnf('a = ((b)) ;\n'),
    abnf('a = [[b]] ;\n'),
    abnf('a = ( [ ( b ) ] ) ;\n'),
    abnf('a = () ;\n'),
    abnf('a = [] ;\n'),
    abnf('a = ( b ;\n'),
    abnf('a = b ) ;\n'),
    abnf('a = [ b ;\n'),
    abnf('a = ( b ] ;\n'),
    abnf('a = { b } ;\n'),
    // repeats
    abnf('a = *b ;\n'),
    abnf('a = 1*b ;\n'),
    abnf('a = *1b ;\n'),
    abnf('a = 0*1b ;\n'),
    abnf('a = 1*2b ;\n'),
    abnf('a = 3*5b ;\n'),
    abnf('a = 0*b ;\n'),
    abnf('a = 0*0b ;\n'),
    abnf('a = 3b ;\n'),
    abnf('a = 0b ;\n'),
    abnf('a = 1b ;\n'),
    abnf('a = 007b ;\n'),
    abnf('a = 12345678901234567890123b ;\n'),
    abnf('a = 12345678901234567890123*b ;\n'),
    abnf('a = *12345678901234567890123b ;\n'),
    abnf('a = * b ;\n'),
    abnf('a = 3 b ;\n'),
    abnf('a = 1 * 2 b ;\n'),
    abnf('a = 1* 2b ;\n'),
    abnf('a = 1 *2b ;\n'),
    abnf('a = 1*2*3b ;\n'),
    abnf('a = **b ;\n'),
    abnf('a = 3 3 b ;\n'),
    abnf('a = *3 b ;\n'),
    abnf('a = *"x" ;\n'),
    abnf('a = 2"x" ;\n'),
    abnf('a = 1*%x41 ;\n'),
    abnf('a = *( b c ) ;\n'),
    abnf('a = 1*( b / c ) ;\n'),
    abnf('a = 2[ b ] ;\n'),
    abnf('a = *[ b ] ;\n'),
    abnf('a = * ;\n'),
    abnf('a = 3 ;\n'),
    abnf('a = 3* ;\n'),
    abnf('a = b* ;\n'),
    abnf('a = b*c ;\n'),
    abnf('a = b 3 ;\n'),
    abnf('a = b3 ;\n'),
    abnf('a = b 3c ;\n'),
    abnf('a = *b *c 1*d 2e ;\n'),
    abnf('a = -1b ;\n'),
    abnf('a = 1.5b ;\n'),
    // concatenation and alternation
    abnf('a = b c ;\n'),
    abnf('a = b c d e ;\n'),
    abnf('a = b / c ;\n'),
    abnf('a = b/c/d ;\n'),
    abnf('a = b c / d e / f ;\n'),
    abnf('a = b / ;\n'),
    abnf('a = / b ;\n'),
    abnf('a = b // c ;\n'),
    abnf('a = b / / c ;\n'),
    abnf('a = b | c ;\n'),
    abnf('a = b , c ;\n'),
    abnf('a = "a" "b" %x43 d ;\n'),
    abnf('a = b"x"c ;\n'),
    abnf('a = b(c)[d] ;\n'),
    // names
    abnf('a-b = c ;\n'),
    abnf('A1 = c ;\n'),
    abnf('1a = c ;\n'),
    abnf('a_b = c ;\n'),
    abnf('-a = c ;\n'),
    abnf('title = c ;\n'),
    abnf('titles = c ;\n'),
    abnf('a = title ;\n'),
    abnf('a = b title ;\n'),
    abnf('accTitle = c ;\n'),
    abnf('a = accDescr ;\n'),
    abnf('railroad-abnf-beta = c ;\n'),
    abnf('a = railroad-abnf-beta ;\n'),
    abnf('a = railroad-abnf-betaX ;\n'),
    abnf('a = b ;\na = c ;\n'),
    abnf('é = c ;\n'),
    abnf('a = "#35;" ;\n'),
    abnf('a = b /* c */ ;\n'),
    // larger texts
    abnf('title "ABNF"\naccTitle: A\naccDescr: D\nscheme = ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ) ;\nALPHA = %x41-5A / %x61-7A ;\nDIGIT = %x30-39 ;\ncrlf = %d13.10 ;\nopt = [ scheme ] 2*3DIGIT 4ALPHA ;\n'),
    abnf("title 'single'\na = b ;\n"),
    abnf('title "esc \\"aped\\" \\n"\na = b ;\n'),
  ],
  railroadPeg: [
    ...head('railroad-peg-beta', PEG_RULE),
    // the rule
    peg('A <- B ;'),
    peg('A<-B;'),
    peg('A\n<-\nB\n;\n'),
    peg('A <- B'),
    peg('A <- ;'),
    peg('A B ;'),
    peg('<- B ;'),
    peg('A <- B ;;'),
    peg(';'),
    peg('A <- B ; C <- D ;'),
    peg('A <- B ;C<-D;'),
    peg('A <- B C <- D ;'),
    peg('A <- B ;\r\nC <- D ;\r\n'),
    peg('A = B ;'),
    peg('A < - B ;'),
    peg('A <-- B ;'),
    peg('A < B ;'),
    peg('A - B ;'),
    peg('A -> B ;'),
    peg('A ← B ;'),
    peg('A <- B <- C ;'),
    // primaries
    peg('A <- "x" ;'),
    peg("A <- 'x' ;"),
    peg('A <- "" ;'),
    peg('A <- "say \\"hi\\"\\n\\t\\r\\\\\\x" ;'),
    peg('A <- "line\nbreak" ;'),
    peg('A <- "unclosed ;'),
    peg('A <- "Ünïcödé 日本語 😀" ;'),
    peg('A <- "# not a comment" ;'),
    peg('A <- x ;'),
    peg('A <- x-y_z1 ;'),
    peg('A <- _x ;'),
    peg('A <- 1x ;'),
    peg('A <- x1 ;'),
    peg('A <- . ;'),
    peg('A <- .. ;'),
    peg('A <- . . ;'),
    peg('A <- .* ;'),
    peg('A <- !. ;'),
    peg('A <- B.C ;'),
    peg('A <- ( B ) ;'),
    peg('A <- (B) ;'),
    peg('A <- ( B / C ) ;'),
    peg('A <- ( B C / D ) ;'),
    peg('A <- ((B)) ;'),
    peg('A <- () ;'),
    peg('A <- ( B ;'),
    peg('A <- B ) ;'),
    peg('A <- [a-z] ;'),
    peg('A <- { B } ;'),
    // prefixes and suffixes
    peg('A <- &B ;'),
    peg('A <- !B ;'),
    peg('A <- & B ;'),
    peg('A <- ! B ;'),
    peg('A <- &"x" ;'),
    peg('A <- !"x" ;'),
    peg('A <- &. ;'),
    peg('A <- &( B C ) ;'),
    peg('A <- !( B / C ) ;'),
    peg('A <- &&B ;'),
    peg('A <- !!B ;'),
    peg('A <- &!B ;'),
    peg('A <- !&B ;'),
    peg('A <- & ;'),
    peg('A <- ! ;'),
    peg('A <- B& ;'),
    peg('A <- B! ;'),
    peg('A <- B? ;'),
    peg('A <- B* ;'),
    peg('A <- B+ ;'),
    peg('A <- B ? ;'),
    peg('A <- B * ;'),
    peg('A <- B + ;'),
    peg('A <- B?? ;'),
    peg('A <- B** ;'),
    peg('A <- B++ ;'),
    peg('A <- B?* ;'),
    peg('A <- B*+ ;'),
    peg('A <- ? ;'),
    peg('A <- * ;'),
    peg('A <- + ;'),
    peg('A <- ?B ;'),
    peg('A <- *B ;'),
    peg('A <- +B ;'),
    peg('A <- "x"? ;'),
    peg('A <- .+ ;'),
    peg('A <- ( B C )* ;'),
    peg('A <- ( B / C )+ ;'),
    peg('A <- ( B )? ;'),
    peg('A <- &B* ;'),
    peg('A <- !B+ ;'),
    peg('A <- &B? ;'),
    peg('A <- !"x"* ;'),
    peg('A <- &( B )+ ;'),
    peg('A <- &B !C D* E+ F? ;'),
    peg('A <- B? C* D+ ;'),
    // sequences and choices
    peg('A <- B C ;'),
    peg('A <- B C D E ;'),
    peg('A <- B / C ;'),
    peg('A <- B/C/D ;'),
    peg('A <- B C / D E / F ;'),
    peg('A <- B / ;'),
    peg('A <- / B ;'),
    peg('A <- B // C ;'),
    peg('A <- B / / C ;'),
    peg('A <- B | C ;'),
    peg('A <- B , C ;'),
    peg('A <- "a" "b" "c" ;'),
    peg('A <- "a""b" ;'),
    peg('A <- B"x"C ;'),
    peg('A <- B(C)D ;'),
    // comments
    peg('# a comment\nA <- B ;'),
    peg('A <- B ; # a comment'),
    peg('A <- B # a comment\n;'),
    peg('A <- B # a comment ;'),
    peg('A <- # comment\nB ;'),
    peg('A <- B ;\n#\nC <- D ;'),
    peg('A <- B ; %% comment\nC <- D ;'),
    peg('A <- B /* not a comment */ ;'),
    peg('A <- B ; // not a comment'),
    peg('#35;\nA <- B ;'),
    // names
    peg('title <- B ;'),
    peg('titles <- B ;'),
    peg('A <- title ;'),
    peg('A <- B title ;'),
    peg('accTitle <- B ;'),
    peg('A <- accDescr ;'),
    peg('railroad-peg-beta <- B ;'),
    peg('A <- railroad-peg-beta ;'),
    peg('A <- railroad-peg-betaX ;'),
    peg('A <- B ;\nA <- C ;'),
    peg('é <- B ;'),
    peg('A <- é ;'),
    peg('A-b <- B ;'),
    peg('A <- "#35;" ;'),
    // larger texts
    peg('title "PEG"\naccTitle: A\naccDescr: D\nExpression <- Term (("+" / "-") Term)* ;\nTerm <- Factor (("*" / "/") Factor)* ;\nFactor <- Number / "(" Expression ")" / !"x" . / &Digit Digit ;\nNumber <- Digit+ ;\nDigit <- "0" / "1" ;\n'),
  ],
};

// ---------------------------------------------------------------------------
// Mutated inputs.

// mutate returns a variant of text with one to three random edits.
function mutate(text, rand, snippets) {
  const pick = (list) => list[Math.floor(rand() * list.length)];
  let s = text;
  const edits = 1 + Math.floor(rand() * 3);
  for (let e = 0; e < edits; e++) {
    const at = Math.floor(rand() * (s.length + 1));
    const kind = Math.floor(rand() * 8);
    if (kind === 0) {
      // Delete a few characters.
      s = s.slice(0, at) + s.slice(at + 1 + Math.floor(rand() * 3));
    } else if (kind <= 3) {
      // Insert a snippet.
      s = s.slice(0, at) + pick(snippets) + s.slice(at);
    } else if (kind === 4) {
      // Replace one character with a snippet.
      s = s.slice(0, at) + pick(snippets) + s.slice(at + 1);
    } else if (kind === 5) {
      // Duplicate or delete a line.
      const lines = s.split('\n');
      const i = Math.floor(rand() * lines.length);
      if (rand() < 0.5) {
        lines.splice(i, 0, lines[i]);
      } else {
        lines.splice(i, 1);
      }
      s = lines.join('\n');
    } else if (kind === 6) {
      // Swap two lines.
      const lines = s.split('\n');
      const i = Math.floor(rand() * lines.length);
      const j = Math.floor(rand() * lines.length);
      [lines[i], lines[j]] = [lines[j], lines[i]];
      s = lines.join('\n');
    } else {
      // Cut the end.
      s = s.slice(0, at);
    }
  }
  return s;
}

const COMMON_SNIPPETS = [
  ...['"', '"', "'", '\n', '\n', ' ', ' ', '\t', '\r\n', '%%', '%% c\n', ':', ',', '.', '-', ';', ';', '='],
  ...['+', '*', '?', '{', '}', '[', ']', '(', ')', '(', ')', '/', '|', '&', '!', '<-', '::=', '#', '%'],
  ...['0', '1', '7', '10', '3*5', '*', '1*', '\\', '\\"', '\\n', 'x', 'ab', 'A', 'B', 'c1', '_', '"s"', "'t'"],
  ...['title', 'title T\n', 'title "Q"\n', 'accTitle: A\n', 'accDescr: D\n', 'accDescr {\n d\n}\n'],
  ...['---\n', '%%{init: {}}%%\n', '/*', '*/', '/* c */', '(*', '*)', '(* c *)', '; c\n', '# c\n'],
  ...['\u00a0', '\u2028', 'é', '😀', '? s ?', '%x41', '%d13.10', '"x"', 'a = b ;\n'],
];
const SNIPPETS = {
  railroad: [
    ...COMMON_SNIPPETS,
    ...['railroad-beta', 'sequence', 'choice', 'optional', 'oneOrMore', 'zeroOrMore', 'terminal', 'nonterminal', 'special'],
    ...['sequence(', 'choice(', 'optional(', 'terminal("x")', 'nonterminal("n")', 'special("s")', 'terminal(', ', ', ') ;'],
    ...['sequence(terminal("a"), terminal("b"))', 'zeroOrMore(nonterminal("n"))', 'r = terminal("x") ;\n'],
  ],
  railroadEbnf: [...COMMON_SNIPPETS, 'railroad-ebnf-beta', ' , ', ' | ', '{ x }', '[ x ]', '( x )', ' - y', 'x?', 'x*', 'x+', 'r ::= x ;\n'],
  railroadAbnf: [...COMMON_SNIPPETS, 'railroad-abnf-beta', ' / ', '*( x )', '[ x ]', '1*x', '2x', '*x', ' ;\n', ' ;\n', 'r = x ;\n'],
  railroadPeg: [...COMMON_SNIPPETS, 'railroad-peg-beta', ' / ', '&x', '!x', 'x?', 'x*', 'x+', '( x )', ' . ', 'R <- x ;\n'],
};

// ---------------------------------------------------------------------------

const { preprocessDiagram } = await internals();

function* walk(dir) {
  for (const e of fs.readdirSync(dir).sort()) {
    const p = path.join(dir, e);
    if (fs.statSync(p).isDirectory()) {
      yield* walk(p);
    } else if (p.endsWith('.mmd')) {
      yield p;
    }
  }
}

// The fixtures by diagram type: the text that mermaid gives to the parser,
// and the text of the file without the front matter.
const fixtures = { railroad: [], railroadEbnf: [], railroadAbnf: [], railroadPeg: [] };
for (const file of walk(CORPUS)) {
  const meta = JSON.parse(fs.readFileSync(path.join(REF, path.relative(CORPUS, file)).replace(/\.mmd$/, '.json'), 'utf8'));
  if (!(meta.diagramType in fixtures)) {
    throw new Error(`${file}: unexpected diagram type ${meta.diagramType}`);
  }
  const source = fs.readFileSync(file, 'utf8');
  fixtures[meta.diagramType].push(encodeEntities(preprocessDiagram(source).code.cleaned) + '\n');
  fixtures[meta.diagramType].push(source.replace(/^-{3}\s*[\n\r](.*?)[\n\r]-{3}\s*[\n\r]+/s, ''));
}

// ---------------------------------------------------------------------------
// Random sentences of each grammar: the tokens of a random syntax tree, with
// random text between them. Most of them are valid.

const GENERATED = 500;

function sentences(name, rand) {
  const pick = (list) => list[Math.floor(rand() * list.length)];
  const chance = (p) => rand() < p;
  const count = (max) => 1 + Math.floor(rand() * max);
  const strings = ['"x"', '"0"', "'y'", '""', '"a b"', '"say \\"hi\\""', "'it\\'s'", '"\\n\\t"', '"+"', '";"', '"é😀"', '"("'];
  const ids = ['a', 'b', 'digit', 'Term', 'x-y', 'a_1', 'B2', 'term', 'sequence1', 'titles', 'accTitleX'];
  // gap is the text between two tokens.
  const gap = () => {
    const r = rand();
    if (r < 0.55) return ' ';
    if (r < 0.7) return '';
    if (r < 0.8) return '\n';
    if (r < 0.86) return '  \t';
    if (r < 0.9) return ' %% c\n';
    if (r < 0.93) return '\r\n';
    if (r < 0.96) return name === 'railroadPeg' ? ' # c\n' : name === 'railroadAbnf' ? '\n' : ' /* c */ ';
    return name === 'railroadEbnf' ? ' (* c *) ' : '\n\n';
  };
  const join = (tokens) => {
    let text = '';
    for (const t of tokens) {
      text += t + gap();
    }
    return text;
  };

  const gen = {
    railroad: () => {
      const expr = (depth, out) => {
        const kinds = depth <= 0 ? ['terminal', 'nonterminal', 'special'] : ['terminal', 'nonterminal', 'special', 'sequence', 'choice', 'optional', 'oneOrMore', 'zeroOrMore', 'sequence', 'choice'];
        const kind = pick(kinds);
        out.push(kind, '(');
        if (kind === 'terminal' || kind === 'nonterminal' || kind === 'special') {
          out.push(pick(strings));
        } else if (kind === 'sequence' || kind === 'choice') {
          const n = count(4);
          for (let i = 0; i < n; i++) {
            if (i > 0) out.push(',');
            expr(depth - 1, out);
          }
        } else {
          expr(depth - 1, out);
        }
        out.push(')');
      };
      const out = [];
      const n = count(4);
      for (let i = 0; i < n; i++) {
        out.push(pick(ids), '=');
        expr(Math.floor(rand() * 4), out);
        out.push(';');
      }
      return out;
    },
    railroadEbnf: () => {
      const primary = (depth, out) => {
        const r = rand();
        if (depth <= 0 || r < 0.35) {
          out.push(pick(ids));
        } else if (r < 0.55) {
          out.push(pick(strings));
        } else if (r < 0.62) {
          out.push(pick(['? special ?', '?any char?', '? x  y ?']));
        } else {
          const [open, close] = pick([['(', ')'], ['[', ']'], ['{', '}']]);
          out.push(open);
          choice(depth - 1, out);
          out.push(close);
        }
      };
      const term = (depth, out) => {
        primary(depth, out);
        while (chance(0.25)) {
          if (chance(0.3)) {
            out.push('-');
            primary(depth - 1, out);
          } else {
            out.push(pick(['?', '*', '+']));
          }
        }
      };
      const sequence = (depth, out) => {
        const n = count(3);
        for (let i = 0; i < n; i++) {
          if (i > 0 && chance(0.6)) out.push(',');
          term(depth, out);
        }
      };
      const choice = (depth, out) => {
        const n = count(3);
        for (let i = 0; i < n; i++) {
          if (i > 0) out.push('|');
          sequence(depth, out);
        }
      };
      const out = [];
      const n = count(4);
      for (let i = 0; i < n; i++) {
        out.push(pick(ids), pick(['=', '::=']));
        choice(Math.floor(rand() * 3), out);
        out.push(';');
      }
      return out;
    },
    railroadAbnf: () => {
      const names = ['a', 'b', 'ALPHA', 'DIGIT', 'x-y', 'B2', 'titles'];
      const primary = (depth, out) => {
        const r = rand();
        if (depth <= 0 || r < 0.4) {
          out.push(pick(names));
        } else if (r < 0.55) {
          out.push(pick(['"x"', '""', '"a b"', '"+"', '"é"', '"\\"']));
        } else if (r < 0.68) {
          out.push(pick(['%x41', '%x41-5A', '%d13.10', '%b1010', '%X30-39.41']));
        } else {
          const [open, close] = pick([['(', ')'], ['[', ']']]);
          out.push(open);
          alternation(depth - 1, out);
          out.push(close);
        }
      };
      const element = (depth, out) => {
        if (chance(0.3)) out.push(pick(['*', '1*', '*1', '0*1', '1*2', '3', '0', '2*', '10*20', '007']));
        primary(depth, out);
      };
      const concatenation = (depth, out) => {
        const n = count(3);
        for (let i = 0; i < n; i++) {
          element(depth, out);
        }
      };
      const alternation = (depth, out) => {
        const n = count(3);
        for (let i = 0; i < n; i++) {
          if (i > 0) out.push('/');
          concatenation(depth, out);
        }
      };
      const out = [];
      const n = count(4);
      for (let i = 0; i < n; i++) {
        out.push(pick(names), '=');
        alternation(Math.floor(rand() * 3), out);
        // The end of a rule is the last text of its line.
        out.push(chance(0.9) ? ';\n' : ';');
      }
      return out;
    },
    railroadPeg: () => {
      const primary = (depth, out) => {
        const r = rand();
        if (depth <= 0 || r < 0.4) {
          out.push(pick(ids));
        } else if (r < 0.6) {
          out.push(pick(strings));
        } else if (r < 0.7) {
          out.push('.');
        } else {
          out.push('(');
          choice(depth - 1, out);
          out.push(')');
        }
      };
      const prefix = (depth, out) => {
        if (chance(0.2)) out.push(pick(['&', '!']));
        primary(depth, out);
        if (chance(0.3)) out.push(pick(['?', '*', '+']));
      };
      const sequence = (depth, out) => {
        const n = count(3);
        for (let i = 0; i < n; i++) {
          prefix(depth, out);
        }
      };
      const choice = (depth, out) => {
        const n = count(3);
        for (let i = 0; i < n; i++) {
          if (i > 0) out.push('/');
          sequence(depth, out);
        }
      };
      const out = [];
      const n = count(4);
      for (let i = 0; i < n; i++) {
        out.push(pick(ids), '<-');
        choice(Math.floor(rand() * 3), out);
        out.push(';');
      }
      return out;
    },
  };

  const keyword = { railroad: 'railroad-beta', railroadEbnf: 'railroad-ebnf-beta', railroadAbnf: 'railroad-abnf-beta', railroadPeg: 'railroad-peg-beta' }[name];
  const texts = [];
  for (let i = 0; i < GENERATED; i++) {
    let text = keyword + pick(['\n', '\n', ' ', '\n\n', ' %% c\n']);
    if (chance(0.2)) {
      text += pick(['title T\n', 'title "Q \\"x\\""\n', "title 'S'\n", 'accTitle: A\n', 'accDescr: D\n', 'accDescr {\n d\n e\n}\n', 'title T\naccTitle: A\naccDescr: D\n']);
    }
    texts.push(text + join(gen[name]()));
  }
  return texts;
}

// ---------------------------------------------------------------------------

const out = {};
const summary = [];
for (const name of Object.keys(GRAMMARS)) {
  const own = fixtures[name];
  // The fixtures of the other grammars are inputs too: most of them fail.
  const foreign = Object.keys(fixtures)
    .filter((n) => n !== name)
    .flatMap((n) => fixtures[n].filter((_, i) => i % 2 === 0));
  const rand = prng(0x72720000 ^ name.length ^ (name.charCodeAt(8) << 8));
  const generated = sentences(name, rand);
  const texts = [...own, ...HAND[name], ...foreign, ...generated];
  const valid = (t) => run(name, t).ast !== undefined;
  const seeds = [...own, ...own, ...HAND[name].filter(valid), ...generated.filter(valid)];
  const seen = new Set(texts);
  for (let i = 0; i < MUTANTS; i++) {
    const text = mutate(seeds[Math.floor(rand() * seeds.length)], rand, SNIPPETS[name]);
    if (seen.has(text)) {
      continue;
    }
    seen.add(text);
    texts.push(text);
  }
  const cases = [...new Set(texts)].map((text) => run(name, text));
  const rejected = cases.filter((c) => c.error).length;
  out[name] = { fixtures: own.length, cases };
  summary.push(`${name}: ${cases.length} cases, ${rejected} that the parser rejects, ${own.length} from fixtures`);
}

fs.mkdirSync(path.dirname(OUT), { recursive: true });
// The gzip header has no time stamp, so the same data gives the same file.
fs.writeFileSync(OUT, zlib.gzipSync(Buffer.from(JSON.stringify(out)), { level: 9 }));
console.log(summary.join('\n'));
console.log(`${path.relative(ROOT, OUT)}: ${(fs.statSync(OUT).size / 1024).toFixed(1)} kB`);
