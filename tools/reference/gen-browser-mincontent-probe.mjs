// Writes internal/browser/testdata/mincontent-cases.json and
// mincontent-results.json: DOM probe cases for the min-content width of a
// text, and what Chromium measures for them.
//
// A label of mermaid that wraps is a div with display: table and a width.
// A table is never narrower than its min-content width, so a label with a
// word that is wider than the wrapping width is as wide as that word. The
// width of the foreignObject, the size of the node and the positions of
// the layout follow from it.
//
// Blink measures the words of a text for the min-content width in a fast
// path (LineBreaker::HandleTextForFastMinContent): the width of a word is
// the difference of the positions of its two ends in the shape result of
// the text, and the positions are float values. Only the last word of the
// text goes through the normal path, which adds the advances of its
// glyphs. The two results can differ by one LayoutUnit (1/64 px) after the
// width is rounded up.
//
// Each case is a div of display: table that is 20px wide, so that the
// rectangle of the div is as wide as the min-content width of the text.
// The widest word is at a random place in the text.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-mincontent-probe.mjs
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { runProbe } from './probe-run.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

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
const rnd = prng(0x6d696e63);
const pick = (arr) => arr[Math.floor(rnd() * arr.length)];

const fonts = [
  "font-family:'trebuchet ms',verdana,arial,sans-serif;font-size:16px",
  "font-family:'trebuchet ms',verdana,arial,sans-serif;font-size:14px",
  'font-family:arial,sans-serif;font-size:14px',
  'font-family:arial,sans-serif;font-size:16px',
  'font-family:arial,sans-serif;font-size:11px',
  'font-family:courier;font-size:14px',
  'font-family:times;font-size:16px',
  'font-family:verdana;font-size:13px',
  'font-family:arial,sans-serif;font-size:18px;font-weight:bold',
];
const shortWords = ['a', 'of', 'the', 'is', 'to', 'in', 'it', 'Lorem', 'dolor', 'sit', 'amet,', 'and', 'or', 'label', 'for', 'this', 'node', 'Wy', 'To:', 'AV', 'fi', '100%', '"sit"', '(x)', 'id1'];
const longWords = [
  'amet,"consectetur"', 'Server:Service', 'adipiscing', 'Supercalifragilistic', 'SAM.CommonFA.CommonFAFinanceNurseryDays',
  'horizontal-cylinder', 'stacked-document', 'notched-pentagon', '[DBServer\\SharedDbInstance].[SupportDb]',
  'markdownAutoWrap', 'https://example.com/a/path', 'WWWWWWWWWW', 'iiiiiiiiiiiiiiiiiiiiiiii', 'Tj.AV.To.Wy.fi', 'internationalization',
];
// text returns words with one long word at the given place.
const text = (count, longAt) => {
  const words = [];
  for (let i = 0; i < count; i++) {
    words.push(i === longAt ? pick(longWords) : pick(shortWords));
  }
  return words;
};

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
// break-spaces is the value of a label of mermaid that wraps, and normal is
// the initial value. The cases do not have pre-wrap, where the spaces at
// the end of a line hang: the engine does not have that rule.
const whiteSpaces = ['break-spaces', 'break-spaces', 'normal'];
const cases = [];
const add = (kind, font, ws, inner) => {
  const style = `display: table; white-space: ${ws}; line-height: 1.5; max-width: 20px; text-align: center; width: 20px; ${font}`;
  cases.push({
    name: `mincontent-${kind}-${cases.length}`,
    html: svg(`<foreignObject id="fo" width="2000px" height="2000px"><div id="d" xmlns="http://www.w3.org/1999/xhtml" style="${style}"><span class="nodeLabel"><p style="margin:0">${inner}</p></span></div></foreignObject>`),
    queries: [{ sel: '#d', op: 'rect' }],
  });
};
for (let i = 0; i < 330; i++) {
  const font = fonts[i % fonts.length];
  const ws = whiteSpaces[i % whiteSpaces.length];
  const count = 3 + Math.floor(rnd() * 14);
  // The long word is the first, the last or a word in the middle.
  const longAt = pick([0, count - 1, 1 + Math.floor(rnd() * Math.max(1, count - 2)), Math.floor(rnd() * count), count - 2]);
  const words = text(count, Math.max(0, longAt));
  const kind = pick(['plain', 'plain', 'plain', 'strong', 'br', 'icon']);
  switch (kind) {
    case 'plain':
      add('plain', font, ws, words.join(' '));
      break;
    case 'strong': {
      // An element in the text: the text is in several items.
      const at = Math.floor(rnd() * words.length);
      words[at] = `<strong>${words[at]}</strong>`;
      add('strong', font, ws, words.join(' '));
      break;
    }
    case 'br': {
      const at = 1 + Math.floor(rnd() * Math.max(1, words.length - 1));
      add('br', font, ws, words.slice(0, at).join(' ') + '<br>' + words.slice(at).join(' '));
      break;
    }
    case 'icon':
      add('icon', font, ws, `<i class="fa fa-cogs"></i> ${words.join(' ')}`);
      break;
  }
}
// The labels of the corpus that showed the rule.
for (const [font, label] of [
  [fonts[2], 'Lorem "ipsum" dolor "sit" amet,"consectetur" adipiscing "elit".'],
  [fonts[5], '<i class="fa fa-cogs"></i> Shared Business Logic Server:Service 1'],
  [fonts[5], '<i class="fa fa-cogs"></i> Dedicated Test Business Logic Server:Service 2'],
  [fonts[5], '<i class="fa fa-circle"></i> [DBServer\\SharedDbInstance].[SupportDb]'],
  [fonts[2], 'This is a label for horizontal-cylinder shape'],
]) {
  add('corpus', font, 'break-spaces', label);
}

runProbe('mincontent', cases);
