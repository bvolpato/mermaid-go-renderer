// Generates internal/js/testdata/lower_purify.json.gz: String.prototype
// .toLowerCase of V8 for every code point that it changes, and for strings
// around the final sigma rule. Plain node is the oracle: the result does not
// depend on a browser.
//
// Usage: node gen-purify-jslower.mjs
import path from 'node:path';
import { ROOT, mulberry32, writeGolden } from './purify-oracle-lib.mjs';

// changed lists [code point, lowercase string] for every code point whose
// lowercase form is not the code point itself.
const changed = [];
for (let cp = 0; cp <= 0x10ffff; cp++) {
  if (cp >= 0xd800 && cp <= 0xdfff) {
    continue;
  }
  const s = String.fromCodePoint(cp);
  const lower = s.toLowerCase();
  if (lower !== s) {
    changed.push([cp, lower]);
  }
}

// Strings with a capital sigma (U+03A3) in every context that the
// Final_Sigma condition reads: cased letters, case-ignorable characters and
// other characters before and after it.
const SIGMA = String.fromCodePoint(0x3a3);
const pieces = [
  '', 'a', 'A', SIGMA, ' ', '.', "'", ':', '-', '1', String.fromCodePoint(0x301), String.fromCodePoint(0xad),
  String.fromCodePoint(0x2019), String.fromCodePoint(0x1f600), String.fromCodePoint(0x391), String.fromCodePoint(0x2b0),
  String.fromCodePoint(0xb7), String.fromCodePoint(0x1d400), String.fromCodePoint(0x130), '_', String.fromCodePoint(0x5d0),
  String.fromCodePoint(0x2071), String.fromCodePoint(0x345), String.fromCodePoint(0x1c5), String.fromCodePoint(0x200d),
];
const rnd = mulberry32(0x6c6f7772);
const pick = () => pieces[Math.floor(rnd() * pieces.length)];
const strings = new Set();
for (const a of pieces) {
  for (const b of pieces) {
    strings.add(a + SIGMA + b);
    strings.add(a + b + SIGMA);
    strings.add(SIGMA + a + b);
  }
}
for (let i = 0; i < 1500; i++) {
  let s = '';
  const n = 1 + Math.floor(rnd() * 6);
  for (let j = 0; j < n; j++) {
    s += rnd() < 0.4 ? SIGMA : pick();
  }
  strings.add(s);
}
const list = [...strings];

writeGolden(path.join(ROOT, 'internal/js/testdata/lower_purify.json.gz'), {
  unicode: process.versions.unicode,
  changed,
  strings: list.map((s) => [s, s.toLowerCase()]),
});
console.log(`toLowerCase: ${changed.length} code points, ${list.length} strings`);
