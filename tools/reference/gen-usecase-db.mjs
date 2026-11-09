// Writes internal/diagrams/usecase/testdata/db.json.gz: what the db of the
// usecase diagram of real mermaid hands to the renderer, inside a real
// render (so that the front matter, the directives and the sanitizer apply):
// the layout data that the layout receives (the nodes and edges of getData
// after prepareUsecaseLayoutData, the config, the direction, the spacing,
// the fonts, the markers, the layout algorithm), the content of the db (the
// actors, the use cases, the system boundaries, the relationships, the
// notes, the JSON nodes, the class definitions, the direction, the title,
// the accessibility values), for
//   - every fixture of the usecase family of the corpus,
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of usecaseDb.ts and of
//     usecaseRenderer.ts that the corpus does not reach.
// A case that mermaid rejects has the error message instead.
//
// The db needs a browser (DOMPurify), so the script runs usecase-db-page.js
// with the esm-eval command of the browser oracle, one time. A corpus
// fixture runs in a new page, as in the reference renders.
//
// Usage (in tools/reference): node gen-usecase-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const corpusDir = 'testdata/corpus/mermaid-e2e';
const family = 'usecase/';
const outFile = path.join(repoRoot, 'internal/diagrams/usecase/testdata/db.json.gz');
const mutantsPerFixture = 4;

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
  .filter((f) => f.startsWith(family))
  .sort();

const cases = [];
for (const f of fixtures) {
  cases.push({ name: `${corpusDir}/${f}`, corpus: true, fresh: true, text: fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8') });
}

// ---------------------------------------------------------------------------
// Seeded mutations.

const snippets = [
  'actor User',
  'actor Admin("Administrator")',
  'actor Bot@{ type: hollow }',
  'actor Star@{ type: awesome }',
  'actor Bell@{ icon: "fa:bell" }',
  'actor Clerk@{ business: true } <<Employee>>',
  'actor "Quoted Actor":::hot',
  'Login("Sign in")',
  'Reset[Reset password]',
  'Pay("Pay")@{ business: true }',
  'Audit(Audit trail) <<Core>>:::hot',
  'Box[Box] <<Record>>',
  '"Free text use case"',
  '"`Markdown **use case**`"',
  'User --> Login',
  'User -- "signs in" --> Login',
  'User -- "`**md** label`" --> Login',
  'Admin --o Reset',
  'Admin --x Reset',
  'Admin o-- Reset',
  'Admin x-- Reset',
  'Admin <-- Reset',
  'Admin -- Reset',
  'Admin ----> Reset',
  'User e1@--> Pay',
  'Login ..> : include Pay',
  'Reset ..> : extend Login',
  'Admin --|> User',
  'Reset --|> Login',
  'note for Login "A note"',
  'note for User plain note text',
  'note for Login "`markdown **note**`"',
  'json Data@{ "a": 1, "b": ["x", "y"], "c": { "d": null }, "e": [], "f": {} }',
  'Login --> Data',
  'systemBoundary Sys',
  'systemBoundary "Other System"@{ type: package }:::hot',
  'end',
  'classDef hot fill:#f96,stroke:#333,stroke-width:2px',
  'classDef default stroke:#00f',
  'class Login hot',
  'class e1 hot',
  'style Login fill:#eef,stroke:#123',
  'style e1 stroke:#f00,stroke-width:3px',
  'e1@{ animate: true }',
  'e1@{ animation: slow }',
  'Login@{ business: true }',
  'User@{ type: hollow }',
  'Sys@{ type: package }',
  'direction TB',
  'direction RL',
  'direction BT',
  'direction LR',
  'accTitle: A title',
  'accDescr: A description',
  'accDescr {\n  multi\n  line\n}',
  '%% comment',
  'A(a <b>b</b> #35; c)',
  '"',
  '}',
  'package X',
];

// mutate changes the lines after the line of the diagram keyword.
function mutate(rnd, text) {
  const lines = text.split('\n');
  // Skip the front matter: its text can have a line that starts with the keyword.
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    if (close > 0) {
      from = close + 1;
    }
  }
  let head = lines.findIndex((l, i) => i >= from && /^\s*usecase-beta/.test(l));
  if (head < 0) {
    head = from;
  }
  const body = lines.slice(head + 1);
  const edits = 1 + Math.floor(rnd() * 3);
  for (let i = 0; i < edits; i++) {
    const at = Math.floor(rnd() * (body.length + 1));
    const r = rnd();
    if (r < 0.45) {
      body.splice(at, 0, '  ' + snippets[Math.floor(rnd() * snippets.length)]);
    } else if (r < 0.65 && body.length > 0) {
      body.splice(Math.min(at, body.length - 1), 1);
    } else if (r < 0.8 && body.length > 1) {
      const a = Math.floor(rnd() * body.length);
      const b = Math.floor(rnd() * body.length);
      [body[a], body[b]] = [body[b], body[a]];
    } else if (body.length > 0) {
      const j = Math.min(at, body.length - 1);
      body.splice(j, 0, body[j]);
    }
  }
  return [...lines.slice(0, head + 1), ...body].join('\n');
}

const rnd = prng(0x75636462);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `mutant${i} of ${f}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const uc = (body, prefix = '') => `${prefix}usecase-beta\n${body}\n`;
const cfg = (yaml) => `---\n${yaml}\n---\n`;
const usecaseCfg = (yaml) => cfg('config:\n  usecase:\n' + yaml.split('\n').map((l) => '    ' + l).join('\n'));

const extras = [
  uc(''),
  uc('actor A'),
  // The sanitizer and the entities.
  uc('A("<b>bold</b> & <i>it</i>")\nB["a < b > c"]\nactor C("x <script>alert(1)</script> y")'),
  uc('A(a &amp; b #35; c #lt;d#gt;)\nB("#quot;q#quot; #9829;")'),
  uc('A("<img src=x onerror=alert(1)>")\nB("<a href=\'javascript:alert(1)\'>link</a>")\nA -- "<u>edge</u> & more" --> B'),
  uc('A("`<b>md</b> **bold** _it_ & more`")\nA -- "`<i>md</i> *edge*`" --> B("`line1\nline2`")'),
  uc('actor A("<b>Actor</b>") << <i>Role</i> >>\nB(Use) << a & b >>\nC[Rect] << <b>x</b> >>\nD(Biz)@{ business: true } << y < z >>'),
  uc('actor A << <script>x</script> >>\nB(Use) << <script>x</script> >>\nC(Biz)@{ business: true } << <script>x</script> >>'),
  uc('systemBoundary "<b>Sys</b> & co"\n  A\nend\nsystemBoundary S2("`**md** title`")\n  B\nend'),
  uc('A\nnote for A "<b>note</b> & <script>x</script>"\nactor B\nnote for B "`md **note**`"\nnote for A second note'),
  uc('accTitle: <b>Title</b> & <script>x</script>\naccDescr: <i>Descr</i> #35;\nA'),
  uc('accDescr {\n   multi\n     line <b>x</b>\n}\naccTitle:    padded   \nA'),
  // Stereotypes that are folded into the label, and their escapes.
  uc('A(Plain *label* [x] _y_ `z` \\ back) <<Core *x* [y]>>\nB[Rect *label*] <<Record_1>>\nC("`**md** label`") <<Md>>'),
  uc('A <<Bare>>\nB(L) <<S>>\nB --> C <<T>>\n"Quoted uc" <<Q>>'),
  // Actors.
  uc('actor A@{ type: hollow, business: true }\nactor B@{ type: awesome }\nactor C@{ icon: "fa:user" }\nactor D@{ icon: "mdi:account" }\nactor E@{ business: true } <<Staff>>:::k\nclassDef k fill:#fee'),
  uc('actor A, B("Bee"), "C c"@{ type: hollow }\nA --> U\nB --> U\n"C c" --> U'),
  // Relationships.
  uc('A --> B\nA <-- C\nA -- D\nA --o E\nA o-- F\nA --x G\nA x-- H\nA ---> I\nA <---- J\nA ----- K'),
  uc('A -- one --> B\nA -- "two words" --- C\nA -- three --o D\nA -- four --x E\nA <-- five -- F\nA o-- six -- G\nA x-- seven -- H\nA -- "" --> I'),
  uc('A ..> : include B\nC ..> : extend A\nD --|> A\nactor X\nactor Y\nY --|> X'),
  uc('A e1@--> B\nA e2@-- "lbl" --> C\ne1@{ animate: true }\ne2@{ animation: slow }\nA e3@--> D\ne3@{ animation: fast, animate: false }\nA e4@--> E\ne4@{ animate: false }'),
  uc('classDef a stroke:#f00\nclassDef b stroke-width:4px,stroke:#0f0\nA e1@--> B\nclass e1 a,b\nstyle e1 stroke-dasharray:5 5\ne1@{ animate: true }'),
  // Classes and styles.
  uc('classDef default fill:#eee,stroke:#111\nclassDef a fill:#f00, color:#fff\nclassDef b fill:#0f0,stroke-width:2px\nA:::a\nB(L):::a,b\nactor C:::b\nclass A b\nstyle A stroke:#00f\nstyle C fill:#123,color:red'),
  uc('classDef a fill:#f00\nclassDef a stroke:#00f\nclassDef x,y font-weight:bold\nA:::a,x,y\nclass A missing'),
  uc('classDef a fill : #f00 ,  stroke :  #00f\nclassDef b :novalue,noproperty\nA:::a,b'),
  uc('systemBoundary S:::a\n  A\nend\nclassDef a fill:#ffe\nstyle S stroke:#f00\nS@{ type: package }'),
  uc('json J@{"a": 1}:::a\nclassDef a fill:#eef\nstyle J stroke:#00f,stroke-width:3px'),
  uc('json J@{"a": 1}\nstyle J stroke-width:abc'),
  // JSON tables.
  uc('json J@{ "2": "two", "1": "one", "s": "<b>bold</b> & x", "n": -0, "big": 1e999, "f": 1.5e-7, "t": true, "z": null }'),
  uc('json J@{ "arr": [1, "two", null, false], "empty": [], "obj": {}, "nested": { "a": { "b": [ { "c": 1 }, [2, 3], "x" ] } } }'),
  uc('json J@{ "dup": 1, "dup": { "x": 2 }, "a/b": { "c~d": [ { "e": "<i>f</i>" } ] }, "": "empty key" }'),
  uc('json J@{}\njson K@{ "only": [] }\nJ --> K\nK <-- J\nJ -- K'),
  uc('json J@{"a": [[], {}, [[1]]]}\nA --> J\nnote for A "about json"'),
  // System boundaries.
  uc('systemBoundary S\n  actor In\n  A(Inside)\n  B[Rect inside]\nend\nactor Out\nOut --> A\nIn --> B\nsystemBoundary T@{ type: package }\n  C\nend\nA --> C'),
  uc('systemBoundary S\nend\nsystemBoundary "Empty too"\nend'),
  uc('A(Label)\nsystemBoundary S\n  A(Label)\nend'),
  // Direction.
  uc('direction TB\nA --> B'),
  uc('direction TD\nA --> B'),
  uc('direction BT\nA --> B'),
  uc('direction RL\nA --> B'),
  uc('direction TB\ndirection LR\nA --> B'),
  // The config of the diagram.
  uc('actor A\nA --> B', usecaseCfg('nodeSpacing: 80\nrankSpacing: 120')),
  uc('actor A\nA --> B', usecaseCfg("nodeSpacing: '80'\nrankSpacing: '120'")),
  uc('actor A\nA --> B', usecaseCfg('wrappingWidth: 60\nminNodeWidth: 10')),
  uc('actor A\nA --> B', usecaseCfg("wrappingWidth: '60'\nminNodeWidth: '10'")),
  uc('actor A\nA --> B', usecaseCfg('actorFontSize: 20\nactorFontFamily: serif\nactorFontWeight: bold')),
  uc('actor A\nA --> B', usecaseCfg("usecaseFontSize: '18'\nusecaseFontFamily: 'Courier New, monospace'\nusecaseFontWeight: 600")),
  uc('actor A\nA --> B', usecaseCfg('diagramPadding: 0\nuseMaxWidth: false')),
  uc('actor A\nA --> B', usecaseCfg("diagramPadding: '12'\nuseMaxWidth: 0")),
  uc('actor A\nA --> B\nB --> C', usecaseCfg('colorScheme: rotate')),
  uc('actor A\nA --> B', usecaseCfg('nodeSpacing: null\nrankSpacing: ~\nwrappingWidth: null')),
  uc('actor A\nA --> B', usecaseCfg('unknownKey: 5\nnodeSpacing: 0')),
  uc('actor A\nA --> B', cfg('config:\n  look: classic')),
  uc('actor A\nA --> B', cfg('config:\n  look: handDrawn\n  theme: forest')),
  uc('actor A\nA --> B', cfg('config:\n  look: neo\n  theme: redux-dark-color')),
  uc('actor A\nA --> B', cfg('config:\n  theme: base\n  themeVariables:\n    primaryColor: "#ff0000"')),
  uc('actor A\nA --> B', cfg('config:\n  layout: elk')),
  uc('actor A\nA --> B', cfg('config:\n  layout: elk.stress')),
  uc('actor A\nA --> B', cfg('config:\n  layout: nonsense')),
  uc('actor A\nA --> B', cfg('config:\n  layout: dagre\n  htmlLabels: false')),
  uc('actor A\nA --> B', cfg('config:\n  flowchart:\n    htmlLabels: false\n    wrappingWidth: 77\n    nodeSpacing: 9')),
  uc('actor A\nA --> B', cfg('config:\n  nodeSpacing: 33\n  rankSpacing: 44')),
  uc('actor A\nA --> B', cfg('config:\n  securityLevel: loose\n  fontFamily: courier\n  fontSize: 20')),
  uc('actor A\nA --> B', cfg('title: The diagram title')),
  uc('actor A\nA --> B', cfg('title: "<b>Title</b>"\nconfig:\n  usecase:\n    diagramPadding: 5')),
  uc('actor A\nA --> B', '%%{init: {"usecase": {"nodeSpacing": 99, "rankSpacing": 11}, "look": "classic"}}%%\n'),
  uc('actor A\nA --> B', '%%{init: {"theme": "dark", "layout": "elk"}}%%\n'),
  uc('%% a comment line\nactor A\n%%{init: {"look": "handDrawn"}}%%\nA --> B'),
  // Texts that mermaid rejects, through the whole pipeline.
  uc('actor A\nA --> "unterminated'),
  uc('actor A\nA -->'),
  uc('actor A\nA(Use case)'),
  uc('actor A\nnote for Missing text'),
  uc('json J@{"a": tru}'),
  uc('json J@{"a": 1,}', cfg('title: shifted')),
  uc('package P'),
  uc('A --> B;'),
  uc('style A fill:#f00;'),
  uc('classDef a fill:#f00;\nA:::a'),
  'usecase-beta x\n',
  'usecase-beta',
  '  usecase-beta\n  actor A',
];
extras.forEach((text, i) => cases.push({ name: `extra${i}`, text }));

// Mutations of the hand-written inputs.
extras.forEach((text, i) => {
  if (/^(---|%%)?[\s\S]*usecase-beta\n/.test(text)) {
    cases.push({ name: `mutant of extra${i}`, text: mutate(rnd, text) });
  }
});

// Inputs that need a site config run in their own page.
const rich = uc(
  'actor A("<b>Actor</b>") << <i>Role</i> >>\nB("<u>Use</u> & <script>x</script>")\nA -- "<i>edge</i>" --> B\nnote for B "<b>n</b>"\njson J@{"k": "<b>v</b>"}\nsystemBoundary "<b>S</b>"\n  C\nend\naccTitle: <i>t</i> <script>y</script>'
);
const sited = [
  { site: { securityLevel: 'loose' }, text: rich },
  { site: { securityLevel: 'antiscript' }, text: rich },
  { site: { securityLevel: 'strict' }, text: rich },
  { site: { securityLevel: 'other' }, text: rich },
  { site: { htmlLabels: false }, text: rich },
  { site: { flowchart: { htmlLabels: false } }, text: rich },
  { site: { dompurifyConfig: { FORBID_TAGS: ['b'] } }, text: rich },
  { site: { look: 'handDrawn', theme: 'forest' }, text: uc('actor A\nA --> B') },
  { site: { look: 'neo', theme: 'redux-dark-color' }, text: uc('actor A\nA --> B\nC') },
  { site: { look: 'classic', theme: 'default' }, text: uc('actor A\nA --> B') },
  { site: { usecase: { nodeSpacing: 55, rankSpacing: 66, wrappingWidth: 99, minNodeWidth: 44 } }, text: uc('actor A\nA --> B') },
  { site: { usecase: { nodeSpacing: 55 } }, text: uc('actor A\nA --> B', usecaseCfg('nodeSpacing: 77\nrankSpacing: 88')) },
  { site: { usecase: { useMaxWidth: false, diagramPadding: 3, colorScheme: 'rotate' } }, text: uc('actor A\nA --> B') },
  { site: { usecase: { look: 'classic', theme: 'forest' } }, text: uc('actor A\nA --> B') },
  { site: { layout: 'elk' }, text: uc('actor A\nA --> B') },
  { site: { layout: 'dagre' }, text: uc('actor A\nA --> B', cfg('config:\n  layout: elk')) },
  { site: { fontFamily: 'serif', fontSize: 18 }, text: uc('actor A\nA --> B') },
  { site: { theme: 'neutral', themeVariables: { fontFamily: 'monospace' } }, text: uc('actor A\nA --> B') },
  { site: { wrap: true, markdownAutoWrap: false }, text: uc('actor A("a long long long long label")\nA --> B') },
];
sited.forEach((c, i) => cases.push({ name: `site${i}`, text: c.text, site: c.site, fresh: true }));

// ---------------------------------------------------------------------------
// Run the oracle.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'usecasedb-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases: cases.map(({ name, text, fresh, site }) => ({ name, text, fresh: !!fresh, site })) }));
execFileSync(process.execPath, ['oracle.mjs', 'esm-eval', 'usecase-db-page.js', '--args', argsFile, '--out', resultFile], {
  cwd: here,
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
const result = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
if (result.cases.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${result.cases.length}`);
}

// The config is large and almost the same for every case. The golden file
// has the config of the first case, and for each other case the leaves that
// differ: [path, value] to set and paths to delete.
const flatten = (v, prefix, out) => {
  if (v !== null && typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length > 0) {
    for (const k of Object.keys(v)) {
      flatten(v[k], [...prefix, k], out);
    }
  } else {
    out.set(JSON.stringify(prefix), v);
  }
  return out;
};
const base = result.cases.find((r) => r.config);
const baseFlat = flatten(base.config, [], new Map());
const golden = result.cases.map((r, i) => {
  const c = cases[i];
  // The text of a corpus fixture is read from the corpus by the test.
  const rec = c.corpus ? { ...r } : { ...r, text: c.text };
  if (c.fresh) {
    rec.fresh = true;
  }
  if (c.site) {
    rec.site = c.site;
  }
  if (r.config) {
    const flat = flatten(r.config, [], new Map());
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
    delete rec.config;
    rec.configDelta = { set, del };
  }
  return rec;
});
const doc = { baseConfig: base.config, cases: golden };
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(doc), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
const errors = golden.filter((r) => r.error).length;
const dumped = golden.filter((r) => r.nodes).length;
console.log(
  `${outFile}: ${golden.length} cases (${fixtures.length} corpus fixtures), ${dumped} with a db, ${errors} with an error, ` +
    `${(fs.statSync(outFile).size / 1024).toFixed(0)} KiB`
);
