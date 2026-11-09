#!/usr/bin/env node
// Writes internal/diagrams/treemap/testdata/ast.json.gz: parser goldens for
// the treemap diagram from the real @mermaid-js/parser (the Langium grammar
// packages/parser/src/language/treemap/treemap.langium).
//
// Each case is a text with the AST of parse('treemap', text), or the fact
// that the parser fails. The texts are the corpus fixtures, hand-written
// inputs for the branches of the grammar, and mutated inputs.
//
// Run it with tools/reference as the working directory:
//
//   node gen-treemap-ast.mjs
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { parse } from '@mermaid-js/parser';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');
const CORPUS = path.join(ROOT, 'testdata/corpus/mermaid-e2e/treemap');
const OUT = path.join(ROOT, 'internal/diagrams/treemap/testdata/ast.json.gz');

// mulberry32, the same generator as gen-jsnum.mjs.
function prng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const f64 = new DataView(new ArrayBuffer(8));
const bits = (v) => {
  if (Number.isNaN(v)) return '7ff8000000000000';
  f64.setFloat64(0, v);
  return f64.getBigUint64(0).toString(16).padStart(16, '0');
};

// ast is the AST in a plain form. A number is the hex text of its bits,
// because a value can be NaN.
function plain(ast) {
  const out = { rows: [] };
  for (const key of ['title', 'accTitle', 'accDescr']) {
    if (ast[key] !== undefined) out[key] = ast[key];
  }
  for (const row of ast.TreemapRows ?? []) {
    const r = {};
    if (row.indent !== undefined) r.indent = row.indent;
    if (row.$type === 'ClassDefStatement') {
      r.classDef = { className: row.className };
      if (row.styleText !== undefined) r.classDef.styleText = row.styleText;
    } else if (row.$type !== 'TreemapRow') {
      throw new Error(`unexpected row type ${row.$type}`);
    }
    if (row.item !== undefined) {
      const item = row.item;
      if (item.$type !== 'Leaf' && item.$type !== 'Section') throw new Error(`unexpected item type ${item.$type}`);
      r.item = { leaf: item.$type === 'Leaf', name: item.name };
      if (item.value !== undefined) r.item.value = bits(item.value);
      if (item.classSelector !== undefined) r.item.classSelector = item.classSelector;
    }
    out.rows.push(r);
  }
  return out;
}

async function run(text) {
  try {
    return { text, ast: plain(await parse('treemap', text)) };
  } catch {
    return { text, error: true };
  }
}

// The text that mermaid gives to the parser: without the front matter and
// the comments, and with a line break at the end.
function prepared(source) {
  let text = source.replace(/\r\n?/g, '\n').replace(/^-{3}\s*[\n\r](.*?)[\n\r]-{3}\s*[\n\r]+/s, '');
  text = text.replace(/\s*%%.*\n/gm, '').trimStart();
  return text + '\n';
}

function* walk(dir) {
  for (const e of fs.readdirSync(dir).sort()) {
    const p = path.join(dir, e);
    if (fs.statSync(p).isDirectory()) yield* walk(p);
    else if (p.endsWith('.mmd')) yield p;
  }
}

const texts = [];
const fixtures = [];
for (const file of walk(CORPUS)) {
  const source = fs.readFileSync(file, 'utf8');
  const body = source.replace(/^-{3}\s*[\n\r](.*?)[\n\r]-{3}\s*[\n\r]+/s, '');
  fixtures.push(prepared(source));
  // The parser itself also takes comments.
  fixtures.push(body);
}
texts.push(...fixtures);

// Hand-written inputs: one for each branch of the grammar and of the value
// converters, and the inputs on which the lexer decides.
texts.push(
  'treemap',
  'treemap\n',
  'treemap-beta\n"A"',
  'treemap\n  classDef cls fill:red\n"A"',
  'treemap\n"A" "B"',
  'treemap\n"A"   \n"B"',
  'treemap\n"a" : 10\n"b":10\n"c" ,10\n"d",  10',
  'treemap\n"a": 10:::cls\n"S":::cls\n',
  'treemap\nclassDef cls\n"Root"\n',
  'treemap\nclassDef cls fill:red;"Root"\n',
  'treemap\ntitle My Title\naccTitle: acc t\naccDescr: desc\n"A"\ntitle Second',
  'treemap\n"a": 1,000.5',
  'treemap\n"a": .5',
  'treemap\n"a": 1.2.3',
  'treemap\n"a": 1_0',
  'treemap\n"a": 5.',
  'treemap\n"a": ,5',
  'treemap\n"a": 5,',
  'treemap\n"a": 5,,6',
  'treemap\n"a": .',
  'treemap\n"a": ...',
  'treemap\n"a": ,',
  'treemap\n"a": 1e5',
  'treemap\n"a": 007',
  'treemap\n"a": 12345678901234567890123',
  'treemap\n"a": ' + '9'.repeat(400),
  'treemap\n"a": 0.1,2',
  'treemap\n"a": 9_9',
  'treemap\n"a": _',
  'treemap\n"a",5',
  'treemap\n"a",,5',
  'treemap\n"a":,5',
  'treemap\n"a": 1 2',
  'treemap\n"a": 10 :::cls',
  'treemap\n"a"::: cls',
  "treemap\n'single': 5\n''",
  'treemap\n"A"\n  "B"\n\t"C"\n \t "D": 1',
  'treemap\n"a":\n10',
  'treemap\n"a"\n:10',
  'treemap\n"a"\n  :10',
  'treemap\n"a"\n  : 10\n   "b"',
  'treemap\n"a": 10 "b": 20',
  'treemap\n"a": 10"b": 20',
  'treemap\n"a":10:::x"b"',
  'treemap\n  "indented root"\n"x"',
  'treemap\n"A"\n    classDef c1 fill:#f00\n    "B":::c1',
  'treemap "A"',
  'treemap"A"',
  'treemapfoo',
  'treemap\naccDescr {\n multi\n line\n}\n"A"',
  'treemap\n"A"\n%% comment\n"B"',
  'treemap\n"A" %% comment\n"B"',
  'treemap\n"A": 5 %% c\n   %% d\n"B"',
  'treemap\n"A"\nfoo',
  'treemap\n"A": 0x10',
  'treemap\n"A": -5',
  'treemap\n"A":5:::a:::b',
  'treemap\n"A":::a:::b',
  'treemap\n"A":::a "B"',
  'treemap\n"A":::a: 5',
  'treemap\nclassDef a fill:red\nclassDef a stroke:blue\n"X":::a',
  'treemap\nclassDef ab fill:red\nclassDef ab stroke:blue\n"X":::ab',
  'treemap\n"multi\nline": 3',
  ' treemap\n"A"',
  '\ntreemap\n"A"',
  '\n\n  \ntreemap\n"A"',
  'treemap\ntitle\n"A"',
  'treemap\ntitleX\n"A"',
  'treemap\n"A"\n   title T',
  'treemap\n"A"\n   accTitle: T\n  "B"',
  'treemap\n"A"\n  "x": 1\n  \n  "y": 2\n',
  'treemap\n"A"  \n',
  'treemap\n"A"\n  ',
  'treemap\n"A":::b1\n',
  'treemap\n"A":::_\n',
  'treemap\nclassDef ab fill:red, stroke:blue;;"X"',
  'treemap\nclassDef ab;\n"X"',
  'treemap\nclassDef\nab fill:red\n"X"',
  'treemap\nclassDef ab\tfill:red\n"X"',
  'treemap\nclassDef ab fill:red\n  \n"X"',
  'treemap\nclassDef ab \n"X"',
  'treemap\nclassDef ab\n\n\n  "X": 5\n"Y"',
  'treemap\nclassDef ab fill:red\n"X"',
  'treemap\nclassDef ab fill:é😀\n"X"',
  'treemap\nclassDef ab fill:red\\,blue,stroke:x\n"X"',
  'treemap\nclassDef _b c\nclassDef a1 b\nclassDef 1a b\n',
  'treemap\naccDescr: a  b   c %% comment\n"X"',
  'treemap\ntitle a %% c\n"X"',
  'treemap\naccTitle:\n"X"',
  'treemap\n"X"accTitle: t',
  'treemap\n"X": 5title t2',
  'treemap\n"X": 5\ntitle t2\ntitle t3\naccDescr { a\n\n  b  c\n }',
  'treemap\naccDescr {\n unclosed\n"X"',
  'treemap\naccDescr{x}accDescr:y\n',
  'treemap\r\n"A"\r\n    "b": 1\r\n',
  'treemap\r"A"',
  'treemap\n"é😀": 1',
  'treemap\n"a\\"b"',
  'treemap\n"a\'b": 1\n\'c"d\': 2',
  'treemap\n"unterminated',
  'TREEMAP\n"a"',
  'treemap-beta-x\n"a"',
  'treemap-betax\n"a"',
  'treemap-beta"a"',
  'treemap treemap',
  'treemap\ntreemap\n"A"',
  'treemap\ntrue\n"A"',
  'treemap\n"A"\n"B"\n"C": 1\n"D"\n  "E": 2',
  '',
  '\n',
  '"A"',
  'treemap\n"ﬂ°quot¶ß": 5',
  'treemap\n"a": 5 :: b',
  'treemap\n"a": 5::b',
  'treemap\n"a"::b',
  'treemap\n"a"::::b',
  'treemap\n"a":::b:5'
);

// Mutated inputs: random edits of the fixtures.
const rnd = prng(0x7265656d);
const pick = (a) => a[Math.floor(rnd() * a.length)];
const pieces = [
  '"', "'", ':', ',', ':::', '\n', '\n', ' ', '  ', '    ', '\t', '0', '7', '42', '.', '_', 'x', 'ab', 'classDef ', 'classDef cls fill:red',
  'title ', 'accTitle: ', 'accDescr: ', 'accDescr {', '}', '%%', ';', '-', 'treemap', 'treemap-beta', '#', 'é', '😀', '\r\n', '"N"', '"N": 1', ':::cls',
];
function mutate(text) {
  const units = Array.from(text);
  const edits = 1 + Math.floor(rnd() * 3);
  for (let e = 0; e < edits; e++) {
    const at = Math.floor(rnd() * (units.length + 1));
    const k = rnd();
    if (k < 0.35) units.splice(at, 0, pick(pieces));
    else if (k < 0.65) units.splice(at, 1 + Math.floor(rnd() * 3));
    else if (k < 0.85) units.splice(at, 1, pick(pieces));
    else {
      // Move a line.
      const lines = units.join('').split('\n');
      const from = Math.floor(rnd() * lines.length);
      const [line] = lines.splice(from, 1);
      lines.splice(Math.floor(rnd() * (lines.length + 1)), 0, line);
      units.length = 0;
      units.push(...Array.from(lines.join('\n')));
    }
  }
  return units.join('');
}
const seen = new Set(texts);
for (let i = 0; i < 2600; i++) {
  const text = mutate(pick(fixtures));
  if (seen.has(text)) continue;
  seen.add(text);
  texts.push(text);
}

const cases = [];
for (const text of texts) cases.push(await run(text));
const failed = cases.filter((c) => c.error).length;
fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, zlib.gzipSync(Buffer.from(JSON.stringify({ fixtures: fixtures.length, cases })), { level: 9 }));
console.log(`${path.relative(ROOT, OUT)}: ${cases.length} cases, ${failed} that the parser rejects, ${fixtures.length} from fixtures`);
