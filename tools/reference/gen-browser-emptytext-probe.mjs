// Writes internal/browser/testdata/emptytext-cases.json and
// emptytext-results.json: DOM probe cases for the bounding box of a
// container that has a text element with no characters, and what Chromium
// measures for them.
//
// A text element with no characters is not part of the box of its
// container. But when it has an inline box (a tspan, an a or a textPath
// that is displayed), Chromium counts it as an empty box at the origin of
// the text: the box of the container then includes that point. mermaid
// makes such a text for an empty SVG label, so the size of a state with a
// title and no description depends on it.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-emptytext-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

const svg = (inner) =>
  `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg"><defs><path id="p" d="M0,0L100,0"/></defs>${inner}</svg>`;
const texts = [
  // No inline box: the text is not part of the container box.
  '<text id="x"></text>',
  '<text id="x"> </text>',
  '<text id="x">\n</text>',
  '<text id="x" x="5" y="7"></text>',
  '<text id="x"><!-- c --></text>',
  '<text id="x"><title>t</title></text>',
  '<text id="x"><desc>t</desc></text>',
  '<text id="x"><g></g></text>',
  '<text id="x"><rect width="5" height="5"/></text>',
  '<text id="x"><tspan style="display:none"></tspan></text>',
  '<text id="x"><tspan style="display:none">abc</tspan></text>',
  '<text id="x"><tspan style="display:none"><tspan style="display:inline"></tspan></tspan></text>',
  '<text id="x"><a style="display:none"><tspan></tspan></a></text>',
  '<text id="x" style="display:none">abc</text>',
  '<text id="x" style="display:none"><tspan></tspan></text>',
  // An inline box and no characters: an empty box at the origin.
  '<text id="x"><tspan></tspan></text>',
  '<text id="x"><tspan> </tspan></text>',
  '<text id="x"><tspan>\n  </tspan></text>',
  '<text id="x"><tspan x="0" y="-0.1em" dy="1.1em"></tspan></text>',
  '<text id="x" y="-10.1"><tspan class="text-outer-tspan" x="0" y="-0.1em" dy="1.1em"></tspan></text>',
  '<text id="x"><tspan x="3" y="4"></tspan></text>',
  '<text id="x" x="3" y="4"><tspan></tspan></text>',
  '<text id="x"><tspan dx="5" dy="6"></tspan></text>',
  '<text id="x"><tspan><tspan></tspan></tspan></text>',
  '<text id="x"><tspan x="3" y="4"><tspan font-style="normal"></tspan></tspan></text>',
  '<text id="x"><tspan><title>t</title></tspan></text>',
  '<text id="x"><tspan visibility="hidden"></tspan></text>',
  '<text id="x" text-anchor="middle"><tspan text-anchor="middle"></tspan></text>',
  '<text id="x"><tspan> </tspan><tspan> </tspan></text>',
  '<text id="x" x="9" y="9"> <tspan></tspan> </text>',
  '<text id="x"><a></a></text>',
  '<text id="x"><a href="#"> </a></text>',
  '<text id="x"><textPath href="#p"></textPath></text>',
  // Characters: the box of the characters. An empty tspan adds nothing.
  '<text id="x">abc</text>',
  '<text id="x"><tspan x="0" y="1em"></tspan><tspan x="0" y="2em">ab</tspan></text>',
  '<text id="x"><tspan x="0" y="1em">ab</tspan><tspan x="0" y="3em"></tspan></text>',
];
const moved = (t) => t.replace('<text id="x"', '<text id="x" transform="translate(100, 200)"');
const rect = '<rect x="10" y="20" width="30" height="40"/>';
const cases = [];
texts.forEach((t, i) => {
  const add = (kind, inner, queries) => cases.push({ name: `emptytext-${kind}-${i}`, html: svg(inner), queries });
  // The text alone in a group, beside a rect, moved, in a nested group as
  // createText makes it, and before the rect.
  add('alone', `<g id="t">${t}</g>`, [{ sel: '#t', op: 'bbox' }, { sel: '#x', op: 'bbox' }, { sel: '#my-svg', op: 'bbox' }]);
  add('rect', `<g id="t">${rect}${t}</g>`, [{ sel: '#t', op: 'bbox' }]);
  add('moved', `<g id="t">${rect}${moved(t)}</g>`, [{ sel: '#t', op: 'bbox' }, { sel: '#x', op: 'bbox' }]);
  add('nested', `<g id="t">${rect}<g transform="translate(-50, -60)"><rect class="background"/>${t}</g></g>`, [{ sel: '#t', op: 'bbox' }]);
  add('first', `<g id="t">${moved(t)}${rect}</g>`, [{ sel: '#t', op: 'bbox' }]);
});
// The label group of the rectWithTitle shape with SVG labels: a title and
// an empty description, and an empty title and a description.
const label = (inner, transform) =>
  `<g><rect class="background" style="stroke: none"></rect><text y="-10.1" transform="${transform}"><tspan class="text-outer-tspan title-row" x="0" y="-0.1em" dy="1.1em">${inner}</tspan></text></g>`;
const word = '<tspan font-style="normal" class="text-inner-tspan" font-weight="normal">State</tspan>';
cases.push(
  { name: 'emptytext-title-only', html: svg(`<g id="t" class="label">${label(word, 'translate( 0, 0)')}${label('', 'translate( 18.25, 28)')}</g>`), queries: [{ sel: '#t', op: 'bbox' }] },
  { name: 'emptytext-descr-only', html: svg(`<g id="t" class="label">${label('', 'translate( 18.25, 0)')}${label(word, 'translate( 0, 9)')}</g>`), queries: [{ sel: '#t', op: 'bbox' }] },
  { name: 'emptytext-both-empty', html: svg(`<g id="t" class="label">${label('', 'translate( 0, 0)')}${label('', 'translate( 0, 9)')}</g>`), queries: [{ sel: '#t', op: 'bbox' }] }
);

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'emptytext-cases.json');
const resultsFile = path.join(outDir, 'emptytext-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
