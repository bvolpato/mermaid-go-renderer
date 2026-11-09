// Writes internal/browser/testdata/pathlength-cases.json and
// pathlength-results.json: DOM probe cases for getTotalLength and
// getPointAtLength of a path, and what Chromium returns for them.
//
// Blink measures a path from its normalized segments in float32
// (PathTraversalState). The length of a line is (end - start).Length() of
// gfx::Vector2dF: the squares and their sum are in double precision, and
// the root is rounded to float32. Float32 squares give another last bit in
// a part of the cases. The center of an arc comes from
// delta.LengthSquared(), a double, and the scale factor is computed in
// double precision too. The cases are paths as the edges of mermaid have
// them: lines and curves of d3 with three decimals, rounded corners with
// full doubles, and arcs of line hops.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-pathlength-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

let a = 0x706c656e >>> 0;
const rnd = () => {
  a = (a + 0x6d2b79f5) >>> 0;
  let t = a;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const r3 = (v) => Math.round(v * 1000) / 1000;
const svg = (d) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg"><path id="p" d="${d}"/></svg>`;
const cases = [];
const add = (kind, d) => {
  const queries = [{ sel: '#p', op: 'len' }];
  for (const arg of [1.5, 30, 61.125, 1000]) {
    queries.push({ sel: '#p', op: 'pt', arg });
  }
  cases.push({ name: `pathlength-${kind}-${cases.length}`, html: svg(d), queries });
};
const walk = (n, fmt, step) => {
  let x = 20 + rnd() * 200;
  let y = 20 + rnd() * 100;
  const pts = [];
  for (let i = 0; i < n; i++) {
    pts.push([fmt(x), fmt(y)]);
    x += (rnd() - 0.5) * step;
    y += (rnd() - 0.25) * step;
  }
  return pts;
};

// Paths that the edge golden files of internal/ru/edges showed to differ.
add('seen', 'M84.893,75.75L72.548,56.18L70.162,59.358L70.503,66.232L66.316,65.751L61.82,72.021L63.306,78.184L64.356,78.71L65.325,83.057L68.869,85.963L64.995,87.012L95.164,63.103');
add('seen', 'M39.72401281858968,111.9375L42.5271802097559,126.49121717736125L50.241860419511795,166.5449343547225L52.75611529193745,179.59865153208375');
add('seen', 'M59.499,40.063L58.47,39.117L88.902,79.617L127.481,128.334L131.48,177.371');
add('seen', 'M59.938,47.063L62.631,47.815');
add('seen', 'M46.578,58.91C46.578,78.061,8.434,78.061,8.434,97.212C8.434,96.384,-18.48,96.384,-18.48,95.557C-18.48,102.683,11.676,102.683,11.676,109.809C11.676,137.799,50.682,137.799,50.682,165.789');
// Polylines: three decimals as d3 writes them, and full doubles.
for (let i = 0; i < 70; i++) {
  const pts = walk(2 + Math.floor(rnd() * 10), i % 2 ? r3 : (v) => v, i % 3 === 0 ? 9 : 120);
  add('lines', 'M' + pts.map((p) => p.join(',')).join('L') + (i % 7 === 0 ? 'Z' : ''));
}
// Cubic curves, as curveBasis and the other curves of d3 write them.
for (let i = 0; i < 60; i++) {
  const pts = walk(1 + 3 * (1 + Math.floor(rnd() * 4)), i % 2 ? r3 : (v) => v, i % 3 === 0 ? 12 : 100);
  let d = `M${pts[0].join(',')}`;
  for (let k = 1; k + 2 < pts.length; k += 3) {
    d += `C${pts[k].join(',')},${pts[k + 1].join(',')},${pts[k + 2].join(',')}`;
  }
  add('cubics', d);
}
// Rounded corners (lines and quadratic curves), as generateRoundedPath
// writes them.
for (let i = 0; i < 40; i++) {
  const pts = walk(3 + Math.floor(rnd() * 6), (v) => v, 90);
  let d = `M${pts[0].join(',')}`;
  for (let k = 1; k < pts.length - 1; k++) {
    const [px, py] = pts[k - 1];
    const [cx, cy] = pts[k];
    const [nx, ny] = pts[k + 1];
    const l1 = Math.hypot(cx - px, cy - py);
    const l2 = Math.hypot(nx - cx, ny - cy);
    const cut = Math.min(5, l1 / 2, l2 / 2);
    d += `L${cx - ((cx - px) / l1) * cut},${cy - ((cy - py) / l1) * cut}Q${cx},${cy} ${cx + ((nx - cx) / l2) * cut},${cy + ((ny - cy) / l2) * cut}`;
  }
  d += `L${pts[pts.length - 1].join(',')}`;
  add('rounded', d);
}
// Lines with arcs, as the line hops write them: the ends of an arc and its
// radius are rounded to three decimals, so the chord is the diameter, or
// 0.001 more, or 0.001 less. With a shorter chord the center of the arc is
// the root of a small difference, and its last bits depend on the
// precision of that computation.
for (let i = 0; i < 110; i++) {
  const horizontal = i % 4 !== 0;
  const y = 20 + rnd() * 200;
  let x = rnd() * 50;
  const at = (a, b) => (horizontal ? `${r3(a)},${r3(b)}` : `${r3(b)},${r3(a)}`);
  let d = i % 5 === 0 ? `M${at(x, y)}` : '';
  const n = i % 5 === 0 ? 1 + Math.floor(rnd() * 3) : 1;
  for (let k = 0; k < n; k++) {
    x += 15 + rnd() * 60;
    const r = i % 3 === 0 ? 6 : 3.6 + rnd() * 2.4;
    const move = d === '' ? 'M' : ' L';
    d += `${move}${at(x - r, y)} A${r3(r)},${r3(r)} 0 0 ${i % 2} ${at(x + r, y)}`;
  }
  if (i % 5 === 0) {
    d += ` L${at(x + 20 + rnd() * 50, y)}`;
  }
  add('hops', d);
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'pathlength-cases.json');
const resultsFile = path.join(outDir, 'pathlength-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
