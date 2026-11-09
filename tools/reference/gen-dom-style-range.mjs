// Writes internal/dom/testdata/style_range.json: which numeric values
// Chromium accepts when element.style.setProperty sets a property whose
// value has a range (d3 selection.style does this call).
//
// CSS allows no negative value for width, max-width, padding and other
// properties. Blink rejects such a value in the parser: the call changes
// nothing. createText of mermaid depends on this: for a node whose width is
// negative it sets `max-width: -10px` on the label, and the label keeps its
// natural width.
//
// Each case is an element with no style attribute and one call of
// setProperty. The result is getAttribute('style') after the call: null
// when Chromium rejects the value.
//
// Usage (in tools/reference): node gen-dom-style-range.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/dom/testdata/style_range.json');

const properties = [
  'width', 'height', 'min-width', 'min-height', 'max-width', 'max-height',
  'top', 'right', 'bottom', 'left',
  'margin-top', 'margin-right', 'margin-bottom', 'margin-left',
  'padding-top', 'padding-right', 'padding-bottom', 'padding-left',
  'text-indent', 'letter-spacing', 'word-spacing', 'row-gap', 'column-gap', 'outline-offset',
  'stroke-width', 'stroke-dashoffset', 'stroke-miterlimit',
  'x', 'y', 'cx', 'cy', 'r', 'rx', 'ry',
  'opacity', 'fill-opacity', 'stroke-opacity', 'stop-opacity', 'flood-opacity',
  'z-index', 'order', 'flex-grow', 'flex-shrink', 'flex-basis',
  'line-height', 'tab-size', 'font-size', 'font-weight',
];

const values = [
  '-10px', '-10', '-0.5em', '-5%', '-.5px', '-1e-3px', '-0px', '-0', '0', '0px', '10px', '10', '5%', '1.5', '+5px',
];

const cases = [];
for (const tag of ['div', 'rect']) {
  for (const property of properties) {
    for (const value of values) {
      cases.push({ name: `${tag} ${property}: ${value}`, tag, style: '', ops: [['set', property, value]] });
    }
  }
}

const page = `
const container = document.getElementById('container');
const out = [];
for (const c of args.cases) {
  container.innerHTML = c.tag === 'div' ? '<section></section>' : '<svg xmlns="http://www.w3.org/2000/svg"><g></g></svg>';
  const parent = container.querySelector(c.tag === 'div' ? 'section' : 'g');
  const el = c.tag === 'div'
    ? document.createElement('div')
    : document.createElementNS('http://www.w3.org/2000/svg', c.tag);
  parent.appendChild(el);
  for (const [, a, b] of c.ops) {
    el.style.setProperty(a, b);
  }
  out.push(el.getAttribute('style'));
}
container.innerHTML = '';
return out;
`;

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'stylerange-'));
const scriptFile = path.join(tmp, 'page.js');
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(scriptFile, page);
fs.writeFileSync(argsFile, JSON.stringify({ cases }));
execFileSync(
  process.execPath,
  ['oracle.mjs', 'eval', scriptFile, '--bare', '--args', argsFile, '--out', resultFile],
  { cwd: here, stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
if (results.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${results.length}`);
}
const golden = cases.map((c, i) => ({ ...c, want: results[i] }));
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, '[\n' + golden.map((g) => JSON.stringify(g)).join(',\n') + '\n]\n');
fs.rmSync(tmp, { recursive: true, force: true });
console.log(`${outFile}: ${golden.length} cases`);
