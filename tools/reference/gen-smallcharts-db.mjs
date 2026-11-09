#!/usr/bin/env node
// Writes internal/diagrams/{pie,packet,radar,quadrant}/testdata/db.json.gz:
// the content of the db of real mermaid after it parses
//   - pie, packet, radar: every text of the parser golden of the diagram
//     (testdata/ast.json.gz, from gen-langium.mjs) that the parser accepts,
//     which includes the corpus fixtures, and hand-written texts with a
//     config in the front matter,
//   - quadrant: the corpus fixtures, hand-written texts for the branches of
//     the grammar and of the db, and seeded mutations of these texts.
// A case that mermaid rejects (in the parser or in the db) has an error.
// The dbs sanitize their texts with DOMPurify, which needs a browser, so the
// script runs smallcharts-db-page.js with the eval command of the browser
// oracle, one time for the four diagram types.
//
// Usage (in tools/reference): node gen-smallcharts-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { prng } from './jison-lib.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');
const QUADRANT_MUTANTS = 500;

const readGz = (file) => JSON.parse(zlib.gunzipSync(fs.readFileSync(path.join(ROOT, file))));
const unique = (list) => [...new Set(list)];

function listFixtures(family) {
  const dir = path.join(ROOT, 'testdata/corpus/mermaid-e2e', family);
  return fs
    .readdirSync(dir, { recursive: true })
    .filter((f) => f.endsWith('.mmd'))
    .sort()
    .map((f) => fs.readFileSync(path.join(dir, f), 'utf8'));
}

// Texts with a config, for the config branches of the dbs.
const CONFIG_HAND = {
  pie: [
    '---\ntitle: Front matter title\n---\npie\n"a": 1\n',
    '---\nconfig:\n  securityLevel: loose\n---\npie title <b>T</b> <script>x</script>\n"a<i>b</i>": 1\n',
    'pie title <b>T</b> & <i>i</i> <script>alert(1)</script>\naccTitle: <b>A</b>\naccDescr: <b>D</b>\n"a": 1\n',
    'pie\naccDescr {\n  line <b>one</b>\n  line two\n}\n"a": 1\n',
    '%%{init: {"htmlLabels": false}}%%\npie title a<br>b\n"a": 1\n',
  ],
  packet: [
    '---\nconfig:\n  packet:\n    bitsPerRow: 8\n---\npacket\n0-3: "a"\n4-20: "b"\n21: "c"\n',
    '---\nconfig:\n  packet:\n    bitsPerRow: 16\n    showBits: false\n    paddingY: 3\n---\npacket\n+8: "a"\n+16: "b"\n+40: "c"\n',
    '---\nconfig:\n  packet:\n    bitsPerRow: 1\n---\npacket\n0-2: "a"\n',
    '---\nconfig:\n  packet:\n    bitsPerRow: 0\n---\npacket\n0-2: "a"\n',
    '---\nconfig:\n  packet:\n    bitsPerRow: 5\n    bitWidth: 10\n    rowHeight: 20\n    paddingX: 1\n---\npacket-beta\ntitle T\n0-11: "a"\n+3: "b"\n15-15: "c"\n',
    '%%{init: {"packet": {"bitsPerRow": 4, "showBits": true}}}%%\npacket\n0-9: "a <b>b</b>"\n',
    'packet\n0-31: "a"\n32-63: "b"\n64-95: "c"\n96-200: "d"\n',
    'packet\n0-100: "a"\n',
    'packet\ntitle <b>T</b>\naccTitle: A\naccDescr: D\n0: "x"\n',
  ],
  radar: [
    '---\nconfig:\n  radar:\n    width: 300\n    height: 200\n    axisScaleFactor: 1.5\n---\nradar-beta\naxis A,B,C\ncurve c{1,2,3}\n',
    '---\nconfig:\n  radar:\n    curveTension: 0.3\n    marginTop: 10\n    useMaxWidth: false\n---\nradar-beta\naxis A,B\ncurve c{1,2}\n',
    '%%{init: {"radar": {"axisLabelFactor": 2}}}%%\nradar-beta\ntitle <b>T</b>\naxis A["<i>a</i>"],B\ncurve c["<b>c</b>"]{B: 2, A: 1}\n',
    'radar-beta\naxis A,B,C\ncurve c{C: 3, A: 1, B: 2}, d{1,2,3}\nticks 40\nmax 10\nmin 1\nshowLegend false\ngraticule polygon\n',
    'radar-beta\naxis A,B,C\ncurve c{A: 1, B: 2}\n',
    'radar-beta\ncurve c{A: 1}\n',
    'radar-beta\naxis A,B\ncurve c{A: 1, A: 2, B: 3}\nticks 32\nticks 33\n',
    'radar-beta\naxis A,B\ncurve c{1,2,3}\nmax 2.5\nmin 0.5\nticks 2.5\n',
  ],
};

const QUADRANT_HAND = [
  'quadrantChart',
  'quadrantChart\n',
  '  quadrantChart  \n',
  '\n\n quadrantChart\n',
  ';quadrantChart;',
  'QUADRANTCHART\n',
  'quadrantChart\n  title Analytics and Business Intelligence Platforms\n',
  'quadrantChart\n  title   spaced   title  \n',
  'quadrantChart\n  title\n',
  'quadrantChart\n  TITLE Upper\n',
  'quadrantChart\n  accTitle: The acc title\n  accDescr: The acc description\n',
  'quadrantChart\n  accTitle   :   spaced  \n  accDescr   :   spaced  \n',
  'quadrantChart\n  accDescr {\n    multi line\n      description\n  }\n',
  'quadrantChart\n  accDescr { one line }\n  title after\n',
  'quadrantChart\n  x-axis Urgent --> Not Urgent\n  y-axis Not Important --> important\n',
  'quadrantChart\n  x-axis Urgent -->\n  y-axis Not Important -->\n',
  'quadrantChart\n  x-axis Urgent\n  y-axis Not Important\n',
  'quadrantChart\n  x-axis "Urgent(* +=[" --> "Not Urgent (* +=["\n',
  'quadrantChart\n  x-axis Urgent ----------> Not Urgent\n',
  'quadrantChart\n  x-axis Urgent--> Not Urgent\n',
  'quadrantChart\n  x-axis --> Right only\n',
  'quadrantChart\n  x-axis\n',
  'quadrantChart\n  y-axis "`md string`" --> "`other`"\n',
  'quadrantChart\n  X-AXIS a --> b\n  Y-Axis c --> d\n',
  'quadrantChart\n  quadrant-1 Plan\n  quadrant-2 Do\n  quadrant-3 Delegate\n  quadrant-4 Delete\n',
  'quadrantChart\n  quadrant-1 "Plan (* +=["\n  quadrant-2 "`md`"\n',
  'quadrantChart\n  quadrant-1 a - b_c.d,e+f=g*h#i&j\n',
  'quadrantChart\n  quadrant-1 Ünïcödé 😀 日本語\n',
  'quadrantChart\n  quadrant-5 Nope\n',
  'quadrantChart\n  quadrant-1\n',
  'quadrantChart\n  quadrant-1 a\n  quadrant-1 b\n',
  'quadrantChart\n  Point A: [0.1, 0.4]\n',
  'quadrantChart\n  Point A: [0.1, 0.4]\n  Point B: [1, 0]\n  Point C: [0, 1]\n',
  'quadrantChart\n  Point A:[0.1,0.4]\n',
  'quadrantChart\n  Point A  :  [  0.1  ,  0.4  ]  \n',
  'quadrantChart\n  "Point (A)": [0.25, 0.75]\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] radius: 10\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] radius: 10, color: #ff0000, stroke-color: #00ff00, stroke-width: 5px\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] color: #f00\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] color: ff0000\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] color: red\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] radius: f\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] radius: 1.5\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] radius\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] radius:\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] radius: 5 : 6\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] stroke-width: 10\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] stroke-width: 10px\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] stroke-color: #ff00ff\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] test_name: value\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] radius: 123456789012345678901234567890\n',
  'quadrantChart\n  Microsoft: [0.75, 0.75] radius: 007\n',
  'quadrantChart\n  Microsoft:::class1: [0.75, 0.75]\n  classDef class1 color: #109060, radius: 8\n',
  'quadrantChart\n  classDef class1 color: #109060\n  Microsoft:::class1: [0.75, 0.75] radius: 3\n',
  'quadrantChart\n  Microsoft:::missing: [0.75, 0.75]\n',
  'quadrantChart\n  Microsoft:::c1: [0.75, 0.75] color: #000000\n  classDef c1 color: #ffffff, stroke-width: 2px, stroke-color: #abc, radius: 4\n',
  'quadrantChart\n  classDef c1 color: #ffffff\n  classDef c1 radius: 4\n  A:::c1: [0.5, 0.5]\n',
  'quadrantChart\n  classDef c1 color: red\n',
  'quadrantChart\n  classDef c1\n',
  'quadrantChart\n  classDef  c1 color: #fff\n',
  'quadrantChart\n  classDef c-1 color: #fff\n  A:::c-1: [0.5, 0.5]\n',
  'quadrantChart\n  A: [0.5, 0.5] radius: 2;B: [0.1, 0.2]\n',
  'quadrantChart\n  A: [0.5]\n',
  'quadrantChart\n  A: [0.5, ]\n',
  'quadrantChart\n  A: [1.5, 0.5]\n',
  'quadrantChart\n  A: [0.5, 2]\n',
  'quadrantChart\n  A: [-0.5, 0.5]\n',
  'quadrantChart\n  A: [.5, 0.5]\n',
  'quadrantChart\n  A: [0x5, 0.5]\n',
  'quadrantChart\n  A: [0e5, 0b1]\n',
  'quadrantChart\n  A: [0a5, 0.5]\n',
  'quadrantChart\n  A: [0 5, 0.5]\n',
  'quadrantChart\n  A: [0.50, 0.123456789012345678]\n',
  'quadrantChart\n  A: [1, 1]\n  B: [0, 0]\n',
  'quadrantChart\n  A: [1.0, 0.5]\n',
  'quadrantChart\n  A: [0.5, 0.5\n',
  'quadrantChart\n  A: 0.5, 0.5]\n',
  'quadrantChart\n  : [0.5, 0.5]\n',
  'quadrantChart\n  A B C: [0.5, 0.5]\n  A-B: [0.2, 0.3]\n',
  'quadrantChart\n  "`md` point": [0.5, 0.5]\n',
  'quadrantChart\n  <b>bold</b>: [0.5, 0.5]\n',
  'quadrantChart\n  title <b>T</b> <script>alert(1)</script>\n  quadrant-1 a <i>b</i> & c\n',
  'quadrantChart\n  %% a comment\n  title T %% not a comment\n  x-axis a --> b %% c\n',
  'quadrantChart\n  x-axis a --> b; y-axis c --> d\n',
  'quadrantChart\n  x-axis a\n  x-axis b --> c\n',
  'quadrantChart\n\n\n  title T\n\n\n',
  'quadrantChart\r\n  title T\r\n  x-axis a --> b\r\n',
  'quadrantChart title T\n',
  'quadrantChart x-axis a --> b\n',
  'quadrantChart\n  section Foo\n',
  'quadrantChart\n  foo bar\n',
  'quadrantChart\n  x-axis a --> b --> c\n',
  'quadrantChart\n  title #35; entity &amp; x\n  quadrant-1 a#b;\n',
  '---\ntitle: Front matter title\nconfig:\n  quadrantChart:\n    chartWidth: 600\n    chartHeight: 400\n---\nquadrantChart\n  x-axis a --> b\n  A: [0.3, 0.6]\n',
  '---\nconfig:\n  quadrantChart:\n    xAxisPosition: bottom\n    yAxisPosition: right\n    titlePadding: 3\n    titleFontSize: 30\n    quadrantPadding: 7\n    xAxisLabelPadding: 2\n    yAxisLabelPadding: 9\n    xAxisLabelFontSize: 11\n    yAxisLabelFontSize: 13\n    quadrantLabelFontSize: 19\n    quadrantTextTopPadding: 4\n    pointTextPadding: 6\n    pointLabelFontSize: 9\n    pointRadius: 8\n    quadrantInternalBorderStrokeWidth: 3\n    quadrantExternalBorderStrokeWidth: 5\n---\nquadrantChart\n  title T\n  x-axis a --> b\n  y-axis c --> d\n  quadrant-1 q1\n  quadrant-2 q2\n  quadrant-3 q3\n  quadrant-4 q4\n  A: [0.3, 0.6]\n  B: [0.45, 0.23]\n',
  '---\nconfig:\n  quadrantChart:\n    xAxisPosition: bottom\n    yAxisPosition: right\n---\nquadrantChart\n  title T\n  x-axis a --> b\n  y-axis c --> d\n',
  '---\nconfig:\n  theme: dark\n---\nquadrantChart\n  title T\n  x-axis a\n  y-axis c\n  A: [0.3, 0.6]\n',
  '%%{init: {"theme": "forest", "quadrantChart": {"chartWidth": 333.5, "pointRadius": 2.5}}}%%\nquadrantChart\n  title T\n  x-axis a --> b\n  A: [0.3, 0.6]\n',
  '%%{init: {"themeVariables": {"quadrant1Fill": "#123456", "quadrantPointFill": "#654321", "quadrantTitleFill": "#111111"}}}%%\nquadrantChart\n  title T\n  A: [0.3, 0.6]\n',
];

const QUADRANT_SNIPPETS = [
  ...['quadrantChart', 'title T', 'x-axis a --> b', 'y-axis c -->', 'quadrant-1 q', 'quadrant-3 "q r"'],
  ...['A: [0.3, 0.6]', 'B:::c1: [0.9, 0.1] radius: 5', 'classDef c1 color: #abcdef', '[', ']', ':', ':::'],
  ...[',', ' ', '\n', '\n', ';', '"', '"`', '`"', '-->', '--', '-', '0.5', '1', '0', '.', '#', '%%'],
  ...['radius: 7', 'color: #f00', 'stroke-width: 2px', 'stroke-color: #00f', 'px', 'accTitle: a'],
  ...['accDescr: d', 'accDescr {\n x\n}', 'é', '😀', '&', '*', '+', '=', '_', 'x', '\t', '\r\n'],
];

// mutate returns a variant of text with one to three random edits.
function mutate(text, rand, snippets) {
  const pick = (list) => list[Math.floor(rand() * list.length)];
  let s = text;
  const edits = 1 + Math.floor(rand() * 3);
  for (let e = 0; e < edits; e++) {
    const at = Math.floor(rand() * (s.length + 1));
    const kind = Math.floor(rand() * 8);
    if (kind === 0) {
      s = s.slice(0, at) + s.slice(at + 1 + Math.floor(rand() * 3));
    } else if (kind <= 3) {
      s = s.slice(0, at) + pick(snippets) + s.slice(at);
    } else if (kind === 4) {
      s = s.slice(0, at) + pick(snippets) + s.slice(at + 1);
    } else if (kind <= 6) {
      const lines = s.split('\n');
      const i = Math.floor(rand() * lines.length);
      const j = Math.floor(rand() * lines.length);
      if (kind === 5) {
        lines.splice(i, rand() < 0.5 ? 0 : 1, ...(rand() < 0.5 ? [lines[j]] : []));
      } else {
        [lines[i], lines[j]] = [lines[j], lines[i]];
      }
      s = lines.join('\n');
    } else {
      s = s.slice(0, at);
    }
  }
  return s;
}

// Collect the cases.
const cases = [];
for (const type of ['pie', 'packet', 'radar']) {
  const valid = readGz(`internal/diagrams/${type}/testdata/ast.json.gz`)
    .filter((c) => c.ast !== undefined)
    .map((c) => c.text);
  for (const text of unique([...valid, ...CONFIG_HAND[type]])) {
    cases.push({ type, text });
  }
}
{
  const seeds = unique([...listFixtures('quadrant-chart'), ...QUADRANT_HAND]);
  const rand = prng(0x71756164);
  const texts = [...seeds];
  for (let i = 0; i < QUADRANT_MUTANTS; i++) {
    texts.push(mutate(seeds[Math.floor(rand() * seeds.length)], rand, QUADRANT_SNIPPETS));
  }
  for (const text of unique(texts)) {
    cases.push({ type: 'quadrantChart', text });
  }
}

// Run the page.
const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'smallcharts-db-'));
let results;
try {
  const argsFile = path.join(dir, 'args.json');
  const outFile = path.join(dir, 'out.json');
  fs.writeFileSync(argsFile, JSON.stringify({ cases }));
  execFileSync(
    process.execPath,
    [path.join(HERE, 'oracle.mjs'), 'eval', path.join(HERE, 'smallcharts-db-page.js'), '--args', argsFile, '--out', outFile],
    { cwd: HERE, stdio: ['ignore', 'inherit', 'inherit'], maxBuffer: 1 << 30 }
  );
  results = JSON.parse(fs.readFileSync(outFile, 'utf8'));
} finally {
  fs.rmSync(dir, { recursive: true, force: true });
}

const packages = { pie: 'pie', packet: 'packet', radar: 'radar', quadrantChart: 'quadrant' };
for (const [type, pkg] of Object.entries(packages)) {
  const list = results.filter((r) => r.type === type).map(({ text, db, error }) => ({ text, db, error }));
  const ok = list.filter((r) => r.error === undefined).length;
  const file = path.join(ROOT, `internal/diagrams/${pkg}/testdata/db.json.gz`);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  // The gzip header has no time stamp, so the same data gives the same file.
  fs.writeFileSync(file, zlib.gzipSync(JSON.stringify(list), { level: 9 }));
  const size = (fs.statSync(file).size / 1024).toFixed(1);
  console.log(`${path.relative(ROOT, file)}: ${list.length} cases, ${ok} without error, ${size} kB`);
}
