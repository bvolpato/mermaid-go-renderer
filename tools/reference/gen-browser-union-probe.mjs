// Writes internal/browser/testdata/union-cases.json and union-results.json:
// DOM probe cases for the bounding box of a container (getBBox of a g or of
// the svg) and what Chromium measures for them.
//
// The bounding box of a container is the union of the boxes of its
// children, in float32 (gfx::RectF::UnionEvenIfEmpty). The width is
// right - left, rounded to float32, so left + width can be less than the
// right edge. Chromium then widens the box to the next float32. The cases
// have children that are rotated by a multiple of 45 degrees, as the labels
// of a gitGraph are, so that the boxes have many digits and a part of the
// cases needs that step.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-union-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

let a = 0x756e696f >>> 0;
const rnd = () => {
  a = (a + 0x6d2b79f5) >>> 0;
  let t = a;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const pick = (list) => list[Math.floor(rnd() * list.length)];
// num gives a multiple of 1/64, as the text metrics of Chromium are. Such a
// number is exact in float32, so the cases do not depend on how the
// browser reads a long decimal number.
const num = (lo, hi) => {
  const v = lo + rnd() * (hi - lo);
  return Math.round(v * pick([1, 2, 4, 64])) / pick([1, 1, 64]);
};

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const shape = () => {
  switch (pick(['rect', 'rect', 'rect', 'circle', 'line'])) {
    case 'rect':
      return `<rect x="${num(-300, 300)}" y="${num(-300, 300)}" width="${num(0.01, 400)}" height="${num(0.01, 200)}"/>`;
    case 'circle':
      return `<circle cx="${num(-300, 300)}" cy="${num(-300, 300)}" r="${num(0.5, 40)}"/>`;
    default:
      return `<line x1="${num(-300, 300)}" y1="${num(-300, 300)}" x2="${num(-300, 300)}" y2="${num(-300, 300)}"/>`;
  }
};
const transform = () => {
  switch (pick(['none', 'none', 'translate', 'rotate', 'rotate3', 'both'])) {
    case 'translate':
      return ` transform="translate(${num(-200, 200)}, ${num(-200, 200)})"`;
    case 'rotate':
      return ` transform="rotate(${pick([45, -45, 45, -45, 90, 135])})"`;
    case 'rotate3':
      return ` transform="rotate(${pick([45, -45])}, ${num(-200, 200)}, ${num(-200, 200)})"`;
    case 'both':
      return ` transform="translate(${num(-50, 50)}, ${num(-50, 50)}) rotate(-45, ${num(0, 300)}, ${num(0, 100)})"`;
    default:
      return '';
  }
};
const withTransform = (s) => s.replace(/\/>$/, `${transform()}/>`);

const queries = [
  { sel: '#t', op: 'bbox' },
  { sel: '#my-svg', op: 'bbox' },
  { sel: '#t > *', op: 'bbox', all: true },
];
const cases = [];
const add = (kind, inner) => cases.push({ name: `union-${kind}-${cases.length}`, html: svg(inner), queries });

// Plain shapes in one container.
for (let i = 0; i < 60; i++) {
  const n = 2 + Math.floor(rnd() * 4);
  let inner = '';
  for (let k = 0; k < n; k++) inner += shape();
  add('shapes', `<g id="t">${inner}</g>`);
}
// Shapes with a transform of their own.
for (let i = 0; i < 120; i++) {
  const n = 2 + Math.floor(rnd() * 4);
  let inner = '';
  for (let k = 0; k < n; k++) inner += withTransform(shape());
  add('transformed', `<g id="t">${inner}</g>`);
}
// Nested containers, as the commit labels of a gitGraph are.
for (let i = 0; i < 100; i++) {
  const n = 2 + Math.floor(rnd() * 3);
  let inner = '';
  for (let k = 0; k < n; k++) {
    inner += `<g${transform()}>${shape()}${withTransform(shape())}</g>`;
  }
  add('nested', `<g id="t"${transform()}>${inner}${shape()}</g><g>${shape()}</g>`);
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'union-cases.json');
const resultsFile = path.join(outDir, 'union-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
