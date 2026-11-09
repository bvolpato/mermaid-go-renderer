// Writes internal/diagrams/mindmap/testdata/db.json.gz: the layout data that
// the mindmap renderer of real mermaid hands to the layout (the nodes and
// edges of getData with the changes of the renderer, the config, the node
// tree, the direction, the markers, the spacing, the shapes, the layout
// algorithm), for
//   - every fixture of the corpus that mermaid renders as a mindmap,
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of mindmapDb.ts and of the grammar.
// A case that mermaid rejects has the error message instead.
//
// The db needs a browser (DOMPurify), so the script runs mindmap-db-page.js
// with the esm-eval command of the browser oracle, one time. A corpus
// fixture runs in a new page, as in the reference renders, and so does a
// case with a site config.
//
// Usage (in tools/reference): node gen-mindmap-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const corpusDir = 'testdata/corpus/mermaid-e2e';
const refDir = '.parity/ref/mermaid-e2e';
const family = 'mindmap/';
const outFile = path.join(repoRoot, 'internal/diagrams/mindmap/testdata/db.json.gz');
const mutantsPerFixture = 3;

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

// ---------------------------------------------------------------------------
// The fixtures of the corpus: every fixture of the mindmap family, and the
// fixtures of other families that the reference rendered as a mindmap.

const fixtures = fs
  .readdirSync(path.join(repoRoot, corpusDir), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => f.split(path.sep).join('/'))
  .filter((f) => {
    if (f.startsWith(family)) {
      return true;
    }
    try {
      const meta = JSON.parse(fs.readFileSync(path.join(repoRoot, refDir, f.replace(/\.mmd$/, '.json')), 'utf8'));
      return meta.diagramType === 'mindmap';
    } catch {
      return false;
    }
  })
  .sort();

const cases = [];
for (const f of fixtures) {
  cases.push({ name: `${corpusDir}/${f}`, corpus: true, fresh: true, text: fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8') });
}

// ---------------------------------------------------------------------------
// Seeded mutations.

const snippets = [
  'child',
  '  deeper',
  '      much deeper',
  'id[Square]',
  'id(Rounded)',
  'id((Circle))',
  'id))Bang((',
  'id)Cloud(',
  'id{{Hexagon}}',
  '[No id square]',
  '((no id circle))',
  '{{no id hexagon}}',
  'id["Quoted **bold** text"]',
  'id["`Markdown *string*\nsecond line`"]',
  '::icon(fa fa-book)',
  '::icon(mdi mdi-fire)',
  ':::urgent large',
  ':::a',
  '%% a comment',
  '',
  '   ',
  'id(-odd-)',
  'id[unclosed',
  'id))',
  ')',
  '}}',
  'a b c d',
  'Ünïcödé 日本語 😀',
  'id[#35; entity #lt;b#gt;]',
  'id["<b>html</b> <script>alert(1)</script>"]',
];

// mutate changes the lines after the line of the diagram keyword.
function mutate(rnd, text) {
  const lines = text.split('\n');
  // Skip the front matter: its text can have a line that starts with "mindmap".
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    from = close < 0 ? 0 : close + 1;
  }
  let start = lines.findIndex((l, i) => i >= from && /^\s*mindmap/.test(l));
  if (start < 0) {
    return text;
  }
  start++;
  const pick = () => start + Math.floor(rnd() * Math.max(1, lines.length - start));
  const indent = () => ' '.repeat(Math.floor(rnd() * 9));
  const edits = 1 + Math.floor(rnd() * 3);
  for (let i = 0; i < edits; i++) {
    const kind = Math.floor(rnd() * 8);
    const p = pick();
    if (kind === 0) {
      lines.splice(p, 1);
    } else if (kind === 1) {
      lines.splice(p, 0, lines[pick()] ?? '');
    } else if (kind === 2) {
      const q = pick();
      [lines[p], lines[q]] = [lines[q] ?? '', lines[p] ?? ''];
    } else if (kind === 3 || kind === 4) {
      lines.splice(p, 0, indent() + snippets[Math.floor(rnd() * snippets.length)]);
    } else if (kind === 5) {
      const l = lines[p] ?? '';
      const at = Math.floor(rnd() * (l.length + 1));
      lines[p] = l.slice(0, at) + l.slice(at + 1 + Math.floor(rnd() * 4));
    } else if (kind === 6) {
      // Change the indentation of a line.
      lines[p] = indent() + (lines[p] ?? '').trimStart();
    } else {
      lines.length = Math.max(start, p);
    }
  }
  return lines.join('\n');
}

const rnd = prng(0x6d6d4462);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const mm = (body, front = '') => front + 'mindmap\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const tree = 'root((Root))\n  A[Square]\n    A1(Rounded)\n      A11))Bang((\n    A2)Cloud(\n  B{{Hexagon}}\n    B1\n  C\n  D\n  E\n  F\n  G\n  H\n  I\n  J\n  K\n  L\n  M\n  N';
const extras = [
  // Headers.
  'mindmap\nroot\n',
  'mindmap',
  'mindmap\n',
  'mindmap\n\n\n',
  '\n\n   mindmap   \n\n\nroot\n\n',
  'mindmap root\n',
  'MINDMAP\nroot\n',
  'Mindmap\nroot\n',
  'mindmapX\nroot\n',
  'mindmap;\nroot\n',
  'mindmap\n%% only a comment\n',
  'mindmap\n   \n  \n',
  '%% comment first\nmindmap\nroot\n',
  '   \nmindmap\nroot\n',
  // Every node type, with and without an id.
  mm('root'),
  mm('root[Square]'),
  mm('root(Rounded)'),
  mm('root((Circle))'),
  mm('root))Bang(('),
  mm('root)Cloud('),
  mm('root{{Hexagon}}'),
  mm('[Square]'),
  mm('(Rounded)'),
  mm('((Circle))'),
  mm('))Bang(('),
  mm(')Cloud('),
  mm('{{Hexagon}}'),
  mm('root(-Odd-)'),
  mm('root(-Odd)'),
  mm('root(Odd-)'),
  mm('root((Mixed)'),
  mm('root(Mixed))'),
  mm('root[Mixed)'),
  mm('root(Mixed]'),
  mm('root{{Mixed))'),
  mm('root))Mixed)'),
  mm('root)Mixed(('),
  mm('root[Square] trailing'),
  mm('root[a][b]'),
  mm('root[]'),
  mm('root[ ]'),
  mm('root(())'),
  // Descriptions.
  mm('root["Quoted text"]'),
  mm('root["Quoted (with) [brackets] {{and}} more"]'),
  mm('root["`Markdown **bold** and *italic*`"]'),
  mm('root["`Markdown\n  on two lines`"]'),
  mm('root["`unterminated markdown]'),
  mm('root["unterminated]'),
  mm('root[""]'),
  mm('root["a" "b"]'),
  mm('root[text with (parens) inside]'),
  mm('root[text with spaces   and   more]'),
  mm('root[#35; hash #lt;b#gt; #9829;]'),
  mm('root[<b>bold</b> <i>it</i> <br/> <script>alert(1)</script>]'),
  mm('root["<img src=x onerror=alert(1)>"]'),
  mm('root[Ünïcödé 日本語 емкость 😀]'),
  mm('Ünïcödé\n  日本語\n  😀'),
  mm('root[A very long label that goes on and on and on and on so that it must wrap somewhere along the way]'),
  mm('root[line1<br/>line2<br>line3]'),
  mm('a b c\n  d e f'),
  mm('root\n  a:b\n  c;d\n  e,f\n  g.h\n  i-j\n  k_l\n  m"n\n  o`p'),
  // Indentation and levels.
  mm('root\n  A\n  B\n  C'),
  mm('root\n  A\n    A1\n      A11\n        A111\n  B'),
  mm('root\n      A\n   B\n     C\n  D'),
  mm('   root\n      A\n        A1\n      B'),
  mm('root\nsecond'),
  mm('  root\n  second'),
  mm('  root\n second'),
  mm('    root\n  A'),
  mm('root\n\tA\n\t\tA1\n\tB'),
  mm('root\n  A\n\n\n  B\n   \n  C'),
  mm('root\n  A\n%% comment\n  B\n  %% indented comment\n  C'),
  mm('root\n  A %% trailing comment\n  B'),
  mm('root\r\n  A\r\n  B'),
  mm(tree),
  // Icons and classes.
  mm('root\n  ::icon(fa fa-book)'),
  mm('root\n  A\n  ::icon(mdi mdi-skull-outline)\n  B\n    ::icon(fa fa-fire)'),
  mm('root\n::icon(fa fa-book)'),
  mm('root\n  :::urgent large'),
  mm('root\n  A\n  :::cls\n  B\n    :::a b c'),
  mm('root\n:::rootClass'),
  mm('root\n  ::icon(fa fa-book)\n  :::urgent'),
  mm('root\n  :::urgent\n  ::icon(fa fa-book)'),
  mm('root\n  ::icon(first)\n  ::icon(second)\n  :::one\n  :::two'),
  mm('::icon(fa fa-book)\nroot'),
  mm(':::cls\nroot'),
  mm('  ::icon(fa fa-book)\n  root'),
  mm('  :::cls\n  root'),
  mm('root\n  ::icon()'),
  mm('root\n  ::icon(unclosed'),
  mm('root\n  ::icon(<b>x</b>)'),
  mm('root\n  ::icon(<script>alert(1)</script>)'),
  mm('root\n  :::<script>alert(1)</script>'),
  mm('root\n  :::'),
  mm('root\n  ::icon(a)b'),
  mm('root\n  A::icon(a)'),
  mm('root\n  A:::cls'),
  // Errors of the db.
  mm('root\nsecond root'),
  mm('  root\nless indented'),
  mm('root[One]\nother[Two]'),
  // Errors of the grammar.
  mm('root[unclosed'),
  mm('root(unclosed'),
  mm('root((unclosed'),
  mm('root{{unclosed'),
  mm('root))unclosed'),
  mm('root]'),
  mm('root}}'),
  mm('root}'),
  mm('root{single}'),
  mm('root\n  ]'),
  mm('root\n  )'),
  mm('root\n  }}'),
  mm('root\n  [a\n  b]'),
  'mindmap\nroot\nmindmap\nother',
  // Config.
  mm(tree, cfg('mindmap:\n  padding: 20\n  maxNodeWidth: 100')),
  mm(tree, cfg('mindmap:\n  colorBy: depth')),
  mm(tree, cfg('mindmap:\n  colorBy: branch')),
  mm(tree, cfg('mindmap:\n  useMaxWidth: false')),
  mm(tree, cfg('layout: elk')),
  mm(tree, cfg('layout: dagre')),
  mm(tree, cfg('layout: cose-bilkent')),
  mm(tree, cfg('layout: tidy-tree')),
  mm(tree, cfg('layout: no-such-layout')),
  mm(tree, cfg('look: handDrawn')),
  mm(tree, cfg('look: neo')),
  mm(tree, cfg('look: classic')),
  mm(tree, cfg('theme: redux')),
  mm(tree, cfg('theme: redux-dark')),
  mm(tree, cfg('theme: redux-color')),
  mm(tree, cfg('theme: neo')),
  mm(tree, cfg('theme: forest')),
  mm(tree, cfg('theme: dark')),
  mm(tree, cfg('theme: base\nthemeVariables:\n  primaryColor: "#ff0000"')),
  mm(tree, cfg('htmlLabels: false')),
  mm(tree, cfg('securityLevel: loose')),
  mm(tree, cfg('securityLevel: antiscript')),
  mm(tree, '%%{init: {"mindmap": {"padding": 5}}}%%\n'),
  mm(tree, '%%{init: {"layout": "elk", "theme": "redux"}}%%\n'),
  mm('root\n  A', '---\ntitle: A title\n---\n'),
];
extras.forEach((text, i) => cases.push({ name: `extra-${i}`, text }));

// Site configs: the argument of mermaid.initialize. Such a case runs in a
// new page.
const sites = [
  { mindmap: { padding: 30, maxNodeWidth: 150 } },
  { mindmap: { colorBy: 'depth' } },
  { layout: 'elk' },
  { layout: 'dagre' },
  { theme: 'redux' },
  { look: 'handDrawn' },
  { htmlLabels: false },
  { securityLevel: 'loose' },
  { theme: 'dark', look: 'neo' },
];
sites.forEach((site, i) => {
  cases.push({ name: `site-${i}`, fresh: true, site, text: mm(tree) });
  cases.push({ name: `site-${i}-directive`, fresh: true, site, text: mm(tree, cfg('layout: cose-bilkent\nmindmap:\n  padding: 12')) });
});

// ---------------------------------------------------------------------------
// Run the page script.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-mindmap-db-'));
const argsFile = path.join(tmp, 'args.json');
const rawFile = path.join(tmp, 'raw.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases: cases.map(({ name, text, fresh, site }) => ({ name, text, fresh, site })) }));
execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'mindmap-db-page.js'), '--args', argsFile, '--out', rawFile], {
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
const raw = JSON.parse(fs.readFileSync(rawFile, 'utf8'));
if (raw.cases.length !== cases.length) {
  throw new Error(`${raw.cases.length} results for ${cases.length} cases`);
}

// The config is the same large object in every case. The file has the config
// of the first corpus fixture one time, and each case has its changes: the
// paths that are set to another value and the paths that are absent.
function flatten(v, prefix, out) {
  if (v !== null && typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length > 0 && !('$num' in v)) {
    for (const k of Object.keys(v)) {
      flatten(v[k], [...prefix, k], out);
    }
  } else {
    out.set(JSON.stringify(prefix), v);
  }
  return out;
}
const baseConfig = raw.cases.find((c) => c.config).config;
const baseFlat = flatten(baseConfig, [], new Map());
const stats = { total: cases.length, errors: 0, empty: 0, layouts: {} };
const out = raw.cases.map((res, i) => {
  const c = cases[i];
  if (res.name !== c.name) {
    throw new Error(`case ${i}: ${res.name} is not ${c.name}`);
  }
  const rec = { ...res };
  if (!c.corpus) {
    rec.text = c.text;
  }
  if (c.fresh) {
    rec.fresh = true;
  }
  if (c.site) {
    rec.site = c.site;
  }
  if (res.error !== undefined) {
    stats.errors++;
  }
  if (res.empty) {
    stats.empty++;
  }
  if (res.layout) {
    stats.layouts[res.layout] = (stats.layouts[res.layout] || 0) + 1;
  }
  if (res.config) {
    const flat = flatten(res.config, [], new Map());
    const set = [];
    const del = [];
    for (const [k, v] of flat) {
      if (!baseFlat.has(k) || JSON.stringify(baseFlat.get(k)) !== JSON.stringify(v)) {
        set.push([JSON.parse(k), v]);
      }
    }
    for (const k of baseFlat.keys()) {
      if (!flat.has(k)) {
        del.push(JSON.parse(k));
      }
    }
    rec.configDelta = { set, del };
    delete rec.config;
  }
  return rec;
});

fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ baseConfig, cases: out }), { level: 9 }));
console.error(JSON.stringify(stats));
console.error(`${path.relative(repoRoot, outFile)}: ${out.length} cases, ${fs.statSync(outFile).size} bytes`);
