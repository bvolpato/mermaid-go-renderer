// Writes internal/browser/testdata/linewrap-cases.json and
// linewrap-results.json: DOM probe cases for the lines of a text that
// wraps, and what Chromium measures for them.
//
// A label of mermaid that wraps is a block of a fixed width. The cases
// show where its lines break and how wide they are:
//
//   - The widest line of a wrapped text: the rectangle of the span starts
//     at the line that is widest, so text-align shows the width of that
//     line. A part of a text on a line is as wide as the sum of its glyphs,
//     rounded up to a LayoutUnit.
//   - A text with inline elements (strong, em) and no break opportunity
//     around them, as markdown makes with no-break spaces: when the content
//     after the last break opportunity of a line does not fit, the line
//     ends at that break opportunity (LineBreaker::HandleOverflow).
//
//   - A line that starts in the middle of a text is shaped again at its
//     start. In Times a W after a space has a part of its advance from the
//     kerning with the space, and at the start of a line it does not.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-linewrap-probe.mjs
import { runProbe } from './probe-run.mjs';

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
const rnd = prng(0x6c696e65);
const pick = (arr) => arr[Math.floor(rnd() * arr.length)];

const fonts = [
  "font-family:'trebuchet ms',verdana,arial,sans-serif;font-size:16px",
  'font-family:arial,sans-serif;font-size:14px',
  'font-family:courier;font-size:14px',
  'font-family:times;font-size:16px',
  'font-family:verdana;font-size:13px',
  'font-family:arial,sans-serif;font-size:11px',
];
const words = ['a', 'of', 'the', 'is', 'to', 'in', 'it', 'Lorem', 'dolor', 'sit', 'amet', 'and', 'or', 'label', 'for', 'this', 'node', 'Wy', 'To', 'AV', 'fi', 'adipiscing', 'consectetur', 'elit', 'paragraph', 'wrapping', 'several', 'lines', 'longer', 'needs'];
const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const cases = [];
const queries = [{ sel: '#fo *', op: 'rect', all: true }];
const add = (kind, style, inner) =>
  cases.push({
    name: `linewrap-${kind}-${cases.length}`,
    html: svg(`<foreignObject id="fo" width="2000px" height="2000px"><div xmlns="http://www.w3.org/1999/xhtml" style="${style}">${inner}</div></foreignObject>`),
    queries,
  });

// The widest line of a text of words.
for (let i = 0; i < 240; i++) {
  const font = fonts[i % fonts.length];
  const align = ['right', 'center', 'left'][i % 3];
  const n = 8 + Math.floor(rnd() * 10);
  const text = Array.from({ length: n }, () => pick(words)).join(' ');
  const w = pick([90, 120, 120, 150, 200, 200]);
  add('words', `width: ${w}px; text-align: ${align}; white-space: normal; line-height: 1.5; ${font}`, `<span>${text}</span>`);
}

// Inline elements in a run with no break opportunity around them.
const nb = ' ';
const runs = [
  `The${nb}dog${nb}in${nb}<strong>the</strong>${nb}hog.(1)..${nb}a${nb}a${nb}a${nb}a${nb}<em>very${nb}long${nb}text</em>${nb}about${nb}it<br>Word!`,
  'The dog in <strong>the</strong> hog.(1).. a a a a <em>very long text</em> about it<br>Word!',
  '`one` and <strong>two</strong> and <em>three</em>',
  'word<strong>bold</strong>word and more<em>italic</em>words here',
  'a.b(<strong>c</strong>)d.e[<em>f</em>]g and so on',
  'short <strong>Supercalifragilistic</strong>expialidocious tail',
  `one${nb}two <strong>three${nb}four</strong>${nb}five six${nb}<em>seven</em> eight`,
  'x-<strong>y</strong>-z a-b-<em>c</em>-d e?f<strong>g</strong>',
  'call(<strong>arg</strong>).then(<em>next</em>).catch(e)',
  '<strong>A</strong>.<em>B</em>.<strong>C</strong>.<em>D</em>.<strong>E</strong>.<em>F</em>',
];
const widths = [40, 60, 90, 120, 200];
const modes = ['white-space: break-spaces;', 'white-space: normal;'];
for (const run of runs) {
  for (const w of widths) {
    for (const mode of modes) {
      const font = fonts[cases.length % fonts.length];
      add('inline', `display: table; ${mode} line-height: 1.5; max-width: ${w}px; text-align: center; width: ${w}px; ${font}`, `<span class="nodeLabel markdown-node-label"><p style="margin: 0">${run}</p></span>`);
    }
  }
}

runProbe('linewrap', cases);
