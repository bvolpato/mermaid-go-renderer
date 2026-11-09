// Writes internal/browser/testdata/fontsize-cases.json and
// fontsize-results.json: DOM probe cases for font sizes that have more than
// two decimals, and what Chromium measures for them.
//
// Blink shapes text with FontDescription::EffectiveFontSize(): the computed
// font size cut to a precision of 1/100 px with float arithmetic,
// floorf(size * 100) / 100. The computed style keeps the full size. So a
// font size such as 33.46875px, 13.337px or 16.8px (which is 16.79999... as
// a float) gives glyph advances of a slightly smaller font.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-fontsize-probe.mjs
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
const rnd = prng(0x666f6e74);
const pick = (arr) => arr[Math.floor(rnd() * arr.length)];

const families = ["'trebuchet ms', verdana, arial, sans-serif", 'arial', 'verdana', 'times', 'courier', "'Open Sans', sans-serif", 'georgia', 'helvetica'];
const texts = [
  'History of Social Media Platform',
  'Timeline of Industrial Revolution',
  'The quick brown fox jumps over the lazy dog 0123456789',
  'WAVE To: Tj fi ffl',
  'x',
];
const fixedSizes = [
  33.46875, 33.46, 33.47, 33.4699, 33.4651, 20.91796875, 20.91, 20.92, 8.3671875, 8.36, 8.37, 13.337, 13.33, 13.34,
  16.8, 16.79, 16.81, 14.4, 14.39, 12.8, 19.2, 15.999, 15.99, 16.001, 16.009, 16.01, 9.995, 12.345678, 100.129,
  17.6, 11.2, 22.4, 10.1, 10.7, 18.72, 13.28, 0.83 * 16, 1.17 * 16, 0.67 * 16, 16 / 1.2, 16 * 1.2 * 1.2,
];

// Blink caches the platform fonts of a page by family, style and a size key
// of 1/100 px. A font that an earlier case made would serve a later case
// with a near size, and the later case would not show what a new page does.
// So each case gets a font (family, weight, style) that no earlier case
// used with a size nearer than 0.05 px.
const fontsInUse = [];
for (const family of families) {
  for (const weight of ['normal', 'bold']) {
    for (const style of ['normal', 'italic']) {
      fontsInUse.push({ css: `font-family:${family};font-weight:${weight};font-style:${style}`, sizes: [] });
    }
  }
}
const fontFor = (size) => {
  const start = Math.floor(rnd() * fontsInUse.length);
  for (let i = 0; i < fontsInUse.length; i++) {
    const font = fontsInUse[(start + i) % fontsInUse.length];
    if (font.sizes.every((used) => Math.abs(used - size) > 0.05)) {
      font.sizes.push(size);
      return font.css;
    }
  }
  return null;
};

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const cases = [];
const add = (kind, html, queries) => cases.push({ name: `fontsize-${kind}-${cases.length}`, html, queries });
const textQueries = [
  { sel: '#t', op: 'bbox' },
  { sel: '#t', op: 'textlen' },
  { sel: '#t', op: 'style', arg: 'font-size' },
];
const htmlQueries = [
  { sel: '#d', op: 'rect' },
  { sel: '#s', op: 'rect' },
  { sel: '#d', op: 'style', arg: 'font-size' },
];
// one adds a case with SVG text and a case with HTML text of the same font
// and size.
const one = (size, text) => {
  const font = fontFor(size);
  if (font === null) {
    return;
  }
  add('svg', svg(`<text id="t" x="10" y="40" style="${font};font-size:${size}px">${text}</text>`), textQueries);
  add('html', svg(`<foreignObject width="2000" height="300"><div id="d" xmlns="http://www.w3.org/1999/xhtml" style="white-space: nowrap; ${font};font-size:${size}px"><span id="s">${text}</span></div></foreignObject>`), htmlQueries);
};
for (const size of fixedSizes) {
  one(size, texts[0]);
}
// Seeded random sizes with many decimals.
for (let i = 0; i < 150; i++) {
  const size = 6 + rnd() * 40;
  one(pick([size, Math.round(size * 1000) / 1000, Math.round(size * 100) / 100, Math.round(size * 10) / 10]), pick(texts));
}
// Relative sizes that give many decimals.
for (const [rel, factor] of [['1.05em', 1.05], ['0.9em', 0.9], ['85%', 0.85], ['1.17em', 1.17], ['smaller', 1 / 1.2], ['larger', 1.2], ['0.83em', 0.83], ['1.3333em', 1.3333]]) {
  for (const base of [16, 14, 13]) {
    const font = fontFor(base * factor);
    add('rel', svg(`<g style="${font};font-size:${base}px"><text id="t" x="10" y="40" style="font-size:${rel}">${texts[1]}</text></g>`), textQueries);
  }
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'fontsize-cases.json');
const resultsFile = path.join(outDir, 'fontsize-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
