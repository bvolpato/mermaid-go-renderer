// Writes internal/browser/testdata/tablecell-cases.json and
// tablecell-results.json: DOM probe cases for a table that fills a
// foreignObject and for percentage heights, and what Chromium measures for
// them.
//
// The journey and eventmodeling diagrams center a text in a box with
//
//   <foreignObject x y width height>
//     <div style="display: table; height: 100%; width: 100%">
//       <div style="display: table-cell; text-align: center; vertical-align: middle">text
//
// and the venn diagram uses a span with display: flex, width: 100% and
// height: 100%. The cases show these rules of Blink:
//
//   - A percentage height refers to the height of the containing block
//     when that height is definite. The foreignObject has a definite
//     height. A block with an automatic height does not.
//   - The height of a table is a minimum: the table grows with its content.
//   - A table-cell in a table is as wide and as high as the table, and
//     vertical-align places its content in it.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-tablecell-probe.mjs
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { runProbe } from './probe-run.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const fonts = ['font: 14px arial;', "font-family: 'trebuchet ms', verdana, arial, sans-serif; font-size: 16px;", 'font: 13px/1.5 verdana;', ''];
const boxes = [
  [550, 50],
  [165, 80],
  [150, 50],
  [60, 21],
  [40, 10],
  [111.24784770870022, 111.24784770870022],
  [74.06279939177557, 33.5],
];
const tables = [
  'display: table; height: 100%; width: 100%;',
  'display: table; width: 100%;',
  'display: table; height: 100%;',
  'display: table; height: 30px; width: 80px;',
  'display: table; height: 50%; width: 50%;',
  'display: table; height: 100%; width: 100%; padding: 3px; border: 1px solid red;',
];
const cells = [
  'display: table-cell; text-align: center; vertical-align: middle;',
  'display: table-cell; text-align: center; vertical-align: top;',
  'display: table-cell; text-align: center; vertical-align: bottom;',
  'display: table-cell; text-align: center;',
  'display: table-cell; vertical-align: middle; padding: 2px 4px;',
  'display: table-cell; text-align: right; vertical-align: middle; height: 40px;',
];
const inners = ['Onboarding', '<b>CartScreen</b>', 'Use Core Functionality', 'one<br>two', '', '<p style="margin: 0">para</p>'];

const cases = [];
const queries = [{ sel: '#fo, #fo *', op: 'rect', all: true }];
const add = (kind, w, h, font, inner) =>
  cases.push({
    name: `tablecell-${kind}-${cases.length}`,
    html: svg(`<g transform="translate(20.5, 30.25)"><foreignObject id="fo" x="10" y="20" width="${w}" height="${h}"><div xmlns="http://www.w3.org/1999/xhtml" style="${font}">${inner}</div></foreignObject></g>`),
    queries,
  });
let n = 0;
for (const table of tables) {
  for (const cell of cells) {
    for (const tag of ['div', 'span']) {
      const [w, h] = boxes[n % boxes.length];
      const font = fonts[n % fonts.length];
      const inner = inners[n % inners.length];
      n++;
      // The table is the child of the foreignObject, as in the diagrams.
      cases.push({
        name: `tablecell-table-${cases.length}`,
        html: svg(`<g transform="translate(20.5, 30.25)"><foreignObject id="fo" x="10" y="20" width="${w}" height="${h}"><div xmlns="http://www.w3.org/1999/xhtml" style="${table}${font}"><${tag} style="${cell}">${inner}</${tag}></div></foreignObject></g>`),
        queries,
      });
    }
  }
}
// A cell in a row, and a cell with no table around it. The cases have no
// table with two cells: the engine does not share the width of a table
// between the cells of a row.
add('row', 200, 60, fonts[0], `<div style="display: table; width: 100%; height: 100%"><div style="display: table-row"><div style="display: table-cell; vertical-align: middle; text-align: center">in a row</div></div></div>`);
add('alone', 200, 60, fonts[0], `<div style="display: table-cell; vertical-align: middle; height: 100%">alone</div>`);
// Percentage heights of blocks and of the flex span of the venn diagram.
for (const [w, h] of boxes) {
  for (const font of fonts.slice(0, 2)) {
    cases.push({
      name: `tablecell-flex-${cases.length}`,
      html: svg(`<g transform="translate(0, 24)"><foreignObject id="fo" class="venn-text-node-fo" width="${w}" height="${h}" x="77.57171630748313" y="95.57171790138433" overflow="visible"><span class="venn-text-node" xmlns="http://www.w3.org/1999/xhtml" style="display: flex; width: 100%; height: 100%; white-space: normal; align-items: center; justify-content: center; text-align: center; overflow-wrap: normal; word-break: normal; ${font}">Apple</span></foreignObject></g>`),
      queries,
    });
    add('block-100', w, h, font, `<div style="height: 100%">text</div>`);
    add('block-50', w, h, font, `<div style="height: 50%; width: 50%">text</div>`);
    add('nested', w, h, font, `<div style="height: 100%"><div style="height: 50%">text</div></div>`);
    add('auto-parent', w, h, font, `<div><div style="height: 50%">text</div></div>`);
    add('px-parent', w, h, font, `<div style="height: 40px"><div style="height: 50%">text</div><div style="height: 25%"></div></div>`);
    add('min-height', w, h, font, `<div style="min-height: 100%">text</div>`);
    add('max-height', w, h, font, `<div style="max-height: 50%">one<br>two<br>three</div>`);
  }
}

// A table whose width is larger than its max-width: a label of mermaid has
// this form when its minimum width is larger than its wrapping width.
for (const font of fonts) {
  for (const inner of ['Start', 'a longer text in the label', 'Supercalifragilisticexpialidocious']) {
    for (const style of ['max-width: 120px; width: 200px;', 'width: 200px; max-width: 120px; min-width: 150px;', 'width: 60px; max-width: 120px;', 'max-width: 120px; width: 100%;']) {
      cases.push({
        name: `tablecell-max-width-${cases.length}`,
        html: svg(`<g transform="translate(36, 0)"><foreignObject id="fo" width="200" height="21"><div xmlns="http://www.w3.org/1999/xhtml" style="display: table; white-space: nowrap; line-height: 1.5; text-align: center; ${style}${font}"><span class="nodeLabel"><p style="margin: 0">${inner}</p></span></div></foreignObject></g>`),
        queries,
      });
    }
  }
}

runProbe('tablecell', cases);
