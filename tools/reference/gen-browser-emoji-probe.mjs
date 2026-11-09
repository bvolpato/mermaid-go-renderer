// Writes internal/browser/testdata/emoji-probe.json.gz: DOM probe cases for
// text with emoji, and what Chromium measures for them.
//
// On macOS the emoji font is Apple Color Emoji, a font of color bitmaps.
// Chromium measures its glyphs in another way than the glyphs of an outline
// font, and it chooses the font by the presentation of a character. The
// cases cover:
//
//   - The advance. CoreText draws a glyph larger than the font size at small
//     sizes (1.25 times the size up to 16px, 20px to 24px between 16px and
//     24px, the font size from 24px), and Blink rounds the advance to a
//     whole pixel.
//   - The bounding box of SVG text, which is the union of the text box and
//     the bounds of the glyphs. The bounds of an emoji glyph are the
//     rectangle of its image, rounded out, plus one pixel on each side. The
//     cases have fonts with a small ascent (Rockwell) and a small descent
//     (Hiragino Sans), so that the bounds of the glyph show at the top and
//     at the bottom.
//   - An svg that scales its content: Blink lays out SVG text with the font
//     of the screen size.
//   - The emoji font as the font of the text: the line metrics follow the
//     drawn size.
//   - The choice of the font: a character with emoji presentation (by
//     default, by the variation selector 16, in a keycap, a flag or a
//     joined sequence) gets the emoji font first. A character with text
//     presentation gets a text font (the cases of the kind "symbol").
//   - HTML content: the size of a block with a line that has an emoji, for
//     line-height normal (the metrics of the emoji font count) and for a
//     line height that is a number.
//
// A query can have the field "compare": "size". The Go test then compares
// only the width and the height of the rectangle. The engine does not have
// the position of an inline-block in a foreignObject, with or without an
// emoji.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-emoji-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/browser/testdata/emoji-probe.json.gz');

const trebuchet = '"trebuchet ms",verdana,arial,sans-serif';
const roots = {
  plain: '',
  // The svg of a treemap of mermaid: the content is scaled by 0.784.
  s784: 'viewBox="0 0 1000 400" style="max-width: 1000px;"',
  s2: 'viewBox="0 0 392 400"',
  s05: 'viewBox="0 0 1568 400"',
};
const svg = (root, family, size, text, extra = '', attrs = '') =>
  `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg" ${roots[root]}><style>#my-svg{font-family:${family};font-size:16px;}</style><g><text id="t" x="50" y="300" ${attrs} style="font-size: ${size}px;${extra}">${text}</text></g></svg>`;
const textQueries = [{ sel: '#t', op: 'bbox' }, { sel: '#t', op: 'textlen' }];
const cases = [];
const add = (kind, html, queries = textQueries, text) => {
  const c = { name: `emoji-${kind}-${cases.length}`, html, queries };
  if (text !== undefined) c.text = text;
  cases.push(c);
};
const r2 = (v) => Math.round(v * 100) / 100;

// The advance and the bounds of one glyph and of a row of glyphs.
for (let s = 1; s <= 64; s += 0.5) {
  add('size', svg('plain', trebuchet, s, '🟡'));
  add('size', svg('plain', trebuchet, s, '🟡🟡🟡'));
}
// Font sizes in steps of 0.1px over the sizes at which the drawn size
// changes its rule, and over the sizes from 24px, where the right edge of
// the bounds is one pixel further out at each whole pixel.
for (let s = 15; s <= 34.001; s = r2(s + 0.1)) add('edge', svg('plain', trebuchet, s, '🟡'));
// The top and the bottom of the bounds, with a font of a small ascent and a
// font of a small descent. The text "x" gives the text box of the font.
for (const family of ['Rockwell', '"Hiragino Sans"']) {
  for (let s = 1; s <= 70; s += 0.5) {
    add('vertical', svg('plain', family, s, '🟡'));
    add('vertical', svg('plain', family, s, 'x'));
  }
  // Sizes at which an edge of the image is at a whole pixel, and large
  // sizes. Skia draws a glyph of 256px or more in another way: the bounds
  // are then not whole pixels, and the cases stop below that size.
  for (const s of [0.3, 0.5, 17.6, 19.2, 20.8, 22.4, 36.3, 40.1, 47.7, 63.9, 64.1, 72, 80, 88, 96, 100, 104, 112, 120, 128, 136, 144, 152, 160, 168, 176, 184, 192, 200, 208, 216, 224, 232, 240, 248, 255.5]) {
    add('vertical', svg('plain', family, s, '🟡'));
  }
}
// An svg that scales its content.
for (const root of ['s784', 's2', 's05']) {
  for (let s = 2; s <= 60; s += 1) {
    add('scaled', svg(root, 'Rockwell', s, '🟡'));
    add('scaled', svg(root, '"Hiragino Sans"', s, '🟡'));
  }
}
// The emoji font as the font of the text.
for (let s = 1; s <= 70; s += 0.5) add('primary', svg('plain', '"Apple Color Emoji"', s, '🟡'));
for (const s of [80, 100, 160]) add('primary', svg('plain', '"Apple Color Emoji"', s, '🟡'));
for (let s = 4; s <= 40; s += 2) add('primary', svg('plain', '"Apple Color Emoji"', s, 'x🟡x'));
// Emoji sequences and emoji with the variation selectors: characters with
// emoji presentation, which get the emoji font first.
const sequences = [
  '👨‍👩‍👧', '🇧🇷', '👍🏽', '1️⃣', '❤️', '☀️', '©️', '😀', '🏳️‍🌈', '👩🏾‍💻', '✅', '⭐', '⚠️', '✔️', '➡️', '🔥x🔥', 'a😀', '🟡\u200d', '#️⃣', '*️⃣',
  '🧑‍🤝‍🧑', '🫠', '🪿', '™️', '↔️', '▶️', '♻️', '✨', '❌', '❓', '➕', '🅰️', '🈚', '〰️', '🀄', '🏴󠁧󠁢󠁥󠁮󠁧󠁿', '👋🏻', '🇺', '🇺🇸🇧', '😀\ufe0e', '3\ufe0f',
  '👨\u200d', '\u200d👩', '🚫', '✂️', '☔', '⌚', '⏰', '⬛', '🔢', 'ℹ️', '‼️',
];
for (const text of sequences) for (const s of [12, 16, 20, 30]) add('sequence', svg('plain', trebuchet, s, text), textQueries, text);
// Characters with text presentation: emoji characters that Unicode draws as
// text by default, without the variation selector 16. They get a text font,
// which CoreText chooses. The last ones have a combining enclosing mark.
const symbols = [
  '❤', '☀', '©', '⚠', '✔', '➡', '™', '↔', '▶', '♻', '🅰', '〰', '☀\ufe0e', '✂', '✈', '✉', '☎', '☑', '♀', '♠', '⌨', '⬆', 'ℹ', '‼',
  '3\u20e3', 'a\u20e3', '1\u20e32', '☀\u20e0', '😀\u20e0',
];
for (const text of symbols) for (const s of [12, 16, 20, 30]) add('symbol', svg('plain', trebuchet, s, text), textQueries, text);
// Baselines and anchors.
for (const base of ['middle', 'central', 'hanging', 'text-before-edge', 'text-after-edge', 'ideographic', 'mathematical']) {
  for (const s of [12, 20, 30]) add('baseline', svg('plain', trebuchet, s, '🟡 a', `dominant-baseline: ${base};`));
}
for (const anchor of ['middle', 'end']) {
  for (const s of [12, 20, 30]) add('anchor', svg('plain', trebuchet, s, '🟡 a 🟡', `text-anchor: ${anchor};`));
}
// Emoji in a text. The font weight, the slant and the family of the text do
// not change the advance of the emoji.
const texts = ['🟡 Low Activity (15%)', '🔴 High Activity (Top 50%)', '🟠 Medium', '🟡 L...', '🟡 Lo...', 'a 🟡 b', '😀 x', '✅ ok', 'x🟡', '🟡x🟡'];
for (const text of texts) {
  for (const s of [4, 10, 12, 14, 16, 18, 21, 28, 38]) {
    add('text', svg('plain', trebuchet, s, text));
    add('text', svg('s784', trebuchet, s, text));
  }
  add('bold', svg('s784', trebuchet, 12, text, 'dominant-baseline: middle; fill:black; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;', 'font-weight="bold"'));
  add('italic', svg('plain', trebuchet, 10, text, '', 'font-style="italic"'));
  add('family', svg('plain', 'times', 13, text));
  add('family', svg('plain', 'courier', 13, text));
}
// HTML content: the size of the block.
const htmlStyle = `<style>#my-svg{font-family:${trebuchet};font-size:16px;}</style>`;
const blockSize = [{ sel: '#d', op: 'rect', compare: 'size' }];
for (const lineHeight of ['', 'line-height: 1.5;', 'line-height: 1;', 'line-height: 30px;']) {
  for (const s of [8, 10, 12, 14, 16, 18, 20, 24, 30, 40]) {
    for (const inner of ['🟡', '🟡 and text', 'text 😀 x more', 'x', '⚠️ warn', '❤ ✔', '1️⃣ 🇧🇷 👍🏽']) {
      add('html', `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${htmlStyle}<foreignObject width="2000" height="300"><div id="d" xmlns="http://www.w3.org/1999/xhtml" style="display: inline-block; white-space: nowrap; font-size:${s}px;${lineHeight}">${inner}</div></foreignObject></svg>`, blockSize);
    }
  }
}
// The label of a flowchart node, as mermaid writes it.
for (const text of ['🔴 High', 'Start 🚀', '⚠️ Warning', '✅', 'a ❤ b', '1️⃣ first', '😀😀😀', 'x']) {
  add(
    'label',
    `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${htmlStyle}<style>#my-svg .label{font-family:${trebuchet};color:#333;}#my-svg .nodeLabel p{margin:0;}#my-svg p{margin:0;}</style><g class="node default"><g class="label" style="" transform="translate(-30, -12)"><rect></rect><foreignObject width="200" height="100"><div id="d" xmlns="http://www.w3.org/1999/xhtml" style="display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;"><span id="t" class="nodeLabel"><p>${text}</p></span></div></foreignObject></g></g></svg>`,
    [{ sel: '#t', op: 'rect' }, { sel: '#d', op: 'rect' }]
  );
}

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'emojiprobe-'));
const casesFile = path.join(tmp, 'cases.json');
const resultsFile = path.join(tmp, 'results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }));
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultsFile, 'utf8'));
if (results.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${results.length}`);
}
const golden = cases.map((c, i) => ({ ...c, results: results[i].results }));
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(Buffer.from(JSON.stringify({ cases: golden })), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
console.error(`${path.relative(repoRoot, outFile)}: ${cases.length} cases, ${(fs.statSync(outFile).size / 1024).toFixed(0)} KiB`);
