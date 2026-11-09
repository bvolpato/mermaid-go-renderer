#!/usr/bin/env node
// Writes internal/diagrams/eventmodeling/testdata/ast.json.gz: parser
// goldens for the eventmodeling diagram from the real @mermaid-js/parser
// (the Langium grammar
// packages/parser/src/language/eventmodeling/event-modeling.langium).
//
// Each case is a text with the AST of parse('eventmodeling', text), or the
// fact that the parser fails. The texts are the corpus fixtures,
// hand-written inputs for the branches of the grammar, and mutated inputs.
// A cross reference is the text of its token ($refText).
//
// Run it with tools/reference as the working directory:
//
//   node gen-eventmodeling-ast.mjs
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { parse } from '@mermaid-js/parser';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');
const CORPUS = path.join(ROOT, 'testdata/corpus/mermaid-e2e/eventmodeling');
const OUT = path.join(ROOT, 'internal/diagrams/eventmodeling/testdata/ast.json.gz');

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

// ast is the AST in a plain form. A property that is undefined is absent.
function plain(ast) {
  if (ast.$type !== 'EventModel') throw new Error(`unexpected type ${ast.$type}`);
  const out = {};
  for (const key of ['accDescr', 'accTitle', 'title']) {
    if (ast[key] !== undefined) out[key] = ast[key];
  }
  const opt = (target, source, ...keys) => {
    for (const key of keys) {
      if (source[key] !== undefined) target[key] = source[key];
    }
    return target;
  };
  const refs = (list) => list.map((r) => r.$refText);
  out.frames = ast.frames.map((f) => {
    if (f.$type !== 'EmTimeFrame' && f.$type !== 'EmResetFrame') throw new Error(`unexpected frame type ${f.$type}`);
    const frame = {
      type: f.$type,
      name: f.name,
      modelEntityType: f.modelEntityType,
      entityIdentifier: f.entityIdentifier,
      sourceFrames: refs(f.sourceFrames),
      line: f.$cstNode.range.start.line,
    };
    if (f.dataReference !== undefined) frame.dataReference = f.dataReference.$refText;
    return opt(frame, f, 'dataType', 'dataInlineValue');
  });
  out.modelEntities = ast.modelEntities.map((e) => ({ name: e.name }));
  out.dataEntities = ast.dataEntities.map((e) => opt({ name: e.name, dataBlockValue: e.dataBlockValue }, e, 'dataType'));
  out.noteEntities = ast.noteEntities.map((e) => opt({ sourceFrame: e.sourceFrame.$refText, dataBlockValue: e.dataBlockValue }, e, 'dataType'));
  const statements = (list) => list.map((s) => s.entityIdentifier.$refText);
  out.gwtEntities = ast.gwtEntities.map((e) => ({
    sourceFrame: e.sourceFrame.$refText,
    givenStatements: statements(e.givenStatements),
    whenStatements: statements(e.whenStatements),
    thenStatements: statements(e.thenStatements),
  }));
  return out;
}

async function run(text) {
  let ast;
  try {
    ast = await parse('eventmodeling', text);
  } catch {
    return { text, error: true };
  }
  return { text, ast: plain(ast) };
}

// The text that mermaid gives to the parser: without the front matter, and
// with a line break at the end. The db of the diagram keeps the comments
// (preserveCommentsWhenParsing).
function prepared(source) {
  const text = source.replace(/\r\n?/g, '\n').replace(/^-{3}\s*[\n\r](.*?)[\n\r]-{3}\s*[\n\r]+/s, '');
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
  fixtures.push(prepared(source));
  // The parser itself also takes the front matter.
  fixtures.push(source);
}
texts.push(...fixtures);

const em = (body) => 'eventmodeling\n' + body + '\n';
// Hand-written inputs: one for each branch of the grammar, and the inputs
// on which the lexer decides.
texts.push(
  // The header.
  'eventmodeling',
  'eventmodeling\n',
  '\n\n  eventmodeling  \n\n',
  'eventmodelingx\ntf 01 ui A',
  'eventmodeling%% comment\ntf 01 ui A',
  'eventmodeling tf 01 ui A',
  'eventmodeling\ttf 01 ui A',
  'Eventmodeling\ntf 01 ui A',
  'EVENTMODELING\ntf 01 ui A',
  'eventmodeling eventmodeling',
  'eventmodeling\neventmodeling\ntf 01 ui A',
  '',
  '\n',
  'tf 01 ui A',
  '---\ntitle: x\n---\neventmodeling\ntf 01 ui A\n',
  '%%{init: {"theme": "dark"}}%%\neventmodeling\ntf 01 ui A\n',
  '%% comment\neventmodeling\ntf 01 ui A\n',
  '// comment\neventmodeling\ntf 01 ui A\n',
  '/* comment */ eventmodeling /* more */ tf 01 ui A\n',
  // Frames: every keyword and every entity type.
  em('tf 01 ui A'),
  em('timeframe 01 ui A'),
  em('rf 01 ui A'),
  em('resetframe 01 ui A'),
  em('tf 1 rmo A\ntf 2 readmodel B\ntf 3 ui C\ntf 4 cmd D\ntf 5 command E\ntf 6 evt F\ntf 7 event G\ntf 8 pcr H\ntf 9 processor I'),
  em('tf 01 ui A tf 02 cmd B rf 03 evt C'),
  em('tf 01 ui A\n\n\n\ntf 02 cmd B'),
  em('tf\n01\nui\nA'),
  em('tf01 ui A'),
  em('tf 01ui A'),
  em('tf 01 uiA'),
  em('tf 01 ui'),
  em('tf 01'),
  em('tf'),
  em('tf ui A'),
  em('tf 01 A'),
  em('tf 01 xx A'),
  em('tf 01 UI A'),
  em('tf 0 ui A'),
  em('tf 000 ui A'),
  em('tf 999 ui A'),
  em('tf 0001 ui A'),
  em('tf 1234 ui A'),
  em('tf 12a ui A'),
  em('tf a1 ui A'),
  em('tf -1 ui A'),
  em('tf 1.5 ui A'),
  em('tf 01 ui 02'),
  em('tf 01 ui _a'),
  em('tf 01 ui a_b_1'),
  em('tf 01 ui 1a'),
  em('tf 01 ui a-b'),
  em('tf 01 ui Ünï'),
  em('tf 01 ui "A"'),
  em('TF 01 ui A'),
  em('tff 01 ui A'),
  em('timeframes 01 ui A'),
  // Keywords as names: a keyword is not an EM_ID, a longer word is.
  em('tf 01 ui ui'),
  em('tf 01 ui uis'),
  em('tf 01 evt event'),
  em('tf 01 evt events'),
  em('tf 01 evt data'),
  em('tf 01 evt datas'),
  em('tf 01 evt tf'),
  em('tf 01 evt json'),
  em('tf 01 evt given'),
  em('tf 01 evt eventmodeling'),
  em('tf 01 evt eventmodelings'),
  em('tf 01 evt title'),
  em('tf 01 evt titles'),
  em('tf 01 evt accTitle'),
  em('tf 01 evt accDescr'),
  em('tf 01 evt  title'),
  em('tf 01 evt\ttitle x'),
  em('tf 01 evt\ntitle'),
  // Qualified names.
  em('tf 01 ui A.B'),
  em('tf 01 ui A.B.C'),
  em('tf 01 ui A . B'),
  em('tf 01 ui A.\nB'),
  em('tf 01 ui A /* c */ . %% d\n B'),
  em('tf 01 ui A.'),
  em('tf 01 ui .A'),
  em('tf 01 ui A..B'),
  em('tf 01 ui A.1'),
  em('tf 01 ui A.ui'),
  em('tf 01 ui ns.title'),
  // Source frames.
  em('tf 01 ui A\ntf 02 cmd B ->> 01'),
  em('tf 01 ui A\ntf 02 cmd B ->> 01 ->> 03 ->> 99'),
  em('tf 01 ui A\ntf 02 cmd B->>01->>01'),
  em('tf 01 ui A\ntf 02 cmd B ->>'),
  em('tf 01 ui A\ntf 02 cmd B ->> A'),
  em('tf 01 ui A\ntf 02 cmd B -> 01'),
  em('tf 01 ui A\ntf 02 cmd B ->>> 01'),
  em('tf 01 ui A\ntf 02 cmd B - >> 01'),
  em('tf 01 ui A\ntf 02 cmd B ->> 0001'),
  em('tf 01 ui A ->> 01'),
  em('tf 02 cmd B ->> 05\ntf 05 evt C'),
  // Data references.
  em('tf 01 evt A [[D]]'),
  em('tf 01 evt A[[D]]'),
  em('tf 01 evt A [[ D ]]'),
  em('tf 01 evt A [[D]] [[E]]'),
  em('tf 01 evt A [[]]'),
  em('tf 01 evt A [[D'),
  em('tf 01 evt A [[D]'),
  em('tf 01 evt A [D]'),
  em('tf 01 evt A [[D.E]]'),
  em('tf 01 evt A [[01]]'),
  em('tf 01 evt A [[ui]]'),
  em('tf 01 evt A ->> 01 [[D]]'),
  em('tf 01 evt A [[D]] ->> 01'),
  em('tf 01 evt A [ [D]]'),
  // Inline data.
  em('tf 01 cmd A { a: 1 }'),
  em('tf 01 cmd A {a: 1}'),
  em('tf 01 cmd A {}'),
  em('tf 01 cmd A { a: { b: 1 } }'),
  em('tf 01 cmd A { a } tf 02 evt B { c }'),
  em('tf 01 cmd A { a: 1 }\ntf 02 evt B { b: 2 }'),
  em('tf 01 cmd A "a string"'),
  em("tf 01 cmd A 'a string'"),
  em('tf 01 cmd A ""'),
  em('tf 01 cmd A "a" b "c"'),
  em('tf 01 cmd A "unterminated'),
  em('tf 01 cmd A { unterminated'),
  em('tf 01 cmd A { a: 1 } { b: 2 }'),
  em('tf 01 cmd A `json` { "a": 1 }'),
  em('tf 01 cmd A `jsobj` { a: 1 }'),
  em('tf 01 cmd A `figma` "url"'),
  em('tf 01 cmd A `salt` {x}'),
  em('tf 01 cmd A `uri` "x"'),
  em('tf 01 cmd A `md` "x"'),
  em('tf 01 cmd A `html` "<b>x</b>"'),
  em('tf 01 cmd A `text` "x"'),
  em('tf 01 cmd A ` json ` { a }'),
  em('tf 01 cmd A `json`{ a }'),
  em('tf 01 cmd A `xml` { a }'),
  em('tf 01 cmd A `json { a }'),
  em('tf 01 cmd A json` { a }'),
  em('tf 01 cmd A `json`'),
  em('tf 01 cmd A ``{ a }'),
  em('tf 01 cmd A [[D]] { a: 1 }'),
  em('tf 01 cmd A [[D]] `json` { a: 1 }'),
  em('tf 01 cmd A ->> 02 [[D]] `json` { a: 1 }'),
  em('tf 01 cmd A { a: 1 } [[D]]'),
  em('tf 01 cmd A { a: 1 } ->> 02'),
  em('tf 01 cmd A { é 😀 }'),
  em('tf 01 cmd A { a: 1 } %% comment'),
  em('tf 01 cmd A { a: 1 // not a comment }'),
  em('tf 01 cmd A { a: "}" }'),
  em('tf 01 cmd A {\n a: 1 }'),
  em('tf 01 cmd A {\n  a: 1\n}'),
  em('tf 01 cmd A { \n}'),
  em('tf 01 cmd A {\n}\ntf 02 evt B'),
  // Entities.
  em('entity A'),
  em('entity A.B.C'),
  em('entity'),
  em('entity 01'),
  em('entity A entity B'),
  em('entity ui'),
  em('entities A'),
  // Data entities.
  em('data D\n{\n  a: 1\n}'),
  em('data D {\n  a: 1\n}'),
  em('data D\n{\n}'),
  em('data D\n{ \t\n  a: 1,\n  b: 2\n}\ntf 01 ui A'),
  em('data D\n{\n  a: 1\n}tf 01 ui A'),
  em('data D\n{\n  a: 1\n} tf 01 ui A'),
  em('data D\n{\n  a: {\n    b: 1\n  }\n}'),
  em('data D\n{\n  a: 1\n}\n}'),
  em('data D\n{\n  a: 1\n  }'),
  em('data D\n{\r\n  a: 1\r\n}\r\n'),
  em('data D { a: 1 }'),
  em('data D\n{ a: 1\n}'),
  em('data D'),
  em('data\n{\n}'),
  em('data D.E\n{\n}'),
  em('data 01\n{\n}'),
  em('data D `json`\n{\n  "a": 1\n}'),
  em('data D `jsobj` {\n  a: 1\n}'),
  em('data D `xml` {\n}'),
  em('data D `json` { a }'),
  em('data D\n{\n  a: 1\n}\ndata E\n{\n  b: 2\n}'),
  em('data D\n{\n  é 😀\n}'),
  em('data D\n{\n  %% not a comment\n}'),
  em('data D\n{\n unterminated'),
  // Notes.
  em('note 01\n{\n  a note\n}'),
  em('note 01 {\n  a note\n}'),
  em('note 01 `md` {\n  # a note\n}'),
  em('note A {\n}'),
  em('note 01'),
  em('note 01 { inline }'),
  em('note\n{\n}'),
  // Given, when, then.
  em('gwt 01 given ui A then evt B'),
  em('gwt 01 given ui A when cmd C then evt B'),
  em('gwt 01 given ui A evt B rmo C when cmd D command E then evt F event G pcr H processor I readmodel J'),
  em('gwt 01\ngiven\n  ui A\nwhen\n  cmd C\nthen\n  evt B'),
  em('gwt 01 given then evt B'),
  em('gwt 01 given ui A then'),
  em('gwt 01 given ui A'),
  em('gwt 01 given ui A when then evt B'),
  em('gwt 01 given ui A when cmd C'),
  em('gwt 01 when cmd C then evt B'),
  em('gwt 01 then evt B'),
  em('gwt given ui A then evt B'),
  em('gwt A given ui A then evt B'),
  em('gwt 01 given A then evt B'),
  em('gwt 01 given ui then evt B'),
  em('gwt 01 given ui 02 then evt B'),
  em('gwt 01 given ui A.B then evt B'),
  em('gwt 01 given ui A then evt B then evt C'),
  em('gwt 01 given ui A then evt B when cmd C'),
  em('gwt 01 given ui A then evt B tf 02 ui X'),
  em('gwt 01 given ui A then evt B gwt 02 given ui A then evt B'),
  em('gwt 01 given ui A then evt B\ntf 02 ui X\nentity A\nentity B'),
  // Title and accessibility: only a token that starts with a blank.
  'eventmodeling title A title\ntf 01 ui A',
  'eventmodeling\ttitle\ntf 01 ui A',
  'eventmodeling title\ntf 01 ui A',
  'eventmodeling accTitle: An accessible title\ntf 01 ui A',
  'eventmodeling accDescr: A description\ntf 01 ui A',
  'eventmodeling accDescr {\n a\n multi line\n}\ntf 01 ui A',
  'eventmodeling accDescr{x}\ntf 01 ui A',
  'eventmodeling title T %% comment\ntf 01 ui A',
  'eventmodeling accTitle: T %% comment\ntf 01 ui A',
  'eventmodeling accTitle:\ntf 01 ui A',
  'eventmodeling accTitle\ntf 01 ui A',
  'eventmodeling accDescr\ntf 01 ui A',
  em('title A title'),
  em(' title A title'),
  em('accTitle: x'),
  em(' accTitle: x'),
  em('accDescr: x'),
  em('tf 01 ui A title after a frame\ntf 02 cmd B'),
  em('tf 01 ui A accTitle: after a frame\ntf 02 cmd B'),
  em('tf 01 ui A accDescr: after a frame\ntf 02 cmd B'),
  em('tf 01 ui A accDescr {\n multi\n}\ntf 02 cmd B'),
  em('tf 01 ui A title one\ntf 02 cmd B title two'),
  em('tf 01 ui A { a } title one'),
  em('entity A title x'),
  em('tf 01 ui A\n\ttitle x'),
  em('tf 01 ui A  \t title   spaced   out  '),
  // Comments.
  em('tf 01 ui A %% comment\ntf 02 cmd B'),
  em('tf 01 ui A // comment\ntf 02 cmd B'),
  em('tf 01 ui A /* comment */ tf 02 cmd B'),
  em('tf 01 ui A /* multi\nline */\ntf 02 cmd B'),
  em('tf 01 ui A /* unterminated\ntf 02 cmd B'),
  em('tf 01 /* in between */ ui A'),
  em('%% only a comment'),
  em('%%{init: {}}%%\ntf 01 ui A'),
  em('tf 01 ui A %%{x}%% tf 02 cmd B'),
  em('---\nyaml: 1\n---\ntf 01 ui A'),
  em('tf 01 ui A\n---\nyaml: 1\n---\ntf 02 cmd B'),
  em('tf 01 ui A / tf 02 cmd B'),
  // Characters that are no token.
  em('tf 01 ui A;'),
  em('tf 01 ui A,B'),
  em('tf 01 ui A:B'),
  em('tf 01 ui A # x'),
  em('tf 01 ui A (x)'),
  em('tf 01 ui A [x]'),
  em('tf 01 ui A }'),
  em('tf 01 ui A ]]'),
  em('tf 01 ui A `'),
  em('tf 01 ui A .'),
  em('tf 01 ui A ->'),
  // Line ends and blanks.
  'eventmodeling\r\ntf 01 ui A\r\ntf 02 cmd B\r\n',
  'eventmodeling\rtf 01 ui A\rtf 02 cmd B',
  'eventmodeling\n tf 01 ui A tf 02 cmd B﻿',
  // Everything together.
  em(
    'tf 01 ui CartScreen\ntf 02 cmd Inventory.AddItem `json` { "productId": 7 }\nrf 03 evt Inventory.ItemAdded ->> 02 [[ItemAddedData]]\n' +
      'tf 04 rmo CartItems ->> 03 ->> 01\n\nentity Inventory.Item\nentity Cart\n\ndata ItemAddedData `jsobj`\n{\n  productId: 7,\n  quantity: 1\n}\n\n' +
      'note 03 `md` {\n  A **note**\n}\n\ngwt 03 given evt Cart when cmd Cart then evt Cart evt Cart\n'
  )
);

// Mutated inputs: random edits of the fixtures and of the hand-written
// inputs that the parser accepts.
const rnd = prng(0x65766d64);
const pick = (a) => a[Math.floor(rnd() * a.length)];
const pieces = [
  'tf ', 'rf ', 'timeframe ', 'resetframe ', '01', '02', '7', '123', '1234', ' ui ', ' cmd ', ' evt ', ' rmo ', ' pcr ', ' command ', ' event ', ' readmodel ', ' processor ',
  'A', 'B', 'Name', 'ns.Name', '.', ' ->> ', '->>', ' ->> 01', ' [[', ']] ', ' [[D]]', '{', '}', ' { a: 1 }', '{\n', '\n}', '\n}\n', '"', "'", '`', '`json`', '`md` ', 'json',
  'data ', 'data D\n{\n  x\n}\n', 'note ', 'note 01 {\n n\n}\n', 'entity ', 'entity E\n', 'gwt ', ' given ', ' when ', ' then ', 'gwt 01 given ui A then evt B\n',
  ' title ', ' title T', ' accTitle: ', ' accDescr: ', ' accDescr {', '%%', ' %% c', '//', '/*', '*/', '---', '\n', '\n', ' ', '  ', '\t', '\r\n', 'é', '😀', '-', '>', ';', ':', 'eventmodeling',
];
function mutate(text) {
  const units = Array.from(text);
  const edits = 1 + Math.floor(rnd() * 3);
  for (let e = 0; e < edits; e++) {
    const at = Math.floor(rnd() * (units.length + 1));
    const k = rnd();
    if (k < 0.4) units.splice(at, 0, pick(pieces));
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

const handWritten = texts.length;
const seeds = [];
for (const text of texts) {
  const r = await run(text);
  if (!r.error) seeds.push(text);
}
const seen = new Set(texts);
for (let i = 0; i < 4000; i++) {
  const text = mutate(rnd() < 0.4 ? pick(fixtures) : pick(seeds));
  if (seen.has(text)) continue;
  seen.add(text);
  texts.push(text);
}

const cases = [];
for (const text of texts) cases.push(await run(text));
const failed = cases.filter((c) => c.error).length;
fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, zlib.gzipSync(Buffer.from(JSON.stringify({ fixtures: fixtures.length, handWritten, cases })), { level: 9 }));
console.log(`${path.relative(ROOT, OUT)}: ${cases.length} cases, ${failed} that the parser rejects, ${fixtures.length} from fixtures, ${handWritten - fixtures.length} hand-written, ${fs.statSync(OUT).size} bytes`);
