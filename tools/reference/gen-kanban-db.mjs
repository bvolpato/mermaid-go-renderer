// Writes internal/diagrams/kanban/testdata/db.json.gz: what the kanban db of
// real mermaid holds after it parsed a text inside a real render (the nodes,
// the sections, the counter), and what getData() hands to the renderer, for
//   - every fixture of the corpus that mermaid renders as a kanban diagram,
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of kanbanDb.ts and of the grammar.
// A case that mermaid rejects has the error message instead.
//
// The db needs a browser (DOMPurify), so the script runs kanban-db-page.js
// with the esm-eval command of the browser oracle, one time. A corpus
// fixture runs in a new page, as in the reference renders, and so does a
// case with a site config.
//
// Usage (in tools/reference): node gen-kanban-db.mjs
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
const family = 'kanban/';
const outFile = path.join(repoRoot, 'internal/diagrams/kanban/testdata/db.json.gz');
const mutantsPerFixture = 12;

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
// The fixtures of the corpus: every fixture of the kanban family, and the
// fixtures of other families that the reference rendered as a kanban diagram.

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
      return meta.diagramType === 'kanban';
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
  'item[An item]',
  '  deeper[Deeper item]',
  '      much deeper',
  'id[Square]',
  'id(Rounded)',
  'id((Circle))',
  'id))Bang((',
  'id)Cloud(',
  'id{{Hexagon}}',
  '[No id square]',
  '((no id circle))',
  'id["Quoted **bold** text"]',
  'id["`Markdown *string*\nsecond line`"]',
  "id[With data]@{ assigned: 'knsv' }",
  'id[With data]@{ ticket: MC-1, priority: High }',
  "id[With data]@{ priority: 'Very Low', icon: book }",
  'id[With data]@{ label: Other, shape: rect }',
  'id[With data]@{ shape: kanbanItem }',
  'id@{ ticket: 5 }',
  '@{ assigned: x }',
  'id[Multi]@{\n  assigned: a\n  ticket: b\n}',
  '::icon(fa fa-book)',
  ':::urgent large',
  '%% a comment',
  '',
  '   ',
  'id[unclosed',
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
  // Skip the front matter: its text can have a line that starts with "kanban".
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    from = close < 0 ? 0 : close + 1;
  }
  let start = lines.findIndex((l, i) => i >= from && /^\s*kanban/.test(l));
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

const rnd = prng(0x6b616e62);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const kb = (body, front = '') => front + 'kanban\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const board =
  'todo[Todo]\n' +
  '  docs[Create Documentation]\n' +
  "  blog[Create Blog about the new diagram]@{ assigned: 'knsv' }\n" +
  'doing[In progress]\n' +
  "  render[Create renderer so that it works in all cases. We also add some extra text here for testing purposes.]@{ ticket: MC-2037, assigned: knsv, priority: 'High' }\n" +
  "  grammar[Design grammar]@{ priority: 'Very Low' }\n" +
  'done[Done]\n' +
  "  db[Update DB function]@{ ticket: MC-2038, priority: 'Very High' }";
// item is a board with one section and one item that has the shape data.
const item = (data) => kb('todo[Todo]\n  a[Item]@{' + data + '}');
const extras = [
  // Headers.
  'kanban\ntodo\n',
  'kanban',
  'kanban\n',
  'kanban\n\n\n',
  '\n\n   kanban   \n\n\ntodo\n\n',
  'kanban todo\n',
  'KANBAN\ntodo\n',
  'Kanban\ntodo\n  a\n',
  'kanbanX\ntodo\n',
  'kanban;\ntodo\n',
  'kanban\n%% only a comment\n',
  'kanban\n   \n  \n',
  '%% comment first\nkanban\ntodo\n',
  '   \nkanban\ntodo\n',
  'kanban\ntodo\nkanban\nother',
  'graph TD\n  A-->B\n',
  // Sections and items.
  kb(board),
  kb('todo\n  a\n  b\ndoing\n  c\ndone'),
  kb('  todo\n    a\n    b\n  doing\n    c'),
  kb('todo\n  a\n    deeper\n      deepest\n  b'),
  kb('todo\n      a\n   b\n     c\n  d'),
  kb('\ttodo\n\t\ta\n\t\tb\n\tdoing'),
  kb('todo\n  a\n\n\n  b\n   \n  c'),
  kb('todo\n  a\n%% comment\n  b\n  %% indented comment\n  c'),
  kb('todo\n  a %% trailing comment\n  b'),
  kb('todo\r\n  a\r\n  b'),
  // The same id two times: an item belongs to every section with the id.
  kb('docs[First]\n  a[Item A]\ndocs[Second]\n  b[Item B]'),
  kb('todo\n  a[One]\n  a[Two]\ndoing\n  a[Three]'),
  kb('s[Section]\n  s[Item with the id of its section]'),
  // A node with a smaller indentation than the first one.
  kb('  todo\n    a\nless'),
  kb('  todo\n    a\nless\nmore'),
  kb('  todo\n    a\nless\n  again'),
  kb('    todo\n  less[Less]\n  other'),
  kb("    todo\n  less@{ label: 5 }\n  other"),
  kb("    todo\n  less@{ label: [a, b] }\n  other"),
  kb("    todo\n  less@{ label: { toString: 1 } }\n  other"),
  // Every node type, with and without an id.
  kb('todo[Square]\n  a(Rounded)\n  b((Circle))\n  c))Bang((\n  d)Cloud(\n  e{{Hexagon}}\n  f'),
  kb('[Square]\n  (Rounded)\n  ((Circle))\n  ))Bang((\n  )Cloud(\n  {{Hexagon}}'),
  kb('todo(-Odd-)'),
  kb('todo((Mixed)'),
  kb('todo(Mixed))'),
  kb('todo[Mixed)'),
  kb('todo{{Mixed))'),
  kb('todo[a][b]'),
  kb('todo[]'),
  kb('todo[ ]\n  a[ ]'),
  kb('todo[Square] trailing'),
  // Ids that sanitizeText empties get a generated id.
  kb('[<script>alert(1)</script>]\n  [<script>x</script>]\n  [<script>y</script>]'),
  kb('<script>a</script>\n  <script>b</script>'),
  // Descriptions.
  kb('todo["Quoted text"]\n  a["Quoted (with) [brackets] {{and}} more"]'),
  kb('todo["`Markdown **bold** and *italic*`"]\n  a["`Markdown\n  on two lines`"]'),
  kb('todo["unterminated]'),
  kb('todo[""]'),
  kb('todo[text with (parens) inside]'),
  kb('todo[#35; hash #lt;b#gt; #9829;]\n  a[#quot;quoted#quot; #amp; more]'),
  kb('todo[<b>bold</b> <i>it</i> <br/> <script>alert(1)</script>]\n  a["<img src=x onerror=alert(1)>"]'),
  kb('Ünïcödé\n  日本語\n  😀[Ünïcödé 日本語 емкость 😀]'),
  kb('todo[line1<br/>line2<br>line3]\n  a[line1<br/>line2]'),
  kb('a b c\n  d e f'),
  kb('todo\n  a:b\n  c;d\n  e,f\n  g.h\n  i-j\n  k_l\n  m"n\n  o`p'),
  // Shape data.
  item(" assigned: 'knsv' "),
  item(' ticket: MC-2037 '),
  item(" priority: 'Very High' "),
  item(" priority: 'High', ticket: MC-2037, assigned: 'K.Sveidqvist' "),
  item(' icon: fa fa-book '),
  item(' label: New label '),
  item(" label: 'A **bold** <b>label</b> <script>x</script>' "),
  item(''),
  item(' '),
  item(' unknown: value '),
  item(' assigned: knsv, assigned: other '),
  item(' assigned: "double quoted" '),
  item(' assigned: "double\n    quoted on two lines" '),
  item(' assigned: "a } brace" '),
  item(' assigned: a ^ b '),
  item('\n    assigned: knsv\n    ticket: MC-1\n    priority: Low\n  '),
  item('\n    assigned: knsv\n    ticket: MC-1\n'),
  item('\n  '),
  item('\n\n'),
  item('\n    null\n  '),
  item('\n    ~\n  '),
  item('\n    a scalar\n  '),
  item('\n    5\n  '),
  item('\n    true\n  '),
  item('\n    - a\n    - b\n  '),
  item('\n    [shape, label]\n  '),
  item('\n    label: Multi\n    shape: rect\n  '),
  // The shape key.
  item(' shape: rect '),
  item(' shape: kanbanItem '),
  item(' shape: Rect '),
  item(' shape: a_b '),
  item(' shape: 5 '),
  item(' shape: 0 '),
  item(' shape: true '),
  item(' shape: false '),
  item(' shape: [rect] '),
  item(' shape: [] '),
  item(' shape: { a: 1 } '),
  item(' shape: "" '),
  item(' shape: null '),
  item(' shape: straße '),
  item(' shape: İ '),
  // Values that are not strings.
  item(' label: 5 '),
  item(' label: 0 '),
  item(' label: true '),
  item(' label: false '),
  item(' label: null '),
  item(' label: "" '),
  item(' label: [a, b] '),
  item(' label: [] '),
  item(' label: [[]] '),
  item(' label: [null, 1] '),
  item(' label: { a: 1 } '),
  item(' label: {} '),
  item(' label: { toString: 1 } '),
  item(' label: 1.50 '),
  item(' label: 1e3 '),
  item(' label: 0x10 '),
  item(' label: .inf '),
  item(' label: "<b>x</b>" '),
  item(' icon: 5 '),
  item(' icon: true '),
  item(' icon: [a, b] '),
  item(' icon: { a: 1 } '),
  item(' icon: { toString: 1 } '),
  item(' icon: 0 '),
  item(' assigned: 5 '),
  item(' assigned: [a, [b, c]] '),
  item(' assigned: { toString: x } '),
  item(' ticket: 2037 '),
  item(' ticket: 1.0 '),
  item(' ticket: true '),
  item(' ticket: { toString: x } '),
  item(' priority: 3 '),
  item(' priority: true '),
  item(' priority: [High] '),
  item(' priority: { level: High } '),
  item(' priority: Medium '),
  item(' priority: unknown '),
  item(' priority: 0 '),
  item(' priority: "" '),
  // Shape data on a section and on a node without brackets.
  kb("todo[Todo]@{ ticket: T-1, assigned: me, priority: High, icon: i, label: Section }\n  a@{ ticket: T-2 }"),
  kb('todo@{ label: Section label }\n  a@{ label: Item label }'),
  kb('@{ label: No node }'),
  kb('todo\n  @{ label: No node }'),
  kb("todo[Todo]\n  a[Item]@{ assigned: 'a' }@{ ticket: 'b' }"),
  kb("todo[Todo]\n  a[Item] @{ assigned: 'a' }"),
  // YAML errors.
  item(' a: [ '),
  item(' a: b: c '),
  item(' "unterminated '),
  item(' : '),
  item(' a: 1 }, { b: 2 '),
  item('\n    a: 1\n   b: 2\n  '),
  item('\n    a: 1\n    ---\n    b: 2\n  '),
  item(' !!js/function x '),
  item(' a: &anchor 1, b: *anchor '),
  item(' a: *missing '),
  // Unterminated shape data.
  kb('todo[Todo]\n  a[Item]@{ assigned: x'),
  kb('todo[Todo]\n  a[Item]@{ assigned: "x'),
  kb('todo[Todo]\n  a[Item]@{'),
  kb('todo[Todo]\n  a[Item]@'),
  kb('todo[Todo]\n  a[Item]@x'),
  // Icons and classes.
  kb('todo\n  a\n  ::icon(fa fa-book)'),
  kb('todo\n  ::icon(mdi mdi-skull-outline)\n  a\n    ::icon(fa fa-fire)'),
  kb('todo\n::icon(fa fa-book)'),
  kb('todo\n  a\n  :::urgent large'),
  kb('todo\n:::sectionClass\n  a\n  :::a b c'),
  kb('todo\n  a\n  ::icon(first)\n  ::icon(second)\n  :::one\n  :::two'),
  kb('::icon(fa fa-book)\ntodo'),
  kb(':::cls\ntodo'),
  kb('  ::icon(fa fa-book)\n  todo'),
  kb('  :::cls\n  todo'),
  kb('todo\n  ::icon()'),
  kb('todo\n  ::icon(unclosed'),
  kb('todo\n  a\n  ::icon(<b>x</b>)'),
  kb('todo\n  a\n  ::icon(<script>alert(1)</script>)'),
  kb('todo\n  a\n  :::<script>alert(1)</script>'),
  kb('todo\n  :::'),
  kb('todo\n  a::icon(a)'),
  kb('todo\n  a:::cls'),
  kb("todo\n  a[Item]@{ icon: from data }\n  ::icon(from decoration)"),
  // Errors of the grammar.
  kb('todo[unclosed'),
  kb('todo(unclosed'),
  kb('todo((unclosed'),
  kb('todo{{unclosed'),
  kb('todo]'),
  kb('todo}}'),
  kb('todo}'),
  kb('todo{single}'),
  kb('todo\n  ]'),
  kb('todo\n  )'),
  kb('todo\n  }}'),
  kb('todo\n  [a\n  b]'),
  // Config.
  kb(board, cfg("kanban:\n  ticketBaseUrl: 'https://example.com/browse/#TICKET#'")),
  kb(board, cfg('kanban:\n  sectionWidth: 300')),
  kb(board, cfg('kanban:\n  sectionWidth: 0')),
  kb(board, cfg('kanban:\n  padding: 20\n  useMaxWidth: false')),
  kb(board, cfg('mindmap:\n  padding: 20\n  maxNodeWidth: 100')),
  kb(board, cfg('mindmap:\n  padding: "7"\n  maxNodeWidth: wide')),
  kb(board, cfg('mindmap:\n  useMaxWidth: false')),
  kb(board, cfg('look: handDrawn')),
  kb(board, cfg('look: neo')),
  kb(board, cfg('look: classic')),
  kb(board, cfg('theme: redux')),
  kb(board, cfg('theme: redux-color')),
  kb(board, cfg('theme: forest')),
  kb(board, cfg('theme: dark')),
  kb(board, cfg('theme: base\nthemeVariables:\n  primaryColor: "#ff0000"')),
  kb(board, cfg('htmlLabels: false')),
  kb(board, cfg('flowchart:\n  htmlLabels: false')),
  kb(board, cfg('securityLevel: loose')),
  kb(board, cfg('securityLevel: antiscript')),
  kb(board, cfg('securityLevel: other')),
  item(' label: 5 ').replace(/^/, cfg('securityLevel: loose')),
  item(' label: 5 ').replace(/^/, cfg('securityLevel: other')),
  item(' label: 5 ').replace(/^/, cfg('htmlLabels: false\nsecurityLevel: other')),
  item(' label: [a, "<b>b</b>"] ').replace(/^/, cfg('securityLevel: loose')),
  kb('todo[<b>bold</b>]\n  a[<i>it</i> <script>x</script>]', cfg('securityLevel: loose')),
  kb(board, '%%{init: {"kanban": {"sectionWidth": 250}}}%%\n'),
  kb(board, '%%{init: {"look": "handDrawn", "theme": "forest"}}%%\n'),
  kb('todo\n  a', '---\ntitle: A title\n---\n'),
  // Shape data with an object as a value: a block mapping, because the
  // lexer ends the shape data at the first closing brace.
  item('\n    label:\n      a: 1\n  '),
  item('\n    label:\n      toString: 1\n  '),
  item('\n    label:\n      valueOf: 1\n  '),
  item('\n    icon:\n      a: 1\n  '),
  item('\n    icon:\n      toString: 1\n  '),
  item('\n    assigned:\n      a: 1\n  '),
  item('\n    assigned:\n      toString: x\n  '),
  item('\n    ticket:\n      a: 1\n  '),
  item('\n    ticket:\n      toString: x\n  '),
  item('\n    priority:\n      level: High\n  '),
  item('\n    shape:\n      a: 1\n  '),
  item('\n    label:\n      - a\n      - b: 1\n  '),
  kb('    todo\n  less@{\n    label:\n      a: 1\n  }\n  other'),
  kb('    todo\n  less@{\n    label:\n      toString: 1\n  }\n  other'),
  kb('todo@{\n    label:\n      a: 1\n  }\n  a'),
  item('\n    label:\n      a: 1\n  ').replace(/^/, cfg('securityLevel: loose')),
  item('\n    label:\n      toString: 1\n  ').replace(/^/, cfg('securityLevel: loose')),
  item('\n    label:\n      a: 1\n  ').replace(/^/, cfg('securityLevel: other')),
  item(' label: 5 ').replace(/^/, cfg('dompurifyConfig:\n  FORBID_TAGS: [b]')),
];
extras.forEach((text, i) => cases.push({ name: `extra-${i}`, text }));

// Site configs: the argument of mermaid.initialize. Such a case runs in a
// new page.
const sites = [
  { kanban: { ticketBaseUrl: 'https://example.com/#TICKET#', sectionWidth: 240 } },
  { mindmap: { padding: 30, maxNodeWidth: 150 } },
  { theme: 'redux' },
  { look: 'handDrawn' },
  { htmlLabels: false },
  { securityLevel: 'loose' },
  { theme: 'dark', look: 'neo' },
  { dompurifyConfig: { FORBID_TAGS: ['b'] } },
];
sites.forEach((site, i) => {
  cases.push({ name: `site-${i}`, fresh: true, site, text: kb(board) });
  cases.push({ name: `site-${i}-directive`, fresh: true, site, text: kb(board, cfg('look: neo\nkanban:\n  sectionWidth: 180')) });
});

// ---------------------------------------------------------------------------
// Run the page script.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-kanban-db-'));
const argsFile = path.join(tmp, 'args.json');
const rawFile = path.join(tmp, 'raw.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases: cases.map(({ name, text, fresh, site }) => ({ name, text, fresh, site })) }));
execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'kanban-db-page.js'), '--args', argsFile, '--out', rawFile], {
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
const raw = JSON.parse(fs.readFileSync(rawFile, 'utf8'));
if (raw.cases.length !== cases.length) {
  throw new Error(`${raw.cases.length} results for ${cases.length} cases`);
}

const stats = { total: cases.length, errors: 0, kanban: 0, otherTypes: 0 };
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
  } else if (res.type === 'kanban') {
    stats.kanban++;
  } else {
    stats.otherTypes++;
  }
  return rec;
});

fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ cases: out }), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
console.error(JSON.stringify(stats));
console.error(`${path.relative(repoRoot, outFile)}: ${out.length} cases, ${fs.statSync(outFile).size} bytes`);
