#!/usr/bin/env node
// Writes internal/diagrams/sankey/testdata/render.json and
// internal/diagrams/treemap/testdata/render.json: the SVG that
// mermaid.render of mermaid 12.2.0 returns in Chromium for hand-written
// texts, or the error. The texts cover config options and inputs that the
// corpus fixtures do not have. The Go tests render the same texts and
// compare the SVG text.
//
// All texts render in one page. The sankey renderer numbers its elements
// with a counter that the page keeps (Uid of rendering-util/uid.ts), so the
// script also stores the value of the counter before each render: the
// number of ids that the renders before it took.
//
// The script runs the eval command of the browser oracle one time.
//
// Usage (in tools/reference): node gen-sankey-treemap-render.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

const cases = [];
const add = (family, text) => cases.push({ family, name: `${family}/render-${cases.length}`, text });
const withConfig = (config, body) => `---\nconfig:\n${config}\n---\n${body}`;

const flow = 'sankey\nAgricultural waste,Bio-conversion,124.729\nBio-conversion,Liquid,0.597\nBio-conversion,Losses,26.862\nBio-conversion,Solid,280.322\nBio-conversion,Gas,81.144\nBiofuel imports,Liquid,35\nCoal imports,Coal,11.606\nCoal,Solid,75.571\nGas,Losses,1.401\nSolid,Losses,5\nLiquid,Losses,2.5\n';
for (const config of [
  '  sankey:\n    showValues: false',
  '  sankey:\n    nodeAlignment: left\n    prefix: "$"\n    suffix: " M"',
  '  sankey:\n    nodeAlignment: right\n    width: 800\n    height: 300',
  '  sankey:\n    nodeAlignment: center\n    nodeWidth: 30\n    nodePadding: 2',
  '  sankey:\n    labelStyle: outlined\n    linkColor: source',
  '  sankey:\n    labelStyle: outlined\n    linkColor: target\n    showValues: false',
  '  sankey:\n    linkColor: "#abc"\n    useMaxWidth: false',
  '  sankey:\n    linkColor: 5',
  '  sankey:\n    width: "500"\n    height: "250"\n    nodePadding: "3"',
  '  sankey:\n    nodeWidth: 0\n    nodePadding: 0',
  '  sankey:\n    nodeColors:\n      Solid: red\n      Losses: "#0f0"\n      Gas: 7',
  '  sankey:\n    nodeAlignment: bogus',
  '  sankey:\n    labelStyle: other\n    width: 100\n    height: 100',
  '  theme: dark\n  sankey:\n    labelStyle: outlined',
  '  theme: forest\n  themeVariables:\n    fontFamily: courier',
]) {
  add('sankey', withConfig(config, flow));
}
for (const text of [
  flow,
  'sankey-beta\na,b,10\n',
  'sankey\na,b,0\n',
  'sankey\na,b,abc\n',
  'sankey\na,b,0\nb,c,5\n',
  'sankey\na,b,-5\nb,c,10\n',
  'sankey\na,b,1e400\nb,c,1\n',
  'sankey\na,b,10\nc,d,20\ne,f,30\n',
  'sankey\na,b,10\na,c,10\na,d,10\nb,e,5\nc,e,5\nd,e,5\ne,f,15\n',
  'sankey\nconstructor,toString,1\n__proto__,constructor,2\nvalueOf,hasOwnProperty,3\n',
  'sankey\n"a, with comma",b,10\n"quoted ""name""",b,3\n"multi\nline",b,2\n',
  'sankey\n<b>bold</b>,<i>it</i>,10\na &amp; b,<i>it</i>,3\n',
  'sankey\na,b,1.005\nb,c,2.675\nc,d,1234567.891\n',
  'sankey\nA long node name here,Another long node name,10\nAnother long node name,x,4\n',
  '---\ntitle: The title\n---\nsankey\na,b,10\n',
  '%%{init: {"sankey": {"showValues": false, "nodeAlignment": "left"}}}%%\nsankey\na,b,10\nb,c,5\na,c,2\n',
  'sankey\na,a,10\n',
  'sankey\na,b,1\nb,c,1\nc,a,1\n',
]) {
  add('sankey', text);
}

const tree = 'treemap-beta\n"Products"\n    "Electronics"\n        "Phones": 50\n        "Computers": 30\n        "Accessories": 20\n    "Clothing"\n        "Men\'s": 40\n        "Women\'s": 40\n"Services": 25\n';
for (const config of [
  '  treemap:\n    showValues: false',
  '  treemap:\n    valueFormat: "$0,0"',
  '  treemap:\n    valueFormat: "$,.2f"',
  '  treemap:\n    valueFormat: "$.1f"',
  '  treemap:\n    valueFormat: "$"',
  '  treemap:\n    valueFormat: ".1%"',
  '  treemap:\n    valueFormat: "bogus"',
  '  treemap:\n    valueFormat: "$bogus"',
  '  treemap:\n    valueFormat: "$,bogus"',
  '  treemap:\n    valueFormat: 5',
  '  treemap:\n    valueFormat: ""',
  '  treemap:\n    padding: 0\n    diagramPadding: 0',
  '  treemap:\n    padding: 30\n    diagramPadding: 25.5',
  '  treemap:\n    nodeWidth: 50\n    nodeHeight: 30',
  '  treemap:\n    nodeWidth: 0\n    nodeHeight: 0',
  '  treemap:\n    useMaxWidth: false',
  '  theme: dark',
  '  theme: neutral\n  treemap:\n    valueFormat: ",.3s"',
  '  theme: base\n  themeVariables:\n    primaryColor: "#ff8800"',
  '  themeVariables:\n    cScale0: "#123456"\n    cScaleLabel1: "#fedcba"\n    cScalePeer2: red',
  '  fontFamily: courier',
  '  htmlLabels: false',
]) {
  add('treemap', withConfig(config, tree));
}
for (const text of [
  tree,
  'treemap\n',
  'treemap\n"Only a section"\n',
  'treemap\n"Only a leaf": 5\n',
  'treemap\n"A": 0\n"B": 0\n',
  'treemap\n"A"\n  "x": 0\n  "y": 5\n  "z": .\n',
  'treemap\ntitle A treemap title\n"A"\n  "x": 10\n  "y": 20\n',
  '---\ntitle: Front matter title\n---\ntreemap\naccTitle: The acc title\naccDescr: The description\n"A"\n  "x": 10\n',
  'treemap\n"A very long section name that does not fit into the header of its section at all"\n  "x": 1\n"B"\n  "A very long leaf name that does not fit into its cell": 2\n  "y": 30\n"C": 400\n',
  'treemap\n"S"\n  "a": 1\n  "b": 1\n  "c": 1\n  "d": 1\n  "e": 1\n  "f": 1\n  "g": 1\n  "h": 1\n  "i": 1\n  "j": 1\n  "k": 1\n  "l": 1\n  "m": 1\n  "n": 1\n  "o": 1\n  "p": 1\n  "q": 1\n  "r": 1\n  "s": 1\n  "t": 1\n  "u": 1\n  "v": 100\n',
  'treemap\n"A":::cls\n  "x": 10:::cls\n  "y": 20:::other\n  "z": 5\nclassDef cls fill:#f00,stroke:#00f,stroke-width:4px,color:#0f0,font-size:20px\nclassDef other color:red,font-weight:bold,opacity:0.5\n',
  'treemap\n"A":::ab\n  "x": 10:::ab\nclassDef ab fill:red\\,blue,color\n',
  'treemap\n"A"\n  "x": 10:::ab\nclassDef ab font-size:5px,display:none\n',
  'treemap\n"S1"\n  "S2"\n    "S3"\n      "S4"\n        "S5"\n          "deep": 3\n  "top": 1\n',
  'treemap\n"Dup"\n  "Dup": 1\n  "Dup": 2\n"Dup"\n  "x": 3\n',
  'treemap\n"<b>bold</b> &amp; x"\n  "a < b > c": 1\n  "quote \' here": 2\n',
  'treemap\n"é ü ñ"\n  "日本語": 10\n  "→ arrow": 5\n',
  'treemap\n"A"\n  "big": 12345678901234567890\n  "small": 0.000001\n  "mid": 1,234.5\n',
  '%%{init: {"treemap": {"valueFormat": "$,.0f", "showValues": true}}}%%\ntreemap\n"A"\n  "x": 1234567\n  "y": 7654321\n',
]) {
  add('treemap', text);
}

const page = `
const container = document.getElementById('container');
const out = [];
let uids = 0;
for (const c of args.cases) {
  const res = { uidsBefore: uids };
  mermaid.initialize({ startOnLoad: false });
  try {
    const { svg, diagramType } = await mermaid.render('my-svg', c.text, container);
    res.svg = svg;
    res.type = diagramType;
    // The ids that this render took from the counter of the page.
    for (const m of svg.matchAll(/ id="(?:node|linearGradient)-(\\d+)"/g)) uids = Math.max(uids, Number(m[1]));
  } catch (e) {
    res.error = String((e && e.message) || e);
    // A render that fails after the layout took no ids: the sankey layout
    // throws before the first element. The next successful render shows it.
  }
  container.innerHTML = '';
  out.push(res);
}
return out;
`;

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'diagramrender-'));
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
  const outFile = path.join(repoRoot, 'internal/diagrams', family, 'testdata/render.json');
  fs.mkdirSync(path.dirname(outFile), { recursive: true });
  fs.writeFileSync(outFile, '[\n' + golden.map((g) => JSON.stringify(g)).join(',\n') + '\n]\n');
  const failed = golden.filter((g) => g.error).length;
  console.log(`${path.relative(repoRoot, outFile)}: ${golden.length} cases, ${failed} that mermaid rejects`);
}
fs.rmSync(tmp, { recursive: true, force: true });
