// Writes internal/browser/testdata/baseline-cases.json and
// baseline-results.json: DOM probe cases for the bounding box of an SVG text
// with a dominant-baseline or an alignment-baseline, and what Chromium
// measures for them.
//
// Blink places the box of a text fragment from FontMetrics::FixedAscent and
// FixedDescent of the baseline type. Each of the two is rounded to a
// LayoutUnit (1/64 px) on its own. So the box of a text with
// dominant-baseline="middle" can be 1/64 px taller than ascent + descent:
// with Arial at 16px it is 9.859375 + 7.15625 = 17.015625, not 17.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-baseline-probe.mjs
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
const rnd = prng(0x62617365);
const pick = (arr) => arr[Math.floor(rnd() * arr.length)];

const fonts = ["'trebuchet ms', verdana, arial, sans-serif", 'arial', 'verdana', 'times', 'courier', "'Open Sans', sans-serif", 'sans-serif', "'Recursive Variable', arial, sans-serif"];
const texts = ['overflown the actor', 'Extremely utterly', 'x', 'Hola', 'gjpqy', 'box as it is much', 'TEST 123', '(a) [b] {c}'];
const baselines = [
  'auto', 'middle', 'central', 'hanging', 'mathematical', 'alphabetic', 'ideographic',
  'text-before-edge', 'text-after-edge', 'text-top', 'text-bottom', 'baseline', 'bogus',
  'before-edge', 'after-edge', 'use-script', 'no-change', 'reset-size',
];
const sizes = [16, 14, 12, 10, 11, 13, 15, 17, 18, 20, 24, 9, 32, 16.5, 13.337, 22.4];
const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const cases = [];
const add = (kind, html, queries) => cases.push({ name: `baseline-${kind}-${cases.length}`, html, queries });
const queries = [{ sel: '#t', op: 'bbox' }];

// Every baseline with every font at 16px, and with Arial at every size.
for (const baseline of baselines) {
  for (const fam of fonts) {
    add('dominant', svg(`<text id="t" x="10" y="50" dominant-baseline="${baseline}" style="font-family:${fam};font-size:16px">${texts[0]}</text>`), queries);
  }
  for (const size of sizes) {
    add('size', svg(`<text id="t" x="10" y="50" dominant-baseline="${baseline}" style="font-family:arial;font-size:${size}px">${texts[1]}</text>`), queries);
  }
  add('alignment', svg(`<text id="t" x="10" y="50" alignment-baseline="${baseline}" style="font-family:arial;font-size:16px">${texts[0]}</text>`), queries);
  add('both', svg(`<text id="t" x="10" y="50" dominant-baseline="${baseline}" alignment-baseline="${baseline}" style="font-family:arial;font-size:16px">${texts[3]}</text>`), queries);
  add('tspan', svg(`<text id="t" x="10" y="50" dominant-baseline="${baseline}" style="font-family:arial;font-size:16px"><tspan x="20" dy="1em">${texts[0]}</tspan></text>`), queries);
  add('css', svg(`<text id="t" x="10" y="50" style="dominant-baseline:${baseline};font-family:arial;font-size:16px">${texts[0]}</text>`), queries);
}
// A dominant-baseline together with another alignment-baseline: the first
// gives the height of the box, and the second its position.
const mixed = ['auto', 'middle', 'central', 'hanging', 'mathematical', 'text-before-edge', 'text-after-edge', 'ideographic', 'alphabetic', 'baseline', 'before-edge', 'bogus'];
for (const dominant of mixed) {
  for (const alignment of mixed) {
    add('mixed', svg(`<text id="t" x="10" y="50" dominant-baseline="${dominant}" alignment-baseline="${alignment}" style="font-family:arial;font-size:16px">${texts[0]}</text>`), queries);
    add('mixed', svg(`<text id="t" x="10" y="50" dominant-baseline="${dominant}" alignment-baseline="${alignment}" style="font-family:verdana;font-size:13px">${texts[4]}</text>`), queries);
  }
}
// An inherited dominant-baseline, and an alignment-baseline on a parent,
// which is not inherited.
for (const baseline of mixed) {
  add('inherit', svg(`<g dominant-baseline="${baseline}"><text id="t" x="10" y="50" style="font-family:arial;font-size:16px">${texts[0]}</text></g>`), queries);
  add('inherit', svg(`<g alignment-baseline="${baseline}"><text id="t" x="10" y="50" style="font-family:arial;font-size:16px">${texts[0]}</text></g>`), queries);
  add('inherit', svg(`<g dominant-baseline="middle"><text id="t" x="10" y="50" dominant-baseline="${baseline}" style="font-family:arial;font-size:16px">${texts[0]}</text></g>`), queries);
  add('inherit', svg(`<text id="t" x="10" y="50" dominant-baseline="${baseline}" style="font-family:arial;font-size:16px"><tspan alignment-baseline="central">${texts[0]}</tspan></text>`), queries);
}
// The texts that the sequence diagram draws: a note line and an actor label.
for (const text of texts) {
  add('note', svg(`<text id="t" x="75" y="135" text-anchor="middle" dominant-baseline="middle" alignment-baseline="middle" class="noteText" dy="1em" style="font-family:arial;font-size:16px;font-weight:400"><tspan x="75">${text}</tspan></text>`), queries);
  add('actor', svg(`<text id="t" x="75" y="32.5" dominant-baseline="central" alignment-baseline="central" style="text-anchor:middle;font-family:arial;font-size:16px;font-weight:400"><tspan x="75" dy="0">${text}</tspan></text>`), queries);
}
// Seeded random combinations.
for (let i = 0; i < 200; i++) {
  const size = pick([pick(sizes), Math.round((8 + rnd() * 30) * 100) / 100]);
  const y = pick([50, 50.5, 33.3, 47.015625, Math.round(rnd() * 8000) / 100]);
  const attr = pick(['dominant-baseline', 'dominant-baseline', 'alignment-baseline']);
  const weight = pick(['', '', 'font-weight:bold;']);
  add('random', svg(`<text id="t" x="${pick([10, 10.25, 0])}" y="${y}" ${attr}="${pick(baselines)}" style="${weight}font-family:${pick(fonts)};font-size:${size}px">${pick(texts)}</text>`), queries);
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'baseline-cases.json');
const resultsFile = path.join(outDir, 'baseline-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
