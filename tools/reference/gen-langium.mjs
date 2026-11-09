#!/usr/bin/env node
// Generates the Langium data of the Go port from the real @mermaid-js/parser
// (the Langium 4.4.0 and Chevrotain 13.2.0 that mermaid 12.2.0 bundles):
//
//   internal/langium/vocab_gen.go
//       The token types of every Langium grammar, in the order of the
//       Chevrotain lexer, as the token builder of the grammar returns them.
//   internal/langium/testdata/tokens.json.gz
//       Lexer goldens of every grammar: corpus fixtures, hand-written inputs
//       and mutated inputs, with the tokens, the hidden tokens and the errors
//       of the real lexer.
//   internal/langium/testdata/convert.json.gz
//       Value converter goldens: the texts of the tokens of the lexer golden
//       and hand-written texts, with the value of the real converter.
//   internal/diagrams/<name>/testdata/ast.json.gz  (info, pie, packet, radar)
//       Parser goldens: the AST of the real parser, or the fact that it fails.
//
// Run it with tools/reference as the working directory:
//
//   node gen-langium.mjs
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import * as P from '@mermaid-js/parser';
import { parseRegex, simplify, normalize, goString, prng } from './jison-lib.mjs';
import { internals } from './lib-mermaid-internals.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');
const CORPUS = path.join(ROOT, 'testdata/corpus/mermaid-e2e');

// The grammars, by the name that parse(diagramType, text) of the parser
// package takes. family is the corpus directory with fixtures of the grammar.
const GRAMMARS = [
  { name: 'info', services: () => P.createInfoServices().Info, family: 'info' },
  { name: 'packet', services: () => P.createPacketServices().Packet, family: 'packet' },
  { name: 'pie', services: () => P.createPieServices().Pie, family: 'pie' },
  { name: 'treeView', services: () => P.createTreeViewServices().TreeView, family: 'tree-view' },
  {
    name: 'architecture',
    services: () => P.createArchitectureServices().Architecture,
    family: 'architecture',
  },
  { name: 'gitGraph', services: () => P.createGitGraphServices().GitGraph, family: 'git' },
  {
    name: 'eventmodeling',
    services: () => P.createEventModelingServices().EventModel,
    family: 'eventmodeling',
  },
  { name: 'radar', services: () => P.createRadarServices().Radar, family: 'radar' },
  { name: 'railroad', services: () => P.createRailroadServices().Railroad, family: 'railroad' },
  {
    name: 'railroadEbnf',
    services: () => P.createRailroadEbnfServices().RailroadEbnf,
    family: 'railroad',
  },
  {
    name: 'railroadAbnf',
    services: () => P.createRailroadAbnfServices().RailroadAbnf,
    family: 'railroad',
  },
  {
    name: 'railroadPeg',
    services: () => P.createRailroadPegServices().RailroadPeg,
    family: 'railroad',
  },
  { name: 'treemap', services: () => P.createTreemapServices().Treemap, family: 'treemap' },
  { name: 'wardley', services: () => P.createWardleyServices().Wardley, family: 'wardley' },
  { name: 'cynefin', services: () => P.createCynefinServices().Cynefin, family: 'cynefin' },
];

// The grammars that have a parser golden, with the number of mutated inputs.
const AST_GRAMMARS = { info: 300, pie: 700, packet: 700, radar: 1200 };
// The number of mutated inputs in the lexer golden of each grammar.
const TOKEN_MUTANTS = 120;

// ---------------------------------------------------------------------------
// Vocabulary

// SKIPPED is the group of Chevrotain for a token that the lexer drops.
const SKIPPED_PREFIX = 'This marks a skipped Token pattern';

// goPattern prints a JavaScript regular expression as a pattern for regexp2
// without options. Every one-unit matcher is an explicit set of UTF-16 code
// units that V8 computed, so the meaning does not depend on the engine.
function goPattern(re) {
  if (/[^dgy]/.test(re.flags)) {
    throw new Error(`regex ${re}: unsupported flags`);
  }
  const tree = simplify(parseRegex(re.source), re.flags);
  const hasWordBoundary = (n) =>
    n.t === 'assert'
      ? n.kind === 'b' || n.kind === 'B'
      : (n.alts ?? n.items ?? (n.body ? [n.body] : [])).some(hasWordBoundary);
  if (hasWordBoundary(tree)) {
    // \b means a Unicode word boundary in regexp2 without the ECMAScript option.
    throw new Error(`regex ${re}: \\b is not supported`);
  }
  // "$" is "no code unit follows". "[^]" is not a class in regexp2.
  return normalize(tree, re.flags, { end: '(?![\\u0000-\\uffff])' }).replaceAll(
    '[^]',
    '[\\u0000-\\uffff]'
  );
}

function vocabulary(services) {
  const built = services.parser.TokenBuilder.buildTokens(services.Grammar, {
    caseInsensitive: services.LanguageMetaData.caseInsensitive,
  });
  if (!Array.isArray(built)) {
    throw new Error('multi-mode lexer definitions are not supported');
  }
  const index = new Map(built.map((t, i) => [t, i]));
  return built.map((t) => {
    const out = { name: t.name };
    if (typeof t.PATTERN === 'string') {
      out.literal = t.PATTERN;
    } else if (t.PATTERN instanceof RegExp) {
      out.source = t.PATTERN.source;
      out.pattern = goPattern(t.PATTERN);
    } else {
      throw new Error(`token ${t.name}: custom patterns are not supported`);
    }
    if (t.GROUP !== undefined) {
      out.group = t.GROUP.startsWith(SKIPPED_PREFIX) ? 'skipped' : t.GROUP;
      if (out.group !== 'skipped' && out.group !== 'hidden') {
        throw new Error(`token ${t.name}: unknown group ${t.GROUP}`);
      }
    }
    if (t.PUSH_MODE !== undefined || t.POP_MODE !== undefined || t.CATEGORIES?.length) {
      throw new Error(`token ${t.name}: modes and categories are not supported`);
    }
    const alts = t.LONGER_ALT === undefined ? [] : [].concat(t.LONGER_ALT);
    if (alts.length > 0) {
      out.longerAlt = alts.map((a) => index.get(a));
    }
    return out;
  });
}

// goRaw prints a Go string literal, as a raw string where that is possible.
const goRaw = (text) => (/[`\n\r]/.test(text) ? goString(text) : '`' + text + '`');

function vocabFile(all) {
  let s = '// Code generated by tools/reference/gen-langium.mjs. DO NOT EDIT.\n\n';
  s += 'package langium\n\n';
  s += '// vocabularies holds the token types of every Langium grammar of the\n';
  s += '// parser package of mermaid 12.2.0, in the order of the Chevrotain lexer.\n';
  s += 'var vocabularies = map[string][]tokenSpec{\n';
  for (const { name, vocab } of all) {
    s += `\t${goString(name)}: {\n`;
    for (const t of vocab) {
      const fields = [`name: ${goString(t.name)}`];
      if (t.literal !== undefined) {
        fields.push(`literal: ${goString(t.literal)}`);
      } else {
        fields.push(`source: ${goRaw(t.source)}`, `pattern: ${goRaw(t.pattern)}`);
      }
      if (t.group === 'skipped') {
        fields.push('group: Skipped');
      } else if (t.group === 'hidden') {
        fields.push('group: Hidden');
      }
      if (t.longerAlt) {
        fields.push(`longerAlt: []int{${t.longerAlt.join(', ')}}`);
      }
      s += `\t\t{${fields.join(', ')}},\n`;
    }
    s += '\t},\n';
  }
  s += '}\n';
  return s;
}

// ---------------------------------------------------------------------------
// Inputs

function listFixtures(family) {
  const dir = path.join(CORPUS, family);
  if (!fs.existsSync(dir)) {
    return [];
  }
  return fs
    .readdirSync(dir, { recursive: true })
    .filter((f) => f.endsWith('.mmd'))
    .sort()
    .map((f) => fs.readFileSync(path.join(dir, f), 'utf8'));
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

// Hand-written inputs, by grammar. They cover the branches of the grammar
// and of the value converters.
const HAND = {
  info: [
    'info',
    'info\n',
    '\n\n  info  \n\n',
    'info showInfo',
    'info\nshowInfo\n',
    'info showInfo\n\ntitle T\n',
    'info\ntitle T\naccTitle: A\naccDescr: D\n',
    'info\ntitle T\n\n\naccTitle: A',
    'info\naccDescr {\n  multi\n   line\n}\n',
    'info showInfo title T',
    'info title',
    'info\ntitle\n',
    'infoshowInfo',
    'info showInfox',
    'info %% comment\n',
    'info%%c\nshowInfo%%d\n',
    '%% c\ninfo\n',
    '---\ntitle: x\n---\ninfo\n',
    '%%{init: {"theme":"dark"}}%%\ninfo\n',
    'info\nshowInfo\nshowInfo\n',
    'info\ntitle a\ntitle b\n',
    'info\ntitle a\nshowInfo\n',
    'info\n\ntitle a\n',
    'info\nfoo\n',
    '',
    '\n',
    'showInfo',
    'info info',
    'Info',
    'info\taccTitle:x',
    'info\r\nshowInfo\r\ntitle x\r\n',
    'info\u00a0showInfo',
    'info\u00a0',
    'info\ntitle  a   b \t c  \n',
    'info\naccTitle:   a   b  %% c\n',
    'info\naccDescr:  a  %% c\n',
    'info\naccDescr{a}\n',
    'info\naccDescr\n\n{a\n\n\n b \t\t c  \n  }\n',
    'info\naccDescr {a} title x\n',
    'info\naccDescr {\r\n  a\r\n\r\n  b\r\n}\r\n',
    'info\naccDescr {\u2028 a \u2028\u2028 b \u00a0}\n',
    'info\naccDescr : x\n',
    'info\naccTitle x\n',
    'info\naccDescr\n',
    'info\n  title  x\n',
    'info\ntitlex\n',
    'info\ntitle\tx\n',
  ],
  pie: [
    'pie',
    'pie\n',
    'pie showData',
    'pie showData\n',
    'pie title Pets\n"Dogs": 386\n"Cats": 85\n"Rats": 15\n',
    'pie showData title X\n"a": 1\n',
    'pie\n"a": 1',
    'pie\n"a": 1\n"a": 2\n',
    'pie\n"a" : 1.5\n\'b\':2\n',
    'pie\n"a": -1\n',
    'pie\n"a": -0\n',
    'pie\n"a": -0.0\n',
    'pie\n"a": 0\n',
    'pie\n"a": 00\n',
    'pie\n"a": 01\n',
    'pie\n"a": 1.\n',
    'pie\n"a": 1..2\n',
    'pie\n"a": .5\n',
    'pie\n"a": 1.5.2\n',
    'pie\n"a": 1e3\n',
    'pie\n"a": 10.50\n',
    'pie\n"a": 123456789012345678901234567890\n',
    'pie\n"a": 0.1234567890123456789\n',
    'pie\n"a" 1\n',
    'pie\n"a":\n',
    'pie\n"a": 1 "b": 2\n',
    'pie\n"a\\"b\\n\\t\\\\c\\x": 1\n',
    'pie\n"a\\\nb": 1\n',
    'pie\n"": 1\n',
    "pie\n'it\\'s': 1\n",
    'pie\n"a\nb": 1\n',
    'pie\n"unterminated: 1\n',
    'pie\n  "a": 1 %% comment\n  "b": 2%%c\n',
    'pie %% c\n"a": 1\n',
    'pie%%c\n"a": 1\n',
    'piex\n',
    'pie"a": 1\n',
    'pie showDatax\n',
    'pie showData"a":1\n',
    'pie showData showData\n',
    'pie\nshowData\n',
    'pie title\n"a": 1\n',
    'pie title A\ntitle B\n"a": 1\n',
    'pie\ntitle A\naccTitle: T\naccDescr: D\n"a": 1\n',
    'pie\naccDescr {\n a\n b\n}\n"a": 1\n',
    'pie\n"a": 1\ntitle late\n',
    'pie\n"a": 1\n\n\n"b": 2\n\n',
    'pie\r\n"a": 1\r\n"b": 2\r\n',
    '\n\n\npie\n',
    '%%{init: {}}%%\npie\n"a": 1\n',
    '---\nconfig:\n  pie:\n    textPosition: 0.5\n---\npie\n"a": 1\n',
    'pie\n"😀 é": 3\n',
    'pie\n"a": 1,\n',
    'pie\n"a":1"b":2',
    'pie title "x": 2\n',
    'pie\n title x %% c\n"a": 5\n',
    'pie\n"a": 5 title x\n',
    'pie\n"a": 5\ttitle x\n',
    'pie\n"a": 5 accTitle: x\n',
    'pie\n"a": - 5\n',
    'pie\n"a": --5\n',
    'pie\n"a": +5\n',
    'pie\n"a": 5-\n',
    'pie\n"a": 0x10\n',
    'pie\n"a": 5 6\n',
    'pie\u00a0showData\n',
    'pie\n"a": 1\u00a0\n',
    'PIE\n',
    'pie\n: 1\n',
    'pie\n"a": "b"\n',
    'pie title a\n\n\n\n',
    'pie title a',
    'pie accTitle: a',
    'pie accDescr {a}',
    'pie accDescr {a} "x": 1',
    'pie\n"a": 1\naccDescr {a}"x": 1',
  ],
  packet: [
    'packet',
    'packet\n',
    'packet-beta',
    'packet-beta\n',
    'packet\n0-10: "hello"\n',
    'packet-beta\n0-10: "hello"\n11: "x"\n',
    'packet\n+8: "a"\n+16: "b"\n',
    'packet\n0-15: "a"\n+16: "b"\n32-47: "c"\n',
    'packet\ntitle Hello\n0-10: "hello"\n',
    'packet title Hello\n0-10: "hello"\n',
    'packet\naccTitle: T\naccDescr: D\n0: "x"\n',
    'packet\n0 - 10 : "a"\n',
    'packet\n0-: "a"\n',
    'packet\n-5: "a"\n',
    'packet\n0-10 "a"\n',
    'packet\n0-10:\n',
    'packet\n0-10: a\n',
    "packet\n0-10: 'a'\n",
    'packet\n0-10: "a" 11-20: "b"\n',
    'packet\n0-10: "a"',
    'packet\n00: "a"\n',
    'packet\n01: "a"\n',
    'packet\n1.5: "a"\n',
    'packet\n1.: "a"\n',
    'packet\n+ 8: "a"\n',
    'packet\n+: "a"\n',
    'packet\n++8: "a"\n',
    'packet\n+8-9: "a"\n',
    'packet\n8+9: "a"\n',
    'packet\n0-10: "a\\"b"\n',
    'packet\n0-10: ""\n',
    'packetx\n',
    'packet-betax\n',
    'packet-beta0: "a"\n',
    'packet0: "a"\n',
    'packet -beta\n',
    'packet-\n',
    'packet--\n',
    'packet\n---\n',
    'packet\n---\nfoo\n---\n0: "a"\n',
    'packet\n0---5: "a"\n',
    'packet\n0--5: "a"\n',
    'packet\n0-\n5: "a"\n',
    'packet %% c\n0: "a" %% d\n',
    'packet\n123456789012345678901234567890: "a"\n',
    'packet\n0-10: "a"\n\n\n11: "b"\n\n',
    'packet\r\n0-10: "a"\r\n',
    '\n\npacket\n',
    'packet beta\n',
    'packet\n0-10: "a"\ntitle x\n',
    'packet\n0-10: "a" title x\n',
    'packet-beta title x\n',
    'packet-betatitle x\n',
    'packet\n10-0: "a"\n',
    'packet\n0: "a"\n0: "b"\n',
    'packet\n+0: "a"\n',
    'packet\n0-0: "a"\n',
    'packet\n0:"a"\n1:"b"',
    'packet\naccDescr {\n x\n}\n0: "a"\n',
    '---\ntitle: x\n---\npacket\n0: "a"\n',
    'packet\n1e3: "a"\n',
    'packet\n0x1: "a"\n',
    'packet\n٣: "a"\n',
  ],
  radar: [
    'radar-beta',
    'radar-beta\n',
    'radar-beta:\n',
    'radar-beta :\n',
    'radar-beta: :\n',
    'radar-beta\n:\n',
    'radar-beta\naxis A, B, C\ncurve c1{1,2,3}\n',
    'radar-beta\n  title Radar\n  axis A["Axis A"], B, C["C c"]\n  curve mycurve["My Curve"]{1,2,3}\n',
    'radar-beta\naxis A,B,C\ncurve c1{ A: 1, B: 2, C: 3 }\n',
    'radar-beta\naxis A,B,C\ncurve c1{ A 1, B 2, C 3 }\n',
    'radar-beta\naxis A,B,C\ncurve c1{ C: 3, A: 1, B: 2 }\n',
    'radar-beta\naxis A,B,C\ncurve c1{ A: 1, 2, C: 3 }\n',
    'radar-beta\naxis A,B,C\ncurve c1{ 1, B: 2, 3 }\n',
    'radar-beta\naxis A,B,C\ncurve c1{\n 1,\n 2,\n 3\n}\n',
    'radar-beta\naxis A,B,C\ncurve c1{\n\n A: 1,\n\n B: 2,\n C: 3\n\n}\n',
    'radar-beta\naxis A,B,C\ncurve c1{1,2,3}, c2{4,5,6}\n',
    'radar-beta\naxis A,B,C\ncurve c1{1,2,3},\n c2{4,5,6}\n',
    'radar-beta\naxis A,\n B\n',
    'radar-beta\naxis A B\n',
    'radar-beta\naxis\n',
    'radar-beta\naxis A,\n',
    'radar-beta\ncurve c1{}\n',
    'radar-beta\ncurve c1{1,}\n',
    'radar-beta\ncurve c1{,1}\n',
    'radar-beta\ncurve c1{1 2}\n',
    'radar-beta\ncurve c1{1\n,2}\n',
    'radar-beta\ncurve c1{1,2\n}\n',
    'radar-beta\ncurve c1 {1,2}\n',
    'radar-beta\ncurve c1\n{1,2}\n',
    'radar-beta\ncurve c1["x"] {1,2}\n',
    'radar-beta\ncurve c1[x]{1,2}\n',
    "radar-beta\ncurve c1['x']{1,2}\n",
    'radar-beta\ncurve c1[]{1,2}\n',
    'radar-beta\ncurve c1["a"]["b"]{1,2}\n',
    'radar-beta\ncurve {1,2}\n',
    'radar-beta\ncurve 1{1,2}\n',
    'radar-beta\ncurve c-1{1,2}\n',
    'radar-beta\ncurve c-{1,2}\n',
    'radar-beta\ncurve _c{1.5,2.25,0,00,1.}\n',
    'radar-beta\ncurve c{1.5,2.25}\n',
    'radar-beta\ncurve c{-1}\n',
    'radar-beta\ncurve c{1e3}\n',
    'radar-beta\ncurve c{1..2}\n',
    'radar-beta\ncurve c{01}\n',
    'radar-beta\ncurve c{1}{2}\n',
    'radar-beta\ncurve c{1} curve d{2}\n',
    'radar-beta\ncurve c{1}\ncurve d{2}\naxis Z\n',
    'radar-beta\nshowLegend true\n',
    'radar-beta\nshowLegend false\n',
    'radar-beta\nshowLegend True\n',
    'radar-beta\nshowLegend 1\n',
    'radar-beta\nshowLegend\n',
    'radar-beta\nticks 5\n',
    'radar-beta\nticks 5.5\n',
    'radar-beta\nticks 40\n',
    'radar-beta\nticks five\n',
    'radar-beta\nmax 10\nmin 1\n',
    'radar-beta\nmax 10, min 1, ticks 3, graticule polygon, showLegend false\n',
    'radar-beta\nmax 10,\n min 1\n',
    'radar-beta\nmax 10 min 1\n',
    'radar-beta\ngraticule circle\n',
    'radar-beta\ngraticule polygon\n',
    'radar-beta\ngraticule square\n',
    'radar-beta\ngraticule circles\n',
    'radar-beta\ngraticule\n',
    'radar-beta\nmax 10\nmax 20\n',
    'radar-beta\nmax -1\n',
    'radar-beta\nmaximum 10\n',
    'radar-beta\nmax10\n',
    'radar-beta\naxis max\n',
    'radar-beta\naxis axis\n',
    'radar-beta\naxis maxi, mini, axis1, curves, ticks2, true1, circle2\n',
    'radar-beta\naxis true\n',
    'radar-beta\naxis circle\n',
    'radar-beta\naxis title\n',
    'radar-beta\naxis title1\n',
    'radar-beta\naxis atitle, accTitle1\n',
    'radar-beta\naxis A, title\n',
    'radar-beta\naxis 1a, 2, 3.5\n',
    'radar-beta\naxis a.b\n',
    'radar-beta\naxis é\n',
    'radar-beta\naxis A_b-c, d--e, f-\n',
    'radar-beta axis A\n',
    'radar-beta: axis A\n',
    'radar-beta:axis A\n',
    'radar-beta title X\naxis A\n',
    'radar-beta\ntitle X\naccTitle: T\naccDescr: D\n',
    'radar-beta\naccDescr {\n a\n}\naxis A\n',
    'radar-betax\n',
    'radar-beta-x\n',
    'radar-beta:x\n',
    'radar\n',
    'radar-beta %% c\naxis A %% d\n',
    'radar-beta%%c\n',
    'radar-beta\naxis A["a\\"b"]\n',
    'radar-beta\naxis A[ "a" ]\n',
    'radar-beta\naxis A ["a"]\n',
    'radar-beta\naxis A[\n"a"]\n',
    'radar-beta\naxis A, A\n',
    'radar-beta\ncurve c{A: 1}\n',
    'radar-beta\naxis A\ncurve c{B: 1}\n',
    'radar-beta\naxis A,B\ncurve c{A: 1}\n',
    'radar-beta\naxis A,B\ncurve c{A: 1, A: 2, B: 3}\n',
    'radar-beta\naxis A,B\ncurve c{A:: 1}\n',
    'radar-beta\naxis A,B\ncurve c{A : 1 , B : 2}\n',
    'radar-beta\naxis A,B\ncurve c{max: 1, B: 2}\n',
    'radar-beta\ncurve c{1,2}\naxis A,B\n',
    'radar-beta\ncurve c{A: 1,B: 2}\naxis A,B\n',
    'radar-beta\r\naxis A,B\r\ncurve c{1,2}\r\n',
    '\n\nradar-beta\n\n\naxis A\n\n',
    'radar-beta\naxis A',
    'radar-beta\ncurve c{1}',
    'radar-beta\nmax 5',
    'radar-beta\naxis A;\n',
    'radar-beta\naxis A,,B\n',
    'radar-beta\n, axis A\n',
    'radar-beta\ncurve c{1,\n\n2,\n3\n\n}\n',
    'radar-beta\ncurve c{\n1\n\n,2}\n',
    'radar-beta\ncurve c{A\n: 1}\n',
    'radar-beta\ncurve c{A:\n1}\n',
    'radar-beta\ncurve c{1}, \n\n d{2}\n',
    'radar-beta\nmin 0.5, max 1.5\ncurve c{0.75}\n',
    '---\ntitle: x\n---\nradar-beta\naxis A\n',
    'radar-beta\naxis showLegend\n',
    'radar-beta\naxis A\nshowLegend true, ticks 4\ncurve c{1}\ngraticule polygon\n',
  ],
};

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

// snippetsOf returns the texts that mutate inserts for a grammar: its
// literal tokens and keywords, and characters that the terminals care about.
function snippetsOf(vocab) {
  const words = vocab
    .filter((t) => t.literal !== undefined || /^[\w:-]+\(\?:\(\?=%%\)/.test(t.source))
    .map((t) => t.name);
  return [
    ...words,
    ...words,
    ...['"', '"', "'", '\n', '\n', ' ', ' ', '\t', '\r\n', '%%', '%% c\n', ':', ',', '.', '-'],
    ...['+', '{', '}', '[', ']', '(', ')', '0', '1', '7', '10', '1.5', '0.25', '-3', '\\', '\\"'],
    ...['title', 'title T\n', 'accTitle: A\n', 'accDescr: D\n', 'accDescr {\n d\n}\n', 'x', 'ab'],
    ...['_', 'true', 'false', '---\n', '%%{init: {}}%%\n', '\u00a0', '\u2028', 'é', '😀', '"s"'],
    ...['circle', 'polygon', 'A', 'B', 'c1'],
  ];
}

// ---------------------------------------------------------------------------
// Goldens

function writeGolden(file, data) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  // The gzip header has no time stamp, so the same data gives the same file.
  fs.writeFileSync(file, zlib.gzipSync(JSON.stringify(data), { level: 9 }));
  console.log(`${path.relative(ROOT, file)}: ${(fs.statSync(file).size / 1024).toFixed(1)} kB`);
}

// lex runs the real lexer and returns compact lists: a token is
// [type index, start offset, length], an error is [offset, length].
function lex(services, typeIndex, text) {
  const r = services.parser.Lexer.tokenize(text);
  const pack = (t) => [typeIndex.get(t.tokenType.name), t.startOffset, t.image.length];
  return {
    text,
    tokens: r.tokens.map(pack),
    hidden: r.hidden.map(pack),
    errors: r.errors.map((e) => [e.offset, e.length]),
  };
}

// plain turns an AST into JSON: it drops the links to the CST and to the
// container, prints a cross-reference as {"$ref": text}, and keeps -0.
function plain(node) {
  if (Array.isArray(node)) {
    return node.map(plain);
  }
  if (typeof node === 'number' && Object.is(node, -0)) {
    return { $n: '-0' };
  }
  if (node === null || typeof node !== 'object') {
    return node;
  }
  if ('$refText' in node) {
    return { $ref: node.$refText };
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

async function ast(name, text) {
  try {
    return { text, ast: plain(await P.parse(name, text)) };
  } catch (e) {
    return { text, error: String(e?.message ?? e) };
  }
}

const unique = (list) => [...new Set(list)];

// The terminal rules that have a converter golden, by grammar.
const CONVERT_RULES = {
  radar: ['ACC_DESCR', 'ACC_TITLE', 'TITLE', 'NUMBER', 'STRING', 'ID', 'BOOLEAN', 'GRATICULE'],
  packet: ['INT'],
  pie: ['NUMBER_PIE', 'STRING'],
  gitGraph: ['STRING', 'INT'],
};

// Hand-written token texts for the converter golden, by rule.
const CONVERT_HAND = {
  ACC_DESCR: [
    'accDescr:',
    'accDescr: a',
    '  accDescr \t:  a   b \t\t c  ',
    'accDescr:\u00a0 a \u00a0\u00a0 b \u00a0',
    'accDescr{}',
    'accDescr {a}',
    'accDescr\n\n{\n\n  a  \n\n\n  b\t\tc  \n\n}',
    'accDescr {\r\n  a\r\n\r\n  b\r\n}',
    'accDescr {\r  a\r\r  b\r}',
    'accDescr { a \u2028 b \u2029\u2029 c \u00a0 }',
    'accDescr {\n\n\n}',
    'accDescr {   }',
    'accDescr { \n a\n \n\n b \n }',
    'accDescr {a\n \n b}',
    'accDescr {a  \n\n  \n  b  \n  }',
    'accDescr {\ta\t\n\tb\t}',
    'accDescr {a\n\r\n\rb}',
    'accDescr {a \n\n b \n\r c \r\n d}',
    'accDescr { a:  b }',
    'accDescr: {a}',
    'accDescr : {a  b}',
    'accDescr {\u000b a \u000c\n\ufeff b \u3000}',
    'accDescr {a\n\n\n\nb\n}',
    'accDescr {  a  \n  b  \n\n}',
  ],
  ACC_TITLE: ['accTitle:', 'accTitle: a', ' \taccTitle \t:  a   b \t\t c  ', 'accTitle:\u00a0a\u2003 b'],
  TITLE: ['title', 'title ', 'title a', '  title   a   b \t\t c  ', 'title\ta', 'title \u00a0a\u00a0'],
  STRING: [
    '""',
    "''",
    '"a"',
    '"a\\"b"',
    '"\\b\\f\\n\\r\\t\\v\\0\\x\\\\\\\'"',
    "'\\''",
    '"\\😀😀"',
    '"é\u00a0"',
    '"a\nb"',
  ],
  NUMBER: ['0', '7', '10', '1.5', '0.25', '00.10', '123456789012345678901234567890', '0.1234567890123456789'],
  NUMBER_PIE: ['-0', '-0.0', '-1', '-1.50', '0', '12.5', '-123456789012345678901234567890'],
  INT: ['0', '7', '10', '9007199254740993', '123456789012345678901234567890', '18446744073709551616'],
  ID: ['a', '^a', '^^a', 'a-b', '_'],
  BOOLEAN: ['true', 'false'],
  GRATICULE: ['circle', 'polygon'],
};

// number keeps -0 in JSON.
const number = (v) => (typeof v === 'number' && Object.is(v, -0) ? { $n: '-0' } : v);

function convertGolden(all, tokenGolden) {
  const cases = [];
  for (const g of all) {
    for (const ruleName of CONVERT_RULES[g.name] ?? []) {
      const rule = g.services.Grammar.rules.find((r) => r.name === ruleName);
      const typeIndex = g.vocab.findIndex((t) => t.name === ruleName);
      const inputs = [...(CONVERT_HAND[ruleName] ?? [])];
      for (const c of tokenGolden[g.name]) {
        for (const [type, offset, length] of c.tokens) {
          if (type === typeIndex) {
            inputs.push(c.text.slice(offset, offset + length));
          }
        }
      }
      for (const input of unique(inputs)) {
        const value = g.services.parser.ValueConverter.runConverter(rule, input, undefined);
        cases.push({ grammar: g.name, rule: ruleName, input, value: number(value) });
      }
    }
  }
  return cases;
}

async function main() {
  const { preprocessDiagram } = await internals();
  const all = GRAMMARS.map((g) => {
    const services = g.services();
    return { ...g, services, vocab: vocabulary(services) };
  });

  const vocabPath = path.join(ROOT, 'internal/langium/vocab_gen.go');
  fs.writeFileSync(vocabPath, vocabFile(all));
  console.log(path.relative(ROOT, vocabPath));

  const tokenGolden = {};
  for (const g of all) {
    const fixtures = listFixtures(g.family);
    // The text that mermaid gives to the parser, and the text of the file.
    const prepared = fixtures.map((text) => {
      try {
        return encodeEntities(preprocessDiagram(text).code) + '\n';
      } catch {
        return text;
      }
    });
    const seeds = unique([...prepared, ...fixtures, ...(HAND[g.name] ?? [])]);
    const snippets = snippetsOf(g.vocab);
    const typeIndex = new Map(g.vocab.map((t, i) => [t.name, i]));
    if (typeIndex.size !== g.vocab.length) {
      throw new Error(`grammar ${g.name}: two token types have the same name`);
    }

    const rand = prng(0x6c616e67 ^ g.name.length ^ (g.name.charCodeAt(0) << 8));
    const texts = [...seeds];
    for (let i = 0; i < TOKEN_MUTANTS && seeds.length > 0; i++) {
      texts.push(mutate(seeds[Math.floor(rand() * seeds.length)], rand, snippets));
    }
    tokenGolden[g.name] = unique(texts).map((text) => lex(g.services, typeIndex, text));

    const mutants = AST_GRAMMARS[g.name];
    if (mutants === undefined) {
      continue;
    }
    const astRand = prng(0x61737400 ^ g.name.length ^ (g.name.charCodeAt(0) << 8));
    const astTexts = [...seeds];
    for (let i = 0; i < mutants; i++) {
      astTexts.push(mutate(seeds[Math.floor(astRand() * seeds.length)], astRand, snippets));
    }
    const cases = [];
    for (const text of unique(astTexts)) {
      cases.push(await ast(g.name, text));
    }
    const ok = cases.filter((c) => c.ast !== undefined).length;
    console.log(`${g.name}: ${cases.length} parser cases, ${ok} valid`);
    writeGolden(path.join(ROOT, `internal/diagrams/${g.name}/testdata/ast.json.gz`), cases);
  }
  const count = Object.values(tokenGolden).reduce((n, cases) => n + cases.length, 0);
  console.log(`${count} lexer cases`);
  writeGolden(path.join(ROOT, 'internal/langium/testdata/tokens.json.gz'), tokenGolden);
  const converted = convertGolden(all, tokenGolden);
  console.log(`${converted.length} converter cases`);
  writeGolden(path.join(ROOT, 'internal/langium/testdata/convert.json.gz'), converted);
}

await main();
