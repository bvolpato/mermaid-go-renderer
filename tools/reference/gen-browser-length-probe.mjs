// Writes internal/browser/testdata/length-cases.json and length-results.json:
// DOM probe cases for the number of an SVG length attribute (x, y, width,
// height, cx, cy, r, rx, ry) and what Chromium measures for them.
//
// Blink reads a plain decimal number of a length attribute with the fast
// path of its CSS parser. The fast path keeps 7 digits of the fraction and
// drops the rest. A number in another form (a plus sign, an exponent) and a
// length in a style declaration go through the tokenizer, which rounds the
// whole number. The two ways differ for a number that is not a float32. The
// cases use numbers that are exactly between two float32 values, numbers
// one double step from such a tie, and numbers with 17 digits.
//
// Each case has the field `check`: the indexes of the bounding box that the
// test compares. A text case checks only the x position.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-length-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

let a = 0x1e9647 >>> 0;
const rnd = () => {
  a = (a + 0x6d2b79f5) >>> 0;
  let t = a;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const f32 = new Float32Array(1);
const u32 = new Uint32Array(f32.buffer);
const f64 = new Float64Array(1);
const u64 = new BigUint64Array(f64.buffer);
// value returns a positive number of the given magnitude and kind.
const value = (mag, kind) => {
  f32[0] = (0.5 + rnd()) * mag;
  const lo = f32[0];
  u32[0] += 1;
  const hi = f32[0];
  const tie = (lo + hi) / 2;
  if (kind === 'tie') {
    return tie;
  }
  if (kind === 'near') {
    f64[0] = tie;
    u64[0] += rnd() < 0.5 ? 1n : -1n;
    return f64[0];
  }
  return (0.5 + rnd()) * mag;
};
const mags = [0.03, 0.4, 3, 41, 82, 300, 3000, 20000];
const kinds = ['tie', 'tie', 'near', 'random'];
const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;

// The forms of a number. The first group takes the fast path.
const forms = {
  plain: (v) => `${v}`,
  dot: (v) => `${v}`.replace(/^0\./, '.'),
  zeros: (v) => `00${v}`,
  px: (v) => `${v}px`,
  PX: (v) => `${v}PX`,
  // The forms below do not take the fast path.
  exp: (v) => v.toExponential(),
  plus: (v) => `+${v}`,
};

const cases = [];
const add = (kind, html, check) => cases.push({ name: `length-${kind}-${cases.length}`, html, queries: [{ sel: '#t', op: 'bbox' }], check });

for (const [form, fmt] of Object.entries(forms)) {
  for (let i = 0; i < 40; i++) {
    const mag = mags[i % mags.length];
    const v = () => value(mag, kinds[Math.floor(rnd() * kinds.length)]);
    const sign = (s) => (rnd() < 0.5 && form !== 'plus' ? '-' + s : s);
    add(`rect-${form}`, svg(`<rect id="t" x="${sign(fmt(v()))}" y="${sign(fmt(v()))}" width="${fmt(v())}" height="${fmt(v())}"/>`), [0, 1, 2, 3]);
  }
}
for (let i = 0; i < 60; i++) {
  const mag = mags[i % mags.length];
  const v = () => value(mag, kinds[Math.floor(rnd() * kinds.length)]);
  const s = () => (rnd() < 0.5 ? -v() : v());
  add('circle', svg(`<circle id="t" cx="${s()}" cy="${s()}" r="${v()}"/>`), [0, 1, 2, 3]);
  add('ellipse', svg(`<ellipse id="t" cx="${s()}" cy="${s()}" rx="${v()}" ry="${v()}"/>`), [0, 1, 2, 3]);
  add('rect-style', svg(`<rect id="t" style="x:${s()}px;y:${s()}px;width:${v()}px;height:${v()}px"/>`), [0, 1, 2, 3]);
  add('text-x', svg(`<text id="t" x="${s()}" y="0" style="font-family:Arial;font-size:16px">Hxg</text>`), [0]);
  add('tspan-x', svg(`<text id="t" style="font-family:Arial;font-size:16px"><tspan x="${s()}" y="0">Hxg</tspan></text>`), [0]);
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'length-cases.json');
const resultsFile = path.join(outDir, 'length-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { cwd: here, stdio: 'inherit' }
);
// The oracle writes indented JSON. One line is smaller.
fs.writeFileSync(resultsFile, JSON.stringify(JSON.parse(fs.readFileSync(resultsFile, 'utf8'))) + '\n');
console.log(`${casesFile}: ${cases.length} cases`);
