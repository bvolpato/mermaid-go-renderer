// Writes internal/diagrams/eventmodeling/testdata/db.json.gz: what the
// eventmodeling db of real mermaid holds after it parsed a text inside a
// real render (getDiagramProps() and getState(): the swimlanes, the boxes
// and the relations), for
//   - every fixture of the corpus that mermaid renders as an event model,
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of db.ts.
// A case that mermaid rejects has the error message instead.
//
// The db measures text, so it needs a browser: the script runs
// eventmodeling-db-page.js with the esm-eval command of the browser oracle,
// one time. A corpus fixture runs in a new page, as in the reference
// renders, and so does a case with a site config.
//
// Usage (in tools/reference): node gen-eventmodeling-db.mjs
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
const family = 'eventmodeling/';
const outFile = path.join(repoRoot, 'internal/diagrams/eventmodeling/testdata/db.json.gz');
const mutantsPerFixture = 15;

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
// The fixtures of the corpus.

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
      return meta.diagramType === 'eventmodeling';
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
// Seeded mutations: line edits after the line of the diagram keyword.

const snippets = [
  'tf 06 ui Screen',
  'tf 07 cmd DoIt { id: 1 }',
  'tf 08 evt Done ->> 02',
  'tf 09 rmo View ->> 03 ->> 01',
  'rf 10 pcr Worker',
  'tf 11 evt ns.Named',
  'tf 12 command ns.Other [[ItemAddedData]]',
  'tf 02 cmd Duplicate',
  'tf 13 readmodel A.B.C "text"',
  "tf 14 processor P 'text'",
  'tf 15 event E `json` { "a": 1 }',
  'timeframe 16 ui T ->> 99',
  'resetframe 17 evt R ->> 01',
  'data ItemAddedData\n{\n  other: 2\n}',
  'data Unused {\n  x\n}',
  'entity ns.Thing',
  'note 02 {\n  a note\n}',
  'gwt 03 given evt A then evt B',
  '%% a comment',
  '// a comment',
  '/* a comment */',
  '',
  ' title A title',
  'tf 18 ui LongNameThatGoesOnAndOnAndOnForAVeryLongTimeUntilItIsWiderThanTheBoxCanBe',
  'tf 19 cmd C { a very long inline text that goes on and on and on for a very long time until it is wider than the box }',
];

function mutate(rnd, text) {
  const lines = text.split('\n');
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    from = close < 0 ? 0 : close + 1;
  }
  let start = lines.findIndex((l, i) => i >= from && /^\s*eventmodeling/.test(l));
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
    } else if (kind === 3 || kind === 4 || kind === 5) {
      lines.splice(p, 0, snippets[Math.floor(rnd() * snippets.length)]);
    } else {
      const l = lines[p] ?? '';
      const at = Math.floor(rnd() * (l.length + 1));
      lines[p] = l.slice(0, at) + l.slice(at + 1 + Math.floor(rnd() * 4));
    }
  }
  return lines.join('\n');
}

const rnd = prng(0x65766d62);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const em = (body, front = '') => front + 'eventmodeling\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const model =
  'tf 01 ui CartScreen\n' +
  'tf 02 cmd AddItem { productId: 7 }\n' +
  'tf 03 evt ItemAdded [[ItemAddedData]]\n' +
  'tf 04 rmo CartItems ->> 03\n' +
  'tf 05 pcr Notifier ->> 04\n' +
  'tf 06 cmd Inventory.Reserve ->> 05\n' +
  'tf 07 evt Inventory.Reserved\n' +
  'rf 08 ui Summary\n\n' +
  'data ItemAddedData\n{\n  productId: 7,\n  quantity: 1\n}\n';
const long = 'A'.repeat(20) + 'B'.repeat(20) + 'C'.repeat(20) + 'D'.repeat(20);
const many = (n, line) => Array.from({ length: n }, (_, i) => line(String(i + 1).padStart(2, '0'), i)).join('\n');
const extras = [
  // Headers and empty models.
  'eventmodeling',
  'eventmodeling\n',
  'eventmodeling\n\n\n',
  '\n\n  eventmodeling  \n\ntf 01 ui A\n',
  'eventmodeling tf 01 ui A',
  'Eventmodeling\ntf 01 ui A\n',
  'eventmodelingX\ntf 01 ui A\n',
  'graph TD\n  A-->B\n',
  em('entity A\nnote 01 {\n  n\n}\ngwt 01 given ui A then evt A'),
  em('data D\n{\n  a: 1\n}'),
  // Every entity type, alone and together.
  em('tf 01 ui A'),
  em('tf 01 pcr A'),
  em('tf 01 processor A'),
  em('tf 01 rmo A'),
  em('tf 01 readmodel A'),
  em('tf 01 cmd A'),
  em('tf 01 command A'),
  em('tf 01 evt A'),
  em('tf 01 event A'),
  em('tf 01 ui A\ntf 02 pcr B\ntf 03 processor C\ntf 04 rmo D\ntf 05 readmodel E\ntf 06 cmd F\ntf 07 command G\ntf 08 evt H\ntf 09 event I'),
  em('tf 09 event I\ntf 08 evt H\ntf 07 command G\ntf 06 cmd F\ntf 05 readmodel E\ntf 04 rmo D\ntf 03 processor C\ntf 02 pcr B\ntf 01 ui A'),
  // The x of a box: the same swimlane again, another swimlane, back.
  em('tf 01 ui A\ntf 02 ui B\ntf 03 ui C'),
  em('tf 01 evt A\ntf 02 evt B\ntf 03 ui C\ntf 04 ui D\ntf 05 evt E'),
  em('tf 01 cmd A\ntf 02 evt B\ntf 03 cmd C\ntf 04 evt D\ntf 05 cmd E\ntf 06 evt F'),
  em('tf 01 ui ' + long + '\ntf 02 cmd B\ntf 03 ui C\ntf 04 evt ' + long + long),
  // Namespaces: a new swimlane each time, also for the same namespace.
  em('tf 01 ui Shop.Screen\ntf 02 cmd Shop.Buy\ntf 03 evt Shop.Bought'),
  em('tf 01 ui Shop.A\ntf 02 ui Shop.B\ntf 03 ui Other.C\ntf 04 ui D'),
  em('tf 01 cmd Shop.A\ntf 02 cmd Shop.B\ntf 03 rmo Other.C\ntf 04 readmodel D\ntf 05 command Shop.E'),
  em('tf 01 evt Shop.A\ntf 02 evt Shop.B\ntf 03 event Other.C\ntf 04 evt D'),
  em('tf 01 ui A.B.C\ntf 02 cmd A.B.C.D\ntf 03 evt X.Y'),
  em('tf 01 evt D\ntf 02 evt ns.A\ntf 03 cmd E\ntf 04 cmd ns.B\ntf 05 ui F\ntf 06 ui ns.C\ntf 07 pcr ns.C'),
  em(many(12, (id, i) => `tf ${id} ${['ui', 'cmd', 'evt'][i % 3]} N${i % 4}.Name${i}`)),
  // More swimlanes of one kind than the range of indexes has.
  em(many(101, (id) => `tf ${id.padStart(3, '0')} ui ns.A`) + '\ntf 500 cmd B\ntf 501 ui ns.C'),
  // Relations: the frame before, in another swimlane.
  em('tf 01 ui A\ntf 02 cmd B\ntf 03 evt C\ntf 04 rmo D\ntf 05 ui E'),
  em('tf 01 evt A\ntf 02 evt B\ntf 03 evt C'),
  em('tf 01 ui A\ntf 02 ui B\ntf 03 cmd C\ntf 04 cmd D\ntf 05 ui E'),
  em('tf 01 evt A\ntf 02 cmd B\ntf 03 ui C'),
  // Source frames.
  em('tf 01 ui A\ntf 02 cmd B\ntf 03 evt C ->> 01'),
  em('tf 01 ui A\ntf 02 cmd B\ntf 03 evt C ->> 02 ->> 01'),
  em('tf 01 ui A\ntf 02 cmd B\ntf 03 evt C ->> 01 ->> 01 ->> 01'),
  em('tf 01 ui A\ntf 02 cmd B ->> 03\ntf 03 evt C'),
  em('tf 01 ui A\ntf 02 cmd B ->> 02'),
  em('tf 01 ui A\ntf 02 cmd B ->> 99'),
  em('tf 01 ui A\ntf 02 cmd B ->> 99 ->> 01 ->> 03\ntf 03 evt C ->> 02'),
  em('tf 01 ui A ->> 01'),
  em('tf 01 ui A ->> 02\ntf 02 cmd B'),
  em('tf 01 ui A\ntf 02 cmd B ->> 1'),
  em('tf 1 ui A\ntf 01 cmd B ->> 1\ntf 001 evt C ->> 01 ->> 1'),
  em('tf 01 ui A\ntf 02 ui B ->> 01\ntf 03 ui C ->> 01 ->> 02'),
  em('tf 01 evt A\ntf 02 ui B ->> 01'),
  // Reset frames.
  em('rf 01 ui A\nrf 02 cmd B\nrf 03 evt C'),
  em('tf 01 ui A\nrf 02 cmd B\ntf 03 evt C'),
  em('tf 01 ui A\nresetframe 02 cmd B ->> 01\ntf 03 evt C'),
  em('rf 01 ui A ->> 01\ntimeframe 02 cmd B'),
  // Inline data.
  em('tf 01 cmd A { a: 1 }'),
  em('tf 01 cmd A {a: 1}'),
  em('tf 01 cmd A {}'),
  em('tf 01 cmd A { }'),
  em('tf 01 cmd A {  }'),
  em('tf 01 cmd A { a: { b: 1 } }'),
  em('tf 01 cmd A { a: 1 } tf 02 evt B { b: 2 }'),
  em('tf 01 cmd A "a string"'),
  em("tf 01 cmd A 'a string'"),
  em('tf 01 cmd A ""'),
  em('tf 01 cmd A "with { braces } inside"'),
  em('tf 01 cmd A "with } only"'),
  em('tf 01 cmd A "with { only"'),
  em("tf 01 cmd A '}{'"),
  em('tf 01 cmd A `json` { "a": 1, "b": [1, 2] }'),
  em('tf 01 cmd A `html` "<b>x</b>"'),
  em('tf 01 cmd A { a: 1,   b:    2,  c: "x  y" }'),
  em('tf 01 cmd A { ' + 'word '.repeat(40) + '}'),
  em('tf 01 cmd A { ' + long + long + long + ' }'),
  em('tf 01 cmd A { é 😀 日本語 }'),
  em('tf 01 cmd A { <b>bold</b> <i>it</i> }'),
  em('tf 01 cmd A { <script>alert(1)</script> }'),
  em('tf 01 cmd A { <img src=x onerror=alert(1)> }'),
  em('tf 01 cmd A { a &amp; b &lt; c }'),
  em('tf 01 cmd A { #35; #lt;b#gt; }'),
  em('tf 01 cmd A { a<br/>b<br>c }'),
  em('tf 01 cmd A { a: 1 } %% a comment'),
  em('tf 01 cmd A { a: 1 // not a comment }'),
  // Data references.
  em('tf 01 evt A [[D]]\ndata D\n{\n  a: 1\n}'),
  em('data D\n{\n  a: 1\n}\ntf 01 evt A [[D]]'),
  em('tf 01 evt A [[Missing]]'),
  em('tf 01 evt A [[Missing]] { inline: 1 }'),
  em('tf 01 evt A [[D]] { inline: 1 }\ndata D\n{\n  block: 2\n}'),
  em('tf 01 evt A [[D]]\ntf 02 evt B [[D]]\ntf 03 evt C [[E]]\ndata D\n{\n  a: 1\n}\ndata E\n{\n  b: 2\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{\n  first: 1\n}\ndata D\n{\n  second: 2\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{\n\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{ \n  a: 1\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{\t\n  a: 1\n}'),
  em('tf 01 evt A [[D]]\ndata D {\n  a: 1\n}'),
  em('tf 01 evt A [[D]]\ndata D `json` {\n  "a": 1,\n  "b": {\n    "c": [1, 2, 3]\n  }\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{\n  a: 1\n  }'),
  em('tf 01 evt A [[D]]\ndata D\n{\n  a: 1 }\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{\n  ' + 'word '.repeat(60) + '\n  second line\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{\n  <b>bold</b> <script>x</script>\n  a &amp; b\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{\n  é 😀 日本語\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{\n  a<br/>b\n}'),
  em('tf 01 evt A [[D]]\ndata D\n{\r\n  a: 1\r\n}\r\n'),
  // Long names wrap.
  em('tf 01 ui ' + long),
  em('tf 01 ui ' + long + long + long),
  em('tf 01 ui ns.' + long + long),
  em('tf 01 ui A_very_long_name_with_under_scores_that_goes_on_and_on_and_on_and_on_and_on_and_on'),
  // Duplicate frame ids. The line counts the comments.
  em('tf 01 ui A\ntf 01 cmd B'),
  em('tf 01 ui A\ntf 02 cmd B\n\n\ntf 01 evt C'),
  em('tf 01 ui A\n%% a comment\n%% another comment\ntf 01 cmd B'),
  em('tf 01 ui A %% trailing\ntf 01 cmd B'),
  em('tf 01 ui A\n/* a\nlong\ncomment */ tf 01 cmd B'),
  em('tf 01 ui A tf 01 cmd B'),
  em('tf 01 ui A\ntf 1 cmd B\ntf 001 evt C'),
  em('tf 01 ui A\nrf 01 cmd B'),
  em('tf 01 ui A\ntf 01 cmd B', '---\ntitle: With front matter\n---\n'),
  em('tf 01 ui A\ntf 01 cmd B', '%%{init: {"theme": "dark"}}%%\n'),
  em('tf 01 ui A\ntf 01 cmd B', '%% a comment in front\n'),
  // Comments.
  em('tf 01 ui A %% a comment\ntf 02 cmd B'),
  em('%% a comment\ntf 01 ui A\n  %% another\ntf 02 cmd B %% trailing'),
  em('tf 01 ui A // a comment\ntf 02 cmd B'),
  em('tf 01 ui A /* a comment */ tf 02 cmd B'),
  em('tf 01 ui A %%{init: {"theme": "dark"}}%%\ntf 02 cmd B'),
  em('tf 01 cmd A { a: 1 } %% trailing\ntf 02 evt B'),
  em('tf 01 evt A [[D]]\ndata D\n{\n  %% not a comment\n  a: 1\n}'),
  // Title and accessibility.
  'eventmodeling title A title\ntf 01 ui A\n',
  'eventmodeling accTitle: An accessible title\ntf 01 ui A\n',
  'eventmodeling accDescr: A description\ntf 01 ui A\n',
  'eventmodeling accDescr {\n a\n multi line\n}\ntf 01 ui A\n',
  em('tf 01 ui A title One\ntf 02 cmd B accTitle: Two\ntf 03 evt C accDescr: Three'),
  em('tf 01 ui A title <b>One</b> <script>x</script>'),
  em('tf 01 ui A title One\ntf 02 cmd B title Two'),
  em('tf 01 ui A', '---\ntitle: Front matter title\n---\n'),
  em('tf 01 ui A title In the text', '---\ntitle: Front matter title\n---\n'),
  // Config.
  em(model),
  em(model, cfg('theme: dark')),
  em(model, cfg('theme: forest')),
  em(model, cfg('theme: neutral')),
  em(model, cfg('theme: base')),
  em(model, cfg('theme: redux')),
  em(model, cfg('theme: redux-dark')),
  em(model, cfg('theme: neo')),
  em(model, cfg('theme: base\nthemeVariables:\n  emUiFill: "#111111"\n  emUiStroke: "#222222"\n  emProcessorFill: red\n  emReadModelStroke: blue\n  emCommandFill: "rgb(1,2,3)"\n  emEventStroke: 5')),
  em(model, cfg('themeVariables:\n  lineColor: "#ff0000"\n  primaryColor: "#00ff00"')),
  em(model, cfg('fontFamily: courier\nfontSize: 20')),
  em(model, cfg('securityLevel: loose')),
  em(model, cfg('htmlLabels: false')),
  em(model, cfg('eventmodeling:\n  padding: 5\n  useMaxWidth: false')),
  em(model, '%%{init: {"theme": "dark"}}%%\n'),
  em('tf 01 cmd A { <b>bold</b> }', cfg('securityLevel: loose')),
];
extras.forEach((text, i) => cases.push({ name: `extra-${i}`, text }));

// Site configs: the argument of mermaid.initialize. Such a case runs in a
// new page.
const sites = [
  { theme: 'dark' },
  { theme: 'base', themeVariables: { emUiFill: '#123456', emArrowhead: '#654321', emRelationStroke: 'green' } },
  { securityLevel: 'loose' },
  { securityLevel: 'antiscript' },
  { htmlLabels: false },
  { dompurifyConfig: { FORBID_TAGS: ['b'] } },
  { eventmodeling: { padding: 10 } },
];
sites.forEach((site, i) => {
  cases.push({ name: `site-${i}`, fresh: true, site, text: em(model) });
  cases.push({ name: `site-${i}-html`, fresh: true, site, text: em('tf 01 cmd A { <b>bold</b> <i>it</i> }\ntf 02 evt B [[D]]\ndata D\n{\n  <u>u</u>\n}') });
});

// ---------------------------------------------------------------------------
// Run the page script.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-eventmodeling-db-'));
const argsFile = path.join(tmp, 'args.json');
const rawFile = path.join(tmp, 'raw.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases: cases.map(({ name, text, fresh, site }) => ({ name, text, fresh, site })) }));
execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'eventmodeling-db-page.js'), '--args', argsFile, '--out', rawFile], {
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
const raw = JSON.parse(fs.readFileSync(rawFile, 'utf8'));
if (raw.cases.length !== cases.length) {
  throw new Error(`${raw.cases.length} results for ${cases.length} cases`);
}

// getDiagramProps() is the same object in every case. The file has it one
// time.
const props = raw.cases.find((c) => c.props).props;
const stats = { total: cases.length, errors: 0, eventmodeling: 0, otherTypes: 0 };
const out = raw.cases.map((res, i) => {
  const c = cases[i];
  if (res.name !== c.name) {
    throw new Error(`case ${i}: ${res.name} is not ${c.name}`);
  }
  const rec = { ...res };
  if (rec.props) {
    if (JSON.stringify(rec.props) !== JSON.stringify(props)) {
      throw new Error(`case ${i}: other diagram props`);
    }
    delete rec.props;
  }
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
  } else if (res.type === 'eventmodeling') {
    stats.eventmodeling++;
  } else {
    stats.otherTypes++;
  }
  return rec;
});

fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ props, cases: out }), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
console.error(JSON.stringify(stats));
console.error(`${path.relative(repoRoot, outFile)}: ${out.length} cases, ${fs.statSync(outFile).size} bytes`);
