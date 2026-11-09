// Writes internal/dom/testdata/style_same_value.json: what Chromium does with
// the style attribute when element.style.setProperty sets a value that the
// inline style already has (d3 selection.style does this call).
//
// Blink compares the parsed values. For an equal value it reports no change
// and the attribute keeps its text, also a text that was authored in a
// non-canonical form such as "font-size: 38px;fill:black;". The treemap
// renderer of mermaid depends on this: it sets the font size of each label
// again after its fitting loop.
//
// Each case is an element with an authored style attribute and a list of
// operations. The result is getAttribute('style') after the last operation.
//
// Usage (in tools/reference): node gen-dom-style-same-value.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/dom/testdata/style_same_value.json');

// An operation is [name, ...arguments]:
//   set prop value [priority]   el.style.setProperty(prop, value, priority)
//   remove prop                 el.style.removeProperty(prop)
const fixed = [
  // The calls of the treemap renderer.
  ['text-anchor: middle; dominant-baseline: middle; font-size: 38px;fill:black;', [['set', 'font-size', '38px']]],
  ['text-anchor: middle; dominant-baseline: middle; font-size: 38px;fill:black;', [['set', 'font-size', '37px']]],
  ['text-anchor: middle; dominant-baseline: middle; font-size: 38px;fill:black;', [['set', 'font-size', '37px'], ['set', 'font-size', '37px']]],
  ['text-anchor: middle; dominant-baseline: middle; font-size: 38px;fill:#ffffff;', [['set', 'font-size', '38px']]],
  ['text-anchor: middle; dominant-baseline: hanging; font-size: 28px;fill:black;', [['set', 'font-size', '28px'], ['remove', 'display']]],
  ['text-anchor: middle; dominant-baseline: hanging; font-size: 28px;fill:black;', [['set', 'font-size', '23px'], ['remove', 'display']]],
  ['text-anchor: middle; dominant-baseline: middle; font-size: 38px;fill:black;fill:blue !important', [['set', 'font-size', '38px']]],
  ['text-anchor: middle; dominant-baseline: middle; font-size: 38px;fill:black;font-size:20px !important', [['set', 'font-size', '20px']]],
  ['text-anchor: middle; dominant-baseline: middle; font-size: 38px;fill:black;font-size:20px !important', [['set', 'font-size', '20px', 'important']]],
  // The same value in another text.
  ['fill:red', [['set', 'fill', 'red']]],
  ['fill:red', [['set', 'fill', 'RED']]],
  ['FILL:RED', [['set', 'fill', 'red']]],
  ['fill:red', [['set', 'fill', ' red ']]],
  ['fill:red', [['set', 'fill', '#ff0000']]],
  ['fill:red', [['set', 'fill', 'rgb(255, 0, 0)']]],
  ['fill:#ff0000', [['set', 'fill', '#ff0000']]],
  ['fill:#ff0000', [['set', 'fill', '#FF0000']]],
  ['fill:#ff0000', [['set', 'fill', '#f00']]],
  ['fill:#ff0000', [['set', 'fill', 'rgb(255, 0, 0)']]],
  ['fill:#ff0000', [['set', 'fill', 'rgb(255,0,0)']]],
  ['fill:#ff0000', [['set', 'fill', 'rgba(255, 0, 0, 1)']]],
  ['fill:#ff0000', [['set', 'fill', 'red']]],
  ['fill:rgb(255,0,0)', [['set', 'fill', '#ff0000']]],
  ['fill:hsl(0, 100%, 50%)', [['set', 'fill', 'hsl(0, 100%, 50%)']]],
  ['fill:hsl(0, 100%, 50%)', [['set', 'fill', '#ff0000']]],
  ['fill:none', [['set', 'fill', 'none']]],
  ['fill:currentColor', [['set', 'fill', 'currentcolor']]],
  ['fill:url(#a)', [['set', 'fill', 'url(#a)']]],
  ['fill:url("#a")', [['set', 'fill', 'url(#a)']]],
  ['color:red', [['set', 'color', 'red']]],
  ['color:#ff0000', [['set', 'color', 'rgb(255, 0, 0)']]],
  ['stroke:#FFD600', [['set', 'stroke', '#ffd600']]],
  // Lengths and numbers.
  ['font-size:16PX', [['set', 'font-size', '16px']]],
  ['font-size:16.0px', [['set', 'font-size', '16px']]],
  ['font-size:16px', [['set', 'font-size', '16.0px']]],
  ['font-size:16px', [['set', 'font-size', '1em']]],
  ['font-size:12pt', [['set', 'font-size', '16px']]],
  ['font-size:12pt', [['set', 'font-size', '12pt']]],
  ['font-size: 38', [['set', 'font-size', '38px']]],
  ['font-size:large', [['set', 'font-size', 'large']]],
  ['font-size:calc(10px + 6px)', [['set', 'font-size', '16px']]],
  ['font-size:calc(10px + 6px)', [['set', 'font-size', 'calc(10px + 6px)']]],
  ['stroke-width: 2', [['set', 'stroke-width', '2']]],
  ['stroke-width: 2', [['set', 'stroke-width', '2px']]],
  ['stroke-width: 2px', [['set', 'stroke-width', '2']]],
  ['stroke-width:2.0px', [['set', 'stroke-width', '2px']]],
  ['width: 0', [['set', 'width', '0px']]],
  ['width: 0px', [['set', 'width', '0']]],
  ['width:10px', [['set', 'width', '10px']]],
  ['opacity:.5', [['set', 'opacity', '0.5']]],
  ['opacity:0.50', [['set', 'opacity', '.5']]],
  ['opacity:50%', [['set', 'opacity', '0.5']]],
  ['fill-opacity:0.5', [['set', 'fill-opacity', '0.5']]],
  ['stroke-dasharray:3', [['set', 'stroke-dasharray', '3']]],
  ['stroke-dasharray:3,3', [['set', 'stroke-dasharray', '3, 3']]],
  ['stroke-dasharray:3 3', [['set', 'stroke-dasharray', '3, 3']]],
  ['line-height:1.5', [['set', 'line-height', '1.5']]],
  ['font-weight:bold', [['set', 'font-weight', 'bold']]],
  ['font-weight:bold', [['set', 'font-weight', '700']]],
  ['font-weight:700', [['set', 'font-weight', '700']]],
  ['font-family:"trebuchet ms",verdana', [['set', 'font-family', '"trebuchet ms", verdana']]],
  ["font-family:'trebuchet ms',verdana", [['set', 'font-family', '"trebuchet ms", verdana']]],
  ['font-family:arial', [['set', 'font-family', 'Arial']]],
  // Keywords.
  ['display:none', [['set', 'display', 'none']]],
  ['display:NONE', [['set', 'display', 'none']]],
  ['display: none;', [['set', 'display', 'none']]],
  ['display: none;', [['set', 'display', 'inline']]],
  ['text-anchor:middle', [['set', 'text-anchor', 'middle']]],
  ['mix-blend-mode:multiply', [['set', 'mix-blend-mode', 'multiply']]],
  ['dominant-baseline:middle', [['set', 'dominant-baseline', 'middle']]],
  ['white-space:nowrap', [['set', 'white-space', 'nowrap']]],
  ['overflow:hidden', [['set', 'overflow', 'hidden']]],
  ['text-overflow:ellipsis', [['set', 'text-overflow', 'ellipsis']]],
  // The priority.
  ['fill:red !important', [['set', 'fill', 'red']]],
  ['fill:red !important', [['set', 'fill', 'red', 'important']]],
  ['fill:red', [['set', 'fill', 'red', 'important']]],
  ['fill:red!important;stroke:blue', [['set', 'stroke', 'blue']]],
  ['fill:red!important;stroke:blue', [['set', 'fill', 'red', 'important']]],
  // Other declarations in the attribute.
  ['fill:red;bogus:1', [['set', 'fill', 'red']]],
  ['fill:red;bogus:1', [['set', 'fill', 'blue']]],
  ['fill:red;fill:blue', [['set', 'fill', 'blue']]],
  ['fill:red;fill:blue', [['set', 'fill', 'red']]],
  ['fill:red;fill:bogus', [['set', 'fill', 'red']]],
  ['fill:red;;  stroke:blue', [['set', 'stroke', 'blue']]],
  ['fill:red;stroke:blue', [['set', 'fill', 'red'], ['set', 'stroke', 'blue']]],
  ['fill:red;stroke:blue', [['set', 'fill', 'red'], ['set', 'stroke', 'green']]],
  ['fill:red;stroke:blue', [['set', 'stroke', 'green'], ['set', 'fill', 'red']]],
  ['fill:red;stroke:blue', [['set', 'stroke', 'blue'], ['set', 'fill', 'red'], ['set', 'opacity', '1']]],
  ['', [['set', 'fill', 'red']]],
  ['', [['set', 'fill', 'red'], ['set', 'fill', 'red']]],
  ['', [['remove', 'fill']]],
  ['', [['set', 'fill', 'bogus value']]],
  ['bogus', [['set', 'fill', 'bogus value']]],
  ['bogus', [['remove', 'fill']]],
  // A property that the style does not have, and a value that is not valid.
  ['fill:red', [['remove', 'stroke']]],
  ['fill:red', [['remove', 'fill']]],
  ['fill:red', [['set', 'fill', 'not a color']]],
  ['fill:red', [['set', 'bogus', '1']]],
  ['fill:red', [['set', 'fill', '']]],
  ['fill:red', [['set', 'stroke', '']]],
  ['fill:red', [['set', 'font-size', 'NaNpx']]],
  // Shorthands.
  ['margin:1px', [['set', 'margin-top', '1px']]],
  ['margin:1px', [['set', 'margin', '1px']]],
  ['margin:1px', [['set', 'margin', '1px 1px']]],
  ['margin:1px 2px', [['set', 'margin', '1px 2px 1px 2px']]],
  ['overflow:hidden', [['set', 'overflow-x', 'hidden']]],
  ['font:12px arial', [['set', 'font-size', '12px']]],
  // Custom properties.
  ['--a:1', [['set', '--a', '1']]],
  ['--a: 1', [['set', '--a', '1']]],
  ['--a:1', [['set', '--a', ' 1']]],
  ['fill:var(--a)', [['set', 'fill', 'var(--a)']]],
];

const cases = [];
for (const tag of ['text', 'div']) {
  fixed.forEach(([style, ops], i) => cases.push({ name: `${tag}-${i}`, tag, style, ops }));
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
  el.setAttribute('style', c.style);
  for (const [op, a, b, priority] of c.ops) {
    if (op === 'set') el.style.setProperty(a, b, priority || '');
    else el.style.removeProperty(a);
  }
  out.push(el.getAttribute('style'));
}
container.innerHTML = '';
return out;
`;

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'samevalue-'));
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
