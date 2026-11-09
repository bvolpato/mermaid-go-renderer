// Writes internal/diagrams/venn/testdata/db.json.gz and
// internal/diagrams/ishikawa/testdata/db.json.gz: the content of the venn db
// (subsets, text nodes, style entries, current sets, indent mode, title) and
// of the ishikawa db (the tree of nodes, the stack levels, the base level)
// of real mermaid after it parses
//   - every fixture of testdata/corpus/mermaid-e2e/venn and ishikawa,
//   - seeded mutations of the body of those fixtures,
//   - seeded random documents made of statement snippets,
//   - seeded random documents that follow the grammar, so that most of them
//     parse and reach every production,
//   - hand-written inputs for the branches of the grammars and the dbs.
// A case that mermaid does not parse has the error message in place of the
// db. The ishikawa db sanitizes its texts with DOMPurify, which needs a
// browser, so the script runs venn-ishikawa-db-page.js with the eval command
// of the browser oracle, one time for both diagram types.
//
// Usage (in tools/reference): node gen-venn-ishikawa-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const mutantsPerFixture = 6;
const randomDocuments = 260;
const validDocuments = 220;

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

const types = {
  venn: {
    family: 'testdata/corpus/mermaid-e2e/venn',
    out: 'internal/diagrams/venn/testdata/db.json.gz',
    header: /^\s*venn-beta/i,
    headers: ['venn-beta', 'venn-beta', 'venn-beta', 'venn-beta  ', 'venn-beta title On the header line'],
    snippets: [
      'set A',
      'set B',
      'set C',
      'set A["Alpha"]',
      'set B[Beta label]',
      'set C[  spaced  label  ]',
      'set D[""]',
      'set D[" "]',
      'set A:20',
      'set B: 12.5',
      'set C:.5',
      'set C:+3',
      'set C:-2',
      'set C:0',
      'set A["Alpha"]:20',
      'set "Quoted Set"',
      'set "Quoted Set"["Label"]: 7',
      'set "1"',
      'set "2":4',
      'set "10"',
      'set constructor',
      'set a-b_c',
      'SET E',
      'Set F["Mixed case keyword"]',
      'union A,B',
      'union A, B',
      'union B,A',
      'union A,B["AB"]',
      'union A,B:3',
      'union A,B["AB"]:3',
      'union A,B,C',
      'union A,B,C["ABC"]:1',
      'union C,B,A',
      'union A,"Quoted Set"',
      'union "2","1","10"',
      'union A,A',
      'union A',
      'union A,Z',
      'union Y,Z',
      'UNION A,B',
      'text A x1',
      'text A "x 2"',
      'text A 42',
      'text A x3["Label 3"]',
      'text A "x 4"[Label 4]',
      'text A,B shared',
      'text B,A "shared 2"["S2"]',
      'text Z orphan',
      'text A x5[""]',
      'text A x6[" "]',
      'text A -1.5',
      '  text "Indented item"',
      '    text indented2["Indented label"]',
      '\ttext 7',
      '  text "Q"["QL"]',
      '  TEXT upper',
      'style A fill:#ff6b6b',
      'style B fill:#4ecdc4, stroke:#333, stroke-width:2',
      'style A,B fill:#ffe66d',
      'style B,A color:red',
      'style A fill-opacity:0.5, color:#fff',
      'style A fill:rgb(1, 2, 3)',
      'style A fill:rgba(1,2,3,0.5), stroke:rgba( 9 , 8 , 7 , .1 )',
      'style A stroke:"3px solid red"',
      'style A stroke-width:1 2 3',
      'style A font:bold 12 serif',
      'style x1 color:blue',
      'style "x 2" color:#00f',
      'style A fill:#abc, fill:#def',
      'style A constructor:red, toString:blue',
      'style A __proto__:red, fill:green',
      'style A fill:',
      'style A',
      'style A fill',
      'title A title',
      'title  spaced   title ',
      'title With # hash',
      'title With ; semicolon',
      'title',
      'TITLE Upper keyword',
      '%% a comment',
      'set G %% trailing comment',
      'set H%% glued comment',
      '%%{init: {"theme": "dark"}}%%',
      'set',
      'set A B',
      'set A[unterminated',
      'set A:',
      'set A:x',
      'set A["L"]["M"]',
      'text',
      'text A',
      'foo bar',
      ',',
      ':',
      'set ünï',
      'set "ünï cödé ☃ 😀"',
      'set J["ünï cödé ☃ 😀"]',
      'set "<b>bold</b>"["<i>it</i> & <script>x</script>"]',
      'set K["a # b ; c"]',
      'set "a,b"',
      'set "a|b"',
      'set "a;b"',
      'set ""',
      'set " "',
      'set \'single\'',
      'set A:99999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999',
    ],
    extras: [
      'venn-beta',
      'venn-beta\n',
      'venn-beta\n\n\n',
      '\n\n  venn-beta\n  set A',
      'venn-beta set A',
      'venn-beta\nset A\nset B\nunion A,B',
      'venn-beta\n  set A\n  set A\n  union A,A',
      'venn-beta\n  set B\n  set A\n  union B,A\n  text B,A t',
      'venn-beta\n  set A\n    text one\n    text two\n  set B\n    text three\n  union A,B\n    text four',
      'venn-beta\n  set A\n  text one\n  text two',
      'venn-beta\n  set A\ntext one\n  text two',
      'venn-beta\n  set A\n  set B\n  text A,B one\n    text two',
      'venn-beta\n  set A\n\n    text one\n   \n    text two',
      'venn-beta\n  set A\n  %% c\n    text one',
      'venn-beta\n  set A\n    text one\n  title T\n    text two',
      'venn-beta\n  set A\n    text one\n  style A fill:red\n    text two',
      'venn-beta\n  set A\n    text one\n  text A x\n    text two',
      'venn-beta\n    text first',
      'venn-beta\n  title T\n    text first',
      'venn-beta\n  set A\n    textual',
      'venn-beta\n  set A\n    text',
      'venn-beta\n  set A\n    text one two',
      'venn-beta\n  set text\n  set style\n  set set',
      'venn-beta\n  set title',
      'venn-beta\n  set A\n  union A',
      'venn-beta\n  set A\n  union A,B',
      'venn-beta\n  set A\n  set B\n  union A,B,C,D',
      'venn-beta\n  union A,B\n  set A\n  set B',
      'venn-beta\n  set "A"\n  union A,"A"',
      'venn-beta\n  set A\n  set B\n  union "A" , "B" ["L"] : 2',
      'venn-beta\n  set A["a\nb"]',
      'venn-beta\n  set A[a\nb]',
      'venn-beta\n  set "a\nb"',
      'venn-beta\n  set A["x"]:1.5\n  set B["y"]:01\n  union A,B:0.25',
      'venn-beta\n  set A:1.2.3',
      'venn-beta\n  set A:1e3',
      'venn-beta\n  set A:--1',
      'venn-beta\n  set A : 5\n  set B :5',
      'venn-beta\n  style A fill:#12, stroke:#123456789',
      'venn-beta\n  style A fill:#ggg',
      'venn-beta\n  style A fill:rgb(1,2), stroke:rgb(1,2,3,4)',
      'venn-beta\n  style A fill:RGB(1,2,3), stroke:RGBA(1,2,3,4)',
      'venn-beta\n  style A a:b, c:d, e:"f g", h:1 2 #abc rgb(1,2,3) rgba(1,2,3,4) x',
      'venn-beta\n  style A,B,C fill:red\n  style C,B,A stroke:blue\n  style A,B,C fill:green',
      'venn-beta\n  style "A" fill:"#fff"',
      'venn-beta\n  style A fill: "quoted value" , color : red',
      'venn-beta\n  style A fill:red,',
      'venn-beta\n  style A fill:red stroke:blue',
      'venn-beta\n  title One\n  title Two',
      'venn-beta\n  title T # rest\n  set A',
      'venn-beta\n  title T ; rest\n  set A',
      'venn-beta\n  titleX\n  set A',
      'venn-beta\n  title\tTab\n  set A',
      'venn-beta\n  set A %% comment\n  set B',
      'venn-beta\n  set A }%% not a comment\n  set B',
      'venn-beta\n%% first\n  %% second\n  set A',
      'venn-beta %% header comment\n  set A',
      'venn-beta\r\n  set A\r\n    text one\r\n  set B\r\n',
      'venn-beta\r  set A\r  set B',
      'venn-beta\n\tset A\n\t\ttext one',
      'venn-beta\n  set A\n \t text one',
      'VENN-BETA\n  set A',
      'Venn-Beta\n  set A',
      'venn-beta\n  set A\n  set B\n  union A,B\n  text A,B "id"["label"]\n  style id color:red',
      'venn-beta\n  set A\n  text A "id"[""]\n  text A "id2"[" "]\n  text A ""\n  text A " "',
      'venn-beta\n  set A[""]\n  set B[" "]\n  set C["\\""]',
      'venn-beta\n  set """\n',
      'venn-beta\n  set "a"b"\n',
      'venn-beta\n  set A\n  set __proto__\n  set toString\n  union A,__proto__',
      'venn-beta\n  set A\n  set B\n  set C\n  set D\n  union A,B\n  union C,D\n  union A,B,C,D',
      'venn-beta\n  set 😀\n',
      'venn-beta\n  set "😀"["🎉 party"]\n  text "😀" "🎈"',
      '---\ntitle: Front matter title\n---\nvenn-beta\n  set A',
      '---\ntitle: Front matter title\n---\nvenn-beta\n  title Body title\n  set A',
      '---\nconfig:\n  securityLevel: loose\n---\nvenn-beta\n  title <b>bold</b> a < b\n  set A["<i>x</i>"]',
      '---\nconfig:\n  venn:\n    width: 400\n    height: 300\n---\nvenn-beta\n  set A',
      '%%{init: {"theme": "dark"}}%%\nvenn-beta\n  set A',
      'venn-beta\n  accTitle: Not in this grammar\n  set A',
      'venn-beta nonsense\n  set A',
      'venn-betax\n  set A',
      'venn-beta\n' + '  set A\n'.repeat(60),
    ],
  },
  ishikawa: {
    family: 'testdata/corpus/mermaid-e2e/ishikawa',
    out: 'internal/diagrams/ishikawa/testdata/db.json.gz',
    header: /^\s*ishikawa(-beta)?\b/i,
    headers: ['ishikawa-beta', 'ishikawa-beta', 'ishikawa', 'ISHIKAWA', 'Ishikawa-Beta', 'ishikawa-beta  ', 'ishikawa  \t'],
    snippets: [
      'Blurry Photo',
      'Process',
      'Out of focus',
      'User',
      'Shaky hands',
      'Equipment',
      'A cause with several words in it',
      'A very long cause label that goes over the wrap width of fifteen characters',
      'Line one<br>line two',
      'Line one<br/>line two<br />line three',
      'Text with <b>bold</b> and <i>italic</i>',
      '<script>alert(1)</script>Text',
      '<img src=x onerror=alert(1)> image',
      'a < b && c > d',
      'Entity #35; and #quot; and #9829;',
      'Ünïcödé cäuse ☃ 😀',
      '%% a comment',
      'Cause %% not a comment',
      'Cause with trailing spaces   ',
      'x',
      '1',
      '0',
      'ishikawa',
      'ishikawa-beta',
      'ishikawa nested header',
      ': ; , . !',
      '"quoted"',
      "'single'",
      'title Not a keyword',
      'accTitle: Not a keyword',
      'tab\tinside',
      'nbsp inside',
      '',
      '   ',
    ],
    extras: [
      'ishikawa',
      'ishikawa\n',
      'ishikawa-beta',
      'ishikawa-beta\n',
      'ishikawa-beta\n\n\n',
      'ishikawa-beta\n  \n',
      'ishikawa-beta\n   \n   \n',
      'ishikawa-beta\n%% only a comment\n',
      'ishikawa-beta Effect on the header line',
      'ishikawa-beta\nEffect',
      'ishikawa-beta\n  Effect',
      'ishikawa-beta\n  Effect\n',
      'ishikawa-beta\n  Effect\n  Cause',
      'ishikawa-beta\n  Effect\n  Cause\n    Sub',
      'ishikawa-beta\n  Effect\nCause at zero\n    Sub',
      'ishikawa-beta\nEffect\nCause\nCause 2\n Sub\n  SubSub\n Sub 2\nCause 3',
      'ishikawa-beta\n      Effect\n  Cause\n    Sub\n  Cause 2',
      'ishikawa-beta\n  Effect\n    Cause\n  Dedent below base\n Another\nZero',
      'ishikawa-beta\n  Effect\n    C1\n      S1\n        SS1\n      S2\n    C2\n  D\n    E',
      'ishikawa-beta\n  Effect\n  C1\n   S1\n    SS1\n     SSS1\n      SSSS1\n   S2',
      'ishikawa-beta\n\tEffect\n\tCause\n\t\tSub\n\t\t\tSubSub',
      'ishikawa-beta\n \tEffect\n\t Cause\n \t Sub',
      'ishikawa-beta\n  Effect\n\n  Cause\n\n\n    Sub\n\n',
      'ishikawa-beta\n  Effect\n   \n  Cause\n \t \n    Sub',
      'ishikawa-beta\n  Effect\n  %% comment\n  Cause\n    %% deeper comment\n    Sub',
      'ishikawa-beta\n  Effect %% trailing\n  Cause',
      'ishikawa-beta\n%% c1\n%% c2\n  Effect\n  Cause',
      '%% before\nishikawa-beta\n  Effect\n  Cause',
      '\n\n  \nishikawa-beta\n  Effect\n  Cause',
      '   ishikawa-beta\n  Effect\n  Cause',
      'ishikawa-beta\r\n  Effect\r\n  Cause\r\n    Sub\r\n',
      'ishikawa-beta\r  Effect\r  Cause',
      'ishikawa-beta\n  Effect   \n  Cause\t\n    Sub  ',
      'ishikawa-beta\n  Effect\n  Cause\n  Cause\n  Cause',
      'ishikawa-beta\n  Effect<br>second line\n  Cause<br/>two\n    Sub<br />three',
      'ishikawa-beta\n  <b>Effect</b>\n  <script>x</script>Cause\n    <img src=x onerror=y>Sub',
      'ishikawa-beta\n  Effect #35; #quot;\n  Cause #9829;',
      'ishikawa-beta\n  😀 Effect\n  🎉 Cause\n    ☃ Sub',
      'ishikawa-beta\n  Effect\n   nbsp indent\n   em space indent',
      'ishikawa-beta\n  Effect\n  Cause\nishikawa\n  ishikawa-beta',
      'ishikawa-betax\n  Effect',
      'ishikawax\n  Effect',
      'ishikawa-beta-x\n  Effect',
      'ishikawa -beta\n  Effect',
      '---\ntitle: Front matter title\n---\nishikawa-beta\n  Effect\n  Cause',
      '---\nconfig:\n  securityLevel: loose\n---\nishikawa-beta\n  <b>Effect</b> a < b\n  <i>Cause</i>',
      '---\nconfig:\n  securityLevel: antiscript\n---\nishikawa-beta\n  <b>Effect</b><script>x</script>\n  Cause',
      '---\nconfig:\n  htmlLabels: false\n---\nishikawa-beta\n  <b>Effect</b> a < b\n  Cause',
      '%%{init: {"theme": "dark"}}%%\nishikawa-beta\n  Effect\n  Cause',
      'ishikawa-beta\n  Effect\n' + '  Cause\n    Sub\n'.repeat(40),
      'ishikawa-beta\n  Effect\n' + Array.from({ length: 30 }, (_, i) => ' '.repeat(i + 2) + 'N' + i).join('\n'),
    ],
  },
};

// vennDocument makes a document that follows the venn grammar: sets, then
// unions of declared sets, each with indented text lines, then style lines,
// and at the end text statements that start in the first column. Such a
// line turns the indent mode off, so no indented text can follow it.
function vennDocument(rnd) {
  const pick = (arr) => arr[Math.floor(rnd() * arr.length)];
  const chance = (p) => rnd() < p;
  const pool = ['A', 'B', 'C', 'D', '"Quoted Set"', '"1"', '"10"', '"2"', 'a-b_c', 'constructor', 'Total', '"ünï ☃"', '"a,b"', '_x'];
  const ids = [];
  const n = 1 + Math.floor(rnd() * 5);
  while (ids.length < n) {
    const id = pick(pool);
    if (!ids.includes(id)) {
      ids.push(id);
    }
  }
  const indent = pick(['  ', '    ', '\t']);
  const lines = [pick(['venn-beta', 'venn-beta', 'venn-beta  '])];
  const noise = () => {
    if (chance(0.1)) {
      lines.push(pick(['', '   ', indent + '%% comment', '%% comment at the start']));
    }
  };
  const label = () => (chance(0.4) ? pick(['["Label"]', '[Unquoted label]', '[" "]', '[""]', '["a  b"]', '[ spaced ]', '["ünï 😀"]', ' ["L"]']) : '');
  const size = () => (chance(0.35) ? pick([':20', ': 12.5', ':.5', ':+3', ':0', ' : 7', ':0010', ':-0', ':1.25']) : '');
  const texts = () => {
    const k = chance(0.5) ? Math.floor(rnd() * 4) : 0;
    for (let i = 0; i < k; i++) {
      lines.push(
        indent +
          indent +
          pick(['text x1', 'text "x 2"', 'text 42', 'text x3["L3"]', 'text "x 4"["L4"]', 'text -1.5', 'TEXT up', 'text x5[unquoted]', 'text "q"[" "]', 'text "r"[""]', 'text .5', 'text x6 [spaced]', 'text  "two  spaces"'])
      );
      noise();
    }
  };
  if (chance(0.3)) {
    lines.push(indent + pick(['title A title', 'title Title: with colon', 'title T  spaced ', 'TITLE upper', 'title "quoted"']));
  }
  for (const id of ids) {
    lines.push(indent + pick(['set ', 'set ', 'SET ', 'Set ', 'set  ']) + id + label() + size());
    noise();
    texts();
  }
  const list = (k) => {
    const members = [];
    while (members.length < k) {
      const id = pick(ids);
      if (!members.includes(id) || chance(0.1)) {
        members.push(id);
      }
    }
    return members.join(pick([',', ', ', ' , ']));
  };
  if (ids.length >= 2) {
    const unions = Math.floor(rnd() * 4);
    for (let i = 0; i < unions; i++) {
      lines.push(indent + pick(['union ', 'union ', 'UNION ']) + list(Math.min(ids.length, 2 + Math.floor(rnd() * 3))) + label() + size());
      noise();
      texts();
    }
  }
  if (chance(0.25)) {
    lines.push(indent + 'title Late title');
  }
  const styles = chance(0.5) ? Math.floor(rnd() * 4) : 0;
  const field = () =>
    pick(['fill', 'stroke', 'color', 'fill-opacity', 'stroke-width', 'font', 'constructor', '__proto__', 'x_y']) +
    pick([':', ': ', ' : ']) +
    pick(['#ff6b6b', '#abc', '#12345678', 'red', 'rgb(1, 2, 3)', 'rgba(1,2,3,0.5)', '0.5', '2', '"quoted value"', '""', '" "', '1 2 3', 'bold 12 serif', 'a #fff rgb(0,0,0) 1']);
  for (let i = 0; i < styles; i++) {
    const fields = [field()];
    while (chance(0.4)) {
      fields.push(field());
    }
    lines.push(indent + pick(['style ', 'STYLE ']) + (chance(0.7) ? list(1 + Math.floor(rnd() * Math.min(3, ids.length))) : pick(['x1', '"x 2"', 'q'])) + ' ' + fields.join(pick([',', ', ', ' , '])));
    noise();
  }
  const explicit = chance(0.4) ? 1 + Math.floor(rnd() * 3) : 0;
  for (let i = 0; i < explicit; i++) {
    lines.push('text ' + list(1 + Math.floor(rnd() * Math.min(2, ids.length))) + ' ' + pick(['e1', '"e 2"', '7', 'e3["EL"]', '"e4"["EL4"]', 'e5[plain]', '-2.5', '"e6"[""]']));
  }
  return lines.join(pick(['\n', '\n', '\n', '\r\n'])) + pick(['', '\n', '\n\n']);
}

// ishikawaDocument makes a document that the ishikawa grammar accepts: a
// tree of lines with indents that grow and shrink, with comment lines and
// empty lines between them.
function ishikawaDocument(rnd) {
  const pick = (arr) => arr[Math.floor(rnd() * arr.length)];
  const chance = (p) => rnd() < p;
  const words = ['Effect', 'Process', 'User', 'Shaky hands', 'Out of focus', 'A cause with several words in it', 'Line one<br>line two', 'a < b', '<b>bold</b>', 'Entity #35;', 'Ünï ☃ 😀', 'x', '"q"', 'ishikawa', 'title T', 'With %% inside', 'tab\tinside', 'Trailing  '];
  const lines = [pick(['ishikawa-beta', 'ishikawa-beta', 'ishikawa', 'ISHIKAWA-BETA', 'Ishikawa'])];
  const unit = pick([' ', '  ', '  ', '    ', '\t']);
  let depth = Math.floor(rnd() * 3);
  const n = Math.floor(rnd() * 14);
  for (let i = 0; i < n; i++) {
    lines.push(unit.repeat(depth) + pick(words));
    if (chance(0.15)) {
      lines.push(pick(['', unit, unit.repeat(depth) + '%% comment', '%% comment', '  %% c  ']));
    }
    depth = Math.max(0, depth + pick([-2, -1, 0, 0, 1, 1, 1, 2]));
  }
  return lines.join('\n') + pick(['', '\n', '\n\n', '\n  \n']);
}

// mutate changes the lines after the header line.
function mutate(rnd, text, spec) {
  const lines = text.split('\n');
  let start = lines.findIndex((l) => spec.header.test(l));
  if (start < 0) {
    return text;
  }
  start++;
  const pick = () => start + Math.floor(rnd() * Math.max(1, lines.length - start));
  const edits = 1 + Math.floor(rnd() * 3);
  for (let i = 0; i < edits; i++) {
    const kind = Math.floor(rnd() * 7);
    const p = pick();
    if (kind === 0) {
      lines.splice(p, 1);
    } else if (kind === 1) {
      lines.splice(p, 0, lines[pick()] ?? '');
    } else if (kind === 2) {
      const q = pick();
      [lines[p], lines[q]] = [lines[q] ?? '', lines[p] ?? ''];
    } else if (kind === 3) {
      lines.splice(p, 0, '  ' + spec.snippets[Math.floor(rnd() * spec.snippets.length)]);
    } else if (kind === 4) {
      const l = lines[p] ?? '';
      const at = Math.floor(rnd() * (l.length + 1));
      lines[p] = l.slice(0, at) + l.slice(at + 1 + Math.floor(rnd() * 4));
    } else if (kind === 5) {
      // Change the indent of a line.
      const l = lines[p] ?? '';
      lines[p] = ' '.repeat(Math.floor(rnd() * 7)) + l.trimStart();
    } else {
      lines.length = Math.max(start, p);
    }
  }
  return lines.join('\n');
}

// randomDocument joins a header and random snippets with random indents.
function randomDocument(rnd, spec) {
  const pick = (arr) => arr[Math.floor(rnd() * arr.length)];
  const lines = [pick(spec.headers)];
  const n = Math.floor(rnd() * 10);
  for (let i = 0; i < n; i++) {
    lines.push(pick(['', '  ', '  ', '    ', '      ', '\t', ' ']) + pick(spec.snippets));
  }
  return lines.join(pick(['\n', '\n', '\n', '\n\n'])) + pick(['', '\n']);
}

const cases = [];
let seed = 0x76656e6e;
for (const [type, spec] of Object.entries(types)) {
  const rnd = prng(seed++);
  const fixtures = fs
    .readdirSync(path.join(repoRoot, spec.family), { recursive: true })
    .filter((f) => f.endsWith('.mmd'))
    .map((f) => spec.family + '/' + f.split(path.sep).join('/'))
    .sort();
  for (const f of fixtures) {
    const text = fs.readFileSync(path.join(repoRoot, f), 'utf8');
    const name = f.slice('testdata/corpus/mermaid-e2e/'.length);
    cases.push({ name, type, text });
    for (let i = 0; i < mutantsPerFixture; i++) {
      cases.push({ name: `${name}#mutant${i}`, type, text: mutate(rnd, text, spec) });
    }
  }
  for (let i = 0; i < randomDocuments; i++) {
    cases.push({ name: `${type}/random-${i}`, type, text: randomDocument(rnd, spec) });
  }
  spec.extras.forEach((text, i) => cases.push({ name: `${type}/extra-${i}`, type, text }));
  const document = type === 'venn' ? vennDocument : ishikawaDocument;
  for (let i = 0; i < validDocuments; i++) {
    cases.push({ name: `${type}/valid-${i}`, type, text: document(rnd) });
  }
}

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'venn-ishikawa-db-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases }));
execFileSync(
  process.execPath,
  [
    path.join(here, 'oracle.mjs'),
    'eval',
    path.join(here, 'venn-ishikawa-db-page.js'),
    '--args',
    argsFile,
    '--out',
    resultFile,
  ],
  { stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });

for (const [type, spec] of Object.entries(types)) {
  const own = results.filter((r) => r.type === type);
  const data = JSON.stringify({ mermaid: '12.2.0', cases: own }, null, 1) + '\n';
  const outFile = path.join(repoRoot, spec.out);
  fs.mkdirSync(path.dirname(outFile), { recursive: true });
  fs.writeFileSync(outFile, zlib.gzipSync(data, { level: 9 }));
  const failed = own.filter((r) => r.error !== undefined).length;
  console.error(`${spec.out}: ${own.length} cases, ${failed} with an error`);
}
