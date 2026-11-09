// Writes internal/browser/testdata/kernsplit-cases.json and
// kernsplit-results.json: DOM probe cases for the width of the parts of an
// SVG text whose font kerns with the legacy `kern` table, and what Chromium
// measures for them.
//
// The kern machine of HarfBuzz (hb-kern.hh) splits the kerning of a pair of
// glyphs: one half goes to the advance of the first glyph, and the rest to
// the advance and to the offset of the second glyph. It halves the value
// after it scaled it to the font size, in 16.16 fixed point. So for an odd
// value of the table the first glyph gets the half of a font unit. With
// Trebuchet MS Bold at 16px the pair "A" + space has the value -113: the
// advance of the "A" is 1297 - 56.5 font units, and not 1297 - 57.
//
// A label of mermaid is a text with one tspan for each word, and the width
// of each tspan is rounded up to 1/64 px on its own. A pair of a letter and
// a space is on the border of two tspans, so the half unit decides the last
// 1/64 px of the width of the label.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-kernsplit-probe.mjs
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
const rnd = prng(0x6b65726e);
const pick = (arr) => arr[Math.floor(rnd() * arr.length)];

const fonts = ["'trebuchet ms', verdana, arial, sans-serif", 'verdana', 'arial', 'times', 'georgia', 'tahoma', "'Recursive Variable', arial, sans-serif", 'sans-serif'];
const weights = ['normal', 'bold', 'bolder'];
const styles = ['', '', 'font-style:italic;'];
const sizes = [16, 14, 12, 10, 11, 13, 18, 20, 24, 32, 13.337, 16.5];
// Words that begin or end with a letter that kerns with a space or with its
// neighbor in many fonts.
const words = ['A', 'label', 'with', 'several', 'words', 'To', 'Type', 'AVA', 'Yes', 'W', 'P.', 'of', 'T', 'F', 'L', 'Vary', 'Tr', 'fly', 'r', 'y', 'T.', 'V,', 'plain', 'node', 'int', 'String', '+get(int', 'i)', ':', 'T', 'void', 'item)', '+put(T', 'Animal', 'Y', 'LT', 'AY', 'Ta', 'ff', '1', '11', 'K'];
const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const cases = [];
// text is a label as createText of mermaid builds it: an outer tspan with
// one inner tspan for each word. Each word but the first starts with a
// space.
const label = (fam, size, weight, style, ws) =>
  svg(
    `<g style="${style}font-family:${fam};font-size:${size}px;font-weight:${weight}"><text id="t" y="-10.1"><tspan id="o" x="0" y="-0.1em" dy="1.1em">${ws
      .map((w, i) => `<tspan id="w${i}">${i === 0 ? w : ' ' + w}</tspan>`)
      .join('')}</tspan></text></g>`
  );
const add = (kind, fam, size, weight, style, ws) =>
  cases.push({
    name: `kernsplit-${kind}-${cases.length}`,
    html: label(fam, size, weight, style, ws),
    queries: [{ sel: '#t', op: 'bbox' }, { sel: '#t', op: 'textlen' }, ...ws.map((_, i) => ({ sel: `#w${i}`, op: 'textlen' })), ...ws.map((_, i) => ({ sel: `#w${i}`, op: 'bbox' }))],
  });

// The label of the class diagram case that showed the difference, word by
// word, in every font and weight.
const sample = ['A', 'label', 'with', 'several', 'words'];
for (const fam of fonts) {
  for (const weight of weights) {
    for (let n = 1; n <= sample.length; n++) {
      add('sample', fam, 16, weight, '', sample.slice(0, n));
    }
  }
}
// Every pair of two words in Trebuchet MS, which has a kern table and no
// kerning in GPOS.
for (const first of ['A', 'T', 'V', 'W', 'Y', 'P.', 'F', 'L', 'r', 'y', 'To', 'AVA', 'K', '1']) {
  for (const second of ['A', 'T', 'V', 'W', 'Y', 'o', 'a', 'v', 'w', 'y', 'J', '1', '.']) {
    add('pair', fonts[0], 16, pick(weights), '', [first, second]);
  }
}
// Seeded random labels.
for (let i = 0; i < 200; i++) {
  const n = 1 + Math.floor(rnd() * 5);
  const ws = [];
  for (let k = 0; k < n; k++) {
    ws.push(pick(words));
  }
  const fam = pick([fonts[0], fonts[0], pick(fonts)]);
  // Tahoma has no italic face. A synthetic italic is another subject.
  const style = pick(styles);
  add('random', fam, pick(sizes), pick(weights), fam === 'tahoma' ? '' : style, ws);
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'kernsplit-cases.json');
const resultsFile = path.join(outDir, 'kernsplit-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
// The oracle prints the results with line breaks. Store them compact.
fs.writeFileSync(resultsFile, JSON.stringify(JSON.parse(fs.readFileSync(resultsFile, 'utf8'))) + '\n');
console.error(`${cases.length} cases`);
