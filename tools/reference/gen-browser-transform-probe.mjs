// Writes internal/browser/testdata/transform-cases.json and
// transform-results.json: values of the SVG transform attribute, and the
// matrix and the mapped bounding box that Chromium computes for them.
//
// Chromium turns the transform attribute into a CSS transform. The matrix
// then comes from gfx::Transform: translations and scales add up in float32
// while the transform is axis-aligned, a rotation by a multiple of 45
// degrees uses exact sine and cosine values, and the rest is in double.
// The matrix of the SVG DOM (transform.baseVal) is a different one.
//
// The script runs browser-transform-probe.js with the eval command of the
// browser oracle, one time.
//
// Usage (in tools/reference): node gen-browser-transform-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

let a = 0x74726e73 >>> 0;
const rnd = () => {
  a = (a + 0x6d2b79f5) >>> 0;
  let t = a;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const pick = (list) => list[Math.floor(rnd() * list.length)];
// num gives a number as mermaid prints them: an integer, a multiple of
// 1/64, or a decimal number with a few digits.
const num = (lo, hi) => {
  const v = lo + rnd() * (hi - lo);
  switch (pick(['int', 'dyadic', 'dyadic', 'decimal', 'decimal', 'long'])) {
    case 'int':
      return Math.round(v);
    case 'dyadic':
      return Math.round(v * 64) / 64;
    case 'decimal':
      return Number(v.toFixed(pick([1, 2, 3])));
    default:
      return v;
  }
};
const ANGLES = [0, 45, -45, 90, -90, 135, 180, 225, 270, 315, 360, 405, -135, -180, -360, 30, -30, 60, 12.5, -73.2, 1, 89.99, 450, 720];
const angle = () => (rnd() < 0.8 ? pick(ANGLES) : num(-360, 360));
const op = () => {
  switch (pick(['translate', 'translate', 'translate', 'translate1', 'rotate', 'rotate', 'rotate3', 'rotate3', 'scale', 'scale1', 'skewX', 'skewY', 'matrix'])) {
    case 'translate':
      return `translate(${num(-300, 300)}, ${num(-300, 300)})`;
    case 'translate1':
      return `translate(${num(-300, 300)})`;
    case 'rotate':
      return `rotate(${angle()})`;
    case 'rotate3':
      return `rotate(${angle()}, ${num(-300, 300)}, ${num(-300, 300)})`;
    case 'scale':
      return `scale(${num(0.2, 3)}, ${num(0.2, 3)})`;
    case 'scale1':
      return `scale(${num(0.2, 3)})`;
    case 'skewX':
      return `skewX(${angle()})`;
    case 'skewY':
      return `skewY(${angle()})`;
    default:
      return `matrix(${num(-2, 2)}, ${num(-2, 2)}, ${num(-2, 2)}, ${num(-2, 2)}, ${num(-100, 100)}, ${num(-100, 100)})`;
  }
};
// The rect has multiples of 1/64, which are exact in float32, so that the
// cases do not depend on how the browser reads a long decimal number of a
// length attribute.
const dyadic = (lo, hi) => Math.round((lo + rnd() * (hi - lo)) * 64) / 64;
const rect = () => [dyadic(-200, 200), dyadic(-200, 200), dyadic(1, 150), dyadic(1, 80)];

const cases = [];
const add = (kind, transform) => cases.push({ name: `transform-${kind}-${cases.length}`, transform, rect: rect() });

// The transforms that mermaid writes.
for (const t of [
  'translate(-13.295, 11.785) rotate(-45, 0, -2)',
  'translate(-13.295, 11.785) rotate(-45, 50, -2)',
  'translate(-13.295, 11.785) rotate(-45, 100, 88)',
  'translate(-19.845312500000002, 18.544531250000002) rotate(-45, 0, 0)',
  'rotate(-45, 107.5, 90)',
  'rotate(-45, 0, 40)',
  'translate(12,12) rotate(45, 0,30)',
  'translate(14,14) rotate(45, 225.7109375,130)',
  'translate(-19, -13.5)',
  'translate(-22.5, 0)',
  'translate(72.7890625, 630)',
  'translate(100, 50)',
  'translate(100.5, 50.25)',
  'rotate(30)',
  'rotate(-90)',
  'rotate(90, 10, 10)',
  'rotate(180)',
  'rotate(270, 3.5, -7.25)',
  'scale(2, 0.5)',
  'scale(1.5)',
  'translate(10,10) scale(2, 0.5) rotate(45 5 5)',
  'translate(10,10) scale(2, 0.5) translate(3.3, 4.4) rotate(45 5 5)',
  'scale(2) translate(10.1, 20.2)',
  'scale(0.1) translate(10.1, 20.2) scale(10) translate(-3.7, 1.9)',
  'matrix(0.866,0.5,-0.5,0.866,10,20)',
  'matrix(1,0,0,1,10.1,20.2) translate(5.5, 6.6)',
  'matrix(2,0,0,3,10.1,20.2) translate(5.5, 6.6)',
  'translate(5.5, 6.6) matrix(2,0,0,3,10.1,20.2) translate(5.5, 6.6)',
  'translate(5.5, 6.6) matrix(1,0,0,1,10.1,20.2) rotate(45)',
  'skewX(30)',
  'skewY(45)',
  'skewX(45) translate(1.1, 2.2)',
  'translate(1.1, 2.2) skewX(0) translate(3.3, 4.4)',
  'translate(1.1, 2.2) rotate(0) translate(3.3, 4.4)',
  'translate(1.1, 2.2) rotate(360) translate(3.3, 4.4)',
  'translate(1.1, 2.2) scale(1) translate(3.3, 4.4)',
  'translate(1.1, 2.2) scale(-1, 1) translate(3.3, 4.4)',
  'translate(0.1, 0.2) translate(0.3, 0.4) translate(0.5, 0.6) translate(100.7, -200.8)',
  'rotate(45) rotate(45)',
  'rotate(45) translate(10.3, 0) rotate(-45)',
]) {
  add('fixed', t);
}
// One operation.
for (let i = 0; i < 150; i++) add('one', op());
// Lists of operations.
for (let i = 0; i < 300; i++) {
  const n = 2 + Math.floor(rnd() * 3);
  const ops = [];
  for (let k = 0; k < n; k++) ops.push(op());
  add('list', ops.join(' '));
}
// Translations and rotations only, as in the diagrams.
for (let i = 0; i < 200; i++) {
  const n = 1 + Math.floor(rnd() * 3);
  const ops = [];
  for (let k = 0; k < n; k++) {
    ops.push(pick([
      () => `translate(${num(-300, 300)}, ${num(-300, 300)})`,
      () => `rotate(${pick([45, -45, 90, -90])}, ${num(-300, 300)}, ${num(-300, 300)})`,
      () => `rotate(${pick([45, -45, 90, -90, 180])})`,
    ])());
  }
  add('diagram', ops.join(' '));
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'transform-cases.json');
const resultsFile = path.join(outDir, 'transform-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(here, 'browser-transform-probe.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
