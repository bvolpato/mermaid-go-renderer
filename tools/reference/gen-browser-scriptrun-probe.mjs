// Writes internal/browser/testdata/scriptrun-cases.json and
// scriptrun-results.json: DOM probe cases for texts that mix scripts, and
// what Chromium measures for them.
//
// Blink splits a text into runs of one script before it shapes the text
// (RunSegmenter with ScriptRunIterator). A character of the script Common,
// such as a space, a digit or a punctuation mark, goes with the run before
// it. HarfBuzz shapes each run alone, so that a kerning pair does not apply
// across the limit of two runs: in "Привет Alice" the space is in the
// Cyrillic run, and the pair of the space and the "A" is not kerned.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-scriptrun-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

const families = [
  "'trebuchet ms', verdana, arial, sans-serif",
  'arial, sans-serif',
  'verdana',
  "'Open Sans', sans-serif",
  'times',
  'georgia',
];
const texts = [
  // A word of another script, then a Latin word that starts with a letter
  // that has kerning pairs with the space.
  'Привет Alice',
  'Alice Привет',
  'Привет, Alice! Yes',
  'OK Привет API service 100%',
  'fox database Привет A Zürich W API',
  'Ελληνικά Alice v1.2.3 authentication API',
  'Alice Ελληνικά Tom',
  '日本語 Alice fox I the',
  'Bob 100% 中文 A I a',
  "it's < 한국어 A",
  '日本語 A',
  'the 中文 jumps < 한국어 Alice (optional)',
  // Brackets: a closing bracket goes with the run of its opening bracket.
  'Привет (Alice) мир',
  '(Привет) Alice',
  'Alice (Привет Tom) Yes',
  '[Ελ] {AV} (То)',
  'Привет (A [Tom] Y) W',
  // Characters of the script Common only: one run of the script Common.
  '1234567890',
  '100%',
  '11.5 (74.1%)',
  '2014-01-07',
  '... !? 1/7',
  // Common characters before the first letter go with the run of the letter.
  '100% Привет',
  '(1) Alice',
  '7 Tom AVATAR',
  // Controls of one script.
  'AVATAR To Wa Yo',
  'Привет мир',
];

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const escapeText = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;');
const cases = [];
const queries = [
  { sel: '#t', op: 'textlen' },
  { sel: '#t', op: 'bbox' },
];
let n = 0;
// The fallback font for Han and Hangul text depends on the kind of the
// first font: CoreText gives a serif font for a serif font. The engine does
// not have that rule, which is not about script runs, so these texts are
// measured with the sans-serif families only.
const serif = new Set(['times', 'georgia']);
const hasCJK = (text) => /[\u3040-\u30ff\u3400-\u9fff\uac00-\ud7af]/.test(text);
for (const text of texts) {
  for (const family of families) {
    if (serif.has(family) && hasCJK(text)) {
      continue;
    }
    // Two of the six combinations of size and weight for each text and
    // family, in turn, to keep the file small.
    const combos = [
      [12, 'normal'],
      [24, 'normal'],
      [16, 'bold'],
      [19, 'bold'],
      [14, 'normal'],
      [23, 'bold'],
    ];
    for (let k = 0; k < 2; k++) {
      const [size, weight] = combos[(n + k * 3) % combos.length];
      cases.push({
        name: `scriptrun-${cases.length}`,
        html: svg(`<text id="t" x="10" y="40" style="font-family:${family};font-size:${size}px;font-weight:${weight}">${escapeText(text)}</text>`),
        queries,
      });
    }
    n++;
  }
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'scriptrun-cases.json');
const resultsFile = path.join(outDir, 'scriptrun-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
