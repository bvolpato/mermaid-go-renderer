#!/usr/bin/env node
// Writes the db goldens and the render goldens of the treeView, cynefin and
// wardley diagrams from mermaid 12.2.0 in Chromium:
//
//   internal/diagrams/<treeview|cynefin|wardley>/testdata/db.json.gz
//       What the db holds after mermaid.render parsed a text, or the error
//       of the render. The texts are the corpus fixtures and the extra
//       inputs of tcw-cases.mjs.
//   internal/diagrams/<treeview|cynefin|wardley>/testdata/render.json.gz
//       The SVG that mermaid.render returns for the extra inputs, or the
//       error. The parity command covers the corpus fixtures.
//
// The db of each of these diagrams is one object of the page. The script
// takes it from mermaid.mermaidAPI.getDiagramFromText, renders each text
// with mermaid.render, which sets the config of the front matter and the
// title, and then reads the db. So the db is the one of a real render.
//
// All texts render in one page. The script checks that this gives the same
// SVG as the reference render of each fixture (.parity/ref), which uses one
// page for each diagram.
//
// The script runs the eval command of the browser oracle one time.
//
// Usage (in tools/reference):
//
//   node gen-treeview-cynefin-wardley-db.mjs [treeview] [cynefin] [wardley]
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { EXTRA } from './tcw-cases.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const corpus = path.join(repoRoot, 'testdata/corpus/mermaid-e2e');
const ref = path.join(repoRoot, '.parity/ref/mermaid-e2e');

const FAMILIES = [
  { pkg: 'treeview', family: 'tree-view', type: 'treeView', probe: 'treeView-beta\n a\n' },
  { pkg: 'cynefin', family: 'cynefin', type: 'cynefin', probe: 'cynefin-beta\n' },
  { pkg: 'wardley', family: 'wardley', type: 'wardley', probe: 'wardley-beta\n' },
];
const selected = process.argv.slice(2);
const families = FAMILIES.filter((f) => selected.length === 0 || selected.includes(f.pkg));

function* walk(dir) {
  for (const e of fs.readdirSync(dir).sort()) {
    const p = path.join(dir, e);
    if (fs.statSync(p).isDirectory()) yield* walk(p);
    else if (p.endsWith('.mmd')) yield p;
  }
}

const cases = [];
for (const f of families) {
  for (const file of walk(path.join(corpus, f.family))) {
    const name = path.relative(corpus, file);
    cases.push({ pkg: f.pkg, type: f.type, name, fixture: true, text: fs.readFileSync(file, 'utf8') });
  }
  EXTRA[f.pkg].forEach((text, i) => cases.push({ pkg: f.pkg, type: f.type, name: `${f.pkg}/extra-${i}`, text }));
}

const page = `
const container = document.getElementById('container');
// encode keeps what JSON.stringify loses: undefined is {"$u":1}, and NaN,
// the infinities and -0 are {"$n":"<text>"}.
const encode = (v) => {
  if (v === undefined) return { $u: 1 };
  if (typeof v === 'number' && (!Number.isFinite(v) || Object.is(v, -0))) {
    return { $n: Object.is(v, -0) ? '-0' : String(v) };
  }
  if (v === null || typeof v !== 'object') return v;
  if (Array.isArray(v)) return Array.from(v, encode);
  const o = {};
  for (const k of Object.keys(v)) {
    if (typeof v[k] !== 'function') o[k] = encode(v[k]);
  }
  return o;
};
// The renderer of the treeView diagram adds BBox to each node.
const treeNode = (n) => ({
  id: n.id,
  level: n.level,
  name: n.name,
  nodeType: n.nodeType,
  icon: n.icon,
  cssClass: n.cssClass,
  description: n.description,
  children: n.children.map(treeNode),
});
const dump = {
  treeView: (db) => ({ root: treeNode(db.getRoot()), count: db.getCount(), config: db.getConfig() }),
  cynefin: (db) => ({ domains: [...db.getDomains()], transitions: db.getTransitions(), config: db.getConfig() }),
  wardley: (db) => ({ data: db.getWardleyData(), config: db.getConfig() }),
};
mermaid.initialize({ startOnLoad: false });
const dbs = {};
for (const p of args.probes) {
  dbs[p.type] = (await mermaid.mermaidAPI.getDiagramFromText(p.probe)).db;
}
const out = [];
for (const c of args.cases) {
  const res = {};
  mermaid.initialize({ startOnLoad: false });
  try {
    const { svg, diagramType } = await mermaid.render('my-svg', c.text, container);
    res.type = diagramType;
    res.svg = svg;
    if (diagramType === c.type) {
      const db = dbs[c.type];
      res.db = encode({
        title: db.getDiagramTitle(),
        accTitle: db.getAccTitle(),
        accDescription: db.getAccDescription(),
        ...dump[c.type](db),
      });
    }
  } catch (e) {
    res.error = String((e && e.message) || e) || 'error';
  }
  container.innerHTML = '';
  out.push(res);
}
return out;
`;

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'tcw-db-'));
const scriptFile = path.join(tmp, 'page.js');
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(scriptFile, page);
fs.writeFileSync(argsFile, JSON.stringify({ cases, probes: families }));
execFileSync(process.execPath, ['oracle.mjs', 'eval', scriptFile, '--args', argsFile, '--out', resultFile], {
  cwd: here,
  stdio: 'inherit',
});
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });
if (results.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${results.length}`);
}

const gz = (file, data) => {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, zlib.gzipSync(Buffer.from(JSON.stringify(data)), { level: 9 }));
  return `${path.relative(repoRoot, file)} (${(fs.statSync(file).size / 1024).toFixed(1)} kB)`;
};

for (const f of families) {
  const db = [];
  const render = [];
  let sameAsRef = 0;
  cases.forEach((c, i) => {
    if (c.pkg !== f.pkg) return;
    const r = results[i];
    if (r.error === undefined && r.type !== c.type) {
      throw new Error(`${c.name}: mermaid detects the type ${r.type}`);
    }
    db.push({ name: c.name, text: c.text, ...(r.error === undefined ? { db: r.db } : { error: r.error }) });
    if (c.fixture) {
      // One page for all texts must give what one page for each text gives.
      const refFile = path.join(ref, c.name.replace(/\.mmd$/, '.svg'));
      if (fs.existsSync(refFile)) {
        if (fs.readFileSync(refFile, 'utf8') !== r.svg) {
          throw new Error(`${c.name}: the SVG differs from the reference render`);
        }
        sameAsRef++;
      }
      return;
    }
    render.push({ name: c.name, text: c.text, ...(r.error === undefined ? { svg: r.svg } : { error: r.error }) });
  });
  const dir = path.join(repoRoot, 'internal/diagrams', f.pkg, 'testdata');
  const failed = db.filter((c) => c.error !== undefined).length;
  console.log(
    `${gz(path.join(dir, 'db.json.gz'), db)}: ${db.length} cases, ${failed} that mermaid rejects, ` +
      `${sameAsRef} fixtures with the SVG of the reference`
  );
  console.log(`${gz(path.join(dir, 'render.json.gz'), render)}: ${render.length} cases`);
}
