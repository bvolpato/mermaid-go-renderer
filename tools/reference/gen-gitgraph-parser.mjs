// Golden generator for the gitGraph parser of internal/diagrams/git.
//
// It runs the real Langium parser of @mermaid-js/parser (the one that
// mermaid 12.2.0 bundles) on the corpus fixtures, on seeded mutations of
// them and on edge cases of each terminal rule. For every input it stores
// the AST, or the fact that the parser reports an error.
//
// Run with tools/reference as the working directory:
//
//   node gen-gitgraph-parser.mjs
//
// Output: internal/diagrams/git/testdata/parser.json.gz
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { parse } from '@mermaid-js/parser';
import { TESTDATA, corpusFiles, stripFrontMatter, mulberry32, mutations } from './gitgraph-cases.mjs';

// strip keeps the properties of an AST node that the grammar assigns, and
// the node type. Langium adds the other properties that start with $.
const strip = (v, top = true) => {
  if (Array.isArray(v)) return v.map((x) => strip(x, false));
  if (v && typeof v === 'object') {
    const o = {};
    for (const k of Object.keys(v)) {
      if (k.startsWith('$') && (k !== '$type' || top)) continue;
      o[k] = strip(v[k], false);
    }
    return o;
  }
  return v;
};

// Edge cases of the lexer and of the grammar.
const HEADERS = ['gitGraph', 'gitGraph:', 'gitGraph :', 'gitGraph LR:', 'gitGraph TB :', 'gitGraph BT:'];
const EDGE = [
  '', ' ', '\n', 'gitGraph', 'gitGraph\n', '\n\n  gitGraph\n\n', 'gitGraph:', 'gitGraph :', 'gitGraph  :  \n',
  'gitGraph LR:', 'gitGraph TB:', 'gitGraph BT:', 'gitGraph LR', 'gitGraph TB\n', 'gitGraph RL:', 'gitGraph LR: commit',
  'gitGraph: commit', 'gitGraph:commit', 'gitGraph commit', 'gitGraphx', 'gitGraph-x\ncommit', 'gitGraph%% c\ncommit',
  'gitGraph %% c\ncommit', 'gitgraph\ncommit', 'GITGRAPH', 'gitGraph::', 'gitGraph: :', 'gitGraph LR::', 'gitGraph LR TB:',
  'gitGraph:LR:', 'gitGraph\ngitGraph', 'commit', 'gitGraph\r\ncommit\r\n', 'gitGraph\rcommit', 'gitGraph\tcommit\t\n',
  'gitGraph commit', 'gitGraph commit', 'gitGraph\n commit', 'gitGraph\n\fcommit', 'gitGraph\n﻿commit',
  '---\ntitle: x\n---\ngitGraph\ncommit', '---\n---\ngitGraph', '--- \t\nx: 1\n---gitGraph', '---\nx\n---\n---\ny\n---\ngitGraph',
  '---\nx\n--- gitGraph', '---\nx\n---x gitGraph', 'gitGraph\n---\nx\n---\ncommit', '---\ngitGraph', '----\nx\n---\ngitGraph',
  '%%{init: {}}%%\ngitGraph\ncommit', '%%{init: {}}%% gitGraph\ncommit', '%%{init: {}}%%gitGraph\ncommit',
  'gitGraph\n %%{x}%%\ncommit', 'gitGraph\n %%{x}%%y}%%\ncommit', 'gitGraph\n %%{x\ncommit', 'gitGraph\n%% only comment',
  'gitGraph\n  %% c1\n  commit %% c2\n%%c3', 'gitGraph\ncommit id: "a" %% c\n', 'gitGraph\ncommit id: "a%%b"\n',
];
for (const h of HEADERS) {
  for (const body of [
    'commit', 'commit\n', 'commit \n commit', 'commit id: "a"', 'commit id:"a"', 'commit id : "a"', 'commit id:\n"a"',
    'commit id: "a" id: "b"', 'commit "m"', 'commit msg: "m"', 'commit msg:"m" "n"', 'commit "m" msg: "n" "o"',
    'commit tag: "t"', 'commit tag: "t" tag: "u" tag: ""', 'commit type: NORMAL', 'commit type: REVERSE',
    'commit type: HIGHLIGHT', 'commit type: MERGE', 'commit type: highlight', 'commit type:', 'commit type: "NORMAL"',
    'commit type: NORMALx', 'commit type: NORMAL type: REVERSE', 'commit id: "a" tag: "t" type: HIGHLIGHT msg: "m"',
    'commit id:', 'commit id: a', 'commit id: 1', 'commit tag:', 'commit msg:', 'commit order: 1', 'commit parent: "x"',
    "commit id: 'single'", 'commit id: "dq\\"esc"', "commit id: 'sq\\'esc'", 'commit id: "a\\nb\\tc\\\\d\\qe\\0f\\bg\\fh\\ri\\vj"',
    'commit id: "multi\nline"', 'commit id: "back\\\nslash"', 'commit id: "unterminated', 'commit id: "a" "b" "c"',
    'commit id: ""', "commit id: ''", 'commit id: "😀\\😀"', 'commit id: "\\', 'commit id: "a"b', 'commit id: "a""b"',
    'commit commit', 'commit branch x', 'commit\nbranch x', 'commit; commit', 'commit, commit', 'commit 1', 'commit x',
    'branch a', 'branch a\n', 'branch "a b"', "branch 'a'", 'branch a order: 1', 'branch a order:1', 'branch a order: 0',
    'branch a order: 01', 'branch a order: 10', 'branch a order: 1.5', 'branch a order: 12.5', 'branch a order: 1.',
    'branch a order: -1', 'branch a order: "1"', 'branch a order:', 'branch a order: 1 order: 2', 'branch a order: 99999999999999999999',
    'branch a order: 9007199254740993', 'branch', 'branch\n', 'branch a b', 'branch a.b', 'branch a.b.', 'branch a/b/', 'branch a-b-',
    'branch a..b', 'branch a./-', 'branch _a', 'branch -a', 'branch .a', 'branch /a', 'branch 1', 'branch 12', 'branch 1a', 'branch a1',
    'branch 1.0', 'branch 12.5', 'branch 0', 'branch 0a', 'branch 00', 'branch 1-a', 'branch a@b', 'branch a#b', 'branch é',
    'branch aé', 'branch a:b', 'branch a :', 'branch id', 'branch id:', 'branch tag', 'branch type', 'branch order', 'branch parent',
    'branch commit', 'branch branch', 'branch merge', 'branch checkout', 'branch switch', 'branch cherry-pick', 'branch cherry',
    'branch gitGraph', 'branch LR', 'branch TB', 'branch BT', 'branch NORMAL', 'branch REVERSE', 'branch HIGHLIGHT',
    'branch commit1', 'branch commits', 'branch commit-x', 'branch branch/x', 'branch LRx', 'branch LR-x', 'branch NORMAL_', 'branch gitGraphs',
    'branch gitGraph/x', 'branch cherry-picked', 'branch cherry-pick/x', 'branch title', 'branch titles', 'branch a title', 'branch accTitle',
    'branch accDescr', 'branch msg', 'branch msg:', 'branch "LR"', 'branch "commit"',
    'merge a', 'merge "a"', 'merge a id: "i"', 'merge a tag: "t" tag: "u"', 'merge a type: REVERSE', 'merge a id: "i" tag: "t" type: HIGHLIGHT',
    'merge a type: NORMAL id: "i"', 'merge a msg: "m"', 'merge a "m"', 'merge a order: 1', 'merge a parent: "p"', 'merge', 'merge a b',
    'merge id: "i"', 'merge a id: "i" id: "j"', 'merge 1', 'merge LR',
    'checkout a', 'checkout "a"', 'switch a', 'switch "a b"', 'checkout', 'switch', 'checkout a b', 'checkout a id: "x"', 'checkout 1',
    'checkout main\nswitch main', 'checkout checkout', 'checkout switchx',
    'cherry-pick id: "a"', 'cherry-pick id:"a" tag: "t"', 'cherry-pick id: "a" tag: ""', 'cherry-pick id: "a" parent: "p"',
    'cherry-pick id: "a" tag: "t" parent: "p" tag: "u" id: "b" parent: "q"', 'cherry-pick', 'cherry-pick tag: "t"', 'cherry-pick parent: "p"',
    'cherry-pick id: "a" type: NORMAL', 'cherry-pick id: "a" msg: "m"', 'cherry-pick id: "a" "m"', 'cherry-pick a', 'cherry-pick id: a',
    'cherry-pickid: "a"', 'cherry -pick id: "a"', 'cherry-pick-x', 'cherrypick id: "a"',
    'title', 'title T', 'title  My   Title  ', 'title\tTabbed', 'titleT', 'title: T', 'title T %% c', 'title T%%c', 'title %%', 'title T\ntitle U',
    'title T\n\n\naccTitle: A', ' \t title T', 'title T commit', 'title\ncommit', 'title\n', 'commit title T', 'commit\ntitle', 'title T\r\ncommit',
    'accTitle: A', 'accTitle:A', 'accTitle : A', 'accTitle:  a   b\t\tc  ', 'accTitle: A %% c', 'accTitle:%%c', 'accTitle:', 'accTitle', 'accTitle A',
    'accTitle: A\naccTitle: B', '  accTitle: indented', 'accTitle: A\ncommit', 'accTitle: a: b',
    'accDescr: D', 'accDescr:D', 'accDescr : D', 'accDescr:  a   b  ', 'accDescr: D %% c', 'accDescr:', 'accDescr', 'accDescr D',
    'accDescr { D }', 'accDescr{D}', 'accDescr {\n  line one\n    line   two\n\n\n  line three  \n}', 'accDescr\n\n{ D }', 'accDescr  { D }',
    'accDescr { }', 'accDescr {}', 'accDescr { a } }', 'accDescr { a', 'accDescr { a\n}\ncommit', 'accDescr { a %% not a comment\n b }',
    'accDescr {\r\n a \r\n\r\n b \r\n}', 'accDescr { \t a \t\t b \t }', 'accDescr {a\n\nb}', 'accDescr {a\n \n \nb}', 'accDescr {a \n\n b}',
    'accDescr { a  b  c }', 'accDescr {\n\n a\n\n}', 'accDescr { a }commit', 'accDescr { a } commit', 'accDescr: a\naccDescr { b }',
    'accDescr { x: y }', 'accDescr: { a }', 'accDescr : { a\n }',
    'commit\n\n\n\ncommit', 'commit\n \n\t\ncommit', 'commit %% c', 'commit%%c', 'commit\n%%c\ncommit', '%%c', 'commit id: "a" %%{x}%%',
    '1', '0', '12', '.', '-', '/', ':', '::', '"s"', 'x', 'LR', 'NORMAL', 'id:', 'order:', '---', '%', '%%', '#', 'ﬂ°lt¶ß',
    'commit id: "ﬂ°°35¶ß" tag: "ﬂ°lt¶ß"', 'commit\x00', 'commit\x0b', 'commit ',
  ]) {
    EDGE.push(h + '\n' + body, h + '\n  ' + body + '\n');
  }
}

const cases = new Map();
// A text with a lone surrogate is left out: JSON cannot carry it to Go.
const add = (text) => {
  if (text.isWellFormed() && !cases.has(text)) cases.set(text, null);
};

const files = corpusFiles();
const rand = mulberry32(0x67697431);
for (const f of files) {
  add(f.text);
  const body = stripFrontMatter(f.text);
  add(body);
  add(body + '\n');
  for (const m of mutations(rand, body, 40)) add(m);
  for (const m of mutations(rand, f.text, 4)) add(m);
}
for (const e of EDGE) add(e);

const out = [];
let errors = 0;
for (const text of cases.keys()) {
  try {
    const ast = await parse('gitGraph', text);
    out.push({ text, ast: strip(ast) });
  } catch {
    errors++;
    out.push({ text, error: true });
  }
}

fs.mkdirSync(TESTDATA, { recursive: true });
const file = path.join(TESTDATA, 'parser.json.gz');
fs.writeFileSync(file, zlib.gzipSync(JSON.stringify({ cases: out }), { level: 9 }));
process.stderr.write(`${out.length} cases (${errors} errors) -> ${file} (${fs.statSync(file).size} bytes)\n`);
