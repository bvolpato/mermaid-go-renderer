// Writes internal/dom/testdata/type-selector-cases.json and
// type-selector-results.json: probe cases for the type selectors of
// Chromium on SVG elements, and what Chromium returns.
//
// A type selector matches an SVG element of an HTML document without regard
// to ASCII case: "foreignobject" matches <foreignObject>. mermaid depends on
// it: mermaidAPI.render selects 'foreignobject > *' to set the xmlns
// attribute of the HTML content of every foreignObject.
//
// The script runs dom-ops-page.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-dom-type-selector-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/dom/testdata');

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const cases = [];
const add = (kind, host, ops) => cases.push({ name: `${kind}-${cases.length}`, html: host.html, target: host.target, ops });

// Type selectors and the case of the names of SVG elements.
const selectorHost = {
  html: svg(
    '<defs><linearGradient id="lg"><stop offset="0"></stop></linearGradient><clipPath id="cp"><rect></rect></clipPath></defs>' +
      '<g id="p" class="node"><foreignObject id="fo" width="100" height="50"><div id="t" class="label"><span>text</span><DIV>upper</DIV></div></foreignObject>' +
      '<text id="tx"><tspan>a</tspan><textPath href="#x">b</textPath></text></g>'
  ),
  target: '#t',
};
const selectors = [
  'foreignobject', 'foreignObject', 'FOREIGNOBJECT', 'ForeignObject', 'foreignobject > *', 'foreignObject > *',
  'foreignobject div', 'g > foreignobject > div', 'svg foreignobject .label', 'lineargradient', 'linearGradient',
  'LINEARGRADIENT', 'clippath', 'clipPath', 'textpath', 'textPath', 'TEXTPATH', 'tspan', 'TSPAN', 'Tspan', 'g', 'G',
  'SVG', 'svg', 'Svg', 'div', 'DIV', 'Div', 'span', 'SPAN', 'foreignobject > DIV', 'FOREIGNOBJECT > div > SPAN',
  'text', 'TEXT', 'rect', 'RECT', 'stop', 'STOP', 'defs > LINEARGRADIENT > Stop',
  ':not(foreignobject)', ':not(FOREIGNOBJECT) > div', 'foreignobject:first-child', 'G.node', 'g.NODE', '#FO', '#fo',
];
for (const sel of selectors) {
  add('selector', selectorHost, [['query', sel], ['matches', sel], ['closest', sel]]);
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'type-selector-cases.json');
const resultsFile = path.join(outDir, 'type-selector-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(here, 'dom-ops-page.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
