// Writes internal/browser/testdata/rootratio-cases.json and
// rootratio-results.json: DOM probe cases for the height of a root svg that
// comes from the ratio of its viewBox, and what Chromium measures for them.
//
// The root svg of a diagram has width="100%", a max-width and a viewBox.
// Its height is the width times the ratio of the viewBox. Blink keeps that
// ratio as two LayoutUnits (LayoutRatioFromSizeF in
// platform/geometry/physical_size.cc): the sizes of the viewBox when both
// are multiples of 1/64, and else a continued fraction of width / height.
// The cases have sizes with decimals, as a diagram has when its padding is
// not a whole number: equal sizes, sizes that are multiples of 1/64, long
// continued fractions and plain decimals. Each case also measures a text,
// because the scale of the svg sets the font size that Blink lays the text
// out with.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-rootratio-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

let a = 0x72617469 >>> 0;
const rnd = () => {
  a = (a + 0x6d2b79f5) >>> 0;
  let t = a;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const pick = (list) => list[Math.floor(rnd() * list.length)];

const text = (t) => `<text id="t" style="font-size:12px;font-family:arial">${t}</text>`;
const allQueries = [
  { sel: '#my-svg', op: 'rect' },
  { sel: '#t', op: 'bbox' },
  { sel: '#t', op: 'textlen' },
];
// A viewBox that is narrower than 1 scales the text more than a thousand
// times. The size of such a text is another subject, so these cases measure
// only the box of the svg.
const queriesFor = (w) => (Number(w) < 1 ? allQueries.slice(0, 1) : allQueries);

// The sizes of a viewBox, as the text of the attribute.
const sizes = [
  // The cases of the cynefin diagram with a padding of 33.3.
  ['866.6', '666.6'],
  ['777.7', '555.5'],
  ['866.6', '680'],
  ['880', '666.6'],
  ['800.3', '600'],
  ['870.1', '670.1'],
  ['1000.7', '432.1'],
  ['433.3', '333.3'],
  // Multiples of 1/64: no continued fraction.
  ['880', '680'],
  ['866.5', '666.25'],
  ['640.015625', '480.984375'],
  // Equal sizes that are not multiples of 1/64.
  ['333.3', '333.3'],
  ['1000.1', '1000.1'],
  // Long continued fractions: the golden ratio, and ratios near it.
  ['1618.034', '1000'],
  ['987.1', '610.1'],
  ['610.1', '987.1'],
  ['1.6180339', '1'],
  // Small and large sizes.
  ['0.3', '0.2'],
  ['3.3', '7.7'],
  ['123456.7', '7.1'],
  ['7.1', '12345.6'],
  ['1000000.3', '999999.7'],
  ['100.01', '100.02'],
  ['50.5', '0.1'],
];
const decimals = (lo, hi) => {
  const v = lo + rnd() * (hi - lo);
  const digits = pick([1, 1, 2, 3, 6]);
  return v.toFixed(digits);
};
for (let i = 0; i < 110; i++) {
  sizes.push([decimals(50, 1500), decimals(50, 1200)]);
}

const cases = [];
sizes.forEach(([w, h], i) => {
  // The svg as a diagram has it: as wide as the container, up to max-width.
  cases.push({
    name: `maxwidth-${i}-${w}x${h}`,
    html: `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg" style="max-width: ${w}px;" viewBox="0 0 ${w} ${h}">${text('Feature discovery')}</svg>`,
    queries: queriesFor(w),
  });
});
// Without max-width the svg has the width of the container.
for (const [w, h] of sizes.slice(0, 40)) {
  cases.push({
    name: `full-${w}x${h}`,
    html: `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${w} ${h}">${text('Hello world')}</svg>`,
    queries: queriesFor(w),
  });
}
// A viewBox that does not start at the origin, and a fixed width.
for (const [w, h] of sizes.slice(0, 30)) {
  cases.push({
    name: `fixed-${w}x${h}`,
    html: `<svg id="my-svg" xmlns="http://www.w3.org/2000/svg" style="width: ${pick(['300px', '517.3px', '80%', '33.3%'])};" viewBox="-10.5 20.25 ${w} ${h}">${text('a')}</svg>`,
    queries: queriesFor(w),
  });
}

const casesFile = path.join(outDir, 'rootratio-cases.json');
const resultsFile = path.join(outDir, 'rootratio-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  ['oracle.mjs', 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { cwd: here, stdio: 'inherit' }
);
console.log(`${cases.length} cases`);
