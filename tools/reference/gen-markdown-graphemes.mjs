// Writes internal/markdown/testdata/graphemes.json.gz: how Intl.Segmenter
// of V8 segments each code point in the contexts that the grapheme cluster
// rules of Unicode name (UAX #29).
//
//   cd tools/reference && node gen-markdown-graphemes.mjs
//
// Each context is a list of code points, where -1 is the place of the code
// point under test. The result of a context is the lengths of the segments,
// in code points, for example "1,2". The file holds, for each context and
// each result, the ranges of code points that have this result. The
// surrogates are left out.
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { REPO } from './markdown-lib.mjs';

const C = -1;
const contexts = [
  [0x61, C, 0x61], // letter, c, letter
  [C, 0x301, C], // c, Extend, c
  [C, C],
  [0x0d, C, 0x0a], // CR, c, LF
  [0x1f600, C, 0x200d, 0x1f600], // Extended_Pictographic, c, ZWJ, Extended_Pictographic
  [0x1f600, 0x200d, C, 0x200d, C], // ZWJ, c
  [0x1f600, 0x1f600, 0x200d, C],
  [0x915, 0x94d, C], // InCB consonant, linker, c
  [0x915, C, 0x915], // consonant, c, consonant
  [0x915, 0x94d, C, 0x915], // consonant, linker, c, consonant
  [0x915, C, 0x94d, 0x915], // consonant, c, linker, consonant
  [C, 0x94d, C],
  [0x1100, C], // Hangul L, c
  [0x1161, C], // Hangul V, c
  [0x11a8, C], // Hangul T, c
  [0xac00, C], // Hangul LV, c
  [0xac01, C], // Hangul LVT, c
  [C, 0x1100],
  [C, 0x1161],
  [C, 0x11a8],
  [0x1f1e6, C, 0x1f1e6], // Regional_Indicator, c, Regional_Indicator
  [0x600, C], // Prepend, c
  [C, 0x903], // c, SpacingMark
];

const segmenter = new Intl.Segmenter();
const results = contexts.map(() => new Map());
const texts = contexts.map((context) => context.map((p) => (p === C ? null : String.fromCodePoint(p))));
for (let cp = 0; cp <= 0x10ffff; cp++) {
  if (cp >= 0xd800 && cp <= 0xdfff) continue;
  const ch = String.fromCodePoint(cp);
  for (let k = 0; k < contexts.length; k++) {
    let text = '';
    for (const part of texts[k]) text += part ?? ch;
    let sig = '';
    for (const { segment } of segmenter.segment(text)) sig += (sig ? ',' : '') + [...segment].length;
    const ranges = results[k].get(sig) ?? results[k].set(sig, []).get(sig);
    const tail = ranges.at(-1);
    // The surrogates are not tested, so a range can go over them.
    if (tail && (tail[1] === cp - 1 || (tail[1] === 0xd7ff && cp === 0xe000))) tail[1] = cp;
    else ranges.push([cp, cp]);
  }
}

const out = {
  unicode: process.versions.unicode,
  icu: process.versions.icu,
  contexts: contexts.map((context, k) => ({ context, results: Object.fromEntries(results[k]) })),
};
const file = path.join(REPO, 'internal/markdown/testdata/graphemes.json.gz');
const data = zlib.gzipSync(JSON.stringify(out), { level: 9 });
fs.writeFileSync(file, data);
console.log(`wrote ${file}: ${contexts.length} contexts, ${data.length} bytes`);
