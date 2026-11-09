#!/usr/bin/env node
// Writes internal/diagrams/sankey/testdata/db.json and
// internal/diagrams/treemap/testdata/db.json: what the db of the sankey and
// of the treemap diagram holds after mermaid 12.2.0 parsed a text in
// Chromium. The Go tests parse the same texts and compare the db.
//
// The texts are the corpus fixtures and hand-written inputs. For each text
// the page calls mermaid.render, which sets the config of the front matter,
// and then mermaid.mermaidAPI.getDiagramFromText, which returns the diagram
// with its db. A number is the hex text of its bits, because a value can be
// NaN.
//
// The script runs the eval command of the browser oracle one time.
//
// Usage (in tools/reference): node gen-sankey-treemap-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const corpus = path.join(repoRoot, 'testdata/corpus/mermaid-e2e');

function* walk(dir) {
  for (const e of fs.readdirSync(dir).sort()) {
    const p = path.join(dir, e);
    if (fs.statSync(p).isDirectory()) yield* walk(p);
    else if (p.endsWith('.mmd')) yield p;
  }
}

const cases = [];
for (const family of ['sankey', 'treemap']) {
  for (const file of walk(path.join(corpus, family))) {
    cases.push({ family, name: path.relative(corpus, file), text: fs.readFileSync(file, 'utf8') });
  }
}
const extra = (family, texts) => texts.forEach((text, i) => cases.push({ family, name: `${family}/extra-${i}`, text }));

extra('sankey', [
  'sankey-beta\na,b,10\n',
  'sankey\n\na,b,10\n\n\nb,c,5.5\n',
  'sankey\r\na,b,10\r\nb,c,20\r\n',
  'sankey\n  a , b , 10 \n b,c, 2e1 \n',
  'sankey\n"a, with comma",b,10\n"quoted ""name""",b,3\n',
  'sankey\n"multi\nline",b,10\n',
  'sankey\na,b,abc\na,c,\n',
  'sankey\na,b,-5\nb,c,+7\nc,d,.5\nd,e,1e400\n',
  'sankey\na,b,10px\na,c,0x10\n',
  'sankey\na,a,10\n',
  'sankey\na,b,10\nb,a,10\n',
  'sankey\n<b>bold</b>,<script>alert(1)</script>x,10\n',
  'sankey\na<br/>b,c &amp; d,10\n',
  'sankey\nconstructor,toString,1\n__proto__,constructor,2\n',
  'sankey\n,b,10\n',
  'sankey\na,,10\n',
  'sankey\na,b\n',
  'sankey\na,b,1,2\n',
  'sankey\n',
  'sankey',
  'sankey a,b,1',
  'sankey\na,b,1\n%% comment\nb,c,2\n',
  'sankey\na;b,c:d,10\n',
  'sankey\nSankey,BETA,1\n',
  'SANKEY-BETA\nx,y,1\n',
  'sankey\n"",b,1\n',
  'sankey\n"a" ,b,1\n',
  'sankey\na,b,1\n   \nb,c,2',
  'sankey\na,b,1 \n\t\nb,c,2\n \n',
  '---\ntitle: A title\nconfig:\n  sankey:\n    showValues: false\n---\nsankey\na,b,10\n',
  'sankey\na,b,١٢\n',
  'sankey\nés,b,1\n',
  'sankey\n"é ü",b,1\n',
  'sankey\na #quot; b,c,1\n',
]);

extra('treemap', [
  'treemap\n',
  'treemap-beta\n"A"\n',
  'treemap\n"Root"\n    "A"\n        "a1": 10\n        "a2": 20:::cls\n    "B": 5\nclassDef cls fill:#f00,stroke:#000;\n',
  'treemap\n"A"\n"B"\n  "b1": 1\n"C": 3\n',
  'treemap\n  "A"\n    "a": 1\n "B"\n      "b": 2\n   "c": 3\n',
  'treemap\n"A": 1\n  "below a leaf": 2\n',
  'treemap\n"S"\n  "T"\n    "U"\n      "leaf": 4\n  "V"\n',
  'treemap\n"a": 1,000.5\n"b": .5\n"c": 1.2.3\n"d": .\n"e": 007\n',
  'treemap\ntitle My Title\naccTitle: The acc title\naccDescr: The description\n"A": 1\n',
  'treemap\naccDescr {\n  multi\n  line  text\n}\n"A": 1\ntitle Second  title\n',
  'treemap\n"A":::ab\n  "x": 1:::ab\n  "y": 2:::none\nclassDef ab fill:red,color:blue\nclassDef ab stroke:#000;\n',
  'treemap\nclassDef ab color\n"X":::ab\n',
  'treemap\nclassDef ab fill:red\\,blue,stroke-width:2px;font-size:10px\n"X": 5:::ab\n',
  'treemap\nclassDef ab\n"X"\n',
  'treemap\nclassDef ab;\n"X":::ab\n',
  'treemap\nclassDef ab fill:#f00;stroke:#00f\n"X":::ab\n',
  'treemap\nclassDef ab fill:#f00,stroke:#00f;\n"X":::ab\n',
  'treemap\n"<b>bold</b>"\n  "<script>x</script>y": 1\n',
  'treemap\n"é 😀": 1\n\'single\': 2\n',
  'treemap\n"A" "B"\n',
  'treemap\n"A"   \n"B": 1\n',
  'treemap\n"A"\n  %% a comment\n  "x": 1\n',
  'treemap\n"A": 5 %% trailing comment\n"B": 6\n',
  'treemap\n"A"\n\n\n  "x": 1\n\n',
  'treemap\n"A"\n  "x": 1\n  \n  "y": 2\n',
  'treemap\n"a": -5\n',
  'treemap\n"a": 1e5\n',
  'treemap\n"a"::: cls\n',
  'treemap\nfoo\n',
  'treemapx\n"A"\n',
  '---\ntitle: Front matter title\nconfig:\n  treemap:\n    valueFormat: "$,.2f"\n    padding: 3\n    showValues: false\n---\ntreemap\n"A": 1\n',
  '---\nconfig:\n  treemap:\n    nodeWidth: 50\n    diagramPadding: 0\n---\ntreemap\ntitle In the text\n"A": 1\n',
  '%%{init: {"treemap": {"useMaxWidth": false}}}%%\ntreemap\n"A": 1\n',
  'treemap\n"a #quot;b#quot;": 1\nclassDef ab fill:#f00;\n',
]);

const page = `
const container = document.getElementById('container');
const f64 = new DataView(new ArrayBuffer(8));
const bits = (v) => {
  if (typeof v !== 'number') return v === undefined ? undefined : { notANumber: String(v) };
  if (Number.isNaN(v)) return '7ff8000000000000';
  f64.setFloat64(0, v);
  return f64.getBigUint64(0).toString(16).padStart(16, '0');
};
const node = (n) => {
  const out = { name: n.name };
  if (n.children !== undefined) out.children = n.children.map(node);
  if (n.value !== undefined) out.value = bits(n.value);
  if (n.classSelector !== undefined) out.classSelector = n.classSelector;
  if (n.cssCompiledStyles !== undefined) out.cssCompiledStyles = n.cssCompiledStyles;
  return out;
};
const out = [];
let i = 0;
for (const c of args.cases) {
  const res = {};
  mermaid.initialize({ startOnLoad: false });
  try {
    await mermaid.render('probe' + i++, c.text, container);
  } catch (e) {
    res.renderError = String((e && e.message) || e);
  }
  try {
    const d = await mermaid.mermaidAPI.getDiagramFromText(c.text);
    res.type = d.type;
    const db = d.db;
    res.title = db.getDiagramTitle();
    res.accTitle = db.getAccTitle();
    res.accDescription = db.getAccDescription();
    if (d.type === 'sankey') {
      const graph = db.getGraph();
      res.graph = {
        nodes: graph.nodes.map((n) => ({ id: n.id })),
        links: graph.links.map((l) => ({ source: l.source, target: l.target, value: bits(l.value) })),
      };
      res.nodes = db.getNodes().map((n) => n.ID);
      res.links = db.getLinks().map((l) => [l.source.ID, l.target.ID, bits(l.value)]);
    } else if (d.type === 'treemap') {
      res.root = node(db.getRoot());
      res.nodes = db.getNodes().map((n) => [n.name, db.levels.get(n)]);
      res.outerNodes = db.outerNodes.map((n) => n.name);
      res.firstRoot = db.root === undefined ? null : db.root.name;
      res.classes = [...db.getClasses()].map(([key, def]) => [key, { id: def.id, styles: def.styles, textStyles: def.textStyles }]);
      res.config = db.getConfig();
    }
  } catch (e) {
    res.error = String((e && e.message) || e);
  }
  container.innerHTML = '';
  out.push(res);
}
return out;
`;

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'diagramdb-'));
const scriptFile = path.join(tmp, 'page.js');
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(scriptFile, page);
fs.writeFileSync(argsFile, JSON.stringify({ cases }));
execFileSync(process.execPath, ['oracle.mjs', 'eval', scriptFile, '--args', argsFile, '--out', resultFile], {
  cwd: here,
  stdio: 'inherit',
});
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
if (results.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${results.length}`);
}
for (const family of ['sankey', 'treemap']) {
  const golden = [];
  cases.forEach((c, i) => {
    if (c.family === family) golden.push({ name: c.name, text: c.text, ...results[i] });
  });
  const outFile = path.join(repoRoot, 'internal/diagrams', family, 'testdata/db.json');
  fs.mkdirSync(path.dirname(outFile), { recursive: true });
  fs.writeFileSync(outFile, '[\n' + golden.map((g) => JSON.stringify(g)).join(',\n') + '\n]\n');
  const failed = golden.filter((g) => g.error).length;
  const other = golden.filter((g) => !g.error && g.type !== family).length;
  console.log(`${path.relative(repoRoot, outFile)}: ${golden.length} cases, ${failed} that mermaid rejects, ${other} of another diagram type`);
}
fs.rmSync(tmp, { recursive: true, force: true });
