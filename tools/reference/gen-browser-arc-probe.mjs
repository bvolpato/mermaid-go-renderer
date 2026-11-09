// Writes internal/browser/testdata/arc-cases.json and arc-results.json: DOM
// probe cases for the bounding box of paths with elliptical arcs that start
// or end on an axis of their ellipse, and what Chromium measures for them.
//
// Skia converts an SVG arc to conics in SkPath::arcTo. It takes the start
// and the end angle from atan2f of the C library. For an arc that starts or
// ends at the leftmost point of its ellipse the call is atan2f(0, x) with a
// negative x. The C library of macOS returns 0x40490FDA for it, the float32
// below pi, and not the nearest float32 0x40490FDB. The control point of the
// first conic then lies 1.2e-7 radii outside the ellipse, and the bounding
// box grows by one float32 step when the center of the ellipse is at 0.
// The bucket shape of mermaid draws such arcs.
//
// The arcs of these cases have half and quarter turns, so that every other
// call of the C library has an argument at which it returns the nearest
// float32.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-arc-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const cases = [];
const add = (kind, d) =>
  cases.push({
    name: `arc-${kind}-${cases.length}`,
    html: svg(`<path id="t" d="${d}"/>`),
    queries: [{ sel: '#t', op: 'bbox' }],
  });

const radii = [
  [75, 12],
  [60, 9.6],
  [45, 7.2],
  [100, 12],
  [40, 5],
  [61.5, 9.84],
  [75, 6],
  [37.5, 6],
  [54, 12],
  [20, 20],
  [12.5, 30.25],
];
const offsets = [-21, -18.5, 0, 3.3, 12.25, 30.5, -100.5];
for (const [rx, ry] of radii) {
  for (const y of offsets) {
    // Half ellipses with the center at x 0: from the right end to the left
    // end, and back, with both sweep flags.
    for (const sweep of [0, 1]) {
      add('half-x0', `M${rx},${y} A${rx},${ry} 0 0 ${sweep} ${-rx},${y}`);
      add('half-x0-back', `M${-rx},${y} A${rx},${ry} 0 0 ${sweep} ${rx},${y}`);
    }
  }
}
for (const [rx, ry] of radii.slice(0, 6)) {
  for (const c of [3.3, -18.5, 40]) {
    // Half ellipses with the center away from 0, as the cylinder shape has.
    add('half-offset', `M${c},${c} a${rx},${ry} 0,0,0 ${2 * rx},0 a${rx},${ry} 0,0,0 ${-2 * rx},0`);
    add('half-offset-cw', `M${c},${c} a${rx},${ry} 0,0,1 ${2 * rx},0 a${rx},${ry} 0,0,1 ${-2 * rx},0`);
    // Half ellipses on the vertical axis.
    add('half-vertical', `M${c},${-ry} A${rx},${ry} 0 0 0 ${c},${ry}`);
    add('half-vertical-cw', `M${c},${-ry} A${rx},${ry} 0 0 1 ${c},${ry}`);
    // Quarter ellipses that start or end at the leftmost point.
    add('quarter-from-left', `M${-rx},${c} A${rx},${ry} 0 0 1 0,${c - ry}`);
    add('quarter-to-left', `M0,${c + ry} A${rx},${ry} 0 0 1 ${-rx},${c}`);
    add('quarter-from-left-ccw', `M${-rx},${c} A${rx},${ry} 0 0 0 0,${c + ry}`);
    add('quarter-to-left-ccw', `M0,${c - ry} A${rx},${ry} 0 0 0 ${-rx},${c}`);
    // Three quarters of an ellipse through the leftmost point.
    add('large', `M0,${c - ry} A${rx},${ry} 0 1 0 ${rx},${c}`);
  }
}
// The body of the bucket shape of mermaid.
for (const [w, y, h] of [
  [150, -21, 54],
  [150, -18.5, 49],
  [120, -15.3, 44.6],
  [90.5, -12.25, 40],
  [200, -30.5, 73],
  [163.4, -22.61, 57.2],
]) {
  const rim = Math.max(Math.min(w * 0.08, 12), 5);
  const bottomW = w * 0.72;
  add(
    'bucket',
    `M${-w / 2},${y} L${-bottomW / 2},${y + h} A${bottomW / 2},${rim} 0 0 0 ${bottomW / 2},${y + h} L${w / 2},${y} A${w / 2},${rim} 0 0 0 ${-w / 2},${y} Z`
  );
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'arc-cases.json');
const resultsFile = path.join(outDir, 'arc-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
