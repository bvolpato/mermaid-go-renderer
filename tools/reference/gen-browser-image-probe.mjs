// Writes internal/browser/testdata/image-cases.json and image-results.json:
// probe cases for img elements in the labels of mermaid, and what Chromium
// measures for them before and after the images load.
//
// A label of mermaid can have img elements. mermaid measures the label a
// first time when it sets the markup, and a second time after each image
// fired its load or error event and got its style (configureLabelImages:
// display: flex, and min-width and max-width for a label with no text, or
// width: 100% for a label with text). The cases show the size of an image
// in each state:
//
//   - pending, right after the markup is set: 0 x 0, also for a data URL;
//   - loaded: the natural size, and the height that the aspect ratio gives
//     when the style gives the width;
//   - failed (an error event): the broken image icon, 16 x 16, with no
//     aspect ratio;
//   - no src or an empty src: 0 x 0, with no event.
//
// The images are data URLs, so the page needs no network. Each case has
// its own URL (a parameter of the media type with the number of the case),
// because Chromium keeps an image that it loaded: an img element with the
// same URL would be complete at once. The cases have no
// alt attribute and no SVG image without a width and a height: the engine
// does not lay out the alternative text of a failed image, and it gives an
// SVG image with only a viewBox its natural size of 300 x 150.
//
// The script runs probes/images.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-image-probe.mjs
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { runProbe } from './probe-run.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

// png returns a data URL of a gray PNG image.
function png(w, h) {
  const chunk = (type, data) => {
    const len = Buffer.alloc(4);
    len.writeUInt32BE(data.length);
    const td = Buffer.concat([Buffer.from(type), data]);
    const crc = Buffer.alloc(4);
    crc.writeUInt32BE(zlib.crc32(td) >>> 0);
    return Buffer.concat([len, td, crc]);
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0);
  ihdr.writeUInt32BE(h, 4);
  ihdr[8] = 8;
  const raw = Buffer.alloc((w + 1) * h, 0x80);
  for (let y = 0; y < h; y++) {
    raw[y * (w + 1)] = 0;
  }
  const file = Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw)), chunk('IEND', Buffer.alloc(0))]);
  return 'data:image/png;base64,' + file.toString('base64');
}
const svgUrl = (attrs) => 'data:image/svg+xml,' + encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" ${attrs}><rect width="10" height="10"/></svg>`);
// Each source is a function of the number of the case.
const unique = (url, n) => url.replace(/^data:([a-z+/]+)/, `data:$1;case=${n}`);
const sources = {
  png40x20: (n) => unique(png(40, 20), n),
  png100x100: (n) => unique(png(100, 100), n),
  png16x64: (n) => unique(png(16, 64), n),
  png300x7: (n) => unique(png(300, 7), n),
  svg30x60: (n) => unique(svgUrl('width="30" height="60"'), n),
  svg40viewBox: (n) => unique(svgUrl('width="40" viewBox="0 0 100 50"'), n),
  badPng: (n) => `data:image/png;case=${n};base64,AAAA`,
  badScheme: (n) => `x-none:foo${n}`,
  empty: () => '',
};

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const font = 'font-family: arial; font-size: 14px;';
// The styles of the div of a label: the first measure, and the two forms
// after it.
const divs = {
  cell: 'display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;',
  table: 'display: table; white-space: nowrap; line-height: 1.5; max-width: 120px; text-align: center; width: 120px;',
  wrap: 'display: table; white-space: break-spaces; line-height: 1.5; max-width: 200px; text-align: center; width: 200px;',
};
// The styles that configureLabelImages sets.
const alone = 'display: flex; flex-direction: column; min-width: 80px; max-width: 80px;';
const withText = 'display: flex; flex-direction: column; width: 100%;';

const cases = [];
const add = (name, div, inner) =>
  cases.push({
    name: `image-${name}-${cases.length}`,
    html: svg(`<g transform="translate(20.5, 30.25)"><foreignObject id="fo" width="2000px" height="2000px"><div xmlns="http://www.w3.org/1999/xhtml" style="${divs[div]}${font}"><span class="nodeLabel"><p style="margin:0">${inner}</p></span></div></foreignObject></g>`),
  });
for (const [name, source] of Object.entries(sources)) {
  const img = (attrs = '') => `<img src="${source(cases.length)}"${attrs}>`;
  for (const div of Object.keys(divs)) {
    add(`${name}-${div}-plain`, div, img());
    add(`${name}-${div}-in-text`, div, `some ${img()} text`);
    add(`${name}-${div}-alone`, div, img(` style="${alone}"`));
    add(`${name}-${div}-with-text`, div, `${img(` style="${withText}"`)} some text`);
    add(`${name}-${div}-two`, div, `${img(` style="${withText}"`)} more text ${img(` style="${withText}"`)}`);
  }
  add(`${name}-attrs`, 'cell', `x ${img(' width="30" height="20"')} y`);
  add(`${name}-width-attr`, 'cell', `x ${img(' width="30"')} y`);
  add(`${name}-height-style`, 'cell', `x ${img(' style="height: 30px"')} y`);
  add(`${name}-max-height`, 'cell', `x ${img(' style="max-height: 10px"')} y`);
  add(`${name}-block`, 'table', `x ${img(' style="display: block"')} y`);
  add(`${name}-padding`, 'cell', `x ${img(' style="padding: 2px 3px; border: 1px solid red; margin: 0 4px"')} y`);
}
add('nosrc-plain', 'cell', '<img>');
add('nosrc-alone', 'cell', `<img style="${alone}">`);
add('nosrc-with-text', 'table', `<img style="${withText}"> some text`);

runProbe('image', cases, path.join(here, 'probes/images.js'));
