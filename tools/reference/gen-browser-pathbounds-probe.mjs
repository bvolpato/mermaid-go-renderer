// Writes internal/browser/testdata/pathbounds-cases.json and
// pathbounds-results.json: DOM probe cases for the bounding box of lines,
// polylines, polygons and paths, and what Chromium measures for them.
//
// Blink converts the bounds of a Skia path (left, top, right, bottom) to a
// gfx::RectF (x, y, width, height) with gfx::RectF::SetByBounds. The width
// and the height are float32 differences. When the rounding of a difference
// makes x + width smaller than right, or y + height smaller than bottom, the
// width or the height becomes the next float32. For example the line from
// y 117.8 to y 423.4 has the height 305.6000061035156 and not
// 305.5999755859375.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-pathbounds-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

function prng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rnd = prng(0x626f756e);
const pick = (arr) => arr[Math.floor(rnd() * arr.length)];
// num is a coordinate with up to three decimals, at a random magnitude.
const num = () => {
  const magnitude = pick([1, 10, 100, 100, 1000, 1000]);
  const digits = pick([0, 1, 1, 1, 2, 3]);
  const v = (rnd() * 2 - 0.3) * magnitude;
  return Number(v.toFixed(digits));
};

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const cases = [];
const add = (kind, html, queries) => cases.push({ name: `pathbounds-${kind}-${cases.length}`, html, queries });
const queries = [
  { sel: '#my-svg', op: 'bbox' },
  { sel: '#l', op: 'bbox' },
  { sel: '#b', op: 'bbox' },
];

// The dashed line of a timeline without sections, alone and with the
// elements that the bounding box of the svg unites it with.
const line = '<line id="l" x1="295" y1="117.8" x2="295" y2="423.4"/>';
for (const inner of [
  `<g id="b">${line}</g>`,
  `<g transform="translate(200, 50)"><rect width="190" height="67.8"/></g><g id="b">${line}</g>`,
  `${line}<rect id="b" x="200" y="-11" width="10" height="10"/>`,
  `<g id="b">${line}</g><rect x="200" y="-11" width="10" height="10"/>`,
  `<rect x="200" y="-11" width="10" height="10"/><g id="b">${line}</g>`,
  `<rect x="200" y="50" width="10" height="10"/><g id="b">${line}</g><rect x="200" y="-11" width="10" height="10"/>`,
  '<path id="l" d="M295,117.8L295,423.4"/><rect id="b" x="200" y="-11" width="10" height="10"/>',
  '<g id="b"><line id="l" x1="295" y1="117.8" x2="300" y2="423.4"/></g><rect x="200" y="-11" width="10" height="10"/>',
  '<g id="b" transform="translate(10.5, 20.25)"><line id="l" x1="295" y1="117.8" x2="300" y2="423.4"/></g>',
  '<g id="b" transform="scale(1.5)"><line id="l" x1="295" y1="117.8" x2="300" y2="423.4"/></g>',
]) {
  add('timeline', svg(inner), queries);
}
// Seeded random shapes.
for (let i = 0; i < 400; i++) {
  const kind = pick(['line', 'line', 'polyline', 'polygon', 'path', 'curve']);
  let shape;
  if (kind === 'line') {
    shape = `<line id="l" x1="${num()}" y1="${num()}" x2="${num()}" y2="${num()}"/>`;
  } else if (kind === 'polyline' || kind === 'polygon') {
    const points = [];
    for (let k = 2 + Math.floor(rnd() * 4); k > 0; k--) {
      points.push(`${num()},${num()}`);
    }
    shape = `<${kind} id="l" points="${points.join(' ')}"/>`;
  } else if (kind === 'path') {
    let d = `M${num()},${num()}`;
    for (let k = 1 + Math.floor(rnd() * 4); k > 0; k--) {
      d += pick([`L${num()},${num()}`, `H${num()}`, `V${num()}`, `l${num()},${num()}`]);
    }
    shape = `<path id="l" d="${d}${pick(['', 'Z'])}"/>`;
  } else {
    shape = `<path id="l" d="M${num()},${num()}Q${num()},${num()} ${num()},${num()}C${num()},${num()} ${num()},${num()} ${num()},${num()}"/>`;
  }
  const other = `<rect x="${num()}" y="${num()}" width="${Math.abs(num())}" height="${Math.abs(num())}"/>`;
  const wrap = pick([
    (s) => `<g id="b">${s}</g>`,
    (s) => `<g id="b">${s}</g>${other}`,
    (s) => `${other}<g id="b">${s}</g>`,
    (s) => `<g id="b" transform="translate(${num()}, ${num()})">${s}</g>${other}`,
    (s) => `<g id="b">${s}${other}</g>`,
  ]);
  add(kind, svg(wrap(shape)), queries);
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'pathbounds-cases.json');
const resultsFile = path.join(outDir, 'pathbounds-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
