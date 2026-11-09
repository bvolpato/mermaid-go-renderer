// Golden generator for the gitGraph db of internal/diagrams/git, with real
// mermaid 12.2.0 in Chromium.
//
// For every fixture of the gitGraph corpus family, for cases with markup
// that reaches DOMPurify and for diagrams that exercise the renderer, it
// stores the db as it is between the parse step and the draw step of
// mermaid.render. Each case runs in a new page (an iframe with the seeded
// Math.random and a new mermaid), because a commit id without a custom id
// comes from Math.random.
//
// The browser part is gitgraph-db-oracle.js. Run, with tools/reference as
// the working directory:
//
//   node gitgraph-db-oracle.mjs cases /tmp/gitgraph-db-args.json
//   node oracle.mjs eval gitgraph-db-oracle.js --args /tmp/gitgraph-db-args.json --out /tmp/gitgraph-db-result.json
//   node gitgraph-db-oracle.mjs finish /tmp/gitgraph-db-result.json
//
// The finish step checks each SVG of a corpus fixture against the reference
// in .parity/ref: an identical SVG shows that the page of this oracle is
// equivalent to the page of the reference render.
//
// Output: internal/diagrams/git/testdata/db_chromium.json.gz (the db of
// every case) and render_chromium.json.gz (the SVG of the cases that are not
// in the corpus)
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { ROOT, TESTDATA, corpusFiles } from './gitgraph-cases.mjs';

const RANDOM_SEED = 0x6d6d6467;
const REF = path.join(ROOT, '.parity/ref/mermaid-e2e/git');

// Cases with text that DOMPurify changes, or that the security level
// changes. `config` is the site config: the argument of mermaid.initialize.
const EXTRA = [
  { name: 'markup-in-id', text: 'gitGraph\n commit id: "<b>bold</b>" tag: "<i>t</i>" msg: "<u>m</u>"\n' },
  { name: 'br-in-id', text: 'gitGraph\n commit id: "a<br>b" tag: "x<br/>y" tag: "p<BR />q" tag: "r</br>s"\n' },
  { name: 'amp-with-br', text: 'gitGraph\n commit id: "a & b <br> c" tag: "1 < 2 <br> 3 > 2" msg: "x=y<br>"\n' },
  { name: 'amp-without-br', text: 'gitGraph\n commit id: "a & b" tag: "1 < 2 > 0" tag: "x=y" tag: "&lt;&amp;"\n' },
  { name: 'entities', text: 'gitGraph\n commit id: "#lt;b#gt;" tag: "#35;1" tag: "a#amp;b<br>c#quot;"\n' },
  { name: 'script', text: 'gitGraph\n commit id: "<script>alert(1)</script>x" tag: "<img src=x onerror=alert(1)>"\n' },
  { name: 'style-tag', text: 'gitGraph\n commit id: "<style>a{}</style>s<br>" tag: "<a href=\\"javascript:x\\">l</a><br>"\n' },
  {
    name: 'branch-markup',
    text: 'gitGraph\n commit\n branch "x<br>y"\n commit\n checkout main\n commit\n merge "x<br>y"\n checkout "x<br>y"\n commit\n branch "a<b"\n commit\n',
  },
  { name: 'branch-entity-clash', text: 'gitGraph\n commit\n branch "a<b"\n commit\n checkout main\n branch "a&lt;b"\n' },
  {
    name: 'cherry-pick-markup',
    text: 'gitGraph\n commit id: "a<br>1"\n branch dev\n commit id: "b<br>2" tag: "v<br>1"\n checkout main\n cherry-pick id: "b<br>2"\n cherry-pick id: "b<br>2" tag: "t<br>x"\n',
  },
  {
    name: 'cherry-pick-parent-markup',
    text: 'gitGraph\n commit id: "a<br>1"\n branch dev\n commit id: "b<br>2"\n checkout main\n commit id: "c"\n merge dev id: "m<br>3"\n checkout dev\n commit id: "d"\n cherry-pick id: "m<br>3" parent: "b<br>2"\n',
  },
  { name: 'merge-id-markup', text: 'gitGraph\n commit id: "a<br>"\n branch dev\n commit\n checkout main\n commit\n merge dev id: "a<br/>"\n' },
  { name: 'duplicate-id-markup', text: 'gitGraph\n commit id: "a<br>"\n commit id: "a<br/>"\n commit id: "a<br >"\n' },
  { name: 'title-markup', text: 'gitGraph\n title <b>T</b> & <i>co</i> <script>x()</script>\n accTitle: <b>acc</b> & <img src=x onerror=y>\n accDescr: <style>a{}</style> d <br> e\n commit\n' },
  { name: 'acc-descr-block-markup', text: 'gitGraph\n accDescr {\n  <b>one</b>\n   two & <br/> three\n }\n commit\n' },
  { name: 'front-matter-title-markup', text: '---\ntitle: "<b>FM</b> & <script>x</script> title"\n---\ngitGraph\n commit\n' },
  { name: 'front-matter-title-then-title', text: '---\ntitle: FM title\n---\ngitGraph\n commit\n title diagram title\n' },
  { name: 'loose-title', config: { securityLevel: 'loose' }, text: 'gitGraph\n title <b>T</b> & <a href="x" target="_blank">l</a>\n accTitle: <b>acc</b>\n commit id: "<b>i</b><br>" tag: "<a href=\\"x\\" target=\\"_blank\\">l</a><br>"\n' },
  { name: 'antiscript-title', config: { securityLevel: 'antiscript' }, text: 'gitGraph\n title <b>T</b> & <a href="x" target="_blank">l</a>\n commit id: "<b>i</b><br>"\n' },
  { name: 'gitgraph-loose', config: { gitGraph: { securityLevel: 'loose' } }, text: 'gitGraph\n commit id: "<b>i</b> & <style>s</style>" tag: "<a href=\\"x\\" target=\\"_blank\\">l</a>" tag: "a=b"\n' },
  {
    name: 'gitgraph-loose-after-title',
    config: { gitGraph: { securityLevel: 'loose' } },
    text: 'gitGraph\n title T\n commit id: "<b>i</b>" tag: "<a href=\\"x\\" target=\\"_blank\\">l</a>" tag: "<a href=\\"x\\" target=\\"_self\\">l</a>"\n',
  },
  { name: 'gitgraph-strict', config: { gitGraph: { securityLevel: 'strict' } }, text: 'gitGraph\n commit id: "<b>i</b> & x=y" tag: "<a href=\\"x\\" target=\\"_blank\\">l</a>" tag: "<script>s</script>t"\n' },
  { name: 'gitgraph-html-labels-false', config: { gitGraph: { htmlLabels: false } }, text: 'gitGraph\n commit id: "<b>i</b> & x=y" tag: "a<br>b"\n' },
  { name: 'gitgraph-flowchart-html-labels-false', config: { gitGraph: { flowchart: { htmlLabels: false } } }, text: 'gitGraph\n commit id: "<b>i</b> & x=y" tag: "a<br>b"\n' },
  { name: 'gitgraph-dompurify-config', config: { gitGraph: { dompurifyConfig: { ALLOWED_TAGS: ['i'] } } }, text: 'gitGraph\n commit id: "a<br>b<i>c</i>" tag: "x=y"\n' },
  { name: 'dompurify-config', config: { dompurifyConfig: { ALLOWED_TAGS: ['i'] } }, text: 'gitGraph\n title <b>T</b><i>i</i>\n commit id: "a<br>b<i>c</i>"\n' },
  { name: 'site-main-branch', config: { gitGraph: { mainBranchName: 'site<br>main', mainBranchOrder: 5 } }, text: 'gitGraph\n commit\n branch dev\n commit\n checkout "site<br>main"\n' },
  { name: 'site-main-branch-ok', config: { gitGraph: { mainBranchName: 'trunk', mainBranchOrder: 5 } }, text: 'gitGraph\n commit\n branch dev order: 7\n commit\n branch rel\n checkout trunk\n merge dev\n' },
  { name: 'main-branch-order-null', config: { gitGraph: { mainBranchOrder: null } }, text: 'gitGraph\n commit\n branch dev\n commit\n branch rel order: 1\n commit\n' },
  { name: 'main-branch-order-string', config: { gitGraph: { mainBranchOrder: '3' } }, text: 'gitGraph\n commit\n branch dev order: 2\n commit\n branch rel order: 4\n commit\n' },
  { name: 'error-merge-self', text: 'gitGraph\n commit id: "<br>"\n merge main\n' },
  { name: 'error-checkout-markup', text: 'gitGraph\n commit\n checkout "no<br>such"\n' },
  { name: 'error-branch-markup', text: 'gitGraph\n commit\n branch "x<br>y"\n checkout main\n branch "x<br/>y"\n' },
  { name: 'error-merge-missing-markup', text: 'gitGraph\n commit\n merge "no<b>such"\n' },
  { name: 'error-cherry-pick-missing', text: 'gitGraph\n commit id: "a"\n branch dev\n commit\n cherry-pick id: "<br>"\n' },
  { name: 'error-merge-dup-id', text: 'gitGraph\n commit id: "a<br>"\n branch dev\n commit\n checkout main\n commit\n merge dev id: "a<br>"\n' },
  { name: 'unicode', text: 'gitGraph\n commit id: "é ü 日本 😀" tag: "ünï<br>cödé"\n branch "naïve"\n commit\n' },
];

// Diagrams for the renderer: every theme and direction, and the options of
// the gitGraph config, on graphs that have each commit type, tags, merges,
// cherry-picks and branch names with two lines. The finish step stores the
// SVG of these cases.
const RICH = `
  commit id: "ZERO" tag: "v0"
  commit type: HIGHLIGHT tag: "h1" tag: "h2"
  branch develop
  commit id: "A" type: REVERSE
  checkout main
  commit id: "ONE"
  branch "feature/long name<br>two lines" order: 3
  commit
  commit id: "FIX" tag: "fix"
  checkout develop
  merge main id: "M1" tag: "merged" type: REVERSE
  checkout main
  cherry-pick id: "A"
  merge develop
  cherry-pick id: "FIX" tag: ""
  commit id: "END" type: HIGHLIGHT
  checkout develop
  merge main type: HIGHLIGHT
  commit "a message"
`;
const MANY =
  '\n  commit\n' +
  Array.from({ length: 11 }, (_, i) => `  branch b${i}\n  commit id: "c${i}" tag: "t${i}"\n  checkout main\n  commit\n`).join('') +
  '  merge b3\n  merge b7 type: REVERSE\n  checkout b10\n  cherry-pick id: "c2"\n';
const frontMatter = (config) => {
  const lines = ['---'];
  if (config.title) lines.push(`title: ${config.title}`);
  lines.push('config:');
  for (const key of ['theme', 'look']) if (config[key]) lines.push(`  ${key}: ${config[key]}`);
  if (config.gitGraph) {
    lines.push('  gitGraph:');
    for (const [key, value] of Object.entries(config.gitGraph)) lines.push(`    ${key}: ${value}`);
  }
  lines.push('---');
  return lines.join('\n') + '\n';
};
const RENDER = [];
const DIRECTIONS = ['LR', 'TB', 'BT'];
for (const theme of ['default', 'dark', 'forest', 'neutral', 'base', 'neo', 'neo-dark', 'redux', 'redux-dark', 'redux-color', 'redux-dark-color']) {
  for (const dir of DIRECTIONS) {
    const look = theme.startsWith('neo') || theme.startsWith('redux') ? 'neo' : undefined;
    RENDER.push({ name: `theme-${theme}-${dir}`, text: frontMatter({ theme, look }) + `gitGraph ${dir}:` + RICH });
  }
}
for (const theme of ['default', 'redux', 'redux-color', 'redux-dark-color', 'neo']) {
  for (const dir of DIRECTIONS) {
    RENDER.push({ name: `classic-many-${theme}-${dir}`, text: frontMatter({ theme }) + `gitGraph ${dir}:` + MANY });
  }
}
for (const [name, gitGraph] of Object.entries({
  'no-rotate': { rotateCommitLabel: false },
  'no-labels': { showCommitLabel: false },
  'no-branches': { showBranches: false },
  parallel: { parallelCommits: true },
  'parallel-no-rotate': { parallelCommits: true, rotateCommitLabel: false },
  'main-name-order': { mainBranchName: 'trunk', mainBranchOrder: 2 },
  'padding-width': { diagramPadding: 20, useMaxWidth: false, titleTopMargin: 40 },
})) {
  for (const dir of DIRECTIONS) {
    const body = name === 'main-name-order' ? RICH.replace(/\bmain\b/g, 'trunk') : RICH;
    RENDER.push({ name: `option-${name}-${dir}`, text: frontMatter({ title: 'The title', gitGraph }) + `gitGraph ${dir}:` + body });
    RENDER.push({ name: `option-${name}-redux-${dir}`, text: frontMatter({ theme: 'redux', look: 'neo', gitGraph }) + `gitGraph ${dir}:` + body });
  }
}
for (const dir of DIRECTIONS) {
  RENDER.push({ name: `empty-${dir}`, text: `gitGraph ${dir}:\n` });
  RENDER.push({ name: `one-commit-${dir}`, text: `gitGraph ${dir}:\n  commit\n  title one commit\n  accTitle: the acc title\n  accDescr: the acc descr\n` });
  RENDER.push({ name: `branches-no-commit-${dir}`, text: `gitGraph ${dir}:\n  branch a\n  branch b\n` });
  RENDER.push({ name: `duplicate-id-${dir}`, text: `gitGraph ${dir}:\n  commit id: "a"\n  commit id: "a" tag: "again"\n  branch x\n  commit id: "a"\n` });
  RENDER.push({ name: `markup-${dir}`, text: `gitGraph ${dir}:\n  commit id: "a<br>b" tag: "x & y<br>z" tag: "1 < 2"\n  branch "x<br/>y\\nz"\n  commit id: "c=d"\n` });
}

const [command, file] = process.argv.slice(2);
if (command === 'cases') {
  const cases = corpusFiles().map((f) => ({ name: 'corpus/' + f.name, text: f.text }));
  for (const c of EXTRA) cases.push({ ...c, name: 'extra/' + c.name });
  for (const c of RENDER) cases.push({ ...c, name: 'render/' + c.name });
  fs.writeFileSync(file, JSON.stringify({ seed: RANDOM_SEED, cases }));
  process.stderr.write(`${cases.length} cases -> ${file}\n`);
} else if (command === 'finish') {
  const result = JSON.parse(fs.readFileSync(file, 'utf8'));
  if (result.error) throw new Error(result.error);
  const cases = [];
  const renders = [];
  let same = 0;
  let checked = 0;
  for (const c of result.cases) {
    if (c.setupError) throw new Error(`${c.name}: ${c.setupError}`);
    if (c.randomInSetup !== 0) throw new Error(`${c.name}: the setup drew ${c.randomInSetup} random numbers`);
    if (c.name.startsWith('corpus/')) {
      const ref = path.join(REF, c.name.slice('corpus/'.length).replace(/\.mmd$/, '.svg'));
      const meta = JSON.parse(fs.readFileSync(ref.replace(/\.svg$/, '.json'), 'utf8'));
      checked++;
      if (meta.error !== undefined) {
        if (c.error === meta.error) same++;
        else process.stderr.write(`ERROR DIFFERS ${c.name}\n  oracle: ${c.error}\n  ref:    ${meta.error}\n`);
      } else if (c.svg === fs.readFileSync(ref, 'utf8') && c.randomBefore === meta.randomBefore && c.randomCalls === meta.randomCalls) {
        same++;
      } else {
        process.stderr.write(`SVG DIFFERS ${c.name}\n`);
      }
    }
    const entry = { name: c.name, text: c.text };
    if (c.config) entry.config = c.config;
    if (c.error !== undefined) {
      if (c.error.startsWith('Parsing failed')) entry.parseError = true;
      else entry.error = c.error;
    }
    if (!entry.parseError) entry.db = c.db;
    cases.push(entry);
    if (!c.name.startsWith('corpus/')) {
      // The SVG of a case that is not in the corpus, for the renderer test.
      const render = { name: c.name, text: c.text };
      if (c.config) render.config = c.config;
      if (c.error !== undefined) render.error = c.error;
      else render.svg = c.svg;
      renders.push(render);
    }
  }
  process.stderr.write(`${same} of ${checked} corpus fixtures give the SVG of the reference\n`);
  if (same !== checked) throw new Error('the oracle page is not equivalent to the reference page');
  fs.mkdirSync(TESTDATA, { recursive: true });
  const outFile = path.join(TESTDATA, 'db_chromium.json.gz');
  fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ cases }), { level: 9 }));
  const errors = cases.filter((c) => c.error || c.parseError).length;
  process.stderr.write(`${cases.length} cases (${errors} errors) -> ${outFile} (${fs.statSync(outFile).size} bytes)\n`);
  const renderFile = path.join(TESTDATA, 'render_chromium.json.gz');
  fs.writeFileSync(renderFile, zlib.gzipSync(JSON.stringify({ cases: renders }), { level: 9 }));
  process.stderr.write(`${renders.length} renders -> ${renderFile} (${fs.statSync(renderFile).size} bytes)\n`);
} else {
  process.stderr.write('usage: gitgraph-db-oracle.mjs cases <args.json> | finish <result.json>\n');
  process.exit(2);
}
