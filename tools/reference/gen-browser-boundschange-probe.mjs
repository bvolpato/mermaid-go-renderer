// Writes internal/browser/testdata/boundschange-cases.json and
// boundschange-results.json: DOM probe cases for the bounding box of an SVG
// container whose descendants have empty boxes at the origin, and what
// Chromium measures for them.
//
// Blink computes the box of a container (g, a, svg) only when the layout of
// one of its children reports that bounds changed
// (SVGLayoutResult::bounds_changed). A shape and a text report it on their
// first layout. A container reports it when its own box or its box with the
// strokes is not the empty rectangle at the origin any more, or when it has
// a transform. So a group whose content is an empty box at the origin, for
// example the label of an edge without a text
//
//   <g class="edgeLabel"><g class="label" transform="translate(0, 0)">
//     <text><tspan></tspan></text></g></g>
//
// has a valid box (0, 0, 0, 0), but the group around it never computes a
// box: it is not valid for its own container, and the point (0, 0) is not a
// part of the box of the diagram. With one level less the point is a part
// of it.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-boundschange-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

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
const rnd = prng(0x626e6473);
const pick = (arr) => arr[Math.floor(rnd() * arr.length)];

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
// A rect that is not empty: the box of the outer group is always computed.
const R = '<rect x="100" y="100" width="10" height="10"/>';
// Leaves. The first ones have an empty box at the origin.
const leaves = {
  emptyTextTspan: '<text y="-10.1"><tspan x="0" y="-0.1em" dy="1.1em"></tspan></text>',
  emptyText: '<text y="-10.1"></text>',
  spaceText: '<text><tspan> </tspan></text>',
  emptyTextAt: '<text x="5" y="6"><tspan></tspan></text>',
  emptyTextMoved: '<text transform="translate(5, 6)"><tspan></tspan></text>',
  emptyTextStroke: '<text stroke="black" stroke-width="4"><tspan></tspan></text>',
  emptyRectStroke: '<rect stroke="black" stroke-width="4"/>',
  emptyRect: '<rect/>',
  emptyRectAt: '<rect x="1" y="1" width="0" height="2"/>',
  line0: '<line x1="0" y1="0" x2="0" y2="0"/>',
  line0Stroke: '<line x1="0" y1="0" x2="0" y2="0" stroke="black" stroke-width="4"/>',
  line5: '<line x1="5" y1="5" x2="5" y2="5"/>',
  pathMove: '<path d="M0,0"/>',
  path0: '<path d="M0,0L0,0"/>',
  path0Stroke: '<path d="M0,0L0,0" style="stroke:#333;stroke-width:2px"/>',
  emptyPath: '<path d=""/>',
  circle0: '<circle r="0"/>',
  polygon0: '<polygon points="0,0 0,0 0,0"/>',
  foreign0: '<foreignObject width="0" height="0"></foreignObject>',
  hidden: '<rect x="50" y="50" width="5" height="5" style="display:none"/>',
  defs: '<defs><rect x="50" y="50" width="5" height="5"/></defs>',
  rect: '<rect x="1" y="1" width="2" height="2"/>',
  text: '<text x="3" y="20">x</text>',
};
const leafNames = Object.keys(leaves);
const transforms = ['', '', ' transform="translate(0, 0)"', ' transform="translate(5, 6)"', ' transform="scale(2)"', ' transform="rotate(30)"', ' transform="translate(30, 40)"'];

const cases = [];
let ids = 0;
const add = (kind, inner) => {
  // Every group gets an id, so that the probe reads the box of each one.
  ids = 0;
  const html = svg(`<g id="t">${inner.replaceAll('<g', () => `<g id="g${ids++}"`)}</g>`);
  const queries = [{ sel: '#t', op: 'bbox' }, { sel: '#my-svg', op: 'bbox' }];
  for (let i = 0; i < ids; i++) {
    queries.push({ sel: `#g${i}`, op: 'bbox' });
  }
  cases.push({ name: `boundschange-${kind}-${cases.length}`, html, queries });
};
const wrap = (inner, depth, transformAt = -1, transform = '') => {
  let out = inner;
  for (let level = depth - 1; level >= 0; level--) {
    out = `<g${level === transformAt ? transform : ''}>${out}</g>`;
  }
  return out;
};

// Every leaf alone and beside the rect, at each depth.
for (const name of leafNames) {
  for (let depth = 0; depth <= 3; depth++) {
    add(`leaf-${name}`, wrap(leaves[name], depth) + R);
    add(`alone-${name}`, wrap(leaves[name], depth));
  }
}
// A transform on one level of the nesting.
for (const name of ['emptyTextTspan', 'line0', 'emptyRect', 'path0', 'rect']) {
  for (let depth = 1; depth <= 3; depth++) {
    for (let at = 0; at < depth; at++) {
      for (const transform of transforms.slice(2)) {
        add(`transform-${name}`, wrap(leaves[name], depth, at, transform) + R);
      }
    }
  }
}
// Two children in a group that is nested once or twice.
for (const first of leafNames) {
  for (const second of ['emptyTextTspan', 'emptyRectAt', 'emptyText']) {
    add('pair', `<g>${leaves[first]}${leaves[second]}</g>` + R);
    add('pair', `<g><g>${leaves[first]}</g>${leaves[second]}</g>` + R);
    add('pair', `<g><g>${leaves[first]}</g><g>${leaves[second]}</g></g>` + R);
    add('pair', `<g><g><g>${leaves[first]}</g></g>${leaves[second]}</g>` + R);
  }
}
// The labels of the edges of mermaid: with a text, without a text, with
// and without a position.
const label = (text, position = '') =>
  `<g class="edgeLabel"${position}><g class="label" transform="translate(0, 0)"><text y="-10.1" text-anchor="middle"><tspan class="text-outer-tspan row" x="0" y="-0.1em" dy="1.1em" text-anchor="middle">${text}</tspan></text></g></g>`;
const background = '<g><rect class="background" style="stroke: none"></rect></g>';
add('mermaid', label('') + background + R);
add('mermaid', label('', ' transform="translate(30, 40)"') + background + R);
add('mermaid', label('<tspan font-style="normal" class="text-inner-tspan" font-weight="normal">Cool</tspan>', ' transform="translate(30, 40)"') + R);
add('mermaid', `<g class="edgeLabels">${label('')}${background}${label('')}${background}</g>` + R);
add('mermaid', `<g class="root"><g class="clusters"></g><g class="edgePaths"></g><g class="edgeLabels">${label('')}${background}</g><g class="nodes">${R}</g></g>`);
add('mermaid', `<g class="edgeTerminals" transform="translate(10, 20)"><g class="inner" transform="translate(0, 0)"></g></g>` + R);
add('mermaid', `<g class="edgeTerminals"><g class="inner" transform="translate(0, 0)"><text><tspan></tspan></text></g></g>` + R);
// Seeded random trees.
const tree = (depth) => {
  const n = 1 + Math.floor(rnd() * 2);
  let out = '';
  for (let i = 0; i < n; i++) {
    if (depth < 3 && rnd() < 0.55) {
      out += `<g${pick(transforms)}>${tree(depth + 1)}</g>`;
    } else {
      out += leaves[pick(leafNames)];
    }
  }
  return out;
};
for (let i = 0; i < 300; i++) {
  add('random', tree(0) + (rnd() < 0.7 ? R : ''));
}

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'boundschange-cases.json');
const resultsFile = path.join(outDir, 'boundschange-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
// The oracle prints the results with line breaks. Store them compact.
fs.writeFileSync(resultsFile, JSON.stringify(JSON.parse(fs.readFileSync(resultsFile, 'utf8'))) + '\n');
console.error(`${cases.length} cases`);
