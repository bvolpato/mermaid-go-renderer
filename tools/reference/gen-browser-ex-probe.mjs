// Writes internal/browser/testdata/ex-cases.json and ex-results.json: DOM
// probe cases for the CSS unit "ex" and what Chromium measures for them.
// The cases cover font-size in ex (attribute and style, which resolve
// against the font of the parent), and ex in other lengths (x, y, dx, dy,
// width, height, letter-spacing, and the width, the padding and the margin
// of HTML content).
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-ex-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

const fonts = [
  "'trebuchet ms', verdana, arial, sans-serif",
  'arial',
  'verdana',
  'courier',
  'times',
  "'Open Sans', sans-serif",
  'sans-serif',
  'georgia',
];
const sizes = [16, 10, 13.5, 24];
const svg = (style, inner) =>
  `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg" style="${style}">${inner}</svg>`;
const textQueries = [
  { sel: '#t', op: 'bbox' },
  { sel: '#t', op: 'style', arg: 'font-size' },
  { sel: '#t', op: 'textlen' },
];

const cases = [];
const add = (kind, html, queries) => cases.push({ name: `ex-${kind}-${cases.length}`, html, queries });

for (const fam of fonts) {
  for (const size of sizes) {
    const root = `font-family:${fam};font-size:${size}px`;
    // font-size in ex as an attribute and as a style, as mermaid writes the
    // titles of the timeline and of the user journey.
    add('attr', svg(root, '<text id="t" x="245" font-size="4ex" font-weight="bold" y="20">History of Social Media Platform</text>'), textQueries);
    add('style', svg(root, '<text id="t" x="10" y="20" style="font-size: 2.5ex">Timeline of Industrial Revolution</text>'), textQueries);
    add('attr', svg(root, '<text id="t" y="25" font-size="1ex">xX</text>'), textQueries);
    // The ex of font-size is the x-height of the font of the parent.
    add('nested', svg(root, '<g style="font-size: 30px; font-family: arial"><text id="t" y="25" font-size="2ex">Nested</text></g>'), textQueries);
    add('nested', svg(root, '<g font-weight="bold" font-style="italic"><text id="t" y="25" font-size="3ex">Bold italic parent</text></g>'), textQueries);
    add('tspan', svg(root, '<text id="t" y="25">a<tspan id="s" font-size="3ex">tspan</tspan></text>'), [...textQueries, { sel: '#s', op: 'style', arg: 'font-size' }, { sel: '#s', op: 'textlen' }]);
    // ex in the position attributes of text.
    add('pos', svg(root, '<text id="t" x="2ex" y="3ex">Positions</text>'), textQueries);
    add('pos', svg(root, '<text id="t" x="10" y="20" dx="1.5ex" dy="2ex">Deltas<tspan x="0" dy="1.2ex">line</tspan></text>'), textQueries);
    add('pos', svg(root, '<text id="t" y="20" font-size="20" font-weight="bold" dy="1ex">Own size</text>'), textQueries);
    // ex in the geometry attributes of shapes.
    add('shape', svg(root, '<rect id="t" x="1ex" y="0.5ex" width="10ex" height="2ex"/><circle id="c" cx="3ex" cy="3ex" r="1.5ex"/><line id="l" x1="0" y1="1ex" x2="7ex" y2="1ex"/>'), [
      { sel: '#t', op: 'bbox' },
      { sel: '#c', op: 'bbox' },
      { sel: '#l', op: 'bbox' },
    ]);
    // ex in CSS properties.
    add('css', svg(root, '<text id="t" y="20" style="letter-spacing: 0.5ex">Spacing</text>'), textQueries);
    add('html', svg(root, '<foreignObject id="fo" width="500" height="500"><div id="d" xmlns="http://www.w3.org/1999/xhtml" style="width: 10ex; padding: 1ex; margin-left: 2ex">text</div><div id="e" xmlns="http://www.w3.org/1999/xhtml" style="font-size: 2ex; width: 5ex">more</div></foreignObject>'), [
      { sel: '#d', op: 'rect' },
      { sel: '#e', op: 'rect' },
      { sel: '#e', op: 'style', arg: 'font-size' },
    ]);
  }
}
// The svg inherits the font of the page when it has no font of its own.
add('page', svg('', '<text id="t" y="30" font-size="4ex">Page font</text>'), textQueries);
add('page', svg('', '<text id="t" y="30" font-size="4ex" font-family="arial">Own family</text>'), textQueries);

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'ex-cases.json');
const resultsFile = path.join(outDir, 'ex-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
