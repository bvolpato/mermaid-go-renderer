#!/usr/bin/env node
// Writes internal/diagrams/treeview/testdata/units.json.gz: goldens for the
// pure functions of the treeView diagram of mermaid 12.2.0, from the real
// code:
//
//   boxDrawing  preprocessBoxDrawing(input): the text and the line map, or
//               the error.
//   remap       remapErrorLines(message, lineMap).
//   icons       getNodeIcon(node, config): the icon reference, undefined,
//               or the error.
//
// The functions are not exports of the build. A module load hook appends
// one export statement to the chunk of the treeView diagram. No other line
// of mermaid changes.
//
// Run it with tools/reference as the working directory:
//
//   node gen-treeview-units.mjs
import { registerHooks } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath, pathToFileURL } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');
const CHUNKS = path.join(HERE, 'node_modules/mermaid/dist/chunks/mermaid.core');
const OUT = path.join(ROOT, 'internal/diagrams/treeview/testdata/units.json.gz');

const chunk = fs
  .readdirSync(CHUNKS)
  .find((f) => f.endsWith('.mjs') && fs.readFileSync(path.join(CHUNKS, f), 'utf8').includes('function preprocessBoxDrawing('));
if (!chunk) {
  throw new Error('no chunk has preprocessBoxDrawing');
}
registerHooks({
  load(url, context, nextLoad) {
    const result = nextLoad(url, context);
    if (url.endsWith('/' + chunk)) {
      const source =
        result.source.toString() +
        '\nexport { preprocessBoxDrawing as __preprocessBoxDrawing, remapErrorLines as __remapErrorLines, getNodeIcon as __getNodeIcon };\n';
      return { ...result, source };
    }
    return result;
  },
});
const mod = await import(pathToFileURL(path.join(CHUNKS, chunk)).href);
const { __preprocessBoxDrawing: preprocessBoxDrawing, __remapErrorLines: remapErrorLines, __getNodeIcon: getNodeIcon } = mod;

// mulberry32, the generator that the golden scripts share.
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
const rnd = prng(0x626f7864);
const pick = (a) => a[Math.floor(rnd() * a.length)];
const chance = (p) => rnd() < p;

// ---------------------------------------------------------------------------
// preprocessBoxDrawing

const boxInputs = [
  'treeView-beta\nroot/\n├── a.txt\n└── b.txt\n',
  'treeView-beta\nroot/\n├── src/\n│   ├── index.js\n│   └── app.js\n└── README.md\n',
  'treeView-beta\nroot/\n┣━━ src/\n┃   ┣━━ index.js\n┃   ┗━━ app.js\n┗━━ README.md\n',
  'treeView-beta\nroot/\n├─ a\n│  ├─ b\n│  │  └─ c\n└─ d\n',
  'treeView-beta\nroot/\n├── a\n│\n│\n└── b\n',
  'treeView-beta\n├── a\n└── b\n',
  'treeView-beta\n├── a\n    ├── b\n',
  'treeView-beta\n  ├── a\n      └── b\n',
  'treeView-beta\nroot/\n├── \n',
  'treeView-beta\nroot/\n├──\n',
  'treeView-beta\nroot/\n├\n',
  'treeView-beta\nroot/\n├── a\n  plain indented\n',
  'treeView-beta\nroot/\n├── a\nplain root\n',
  'treeView-beta\nroot/\n├── a\nSection ─ A.txt\n',
  'treeView-beta\nroot/\n├── a\n  Section ─ A.txt\n',
  'treeView-beta\nroot/\n├── a\n───\n',
  'treeView-beta\nroot/\n├── a\n │ ─ ━ ┃\n',
  'treeView-beta\nroot/\n\t├── a\n\t\t└── b\n',
  'treeView-beta\nroot/\n├──\ta\n',
  'treeView-beta\nroot/\n├── a\t\n',
  'treeView-beta\ntitle My tree\naccTitle: acc\naccDescr: descr\nroot/\n├── a\n',
  'treeView-beta\n  title My tree\n  accTitle : acc\n  accDescr {\n├── a\n',
  'treeView-beta\ntitle\n├── a\n',
  'treeView-beta\ntitled.txt\n├── a\n',
  'treeView-beta\naccDescr{x}\n├── a\n',
  'treeView-beta\n%% comment\nroot/\n  %% indented comment ├──\n├── a\n',
  'treeView-beta\nroot/\n├── a %% trailing\n',
  'treeView-beta\nroot/\n├── a :::highlight icon(fa:bell) ## desc\n└── "quoted name"\n',
  'treeView-beta\n🚀 app/\n├── 📦 a\n🚀├── b\n',
  'treeView-beta\nroot/\n    ├── deep\n├── shallow\n',
  'treeView-beta\nroot/\n├── a\n│   ├── b\n│   │   ├── c\n│   │   │   └── d\n',
  'treeView-beta\nroot/\n├──── a\n├─ b\n├ c\n├c\n├   d\n',
  'treeView-beta\nroot/\n├── a ├── b\n',
  'treeView-beta\nroot/\n└── a └\n',
  'treeView-beta\r\nroot/\r\n├── a\r\n└── b\r\n',
  'treeView-beta\n\n\nroot/\n\n├── a\n\n',
  '  treeView-beta  \nroot/\n├── a\n',
  'treeView-beta  \nroot/\n├── a\n',
  'x\ntreeView-beta\nroot/\n├── a\n',
  'treeView-beta x\nroot/\n├── a\n',
  'treeView\n├── a\n',
  '├── a\n',
  '',
  'treeView-beta',
  'treeView-beta\n',
  'treeView-beta\n  a\n    b\n',
  'treeView-beta\n│\n',
  'treeView-beta\n│ │\n  a\n',
  'treeView-beta\nroot/\n├── a\n  x\n',
  'treeView-beta\nroot/\n├── a\n ├── b\n',
  'treeView-beta\nroot/\n├── a\n　plain\n',
  'treeView-beta\ntreeView-beta\n├── a\n',
  'treeView-beta\nroot/\n├── a\ntreeView-beta\n',
  'treeView-beta\na\n' + '├── x\n'.repeat(3) + '      └── far\n',
];

const names = ['a', 'b.txt', 'src/', 'my file.md', '"quoted"', 'x :::highlight', 'y icon(fa:bell)', 'z ## d', '🚀 r/', 'é', 'title', 'a ─ b', 'c │ d'];
function randomBoxInput() {
  const heavy = chance(0.25);
  const [branch, last, dash, bar] = heavy ? ['┣', '┗', '━', '┃'] : ['├', '└', '─', '│'];
  const seg = pick([2, 3, 4, 4, 4, 5, 6]);
  const dashes = dash.repeat(pick([0, 1, 2, 2, 2, 3]));
  const lines = [];
  if (chance(0.15)) lines.push(pick(['', '%% c', '  ']));
  lines.push(pick(['treeView-beta', 'treeView-beta', 'treeView-beta', ' treeView-beta', 'treeView-beta  ', 'treeView-betax']));
  if (chance(0.3)) lines.push(pick(['title T', '  title T', 'accTitle: A', 'accDescr: D', 'title', 'accDescr {']));
  if (chance(0.7)) lines.push(pick(names));
  const count = 1 + Math.floor(rnd() * 7);
  let depth = 0;
  for (let i = 0; i < count; i++) {
    depth = Math.max(0, Math.min(4, depth + pick([-2, -1, 0, 0, 1, 1])));
    let prefix = '';
    for (let d = 0; d < depth; d++) {
      prefix += (chance(0.5) ? bar : ' ') + ' '.repeat(seg - 1);
    }
    if (chance(0.08)) prefix = prefix.replace(/ {4}/, '\t');
    if (chance(0.05)) prefix = pick(['🚀', 'ab', ' ', ' ']) + prefix;
    let line = prefix + (chance(0.5) ? branch : last) + dashes + pick([' ', ' ', ' ', '', '  ', '\t']) + (chance(0.05) ? '' : pick(names));
    const k = rnd();
    if (k < 0.05) line = prefix + bar;
    else if (k < 0.08) line = prefix + pick(names);
    else if (k < 0.1) line = pick(['', '  ', '%% c', '  %% c']);
    else if (k < 0.12) line = dash.repeat(3);
    else if (k < 0.14) line += pick([' ', '\t', '\r', ' ' + branch]);
    lines.push(line);
  }
  return lines.join(pick(['\n', '\n', '\n', '\n', '\r\n'])) + pick(['\n', '\n', '']);
}
for (let i = 0; i < 1500; i++) {
  boxInputs.push(randomBoxInput());
}

const boxDrawing = [...new Set(boxInputs)].map((input) => {
  try {
    const { text, lineMap } = preprocessBoxDrawing(input);
    return { input, text, lineMap: [...lineMap] };
  } catch (e) {
    return { input, error: String(e?.message ?? e) };
  }
});

// ---------------------------------------------------------------------------
// remapErrorLines

const lineMaps = [
  [],
  [[1, 1], [2, 3], [3, 7]],
  [[1, 4], [2, 5], [10, 20], [100, 250]],
  [[2, 0], [3, 9]],
];
const messages = [
  'Parse error on line 2, column 5: Expecting token',
  'Parsing failed: Lexer error on line 3, column 1: x\nParse error on line 10, column 2: y',
  'line 2 and Line 3 and LINE  10 and line\t100',
  'online 2 line2 line 2x line 22 line 2.5 underline 3 line_2 line-3',
  'line 1line 2',
  'no numbers here',
  'line 99999999999999999999 line 0 line 007 line 03',
  'line 3 line\n2 line 3',
  '',
];
const remap = [];
for (const entries of lineMaps) {
  for (const message of messages) {
    remap.push({ message, lineMap: entries, result: remapErrorLines(message, new Map(entries)) });
  }
}

// ---------------------------------------------------------------------------
// getNodeIcon

const fileNames = [
  'README.md', 'index.ts', 'main.py', 'data.xyz', 'unmapped.bin', 'Makefile', '.env', '.gitignore', 'a.b.c', 'UPPER.TS',
  'noext', 'trailing.', '', 'constructor', 'toString', '__proto__', 'valueOf', 'hasOwnProperty', 'x.constructor',
  'x.toString', 'x.tostring', 'x.__proto__', 'x.valueOf', 'é.Ü', 'a.İ', 'dir.d/', 'name with space.txt', 'none', 'x.none',
  'archive.tar.gz', '.', '..', 'a..b', '0', '1', 'length', 'x.length', 'x.0', 'trim', 'x.at', 'x.map', 'join', 'toFixed', 'x.tofixed',
];
const iconValues = [
  'typescript', 'fa:bell', 'none', '', 'folder', 'file', 'constructor', 'pack:name:extra', 5, 0, null, true, false,
  'typescript', 'fa:bell', 'none', [], [':'], ['a'], ['fa:bell'], ['a', ':'], ['none'], {}, { a: 'b' },
];
const nodeIconValues = [undefined, undefined, undefined, 'none', 'folder', 'file', 'fa:bell', 'bell', 'constructor', 'toString', '__proto__', 'a:b'];
const packs = ['', '', 'material-icon-theme', 'fa', 5, 0, null, true];
const randomMap = () => {
  if (chance(0.15)) return pick([undefined, null, 5, 'text', 'README.md', [], ['a:b', 'none', 'x'], true]);
  const m = {};
  const n = Math.floor(rnd() * 5);
  for (let i = 0; i < n; i++) {
    const name = pick(fileNames);
    const key = pick([name, name, '.' + name.split('.').pop(), name.split('.').pop(), '.ts', 'ts', '.md', 'py', '.constructor', 'b.c', '.ü']);
    m[key] = pick(iconValues);
  }
  return m;
};
// encode keeps undefined: JSON has no text for it.
const encode = (v) => {
  if (v === undefined) return { $u: 1 };
  if (v === null || typeof v !== 'object') return v;
  if (Array.isArray(v)) return v.map(encode);
  const o = {};
  for (const k of Object.keys(v)) o[k] = encode(v[k]);
  return o;
};
const icons = [];
for (let i = 0; i < 3000; i++) {
  const node = { name: pick(fileNames), nodeType: pick(['file', 'file', 'directory']), icon: pick(nodeIconValues) };
  const config = {
    showIcons: pick([true, true, true, false, 1, 0, 'yes', null]),
    defaultIconPack: pick(packs),
    filenameIcons: randomMap(),
    extensionIcons: randomMap(),
  };
  const c = { node: encode(node), config: encode(config) };
  try {
    c.result = encode(getNodeIcon(node, config));
  } catch (e) {
    c.error = String(e?.message ?? e);
  }
  icons.push(c);
}

fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, zlib.gzipSync(Buffer.from(JSON.stringify({ boxDrawing, remap, icons })), { level: 9 }));
const boxErrors = boxDrawing.filter((c) => c.error !== undefined).length;
const boxChanged = boxDrawing.filter((c) => c.error === undefined && c.text !== c.input).length;
const iconErrors = icons.filter((c) => c.error !== undefined).length;
console.log(
  `${path.relative(ROOT, OUT)}: ${boxDrawing.length} box-drawing cases (${boxChanged} that change the text, ${boxErrors} errors), ` +
    `${remap.length} remap cases, ${icons.length} icon cases (${iconErrors} errors), ${(fs.statSync(OUT).size / 1024).toFixed(1)} kB`
);
